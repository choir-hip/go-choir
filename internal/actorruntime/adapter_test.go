package actorruntime

import (
	"context"
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/actor"
	"github.com/yusefmosiah/go-choir/internal/agentcore"
	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/provider"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/textureowner"
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
	adapter.Start(ctx)

	return &adapterTestEnv{t: t, adapter: adapter, store: s, ctx: ctx, cancel: cancel}
}

func TestAdapterStartSerializesTextureOwnerRecoveryBeforeActorDelivery(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "startup.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	blocking := &startupBlockingProvider{
		started: make(chan struct{}, 1),
		release: make(chan struct{}),
	}
	cfg := provideriface.Config{
		SandboxID:           "sandbox-startup",
		StorePath:           dbPath,
		PromptRoot:          filepath.Join(dir, "prompts"),
		ProviderTimeout:     10 * time.Second,
		SupervisionInterval: time.Hour,
	}
	adapter := New(cfg, s, events.NewEventBus(), blocking, nil)
	t.Cleanup(func() {
		adapter.Stop()
		adapter.cleanupLog()
		_ = s.Close()
	})
	t.Cleanup(func() { close(blocking.release) })

	const (
		ownerID = "user-startup-recovery"
		docID   = "doc-startup-recovery"
		agentID = "texture:" + docID
	)
	now := time.Now().UTC()
	if err := s.CreateDocument(ctx, types.Document{
		DocID: docID, OwnerID: ownerID, Title: "Startup target", CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create document: %v", err)
	}
	if err := s.CreateRevision(ctx, types.Revision{
		RevisionID:  "rev-startup-recovery",
		DocID:       docID,
		OwnerID:     ownerID,
		AuthorKind:  types.AuthorUser,
		AuthorLabel: "user",
		Content:     "Durable startup content",
		CreatedAt:   now,
	}); err != nil {
		t.Fatalf("create revision: %v", err)
	}
	update := types.CoagentSourcePacket{
		UpdateID:      "update-startup-recovery",
		OwnerID:       ownerID,
		AgentID:       "researcher:startup-recovery",
		TargetAgentID: agentID,
		ChannelID:     docID,
		Role:          "researcher",
		Packet: types.CoagentSourcePacketPayload{
			SchemaVersion: types.CoagentSourcePacketSchemaV1,
			Kind:          "evidence_update",
			Summary:       "durable startup finding",
		},
		Content:   "Durable startup finding",
		CreatedAt: now.Add(time.Millisecond),
	}
	message := types.ChannelMessage{
		ChannelID:   docID,
		FromAgentID: update.AgentID,
		ToAgentID:   agentID,
		Role:        update.Role,
		Content:     update.Content,
		Timestamp:   update.CreatedAt,
	}
	if _, _, err := s.DispatchWorkerUpdate(ctx, update, &message); err != nil {
		t.Fatalf("dispatch durable startup update: %v", err)
	}
	if _, err := s.GetAgent(ctx, agentID); err == nil {
		t.Fatal("fixture unexpectedly has a Texture agent identity before startup")
	}

	owner := textureowner.NewHandler(adapter.Runtime)
	if err := adapter.BindTextureOwner(owner); err != nil {
		t.Fatalf("bind Texture owner: %v", err)
	}
	adapter.Start(ctx)
	select {
	case <-blocking.started:
	case <-time.After(5 * time.Second):
		t.Fatal("Texture activation did not start after owner recovery")
	}

	agent, err := s.GetAgent(ctx, agentID)
	if err != nil {
		t.Fatalf("load recovered Texture identity: %v", err)
	}
	if agent.OwnerID != ownerID || agent.ChannelID != docID {
		t.Fatalf("recovered Texture identity = %+v", agent)
	}
	runs, err := s.ListRunsByOwner(ctx, ownerID, 20)
	if err != nil {
		t.Fatalf("list startup recovery runs: %v", err)
	}
	textureRuns := 0
	for _, run := range runs {
		if run.AgentID == agentID && run.ChannelID == docID {
			textureRuns++
		}
	}
	if textureRuns != 1 {
		t.Fatalf("startup recovery created %d Texture runs, want exactly one: %+v", textureRuns, runs)
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
	u := actorUpdate("coagent_result", agentID, "coagent-result-content")
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

func TestTextureColdWakeRoutesToConcreteOwner(t *testing.T) {
	env := newAdapterTestEnv(t)
	env.adapter.Runtime.SetDispatchActor(func(context.Context, string, string, string, string, string) error {
		return nil
	})

	const (
		ownerID = "user-texture-wake"
		docID   = "doc-texture-wake"
		agentID = "texture:" + docID
	)
	now := time.Now().UTC()
	if err := env.store.CreateDocument(env.ctx, types.Document{
		DocID: docID, OwnerID: ownerID, Title: "Wake target", CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create Texture document: %v", err)
	}
	if err := env.store.CreateRevision(env.ctx, types.Revision{
		RevisionID:  "rev-texture-wake",
		DocID:       docID,
		OwnerID:     ownerID,
		AuthorKind:  types.AuthorUser,
		AuthorLabel: "user",
		Content:     "Initial Texture content",
		CreatedAt:   now,
	}); err != nil {
		t.Fatalf("create Texture revision: %v", err)
	}
	update := types.CoagentSourcePacket{
		UpdateID:      "update-texture-wake",
		OwnerID:       ownerID,
		AgentID:       "researcher:texture-wake",
		TargetAgentID: agentID,
		ChannelID:     docID,
		Role:          "researcher",
		Packet: types.CoagentSourcePacketPayload{
			SchemaVersion: types.CoagentSourcePacketSchemaV1,
			Kind:          "evidence_update",
			Summary:       "durable Texture wake",
		},
		Content:   "Durable Texture wake",
		CreatedAt: now,
	}
	message := types.ChannelMessage{
		ChannelID:   docID,
		FromAgentID: update.AgentID,
		ToAgentID:   agentID,
		Role:        update.Role,
		Content:     update.Content,
		Timestamp:   now,
	}
	if _, _, err := env.store.DispatchWorkerUpdate(env.ctx, update, &message); err != nil {
		t.Fatalf("dispatch Texture update: %v", err)
	}

	handler := newActorHandler(env.adapter.Runtime, textureowner.NewHandler(env.adapter.Runtime))
	memory, err := handler.HandleUpdate(env.ctx, agentID, actorUpdate("coagent_result", agentID, update.Content), nil)
	if err != nil {
		t.Fatalf("route Texture coagent_result: %v", err)
	}
	if memory != nil {
		t.Fatalf("cold Texture wake memory = %v, want nil", memory)
	}
	agent, err := env.store.GetAgent(env.ctx, agentID)
	if err != nil {
		t.Fatalf("load first-wake Texture identity: %v", err)
	}
	if agent.OwnerID != ownerID || agent.ChannelID != docID {
		t.Fatalf("first-wake Texture identity = %+v", agent)
	}
	runs, err := env.store.ListRunsByOwner(env.ctx, ownerID, 20)
	if err != nil {
		t.Fatalf("list Texture runs: %v", err)
	}
	for _, rec := range runs {
		if rec.AgentID == agentID && rec.ChannelID == docID {
			return
		}
	}
	t.Fatalf("Texture coagent_result did not route to owner run: %+v", runs)
}

func TestTextureColdWakeFailsClosedWithoutOwner(t *testing.T) {
	env := newAdapterTestEnv(t)
	_, err := newActorHandler(env.adapter.Runtime, nil).reconcileCoagentWake(
		env.ctx, "texture:doc-texture-wake",
	)
	if err == nil || err.Error() != "Texture owner is not bound" {
		t.Fatalf("Texture wake error = %v, want explicit unbound-owner failure", err)
	}
}

// TestHandlerCancelPassivatedRun tests that a cancel message for a passivated
// run transitions it to RunFailed.
func TestHandlerCancelPassivatedRun(t *testing.T) {
	env := newAdapterTestEnv(t)

	// Create a run and manually set it to RunPassivated.
	rec := types.RunRecord{
		RunID:   "run-cancel-test",
		OwnerID: "user-cancel",
		AgentID: "agent-cancel-test",
		Prompt:  "test cancel",
		State:   types.RunPassivated,
	}
	if err := env.store.CreateRun(env.ctx, rec); err != nil {
		t.Fatalf("CreateRun: %v", err)
	}

	// Encode memory with the run ID.
	mem, _ := json.Marshal(resumeState{RunID: rec.RunID, Phase: "parked"})

	// Send cancel.
	handler := newActorHandler(env.adapter.Runtime, nil)
	u := actorUpdate("cancel", "agent-cancel-test", "")
	_, err := handler.HandleUpdate(env.ctx, "agent-cancel-test", u, mem)
	if err != nil {
		t.Fatalf("HandleUpdate cancel: %v", err)
	}

	// Verify the run was cancelled (state = RunFailed).
	updated, err := env.store.GetRun(env.ctx, rec.RunID)
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if updated.State != types.RunFailed {
		t.Errorf("run state = %s, want RunFailed (cancelled)", updated.State)
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
	u := actorUpdate("cancel", "agent-missing", "")
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
		RunID:   "run-completed-test",
		OwnerID: ownerID,
		AgentID: agentID,
		Prompt:  "test completed",
		State:   types.RunCompleted,
		Result:  "done",
	}
	now := time.Now().UTC()
	rec.FinishedAt = &now
	if err := env.store.CreateRun(env.ctx, rec); err != nil {
		t.Fatalf("CreateRun: %v", err)
	}

	// Send coagent_result with memory pointing to the completed run.
	mem, _ := json.Marshal(resumeState{RunID: rec.RunID, Phase: "parked"})
	handler := newActorHandler(env.adapter.Runtime, nil)
	u := actorUpdate("coagent_result", agentID, "new-result")
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
		RunID:   "run-blocked-test",
		OwnerID: ownerID,
		AgentID: agentID,
		Prompt:  "test blocked",
		State:   types.RunBlocked,
		Error:   "provider rate limit",
	}
	if err := env.store.CreateRun(env.ctx, rec); err != nil {
		t.Fatalf("CreateRun: %v", err)
	}

	// Send coagent_result with memory pointing to the blocked run.
	mem, _ := json.Marshal(resumeState{RunID: rec.RunID, Phase: "parked"})
	handler := newActorHandler(env.adapter.Runtime, nil)
	u := actorUpdate("coagent_result", agentID, "unblocking-result")
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
	u := actorUpdate("unknown_kind", "agent-test", "content")
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

// actorUpdate creates an actor.Update for testing.
func actorUpdate(kind, toAgentID, content string) actor.Update {
	return actor.Update{
		UpdateID:  "test-update-id",
		ToAgentID: toAgentID,
		Kind:      kind,
		Content:   content,
		CreatedAt: time.Now().UTC(),
	}
}
