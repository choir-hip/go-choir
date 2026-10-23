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
	"strings"
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
