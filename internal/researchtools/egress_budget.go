package researchtools

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/yusefmosiah/go-choir/internal/toolregistry"
)

// EgressBudgetLedger is the D2 network-cap boundary for desk-carrier
// research (R3r): every host-mediated network call a research activation
// makes — search, fetch, source import — charges a per-activation budget
// keyed on the run/activation identity. Cells never reach the network
// themselves (the worker allowlist bans net/net-http), so this ledger is
// the single accounting point for all research egress.
//
// The ledger is host-side state owned by the Runtime, not worker memory:
// it survives desk-worker respawn and accumulates across the activation's
// cells — deliberately, because the cap binds an activation's lifecycle,
// not one cell.
type EgressBudgetLedger struct {
	mu sync.Mutex
	// counters are keyed by activation id (RunRecord.RunID / exec ctx RunID).
	counters map[string]*egressCounter
	// MaxCalls bounds total host-mediated network calls per activation;
	// zero means unlimited.
	MaxCalls int64
	// MaxFetchedBytes bounds fetched response bytes per activation; zero
	// means unlimited.
	MaxFetchedBytes int64
}

type egressCounter struct {
	calls   int64
	fetched int64
}

// DefaultResearchEgress* are the R3r cap values: generous enough for real
// research saturation loops, tight enough that a runaway activation
// exhausts calls before it can do harm. One HTTP response is byte-capped
// at the tool layer (256KiB fetch / 4MiB source decode); the byte budget
// caps the activation total.
const (
	DefaultResearchEgressMaxCalls        int64 = 64
	DefaultResearchEgressMaxFetchedBytes int64 = 32 << 20 // 32 MiB
)

// NewEgressBudgetLedger builds a ledger with the given limits. The
// Runtime owns one ledger shared by every desk's research surface.
func NewEgressBudgetLedger(maxCalls, maxFetchedBytes int64) *EgressBudgetLedger {
	return &EgressBudgetLedger{
		counters:        map[string]*egressCounter{},
		MaxCalls:        maxCalls,
		MaxFetchedBytes: maxFetchedBytes,
	}
}

// chargeCall admits one host-mediated network call for the activation in
// execCtx, or refuses with a budget error.
func (l *EgressBudgetLedger) chargeCall(execCtx toolregistry.ExecutionContext, tool string) error {
	if l == nil {
		return nil
	}
	key := egressKey(execCtx)
	l.mu.Lock()
	defer l.mu.Unlock()
	c := l.counters[key]
	if c == nil {
		c = &egressCounter{}
		l.counters[key] = c
	}
	if l.MaxCalls > 0 && c.calls >= l.MaxCalls {
		return fmt.Errorf("%s: activation egress budget exhausted (%d/%d network calls this activation)", tool, c.calls, l.MaxCalls)
	}
	c.calls++
	return nil
}

// chargeBytes admits fetched response bytes under the activation's byte
// budget. Metered tools call it with the measured body length.
func (l *EgressBudgetLedger) chargeBytes(execCtx toolregistry.ExecutionContext, tool string, n int) error {
	if l == nil || n <= 0 {
		return nil
	}
	key := egressKey(execCtx)
	l.mu.Lock()
	defer l.mu.Unlock()
	c := l.counters[key]
	if c == nil {
		c = &egressCounter{}
		l.counters[key] = c
	}
	if l.MaxFetchedBytes > 0 && c.fetched+int64(n) > l.MaxFetchedBytes {
		return fmt.Errorf("%s: activation egress byte budget exceeded (%d+%d > %d fetched bytes this activation)", tool, c.fetched, n, l.MaxFetchedBytes)
	}
	c.fetched += int64(n)
	return nil
}

// Usage reports an activation's charged counters (tests and diagnostics).
func (l *EgressBudgetLedger) Usage(execCtx toolregistry.ExecutionContext) (calls, fetchedBytes int64) {
	if l == nil {
		return 0, 0
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if c := l.counters[egressKey(execCtx)]; c != nil {
		return c.calls, c.fetched
	}
	return 0, 0
}

// egressKey is the activation identity the ledger charges — the same RunID
// a desk worker binds to, so the budget survives worker respawn.
func egressKey(execCtx toolregistry.ExecutionContext) string {
	if execCtx.RunRecord != nil {
		if id := strings.TrimSpace(execCtx.RunRecord.RunID); id != "" {
			return id
		}
	}
	if id := strings.TrimSpace(execCtx.RunID); id != "" {
		return id
	}
	return "activation:unknown"
}

// chargeEgressCall admits one network call for the activation in ctx under
// the ledger — the single pre-call gate every host-mediated network tool
// runs first.
func (d Dependencies) chargeEgressCall(ctx context.Context, tool string) error {
	if d.Egress == nil {
		return nil
	}
	return d.Egress.chargeCall(toolregistry.ExecutionContextFrom(ctx), tool)
}

// chargeEgressBytes admits fetched response bytes under the ledger —
// the metered-post-body gate for tools that read a response body.
func (d Dependencies) chargeEgressBytes(ctx context.Context, tool string, n int) error {
	if d.Egress == nil {
		return nil
	}
	return d.Egress.chargeBytes(toolregistry.ExecutionContextFrom(ctx), tool, n)
}
