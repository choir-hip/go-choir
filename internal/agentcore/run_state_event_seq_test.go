package agentcore

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// TestRunStateEventPublishCarriesStoredSeq pins docs/problems/run-events-without-seq-2026-10-22.md
// at the runtime layer: for a lifecycle-bound run, the run-state event that
// persistActivationStateAndEmit folds into the lifecycle batch is published on
// the live bus with the same seq and stream_seq it is stored with, and a
// reader of the run's events from after=0 sees it.
func TestRunStateEventPublishCarriesStoredSeq(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	ownerID, docID := "owner-run-state-seq", "doc-run-state-seq"
	trajectoryID := seedDurableTextureSubject(t, s, ownerID, docID)
	now := time.Now().UTC()
	rec := types.RunRecord{
		RunID: "run-state-seq-1", AgentID: "texture:" + docID, ChannelID: docID,
		TrajectoryID: trajectoryID, AgentProfile: agentprofile.Texture, AgentRole: agentprofile.Texture,
		OwnerID: ownerID, ComputerID: rt.TextureComputerID(), State: types.RunRunning,
		CreatedAt: now, UpdatedAt: now,
		Metadata: map[string]any{runMetadataTrajectoryID: trajectoryID, runMetadataChannelID: docID},
	}
	if err := s.CreateRun(ctx, rec); err != nil {
		t.Fatalf("create lifecycle run: %v", err)
	}
	sub := rt.EventBus().SubscribeWithBuffer(16)
	defer rt.EventBus().Unsubscribe(sub)

	rec.State, rec.UpdatedAt = types.RunPassivated, now.Add(time.Second)
	persisted, err := rt.persistActivationStateAndEmit(ctx, &rec, types.EventRunPassivated, events.CauseTaskLifecycle, json.RawMessage(`{}`))
	if err != nil || !persisted {
		t.Fatalf("persist and emit: persisted=%t err=%v", persisted, err)
	}
	var published types.EventRecord
	select {
	case ev := <-sub:
		published = ev.Record
	case <-time.After(5 * time.Second):
		t.Fatal("no run-state event published on the bus")
	}
	t.Logf("[M] published: seq=%d stream_seq=%d", published.Seq, published.StreamSeq)
	if published.Seq <= 0 || published.StreamSeq <= 0 {
		t.Fatalf("published run-state event has seq=%d stream_seq=%d, want both > 0", published.Seq, published.StreamSeq)
	}
	stored, err := s.ListEventsAfter(ctx, rec.RunID, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range stored {
		if e.EventID == published.EventID {
			if e.Seq != published.Seq || e.StreamSeq != published.StreamSeq {
				t.Fatalf("stored seq=%d stream_seq=%d, published seq=%d stream_seq=%d", e.Seq, e.StreamSeq, published.Seq, published.StreamSeq)
			}
			return
		}
	}
	t.Fatalf("published event %s is not in ListEventsAfter(after=0): %+v", published.EventID, stored)
}
