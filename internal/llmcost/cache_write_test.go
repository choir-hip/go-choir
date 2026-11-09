package llmcost

import (
	"math"
	"testing"
)

// A priced call with known usage equals the hand-computed figure. Haiku 5.5
// on Bedrock: input $0.10/M, read $0.01/M, 5-minute write $0.125/M (1.25x),
// 1-hour write $0.20/M (2x), output $0.50/M.
//
// Failure modes pinned here: cache writes priced as plain input (the $20
// Bedrock budget undercounts); the 5m and 1h rates swapped; written tokens
// double-counted as plain input; a model with no write premium priced at $0.
func TestCacheWritesArePriced(t *testing.T) {
	// 100,000-token prompt: 60,000 read, 10,000 written 5m, 20,000 written
	// 1h, 10,000 plain; 2,000 out.
	got := EstimateProviderCallWithCacheWrites("bedrock", "global.anthropic.claude-haiku-5-5", 100_000, 60_000, 10_000, 20_000, 2_000)
	want := 10_000*0.10/1e6 + 60_000*0.01/1e6 + 10_000*0.125/1e6 + 20_000*0.20/1e6 + 2_000*0.50/1e6
	if !got.Found || math.Abs(got.USD-want) > 1e-12 || math.Abs(want-0.00785) > 1e-12 {
		t.Fatalf("usd = %v (found %v), want %v (= $0.00785)", got.USD, got.Found, want)
	}
	// The same call with writes ignored (the old figure) is lower.
	old := EstimateProviderCallWithCache("bedrock", "global.anthropic.claude-haiku-5-5", 100_000, 60_000, 2_000)
	if !(old.USD < got.USD) {
		t.Fatalf("old %v should undercount %v", old.USD, got.USD)
	}
	// A model with no write rate prices written tokens as plain input.
	plain := EstimateCallWithCacheWrites("deepseek-v4.1-flash", 1_000_000, 0, 0, 500_000, 0)
	if math.Abs(plain.USD-0.30) > 1e-12 {
		t.Fatalf("no-premium model usd = %v, want 0.30", plain.USD)
	}
	// Writes beyond the prompt are clamped, never negative plain input.
	clamped := EstimateProviderCallWithCacheWrites("bedrock", "claude-haiku-5.5", 1_000, 0, 5_000, 5_000, 0)
	if math.Abs(clamped.USD-1_000*0.125/1e6) > 1e-12 {
		t.Fatalf("clamped usd = %v", clamped.USD)
	}
}
