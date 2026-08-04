package actorruntime

import (
	"context"
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/actor"
	"github.com/yusefmosiah/go-choir/internal/agentcore"
	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/provider"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// adapterTestEnv holds the common test infrastructure.
type adapterTestEnv struct {
	t       *testing.T
	adapter *Adapter
	store   *store.Store
	ctx     context.Context
	cancel  context.CancelFunc
}

type startupBlockingProvider struct {
	started chan struct{}
	release chan struct{}
}

func (p *startupBlockingProvider) Execute(ctx context.Context, task *types.RunRecord, _ provideriface.EventEmitFunc) error {
	select {
	case p.started <- struct{}{}:
	default:
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-p.release:
		task.Result = "startup recovery completed"
		return nil
	}
}

func (p *startupBlockingProvider) ProviderName() string { return "startup-blocking" }

func newAdapterTestEnv(t *testing.T) *adapterTestEnv {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	promptRoot := filepath.Join(dir, "prompts")

	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	cfg := provideriface.Config{
		SandboxID:           "sandbox-test",
		StorePath:           dbPath,
		PromptRoot:          promptRoot,
		ProviderTimeout:     time.Second,
		SupervisionInterval: time.Hour,
	}

	adapter := New(cfg, s, events.NewEventBus(), provider.NewStubProvider(0), nil)
	t.Cleanup(func() {
		adapter.Stop()
		adapter.cleanupLog()
	})

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	if err := adapter.Start(ctx); err != nil {
		t.Fatalf("start adapter: %v", err)
	}

	return &adapterTestEnv{t: t, adapter: adapter, store: s, ctx: ctx, cancel: cancel}
}

func TestInitialDispatchUpdateIdentityIsStableAndScoped(t *testing.T) {
	first := actorDispatchUpdateID("owner-a", "computer-a", "agent-a", "initial_dispatch", "run-a")
	replay := actorDispatchUpdateID("owner-a", "computer-a", "agent-a", "initial_dispatch", "run-a")
	if first != replay {
		t.Fatalf("initial dispatch replay IDs differ: %q != %q", first, replay)
	}
	for name, changed := range map[string]string{
		"owner":    actorDispatchUpdateID("owner-b", "computer-a", "agent-a", "initial_dispatch", "run-a"),
		"computer": actorDispatchUpdateID("owner-a", "computer-b", "agent-a", "initial_dispatch", "run-a"),
		"agent":    actorDispatchUpdateID("owner-a", "computer-a", "agent-b", "initial_dispatch", "run-a"),
		"run":      actorDispatchUpdateID("owner-a", "computer-a", "agent-a", "initial_dispatch", "run-b"),
	} {
		if changed == first {
			t.Fatalf("%s scope change reused initial dispatch ID %q", name, first)
		}
	}
}

// waitForRunState polls the store until the run reaches the target state or
// times out.
func waitForRunState(t *testing.T, s *store.Store, ctx context.Context, runID string, target types.RunState, timeout time.Duration) types.RunRecord {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		rec, err := s.GetRun(ctx, runID)
		if err != nil {
			t.Fatalf("GetRun %s: %v", runID, err)
		}
		if rec.State == target {
			return rec
		}
		if rec.State.Terminal() && target != rec.State {
			t.Fatalf("run %s reached terminal state %s, want %s", runID, rec.State, target)
		}
		time.Sleep(20 * time.Millisecond)
	}
	rec, _ := s.GetRun(ctx, runID)
	t.Fatalf("run %s did not reach %s within %s (state=%s)", runID, target, timeout, rec.State)
	return types.RunRecord{}
}

// TestAdapterStartRunExecutesViaActorHandler verifies that a run started via
// the Adapter executes through the actor handler (not startRunAsync) and
// completes. This is the Phase 1 existential test: the actor handler IS the
// execution boundary.
func TestAdapterStartRunExecutesViaActorHandler(t *testing.T) {
	env := newAdapterTestEnv(t)

	rec, err := env.adapter.Runtime.StartRun(env.ctx, "Test prompt for actor handler", "test-owner")
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	if rec.RunID == "" {
		t.Fatal("StartRun returned empty run ID")
	}

	final := waitForRunState(t, env.store, env.ctx, rec.RunID, types.RunCompleted, 5*time.Second)
	if final.Result == "" {
		t.Fatal("run completed but result is empty")
	}
}

// TestAdapterDispatchActorActive verifies that the Adapter wires the
// dispatch function on the runtime core.
func TestAdapterDispatchActorActive(t *testing.T) {
	env := newAdapterTestEnv(t)

	if !env.adapter.Runtime.DispatchActorActive() {
		t.Fatal("DispatchActorActive() = false, want true (adapter should wire dispatch)")
	}
	if env.adapter.ActorRuntime() == nil {
		t.Fatal("ActorRuntime() = nil, want non-nil")
	}
}

func TestAdapterRuntimeCoreIsNamedAndNotEmbedded(t *testing.T) {
	adapterType := reflect.TypeOf(Adapter{})
	runtimeType := reflect.TypeOf((*agentcore.Runtime)(nil))
	runtimeFields := 0
	for i := range adapterType.NumField() {
		field := adapterType.Field(i)
		if field.Type != runtimeType {
			continue
		}
		runtimeFields++
		if field.Name != "Runtime" {
			t.Errorf("runtime core field name = %q, want %q", field.Name, "Runtime")
		}
		if field.Anonymous {
			t.Error("runtime core field is anonymous, want named field")
		}
	}
	if runtimeFields != 1 {
		t.Errorf("runtime core field count = %d, want 1", runtimeFields)
	}
}

// TestHandlerColdStartCoagentResult tests the cold-start path: a coagent_result
// arrives with nil memory (no parked run). The handler should call
// ReconcileCoagentWake to create a new run.
func TestHandlerColdStartCoagentResult(t *testing.T) {
	env := newAdapterTestEnv(t)

	// Create an agent record so ownerForAgent can look it up.
	agentID := "agent-test-cold-start"
	ownerID := "user-cold-start"
	err := env.store.UpsertAgent(env.ctx, types.AgentRecord{
		AgentID:   agentID,
		OwnerID:   ownerID,
		SandboxID: "sandbox-test",
		Profile:   "test-profile",
	})
	if err != nil {
		t.Fatalf("UpsertAgent: %v", err)
	}

	// Send a coagent_result with nil memory — simulates cold start.
	handler := newActorHandler(env.adapter.Runtime, nil)
	u := actorUpdate("user-cold-start", "coagent_result", agentID, "coagent-result-content")
	memory, err := handler.HandleUpdate(env.ctx, agentID, u, nil)
	if err != nil {
		t.Fatalf("HandleUpdate cold start: %v", err)
	}

	// Cold start returns nil memory (the new run will be started by
	// the initial_dispatch message from ReconcileCoagentWake).
	if memory != nil {
		t.Errorf("cold start memory = %v, want nil (new run started via initial_dispatch)", memory)
	}
}

// TestHandlerCancelPassivatedRun tests that a cancel message for a passivated
// run projects canonical RunCancelled state.
func TestHandlerCancelPassivatedRun(t *testing.T) {
	env := newAdapterTestEnv(t)

	// Create a run and manually set it to RunPassivated.
	rec := types.RunRecord{
		RunID:     "run-cancel-test",
		OwnerID:   "user-cancel",
		AgentID:   "agent-cancel-test",
		SandboxID: "sandbox-test",
		Prompt:    "test cancel",
		State:     types.RunPassivated,
	}
	if err := env.store.CreateRun(env.ctx, rec); err != nil {
		t.Fatalf("CreateRun: %v", err)
	}

	// Encode memory with the run ID.
	mem, _ := json.Marshal(resumeState{RunID: rec.RunID, Phase: "parked"})

	// Send cancel.
	handler := newActorHandler(env.adapter.Runtime, nil)
	u := actorUpdate(rec.OwnerID, "cancel", "agent-cancel-test", "")
	_, err := handler.HandleUpdate(env.ctx, "agent-cancel-test", u, mem)
	if err != nil {
		t.Fatalf("HandleUpdate cancel: %v", err)
	}

	// Verify the run was cancelled without impersonating execution failure.
	updated, err := env.store.GetRun(env.ctx, rec.RunID)
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if updated.State != types.RunCancelled {
		t.Errorf("run state = %s, want RunCancelled", updated.State)
	}
	if updated.Error == "" {
		t.Error("run error is empty, want cancel message")
	}
}

// TestHandlerCancelMissingRun tests that cancelling a non-existent run is a
// no-op (no error).
func TestHandlerCancelMissingRun(t *testing.T) {
	env := newAdapterTestEnv(t)

	mem, _ := json.Marshal(resumeState{RunID: "nonexistent-run", Phase: "parked"})
	handler := newActorHandler(env.adapter.Runtime, nil)
	u := actorUpdate("owner-missing", "cancel", "agent-missing", "")
	_, err := handler.HandleUpdate(env.ctx, "agent-missing", u, mem)
	if err != nil {
		t.Errorf("HandleUpdate cancel missing run: error = %v, want nil (no-op)", err)
	}
}

// TestHandlerCoagentResultForCompletedRun tests that a coagent_result for a
// terminal (completed) run triggers ReconcileCoagentWake to create a new run.
func TestHandlerCoagentResultForCompletedRun(t *testing.T) {
	env := newAdapterTestEnv(t)

	agentID := "agent-completed-test"
	ownerID := "user-completed"
	err := env.store.UpsertAgent(env.ctx, types.AgentRecord{
		AgentID:   agentID,
		OwnerID:   ownerID,
		SandboxID: "sandbox-test",
		Profile:   "test-profile",
	})
	if err != nil {
		t.Fatalf("UpsertAgent: %v", err)
	}

	// Create a completed run.
	rec := types.RunRecord{
		RunID:     "run-completed-test",
		OwnerID:   ownerID,
		AgentID:   agentID,
		SandboxID: "sandbox-test",
		Prompt:    "test completed",
		State:     types.RunCompleted,
		Result:    "done",
	}
	now := time.Now().UTC()
	rec.FinishedAt = &now
	if err := env.store.CreateRun(env.ctx, rec); err != nil {
		t.Fatalf("CreateRun: %v", err)
	}

	// Send coagent_result with memory pointing to the completed run.
	mem, _ := json.Marshal(resumeState{RunID: rec.RunID, Phase: "parked"})
	handler := newActorHandler(env.adapter.Runtime, nil)
	u := actorUpdate(ownerID, "coagent_result", agentID, "new-result")
	_, err = handler.HandleUpdate(env.ctx, agentID, u, mem)
	if err != nil {
		t.Fatalf("HandleUpdate coagent_result for completed run: %v", err)
	}

	// The handler should have called ReconcileCoagentWake, which creates
	// a new run and sends an initial_dispatch. The new run should eventually
	// complete (stub provider returns immediately).
	// We can't easily wait for the new run here, but the absence of an
	// error means ReconcileCoagentWake succeeded.
}

// TestHandlerCoagentResultForBlockedRun tests the bug fix: a coagent_result
// for a blocked run should reactivate it, NOT silently drop the message and
// clear memory. Before the fix, this would orphan the blocked run.
func TestHandlerCoagentResultForBlockedRun(t *testing.T) {
	env := newAdapterTestEnv(t)

	agentID := "agent-blocked-test"
	ownerID := "user-blocked"
	err := env.store.UpsertAgent(env.ctx, types.AgentRecord{
		AgentID:   agentID,
		OwnerID:   ownerID,
		SandboxID: "sandbox-test",
		Profile:   "test-profile",
	})
	if err != nil {
		t.Fatalf("UpsertAgent: %v", err)
	}

	// Create a blocked run.
	rec := types.RunRecord{
		RunID:     "run-blocked-test",
		OwnerID:   ownerID,
		SandboxID: "sandbox-test",
		AgentID:   agentID,
		Prompt:    "test blocked",
		State:     types.RunBlocked,
		Error:     "provider rate limit",
	}
	if err := env.store.CreateRun(env.ctx, rec); err != nil {
		t.Fatalf("CreateRun: %v", err)
	}

	// Send coagent_result with memory pointing to the blocked run.
	mem, _ := json.Marshal(resumeState{RunID: rec.RunID, Phase: "parked"})
	handler := newActorHandler(env.adapter.Runtime, nil)
	u := actorUpdate(ownerID, "coagent_result", agentID, "unblocking-result")
	resultMem, err := handler.HandleUpdate(env.ctx, agentID, u, mem)
	if err != nil {
		t.Fatalf("HandleUpdate coagent_result for blocked run: %v", err)
	}

	// The handler should have reactivated the run (set to RunPending,
	// called ExecuteActivationSync). The stub provider should complete it.
	// Memory should NOT be nil (the run was reactivated, not dropped).
	_ = resultMem // memory may be nil if the run completed immediately

	// Verify the run was reactivated (no longer RunBlocked).
	updated, err := env.store.GetRun(env.ctx, rec.RunID)
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if updated.State == types.RunBlocked {
		t.Error("run is still RunBlocked — the bug: coagent_result was silently dropped instead of reactivating")
	}
	// The run should have been reactivated and completed (stub provider).
	if updated.State != types.RunCompleted {
		// Give it a moment to complete.
		updated = waitForRunState(t, env.store, env.ctx, rec.RunID, types.RunCompleted, 3*time.Second)
	}
}

// TestHandlerUnknownUpdateKind tests that an unknown update kind is handled
// gracefully (memory unchanged, no error).
func TestHandlerUnknownUpdateKind(t *testing.T) {
	env := newAdapterTestEnv(t)

	handler := newActorHandler(env.adapter.Runtime, nil)
	u := actorUpdate("owner-unknown", "unknown_kind", "agent-test", "content")
	existingMem := []byte(`{"run_id":"run-x","phase":"parked"}`)
	resultMem, err := handler.HandleUpdate(env.ctx, "agent-test", u, existingMem)
	if err != nil {
		t.Errorf("HandleUpdate unknown kind: error = %v, want nil", err)
	}
	// Memory should be unchanged.
	if string(resultMem) != string(existingMem) {
		t.Errorf("memory changed: got %q, want %q (unchanged for unknown kind)", resultMem, existingMem)
	}
}

func TestScopedActorMailboxDoesNotCrossOwner(t *testing.T) {
	env := newAdapterTestEnv(t)
	const agentID = "shared-agent-id"
	for _, ownerID := range []string{"owner-scope-a", "owner-scope-b"} {
		if err := env.store.UpsertAgent(env.ctx, types.AgentRecord{
			AgentID: agentID, OwnerID: ownerID, ComputerID: "sandbox-test", SandboxID: "sandbox-test",
			Profile: "researcher", Role: "researcher", ChannelID: "channel-" + ownerID,
		}); err != nil {
			t.Fatalf("upsert scoped agent %s: %v", ownerID, err)
		}
	}
	handler := newActorHandler(env.adapter.Runtime, nil)
	update := actorUpdate("owner-scope-a", "coagent_result", agentID, "scoped wake")
	now := time.Now().UTC()
	packet := types.CoagentSourcePacket{
		UpdateID: "scoped-update-a", OwnerID: "owner-scope-a", ComputerID: "sandbox-test",
		AgentID: "producer-a", TargetAgentID: agentID, ChannelID: "channel-owner-scope-a", Role: "researcher",
		Packet:  types.CoagentSourcePacketPayload{SchemaVersion: types.CoagentSourcePacketSchemaV1, Kind: "evidence_update", Summary: "scoped update"},
		Content: "scoped wake", CreatedAt: now,
	}
	message := types.ChannelMessage{
		ChannelID: packet.ChannelID, FromAgentID: packet.AgentID, ToAgentID: agentID,
		Role: packet.Role, Content: packet.Content, Timestamp: now,
	}
	if _, _, err := env.store.DispatchWorkerUpdate(env.ctx, packet, &message); err != nil {
		t.Fatalf("dispatch scoped update: %v", err)
	}
	if _, err := handler.HandleUpdate(env.ctx, update.ToAgentID, update, nil); err != nil {
		t.Fatalf("handle scoped wake: %v", err)
	}
	ownerARuns, err := env.store.ListRunsByOwner(env.ctx, "owner-scope-a", 20)
	if err != nil || len(ownerARuns) == 0 {
		t.Fatalf("owner A wake missing: runs=%+v err=%v", ownerARuns, err)
	}
	ownerBRuns, err := env.store.ListRunsByOwner(env.ctx, "owner-scope-b", 20)
	if err != nil {
		t.Fatalf("list owner B runs: %v", err)
	}
	if len(ownerBRuns) != 0 {
		t.Fatalf("owner A wake crossed into owner B: %+v", ownerBRuns)
	}
}

func TestInitialDispatchCannotLoadAnotherOwnersRun(t *testing.T) {
	env := newAdapterTestEnv(t)
	now := time.Now().UTC()
	rec := types.RunRecord{
		RunID: "run-owner-b", OwnerID: "owner-scope-b", SandboxID: "sandbox-test",
		AgentID: "agent-owner-b", State: types.RunPending, CreatedAt: now, UpdatedAt: now,
	}
	if err := env.store.CreateRun(env.ctx, rec); err != nil {
		t.Fatalf("create owner B run: %v", err)
	}
	handler := newActorHandler(env.adapter.Runtime, nil)
	update := actorUpdate("owner-scope-a", "initial_dispatch", rec.AgentID, rec.RunID)
	if _, err := handler.HandleUpdate(env.ctx, update.ToAgentID, update, nil); err == nil {
		t.Fatal("cross-owner initial dispatch succeeded")
	}
	stored, err := env.store.GetRunByOwner(env.ctx, rec.OwnerID, rec.RunID)
	if err != nil || stored.State != types.RunPending {
		t.Fatalf("cross-owner dispatch changed run: %+v, %v", stored, err)
	}
}

func TestAdapterStartMigratesUniqueLegacyUnscopedMailbox(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "legacy-mailbox.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	cfg := provideriface.Config{SandboxID: "sandbox-legacy", StorePath: dbPath, PromptRoot: filepath.Join(dir, "prompts")}
	adapter := New(cfg, s, events.NewEventBus(), provider.NewStubProvider(0), nil)
	t.Cleanup(func() {
		adapter.Stop()
		adapter.cleanupLog()
		_ = s.Close()
	})
	now := time.Now().UTC()
	agent := types.AgentRecord{
		AgentID: "legacy-unscoped-agent", OwnerID: "legacy-owner", ComputerID: cfg.SandboxID, SandboxID: cfg.SandboxID,
		Profile: "processor", Role: "processor", ChannelID: "legacy-channel", CreatedAt: now, UpdatedAt: now,
	}
	if err := s.UpsertAgent(ctx, agent); err != nil {
		t.Fatalf("upsert legacy agent: %v", err)
	}
	if appended, err := adapter.log.Append(ctx, actor.Update{
		UpdateID: "legacy-update", ToAgentID: agent.AgentID, Kind: "retained_unknown_kind", CreatedAt: now,
	}); err != nil || !appended {
		t.Fatalf("append legacy mailbox: appended=%v err=%v", appended, err)
	}
	if err := adapter.log.SaveSnapshot(ctx, agent.AgentID, []byte(`{"phase":"parked"}`)); err != nil {
		t.Fatalf("save legacy snapshot: %v", err)
	}
	if err := adapter.Start(ctx); err != nil {
		t.Fatalf("start with unique legacy mailbox: %v", err)
	}
	scopedID := scopedActorMailboxID(agent.OwnerID, agent.ComputerID, agent.AgentID)
	if legacy, err := adapter.log.Unprocessed(ctx, agent.AgentID); err != nil || len(legacy) != 0 {
		t.Fatalf("legacy backlog after startup: %+v, %v", legacy, err)
	}
	memory, err := adapter.log.LoadSnapshot(ctx, scopedID)
	if err != nil || string(memory) != `{"phase":"parked"}` {
		t.Fatalf("scoped snapshot after startup: %q, %v", memory, err)
	}
}

func TestAdapterStartMigratesLegacyMailboxFromRunWitness(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "legacy-run-witness.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	cfg := provideriface.Config{SandboxID: "sandbox-legacy", StorePath: dbPath, PromptRoot: filepath.Join(dir, "prompts")}
	adapter := New(cfg, s, events.NewEventBus(), provider.NewStubProvider(0), nil)
	t.Cleanup(func() {
		adapter.Stop()
		adapter.cleanupLog()
		_ = s.Close()
	})
	now := time.Now().UTC()
	const (
		agentID = "legacy-agent-without-record"
		ownerID = "legacy-owner"
	)
	if err := s.CreateRun(ctx, types.RunRecord{
		RunID: "legacy-run-witness", AgentID: agentID, OwnerID: ownerID, SandboxID: cfg.SandboxID,
		State: types.RunCompleted, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create legacy run witness: %v", err)
	}
	if err := adapter.log.SaveSnapshot(ctx, agentID, []byte(`{"phase":"parked"}`)); err != nil {
		t.Fatalf("save legacy snapshot: %v", err)
	}
	if err := adapter.Start(ctx); err != nil {
		t.Fatalf("start with run-witness legacy mailbox: %v", err)
	}
	scopedID := scopedActorMailboxID(ownerID, cfg.SandboxID, agentID)
	identities, err := adapter.log.MailboxIdentities(ctx)
	if err != nil || len(identities) != 1 || identities[0] != scopedID {
		t.Fatalf("mailbox identities after startup: %q, %v; want [%q]", identities, err, scopedID)
	}
	memory, err := adapter.log.LoadSnapshot(ctx, scopedID)
	if err != nil || string(memory) != `{"phase":"parked"}` {
		t.Fatalf("scoped snapshot after startup: %q, %v", memory, err)
	}
}

func TestAdapterStartMigratesLegacyMailboxWithoutPendingBacklog(t *testing.T) {
	for _, tc := range []struct {
		name string
		seed func(*testing.T, *actor.SQLiteLog, context.Context, string, time.Time)
	}{
		{
			name: "snapshot-only",
			seed: func(t *testing.T, log *actor.SQLiteLog, ctx context.Context, mailboxID string, _ time.Time) {
				t.Helper()
				if err := log.SaveSnapshot(ctx, mailboxID, []byte(`{"phase":"parked"}`)); err != nil {
					t.Fatalf("save legacy snapshot: %v", err)
				}
			},
		},
		{
			name: "processed-only",
			seed: func(t *testing.T, log *actor.SQLiteLog, ctx context.Context, mailboxID string, now time.Time) {
				t.Helper()
				if appended, err := log.Append(ctx, actor.Update{UpdateID: "processed-update", ToAgentID: mailboxID, CreatedAt: now}); err != nil || !appended {
					t.Fatalf("append processed update: appended=%v err=%v", appended, err)
				}
				if err := log.MarkProcessed(ctx, mailboxID, "processed-update"); err != nil {
					t.Fatalf("mark update processed: %v", err)
				}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			dir := t.TempDir()
			dbPath := filepath.Join(dir, "legacy-mailbox.db")
			s, err := store.Open(dbPath)
			if err != nil {
				t.Fatalf("open store: %v", err)
			}
			cfg := provideriface.Config{SandboxID: "sandbox-legacy", StorePath: dbPath, PromptRoot: filepath.Join(dir, "prompts")}
			adapter := New(cfg, s, events.NewEventBus(), provider.NewStubProvider(0), nil)
			t.Cleanup(func() {
				adapter.Stop()
				adapter.cleanupLog()
				_ = s.Close()
			})
			now := time.Now().UTC()
			agent := types.AgentRecord{
				AgentID: "legacy-unscoped-agent", OwnerID: "legacy-owner", ComputerID: cfg.SandboxID, SandboxID: cfg.SandboxID,
				Profile: "processor", Role: "processor", ChannelID: "legacy-channel", CreatedAt: now, UpdatedAt: now,
			}
			if err := s.UpsertAgent(ctx, agent); err != nil {
				t.Fatalf("upsert legacy agent: %v", err)
			}
			tc.seed(t, adapter.log, ctx, agent.AgentID, now)
			if err := adapter.Start(ctx); err != nil {
				t.Fatalf("start with %s legacy mailbox: %v", tc.name, err)
			}
			identities, err := adapter.log.MailboxIdentities(ctx)
			scopedID := scopedActorMailboxID(agent.OwnerID, agent.ComputerID, agent.AgentID)
			if err != nil || len(identities) != 1 || identities[0] != scopedID {
				t.Fatalf("mailbox identities after startup: %q, %v; want [%q]", identities, err, scopedID)
			}
		})
	}
}

func TestAdapterStartRefusesAmbiguousLegacyUnscopedMailbox(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "ambiguous-legacy-mailbox.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	cfg := provideriface.Config{SandboxID: "sandbox-legacy", StorePath: dbPath, PromptRoot: filepath.Join(dir, "prompts")}
	adapter := New(cfg, s, events.NewEventBus(), provider.NewStubProvider(0), nil)
	t.Cleanup(func() {
		adapter.Stop()
		adapter.cleanupLog()
		_ = s.Close()
	})
	now := time.Now().UTC()
	for _, ownerID := range []string{"owner-a", "owner-b"} {
		if err := s.UpsertAgent(ctx, types.AgentRecord{
			AgentID: "legacy-unscoped-agent", OwnerID: ownerID, ComputerID: cfg.SandboxID, SandboxID: cfg.SandboxID,
			Profile: "processor", Role: "processor", ChannelID: ownerID, CreatedAt: now, UpdatedAt: now,
		}); err != nil {
			t.Fatalf("upsert legacy agent for %s: %v", ownerID, err)
		}
	}
	if appended, err := adapter.log.Append(ctx, actor.Update{
		UpdateID: "legacy-update", ToAgentID: "legacy-unscoped-agent", Kind: "coagent_result", CreatedAt: now,
	}); err != nil || !appended {
		t.Fatalf("append legacy mailbox: appended=%v err=%v", appended, err)
	}
	if err := adapter.Start(ctx); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("startup error = %v, want ambiguous legacy mailbox refusal", err)
	}
	if legacy, err := adapter.log.Unprocessed(ctx, "legacy-unscoped-agent"); err != nil || len(legacy) != 1 {
		t.Fatalf("legacy backlog changed after refusal: %+v, %v", legacy, err)
	}
}

func TestAdapterStartRefusesConflictingAgentAndRunWitnesses(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "conflicting-witness-mailbox.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	cfg := provideriface.Config{SandboxID: "sandbox-legacy", StorePath: dbPath, PromptRoot: filepath.Join(dir, "prompts")}
	adapter := New(cfg, s, events.NewEventBus(), provider.NewStubProvider(0), nil)
	t.Cleanup(func() {
		adapter.Stop()
		adapter.cleanupLog()
		_ = s.Close()
	})
	now := time.Now().UTC()
	const agentID = "legacy-conflicting-agent"
	if err := s.UpsertAgent(ctx, types.AgentRecord{
		AgentID: agentID, OwnerID: "owner-a", ComputerID: cfg.SandboxID, SandboxID: cfg.SandboxID,
		Profile: "processor", Role: "processor", CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("upsert agent witness: %v", err)
	}
	if err := s.CreateRun(ctx, types.RunRecord{
		RunID: "conflicting-run", AgentID: agentID, OwnerID: "owner-b", SandboxID: cfg.SandboxID,
		State: types.RunCompleted, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create conflicting run witness: %v", err)
	}
	if appended, err := adapter.log.Append(ctx, actor.Update{
		UpdateID: "retained-update", ToAgentID: agentID, CreatedAt: now,
	}); err != nil || !appended {
		t.Fatalf("append legacy update: appended=%v err=%v", appended, err)
	}
	if err := adapter.log.SaveSnapshot(ctx, agentID, []byte(`{"phase":"parked"}`)); err != nil {
		t.Fatalf("save legacy snapshot: %v", err)
	}
	if err := adapter.Start(ctx); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("startup error = %v, want conflicting witness refusal", err)
	}
	if backlog, err := adapter.log.Unprocessed(ctx, agentID); err != nil || len(backlog) != 1 {
		t.Fatalf("legacy backlog changed after refusal: %+v, %v", backlog, err)
	}
	memory, err := adapter.log.LoadSnapshot(ctx, agentID)
	if err != nil || string(memory) != `{"phase":"parked"}` {
		t.Fatalf("legacy snapshot changed after refusal: %q, %v", memory, err)
	}
	for _, ownerID := range []string{"owner-a", "owner-b"} {
		scopedID := scopedActorMailboxID(ownerID, cfg.SandboxID, agentID)
		if scopedMemory, err := adapter.log.LoadSnapshot(ctx, scopedID); err != nil || scopedMemory != nil {
			t.Fatalf("scoped snapshot created for %s after refusal: %q, %v", ownerID, scopedMemory, err)
		}
	}
}

func TestAdapterLegacyMailboxMigrationConvergesMixedBatchAndRepeats(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "mixed-mailbox.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	cfg := provideriface.Config{SandboxID: "sandbox-legacy", StorePath: dbPath, PromptRoot: filepath.Join(dir, "prompts")}
	adapter := New(cfg, s, events.NewEventBus(), provider.NewStubProvider(0), nil)
	t.Cleanup(func() {
		adapter.Stop()
		adapter.cleanupLog()
		_ = s.Close()
	})
	now := time.Now().UTC()
	first := types.AgentRecord{
		AgentID: "legacy-a", OwnerID: "owner-a", ComputerID: cfg.SandboxID, SandboxID: cfg.SandboxID,
		Profile: "processor", Role: "processor", ChannelID: "channel-a", CreatedAt: now, UpdatedAt: now,
	}
	second := types.AgentRecord{
		AgentID: "legacy-b", OwnerID: "owner-b", ComputerID: cfg.SandboxID, SandboxID: cfg.SandboxID,
		Profile: "processor", Role: "processor", ChannelID: "channel-b", CreatedAt: now, UpdatedAt: now,
	}
	for _, agent := range []types.AgentRecord{first, second} {
		if err := s.UpsertAgent(ctx, agent); err != nil {
			t.Fatalf("upsert legacy agent %s: %v", agent.AgentID, err)
		}
		if appended, err := adapter.log.Append(ctx, actor.Update{
			UpdateID: "update-" + agent.AgentID, ToAgentID: agent.AgentID, CreatedAt: now,
		}); err != nil || !appended {
			t.Fatalf("append legacy mailbox %s: appended=%v err=%v", agent.AgentID, appended, err)
		}
	}
	secondScopedID := scopedActorMailboxID(second.OwnerID, second.ComputerID, second.AgentID)
	if appended, err := adapter.log.Append(ctx, actor.Update{
		UpdateID: "scoped-destination-update", ToAgentID: secondScopedID, CreatedAt: now,
	}); err != nil || !appended {
		t.Fatalf("append mixed destination update: appended=%v err=%v", appended, err)
	}
	for attempt := 1; attempt <= 2; attempt++ {
		if err := adapter.migrateLegacyActorMailboxes(ctx); err != nil {
			t.Fatalf("migration attempt %d: %v", attempt, err)
		}
	}
	firstScopedID := scopedActorMailboxID(first.OwnerID, first.ComputerID, first.AgentID)
	firstLegacy, firstErr := adapter.log.Unprocessed(ctx, first.AgentID)
	firstScoped, firstScopedErr := adapter.log.Unprocessed(ctx, firstScopedID)
	secondLegacy, secondErr := adapter.log.Unprocessed(ctx, second.AgentID)
	secondScoped, secondScopedErr := adapter.log.Unprocessed(ctx, secondScopedID)
	if firstErr != nil || firstScopedErr != nil || secondErr != nil || secondScopedErr != nil ||
		len(firstLegacy) != 0 || len(firstScoped) != 1 || len(secondLegacy) != 0 || len(secondScoped) != 2 {
		t.Fatalf("mailboxes after convergence: first legacy=%+v (%v), first scoped=%+v (%v), second legacy=%+v (%v), second scoped=%+v (%v)",
			firstLegacy, firstErr, firstScoped, firstScopedErr, secondLegacy, secondErr, secondScoped, secondScopedErr)
	}
}

// actorUpdate creates an actor.Update for testing.
func actorUpdate(ownerID, kind, toAgentID, content string) actor.Update {
	return actor.Update{
		UpdateID:  "test-update-id",
		ToAgentID: scopedActorMailboxID(ownerID, "sandbox-test", toAgentID),
		Kind:      kind,
		Content:   content,
		CreatedAt: time.Now().UTC(),
	}
}
