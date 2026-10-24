package store

// The event-table writer and the stream_seq lease
// (docs/problems/stream-seq-high-water-mark-design-2026-10-23.md, section 4
// "When" and the residual list): the deploy-2 writer refuses to stamp, and
// store.event_table refuses to turn on, until the lease state is ready with a
// lease on the tape. Ways the gate can fail, written before the code:
//
//   - the switch armed on an unseeded store reports itself as writing, or
//     stamps once anything allocates before the lease is on the tape;
//   - a seeding lease (the exact seed in flight, 163 s on the paper) admits
//     the writer;
//   - a lease seeded while no tape was bound (an og row with no tape event,
//     which replay cannot reproduce) admits the writer once a tape binds;
//   - the refusal is not an error the caller can recognise
//     (ErrEventTableLeaseNotReady) or does not name the lease state;
//   - once the lease is ready on the tape the writer still refuses.
//
// The gate reads the lease's own state (stream_seq_lease.go); it is not a
// second record of it.

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// stampsNow is stampsEventTable under eventMu, as AppendEvent calls it.
func stampsNow(s *Store, kind types.EventKind) bool {
	s.eventMu.Lock()
	defer s.eventMu.Unlock()
	return s.stampsEventTable(kind)
}

func wantLeaseRefusal(t *testing.T, s *Store, state string) {
	t.Helper()
	err := s.EventTableLeaseReady()
	if !errors.Is(err, ErrEventTableLeaseNotReady) {
		t.Fatalf("EventTableLeaseReady = %v, want ErrEventTableLeaseNotReady (lease %+v)", err, s.StreamSeqLeaseStats())
	}
	if !strings.Contains(err.Error(), state) {
		t.Fatalf("the refusal %q does not name the lease state %q", err, state)
	}
	if s.EventTableWritesOn() {
		t.Fatalf("the writer reports itself on with the lease %s", state)
	}
	if stampsNow(s, types.EventToolResult) {
		t.Fatalf("the writer would stamp with the lease %s", state)
	}
}

func TestEventTableWriterWaitsForTheLease(t *testing.T) {
	ctx := context.Background()
	h := newTapeHarness(t)
	s := h.store
	// The switch arms (the floor is written), but nothing stamps yet.
	if err := s.SetEventTableWrites(true); err != nil {
		t.Fatalf("arming the switch on a bound store: %v", err)
	}
	if got := s.StreamSeqLeaseStats().State; got != "unseeded" {
		t.Fatalf("fixture: lease state %q, want unseeded", got)
	}
	wantLeaseRefusal(t, s, "unseeded")

	// Seeding: the seed's lease write is held.
	reached := make(chan struct{})
	gate := make(chan struct{})
	released := false
	release := func() {
		if !released {
			released = true
			close(gate)
		}
	}
	t.Cleanup(release)
	s.leaseWriteHook = func(phase string, _ int64) error {
		if phase == "seed" {
			close(reached)
			<-gate
		}
		return nil
	}
	s.StartStreamSeqSeed()
	select {
	case <-reached:
	case <-time.After(30 * time.Second):
		t.Fatalf("the seeder never reached its lease write: %+v", s.StreamSeqLeaseStats())
	}
	if got := s.StreamSeqLeaseStats().State; got != "seeding" {
		t.Fatalf("lease state %q while the seed's write is held, want seeding", got)
	}
	wantLeaseRefusal(t, s, "seeding")

	// Ready, with the lease on the tape: the writer stamps.
	release()
	waitLeaseReady(t, s)
	if err := s.EventTableLeaseReady(); err != nil {
		t.Fatalf("EventTableLeaseReady with the lease ready on the tape = %v", err)
	}
	if !s.EventTableWritesOn() || !stampsNow(s, types.EventToolResult) {
		t.Fatalf("the writer refuses with the lease ready on the tape: on=%v %+v", s.EventTableWritesOn(), s.StreamSeqLeaseStats())
	}
	if err := s.AppendEvent(ctx, h.event("gate-1", "run-gate", "owner-1", types.EventToolResult, telemetryBase)); err != nil {
		t.Fatal(err)
	}
	if n := tableCount(t, s, "choir_events", ""); n != 1 {
		t.Fatalf("choir_events rows = %d after one stamped append, want 1", n)
	}
}

func TestEventTableWriterRefusesALeaseOffTheTape(t *testing.T) {
	ctx := context.Background()
	path := testStorePath(t)
	cleanupTestStorePath(path)
	t.Cleanup(func() { cleanupTestStorePath(path) })
	h := &tapeHarness{t: t, path: path, computerID: "computer-telemetry", tape: newRecordingTape(t)}
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	h.store = s
	t.Cleanup(func() { _ = h.store.Close() })
	prepareGenesis(t, s, h.computerID, "genesis-telemetry")
	// An append before any tape is bound seeds the lease with an og row and
	// no tape event.
	if err := s.AppendEvent(ctx, h.event("pre-bind-1", "run-pre", "owner-1", types.EventToolResult, telemetryBase)); err != nil {
		t.Fatal(err)
	}
	if got := s.StreamSeqLeaseStats().State; got != "ready" {
		t.Fatalf("fixture: lease state %q after the unbound append, want ready", got)
	}
	h.bind()
	if err := s.SetEventTableWrites(true); err != nil {
		t.Fatalf("arming the switch on a bound store: %v", err)
	}
	wantLeaseRefusal(t, s, "tape")
	if err := s.AppendEvent(ctx, h.event("post-bind-1", "run-pre", "owner-1", types.EventToolResult, telemetryBase.Add(time.Second))); err != nil {
		t.Fatal(err)
	}
	if n := tableCount(t, s, "choir_events", ""); n != 0 {
		t.Fatalf("choir_events rows = %d with no lease on the tape, want 0", n)
	}
}

// Round 2 of the two-seat review (bunch 178, Devin 1, DeepSeek 1 and 4).
// Ways the lease and the gate can fail around Reopen and a refill, written
// before the fix:
//
//   - Reopen lifts the lease's closing flag before it re-reads the adopted
//     workspace, so a seeder started in between reads the untrust marker of
//     the workspace Reopen just left;
//   - onTape survives Reopen: the adopted workspace may hold an older lease
//     row, or none, and the gate still says the latest lease row is on the
//     tape;
//   - the refill that puts a lease row on the tape (the error message's
//     promise for a lease seeded before the tape was bound) never opens the
//     gate.

func TestReopenReadsTheAdoptedWorkspaceBeforeTheLeaseMayStartWork(t *testing.T) {
	s, _ := openSeedStore(t)
	oldWorkspace := s.texturePath
	pathB := strings.TrimSuffix(testStorePath(t), ".db") + "-adopted.db"
	cleanupTestStorePath(pathB)
	t.Cleanup(func() { cleanupTestStorePath(pathB) })

	var atResume string
	var resumed bool
	leaseResumeHook = func(st *Store) {
		resumed = true
		st.eventMu.Lock()
		atResume = st.lease.workspace
		st.eventMu.Unlock()
	}
	t.Cleanup(func() { leaseResumeHook = nil })

	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.Reopen(pathB); err != nil {
		t.Fatal(err)
	}
	if !resumed {
		t.Fatal("Reopen never resumed the lease work")
	}
	if s.texturePath == oldWorkspace {
		t.Fatalf("fixture: Reopen kept the workspace %q", oldWorkspace)
	}
	if atResume != s.texturePath {
		t.Fatalf("the lease could start work while it still named the workspace %q, not the adopted %q: a seeder would read the pre-swap marker", atResume, s.texturePath)
	}
}

func TestReopenClosesTheGateUntilAFreshLeaseRowLandsOnTheTape(t *testing.T) {
	shrinkLease(t, 64)
	h := newTapeHarness(t)
	s := h.store
	if err := s.SetEventTableWrites(true); err != nil {
		t.Fatal(err)
	}
	s.StartStreamSeqSeed()
	waitLeaseReady(t, s)
	if err := s.EventTableLeaseReady(); err != nil {
		t.Fatalf("fixture: the gate is closed with the lease ready on the tape: %v", err)
	}

	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	// Hold the lease row Reopen's refill writes, so the window is observable.
	reached := make(chan struct{})
	gate := make(chan struct{})
	var once sync.Once
	release := func() { once.Do(func() { close(gate) }) }
	t.Cleanup(release)
	s.leaseWriteHook = func(phase string, _ int64) error {
		if phase == "refill" {
			select {
			case <-reached:
			default:
				close(reached)
			}
			<-gate
		}
		return nil
	}
	t.Cleanup(func() { s.leaseWriteHook = nil })
	if err := s.Reopen(h.path); err != nil {
		t.Fatal(err)
	}
	// The adopted workspace may hold an older lease row or none: nothing
	// shows this process's latest lease row on the tape yet.
	wantLeaseRefusal(t, s, "tape")
	if s.StreamSeqLeaseStats().OnTape {
		t.Fatalf("the lease stats say on_tape after Reopen: %+v", s.StreamSeqLeaseStats())
	}

	select {
	case <-reached:
	case <-time.After(30 * time.Second):
		t.Fatalf("Reopen never wrote a fresh lease row: %+v", s.StreamSeqLeaseStats())
	}
	release()
	deadline := time.Now().Add(30 * time.Second)
	for s.EventTableLeaseReady() != nil {
		if time.Now().After(deadline) {
			t.Fatalf("the gate never reopened after the fresh lease row: %+v", s.StreamSeqLeaseStats())
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !s.StreamSeqLeaseStats().OnTape {
		t.Fatalf("the gate is open but on_tape is false: %+v", s.StreamSeqLeaseStats())
	}
}

func TestARefillOntoTheTapeOpensTheEventTableGate(t *testing.T) {
	ctx := context.Background()
	shrinkLease(t, 8)
	path := testStorePath(t)
	cleanupTestStorePath(path)
	t.Cleanup(func() { cleanupTestStorePath(path) })
	h := &tapeHarness{t: t, path: path, computerID: "computer-telemetry", tape: newRecordingTape(t)}
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	h.store = s
	t.Cleanup(func() { _ = h.store.Close() })
	prepareGenesis(t, s, h.computerID, "genesis-telemetry")
	// Seeded before any tape is bound: an og row only.
	if err := s.AppendEvent(ctx, h.event("pre-bind-1", "run-pre", "owner-1", types.EventToolResult, telemetryBase)); err != nil {
		t.Fatal(err)
	}
	h.bind()
	if err := s.SetEventTableWrites(true); err != nil {
		t.Fatal(err)
	}
	wantLeaseRefusal(t, s, "tape")

	// Appends consume the small block until a refill is written through the
	// bound tape; the gate opens with it.
	opened := false
	for i := 0; i < 60 && !opened; i++ {
		id := fmt.Sprintf("refill-%d", i)
		if err := s.AppendEvent(ctx, h.event(id, "run-pre", "owner-1", types.EventToolResult, telemetryBase.Add(time.Duration(i+1)*time.Second))); err != nil {
			t.Fatal(err)
		}
		opened = s.EventTableLeaseReady() == nil
		if !opened {
			time.Sleep(20 * time.Millisecond)
		}
	}
	if !opened {
		t.Fatalf("the gate never opened after appends past a small block: %+v", s.StreamSeqLeaseStats())
	}
	if err := s.AppendEvent(ctx, h.event("after-refill", "run-pre", "owner-1", types.EventToolResult, telemetryBase.Add(time.Hour))); err != nil {
		t.Fatal(err)
	}
	if n := tableCount(t, s, "choir_events", ""); n < 1 {
		t.Fatalf("choir_events rows = %d after the gate opened, want at least 1", n)
	}
}
