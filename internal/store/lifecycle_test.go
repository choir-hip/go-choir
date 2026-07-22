package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

func lifecycleStartFixture() types.StartLifecycleRequest {
	req := types.StartLifecycleRequest{
		OwnerID: "owner-lifecycle", ComputerID: "computer-lifecycle",
		CommandID:    "command-start-1",
		TrajectoryID: "trajectory-lifecycle-1", Kind: types.TrajectoryKindTask,
		SubjectRefs:     map[string]string{"artifact": "texture://artifact/1", "doc_id": "document-lifecycle-1"},
		SettlementRule:  types.SettlementRule{RequireNoOpenWorkItems: true, RequiredSubjectRefs: []string{"artifact"}},
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

	settle := types.SettleLifecycleWorkRequest{
		OwnerID: start.OwnerID, ComputerID: start.ComputerID,
		CommandID: "command-settle-1", TrajectoryID: start.TrajectoryID,
		WorkItemID: start.InitialWork.WorkItemID, ResultRef: "texture://result/1",
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
	apply.Revision = types.Revision{RevisionID: "revision-lifecycle-v1", AuthorKind: types.AuthorAppAgent, AuthorLabel: "researcher", Content: "Incorporated update"}
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
	cancel := types.CancelLifecycleRequest{
		OwnerID: start.OwnerID, ComputerID: start.ComputerID,
		CommandID: "command-cancel-1", TrajectoryID: start.TrajectoryID,
		Reason: "owner cancelled",
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
	apply.WorkItemID = start.InitialWork.WorkItemID
	apply.CommandDigest, _ = ComputeApplyLifecycleUpdateDigest(apply)
	result, err := s.ApplyLifecycleUpdate(ctx, apply)
	if err != nil {
		t.Fatalf("reject update: %v", err)
	}
	if result.WorkItem == nil || result.WorkItem.Status != types.WorkItemRefused {
		t.Fatalf("rejected producer work not refused: %+v", result.WorkItem)
	}
	for _, event := range result.Events {
		if event.Kind == types.LifecycleWorkSettled {
			t.Fatalf("rejected update emitted successful work settlement: %+v", result.Events)
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
			AssignedAgentID: start.Agent.AgentID, AuthorityProfile: "reviewer", StepBudget: 4,
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
		WorkItemID: open.WorkItem.WorkItemID, Reason: "authority is too narrow",
	}
	refuse.CommandDigest, _ = ComputeRefuseLifecycleWorkDigest(refuse)
	refused, err := s.RefuseLifecycleWork(ctx, refuse)
	if err != nil {
		t.Fatalf("refuse work: %v", err)
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
		Reason: "owner completed lifecycle before archival",
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

func TestLifecycleReplaceActivationCommitsRunAndProjectionAtomically(t *testing.T) {
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
	if replaced.Agent == nil || replaced.Agent.AgentID != started.Agent.AgentID || replaced.Trajectory.ReducerSeq != 2 {
		t.Fatalf("unexpected activation result: %+v", replaced)
	}
	storedRun, err := s.GetRunByOwner(ctx, start.OwnerID, run.RunID)
	if err != nil || storedRun.State != types.RunPending {
		t.Fatalf("atomic run record = %+v, %v", storedRun, err)
	}
	snapshot, err := s.GetLifecycleSnapshot(ctx, start.OwnerID, start.ComputerID, start.TrajectoryID)
	if err != nil || snapshot.Activation.RunID != run.RunID || snapshot.Activation.State != types.RunPending {
		t.Fatalf("activation snapshot = %+v, %v", snapshot.Activation, err)
	}
	replay, err := s.ReplaceLifecycleActivation(ctx, replace)
	if err != nil || !replay.Replay || replay.Trajectory.ReducerSeq != 2 {
		t.Fatalf("activation replay = %+v, %v", replay, err)
	}
}
