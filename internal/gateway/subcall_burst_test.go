package gateway

import (
	"bytes"
	"errors"
	"log"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/health"
)

// docs/problems/sub-call-spacer-outlived-its-bucket-2026-11-24.md: with the
// guest's fixed spacer gone, a burst of sub-call model calls is held back
// only by the provider's own refusals, and that back-off is the gateway's
// per-model breaker, which every caller of the model shares (no second gate
// is written). Behaviour pinned: with no refusal a burst of 16 is not
// spaced or limited at all; refusals open the model for every caller at once,
// so the burst does not become 16 refused calls each retrying alone; when the
// provider answers again the model opens to the whole burst.
func TestBurstOfSubCallsIsHeldOnlyByTheProvidersRefusals(t *testing.T) {
	const burst = 16
	var logs bytes.Buffer
	log.SetOutput(&logs)
	defer log.SetOutput(os.Stderr)
	stub := newModelStub(map[string]error{})
	cbp := NewCircuitBreakingProvider(stub, health.BreakerConfig{FailureThreshold: 2, OpenTimeout: 80 * time.Millisecond})
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

	// No refusal: every call of the burst reaches the provider.
	if ok, open := run(); ok != burst || open != 0 || stub.count("m") != burst {
		t.Fatalf("no refusals: %d ok, %d refused at the gate, %d upstream calls; want all %d through", ok, open, stub.count("m"), burst)
	}

	// The provider refuses: two refusals open the model for every caller.
	stub.mu.Lock()
	stub.errs["m"] = errors.New("opencode-go: status 429 Too Many Requests (sanitized)")
	stub.mu.Unlock()
	before := stub.count("m")
	for i := 0; i < 2; i++ {
		if err := callModel(cbp, "m"); err == nil {
			t.Fatal("a refused call succeeded")
		}
	}
	if ok, open := run(); ok != 0 || open != burst || stub.count("m") != before+2 {
		t.Fatalf("after refusals: %d ok, %d refused at the gate, %d new upstream calls; want %d refused without reaching the provider", ok, open, stub.count("m")-before, burst)
	}

	// The narrowing is logged once, with the refusal that caused it.
	if got := strings.Count(logs.String(), "narrowed by a provider failure"); got != 1 || !strings.Contains(logs.String(), "429") {
		t.Fatalf("narrowing log lines = %d, want one naming the 429:\n%s", got, logs.String())
	}

	// The provider answers again: the probe succeeds and the model opens to
	// the whole burst.
	stub.mu.Lock()
	delete(stub.errs, "m")
	stub.mu.Unlock()
	time.Sleep(150 * time.Millisecond)
	if err := callModel(cbp, "m"); err != nil {
		t.Fatalf("the probe after the open window failed: %v", err)
	}
	if ok, open := run(); ok != burst || open != 0 {
		t.Fatalf("after recovery: %d ok, %d refused; want all %d through", ok, open, burst)
	}
}
