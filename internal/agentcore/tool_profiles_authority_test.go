package agentcore

import (
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/capsule"
)

func TestSelfDevelopmentRolesExposeOnlyCapsuleEffects(t *testing.T) {
	rt := &Runtime{capsuleExecutor: new(capsule.Executor)}
	if err := rt.InstallDefaultAgentTools(t.TempDir()); err != nil {
		t.Fatalf("install tools: %v", err)
	}

	super := rt.ToolRegistryForProfile(agentprofile.Super)
	for _, required := range []string{"read_file", "update_coagent", "spawn_capsule", "destroy_capsule", "inspect_capsule"} {
		if _, ok := super.Lookup(required); !ok {
			t.Errorf("super missing %q", required)
		}
	}
	for _, forbidden := range []string{"bash", "write_file", "edit_file", "publish_app_change_package", "fork_desktop", "publish_desktop", "request_worker_vm", "start_worker_delegation", "capsule_exec", "capsule_write_file", "commit_transaction", "record_self_development_verification"} {
		if _, ok := super.Lookup(forbidden); ok {
			t.Errorf("super exposes forbidden effect tool %q", forbidden)
		}
	}

	coSuper := rt.ToolRegistryForProfile(agentprofile.CoSuper)
	for _, required := range []string{"update_coagent", "capsule_exec", "capsule_read_file", "capsule_write_file", "capsule_list_dir", "commit_transaction", "record_self_development_verification"} {
		if _, ok := coSuper.Lookup(required); !ok {
			t.Errorf("co-super missing %q", required)
		}
	}
	for _, forbidden := range []string{"bash", "read_file", "write_file", "spawn_agent", "save_evidence", "publish_app_change_package", "spawn_capsule", "destroy_capsule", "request_worker_vm"} {
		if _, ok := coSuper.Lookup(forbidden); ok {
			t.Errorf("co-super exposes forbidden direct tool %q", forbidden)
		}
	}
}
