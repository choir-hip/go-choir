package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Review findings of lane/s5-coalesce (both seats ACCEPT, should-fixes).

func rmOp(id, run string, seq int64) computerevent.ProjectionOp {
	body, _ := json.Marshal(computerevent.RunMemoryEntryProjection{
		EntryID: id, RunID: run, OwnerID: "owner-1", Seq: seq, Kind: "message", Role: "user",
		MessageJSON: `{"role":"user","content":"x"}`, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano),
	})
	return computerevent.ProjectionOp{Kind: computerevent.ProjectionOpRunMemoryEntry, CanonicalID: id, Body: body}
}

// A panic inside a leader must not leave the leader token full: every later
// writer would block for ever, and the members it had taken would never be
// answered.
func TestCoalescePanicInTheLeaderDoesNotWedgeTheTape(t *testing.T) {
	h := newTapeHarness(t)
	h.warm()
	gate := h.holdNextCAS()
	type res struct {
		id       string
		err      error
		panicked bool
	}
	results := make(chan res, 8)
	run := func(id string) {
		defer func() {
			if r := recover(); r != nil {
				results <- res{id: id, panicked: true}
			}
		}()
		err := h.store.AppendEvent(context.Background(), h.event(id, "run-"+id, "owner-1", types.EventRunCompleted, telemetryBase))
		results <- res{id: id, err: err}
	}
	go run("lead")
	<-gate.entered
	for i := 0; i < 3; i++ {
		go run(fmt.Sprintf("p%d", i))
		waitFor(t, "member pending", 5*time.Second, func() bool { return h.pendingMembers() >= i+1 })
	}
	h.tape.mu.Lock()
	h.tape.casErr = func(computerevent.CASRequest) error { panic("injected panic in the group's CAS") }
	h.tape.mu.Unlock()
	close(gate.release)
	panics := 0
	for i := 0; i < 4; i++ {
		select {
		case r := <-results:
			switch {
			case r.id == "lead":
				if r.err != nil || r.panicked {
					t.Fatalf("lead: %+v", r)
				}
			case r.panicked:
				panics++
			case r.err == nil:
				t.Fatalf("%s was acknowledged although the group's append panicked", r.id)
			}
		case <-time.After(10 * time.Second):
			t.Fatal("a taken member was never answered: the panic wedged the group")
		}
	}
	if panics > 1 {
		t.Fatalf("%d goroutines panicked, want only the leader", panics)
	}
	h.tape.mu.Lock()
	h.tape.casErr = nil
	h.tape.mu.Unlock()
	done := make(chan error, 1)
	go func() {
		done <- h.store.AppendEvent(context.Background(), h.event("after", "run-after", "owner-1", types.EventRunCompleted, telemetryBase))
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("a write after the panic: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("a later writer blocked: the leader token was never released")
	}
}

// A caller whose context is already cancelled must not win the leader token
// and commit its own write.
func TestCoalesceCancelledCallerNeverCommitsItsOwnWrite(t *testing.T) {
	h := newTapeHarness(t)
	h.warm()
	for i := 0; i < 40; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		id := fmt.Sprintf("cancelled-%d", i)
		err := h.store.projectionTape.appendOps(ctx, []computerevent.ProjectionOp{rmOp(id, "run-cancelled", int64(i+1))})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("write %d with a cancelled context returned %v, want context.Canceled", i, err)
		}
		if got := h.eventsHolding("run_memory:" + id); len(got) != 0 {
			t.Fatalf("write %d of a cancelled caller reached the tape", i)
		}
	}
}

// The split after a refusal gives every member its own deadline: members late
// in a big group must not inherit the group's elapsed time.
func TestCoalesceRefusalSplitGivesEachMemberItsOwnDeadline(t *testing.T) {
	h := newTapeHarness(t)
	h.warm()
	h.dryRunHook = func(batch *computerevent.ProjectionBatch) error {
		for _, op := range batch.Ops {
			if op.CanonicalID == "rm-poison" {
				return errors.New("poison refused by the dry run")
			}
		}
		return nil
	}
	gate := h.holdNextCAS()
	results := make(chan writeResult, 16)
	go func() {
		err := h.store.AppendEvent(context.Background(), h.event("lead", "run-lead", "owner-1", types.EventRunCompleted, telemetryBase))
		results <- writeResult{"lead", err}
	}()
	<-gate.entered
	h.store.tapeGroup.timeoutNanos.Store(int64(600 * time.Millisecond))
	const n = 8
	runs := distinctRunMemoryRuns(h.store, n)
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("rm-ok-%d", i)
		if i == 0 {
			id = "rm-poison"
		}
		id, run := id, runs[i]
		go func() {
			_, err := h.store.AppendRunMemoryEntry(context.Background(), types.RunMemoryEntry{EntryID: id, RunID: run, OwnerID: "owner-1", Kind: types.RunMemoryEntryMessage, Role: "user", Message: json.RawMessage(`{"role":"user","content":"x"}`)})
			results <- writeResult{id, err}
		}()
		waitFor(t, id+" pending", 5*time.Second, func() bool { return h.pendingMembers() >= i+1 })
	}
	h.tape.mu.Lock()
	h.tape.casErr = func(computerevent.CASRequest) error { time.Sleep(150 * time.Millisecond); return nil }
	h.tape.mu.Unlock()
	close(gate.release)
	for i := 0; i < n+1; i++ {
		r := <-results
		if r.id == "rm-poison" {
			if !errors.Is(r.err, computerevent.ErrBatchRefused) {
				t.Fatalf("poison: %v", r.err)
			}
			continue
		}
		if r.err != nil {
			t.Fatalf("%s: %v (the split shared the group's deadline)", r.id, r.err)
		}
	}
}

// telemetry_ridden counts records that rode an event that was committed.
func TestCoalesceTelemetryRiddenCountsOnlyOnSuccess(t *testing.T) {
	h := newTapeHarness(t)
	h.enableBatching(farFuture)
	h.warm()
	ctx := context.Background()
	if err := h.store.AppendEvent(ctx, h.event("t1", "run-r", "owner-1", types.EventRunProgress, telemetryBase)); err != nil {
		t.Fatal(err)
	}
	h.tape.mu.Lock()
	h.tape.casErr = func(computerevent.CASRequest) error { return errors.New("cas outcome unknown") }
	h.tape.mu.Unlock()
	if err := h.store.AppendEvent(ctx, h.event("a1", "run-r", "owner-1", types.EventRunCompleted, telemetryBase)); err == nil {
		t.Fatal("the failed group was acknowledged")
	}
	if got := h.store.TelemetryStats().GroupCommit.TelemetryRidden; got != 0 {
		t.Fatalf("telemetry_ridden = %d after a failed ride, want 0", got)
	}
}

// A member cancelled after it was taken returns its group's outcome, and its
// write is on the tape.
func TestCoalesceMemberCancelledAfterTakenReturnsTheGroupOutcome(t *testing.T) {
	h := newTapeHarness(t)
	h.warm()
	gate := h.holdNextCAS()
	results := make(chan writeResult, 8)
	go func() {
		err := h.store.AppendEvent(context.Background(), h.event("lead", "run-lead", "owner-1", types.EventRunCompleted, telemetryBase))
		results <- writeResult{"lead", err}
	}()
	<-gate.entered
	h.startAcked(results, "a", "run-a", 1)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		results <- writeResult{"b", h.store.AppendEvent(ctx, h.event("b", "run-b", "owner-1", types.EventRunCompleted, telemetryBase))}
	}()
	waitFor(t, "b pending", 5*time.Second, func() bool { return h.pendingMembers() >= 2 })
	entered2 := make(chan struct{})
	release2 := make(chan struct{})
	h.tape.mu.Lock()
	h.tape.casErr = func(computerevent.CASRequest) error {
		select {
		case <-entered2:
		default:
			close(entered2)
		}
		<-release2
		return nil
	}
	h.tape.mu.Unlock()
	close(gate.release)
	<-entered2 // the group holding a and b is in its CAS: both are taken
	cancel()
	time.Sleep(50 * time.Millisecond)
	close(release2)
	got := map[string]error{}
	for i := 0; i < 3; i++ {
		r := <-results
		got[r.id] = r.err
	}
	if got["b"] != nil || got["a"] != nil {
		t.Fatalf("results %v: a taken member must get the group's outcome", got)
	}
	if len(h.eventsHolding("loop.completed:b")) != 1 {
		t.Fatal("b's write is not on the tape")
	}
}

// An unclear CAS leaves a prepared row. A retried run-memory write must not
// number itself from the committed table before that row is settled: it would
// reuse the (run, seq) of the first event, commit a second event, and fail its
// finalize on the unique key (review: gemini should-fix 4).
func TestRunMemoryRetryAfterUnclearCASDoesNotDuplicateSeq(t *testing.T) {
	h := newTapeHarness(t)
	h.hostChain = &hostChain{}
	h.bind()
	h.warm()
	ctx := context.Background()
	msg := func(id string) types.RunMemoryEntry {
		return types.RunMemoryEntry{EntryID: id, RunID: "run-retry", OwnerID: "owner-1", Kind: types.RunMemoryEntryMessage, Role: "user", Message: json.RawMessage(`{"role":"user","content":"x"}`)}
	}
	if _, err := h.store.AppendRunMemoryEntry(ctx, msg("rm-first")); err != nil {
		t.Fatal(err)
	}
	// The platform commits, both replies are lost: the prepared row is kept.
	lost := 2
	h.tape.mu.Lock()
	h.tape.casAfter = func(request computerevent.CASRequest) error {
		h.tape.mu.Lock()
		lease := h.tape.leaseDigests[request.Event.PayloadCommitment]
		h.tape.mu.Unlock()
		if lease || lost == 0 {
			return nil
		}
		lost--
		return errors.New("reply lost")
	}
	h.tape.mu.Unlock()
	if _, err := h.store.AppendRunMemoryEntry(ctx, msg("rm-second")); err == nil {
		t.Fatal("fixture: the second append should have failed with an unknown outcome")
	}
	// The retry, with a new entry id, as a caller does.
	if _, err := h.store.AppendRunMemoryEntry(ctx, msg("rm-third")); err != nil {
		t.Fatalf("retry after an unclear CAS: %v", err)
	}
	entries, err := h.store.ListRunMemoryEntries(ctx, "owner-1", "run-retry")
	if err != nil {
		t.Fatal(err)
	}
	var seqs []string
	for i, e := range entries {
		seqs = append(seqs, fmt.Sprintf("%s=%d", e.EntryID, e.Seq))
		if e.Seq != int64(i+1) {
			t.Fatalf("seqs %s: not dense", strings.Join(seqs, ","))
		}
	}
	if st := h.appender.WedgeStatus(); st.Active {
		t.Fatalf("wedge tripped: %+v", st)
	}
}

// hostChain is the host's own head for the harness (see tapeHarness.hostChain).
type hostChain struct {
	mu   sync.Mutex
	head *computerevent.Head
}

type hostChainCAS struct {
	computerevent.HeadCAS
	chain *hostChain
	tape  *recordingTape
	local *Store
}

func (c hostChainCAS) Head(ctx context.Context, computerID string) (*computerevent.Head, error) {
	c.chain.mu.Lock()
	defer c.chain.mu.Unlock()
	if c.chain.head == nil {
		head, err := c.local.Head(ctx, computerID)
		if err != nil || head == nil {
			return head, err
		}
		c.chain.head = head
	}
	cp := *c.chain.head
	return &cp, nil
}

func (c hostChainCAS) CompareAndSwap(ctx context.Context, request computerevent.CASRequest) (computerevent.Receipt, error) {
	if _, err := c.Head(ctx, request.Event.ComputerID); err != nil {
		return computerevent.Receipt{}, err
	}
	c.chain.mu.Lock()
	known := false
	for _, r := range c.tape.snapshot() {
		known = known || r.IdempotencyKey == request.Event.IdempotencyKey
	}
	for _, r := range c.tape.leaseSnapshot() {
		known = known || r.IdempotencyKey == request.Event.IdempotencyKey
	}
	if !known && c.chain.head != nil && request.Event.PreviousHead != c.chain.head.CanonicalEventHead {
		c.chain.mu.Unlock()
		return computerevent.Receipt{}, &computerevent.CASError{Status: 409, Definite: true, Err: errors.New("previous head is not the host's")}
	}
	c.chain.mu.Unlock()
	receipt, err := c.HeadCAS.CompareAndSwap(ctx, request)
	committed := err == nil
	for _, r := range c.tape.snapshot() {
		committed = committed || r.IdempotencyKey == request.Event.IdempotencyKey
	}
	for _, r := range c.tape.leaseSnapshot() {
		committed = committed || r.IdempotencyKey == request.Event.IdempotencyKey
	}
	if committed && !known {
		next := request.Next
		c.chain.mu.Lock()
		c.chain.head = &next
		c.chain.mu.Unlock()
	}
	return receipt, err
}

// A cancellation that lands after a caller won the leader token must not let it
// commit its own write: the token goes back and the caller returns ctx.Err().
func TestCoalesceCancelAfterWinningTheTokenWritesNothing(t *testing.T) {
	h := newTapeHarness(t)
	h.warm()
	ctx, cancel := context.WithCancel(context.Background())
	h.store.tapeGroup.afterToken = func() { cancel() }
	defer func() { h.store.tapeGroup.afterToken = nil }()
	err := h.store.projectionTape.appendOps(ctx, []computerevent.ProjectionOp{rmOp("rm-late-cancel", "run-late-cancel", 1)})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("returned %v, want context.Canceled", err)
	}
	if got := h.eventsHolding("run_memory:rm-late-cancel"); len(got) != 0 {
		t.Fatal("a caller cancelled after winning the token committed its own write")
	}
	h.store.tapeGroup.afterToken = nil
	if err := h.store.AppendEvent(context.Background(), h.event("after", "run-after", "owner-1", types.EventRunCompleted, telemetryBase)); err != nil {
		t.Fatalf("the token was not handed back: %v", err)
	}
}
