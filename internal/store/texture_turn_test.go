package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/texturedoc"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func textureTurnControlPacket(summary string) types.CoagentSourcePacketPayload {
	return types.CoagentSourcePacketPayload{
		SchemaVersion: types.CoagentSourcePacketSchemaV1, Kind: "execution_request", Summary: summary,
		Actions: []types.CoagentPacketAction{{
			ActionID: "action-" + summary, Type: "research", Objective: summary,
			Safety: types.CoagentPacketActionSafety{MutationClass: "none", Network: "none", FileMutation: "none"},
		}},
	}
}

func textureTurnBaseRequest(t *testing.T, s *Store, start types.StartLifecycleRequest, caller types.RunRecord, outcome types.TextureTurnOutcome) types.ApplyTextureTurnRequest {
	t.Helper()
	ctx := context.Background()
	trajectory, err := s.GetLifecycleTrajectory(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID)
	if err != nil {
		t.Fatal(err)
	}
	agent, err := s.GetAgentByScope(ctx, start.OwnerID, start.ComputerID, start.Agent.AgentID)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := s.GetLifecycleDocument(ctx, start.OwnerID, start.ComputerID, start.InitialDocument.DocID)
	if err != nil {
		t.Fatal(err)
	}
	return types.ApplyTextureTurnRequest{
		OwnerID: start.OwnerID, ComputerID: start.ComputerID, CommandID: "texture-turn-command",
		DocumentID: start.InitialDocument.DocID, TrajectoryID: start.TrajectoryID,
		CallerAgentID: caller.AgentID, CallerRunID: caller.RunID,
		ExpectedLifecycleVersion: trajectory.LifecycleVersion, ExpectedCallerLifecycleVersion: agent.LifecycleVersion,
		ExpectedHeadRevisionID: doc.CurrentRevisionID, CallerWorkItemID: start.InitialWork.WorkItemID,
		CallerWorkDisposition: types.WorkItemOpen, Outcome: outcome, Reason: "explicit Texture outcome",
	}
}

func textureTurnQueueResearcherReport(t *testing.T, s *Store, start types.StartLifecycleRequest, work types.WorkItemRecord, id string) types.QueueLifecycleUpdateRequest {
	t.Helper()
	packet := types.CoagentSourcePacketPayload{SchemaVersion: types.CoagentSourcePacketSchemaV1, Kind: "evidence_update", Summary: id}
	content := "report " + id
	payloadDigest, err := ComputeLifecycleUpdatePayloadDigest(packet, content)
	if err != nil {
		t.Fatal(err)
	}
	req := types.QueueLifecycleUpdateRequest{
		OwnerID: start.OwnerID, ComputerID: start.ComputerID, CommandID: "queue-" + id,
		TrajectoryID: start.TrajectoryID, TargetAgentID: start.Agent.AgentID,
		ProducerAgentID: work.AssignedAgentID, ProducerUpdateID: "producer-" + id, UpdateID: "update-" + id,
		ChannelID: start.InitialDocument.DocID, Role: "researcher", SourceRunID: "run-researcher-target",
		Packet: packet, Content: content, PayloadDigest: payloadDigest,
		WorkItemID: work.WorkItemID, WorkDisposition: types.WorkItemOpen,
	}
	req.CommandDigest, err = ComputeQueueLifecycleUpdateDigest(req)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.QueueLifecycleUpdate(context.Background(), req); err != nil {
		t.Fatalf("queue report: %v", err)
	}
	return req
}

func textureTurnControl(t *testing.T, id, targetAgentID, targetWorkID string) types.TextureTurnControl {
	t.Helper()
	packet := textureTurnControlPacket(id)
	content := "control " + id
	digest, err := ComputeLifecycleUpdatePayloadDigest(packet, content)
	if err != nil {
		t.Fatal(err)
	}
	return types.TextureTurnControl{ControlID: id, TargetAgentID: targetAgentID, TargetWorkItemID: targetWorkID, Packet: packet, Content: content, PayloadDigest: digest}
}

func setTextureTurnDigest(t *testing.T, req *types.ApplyTextureTurnRequest, graph TextureSourceGraphWriteSet) {
	t.Helper()
	digest, err := ComputeApplyTextureTurnWithSourceGraphDigest(*req, graph)
	if err != nil {
		t.Fatal(err)
	}
	req.CommandDigest = digest
}

func TestApplyTextureTurnOrderedControlsAtomicReplayAndLegacyIsolation(t *testing.T) {
	s, start, caller, researcherWork := setupLifecycleTextureTargetFixture(t)
	ctx := context.Background()
	report := textureTurnQueueResearcherReport(t, s, start, researcherWork, "ordered")
	req := textureTurnBaseRequest(t, s, start, caller, types.TextureTurnNoSemanticChange)
	req.CommandID = "texture-turn-ordered"
	req.Inbound = []types.TextureTurnInboundDisposition{{
		TargetAgentID: start.Agent.AgentID, ProducerAgentID: report.ProducerAgentID, ProducerUpdateID: report.ProducerUpdateID,
		UpdateID: report.UpdateID, Disposition: types.UpdateIncorporated, ProducerWorkItemID: researcherWork.WorkItemID,
		WorkDisposition: types.WorkItemOpen,
	}}
	req.Controls = []types.TextureTurnControl{
		textureTurnControl(t, "control-first", researcherWork.AssignedAgentID, researcherWork.WorkItemID),
		textureTurnControl(t, "control-second", researcherWork.AssignedAgentID, researcherWork.WorkItemID),
	}
	req.SubjectRefs = map[string]string{"latest_evidence": "evidence://ordered"}
	setTextureTurnDigest(t, &req, TextureSourceGraphWriteSet{})
	result, err := s.ApplyTextureTurn(ctx, req)
	if err != nil {
		t.Fatalf("apply Texture turn: %v", err)
	}
	if result.TextureTurn == nil || result.TextureTurn.Outcome != types.TextureTurnNoSemanticChange || result.Revision != nil ||
		len(result.Controls) != 2 || result.Controls[0].UpdateID != "control-first" || result.Controls[1].UpdateID != "control-second" {
		t.Fatalf("turn result = %+v", result)
	}
	storedReport, err := s.GetLifecycleUpdate(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID, start.Agent.AgentID, report.ProducerAgentID, report.ProducerUpdateID)
	if err != nil || storedReport.Disposition != types.UpdateIncorporated || storedReport.DispositionRef != start.InitialRevision.RevisionID {
		t.Fatalf("explicit no-change inbound disposition = %+v, %v", storedReport, err)
	}
	pending, err := s.ListPendingLifecycleUpdates(ctx, start.OwnerID, start.ComputerID, researcherWork.AssignedAgentID, 10)
	if err != nil || len(pending) != 2 || pending[0].UpdateID != "control-first" || pending[1].UpdateID != "control-second" {
		t.Fatalf("ordered pending controls = %+v, %v", pending, err)
	}
	for _, packet := range pending {
		producer, target, resolveErr := ResolveLifecyclePacketWorkBindings(packet)
		if resolveErr != nil || producer != "" || target != researcherWork.WorkItemID || packet.Direction != types.LifecyclePacketDirectionControl {
			t.Fatalf("control work binding = %+v producer=%q target=%q err=%v", packet, producer, target, resolveErr)
		}
	}
	legacy, err := s.ListPendingWorkerUpdates(ctx, start.OwnerID, researcherWork.AssignedAgentID, 10)
	if err != nil || len(legacy) != 0 {
		t.Fatalf("legacy mailbox exposed controls: %+v, %v", legacy, err)
	}
	snapshot, err := s.GetLifecycleSnapshot(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID)
	if err != nil || snapshot.HeadRevision.RevisionID != start.InitialRevision.RevisionID || snapshot.Trajectory.SubjectRefs["latest_evidence"] != "evidence://ordered" {
		t.Fatalf("no-change snapshot = %+v, %v", snapshot, err)
	}

	peer := &Store{ogStore: s.ogStore, ogReadStore: s.ogReadStore}
	replay, err := peer.ApplyTextureTurn(ctx, req)
	if err != nil || !replay.Replay || replay.TextureTurn == nil || len(replay.Controls) != 2 || replay.Controls[0].UpdateID != "control-first" {
		t.Fatalf("restart replay = %+v, %v", replay, err)
	}
	reordered := req
	reordered.Controls = []types.TextureTurnControl{req.Controls[1], req.Controls[0]}
	setTextureTurnDigest(t, &reordered, TextureSourceGraphWriteSet{})
	if _, err := peer.ApplyTextureTurn(ctx, reordered); !errors.Is(err, ErrLifecycleCommandConflict) {
		t.Fatalf("reordered replay error = %v", err)
	}
}

func TestApplyTextureTurnResearcherOpenerAgentWorkAndFirstControlAreAtomic(t *testing.T) {
	s, start, caller, _ := setupLifecycleTextureTargetFixture(t)
	ctx := context.Background()
	targetAgentID, targetWorkID := "researcher:atomic-open", "work-researcher-atomic-open"
	req := textureTurnBaseRequest(t, s, start, caller, types.TextureTurnWait)
	req.CommandID = "texture-turn-researcher-opener"
	control := textureTurnControl(t, "control-researcher-first", targetAgentID, targetWorkID)
	control.Packet = types.CoagentSourcePacketPayload{SchemaVersion: types.CoagentSourcePacketSchemaV1, Kind: "question", Summary: "research exact gap", Questions: []string{"What exact evidence resolves the gap?"}}
	control.Content = "research exact gap"
	control.PayloadDigest, _ = ComputeLifecycleUpdatePayloadDigest(control.Packet, control.Content)
	control.OpenAgent = &types.AgentRecord{AgentID: targetAgentID, Profile: "researcher", Role: "researcher", ChannelID: start.InitialDocument.DocID}
	control.OpenWork = &types.WorkItemRecord{WorkItemID: targetWorkID, Objective: "research exact gap", AuthorityProfile: "researcher", AssignedAgentID: targetAgentID}
	req.Controls = []types.TextureTurnControl{control}
	setTextureTurnDigest(t, &req, TextureSourceGraphWriteSet{})

	result, err := s.ApplyTextureTurn(ctx, req)
	if err != nil || len(result.Controls) != 1 || len(result.TargetWorkItems) != 1 {
		t.Fatalf("atomic Researcher opener = %+v, %v", result, err)
	}
	agent, err := s.GetAgentByScope(ctx, start.OwnerID, start.ComputerID, targetAgentID)
	if err != nil || agent.Profile != "researcher" || agent.Role != "researcher" || agent.ChannelID != start.InitialDocument.DocID || agent.LifecycleVersion != 1 {
		t.Fatalf("atomic Researcher agent = %+v, %v", agent, err)
	}
	work, err := s.GetLifecycleWorkItem(ctx, start.OwnerID, start.ComputerID, targetWorkID)
	if err != nil || work.Status != types.WorkItemOpen || work.AssignedAgentID != targetAgentID || work.CreatedByRunID != caller.RunID {
		t.Fatalf("atomic Researcher work = %+v, %v", work, err)
	}
	pending, err := s.ListPendingLifecycleUpdates(ctx, start.OwnerID, start.ComputerID, targetAgentID, 10)
	if err != nil || len(pending) != 1 || pending[0].UpdateID != control.ControlID || pending[0].TargetWorkItemID != targetWorkID || pending[0].Direction != types.LifecyclePacketDirectionControl {
		t.Fatalf("atomic Researcher first control = %+v, %v", pending, err)
	}
	legacy, err := s.ListPendingWorkerUpdates(ctx, start.OwnerID, targetAgentID, 10)
	if err != nil || len(legacy) != 0 {
		t.Fatalf("Researcher opener leaked to legacy mailbox: %+v, %v", legacy, err)
	}
	replay, err := s.ApplyTextureTurn(ctx, req)
	if err != nil || !replay.Replay || len(replay.Controls) != 1 {
		t.Fatalf("Researcher opener replay = %+v, %v", replay, err)
	}
}

func TestApplyTextureTurnResearcherOpenerRefusesMismatchedAgentWithoutPartialMutation(t *testing.T) {
	s, start, caller, _ := setupLifecycleTextureTargetFixture(t)
	ctx := context.Background()
	req := textureTurnBaseRequest(t, s, start, caller, types.TextureTurnWait)
	req.CommandID = "texture-turn-researcher-opener-refusal"
	control := textureTurnControl(t, "control-researcher-refused", "researcher:expected", "work-researcher-refused")
	control.OpenAgent = &types.AgentRecord{AgentID: "researcher:forged", Profile: "researcher", Role: "researcher", ChannelID: start.InitialDocument.DocID}
	control.OpenWork = &types.WorkItemRecord{WorkItemID: control.TargetWorkItemID, Objective: "must refuse", AuthorityProfile: "researcher", AssignedAgentID: control.TargetAgentID}
	req.Controls = []types.TextureTurnControl{control}
	setTextureTurnDigest(t, &req, TextureSourceGraphWriteSet{})
	before, _ := s.GetLifecycleSnapshot(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID)
	if _, err := s.ApplyTextureTurn(ctx, req); err == nil {
		t.Fatal("mismatched Researcher opener accepted")
	}
	after, _ := s.GetLifecycleSnapshot(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID)
	if after.SnapshotCursor != before.SnapshotCursor {
		t.Fatalf("refused Researcher opener partially mutated trajectory: %d -> %d", before.SnapshotCursor, after.SnapshotCursor)
	}
	if _, err := s.GetAgentByScope(ctx, start.OwnerID, start.ComputerID, "researcher:forged"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("refused opener created agent: %v", err)
	}
	if _, err := s.GetLifecycleWorkItem(ctx, start.OwnerID, start.ComputerID, control.TargetWorkItemID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("refused opener created work: %v", err)
	}
}

func TestApplyTextureTurnPersistentSuperOpenerIsAtomic(t *testing.T) {
	s, start, caller, _ := setupLifecycleTextureTargetFixture(t)
	ctx := context.Background()
	now := time.Now().UTC()
	superID := "super:" + start.OwnerID
	if err := s.UpsertAgent(ctx, types.AgentRecord{AgentID: superID, OwnerID: start.OwnerID, ComputerID: start.ComputerID,
		Profile: "super", Role: "super", ChannelID: superID, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	req := textureTurnBaseRequest(t, s, start, caller, types.TextureTurnWait)
	req.CommandID, req.Reason = "texture-turn-super-opener", "Super execution is required"
	control := textureTurnControl(t, "control-super-first", superID, "work-super-target")
	control.OpenWork = &types.WorkItemRecord{WorkItemID: "work-super-target", Objective: "coordinate exact implementation",
		AuthorityProfile: "super", AssignedAgentID: superID, StepBudget: 8}
	req.Controls = []types.TextureTurnControl{control}
	setTextureTurnDigest(t, &req, TextureSourceGraphWriteSet{})
	result, err := s.ApplyTextureTurn(ctx, req)
	if err != nil {
		t.Fatalf("apply Super opener: %v", err)
	}
	if result.TextureTurn == nil || result.TextureTurn.Outcome != types.TextureTurnWait || len(result.TargetWorkItems) != 1 ||
		result.TargetWorkItems[0].WorkItemID != control.TargetWorkItemID || result.TargetWorkItems[0].CreatedByRunID != caller.RunID {
		t.Fatalf("Super opener result = %+v", result)
	}
	work, err := s.GetLifecycleWorkItem(ctx, start.OwnerID, start.ComputerID, control.TargetWorkItemID)
	if err != nil || work.Status != types.WorkItemOpen || work.AssignedAgentID != superID || work.Details["requested_by_agent_id"] != caller.AgentID {
		t.Fatalf("atomic Super work = %+v, %v", work, err)
	}
	pending, err := s.ListPendingLifecycleUpdates(ctx, start.OwnerID, start.ComputerID, superID, 10)
	if err != nil || len(pending) != 1 || pending[0].Packet.Kind != "execution_request" || pending[0].TargetWorkItemID != work.WorkItemID {
		t.Fatalf("atomic Super first request = %+v, %v", pending, err)
	}
	legacy, err := s.ListPendingWorkerUpdates(ctx, start.OwnerID, superID, 10)
	if err != nil || len(legacy) != 0 {
		t.Fatalf("Super control leaked to legacy mailbox: %+v, %v", legacy, err)
	}

	replay, err := s.ApplyTextureTurn(ctx, req)
	if err != nil || !replay.Replay || len(replay.Controls) != 1 || len(replay.TargetWorkItems) != 1 {
		t.Fatalf("equal Super opener replay = %+v, %v", replay, err)
	}
	changedPayload := req
	changedPayload.Controls = append([]types.TextureTurnControl(nil), req.Controls...)
	changedPayload.Controls[0].Packet.Summary = "changed execution request"
	changedPayload.Controls[0].PayloadDigest, _ = ComputeLifecycleUpdatePayloadDigest(changedPayload.Controls[0].Packet, changedPayload.Controls[0].Content)
	setTextureTurnDigest(t, &changedPayload, TextureSourceGraphWriteSet{})
	if _, err := s.ApplyTextureTurn(ctx, changedPayload); !errors.Is(err, ErrLifecycleCommandConflict) {
		t.Fatalf("changed Super opener payload replay error = %v", err)
	}

	reuse := textureTurnBaseRequest(t, s, start, caller, types.TextureTurnWait)
	reuse.CommandID, reuse.Reason = "texture-turn-super-reuse", "continue exact Super work"
	reuseControl := textureTurnControl(t, "control-super-second", superID, work.WorkItemID)
	reuseControl.OpenWork = control.OpenWork
	reuse.Controls = []types.TextureTurnControl{reuseControl}
	setTextureTurnDigest(t, &reuse, TextureSourceGraphWriteSet{})
	reused, err := s.ApplyTextureTurn(ctx, reuse)
	if err != nil || len(reused.TargetWorkItems) != 1 || reused.TargetWorkItems[0].WorkItemID != work.WorkItemID {
		t.Fatalf("exact Super work reuse = %+v, %v", reused, err)
	}
	snapshot, err := s.GetLifecycleSnapshot(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID)
	if err != nil {
		t.Fatal(err)
	}
	workCount := 0
	for _, item := range snapshot.WorkItems {
		if item.WorkItemID == work.WorkItemID {
			workCount++
		}
	}
	pending, err = s.ListPendingLifecycleUpdates(ctx, start.OwnerID, start.ComputerID, superID, 10)
	if err != nil || workCount != 1 || len(pending) != 2 || pending[1].UpdateID != "control-super-second" {
		t.Fatalf("Super reuse duplicated work or lost ordered control: work_count=%d pending=%+v err=%v", workCount, pending, err)
	}

	conflict := textureTurnBaseRequest(t, s, start, caller, types.TextureTurnWait)
	conflict.CommandID, conflict.Reason = "texture-turn-super-reuse-conflict", "conflicting reuse"
	conflictingControl := textureTurnControl(t, "control-super-conflict", superID, work.WorkItemID)
	conflictingWork := *control.OpenWork
	conflictingWork.Objective = "different authority under reused identity"
	conflictingControl.OpenWork = &conflictingWork
	conflict.Controls = []types.TextureTurnControl{conflictingControl}
	setTextureTurnDigest(t, &conflict, TextureSourceGraphWriteSet{})
	if _, err := s.ApplyTextureTurn(ctx, conflict); !errors.Is(err, ErrLifecycleCommandConflict) {
		t.Fatalf("changed Super work reuse error = %v, want conflict", err)
	}
	pending, err = s.ListPendingLifecycleUpdates(ctx, start.OwnerID, start.ComputerID, superID, 10)
	if err != nil || len(pending) != 2 {
		t.Fatalf("conflicting Super reuse queued control: %+v, %v", pending, err)
	}
	malformed := textureTurnBaseRequest(t, s, start, caller, types.TextureTurnWait)
	malformed.CommandID, malformed.Reason = "texture-turn-super-malformed-continuation", "must refuse malformed continuation"
	bad := textureTurnControl(t, "control-super-malformed", superID, work.WorkItemID)
	bad.Packet.Kind, bad.Packet.Actions = "evidence_update", nil
	bad.PayloadDigest, _ = ComputeLifecycleUpdatePayloadDigest(bad.Packet, bad.Content)
	malformed.Controls = []types.TextureTurnControl{bad}
	setTextureTurnDigest(t, &malformed, TextureSourceGraphWriteSet{})
	if _, err := s.ApplyTextureTurn(ctx, malformed); err == nil || !strings.Contains(err.Error(), "persistent-Super control requires execution_request actions") {
		t.Fatalf("malformed Super continuation error = %v", err)
	}
	afterMalformed, err := s.ListPendingLifecycleUpdates(ctx, start.OwnerID, start.ComputerID, superID, 10)
	if err != nil || len(afterMalformed) != 2 {
		t.Fatalf("malformed continuation poisoned backlog: %+v, %v", afterMalformed, err)
	}
}

func TestApplyTextureTurnEveryOutcomeRequiresCompleteOrderedSameHeadOwnerInstructions(t *testing.T) {
	for _, outcome := range []types.TextureTurnOutcome{types.TextureTurnRevision, types.TextureTurnNoSemanticChange, types.TextureTurnWait, types.TextureTurnBlock} {
		t.Run(string(outcome), func(t *testing.T) {
			s, start, caller, _ := setupLifecycleTextureTargetFixture(t)
			ctx := context.Background()
			req := textureTurnBaseRequest(t, s, start, caller, outcome)
			req.CommandID = "texture-turn-owner-completeness-" + string(outcome)
			if outcome == types.TextureTurnRevision {
				req.Revision = types.Revision{RevisionID: "revision-owner-completeness", AuthorKind: types.AuthorAppAgent, AuthorLabel: "Texture"}
			}

			// The turn was shaped before a same-head owner occurrence arrived. Refresh
			// only the ordinary lifecycle CAS version to prove the independent exact-set
			// precondition refuses the late occurrence for every semantic outcome.
			queued := ownerInstructionRequest(t, s, start, "late-"+string(outcome), "late same-head owner instruction")
			if _, err := s.QueueLifecycleOwnerInstruction(ctx, queued); err != nil {
				t.Fatal(err)
			}
			trajectory, err := s.GetLifecycleTrajectory(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID)
			if err != nil {
				t.Fatal(err)
			}
			req.ExpectedLifecycleVersion = trajectory.LifecycleVersion
			setTextureTurnDigest(t, &req, TextureSourceGraphWriteSet{})
			before, _ := s.GetLifecycleSnapshot(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID)
			if _, err := s.ApplyTextureTurn(ctx, req); !errors.Is(err, ErrConcurrentStateChange) {
				t.Fatalf("incomplete %s owner set error = %v, want concurrent-state refusal", outcome, err)
			}
			after, _ := s.GetLifecycleSnapshot(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID)
			if after.SnapshotCursor != before.SnapshotCursor || after.HeadRevision.RevisionID != before.HeadRevision.RevisionID {
				t.Fatalf("incomplete %s owner set mutated atomically guarded state: before=%+v after=%+v", outcome, before, after)
			}

			if outcome != types.TextureTurnRevision {
				req.OwnerInstructions = []types.TextureTurnOwnerInstruction{{InstructionID: queued.InstructionID, RequestID: queued.RequestID}}
				setTextureTurnDigest(t, &req, TextureSourceGraphWriteSet{})
				if _, err := s.ApplyTextureTurn(ctx, req); err != nil {
					t.Fatalf("complete %s owner set: %v", outcome, err)
				}
			}
		})
	}
}

func TestApplyTextureTurnNonRevisionOutcomesDispositionInboundWithoutFakeRevision(t *testing.T) {
	for _, outcome := range []types.TextureTurnOutcome{types.TextureTurnNoSemanticChange, types.TextureTurnWait, types.TextureTurnBlock} {
		t.Run(string(outcome), func(t *testing.T) {
			s, start, caller, work := setupLifecycleTextureTargetFixture(t)
			report := textureTurnQueueResearcherReport(t, s, start, work, string(outcome))
			req := textureTurnBaseRequest(t, s, start, caller, outcome)
			req.CommandID = "texture-turn-" + string(outcome)
			req.Inbound = []types.TextureTurnInboundDisposition{{TargetAgentID: start.Agent.AgentID,
				ProducerAgentID: report.ProducerAgentID, ProducerUpdateID: report.ProducerUpdateID, UpdateID: report.UpdateID,
				Disposition: types.UpdateRejected, ProducerWorkItemID: work.WorkItemID, WorkDisposition: types.WorkItemOpen, Reason: "explicitly unusable now"}}
			setTextureTurnDigest(t, &req, TextureSourceGraphWriteSet{})
			result, err := s.ApplyTextureTurn(context.Background(), req)
			if err != nil || result.Revision != nil || result.Document.CurrentRevisionID != start.InitialRevision.RevisionID {
				t.Fatalf("non-revision result = %+v, %v", result, err)
			}
			stored, err := s.GetLifecycleUpdate(context.Background(), start.OwnerID, start.ComputerID, start.TrajectoryID,
				start.Agent.AgentID, report.ProducerAgentID, report.ProducerUpdateID)
			if err != nil || stored.Disposition != types.UpdateRejected || stored.DispositionRef != start.InitialRevision.RevisionID {
				t.Fatalf("non-revision explicit disposition = %+v, %v", stored, err)
			}
			if _, err := s.GetLifecycleRevision(context.Background(), start.OwnerID, start.ComputerID, req.CommandID); !errors.Is(err, ErrNotFound) {
				t.Fatalf("non-revision outcome manufactured revision: %v", err)
			}
		})
	}
}

func TestApplyTextureTurnRevisionUsesStructuredAuthorityAndExactSourceManifest(t *testing.T) {
	s, start, caller, _ := setupLifecycleTextureTargetFixture(t)
	ctx := context.Background()
	req := textureTurnBaseRequest(t, s, start, caller, types.TextureTurnRevision)
	req.CommandID, req.Reason = "texture-turn-revision", "publish grounded interim learning"
	entity := texturedoc.SourceEntity{
		SourceEntityID: "source-one", Target: texturedoc.SourceTarget{Kind: "web_url", URI: "https://example.com/exact"},
		Selectors:  []texturedoc.SourceSelector{{Kind: "text_quote", Data: map[string]any{"exact": "grounded"}}},
		Display:    texturedoc.SourceDisplay{Mode: "expanded_ref", Title: "Exact source"},
		Evidence:   texturedoc.SourceEvidence{State: "confirms", OpenSurface: "source"},
		Provenance: texturedoc.SourceEntityProvenance{CreatedBy: "Texture"}, ReaderSnapshot: map[string]any{"text": "grounded"},
	}
	doc := texturedoc.StructuredTextureDoc{Schema: texturedoc.SchemaV1, Doc: texturedoc.Node{Type: "doc", Attrs: map[string]any{"id": "doc-root"}, Content: []texturedoc.Node{{
		Type: "paragraph", Attrs: map[string]any{"id": "p-one"}, Content: []texturedoc.Node{{Type: "text", Text: "Grounded "},
			{Type: "source_ref", Attrs: map[string]any{"id": "ref-one", "source_entity_id": entity.SourceEntityID, "display_mode": "expanded_ref"}}},
	}}}}
	req.Revision = types.Revision{RevisionID: "revision-texture-turn-v1", ParentRevisionID: req.ExpectedHeadRevisionID,
		AuthorKind: types.AuthorAppAgent, AuthorLabel: "appagent", BodyDoc: mustJSONRaw(t, doc), SourceEntities: mustJSONRaw(t, []texturedoc.SourceEntity{entity})}
	preview := req.Revision
	preview.OwnerID, preview.ComputerID, preview.TrajectoryID, preview.DocID = start.OwnerID, start.ComputerID, start.TrajectoryID, start.InitialDocument.DocID
	var prepareErr error
	preview, _, _, prepareErr = prepareTextureRevisionV2(preview)
	if prepareErr != nil {
		t.Fatal(prepareErr)
	}
	entityRecord, err := textureTurnExpectedSourceEntity(preview, entity, caller.RunID)
	if err != nil {
		t.Fatal(err)
	}
	refs, err := textureTurnExpectedSourceRefs(preview, doc, map[string]TextureSourceEntityGraphRecord{entity.SourceEntityID: entityRecord}, caller.RunID)
	if err != nil {
		t.Fatal(err)
	}
	graph := TextureSourceGraphWriteSet{SourceEntities: []TextureSourceEntityGraphRecord{entityRecord}, SourceRefs: refs}
	setTextureTurnDigest(t, &req, graph)
	result, err := s.ApplyTextureTurnWithSourceGraph(ctx, req, graph)
	if err != nil {
		t.Fatalf("apply structured revision: %v", err)
	}
	projection, projectionErr := texturedoc.Project(doc, []texturedoc.SourceEntity{entity})
	if projectionErr != nil || result.Revision == nil || result.Revision.Content != projection.Text || result.Document.CurrentRevisionID != req.Revision.RevisionID {
		t.Fatalf("structured revision result = %+v projection=%+v err=%v", result, projection, projectionErr)
	}
	storedGraph, err := s.ListTextureSourceGraphForRevisionsByScope(ctx, start.OwnerID, start.ComputerID, start.InitialDocument.DocID, []string{req.Revision.RevisionID})
	if err != nil || len(storedGraph[req.Revision.RevisionID].SourceEntities) != 1 || len(storedGraph[req.Revision.RevisionID].SourceRefs) != 1 {
		t.Fatalf("stored exact source graph = %+v, %v", storedGraph, err)
	}
}

func TestApplyTextureTurnRejectsAuthorityManifestAndConditionalFailuresWithoutMutation(t *testing.T) {
	tests := map[string]func(*types.ApplyTextureTurnRequest, *TextureSourceGraphWriteSet){
		"stale head": func(req *types.ApplyTextureTurnRequest, _ *TextureSourceGraphWriteSet) {
			req.ExpectedHeadRevisionID = "revision-stale"
		},
		"conflicting parent": func(req *types.ApplyTextureTurnRequest, _ *TextureSourceGraphWriteSet) {
			req.Revision.ParentRevisionID = "revision-foreign"
		},
		"missing source graph": func(_ *types.ApplyTextureTurnRequest, graph *TextureSourceGraphWriteSet) {
			*graph = TextureSourceGraphWriteSet{}
		},
		"extra source ref": func(_ *types.ApplyTextureTurnRequest, graph *TextureSourceGraphWriteSet) {
			graph.SourceRefs = append(graph.SourceRefs, graph.SourceRefs[0])
		},
		"selector identity mismatch": func(_ *types.ApplyTextureTurnRequest, graph *TextureSourceGraphWriteSet) {
			var metadata map[string]any
			_ = json.Unmarshal(graph.SourceEntities[0].Metadata, &metadata)
			metadata["selectors"] = []any{map[string]any{"kind": "whole_resource"}}
			graph.SourceEntities[0].Metadata = mustJSONRaw(t, metadata)
			graph.SourceEntities[0].VersionID, graph.SourceEntities[0].ContentHash = "", ""
		},
		"open identity mismatch": func(_ *types.ApplyTextureTurnRequest, graph *TextureSourceGraphWriteSet) {
			var metadata map[string]any
			_ = json.Unmarshal(graph.SourceEntities[0].Metadata, &metadata)
			evidence := metadata["evidence"].(map[string]any)
			evidence["open_surface"] = "texture"
			graph.SourceEntities[0].Metadata = mustJSONRaw(t, metadata)
			graph.SourceEntities[0].VersionID, graph.SourceEntities[0].ContentHash = "", ""
		},
		"content hash mismatch": func(_ *types.ApplyTextureTurnRequest, graph *TextureSourceGraphWriteSet) {
			graph.SourceEntities[0].ContentHash = "sha256:wrong"
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			s, start, caller, _ := setupLifecycleTextureTargetFixture(t)
			ctx := context.Background()
			before, err := s.GetLifecycleSnapshot(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID)
			if err != nil {
				t.Fatal(err)
			}
			req := textureTurnBaseRequest(t, s, start, caller, types.TextureTurnRevision)
			req.CommandID, req.Reason = "texture-turn-refused", "candidate"
			entity := texturedoc.SourceEntity{SourceEntityID: "source-missing", Target: texturedoc.SourceTarget{Kind: "web_url", URI: "https://example.com/missing"},
				Display: texturedoc.SourceDisplay{Mode: "numbered_ref"}, Evidence: texturedoc.SourceEvidence{State: "confirms", OpenSurface: "source"},
				Provenance: texturedoc.SourceEntityProvenance{CreatedBy: "Texture"}}
			doc := texturedoc.StructuredTextureDoc{Schema: texturedoc.SchemaV1, Doc: texturedoc.Node{Type: "doc", Attrs: map[string]any{"id": "root"}, Content: []texturedoc.Node{{Type: "paragraph", Attrs: map[string]any{"id": "p"}, Content: []texturedoc.Node{{Type: "source_ref", Attrs: map[string]any{"id": "ref", "source_entity_id": entity.SourceEntityID}}}}}}}
			req.Revision = types.Revision{RevisionID: "revision-refused", ParentRevisionID: req.ExpectedHeadRevisionID,
				AuthorKind: types.AuthorAppAgent, AuthorLabel: "Texture", BodyDoc: mustJSONRaw(t, doc), SourceEntities: mustJSONRaw(t, []texturedoc.SourceEntity{entity})}
			preview := req.Revision
			preview.OwnerID, preview.ComputerID, preview.TrajectoryID, preview.DocID = start.OwnerID, start.ComputerID, start.TrajectoryID, start.InitialDocument.DocID
			var prepareErr error
			preview, _, _, prepareErr = prepareTextureRevisionV2(preview)
			if prepareErr != nil {
				t.Fatal(prepareErr)
			}
			entityRecord, _ := textureTurnExpectedSourceEntity(preview, entity, caller.RunID)
			refs, _ := textureTurnExpectedSourceRefs(preview, doc, map[string]TextureSourceEntityGraphRecord{entity.SourceEntityID: entityRecord}, caller.RunID)
			graph := TextureSourceGraphWriteSet{SourceEntities: []TextureSourceEntityGraphRecord{entityRecord}, SourceRefs: refs}
			mutate(&req, &graph)
			setTextureTurnDigest(t, &req, graph)
			if _, err := s.ApplyTextureTurnWithSourceGraph(ctx, req, graph); err == nil {
				t.Fatal("unsafe turn unexpectedly committed")
			}
			after, err := s.GetLifecycleSnapshot(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID)
			if err != nil {
				t.Fatal(err)
			}
			if after.Trajectory.LifecycleVersion != before.Trajectory.LifecycleVersion || after.HeadRevision.RevisionID != before.HeadRevision.RevisionID || len(after.Events) != len(before.Events) {
				t.Fatalf("refused turn mutated snapshot: before=%+v after=%+v", before, after)
			}
		})
	}
}

func TestApplyTextureTurnRefusesUnsafeTargetsBeforeAnyControlCommit(t *testing.T) {
	for _, targetID := range []string{"co-super:direct", "super:arbitrary", "researcher:cross-scope"} {
		t.Run(targetID, func(t *testing.T) {
			s, start, caller, work := setupLifecycleTextureTargetFixture(t)
			ctx := context.Background()
			now := time.Now().UTC()
			target := types.AgentRecord{AgentID: targetID, OwnerID: start.OwnerID, ComputerID: start.ComputerID, ChannelID: start.InitialDocument.DocID, CreatedAt: now, UpdatedAt: now}
			switch targetID {
			case "co-super:direct":
				target.Profile, target.Role = "co-super", "co-super"
			case "super:arbitrary":
				target.Profile, target.Role = "super", "super"
			case "researcher:cross-scope":
				target.Profile, target.Role, target.ComputerID, target.ComputerID = "researcher", "researcher", "computer-foreign", "computer-foreign"
			}
			if err := s.UpsertAgent(ctx, target); err != nil {
				t.Fatal(err)
			}
			before, _ := s.GetLifecycleSnapshot(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID)
			req := textureTurnBaseRequest(t, s, start, caller, types.TextureTurnBlock)
			req.CommandID = "texture-turn-refuse-" + targetID
			req.Controls = []types.TextureTurnControl{
				textureTurnControl(t, "control-valid-first", work.AssignedAgentID, work.WorkItemID),
				textureTurnControl(t, "control-invalid-second", targetID, work.WorkItemID),
			}
			setTextureTurnDigest(t, &req, TextureSourceGraphWriteSet{})
			if _, err := s.ApplyTextureTurn(ctx, req); err == nil {
				t.Fatal("unsafe target unexpectedly accepted")
			}
			pending, err := s.ListPendingLifecycleUpdates(ctx, start.OwnerID, start.ComputerID, work.AssignedAgentID, 10)
			if err != nil || len(pending) != 0 {
				t.Fatalf("earlier control committed before later refusal: %+v, %v", pending, err)
			}
			after, _ := s.GetLifecycleSnapshot(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID)
			if after.Trajectory.LifecycleVersion != before.Trajectory.LifecycleVersion || len(after.Events) != len(before.Events) {
				t.Fatalf("unsafe target refusal mutated lifecycle")
			}
		})
	}
}

func TestApplyTextureTurnDigestConflictsOnOrderPayloadTargetWorkHeadAndOutcome(t *testing.T) {
	base := types.ApplyTextureTurnRequest{CommandID: "digest-turn", DocumentID: "doc", TrajectoryID: "trajectory",
		CallerAgentID: "texture:doc", CallerRunID: "run", ExpectedLifecycleVersion: 4, ExpectedCallerLifecycleVersion: 3,
		ExpectedHeadRevisionID: "head", CallerWorkItemID: "texture-work", CallerWorkDisposition: types.WorkItemOpen,
		Outcome: types.TextureTurnWait, Reason: "wait"}
	base.Controls = []types.TextureTurnControl{textureTurnControl(t, "a", "researcher:a", "work-a"), textureTurnControl(t, "b", "researcher:b", "work-b")}
	original, err := ComputeApplyTextureTurnDigest(base)
	if err != nil {
		t.Fatal(err)
	}
	mutations := map[string]func(*types.ApplyTextureTurnRequest){
		"order": func(req *types.ApplyTextureTurnRequest) {
			req.Controls[0], req.Controls[1] = req.Controls[1], req.Controls[0]
		},
		"payload": func(req *types.ApplyTextureTurnRequest) { req.Controls[0].PayloadDigest = "different" },
		"target":  func(req *types.ApplyTextureTurnRequest) { req.Controls[0].TargetAgentID = "researcher:other" },
		"work":    func(req *types.ApplyTextureTurnRequest) { req.Controls[0].TargetWorkItemID = "work-other" },
		"head":    func(req *types.ApplyTextureTurnRequest) { req.ExpectedHeadRevisionID = "head-other" },
		"outcome": func(req *types.ApplyTextureTurnRequest) { req.Outcome, req.Reason = types.TextureTurnBlock, "block" },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			candidate := base
			candidate.Controls = append([]types.TextureTurnControl(nil), base.Controls...)
			mutate(&candidate)
			digest, err := ComputeApplyTextureTurnDigest(candidate)
			if err != nil {
				t.Fatal(err)
			}
			if digest == original {
				t.Fatal("mutation did not change digest")
			}
		})
	}
}

func mustJSONRaw(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestApplyTextureTurnCallerWorkConsequenceAtomicReplayAndRefusal(t *testing.T) {
	t.Run("completed revision settles exact caller work and replays", func(t *testing.T) {
		s, start, caller, _ := setupLifecycleTextureTargetFixture(t)
		doc := texturedoc.StructuredTextureDoc{Schema: texturedoc.SchemaV1, Doc: texturedoc.Node{Type: "doc", Attrs: map[string]any{"id": "doc-root"}, Content: []texturedoc.Node{{Type: "paragraph", Attrs: map[string]any{"id": "p-one"}, Content: []texturedoc.Node{{Type: "text", Text: "Completed result."}}}}}}
		req := textureTurnBaseRequest(t, s, start, caller, types.TextureTurnRevision)
		req.CommandID = "texture-turn-caller-complete"
		req.CallerWorkDisposition = types.WorkItemCompleted
		req.Revision = types.Revision{RevisionID: "revision-caller-complete", ParentRevisionID: req.ExpectedHeadRevisionID, AuthorKind: types.AuthorAppAgent, AuthorLabel: "appagent", BodyDoc: mustJSONRaw(t, doc), SourceEntities: json.RawMessage("[]")}
		setTextureTurnDigest(t, &req, TextureSourceGraphWriteSet{})
		result, err := s.ApplyTextureTurn(context.Background(), req)
		if err != nil {
			t.Fatal(err)
		}
		if result.WorkItem == nil || result.WorkItem.Status != types.WorkItemCompleted || result.WorkItem.ResultRef != req.Revision.RevisionID || result.TextureTurn.CallerWorkItemID != start.InitialWork.WorkItemID {
			t.Fatalf("caller work consequence = %+v turn=%+v", result.WorkItem, result.TextureTurn)
		}
		replay, err := s.ApplyTextureTurn(context.Background(), req)
		if err != nil || !replay.Replay || replay.WorkItem == nil || replay.WorkItem.Status != types.WorkItemCompleted {
			t.Fatalf("equal caller-work replay = %+v, %v", replay, err)
		}
		changed := req
		changed.CallerWorkDisposition = types.WorkItemOpen
		setTextureTurnDigest(t, &changed, TextureSourceGraphWriteSet{})
		if _, err := s.ApplyTextureTurn(context.Background(), changed); !errors.Is(err, ErrLifecycleCommandConflict) {
			t.Fatalf("changed caller-work replay err = %v", err)
		}
	})

	t.Run("wrong caller work refuses before mutation", func(t *testing.T) {
		s, start, caller, researcherWork := setupLifecycleTextureTargetFixture(t)
		req := textureTurnBaseRequest(t, s, start, caller, types.TextureTurnWait)
		req.CommandID = "texture-turn-wrong-caller-work"
		req.CallerWorkItemID = researcherWork.WorkItemID
		setTextureTurnDigest(t, &req, TextureSourceGraphWriteSet{})
		before, _ := s.GetLifecycleSnapshot(context.Background(), start.OwnerID, start.ComputerID, start.TrajectoryID)
		if _, err := s.ApplyTextureTurn(context.Background(), req); err == nil {
			t.Fatal("wrong caller work accepted")
		}
		after, _ := s.GetLifecycleSnapshot(context.Background(), start.OwnerID, start.ComputerID, start.TrajectoryID)
		if after.Trajectory.LifecycleVersion != before.Trajectory.LifecycleVersion || len(after.Events) != len(before.Events) || after.HeadRevision.RevisionID != before.HeadRevision.RevisionID {
			t.Fatalf("wrong caller-work refusal mutated lifecycle")
		}
	})
}

func TestApplyTextureTurnConsumesComplete101OwnerOccurrenceSet(t *testing.T) {
	s, start, caller, _ := setupLifecycleTextureTargetFixture(t)
	ctx := context.Background()
	bindings := make([]types.TextureTurnOwnerInstruction, 0, 101)
	for i := 0; i < 101; i++ {
		queued := ownerInstructionRequest(t, s, start, fmt.Sprintf("bulk-%03d", i), fmt.Sprintf("owner tell %03d", i))
		if _, err := s.QueueLifecycleOwnerInstruction(ctx, queued); err != nil {
			t.Fatal(err)
		}
		bindings = append(bindings, types.TextureTurnOwnerInstruction{InstructionID: queued.InstructionID, RequestID: queued.RequestID})
	}
	complete, err := s.ListPendingLifecycleOwnerInstructionsForHead(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID, caller.AgentID, start.InitialRevision.RevisionID)
	if err != nil || len(complete) != 101 {
		t.Fatalf("complete owner set=%d err=%v", len(complete), err)
	}
	req := textureTurnBaseRequest(t, s, start, caller, types.TextureTurnWait)
	req.CommandID, req.Reason, req.OwnerInstructions = "texture-turn-owner-101", "consume full unbounded occurrence set", bindings
	setTextureTurnDigest(t, &req, TextureSourceGraphWriteSet{})
	if _, err := s.ApplyTextureTurn(ctx, req); err != nil {
		t.Fatalf("apply 101 owner tells: %v", err)
	}
	remaining, err := s.ListPendingLifecycleOwnerInstructionsForHead(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID, caller.AgentID, start.InitialRevision.RevisionID)
	if err != nil || len(remaining) != 0 {
		t.Fatalf("remaining owner occurrences=%d err=%v", len(remaining), err)
	}
}
