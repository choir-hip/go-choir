package gateway

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/health"
	"github.com/yusefmosiah/go-choir/internal/provider"
)

// One computer's rejected requests must not open the shared provider for
// every computer (problems/inference-breaker-trips-on-client-errors-2026-10-10.md).
func TestCircuitBreakingProviderCountsOnlyProviderSideFailures(t *testing.T) {
	cases := []struct {
		name     string
		err      error
		cancel   bool
		wantOpen bool
	}{
		{name: "400 request rejected", err: errors.New("opencode-go: status 400 Bad Request (sanitized)"), wantOpen: false},
		{name: "413 request too large", err: errors.New("zai: status 413 Payload Too Large (sanitized)"), wantOpen: false},
		{name: "caller cancelled", err: context.Canceled, cancel: true, wantOpen: false},
		{name: "500 provider error", err: errors.New("opencode-go: status 500 Internal Server Error (sanitized)"), wantOpen: true},
		{name: "503 unavailable", err: errors.New("bedrock: status 503 Service Unavailable (sanitized)"), wantOpen: true},
		{name: "429 rate limited", err: errors.New("opencode-go: status 429 Too Many Requests (sanitized)"), wantOpen: true},
		{name: "408 provider timeout", err: errors.New("opencode-go: status 408 Request Timeout (sanitized)"), wantOpen: true},
		{name: "transport error", err: errors.New("opencode-go: http call: connection reset by peer"), wantOpen: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := &stubProvider{name: "stub", real: true, callFn: func(ctx context.Context, req provider.LLMRequest) (*provider.LLMResponse, error) {
				return nil, tc.err
			}}
			cbp := NewCircuitBreakingProvider(p, health.BreakerConfig{FailureThreshold: 2, OpenTimeout: time.Hour})
			for i := 0; i < 3; i++ {
				ctx := context.Background()
				var cancel context.CancelFunc
				if tc.cancel {
					ctx, cancel = context.WithCancel(ctx)
					cancel()
				}
				_, err := cbp.Call(ctx, provider.LLMRequest{})
				if !tc.wantOpen && !errors.Is(err, tc.err) && err.Error() != tc.err.Error() {
					t.Fatalf("call %d: error = %v, want the provider's own error passed through", i, err)
				}
			}
			if open := cbp.Breaker().State() == health.StateOpen; open != tc.wantOpen {
				t.Fatalf("breaker open = %v, want %v", open, tc.wantOpen)
			}
		})
	}
}
