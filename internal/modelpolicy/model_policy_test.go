package modelpolicy

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/modelcatalog"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
)

func TestPolicyParsesAndResolvesRoles(t *testing.T) {
	policy, err := parsePolicy(`
[defaults]
fallback_provider = "chatgpt"
fallback_model = "gpt-5.5"
reasoning = "low"
max_tokens = 12000

[roles.management]
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
	management := policy.Resolve(agentprofile.Management)
	if management.Provider != "chatgpt" || management.Model != "gpt-5.5" || management.ReasoningEffort != "medium" || management.MaxTokens != 24000 {
		t.Fatalf("super selection = %+v", management)
	}
	texture := policy.Resolve(agentprofile.Texture)
	if texture.Provider != "fireworks" || texture.Model != "accounts/fireworks/models/deepseek-v4-flash" || texture.MaxTokens != 12000 {
		t.Fatalf("texture selection = %+v", texture)
	}
	unknown := policy.Resolve("unknown")
	if unknown.Provider != "chatgpt" || unknown.Model != "gpt-5.5" || unknown.ReasoningEffort != "low" {
		t.Fatalf("default selection = %+v", unknown)
	}
}

func TestManagerPreservesExistingPolicyAndLastValidCache(t *testing.T) {
	path := filepath.Join(t.TempDir(), "System", "model-policy.toml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	valid := `# owner-edited
[defaults]
fallback_provider = "chatgpt"
fallback_model = "gpt-5.5"

[roles.management]
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
	if err := os.WriteFile(path, []byte("# owner-edited\nthis is not a policy assignment"), 0o644); err != nil {
		t.Fatal(err)
	}
	policy, err := manager.Load(context.Background(), "owner")
	if err == nil || !strings.Contains(err.Error(), "previous valid policy") {
		t.Fatalf("load error = %v", err)
	}
	selection := policy.Resolve(agentprofile.Management)
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
	if err := os.WriteFile(filepath.Join(overlayDir, "safe-eval.toml"), []byte(`
[overlay]
expires_at = "2099-01-01T00:00:00Z"

[roles.research]
provider = "chatgpt"
model = "gpt-5.6-luna"
reasoning = "medium"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	selection, err := manager.Resolve(context.Background(), "owner", agentprofile.Research, "safe-eval")
	if err != nil {
		t.Fatalf("resolve overlay: %v", err)
	}
	if selection.Provider != "chatgpt" || selection.Model != "gpt-5.6-luna" || selection.ReasoningEffort != "medium" || !strings.HasSuffix(selection.Source, "safe-eval.toml") {
		t.Fatalf("overlay selection = %+v", selection)
	}
	if _, err := manager.Resolve(context.Background(), "owner", agentprofile.Research, "../escape"); err == nil || !strings.Contains(err.Error(), "not allowed") {
		t.Fatalf("unsafe overlay error = %v", err)
	}
	expiredAt := time.Now().UTC().Add(-time.Hour).Format(time.RFC3339)
	if err := os.WriteFile(filepath.Join(overlayDir, "expired.toml"), []byte("[overlay]\nexpires_at = \""+expiredAt+"\"\n\n[roles.research]\nprovider = \"chatgpt\"\nmodel = \"gpt-5.6-luna\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fallback, err := manager.Resolve(context.Background(), "owner", agentprofile.Research, "expired")
	if err == nil || !strings.Contains(err.Error(), "overlay expired") {
		t.Fatalf("expired overlay error = %v", err)
	}
	if fallback.Model != "pool:default" {
		t.Fatalf("expired overlay fallback = %+v", fallback)
	}
}

func TestManagerEnrichesMetadataAndPreservesExplicitSelection(t *testing.T) {
	manager := NewManager(ManagerConfig{})
	metadata := manager.EnrichMetadata(context.Background(), "owner", agentprofile.Management, map[string]any{"trajectory_id": "traj-1"})
	member := metadata[MetadataProvider].(string) + "/" + metadata[MetadataModel].(string)
	// The fallbacks are the other pool members in pool order after the
	// assigned one, wrapping.
	pool := modelcatalog.Pools()[modelcatalog.DefaultPool]
	var rest []string
	for i, m := range pool {
		if m.Provider+"/"+m.Model != member {
			continue
		}
		for j := 1; j < len(pool); j++ {
			next := pool[(i+j)%len(pool)]
			rest = append(rest, next.Provider+"/"+next.Model)
		}
	}
	other := strings.Join(rest, ",")
	if other == "" || metadata[MetadataFallbacks] != other || metadata[MetadataSpec] != "pool:default" || metadata[MetadataAssignment] != "hash" || metadata[MetadataThread] != "traj-1" || metadata[MetadataReasoningEffort] != "high" || metadata[MetadataPolicySource] != "platform_fallback" {
		t.Fatalf("enriched metadata = %#v", metadata)
	}
	explicit := map[string]any{MetadataProvider: "custom", MetadataModel: "custom-model"}
	got := manager.EnrichMetadata(context.Background(), "owner", agentprofile.Management, explicit)
	if got[MetadataProvider] != "custom" || got[MetadataModel] != "custom-model" || len(got) != 2 {
		t.Fatalf("explicit metadata changed = %#v", got)
	}
}

func TestProviderPreconditionFallbacksRefuseAlternativeSelections(t *testing.T) {
	for _, selection := range []provideriface.LLMSelection{
		{Provider: "opencode-go", Model: "deepseek-v4.1-flash"},
		{Provider: "chatgpt", Model: "gpt-5.6-luna"},
		{},
	} {
		if fallbacks := ProviderPreconditionFallbackSelections(selection); len(fallbacks) != 0 {
			t.Fatalf("fallbacks for %+v = %+v, want none", selection, fallbacks)
		}
	}
}

// A pool spec assigns each thread one member by a stable hash: one key is
// always the same model, and many keys reach every member.
func TestAssignPoolIsStablePerThreadAndSpreadsAcrossThreads(t *testing.T) {
	spec := provideriface.LLMSelection{Model: "pool:default", ReasoningEffort: "high"}
	first, fallbacks, assignment := Assign(spec, "texture:doc-1")
	for i := 0; i < 5; i++ {
		again, _, _ := Assign(spec, "texture:doc-1")
		if again.Model != first.Model {
			t.Fatalf("thread moved from %s to %s", first.Model, again.Model)
		}
	}
	pool := modelcatalog.Pools()[modelcatalog.DefaultPool]
	if assignment != "hash" || len(fallbacks) != len(pool)-1 || first.ReasoningEffort != "high" {
		t.Fatalf("assignment %q, chosen %+v, fallbacks %+v", assignment, first, fallbacks)
	}
	for _, f := range fallbacks {
		if f.Model == first.Model {
			t.Fatalf("a thread falls back to its own model: chosen %+v, fallbacks %+v", first, fallbacks)
		}
	}
	seen := map[string]int{}
	for i := 0; i < 400; i++ {
		chosen, _, _ := Assign(spec, fmt.Sprintf("trajectory-%d", i))
		seen[chosen.Model]++
	}
	total := 0
	for _, m := range pool {
		total += m.SlotWeight()
	}
	for _, m := range pool {
		if want := 400 * m.SlotWeight() / total; seen[m.Model] < want/2 {
			t.Fatalf("assignment skewed: %v (want about %d for %s)", seen, want, m.Model)
		}
	}
}

// The default pool is weighted (owner 2026-10-15): gpt-6-luna takes about
// three threads in four and deepseek-v4.1-flash one in four.
func TestAssignDefaultPoolIsWeightedThreeToOne(t *testing.T) {
	spec := provideriface.LLMSelection{Model: "pool:default"}
	seen := map[string]int{}
	const n = 4000
	for i := 0; i < n; i++ {
		chosen, fallbacks, _ := Assign(spec, fmt.Sprintf("thread-%d", i))
		seen[chosen.Model]++
		if len(fallbacks) != 1 || fallbacks[0].Model == chosen.Model {
			t.Fatalf("thread-%d: chosen %+v, fallbacks %+v", i, chosen, fallbacks)
		}
	}
	luna := float64(seen["gpt-6-luna"]) / n
	if luna < 0.70 || luna > 0.80 {
		t.Fatalf("gpt-6-luna share = %.3f (%v), want about 0.75", luna, seen)
	}
}

// A retired model is remapped to the default pool rather than sent to a
// gateway that refuses it; a concrete pool member falls back to the other.
func TestAssignRemapsRetiredModelsAndPairsPoolMembers(t *testing.T) {
	chosen, fallbacks, assignment := Assign(provideriface.LLMSelection{Provider: "chatgpt", Model: "gpt-5.6-luna"}, "traj")
	poolSize := len(modelcatalog.Pools()[modelcatalog.DefaultPool])
	if assignment != "retired_remapped" || chosen.Model == "gpt-5.6-luna" || len(fallbacks) != poolSize-1 {
		t.Fatalf("retired: %q %+v %+v", assignment, chosen, fallbacks)
	}
	chosen, fallbacks, assignment = Assign(provideriface.LLMSelection{Provider: "chatgpt", Model: "gpt-6-luna"}, "traj")
	if assignment != "explicit" || chosen.Model != "gpt-6-luna" || len(fallbacks) != poolSize-1 || fallbacks[0].Model != "deepseek-v4.1-flash" {
		t.Fatalf("explicit member: %q %+v %+v", assignment, chosen, fallbacks)
	}
	chosen, fallbacks, _ = Assign(provideriface.LLMSelection{Provider: "fireworks", Model: "some-model"}, "traj")
	if chosen.Model != "some-model" || len(fallbacks) != 0 {
		t.Fatalf("uncatalogued provider: %+v %+v", chosen, fallbacks)
	}
}

// A policy file from an older platform default is rewritten at load; the
// new file uses the default pool everywhere.
func TestManagerRewritesUnversionedPlatformPolicy(t *testing.T) {
	path := filepath.Join(t.TempDir(), "System", "model-policy.toml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	old := "[defaults]\nfallback_provider = \"chatgpt\"\nfallback_model = \"gpt-5.6-luna\"\n"
	if err := os.WriteFile(path, []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}
	policy, err := NewManager(ManagerConfig{PolicyPath: path}).Load(context.Background(), "owner")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	raw, _ := os.ReadFile(path)
	if !strings.Contains(string(raw), policyVersionLine) || strings.Contains(string(raw), "gpt-5.6") {
		t.Fatalf("policy file not rewritten:\n%s", raw)
	}
	if got := policy.Resolve(agentprofile.Texture); got.Model != "pool:default" || got.ReasoningEffort != "high" {
		t.Fatalf("texture = %+v", got)
	}
}
