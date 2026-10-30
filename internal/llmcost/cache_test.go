package llmcost

import (
	"encoding/json"
	"math"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// Prompt-cache measurement (docs/archive/reports/prompt-cache-analysis-2026-10-14.md).
//
// Failure modes pinned here:
//   - cached tokens are priced at the full input rate (overstates cost up to
//     50x on DeepSeek V4.1 Flash);
//   - cached tokens are priced twice (once as input, once as cached);
//   - a model with no cached rate prices cached tokens at $0 (understates);
//   - an unpriced model becomes "found" because cached tokens were present;
//   - events written before this field existed lose their cost or count as
//     a 0% cache hit, dragging the measured hit rate down;
//   - the hit rate divides by zero when no call reported caching;
//   - breakdowns (by model, provider, run, day) drop the cached counts.

func approx(a, b float64) bool { return math.Abs(a-b) < 1e-12 }

func TestCachedInputPricesInUse(t *testing.T) {
	t.Parallel()
	cases := []struct {
		model  string
		cached float64
	}{
		{"deepseek-v4.1-flash", 0.006},
		{"deepseek-v4-pro", 0.044},
		{"gpt-6-luna", 0.01},
		{"kimi-k2p6", 0.16},
	}
	for _, tc := range cases {
		e, ok := LookupPricing(tc.model)
		if !ok || e.CachedInputPerMillion != tc.cached {
			t.Fatalf("LookupPricing(%q) cached = %v (found=%v), want %v", tc.model, e.CachedInputPerMillion, ok, tc.cached)
		}
	}
}

func TestEstimateCallWithCachePricesCachedAtCachedRate(t *testing.T) {
	t.Parallel()
	// DeepSeek V4.1 Flash: 0.30 input, 0.006 cached, 1.20 output per 1M.
	// 1M prompt tokens of which 900K cached, 10K output:
	// 100K*0.30 + 900K*0.006 + 10K*1.20 = 0.03 + 0.0054 + 0.012 = 0.0474.
	c := EstimateCallWithCache("deepseek-v4.1-flash", 1_000_000, 900_000, 10_000)
	if !c.Found || !approx(c.USD, 0.0474) {
		t.Fatalf("cost = %+v, want found 0.0474", c)
	}
	if c.CachedInputTokens != 900_000 || c.InputTokens != 1_000_000 {
		t.Fatalf("tokens = %+v", c)
	}
	// Savings against the uncached price: 900K*(0.30-0.006) = 0.2646.
	if !approx(c.CacheSavingsUSD, 0.2646) {
		t.Fatalf("savings = %v, want 0.2646", c.CacheSavingsUSD)
	}
	// Zero cached equals the old estimator exactly.
	if a, b := EstimateCallWithCache("gpt-6-luna", 5000, 0, 100), EstimateCall("gpt-6-luna", 5000, 100); !approx(a.USD, b.USD) {
		t.Fatalf("cached=0 %v != EstimateCall %v", a.USD, b.USD)
	}
}

func TestEstimateCallWithCacheNoCachedRateUsesInputRate(t *testing.T) {
	t.Parallel()
	// jev-1.13 lists no cached rate: cached tokens bill at the input rate.
	c := EstimateCallWithCache("jev-1.13", 1_000_000, 500_000, 0)
	if !approx(c.USD, 0.042) || c.CacheSavingsUSD != 0 {
		t.Fatalf("jev cost = %+v, want 0.042 and no savings", c)
	}
	// Unpriced stays unpriced.
	u := EstimateCallWithCache("unknown-model", 1000, 900, 10)
	if u.Found || u.USD != 0 || u.CachedInputTokens != 900 {
		t.Fatalf("unpriced = %+v", u)
	}
	// Cached above input is clamped.
	k := EstimateCallWithCache("gpt-6-luna", 100, 5000, 0)
	if k.CachedInputTokens != 100 {
		t.Fatalf("clamp = %+v", k)
	}
}

func cacheEvent(id, model string, in, cached, out int, reported bool, withField bool) types.EventRecord {
	p := map[string]any{
		"phase": "tool_loop", "llm_model": model, "llm_provider": "opencode-go",
		"input_tokens": in, "output_tokens": out,
	}
	if withField {
		p["cached_input_tokens"] = cached
		p["cache_reported"] = reported
	}
	raw, _ := json.Marshal(p)
	return types.EventRecord{EventID: id, RunID: "run-1", Kind: types.EventRunProgress, Payload: raw, Timestamp: time.Unix(1700000000, 0)}
}

func TestExtractAndAggregateCachedTokens(t *testing.T) {
	t.Parallel()
	events := []types.EventRecord{
		cacheEvent("e1", "deepseek-v4.1-flash", 10_000, 9_000, 100, true, true),
		cacheEvent("e2", "deepseek-v4.1-flash", 10_000, 0, 100, true, true),     // reported miss
		cacheEvent("e3", "deepseek-v4.1-flash", 10_000, 0, 100, false, false),   // old event, no field
		cacheEvent("e4", "longcat-2.5-preview-free", 4_000, 0, 10, false, true), // provider did not report
	}
	entries := ExtractCostEntries(events)
	if len(entries) != 4 {
		t.Fatalf("entries = %d", len(entries))
	}
	if entries[0].CachedInputTokens != 9_000 || !entries[0].CacheReported {
		t.Fatalf("entry 0 = %+v", entries[0])
	}
	if entries[2].CacheReported || entries[3].CacheReported {
		t.Fatalf("old/unreported entries marked reported: %+v %+v", entries[2], entries[3])
	}
	// e1 priced with cache: 1K*0.30 + 9K*0.006 + 100*1.20 per 1M.
	want := (1_000*0.30 + 9_000*0.006 + 100*1.20) / 1e6
	if !approx(entries[0].Cost.USD, want) {
		t.Fatalf("entry 0 cost = %v, want %v", entries[0].Cost.USD, want)
	}

	s := Aggregate(entries)
	if s.TotalInputTokens != 34_000 || s.TotalCachedInputTokens != 9_000 {
		t.Fatalf("totals = in %d cached %d", s.TotalInputTokens, s.TotalCachedInputTokens)
	}
	if s.CacheReportedCallCount != 2 || s.CacheReportedInputTokens != 20_000 {
		t.Fatalf("reported = calls %d input %d", s.CacheReportedCallCount, s.CacheReportedInputTokens)
	}
	if !approx(s.CacheHitRate, 0.45) {
		t.Fatalf("hit rate = %v, want 0.45 (9K of 20K reported)", s.CacheHitRate)
	}
	if !approx(s.CacheSavingsUSD, 9_000*(0.30-0.006)/1e6) {
		t.Fatalf("savings = %v", s.CacheSavingsUSD)
	}
	m := s.ByModel["deepseek-v4.1-flash"]
	if m.TotalCachedInputTokens != 9_000 || !approx(m.CacheHitRate, 0.45) {
		t.Fatalf("by model = %+v", m)
	}
	if l := s.ByModel["longcat-2.5-preview-free"]; l.CacheHitRate != 0 || l.CacheReportedCallCount != 0 {
		t.Fatalf("unreported model bucket = %+v", l)
	}
	if r := s.ByRun["run-1"]; r.TotalCachedInputTokens != 9_000 {
		t.Fatalf("by run = %+v", r)
	}
}

func TestAggregateNoReportedCallsHasZeroHitRate(t *testing.T) {
	t.Parallel()
	s := Aggregate([]CostEntry{{Model: "gpt-4o", InputTokens: 100, Cost: EstimateCall("gpt-4o", 100, 0)}})
	if s.CacheHitRate != 0 || math.IsNaN(s.CacheHitRate) {
		t.Fatalf("hit rate = %v", s.CacheHitRate)
	}
	if s := Aggregate(nil); s.CacheHitRate != 0 {
		t.Fatalf("empty hit rate = %v", s.CacheHitRate)
	}
}

func TestExtractExecutionEventCachedTokens(t *testing.T) {
	t.Parallel()
	raw, _ := json.Marshal(map[string]string{
		"phase": "execution", "status": "completed", "provider": "chatgpt", "model": "gpt-6-luna",
		"tokens_in": "5000", "tokens_out": "20", "tokens_cached": "4096", "cache_reported": "true",
	})
	entries := ExtractCostEntries([]types.EventRecord{{EventID: "x", Kind: types.EventRunProgress, Payload: raw}})
	if len(entries) != 1 || entries[0].CachedInputTokens != 4096 || !entries[0].CacheReported {
		t.Fatalf("entries = %+v", entries)
	}
}
