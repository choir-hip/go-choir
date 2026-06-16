package runtime

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseModelPolicyResolvesRoles(t *testing.T) {
	raw := `
[defaults]
fallback_provider = "chatgpt"
fallback_model = "gpt-5.5"
reasoning = "low"
max_tokens = 12000

[roles.super]
provider = "chatgpt"
model = "gpt-5.5"
reasoning = "medium"
max_tokens = 24000

[roles.texture]
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
	if super.MaxTokens != 24000 {
		t.Fatalf("super max tokens = %d, want 24000", super.MaxTokens)
	}
	texture := policy.Resolve(AgentProfileTexture)
	if texture.Provider != "fireworks" || texture.Model != "accounts/fireworks/models/deepseek-v4-flash" {
		t.Fatalf("texture selection = %+v", texture)
	}
	if texture.MaxTokens != 12000 {
		t.Fatalf("texture inherited max tokens = %d, want 12000", texture.MaxTokens)
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

func TestMaxInteractiveOutputTokensForSelectionUsesModelCatalog(t *testing.T) {
	sel := LLMSelection{Provider: "fireworks", Model: "accounts/fireworks/models/deepseek-v4-flash"}
	if got := MaxInteractiveOutputTokensForSelection(sel, AgentProfileConductor); got != 0 {
		t.Fatalf("conductor interactive tokens = %d, want 0 to omit Fireworks max_tokens", got)
	}
	if got := MaxInteractiveOutputTokensForSelection(sel, AgentProfileTexture); got != 0 {
		t.Fatalf("texture interactive tokens = %d, want 0 to omit Fireworks max_tokens", got)
	}
	if got := MaxInteractiveOutputTokensForSelection(LLMSelection{Provider: "chatgpt", Model: "gpt-5.5"}, AgentProfileTexture); got != 0 {
		t.Fatalf("ChatGPT interactive tokens = %d, want 0 to omit unsupported max_output_tokens", got)
	}
	if got := MaxInteractiveOutputTokensForSelection(LLMSelection{Provider: "chatgpt", Model: "gpt-5.5", MaxTokens: 32768}, AgentProfileSuper); got != 0 {
		t.Fatalf("explicit ChatGPT interactive tokens = %d, want 0 to omit unsupported max_output_tokens", got)
	}
	if got := MaxInteractiveOutputTokensForSelection(LLMSelection{Provider: "xiaomi", Model: "mimo-v2.5-pro"}, AgentProfileSuper); got != 0 {
		t.Fatalf("Xiaomi interactive tokens = %d, want 0 to omit OpenAI-compatible chat budget", got)
	}
	if got := MaxInteractiveOutputTokensForSelection(LLMSelection{Provider: "fireworks", Model: "accounts/fireworks/models/deepseek-v4-flash", MaxTokens: 32768}, AgentProfileSuper); got != 32768 {
		t.Fatalf("explicit Fireworks interactive tokens = %d, want 32768", got)
	}
	if got := MaxInteractiveOutputTokensForSelection(LLMSelection{Model: "us.anthropic.claude-haiku-4-5-20251001-v1:0"}, AgentProfileSuper); got != 8192 {
		t.Fatalf("low-limit model interactive tokens = %d, want 8192", got)
	}
}

func TestFallbackModelPolicyUsesGeneratedMimoDefaults(t *testing.T) {
	policy := fallbackModelPolicy(Config{})
	conductor := policy.Resolve(AgentProfileConductor)
	if conductor.Provider != "deepseek" || conductor.Model != "deepseek-v4-flash" || conductor.ReasoningEffort != "medium" {
		t.Fatalf("conductor selection = %+v", conductor)
	}
	super := policy.Resolve(AgentProfileSuper)
	if super.Provider != "deepseek" || super.Model != "deepseek-v4-flash" || super.ReasoningEffort != "medium" {
		t.Fatalf("super selection = %+v", super)
	}
	texture := policy.Resolve(AgentProfileTexture)
	if texture.Provider != "xiaomi" || texture.Model != "mimo-v2.5" || texture.ReasoningEffort != "medium" {
		t.Fatalf("texture selection = %+v", texture)
	}
	processor := policy.Resolve(AgentProfileProcessor)
	if processor.Provider != "xiaomi" || processor.Model != "mimo-v2.5" || processor.ReasoningEffort != "medium" {
		t.Fatalf("processor selection = %+v", processor)
	}
	vsuper := policy.Resolve(AgentProfileVSuper)
	if vsuper.Provider != "deepseek" || vsuper.Model != "deepseek-v4-flash" {
		t.Fatalf("vsuper selection = %+v", vsuper)
	}
	cosuper := policy.Resolve(AgentProfileCoSuper)
	if cosuper.Provider != "deepseek" || cosuper.Model != "deepseek-v4-flash" {
		t.Fatalf("co-super selection = %+v", cosuper)
	}
	reconciler := policy.Resolve(AgentProfileReconciler)
	if reconciler.Provider != "deepseek" || reconciler.Model != "deepseek-v4-flash" {
		t.Fatalf("reconciler selection = %+v", reconciler)
	}
	verifier := policy.Resolve("verifier")
	if verifier.Provider != "deepseek" || verifier.Model != "deepseek-v4-flash" {
		t.Fatalf("verifier selection = %+v", verifier)
	}
	multimodal := policy.Resolve("verifier_multimodal")
	if multimodal.Provider != "xiaomi" || multimodal.Model != "mimo-v2.5" {
		t.Fatalf("multimodal verifier selection = %+v", multimodal)
	}
}

func TestGeneratedModelPolicyUsesTextureRoleKey(t *testing.T) {
	raw := defaultModelPolicyText(Config{})
	if !strings.Contains(raw, "[roles.texture]") {
		t.Fatalf("generated model policy missing [roles.texture]:\n%s", raw)
	}
	policy, err := parseModelPolicy(raw, "/System/model-policy.toml")
	if err != nil {
		t.Fatalf("parse generated model policy: %v", err)
	}
	texture := policy.Resolve(AgentProfileTexture)
	if texture.Provider != "xiaomi" || texture.Model != "mimo-v2.5" {
		t.Fatalf("texture selection = %+v, want generated Xiaomi default", texture)
	}
}

func TestNormalizeModelPolicyRoleSeparatesVerifierModalities(t *testing.T) {
	if got := normalizeModelPolicyRole("verifier"); got != "verifier" {
		t.Fatalf("verifier normalized to %q, want verifier", got)
	}
	if got := normalizeModelPolicyRole("verifier_text"); got != "verifier" {
		t.Fatalf("verifier_text normalized to %q, want verifier", got)
	}
	if got := normalizeModelPolicyRole("verifier_multimodal"); got != "verifier_multimodal" {
		t.Fatalf("verifier_multimodal normalized to %q", got)
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
	if conductor.Provider != "deepseek" || conductor.Model != "deepseek-v4-flash" {
		t.Fatalf("migrated conductor selection = %+v", conductor)
	}
}

func TestDefaultModelPolicyIgnoresChatGPTProcessFallback(t *testing.T) {
	raw := defaultModelPolicyText(Config{LLMProvider: "chatgpt", LLMModel: "gpt-5.5", LLMReasoningEffort: "low"})
	policy, err := parseModelPolicy(raw, "generated")
	if err != nil {
		t.Fatalf("parse generated policy: %v", err)
	}
	if got := policy.Resolve("unknown-role"); got.Provider != "deepseek" || got.Model != "deepseek-v4-flash" {
		t.Fatalf("generated fallback selection = %+v", got)
	}
	if got := policy.Resolve(AgentProfileConductor); got.Provider != "deepseek" || got.Model != "deepseek-v4-flash" || got.ReasoningEffort != "medium" {
		t.Fatalf("generated conductor selection = %+v", got)
	}
	if got := policy.Resolve(AgentProfileSuper); got.Provider != "deepseek" || got.Model != "deepseek-v4-flash" || got.ReasoningEffort != "medium" {
		t.Fatalf("generated super selection = %+v", got)
	}
}

func TestEnsureDefaultModelPolicyMigratesGeneratedFlashNoneForegroundPolicy(t *testing.T) {
	policyPath := filepath.Join(t.TempDir(), "System", "model-policy.toml")
	if err := os.MkdirAll(filepath.Dir(policyPath), 0o755); err != nil {
		t.Fatal(err)
	}
	raw := `# Choir model policy
# This computer-owned file maps agent roles to provider/model choices.
# Provider secrets stay server-owned; this file names models only.
# Optional max_tokens requests an explicit per-call budget. Omit it for provider
# defaults, especially Fireworks chat completions.

[defaults]
fallback_provider = "fireworks"
fallback_model = "accounts/fireworks/models/deepseek-v4-flash"

[roles.conductor]
provider = "fireworks"
model = "accounts/fireworks/models/deepseek-v4-flash"
reasoning = "none"

[roles.super]
provider = "fireworks"
model = "accounts/fireworks/models/deepseek-v4-pro"
reasoning = "medium"

[roles.vsuper]
provider = "fireworks"
model = "accounts/fireworks/models/deepseek-v4-pro"

[roles.co-super]
provider = "fireworks"
model = "accounts/fireworks/models/deepseek-v4-pro"

[roles.researcher]
provider = "fireworks"
model = "accounts/fireworks/models/deepseek-v4-flash"
reasoning = "none"

[roles.texture]
provider = "fireworks"
model = "accounts/fireworks/models/deepseek-v4-flash"
reasoning = "none"

[roles.verifier]
provider = "fireworks"
model = "accounts/fireworks/models/deepseek-v4-pro"
requires = ["text", "tool_use"]

[roles.verifier_multimodal]
provider = "fireworks"
model = "accounts/fireworks/models/kimi-k2p6"
requires = ["image", "tool_use"]
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
	for _, role := range []string{AgentProfileConductor, AgentProfileResearcher} {
		got := policy.Resolve(role)
		if got.Provider != "deepseek" || got.Model != "deepseek-v4-flash" || got.ReasoningEffort != "medium" {
			t.Fatalf("migrated %s selection = %+v", role, got)
		}
	}
	got := policy.Resolve(AgentProfileTexture)
	if got.Provider != "xiaomi" || got.Model != "mimo-v2.5" || got.ReasoningEffort != "medium" {
		t.Fatalf("migrated texture selection = %+v", got)
	}
}

func TestEnsureDefaultModelPolicyMigratesGeneratedDeepSeekPolicy(t *testing.T) {
	policyPath := filepath.Join(t.TempDir(), "System", "model-policy.toml")
	if err := os.MkdirAll(filepath.Dir(policyPath), 0o755); err != nil {
		t.Fatal(err)
	}
	raw := `# Choir model policy
# This computer-owned file maps agent roles to provider/model choices.
# Provider secrets stay server-owned; this file names models only.

[defaults]
fallback_provider = "deepseek"
fallback_model = "deepseek-v4-flash"
reasoning = "medium"

[roles.conductor]
provider = "deepseek"
model = "deepseek-v4-flash"
reasoning = "medium"

[roles.super]
provider = "deepseek"
model = "deepseek-v4-pro"
reasoning = "medium"

[roles.vsuper]
provider = "deepseek"
model = "deepseek-v4-pro"

[roles.co-super]
provider = "deepseek"
model = "deepseek-v4-pro"

[roles.researcher]
provider = "deepseek"
model = "deepseek-v4-flash"
reasoning = "medium"

[roles.texture]
provider = "deepseek"
model = "deepseek-v4-flash"
reasoning = "medium"

[roles.verifier]
provider = "deepseek"
model = "deepseek-v4-pro"
requires = ["text", "tool_use"]

[roles.verifier_multimodal]
provider = "xiaomi"
model = "mimo-v2.5"
requires = ["image", "tool_use"]
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
	if got := policy.Resolve(AgentProfileConductor); got.Provider != "deepseek" || got.Model != "deepseek-v4-flash" {
		t.Fatalf("migrated conductor selection = %+v", got)
	}
	if got := policy.Resolve(AgentProfileVSuper); got.Provider != "deepseek" || got.Model != "deepseek-v4-flash" {
		t.Fatalf("migrated vsuper selection = %+v", got)
	}
	if got := policy.Resolve(AgentProfileSuper); got.Provider != "deepseek" || got.Model != "deepseek-v4-flash" {
		t.Fatalf("migrated super selection = %+v", got)
	}
	if got := policy.Resolve(AgentProfileReconciler); got.Provider != "deepseek" || got.Model != "deepseek-v4-flash" {
		t.Fatalf("migrated reconciler selection = %+v", got)
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

[roles.texture]
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
	if got := policy.Resolve(AgentProfileSuper); got.Provider != "deepseek" || got.Model != "deepseek-v4-flash" {
		t.Fatalf("migrated super selection = %+v", got)
	}
}

func TestEnsureDefaultModelPolicyMigratesPartialDeepSeekPolicy(t *testing.T) {
	policyPath := filepath.Join(t.TempDir(), "System", "model-policy.toml")
	if err := os.MkdirAll(filepath.Dir(policyPath), 0o755); err != nil {
		t.Fatal(err)
	}
	raw := `# Choir model policy
# This computer-owned file maps agent roles to provider/model choices.
# Provider secrets stay server-owned; this file names models only.

[defaults]
fallback_provider = "deepseek"
fallback_model = "deepseek-v4-flash"
reasoning = "medium"

[roles.researcher]
provider = "deepseek"
model = "deepseek-v4-flash"
reasoning = "medium"
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
	if got := policy.Resolve(AgentProfileResearcher); got.Provider != "deepseek" || got.Model != "deepseek-v4-flash" {
		t.Fatalf("migrated researcher selection = %+v", got)
	}
	if got := policy.Resolve(AgentProfileSuper); got.Provider != "deepseek" || got.Model != "deepseek-v4-flash" {
		t.Fatalf("migrated super selection = %+v", got)
	}
}

func TestEnsureDefaultModelPolicyMigratesStaleForegroundChatGPTPins(t *testing.T) {
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
provider = "chatgpt"
model = "gpt-5.5"

[roles.super]
provider = "chatgpt"
model = "gpt-5.5"
reasoning = "medium"

[roles.texture]
provider = "fireworks"
model = "accounts/fireworks/models/deepseek-v4-flash"
reasoning = "none"
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
	if got := policy.Resolve(AgentProfileConductor); got.Provider != "deepseek" || got.Model != "deepseek-v4-flash" || got.ReasoningEffort != "medium" {
		t.Fatalf("migrated conductor selection = %+v", got)
	}
	if got := policy.Resolve(AgentProfileSuper); got.Provider != "deepseek" || got.Model != "deepseek-v4-flash" || got.ReasoningEffort != "medium" {
		t.Fatalf("migrated super selection = %+v", got)
	}
}

func TestEnsureDefaultModelPolicyMigratesLegacyChatGPTFallback(t *testing.T) {
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
reasoning = "low"

[roles.texture]
provider = "fireworks"
model = "accounts/fireworks/models/deepseek-v4-flash"
reasoning = "none"
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
	if got := policy.Resolve(AgentProfileConductor); got.Provider != "deepseek" || got.Model != "deepseek-v4-flash" || got.ReasoningEffort != "medium" {
		t.Fatalf("migrated conductor selection = %+v", got)
	}
	if got := policy.Resolve(AgentProfileResearcher); got.Provider != "deepseek" || got.Model != "deepseek-v4-flash" || got.ReasoningEffort != "medium" {
		t.Fatalf("migrated researcher selection = %+v", got)
	}
}

func TestEnsureDefaultModelPolicyMigratesCustomDeepSeekPolicy(t *testing.T) {
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
	migrated, err := os.ReadFile(policyPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(migrated) == raw {
		t.Fatalf("deprecated deepseek policy was not rewritten")
	}
	policy, err := parseModelPolicy(string(migrated), policyPath)
	if err != nil {
		t.Fatalf("parse migrated policy: %v", err)
	}
	if got := policy.Resolve(AgentProfileConductor); got.Provider != "deepseek" || got.Model != "deepseek-v4-flash" {
		t.Fatalf("migrated conductor selection = %+v", got)
	}
	if got := policy.Resolve(AgentProfileSuper); got.Provider != "deepseek" || got.Model != "deepseek-v4-flash" {
		t.Fatalf("migrated super selection = %+v", got)
	}
}

func TestEnsureDefaultModelPolicyMigratesAllXiaomiPolicy(t *testing.T) {
	policyPath := filepath.Join(t.TempDir(), "System", "model-policy.toml")
	if err := os.MkdirAll(filepath.Dir(policyPath), 0o755); err != nil {
		t.Fatal(err)
	}
	raw := `# Choir model policy
# This computer-owned file maps agent roles to provider/model choices.
# Provider secrets stay server-owned; this file names models only.

[defaults]
fallback_provider = "xiaomi"
fallback_model = "mimo-v2.5"
reasoning = "medium"

[roles.conductor]
provider = "xiaomi"
model = "mimo-v2.5"
reasoning = "medium"

[roles.super]
provider = "xiaomi"
model = "mimo-v2.5-pro"
reasoning = "medium"
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
	if got := policy.Resolve(AgentProfileConductor); got.Provider != "deepseek" || got.Model != "deepseek-v4-flash" {
		t.Fatalf("migrated conductor selection = %+v", got)
	}
	if got := policy.Resolve(AgentProfileSuper); got.Provider != "deepseek" || got.Model != "deepseek-v4-flash" {
		t.Fatalf("migrated super selection = %+v", got)
	}
}

func TestEnsureDefaultModelPolicyPreservesIntentionalChatGPTPolicy(t *testing.T) {
	policyPath := filepath.Join(t.TempDir(), "System", "model-policy.toml")
	if err := os.MkdirAll(filepath.Dir(policyPath), 0o755); err != nil {
		t.Fatal(err)
	}
	raw := `# Choir model policy
# This computer-owned file maps agent roles to provider/model choices.
# Provider secrets stay server-owned; this file names models only.

[defaults]
fallback_provider = "chatgpt"
fallback_model = "gpt-5.4"

[roles.conductor]
provider = "chatgpt"
model = "gpt-5.4"
reasoning = "low"

[roles.super]
provider = "chatgpt"
model = "gpt-5.4"
reasoning = "high"
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
		t.Fatalf("intentional ChatGPT policy was unexpectedly rewritten")
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

func TestRuntimeResolvesModelPolicyOverlayIntoRunMetadata(t *testing.T) {
	rt := testPromptRuntime(t)
	policyPath := filepath.Join(t.TempDir(), "System", "model-policy.toml")
	overlayDir := filepath.Join(filepath.Dir(policyPath), "model-policy-overlays")
	if err := os.MkdirAll(overlayDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(policyPath, []byte(`
[defaults]
fallback_provider = "deepseek"
fallback_model = "deepseek-v4-flash"
reasoning = "medium"

[roles.researcher]
provider = "deepseek"
model = "deepseek-v4-flash"
reasoning = "medium"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	future := time.Now().UTC().Add(time.Hour).Format(time.RFC3339)
	if err := os.WriteFile(filepath.Join(overlayDir, "mimo-arm.toml"), []byte(`
[overlay]
expires_at = "`+future+`"

[roles.researcher]
provider = "xiaomi"
model = "mimo-v2.5-pro"
reasoning = "medium"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	rt.cfg.ModelPolicyPath = policyPath

	metadata := rt.ensureResolvedLLMMetadata(context.Background(), "user-alice", map[string]any{
		runMetadataAgentProfile:       AgentProfileResearcher,
		runMetadataAgentRole:          AgentProfileResearcher,
		runMetadataLLMPolicyOverlayID: "mimo-arm",
	})

	if got := metadataStringValue(metadata, runMetadataLLMProvider); got != "xiaomi" {
		t.Fatalf("llm_provider = %q, want xiaomi; metadata=%+v", got, metadata)
	}
	if got := metadataStringValue(metadata, runMetadataLLMModel); got != "mimo-v2.5-pro" {
		t.Fatalf("llm_model = %q", got)
	}
	if got := metadataStringValue(metadata, runMetadataLLMReasoningEffort); got != "medium" {
		t.Fatalf("llm_reasoning_effort = %q", got)
	}
	if got := metadataStringValue(metadata, runMetadataLLMPolicySource); got != filepath.Join(overlayDir, "mimo-arm.toml") {
		t.Fatalf("llm_policy_source = %q", got)
	}
	if metadataStringValue(metadata, runMetadataLLMPolicyError) != "" {
		t.Fatalf("unexpected policy error: %+v", metadata)
	}
}

func TestRuntimeRejectsExpiredModelPolicyOverlay(t *testing.T) {
	rt := testPromptRuntime(t)
	policyPath := filepath.Join(t.TempDir(), "System", "model-policy.toml")
	overlayDir := filepath.Join(filepath.Dir(policyPath), "model-policy-overlays")
	if err := os.MkdirAll(overlayDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(policyPath, []byte(`
[defaults]
fallback_provider = "deepseek"
fallback_model = "deepseek-v4-flash"

[roles.researcher]
provider = "deepseek"
model = "deepseek-v4-flash"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	past := time.Now().UTC().Add(-time.Hour).Format(time.RFC3339)
	if err := os.WriteFile(filepath.Join(overlayDir, "expired.toml"), []byte(`
[overlay]
expires_at = "`+past+`"

[roles.researcher]
provider = "xiaomi"
model = "mimo-v2.5-pro"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	rt.cfg.ModelPolicyPath = policyPath

	metadata := rt.ensureResolvedLLMMetadata(context.Background(), "user-alice", map[string]any{
		runMetadataAgentProfile:       AgentProfileResearcher,
		runMetadataAgentRole:          AgentProfileResearcher,
		runMetadataLLMPolicyOverlayID: "expired",
	})

	if got := metadataStringValue(metadata, runMetadataLLMProvider); got != "deepseek" {
		t.Fatalf("llm_provider = %q, want base fallback deepseek; metadata=%+v", got, metadata)
	}
	if got := metadataStringValue(metadata, runMetadataLLMModel); got != "deepseek-v4-flash" {
		t.Fatalf("llm_model = %q", got)
	}
	if errText := metadataStringValue(metadata, runMetadataLLMPolicyError); errText == "" {
		t.Fatalf("expected expired overlay policy error")
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

[roles.texture]
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

	child, err := rt.StartChildRun(ctx, parent.RunID, "revise texture", "user-alice", map[string]any{
		runMetadataAgentProfile: AgentProfileTexture,
		runMetadataAgentRole:    AgentProfileTexture,
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

func TestStartChildRunResolvesModelPolicyOverlayIntoRunMetadata(t *testing.T) {
	rt, _ := testRuntime(t)
	ctx := context.Background()
	policyPath := filepath.Join(t.TempDir(), "System", "model-policy.toml")
	overlayDir := filepath.Join(filepath.Dir(policyPath), "model-policy-overlays")
	if err := os.MkdirAll(overlayDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(policyPath, []byte(`
[defaults]
fallback_provider = "deepseek"
fallback_model = "deepseek-v4-flash"

[roles.researcher]
provider = "deepseek"
model = "deepseek-v4-flash"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	future := time.Now().UTC().Add(time.Hour).Format(time.RFC3339)
	if err := os.WriteFile(filepath.Join(overlayDir, "gpt-mini-arm.toml"), []byte(`
[overlay]
expires_at = "`+future+`"

[roles.researcher]
provider = "chatgpt"
model = "gpt-5.4-mini"
reasoning = "low"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	rt.cfg.ModelPolicyPath = policyPath

	parent, err := rt.createRunWithMetadata(ctx, "parent", "user-alice", map[string]any{
		runMetadataAgentProfile: AgentProfileSuper,
		runMetadataAgentRole:    AgentProfileSuper,
	})
	if err != nil {
		t.Fatalf("create parent: %v", err)
	}

	child, err := rt.StartChildRun(ctx, parent.RunID, "research under gpt mini", "user-alice", map[string]any{
		runMetadataAgentProfile:       AgentProfileResearcher,
		runMetadataAgentRole:          AgentProfileResearcher,
		runMetadataLLMPolicyOverlayID: "gpt-mini-arm",
	})
	if err != nil {
		t.Fatalf("start child: %v", err)
	}

	if got := metadataStringValue(child.Metadata, runMetadataLLMProvider); got != "chatgpt" {
		t.Fatalf("child llm_provider = %q, want chatgpt; metadata=%+v", got, child.Metadata)
	}
	if got := metadataStringValue(child.Metadata, runMetadataLLMModel); got != "gpt-5.4-mini" {
		t.Fatalf("child llm_model = %q", got)
	}
	if got := metadataStringValue(child.Metadata, runMetadataLLMReasoningEffort); got != "low" {
		t.Fatalf("child reasoning = %q", got)
	}
	if got := metadataStringValue(child.Metadata, runMetadataLLMPolicyOverlayID); got != "gpt-mini-arm" {
		t.Fatalf("overlay id = %q", got)
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

func TestProviderPreconditionFallbackSelectionsUseCrossProviderFlash(t *testing.T) {
	fallbacks := providerPreconditionFallbackSelections(LLMSelection{
		Provider:        "deepseek",
		Model:           "deepseek-v4-flash",
		ReasoningEffort: "medium",
	})
	if len(fallbacks) != 2 {
		t.Fatalf("fallbacks = %+v, want Xiaomi flash then gpt-5.4-mini", fallbacks)
	}
	if got := fallbacks[0]; got.Provider != "xiaomi" ||
		got.Model != "mimo-v2.5" ||
		got.ReasoningEffort != "medium" ||
		got.Source != "provider_precondition_fallback" {
		t.Fatalf("fallback = %+v", got)
	}
	if got := fallbacks[1]; got.Provider != "chatgpt" ||
		got.Model != "gpt-5.4-mini" ||
		got.ReasoningEffort != "low" ||
		got.Source != "provider_precondition_terminal_fallback" {
		t.Fatalf("terminal fallback = %+v", got)
	}

	xiaomiFallbacks := providerPreconditionFallbackSelections(LLMSelection{
		Provider: "xiaomi",
		Model:    "mimo-v2.5",
	})
	if len(xiaomiFallbacks) != 2 ||
		xiaomiFallbacks[0].Provider != "deepseek" ||
		xiaomiFallbacks[0].Model != "deepseek-v4-flash" ||
		xiaomiFallbacks[1].Provider != "chatgpt" ||
		xiaomiFallbacks[1].Model != "gpt-5.4-mini" {
		t.Fatalf("xiaomi fallbacks = %+v, want deepseek flash then gpt-5.4-mini", xiaomiFallbacks)
	}

	if got := providerPreconditionFallbackSelections(LLMSelection{
		Provider: "deepseek",
		Model:    "deepseek-v4-pro",
	}); len(got) != 2 || got[0].Provider != "xiaomi" || got[0].Model != "mimo-v2.5" ||
		got[1].Provider != "chatgpt" || got[1].Model != "gpt-5.4-mini" {
		t.Fatalf("pro fallbacks = %+v, want xiaomi flash then gpt-5.4-mini", got)
	}

	fireworksFlashFallbacks := providerPreconditionFallbackSelections(LLMSelection{
		Provider: "fireworks",
		Model:    "accounts/fireworks/models/deepseek-v4-flash",
	})
	if len(fireworksFlashFallbacks) != 3 ||
		fireworksFlashFallbacks[0].Provider != "xiaomi" ||
		fireworksFlashFallbacks[1].Provider != "deepseek" ||
		fireworksFlashFallbacks[2].Provider != "chatgpt" ||
		fireworksFlashFallbacks[2].Model != "gpt-5.4-mini" {
		t.Fatalf("fireworks flash fallbacks = %+v, want xiaomi, deepseek flash, then gpt-5.4-mini", fireworksFlashFallbacks)
	}
}
