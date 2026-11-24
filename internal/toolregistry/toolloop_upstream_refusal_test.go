package toolregistry

import (
	"context"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/provideriface"
)

// docs/problems/sub-call-spacer-outlived-its-bucket-2026-11-24.md (review
// round): an upstream refusal the gateway answers 429 with a Retry-After is
// waited out for that long, not for a fixed 5, 20 and 60 seconds, and a run
// with a fallback member fails over at once.
func TestUpstreamRefusalFailsOverAtOnceOrWaitsItsRetryAfter(t *testing.T) {
	original := providerRateLimitRetryDelays
	providerRateLimitRetryDelays = []time.Duration{5 * time.Second}
	t.Cleanup(func() { providerRateLimitRetryDelays = original })
	refusal := &provideriface.UpstreamRefusalError{Provider: "opencode-go", Model: "m", Message: "status 429", RetryAfter: 40 * time.Millisecond}

	// A member left: one call, back at once.
	p := &poolProvider{errs: map[string][]error{"chatgpt": {refusal}}}
	start := time.Now()
	_, err := callToolLoopProviderWithPoolRetries(context.Background(), p, provideriface.ToolLoopRequest{Provider: "chatgpt"}, nil, true, 1)
	if err == nil || p.count("chatgpt") != 1 || time.Since(start) > time.Second {
		t.Fatalf("with a fallback: err %v, %d calls in %s; want the refusal after one call at once", err, p.count("chatgpt"), time.Since(start))
	}

	// No member left: wait the refusal's own Retry-After, then call again.
	p = &poolProvider{errs: map[string][]error{"chatgpt": {refusal, nil}}}
	start = time.Now()
	if _, err := callToolLoopProviderWithPoolRetries(context.Background(), p, provideriface.ToolLoopRequest{Provider: "chatgpt"}, nil, false, 1); err != nil {
		t.Fatalf("without a fallback: %v", err)
	}
	if elapsed := time.Since(start); elapsed < 40*time.Millisecond || elapsed > 2*time.Second || p.count("chatgpt") != 2 {
		t.Fatalf("without a fallback: %d calls in %s; want a wait of its 40 ms Retry-After, not the fixed delays", p.count("chatgpt"), elapsed)
	}
}

// The outer loop's own transient retry (three times, 15 s apart) must not pick
// up a typed upstream refusal that the pool retry already waited out: that is
// the blind retry again, one level up.
func TestUpstreamRefusalIsNotRetriedBlindByTheOuterLoop(t *testing.T) {
	original, transient := providerRateLimitRetryDelays, transientProviderRetryDelay
	providerRateLimitRetryDelays = []time.Duration{0}
	transientProviderRetryDelay = time.Millisecond
	t.Cleanup(func() { providerRateLimitRetryDelays, transientProviderRetryDelay = original, transient })
	refusal := &provideriface.UpstreamRefusalError{Provider: "chatgpt", Model: "gpt-6-luna", Message: "status 429", RetryAfter: time.Millisecond}
	p := &poolProvider{errs: map[string][]error{"chatgpt": {refusal}}}
	_, err, _ := runPool(context.Background(), p, lunaMember)
	if err == nil {
		t.Fatal("a run whose only member keeps refusing succeeded")
	}
	// One call, then one wait of its Retry-After and one more call: no
	// three further blind retries.
	if got := p.count("chatgpt"); got != 2 {
		t.Fatalf("calls = %d, want 2 (the call and the one Retry-After retry)", got)
	}
}
