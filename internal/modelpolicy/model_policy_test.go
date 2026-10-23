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
	// The default pool is gpt-6-luna alone tonight (owner 2026-10-23), so
	// a thread has no fallback at all; the fallbacks of a larger pool are
	// the other members in pool order after the assigned one, wrapping.
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
	fallbacks, _ := metadata[MetadataFallbacks].(string)
	if member != "chatgpt/gpt-6-luna" || fallbacks != other || metadata[MetadataSpec] != "pool:default" || metadata[MetadataAssignment] != "hash" || metadata[MetadataThread] != "traj-1" || metadata[MetadataReasoningEffort] != "high" || metadata[MetadataPolicySource] != "platform_fallback" {
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

// Owner 2026-10-22: the paper runs half Synthetic DeepSeek 1017, half
// gpt-6-luna overnight (pool synthetic-luna, 1:1). Owner 2026-10-23: "I'd
// rather them fail than fallback" and "Don't use Opencode as fallback
// tonight." Ways it could fail: one member takes every thread; the split
// drifts from 1:1; a thread gets a fallback (to the other member, to the
// default pool, or to OpenCode); a concrete member of the pool picked by
// name gets one. A pool whose members are all fallback-only still assigns
// its first member.
func TestAssignSyntheticLunaSplitsOneToOneAndNeverFailsOver(t *testing.T) {
	spec := provideriface.LLMSelection{Model: "pool:synthetic-luna", ReasoningEffort: "high"}
	seen := map[string]int{}
	for i := 0; i < 4000; i++ {
		chosen, fallbacks, assignment := Assign(spec, fmt.Sprintf("thread-%d", i))
		seen[chosen.Provider+"/"+chosen.Model]++
		if len(fallbacks) != 0 || assignment != "hash" || chosen.ReasoningEffort != "high" {
			t.Fatalf("thread-%d on %s has fallbacks %+v (%s), want none", i, chosen.Model, fallbacks, assignment)
		}
	}
	syn, luna := seen["synthetic/hf:deepseek-ai/DeepSeek-V4.1-Flash-1017"], seen["chatgpt/gpt-6-luna"]
	if syn < 1800 || syn > 2200 || syn+luna != 4000 {
		t.Fatalf("split = %v, want about 2000 each of 4000", seen)
	}
	// The default pool is gpt-6-luna alone: no thread falls back to OpenCode.
	for i := 0; i < 50; i++ {
		chosen, fallbacks, _ := Assign(provideriface.LLMSelection{Model: "pool:default"}, fmt.Sprintf("thread-%d", i))
		if chosen.Model != "gpt-6-luna" || len(fallbacks) != 0 {
			t.Fatalf("default pool thread-%d: %+v with fallbacks %+v", i, chosen, fallbacks)
		}
	}
	// A concrete member named directly has no fallback either.
	chosen, fallbacks, _ := Assign(provideriface.LLMSelection{Provider: "synthetic", Model: "hf:deepseek-ai/DeepSeek-V4.1-Flash-1017"}, "traj")
	if chosen.Provider != "synthetic" || len(fallbacks) != 0 {
		t.Fatalf("explicit synthetic member: %+v %+v", chosen, fallbacks)
	}
	only := []modelcatalog.PoolMember{{Provider: "chatgpt", Model: "gpt-6-luna", FallbackOnly: true}}
	if got := weightedIndex(only, 7); got != 0 {
		t.Fatalf("all-fallback pool index = %d, want 0", got)
	}
}

// pool:synthetic-luna parses in the computer's model-policy.toml like
// pool:default, and the policy names both members.
func TestPolicyAcceptsSyntheticLunaPool(t *testing.T) {
	policy, err := parsePolicy("# owner-edited\n[defaults]\nmodel = \"pool:synthetic-luna\"\nreasoning_effort = \"low\"\n\n[roles.texture]\nmodel = \"pool:synthetic-luna\"\nreasoning_effort = \"high\"\n", "test")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := policy.Resolve(agentprofile.Texture); got.Model != "pool:synthetic-luna" || got.ReasoningEffort != "high" {
		t.Fatalf("texture = %+v", got)
	}
	models := map[string]bool{}
	for _, s := range policy.Selections() {
		models[s.Provider+"/"+s.Model] = true
	}
	if !models["synthetic/hf:deepseek-ai/DeepSeek-V4.1-Flash-1017"] || !models["chatgpt/gpt-6-luna"] {
		t.Fatalf("policy selections = %v", models)
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
	if assignment != "explicit" || chosen.Model != "gpt-6-luna" || len(fallbacks) != poolSize-1 {
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

// withPools swaps the pool table for one test.
func withPools(t *testing.T, pools map[string][]modelcatalog.PoolMember) {
	t.Helper()
	prev := catalogPools
	catalogPools = func() map[string][]modelcatalog.PoolMember { return pools }
	t.Cleanup(func() { catalogPools = prev })
}

// Owner 2026-10-23 (R2): "I'd rather them fail than fallback." Review bunch
// 147, Medium: no-failover was a property of the pool spec only, so a
// concrete provider/model pin that is a member of the default pool got the
// rest of that pool as fallbacks. Ways it could fail: a pin inherits the
// default pool's other members; a retired-model remap loses its pool
// fallbacks (it asked for a pool, so it keeps them); a pool spec loses its
// own. Fails on the unfixed tip: the explicit pin gets deepseek.
func TestAssignExplicitPinGetsNoDefaultPoolFallbacks(t *testing.T) {
	withPools(t, map[string][]modelcatalog.PoolMember{
		modelcatalog.DefaultPool: {
			{Provider: "chatgpt", Model: "gpt-6-luna", Weight: 1},
			{Provider: "opencode-go", Model: "deepseek-v4.1-flash", Weight: 1},
		},
	})
	chosen, fallbacks, assignment := Assign(provideriface.LLMSelection{Provider: "chatgpt", Model: "gpt-6-luna"}, "traj")
	if assignment != "explicit" || chosen.Model != "gpt-6-luna" || len(fallbacks) != 0 {
		t.Fatalf("explicit pin: %q %+v with fallbacks %+v, want none", assignment, chosen, fallbacks)
	}
	chosen, fallbacks, _ = Assign(provideriface.LLMSelection{Provider: "opencode-go", Model: "deepseek-v4.1-flash"}, "traj")
	if chosen.Model != "deepseek-v4.1-flash" || len(fallbacks) != 0 {
		t.Fatalf("explicit second member: %+v with fallbacks %+v, want none", chosen, fallbacks)
	}
	// A pool spec still fails over inside its pool.
	_, fallbacks, assignment = Assign(provideriface.LLMSelection{Model: "pool:default"}, "traj")
	if assignment != "hash" || len(fallbacks) != 1 {
		t.Fatalf("pool spec: %q fallbacks %+v, want the other member", assignment, fallbacks)
	}
}

// Review bunch 147, High deploy gate: an unknown pool name resolved silently
// as pool:default. Ways it could fail: Assign remaps it; the parser accepts
// it; the Manager loads it without an error; EnrichMetadata runs the role on
// the default model with no policy error; an unknown pool in a role is
// rejected only for that role while the rest of the file is silently kept.
// Fails on the unfixed tip at every step.
func TestUnknownPoolFailsClosedNamingThePool(t *testing.T) {
	chosen, fallbacks, assignment := Assign(provideriface.LLMSelection{Model: "pool:no-such-pool"}, "traj")
	if assignment != "unknown_pool" || chosen.Provider != "" || len(fallbacks) != 0 {
		t.Fatalf("Assign remapped an unknown pool: %q %+v %+v", assignment, chosen, fallbacks)
	}
	if _, err := parsePolicy("[defaults]\nmodel = \"pool:default\"\n\n[roles.texture]\nmodel = \"pool:no-such-pool\"\n", "test"); err == nil || !strings.Contains(err.Error(), "no-such-pool") {
		t.Fatalf("parse accepted an unknown pool or did not name it: %v", err)
	}
	if _, err := parsePolicy("[defaults]\nmodel = \"pool:typo\"\n", "test"); err == nil || !strings.Contains(err.Error(), "typo") {
		t.Fatalf("parse accepted an unknown default pool: %v", err)
	}
	path := filepath.Join(t.TempDir(), "System", "model-policy.toml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("# owner-edited\n[defaults]\nmodel = \"pool:default\"\n\n[roles.texture]\nmodel = \"pool:no-such-pool\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	manager := NewManager(ManagerConfig{PolicyPath: path})
	if _, err := manager.Load(context.Background(), "owner"); err == nil || !strings.Contains(err.Error(), "no-such-pool") {
		t.Fatalf("Load did not report the unknown pool: %v", err)
	}
	meta := manager.EnrichMetadata(context.Background(), "owner", agentprofile.Texture, map[string]any{"trajectory_id": "t1"})
	if errText, _ := meta[MetadataPolicyError].(string); !strings.Contains(errText, "no-such-pool") {
		t.Fatalf("EnrichMetadata hid the unknown pool: policy error = %q", errText)
	}
	// A policy that is valid still carries no error.
	if err := os.WriteFile(path, []byte("# owner-edited\n[defaults]\nmodel = \"pool:synthetic-luna\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Load(context.Background(), "owner"); err != nil {
		t.Fatalf("a known pool must load: %v", err)
	}
}
