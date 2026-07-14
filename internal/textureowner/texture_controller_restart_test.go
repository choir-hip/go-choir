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
	if err := s1.CreateDocument(ctx, types.Document{
		DocID: docID, OwnerID: ownerID, Title: "Restart target", CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create document: %v", err)
	}
	if err := s1.CreateRevision(ctx, types.Revision{
		RevisionID:  "rev-texture-restart",
		DocID:       docID,
		OwnerID:     ownerID,
		AuthorKind:  types.AuthorUser,
		AuthorLabel: "user",
		Content:     "Durable content before restart",
		CreatedAt:   now,
	}); err != nil {
		t.Fatalf("create revision: %v", err)
	}
	update := types.CoagentSourcePacket{
		UpdateID:      "update-texture-restart",
		OwnerID:       ownerID,
		AgentID:       "researcher:texture-restart",
		TargetAgentID: agentID,
		ChannelID:     docID,
		Role:          "researcher",
		Packet: types.CoagentSourcePacketPayload{
			SchemaVersion: types.CoagentSourcePacketSchemaV1,
			Kind:          "evidence_update",
			Summary:       "durable finding",
		},
		Content:   "Durable finding",
		CreatedAt: now.Add(time.Millisecond),
	}
	message := types.ChannelMessage{
		ChannelID:   docID,
		FromAgentID: update.AgentID,
		ToAgentID:   agentID,
		Role:        update.Role,
		Content:     update.Content,
		Timestamp:   update.CreatedAt,
	}
	if _, _, err := s1.DispatchWorkerUpdate(ctx, update, &message); err != nil {
		t.Fatalf("dispatch durable update: %v", err)
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
	rt.SetDispatchActor(func(context.Context, string, string, string, string, string) error { return nil })
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
