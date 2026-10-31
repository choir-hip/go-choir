package store

// The append-clustered store measurement
// (docs/design/append-clustered-store-2026-10-31.md section 5). Env-gated:
//
//	CHOIR_CHURN_BENCH_OG_ROWS=3340000 CHOIR_CHURN_BENCH_DIR=/big/dir \
//	CHOIR_CHURN_BENCH_OUT=/path/report.json \
//	go test ./internal/store -run TestChurnBench -timeout 6h -v
//
// It builds one synthetic store per layout, shaped like the paper's state.db
// of 2026-10-31 (og_objects kind mix, body and metadata sizes, run_id
// clustering; run_memory_entries at 1.14/3.34 of the og rows with ~2.2 KB
// messages), then measures with the guest's own pragmas (WAL, 4 KiB pages):
//
//   - pages dirtied per 1,000 writes: distinct page numbers in the WAL after
//     the writes, with autocheckpoint off and the WAL empty before, so it is
//     exactly the set of pages the next checkpoint writes into the file;
//   - snapshot chunk reuse across an hour of writes: the 4 MiB page-range
//     chunks of internal/snapshot (1024 pages) hashed before and after the
//     paper's hourly write volume, checkpointed;
//   - zstd CPU per MB on the bodies over 1 KiB.
//
// Layouts: "current" is today's DDL (objectgraph.SchemaDDL and schemaDDL,
// UUIDv4 entry ids); "clustered" is the og_id INTEGER PRIMARY KEY layout with
// canonical_id UNIQUE and no canonical_id in any secondary index, plus UUIDv7
// entry ids; "clustered_zstd" adds zstd to bodies and messages over 1 KiB.

import (
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"io"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/klauspost/compress/zstd"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

type churnLayout struct {
	name      string
	clustered bool
	zstd      bool
}

// clusteredOGDDL is the og_objects DDL of the append-clustered layout:
// derived from the current DDL so the two differ only in the key.
func clusteredOGDDL() string {
	ddl := objectgraph.SchemaDDL
	ddl = strings.Replace(ddl, "canonical_id  TEXT NOT NULL PRIMARY KEY,", "og_id INTEGER PRIMARY KEY,\n\tcanonical_id TEXT NOT NULL UNIQUE,", 1)
	ddl = strings.ReplaceAll(ddl, ", canonical_id)", ")")
	return ddl
}

type churnKind struct {
	kind     string
	share    float64
	bodyMean float64
}

// churnKinds is the paper's og_objects mix (small-object-storage section 1):
// choir.event 52% at 1.37 KB, the semantic kinds after it.
var churnKinds = []churnKind{
	{"choir.event", 0.524, 1370},
	{"choir.lifecycle_command", 0.11, 1900},
	{"choir.texture_revision", 0.06, 2600},
	{"choir.run", 0.05, 2800},
	{"choir.source_ref", 0.09, 1400},
	{"choir.commitment_record", 0.07, 1500},
	{"choir.agent", 0.02, 900},
	{"choir.inbox", 0.02, 600},
	{"choir.capture_label", 0.02, 500},
	{"choir.trajectory", 0.015, 1200},
	{"choir.computer_setting", 0.005, 300},
	{"choir.media_state", 0.016, 400},
}

type churnGen struct {
	rng      *rand.Rand
	zipf     *rand.Zipf
	words    []string
	owner    string
	computer string
	now      time.Time
	runs     []string
	runLeft  []int
	trajs    []string
	channels []string
	seq      int64
	inboxSeq int64
	runObjs  []string // canonical ids of the active runs' run objects
	loops    []string
	loopSeq  map[string]int64
	loopPrev map[string]string
	enc      *zstd.Encoder
	zstdIn   int64
	zstdOut  int64
	zstdTime time.Duration
}

func newChurnGen(seed uint64) *churnGen {
	g := &churnGen{rng: rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15)), loopSeq: map[string]int64{}, loopPrev: map[string]string{}}
	for i := 0; i < 4000; i++ {
		n := 3 + g.rng.IntN(8)
		b := make([]byte, n)
		for j := range b {
			b[j] = byte('a' + g.rng.IntN(26))
		}
		g.words = append(g.words, string(b))
	}
	// Zipf word frequencies, like prose: per-record zstd lands near what
	// JSON bodies give, not the 1.2x of uniformly random words.
	g.zipf = rand.NewZipf(g.rng, 1.1, 2, uint64(len(g.words)-1))
	g.owner = g.b64(32)
	g.computer = "computer-" + g.hex(16)
	g.now = time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 24; i++ {
		g.newRun(i)
	}
	for i := 0; i < 200; i++ {
		g.trajs = append(g.trajs, "traj-"+g.hex(8))
		g.channels = append(g.channels, "chan-"+g.hex(8))
	}
	for i := 0; i < 24; i++ {
		g.loops = append(g.loops, "loop-"+g.hex(16))
	}
	g.enc, _ = zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedDefault), zstd.WithEncoderConcurrency(1))
	return g
}

func (g *churnGen) hex(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(g.rng.Uint32())
	}
	return hex.EncodeToString(b)
}

func (g *churnGen) b64(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(g.rng.Uint32())
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

func (g *churnGen) newRun(i int) {
	id := "run-" + g.hex(16)
	if i < len(g.runs) {
		g.runs[i], g.runLeft[i] = id, 20+g.rng.IntN(200)
		g.runObjs[i] = ""
		return
	}
	g.runs = append(g.runs, id)
	g.runLeft = append(g.runLeft, 20+g.rng.IntN(200))
	g.runObjs = append(g.runObjs, "")
}

func (g *churnGen) text(mean float64) string {
	// Lognormal around the kind's mean, a tail past the overflow threshold.
	size := int(mean * math.Exp(g.rng.NormFloat64()*0.6-0.18))
	var b strings.Builder
	for b.Len() < size {
		b.WriteString(g.words[g.zipf.Uint64()])
		b.WriteByte(' ')
	}
	return b.String()
}

func (g *churnGen) compress(raw []byte) []byte {
	if len(raw) <= 1024 {
		return raw
	}
	start := time.Now()
	out := g.enc.EncodeAll(raw, nil)
	g.zstdTime += time.Since(start)
	g.zstdIn += int64(len(raw))
	g.zstdOut += int64(len(out))
	return out
}

type churnObj struct {
	id, kind, metadata string
	body               []byte
	at                 time.Time
}

// object returns the next og_objects row in time order.
func (g *churnGen) object() churnObj {
	g.now = g.now.Add(time.Duration(300+g.rng.IntN(300)) * time.Millisecond)
	x := g.rng.Float64()
	k := churnKinds[len(churnKinds)-1]
	for _, c := range churnKinds {
		if x < c.share {
			k = c
			break
		}
		x -= c.share
	}
	slot := g.rng.IntN(len(g.runs))
	runID := g.runs[slot]
	g.runLeft[slot]--
	if g.runLeft[slot] <= 0 {
		g.newRun(slot)
	}
	meta := map[string]any{"run_id": runID, "seq": g.seq}
	g.seq++
	if g.rng.IntN(3) == 0 {
		meta["trajectory_id"] = g.trajs[g.rng.IntN(len(g.trajs))]
	}
	if g.rng.IntN(5) == 0 {
		meta["channel_id"] = g.channels[g.rng.IntN(len(g.channels))]
	}
	if k.kind == "choir.commitment_record" {
		g.inboxSeq++
		meta["inbox"] = "inbox:" + g.trajs[g.rng.IntN(20)]
		meta["sender_inbox"] = "inbox:" + g.trajs[g.rng.IntN(20)]
		meta["conversation_id"] = "conv-" + g.trajs[g.rng.IntN(len(g.trajs))]
		meta["inbox_seq"] = g.inboxSeq
	}
	if k.kind == "choir.event" {
		meta["event_kind"] = []string{"loop.progress", "desk.cell", "tool.result", "desk.egress"}[g.rng.IntN(4)]
	}
	mj, _ := json.Marshal(meta)
	body, _ := json.Marshal(map[string]any{"kind": k.kind, "text": g.text(k.bodyMean), "at": g.now.Format(time.RFC3339Nano)})
	o := churnObj{id: "obj:" + k.kind + ":" + g.owner + ":sha256-" + g.hex(32), kind: k.kind, metadata: string(mj), body: body, at: g.now}
	if k.kind == "choir.run" && g.runObjs[slot] == "" {
		g.runObjs[slot] = o.id
	}
	return o
}

// update returns an in-place update of an active run's run object.
func (g *churnGen) update() (churnObj, bool) {
	slot := g.rng.IntN(len(g.runs))
	id := g.runObjs[slot]
	if id == "" {
		return churnObj{}, false
	}
	g.now = g.now.Add(time.Duration(100+g.rng.IntN(200)) * time.Millisecond)
	mj, _ := json.Marshal(map[string]any{"run_id": g.runs[slot], "state": "running", "seq": g.seq})
	g.seq++
	body, _ := json.Marshal(map[string]any{"kind": "choir.run", "text": g.text(2800), "at": g.now.Format(time.RFC3339Nano)})
	return churnObj{id: id, kind: "choir.run", metadata: string(mj), body: body, at: g.now}, true
}

type churnMemory struct {
	entryID, loopID, parent, message string
	seq                              int64
	at                               time.Time
}

func (g *churnGen) memory(v7 bool) churnMemory {
	g.now = g.now.Add(time.Duration(200+g.rng.IntN(400)) * time.Millisecond)
	slot := g.rng.IntN(len(g.loops))
	loop := g.loops[slot]
	if g.loopSeq[loop] > int64(10+g.rng.IntN(80)) {
		delete(g.loopSeq, loop)
		delete(g.loopPrev, loop)
		loop = "loop-" + g.hex(16)
		g.loops[slot] = loop
	}
	g.loopSeq[loop]++
	var id string
	if v7 {
		// A UUIDv7 at the generator's clock: time-ordered like the live
		// writer's uuid.NewV7 at wall time.
		u, _ := uuid.NewV7()
		ms := uint64(g.now.UnixMilli())
		u[0], u[1], u[2], u[3], u[4], u[5] = byte(ms>>40), byte(ms>>32), byte(ms>>24), byte(ms>>16), byte(ms>>8), byte(ms)
		id = u.String()
	} else {
		id = uuid.NewString()
	}
	m := churnMemory{entryID: id, loopID: loop, parent: g.loopPrev[loop], seq: g.loopSeq[loop], at: g.now}
	g.loopPrev[loop] = id
	msg, _ := json.Marshal(map[string]any{"role": "assistant", "content": g.text(2200)})
	m.message = string(msg)
	return m
}

const churnUpsertSQL = `INSERT INTO og_objects
	(canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, metadata, created_at, updated_at, tombstone, superseded_by)
	VALUES (?, ?, ?, ?, '', ?, ?, ?, ?, ?, 0, '')
	ON CONFLICT (canonical_id) DO UPDATE SET content_hash = excluded.content_hash, body = excluded.body,
		metadata = excluded.metadata, updated_at = excluded.updated_at`

const churnMemorySQL = `INSERT INTO run_memory_entries (entry_id, loop_id, owner_id, agent_id, parent_entry_id, seq, kind, role, message_json, created_at)
	VALUES (?, ?, ?, 'agent', ?, ?, 'message', 'assistant', ?, ?)`

type churnWriter struct {
	g      *churnGen
	layout churnLayout
	og     *sql.Stmt
	mem    *sql.Stmt
}

func (w *churnWriter) putObject(tx *sql.Tx, o churnObj) error {
	body := o.body
	if w.layout.zstd && o.kind != "choir.commitment_record" {
		body = w.g.compress(body)
	}
	sum := sha256.Sum256(o.body)
	_, err := tx.Stmt(w.og).Exec(o.id, o.kind, w.g.owner, w.g.computer, hex.EncodeToString(sum[:]), body, o.metadata, o.at, o.at)
	return err
}

func (w *churnWriter) putMemory(tx *sql.Tx, m churnMemory) error {
	msg := any(m.message)
	if w.layout.zstd {
		if c := w.g.compress([]byte(m.message)); len(c) != len(m.message) {
			msg = c
		}
	}
	_, err := tx.Stmt(w.mem).Exec(m.entryID, m.loopID, w.g.owner, m.parent, m.seq, msg, m.at)
	return err
}

func openChurnDB(t *testing.T, path string, build bool) *sql.DB {
	t.Helper()
	dsn := "file:" + path + "?_pragma=journal_mode(WAL)&_pragma=synchronous(OFF)&_pragma=wal_autocheckpoint(0)&_pragma=cache_size(-262144)&_time_format=sqlite"
	if build {
		dsn = "file:" + path + "?_pragma=journal_mode(OFF)&_pragma=synchronous(OFF)&_pragma=cache_size(-1048576)&_pragma=auto_vacuum(INCREMENTAL)&_time_format=sqlite"
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	return db
}

func (w *churnWriter) prepare(t *testing.T, db *sql.DB) {
	t.Helper()
	var err error
	if w.og, err = db.Prepare(churnUpsertSQL); err != nil {
		t.Fatal(err)
	}
	if w.mem, err = db.Prepare(churnMemorySQL); err != nil {
		t.Fatal(err)
	}
}

// walDistinctPages reads the WAL's frame headers: the set of database pages
// the next checkpoint writes, and the frame count.
func walDistinctPages(t *testing.T, walPath string) (distinct, frames int) {
	t.Helper()
	f, err := os.Open(walPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	hdr := make([]byte, 32)
	if _, err := io.ReadFull(f, hdr); err != nil {
		return 0, 0
	}
	pageSize := int64(binary.BigEndian.Uint32(hdr[8:12]))
	salt1, salt2 := binary.BigEndian.Uint32(hdr[16:20]), binary.BigEndian.Uint32(hdr[20:24])
	seen := map[uint32]struct{}{}
	fh := make([]byte, 24)
	for off := int64(32); ; off += 24 + pageSize {
		if _, err := f.ReadAt(fh, off); err != nil {
			break
		}
		if binary.BigEndian.Uint32(fh[8:12]) != salt1 || binary.BigEndian.Uint32(fh[12:16]) != salt2 {
			break
		}
		seen[binary.BigEndian.Uint32(fh[0:4])] = struct{}{}
		frames++
	}
	return len(seen), frames
}

func chunkDigests(t *testing.T, path string) []string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	buf := make([]byte, 4<<20)
	var out []string
	for {
		n, err := io.ReadFull(f, buf)
		if n > 0 {
			sum := sha256.Sum256(buf[:n])
			out = append(out, hex.EncodeToString(sum[:8]))
		}
		if err != nil {
			break
		}
	}
	return out
}

func checkpointTruncate(t *testing.T, db *sql.DB) {
	t.Helper()
	var busy, log, done int
	if err := db.QueryRow(`PRAGMA wal_checkpoint(TRUNCATE)`).Scan(&busy, &log, &done); err != nil || busy != 0 {
		t.Fatalf("checkpoint: busy=%d err=%v", busy, err)
	}
}

type churnBatch struct {
	Writes        int     `json:"writes"`
	DistinctPages float64 `json:"distinct_pages_per_1000"`
	Frames        float64 `json:"wal_frames_per_1000"`
	MsPer1000     float64 `json:"ms_per_1000"`
}

type churnResult struct {
	Layout           string                `json:"layout"`
	OGRows           int                   `json:"og_rows"`
	MemoryRows       int                   `json:"memory_rows"`
	BuildSeconds     float64               `json:"build_seconds"`
	FileBytes        int64                 `json:"file_bytes"`
	Objects          map[string]int64      `json:"bytes_by_object,omitempty"`
	Batches          map[string]churnBatch `json:"pages_dirtied"`
	HourChunks       int                   `json:"hour_chunks"`
	HourChunksSame   int                   `json:"hour_chunks_same_position"`
	HourChunksHeld   int                   `json:"hour_chunks_digest_held"`
	HourPagesDirty   int                   `json:"hour_distinct_pages"`
	HourWALFrames    int                   `json:"hour_wal_frames"`
	ZstdInMB         float64               `json:"zstd_in_mb,omitempty"`
	ZstdRatio        float64               `json:"zstd_ratio,omitempty"`
	ZstdCPUmsPerMB   float64               `json:"zstd_cpu_ms_per_mb,omitempty"`
	UnzstdCPUmsPerMB float64               `json:"unzstd_cpu_ms_per_mb,omitempty"`
}

func TestChurnBench(t *testing.T) {
	rows, _ := strconv.Atoi(os.Getenv("CHOIR_CHURN_BENCH_OG_ROWS"))
	if rows <= 0 {
		t.Skip("CHOIR_CHURN_BENCH_OG_ROWS unset")
	}
	dir := os.Getenv("CHOIR_CHURN_BENCH_DIR")
	if dir == "" {
		dir = t.TempDir()
	}
	layouts := []churnLayout{{"current", false, false}, {"clustered", true, false}, {"clustered_zstd", true, true}}
	if only := os.Getenv("CHOIR_CHURN_BENCH_LAYOUTS"); only != "" {
		var keep []churnLayout
		for _, l := range layouts {
			if strings.Contains(","+only+",", ","+l.name+",") {
				keep = append(keep, l)
			}
		}
		layouts = keep
	}
	var results []churnResult
	for _, layout := range layouts {
		results = append(results, runChurnLayout(t, dir, layout, rows))
		raw, _ := json.MarshalIndent(results, "", "  ")
		t.Logf("%s", raw)
		if out := os.Getenv("CHOIR_CHURN_BENCH_OUT"); out != "" {
			_ = os.WriteFile(out, raw, 0o644)
		}
	}
}

func runChurnLayout(t *testing.T, dir string, layout churnLayout, ogRows int) churnResult {
	path := filepath.Join(dir, "churn-"+layout.name+".db")
	for _, s := range []string{"", "-wal", "-shm"} {
		_ = os.Remove(path + s)
	}
	memRows := ogRows * 114 / 334
	res := churnResult{Layout: layout.name, OGRows: ogRows, MemoryRows: memRows, Batches: map[string]churnBatch{}}
	g := newChurnGen(20261031)
	w := &churnWriter{g: g, layout: layout}

	// Build: time order, one transaction per 5000 rows, the memory rows
	// interleaved at their share.
	started := time.Now()
	db := openChurnDB(t, path, true)
	ogDDL := objectgraph.SchemaDDL
	if layout.clustered {
		ogDDL = clusteredOGDDL()
	}
	for _, ddl := range []string{storeMetaDDL, schemaDDL, ogDDL, objectgraph.ChoirEventUpdatedIndexDDL} {
		if _, err := db.Exec(ddl); err != nil {
			t.Fatalf("%s: ddl: %v", layout.name, err)
		}
	}
	w.prepare(t, db)
	tx, _ := db.Begin()
	memDone := 0
	for i := 0; i < ogRows; i++ {
		if err := w.putObject(tx, g.object()); err != nil {
			t.Fatal(err)
		}
		for memDone*ogRows < (i+1)*memRows {
			if err := w.putMemory(tx, g.memory(layout.clustered)); err != nil {
				t.Fatal(err)
			}
			memDone++
		}
		if i%5000 == 4999 {
			if err := tx.Commit(); err != nil {
				t.Fatal(err)
			}
			tx, _ = db.Begin()
			if i%500000 == 499999 {
				t.Logf("%s: %d rows in %s", layout.name, i+1, time.Since(started).Round(time.Second))
			}
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()
	res.BuildSeconds = time.Since(started).Seconds()

	db = openChurnDB(t, path, false)
	defer db.Close()
	w.prepare(t, db)
	if layout.zstd && g.zstdIn > 0 {
		res.ZstdInMB = float64(g.zstdIn) / 1e6
		res.ZstdRatio = float64(g.zstdIn) / float64(g.zstdOut)
		res.ZstdCPUmsPerMB = float64(g.zstdTime.Milliseconds()) / res.ZstdInMB
		res.UnzstdCPUmsPerMB = measureUnzstd(t, db)
	}
	res.Objects = churnObjectBytes(db)

	walPath := path + "-wal"
	measure := func(name string, writes, perTx int, step func(tx *sql.Tx) error) churnBatch {
		var b churnBatch
		const reps = 3
		for r := 0; r < reps; r++ {
			checkpointTruncate(t, db)
			began := time.Now()
			for done := 0; done < writes; {
				tx, err := db.Begin()
				if err != nil {
					t.Fatal(err)
				}
				for j := 0; j < perTx && done < writes; j++ {
					if err := step(tx); err != nil {
						t.Fatal(err)
					}
					done++
				}
				if err := tx.Commit(); err != nil {
					t.Fatal(err)
				}
			}
			el := time.Since(began)
			d, f := walDistinctPages(t, walPath)
			b.DistinctPages += float64(d) * 1000 / float64(writes) / reps
			b.Frames += float64(f) * 1000 / float64(writes) / reps
			b.MsPer1000 += float64(el.Milliseconds()) * 1000 / float64(writes) / reps
		}
		b.Writes = writes
		res.Batches[name] = b
		t.Logf("%s %s: %+v", layout.name, name, b)
		return b
	}
	insert := func(tx *sql.Tx) error { return w.putObject(tx, g.object()) }
	mixed := func(tx *sql.Tx) error {
		if g.rng.IntN(10) < 3 {
			if o, ok := g.update(); ok {
				return w.putObject(tx, o)
			}
		}
		return w.putObject(tx, g.object())
	}
	memory := func(tx *sql.Tx) error { return w.putMemory(tx, g.memory(layout.clustered)) }
	measure("og_insert_tx1", 1000, 1, insert)
	measure("og_insert_tx10", 1000, 10, insert)
	measure("og_mixed_tx10", 1000, 10, mixed)
	measure("memory_insert_tx1", 1000, 1, memory)

	// An hour of the paper's writes: ~10,000 og writes (70% inserts, 30%
	// in-place run updates) and ~5,200 run-memory entries, in transactions
	// of ten, interleaved; then the snapshot's 4 MiB chunks again.
	checkpointTruncate(t, db)
	before := chunkDigests(t, path)
	for i := 0; i < 1520; i++ {
		tx, err := db.Begin()
		if err != nil {
			t.Fatal(err)
		}
		// Seven og writes and three or four memory entries: 10,640 and
		// 5,320 an hour.
		n := 10
		if i%2 == 0 {
			n = 11
		}
		for j := 0; j < n; j++ {
			step := memory
			if j < 7 {
				step = mixed
			}
			if err := step(tx); err != nil {
				t.Fatal(err)
			}
		}
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
	}
	res.HourPagesDirty, res.HourWALFrames = walDistinctPages(t, walPath)
	checkpointTruncate(t, db)
	after := chunkDigests(t, path)
	held := map[string]bool{}
	for i, d := range before {
		held[d] = true
		if i < len(after) && after[i] == d {
			res.HourChunksSame++
		}
	}
	for _, d := range after {
		if held[d] {
			res.HourChunksHeld++
		}
	}
	res.HourChunks = len(after)
	if info, err := os.Stat(path); err == nil {
		res.FileBytes = info.Size()
	}
	t.Logf("%s hour: %d chunks, %d same position, %d held, %d distinct pages", layout.name, res.HourChunks, res.HourChunksSame, res.HourChunksHeld, res.HourPagesDirty)
	return res
}

// churnObjectBytes is the bytes of each table and index (dbstat), when the
// driver has it.
func churnObjectBytes(db *sql.DB) map[string]int64 {
	rows, err := db.Query(`SELECT name, SUM(pgsize) FROM dbstat GROUP BY name ORDER BY 2 DESC`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := map[string]int64{}
	for rows.Next() {
		var name string
		var n int64
		if rows.Scan(&name, &n) == nil {
			out[name] = n
		}
	}
	return out
}

func measureUnzstd(t *testing.T, db *sql.DB) float64 {
	dec, err := zstd.NewReader(nil, zstd.WithDecoderConcurrency(1))
	if err != nil {
		t.Fatal(err)
	}
	defer dec.Close()
	rows, err := db.Query(`SELECT body FROM og_objects WHERE og_id > (SELECT MAX(og_id) - 20000 FROM og_objects)`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out int64
	var took time.Duration
	for rows.Next() {
		var b []byte
		if err := rows.Scan(&b); err != nil {
			t.Fatal(err)
		}
		if len(b) < 4 || b[0] != 0x28 || b[1] != 0xb5 {
			continue
		}
		start := time.Now()
		plain, err := dec.DecodeAll(b, nil)
		took += time.Since(start)
		if err != nil {
			t.Fatal(err)
		}
		out += int64(len(plain))
	}
	if out == 0 {
		return 0
	}
	return float64(took.Microseconds()) / 1000 / (float64(out) / 1e6)
}

// BenchmarkChurnZstd is the CPU price of compressing one body over 1 KiB
// record at a time (zstd SpeedDefault, one encoder reused, no dictionary) and
// of reading it back, on the bench's Zipf bodies of the paper's sizes.
func BenchmarkChurnZstd(b *testing.B) {
	g := newChurnGen(7)
	var bodies [][]byte
	var total int64
	for len(bodies) < 2000 {
		o := g.object()
		if len(o.body) > 1024 {
			bodies = append(bodies, o.body)
			total += int64(len(o.body))
		}
	}
	level := zstd.SpeedDefault
	if os.Getenv("CHOIR_CHURN_ZSTD_FASTEST") != "" {
		level = zstd.SpeedFastest
	}
	enc, _ := zstd.NewWriter(nil, zstd.WithEncoderLevel(level), zstd.WithEncoderConcurrency(1))
	dec, _ := zstd.NewReader(nil, zstd.WithDecoderConcurrency(1))
	packed := make([][]byte, len(bodies))
	var out int64
	for i, body := range bodies {
		packed[i] = enc.EncodeAll(body, nil)
		out += int64(len(packed[i]))
	}
	b.Logf("bodies %d, %.1f MB, ratio %.2f", len(bodies), float64(total)/1e6, float64(total)/float64(out))
	b.Run("encode", func(b *testing.B) {
		b.SetBytes(total)
		var dst []byte
		for i := 0; i < b.N; i++ {
			for _, body := range bodies {
				dst = enc.EncodeAll(body, dst[:0])
			}
		}
	})
	b.Run("decode", func(b *testing.B) {
		b.SetBytes(total)
		var dst []byte
		for i := 0; i < b.N; i++ {
			for _, p := range packed {
				dst, _ = dec.DecodeAll(p, dst[:0])
			}
		}
	})
}
