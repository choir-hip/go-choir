package agentcore

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestActorWakeOutboxProjectorDispatchesOnceAndMarksProjected(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	const (
		ownerID = "user-actor-wake-outbox"
		docID   = "doc-actor-wake-outbox"
	)
	trajectoryID := seedDurableTextureSubject(t, s, ownerID, docID)
	targetAgentID := currentTextureAgentID(docID)
	producerAgentID, producerWorkID, producerRunID := projectTestLifecycleProducer(t, s, ownerID, "autoputer-test", trajectoryID, docID, "actor-wake-outbox")
	setupWakes, err := s.ListUnprojectedActorWakes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, wake := range setupWakes {
		if err := s.MarkActorWakeProjected(ctx, wake.CanonicalID); err != nil {
			t.Fatalf("mark setup wake projected: %v", err)
		}
	}
	packet := types.CoagentSourcePacketPayload{SchemaVersion: types.CoagentSourcePacketSchemaV1, Kind: "evidence_update", Summary: "durable actor wake"}
	req := types.QueueLifecycleUpdateRequest{
		OwnerID: ownerID, ComputerID: "autoputer-test", CommandID: "queue-actor-wake-outbox",
		TrajectoryID: trajectoryID, TargetAgentID: targetAgentID, ProducerAgentID: producerAgentID,
		ProducerUpdateID: "producer-actor-wake-outbox", UpdateID: "update-actor-wake-outbox",
		ChannelID: docID, Role: agentprofile.Research, SourceRunID: producerRunID,
		WorkItemID: producerWorkID, WorkDisposition: types.WorkItemOpen,
		Packet: packet, Content: "actor wake content",
	}
	req.PayloadDigest, _ = store.ComputeLifecycleUpdatePayloadDigest(req.Packet, req.Content)
	req.CommandDigest, _ = store.ComputeQueueLifecycleUpdateDigest(req)
	if _, err := s.QueueLifecycleUpdate(ctx, req); err != nil {
		t.Fatalf("queue lifecycle update: %v", err)
	}

	var dispatches atomic.Int32
	rt.SetDispatchActor(func(_ context.Context, owner, computer, target, kind, content, trajectory, from string) error {
		if owner == ownerID && computer == "autoputer-test" && target == targetAgentID && kind == "coagent_result" &&
			content != "" && trajectory == trajectoryID && from == producerAgentID {
			dispatches.Add(1)
		}
		return nil
	})
	rt.sweepActorWakeOutbox(ctx)
	rt.sweepActorWakeOutbox(ctx)
	if got := dispatches.Load(); got != 1 {
		t.Fatalf("outbox projector dispatches = %d, want exactly 1", got)
	}
	wakes, err := s.ListUnprojectedActorWakes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(wakes) != 0 {
		t.Fatalf("unprojected actor wakes after projector = %+v", wakes)
	}
}
