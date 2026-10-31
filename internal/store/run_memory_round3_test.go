package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Round 3, finding 5: a failed seal on the way off is counted, not silent.
func TestSetTapeGroupCommitOffCountsAFailedSeal(t *testing.T) {
	h, _ := windowHarness(t)
	ctx := context.Background()
	h.dryRunHook = func(batch *computerevent.ProjectionBatch) error {
		for _, op := range batch.Ops {
			if op.CanonicalID == "poison-off" {
				return errors.New("refused")
			}
		}
		return nil
	}
	if _, err := h.store.AppendRunMemoryEntry(ctx, rmMessage("run-off-fail", "poison-off")); err != nil {
		t.Fatal(err)
	}
	h.store.SetTapeGroupCommit(false)
	if got := h.store.groupCommitStats().OffSealFailures; got != 1 {
		t.Fatalf("off_seal_failures = %d, want 1", got)
	}
	if !h.store.RunMemoryDamaged("run-off-fail") {
		t.Fatal("the run whose entry was lost on the way off is not damaged")
	}
}

// Round 3, finding 4: a damaged run is never a seed, whatever its state.
func TestLatestActorRunMemorySkipsADamagedRun(t *testing.T) {
	h, _ := windowHarness(t)
	ctx := context.Background()
	now := time.Now().UTC()
	rec := types.RunRecord{RunID: "run-seed-damaged", AgentID: "agent-1", OwnerID: "owner-1", ComputerID: "computer-1",
		State: types.RunPassivated, CreatedAt: now, UpdatedAt: now}
	if err := h.store.CreateRun(ctx, rec); err != nil {
		t.Fatal(err)
	}
	if _, err := h.store.AppendRunMemoryEntry(ctx, rmMessage("run-seed-damaged", "seed-1")); err != nil {
		t.Fatal(err)
	}
	h.store.SealRunMemory("run-seed-damaged")
	if src, _, err := h.store.LatestActorRunMemoryEntries(ctx, "owner-1", "computer-1", "agent-1", "next"); err != nil || src != "run-seed-damaged" {
		t.Fatalf("a whole passivated run is not a seed: %q %v", src, err)
	}
	h.store.MarkRunMemoryDamaged("run-seed-damaged")
	src, entries, err := h.store.LatestActorRunMemoryEntries(ctx, "owner-1", "computer-1", "agent-1", "next")
	if (err != nil && !errors.Is(err, ErrNotFound)) || src != "" || len(entries) != 0 {
		t.Fatalf("a damaged run seeded the next: %q %d %v", src, len(entries), err)
	}
	if !h.store.AnyRunMemoryDamaged() {
		t.Fatal("AnyRunMemoryDamaged is false with a damaged run")
	}
}
