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
	override, err := parsePolicy("[defaults]\nmodel = \"pool:default\"\n\n[roles.management_task]\nprovider = \"chatgpt\"\nmodel = \"gpt-6-luna\"\n", "/System/model-policy.toml")
	if err != nil {
		t.Fatal(err)
	}
	if got := override.Resolve(agentprofile.ManagementTask); got.Provider != "chatgpt" || got.Model != "gpt-6-luna" {
		t.Fatalf("the owner's override was ignored: %+v", got)
	}
	// Through the manager and the run metadata: one model, no failover
	// list (a pinned task never wraps to a pool member).
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
	if _, ok := meta[MetadataFallbacks]; ok {
		t.Fatalf("a pinned task model has fallbacks: %+v", meta)
	}
}
