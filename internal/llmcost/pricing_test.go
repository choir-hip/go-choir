package llmcost

import (
	"math"
	"testing"
)

func TestLookupPricingExact(t *testing.T) {
	t.Parallel()
	cases := []struct {
		model    string
		provider string
	}{
		{"gpt-4", "openai"},
		{"gpt-4o", "openai"},
		{"gpt-4o-mini", "openai"},
		{"claude-3.5-sonnet", "anthropic"},
		{"claude-3-opus", "anthropic"},
		{"claude-3-haiku", "anthropic"},
	}
	for _, tc := range cases {
		entry, ok := LookupPricing(tc.model)
		if !ok {
			t.Fatalf("LookupPricing(%q): expected match, got not found", tc.model)
		}
		if entry.Provider != tc.provider {
			t.Fatalf("LookupPricing(%q) provider: got %q, want %q", tc.model, entry.Provider, tc.provider)
		}
		if entry.InputPerMillion <= 0 || entry.OutputPerMillion <= 0 {
			t.Fatalf("LookupPricing(%q): expected positive pricing, got in=%v out=%v", tc.model, entry.InputPerMillion, entry.OutputPerMillion)
		}
	}
}

func TestLookupPricingDatedSnapshot(t *testing.T) {
	t.Parallel()
	// Dated OpenAI snapshots should resolve to the base pricing entry.
	cases := []struct {
		query    string
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

func TestEstimateCallGPT4(t *testing.T) {
	t.Parallel()
	// GPT-4: $30/1M input, $60/1M output.
	cost := EstimateCall("gpt-4", 1_000_000, 1_000_000)
	if !cost.Found {
		t.Fatal("gpt-4 estimate should be found")
	}
	if math.Abs(cost.USD-90.0) > 0.0001 {
		t.Fatalf("gpt-4 1M/1M cost: got %.4f, want 90.0", cost.USD)
	}
	if cost.Provider != "openai" {
		t.Fatalf("gpt-4 provider: got %q, want openai", cost.Provider)
	}
}

func TestEstimateCallGPT4o(t *testing.T) {
	t.Parallel()
	// GPT-4o: $5/1M input, $15/1M output.
	cost := EstimateCall("gpt-4o", 1_000_000, 1_000_000)
	if !cost.Found {
		t.Fatal("gpt-4o estimate should be found")
	}
	if math.Abs(cost.USD-20.0) > 0.0001 {
		t.Fatalf("gpt-4o 1M/1M cost: got %.4f, want 20.0", cost.USD)
	}
}

func TestEstimateCallClaude35Sonnet(t *testing.T) {
	t.Parallel()
	// Claude 3.5 Sonnet: $3/1M input, $15/1M output.
	cost := EstimateCall("claude-3.5-sonnet", 1_000_000, 1_000_000)
	if !cost.Found {
		t.Fatal("claude-3.5-sonnet estimate should be found")
	}
	if math.Abs(cost.USD-18.0) > 0.0001 {
		t.Fatalf("claude-3.5-sonnet 1M/1M cost: got %.4f, want 18.0", cost.USD)
	}
	if cost.Provider != "anthropic" {
		t.Fatalf("claude-3.5-sonnet provider: got %q, want anthropic", cost.Provider)
	}
}

func TestEstimateCallClaude3Opus(t *testing.T) {
	t.Parallel()
	// Claude 3 Opus: $15/1M input, $75/1M output.
	cost := EstimateCall("claude-3-opus", 1_000_000, 1_000_000)
	if !cost.Found {
		t.Fatal("claude-3-opus estimate should be found")
	}
	if math.Abs(cost.USD-90.0) > 0.0001 {
		t.Fatalf("claude-3-opus 1M/1M cost: got %.4f, want 90.0", cost.USD)
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

func TestEstimateCallSmallTokenCount(t *testing.T) {
	t.Parallel()
	// GPT-4o: $5/1M input, $15/1M output.
	// 1000 input + 500 output = 0.005 + 0.0075 = 0.0125
	cost := EstimateCall("gpt-4o", 1000, 500)
	if !cost.Found {
		t.Fatal("gpt-4o estimate should be found")
	}
	if math.Abs(cost.USD-0.0125) > 0.0000001 {
		t.Fatalf("gpt-4o 1000/500 cost: got %.8f, want 0.0125", cost.USD)
	}
}

func TestCostAdd(t *testing.T) {
	t.Parallel()
	a := EstimateCall("gpt-4o", 1000, 500)
	b := EstimateCall("gpt-4o", 2000, 1000)
	a.Add(b)
	if a.InputTokens != 3000 || a.OutputTokens != 1500 {
		t.Fatalf("Add tokens: got in=%d out=%d, want 3000/1500", a.InputTokens, a.OutputTokens)
	}
	want := 0.0125 + 0.025
	if math.Abs(a.USD-want) > 0.0000001 {
		t.Fatalf("Add USD: got %.8f, want %.8f", a.USD, want)
	}
	if !a.Found {
		t.Fatal("Add of two found costs should remain found")
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

func TestKnownModelsCoversRequired(t *testing.T) {
	t.Parallel()
	models := KnownModels()
	want := []string{"gpt-4", "gpt-4o", "claude-3.5-sonnet", "claude-3-opus"}
	have := make(map[string]bool, len(models))
	for _, m := range models {
		have[m.Model] = true
	}
	for _, w := range want {
		if !have[w] {
			t.Fatalf("KnownModels: required model %q not present", w)
		}
	}
}
