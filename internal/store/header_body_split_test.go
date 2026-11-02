package store

// The header/body split migration (header_body_split.go). Ways it can fail,
// written before the tests:
//
//   - S1: a row, a body or a derived value is lost or changed by the copy
//     (a byte that differs, a type that changes, a NULL body that becomes an
//     empty one, a phase or claim computed differently);
//   - S2: a table other than the two split ones is not carried, or carried
//     without its indexes, its sqlite_sequence or its derived-key rows;
//   - S3: a crash at any step leaves a store the next boot cannot open, or
//     trusts a partial staging file;
//   - S4: a staged copy is swapped in over an old file that another binary
//     changed after the copy was made;
//   - S5: a crash between the swap's renames leaves no state file, or the
//     next boot makes an empty store instead of finishing or undoing;
//   - S6: a disk too small is found after the store was touched (a floor
//     raised, a file renamed), so the old binary cannot come back cleanly;
//   - S7: a migration that keeps failing is retried forever (the host kills
//     the guest each time) instead of refusing with a witness;
//   - S8: Reopen (a restore or base install of an old snapshot) opens an
//     old-layout file without migrating it;
//   - S9: a compact copy staged by the old binary is swapped in;
//   - S10: the rollback copy is never removed, or compaction stacks a third
//     copy beside it.

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/dbx"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/storeschema"
)

var errSplitTestCrash = errors.New("split test: simulated crash")

// splitTestDisk makes the preflight read free bytes of disk.
func splitTestDisk(t *testing.T, free int64) {
	t.Helper()
	prev := compactFreeBytes
	compactFreeBytes = func(string) (int64, error) { return free, nil }
	t.Cleanup(func() { compactFreeBytes = prev })
}

func splitTestHook(t *testing.T, hook func(step string) error) {
	t.Helper()
	prev := testHookHeaderBodySplitStep
	testHookHeaderBodySplitStep = hook
	t.Cleanup(func() { testHookHeaderBodySplitStep = prev })
}

func crashAt(step string) func(string) error {
	return func(s string) error {
		if s == step {
			return errSplitTestCrash
		}
		return nil
	}
}

type splitFixture struct {
	dbPath, statePath string
	objects, events   int
	bodyBytes         int64
}

// newSplitFixture builds a pre-split store from the frozen DDL: og_objects
// with NULL, empty, JSON, non-JSON and large bodies (commitment records with
// and without a directive, events with payload and metadata phases),
// choir_events, og_key_index, og_edges, an AUTOINCREMENT table and a
// projection head.
func newSplitFixture(t *testing.T, objects, events int) splitFixture {
	t.Helper()
	dir := t.TempDir()
	f := splitFixture{dbPath: filepath.Join(dir, "runtime.db"), objects: objects, events: events}
	f.statePath = StateDBPath(f.dbPath)
	schema, err := os.ReadFile(filepath.Join("testdata", "presplit", "schema.sql"))
	if err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", dbx.SQLiteDSN(f.statePath, dbx.SQLiteOptions{}, false))
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	defer db.Close()
	if _, err := db.Exec(string(schema)); err != nil {
		t.Fatalf("frozen schema: %v", err)
	}
	at := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	mustExec := func(q string, args ...any) {
		t.Helper()
		if _, err := db.Exec(q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	mustExec(`INSERT INTO store_meta (key, value) VALUES ('schema_version', '1')`)
	mustExec(`INSERT INTO computer_event_projection_heads (computer_id, sequence, canonical_event_head, desired_event_head, effective_event_head, desired_state_commitment, effective_state_commitment, reducer_version, credential_revocation_epoch, updated_at)
		VALUES ('computer-split', 4242, 'head-4242', 'head-4242', 'head-4242', 'c', 'c', 1, 0, ?)`, at)
	big := []byte(strings.Repeat(`{"payload":{"phase":"big"},"x":"`, 1) + strings.Repeat("z", 200<<10) + `"}`)
	var jsonb []byte
	if err := db.QueryRow(`SELECT jsonb('{"directive":"x","payload":{"phase":"jb"}}')`).Scan(&jsonb); err != nil {
		t.Fatal(err)
	}
	bodies := []any{nil, []byte{}, []byte("not json"), []byte(`{"directive":""}`), []byte(`{"directive":"go"}`),
		[]byte(`{"payload":{"phase":"started"}}`), []byte(`{"payload":{"phase":7}}`), jsonb, "text body", big}
	kinds := []string{"choir.commitment_record", "choir.event", "choir.run", "choir.document"}
	metas := []string{`{}`, `{"phase":"meta"}`, `{"kind":"tool.result","run_id":"r1"}`, `{"kind":"llm.call","trajectory_id":"t1","inbox":"in-1","inbox_seq":3}`}
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < objects; i++ {
		body := bodies[i%len(bodies)]
		if i%len(bodies) == len(bodies)-1 && i%(len(bodies)*7) != len(bodies)-1 {
			body = bodies[i%3+3] // keep the large bodies to about one in seventy
		}
		switch b := body.(type) {
		case []byte:
			f.bodyBytes += int64(len(b))
		case string:
			f.bodyBytes += int64(len(b))
		}
		id := fmt.Sprintf("obj-%06d", (i*7919)%1000003) // canonical order differs from rowid order
		if _, err := tx.Exec(`INSERT INTO og_objects (canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, metadata, created_at, updated_at, tombstone, superseded_by)
			VALUES (?, ?, 'owner', 'computer-split', 'v1', ?, ?, ?, ?, ?, ?, '')`,
			id, kinds[i%len(kinds)], fmt.Sprintf("h%d", i), body, metas[i%len(metas)], at, at.Add(time.Duration(i)*time.Second), i%11 == 0); err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(`INSERT INTO og_key_index (canonical_id, key_name, key_value, owner_id, updated_at, content_hash, derive_ok) VALUES (?, 'addressee', ?, 'owner', ?, ?, ?)`,
			id, fmt.Sprintf("a%d", i%5), at, fmt.Sprintf("h%d", i), i%13 != 0); err != nil {
			t.Fatal(err)
		}
	}
	// Some deletes: sparse rowids.
	if _, err := tx.Exec(`DELETE FROM og_objects WHERE rowid % 17 = 0`); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < events; i++ {
		body := bodies[(i%(len(bodies)-1))+1] // NOT NULL in choir_events
		if b, ok := body.([]byte); ok {
			f.bodyBytes += int64(len(b))
		}
		if _, err := tx.Exec(`INSERT INTO choir_events (tape_seq, op_index, canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, metadata, created_at, updated_at, tombstone, superseded_by, run_id, seq, stream_seq, trajectory_id, event_kind)
			VALUES (?, ?, ?, 'choir.event', 'owner', 'computer-split', 'v1', ?, ?, ?, ?, ?, 0, '', ?, ?, ?, ?, ?)`,
			100+i/3, i%3, fmt.Sprintf("ev-%06d", i), fmt.Sprintf("eh%d", i), body, metas[i%len(metas)], at, at.Add(time.Duration(i)*time.Second),
			fmt.Sprintf("run-%d", i%9), i, i, fmt.Sprintf("traj-%d", i%4), []string{"tool.result", "llm.call", ""}[i%3]); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 50; i++ {
		if _, err := tx.Exec(`INSERT INTO og_edges (edge_id, from_id, to_id, kind, metadata, created_at) VALUES (?, ?, ?, 'cites', '{}', ?)`,
			fmt.Sprintf("edge-%d", i), fmt.Sprintf("obj-%06d", i), fmt.Sprintf("obj-%06d", i+1), at); err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(`INSERT INTO wire_publish_debounce_entries (owner_id, computer_id, agent_id, content, created_at) VALUES ('o', 'c', 'a', ?, ?)`, fmt.Sprint(i), at); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := tx.Exec(`DELETE FROM wire_publish_debounce_entries WHERE entry_id > 40`); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	return f
}

// splitDigest hashes the rows of query, each value through quote() so the
// storage class is part of the digest.
func splitDigest(t *testing.T, path, query string) (string, int) {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	rows, err := db.Query(query)
	if err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	defer rows.Close()
	cols, _ := rows.Columns()
	h := sha256.New()
	n := 0
	vals := make([]sql.RawBytes, len(cols))
	ptrs := make([]any, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	for rows.Next() {
		if err := rows.Scan(ptrs...); err != nil {
			t.Fatal(err)
		}
		for _, v := range vals {
			_, _ = io.WriteString(h, string(v)+"\x1f")
		}
		_, _ = io.WriteString(h, "\x1e")
		n++
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(h.Sum(nil)), n
}

func quoted(cols ...string) string {
	out := make([]string, len(cols))
	for i, c := range cols {
		out[i] = "quote(" + c + ")"
	}
	return strings.Join(out, ", ")
}

var (
	oldObjectCols = []string{"canonical_id", "object_kind", "owner_id", "computer_id", "version_id", "content_hash", "body", "metadata", "created_at", "updated_at", "tombstone", "superseded_by"}
	oldEventCols  = []string{"tape_seq", "op_index", "canonical_id", "object_kind", "owner_id", "computer_id", "version_id", "content_hash", "body", "metadata", "created_at", "updated_at", "tombstone", "superseded_by", "run_id", "seq", "stream_seq", "trajectory_id", "event_kind"}
)

func withPrefix(p string, cols []string) []string {
	out := make([]string, len(cols))
	for i, c := range cols {
		if c == "body" {
			out[i] = "b.body"
		} else {
			out[i] = p + c
		}
	}
	return out
}

// splitLogicalDigests are the old file's og_objects and choir_events and the
// new file's joined equivalents (the witness logical tables' shape), plus the
// tables the migration copies verbatim.
type splitDigests map[string]string

func oldSplitDigests(t *testing.T, path string) splitDigests {
	t.Helper()
	out := splitDigests{}
	out["og_objects"], _ = splitDigest(t, path, `SELECT `+quoted(oldObjectCols...)+` FROM og_objects ORDER BY canonical_id`)
	out["choir_events"], _ = splitDigest(t, path, `SELECT `+quoted(oldEventCols...)+` FROM choir_events ORDER BY tape_seq, op_index`)
	claim := `CASE WHEN object_kind = 'choir.commitment_record' THEN og_rec_claim END`
	phase := `CASE WHEN object_kind = 'choir.event' THEN ` + objectgraph.ChoirEventPhaseSQLFor("body", "metadata") + ` END`
	out["derived"], _ = splitDigest(t, path, `SELECT quote(canonical_id), quote(`+claim+`), quote(`+phase+`) FROM og_objects ORDER BY canonical_id`)
	out["event_phase"], _ = splitDigest(t, path, `SELECT quote(`+objectgraph.ChoirEventPhaseSQLFor("body", "metadata")+`) FROM choir_events ORDER BY tape_seq, op_index`)
	for table, order := range verbatimSplitTables {
		out[table], _ = splitDigest(t, path, `SELECT * FROM `+table+` ORDER BY `+order)
	}
	return out
}

var verbatimSplitTables = map[string]string{
	"og_key_index":                    "canonical_id, key_name",
	"og_edges":                        "edge_id",
	"wire_publish_debounce_entries":   "entry_id",
	"sqlite_sequence":                 "name",
	"computer_event_projection_heads": "computer_id",
}

func newSplitDigests(t *testing.T, path string) splitDigests {
	t.Helper()
	out := splitDigests{}
	out["og_objects"], _ = splitDigest(t, path, `SELECT `+quoted(withPrefix("h.", oldObjectCols)...)+` FROM og_objects h LEFT JOIN og_bodies b ON b.obj_id = h.obj_id ORDER BY h.canonical_id`)
	out["choir_events"], _ = splitDigest(t, path, `SELECT `+quoted(withPrefix("h.", oldEventCols)...)+` FROM choir_events h LEFT JOIN choir_event_bodies b ON b.ev_id = h.ev_id ORDER BY h.tape_seq, h.op_index`)
	out["derived"], _ = splitDigest(t, path, `SELECT quote(canonical_id), quote(og_rec_claim), quote(event_phase) FROM og_objects ORDER BY canonical_id`)
	out["event_phase"], _ = splitDigest(t, path, `SELECT quote(event_phase) FROM choir_events ORDER BY tape_seq, op_index`)
	for table, order := range verbatimSplitTables {
		out[table], _ = splitDigest(t, path, `SELECT * FROM `+table+` ORDER BY `+order)
	}
	return out
}

func requireSplitDigestsEqual(t *testing.T, before, after splitDigests) {
	t.Helper()
	for k, v := range before {
		if after[k] != v {
			t.Errorf("digest %s differs across the split: old %s new %s", k, v, after[k])
		}
	}
}

func scalar[T any](t *testing.T, path, query string) T {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var v T
	if err := db.QueryRow(query).Scan(&v); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return v
}

func requireSplitLayout(t *testing.T, f splitFixture) {
	t.Helper()
	old, err := statePresplitLayout(f.statePath)
	if err != nil {
		t.Fatal(err)
	}
	if old {
		t.Fatalf("state file still in the old layout")
	}
	if v := scalar[string](t, f.statePath, `SELECT value FROM store_meta WHERE key = 'schema_version'`); v != "2" {
		t.Fatalf("store_meta schema_version = %q, want 2", v)
	}
	floor, found, err := storeschema.ReadFloor(filepath.Join(deriveTextureWorkspacePath(f.dbPath), storeschema.FloorFile))
	if err != nil || !found || floor.Version != headerBodySplitFloor {
		t.Fatalf("floor = %+v found=%t err=%v, want %d", floor, found, err, headerBodySplitFloor)
	}
	for _, leftover := range []string{splitStagingSuffix, splitSidecarSuffix, splitAttemptsSuffix} {
		if _, err := os.Stat(f.statePath + leftover); err == nil {
			t.Fatalf("%s left behind", leftover)
		}
	}
}

func TestSplitMigratesFixtureStore(t *testing.T) {
	splitTestDisk(t, 1<<40)
	f := newSplitFixture(t, 6000, 4000)
	before := oldSplitDigests(t, f.statePath)
	oldObjects := scalar[int64](t, f.statePath, `SELECT COUNT(*) FROM og_objects`)
	oldEvents := scalar[int64](t, f.statePath, `SELECT COUNT(*) FROM choir_events`)
	oldSize := fileSizeOrZero(f.statePath)
	var last uint64
	var calls int
	started := time.Now()
	if err := MigrateHeaderBodySplit(f.dbPath, func(n uint64) {
		if n < last {
			t.Errorf("progress went backwards: %d after %d", n, last)
		}
		last = n
		calls++
	}); err != nil {
		t.Fatal(err)
	}
	took := time.Since(started)
	requireSplitLayout(t, f)
	after := newSplitDigests(t, f.statePath)
	requireSplitDigestsEqual(t, before, after)
	if n := scalar[int64](t, f.statePath, `SELECT COUNT(*) FROM og_objects`); n != oldObjects {
		t.Fatalf("og_objects %d, want %d", n, oldObjects)
	}
	if n := scalar[int64](t, f.statePath, `SELECT COUNT(*) FROM og_bodies`); n != oldObjects {
		t.Fatalf("og_bodies %d, want %d", n, oldObjects)
	}
	if n := scalar[int64](t, f.statePath, `SELECT COUNT(*) FROM choir_event_bodies`); n != oldEvents {
		t.Fatalf("choir_event_bodies %d, want %d", n, oldEvents)
	}
	if n := scalar[int64](t, f.statePath, `SELECT COUNT(*) FROM og_objects WHERE obj_id NOT IN (SELECT obj_id FROM og_bodies)`); n != 0 {
		t.Fatalf("%d headers without a body", n)
	}
	if last < uint64(oldObjects+oldEvents) || calls == 0 {
		t.Fatalf("progress %d over %d calls, want at least %d", last, calls, oldObjects+oldEvents)
	}
	// Every index of the new DDL is there, and the old expression indexes
	// are not.
	for _, ix := range append([]string{"idx_og_key_index_lookup", "idx_og_edges_to", "idx_wire_publish_debounce_entries_scope", "idx_choir_events_kind_updated_phase"}, objectgraph.ChoirEventRetireIndex, objectgraph.ChoirEventUpdatedIndex, "idx_og_objects_rec_claim") {
		if n := scalar[int](t, f.statePath, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = '`+ix+`'`); n != 1 {
			t.Errorf("index %s missing", ix)
		}
	}
	for _, ix := range []string{"idx_og_objects_choir_event_kind_updated", "idx_og_objects_choir_event_phase_updated", "idx_choir_events_kind_updated"} {
		if n := scalar[int](t, f.statePath, `SELECT COUNT(*) FROM sqlite_master WHERE name = '`+ix+`'`); n != 0 {
			t.Errorf("old index %s carried", ix)
		}
	}
	if av := scalar[int](t, f.statePath, `PRAGMA auto_vacuum`); av != 2 {
		t.Fatalf("auto_vacuum = %d, want 2", av)
	}
	if ps := scalar[int](t, f.statePath, `PRAGMA page_size`); ps != 4096 {
		t.Fatalf("page_size = %d", ps)
	}
	if jm := scalar[string](t, f.statePath, `PRAGMA journal_mode`); jm != "wal" {
		t.Fatalf("journal_mode = %q", jm)
	}
	if !presplitHeld(f.statePath) {
		t.Fatal("no rollback copy")
	}
	if old, _ := statePresplitLayout(f.statePath + presplitSuffix); !old {
		t.Fatal("the rollback copy is not the old file")
	}
	if _, ok := readLayoutMarker(f.statePath); !ok {
		t.Fatal("no layout marker")
	}
	stats := headerBodySplitStatsAt(f.statePath)
	if stats == nil || stats.Summary == nil || stats.Summary.Rows["og_objects"] != oldObjects || stats.Summary.SourceHeadSequence != 4242 || !stats.PresplitPresent {
		t.Fatalf("stats = %+v", stats)
	}
	// A second run is a cheap check.
	if err := MigrateHeaderBodySplit(f.dbPath, nil); err != nil {
		t.Fatal(err)
	}
	// The migrated store opens, and a large body reads back.
	s, err := Open(f.dbPath)
	if err != nil {
		t.Fatalf("open the migrated store: %v", err)
	}
	defer s.Close()
	var bigID string
	if err := s.db.QueryRow(`SELECT h.canonical_id FROM og_objects h JOIN og_bodies b ON b.obj_id = h.obj_id ORDER BY length(b.body) DESC LIMIT 1`).Scan(&bigID); err != nil {
		t.Fatal(err)
	}
	obj, err := s.ogStore.GetObject(context.Background(), bigID)
	if err != nil {
		t.Fatal(err)
	}
	if len(obj.Body) < 200<<10 {
		t.Fatalf("large body read back %d bytes", len(obj.Body))
	}
	rows := oldObjects + oldEvents
	t.Logf("split migration: %d rows (%d objects, %d events), %.1f MB of bodies, old file %.1f MB, new %.1f MB, in %s: %.0f rows/s, %.1f MB/s of old file",
		rows, oldObjects, oldEvents, float64(f.bodyBytes)/1e6, float64(oldSize)/1e6, float64(fileSizeOrZero(f.statePath))/1e6, took.Round(time.Millisecond),
		float64(rows)/took.Seconds(), float64(oldSize)/1e6/took.Seconds())
}

// Crash injection after every step: the next run completes, and the result is
// the same as an uninterrupted migration's.
func TestSplitCrashAfterEveryStepThenNextOpenCompletes(t *testing.T) {
	splitTestDisk(t, 1<<40)
	steps := []string{"preflight", "checkpoint-old", "stage", "copy-schema", "copy-tables", "copy-og_objects", "copy-choir_events",
		"build-indexes", "verify", "staged", "swap/renamed-old", "swap/renamed-new", "swap/floored", "swap"}
	for _, step := range steps {
		t.Run(strings.ReplaceAll(step, "/", "_"), func(t *testing.T) {
			f := newSplitFixture(t, 600, 300)
			before := oldSplitDigests(t, f.statePath)
			splitTestHook(t, crashAt(step))
			if err := MigrateHeaderBodySplit(f.dbPath, nil); !errors.Is(err, errSplitTestCrash) {
				t.Fatalf("crash at %s: err = %v", step, err)
			}
			testHookHeaderBodySplitStep = nil
			needed, err := HeaderBodySplitNeeded(f.dbPath)
			if err != nil {
				t.Fatal(err)
			}
			if step != "swap" && step != "swap/renamed-new" && step != "swap/floored" && !needed {
				t.Fatalf("after a crash at %s the store reports no work", step)
			}
			// The next boot's Open completes the migration.
			s, err := Open(f.dbPath)
			if err != nil {
				t.Fatalf("open after a crash at %s: %v", step, err)
			}
			_ = s.Close()
			requireSplitLayout(t, f)
			requireSplitDigestsEqual(t, before, newSplitDigests(t, f.statePath))
			if !presplitHeld(f.statePath) {
				t.Fatal("no rollback copy")
			}
		})
	}
}

// An old binary that ran between staging and the swap moved the source: the
// staged copy is discarded and the migration is redone from the moved file.
func TestSplitHeadMovedStagingDiscarded(t *testing.T) {
	splitTestDisk(t, 1<<40)
	f := newSplitFixture(t, 400, 200)
	splitTestHook(t, crashAt("staged"))
	if err := MigrateHeaderBodySplit(f.dbPath, nil); !errors.Is(err, errSplitTestCrash) {
		t.Fatal(err)
	}
	testHookHeaderBodySplitStep = nil
	if _, ok := readSplitSidecar(f.statePath); !ok {
		t.Fatal("no staged copy")
	}
	db, err := sql.Open("sqlite", dbx.SQLiteDSN(f.statePath, dbx.SQLiteOptions{}, false))
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond) // a distinct mtime
	if _, err := db.Exec(`UPDATE computer_event_projection_heads SET sequence = 4243, canonical_event_head = 'head-4243'`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO og_objects (canonical_id, object_kind, owner_id, content_hash, body, metadata, created_at, updated_at) VALUES ('obj-late', 'choir.run', 'owner', 'h', 'late', '{}', '2026-11-01', '2026-11-01')`); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()
	before := oldSplitDigests(t, f.statePath)
	if err := MigrateHeaderBodySplit(f.dbPath, nil); err != nil {
		t.Fatal(err)
	}
	requireSplitLayout(t, f)
	requireSplitDigestsEqual(t, before, newSplitDigests(t, f.statePath))
	if seq := scalar[int64](t, f.statePath, `SELECT sequence FROM computer_event_projection_heads`); seq != 4243 {
		t.Fatalf("head %d: the stale copy was swapped in", seq)
	}
}

// A crash between the renames: finished when the staged copy is intact,
// undone (the old file back as state.db) when it is not.
func TestSplitInterruptedSwapFinishedAndUndone(t *testing.T) {
	splitTestDisk(t, 1<<40)
	t.Run("finished", func(t *testing.T) {
		f := newSplitFixture(t, 300, 100)
		before := oldSplitDigests(t, f.statePath)
		splitTestHook(t, crashAt("swap/renamed-old"))
		if err := MigrateHeaderBodySplit(f.dbPath, nil); !errors.Is(err, errSplitTestCrash) {
			t.Fatal(err)
		}
		testHookHeaderBodySplitStep = nil
		if _, err := os.Stat(f.statePath); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("state file present between the renames: %v", err)
		}
		if needed, _ := HeaderBodySplitNeeded(f.dbPath); !needed {
			t.Fatal("an interrupted swap reports no work")
		}
		if err := MigrateHeaderBodySplit(f.dbPath, nil); err != nil {
			t.Fatal(err)
		}
		requireSplitLayout(t, f)
		requireSplitDigestsEqual(t, before, newSplitDigests(t, f.statePath))
	})
	t.Run("undone", func(t *testing.T) {
		f := newSplitFixture(t, 300, 100)
		raw, err := os.ReadFile(f.statePath)
		if err != nil {
			t.Fatal(err)
		}
		splitTestHook(t, crashAt("swap/renamed-old"))
		if err := MigrateHeaderBodySplit(f.dbPath, nil); !errors.Is(err, errSplitTestCrash) {
			t.Fatal(err)
		}
		testHookHeaderBodySplitStep = nil
		// The staged copy is damaged: the swap cannot be finished.
		if err := os.Truncate(f.statePath+splitStagingSuffix, 4096); err != nil {
			t.Fatal(err)
		}
		// With too little disk to redo, the undo alone is observable.
		splitTestDisk(t, 1<<20)
		var refusal *HeaderBodySplitRefusal
		if err := MigrateHeaderBodySplit(f.dbPath, nil); !errors.As(err, &refusal) {
			t.Fatalf("err = %v, want a refusal after the undo", err)
		}
		got, err := os.ReadFile(f.statePath)
		if err != nil {
			t.Fatalf("the undo left no state file: %v", err)
		}
		if sha256.Sum256(got) != sha256.Sum256(raw) {
			t.Fatal("the undone state file is not the old file")
		}
		if presplitHeld(f.statePath) {
			t.Fatal("rollback copy still present after the undo")
		}
		splitTestDisk(t, 1<<40)
		if err := MigrateHeaderBodySplit(f.dbPath, nil); err != nil {
			t.Fatal(err)
		}
		requireSplitLayout(t, f)
	})
}

// A disk too small refuses before any mutation: the state file is byte-equal,
// no floor, no staging, no attempt counted.
func TestSplitPreflightRefusalLeavesStoreUntouchedAndFloorFree(t *testing.T) {
	f := newSplitFixture(t, 300, 100)
	raw, err := os.ReadFile(f.statePath)
	if err != nil {
		t.Fatal(err)
	}
	live := int64(len(raw))
	splitTestDisk(t, live+compactFreeFloorBytes) // under live*1.05 + 1 GiB + floor
	var refusal *HeaderBodySplitRefusal
	err = MigrateHeaderBodySplit(f.dbPath, nil)
	if !errors.As(err, &refusal) {
		t.Fatalf("err = %v, want a refusal", err)
	}
	if refusal.Need <= refusal.Free || refusal.Live == 0 {
		t.Fatalf("refusal = %+v", refusal)
	}
	got, err := os.ReadFile(f.statePath)
	if err != nil || sha256.Sum256(got) != sha256.Sum256(raw) {
		t.Fatalf("state file changed by a refused migration (%v)", err)
	}
	if _, found, _ := storeschema.ReadFloor(filepath.Join(deriveTextureWorkspacePath(f.dbPath), storeschema.FloorFile)); found {
		t.Fatal("a refused migration wrote a floor")
	}
	for _, leftover := range []string{splitStagingSuffix, splitSidecarSuffix, splitAttemptsSuffix, presplitSuffix} {
		if _, err := os.Stat(f.statePath + leftover); err == nil {
			t.Fatalf("a refused migration left %s", leftover)
		}
	}
	// Open refuses the same way, and creates nothing.
	if _, err := Open(f.dbPath); !errors.As(err, &refusal) {
		t.Fatalf("Open err = %v, want the refusal", err)
	}
}

// Three migrations that never finish: the fourth boot refuses with the typed
// refusal instead of trying again.
func TestSplitAttemptsCapTypedRefusal(t *testing.T) {
	splitTestDisk(t, 1<<40)
	f := newSplitFixture(t, 200, 50)
	splitTestHook(t, crashAt("copy-og_objects"))
	for i := 0; i < headerBodySplitMaxAttempts; i++ {
		if err := MigrateHeaderBodySplit(f.dbPath, nil); !errors.Is(err, errSplitTestCrash) {
			t.Fatalf("attempt %d: %v", i+1, err)
		}
	}
	testHookHeaderBodySplitStep = nil
	var refusal *HeaderBodySplitRefusal
	err := MigrateHeaderBodySplit(f.dbPath, nil)
	if !errors.As(err, &refusal) || refusal.Attempts != headerBodySplitMaxAttempts {
		t.Fatalf("err = %v, want the attempts refusal", err)
	}
	if !strings.Contains(err.Error(), "attempts") {
		t.Fatalf("refusal %q does not name the attempts", err)
	}
	if old, _ := statePresplitLayout(f.statePath); !old {
		t.Fatal("the refused store is not the old file")
	}
	if _, err := os.Stat(f.statePath + splitStagingSuffix); err == nil {
		t.Fatal("a refused attempt left its staging file")
	}
}

// Reopen of an old-layout snapshot (a restore or base install) migrates it.
func TestSplitReopenOfOldLayoutSnapshotMigrates(t *testing.T) {
	splitTestDisk(t, 1<<40)
	s, _ := trimTestOpen(t)
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	f := newSplitFixture(t, 300, 100)
	before := oldSplitDigests(t, f.statePath)
	if err := s.Reopen(f.dbPath); err != nil {
		t.Fatalf("reopen an old-layout snapshot: %v", err)
	}
	defer s.Close()
	requireSplitLayout(t, f)
	requireSplitDigestsEqual(t, before, newSplitDigests(t, f.statePath))
	var id string
	if err := s.db.QueryRow(`SELECT canonical_id FROM og_objects ORDER BY obj_id LIMIT 1`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ogStore.GetObject(context.Background(), id); err != nil {
		t.Fatalf("read after reopen: %v", err)
	}
}

// A compact copy staged by the old binary is discarded: by the planned rebase
// (it would only be migrated again) and by the migration's preflight.
func TestSplitDiscardsOldLayoutStagedCompactCopy(t *testing.T) {
	splitTestDisk(t, 1<<40)
	t.Run("rebase", func(t *testing.T) {
		f := newSplitFixture(t, 100, 20)
		staged := f.statePath + compactStagedSuffix
		raw, err := os.ReadFile(f.statePath)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(staged, raw, 0o644); err != nil {
			t.Fatal(err)
		}
		side, _ := json.Marshal(compactSidecar{ComputerID: "computer-split", Sequence: 4242, Head: "head-4242", Size: int64(len(raw)), CreatedAt: time.Now()})
		if err := os.WriteFile(f.statePath+compactSidecarSuffix, side, 0o644); err != nil {
			t.Fatal(err)
		}
		did, err := RebaseOntoCompactCopy(f.dbPath, true)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(did, "pre-split layout") {
			t.Fatalf("rebase = %q, want the old-layout discard", did)
		}
		if _, err := os.Stat(staged); err == nil {
			t.Fatal("old-layout staged copy kept")
		}
	})
	t.Run("preflight", func(t *testing.T) {
		f := newSplitFixture(t, 100, 20)
		for _, suffix := range []string{compactStagedSuffix, compactSidecarSuffix, compactStagingSuffix} {
			if err := os.WriteFile(f.statePath+suffix, []byte("old"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		if err := MigrateHeaderBodySplit(f.dbPath, nil); err != nil {
			t.Fatal(err)
		}
		for _, suffix := range []string{compactStagedSuffix, compactSidecarSuffix, compactStagingSuffix} {
			if _, err := os.Stat(f.statePath + suffix); err == nil {
				t.Fatalf("%s kept across the split", suffix)
			}
		}
	})
}

// The rollback copy: compaction does not stack a copy beside it, finalize
// removes it, and it is dropped automatically under the free-space floor.
func TestSplitPresplitLifecycle(t *testing.T) {
	splitTestDisk(t, 1<<40)
	f := newSplitFixture(t, 200, 50)
	if err := MigrateHeaderBodySplit(f.dbPath, nil); err != nil {
		t.Fatal(err)
	}
	s, err := Open(f.dbPath)
	if err != nil {
		t.Fatal(err)
	}
	staged, err := s.prepareCompactCopy(context.Background(), true)
	if staged || err == nil || !strings.Contains(err.Error(), "split-finalize") {
		t.Fatalf("compact copy beside the rollback copy: staged=%t err=%v", staged, err)
	}
	if st := s.HeaderBodySplitStats(); st == nil || !st.PresplitPresent || st.PresplitBytes == 0 {
		t.Fatalf("stats = %+v", st)
	}
	freed, err := FinalizeHeaderBodySplit(f.dbPath)
	if err != nil || freed == 0 || presplitHeld(f.statePath) {
		t.Fatalf("finalize freed=%d err=%v held=%t", freed, err, presplitHeld(f.statePath))
	}
	if st := s.HeaderBodySplitStats(); st == nil || st.PresplitPresent || st.Summary == nil || st.Summary.PresplitRemoved != "finalized" {
		t.Fatalf("stats after finalize = %+v", st)
	}
	_ = s.Close()

	g := newSplitFixture(t, 200, 50)
	if err := MigrateHeaderBodySplit(g.dbPath, nil); err != nil {
		t.Fatal(err)
	}
	splitTestDisk(t, compactFreeFloorBytes-1)
	if err := MigrateHeaderBodySplit(g.dbPath, nil); err != nil {
		t.Fatal(err)
	}
	st := headerBodySplitStatsAt(g.statePath)
	if presplitHeld(g.statePath) || st == nil || !st.PresplitAutoDropped || st.Summary.PresplitRemoved != "auto_dropped_low_space" {
		t.Fatalf("under the floor: held=%t stats=%+v", presplitHeld(g.statePath), st)
	}
}

// The staging connection puts the sorter's temp files beside the store.
func TestSplitTempFilesBesideTheStore(t *testing.T) {
	f := newSplitFixture(t, 10, 5)
	m := &splitCopier{statePath: f.statePath, staging: f.statePath + splitStagingSuffix, side: &HeaderBodySplitSummary{Rows: map[string]int64{}, StepMillis: map[string]int64{}}}
	if err := m.open(); err != nil {
		t.Fatal(err)
	}
	defer m.close()
	var dir string
	if err := m.conn.QueryRowContext(context.Background(), `PRAGMA temp_store_directory`).Scan(&dir); err != nil {
		t.Fatal(err)
	}
	if dir != filepath.Dir(f.statePath) {
		t.Fatalf("temp_store_directory = %q, want %q", dir, filepath.Dir(f.statePath))
	}
	var store int
	if err := m.conn.QueryRowContext(context.Background(), `PRAGMA temp_store`).Scan(&store); err != nil || store != 1 {
		t.Fatalf("temp_store = %d (%v), want 1 (FILE)", store, err)
	}
	if _, indexes := splitChoirEventsDDL(); len(indexes) != 5 {
		t.Fatalf("choirEventsDDL indexes = %d, want 5", len(indexes))
	}
}
