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
		agentprofile.Conductor:   {"cancel_agent"},
		agentprofile.Management:  append(slices.Clone(ordinary), "cancel_co_super_assignment", "report_to_texture"),
		agentprofile.Engineering: {},
		agentprofile.Research:    slices.Clone(ordinary),
		agentprofile.Texture:     {"get_run_memory_entry"},
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

func TestTextureRegistryHasNoGenericCancellationOrCapsuleLifecycleAuthority(t *testing.T) {
	rt := &Runtime{capsuleExecutor: new(capsule.Executor)}
	if err := rt.InstallDefaultAgentTools(t.TempDir()); err != nil {
		t.Fatalf("install tools: %v", err)
	}
	registry := rt.ToolRegistryForProfile(agentprofile.Texture)
	for _, name := range []string{"cancel_agent", "assign_co_super", "cancel_co_super_assignment", "spawn_capsule", "destroy_capsule", "capsule_exec"} {
		if _, ok := registry.Lookup(name); ok {
			t.Fatalf("Texture exposes forbidden generic cancellation/capsule tool %q", name)
		}
	}
}

func TestDelegatedEngineeringCannotReachHostEffectToolsOrCallbacks(t *testing.T) {
	rt := &Runtime{capsuleExecutor: new(capsule.Executor)}
	if err := rt.InstallDefaultAgentTools(t.TempDir()); err != nil {
		t.Fatalf("install tools: %v", err)
	}
	registry := rt.ToolRegistryForProfile(agentprofile.Engineering)

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

func TestAssignedEngineeringBuilderIsExactClosedSet(t *testing.T) {
	registry, err := buildAssignedEngineeringRegistry(nil)
	if err != nil {
		t.Fatalf("build assigned registry: %v", err)
	}
	want := []string{"capsule_go_eval"}
	if got := registryToolNames(registry); !slices.Equal(got, want) {
		t.Fatalf("assigned registry tools = %v, want exact %v", got, want)
	}
	for _, absent := range []string{"capsule_exec", "capsule_read_file", "capsule_write_file", "capsule_list_dir", "read_file", "glob", "grep", "save_evidence", "verify_model_capability", "spawn_agent", "spawn_capsule", "destroy_capsule", "propose_effect", "finalize_effect", "materialize_self_development", "create_checkpoint"} {
		if _, ok := registry.Lookup(absent); ok {
			t.Fatalf("assigned registry inherited forbidden callback %q", absent)
		}
	}
}

func TestCapsuleLocalInstallerIsExact(t *testing.T) {
	capsuleLocal := toolregistry.MustNewToolRegistry()
	if err := RegisterCapsuleLocalTools(capsuleLocal, nil); err != nil {
		t.Fatalf("register capsule-local tools: %v", err)
	}
	if got, want := registryToolNames(capsuleLocal), []string{"capsule_go_eval"}; !slices.Equal(got, want) {
		t.Fatalf("capsule-local tools = %v, want %v", got, want)
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

func TestAssignedEngineeringSchemaHasNoModelAuthoredRuntimeBindings(t *testing.T) {
	registry := toolregistry.MustNewToolRegistry()
	if err := RegisterAssignedEngineeringTools(registry, &Runtime{}); err != nil {
		t.Fatal(err)
	}
	if _, ok := registry.Lookup("assign_co_super"); ok {
		t.Fatal("assign_co_super remains registered; the document channel is the opener")
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

func TestAssignmentIdentityUsesOnlyDocumentRevisionAndKind(t *testing.T) {
	ownerID, computerID, trajectoryID, revisionID := "owner", "computer", "trajectory", "revision"
	left := deterministicDocumentAssignmentIdentity(ownerID, computerID, trajectoryID, revisionID, types.EngineeringAssignmentImplementation, "")
	right := deterministicDocumentAssignmentIdentity(ownerID, computerID, trajectoryID, revisionID, types.EngineeringAssignmentVerification, "candidate")
	if left == right {
		t.Fatal("assignment kind absent from document assignment identity")
	}
	if left == deterministicDocumentAssignmentIdentity(ownerID, computerID, trajectoryID, "other-revision", types.EngineeringAssignmentImplementation, "") {
		t.Fatal("admitting revision absent from assignment identity")
	}
	if left == deterministicDocumentAssignmentIdentity(ownerID, computerID, "other-trajectory", revisionID, types.EngineeringAssignmentImplementation, "") {
		t.Fatal("trajectory absent from assignment identity")
	}
	if left != deterministicDocumentAssignmentIdentity(ownerID, computerID, trajectoryID, revisionID, types.EngineeringAssignmentImplementation, "") {
		t.Fatal("document assignment identity is not deterministic")
	}
}

func TestStartCoagentRunHardRefusesEngineeringForEveryCaller(t *testing.T) {
	s, err := openTestStore(filepath.Join(t.TempDir(), "runtime.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	now := time.Now().UTC()
	parent := types.RunRecord{RunID: "parent", AgentID: "management:owner", AgentProfile: agentprofile.Management, AgentRole: agentprofile.Management, OwnerID: "owner", ComputerID: "computer", State: types.RunRunning, CreatedAt: now, UpdatedAt: now}
	if err := s.CreateRun(context.Background(), parent); err != nil {
		t.Fatal(err)
	}
	rt := &Runtime{store: s, cfg: provideriface.Config{ComputerID: "computer"}}
	for _, constraints := range []map[string]any{
		{runMetadataAgentProfile: agentprofile.Engineering, runMetadataAgentRole: agentprofile.Engineering},
		{runMetadataAgentRole: "engineering"},
	} {
		if _, err := rt.StartCoagentRun(context.Background(), parent.RunID, "forbidden", parent.OwnerID, constraints); err == nil || !strings.Contains(err.Error(), "refuses all Engineering") {
			t.Fatalf("generic Engineering activation error=%v", err)
		}
	}
}

func TestAssignedEngineeringPromptNamesExactKindWithoutFutureToolLie(t *testing.T) {
	t.Setenv(capsule.ActuatorEnvVar, capsule.ActuatorRLM)
	rt := &Runtime{}
	for _, kind := range []types.EngineeringAssignmentKind{types.EngineeringAssignmentImplementation, types.EngineeringAssignmentVerification} {
		rec := &types.RunRecord{RunID: "assigned", AgentID: "engineering:assigned", AgentProfile: agentprofile.Engineering, AgentRole: agentprofile.Engineering, Metadata: map[string]any{"assignment_id": "assignment", "assignment_kind": string(kind), "subject_digest": "sha256:subject", "source_candidate_id": "candidate"}}
		prompt, err := rt.systemPromptForRun(rec)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(prompt, "kind="+string(kind)) {
			t.Fatalf("prompt does not name exact %s assignment: %s", kind, prompt)
		}
		if !strings.Contains(prompt, "choir.Message") {
			t.Fatalf("assigned Engineering prompt omits the in-cell report channel: %s", prompt)
		}
		for _, name := range []string{"commit_transaction", "inspect_self_development_bundle", "record_self_development_verification", "update_coagent", "record_assignment_result"} {
			if strings.Contains(prompt, name) {
				t.Fatalf("assigned Engineering prompt still names retired tool %s: %s", name, prompt)
			}
		}
		if strings.Contains(prompt, "may be added later") || strings.Contains(prompt, "report one precise result through update_coagent") {
			t.Fatalf("prompt retains future/static tool lie: %s", prompt)
		}
	}
}

func TestPersistentManagementReportToolDoesNotDependOnCapsuleExecutor(t *testing.T) {
	rt := &Runtime{}
	if err := rt.InstallDefaultAgentTools(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	registry := rt.ToolRegistryForProfile(agentprofile.Management)
	if _, ok := registry.Lookup("report_to_texture"); !ok {
		t.Fatal("persistent Management lacks capsule-independent report_to_texture")
	}
	for _, name := range []string{"assign_co_super", "cancel_co_super_assignment", "spawn_capsule"} {
		if _, ok := registry.Lookup(name); ok {
			t.Fatalf("capsule-unavailable Management exposes %s", name)
		}
	}
}

// TestRLMAssignedEngineeringOverlayIsSealedGo is Def 2 item 4 schema derivation:
// under the RLM route the assigned Engineering model schema keeps capsule_go_eval
// as the sole capsule-effect entry plus host reconciliation channels, with
// the JSON exec/file tools removed (subsumed by in-cell choir ops).
func TestRLMAssignedEngineeringOverlayIsSealedGo(t *testing.T) {
	t.Setenv(capsule.ActuatorEnvVar, capsule.ActuatorRLM)
	if !capsule.HostSelectsRLM() {
		t.Fatal("host route authority did not select RLM")
	}
	registry, err := buildAssignedEngineeringRegistry(nil)
	if err != nil {
		t.Fatalf("build RLM assigned registry: %v", err)
	}
	want := []string{"capsule_go_eval"}
	if got := registryToolNames(registry); !slices.Equal(got, want) {
		t.Fatalf("RLM assigned registry tools = %v, want exact %v", got, want)
	}
	for _, absent := range []string{"capsule_exec", "capsule_read_file", "capsule_write_file", "capsule_list_dir", "commit_transaction", "inspect_self_development_bundle", "record_self_development_verification", "update_coagent", "record_assignment_result"} {
		if _, ok := registry.Lookup(absent); ok {
			t.Fatalf("RLM registry kept JSON capsule tool %q", absent)
		}
	}
}
