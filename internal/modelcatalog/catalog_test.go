package modelcatalog

import "testing"

func TestContextWindowTokensForNewProviderModels(t *testing.T) {
	for _, modelID := range []string{
		"deepseek-v4-pro",
		"deepseek-v4-flash",
		"mimo-v2.5-pro",
		"mimo-v2.5",
	} {
		if got := ContextWindowTokensForModel(modelID); got != 1_000_000 {
			t.Fatalf("%s context window = %d, want 1000000", modelID, got)
		}
	}
	if got := ContextWindowTokensForModel("unknown-model"); got != DefaultContextWindowTokens {
		t.Fatalf("unknown model context window = %d, want %d", got, DefaultContextWindowTokens)
	}
}
