package store

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func rmEntry(id, run string) types.RunMemoryEntry {
	return types.RunMemoryEntry{EntryID: id, RunID: run, OwnerID: "owner-1", Kind: types.RunMemoryEntryMessage, Role: "user", Message: json.RawMessage(`{"role":"user","content":"x"}`)}
}

// Two runs share a group whose CAS outcome is unknown after the host
// committed: both fail, one kept prepared row holds both entries. The first
// run's settle is blocked (the appender is held); the second run retries. It
// must wait for the settle, then number itself from a table that has the
// landed rows: no duplicate (run, seq) on the tape, no wedge, dense seqs
// (review of 6a3a8e4b3: the latch was cleared before the settle finished).
func TestRunMemoryRetryOfASecondRunWaitsForTheFirstRunsSettle(t *testing.T) {
	h := newTapeHarness(t)
	h.hostChain = &hostChain{}
	h.bind()
	h.warm()
	ctx := context.Background()
	runs := distinctRunMemoryRuns(h.store, 2)
	a, b := runs[0], runs[1]

	gate := h.holdNextCAS()
	results := make(chan writeResult, 8)
	go func() {
		results <- writeResult{"lead", h.store.AppendEvent(ctx, h.event("lead", "run-lead", "owner-1", types.EventRunCompleted, telemetryBase))}
	}()
	<-gate.entered
	for i, c := range []struct{ id, run string }{{"a-1", a}, {"b-1", b}} {
		c := c
		go func() {
			_, err := h.store.AppendRunMemoryEntry(ctx, rmEntry(c.id, c.run))
			results <- writeResult{c.id, err}
		}()
		waitFor(t, c.id+" pending", 5*time.Second, func() bool { return h.pendingMembers() >= i+1 })
	}
	// The lead's own CAS passes; the group's first send and its re-send lose
	// their replies after the host committed.
	var calls atomic.Int32
	h.tape.mu.Lock()
	h.tape.casAfter = func(computerevent.CASRequest) error {
		if calls.Add(1) == 1 {
			return nil
		}
		if calls.Load() <= 3 {
			return errors.New("reply lost")
		}
		return nil
	}
	h.tape.mu.Unlock()
	close(gate.release)
	failed := 0
	for i := 0; i < 3; i++ {
		r := <-results
		if r.err != nil {
			failed++
		} else if r.id != "lead" {
			t.Fatalf("%s was acknowledged although the group's outcome was unknown", r.id)
		}
	}
	if failed != 2 {
		t.Fatalf("%d members failed, want 2", failed)
	}

	// Hold the appender so the first settle cannot finish, then retry both.
	held, releaseHold := make(chan struct{}), make(chan struct{})
	holdDone := make(chan struct{})
	go func() {
		defer close(holdDone)
		_ = h.appender.HoldAppends(func() error { close(held); <-releaseHold; return nil })
	}()
	<-held
	go func() {
		_, err := h.store.AppendRunMemoryEntry(ctx, rmEntry("a-2", a))
		results <- writeResult{"a-2", err}
	}()
	time.Sleep(200 * time.Millisecond) // a-2 is inside its settle, blocked on the appender
	go func() {
		_, err := h.store.AppendRunMemoryEntry(ctx, rmEntry("b-2", b))
		results <- writeResult{"b-2", err}
	}()
	time.Sleep(200 * time.Millisecond)
	close(releaseHold)
	<-holdDone
	for i := 0; i < 2; i++ {
		select {
		case r := <-results:
			if r.err != nil {
				t.Fatalf("%s: %v", r.id, r.err)
			}
		case <-time.After(20 * time.Second):
			t.Fatal("a retry never returned")
		}
	}
	if st := h.appender.WedgeStatus(); st.Active {
		t.Fatalf("wedge tripped: %+v", st)
	}
	for _, run := range []string{a, b} {
		entries, err := h.store.ListRunMemoryEntries(ctx, "owner-1", run)
		if err != nil {
			t.Fatal(err)
		}
		for i, e := range entries {
			if e.Seq != int64(i+1) {
				t.Fatalf("run %s: entry %s has seq %d at position %d: not dense: %+v", run, e.EntryID, e.Seq, i+1, entries)
			}
		}
		if len(entries) != 2 {
			t.Fatalf("run %s has %d entries, want 2", run, len(entries))
		}
	}
}

// A panic through the append must leave the settle latch set, like an error:
// the panic may have left a prepared row.
func TestRunMemoryAppendPanicSetsTheSettleLatch(t *testing.T) {
	h := newTapeHarness(t)
	h.warm()
	h.tape.mu.Lock()
	h.tape.casErr = func(computerevent.CASRequest) error { panic("injected panic in the append") }
	h.tape.mu.Unlock()
	func() {
		defer func() { _ = recover() }()
		_, _ = h.store.AppendRunMemoryEntry(context.Background(), rmEntry("rm-panic", "run-panic"))
		t.Fatal("fixture: the append did not panic")
	}()
	if !h.store.runMemorySettle.Load() {
		t.Fatal("the settle latch is not set after a panic through the append")
	}
}
