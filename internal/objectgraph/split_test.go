package objectgraph

// The header/body split of og_objects (docs/design/store-header-body-split-2026-11-01.md).
// Ways it can fail, written before the tests:
//
//  1. A header read touches og_bodies (a plan names it), so a scan reads
//     bodies again; or a body read scans og_bodies, or puts it outer.
//  2. An upsert or a rename moves obj_id, orphaning or misattaching a body;
//     a delete leaves a body behind.
//  3. og_rec_claim, event_phase or event_kind differ from what the
//     pre-split generated column and expression indexes computed (affinity,
//     NULL handling, non-JSON and JSONB-looking bodies, numeric values).
//  4. The derived-key mark identity drifts, so the first split boot
//     re-validates every family.
//  5. A REPLACE on a split table renumbers a row.
//  6. A two-phase read attaches a body by an obj_id reused between phases.

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/dbx/dbxplan"
)

func openSplitStore(t *testing.T) (*sql.DB, *SQLStore) {
	t.Helper()
	db := openTestDB(t)
	store := NewSQLStore(db)
	if err := store.EnsureSchema(context.Background()); err != nil {
		t.Fatal(err)
	}
	return db, store
}

func writeObj(t *testing.T, x ObjectWriteTx, obj Object) int64 {
	t.Helper()
	id, err := WriteObjectTx(context.Background(), x, obj)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func splitObj(id string, kind ObjectKind, owner, body, meta string, at time.Time) Object {
	var b []byte
	if body != "\x00nil" {
		b = []byte(body)
	}
	return Object{CanonicalID: id, ObjectKind: kind, OwnerID: owner, ComputerID: "c", VersionID: "v",
		ContentHash: "h-" + id, Body: b, Metadata: []byte(meta), CreatedAt: at, UpdatedAt: at}
}

func TestHeaderReadsReadNoBody(t *testing.T) {
	db, _ := openSplitStore(t)
	lo, hi := KindIDRange("choir.run")
	fam := derivedFamilyByName(KeyCommitmentAddressee)
	readIDs := `SELECT ` + headColumns(fam.ordered) + ` FROM og_objects
		WHERE object_kind = '` + string(fam.Kind) + `' AND canonical_id IN (?,?) ORDER BY canonical_id ASC`
	trajQ, trajArgs := trajectoryRefsSQL("choir.run", "t1")
	headQ, headArgs := headersByKindOwnerSQL("choir.run", "o", 50)
	allQ, allArgs := headersByKindOwnerSQL("choir.run", "o", -1)
	pageQ, pageArgs := ogHeadPageSQL(lo, hi, 50, false)
	pageOQ, pageOArgs := ogHeadPageSQL(lo, hi, 50, true)
	cases := []struct {
		name, index, query string
		args               []any
	}{
		{"newest headers", "idx_og_objects_updated", NewestObjectHeadersSQL, []any{10}},
		{"kind exists", "", ObjectKindExistsSQL, []any{"choir.run"}},
		{"kind id range page", "", KindIDRangePageSQL, []any{lo, hi, 50}},
		{"kind metadata page", "idx_og_objects_kind_id", KindMetadataPageSQL, []any{"choir.run", "", 50}},
		{"kind outside range", "", KindOutsideRangeSQL, []any{"choir.run", lo, hi}},
		{"trajectory refs", "", trajQ, trajArgs},
		{"snapshot phase 1, all kinds", "idx_og_objects_owner_computer_kind", snapshotHeadersSQL(0), []any{"o", "c"}},
		{"snapshot phase 1, kinds", "", snapshotHeadersSQL(2), []any{"o", "c", "a", "b"}},
		{"kind owner where phase 1", "idx_og_objects_kind_owner_updated", headQ, headArgs},
		{"kind owner where phase 1, all", "idx_og_objects_kind_owner_updated", allQ, allArgs},
		{"conditions", "", `SELECT version_id, content_hash FROM og_objects WHERE canonical_id = ?`, []any{"x"}},
		{"derived head page", "", pageQ, pageArgs},
		{"derived head page ordered", "", pageOQ, pageOArgs},
		{"derived validation ids", "", readIDs, []any{"a", "b"}},
		{"derived count", "", `SELECT COUNT(*) FROM og_objects WHERE object_kind = ?`, []any{"choir.run"}},
		{"count by edge to", "", countObjectsByEdgeToSQL, []any{"to", "k", "choir.run"}},
		{"retire oldest", ChoirEventUpdatedIndex, `SELECT MIN(updated_at) FROM og_objects INDEXED BY ` + ChoirEventUpdatedIndex + ` WHERE object_kind = 'choir.event' AND updated_at < ?`, []any{time.Now()}},
		{"retire slice", ChoirEventRetireIndex, `SELECT canonical_id FROM og_objects INDEXED BY ` + ChoirEventRetireIndex + ` WHERE object_kind = 'choir.event' AND event_kind IN (?) AND updated_at >= ? AND updated_at < ? AND event_phase = ''`, []any{"tool.result", time.Now(), time.Now()}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			plan := dbxplan.HeaderOnly(t, db, c.index, c.query, c.args...)
			if strings.Contains(c.query, "body") {
				t.Fatalf("a header read names body:\n%s", c.query)
			}
			t.Log(plan)
		})
	}
}

func TestBodyReadsSeekBodiesByRowid(t *testing.T) {
	db, _ := openSplitStore(t)
	listMeta, listMetaArgs := metadataListSQL("choir.run", "$.run_id", "r", 10)
	listJSON, listJSONArgs := metadataListJSONSQL("choir.run", "$.other", "r", 10)
	pageMeta, pageMetaArgs := metadataPageSQL("choir.run", "$.run_id", "r", "", 10)
	pageJSON, pageJSONArgs := metadataPageJSONSQL("choir.run", "$.other", "r", "", 10)
	inbox, inboxArgs := recordsByInboxSeqSQL("i", 0, 10)
	claim, claimArgs := commitmentClaimSQL("o", 10)
	ownerBody, ownerBodyArgs, err := ownerBodySQL(string(commitmentRecordKind), "o", []JSONFieldMatch{{JSONPath: "$.directive", Value: "", MissingMatchesEmpty: true}}, 10)
	if err != nil {
		t.Fatal(err)
	}
	fam := derivedFamilyByName(KeyCommitmentAddressee)
	cases := []struct {
		name, alias, query string
		args               []any
	}{
		{"get object", "b", objectSelectSQL + ` WHERE h.canonical_id = ?`, []any{"x"}},
		{"list objects", "b", objectSelectSQL + ` WHERE 1=1 AND h.object_kind = ? AND h.owner_id = ? ORDER BY h.updated_at DESC LIMIT ?`, []any{"k", "o", 10}},
		{"snapshot", "b", objectSelectSQL + ` WHERE h.owner_id = ? AND h.computer_id = ? ORDER BY h.canonical_id`, []any{"o", "c"}},
		{"fetch by id", "b", snapshotFetchByIDSQL(3), []any{"a", "b", "c"}},
		{"by metadata", "b", objectSelectSQL + ` WHERE h.object_kind = ? AND ` + jsonTextSQL(`h.metadata`, `?`) + ` = ? LIMIT 1`, []any{"k", "$.x", "$.x", "v"}},
		{"metadata list", "b", listMeta, listMetaArgs},
		{"metadata list json", "b", listJSON, listJSONArgs},
		{"metadata page", "b", pageMeta, pageMetaArgs},
		{"metadata page json", "b", pageJSON, pageJSONArgs},
		{"objects page", "b", objectsPageSQL, []any{"k", "", 10}},
		{"scope page", "b", ObjectsByKindOwnerComputerPageSQL, []any{"k", "o", "c", "", 10}},
		{"by edge to", "b", objectsByEdgeToSQL, []any{"to", "k", "choir.run"}},
		{"owner and body", "b", ownerBody, ownerBodyArgs},
		{"inbox seq", "b", inbox, inboxArgs},
		{"claims", "b", claim, claimArgs},
		{"json body fields", "b", `SELECT h.canonical_id, h.computer_id, ` + jsonTextSQL(bodyTextSQL, `?`) + jsonBodyFieldsFromSQL, []any{"$.a", "$.a", "k", "o", 10}},
		{"phase 2 bodies", "og_bodies", bodiesByIDSQL(3), []any{1, 2, 3}},
		{"derive addressee", "og_bodies", deriveSQL(fam, `canonical_id = ?`, true), []any{fam.Name, string(fam.Kind), "x"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Log(dbxplan.BodyByRowid(t, db, c.alias, c.query, c.args...))
		})
	}
	// The edge read seeks the edges first, not the whole kind.
	if plan := dbxplan.Plan(t, db, objectsByEdgeToSQL, "to", "k", "choir.run"); !strings.HasPrefix(plan, "SEARCH e USING INDEX idx_og_edges_to") {
		t.Fatalf("edge read is not edge-outer:\n%s", plan)
	}
	// The metadata families derive from the header alone.
	for _, f := range derivedFamilies {
		if f.needsBody {
			continue
		}
		dbxplan.HeaderOnly(t, db, "", deriveSQL(&f, `canonical_id = ?`, true), f.Name, string(f.Kind), "x")
	}
}

func objIDOf(t *testing.T, db *sql.DB, id string) int64 {
	t.Helper()
	var objID int64
	if err := db.QueryRow(`SELECT obj_id FROM og_objects WHERE canonical_id = ?`, id).Scan(&objID); err != nil {
		t.Fatal(err)
	}
	return objID
}

func countRows(t *testing.T, db *sql.DB, query string, args ...any) int {
	t.Helper()
	var n int
	if err := db.QueryRow(query, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestWriteKeepsObjIDAcrossUpsertAndRename(t *testing.T) {
	ctx := context.Background()
	db, store := openSplitStore(t)
	at := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	first := writeObj(t, db, splitObj("a", "choir.run", "o", `{"v":1}`, `{}`, at))
	writeObj(t, db, splitObj("b", "choir.run", "o", `{"v":"b"}`, `{}`, at))
	again := writeObj(t, db, splitObj("a", "choir.run", "o", `{"v":2}`, `{"x":1}`, at.Add(time.Second)))
	if again != first || objIDOf(t, db, "a") != first {
		t.Fatalf("upsert moved obj_id: %d then %d", first, again)
	}
	got, err := store.GetObject(ctx, "a")
	if err != nil || string(got.Body) != `{"v":2}` || string(got.Metadata) != `{"x":1}` {
		t.Fatalf("upsert read back %+v (%v)", got, err)
	}
	if n := countRows(t, db, `SELECT COUNT(*) FROM og_bodies`); n != 2 {
		t.Fatalf("og_bodies rows = %d, want 2", n)
	}
	if _, err := db.Exec(`UPDATE og_objects SET canonical_id = 'a2' WHERE canonical_id = 'a'`); err != nil {
		t.Fatal(err)
	}
	if objIDOf(t, db, "a2") != first {
		t.Fatal("rename moved obj_id")
	}
	got, err = store.GetObject(ctx, "a2")
	if err != nil || string(got.Body) != `{"v":2}` {
		t.Fatalf("renamed row's body %q (%v)", got.Body, err)
	}
	// A nil body is a body row holding NULL, not a missing row.
	writeObj(t, db, splitObj("n", "choir.run", "o", "\x00nil", `{}`, at))
	got, err = store.GetObject(ctx, "n")
	if err != nil || got.Body != nil {
		t.Fatalf("nil body read back %q (%v)", got.Body, err)
	}
	// A header whose body row is gone is an error, never a NULL body.
	if _, err := db.Exec(`DELETE FROM og_bodies WHERE obj_id = ?`, objIDOf(t, db, "b")); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetObject(ctx, "b"); err == nil || !strings.Contains(err.Error(), "no body row") {
		t.Fatalf("missing body read as %v", err)
	}
}

func TestDeleteDropsBody(t *testing.T) {
	ctx := context.Background()
	db, store := openSplitStore(t)
	at := time.Now().UTC()
	writeObj(t, db, splitObj("a", "choir.run", "o", `{}`, `{}`, at))
	writeObj(t, db, splitObj("b", "choir.run", "o", `{}`, `{}`, at))
	writeObj(t, db, splitObj("c", "choir.run", "o", `{}`, `{}`, at))
	bID := objIDOf(t, db, "b")
	if err := store.DeleteObject(ctx, "a"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM og_objects WHERE canonical_id = 'b'`); err != nil {
		t.Fatal(err)
	}
	if n := countRows(t, db, `SELECT COUNT(*) FROM og_bodies`); n != 1 {
		t.Fatalf("og_bodies rows after two deletes = %d, want 1", n)
	}
	if n := countRows(t, db, `SELECT COUNT(*) FROM og_bodies WHERE obj_id = ?`, bID); n != 0 {
		t.Fatal("plain DELETE left its body")
	}
	if n := countRows(t, db, `SELECT COUNT(*) FROM og_bodies b LEFT JOIN og_objects h ON h.obj_id = b.obj_id WHERE h.obj_id IS NULL`); n != 0 {
		t.Fatalf("%d orphan bodies", n)
	}
}

// presplitSchema is the frozen pre-split guest store schema.
func presplitSchema(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "store", "testdata", "presplit", "schema.sql"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

const presplitUpsertSQL = `INSERT INTO og_objects
	(canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, metadata, created_at, updated_at, tombstone, superseded_by)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

func TestBodyDerivedMatchesOldExpressions(t *testing.T) {
	newDB, _ := openSplitStore(t)
	oldDB := openTestDB(t)
	if _, err := oldDB.Exec(presplitSchema(t)); err != nil {
		t.Fatalf("frozen schema: %v", err)
	}
	var jsonb []byte
	if err := oldDB.QueryRow(`SELECT jsonb('{"directive":"x","payload":{"phase":"jb"}}')`).Scan(&jsonb); err != nil {
		t.Fatal(err)
	}
	bodies := []any{nil, []byte{}, []byte("not json"), []byte(`{}`), []byte(`{"directive":""}`), []byte(`{"directive":"x"}`),
		[]byte(`{"directive":5}`), []byte(`{"directive":null}`), []byte(`{"payload":{"phase":"p"}}`), []byte(`{"payload":{"phase":7}}`),
		[]byte(`{"payload":{"phase":""}}`), []byte(`[1,2]`), []byte(`"s"`), []byte(`12`), jsonb, "text body", `{"payload":{"phase":"t"}}`}
	metas := []string{`{}`, `{"phase":"m"}`, `{"kind":"tool.result"}`, `{"kind":5,"phase":3}`, `{"kind":"x","phase":""}`, `{"kind":null}`, `{"kind":"5"}`} // non-JSON metadata is refused by both layouts (the og_meta_* columns)
	kinds := []ObjectKind{commitmentRecordKind, "choir.event", "choir.run"}
	at := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	n := 0
	for _, kind := range kinds {
		for bi, body := range bodies {
			for mi, meta := range metas {
				id := fmt.Sprintf("%s-%d-%d", kind, bi, mi)
				args := []any{id, string(kind), "o", "c", "v", "h", body, meta, at, at, false, ""}
				if _, err := oldDB.Exec(presplitUpsertSQL, args...); err != nil {
					t.Fatalf("old put %s: %v", id, err)
				}
				// The new writer binds the same values: header ?1..?11, body ?12.
				newArgs := []any{id, string(kind), "o", "c", "v", "h", meta, at, at, false, "", body}
				var objID int64
				if err := newDB.QueryRow(ObjectHeaderUpsertSQL, newArgs...).Scan(&objID); err != nil {
					t.Fatalf("new put %s: %v", id, err)
				}
				if _, err := newDB.Exec(ObjectBodyUpsertSQL, objID, body); err != nil {
					t.Fatal(err)
				}
				n++
			}
		}
	}
	oldPhase := `CASE WHEN object_kind = 'choir.event' THEN ` + ChoirEventPhaseSQLFor("body", "metadata") + ` END`
	oldKind := `CASE WHEN object_kind = 'choir.event' THEN (` + ChoirEventKindSQL + `) END`
	read := func(db *sql.DB, query string) map[string]string {
		rows, err := db.Query(query)
		if err != nil {
			t.Fatalf("%v\n%s", err, query)
		}
		defer rows.Close()
		out := map[string]string{}
		for rows.Next() {
			var id, v string
			if err := rows.Scan(&id, &v); err != nil {
				t.Fatal(err)
			}
			out[id] = v
		}
		return out
	}
	old := read(oldDB, `SELECT canonical_id, quote(og_rec_claim)||typeof(og_rec_claim)||'|'||quote(`+oldPhase+`)||typeof(`+oldPhase+`)||'|'||quote(`+oldKind+`)||typeof(`+oldKind+`) FROM og_objects`)
	cur := read(newDB, `SELECT canonical_id, quote(og_rec_claim)||typeof(og_rec_claim)||'|'||quote(event_phase)||typeof(event_phase)||'|'||quote(event_kind)||typeof(event_kind) FROM og_objects`)
	if len(old) != n || len(cur) != n {
		t.Fatalf("rows old=%d new=%d want %d", len(old), len(cur), n)
	}
	for id, w := range old {
		if cur[id] != w {
			t.Errorf("%s: new %s, old %s", id, cur[id], w)
		}
	}
	// The retire predicates select the same rows, kind and phase together,
	// with text and numeric kind arguments.
	for _, kindArg := range []any{"tool.result", 5, "5", "x"} {
		for _, phase := range []string{"", " AND (" + ChoirEventPhaseSQLFor("body", "metadata") + ") = ''", " AND (" + ChoirEventPhaseSQLFor("body", "metadata") + ") <> ''"} {
			newPhase := strings.NewReplacer(ChoirEventPhaseSQLFor("body", "metadata"), "event_phase").Replace(phase)
			oldIDs := idsOf(t, oldDB, `SELECT canonical_id FROM og_objects WHERE object_kind = 'choir.event' AND (`+ChoirEventKindSQL+`) IN (?)`+phase+` ORDER BY canonical_id`, kindArg)
			newIDs := idsOf(t, newDB, `SELECT canonical_id FROM og_objects INDEXED BY `+ChoirEventRetireIndex+` WHERE object_kind = 'choir.event' AND event_kind IN (?)`+newPhase+` ORDER BY canonical_id`, kindArg)
			if fmt.Sprint(oldIDs) != fmt.Sprint(newIDs) {
				t.Errorf("retire kind %v%s:\n new %v\n old %v", kindArg, newPhase, newIDs, oldIDs)
			}
		}
	}
}

func idsOf(t *testing.T, db *sql.DB, query string, args ...any) []string {
	t.Helper()
	rows, err := db.Query(query, args...)
	if err != nil {
		t.Fatalf("%v\n%s", err, query)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		out = append(out, id)
	}
	return out
}

// The mark identities of the four families at the split: unchanged from the
// pre-split code, so TrustDerivedKeyMarks trusts the copied marks at the
// first split boot.
func TestMarkIdentityGolden(t *testing.T) {
	want := map[string]string{
		"actor_wake_outbox.projected": "og_key_index mark g1 ecb480a78a946ea9",
		"run.agent_id":                "og_key_index mark g1 25fa1e738a5bb1aa",
		"commitment_record.addressee": "og_key_index mark g1 1a5fad15f41c6dc2",
		"source_ref.doc_id":           "og_key_index mark g1 f0a841878f6483da",
	}
	if len(derivedFamilies) != len(want) {
		t.Fatalf("%d families, golden has %d", len(derivedFamilies), len(want))
	}
	for i := range derivedFamilies {
		fam := &derivedFamilies[i]
		if got := fam.markIdentity(); got != want[fam.Name] {
			t.Errorf("%s mark identity %s, want %s", fam.Name, got, want[fam.Name])
		}
	}
}

// No statement may REPLACE a row of a split table: REPLACE deletes and
// reinserts, which gives the header a new rowid and orphans or misattaches
// its body.
func TestNoReplaceOnSplitTables(t *testing.T) {
	replace := regexp.MustCompile(`(?is)\b(REPLACE\s+INTO|OR\s+REPLACE|ON\s+CONFLICT\s+REPLACE)\b`)
	tables := regexp.MustCompile(`\b(og_objects|og_bodies|choir_events|choir_event_bodies)\b`)
	for _, dir := range []string{".", filepath.Join("..", "store")} {
		files, err := filepath.Glob(filepath.Join(dir, "*.go"))
		if err != nil {
			t.Fatal(err)
		}
		for _, file := range files {
			src, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			for _, loc := range replace.FindAllIndex(src, -1) {
				end := min(loc[1]+200, len(src))
				if tables.Match(src[loc[1]:end]) {
					t.Errorf("%s: REPLACE near a split table: %q", file, src[loc[0]:end])
				}
			}
		}
	}
}

// A row deleted, and its obj_id reused, between the two phases of
// ListObjectsByKindOwnerWhere does not give a header another row's body:
// both phases read one snapshot.
func TestListObjectsByKindOwnerWhereOneTx(t *testing.T) {
	ctx := context.Background()
	write, read := openTestDBs(t)
	writer := NewSQLStore(write)
	if err := writer.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}
	reader := NewSQLStore(read)
	at := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	writeObj(t, write, splitObj("old", "choir.run", "o", `{"who":"old"}`, `{}`, at))
	victim := writeObj(t, write, splitObj("victim", "choir.run", "o", `{"who":"victim"}`, `{}`, at.Add(time.Second)))
	var reused int64
	got, err := reader.ListObjectsByKindOwnerWhere(ctx, "choir.run", "o", -1, func(h HeaderRow) (bool, error) {
		if h.CanonicalID == "victim" && reused == 0 {
			if h.Body != nil {
				return false, fmt.Errorf("phase 1 saw a body")
			}
			if _, err := write.Exec(`DELETE FROM og_objects WHERE canonical_id = 'victim'`); err != nil {
				return false, err
			}
			reused = writeObj(t, write, splitObj("intruder", "choir.run", "o", `{"who":"intruder"}`, `{}`, at.Add(2*time.Second)))
		}
		return true, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if reused != victim {
		t.Fatalf("premise: the intruder took obj_id %d, not the victim's %d", reused, victim)
	}
	if len(got) != 2 {
		t.Fatalf("got %d objects", len(got))
	}
	for _, obj := range got {
		if want := `{"who":"` + obj.CanonicalID + `"}`; string(obj.Body) != want {
			t.Fatalf("%s got body %s", obj.CanonicalID, obj.Body)
		}
	}
	// The keep filter and the limit.
	got, err = reader.ListObjectsByKindOwnerWhere(ctx, "choir.run", "o", 1, nil)
	if err != nil || len(got) != 1 || got[0].CanonicalID != "intruder" {
		t.Fatalf("limit 1 = %v (%v)", got, err)
	}
	if _, err := reader.ListObjectsByKindOwnerWhere(ctx, "choir.run", "o", -1, func(HeaderRow) (bool, error) { return false, fmt.Errorf("boom") }); err == nil {
		t.Fatal("a keep error was swallowed")
	}
}
