package snapshot

import (
	"context"
	"database/sql"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
)

// Ways a failed attempt could still cost the host a store's worth of chunks
// (the paper's failed snapshots wrote 6 GiB an hour, 2026-11-12), written
// before the code:
//
//  W1. the witness runs after the page stream, so an attempt that cannot
//      finish its witness inside the budget has already uploaded the store;
//  W2. moving the witness first lets it describe a different state than the
//      pages (it must run in the same read transaction, before the pages).

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

// W2: the witness finishes before the first page is read, in the same
// transaction as the head and the pages, and a write that commits between the
// witness and the stream stays out of both.
func TestTheWitnessRunsBeforeThePagesInTheSameTransaction(t *testing.T) {
	f := newFixture(t, ChunkSize*3)
	live := liveWitness(t, f)
	var witnessDone atomic.Bool
	inner := f.cfg.Witness
	f.cfg.Witness = func(ctx context.Context, tx *sql.Tx) (selfdevprotocol.VMLocalContentWitness, error) {
		w, err := inner(ctx, tx)
		// A commit after the witness read: the pages must not carry it.
		if _, werr := f.db.Write.Exec(`INSERT INTO blobs (id, body, note) VALUES (?, ?, ?)`, 1_000_000, []byte("late"), "late"); werr != nil {
			t.Error(werr)
		}
		witnessDone.Store(true)
		return w, err
	}
	var pagesBeforeWitness atomic.Int64
	f.cfg.pageHook = func(int64) {
		if !witnessDone.Load() {
			pagesBeforeWitness.Add(1)
		}
	}
	ref, err := TakeSnapshot(context.Background(), f.cfg)
	if err != nil {
		t.Fatal(err)
	}
	if n := pagesBeforeWitness.Load(); n != 0 {
		t.Fatalf("%d pages were read before the witness finished", n)
	}
	if ref.Record.WitnessRoot != live.ContentRoot {
		t.Fatalf("recorded witness %s, the store at the snapshot point had %s", ref.Record.WitnessRoot, live.ContentRoot)
	}
	restored, _ := restoreAndOpen(t, f)
	got, err := witnessOf(context.Background(), restored.Read)
	if err != nil {
		t.Fatal(err)
	}
	if got.ContentRoot != ref.Record.WitnessRoot {
		t.Fatalf("restored witness %s differs from the recorded %s: pages and witness are not one state", got.ContentRoot, ref.Record.WitnessRoot)
	}
}
