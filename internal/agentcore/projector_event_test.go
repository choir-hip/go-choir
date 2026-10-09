package agentcore

import (
	"context"
	"errors"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// SL slice 2 (docs/definitions/choir-appdev-sl-obligation-terminality-2026-10-08.md).
// Failure modes pinned: the store signals before commit or not at all; a
// persistent dispatch error retries forever; a transient error exhausts
// without backoff; exhaustion drops the wake without a durable fate.

// queueOneActorWake commits one lifecycle wake. Seed wakes are marked
// projected first, except those addressed to keep (wakes earlier calls queued).
func queueOneActorWake(t *testing.T, s *store.Store, suffix string, keep ...string) (target string) {
	t.Helper()
	ctx := context.Background()
	ownerID := "user-wake-" + suffix
	docID := "doc-wake-" + suffix
	trajectoryID := seedDurableTextureSubject(t, s, ownerID, docID)
	target = currentTextureAgentID(docID)
	producerAgentID, producerWorkID, producerRunID := projectTestLifecycleProducer(t, s, ownerID, "autoputer-test", trajectoryID, docID, "wake-"+suffix)
	setup, err := s.ListUnprojectedActorWakes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, wake := range setup {
		if slices.Contains(keep, wake.TargetAgentID) {
			continue
		}
		if err := s.MarkActorWakeProjected(ctx, wake.CanonicalID); err != nil {
			t.Fatalf("mark setup wake projected: %v", err)
		}
	}
	select {
	case <-s.ActorWakeSignal():
	default:
	}
	packet := types.CoagentSourcePacketPayload{SchemaVersion: types.CoagentSourcePacketSchemaV1, Kind: "evidence_update", Summary: "wake " + suffix}
	req := types.QueueLifecycleUpdateRequest{
		OwnerID: ownerID, ComputerID: "autoputer-test", CommandID: "queue-wake-" + suffix,
		TrajectoryID: trajectoryID, TargetAgentID: target, ProducerAgentID: producerAgentID,
		ProducerUpdateID: "producer-wake-" + suffix, UpdateID: "update-wake-" + suffix,
		ChannelID: docID, Role: agentprofile.Research, SourceRunID: producerRunID,
		WorkItemID: producerWorkID, WorkDisposition: types.WorkItemOpen,
		Packet: packet, Content: "wake content " + suffix,
	}
	req.PayloadDigest, _ = store.ComputeLifecycleUpdatePayloadDigest(req.Packet, req.Content)
	req.CommandDigest, _ = store.ComputeQueueLifecycleUpdateDigest(req)
	if _, err := s.QueueLifecycleUpdate(ctx, req); err != nil {
		t.Fatalf("queue lifecycle update: %v", err)
	}
	return target
}

func TestStoreSignalsAfterCommittingAnActorWake(t *testing.T) {
	_, s := testRuntime(t)
	queueOneActorWake(t, s, "signal")
	select {
	case <-s.ActorWakeSignal():
	default:
		t.Fatal("committing an actor wake did not signal the projector")
	}
	wakes, err := s.ListUnprojectedActorWakes(context.Background())
	if err != nil || len(wakes) == 0 {
		t.Fatalf("signalled wake not committed: wakes=%d err=%v", len(wakes), err)
	}
}

func TestFailingWakeDispatchBacksOffThenExhaustsVisibly(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	target := queueOneActorWake(t, s, "exhaust")
	now := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	rt.wakeRetries.now = func() time.Time { return now }
	var calls atomic.Int32
	rt.SetDispatchActor(func(_ context.Context, _, _, to, _, _, _, _ string) error {
		if to == target {
			calls.Add(1)
		}
		return errors.New("actor log unavailable")
	})

	rt.sweepActorWakeOutbox(ctx)
	if got := calls.Load(); got != 1 {
		t.Fatalf("first sweep dispatches = %d, want 1", got)
	}
	rt.sweepActorWakeOutbox(ctx)
	if got := calls.Load(); got != 1 {
		t.Fatalf("retry ran before its backoff: dispatches = %d", got)
	}
	for _, step := range []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second} {
		now = now.Add(step)
		rt.sweepActorWakeOutbox(ctx)
	}
	if got := calls.Load(); got != wakeDispatchMaxAttempts {
		t.Fatalf("dispatch attempts = %d, want %d", got, wakeDispatchMaxAttempts)
	}
	wakes, err := s.ListUnprojectedActorWakes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, wake := range wakes {
		if wake.TargetAgentID == target {
			t.Fatalf("exhausted wake still in the drain: %+v", wake)
		}
	}
	exhausted, err := s.ListExhaustedActorWakes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, wake := range exhausted {
		if wake.TargetAgentID == target && wake.LastDispatchError != "" && wake.DispatchAttempts == wakeDispatchMaxAttempts {
			found = true
		}
	}
	if !found {
		t.Fatalf("exhausted wake has no durable fate: %+v", exhausted)
	}
	now = now.Add(time.Hour)
	rt.sweepActorWakeOutbox(ctx)
	if got := calls.Load(); got != wakeDispatchMaxAttempts {
		t.Fatalf("exhausted wake dispatched again: %d", got)
	}
}

// SL fault-matrix leg c: one poison wake (its dispatch always fails) must not
// stall the drain for a healthy wake queued behind it, and must reach its own
// visible fate.
func TestPoisonWakeDoesNotStallHealthyWake(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	poison := queueOneActorWake(t, s, "poison")
	healthy := queueOneActorWake(t, s, "healthy", poison)
	now := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	rt.wakeRetries.now = func() time.Time { return now }
	var healthyCalls atomic.Int32
	rt.SetDispatchActor(func(_ context.Context, _, _, to, _, _, _, _ string) error {
		if to == poison {
			return errors.New("validation: poison payload")
		}
		if to == healthy {
			healthyCalls.Add(1)
		}
		return nil
	})

	rt.sweepActorWakeOutbox(ctx)
	if got := healthyCalls.Load(); got != 1 {
		t.Fatalf("healthy wake dispatches after first sweep = %d, want 1", got)
	}
	for _, step := range []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second} {
		now = now.Add(step)
		rt.sweepActorWakeOutbox(ctx)
	}
	if got := healthyCalls.Load(); got != 1 {
		t.Fatalf("healthy wake dispatched %d times, want once", got)
	}
	wakes, err := s.ListUnprojectedActorWakes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, wake := range wakes {
		if wake.TargetAgentID == poison || wake.TargetAgentID == healthy {
			t.Fatalf("wake still owed after the drain: %+v", wake)
		}
	}
	exhausted, err := s.ListExhaustedActorWakes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var poisonFate, healthyFate bool
	for _, wake := range exhausted {
		poisonFate = poisonFate || wake.TargetAgentID == poison
		healthyFate = healthyFate || wake.TargetAgentID == healthy
	}
	if !poisonFate || healthyFate {
		t.Fatalf("exhausted fates: poison=%v healthy=%v, want poison only", poisonFate, healthyFate)
	}
}
