package runtime

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestParseModelPolicyResolvesRoles(t *testing.T) {
	raw := `
[defaults]
fallback_provider = "chatgpt"
fallback_model = "gpt-5.5"
reasoning = "low"

[roles.super]
provider = "chatgpt"
model = "gpt-5.5"
reasoning = "medium"

[roles.vtext]
provider = "fireworks"
model = "accounts/fireworks/models/deepseek-v4-flash"
`

	policy, err := parseModelPolicy(raw, "/System/model-policy.toml")
	if err != nil {
		t.Fatalf("parseModelPolicy: %v", err)
	}
	super := policy.Resolve(AgentProfileSuper)
	if super.Provider != "chatgpt" || super.Model != "gpt-5.5" || super.ReasoningEffort != "medium" {
		t.Fatalf("super selection = %+v", super)
	}
	vtext := policy.Resolve(AgentProfileVText)
	if vtext.Provider != "fireworks" || vtext.Model != "accounts/fireworks/models/deepseek-v4-flash" {
		t.Fatalf("vtext selection = %+v", vtext)
	}
	researcher := policy.Resolve(AgentProfileResearcher)
	if researcher.Provider != "chatgpt" || researcher.Model != "gpt-5.5" || researcher.ReasoningEffort != "low" {
		t.Fatalf("researcher fallback = %+v", researcher)
	}
}

func TestRuntimeResolvesModelPolicyIntoRunMetadata(t *testing.T) {
	rt, _ := testRuntime(t)
	policyPath := filepath.Join(t.TempDir(), "System", "model-policy.toml")
	if err := os.MkdirAll(filepath.Dir(policyPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(policyPath, []byte(`
[defaults]
fallback_provider = "chatgpt"
fallback_model = "gpt-5.5"

[roles.researcher]
provider = "fireworks"
model = "accounts/fireworks/models/deepseek-v4-flash"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	rt.cfg.ModelPolicyPath = policyPath

	rec, err := rt.createRunWithMetadata(context.Background(), "research", "user-alice", map[string]any{
		runMetadataAgentProfile: AgentProfileResearcher,
		runMetadataAgentRole:    AgentProfileResearcher,
	})
	if err != nil {
		t.Fatalf("createRunWithMetadata: %v", err)
	}

	if got := metadataStringValue(rec.Metadata, runMetadataLLMProvider); got != "fireworks" {
		t.Fatalf("llm_provider = %q, want fireworks", got)
	}
	if got := metadataStringValue(rec.Metadata, runMetadataLLMModel); got != "accounts/fireworks/models/deepseek-v4-flash" {
		t.Fatalf("llm_model = %q", got)
	}
	if got := metadataStringValue(rec.Metadata, runMetadataLLMPolicySource); got != policyPath {
		t.Fatalf("llm_policy_source = %q, want %q", got, policyPath)
	}
}

func TestRuntimeFallsBackToPreviousValidModelPolicy(t *testing.T) {
	rt, _ := testRuntime(t)
	policyPath := filepath.Join(t.TempDir(), "System", "model-policy.toml")
	if err := os.MkdirAll(filepath.Dir(policyPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(policyPath, []byte(`
[defaults]
fallback_provider = "chatgpt"
fallback_model = "gpt-5.5"

[roles.super]
provider = "chatgpt"
model = "gpt-5.5"
reasoning = "medium"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	rt.cfg.ModelPolicyPath = policyPath
	if _, err := rt.loadModelPolicy(context.Background(), "user-alice"); err != nil {
		t.Fatalf("load valid policy: %v", err)
	}

	if err := os.WriteFile(policyPath, []byte(`this is not a policy assignment`), 0o644); err != nil {
		t.Fatal(err)
	}
	policy, err := rt.loadModelPolicy(context.Background(), "user-alice")
	if err == nil {
		t.Fatalf("expected invalid policy warning")
	}
	super := policy.Resolve(AgentProfileSuper)
	if super.Provider != "chatgpt" || super.Model != "gpt-5.5" || super.ReasoningEffort != "medium" {
		t.Fatalf("cached super policy = %+v", super)
	}
}
