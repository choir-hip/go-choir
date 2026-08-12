package textureowner

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentcore"
	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/provider"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func projectTextureOwnerTestProducer(t *testing.T, s *store.Store, start types.StartLifecycleRequest, suffix string) (string, string, string) {
	t.Helper()
	ctx := context.Background()
	now := time.Now().UTC()
	agentID := "researcher:" + suffix
	workID := "producer-work:" + suffix
	runID := "producer-run:" + suffix
	if err := s.UpsertAgent(ctx, types.AgentRecord{
		AgentID: agentID, OwnerID: start.OwnerID, ComputerID: start.ComputerID,
		Profile: "researcher", Role: "researcher", ChannelID: start.InitialDocument.DocID, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("seed lifecycle producer: %v", err)
	}
	open := types.OpenLifecycleWorkRequest{
		OwnerID: start.OwnerID, ComputerID: start.ComputerID, CommandID: "open-producer:" + suffix,
		TrajectoryID: start.TrajectoryID,
		WorkItem:     types.WorkItemRecord{WorkItemID: workID, Objective: "produce durable update", AssignedAgentID: agentID, AuthorityProfile: "researcher"},
	}
	open.CommandDigest, _ = store.ComputeOpenLifecycleWorkDigest(open)
	if _, err := s.OpenLifecycleWork(ctx, open); err != nil {
		t.Fatalf("open lifecycle producer work: %v", err)
	}
	run := types.RunRecord{
		RunID: runID, AgentID: agentID, ChannelID: start.InitialDocument.DocID, TrajectoryID: start.TrajectoryID,
		AgentProfile: "researcher", AgentRole: "researcher", OwnerID: start.OwnerID, ComputerID: start.ComputerID,
		State: types.RunRunning, CreatedAt: now, UpdatedAt: now, Metadata: map[string]any{"lifecycle_work_item_id": workID},
	}
	project := types.ReplaceLifecycleActivationRequest{
		OwnerID: start.OwnerID, ComputerID: start.ComputerID, CommandID: "project-producer:" + suffix,
		TrajectoryID: start.TrajectoryID, AgentID: agentID, Run: run,
	}
	project.CommandDigest, _ = store.ComputeReplaceLifecycleActivationDigest(project)
	if _, err := s.ReplaceLifecycleActivation(ctx, project); err != nil {
		t.Fatalf("project lifecycle producer: %v", err)
	}
	return agentID, workID, runID
}

func TestTextureOwnerStartRecoversDurableWakeAfterRestart(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "texture-restart.db")
	s1, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open first store: %v", err)
	}

	const (
		ownerID = "user-texture-restart"
		docID   = "doc-texture-restart"
		agentID = "texture:" + docID
	)
	now := time.Now().UTC()
	start := types.StartLifecycleRequest{
		OwnerID: ownerID, ComputerID: "autoputer-texture-restart", CommandID: "start-texture-restart",
		TrajectoryID: "trajectory-texture-restart", Kind: types.TrajectoryKindDocument,
		SettlementRule: types.SettlementRule{Version: types.LifecycleReducerVersion, RequireNoOpenWorkItems: true, RequiredSubjectRefs: []string{"artifact"}},
		SubjectRefs:    map[string]string{"artifact": "texture://documents/" + docID, "doc_id": docID},
		InitialWork: types.WorkItemRecord{
			WorkItemID: "work-texture-restart", Objective: "incorporate durable finding", AssignedAgentID: agentID,
		},
		InitialDocument: types.Document{DocID: docID, Title: "Restart target"},
		InitialRevision: types.Revision{
			RevisionID: "rev-texture-restart", AuthorKind: types.AuthorUser, AuthorLabel: "user",
			Content: "Durable content before restart",
		},
		Agent: types.AgentRecord{
			AgentID: agentID, OwnerID: ownerID, ComputerID: "autoputer-texture-restart",
			Profile: "texture", Role: "texture", ChannelID: docID, CreatedAt: now, UpdatedAt: now,
		},
	}
	start.StartRequestDigest, _ = store.ComputeStartLifecycleRequestDigest(start)
	if _, err := s1.StartLifecycle(ctx, start); err != nil {
		t.Fatalf("start durable lifecycle: %v", err)
	}
	producerAgentID, producerWorkID, producerRunID := projectTextureOwnerTestProducer(t, s1, start, "texture-restart")
	packet := types.CoagentSourcePacketPayload{
		SchemaVersion: types.CoagentSourcePacketSchemaV1, Kind: "evidence_update", Summary: "durable finding",
	}
	payloadDigest, _ := store.ComputeLifecycleUpdatePayloadDigest(packet, "Durable finding")
	queue := types.QueueLifecycleUpdateRequest{
		OwnerID: ownerID, ComputerID: "autoputer-texture-restart", CommandID: "queue-texture-restart",
		TrajectoryID: start.TrajectoryID, TargetAgentID: agentID,
		ProducerAgentID: producerAgentID, ProducerUpdateID: "update-texture-restart",
		UpdateID: "update-texture-restart", ChannelID: docID, Role: "researcher", SourceRunID: producerRunID,
		WorkItemID: producerWorkID, WorkDisposition: types.WorkItemOpen,
		Packet: packet, Content: "Durable finding", PayloadDigest: payloadDigest,
	}
	queue.CommandDigest, _ = store.ComputeQueueLifecycleUpdateDigest(queue)
	if _, err := s1.QueueLifecycleUpdate(ctx, queue); err != nil {
		t.Fatalf("queue durable update: %v", err)
	}
	if err := s1.CreateAgentMutation(ctx, store.AgentMutation{
		DocID: docID, RunID: "orphan-preprojection-run", OwnerID: ownerID,
		ComputerID: "autoputer-texture-restart", State: "pending", CreatedAt: now,
	}); err != nil {
		t.Fatalf("create orphan pre-projection mutation: %v", err)
	}
	if err := s1.CreateAgentMutation(ctx, store.AgentMutation{
		DocID: docID, RunID: "orphan-preprojection-run-newer", OwnerID: ownerID,
		ComputerID: "autoputer-texture-restart", State: "pending", CreatedAt: now.Add(time.Second),
	}); err != nil {
		t.Fatalf("create newer orphan pre-projection mutation: %v", err)
	}
	if err := s1.Close(); err != nil {
		t.Fatalf("close first store: %v", err)
	}

	s2, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	t.Cleanup(func() { _ = s2.Close() })
	rt := agentcore.New(provideriface.Config{
		ComputerID:          "autoputer-texture-restart",
		StorePath:           dbPath,
		PromptRoot:          filepath.Join(t.TempDir(), "prompts"),
		ProviderTimeout:     time.Second,
		SupervisionInterval: time.Hour,
	}, s2, events.NewEventBus(), provider.NewStubProvider(0))
	rt.SetDispatchActor(func(context.Context, string, string, string, string, string, string, string) error { return nil })
	t.Cleanup(rt.Stop)

	NewHandler(rt).Start(ctx)
	runs, err := s2.ListLifecycleRunsByOwner(ctx, ownerID, "autoputer-texture-restart", 20)
	if err != nil {
		t.Fatalf("list recovered runs: %v", err)
	}
	for _, run := range runs {
		if run.AgentID == agentID && run.ChannelID == docID && run.State == types.RunPending {
			for _, orphanRunID := range []string{"orphan-preprojection-run", "orphan-preprojection-run-newer"} {
				orphan, orphanErr := s2.GetAgentMutationByRun(ctx, ownerID, "autoputer-texture-restart", orphanRunID)
				if orphanErr != nil || orphan == nil || orphan.State != "stale_activation" {
					t.Fatalf("orphan mutation %s was not staled before recovery: %+v, %v", orphanRunID, orphan, orphanErr)
				}
			}
			return
		}
	}
	t.Fatalf("durable Texture wake did not create a pending owner run after restart: %+v", runs)
}

func TestTextureOwnerRestartDoesNotCrossComputerPendingMutation(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "texture-mutation-scope-restart.db")
	s1, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open first store: %v", err)
	}
	const (
		ownerID      = "owner-shared-restart"
		docID        = "doc-shared-restart"
		agentID      = "texture:" + docID
		trajectoryID = "trajectory-shared-restart"
	)
	now := time.Now().UTC()
	for _, computerID := range []string{"computer-a", "computer-b"} {
		start := types.StartLifecycleRequest{
			OwnerID: ownerID, ComputerID: computerID, CommandID: "start:" + computerID,
			TrajectoryID: trajectoryID, Kind: types.TrajectoryKindDocument,
			SettlementRule: types.SettlementRule{
				Version: types.LifecycleReducerVersion, RequireNoOpenWorkItems: true,
				RequiredSubjectRefs: []string{"artifact"},
			},
			SubjectRefs: map[string]string{"artifact": "texture://documents/" + docID, "doc_id": docID},
			InitialWork: types.WorkItemRecord{
				WorkItemID: "work-shared-restart", Objective: "incorporate scoped update", AssignedAgentID: agentID,
			},
			InitialDocument: types.Document{DocID: docID, Title: "Scoped restart target"},
			InitialRevision: types.Revision{
				RevisionID: "revision-shared-restart", AuthorKind: types.AuthorUser, AuthorLabel: ownerID, Content: "Initial scoped content",
			},
			Agent: types.AgentRecord{
				AgentID: agentID, OwnerID: ownerID, ComputerID: computerID,
				Profile: "texture", Role: "texture", ChannelID: docID, CreatedAt: now, UpdatedAt: now,
			},
		}
		start.StartRequestDigest, _ = store.ComputeStartLifecycleRequestDigest(start)
		if _, err := s1.StartLifecycle(ctx, start); err != nil {
			t.Fatalf("start lifecycle for %s: %v", computerID, err)
		}
	}
	computerBStart := types.StartLifecycleRequest{
		OwnerID: ownerID, ComputerID: "computer-b", TrajectoryID: trajectoryID,
		InitialWork:     types.WorkItemRecord{WorkItemID: "work-shared-restart"},
		InitialDocument: types.Document{DocID: docID},
	}
	producerAgentID, producerWorkID, producerRunID := projectTextureOwnerTestProducer(t, s1, computerBStart, "computer-b")
	if err := s1.CreateAgentMutation(ctx, store.AgentMutation{
		DocID: docID, RunID: "shared-run", OwnerID: ownerID, ComputerID: "computer-a",
		State: "pending", CreatedAt: now,
	}); err != nil {
		t.Fatalf("create computer A pending mutation: %v", err)
	}
	packet := types.CoagentSourcePacketPayload{
		SchemaVersion: types.CoagentSourcePacketSchemaV1, Kind: "evidence_update", Summary: "computer B update",
	}
	payloadDigest, _ := store.ComputeLifecycleUpdatePayloadDigest(packet, "computer B update")
	queue := types.QueueLifecycleUpdateRequest{
		OwnerID: ownerID, ComputerID: "computer-b", CommandID: "queue:computer-b",
		TrajectoryID: trajectoryID, TargetAgentID: agentID, ProducerAgentID: producerAgentID,
		ProducerUpdateID: "update-computer-b", UpdateID: "update-computer-b", ChannelID: docID,
		Role: "researcher", SourceRunID: producerRunID, WorkItemID: producerWorkID, WorkDisposition: types.WorkItemOpen,
		Packet: packet, Content: "computer B update", PayloadDigest: payloadDigest,
	}
	queue.CommandDigest, _ = store.ComputeQueueLifecycleUpdateDigest(queue)
	if _, err := s1.QueueLifecycleUpdate(ctx, queue); err != nil {
		t.Fatalf("queue computer B update: %v", err)
	}
	if err := s1.Close(); err != nil {
		t.Fatalf("close first store: %v", err)
	}

	s2, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	t.Cleanup(func() { _ = s2.Close() })
	rt := agentcore.New(provideriface.Config{
		ComputerID: "computer-b", StorePath: dbPath, PromptRoot: filepath.Join(t.TempDir(), "prompts"),
		ProviderTimeout: time.Second, SupervisionInterval: time.Hour,
	}, s2, events.NewEventBus(), provider.NewStubProvider(0))
	rt.SetDispatchActor(func(context.Context, string, string, string, string, string, string, string) error { return nil })
	t.Cleanup(rt.Stop)

	NewHandler(rt).Start(ctx)
	runs, err := s2.ListLifecycleRunsByOwner(ctx, ownerID, "computer-b", 20)
	if err != nil {
		t.Fatalf("list computer B runs: %v", err)
	}
	for _, run := range runs {
		if run.AgentID == agentID && run.State == types.RunPending {
			pendingA, pendingErr := s2.GetPendingAgentMutationByDoc(ctx, ownerID, "computer-a", docID)
			if pendingErr != nil || pendingA == nil || pendingA.RunID != "shared-run" {
				t.Fatalf("computer A mutation changed during B restart: %+v, %v", pendingA, pendingErr)
			}
			return
		}
	}
	t.Fatalf("computer A pending mutation suppressed computer B restart wake: %+v", runs)
}

func TestTextureOwnerStartSkipsRetainedCancelledInstructionWithoutNewActivation(t *testing.T) {
	core, handler := testAPISetup(t)
	core.SetDispatchActor(func(context.Context, string, string, string, string, string, string, string) error { return nil })
	start := startObservationLifecycle(t, core.Store())
	handler.wakeOwnerInstruction = func(context.Context, string, string, string) error { return nil }
	path := "/api/texture/documents/" + start.InitialDocument.DocID + "/tell"
	queued := postOwnerInstruction(t, handler, path, start.OwnerID, "terminal-boot", "retain terminal instruction", start.InitialRevision.RevisionID)
	if queued.Code != 202 {
		t.Fatalf("queue status=%d body=%s", queued.Code, queued.Body.String())
	}
	snapshot, err := core.Store().GetLifecycleSnapshot(t.Context(), start.OwnerID, start.ComputerID, start.TrajectoryID)
	if err != nil {
		t.Fatal(err)
	}
	cancel := types.CancelLifecycleRequest{
		OwnerID: start.OwnerID, ComputerID: start.ComputerID, CommandID: "cancel-terminal-texture-boot",
		TrajectoryID: start.TrajectoryID, ExpectedLifecycleVersion: snapshot.Trajectory.LifecycleVersion,
		ExpectedHeadRevisionID: snapshot.HeadRevision.RevisionID, Reason: "terminal boot regression",
	}
	cancel.CommandDigest, _ = store.ComputeCancelLifecycleDigest(cancel)
	if _, err := core.Store().CancelLifecycleTrajectory(t.Context(), cancel); err != nil {
		t.Fatal(err)
	}
	if err := handler.Start(t.Context()); err != nil {
		t.Fatalf("terminal Texture boot reconciliation failed: %v", err)
	}
	runs, err := core.Store().ListLifecycleRunsByOwner(t.Context(), start.OwnerID, start.ComputerID, 20)
	if err != nil {
		t.Fatal(err)
	}
	for _, run := range runs {
		if run.AgentID == start.Agent.AgentID {
			t.Fatalf("terminal boot created Texture run: %+v", run)
		}
	}
	pending, err := core.Store().ListPendingLifecycleOwnerInstructions(t.Context(), start.OwnerID, start.ComputerID, start.TrajectoryID, start.Agent.AgentID, 10)
	if err != nil || len(pending) != 1 || pending[0].Content != "retain terminal instruction" {
		t.Fatalf("retained pending instruction=%+v err=%v", pending, err)
	}
	terminal, err := core.Store().GetLifecycleSnapshot(t.Context(), start.OwnerID, start.ComputerID, start.TrajectoryID)
	if err != nil || terminal.Trajectory.Status != types.TrajectoryCancelled || terminal.WorkItems[0].Status != types.WorkItemCancelled {
		t.Fatalf("terminal evidence=%+v err=%v", terminal, err)
	}
}

func TestTextureOwnerWakeSkipsLiveTrajectoryWithCancellationIntent(t *testing.T) {
	core, handler := testAPISetup(t)
	start := startObservationLifecycle(t, core.Store())
	handler.wakeOwnerInstruction = func(context.Context, string, string, string) error { return nil }
	path := "/api/texture/documents/" + start.InitialDocument.DocID + "/tell"
	queued := postOwnerInstruction(t, handler, path, start.OwnerID, "prepared-cancel", "do not outrun cancellation", start.InitialRevision.RevisionID)
	if queued.Code != 202 {
		t.Fatalf("queue status=%d body=%s", queued.Code, queued.Body.String())
	}
	snapshot, err := core.Store().GetLifecycleSnapshot(t.Context(), start.OwnerID, start.ComputerID, start.TrajectoryID)
	if err != nil {
		t.Fatal(err)
	}
	cancel := types.CancelLifecycleRequest{
		OwnerID: start.OwnerID, ComputerID: start.ComputerID, CommandID: "prepare-terminal-texture-boot",
		TrajectoryID: start.TrajectoryID, ExpectedLifecycleVersion: snapshot.Trajectory.LifecycleVersion,
		ExpectedHeadRevisionID: snapshot.HeadRevision.RevisionID, Reason: "prepared cancellation",
	}
	cancel.CommandDigest, _ = store.ComputeCancelLifecycleDigest(cancel)
	if _, err := core.Store().PrepareLifecycleCancellation(t.Context(), cancel); err != nil {
		t.Fatal(err)
	}
	run, err := handler.ReconcileAgentWake(t.Context(), start.OwnerID, start.InitialDocument.DocID)
	if err != nil || run != nil {
		t.Fatalf("prepared cancellation wake run=%+v err=%v", run, err)
	}
	runs, err := core.Store().ListLifecycleRunsByOwner(t.Context(), start.OwnerID, start.ComputerID, 20)
	if err != nil {
		t.Fatal(err)
	}
	for _, candidate := range runs {
		if candidate.AgentID == start.Agent.AgentID {
			t.Fatalf("prepared cancellation created Texture run: %+v", candidate)
		}
	}
}

func TestTextureLifecycleActivationEligibilityPropagatesStoreFailure(t *testing.T) {
	core, handler := testAPISetup(t)
	start := startObservationLifecycle(t, core.Store())
	doc, err := core.Store().GetLifecycleDocument(t.Context(), start.OwnerID, start.ComputerID, start.InitialDocument.DocID)
	if err != nil {
		t.Fatal(err)
	}
	mismatched := doc
	mismatched.CurrentRevisionID = "foreign-head"
	if eligible, mismatchErr := handler.textureLifecycleActivationEligible(t.Context(), mismatched); mismatchErr == nil || eligible {
		t.Fatalf("mismatched snapshot eligibility=%v err=%v", eligible, mismatchErr)
	}
	if err := core.Store().Close(); err != nil {
		t.Fatal(err)
	}
	eligible, err := handler.textureLifecycleActivationEligible(t.Context(), doc)
	if err == nil || eligible {
		t.Fatalf("closed Store eligibility=%v err=%v", eligible, err)
	}
}

func TestTextureLifecycleActivationClassificationRejectsUnknownStatusAndIntentFailure(t *testing.T) {
	core, _ := testAPISetup(t)
	start := startObservationLifecycle(t, core.Store())
	doc, err := core.Store().GetLifecycleDocument(t.Context(), start.OwnerID, start.ComputerID, start.InitialDocument.DocID)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := core.Store().GetLifecycleSnapshot(t.Context(), start.OwnerID, start.ComputerID, start.TrajectoryID)
	if err != nil {
		t.Fatal(err)
	}
	unknown := snapshot
	unknown.Trajectory.Status = types.TrajectoryStatus("corrupt-status")
	if eligible, classifyErr := classifyTextureLifecycleActivationSnapshot(doc, unknown); classifyErr == nil || eligible {
		t.Fatalf("unknown status eligibility=%v err=%v", eligible, classifyErr)
	}
	operational := errors.New("intent backend unavailable")
	if eligible, intentErr := textureCancellationIntentPermitsActivation(operational); intentErr == nil || eligible || !errors.Is(intentErr, operational) {
		t.Fatalf("operational intent eligibility=%v err=%v", eligible, intentErr)
	}
	if eligible, intentErr := textureCancellationIntentPermitsActivation(store.ErrNotFound); intentErr != nil || !eligible {
		t.Fatalf("missing intent eligibility=%v err=%v", eligible, intentErr)
	}
	if eligible, intentErr := textureCancellationIntentPermitsActivation(nil); intentErr != nil || eligible {
		t.Fatalf("present intent eligibility=%v err=%v", eligible, intentErr)
	}
}

func TestTextureOwnerWakeKeepsMissingOpenWorkFatalWhileTrajectoryIsLive(t *testing.T) {
	core, handler := testAPISetup(t)
	start := startObservationLifecycle(t, core.Store())
	handler.wakeOwnerInstruction = func(context.Context, string, string, string) error { return nil }
	path := "/api/texture/documents/" + start.InitialDocument.DocID + "/tell"
	queued := postOwnerInstruction(t, handler, path, start.OwnerID, "live-missing-work", "must not invent live work", start.InitialRevision.RevisionID)
	if queued.Code != 202 {
		t.Fatalf("queue status=%d body=%s", queued.Code, queued.Body.String())
	}
	refuse := types.RefuseLifecycleWorkRequest{
		OwnerID: start.OwnerID, ComputerID: start.ComputerID, CommandID: "refuse-live-texture-work",
		TrajectoryID: start.TrajectoryID, WorkItemID: start.InitialWork.WorkItemID, ActingAgentID: start.Agent.AgentID,
		RefusalRef: "refusal://live/missing-work", Reason: "test live no-work authority",
	}
	refuse.CommandDigest, _ = store.ComputeRefuseLifecycleWorkDigest(refuse)
	if _, err := core.Store().RefuseLifecycleWork(t.Context(), refuse); err != nil {
		t.Fatal(err)
	}
	run, err := handler.ReconcileAgentWake(t.Context(), start.OwnerID, start.InitialDocument.DocID)
	if err == nil || run != nil || !errors.Is(err, errTextureLifecycleOpenWorkUnavailable) {
		t.Fatalf("live no-work wake run=%+v err=%v", run, err)
	}
}
