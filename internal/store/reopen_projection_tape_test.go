package store

import (
	"context"
	"testing"
)

// Store.Reopen adopts the handles of a freshly opened store, including its
// object-graph store, which has no mutation interceptor: BindProjectionTape
// installed the interceptor on the store that Reopen replaces. If nothing
// installs it again, an og_objects write after Reopen is direct SQL with no
// tape event, and the replay-completeness witness cannot reproduce it.
// Written for docs/problems/selfdev-operations-replay-not-equivalent-2026-10-23.md
// (section "Store.Reopen and the OG interceptor"). It records what the code
// does; it does not fix it.
func TestReopenKeepsTheObjectGraphOnTheProjectionTape(t *testing.T) {
	ctx := context.Background()
	productStore := openProjectStore(t)
	computerID := "computer-reopen-tape"
	prepareGenesis(t, productStore, computerID, "genesis-reopen-tape")
	if err := bindResidueTape(t, productStore, computerID); err != nil {
		t.Fatal(err)
	}
	sequence := func() uint64 {
		t.Helper()
		head, err := productStore.Head(ctx, computerID)
		if err != nil || head == nil {
			t.Fatalf("head: %v %#v", err, head)
		}
		return head.Sequence
	}

	before := sequence()
	if err := productStore.ogStore.PutObject(ctx, residueObject(computerID, "obj-before-reopen")); err != nil {
		t.Fatal(err)
	}
	if got := sequence(); got != before+1 {
		t.Fatalf("control: a bound OG write moved the tape %d -> %d, want one event", before, got)
	}

	path := productStore.path
	if err := productStore.Close(); err != nil {
		t.Fatal(err)
	}
	if err := productStore.Reopen(path); err != nil {
		t.Fatal(err)
	}
	if !productStore.ProjectionTapeBound() {
		t.Fatal("Reopen unbound the projection tape")
	}

	before = sequence()
	if err := productStore.ogStore.PutObject(ctx, residueObject(computerID, "obj-after-reopen")); err != nil {
		t.Fatal(err)
	}
	if got := sequence(); got != before+1 {
		t.Fatalf("after Reopen an OG write left the tape at %d (was %d): the mutation interceptor was not carried over, so the write is direct SQL with no event", got, before)
	}
}
