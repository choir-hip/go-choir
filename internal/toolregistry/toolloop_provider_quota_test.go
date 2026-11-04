package toolregistry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Provider quota park, the guest half
// (docs/problems/provider-quota-kills-desk-loops-2026-11-04.md). Ways it
// could fail, written down for these tests:
//   - the loop still retries the quota 3 times at 15 s and dies (the
//     incident);
//   - the park never resumes, or resumes before the reset time;
//   - the wait is untyped, or names no provider, model or reset time;
//   - the wait is never cleared, so management and Texture read a loop
//     that resumed as still waiting;
//   - a model failure parks too (a dying model must not hold loops);
//   - a pool with a member left parks instead of failing over;
//   - a cancelled run stays parked.

func shortenQuotaPark(t *testing.T) {
	t.Helper()
	base, max, min, transient := providerQuotaBackoffBase, providerQuotaCheckMax, providerQuotaMinWait, transientProviderRetryDelay
	providerQuotaBackoffBase, providerQuotaCheckMax, providerQuotaMinWait, transientProviderRetryDelay = 10*time.Millisecond, 200*time.Millisecond, time.Millisecond, time.Millisecond
	t.Cleanup(func() {
		providerQuotaBackoffBase, providerQuotaCheckMax, providerQuotaMinWait, transientProviderRetryDelay = base, max, min, transient
	})
}

type quotaRun struct {
	mu     sync.Mutex
	waits  []*types.WaitState
	ends   []string
	phases map[string]int
}

func (r *quotaRun) record(_ context.Context, w *types.WaitState, ended string) {
	r.mu.Lock()
	r.waits = append(r.waits, w)
	if w == nil {
		r.ends = append(r.ends, ended)
	}
	r.mu.Unlock()
}

func runQuota(ctx context.Context, p *poolProvider, r *quotaRun, primary provideriface.LLMSelection, fallbacks ...provideriface.LLMSelection) (string, error) {
	r.phases = map[string]int{}
	var mu sync.Mutex
	emit := func(_ types.EventKind, name string, _ json.RawMessage) {
		mu.Lock()
		r.phases[name]++
		mu.Unlock()
	}
	text, _, err := RunToolLoop(ctx, p, nil, []json.RawMessage{json.RawMessage(`{"role":"user","content":"hello"}`)}, "system", 0, emit, nil,
		WithToolLoopLLMConfig(primary), WithToolLoopFallbackLLMConfigs(fallbacks), WithToolLoopConversationID("run-1"),
		WithProviderWait(r.record))
	return text, err
}

func quotaErr(retryAt time.Time) error {
	return fmt.Errorf("gateway call failed: %w", &provideriface.ProviderQuotaError{Provider: "synthetic", Model: lunaSyntheticModel,
		Reason: "provider quota exhausted (synthetic)", Message: "synthetic: quota exhausted: status 429 Too Many Requests (sanitized)", RetryAt: retryAt})
}

func TestQuotaParksTheLoopAndResumesAfterTheReset(t *testing.T) {
	shortenQuotaPark(t)
	reset := time.Now().Add(80 * time.Millisecond)
	p := &poolProvider{errs: map[string][]error{"synthetic": {quotaErr(reset), quotaErr(reset), nil}}}
	r := &quotaRun{}
	started := time.Now()
	text, err := runQuota(context.Background(), p, r, synthMember)
	if err != nil || text != "done by synthetic" {
		t.Fatalf("run = %q, %v; want it to resume and finish on the same model", text, err)
	}
	if time.Now().Before(reset) {
		t.Fatalf("resumed after %s, before the reset time", time.Since(started))
	}
	if r.phases["provider_quota"] != 2 || r.phases["provider_quota_cleared"] != 1 || r.phases["transient_provider"] != 0 || r.phases["provider_rate_limit"] != 0 {
		t.Fatalf("phases = %v, want two parks, one clear, no outage retries", r.phases)
	}
	if len(r.waits) != 3 || r.waits[2] != nil || len(r.ends) != 1 || r.ends[0] != ProviderWaitResumed {
		t.Fatalf("waits = %v ends %v, want two parks then the resume", r.waits, r.ends)
	}
	w := r.waits[0]
	if w.Kind != types.WaitKindProviderQuota || w.Class != types.WaitClassContention || w.Provider != "synthetic" || w.Model != lunaSyntheticModel ||
		!w.ExpectedUntil.Equal(reset.UTC()) || !strings.Contains(w.Sentence, "waiting for the Synthetic quota to reset (about 1 min)") {
		t.Fatalf("wait = %+v, want a typed provider_quota contention wait with the reset time", w)
	}
	if !r.waits[1].Since.Equal(w.Since) {
		t.Fatalf("the re-check moved the wait's start: %s -> %s", w.Since, r.waits[1].Since)
	}
	if line := w.TextureLine(w.Since); !strings.Contains(line, "Waiting 1 min for the Synthetic quota to reset (about 1 min)") {
		t.Fatalf("texture line = %q", line)
	}
}

func TestQuotaWithoutResetTimeBacksOff(t *testing.T) {
	shortenQuotaPark(t)
	p := &poolProvider{errs: map[string][]error{"synthetic": {quotaErr(time.Time{}), quotaErr(time.Time{}), quotaErr(time.Time{}), nil}}}
	r := &quotaRun{}
	if _, err := runQuota(context.Background(), p, r, synthMember); err != nil {
		t.Fatal(err)
	}
	if got := []time.Duration{providerQuotaParkDelay(time.Time{}, time.Now(), 1), providerQuotaParkDelay(time.Time{}, time.Now(), 2), providerQuotaParkDelay(time.Time{}, time.Now(), 9)}; got[0] != 10*time.Millisecond || got[1] != 20*time.Millisecond || got[2] != 200*time.Millisecond {
		t.Fatalf("backoff = %v, want doubling from the base to the cap", got)
	}
	if r.phases["provider_quota"] != 3 || r.waits[0].CheckEverySec < 1 || !r.waits[0].ExpectedUntil.IsZero() {
		t.Fatalf("phases %v, wait %+v", r.phases, r.waits[0])
	}
	// A reset far off is re-checked at the cap, so an early reset is seen.
	if d := providerQuotaParkDelay(time.Now().Add(time.Hour), time.Now(), 1); d != providerQuotaCheckMax {
		t.Fatalf("far reset delay = %s, want the check cap", d)
	}
}

func TestModelFailureDoesNotPark(t *testing.T) {
	shortenQuotaPark(t)
	outage := errors.New("gateway call failed: gateway client: provider synthetic model m: circuit open (model unhealthy) status 502 Bad Gateway")
	p := &poolProvider{errs: map[string][]error{"synthetic": {outage}}}
	r := &quotaRun{}
	_, err := runQuota(context.Background(), p, r, synthMember)
	if err == nil || !strings.Contains(err.Error(), "model unhealthy") {
		t.Fatalf("err = %v, want today's failure after the outage retries", err)
	}
	if len(r.waits) != 0 || r.phases["provider_quota"] != 0 || r.phases["transient_provider"] != 3 {
		t.Fatalf("waits %v phases %v, want no park and the three outage retries", r.waits, r.phases)
	}
}

func TestQuotaWithAPoolMemberLeftFailsOver(t *testing.T) {
	shortenQuotaPark(t)
	p := &poolProvider{errs: map[string][]error{"synthetic": {quotaErr(time.Now().Add(time.Hour))}}}
	r := &quotaRun{}
	text, err := runQuota(context.Background(), p, r, synthMember, lunaMember)
	if err != nil || text != "done by chatgpt" || len(r.waits) != 0 || r.phases["model_failover"] != 1 {
		t.Fatalf("run = %q, %v, waits %v, phases %v; want a failover, no park", text, err, r.waits, r.phases)
	}
}

func TestCancelledRunLeavesThePark(t *testing.T) {
	shortenQuotaPark(t)
	p := &poolProvider{errs: map[string][]error{"synthetic": {quotaErr(time.Now().Add(time.Hour))}}}
	r := &quotaRun{}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := runQuota(ctx, p, r, synthMember)
	if err == nil || !errors.Is(err, context.DeadlineExceeded) || !strings.Contains(err.Error(), "parked on waiting for the Synthetic quota") {
		t.Fatalf("err = %v, want the run's deadline, naming the park", err)
	}
	if n := len(r.waits); n < 2 || r.waits[n-1] != nil || len(r.ends) != 1 || r.ends[0] != ProviderWaitLoopEnded {
		t.Fatalf("waits = %v ends %v, want the park ended as the loop's end, not a reset", r.waits, r.ends)
	}
}
