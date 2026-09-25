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
