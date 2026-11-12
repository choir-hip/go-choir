package store

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/computerevent"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// The platform accepts an append, the caller's context is cancelled while the
// reply is in flight (the paper, 2026-11-12). The appender re-sends the same
// request and the platform answers with the stored receipt; the SQLite
// projection must then finalize on a context that is not the cancelled
// caller's (FinalizeBatch begins its transaction on the ctx it is given).
func TestAppendWithACancelledCallerAfterThePlatformCommittedConverges(t *testing.T) {
	h := newTapeHarness(t)
	before, err := h.store.Head(context.Background(), h.computerID)
	if err != nil || before == nil {
		t.Fatalf("head before: %v %v", before, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var fired atomic.Bool
	h.tape.mu.Lock()
	h.tape.casAfter = func(request computerevent.CASRequest) error {
		h.tape.mu.Lock()
		lease := h.tape.leaseDigests[request.Event.PayloadCommitment]
		h.tape.mu.Unlock()
		if lease {
			return nil // the stream_seq lease's own batch is not the append under test
		}
		if fired.CompareAndSwap(false, true) {
			cancel()
			return context.Canceled
		}
		return nil
	}
	h.tape.mu.Unlock()

	if err := h.store.AppendEvent(ctx, h.event("lost-reply", "run-lost", "owner-1", types.EventRunCompleted, telemetryBase)); err != nil {
		t.Fatalf("append whose reply was lost to a cancelled caller: %v", err)
	}
	if !fired.Load() {
		t.Fatal("fixture: the reply was never lost")
	}
	after, err := h.store.Head(context.Background(), h.computerID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Sequence != before.Sequence+1 {
		t.Fatalf("local head %d, want %d", after.Sequence, before.Sequence+1)
	}
	prepared, err := h.store.Prepared(context.Background(), h.computerID)
	if err != nil || len(prepared) != 0 {
		t.Fatalf("prepared rows after the append: %d %v", len(prepared), err)
	}
	if st := h.appender.WedgeStatus(); st.Active {
		t.Fatalf("wedge tripped: %+v", st)
	}
}
