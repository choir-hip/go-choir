package objectgraph

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"
)

// Generated-column access paths for the hot record reads
// (docs/problems/guest-cpu-is-unindexed-store-scans-2026-10-14.md). Ways
// they can fail, written before the code:
//
//  1. A new column refuses a write: a long value, a non-JSON body.
//  2. The indexed read returns other rows, or another order, than the JSON
//     read it replaces (collation, trailing spaces, case, JSON null,
//     numbers, a missing field, a value longer than the column).
//  3. The planner ignores the index or sorts the whole range; a later index
//     or engine upgrade flips the plan silently.
//  4. The migration fails on a store that already has rows, or is not
//     idempotent across opens.

// explainPlan returns the engine's plan for query, one line per node.
func explainPlan(t *testing.T, db *sql.DB, query string, args ...any) string {
	t.Helper()
	rows, err := db.Query("EXPLAIN PLAN "+query, args...)
	if err != nil {
		t.Fatalf("explain: %v\n%s", err, query)
	}
	defer rows.Close()
	var b strings.Builder
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			t.Fatalf("scan plan: %v", err)
		}
		b.WriteString(line + "\n")
	}
	return b.String()
}

// assertIndexedPlan fails unless the plan reads og_objects through index
// (its columns, comma-joined as EXPLAIN prints them) and, when sortFree,
// has no TopN or Sort node: the index order serves the ORDER BY.
func assertIndexedPlan(t *testing.T, plan, index string, sortFree bool) {
	t.Helper()
	if !strings.Contains(plan, "IndexedTableAccess(og_objects)") {
		t.Fatalf("plan does not use an index on og_objects:\n%s", plan)
	}
	if !strings.Contains(plan, "index: ["+index+"]") {
		t.Fatalf("plan does not use index [%s]:\n%s", index, plan)
	}
	if strings.Contains(plan, "Table(og_objects)") && !strings.Contains(plan, "IndexedTable") {
		t.Fatalf("plan scans og_objects:\n%s", plan)
	}
	if sortFree && (strings.Contains(plan, "TopN") || strings.Contains(plan, "Sort(")) {
		t.Fatalf("plan sorts instead of reading in index order:\n%s", plan)
	}
}

func recordIndex(column string) string {
	return "og_objects.object_kind,og_objects." + column + ",og_objects.canonical_id"
}

func putTestObject(t *testing.T, store *DoltStore, id string, kind ObjectKind, metadata map[string]any, body string, at time.Time) {
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
	db := openTestDoltDB(t)
	store := NewDoltStore(db)
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
		if !store.live(column) {
			t.Fatalf("column %s not live: %v", column, store.liveNames())
		}
		query, args := store.metadataPageSQL(kind, "$."+field, "v", "", 256)
		assertIndexedPlan(t, explainPlan(t, db, query, args...), recordIndex(column), true)
		query, args = store.metadataListSQL(kind, "$."+field, "v", 50)
		assertIndexedPlan(t, explainPlan(t, db, query, args...), recordIndex(column), false)
	}
	// A legacy column still seeks its (kind, column, updated_at) range.
	query, args := store.metadataPageSQL(kind, "$.run_id", "r", "", 256)
	assertIndexedPlan(t, explainPlan(t, db, query, args...), "og_objects.object_kind,og_objects.og_meta_run_id,og_objects.updated_at", false)
	// Another kind, or a value too long for the column, keeps the JSON path.
	if _, ok := store.metadataColumnFor("choir.inbox_head", "$.inbox", "management:o"); ok {
		t.Fatal("record column used for another kind")
	}
	if _, ok := store.metadataColumnFor(kind, "$.inbox", strings.Repeat("x", generatedColumnMaxChars)); ok {
		t.Fatal("truncated column used for a value it cannot hold whole")
	}
}

func TestRecordColumnsMatchJSONReads(t *testing.T) {
	ctx := context.Background()
	db := openTestDoltDB(t)
	indexed := NewDoltStore(db)
	if err := indexed.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}
	// A store over the same table that never learned the columns: the JSON
	// reads every lookup used before.
	unindexed := NewDoltStore(db)
	unindexed.ShareEngineMutex(indexed)

	p255 := strings.Repeat("p", generatedColumnMaxChars)
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
	pageAll := func(store *DoltStore, kind, path, value string) []string {
		var out []string
		after := ""
		for {
			page, err := store.ListObjectsByMetadataPage(ctx, kind, path, value, after, 7)
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
				want := pageAll(unindexed, string(kind), "$."+field, value)
				got := pageAll(indexed, string(kind), "$."+field, value)
				if fmt.Sprint(got) != fmt.Sprint(want) {
					t.Fatalf("page %s %s=%q:\n got %v\nwant %v", kind, field, value, got, want)
				}
				wantList, err := unindexed.ListObjectsByMetadata(ctx, string(kind), "$."+field, value, 40)
				if err != nil {
					t.Fatal(err)
				}
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

func TestRecordColumnsMigrateExistingStore(t *testing.T) {
	ctx := context.Background()
	db := openTestDoltDB(t)
	// An existing store from before the record columns: the base schema and
	// the three legacy columns, with rows in it.
	if _, err := db.Exec(SchemaDDL); err != nil {
		t.Fatal(err)
	}
	for _, mc := range metadataIndexColumns[:3] {
		column := mc.generatedColumn()
		if _, err := db.Exec(`ALTER TABLE og_objects ADD COLUMN ` + column.Name + ` ` + column.Def); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`CREATE INDEX ` + mc.indexName() + ` ON og_objects(` + mc.indexColumns() + `)`); err != nil {
			t.Fatal(err)
		}
	}
	old := NewDoltStore(db)
	at := time.Now().UTC()
	for i := 0; i < 30; i++ {
		putTestObject(t, old, fmt.Sprintf("old-%02d", i), commitmentRecordKind, map[string]any{"inbox": fmt.Sprintf("texture:doc-%d", i%3)}, `{}`, at)
	}
	// Opening a store with rows does not rewrite it: the record paths stay
	// on JSON (and still answer correctly) until MigrateAccessPaths.
	opened := NewDoltStore(db)
	if err := opened.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}
	if opened.live("og_rec_inbox") || opened.live(recordClaimIndex) || !opened.live("og_meta_run_id") {
		t.Fatalf("open of a store with rows: live %v, want the legacy columns only", opened.liveNames())
	}
	if _, ok, err := opened.ListCommitmentClaimObjects(ctx, "owner", 10); ok || err != nil {
		t.Fatalf("claim index used before the migration: ok=%v err=%v", ok, err)
	}
	if got, err := opened.ListObjectsByMetadataPage(ctx, string(commitmentRecordKind), "$.inbox", "texture:doc-1", "", 100); err != nil || len(got) != 10 {
		t.Fatalf("JSON read before the migration = %d, %v", len(got), err)
	}
	reader := NewDoltStore(db) // a second handle on the engine, like the read pool
	reader.ShareEngineMutex(opened)
	reader.InheritMetadataColumns(opened)
	if _, err := opened.MigrateAccessPaths(ctx, nil); err != nil {
		t.Fatal(err)
	}
	reader.InheritMetadataColumns(opened)
	if !reader.live(recordClaimIndex) {
		t.Fatalf("read handle did not learn the migrated paths: %v", reader.liveNames())
	}
	for open := 0; open < 2; open++ { // idempotent across opens
		store := NewDoltStore(db)
		if err := store.EnsureSchema(ctx); err != nil {
			t.Fatalf("open %d: %v", open, err)
		}
		if _, err := store.MigrateAccessPaths(ctx, nil); err != nil {
			t.Fatalf("open %d: %v", open, err)
		}
		for _, mc := range metadataIndexColumns {
			if !store.live(mc.Column) {
				t.Fatalf("open %d: %s not live: %v", open, mc.Column, store.liveNames())
			}
		}
		if !store.live(recordInboxSeqIndex) || !store.live(recordClaimIndex) {
			t.Fatalf("open %d: record access paths not live: %v", open, store.liveNames())
		}
		claims, ok, err := store.ListCommitmentClaimObjects(ctx, "owner", 100)
		if err != nil || !ok || len(claims) != 30 {
			t.Fatalf("open %d: migrated claims = %d ok=%v err=%v", open, len(claims), ok, err)
		}
		got, err := store.ListObjectsByMetadataPage(ctx, string(commitmentRecordKind), "$.inbox", "texture:doc-1", "", 100)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 10 {
			t.Fatalf("open %d: migrated rows by inbox = %d, want 10", open, len(got))
		}
	}
}

// A value longer than a record column, on a record, still writes: the
// column holds its first 255 characters and lookups of it keep the JSON
// path.
func TestRecordColumnsAcceptLongValues(t *testing.T) {
	ctx := context.Background()
	db := openTestDoltDB(t)
	store := NewDoltStore(db)
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
	db := openTestDoltDB(t)
	store := NewDoltStore(db)
	if err := store.EnsureSchema(context.Background()); err != nil {
		t.Fatal(err)
	}
	at := time.Now().UTC()
	for i := 0; i < 20; i++ {
		putTestObject(t, store, fmt.Sprintf("rec-%02d", i), commitmentRecordKind,
			map[string]any{"inbox": "management:o", "inbox_seq": i + 1}, `{"record_id":"x"}`, at.Add(time.Duration(i)*time.Second))
	}
	if !store.live(recordInboxSeqIndex) || !store.live(recordClaimIndex) {
		t.Fatalf("record access paths not live: %v", store.liveNames())
	}
	seqIndex := "og_objects.object_kind,og_objects.og_rec_inbox,og_objects.og_rec_inbox_seq"
	for _, through := range []int64{0, 15} {
		query, args := recordsByInboxSeqSQL("management:o", 5, through)
		assertIndexedPlan(t, explainPlan(t, db, query, args...), seqIndex, true)
	}
	query, args := commitmentClaimSQL("owner", 256)
	plan := explainPlan(t, db, query, args...)
	assertIndexedPlan(t, plan, "og_objects.object_kind,og_objects.owner_id,og_objects.og_rec_claim,og_objects.updated_at,og_objects.canonical_id", true)
	if !strings.Contains(plan, "reverse: true") {
		t.Fatalf("claim read is not a reverse index walk:\n%s", plan)
	}
}

// The claim read returns what the body predicate returned: the newest
// records whose body has no directive, missing or empty, in the same order
// (distinct timestamps; equal ones are ordered by canonical id now).
func TestCommitmentClaimObjectsMatchBodyRead(t *testing.T) {
	ctx := context.Background()
	db := openTestDoltDB(t)
	indexed := NewDoltStore(db)
	if err := indexed.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}
	unindexed := NewDoltStore(db)
	unindexed.ShareEngineMutex(indexed)
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
			want, err := unindexed.ListObjectsByOwnerAndBody(ctx, string(commitmentRecordKind), owner, matches, limit)
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
	// No index yet: the caller is told to read its own way.
	if _, ok, err := unindexed.ListCommitmentClaimObjects(ctx, owners[0], 10); ok || err != nil {
		t.Fatalf("unmigrated store claimed the index: ok=%v err=%v", ok, err)
	}
}

// A record whose body is not JSON still writes; it is not a claim. (The
// unindexed predicate failed the whole query on such a row.) A non-numeric
// inbox_seq is 0, outside every range a reader asks for.
func TestRecordColumnsAcceptMalformedRecords(t *testing.T) {
	ctx := context.Background()
	db := openTestDoltDB(t)
	store := NewDoltStore(db)
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
