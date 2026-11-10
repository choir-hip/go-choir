package gateway

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/provider"
)

// --- PerAutoputerRateLimiter unit tests ---

func TestRateLimiterAllowsUnderLimit(t *testing.T) {
	rl := NewPerAutoputerRateLimiter(5, 1*time.Second)

	for i := 0; i < 5; i++ {
		if !rl.Allow("autoputer-1") {
			t.Fatalf("request %d should be allowed", i+1)
		}
	}
}

func TestRateLimiterBlocksOverLimit(t *testing.T) {
	rl := NewPerAutoputerRateLimiter(3, 1*time.Second)

	for i := 0; i < 3; i++ {
		if !rl.Allow("autoputer-1") {
			t.Fatalf("request %d should be allowed", i+1)
		}
	}

	if rl.Allow("autoputer-1") {
		t.Fatal("request 4 should be blocked (over limit)")
	}
}

func TestRateLimiterIsolationBetweenAutoputeres(t *testing.T) {
	rl := NewPerAutoputerRateLimiter(2, 1*time.Second)

	// autoputer-1 uses its full quota.
	rl.Allow("autoputer-1")
	rl.Allow("autoputer-1")

	// autoputer-1 is now blocked.
	if rl.Allow("autoputer-1") {
		t.Fatal("autoputer-1 should be blocked after using its quota")
	}

	// autoputer-2 is independent and should still succeed.
	if !rl.Allow("autoputer-2") {
		t.Fatal("autoputer-2 should be allowed (independent quota)")
	}
	if !rl.Allow("autoputer-2") {
		t.Fatal("autoputer-2 second request should be allowed")
	}

	// autoputer-2 is now also blocked.
	if rl.Allow("autoputer-2") {
		t.Fatal("autoputer-2 should be blocked after using its quota")
	}

	// autoputer-1 is still blocked (its quota was not freed by autoputer-2).
	if rl.Allow("autoputer-1") {
		t.Fatal("autoputer-1 should still be blocked")
	}
}

func TestRateLimiterConcurrentAccess(t *testing.T) {
	rl := NewPerAutoputerRateLimiter(100, 1*time.Second)

	var wg sync.WaitGroup
	allowed := atomic.Int32{}

	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if rl.Allow("autoputer-1") {
				allowed.Add(1)
			}
		}()
	}
	wg.Wait()

	count := allowed.Load()
	if count != 100 {
		t.Fatalf("expected exactly 100 allowed requests, got %d", count)
	}
}

func TestRateLimiterConcurrentMultiAutoputer(t *testing.T) {
	rl := NewPerAutoputerRateLimiter(50, 1*time.Second)

	var wg sync.WaitGroup
	autoputer1Allowed := atomic.Int32{}
	autoputer2Allowed := atomic.Int32{}

	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			if rl.Allow("autoputer-1") {
				autoputer1Allowed.Add(1)
			}
		}()
		go func() {
			defer wg.Done()
			if rl.Allow("autoputer-2") {
				autoputer2Allowed.Add(1)
			}
		}()
	}
	wg.Wait()

	s1 := autoputer1Allowed.Load()
	s2 := autoputer2Allowed.Load()
	if s1 != 50 {
		t.Fatalf("autoputer-1: expected 50 allowed, got %d", s1)
	}
	if s2 != 50 {
		t.Fatalf("autoputer-2: expected 50 allowed, got %d", s2)
	}
}

func TestRateLimiterWindowReset(t *testing.T) {
	rl := NewPerAutoputerRateLimiter(2, 100*time.Millisecond)

	rl.Allow("autoputer-1")
	rl.Allow("autoputer-1")

	if rl.Allow("autoputer-1") {
		t.Fatal("should be blocked")
	}

	// Wait for the window to pass.
	time.Sleep(150 * time.Millisecond)

	if !rl.Allow("autoputer-1") {
		t.Fatal("should be allowed after window reset")
	}
}

func TestRateLimiterRecordReturnsFalseOverLimit(t *testing.T) {
	rl := NewPerAutoputerRateLimiter(2, 1*time.Second)

	if !rl.Record("autoputer-1") {
		t.Fatal("first record should succeed")
	}
	if !rl.Record("autoputer-1") {
		t.Fatal("second record should succeed")
	}
	if rl.Record("autoputer-1") {
		t.Fatal("third record should fail (over limit)")
	}
}

// --- Helpers ---

// setupHandlerWithRateLimit creates a Handler with a mock provider and
// a per-autoputer rate limiter configured with the given parameters.
func setupHandlerWithRateLimit(t *testing.T, maxReqs int, window time.Duration) (*Handler, *IdentityRegistry, *mockProvider) {
	t.Helper()
	reg := NewIdentityRegistry(1 * time.Hour)
	mp := &mockProvider{
		name: "bedrock",
		real: true,
		response: &provider.LLMResponse{
			ID:           "resp-123",
			Text:         "Hello from Bedrock!",
			Model:        "claude-sonnet",
			StopReason:   "end_turn",
			ProviderName: "bedrock",
			Usage:        provider.Usage{InputTokens: 10, OutputTokens: 20},
		},
	}
	rl := NewPerAutoputerRateLimiter(maxReqs, window)
	return NewHandlerWithRateLimit(reg, mp, rl), reg, mp
}
