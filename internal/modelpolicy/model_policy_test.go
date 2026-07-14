package modelpolicy

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
)

func TestPolicyParsesAndResolvesRoles(t *testing.T) {
	policy, err := parsePolicy(`
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
`, "/System/model-policy.toml")
	if err != nil {
		t.Fatalf("parse policy: %v", err)
	}
	super := policy.Resolve(agentprofile.Super)
	if super.Provider != "chatgpt" || super.Model != "gpt-5.5" || super.ReasoningEffort != "medium" || super.MaxTokens != 24000 {
		t.Fatalf("super selection = %+v", super)
	}
	texture := policy.Resolve("texture-agent")
	if texture.Provider != "fireworks" || texture.Model != "accounts/fireworks/models/deepseek-v4-flash" || texture.MaxTokens != 12000 {
		t.Fatalf("texture selection = %+v", texture)
	}
	unknown := policy.Resolve("unknown")
	if unknown.Provider != "chatgpt" || unknown.Model != "gpt-5.5" || unknown.ReasoningEffort != "low" {
		t.Fatalf("default selection = %+v", unknown)
	}
}

func TestManagerCreatesDefaultCutoverPolicy(t *testing.T) {
	path := filepath.Join(t.TempDir(), "System", "model-policy.toml")
	manager := NewManager(ManagerConfig{PolicyPath: path})
	policy, err := manager.Load(context.Background(), "owner")
	if err != nil {
		t.Fatalf("load generated policy: %v", err)
	}
	for role, want := range map[string]provideriface.LLMSelection{
		agentprofile.Conductor: {Provider: "chatgpt", Model: "gpt-5.4-mini", ReasoningEffort: "low"},
		agentprofile.Super:     {Provider: "chatgpt", Model: "gpt-5.5", ReasoningEffort: "high"},
		agentprofile.Texture:   {Provider: "chatgpt", Model: "gpt-5.5", ReasoningEffort: "low"},
		VerifierRole:           {Provider: "deepseek", Model: "deepseek-v4-flash", ReasoningEffort: "low"},
		MultimodalVerifierRole: {Provider: "xiaomi", Model: "mimo-v2.5", ReasoningEffort: "low"},
	} {
		got := policy.Resolve(role)
		if got.Provider != want.Provider || got.Model != want.Model || got.ReasoningEffort != want.ReasoningEffort {
			t.Errorf("%s selection = %+v, want %+v", role, got, want)
		}
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "[roles.texture]") || strings.Contains(string(raw), "[roles.vtext]") {
		t.Fatalf("generated policy has wrong texture role:\n%s", raw)
	}
}

func TestManagerPreservesExistingPolicyAndLastValidCache(t *testing.T) {
	path := filepath.Join(t.TempDir(), "System", "model-policy.toml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	valid := `[defaults]
fallback_provider = "chatgpt"
fallback_model = "gpt-5.5"

[roles.super]
provider = "chatgpt"
model = "gpt-5.5"
reasoning = "medium"
`
	if err := os.WriteFile(path, []byte(valid), 0o644); err != nil {
		t.Fatal(err)
	}
	manager := NewManager(ManagerConfig{PolicyPath: path})
	if _, err := manager.Load(context.Background(), "owner"); err != nil {
		t.Fatalf("load valid policy: %v", err)
	}
	kept, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(kept) != valid {
		t.Fatal("existing policy was rewritten")
	}
	if err := os.WriteFile(path, []byte("this is not a policy assignment"), 0o644); err != nil {
		t.Fatal(err)
	}
	policy, err := manager.Load(context.Background(), "owner")
	if err == nil || !strings.Contains(err.Error(), "previous valid policy") {
		t.Fatalf("load error = %v", err)
	}
	selection := policy.Resolve(agentprofile.Super)
	if selection.Model != "gpt-5.5" || selection.ReasoningEffort != "medium" {
		t.Fatalf("cached selection = %+v", selection)
	}
}

func TestManagerAppliesSafeOverlayAndRejectsUnsafeOrExpiredOverlay(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "System", "model-policy.toml")
	manager := NewManager(ManagerConfig{PolicyPath: path})
	if _, err := manager.Load(context.Background(), "owner"); err != nil {
		t.Fatal(err)
	}
	overlayDir := filepath.Join(dir, "System", "model-policy-overlays")
	if err := os.MkdirAll(overlayDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(overlayDir, "mimo-eval.toml"), []byte(`
[overlay]
expires_at = "2099-01-01T00:00:00Z"

[roles.researcher]
provider = "xiaomi"
model = "mimo-v2.5"
reasoning = "medium"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	selection, err := manager.Resolve(context.Background(), "owner", agentprofile.Researcher, "mimo-eval")
	if err != nil {
		t.Fatalf("resolve overlay: %v", err)
	}
	if selection.Provider != "xiaomi" || selection.Model != "mimo-v2.5" || selection.ReasoningEffort != "medium" || !strings.HasSuffix(selection.Source, "mimo-eval.toml") {
		t.Fatalf("overlay selection = %+v", selection)
	}
	if _, err := manager.Resolve(context.Background(), "owner", agentprofile.Researcher, "../escape"); err == nil || !strings.Contains(err.Error(), "not allowed") {
		t.Fatalf("unsafe overlay error = %v", err)
	}
	expiredAt := time.Now().UTC().Add(-time.Hour).Format(time.RFC3339)
	if err := os.WriteFile(filepath.Join(overlayDir, "expired.toml"), []byte("[overlay]\nexpires_at = \""+expiredAt+"\"\n\n[roles.researcher]\nprovider = \"xiaomi\"\nmodel = \"mimo-v2.5\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fallback, err := manager.Resolve(context.Background(), "owner", agentprofile.Researcher, "expired")
	if err == nil || !strings.Contains(err.Error(), "overlay expired") {
		t.Fatalf("expired overlay error = %v", err)
	}
	if fallback.Provider != "chatgpt" || fallback.Model != "gpt-5.4-mini" {
		t.Fatalf("expired overlay fallback = %+v", fallback)
	}
}

func TestManagerEnrichesMetadataAndPreservesExplicitSelection(t *testing.T) {
	manager := NewManager(ManagerConfig{})
	metadata := manager.EnrichMetadata(context.Background(), "owner", agentprofile.Super, nil)
	if metadata[MetadataProvider] != "chatgpt" || metadata[MetadataModel] != "gpt-5.5" || metadata[MetadataReasoningEffort] != "high" || metadata[MetadataPolicySource] != "platform_fallback" {
		t.Fatalf("enriched metadata = %#v", metadata)
	}
	explicit := map[string]any{MetadataProvider: "custom", MetadataModel: "custom-model"}
	got := manager.EnrichMetadata(context.Background(), "owner", agentprofile.Super, explicit)
	if got[MetadataProvider] != "custom" || got[MetadataModel] != "custom-model" || len(got) != 2 {
		t.Fatalf("explicit metadata changed = %#v", got)
	}
}

func TestProviderPreconditionFallbacksPreserveOrder(t *testing.T) {
	fallbacks := ProviderPreconditionFallbackSelections(provideriface.LLMSelection{Provider: "fireworks", Model: "accounts/fireworks/models/deepseek-v4-flash"})
	if len(fallbacks) != 3 || fallbacks[0].Provider != "xiaomi" || fallbacks[1].Provider != "deepseek" || fallbacks[2].Provider != "chatgpt" || fallbacks[2].Model != "gpt-5.4-mini" {
		t.Fatalf("fallbacks = %+v", fallbacks)
	}
}
