package snapshot

import (
	"context"
	"errors"
	"testing"
)

// A cancel that lands in the file-tree root lookup is a cancellation, not a
// failure: the caller's cause must survive (the producer tells a restart from
// a failure by it).
func TestACancelInTheFileTreeRootLookupKeepsTheCallersCause(t *testing.T) {
	f := newFixture(t, ChunkSize)
	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)
	f.cfg.FileTreeRoot = func(c context.Context) (string, error) {
		cancel(errTestRestart)
		return "", c.Err()
	}
	_, err := TakeSnapshot(ctx, f.cfg)
	if !errors.Is(err, errTestRestart) {
		t.Fatalf("err = %v, want the caller's cause", err)
	}
	if len(f.tape.events) != 0 || len(f.chunks.roots) != 0 {
		t.Fatalf("left %d events and %d roots", len(f.tape.events), len(f.chunks.roots))
	}
}
