package agentcore

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// docs/problems/research-more-slow-and-hidden-2026-10-21.md, E6: a signalled
// sweep dispatches the wakes its commit announced, read by key, without
// listing the outbox; a second sweep does not dispatch them again.
func TestSignalledSweepDispatchesAnnouncedWakesWithoutListing(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	const (
		ownerID = "user-actor-wake-announced"
		docID   = "doc-actor-wake-announced"
	)
	trajectoryID := seedDurableTextureSubject(t, s, ownerID, docID)
	targetAgentID := currentTextureAgentID(docID)
	producerAgentID, producerWorkID, producerRunID := projectTestLifecycleProducer(t, s, ownerID, "autoputer-test", trajectoryID, docID, "actor-wake-announced")
	setupWakes, err := s.ListUnprojectedActorWakes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, wake := range setupWakes {
		if err := s.MarkActorWakeProjected(ctx, wake.CanonicalID); err != nil {
			t.Fatalf("mark setup wake projected: %v", err)
		}
	}
	if _, _, err := s.TakeAnnouncedActorWakes(ctx); err != nil {
		t.Fatal(err)
	}
	packet := types.SourcePacket{SchemaVersion: types.CoagentSourcePacketSchemaV1, Kind: "evidence_update", Summary: "announced actor wake"}
	req := types.QueueLifecycleUpdateRequest{
		OwnerID: ownerID, ComputerID: "autoputer-test", CommandID: "queue-actor-wake-announced",
		TrajectoryID: trajectoryID, TargetAgentID: targetAgentID, ProducerAgentID: producerAgentID,
		ProducerUpdateID: "producer-actor-wake-announced", UpdateID: "update-actor-wake-announced",
		ChannelID: docID, Role: agentprofile.Research, SourceRunID: producerRunID,
		WorkItemID: producerWorkID, WorkDisposition: types.WorkItemOpen,
		Packet: packet, Content: "announced wake content",
	}
	req.PayloadDigest, _ = store.ComputeLifecycleUpdatePayloadDigest(req.Packet, req.Content)
	req.CommandDigest, _ = store.ComputeQueueLifecycleUpdateDigest(req)
	if _, err := s.QueueLifecycleUpdate(ctx, req); err != nil {
		t.Fatalf("queue lifecycle update: %v", err)
	}
	var dispatches atomic.Int32
	rt.SetDispatchActor(func(_ context.Context, owner, computer, target, kind, content, trajectory, from string) error {
		if target == targetAgentID && kind == "coagent_result" {
			dispatches.Add(1)
		}
		return nil
	})
	rt.sweepActorWakes(ctx, true)
	rt.sweepActorWakes(ctx, true)
	if got := dispatches.Load(); got != 1 {
		t.Fatalf("announced dispatches = %d, want exactly 1", got)
	}
	if wakes, _ := s.ListUnprojectedActorWakes(ctx); len(wakes) != 0 {
		t.Fatalf("unprojected wakes after the signalled sweep: %+v", wakes)
	}
}
