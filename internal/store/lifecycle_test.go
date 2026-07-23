package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func lifecycleStartFixture() types.StartLifecycleRequest {
	req := types.StartLifecycleRequest{
		OwnerID: "owner-lifecycle", ComputerID: "computer-lifecycle",
		CommandID:    "command-start-1",
		TrajectoryID: "trajectory-lifecycle-1", Kind: types.TrajectoryKindTask,
		SubjectRefs:     map[string]string{"artifact": "texture://artifact/1", "doc_id": "document-lifecycle-1"},
		SettlementRule:  types.SettlementRule{Version: types.LifecycleReducerVersion, RequireNoOpenWorkItems: true, RequiredSubjectRefs: []string{"artifact"}},
		InitialWork:     types.WorkItemRecord{WorkItemID: "work-lifecycle-1", Objective: "produce artifact"},
		InitialDocument: types.Document{DocID: "document-lifecycle-1", Title: "Lifecycle artifact"},
		InitialRevision: types.Revision{RevisionID: "revision-lifecycle-v0", AuthorKind: types.AuthorAppAgent, AuthorLabel: "Choir", Content: "Initial artifact"},
		Agent:           types.AgentRecord{AgentID: "texture:document-lifecycle-1", Profile: "texture", Role: "texture", ChannelID: "document-lifecycle-1"},
	}
	digest, err := ComputeStartLifecycleRequestDigest(req)
	if err != nil {
		panic(err)
	}
	req.StartRequestDigest = digest
	return req
}

func TestStartLifecycleRefusesUnknownOrIncompleteSettlementRule(t *testing.T) {
	s := openTestStore(t)
	for name, mutate := range map[string]func(*types.StartLifecycleRequest){
		"missing version":     func(req *types.StartLifecycleRequest) { req.SettlementRule.Version = "" },
		"unknown version":     func(req *types.StartLifecycleRequest) { req.SettlementRule.Version = "durable-work/v2" },
		"missing predicate":   func(req *types.StartLifecycleRequest) { req.SettlementRule.RequireNoOpenWorkItems = false },
		"missing subject ref": func(req *types.StartLifecycleRequest) { req.SettlementRule.RequiredSubjectRefs = []string{"missing"} },
	} {
		t.Run(name, func(t *testing.T) {
			req := lifecycleStartFixture()
			mutate(&req)
			req.StartRequestDigest, _ = ComputeStartLifecycleRequestDigest(req)
			if _, err := s.StartLifecycle(context.Background(), req); !errors.Is(err, ErrLifecycleInvalidTransition) {
				t.Fatalf("start error=%v, want ErrLifecycleInvalidTransition", err)
			}
		})
	}
}

func TestStartLifecycleAtomicReplayAndScope(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	req := lifecycleStartFixture()

	result, err := s.StartLifecycle(ctx, req)
	if err != nil {
		t.Fatalf("start lifecycle: %v", err)
	}
	if result.Replay || result.Trajectory.ReducerSeq != 1 || len(result.Events) != 1 {
		t.Fatalf("unexpected start result: %+v", result)
	}
	snapshot, err := s.GetLifecycleSnapshot(ctx, req.OwnerID, req.ComputerID, req.TrajectoryID)
	if err != nil || snapshot.Activation.AgentID != req.Agent.AgentID ||
		snapshot.Activation.RunID != "" || snapshot.Activation.State != types.RunPassivated {
		t.Fatalf("atomic start snapshot activation = %+v, %v", snapshot.Activation, err)
	}
	work, err := s.GetLifecycleWorkItem(ctx, req.OwnerID, req.ComputerID, req.InitialWork.WorkItemID)
	if err != nil {
		t.Fatalf("get work item: %v", err)
	}
	if work.Status != types.WorkItemOpen || work.LastReducerSeq != 1 {
		t.Fatalf("unexpected work item: %+v", work)
	}
	agent, err := s.GetAgentByScope(ctx, req.OwnerID, req.ComputerID, req.Agent.AgentID)
	if err != nil {
		t.Fatalf("get agent by scope: %v", err)
	}
	if agent.LastReducerSeq != 1 || agent.ComputerID != req.ComputerID {
		t.Fatalf("unexpected agent: %+v", agent)
	}

	replay, err := s.StartLifecycle(ctx, req)
	if err != nil {
		t.Fatalf("replay lifecycle: %v", err)
	}
	if !replay.Replay || replay.Receipt.CommandID != req.CommandID || len(replay.Events) != 1 {
		t.Fatalf("unexpected replay: %+v", replay)
	}
	events, err := s.ListLifecycleEvents(ctx, req.OwnerID, req.ComputerID, req.TrajectoryID)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("event count = %d, want 1", len(events))
	}

	conflict := req
	conflict.StartRequestDigest = "sha256:different"
	if _, err := s.StartLifecycle(ctx, conflict); !errors.Is(err, ErrLifecycleCommandConflict) {
		t.Fatalf("conflicting replay error = %v, want ErrLifecycleCommandConflict", err)
	}
}

func TestCommitLifecycleArtifactHeadAtomicReplayAndCAS(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	start := lifecycleStartFixture()
	if _, err := s.StartLifecycle(ctx, start); err != nil {
		t.Fatalf("start lifecycle: %v", err)
	}
	commit := types.CommitLifecycleArtifactHeadRequest{
		OwnerID: start.OwnerID, ComputerID: start.ComputerID,
		CommandID: "commit-head-1", TrajectoryID: start.TrajectoryID,
		ExpectedLifecycleVersion: 1, ExpectedHeadRevisionID: start.InitialRevision.RevisionID,
		Revision: types.Revision{RevisionID: "revision-lifecycle-v1", AuthorKind: types.AuthorUser, AuthorLabel: "owner", Content: "Owner revision"},
	}
	commit.CommandDigest, _ = ComputeCommitLifecycleArtifactHeadDigest(commit)
	result, err := s.CommitLifecycleArtifactHead(ctx, commit)
	if err != nil {
		t.Fatalf("commit lifecycle head: %v", err)
	}
	if result.Replay || result.Trajectory.ReducerSeq != 2 || result.Trajectory.LifecycleVersion != 2 ||
		result.Revision == nil || result.Revision.RevisionID != commit.Revision.RevisionID ||
		len(result.Events) != 1 || result.Events[0].Kind != types.LifecycleArtifactHeadAdvanced {
		t.Fatalf("unexpected commit result: %+v", result)
	}
	snapshot, err := s.GetLifecycleSnapshot(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID)
	if err != nil {
		t.Fatalf("get lifecycle snapshot: %v", err)
	}
	if snapshot.Document.CurrentRevisionID != commit.Revision.RevisionID || snapshot.HeadRevision.ParentRevisionID != start.InitialRevision.RevisionID ||
		snapshot.HeadRevision.VersionNumber != 1 || snapshot.Trajectory.ReducerSeq != 2 {
		t.Fatalf("unexpected committed snapshot: %+v", snapshot)
	}
	replay, err := s.CommitLifecycleArtifactHead(ctx, commit)
	if err != nil || !replay.Replay || replay.Revision == nil || replay.Revision.RevisionID != commit.Revision.RevisionID {
		t.Fatalf("unexpected replay: %+v, %v", replay, err)
	}
	second := commit
	second.CommandID = "commit-head-2"
	second.ExpectedLifecycleVersion = 2
	second.ExpectedHeadRevisionID = commit.Revision.RevisionID
	second.Revision = types.Revision{RevisionID: "revision-lifecycle-v2", AuthorKind: types.AuthorUser, AuthorLabel: "owner", Content: "Second owner revision"}
	second.CommandDigest, _ = ComputeCommitLifecycleArtifactHeadDigest(second)
	if _, err := s.CommitLifecycleArtifactHead(ctx, second); err != nil {
		t.Fatalf("commit second lifecycle head: %v", err)
	}
	replay, err = s.CommitLifecycleArtifactHead(ctx, commit)
	if err != nil || replay.Revision == nil || replay.Revision.RevisionID != commit.Revision.RevisionID {
		t.Fatalf("replay after later head returned wrong revision: %+v, %v", replay, err)
	}
	startReplay, err := s.StartLifecycle(ctx, start)
	if err != nil || !startReplay.Replay || startReplay.Revision == nil ||
		startReplay.Revision.RevisionID != start.InitialRevision.RevisionID ||
		startReplay.Trajectory.ReducerSeq != 1 {
		t.Fatalf("start replay after head advance was not original result: %+v, %v", startReplay, err)
	}
	if err := s.PatchRevisionMetadata(ctx, start.OwnerID, commit.Revision.RevisionID, map[string]any{"mutable": true}); !errors.Is(err, ErrLifecycleAuthorityRequired) {
		t.Fatalf("lifecycle revision metadata patch error = %v, want ErrLifecycleAuthorityRequired", err)
	}
	stale := second
	stale.CommandID = "commit-head-stale"
	stale.Revision.RevisionID = "revision-lifecycle-stale"
	stale.CommandDigest, _ = ComputeCommitLifecycleArtifactHeadDigest(stale)
	if _, err := s.CommitLifecycleArtifactHead(ctx, stale); !errors.Is(err, ErrConcurrentStateChange) {
		t.Fatalf("stale commit error = %v, want ErrConcurrentStateChange", err)
	}
	events, err := s.ListLifecycleEvents(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID)
	if err != nil || len(events) != 3 {
		t.Fatalf("events after commits = %d, %v", len(events), err)
	}
}

func TestStartLifecycleDigestIgnoresRetryEphemeraButBindsContent(t *testing.T) {
	first := lifecycleStartFixture()
	now := time.Now().UTC()
	first.InitialWork.CreatedByRunID = "run-first"
	first.InitialWork.CreatedAt, first.InitialWork.UpdatedAt = now, now
	first.InitialDocument.CreatedAt, first.InitialDocument.UpdatedAt = now, now
	first.InitialRevision.CreatedAt = now
	first.InitialRevision.Metadata, _ = json.Marshal(map[string]any{
		"seed_prompt": "write a durable note", "conductor_loop_id": "run-first", "prompt_unix_ts": now.Unix(),
	})
	first.Agent.CreatedAt, first.Agent.UpdatedAt = now, now
	firstDigest, err := ComputeStartLifecycleRequestDigest(first)
	if err != nil {
		t.Fatal(err)
	}

	retry := first
	retry.InitialWork.CreatedByRunID = "run-retry"
	retry.InitialWork.CreatedAt, retry.InitialWork.UpdatedAt = now.Add(time.Minute), now.Add(time.Minute)
	retry.InitialDocument.CreatedAt, retry.InitialDocument.UpdatedAt = now.Add(time.Minute), now.Add(time.Minute)
	retry.InitialRevision.CreatedAt = now.Add(time.Minute)
	retry.InitialRevision.Metadata, _ = json.Marshal(map[string]any{
		"seed_prompt": "write a durable note", "conductor_loop_id": "run-retry", "prompt_unix_ts": now.Add(time.Minute).Unix(),
	})
	retry.Agent.CreatedAt, retry.Agent.UpdatedAt = now.Add(time.Minute), now.Add(time.Minute)
	retryDigest, err := ComputeStartLifecycleRequestDigest(retry)
	if err != nil {
		t.Fatal(err)
	}
	if retryDigest != firstDigest {
		t.Fatalf("retry digest = %q, want stable %q", retryDigest, firstDigest)
	}
	retry.InitialRevision.Content = "different durable note"
	changedDigest, err := ComputeStartLifecycleRequestDigest(retry)
	if err != nil {
		t.Fatal(err)
	}
	if changedDigest == firstDigest {
		t.Fatal("semantic content change reused the start request digest")
	}
}

func queueLifecycleUpdateFixture(t *testing.T, req types.StartLifecycleRequest, commandID string) types.QueueLifecycleUpdateRequest {
	t.Helper()
	packet := testStoreCoagentPacket("result", "durable update")
	payloadDigest, err := ComputeLifecycleUpdatePayloadDigest(packet, "update content")
	if err != nil {
		t.Fatalf("payload digest: %v", err)
	}
	update := types.QueueLifecycleUpdateRequest{
		OwnerID: req.OwnerID, ComputerID: req.ComputerID, CommandID: commandID,
		TrajectoryID: req.TrajectoryID, TargetAgentID: req.Agent.AgentID,
		ProducerAgentID: "producer-agent", ProducerUpdateID: "producer-update-1",
		UpdateID: "update-lifecycle-1", Packet: packet, Content: "update content",
		PayloadDigest: payloadDigest,
	}
	digest, err := ComputeQueueLifecycleUpdateDigest(update)
	if err != nil {
		t.Fatalf("queue digest: %v", err)
	}
	update.CommandDigest = digest
	return update
}

func TestQueueLifecycleUpdateValidatesProducerWorkBinding(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	start := lifecycleStartFixture()
	start.CommandID = "command-start-work-binding"
	start.TrajectoryID = "trajectory-work-binding"
	start.InitialWork.WorkItemID = "work-binding-root"
	start.StartRequestDigest, _ = ComputeStartLifecycleRequestDigest(start)
	if _, err := s.StartLifecycle(ctx, start); err != nil {
		t.Fatalf("start lifecycle: %v", err)
	}
	open := types.OpenLifecycleWorkRequest{
		OwnerID: start.OwnerID, ComputerID: start.ComputerID,
		CommandID: "command-open-producer-work", TrajectoryID: start.TrajectoryID,
		WorkItem: types.WorkItemRecord{
			WorkItemID: "work-binding-producer", Objective: "produce an update",
			AssignedAgentID: "producer-agent", AuthorityProfile: "researcher",
		},
	}
	open.CommandDigest, _ = ComputeOpenLifecycleWorkDigest(open)
	if _, err := s.OpenLifecycleWork(ctx, open); err != nil {
		t.Fatalf("open producer work: %v", err)
	}
	if _, err := s.GetAgentByScope(ctx, start.OwnerID, start.ComputerID, open.WorkItem.AssignedAgentID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("open work created a durable subject: %v", err)
	}
	base := queueLifecycleUpdateFixture(t, start, "command-queue-work-binding")
	base.WorkItemID, base.WorkDisposition = open.WorkItem.WorkItemID, types.WorkItemOpen

	assertRefused := func(name string, candidate types.QueueLifecycleUpdateRequest) {
		t.Helper()
		candidate.CommandID = "command-queue-invalid-" + name
		candidate.ProducerUpdateID = "producer-update-" + name
		candidate.UpdateID = "update-" + name
		candidate.CommandDigest, _ = ComputeQueueLifecycleUpdateDigest(candidate)
		if _, err := s.QueueLifecycleUpdate(ctx, candidate); err == nil {
			t.Fatalf("%s work binding unexpectedly queued", name)
		}
	}
	nonexistent := base
	nonexistent.WorkItemID = "work-does-not-exist"
	assertRefused("nonexistent", nonexistent)
	wrongAssignee := base
	wrongAssignee.ProducerAgentID = "different-producer"
	assertRefused("wrong-assignee", wrongAssignee)

	second := lifecycleStartFixture()
	second.CommandID = "command-start-foreign-work"
	second.TrajectoryID = "trajectory-foreign-work"
	second.InitialWork.WorkItemID = "work-foreign"
	second.InitialDocument.DocID = "document-foreign"
	second.InitialRevision.RevisionID = "revision-foreign"
	second.Agent.AgentID = "texture:document-foreign"
	second.Agent.ChannelID = second.InitialDocument.DocID
	second.InitialWork.AssignedAgentID = second.Agent.AgentID
	second.StartRequestDigest, _ = ComputeStartLifecycleRequestDigest(second)
	if _, err := s.StartLifecycle(ctx, second); err != nil {
		t.Fatalf("start foreign lifecycle: %v", err)
	}
	foreign := base
	foreign.WorkItemID = second.InitialWork.WorkItemID
	foreign.ProducerAgentID = second.InitialWork.AssignedAgentID
	assertRefused("foreign-trajectory", foreign)

	settle := types.SettleLifecycleWorkRequest{
		OwnerID: start.OwnerID, ComputerID: start.ComputerID,
		CommandID: "command-settle-producer-work", TrajectoryID: start.TrajectoryID,
		WorkItemID: open.WorkItem.WorkItemID, ResultRef: "artifact://producer-result",
		ActingAgentID: "producer-agent",
	}
	settle.CommandDigest, _ = ComputeSettleLifecycleWorkDigest(settle)
	if _, err := s.SettleLifecycleWork(ctx, settle); err != nil {
		t.Fatalf("settle producer work: %v", err)
	}
	terminal := base
	terminal.WorkDisposition = types.WorkItemCompleted
	terminal.WorkResultRef = "artifact://producer-result"
	assertRefused("terminal-work", terminal)
}

func TestLifecycleSettlementWaitsForUpdateDisposition(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	start := lifecycleStartFixture()
	start.StartRequestDigest, _ = ComputeStartLifecycleRequestDigest(start)
	if _, err := s.StartLifecycle(ctx, start); err != nil {
		t.Fatalf("start lifecycle: %v", err)
	}
	queue := queueLifecycleUpdateFixture(t, start, "command-queue-1")
	queued, err := s.QueueLifecycleUpdate(ctx, queue)
	if err != nil {
		t.Fatalf("queue update: %v", err)
	}
	if queued.Trajectory.ReducerSeq != 2 {
		t.Fatalf("queue reducer seq = %d, want 2", queued.Trajectory.ReducerSeq)
	}
	replayed, err := s.QueueLifecycleUpdate(ctx, queue)
	if err != nil || !replayed.Replay {
		t.Fatalf("queue replay = %+v, %v", replayed, err)
	}
	for name, mutate := range map[string]func(*types.QueueLifecycleUpdateRequest){
		"payload": func(candidate *types.QueueLifecycleUpdateRequest) {
			candidate.PayloadDigest = "sha256:changed-payload"
		},
		"work consequence": func(candidate *types.QueueLifecycleUpdateRequest) {
			candidate.WorkDisposition = types.WorkItemOpen
			candidate.WorkItemID = start.InitialWork.WorkItemID
		},
	} {
		t.Run("same key rejects changed "+name, func(t *testing.T) {
			candidate := queue
			mutate(&candidate)
			candidate.CommandDigest, _ = ComputeQueueLifecycleUpdateDigest(candidate)
			if _, err := s.QueueLifecycleUpdate(ctx, candidate); !errors.Is(err, ErrLifecycleCommandConflict) {
				t.Fatalf("changed %s error = %v, want command conflict", name, err)
			}
		})
	}
	activationRetry := queue
	activationRetry.UpdateID = "different-activation-update-id"
	activationRetry.SourceRunID = "replacement-activation"
	activationRetry.CommandDigest, _ = ComputeQueueLifecycleUpdateDigest(activationRetry)
	activationReplay, err := s.QueueLifecycleUpdate(ctx, activationRetry)
	if err != nil || !activationReplay.Replay || activationReplay.Update == nil ||
		activationReplay.Update.UpdateID != queue.UpdateID {
		t.Fatalf("activation-independent queue replay = %+v, %v", activationReplay, err)
	}
	updateKeyRetry := activationRetry
	updateKeyRetry.CommandID = "command-queue-replacement-activation"
	updateKeyRetry.CommandDigest, _ = ComputeQueueLifecycleUpdateDigest(updateKeyRetry)
	updateKeyReplay, err := s.QueueLifecycleUpdate(ctx, updateKeyRetry)
	if err != nil || !updateKeyReplay.Replay || updateKeyReplay.Update == nil ||
		updateKeyReplay.Update.UpdateID != queue.UpdateID {
		t.Fatalf("update-key replay = %+v, %v", updateKeyReplay, err)
	}

	settle := types.SettleLifecycleWorkRequest{
		OwnerID: start.OwnerID, ComputerID: start.ComputerID,
		CommandID: "command-settle-1", TrajectoryID: start.TrajectoryID,
		WorkItemID: start.InitialWork.WorkItemID, ResultRef: "texture://result/1",
		ActingAgentID: start.Agent.AgentID,
	}
	settle.CommandDigest, _ = ComputeSettleLifecycleWorkDigest(settle)
	settledWork, err := s.SettleLifecycleWork(ctx, settle)
	if err != nil {
		t.Fatalf("settle work: %v", err)
	}
	if settledWork.Trajectory.Status != types.TrajectoryLive {
		t.Fatalf("trajectory status with pending update = %q, want live", settledWork.Trajectory.Status)
	}

	apply := types.ApplyLifecycleUpdateRequest(queue)
	apply.CommandID = "command-apply-1"
	apply.Disposition = types.UpdateIncorporated
	apply.Revision = types.Revision{
		RevisionID: "revision-lifecycle-v1", AuthorKind: types.AuthorAppAgent, AuthorLabel: "researcher", Content: "Incorporated update",
		CreatedAt:  time.Unix(100, 0).UTC(),
		Provenance: json.RawMessage(`{"schema_version":1,"authored_at":"1970-01-01T00:01:40Z","authoring_model":{"provider":"test","model":"stable"}}`),
	}
	apply.DispositionRef = apply.Revision.RevisionID
	apply.CommandDigest = ""
	apply.CommandDigest, _ = ComputeApplyLifecycleUpdateDigest(apply)
	applied, err := s.ApplyLifecycleUpdate(ctx, apply)
	if err != nil {
		t.Fatalf("apply update: %v", err)
	}
	if applied.Trajectory.Status != types.TrajectoryLive || len(applied.Events) != 1 {
		t.Fatalf("update disposition settled trajectory implicitly: %+v", applied)
	}
	applyRetry := apply
	applyRetry.UpdateID = "replacement-activation-update"
	applyRetry.SourceRunID = "replacement-activation-run"
	applyRetry.ChannelID = "replacement-channel"
	applyRetry.Role = "replacement-role"
	applyRetry.MessageSeq = 999
	applyRetry.Revision.CreatedAt = time.Unix(200, 0).UTC()
	applyRetry.Revision.Provenance = json.RawMessage(`{"schema_version":1,"authored_at":"1970-01-01T00:03:20Z","authoring_model":{"provider":"test","model":"stable"}}`)
	applyRetry.CommandDigest, _ = ComputeApplyLifecycleUpdateDigest(applyRetry)
	if applyRetry.CommandDigest != apply.CommandDigest {
		t.Fatalf("apply digest changed under activation replacement: %s != %s", applyRetry.CommandDigest, apply.CommandDigest)
	}
	applyReplay, err := s.ApplyLifecycleUpdate(ctx, applyRetry)
	if err != nil || !applyReplay.Replay || applyReplay.Revision == nil ||
		applyReplay.Revision.RevisionID != apply.Revision.RevisionID {
		t.Fatalf("activation-independent apply replay = %+v, %v", applyReplay, err)
	}
	settleTrajectory := types.SettleLifecycleTrajectoryRequest{
		OwnerID: start.OwnerID, ComputerID: start.ComputerID,
		CommandID: "command-settle-trajectory-1", TrajectoryID: start.TrajectoryID,
		ExpectedLifecycleVersion: applied.Trajectory.LifecycleVersion,
		ExpectedHeadRevisionID:   apply.Revision.RevisionID,
	}
	settleTrajectory.CommandDigest, _ = ComputeSettleLifecycleTrajectoryDigest(settleTrajectory)
	settledTrajectory, err := s.SettleLifecycleTrajectory(ctx, settleTrajectory)
	if err != nil {
		t.Fatalf("settle trajectory: %v", err)
	}
	if settledTrajectory.Trajectory.Status != types.TrajectorySettled || len(settledTrajectory.Events) != 1 {
		t.Fatalf("unexpected explicit settlement: %+v", settledTrajectory)
	}
	lateQueue := queueLifecycleUpdateFixture(t, start, "command-queue-late")
	lateQueue.UpdateID, lateQueue.ProducerUpdateID = "update-lifecycle-late", "producer-update-late"
	lateQueue.CommandDigest = ""
	lateQueue.CommandDigest, _ = ComputeQueueLifecycleUpdateDigest(lateQueue)
	late, err := s.QueueLifecycleUpdate(ctx, lateQueue)
	if err != nil {
		t.Fatalf("record late update: %v", err)
	}
	if late.Trajectory.Status != types.TrajectorySettled || late.Trajectory.ReducerSeq != settledTrajectory.Trajectory.ReducerSeq ||
		len(late.Events) != 1 || late.Events[0].Kind != types.LifecycleUpdateLate {
		t.Fatalf("late update mutated terminal trajectory: %+v", late)
	}
	if late.Update == nil || late.Update.DispositionRef != lifecycleTerminalTrajectoryRef(start.TrajectoryID) {
		t.Fatalf("late update terminal ref = %+v", late.Update)
	}
	update, err := s.GetLifecycleUpdate(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID, queue.TargetAgentID, queue.ProducerAgentID, queue.ProducerUpdateID)
	if err != nil {
		t.Fatalf("get lifecycle update: %v", err)
	}
	if update.Disposition != types.UpdateIncorporated || update.DispositionRef != apply.Revision.RevisionID || update.DeliveredToRunID != "" {
		t.Fatalf("unexpected incorporated update: %+v", update)
	}
	snapshot, err := s.GetLifecycleSnapshot(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID)
	if err != nil || snapshot.Document.CurrentRevisionID != apply.Revision.RevisionID || snapshot.HeadRevision.ParentRevisionID != start.InitialRevision.RevisionID {
		t.Fatalf("unexpected artifact head after incorporation: %+v, %v", snapshot, err)
	}
}

func TestLifecycleUpdateWorkConsequenceRequiresExplicitDispositionAndAssignedProducer(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	start := lifecycleStartFixture()
	start.StartRequestDigest, _ = ComputeStartLifecycleRequestDigest(start)
	if _, err := s.StartLifecycle(ctx, start); err != nil {
		t.Fatalf("start lifecycle: %v", err)
	}
	ambiguous := queueLifecycleUpdateFixture(t, start, "command-queue-ambiguous-work")
	ambiguous.WorkItemID = start.InitialWork.WorkItemID
	ambiguous.CommandDigest, _ = ComputeQueueLifecycleUpdateDigest(ambiguous)
	if _, err := s.QueueLifecycleUpdate(ctx, ambiguous); err == nil {
		t.Fatal("queue accepted work_item_id without explicit terminal work disposition")
	}

	queue := queueLifecycleUpdateFixture(t, start, "command-queue-explicit-work")
	queue.WorkDisposition = types.WorkItemCompleted
	queue.WorkItemID = start.InitialWork.WorkItemID
	queue.CommandDigest, _ = ComputeQueueLifecycleUpdateDigest(queue)
	if _, err := s.QueueLifecycleUpdate(ctx, queue); err == nil {
		t.Fatal("queue accepted work consequence from producer not assigned to work")
	}
	snapshot, err := s.GetLifecycleSnapshot(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID)
	if err != nil || snapshot.WorkItems[0].Status != types.WorkItemOpen {
		t.Fatalf("refused work consequence mutated assigned work: %+v, %v", snapshot.WorkItems, err)
	}
}

func TestCancelLifecycleTrajectoryCancelsWorkAndPendingUpdates(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	start := lifecycleStartFixture()
	start.CommandID = "command-start-cancel"
	start.TrajectoryID = "trajectory-cancel"
	start.InitialWork.WorkItemID = "work-cancel"
	start.StartRequestDigest, _ = ComputeStartLifecycleRequestDigest(start)
	if _, err := s.StartLifecycle(ctx, start); err != nil {
		t.Fatalf("start lifecycle: %v", err)
	}
	queue := queueLifecycleUpdateFixture(t, start, "command-queue-cancel")
	if _, err := s.QueueLifecycleUpdate(ctx, queue); err != nil {
		t.Fatalf("queue update: %v", err)
	}
	snapshot, err := s.GetLifecycleSnapshot(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID)
	if err != nil {
		t.Fatalf("snapshot before cancel: %v", err)
	}
	cancel := types.CancelLifecycleRequest{
		OwnerID: start.OwnerID, ComputerID: start.ComputerID,
		CommandID: "command-cancel-1", TrajectoryID: start.TrajectoryID,
		ExpectedLifecycleVersion: snapshot.Trajectory.LifecycleVersion,
		ExpectedHeadRevisionID:   snapshot.HeadRevision.RevisionID,
		Reason:                   "owner cancelled",
	}
	stale := cancel
	stale.ExpectedLifecycleVersion--
	stale.CommandID = "command-cancel-stale"
	stale.CommandDigest, _ = ComputeCancelLifecycleDigest(stale)
	if _, err := s.CancelLifecycleTrajectory(ctx, stale); !errors.Is(err, ErrConcurrentStateChange) {
		t.Fatalf("stale cancellation error = %v, want ErrConcurrentStateChange", err)
	}
	live, err := s.GetLifecycleTrajectory(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID)
	if err != nil || live.Status != types.TrajectoryLive {
		t.Fatalf("stale cancellation changed trajectory = %+v, %v", live, err)
	}
	cancel.CommandDigest, _ = ComputeCancelLifecycleDigest(cancel)
	result, err := s.CancelLifecycleTrajectory(ctx, cancel)
	if err != nil {
		t.Fatalf("cancel lifecycle: %v", err)
	}
	if result.Trajectory.Status != types.TrajectoryCancelled || result.Trajectory.CancelledAt == nil {
		t.Fatalf("unexpected cancelled trajectory: %+v", result.Trajectory)
	}
	work, err := s.GetLifecycleWorkItem(ctx, start.OwnerID, start.ComputerID, start.InitialWork.WorkItemID)
	if err != nil || work.Status != types.WorkItemCancelled {
		t.Fatalf("cancelled work = %+v, %v", work, err)
	}
	update, err := s.GetLifecycleUpdate(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID, queue.TargetAgentID, queue.ProducerAgentID, queue.ProducerUpdateID)
	if err != nil || update.Disposition != types.UpdateCancelled {
		t.Fatalf("cancelled update = %+v, %v", update, err)
	}
	if update.DispositionRef != lifecycleTerminalTrajectoryRef(start.TrajectoryID) {
		t.Fatalf("cancelled update terminal ref = %q", update.DispositionRef)
	}
	cancelledSnapshot, err := s.GetLifecycleSnapshot(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID)
	if err != nil {
		t.Fatalf("cancelled snapshot: %v", err)
	}
	apply := types.ApplyLifecycleUpdateRequest(queue)
	apply.CommandID = "command-apply-after-cancel"
	apply.Disposition = types.UpdateIncorporated
	apply.ReferenceExistingArtifact = true
	apply.DispositionRef = start.InitialRevision.RevisionID
	apply.CommandDigest, _ = ComputeApplyLifecycleUpdateDigest(apply)
	if _, err := s.ApplyLifecycleUpdate(ctx, apply); !errors.Is(err, ErrLifecycleInvalidTransition) {
		t.Fatalf("apply after cancellation error = %v, want invalid transition", err)
	}
	unchanged, err := s.GetLifecycleSnapshot(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID)
	if err != nil || unchanged.Trajectory.ReducerSeq != cancelledSnapshot.Trajectory.ReducerSeq ||
		unchanged.Updates[0].Disposition != types.UpdateCancelled {
		t.Fatalf("terminal apply mutated lifecycle: before=%+v after=%+v err=%v", cancelledSnapshot, unchanged, err)
	}
}

func TestLifecycleApplyAndCancellationRaceIsLinearizable(t *testing.T) {
	path := testStorePath(t)
	cleanupTestStorePath(path)
	t.Cleanup(func() { cleanupTestStorePath(path) })
	ctx := context.Background()
	s, err := Open(path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer s.Close()

	type outcome struct {
		result types.LifecycleResult
		err    error
	}
	runRound := func(round int, ordering string) string {
		t.Helper()
		suffix := string(rune('a' + round))
		start := lifecycleStartFixture()
		start.CommandID = "command-race-start-" + suffix
		start.TrajectoryID = "trajectory-race-" + suffix
		start.InitialWork.WorkItemID = "work-race-" + suffix
		start.InitialDocument.DocID = "document-race-" + suffix
		start.InitialRevision.RevisionID = "revision-race-v0-" + suffix
		start.SubjectRefs["doc_id"] = start.InitialDocument.DocID
		start.Agent.AgentID = "texture:" + start.InitialDocument.DocID
		start.Agent.ChannelID = start.InitialDocument.DocID
		start.StartRequestDigest, _ = ComputeStartLifecycleRequestDigest(start)
		if _, err := s.StartLifecycle(ctx, start); err != nil {
			t.Fatalf("start lifecycle %s: %v", suffix, err)
		}
		queue := queueLifecycleUpdateFixture(t, start, "command-race-queue-"+suffix)
		queue.UpdateID, queue.ProducerUpdateID = "update-race-"+suffix, "producer-update-race-"+suffix
		queue.ProducerAgentID = start.Agent.AgentID
		queue.WorkDisposition, queue.WorkItemID = types.WorkItemCompleted, start.InitialWork.WorkItemID
		queue.CommandDigest, _ = ComputeQueueLifecycleUpdateDigest(queue)
		if _, err := s.QueueLifecycleUpdate(ctx, queue); err != nil {
			t.Fatalf("queue lifecycle update %s: %v", suffix, err)
		}
		before, err := s.GetLifecycleSnapshot(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID)
		if err != nil {
			t.Fatalf("snapshot before race %s: %v", suffix, err)
		}
		apply := types.ApplyLifecycleUpdateRequest(queue)
		apply.CommandID = "command-race-apply-" + suffix
		apply.Disposition, apply.DispositionRef = types.UpdateIncorporated, "revision-race-v1-"+suffix
		apply.WorkResultRef = apply.DispositionRef
		apply.Revision = types.Revision{
			RevisionID: apply.DispositionRef, AuthorKind: types.AuthorAppAgent,
			AuthorLabel: "Choir", Content: "linearizable artifact " + suffix,
		}
		apply.CommandDigest, _ = ComputeApplyLifecycleUpdateDigest(apply)
		cancel := types.CancelLifecycleRequest{
			OwnerID: start.OwnerID, ComputerID: start.ComputerID,
			CommandID: "command-race-cancel-" + suffix, TrajectoryID: start.TrajectoryID,
			ExpectedLifecycleVersion: before.Trajectory.LifecycleVersion,
			ExpectedHeadRevisionID:   before.HeadRevision.RevisionID,
			Reason:                   "race cancellation",
		}
		cancel.CommandDigest, _ = ComputeCancelLifecycleDigest(cancel)
		applyCall := func() outcome {
			result, err := s.ApplyLifecycleUpdate(ctx, apply)
			return outcome{result: result, err: err}
		}
		cancelCall := func() outcome {
			result, err := s.CancelLifecycleTrajectory(ctx, cancel)
			return outcome{result: result, err: err}
		}
		var applied, cancelled outcome
		switch ordering {
		case "apply-first":
			applied = applyCall()
			cancelled = cancelCall()
		case "cancel-first":
			cancelled = cancelCall()
			applied = applyCall()
		default:
			startRace := make(chan struct{})
			applyDone, cancelDone := make(chan outcome, 1), make(chan outcome, 1)
			go func() {
				<-startRace
				applyDone <- applyCall()
			}()
			go func() {
				<-startRace
				cancelDone <- cancelCall()
			}()
			close(startRace)
			applied, cancelled = <-applyDone, <-cancelDone
		}
		if (applied.err == nil) == (cancelled.err == nil) {
			t.Fatalf("%s round %s did not produce exactly one winner: apply=%v cancel=%v", ordering, suffix, applied.err, cancelled.err)
		}
		winner := "cancel"
		winnerCommand, loserCommand := cancel.CommandID, apply.CommandID
		if applied.err == nil {
			winner, winnerCommand, loserCommand = "apply", apply.CommandID, cancel.CommandID
		}
		loserErr := applied.err
		if winner == "apply" {
			loserErr = cancelled.err
		}
		if !errors.Is(loserErr, ErrConcurrentStateChange) && !errors.Is(loserErr, ErrLifecycleInvalidTransition) {
			t.Fatalf("%s round %s loser error = %v", ordering, suffix, loserErr)
		}
		if ordering == "apply-first" && winner != "apply" || ordering == "cancel-first" && winner != "cancel" {
			t.Fatalf("forced %s round %s winner = %s", ordering, suffix, winner)
		}

		var replayed types.LifecycleResult
		if winner == "apply" {
			replayed, err = s.ApplyLifecycleUpdate(ctx, apply)
			if _, loserRetryErr := s.CancelLifecycleTrajectory(ctx, cancel); !errors.Is(loserRetryErr, ErrConcurrentStateChange) && !errors.Is(loserRetryErr, ErrLifecycleInvalidTransition) {
				t.Fatalf("cancel loser retry %s = %v", suffix, loserRetryErr)
			}
			if replayed.Receipt.CommandID != applied.result.Receipt.CommandID || replayed.Receipt.CommandDigest != applied.result.Receipt.CommandDigest || replayed.Receipt.ReducerSeq != applied.result.Receipt.ReducerSeq {
				t.Fatalf("apply replay receipt changed %s: %+v != %+v", suffix, replayed.Receipt, applied.result.Receipt)
			}
		} else {
			replayed, err = s.CancelLifecycleTrajectory(ctx, cancel)
			if _, loserRetryErr := s.ApplyLifecycleUpdate(ctx, apply); !errors.Is(loserRetryErr, ErrConcurrentStateChange) && !errors.Is(loserRetryErr, ErrLifecycleInvalidTransition) {
				t.Fatalf("apply loser retry %s = %v", suffix, loserRetryErr)
			}
			if replayed.Receipt.CommandID != cancelled.result.Receipt.CommandID || replayed.Receipt.CommandDigest != cancelled.result.Receipt.CommandDigest || replayed.Receipt.ReducerSeq != cancelled.result.Receipt.ReducerSeq {
				t.Fatalf("cancel replay receipt changed %s: %+v != %+v", suffix, replayed.Receipt, cancelled.result.Receipt)
			}
		}
		if err != nil || !replayed.Replay {
			t.Fatalf("%s winner retry %s did not replay: %+v, %v", winner, suffix, replayed.Receipt, err)
		}

		after, err := s.GetLifecycleSnapshot(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID)
		if err != nil {
			t.Fatalf("snapshot after race %s: %v", suffix, err)
		}
		if len(after.WorkItems) != 1 || len(after.Updates) != 1 {
			t.Fatalf("race %s changed object cardinality: %+v", suffix, after)
		}
		if after.SnapshotCursor != after.Trajectory.ReducerSeq || len(after.Events) <= len(before.Events) {
			t.Fatalf("race %s has inconsistent cursor/events: %+v", suffix, after)
		}
		for i, event := range after.Events {
			if i > 0 && event.ReducerSeq <= after.Events[i-1].ReducerSeq {
				t.Fatalf("race %s reducer sequence is not strictly monotonic: %+v", suffix, after.Events)
			}
			if event.CommandID == loserCommand {
				t.Fatalf("race %s persisted loser event: %+v", suffix, event)
			}
		}
		winnerEvents := 0
		for _, event := range after.Events {
			if event.CommandID == winnerCommand {
				winnerEvents++
			}
		}
		if winnerEvents == 0 {
			t.Fatalf("race %s persisted no winner events: %+v", suffix, after.Events)
		}
		if winner == "apply" {
			if after.HeadRevision.RevisionID != apply.Revision.RevisionID ||
				after.Trajectory.Status != types.TrajectoryLive ||
				after.WorkItems[0].Status != types.WorkItemCompleted ||
				after.Updates[0].Disposition != types.UpdateIncorporated {
				t.Fatalf("applied race outcome %s leaked partial state: %+v", suffix, after)
			}
		} else if after.HeadRevision.RevisionID != start.InitialRevision.RevisionID ||
			after.Trajectory.Status != types.TrajectoryCancelled ||
			after.WorkItems[0].Status != types.WorkItemCancelled ||
			after.Updates[0].Disposition != types.UpdateCancelled {
			t.Fatalf("cancelled race outcome %s leaked partial state: %+v", suffix, after)
		}
		return winner
	}

	if winner := runRound(0, "apply-first"); winner != "apply" {
		t.Fatalf("apply-first proof winner = %s", winner)
	}
	if winner := runRound(1, "cancel-first"); winner != "cancel" {
		t.Fatalf("cancel-first proof winner = %s", winner)
	}
	for round := 2; round < 22; round++ {
		runRound(round, "concurrent")
	}
}

func TestLifecycleSnapshotReconstructsAfterRestart(t *testing.T) {
	path := testStorePath(t)
	cleanupTestStorePath(path)
	t.Cleanup(func() { cleanupTestStorePath(path) })
	ctx := context.Background()
	first, err := Open(path)
	if err != nil {
		t.Fatalf("open first store: %v", err)
	}
	start := lifecycleStartFixture()
	start.CommandID = "command-start-restart"
	start.TrajectoryID = "trajectory-restart"
	start.InitialWork.WorkItemID = "work-restart"
	start.StartRequestDigest, _ = ComputeStartLifecycleRequestDigest(start)
	if _, err := first.StartLifecycle(ctx, start); err != nil {
		t.Fatalf("start lifecycle: %v", err)
	}
	queue := queueLifecycleUpdateFixture(t, start, "command-queue-restart")
	if _, err := first.QueueLifecycleUpdate(ctx, queue); err != nil {
		t.Fatalf("queue update: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("close first store: %v", err)
	}
	reopened, err := Open(path)
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	defer reopened.Close()
	replayed, err := reopened.QueueLifecycleUpdate(ctx, queue)
	if err != nil || !replayed.Replay {
		t.Fatalf("queue replay after restart = %+v, %v", replayed, err)
	}
	replacementRetry := queue
	replacementRetry.UpdateID = "update-after-replacement"
	replacementRetry.SourceRunID = "run-after-replacement"
	replacementRetry.CommandDigest, _ = ComputeQueueLifecycleUpdateDigest(replacementRetry)
	replacementReplay, err := reopened.QueueLifecycleUpdate(ctx, replacementRetry)
	if err != nil || !replacementReplay.Replay || replacementReplay.Update == nil ||
		replacementReplay.Update.UpdateID != queue.UpdateID {
		t.Fatalf("replacement activation replay after restart = %+v, %v", replacementReplay, err)
	}
	conflicting := queue
	conflicting.PayloadDigest = "sha256:changed-after-restart"
	conflicting.CommandDigest, _ = ComputeQueueLifecycleUpdateDigest(conflicting)
	if _, err := reopened.QueueLifecycleUpdate(ctx, conflicting); !errors.Is(err, ErrLifecycleCommandConflict) {
		t.Fatalf("queue conflict after restart = %v, want command conflict", err)
	}
	snapshot, err := reopened.GetLifecycleSnapshot(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID)
	if err != nil {
		t.Fatalf("reconstruct snapshot: %v", err)
	}
	if snapshot.Trajectory.ReducerSeq != 2 || len(snapshot.WorkItems) != 1 || len(snapshot.Agents) != 1 || len(snapshot.Updates) != 1 || len(snapshot.Events) != 2 {
		t.Fatalf("unexpected reconstructed snapshot: %+v", snapshot)
	}
	if snapshot.Updates[0].Disposition != types.UpdatePending {
		t.Fatalf("reconstructed update disposition = %q, want pending", snapshot.Updates[0].Disposition)
	}
}

func TestLifecycleRelatedUpdatesCommitWithArtifactAndWorkAtomically(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	start := lifecycleStartFixture()
	start.CommandID = "command-start-related"
	start.TrajectoryID = "trajectory-related"
	start.InitialWork.WorkItemID = "work-texture-related"
	start.InitialDocument.DocID = "document-related"
	start.InitialRevision.RevisionID = "revision-related-v0"
	start.SubjectRefs["doc_id"] = start.InitialDocument.DocID
	start.Agent.AgentID = "texture:" + start.InitialDocument.DocID
	start.Agent.ChannelID = start.InitialDocument.DocID
	start.StartRequestDigest, _ = ComputeStartLifecycleRequestDigest(start)
	if _, err := s.StartLifecycle(ctx, start); err != nil {
		t.Fatalf("start lifecycle: %v", err)
	}
	open := types.OpenLifecycleWorkRequest{
		OwnerID: start.OwnerID, ComputerID: start.ComputerID,
		CommandID: "command-open-related-work", TrajectoryID: start.TrajectoryID,
		WorkItem: types.WorkItemRecord{
			WorkItemID: "work-producer-related", Objective: "produce evidence",
			AssignedAgentID: start.Agent.AgentID, AuthorityProfile: "texture",
		},
	}
	open.CommandDigest, _ = ComputeOpenLifecycleWorkDigest(open)
	if _, err := s.OpenLifecycleWork(ctx, open); err != nil {
		t.Fatalf("open producer work: %v", err)
	}

	primary := queueLifecycleUpdateFixture(t, start, "command-queue-related-primary")
	primary.UpdateID, primary.ProducerUpdateID = "update-related-primary", "producer-update-related-primary"
	primary.ProducerAgentID = start.Agent.AgentID
	primary.WorkDisposition, primary.WorkItemID = types.WorkItemCompleted, start.InitialWork.WorkItemID
	primary.CommandDigest, _ = ComputeQueueLifecycleUpdateDigest(primary)
	if _, err := s.QueueLifecycleUpdate(ctx, primary); err != nil {
		t.Fatalf("queue primary update: %v", err)
	}
	related := queueLifecycleUpdateFixture(t, start, "command-queue-related-evidence")
	related.UpdateID, related.ProducerUpdateID = "update-related-evidence", "producer-update-related-evidence"
	related.ProducerAgentID = open.WorkItem.AssignedAgentID
	related.WorkDisposition, related.WorkItemID = types.WorkItemCompleted, open.WorkItem.WorkItemID
	related.CommandDigest, _ = ComputeQueueLifecycleUpdateDigest(related)
	if _, err := s.QueueLifecycleUpdate(ctx, related); err != nil {
		t.Fatalf("queue related update: %v", err)
	}
	relatedSecond := queueLifecycleUpdateFixture(t, start, "command-queue-related-second")
	relatedSecond.UpdateID, relatedSecond.ProducerUpdateID = "update-related-second", "producer-update-a"
	relatedSecond.ProducerAgentID = start.Agent.AgentID
	relatedSecond.WorkDisposition, relatedSecond.WorkItemID = types.WorkItemOpen, start.InitialWork.WorkItemID
	relatedSecond.CommandDigest, _ = ComputeQueueLifecycleUpdateDigest(relatedSecond)
	if _, err := s.QueueLifecycleUpdate(ctx, relatedSecond); err != nil {
		t.Fatalf("queue second related update: %v", err)
	}

	apply := types.ApplyLifecycleUpdateRequest(primary)
	apply.CommandID = "command-apply-related"
	apply.Disposition, apply.DispositionRef = types.UpdateIncorporated, "revision-related-v1"
	apply.WorkResultRef = apply.DispositionRef
	apply.Revision = types.Revision{
		RevisionID: apply.DispositionRef, AuthorKind: types.AuthorAppAgent,
		AuthorLabel: "Choir", Content: "Evidence incorporated atomically",
	}
	apply.RelatedUpdates = []types.ApplyLifecycleRelatedUpdate{{
		TargetAgentID: related.TargetAgentID, ProducerAgentID: related.ProducerAgentID,
		ProducerUpdateID: related.ProducerUpdateID, UpdateID: related.UpdateID,
		Disposition: types.UpdateIncorporated, DispositionRef: apply.DispositionRef,
		WorkDisposition: related.WorkDisposition, WorkItemID: related.WorkItemID,
		WorkResultRef: apply.DispositionRef,
	}}
	apply.RelatedUpdates = append(apply.RelatedUpdates, types.ApplyLifecycleRelatedUpdate{
		TargetAgentID: relatedSecond.TargetAgentID, ProducerAgentID: relatedSecond.ProducerAgentID,
		ProducerUpdateID: relatedSecond.ProducerUpdateID, UpdateID: relatedSecond.UpdateID,
		Disposition: types.UpdateRejected, DispositionRef: apply.DispositionRef,
		WorkDisposition: relatedSecond.WorkDisposition, WorkItemID: relatedSecond.WorkItemID,
		Reason: "interim checkpoint was unusable",
	})
	invalid := apply
	invalid.CommandID = "command-apply-related-invalid"
	invalid.RelatedUpdates = append([]types.ApplyLifecycleRelatedUpdate(nil), apply.RelatedUpdates...)
	invalid.RelatedUpdates[0].WorkItemID = start.InitialWork.WorkItemID
	invalid.CommandDigest, _ = ComputeApplyLifecycleUpdateDigest(invalid)
	if _, err := s.ApplyLifecycleUpdate(ctx, invalid); err == nil {
		t.Fatal("related update identity mismatch unexpectedly committed")
	}
	before, err := s.GetLifecycleSnapshot(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID)
	if err != nil {
		t.Fatalf("snapshot after refused batch: %v", err)
	}
	if before.HeadRevision.RevisionID != start.InitialRevision.RevisionID {
		t.Fatalf("refused batch advanced artifact head: %+v", before)
	}
	for _, work := range before.WorkItems {
		if work.Status != types.WorkItemOpen {
			t.Fatalf("refused batch terminalized work: %+v", before.WorkItems)
		}
	}
	for _, update := range before.Updates {
		if update.Disposition != types.UpdatePending {
			t.Fatalf("refused batch terminalized update: %+v", before.Updates)
		}
	}

	apply.RelatedUpdates = []types.ApplyLifecycleRelatedUpdate{apply.RelatedUpdates[1], apply.RelatedUpdates[0]}
	apply.CommandDigest, _ = ComputeApplyLifecycleUpdateDigest(apply)
	if apply.RelatedUpdates[0].UpdateID != relatedSecond.UpdateID {
		t.Fatalf("digest computation mutated caller order: %+v", apply.RelatedUpdates)
	}
	if _, err := s.ApplyLifecycleUpdate(ctx, apply); err != nil {
		t.Fatalf("apply reverse-ordered related updates atomically: %v", err)
	}
	retry := apply
	retry.RelatedUpdates = append([]types.ApplyLifecycleRelatedUpdate(nil), apply.RelatedUpdates...)
	retry.RelatedUpdates[0], retry.RelatedUpdates[1] = retry.RelatedUpdates[1], retry.RelatedUpdates[0]
	retryDigest, _ := ComputeApplyLifecycleUpdateDigest(retry)
	if retryDigest != apply.CommandDigest {
		t.Fatalf("canonical related digest changed by input order: %s != %s", retryDigest, apply.CommandDigest)
	}
	retry.CommandDigest = retryDigest
	replayed, err := s.ApplyLifecycleUpdate(ctx, retry)
	if err != nil || !replayed.Replay {
		t.Fatalf("reverse-order exact related batch retry did not replay: %+v, %v", replayed.Receipt, err)
	}
	conflicting := retry
	conflicting.Reason = "changed retry payload"
	conflicting.CommandDigest, _ = ComputeApplyLifecycleUpdateDigest(conflicting)
	if _, err := s.ApplyLifecycleUpdate(ctx, conflicting); !errors.Is(err, ErrLifecycleCommandConflict) {
		t.Fatalf("reverse-order conflicting related batch retry = %v, want command conflict", err)
	}
	after, err := s.GetLifecycleSnapshot(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID)
	if err != nil {
		t.Fatalf("snapshot applied batch: %v", err)
	}
	var producerWork, primaryWork types.WorkItemRecord
	for _, work := range after.WorkItems {
		if work.WorkItemID == related.WorkItemID {
			producerWork = work
		}
		if work.WorkItemID == primary.WorkItemID {
			primaryWork = work
		}
	}
	if primaryWork.Status != types.WorkItemCompleted || primaryWork.ResultRef != apply.Revision.RevisionID {
		t.Fatalf("same-work open related consequence blocked primary completion: %+v", primaryWork)
	}
	if after.HeadRevision.RevisionID != apply.Revision.RevisionID ||
		producerWork.Status != types.WorkItemCompleted || producerWork.ResultRef != apply.Revision.RevisionID {
		t.Fatalf("artifact and producer work did not commit together: %+v", after)
	}
	for _, update := range after.Updates {
		wantDisposition := types.UpdateIncorporated
		if update.UpdateID == relatedSecond.UpdateID {
			wantDisposition = types.UpdateRejected
		}
		if update.Disposition != wantDisposition || update.DispositionRef != apply.Revision.RevisionID {
			t.Fatalf("batch left update with wrong explicit consequence: %+v", after.Updates)
		}
	}
}
func TestDurableWorkLifecycleSmokeTrace(t *testing.T) {
	path := testStorePath(t)
	cleanupTestStorePath(path)
	t.Cleanup(func() { cleanupTestStorePath(path) })
	ctx := context.Background()
	first, err := Open(path)
	if err != nil {
		t.Fatalf("open first store: %v", err)
	}

	start := lifecycleStartFixture()
	start.CommandID = "command-smoke-start"
	start.TrajectoryID = "trajectory-smoke"
	start.InitialWork.WorkItemID = "work-smoke"
	start.InitialDocument.DocID = "document-smoke"
	start.InitialRevision.RevisionID = "revision-smoke-v0"
	start.Agent.AgentID = "texture:document-smoke"
	start.Agent.ChannelID = "document-smoke"
	start.SubjectRefs["artifact"] = "texture://document-smoke"
	start.SubjectRefs["doc_id"] = start.InitialDocument.DocID
	start.StartRequestDigest, _ = ComputeStartLifecycleRequestDigest(start)
	started, err := first.StartLifecycle(ctx, start)
	if err != nil {
		t.Fatalf("start durable work: %v", err)
	}
	if started.Trajectory.Status != types.TrajectoryLive || started.WorkItem.Status != types.WorkItemOpen {
		t.Fatalf("unexpected start state: %+v", started)
	}

	queue := queueLifecycleUpdateFixture(t, start, "command-smoke-queue")
	queue.UpdateID = "update-smoke"
	queue.WorkDisposition = types.WorkItemCompleted
	queue.WorkItemID = start.InitialWork.WorkItemID
	queue.ProducerUpdateID = "producer-update-smoke"
	queue.ProducerAgentID = start.Agent.AgentID
	queue.CommandDigest, _ = ComputeQueueLifecycleUpdateDigest(queue)
	queued, err := first.QueueLifecycleUpdate(ctx, queue)
	if err != nil {
		t.Fatalf("queue durable update: %v", err)
	}
	if queued.Update.Disposition != types.UpdatePending {
		t.Fatalf("interim update was not left pending: %+v", queued)
	}
	openSnapshot, err := first.GetLifecycleSnapshot(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID)
	if err != nil || len(openSnapshot.WorkItems) != 1 || openSnapshot.WorkItems[0].Status != types.WorkItemOpen {
		t.Fatalf("interim update changed work state: %+v, %v", openSnapshot.WorkItems, err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("close store with pending update: %v", err)
	}
	second, err := Open(path)
	if err != nil {
		t.Fatalf("reopen store with pending update: %v", err)
	}
	pendingSnapshot, err := second.GetLifecycleSnapshot(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID)
	if err != nil || len(pendingSnapshot.Updates) != 1 ||
		pendingSnapshot.Updates[0].Disposition != types.UpdatePending ||
		pendingSnapshot.Updates[0].WorkDisposition != types.WorkItemCompleted ||
		pendingSnapshot.Updates[0].WorkItemID != start.InitialWork.WorkItemID {
		t.Fatalf("restart lost pending work consequence: %+v, %v", pendingSnapshot.Updates, err)
	}
	apply := types.ApplyLifecycleUpdateRequest(queue)
	apply.CommandID = "command-smoke-apply"
	apply.Disposition = types.UpdateIncorporated
	apply.DispositionRef = "revision-smoke-v1"
	apply.WorkItemID = start.InitialWork.WorkItemID
	apply.WorkDisposition = types.WorkItemCompleted
	apply.WorkResultRef = apply.DispositionRef
	apply.Revision = types.Revision{
		RevisionID: apply.DispositionRef, AuthorKind: types.AuthorAppAgent,
		AuthorLabel: "Choir", Content: "Durable update incorporated",
	}
	apply.CommandDigest, _ = ComputeApplyLifecycleUpdateDigest(apply)
	applied, err := second.ApplyLifecycleUpdate(ctx, apply)
	if err != nil {
		t.Fatalf("incorporate durable update: %v", err)
	}
	if applied.WorkItem == nil || applied.WorkItem.Status != types.WorkItemCompleted ||
		applied.Trajectory.Status != types.TrajectoryLive {
		t.Fatalf("unexpected incorporated work state: %+v", applied)
	}
	appliedSnapshot, err := second.GetLifecycleSnapshot(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID)
	if err != nil || len(appliedSnapshot.Updates) != 1 ||
		appliedSnapshot.Updates[0].Disposition != types.UpdateIncorporated {
		t.Fatalf("unexpected incorporated update state: %+v, %v", appliedSnapshot.Updates, err)
	}
	if err := second.Close(); err != nil {
		t.Fatalf("close applied store: %v", err)
	}

	reopened, err := Open(path)
	if err != nil {
		t.Fatalf("reopen durable store: %v", err)
	}
	defer reopened.Close()
	snapshot, err := reopened.GetLifecycleSnapshot(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID)
	if err != nil {
		t.Fatalf("reconstruct durable lifecycle: %v", err)
	}
	if snapshot.HeadRevision.RevisionID != apply.Revision.RevisionID ||
		len(snapshot.WorkItems) != 1 || snapshot.WorkItems[0].Status != types.WorkItemCompleted ||
		len(snapshot.Updates) != 1 || snapshot.Updates[0].Disposition != types.UpdateIncorporated {
		t.Fatalf("restart reconstruction lost lifecycle state: %+v", snapshot)
	}

	settle := types.SettleLifecycleTrajectoryRequest{
		OwnerID: start.OwnerID, ComputerID: start.ComputerID,
		CommandID: "command-smoke-settle", TrajectoryID: start.TrajectoryID,
		ExpectedLifecycleVersion: snapshot.Trajectory.LifecycleVersion,
		ExpectedHeadRevisionID:   snapshot.HeadRevision.RevisionID,
	}
	settle.CommandDigest, _ = ComputeSettleLifecycleTrajectoryDigest(settle)
	settled, err := reopened.SettleLifecycleTrajectory(ctx, settle)
	if err != nil {
		t.Fatalf("settle reconstructed lifecycle: %v", err)
	}
	if settled.Trajectory.Status != types.TrajectorySettled {
		t.Fatalf("durable lifecycle did not settle: %+v", settled)
	}
}

func TestLifecycleRejectedUpdateRefusesProducerWork(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	start := lifecycleStartFixture()
	start.CommandID = "command-start-rejection"
	start.TrajectoryID = "trajectory-rejection"
	start.InitialWork.WorkItemID = "work-rejection"
	start.InitialDocument.DocID = "document-rejection"
	start.InitialRevision.RevisionID = "revision-rejection-v0"
	start.SubjectRefs["doc_id"] = start.InitialDocument.DocID
	start.Agent.AgentID = "texture:" + start.InitialDocument.DocID
	start.Agent.ChannelID = start.InitialDocument.DocID
	start.StartRequestDigest, _ = ComputeStartLifecycleRequestDigest(start)
	if _, err := s.StartLifecycle(ctx, start); err != nil {
		t.Fatalf("start lifecycle: %v", err)
	}
	queue := queueLifecycleUpdateFixture(t, start, "command-queue-rejection")
	queue.UpdateID = "update-rejection"
	queue.ProducerUpdateID = "producer-update-rejection"
	queue.ProducerAgentID = start.Agent.AgentID
	queue.WorkDisposition = types.WorkItemCompleted
	queue.WorkItemID = start.InitialWork.WorkItemID
	queue.CommandDigest, _ = ComputeQueueLifecycleUpdateDigest(queue)
	if _, err := s.QueueLifecycleUpdate(ctx, queue); err != nil {
		t.Fatalf("queue update: %v", err)
	}
	apply := types.ApplyLifecycleUpdateRequest(queue)
	apply.CommandID = "command-apply-rejection"
	apply.CommandDigest = ""
	apply.Disposition = types.UpdateRejected
	apply.DispositionRef = "evidence://rejection/1"
	apply.Reason = "producer result failed verification"
	apply.WorkDisposition = types.WorkItemRefused
	apply.CommandDigest, _ = ComputeApplyLifecycleUpdateDigest(apply)
	result, err := s.ApplyLifecycleUpdate(ctx, apply)
	if err != nil {
		t.Fatalf("reject update: %v", err)
	}
	if result.WorkItem == nil || result.WorkItem.Status != types.WorkItemRefused || result.WorkItem.ResultRef != apply.DispositionRef {
		t.Fatalf("rejected producer work lacks refusal result: %+v", result.WorkItem)
	}
	for _, event := range result.Events {
		if event.Kind == types.LifecycleWorkSettled {
			t.Fatalf("rejected update emitted successful work settlement: %+v", result.Events)
		}
		if event.Kind == types.LifecycleWorkRefused && (len(event.ArtifactRefs) != 1 || event.ArtifactRefs[0] != apply.DispositionRef) {
			t.Fatalf("work refusal event lacks durable ref: %+v", event)
		}
	}
	snapshot, err := s.GetLifecycleSnapshot(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID)
	if err != nil {
		t.Fatalf("snapshot rejected lifecycle: %v", err)
	}
	if len(snapshot.WorkItems) != 1 || snapshot.WorkItems[0].Status != types.WorkItemRefused || snapshot.Updates[0].Disposition != types.UpdateRejected {
		t.Fatalf("unexpected rejection reconstruction: %+v", snapshot)
	}
}

func TestLifecycleRejectedOpenUpdateKeepsProducerWorkOpen(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	start := lifecycleStartFixture()
	start.CommandID = "command-start-open-rejection"
	start.TrajectoryID = "trajectory-open-rejection"
	start.InitialWork.WorkItemID = "work-open-rejection"
	start.InitialDocument.DocID = "document-open-rejection"
	start.InitialRevision.RevisionID = "revision-open-rejection-v0"
	start.SubjectRefs["doc_id"] = start.InitialDocument.DocID
	start.Agent.AgentID = "texture:" + start.InitialDocument.DocID
	start.Agent.ChannelID = start.InitialDocument.DocID
	start.StartRequestDigest, _ = ComputeStartLifecycleRequestDigest(start)
	if _, err := s.StartLifecycle(ctx, start); err != nil {
		t.Fatalf("start lifecycle: %v", err)
	}
	queue := queueLifecycleUpdateFixture(t, start, "command-queue-open-rejection")
	queue.UpdateID, queue.ProducerUpdateID = "update-open-rejection", "producer-update-open-rejection"
	queue.ProducerAgentID = start.Agent.AgentID
	queue.WorkDisposition, queue.WorkItemID = types.WorkItemOpen, start.InitialWork.WorkItemID
	queue.CommandDigest, _ = ComputeQueueLifecycleUpdateDigest(queue)
	if _, err := s.QueueLifecycleUpdate(ctx, queue); err != nil {
		t.Fatalf("queue open update: %v", err)
	}
	apply := types.ApplyLifecycleUpdateRequest(queue)
	apply.CommandID, apply.CommandDigest = "command-reject-open-update", ""
	apply.Disposition, apply.DispositionRef = types.UpdateRejected, "evidence://rejection/open"
	apply.Reason = "interim evidence was unusable"
	malicious := apply
	malicious.CommandID, malicious.WorkDisposition = "command-refuse-open-work", types.WorkItemRefused
	malicious.CommandDigest, _ = ComputeApplyLifecycleUpdateDigest(malicious)
	if _, err := s.ApplyLifecycleUpdate(ctx, malicious); !errors.Is(err, ErrLifecycleCommandConflict) {
		t.Fatalf("open update accepted refused work consequence: %v", err)
	}
	apply.CommandDigest, _ = ComputeApplyLifecycleUpdateDigest(apply)
	result, err := s.ApplyLifecycleUpdate(ctx, apply)
	if err != nil {
		t.Fatalf("reject open update: %v", err)
	}
	if result.WorkItem != nil {
		t.Fatalf("rejected open update returned a work mutation: %+v", result.WorkItem)
	}
	for _, event := range result.Events {
		if event.Kind == types.LifecycleWorkSettled || event.Kind == types.LifecycleWorkRefused {
			t.Fatalf("rejected open update emitted terminal work event: %+v", result.Events)
		}
	}
	snapshot, err := s.GetLifecycleSnapshot(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID)
	if err != nil {
		t.Fatalf("snapshot rejected open lifecycle: %v", err)
	}
	if len(snapshot.WorkItems) != 1 || snapshot.WorkItems[0].Status != types.WorkItemOpen ||
		snapshot.WorkItems[0].ResultRef != "" || len(snapshot.Updates) != 1 ||
		snapshot.Updates[0].Disposition != types.UpdateRejected {
		t.Fatalf("rejected open update changed work consequence: %+v", snapshot)
	}
}

func TestLifecycleRevisionHeadCASRejectsStaleParent(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	start := lifecycleStartFixture()
	start.CommandID = "command-start-head-cas"
	start.TrajectoryID = "trajectory-head-cas"
	start.InitialWork.WorkItemID = "work-head-cas"
	start.InitialDocument.DocID = "document-head-cas"
	start.InitialRevision.RevisionID = "revision-head-cas-v0"
	start.SubjectRefs["doc_id"] = start.InitialDocument.DocID
	start.Agent.AgentID = "texture:" + start.InitialDocument.DocID
	start.Agent.ChannelID = start.InitialDocument.DocID
	start.StartRequestDigest, _ = ComputeStartLifecycleRequestDigest(start)
	if _, err := s.StartLifecycle(ctx, start); err != nil {
		t.Fatalf("start lifecycle: %v", err)
	}
	queueA := queueLifecycleUpdateFixture(t, start, "command-queue-head-a")
	queueA.UpdateID, queueA.ProducerUpdateID = "update-head-a", "producer-head-a"
	queueA.CommandDigest, _ = ComputeQueueLifecycleUpdateDigest(queueA)
	if _, err := s.QueueLifecycleUpdate(ctx, queueA); err != nil {
		t.Fatalf("queue A: %v", err)
	}
	queueB := queueLifecycleUpdateFixture(t, start, "command-queue-head-b")
	queueB.UpdateID, queueB.ProducerUpdateID = "update-head-b", "producer-head-b"
	queueB.CommandDigest, _ = ComputeQueueLifecycleUpdateDigest(queueB)
	if _, err := s.QueueLifecycleUpdate(ctx, queueB); err != nil {
		t.Fatalf("queue B: %v", err)
	}
	applyA := types.ApplyLifecycleUpdateRequest(queueA)
	applyA.CommandID, applyA.CommandDigest = "command-apply-head-a", ""
	applyA.Disposition = types.UpdateIncorporated
	applyA.Revision = types.Revision{RevisionID: "revision-head-cas-v1", ParentRevisionID: start.InitialRevision.RevisionID, AuthorKind: types.AuthorAppAgent, Content: "first"}
	applyA.DispositionRef = applyA.Revision.RevisionID
	applyA.CommandDigest, _ = ComputeApplyLifecycleUpdateDigest(applyA)
	if _, err := s.ApplyLifecycleUpdate(ctx, applyA); err != nil {
		t.Fatalf("apply A: %v", err)
	}
	applyB := types.ApplyLifecycleUpdateRequest(queueB)
	applyB.CommandID, applyB.CommandDigest = "command-apply-head-b", ""
	applyB.Disposition = types.UpdateIncorporated
	applyB.Revision = types.Revision{RevisionID: "revision-head-cas-v2", ParentRevisionID: start.InitialRevision.RevisionID, AuthorKind: types.AuthorAppAgent, Content: "stale"}
	applyB.DispositionRef = applyB.Revision.RevisionID
	applyB.CommandDigest, _ = ComputeApplyLifecycleUpdateDigest(applyB)
	if _, err := s.ApplyLifecycleUpdate(ctx, applyB); !errors.Is(err, ErrConcurrentStateChange) {
		t.Fatalf("stale parent error = %v, want ErrConcurrentStateChange", err)
	}
	snapshot, err := s.GetLifecycleSnapshot(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID)
	if err != nil || snapshot.HeadRevision.RevisionID != applyA.Revision.RevisionID {
		t.Fatalf("stale parent changed head: %+v, %v", snapshot.HeadRevision, err)
	}
}

func TestLifecycleOpenAmendAndRecordRefs(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	start := lifecycleStartFixture()
	start.StartRequestDigest, _ = ComputeStartLifecycleRequestDigest(start)
	if _, err := s.StartLifecycle(ctx, start); err != nil {
		t.Fatalf("start lifecycle: %v", err)
	}

	open := types.OpenLifecycleWorkRequest{
		OwnerID: start.OwnerID, ComputerID: start.ComputerID,
		CommandID: "command-open-work-2", TrajectoryID: start.TrajectoryID,
		WorkItem: types.WorkItemRecord{
			WorkItemID: "work-lifecycle-2", Objective: "verify artifact",
			AssignedAgentID: start.Agent.AgentID, AuthorityProfile: "texture", StepBudget: 4,
		},
	}
	open.CommandDigest, _ = ComputeOpenLifecycleWorkDigest(open)
	opened, err := s.OpenLifecycleWork(ctx, open)
	if err != nil {
		t.Fatalf("open work: %v", err)
	}
	if opened.WorkItem == nil || opened.WorkItem.Status != types.WorkItemOpen || opened.WorkItem.ObjectiveFingerprint == "" {
		t.Fatalf("unexpected opened work: %+v", opened.WorkItem)
	}

	refuse := types.RefuseLifecycleWorkRequest{
		OwnerID: start.OwnerID, ComputerID: start.ComputerID,
		CommandID: "command-refuse-work-2", TrajectoryID: start.TrajectoryID,
		WorkItemID: open.WorkItem.WorkItemID, Reason: "authority is too narrow", RefusalRef: "refusal://authority/profile-too-narrow",
		ActingAgentID: start.Agent.AgentID,
	}
	wrongActor := refuse
	wrongActor.CommandID = "command-refuse-work-wrong-actor"
	wrongActor.ActingAgentID = "texture:other-document"
	wrongActor.CommandDigest, _ = ComputeRefuseLifecycleWorkDigest(wrongActor)
	if _, err := s.RefuseLifecycleWork(ctx, wrongActor); err == nil {
		t.Fatal("refuse work accepted an agent other than the assigned actor")
	}
	refuse.CommandDigest, _ = ComputeRefuseLifecycleWorkDigest(refuse)
	refused, err := s.RefuseLifecycleWork(ctx, refuse)
	if err != nil {
		t.Fatalf("refuse work: %v", err)
	}
	if refused.WorkItem == nil || refused.WorkItem.ResultRef != refuse.RefusalRef ||
		len(refused.Events) != 1 || len(refused.Events[0].ArtifactRefs) != 1 || refused.Events[0].ArtifactRefs[0] != refuse.RefusalRef {
		t.Fatalf("refused work lacks durable refusal ref: %+v", refused)
	}

	amend := types.AmendLifecycleWorkRequest{
		OwnerID: start.OwnerID, ComputerID: start.ComputerID,
		CommandID: "command-amend-work-2", TrajectoryID: start.TrajectoryID,
		WorkItemID: open.WorkItem.WorkItemID, ExpectedLifecycleVersion: refused.WorkItem.LifecycleVersion,
		WorkItem: types.WorkItemRecord{
			WorkItemID: open.WorkItem.WorkItemID, Objective: "verify artifact with read authority",
			Reason: "resolved refusal by granting read authority", AssignedAgentID: start.Agent.AgentID,
			AuthorityProfile: "reviewer-read", StepBudget: 6,
		},
	}
	amend.CommandDigest, _ = ComputeAmendLifecycleWorkDigest(amend)
	if _, err := s.AmendLifecycleWork(ctx, amend); !errors.Is(err, ErrConcurrentStateChange) {
		t.Fatalf("amend refused work error = %v, want ErrConcurrentStateChange", err)
	}
	replacement := open
	replacement.CommandID = "command-open-work-3"
	replacement.WorkItem.WorkItemID = "work-lifecycle-3"
	replacement.WorkItem.Objective = "verify artifact with read authority"
	replacement.WorkItem.Reason = "follow-up to refused work-lifecycle-2"
	replacement.CommandDigest, _ = ComputeOpenLifecycleWorkDigest(replacement)
	replacementResult, err := s.OpenLifecycleWork(ctx, replacement)
	if err != nil {
		t.Fatalf("open replacement work: %v", err)
	}
	if replacementResult.WorkItem == nil || replacementResult.WorkItem.Status != types.WorkItemOpen {
		t.Fatalf("unexpected replacement work: %+v", replacementResult.WorkItem)
	}

	record := types.RecordLifecycleRefsRequest{
		OwnerID: start.OwnerID, ComputerID: start.ComputerID,
		CommandID: "command-record-refs-1", TrajectoryID: start.TrajectoryID,
		WorkItemID:   replacement.WorkItem.WorkItemID,
		ArtifactRefs: []string{"texture://artifact/review", "texture://artifact/review"},
		EvidenceRefs: []string{"trace://review/1"},
		SubjectRefs:  map[string]string{"review_ref": "trace://review/1"},
	}
	record.CommandDigest, _ = ComputeRecordLifecycleRefsDigest(record)
	recorded, err := s.RecordLifecycleRefs(ctx, record)
	if err != nil {
		t.Fatalf("record refs: %v", err)
	}
	if len(recorded.Events) != 1 || len(recorded.Events[0].ArtifactRefs) != 1 || len(recorded.Events[0].EvidenceRefs) != 1 {
		t.Fatalf("unexpected refs event: %+v", recorded.Events)
	}
	if recorded.Trajectory.SubjectRefs["review_ref"] != "trace://review/1" {
		t.Fatalf("trajectory subject refs not updated: %+v", recorded.Trajectory.SubjectRefs)
	}
}

func TestLifecycleArchiveRetainsHistoryAndRejectsRawMutation(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	start := lifecycleStartFixture()
	start.StartRequestDigest, _ = ComputeStartLifecycleRequestDigest(start)
	started, err := s.StartLifecycle(ctx, start)
	if err != nil {
		t.Fatalf("start lifecycle: %v", err)
	}
	archive := types.ArchiveLifecycleArtifactRequest{
		OwnerID: start.OwnerID, ComputerID: start.ComputerID,
		CommandID: "command-archive-1", TrajectoryID: start.TrajectoryID,
		ExpectedLifecycleVersion: started.Trajectory.LifecycleVersion,
		ExpectedHeadRevisionID:   start.InitialRevision.RevisionID,
		Reason:                   "owner archived completed artifact",
	}
	archive.CommandDigest, _ = ComputeArchiveLifecycleArtifactDigest(archive)
	if _, err := s.ArchiveLifecycleArtifact(ctx, archive); !errors.Is(err, ErrLifecycleInvalidTransition) {
		t.Fatalf("archive live lifecycle error = %v, want ErrLifecycleInvalidTransition", err)
	}
	cancel := types.CancelLifecycleRequest{
		OwnerID: start.OwnerID, ComputerID: start.ComputerID,
		CommandID: "command-cancel-before-archive", TrajectoryID: start.TrajectoryID,
		ExpectedLifecycleVersion: started.Trajectory.LifecycleVersion,
		ExpectedHeadRevisionID:   start.InitialRevision.RevisionID,
		Reason:                   "owner completed lifecycle before archival",
	}
	cancel.CommandDigest, _ = ComputeCancelLifecycleDigest(cancel)
	cancelled, err := s.CancelLifecycleTrajectory(ctx, cancel)
	if err != nil {
		t.Fatalf("cancel before archive: %v", err)
	}
	archive.ExpectedLifecycleVersion = cancelled.Trajectory.LifecycleVersion
	archive.CommandDigest, _ = ComputeArchiveLifecycleArtifactDigest(archive)
	archived, err := s.ArchiveLifecycleArtifact(ctx, archive)
	if err != nil {
		t.Fatalf("archive lifecycle artifact: %v", err)
	}
	if archived.Document == nil || archived.Document.ArchivedAt == nil || archived.Document.CurrentRevisionID != start.InitialRevision.RevisionID {
		t.Fatalf("unexpected archived document: %+v", archived.Document)
	}
	if archived.Trajectory.LifecycleVersion != cancelled.Trajectory.LifecycleVersion ||
		archived.Trajectory.ReducerSeq != cancelled.Trajectory.ReducerSeq ||
		!archived.Trajectory.UpdatedAt.Equal(cancelled.Trajectory.UpdatedAt) {
		t.Fatalf("archive mutated terminal trajectory: before=%+v after=%+v", cancelled.Trajectory, archived.Trajectory)
	}
	replay, err := s.ArchiveLifecycleArtifact(ctx, archive)
	if err != nil || !replay.Replay {
		t.Fatalf("archive replay: %+v, %v", replay, err)
	}
	if err := s.deleteDocumentPhysicalForTest(ctx, start.InitialDocument.DocID, start.OwnerID); !errors.Is(err, ErrLifecycleAuthorityRequired) {
		t.Fatalf("raw lifecycle document delete error = %v, want ErrLifecycleAuthorityRequired", err)
	}
	rawRevision := start.InitialRevision
	rawRevision.RevisionID = "raw-after-archive"
	rawRevision.ParentRevisionID = start.InitialRevision.RevisionID
	rawRevision.TrajectoryID = start.TrajectoryID
	if err := s.CreateRevision(ctx, rawRevision); !errors.Is(err, ErrLifecycleAuthorityRequired) {
		t.Fatalf("raw lifecycle revision error = %v, want ErrLifecycleAuthorityRequired", err)
	}
	snapshot, err := s.GetLifecycleSnapshot(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID)
	if err != nil {
		t.Fatalf("snapshot archived lifecycle: %v", err)
	}
	if snapshot.Document.ArchivedAt == nil || snapshot.HeadRevision.Content != start.InitialRevision.Content {
		t.Fatalf("archival erased or changed retained history: %+v %+v", snapshot.Document, snapshot.HeadRevision)
	}
}

func TestLifecycleReplaceActivationWritesProjectionWithoutAdvancingReducer(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	start := lifecycleStartFixture()
	start.StartRequestDigest, _ = ComputeStartLifecycleRequestDigest(start)
	started, err := s.StartLifecycle(ctx, start)
	if err != nil {
		t.Fatalf("start lifecycle: %v", err)
	}
	now := time.Now().UTC()
	run := types.RunRecord{
		RunID: "run-lifecycle-activation-1", AgentID: started.Agent.AgentID, ChannelID: started.Agent.ChannelID,
		TrajectoryID: start.TrajectoryID, AgentProfile: started.Agent.Profile, AgentRole: started.Agent.Role,
		OwnerID: start.OwnerID, SandboxID: start.ComputerID, State: types.RunPending,
		Prompt: "resume durable work", CreatedAt: now, UpdatedAt: now,
	}
	replace := types.ReplaceLifecycleActivationRequest{
		OwnerID: start.OwnerID, ComputerID: start.ComputerID, CommandID: "command-activation-1",
		TrajectoryID: start.TrajectoryID, AgentID: started.Agent.AgentID, Run: run,
	}
	replace.CommandDigest, _ = ComputeReplaceLifecycleActivationDigest(replace)
	replaced, err := s.ReplaceLifecycleActivation(ctx, replace)
	if err != nil {
		t.Fatalf("replace activation: %v", err)
	}
	if replaced.Agent != nil || replaced.Trajectory.ReducerSeq != started.Trajectory.ReducerSeq {
		t.Fatalf("unexpected activation result: %+v", replaced)
	}
	storedRun, err := s.GetLifecycleRun(ctx, start.OwnerID, start.ComputerID, run.RunID)
	if err != nil || storedRun.State != types.RunPending {
		t.Fatalf("atomic run record = %+v, %v", storedRun, err)
	}
	snapshot, err := s.GetLifecycleSnapshot(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID)
	if err != nil || snapshot.Activation.RunID != run.RunID || snapshot.Activation.State != types.RunPending {
		t.Fatalf("activation snapshot = %+v, %v", snapshot.Activation, err)
	}
	replay, err := s.ReplaceLifecycleActivation(ctx, replace)
	if err != nil || !replay.Replay || replay.Trajectory.ReducerSeq != started.Trajectory.ReducerSeq {
		t.Fatalf("activation replay = %+v, %v", replay, err)
	}
}

func TestLifecycleActivationAdmissionUsesCanonicalAgentCAS(t *testing.T) {
	s := openTestStore(t)
	peer := &Store{ogStore: s.ogStore, ogReadStore: s.ogReadStore}
	ctx := context.Background()
	start := lifecycleStartFixture()
	started, err := s.StartLifecycle(ctx, start)
	if err != nil {
		t.Fatalf("start lifecycle: %v", err)
	}
	newRequest := func(runID string) types.ReplaceLifecycleActivationRequest {
		now := time.Now().UTC()
		run := types.RunRecord{
			RunID: runID, AgentID: started.Agent.AgentID, ChannelID: started.Agent.ChannelID,
			TrajectoryID: start.TrajectoryID, AgentProfile: started.Agent.Profile, AgentRole: started.Agent.Role,
			OwnerID: start.OwnerID, SandboxID: start.ComputerID, State: types.RunPending,
			Prompt: "resume durable work", CreatedAt: now, UpdatedAt: now,
			Metadata: map[string]any{"lifecycle_work_item_id": start.InitialWork.WorkItemID},
		}
		req := types.ReplaceLifecycleActivationRequest{
			OwnerID: start.OwnerID, ComputerID: start.ComputerID,
			CommandID: "command-" + runID, TrajectoryID: start.TrajectoryID,
			AgentID: started.Agent.AgentID, Run: run,
		}
		req.CommandDigest, _ = ComputeReplaceLifecycleActivationDigest(req)
		return req
	}
	requests := []types.ReplaceLifecycleActivationRequest{newRequest("run-activation-cas-a"), newRequest("run-activation-cas-b")}
	type result struct {
		index int
		err   error
	}
	results := make(chan result, len(requests))
	for i, candidate := range requests {
		target := s
		if i == 1 {
			target = peer
		}
		go func(index int, st *Store, req types.ReplaceLifecycleActivationRequest) {
			_, projectErr := st.ReplaceLifecycleActivation(ctx, req)
			results <- result{index: index, err: projectErr}
		}(i, target, candidate)
	}
	first, second := <-results, <-results
	outcomes := []result{first, second}
	winner := -1
	for _, outcome := range outcomes {
		if outcome.err == nil {
			if winner >= 0 {
				t.Fatalf("independent stores admitted duplicate active runs: %+v", outcomes)
			}
			winner = outcome.index
			continue
		}
		if !errors.Is(outcome.err, ErrConcurrentStateChange) && !errors.Is(outcome.err, ErrLifecycleInvalidTransition) {
			t.Fatalf("activation admission error = %v, want canonical conflict", outcome.err)
		}
	}
	if winner < 0 {
		t.Fatalf("no activation won canonical admission: %+v", outcomes)
	}
	loser := 1 - winner
	activeRunID := func(snapshot types.LifecycleSnapshot) string {
		for _, agent := range snapshot.Agents {
			if agent.AgentID == started.Agent.AgentID {
				return agent.ActiveRunID
			}
		}
		return ""
	}
	snapshot, err := s.GetLifecycleSnapshot(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID)
	if err != nil {
		t.Fatalf("load activation snapshot: %v", err)
	}
	if got := activeRunID(snapshot); got != requests[winner].Run.RunID {
		t.Fatalf("agent active_run_id = %q, want %q", got, requests[winner].Run.RunID)
	}

	terminal := requests[winner]
	terminal.CommandID += "-terminal"
	terminal.Run.State = types.RunCompleted
	finishedAt := time.Now().UTC()
	terminal.Run.UpdatedAt, terminal.Run.FinishedAt = finishedAt, &finishedAt
	terminal.CommandDigest, _ = ComputeReplaceLifecycleActivationDigest(terminal)
	if _, err := s.ProjectTerminalLifecycleRun(ctx, terminal); err != nil {
		t.Fatalf("project winning terminal run: %v", err)
	}
	if _, err := peer.ReplaceLifecycleActivation(ctx, requests[loser]); err != nil {
		t.Fatalf("matching terminal projection did not release activation admission: %v", err)
	}
	snapshot, err = s.GetLifecycleSnapshot(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID)
	if err != nil {
		t.Fatalf("reload activation snapshot: %v", err)
	}
	if got := activeRunID(snapshot); got != requests[loser].Run.RunID {
		t.Fatalf("replacement active_run_id = %q, want %q", got, requests[loser].Run.RunID)
	}
}

func TestBlockedLifecycleActivationReleasesCanonicalAdmission(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	start := lifecycleStartFixture()
	started, err := s.StartLifecycle(ctx, start)
	if err != nil {
		t.Fatalf("start lifecycle: %v", err)
	}
	newRequest := func(runID string, state types.RunState) types.ReplaceLifecycleActivationRequest {
		now := time.Now().UTC()
		run := types.RunRecord{
			RunID: runID, AgentID: started.Agent.AgentID, ChannelID: started.Agent.ChannelID,
			TrajectoryID: start.TrajectoryID, AgentProfile: started.Agent.Profile, AgentRole: started.Agent.Role,
			OwnerID: start.OwnerID, SandboxID: start.ComputerID, State: state,
			Prompt: "resume durable work", CreatedAt: now, UpdatedAt: now,
			Metadata: map[string]any{"lifecycle_work_item_id": start.InitialWork.WorkItemID},
		}
		req := types.ReplaceLifecycleActivationRequest{
			OwnerID: start.OwnerID, ComputerID: start.ComputerID,
			CommandID: "command-" + runID + "-" + string(state), TrajectoryID: start.TrajectoryID,
			AgentID: started.Agent.AgentID, Run: run,
		}
		req.CommandDigest, _ = ComputeReplaceLifecycleActivationDigest(req)
		return req
	}
	active := newRequest("run-blocked-release", types.RunPending)
	if _, err := s.ReplaceLifecycleActivation(ctx, active); err != nil {
		t.Fatalf("activate run: %v", err)
	}
	blocked := newRequest(active.Run.RunID, types.RunBlocked)
	blocked.Run.CreatedAt = active.Run.CreatedAt
	blocked.CommandDigest, _ = ComputeReplaceLifecycleActivationDigest(blocked)
	if _, err := s.ReplaceLifecycleActivation(ctx, blocked); err != nil {
		t.Fatalf("project blocked run: %v", err)
	}
	replacement := newRequest("run-after-blocked-release", types.RunPending)
	if _, err := s.ReplaceLifecycleActivation(ctx, replacement); err != nil {
		t.Fatalf("blocked activation retained canonical admission: %v", err)
	}
	snapshot, err := s.GetLifecycleSnapshot(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID)
	if err != nil {
		t.Fatalf("load activation snapshot: %v", err)
	}
	for _, agent := range snapshot.Agents {
		if agent.AgentID == started.Agent.AgentID && agent.ActiveRunID != replacement.Run.RunID {
			t.Fatalf("replacement active_run_id = %q, want %q", agent.ActiveRunID, replacement.Run.RunID)
		}
	}
}

func TestBlockedLifecycleProjectionAfterSettlementReleasesCanonicalAdmission(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	start := lifecycleStartFixture()
	started, err := s.StartLifecycle(ctx, start)
	if err != nil {
		t.Fatalf("start lifecycle: %v", err)
	}
	now := time.Now().UTC()
	active := types.ReplaceLifecycleActivationRequest{
		OwnerID: start.OwnerID, ComputerID: start.ComputerID,
		CommandID: "command-settled-blocked-running", TrajectoryID: start.TrajectoryID,
		AgentID: started.Agent.AgentID,
		Run: types.RunRecord{
			RunID: "run-settled-blocked", AgentID: started.Agent.AgentID, ChannelID: started.Agent.ChannelID,
			TrajectoryID: start.TrajectoryID, AgentProfile: started.Agent.Profile, AgentRole: started.Agent.Role,
			OwnerID: start.OwnerID, SandboxID: start.ComputerID, State: types.RunRunning,
			Prompt: "finish durable work", CreatedAt: now, UpdatedAt: now,
			Metadata: map[string]any{"lifecycle_work_item_id": start.InitialWork.WorkItemID},
		},
	}
	active.CommandDigest, _ = ComputeReplaceLifecycleActivationDigest(active)
	if _, err := s.ReplaceLifecycleActivation(ctx, active); err != nil {
		t.Fatalf("activate running lifecycle: %v", err)
	}
	settleWork := types.SettleLifecycleWorkRequest{
		OwnerID: start.OwnerID, ComputerID: start.ComputerID,
		CommandID: "command-settled-blocked-work", TrajectoryID: start.TrajectoryID,
		WorkItemID: start.InitialWork.WorkItemID, ResultRef: "texture://settled-blocked/result",
		ActingAgentID: started.Agent.AgentID,
	}
	settleWork.CommandDigest, _ = ComputeSettleLifecycleWorkDigest(settleWork)
	settledWork, err := s.SettleLifecycleWork(ctx, settleWork)
	if err != nil {
		t.Fatalf("settle lifecycle work: %v", err)
	}
	settleTrajectory := types.SettleLifecycleTrajectoryRequest{
		OwnerID: start.OwnerID, ComputerID: start.ComputerID,
		CommandID: "command-settled-blocked-trajectory", TrajectoryID: start.TrajectoryID,
		ExpectedLifecycleVersion: settledWork.Trajectory.LifecycleVersion,
		ExpectedHeadRevisionID:   start.InitialRevision.RevisionID,
	}
	settleTrajectory.CommandDigest, _ = ComputeSettleLifecycleTrajectoryDigest(settleTrajectory)
	if _, err := s.SettleLifecycleTrajectory(ctx, settleTrajectory); err != nil {
		t.Fatalf("settle lifecycle trajectory: %v", err)
	}
	blocked := active
	blocked.CommandID = "command-settled-blocked-outcome"
	blocked.Run.State = types.RunBlocked
	blocked.Run.UpdatedAt = now.Add(time.Second)
	blocked.CommandDigest, _ = ComputeReplaceLifecycleActivationDigest(blocked)
	if _, err := s.ProjectTerminalLifecycleRun(ctx, blocked); err != nil {
		t.Fatalf("project blocked run after settlement: %v", err)
	}
	snapshot, err := s.GetLifecycleSnapshot(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID)
	if err != nil {
		t.Fatalf("load settled blocked snapshot: %v", err)
	}
	if snapshot.Activation.State != types.RunBlocked {
		t.Fatalf("settled blocked activation = %+v, want blocked", snapshot.Activation)
	}
	for _, agent := range snapshot.Agents {
		if agent.AgentID == started.Agent.AgentID && agent.ActiveRunID != "" {
			t.Fatalf("settled blocked active_run_id = %q, want released", agent.ActiveRunID)
		}
	}
}

func TestLifecycleActiveRunIDIsReducerOwnedAcrossOtherAgentWriters(t *testing.T) {
	t.Run("generic upsert preserves canonical activation", func(t *testing.T) {
		s := openTestStore(t)
		ctx := context.Background()
		start := lifecycleStartFixture()
		started, err := s.StartLifecycle(ctx, start)
		if err != nil {
			t.Fatalf("start lifecycle: %v", err)
		}
		const agentID = "researcher-active-run-authority"
		const channelID = "researcher-active-run-channel"
		const workItemID = "work-active-run-authority"
		agent := types.AgentRecord{
			AgentID: agentID, OwnerID: start.OwnerID, ComputerID: start.ComputerID, SandboxID: start.ComputerID,
			Profile: "researcher", Role: "researcher", ChannelID: channelID,
		}
		if err := s.UpsertAgent(ctx, agent); err != nil {
			t.Fatalf("seed generic agent: %v", err)
		}
		open := types.OpenLifecycleWorkRequest{
			OwnerID: start.OwnerID, ComputerID: start.ComputerID,
			CommandID: "command-open-active-run-authority", TrajectoryID: start.TrajectoryID,
			WorkItem: types.WorkItemRecord{
				WorkItemID: workItemID, Objective: "prove reducer-owned activation",
				AssignedAgentID: agentID, AuthorityProfile: "researcher",
			},
		}
		open.CommandDigest, _ = ComputeOpenLifecycleWorkDigest(open)
		if _, err := s.OpenLifecycleWork(ctx, open); err != nil {
			t.Fatalf("open researcher work: %v", err)
		}
		now := time.Now().UTC()
		run := types.RunRecord{
			RunID: "run-active-run-authority", AgentID: agentID, ChannelID: channelID,
			TrajectoryID: start.TrajectoryID, AgentProfile: "researcher", AgentRole: "researcher",
			OwnerID: start.OwnerID, SandboxID: start.ComputerID, State: types.RunPending,
			Prompt: "resume durable research", CreatedAt: now, UpdatedAt: now,
			Metadata: map[string]any{"lifecycle_work_item_id": workItemID},
		}
		activate := types.ReplaceLifecycleActivationRequest{
			OwnerID: start.OwnerID, ComputerID: start.ComputerID,
			CommandID: "command-active-run-authority", TrajectoryID: start.TrajectoryID,
			AgentID: agentID, Run: run,
		}
		activate.CommandDigest, _ = ComputeReplaceLifecycleActivationDigest(activate)
		if _, err := s.ReplaceLifecycleActivation(ctx, activate); err != nil {
			t.Fatalf("activate researcher: %v", err)
		}
		for name, mutate := range map[string]func(*types.AgentRecord){
			"profile": func(candidate *types.AgentRecord) { candidate.Profile = "processor" },
			"role":    func(candidate *types.AgentRecord) { candidate.Role = "processor" },
			"channel": func(candidate *types.AgentRecord) { candidate.ChannelID = "other-channel" },
		} {
			t.Run("reject active "+name+" mutation", func(t *testing.T) {
				candidate := agent
				mutate(&candidate)
				if err := s.UpsertAgent(ctx, candidate); !errors.Is(err, ErrLifecycleAuthorityRequired) {
					t.Fatalf("active %s mutation error = %v, want ErrLifecycleAuthorityRequired", name, err)
				}
			})
		}
		agent.UpdatedAt = now.Add(time.Second)
		if err := s.UpsertAgent(ctx, agent); err != nil {
			t.Fatalf("generic upsert while active: %v", err)
		}
		stored, err := s.GetAgentByScope(ctx, start.OwnerID, start.ComputerID, agentID)
		if err != nil || stored.ActiveRunID != run.RunID {
			t.Fatalf("generic upsert active_run_id = %q, %v; want %q", stored.ActiveRunID, err, run.RunID)
		}
		terminal := activate
		terminal.CommandID = "command-active-run-authority-terminal"
		terminal.Run.State = types.RunCompleted
		finishedAt := now.Add(2 * time.Second)
		terminal.Run.UpdatedAt, terminal.Run.FinishedAt = finishedAt, &finishedAt
		terminal.CommandDigest, _ = ComputeReplaceLifecycleActivationDigest(terminal)
		agent.UpdatedAt = now.Add(3 * time.Second)
		peer := &Store{ogStore: s.ogStore, ogReadStore: s.ogReadStore}
		startRace := make(chan struct{})
		results := make(chan error, 2)
		go func() {
			<-startRace
			_, projectErr := s.ProjectTerminalLifecycleRun(ctx, terminal)
			results <- projectErr
		}()
		go func() {
			<-startRace
			results <- peer.UpsertAgent(ctx, agent)
		}()
		close(startRace)
		for range 2 {
			if err := <-results; err != nil {
				t.Fatalf("terminal/upsert race: %v", err)
			}
		}
		stored, err = s.GetAgentByScope(ctx, start.OwnerID, start.ComputerID, agentID)
		if err != nil || stored.ActiveRunID != "" {
			t.Fatalf("terminal/upsert race active_run_id = %q, %v; want released", stored.ActiveRunID, err)
		}
		agent.ActiveRunID = "caller-injected-run"
		if err := s.UpsertAgent(ctx, agent); !errors.Is(err, ErrLifecycleAuthorityRequired) {
			t.Fatalf("caller active_run_id injection error = %v, want ErrLifecycleAuthorityRequired", err)
		}
		if started.Agent == nil {
			t.Fatal("started lifecycle omitted agent")
		}
	})

	t.Run("lifecycle start preserves existing activation and rejects injection", func(t *testing.T) {
		s := openTestStore(t)
		ctx := context.Background()
		start := lifecycleStartFixture()
		now := time.Now().UTC()
		storedAgent := start.Agent
		storedAgent.OwnerID, storedAgent.ComputerID, storedAgent.SandboxID = start.OwnerID, start.ComputerID, start.ComputerID
		storedAgent.ActiveRunID = "run-start-preserves-active"
		storedAgent.CreatedAt, storedAgent.UpdatedAt = now, now
		agentObj, err := lifecycleObject(
			ogKindAgent, start.OwnerID, start.ComputerID, storedAgent.AgentID, storedAgent,
			lifecycleMetadata("agent_id", storedAgent.AgentID, start.ComputerID, start.TrajectoryID, 0), now, now,
		)
		if err != nil {
			t.Fatalf("build existing active agent: %v", err)
		}
		if err := s.ogStore.PutBatchConditional(ctx,
			[]objectgraph.ObjectCondition{{CanonicalID: agentObj.CanonicalID}},
			objectgraph.Batch{Objects: []objectgraph.Object{agentObj}},
		); err != nil {
			t.Fatalf("seed existing active agent: %v", err)
		}
		if _, err := s.StartLifecycle(ctx, start); err != nil {
			t.Fatalf("start lifecycle with existing active agent: %v", err)
		}
		stored, err := s.GetAgentByScope(ctx, start.OwnerID, start.ComputerID, storedAgent.AgentID)
		if err != nil || stored.ActiveRunID != storedAgent.ActiveRunID {
			t.Fatalf("lifecycle start active_run_id = %q, %v; want %q", stored.ActiveRunID, err, storedAgent.ActiveRunID)
		}
		injected := lifecycleStartFixture()
		injected.CommandID = "command-start-injected-active"
		injected.TrajectoryID = "trajectory-start-injected-active"
		injected.InitialWork.WorkItemID = "work-start-injected-active"
		injected.InitialDocument.DocID = "document-start-injected-active"
		injected.InitialRevision.RevisionID = "revision-start-injected-active-v0"
		injected.InitialRevision.DocID = injected.InitialDocument.DocID
		injected.Agent.ActiveRunID = "caller-injected-run"
		injected.StartRequestDigest, _ = ComputeStartLifecycleRequestDigest(injected)
		if _, err := s.StartLifecycle(ctx, injected); !errors.Is(err, ErrLifecycleInvalidTransition) {
			t.Fatalf("injected lifecycle start error = %v, want ErrLifecycleInvalidTransition", err)
		}
	})
}

func TestLifecycleActivationAdmissionRequiresCurrentOpenWork(t *testing.T) {
	for _, tc := range []struct {
		name    string
		prepare func(*testing.T, *Store, types.StartLifecycleRequest, types.LifecycleResult, string)
		wantErr bool
	}{
		{
			name: "settled",
			prepare: func(t *testing.T, s *Store, start types.StartLifecycleRequest, started types.LifecycleResult, workItemID string) {
				settle := types.SettleLifecycleWorkRequest{
					OwnerID: start.OwnerID, ComputerID: start.ComputerID,
					CommandID: "command-settle-activation-admission", TrajectoryID: start.TrajectoryID,
					WorkItemID: workItemID, ResultRef: "artifact://activation-admission/settled",
					ActingAgentID: started.Agent.AgentID,
				}
				settle.CommandDigest, _ = ComputeSettleLifecycleWorkDigest(settle)
				if _, err := s.SettleLifecycleWork(context.Background(), settle); err != nil {
					t.Fatalf("settle bound work: %v", err)
				}
			},
			wantErr: true,
		},
		{
			name: "terminal_pending",
			prepare: func(t *testing.T, s *Store, start types.StartLifecycleRequest, started types.LifecycleResult, workItemID string) {
				queue := queueLifecycleUpdateFixture(t, start, "command-queue-activation-terminal-pending")
				queue.ProducerAgentID = started.Agent.AgentID
				queue.ProducerUpdateID = "producer-update-activation-terminal-pending"
				queue.UpdateID = "update-activation-terminal-pending"
				queue.WorkItemID, queue.WorkDisposition = workItemID, types.WorkItemCompleted
				queue.CommandDigest, _ = ComputeQueueLifecycleUpdateDigest(queue)
				if _, err := s.QueueLifecycleUpdate(context.Background(), queue); err != nil {
					t.Fatalf("queue terminal work disposition: %v", err)
				}
			},
			wantErr: true,
		},
		{name: "open"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := openTestStore(t)
			ctx := context.Background()
			start := lifecycleStartFixture()
			start.CommandID = "command-start-activation-work-admission-" + tc.name
			start.TrajectoryID = "trajectory-activation-work-admission-" + tc.name
			start.InitialWork.WorkItemID = "work-activation-work-admission-root-" + tc.name
			start.InitialDocument.DocID = "document-activation-work-admission-" + tc.name
			start.InitialRevision.RevisionID = "revision-activation-work-admission-" + tc.name
			start.Agent.AgentID = "texture:" + start.InitialDocument.DocID
			start.Agent.ChannelID = start.InitialDocument.DocID
			start.SubjectRefs["doc_id"] = start.InitialDocument.DocID
			start.SubjectRefs["artifact"] = "texture://artifact/activation-work-admission-" + tc.name
			start.StartRequestDigest, _ = ComputeStartLifecycleRequestDigest(start)
			started, err := s.StartLifecycle(ctx, start)
			if err != nil {
				t.Fatalf("start lifecycle: %v", err)
			}
			workItemID := "work-activation-admission-" + tc.name
			open := types.OpenLifecycleWorkRequest{
				OwnerID: start.OwnerID, ComputerID: start.ComputerID,
				CommandID: "command-open-" + workItemID, TrajectoryID: start.TrajectoryID,
				WorkItem: types.WorkItemRecord{
					WorkItemID: workItemID, Objective: "admit only current open work",
					AssignedAgentID: started.Agent.AgentID, AuthorityProfile: started.Agent.Profile,
				},
			}
			open.CommandDigest, _ = ComputeOpenLifecycleWorkDigest(open)
			if _, err := s.OpenLifecycleWork(ctx, open); err != nil {
				t.Fatalf("open lifecycle work: %v", err)
			}
			if tc.prepare != nil {
				tc.prepare(t, s, start, started, workItemID)
			}
			snapshot, err := s.GetLifecycleSnapshot(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID)
			if err != nil || snapshot.Trajectory.Status != types.TrajectoryLive {
				t.Fatalf("activation admission prerequisite trajectory = %+v, %v", snapshot.Trajectory, err)
			}
			now := time.Now().UTC()
			run := types.RunRecord{
				RunID:   "run-activation-admission-" + tc.name,
				AgentID: started.Agent.AgentID, ChannelID: started.Agent.ChannelID,
				TrajectoryID: start.TrajectoryID, AgentProfile: started.Agent.Profile, AgentRole: started.Agent.Role,
				OwnerID: start.OwnerID, SandboxID: start.ComputerID, State: types.RunPending,
				Prompt: "resume bound lifecycle work", CreatedAt: now, UpdatedAt: now,
				Metadata: map[string]any{
					"lifecycle_work_item_id": workItemID,
					"work_item_ids":          []string{workItemID},
				},
			}
			replace := types.ReplaceLifecycleActivationRequest{
				OwnerID: start.OwnerID, ComputerID: start.ComputerID,
				CommandID:    "command-project-activation-admission-" + tc.name,
				TrajectoryID: start.TrajectoryID, AgentID: started.Agent.AgentID, Run: run,
			}
			replace.CommandDigest, _ = ComputeReplaceLifecycleActivationDigest(replace)
			_, err = s.ReplaceLifecycleActivation(ctx, replace)
			if tc.wantErr {
				if !errors.Is(err, ErrLifecycleInvalidTransition) {
					t.Fatalf("activation error = %v, want ErrLifecycleInvalidTransition", err)
				}
			} else if err != nil {
				t.Fatalf("current open-work activation: %v", err)
			}
		})
	}
}

func TestLifecycleRunProjectionIsComputerScoped(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	const runID = "run-same-across-computers"
	for _, computerID := range []string{"computer-a", "computer-b"} {
		start := lifecycleStartFixture()
		start.ComputerID = computerID
		start.StartRequestDigest, _ = ComputeStartLifecycleRequestDigest(start)
		started, err := s.StartLifecycle(ctx, start)
		if err != nil {
			t.Fatalf("start lifecycle on %s: %v", computerID, err)
		}
		now := time.Now().UTC()
		run := types.RunRecord{
			RunID: runID, AgentID: started.Agent.AgentID, ChannelID: started.Agent.ChannelID,
			TrajectoryID: start.TrajectoryID, AgentProfile: started.Agent.Profile, AgentRole: started.Agent.Role,
			OwnerID: start.OwnerID, SandboxID: computerID, State: types.RunPending,
			Prompt: "resume scoped work", CreatedAt: now, UpdatedAt: now,
		}
		replace := types.ReplaceLifecycleActivationRequest{
			OwnerID: start.OwnerID, ComputerID: computerID, CommandID: "activate-" + computerID,
			TrajectoryID: start.TrajectoryID, AgentID: started.Agent.AgentID, Run: run,
		}
		replace.CommandDigest, _ = ComputeReplaceLifecycleActivationDigest(replace)
		if _, err := s.ReplaceLifecycleActivation(ctx, replace); err != nil {
			t.Fatalf("project lifecycle run on %s: %v", computerID, err)
		}
	}
	for _, computerID := range []string{"computer-a", "computer-b"} {
		run, err := s.GetLifecycleRun(ctx, "owner-lifecycle", computerID, runID)
		if err != nil || run.SandboxID != computerID {
			t.Fatalf("scoped run on %s = %+v, %v", computerID, run, err)
		}
	}
	if _, err := s.GetRunByOwner(ctx, "owner-lifecycle", runID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("owner-only run lookup exposed lifecycle projection: %v", err)
	}
	if _, err := s.GetRun(ctx, runID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("global run lookup exposed lifecycle projection: %v", err)
	}
	if runs, err := s.ListRunsByOwner(ctx, "owner-lifecycle", 100); err != nil || len(runs) != 0 {
		t.Fatalf("owner run list exposed lifecycle projections: %+v, %v", runs, err)
	}
	if runs, err := s.ListActiveRunsByTrajectory(ctx, "owner-lifecycle", lifecycleStartFixture().TrajectoryID, 0); err != nil || len(runs) != 0 {
		t.Fatalf("trajectory run list exposed lifecycle projections: %+v, %v", runs, err)
	}
	if runs, err := s.ListRunsByState(ctx, types.RunPending, 100); err != nil || len(runs) != 0 {
		t.Fatalf("state run list exposed lifecycle projections: %+v, %v", runs, err)
	}
	if _, err := s.GetLatestActiveRunByAgent(ctx, "owner-lifecycle", lifecycleStartFixture().Agent.AgentID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("legacy active selector exposed lifecycle projection: %v", err)
	}
}

func TestLifecycleSourceGraphIsComputerScoped(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	const (
		ownerID    = "owner-source-collision"
		docID      = "doc-source-collision"
		revisionID = "revision-source-collision"
	)
	canonicalByComputer := map[string]string{}
	for _, computerID := range []string{"computer-a", "computer-b"} {
		ownerScope := ownerID + "\x00" + computerID
		entityCanonicalID, err := BuildTextureSourceEntityCanonicalID(ownerID, ownerScope, "web_url", "https://example.com/same-source")
		if err != nil {
			t.Fatal(err)
		}
		refCanonicalID, err := BuildTextureSourceRefCanonicalIDByScope(ownerID, computerID, revisionID, "doc/source-ref")
		if err != nil {
			t.Fatal(err)
		}
		entityMetadata := json.RawMessage(fmt.Sprintf(`{"computer_id":%q,"source_kind":"web_url","target":{"identity":"https://example.com/same-source"},"texture_doc_id":%q,"texture_revision_id":%q}`, computerID, docID, revisionID))
		refMetadata := json.RawMessage(fmt.Sprintf(`{"computer_id":%q,"identity_key":"doc/source-ref","doc_id":%q,"texture_revision_id":%q}`, computerID, docID, revisionID))
		graph := TextureSourceGraphWriteSet{
			SourceEntities: []TextureSourceEntityGraphRecord{{
				CanonicalID: entityCanonicalID, OwnerID: ownerID, ComputerID: computerID,
				Body: []byte("same evidence"), Metadata: entityMetadata, LegacySourceEntityID: "source-same",
			}},
			SourceRefs: []TextureSourceRefGraphRecord{{
				CanonicalID: refCanonicalID, OwnerID: ownerID, ComputerID: computerID,
				DocID: docID, TextureRevisionID: revisionID, BodyNodeID: "source-ref",
				SourceEntityCanonicalID: entityCanonicalID, DisplayMode: TextureSourceRefDisplayNumbered,
				CitationState: "cited", Metadata: refMetadata,
			}},
		}
		entityVersion, _, _, err := TextureSourceGraphVersionID(TextureSourceEntityObjectKind, graph.SourceEntities[0].Body, graph.SourceEntities[0].Metadata)
		if err != nil {
			t.Fatal(err)
		}
		graph.SourceRefs[0].SourceEntityVersionID = entityVersion
		if computerID == "computer-a" {
			ownerOnlyEntityID, buildErr := BuildTextureSourceEntityCanonicalID(ownerID, ownerID, "web_url", "https://example.com/same-source")
			if buildErr != nil {
				t.Fatal(buildErr)
			}
			badEntityGraph := graph
			badEntityGraph.SourceEntities = append([]TextureSourceEntityGraphRecord(nil), graph.SourceEntities...)
			badEntityGraph.SourceRefs = append([]TextureSourceRefGraphRecord(nil), graph.SourceRefs...)
			badEntityGraph.SourceEntities[0].CanonicalID = ownerOnlyEntityID
			badEntityGraph.SourceRefs[0].SourceEntityCanonicalID = ownerOnlyEntityID
			if _, _, err := s.lifecycleSourceGraphBatch(ctx, types.Revision{
				RevisionID: revisionID, DocID: docID, OwnerID: ownerID, ComputerID: computerID, CreatedAt: time.Now().UTC(),
			}, badEntityGraph, time.Now().UTC()); err == nil {
				t.Fatal("lifecycle source graph accepted owner-only source entity canonical ID")
			}
			ownerOnlyRefID, buildErr := BuildTextureSourceRefCanonicalID(ownerID, revisionID, "doc/source-ref")
			if buildErr != nil {
				t.Fatal(buildErr)
			}
			badRefGraph := graph
			badRefGraph.SourceEntities = append([]TextureSourceEntityGraphRecord(nil), graph.SourceEntities...)
			badRefGraph.SourceRefs = append([]TextureSourceRefGraphRecord(nil), graph.SourceRefs...)
			badRefGraph.SourceRefs[0].CanonicalID = ownerOnlyRefID
			if _, _, err := s.lifecycleSourceGraphBatch(ctx, types.Revision{
				RevisionID: revisionID, DocID: docID, OwnerID: ownerID, ComputerID: computerID, CreatedAt: time.Now().UTC(),
			}, badRefGraph, time.Now().UTC()); err == nil {
				t.Fatal("lifecycle source graph accepted owner-only source ref canonical ID")
			}
		}
		revision := types.Revision{
			RevisionID: revisionID, DocID: docID, OwnerID: ownerID, ComputerID: computerID,
			CreatedAt: time.Now().UTC(),
		}
		objects, conditions, err := s.lifecycleSourceGraphBatch(ctx, revision, graph, revision.CreatedAt)
		if err != nil {
			t.Fatalf("build %s source graph: %v", computerID, err)
		}
		if err := s.ogStore.PutBatchConditional(ctx, conditions, objectgraph.Batch{Objects: objects}); err != nil {
			t.Fatalf("write %s source graph: %v", computerID, err)
		}
		entities, err := s.ListTextureSourceEntitiesForRevisionByScope(ctx, ownerID, computerID, docID, revisionID)
		if err != nil || len(entities) != 1 || entities[0].ComputerID != computerID {
			t.Fatalf("%s scoped source entities = %+v, %v", computerID, entities, err)
		}
		refs, err := s.ListTextureSourceRefsForRevisionByScope(ctx, ownerID, computerID, docID, revisionID)
		if err != nil || len(refs) != 1 || refs[0].ComputerID != computerID ||
			refs[0].SourceEntityCanonicalID != entities[0].CanonicalID {
			t.Fatalf("%s scoped source refs = %+v, %v", computerID, refs, err)
		}
		if _, err := s.GetTextureSourceEntityOG(ctx, entityCanonicalID, entityVersion); !errors.Is(err, ErrNotFound) {
			t.Fatalf("legacy direct source entity getter exposed %s lifecycle record: %v", computerID, err)
		}
		if exists, err := s.TextureSourceEntityVersionExistsOG(ctx, entityCanonicalID, entityVersion); err != nil || exists {
			t.Fatalf("legacy source entity existence exposed %s lifecycle record: %v, %v", computerID, exists, err)
		}
		if exists, err := s.TextureSourceRefVersionExistsOG(ctx, refCanonicalID, refs[0].VersionID); err != nil || exists {
			t.Fatalf("legacy source ref existence exposed %s lifecycle record: %v, %v", computerID, exists, err)
		}
		canonicalByComputer[computerID] = entities[0].CanonicalID
	}
	if canonicalByComputer["computer-a"] == canonicalByComputer["computer-b"] {
		t.Fatalf("source canonical IDs collided across computers: %q", canonicalByComputer["computer-a"])
	}
	if entities, err := s.ListTextureSourceEntitiesForRevision(ctx, ownerID, docID, revisionID); err != nil || len(entities) != 0 {
		t.Fatalf("legacy source entity read exposed lifecycle graph: %+v, %v", entities, err)
	}
	if refs, err := s.ListTextureSourceRefsForRevision(ctx, ownerID, docID, revisionID); err != nil || len(refs) != 0 {
		t.Fatalf("legacy source ref read exposed lifecycle graph: %+v, %v", refs, err)
	}
}

func TestStartLifecycleRejectsCallerSuppliedRevisionHashMismatch(t *testing.T) {
	s := openTestStore(t)
	req := lifecycleStartFixture()
	req.InitialRevision.RevisionHash = "sha256:caller-mismatch"
	req.StartRequestDigest, _ = ComputeStartLifecycleRequestDigest(req)
	if _, err := s.StartLifecycle(context.Background(), req); err == nil {
		t.Fatal("lifecycle start accepted mismatched initial revision hash")
	}
}

func TestLifecycleRejectsEffectsCapableSubjectAndAssignment(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	superStart := lifecycleStartFixture()
	superStart.Agent.AgentID = "super:forbidden"
	superStart.Agent.Profile, superStart.Agent.Role = "super", "super"
	superStart.InitialWork.AssignedAgentID = superStart.Agent.AgentID
	superStart.StartRequestDigest, _ = ComputeStartLifecycleRequestDigest(superStart)
	if _, err := s.StartLifecycle(ctx, superStart); !errors.Is(err, ErrLifecycleInvalidTransition) {
		t.Fatalf("effects-capable lifecycle start error = %v, want ErrLifecycleInvalidTransition", err)
	}

	start := lifecycleStartFixture()
	start.StartRequestDigest, _ = ComputeStartLifecycleRequestDigest(start)
	if _, err := s.StartLifecycle(ctx, start); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := s.UpsertAgent(ctx, types.AgentRecord{
		AgentID: "super:forbidden", OwnerID: start.OwnerID, ComputerID: start.ComputerID,
		SandboxID: start.ComputerID, Profile: "super", Role: "super", CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	open := types.OpenLifecycleWorkRequest{
		OwnerID: start.OwnerID, ComputerID: start.ComputerID, CommandID: "command-open-forbidden",
		TrajectoryID: start.TrajectoryID,
		WorkItem: types.WorkItemRecord{
			WorkItemID: "work-forbidden", Objective: "must not run",
			AuthorityProfile: "super", AssignedAgentID: "super:forbidden",
		},
	}
	open.CommandDigest, _ = ComputeOpenLifecycleWorkDigest(open)
	if _, err := s.OpenLifecycleWork(ctx, open); !errors.Is(err, ErrLifecycleInvalidTransition) {
		t.Fatalf("effects-capable assignment error = %v, want ErrLifecycleInvalidTransition", err)
	}
}

func TestListLifecycleSubjectsIsComputerScoped(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	first := lifecycleStartFixture()
	first.StartRequestDigest, _ = ComputeStartLifecycleRequestDigest(first)
	if _, err := s.StartLifecycle(ctx, first); err != nil {
		t.Fatal(err)
	}
	second := lifecycleStartFixture()
	second.ComputerID = "computer-lifecycle-other"
	second.CommandID = "command-start-other"
	second.TrajectoryID = "trajectory-lifecycle-other"
	second.InitialWork.WorkItemID = "work-lifecycle-other"
	second.InitialDocument.DocID = "document-lifecycle-other"
	second.InitialRevision.DocID = second.InitialDocument.DocID
	second.InitialRevision.RevisionID = "revision-lifecycle-other"
	second.Agent.AgentID = "texture:document-lifecycle-other"
	second.Agent.ChannelID = second.InitialDocument.DocID
	second.StartRequestDigest, _ = ComputeStartLifecycleRequestDigest(second)
	if _, err := s.StartLifecycle(ctx, second); err != nil {
		t.Fatal(err)
	}
	subjects, err := s.ListLifecycleSubjects(ctx, first.ComputerID)
	if err != nil {
		t.Fatal(err)
	}
	if len(subjects) != 1 || subjects[0].ComputerID != first.ComputerID || subjects[0].AgentID != first.Agent.AgentID {
		t.Fatalf("computer-scoped subjects = %+v", subjects)
	}
}

func TestLifecycleLateUpdatesUseCASLinearizedSequenceAcrossStores(t *testing.T) {
	path := testStorePath(t)
	cleanupTestStorePath(path)
	t.Cleanup(func() { cleanupTestStorePath(path) })
	ctx := context.Background()
	first, err := Open(path)
	if err != nil {
		t.Fatalf("open first store: %v", err)
	}
	t.Cleanup(func() { _ = first.Close() })

	start := lifecycleStartFixture()
	start.CommandID = "command-start-late-race"
	start.TrajectoryID = "trajectory-late-race"
	start.InitialWork.WorkItemID = "work-late-race"
	start.InitialDocument.DocID = "document-late-race"
	start.InitialRevision.RevisionID = "revision-late-race"
	start.Agent.AgentID = "texture:document-late-race"
	start.Agent.ChannelID = "document-late-race"
	start.SubjectRefs["artifact"] = "texture://document-late-race"
	start.SubjectRefs["doc_id"] = start.InitialDocument.DocID
	start.StartRequestDigest, _ = ComputeStartLifecycleRequestDigest(start)
	started, err := first.StartLifecycle(ctx, start)
	if err != nil {
		t.Fatalf("start lifecycle: %v", err)
	}
	cancel := types.CancelLifecycleRequest{
		OwnerID: start.OwnerID, ComputerID: start.ComputerID, CommandID: "command-cancel-late-race",
		TrajectoryID: start.TrajectoryID, ExpectedLifecycleVersion: started.Trajectory.LifecycleVersion,
		ExpectedHeadRevisionID: started.Revision.RevisionID,
		Reason:                 "exercise terminal append sequence",
	}
	cancel.CommandDigest, _ = ComputeCancelLifecycleDigest(cancel)
	if _, err := first.CancelLifecycleTrajectory(ctx, cancel); err != nil {
		t.Fatalf("cancel lifecycle: %v", err)
	}
	second := &Store{ogStore: first.ogStore, ogReadStore: first.ogReadStore}

	requests := []types.QueueLifecycleUpdateRequest{
		queueLifecycleUpdateFixture(t, start, "command-late-race-a"),
		queueLifecycleUpdateFixture(t, start, "command-late-race-b"),
	}
	for i := 0; i < len(requests); i++ {
		requests[i].UpdateID = "update-late-race-" + string(rune('a'+i))
		requests[i].ProducerUpdateID = "producer-late-race-" + string(rune('a'+i))
		requests[i].CommandDigest, _ = ComputeQueueLifecycleUpdateDigest(requests[i])
	}
	stores := []*Store{first, second}
	startRace := make(chan struct{})
	results := make(chan error, len(requests))
	for i := 0; i < len(requests); i++ {
		i := i
		go func() {
			<-startRace
			var queueErr error
			for attempt := 0; attempt < 10; attempt++ {
				_, queueErr = stores[i].QueueLifecycleUpdate(ctx, requests[i])
				if !errors.Is(queueErr, ErrConcurrentStateChange) {
					break
				}
			}
			results <- queueErr
		}()
	}
	close(startRace)
	for i := 0; i < len(requests); i++ {
		if err := <-results; err != nil {
			t.Fatalf("queue terminal update: %v", err)
		}
	}
	events, err := first.ListLifecycleEvents(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID)
	if err != nil {
		t.Fatalf("list lifecycle events: %v", err)
	}
	var late []types.LifecycleEvent
	for _, event := range events {
		if event.Kind == types.LifecycleUpdateLate {
			late = append(late, event)
		}
	}
	if len(late) != 2 || late[0].ReducerSeq+1 != late[1].ReducerSeq {
		t.Fatalf("late event sequence is not unique and contiguous: %+v", late)
	}
	page, err := first.ListLifecycleEventPage(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID, late[0].ReducerSeq-1, 10)
	if err != nil || len(page.Events) != 2 || page.Events[0].EventID == page.Events[1].EventID {
		t.Fatalf("late cursor replay lost an event: %+v, %v", page, err)
	}
}

func TestLifecycleTerminalUpdateRefsReconstructAfterRestart(t *testing.T) {
	path := testStorePath(t)
	cleanupTestStorePath(path)
	t.Cleanup(func() { cleanupTestStorePath(path) })
	ctx := context.Background()
	first, err := Open(path)
	if err != nil {
		t.Fatalf("open first store: %v", err)
	}
	start := lifecycleStartFixture()
	start.CommandID = "command-start-terminal-ref-restart"
	start.TrajectoryID = "trajectory-terminal-ref-restart"
	start.InitialWork.WorkItemID = "work-terminal-ref-restart"
	start.InitialDocument.DocID = "document-terminal-ref-restart"
	start.InitialRevision.RevisionID = "revision-terminal-ref-restart"
	start.Agent.AgentID = "texture:document-terminal-ref-restart"
	start.Agent.ChannelID = "document-terminal-ref-restart"
	start.SubjectRefs["artifact"] = "texture://document-terminal-ref-restart"
	start.SubjectRefs["doc_id"] = start.InitialDocument.DocID
	start.StartRequestDigest, _ = ComputeStartLifecycleRequestDigest(start)
	started, err := first.StartLifecycle(ctx, start)
	if err != nil {
		t.Fatalf("start lifecycle: %v", err)
	}
	pending := queueLifecycleUpdateFixture(t, start, "command-queue-before-terminal-ref-restart")
	pending.UpdateID = "update-cancelled-terminal-ref-restart"
	pending.ProducerUpdateID = "producer-cancelled-terminal-ref-restart"
	pending.CommandDigest, _ = ComputeQueueLifecycleUpdateDigest(pending)
	if _, err := first.QueueLifecycleUpdate(ctx, pending); err != nil {
		t.Fatalf("queue pending update: %v", err)
	}
	cancel := types.CancelLifecycleRequest{
		OwnerID: start.OwnerID, ComputerID: start.ComputerID, CommandID: "command-cancel-terminal-ref-restart",
		TrajectoryID: start.TrajectoryID, ExpectedLifecycleVersion: started.Trajectory.LifecycleVersion + 1,
		ExpectedHeadRevisionID: started.Revision.RevisionID, Reason: "verify terminal refs",
	}
	cancel.CommandDigest, _ = ComputeCancelLifecycleDigest(cancel)
	if _, err := first.CancelLifecycleTrajectory(ctx, cancel); err != nil {
		t.Fatalf("cancel lifecycle: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("close first store: %v", err)
	}
	second, err := Open(path)
	if err != nil {
		t.Fatalf("reopen cancelled lifecycle: %v", err)
	}
	cancelled, err := second.GetLifecycleUpdate(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID, pending.TargetAgentID, pending.ProducerAgentID, pending.ProducerUpdateID)
	if err != nil || cancelled.Disposition != types.UpdateCancelled ||
		cancelled.DispositionRef != lifecycleTerminalTrajectoryRef(start.TrajectoryID) {
		t.Fatalf("cancelled terminal ref after restart = %+v, %v", cancelled, err)
	}
	late := queueLifecycleUpdateFixture(t, start, "command-late-terminal-ref-restart")
	late.UpdateID = "update-late-terminal-ref-restart"
	late.ProducerUpdateID = "producer-late-terminal-ref-restart"
	late.CommandDigest, _ = ComputeQueueLifecycleUpdateDigest(late)
	if _, err := second.QueueLifecycleUpdate(ctx, late); err != nil {
		t.Fatalf("queue late update after restart: %v", err)
	}
	if err := second.Close(); err != nil {
		t.Fatalf("close second store: %v", err)
	}
	third, err := Open(path)
	if err != nil {
		t.Fatalf("reopen late lifecycle: %v", err)
	}
	defer third.Close()
	reconstructed, err := third.GetLifecycleUpdate(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID, late.TargetAgentID, late.ProducerAgentID, late.ProducerUpdateID)
	if err != nil || reconstructed.Disposition != types.UpdateLate ||
		reconstructed.DispositionRef != lifecycleTerminalTrajectoryRef(start.TrajectoryID) ||
		reconstructed.Content != late.Content ||
		reconstructed.Packet.SchemaVersion != late.Packet.SchemaVersion ||
		reconstructed.Packet.Summary != late.Packet.Summary ||
		reconstructed.CreatedAt.IsZero() {
		t.Fatalf("late payload after restart = %+v, %v", reconstructed, err)
	}
}

func TestApplyLifecycleDigestIgnoresReducerGeneratedTimestamps(t *testing.T) {
	req := types.ApplyLifecycleUpdateRequest{
		CommandID: "command-digest-stable", TrajectoryID: "trajectory-digest-stable",
		Disposition: types.UpdateIncorporated,
		Revision: types.Revision{
			RevisionID: "revision-digest-stable", Content: "stable content",
			CreatedAt:  time.Unix(100, 0).UTC(),
			Provenance: json.RawMessage(`{"schema_version":1,"authored_at":"1970-01-01T00:01:40Z","authoring_model":{"provider":"test","model":"stable"}}`),
		},
	}
	graphA := TextureSourceGraphWriteSet{
		SourceEntities: []TextureSourceEntityGraphRecord{{
			CanonicalID: "source-entity", OwnerID: "owner-a", ComputerID: "computer-a",
			VersionID: "version-a", ContentHash: "sha256:stable", CreatedAt: time.Unix(100, 0).UTC(),
		}},
		SourceRefs: []TextureSourceRefGraphRecord{{
			CanonicalID: "source-ref", OwnerID: "owner-a", ComputerID: "computer-a",
			VersionID: "version-ref", ContentHash: "sha256:stable-ref", DocID: "doc-a",
			TextureRevisionID: "revision-digest-stable", CreatedAt: time.Unix(100, 0).UTC(),
		}},
	}
	first, err := ComputeApplyLifecycleUpdateWithSourceGraphDigest(req, graphA)
	if err != nil {
		t.Fatalf("first digest: %v", err)
	}
	req.Revision.CreatedAt = time.Unix(200, 0).UTC()
	req.Revision.Provenance = json.RawMessage(`{"schema_version":1,"authored_at":"1970-01-01T00:03:20Z","authoring_model":{"provider":"test","model":"stable"}}`)
	graphB := normalizeApplyLifecycleSourceGraphDigest(graphA)
	graphB.SourceEntities[0].OwnerID, graphB.SourceEntities[0].ComputerID = "owner-b", "computer-b"
	graphB.SourceEntities[0].CreatedAt = time.Unix(200, 0).UTC()
	graphB.SourceRefs[0].OwnerID, graphB.SourceRefs[0].ComputerID = "owner-b", "computer-b"
	graphB.SourceRefs[0].CreatedAt = time.Unix(200, 0).UTC()
	second, err := ComputeApplyLifecycleUpdateWithSourceGraphDigest(req, graphB)
	if err != nil {
		t.Fatalf("second digest: %v", err)
	}
	if first != second {
		t.Fatalf("reducer-generated timestamps changed apply identity: %s != %s", first, second)
	}
}

func TestLegacyPendingWorkerListsExcludeLifecycleUpdates(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	start := lifecycleStartFixture()
	start.StartRequestDigest, _ = ComputeStartLifecycleRequestDigest(start)
	if _, err := s.StartLifecycle(ctx, start); err != nil {
		t.Fatalf("start lifecycle: %v", err)
	}
	queue := queueLifecycleUpdateFixture(t, start, "command-queue-legacy-list-guard")
	if _, err := s.QueueLifecycleUpdate(ctx, queue); err != nil {
		t.Fatalf("queue lifecycle update: %v", err)
	}
	if updates, err := s.ListPendingWorkerUpdates(ctx, start.OwnerID, queue.TargetAgentID, 10); err != nil || len(updates) != 0 {
		t.Fatalf("target legacy pending list exposed lifecycle update: %+v, %v", updates, err)
	}
	if updates, err := s.ListPendingWorkerUpdatesAll(ctx, 10); err != nil || len(updates) != 0 {
		t.Fatalf("global legacy pending list exposed lifecycle update: %+v, %v", updates, err)
	}
	if updates, err := s.ListPendingLifecycleUpdates(ctx, start.OwnerID, start.ComputerID, queue.TargetAgentID, 10); err != nil || len(updates) != 1 {
		t.Fatalf("scoped lifecycle pending list = %+v, %v", updates, err)
	}
	if _, err := s.GetWorkerUpdate(ctx, start.OwnerID, queue.UpdateID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("legacy update lookup error = %v, want ErrNotFound", err)
	}
	if updates, err := s.ListWorkerUpdatesByTrajectory(ctx, start.OwnerID, start.TrajectoryID, 10); err != nil || len(updates) != 0 {
		t.Fatalf("legacy trajectory update list exposed lifecycle update: %+v, %v", updates, err)
	}
	if _, err := s.GetTrajectory(ctx, start.OwnerID, start.TrajectoryID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("legacy trajectory lookup error = %v, want ErrNotFound", err)
	}
	if trajectories, err := s.ListTrajectoriesByOwner(ctx, start.OwnerID, 10); err != nil || len(trajectories) != 0 {
		t.Fatalf("legacy trajectory list exposed lifecycle trajectory: %+v, %v", trajectories, err)
	}
	if _, err := s.GetWorkItem(ctx, start.OwnerID, start.InitialWork.WorkItemID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("legacy work lookup error = %v, want ErrNotFound", err)
	}
	if work, err := s.ListWorkItemsByTrajectory(ctx, start.OwnerID, start.TrajectoryID, false); err != nil || len(work) != 0 {
		t.Fatalf("legacy work list exposed lifecycle work: %+v, %v", work, err)
	}
	if pending, err := s.CountPendingWorkerUpdatesByTrajectory(ctx, start.OwnerID, start.TrajectoryID); err != nil || pending != 0 {
		t.Fatalf("legacy pending count exposed lifecycle update: %d, %v", pending, err)
	}
}

func TestLegacyListLimitsApplyAfterLifecycleExclusion(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	start := lifecycleStartFixture()
	legacyTrajectory := types.TrajectoryRecord{
		TrajectoryID: "legacy-" + start.TrajectoryID, OwnerID: start.OwnerID,
		Kind: types.TrajectoryKindTask, Status: types.TrajectoryLive,
		SettlementRule: types.SettlementRule{
			Version: types.LifecycleReducerVersion, RequireNoOpenWorkItems: true,
		},
		CreatedAt: time.Unix(1, 0).UTC(), UpdatedAt: time.Unix(1, 0).UTC(),
	}
	if _, err := s.CreateTrajectoryIfAbsent(ctx, legacyTrajectory); err != nil {
		t.Fatalf("create legacy trajectory: %v", err)
	}
	legacyUpdate := types.CoagentSourcePacket{
		UpdateID: "update-shared-lifecycle-exclusion", OwnerID: start.OwnerID,
		AgentID: "legacy-producer", TargetAgentID: start.Agent.AgentID,
		TrajectoryID: start.TrajectoryID, Content: "legacy update",
		CreatedAt: time.Unix(1, 0).UTC(),
	}
	start.StartRequestDigest, _ = ComputeStartLifecycleRequestDigest(start)
	if _, err := s.StartLifecycle(ctx, start); err != nil {
		t.Fatalf("start scoped lifecycle: %v", err)
	}
	queue := queueLifecycleUpdateFixture(t, start, "command-queue-shared-lifecycle-exclusion")
	queue.UpdateID = legacyUpdate.UpdateID
	queue.CommandDigest, _ = ComputeQueueLifecycleUpdateDigest(queue)
	if _, err := s.QueueLifecycleUpdate(ctx, queue); err != nil {
		t.Fatalf("queue scoped lifecycle update: %v", err)
	}
	if err := s.CreateWorkerUpdateOG(ctx, legacyUpdate); err != nil {
		t.Fatalf("create legacy update: %v", err)
	}
	for i := 0; i < ogMetadataPageSize+1; i++ {
		updateID := fmt.Sprintf("update-scoped-page-%03d", i)
		scoped := types.CoagentSourcePacket{
			UpdateID: updateID, OwnerID: start.OwnerID, ComputerID: start.ComputerID,
			AgentID: "scoped-producer", TargetAgentID: start.Agent.AgentID,
			TrajectoryID: start.TrajectoryID, Content: "scoped lifecycle update",
			Disposition: types.UpdatePending, LifecycleVersion: 1, CreatedAt: time.Now().UTC(),
		}
		metadata := map[string]any{
			"update_id": updateID, "target_agent_id": scoped.TargetAgentID,
			"trajectory_id": scoped.TrajectoryID,
		}
		if _, err := s.ogPut(ctx, objectgraph.ObjectKind("choir.worker_update"), start.OwnerID, "scoped-page-"+updateID, scoped, metadata, scoped.CreatedAt); err != nil {
			t.Fatalf("seed paginated scoped update %d: %v", i, err)
		}
	}
	trajectories, err := s.ListTrajectoriesByOwner(ctx, start.OwnerID, 1)
	if err != nil || len(trajectories) != 1 || trajectories[0].LifecycleVersion != 0 {
		t.Fatalf("post-filter trajectory limit = %+v, %v", trajectories, err)
	}
	updates, err := s.ListWorkerUpdatesByTrajectory(ctx, start.OwnerID, start.TrajectoryID, 1)
	if err != nil || len(updates) != 1 || updates[0].LifecycleVersion != 0 || updates[0].UpdateID != legacyUpdate.UpdateID {
		t.Fatalf("post-filter update limit = %+v, %v", updates, err)
	}
	pending, err := s.ListPendingWorkerUpdates(ctx, start.OwnerID, start.Agent.AgentID, 1)
	if err != nil || len(pending) != 1 || pending[0].UpdateID != legacyUpdate.UpdateID {
		t.Fatalf("post-filter pending limit = %+v, %v", pending, err)
	}
	allPending, err := s.ListPendingWorkerUpdatesAll(ctx, 1)
	if err != nil || len(allPending) != 1 || allPending[0].UpdateID != legacyUpdate.UpdateID {
		t.Fatalf("post-filter global pending limit = %+v, %v", allPending, err)
	}
	backlog, err := s.ListCoagentMailboxBacklog(ctx, start.OwnerID, start.Agent.AgentID, 1)
	if err != nil || len(backlog) != 1 || backlog[0].UpdateID != legacyUpdate.UpdateID {
		t.Fatalf("post-filter mailbox limit = %+v, %v", backlog, err)
	}
	count, err := s.CountPendingWorkerUpdatesByTrajectory(ctx, start.OwnerID, start.TrajectoryID)
	if err != nil || count != 1 {
		t.Fatalf("post-filter pending count = %d, %v", count, err)
	}
	stored, err := s.GetWorkerUpdate(ctx, start.OwnerID, legacyUpdate.UpdateID)
	if err != nil || stored.LifecycleVersion != 0 {
		t.Fatalf("legacy update lookup after lifecycle collision = %+v, %v", stored, err)
	}
}

func TestScopedAndLegacyListsExhaustPagesBeforeAuthorityFilters(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	start := lifecycleStartFixture()
	start.CommandID = "command-start-paginated-authority"
	start.TrajectoryID = "trajectory-paginated-authority"
	start.InitialWork.WorkItemID = "work-paginated-authority"
	start.InitialDocument.DocID = "document-paginated-authority"
	start.InitialRevision.RevisionID = "revision-paginated-authority"
	start.Agent.AgentID = "texture:document-paginated-authority"
	start.Agent.ChannelID = start.InitialDocument.DocID
	start.SubjectRefs["artifact"] = "texture://" + start.InitialDocument.DocID
	start.SubjectRefs["doc_id"] = start.InitialDocument.DocID
	start.StartRequestDigest, _ = ComputeStartLifecycleRequestDigest(start)
	if _, err := s.StartLifecycle(ctx, start); err != nil {
		t.Fatalf("start paginated lifecycle: %v", err)
	}
	legacyRun := types.RunRecord{
		RunID: "run-valid-after-authority-pages", OwnerID: start.OwnerID,
		AgentID: "legacy:valid-after-authority-pages", ChannelID: "channel-paginated-authority",
		State: types.RunPending, CreatedAt: time.Unix(1, 0).UTC(), UpdatedAt: time.Unix(1, 0).UTC(),
	}
	if err := s.CreateRun(ctx, legacyRun); err != nil {
		t.Fatalf("create legacy run: %v", err)
	}
	legacyDoc := types.Document{
		DocID: "document-valid-after-authority-pages", OwnerID: start.OwnerID,
		Title: "Legacy document", CreatedAt: time.Unix(1, 0).UTC(), UpdatedAt: time.Unix(1, 0).UTC(),
	}
	if err := s.CreateDocument(ctx, legacyDoc); err != nil {
		t.Fatalf("create legacy document: %v", err)
	}
	legacyRevision := types.Revision{
		RevisionID: "revision-valid-after-authority-pages", DocID: legacyDoc.DocID, OwnerID: start.OwnerID,
		VersionNumber: 1, AuthorKind: types.AuthorUser, Content: "Legacy revision", CreatedAt: time.Unix(1, 0).UTC(),
	}
	if err := s.CreateRevision(ctx, legacyRevision); err != nil {
		t.Fatalf("create legacy revision: %v", err)
	}
	now := time.Now().UTC()
	batch := objectgraph.Batch{Objects: make([]objectgraph.Object, 0, (ogMetadataPageSize+1)*3)}
	for i := 0; i < ogMetadataPageSize+1; i++ {
		computerID := fmt.Sprintf("computer-decoy-%03d", i)
		runID := fmt.Sprintf("run-authority-decoy-%03d", i)
		run := types.RunRecord{
			RunID: runID, OwnerID: start.OwnerID, SandboxID: computerID,
			AgentID: "agent-authority-decoy", ChannelID: legacyRun.ChannelID,
			TrajectoryID: start.TrajectoryID, State: types.RunPending,
			CreatedAt: now.Add(time.Duration(i) * time.Second), UpdatedAt: now.Add(time.Duration(i) * time.Second),
		}
		runObj, err := lifecycleObject(ogKindRun, start.OwnerID, computerID, runID, run, lifecycleMetadata("run_id", runID, computerID, start.TrajectoryID, 1), run.CreatedAt, run.UpdatedAt)
		if err != nil {
			t.Fatalf("build decoy run %d: %v", i, err)
		}
		docID := fmt.Sprintf("document-authority-decoy-%03d", i)
		doc := types.Document{
			DocID: docID, OwnerID: start.OwnerID, ComputerID: computerID,
			TrajectoryID: start.TrajectoryID, Title: "Lifecycle decoy",
			CurrentRevisionID: fmt.Sprintf("revision-authority-decoy-%03d", i),
			CreatedAt:         run.CreatedAt, UpdatedAt: run.UpdatedAt,
		}
		docObj, err := lifecycleObject(ogKindTexDoc, start.OwnerID, computerID, docID, doc, lifecycleMetadata("doc_id", docID, computerID, start.TrajectoryID, 1), doc.CreatedAt, doc.UpdatedAt)
		if err != nil {
			t.Fatalf("build decoy document %d: %v", i, err)
		}
		revisionID := fmt.Sprintf("revision-cross-computer-decoy-%03d", i)
		revision := types.Revision{
			RevisionID: revisionID, DocID: start.InitialDocument.DocID, OwnerID: start.OwnerID,
			ComputerID: computerID, TrajectoryID: start.TrajectoryID, VersionNumber: i + 2,
			AuthorKind: types.AuthorAppAgent, Content: "Cross-computer decoy", CreatedAt: run.CreatedAt,
		}
		revisionObj, err := lifecycleObject(ogKindTexRev, start.OwnerID, computerID, revisionID, revision, lifecycleMetadata("revision_id", revisionID, computerID, start.TrajectoryID, 1), revision.CreatedAt, revision.CreatedAt)
		if err != nil {
			t.Fatalf("build decoy revision %d: %v", i, err)
		}
		revisionMeta := map[string]any{}
		if err := json.Unmarshal(revisionObj.Metadata, &revisionMeta); err != nil {
			t.Fatalf("decode decoy revision metadata %d: %v", i, err)
		}
		revisionMeta["doc_id"] = start.InitialDocument.DocID
		revisionObj.Metadata, err = objectgraph.NormalizeMetadata(revisionMeta)
		if err != nil {
			t.Fatalf("normalize decoy revision metadata %d: %v", i, err)
		}
		revisionObj.ContentHash = objectgraph.ContentHash(revisionObj.ObjectKind, revisionObj.Body, revisionObj.Metadata)
		batch.Objects = append(batch.Objects, runObj, docObj, revisionObj)
	}
	if err := s.ogStore.PutBatch(ctx, batch); err != nil {
		t.Fatalf("seed authority pages: %v", err)
	}
	if runs, err := s.ListRunsByOwner(ctx, start.OwnerID, 1); err != nil || len(runs) != 1 || runs[0].RunID != legacyRun.RunID {
		t.Fatalf("owner run list after authority pages = %+v, %v", runs, err)
	}
	if runs, err := s.ListRunsByChannel(ctx, start.OwnerID, legacyRun.ChannelID, 1); err != nil || len(runs) != 1 || runs[0].RunID != legacyRun.RunID {
		t.Fatalf("channel run list after authority pages = %+v, %v", runs, err)
	}
	if docs, err := s.ListDocumentsByOwner(ctx, start.OwnerID, 1); err != nil || len(docs) != 1 || docs[0].DocID != legacyDoc.DocID {
		t.Fatalf("owner document list after authority pages = %+v, %v", docs, err)
	}
	if docs, err := s.ListDocumentsByScope(ctx, start.OwnerID, start.ComputerID, 1); err != nil || len(docs) != 1 || docs[0].DocID != start.InitialDocument.DocID {
		t.Fatalf("computer document list after other-computer pages = %+v, %v", docs, err)
	}
	if revisions, err := s.ListRevisionsByDoc(ctx, legacyDoc.DocID, start.OwnerID, 1); err != nil || len(revisions) != 1 || revisions[0].RevisionID != legacyRevision.RevisionID {
		t.Fatalf("legacy revision list after authority pages = %+v, %v", revisions, err)
	}
	if revisions, err := s.ListRevisionsByScope(ctx, start.InitialDocument.DocID, start.OwnerID, start.ComputerID, 1); err != nil || len(revisions) != 1 || revisions[0].RevisionID != start.InitialRevision.RevisionID {
		t.Fatalf("computer revision list after other-computer pages = %+v, %v", revisions, err)
	}
}

func TestTerminalLifecyclePinsArtifactHeadWhileDocumentAdvances(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	start := lifecycleStartFixture()
	start.CommandID = "command-start-pinned-terminal-head"
	start.TrajectoryID = "trajectory-pinned-terminal-head"
	start.InitialWork.WorkItemID = "work-pinned-terminal-head"
	start.InitialDocument.DocID = "document-pinned-terminal-head"
	start.InitialRevision.RevisionID = "revision-pinned-terminal-head"
	start.Agent.AgentID = "texture:document-pinned-terminal-head"
	start.Agent.ChannelID = start.InitialDocument.DocID
	start.SubjectRefs["artifact"] = "texture://" + start.InitialDocument.DocID
	start.SubjectRefs["doc_id"] = start.InitialDocument.DocID
	start.StartRequestDigest, _ = ComputeStartLifecycleRequestDigest(start)
	started, err := s.StartLifecycle(ctx, start)
	if err != nil {
		t.Fatalf("start lifecycle: %v", err)
	}
	cancel := types.CancelLifecycleRequest{
		OwnerID: start.OwnerID, ComputerID: start.ComputerID, CommandID: "command-cancel-pinned-terminal-head",
		TrajectoryID: start.TrajectoryID, ExpectedLifecycleVersion: started.Trajectory.LifecycleVersion,
		ExpectedHeadRevisionID: started.Revision.RevisionID, Reason: "pin accepted artifact",
	}
	cancel.CommandDigest, _ = ComputeCancelLifecycleDigest(cancel)
	cancelled, err := s.CancelLifecycleTrajectory(ctx, cancel)
	if err != nil {
		t.Fatalf("cancel lifecycle: %v", err)
	}
	advance := types.CommitLifecycleArtifactHeadRequest{
		OwnerID: start.OwnerID, ComputerID: start.ComputerID, CommandID: "command-unbound-head-after-terminal",
		TrajectoryID: start.TrajectoryID, ExpectedLifecycleVersion: cancelled.Trajectory.LifecycleVersion,
		ExpectedHeadRevisionID: started.Revision.RevisionID, Unbound: true,
		Revision: types.Revision{
			RevisionID: "revision-unbound-after-terminal", AuthorKind: types.AuthorUser,
			AuthorLabel: start.OwnerID, Content: "Independent post-terminal edit",
		},
	}
	advance.CommandDigest, _ = ComputeCommitLifecycleArtifactHeadDigest(advance)
	advanced, err := s.CommitLifecycleArtifactHead(ctx, advance)
	if err != nil {
		t.Fatalf("commit unbound head: %v", err)
	}
	if advanced.Trajectory.LifecycleVersion != cancelled.Trajectory.LifecycleVersion ||
		advanced.Trajectory.TerminalArtifactHeadRef != started.Revision.RevisionID ||
		advanced.Revision == nil || advanced.Revision.TrajectoryID != "" {
		t.Fatalf("unbound head changed terminal lifecycle: %+v", advanced)
	}
	snapshot, err := s.GetLifecycleSnapshot(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID)
	if err != nil {
		t.Fatalf("get lifecycle snapshot: %v", err)
	}
	if snapshot.HeadRevision.RevisionID != started.Revision.RevisionID ||
		snapshot.Document.CurrentRevisionID != advance.Revision.RevisionID ||
		snapshot.CurrentDocumentHead == nil ||
		snapshot.CurrentDocumentHead.RevisionID != advance.Revision.RevisionID {
		t.Fatalf("terminal and current heads not separated: %+v", snapshot)
	}
	replayed, err := s.CommitLifecycleArtifactHead(ctx, advance)
	if err != nil || !replayed.Replay || replayed.Revision == nil ||
		replayed.Revision.RevisionID != advance.Revision.RevisionID {
		t.Fatalf("unbound head replay = %+v, %v", replayed, err)
	}
	secondAdvance := advance
	secondAdvance.CommandID = "command-unbound-head-after-terminal-2"
	secondAdvance.CommandDigest = ""
	secondAdvance.ExpectedHeadRevisionID = advance.Revision.RevisionID
	secondAdvance.Revision = types.Revision{
		RevisionID: "revision-unbound-after-terminal-2", AuthorKind: types.AuthorUser,
		AuthorLabel: start.OwnerID, Content: "Second independent post-terminal edit",
	}
	secondAdvance.CommandDigest, _ = ComputeCommitLifecycleArtifactHeadDigest(secondAdvance)
	secondAdvanced, err := s.CommitLifecycleArtifactHead(ctx, secondAdvance)
	if err != nil {
		t.Fatalf("commit second unbound head: %v", err)
	}
	if secondAdvanced.Trajectory.LifecycleVersion != cancelled.Trajectory.LifecycleVersion ||
		secondAdvanced.Trajectory.TerminalArtifactHeadRef != started.Revision.RevisionID ||
		secondAdvanced.Receipt.ReducerSeq != advanced.Receipt.ReducerSeq+1 ||
		secondAdvanced.Revision == nil || secondAdvanced.Revision.TrajectoryID != "" {
		t.Fatalf("second unbound head changed terminal lifecycle or sequence: %+v", secondAdvanced)
	}
	secondReplay, err := s.CommitLifecycleArtifactHead(ctx, secondAdvance)
	if err != nil || !secondReplay.Replay || secondReplay.Receipt.ReducerSeq != secondAdvanced.Receipt.ReducerSeq {
		t.Fatalf("second unbound head replay = %+v, %v", secondReplay, err)
	}
	events, err := s.ListLifecycleEventPage(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID, cancelled.Trajectory.ReducerSeq, 10)
	if err != nil || len(events.Events) != 2 ||
		events.Events[0].Kind != types.LifecycleArtifactHeadAdvanced ||
		events.Events[1].Kind != types.LifecycleArtifactHeadAdvanced ||
		events.Events[0].ReducerSeq+1 != events.Events[1].ReducerSeq {
		t.Fatalf("post-terminal head events = %+v, %v", events, err)
	}
	snapshot, err = s.GetLifecycleSnapshot(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID)
	if err != nil || snapshot.Document.CurrentRevisionID != secondAdvance.Revision.RevisionID ||
		snapshot.HeadRevision.RevisionID != started.Revision.RevisionID {
		t.Fatalf("second current head did not preserve terminal head: %+v, %v", snapshot, err)
	}
	if trajectories, err := s.ListTrajectoriesByOwner(ctx, start.OwnerID, 10); err != nil || len(trajectories) != 0 {
		t.Fatalf("legacy trajectory list exposed lifecycle trajectory: %+v, %v", trajectories, err)
	}
	if work, err := s.ListWorkItemsByTrajectory(ctx, start.OwnerID, start.TrajectoryID, false); err != nil || len(work) != 0 {
		t.Fatalf("legacy work list exposed lifecycle work: %+v, %v", work, err)
	}
	if pending, err := s.CountPendingWorkerUpdatesByTrajectory(ctx, start.OwnerID, start.TrajectoryID); err != nil || pending != 0 {
		t.Fatalf("legacy pending count exposed lifecycle update: %d, %v", pending, err)
	}
}

func TestLifecycleWorkAndUpdatesDoNotCrossComputerScope(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	const (
		ownerID      = "owner-shared-lifecycle-scope"
		trajectoryID = "trajectory-shared-lifecycle-scope"
	)
	for _, computerID := range []string{"computer-a", "computer-b"} {
		start := lifecycleStartFixture()
		start.OwnerID = ownerID
		start.ComputerID = computerID
		start.CommandID = "command-start-" + computerID
		start.TrajectoryID = trajectoryID
		start.InitialWork.WorkItemID = "work-shared-lifecycle-scope"
		start.InitialWork.Objective = "work on " + computerID
		start.InitialDocument.DocID = "document-shared-lifecycle-scope"
		start.InitialRevision.RevisionID = "revision-shared-lifecycle-scope"
		start.InitialRevision.Content = "artifact on " + computerID
		start.Agent.AgentID = "texture:document-shared-lifecycle-scope"
		start.Agent.ChannelID = start.InitialDocument.DocID
		start.SubjectRefs["artifact"] = "texture://" + start.InitialDocument.DocID
		start.SubjectRefs["doc_id"] = start.InitialDocument.DocID
		start.StartRequestDigest, _ = ComputeStartLifecycleRequestDigest(start)
		if _, err := s.StartLifecycle(ctx, start); err != nil {
			t.Fatalf("start lifecycle on %s: %v", computerID, err)
		}
		queue := queueLifecycleUpdateFixture(t, start, "command-queue-"+computerID)
		queue.UpdateID = "update-shared-lifecycle-scope"
		queue.ProducerUpdateID = "producer-update-shared-lifecycle-scope"
		queue.Content = "update on " + computerID
		payloadDigest, digestErr := ComputeLifecycleUpdatePayloadDigest(queue.Packet, queue.Content)
		if digestErr != nil {
			t.Fatalf("payload digest on %s: %v", computerID, digestErr)
		}
		queue.PayloadDigest = payloadDigest
		queue.CommandDigest, _ = ComputeQueueLifecycleUpdateDigest(queue)
		if _, err := s.QueueLifecycleUpdate(ctx, queue); err != nil {
			t.Fatalf("queue lifecycle update on %s: %v", computerID, err)
		}
	}
	for _, computerID := range []string{"computer-a", "computer-b"} {
		snapshot, err := s.GetLifecycleSnapshot(ctx, ownerID, computerID, trajectoryID)
		if err != nil {
			t.Fatalf("snapshot on %s: %v", computerID, err)
		}
		if snapshot.HeadRevision.Content != "artifact on "+computerID ||
			len(snapshot.WorkItems) != 1 || snapshot.WorkItems[0].Objective != "work on "+computerID ||
			len(snapshot.Updates) != 1 || snapshot.Updates[0].ComputerID != computerID ||
			snapshot.Updates[0].Content != "update on "+computerID {
			t.Fatalf("snapshot crossed computer scope on %s: %+v", computerID, snapshot)
		}
	}
}
