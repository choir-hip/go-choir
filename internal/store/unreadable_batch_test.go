package store

import (
	"context"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

// The no-change stand-in for a sealed batch that does not parse
// (computerevent.ErrProjectionBatchUnreadable) finalizes on the real store:
// the head advances to its event and nothing else is written.
func TestFinalizeReplayBatchAdvancesHeadForUnreadableStandIn(t *testing.T) {
	ctx := context.Background()
	productStore := openProjectStore(t)
	computerID := "computer-unreadable-batch"
	prepareGenesis(t, productStore, computerID, "genesis-unreadable-batch")
	head, err := productStore.Head(ctx, computerID)
	if err != nil || head == nil {
		t.Fatalf("head: %v %#v", err, head)
	}
	event, digest := prepareProjectionEvent(t, productStore, *head, mustEventID(t), "unreadable-batch")
	batch := computerevent.ProjectionBatch{
		Version: computerevent.ProjectionBatchV2, ProjectorVersion: computerevent.ProjectorVersionV2,
		ComputerID: computerID, EventID: event.EventID, EventDigest: digest, Unreadable: true,
	}
	if err := productStore.FinalizeReplayBatch(ctx, computerID, digest, signedReceipt(t, computerID, digest, event.Sequence), &batch); err != nil {
		t.Fatal(err)
	}
	after, err := productStore.Head(ctx, computerID)
	if err != nil || after == nil || after.Sequence != event.Sequence || after.CanonicalEventHead != digest {
		t.Fatalf("head after = %#v (%v), want sequence %d at %s", after, err, event.Sequence, digest)
	}
}
