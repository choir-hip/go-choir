package provideriface

import "testing"

// Run records stored before the vocabulary infix rule was narrowed hold
// model ids it rewrote ("-super-" to "-management-",
// problems/vocab-infix-rewrite-mangles-model-ids-2026-11-05.md). Reading
// them back must name the catalog model, not the mangled one. Failure modes
// pinned: the primary or a fallback keeps the mangled id; a catalogued id is
// changed; an uncatalogued id with no catalogued inverse is changed.
func TestResolvedLLMConfigRepairsVocabularyMangledModelIDs(t *testing.T) {
	metadata := map[string]any{
		"llm_provider":  "openrouter",
		"llm_model":     "nvidia/nemotron-3.5-management-vl:free",
		"llm_fallbacks": "openrouter/nvidia/nemotron-3.5-management-vl:free,opencode-go/deepseek-v4.1-flash",
	}
	sel := ResolvedLLMConfigFromMetadata(metadata)
	if sel.Model != "nvidia/nemotron-3.5-super-vl:free" {
		t.Fatalf("primary model = %q, want the catalog id", sel.Model)
	}
	fallbacks := FallbackLLMConfigsFromMetadata(metadata, sel)
	if len(fallbacks) != 2 || fallbacks[0].Model != "nvidia/nemotron-3.5-super-vl:free" || fallbacks[1].Model != "deepseek-v4.1-flash" {
		t.Fatalf("fallbacks = %+v", fallbacks)
	}
	for _, tc := range []struct{ provider, model string }{
		{"openrouter", "nvidia/nemotron-3.5-super-vl:free"},
		{"opencode-go", "deepseek-v4.1-flash"},
		{"openrouter", "vendor/unknown-management-model"},
		{"not-catalogued", "x-management-y"},
	} {
		got := ResolvedLLMConfigFromMetadata(map[string]any{"llm_provider": tc.provider, "llm_model": tc.model})
		if got.Model != tc.model {
			t.Errorf("model %s/%s rewritten to %q", tc.provider, tc.model, got.Model)
		}
	}
}
