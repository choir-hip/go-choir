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

func TestMaxOutputTokensForSelectionUsesModelCatalog(t *testing.T) {
	if got := MaxOutputTokensForSelection(LLMSelection{Model: "accounts/fireworks/models/deepseek-v4-flash"}); got != 131072 {
		t.Fatalf("deepseek flash max tokens = %d, want 131072", got)
	}
	if got := MaxOutputTokensForSelection(LLMSelection{Model: "gpt-5.5"}); got != 65536 {
		t.Fatalf("gpt-5.5 max tokens = %d, want 65536", got)
	}
	if got := MaxOutputTokensForSelection(LLMSelection{Model: "unknown-model"}); got != 65536 {
		t.Fatalf("unknown model max tokens = %d, want safe default 65536", got)
	}
}

func TestFallbackModelPolicyKeepsForegroundRolesOffChatGPT(t *testing.T) {
	policy := fallbackModelPolicy(Config{})
	conductor := policy.Resolve(AgentProfileConductor)
	if conductor.Provider != "fireworks" || conductor.Model != "accounts/fireworks/models/deepseek-v4-flash" || conductor.ReasoningEffort != "low" {
		t.Fatalf("conductor selection = %+v", conductor)
	}
	super := policy.Resolve(AgentProfileSuper)
	if super.Provider != "fireworks" || super.Model != "accounts/fireworks/models/deepseek-v4-pro" || super.ReasoningEffort != "medium" {
		t.Fatalf("super selection = %+v", super)
	}
}

func TestEnsureDefaultModelPolicyMigratesLegacyGeneratedPolicy(t *testing.T) {
	policyPath := filepath.Join(t.TempDir(), "System", "model-policy.toml")
	if err := os.MkdirAll(filepath.Dir(policyPath), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := Config{LLMProvider: "chatgpt", LLMModel: "gpt-5.5", LLMReasoningEffort: "low"}
	if err := os.WriteFile(policyPath, []byte(legacyGeneratedModelPolicyText(cfg)), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ensureDefaultModelPolicyFile(policyPath, cfg); err != nil {
		t.Fatalf("ensureDefaultModelPolicyFile: %v", err)
	}
	raw, err := os.ReadFile(policyPath)
	if err != nil {
		t.Fatal(err)
	}
	policy, err := parseModelPolicy(string(raw), policyPath)
	if err != nil {
		t.Fatalf("parse migrated policy: %v", err)
	}
	conductor := policy.Resolve(AgentProfileConductor)
	if conductor.Provider != "fireworks" || conductor.Model != "accounts/fireworks/models/deepseek-v4-flash" {
		t.Fatalf("migrated conductor selection = %+v", conductor)
	}
}

func TestEnsureDefaultModelPolicyMigratesSemanticallyLegacyGeneratedPolicy(t *testing.T) {
	policyPath := filepath.Join(t.TempDir(), "System", "model-policy.toml")
	if err := os.MkdirAll(filepath.Dir(policyPath), 0o755); err != nil {
		t.Fatal(err)
	}
	raw := `# Choir model policy
# This computer-owned file maps agent roles to provider/model choices.
# Provider secrets stay server-owned; this file names models only.

[defaults]
fallback_provider = "chatgpt"
fallback_model = "gpt-5.5"

[roles.conductor]
provider = "chatgpt"
model = "gpt-5.5"
reasoning = "low"

[roles.super]
provider = "chatgpt"
model = "gpt-5.5"
reasoning = "medium"

[roles.researcher]
provider = "fireworks"
model = "accounts/fireworks/models/deepseek-v4-flash"

[roles.vtext]
provider = "fireworks"
model = "accounts/fireworks/models/deepseek-v4-flash"
`
	if err := os.WriteFile(policyPath, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ensureDefaultModelPolicyFile(policyPath, Config{}); err != nil {
		t.Fatalf("ensureDefaultModelPolicyFile: %v", err)
	}
	migrated, err := os.ReadFile(policyPath)
	if err != nil {
		t.Fatal(err)
	}
	policy, err := parseModelPolicy(string(migrated), policyPath)
	if err != nil {
		t.Fatalf("parse migrated policy: %v", err)
	}
	if got := policy.Resolve(AgentProfileSuper); got.Provider != "fireworks" || got.Model != "accounts/fireworks/models/deepseek-v4-pro" {
		t.Fatalf("migrated super selection = %+v", got)
	}
}

func TestEnsureDefaultModelPolicyPreservesCustomPolicy(t *testing.T) {
	policyPath := filepath.Join(t.TempDir(), "System", "model-policy.toml")
	if err := os.MkdirAll(filepath.Dir(policyPath), 0o755); err != nil {
		t.Fatal(err)
	}
	raw := `# Choir model policy
# This computer-owned file maps agent roles to provider/model choices.
# Provider secrets stay server-owned; this file names models only.

[defaults]
fallback_provider = "fireworks"
fallback_model = "accounts/fireworks/models/deepseek-v4-flash"

[roles.conductor]
provider = "fireworks"
model = "accounts/fireworks/models/deepseek-v4-flash"

[roles.super]
provider = "fireworks"
model = "accounts/fireworks/models/deepseek-v4-pro"
`
	if err := os.WriteFile(policyPath, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ensureDefaultModelPolicyFile(policyPath, Config{}); err != nil {
		t.Fatalf("ensureDefaultModelPolicyFile: %v", err)
	}
	kept, err := os.ReadFile(policyPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(kept) != raw {
		t.Fatalf("custom policy was unexpectedly rewritten")
	}
}

func TestRuntimeResolvesModelPolicyIntoRunMetadata(t *testing.T) {
	rt := testPromptRuntime(t)
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

	metadata := rt.ensureResolvedLLMMetadata(context.Background(), "user-alice", map[string]any{
		runMetadataAgentProfile: AgentProfileResearcher,
		runMetadataAgentRole:    AgentProfileResearcher,
	})

	if got := metadataStringValue(metadata, runMetadataLLMProvider); got != "fireworks" {
		t.Fatalf("llm_provider = %q, want fireworks", got)
	}
	if got := metadataStringValue(metadata, runMetadataLLMModel); got != "accounts/fireworks/models/deepseek-v4-flash" {
		t.Fatalf("llm_model = %q", got)
	}
	if got := metadataStringValue(metadata, runMetadataLLMPolicySource); got != policyPath {
		t.Fatalf("llm_policy_source = %q, want %q", got, policyPath)
	}
}

func TestStartChildRunResolvesModelPolicyIntoRunMetadata(t *testing.T) {
	rt, _ := testRuntime(t)
	ctx := context.Background()
	policyPath := filepath.Join(t.TempDir(), "System", "model-policy.toml")
	if err := os.MkdirAll(filepath.Dir(policyPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(policyPath, []byte(`
[defaults]
fallback_provider = "chatgpt"
fallback_model = "gpt-5.5"

[roles.vtext]
provider = "fireworks"
model = "accounts/fireworks/models/deepseek-v4-flash"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	rt.cfg.ModelPolicyPath = policyPath

	parent, err := rt.createRunWithMetadata(ctx, "parent", "user-alice", map[string]any{
		runMetadataAgentProfile: AgentProfileConductor,
		runMetadataAgentRole:    AgentProfileConductor,
	})
	if err != nil {
		t.Fatalf("create parent: %v", err)
	}

	child, err := rt.StartChildRun(ctx, parent.RunID, "revise vtext", "user-alice", map[string]any{
		runMetadataAgentProfile: AgentProfileVText,
		runMetadataAgentRole:    AgentProfileVText,
	})
	if err != nil {
		t.Fatalf("start child: %v", err)
	}

	if got := metadataStringValue(child.Metadata, runMetadataLLMProvider); got != "fireworks" {
		t.Fatalf("child llm_provider = %q, want fireworks; metadata=%+v", got, child.Metadata)
	}
	if got := metadataStringValue(child.Metadata, runMetadataLLMModel); got != "accounts/fireworks/models/deepseek-v4-flash" {
		t.Fatalf("child llm_model = %q", got)
	}
	if got := metadataStringValue(child.Metadata, runMetadataLLMPolicySource); got != policyPath {
		t.Fatalf("child llm_policy_source = %q, want %q", got, policyPath)
	}
}

func TestRuntimeFallsBackToPreviousValidModelPolicy(t *testing.T) {
	rt := testPromptRuntime(t)
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
