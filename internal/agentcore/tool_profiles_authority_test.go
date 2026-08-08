package agentcore

import (
	"context"
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
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

func TestAssignCoSuperSchemaHasNoModelAuthoredRuntimeBindings(t *testing.T) {
	registry := toolregistry.MustNewToolRegistry()
	if err := RegisterAssignedCoSuperTools(registry, &Runtime{}); err != nil {
		t.Fatal(err)
	}
	tool, ok := registry.Lookup("assign_co_super")
	if !ok {
		t.Fatal("assign_co_super missing")
	}
	properties, _ := tool.Parameters["properties"].(map[string]any)
	for _, forbidden := range []string{"attempt", "scope_digest", "subject_digest", "assignment_id", "capsule_id"} {
		if _, present := properties[forbidden]; present {
			t.Fatalf("model-authored runtime binding %q remains in schema", forbidden)
		}
	}
	for _, required := range []string{"objective", "kind", "parent_work_item_id", "candidate_id"} {
		if _, present := properties[required]; !present {
			t.Fatalf("assignment semantic field %q missing", required)
		}
	}
	cancel, ok := registry.Lookup("cancel_co_super_assignment")
	if !ok {
		t.Fatal("cancel assignment missing")
	}
	cancelProperties, _ := cancel.Parameters["properties"].(map[string]any)
	if _, present := cancelProperties["attempt"]; present {
		t.Fatal("cancel schema retains model-authored attempt")
	}
}

func TestAssignmentIdentityUsesOnlyAuthenticatedParentRunAndToolCall(t *testing.T) {
	parent := types.RunRecord{RunID: "parent-run", OwnerID: "owner", SandboxID: "computer"}
	left := StartAssignedCoSuperRequest{Objective: "one", Kind: types.CoSuperAssignmentImplementation, ParentWorkItemID: "work", ToolCallID: "call"}
	right := StartAssignedCoSuperRequest{Objective: "changed", Kind: types.CoSuperAssignmentVerification, CandidateID: "candidate", ParentWorkItemID: "other", ToolCallID: "call"}
	if deterministicAssignmentIdentity(parent, left) != deterministicAssignmentIdentity(parent, right) {
		t.Fatal("semantic arguments changed authenticated assignment identity")
	}
	other := parent
	other.RunID = "other-parent"
	if deterministicAssignmentIdentity(parent, left) == deterministicAssignmentIdentity(other, left) {
		t.Fatal("parent run absent from assignment identity")
	}
	right.ToolCallID = "other-call"
	if deterministicAssignmentIdentity(parent, left) == deterministicAssignmentIdentity(parent, right) {
		t.Fatal("tool call absent from assignment identity")
	}
}

func TestStartCoagentRunHardRefusesCoSuperForEveryCaller(t *testing.T) {
	s, err := openTestStore(filepath.Join(t.TempDir(), "runtime.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	now := time.Now().UTC()
	parent := types.RunRecord{RunID: "parent", AgentID: "super:owner", AgentProfile: agentprofile.Super, AgentRole: agentprofile.Super, OwnerID: "owner", SandboxID: "computer", State: types.RunRunning, CreatedAt: now, UpdatedAt: now}
	if err := s.CreateRun(context.Background(), parent); err != nil {
		t.Fatal(err)
	}
	rt := &Runtime{store: s, cfg: provideriface.Config{SandboxID: "computer"}}
	for _, constraints := range []map[string]any{
		{runMetadataAgentProfile: agentprofile.CoSuper, runMetadataAgentRole: agentprofile.CoSuper},
		{runMetadataAgentRole: "coagent"},
	} {
		if _, err := rt.StartCoagentRun(context.Background(), parent.RunID, "forbidden", parent.OwnerID, constraints); err == nil || !strings.Contains(err.Error(), "refuses all CoSuper") {
			t.Fatalf("generic CoSuper activation error=%v", err)
		}
	}
}

func TestAssignedCoSuperPromptNamesExactKindWithoutFutureToolLie(t *testing.T) {
	rt := &Runtime{}
	for _, kind := range []types.CoSuperAssignmentKind{types.CoSuperAssignmentImplementation, types.CoSuperAssignmentVerification} {
		rec := &types.RunRecord{RunID: "assigned", AgentID: "co-super:assigned", AgentProfile: agentprofile.CoSuper, AgentRole: agentprofile.CoSuper, Metadata: map[string]any{"assignment_id": "assignment", "assignment_kind": string(kind), "subject_digest": "sha256:subject", "source_candidate_id": "candidate"}}
		prompt, err := rt.systemPromptForRun(rec)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(prompt, "kind="+string(kind)) {
			t.Fatalf("prompt does not name exact %s assignment: %s", kind, prompt)
		}
		if strings.Contains(prompt, "may be added later") || strings.Contains(prompt, "report one precise result through update_coagent") {
			t.Fatalf("prompt retains future/static tool lie: %s", prompt)
		}
	}
}
