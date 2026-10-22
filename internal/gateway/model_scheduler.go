package gateway

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/provider"
)

// RES-11: a fair model-call scheduler
// (docs/problems/gateway-model-calls-have-no-scheduler-2026-10-21.md; design
// in docs/definitions/choir-launch-fp7-gateway-identity-2026-10-21.md).
//
// With GATEWAY_MODEL_SCHEDULER=on, inference and OpenAI-compatible chat calls
// take a slot here instead of passing the fixed one-minute window. There is
// no per-minute count: fairness, not a spending cap (owner: track spending,
// don't cap). The bounds are concurrency (a global cap on calls in flight and
// a per-computer cap), queue depth and wait, which keep a runaway loop from
// reaching a provider at full speed while a legitimate burst queues and the
// guest, which already waits out a Retry-After, comes back.
//
// Order: each computer has a FIFO queue. When a slot frees, the next call
// comes from the eligible computer with the fewest calls in flight, ties to
// the one served longest ago (a computer that has never been served, or has
// been idle, is the longest ago). A computer with one call is therefore never
// queued behind another's backlog, and two busy computers share the slots
// evenly. A slot is held for the whole call, a stream included.

// ErrModelQueueFull: the computer's queue is at its depth.
var ErrModelQueueFull = errors.New("model call queue is full")

// ErrModelWaitExceeded: the call waited the whole bound without a slot.
var ErrModelWaitExceeded = errors.New("model call waited too long for a slot")

// Environment switches (gateway unit).
const (
	ModelSchedulerEnv            = "GATEWAY_MODEL_SCHEDULER"
	ModelSchedulerInflightEnv    = "GATEWAY_MODEL_INFLIGHT"
	ModelSchedulerPerComputerEnv = "GATEWAY_MODEL_INFLIGHT_PER_COMPUTER"
	ModelSchedulerQueueDepthEnv  = "GATEWAY_MODEL_QUEUE_DEPTH"
	ModelSchedulerQueueWaitEnv   = "GATEWAY_MODEL_QUEUE_WAIT"

	defaultModelInflight    = 24
	defaultModelPerComputer = 6
	defaultModelQueueDepth  = 256
	defaultModelQueueWait   = 120 * time.Second
	// maxModelQueueWait caps the wait: the wait counts against the call's
	// inferenceTimeout (scheduleModelCall), so a longer one would leave a
	// call no time to run and end in a hang-up instead of a 429.
	maxModelQueueWait   = 5 * time.Minute
	modelQueueFullRetry = 10 // seconds, the Retry-After of a full queue
	modelQueueWaitRetry = 5  // seconds, the Retry-After of an exceeded wait
)

// ModelSchedulerConfig bounds the scheduler. Zero or negative values are
// replaced by the defaults; PerComputer never exceeds Global.
type ModelSchedulerConfig struct {
	Global      int
	PerComputer int
	QueueDepth  int
	MaxWait     time.Duration
}

func (c ModelSchedulerConfig) resolve() ModelSchedulerConfig {
	if c.Global <= 0 {
		c.Global = defaultModelInflight
	}
	if c.PerComputer <= 0 {
		c.PerComputer = defaultModelPerComputer
	}
	if c.PerComputer > c.Global {
		c.PerComputer = c.Global
	}
	if c.QueueDepth <= 0 {
		c.QueueDepth = defaultModelQueueDepth
	}
	if c.MaxWait <= 0 {
		c.MaxWait = defaultModelQueueWait
	}
	if c.MaxWait > maxModelQueueWait {
		c.MaxWait = maxModelQueueWait
	}
	return c
}

// ModelSchedulerEnabled reads GATEWAY_MODEL_SCHEDULER. Only on, true or 1
// turn it on; anything else (including unset and unreadable values) leaves
// the old limiter in place.
func ModelSchedulerEnabled(get func(string) string) bool {
	switch strings.ToLower(strings.TrimSpace(get(ModelSchedulerEnv))) {
	case "on", "true", "1":
		return true
	}
	return false
}

// ModelSchedulerConfigFromGetenv reads the bounds; an unreadable value is the
// default.
func ModelSchedulerConfigFromGetenv(get func(string) string) ModelSchedulerConfig {
	var c ModelSchedulerConfig
	if n, err := strconv.Atoi(strings.TrimSpace(get(ModelSchedulerInflightEnv))); err == nil {
		c.Global = n
	}
	if n, err := strconv.Atoi(strings.TrimSpace(get(ModelSchedulerPerComputerEnv))); err == nil {
		c.PerComputer = n
	}
	if n, err := strconv.Atoi(strings.TrimSpace(get(ModelSchedulerQueueDepthEnv))); err == nil {
		c.QueueDepth = n
	}
	if d, err := time.ParseDuration(strings.TrimSpace(get(ModelSchedulerQueueWaitEnv))); err == nil {
		c.MaxWait = d
	}
	return c.resolve()
}

// ModelSchedulerStats is the scheduler's state and counters since start.
type ModelSchedulerStats struct {
	InFlight     int
	Queued       int
	Granted      int64
	QueueFull    int64
	WaitExceeded int64
	Cancelled    int64
	// WaitTotal is the summed time granted calls spent queued.
	WaitTotal time.Duration
}

type modelWaiter struct {
	computer string
	enqueued time.Time
	ready    chan struct{} // closed when a slot is granted
	granted  bool
	gone     bool // removed from the queue by its caller
}

type modelComputer struct {
	inflight   int
	queue      []*modelWaiter
	lastServed uint64
}

// ModelScheduler is the fair model-call scheduler.
type ModelScheduler struct {
	cfg ModelSchedulerConfig

	mu        sync.Mutex
	computers map[string]*modelComputer
	inflight  int
	queued    int
	clock     uint64
	stats     ModelSchedulerStats
}

// NewModelScheduler builds a scheduler with the resolved bounds.
func NewModelScheduler(cfg ModelSchedulerConfig) *ModelScheduler {
	return &ModelScheduler{cfg: cfg.resolve(), computers: map[string]*modelComputer{}}
}

// Stats returns a snapshot.
func (s *ModelScheduler) Stats() ModelSchedulerStats {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.stats
	st.InFlight = s.inflight
	st.Queued = s.queued
	return st
}

// Acquire waits for a slot for one call of the computer. It returns the
// function that gives the slot back (safe to call more than once; the caller
// must call it on every path, normally by defer) and how long the call
// waited. It returns ErrModelQueueFull at once when the computer's queue is at
// its depth, ErrModelWaitExceeded after the wait bound, and the context's
// error when the caller goes away; in each case no slot is held.
func (s *ModelScheduler) Acquire(ctx context.Context, computerID string) (func(), time.Duration, error) {
	started := time.Now()
	// A caller already gone is never queued or handed a slot (B7).
	if err := ctx.Err(); err != nil {
		s.mu.Lock()
		s.stats.Cancelled++
		s.mu.Unlock()
		return nil, 0, err
	}
	s.mu.Lock()
	c := s.computers[computerID]
	if c == nil {
		c = &modelComputer{}
		s.computers[computerID] = c
	}
	if len(c.queue) >= s.cfg.QueueDepth {
		s.stats.QueueFull++
		s.dropIfIdleLocked(computerID, c)
		s.mu.Unlock()
		return nil, 0, ErrModelQueueFull
	}
	w := &modelWaiter{computer: computerID, enqueued: started, ready: make(chan struct{})}
	c.queue = append(c.queue, w)
	s.queued++
	s.dispatchLocked()
	s.mu.Unlock()

	timer := time.NewTimer(s.cfg.MaxWait)
	defer timer.Stop()
	var failure error
	select {
	case <-w.ready:
	case <-ctx.Done():
		failure = ctx.Err()
	case <-timer.C:
		failure = ErrModelWaitExceeded
	}
	// The slot and the cancellation can arrive together, and select picks
	// either: a cancelled caller gives the slot back.
	if failure == nil && ctx.Err() != nil {
		failure = ctx.Err()
	}
	if failure != nil {
		s.mu.Lock()
		if w.granted {
			// The slot arrived as the caller gave up. A timeout keeps it (the
			// call can still run); a cancelled caller gives it straight back.
			if failure == ErrModelWaitExceeded {
				s.mu.Unlock()
				return s.releaser(computerID), time.Since(started), nil
			}
			s.releaseLocked(computerID)
			s.stats.Cancelled++
			s.mu.Unlock()
			return nil, 0, failure
		}
		s.removeWaiterLocked(w)
		if failure == ErrModelWaitExceeded {
			s.stats.WaitExceeded++
		} else {
			s.stats.Cancelled++
		}
		s.mu.Unlock()
		return nil, 0, failure
	}
	waited := time.Since(started)
	s.mu.Lock()
	s.stats.WaitTotal += waited
	s.mu.Unlock()
	return s.releaser(computerID), waited, nil
}

func (s *ModelScheduler) releaser(computerID string) func() {
	var once sync.Once
	return func() {
		once.Do(func() {
			s.mu.Lock()
			s.releaseLocked(computerID)
			s.mu.Unlock()
		})
	}
}

// releaseLocked gives one slot back and hands freed capacity to the queues.
func (s *ModelScheduler) releaseLocked(computerID string) {
	c := s.computers[computerID]
	if c == nil || c.inflight == 0 {
		return
	}
	c.inflight--
	s.inflight--
	s.dropIfIdleLocked(computerID, c)
	s.dispatchLocked()
}

// removeWaiterLocked takes a waiter that was never granted out of its queue.
func (s *ModelScheduler) removeWaiterLocked(w *modelWaiter) {
	c := s.computers[w.computer]
	if c == nil {
		return
	}
	for i, q := range c.queue {
		if q == w {
			c.queue = append(c.queue[:i], c.queue[i+1:]...)
			s.queued--
			w.gone = true
			break
		}
	}
	s.dropIfIdleLocked(w.computer, c)
}

// dropIfIdleLocked forgets a computer with nothing in flight and nothing
// queued, so the table does not grow and an idle computer rejoins with the
// priority of one not served recently.
func (s *ModelScheduler) dropIfIdleLocked(id string, c *modelComputer) {
	if c.inflight == 0 && len(c.queue) == 0 {
		delete(s.computers, id)
	}
}

// dispatchLocked grants slots while there is global capacity and an eligible
// queued call: the computer with the fewest in flight, then the longest since
// served, then the smallest id (so the choice is deterministic).
func (s *ModelScheduler) dispatchLocked() {
	for s.inflight < s.cfg.Global {
		var pickID string
		var pick *modelComputer
		for id, c := range s.computers {
			if len(c.queue) == 0 || c.inflight >= s.cfg.PerComputer {
				continue
			}
			if pick == nil || c.inflight < pick.inflight ||
				(c.inflight == pick.inflight && (c.lastServed < pick.lastServed ||
					(c.lastServed == pick.lastServed && id < pickID))) {
				pickID, pick = id, c
			}
		}
		if pick == nil {
			return
		}
		w := pick.queue[0]
		pick.queue = pick.queue[1:]
		s.queued--
		pick.inflight++
		s.inflight++
		s.clock++
		pick.lastServed = s.clock
		w.granted = true
		s.stats.Granted++
		close(w.ready)
	}
}

// --- handler integration ---

// SetModelScheduler attaches the scheduler. A handler without one keeps the
// fixed-window limiter for model calls.
func (h *Handler) SetModelScheduler(s *ModelScheduler) {
	h.modelScheduler = s
}

// schedulerOn reports whether model calls are scheduled rather than limited
// by the fixed window.
func (h *Handler) schedulerOn() bool { return h.modelScheduler != nil }

// scheduleModelCall takes a slot for one model call. It returns the release
// func (never nil when ok) and ok=false after answering the caller itself:
// 429 with Retry-After when the queue is full or the wait ran out; nothing
// when the caller has gone. With no scheduler it is a no-op that always
// succeeds, so the old limiter remains the only gate.
//
// The returned request carries a deadline of inferenceTimeout from the moment
// the call asked for a slot, so the queue wait counts against the call's
// budget: wait plus call stay inside the gateway's 10m30s write timeout and
// the guest's client timeout (RES-11 record, finding 11). The handlers'
// own inferenceTimeout contexts derive from it, so the earlier deadline wins.
// In both modes it also names the computer as the call's image decode
// caller, so one computer holds at most its share of the decode slots
// (bunch 116, F2).
func (h *Handler) scheduleModelCall(w http.ResponseWriter, r *http.Request, computerID string, openAI bool) (func(), *http.Request, bool) {
	r = r.WithContext(provider.WithImageDecodeCaller(r.Context(), computerID))
	if h.modelScheduler == nil {
		return func() {}, r, true
	}
	ctx, cancel := context.WithTimeout(r.Context(), inferenceTimeout)
	release, _, err := h.modelScheduler.Acquire(ctx, computerID)
	if err == nil {
		return func() { release(); cancel() }, r.WithContext(ctx), true
	}
	cancel()
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return nil, nil, false // the caller went away; nobody reads a response
	}
	retry := modelQueueWaitRetry
	msg := "model calls for this autoputer waited too long for a slot; retry"
	if errors.Is(err, ErrModelQueueFull) {
		retry = modelQueueFullRetry
		msg = "model call queue is full for this autoputer; retry"
	}
	w.Header().Set("Retry-After", strconv.Itoa(retry))
	if openAI {
		writeGatewayJSON(w, http.StatusTooManyRequests, openAIErrorResponse(msg))
	} else {
		writeGatewayJSON(w, http.StatusTooManyRequests, ErrorResponse{Error: msg})
	}
	return nil, nil, false
}

// gatewaySchedulerHealth is the scheduler as /health reports it.
type gatewaySchedulerHealth struct {
	Mode         string `json:"mode"`
	InFlight     int    `json:"in_flight"`
	Queued       int    `json:"queued"`
	Granted      int64  `json:"granted"`
	QueueFull    int64  `json:"queue_full"`
	WaitExceeded int64  `json:"wait_exceeded"`
	Cancelled    int64  `json:"cancelled"`
	AvgWaitMs    int64  `json:"avg_wait_ms"`
	MaxInFlight  int    `json:"max_in_flight"`
	PerComputer  int    `json:"max_per_computer"`
}

func (h *Handler) schedulerHealth() *gatewaySchedulerHealth {
	if h.modelScheduler == nil {
		return &gatewaySchedulerHealth{Mode: "off"}
	}
	st := h.modelScheduler.Stats()
	out := &gatewaySchedulerHealth{
		Mode: "on", InFlight: st.InFlight, Queued: st.Queued, Granted: st.Granted,
		QueueFull: st.QueueFull, WaitExceeded: st.WaitExceeded, Cancelled: st.Cancelled,
		MaxInFlight: h.modelScheduler.cfg.Global, PerComputer: h.modelScheduler.cfg.PerComputer,
	}
	if st.Granted > 0 {
		out.AvgWaitMs = st.WaitTotal.Milliseconds() / st.Granted
	}
	return out
}
