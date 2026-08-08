package agentcore

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
)

func TestDefaultProfileRegistriesExactAuthorityContract(t *testing.T) {
	rt := &Runtime{capsuleExecutor: new(capsule.Executor)}
	if err := rt.InstallDefaultAgentTools(t.TempDir()); err != nil {
		t.Fatalf("install tools: %v", err)
	}

	ordinary := []string{
		"cancel_agent", "fetch_url", "get_run_memory_entry", "glob", "grep",
		"import_document_content", "import_url_content", "list_content_item_selectors",
		"list_evidence", "read_content_item", "read_content_item_selector", "read_evidence",
		"read_file", "save_evidence", "search_wire_corpus", "source_search",
		"verify_model_capability", "web_search",
	}
	expected := map[string][]string{
		agentprofile.Conductor: {"cancel_agent"},
		agentprofile.Super: append(append(slices.Clone(ordinary),
			"update_coagent"), "assign_co_super", "cancel_co_super_assignment"),
		agentprofile.CoSuper:    {},
		agentprofile.Researcher: append(slices.Clone(ordinary), "update_coagent"),
		agentprofile.Texture:    {"cancel_agent", "get_run_memory_entry"},
		agentprofile.Processor: append(append(slices.Clone(ordinary), "update_coagent"),
			"record_wire_processor_decision"),
		agentprofile.Reconciler: append(slices.Clone(ordinary), "update_coagent"),
		agentprofile.Email:      {},
	}
	for profile, want := range expected {
		profile, want := profile, want
		t.Run(profile, func(t *testing.T) {
			slices.Sort(want)
			got := registryToolNames(rt.ToolRegistryForProfile(profile))
			if !slices.Equal(got, want) {
				t.Fatalf("%s registry tools = %v, want exact authority set %v", profile, got, want)
			}
		})
	}
}

func TestDelegatedCoSuperCannotReachHostEffectToolsOrCallbacks(t *testing.T) {
	rt := &Runtime{capsuleExecutor: new(capsule.Executor)}
	if err := rt.InstallDefaultAgentTools(t.TempDir()); err != nil {
		t.Fatalf("install tools: %v", err)
	}
	registry := rt.ToolRegistryForProfile(agentprofile.CoSuper)

	// Keep the full prohibited authority vocabulary explicit. The exact-set
	// contract above rejects every unlisted tool as well; this table documents
	// the security-significant host callbacks that must stay unreachable.
	forbidden := []string{
		// Self-development verification, event append, proposal, and finalization.
		"inspect_self_development_bundle", "record_self_development_verification",
		"append_computer_event", "commit_transaction", "propose_effect", "finalize_effect",
		// Acceptance, materialization, checkpoints, routing, VM, and host paths.
		"synthesize_run_acceptance", "accept_run", "materialize_self_development",
		"create_checkpoint", "project_checkpoint", "route_candidate", "change_route",
		"start_vm", "stop_vm", "restart_vm", "read_host_file", "write_host_file",
		// Owner decisions and product-authority writes.
		"record_texture_decision", "record_wire_processor_decision", "patch_texture",
		"rewrite_texture", "request_email_draft", "product_api_request",
		// Capsule lifecycle and capsule-local execution are absent from the static
		// host registry; exact assignments receive a per-run overlay only.
		"spawn_capsule", "destroy_capsule", "list_capsules", "inspect_capsule",
		"capsule_exec", "capsule_read_file", "capsule_write_file", "capsule_list_dir", "record_assignment_result",
		// Agent lifecycle authority is distinct from result reporting.
		"spawn_agent", "cancel_agent",
	}
	for _, name := range forbidden {
		name := name
		t.Run(name, func(t *testing.T) {
			if _, ok := registry.Lookup(name); ok {
				t.Fatalf("delegated co-super exposes forbidden tool %q", name)
			}
			if _, err := registry.Execute(context.Background(), name, json.RawMessage(`{}`)); err == nil || !strings.Contains(err.Error(), "not found") {
				t.Fatalf("execute forbidden tool %q error = %v, want registry rejection", name, err)
			}
		})
	}
}

func TestAssignedCoSuperBuilderIsExactClosedSet(t *testing.T) {
	registry, err := buildAssignedCoSuperRegistry(nil)
	if err != nil {
		t.Fatalf("build assigned registry: %v", err)
	}
	want := []string{"capsule_exec", "capsule_list_dir", "capsule_read_file", "capsule_write_file", "record_assignment_result"}
	if got := registryToolNames(registry); !slices.Equal(got, want) {
		t.Fatalf("assigned registry tools = %v, want exact %v", got, want)
	}
	for _, absent := range []string{"read_file", "glob", "grep", "save_evidence", "verify_model_capability", "update_coagent", "spawn_agent", "spawn_capsule", "destroy_capsule"} {
		if _, ok := registry.Lookup(absent); ok {
			t.Fatalf("assigned registry inherited forbidden callback %q", absent)
		}
	}
}

func TestCapsuleLocalAndHostSelfDevelopmentInstallersAreDisjoint(t *testing.T) {
	capsuleLocal := toolregistry.MustNewToolRegistry()
	if err := RegisterCapsuleLocalTools(capsuleLocal, nil); err != nil {
		t.Fatalf("register capsule-local tools: %v", err)
	}
	if got, want := registryToolNames(capsuleLocal), []string{
		"capsule_exec", "capsule_list_dir", "capsule_read_file", "capsule_write_file", "record_assignment_result",
	}; !slices.Equal(got, want) {
		t.Fatalf("capsule-local tools = %v, want %v", got, want)
	}

	hostSelfDevelopment := toolregistry.MustNewToolRegistry()
	if err := registerHostSelfDevelopmentTools(hostSelfDevelopment); err != nil {
		t.Fatalf("register host self-development tools: %v", err)
	}
	if got, want := registryToolNames(hostSelfDevelopment), []string{
		"commit_transaction", "inspect_self_development_bundle", "record_self_development_verification",
	}; !slices.Equal(got, want) {
		t.Fatalf("host self-development tools = %v, want %v", got, want)
	}
}

func registryToolNames(registry *toolregistry.ToolRegistry) []string {
	if registry == nil {
		return nil
	}
	tools := registry.Tools()
	names := make([]string, len(tools))
	for index, tool := range tools {
		names[index] = tool.Name
	}
	return names
}
