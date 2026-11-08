package store

import (
	"context"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// Enforce flip S1, section 6.7 (background-context audit): a spawn that fails
// releases its engineering slot claim on context.Background(), and the boot
// reconciliation releases one on the boot context. Neither declares a reader or
// a writer, so at enforce the release was refused. The release is the
// custodian's own bookkeeping and declares the slot kind's custodian for the
// graph read and write it makes.
func TestSlotReleaseOnABareContextDeclaresTheCustodian(t *testing.T) {
	h := newTapeHarness(t)
	raceSetting(t, h, true)
	raceRun(t, h, "run-rel")
	ctx := context.Background()
	if _, claimed, err := h.store.ClaimEngineeringSlot(ctx, raceOwner, "traj-race", "implementation", "run-rel", "agent-run-rel", ""); err != nil || !claimed {
		t.Fatalf("claim: claimed=%v err=%v", claimed, err)
	}
	objectgraph.ResetReaderGateCounts()
	objectgraph.ResetWriterGateCounts()
	if err := h.store.ReleaseEngineeringSlotClaim(ctx, raceOwner, "traj-race", "implementation", "run-rel"); err != nil {
		t.Fatal(err)
	}
	for key, n := range objectgraph.ReaderGateCounts() {
		if strings.Contains(key, "co_super_slot") {
			t.Errorf("the release read the slot with no declared reader: %s x%d", key, n)
		}
	}
	for key, n := range objectgraph.WriterGateCounts() {
		if strings.Contains(key, "co_super_slot") {
			t.Errorf("the release wrote the slot with no declared writer: %s x%d", key, n)
		}
	}
}
