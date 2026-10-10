package store

import (
	"context"
	"os"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

// A live store without an upcast ledger never upcast its deposits, so the
// from-genesis replay that the apply checkpoint compares it with (and the
// restore that installs it) must keep the recorded bytes too. Upcasting
// there re-keyed recorded tool-call events and V1-spelled lifecycle ids and
// made every such computer unreplayable
// (problems/selfdev-apply-checkpoint-starved-by-resumed-work-2026-10-10.md,
// reruns 8 and 9). Failure modes pinned: a fresh replay that upcasts while
// asked to retain; a retained replay that differs from the live projection
// of the same tape; the default fresh replay silently changing (it still
// upcasts); retention requested after the upcast activated, or on a store
// whose deposits are already upcast.
func TestRetainedReplayDepositsMatchLiveProjection(t *testing.T) {
	full, _ := vocabUpcastBuildTape(t)
	tape := full[:5] // genesis + V1 create, V1 event, V1 edge, V1 update

	live, _ := vocabUpcastOpenStore(t, "live")
	for _, tapeEvent := range tape {
		vocabUpcastApplyLiveEvent(t, live, tapeEvent)
	}
	if upcast, err := live.DepositsUpcast(); err != nil || upcast {
		t.Fatalf("live store DepositsUpcast = %v, %v; want false", upcast, err)
	}

	retained, _ := vocabUpcastOpenStore(t, "retained")
	if err := retained.RetainReplayDeposits(); err != nil {
		t.Fatal(err)
	}
	vocabUpcastReplayTape(t, retained, tape, 0, len(tape))
	want, got := vocabUpcastOGSnapshot(t, live), vocabUpcastOGSnapshot(t, retained)
	vocabUpcastAssertSnapshotsEqual(t, "retained og_objects", want["og_objects"], got["og_objects"])
	vocabUpcastAssertSnapshotsEqual(t, "retained og_edges", want["og_edges"], got["og_edges"])
	if upcast, err := retained.DepositsUpcast(); err != nil || upcast {
		t.Fatalf("retained replay wrote an upcast ledger: %v, %v", upcast, err)
	}
	if _, err := os.Stat(retained.depositUpcastLedgerPath()); !os.IsNotExist(err) {
		t.Fatalf("retained replay ledger stat = %v, want not exist", err)
	}

	upcastReplay, _ := vocabUpcastOpenStore(t, "upcast")
	vocabUpcastReplayTape(t, upcastReplay, tape, 0, len(tape))
	if upcast, err := upcastReplay.DepositsUpcast(); err != nil || !upcast {
		t.Fatalf("default fresh replay DepositsUpcast = %v, %v; want true", upcast, err)
	}
	if equalSnapshots(want["og_objects"], vocabUpcastOGSnapshot(t, upcastReplay)["og_objects"]) {
		t.Fatal("default fresh replay matched the V1 live projection; the tape no longer exercises the upcast")
	}
	if err := upcastReplay.RetainReplayDeposits(); err == nil {
		t.Fatal("retention accepted on a store whose deposits are already upcast")
	}
}

func vocabUpcastApplyLiveEvent(t *testing.T, s *Store, tapeEvent vocabUpcastTapeEvent) {
	t.Helper()
	ctx := context.Background()
	digest, err := tapeEvent.Event.Digest()
	if err != nil {
		t.Fatal(err)
	}
	request := computerevent.CASRequest{
		Event: tapeEvent.Event, EventDigest: digest, EventArtifactDigest: digest,
		EventPinReceiptDigest: storeTestDigest('c'), Input: tapeEvent.Input, Next: tapeEvent.Next,
	}
	if err := s.Prepare(ctx, request); err != nil {
		t.Fatalf("prepare sequence %d: %v", tapeEvent.Event.Sequence, err)
	}
	var batch *computerevent.ProjectionBatch
	if tapeEvent.Ops != nil {
		batch = &computerevent.ProjectionBatch{
			Version: computerevent.ProjectionBatchV2, ProjectorVersion: computerevent.ProjectorVersionV2,
			ComputerID: tapeEvent.Event.ComputerID, EventID: tapeEvent.Event.EventID, EventDigest: digest,
			Ops: vocabUpcastCopyOps(tapeEvent.Ops),
		}
	}
	if err := s.FinalizeBatch(ctx, tapeEvent.Event.ComputerID, digest, tapeEvent.Receipt, batch); err != nil {
		t.Fatalf("live finalize sequence %d: %v", tapeEvent.Event.Sequence, err)
	}
}

func equalSnapshots(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
