package store

// Engineering slot claims across the two homes (review bunch 206, F1 / B1 / B2,
// docs/problems/data-has-more-than-one-home-2026-10-24.md failure mode 3).
// Ways it could fail, written before the fix:
//
//   - a claim that read engineering.slots_graph on is in flight (its tape
//     append not yet landed) while the setting turns off and a second claim
//     takes the table home: the table path's graph-holder check sees no
//     object, so both return claimed=true and two runs hold one slot;
//   - the mirror: a table claim in flight while the setting turns on;
//   - the setting is read before the home's lock is taken, so the claim that
//     waited acts on a stale setting.
//
// The one slot must have one holder whatever the setting does mid-claim.

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

const raceOwner = "owner-slot-race"

func raceSetting(t *testing.T, h *tapeHarness, value bool) {
	t.Helper()
	if err := h.store.PutComputerSetting(context.Background(), ComputerSetting{
		OwnerID: raceOwner, ComputerID: h.computerID, Key: SettingEngineeringSlotsGraph, Value: value,
	}); err != nil {
		t.Fatal(err)
	}
}

func raceRun(t *testing.T, h *tapeHarness, runID string) {
	t.Helper()
	now := time.Now().UTC()
	if err := h.store.CreateRun(context.Background(), types.RunRecord{
		RunID: runID, AgentID: "agent-" + runID, OwnerID: raceOwner, ComputerID: "c",
		State: types.RunRunning, Prompt: "p", TrajectoryID: "traj-race", CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
}

type raceClaim struct {
	claimed bool
	err     error
}

func raceClaimAsync(h *tapeHarness, runID string) chan raceClaim {
	out := make(chan raceClaim, 1)
	go func() {
		_, claimed, err := h.store.ClaimEngineeringSlot(context.Background(), raceOwner, "traj-race", "implementation", runID, "agent-"+runID, "requester")
		out <- raceClaim{claimed, err}
	}()
	return out
}

// TestSlotClaimAcrossTheSettingFlipHasOneHolder holds a graph-home claim open
// after its table check (the slot object is not yet written), turns the setting
// off, and starts a second claim, which takes the table home. Exactly one of
// the two may be told claimed=true.
func TestSlotClaimAcrossTheSettingFlipHasOneHolder(t *testing.T) {
	h := newTapeHarness(t)
	raceSetting(t, h, true)
	raceRun(t, h, "run-a")
	raceRun(t, h, "run-b")

	inFlight := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	h.store.slotClaimHook = func() {
		once.Do(func() {
			close(inFlight)
			<-release
		})
	}

	first := raceClaimAsync(h, "run-a")
	select {
	case <-inFlight:
	case <-time.After(30 * time.Second):
		t.Fatal("the graph claim never reached the hook")
	}
	raceSetting(t, h, false)
	second := raceClaimAsync(h, "run-b")

	var results []raceClaim
	var secondDone bool
	select {
	case r := <-second:
		// Claim two finished while claim one was still in flight: only legal
		// if it was told the slot is held. Collect it either way.
		results, secondDone = append(results, r), true
	case <-time.After(500 * time.Millisecond):
		// Claim two is waiting for claim one: the homes are serialized.
	}
	close(release)
	results = append(results, <-first)
	if !secondDone {
		results = append(results, <-second)
	}
	winners := 0
	for _, r := range results {
		if r.err != nil {
			t.Fatalf("claim: %v", r.err)
		}
		if r.claimed {
			winners++
		}
	}
	if winners != 1 {
		t.Fatalf("claims told claimed=true = %d, want exactly 1 (two runs hold one slot)", winners)
	}
}

// TestSlotClaimsWhileTheSettingFlipsHaveOneHolder races claimants against a
// goroutine turning the setting on and off: whatever home each claim lands in,
// the slot has one holder.
func TestSlotClaimsWhileTheSettingFlipsHaveOneHolder(t *testing.T) {
	h := newTapeHarness(t)
	const n = 6
	for i := 0; i < n; i++ {
		raceRun(t, h, fmt.Sprintf("run-%d", i))
	}
	stop := make(chan struct{})
	var flipper sync.WaitGroup
	flipper.Add(1)
	go func() {
		defer flipper.Done()
		for on := true; ; on = !on {
			select {
			case <-stop:
				return
			default:
			}
			_ = h.store.PutComputerSetting(context.Background(), ComputerSetting{
				OwnerID: raceOwner, ComputerID: h.computerID, Key: SettingEngineeringSlotsGraph, Value: on,
			})
		}
	}()
	var claims []chan raceClaim
	for i := 0; i < n; i++ {
		claims = append(claims, raceClaimAsync(h, fmt.Sprintf("run-%d", i)))
	}
	winners := 0
	for _, c := range claims {
		r := <-c
		if r.err != nil {
			t.Errorf("claim: %v", r.err)
		}
		if r.claimed {
			winners++
		}
	}
	close(stop)
	flipper.Wait()
	if winners != 1 {
		t.Fatalf("claims told claimed=true = %d, want exactly 1", winners)
	}
}

// TestTableSlotHeldByALiveRunIsNotClaimedTwice pins the legacy home: a second
// claim of a slot a live run holds is refused and names the holder. (The
// INSERT ... ON DUPLICATE KEY UPDATE it used told the second claimer
// claimed=true, because the driver counts the matched row.)
func TestTableSlotHeldByALiveRunIsNotClaimedTwice(t *testing.T) {
	h := newTapeHarness(t)
	raceRun(t, h, "run-1")
	raceRun(t, h, "run-2")
	ctx := context.Background()
	if _, claimed, err := h.store.ClaimEngineeringSlot(ctx, raceOwner, "traj-race", "implementation", "run-1", "agent-1", ""); err != nil || !claimed {
		t.Fatalf("first claim: claimed=%v err=%v", claimed, err)
	}
	held, claimed, err := h.store.ClaimEngineeringSlot(ctx, raceOwner, "traj-race", "implementation", "run-2", "agent-2", "")
	if err != nil {
		t.Fatal(err)
	}
	if claimed || held.RunID != "run-1" {
		t.Fatalf("second claim: claimed=%v holder=%q, want claimed=false holder=run-1", claimed, held.RunID)
	}
}
