package gateway

import (
	"context"
	"math"
	"path/filepath"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/provider"
)

// Failure modes are listed in the header of paid_budget.go; each test below
// answers one: cost math, UTC-midnight reset, persistence, a typed refusal
// with the midnight reset, under-budget calls passing, free providers never
// refused, the snapshot's fields (including per computer).

type spendClock struct{ t time.Time }

func (c *spendClock) now() time.Time { return c.t }

func newTestSpend(t *testing.T, path string, clk *spendClock) *PaidSpend {
	t.Helper()
	return NewPaidSpend(path, map[string]float64{"bedrock": 20, "river": 1}, clk.now)
}

const riverModel = "deepseek-ai/DeepSeek-V4.1-Flash-1017"

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestPaidSpendCostMath(t *testing.T) {
	clk := &spendClock{time.Date(2026, 11, 9, 10, 0, 0, 0, time.UTC)}
	s := newTestSpend(t, "", clk)
	// River: 1M input of which 500k cached, 100k output at River's 0.12/0.0024/0.48.
	got := s.Record("river", "vm-a", riverModel, provider.Usage{InputTokens: 1_000_000, CachedInputTokens: 500_000, OutputTokens: 100_000})
	want := 0.5*0.12 + 0.5*0.0024 + 0.1*0.48
	if !near(got, want) {
		t.Fatalf("cost %v want %v", got, want)
	}
	// Bedrock Haiku 5.5 at $0.10/$0.50 per M, by a Bedrock-style id.
	got = s.Record("bedrock", "vm-a", "us.anthropic.claude-haiku-5-5-20260101-v1:0", provider.Usage{InputTokens: 1_000_000, OutputTokens: 1_000_000})
	if !near(got, 0.60) {
		t.Fatalf("haiku cost %v want 0.60", got)
	}
	// An unpriced model on a capped provider counts at the dear rate, not $0.
	got = s.Record("bedrock", "vm-a", "us.anthropic.claude-mystery-9", provider.Usage{InputTokens: 1_000_000})
	if !near(got, 5.0) {
		t.Fatalf("unpriced capped cost %v want 5", got)
	}
	// An unpriced model on an uncapped provider is $0 and visibly unpriced.
	s.Record("mystery", "vm-a", "no-such-model", provider.Usage{InputTokens: 1_000_000})
	if p := s.Snapshot().Providers["mystery"]; p.USD != 0 || p.Unpriced != 1 {
		t.Fatalf("uncapped unpriced: %+v", p)
	}
}

func TestPaidSpendResetsAtUTCMidnight(t *testing.T) {
	clk := &spendClock{time.Date(2026, 11, 9, 23, 59, 0, 0, time.UTC)}
	s := newTestSpend(t, "", clk)
	s.Record("river", "vm-a", riverModel, provider.Usage{InputTokens: 10_000_000})
	if s.Refusal("river", riverModel) == nil {
		t.Fatal("river should be spent at $1.20")
	}
	clk.t = time.Date(2026, 11, 10, 0, 0, 1, 0, time.UTC)
	if err := s.Refusal("river", riverModel); err != nil {
		t.Fatalf("new day still refused: %v", err)
	}
	snap := s.Snapshot()
	if snap.Day != "2026-11-10" || snap.Providers["river"].USD != 0 {
		t.Fatalf("today not reset: %+v", snap)
	}
	if snap.PreviousDay != "2026-11-09" || !near(snap.Previous["river"].USD, 1.2) {
		t.Fatalf("previous day lost: %+v", snap)
	}
}

func TestPaidSpendPersistsAcrossRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "spend.json")
	clk := &spendClock{time.Date(2026, 11, 9, 8, 0, 0, 0, time.UTC)}
	s := newTestSpend(t, path, clk)
	s.Record("river", "vm-a", riverModel, provider.Usage{InputTokens: 10_000_000})
	s2 := newTestSpend(t, path, clk)
	if s2.Refusal("river", riverModel) == nil {
		t.Fatal("restart forgot the day's spend")
	}
	if p := s2.Snapshot().Providers["river"]; p.Calls != 1 || p.Computers["vm-a"] < 1.2-1e-9 {
		t.Fatalf("restored: %+v", p)
	}
	// A restart on the next day starts clean and keeps yesterday as previous.
	clk.t = clk.t.Add(24 * time.Hour)
	s3 := newTestSpend(t, path, clk)
	if s3.Refusal("river", riverModel) != nil || s3.Snapshot().PreviousDay != "2026-11-09" {
		t.Fatalf("next-day restart: %+v", s3.Snapshot())
	}
}

func TestPaidSpendRefusalIsTypedQuotaWithMidnightReset(t *testing.T) {
	clk := &spendClock{time.Date(2026, 11, 9, 15, 30, 0, 0, time.UTC)}
	s := newTestSpend(t, "", clk)
	s.Record("bedrock", "vm-a", "us.anthropic.claude-opus-4-6-v1", provider.Usage{InputTokens: 4_000_000}) // $20
	err := s.Refusal("bedrock", "m")
	q, ok := provider.AsQuotaExhausted(err)
	if !ok {
		t.Fatalf("not a typed quota error: %v", err)
	}
	if want := time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC); !q.RetryAt.Equal(want) {
		t.Fatalf("reset %v want %v", q.RetryAt, want)
	}
	if q.Provider != "bedrock" || !q.CircuitOpen {
		t.Fatalf("quota fields: %+v", q)
	}
}

func TestPaidSpendUnderBudgetAndFreeProvidersPass(t *testing.T) {
	clk := &spendClock{time.Date(2026, 11, 9, 15, 30, 0, 0, time.UTC)}
	s := newTestSpend(t, "", clk)
	s.Record("river", "vm-a", riverModel, provider.Usage{InputTokens: 1_000_000}) // $0.60 < $1
	if err := s.Refusal("river", riverModel); err != nil {
		t.Fatalf("under budget refused: %v", err)
	}
	// Synthetic spends far more than any cap and is never refused.
	s.Record("synthetic", "vm-a", riverModel, provider.Usage{InputTokens: 500_000_000})
	if err := s.Refusal("synthetic", riverModel); err != nil {
		t.Fatalf("synthetic refused: %v", err)
	}
	if err := s.Refusal("opencode-go", "x"); err != nil {
		t.Fatalf("free provider refused: %v", err)
	}
}

func TestPaidSpendSnapshotFields(t *testing.T) {
	clk := &spendClock{time.Date(2026, 11, 9, 15, 30, 0, 0, time.UTC)}
	s := newTestSpend(t, "", clk)
	s.Record("river", "vm-a", riverModel, provider.Usage{InputTokens: 100, CachedInputTokens: 40, OutputTokens: 10})
	s.Record("river", "vm-b", riverModel, provider.Usage{InputTokens: 100, OutputTokens: 10})
	snap := s.Snapshot()
	r := snap.Providers["river"]
	if r.Calls != 2 || r.InputTokens != 200 || r.OutputTokens != 20 || r.CachedTokens != 40 || r.BudgetUSD != 1 {
		t.Fatalf("river block: %+v", r)
	}
	if len(r.Computers) != 2 || r.Computers["vm-a"] <= 0 || r.Computers["vm-b"] <= 0 {
		t.Fatalf("per computer: %+v", r.Computers)
	}
	if b, ok := snap.Providers["bedrock"]; !ok || b.BudgetUSD != 20 {
		t.Fatalf("capped provider missing at $0: %+v", snap.Providers)
	}
	if snap.ResetsAt != "2026-11-10T00:00:00Z" {
		t.Fatalf("resets_at %q", snap.ResetsAt)
	}
	if line := s.SummaryLine(); line == "" {
		t.Fatal("empty summary")
	}
}

type spendStub struct {
	name  string
	calls int
	usage provider.Usage
}

func (p *spendStub) Call(ctx context.Context, req provider.LLMRequest) (*provider.LLMResponse, error) {
	p.calls++
	return &provider.LLMResponse{Text: "ok", Model: req.Model, Usage: p.usage}, nil
}
func (p *spendStub) Stream(ctx context.Context, req provider.LLMRequest, on func(provider.StreamChunk)) (*provider.LLMResponse, error) {
	return p.Call(ctx, req)
}
func (p *spendStub) Name() string { return p.name }
func (p *spendStub) IsReal() bool { return true }

func TestPaidSpendProviderWrapsCountsAndRefuses(t *testing.T) {
	clk := &spendClock{time.Date(2026, 11, 9, 15, 30, 0, 0, time.UTC)}
	s := newTestSpend(t, "", clk)
	inner := &spendStub{name: "river", usage: provider.Usage{InputTokens: 10_000_000}}
	w := &paidSpendProvider{inner: inner, spend: s}
	ctx := provider.WithImageDecodeCaller(context.Background(), "vm-z")
	if _, err := w.Call(ctx, provider.LLMRequest{Model: riverModel}); err != nil {
		t.Fatal(err)
	}
	if got := s.Snapshot().Providers["river"].Computers["vm-z"]; got < 1.2-1e-9 {
		t.Fatalf("vm-z charged %v", got)
	}
	_, err := w.Call(ctx, provider.LLMRequest{Model: riverModel})
	if _, ok := provider.AsQuotaExhausted(err); !ok || inner.calls != 1 {
		t.Fatalf("second call should be refused before dispatch: err=%v calls=%d", err, inner.calls)
	}
	if _, err := w.Stream(ctx, provider.LLMRequest{Model: riverModel}, func(provider.StreamChunk) {}); err == nil {
		t.Fatal("stream not refused")
	}
}

func TestPaidSpendEnvBudgets(t *testing.T) {
	env := map[string]string{PaidBudgetBedrockEnv: "7.5", PaidBudgetRiverEnv: "junk"}
	s := newPaidSpendFromGetenv(func(k string) string { return env[k] })
	if s.budgets["bedrock"] != 7.5 || s.budgets["river"] != defaultRiverDailyUSD {
		t.Fatalf("budgets: %+v", s.budgets)
	}
}

// TestPaidSpendInfiniteBudgetIsTheDefault (Devin r1): "inf" parses as a float,
// and must not lift the cap.
func TestPaidSpendInfiniteBudgetIsTheDefault(t *testing.T) {
	env := map[string]string{PaidBudgetBedrockEnv: "inf", PaidBudgetRiverEnv: "+Infinity"}
	s := newPaidSpendFromGetenv(func(k string) string { return env[k] })
	if s.budgets["bedrock"] != defaultBedrockDailyUSD || s.budgets["river"] != defaultRiverDailyUSD {
		t.Fatalf("budgets: %+v", s.budgets)
	}
}

// TestPaidSpendPricesGlobalBedrockProfiles (Devin r1): a global. profile
// prices at its model's rate, not the unpriced rate.
func TestPaidSpendPricesGlobalBedrockProfiles(t *testing.T) {
	if _, priced := paidCallCost("bedrock", "global.anthropic.claude-haiku-5-5", provider.Usage{InputTokens: 1000}, true); !priced {
		t.Fatal("global.anthropic.claude-haiku-5-5 is unpriced")
	}
}

// TestPaidSpendPricesRiverAtRiversRate (owner, 2026-11-09): River serves the
// same DeepSeek snapshot as Synthetic at a fifth of the price, so the River
// budget must count River's rate, not Synthetic's.
func TestPaidSpendPricesRiverAtRiversRate(t *testing.T) {
	usd, priced := paidCallCost("river", "deepseek-ai/DeepSeek-V4.1-Flash-1017", provider.Usage{InputTokens: 1_000_000, OutputTokens: 1_000_000}, true)
	if !priced || usd < 0.599 || usd > 0.601 {
		t.Fatalf("river 1M in + 1M out = $%.4f (priced %v), want $0.60", usd, priced)
	}
	usd, _ = paidCallCost("synthetic", "deepseek-v4.1-flash-1017", provider.Usage{InputTokens: 1_000_000}, false)
	if usd < 0.599 || usd > 0.601 {
		t.Fatalf("synthetic 1M in = $%.4f, want $0.60", usd)
	}
	// Haiku 5.5 cache reads price at the cached rate.
	usd, _ = paidCallCost("bedrock", "us.anthropic.claude-haiku-5-5", provider.Usage{InputTokens: 1_000_000, CachedInputTokens: 1_000_000}, true)
	if usd > 0.02 {
		t.Fatalf("1M cached Haiku tokens = $%.4f, want the cached rate", usd)
	}
}
