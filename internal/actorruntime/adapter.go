// Package actorruntime adapts the durable actor runtime (internal/actor) to
// the surface that cmd/sandbox/main.go expects from the old runtime
// (internal/runtime).
//
// STATUS: skeleton — the swap point is created but the full adapter is not
// yet implemented. See docs/mission-3c-actor-runtime-migration-v0.md for the
// migration plan and the open_handoff status documenting why the full adapter
// requires separating 3797 lines of intertwined business logic and concurrency
// code in the old runtime.
//
// The key challenge: the old *runtime.Runtime has 71+ methods containing both
// business logic (StartRun, tool loops, coagent spawning, state transitions)
// and concurrency code (channels, mutexes, goroutine management). The actor
// runtime (internal/actor) provides only the concurrency substrate (Send,
// Sweep, Evict, Stop). Building the adapter requires extracting the business
// logic from the old runtime and reimplementing it on top of the actor model.
//
// The next agent should:
// 1. Identify the business logic methods that must be preserved (StartRun,
//    executeActivation, executeWithToolLoop, etc.)
// 2. Implement them as actor handlers that use actor.Runtime.Send/Sweep
//    for concurrency instead of the old channels/mutexes
// 3. Wire the adapter to provide the same surface as runtime.New
package actorruntime

import (
	"github.com/yusefmosiah/go-choir/internal/actor"
	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/store"
)

// RuntimeOption configures optional adapter components.
// This mirrors runtime.RuntimeOption so cmd/sandbox/main.go can use the same
// option pattern.
type RuntimeOption func(*Adapter)

// Adapter wraps an actor.Runtime to provide the same surface as the old
// runtime.Runtime. It is the production concurrency substrate replacement.
//
// TODO(mission-3c): implement all methods that cmd/sandbox/main.go and
// APIHandler call on *runtime.Runtime. The initial implementation should
// delegate to the old runtime while the business logic is gradually migrated
// to actor-based execution.
type Adapter struct {
	cfg      provideriface.Config
	store    *store.Store
	bus      *events.EventBus
	provider provideriface.Provider
	actorRT  *actor.Runtime
}

// New creates a new actor-based runtime adapter. It mirrors the old
// runtime.New signature so cmd/sandbox/main.go can switch with a single
// import change.
//
// TODO(mission-3c): implement the full adapter. The current skeleton creates
// the actor runtime but does not wire business logic.
func New(cfg provideriface.Config, s *store.Store, bus *events.EventBus, provider provideriface.Provider, opts ...RuntimeOption) *Adapter {
	a := &Adapter{
		cfg:      cfg,
		store:    s,
		bus:      bus,
		provider: provider,
	}
	for _, opt := range opts {
		opt(a)
	}
	// The actor runtime is created here but not yet wired to business logic.
	// A Log implementation backed by the store and a Handler implementation
	// that runs the tool loop must be provided.
	// a.actorRT = actor.NewRuntime(log, handler, actor.Options{...})
	return a
}
