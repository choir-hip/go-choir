package store

import (
	"context"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Regression: a pending lifecycle control bound to a delivery run
// (delivered_to_loop_id set) that crashed or passivated before consuming the
// control must still mint an actor wake on boot migration. The prior guard
// suppressed the wake on the mere presence of the binding, stranding the
// control with no delivery path (research:cfa90b87 / update 4158e48b).
func TestMigrateActorWakeOutboxReArmsDeliveredToPassivatedRun(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	ownerID, computerID := "owner-wake", "computer-wake"
	boundRunID := "run-bound-passivated"

	writeRun := func(state types.RunState) {
		run := types.RunRecord{
			RunID: boundRunID, AgentID: "research:cfa90b87-x", OwnerID: ownerID,
			ComputerID: computerID, TrajectoryID: "traj-wake", AgentProfile: "research",
			AgentRole: "research", State: state, Prompt: "cell",
			CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
		}
		if state.Terminal() {
			now := time.Now().UTC()
			run.FinishedAt = &now
		}
		if err := s.CreateRunOG(ctx, run); err != nil {
			t.Fatalf("write run: %v", err)
		}
	}
	writeRun(types.RunPassivated)

	deliveredAt := time.Now().UTC()
	update := types.CoagentSourcePacket{
		UpdateID: "upd-bound-control", OwnerID: ownerID, ComputerID: computerID,
		AgentID: "texture:ch", TargetAgentID: "research:cfa90b87-x", ChannelID: "ch",
		TrajectoryID: "traj-wake", Role: "texture", LifecycleVersion: 1,
		Disposition: types.UpdatePending, Direction: types.LifecyclePacketDirectionControl,
		Packet:           types.CoagentSourcePacketPayload{SchemaVersion: types.CoagentSourcePacketSchemaV1, Kind: "execution_request", Summary: "q"},
		Content:          "bound question", CreatedAt: deliveredAt,
		DeliveredToRunID: boundRunID, DeliveredAt: &deliveredAt,
	}
	canon, err := lifecycleCanonicalID(ogKindWorkerUpdate, ownerID, computerID, update.UpdateID)
	if err != nil {
		t.Fatalf("canon: %v", err)
	}
	uobj, err := lifecycleObject(ogKindWorkerUpdate, ownerID, computerID, update.UpdateID, update,
		lifecycleMetadata("update_id", update.UpdateID, computerID, update.TrajectoryID, 1), update.CreatedAt, update.CreatedAt)
	if err != nil {
		t.Fatalf("build update: %v", err)
	}
	uobj.CanonicalID = canon
	if err := s.ogStore.PutBatchConditional(ctx, []objectgraph.ObjectCondition{
		{CanonicalID: uobj.CanonicalID, Exists: false},
	}, objectgraph.Batch{Objects: []objectgraph.Object{uobj}}); err != nil {
		t.Fatalf("write update: %v", err)
	}

	minted, err := s.MigrateActorWakeOutbox(ctx)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if minted == 0 {
		t.Fatal("migration minted no wake for delivered-to-passivated control")
	}
	wakes, err := s.ListUnprojectedActorWakes(ctx)
	if err != nil {
		t.Fatalf("list wakes: %v", err)
	}
	found := false
	for _, w := range wakes {
		if w.SourceUpdateID == update.UpdateID && w.TargetAgentID == update.TargetAgentID {
			found = true
		}
	}
	if !found {
		t.Fatalf("delivered-to-passivated control wake not minted: %+v", wakes)
	}
}

// Companion guard: a pending control bound to a still-Active run must NOT
// mint a duplicate wake (the live run will inject it).
func TestMigrateActorWakeOutboxSuppressesDeliveredToActiveRun(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	ownerID, computerID := "owner-wake2", "computer-wake2"
	boundRunID := "run-bound-active"

	run := types.RunRecord{
		RunID: boundRunID, AgentID: "research:live-x", OwnerID: ownerID,
		ComputerID: computerID, TrajectoryID: "traj-wake2", AgentProfile: "research",
		AgentRole: "research", State: types.RunRunning, Prompt: "cell",
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}
	if err := s.CreateRunOG(ctx, run); err != nil {
		t.Fatalf("write run: %v", err)
	}

	deliveredAt := time.Now().UTC()
	update := types.CoagentSourcePacket{
		UpdateID: "upd-bound-active", OwnerID: ownerID, ComputerID: computerID,
		AgentID: "texture:ch", TargetAgentID: "research:live-x", ChannelID: "ch",
		TrajectoryID: "traj-wake2", Role: "texture", LifecycleVersion: 1,
		Disposition: types.UpdatePending, Direction: types.LifecyclePacketDirectionControl,
		Packet:           types.CoagentSourcePacketPayload{SchemaVersion: types.CoagentSourcePacketSchemaV1, Kind: "execution_request", Summary: "q"},
		Content:          "bound question", CreatedAt: deliveredAt,
		DeliveredToRunID: boundRunID, DeliveredAt: &deliveredAt,
	}
	canon, err := lifecycleCanonicalID(ogKindWorkerUpdate, ownerID, computerID, update.UpdateID)
	if err != nil {
		t.Fatalf("canon: %v", err)
	}
	uobj, err := lifecycleObject(ogKindWorkerUpdate, ownerID, computerID, update.UpdateID, update,
		lifecycleMetadata("update_id", update.UpdateID, computerID, update.TrajectoryID, 1), update.CreatedAt, update.CreatedAt)
	if err != nil {
		t.Fatalf("build update: %v", err)
	}
	uobj.CanonicalID = canon
	if err := s.ogStore.PutBatchConditional(ctx, []objectgraph.ObjectCondition{
		{CanonicalID: uobj.CanonicalID, Exists: false},
	}, objectgraph.Batch{Objects: []objectgraph.Object{uobj}}); err != nil {
		t.Fatalf("write update: %v", err)
	}

	if _, err := s.MigrateActorWakeOutbox(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	wakes, err := s.ListUnprojectedActorWakes(ctx)
	if err != nil {
		t.Fatalf("list wakes: %v", err)
	}
	for _, w := range wakes {
		if w.SourceUpdateID == update.UpdateID {
			t.Fatalf("active-bound control minted duplicate wake: %+v", w)
		}
	}
}
