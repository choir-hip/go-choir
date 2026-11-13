package snapshot

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
)

// A planned restart cancels the capture through its context
// (docs/problems/snapshot-loop-blocks-its-own-fix-2026-11-12.md). Ways that
// could fail, written before the code:
//
//  C1. a cancel in the witness leaves the read transaction open (the WAL stays
//      pinned) or uploads chunks;
//  C2. a cancel in the page stream leaves a root or a snapshot_recorded event,
//      or hides the caller's cause behind a generic context error (the
//      producer must tell a restart from a failure);
//  C3. a cancel is read as the time budget.

var errTestRestart = errors.New("cancelled for a planned restart (test)")

func TestACancelInTheWitnessLeavesNothingAndKeepsTheCause(t *testing.T) {
	f := newFixture(t, ChunkSize*3)
	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)
	f.cfg.Witness = func(wctx context.Context, tx *sql.Tx) (selfdevprotocol.VMLocalContentWitness, error) {
		go func() { time.Sleep(50 * time.Millisecond); cancel(errTestRestart) }()
		<-wctx.Done()
		return selfdevprotocol.VMLocalContentWitness{}, wctx.Err()
	}
	_, err := TakeSnapshot(ctx, f.cfg)
	if !errors.Is(err, errTestRestart) || errors.Is(err, ErrTimeBudget) {
		t.Fatalf("err = %v, want the caller's cause and not the time budget", err)
	}
	f.chunks.mu.Lock()
	puts := len(f.chunks.puts)
	f.chunks.mu.Unlock()
	if puts != 0 || len(f.tape.events) != 0 || len(f.chunks.roots) != 0 {
		t.Fatalf("a cancelled witness left %d chunks, %d events, %d roots", puts, len(f.tape.events), len(f.chunks.roots))
	}
	assertWALReleased(t, f)
}

func TestACancelInThePageStreamLeavesNoRootNoEventAndReleasesTheWAL(t *testing.T) {
	f := newFixture(t, ChunkSize*3)
	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)
	pages := 0
	f.cfg.pageHook = func(int64) {
		pages++
		if pages == 600 {
			cancel(errTestRestart)
		}
	}
	_, err := TakeSnapshot(ctx, f.cfg)
	if !errors.Is(err, errTestRestart) || errors.Is(err, ErrTimeBudget) {
		t.Fatalf("err = %v, want the caller's cause and not the time budget", err)
	}
	if len(f.tape.events) != 0 || len(f.chunks.roots) != 0 {
		t.Fatalf("a cancelled stream left %d events and %d roots", len(f.tape.events), len(f.chunks.roots))
	}
	assertWALReleased(t, f)
}
