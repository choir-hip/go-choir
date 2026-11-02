package gateway

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/health"
	"github.com/yusefmosiah/go-choir/internal/provider"
)

// CircuitBreakingProvider wraps a provider.Provider with circuit breakers so
// the gateway stops forwarding into a failing upstream instead of retrying it
// endlessly (production-readiness checklist: "LLM provider failures
// circuit-break (not retry endlessly)").
//
// It keeps one breaker per model and one for the provider itself. A
// provider serves several models, and most failures belong to one of them:
// a free preview model's upstream rate limit (429), its 5xx, its timeouts.
// Those open that model's breaker only; one provider-wide breaker let
// cat-jam-free's 429s fail deepseek on the same provider
// (docs/archive/problems/free-model-429-opens-provider-circuit-stub-stories-2026-10-14.md).
// The provider breaker counts only failures to reach the provider at all
// (refused or reset connections, DNS, TLS), which no model escapes.
//
// A 429 still counts against its model. The open model breaker is the short
// per-model backoff: it lasts OpenTimeout, fails the next calls at once with
// "circuit open", and that is what moves a run to its next pool member
// instead of an 85 s rate-limit backoff against the same model. The
// upstream's Retry-After does not survive the provider's sanitized errors,
// so it is not read here.
//
// The model breaker is each pool member's health, mirroring the search
// plane's per-provider cooldowns (docs/design/model-pool-autoroute-2026-11-02.md):
// the open duration doubles on each failed probe and resets on a success; an
// exhausted quota (provider.ErrUpstreamQuotaExhausted) opens it on the first
// one, for a longer cooldown; a Synthetic concurrency 429 opens it only after
// ConcurrencyStreakBeforeCooldown in a row with no success between. The
// guest fails over at once on "circuit open", so a cooling member costs a
// run one fast refusal, not an 85 s same-model retry.
//
// The decorator is transparent when the upstream is healthy: a closed breaker
// forwards every call and records the outcome.
type CircuitBreakingProvider struct {
	inner   provider.Provider
	cfg     health.BreakerConfig
	breaker *health.CircuitBreaker
	// quotaCooldown is the open duration for an exhausted quota's trips-th
	// open in a row.
	quotaCooldown func(trips int) time.Duration

	mu     sync.Mutex
	models map[string]*health.CircuitBreaker
	// concurrencyStreak counts each model's concurrency 429s since its last
	// success.
	concurrencyStreak map[string]int
}

// ConcurrencyStreakBeforeCooldown is how many concurrency 429s in a row, with
// no success between them, put a model in cooldown. One burst is the
// account's in-flight ceiling, not model ill-health, and must not demote a
// healthy member (docs/problems/synthetic-provider-onboarding-2026-10-22.md);
// a streak this long means calls are not getting through at all. The search
// plane's EmptyStreakBeforeCooldown is the same rule.
const ConcurrencyStreakBeforeCooldown = 10

const (
	// defaultModelCooldownMax bounds the doubling cooldown of a model that
	// keeps failing (5xx, timeouts, rate limits): a probe every ten minutes.
	defaultModelCooldownMax = 10 * time.Minute
	// Quota exhaustion does not clear in seconds, but an owner reset or a
	// window rollover should be seen the same hour: 5 min doubling to 1 h.
	// CHOIR_MODEL_QUOTA_COOLDOWN_BASE / _MAX (Go durations) override.
	defaultQuotaCooldownBase = 5 * time.Minute
	defaultQuotaCooldownMax  = time.Hour
)

// NewCircuitBreakingProvider wraps p with circuit breakers using cfg. Unless
// cfg names its own Backoff, an open breaker stays open cfg.OpenTimeout on
// its first trip, doubling on each failed probe up to ten minutes (or
// OpenTimeout when that is longer).
func NewCircuitBreakingProvider(p provider.Provider, cfg health.BreakerConfig) *CircuitBreakingProvider {
	if cfg.Backoff == nil {
		base := cfg.OpenTimeout
		if base <= 0 {
			base = 30 * time.Second
		}
		limit := defaultModelCooldownMax
		if base > limit {
			limit = base
		}
		cfg.Backoff = doublingCooldown(base, limit)
	}
	qBase := envDuration("CHOIR_MODEL_QUOTA_COOLDOWN_BASE", defaultQuotaCooldownBase)
	qMax := envDuration("CHOIR_MODEL_QUOTA_COOLDOWN_MAX", defaultQuotaCooldownMax)
	if qMax < qBase {
		qMax = qBase
	}
	return &CircuitBreakingProvider{
		inner:             p,
		cfg:               cfg,
		breaker:           health.NewCircuitBreaker(cfg),
		quotaCooldown:     doublingCooldown(qBase, qMax),
		models:            make(map[string]*health.CircuitBreaker),
		concurrencyStreak: make(map[string]int),
	}
}

// doublingCooldown returns base on the first trip, doubling on each trip in
// a row, never above limit.
func doublingCooldown(base, limit time.Duration) func(trips int) time.Duration {
	return func(trips int) time.Duration {
		d := base
		for i := 1; i < trips && d < limit; i++ {
			d *= 2
		}
		if d > limit {
			d = limit
		}
		return d
	}
}

func envDuration(name string, def time.Duration) time.Duration {
	if v := strings.TrimSpace(os.Getenv(name)); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return d
		}
	}
	return def
}

// Breaker returns the provider-wide breaker (failures to reach the
// provider) for observability and admin reset endpoints.
func (c *CircuitBreakingProvider) Breaker() *health.CircuitBreaker { return c.breaker }

// ModelBreaker returns the breaker for model, creating it closed on first
// use. The empty model is the provider's default model.
func (c *CircuitBreakingProvider) ModelBreaker(model string) *health.CircuitBreaker {
	model = strings.TrimSpace(model)
	c.mu.Lock()
	defer c.mu.Unlock()
	b, ok := c.models[model]
	if !ok {
		b = health.NewCircuitBreaker(c.cfg)
		c.models[model] = b
	}
	return b
}

// modelBreakers returns the model breakers created so far, by model.
func (c *CircuitBreakingProvider) modelBreakers() map[string]*health.CircuitBreaker {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make(map[string]*health.CircuitBreaker, len(c.models))
	for model, b := range c.models {
		out[model] = b
	}
	return out
}

// lookupModelBreaker returns model's breaker if it exists.
func (c *CircuitBreakingProvider) lookupModelBreaker(model string) *health.CircuitBreaker {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.models[strings.TrimSpace(model)]
}

// Name returns the wrapped provider's name.
func (c *CircuitBreakingProvider) Name() string { return c.inner.Name() }

// IsReal returns the wrapped provider's realness.
func (c *CircuitBreakingProvider) IsReal() bool { return c.inner.IsReal() }

// Call executes the LLM request through the provider and model breakers.
func (c *CircuitBreakingProvider) Call(ctx context.Context, req provider.LLMRequest) (*provider.LLMResponse, error) {
	var resp *provider.LLMResponse
	err := c.execute(ctx, req.Model, func() error {
		var callErr error
		resp, callErr = c.inner.Call(ctx, req)
		return callErr
	})
	return resp, err
}

// Stream executes the streaming LLM request through the provider and model
// breakers. The breakers record the outcome based on whether the stream
// completed without error; per-chunk failures are surfaced to onChunk
// callers but the breakers only transition on the terminal Stream result.
func (c *CircuitBreakingProvider) Stream(ctx context.Context, req provider.LLMRequest, onChunk func(provider.StreamChunk)) (*provider.LLMResponse, error) {
	var resp *provider.LLMResponse
	err := c.execute(ctx, req.Model, func() error {
		var streamErr error
		resp, streamErr = c.inner.Stream(ctx, req, onChunk)
		return streamErr
	})
	return resp, err
}

// execute runs fn inside the provider breaker and, within it, the model's
// breaker. An open model breaker hands the provider breaker's probe slot
// back untouched: ExecuteClassified records its ErrCircuitOpen as neither a
// failure nor a success.
func (c *CircuitBreakingProvider) execute(ctx context.Context, model string, fn func() error) error {
	modelBreaker := c.ModelBreaker(model)
	modelOpen := false
	err := c.breaker.ExecuteClassified(func() error {
		ran := false
		var callErr error
		innerErr := modelBreaker.ExecuteClassified(func() error {
			ran = true
			callErr = fn()
			return callErr
		}, func(err error) bool { return providerFailureCounts(ctx, err) })
		if innerErr == health.ErrCircuitOpen && !ran {
			modelOpen = true
		}
		if ran {
			c.observeModelOutcome(ctx, model, modelBreaker, callErr)
		}
		return innerErr
	}, func(err error) bool { return providerUnreachable(ctx, err) })
	if err == health.ErrCircuitOpen {
		if modelOpen {
			// No status code in this message: the runtime reads a "429"
			// text as a rate limit to wait out, and an open circuit as a
			// reason to fail over now.
			return fmt.Errorf("provider %s model %s: circuit open (model unhealthy)", c.inner.Name(), modelLabel(model))
		}
		return fmt.Errorf("provider %s: circuit open (upstream unhealthy)", c.inner.Name())
	}
	return err
}

// observeModelOutcome applies the outcomes the breaker's failure count does
// not: an exhausted quota opens the model at once, and a concurrency 429
// opens it only at the end of a streak. A success ends the streak.
func (c *CircuitBreakingProvider) observeModelOutcome(ctx context.Context, model string, b *health.CircuitBreaker, err error) {
	key := strings.TrimSpace(model)
	switch {
	case err == nil:
		c.mu.Lock()
		delete(c.concurrencyStreak, key)
		c.mu.Unlock()
	case ctx.Err() != nil && errors.Is(ctx.Err(), context.Canceled):
		// The caller went away: no evidence either way.
	case provider.IsQuotaExhausted(err):
		if b.Trip(c.quotaCooldown) {
			snap := b.Snapshot()
			log.Printf("gateway: model %s/%s cooling down (quota exhausted, trip %d, until %s)",
				c.inner.Name(), modelLabel(model), snap.Trips, formatUntil(snap.OpenUntil))
		}
	case errors.Is(err, provider.ErrUpstreamConcurrencyLimit):
		c.mu.Lock()
		c.concurrencyStreak[key]++
		streak := c.concurrencyStreak[key]
		if streak >= ConcurrencyStreakBeforeCooldown {
			delete(c.concurrencyStreak, key)
		}
		c.mu.Unlock()
		if streak >= ConcurrencyStreakBeforeCooldown && b.Trip(nil) {
			snap := b.Snapshot()
			log.Printf("gateway: model %s/%s cooling down (%d concurrency 429s in a row, trip %d, until %s)",
				c.inner.Name(), modelLabel(model), streak, snap.Trips, formatUntil(snap.OpenUntil))
		}
	}
}

func formatUntil(t *time.Time) string {
	if t == nil {
		return "-"
	}
	return t.UTC().Format(time.RFC3339)
}

// modelLabel names a model in errors and breaker keys; the empty model is
// the provider's default.
func modelLabel(model string) string {
	if model = strings.TrimSpace(model); model != "" {
		return model
	}
	return "(default)"
}

var providerStatusPattern = regexp.MustCompile(`: status (\d{3})\b`)

// providerFailureCounts reports whether err is evidence that the upstream is
// unhealthy for the model called. A caller that went away and a request the
// upstream rejected (4xx other than 408 and 429) say nothing about the
// provider; counting them let one computer's bad requests open the provider
// for every computer
// (problems/inference-breaker-trips-on-client-errors-2026-10-10.md).
func providerFailureCounts(ctx context.Context, err error) bool {
	if err == nil {
		return false
	}
	// A deadline overrun still counts: the provider was slow.
	if errors.Is(ctx.Err(), context.Canceled) {
		return false
	}
	// An image the gateway refused before the call says nothing about the
	// upstream (problems/models-configured-text-only-2026-10-15.md, FM1).
	if provider.IsMediaError(err) {
		return false
	}
	// A call that ended waiting for an image decode slot never reached
	// the upstream (bunch 116, F2).
	if errors.Is(err, provider.ErrImageDecodeWait) {
		return false
	}
	// Synthetic's concurrency 429 says the account has too many calls in
	// flight, not that the model is unhealthy, and a call that ended waiting
	// for a Synthetic slot never reached the upstream. These calls fail over
	// on their own (the guest treats them as transient); opening the breaker
	// on them would also fail healthy calls unsent (docs/problems/synthetic-provider-onboarding-2026-10-22.md,
	// "Staging: concurrency 429s and the circuit breaker"). Other providers'
	// 429s still count: that is the designed per-model backoff.
	if errors.Is(err, provider.ErrUpstreamConcurrencyLimit) || errors.Is(err, provider.ErrConcurrencyGateWait) {
		return false
	}
	// An exhausted quota opens the breaker at once with its own cooldown
	// (observeModelOutcome); counting it too would stack a second trip.
	if provider.IsQuotaExhausted(err) {
		return false
	}
	if m := providerStatusPattern.FindStringSubmatch(err.Error()); m != nil {
		code, _ := strconv.Atoi(m[1])
		if code >= 400 && code < 500 && code != 408 && code != 429 {
			return false
		}
	}
	return true
}

// providerUnreachable reports whether err says the provider itself could
// not be reached, which no model escapes: the call failed in transport
// ("<provider>: http call: ...") before any response, and not because a
// deadline ran out (a slow model is that model's problem) or the caller
// went away.
func providerUnreachable(ctx context.Context, err error) bool {
	if err == nil || ctx.Err() != nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return false
	}
	return strings.Contains(err.Error(), ": http call: ")
}

// WrapMultiProvider returns a new MultiProvider where every registered provider
// is wrapped with a CircuitBreakingProvider using cfg. Providers registered
// after wrapping are NOT wrapped; call this once after ResolveAll.
func WrapMultiProvider(mp *provider.MultiProvider, cfg health.BreakerConfig) *provider.MultiProvider {
	out := provider.NewMultiProvider()
	for _, name := range mp.Names() {
		p := mp.Get(name)
		if p == nil {
			continue
		}
		out.Register(name, NewCircuitBreakingProvider(p, cfg))
	}
	return out
}

// BreakerRegistry tracks circuit breakers so the gateway can expose their
// state via health/ops endpoints and reset them: plain breakers by name, and
// each wrapped LLM provider's breakers as "<provider>" (the provider-wide
// breaker) and "<provider>/<model>" (one per model it has served).
type BreakerRegistry struct {
	mu        sync.Mutex
	breakers  map[string]*health.CircuitBreaker
	providers map[string]*CircuitBreakingProvider
}

// NewBreakerRegistry returns an empty registry.
func NewBreakerRegistry() *BreakerRegistry {
	return &BreakerRegistry{
		breakers:  make(map[string]*health.CircuitBreaker),
		providers: make(map[string]*CircuitBreakingProvider),
	}
}

// Register associates a breaker with a name.
func (r *BreakerRegistry) Register(name string, b *health.CircuitBreaker) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.breakers[name] = b
}

// RegisterProvider registers a wrapped LLM provider's provider-wide and
// per-model breakers under name.
func (r *BreakerRegistry) RegisterProvider(name string, p *CircuitBreakingProvider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[name] = p
}

// Snapshot returns the current state of every registered breaker, model
// breakers keyed "<provider>/<model>".
func (r *BreakerRegistry) Snapshot() map[string]health.Snapshot {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make(map[string]health.Snapshot, len(r.breakers)+len(r.providers))
	for name, b := range r.breakers {
		out[name] = b.Snapshot()
	}
	for name, p := range r.providers {
		out[name] = p.Breaker().Snapshot()
		for model, b := range p.modelBreakers() {
			out[name+"/"+modelLabel(model)] = b.Snapshot()
		}
	}
	return out
}

// Reset clears the named breaker, returning it to closed. A provider name
// resets the provider-wide breaker and every model breaker under it;
// "<provider>/<model>" resets that model's breaker alone.
func (r *BreakerRegistry) Reset(name string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if b, ok := r.breakers[name]; ok {
		b.Reset()
		return true
	}
	if p, ok := r.providers[name]; ok {
		p.Breaker().Reset()
		for _, b := range p.modelBreakers() {
			b.Reset()
		}
		return true
	}
	if slash := strings.Index(name, "/"); slash > 0 {
		if p, ok := r.providers[name[:slash]]; ok {
			model := name[slash+1:]
			if model == modelLabel("") {
				model = ""
			}
			if b := p.lookupModelBreaker(model); b != nil {
				b.Reset()
				return true
			}
		}
	}
	return false
}

// Names returns the registered breaker and provider names.
func (r *BreakerRegistry) Names() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	names := make([]string, 0, len(r.breakers)+len(r.providers))
	for n := range r.breakers {
		names = append(names, n)
	}
	for n := range r.providers {
		names = append(names, n)
	}
	return names
}

// breakerFor returns the circuit breaker registered for the named service,
// or the provider-wide breaker of a registered provider, or nil. It is used
// by the per-service health endpoint to surface the breaker state alongside
// the dependency probe (M22b / C20). The lookup is case-sensitive.
func (r *BreakerRegistry) breakerFor(name string) *health.CircuitBreaker {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if b, ok := r.breakers[name]; ok {
		return b
	}
	if p, ok := r.providers[name]; ok {
		return p.Breaker()
	}
	return nil
}
