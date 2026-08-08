package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

func bindControlRequestForTest(t *testing.T, s *Store, start types.StartLifecycleRequest, run types.RunRecord, controls []types.CoagentSourcePacket) types.BindLifecycleControlDeliveryRequest {
	t.Helper()
	snapshot, err := s.GetLifecycleSnapshot(context.Background(), start.OwnerID, start.ComputerID, start.TrajectoryID)
	if err != nil {
		t.Fatal(err)
	}
	items := make([]types.BindLifecycleControlDeliveryItem, 0, len(controls))
	for _, control := range controls {
		items = append(items, types.BindLifecycleControlDeliveryItem{UpdateID: control.UpdateID, ProducerAgentID: control.AgentID, ProducerUpdateID: control.ProducerUpdateID, TargetWorkItemID: control.TargetWorkItemID})
	}
	req := types.BindLifecycleControlDeliveryRequest{OwnerID: start.OwnerID, ComputerID: start.ComputerID, CommandID: "bind-delivery:" + run.RunID, TrajectoryID: start.TrajectoryID, TargetAgentID: run.AgentID, TargetRunID: run.RunID, ExpectedLifecycleVersion: snapshot.Trajectory.LifecycleVersion, Controls: items}
	req.CommandDigest, err = ComputeBindLifecycleControlDeliveryDigest(req)
	if err != nil {
		t.Fatal(err)
	}
	return req
}

func TestBindLifecycleControlDeliveryResearcherAtomicReplayAndPendingExclusion(t *testing.T) {
	s, start, caller, researcherWork := setupLifecycleTextureTargetFixture(t)
	req := textureTurnBaseRequest(t, s, start, caller, types.TextureTurnWait)
	req.CommandID = "turn-for-delivery"
	req.Controls = []types.TextureTurnControl{textureTurnControl(t, "control-delivery", researcherWork.AssignedAgentID, researcherWork.WorkItemID)}
	setTextureTurnDigest(t, &req, TextureSourceGraphWriteSet{})
	turn, err := s.ApplyTextureTurn(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	run, err := s.GetLifecycleRun(context.Background(), start.OwnerID, start.ComputerID, "run-researcher-target")
	if err != nil {
		t.Fatal(err)
	}
	bind := bindControlRequestForTest(t, s, start, run, turn.Controls)
	result, err := s.BindLifecycleControlDelivery(context.Background(), bind)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Controls) != 1 || result.Controls[0].DeliveredToRunID != run.RunID || result.Controls[0].DeliveredAt == nil {
		t.Fatalf("delivery result = %+v", result.Controls)
	}
	pending, err := s.ListPendingLifecycleUpdates(context.Background(), start.OwnerID, start.ComputerID, run.AgentID, 100)
	if err != nil || len(pending) != 0 {
		t.Fatalf("delivered control remains pending: %+v %v", pending, err)
	}
	replay, err := s.BindLifecycleControlDelivery(context.Background(), bind)
	if err != nil || !replay.Replay {
		t.Fatalf("equal delivery replay = %+v %v", replay, err)
	}
	changed := bind
	changed.TargetRunID = "other-run"
	changed.CommandDigest, _ = ComputeBindLifecycleControlDeliveryDigest(changed)
	if _, err := s.BindLifecycleControlDelivery(context.Background(), changed); !errors.Is(err, ErrLifecycleCommandConflict) {
		t.Fatalf("changed delivery replay err=%v", err)
	}
}

func TestBindLifecycleControlDeliveryExactPersistentSuperRemainsNonLifecycle(t *testing.T) {
	s, start, caller, _ := setupLifecycleTextureTargetFixture(t)
	now := time.Now().UTC()
	superID := "super:" + start.OwnerID
	if err := s.UpsertAgent(context.Background(), types.AgentRecord{AgentID: superID, OwnerID: start.OwnerID, ComputerID: start.ComputerID, SandboxID: start.ComputerID, Profile: "super", Role: "super", ChannelID: superID, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	packet := textureTurnControlPacket("persistent-super")
	digest, _ := ComputeLifecycleUpdatePayloadDigest(packet, "control persistent-super")
	workID := "persistent-super-work"
	turnReq := textureTurnBaseRequest(t, s, start, caller, types.TextureTurnWait)
	turnReq.CommandID = "turn-persistent-super-delivery"
	turnReq.Controls = []types.TextureTurnControl{{ControlID: "control-persistent-super", TargetAgentID: superID, TargetWorkItemID: workID, OpenWork: &types.WorkItemRecord{WorkItemID: workID, Objective: "execute exact request", AuthorityProfile: "super", AssignedAgentID: superID}, Packet: packet, Content: "control persistent-super", PayloadDigest: digest}}
	setTextureTurnDigest(t, &turnReq, TextureSourceGraphWriteSet{})
	turn, err := s.ApplyTextureTurn(context.Background(), turnReq)
	if err != nil {
		t.Fatal(err)
	}
	run := types.RunRecord{RunID: "persistent-super-run", OwnerID: start.OwnerID, SandboxID: start.ComputerID, AgentID: superID, AgentProfile: "super", AgentRole: "super", ChannelID: superID, State: types.RunRunning, Metadata: map[string]any{"assignment_trajectory_id": start.TrajectoryID, "work_item_ids": []string{workID}, "lifecycle_work_item_id": workID}, CreatedAt: now, UpdatedAt: now}
	if err := s.CreateRun(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	bind := bindControlRequestForTest(t, s, start, run, turn.Controls)
	if _, err := s.BindLifecycleControlDelivery(context.Background(), bind); err != nil {
		t.Fatal(err)
	}
	stored, err := s.GetRunByOwner(context.Background(), start.OwnerID, run.RunID)
	if err != nil || stored.TrajectoryID != "" {
		t.Fatalf("persistent Super promoted to lifecycle: %+v %v", stored, err)
	}
	bindings, ok := stored.Metadata["lifecycle_control_bindings"].([]any)
	if !ok || len(bindings) != 1 { // JSON decode from OG uses []any.
		t.Fatalf("persistent Super lacks ordered control bindings: %#v", stored.Metadata["lifecycle_control_bindings"])
	}
}
