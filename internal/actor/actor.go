// Package actor implements the durable-actor core of the runtime
// rearchitecture (docs/choir-rearchitecture-durable-actors-2026-06-11.md),
// conforming to specs/actor_protocol.tla.
//
// An agent is a long-lived actor: a goroutine with a Go-channel mailbox while
// resident, an idempotent durable update log plus a compacted memory snapshot
// while passivated. Sending to a cold actor activates it; sending to a warm
// actor steers it. Actors never "complete" — they passivate on quiescence and
// re-warm on the next send or sweep.
//
//	The database remembers. Go delivers.
//
// The durable log is the recovery substrate: it is replayed once on cold-start
// activation and queried by the boot/periodic Sweep. It is never polled as a
// delivery mechanism while the actor is warm — the Go channel is the delivery
// mechanism. If the channel buffer overflows, the update stays in the log and
// is caught by a single backlog query after the channel drains.
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
//
// Handlers must be idempotent: the same Update may be delivered more than
// once if it arrives through both the channel and the log backlog query
// (e.g. a warm steer that lands in the log during cold-start replay). The
// handler should check durable state (e.g. run state) before acting.
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
	// MailboxCapacity is the buffer size of the Go-channel mailbox
	// (default 256). If the buffer overflows, the update stays in the
	// durable log and is caught by a backlog query after the channel
	// drains.
	MailboxCapacity int
	// IdleTimeout is how long an actor waits with an empty mailbox before
	// passivating (default 5s). Shorter values passivate faster but cause
	// more cold-start replays; longer values keep actors resident but
	// consume memory.
	IdleTimeout time.Duration
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
	mailbox chan Update // Go-channel mailbox: the delivery mechanism while warm
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
	if opts.MailboxCapacity <= 0 {
		opts.MailboxCapacity = 256
	}
	if opts.IdleTimeout <= 0 {
		opts.IdleTimeout = 5 * time.Second
	}
	return &Runtime{
		log:      log,
		handler:  handler,
		opts:     opts,
		resident: map[string]*residentActor{},
	}
}

// Send durably appends the update, then delivers it: into the recipient's
// Go-channel mailbox if warm (steering), by activating it if cold. A resend
// of an already-logged UpdateID is a no-op. Ledger effects keyed to specific
// kinds belong in the same transaction as Append (Log implementations own
// this).
//
// The log append happens BEFORE the residency check so that the update
// survives even if the actor passivates between the append and the delivery.
// If the mailbox channel is full, the update stays in the log; the loop's
// post-drain backlog query catches it.
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
		// Warm: steer via Go channel. Non-blocking — if the buffer is full,
		// the update is in the log and the loop's backlog query catches it.
		select {
		case r.mailbox <- u:
		default:
		}
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
	r := &residentActor{
		agentID: agentID,
		mailbox: make(chan Update, rt.opts.MailboxCapacity),
		cancel:  cancel,
	}
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

// loop is one activation: cold-start replay → warm select → passivate.
//
// Cold start: replay the durable log backlog (the ONLY time the log is
// queried for delivery). Then enter the warm loop: select on the Go-channel
// mailbox with an idle timer. When the mailbox drains, do one backlog query
// to catch overflow (updates that didn't fit in the channel buffer) and
// handler-error retries. When the idle timer fires with an empty mailbox,
// passivate.
//
// To prevent double processing (a message can be in both the channel and the
// log because Send writes to both), the loop tracks UpdateIDs processed from
// the channel and passes them to processBacklog as a skip set. The set
// persists for the entire activation lifetime and is NOT cleared between
// iterations: clearing it opens a race window where a Send that lands in both
// the log and the channel between the clear and the next backlog query would
// be double-processed. The set is bounded by the number of unique UpdateIDs
// processed during one activation, and a fresh set is allocated on each
// re-activation (loop is one activation).
//
// The passivation idle-check is atomic with Send's delivery (both under
// rt.mu) — the spec's central obligation. If a Send appends to the log and
// then finds the actor cold, it activates a fresh one that replays the log.
func (rt *Runtime) loop(ctx context.Context, r *residentActor) {
	defer rt.wg.Done()
	memory, err := rt.log.LoadSnapshot(ctx, r.agentID)
	if err != nil {
		memory = nil
	}

	// Cold start: replay durable backlog. This is the only time the log is
	// queried as a delivery source. The skip set collects processed IDs so
	// we can drain the channel of duplicates after replay.
	skip := make(map[string]bool)
	rt.processBacklog(ctx, r, &memory, skip)

	// Drain channel of messages already processed during cold-start replay.
	// Any message in the channel was also appended to the log (Send writes
	// both), so processBacklog already handled it. Messages that arrived
	// AFTER the last log query are NOT in skip and get processed here.
	drainCold:
	for {
		select {
		case u := <-r.mailbox:
			if !skip[u.UpdateID] {
				rt.processOne(ctx, r, u, &memory, skip)
			}
		default:
			break drainCold
		}
	}

	// Warm loop: Go-channel delivery with idle passivation.
	idleTimer := time.NewTimer(rt.opts.IdleTimeout)
	defer idleTimer.Stop()

	for {
		select {
		case <-ctx.Done():
			rt.deregister(r)
			return

		case u := <-r.mailbox:
			rt.processOne(ctx, r, u, &memory, skip)
			// Drain any more that arrived while we worked.
			drainLoop:
			for {
				select {
				case u2 := <-r.mailbox:
					rt.processOne(ctx, r, u2, &memory, skip)
				default:
					break drainLoop
				}
			}
			// Overflow check: catch updates that didn't fit in the channel
			// buffer (Send's non-blocking send fell through to default).
			// Also catches updates that failed handler processing (still
			// unprocessed in the log). Skip IDs already processed from the
			// channel. This is NOT polling — it's a single query after the
			// channel drains.
			rt.processBacklog(ctx, r, &memory, skip)
			resetTimer(idleTimer, rt.opts.IdleTimeout)

		case <-idleTimer.C:
			// Attempt passivation. Atomicity argument: Send appends to the
			// log BEFORE taking rt.mu. If an append landed after our last
			// drain, either (a) Send acquires mu before us and sends to the
			// channel — we observe a non-empty mailbox and continue — or
			// (b) we deregister first and Send finds the actor cold and
			// activates a fresh one. In both cases the update is delivered:
			// no lost wake.
			rt.mu.Lock()
			if len(r.mailbox) == 0 && !r.evicted {
				delete(rt.resident, r.agentID)
				// Save snapshot under the lock so callers that observe
				// !Resident() are guaranteed to see the saved snapshot.
				_ = rt.log.SaveSnapshot(context.Background(), r.agentID, memory)
				rt.mu.Unlock()
				return
			}
			evicted := r.evicted
			rt.mu.Unlock()
			if evicted {
				rt.deregister(r)
				return
			}
			idleTimer.Reset(rt.opts.IdleTimeout)
		}
	}
}

// processBacklog queries the durable log for unprocessed updates and processes
// them all, looping until the backlog is empty. On handler error, the update
// stays unprocessed and is retried after backoff. The skip set prevents
// processing updates that were already handled from the channel.
func (rt *Runtime) processBacklog(ctx context.Context, r *residentActor, memory *[]byte, skip map[string]bool) {
	for {
		if ctx.Err() != nil {
			return
		}
		backlog, err := rt.log.Unprocessed(ctx, r.agentID)
		if err != nil {
			if !sleepCtx(ctx, rt.opts.HandlerRetryBackoff) {
				return
			}
			continue
		}
		if len(backlog) == 0 {
			return
		}
		for _, u := range backlog {
			if ctx.Err() != nil {
				return
			}
			if skip[u.UpdateID] {
				continue // already processed from channel
			}
			next, err := rt.handler.HandleUpdate(ctx, r.agentID, u, *memory)
			if err != nil {
				if !sleepCtx(ctx, rt.opts.HandlerRetryBackoff) {
					return
				}
				break // re-query; the failed update stays unprocessed
			}
			*memory = next
			if err := rt.log.MarkProcessed(ctx, r.agentID, u.UpdateID); err != nil {
				if !sleepCtx(ctx, rt.opts.HandlerRetryBackoff) {
					return
				}
				break
			}
			if skip != nil {
				skip[u.UpdateID] = true
			}
		}
	}
}

// processOne processes a single update from the channel. On handler error,
// the update stays unprocessed in the log and is caught by the next
// processBacklog call. The UpdateID is added to skip so processBacklog
// doesn't double-process it.
func (rt *Runtime) processOne(ctx context.Context, r *residentActor, u Update, memory *[]byte, skip map[string]bool) {
	if ctx.Err() != nil {
		return
	}
	if skip[u.UpdateID] {
		return // already processed (by processBacklog or a previous processOne)
	}
	next, err := rt.handler.HandleUpdate(ctx, r.agentID, u, *memory)
	if err != nil {
		// Leave unprocessed; the post-drain processBacklog will retry it.
		// Don't add to skip — we want processBacklog to find and retry it.
		_ = sleepCtx(ctx, rt.opts.HandlerRetryBackoff)
		return
	}
	*memory = next
	_ = rt.log.MarkProcessed(ctx, r.agentID, u.UpdateID)
	skip[u.UpdateID] = true
}

// deregister removes an evicted/cancelled actor without saving a snapshot.
func (rt *Runtime) deregister(r *residentActor) {
	rt.mu.Lock()
	if current, ok := rt.resident[r.agentID]; ok && current == r {
		delete(rt.resident, r.agentID)
	}
	rt.mu.Unlock()
}

func resetTimer(t *time.Timer, d time.Duration) {
	if !t.Stop() {
		select {
		case <-t.C:
		default:
		}
	}
	t.Reset(d)
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
