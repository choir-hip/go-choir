package textureowner

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
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
	agentID := "research:" + suffix
	workID := "producer-work:" + suffix
	runID := "producer-run:" + suffix
	if err := s.UpsertAgent(ctx, types.AgentRecord{
		AgentID: agentID, OwnerID: start.OwnerID, ComputerID: start.ComputerID,
		Profile: "research", Role: "research", ChannelID: start.InitialDocument.DocID, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("seed lifecycle producer: %v", err)
	}
	open := types.OpenLifecycleWorkRequest{
		OwnerID: start.OwnerID, ComputerID: start.ComputerID, CommandID: "open-producer:" + suffix,
		TrajectoryID: start.TrajectoryID,
		WorkItem:     types.WorkItemRecord{WorkItemID: workID, Objective: "produce durable update", AssignedAgentID: agentID, AuthorityProfile: "research"},
	}
	open.CommandDigest, _ = store.ComputeOpenLifecycleWorkDigest(open)
	if _, err := s.OpenLifecycleWork(ctx, open); err != nil {
		t.Fatalf("open lifecycle producer work: %v", err)
	}
	run := types.RunRecord{
		RunID: runID, AgentID: agentID, ChannelID: start.InitialDocument.DocID, TrajectoryID: start.TrajectoryID,
		AgentProfile: "research", AgentRole: "research", OwnerID: start.OwnerID, ComputerID: start.ComputerID,
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
		UpdateID: "update-texture-restart", ChannelID: docID, Role: "research", SourceRunID: producerRunID,
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
		Role: "research", SourceRunID: producerRunID, WorkItemID: producerWorkID, WorkDisposition: types.WorkItemOpen,
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

// A texture run passivated with passivated_reason=runtime_restarted while an
// owner revision is still pending must be reactivated by Handler.Start, not
// left stranded. Production receipt: run 654acaec on the owner computer was
// passivated at 03:48 and never reactivated across the 04:21 restart — the
// owner revision stayed armed-but-undelivered.
func TestTextureOwnerStartReactivatesRuntimeRestartedPassivatedRun(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "texture-restart-rewake.db")
	s1, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open first store: %v", err)
	}

	const (
		ownerID      = "user-texture-rewake"
		docID        = "doc-texture-rewake"
		agentID      = "texture:" + docID
		trajectoryID = "trajectory-texture-rewake"
		revisionID   = "rev-texture-rewake"
		computerID   = "autoputer-texture-rewake"
		passivatedID = "passivated-texture-rewake"
	)
	now := time.Now().UTC()
	start := types.StartLifecycleRequest{
		OwnerID: ownerID, ComputerID: computerID, CommandID: "start-texture-rewake",
		TrajectoryID: trajectoryID, Kind: types.TrajectoryKindDocument,
		SettlementRule: types.SettlementRule{Version: types.LifecycleReducerVersion, RequireNoOpenWorkItems: true, RequiredSubjectRefs: []string{"artifact"}},
		SubjectRefs:    map[string]string{"artifact": "texture://documents/" + docID, "doc_id": docID},
		InitialWork: types.WorkItemRecord{
			WorkItemID: "work-texture-rewake", Objective: "owner directive", AssignedAgentID: agentID,
		},
		InitialDocument: types.Document{DocID: docID, Title: "Rewake target"},
		InitialRevision: types.Revision{
			RevisionID: revisionID, AuthorKind: types.AuthorUser, AuthorLabel: "user",
			Content: "Owner directive pending rewake",
		},
		Agent: types.AgentRecord{
			AgentID: agentID, OwnerID: ownerID, ComputerID: computerID,
			Profile: "texture", Role: "texture", ChannelID: docID, CreatedAt: now, UpdatedAt: now,
		},
	}
	start.StartRequestDigest, _ = store.ComputeStartLifecycleRequestDigest(start)
	if _, err := s1.StartLifecycle(ctx, start); err != nil {
		t.Fatalf("start durable lifecycle: %v", err)
	}

	// Seed exactly as production: insert a pending texture activation via the
	// lifecycle projection, then passivate it the way a runtime_restarted
	// boot does — UpdateRun on the same record, keeping TrajectoryID.
	live := types.RunRecord{
		RunID: passivatedID, AgentID: agentID, OwnerID: ownerID, ComputerID: computerID,
		ChannelID: docID, TrajectoryID: trajectoryID,
		AgentProfile: "texture", AgentRole: "texture",
		State:     types.RunPending,
		CreatedAt: now, UpdatedAt: now,
		Metadata: map[string]any{
			"type":                    textureAgentRevisionTaskType,
			"doc_id":                  docID,
			"current_revision_id":     revisionID,
			"lifecycle_trajectory_id": trajectoryID,
		},
	}
	insertReq := types.ReplaceLifecycleActivationRequest{
		OwnerID: ownerID, ComputerID: computerID, CommandID: "seed-live:" + passivatedID,
		TrajectoryID: trajectoryID, AgentID: agentID, Run: live,
	}
	insertReq.CommandDigest, _ = store.ComputeReplaceLifecycleActivationDigest(insertReq)
	if _, err := s1.ReplaceLifecycleActivation(ctx, insertReq); err != nil {
		t.Fatalf("seed live texture run: %v", err)
	}
	passivated := live
	passivated.State = types.RunPassivated
	passivated.Metadata = map[string]any{}
	for k, v := range live.Metadata {
		passivated.Metadata[k] = v
	}
	passivated.Metadata["passivated_reason"] = "runtime_restarted"
	if err := s1.UpdateRun(ctx, passivated); err != nil {
		t.Fatalf("passivate texture run: %v", err)
	}
	// Boot passivation stales the mutation; reactivation must accept that state.
	if err := s1.CreateAgentMutation(ctx, store.AgentMutation{
		DocID: docID, RunID: passivatedID, OwnerID: ownerID, ComputerID: computerID,
		State: "stale_activation", RevisionID: revisionID, CreatedAt: now,
	}); err != nil {
		t.Fatalf("seed stale_activation mutation: %v", err)
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
		ComputerID: computerID, StorePath: dbPath,
		PromptRoot: filepath.Join(t.TempDir(), "prompts"),
		ProviderTimeout: time.Second, SupervisionInterval: time.Hour,
	}, s2, events.NewEventBus(), provider.NewStubProvider(0))
	rt.SetDispatchActor(func(context.Context, string, string, string, string, string, string, string) error { return nil })
	t.Cleanup(rt.Stop)

	if err := NewHandler(rt).Start(ctx); err != nil {
		t.Fatalf("texture owner start: %v", err)
	}

	runs, err := s2.ListLifecycleRunsByChannel(ctx, ownerID, computerID, docID, 10)
	if err != nil {
		t.Fatalf("list lifecycle runs: %v", err)
	}
	var run *types.RunRecord
	for i := range runs {
		if runs[i].RunID == passivatedID {
			run = &runs[i]
		}
	}
	if run == nil {
		t.Fatalf("passivated texture run not listed")
	}
	if run.State != types.RunPending {
		t.Fatalf("runtime_restarted texture run was not reactivated; state=%s reason=%s",
			run.State, metadataStringValue(run.Metadata, "passivated_reason"))
	}
}

// Boot with the open work item as the only arm (desk-authored head, no
// producer reports) must leave a passivated Texture run passivated. Boot
// dispatches no occurrence for open work, and only an occurrence executes a
// run, so a re-arm here produced a run that stayed pending forever and a
// document that opened into "Revising…" on every boot (67 documents on the
// owner computer; docs/problems/texture-zombie-activations-revising-forever-
// 2026-10-09.md). Failure modes pinned: the run re-armed to pending; its
// mutation re-armed (agent_revision_pending); a new run minted beside it.
func TestTextureOwnerStartLeavesOpenWorkPassivatedRunWithoutExecutor(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "texture-restart-workrewake.db")
	s1, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open first store: %v", err)
	}

	const (
		ownerID      = "user-texture-workrewake"
		docID        = "doc-texture-workrewake"
		agentID      = "texture:" + docID
		trajectoryID = "trajectory-texture-workrewake"
		revisionID   = "rev-texture-workrewake"
		computerID   = "autoputer-texture-workrewake"
		passivatedID = "passivated-texture-workrewake"
	)
	now := time.Now().UTC()
	// Desk-authored head (author app agent, texture_cell source) → not
	// owner-input, so PendingTextureOwnerRevision reports false and the open
	// work item is the sole wake arm.
	headMeta, _ := json.Marshal(map[string]any{"source": "texture_cell"})
	start := types.StartLifecycleRequest{
		OwnerID: ownerID, ComputerID: computerID, CommandID: "start-texture-workrewake",
		TrajectoryID: trajectoryID, Kind: types.TrajectoryKindDocument,
		SettlementRule: types.SettlementRule{Version: types.LifecycleReducerVersion, RequireNoOpenWorkItems: true, RequiredSubjectRefs: []string{"artifact"}},
		SubjectRefs:    map[string]string{"artifact": "texture://documents/" + docID, "doc_id": docID},
		InitialWork: types.WorkItemRecord{
			WorkItemID: "work-texture-workrewake", Objective: "desk work pending rewake", AssignedAgentID: agentID,
		},
		InitialDocument: types.Document{DocID: docID, Title: "Work rewake target"},
		InitialRevision: types.Revision{
			RevisionID: revisionID, AuthorKind: types.AuthorAppAgent, AuthorLabel: "texture",
			BodyDoc: observationBodyDoc("Desk head"), Metadata: headMeta,
		},
		Agent: types.AgentRecord{
			AgentID: agentID, OwnerID: ownerID, ComputerID: computerID,
			Profile: "texture", Role: "texture", ChannelID: docID, CreatedAt: now, UpdatedAt: now,
		},
	}
	start.StartRequestDigest, _ = store.ComputeStartLifecycleRequestDigest(start)
	if _, err := s1.StartLifecycle(ctx, start); err != nil {
		t.Fatalf("start durable lifecycle: %v", err)
	}

	live := types.RunRecord{
		RunID: passivatedID, AgentID: agentID, OwnerID: ownerID, ComputerID: computerID,
		ChannelID: docID, TrajectoryID: trajectoryID,
		AgentProfile: "texture", AgentRole: "texture",
		State:     types.RunPending,
		CreatedAt: now, UpdatedAt: now,
		Metadata: map[string]any{
			"type":                    textureAgentRevisionTaskType,
			"doc_id":                  docID,
			"current_revision_id":     revisionID,
			"lifecycle_trajectory_id": trajectoryID,
		},
	}
	insertReq := types.ReplaceLifecycleActivationRequest{
		OwnerID: ownerID, ComputerID: computerID, CommandID: "seed-live-work:" + passivatedID,
		TrajectoryID: trajectoryID, AgentID: agentID, Run: live,
	}
	insertReq.CommandDigest, _ = store.ComputeReplaceLifecycleActivationDigest(insertReq)
	if _, err := s1.ReplaceLifecycleActivation(ctx, insertReq); err != nil {
		t.Fatalf("seed live texture run: %v", err)
	}
	passivated := live
	passivated.State = types.RunPassivated
	passivated.Metadata = map[string]any{}
	for k, v := range live.Metadata {
		passivated.Metadata[k] = v
	}
	passivated.Metadata["passivated_reason"] = "runtime_restarted"
	if err := s1.UpdateRun(ctx, passivated); err != nil {
		t.Fatalf("passivate texture run: %v", err)
	}
	if err := s1.CreateAgentMutation(ctx, store.AgentMutation{
		DocID: docID, RunID: passivatedID, OwnerID: ownerID, ComputerID: computerID,
		State: "stale_activation", RevisionID: revisionID, CreatedAt: now,
	}); err != nil {
		t.Fatalf("seed stale_activation mutation: %v", err)
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
		ComputerID: computerID, StorePath: dbPath,
		PromptRoot: filepath.Join(t.TempDir(), "prompts"),
		ProviderTimeout: time.Second, SupervisionInterval: time.Hour,
	}, s2, events.NewEventBus(), provider.NewStubProvider(0))
	dispatches := 0
	rt.SetDispatchActor(func(context.Context, string, string, string, string, string, string, string) error {
		dispatches++
		return nil
	})
	t.Cleanup(rt.Stop)

	if err := NewHandler(rt).Start(ctx); err != nil {
		t.Fatalf("texture owner start: %v", err)
	}

	runs, err := s2.ListLifecycleRunsByChannel(ctx, ownerID, computerID, docID, 10)
	if err != nil {
		t.Fatalf("list lifecycle runs: %v", err)
	}
	var run *types.RunRecord
	for i := range runs {
		if runs[i].RunID == passivatedID {
			run = &runs[i]
		}
	}
	if run == nil {
		t.Fatalf("passivated texture run not listed")
	}
	if run.State != types.RunPassivated {
		t.Fatalf("boot re-armed an open-work texture run with no executor; state=%s", run.State)
	}
	if len(runs) != 1 {
		t.Fatalf("boot minted a run beside the passivated one: %d runs", len(runs))
	}
	if pending, err := s2.GetPendingAgentMutationByDoc(ctx, ownerID, computerID, docID); err != nil || pending != nil {
		t.Fatalf("boot re-armed the document mutation (agent_revision_pending): %+v err=%v", pending, err)
	}
	if dispatches != 0 {
		t.Fatalf("boot dispatched %d occurrences for open work alone", dispatches)
	}
}

func TestTextureOwnerRevisionRejectsTerminalLifecycleWithoutDispatch(t *testing.T) {
	core, handler := testAPISetup(t)
	start := startObservationLifecycle(t, core.Store())
	var dispatches int
	core.SetDispatchActor(func(context.Context, string, string, string, string, string, string, string) error {
		dispatches++
		return nil
	})
	snapshot, err := core.Store().GetLifecycleSnapshot(t.Context(), start.OwnerID, start.ComputerID, start.TrajectoryID)
	if err != nil {
		t.Fatal(err)
	}
	cancel := types.CancelLifecycleRequest{
		OwnerID: start.OwnerID, ComputerID: start.ComputerID, CommandID: "cancel-terminal-texture-owner-revision",
		TrajectoryID: start.TrajectoryID, ExpectedLifecycleVersion: snapshot.Trajectory.LifecycleVersion,
		ExpectedHeadRevisionID: snapshot.HeadRevision.RevisionID, Reason: "terminal revision regression",
	}
	cancel.CommandDigest, _ = store.ComputeCancelLifecycleDigest(cancel)
	if _, err := core.Store().CancelLifecycleTrajectory(t.Context(), cancel); err != nil {
		t.Fatal(err)
	}
	response := postOwnerInstruction(t, handler, "/api/texture/documents/"+start.InitialDocument.DocID+"/revise", start.OwnerID, "terminal-owner-revision", "must not revive terminal lifecycle", snapshot.HeadRevision.RevisionID)
	if response.Code != http.StatusConflict || dispatches != 0 {
		t.Fatalf("terminal owner revision status=%d dispatches=%d body=%s", response.Code, dispatches, response.Body.String())
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
}

func TestTextureOwnerRevisionRejectsCancellationIntentWithoutDispatch(t *testing.T) {
	core, handler := testAPISetup(t)
	start := startObservationLifecycle(t, core.Store())
	var dispatches int
	core.SetDispatchActor(func(context.Context, string, string, string, string, string, string, string) error {
		dispatches++
		return nil
	})
	snapshot, err := core.Store().GetLifecycleSnapshot(t.Context(), start.OwnerID, start.ComputerID, start.TrajectoryID)
	if err != nil {
		t.Fatal(err)
	}
	cancel := types.CancelLifecycleRequest{
		OwnerID: start.OwnerID, ComputerID: start.ComputerID, CommandID: "prepare-terminal-texture-owner-revision",
		TrajectoryID: start.TrajectoryID, ExpectedLifecycleVersion: snapshot.Trajectory.LifecycleVersion,
		ExpectedHeadRevisionID: snapshot.HeadRevision.RevisionID, Reason: "prepared cancellation",
	}
	cancel.CommandDigest, _ = store.ComputeCancelLifecycleDigest(cancel)
	if _, err := core.Store().PrepareLifecycleCancellation(t.Context(), cancel); err != nil {
		t.Fatal(err)
	}
	response := postOwnerInstruction(t, handler, "/api/texture/documents/"+start.InitialDocument.DocID+"/revise", start.OwnerID, "prepared-cancel", "do not outrun cancellation", snapshot.HeadRevision.RevisionID)
	if response.Code != http.StatusConflict || dispatches != 0 {
		t.Fatalf("prepared cancellation owner revision status=%d dispatches=%d body=%s", response.Code, dispatches, response.Body.String())
	}
	run, err := handler.ReconcileAgentWake(t.Context(), start.OwnerID, start.InitialDocument.DocID)
	if err != nil || run != nil {
		t.Fatalf("prepared cancellation wake run=%+v err=%v", run, err)
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

func TestTextureOwnerRevisionRejectsMissingOpenWorkWithoutDispatch(t *testing.T) {
	core, handler := testAPISetup(t)
	start := startObservationLifecycle(t, core.Store())
	var dispatches int
	core.SetDispatchActor(func(context.Context, string, string, string, string, string, string, string) error {
		dispatches++
		return nil
	})
	refuse := types.RefuseLifecycleWorkRequest{
		OwnerID: start.OwnerID, ComputerID: start.ComputerID, CommandID: "refuse-live-texture-work",
		TrajectoryID: start.TrajectoryID, WorkItemID: start.InitialWork.WorkItemID, ActingAgentID: start.Agent.AgentID,
		RefusalRef: "refusal://live/missing-work", Reason: "test live no-work authority",
	}
	refuse.CommandDigest, _ = store.ComputeRefuseLifecycleWorkDigest(refuse)
	if _, err := core.Store().RefuseLifecycleWork(t.Context(), refuse); err != nil {
		t.Fatal(err)
	}
	response := postOwnerInstruction(t, handler, "/api/texture/documents/"+start.InitialDocument.DocID+"/revise", start.OwnerID, "live-missing-work", "must not invent live work", start.InitialRevision.RevisionID)
	if response.Code != http.StatusConflict || dispatches != 0 {
		t.Fatalf("missing-work owner revision status=%d dispatches=%d body=%s", response.Code, dispatches, response.Body.String())
	}
	run, err := handler.ReconcileAgentWake(t.Context(), start.OwnerID, start.InitialDocument.DocID)
	if err != nil || run != nil {
		t.Fatalf("live no-work wake run=%+v err=%v", run, err)
	}
}

func TestTextureOwnerStartBypassesStaleTerminalActiveRunID(t *testing.T) {
	core, handler := testAPISetup(t)
	start := startObservationLifecycle(t, core.Store())
	ctx := t.Context()

	runID := "stale-run-123"
	now := time.Now().UTC()
	staleRun := types.RunRecord{
		RunID: runID, AgentID: start.Agent.AgentID, ChannelID: start.InitialDocument.DocID,
		TrajectoryID: start.TrajectoryID, AgentProfile: "texture", AgentRole: "texture",
		OwnerID: start.OwnerID, ComputerID: start.ComputerID, State: types.RunPassivated,
		CreatedAt: now, UpdatedAt: now,
		Metadata: map[string]any{"type": "texture_revision"},
	}
	replaceReq := types.ReplaceLifecycleActivationRequest{
		OwnerID: start.OwnerID, ComputerID: start.ComputerID, CommandID: "replace-stale-act",
		TrajectoryID: start.TrajectoryID, AgentID: start.Agent.AgentID, Run: staleRun,
	}
	replaceReq.CommandDigest, _ = store.ComputeReplaceLifecycleActivationDigest(replaceReq)
	if _, err := core.Store().ReplaceLifecycleActivation(ctx, replaceReq); err != nil {
		t.Fatalf("replace lifecycle activation: %v", err)
	}

	// Start() should cleanly bypass the stale passivated ActiveRunID instead of returning
	// fatal error "boot Texture run is not exact canonical authority".
	if err := handler.Start(ctx); err != nil {
		t.Fatalf("handler.Start() failed with stale passivated ActiveRunID: %v", err)
	}
}
