package agentcore

// The checkpoint bind must not hold appends or the write connection across the
// witness scan or the publish. Written before the code. Ways this could fail:
//
//  1. an append, or any other write, issued while the witness scan runs blocks
//     until the scan ends (the scan held the append mutex, or ran on the
//     store's single write connection): on the paper computer's 30 GB store
//     that is minutes of a stopped computer at every update;
//  2. the witness is not the state at the head the binding names: the head is
//     read separately from the scan, or the scan sees a write that committed
//     after the head was read, so restore reconstructs to the head, compares
//     and refuses a checkpoint that was fine when taken;
//  3. the change alters the witness bytes: the same state must give the same
//     v3 witness as the write-connection extraction it replaces;
//  4. the publish runs inside the append hold, so every append waits on a
//     network call;
//  5. a head that moved during the scan is published anyway (the platform
//     refuses it) or retried forever (a busy computer never checkpoints): the
//     bind must re-bind pinned and publish the head it then names.

import (
	"context"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
)

const unheldWait = 10 * time.Second

// withinWait runs fn on its own goroutine and reports whether it finished
// within unheldWait. A blocked fn is left to finish once the bind releases it.
func withinWait(fn func()) bool {
	done := make(chan struct{})
	go func() {
		defer close(done)
		fn()
	}()
	select {
	case <-done:
		return true
	case <-time.After(unheldWait):
		return false
	}
}

// appendDeskEventErr is appendDeskEvent for goroutines other than the test's:
// it reports with Errorf, never Fatalf.
func appendDeskEventErr(t *testing.T, fx *checkpointHeadFixture, key string) {
	t.Helper()
	eventID, _ := computerevent.NewEventID()
	commitment := strings.Repeat("f", 64)
	update := computerevent.Event{SchemaVersion: 1, EventID: eventID, ComputerID: fx.rt.cfg.ComputerID, EventKind: computerevent.EventResearchUpdate,
		OccurredAt: time.Now().UTC().Format(time.RFC3339Nano), IdempotencyKey: key, ActorProfile: "research", AuthorityRef: "typed-update",
		PayloadCommitment: strings.Repeat("0", 64), PrivacyClass: "owner", ResultingEffectiveCommitment: commitment, ReducerVersion: 1}
	if _, err := fx.rt.eventAppender.AppendNew(context.Background(), update, computerevent.TransitionInput{TargetStateCommitment: commitment}, nil); err != nil {
		t.Errorf("desk append: %v", err)
	}
}

func unheldBind(t *testing.T, fx *checkpointHeadFixture, publish checkpointPublishFunc) (checkpointBinding, *selfdevprotocol.CheckpointResponse, error) {
	t.Helper()
	ctx := context.Background()
	manifest, err := fx.rt.ensureCheckpointReleaseManifest(ctx, fx.rt.cfg.ComputerID)
	if err != nil {
		t.Fatal(err)
	}
	return fx.rt.bindCheckpoint(ctx, fx.rt.cfg.ComputerID, manifest.ContentDigest, manifest.Files, publish)
}

// Failure modes 1 and 2: an append and a plain write complete while the scan
// is in flight, and neither is in the witness, which is the state at the head
// the binding names.
func TestCheckpointBindScanDoesNotBlockWritesAndWitnessIsTheBoundHeadsState(t *testing.T) {
	fx := newCheckpointHeadFixture(t, "computer-bind-unheld-scan")
	ctx := context.Background()
	fx.appendDeskEvent(t, "before-bind")
	headBefore := fx.liveHead(t)
	witnessBefore, err := LiveStoreWitness(ctx, fx.rt.store.DB())
	if err != nil {
		t.Fatal(err)
	}

	var hookCalls int
	var appendedDuringScan, wroteDuringScan bool
	fx.rt.checkpointScanHook = func(pinned bool) {
		hookCalls++
		if pinned {
			t.Errorf("a bind with no publish and no contention pinned appends")
			return
		}
		appendedDuringScan = withinWait(func() { appendDeskEventErr(t, fx, "during-scan") })
		wroteDuringScan = withinWait(func() {
			if _, err := fx.rt.store.DB().ExecContext(ctx, `CREATE TABLE IF NOT EXISTS unheld_probe (id INTEGER PRIMARY KEY)`); err != nil {
				t.Errorf("write during scan: %v", err)
			}
		})
	}
	binding, _, err := unheldBind(t, fx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if hookCalls != 1 {
		t.Fatalf("scan hook ran %d times, want 1", hookCalls)
	}
	if !appendedDuringScan {
		t.Fatal("an append issued during the witness scan did not complete: the scan holds appends")
	}
	if !wroteDuringScan {
		t.Fatal("a write issued during the witness scan did not complete: the scan holds the write connection")
	}
	if binding.Head == nil || binding.Head.CanonicalEventHead != headBefore {
		t.Fatalf("binding names head %+v, want the head the snapshot was taken at, %s", binding.Head, headBefore)
	}
	if err := selfdevprotocol.WitnessContentMatches(witnessBefore, binding.Witness); err != nil {
		t.Fatalf("the witness is not the state at the bound head (it saw a later write): %v", err)
	}
	if fx.liveHead(t) == headBefore {
		t.Fatal("the append during the scan did not advance the live head")
	}
	witnessAfter, err := LiveStoreWitness(ctx, fx.rt.store.DB())
	if err != nil {
		t.Fatal(err)
	}
	if witnessAfter.ContentRoot == binding.Witness.ContentRoot {
		t.Fatal("the live state did not change under the scan, so the test proves nothing")
	}
}

// Failure mode 3: the same state gives the same witness as the write-connection
// extraction, and as a tape replay.
func TestCheckpointBindWitnessBytesUnchanged(t *testing.T) {
	fx := newCheckpointHeadFixture(t, "computer-bind-unheld-bytes")
	fx.appendDeskEvent(t, "desk-turn-1")
	fx.appendDeskEvent(t, "desk-turn-2")
	ctx := context.Background()
	old, err := LiveStoreWitness(ctx, fx.rt.store.DB())
	if err != nil {
		t.Fatal(err)
	}
	binding, _, err := unheldBind(t, fx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(old, binding.Witness) {
		t.Fatalf("witness changed for an unchanged store:\n old %+v\n new %+v", old, binding.Witness)
	}
	if err := selfdevprotocol.WitnessContentMatches(tapeReplayWitness(t, fx), binding.Witness); err != nil {
		t.Fatalf("witness differs from the tape replay's: %v", err)
	}
}

// Failure mode 4: the publish runs outside the append hold, and an append
// issued from inside it completes.
func TestCheckpointBindPublishRunsOutsideAppendHold(t *testing.T) {
	fx := newCheckpointHeadFixture(t, "computer-bind-unheld-publish")
	fx.appendDeskEvent(t, "before-bind")
	var published, appendedInPublish bool
	var publishedHead string
	_, response, err := unheldBind(t, fx, func(ctx context.Context, binding checkpointBinding) (selfdevprotocol.CheckpointResponse, error) {
		published = true
		publishedHead = binding.Head.CanonicalEventHead
		appendedInPublish = withinWait(func() { appendDeskEventErr(t, fx, "during-publish") })
		return selfdevprotocol.CheckpointResponse{}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !published || response == nil {
		t.Fatal("the bind did not publish")
	}
	if !appendedInPublish {
		t.Fatal("an append issued from the publish did not complete: the publish runs under the append hold")
	}
	if publishedHead == fx.liveHead(t) {
		t.Fatal("the append during publish did not advance the head, so the test proves nothing")
	}
}

// Failure mode 5: a head that moves during the first scan is not published;
// the bind re-binds pinned and publishes the head it then names, once, with
// that state's witness.
func TestCheckpointBindMovedHeadRebindsPinnedAndPublishesCurrentHead(t *testing.T) {
	fx := newCheckpointHeadFixture(t, "computer-bind-unheld-moved")
	fx.appendDeskEvent(t, "before-bind")
	ctx := context.Background()

	var mu sync.Mutex
	var scans []bool
	fx.rt.checkpointScanHook = func(pinned bool) {
		mu.Lock()
		scans = append(scans, pinned)
		mu.Unlock()
		if !pinned {
			if !withinWait(func() { appendDeskEventErr(t, fx, "during-first-scan") }) {
				t.Errorf("append during the first scan did not complete")
			}
		}
	}
	var publishedHeads []string
	var publishedWitness selfdevprotocol.VMLocalContentWitness
	binding, response, err := unheldBind(t, fx, func(ctx context.Context, binding checkpointBinding) (selfdevprotocol.CheckpointResponse, error) {
		publishedHeads = append(publishedHeads, binding.Head.CanonicalEventHead)
		publishedWitness = binding.Witness
		return selfdevprotocol.CheckpointResponse{}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if response == nil {
		t.Fatal("the bind did not publish")
	}
	if !reflect.DeepEqual(scans, []bool{false, true}) {
		t.Fatalf("scans pinned = %v, want an unpinned scan then a pinned re-bind", scans)
	}
	if len(publishedHeads) != 1 || publishedHeads[0] != fx.liveHead(t) || binding.Head.CanonicalEventHead != publishedHeads[0] {
		t.Fatalf("published heads %v, binding head %+v, live head %s: want exactly the live head", publishedHeads, binding.Head, fx.liveHead(t))
	}
	live, err := LiveStoreWitness(ctx, fx.rt.store.DB())
	if err != nil {
		t.Fatal(err)
	}
	if err := selfdevprotocol.WitnessContentMatches(live, publishedWitness); err != nil {
		t.Fatalf("the published witness is not the live state at the published head: %v", err)
	}
}
