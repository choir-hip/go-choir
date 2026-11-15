package platform

import (
	"fmt"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/recoveryplan"
	"github.com/yusefmosiah/go-choir/internal/snapshot"
)

// Failure modes of snapshot-root retention once snapshots are due by replay
// time (lane/snapshot-by-replay-time, review round 1, finding 2): the host
// keeps the newest k roots, so a snapshot every four hours keeps 16 hours of
// them and a checkpoint 35 hours old (inside the recovery bound) finds no
// base, where the 540,000-event cadence kept a base for the whole bound.
//
//  1. a checkpoint inside the recovery bound has no kept root at or below it;
//  2. the nearest kept base below a checkpoint is more than the recovery bound
//     of events away, so the restore replays what recovery refuses;
//  3. the kept chain is frozen: one old anchor kept forever, never advancing;
//  4. the chain is unbounded: every root kept, so storage is not bounded;
//  5. the extra roots are collectable: a kept row is the artifact collector's
//     liveness, so a row dropped is a chunk collected;
//  6. the kept chain exists on the host but the guest's selection cannot reach
//     it (review round 2): the tests run the real plan, snapshot.PlanCandidates
//     over every snapshot event on the tape, with restoreOne's own filter (the
//     record's head at or below the restore point) and the host's live roots.

func registerWindowRoots(t *testing.T, store *Store, computer string, n int, spacing int64, chunkCount int) []int64 {
	t.Helper()
	var heads []int64
	for i := 1; i <= n; i++ {
		head := int64(i) * spacing
		root := gcDigest(fmt.Sprintf("window/%s/%d", computer, i))
		nameRootRow(t, store, computer, root, head)
		chunks := make([]string, chunkCount)
		for c := range chunks {
			chunks[c] = gcDigest(fmt.Sprintf("window/%s/%d/%d", computer, i, c))
		}
		if err := store.recordSnapshotRoot(t.Context(), computer, root, head, chunks); err != nil {
			t.Fatal(err)
		}
		heads = append(heads, head)
	}
	return heads
}

// selectBase is the guest's choice for a checkpoint: the plan's candidates
// (every snapshot event on the tape at or below the target within the
// recovery bound), newest first, the first whose record head is at or below the
// target (restoreOne's MaxSequence filter) and whose chunks the host still
// holds (the install's probe: the root is live). Event sequences are 2 above
// their heads, as a capture's append is.
func selectBase(heads []int64, live map[int64]bool, cp int64) (head int64, attempts int, ok bool) {
	var tape []snapshot.Candidate
	for _, h := range heads {
		tape = append(tape, snapshot.Candidate{Sequence: uint64(h) + 2, EventID: fmt.Sprint(h)})
	}
	plan := snapshot.PlanCandidates(tape, uint64(cp), recoveryplan.MaxRecoveryTailEvents)
	for i := len(plan) - 1; i >= 0; i-- {
		attempts++
		var h int64
		fmt.Sscan(plan[i].EventID, &h)
		if h <= cp && live[h] {
			return h, attempts, true
		}
	}
	return 0, attempts, false
}

func assertRestoreWindowHeld(t *testing.T, store *Store, computer string, heads []int64, spacing int64, maxRows int) {
	t.Helper()
	newest := heads[len(heads)-1]
	liveRows, err := store.LiveSnapshotRoots(t.Context(), computer)
	if err != nil {
		t.Fatal(err)
	}
	if len(liveRows) > maxRows {
		t.Fatalf("%d roots kept, want at most %d (storage must stay bounded)", len(liveRows), maxRows)
	}
	live := map[int64]bool{}
	for _, r := range liveRows {
		live[r.HeadSequence] = true
	}
	bound := int64(recoveryplan.MaxRecoveryTailEvents)
	// Every checkpoint inside the recovery bound of the newest snapshot's head
	// is restorable by the guest's own selection, from a base no further than
	// the bound below it, within the attempt cap.
	for cp := newest - bound; cp <= newest; cp += spacing / 3 {
		base, attempts, ok := selectBase(heads, live, cp)
		if !ok {
			t.Fatalf("checkpoint %d (%d events behind the newest root): the guest's selection finds no live base after %d attempts; kept %+v", cp, newest-cp, attempts, liveRows)
		}
		if cp-base > bound {
			t.Fatalf("checkpoint %d: selected base %d is %d events below, past the recovery bound", cp, base, cp-base)
		}
		if attempts > snapshot.MaxRestoreAttempts {
			t.Fatalf("checkpoint %d took %d attempts, over the cap %d", cp, attempts, snapshot.MaxRestoreAttempts)
		}
	}
	// The chain advances: the oldest kept root is the first one at or past
	// the window, not an old frozen anchor.
	oldest := liveRows[len(liveRows)-1].HeadSequence
	if newest-oldest < bound || newest-oldest > bound+bound/2+spacing {
		t.Fatalf("oldest kept root is %d events behind the newest; want between the bound %d and %d (frozen or unbounded chain)", newest-oldest, bound, bound+bound/2+spacing)
	}
}

// d@e: a snapshot every four hours at 1.8 events/s (25,920 events), a small
// store. A checkpoint 35 hours old (226,800 events) still has a base.
func TestSnapshotRetentionKeepsTheRecoveryWindowOnASmallStore(t *testing.T) {
	store, _ := openTestPlatformStore(t)
	const spacing = 25920
	heads := registerWindowRoots(t, store, "computer-window-small", 60, spacing, 3)
	assertRestoreWindowHeld(t, store, "computer-window-small", heads, spacing, snapshotRootRetention+snapshotRootAnchorsMax)
}

// The reviewer's case on the 60-root fixture: a checkpoint at 1,100,000 must
// select the nearest live root below it although the snapshots just below it
// are pruned rows (the reviewer's hand-built kept set had 829,440 there; the
// real prune's set is read from the host rows).
func TestSnapshotRetentionSelectsTheChainRootBelowAPrunedRun(t *testing.T) {
	store, _ := openTestPlatformStore(t)
	const spacing = 25920
	heads := registerWindowRoots(t, store, "computer-window-case", 60, spacing, 3)
	liveRows, err := store.LiveSnapshotRoots(t.Context(), "computer-window-case")
	if err != nil {
		t.Fatal(err)
	}
	live := map[int64]bool{}
	for _, r := range liveRows {
		live[r.HeadSequence] = true
	}
	const cp = 1_100_000
	var want int64 = -1
	for h := range live {
		if h <= cp && h > want {
			want = h
		}
	}
	base, attempts, ok := selectBase(heads, live, cp)
	if !ok || base != want {
		t.Fatalf("checkpoint %d selected %d (ok=%v) after %d attempts, want the nearest live root %d", cp, base, ok, attempts, want)
	}
	// The case that matters: the snapshots just below the checkpoint are pruned
	// rows, so the live base is further down than the newest-four rule reached.
	if attempts <= snapshot.MaxCandidates {
		t.Fatalf("the base was found in %d attempts: the fixture no longer shows a pruned run below the checkpoint", attempts)
	}
}

// The paper: a large store, two newest roots, a capture every 13.4 hours at
// 3.3 events/s (159,000 events).
func TestSnapshotRetentionKeepsTheRecoveryWindowOnALargeStore(t *testing.T) {
	store, _ := openTestPlatformStore(t)
	const spacing = 159000
	heads := registerWindowRoots(t, store, "computer-window-large", 14, spacing, largeSnapshotChunks+1)
	assertRestoreWindowHeld(t, store, "computer-window-large", heads, spacing, largeSnapshotRetention+snapshotRootAnchorsMax)
}

// Roots kept at the old 540,000-event cadence behave as before: nothing past
// the newest k is kept when the k roots already span the window.
func TestSnapshotRetentionKeepsNoExtraWhenTheNewestRootsSpanTheWindow(t *testing.T) {
	store, _ := openTestPlatformStore(t)
	const spacing = 540000
	registerWindowRoots(t, store, "computer-window-old", 8, spacing, 3)
	live, err := store.LiveSnapshotRoots(t.Context(), "computer-window-old")
	if err != nil || len(live) != snapshotRootRetention {
		t.Fatalf("live=%d err=%v: roots 540,000 events apart keep the newest %d", len(live), err, snapshotRootRetention)
	}
}
