package objectgraph

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math/rand"
	"regexp"
	"strings"
	"testing"
	"time"
)

// Generated-column access paths for the hot record reads
// (docs/problems/guest-cpu-is-unindexed-store-scans-2026-10-14.md). Ways
// they can fail, written before the code:
//
//  1. A generated column refuses a write: a long value, a non-JSON body.
//  2. The indexed read returns other rows, or another order, than the JSON
//     read it replaces (case, trailing spaces, JSON null, booleans, numbers,
//     a missing field, a long value).
//  3. The planner ignores the index or sorts the whole range; a later index
//     or engine upgrade flips the plan silently.

// explainPlan returns SQLite's query plan for query, one detail line per node.
func explainPlan(t *testing.T, db *sql.DB, query string, args ...any) string {
	t.Helper()
	rows, err := db.Query("EXPLAIN QUERY PLAN "+query, args...)
	if err != nil {
		t.Fatalf("explain: %v\n%s", err, query)
	}
	defer rows.Close()
	var b strings.Builder
	for rows.Next() {
		var id, parent, notUsed int
		var detail string
		if err := rows.Scan(&id, &parent, &notUsed, &detail); err != nil {
			t.Fatalf("scan plan: %v", err)
		}
		b.WriteString(detail + "\n")
	}
	return b.String()
}

// assertIndexedPlan fails unless the plan reads og_objects through the named
// index and, when sortFree, has no temp b-tree for the ORDER BY: the index
// order serves it.
func assertIndexedPlan(t *testing.T, plan, index string, sortFree bool) {
	t.Helper()
	if !regexp.MustCompile(`USING (COVERING )?INDEX ` + index + `\b`).MatchString(plan) {
		t.Fatalf("plan does not use index %s:\n%s", index, plan)
	}
	for _, line := range strings.Split(plan, "\n") {
		if strings.HasPrefix(line, "SCAN og_objects") && !strings.Contains(line, "USING") {
			t.Fatalf("plan scans og_objects:\n%s", plan)
		}
	}
	if sortFree && strings.Contains(plan, "TEMP B-TREE") {
		t.Fatalf("plan sorts instead of reading in index order:\n%s", plan)
	}
}

func recordIndex(column string) string {
	return "idx_og_objects_" + column
}

func putTestObject(t *testing.T, store *SQLStore, id string, kind ObjectKind, metadata map[string]any, body string, at time.Time) {
	t.Helper()
	meta, err := json.Marshal(metadata)
	if err != nil {
		t.Fatal(err)
	}
	obj := Object{CanonicalID: id, ObjectKind: kind, OwnerID: "owner", ComputerID: "computer", VersionID: "v",
		ContentHash: "h-" + id, Body: []byte(body), Metadata: meta, CreatedAt: at, UpdatedAt: at}
	if err := store.PutObject(context.Background(), obj); err != nil {
		t.Fatalf("put %s (%v): %v", id, metadata, err)
	}
}

func TestRecordColumnPlansUseIndexOrder(t *testing.T) {
	db := openTestDB(t)
	store := NewSQLStore(db)
	if err := store.EnsureSchema(context.Background()); err != nil {
		t.Fatal(err)
	}
	at := time.Now().UTC()
	for i := 0; i < 20; i++ {
		putTestObject(t, store, fmt.Sprintf("rec-%02d", i), commitmentRecordKind,
			map[string]any{"inbox": "management:o", "sender_inbox": "source:s1", "conversation_id": "conv:management:o|source:s1", "run_id": "r"}, `{}`, at)
	}
	kind := string(commitmentRecordKind)
	for _, field := range []string{"inbox", "sender_inbox", "conversation_id"} {
		column := "og_rec_" + field
		query, args := metadataPageSQL(kind, "$."+field, "v", "", 256)
		assertIndexedPlan(t, explainPlan(t, db, query, args...), recordIndex(column), true)
		query, args = metadataListSQL(kind, "$."+field, "v", 50)
		assertIndexedPlan(t, explainPlan(t, db, query, args...), recordIndex(column), false)
	}
	// A universal column pages by its keyset index and lists newest first by
	// its recency index.
	query, args := metadataPageSQL(kind, "$.run_id", "r", "", 256)
	assertIndexedPlan(t, explainPlan(t, db, query, args...), "idx_og_objects_og_meta_run_id", true)
	query, args = metadataListSQL(kind, "$.run_id", "r", 50)
	assertIndexedPlan(t, explainPlan(t, db, query, args...), "idx_og_objects_og_meta_run_id_recent", true)
	// Another kind keeps the JSON path.
	if _, ok := metadataColumnFor("choir.inbox_head", "$.inbox"); ok {
		t.Fatal("record column used for another kind")
	}
}

func TestRecordColumnsMatchJSONReads(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	indexed := NewSQLStore(db)
	if err := indexed.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}

	p255 := strings.Repeat("p", 255)
	values := []any{
		"management:o", "Management:o", "management:o ", " management:o", "texture:doc-1", "texture:dóc-ü",
		`quote"d:x`, "", "null", 12, true, nil, map[string]any{"a": 1},
		p255, p255 + "z", p255[:254], strings.Repeat("é", 300),
	}
	lookups := []string{"management:o", "Management:o", "management:o ", "texture:dóc-ü", `quote"d:x`, "", "null", "12", "true",
		p255, p255 + "z", p255[:254], strings.Repeat("é", 300), "absent"}
	kinds := []ObjectKind{commitmentRecordKind, "choir.inbox_head", "choir.event"}
	fields := []string{"inbox", "sender_inbox", "conversation_id"}

	rng := rand.New(rand.NewSource(1014))
	base := time.Now().UTC().Truncate(time.Second).Add(-time.Hour)
	for i := 0; i < 300; i++ {
		metadata := map[string]any{}
		for _, field := range fields {
			if rng.Intn(5) > 0 { // sometimes missing
				metadata[field] = values[rng.Intn(len(values))]
			}
		}
		id := fmt.Sprintf("obj-%03d-%d", rng.Intn(1000), i)
		putTestObject(t, indexed, id, kinds[rng.Intn(len(kinds))], metadata, `{}`, base.Add(time.Duration(i)*time.Second))
		if rng.Intn(10) == 0 { // an upsert moves the row to new values
			metadata["inbox"] = values[rng.Intn(len(values))]
			putTestObject(t, indexed, id, commitmentRecordKind, metadata, `{}`, base.Add(time.Duration(300+i)*time.Second))
		}
	}

	ids := func(objs []Object) []string {
		out := make([]string, len(objs))
		for i, o := range objs {
			out[i] = o.CanonicalID
		}
		return out
	}
	// jsonObjects runs a JSON-path query (the read the generated columns
	// replace) and scans its objects.
	jsonObjects := func(query string, args []any) []Object {
		rows, err := db.QueryContext(ctx, query, args...)
		if err != nil {
			t.Fatalf("json read: %v", err)
		}
		defer rows.Close()
		var out []Object
		for rows.Next() {
			obj, err := scanObject(rows)
			if err != nil {
				t.Fatal(err)
			}
			out = append(out, obj)
		}
		return out
	}
	pageAll := func(jsonPath bool, kind, path, value string) []string {
		var out []string
		after := ""
		for {
			var page []Object
			var err error
			if jsonPath {
				query, args := metadataPageJSONSQL(kind, path, value, after, 7)
				page = jsonObjects(query, args)
			} else {
				page, err = indexed.ListObjectsByMetadataPage(ctx, kind, path, value, after, 7)
			}
			if err != nil {
				t.Fatalf("page %s %s=%q: %v", kind, path, value, err)
			}
			if len(page) == 0 {
				return out
			}
			out = append(out, ids(page)...)
			after = page[len(page)-1].CanonicalID
		}
	}
	matched := 0
	for _, kind := range kinds {
		for _, field := range fields {
			for _, value := range lookups {
				want := pageAll(true, string(kind), "$."+field, value)
				got := pageAll(false, string(kind), "$."+field, value)
				if fmt.Sprint(got) != fmt.Sprint(want) {
					t.Fatalf("page %s %s=%q:\n got %v\nwant %v", kind, field, value, got, want)
				}
				listQuery, listArgs := metadataListJSONSQL(string(kind), "$."+field, value, 40)
				wantList := jsonObjects(listQuery, listArgs)
				gotList, err := indexed.ListObjectsByMetadata(ctx, string(kind), "$."+field, value, 40)
				if err != nil {
					t.Fatal(err)
				}
				if fmt.Sprint(ids(gotList)) != fmt.Sprint(ids(wantList)) {
					t.Fatalf("list %s %s=%q:\n got %v\nwant %v", kind, field, value, ids(gotList), ids(wantList))
				}
				matched += len(want)
			}
		}
	}
	if matched == 0 {
		t.Fatal("fixture matched nothing; the comparison proves nothing")
	}
}

// A value of any length, on a record, writes and is found by its column.
func TestRecordColumnsAcceptLongValues(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	store := NewSQLStore(db)
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}
	long := "texture:" + strings.Repeat("d", 400)
	putTestObject(t, store, "long-1", commitmentRecordKind, map[string]any{"inbox": long, "sender_inbox": long, "conversation_id": long}, `{}`, time.Now().UTC())
	got, err := store.ListObjectsByMetadataPage(ctx, string(commitmentRecordKind), "$.inbox", long, "", 10)
	if err != nil || len(got) != 1 {
		t.Fatalf("long inbox lookup = %v, %v", got, err)
	}
}

func TestRecordSeqAndClaimPlansUseIndexOrder(t *testing.T) {
	db := openTestDB(t)
	store := NewSQLStore(db)
	if err := store.EnsureSchema(context.Background()); err != nil {
		t.Fatal(err)
	}
	at := time.Now().UTC()
	for i := 0; i < 20; i++ {
		putTestObject(t, store, fmt.Sprintf("rec-%02d", i), commitmentRecordKind,
			map[string]any{"inbox": "management:o", "inbox_seq": i + 1}, `{"record_id":"x"}`, at.Add(time.Duration(i)*time.Second))
	}
	seqIndex := recordInboxSeqIndex
	for _, through := range []int64{0, 15} {
		query, args := recordsByInboxSeqSQL("management:o", 5, through)
		assertIndexedPlan(t, explainPlan(t, db, query, args...), seqIndex, true)
	}
	query, args := commitmentClaimSQL("owner", 256)
	plan := explainPlan(t, db, query, args...)
	assertIndexedPlan(t, plan, recordClaimIndex, true)
}

// The claim read returns what the body predicate returned: the newest
// records whose body has no directive, missing or empty, in the same order
// (distinct timestamps; equal ones are ordered by canonical id now).
func TestCommitmentClaimObjectsMatchBodyRead(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	indexed := NewSQLStore(db)
	if err := indexed.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}
	bodies := []string{
		`{"record_id":"a"}`, `{"directive":null}`, `{"directive":""}`, `{"directive":"x"}`, `{"directive":" "}`,
		`{"directive":{"subtype":"cast"}}`, `{"directive":{}}`, `{"directive":[]}`, `{"directive":0}`, `{"directive":false}`,
		`{"nested":{"directive":"x"}}`, `{}`,
	}
	owners := []string{"owner-a", "owner-b"}
	kinds := []ObjectKind{commitmentRecordKind, commitmentRecordKind, "choir.event"}
	rng := rand.New(rand.NewSource(1014))
	base := time.Now().UTC().Truncate(time.Second).Add(-24 * time.Hour)
	for i := 0; i < 250; i++ {
		id := fmt.Sprintf("claim-%03d-%d", rng.Intn(1000), i)
		obj := Object{CanonicalID: id, ObjectKind: kinds[rng.Intn(len(kinds))], OwnerID: owners[rng.Intn(len(owners))], ComputerID: "c",
			VersionID: "v", ContentHash: "h", Body: []byte(bodies[rng.Intn(len(bodies))]), Metadata: []byte(`{}`),
			CreatedAt: base, UpdatedAt: base.Add(time.Duration(i) * time.Second)}
		if err := indexed.PutObject(ctx, obj); err != nil {
			t.Fatalf("put %s: %v", id, err)
		}
		if rng.Intn(8) == 0 { // an upsert changes the body and time
			obj.Body, obj.UpdatedAt = []byte(bodies[rng.Intn(len(bodies))]), base.Add(time.Duration(300+i)*time.Second)
			if err := indexed.PutObject(ctx, obj); err != nil {
				t.Fatalf("upsert %s: %v", id, err)
			}
		}
	}
	matches := []JSONFieldMatch{{JSONPath: "$.directive", Value: "", MissingMatchesEmpty: true}}
	for _, owner := range owners {
		for _, limit := range []int{1, 7, 50, 1000} {
			want, err := indexed.ListObjectsByOwnerAndBody(ctx, string(commitmentRecordKind), owner, matches, limit)
			if err != nil {
				t.Fatal(err)
			}
			got, ok, err := indexed.ListCommitmentClaimObjects(ctx, owner, limit)
			if err != nil || !ok {
				t.Fatalf("claims: ok=%v err=%v", ok, err)
			}
			wantIDs, gotIDs := make([]string, len(want)), make([]string, len(got))
			for i := range want {
				wantIDs[i] = want[i].CanonicalID
			}
			for i := range got {
				gotIDs[i] = got[i].CanonicalID
			}
			if fmt.Sprint(gotIDs) != fmt.Sprint(wantIDs) {
				t.Fatalf("claims of %s limit %d:\n got %v\nwant %v", owner, limit, gotIDs, wantIDs)
			}
			if limit == 1000 && len(want) < 20 {
				t.Fatalf("fixture too thin: %d claims", len(want))
			}
		}
	}
}

// A record whose body is not JSON still writes; it is not a claim. (The
// unindexed predicate failed the whole query on such a row.) A non-numeric
// inbox_seq is 0, outside every range a reader asks for.
func TestRecordColumnsAcceptMalformedRecords(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	store := NewSQLStore(db)
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}
	at := time.Now().UTC()
	putTestObject(t, store, "bad-body", commitmentRecordKind, map[string]any{"inbox": "management:o", "inbox_seq": "seven"}, "not json", at)
	putTestObject(t, store, "good-body", commitmentRecordKind, map[string]any{"inbox": "management:o", "inbox_seq": 3}, `{"record_id":"g"}`, at)
	claims, ok, err := store.ListCommitmentClaimObjects(ctx, "owner", 10)
	if err != nil || !ok || len(claims) != 1 || claims[0].CanonicalID != "good-body" {
		t.Fatalf("claims = %v ok=%v err=%v", claims, ok, err)
	}
	inRange, ok, err := store.ListRecordsByInboxSeq(ctx, "management:o", 0, 0)
	if err != nil || !ok || len(inRange) != 1 || inRange[0].CanonicalID != "good-body" {
		t.Fatalf("records by seq = %v ok=%v err=%v", inRange, ok, err)
	}
}
