package modelpolicy

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
)

// The management task model (management delegation S2a, R2-9). Ways it can
// fail, written before the code:
//   - the role is unknown to the policy, so a task resolves the defaults:
//     the default pool, whose members include a free model, assigned per
//     thread (a free preview took up to 932 s a call on the paper);
//   - an owner-edited policy file that names no task role (the paper's)
//     resolves the defaults the same way;
//   - a policy that sets the task role to a pool, or to a free or preview
//     model, is honoured;
//   - an explicit, non-free provider and model for the role (the owner's
//     override, O5) is ignored.
func TestManagementTaskModelIsPinned(t *testing.T) {
	if NormalizeRole(agentprofile.ManagementTask) != ManagementTaskRole {
		t.Fatalf("NormalizeRole(%q) = %q", agentprofile.ManagementTask, NormalizeRole(agentprofile.ManagementTask))
	}
	pinned := func(source string, p Policy) {
		t.Helper()
		got := p.Resolve(agentprofile.ManagementTask)
		if got.Provider != ManagementTaskProvider || got.Model != ManagementTaskModel {
			t.Fatalf("%s: task selection = %+v, want %s/%s", source, got, ManagementTaskProvider, ManagementTaskModel)
		}
	}
	pinned("platform fallback", fallbackPolicy(provideriface.Config{}))
	paper, err := parsePolicy(`# owner-edited
[defaults]
model = "pool:default"
reasoning = "low"

[roles.management]
provider = "opencode-go"
model = "deepseek-v4.1-flash"

[roles.texture]
provider = "chatgpt"
model = "gpt-6-luna"
`, "/System/model-policy.toml")
	if err != nil {
		t.Fatal(err)
	}
	pinned("owner-edited file with no task role", paper)
	for _, configured := range []string{
		"[roles.management_task]\nmodel = \"pool:default\"\n",
		"[roles.management_task]\nprovider = \"opencode-go\"\nmodel = \"cat-jam-free\"\n",
		"[roles.management_task]\nprovider = \"opencode-go\"\nmodel = \"longcat-2.5-preview-free\"\n",
	} {
		p, err := parsePolicy("[defaults]\nmodel = \"pool:default\"\n\n"+configured, "/System/model-policy.toml")
		if err != nil {
			t.Fatalf("parse %q: %v", configured, err)
		}
		pinned(configured, p)
	}
	override, err := parsePolicy("[defaults]\nmodel = \"pool:default\"\n\n[roles.management_task]\nprovider = \"opencode-go\"\nmodel = \"deepseek-v4.1-flash\"\n", "/System/model-policy.toml")
	if err != nil {
		t.Fatal(err)
	}
	if got := override.Resolve(agentprofile.ManagementTask); got.Provider != "opencode-go" || got.Model != "deepseek-v4.1-flash" {
		t.Fatalf("the owner's override was ignored: %+v", got)
	}
	// Through the manager and the run metadata: one model. Owner
	// 2026-10-28: every desk falls back to the other default-pool model
	// when one is out, so the pin's only fallback is the default pool's
	// other member; never a free or preview model (R2-9).
	dir := t.TempDir()
	path := filepath.Join(dir, "model-policy.toml")
	if err := os.WriteFile(path, []byte("# owner-edited\n[defaults]\nmodel = \"pool:default\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := NewManager(ManagerConfig{PolicyPath: path})
	meta := m.EnrichMetadata(context.Background(), "owner", agentprofile.ManagementTask, map[string]any{"trajectory_id": "t1"})
	if meta[MetadataProvider] != ManagementTaskProvider || meta[MetadataModel] != ManagementTaskModel {
		t.Fatalf("task run metadata = %+v", meta)
	}
	if meta[MetadataFallbacks] != "synthetic/hf:deepseek-ai/DeepSeek-V4.1-Flash-1017" {
		t.Fatalf("the pinned task's fallback = %+v, want the default pool's other member", meta)
	}
	// An owner override naming the other member falls back to the pin's model.
	if err := os.WriteFile(path, []byte("# owner-edited\n[defaults]\nmodel = \"pool:default\"\n\n[roles.management_task]\nprovider = \"synthetic\"\nmodel = \"hf:deepseek-ai/DeepSeek-V4.1-Flash-1017\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	meta = NewManager(ManagerConfig{PolicyPath: path}).EnrichMetadata(context.Background(), "owner", agentprofile.ManagementTask, map[string]any{"trajectory_id": "t1"})
	if meta[MetadataProvider] != "synthetic" || meta[MetadataFallbacks] != "chatgpt/gpt-6-luna" {
		t.Fatalf("synthetic task override metadata = %+v", meta)
	}
	// An override outside the default pool fails over to the default pool's
	// members, never to a free or preview model.
	if err := os.WriteFile(path, []byte("# owner-edited\n[defaults]\nmodel = \"pool:default\"\n\n[roles.management_task]\nprovider = \"opencode-go\"\nmodel = \"deepseek-v4.1-flash\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	meta = NewManager(ManagerConfig{PolicyPath: path}).EnrichMetadata(context.Background(), "owner", agentprofile.ManagementTask, map[string]any{"trajectory_id": "t1"})
	if fb, _ := meta[MetadataFallbacks].(string); fb != "synthetic/hf:deepseek-ai/DeepSeek-V4.1-Flash-1017,chatgpt/gpt-6-luna" {
		t.Fatalf("opencode task override fallback = %+v", meta)
	}
}
