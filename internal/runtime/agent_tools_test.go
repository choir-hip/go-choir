package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/promotion"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

func toolSchemaStringEnum(schema map[string]any, property string) []string {
	props, _ := schema["properties"].(map[string]any)
	prop, _ := props[property].(map[string]any)
	rawEnum, _ := prop["enum"].([]string)
	if len(rawEnum) > 0 {
		return rawEnum
	}
	rawAny, _ := prop["enum"].([]any)
	out := make([]string, 0, len(rawAny))
	for _, item := range rawAny {
		if s, _ := item.(string); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func TestInstallDefaultAgentToolsProfiles(t *testing.T) {
	rt, _, cwd := testRuntimeWithTempCWD(t)
	if err := rt.InstallDefaultAgentTools(cwd); err != nil {
		t.Fatalf("install default agent tools: %v", err)
	}

	super := rt.ToolRegistryForProfile(AgentProfileSuper)
	coSuper := rt.ToolRegistryForProfile(AgentProfileCoSuper)
	vSuper := rt.ToolRegistryForProfile(AgentProfileVSuper)
	conductor := rt.ToolRegistryForProfile(AgentProfileConductor)
	researcher := rt.ToolRegistryForProfile(AgentProfileResearcher)
	vtext := rt.ToolRegistryForProfile(AgentProfileVText)

	for _, name := range []string{"bash", "read_file", "web_search", "spawn_agent", "cast_agent", "save_evidence", "submit_worker_update", "export_patchset", "fork_desktop", "publish_desktop", "request_worker_vm", "delegate_worker_vm"} {
		if _, ok := super.Lookup(name); !ok {
			t.Fatalf("super missing tool %q", name)
		}
	}
	for _, name := range []string{"bash", "read_file", "web_search", "spawn_agent", "cast_agent", "save_evidence", "submit_worker_update", "export_patchset"} {
		if _, ok := coSuper.Lookup(name); !ok {
			t.Fatalf("co-super missing tool %q", name)
		}
	}
	if _, ok := coSuper.Lookup("fork_desktop"); ok {
		t.Fatalf("co-super should not have fork_desktop")
	}
	if _, ok := coSuper.Lookup("publish_desktop"); ok {
		t.Fatalf("co-super should not have publish_desktop")
	}
	if _, ok := coSuper.Lookup("request_worker_vm"); ok {
		t.Fatalf("co-super should not have request_worker_vm")
	}
	if _, ok := coSuper.Lookup("delegate_worker_vm"); ok {
		t.Fatalf("co-super should not have delegate_worker_vm")
	}
	for _, name := range []string{"bash", "read_file", "web_search", "spawn_agent", "cast_agent", "save_evidence", "submit_worker_update", "export_patchset"} {
		if _, ok := vSuper.Lookup(name); !ok {
			t.Fatalf("vsuper missing tool %q", name)
		}
	}
	if _, ok := vSuper.Lookup("fork_desktop"); ok {
		t.Fatalf("vsuper should not have fork_desktop")
	}
	if _, ok := vSuper.Lookup("publish_desktop"); ok {
		t.Fatalf("vsuper should not have publish_desktop")
	}
	if _, ok := vSuper.Lookup("request_worker_vm"); ok {
		t.Fatalf("vsuper should not have request_worker_vm")
	}
	if _, ok := vSuper.Lookup("delegate_worker_vm"); ok {
		t.Fatalf("vsuper should not have delegate_worker_vm")
	}
	for _, name := range []string{"spawn_agent", "cast_agent", "cancel_agent"} {
		if _, ok := conductor.Lookup(name); !ok {
			t.Fatalf("conductor missing tool %q", name)
		}
	}
	if _, ok := conductor.Lookup("bash"); ok {
		t.Fatalf("conductor should not have bash")
	}
	if _, ok := conductor.Lookup("web_search"); ok {
		t.Fatalf("conductor should not have web_search")
	}

	if _, ok := researcher.Lookup("bash"); ok {
		t.Fatalf("researcher should not have bash")
	}
	if _, ok := researcher.Lookup("write_file"); ok {
		t.Fatalf("researcher should not have write_file")
	}
	if _, ok := researcher.Lookup("edit_file"); ok {
		t.Fatalf("researcher should not have edit_file")
	}
	if _, ok := researcher.Lookup("edit_vtext"); ok {
		t.Fatalf("researcher should not have edit_vtext")
	}
	if _, ok := researcher.Lookup("export_patchset"); ok {
		t.Fatalf("researcher should not have export_patchset")
	}
	if _, ok := researcher.Lookup("delegate_worker_vm"); ok {
		t.Fatalf("researcher should not have delegate_worker_vm")
	}
	for _, name := range []string{"read_file", "web_search", "cast_agent", "cancel_agent", "save_evidence", "submit_research_findings", "submit_worker_update"} {
		if _, ok := researcher.Lookup(name); !ok {
			t.Fatalf("researcher missing tool %q", name)
		}
	}
	if _, ok := researcher.Lookup("spawn_agent"); ok {
		t.Fatalf("researcher should not have spawn_agent")
	}
	for _, name := range []string{"spawn_agent", "cast_agent", "cancel_agent", "save_evidence", "read_evidence", "edit_vtext", "request_super_execution"} {
		if _, ok := vtext.Lookup(name); !ok {
			t.Fatalf("vtext missing tool %q", name)
		}
	}
	if _, ok := vtext.Lookup("bash"); ok {
		t.Fatalf("vtext should not have bash")
	}
	if _, ok := vtext.Lookup("web_search"); ok {
		t.Fatalf("vtext should not have web_search")
	}
	if _, ok := vtext.Lookup("submit_research_findings"); ok {
		t.Fatalf("vtext should not have submit_research_findings")
	}
	if _, ok := vtext.Lookup("submit_worker_update"); ok {
		t.Fatalf("vtext should not have submit_worker_update")
	}
	if _, ok := vtext.Lookup("export_patchset"); ok {
		t.Fatalf("vtext should not have export_patchset")
	}
	if _, ok := vtext.Lookup("delegate_worker_vm"); ok {
		t.Fatalf("vtext should not have delegate_worker_vm")
	}
	if _, ok := conductor.Lookup("edit_vtext"); ok {
		t.Fatalf("conductor should not have edit_vtext")
	}
	if _, ok := conductor.Lookup("export_patchset"); ok {
		t.Fatalf("conductor should not have export_patchset")
	}
	if _, ok := conductor.Lookup("delegate_worker_vm"); ok {
		t.Fatalf("conductor should not have delegate_worker_vm")
	}
	if _, ok := super.Lookup("edit_vtext"); ok {
		t.Fatalf("super should not have edit_vtext")
	}
}

func TestForegroundSuperMutationGuardBlocksWritableTools(t *testing.T) {
	rt, _, cwd := testRuntimeWithTempCWD(t)
	if err := rt.InstallDefaultAgentTools(cwd); err != nil {
		t.Fatalf("install default agent tools: %v", err)
	}
	t.Setenv("RUNTIME_SUPER_FOREGROUND_MUTATION_MODE", "worker_only")

	superRun, err := rt.StartRunWithMetadata(context.Background(), "try foreground mutation", "user-alice", map[string]any{
		runMetadataAgentProfile: AgentProfileSuper,
		runMetadataAgentRole:    AgentProfileSuper,
	})
	if err != nil {
		t.Fatalf("start super run: %v", err)
	}
	superRegistry := rt.ToolRegistryForProfile(AgentProfileSuper)
	if _, err := superRegistry.Execute(WithToolExecutionContext(context.Background(), superRun), "bash", json.RawMessage(`{"command":"touch should-not-exist"}`)); err == nil || !strings.Contains(err.Error(), "blocked for foreground super") {
		t.Fatalf("super bash error = %v, want foreground mutation guard", err)
	}
	if _, err := superRegistry.Execute(WithToolExecutionContext(context.Background(), superRun), "write_file", json.RawMessage(`{"path":"should-not-exist.txt","content":"blocked"}`)); err == nil || !strings.Contains(err.Error(), "blocked for foreground super") {
		t.Fatalf("super write_file error = %v, want foreground mutation guard", err)
	}

	workerRun, err := rt.StartRunWithMetadata(context.Background(), "worker mutation", "user-alice", map[string]any{
		runMetadataAgentProfile: AgentProfileCoSuper,
		runMetadataAgentRole:    AgentProfileCoSuper,
		runMetadataToolCWD:      cwd,
	})
	if err != nil {
		t.Fatalf("start worker run: %v", err)
	}
	workerRegistry := rt.ToolRegistryForProfile(AgentProfileCoSuper)
	if _, err := workerRegistry.Execute(WithToolExecutionContext(context.Background(), workerRun), "write_file", json.RawMessage(`{"path":"worker-ok.txt","content":"allowed"}`)); err != nil {
		t.Fatalf("co-super write_file should be allowed: %v", err)
	}
}

func TestCoagentToolsSupportAddressedCastAcrossProfiles(t *testing.T) {
	rt, s, cwd := testRuntimeWithTempCWD(t)
	if err := rt.InstallDefaultAgentTools(cwd); err != nil {
		t.Fatalf("install default agent tools: %v", err)
	}

	parent, err := rt.StartRunWithMetadata(context.Background(), "coordinate work", "user-alice", map[string]any{
		runMetadataAgentProfile: AgentProfileSuper,
		runMetadataAgentRole:    AgentProfileSuper,
	})
	if err != nil {
		t.Fatalf("submit parent task: %v", err)
	}
	time.Sleep(50 * time.Millisecond)

	superRegistry := rt.ToolRegistryForProfile(AgentProfileSuper)
	spawnRaw, err := superRegistry.Execute(WithToolExecutionContext(context.Background(), parent), "spawn_agent", json.RawMessage(`{
		"objective":"research the codebase and report back",
		"role":"researcher",
		"channel_id":"shared-work"
	}`))
	if err != nil {
		t.Fatalf("spawn_agent: %v", err)
	}

	var spawnResp struct {
		RunID     string `json:"loop_id"`
		ChannelID string `json:"channel_id"`
		Profile   string `json:"profile"`
	}
	if err := json.Unmarshal([]byte(spawnRaw), &spawnResp); err != nil {
		t.Fatalf("decode spawn response: %v", err)
	}
	if spawnResp.Profile != AgentProfileResearcher {
		t.Fatalf("spawned profile = %q, want %q", spawnResp.Profile, AgentProfileResearcher)
	}
	if spawnResp.ChannelID != "shared-work" {
		t.Fatalf("spawned channel_id = %q, want shared-work", spawnResp.ChannelID)
	}

	child, err := s.GetRun(context.Background(), spawnResp.RunID)
	if err != nil {
		t.Fatalf("get child task: %v", err)
	}
	if got := child.Metadata[runMetadataAgentProfile]; got != AgentProfileResearcher {
		t.Fatalf("child agent_profile = %v, want %q", got, AgentProfileResearcher)
	}
	if child.ChannelID != "shared-work" {
		t.Fatalf("child channel_id = %q, want shared-work", child.ChannelID)
	}

	postRaw, err := superRegistry.Execute(
		WithToolExecutionContext(context.Background(), parent),
		"cast_agent",
		json.RawMessage(fmt.Sprintf(`{
		"agent_id":"%s",
		"channel_id":"shared-work",
		"content":"please inspect the runtime tool wiring"
	}`, child.AgentID)),
	)
	if err != nil {
		t.Fatalf("cast_agent: %v", err)
	}
	var postResp struct {
		Cursor uint64 `json:"cursor"`
	}
	if err := json.Unmarshal([]byte(postRaw), &postResp); err != nil {
		t.Fatalf("decode post response: %v", err)
	}

	msgs, _, err := rt.ChannelRead("shared-work", 0)
	if err != nil {
		t.Fatalf("channel read: %v", err)
	}
	if len(msgs) != 1 || msgs[0].Content != "please inspect the runtime tool wiring" {
		t.Fatalf("unexpected channel messages: %+v", msgs)
	}
	if msgs[0].ToAgentID != child.AgentID {
		t.Fatalf("channel message to_agent_id = %q, want %q", msgs[0].ToAgentID, child.AgentID)
	}
	deliveries, err := s.ListPendingInboxDeliveries(context.Background(), "user-alice", child.AgentID, 10)
	if err != nil {
		t.Fatalf("list inbox deliveries: %v", err)
	}
	if len(deliveries) != 1 || deliveries[0].Content != "please inspect the runtime tool wiring" {
		t.Fatalf("unexpected deliveries: %+v", deliveries)
	}
}

func TestChannelCastDedupesPendingAddressedDelivery(t *testing.T) {
	rt, s, _ := testRuntimeWithTempCWD(t)
	parent, err := rt.StartRunWithMetadata(context.Background(), "coordinate repeated work", "user-alice", map[string]any{
		runMetadataAgentProfile: AgentProfileVText,
		runMetadataAgentRole:    AgentProfileVText,
		runMetadataAgentID:      "vtext:doc-repeat",
		runMetadataTrajectoryID: "trajectory-repeat",
	})
	if err != nil {
		t.Fatalf("start parent run: %v", err)
	}
	ctx := WithToolExecutionContext(context.Background(), parent)

	for i := 0; i < 2; i++ {
		if _, err := rt.ChannelCast(ctx, "doc-repeat-work", "agent-super-user-alice", "", "vtext", "user", "please run the same candidate-world probe"); err != nil {
			t.Fatalf("channel cast %d: %v", i, err)
		}
	}

	deliveries, err := s.ListPendingInboxDeliveries(context.Background(), "user-alice", "agent-super-user-alice", 10)
	if err != nil {
		t.Fatalf("list pending deliveries: %v", err)
	}
	if len(deliveries) != 1 {
		t.Fatalf("pending deliveries = %d, want one deduped delivery: %+v", len(deliveries), deliveries)
	}
	if deliveries[0].Content != "please run the same candidate-world probe" {
		t.Fatalf("delivery content = %q", deliveries[0].Content)
	}

	messages, _, err := rt.ChannelRead("doc-repeat-work", 0)
	if err != nil {
		t.Fatalf("channel read: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("channel messages = %d, want audit log to retain both casts", len(messages))
	}
}

func TestDelegationAllowlistsAndEvidenceTools(t *testing.T) {
	rt, s, cwd := testRuntimeWithTempCWD(t)
	if err := rt.InstallDefaultAgentTools(cwd); err != nil {
		t.Fatalf("install default agent tools: %v", err)
	}

	vtextTask, err := rt.StartRunWithMetadata(context.Background(), "revise document", "user-alice", map[string]any{
		runMetadataAgentProfile: AgentProfileVText,
		runMetadataAgentRole:    AgentProfileVText,
	})
	if err != nil {
		t.Fatalf("submit vtext task: %v", err)
	}
	superTask, err := rt.StartRunWithMetadata(context.Background(), "coordinate execution", "user-alice", map[string]any{
		runMetadataAgentProfile: AgentProfileSuper,
		runMetadataAgentRole:    AgentProfileSuper,
	})
	if err != nil {
		t.Fatalf("submit super task: %v", err)
	}
	time.Sleep(50 * time.Millisecond)

	vtextRegistry := rt.ToolRegistryForProfile(AgentProfileVText)
	if _, err := vtextRegistry.Execute(WithToolExecutionContext(context.Background(), vtextTask), "spawn_agent", json.RawMessage(`{
		"objective":"handle execution-heavy follow-up",
		"role":"super",
		"channel_id":"doc-exec-work"
	}`)); err == nil {
		t.Fatalf("vtext should not be allowed to spawn super")
	}

	superRequestRaw, err := vtextRegistry.Execute(WithToolExecutionContext(context.Background(), vtextTask), "request_super_execution", json.RawMessage(`{
		"objective":"handle execution-heavy follow-up",
		"channel_id":"doc-exec-work"
	}`))
	if err != nil {
		t.Fatalf("vtext request super execution: %v", err)
	}
	var superRequest struct {
		AgentID   string `json:"agent_id"`
		RunID     string `json:"loop_id"`
		Profile   string `json:"profile"`
		ChannelID string `json:"channel_id"`
	}
	if err := json.Unmarshal([]byte(superRequestRaw), &superRequest); err != nil {
		t.Fatalf("decode super request: %v", err)
	}
	if superRequest.Profile != AgentProfileSuper {
		t.Fatalf("super request profile = %q, want %q", superRequest.Profile, AgentProfileSuper)
	}
	if superRequest.AgentID != persistentSuperAgentID("user-alice") {
		t.Fatalf("super request agent_id = %q, want %q", superRequest.AgentID, persistentSuperAgentID("user-alice"))
	}
	if superRequest.ChannelID != "doc-exec-work" {
		t.Fatalf("super request channel_id = %q, want doc-exec-work", superRequest.ChannelID)
	}

	superRegistry := rt.ToolRegistryForProfile(AgentProfileSuper)
	coSuperRaw, err := superRegistry.Execute(WithToolExecutionContext(context.Background(), superTask), "spawn_agent", json.RawMessage(`{
		"objective":"handle execution subtree",
		"role":"co-super"
	}`))
	if err != nil {
		t.Fatalf("super spawn co-super: %v", err)
	}
	var coSuperSpawn struct {
		RunID   string `json:"loop_id"`
		Profile string `json:"profile"`
	}
	if err := json.Unmarshal([]byte(coSuperRaw), &coSuperSpawn); err != nil {
		t.Fatalf("decode co-super spawn: %v", err)
	}
	if coSuperSpawn.Profile != AgentProfileCoSuper {
		t.Fatalf("co-super profile = %q, want %q", coSuperSpawn.Profile, AgentProfileCoSuper)
	}

	child, err := s.GetRun(context.Background(), coSuperSpawn.RunID)
	if err != nil {
		t.Fatalf("get co-super task: %v", err)
	}
	coSuperRegistry := rt.ToolRegistryForProfile(AgentProfileCoSuper)
	if _, err := coSuperRegistry.Execute(WithToolExecutionContext(context.Background(), &child), "spawn_agent", json.RawMessage(`{
		"objective":"try to escape supervision",
		"role":"super"
	}`)); err == nil {
		t.Fatalf("co-super should not be allowed to spawn super")
	}

	researcherTask, err := rt.StartRunWithMetadata(context.Background(), "gather evidence", "user-alice", map[string]any{
		runMetadataAgentProfile: AgentProfileResearcher,
		runMetadataAgentRole:    AgentProfileResearcher,
	})
	if err != nil {
		t.Fatalf("submit researcher task: %v", err)
	}
	researcherRegistry := rt.ToolRegistryForProfile(AgentProfileResearcher)
	saveRaw, err := researcherRegistry.Execute(WithToolExecutionContext(context.Background(), researcherTask), "save_evidence", json.RawMessage(`{
		"kind":"web_page",
		"source_uri":"https://example.com",
		"title":"Example",
		"content":"captured evidence"
	}`))
	if err != nil {
		t.Fatalf("save_evidence: %v", err)
	}
	var saveResp struct {
		EvidenceID string `json:"evidence_id"`
		AgentID    string `json:"agent_id"`
	}
	if err := json.Unmarshal([]byte(saveRaw), &saveResp); err != nil {
		t.Fatalf("decode save_evidence: %v", err)
	}
	if saveResp.AgentID == "" || saveResp.EvidenceID == "" {
		t.Fatalf("unexpected save response: %+v", saveResp)
	}
	evidence, err := s.GetEvidence(context.Background(), saveResp.EvidenceID, "user-alice")
	if err != nil {
		t.Fatalf("get evidence: %v", err)
	}
	if evidence.Content != "captured evidence" {
		t.Fatalf("evidence content = %q, want %q", evidence.Content, "captured evidence")
	}
}

func TestSuperForkDesktopClonesStateAndPublishRequestsVM(t *testing.T) {
	rt, s, cwd := testRuntimeWithTempCWD(t)

	reg := vmctl.NewOwnershipRegistry("http://sandbox.test")
	sourceOwn, err := reg.ResolveOrAssignDesktop("user-alice", types.PrimaryDesktopID)
	if err != nil {
		t.Fatalf("resolve source desktop: %v", err)
	}
	handler := vmctl.NewHandler(reg)
	mux := http.NewServeMux()
	mux.HandleFunc("/internal/vmctl/fork-desktop", handler.HandleForkDesktop)
	mux.HandleFunc("/internal/vmctl/publish-desktop", handler.HandlePublishDesktop)
	mux.HandleFunc("/internal/vmctl/request-worker", handler.HandleRequestWorker)
	vmctlSrv := httptest.NewServer(mux)
	t.Cleanup(func() { vmctlSrv.Close() })

	rt.cfg.VmctlURL = vmctlSrv.URL
	if err := rt.InstallDefaultAgentTools(cwd); err != nil {
		t.Fatalf("install default agent tools: %v", err)
	}

	now := time.Now().UTC()
	if err := s.SaveDesktopStateForDesktop(context.Background(), types.DesktopState{
		OwnerID:   "user-alice",
		DesktopID: types.PrimaryDesktopID,
		Windows: []types.WindowState{
			{
				WindowID: "win-vtext",
				AppID:    "vtext",
				Title:    "Draft",
				Geometry: types.WindowGeometry{X: 20, Y: 30, Width: 900, Height: 700},
				Mode:     types.WindowNormal,
				ZIndex:   1,
				AppContext: map[string]any{
					"doc_id": "doc-1",
				},
				CreatedAt: now,
				UpdatedAt: now,
			},
		},
		ActiveWindowID: "win-vtext",
		UpdatedAt:      now,
	}); err != nil {
		t.Fatalf("save source desktop state: %v", err)
	}

	superTask, err := rt.StartRunWithMetadata(context.Background(), "coordinate execution", "user-alice", map[string]any{
		runMetadataAgentProfile: AgentProfileSuper,
		runMetadataAgentRole:    AgentProfileSuper,
		runMetadataDesktopID:    types.PrimaryDesktopID,
	})
	if err != nil {
		t.Fatalf("submit super task: %v", err)
	}

	superRegistry := rt.ToolRegistryForProfile(AgentProfileSuper)
	raw, err := superRegistry.Execute(WithToolExecutionContext(context.Background(), superTask), "fork_desktop", json.RawMessage(`{
		"desktop_id":"branch-a"
	}`))
	if err != nil {
		t.Fatalf("fork_desktop: %v", err)
	}
	var resp struct {
		Status            string `json:"status"`
		DesktopID         string `json:"desktop_id"`
		ParentDesktopID   string `json:"parent_desktop_id"`
		ParentVMID        string `json:"parent_vm_id"`
		SnapshotKind      string `json:"snapshot_kind"`
		Published         bool   `json:"published"`
		Availability      string `json:"availability"`
		CopiedWindowCount int    `json:"copied_window_count"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("decode fork_desktop: %v", err)
	}
	if resp.Status != "forked_background" {
		t.Fatalf("status = %q, want forked_background", resp.Status)
	}
	if resp.DesktopID != "branch-a" {
		t.Fatalf("desktop_id = %q, want branch-a", resp.DesktopID)
	}
	if resp.ParentDesktopID != types.PrimaryDesktopID {
		t.Fatalf("parent_desktop_id = %q, want %q", resp.ParentDesktopID, types.PrimaryDesktopID)
	}
	if resp.ParentVMID != sourceOwn.VMID {
		t.Fatalf("parent_vm_id = %q, want %q", resp.ParentVMID, sourceOwn.VMID)
	}
	if resp.SnapshotKind != "metadata_only" {
		t.Fatalf("snapshot_kind = %q, want metadata_only", resp.SnapshotKind)
	}
	if resp.Published {
		t.Fatal("forked candidate desktop should not be published yet")
	}
	if resp.Availability != "background_only" {
		t.Fatalf("availability = %q, want background_only", resp.Availability)
	}
	if resp.CopiedWindowCount != 1 {
		t.Fatalf("copied_window_count = %d, want 1", resp.CopiedWindowCount)
	}

	branchOwn := reg.GetOwnershipForDesktop("user-alice", "branch-a")
	if branchOwn == nil {
		t.Fatal("expected branch desktop ownership")
	}
	if branchOwn.VMID == sourceOwn.VMID {
		t.Fatalf("expected distinct VM for branch desktop, got %s", branchOwn.VMID)
	}
	if branchOwn.ParentDesktopID != types.PrimaryDesktopID {
		t.Fatalf("branch parent_desktop_id = %q, want %q", branchOwn.ParentDesktopID, types.PrimaryDesktopID)
	}
	if branchOwn.Published {
		t.Fatal("branch desktop should remain unpublished until publish_desktop")
	}

	branchState, err := s.GetDesktopStateForDesktop(context.Background(), "user-alice", "branch-a")
	if err != nil {
		t.Fatalf("get branch desktop state: %v", err)
	}
	if len(branchState.Windows) != 1 || branchState.Windows[0].WindowID != "win-vtext" {
		t.Fatalf("branch windows = %+v", branchState.Windows)
	}
	if branchState.ActiveWindowID != "win-vtext" {
		t.Fatalf("branch active_window_id = %q, want win-vtext", branchState.ActiveWindowID)
	}

	publishRaw, err := superRegistry.Execute(WithToolExecutionContext(context.Background(), superTask), "publish_desktop", json.RawMessage(`{
		"desktop_id":"branch-a"
	}`))
	if err != nil {
		t.Fatalf("publish_desktop: %v", err)
	}
	var publishResp struct {
		Status     string `json:"status"`
		DesktopID  string `json:"desktop_id"`
		Published  bool   `json:"published"`
		DesktopURL string `json:"desktop_url"`
	}
	if err := json.Unmarshal([]byte(publishRaw), &publishResp); err != nil {
		t.Fatalf("decode publish_desktop: %v", err)
	}
	if publishResp.Status != "published" || !publishResp.Published {
		t.Fatalf("unexpected publish response: %+v", publishResp)
	}
	if publishResp.DesktopURL != "/?desktop_id=branch-a" {
		t.Fatalf("desktop_url = %q", publishResp.DesktopURL)
	}
	if !reg.GetOwnershipForDesktop("user-alice", "branch-a").Published {
		t.Fatal("branch desktop should be published after publish_desktop")
	}
}

func TestSuperRequestWorkerVMReturnsTypedHandle(t *testing.T) {
	rt, _, cwd := testRuntimeWithTempCWD(t)

	reg := vmctl.NewOwnershipRegistry("http://sandbox.test")
	if _, err := reg.ResolveOrAssignDesktop("user-alice", types.PrimaryDesktopID); err != nil {
		t.Fatalf("resolve source desktop: %v", err)
	}
	handler := vmctl.NewHandler(reg)
	mux := http.NewServeMux()
	mux.HandleFunc("/internal/vmctl/request-worker", handler.HandleRequestWorker)
	vmctlSrv := httptest.NewServer(mux)
	t.Cleanup(func() { vmctlSrv.Close() })

	rt.cfg.VmctlURL = vmctlSrv.URL
	if err := rt.InstallDefaultAgentTools(cwd); err != nil {
		t.Fatalf("install default agent tools: %v", err)
	}

	superTask, err := rt.StartRunWithMetadata(context.Background(), "coordinate execution", "user-alice", map[string]any{
		runMetadataAgentProfile: AgentProfileSuper,
		runMetadataAgentRole:    AgentProfileSuper,
		runMetadataAgentID:      "super:primary",
		runMetadataDesktopID:    types.PrimaryDesktopID,
	})
	if err != nil {
		t.Fatalf("submit super task: %v", err)
	}

	superRegistry := rt.ToolRegistryForProfile(AgentProfileSuper)
	raw, err := superRegistry.Execute(WithToolExecutionContext(context.Background(), superTask), "request_worker_vm", json.RawMessage(`{
		"purpose":"Run background coding task",
		"machine_class":"worker-medium"
	}`))
	if err != nil {
		t.Fatalf("request_worker_vm: %v", err)
	}

	var resp struct {
		Status string `json:"status"`
		Handle struct {
			Kind          string `json:"kind"`
			WorkerID      string `json:"worker_id"`
			VMID          string `json:"vm_id"`
			UserID        string `json:"user_id"`
			DesktopID     string `json:"desktop_id"`
			ParentAgentID string `json:"parent_agent_id"`
			TrajectoryID  string `json:"trajectory_id"`
			Purpose       string `json:"purpose"`
			MachineClass  string `json:"machine_class"`
			SandboxURL    string `json:"sandbox_url"`
			State         string `json:"state"`
		} `json:"handle"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("decode request_worker_vm: %v", err)
	}
	if resp.Status != "worker_requested" {
		t.Fatalf("status = %q, want worker_requested", resp.Status)
	}
	if resp.Handle.Kind != "worker" {
		t.Fatalf("kind = %q, want worker", resp.Handle.Kind)
	}
	if resp.Handle.WorkerID == "" || resp.Handle.VMID == "" {
		t.Fatalf("expected non-empty worker handle identifiers: %+v", resp.Handle)
	}
	if resp.Handle.UserID != "user-alice" {
		t.Fatalf("user_id = %q, want user-alice", resp.Handle.UserID)
	}
	if resp.Handle.DesktopID != types.PrimaryDesktopID {
		t.Fatalf("desktop_id = %q, want %q", resp.Handle.DesktopID, types.PrimaryDesktopID)
	}
	if resp.Handle.ParentAgentID != "super:primary" {
		t.Fatalf("parent_agent_id = %q, want super:primary", resp.Handle.ParentAgentID)
	}
	if resp.Handle.TrajectoryID != superTask.RunID {
		t.Fatalf("trajectory_id = %q, want %q", resp.Handle.TrajectoryID, superTask.RunID)
	}
	if resp.Handle.Purpose != "Run background coding task" {
		t.Fatalf("purpose = %q, want %q", resp.Handle.Purpose, "Run background coding task")
	}
	if resp.Handle.MachineClass != "worker-medium" {
		t.Fatalf("machine_class = %q, want worker-medium", resp.Handle.MachineClass)
	}
	if resp.Handle.SandboxURL != "http://sandbox.test" {
		t.Fatalf("sandbox_url = %q, want %q", resp.Handle.SandboxURL, "http://sandbox.test")
	}
	if resp.Handle.State != "active" {
		t.Fatalf("state = %q, want active", resp.Handle.State)
	}
}

func TestSuperRequestWorkerVMNormalizesStandardMachineClass(t *testing.T) {
	rt, _, cwd := testRuntimeWithTempCWD(t)

	reg := vmctl.NewOwnershipRegistry("http://sandbox.test")
	if _, err := reg.ResolveOrAssignDesktop("user-alice", types.PrimaryDesktopID); err != nil {
		t.Fatalf("resolve source desktop: %v", err)
	}
	handler := vmctl.NewHandler(reg)
	mux := http.NewServeMux()
	mux.HandleFunc("/internal/vmctl/request-worker", handler.HandleRequestWorker)
	vmctlSrv := httptest.NewServer(mux)
	t.Cleanup(func() { vmctlSrv.Close() })

	rt.cfg.VmctlURL = vmctlSrv.URL
	if err := rt.InstallDefaultAgentTools(cwd); err != nil {
		t.Fatalf("install default agent tools: %v", err)
	}

	superTask, err := rt.StartRunWithMetadata(context.Background(), "coordinate execution", "user-alice", map[string]any{
		runMetadataAgentProfile: AgentProfileSuper,
		runMetadataAgentRole:    AgentProfileSuper,
		runMetadataAgentID:      "super:primary",
		runMetadataDesktopID:    types.PrimaryDesktopID,
	})
	if err != nil {
		t.Fatalf("submit super task: %v", err)
	}

	superRegistry := rt.ToolRegistryForProfile(AgentProfileSuper)
	raw, err := superRegistry.Execute(WithToolExecutionContext(context.Background(), superTask), "request_worker_vm", json.RawMessage(`{
		"purpose":"Run background coding task",
		"machine_class":"standard"
	}`))
	if err != nil {
		t.Fatalf("request_worker_vm: %v", err)
	}

	var resp struct {
		MachineClassNormalizedFrom string `json:"machine_class_normalized_from"`
		MachineClass               string `json:"machine_class"`
		Handle                     struct {
			MachineClass string `json:"machine_class"`
		} `json:"handle"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("decode request_worker_vm: %v", err)
	}
	if resp.MachineClassNormalizedFrom != "standard" {
		t.Fatalf("machine_class_normalized_from = %q, want standard", resp.MachineClassNormalizedFrom)
	}
	if resp.MachineClass != "worker-small" || resp.Handle.MachineClass != "worker-small" {
		t.Fatalf("machine class not normalized to worker-small: %+v", resp)
	}
}

func TestSuperRequestWorkerVMReusesActiveLeaseUnlessParallelAllowed(t *testing.T) {
	rt, _, cwd := testRuntimeWithTempCWD(t)

	reg := vmctl.NewOwnershipRegistry("http://sandbox.test")
	if _, err := reg.ResolveOrAssignDesktop("user-alice", types.PrimaryDesktopID); err != nil {
		t.Fatalf("resolve source desktop: %v", err)
	}
	handler := vmctl.NewHandler(reg)
	mux := http.NewServeMux()
	mux.HandleFunc("/internal/vmctl/request-worker", handler.HandleRequestWorker)
	vmctlSrv := httptest.NewServer(mux)
	t.Cleanup(func() { vmctlSrv.Close() })

	rt.cfg.VmctlURL = vmctlSrv.URL
	if err := rt.InstallDefaultAgentTools(cwd); err != nil {
		t.Fatalf("install default agent tools: %v", err)
	}
	superTask, err := rt.StartRunWithMetadata(context.Background(), "coordinate repeated execution", "user-alice", map[string]any{
		runMetadataAgentProfile: AgentProfileSuper,
		runMetadataAgentRole:    AgentProfileSuper,
		runMetadataAgentID:      "super:primary",
		runMetadataDesktopID:    types.PrimaryDesktopID,
		runMetadataTrajectoryID: "traj-repeat",
	})
	if err != nil {
		t.Fatalf("submit super task: %v", err)
	}
	superRegistry := rt.ToolRegistryForProfile(AgentProfileSuper)
	request := func(raw json.RawMessage) string {
		t.Helper()
		out, err := superRegistry.Execute(WithToolExecutionContext(context.Background(), superTask), "request_worker_vm", raw)
		if err != nil {
			t.Fatalf("request_worker_vm: %v", err)
		}
		var resp struct {
			Handle struct {
				WorkerID             string `json:"worker_id"`
				VMID                 string `json:"vm_id"`
				ObjectiveFingerprint string `json:"objective_fingerprint"`
			} `json:"handle"`
		}
		if err := json.Unmarshal([]byte(out), &resp); err != nil {
			t.Fatalf("decode request_worker_vm: %v", err)
		}
		if resp.Handle.ObjectiveFingerprint == "" {
			t.Fatalf("request_worker_vm returned empty objective fingerprint: %s", out)
		}
		return resp.Handle.WorkerID + "/" + resp.Handle.VMID
	}

	first := request(json.RawMessage(`{"purpose":"Run the same product patch","machine_class":"worker-small"}`))
	second := request(json.RawMessage(`{"purpose":" run THE same/product patch!! ","machine_class":"worker-small"}`))
	if second != first {
		t.Fatalf("second worker = %s, want reused %s", second, first)
	}
	parallel := request(json.RawMessage(`{"purpose":"Run the same product patch","machine_class":"worker-small","allow_parallel":true}`))
	if parallel == first {
		t.Fatalf("parallel worker reused %s unexpectedly", parallel)
	}
}

func TestConductorCanSpawnVTextAndVTextCanSpawnResearcher(t *testing.T) {
	rt, s, cwd := testRuntimeWithTempCWD(t)
	if err := rt.InstallDefaultAgentTools(cwd); err != nil {
		t.Fatalf("install default agent tools: %v", err)
	}

	conductorTask, err := rt.StartRunWithMetadata(context.Background(), "route this request", "user-alice", map[string]any{
		runMetadataAgentProfile: AgentProfileConductor,
		runMetadataAgentRole:    AgentProfileConductor,
	})
	if err != nil {
		t.Fatalf("submit conductor task: %v", err)
	}
	time.Sleep(50 * time.Millisecond)

	conductorRegistry := rt.ToolRegistryForProfile(AgentProfileConductor)
	if spawnTool, ok := conductorRegistry.Lookup("spawn_agent"); !ok {
		t.Fatal("conductor missing spawn_agent")
	} else if got := toolSchemaStringEnum(spawnTool.Parameters, "role"); len(got) != 1 || got[0] != AgentProfileVText {
		t.Fatalf("conductor spawn_agent role enum = %#v, want only %q", got, AgentProfileVText)
	}
	if _, err := conductorRegistry.Execute(WithToolExecutionContext(context.Background(), conductorTask), "spawn_agent", json.RawMessage(`{
		"objective":"research should be owned by vtext, not conductor",
		"role":"researcher"
	}`)); err == nil {
		t.Fatal("conductor should not be allowed to spawn researcher")
	}
	if _, err := conductorRegistry.Execute(WithToolExecutionContext(context.Background(), conductorTask), "spawn_agent", json.RawMessage(`{
		"objective":"create v0 and own the document",
		"role":"vtext"
	}`)); err == nil {
		t.Fatal("conductor spawn vtext without initial_content should fail")
	}
	vtextSpawnRaw, err := conductorRegistry.Execute(WithToolExecutionContext(context.Background(), conductorTask), "spawn_agent", json.RawMessage(`{
		"objective":"create v0 and own the document",
		"role":"vtext",
		"channel_id":"doc-work",
		"initial_content":"# Routed document\n\nInitial conductor-authored abstract."
	}`))
	if err != nil {
		t.Fatalf("conductor spawn vtext: %v", err)
	}
	var vtextSpawn struct {
		AgentID           string `json:"agent_id"`
		RunID             string `json:"loop_id"`
		Profile           string `json:"profile"`
		ChannelID         string `json:"channel_id"`
		State             string `json:"state"`
		UserRevisionID    string `json:"user_revision_id"`
		FramingRevisionID string `json:"framing_revision_id"`
		InitialRevisionID string `json:"initial_revision_id"`
	}
	if err := json.Unmarshal([]byte(vtextSpawnRaw), &vtextSpawn); err != nil {
		t.Fatalf("decode vtext spawn: %v", err)
	}
	if vtextSpawn.AgentID != "vtext:"+vtextSpawn.ChannelID {
		t.Fatalf("vtext spawn agent_id = %q, want vtext:%s", vtextSpawn.AgentID, vtextSpawn.ChannelID)
	}
	if vtextSpawn.Profile != AgentProfileVText {
		t.Fatalf("vtext spawn profile = %q, want %q", vtextSpawn.Profile, AgentProfileVText)
	}
	if vtextSpawn.ChannelID == "" {
		t.Fatal("vtext spawn channel_id should not be empty")
	}
	if vtextSpawn.RunID == "" {
		t.Fatal("vtext spawn loop_id should point to the initial product-path vtext run")
	}
	if vtextSpawn.State != "open" {
		t.Fatalf("vtext spawn state = %q, want open", vtextSpawn.State)
	}
	if vtextSpawn.UserRevisionID == "" || vtextSpawn.FramingRevisionID == "" || vtextSpawn.InitialRevisionID != vtextSpawn.FramingRevisionID {
		t.Fatalf("unexpected vtext spawn revision ids: %+v", vtextSpawn)
	}
	vtextAgent, err := s.GetAgent(context.Background(), vtextSpawn.AgentID)
	if err != nil {
		t.Fatalf("get vtext agent: %v", err)
	}
	if vtextAgent.ChannelID != vtextSpawn.ChannelID {
		t.Fatalf("vtext agent channel_id = %q, want %q", vtextAgent.ChannelID, vtextSpawn.ChannelID)
	}
	parentAfterSpawn, err := s.GetRun(context.Background(), conductorTask.RunID)
	if err != nil {
		t.Fatalf("get conductor task: %v", err)
	}
	if parentAfterSpawn.Metadata["doc_id"] != vtextSpawn.ChannelID {
		t.Fatalf("conductor metadata doc_id = %v, want %q", parentAfterSpawn.Metadata["doc_id"], vtextSpawn.ChannelID)
	}
	if strings.TrimSpace(parentAfterSpawn.Result) == "" {
		t.Fatal("conductor result should be populated as soon as vtext is opened")
	}
	var parentDecision struct {
		Action            string `json:"action"`
		App               string `json:"app"`
		DocID             string `json:"doc_id"`
		InitialRunID      string `json:"initial_loop_id"`
		InitialRevisionID string `json:"initial_revision_id"`
		UserRevisionID    string `json:"user_revision_id"`
		FramingRevisionID string `json:"framing_revision_id"`
	}
	if err := json.Unmarshal([]byte(parentAfterSpawn.Result), &parentDecision); err != nil {
		t.Fatalf("decode conductor result: %v", err)
	}
	if parentDecision.Action != "open_app" || parentDecision.App != AgentProfileVText {
		t.Fatalf("unexpected conductor decision: %+v", parentDecision)
	}
	if parentDecision.DocID != vtextSpawn.ChannelID {
		t.Fatalf("conductor result doc_id = %q, want %q", parentDecision.DocID, vtextSpawn.ChannelID)
	}
	if parentDecision.InitialRunID != vtextSpawn.RunID {
		t.Fatalf("conductor result initial_loop_id = %q, want %q", parentDecision.InitialRunID, vtextSpawn.RunID)
	}
	if parentDecision.UserRevisionID != vtextSpawn.UserRevisionID || parentDecision.FramingRevisionID != vtextSpawn.FramingRevisionID || parentDecision.InitialRevisionID != vtextSpawn.FramingRevisionID {
		t.Fatalf("unexpected conductor result revision ids: %+v; spawn=%+v", parentDecision, vtextSpawn)
	}

	vtextTask, err := rt.StartRunWithMetadata(context.Background(), "own a later document step", "user-alice", map[string]any{
		runMetadataAgentProfile: AgentProfileVText,
		runMetadataAgentRole:    AgentProfileVText,
		runMetadataAgentID:      vtextSpawn.AgentID,
		runMetadataChannelID:    vtextSpawn.ChannelID,
		"doc_id":                vtextSpawn.ChannelID,
	})
	if err != nil {
		t.Fatalf("start vtext run for delegation: %v", err)
	}
	vtextRegistry := rt.ToolRegistryForProfile(AgentProfileVText)
	researchSpawnRaw, err := vtextRegistry.Execute(WithToolExecutionContext(context.Background(), vtextTask), "spawn_agent", json.RawMessage(`{
		"objective":"research background facts for the document",
		"role":"researcher",
		"channel_id":"`+vtextSpawn.ChannelID+`"
	}`))
	if err != nil {
		t.Fatalf("vtext spawn researcher: %v", err)
	}
	var researchSpawn struct {
		RunID     string `json:"loop_id"`
		Profile   string `json:"profile"`
		ChannelID string `json:"channel_id"`
	}
	if err := json.Unmarshal([]byte(researchSpawnRaw), &researchSpawn); err != nil {
		t.Fatalf("decode researcher spawn: %v", err)
	}
	if researchSpawn.Profile != AgentProfileResearcher {
		t.Fatalf("research spawn profile = %q, want %q", researchSpawn.Profile, AgentProfileResearcher)
	}
	if researchSpawn.ChannelID != vtextSpawn.ChannelID {
		t.Fatalf("research spawn channel_id = %q, want %q", researchSpawn.ChannelID, vtextSpawn.ChannelID)
	}

	researchAliasSpawnRaw, err := vtextRegistry.Execute(WithToolExecutionContext(context.Background(), vtextTask), "spawn_agent", json.RawMessage(`{
		"objective":"research current facts for the document",
		"role":"research",
		"channel_id":"`+vtextSpawn.ChannelID+`"
	}`))
	if err != nil {
		t.Fatalf("vtext spawn research alias: %v", err)
	}
	var researchAliasSpawn struct {
		Role    string `json:"role"`
		Profile string `json:"profile"`
	}
	if err := json.Unmarshal([]byte(researchAliasSpawnRaw), &researchAliasSpawn); err != nil {
		t.Fatalf("decode researcher alias spawn: %v", err)
	}
	if researchAliasSpawn.Role != AgentProfileResearcher || researchAliasSpawn.Profile != AgentProfileResearcher {
		t.Fatalf("research alias spawn = role %q profile %q, want researcher/researcher", researchAliasSpawn.Role, researchAliasSpawn.Profile)
	}
}

func TestConcurrentConductorVTextSpawnsShareRoute(t *testing.T) {
	rt, s, cwd := testRuntimeWithTempCWD(t)
	if err := rt.InstallDefaultAgentTools(cwd); err != nil {
		t.Fatalf("install default agent tools: %v", err)
	}

	ctx := context.Background()
	conductorTask := &types.RunRecord{
		RunID:        "conductor-concurrent-vtext",
		OwnerID:      "user-alice",
		SandboxID:    "sandbox-test",
		State:        types.RunRunning,
		Prompt:       "Create one durable vtext document.",
		ChannelID:    "conductor-concurrent-vtext",
		AgentProfile: AgentProfileConductor,
		AgentRole:    AgentProfileConductor,
		Metadata: map[string]any{
			runMetadataAgentProfile: AgentProfileConductor,
			runMetadataAgentRole:    AgentProfileConductor,
			"requested_app":         AgentProfileVText,
			"seed_prompt":           "Create one durable vtext document.",
		},
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := s.CreateRun(ctx, *conductorTask); err != nil {
		t.Fatalf("create conductor run: %v", err)
	}

	registry := rt.ToolRegistryForProfile(AgentProfileConductor)
	rawArgs := json.RawMessage(`{"objective":"Create one durable vtext document.","role":"vtext","initial_content":"# Durable vtext document\n\nInitial conductor-authored abstract."}`)
	results := make([]string, 2)
	errs := make([]error, 2)
	var wg sync.WaitGroup
	for i := range results {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			results[idx], errs[idx] = registry.Execute(WithToolExecutionContext(ctx, conductorTask), "spawn_agent", rawArgs)
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("spawn %d: %v", i, err)
		}
	}

	var first struct {
		DocID   string `json:"doc_id"`
		LoopID  string `json:"loop_id"`
		AgentID string `json:"agent_id"`
	}
	if err := json.Unmarshal([]byte(results[0]), &first); err != nil {
		t.Fatalf("decode first spawn: %v", err)
	}
	for i, raw := range results[1:] {
		var got struct {
			DocID   string `json:"doc_id"`
			LoopID  string `json:"loop_id"`
			AgentID string `json:"agent_id"`
		}
		if err := json.Unmarshal([]byte(raw), &got); err != nil {
			t.Fatalf("decode spawn %d: %v", i+1, err)
		}
		if got.DocID != first.DocID || got.LoopID != first.LoopID || got.AgentID != first.AgentID {
			t.Fatalf("concurrent spawn returned different routes: first=%+v got=%+v", first, got)
		}
	}

	docs, err := s.ListDocumentsByOwner(ctx, "user-alice", 10)
	if err != nil {
		t.Fatalf("list documents: %v", err)
	}
	if len(docs) != 1 || docs[0].DocID != first.DocID {
		t.Fatalf("documents = %+v, want exactly the shared vtext doc %q", docs, first.DocID)
	}
	runs, err := s.ListRunsByChannel(ctx, "user-alice", first.DocID, 10)
	if err != nil {
		t.Fatalf("list runs by channel: %v", err)
	}
	vtextRuns := 0
	for _, run := range runs {
		if run.AgentProfile == AgentProfileVText {
			vtextRuns++
		}
	}
	if vtextRuns != 1 {
		t.Fatalf("vtext runs on shared route = %d, want 1; runs=%+v", vtextRuns, runs)
	}
	parentAfter, err := s.GetRun(ctx, conductorTask.RunID)
	if err != nil {
		t.Fatalf("get parent run: %v", err)
	}
	if parentAfter.Metadata["doc_id"] != first.DocID || parentAfter.Metadata["initial_loop_id"] != first.LoopID {
		t.Fatalf("parent route metadata = %+v, want doc %q loop %q", parentAfter.Metadata, first.DocID, first.LoopID)
	}
}

func TestResearcherSubmitResearchFindingsPersistsEvidenceAndDedupes(t *testing.T) {
	rt, s, cwd := testRuntimeWithTempCWD(t)
	if err := rt.InstallDefaultAgentTools(cwd); err != nil {
		t.Fatalf("install default agent tools: %v", err)
	}

	vtextTask, err := rt.StartRunWithMetadata(context.Background(), "own the draft", "user-alice", map[string]any{
		runMetadataAgentProfile: AgentProfileVText,
		runMetadataAgentRole:    AgentProfileVText,
		runMetadataChannelID:    "doc-1",
		runMetadataAgentID:      "vtext:doc-1",
	})
	if err != nil {
		t.Fatalf("submit vtext task: %v", err)
	}
	researcherTask, err := rt.StartChildRun(context.Background(), vtextTask.RunID, "research the claim", "user-alice", map[string]any{
		runMetadataAgentProfile: AgentProfileResearcher,
		runMetadataAgentRole:    AgentProfileResearcher,
		runMetadataChannelID:    "doc-1",
	})
	if err != nil {
		t.Fatalf("submit researcher task: %v", err)
	}

	researcherRegistry := rt.ToolRegistryForProfile(AgentProfileResearcher)
	raw, err := researcherRegistry.Execute(WithToolExecutionContext(context.Background(), researcherTask), "submit_research_findings", json.RawMessage(`{
		"finding_id":"finding-001",
		"findings":["Model releases this week improved reasoning and tool use."],
		"evidence":[
			{
				"kind":"web_page",
				"source_uri":"https://example.com/release",
				"title":"Release notes",
				"content":"Release notes describing stronger reasoning and tool use."
			}
		],
		"notes":["The claim is recent enough that priors alone are weak."],
		"questions":["Should we mention the release date explicitly?"]
	}`))
	if err != nil {
		t.Fatalf("submit_research_findings: %v", err)
	}

	var resp struct {
		FindingID   string   `json:"finding_id"`
		AgentID     string   `json:"agent_id"`
		ChannelID   string   `json:"channel_id"`
		Cursor      int64    `json:"cursor"`
		EvidenceIDs []string `json:"evidence_ids"`
		Status      string   `json:"status"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("decode submit_research_findings: %v", err)
	}
	if resp.Status != "submitted" {
		t.Fatalf("status = %q, want submitted", resp.Status)
	}
	if resp.AgentID != "vtext:doc-1" {
		t.Fatalf("agent_id = %q, want %q", resp.AgentID, "vtext:doc-1")
	}
	if len(resp.EvidenceIDs) != 1 {
		t.Fatalf("evidence_ids = %+v, want 1 id", resp.EvidenceIDs)
	}

	evidence, err := s.GetEvidence(context.Background(), resp.EvidenceIDs[0], "user-alice")
	if err != nil {
		t.Fatalf("get evidence: %v", err)
	}
	if evidence.Title != "Release notes" {
		t.Fatalf("evidence title = %q, want %q", evidence.Title, "Release notes")
	}

	finding, err := s.GetResearchFinding(context.Background(), "user-alice", "finding-001")
	if err != nil {
		t.Fatalf("get research finding: %v", err)
	}
	if finding.MessageSeq != resp.Cursor {
		t.Fatalf("finding message_seq = %d, want %d", finding.MessageSeq, resp.Cursor)
	}

	rawAgain, err := researcherRegistry.Execute(WithToolExecutionContext(context.Background(), researcherTask), "submit_research_findings", json.RawMessage(`{
		"finding_id":"finding-001",
		"findings":["Model releases this week improved reasoning and tool use."],
		"evidence":[
			{
				"kind":"web_page",
				"source_uri":"https://example.com/release",
				"title":"Release notes",
				"content":"Release notes describing stronger reasoning and tool use."
			}
		],
		"notes":["The claim is recent enough that priors alone are weak."],
		"questions":["Should we mention the release date explicitly?"]
	}`))
	if err != nil {
		t.Fatalf("repeat submit_research_findings: %v", err)
	}
	var respAgain struct {
		Cursor int64  `json:"cursor"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal([]byte(rawAgain), &respAgain); err != nil {
		t.Fatalf("decode repeated submit_research_findings: %v", err)
	}
	if respAgain.Status != "existing" {
		t.Fatalf("repeat status = %q, want existing", respAgain.Status)
	}
	if respAgain.Cursor != resp.Cursor {
		t.Fatalf("repeat cursor = %d, want %d", respAgain.Cursor, resp.Cursor)
	}

	messages, err := s.ListChannelMessages(context.Background(), "user-alice", "doc-1", 0, 10)
	if err != nil {
		t.Fatalf("list channel messages: %v", err)
	}
	var findingsMessage *types.ChannelMessage
	for i := range messages {
		if messages[i].ToAgentID == "vtext:doc-1" && strings.Contains(messages[i].Content, resp.EvidenceIDs[0]) {
			findingsMessage = &messages[i]
			break
		}
	}
	if findingsMessage == nil {
		t.Fatalf("expected findings packet in channel messages, got %+v", messages)
	}
}

func TestSubmitWorkerUpdatePersistsStructuredNonPatchUpdate(t *testing.T) {
	rt, s, cwd := testRuntimeWithTempCWD(t)
	if err := rt.InstallDefaultAgentTools(cwd); err != nil {
		t.Fatalf("install default agent tools: %v", err)
	}

	ctx := context.Background()
	ownerID := "user-alice"
	docID := "doc-structured-worker-update"
	now := time.Now().UTC()
	if err := s.CreateDocument(ctx, types.Document{
		DocID:     docID,
		OwnerID:   ownerID,
		Title:     "Structured Worker Update",
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create document: %v", err)
	}
	if err := s.UpsertAgent(ctx, types.AgentRecord{
		AgentID:   "vtext:" + docID,
		OwnerID:   ownerID,
		SandboxID: "sandbox-test",
		Profile:   AgentProfileVText,
		Role:      AgentProfileVText,
		ChannelID: docID,
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("upsert vtext agent: %v", err)
	}

	superRun, err := rt.StartRunWithMetadata(ctx, "Generate and verify the toy model artifact", ownerID, map[string]any{
		runMetadataAgentProfile: AgentProfileSuper,
		runMetadataAgentRole:    AgentProfileSuper,
		runMetadataAgentID:      "super:primary",
		runMetadataChannelID:    docID,
		runMetadataTrajectoryID: "traj-structured-worker-update",
	})
	if err != nil {
		t.Fatalf("start super run: %v", err)
	}

	superRegistry := rt.ToolRegistryForProfile(AgentProfileSuper)
	rawArgs := json.RawMessage(`{
		"update_id":"super-update-1",
		"agent_id":"vtext:doc-structured-worker-update",
		"channel_id":"doc-structured-worker-update",
		"findings":["A deterministic seed keeps the cellular automata visualization reproducible."],
		"artifacts":["artifacts/evolution-ca.html"],
		"refs":["git:abc123"],
		"tests":["node artifacts/evolution-ca.verify.js passed"],
		"questions":["Should mutation rate be user-adjustable in the UI?"],
		"proposals":["Expose generation count, population, and mean fitness as visible controls."],
		"notes":["This is a structured worker update, not a document patch."]
	}`)
	raw, err := superRegistry.Execute(WithToolExecutionContext(ctx, superRun), "submit_worker_update", rawArgs)
	if err != nil {
		t.Fatalf("submit_worker_update: %v", err)
	}
	var resp struct {
		UpdateID     string `json:"update_id"`
		AgentID      string `json:"agent_id"`
		ChannelID    string `json:"channel_id"`
		Cursor       int64  `json:"cursor"`
		TrajectoryID string `json:"trajectory_id"`
		Status       string `json:"status"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("decode submit_worker_update response: %v", err)
	}
	if resp.UpdateID != "super-update-1" || resp.AgentID != "vtext:"+docID || resp.ChannelID != docID || resp.Cursor == 0 || resp.Status != "submitted" {
		t.Fatalf("unexpected submit_worker_update response: %+v", resp)
	}
	if resp.TrajectoryID != "traj-structured-worker-update" {
		t.Fatalf("trajectory_id = %q, want traj-structured-worker-update", resp.TrajectoryID)
	}

	update, err := s.GetWorkerUpdate(ctx, ownerID, "super-update-1")
	if err != nil {
		t.Fatalf("get worker update: %v", err)
	}
	if update.AgentID != "super:primary" || update.TargetAgentID != "vtext:"+docID || update.Role != AgentProfileSuper {
		t.Fatalf("unexpected worker update identity: %+v", update)
	}
	if len(update.Artifacts) != 1 || update.Artifacts[0] != "artifacts/evolution-ca.html" {
		t.Fatalf("artifacts = %+v", update.Artifacts)
	}
	if len(update.Tests) != 1 || !strings.Contains(update.Tests[0], "passed") {
		t.Fatalf("tests = %+v", update.Tests)
	}
	if !strings.Contains(update.Content, "Artifacts:") || !strings.Contains(update.Content, "Tests:") || !strings.Contains(update.Content, "Proposals:") {
		t.Fatalf("worker update content is not structured: %q", update.Content)
	}

	messages, err := s.ListChannelMessages(ctx, ownerID, docID, 0, 10)
	if err != nil {
		t.Fatalf("list channel messages: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("messages len = %d, want 1: %+v", len(messages), messages)
	}
	if messages[0].Seq != resp.Cursor || messages[0].ToAgentID != "vtext:"+docID || messages[0].Role != AgentProfileSuper {
		t.Fatalf("unexpected channel message: %+v", messages[0])
	}
	if !strings.Contains(messages[0].Content, "Worker update ready.") || strings.Contains(strings.ToLower(messages[0].Content), "apply this patch") {
		t.Fatalf("channel message should be a structured update, not a patch: %q", messages[0].Content)
	}

	deliveries, err := s.ListPendingInboxDeliveries(ctx, ownerID, "vtext:"+docID, 10)
	if err != nil {
		t.Fatalf("list inbox deliveries: %v", err)
	}
	if len(deliveries) != 1 || deliveries[0].FromAgentID != "super:primary" || deliveries[0].ChannelID != docID {
		t.Fatalf("unexpected inbox deliveries: %+v", deliveries)
	}

	updates, err := s.ListWorkerUpdatesByTrajectory(ctx, ownerID, "traj-structured-worker-update", 10)
	if err != nil {
		t.Fatalf("list worker updates by trajectory: %v", err)
	}
	if len(updates) != 1 || updates[0].UpdateID != "super-update-1" {
		t.Fatalf("trajectory updates = %+v", updates)
	}

	rawAgain, err := superRegistry.Execute(WithToolExecutionContext(ctx, superRun), "submit_worker_update", rawArgs)
	if err != nil {
		t.Fatalf("repeat submit_worker_update: %v", err)
	}
	var repeat struct {
		Cursor int64  `json:"cursor"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal([]byte(rawAgain), &repeat); err != nil {
		t.Fatalf("decode repeat response: %v", err)
	}
	if repeat.Status != "existing" || repeat.Cursor != resp.Cursor {
		t.Fatalf("repeat response = %+v, want existing cursor %d", repeat, resp.Cursor)
	}
	messages, err = s.ListChannelMessages(ctx, ownerID, docID, 0, 10)
	if err != nil {
		t.Fatalf("list channel messages after repeat: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("repeat submit should not duplicate messages, got %+v", messages)
	}

	_, err = superRegistry.Execute(WithToolExecutionContext(ctx, superRun), "submit_worker_update", json.RawMessage(`{
		"update_id":"super-update-1",
		"agent_id":"vtext:doc-structured-worker-update",
		"tests":["different test payload"]
	}`))
	if err == nil {
		t.Fatal("same update_id with different payload should fail")
	}
}

func TestSubmitWorkerUpdateUsesTargetChannelOverExplicitChannel(t *testing.T) {
	rt, s, cwd := testRuntimeWithTempCWD(t)
	if err := rt.InstallDefaultAgentTools(cwd); err != nil {
		t.Fatalf("install default agent tools: %v", err)
	}

	ctx := context.Background()
	ownerID := "user-alice"
	docID := "doc-authoritative-channel"
	wrongChannelID := "not-the-vtext-doc-channel"
	now := time.Now().UTC()
	if err := s.CreateDocument(ctx, types.Document{
		DocID:     docID,
		OwnerID:   ownerID,
		Title:     "Authoritative Channel",
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create document: %v", err)
	}
	if err := s.UpsertAgent(ctx, types.AgentRecord{
		AgentID:   "vtext:" + docID,
		OwnerID:   ownerID,
		SandboxID: "sandbox-test",
		Profile:   AgentProfileVText,
		Role:      AgentProfileVText,
		ChannelID: docID,
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("upsert vtext agent: %v", err)
	}

	superRun, err := rt.StartRunWithMetadata(ctx, "Report an artifact", ownerID, map[string]any{
		runMetadataAgentProfile: AgentProfileSuper,
		runMetadataAgentRole:    AgentProfileSuper,
		runMetadataAgentID:      "super:authoritative",
		runMetadataChannelID:    wrongChannelID,
		runMetadataTrajectoryID: "traj-authoritative-channel",
	})
	if err != nil {
		t.Fatalf("start super run: %v", err)
	}

	superRegistry := rt.ToolRegistryForProfile(AgentProfileSuper)
	raw, err := superRegistry.Execute(WithToolExecutionContext(ctx, superRun), "submit_worker_update", json.RawMessage(`{
		"update_id":"super-authoritative-channel",
		"agent_id":"vtext:doc-authoritative-channel",
		"channel_id":"not-the-vtext-doc-channel",
		"artifacts":["artifacts/authoritative.txt"],
		"tests":["verified authoritative channel routing"]
	}`))
	if err != nil {
		t.Fatalf("submit_worker_update: %v", err)
	}
	var resp struct {
		ChannelID string `json:"channel_id"`
		Cursor    int64  `json:"cursor"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("decode submit_worker_update response: %v", err)
	}
	if resp.ChannelID != docID || resp.Cursor == 0 {
		t.Fatalf("response routed to channel %q cursor %d, want %q with cursor", resp.ChannelID, resp.Cursor, docID)
	}

	messages, err := s.ListChannelMessages(ctx, ownerID, docID, 0, 10)
	if err != nil {
		t.Fatalf("list target channel messages: %v", err)
	}
	if len(messages) != 1 || messages[0].ChannelID != docID || messages[0].ToAgentID != "vtext:"+docID {
		t.Fatalf("unexpected target channel messages: %+v", messages)
	}
	wrongMessages, err := s.ListChannelMessages(ctx, ownerID, wrongChannelID, 0, 10)
	if err != nil {
		t.Fatalf("list wrong channel messages: %v", err)
	}
	if len(wrongMessages) != 0 {
		t.Fatalf("worker update leaked onto explicit wrong channel: %+v", wrongMessages)
	}
}

func TestSubmitWorkerUpdateUsesParentAgentOverExplicitAgent(t *testing.T) {
	rt, s, cwd := testRuntimeWithTempCWD(t)
	if err := rt.InstallDefaultAgentTools(cwd); err != nil {
		t.Fatalf("install default agent tools: %v", err)
	}

	ctx := context.Background()
	ownerID := "user-alice"
	docID := "doc-authoritative-parent"
	now := time.Now().UTC()
	if err := s.CreateDocument(ctx, types.Document{
		DocID:     docID,
		OwnerID:   ownerID,
		Title:     "Authoritative Parent",
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create document: %v", err)
	}
	if err := s.UpsertAgent(ctx, types.AgentRecord{
		AgentID:   "vtext:" + docID,
		OwnerID:   ownerID,
		SandboxID: "sandbox-test",
		Profile:   AgentProfileVText,
		Role:      AgentProfileVText,
		ChannelID: docID,
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("upsert vtext agent: %v", err)
	}
	if err := s.UpsertAgent(ctx, types.AgentRecord{
		AgentID:   "researcher:decoy",
		OwnerID:   ownerID,
		SandboxID: "sandbox-test",
		Profile:   AgentProfileResearcher,
		Role:      AgentProfileResearcher,
		ChannelID: docID,
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("upsert decoy agent: %v", err)
	}

	vtextRun, err := rt.StartRunWithMetadata(ctx, "Own this document", ownerID, map[string]any{
		runMetadataAgentProfile: AgentProfileVText,
		runMetadataAgentRole:    AgentProfileVText,
		runMetadataAgentID:      "vtext:" + docID,
		runMetadataChannelID:    docID,
	})
	if err != nil {
		t.Fatalf("start vtext run: %v", err)
	}
	superRun, err := rt.StartChildRun(ctx, vtextRun.RunID, "Report a result", ownerID, map[string]any{
		runMetadataAgentProfile: AgentProfileSuper,
		runMetadataAgentRole:    AgentProfileSuper,
		runMetadataAgentID:      "super:authoritative-parent",
		runMetadataChannelID:    docID,
	})
	if err != nil {
		t.Fatalf("start super child: %v", err)
	}

	superRegistry := rt.ToolRegistryForProfile(AgentProfileSuper)
	raw, err := superRegistry.Execute(WithToolExecutionContext(ctx, superRun), "submit_worker_update", json.RawMessage(`{
		"update_id":"super-authoritative-parent",
		"agent_id":"researcher:decoy",
		"artifacts":["artifacts/parent.txt"],
		"tests":["verified parent target routing"]
	}`))
	if err != nil {
		t.Fatalf("submit_worker_update: %v", err)
	}
	var resp struct {
		AgentID   string `json:"agent_id"`
		ChannelID string `json:"channel_id"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("decode submit_worker_update response: %v", err)
	}
	if resp.AgentID != "vtext:"+docID || resp.ChannelID != docID {
		t.Fatalf("response target = %q channel = %q, want parent vtext target/channel", resp.AgentID, resp.ChannelID)
	}

	update, err := s.GetWorkerUpdate(ctx, ownerID, "super-authoritative-parent")
	if err != nil {
		t.Fatalf("get worker update: %v", err)
	}
	if update.TargetAgentID != "vtext:"+docID || update.ChannelID != docID {
		t.Fatalf("worker update target/channel = %q/%q, want parent vtext", update.TargetAgentID, update.ChannelID)
	}
	messages, err := s.ListChannelMessages(ctx, ownerID, docID, 0, 10)
	if err != nil {
		t.Fatalf("list channel messages: %v", err)
	}
	found := false
	for _, message := range messages {
		if message.ToAgentID == "vtext:"+docID && message.FromRunID == superRun.RunID && message.Role == AgentProfileSuper {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("unexpected addressed messages: %+v", messages)
	}
}

func TestSubmitWorkerUpdateUsesVTextRequesterOverExplicitAgent(t *testing.T) {
	rt, s, cwd := testRuntimeWithTempCWD(t)
	if err := rt.InstallDefaultAgentTools(cwd); err != nil {
		t.Fatalf("install default agent tools: %v", err)
	}

	ctx := context.Background()
	ownerID := "user-alice"
	docID := "doc-requester-target"
	now := time.Now().UTC()
	if err := s.CreateDocument(ctx, types.Document{
		DocID:     docID,
		OwnerID:   ownerID,
		Title:     "Requester Target",
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create document: %v", err)
	}
	if err := s.UpsertAgent(ctx, types.AgentRecord{
		AgentID:   "vtext:" + docID,
		OwnerID:   ownerID,
		SandboxID: "sandbox-test",
		Profile:   AgentProfileVText,
		Role:      AgentProfileVText,
		ChannelID: docID,
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("upsert vtext agent: %v", err)
	}
	if err := s.UpsertAgent(ctx, types.AgentRecord{
		AgentID:   "researcher:decoy-requester",
		OwnerID:   ownerID,
		SandboxID: "sandbox-test",
		Profile:   AgentProfileResearcher,
		Role:      AgentProfileResearcher,
		ChannelID: "decoy-channel",
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("upsert decoy agent: %v", err)
	}

	superRun, err := rt.StartRunWithMetadata(ctx, "Report requester-scoped work", ownerID, map[string]any{
		runMetadataAgentProfile: AgentProfileSuper,
		runMetadataAgentRole:    AgentProfileSuper,
		runMetadataAgentID:      "super:requester-target",
		runMetadataChannelID:    docID,
		"requested_by_agent_id": "vtext:" + docID,
		"requested_by_profile":  AgentProfileVText,
		"request_source":        "vtext",
	})
	if err != nil {
		t.Fatalf("start requester super run: %v", err)
	}

	superRegistry := rt.ToolRegistryForProfile(AgentProfileSuper)
	raw, err := superRegistry.Execute(WithToolExecutionContext(ctx, superRun), "submit_worker_update", json.RawMessage(`{
		"update_id":"super-requester-target",
		"agent_id":"researcher:decoy-requester",
		"artifacts":["artifacts/requester.txt"],
		"tests":["verified requester target routing"]
	}`))
	if err != nil {
		t.Fatalf("submit_worker_update: %v", err)
	}
	var resp struct {
		AgentID   string `json:"agent_id"`
		ChannelID string `json:"channel_id"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("decode submit_worker_update response: %v", err)
	}
	if resp.AgentID != "vtext:"+docID || resp.ChannelID != docID {
		t.Fatalf("response target = %q channel = %q, want vtext requester target/channel", resp.AgentID, resp.ChannelID)
	}

	update, err := s.GetWorkerUpdate(ctx, ownerID, "super-requester-target")
	if err != nil {
		t.Fatalf("get worker update: %v", err)
	}
	if update.TargetAgentID != "vtext:"+docID || update.ChannelID != docID {
		t.Fatalf("worker update target/channel = %q/%q, want vtext requester", update.TargetAgentID, update.ChannelID)
	}
}

func TestResearcherWebSearchRoutesThroughGateway(t *testing.T) {
	var gotAuth string
	var gotMethod string
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotMethod = r.Method
		gotPath = r.URL.Path
		if r.URL.Path != "/provider/v1/search" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"query":    "latest model releases",
			"provider": "mock-gateway",
			"results": []map[string]any{
				{
					"title":   "Release notes",
					"url":     "https://example.com/release",
					"snippet": "Grounded search result from the gateway path.",
				},
			},
		})
	}))
	defer server.Close()

	t.Setenv("RUNTIME_GATEWAY_URL", server.URL)
	t.Setenv("RUNTIME_GATEWAY_TOKEN", "sandbox-token")

	rt, _, cwd := testRuntimeWithTempCWD(t)
	if err := rt.InstallDefaultAgentTools(cwd); err != nil {
		t.Fatalf("install default agent tools: %v", err)
	}

	researcherTask, err := rt.StartRunWithMetadata(context.Background(), "Search the web", "user-alice", map[string]any{
		runMetadataAgentProfile: AgentProfileResearcher,
		runMetadataAgentRole:    AgentProfileResearcher,
	})
	if err != nil {
		t.Fatalf("submit researcher task: %v", err)
	}

	researcherRegistry := rt.ToolRegistryForProfile(AgentProfileResearcher)
	raw, err := researcherRegistry.Execute(WithToolExecutionContext(context.Background(), researcherTask), "web_search", json.RawMessage(`{
		"query":"latest model releases",
		"max_results":3
	}`))
	if err != nil {
		t.Fatalf("web_search: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Fatalf("gateway search method = %q, want POST", gotMethod)
	}
	if gotPath != "/provider/v1/search" {
		t.Fatalf("gateway search path = %q, want /provider/v1/search", gotPath)
	}
	if gotAuth != "Bearer sandbox-token" {
		t.Fatalf("gateway search auth = %q, want Bearer sandbox-token", gotAuth)
	}

	var resp struct {
		Provider string `json:"provider"`
		Results  []struct {
			URL string `json:"url"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("decode web_search: %v", err)
	}
	if resp.Provider != "mock-gateway" {
		t.Fatalf("provider = %q, want mock-gateway", resp.Provider)
	}
	if len(resp.Results) != 1 || resp.Results[0].URL != "https://example.com/release" {
		t.Fatalf("results = %+v, want gateway search result", resp.Results)
	}
}

func TestResearcherWebSearchFallsBackToProxyGatewayURL(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewEncoder(w).Encode(map[string]any{
			"query":    "latest model releases",
			"provider": "proxy-gateway",
			"results":  []map[string]any{},
		})
	}))
	defer server.Close()

	t.Setenv("RUNTIME_GATEWAY_URL", "")
	t.Setenv("PROXY_VMCTL_URL", server.URL)
	t.Setenv("RUNTIME_GATEWAY_TOKEN", "sandbox-token")

	rt, _, cwd := testRuntimeWithTempCWD(t)
	if err := rt.InstallDefaultAgentTools(cwd); err != nil {
		t.Fatalf("install default agent tools: %v", err)
	}

	researcherTask, err := rt.StartRunWithMetadata(context.Background(), "Search the web", "user-alice", map[string]any{
		runMetadataAgentProfile: AgentProfileResearcher,
		runMetadataAgentRole:    AgentProfileResearcher,
	})
	if err != nil {
		t.Fatalf("submit researcher task: %v", err)
	}

	researcherRegistry := rt.ToolRegistryForProfile(AgentProfileResearcher)
	raw, err := researcherRegistry.Execute(WithToolExecutionContext(context.Background(), researcherTask), "web_search", json.RawMessage(`{
		"query":"latest model releases"
	}`))
	if err != nil {
		t.Fatalf("web_search: %v", err)
	}

	if gotPath != "/provider/v1/search" {
		t.Fatalf("proxy gateway search path = %q, want /provider/v1/search", gotPath)
	}

	var resp struct {
		Provider string `json:"provider"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("decode web_search: %v", err)
	}
	if resp.Provider != "proxy-gateway" {
		t.Fatalf("provider = %q, want proxy-gateway", resp.Provider)
	}
}

func TestResearcherWebSearchWithoutGatewayIsUnavailable(t *testing.T) {
	t.Setenv("RUNTIME_GATEWAY_URL", "")
	t.Setenv("PROXY_VMCTL_URL", "")
	t.Setenv("RUNTIME_GATEWAY_TOKEN", "")
	for _, key := range []string{"TAVILY_API_KEY", "BRAVE_API_KEY", "EXA_API_KEY", "SERPER_API_KEY"} {
		t.Setenv(key, "")
	}

	rt, _, cwd := testRuntimeWithTempCWD(t)
	if err := rt.InstallDefaultAgentTools(cwd); err != nil {
		t.Fatalf("install default agent tools: %v", err)
	}

	researcherTask, err := rt.StartRunWithMetadata(context.Background(), "Search the web", "user-alice", map[string]any{
		runMetadataAgentProfile: AgentProfileResearcher,
		runMetadataAgentRole:    AgentProfileResearcher,
	})
	if err != nil {
		t.Fatalf("submit researcher task: %v", err)
	}

	researcherRegistry := rt.ToolRegistryForProfile(AgentProfileResearcher)
	_, err = researcherRegistry.Execute(WithToolExecutionContext(context.Background(), researcherTask), "web_search", json.RawMessage(`{
		"query":"latest model releases"
	}`))
	if err == nil || !strings.Contains(err.Error(), "search client not configured") {
		t.Fatalf("web_search err = %v, want search client not configured", err)
	}
}

func TestExportPatchsetToolExportsWithoutGitHubPush(t *testing.T) {
	rt, _, cwd := testRuntimeWithTempCWD(t)
	rt.cfg.SandboxID = "vm-tool-export"
	if err := rt.InstallDefaultAgentTools(cwd); err != nil {
		t.Fatalf("install default agent tools: %v", err)
	}

	repo := filepath.Join(cwd, "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatalf("create repo: %v", err)
	}
	runGit(t, repo, "init")
	runGit(t, repo, "config", "user.name", "Choir Test")
	runGit(t, repo, "config", "user.email", "choir-test@example.com")
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("base\n"), 0o644); err != nil {
		t.Fatalf("write base file: %v", err)
	}
	runGit(t, repo, "add", "README.md")
	runGit(t, repo, "commit", "-m", "base")
	base := strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD"))
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("base\nworker proof\n"), 0o644); err != nil {
		t.Fatalf("write changed file: %v", err)
	}
	runGit(t, repo, "add", "README.md")
	runGit(t, repo, "commit", "-m", "worker change")

	superRun, err := rt.StartRunWithMetadata(context.Background(), "export worker patchset", "user-alice", map[string]any{
		runMetadataAgentProfile: AgentProfileCoSuper,
		runMetadataAgentRole:    AgentProfileCoSuper,
		runMetadataTrajectoryID: "trace-export-tool",
	})
	if err != nil {
		t.Fatalf("start co-super run: %v", err)
	}
	registry := rt.ToolRegistryForProfile(AgentProfileCoSuper)
	raw, err := registry.Execute(WithToolExecutionContext(context.Background(), superRun), "export_patchset", json.RawMessage(fmt.Sprintf(`{
		"repo_path": "repo",
		"output_dir": "exports/proof",
		"base_sha": %q,
		"snapshot_id": "snapshot-tool",
		"summary": "worker export proof",
		"checks": ["grep -q worker README.md"]
	}`, base)))
	if err != nil {
		t.Fatalf("export_patchset: %v", err)
	}

	var result struct {
		Status       string `json:"status"`
		ManifestPath string `json:"manifest_path"`
		PatchsetPath string `json:"patchset_path"`
		GitHubPush   bool   `json:"github_push"`
	}
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("decode export result: %v\n%s", err, raw)
	}
	if result.Status != "exported" || result.GitHubPush {
		t.Fatalf("unexpected export result: %+v", result)
	}
	for _, path := range []string{result.ManifestPath, result.PatchsetPath} {
		if !strings.HasPrefix(path, cwd+string(os.PathSeparator)) {
			t.Fatalf("export path escaped cwd: %s", path)
		}
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected export artifact %s: %v", path, err)
		}
	}

	data, err := os.ReadFile(result.ManifestPath)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var manifest struct {
		RunID           string `json:"run_id"`
		TraceID         string `json:"trace_id"`
		VMID            string `json:"vm_id"`
		SnapshotID      string `json:"snapshot_id"`
		BaseSHA         string `json:"base_sha"`
		ExpectedHeadSHA string `json:"expected_head_sha"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	if manifest.RunID != superRun.RunID || manifest.TraceID != "trace-export-tool" || manifest.VMID != "vm-tool-export" || manifest.SnapshotID != "snapshot-tool" || manifest.BaseSHA != base || manifest.ExpectedHeadSHA == "" {
		t.Fatalf("manifest provenance mismatch: %+v", manifest)
	}
}

func TestDelegateWorkerVMToolRunsWorkerRuntimeAndCollectsExport(t *testing.T) {
	activeRT, _, activeCWD := testRuntimeWithTempCWD(t)
	if err := activeRT.InstallDefaultAgentTools(activeCWD); err != nil {
		t.Fatalf("install active tools: %v", err)
	}

	workerDir := t.TempDir()
	workerDB, err := store.Open(filepath.Join(workerDir, "worker.db"))
	if err != nil {
		t.Fatalf("open worker store: %v", err)
	}
	workerCWD := filepath.Join(workerDir, "files")
	if err := os.MkdirAll(workerCWD, 0o755); err != nil {
		t.Fatalf("create worker cwd: %v", err)
	}

	repo := filepath.Join(workerCWD, "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatalf("create worker repo: %v", err)
	}
	runGit(t, repo, "init")
	runGit(t, repo, "config", "user.name", "Choir Worker")
	runGit(t, repo, "config", "user.email", "worker@example.com")
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("base\n"), 0o644); err != nil {
		t.Fatalf("write worker base: %v", err)
	}
	runGit(t, repo, "add", "README.md")
	runGit(t, repo, "commit", "-m", "base")
	base := strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD"))
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("base\nbackground worker change\n"), 0o644); err != nil {
		t.Fatalf("write worker change: %v", err)
	}
	runGit(t, repo, "add", "README.md")
	runGit(t, repo, "commit", "-m", "worker change")

	exportArgs := fmt.Sprintf(`{
		"repo_path": "repo",
		"output_dir": "exports/background-proof",
		"base_sha": %q,
		"snapshot_id": "snapshot-worker-proof",
		"summary": "background worker export proof",
		"checks": ["grep -q background README.md"]
	}`, base)
	workerProvider := newMockToolLoopProvider(
		&ToolLoopResponse{
			StopReason: "tool_use",
			ToolCalls: []types.ToolCall{{
				ID:        "call-export",
				Name:      "export_patchset",
				Arguments: json.RawMessage(exportArgs),
			}},
		},
		&ToolLoopResponse{
			StopReason: "end_turn",
			Text:       "Exported background worker patchset.",
		},
	)
	workerRT := New(Config{
		SandboxID:           "vm-worker-proof",
		StorePath:           filepath.Join(workerDir, "worker.db"),
		ProviderTimeout:     5 * time.Second,
		SupervisionInterval: time.Hour,
	}, workerDB, events.NewEventBus(), workerProvider)
	if err := workerRT.InstallDefaultAgentTools(workerCWD); err != nil {
		t.Fatalf("install worker tools: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	workerRT.Start(ctx)
	t.Cleanup(func() {
		workerRT.Stop()
		_ = workerDB.Close()
	})

	workerHandler := NewAPIHandler(workerRT)
	mux := http.NewServeMux()
	mux.HandleFunc("/internal/runtime/runs", workerHandler.HandleInternalRunSubmission)
	mux.HandleFunc("/internal/runtime/runs/", workerHandler.HandleInternalRuntimeRunRouter)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	superRun, err := activeRT.StartRunWithMetadata(context.Background(), "delegate to background worker", "user-alice", map[string]any{
		runMetadataAgentProfile: AgentProfileSuper,
		runMetadataAgentRole:    AgentProfileSuper,
		runMetadataTrajectoryID: "trace-worker-delegation",
	})
	if err != nil {
		t.Fatalf("start active super run: %v", err)
	}
	registry := activeRT.ToolRegistryForProfile(AgentProfileSuper)
	raw, err := registry.Execute(WithToolExecutionContext(context.Background(), superRun), "delegate_worker_vm", json.RawMessage(fmt.Sprintf(`{
		"worker_sandbox_url": %q,
		"worker_id": "worker-proof",
		"vm_id": "vm-worker-proof",
		"objective": "Export the committed worker patchset.",
		"profile": "co-super",
		"timeout_seconds": 10
	}`, srv.URL)))
	if err != nil {
		t.Fatalf("delegate_worker_vm: %v", err)
	}

	var result struct {
		State           types.RunState   `json:"state"`
		WorkerVMID      string           `json:"worker_vm_id"`
		ExportPatchsets []map[string]any `json:"export_patchsets"`
		PromotionQueue  []map[string]any `json:"promotion_queue"`
	}
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("decode delegate result: %v\n%s", err, raw)
	}
	if result.State != types.RunCompleted || result.WorkerVMID != "vm-worker-proof" || len(result.ExportPatchsets) != 1 {
		t.Fatalf("unexpected delegate result: %+v\nraw=%s", result, raw)
	}
	if pushed, _ := result.ExportPatchsets[0]["github_push"].(bool); pushed {
		t.Fatalf("worker export reported github push: %+v", result.ExportPatchsets[0])
	}
	if got, _ := result.ExportPatchsets[0]["worker_head"].(string); strings.TrimSpace(got) == "" {
		t.Fatalf("worker export missing worker_head: %+v", result.ExportPatchsets[0])
	}
	if len(result.PromotionQueue) != 1 {
		t.Fatalf("expected one queued promotion candidate, got %+v", result.PromotionQueue)
	}
	if got, _ := result.PromotionQueue[0]["candidate_id"].(string); strings.TrimSpace(got) == "" {
		t.Fatalf("queued promotion missing candidate_id: %+v", result.PromotionQueue[0])
	}
}

func TestDelegateWorkerVMAddsRemoteRepoBootstrapForDistinctWorker(t *testing.T) {
	activeRT, _, activeCWD := testRuntimeWithTempCWD(t)
	if err := os.RemoveAll(activeCWD); err != nil {
		t.Fatalf("reset active cwd: %v", err)
	}
	if err := os.MkdirAll(activeCWD, 0o755); err != nil {
		t.Fatalf("recreate active cwd: %v", err)
	}
	runGit(t, activeCWD, "init")
	runGit(t, activeCWD, "config", "user.name", "Choir Active")
	runGit(t, activeCWD, "config", "user.email", "active@example.com")
	if err := os.WriteFile(filepath.Join(activeCWD, "README.md"), []byte("foreground base\n"), 0o644); err != nil {
		t.Fatalf("write active base: %v", err)
	}
	runGit(t, activeCWD, "add", "README.md")
	runGit(t, activeCWD, "commit", "-m", "active base")
	runGit(t, activeCWD, "remote", "add", "origin", "https://github.com/yusefmosiah/go-choir.git")
	base := strings.TrimSpace(runGit(t, activeCWD, "rev-parse", "HEAD"))
	if err := activeRT.InstallDefaultAgentTools(activeCWD); err != nil {
		t.Fatalf("install active tools: %v", err)
	}

	workerDir := t.TempDir()
	workerDB, err := store.Open(filepath.Join(workerDir, "worker.db"))
	if err != nil {
		t.Fatalf("open worker store: %v", err)
	}
	workerCWD := filepath.Join(workerDir, "files")
	if err := os.MkdirAll(workerCWD, 0o755); err != nil {
		t.Fatalf("create worker cwd: %v", err)
	}
	workerProvider := newMockToolLoopProvider(&ToolLoopResponse{
		StopReason: "end_turn",
		Text:       "Received bootstrap instructions.",
	})
	workerRT := New(Config{
		SandboxID:           "vm-worker-bootstrap",
		StorePath:           filepath.Join(workerDir, "worker.db"),
		ProviderTimeout:     5 * time.Second,
		SupervisionInterval: time.Hour,
	}, workerDB, events.NewEventBus(), workerProvider)
	if err := workerRT.InstallDefaultAgentTools(workerCWD); err != nil {
		t.Fatalf("install worker tools: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	workerRT.Start(ctx)
	t.Cleanup(func() {
		workerRT.Stop()
		_ = workerDB.Close()
	})

	workerHandler := NewAPIHandler(workerRT)
	mux := http.NewServeMux()
	mux.HandleFunc("/internal/runtime/runs", workerHandler.HandleInternalRunSubmission)
	mux.HandleFunc("/internal/runtime/runs/", workerHandler.HandleInternalRuntimeRunRouter)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	superRun, err := activeRT.StartRunWithMetadata(context.Background(), "delegate to distinct worker", "user-alice", map[string]any{
		runMetadataAgentProfile: AgentProfileSuper,
		runMetadataAgentRole:    AgentProfileSuper,
		runMetadataTrajectoryID: "trace-worker-bootstrap",
	})
	if err != nil {
		t.Fatalf("start active super run: %v", err)
	}
	registry := activeRT.ToolRegistryForProfile(AgentProfileSuper)
	raw, err := registry.Execute(WithToolExecutionContext(context.Background(), superRun), "delegate_worker_vm", json.RawMessage(fmt.Sprintf(`{
		"worker_sandbox_url": %q,
		"worker_id": "worker-bootstrap",
		"vm_id": "vm-worker-bootstrap",
		"objective": "Inspect the candidate checkout.",
		"profile": "vsuper",
		"timeout_seconds": 10
	}`, srv.URL)))
	if err != nil {
		t.Fatalf("delegate_worker_vm: %v", err)
	}

	var result struct {
		State               types.RunState `json:"state"`
		RunID               string         `json:"loop_id"`
		WorkerRepoBootstrap string         `json:"worker_repo_bootstrap"`
		WorkerRepoRemoteURL string         `json:"worker_repo_remote_url"`
		WorkerRepoBaseSHA   string         `json:"worker_repo_base_sha"`
	}
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("decode delegate result: %v\n%s", err, raw)
	}
	if result.State != types.RunCompleted || result.WorkerRepoBootstrap != "remote_git_clone" {
		t.Fatalf("unexpected bootstrap result: %+v\nraw=%s", result, raw)
	}
	if result.WorkerRepoRemoteURL != "https://github.com/yusefmosiah/go-choir.git" || result.WorkerRepoBaseSHA != base {
		t.Fatalf("bootstrap provenance mismatch: %+v", result)
	}
	workerRun, err := workerRT.GetRun(context.Background(), result.RunID, "user-alice")
	if err != nil {
		t.Fatalf("get worker run: %v", err)
	}
	for _, want := range []string{
		"Remote worker repository bootstrap is available.",
		"git clone https://github.com/yusefmosiah/go-choir.git go-choir-candidate",
		"git checkout " + base,
		"Use repo_path \"go-choir-candidate\" and base_sha " + base,
		"Inspect the candidate checkout.",
	} {
		if !strings.Contains(workerRun.Prompt, want) {
			t.Fatalf("worker prompt missing %q in %q", want, workerRun.Prompt)
		}
	}
}

func TestDelegateWorkerVMRefusesSameRuntimeWithoutIsolation(t *testing.T) {
	activeRT, _, activeCWD := testRuntimeWithTempCWD(t)
	if err := activeRT.InstallDefaultAgentTools(activeCWD); err != nil {
		t.Fatalf("install active tools: %v", err)
	}
	t.Setenv("RUNTIME_SELF_URL", "http://127.0.0.1:8085")
	t.Setenv("RUNTIME_LOCAL_WORKER_MODE", "")

	superRun, err := activeRT.StartRunWithMetadata(context.Background(), "delegate to local worker", "user-alice", map[string]any{
		runMetadataAgentProfile: AgentProfileSuper,
		runMetadataAgentRole:    AgentProfileSuper,
	})
	if err != nil {
		t.Fatalf("start active super run: %v", err)
	}
	registry := activeRT.ToolRegistryForProfile(AgentProfileSuper)
	_, err = registry.Execute(WithToolExecutionContext(context.Background(), superRun), "delegate_worker_vm", json.RawMessage(`{
		"worker_sandbox_url": "http://127.0.0.1:8085",
		"worker_id": "worker-local",
		"vm_id": "vm-local",
		"objective": "mutate in local fallback",
		"profile": "co-super",
		"timeout_seconds": 1
	}`))
	if err == nil || !strings.Contains(err.Error(), "refused same-runtime worker delegation without isolation") {
		t.Fatalf("delegate_worker_vm error = %v, want same-runtime isolation refusal", err)
	}
}

func TestDelegateWorkerVMLocalWorktreeIsolationUsesToolCWD(t *testing.T) {
	activeRT, _, activeCWD := testRuntimeWithTempCWD(t)
	if err := os.RemoveAll(activeCWD); err != nil {
		t.Fatalf("reset active cwd: %v", err)
	}
	if err := os.MkdirAll(activeCWD, 0o755); err != nil {
		t.Fatalf("recreate active cwd: %v", err)
	}
	runGit(t, activeCWD, "init")
	runGit(t, activeCWD, "config", "user.name", "Choir Active")
	runGit(t, activeCWD, "config", "user.email", "active@example.com")
	if err := os.WriteFile(filepath.Join(activeCWD, "README.md"), []byte("foreground base\n"), 0o644); err != nil {
		t.Fatalf("write active base: %v", err)
	}
	runGit(t, activeCWD, "add", "README.md")
	runGit(t, activeCWD, "commit", "-m", "active base")
	base := strings.TrimSpace(runGit(t, activeCWD, "rev-parse", "HEAD"))
	if err := activeRT.InstallDefaultAgentTools(activeCWD); err != nil {
		t.Fatalf("install active tools: %v", err)
	}

	workerDir := t.TempDir()
	workerDB, err := store.Open(filepath.Join(workerDir, "worker.db"))
	if err != nil {
		t.Fatalf("open worker store: %v", err)
	}
	workerCWD := filepath.Join(workerDir, "files")
	if err := os.MkdirAll(workerCWD, 0o755); err != nil {
		t.Fatalf("create worker cwd: %v", err)
	}

	bashArgs, _ := json.Marshal(map[string]any{
		"command": strings.Join([]string{
			"printf 'local worker proof\\n' > isolated-worker-proof.txt",
			"git add isolated-worker-proof.txt",
			"git commit -m 'local worker isolated change'",
		}, " && "),
		"timeout_ms": 15000,
	})
	exportArgs, _ := json.Marshal(map[string]any{
		"repo_path":   ".",
		"output_dir":  ".choir/exports/local-worktree-proof",
		"base_sha":    base,
		"snapshot_id": "snapshot-local-worktree",
		"summary":     "local worktree isolation proof",
		"checks":      []string{"test -f isolated-worker-proof.txt"},
	})
	workerProvider := newMockToolLoopProvider(
		&ToolLoopResponse{
			StopReason: "tool_use",
			ToolCalls: []types.ToolCall{{
				ID:        "call-bash",
				Name:      "bash",
				Arguments: bashArgs,
			}},
		},
		&ToolLoopResponse{
			StopReason: "tool_use",
			ToolCalls: []types.ToolCall{{
				ID:        "call-export",
				Name:      "export_patchset",
				Arguments: exportArgs,
			}},
		},
		&ToolLoopResponse{
			StopReason: "end_turn",
			Text:       "Exported local worktree patchset.",
		},
	)
	workerRT := New(Config{
		SandboxID:           "sandbox-local-runtime",
		StorePath:           filepath.Join(workerDir, "worker.db"),
		ProviderTimeout:     5 * time.Second,
		SupervisionInterval: time.Hour,
	}, workerDB, events.NewEventBus(), workerProvider)
	if err := workerRT.InstallDefaultAgentTools(workerCWD); err != nil {
		t.Fatalf("install worker tools: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	workerRT.Start(ctx)
	t.Cleanup(func() {
		workerRT.Stop()
		_ = workerDB.Close()
	})

	workerHandler := NewAPIHandler(workerRT)
	mux := http.NewServeMux()
	mux.HandleFunc("/internal/runtime/runs", workerHandler.HandleInternalRunSubmission)
	mux.HandleFunc("/internal/runtime/runs/", workerHandler.HandleInternalRuntimeRunRouter)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	t.Setenv("RUNTIME_SELF_URL", srv.URL)
	t.Setenv("RUNTIME_LOCAL_WORKER_MODE", "worktree")
	t.Setenv("RUNTIME_LOCAL_WORKER_ROOT", filepath.Join(t.TempDir(), "worktrees"))

	superRun, err := activeRT.StartRunWithMetadata(context.Background(), "delegate to local worker", "user-alice", map[string]any{
		runMetadataAgentProfile: AgentProfileSuper,
		runMetadataAgentRole:    AgentProfileSuper,
		runMetadataTrajectoryID: "trace-local-worktree",
	})
	if err != nil {
		t.Fatalf("start active super run: %v", err)
	}
	registry := activeRT.ToolRegistryForProfile(AgentProfileSuper)
	raw, err := registry.Execute(WithToolExecutionContext(context.Background(), superRun), "delegate_worker_vm", json.RawMessage(fmt.Sprintf(`{
		"worker_sandbox_url": %q,
		"worker_id": "worker-local-worktree",
		"vm_id": "vm-local-worktree",
		"objective": "Commit and export a local worktree proof.",
		"profile": "co-super",
		"timeout_seconds": 10
	}`, srv.URL)))
	if err != nil {
		t.Fatalf("delegate_worker_vm: %v", err)
	}

	var result struct {
		State           types.RunState   `json:"state"`
		RunID           string           `json:"loop_id"`
		WorkerIsolation string           `json:"worker_isolation"`
		WorkerWorktree  string           `json:"worker_worktree_path"`
		WorkerBranch    string           `json:"worker_branch"`
		WorkerBaseSHA   string           `json:"worker_base_sha"`
		ExportPatchsets []map[string]any `json:"export_patchsets"`
		PromotionQueue  []map[string]any `json:"promotion_queue"`
	}
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("decode delegate result: %v\n%s", err, raw)
	}
	if result.State != types.RunCompleted || result.WorkerIsolation != "local_worktree" {
		t.Fatalf("unexpected local worktree result: %+v\nraw=%s", result, raw)
	}
	if result.WorkerWorktree == "" || result.WorkerBranch == "" || result.WorkerBaseSHA != base {
		t.Fatalf("missing worktree provenance: %+v", result)
	}
	if _, err := os.Stat(filepath.Join(activeCWD, "isolated-worker-proof.txt")); !os.IsNotExist(err) {
		t.Fatalf("foreground repo was mutated; stat err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(result.WorkerWorktree, "isolated-worker-proof.txt")); err != nil {
		t.Fatalf("worker proof missing from isolated worktree: %v", err)
	}
	if len(result.ExportPatchsets) != 1 || len(result.PromotionQueue) != 1 {
		t.Fatalf("expected exported patchset and queued promotion: %+v", result)
	}
	if got, _ := result.PromotionQueue[0]["vm_id"].(string); got != "vm-local-worktree" {
		t.Fatalf("queued promotion vm_id = %q, want vm-local-worktree; queue=%+v", got, result.PromotionQueue[0])
	}
}

func TestQueuePromotionCandidatesForWorkerExportsDedupesExactExport(t *testing.T) {
	rt, _, _ := testRuntimeWithTempCWD(t)
	ctx := context.Background()
	export := map[string]any{
		"loop_id":         "candidate-loop-dedupe",
		"vm_id":           "sandbox-dev",
		"snapshot_id":     "snapshot-dedupe",
		"base_sha":        "base-dedupe",
		"worker_head":     "worker-head-dedupe",
		"manifest_path":   "/tmp/dedupe-manifest.json",
		"patchset_path":   "/tmp/dedupe.patch",
		"worker_head_sha": "worker-head-dedupe",
	}
	in := workerExportQueueContext{
		OwnerID:        "user-alice",
		ParentRunID:    "super-run-dedupe",
		CandidateRunID: "candidate-loop-dedupe",
		TraceID:        "trace-dedupe",
		WorkerVMID:     "vm-worker-dedupe",
		WorkerID:       "worker-dedupe",
		Objective:      "dedupe exact worker export",
		Exports:        []map[string]any{export},
	}

	first, err := queuePromotionCandidatesForWorkerExports(ctx, rt, in)
	if err != nil {
		t.Fatalf("queue first candidate: %v", err)
	}
	second, err := queuePromotionCandidatesForWorkerExports(ctx, rt, in)
	if err != nil {
		t.Fatalf("queue duplicate candidate: %v", err)
	}
	if len(first) != 1 || len(second) != 1 {
		t.Fatalf("queue results = first %+v second %+v, want one result each", first, second)
	}
	if first[0]["candidate_id"] != second[0]["candidate_id"] {
		t.Fatalf("duplicate export queued new candidate: first=%+v second=%+v", first[0], second[0])
	}
	if first[0]["vm_id"] != "vm-worker-dedupe" || second[0]["vm_id"] != "vm-worker-dedupe" {
		t.Fatalf("queued candidate should preserve worker VM id: first=%+v second=%+v", first[0], second[0])
	}
	candidates, err := rt.Store().ListPromotionCandidates(ctx, "user-alice", 10)
	if err != nil {
		t.Fatalf("list candidates: %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("stored candidates = %+v, want one deduped candidate", candidates)
	}
}

func TestQueuePromotionCandidatesDedupesEquivalentPatchsetFingerprint(t *testing.T) {
	rt, _, _ := testRuntimeWithTempCWD(t)
	ctx := context.Background()
	dir := t.TempDir()
	patchA := filepath.Join(dir, "candidate-a.patch")
	patchB := filepath.Join(dir, "candidate-b.patch")
	patchContent := []byte("diff --git a/product.txt b/product.txt\n+same product patch\n")
	if err := os.WriteFile(patchA, patchContent, 0o644); err != nil {
		t.Fatalf("write patch A: %v", err)
	}
	if err := os.WriteFile(patchB, patchContent, 0o644); err != nil {
		t.Fatalf("write patch B: %v", err)
	}

	in := workerExportQueueContext{
		OwnerID:     "user-alice",
		ParentRunID: "super-run-fingerprint",
		TraceID:     "trace-fingerprint",
		WorkerVMID:  "vm-worker-fingerprint",
		WorkerID:    "worker-fingerprint",
		Objective:   "Run the same product patch",
		Exports: []map[string]any{{
			"loop_id":         "candidate-loop-a",
			"base_sha":        "base-fingerprint",
			"worker_head":     "worker-head-a",
			"manifest_path":   filepath.Join(dir, "manifest-a.json"),
			"patchset_path":   patchA,
			"worker_head_sha": "worker-head-a",
		}},
	}
	first, err := queuePromotionCandidatesForWorkerExports(ctx, rt, in)
	if err != nil {
		t.Fatalf("queue first candidate: %v", err)
	}
	in.Objective = " run THE same/product patch!! "
	in.Exports = []map[string]any{{
		"loop_id":         "candidate-loop-b",
		"base_sha":        "base-fingerprint",
		"worker_head":     "worker-head-b",
		"manifest_path":   filepath.Join(dir, "manifest-b.json"),
		"patchset_path":   patchB,
		"worker_head_sha": "worker-head-b",
	}}
	second, err := queuePromotionCandidatesForWorkerExports(ctx, rt, in)
	if err != nil {
		t.Fatalf("queue equivalent candidate: %v", err)
	}
	if len(first) != 1 || len(second) != 1 {
		t.Fatalf("queue results = first %+v second %+v, want one result each", first, second)
	}
	if first[0]["candidate_id"] != second[0]["candidate_id"] {
		t.Fatalf("equivalent export queued new candidate: first=%+v second=%+v", first[0], second[0])
	}
	if first[0]["objective_fingerprint"] == "" || first[0]["patchset_sha256"] == "" {
		t.Fatalf("candidate queue missing fingerprint evidence: %+v", first[0])
	}
	candidates, err := rt.Store().ListPromotionCandidates(ctx, "user-alice", 10)
	if err != nil {
		t.Fatalf("list candidates: %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("stored candidates = %+v, want one fingerprint-deduped candidate", candidates)
	}
}

func TestQueuePromotionCandidatesMaterializesInlineWorkerExportArtifacts(t *testing.T) {
	rt, _, _ := testRuntimeWithTempCWD(t)
	ctx := context.Background()
	_ = os.RemoveAll(promotionArtifactRoot(rt))
	patchContent := "diff --git a/product.txt b/product.txt\n--- a/product.txt\n+++ b/product.txt\n@@ -0,0 +1 @@\n+materialized worker patch\n"
	manifestContent := `{"run_id":"candidate-loop-materialized","trace_id":"trace-materialized","vm_id":"vm-worker-materialized","base_sha":"base-materialized"}`
	in := workerExportQueueContext{
		OwnerID:     "user-alice",
		ParentRunID: "super-run-materialized",
		TraceID:     "trace-materialized",
		WorkerVMID:  "vm-worker-materialized",
		WorkerID:    "worker-materialized",
		Objective:   "Materialize an inline worker export",
		Exports: []map[string]any{{
			"loop_id":          "candidate-loop-materialized",
			"base_sha":         "base-materialized",
			"worker_head":      "worker-head-materialized",
			"worker_head_sha":  "worker-head-materialized",
			"manifest_path":    "/worker-only/manifest.json",
			"patchset_path":    "/worker-only/changes.patch",
			"manifest_json":    manifestContent,
			"patchset_content": patchContent,
		}},
	}

	queued, err := queuePromotionCandidatesForWorkerExports(ctx, rt, in)
	if err != nil {
		t.Fatalf("queue candidate: %v", err)
	}
	if len(queued) != 1 {
		t.Fatalf("queued = %+v, want one candidate", queued)
	}
	candidates, err := rt.Store().ListPromotionCandidates(ctx, "user-alice", 10)
	if err != nil {
		t.Fatalf("list candidates: %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("stored candidates = %+v, want one", candidates)
	}
	patchBytes, err := os.ReadFile(candidates[0].PatchsetPath)
	if err != nil {
		t.Fatalf("read materialized patch: %v", err)
	}
	if string(patchBytes) != patchContent {
		t.Fatalf("materialized patch = %q, want %q", string(patchBytes), patchContent)
	}
	manifestBytes, err := os.ReadFile(candidates[0].ManifestPath)
	if err != nil {
		t.Fatalf("read materialized manifest: %v", err)
	}
	if string(manifestBytes) != manifestContent {
		t.Fatalf("materialized manifest = %q, want %q", string(manifestBytes), manifestContent)
	}
	var world promotion.CandidateWorld
	if err := json.Unmarshal(candidates[0].CandidateJSON, &world); err != nil {
		t.Fatalf("decode candidate world: %v", err)
	}
	if world.PatchsetSHA256 == "" || world.PatchsetPath != candidates[0].PatchsetPath {
		t.Fatalf("candidate world missing materialized patch evidence: %+v", world)
	}
	if _, err := queuePromotionCandidatesForWorkerExports(ctx, rt, in); err != nil {
		t.Fatalf("queue duplicate candidate: %v", err)
	}
	artifactDirs, err := os.ReadDir(promotionArtifactRoot(rt))
	if err != nil {
		t.Fatalf("read promotion artifact root: %v", err)
	}
	if len(artifactDirs) != 1 {
		t.Fatalf("artifact dirs = %d, want one materialized candidate after duplicate export", len(artifactDirs))
	}
}

func testRuntimeWithTempCWD(t *testing.T) (*Runtime, *store.Store, string) {
	t.Helper()

	dir := filepath.Join(os.TempDir(), "go-choir-m3-agent-tools-test")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	dbPath := filepath.Join(dir, t.Name()+".db")
	cwd := filepath.Join(dir, t.Name()+"-cwd")
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		t.Fatalf("create tool cwd: %v", err)
	}
	_ = os.Remove(dbPath)

	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	rt := New(Config{
		SandboxID:           "sandbox-test",
		StorePath:           dbPath,
		ProviderTimeout:     5 * time.Second,
		SupervisionInterval: time.Hour,
	}, s, events.NewEventBus(), NewStubProvider(10*time.Millisecond))

	t.Cleanup(func() {
		rt.Stop()
		_ = s.Close()
		_ = os.Remove(dbPath)
	})

	return rt, s, cwd
}

func runGit(t *testing.T, repo string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = repo
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}
