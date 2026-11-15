package platform

import (
	"fmt"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/recoveryplan"
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
//     liveness, so a row dropped is a chunk collected.

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

func assertRestoreWindowHeld(t *testing.T, store *Store, computer string, newest int64, spacing int64, maxRows int) {
	t.Helper()
	live, err := store.LiveSnapshotRoots(t.Context(), computer)
	if err != nil {
		t.Fatal(err)
	}
	if len(live) > maxRows {
		t.Fatalf("%d roots kept, want at most %d (storage must stay bounded)", len(live), maxRows)
	}
	bound := int64(recoveryplan.MaxRecoveryTailEvents)
	// Every checkpoint inside the recovery bound of the newest snapshot's
	// head (the store head is at least that) has a kept base at or below it,
	// and the nearest one is no further than the recovery bound.
	for cp := newest - bound; cp <= newest; cp += spacing / 3 {
		base := int64(-1)
		for _, r := range live {
			if r.HeadSequence <= cp && r.HeadSequence > base {
				base = r.HeadSequence
			}
		}
		if base < 0 {
			t.Fatalf("checkpoint %d (%d events behind the newest root) has no kept base; kept %+v", cp, newest-cp, live)
		}
		if cp-base > bound {
			t.Fatalf("checkpoint %d: nearest kept base %d is %d events below, past the recovery bound", cp, base, cp-base)
		}
	}
	// The chain advances: the oldest kept root is the first one at or past
	// the window, not an old frozen anchor.
	oldest := live[len(live)-1].HeadSequence
	if newest-oldest < bound || newest-oldest > bound+bound/2+spacing {
		t.Fatalf("oldest kept root is %d events behind the newest; want between the bound %d and %d (frozen or unbounded chain)", newest-oldest, bound, bound+bound/2+spacing)
	}
}

// d@e: a snapshot every four hours at 1.8 events/s (25,920 events), a small
// store. A checkpoint 35 hours old (226,800 events) still has a base.
func TestSnapshotRetentionKeepsTheRecoveryWindowOnASmallStore(t *testing.T) {
	store, _ := openTestPlatformStore(t)
	const spacing = 25920
	registerWindowRoots(t, store, "computer-window-small", 60, spacing, 3)
	assertRestoreWindowHeld(t, store, "computer-window-small", 60*spacing, spacing, snapshotRootRetention+snapshotRootAnchorsMax)
}

// The paper: a large store, two newest roots, a capture every 13.4 hours at
// 3.3 events/s (159,000 events).
func TestSnapshotRetentionKeepsTheRecoveryWindowOnALargeStore(t *testing.T) {
	store, _ := openTestPlatformStore(t)
	const spacing = 159000
	registerWindowRoots(t, store, "computer-window-large", 14, spacing, largeSnapshotChunks+1)
	assertRestoreWindowHeld(t, store, "computer-window-large", 14*spacing, spacing, largeSnapshotRetention+snapshotRootAnchorsMax)
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
