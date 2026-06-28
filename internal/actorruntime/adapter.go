// Package actorruntime adapts the durable actor runtime (internal/actor) to
// the surface that cmd/sandbox/main.go expects from the old runtime
// (internal/runtime).
//
// The Adapter embeds *runtime.Runtime for business logic access (tool loops,
// coagent spawning, state transitions, wire synthesis) and replaces the old
// runtime's concurrency substrate (startRunAsync, channels, agentWaiters,
// 15 mutexes) with the actor runtime's single-mutex mailbox model.
//
// The actor handler (handler.go) is the execution boundary: HandleUpdate calls
// runtime.ExecuteActivationSync synchronously. The actor goroutine IS the run
// goroutine. Park-resume is via the actor's memory snapshot (a compact resume
// pointer; the store holds the full conversation history).
package actorruntime

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"

	"github.com/yusefmosiah/go-choir/internal/actor"
	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/runtime"
	"github.com/yusefmosiah/go-choir/internal/store"
)

// RuntimeOption configures optional adapter components.
// This mirrors runtime.RuntimeOption so cmd/sandbox/main.go can use the same
// option pattern.
type RuntimeOption func(*Adapter)

// Adapter wraps an actor.Runtime to provide the same surface as the old
// runtime.Runtime. It embeds *runtime.Runtime for business logic access and
// replaces the concurrency substrate with the actor runtime.
//
// The Adapter sets a dispatch function on the embedded runtime: when the
// business logic calls rt.activate(rec) or rt.wakeUpdatedCoagent(...), the
// dispatch function sends actor messages through actor.Send.
type Adapter struct {
	*runtime.Runtime // embedded for business logic (promoted methods)

	cfg      provideriface.Config
	store    *store.Store
	bus      *events.EventBus
	provider provideriface.Provider
	actorRT  *actor.Runtime
	log      *actor.SQLiteLog
	logDB    *sql.DB
	logPath  string

	startOnce sync.Once
	started   bool
}

// New creates a new actor-based runtime adapter. It mirrors the old
// runtime.New signature so cmd/sandbox/main.go can switch with a single
// import change.
//
// The adapter creates a *runtime.Runtime for business logic, an actor runtime
// for concurrency, and wires them together: the runtime's ActorBridge is set
// to this adapter, so run activations and coagent wakes go through actor.Send.
func New(cfg provideriface.Config, s *store.Store, bus *events.EventBus, provider provideriface.Provider, opts ...RuntimeOption) *Adapter {
	// Create the business-logic runtime with the same options pattern.
	rtOpts := convertOpts(opts)
	rt := runtime.New(cfg, s, bus, provider, rtOpts...)

	a := &Adapter{
		Runtime:  rt,
		cfg:      cfg,
		store:    s,
		bus:      bus,
		provider: provider,
	}

	for _, opt := range opts {
		opt(a)
	}

	// Open a separate SQLite database for the actor durable log. The store
	// uses Dolt (MySQL-compatible); the actor log uses SQLite. The file
	// lives alongside the store so it survives restarts.
	logPath := actorLogPath(s.Path())
	logDB, err := sql.Open("sqlite", logPath+"?_busy_timeout=60000")
	if err != nil {
		log.Fatalf("actorruntime: open actor log db: %v", err)
	}
	actorLog, err := actor.NewSQLiteLog(logDB)
	if err != nil {
		_ = logDB.Close()
		log.Fatalf("actorruntime: init actor log schema: %v", err)
	}
	a.log = actorLog
	a.logDB = logDB
	a.logPath = logPath

	// Create the handler and actor runtime.
	handler := newActorHandler(rt)
	a.actorRT = actor.NewRuntime(actorLog, handler, actor.Options{
		MaxResident:         0, // unlimited for now
		HandlerRetryBackoff: 100 * time.Millisecond,
		MailboxCapacity:     256,
		IdleTimeout:         30 * time.Second,
	})

	// Wire the dispatch function. From this point, rt.activate(rec)
	// sends an actor message and rt.wakeUpdatedCoagent(...) sends an
	// actor message. No fallback path exists.
	rt.SetDispatchActor(a.dispatch)

	return a
}

// dispatch is the function hook that the embedded runtime calls to send
// actor messages. It is set via rt.SetDispatchActor(a.dispatch).
func (a *Adapter) dispatch(ctx context.Context, toAgentID, kind, content, trajectoryID, fromAgentID string) error {
	toAgentID = strings.TrimSpace(toAgentID)
	if toAgentID == "" {
		return fmt.Errorf("actorruntime: dispatch: empty toAgentID")
	}
	u := actor.Update{
		UpdateID:     uuid.New().String(),
		ToAgentID:    toAgentID,
		FromAgentID:  fromAgentID,
		Kind:         kind,
		Content:      content,
		TrajectoryID: trajectoryID,
		CreatedAt:    time.Now().UTC(),
	}
	return a.actorRT.Send(ctx, u)
}

// Start starts the runtime. It calls the embedded runtime's Start (which
// performs boot recovery: passivate interrupted runs, sweep pending actors,
// reconcile texture documents) and then sweeps the actor log to recover any
// actors with unprocessed backlog from a previous process.
func (a *Adapter) Start(ctx context.Context) {
	a.Runtime.Start(ctx)
	// Recover any actors with durable backlog from a previous process.
	if err := a.actorRT.Sweep(ctx); err != nil {
		log.Printf("actorruntime: boot sweep: %v", err)
	}
	a.startOnce.Do(func() { a.started = true })
}

// Stop gracefully shuts down the actor runtime and the embedded runtime.
func (a *Adapter) Stop() {
	a.actorRT.Stop()
	a.Runtime.Stop()
	if a.logDB != nil {
		_ = a.logDB.Close()
	}
}

// ActorRuntime returns the underlying actor runtime (for diagnostics/tests).
func (a *Adapter) ActorRuntime() *actor.Runtime {
	return a.actorRT
}

// convertOpts translates actorruntime.RuntimeOption into runtime.RuntimeOption.
// Since both are func(T) patterns on different types, and the Adapter embeds
// *runtime.Runtime, the options that target the Adapter are applied in New;
// the ones that should target the runtime are passed through. For now, all
// options target the Adapter (they configure adapter-level concerns). The
// runtime gets its own options via runtime.New's internal defaults.
func convertOpts(opts []RuntimeOption) []runtime.RuntimeOption {
	// No translation needed yet: runtime.RuntimeOption is applied inside
	// runtime.New via its own defaults. Adapter options configure the
	// adapter (e.g., MaxResident in the future). If an option needs to
	// reach the embedded runtime, it can do so via a.Adapter.Runtime.
	return nil
}

// actorLogPath derives the actor log SQLite file path from the store path.
func actorLogPath(storePath string) string {
	dir := filepath.Dir(storePath)
	base := filepath.Base(storePath)
	return filepath.Join(dir, base+"-actor.db")
}

// Cleanup removes the actor log file (for tests).
func (a *Adapter) cleanupLog() {
	if a.logDB != nil {
		_ = a.logDB.Close()
		a.logDB = nil
	}
	if a.logPath != "" {
		_ = os.Remove(a.logPath)
	}
}
