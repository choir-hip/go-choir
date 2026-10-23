package llmcost

import "testing"

func TestLookupPricingDatedSnapshot(t *testing.T) {
	t.Parallel()
	// Dated OpenAI snapshots should resolve to the base pricing entry.
	cases := []struct {
		query     string
		baseModel string
	}{
		{"gpt-4-0613", "gpt-4"},
		{"gpt-4o-2024-08-06", "gpt-4o"},
		{"gpt-4o-mini-2024-07-18", "gpt-4o-mini"},
		{"claude-3-5-sonnet-20240620", "claude-3.5-sonnet"},
		{"claude-3-opus-20240229", "claude-3-opus"},
	}
	for _, tc := range cases {
		entry, ok := LookupPricing(tc.query)
		if !ok {
			t.Fatalf("LookupPricing(%q): expected match for dated snapshot, got not found", tc.query)
		}
		if entry.Model != tc.baseModel {
			t.Fatalf("LookupPricing(%q) model: got %q, want %q", tc.query, entry.Model, tc.baseModel)
		}
	}
}

func TestLookupPricingUnknown(t *testing.T) {
	t.Parallel()
	_, ok := LookupPricing("some-unknown-model-xyz")
	if ok {
		t.Fatal("LookupPricing for unknown model should return false")
	}
	_, ok = LookupPricing("")
	if ok {
		t.Fatal("LookupPricing for empty model should return false")
	}
}
func TestEstimateCallUnknownModel(t *testing.T) {
	t.Parallel()
	cost := EstimateCall("unknown-model", 1000, 2000)
	if cost.Found {
		t.Fatal("unknown model should not be found")
	}
	if cost.USD != 0 {
		t.Fatalf("unknown model cost: got %.4f, want 0", cost.USD)
	}
	if cost.InputTokens != 1000 || cost.OutputTokens != 2000 {
		t.Fatalf("unknown model tokens: got in=%d out=%d, want 1000/2000", cost.InputTokens, cost.OutputTokens)
	}
}
func TestCostAddUnfound(t *testing.T) {
	t.Parallel()
	a := EstimateCall("gpt-4o", 1000, 500)
	b := EstimateCall("unknown", 1000, 500)
	a.Add(b)
	if a.Found {
		t.Fatal("Add with unfound cost should produce unfound result")
	}
}

// The models Choir calls in 2026-10 resolve to their researched prices
// (docs/evidence/model-prices-2026-10-14.md). Free routes are found at $0,
// not reported as unpriced.
func TestLookupPricingModelsInUse(t *testing.T) {
	t.Parallel()
	cases := []struct {
		query   string
		in, out float64
	}{
		{"deepseek-v4.1-flash", 0.30, 1.20},
		{"gpt-6-luna", 0.10, 0.50},
		{"longcat-2.5-preview-free", 0, 0},
		{"cat-jam-free", 0, 0},
		{"typesafe/jev-1.13", 0.042, 0},
		{"jev-1.13", 0.042, 0},
		{"typesafe/jev-1.14-20261014", 0.042, 0},
		{"~typesafe/jev-latest", 0.042, 0},
		{"deepseek-v4-pro", 1.32, 3.96},
		{"kimi-k2p6", 0.95, 4.00},
	}
	for _, tc := range cases {
		entry, ok := LookupPricing(tc.query)
		if !ok {
			t.Fatalf("LookupPricing(%q): not found", tc.query)
		}
		if entry.InputPerMillion != tc.in || entry.OutputPerMillion != tc.out {
			t.Fatalf("LookupPricing(%q): got %v/%v, want %v/%v", tc.query, entry.InputPerMillion, entry.OutputPerMillion, tc.in, tc.out)
		}
	}
	// gpt-6-luna must not fall to an older gpt entry, nor v4.1-flash to v4-pro.
	if e, _ := LookupPricing("gpt-6-luna"); e.Model != "gpt-6-luna" {
		t.Fatalf("gpt-6-luna resolved to %q", e.Model)
	}
}

// Synthetic's 1017 model is priced at Synthetic's rate, not by prefix at
// DeepSeek's own (0.30 in, 0.006 cached), which understated input by half.
func TestSyntheticDeepSeek1017PricedAtSyntheticRate(t *testing.T) {
	entry, ok := LookupPricing("hf:deepseek-ai/DeepSeek-V4.1-Flash-1017")
	if !ok || entry.Provider != "synthetic" || entry.InputPerMillion != 0.60 || entry.CachedInputPerMillion != 0.03 || entry.OutputPerMillion != 1.20 {
		t.Fatalf("pricing = %+v (found=%v)", entry, ok)
	}
}
