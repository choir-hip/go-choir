package gateway

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/health"
)

// docs/problems/sub-call-spacer-outlived-its-bucket-2026-11-24.md: with the
// guest's fixed spacer gone, a burst of sub-call model calls is held back only
// by the provider's own refusals, and that back-off is the gateway's per-model
// breaker, which every caller of the model shares. Pinned here: with no
// refusal a burst of 16 is not spaced or limited at all, and when the
// provider answers again after a refusal the model opens to the whole burst.
// (The refusal side, from inside a concurrent burst, is
// TestBurstOpensTheBreakerOnceFromInsideTheBurst.)
func TestBurstOfSubCallsIsUngatedWithoutRefusalsAndOpensAgainOnARecoveredProbe(t *testing.T) {
	const burst = 16
	clock := &fakeClock{now: time.Unix(1000, 0)}
	stub := newModelStub(map[string]error{})
	cbp := NewCircuitBreakingProvider(stub, health.BreakerConfig{FailureThreshold: 2, OpenTimeout: time.Minute, Now: clock.Now})
	run := func() (ok int, open int) {
		var wg sync.WaitGroup
		var mu sync.Mutex
		for i := 0; i < burst; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				err := callModel(cbp, "m")
				mu.Lock()
				defer mu.Unlock()
				switch {
				case err == nil:
					ok++
				case strings.Contains(err.Error(), "circuit open"):
					open++
				}
			}()
		}
		wg.Wait()
		return
	}

	if ok, open := run(); ok != burst || open != 0 || stub.count("m") != burst {
		t.Fatalf("no refusals: %d ok, %d refused at the gate, %d upstream calls; want all %d through", ok, open, stub.count("m"), burst)
	}

	stub.mu.Lock()
	stub.errs["m"] = errors.New("opencode-go: status 429 Too Many Requests (sanitized)")
	stub.mu.Unlock()
	_ = callModel(cbp, "m")
	_ = callModel(cbp, "m") // two refusals open the model for every caller
	if ok, open := run(); ok != 0 || open != burst {
		t.Fatalf("after refusals: %d ok, %d refused at the gate; want %d refused without reaching the provider", ok, open, burst)
	}

	stub.mu.Lock()
	delete(stub.errs, "m")
	stub.mu.Unlock()
	clock.Advance(time.Minute)
	if err := callModel(cbp, "m"); err != nil {
		t.Fatalf("the probe after the open window failed: %v", err)
	}
	if ok, open := run(); ok != burst || open != 0 {
		t.Fatalf("after recovery: %d ok, %d refused; want all %d through", ok, open, burst)
	}
}
