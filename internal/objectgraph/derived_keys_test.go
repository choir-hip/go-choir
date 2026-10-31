package objectgraph

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math/rand"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

// The three hot reads (run by agent_id, records by addressee, source_ref by
// doc_id) on the og_key_index side table
// (docs/archive/problems/store-hot-reads-need-a-rollback-safe-side-table-2026-10-21.md).
// Ways the change can fail, written before the code; each has a test here
// unless it says where it lives:
//
//  1. Boot scans or rewrites og_objects (TestOpenDoesNotTouchOgObjects).
//  2. The side read returns other rows, or another order or count, than the
//     JSON read it replaces (TestDerivedKeyReadsMatchJSONReads).
//  3. A writer misses the table (TestEveryObjectWriterMaintainsDerivedKeys,
//     TestOnlyMaintainedWritersTouchOgObjects).
//  4. An older binary wrote og_objects meanwhile, and the table is stale
//     after rollback-then-forward (TestRollbackThenForwardRepairsTheTable,
//     TestReadTripwireCatchesAStaleRow).
//  5. A validation pass and writers interleave wrongly (TestValidatorWithWriterStorm).
//  6. The validator starves the engine (TestValidatorHoldsTheWriteConnectionOnlyPerPage).
//  7. A malformed body fails a write, or reads differently from the JSON
//     path (TestMalformedBodyNeverFailsAWriteAndKeepsTheJSONPath).
//  8. The stray STORED columns of the reverted release break something
//     (TestDerivedKeysIgnoreStrayStoredColumns).
//  9. Replay and witness extraction: internal/computerversion.
// 10. Op labels (TestEngineOpsNameTheirKind) and plan shape (TestDerivedKeyPlansUseTheirIndexes).
// 11. A withdrawal (a failed maintenance, a malformed write) lands while a
//     pass publishes and is overwritten by it
//     (TestPassPublicationKeepsAConcurrentWithdrawal,
//     TestPassPublicationKeepsAConcurrentMalformedWrite).
// 12. A read admitted to the side table before a writer withdraws the family
//     reads it anyway; the production projector holds no lock across the append, so
//     holding the mutex does not order them
//     (TestReadSeesAMalformedWriteCommittedAfterItsAdmission,
//     TestReadSeesAWithdrawalCommittedAfterItsAdmission).

const (
	testRunKind       = ObjectKind("choir.run")
	testSourceRefKind = ObjectKind("choir.source_ref")
)

type keyedObj struct {
	id        string
	kind      ObjectKind
	owner     string
	computer  string
	meta      map[string]any
	body      string
	at        time.Time
	tombstone bool
}

func (k keyedObj) object(t *testing.T) Object {
	t.Helper()
	meta, err := json.Marshal(k.meta)
	if err != nil {
		t.Fatal(err)
	}
	owner := k.owner
	if owner == "" {
		owner = "o1"
	}
	return Object{CanonicalID: k.id, ObjectKind: k.kind, OwnerID: owner, ComputerID: k.computer, VersionID: "v",
		ContentHash: ContentHash(k.kind, []byte(k.body), meta), Body: []byte(k.body), Metadata: meta,
		CreatedAt: k.at, UpdatedAt: k.at, Tombstone: k.tombstone}
}

func openDerivedStore(t *testing.T) (*sql.DB, *SQLStore) {
	t.Helper()
	db := openTestDB(t)
	store := NewSQLStore(db)
	if err := store.EnsureSchema(context.Background()); err != nil {
		t.Fatal(err)
	}
	return db, store
}

// rawOldBinaryPut writes og_objects with the upsert and nothing else: no
// derived-key maintenance, as a writer that predates the side table would.
func rawOldBinaryPut(t *testing.T, db *sql.DB, obj Object) {
	t.Helper()
	if _, err := db.Exec(upsertObjectSQL, upsertObjectArgs(obj)...); err != nil {
		t.Fatalf("raw put %s: %v", obj.CanonicalID, err)
	}
}

type sideRow struct {
	Key, Owner, Hash string
	Value            sql.NullString
	UpdatedAt        time.Time
	OK               bool
}

func readSideRows(t *testing.T, db *sql.DB) map[string]sideRow {
	t.Helper()
	rows, err := db.Query(`SELECT canonical_id, key_name, key_value, owner_id, updated_at, content_hash, derive_ok FROM ` + DerivedKeysTable)
	if err != nil {
		t.Fatalf("read side table: %v", err)
	}
	defer rows.Close()
	out := map[string]sideRow{}
	for rows.Next() {
		var id string
		var r sideRow
		if err := rows.Scan(&id, &r.Key, &r.Value, &r.Owner, &r.UpdatedAt, &r.Hash, &r.OK); err != nil {
			t.Fatal(err)
		}
		out[id+"|"+r.Key] = r
	}
	return out
}

// truthSideRows recomputes every side row from og_objects with the engine's
// own expressions, one row at a time, the way the JSON reads evaluate them.
func truthSideRows(t *testing.T, db *sql.DB) map[string]sideRow {
	t.Helper()
	out := map[string]sideRow{}
	for _, fam := range derivedFamilies {
		rows, err := db.Query(`SELECT canonical_id, owner_id, updated_at, content_hash FROM og_objects WHERE object_kind = ?`, string(fam.Kind))
		if err != nil {
			t.Fatal(err)
		}
		type head struct {
			id, owner, hash string
			at              time.Time
		}
		var heads []head
		for rows.Next() {
			var h head
			if err := rows.Scan(&h.id, &h.owner, &h.at, &h.hash); err != nil {
				t.Fatal(err)
			}
			heads = append(heads, h)
		}
		rows.Close()
		for _, h := range heads {
			var v sql.NullString
			ok := true
			if err := db.QueryRow(`SELECT substr(`+fam.expr+`, 1, 255) FROM og_objects WHERE canonical_id = ?`, h.id).Scan(&v); err != nil {
				v, ok = sql.NullString{}, false
			}
			out[h.id+"|"+fam.Name] = sideRow{Key: fam.Name, Owner: h.owner, Hash: h.hash, Value: v, UpdatedAt: h.at, OK: ok}
		}
	}
	return out
}

func assertSideMatchesTruth(t *testing.T, db *sql.DB) {
	t.Helper()
	got, want := readSideRows(t, db), truthSideRows(t, db)
	var diffs []string
	for k, w := range want {
		g, ok := got[k]
		switch {
		case !ok:
			diffs = append(diffs, "missing "+k)
		case g.Value != w.Value || g.Owner != w.Owner || g.Hash != w.Hash || !g.UpdatedAt.Equal(w.UpdatedAt) || g.OK != w.OK:
			diffs = append(diffs, fmt.Sprintf("differs %s: have %+v want %+v", k, g, w))
		}
	}
	for k := range got {
		if _, ok := want[k]; !ok {
			diffs = append(diffs, "orphan "+k)
		}
	}
	sort.Strings(diffs)
	if len(diffs) > 0 {
		if len(diffs) > 12 {
			diffs = append(diffs[:12], fmt.Sprintf("... %d more", len(diffs)-12))
		}
		t.Fatalf("side table is not the recomputation of og_objects:\n%s", strings.Join(diffs, "\n"))
	}
}

// pageIDs reads every page of ListObjectsByMetadataPage the way
// store.ogListAllByMetadata does.
func pageIDs(t *testing.T, s *SQLStore, kind ObjectKind, field, value string, pageSize int) ([]string, error) {
	t.Helper()
	var ids []string
	after := ""
	for {
		page, err := s.ListObjectsByMetadataPage(context.Background(), string(kind), "$."+field, value, after, pageSize)
		if err != nil {
			return ids, err
		}
		if len(page) == 0 {
			return ids, nil
		}
		for _, o := range page {
			ids = append(ids, o.CanonicalID)
		}
		after = page[len(page)-1].CanonicalID
	}
}

func bodyIDs(t *testing.T, s *SQLStore, owner, addressee string, limit int) ([]string, error) {
	t.Helper()
	objs, err := s.ListObjectsByOwnerAndBody(context.Background(), string(commitmentRecordKind), owner,
		[]JSONFieldMatch{{JSONPath: "$.addressee", Value: addressee}}, limit)
	ids := make([]string, 0, len(objs))
	for _, o := range objs {
		ids = append(ids, o.CanonicalID)
	}
	return ids, err
}

// jsonOnly runs fn with every family on the JSON path, then restores.
func jsonOnly(s *SQLStore, fn func()) {
	d := s.derived
	saved := map[string]bool{}
	for name := range d.validated {
		saved[name] = d.Validated(name)
		d.setValidated(name, false)
	}
	defer func() {
		for name, v := range saved {
			d.setValidated(name, v)
		}
	}()
	fn()
}

// sideCalls counts the reads the store answered from the side table.
func sideCalls(s *SQLStore) uint64 { return s.derived.sideReads.Load() }

func edgeValues() []any {
	return []any{
		"texture:a", "Texture:A", "texture:a ", " texture:a", "texture:ü", "ü", "", "null", "12", "true", `a"b\c`, "<b>&amp;</b>",
		"line\nbreak", strings.Repeat("x", 254), strings.Repeat("y", 255), strings.Repeat("z", 256),
		strings.Repeat("ü", 254), strings.Repeat("ü", 255), strings.Repeat("ü", 300),
		nil, 12, 12.5, true, false, map[string]any{"k": "v"}, []any{"texture:a"},
	}
}

func TestDerivedKeyReadsMatchJSONReads(t *testing.T) {
	ctx := context.Background()
	db, store := openDerivedStore(t)
	rng := rand.New(rand.NewSource(7))
	at := time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC)
	values := edgeValues()
	n := 0
	put := func(k keyedObj) {
		n++
		k.at = at.Add(time.Duration(n) * time.Second) // distinct updated_at: no ties to order
		if err := store.PutObject(ctx, k.object(t)); err != nil {
			t.Fatalf("put %s: %v", k.id, err)
		}
	}
	for i := 0; i < 80; i++ {
		v := values[rng.Intn(len(values))]
		owner := []string{"o1", "o2"}[rng.Intn(2)]
		tomb := rng.Intn(9) == 0
		put(keyedObj{id: fmt.Sprintf("run-%03d", i), kind: testRunKind, owner: owner, meta: map[string]any{"agent_id": v, "x": i}, body: `{}`, tombstone: tomb})
		put(keyedObj{id: fmt.Sprintf("ref-%03d", i), kind: testSourceRefKind, owner: owner, meta: map[string]any{"doc_id": v}, body: `{}`, tombstone: tomb})
		put(keyedObj{id: fmt.Sprintf("rec-%03d", i), kind: commitmentRecordKind, owner: owner, meta: map[string]any{"inbox": "i"}, body: mustJSON(map[string]any{"addressee": v, "n": i}), tombstone: tomb})
	}
	// Rows with no field at all, with another kind, and a non-object body.
	put(keyedObj{id: "run-none", kind: testRunKind, meta: map[string]any{"other": 1}, body: `{}`})
	put(keyedObj{id: "rec-none", kind: commitmentRecordKind, meta: map[string]any{}, body: `{"other":1}`})
	put(keyedObj{id: "rec-array", kind: commitmentRecordKind, meta: map[string]any{}, body: `[1,2]`})
	put(keyedObj{id: "other-kind", kind: ObjectKind("choir.agent"), meta: map[string]any{"agent_id": "texture:a"}, body: `{"addressee":"texture:a"}`})
	if err := store.ValidateDerivedKeys(ctx, DerivedKeyValidation{PageSize: 37}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{KeyRunAgentID, KeyCommitmentAddressee, KeySourceRefDocID} {
		if !store.derived.Validated(name) {
			t.Fatalf("family %s not validated after a clean pass", name)
		}
	}
	assertSideMatchesTruth(t, db)

	lookups := []string{"texture:a", "Texture:A", "texture:a ", "texture:ü", "ü", "null", "12", "12.5", "true", "false", `a"b\c`, "<b>&amp;</b>", "line\nbreak",
		strings.Repeat("x", 254), strings.Repeat("y", 255), strings.Repeat("z", 256),
		strings.Repeat("ü", 254), strings.Repeat("ü", 255), strings.Repeat("ü", 300), "absent", ""}
	sidePathCalls := uint64(0)
	for _, v := range lookups {
		for _, pageSize := range []int{2, 5, 256} {
			for _, c := range []struct {
				kind  ObjectKind
				field string
			}{{testRunKind, "agent_id"}, {testSourceRefKind, "doc_id"}} {
				before := sideCalls(store)
				got, err := pageIDs(t, store, c.kind, c.field, v, pageSize)
				if err != nil {
					t.Fatalf("side %s %q: %v", c.field, v, err)
				}
				if utf8Len(v) < derivedKeyValueMaxChars && sideCalls(store) == before {
					t.Fatalf("%s %q (%d chars) did not use the side table", c.field, v, utf8Len(v))
				}
				if utf8Len(v) >= derivedKeyValueMaxChars && sideCalls(store) != before {
					t.Fatalf("%s %q (%d chars) used the side table; values of 255 characters or more keep the JSON path", c.field, v, utf8Len(v))
				}
				sidePathCalls += sideCalls(store) - before
				var want []string
				jsonOnly(store, func() { want, err = pageIDs(t, store, c.kind, c.field, v, pageSize) })
				if err != nil {
					t.Fatalf("json %s %q: %v", c.field, v, err)
				}
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("%s=%q page %d: side table read %v, JSON read %v", c.field, v, pageSize, got, want)
				}
			}
		}
		for _, owner := range []string{"o1", "o2", "nobody"} {
			for _, limit := range []int{1, 3, 1000} {
				before := sideCalls(store)
				got, err := bodyIDs(t, store, owner, v, limit)
				if err != nil {
					t.Fatalf("side addressee %q: %v", v, err)
				}
				if utf8Len(v) < derivedKeyValueMaxChars && sideCalls(store) == before {
					t.Fatalf("addressee %q did not use the side table", v)
				}
				var want []string
				jsonOnly(store, func() { want, err = bodyIDs(t, store, owner, v, limit) })
				if err != nil {
					t.Fatalf("json addressee %q: %v", v, err)
				}
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("addressee=%q owner=%s limit=%d: side table read %v, JSON read %v", v, owner, limit, got, want)
				}
			}
		}
	}
	if sidePathCalls == 0 {
		t.Fatal("no read used the side table")
	}
}

func utf8Len(s string) int { return len([]rune(s)) }

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func TestReadsUseTheJSONPathUntilValidated(t *testing.T) {
	ctx := context.Background()
	_, store := openDerivedStore(t)
	at := time.Now().UTC()
	if err := store.PutObject(ctx, keyedObj{id: "run-1", kind: testRunKind, meta: map[string]any{"agent_id": "a"}, body: `{}`, at: at}.object(t)); err != nil {
		t.Fatal(err)
	}
	if ids, err := pageIDs(t, store, testRunKind, "agent_id", "a", 10); err != nil || len(ids) != 1 {
		t.Fatalf("read before validation = %v, %v", ids, err)
	}
	if n := sideCalls(store); n != 0 {
		t.Fatalf("%d side-table reads before the family was validated", n)
	}
}

func TestOpenDoesNotTouchOgObjects(t *testing.T) {
	ctx := context.Background()
	db, store := openDerivedStore(t)
	// A store that already has many rows and no side table rows: the paper
	// the day this ships.
	const rows = 2000
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC)
	for start := 0; start < rows; start += 500 {
		var sb strings.Builder
		var args []any
		sb.WriteString(`INSERT INTO og_objects (canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, metadata, created_at, updated_at, tombstone, superseded_by) VALUES `)
		for i := start; i < start+500; i++ {
			if i > start {
				sb.WriteString(",")
			}
			sb.WriteString("(?,?,?,?,?,?,?,?,?,?,?,?)")
			kind := []ObjectKind{testRunKind, commitmentRecordKind, testSourceRefKind, "choir.agent"}[i%4]
			args = append(args, fmt.Sprintf("row-%06d", i), string(kind), "o1", "c1", "v", fmt.Sprintf("h-%d", i),
				fmt.Sprintf(`{"addressee":"a%d"}`, i%50), fmt.Sprintf(`{"agent_id":"a%d","doc_id":"d%d"}`, i%50, i%90), at, at, false, "")
		}
		if _, err := tx.Exec(sb.String(), args...); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	// total_changes counts every row the connection has inserted, updated or
	// deleted: an open that rewrote og_objects would move it.
	tableHash := func() int64 {
		var n int64
		if err := db.QueryRow(`SELECT total_changes()`).Scan(&n); err != nil {
			t.Fatalf("total_changes: %v", err)
		}
		return n
	}
	columns := func() string {
		rows, err := db.Query(`SELECT name FROM pragma_table_xinfo('og_objects') ORDER BY cid`)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var cols []string
		for rows.Next() {
			var c string
			rows.Scan(&c)
			cols = append(cols, c)
		}
		return strings.Join(cols, ",")
	}
	beforeChanges, beforeCols := tableHash(), columns()

	// The next boot: a fresh store over the same database.
	reopened := NewSQLStore(db)
	start := time.Now()
	if err := reopened.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}
	elapsed := time.Since(start)
	if got := tableHash(); got != beforeChanges {
		t.Fatalf("opening the store changed rows (total_changes %d -> %d)", beforeChanges, got)
	}
	if got := columns(); got != beforeCols {
		t.Fatalf("opening the store changed og_objects' columns: %s -> %s", beforeCols, got)
	}
	var sideCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM ` + DerivedKeysTable).Scan(&sideCount); err != nil {
		t.Fatalf("the side table does not exist after open: %v", err)
	}
	if sideCount != 0 {
		t.Fatalf("opening the store filled the side table with %d rows", sideCount)
	}
	for name := range reopened.derived.validated {
		if reopened.derived.Validated(name) {
			t.Fatalf("family %s is validated at open", name)
		}
	}
	if elapsed > 5*time.Second {
		t.Fatalf("EnsureSchema over %d rows took %s", rows, elapsed)
	}
	_ = store
}

func TestEveryObjectWriterMaintainsDerivedKeys(t *testing.T) {
	ctx := context.Background()
	db, store := openDerivedStore(t)
	at := time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC)

	run := keyedObj{id: "run-a", kind: testRunKind, meta: map[string]any{"agent_id": "texture:1"}, body: `{}`, at: at}
	rec := keyedObj{id: "rec-a", kind: commitmentRecordKind, meta: map[string]any{}, body: `{"addressee":"management:o1"}`, at: at}
	ref := keyedObj{id: "ref-a", kind: testSourceRefKind, meta: map[string]any{"doc_id": "doc-1"}, body: `{}`, at: at}

	if err := store.PutObject(ctx, run.object(t)); err != nil {
		t.Fatal(err)
	}
	if err := store.PutBatch(ctx, Batch{Objects: []Object{rec.object(t), ref.object(t)}}); err != nil {
		t.Fatal(err)
	}
	assertSideMatchesTruth(t, db)
	if len(readSideRows(t, db)) != 3 {
		t.Fatalf("PutObject and PutBatch left %d side rows, want 3", len(readSideRows(t, db)))
	}

	// An update that changes the key replaces the row; a conditional batch
	// maintains it too.
	run2 := run
	run2.meta = map[string]any{"agent_id": "texture:2"}
	run2.at = at.Add(time.Minute)
	prev := run.object(t)
	if err := store.PutBatchConditional(ctx, []ObjectCondition{{CanonicalID: "run-a", Exists: true, ExpectedContentHash: prev.ContentHash}}, Batch{Objects: []Object{run2.object(t)}}); err != nil {
		t.Fatal(err)
	}
	assertSideMatchesTruth(t, db)
	if v := readSideRows(t, db)["run-a|"+KeyRunAgentID].Value.String; v != "texture:2" {
		t.Fatalf("a conditional update left key %q, want texture:2", v)
	}

	// A tombstone keeps the row and its key, as the JSON read does.
	rec.tombstone = true
	rec.at = at.Add(2 * time.Minute)
	if err := store.PutObject(ctx, rec.object(t)); err != nil {
		t.Fatal(err)
	}
	assertSideMatchesTruth(t, db)

	// A delete drops the row.
	if err := store.DeleteObject(ctx, "ref-a"); err != nil {
		t.Fatal(err)
	}
	assertSideMatchesTruth(t, db)
	if _, ok := readSideRows(t, db)["ref-a|"+KeySourceRefDocID]; ok {
		t.Fatal("DeleteObject left the side row")
	}

	// An object of a kind with no family writes no side row and no statement.
	if err := store.PutObject(ctx, keyedObj{id: "agent-a", kind: "choir.agent", meta: map[string]any{"agent_id": "x"}, body: `{}`, at: at}.object(t)); err != nil {
		t.Fatal(err)
	}
	if len(readSideRows(t, db)) != 2 {
		t.Fatalf("a kind with no family wrote a side row: %v", readSideRows(t, db))
	}
}

func TestRollbackThenForwardRepairsTheTable(t *testing.T) {
	ctx := context.Background()
	db, store := openDerivedStore(t)
	at := time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC)
	var all []keyedObj
	for i := 0; i < 60; i++ {
		all = append(all,
			keyedObj{id: fmt.Sprintf("run-%02d", i), kind: testRunKind, meta: map[string]any{"agent_id": fmt.Sprintf("a%d", i%6)}, body: `{}`, at: at.Add(time.Duration(i) * time.Second)},
			keyedObj{id: fmt.Sprintf("rec-%02d", i), kind: commitmentRecordKind, meta: map[string]any{}, body: mustJSON(map[string]any{"addressee": fmt.Sprintf("a%d", i%6)}), at: at.Add(time.Duration(i) * time.Second)},
			keyedObj{id: fmt.Sprintf("ref-%02d", i), kind: testSourceRefKind, meta: map[string]any{"doc_id": fmt.Sprintf("d%d", i%6)}, body: `{}`, at: at.Add(time.Duration(i) * time.Second)})
	}
	for _, k := range all {
		if err := store.PutObject(ctx, k.object(t)); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.ValidateDerivedKeys(ctx, DerivedKeyValidation{PageSize: 16}); err != nil {
		t.Fatal(err)
	}
	assertSideMatchesTruth(t, db)

	// The older binary runs for a while: it knows nothing of the side table.
	changed := all[0]
	changed.meta = map[string]any{"agent_id": "moved"} // a key change
	changed.at = at.Add(time.Hour)
	rawOldBinaryPut(t, db, changed.object(t))
	added := keyedObj{id: "run-new", kind: testRunKind, meta: map[string]any{"agent_id": "a1"}, body: `{}`, at: at.Add(2 * time.Hour)}
	rawOldBinaryPut(t, db, added.object(t))
	recChanged := all[1]
	recChanged.body = mustJSON(map[string]any{"addressee": "moved"})
	recChanged.at = at.Add(3 * time.Hour)
	rawOldBinaryPut(t, db, recChanged.object(t))
	touched := all[2] // same content, newer updated_at: only the signature columns differ
	touched.at = at.Add(4 * time.Hour)
	rawOldBinaryPut(t, db, touched.object(t))
	if _, err := db.Exec(`DELETE FROM og_objects WHERE canonical_id = ?`, "ref-03"); err != nil {
		t.Fatal(err)
	}

	// Forward again: a new process, a new store over the same database.
	forward := NewSQLStore(db)
	if err := forward.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}
	for name := range forward.derived.validated {
		if forward.derived.Validated(name) {
			t.Fatalf("family %s is validated at the first open after a rollback", name)
		}
	}
	// Before the validator: every read is the JSON read, and right.
	got, err := pageIDs(t, forward, testRunKind, "agent_id", "moved", 8)
	if err != nil || !reflect.DeepEqual(got, []string{"run-00"}) {
		t.Fatalf("read of the older binary's update before validation = %v, %v", got, err)
	}
	if n := sideCalls(forward); n != 0 {
		t.Fatalf("%d side-table reads before validation", n)
	}
	// The online pass repairs the table and validates.
	if err := forward.ValidateDerivedKeys(ctx, DerivedKeyValidation{PageSize: 16}); err != nil {
		t.Fatal(err)
	}
	assertSideMatchesTruth(t, db)
	got, err = pageIDs(t, forward, testRunKind, "agent_id", "moved", 8)
	if err != nil || !reflect.DeepEqual(got, []string{"run-00"}) || sideCalls(forward) == 0 {
		t.Fatalf("read after the pass = %v, %v (side-table reads %d)", got, err, sideCalls(forward))
	}
	ids, err := bodyIDs(t, forward, "o1", "moved", 10)
	if err != nil || !reflect.DeepEqual(ids, []string{"rec-00"}) {
		t.Fatalf("addressee read after the pass = %v, %v", ids, err)
	}
	if ids, _ := pageIDs(t, forward, testSourceRefKind, "doc_id", "d3", 100); contains(ids, "ref-03") {
		t.Fatalf("the deleted row is still read: %v", ids)
	}
}

func contains(ids []string, id string) bool {
	for _, v := range ids {
		if v == id {
			return true
		}
	}
	return false
}

func TestReadTripwireCatchesAStaleRow(t *testing.T) {
	ctx := context.Background()
	db, store := openDerivedStore(t)
	at := time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC)
	k := keyedObj{id: "run-1", kind: testRunKind, meta: map[string]any{"agent_id": "a"}, body: `{}`, at: at}
	if err := store.PutObject(ctx, k.object(t)); err != nil {
		t.Fatal(err)
	}
	if err := store.ValidateDerivedKeys(ctx, DerivedKeyValidation{}); err != nil {
		t.Fatal(err)
	}
	// A writer this change does not know about changes the row behind the
	// table's back while the family is validated.
	k.meta = map[string]any{"agent_id": "b"}
	k.at = at.Add(time.Minute)
	rawOldBinaryPut(t, db, k.object(t))
	ids, err := pageIDs(t, store, testRunKind, "agent_id", "a", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 0 {
		t.Fatalf("the stale row still answers agent a: %v", ids)
	}
	if store.derived.Validated(KeyRunAgentID) {
		t.Fatal("a stale row did not invalidate the family")
	}
	if store.derived.Mismatches(KeyRunAgentID) == 0 {
		t.Fatal("the tripwire did not count the mismatch")
	}
	ids, err = pageIDs(t, store, testRunKind, "agent_id", "b", 10)
	if err != nil || !reflect.DeepEqual(ids, []string{"run-1"}) {
		t.Fatalf("after the tripwire the JSON read = %v, %v", ids, err)
	}
}

func TestValidatorWithWriterStorm(t *testing.T) {
	ctx := context.Background()
	db, store := openDerivedStore(t)
	at := time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC)
	// Existing rows with no side rows (an older binary wrote them).
	for i := 0; i < 150; i++ {
		rawOldBinaryPut(t, db, keyedObj{id: fmt.Sprintf("run-%04d", i), kind: testRunKind, meta: map[string]any{"agent_id": fmt.Sprintf("a%d", i%7)}, body: `{}`, at: at}.object(t))
		rawOldBinaryPut(t, db, keyedObj{id: fmt.Sprintf("rec-%04d", i), kind: commitmentRecordKind, meta: map[string]any{}, body: mustJSON(map[string]any{"addressee": fmt.Sprintf("a%d", i%7)}), at: at}.object(t))
	}
	var wg sync.WaitGroup
	stop := make(chan struct{})
	for w := 0; w < 3; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			rng := rand.New(rand.NewSource(int64(w)))
			for n := 0; ; n++ {
				select {
				case <-stop:
					return
				default:
				}
				i := rng.Intn(400) // some behind the cursor, some ahead, some new
				k := keyedObj{id: fmt.Sprintf("run-%04d", i), kind: testRunKind, meta: map[string]any{"agent_id": fmt.Sprintf("w%d-%d", w, n%5)}, body: `{}`, at: at.Add(time.Duration(n+1) * time.Second)}
				if rng.Intn(5) == 0 {
					_ = store.DeleteObject(ctx, k.id)
					continue
				}
				if err := store.PutObject(ctx, k.object(t)); err != nil {
					t.Errorf("writer %d: %v", w, err)
					return
				}
			}
		}(w)
	}
	if err := store.ValidateDerivedKeys(ctx, DerivedKeyValidation{PageSize: 20}); err != nil {
		t.Fatal(err)
	}
	if !store.derived.Validated(KeyRunAgentID) {
		t.Fatal("the pass did not validate")
	}
	time.Sleep(200 * time.Millisecond) // writers keep going after the pass
	close(stop)
	wg.Wait()
	assertSideMatchesTruth(t, db) // no second pass: the writers kept it right
}

func TestValidatorHoldsTheWriteConnectionOnlyPerPage(t *testing.T) {
	ctx := context.Background()
	db, store := openDerivedStore(t)
	at := time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC)
	for i := 0; i < 400; i++ {
		rawOldBinaryPut(t, db, keyedObj{id: fmt.Sprintf("rec-%05d", i), kind: commitmentRecordKind, meta: map[string]any{}, body: mustJSON(map[string]any{"addressee": fmt.Sprintf("a%d", i%7)}), at: at}.object(t))
	}
	done := make(chan struct{})
	var maxWait time.Duration
	var reads int
	deadline := time.Now().Add(2 * time.Minute)
	go func() {
		defer close(done)
		for time.Now().Before(deadline) {
			time.Sleep(5 * time.Millisecond)
			start := time.Now()
			_, err := store.GetObject(ctx, "rec-00001")
			if err != nil {
				t.Errorf("GetObject during validation: %v", err)
				return
			}
			if d := time.Since(start); d > maxWait {
				maxWait = d
			}
			reads++
			if store.derived.Validated(KeyCommitmentAddressee) {
				return
			}
		}
	}()
	var logged []string
	var logMu sync.Mutex
	if err := store.ValidateDerivedKeys(ctx, DerivedKeyValidation{PageSize: 50, DutyCycle: 0.5, Logf: func(f string, a ...any) {
		logMu.Lock()
		logged = append(logged, fmt.Sprintf(f, a...))
		logMu.Unlock()
	}}); err != nil {
		t.Fatal(err)
	}
	<-done
	if pages := store.derived.passPages.Load(); pages < 400/50 {
		t.Fatalf("validator took %d pages for 400 rows at 50 a page", pages)
	}
	if hold := time.Duration(store.derived.maxPageHoldNanos.Load()); hold > 3*time.Second {
		t.Fatalf("one page held the write connection %s", hold)
	}
	if reads < 3 || maxWait > 4*time.Second {
		t.Fatalf("a concurrent reader made %d reads, longest %s", reads, maxWait)
	}
	logMu.Lock()
	defer logMu.Unlock()
	if len(logged) == 0 || !strings.Contains(strings.Join(logged, "\n"), "commitment_record.addressee") {
		t.Fatalf("the validator logged no progress for the family: %v", logged)
	}
}

func TestMalformedBodyNeverFailsAWriteAndKeepsTheJSONPath(t *testing.T) {
	ctx := context.Background()
	db, store := openDerivedStore(t)
	at := time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC)
	good := keyedObj{id: "rec-good", kind: commitmentRecordKind, meta: map[string]any{}, body: `{"addressee":"a"}`, at: at}
	bad := keyedObj{id: "rec-bad", kind: commitmentRecordKind, meta: map[string]any{}, body: `not json`, at: at.Add(time.Second)}
	if err := store.PutObject(ctx, good.object(t)); err != nil {
		t.Fatal(err)
	}
	if err := store.PutObject(ctx, bad.object(t)); err != nil {
		t.Fatalf("a malformed body failed the write: %v", err)
	}
	rows := readSideRows(t, db)
	if r := rows["rec-bad|"+KeyCommitmentAddressee]; r.OK || r.Value.Valid {
		t.Fatalf("the malformed row's side row = %+v, want derive_ok false and no key", r)
	}
	if err := store.ValidateDerivedKeys(ctx, DerivedKeyValidation{}); err != nil {
		t.Fatal(err)
	}
	// The JSON read errors on a malformed body of the kind and owner; the
	// side table must not turn that into an answer.
	var jsonErr error
	jsonOnly(store, func() { _, jsonErr = bodyIDs(t, store, "o1", "a", 10) })
	if jsonErr == nil {
		t.Fatal("test premise: the JSON read should fail on a malformed body")
	}
	_, err := bodyIDs(t, store, "o1", "a", 10)
	if err == nil || !strings.Contains(err.Error(), "malformed JSON") {
		t.Fatalf("read with a malformed row present = %v, want the JSON read's error", err)
	}
	// The other families are not affected.
	if !store.derived.Validated(KeyRunAgentID) {
		t.Fatal("a malformed record body invalidated the run family")
	}
	// Fixing the body (a new write) clears it.
	bad.body = `{"addressee":"b"}`
	bad.at = at.Add(time.Minute)
	if err := store.PutObject(ctx, bad.object(t)); err != nil {
		t.Fatal(err)
	}
	if err := store.ValidateDerivedKeys(ctx, DerivedKeyValidation{}); err != nil {
		t.Fatal(err)
	}
	ids, err := bodyIDs(t, store, "o1", "b", 10)
	if err != nil || !reflect.DeepEqual(ids, []string{"rec-bad"}) {
		t.Fatalf("after the body was fixed: %v, %v", ids, err)
	}
	if sideCalls(store) == 0 {
		t.Fatal("the fixed family does not use the side table")
	}
}

func TestDerivedKeyPlansUseTheirIndexes(t *testing.T) {
	ctx := context.Background()
	db, store := openDerivedStore(t)
	at := time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC)
	for i := 0; i < 40; i++ {
		store.PutObject(ctx, keyedObj{id: fmt.Sprintf("rec-%02d", i), kind: commitmentRecordKind, meta: map[string]any{}, body: `{"addressee":"a"}`, at: at.Add(time.Duration(i) * time.Second)}.object(t))
	}
	idsQuery, idsArgs := derivedIDsByKeySQL(KeyCommitmentAddressee, "a", "o1", "", 50)
	plan := explainPlan(t, db, idsQuery, idsArgs...)
	if !strings.Contains(plan, "USING INDEX idx_og_key_index_recent") {
		t.Fatalf("the newest-first id query does not read its index:\n%s", plan)
	}
	if strings.Contains(plan, "TEMP B-TREE") {
		t.Fatalf("the newest-first id query sorts instead of walking the index:\n%s", plan)
	}
	pageQuery, pageArgs := derivedPageIDsSQL(KeyRunAgentID, "a", "", 256)
	plan = explainPlan(t, db, pageQuery, pageArgs...)
	if !strings.Contains(plan, "USING INDEX idx_og_key_index_lookup") || strings.Contains(plan, "TEMP B-TREE") {
		t.Fatalf("the keyset id query does not walk its index in order:\n%s", plan)
	}
	// The validator's page of og_objects headers is a primary-key range, not
	// a sort of the whole kind per page.
	lo, hi := derivedKeyRange(commitmentRecordKind)
	headQuery, headArgs := ogHeadPageSQL(lo, hi, 200)
	plan = explainPlan(t, db, headQuery, headArgs...)
	if strings.Contains(plan, "TEMP B-TREE") || strings.Contains(plan, "SCAN og_objects") && !strings.Contains(plan, "USING") {
		t.Fatalf("the validator's head page sorts or scans the kind on every page:\n%s", plan)
	}
}

func TestValidatorPagesAKindsPrimaryKeyRangeAndFindsStrays(t *testing.T) {
	ctx := context.Background()
	db, store := openDerivedStore(t)
	at := time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC)
	for i := 0; i < 420; i++ {
		id, err := BuildCanonicalID(commitmentRecordKind, fmt.Sprintf("owner%d", i%3), fmt.Sprintf("key-%05d", i))
		if err != nil {
			t.Fatal(err)
		}
		rawOldBinaryPut(t, db, keyedObj{id: id, kind: commitmentRecordKind, owner: fmt.Sprintf("owner%d", i%3), meta: map[string]any{}, body: mustJSON(map[string]any{"addressee": fmt.Sprintf("a%d", i%5)}), at: at.Add(time.Duration(i) * time.Second)}.object(t))
	}
	// Ids a code path never builds, below and above the kind's range, and
	// a key row with no object (an orphan inside the range).
	for _, id := range []string{"aaa-stray", "zzz-stray"} {
		rawOldBinaryPut(t, db, keyedObj{id: id, kind: commitmentRecordKind, meta: map[string]any{}, body: `{"addressee":"a1"}`, at: at}.object(t))
	}
	if _, err := db.Exec(`INSERT INTO `+DerivedKeysTable+` (canonical_id, key_name, key_value, owner_id, updated_at, content_hash, derive_ok) VALUES ('obj:choir.commitment_record:x:gone', ?, 'a1', 'o1', ?, 'h', 1), ('stray-orphan', ?, 'a1', 'o1', ?, 'h', 1)`, KeyCommitmentAddressee, at, KeyCommitmentAddressee, at); err != nil {
		t.Fatal(err)
	}
	if err := store.ValidateDerivedKeys(ctx, DerivedKeyValidation{PageSize: 50}); err != nil {
		t.Fatal(err)
	}
	assertSideMatchesTruth(t, db)
	if pages := store.derived.passPages.Load(); pages < 420/50 {
		t.Fatalf("a kind of 420 rows took %d pages at 50 a page", pages)
	}
	ids, err := bodyIDs(t, store, "o1", "a1", 1000)
	if err != nil || !contains(ids, "zzz-stray") {
		t.Fatalf("a stray id is not read through the table: %v, %v", ids, err)
	}
	var want []string
	jsonOnly(store, func() { want, err = bodyIDs(t, store, "o1", "a1", 1000) })
	sort.Strings(ids) // equal updated_at: the order among ties is the engine's
	sort.Strings(want)
	if err != nil || !reflect.DeepEqual(ids, want) {
		t.Fatalf("side %v\nJSON %v (%v)", ids, want, err)
	}
}

// The first pass over a store that has rows and no key rows (the paper the day
// this ships) derives everything; every later pass compares narrow columns and
// derives nothing. Measured here on 3,600 rows so the numbers are in the log
// of every run; the assertions are loose on purpose (a slow laptop must not
// fail them) and tight on the shape: bounded pages, and a repeat pass much
// cheaper than the first.
func TestValidatorFirstPassAndRepeatPassOnManyRows(t *testing.T) {
	ctx := context.Background()
	db, store := openDerivedStore(t)
	at := time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC)
	const perKind = 1200
	insert := func(kind ObjectKind, make func(i int) (meta, body string)) {
		tx, err := db.Begin()
		if err != nil {
			t.Fatal(err)
		}
		for start := 0; start < perKind; start += 400 {
			var sb strings.Builder
			var args []any
			sb.WriteString(`INSERT INTO og_objects (canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, metadata, created_at, updated_at, tombstone, superseded_by) VALUES `)
			for i := start; i < start+400; i++ {
				if i > start {
					sb.WriteString(",")
				}
				sb.WriteString("(?,?,?,?,?,?,?,?,?,?,?,?)")
				id, err := BuildCanonicalID(kind, "o1", fmt.Sprintf("key-%06d", i))
				if err != nil {
					t.Fatal(err)
				}
				meta, body := make(i)
				args = append(args, id, string(kind), "o1", "c1", "v", ContentHash(kind, []byte(body), []byte(meta)), body, meta, at.Add(time.Duration(i)*time.Second), at.Add(time.Duration(i)*time.Second), false, "")
			}
			if _, err := tx.Exec(sb.String(), args...); err != nil {
				t.Fatal(err)
			}
		}
		if err := tx.Commit(); err != nil {
			t.Fatal(err)
		}
	}
	filler := strings.Repeat("evidence text, a sentence a record carries. ", 30) // ~1.3 KB
	insert(testRunKind, func(i int) (string, string) { return fmt.Sprintf(`{"agent_id":"texture:%d"}`, i%300), `{}` })
	insert(commitmentRecordKind, func(i int) (string, string) {
		return `{}`, fmt.Sprintf(`{"addressee":"texture:%d","text":%q}`, i%300, filler)
	})
	insert(testSourceRefKind, func(i int) (string, string) { return fmt.Sprintf(`{"doc_id":"doc-%d"}`, i%500), `{}` })

	first := time.Now()
	var lines []string
	logf := func(f string, a ...any) { lines = append(lines, fmt.Sprintf(f, a...)) }
	if err := store.ValidateDerivedKeys(ctx, DerivedKeyValidation{PageSize: 200, Logf: logf}); err != nil {
		t.Fatal(err)
	}
	firstTook := time.Since(first)
	t.Logf("first pass over %d rows: %s (%.0f rows/s), %d pages, longest hold %s",
		3*perKind, firstTook.Round(time.Millisecond), float64(3*perKind)/firstTook.Seconds(), store.derived.passPages.Load(),
		time.Duration(store.derived.maxPageHoldNanos.Load()).Round(time.Millisecond))
	assertSideMatchesTruthSample(t, db)

	second, secondLines := time.Now(), len(lines)
	for name := range store.derived.validated {
		store.derived.setValidated(name, false)
	}
	if err := store.ValidateDerivedKeys(ctx, DerivedKeyValidation{PageSize: 200, Logf: logf}); err != nil {
		t.Fatal(err)
	}
	secondTook := time.Since(second)
	t.Logf("repeat pass over the same rows: %s (%.0f rows/s)", secondTook.Round(time.Millisecond), float64(3*perKind)/secondTook.Seconds())
	if hold := time.Duration(store.derived.maxPageHoldNanos.Load()); hold > 5*time.Second {
		t.Fatalf("a page held the write connection %s", hold)
	}
	// The repeat pass compares and derives nothing: every "validated" line
	// of the second pass reports 0 re-derived. Progress lines are timed, so
	// the passes log different numbers of lines (the race detector's slower
	// first pass logged more and pushed its own line past the midpoint).
	for _, line := range lines[secondLines:] {
		if strings.Contains(line, "validated ") && !strings.Contains(line, "(0 re-derived,") {
			t.Errorf("the repeat pass derived again: %s", line)
		}
	}
}

// assertSideMatchesTruthSample checks a spread of rows against the recomputation
// (the full recomputation of 18,000 rows one query at a time is the slow part).
func assertSideMatchesTruthSample(t *testing.T, db *sql.DB) {
	t.Helper()
	got := readSideRows(t, db)
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM og_objects WHERE object_kind IN ('choir.run','choir.commitment_record','choir.source_ref')`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if len(got) != n {
		t.Fatalf("%d key rows for %d keyed objects", len(got), n)
	}
	for _, fam := range derivedFamilies {
		rows, err := db.Query(`SELECT canonical_id, substr(`+fam.expr+`, 1, 255) FROM og_objects WHERE object_kind = ? ORDER BY canonical_id LIMIT 300`, string(fam.Kind))
		if err != nil {
			t.Fatal(err)
		}
		for rows.Next() {
			var id string
			var v sql.NullString
			if err := rows.Scan(&id, &v); err != nil {
				t.Fatal(err)
			}
			if g := got[id+"|"+fam.Name]; g.Value != v || !g.OK {
				t.Fatalf("%s %s: key row %+v, expression %v", fam.Name, id, g, v)
			}
		}
		rows.Close()
	}
}

// projectorPut writes og_objects the way the production projector does: in
// its own transaction, without any lock beyond the write connection, maintaining the side table
// inside it. failMaintain stands for a maintenance statement that failed:
// the row is written and the family withdrawn, with no key row.
func projectorPut(t *testing.T, db *sql.DB, d *DerivedKeys, obj Object, failMaintain bool) {
	t.Helper()
	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, upsertObjectSQL, upsertObjectArgs(obj)...); err != nil {
		t.Fatalf("projector put %s: %v", obj.CanonicalID, err)
	}
	if failMaintain {
		d.Invalidate(derivedFamilyForKind(obj.ObjectKind).Name, "test: maintenance failed")
	} else {
		d.Maintain(ctx, tx, obj.CanonicalID, obj.ObjectKind)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
}

// once wraps a seam so the writer runs at the first matching call only.
func once(family string, fn func()) func(string) {
	var fired bool
	return func(name string) {
		if name == family && !fired {
			fired = true
			fn()
		}
	}
}

func TestPassPublicationKeepsAConcurrentWithdrawal(t *testing.T) {
	ctx := context.Background()
	db, store := openDerivedStore(t)
	at := time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC)
	if err := store.PutObject(ctx, keyedObj{id: "run-1", kind: testRunKind, meta: map[string]any{"agent_id": "a"}, body: `{}`, at: at}.object(t)); err != nil {
		t.Fatal(err)
	}
	// While the pass publishes, the projector writes a run and its
	// maintenance fails: the family is withdrawn and the run has no key row.
	late := keyedObj{id: "run-2", kind: testRunKind, meta: map[string]any{"agent_id": "a"}, body: `{}`, at: at.Add(time.Second)}
	fired := false
	store.derived.beforePublish = once(KeyRunAgentID, func() {
		fired = true
		projectorPut(t, db, store.derived, late.object(t), true)
	})
	if err := store.ValidateDerivedKeys(ctx, DerivedKeyValidation{}); err != nil {
		t.Fatal(err)
	}
	store.derived.beforePublish = nil
	if !fired {
		t.Fatal("test premise: the seam did not run")
	}
	if store.derived.Validated(KeyRunAgentID) {
		t.Fatal("the pass published over a withdrawal that landed while it published")
	}
	ids, err := pageIDs(t, store, testRunKind, "agent_id", "a", 10)
	if err != nil || !reflect.DeepEqual(ids, []string{"run-1", "run-2"}) {
		t.Fatalf("read after the withdrawal = %v, %v; want both runs", ids, err)
	}
}

func TestPassPublicationKeepsAConcurrentMalformedWrite(t *testing.T) {
	ctx := context.Background()
	db, store := openDerivedStore(t)
	at := time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC)
	if err := store.PutObject(ctx, keyedObj{id: "rec-good", kind: commitmentRecordKind, meta: map[string]any{}, body: `{"addressee":"a"}`, at: at}.object(t)); err != nil {
		t.Fatal(err)
	}
	// While the pass publishes, the projector commits a record whose body is
	// not JSON: the family must keep the JSON path, whose read errors.
	bad := keyedObj{id: "rec-bad", kind: commitmentRecordKind, meta: map[string]any{}, body: `not json`, at: at.Add(time.Second)}
	fired := false
	store.derived.beforePublish = once(KeyCommitmentAddressee, func() {
		fired = true
		projectorPut(t, db, store.derived, bad.object(t), false)
	})
	if err := store.ValidateDerivedKeys(ctx, DerivedKeyValidation{}); err != nil {
		t.Fatal(err)
	}
	store.derived.beforePublish = nil
	if !fired {
		t.Fatal("test premise: the seam did not run")
	}
	if store.derived.readable(KeyCommitmentAddressee) {
		t.Fatal("the pass published the family readable over a malformed write that landed while it published")
	}
	if _, err := bodyIDs(t, store, "o1", "a", 10); err == nil || !strings.Contains(err.Error(), "malformed JSON") {
		t.Fatalf("read with a malformed row present = %v, want the JSON read's error", err)
	}
}

func TestReadSeesAMalformedWriteCommittedAfterItsAdmission(t *testing.T) {
	ctx := context.Background()
	db, store := openDerivedStore(t)
	at := time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC)
	if err := store.PutObject(ctx, keyedObj{id: "rec-good", kind: commitmentRecordKind, meta: map[string]any{}, body: `{"addressee":"a"}`, at: at}.object(t)); err != nil {
		t.Fatal(err)
	}
	if err := store.ValidateDerivedKeys(ctx, DerivedKeyValidation{}); err != nil {
		t.Fatal(err)
	}
	if !store.derived.readable(KeyCommitmentAddressee) {
		t.Fatal("test premise: the family is readable")
	}
	// The read has chosen the side table; the
	// projector, which does not take it, commits a malformed record.
	bad := keyedObj{id: "rec-bad", kind: commitmentRecordKind, meta: map[string]any{}, body: `not json`, at: at.Add(time.Second)}
	fired := false
	store.derived.beforeRead = once(KeyCommitmentAddressee, func() {
		fired = true
		projectorPut(t, db, store.derived, bad.object(t), false)
	})
	_, err := bodyIDs(t, store, "o1", "a", 10)
	store.derived.beforeRead = nil
	if !fired {
		t.Fatal("test premise: the seam did not run")
	}
	if err == nil || !strings.Contains(err.Error(), "malformed JSON") {
		t.Fatalf("read admitted before a malformed commit = %v, want the JSON read's error", err)
	}
}

func TestReadSeesAWithdrawalCommittedAfterItsAdmission(t *testing.T) {
	at := time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name        string
		family      string
		first, late keyedObj
		read        func(t *testing.T, s *SQLStore) ([]string, error)
	}{
		{
			name: "run by agent_id", family: KeyRunAgentID,
			first: keyedObj{id: "run-1", kind: testRunKind, meta: map[string]any{"agent_id": "a"}, body: `{}`, at: at},
			late:  keyedObj{id: "run-2", kind: testRunKind, meta: map[string]any{"agent_id": "a"}, body: `{}`, at: at.Add(time.Second)},
			// One page: a caller that pages on would find the row on its
			// next page, from the JSON path, and hide the missed one.
			read: func(t *testing.T, s *SQLStore) ([]string, error) {
				objs, err := s.ListObjectsByMetadataPage(context.Background(), string(testRunKind), "$.agent_id", "a", "", 10)
				ids := make([]string, 0, len(objs))
				for _, o := range objs {
					ids = append(ids, o.CanonicalID)
				}
				return ids, err
			},
		},
		{
			name: "records by addressee", family: KeyCommitmentAddressee,
			first: keyedObj{id: "rec-1", kind: commitmentRecordKind, meta: map[string]any{}, body: `{"addressee":"a"}`, at: at},
			late:  keyedObj{id: "rec-2", kind: commitmentRecordKind, meta: map[string]any{}, body: `{"addressee":"a"}`, at: at.Add(time.Second)},
			read: func(t *testing.T, s *SQLStore) ([]string, error) {
				ids, err := bodyIDs(t, s, "o1", "a", 10)
				sort.Strings(ids)
				return ids, err
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			db, store := openDerivedStore(t)
			if err := store.PutObject(ctx, tc.first.object(t)); err != nil {
				t.Fatal(err)
			}
			if err := store.ValidateDerivedKeys(ctx, DerivedKeyValidation{}); err != nil {
				t.Fatal(err)
			}
			// Admitted to the side table, then the projector commits a row
			// whose maintenance failed: no key row, family withdrawn.
			fired := false
			store.derived.beforeRead = once(tc.family, func() {
				fired = true
				projectorPut(t, db, store.derived, tc.late.object(t), true)
			})
			ids, err := tc.read(t, store)
			store.derived.beforeRead = nil
			if !fired {
				t.Fatal("test premise: the seam did not run")
			}
			if want := []string{tc.first.id, tc.late.id}; err != nil || !reflect.DeepEqual(ids, want) {
				t.Fatalf("read admitted before a withdrawal = %v, %v; want %v", ids, err, want)
			}
		})
	}
}
