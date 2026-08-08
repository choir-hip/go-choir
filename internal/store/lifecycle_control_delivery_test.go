package store

import (
	"context"
	"errors"
	"path/filepath"
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
	delivered, err := s.ListLifecycleControlsDeliveredToRun(context.Background(), start.OwnerID, start.ComputerID, start.TrajectoryID, run.AgentID, run.RunID, 100)
	if err != nil || len(delivered) != 1 || delivered[0].UpdateID != turn.Controls[0].UpdateID || delivered[0].Content != turn.Controls[0].Content || delivered[0].Packet.Kind != turn.Controls[0].Packet.Kind {
		t.Fatalf("exact delivered control payload = %+v, %v", delivered, err)
	}
	if _, err := s.ListLifecycleControlsDeliveredToRun(context.Background(), start.OwnerID, start.ComputerID, start.TrajectoryID, run.AgentID, "other-run", 100); !errors.Is(err, ErrNotFound) {
		t.Fatalf("other run read exact control: %v", err)
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
	binding, ok := bindings[0].(map[string]any)
	if !ok || binding["trajectory_id"] != start.TrajectoryID || binding["update_id"] != turn.Controls[0].UpdateID || binding["target_work_item_id"] != workID {
		t.Fatalf("persistent Super decoded binding mismatch: %#v", bindings[0])
	}
	delivered, err := s.ListLifecycleControlsDeliveredToRun(context.Background(), start.OwnerID, start.ComputerID, start.TrajectoryID, superID, run.RunID, 10)
	if err != nil || len(delivered) != 1 || delivered[0].Content != "control persistent-super" || delivered[0].Packet.Kind != "execution_request" {
		t.Fatalf("persistent Super exact delivered payload=%+v err=%v", delivered, err)
	}
}

func TestDeliveredLifecycleControlReaderSurvivesStoreRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "delivered-control-restart.db")
	first, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	_, start, caller, researcherWork := setupLifecycleTextureTargetFixtureWithStore(t, first)
	request := textureTurnBaseRequest(t, first, start, caller, types.TextureTurnWait)
	request.CommandID = "turn-delivery-restart"
	request.Controls = []types.TextureTurnControl{textureTurnControl(t, "control-delivery-restart", researcherWork.AssignedAgentID, researcherWork.WorkItemID)}
	setTextureTurnDigest(t, &request, TextureSourceGraphWriteSet{})
	turn, err := first.ApplyTextureTurn(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	run, err := first.GetLifecycleRun(context.Background(), start.OwnerID, start.ComputerID, "run-researcher-target")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := first.BindLifecycleControlDelivery(context.Background(), bindControlRequestForTest(t, first, start, run, turn.Controls)); err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	second, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	pending, err := second.ListPendingLifecycleUpdates(context.Background(), start.OwnerID, start.ComputerID, run.AgentID, 10)
	if err != nil || len(pending) != 0 {
		t.Fatalf("restart new-run selection=%+v err=%v", pending, err)
	}
	delivered, err := second.ListLifecycleControlsDeliveredToRun(context.Background(), start.OwnerID, start.ComputerID, start.TrajectoryID, run.AgentID, run.RunID, 10)
	if err != nil || len(delivered) != 1 || delivered[0].UpdateID != turn.Controls[0].UpdateID || delivered[0].Content != turn.Controls[0].Content || delivered[0].Packet.Kind != turn.Controls[0].Packet.Kind {
		t.Fatalf("restart exact delivered payload=%+v err=%v", delivered, err)
	}
}
