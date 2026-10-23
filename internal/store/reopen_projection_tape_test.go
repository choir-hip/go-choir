package store

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
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

// Open builds the new object-graph store with
// SetWriteValidator(live.vocabWriteGuard), a method value bound to the
// temporary store, and Reopen then zeroes that store (*live = Store{}). The
// validator the adopted ogStore kept read the zeroed store's vocabCutover, so
// the vocabulary write fence was disarmed after a Reopen even when s had cut
// over (boot rebase at autoputer/projection_base.go:172-196, runtime
// rematerialize at agentcore/rematerialize.go:262). Reopen binds the
// validator to s and never lowers an armed fence
// (docs/problems/selfdev-operations-replay-not-equivalent-2026-10-23.md).
func TestReopenKeepsTheVocabularyWriteFenceArmed(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)
	s.vocabCutover.Store(true)
	stale := []objectgraph.Object{{ObjectKind: objectgraph.ObjectKind("choir.agent"), Metadata: []byte(`{"role":"boss"}`), Body: []byte(`{}`)}}
	if err := s.ogStore.PreflightWrite(ctx, stale, nil); err == nil {
		t.Fatal("control: the fence accepted an unknown role before Reopen")
	}
	path := s.path
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.Reopen(path); err != nil {
		t.Fatal(err)
	}
	if err := s.ogStore.PreflightWrite(ctx, stale, nil); err == nil {
		t.Fatal("after Reopen the vocabulary write fence accepts an unknown role: its validator is bound to the zeroed temporary store")
	}
}

// No OG write can bypass the tape after Reopen, by either object-graph door
// the store has (the service s.og that ogPut uses, and ogStore directly).
// The interceptor is installed on the adopted store before Reopen publishes
// it, and the store is closed until Reopen returns, so there is no window to
// race; this test is the race-safe evidence: writers start the moment Reopen
// returns, under -race, and every write must be exactly one tape event.
func TestReopenedStoreRoutesConcurrentObjectGraphWritesThroughTheTape(t *testing.T) {
	ctx := context.Background()
	productStore := openProjectStore(t)
	computerID := "computer-reopen-race"
	prepareGenesis(t, productStore, computerID, "genesis-reopen-race")
	if err := bindResidueTape(t, productStore, computerID); err != nil {
		t.Fatal(err)
	}
	path := productStore.path
	if err := productStore.Close(); err != nil {
		t.Fatal(err)
	}
	if err := productStore.Reopen(path); err != nil {
		t.Fatal(err)
	}
	head, err := productStore.Head(ctx, computerID)
	if err != nil || head == nil {
		t.Fatalf("head: %v %#v", err, head)
	}
	before := head.Sequence

	const writers = 6
	var wg sync.WaitGroup
	errs := make(chan error, 2*writers)
	for i := 0; i < writers; i++ {
		wg.Add(2)
		go func(i int) { // the service door: ogPut -> s.og
			defer wg.Done()
			errs <- productStore.PutComputerSetting(ctx, ComputerSetting{
				OwnerID: fmt.Sprintf("owner-race-%d", i), ComputerID: computerID, Key: SettingIntakeLabels, Value: true,
			})
		}(i)
		go func(i int) { // the store door
			defer wg.Done()
			errs <- productStore.ogStore.PutObject(ctx, residueObject(computerID, fmt.Sprintf("obj-race-%d", i)))
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	head, err = productStore.Head(ctx, computerID)
	if err != nil || head == nil {
		t.Fatalf("head after writes: %v %#v", err, head)
	}
	if got := head.Sequence - before; got != 2*writers {
		t.Fatalf("%d concurrent OG writes appended %d tape events; a write bypassed the tape", 2*writers, got)
	}
}

// The same, with the cutover the way a computer has it: the fenced migration
// persists the report in the workspace, the report survives the Reopen, and
// the fence keeps refusing a stale role while still admitting a live one.
func TestReopenAfterTheFencedCutoverKeepsTheFenceArmedAndAdmitsLiveRoles(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)
	if _, err := s.MigrateAndFenceServingVocabulary(ctx, false, nil); err != nil {
		t.Fatal(err)
	}
	agent := func(role string) []objectgraph.Object {
		return []objectgraph.Object{{ObjectKind: objectgraph.ObjectKind("choir.agent"), Metadata: []byte(fmt.Sprintf(`{"role":%q}`, role)), Body: []byte(`{}`)}}
	}
	path := s.path
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.Reopen(path); err != nil {
		t.Fatal(err)
	}
	if !s.vocabCutover.Load() {
		t.Fatal("Reopen lost the cutover the adopted workspace's report records")
	}
	if err := s.ogStore.PreflightWrite(ctx, agent("boss"), nil); err == nil {
		t.Fatal("after Reopen the fence accepts an unknown role")
	}
	for _, role := range []string{"management", "engineering", "research", "texture", "management_task"} {
		if err := s.ogStore.PreflightWrite(ctx, agent(role), nil); err != nil {
			t.Fatalf("after Reopen the fence refuses live role %q: %v", role, err)
		}
	}
}
