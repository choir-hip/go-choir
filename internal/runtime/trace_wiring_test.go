package runtime

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/provider"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/trace"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// stubTraceStore is a minimal trace.Store implementation for wiring tests. It
// records appended events and can be configured to fail on Append to exercise
// graceful degradation.
type stubTraceStore struct {
	appended  []trace.Event
	appendErr error
	getErr    error
	closed    bool
}

func (s *stubTraceStore) Append(_ context.Context, e *trace.Event) error {
	if s.appendErr != nil {
		return s.appendErr
	}
	s.appended = append(s.appended, *e)
	return nil
}

func (s *stubTraceStore) Get(_ context.Context, id string) (*trace.Event, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	for i := range s.appended {
		if s.appended[i].ID == id {
			ev := s.appended[i]
			return &ev, nil
		}
	}
	return nil, trace.ErrNotFound
}

func (s *stubTraceStore) ListByRun(context.Context, string, int) ([]trace.Event, error) {
	return nil, nil
}

func (s *stubTraceStore) ListByOwner(context.Context, string, int) ([]trace.Event, error) {
	return nil, nil
}

func (s *stubTraceStore) ListByTrajectory(context.Context, string, string, int) ([]trace.Event, error) {
	return nil, nil
}

func (s *stubTraceStore) Close() error {
	s.closed = true
	return nil
}

// newTraceWiringRuntime builds a Runtime backed by a real Dolt store with the
// given trace store mounted via WithTraceStore. Returns the runtime, the store,
// and a cleanup func.
func newTraceWiringRuntime(t *testing.T, traceStore trace.Store) (*Runtime, func()) {
	t.Helper()
	dir := filepath.Join(os.TempDir(), "go-choir-m20-trace-wiring")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	dbPath := filepath.Join(dir, t.Name()+".db")
	promptRoot := filepath.Join(dir, t.Name()+"-prompts")
	_ = os.Remove(dbPath)
	_ = os.RemoveAll(promptRoot)

	s, err := openTestStore(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	opts := []RuntimeOption{WithTraceStore(traceStore)}
	rt := New(provideriface.Config{
		SandboxID:           "sandbox-trace-test",
		StorePath:           dbPath,
		PromptRoot:          promptRoot,
		ProviderTimeout:     time.Second,
		SupervisionInterval: time.Hour,
	}, s, events.NewEventBus(), provider.NewStubProvider(0), opts...)
	setTestDispatch(rt, s)

	cleanup := func() {
		rt.Stop()
		_ = s.Close()
		_ = os.Remove(dbPath)
		_ = os.RemoveAll(promptRoot)
	}
	return rt, cleanup
}

// TestEmitEventProjectsToTraceStore verifies that emitEvent persists the event
// to the existing store AND projects it into the mounted trace observability
// store. Existing behavior (store append + bus publish) is unchanged.
func TestEmitEventProjectsToTraceStore(t *testing.T) {
	ts := &stubTraceStore{}
	rt, cleanup := newTraceWiringRuntime(t, ts)
	defer cleanup()

	rec := &types.RunRecord{
		RunID:     "run-trace-1",
		AgentID:   "agent-trace",
		ChannelID: "chan-trace",
		OwnerID:   "user-alice",
		SandboxID: "sandbox-trace-test",
		State:     types.RunRunning,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := rt.store.CreateRun(context.Background(), *rec); err != nil {
		t.Fatalf("create run: %v", err)
	}

	rt.emitEvent(context.Background(), rec, types.EventRunStarted, events.CauseTaskLifecycle, nil)

	if len(ts.appended) != 1 {
		t.Fatalf("expected 1 trace event, got %d", len(ts.appended))
	}
	got := ts.appended[0]
	if got.RunID != "run-trace-1" || got.EventType != string(types.EventRunStarted) {
		t.Fatalf("trace event mismatch: %+v", got)
	}
	if got.Actor != "agent-trace" || got.OwnerID != "user-alice" {
		t.Fatalf("trace identity mismatch: %+v", got)
	}
	if got.ID == "" {
		t.Fatal("trace event id is empty")
	}
}

// TestPersistEventProjectsToTraceStore verifies the non-published persistEvent
// path also projects into the trace store.
func TestPersistEventProjectsToTraceStore(t *testing.T) {
	ts := &stubTraceStore{}
	rt, cleanup := newTraceWiringRuntime(t, ts)
	defer cleanup()

	rec := &types.RunRecord{
		RunID:     "run-trace-2",
		AgentID:   "agent-trace",
		ChannelID: "chan-trace",
		OwnerID:   "user-alice",
		SandboxID: "sandbox-trace-test",
		State:     types.RunRunning,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := rt.store.CreateRun(context.Background(), *rec); err != nil {
		t.Fatalf("create run: %v", err)
	}

	if err := rt.persistEvent(context.Background(), rec, types.EventRunPassivated, nil); err != nil {
		t.Fatalf("persistEvent: %v", err)
	}
	if len(ts.appended) != 1 {
		t.Fatalf("expected 1 trace event, got %d", len(ts.appended))
	}
	if ts.appended[0].EventType != string(types.EventRunPassivated) {
		t.Fatalf("trace event kind mismatch: %s", ts.appended[0].EventType)
	}
}

// TestEmitEventDegradesGracefullyOnTraceStoreFailure verifies that a trace
// store Append error does not change request handling: the existing store
// append still succeeds, the event is still published on the bus, and the
// error is swallowed (logged). This is the graceful-degradation invariant.
func TestEmitEventDegradesGracefullyOnTraceStoreFailure(t *testing.T) {
	ts := &stubTraceStore{appendErr: errors.New("dolt unavailable")}
	rt, cleanup := newTraceWiringRuntime(t, ts)
	defer cleanup()

	rec := &types.RunRecord{
		RunID:     "run-trace-3",
		AgentID:   "agent-trace",
		ChannelID: "chan-trace",
		OwnerID:   "user-alice",
		SandboxID: "sandbox-trace-test",
		State:     types.RunRunning,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := rt.store.CreateRun(context.Background(), *rec); err != nil {
		t.Fatalf("create run: %v", err)
	}

	busCh := rt.bus.Subscribe()
	defer rt.bus.Unsubscribe(busCh)

	// emitEvent must not return an error (it never has) and must still publish.
	rt.emitEvent(context.Background(), rec, types.EventRunCompleted, events.CauseTaskLifecycle, nil)

	// The existing store must still have the event.
	evts, err := rt.store.ListEvents(context.Background(), rec.RunID, 10)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	if len(evts) != 1 {
		t.Fatalf("expected 1 persisted event, got %d", len(evts))
	}
	// The bus must still receive the event.
	select {
	case ev := <-busCh:
		if ev.Record.RunID != rec.RunID || ev.Record.Kind != types.EventRunCompleted {
			t.Fatalf("bus event mismatch: %+v", ev)
		}
	case <-time.After(time.Second):
		t.Fatal("expected event on bus even when trace store fails")
	}
	// The trace store recorded nothing because every Append failed.
	if len(ts.appended) != 0 {
		t.Fatalf("expected no trace events on failure, got %d", len(ts.appended))
	}
}

// TestNilTraceStoreIsNoOp verifies that when no trace store is mounted, event
// emission is unchanged (no projection, no crash).
func TestNilTraceStoreIsNoOp(t *testing.T) {
	rt, cleanup := newTraceWiringRuntime(t, nil)
	defer cleanup()

	if rt.traceStore != nil {
		t.Fatal("expected nil trace store")
	}
	rec := &types.RunRecord{
		RunID:     "run-trace-4",
		AgentID:   "agent-trace",
		ChannelID: "chan-trace",
		OwnerID:   "user-alice",
		SandboxID: "sandbox-trace-test",
		State:     types.RunRunning,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := rt.store.CreateRun(context.Background(), *rec); err != nil {
		t.Fatalf("create run: %v", err)
	}
	// Must not panic.
	rt.emitEvent(context.Background(), rec, types.EventRunStarted, events.CauseTaskLifecycle, nil)
}

// TestPersistSubmittedRunProjectsToTraceStore verifies the submitted-event path
// also projects into the trace store when one is supplied.
func TestPersistSubmittedRunProjectsToTraceStore(t *testing.T) {
	ts := &stubTraceStore{}
	rt, cleanup := newTraceWiringRuntime(t, ts)
	defer cleanup()

	createdAt := time.Date(2026, 5, 24, 1, 2, 3, 0, time.UTC)
	agent := types.AgentRecord{
		AgentID:   "agent-sub",
		OwnerID:   "user-alice",
		SandboxID: "sandbox-trace-test",
		Profile:   agentprofile.Texture,
		Role:      agentprofile.Texture,
		ChannelID: "chan-sub",
		CreatedAt: createdAt,
		UpdatedAt: createdAt,
	}
	rec := &types.RunRecord{
		RunID:        "run-sub-1",
		AgentID:      agent.AgentID,
		ChannelID:    agent.ChannelID,
		AgentProfile: agent.Profile,
		AgentRole:    agent.Role,
		OwnerID:      agent.OwnerID,
		SandboxID:    agent.SandboxID,
		State:        types.RunPending,
		Prompt:       "revise",
		CreatedAt:    createdAt,
		UpdatedAt:    createdAt,
	}
	if err := persistSubmittedRun(context.Background(), rt.store, rt.bus, agent, rec, len(rec.Prompt), ts); err != nil {
		t.Fatalf("persistSubmittedRun: %v", err)
	}
	if len(ts.appended) != 1 {
		t.Fatalf("expected 1 trace event, got %d", len(ts.appended))
	}
	if ts.appended[0].EventType != string(types.EventRunSubmitted) {
		t.Fatalf("trace event kind mismatch: %s", ts.appended[0].EventType)
	}
	if ts.appended[0].RunID != "run-sub-1" {
		t.Fatalf("trace run id mismatch: %s", ts.appended[0].RunID)
	}
}

// TestStopClosesTraceStore verifies the runtime closes the trace store on
// shutdown (relevant for the SQLite-owned backend; Dolt-backed store Close is
// a no-op).
func TestStopClosesTraceStore(t *testing.T) {
	ts := &stubTraceStore{}
	_, cleanup := newTraceWiringRuntime(t, ts)
	// Run cleanup (which calls Stop) and verify closed flag.
	cleanup()
	if !ts.closed {
		t.Fatal("expected trace store to be closed on Stop")
	}
}

// TestEmitEventProjectsToSQLiteTraceStore exercises the full projection path
// against the real in-memory SQLite trace store backend to confirm the
// FromEventRecord + Append round-trip works end-to-end from the runtime.
func TestEmitEventProjectsToSQLiteTraceStore(t *testing.T) {
	ts, err := trace.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("open sqlite trace store: %v", err)
	}
	rt, cleanup := newTraceWiringRuntime(t, ts)
	defer cleanup()

	rec := &types.RunRecord{
		RunID:     "run-sqlite-1",
		AgentID:   "agent-sqlite",
		ChannelID: "chan-sqlite",
		OwnerID:   "user-alice",
		SandboxID: "sandbox-trace-test",
		State:     types.RunRunning,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := rt.store.CreateRun(context.Background(), *rec); err != nil {
		t.Fatalf("create run: %v", err)
	}
	rt.emitEvent(context.Background(), rec, types.EventToolInvoked, events.CauseToolExecution, nil)

	got, err := ts.ListByRun(context.Background(), "run-sqlite-1", 10)
	if err != nil {
		t.Fatalf("ListByRun: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 trace event in sqlite store, got %d", len(got))
	}
	if got[0].EventType != string(types.EventToolInvoked) {
		t.Fatalf("trace event kind mismatch: %s", got[0].EventType)
	}
}
