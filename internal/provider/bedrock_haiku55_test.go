package provider

import (
	"encoding/json"
	"strings"
	"testing"
)

// Claude Haiku 5.5 on Bedrock (owner 2026-11-09). Ways it could fail: effort
// not sent or not mapped; budget_tokens, sampling or thinking sent (400s);
// an older Bedrock model gaining output_config; the provider not resolving
// with a token and no AWS_REGION.
func TestBedrockHaiku55RequestBody(t *testing.T) {
	p, err := NewBedrockProvider(BedrockConfig{Region: "us-east-1", ModelID: "us.anthropic.claude-haiku-5-5", AuthToken: "tok"})
	if err != nil {
		t.Fatal(err)
	}
	for effort, want := range map[string]string{"": "low", "low": "low", "minimal": "low", "medium": "medium", "high": "high"} {
		raw, _ := json.Marshal(p.buildRequestBody(LLMRequest{Model: "us.anthropic.claude-haiku-5-5", ReasoningEffort: effort}))
		s := string(raw)
		if !strings.Contains(s, `"output_config":{"effort":"`+want+`"}`) {
			t.Fatalf("effort %q: body %s lacks effort %s", effort, s, want)
		}
		for _, banned := range []string{"budget_tokens", "temperature", "top_p", "top_k", `"thinking"`} {
			if strings.Contains(s, banned) {
				t.Fatalf("body carries %s: %s", banned, s)
			}
		}
	}
	old, _ := NewBedrockProvider(BedrockConfig{Region: "us-east-1", ModelID: "us.anthropic.claude-sonnet-4-6", AuthToken: "tok"})
	raw, _ := json.Marshal(old.buildRequestBody(LLMRequest{ReasoningEffort: "low"}))
	if strings.Contains(string(raw), "output_config") {
		t.Fatalf("older model got output_config: %s", raw)
	}
}

func TestBedrockResolvesWithTokenAndNoRegion(t *testing.T) {
	t.Setenv("AWS_REGION", "")
	t.Setenv("AWS_BEARER_TOKEN_BEDROCK", "tok-not-a-real-secret")
	p, err := NewBedrockProviderFromEnv("us.anthropic.claude-haiku-5-5")
	if err != nil || p.region != "us-east-1" {
		t.Fatalf("p=%v err=%v", p, err)
	}
	t.Setenv("AWS_REGION", "us-west-2")
	if p, _ = NewBedrockProviderFromEnv("x"); p.region != "us-west-2" {
		t.Fatalf("region = %s", p.region)
	}
}
