package textureowner

import (
	"context"
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
		OwnerID: ownerID, ComputerID: "sandbox-texture-restart", CommandID: "start-texture-restart",
		TrajectoryID: "trajectory-texture-restart", Kind: types.TrajectoryKindDocument,
		SettlementRule: types.SettlementRule{Version: types.LifecycleReducerVersion, RequireNoOpenWorkItems: true, RequiredSubjectRefs: []string{"artifact"}},
		SubjectRefs:    map[string]string{"artifact": "texture://documents/" + docID},
		InitialWork: types.WorkItemRecord{
			WorkItemID: "work-texture-restart", Objective: "incorporate durable finding", AssignedAgentID: agentID,
		},
		InitialDocument: types.Document{DocID: docID, Title: "Restart target"},
		InitialRevision: types.Revision{
			RevisionID: "rev-texture-restart", AuthorKind: types.AuthorUser, AuthorLabel: "user",
			Content: "Durable content before restart",
		},
		Agent: types.AgentRecord{
			AgentID: agentID, OwnerID: ownerID, ComputerID: "sandbox-texture-restart", SandboxID: "sandbox-texture-restart",
			Profile: "texture", Role: "texture", ChannelID: docID, CreatedAt: now, UpdatedAt: now,
		},
	}
	start.StartRequestDigest, _ = store.ComputeStartLifecycleRequestDigest(start)
	if _, err := s1.StartLifecycle(ctx, start); err != nil {
		t.Fatalf("start durable lifecycle: %v", err)
	}
	packet := types.CoagentSourcePacketPayload{
		SchemaVersion: types.CoagentSourcePacketSchemaV1, Kind: "evidence_update", Summary: "durable finding",
	}
	payloadDigest, _ := store.ComputeLifecycleUpdatePayloadDigest(packet, "Durable finding")
	queue := types.QueueLifecycleUpdateRequest{
		OwnerID: ownerID, ComputerID: "sandbox-texture-restart", CommandID: "queue-texture-restart",
		TrajectoryID: start.TrajectoryID, TargetAgentID: agentID,
		ProducerAgentID: "researcher:texture-restart", ProducerUpdateID: "update-texture-restart",
		UpdateID: "update-texture-restart", ChannelID: docID, Role: "researcher",
		Packet: packet, Content: "Durable finding", PayloadDigest: payloadDigest,
	}
	queue.CommandDigest, _ = store.ComputeQueueLifecycleUpdateDigest(queue)
	if _, err := s1.QueueLifecycleUpdate(ctx, queue); err != nil {
		t.Fatalf("queue durable update: %v", err)
	}
	if err := s1.CreateAgentMutation(ctx, store.AgentMutation{
		DocID: docID, RunID: "orphan-preprojection-run", OwnerID: ownerID,
		ComputerID: "sandbox-texture-restart", State: "pending", CreatedAt: now,
	}); err != nil {
		t.Fatalf("create orphan pre-projection mutation: %v", err)
	}
	if err := s1.CreateAgentMutation(ctx, store.AgentMutation{
		DocID: docID, RunID: "orphan-preprojection-run-newer", OwnerID: ownerID,
		ComputerID: "sandbox-texture-restart", State: "pending", CreatedAt: now.Add(time.Second),
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
		SandboxID:           "sandbox-texture-restart",
		StorePath:           dbPath,
		PromptRoot:          filepath.Join(t.TempDir(), "prompts"),
		ProviderTimeout:     time.Second,
		SupervisionInterval: time.Hour,
	}, s2, events.NewEventBus(), provider.NewStubProvider(0))
	rt.SetDispatchActor(func(context.Context, string, string, string, string, string, string, string) error { return nil })
	t.Cleanup(rt.Stop)

	NewHandler(rt).Start(ctx)
	runs, err := s2.ListLifecycleRunsByOwner(ctx, ownerID, "sandbox-texture-restart", 20)
	if err != nil {
		t.Fatalf("list recovered runs: %v", err)
	}
	for _, run := range runs {
		if run.AgentID == agentID && run.ChannelID == docID && run.State == types.RunPending {
			for _, orphanRunID := range []string{"orphan-preprojection-run", "orphan-preprojection-run-newer"} {
				orphan, orphanErr := s2.GetAgentMutationByRun(ctx, ownerID, "sandbox-texture-restart", orphanRunID)
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
			SubjectRefs: map[string]string{"artifact": "texture://documents/" + docID},
			InitialWork: types.WorkItemRecord{
				WorkItemID: "work-shared-restart", Objective: "incorporate scoped update", AssignedAgentID: agentID,
			},
			InitialDocument: types.Document{DocID: docID, Title: "Scoped restart target"},
			InitialRevision: types.Revision{
				RevisionID: "revision-shared-restart", AuthorKind: types.AuthorUser, AuthorLabel: ownerID, Content: "Initial scoped content",
			},
			Agent: types.AgentRecord{
				AgentID: agentID, OwnerID: ownerID, ComputerID: computerID, SandboxID: computerID,
				Profile: "texture", Role: "texture", ChannelID: docID, CreatedAt: now, UpdatedAt: now,
			},
		}
		start.StartRequestDigest, _ = store.ComputeStartLifecycleRequestDigest(start)
		if _, err := s1.StartLifecycle(ctx, start); err != nil {
			t.Fatalf("start lifecycle for %s: %v", computerID, err)
		}
	}
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
		TrajectoryID: trajectoryID, TargetAgentID: agentID, ProducerAgentID: "researcher:computer-b",
		ProducerUpdateID: "update-computer-b", UpdateID: "update-computer-b", ChannelID: docID,
		Role: "researcher", Packet: packet, Content: "computer B update", PayloadDigest: payloadDigest,
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
		SandboxID: "computer-b", StorePath: dbPath, PromptRoot: filepath.Join(t.TempDir(), "prompts"),
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
