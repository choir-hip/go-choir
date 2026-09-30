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
		Packet:  types.CoagentSourcePacketPayload{SchemaVersion: types.CoagentSourcePacketSchemaV1, Kind: "execution_request", Summary: "q"},
		Content: "bound question", CreatedAt: deliveredAt,
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
		Packet:  types.CoagentSourcePacketPayload{SchemaVersion: types.CoagentSourcePacketSchemaV1, Kind: "execution_request", Summary: "q"},
		Content: "bound question", CreatedAt: deliveredAt,
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

// Diagnostic: does the resolver actually find the bound run object?
// If getRunObjectByOwnerOG misses (canonical-ID scheme mismatch), the
// stranded-run check fails open (mints anyway) — but we want to prove the
// Active() suppression path is what the resolver fed, not an absent run.
func TestResolverFindsBoundRun(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	ownerID, computerID := "owner-res", "computer-res"
	boundRunID := "run-res-passivated"

	run := types.RunRecord{
		RunID: boundRunID, AgentID: "research:r-x", OwnerID: ownerID,
		ComputerID: computerID, TrajectoryID: "traj-res", AgentProfile: "research",
		AgentRole: "research", State: types.RunPassivated, Prompt: "cell",
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}
	if err := s.CreateRunOG(ctx, run); err != nil {
		t.Fatalf("write run: %v", err)
	}
	obj, err := s.getRunObjectByOwnerOG(ctx, ownerID, boundRunID)
	if err != nil {
		t.Fatalf("resolver lookup miss: %v", err)
	}
	rec, derr := decodeLifecycleObject[types.RunRecord](obj)
	if derr != nil || rec.State != types.RunPassivated {
		t.Fatalf("resolver run decode: %v state=%s", derr, rec.State)
	}
	t.Logf("resolver found bound run %s state=%s", rec.RunID, rec.State)
}

// The delivered-not-consumed strand is not just an unprojected wake: at the
// 17:12 commit a wake was minted AND projected (delivered). The run then died
// before consuming. A second boot migration must re-arm that projected wake —
// not skip it — because the obligation is still open. Before the fix the
// Exists:false mint condition dropped every projected wake forever.
func TestMigrateActorWakeOutboxRearmsProjectedWakeForOpenBoundControl(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	ownerID, computerID := "owner-rearm", "computer-rearm"
	boundRunID := "run-rearm-dead"

	run := types.RunRecord{
		RunID: boundRunID, AgentID: "research:re-x", OwnerID: ownerID,
		ComputerID: computerID, TrajectoryID: "traj-re", AgentProfile: "research",
		AgentRole: "research", State: types.RunPassivated, Prompt: "cell",
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}
	if err := s.CreateRunOG(ctx, run); err != nil {
		t.Fatalf("write run: %v", err)
	}
	deliveredAt := time.Now().UTC()
	update := types.CoagentSourcePacket{
		UpdateID: "upd-rearm", OwnerID: ownerID, ComputerID: computerID,
		AgentID: "texture:ch", TargetAgentID: "research:re-x", ChannelID: "ch",
		TrajectoryID: "traj-re", Role: "texture", LifecycleVersion: 1,
		Disposition: types.UpdatePending, Direction: types.LifecyclePacketDirectionControl,
		Packet:  types.CoagentSourcePacketPayload{SchemaVersion: types.CoagentSourcePacketSchemaV1, Kind: "execution_request", Summary: "q"},
		Content: "bound q", CreatedAt: deliveredAt,
		DeliveredToRunID: boundRunID, DeliveredAt: &deliveredAt,
	}
	canon, _ := lifecycleCanonicalID(ogKindWorkerUpdate, ownerID, computerID, update.UpdateID)
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

	// First migration mints the wake (fresh).
	if minted, err := s.MigrateActorWakeOutbox(ctx); err != nil || minted == 0 {
		t.Fatalf("first migration minted=%d err=%v", minted, err)
	}
	wakes, _ := s.ListUnprojectedActorWakes(ctx)
	var wakeID string
	for _, w := range wakes {
		if w.SourceUpdateID == update.UpdateID {
			wakeID = w.CanonicalID
		}
	}
	if wakeID == "" {
		t.Fatal("no wake minted for bound control")
	}
	// Projector drains it: mark projected (as sweepActorWakeOutbox does after
	// the actor append succeeds). This simulates the delivered-then-crashed run.
	if err := s.MarkActorWakeProjected(ctx, wakeID); err != nil {
		t.Fatalf("mark projected: %v", err)
	}
	wakesAfterProject, _ := s.ListUnprojectedActorWakes(ctx)
	for _, w := range wakesAfterProject {
		if w.CanonicalID == wakeID {
			t.Fatal("wake still unprojected after mark")
		}
	}
	// Second migration must re-arm the projected wake because the obligation is
	// still open (control pending, bound run dead).
	if minted2, err := s.MigrateActorWakeOutbox(ctx); err != nil {
		t.Fatalf("second migration: %v", err)
	} else if minted2 == 0 {
		t.Fatal("second migration re-armed no wakes for open obligation")
	}
	wakesFinal, _ := s.ListUnprojectedActorWakes(ctx)
	rearmed := false
	for _, w := range wakesFinal {
		if w.CanonicalID == wakeID {
			rearmed = true
		}
	}
	if !rearmed {
		t.Fatal("projected wake was not re-armed for open obligation")
	}
}
