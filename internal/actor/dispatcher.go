package actor

import (
	"context"
	"fmt"
	"log"
	"runtime/debug"
	"sync"
	"time"
)

// Dispatcher is the sole consumer of the tape-derived pending projection
// (ontology Move 1 + 2). It replaces the Go-channel mailbox and the boot
// Sweep as the delivery authority: pending work is a projection (tape events
// addressed to actors minus incorporated events), and the dispatcher fires
// one serial activation per actor until each pending event is incorporated.
//
// Invariants:
//   - delivered = in state head. An event is delivered exactly when the
//     addressed actor's fenced Commit marks it incorporated.
//   - serial-per-actor: the dispatcher never runs two activations of the same
//     actor; events arriving during an activation are folded by the next.
//   - fenced commit: each activation commits {emitted events + new state
//     head} in one conditional append at the epoch it started from. A stale
//     or preempted activation's commit fails its epoch check.
//   - scheduled work: an unaddressed event carrying NotBefore is minted into
//     an addressed wake by the dispatcher's due-index when it comes due.
//   - retry = the lease: the dispatcher re-fires until incorporation; retry
//     accounting is itself tape events, so poison detection survives restart.
//
// The dispatcher is a thin process loop, not an actor on the tape — the
// residual-hard-point note in the ontology leaves multi-dispatcher failover
// unresolved, and a single dispatcher per computer is assumed.

// emissionBuffer accumulates the updates one activation emits. The
// dispatcher places it in the activation's context; the dispatch hook
// (actorruntime.Adapter.dispatch) routes emissions into it instead of
// sending immediately. Commit flushes the buffer atomically with the state
// head — so a stale or preempted activation's emissions die with it and are
// never durable (ontology fenced-commit clause).
type emissionBuffer struct {
	mu      sync.Mutex
	updates []Update
}

type emissionCtxKey struct{}

// withEmissions returns a context carrying a fresh emission buffer.
func withEmissions(ctx context.Context) (context.Context, *emissionBuffer) {
	buf := &emissionBuffer{}
	return context.WithValue(ctx, emissionCtxKey{}, buf), buf
}

// EmissionsFromCtx returns the activation's emission buffer, or nil when the
// caller is not inside a dispatcher activation (e.g. a boot or API-initiated
// send, which must emit immediately).
func EmissionsFromCtx(ctx context.Context) *emissionBuffer {
	buf, _ := ctx.Value(emissionCtxKey{}).(*emissionBuffer)
	return buf
}

// Add appends an emitted update to the activation's buffer.
func (b *emissionBuffer) Add(u Update) {
	b.mu.Lock()
	b.updates = append(b.updates, u)
	b.mu.Unlock()
}

// drain returns the buffered updates in emission order.
func (b *emissionBuffer) drain() []Update {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]Update, len(b.updates))
	copy(out, b.updates)
	return out
}
type Dispatcher struct {
	log     KernelLog
	handler Handler
	opts    DispatcherOptions

	mu      sync.Mutex
	running map[string]bool // agentID -> activation in flight (serial fence)
	wg      sync.WaitGroup // in-flight activations; Stop waits for them
	wake    chan struct{}  // pending-projection change signal
	stop    chan struct{}
	done    chan struct{}
	started bool // Run was launched; Stop waits on done only then
	stopped bool
}

// KernelLog is the durable surface the dispatcher needs: the pending
// projection, the fenced commit, and the due-index. *SQLiteLog implements it.
type KernelLog interface {
	// PendingAgents is the tape-derived pending projection: agents with a due
	// unprocessed event.
	PendingAgents(ctx context.Context, now time.Time) ([]string, error)
	// Unprocessed returns one agent's due backlog in append order.
	Unprocessed(ctx context.Context, agentID string) ([]Update, error)
	// Commit is the fenced atomic commit of emitted events + incorporated head
	// + folded memory snapshot (all in one transaction).
	Commit(ctx context.Context, agentID string, expectEpoch int64, emitted []Update, incorporated []string, memory []byte) (int64, error)
	// Epoch returns the actor's current durable epoch.
	Epoch(ctx context.Context, agentID string) (int64, error)
	// NextDue is the due-index: earliest future not_before among unprocessed
	// scheduled events.
	NextDue(ctx context.Context, now time.Time) (time.Time, bool, error)
	// RecordAttempt durably increments the dispatch-attempt counter for an
	// unprocessed update (retry accounting as tape state).
	RecordAttempt(ctx context.Context, agentID, updateID string) (int, error)
	// LoadSnapshot / SaveSnapshot persist the actor's compacted memory.
	LoadSnapshot(ctx context.Context, agentID string) ([]byte, error)
	SaveSnapshot(ctx context.Context, agentID string, memory []byte) error
}

// DispatcherOptions bound the dispatcher loop.
type DispatcherOptions struct {
	// PollInterval is how often the dispatcher re-reads the pending
	// projection when idle (default 250ms). The projection is the authority;
	// the poll is only a fallback for missed wake signals.
	PollInterval time.Duration
	// MaxAttempts is how many times the dispatcher retries an event before
	// emitting delivery_failed to the error sink (0 = retry forever).
	MaxAttempts int
	// ErrorSink is the agent ID that receives delivery_failed events. Empty
	// disables poison routing.
	ErrorSink string
	// MaxConcurrent bounds how many actor activations may run at once across
	// all agents (0 = default 8). Prevents unbounded goroutine fan-out.
	MaxConcurrent int
	// OnActorFailure is called when an activation's handler panics. The
	// callback receives the agent ID and the recovered error. It must not
	// block. When nil, panics are logged only.
	OnActorFailure FailureFunc
}

// NewDispatcher constructs a dispatcher over the kernel log. Call Run to
// start the loop; it consumes the pending projection until Stop.
func NewDispatcher(log KernelLog, handler Handler, opts DispatcherOptions) *Dispatcher {
	if opts.PollInterval <= 0 {
		opts.PollInterval = 250 * time.Millisecond
	}
	if opts.MaxConcurrent <= 0 {
		opts.MaxConcurrent = 8
	}
	return &Dispatcher{
		log:     log,
		handler: handler,
		opts:    opts,
		running: make(map[string]bool),
		wake:    make(chan struct{}, 1),
		stop:    make(chan struct{}),
		done:    make(chan struct{}),
	}
}

// Notify signals that the pending projection may have changed (an event was
// appended). It is a hint, not the authority — the dispatcher re-reads the
// projection regardless, so a missed Notify only delays delivery by one
// PollInterval.
func (d *Dispatcher) Notify() {
	select {
	case d.wake <- struct{}{}:
	default:
	}
}

// Run is the dispatcher loop: read the pending projection, fire a serial
// activation per due agent, self-wake at the next due-index time. It blocks
// until Stop.
func (d *Dispatcher) Run(ctx context.Context) {
	d.mu.Lock()
	d.started = true
	d.mu.Unlock()
	defer close(d.done)
	var dueTimer *time.Timer
	var dueC <-chan time.Time
	defer func() {
		if dueTimer != nil {
			dueTimer.Stop()
		}
	}()

	for {
		// Re-arm the due-index self-wake at the earliest scheduled event.
		if next, ok, err := d.log.NextDue(ctx, time.Now()); err == nil && ok {
			delay := time.Until(next)
			if delay < 0 {
				delay = 0
			}
			if dueTimer == nil {
				dueTimer = time.NewTimer(delay)
			} else {
				resetTimer(dueTimer, delay)
			}
			dueC = dueTimer.C
		} else {
			dueC = nil
		}

		// Consume the pending projection: one serial activation per due agent.
		d.dispatchPending(ctx)

		select {
		case <-ctx.Done():
			return
		case <-d.stop:
			return
		case <-d.wake:
		case <-dueC:
		case <-time.After(d.opts.PollInterval):
		}
	}
}

// dispatchPending reads the projection and fires an activation for every due
// agent not already running (the serial-per-actor fence).
func (d *Dispatcher) dispatchPending(ctx context.Context) {
	agents, err := d.log.PendingAgents(ctx, time.Now())
	if err != nil {
		log.Printf("dispatcher: pending projection: %v", err)
		return
	}
	for _, agentID := range agents {
		d.mu.Lock()
		if d.running[agentID] || len(d.running) >= d.opts.MaxConcurrent {
			d.mu.Unlock()
			continue
		}
		d.running[agentID] = true
		d.mu.Unlock()
		d.wg.Add(1)
		go func(id string) {
			defer d.wg.Done()
			defer func() {
				if rv := recover(); rv != nil {
					err := fmt.Errorf("actor %s panic: %v", id, rv)
					log.Printf("dispatcher: %v\n%s", err, debug.Stack())
					if d.opts.OnActorFailure != nil {
						d.opts.OnActorFailure(id, err)
					}
				}
				d.mu.Lock()
				delete(d.running, id)
				d.mu.Unlock()
				// A completed activation may have left new pending events
				// (its own emissions, or arrivals during the run). Signal the
				// loop to re-read the projection.
				d.Notify()
			}()
			d.activate(ctx, id)
		}(agentID)
	}
}

// activate runs one fenced activation: snapshot the actor's epoch, fold each
// due event through the handler, then Commit {emitted + incorporated} at the
// starting epoch. On epoch conflict the activation is stale — it discards
// its work and the dispatcher re-fires from the new head.
func (d *Dispatcher) activate(ctx context.Context, agentID string) {
	epoch, err := d.log.Epoch(ctx, agentID)
	if err != nil {
		log.Printf("dispatcher: epoch %s: %v", agentID, err)
		return
	}
	memory, err := d.log.LoadSnapshot(ctx, agentID)
	if err != nil {
		log.Printf("dispatcher: snapshot %s: %v", agentID, err)
		return
	}
	pending, err := d.log.Unprocessed(ctx, agentID)
	if err != nil {
		log.Printf("dispatcher: backlog %s: %v", agentID, err)
		return
	}
	if len(pending) == 0 {
		return
	}
	// The activation's context carries the emission buffer: the dispatch hook
	// routes this activation's emitted updates into it, and Commit flushes
	// them atomically with the state head.
	actx, buf := withEmissions(ctx)

	var incorporated []string
	var emitted []Update
	for _, u := range pending {
		// Skip scheduled events not yet due — the due-index fires them later.
		if !u.NotBefore.IsZero() && time.Now().Before(u.NotBefore) {
			continue
		}
		newMemory, herr := d.handler.HandleUpdate(actx, agentID, u, memory)
		// Drain this event's emissions now so a failed handler's partial
		// emissions are discarded, not committed with the batch.
		evEmitted := buf.drain()
		if herr != nil {
			// At-least-once: leave unprocessed; the dispatcher retries.
			// Retry accounting is durable tape state so poison detection
			// survives a restart.
			attempts, aerr := d.log.RecordAttempt(ctx, agentID, u.UpdateID)
			if aerr != nil {
				log.Printf("dispatcher: record attempt %s/%s: %v", agentID, u.UpdateID, aerr)
			}
			if d.opts.MaxAttempts > 0 && attempts >= d.opts.MaxAttempts && d.opts.ErrorSink != "" && u.Kind != "delivery_failed" {
				// Poison event: emit delivery_failed to the error sink and
				// incorporate the poisoned event so it stops re-firing. Do NOT
				// fold the failed handler's memory — it is unreliable.
				emitted = append(emitted, Update{
					UpdateID:    u.UpdateID + ":delivery_failed",
					ToAgentID:   d.opts.ErrorSink,
					FromAgentID: agentID,
					Kind:        "delivery_failed",
					Content:     u.UpdateID,
					CreatedAt:   time.Now().UTC(),
				})
				incorporated = append(incorporated, u.UpdateID)
				log.Printf("dispatcher: poison %s/%s after %d attempts -> %s", agentID, u.UpdateID, attempts, d.opts.ErrorSink)
				continue
			}
			log.Printf("dispatcher: handle %s/%s: %v", agentID, u.UpdateID, herr)
			// Tape order among eligible events: stop at the first failure so
			// a later event is not incorporated past an unprocessed earlier
			// one. The failed event's emissions were already discarded.
			break
		}
		emitted = append(emitted, evEmitted...)
		memory = newMemory
		incorporated = append(incorporated, u.UpdateID)
	}
	if len(incorporated) == 0 && len(emitted) == 0 {
		return
	}

	if _, err := d.log.Commit(ctx, agentID, epoch, emitted, incorporated, memory); err != nil {
		if err == ErrEpochConflict {
			// Stale activation: discard; the dispatcher re-fires from the new
			// head on the next projection read.
			return
		}
		log.Printf("dispatcher: commit %s: %v", agentID, err)
		return
	}
}

// Stop halts the dispatcher loop and waits for it to exit, then waits for
// every in-flight activation to finish or fail its fenced commit. No
// activation is orphaned: a restart cannot overlap a still-running handler.
func (d *Dispatcher) Stop() {
	d.mu.Lock()
	if !d.stopped {
		d.stopped = true
		close(d.stop)
	}
	started := d.started
	d.mu.Unlock()
	// done closes only when Run returns; if Run was never launched (an adapter
	// constructed but never StartKernel'd), waiting on it would deadlock.
	if started {
		<-d.done
	}
	d.wg.Wait()
}
