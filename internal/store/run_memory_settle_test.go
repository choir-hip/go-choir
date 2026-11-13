package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	refused := map[string]string{}
	for i := 0; i < 2; i++ {
		select {
		case r := <-results:
			// A retry numbered before the landed rows surfaced may be refused
			// (ErrBatchRefused, nothing sent); its next attempt succeeds.
			if r.err != nil && !errors.Is(r.err, computerevent.ErrBatchRefused) {
				t.Fatalf("%s: %v", r.id, r.err)
			} else if r.err != nil {
				refused[r.id] = map[string]string{"a-2": a, "b-2": b}[r.id]
			}
		case <-time.After(20 * time.Second):
			t.Fatal("a retry never returned")
		}
	}
	for id, run := range refused {
		retryRunMemory(t, h, id, run)
	}
	assertDenseRuns(t, h, a, b)
}

// retryRunMemory appends the entry, retrying while the appender refuses a
// batch numbered before landed rows surfaced (at most four attempts).
func retryRunMemory(t *testing.T, h *tapeHarness, id, run string) {
	t.Helper()
	var err error
	for attempt := 0; attempt < 4; attempt++ {
		entryID := fmt.Sprintf("%s-try%d", id, attempt)
		if _, err = h.store.AppendRunMemoryEntry(context.Background(), rmEntry(entryID, run)); err == nil || !errors.Is(err, computerevent.ErrBatchRefused) {
			break
		}
	}
	if err != nil {
		t.Fatalf("%s: %v", id, err)
	}
}

func assertDenseRuns(t *testing.T, h *tapeHarness, runs ...string) {
	t.Helper()
	ctx := context.Background()
	if st := h.appender.WedgeStatus(); st.Active {
		t.Fatalf("wedge tripped: %+v", st)
	}
	for _, run := range runs {
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


// B2's interleaving: two runs, both with landed-unacknowledged entries (one
// kept prepared row for the group), retry at the same time, repeatedly. With no
// latch to erase, the appender's guard must keep every attempt either landed
// or refused: dense seqs, no wedge, no duplicate on the tape.
func TestRunMemoryConcurrentRetriesAfterAnUnclearGroupNeverDuplicateSeq(t *testing.T) {
	for round := 0; round < 6; round++ {
		h := newTapeHarness(t)
		h.hostChain = &hostChain{}
		h.bind()
		h.warm()
		ctx := context.Background()
		runs := distinctRunMemoryRuns(h.store, 2)
		gate := h.holdNextCAS()
		results := make(chan writeResult, 8)
		go func() {
			results <- writeResult{"lead", h.store.AppendEvent(ctx, h.event("lead", "run-lead", "owner-1", types.EventRunCompleted, telemetryBase))}
		}()
		<-gate.entered
		for i, run := range runs {
			id, run := fmt.Sprintf("r%d-1", i), run
			go func() {
				_, err := h.store.AppendRunMemoryEntry(ctx, rmEntry(id, run))
				results <- writeResult{id, err}
			}()
			waitFor(t, id+" pending", 5*time.Second, func() bool { return h.pendingMembers() >= i+1 })
		}
		var calls atomic.Int32
		h.tape.mu.Lock()
		h.tape.casAfter = func(computerevent.CASRequest) error {
			if n := calls.Add(1); n == 2 || n == 3 {
				return errors.New("reply lost")
			}
			return nil
		}
		h.tape.mu.Unlock()
		close(gate.release)
		for i := 0; i < 3; i++ {
			<-results
		}
		done := make(chan struct{}, 2)
		for i, run := range runs {
			id, run := fmt.Sprintf("r%d-2", i), run
			go func() {
				defer func() { done <- struct{}{} }()
				retryRunMemory(t, h, id, run)
			}()
		}
		for i := 0; i < 2; i++ {
			select {
			case <-done:
			case <-time.After(30 * time.Second):
				t.Fatal("a retry never returned")
			}
		}
		assertDenseRuns(t, h, runs...)
	}
}
