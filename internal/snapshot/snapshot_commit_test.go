package snapshot

import (
	"context"
	"errors"
	"testing"
	"time"
)

// The point of no return. TakeSnapshot registers the root with the host, then
// appends snapshot_recorded. A cancel between the two would leave a root the
// tape does not know, so once BeforeCommit has said yes the two steps run to
// the end on a context the cancel cannot reach, bounded by CommitTimeout.
// Ways that could fail, written before the code:
//
//  N1. BeforeCommit is not asked, so the producer cannot refuse a late cancel;
//  N2. a refusal from BeforeCommit still registers a root or appends;
//  N3. a cancel after BeforeCommit answers yes cuts the register or the
//      append (root without event);
//  N4. the commit steps are unbounded when the host stops answering.

func TestBeforeCommitRefusalRegistersNothingAndAppendsNothing(t *testing.T) {
	f := newFixture(t, ChunkSize*2)
	asked := 0
	f.cfg.BeforeCommit = func() error { asked++; return errTestRestart }
	_, err := TakeSnapshot(context.Background(), f.cfg)
	if !errors.Is(err, errTestRestart) || asked != 1 {
		t.Fatalf("err = %v, asked %d times, want the refusal's error once", err, asked)
	}
	if len(f.tape.events) != 0 || len(f.chunks.roots) != 0 {
		t.Fatalf("a refused commit left %d events and %d roots", len(f.tape.events), len(f.chunks.roots))
	}
	assertWALReleased(t, f)
}

func TestACancelAfterBeforeCommitSaidYesStillRegistersAndAppendsBoth(t *testing.T) {
	f := newFixture(t, ChunkSize*2)
	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)
	f.cfg.BeforeCommit = func() error { cancel(errTestRestart); return nil }
	ref, err := TakeSnapshot(ctx, f.cfg)
	if err != nil {
		t.Fatalf("a cancel inside the commit cut it: %v", err)
	}
	if len(f.tape.events) != 1 || len(f.chunks.roots) != 1 || ref.EventID == "" {
		t.Fatalf("%d events and %d roots, want one each", len(f.tape.events), len(f.chunks.roots))
	}
}

func TestTheCommitIsBoundedWhenTheHostStopsAnswering(t *testing.T) {
	f := newFixture(t, ChunkSize*2)
	f.cfg.CommitTimeout = 200 * time.Millisecond
	f.cfg.BeforeCommit = func() error { return nil }
	f.chunks.registerHook = func(ctx context.Context) error { <-ctx.Done(); return ctx.Err() }
	start := time.Now()
	_, err := TakeSnapshot(context.Background(), f.cfg)
	if err == nil || time.Since(start) > 5*time.Second {
		t.Fatalf("err = %v after %s, want a bounded failure", err, time.Since(start))
	}
}
