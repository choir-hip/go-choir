package objectgraph

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// The root desk reads what changed, not every document of the computer. Ways
// the query can fail, written before it: it walks the whole kind (a scan of
// every document object on a computer with thousands), returns rows of
// another owner or computer, or reads them in another order than oldest
// change first with a stable tie break.
func TestChangedSinceReadsTheRangeInIndexOrder(t *testing.T) {
	db := openTestDB(t)
	store := NewSQLStore(db)
	ctx := context.Background()
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}
	at := time.Now().UTC().Truncate(time.Second)
	for i := 0; i < 20; i++ {
		putTestObject(t, store, fmt.Sprintf("doc-%02d", i), commitmentRecordKind, map[string]any{}, `{}`, at.Add(time.Duration(i)*time.Second))
	}
	// A row of another computer in the range is not returned.
	other := Object{CanonicalID: "doc-other", ObjectKind: commitmentRecordKind, OwnerID: "owner", ComputerID: "elsewhere", VersionID: "v", ContentHash: "h", Body: []byte(`{}`), Metadata: []byte(`{}`), CreatedAt: at, UpdatedAt: at.Add(10 * time.Second)}
	if err := store.PutObject(ctx, other); err != nil {
		t.Fatal(err)
	}
	query, args := changedSinceSQL(at.Add(5*time.Second), ChangeCursor{}, 5)
	assertIndexedPlan(t, explainPlan(t, db, query, args...), "idx_og_objects_kind_owner_updated", false)

	got, err := store.ListObjectsChangedSince(ctx, string(commitmentRecordKind), "owner", "computer", at.Add(5*time.Second), ChangeCursor{}, 4)
	if err != nil || len(got) != 4 || got[0].CanonicalID != "doc-05" || got[3].CanonicalID != "doc-08" {
		t.Fatalf("first page = %d rows %v, %v; want doc-05..doc-08 oldest first", len(got), ids(got), err)
	}
	last := got[len(got)-1]
	next, err := store.ListObjectsChangedSince(ctx, string(commitmentRecordKind), "owner", "computer", at.Add(5*time.Second), ChangeCursor{At: last.UpdatedAt, ID: last.CanonicalID}, 4)
	if err != nil || len(next) != 4 || next[0].CanonicalID != "doc-09" {
		t.Fatalf("page after the cursor = %v, %v; want it to start at doc-09", ids(next), err)
	}
	for _, o := range next {
		if o.CanonicalID == "doc-other" {
			t.Fatal("a row of another computer was returned")
		}
	}
}

func ids(objs []Object) []string {
	out := make([]string, 0, len(objs))
	for _, o := range objs {
		out = append(out, o.CanonicalID)
	}
	return out
}
