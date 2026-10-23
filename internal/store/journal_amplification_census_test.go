package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dolthub/dolt/go/gen/fb/serial"
	"github.com/dolthub/dolt/go/store/nbs"
	"github.com/dolthub/dolt/go/store/prolly/tree"
	"github.com/google/uuid"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// TestJournalAmplificationCensus is the repeatable artifact of
// docs/problems/store-journal-write-amplification-2026-10-23.md. It measures
// how many bytes of Dolt journal one run-memory append writes, and what each
// factor contributes, on a local embedded Dolt store. It reads the journal
// file itself: every chunk record written during a measured window is
// decompressed and classified (prolly tree node by level, address map, blob,
// root value, working set, table, commit), so the bytes are attributed to a
// kind of chunk and not inferred from a total.
//
// Off by default (it seeds 100,000 rows several times). Pick sections with
//
//	CHOIR_JOURNAL_AMP=depth,index,rowsize,floor,batch,cell,drops,commit,tape,events,og \
//	  go test ./internal/store -run TestJournalAmplificationCensus -v -count=1 -timeout 60m
//
// CHOIR_JOURNAL_AMP_ROWS sets the seed row counts of the depth section
// (default 1000,10000,100000); every other section seeds 100000 rows
// (CHOIR_JOURNAL_AMP_SEED overrides). CHOIR_JOURNAL_AMP_N sets the appends per
// measurement (default 200). The numbers are the artifact; the only assertions
// are that a window wrote something and that the journal parsed.
func TestJournalAmplificationCensus(t *testing.T) {
	sections := os.Getenv("CHOIR_JOURNAL_AMP")
	if sections == "" {
		t.Skip("set CHOIR_JOURNAL_AMP=depth,index,rowsize,floor,batch,cell,drops,commit,tape,events,og to run")
	}
	want := map[string]bool{}
	for _, sec := range strings.Split(sections, ",") {
		want[strings.TrimSpace(sec)] = true
	}
	for _, sec := range []struct {
		name string
		fn   func(*testing.T)
	}{
		{"depth", ampDepth}, {"index", ampIndexes}, {"rowsize", ampRowSize}, {"floor", ampFloor},
		{"batch", ampBatch}, {"cell", ampCell}, {"drops", ampDrops}, {"commit", ampCommit}, {"tape", ampTape}, {"events", ampEvents}, {"og", ampOG},
	} {
		if want[sec.name] {
			t.Run(sec.name, sec.fn)
		}
	}
}

func ampEnvInt(name string, def int) int {
	if v := os.Getenv(name); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}

func ampN() int    { return ampEnvInt("CHOIR_JOURNAL_AMP_N", 200) }
func ampSeed() int { return ampEnvInt("CHOIR_JOURNAL_AMP_SEED", 100000) }

// ---- the meter ---------------------------------------------------------------

type ampWindow struct {
	off   int64
	stats map[string]*ampClassStat
	total int64 // bytes written in the window (journal size delta)
}

type ampClassStat struct {
	chunks   int
	rawBytes int64 // uncompressed chunk data
	recBytes int64 // journal record bytes (compressed payload + 32 bytes of framing)
}

type ampMeter struct {
	t    *testing.T
	path string
}

func newAmpMeter(t *testing.T, s *Store) *ampMeter {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(s.texturePath, "*", ".dolt", "noms", doltJournalFileID))
	if err != nil || len(matches) != 1 {
		t.Fatalf("journal file under %s: %v %v", s.texturePath, matches, err)
	}
	return &ampMeter{t: t, path: matches[0]}
}

func (m *ampMeter) size() int64 {
	info, err := os.Stat(m.path)
	if err != nil {
		m.t.Fatal(err)
	}
	return info.Size()
}

// mark returns the journal offset a window starts at. The previous statement
// has committed, so the offset sits on a record boundary.
func (m *ampMeter) mark() int64 { return m.size() }

const (
	ampChunkFraming = 32 // len, tags, kind, 20-byte address, crc
	ampRootRecord   = 40 // a root-hash record
)

// window parses every chunk record from off to the end of the journal.
func (m *ampMeter) window(off int64) *ampWindow {
	m.t.Helper()
	end := m.size()
	w := &ampWindow{off: off, stats: map[string]*ampClassStat{}, total: end - off}
	f, err := os.Open(m.path)
	if err != nil {
		m.t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.Seek(off, 0); err != nil {
		m.t.Fatal(err)
	}
	lr := &limitedReader{f: f, left: end - off}
	var seen int64
	err = nbs.VisitJournalReaderChunks(context.Background(), lr, off, func(c nbs.CompressedChunk) error {
		ch, err := c.ToChunk()
		if err != nil {
			return err
		}
		class := ampClass(ch.Data())
		st := w.stats[class]
		if st == nil {
			st = &ampClassStat{}
			w.stats[class] = st
		}
		st.chunks++
		st.rawBytes += int64(len(ch.Data()))
		rec := int64(len(c.FullCompressedChunk)) + ampChunkFraming
		st.recBytes += rec
		seen += rec
		return nil
	})
	if err != nil {
		m.t.Fatalf("parse journal window [%d,%d): %v", off, end, err)
	}
	roots := w.total - seen
	if roots < 0 {
		m.t.Fatalf("parsed %d bytes of chunk records in a %d byte window", seen, w.total)
	}
	w.stats["root-hash record"] = &ampClassStat{chunks: int(roots / ampRootRecord), recBytes: roots}
	return w
}

type limitedReader struct {
	f    *os.File
	left int64
}

func (l *limitedReader) Read(p []byte) (int, error) {
	if l.left <= 0 {
		return 0, io.EOF
	}
	if int64(len(p)) > l.left {
		p = p[:l.left]
	}
	n, err := l.f.Read(p)
	l.left -= int64(n)
	return n, err
}

func ampClass(d []byte) string {
	id := serial.GetFileID(d)
	switch id {
	case serial.ProllyTreeNodeFileID, serial.AddressMapFileID:
		nd, _, err := tree.NodeFromBytes(d)
		if err != nil {
			return id + " ?"
		}
		return fmt.Sprintf("%s L%d", id, nd.Level())
	case serial.BlobFileID:
		return "BLOB"
	case serial.RootValueFileID:
		return "RTVL root value"
	case serial.WorkingSetFileID:
		return "WRST working set"
	case serial.TableFileID:
		return "DTBL table"
	case serial.CommitFileID:
		return "DCMT commit"
	case serial.CommitClosureFileID:
		return "CMCL commit closure"
	case serial.TableSchemaFileID:
		return "DSCH schema"
	case "":
		return "other (no file id)"
	}
	return id
}

// perOp renders a window as KiB per operation with the chunk classes.
func (w *ampWindow) perOp(ops int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%.1f KiB/op", float64(w.total)/1024/float64(ops))
	classes := make([]string, 0, len(w.stats))
	for k := range w.stats {
		classes = append(classes, k)
	}
	sort.Slice(classes, func(i, j int) bool { return w.stats[classes[i]].recBytes > w.stats[classes[j]].recBytes })
	var chunks int
	for _, k := range classes {
		chunks += w.stats[k].chunks
	}
	fmt.Fprintf(&b, " in %.1f chunks/op;", float64(chunks)/float64(ops))
	for _, k := range classes {
		st := w.stats[k]
		raw := ""
		if st.rawBytes > 0 {
			raw = fmt.Sprintf(" raw %.1f", float64(st.rawBytes)/1024/float64(ops))
		}
		fmt.Fprintf(&b, " [%s: %.2f chunks, %.1f KiB%s]", k, float64(st.chunks)/float64(ops), float64(st.recBytes)/1024/float64(ops), raw)
	}
	return b.String()
}

// ---- seeding and appending ---------------------------------------------------

var ampWords = strings.Fields(`the run memory entry model tool call result agent desk cell inbox cursor commit
revision document source story outlet citation published channel owner message role assistant user system
planning reading writing summary checking verify search query result schema table index store journal
wake outbox schedule due paper edition section headline body quote reference evidence ledger fate success
failure retry timeout restart boot health stall write read connection engine mutex prolly chunk tree leaf`)

// ampText returns n bytes of word-salad text: compressible about two to three
// times by snappy, as prose and JSON are. A string of one repeated byte would
// compress to nothing and hide the cost of a large row.
func ampText(rng *rand.Rand, n int) string {
	if n <= 0 {
		return ""
	}
	var b strings.Builder
	for b.Len() < n {
		b.WriteString(ampWords[rng.Intn(len(ampWords))])
		b.WriteByte(' ')
	}
	return b.String()[:n]
}

// ampRM is one run-memory row.
type ampRM struct {
	id, loop, owner, agent, parent string
	seq                            int64
	kind, message, details         string
	at                             time.Time
}

const ampOwner = "owner-amp"

// ampRows generates rows the way a desk produces them: R concurrent runs, each
// appending in sequence, rows of a run chained by parent_entry_id, timestamps
// increasing, UUID entry ids. msgBytes is the size of message_json.
type ampGen struct {
	rng      *rand.Rand
	runs     int
	next     int
	lastID   map[int]string
	seq      map[int]int64
	base     time.Time
	msgBytes int
	prefix   string
}

func newAmpGen(prefix string, runs, msgBytes int) *ampGen {
	return &ampGen{rng: rand.New(rand.NewSource(int64(len(prefix)) + 7)), runs: runs, lastID: map[int]string{},
		seq: map[int]int64{}, base: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), msgBytes: msgBytes, prefix: prefix}
}

func (g *ampGen) row() ampRM {
	r := g.next % g.runs
	i := g.next
	g.next++
	g.seq[r]++
	id := uuid.NewString()
	row := ampRM{
		id: id, loop: fmt.Sprintf("run-%s-%d-%d", g.prefix, r, g.seq[r]/250), owner: ampOwner, agent: fmt.Sprintf("agent-%02d", r%8),
		parent: g.lastID[r], seq: g.seq[r], kind: "message", at: g.base.Add(time.Duration(i) * time.Second),
		details: `{}`,
	}
	if g.msgBytes > 0 {
		row.message = `{"role":"assistant","content":"` + ampText(g.rng, g.msgBytes) + `"}`
	} else {
		row.kind = "rlm_inbox_cursor"
		row.details = fmt.Sprintf(`{"channel_id":"ch-1","desk_agent_id":%q,"cursor":%d}`, row.agent, i)
	}
	g.lastID[r] = id
	return row
}

func (r ampRM) args() []any {
	return []any{r.id, r.loop, r.owner, r.agent, r.parent, r.seq, r.kind, "", r.message, "", "", 0, "", "", r.details, r.at}
}

const ampCols = `(entry_id, loop_id, owner_id, agent_id, parent_entry_id, seq, kind, role, message_json, summary,
	first_kept_entry_id, tokens_before, reason, model, details_json, created_at)`
const ampMarks = "(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)"

type ampExec interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func ampInsert(t *testing.T, ex ampExec, table string, rows []ampRM) {
	t.Helper()
	var vals []string
	var args []any
	for _, r := range rows {
		vals = append(vals, ampMarks)
		args = append(args, r.args()...)
	}
	if _, err := ex.ExecContext(context.Background(), `INSERT INTO `+table+` `+ampCols+` VALUES `+strings.Join(vals, ","), args...); err != nil {
		t.Fatal(err)
	}
}

// ampSeedTable inserts n rows in one transaction, 100 rows a statement.
func ampSeedTable(t *testing.T, s *Store, table string, g *ampGen, n int) {
	t.Helper()
	t0 := time.Now()
	tx, err := s.db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for done := 0; done < n; {
		k := min(100, n-done)
		rows := make([]ampRM, k)
		for i := range rows {
			rows[i] = g.row()
		}
		ampInsert(t, tx, table, rows)
		done += k
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	t.Logf("seeded %s with %d rows (message_json %d bytes) in %v", table, n, g.msgBytes, time.Since(t0).Round(time.Second))
}

func ampMustExec(t *testing.T, s *Store, q string, args ...any) {
	t.Helper()
	if _, err := s.db.ExecContext(context.Background(), q, args...); err != nil {
		t.Fatalf("%s: %v", q, err)
	}
}

// ampAppendEach appends n rows, each its own autocommit statement, and
// returns the window.
func ampAppendEach(t *testing.T, s *Store, m *ampMeter, table string, g *ampGen, n int) *ampWindow {
	t.Helper()
	off := m.mark()
	for i := 0; i < n; i++ {
		ampInsert(t, s.db, table, []ampRM{g.row()})
	}
	return m.window(off)
}

// ---- the production table, index by index -----------------------------------

var ampProdIndexes = []struct{ name, cols string }{
	{"idx_run_memory_entries_loop_seq", "loop_id, seq"},
	{"idx_run_memory_entries_owner_loop_seq", "owner_id, loop_id, seq"},
	{"idx_run_memory_entries_owner_agent_created", "owner_id, agent_id, created_at"},
	{"idx_run_memory_entries_parent", "parent_entry_id"},
	{RunMemoryKindIndexName, "owner_id, agent_id, kind, created_at"},
}

func ampHasIndex(t *testing.T, s *Store, table, name string) bool {
	t.Helper()
	var n int
	if err := s.db.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = ? AND index_name = ?`,
		table, name).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n > 0
}

func ampSetIndexes(t *testing.T, s *Store, table string, keep map[string]bool) {
	t.Helper()
	for _, ix := range ampProdIndexes {
		has := ampHasIndex(t, s, table, ix.name)
		switch {
		case keep[ix.name] && !has:
			ampMustExec(t, s, `CREATE INDEX `+ix.name+` ON `+table+`(`+ix.cols+`)`)
		case !keep[ix.name] && has:
			ampMustExec(t, s, `DROP INDEX `+ix.name+` ON `+table)
		}
	}
}

func ampAllIndexes() map[string]bool {
	m := map[string]bool{}
	for _, ix := range ampProdIndexes {
		m[ix.name] = true
	}
	return m
}

// ---- sections ----------------------------------------------------------------

// ampDepth: journal per append against the table's size (tree depth), with the
// production schema (primary key, UNIQUE(loop_id, seq) and five indexes).
func ampDepth(t *testing.T) {
	sizes := []int{1000, 10000, 100000}
	if v := os.Getenv("CHOIR_JOURNAL_AMP_ROWS"); v != "" {
		sizes = nil
		for _, f := range strings.Split(v, ",") {
			n, err := strconv.Atoi(strings.TrimSpace(f))
			if err != nil {
				t.Fatal(err)
			}
			sizes = append(sizes, n)
		}
	}
	for _, size := range sizes {
		t.Run(fmt.Sprintf("rows=%d", size), func(t *testing.T) {
			s := openTestStore(t)
			m := newAmpMeter(t, s)
			g := newAmpGen("d", 8, 1500)
			ampSeedTable(t, s, "run_memory_entries", g, size)
			n := ampN()
			w := ampAppendEach(t, s, m, "run_memory_entries", g, n)
			if w.total <= 0 {
				t.Fatal("empty window")
			}
			t.Logf("DEPTH rows=%d, 1.5 KB message, one autocommit INSERT each: %s", size, w.perOp(n))
		})
	}
}

// ampIndexes: journal per append as indexes are added one at a time to the
// primary key, and as each is removed from the full set.
func ampIndexes(t *testing.T) {
	s := openTestStore(t)
	m := newAmpMeter(t, s)
	g := newAmpGen("i", 8, 1500)
	seed := ampSeed()
	ampSeedTable(t, s, "run_memory_entries", g, seed)
	n := ampN()
	all := ampAllIndexes()

	// The UNIQUE(loop_id, seq) key stays: it is part of the table's definition
	// and is what the production insert path relies on.
	keep := map[string]bool{}
	ampSetIndexes(t, s, "run_memory_entries", keep)
	w := ampAppendEach(t, s, m, "run_memory_entries", g, n)
	t.Logf("INDEX seed=%d cumulative: primary key + UNIQUE(loop_id,seq) only: %s", seed, w.perOp(n))
	for _, ix := range ampProdIndexes {
		keep[ix.name] = true
		ampSetIndexes(t, s, "run_memory_entries", keep)
		w = ampAppendEach(t, s, m, "run_memory_entries", g, n)
		t.Logf("INDEX seed=%d cumulative: + %s (%s): %s", seed, ix.name, ix.cols, w.perOp(n))
	}
	for _, ix := range ampProdIndexes {
		keep := map[string]bool{}
		for k := range all {
			keep[k] = k != ix.name
		}
		ampSetIndexes(t, s, "run_memory_entries", keep)
		w = ampAppendEach(t, s, m, "run_memory_entries", g, n)
		t.Logf("INDEX seed=%d leave-one-out: the full set without %s: %s", seed, ix.name, w.perOp(n))
	}
	ampSetIndexes(t, s, "run_memory_entries", all)
	w = ampAppendEach(t, s, m, "run_memory_entries", g, n)
	t.Logf("INDEX seed=%d the full production set again: %s", seed, w.perOp(n))
}

// ampRowSize: the same append with message_json of different sizes, the table
// seeded with rows of the same size. Dolt stores a text column inline while
// the whole tuple is at most 2 KiB (val.DefaultTupleLengthTarget) and moves the
// largest values out of the row into content-addressed blobs above that.
func ampRowSize(t *testing.T) {
	seed := ampEnvInt("CHOIR_JOURNAL_AMP_SEED", 50000)
	for _, size := range []int{0, 500, 1500, 3000, 8000, 64000} {
		size := size
		t.Run(fmt.Sprintf("message=%d", size), func(t *testing.T) {
			n := seed
			if size >= 8000 {
				n = min(seed, 20000) // 64 KB x 20k is a 1.3 GB seed; the leaf is out of band either way
			}
			s := openTestStore(t)
			m := newAmpMeter(t, s)
			g := newAmpGen("r", 8, size)
			ampSeedTable(t, s, "run_memory_entries", g, n)
			cnt := ampN()
			w := ampAppendEach(t, s, m, "run_memory_entries", g, cnt)
			t.Logf("ROWSIZE seed=%d message_json=%d bytes (row %d bytes): %s", n, size, size+260, w.perOp(cnt))
		})
	}
}

// ampFloor: what a transaction costs with no row to speak of: a one-column
// insert into an empty table, in the same store (so the root value lists the
// same tables), then with a DOLT_COMMIT after each.
func ampFloor(t *testing.T) {
	s := openTestStore(t)
	m := newAmpMeter(t, s)
	n := ampN()
	ampMustExec(t, s, `CREATE TABLE amp_floor (k BIGINT NOT NULL PRIMARY KEY, v BIGINT NOT NULL DEFAULT 0)`)
	off := m.mark()
	for i := 1; i <= n; i++ {
		ampMustExec(t, s, `INSERT INTO amp_floor (k) VALUES (?)`, i)
	}
	t.Logf("FLOOR amp_floor, one BIGINT row per autocommit INSERT, sequential key, table of up to %d rows: %s", n, m.window(off).perOp(n))

	off = m.mark()
	for i := n + 1; i <= 2*n; i++ {
		ampMustExec(t, s, `INSERT INTO amp_floor (k) VALUES (?)`, i)
		ampMustExec(t, s, `CALL DOLT_COMMIT('-Am', 'amp floor')`)
	}
	t.Logf("FLOOR the same with a CALL DOLT_COMMIT('-Am') after each insert: %s", m.window(off).perOp(n))

	var tables int
	if err := s.db.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE()`).Scan(&tables); err != nil {
		t.Fatal(err)
	}
	t.Logf("FLOOR the store has %d tables in its root value", tables)
}

// ampBatch: the same 200 rows written five ways to the production table.
func ampBatch(t *testing.T) {
	s := openTestStore(t)
	m := newAmpMeter(t, s)
	g := newAmpGen("b", 8, 1500)
	seed := ampSeed()
	ampSeedTable(t, s, "run_memory_entries", g, seed)
	n := ampN()
	ctx := context.Background()

	rows := func(k int) []ampRM {
		out := make([]ampRM, k)
		for i := range out {
			out[i] = g.row()
		}
		return out
	}
	report := func(label string, off int64) {
		t.Logf("BATCH seed=%d %s: %s", seed, label, m.window(off).perOp(n))
	}

	off := m.mark()
	for _, r := range rows(n) {
		ampInsert(t, s.db, "run_memory_entries", []ampRM{r})
	}
	report("one row per statement, one statement per transaction (today)", off)

	off = m.mark()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rows(n) {
		ampInsert(t, tx, "run_memory_entries", []ampRM{r})
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	report(fmt.Sprintf("%d single-row statements in ONE transaction", n), off)

	for _, k := range []int{2, 4, 8, 16, 64, n} {
		off = m.mark()
		for done := 0; done < n; done += k {
			ampInsert(t, s.db, "run_memory_entries", rows(min(k, n-done)))
		}
		report(fmt.Sprintf("one multi-row INSERT of %d rows per transaction", k), off)
	}

	off = m.mark()
	tx, err = s.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	for done := 0; done < n; done += 10 {
		ampInsert(t, tx, "run_memory_entries", rows(min(10, n-done)))
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	report("one transaction of 10-row statements (all rows in one transaction)", off)

	// Ordered keys: the same rows with a time-ordered entry id. The primary
	// key and the parent index then append at the tail instead of landing in
	// a random leaf each.
	s2 := openTestStore(t)
	m2 := newAmpMeter(t, s2)
	g2 := newAmpGen("o", 8, 1500)
	ampSeedTable(t, s2, "run_memory_entries", g2, seed)
	ordered := func(k int) []ampRM {
		out := make([]ampRM, k)
		for i := range out {
			out[i] = g2.row()
			out[i].id = fmt.Sprintf("e-%020d", g2.next)
		}
		return out
	}
	// The seed used UUID ids; the appended rows are time-ordered ids, so they
	// land at the tail of the primary key (beyond the 'e-' prefix? 'e' sorts
	// after the hex digits of a UUID except 'f'; the point is one tail).
	off = m2.mark()
	for done := 0; done < n; done++ {
		ampInsert(t, s2.db, "run_memory_entries", ordered(1))
	}
	t.Logf("BATCH seed=%d ordered entry id, one row per transaction: %s", seed, m2.window(off).perOp(n))
	off = m2.mark()
	for done := 0; done < n; done += 16 {
		ampInsert(t, s2.db, "run_memory_entries", ordered(min(16, n-done)))
	}
	t.Logf("BATCH seed=%d ordered entry id, 16-row statements: %s", seed, m2.window(off).perOp(n))
}

// ampCommit: the cost of a DOLT_COMMIT, alone and per row at the production
// cadence (one commit per 100 finalized tape events, store.go doltCheckpointEvery).
func ampCommit(t *testing.T) {
	s := openTestStore(t)
	m := newAmpMeter(t, s)
	g := newAmpGen("c", 8, 1500)
	ampSeedTable(t, s, "run_memory_entries", g, ampSeed())
	ampMustExec(t, s, `CALL DOLT_COMMIT('-Am', 'seed')`)
	n := ampN()
	off := m.mark()
	for i := 0; i < n; i++ {
		ampInsert(t, s.db, "run_memory_entries", []ampRM{g.row()})
	}
	base := m.window(off)
	off = m.mark()
	ampMustExec(t, s, `CALL DOLT_COMMIT('-Am', 'one commit after the window')`)
	one := m.window(off)
	t.Logf("COMMIT %d appends, no commit: %s", n, base.perOp(n))
	t.Logf("COMMIT one CALL DOLT_COMMIT('-Am') after them: %.1f KiB in total (%s)", float64(one.total)/1024, one.perOp(1))
	for _, every := range []int{1, 10, 100} {
		off = m.mark()
		for i := 0; i < n; i++ {
			ampInsert(t, s.db, "run_memory_entries", []ampRM{g.row()})
			if (i+1)%every == 0 {
				ampMustExec(t, s, `CALL DOLT_COMMIT('-Am', 'cadence')`)
			}
		}
		t.Logf("COMMIT a DOLT_COMMIT every %d appends: %s", every, m.window(off).perOp(n))
	}
}

// ampTape: the production path. A tape-bound store appends run-memory entries
// as projection_batch_recorded events (dry run, prepare, finalize), one op each,
// then k ops per event.
func ampTape(t *testing.T) {
	h := newTapeHarness(t)
	s := h.store
	m := newAmpMeter(t, s)
	g := newAmpGen("t", 8, 1500)
	ampSeedTable(t, s, "run_memory_entries", g, ampSeed())
	ctx := context.Background()
	n := ampN()

	mk := func(i int) types.RunMemoryEntry {
		r := g.row()
		return types.RunMemoryEntry{RunID: r.loop, OwnerID: r.owner, AgentID: r.agent, Kind: "message", Seq: r.seq, ParentEntryID: r.parent,
			EntryID: r.id, Message: []byte(r.message), CreatedAt: r.at, Details: map[string]any{}}
	}
	off := m.mark()
	for i := 0; i < n; i++ {
		if _, err := s.AppendRunMemoryEntry(ctx, mk(i)); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("TAPE AppendRunMemoryEntry through the tape (1 op per event): %s", m.window(off).perOp(n))

	for _, k := range []int{4, 16, 64} {
		off = m.mark()
		for done := 0; done < n; done += k {
			var ops []computerevent.ProjectionOp
			for j := 0; j < k && done+j < n; j++ {
				e := mk(done + j)
				body, err := marshalRunMemoryProjection(e)
				if err != nil {
					t.Fatal(err)
				}
				ops = append(ops, computerevent.ProjectionOp{Kind: computerevent.ProjectionOpRunMemoryEntry, CanonicalID: e.EntryID, Body: body})
			}
			if err := s.projectionTape.appendOps(computerevent.WithBarrierExemption(ctx), ops); err != nil {
				t.Fatal(err)
			}
		}
		t.Logf("TAPE %d run-memory ops per tape event (one INSERT statement each): %s", k, m.window(off).perOp(n))
	}
}

func marshalRunMemoryProjection(e types.RunMemoryEntry) ([]byte, error) {
	return json.Marshal(computerevent.RunMemoryEntryProjection{
		EntryID: e.EntryID, RunID: e.RunID, OwnerID: e.OwnerID, AgentID: e.AgentID, ParentEntryID: e.ParentEntryID,
		Seq: e.Seq, Kind: string(e.Kind), MessageJSON: string(e.Message), DetailsJSON: "{}",
		CreatedAt: e.CreatedAt.UTC().Format(time.RFC3339Nano),
	})
}

// ampCell: batches whose rows all belong to one run, the shape of a desk
// cell's end (assistant message, tool result, cursor, fate), against the
// interleaved batches of ampBatch. The same-run rows are adjacent in
// UNIQUE(loop_id, seq) and the parent chain; the created_at trees take every
// row at the tail either way. One run at a time, 100,000 rows seeded.
func ampCell(t *testing.T) {
	s := openTestStore(t)
	m := newAmpMeter(t, s)
	seed := ampSeed()
	ampSeedTable(t, s, "run_memory_entries", newAmpGen("s", 8, 1500), seed)
	n := ampN()
	for _, k := range []int{1, 2, 4, 8, 16} {
		g := newAmpGen(fmt.Sprintf("cell%d", k), 1, 1500)
		off := m.mark()
		for done := 0; done < n; done += k {
			rows := make([]ampRM, min(k, n-done))
			for i := range rows {
				rows[i] = g.row()
			}
			ampInsert(t, s.db, "run_memory_entries", rows)
		}
		t.Logf("CELL seed=%d %d rows of ONE run per multi-row INSERT: %s", seed, k, m.window(off).perOp(n))
	}
}

// ampDrops: the full set, then indexes no reader needs removed in cumulative
// order. The witness note is in the record: dropping the parent index changes
// SHOW COLUMNS for parent_entry_id (MUL to empty).
func ampDrops(t *testing.T) {
	s := openTestStore(t)
	m := newAmpMeter(t, s)
	g := newAmpGen("x", 8, 1500)
	seed := ampSeed()
	ampSeedTable(t, s, "run_memory_entries", g, seed)
	n := ampN()
	keep := ampAllIndexes()
	w := ampAppendEach(t, s, m, "run_memory_entries", g, n)
	t.Logf("DROPS seed=%d full production set (7 trees): %s", seed, w.perOp(n))
	for _, step := range []string{"idx_run_memory_entries_loop_seq", "idx_run_memory_entries_owner_loop_seq", "idx_run_memory_entries_parent", "idx_run_memory_entries_owner_agent_created"} {
		delete(keep, step)
		ampSetIndexes(t, s, "run_memory_entries", keep)
		w = ampAppendEach(t, s, m, "run_memory_entries", g, n)
		t.Logf("DROPS seed=%d cumulative, without ...%s (%d trees left): %s", seed, strings.TrimPrefix(step, "idx_run_memory_entries"), 2+len(keep), w.perOp(n))
		if step == "idx_run_memory_entries_parent" {
			// The two levers together: the four-tree table, rows of one run
			// written four to a statement (a cell's end).
			for _, k := range []int{4, 16} {
				cg := newAmpGen(fmt.Sprintf("dc%d", k), 1, 1500)
				off := m.mark()
				for done := 0; done < n; done += k {
					rows := make([]ampRM, min(k, n-done))
					for i := range rows {
						rows[i] = cg.row()
					}
					ampInsert(t, s.db, "run_memory_entries", rows)
				}
				t.Logf("DROPS seed=%d four trees AND %d rows of one run per multi-row INSERT: %s", seed, k, m.window(off).perOp(n))
			}
		}
	}
}

// ampEvents: the other side of "fewer or merged tables". One trace line
// (a desk.cell choir.event, 300 bytes of payload) appended through the tape,
// as an og_objects row (13 trees, today's route on the paper) and as a
// choir_events row (the F-c table: primary key plus three indexes; the writer
// that stamps the op is deploy 2 and is not built, so the stamp is put on the
// op by hand, as event_table_test.go does). Both tables are seeded with
// CHOIR_JOURNAL_AMP_SEED events (default 100,000) by direct projection.
func ampEvents(t *testing.T) {
	h := newTapeHarness(t)
	s := h.store
	m := newAmpMeter(t, s)
	ctx := context.Background()
	seed := ampSeed()
	n := ampN()
	rng := rand.New(rand.NewSource(11))
	mk := func(i int) *types.EventRecord {
		run := fmt.Sprintf("run-ev-%d", i%8)
		return &types.EventRecord{
			EventID: fmt.Sprintf("ev-%09d", i), RunID: run, OwnerID: ampOwner, TrajectoryID: "traj-" + run, Seq: int64(i/8 + 1), StreamSeq: int64(i + 1),
			Timestamp: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(i) * time.Second), Kind: types.EventDeskCell,
			Payload: []byte(`{"cell":"` + ampText(rng, 300) + `"}`),
		}
	}
	op := func(i int, stamped bool) computerevent.ProjectionOp { return eventTableOp(t, mk(i), stamped) }
	seedTable := func(stamped bool, base int) {
		pos := uint64(1_000_000)
		if stamped {
			pos = 2_000_000
		}
		for done := 0; done < seed; done += 100 {
			var ops []computerevent.ProjectionOp
			for j := done; j < min(done+100, seed); j++ {
				ops = append(ops, op(base+j, stamped))
			}
			projectAt(t, s, pos+uint64(done), ops...)
		}
	}
	t0 := time.Now()
	seedTable(false, 0)
	seedTable(true, 10_000_000)
	t.Logf("seeded og_objects and choir_events with %d events each in %v", seed, time.Since(t0).Round(time.Second))
	for _, c := range []struct {
		label   string
		stamped bool
		base    int
	}{{"og_objects (13 trees)", false, 20_000_000}, {"choir_events (4 trees)", true, 30_000_000}} {
		off := m.mark()
		for i := 0; i < n; i++ {
			if err := s.projectionTape.appendOps(computerevent.WithBarrierExemption(ctx), []computerevent.ProjectionOp{op(c.base+i, c.stamped)}); err != nil {
				t.Fatal(err)
			}
		}
		t.Logf("EVENTS seed=%d one desk.cell trace line per tape event into %s: %s", seed, c.label, m.window(off).perOp(n))
	}
	// The same lines, k to a tape event (each its own INSERT statement).
	for _, k := range []int{6, 24} {
		for _, c := range []struct {
			label   string
			stamped bool
			base    int
		}{{"og_objects", false, 40_000_000}, {"choir_events", true, 50_000_000}} {
			off := m.mark()
			for done := 0; done < n; done += k {
				var ops []computerevent.ProjectionOp
				for j := done; j < min(done+k, n); j++ {
					ops = append(ops, op(c.base+k*1000000+j, c.stamped))
				}
				if err := s.projectionTape.appendOps(computerevent.WithBarrierExemption(ctx), ops); err != nil {
					t.Fatal(err)
				}
			}
			t.Logf("EVENTS seed=%d %d trace lines per tape event into %s: %s", seed, k, c.label, m.window(off).perOp(n))
		}
	}
}

// ampOG: what each og_objects index costs a trace-line append. The table is
// seeded with CHOIR_JOURNAL_AMP_SEED choir.event rows by direct projection,
// then 200 events are appended through the tape one op each; after that each
// secondary index is dropped in turn (the scratch store only) and the same
// 200 appends are measured again, so the marginal cost of a tree is the
// difference of two consecutive lines.
func ampOG(t *testing.T) {
	h := newTapeHarness(t)
	s := h.store
	m := newAmpMeter(t, s)
	ctx := context.Background()
	seed := ampSeed()
	n := ampN()
	rng := rand.New(rand.NewSource(13))
	mk := func(i int) computerevent.ProjectionOp {
		run := fmt.Sprintf("run-og-%d", i%8)
		return eventTableOp(t, &types.EventRecord{
			EventID: fmt.Sprintf("og-%09d", i), RunID: run, OwnerID: ampOwner, TrajectoryID: "traj-" + run, Seq: int64(i/8 + 1), StreamSeq: int64(i + 1),
			Timestamp: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(i) * time.Second), Kind: types.EventDeskCell,
			Payload: []byte(`{"cell":"` + ampText(rng, 300) + `"}`),
		}, false)
	}
	for done := 0; done < seed; done += 100 {
		var ops []computerevent.ProjectionOp
		for j := done; j < min(done+100, seed); j++ {
			ops = append(ops, mk(j))
		}
		projectAt(t, s, uint64(1_000_000+done), ops...)
	}
	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT index_name FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'og_objects' AND index_name <> 'PRIMARY' ORDER BY index_name`)
	if err != nil {
		t.Fatal(err)
	}
	var idx []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		idx = append(idx, name)
	}
	_ = rows.Close()
	next := 100_000_000
	run := func(label string) {
		off := m.mark()
		for i := 0; i < n; i++ {
			next++
			if err := s.projectionTape.appendOps(computerevent.WithBarrierExemption(ctx), []computerevent.ProjectionOp{mk(next)}); err != nil {
				t.Fatal(err)
			}
		}
		t.Logf("OG seed=%d %s: %s", seed, label, m.window(off).perOp(n))
	}
	run(fmt.Sprintf("all %d secondary indexes %v", len(idx), idx))
	for i, name := range idx {
		ampMustExec(t, s, "DROP INDEX `"+name+"` ON og_objects")
		run(fmt.Sprintf("without %s (%d trees left)", name, len(idx)-i))
	}
}
