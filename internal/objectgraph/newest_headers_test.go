package objectgraph

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

// The newest-headers read serves the event stream_seq seed with bounded
// work (docs/problems/stream-seq-seed-scans-the-event-kind-at-boot-2026-10-23.md).
//
// Ways it can fail, written before the code:
//  1. The engine plans it as it planned the old seed: the kind index, a TopN
//     over every row of a kind. The plan test pins the reverse walk of
//     idx_og_objects_updated, no sort, no kind index.
//  2. The existence probe scans a kind instead of seeking one row.
//  3. It returns rows in the wrong order, ignores the limit, or drops the
//     metadata the seed reads.
//  4. It runs outside the engine mutex, or unnamed, so a stall cannot be
//     attributed.

func newestHeadersStore(t *testing.T) (*DoltStore, time.Time) {
	t.Helper()
	db := openTestDoltDB(t)
	store := NewDoltStore(db)
	if err := store.EnsureSchema(context.Background()); err != nil {
		t.Fatal(err)
	}
	return store, time.Date(2026, 10, 23, 6, 0, 0, 0, time.UTC)
}

func TestNewestObjectHeadersPlanWalksTheTimeIndex(t *testing.T) {
	store, base := newestHeadersStore(t)
	for i := 0; i < 30; i++ {
		kind := ObjectKind("choir.event")
		if i%3 == 0 {
			kind = "choir.run"
		}
		putTestObject(t, store, fmt.Sprintf("o-%02d", i), kind, map[string]any{"stream_seq": i}, `{}`, base.Add(time.Duration(i)*time.Second))
	}
	plan := explainPlan(t, store.db, NewestObjectHeadersSQL, 500)
	assertIndexedPlan(t, plan, "og_objects.updated_at", true)
	if !strings.Contains(plan, "reverse: true") {
		t.Fatalf("plan does not walk the time index newest first:\n%s", plan)
	}
	if strings.Contains(plan, "og_objects.object_kind,og_objects.owner_id") {
		t.Fatalf("plan reads the kind index:\n%s", plan)
	}

	plan = explainPlan(t, store.db, ObjectKindExistsSQL, "choir.event")
	if !strings.Contains(plan, "IndexedTableAccess(og_objects)") || !strings.Contains(plan, "Limit(1)") {
		t.Fatalf("the existence probe is not one index seek:\n%s", plan)
	}
	if strings.Contains(plan, "TopN") || strings.Contains(plan, "Sort(") {
		t.Fatalf("the existence probe sorts:\n%s", plan)
	}
}

func TestNewestObjectHeadersReturnsTheNewestFirst(t *testing.T) {
	ctx := context.Background()
	store, base := newestHeadersStore(t)
	for i := 0; i < 10; i++ {
		kind := ObjectKind("choir.event")
		if i%2 == 1 {
			kind = "choir.run"
		}
		putTestObject(t, store, fmt.Sprintf("o-%02d", i), kind, map[string]any{"stream_seq": i}, `{"big":"body"}`, base.Add(time.Duration(i)*time.Second))
	}
	got, err := store.ListNewestObjectHeaders(ctx, 4)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 4 {
		t.Fatalf("got %d headers, want the limit 4", len(got))
	}
	for i, h := range got {
		want := 9 - i
		wantKind := ObjectKind("choir.event")
		if want%2 == 1 {
			wantKind = "choir.run"
		}
		if h.Kind != wantKind || string(h.Metadata) != fmt.Sprintf(`{"stream_seq":%d}`, want) {
			t.Fatalf("header %d = %s %s, want %s stream_seq %d", i, h.Kind, h.Metadata, wantKind, want)
		}
	}
	if all, err := store.ListNewestObjectHeaders(ctx, 100); err != nil || len(all) != 10 {
		t.Fatalf("a window larger than the store: %d headers, %v; want all 10", len(all), err)
	}
	if st := store.EngineMutexStats()["(*DoltStore).ListNewestObjectHeaders"]; st.Calls != 2 {
		t.Fatalf("the read is not named on the engine mutex: %v", store.EngineMutexStats())
	}
}

func TestHasObjectOfKind(t *testing.T) {
	ctx := context.Background()
	store, base := newestHeadersStore(t)
	putTestObject(t, store, "run-1", "choir.run", map[string]any{}, `{}`, base)
	if ok, err := store.HasObjectOfKind(ctx, "choir.run"); err != nil || !ok {
		t.Fatalf("run kind present: %v, %v", ok, err)
	}
	if ok, err := store.HasObjectOfKind(ctx, "choir.event"); err != nil || ok {
		t.Fatalf("event kind absent: %v, %v", ok, err)
	}
	if st := store.EngineMutexStats()["(*DoltStore).HasObjectOfKind"]; st.Calls != 2 {
		t.Fatalf("the probe is not named on the engine mutex: %v", store.EngineMutexStats())
	}
}
