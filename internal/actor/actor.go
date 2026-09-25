// Package actor implements the durable-actor core of the runtime
// rearchitecture (docs/choir-rearchitecture-durable-actors-2026-06-11.md),
// conforming to specs/actor_protocol.tla.
//
// An agent is a durable actor: continuation obligations live on the tape as
// unprocessed updates, and the dispatcher's pending projection is the sole
// delivery authority. There is no Go-channel mailbox and no boot Sweep —
// pending work is a projection (tape events addressed to actors minus
// incorporated events), and the dispatcher fires one serial, fenced
// activation per actor until each pending event is incorporated.
//
//	The database remembers. The dispatcher delivers.
//
// Spec obligations honored here (see actor_protocol.tla header):
//   - sends dedupe on UpdateID (Log.Append is idempotent);
//   - delivered = in state head: an event is delivered exactly when the
//     addressed actor's fenced Commit marks it incorporated;
//   - serial-per-actor: the dispatcher never runs two activations of the
//     same actor;
//   - restart recovery is the pending projection, not a scan.
package actor

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// ErrDeferUnprocessed tells the dispatcher to retain the exact durable
// occurrence without incorporating it. The activation stops at the deferred
// head (tape order) and retries on the next projection read.
var ErrDeferUnprocessed = errors.New("actor: defer unprocessed occurrence")

// Update is the one agent-to-agent message primitive (update_coagent).
type Update struct {
	UpdateID     string
	ToAgentID    string
	FromAgentID  string
	Kind         string
	Content      string
	TrajectoryID string
	CreatedAt    time.Time
	// NotBefore, when set on an unaddressed scheduled event, defers the
	// dispatcher's minting of the addressed wake until that time. Zero means
	// immediately eligible.
	NotBefore time.Time
}

// Log is the durable append surface the runtime needs. Implementations must
// make Append idempotent on UpdateID and keep it crash-durable. The dispatch
// surface (pending projection, fenced commit, due-index) is KernelLog.
type Log interface {
	// Append durably stores the update. It returns false when an update with
	// the same UpdateID already exists (the resend no-op).
	Append(ctx context.Context, u Update) (bool, error)
}

// Handler incorporates one update for one agent. It receives the actor's
// working memory and returns the updated memory. Handlers emit further
// updates through the activation's emission buffer (flushed atomically with
// the fenced commit). A handler error leaves the update unprocessed; the
// dispatcher retries with durable attempt accounting (at-least-once).
//
// Handlers must be idempotent: the same Update may be delivered more than
// once across restarts. The handler should check durable state (e.g. run
// state) before acting.
type Handler interface {
	HandleUpdate(ctx context.Context, agentID string, u Update, memory []byte) ([]byte, error)
}

// HandlerFunc adapts a function to the Handler interface.
type HandlerFunc func(ctx context.Context, agentID string, u Update, memory []byte) ([]byte, error)

func (f HandlerFunc) HandleUpdate(ctx context.Context, agentID string, u Update, memory []byte) ([]byte, error) {
	return f(ctx, agentID, u, memory)
}

// Options bound the runtime.
type Options struct {
	// OnActorFailure is called when an actor's activation dies from a panic.
	// The callback receives the agent ID and the error. It must not block.
	// When nil, failures are logged only.
	OnActorFailure FailureFunc
}

// Runtime is the durable-append front door to the kernel. The dispatcher
// (Runtime.kernel) is the sole delivery authority; Runtime only appends to
// the log and signals the projection.
type Runtime struct {
	log    Log
	kernel *Dispatcher

	mu     sync.Mutex
	closed bool
}

// ErrClosed is returned by Send after Stop.
var ErrClosed = errors.New("actor runtime is closed")

// FailureFunc is called when an actor's activation dies from a panic. The
// supervisor receives the agent ID and the error. The callback must not
// block.
type FailureFunc func(agentID string, err error)

// NewKernelRuntime constructs a runtime whose delivery authority is the
// dispatcher's tape-derived pending projection. klog must be the same durable
// log as log (the kernel needs the projection and fenced-commit surface).
// The dispatcher runs until Stop/Drain.
//
// There is exactly one delivery authority: Send appends and signals the
// projection. This is the ontology-kernel cutover point — the legacy
// channel-mailbox runtime and its boot Sweep are removed.
func NewKernelRuntime(log Log, klog KernelLog, handler Handler, opts Options, dopts DispatcherOptions) *Runtime {
	dopts.OnActorFailure = opts.OnActorFailure
	return &Runtime{
		log:    log,
		kernel: NewDispatcher(klog, handler, dopts),
	}
}

// StartKernel launches the dispatcher loop. The pending projection is the
// recovery rule, so no separate boot scan is needed. Call once after
// NewKernelRuntime.
func (rt *Runtime) StartKernel(ctx context.Context) {
	if rt.kernel == nil {
		return
	}
	// Mark started before launching so a concurrent Stop observes it even if
	// the goroutine has not yet been scheduled to run its first line.
	rt.kernel.mu.Lock()
	rt.kernel.started = true
	rt.kernel.mu.Unlock()
	go rt.kernel.Run(ctx)
}

// Send durably appends the update, then signals the dispatcher's pending
// projection. The append IS the delivery — the dispatcher re-reads the
// projection regardless, so a missed signal only delays delivery by one
// PollInterval. A resend of an already-logged UpdateID is a no-op.
func (rt *Runtime) Send(ctx context.Context, u Update) error {
	u.ToAgentID = strings.TrimSpace(u.ToAgentID)
	u.UpdateID = strings.TrimSpace(u.UpdateID)
	if u.ToAgentID == "" || u.UpdateID == "" {
		return fmt.Errorf("actor send: update_id and to_agent_id are required")
	}
	if u.CreatedAt.IsZero() {
		u.CreatedAt = time.Now().UTC()
	}
	rt.mu.Lock()
	if rt.closed {
		rt.mu.Unlock()
		// The update is durably logged; the next process's projection
		// delivers it.
		return ErrClosed
	}
	rt.mu.Unlock()
	if _, err := rt.log.Append(ctx, u); err != nil {
		return fmt.Errorf("actor send: append: %w", err)
	}
	rt.kernel.Notify()
	return nil
}

// Stop halts the dispatcher and waits for in-flight activations to finish or
// fail their fenced commit. Durable state is untouched; a new runtime over
// the same log recovers via the pending projection.
func (rt *Runtime) Stop() {
	rt.kernel.Stop()
	rt.mu.Lock()
	rt.closed = true
	rt.mu.Unlock()
}

// Drain gracefully shuts down the actor runtime. The dispatcher's Stop waits
// for every in-flight activation to finish or fail its fenced commit, so no
// activation is orphaned across a restart. Durable state is untouched; a new
// runtime over the same log recovers via the pending projection.
//
// Drain is safe to call instead of Stop. It is also safe to call Stop after
// Drain.
func (rt *Runtime) Drain(timeout time.Duration) {
	rt.Stop()
}

// resetTimer drains and resets a timer, tolerating an already-fired timer.
func resetTimer(t *time.Timer, d time.Duration) {
	if !t.Stop() {
		select {
		case <-t.C:
		default:
		}
	}
	t.Reset(d)
}
