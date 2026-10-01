package objectgraph

import (
	"context"
	"testing"
)

// Repro: 12-column explicit INSERT ON DUP KEY UPDATE against the full
// og_objects schema after generated columns exist (the EnsureSchema path).
func TestGeneratedColumnInsertRepro(t *testing.T) {
	db := openTestDoltDB(t)
	store := NewDoltStore(db)
	ctx := context.Background()
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("schema: %v", err)
	}
	if !store.metaColumns["run_id"] {
		t.Skipf("generated columns not created: %v", store.metaColumns)
	}
	obj := Object{
		CanonicalID: "repro-1",
		ObjectKind:  ObjectKind("choir.run.event"),
		OwnerID:     "owner",
		ComputerID:  "comp",
		VersionID:   "v1",
		ContentHash: "h",
		Body:        []byte("{}"),
		Metadata:    []byte(`{"run_id":"r1"}`),
	}
	if err := store.PutBatch(ctx, Batch{Objects: []Object{obj}}); err != nil {
		t.Fatalf("put batch: %v", err)
	}
	got, err := store.ListObjectsByMetadata(ctx, "choir.run.event", "$.run_id", "r1", 10)
	if err != nil {
		t.Fatalf("list by metadata: %v", err)
	}
	if len(got) != 1 || got[0].CanonicalID != "repro-1" {
		t.Fatalf("indexed lookup returned %+v", got)
	}
	// Fallback path still works for unmapped fields.
	got, err = store.ListObjectsByMetadata(ctx, "choir.run.event", "$.nope", "r1", 10)
	if err != nil {
		t.Fatalf("fallback metadata query: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty, got %+v", got)
	}
}
