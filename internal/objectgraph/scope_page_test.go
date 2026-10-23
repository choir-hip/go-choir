package objectgraph

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

// ListObjectsByKindOwnerComputerPage serves GET /api/runs' legacy half: the
// runs of one owner whose computer_id is empty, without reading every run of
// the kind (docs/problems/paper-remaining-store-stall-holders-2026-10-23.md
// section 4).
//
// Ways it can fail, written before the code:
//  1. The engine plans it as ListObjectsPage was planned: the kind index, a
//     TopN over every remaining row of the kind per page (quadratic over the
//     pages). The plan test pins an index range on (kind, owner, computer)
//     with all three as equality bounds, so the TopN sees only the scope's
//     rows (the engine keeps a TopN for the cursor order; the range, not the
//     sort, is what bounds the work).
//  2. The index is absent (a populated store that was never migrated): the
//     read is wrong or errors instead of answering, slower, from the
//     (kind, owner) index every store has.
//  3. It returns rows of another owner, another kind, or a non-empty
//     computer; the empty computer is matched exactly.
//  4. The cursor skips or repeats a row at a page boundary, or the pages are
//     not in canonical-ID order.
//  5. It runs unnamed or outside the engine mutex.

const scopePageIndex = "og_objects.object_kind,og_objects.owner_id,og_objects.computer_id"

func putScopeObject(t *testing.T, store *DoltStore, id string, kind ObjectKind, owner, computer string, at time.Time) {
	t.Helper()
	obj := Object{CanonicalID: id, ObjectKind: kind, OwnerID: owner, ComputerID: computer, VersionID: "v",
		ContentHash: "h-" + id, Body: []byte(`{"id":"` + id + `"}`), Metadata: []byte(`{}`), CreatedAt: at, UpdatedAt: at}
	if err := store.PutObject(context.Background(), obj); err != nil {
		t.Fatalf("put %s: %v", id, err)
	}
}

// seedScopeMix writes a mix: two owners, two kinds, three computer values.
// It returns the ids that match (choir.run, owner-a, computer "").
func seedScopeMix(t *testing.T, store *DoltStore, n int) []string {
	t.Helper()
	at := time.Date(2026, 10, 23, 6, 0, 0, 0, time.UTC)
	var want []string
	for i := 0; i < n; i++ {
		owner := "owner-a"
		if i%4 == 1 {
			owner = "owner-b"
		}
		kind := ObjectKind("choir.run")
		if i%7 == 3 {
			kind = "choir.event"
		}
		computer := ""
		switch i % 3 {
		case 1:
			computer = "computer-1"
		case 2:
			computer = "computer-2"
		}
		id := fmt.Sprintf("obj:%s:%s:%04d", kind, owner, (i*37)%n) // insertion order differs from id order
		putScopeObject(t, store, id, kind, owner, computer, at.Add(time.Duration(i)*time.Second))
		if kind == "choir.run" && owner == "owner-a" && computer == "" {
			want = append(want, id)
		}
	}
	// Sorted by canonical id: the page order.
	for i := 1; i < len(want); i++ {
		for j := i; j > 0 && want[j] < want[j-1]; j-- {
			want[j], want[j-1] = want[j-1], want[j]
		}
	}
	return want
}

func walkScopePages(t *testing.T, store *DoltStore, kind, owner, computer string, pageSize int) []string {
	t.Helper()
	var got []string
	after := ""
	for pages := 0; ; pages++ {
		if pages > 10000 {
			t.Fatal("cursor never ends")
		}
		page, err := store.ListObjectsByKindOwnerComputerPage(context.Background(), kind, owner, computer, after, pageSize)
		if err != nil {
			t.Fatal(err)
		}
		if len(page) == 0 {
			return got
		}
		if len(page) > pageSize {
			t.Fatalf("page of %d exceeds the limit %d", len(page), pageSize)
		}
		for _, o := range page {
			if string(o.ObjectKind) != kind || o.OwnerID != owner || o.ComputerID != computer {
				t.Fatalf("row %s (%s, %s, %q) is outside the scope (%s, %s, %q)", o.CanonicalID, o.ObjectKind, o.OwnerID, o.ComputerID, kind, owner, computer)
			}
			if o.CanonicalID <= after {
				t.Fatalf("cursor %q repeats or reorders %q", after, o.CanonicalID)
			}
			after = o.CanonicalID
			got = append(got, o.CanonicalID)
		}
	}
}

func TestScopePagePlanSeeksTheKindOwnerComputerIndex(t *testing.T) {
	db := openTestDoltDB(t)
	store := NewDoltStore(db)
	if err := store.EnsureSchema(context.Background()); err != nil {
		t.Fatal(err)
	}
	// F1 on a fresh store, where open builds the base indexes on the empty table.
	if !containsString(ogIndexes(t, db), "idx_og_objects_kind_owner_computer") {
		t.Fatalf("a fresh store lacks idx_og_objects_kind_owner_computer: %v", ogIndexes(t, db))
	}
	seedScopeMix(t, store, 60)
	plan := explainPlan(t, db, ObjectsByKindOwnerComputerPageSQL, "choir.run", "owner-a", "", "", 256)
	assertScopeRange(t, plan)
	t.Logf("plan:\n%s", plan)
}

// assertScopeRange fails unless the plan reads the (kind, owner, computer)
// index with an equality bound on all three columns and does not scan.
func assertScopeRange(t *testing.T, plan string) {
	t.Helper()
	assertIndexedPlan(t, plan, scopePageIndex, false)
	if !strings.Contains(plan, "filters: [{[choir.run, choir.run], [owner-a, owner-a], [, ]}]") {
		t.Fatalf("plan does not bound kind, owner and computer_id by equality:\n%s", plan)
	}
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func TestScopePageReturnsExactlyTheScopeAcrossPageSizes(t *testing.T) {
	db := openTestDoltDB(t)
	store := NewDoltStore(db)
	if err := store.EnsureSchema(context.Background()); err != nil {
		t.Fatal(err)
	}
	want := seedScopeMix(t, store, 300)
	if len(want) < 20 {
		t.Fatalf("seed is too small: %d matching", len(want))
	}
	for _, size := range []int{1, 2, 7, 64, 1000} {
		got := walkScopePages(t, store, "choir.run", "owner-a", "", size)
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Fatalf("page size %d: got %d ids, want %d\n got %v\nwant %v", size, len(got), len(want), got, want)
		}
	}
	// F3: an owner or kind with no rows in the scope answers empty, not an error.
	if got := walkScopePages(t, store, "choir.run", "owner-none", "", 10); len(got) != 0 {
		t.Fatalf("unknown owner returned %v", got)
	}
	// A named computer is its own scope.
	if got := walkScopePages(t, store, "choir.run", "owner-a", "computer-1", 10); len(got) == 0 {
		t.Fatal("computer-1 scope is empty")
	}
	// F5: the read is named by kind in the engine-mutex stats.
	var named bool
	for op := range store.EngineMutexStats() {
		if strings.Contains(op, "ListObjectsByKindOwnerComputerPage choir.run") {
			named = true
		}
	}
	if !named {
		t.Fatalf("the read is not named in the engine-mutex stats: %v", store.EngineMutexStats())
	}
}

// F2: a store with rows that was never migrated has no
// idx_og_objects_kind_owner_computer (open adds base indexes only to an empty
// table). The read must answer the same, from the (kind, owner) index every
// store has, and the explicit migration must change only the speed.
func TestScopePageAnswersWithoutTheBaseIndexAndAfterMigration(t *testing.T) {
	ctx := context.Background()
	db := openTestDoltDB(t)
	if _, err := db.Exec(SchemaDDL); err != nil {
		t.Fatal(err)
	}
	store := NewDoltStore(db)
	want := seedScopeMix(t, store, 300)
	if err := store.EnsureSchema(ctx); err != nil { // populated: discovery only
		t.Fatal(err)
	}
	if containsString(ogIndexes(t, db), "idx_og_objects_kind_owner_computer") {
		t.Fatalf("open built the base index on a populated store: %v", ogIndexes(t, db))
	}
	got := walkScopePages(t, store, "choir.run", "owner-a", "", 16)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("without the base index: got %v want %v", got, want)
	}
	plan := explainPlan(t, db, ObjectsByKindOwnerComputerPageSQL, "choir.run", "owner-a", "", "", 256)
	// The (kind, owner) index every store has bounds it to the owner's rows
	// of the kind: one owner's range, not the kind.
	assertIndexedPlan(t, plan, "og_objects.object_kind,og_objects.owner_id", false)
	t.Logf("plan without the base index:\n%s", plan)

	if _, err := store.MigrateAccessPaths(ctx, func(AccessPathStep) {}); err != nil {
		t.Fatal(err)
	}
	if !containsString(ogIndexes(t, db), "idx_og_objects_kind_owner_computer") {
		t.Fatalf("migration did not build the base index: %v", ogIndexes(t, db))
	}
	got = walkScopePages(t, store, "choir.run", "owner-a", "", 16)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("after migration: got %v want %v", got, want)
	}
	assertScopeRange(t, explainPlan(t, db, ObjectsByKindOwnerComputerPageSQL, "choir.run", "owner-a", "", "", 256))
}
