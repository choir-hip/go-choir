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
		SettlementRule: types.SettlementRule{RequireNoOpenWorkItems: true, RequiredSubjectRefs: []string{"artifact"}},
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
	runs, err := s2.ListRunsByOwner(ctx, ownerID, 20)
	if err != nil {
		t.Fatalf("list recovered runs: %v", err)
	}
	for _, run := range runs {
		if run.AgentID == agentID && run.ChannelID == docID && run.State == types.RunPending {
			return
		}
	}
	t.Fatalf("durable Texture wake did not create a pending owner run after restart: %+v", runs)
}
