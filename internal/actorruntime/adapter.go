// Package actorruntime adapts the durable actor runtime (internal/actor) to
// the surface that cmd/sandbox/main.go expects from the old runtime
// (internal/runtime).

// The Adapter retains a named runtime core for business logic (tool loops,
// coagent spawning, state transitions, wire synthesis) and replaces the
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
	"github.com/yusefmosiah/go-choir/internal/agentcore"
	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/textureowner"
	"github.com/yusefmosiah/go-choir/internal/trace"
)

// RuntimeOption configures optional adapter components.
type RuntimeOption func(*Adapter)

// WithTraceStore mounts a Dolt-backed trace observability store into the
// runtime core so trace events are persisted alongside existing event
// recording. This is a passthrough to runtime.WithTraceStore; the adapter does
// not own the store connection (the caller manages the *sql.DB lifecycle).
func WithTraceStore(s trace.Store) RuntimeOption {
	return func(a *Adapter) {
		agentcore.WithTraceStore(s)(a.Runtime)
	}
}

// WithInboxCapacity sets the mailbox capacity for each actor (default: 1000
// in the adapter, 256 in the bare actor runtime). This bounds the Go-channel
// buffer. When the buffer is full, behavior depends on whether backpressure
// is enabled (see WithBackpressure).
func WithInboxCapacity(n int) RuntimeOption {
	return func(a *Adapter) {
		if n > 0 {
			a.inboxCapacity = n
		}
	}
}

// WithSendTimeout sets the timeout for blocking Send when the mailbox is full
// and backpressure is enabled in blocking mode (default 5s).
func WithSendTimeout(d time.Duration) RuntimeOption {
	return func(a *Adapter) {
		if d > 0 {
			a.sendTimeout = d
		}
	}
}

// WithBackpressure enables backpressure on Send. When the mailbox is full:
//   - blocking=false: Send returns actor.ErrInboxFull immediately
//     (non-blocking backpressure).
//   - blocking=true: Send waits up to WithSendTimeout for space, then
//     returns actor.ErrInboxFull (blocking backpressure).
//
// Without this option, Send silently drops to the durable log when the
// mailbox is full (legacy behavior, backward compatible).
func WithBackpressure(blocking bool) RuntimeOption {
	return func(a *Adapter) {
		a.backpressure = true
		if blocking {
			a.sendMode = actor.SendModeBlocking
		} else {
			a.sendMode = actor.SendModeNonBlocking
		}
	}
}

// WithOnActorFailure sets a callback invoked when an actor dies from a panic
// or unrecoverable error. The callback receives the agent ID and the error.
// It must not block (it is called from the dying actor's goroutine). When
// not set, failures are logged only.
func WithOnActorFailure(fn func(agentID string, err error)) RuntimeOption {
	return func(a *Adapter) {
		if fn != nil {
			a.onActorFailure = fn
		}
	}
}

// Adapter owns actor dispatch and lifecycle around an explicitly named runtime
// business-logic core. Naming the field keeps the runtime method set from being
// promoted onto the adapter.
//
// The Adapter sets a dispatch function on the runtime core: when the business
// logic activates a run or wakes a coagent, the dispatch function sends actor
// messages through actor.Send.
type Adapter struct {
	Runtime *agentcore.Runtime

	cfg          provideriface.Config
	store        *store.Store
	bus          *events.EventBus
	provider     provideriface.Provider
	actorRT      *actor.Runtime
	log          *actor.SQLiteLog
	handler      *actorHandler
	textureOwner *textureowner.Handler
	logDB        *sql.DB
	logPath      string

	// Actor runtime options (applied before actorRT construction).
	inboxCapacity  int               // 0 = use actor default
	backpressure   bool              // opt-in backpressure on Send
	sendMode       actor.SendMode    // non-blocking (default) or blocking
	sendTimeout    time.Duration     // blocking send timeout (default 5s)
	onActorFailure actor.FailureFunc // supervisor callback for actor deaths

	startOnce sync.Once
	started   bool

	dispatchMu     sync.Mutex
	dispatchReady  bool
	bootDispatches []actor.Update
}

// New creates a runtime business-logic core and its actor-based lifecycle
// adapter. The core remains explicitly available as Adapter.Runtime without
// promoting its method set onto Adapter.
//
// The runtime core's ActorBridge is set to the adapter, so run activations and
// coagent wakes go through actor.Send.
func New(cfg provideriface.Config, s *store.Store, bus *events.EventBus, provider provideriface.Provider, coreOpts []agentcore.RuntimeOption, opts ...RuntimeOption) *Adapter {
	rt := agentcore.New(cfg, s, bus, provider, coreOpts...)

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

	// Create the handler and actor runtime. Texture ownership is bound by the
	// composition root before Start.
	handler := newActorHandler(a.Runtime, nil)
	a.handler = handler
	actorOpts := actor.Options{
		MaxResident:         0, // unlimited for now
		HandlerRetryBackoff: 100 * time.Millisecond,
		MailboxCapacity:     1000, // adapter default; override via WithInboxCapacity
		IdleTimeout:         30 * time.Second,
	}
	if a.inboxCapacity > 0 {
		actorOpts.MailboxCapacity = a.inboxCapacity
	}
	if a.backpressure {
		actorOpts.Backpressure = true
		actorOpts.SendMode = a.sendMode
		if a.sendTimeout > 0 {
			actorOpts.SendTimeout = a.sendTimeout
		}
	}
	if a.onActorFailure != nil {
		actorOpts.OnActorFailure = a.onActorFailure
	}
	a.actorRT = actor.NewRuntime(actorLog, handler, actorOpts)

	// Wire the dispatch function. From this point, rt.activate(rec)
	// sends an actor message and rt.wakeUpdatedCoagent(...) sends an
	// actor message. No fallback path exists.
	rt.SetDispatchActor(a.dispatch)

	return a
}

func actorDispatchUpdateID(ownerID, computerID, toAgentID, kind, content string) string {
	if kind == "initial_dispatch" && strings.TrimSpace(content) != "" {
		return uuid.NewSHA1(uuid.NameSpaceOID, []byte(strings.Join(
			[]string{"choir:initial-dispatch", ownerID, computerID, toAgentID, strings.TrimSpace(content)}, "\x00",
		))).String()
	}
	return uuid.New().String()
}

// dispatch is the function hook that the runtime core calls to send actor
// messages. It is set via rt.SetDispatchActor(a.dispatch).
func (a *Adapter) dispatch(ctx context.Context, ownerID, computerID, toAgentID, kind, content, trajectoryID, fromAgentID string) error {
	ownerID, computerID, toAgentID = strings.TrimSpace(ownerID), strings.TrimSpace(computerID), strings.TrimSpace(toAgentID)
	if ownerID == "" || computerID == "" || toAgentID == "" {
		return fmt.Errorf("actorruntime: dispatch: owner_id, computer_id, and to_agent_id are required")
	}
	updateID := actorDispatchUpdateID(ownerID, computerID, toAgentID, kind, content)
	u := actor.Update{
		UpdateID:     updateID,
		ToAgentID:    scopedActorMailboxID(ownerID, computerID, toAgentID),
		FromAgentID:  fromAgentID,
		Kind:         kind,
		Content:      content,
		TrajectoryID: trajectoryID,
		CreatedAt:    time.Now().UTC(),
	}
	a.dispatchMu.Lock()
	if !a.dispatchReady {
		a.bootDispatches = append(a.bootDispatches, u)
		a.dispatchMu.Unlock()
		return nil
	}
	a.dispatchMu.Unlock()
	return a.actorRT.Send(ctx, u)
}

func (a *Adapter) flushBootDispatches(ctx context.Context) error {
	for {
		a.dispatchMu.Lock()
		if len(a.bootDispatches) == 0 {
			a.dispatchReady = true
			a.dispatchMu.Unlock()
			return nil
		}
		pending := a.bootDispatches
		a.bootDispatches = nil
		a.dispatchMu.Unlock()

		for index, update := range pending {
			attempt := 0
			for {
				err := a.actorRT.Send(ctx, update)
				if err == nil {
					break
				}
				attempt++
				if attempt == 1 || attempt%20 == 0 {
					log.Printf("actorruntime: retry boot dispatch to %s: %v", update.ToAgentID, err)
				}
				timer := time.NewTimer(50 * time.Millisecond)
				select {
				case <-ctx.Done():
					if !timer.Stop() {
						<-timer.C
					}
					a.dispatchMu.Lock()
					a.bootDispatches = append(pending[index:], a.bootDispatches...)
					a.dispatchMu.Unlock()
					return ctx.Err()
				case <-timer.C:
				}
			}
		}
	}
}

// BindTextureOwner installs the concrete Texture lifecycle owner before actor
// processing begins. It is a direct owner composition, not a callback seam.
func (a *Adapter) BindTextureOwner(owner *textureowner.Handler) error {
	if owner == nil {
		return fmt.Errorf("actorruntime: bind Texture owner: nil owner")
	}
	if a.started {
		return fmt.Errorf("actorruntime: bind Texture owner after start")
	}
	a.textureOwner = owner
	a.handler.textureOwner = owner
	return nil
}

// Start keeps actor delivery paused while the generic core and concrete Texture
// owner reconcile durable state. Only after both scans finish are boot
// dispatches released and the actor log swept.
func (a *Adapter) Start(ctx context.Context) error {
	backlogs, err := a.log.AgentsWithBacklog(ctx)
	if err != nil {
		return fmt.Errorf("actorruntime: inspect durable mailbox identities: %w", err)
	}
	for _, mailboxID := range backlogs {
		if _, _, _, err := parseScopedActorMailboxID(mailboxID); err != nil {
			return fmt.Errorf("actorruntime: unsupported legacy durable mailbox %q: %w", mailboxID, err)
		}
	}
	a.Runtime.Start(ctx)
	if a.textureOwner != nil {
		a.textureOwner.Start(ctx)
	}
	if err := a.flushBootDispatches(ctx); err != nil {
		return fmt.Errorf("actorruntime: boot dispatch flush: %w", err)
	}
	if err := a.actorRT.Sweep(ctx); err != nil {
		return fmt.Errorf("actorruntime: boot sweep: %w", err)
	}
	a.startOnce.Do(func() { a.started = true })
	return nil
}

// Stop gracefully shuts down the actor runtime and the runtime core.
func (a *Adapter) Stop() {
	a.actorRT.Stop()
	a.Runtime.Stop()
	if a.logDB != nil {
		_ = a.logDB.Close()
	}
}

// Drain gracefully shuts down the actor runtime with a timeout, then stops
// the runtime core. In-flight actor handlers receive a cancellation context;
// actors that do not finish within the timeout are logged (their partial side
// effects are visible in the durable log). This is the backpressure-aware
// alternative to Stop.
func (a *Adapter) Drain(timeout time.Duration) {
	a.actorRT.Drain(timeout)
	a.Runtime.Stop()
	if a.logDB != nil {
		_ = a.logDB.Close()
	}
}

// ActorRuntime returns the underlying actor runtime (for diagnostics/tests).
func (a *Adapter) ActorRuntime() *actor.Runtime {
	return a.actorRT
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
