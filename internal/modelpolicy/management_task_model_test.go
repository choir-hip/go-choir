package modelpolicy

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
)

// Owner 2026-11-05: the management task runs on the pool like every other
// role, "to spread the load, and also to see which models fail at this".
// Ways it could fail: the task still resolves a platform pin; an owner's
// explicit model or pool for the role is ignored; the task's run carries
// no fallbacks, so a quota fails it instead of moving it along the pool.
func TestManagementTaskResolvesLikeAnyRole(t *testing.T) {
	if NormalizeRole(agentprofile.ManagementTask) != ManagementTaskRole {
		t.Fatalf("NormalizeRole(%q) = %q", agentprofile.ManagementTask, NormalizeRole(agentprofile.ManagementTask))
	}
	for _, text := range []string{
		"[defaults]\nmodel = \"pool:default\"\n",
		"# owner-edited\n[defaults]\nmodel = \"pool:default\"\n",
	} {
		p, err := parsePolicy(text, "/System/model-policy.toml")
		if err != nil {
			t.Fatal(err)
		}
		if got := p.Resolve(agentprofile.ManagementTask); got.Model != "pool:default" {
			t.Fatalf("management task resolves %+v from %q, want the default pool", got, text)
		}
	}
	override, err := parsePolicy("[defaults]\nmodel = \"pool:default\"\n\n[roles.management_task]\nprovider = \"chatgpt\"\nmodel = \"gpt-6-luna\"\n", "/System/model-policy.toml")
	if err != nil {
		t.Fatal(err)
	}
	if got := override.Resolve(agentprofile.ManagementTask); got.Provider != "chatgpt" || got.Model != "gpt-6-luna" {
		t.Fatalf("the owner's override was ignored: %+v", got)
	}
	path := filepath.Join(t.TempDir(), "model-policy.toml")
	if err := os.WriteFile(path, []byte("# owner-edited\n[defaults]\nmodel = \"pool:default\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	meta := NewManager(ManagerConfig{PolicyPath: path}).EnrichMetadata(context.Background(), "owner", agentprofile.ManagementTask, map[string]any{"trajectory_id": "t1"})
	if meta[MetadataSpec] != "pool:default" || meta[MetadataAssignment] != "hash" || meta[MetadataFallbacks] == nil || meta[MetadataFallbacks] == "" {
		t.Fatalf("task run metadata = %+v, want a pool assignment with fallbacks", meta)
	}
}
