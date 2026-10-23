package store

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// Test 35. A crash between the lease CAS and the local finalize, then a
// replay, seeds above: the lease the platform accepted is what the next boot
// reads, and no number handed out under the old R is at or above it.
// Ways it can fail, written before the code:
//  1. The refill raises the in-memory R when the CAS landed but the finalize
//     failed: a number is handed out above a lease the local store does not
//     hold, and a restart that replays only to the local head reissues it.
//  2. After the replay the lease row is absent, so the next boot takes the
//     exact seed (correct but slow) or, worse, a stale trusted row.
func TestCrashBetweenTheLeaseCASAndFinalizeSeedsAbove(t *testing.T) {
	shrinkLease(t, 8)
	ctx := leaseCtx()
	h := newTapeHarness(t)
	var fail atomic.Bool
	h.setFinalizeFail(&fail)

	first := h.event("a-0", "run-a", "owner-1", types.EventRunStarted, eventTableBase)
	if err := h.store.AppendEvent(ctx, first); err != nil {
		t.Fatal(err)
	}
	handed := []int64{first.StreamSeq}
	before, found := readLeaseQuiet(h.store)
	if !found {
		t.Fatal("no lease after the first allocation")
	}

	// From here the platform accepts a lease batch whose local commit fails:
	// the refill at the half-block trigger. Every number taken, stored or not,
	// was published, so every one counts.
	fail.Store(true)
	for i := 1; i <= 6; i++ {
		rec := h.event(fmt.Sprintf("a-%d", i), "run-a", "owner-1", types.EventRunStarted, eventTableBase)
		_ = h.store.AppendEvent(ctx, rec)
		if rec.StreamSeq != 0 {
			handed = append(handed, rec.StreamSeq)
		}
	}
	for deadline := time.Now().Add(5 * time.Second); leaseStats(h.store).RefillFailed == 0; {
		if time.Now().After(deadline) {
			t.Fatalf("the refill never failed locally: %+v", leaseStats(h.store))
		}
		time.Sleep(10 * time.Millisecond)
	}
	for _, n := range handed {
		if n > before.Reserved {
			t.Fatalf("number %d handed out above the lease %d the local store held", n, before.Reserved)
		}
	}
	fail.Store(false)

	// The process dies. The platform tape holds the refill; the local store
	// holds it prepared. The next boot recovers it before anything runs.
	h.reopen()
	h.appender.SetPayloadResolver(tapePayloads{tape: h.tape}, nil)
	if err := h.appender.RecoverPrepared(ctx); err != nil {
		t.Fatalf("recover prepared: %v", err)
	}
	after, found := readLeaseQuiet(h.store)
	if !found || after.Reserved < before.Reserved {
		t.Fatalf("after the replay the lease row is %+v found=%v, want at least R=%d", after, found, before.Reserved)
	}
	next := h.event("after", "run-a", "owner-1", types.EventRunStarted, eventTableBase)
	if err := h.store.AppendEvent(ctx, next); err != nil {
		t.Fatal(err)
	}
	for _, n := range handed {
		if next.StreamSeq <= n {
			t.Fatalf("stream_seq %d after the crash reissued %d", next.StreamSeq, n)
		}
	}
	if next.StreamSeq <= after.Reserved {
		t.Fatalf("stream_seq %d is not above the recovered lease %d", next.StreamSeq, after.Reserved)
	}
}
