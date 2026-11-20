package snapshot

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
)

// Ways a failed attempt could still cost the host a store's worth of chunks
// (the paper's failed snapshots wrote 6 GiB an hour, 2026-11-12), written
// before the code:
//
//  W1. the witness runs after the upload, so an attempt that cannot finish its
//      witness inside the budget has already uploaded the store.
//
// W2 (the witness shares the pages' transaction) is back with S1 of
// docs/design/snapshot-without-scratch-copy-2026-11-20.md
// (snapshot_stream_capture_test.go).

// W1: the witness is the long step on the paper (22 to 35 minutes); a budget
// abort inside it must have uploaded nothing.
func TestABudgetAbortInTheWitnessUploadsNothing(t *testing.T) {
	f := newFixture(t, ChunkSize*3)
	f.cfg.MaxDuration = 200 * time.Millisecond
	f.cfg.Witness = func(ctx context.Context, tx *sql.Tx) (selfdevprotocol.VMLocalContentWitness, error) {
		<-ctx.Done()
		return selfdevprotocol.VMLocalContentWitness{}, ctx.Err()
	}
	_, err := TakeSnapshot(context.Background(), f.cfg)
	if !errors.Is(err, ErrTimeBudget) {
		t.Fatalf("err = %v, want ErrTimeBudget", err)
	}
	f.chunks.mu.Lock()
	puts, probes := len(f.chunks.puts), f.chunks.probes
	f.chunks.mu.Unlock()
	if puts != 0 || probes != 0 {
		t.Fatalf("an attempt that died in its witness uploaded %d chunks and probed %d", puts, probes)
	}
	if len(f.tape.events) != 0 || len(f.chunks.roots) != 0 {
		t.Fatalf("left %d events and %d roots", len(f.tape.events), len(f.chunks.roots))
	}
	assertWALReleased(t, f)
}
