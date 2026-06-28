package actorruntime

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/runtime"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// TestAdapterStartRunExecutesViaActorHandler verifies that a run started via
// the Adapter executes through the actor handler (not startRunAsync) and
// completes. This is the Phase 1 existential test: the actor handler IS the
// execution boundary.
func TestAdapterStartRunExecutesViaActorHandler(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	promptRoot := filepath.Join(dir, "prompts")

	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	cfg := runtime.Config{
		SandboxID:           "sandbox-test",
		StorePath:           dbPath,
		PromptRoot:          promptRoot,
		ProviderTimeout:     time.Second,
		SupervisionInterval: time.Hour,
	}

	adapter := New(cfg, s, events.NewEventBus(), runtime.NewStubProvider(0))
	t.Cleanup(func() {
		adapter.Stop()
		adapter.cleanupLog()
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	adapter.Start(ctx)

	// Start a run. In actor mode, activate() sends an initial_dispatch
	// actor message. The handler picks it up and calls
	// ExecuteActivationSync synchronously in the actor goroutine.
	rec, err := adapter.StartRun(ctx, "Test prompt for actor handler", "test-owner")
	if err != nil {
		t.Fatalf("StartRun: %v", err)
	}
	if rec.RunID == "" {
		t.Fatal("StartRun returned empty run ID")
	}

	// Wait for the run to complete. The stub provider returns immediately.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		loaded, err := s.GetRun(ctx, rec.RunID)
		if err != nil {
			t.Fatalf("GetRun: %v", err)
		}
		if loaded.State.Terminal() {
			if loaded.State != types.RunCompleted {
				t.Fatalf("run state = %s, want RunCompleted", loaded.State)
			}
			if loaded.Result == "" {
				t.Fatal("run completed but result is empty")
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("run %s did not complete within 5s (state=%s)", rec.RunID, rec.State)
}

// TestAdapterActorBridgeActive verifies that the Adapter wires the actor
// bridge on the embedded runtime.
func TestAdapterActorBridgeActive(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	promptRoot := filepath.Join(dir, "prompts")

	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	cfg := runtime.Config{
		SandboxID:           "sandbox-test",
		StorePath:           dbPath,
		PromptRoot:          promptRoot,
		ProviderTimeout:     time.Second,
		SupervisionInterval: time.Hour,
	}

	adapter := New(cfg, s, events.NewEventBus(), runtime.NewStubProvider(0))
	t.Cleanup(func() {
		adapter.Stop()
		adapter.cleanupLog()
	})

	if !adapter.Runtime.ActorBridgeActive() {
		t.Fatal("ActorBridgeActive() = false, want true (adapter should wire the bridge)")
	}
	if adapter.ActorRuntime() == nil {
		t.Fatal("ActorRuntime() = nil, want non-nil")
	}
}
