package store

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
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
		work, err := s.GetLifecycleWorkItem(context.Background(), start.OwnerID, start.ComputerID, control.TargetWorkItemID)
		if err != nil {
			t.Fatal(err)
		}
		items = append(items, types.BindLifecycleControlDeliveryItem{UpdateID: control.UpdateID, ProducerAgentID: control.AgentID, ProducerUpdateID: control.ProducerUpdateID, TargetWorkItemID: control.TargetWorkItemID, ExpectedControlLifecycleVersion: control.LifecycleVersion, ExpectedWorkLifecycleVersion: work.LifecycleVersion})
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

func TestBindLifecycleControlDeliveryVersionsAndActivationRefreshFateShare(t *testing.T) {
	s, start, caller, researcherWork := setupLifecycleTextureTargetFixture(t)
	turnReq := textureTurnBaseRequest(t, s, start, caller, types.TextureTurnWait)
	turnReq.CommandID = "turn-versioned-refresh"
	turnReq.Controls = []types.TextureTurnControl{textureTurnControl(t, "control-versioned-refresh", researcherWork.AssignedAgentID, researcherWork.WorkItemID)}
	setTextureTurnDigest(t, &turnReq, TextureSourceGraphWriteSet{})
	turn, err := s.ApplyTextureTurn(context.Background(), turnReq)
	if err != nil {
		t.Fatal(err)
	}
	run, err := s.GetLifecycleRun(context.Background(), start.OwnerID, start.ComputerID, "run-researcher-target")
	if err != nil {
		t.Fatal(err)
	}
	stale := bindControlRequestForTest(t, s, start, run, turn.Controls)
	oldWork, err := s.GetLifecycleWorkItem(context.Background(), start.OwnerID, start.ComputerID, researcherWork.WorkItemID)
	if err != nil {
		t.Fatal(err)
	}
	stale.ActivationRefresh = &types.LifecycleControlActivationRefresh{Prompt: "old objective prompt", LogicalActivationKey: "sha256:logical", FailedAttemptKey: "sha256:attempt-old", BuildCommit: "build-a", Versions: []types.LifecycleControlActivationVersion{{UpdateID: turn.Controls[0].UpdateID, TargetWorkItemID: oldWork.WorkItemID, ControlLifecycleVersion: turn.Controls[0].LifecycleVersion, WorkLifecycleVersion: oldWork.LifecycleVersion}}, WorkItemIDs: []string{oldWork.WorkItemID}}
	stale.CommandDigest, _ = ComputeBindLifecycleControlDeliveryDigest(stale)

	amended := oldWork
	amended.Objective = "new canonical objective"
	amend := types.AmendLifecycleWorkRequest{OwnerID: start.OwnerID, ComputerID: start.ComputerID, CommandID: "amend-before-bind", TrajectoryID: start.TrajectoryID, WorkItemID: oldWork.WorkItemID, ExpectedLifecycleVersion: oldWork.LifecycleVersion, WorkItem: amended}
	amend.CommandDigest, _ = ComputeAmendLifecycleWorkDigest(amend)
	if _, err := s.AmendLifecycleWork(context.Background(), amend); err != nil {
		t.Fatal(err)
	}
	if _, err := s.BindLifecycleControlDelivery(context.Background(), stale); !errors.Is(err, ErrConcurrentStateChange) {
		t.Fatalf("stale bind err=%v", err)
	}
	pending, err := s.GetLifecycleUpdate(context.Background(), start.OwnerID, start.ComputerID, start.TrajectoryID, run.AgentID, turn.Controls[0].AgentID, turn.Controls[0].ProducerUpdateID)
	if err != nil || pending.DeliveredAt != nil || pending.DeliveredToRunID != "" {
		t.Fatalf("stale bind delivered=%+v err=%v", pending, err)
	}
	unchanged, err := s.GetLifecycleRun(context.Background(), start.OwnerID, start.ComputerID, run.RunID)
	if err != nil || unchanged.Prompt == "old objective prompt" {
		t.Fatalf("stale refresh wrote run=%+v err=%v", unchanged, err)
	}

	freshWork, err := s.GetLifecycleWorkItem(context.Background(), start.OwnerID, start.ComputerID, oldWork.WorkItemID)
	if err != nil {
		t.Fatal(err)
	}
	freshControl, err := s.GetLifecycleUpdate(context.Background(), start.OwnerID, start.ComputerID, start.TrajectoryID, run.AgentID, turn.Controls[0].AgentID, turn.Controls[0].ProducerUpdateID)
	if err != nil {
		t.Fatal(err)
	}
	fresh := bindControlRequestForTest(t, s, start, run, []types.CoagentSourcePacket{freshControl})
	fresh.ActivationRefresh = &types.LifecycleControlActivationRefresh{Prompt: "new canonical objective prompt", LogicalActivationKey: "sha256:logical", FailedAttemptKey: "sha256:attempt-new", BuildCommit: "build-a", Versions: []types.LifecycleControlActivationVersion{{UpdateID: freshControl.UpdateID, TargetWorkItemID: freshWork.WorkItemID, ControlLifecycleVersion: freshControl.LifecycleVersion, WorkLifecycleVersion: freshWork.LifecycleVersion}}, WorkItemIDs: []string{freshWork.WorkItemID}}
	fresh.CommandDigest, _ = ComputeBindLifecycleControlDeliveryDigest(fresh)
	if _, err := s.BindLifecycleControlDelivery(context.Background(), fresh); err != nil {
		t.Fatal(err)
	}
	stored, err := s.GetLifecycleRun(context.Background(), start.OwnerID, start.ComputerID, run.RunID)
	if err != nil || stored.Prompt != fresh.ActivationRefresh.Prompt || metadataStringValueStore(stored.Metadata, "lifecycle_failed_attempt_key") != fresh.ActivationRefresh.FailedAttemptKey {
		t.Fatalf("fresh atomic refresh=%+v err=%v", stored, err)
	}
}

func TestBindLifecycleControlDeliveryExactPersistentSuperRemainsNonLifecycle(t *testing.T) {
	s, start, caller, _ := setupLifecycleTextureTargetFixture(t)
	now := time.Now().UTC()
	superID := "super:" + start.OwnerID
	if err := s.UpsertAgent(context.Background(), types.AgentRecord{AgentID: superID, OwnerID: start.OwnerID, ComputerID: start.ComputerID, Profile: "super", Role: "super", ChannelID: superID, CreatedAt: now, UpdatedAt: now}); err != nil {
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
	run := types.RunRecord{RunID: "persistent-super-run", OwnerID: start.OwnerID, ComputerID: start.ComputerID, AgentID: superID, AgentProfile: "super", AgentRole: "super", ChannelID: start.InitialDocument.DocID, State: types.RunRunning, Metadata: map[string]any{"assignment_trajectory_id": start.TrajectoryID, "work_item_ids": []string{workID}, "lifecycle_work_item_id": workID}, CreatedAt: now, UpdatedAt: now}
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
	injectionEnvelope, _ := json.Marshal(map[string]any{
		"schema": "choir.lifecycle_injection.v1", "packet_type": "coagent_update",
		"owner_id": start.OwnerID, "computer_id": start.ComputerID, "trajectory_id": start.TrajectoryID,
		"target_agent_id": superID, "target_run_id": run.RunID,
		"updates": []map[string]any{{"update_id": turn.Controls[0].UpdateID}},
	})
	injectionMessage, _ := json.Marshal(map[string]any{"role": "user", "content": "Choir authenticated coagent update packet.\n\n" + string(injectionEnvelope)})
	reportPacket := types.CoagentSourcePacketPayload{SchemaVersion: types.CoagentSourcePacketSchemaV1, Kind: "execution_result", Summary: "super progress", Notes: []string{"evidence:progress"}}
	reportPayloadDigest, _ := ComputeLifecycleUpdatePayloadDigest(reportPacket, "super progress")
	report := types.QueueLifecycleUpdateRequest{OwnerID: start.OwnerID, ComputerID: start.ComputerID, CommandID: "queue-super-progress", TrajectoryID: start.TrajectoryID,
		TargetAgentID: caller.AgentID, ProducerAgentID: superID, ControlBindingID: turn.Controls[0].UpdateID, TargetWorkItemID: start.InitialWork.WorkItemID,
		ConsumedDeliveryUpdateIDs: []string{turn.Controls[0].UpdateID},
		ProducerUpdateID:          "super-progress-occurrence", UpdateID: "super-progress-result", ChannelID: start.InitialDocument.DocID, Role: "super", SourceRunID: run.RunID,
		Packet: reportPacket, Content: "super progress", WorkDisposition: types.WorkItemCompleted, WorkItemID: workID, PayloadDigest: reportPayloadDigest}
	report.CommandDigest, _ = ComputeQueuePersistentSuperReportDigest(report)
	if _, err := s.QueueLifecycleUpdate(context.Background(), report); !errors.Is(err, ErrLifecycleInvalidTransition) {
		t.Fatalf("persistent Super report accepted caller IDs without durable runtime injection: %v", err)
	}
	if _, err := s.AppendRunMemoryEntry(context.Background(), types.RunMemoryEntry{
		RunID: run.RunID, OwnerID: start.OwnerID, AgentID: superID, Kind: types.RunMemoryEntryMessage,
		Role: types.RunMemoryRoleRuntimeInjection, Message: injectionMessage, CreatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	queued, err := s.QueueLifecycleUpdate(context.Background(), report)
	if err != nil || queued.Update == nil || queued.Update.Direction != types.LifecyclePacketDirectionProducerReport || queued.Update.TargetWorkItemID != start.InitialWork.WorkItemID || queued.Update.ControlBindingID != turn.Controls[0].UpdateID {
		t.Fatalf("persistent Super report = %+v err=%v", queued, err)
	}
	history, historyErr := s.ListLifecycleControlsDeliveredToRun(context.Background(), start.OwnerID, start.ComputerID, start.TrajectoryID, superID, run.RunID, 10)
	if historyErr != nil || len(history) != 1 || history[0].Disposition != types.UpdateIncorporated {
		t.Fatalf("incorporated exact-run delivery history = %+v err=%v", history, historyErr)
	}
	replayed, err := s.QueueLifecycleUpdate(context.Background(), report)
	if err != nil || !replayed.Replay || replayed.Update == nil || replayed.Update.UpdateID != queued.Update.UpdateID {
		t.Fatalf("super report replay = %+v err=%v", replayed, err)
	}
	conflict := report
	conflict.Content = "conflicting progress"
	conflict.PayloadDigest, _ = ComputeLifecycleUpdatePayloadDigest(conflict.Packet, conflict.Content)
	conflict.CommandDigest, _ = ComputeQueuePersistentSuperReportDigest(conflict)
	if _, err := s.QueueLifecycleUpdate(context.Background(), conflict); !errors.Is(err, ErrLifecycleCommandConflict) {
		t.Fatalf("super report conflict = %v", err)
	}
	consume := textureTurnBaseRequest(t, s, start, caller, types.TextureTurnWait)
	consume.CommandID = "consume-super-progress"
	consume.Inbound = []types.TextureTurnInboundDisposition{{TargetAgentID: caller.AgentID, ProducerAgentID: superID,
		ProducerUpdateID: report.ProducerUpdateID, UpdateID: report.UpdateID, Disposition: types.UpdateIncorporated,
		ProducerWorkItemID: workID, WorkDisposition: types.WorkItemCompleted, WorkResultRef: "super-result:complete", Reason: "progress incorporated"}}
	setTextureTurnDigest(t, &consume, TextureSourceGraphWriteSet{})
	if _, err := s.ApplyTextureTurn(context.Background(), consume); err != nil {
		t.Fatalf("consume target-correlated Super report: %v", err)
	}
	producerWork, _ := s.GetLifecycleWorkItem(context.Background(), start.OwnerID, start.ComputerID, workID)
	targetWork, _ := s.GetLifecycleWorkItem(context.Background(), start.OwnerID, start.ComputerID, start.InitialWork.WorkItemID)
	if producerWork.Status != types.WorkItemCompleted || targetWork.Status != types.WorkItemOpen {
		t.Fatalf("report consumption settled wrong work: producer=%+v target=%+v", producerWork, targetWork)
	}
	if legacy, err := s.ListCoagentMailboxBacklog(context.Background(), start.OwnerID, caller.AgentID, 10); err != nil || len(legacy) != 0 {
		t.Fatalf("super report entered legacy mailbox: %+v err=%v", legacy, err)
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

func TestQueueLifecycleUpdateDigestPreservesHistoricalShape(t *testing.T) {
	base := types.QueueLifecycleUpdateRequest{CommandID: "command", TrajectoryID: "trajectory", TargetAgentID: "texture", ProducerAgentID: "researcher", UpdateID: "update", ProducerUpdateID: "producer-update", PayloadDigest: strings.Repeat("a", 64), WorkItemID: "producer-work", SourceRunID: "run", ChannelID: "channel", Role: "researcher", WorkDisposition: types.WorkItemOpen}
	historical, err := ComputeQueueLifecycleUpdateDigest(base)
	if err != nil {
		t.Fatal(err)
	}
	extended := base
	extended.ControlBindingID, extended.TargetWorkItemID = "control", "texture-work"
	compatible, err := ComputeQueueLifecycleUpdateDigest(extended)
	if err != nil || compatible != historical {
		t.Fatalf("historical digest changed: base=%s extended=%s err=%v", historical, compatible, err)
	}
	reportDigest, err := ComputeQueuePersistentSuperReportDigest(extended)
	if err != nil || reportDigest == historical {
		t.Fatalf("persistent Super report lacks distinct digest domain: historical=%s report=%s err=%v", historical, reportDigest, err)
	}
}
