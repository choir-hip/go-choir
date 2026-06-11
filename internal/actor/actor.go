// Package actor implements the durable-actor core of the runtime
// rearchitecture (docs/choir-rearchitecture-durable-actors-2026-06-11.md),
// conforming to specs/actor_protocol.tla.
//
// An agent is a long-lived actor: a goroutine with an in-memory mailbox while
// resident, an idempotent durable update log plus a compacted memory snapshot
// while passivated. Sending to a cold actor activates it; sending to a warm
// actor steers it. Actors never "complete" — they passivate on quiescence and
// re-warm on the next send or sweep.
//
//	The database remembers. Go delivers.
//
// Spec obligations honored here (see actor_protocol.tla header):
//   - sends dedupe on UpdateID (Log.Append is idempotent);
//   - {residency check + mailbox delivery} and {idle check + deregister}
//     are atomic with respect to each other (both hold Runtime.mu);
//   - the boot/periodic Sweep activates any agent with unprocessed backlog,
//     covering crash windows and post-eviction re-wake;
//   - eviction is crash-equivalent: no snapshot save, backlog stays durable,
//     the sweep re-activates.
package actor

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Update is the one agent-to-agent message primitive (update_coagent).
type Update struct {
	UpdateID     string
	ToAgentID    string
	FromAgentID  string
	Kind         string
	Content      string
	TrajectoryID string
	CreatedAt    time.Time
}

// Log is the durable side of the protocol. Implementations must make Append
// idempotent on UpdateID and keep everything here crash-durable.
type Log interface {
	// Append durably stores the update. It returns false when an update with
	// the same UpdateID already exists (the resend no-op).
	Append(ctx context.Context, u Update) (bool, error)
	// Unprocessed returns the durable backlog for an agent in append order.
	Unprocessed(ctx context.Context, agentID string) ([]Update, error)
	// MarkProcessed durably records that the agent incorporated the update.
	MarkProcessed(ctx context.Context, agentID, updateID string) error
	// AgentsWithBacklog lists agents that have unprocessed updates (the
	// sweep's query).
	AgentsWithBacklog(ctx context.Context) ([]string, error)
	// SaveSnapshot / LoadSnapshot persist the actor's compacted memory.
	// LoadSnapshot returns nil memory for an agent with no snapshot.
	SaveSnapshot(ctx context.Context, agentID string, memory []byte) error
	LoadSnapshot(ctx context.Context, agentID string) ([]byte, error)
}

// Handler incorporates one update for one agent. It receives the actor's
// working memory and returns the updated memory. Handlers send further
// updates through Runtime.Send. A handler error leaves the update
// unprocessed; the loop retries with backoff (at-least-once visibility).
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
	// MaxResident caps concurrent activations (0 = unlimited). The cap is
	// load-bearing for liveness in the bounded-eviction sense: it is the
	// implementation of the spec's MaxEvictions bound.
	MaxResident int
	// HandlerRetryBackoff is the delay before retrying a failed handler
	// (default 100ms).
	HandlerRetryBackoff time.Duration
}

// Runtime hosts resident actors over a durable log.
type Runtime struct {
	log     Log
	handler Handler
	opts    Options

	mu       sync.Mutex
	resident map[string]*residentActor
	closed   bool
	wg       sync.WaitGroup
}

type residentActor struct {
	agentID string
	pending []Update // warm-steer deliveries since the loop's last log query
	cancel  context.CancelFunc
	evicted bool
}

// ErrClosed is returned by Send after Stop.
var ErrClosed = errors.New("actor runtime is closed")

// NewRuntime constructs a runtime. Call Sweep afterwards to recover any
// backlog left by a previous process (boot recovery).
func NewRuntime(log Log, handler Handler, opts Options) *Runtime {
	if opts.HandlerRetryBackoff <= 0 {
		opts.HandlerRetryBackoff = 100 * time.Millisecond
	}
	return &Runtime{
		log:      log,
		handler:  handler,
		opts:     opts,
		resident: map[string]*residentActor{},
	}
}

// Send durably appends the update, then delivers it: into the recipient's
// mailbox if warm (steering), by activating it if cold. A resend of an
// already-logged UpdateID is a no-op. Ledger effects keyed to specific kinds
// belong in the same transaction as Append (Log implementations own this).
func (rt *Runtime) Send(ctx context.Context, u Update) error {
	u.ToAgentID = strings.TrimSpace(u.ToAgentID)
	u.UpdateID = strings.TrimSpace(u.UpdateID)
	if u.ToAgentID == "" || u.UpdateID == "" {
		return fmt.Errorf("actor send: update_id and to_agent_id are required")
	}
	if u.CreatedAt.IsZero() {
		u.CreatedAt = time.Now().UTC()
	}
	appended, err := rt.log.Append(ctx, u)
	if err != nil {
		return fmt.Errorf("actor send: append: %w", err)
	}
	if !appended {
		return nil // resend: durable state unchanged, no redelivery
	}

	rt.mu.Lock()
	defer rt.mu.Unlock()
	if rt.closed {
		// The update is durably logged; the next process's sweep delivers it.
		return ErrClosed
	}
	if r, ok := rt.resident[u.ToAgentID]; ok && !r.evicted {
		r.pending = append(r.pending, u) // warm: steer
		return nil
	}
	return rt.activateLocked(u.ToAgentID) // cold: wake
}

// Sweep activates every agent with unprocessed backlog. It is the boot
// recovery rule and the post-eviction re-wake rule in one.
func (rt *Runtime) Sweep(ctx context.Context) error {
	agents, err := rt.log.AgentsWithBacklog(ctx)
	if err != nil {
		return fmt.Errorf("actor sweep: %w", err)
	}
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if rt.closed {
		return ErrClosed
	}
	for _, agentID := range agents {
		if _, ok := rt.resident[agentID]; ok {
			continue
		}
		if err := rt.activateLocked(agentID); err != nil {
			return err
		}
	}
	return nil
}

// activateLocked starts an actor goroutine. Caller holds rt.mu.
func (rt *Runtime) activateLocked(agentID string) error {
	if rt.opts.MaxResident > 0 && len(rt.resident) >= rt.opts.MaxResident {
		// Backlog stays durable; a later sweep re-attempts. This bound is the
		// liveness-critical activation cap.
		return fmt.Errorf("actor activate %s: resident cap %d reached; backlog retained for sweep", agentID, rt.opts.MaxResident)
	}
	ctx, cancel := context.WithCancel(context.Background())
	r := &residentActor{agentID: agentID, cancel: cancel}
	rt.resident[agentID] = r
	rt.wg.Add(1)
	go rt.loop(ctx, r)
	return nil
}

// Evict force-passivates a resident actor: crash-equivalent. No snapshot is
// saved; unprocessed backlog stays in the log; Sweep re-activates.
func (rt *Runtime) Evict(agentID string) {
	rt.mu.Lock()
	r, ok := rt.resident[agentID]
	if ok {
		r.evicted = true
	}
	rt.mu.Unlock()
	if ok {
		r.cancel()
	}
}

// Resident reports whether an agent currently holds a goroutine.
func (rt *Runtime) Resident(agentID string) bool {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	_, ok := rt.resident[agentID]
	return ok
}

// Stop evicts all actors and waits for their goroutines to exit. Durable
// state is untouched; a new runtime over the same log recovers via Sweep.
func (rt *Runtime) Stop() {
	rt.mu.Lock()
	rt.closed = true
	for _, r := range rt.resident {
		r.evicted = true
		r.cancel()
	}
	rt.mu.Unlock()
	rt.wg.Wait()
}

// loop is one activation: wake → work (backlog + steering, possibly hours)
// → passivate. The passivation idle-check is atomic with Send's delivery
// (both under rt.mu) — the spec's central obligation.
func (rt *Runtime) loop(ctx context.Context, r *residentActor) {
	defer rt.wg.Done()
	memory, err := rt.log.LoadSnapshot(ctx, r.agentID)
	if err != nil {
		memory = nil
	}

	for {
		if ctx.Err() != nil { // evicted: crash-equivalent exit
			rt.deregister(r)
			return
		}
		backlog, err := rt.log.Unprocessed(ctx, r.agentID)
		if err != nil {
			if !sleepCtx(ctx, rt.opts.HandlerRetryBackoff) {
				rt.deregister(r)
				return
			}
			continue
		}
		if len(backlog) == 0 {
			// Attempt passivation. Atomicity argument: Send appends to the
			// log BEFORE taking rt.mu. If an append landed after our query,
			// either (a) Send acquires mu before us and appends to pending —
			// we observe it below and continue — or (b) we deregister first
			// and Send finds the actor cold and activates a fresh one. In
			// both cases the update is delivered: no lost wake.
			rt.mu.Lock()
			if len(r.pending) == 0 && !r.evicted {
				delete(rt.resident, r.agentID)
				rt.mu.Unlock()
				_ = rt.log.SaveSnapshot(context.Background(), r.agentID, memory)
				return
			}
			r.pending = r.pending[:0] // steers are already in the log; re-query
			evicted := r.evicted
			rt.mu.Unlock()
			if evicted {
				rt.deregister(r)
				return
			}
			continue
		}
		for _, u := range backlog {
			if ctx.Err() != nil {
				rt.deregister(r)
				return
			}
			next, err := rt.handler.HandleUpdate(ctx, r.agentID, u, memory)
			if err != nil {
				// Leave unprocessed (at-least-once); back off, then re-query.
				if !sleepCtx(ctx, rt.opts.HandlerRetryBackoff) {
					rt.deregister(r)
					return
				}
				break
			}
			memory = next
			if err := rt.log.MarkProcessed(ctx, r.agentID, u.UpdateID); err != nil {
				if !sleepCtx(ctx, rt.opts.HandlerRetryBackoff) {
					rt.deregister(r)
					return
				}
				break
			}
		}
		// Drain steering signals delivered while we worked; their updates are
		// in the log and the next Unprocessed query returns them.
		rt.mu.Lock()
		r.pending = r.pending[:0]
		rt.mu.Unlock()
	}
}

// deregister removes an evicted/cancelled actor without saving a snapshot.
func (rt *Runtime) deregister(r *residentActor) {
	rt.mu.Lock()
	if current, ok := rt.resident[r.agentID]; ok && current == r {
		delete(rt.resident, r.agentID)
	}
	rt.mu.Unlock()
}

func sleepCtx(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}
