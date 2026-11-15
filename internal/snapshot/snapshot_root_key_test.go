package snapshot

// Stage 0 of docs/design/snapshots-cost-the-change-2026-11-13.md: the host
// decides which snapshot roots the tape names by the receipt key
// 'snapshot:'||root (internal/platform pruneSnapshotRoots, RecoveryBase). The
// key the guest really writes must be that string, and the root must be
// registered before the event is appended.

import (
	"context"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

type orderedChunks struct {
	*memChunks
	tape  *fakeTape
	roots []string
	seen  []int // events on the tape when each root was registered
}

func (o *orderedChunks) RegisterSnapshotRoot(ctx context.Context, root string, head uint64, chunks []string) error {
	o.roots = append(o.roots, root)
	o.seen = append(o.seen, len(o.tape.events))
	return o.memChunks.RegisterSnapshotRoot(ctx, root, head, chunks)
}

func TestSnapshotRecordedKeyIsTheRegisteredRootTheHostMatches(t *testing.T) {
	f := newFixture(t, ChunkSize)
	oc := &orderedChunks{memChunks: f.chunks, tape: f.tape}
	f.cfg.Chunks = oc
	if _, err := TakeSnapshot(context.Background(), f.cfg); err != nil {
		t.Fatal(err)
	}
	if len(oc.roots) != 1 || len(f.tape.events) != 1 {
		t.Fatalf("registered %d roots and appended %d events, want 1 and 1", len(oc.roots), len(f.tape.events))
	}
	if oc.seen[0] != 0 {
		t.Fatalf("the root was registered after the append (%d events on the tape)", oc.seen[0])
	}
	event := f.tape.events[0].Request.Event
	if event.EventKind != computerevent.EventSnapshotRecorded || event.IdempotencyKey != "snapshot:"+oc.roots[0] {
		t.Fatalf("event %s key %q, want snapshot_recorded with key %q", event.EventKind, event.IdempotencyKey, "snapshot:"+oc.roots[0])
	}
}
