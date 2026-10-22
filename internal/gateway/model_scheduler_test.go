package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/provider"
)

// RES-11: model calls take a slot from one fair scheduler instead of being
// rejected by a fixed one-minute window
// (docs/problems/gateway-model-calls-have-no-scheduler-2026-10-21.md).
// Ways the repair can fail, written before the code (numbering follows the
// problem record):
//  1. a slot leaks (cancel, timeout, panic, a provider error, a stream's end,
//     a double release) and the pool drains
//     (TestModelSchedulerLeaksNoSlotUnderCancellation, ...CancelWhileQueued...,
//     ...ReleaseIsIdempotent, ...APanickingProviderReleasesItsSlot,
//     ...StreamingHoldsTheSlotUntilTheStreamEnds);
//  2. a heavy computer starves a light one, or a light one waits behind a
//     heavy one's backlog (TestModelSchedulerServesTheLightComputerFirst,
//     ...AlternatesBetweenComputers);
//  3. one computer takes every slot (...PerComputerCapHolds);
//  4. the queue is unbounded, or a call waits forever
//     (...QueueDepthIsBounded, ...WaitIsBounded);
//  5. a queued call outlives its caller (...CancelWhileQueued...);
//  6. the scheduler replaces the old backstop and a runaway caller reaches the
//     provider at full speed (...GlobalCapHolds);
//  7. no way back to the old behavior: off is the default
//     (TestModelSchedulerIsOffByDefaultAndKeepsTheOldLimiter);
//  8. search and Jev calls are queued behind model calls
//     (TestModelSchedulerLeavesSearchOnTheOldLimiter);
//  9. the wait is invisible (TestModelSchedulerHealthReportsTheQueue).

func newTestScheduler(global, perComputer, depth int, wait time.Duration) *ModelScheduler {
	return NewModelScheduler(ModelSchedulerConfig{Global: global, PerComputer: perComputer, QueueDepth: depth, MaxWait: wait})
}

func waitUntil(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(time.Millisecond)
	}
}

func mustAcquire(t *testing.T, s *ModelScheduler, computer string) func() {
	t.Helper()
	release, _, err := s.Acquire(context.Background(), computer)
	if err != nil {
		t.Fatalf("acquire for %s: %v", computer, err)
	}
	return release
}

// acquireAsync starts an acquire that may block and reports its result.
type pending struct {
	computer string
	done     chan struct{}
	release  func()
	err      error
}

func acquireAsync(ctx context.Context, s *ModelScheduler, computer string) *pending {
	p := &pending{computer: computer, done: make(chan struct{})}
	go func() {
		p.release, _, p.err = s.Acquire(ctx, computer)
		close(p.done)
	}()
	return p
}

func (p *pending) granted() bool {
	select {
	case <-p.done:
		return p.err == nil
	default:
		return false
	}
}

func TestModelSchedulerGlobalCapHolds(t *testing.T) {
	s := newTestScheduler(2, 2, 16, time.Minute)
	r1 := mustAcquire(t, s, "a")
	r2 := mustAcquire(t, s, "b")
	third := acquireAsync(context.Background(), s, "c")
	waitUntil(t, "the third call to queue", func() bool { return s.Stats().Queued == 1 })
	if third.granted() {
		t.Fatal("a third call ran past a global cap of 2")
	}
	r1()
	waitUntil(t, "the queued call to run after a release", third.granted)
	r2()
	third.release()
	if st := s.Stats(); st.InFlight != 0 || st.Queued != 0 {
		t.Fatalf("stats after drain = %+v", st)
	}
}

func TestModelSchedulerPerComputerCapHolds(t *testing.T) {
	s := newTestScheduler(10, 2, 16, time.Minute)
	a1 := mustAcquire(t, s, "a")
	a2 := mustAcquire(t, s, "a")
	a3 := acquireAsync(context.Background(), s, "a")
	waitUntil(t, "a's third call to queue", func() bool { return s.Stats().Queued == 1 })
	if a3.granted() {
		t.Fatal("one computer took more than its cap while the pool had room")
	}
	b := mustAcquire(t, s, "b") // another computer is not held back by a's backlog
	a1()
	waitUntil(t, "a's queued call after a's release", a3.granted)
	a2()
	a3.release()
	b()
}

func TestModelSchedulerServesTheLightComputerFirst(t *testing.T) {
	s := newTestScheduler(2, 2, 64, time.Minute)
	a1 := mustAcquire(t, s, "heavy")
	a2 := mustAcquire(t, s, "heavy")
	var heavy []*pending
	for i := 0; i < 5; i++ {
		heavy = append(heavy, acquireAsync(context.Background(), s, "heavy"))
	}
	waitUntil(t, "the heavy backlog to queue", func() bool { return s.Stats().Queued == 5 })
	light := acquireAsync(context.Background(), s, "light")
	waitUntil(t, "the light call to queue", func() bool { return s.Stats().Queued == 6 })
	a1() // one slot frees: the light computer, with nothing in flight, goes first
	waitUntil(t, "the light call to run", light.granted)
	for i, h := range heavy {
		if h.granted() {
			t.Fatalf("heavy backlog entry %d ran ahead of the light computer's call", i)
		}
	}
	a2()
	light.release()
}

func TestModelSchedulerAlternatesBetweenComputers(t *testing.T) {
	s := newTestScheduler(1, 1, 64, time.Minute)
	holder := mustAcquire(t, s, "heavy")
	type grant struct {
		computer string
		release  func()
	}
	grants := make(chan grant, 64)
	enqueue := func(computer string, n int) {
		for i := 0; i < n; i++ {
			go func() {
				release, _, err := s.Acquire(context.Background(), computer)
				if err == nil {
					grants <- grant{computer, release}
				}
			}()
		}
	}
	enqueue("heavy", 20)
	enqueue("light", 5)
	waitUntil(t, "25 calls to queue", func() bool { return s.Stats().Queued == 25 })
	holder()
	var order []string
	for len(order) < 25 {
		g := <-grants
		order = append(order, g.computer)
		g.release()
	}
	lightDone := 0
	for i, c := range order {
		if c == "light" {
			lightDone = i + 1
		}
	}
	if lightDone > 10 {
		t.Fatalf("the light computer's 5 calls finished at position %d of 25 behind a 20-call backlog; order: %v", lightDone, order)
	}
}

func TestModelSchedulerIsFIFOWithinAComputer(t *testing.T) {
	s := newTestScheduler(1, 1, 16, time.Minute)
	holder := mustAcquire(t, s, "a")
	var got []int
	var mu sync.Mutex
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			release, _, err := s.Acquire(context.Background(), "a")
			if err != nil {
				return
			}
			mu.Lock()
			got = append(got, i)
			mu.Unlock()
			release()
		}()
		waitUntil(t, "call to queue in order", func() bool { return s.Stats().Queued == i+1 })
	}
	holder()
	wg.Wait()
	for i, v := range got {
		if v != i {
			t.Fatalf("a computer's own calls ran out of order: %v", got)
		}
	}
}

func TestModelSchedulerQueueDepthIsBounded(t *testing.T) {
	s := newTestScheduler(1, 1, 2, time.Minute)
	holder := mustAcquire(t, s, "a")
	q1 := acquireAsync(context.Background(), s, "a")
	q2 := acquireAsync(context.Background(), s, "a")
	waitUntil(t, "two queued", func() bool { return s.Stats().Queued == 2 })
	start := time.Now()
	_, _, err := s.Acquire(context.Background(), "a")
	if !errors.Is(err, ErrModelQueueFull) {
		t.Fatalf("third queued call: %v, want ErrModelQueueFull", err)
	}
	if time.Since(start) > time.Second {
		t.Fatal("a full queue made the caller wait")
	}
	// Another computer's queue is its own.
	b := acquireAsync(context.Background(), s, "b")
	waitUntil(t, "b to queue", func() bool { return s.Stats().Queued == 3 })
	holder()
	// With one global slot the three run one at a time, in the scheduler's
	// order; release whichever is running until all have run.
	remaining := []*pending{q1, q2, b}
	for len(remaining) > 0 {
		waitUntil(t, "one queued call to run", func() bool {
			for _, p := range remaining {
				if p.granted() {
					return true
				}
			}
			return false
		})
		for i, p := range remaining {
			if p.granted() {
				p.release()
				remaining = append(remaining[:i], remaining[i+1:]...)
				break
			}
		}
	}
	if st := s.Stats(); st.QueueFull != 1 {
		t.Fatalf("queue_full counter = %d, want 1", st.QueueFull)
	}
}

func TestModelSchedulerWaitIsBounded(t *testing.T) {
	s := newTestScheduler(1, 1, 8, 60*time.Millisecond)
	holder := mustAcquire(t, s, "a")
	defer holder()
	start := time.Now()
	_, _, err := s.Acquire(context.Background(), "b")
	if !errors.Is(err, ErrModelWaitExceeded) {
		t.Fatalf("blocked call: %v, want ErrModelWaitExceeded", err)
	}
	if d := time.Since(start); d < 50*time.Millisecond || d > 2*time.Second {
		t.Fatalf("waited %s for a 60ms bound", d)
	}
	if st := s.Stats(); st.Queued != 0 || st.WaitExceeded != 1 {
		t.Fatalf("stats after a timed-out wait = %+v", st)
	}
}

func TestModelSchedulerCancelWhileQueuedFreesTheEntry(t *testing.T) {
	s := newTestScheduler(1, 1, 8, time.Minute)
	holder := mustAcquire(t, s, "a")
	ctx, cancel := context.WithCancel(context.Background())
	p := acquireAsync(ctx, s, "b")
	waitUntil(t, "b to queue", func() bool { return s.Stats().Queued == 1 })
	cancel()
	select {
	case <-p.done:
	case <-time.After(5 * time.Second):
		t.Fatal("a cancelled queued call did not return")
	}
	if !errors.Is(p.err, context.Canceled) {
		t.Fatalf("cancelled call: %v", p.err)
	}
	if st := s.Stats(); st.Queued != 0 {
		t.Fatalf("a cancelled call stayed queued: %+v", st)
	}
	next := acquireAsync(context.Background(), s, "c")
	waitUntil(t, "c to queue", func() bool { return s.Stats().Queued == 1 })
	holder() // the slot must go to c, not to the cancelled entry
	waitUntil(t, "c to run", next.granted)
	next.release()
	if st := s.Stats(); st.InFlight != 0 {
		t.Fatalf("a slot leaked: %+v", st)
	}
}

func TestModelSchedulerLeaksNoSlotUnderCancellation(t *testing.T) {
	s := newTestScheduler(3, 2, 512, 40*time.Millisecond)
	var wg sync.WaitGroup
	var served atomic.Int64
	for i := 0; i < 400; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			computer := string(rune('a' + i%5))
			ctx, cancel := context.WithTimeout(context.Background(), time.Duration(rand.Intn(30))*time.Millisecond)
			defer cancel()
			release, _, err := s.Acquire(ctx, computer)
			if err != nil {
				return
			}
			served.Add(1)
			time.Sleep(time.Duration(rand.Intn(3)) * time.Millisecond)
			release()
		}()
	}
	wg.Wait()
	waitUntil(t, "the scheduler to drain", func() bool { st := s.Stats(); return st.InFlight == 0 && st.Queued == 0 })
	if served.Load() == 0 {
		t.Fatal("nothing was served")
	}
	// And it still works.
	mustAcquire(t, s, "z")()
}

func TestModelSchedulerReleaseIsIdempotent(t *testing.T) {
	// Two of a's calls hold the two global slots. Releasing the first one
	// repeatedly must free exactly one slot, not the second call's as well.
	s := newTestScheduler(2, 2, 8, time.Minute)
	first := mustAcquire(t, s, "a")
	second := mustAcquire(t, s, "a")
	first()
	first()
	first()
	b := mustAcquire(t, s, "b") // takes the one slot first() freed
	waiting := acquireAsync(context.Background(), s, "c")
	waitUntil(t, "c to queue", func() bool { return s.Stats().Queued == 1 })
	if waiting.granted() {
		t.Fatal("repeated releases freed a slot that was still held: the global cap of 2 was exceeded")
	}
	if st := s.Stats(); st.InFlight != 2 {
		t.Fatalf("in flight = %d, want 2 (second and b)", st.InFlight)
	}
	second()
	waitUntil(t, "c to run once a real slot frees", waiting.granted)
	waiting.release()
	b()
}

func TestModelSchedulerHealthReportsTheQueue(t *testing.T) {
	h, _, _ := setupHandler(t)
	s := newTestScheduler(1, 1, 8, time.Minute)
	h.SetModelScheduler(s)
	holder := mustAcquire(t, s, "a")
	p := acquireAsync(context.Background(), s, "b")
	waitUntil(t, "b to queue", func() bool { return s.Stats().Queued == 1 })

	rec := httptest.NewRecorder()
	h.HandleHealth(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
	var body struct {
		ModelScheduler struct {
			Mode     string `json:"mode"`
			InFlight int    `json:"in_flight"`
			Queued   int    `json:"queued"`
			Granted  int64  `json:"granted"`
		} `json:"model_scheduler"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	ms := body.ModelScheduler
	if ms.Mode != "on" || ms.InFlight != 1 || ms.Queued != 1 {
		t.Fatalf("model_scheduler = %+v; body %s", ms, rec.Body.String())
	}
	holder()
	waitUntil(t, "b to run", p.granted)
	p.release()
}

func TestModelSchedulerConfigDefaultsAndParsing(t *testing.T) {
	get := func(m map[string]string) func(string) string { return func(k string) string { return m[k] } }
	if ModelSchedulerEnabled(get(nil)) {
		t.Fatal("the scheduler is on by default")
	}
	for _, v := range []string{"", "off", "no", "garbage", "0"} {
		if ModelSchedulerEnabled(get(map[string]string{"GATEWAY_MODEL_SCHEDULER": v})) {
			t.Fatalf("GATEWAY_MODEL_SCHEDULER=%q turned the scheduler on", v)
		}
	}
	for _, v := range []string{"on", "ON", " true ", "1"} {
		if !ModelSchedulerEnabled(get(map[string]string{"GATEWAY_MODEL_SCHEDULER": v})) {
			t.Fatalf("GATEWAY_MODEL_SCHEDULER=%q did not turn the scheduler on", v)
		}
	}
	d := ModelSchedulerConfigFromGetenv(get(nil))
	if d.Global != 24 || d.PerComputer != 6 || d.QueueDepth != 256 || d.MaxWait != 120*time.Second {
		t.Fatalf("defaults = %+v", d)
	}
	c := ModelSchedulerConfigFromGetenv(get(map[string]string{
		"GATEWAY_MODEL_INFLIGHT": "8", "GATEWAY_MODEL_INFLIGHT_PER_COMPUTER": "3",
		"GATEWAY_MODEL_QUEUE_DEPTH": "10", "GATEWAY_MODEL_QUEUE_WAIT": "30s",
	}))
	if c.Global != 8 || c.PerComputer != 3 || c.QueueDepth != 10 || c.MaxWait != 30*time.Second {
		t.Fatalf("parsed = %+v", c)
	}
	bad := ModelSchedulerConfigFromGetenv(get(map[string]string{
		"GATEWAY_MODEL_INFLIGHT": "-1", "GATEWAY_MODEL_INFLIGHT_PER_COMPUTER": "x",
		"GATEWAY_MODEL_QUEUE_DEPTH": "0", "GATEWAY_MODEL_QUEUE_WAIT": "soon",
	}))
	if bad != d {
		t.Fatalf("unreadable values must fall back to the defaults: %+v", bad)
	}
	// A wait that would eat the call's whole deadline is capped, so a long
	// wait still ends in a 429 the guest retries, never a silent hang-up.
	if long := ModelSchedulerConfigFromGetenv(get(map[string]string{"GATEWAY_MODEL_QUEUE_WAIT": "1h"})); long.MaxWait != maxModelQueueWait || maxModelQueueWait > inferenceTimeout/2 {
		t.Fatalf("a 1h wait resolved to %v (cap %v, call deadline %v)", long.MaxWait, maxModelQueueWait, inferenceTimeout)
	}
	// A per-computer cap above the global cap is the global cap.
	if s := NewModelScheduler(ModelSchedulerConfig{Global: 2, PerComputer: 9}); s.cfg.PerComputer != 2 {
		t.Fatalf("per-computer cap %d above the global 2", s.cfg.PerComputer)
	}
}

// --- handler integration ---

// gateProvider blocks each call until released, so a test can hold slots.
type gateProvider struct {
	name    string
	gate    chan struct{}
	calls   atomic.Int64
	panicOn bool
	stream  bool
}

func (g *gateProvider) Name() string { return g.name }
func (g *gateProvider) IsReal() bool { return true }
func (g *gateProvider) Call(ctx context.Context, req provider.LLMRequest) (*provider.LLMResponse, error) {
	g.calls.Add(1)
	if g.panicOn {
		panic("provider exploded")
	}
	select {
	case <-g.gate:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return &provider.LLMResponse{ID: "r", Text: "ok", Model: "m", StopReason: "end_turn", ProviderName: g.name}, nil
}
func (g *gateProvider) Stream(ctx context.Context, req provider.LLMRequest, onChunk func(provider.StreamChunk)) (*provider.LLMResponse, error) {
	resp, err := g.Call(ctx, req)
	if err == nil {
		onChunk(provider.StreamChunk{Type: "content_block_delta", Delta: "ok"})
	}
	return resp, err
}

// releaseOne lets one blocked provider call finish.
func releaseOne(t *testing.T, g *gateProvider) {
	t.Helper()
	select {
	case g.gate <- struct{}{}:
	case <-time.After(3 * time.Second):
		t.Fatal("no provider call was waiting to be released")
	}
}

func schedulerHandler(t *testing.T, p provider.Provider, limiter *PerAutoputerRateLimiter, s *ModelScheduler) (*Handler, *IdentityRegistry) {
	t.Helper()
	reg := NewIdentityRegistry(time.Hour)
	h := NewHandlerWithRateLimit(reg, p, limiter)
	if s != nil {
		h.SetModelScheduler(s)
	}
	return h, reg
}

func inferAs(h *Handler, raw string, stream bool) *httptest.ResponseRecorder {
	body := `{"messages":[{"role":"user","content":[{"type":"text","text":"hi"}]}],"stream":` + map[bool]string{true: "true", false: "false"}[stream] + `}`
	req := httptest.NewRequest(http.MethodPost, "/provider/v1/inference", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+raw)
	w := httptest.NewRecorder()
	h.HandleInference(w, req)
	return w
}

func TestModelSchedulerQueuesInsteadOfRejectingThroughTheHandler(t *testing.T) {
	gp := &gateProvider{name: "bedrock", gate: make(chan struct{})}
	s := newTestScheduler(1, 1, 16, time.Minute)
	// The old limiter would reject the second call in the window; the
	// scheduler replaces it for model calls.
	h, reg := schedulerHandler(t, gp, NewPerAutoputerRateLimiter(1, time.Minute), s)
	a, _ := reg.IssueCredential("computer-a")
	b, _ := reg.IssueCredential("computer-b")

	var wg sync.WaitGroup
	codes := make(chan int, 8)
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			codes <- inferAs(h, a.RawToken, false).Code
		}()
	}
	waitUntil(t, "a's calls to queue behind one slot", func() bool { st := s.Stats(); return st.InFlight == 1 && st.Queued == 2 })
	// b's one call is served ahead of a's backlog once a slot frees.
	bDone := make(chan int, 1)
	go func() { bDone <- inferAs(h, b.RawToken, false).Code }()
	waitUntil(t, "b to queue", func() bool { return s.Stats().Queued == 3 })
	releaseOne(t, gp) // finish a's running call: the slot goes to b, not to a's backlog
	releaseOne(t, gp) // let b's call finish
	select {
	case <-bDone:
	case <-time.After(3 * time.Second):
		t.Fatal("b's single call waited behind a's backlog")
	}
	close(gp.gate)
	wg.Wait()
	close(codes)
	for code := range codes {
		if code != http.StatusOK {
			t.Fatalf("a queued call ended with %d, want 200 (queued, not rejected)", code)
		}
	}
	if st := s.Stats(); st.InFlight != 0 || st.Queued != 0 {
		t.Fatalf("slots leaked: %+v", st)
	}
}

func TestModelSchedulerTellsAFullQueueToComeBack(t *testing.T) {
	gp := &gateProvider{name: "bedrock", gate: make(chan struct{})}
	s := newTestScheduler(1, 1, 1, time.Minute)
	h, reg := schedulerHandler(t, gp, nil, s)
	a, _ := reg.IssueCredential("computer-a")
	first := make(chan *httptest.ResponseRecorder, 1)
	go func() { first <- inferAs(h, a.RawToken, false) }()
	waitUntil(t, "first call to run", func() bool { return s.Stats().InFlight == 1 })
	second := make(chan *httptest.ResponseRecorder, 1)
	go func() { second <- inferAs(h, a.RawToken, false) }()
	waitUntil(t, "second call to queue", func() bool { return s.Stats().Queued == 1 })
	w := inferAs(h, a.RawToken, false)
	if w.Code != http.StatusTooManyRequests || w.Header().Get("Retry-After") == "" {
		t.Fatalf("overflow call: %d retry-after=%q, want 429 with Retry-After", w.Code, w.Header().Get("Retry-After"))
	}
	close(gp.gate)
	<-first
	<-second
}

func TestModelSchedulerAPanickingProviderReleasesItsSlot(t *testing.T) {
	gp := &gateProvider{name: "bedrock", gate: make(chan struct{}), panicOn: true}
	s := newTestScheduler(1, 1, 4, time.Minute)
	h, reg := schedulerHandler(t, gp, nil, s)
	a, _ := reg.IssueCredential("computer-a")
	func() {
		defer func() { _ = recover() }()
		inferAs(h, a.RawToken, false)
	}()
	if st := s.Stats(); st.InFlight != 0 {
		t.Fatalf("a panicking provider kept its slot: %+v", st)
	}
	gp.panicOn = false
	close(gp.gate)
	if w := inferAs(h, a.RawToken, false); w.Code != http.StatusOK {
		t.Fatalf("after a panic: %d", w.Code)
	}
}

func TestModelSchedulerStreamingHoldsTheSlotUntilTheStreamEnds(t *testing.T) {
	gp := &gateProvider{name: "bedrock", gate: make(chan struct{})}
	s := newTestScheduler(1, 1, 4, time.Minute)
	h, reg := schedulerHandler(t, gp, nil, s)
	a, _ := reg.IssueCredential("computer-a")
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() { done <- inferAs(h, a.RawToken, true) }()
	waitUntil(t, "the stream to hold a slot", func() bool { return s.Stats().InFlight == 1 })
	select {
	case <-done:
		t.Fatal("the stream ended early")
	case <-time.After(30 * time.Millisecond):
	}
	if s.Stats().InFlight != 1 {
		t.Fatal("the slot was released while the stream was running")
	}
	close(gp.gate)
	<-done
	if s.Stats().InFlight != 0 {
		t.Fatal("the slot was not released when the stream ended")
	}
}

func TestModelSchedulerCancelledCallerFreesItsQueueEntry(t *testing.T) {
	gp := &gateProvider{name: "bedrock", gate: make(chan struct{})}
	s := newTestScheduler(1, 1, 4, time.Minute)
	h, reg := schedulerHandler(t, gp, nil, s)
	a, _ := reg.IssueCredential("computer-a")
	running := make(chan struct{})
	go func() { inferAs(h, a.RawToken, false); close(running) }()
	waitUntil(t, "the first call to run", func() bool { return s.Stats().InFlight == 1 })

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodPost, "/provider/v1/inference", strings.NewReader(`{"messages":[{"role":"user","content":[{"type":"text","text":"hi"}]}]}`)).WithContext(ctx)
	req.Header.Set("Authorization", "Bearer "+a.RawToken)
	finished := make(chan struct{})
	go func() { h.HandleInference(httptest.NewRecorder(), req); close(finished) }()
	waitUntil(t, "the second call to queue", func() bool { return s.Stats().Queued == 1 })
	cancel()
	select {
	case <-finished:
	case <-time.After(2 * time.Second):
		t.Fatal("a cancelled caller stayed in the queue")
	}
	if gp.calls.Load() != 1 {
		t.Fatalf("the cancelled call reached the provider (%d calls)", gp.calls.Load())
	}
	close(gp.gate)
	<-running
}

func TestModelSchedulerIsOffByDefaultAndKeepsTheOldLimiter(t *testing.T) {
	gp := &gateProvider{name: "bedrock", gate: make(chan struct{})}
	close(gp.gate)
	h, reg := schedulerHandler(t, gp, NewPerAutoputerRateLimiter(1, time.Minute), nil)
	a, _ := reg.IssueCredential("computer-a")
	if w := inferAs(h, a.RawToken, false); w.Code != http.StatusOK {
		t.Fatalf("first call: %d", w.Code)
	}
	if w := inferAs(h, a.RawToken, false); w.Code != http.StatusTooManyRequests {
		t.Fatalf("with no scheduler the old fixed window must still reject the second call: %d", w.Code)
	}
}

func TestModelSchedulerLeavesSearchOnTheOldLimiter(t *testing.T) {
	gp := &gateProvider{name: "bedrock", gate: make(chan struct{})}
	close(gp.gate)
	s := newTestScheduler(1, 1, 4, time.Minute)
	h, reg := schedulerHandler(t, gp, NewPerAutoputerRateLimiter(1, time.Minute), s)
	a, _ := reg.IssueCredential("computer-a")
	search := func() int {
		req := httptest.NewRequest(http.MethodPost, "/provider/v1/search", strings.NewReader(`{}`))
		req.Header.Set("Authorization", "Bearer "+a.RawToken)
		w := httptest.NewRecorder()
		h.HandleSearch(w, req)
		return w.Code
	}
	_ = search()
	if code := search(); code != http.StatusTooManyRequests {
		t.Fatalf("search lost its rate bucket under the scheduler: %d", code)
	}
}

// deadlineProvider records the deadline each call's context carries; the
// first call blocks on gate so a test can make the next one queue.
type deadlineProvider struct {
	gateProvider
	mu        sync.Mutex
	deadlines []time.Time
}

func (d *deadlineProvider) Call(ctx context.Context, req provider.LLMRequest) (*provider.LLMResponse, error) {
	dl, _ := ctx.Deadline()
	d.mu.Lock()
	d.deadlines = append(d.deadlines, dl)
	d.mu.Unlock()
	return d.gateProvider.Call(ctx, req)
}

func (d *deadlineProvider) Stream(ctx context.Context, req provider.LLMRequest, onChunk func(provider.StreamChunk)) (*provider.LLMResponse, error) {
	resp, err := d.Call(ctx, req)
	if err == nil {
		onChunk(provider.StreamChunk{Type: "content_block_delta", Delta: "ok"})
	}
	return resp, err
}

// Batch 1 review, RES-11 finding 11: the queue wait counts against the call's
// deadline, so wait plus call stay inside inferenceTimeout and therefore inside
// the 10m30s write and guest client timeouts.
func TestModelSchedulerQueueWaitCountsAgainstTheCallDeadline(t *testing.T) {
	const queued = 250 * time.Millisecond
	cases := []struct {
		name string
		call func(h *Handler, raw string) int
	}{
		{"inference", func(h *Handler, raw string) int { return inferAs(h, raw, false).Code }},
		{"inference stream", func(h *Handler, raw string) int { return inferAs(h, raw, true).Code }},
		{"openai chat", func(h *Handler, raw string) int {
			body := `{"model":"m","messages":[{"role":"user","content":"hi"}]}`
			req := httptest.NewRequest(http.MethodPost, "/provider/openai/v1/chat/completions", strings.NewReader(body))
			req.Header.Set("Authorization", "Bearer "+raw)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			h.HandleOpenAIChatCompletions(w, req)
			return w.Code
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dp := &deadlineProvider{gateProvider: gateProvider{name: "bedrock", gate: make(chan struct{})}}
			s := newTestScheduler(1, 1, 4, time.Minute)
			h, reg := schedulerHandler(t, dp, nil, s)
			a, _ := reg.IssueCredential("computer-a")
			first := make(chan int, 1)
			go func() { first <- tc.call(h, a.RawToken) }()
			waitUntil(t, "the first call to hold the slot", func() bool { return s.Stats().InFlight == 1 })
			asked := time.Now()
			second := make(chan int, 1)
			go func() { second <- tc.call(h, a.RawToken) }()
			waitUntil(t, "the second call to queue", func() bool { return s.Stats().Queued == 1 })
			time.Sleep(queued)
			releaseOne(t, &dp.gateProvider)
			if code := <-first; code != http.StatusOK {
				t.Fatalf("first call: %d", code)
			}
			releaseOne(t, &dp.gateProvider)
			if code := <-second; code != http.StatusOK {
				t.Fatalf("second call: %d", code)
			}
			dp.mu.Lock()
			defer dp.mu.Unlock()
			if len(dp.deadlines) != 2 || dp.deadlines[1].IsZero() {
				t.Fatalf("deadlines seen: %v", dp.deadlines)
			}
			if budget := dp.deadlines[1].Sub(asked); budget > inferenceTimeout+queued/2 {
				t.Fatalf("the queued call's deadline is %v after it asked for a slot; want at most %v (the wait must count)", budget, inferenceTimeout)
			}
		})
	}
}

// A caller that has already gone away is never handed a slot, even when one
// is free, and returns at once (B7,
// docs/problems/gateway-request-bodies-unbounded-2026-10-22.md).
func TestModelSchedulerRefusesAnAlreadyCancelledCaller(t *testing.T) {
	s := newTestScheduler(4, 4, 8, time.Minute)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	started := time.Now()
	for i := 0; i < 200; i++ {
		release, _, err := s.Acquire(ctx, "gone")
		if err == nil {
			release()
			t.Fatalf("try %d: a cancelled caller was handed a slot", i)
		}
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("try %d: err = %v, want context.Canceled", i, err)
		}
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("200 cancelled acquires took %s", elapsed)
	}
	if st := s.Stats(); st.Granted != 0 || st.InFlight != 0 || st.Queued != 0 {
		t.Fatalf("cancelled callers left a mark: %+v", st)
	}
	// With the computer's slots taken, a cancelled caller is not queued.
	holders := []func(){mustAcquire(t, s, "busy"), mustAcquire(t, s, "busy"), mustAcquire(t, s, "busy"), mustAcquire(t, s, "busy")}
	if _, _, err := s.Acquire(ctx, "busy"); !errors.Is(err, context.Canceled) {
		t.Fatalf("a cancelled caller behind a full computer: %v", err)
	}
	if st := s.Stats(); st.Queued != 0 {
		t.Fatalf("a cancelled caller was queued: %+v", st)
	}
	for _, release := range holders {
		release()
	}
}
