package platform

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// Store B retrieval rung 1 (problem record
// docs/archive/problems/corpus-retrieval-windowed-unindexed-untimed-2026-10-14.md).
// Failure modes guarded here, written before the code:
//   1. the time-bounded ladder changes an answer (boundary rows, future
//      rows, rows older than every window, ties, every limit);
//   2. a rewritten query does not use its index, or an added index flips
//      another query's plan;
//   3. the lookup key makes a write fail (upsert, invalid or non-string
//      metadata, long values);
//   4. the lookup key misses a row the JSON scan finds (truncated key,
//      numeric value, objects written before the migration, a changed
//      value, a deleted object).

func putTestObject(t *testing.T, og *ObjectGraphStore, kind, owner, computer, suffix string, updated time.Time, metadata map[string]any) objectgraph.Object {
	t.Helper()
	id, err := objectgraph.BuildCanonicalID(objectgraph.ObjectKind(kind), owner, suffix)
	if err != nil {
		t.Fatalf("BuildCanonicalID: %v", err)
	}
	raw, _ := json.Marshal(metadata)
	obj := objectgraph.Object{CanonicalID: id, ObjectKind: objectgraph.ObjectKind(kind), OwnerID: owner, ComputerID: computer,
		ContentHash: "h-" + suffix, Metadata: raw, CreatedAt: updated, UpdatedAt: updated}
	if err := og.PutObject(context.Background(), obj); err != nil {
		t.Fatalf("PutObject %s: %v", suffix, err)
	}
	return obj
}

func ids(objs []objectgraph.Object) []string {
	out := make([]string, len(objs))
	for i, o := range objs {
		out[i] = o.CanonicalID
	}
	return out
}

// TestListObjectsWindowedEqualsUnbounded: failure mode 1.
func TestListObjectsWindowedEqualsUnbounded(t *testing.T) {
	store, _ := openTestPlatformStore(t)
	og := NewObjectGraphStore(store)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	ages := []time.Duration{
		-2 * time.Hour, // a future row
		0, 0, 0,        // ties at now
		2 * time.Hour,
		24*time.Hour - time.Second, 24 * time.Hour, 24*time.Hour + time.Second, // the first boundary
		3 * 24 * time.Hour,
		7 * 24 * time.Hour, // the second boundary
		8 * 24 * time.Hour,
		40 * 24 * time.Hour,
		365 * 24 * time.Hour, 366 * 24 * time.Hour,
		1000 * 24 * time.Hour, 1000 * 24 * time.Hour, // ties older than every window
	}
	for i, age := range ages {
		putTestObject(t, og, "choir.web_capture", "owner-a", "computer-a", fmt.Sprintf("c%02d", i), now.Add(-age), map[string]any{"i": i})
	}
	// Noise: another computer, another owner, another kind.
	putTestObject(t, og, "choir.web_capture", "owner-a", "computer-b", "other-computer", now, map[string]any{})
	putTestObject(t, og, "choir.web_capture", "owner-b", "computer-a", "other-owner", now, map[string]any{})
	putTestObject(t, og, "choir.publication", "owner-a", "computer-a", "other-kind", now, map[string]any{"publication_id": "p"})

	unbounded := func(filter objectgraph.ListFilter, limit int) []string {
		query, args := listObjectsWindowSQL(filter, newestWindow{}, limit)
		rows, err := store.corpus().QueryContext(ctx, query, args...)
		if err != nil {
			t.Fatalf("unbounded query: %v", err)
		}
		defer rows.Close()
		var out []string
		for rows.Next() {
			obj, _, err := scanObjectGraphObject(rows)
			if err != nil {
				t.Fatalf("scan: %v", err)
			}
			out = append(out, obj.CanonicalID)
		}
		return out
	}
	filters := []objectgraph.ListFilter{
		{Kind: "choir.web_capture", OwnerID: "owner-a", ComputerID: "computer-a"},
		{Kind: "choir.web_capture", OwnerID: "owner-a"},
		{Kind: "choir.web_capture"},
		{OwnerID: "owner-a"},
	}
	for _, f := range filters {
		for limit := 1; limit <= len(ages)+4; limit++ {
			f.Limit = limit
			got, err := og.ListObjects(ctx, f)
			if err != nil {
				t.Fatalf("ListObjects: %v", err)
			}
			want := unbounded(f, limit)
			if strings.Join(ids(got), ",") != strings.Join(want, ",") {
				t.Fatalf("filter %+v limit %d:\n windowed  %v\n unbounded %v", f, limit, ids(got), want)
			}
		}
	}
	shapes := store.RetrievalSnapshot().Ops[retrievalListObjects].Shapes
	if shapes["window_24h"] == 0 || shapes["window_all"] == 0 {
		t.Fatalf("ladder shapes %v: want reads that stopped in the first window and reads that reached the end", shapes)
	}
}

// TestFrontPublicationWindowedEqualsUnbounded: failure mode 1 for the
// cursor-paged front list.
func TestFrontPublicationWindowedEqualsUnbounded(t *testing.T) {
	store, root := openTestPlatformStore(t)
	svc := NewService(store, root+"/artifacts", "")
	og := NewObjectGraphStore(store)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	for i, age := range []time.Duration{0, 0, time.Hour, 23 * time.Hour, 24 * time.Hour, 25 * time.Hour, 6 * 24 * time.Hour, 9 * 24 * time.Hour, 400 * 24 * time.Hour, 400 * 24 * time.Hour} {
		putTestObject(t, og, "choir.publication", "owner-a", "", fmt.Sprintf("p%02d", i), now.Add(-age), map[string]any{"publication_id": fmt.Sprintf("pub-%d", i)})
	}
	unbounded := func(after *frontCursor, n int) []string {
		query, args := frontPublicationWindowSQL(after, newestWindow{}, n)
		rows, err := store.corpus().QueryContext(ctx, query, args...)
		if err != nil {
			t.Fatalf("unbounded: %v", err)
		}
		defer rows.Close()
		var out []string
		for rows.Next() {
			var id, meta string
			var at time.Time
			if err := rows.Scan(&id, &meta, &at); err != nil {
				t.Fatalf("scan: %v", err)
			}
			out = append(out, id)
		}
		return out
	}
	for n := 1; n <= 12; n++ {
		var after *frontCursor
		for page := 0; page < 12; page++ {
			got, err := svc.frontPublicationBatch(ctx, after, n)
			if err != nil {
				t.Fatalf("frontPublicationBatch: %v", err)
			}
			var gotIDs []string
			for _, r := range got {
				gotIDs = append(gotIDs, r.CanonicalID)
			}
			want := unbounded(after, n)
			if strings.Join(gotIDs, ",") != strings.Join(want, ",") {
				t.Fatalf("n %d page %d: windowed %v unbounded %v", n, page, gotIDs, want)
			}
			if len(got) == 0 {
				break
			}
			last := got[len(got)-1]
			after = &frontCursor{UpdatedAt: last.UpdatedAt.Unix(), CanonicalID: last.CanonicalID}
		}
	}
}

// TestLookupKeyNeverFailsAWriteAndNeverMisses: failure modes 3 and 4.
func TestLookupKeyNeverFailsAWriteAndNeverMisses(t *testing.T) {
	store, _ := openTestPlatformStore(t)
	og := NewObjectGraphStore(store)
	ctx := context.Background()
	now := time.Now().UTC()

	// Upsert twice: the key row follows the changed value.
	route := putTestObject(t, og, "choir.public_route", "owner-a", "", "r1", now, map[string]any{"route_path": "/pub/a"})
	putTestObject(t, og, "choir.public_route", "owner-a", "", "r1", now.Add(time.Second), map[string]any{"route_path": "/pub/a2"})
	if _, err := og.GetObjectByMetadata(ctx, "choir.public_route", "$.route_path", "/pub/a"); err != objectgraph.ErrNotFound {
		t.Fatalf("old value after a change: err %v, want ErrNotFound", err)
	}
	got, err := og.GetObjectByMetadata(ctx, "choir.public_route", "$.route_path", "/pub/a2")
	if err != nil || got.CanonicalID != route.CanonicalID {
		t.Fatalf("changed value: %v %v", got.CanonicalID, err)
	}

	long := "/pub/" + strings.Repeat("ü", 400)
	longObj := putTestObject(t, og, "choir.public_route", "owner-a", "", "r-long", now, map[string]any{"route_path": long})
	// Two values sharing the key's 255-character prefix.
	putTestObject(t, og, "choir.public_route", "owner-a", "", "r-long-twin", now, map[string]any{"route_path": long[:len(long)-2] + "x"})
	got, err = og.GetObjectByMetadata(ctx, "choir.public_route", "$.route_path", long)
	if err != nil || got.CanonicalID != longObj.CanonicalID {
		t.Fatalf("long value: %v %v", got.CanonicalID, err)
	}

	numeric := putTestObject(t, og, "choir.publication_check", "owner-a", "", "chk-num", now, map[string]any{"publication_version_id": 123})
	list, err := og.ListObjectsByMetadata(ctx, "choir.publication_check", "$.publication_version_id", "123", 10)
	if err != nil || len(list) != 1 || list[0].CanonicalID != numeric.CanonicalID {
		t.Fatalf("numeric value: %v %v", ids(list), err)
	}

	// Metadata the key cannot read still writes.
	id, _ := objectgraph.BuildCanonicalID("choir.public_route", "owner-a", "r-bad")
	if err := og.PutObject(ctx, objectgraph.Object{CanonicalID: id, ObjectKind: "choir.public_route", OwnerID: "owner-a", ContentHash: "h",
		Metadata: json.RawMessage(`{"route_path": null}`), CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("null field write: %v", err)
	}

	// Several checks per version, newest first, through the key.
	for i := 0; i < 3; i++ {
		putTestObject(t, og, "choir.publication_check", "owner-a", "", fmt.Sprintf("chk-%d", i), now.Add(time.Duration(i)*time.Minute), map[string]any{"publication_version_id": "v1"})
	}
	list, err = og.ListObjectsByMetadata(ctx, "choir.publication_check", "$.publication_version_id", "v1", 2)
	if err != nil || len(list) != 2 || !strings.HasSuffix(list[0].CanonicalID, "chk-2") {
		t.Fatalf("checks newest first: %v %v", ids(list), err)
	}

	// Delete removes the key.
	if err := og.DeleteObject(ctx, longObj.CanonicalID); err != nil {
		t.Fatalf("DeleteObject: %v", err)
	}
	var n int
	if err := store.corpus().QueryRowContext(ctx, `SELECT COUNT(*) FROM og_lookup_keys WHERE canonical_id = ?`, longObj.CanonicalID).Scan(&n); err != nil || n != 0 {
		t.Fatalf("key after delete: %d %v", n, err)
	}

	shapes := store.RetrievalSnapshot().Ops[retrievalGetByMetadata].Shapes
	if shapes["lookup_key"] == 0 || shapes["json_scan"] != 0 {
		t.Fatalf("get_by_metadata shapes %v: want every registered lookup through the key", shapes)
	}
}

// TestLookupKeyBackfillsObjectsWrittenBeforeTheMigration: failure mode 4.
func TestLookupKeyBackfillsObjectsWrittenBeforeTheMigration(t *testing.T) {
	store, _ := openTestPlatformStore(t)
	og := NewObjectGraphStore(store)
	ctx := context.Background()
	// Simulate a store from before the migration: no key rows, keys off.
	store.retrievalState.lookupKeysReady.Store(false)
	store.retrievalState.lookupKeyLive.Store(false)
	if _, err := store.corpus().ExecContext(ctx, `DROP TABLE og_lookup_keys`); err != nil {
		t.Fatalf("drop: %v", err)
	}
	obj := putTestObject(t, og, "choir.publication_proposal", "owner-a", "", "prop-1", time.Now().UTC(), map[string]any{"proposal_id": "readerprop-1"})
	// Before the migration the lookup scans and still answers.
	list, err := og.ListObjectsByMetadata(ctx, "choir.publication_proposal", "$.proposal_id", "readerprop-1", 1)
	if err != nil || len(list) != 1 {
		t.Fatalf("pre-migration scan: %v %v", ids(list), err)
	}
	if err := store.MigrateRetrieval(ctx); err != nil {
		t.Fatalf("MigrateRetrieval: %v", err)
	}
	if err := store.MigrateRetrieval(ctx); err != nil {
		t.Fatalf("MigrateRetrieval again (idempotent): %v", err)
	}
	list, err = og.ListObjectsByMetadata(ctx, "choir.publication_proposal", "$.proposal_id", "readerprop-1", 1)
	if err != nil || len(list) != 1 || list[0].CanonicalID != obj.CanonicalID {
		t.Fatalf("post-migration key: %v %v", ids(list), err)
	}
	if got := store.RetrievalSnapshot().Ops[retrievalListByMetadata].Shapes; got["lookup_key"] != 1 || got["json_scan"] != 1 {
		t.Fatalf("shapes %v: want one scan before and one key seek after", got)
	}
}

// explainPlan returns the EXPLAIN text of query with args, planned with
// sequential scans disabled: the tables are small, so only an index that
// can serve the query shape yields an index plan.
func explainPlan(t *testing.T, db *sql.DB, query string, args ...any) string {
	t.Helper()
	ctx := context.Background()
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("explain conn: %v", err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, "ANALYZE"); err != nil {
		t.Fatalf("explain analyze: %v", err)
	}
	if _, err := conn.ExecContext(ctx, "SET enable_seqscan = off"); err != nil {
		t.Fatalf("explain set: %v", err)
	}
	defer func() { _, _ = conn.ExecContext(ctx, "RESET enable_seqscan") }()
	rows, err := conn.QueryContext(ctx, "EXPLAIN "+query, args...)
	if err != nil {
		t.Fatalf("explain %q: %v", query, err)
	}
	defer rows.Close()
	var lines []string
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			t.Fatalf("scan plan: %v", err)
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

// assertPlan fails unless the plan reads table through some index and never
// scans a table in full. columns name the indexed columns for the reader;
// assertPlanIndex pins a specific index.
func assertPlan(t *testing.T, name, plan, table string, columns ...string) {
	t.Helper()
	re := regexp.MustCompile(`Index (Only )?Scan( Backward)? using \S+ on ` + regexp.QuoteMeta(table) + `\b|Bitmap Heap Scan on ` + regexp.QuoteMeta(table) + `\b`)
	if !re.MatchString(plan) {
		t.Fatalf("%s: plan does not read %s through an index (%v):\n%s", name, table, columns, plan)
	}
	if strings.Contains(plan, "Seq Scan") {
		t.Fatalf("%s: full table read:\n%s", name, plan)
	}
}

// assertPlanIndex fails unless the plan reads through one of the named
// indexes and never scans a table in full.
func assertPlanIndex(t *testing.T, name, plan string, indexes ...string) {
	t.Helper()
	used := false
	for _, index := range indexes {
		re := regexp.MustCompile(`(Index Scan|Index Only Scan)( Backward)? using ` + regexp.QuoteMeta(index) + `\b|Bitmap Index Scan on ` + regexp.QuoteMeta(index) + `\b`)
		if re.MatchString(plan) {
			used = true
		}
	}
	if !used {
		t.Fatalf("%s: plan does not use any of %v:\n%s", name, indexes, plan)
	}
	if strings.Contains(plan, "Seq Scan") {
		t.Fatalf("%s: full table read:\n%s", name, plan)
	}
}

// TestRetrievalQueryPlans: failure mode 2. Every rewritten query, and the
// unchanged edge and primary-key reads beside the new indexes, keep an
// index plan. Rows of several kinds are present so the planner has
// choices.
func TestRetrievalQueryPlans(t *testing.T) {
	store, _ := openTestPlatformStore(t)
	og := NewObjectGraphStore(store)
	ctx := context.Background()
	now := time.Now().UTC()
	for i := 0; i < 60; i++ {
		putTestObject(t, og, "choir.web_capture", "owner-a", fmt.Sprintf("computer-%d", i%3), fmt.Sprintf("c%d", i), now.Add(-time.Duration(i)*time.Hour), map[string]any{"i": i})
		putTestObject(t, og, "choir.publication", "owner-a", "", fmt.Sprintf("p%d", i), now.Add(-time.Duration(i)*time.Hour), map[string]any{"publication_id": fmt.Sprintf("pub-%d", i)})
		putTestObject(t, og, "choir.public_route", "owner-a", "", fmt.Sprintf("r%d", i), now, map[string]any{"route_path": fmt.Sprintf("/pub/%d", i)})
		if err := og.PutEdge(ctx, objectgraph.Edge{EdgeID: fmt.Sprintf("e%d", i), FromID: fmt.Sprintf("from-%d", i%7), ToID: fmt.Sprintf("to-%d", i%5),
			Kind: "reviews", Metadata: json.RawMessage(`{}`), CreatedAt: now}); err != nil {
			t.Fatalf("PutEdge: %v", err)
		}
	}
	db := store.corpus()
	w := newestWindow{lo: now.Add(-24 * time.Hour)}
	older := newestWindow{lo: now.Add(-7 * 24 * time.Hour), hi: now.Add(-24 * time.Hour)}

	q, a := listObjectsWindowSQL(objectgraph.ListFilter{Kind: "choir.web_capture", OwnerID: "owner-a", ComputerID: "computer-1"}, w, 1000)
	assertPlanIndex(t, "corpus window, first rung", explainPlan(t, db, q, a...), idxObjectsKindOwnerComputerUpdated)
	q, a = listObjectsWindowSQL(objectgraph.ListFilter{Kind: "choir.web_capture", OwnerID: "owner-a", ComputerID: "computer-1"}, older, 1000)
	assertPlanIndex(t, "corpus window, later rung", explainPlan(t, db, q, a...), idxObjectsKindOwnerComputerUpdated)
	q, a = listObjectsWindowSQL(objectgraph.ListFilter{Kind: "choir.web_capture", OwnerID: "owner-a"}, w, 50)
	assertPlanIndex(t, "owner window", explainPlan(t, db, q, a...), idxObjectsKindOwnerComputerUpdated)

	q, a = frontPublicationWindowSQL(nil, w, 21)
	plan := explainPlan(t, db, q, a...)
	assertPlanIndex(t, "front window", plan, "idx_og_objects_updated", "idx_og_objects_kind_owner", idxObjectsKindOwnerComputerUpdated)
	q, a = frontPublicationWindowSQL(&frontCursor{UpdatedAt: now.Unix(), CanonicalID: "obj:x"}, older, 21)
	plan = explainPlan(t, db, q, a...)
	assertPlanIndex(t, "front window after a cursor", plan, "idx_og_objects_updated", "idx_og_objects_kind_owner", idxObjectsKindOwnerComputerUpdated)

	assertPlanIndex(t, "lookup key seek", explainPlan(t, db, lookupKeySeekSQL, "choir.public_route", "/pub/3", ogLookupKeyMaxIDs), "og_lookup_keys_pkey")
	where, args, shape, none, err := og.metadataPredicate(ctx, "choir.public_route", "$.route_path", "/pub/3")
	if err != nil || none || shape != "lookup_key" {
		t.Fatalf("metadataPredicate: %q %v %v", shape, none, err)
	}
	assertPlanIndex(t, "objects named by the key", explainPlan(t, db, `SELECT canonical_id FROM og_objects WHERE `+where+` ORDER BY updated_at DESC LIMIT 1`, args...), "og_objects_pkey")

	assertPlanIndex(t, "provenance edges", explainPlan(t, db, provenanceEdgesSQL, "to-1"), idxEdgesToKind, "idx_og_edges_to")
	assertPlanIndex(t, "edges to an id with a kind", explainPlan(t, db, `SELECT edge_id FROM og_edges WHERE 1=1 AND to_id = ? AND kind = ? ORDER BY created_at LIMIT ?`, "to-1", "reviews", 50), idxEdgesToKind, "idx_og_edges_to")
	// Unchanged reads beside the new indexes keep their plans (§4 G3).
	assertPlanIndex(t, "edges from an id", explainPlan(t, db, `SELECT edge_id FROM og_edges WHERE from_id = ? AND tombstone = FALSE ORDER BY created_at`, "from-1"), "idx_og_edges_from")
	assertPlanIndex(t, "edges from an id with a kind", explainPlan(t, db, `SELECT edge_id FROM og_edges WHERE from_id = ? AND kind = ? AND tombstone = FALSE ORDER BY created_at`, "from-1", "reviews"), "idx_og_edges_from")
	toPlan := explainPlan(t, db, `SELECT edge_id FROM og_edges WHERE 1=1 AND to_id = ? ORDER BY created_at LIMIT ?`, "to-1", 50)
	assertPlanIndex(t, "edges to an id", toPlan, idxEdgesToKind, "idx_og_edges_to")
	assertPlanIndex(t, "object by primary key", explainPlan(t, db, `SELECT canonical_id FROM og_objects WHERE canonical_id = ?`, "obj:x"), "og_objects_pkey")
}
