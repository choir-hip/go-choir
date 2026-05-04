package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

func TestInstallDefaultAgentToolsProfiles(t *testing.T) {
	rt, _, cwd := testRuntimeWithTempCWD(t)
	if err := rt.InstallDefaultAgentTools(cwd); err != nil {
		t.Fatalf("install default agent tools: %v", err)
	}

	super := rt.ToolRegistryForProfile(AgentProfileSuper)
	coSuper := rt.ToolRegistryForProfile(AgentProfileCoSuper)
	conductor := rt.ToolRegistryForProfile(AgentProfileConductor)
	researcher := rt.ToolRegistryForProfile(AgentProfileResearcher)
	vtext := rt.ToolRegistryForProfile(AgentProfileVText)

	for _, name := range []string{"bash", "read_file", "web_search", "spawn_agent", "cast_agent", "save_evidence", "submit_worker_update", "fork_desktop", "publish_desktop", "request_worker_vm"} {
		if _, ok := super.Lookup(name); !ok {
			t.Fatalf("super missing tool %q", name)
		}
	}
	for _, name := range []string{"bash", "read_file", "web_search", "spawn_agent", "cast_agent", "save_evidence", "submit_worker_update"} {
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
	for _, name := range []string{"read_file", "web_search", "spawn_agent", "cast_agent", "save_evidence", "submit_research_findings", "submit_worker_update"} {
		if _, ok := researcher.Lookup(name); !ok {
			t.Fatalf("researcher missing tool %q", name)
		}
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
	if _, ok := conductor.Lookup("edit_vtext"); ok {
		t.Fatalf("conductor should not have edit_vtext")
	}
	if _, ok := super.Lookup("edit_vtext"); ok {
		t.Fatalf("super should not have edit_vtext")
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
