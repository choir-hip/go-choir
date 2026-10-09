package store

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Consume-at-commit: dispatch binds DeliveredToRunID, a dead binding rebinds or
// exhausts at the cap, and consumption itself happens inside ApplyTextureTurn.
// These tests pin the bookkeeping contract — no packet escapes pending without
// a disposition, and the breaker event is exactly-once per command id.
// Receipt: docs/problems/texture-desk-activation-contract-respawn-loop-2026-09-28.md

func seedBoundProducerReport(t *testing.T, s *Store, start types.StartLifecycleRequest, updateID, producerUpdateID, deliveredToRunID string, attempts int) types.CoagentSourcePacket {
	t.Helper()
	ctx := context.Background()
	update := types.CoagentSourcePacket{
		UpdateID:           updateID,
		OwnerID:            start.OwnerID,
		ComputerID:         start.ComputerID,
		AgentID:            "research:producer-1",
		ProducerUpdateID:   producerUpdateID,
		TargetAgentID:      start.Agent.AgentID,
		ChannelID:          start.Agent.ChannelID,
		TrajectoryID:       start.TrajectoryID,
		Role:               "research",
		Direction:          types.LifecyclePacketDirectionProducerReport,
		Disposition:        types.UpdatePending,
		DeliveredToRunID:   deliveredToRunID,
		DeliveryAttempts:   attempts,
		LifecycleVersion:   1,
		ProducerWorkItemID: "work-producer-1",
		Packet:             types.CoagentSourcePacketPayload{SchemaVersion: types.CoagentSourcePacketSchemaV1, Kind: "findings", Summary: "result"},
		Content:            "findings",
		CreatedAt:          time.Now().UTC(),
	}
	key := start.TrajectoryID + "\x00" + start.Agent.AgentID + "\x00" + update.AgentID + "\x00" + producerUpdateID
	obj, err := lifecycleObject(ogKindWorkerUpdate, start.OwnerID, start.ComputerID, key, update,
		lifecycleMetadata("update_id", updateID, start.ComputerID, start.TrajectoryID, 1), update.CreatedAt, update.CreatedAt)
	if err != nil {
		t.Fatalf("build update object: %v", err)
	}
	if err := s.ogStore.PutBatchConditional(ctx, []objectgraph.ObjectCondition{
		{CanonicalID: obj.CanonicalID, Exists: false},
	}, objectgraph.Batch{Objects: []objectgraph.Object{obj}}); err != nil {
		t.Fatalf("seed update: %v", err)
	}
	return update
}

func reconcileDeliveryForTest(t *testing.T, s *Store, start types.StartLifecycleRequest, commandID, targetRunID, breakerReason string, items []types.ReconcileUpdateDeliveryItem) types.LifecycleResult {
	t.Helper()
	req := types.ReconcileUpdateDeliveryRequest{
		OwnerID:       start.OwnerID,
		ComputerID:    start.ComputerID,
		CommandID:     commandID,
		TrajectoryID:  start.TrajectoryID,
		TargetAgentID: start.Agent.AgentID,
		TargetRunID:   targetRunID,
		MaxAttempts:   3,
		Items:         items,
		BreakerReason: breakerReason,
	}
	digest, err := ComputeReconcileUpdateDeliveryDigest(req)
	if err != nil {
		t.Fatalf("digest: %v", err)
	}
	req.CommandDigest = digest
	res, err := s.ReconcileUpdateDelivery(context.Background(), req)
	if err != nil {
		t.Fatalf("reconcile %s: %v", commandID, err)
	}
	return res
}

func deliveryItemFor(update types.CoagentSourcePacket, expectedRunID string, exhaust bool) types.ReconcileUpdateDeliveryItem {
	return types.ReconcileUpdateDeliveryItem{
		UpdateID:                 update.UpdateID,
		ProducerAgentID:          update.AgentID,
		ProducerUpdateID:         update.ProducerUpdateID,
		ExpectedRunID:            expectedRunID,
		ExpectedLifecycleVersion: update.LifecycleVersion,
		Exhaust:                  exhaust,
	}
}

func getSeededUpdate(t *testing.T, s *Store, start types.StartLifecycleRequest, update types.CoagentSourcePacket) types.CoagentSourcePacket {
	t.Helper()
	key := start.TrajectoryID + "\x00" + start.Agent.AgentID + "\x00" + update.AgentID + "\x00" + update.ProducerUpdateID
	_, got, err := s.textureTurnUpdateObject(context.Background(), start.OwnerID, start.ComputerID, key)
	if err != nil {
		t.Fatalf("reload update %s: %v", update.UpdateID, err)
	}
	return got
}

func TestReconcileUpdateDeliveryBindsPendingToLiveRun(t *testing.T) {
	s := openTestStore(t)
	start := lifecycleStartFixture()
	if _, err := s.StartLifecycle(context.Background(), start); err != nil {
		t.Fatalf("start lifecycle: %v", err)
	}
	u := seedBoundProducerReport(t, s, start, "update-bind-1", "producer-update-1", "", 0)

	res := reconcileDeliveryForTest(t, s, start, "cmd-bind-1", "run-desk-1", "",
		[]types.ReconcileUpdateDeliveryItem{deliveryItemFor(u, "", false)})

	got := getSeededUpdate(t, s, start, u)
	if got.Disposition != types.UpdatePending || got.DeliveredToRunID != "run-desk-1" || got.DeliveryAttempts != 1 {
		t.Fatalf("bind did not claim packet: %+v", got)
	}
	if len(res.Events) != 1 || res.Events[0].Kind != types.LifecycleUpdateDelivered {
		t.Fatalf("missing delivered event: %+v", res.Events)
	}
}

func TestReconcileUpdateDeliveryRebindsStrandedAndExhaustsAtCap(t *testing.T) {
	s := openTestStore(t)
	start := lifecycleStartFixture()
	if _, err := s.StartLifecycle(context.Background(), start); err != nil {
		t.Fatalf("start lifecycle: %v", err)
	}
	// Bound to a dead run, attempts 1 < cap 3 → rebinds to the fresh run.
	stranded := seedBoundProducerReport(t, s, start, "update-rebind-1", "producer-update-rb", "run-dead-1", 1)
	// At cap → terminalizes delivered, clears the claim, never rebinds.
	dead := seedBoundProducerReport(t, s, start, "update-exhaust-1", "producer-update-ex", "run-dead-1", 3)

	res := reconcileDeliveryForTest(t, s, start, "cmd-rebind-1", "run-desk-2", "",
		[]types.ReconcileUpdateDeliveryItem{
			deliveryItemFor(stranded, "run-dead-1", false),
			deliveryItemFor(dead, "run-dead-1", false),
		})

	gotStranded := getSeededUpdate(t, s, start, stranded)
	if gotStranded.Disposition != types.UpdatePending || gotStranded.DeliveredToRunID != "run-desk-2" || gotStranded.DeliveryAttempts != 2 {
		t.Fatalf("stranded packet did not rebind: %+v", gotStranded)
	}
	gotDead := getSeededUpdate(t, s, start, dead)
	if gotDead.Disposition != types.UpdateDelivered || gotDead.DeliveredToRunID != "" || gotDead.DispositionReason != "delivery_attempts_exhausted" {
		t.Fatalf("at-cap packet did not terminalize: %+v", gotDead)
	}
	delivered, activationFailed := 0, 0
	for _, ev := range res.Events {
		switch ev.Kind {
		case types.LifecycleUpdateDelivered:
			delivered++
		case types.LifecycleTextureActivationFailed:
			activationFailed++
		}
	}
	if delivered != 1 || activationFailed != 1 {
		t.Fatalf("expected 1 delivered + 1 activation_failed event, got %+v", res.Events)
	}
}

func TestReconcileUpdateDeliveryBreakerEmitsOncePerCommand(t *testing.T) {
	s := openTestStore(t)
	start := lifecycleStartFixture()
	if _, err := s.StartLifecycle(context.Background(), start); err != nil {
		t.Fatalf("start lifecycle: %v", err)
	}
	reason := "activation_breaker: 3 consecutive desk activations terminated without a committed turn"
	res1 := reconcileDeliveryForTest(t, s, start, "cmd-breaker-1", "", reason, nil)
	res2 := reconcileDeliveryForTest(t, s, start, "cmd-breaker-1", "", reason, nil)

	if len(res1.Events) != 1 || res1.Events[0].Kind != types.LifecycleTextureActivationFailed || res1.Events[0].Reason != reason {
		t.Fatalf("breaker event missing or wrong: %+v", res1.Events)
	}
	// Replay returns the recorded result — same event identity, no re-emission.
	if len(res2.Events) != 1 || res2.Events[0].EventID != res1.Events[0].EventID {
		t.Fatalf("replay must return the recorded breaker event, got %+v", res2.Events)
	}
}

func TestReconcileUpdateDeliveryConflictsOnStaleClaim(t *testing.T) {
	s := openTestStore(t)
	start := lifecycleStartFixture()
	if _, err := s.StartLifecycle(context.Background(), start); err != nil {
		t.Fatalf("start lifecycle: %v", err)
	}
	u := seedBoundProducerReport(t, s, start, "update-conflict-1", "producer-update-c1", "run-dead-9", 0)

	// ExpectedRunID disagrees with the stored claim → conflict, no mutation.
	item := deliveryItemFor(u, "run-other", false)
	req := types.ReconcileUpdateDeliveryRequest{
		OwnerID:       start.OwnerID,
		ComputerID:    start.ComputerID,
		CommandID:     "cmd-conflict-1",
		TrajectoryID:  start.TrajectoryID,
		TargetAgentID: start.Agent.AgentID,
		TargetRunID:   "run-desk-3",
		Items:         []types.ReconcileUpdateDeliveryItem{item},
	}
	digest, err := ComputeReconcileUpdateDeliveryDigest(req)
	if err != nil {
		t.Fatalf("digest: %v", err)
	}
	req.CommandDigest = digest
	if _, err := s.ReconcileUpdateDelivery(context.Background(), req); err != ErrLifecycleCommandConflict {
		t.Fatalf("expected ErrLifecycleCommandConflict, got %v", err)
	}
	got := getSeededUpdate(t, s, start, u)
	if got.DeliveredToRunID != "run-dead-9" || got.DeliveryAttempts != 0 {
		t.Fatalf("conflict mutated the packet: %+v", got)
	}
}

func TestReconcileUpdateDeliveryForcedExhaust(t *testing.T) {
	s := openTestStore(t)
	start := lifecycleStartFixture()
	if _, err := s.StartLifecycle(context.Background(), start); err != nil {
		t.Fatalf("start lifecycle: %v", err)
	}
	// Breaker-tripped path: under-cap packets are terminalized by Exhaust:true.
	u := seedBoundProducerReport(t, s, start, "update-bx-1", "producer-update-bx", "run-dead-4", 0)

	res := reconcileDeliveryForTest(t, s, start, "cmd-bx-1", "",
		"activation_breaker: breaker tripped",
		[]types.ReconcileUpdateDeliveryItem{deliveryItemFor(u, "run-dead-4", true)})

	got := getSeededUpdate(t, s, start, u)
	if got.Disposition != types.UpdateDelivered || got.DispositionReason != "delivery_attempts_exhausted" {
		t.Fatalf("forced exhaust did not terminalize: %+v", got)
	}
	failed := 0
	for _, ev := range res.Events {
		if ev.Kind == types.LifecycleTextureActivationFailed {
			failed++
		}
	}
	if failed != 2 { // one per-item exhaustion event + one breaker event
		t.Fatalf("expected 2 activation_failed events, got %d: %+v", failed, res.Events)
	}
}

// Regression for the stranded-rebind defect: a control packet bound to a dead
// carrier must, on pure unbind (TargetRunID=""), clear DeliveredAt so the
// pending scan sees it again. Before the fix the reducer re-stamped
// DeliveredAt=&now on unbind, so the freed packet failed the
// DeliveredAt==nil filter in ListPendingLifecycleUpdates and never rebound.
// Receipt: docs/problems/s0m-freed-control-never-rebinds-2026-10-03.md
func seedBoundControlPacket(t *testing.T, s *Store, start types.StartLifecycleRequest, updateID, producerUpdateID, deliveredToRunID, workItemID string, attempts int) types.CoagentSourcePacket {
	t.Helper()
	ctx := context.Background()
	now := time.Now().UTC()
	var deliveredAt *time.Time
	if strings.TrimSpace(deliveredToRunID) != "" {
		deliveredAt = &now
	}
	update := types.CoagentSourcePacket{
		UpdateID:         updateID,
		OwnerID:          start.OwnerID,
		ComputerID:       start.ComputerID,
		AgentID:          "research:desk-dead",
		ProducerUpdateID: producerUpdateID,
		TargetAgentID:    start.Agent.AgentID,
		TargetWorkItemID: workItemID,
		ChannelID:        start.Agent.ChannelID,
		TrajectoryID:     start.TrajectoryID,
		Role:             "texture",
		Direction:        types.LifecyclePacketDirectionControl,
		Disposition:      types.UpdatePending,
		DeliveredToRunID: deliveredToRunID,
		DeliveredAt:      deliveredAt,
		DeliveryAttempts: attempts,
		LifecycleVersion: 3,
		Packet:           types.CoagentSourcePacketPayload{SchemaVersion: types.CoagentSourcePacketSchemaV1, Kind: "question", Summary: "ask"},
		Content:          "control ask",
		CreatedAt:        now,
	}
	key := start.TrajectoryID + "\x00" + start.Agent.AgentID + "\x00" + update.AgentID + "\x00" + producerUpdateID
	obj, err := lifecycleObject(ogKindWorkerUpdate, start.OwnerID, start.ComputerID, key, update,
		lifecycleMetadata("update_id", updateID, start.ComputerID, start.TrajectoryID, 3), update.CreatedAt, update.CreatedAt)
	if err != nil {
		t.Fatalf("build control update: %v", err)
	}
	if err := s.ogStore.PutBatchConditional(ctx, []objectgraph.ObjectCondition{
		{CanonicalID: obj.CanonicalID, Exists: false},
	}, objectgraph.Batch{Objects: []objectgraph.Object{obj}}); err != nil {
		t.Fatalf("seed control update: %v", err)
	}
	return update
}

func TestReconcileUpdateDeliveryUnbindClearsDeliveredAtForPendingRescan(t *testing.T) {
	s := openTestStore(t)
	start := lifecycleStartFixture()
	if _, err := s.StartLifecycle(context.Background(), start); err != nil {
		t.Fatalf("start lifecycle: %v", err)
	}
	// Control bound to a dead run, DeliveredAt set — the live claim the unbind
	// must clear so the freed packet re-enters the pending scan.
	u := seedBoundControlPacket(t, s, start, "update-unbind-1", "producer-update-ub", "run-dead-carrier", "work-ask-1", 1)

	// Pure unbind: TargetRunID="" reclaims the dead claim, must NOT re-stamp
	// DeliveredAt. Attempts increments (the dead-claim retry budget).
	res := reconcileDeliveryForTest(t, s, start, "cmd-unbind-1", "", "",
		[]types.ReconcileUpdateDeliveryItem{deliveryItemFor(u, "run-dead-carrier", false)})

	got := getSeededUpdate(t, s, start, u)
	if got.Disposition != types.UpdatePending || got.DeliveredToRunID != "" {
		t.Fatalf("unbind did not release the claim: %+v", got)
	}
	if got.DeliveredAt != nil {
		t.Fatalf("unbind left DeliveredAt stamped — packet invisible to pending scan: %+v", got)
	}
	if got.DeliveryAttempts != 2 {
		t.Fatalf("dead-claim reclaim should count one attempt, got %d", got.DeliveryAttempts)
	}
	// Re-queue event, not a false bound_to_activation.
	if len(res.Events) != 1 || res.Events[0].Kind != types.LifecycleControlQueued || res.Events[0].Reason != "released_to_pending" {
		t.Fatalf("expected released_to_pending requeue event, got %+v", res.Events)
	}
	// The freed packet is now visible to the pending scan — the exact predicate
	// reconcileUpdatedCoagentActor uses to mint a fresh carrier.
	pending, err := s.ListAllPendingLifecycleUpdates(context.Background(), start.OwnerID, start.ComputerID, start.Agent.AgentID)
	if err != nil {
		t.Fatalf("list pending: %v", err)
	}
	found := false
	for _, p := range pending {
		if p.UpdateID == "update-unbind-1" {
			found = true
		}
	}
	if !found {
		t.Fatalf("freed control packet still invisible to pending scan: %+v", pending)
	}
}
