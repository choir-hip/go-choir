//go:build comprehensive

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

func TestWorkerRepoBootstrapPromptIncludesHumanEvidenceBrowserContract(t *testing.T) {
	prompt := remoteWorkerRepoBootstrapPrompt("https://github.com/yusefmosiah/go-choir.git", "abc123")
	for _, want := range []string{
		"node, npm",
		"Obscura browser binary",
		"node/npm, Obscura",
		"Chrome/Playwright is an external verifier",
		"mount the actual app/component or use the product path",
		"static fixture that hand-creates expected markup is diagnostic only",
		"publish an honest evidence_pending AppChangePackage",
		"worker-local commit is not enough for another worker to inspect",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("worker bootstrap prompt missing %q:\n%s", want, prompt)
		}
	}
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
	processor := rt.ToolRegistryForProfile(AgentProfileProcessor)
	reconciler := rt.ToolRegistryForProfile(AgentProfileReconciler)

	for _, name := range []string{"bash", "read_file", "web_search", "source_search", "spawn_agent", "cast_agent", "cast_agent_update", "wait_agent", "save_evidence", "submit_coagent_update", "publish_app_change_package", "fork_desktop", "publish_desktop", "request_worker_vm", "product_api_request", "start_worker_delegation", "observe_worker_delegation", "redirect_worker_delegation", "finish_worker_delegation", "cancel_worker_delegation", "delegate_worker_vm"} {
		if _, ok := super.Lookup(name); !ok {
			t.Fatalf("super missing tool %q", name)
		}
	}
	for _, name := range []string{"bash", "read_file", "web_search", "source_search", "spawn_agent", "cast_agent", "cast_agent_update", "wait_agent", "save_evidence", "submit_coagent_update", "publish_app_change_package"} {
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
	for _, name := range []string{"bash", "read_file", "web_search", "source_search", "spawn_agent", "cast_agent", "cast_agent_update", "wait_agent", "save_evidence", "submit_coagent_update", "publish_app_change_package"} {
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
	for _, name := range []string{"spawn_agent", "cast_agent", "wait_agent", "cancel_agent"} {
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
	if _, ok := conductor.Lookup("source_search"); ok {
		t.Fatalf("conductor should not have source_search")
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
	if _, ok := researcher.Lookup("publish_app_change_package"); ok {
		t.Fatalf("researcher should not have publish_app_change_package")
	}
	if _, ok := researcher.Lookup("delegate_worker_vm"); ok {
		t.Fatalf("researcher should not have delegate_worker_vm")
	}
	for _, name := range []string{"read_file", "web_search", "source_search", "import_document_content", "list_content_item_selectors", "read_content_item_selector", "cast_agent", "wait_agent", "cancel_agent", "save_evidence", "submit_coagent_update"} {
		if _, ok := researcher.Lookup(name); !ok {
			t.Fatalf("researcher missing tool %q", name)
		}
	}
	if _, ok := researcher.Lookup("spawn_agent"); ok {
		t.Fatalf("researcher should not have spawn_agent")
	}
	for profile, registry := range map[string]*ToolRegistry{
		AgentProfileProcessor:  processor,
		AgentProfileReconciler: reconciler,
	} {
		for _, name := range []string{"read_file", "web_search", "source_search", "import_document_content", "list_content_item_selectors", "read_content_item_selector", "spawn_agent", "cast_agent", "cast_agent_update", "wait_agent", "cancel_agent", "save_evidence", "submit_coagent_update"} {
			if _, ok := registry.Lookup(name); !ok {
				t.Fatalf("%s missing tool %q", profile, name)
			}
		}
		for _, forbidden := range []string{"bash", "write_file", "edit_file", "edit_vtext", "publish_app_change_package", "fork_desktop", "publish_desktop", "request_worker_vm", "delegate_worker_vm"} {
			if _, ok := registry.Lookup(forbidden); ok {
				t.Fatalf("%s should not have %s", profile, forbidden)
			}
		}
		spawnTool, ok := registry.Lookup("spawn_agent")
		if !ok {
			t.Fatalf("%s missing spawn_agent", profile)
		}
		if got := toolSchemaStringEnum(spawnTool.Parameters, "role"); len(got) != 1 || got[0] != AgentProfileVText {
			t.Fatalf("%s spawn_agent role enum = %#v, want only vtext", profile, got)
		}
	}
	for _, name := range []string{"spawn_agent", "cast_agent", "wait_agent", "cancel_agent", "save_evidence", "read_evidence", "edit_vtext", "request_super_execution"} {
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
	if _, ok := vtext.Lookup("source_search"); ok {
		t.Fatalf("vtext should not have source_search")
	}
	if _, ok := vtext.Lookup("submit_coagent_update"); ok {
		t.Fatalf("vtext should not have submit_coagent_update")
	}
	if _, ok := vtext.Lookup("publish_app_change_package"); ok {
		t.Fatalf("vtext should not have publish_app_change_package")
	}
	if _, ok := vtext.Lookup("delegate_worker_vm"); ok {
		t.Fatalf("vtext should not have delegate_worker_vm")
	}
	if _, ok := conductor.Lookup("edit_vtext"); ok {
		t.Fatalf("conductor should not have edit_vtext")
	}
	if _, ok := conductor.Lookup("publish_app_change_package"); ok {
		t.Fatalf("conductor should not have publish_app_change_package")
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

func TestPersistentSuperInboxBashRequiresCoagentUpdate(t *testing.T) {
	ownerID := "owner-super-bash"
	run := &types.RunRecord{
		RunID:        "run-super-bash",
		AgentID:      persistentSuperAgentID(ownerID),
		OwnerID:      ownerID,
		AgentProfile: AgentProfileSuper,
		AgentRole:    AgentProfileSuper,
		Metadata: map[string]any{
			runMetadataAgentProfile: AgentProfileSuper,
			runMetadataAgentRole:    AgentProfileSuper,
			runMetadataAgentID:      persistentSuperAgentID(ownerID),
			"request_source":        "super_inbox",
		},
	}
	raw, err := newBashTool("").Func(WithToolExecutionContext(context.Background(), run), json.RawMessage(`{"command":"printf durable"}`))
	if err != nil {
		t.Fatalf("bash: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("decode bash result: %v", err)
	}
	if _, ok := payload["next_required_tool"]; ok {
		t.Fatalf("next_required_tool should be omitted from bash result; payload=%#v", payload)
	}
	if instruction := fmt.Sprint(payload["next_instruction"]); !strings.Contains(instruction, "Report this command result to the addressed VText document") {
		t.Fatalf("next_instruction = %q", instruction)
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
	if len(deliveries) > 1 {
		t.Fatalf("unexpected deliveries: %+v", deliveries)
	}
	if len(deliveries) == 1 && deliveries[0].Content != "please inspect the runtime tool wiring" {
		t.Fatalf("unexpected delivery content: %+v", deliveries[0])
	}
}

func TestSuperSkipLevelCastRequiresCopiedVSuper(t *testing.T) {
	rt, s, cwd := testRuntimeWithTempCWD(t)
	if err := rt.InstallDefaultAgentTools(cwd); err != nil {
		t.Fatalf("install default agent tools: %v", err)
	}
	ctx := context.Background()
	now := time.Now().UTC()
	superRun := types.RunRecord{
		RunID:        "super-skip-level",
		AgentID:      "agent-super",
		ChannelID:    "skip-level-channel",
		AgentProfile: AgentProfileSuper,
		AgentRole:    AgentProfileSuper,
		OwnerID:      "user-alice",
		SandboxID:    "sandbox-test",
		State:        types.RunRunning,
		Prompt:       "Coordinate without private skip-level directives.",
		CreatedAt:    now,
		UpdatedAt:    now,
		Metadata: map[string]any{
			runMetadataAgentProfile: AgentProfileSuper,
			runMetadataAgentRole:    AgentProfileSuper,
		},
	}
	vsuperRun := types.RunRecord{
		RunID:        "vsuper-skip-level",
		AgentID:      "agent-vsuper-skip",
		ChannelID:    "skip-level-channel",
		AgentProfile: AgentProfileVSuper,
		AgentRole:    AgentProfileVSuper,
		OwnerID:      "user-alice",
		SandboxID:    "sandbox-test",
		State:        types.RunRunning,
		Prompt:       "Supervise co-super.",
		CreatedAt:    now,
		UpdatedAt:    now,
		Metadata: map[string]any{
			runMetadataAgentProfile: AgentProfileVSuper,
			runMetadataAgentRole:    AgentProfileVSuper,
		},
	}
	coRun := types.RunRecord{
		RunID:        "cosuper-skip-level",
		AgentID:      "agent-cosuper-skip",
		ChannelID:    "skip-level-channel",
		ParentRunID:  vsuperRun.RunID,
		AgentProfile: AgentProfileCoSuper,
		AgentRole:    AgentProfileCoSuper,
		OwnerID:      "user-alice",
		SandboxID:    "sandbox-test",
		State:        types.RunRunning,
		Prompt:       "Implement under vsuper.",
		CreatedAt:    now,
		UpdatedAt:    now,
		Metadata: map[string]any{
			runMetadataAgentProfile: AgentProfileCoSuper,
			runMetadataAgentRole:    AgentProfileCoSuper,
		},
	}
	for _, agent := range []types.AgentRecord{
		{AgentID: vsuperRun.AgentID, OwnerID: "user-alice", SandboxID: "sandbox-test", Profile: AgentProfileVSuper, Role: AgentProfileVSuper, ChannelID: "skip-level-channel", CreatedAt: now, UpdatedAt: now},
		{AgentID: coRun.AgentID, OwnerID: "user-alice", SandboxID: "sandbox-test", Profile: AgentProfileCoSuper, Role: AgentProfileCoSuper, ChannelID: "skip-level-channel", CreatedAt: now, UpdatedAt: now},
	} {
		if err := s.UpsertAgent(ctx, agent); err != nil {
			t.Fatalf("upsert agent: %v", err)
		}
	}
	for _, run := range []types.RunRecord{superRun, vsuperRun, coRun} {
		if err := s.CreateRun(ctx, run); err != nil {
			t.Fatalf("create run %s: %v", run.RunID, err)
		}
	}
	registry := rt.ToolRegistryForProfile(AgentProfileSuper)
	toolCtx := WithToolExecutionContext(ctx, &superRun)
	_, err := registry.Execute(toolCtx, "cast_agent", json.RawMessage(`{
		"agent_id":"agent-cosuper-skip",
		"content":"change direction privately"
	}`))
	if err == nil || !strings.Contains(err.Error(), "skip-level directive") {
		t.Fatalf("cast_agent error = %v, want skip-level directive rejection", err)
	}

	raw, err := registry.Execute(toolCtx, "cast_agent_update", json.RawMessage(`{
		"message_class":"directive",
		"content":"adjust the verifier scope with vsuper copied",
		"recipients":[
			{"agent_id":"agent-cosuper-skip","loop_id":"cosuper-skip-level"},
			{"agent_id":"agent-vsuper-skip","loop_id":"vsuper-skip-level"}
		]
	}`))
	if err != nil {
		t.Fatalf("cast_agent_update copied directive: %v", err)
	}
	var resp struct {
		CopyGroupID string   `json:"copy_group_id"`
		Recipients  []string `json:"recipients"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("decode cast_agent_update: %v\n%s", err, raw)
	}
	if resp.CopyGroupID == "" || len(resp.Recipients) != 2 {
		t.Fatalf("copy-aware response incomplete: %+v", resp)
	}
	messages, _, err := rt.ChannelRead("skip-level-channel", 0)
	if err != nil {
		t.Fatalf("read channel: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("messages = %d, want copied directive to both agents: %+v", len(messages), messages)
	}
	for _, msg := range messages {
		if !strings.Contains(msg.Content, "copy_group_id="+resp.CopyGroupID) {
			t.Fatalf("message missing copy group %q: %+v", resp.CopyGroupID, msg)
		}
	}
}

func TestWaitAgentToolReceivesChildResult(t *testing.T) {
	rt, s, cwd := testRuntimeWithTempCWD(t)
	if err := rt.InstallDefaultAgentTools(cwd); err != nil {
		t.Fatalf("install default agent tools: %v", err)
	}

	ctx := context.Background()
	now := time.Now().UTC()
	parent := types.RunRecord{
		RunID:        "vsuper-wait-parent",
		AgentID:      "agent-vsuper-wait",
		ChannelID:    "worker-wait-channel",
		AgentProfile: AgentProfileVSuper,
		AgentRole:    AgentProfileVSuper,
		OwnerID:      "user-alice",
		SandboxID:    "sandbox-test",
		State:        types.RunRunning,
		Prompt:       "Coordinate implementation and verifier children.",
		CreatedAt:    now,
		UpdatedAt:    now,
		Metadata: map[string]any{
			runMetadataAgentProfile: AgentProfileVSuper,
			runMetadataAgentRole:    AgentProfileVSuper,
		},
	}
	if err := s.CreateRun(ctx, parent); err != nil {
		t.Fatalf("create parent run: %v", err)
	}
	child := types.RunRecord{
		RunID:        "co-super-wait-child",
		AgentID:      "agent-impl-wait",
		ChannelID:    parent.ChannelID,
		ParentRunID:  parent.RunID,
		AgentProfile: AgentProfileCoSuper,
		AgentRole:    AgentProfileCoSuper,
		OwnerID:      parent.OwnerID,
		SandboxID:    parent.SandboxID,
		State:        types.RunRunning,
		Prompt:       "Implement candidate and report terminal evidence.",
		CreatedAt:    now,
		UpdatedAt:    now,
		Metadata: map[string]any{
			runMetadataAgentProfile: AgentProfileCoSuper,
			runMetadataAgentRole:    AgentProfileCoSuper,
			runMetadataCoSuperSlot:  "implementation",
		},
	}
	if err := s.UpsertAgent(ctx, types.AgentRecord{
		AgentID:   child.AgentID,
		OwnerID:   child.OwnerID,
		SandboxID: child.SandboxID,
		Profile:   child.AgentProfile,
		Role:      child.AgentRole,
		ChannelID: child.ChannelID,
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create child agent: %v", err)
	}
	if err := s.CreateRun(ctx, child); err != nil {
		t.Fatalf("create child run: %v", err)
	}

	go func() {
		time.Sleep(25 * time.Millisecond)
		_, _ = rt.PostChildResult(WithToolExecutionContext(context.Background(), &child), parent.ChannelID, child.RunID, "implementation committed abc123 and published a reviewable app change package")
	}()

	registry := rt.ToolRegistryForProfile(AgentProfileVSuper)
	raw, err := registry.Execute(WithToolExecutionContext(ctx, &parent), "wait_agent", json.RawMessage(`{
		"agent_id":"agent-impl-wait",
		"channel_id":"worker-wait-channel",
		"roles":["result"],
		"timeout_ms":1000
	}`))
	if err != nil {
		t.Fatalf("wait_agent: %v", err)
	}
	var resp struct {
		Status   string `json:"status"`
		AgentID  string `json:"agent_id"`
		Cursor   uint64 `json:"cursor"`
		Messages []struct {
			FromAgentID string `json:"from_agent_id"`
			FromLoopID  string `json:"from_loop_id"`
			Role        string `json:"role"`
			Content     string `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("decode wait response: %v\n%s", err, raw)
	}
	if resp.Status != "messages" || resp.AgentID != child.AgentID || resp.Cursor == 0 {
		t.Fatalf("unexpected wait response: %+v", resp)
	}
	if len(resp.Messages) != 1 {
		t.Fatalf("messages = %d, want 1: %+v", len(resp.Messages), resp.Messages)
	}
	msg := resp.Messages[0]
	if msg.FromAgentID != child.AgentID || msg.FromLoopID != child.RunID || msg.Role != "result" || !strings.Contains(msg.Content, "published a reviewable app change package") {
		t.Fatalf("unexpected waited message: %+v", msg)
	}
}

func TestWaitAgentToolTimesOutWithRunState(t *testing.T) {
	rt, s, cwd := testRuntimeWithTempCWD(t)
	if err := rt.InstallDefaultAgentTools(cwd); err != nil {
		t.Fatalf("install default agent tools: %v", err)
	}

	ctx := context.Background()
	now := time.Now().UTC()
	parent := types.RunRecord{
		RunID:        "vsuper-timeout-parent",
		AgentID:      "agent-vsuper-timeout",
		ChannelID:    "worker-timeout-channel",
		AgentProfile: AgentProfileVSuper,
		AgentRole:    AgentProfileVSuper,
		OwnerID:      "user-alice",
		SandboxID:    "sandbox-test",
		State:        types.RunRunning,
		Prompt:       "Coordinate implementation and verifier children.",
		CreatedAt:    now,
		UpdatedAt:    now,
		Metadata: map[string]any{
			runMetadataAgentProfile: AgentProfileVSuper,
			runMetadataAgentRole:    AgentProfileVSuper,
		},
	}
	if err := s.CreateRun(ctx, parent); err != nil {
		t.Fatalf("create parent run: %v", err)
	}
	child := types.RunRecord{
		RunID:        "co-super-timeout-child",
		AgentID:      "agent-impl-timeout",
		ChannelID:    parent.ChannelID,
		ParentRunID:  parent.RunID,
		AgentProfile: AgentProfileCoSuper,
		AgentRole:    AgentProfileCoSuper,
		OwnerID:      parent.OwnerID,
		SandboxID:    parent.SandboxID,
		State:        types.RunRunning,
		Prompt:       "Implement candidate and report terminal evidence.",
		CreatedAt:    now,
		UpdatedAt:    now,
		Metadata: map[string]any{
			runMetadataAgentProfile: AgentProfileCoSuper,
			runMetadataAgentRole:    AgentProfileCoSuper,
			runMetadataCoSuperSlot:  "implementation",
		},
	}
	if err := s.UpsertAgent(ctx, types.AgentRecord{
		AgentID:   child.AgentID,
		OwnerID:   child.OwnerID,
		SandboxID: child.SandboxID,
		Profile:   child.AgentProfile,
		Role:      child.AgentRole,
		ChannelID: child.ChannelID,
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create child agent: %v", err)
	}
	if err := s.CreateRun(ctx, child); err != nil {
		t.Fatalf("create child run: %v", err)
	}

	registry := rt.ToolRegistryForProfile(AgentProfileVSuper)
	raw, err := registry.Execute(WithToolExecutionContext(ctx, &parent), "wait_agent", json.RawMessage(`{
		"agent_id":"agent-impl-timeout",
		"channel_id":"worker-timeout-channel",
		"roles":["result"],
		"timeout_ms":20
	}`))
	if err != nil {
		t.Fatalf("wait_agent timeout: %v", err)
	}
	var resp struct {
		Status          string `json:"status"`
		Messages        []any  `json:"messages"`
		LatestTargetRun struct {
			LoopID string         `json:"loop_id"`
			State  types.RunState `json:"state"`
		} `json:"latest_target_run"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("decode wait timeout response: %v\n%s", err, raw)
	}
	if resp.Status != "timeout" || len(resp.Messages) != 0 {
		t.Fatalf("unexpected timeout response: %+v", resp)
	}
	if resp.LatestTargetRun.LoopID != child.RunID || resp.LatestTargetRun.State != types.RunRunning {
		t.Fatalf("latest run summary = %+v, want child running", resp.LatestTargetRun)
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

func TestRedirectWorkerDelegationPostsSuperAuthoredWorkerInbox(t *testing.T) {
	activeRT, _, activeCWD := testRuntimeWithTempCWD(t)
	if err := activeRT.InstallDefaultAgentTools(activeCWD); err != nil {
		t.Fatalf("install active tools: %v", err)
	}
	workerDir := t.TempDir()
	workerDB := filepath.Join(workerDir, "worker.db")
	workerStore, err := store.Open(workerDB)
	if err != nil {
		t.Fatalf("open worker store: %v", err)
	}
	workerRT := New(Config{
		SandboxID:           "sandbox-worker-redirect",
		StorePath:           workerDB,
		ProviderTimeout:     5 * time.Second,
		SupervisionInterval: time.Hour,
	}, workerStore, events.NewEventBus(), NewStubProvider(10*time.Millisecond))
	t.Cleanup(func() {
		workerRT.Stop()
		_ = workerStore.Close()
	})
	workerHandler := NewAPIHandler(workerRT)
	workerMux := http.NewServeMux()
	workerMux.HandleFunc("/internal/runtime/channel-casts", workerHandler.HandleInternalChannelCast)
	workerSrv := httptest.NewServer(workerMux)
	t.Cleanup(workerSrv.Close)

	superRun, err := activeRT.StartRunWithMetadata(context.Background(), "supervise worker", "user-alice", map[string]any{
		runMetadataAgentProfile: AgentProfileSuper,
		runMetadataAgentRole:    AgentProfileSuper,
		runMetadataAgentID:      "super:primary",
		runMetadataChannelID:    "doc-super",
		runMetadataTrajectoryID: "trajectory-super",
	})
	if err != nil {
		t.Fatalf("start super run: %v", err)
	}

	args, err := json.Marshal(map[string]any{
		"worker_sandbox_url": workerSrv.URL,
		"worker_run_id":      "worker-run-redirect",
		"channel_id":         "worker-doc",
		"target_agent_id":    "vsuper:worker",
		"message_class":      "redirection",
		"message":            "Narrow scope to one human-readable VText checkpoint before continuing.",
	})
	if err != nil {
		t.Fatalf("marshal redirect args: %v", err)
	}
	raw, err := activeRT.ToolRegistryForProfile(AgentProfileSuper).Execute(WithToolExecutionContext(context.Background(), superRun), "redirect_worker_delegation", args)
	if err != nil {
		t.Fatalf("redirect_worker_delegation: %v", err)
	}
	var result struct {
		Status        string `json:"status"`
		WorkerRunID   string `json:"worker_run_id"`
		TargetAgentID string `json:"target_agent_id"`
		Cursor        uint64 `json:"cursor"`
	}
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("decode redirect result: %v\n%s", err, raw)
	}
	if result.Status != "worker_redirect_sent" || result.WorkerRunID != "worker-run-redirect" || result.TargetAgentID != "vsuper:worker" || result.Cursor == 0 {
		t.Fatalf("unexpected redirect result: %+v\nraw=%s", result, raw)
	}

	messages, err := workerStore.ListChannelMessages(context.Background(), "user-alice", "worker-doc", 0, 10)
	if err != nil {
		t.Fatalf("list worker channel messages: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("worker channel messages = %d, want 1: %+v", len(messages), messages)
	}
	msg := messages[0]
	if msg.FromAgentID != "super:primary" || msg.FromRunID != superRun.RunID {
		t.Fatalf("message source = (%q,%q), want super source (%q,%q)", msg.FromAgentID, msg.FromRunID, "super:primary", superRun.RunID)
	}
	if msg.ToAgentID != "vsuper:worker" || msg.ToRunID != "worker-run-redirect" {
		t.Fatalf("message target = (%q,%q), want worker vsuper target", msg.ToAgentID, msg.ToRunID)
	}
	if msg.Role != AgentProfileSuper || !strings.Contains(msg.Content, "[message_class=redirection]") {
		t.Fatalf("message content/role not typed super redirect: %+v", msg)
	}

	deliveries, err := workerStore.ListPendingInboxDeliveries(context.Background(), "user-alice", "vsuper:worker", 10)
	if err != nil {
		t.Fatalf("list worker inbox deliveries: %v", err)
	}
	if len(deliveries) != 1 {
		t.Fatalf("pending deliveries = %d, want 1: %+v", len(deliveries), deliveries)
	}
	if deliveries[0].FromAgentID != "super:primary" || deliveries[0].FromRunID != superRun.RunID {
		t.Fatalf("delivery source = (%q,%q), want super source", deliveries[0].FromAgentID, deliveries[0].FromRunID)
	}
}

func TestRequestSuperExecutionDedupesSameVTextRun(t *testing.T) {
	rt, s, cwd := testRuntimeWithTempCWD(t)
	if err := rt.InstallDefaultAgentTools(cwd); err != nil {
		t.Fatalf("install default tools: %v", err)
	}
	vtextRun, err := rt.StartRunWithMetadata(context.Background(), "request privileged work", "user-alice", map[string]any{
		runMetadataAgentProfile: AgentProfileVText,
		runMetadataAgentRole:    AgentProfileVText,
		runMetadataAgentID:      "vtext:doc-super-dedupe",
		runMetadataChannelID:    "doc-super-dedupe",
		runMetadataTrajectoryID: "trace-super-dedupe",
	})
	if err != nil {
		t.Fatalf("start vtext run: %v", err)
	}
	registry := rt.ToolRegistryForProfile(AgentProfileVText)
	rawArgs := json.RawMessage(`{"objective":"Run exactly one bounded candidate-world probe.","channel_id":"doc-super-dedupe","model":"gpt-5-codex"}`)
	firstRaw, err := registry.Execute(WithToolExecutionContext(context.Background(), vtextRun), "request_super_execution", rawArgs)
	if err != nil {
		t.Fatalf("first request_super_execution: %v", err)
	}
	secondRaw, err := registry.Execute(WithToolExecutionContext(context.Background(), vtextRun), "request_super_execution", rawArgs)
	if err != nil {
		t.Fatalf("second request_super_execution: %v", err)
	}
	var first, second struct {
		AgentID string `json:"agent_id"`
		Cursor  int64  `json:"cursor"`
		Deduped bool   `json:"deduped"`
	}
	if err := json.Unmarshal([]byte(firstRaw), &first); err != nil {
		t.Fatalf("decode first response: %v\n%s", err, firstRaw)
	}
	if err := json.Unmarshal([]byte(secondRaw), &second); err != nil {
		t.Fatalf("decode second response: %v\n%s", err, secondRaw)
	}
	if first.AgentID == "" || second.AgentID != first.AgentID {
		t.Fatalf("unexpected super agent ids: first=%+v second=%+v", first, second)
	}
	if first.Deduped || !second.Deduped || second.Cursor != first.Cursor {
		t.Fatalf("unexpected dedupe responses: first=%+v second=%+v", first, second)
	}
	messages, err := s.ListChannelMessages(context.Background(), "user-alice", "doc-super-dedupe", 0, 20)
	if err != nil {
		t.Fatalf("list channel messages: %v", err)
	}
	superMessages := 0
	for _, msg := range messages {
		if msg.ToAgentID == first.AgentID && msg.FromRunID == vtextRun.RunID {
			superMessages++
		}
	}
	if superMessages != 1 {
		t.Fatalf("super channel messages = %d, want one same-run request: %+v", superMessages, messages)
	}
	deliveries, err := s.ListPendingInboxDeliveries(context.Background(), "user-alice", first.AgentID, 20)
	if err != nil {
		t.Fatalf("list pending deliveries: %v", err)
	}
	if len(deliveries) != 0 {
		t.Fatalf("pending super deliveries = %d, want none after run ownership: %+v", len(deliveries), deliveries)
	}
}

func TestRequestSuperExecutionDedupesDifferentObjectivesInSameVTextRun(t *testing.T) {
	rt, s, cwd := testRuntimeWithTempCWD(t)
	if err := rt.InstallDefaultAgentTools(cwd); err != nil {
		t.Fatalf("install default tools: %v", err)
	}
	vtextRun, err := rt.StartRunWithMetadata(context.Background(), "request one privileged turn", "user-alice", map[string]any{
		runMetadataAgentProfile: AgentProfileVText,
		runMetadataAgentRole:    AgentProfileVText,
		runMetadataAgentID:      "vtext:doc-super-turn-dedupe",
		runMetadataChannelID:    "doc-super-turn-dedupe",
		runMetadataTrajectoryID: "trace-super-turn-dedupe",
	})
	if err != nil {
		t.Fatalf("start vtext run: %v", err)
	}
	registry := rt.ToolRegistryForProfile(AgentProfileVText)
	firstRaw, err := registry.Execute(WithToolExecutionContext(context.Background(), vtextRun), "request_super_execution", json.RawMessage(`{
		"objective":"Run one bounded candidate-world probe for onboarding copy.",
		"channel_id":"doc-super-turn-dedupe"
	}`))
	if err != nil {
		t.Fatalf("first request_super_execution: %v", err)
	}
	secondRaw, err := registry.Execute(WithToolExecutionContext(context.Background(), vtextRun), "request_super_execution", json.RawMessage(`{
		"objective":"Also run a second candidate-world probe for prompt bar layout.",
		"channel_id":"doc-super-turn-dedupe"
	}`))
	if err != nil {
		t.Fatalf("second request_super_execution: %v", err)
	}
	var first, second struct {
		AgentID      string `json:"agent_id"`
		Cursor       int64  `json:"cursor"`
		Deduped      bool   `json:"deduped"`
		DedupeReason string `json:"dedupe_reason"`
	}
	if err := json.Unmarshal([]byte(firstRaw), &first); err != nil {
		t.Fatalf("decode first result: %v\n%s", err, firstRaw)
	}
	if err := json.Unmarshal([]byte(secondRaw), &second); err != nil {
		t.Fatalf("decode second result: %v\n%s", err, secondRaw)
	}
	if first.Deduped {
		t.Fatalf("first request should not be deduped: %+v", first)
	}
	if !second.Deduped || second.DedupeReason != "vtext_run_already_requested_super" {
		t.Fatalf("second request should be deduped by vtext turn: %+v", second)
	}
	if second.Cursor != first.Cursor {
		t.Fatalf("second cursor = %d, want first cursor %d", second.Cursor, first.Cursor)
	}
	messages, err := s.ListChannelMessages(context.Background(), "user-alice", "doc-super-turn-dedupe", 0, 20)
	if err != nil {
		t.Fatalf("list channel messages: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("channel messages = %d, want one: %+v", len(messages), messages)
	}
	if !strings.Contains(messages[0].Content, "onboarding copy") {
		t.Fatalf("dedupe should preserve first privileged objective, got %q", messages[0].Content)
	}
	deliveries, err := s.ListPendingInboxDeliveries(context.Background(), "user-alice", first.AgentID, 20)
	if err != nil {
		t.Fatalf("list pending deliveries: %v", err)
	}
	if len(deliveries) > 1 {
		t.Fatalf("pending deliveries = %d, want at most one deduped delivery: %+v", len(deliveries), deliveries)
	}
}

func TestPersistentSuperProcessesConcurrentInboxDeliveriesInFollowupRun(t *testing.T) {
	dir := filepath.Join(os.TempDir(), "go-choir-m3-agent-tools-test")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	dbPath := filepath.Join(dir, t.Name()+".db")
	_ = os.Remove(dbPath)
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	provider := newBlockingExecuteProvider()
	rt := New(Config{
		SandboxID:           "sandbox-test",
		StorePath:           dbPath,
		ProviderTimeout:     5 * time.Second,
		SupervisionInterval: time.Hour,
	}, s, events.NewEventBus(), provider)
	t.Cleanup(func() {
		provider.releaseAll()
		rt.Stop()
		_ = s.Close()
		_ = os.Remove(dbPath)
	})
	if err := rt.InstallDefaultAgentTools(t.TempDir()); err != nil {
		t.Fatalf("install default tools: %v", err)
	}

	registry := rt.ToolRegistryForProfile(AgentProfileVText)
	firstVText, err := rt.StartRunWithMetadata(context.Background(), "request liquid lane", "user-alice", map[string]any{
		runMetadataAgentProfile: AgentProfileVText,
		runMetadataAgentRole:    AgentProfileVText,
		runMetadataAgentID:      "vtext:doc-liquid",
		runMetadataChannelID:    "doc-liquid",
		runMetadataTrajectoryID: "trace-liquid",
	})
	if err != nil {
		t.Fatalf("start first vtext run: %v", err)
	}
	firstRaw, err := registry.Execute(WithToolExecutionContext(context.Background(), firstVText), "request_super_execution", json.RawMessage(`{
		"objective":"Process the liquid package lane.",
		"channel_id":"doc-liquid"
	}`))
	if err != nil {
		t.Fatalf("first request_super_execution: %v", err)
	}
	var firstResp struct {
		AgentID string `json:"agent_id"`
		LoopID  string `json:"loop_id"`
	}
	if err := json.Unmarshal([]byte(firstRaw), &firstResp); err != nil {
		t.Fatalf("decode first response: %v\n%s", err, firstRaw)
	}
	_ = provider.waitForRun(t, "Process the liquid package lane.")
	if pending := pendingDeliveriesForAgent(t, s, "user-alice", firstResp.AgentID); len(pending) != 0 {
		t.Fatalf("initial super delivery should be owned by first run, still pending: %+v", pending)
	}

	secondVText, err := rt.StartRunWithMetadata(context.Background(), "request python lane", "user-alice", map[string]any{
		runMetadataAgentProfile: AgentProfileVText,
		runMetadataAgentRole:    AgentProfileVText,
		runMetadataAgentID:      "vtext:doc-python",
		runMetadataChannelID:    "doc-python",
		runMetadataTrajectoryID: "trace-python",
	})
	if err != nil {
		t.Fatalf("start second vtext run: %v", err)
	}
	secondRaw, err := registry.Execute(WithToolExecutionContext(context.Background(), secondVText), "request_super_execution", json.RawMessage(`{
		"objective":"Process the python package lane.",
		"channel_id":"doc-python"
	}`))
	if err != nil {
		t.Fatalf("second request_super_execution: %v", err)
	}
	var secondResp struct {
		AgentID string `json:"agent_id"`
		LoopID  string `json:"loop_id"`
		State   string `json:"state"`
	}
	if err := json.Unmarshal([]byte(secondRaw), &secondResp); err != nil {
		t.Fatalf("decode second response: %v\n%s", err, secondRaw)
	}
	if secondResp.AgentID != firstResp.AgentID || secondResp.LoopID != firstResp.LoopID {
		t.Fatalf("second request should attach to active persistent super: first=%+v second=%+v", firstResp, secondResp)
	}
	pending := pendingDeliveriesForAgent(t, s, "user-alice", firstResp.AgentID)
	if len(pending) != 1 || !strings.Contains(pending[0].Content, "python package lane") {
		t.Fatalf("python delivery should remain pending for follow-up run, got %+v", pending)
	}

	provider.releaseOne()
	secondSuperRun := provider.waitForRun(t, "Process the python package lane.")
	active, err := s.GetLatestActiveRunByAgent(context.Background(), "user-alice", firstResp.AgentID)
	if err != nil {
		t.Fatalf("lookup follow-up active super run: %v", err)
	}
	if active.RunID == firstResp.LoopID {
		t.Fatalf("python delivery reused first super run %s; want follow-up run", firstResp.LoopID)
	}
	if !strings.Contains(secondSuperRun.Prompt, "Process the python package lane.") || strings.Contains(secondSuperRun.Prompt, "Process the liquid package lane.") {
		t.Fatalf("follow-up prompt did not isolate python delivery:\n%s", secondSuperRun.Prompt)
	}
	if pending := pendingDeliveriesForAgent(t, s, "user-alice", firstResp.AgentID); len(pending) != 0 {
		t.Fatalf("follow-up super delivery should be owned by second run, still pending: %+v", pending)
	}
}

func TestPersistentSuperBlockedRunDoesNotStarveFreshInboxDelivery(t *testing.T) {
	rt, s, cwd := testRuntimeWithTempCWD(t)
	if err := rt.InstallDefaultAgentTools(cwd); err != nil {
		t.Fatalf("install default tools: %v", err)
	}
	superAgent, err := rt.EnsurePersistentSuperAgent(context.Background(), "user-alice")
	if err != nil {
		t.Fatalf("ensure persistent super: %v", err)
	}
	now := time.Now().UTC()
	blocked := types.RunRecord{
		RunID:        "blocked-super-run",
		AgentID:      superAgent.AgentID,
		ChannelID:    superAgent.ChannelID,
		AgentProfile: AgentProfileSuper,
		AgentRole:    AgentProfileSuper,
		OwnerID:      "user-alice",
		SandboxID:    rt.cfg.SandboxID,
		State:        types.RunBlocked,
		Prompt:       "blocked old super loop",
		Error:        "old provider blocker",
		CreatedAt:    now.Add(-time.Hour),
		UpdatedAt:    now,
		Metadata: map[string]any{
			runMetadataAgentProfile: AgentProfileSuper,
			runMetadataAgentRole:    AgentProfileSuper,
			runMetadataAgentID:      superAgent.AgentID,
			"request_source":        "super_inbox",
		},
	}
	if err := s.CreateRun(context.Background(), blocked); err != nil {
		t.Fatalf("create blocked super run: %v", err)
	}

	vtextRun, err := rt.StartRunWithMetadata(context.Background(), "request fresh super work", "user-alice", map[string]any{
		runMetadataAgentProfile: AgentProfileVText,
		runMetadataAgentRole:    AgentProfileVText,
		runMetadataAgentID:      "vtext:doc-fresh-super",
		runMetadataChannelID:    "doc-fresh-super",
		runMetadataTrajectoryID: "trace-fresh-super",
	})
	if err != nil {
		t.Fatalf("start vtext run: %v", err)
	}
	raw, err := rt.ToolRegistryForProfile(AgentProfileVText).Execute(WithToolExecutionContext(context.Background(), vtextRun), "request_super_execution", json.RawMessage(`{
		"objective":"Process the fresh Universal Wire product API handoff.",
		"channel_id":"doc-fresh-super"
	}`))
	if err != nil {
		t.Fatalf("request_super_execution: %v", err)
	}
	var resp struct {
		LoopID string `json:"loop_id"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("decode response: %v\n%s", err, raw)
	}
	if resp.LoopID == "" || resp.LoopID == blocked.RunID {
		t.Fatalf("fresh delivery loop_id = %q, want new run distinct from blocked %q", resp.LoopID, blocked.RunID)
	}
	fresh, err := s.GetRun(context.Background(), resp.LoopID)
	if err != nil {
		t.Fatalf("get fresh super run: %v", err)
	}
	if fresh.AgentID != superAgent.AgentID || !strings.Contains(fresh.Prompt, "fresh Universal Wire product API handoff") {
		t.Fatalf("fresh super run did not own new delivery: %+v", fresh)
	}
	if pending := pendingDeliveriesForAgent(t, s, "user-alice", superAgent.AgentID); len(pending) != 0 {
		t.Fatalf("fresh delivery should be assigned to new run, still pending: %+v", pending)
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
		Status             string         `json:"status"`
		DelegationRequired bool           `json:"delegation_required"`
		NextTool           string         `json:"next_tool"`
		StartArgs          map[string]any `json:"start_args"`
		NextRequiredArgs   map[string]any `json:"next_required_args"`
		Handle             struct {
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
	if !resp.DelegationRequired || resp.NextTool != "start_worker_delegation" {
		t.Fatalf("async delegation guidance missing: %+v", resp)
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
	if resp.StartArgs["worker_sandbox_url"] != resp.Handle.SandboxURL || resp.StartArgs["worker_id"] != resp.Handle.WorkerID || resp.StartArgs["vm_id"] != resp.Handle.VMID {
		t.Fatalf("start_args do not match handle: args=%+v handle=%+v", resp.StartArgs, resp.Handle)
	}
	if timeout, _ := resp.StartArgs["timeout_seconds"].(float64); int(timeout) != int(defaultDelegateWorkerVMTimeout.Seconds()) {
		t.Fatalf("timeout_seconds = %v, want %d", resp.StartArgs["timeout_seconds"], int(defaultDelegateWorkerVMTimeout.Seconds()))
	}
	if resp.NextRequiredArgs["worker_sandbox_url"] != resp.StartArgs["worker_sandbox_url"] ||
		resp.NextRequiredArgs["worker_id"] != resp.StartArgs["worker_id"] ||
		resp.NextRequiredArgs["vm_id"] != resp.StartArgs["vm_id"] {
		t.Fatalf("compat next_required_args should mirror start_args: next=%+v start=%+v", resp.NextRequiredArgs, resp.StartArgs)
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

func TestSuperRequestWorkerVMDedupesSameRunByMachineClass(t *testing.T) {
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
	superTask, err := rt.StartRunWithMetadata(context.Background(), "coordinate one worker lease", "user-alice", map[string]any{
		runMetadataAgentProfile: AgentProfileSuper,
		runMetadataAgentRole:    AgentProfileSuper,
		runMetadataAgentID:      "super:primary",
		runMetadataDesktopID:    types.PrimaryDesktopID,
	})
	if err != nil {
		t.Fatalf("submit super task: %v", err)
	}
	superRegistry := rt.ToolRegistryForProfile(AgentProfileSuper)
	request := func(raw json.RawMessage) (string, bool) {
		t.Helper()
		out, err := superRegistry.Execute(WithToolExecutionContext(context.Background(), superTask), "request_worker_vm", raw)
		if err != nil {
			t.Fatalf("request_worker_vm: %v", err)
		}
		var resp struct {
			Deduped bool `json:"deduped"`
			Handle  struct {
				WorkerID string `json:"worker_id"`
				VMID     string `json:"vm_id"`
			} `json:"handle"`
		}
		if err := json.Unmarshal([]byte(out), &resp); err != nil {
			t.Fatalf("decode request_worker_vm: %v", err)
		}
		return resp.Handle.WorkerID + "/" + resp.Handle.VMID, resp.Deduped
	}

	first, firstDeduped := request(json.RawMessage(`{"purpose":"Run onboarding copy candidate","machine_class":"worker-small"}`))
	second, secondDeduped := request(json.RawMessage(`{"purpose":"Run prompt bar spacing candidate","machine_class":"worker-large"}`))
	if firstDeduped {
		t.Fatal("first worker request should not be deduped")
	}
	if second == first || secondDeduped {
		t.Fatalf("second worker = %s deduped=%v, want distinct worker from %s for different machine class", second, secondDeduped, first)
	}
	third, thirdDeduped := request(json.RawMessage(`{"purpose":"Run another small candidate","machine_class":"worker-small"}`))
	if third != first || !thirdDeduped {
		t.Fatalf("third worker = %s deduped=%v, want reused %s with dedupe marker for same machine class", third, thirdDeduped, first)
	}
}

func TestSuperRequestWorkerVMReplacesUnreachableLeaseAfterDelegateFailure(t *testing.T) {
	rt, s, cwd := testRuntimeWithTempCWD(t)

	workerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("unreachable worker server should be closed before delegation")
	}))
	workerURL := workerSrv.URL
	workerSrv.Close()

	reg := vmctl.NewOwnershipRegistry(workerURL)
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
	superTask, err := rt.StartRunWithMetadata(context.Background(), "coordinate worker recovery", "user-alice", map[string]any{
		runMetadataAgentProfile: AgentProfileSuper,
		runMetadataAgentRole:    AgentProfileSuper,
		runMetadataAgentID:      "super:primary",
		runMetadataDesktopID:    types.PrimaryDesktopID,
		runMetadataTrajectoryID: "traj-worker-recovery",
	})
	if err != nil {
		t.Fatalf("submit super task: %v", err)
	}
	superRegistry := rt.ToolRegistryForProfile(AgentProfileSuper)
	toolCtx := WithToolExecutionContext(context.Background(), superTask)

	requestRaw := json.RawMessage(`{"purpose":"repair mobile UX substrate","machine_class":"worker-small"}`)
	firstRaw, err := superRegistry.Execute(toolCtx, "request_worker_vm", requestRaw)
	if err != nil {
		t.Fatalf("first request_worker_vm: %v", err)
	}
	var first map[string]any
	if err := json.Unmarshal([]byte(firstRaw), &first); err != nil {
		t.Fatalf("decode first request_worker_vm: %v\n%s", err, firstRaw)
	}
	appendRuntimeToolResult(t, s, *superTask, "request_worker_vm", first)
	firstHandle, _ := first["handle"].(map[string]any)
	firstWorkerID := stringMapValue(firstHandle, "worker_id")
	firstVMID := stringMapValue(firstHandle, "vm_id")
	firstSandboxURL := stringMapValue(firstHandle, "sandbox_url")
	if firstWorkerID == "" || firstVMID == "" || firstSandboxURL == "" {
		t.Fatalf("first worker handle incomplete: %s", firstRaw)
	}

	delegateRaw, err := executeWorkerDelegationUntilSettled(t, superRegistry, toolCtx, json.RawMessage(fmt.Sprintf(`{
		"worker_sandbox_url": %q,
		"worker_id": %q,
		"vm_id": %q,
		"objective": "run a candidate repair",
		"profile": "vsuper",
		"timeout_seconds": 1
	}`, firstSandboxURL, firstWorkerID, firstVMID)))
	if err != nil {
		t.Fatalf("delegate_worker_vm should return structured unreachable-worker evidence, got error: %v", err)
	}
	var delegateResult map[string]any
	if err := json.Unmarshal([]byte(delegateRaw), &delegateResult); err != nil {
		t.Fatalf("decode delegate_worker_vm: %v\n%s", err, delegateRaw)
	}
	appendRuntimeToolResult(t, s, *superTask, "delegate_worker_vm", delegateResult)
	if stringMapValue(delegateResult, "status") != "worker_run_submit_failed" {
		t.Fatalf("delegate status = %q, want worker_run_submit_failed\nraw=%s", stringMapValue(delegateResult, "status"), delegateRaw)
	}
	if invalidated, _ := delegateResult["worker_request_cache_invalidated"].(bool); !invalidated {
		t.Fatalf("delegate result did not invalidate stale worker request cache: %s", delegateRaw)
	}

	secondRaw, err := superRegistry.Execute(toolCtx, "request_worker_vm", requestRaw)
	if err != nil {
		t.Fatalf("second request_worker_vm: %v", err)
	}
	var second map[string]any
	if err := json.Unmarshal([]byte(secondRaw), &second); err != nil {
		t.Fatalf("decode second request_worker_vm: %v\n%s", err, secondRaw)
	}
	secondHandle, _ := second["handle"].(map[string]any)
	secondWorkerID := stringMapValue(secondHandle, "worker_id")
	secondVMID := stringMapValue(secondHandle, "vm_id")
	if secondWorkerID == firstWorkerID || secondVMID == firstVMID {
		t.Fatalf("second request reused unreachable worker: first=%s/%s second=%s/%s raw=%s", firstWorkerID, firstVMID, secondWorkerID, secondVMID, secondRaw)
	}
	if replaced, _ := second["replaced_unreachable_worker_request"].(bool); !replaced {
		t.Fatalf("second request did not record fresh lease replacement: %s", secondRaw)
	}
}

func TestSuperDelegateWorkerVMDedupesSameWorkerInRun(t *testing.T) {
	rt, s, cwd := testRuntimeWithTempCWD(t)
	if err := rt.InstallDefaultAgentTools(cwd); err != nil {
		t.Fatalf("install default agent tools: %v", err)
	}
	superTask, err := rt.StartRunWithMetadata(context.Background(), "coordinate one worker delegation", "user-alice", map[string]any{
		runMetadataAgentProfile: AgentProfileSuper,
		runMetadataAgentRole:    AgentProfileSuper,
		runMetadataAgentID:      "super:primary",
		runMetadataDesktopID:    types.PrimaryDesktopID,
		runMetadataTrajectoryID: "traj-delegate-dedupe",
	})
	if err != nil {
		t.Fatalf("submit super task: %v", err)
	}
	existing := map[string]any{
		"status":             "worker_run_completed",
		"worker_id":          "worker-duplicate",
		"worker_vm_id":       "vm-duplicate",
		"worker_sandbox_url": "http://worker-duplicate.test",
		"loop_id":            "worker-run-existing",
		"profile":            AgentProfileVSuper,
		"state":              string(types.RunCompleted),
		"app_change_packages": []map[string]any{{
			"package_id":                   "package-existing",
			"package_manifest_sha256":      "manifest-existing",
			"canonical_package_id":         "package-existing",
			"canonical_mirror_status":      "mirrored",
			"canonical_product_visibility": "published_unlisted",
		}},
	}
	appendRuntimeToolResult(t, s, *superTask, "delegate_worker_vm", existing)

	registry := rt.ToolRegistryForProfile(AgentProfileSuper)
	raw, err := registry.Execute(WithToolExecutionContext(context.Background(), superTask), "delegate_worker_vm", json.RawMessage(`{
		"worker_sandbox_url": "http://worker-duplicate.test",
		"worker_id": "worker-duplicate",
		"vm_id": "vm-duplicate",
		"objective": "run the same candidate-world package work again",
		"profile": "vsuper",
		"timeout_seconds": 1
	}`))
	if err != nil {
		t.Fatalf("delegate_worker_vm dedupe: %v", err)
	}
	var got struct {
		Deduped      bool             `json:"deduped"`
		DedupeReason string           `json:"dedupe_reason"`
		Status       string           `json:"status"`
		LoopID       string           `json:"loop_id"`
		Packages     []map[string]any `json:"app_change_packages"`
	}
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("decode delegate_worker_vm dedupe: %v\n%s", err, raw)
	}
	if !got.Deduped || got.DedupeReason != "super_run_already_started_worker_delegation" {
		t.Fatalf("dedupe fields = %+v, raw=%s", got, raw)
	}
	if got.Status != "worker_run_completed" || got.LoopID != "worker-run-existing" {
		t.Fatalf("deduped status/loop = %+v, raw=%s", got, raw)
	}
	if len(got.Packages) != 1 || stringMapValue(got.Packages[0], "package_id") != "package-existing" {
		t.Fatalf("deduped packages = %+v, raw=%s", got.Packages, raw)
	}
}

func TestSuperDelegateWorkerVMDedupesSameWorkerAcrossTrajectoryRuns(t *testing.T) {
	rt, s, cwd := testRuntimeWithTempCWD(t)
	if err := rt.InstallDefaultAgentTools(cwd); err != nil {
		t.Fatalf("install default agent tools: %v", err)
	}
	ownerID := "user-alice"
	agentID := persistentSuperAgentID(ownerID)
	trajectoryID := "traj-delegate-trajectory-dedupe"
	firstRun, err := rt.StartRunWithMetadata(context.Background(), "start worker in first super turn", ownerID, map[string]any{
		runMetadataAgentProfile: AgentProfileSuper,
		runMetadataAgentRole:    AgentProfileSuper,
		runMetadataAgentID:      agentID,
		runMetadataDesktopID:    types.PrimaryDesktopID,
		runMetadataTrajectoryID: trajectoryID,
	})
	if err != nil {
		t.Fatalf("submit first super task: %v", err)
	}
	existing := map[string]any{
		"status":              "worker_run_started",
		"worker_id":           "worker-trajectory",
		"worker_vm_id":        "vm-trajectory",
		"worker_sandbox_url":  "http://worker-trajectory.test",
		"loop_id":             "worker-run-existing-trajectory",
		"worker_run_id":       "worker-run-existing-trajectory",
		"profile":             AgentProfileVSuper,
		"state":               string(types.RunRunning),
		"app_change_packages": []map[string]any{},
	}
	appendRuntimeToolResult(t, s, *firstRun, "start_worker_delegation", existing)

	secondRun, err := rt.StartRunWithMetadata(context.Background(), "continue worker supervision in second super turn", ownerID, map[string]any{
		runMetadataAgentProfile: AgentProfileSuper,
		runMetadataAgentRole:    AgentProfileSuper,
		runMetadataAgentID:      agentID,
		runMetadataDesktopID:    types.PrimaryDesktopID,
		runMetadataTrajectoryID: trajectoryID,
	})
	if err != nil {
		t.Fatalf("submit second super task: %v", err)
	}
	registry := rt.ToolRegistryForProfile(AgentProfileSuper)
	raw, err := registry.Execute(WithToolExecutionContext(context.Background(), secondRun), "start_worker_delegation", json.RawMessage(`{
		"worker_sandbox_url": "http://worker-trajectory.test",
		"worker_id": "worker-trajectory",
		"vm_id": "vm-trajectory",
		"objective": "continue the same async worker package proof",
		"profile": "vsuper",
		"timeout_seconds": 1
	}`))
	if err != nil {
		t.Fatalf("start_worker_delegation trajectory dedupe: %v", err)
	}
	var got struct {
		Deduped      bool   `json:"deduped"`
		DedupeReason string `json:"dedupe_reason"`
		Status       string `json:"status"`
		LoopID       string `json:"loop_id"`
		WorkerRunID  string `json:"worker_run_id"`
	}
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("decode trajectory dedupe: %v\n%s", err, raw)
	}
	if !got.Deduped || got.DedupeReason != "super_run_already_started_worker_delegation" {
		t.Fatalf("dedupe fields = %+v, raw=%s", got, raw)
	}
	if got.Status != "worker_run_started" || got.LoopID != "worker-run-existing-trajectory" || got.WorkerRunID != "worker-run-existing-trajectory" {
		t.Fatalf("deduped worker run mismatch = %+v, raw=%s", got, raw)
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
		t.Fatal("conductor spawn vtext without channel_id should fail")
	}
	vtextSpawnRaw, err := conductorRegistry.Execute(WithToolExecutionContext(context.Background(), conductorTask), "spawn_agent", json.RawMessage(`{
		"objective":"create v0 and own the document",
		"role":"vtext",
		"channel_id":"doc-work"
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
	if vtextSpawn.UserRevisionID == "" || vtextSpawn.FramingRevisionID != "" || vtextSpawn.InitialRevisionID != vtextSpawn.UserRevisionID {
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
	if parentDecision.UserRevisionID != vtextSpawn.UserRevisionID || parentDecision.FramingRevisionID != "" || parentDecision.InitialRevisionID != vtextSpawn.UserRevisionID {
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

	noisyResearchSpawnRaw, err := vtextRegistry.Execute(WithToolExecutionContext(context.Background(), vtextTask), "spawn_agent", json.RawMessage(`{
		"objective":"research current facts despite provider wrapper noise",
		"role":"researcher</parameter> </invoke>",
		"channel_id":"`+vtextSpawn.ChannelID+`"
	}`))
	if err != nil {
		t.Fatalf("vtext spawn noisy researcher role: %v", err)
	}
	var noisyResearchSpawn struct {
		Role    string `json:"role"`
		Profile string `json:"profile"`
	}
	if err := json.Unmarshal([]byte(noisyResearchSpawnRaw), &noisyResearchSpawn); err != nil {
		t.Fatalf("decode noisy researcher spawn: %v", err)
	}
	if noisyResearchSpawn.Role != AgentProfileResearcher || noisyResearchSpawn.Profile != AgentProfileResearcher {
		t.Fatalf("noisy research spawn = role %q profile %q, want researcher/researcher", noisyResearchSpawn.Role, noisyResearchSpawn.Profile)
	}
}

func TestProcessorAndReconcilerProfilesDelegateToVTextOnly(t *testing.T) {
	rt, s, cwd := testRuntimeWithTempCWD(t)
	if err := rt.InstallDefaultAgentTools(cwd); err != nil {
		t.Fatalf("install default agent tools: %v", err)
	}
	now := time.Now().UTC()
	sourceItem := types.ContentItem{
		ContentID:    "content-fed-rates-brief",
		OwnerID:      "user-alice",
		SourceType:   "url",
		MediaType:    "text/html",
		AppHint:      "source",
		Title:        "Central bank rate bulletin",
		SourceURL:    "https://example.test/rates",
		CanonicalURL: "https://example.test/rates",
		TextContent:  "The central bank held rates steady while inflation remained above target.",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.CreateContentItem(context.Background(), sourceItem); err != nil {
		t.Fatalf("create source content item: %v", err)
	}

	processorRun, err := rt.StartRunWithMetadata(context.Background(), "ingest source batch", "user-alice", map[string]any{
		runMetadataAgentProfile:       "source-processor",
		runMetadataAgentRole:          "source-processor",
		runMetadataProcessorKey:       "processor:global_firehose:global:gdelt",
		"source_network_cycle_id":     "cycle-test",
		"source_network_request_id":   "processor-test",
		"source_network_request_kind": "processor",
		"ingestion_handoff_cycle_id":        "cycle-test",
		"ingestion_handoff_request_id":      "processor-test",
		"ingestion_handoff_request_kind":    "processor",
		"source_item_ids":             []string{sourceItem.ContentID},
	})
	if err != nil {
		t.Fatalf("start processor run: %v", err)
	}
	if processorRun.AgentProfile != AgentProfileProcessor || processorRun.AgentRole != AgentProfileProcessor {
		t.Fatalf("processor run profile/role = %q/%q, want processor/processor", processorRun.AgentProfile, processorRun.AgentRole)
	}
	if processorRun.AgentID != "processor:processor-global_firehose-global-gdelt" {
		t.Fatalf("processor agent id = %q", processorRun.AgentID)
	}
	if processorRun.ChannelID != processorRun.AgentID {
		t.Fatalf("processor channel id = %q, want agent id", processorRun.ChannelID)
	}

	processorRegistry := rt.ToolRegistryForProfile(AgentProfileProcessor)
	if _, err := processorRegistry.Execute(WithToolExecutionContext(context.Background(), processorRun), "spawn_agent", json.RawMessage(`{
		"objective":"verify the strongest claims in this source batch",
		"role":"researcher"
	}`)); err == nil {
		t.Fatal("processor should not be allowed to spawn researcher")
	}

	spawnVTextRaw, err := processorRegistry.Execute(WithToolExecutionContext(context.Background(), processorRun), "spawn_agent", json.RawMessage(`{
		"objective":"write a source-grounded article from this processor brief about Fed rates and inflation",
		"role":"vtext",
		"channel_id":"universal-wire-story-candidate"
	}`))
	if err != nil {
		t.Fatalf("processor spawn vtext: %v", err)
	}
	var vtextSpawn struct {
		AgentID         string         `json:"agent_id"`
		DocID           string         `json:"doc_id"`
		SeedRevisionID  string         `json:"seed_revision_id"`
		LoopID          string         `json:"loop_id"`
		ChannelID       string         `json:"channel_id"`
		Profile         string         `json:"profile"`
		Role            string         `json:"role"`
		State           types.RunState `json:"state"`
		CreatedDocument bool           `json:"created_document"`
	}
	if err := json.Unmarshal([]byte(spawnVTextRaw), &vtextSpawn); err != nil {
		t.Fatalf("decode processor vtext spawn: %v", err)
	}
	if vtextSpawn.Profile != AgentProfileVText || vtextSpawn.Role != AgentProfileVText {
		t.Fatalf("processor vtext spawn profile/role = %+v", vtextSpawn)
	}
	if vtextSpawn.DocID == "" || vtextSpawn.AgentID != "vtext:"+vtextSpawn.DocID || vtextSpawn.ChannelID != vtextSpawn.DocID {
		t.Fatalf("processor vtext spawn did not return normal vtext handle: %+v", vtextSpawn)
	}
	if !vtextSpawn.CreatedDocument || vtextSpawn.SeedRevisionID == "" || vtextSpawn.LoopID == "" {
		t.Fatalf("processor vtext spawn missing document/revision/run identity: %+v", vtextSpawn)
	}
	doc, err := rt.Store().GetDocument(context.Background(), vtextSpawn.DocID, "user-alice")
	if err != nil {
		t.Fatalf("get processor-spawned vtext document: %v", err)
	}
	if doc.CurrentRevisionID != vtextSpawn.SeedRevisionID {
		t.Fatalf("processor-spawned doc head = %q, want seed %q", doc.CurrentRevisionID, vtextSpawn.SeedRevisionID)
	}
	seedRev, err := rt.Store().GetRevision(context.Background(), vtextSpawn.SeedRevisionID, "user-alice")
	if err != nil {
		t.Fatalf("get processor-spawned seed revision: %v", err)
	}
	if seedRev.AuthorKind != types.AuthorAppAgent || !strings.Contains(seedRev.Content, "Source brief:") {
		t.Fatalf("processor seed revision = author %q content %q", seedRev.AuthorKind, seedRev.Content)
	}
	if !strings.Contains(seedRev.Content, "Style.vtext: Market Brief") {
		t.Fatalf("processor seed revision missing selected Style.vtext source: %q", seedRev.Content)
	}
	seedMeta := decodeRevisionMetadata(seedRev.Metadata)
	if metadataString(seedMeta, "artifact_kind") != "source_brief" || metadataString(seedMeta, "vtext_version") == "v0" {
		t.Fatalf("processor seed should be a non-article source brief, metadata=%+v", seedMeta)
	}
	if metadataString(seedMeta, "revision_role") != vtextRevisionRoleInput ||
		metadataString(seedMeta, "input_origin") != vtextInputOriginProcessorHandoff {
		t.Fatalf("processor seed should be input/processor_handoff: %+v", seedMeta)
	}
	if _, ok := seedMeta["article_version"]; ok {
		t.Fatalf("processor seed should not write article_version: %+v", seedMeta)
	}
	sourceEntities := decodeVTextSourceEntities(seedMeta["source_entities"])
	if len(sourceEntities) != 1 || sourceEntities[0].Target.ContentID != sourceItem.ContentID {
		t.Fatalf("processor seed source_entities = %#v, want content item %q", sourceEntities, sourceItem.ContentID)
	}
	if !strings.Contains(seedRev.Content, "(source:"+sourceEntities[0].EntityID+")") {
		t.Fatalf("processor seed should contain native source ref %q in %q", sourceEntities[0].EntityID, seedRev.Content)
	}
	if metadataString(seedMeta, "selected_style_rationale") == "" {
		t.Fatalf("processor seed revision missing style rationale metadata: %+v", seedMeta)
	}
	vtextRun, err := rt.GetRun(context.Background(), vtextSpawn.LoopID, "user-alice")
	if err != nil {
		t.Fatalf("get processor-spawned vtext run: %v", err)
	}
	if vtextRun.AgentID != "vtext:"+vtextSpawn.DocID || vtextRun.ChannelID != vtextSpawn.DocID || metadataString(vtextRun.Metadata, "type") != "vtext_agent_revision" {
		t.Fatalf("processor vtext run is not a vtext revision run: %+v", vtextRun)
	}
	if metadataString(vtextRun.Metadata, "source_network_cycle_id") != "cycle-test" || metadataString(vtextRun.Metadata, "processor_key") != "processor:global_firehose:global:gdelt" {
		t.Fatalf("processor vtext run did not preserve source-network metadata: %+v", vtextRun.Metadata)
	}
	if metadataString(vtextRun.Metadata, "request_intent") != "universal_wire_processor_article_revision" {
		t.Fatalf("processor vtext run request_intent = %q", metadataString(vtextRun.Metadata, "request_intent"))
	}
	runSourceEntities := decodeVTextSourceEntities(vtextRun.Metadata["source_entities"])
	if len(runSourceEntities) != 1 || runSourceEntities[0].Target.ContentID != sourceItem.ContentID {
		t.Fatalf("processor vtext run source_entities = %#v", runSourceEntities)
	}
	if metadataString(vtextRun.Metadata, "selected_style_rationale") == "" || !strings.Contains(vtextRun.Prompt, "Selected Style.vtext source context") || !strings.Contains(vtextRun.Prompt, "Style.vtext: Market Brief") {
		t.Fatalf("processor vtext run missing Style.vtext context: metadata=%+v prompt=%q", vtextRun.Metadata, vtextRun.Prompt)
	}
	if !strings.Contains(vtextRun.Prompt, "must be a publishable article or correction/update draft") ||
		!strings.Contains(vtextRun.Prompt, "not a Source Brief, Working Revision, Evidence Gathering note") ||
		!strings.Contains(vtextRun.Prompt, "(source:"+sourceEntities[0].EntityID+")") ||
		!strings.Contains(vtextRun.Prompt, "cite at least 1 distinct listed native source handle") ||
		!strings.Contains(vtextRun.Prompt, "reader-facing article prose using [label](source:entity_id)") ||
		!strings.Contains(vtextRun.Prompt, "Citations that appear only in Source Handles, Source Manifest, source inventories, notes, or metadata sections do not satisfy this requirement") ||
		!strings.Contains(vtextRun.Prompt, "do not replace them with a plain source manifest") ||
		!strings.Contains(vtextRun.Prompt, "Use the selected Style.vtext sources to shape voice, structure, and editorial judgment") ||
		!strings.Contains(vtextRun.Prompt, "do not name the selected Style.vtext or style rationale in reader-facing prose") ||
		!strings.Contains(vtextRun.Prompt, "Keep Style.vtext selection, source inventories, provenance notes, revision state, and handoff mechanics out of the visible article body") ||
		!strings.Contains(vtextRun.Prompt, "Do not include placeholder metadata or publication labels") ||
		!strings.Contains(vtextRun.Prompt, "Breaking News |") ||
		!strings.Contains(vtextRun.Prompt, "By Choir News") {
		t.Fatalf("processor vtext run missing article-head completion contract: %q", vtextRun.Prompt)
	}
	articleContent := "# Fed rate cut expectations cool as inflation prints remain uneven\n\nMarkets repriced the near-term rate path after the latest inflation batch, but the stronger claim is not that a cut is off the table. The useful reading is narrower: officials have less room to declare victory while price pressure remains uneven across services and shelter measures [source:" + sourceEntities[0].EntityID + "].\n\nThe result is a market-moving macro update with a narrower evidentiary claim: officials can still cut later, but the latest batch gives them less room to declare inflation contained.\n"
	editArgs, err := json.Marshal(map[string]any{
		"doc_id":           vtextSpawn.DocID,
		"base_revision_id": vtextSpawn.SeedRevisionID,
		"operation":        "replace_all",
		"content":          articleContent,
		"rationale":        "Create the first Universal Wire article revision from the processor brief.",
	})
	if err != nil {
		t.Fatalf("marshal vtext edit args: %v", err)
	}
	vtextRegistry := rt.ToolRegistryForProfile(AgentProfileVText)
	if _, err := vtextRegistry.Execute(WithToolExecutionContext(context.Background(), vtextRun), "edit_vtext", editArgs); err != nil {
		t.Fatalf("vtext article edit: %v", err)
	}
	articleDoc, err := rt.Store().GetDocument(context.Background(), vtextSpawn.DocID, "user-alice")
	if err != nil {
		t.Fatalf("get article doc after edit: %v", err)
	}
	if articleDoc.CurrentRevisionID == vtextSpawn.SeedRevisionID {
		t.Fatalf("article edit did not advance document head")
	}
	articleRev, err := rt.Store().GetRevision(context.Background(), articleDoc.CurrentRevisionID, "user-alice")
	if err != nil {
		t.Fatalf("get article revision: %v", err)
	}
	if articleRev.ParentRevisionID != vtextSpawn.SeedRevisionID ||
		!strings.Contains(articleRev.Content, "](source:"+sourceEntities[0].EntityID+")") ||
		strings.Contains(articleRev.Content, "[source:"+sourceEntities[0].EntityID+"]") {
		t.Fatalf("article revision content/lineage invalid: %+v", articleRev)
	}
	articleMeta := decodeRevisionMetadata(articleRev.Metadata)
	if metadataString(articleMeta, "artifact_kind") != "article_revision" ||
		metadataString(articleMeta, "revision_role") != vtextRevisionRoleCanonical ||
		metadataString(articleMeta, "vtext_version_stage") != "article_revision" {
		t.Fatalf("vtext-owned article revision metadata invalid: %#v", articleMeta)
	}
	if len(decodeVTextSourceEntities(articleMeta["source_entities"])) != 1 ||
		metadataString(articleMeta, "source_network_cycle_id") != "cycle-test" ||
		metadataString(articleMeta, "processor_key") != "processor:global_firehose:global:gdelt" ||
		metadataString(articleMeta, "selected_style_rationale") == "" {
		t.Fatalf("vtext-owned article revision lost durable source/style metadata: %#v", articleMeta)
	}
	if normalization, ok := articleMeta["source_ref_normalization"].(map[string]any); !ok || metadataIntValue(normalization, "normalized_bare_source_refs") != 1 {
		t.Fatalf("vtext-owned article revision did not record bare source ref normalization: %#v", articleMeta)
	}
	if _, err := processorRegistry.Execute(WithToolExecutionContext(context.Background(), processorRun), "spawn_agent", json.RawMessage(`{
		"objective":"mutate code",
		"role":"co-super"
	}`)); err == nil {
		t.Fatal("processor should not be allowed to spawn co-super")
	}

	reconcilerRun, err := rt.StartRunWithMetadata(context.Background(), "reconcile story corpus", "user-alice", map[string]any{
		runMetadataAgentProfile:    "corpus-reconciler",
		runMetadataAgentRole:       "corpus-reconciler",
		runMetadataReconcilerScope: "story-corpus",
	})
	if err != nil {
		t.Fatalf("start reconciler run: %v", err)
	}
	if reconcilerRun.AgentProfile != AgentProfileReconciler || reconcilerRun.AgentRole != AgentProfileReconciler {
		t.Fatalf("reconciler run profile/role = %q/%q, want reconciler/reconciler", reconcilerRun.AgentProfile, reconcilerRun.AgentRole)
	}
	if reconcilerRun.AgentID != "reconciler:story-corpus" {
		t.Fatalf("reconciler agent id = %q", reconcilerRun.AgentID)
	}
	reconcilerRegistry := rt.ToolRegistryForProfile(AgentProfileReconciler)
	reconcilerVTextRaw, err := reconcilerRegistry.Execute(WithToolExecutionContext(context.Background(), reconcilerRun), "spawn_agent", json.RawMessage(`{
		"objective":"draft a correction/update VText from this corpus reconciliation",
		"role":"vtext",
		"channel_id":"`+vtextSpawn.DocID+`"
	}`))
	if err != nil {
		t.Fatalf("reconciler spawn vtext: %v", err)
	}
	var reconcilerVTextSpawn struct {
		DocID              string `json:"doc_id"`
		LoopID             string `json:"loop_id"`
		ChannelID          string `json:"channel_id"`
		CreatedDocument    bool   `json:"created_document"`
		RevisedExistingDoc bool   `json:"revised_existing_doc"`
		SeedRevisionID     string `json:"seed_revision_id"`
	}
	if err := json.Unmarshal([]byte(reconcilerVTextRaw), &reconcilerVTextSpawn); err != nil {
		t.Fatalf("decode reconciler vtext spawn: %v", err)
	}
	if reconcilerVTextSpawn.DocID != vtextSpawn.DocID || reconcilerVTextSpawn.ChannelID != vtextSpawn.DocID {
		t.Fatalf("reconciler did not target existing vtext doc: %+v", reconcilerVTextSpawn)
	}
	if reconcilerVTextSpawn.CreatedDocument || !reconcilerVTextSpawn.RevisedExistingDoc || reconcilerVTextSpawn.SeedRevisionID != "" {
		t.Fatalf("reconciler existing-doc revision flags wrong: %+v", reconcilerVTextSpawn)
	}
	reconcilerVTextRun, err := rt.GetRun(context.Background(), reconcilerVTextSpawn.LoopID, "user-alice")
	if err != nil {
		t.Fatalf("get reconciler-spawned vtext run: %v", err)
	}
	if reconcilerVTextRun.AgentID != "vtext:"+vtextSpawn.DocID || reconcilerVTextRun.ChannelID != vtextSpawn.DocID || metadataString(reconcilerVTextRun.Metadata, "type") != "vtext_agent_revision" {
		t.Fatalf("reconciler vtext run is not a vtext revision run: %+v", reconcilerVTextRun)
	}
	if _, err := reconcilerRegistry.Execute(WithToolExecutionContext(context.Background(), reconcilerRun), "spawn_agent", json.RawMessage(`{
		"objective":"verify claims independently",
		"role":"researcher"
	}`)); err == nil {
		t.Fatal("reconciler should not be allowed to spawn researcher")
	}
	if _, err := reconcilerRegistry.Execute(WithToolExecutionContext(context.Background(), reconcilerRun), "spawn_agent", json.RawMessage(`{
		"objective":"draft a corpus-wide new article without a target doc",
		"role":"vtext"
	}`)); err == nil {
		t.Fatal("reconciler corpus_wake should require existing channel_id")
	}
	if _, err := reconcilerRegistry.Execute(WithToolExecutionContext(context.Background(), reconcilerRun), "spawn_agent", json.RawMessage(`{
		"objective":"privileged mutation",
		"role":"super"
	}`)); err == nil {
		t.Fatal("reconciler should not be allowed to spawn super")
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

func TestVSuperSpawnAgentEnforcesActiveChildBudget(t *testing.T) {
	rt, s, cwd := testRuntimeWithTempCWD(t)
	if err := rt.InstallDefaultAgentTools(cwd); err != nil {
		t.Fatalf("install default agent tools: %v", err)
	}

	ctx := context.Background()
	now := time.Now().UTC()
	parent := types.RunRecord{
		RunID:        "vsuper-budget-parent",
		OwnerID:      "user-alice",
		SandboxID:    "sandbox-test",
		State:        types.RunRunning,
		Prompt:       "Coordinate worker and verifier co-super agents.",
		ChannelID:    "worker-channel",
		AgentProfile: AgentProfileVSuper,
		AgentRole:    AgentProfileVSuper,
		Metadata: map[string]any{
			runMetadataAgentProfile: AgentProfileVSuper,
			runMetadataAgentRole:    AgentProfileVSuper,
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.CreateRun(ctx, parent); err != nil {
		t.Fatalf("create vsuper parent: %v", err)
	}

	children := []types.RunRecord{
		{
			RunID:        "vsuper-budget-child-worker",
			AgentID:      "agent-worker-child",
			ChannelID:    parent.ChannelID,
			ParentRunID:  parent.RunID,
			AgentProfile: AgentProfileCoSuper,
			AgentRole:    AgentProfileCoSuper,
			OwnerID:      parent.OwnerID,
			SandboxID:    parent.SandboxID,
			State:        types.RunRunning,
			Prompt:       "Implement candidate change.",
			CreatedAt:    now,
			UpdatedAt:    now,
			Metadata:     map[string]any{runMetadataAgentProfile: AgentProfileCoSuper, runMetadataAgentRole: AgentProfileCoSuper},
		},
		{
			RunID:        "vsuper-budget-child-verifier",
			AgentID:      "agent-verifier-child",
			ChannelID:    parent.ChannelID,
			ParentRunID:  parent.RunID,
			AgentProfile: AgentProfileCoSuper,
			AgentRole:    AgentProfileCoSuper,
			OwnerID:      parent.OwnerID,
			SandboxID:    parent.SandboxID,
			State:        types.RunRunning,
			Prompt:       "Verify candidate change.",
			CreatedAt:    now,
			UpdatedAt:    now,
			Metadata:     map[string]any{runMetadataAgentProfile: AgentProfileCoSuper, runMetadataAgentRole: AgentProfileCoSuper},
		},
	}
	for _, child := range children {
		if err := s.CreateRun(ctx, child); err != nil {
			t.Fatalf("create active child %s: %v", child.RunID, err)
		}
	}

	registry := rt.ToolRegistryForProfile(AgentProfileVSuper)
	_, err := registry.Execute(WithToolExecutionContext(ctx, &parent), "spawn_agent", json.RawMessage(`{
		"objective":"start duplicate verifier",
		"role":"co-super",
		"slot":"verifier"
	}`))
	if err == nil || !strings.Contains(err.Error(), "vsuper active child-run limit reached") {
		t.Fatalf("spawn error = %v, want active child budget refusal", err)
	}

	finishedAt := now.Add(time.Second)
	children[1].State = types.RunCompleted
	children[1].FinishedAt = &finishedAt
	children[1].UpdatedAt = finishedAt
	if err := s.UpdateRun(ctx, children[1]); err != nil {
		t.Fatalf("complete verifier child: %v", err)
	}
	raw, err := registry.Execute(WithToolExecutionContext(ctx, &parent), "spawn_agent", json.RawMessage(`{
		"objective":"start another implementation child after budget frees up",
		"role":"co-super",
		"slot":"implementation"
	}`))
	if err != nil {
		t.Fatalf("spawn replacement implementation co-super: %v", err)
	}
	var resp struct {
		Profile string `json:"profile"`
		Slot    string `json:"slot"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("decode replacement spawn: %v", err)
	}
	if resp.Profile != AgentProfileCoSuper || resp.Slot != "implementation" {
		t.Fatalf("replacement response = %+v, want co-super implementation", resp)
	}
}

func TestVSuperVerifierSpawnRequiresCompletedImplementation(t *testing.T) {
	rt, s, cwd := testRuntimeWithTempCWD(t)
	if err := rt.InstallDefaultAgentTools(cwd); err != nil {
		t.Fatalf("install default agent tools: %v", err)
	}

	ctx := context.Background()
	now := time.Now().UTC()
	parent := types.RunRecord{
		RunID:        "vsuper-sequencing-parent",
		OwnerID:      "user-alice",
		SandboxID:    "sandbox-test",
		State:        types.RunRunning,
		Prompt:       "Coordinate implementation then verification.",
		ChannelID:    "worker-channel",
		AgentProfile: AgentProfileVSuper,
		AgentRole:    AgentProfileVSuper,
		Metadata: map[string]any{
			runMetadataAgentProfile: AgentProfileVSuper,
			runMetadataAgentRole:    AgentProfileVSuper,
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.CreateRun(ctx, parent); err != nil {
		t.Fatalf("create vsuper parent: %v", err)
	}

	registry := rt.ToolRegistryForProfile(AgentProfileVSuper)
	_, err := registry.Execute(WithToolExecutionContext(ctx, &parent), "spawn_agent", json.RawMessage(`{
		"objective":"verify before implementation",
		"role":"co-super",
		"slot":"verifier"
	}`))
	if err == nil || !strings.Contains(err.Error(), "requires prior implementation co-super evidence") {
		t.Fatalf("early verifier error = %v, want prerequisite refusal", err)
	}

	impl := types.RunRecord{
		RunID:        "vsuper-sequencing-impl",
		AgentID:      "agent-implementation-child",
		ChannelID:    parent.ChannelID,
		ParentRunID:  parent.RunID,
		AgentProfile: AgentProfileCoSuper,
		AgentRole:    AgentProfileCoSuper,
		OwnerID:      parent.OwnerID,
		SandboxID:    parent.SandboxID,
		State:        types.RunRunning,
		Prompt:       "Implement candidate change.",
		CreatedAt:    now,
		UpdatedAt:    now,
		Metadata: map[string]any{
			runMetadataAgentProfile: AgentProfileCoSuper,
			runMetadataAgentRole:    AgentProfileCoSuper,
			runMetadataCoSuperSlot:  "implementation",
		},
	}
	if err := s.CreateRun(ctx, impl); err != nil {
		t.Fatalf("create active implementation child: %v", err)
	}

	_, err = registry.Execute(WithToolExecutionContext(ctx, &parent), "spawn_agent", json.RawMessage(`{
		"objective":"verify while implementation is still active",
		"role":"co-super",
		"slot":"verifier"
	}`))
	if err == nil || !strings.Contains(err.Error(), "blocked until implementation co-super") {
		t.Fatalf("active implementation verifier error = %v, want active implementation refusal", err)
	}

	finishedAt := now.Add(time.Second)
	impl.State = types.RunCompleted
	impl.FinishedAt = &finishedAt
	impl.UpdatedAt = finishedAt
	if err := s.UpdateRun(ctx, impl); err != nil {
		t.Fatalf("complete implementation child: %v", err)
	}

	raw, err := registry.Execute(WithToolExecutionContext(ctx, &parent), "spawn_agent", json.RawMessage(`{
		"objective":"verify implementation commit abc123 and saved evidence",
		"role":"co-super",
		"slot":"verifier"
	}`))
	if err != nil {
		t.Fatalf("spawn verifier after implementation: %v", err)
	}
	var resp struct {
		Profile string `json:"profile"`
		Slot    string `json:"slot"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("decode verifier spawn: %v", err)
	}
	if resp.Profile != AgentProfileCoSuper || resp.Slot != "verifier" {
		t.Fatalf("verifier response = %+v, want co-super verifier", resp)
	}
}

func TestVSuperSpawnAgentReusesActiveCoSuperSlot(t *testing.T) {
	rt, s, cwd := testRuntimeWithTempCWD(t)
	if err := rt.InstallDefaultAgentTools(cwd); err != nil {
		t.Fatalf("install default agent tools: %v", err)
	}

	ctx := context.Background()
	now := time.Now().UTC()
	parent := types.RunRecord{
		RunID:        "vsuper-slot-parent",
		OwnerID:      "user-alice",
		SandboxID:    "sandbox-test",
		State:        types.RunRunning,
		Prompt:       "Coordinate worker and verifier co-super agents.",
		ChannelID:    "worker-channel",
		AgentProfile: AgentProfileVSuper,
		AgentRole:    AgentProfileVSuper,
		Metadata: map[string]any{
			runMetadataAgentProfile: AgentProfileVSuper,
			runMetadataAgentRole:    AgentProfileVSuper,
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.CreateRun(ctx, parent); err != nil {
		t.Fatalf("create vsuper parent: %v", err)
	}

	existing := types.RunRecord{
		RunID:        "vsuper-slot-child-worker",
		AgentID:      "agent-worker-child",
		ChannelID:    parent.ChannelID,
		ParentRunID:  parent.RunID,
		AgentProfile: AgentProfileCoSuper,
		AgentRole:    AgentProfileCoSuper,
		OwnerID:      parent.OwnerID,
		SandboxID:    parent.SandboxID,
		State:        types.RunRunning,
		Prompt:       "Implement candidate change.",
		CreatedAt:    now,
		UpdatedAt:    now,
		Metadata: map[string]any{
			runMetadataAgentProfile: AgentProfileCoSuper,
			runMetadataAgentRole:    AgentProfileCoSuper,
			runMetadataCoSuperSlot:  "implementation",
		},
	}
	if err := s.CreateRun(ctx, existing); err != nil {
		t.Fatalf("create active implementation child: %v", err)
	}

	registry := rt.ToolRegistryForProfile(AgentProfileVSuper)
	raw, err := registry.Execute(WithToolExecutionContext(ctx, &parent), "spawn_agent", json.RawMessage(`{
		"objective":"start another implementation co-super for the same candidate checkout",
		"role":"co-super",
		"slot":"implementation"
	}`))
	if err != nil {
		t.Fatalf("reuse implementation co-super: %v", err)
	}
	var resp struct {
		LoopID              string `json:"loop_id"`
		Slot                string `json:"slot"`
		ReusedExistingChild bool   `json:"reused_existing_child"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("decode reused child: %v", err)
	}
	if resp.LoopID != existing.RunID || resp.Slot != "implementation" || !resp.ReusedExistingChild {
		t.Fatalf("reused child response = %+v, want existing loop %q with implementation slot", resp, existing.RunID)
	}
	active, err := s.CountActiveChildRuns(ctx, parent.RunID)
	if err != nil {
		t.Fatalf("count active children: %v", err)
	}
	if active != 1 {
		t.Fatalf("active children = %d, want no duplicate child launch", active)
	}
}

func TestVSuperCancelAgentDoesNotCancelExportedChild(t *testing.T) {
	rt, s, cwd := testRuntimeWithTempCWD(t)
	if err := rt.InstallDefaultAgentTools(cwd); err != nil {
		t.Fatalf("install default agent tools: %v", err)
	}

	ctx := context.Background()
	now := time.Now().UTC()
	parent := types.RunRecord{
		RunID:        "vsuper-cancel-parent",
		OwnerID:      "user-alice",
		SandboxID:    "sandbox-test",
		State:        types.RunRunning,
		Prompt:       "Coordinate worker and verifier co-super agents.",
		ChannelID:    "worker-channel",
		AgentProfile: AgentProfileVSuper,
		AgentRole:    AgentProfileVSuper,
		Metadata: map[string]any{
			runMetadataAgentProfile: AgentProfileVSuper,
			runMetadataAgentRole:    AgentProfileVSuper,
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.CreateRun(ctx, parent); err != nil {
		t.Fatalf("create vsuper parent: %v", err)
	}
	child := types.RunRecord{
		RunID:        "vsuper-cancel-child",
		AgentID:      "agent-exported-child",
		ChannelID:    parent.ChannelID,
		ParentRunID:  parent.RunID,
		AgentProfile: AgentProfileCoSuper,
		AgentRole:    AgentProfileCoSuper,
		OwnerID:      parent.OwnerID,
		SandboxID:    parent.SandboxID,
		State:        types.RunRunning,
		Prompt:       "Implement candidate change and export.",
		CreatedAt:    now,
		UpdatedAt:    now,
		Metadata: map[string]any{
			runMetadataAgentProfile: AgentProfileCoSuper,
			runMetadataAgentRole:    AgentProfileCoSuper,
			runMetadataCoSuperSlot:  "implementation",
		},
	}
	if err := s.CreateRun(ctx, child); err != nil {
		t.Fatalf("create active child: %v", err)
	}
	appendRuntimeToolResult(t, s, child, "publish_app_change_package", map[string]any{
		"status":                      "published_unlisted",
		"package_id":                  "child-package",
		"package_manifest_sha256":     "child-manifest-sha",
		"candidate_head_sha":          "child-head",
		"recipient_build_required":    true,
		"runtime_source_delta_sha256": "child-runtime-sha",
		"ui_source_delta_sha256":      "child-ui-sha",
	})

	registry := rt.ToolRegistryForProfile(AgentProfileVSuper)
	raw, err := registry.Execute(WithToolExecutionContext(ctx, &parent), "cancel_agent", json.RawMessage(`{
		"agent_id":"agent-exported-child"
	}`))
	if err != nil {
		t.Fatalf("cancel exported child: %v", err)
	}
	var resp struct {
		Status string `json:"status"`
		LoopID string `json:"loop_id"`
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("decode cancel response: %v", err)
	}
	if resp.Status != "not_cancelled" || resp.LoopID != child.RunID || !strings.Contains(resp.Reason, "publish_app_change_package") {
		t.Fatalf("cancel response = %+v, want exported child preserved", resp)
	}
	stored, err := s.GetRun(ctx, child.RunID)
	if err != nil {
		t.Fatalf("get child: %v", err)
	}
	if stored.State != types.RunRunning {
		t.Fatalf("child state = %s, want running", stored.State)
	}
}

func TestVSuperPublishAppChangePackageReusesChildPackage(t *testing.T) {
	rt, s, cwd := testRuntimeWithTempCWD(t)
	if err := rt.InstallDefaultAgentTools(cwd); err != nil {
		t.Fatalf("install default agent tools: %v", err)
	}

	ctx := context.Background()
	now := time.Now().UTC()
	parent := types.RunRecord{
		RunID:        "vsuper-export-parent",
		OwnerID:      "user-alice",
		SandboxID:    "sandbox-test",
		State:        types.RunRunning,
		Prompt:       "Coordinate worker and verifier co-super agents.",
		ChannelID:    "worker-channel",
		AgentProfile: AgentProfileVSuper,
		AgentRole:    AgentProfileVSuper,
		Metadata: map[string]any{
			runMetadataAgentProfile: AgentProfileVSuper,
			runMetadataAgentRole:    AgentProfileVSuper,
			runMetadataTrajectoryID: "trace-child-export",
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.CreateRun(ctx, parent); err != nil {
		t.Fatalf("create vsuper parent: %v", err)
	}
	child := types.RunRecord{
		RunID:        "vsuper-export-child",
		AgentID:      "agent-export-child",
		ChannelID:    parent.ChannelID,
		ParentRunID:  parent.RunID,
		AgentProfile: AgentProfileCoSuper,
		AgentRole:    AgentProfileCoSuper,
		OwnerID:      parent.OwnerID,
		SandboxID:    parent.SandboxID,
		State:        types.RunCompleted,
		Prompt:       "Implement candidate change and publish a package.",
		CreatedAt:    now,
		UpdatedAt:    now,
		Metadata: map[string]any{
			runMetadataAgentProfile: AgentProfileCoSuper,
			runMetadataAgentRole:    AgentProfileCoSuper,
			runMetadataCoSuperSlot:  "implementation",
		},
	}
	if err := s.CreateRun(ctx, child); err != nil {
		t.Fatalf("create child: %v", err)
	}
	appendRuntimeToolResult(t, s, child, "publish_app_change_package", map[string]any{
		"status":                  "published_unlisted",
		"package_id":              "child-package",
		"package_manifest_sha256": "child-manifest-sha",
		"candidate_head_sha":      "child-head",
	})

	registry := rt.ToolRegistryForProfile(AgentProfileVSuper)
	raw, err := registry.Execute(WithToolExecutionContext(ctx, &parent), "publish_app_change_package", json.RawMessage(`{
		"repo_path":"does-not-exist",
		"base_sha":"base-sha"
	}`))
	if err != nil {
		t.Fatalf("reuse child export: %v", err)
	}
	var resp struct {
		PackageID          string `json:"package_id"`
		ManifestSHA256     string `json:"package_manifest_sha256"`
		ChildLoopID        string `json:"child_loop_id"`
		ChildSlot          string `json:"child_slot"`
		ReusedChildPackage bool   `json:"reused_child_package"`
		ParentLoopID       string `json:"parent_loop_id"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("decode package response: %v", err)
	}
	if resp.PackageID != "child-package" || resp.ManifestSHA256 != "child-manifest-sha" || resp.ChildLoopID != child.RunID || resp.ChildSlot != "implementation" || !resp.ReusedChildPackage || resp.ParentLoopID != parent.RunID {
		t.Fatalf("package response = %+v, want reused child package", resp)
	}
}

func TestVerifierCoSuperCannotPublishAppChangePackage(t *testing.T) {
	rt, _, cwd := testRuntimeWithTempCWD(t)
	if err := rt.InstallDefaultAgentTools(cwd); err != nil {
		t.Fatalf("install default agent tools: %v", err)
	}
	run := &types.RunRecord{
		RunID:        "verifier-publish-run",
		AgentID:      "agent-verifier",
		AgentProfile: AgentProfileCoSuper,
		AgentRole:    AgentProfileCoSuper,
		OwnerID:      "user-alice",
		SandboxID:    "sandbox-test",
		State:        types.RunRunning,
		Prompt:       "Verify candidate evidence.",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
		Metadata: map[string]any{
			runMetadataAgentProfile: AgentProfileCoSuper,
			runMetadataAgentRole:    AgentProfileCoSuper,
			runMetadataCoSuperSlot:  "verifier",
		},
	}
	registry := rt.ToolRegistryForProfile(AgentProfileCoSuper)
	_, err := registry.Execute(WithToolExecutionContext(context.Background(), run), "publish_app_change_package", json.RawMessage(`{
		"repo_path":".",
		"base_sha":"base"
	}`))
	if err == nil || !strings.Contains(err.Error(), "verifier co-super cannot publish_app_change_package") {
		t.Fatalf("publish_app_change_package error = %v, want verifier authority guard", err)
	}
}

func appendRuntimeToolResult(t *testing.T, s *store.Store, run types.RunRecord, tool string, output map[string]any) {
	t.Helper()
	outputJSON, _ := json.Marshal(output)
	payload, _ := json.Marshal(map[string]any{
		"tool":     tool,
		"is_error": false,
		"output":   string(outputJSON),
	})
	if err := s.AppendEvent(context.Background(), &types.EventRecord{
		EventID:      run.RunID + "-" + tool + "-result",
		RunID:        run.RunID,
		AgentID:      run.AgentID,
		ChannelID:    run.ChannelID,
		OwnerID:      run.OwnerID,
		TrajectoryID: metadataStringValue(run.Metadata, runMetadataTrajectoryID),
		Timestamp:    time.Now().UTC(),
		Kind:         types.EventToolResult,
		Phase:        "tool_call",
		Payload:      payload,
	}); err != nil {
		t.Fatalf("append %s result: %v", tool, err)
	}
}

func TestResearcherSubmitCoagentUpdatePersistsEvidenceAndDedupes(t *testing.T) {
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
	raw, err := researcherRegistry.Execute(WithToolExecutionContext(context.Background(), researcherTask), "submit_coagent_update", json.RawMessage(`{
		"update_id":"finding-001",
		"kind":"findings",
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
		t.Fatalf("submit_coagent_update: %v", err)
	}

	var resp struct {
		UpdateID    string   `json:"update_id"`
		AgentID     string   `json:"agent_id"`
		ChannelID   string   `json:"channel_id"`
		Cursor      int64    `json:"cursor"`
		EvidenceIDs []string `json:"evidence_ids"`
		Status      string   `json:"status"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("decode submit_coagent_update: %v", err)
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

	finding, err := s.GetWorkerUpdate(context.Background(), "user-alice", "finding-001")
	if err != nil {
		t.Fatalf("get coagent update: %v", err)
	}
	if finding.MessageSeq != resp.Cursor {
		t.Fatalf("update message_seq = %d, want %d", finding.MessageSeq, resp.Cursor)
	}
	if finding.Kind != "findings" || finding.Role != AgentProfileResearcher {
		t.Fatalf("unexpected coagent update role/kind: %+v", finding)
	}

	rawAgain, err := researcherRegistry.Execute(WithToolExecutionContext(context.Background(), researcherTask), "submit_coagent_update", json.RawMessage(`{
		"update_id":"finding-001",
		"kind":"findings",
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
		t.Fatalf("repeat submit_coagent_update: %v", err)
	}
	var respAgain struct {
		Cursor int64  `json:"cursor"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal([]byte(rawAgain), &respAgain); err != nil {
		t.Fatalf("decode repeated submit_coagent_update: %v", err)
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

func TestResearcherReadContentItemReturnsPrivateSourceArtifact(t *testing.T) {
	rt, s, cwd := testRuntimeWithTempCWD(t)
	if err := rt.InstallDefaultAgentTools(cwd); err != nil {
		t.Fatalf("install default agent tools: %v", err)
	}

	now := time.Now().UTC()
	item := types.ContentItem{
		ContentID:    "content-transcript-1",
		OwnerID:      "user-alice",
		SourceType:   "derived_transcript",
		MediaType:    "text/x-youtube-transcript",
		AppHint:      "vtext",
		Title:        "Transcript",
		SourceURL:    "https://www.youtube.com/watch?v=abc12345678",
		CanonicalURL: "youtube://abc12345678/transcript/en",
		TextContent:  "first segment text\nsecond segment text",
		Metadata:     json.RawMessage(`{"availability":"available","provider":"unit","segments":[{"start":0,"duration":1.2,"text":"first segment text"},{"start":1.2,"duration":1.1,"text":"second segment text"}]}`),
		Provenance:   json.RawMessage(`{"rights_scope":"private_user_source","untrusted_source_text":true}`),
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.CreateContentItem(context.Background(), item); err != nil {
		t.Fatalf("create content item: %v", err)
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
	researcherTask, err := rt.StartChildRun(context.Background(), vtextTask.RunID, "read source packet", "user-alice", map[string]any{
		runMetadataAgentProfile: AgentProfileResearcher,
		runMetadataAgentRole:    AgentProfileResearcher,
		runMetadataChannelID:    "doc-1",
	})
	if err != nil {
		t.Fatalf("submit researcher task: %v", err)
	}

	researcherRegistry := rt.ToolRegistryForProfile(AgentProfileResearcher)
	raw, err := researcherRegistry.Execute(WithToolExecutionContext(context.Background(), researcherTask), "read_content_item", json.RawMessage(`{
		"content_id":"content-transcript-1",
		"max_text_chars":12,
		"max_segments":1
	}`))
	if err != nil {
		t.Fatalf("read_content_item: %v", err)
	}
	var resp map[string]any
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("decode read_content_item: %v", err)
	}
	if got := resp["text_content"]; got != "first segmen" {
		t.Fatalf("text_content = %#v, want truncated transcript prefix", got)
	}
	if got := resp["text_truncated"]; got != true {
		t.Fatalf("text_truncated = %#v, want true", got)
	}
	if got := int(resp["segment_count"].(float64)); got != 2 {
		t.Fatalf("segment_count = %d, want 2", got)
	}
	if segments, _ := resp["segments"].([]any); len(segments) != 1 {
		t.Fatalf("segments len = %d, want 1; resp=%#v", len(segments), resp)
	}
	provenance, _ := resp["provenance"].(map[string]any)
	if provenance["rights_scope"] != "private_user_source" || provenance["untrusted_source_text"] != true {
		t.Fatalf("provenance = %#v", provenance)
	}
	if got := resp["next_required_tool"]; got != "submit_coagent_update" {
		t.Fatalf("next_required_tool = %#v, want submit_coagent_update", got)
	}
}

func TestResearcherDocumentSelectorToolsReadPPTXSourceArtifact(t *testing.T) {
	rt, _, cwd := testRuntimeWithTempCWD(t)
	if err := rt.InstallDefaultAgentTools(cwd); err != nil {
		t.Fatalf("install default agent tools: %v", err)
	}
	filesRoot := t.TempDir()
	t.Setenv("SANDBOX_FILES_ROOT", filesRoot)
	if err := os.MkdirAll(filepath.Join(filesRoot, "imports"), 0o755); err != nil {
		t.Fatalf("create imports dir: %v", err)
	}
	pptxBytes := buildMinimalPPTX(t, []string{
		"Mission gradient\nFrozen source corpus",
		"Recall check\nExact slide detail",
	})
	if err := os.WriteFile(filepath.Join(filesRoot, "imports", "deck.pptx"), pptxBytes, 0o644); err != nil {
		t.Fatalf("write pptx: %v", err)
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
	researcherTask, err := rt.StartChildRun(context.Background(), vtextTask.RunID, "read deck source", "user-alice", map[string]any{
		runMetadataAgentProfile: AgentProfileResearcher,
		runMetadataAgentRole:    AgentProfileResearcher,
		runMetadataChannelID:    "doc-1",
	})
	if err != nil {
		t.Fatalf("submit researcher task: %v", err)
	}

	researcherRegistry := rt.ToolRegistryForProfile(AgentProfileResearcher)
	rawImport, err := researcherRegistry.Execute(WithToolExecutionContext(context.Background(), researcherTask), "import_document_content", json.RawMessage(`{
		"file_path":"imports/deck.pptx"
	}`))
	if err != nil {
		t.Fatalf("import_document_content: %v", err)
	}
	var imported map[string]any
	if err := json.Unmarshal([]byte(rawImport), &imported); err != nil {
		t.Fatalf("decode import_document_content: %v", err)
	}
	contentID, _ := imported["content_id"].(string)
	if contentID == "" || imported["app_hint"] != "slides" {
		t.Fatalf("imported = %#v", imported)
	}
	expectedHash := contentHashBytes(pptxBytes)
	if imported["content_hash"] != expectedHash {
		t.Fatalf("content_hash = %#v, want raw artifact hash %s", imported["content_hash"], expectedHash)
	}
	if imported["selector_count"].(float64) != 2 {
		t.Fatalf("selector_count = %#v", imported["selector_count"])
	}

	rawList, err := researcherRegistry.Execute(WithToolExecutionContext(context.Background(), researcherTask), "list_content_item_selectors", json.RawMessage(`{
		"content_id":"`+contentID+`"
	}`))
	if err != nil {
		t.Fatalf("list_content_item_selectors: %v", err)
	}
	if !strings.Contains(rawList, "slide-2") || !strings.Contains(rawList, "Exact slide detail") {
		t.Fatalf("selector list = %s", rawList)
	}

	rawRead, err := researcherRegistry.Execute(WithToolExecutionContext(context.Background(), researcherTask), "read_content_item_selector", json.RawMessage(`{
		"content_id":"`+contentID+`",
		"selector_id":"slide-2"
	}`))
	if err != nil {
		t.Fatalf("read_content_item_selector: %v", err)
	}
	if !strings.Contains(rawRead, "Recall check") || !strings.Contains(rawRead, "Exact slide detail") {
		t.Fatalf("selector read = %s", rawRead)
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
		"capability_requests":[{
			"capability":"research",
			"requested_role":"researcher",
			"objective":"Ground whether the chosen mutation model has precedent in published toy-model literature.",
			"why_needed":"Super should not invent source context while reporting execution evidence.",
			"blocking":true,
			"evidence_needed_for":"Source Ledger [S2]",
			"suggested_next_owner":"vtext"
		}],
		"notes":["This is a structured worker update, not a document patch."]
	}`)
	raw, err := superRegistry.Execute(WithToolExecutionContext(ctx, superRun), "submit_coagent_update", rawArgs)
	if err != nil {
		t.Fatalf("submit_coagent_update: %v", err)
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
		t.Fatalf("decode submit_coagent_update response: %v", err)
	}
	if resp.UpdateID != "super-update-1" || resp.AgentID != "vtext:"+docID || resp.ChannelID != docID || resp.Cursor == 0 || resp.Status != "submitted" {
		t.Fatalf("unexpected submit_coagent_update response: %+v", resp)
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
	if len(update.CapabilityRequests) != 1 || update.CapabilityRequests[0].Capability != "research" || !update.CapabilityRequests[0].Blocking {
		t.Fatalf("capability_requests = %+v", update.CapabilityRequests)
	}
	if !strings.Contains(update.Content, "Artifacts:") || !strings.Contains(update.Content, "Tests:") || !strings.Contains(update.Content, "Proposals:") || !strings.Contains(update.Content, "Capability requests:") {
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
	if !strings.Contains(messages[0].Content, "Coagent update ready.") || strings.Contains(strings.ToLower(messages[0].Content), "apply this patch") {
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

	rawAgain, err := superRegistry.Execute(WithToolExecutionContext(ctx, superRun), "submit_coagent_update", rawArgs)
	if err != nil {
		t.Fatalf("repeat submit_coagent_update: %v", err)
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

	_, err = superRegistry.Execute(WithToolExecutionContext(ctx, superRun), "submit_coagent_update", json.RawMessage(`{
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
	raw, err := superRegistry.Execute(WithToolExecutionContext(ctx, superRun), "submit_coagent_update", json.RawMessage(`{
		"update_id":"super-authoritative-channel",
		"agent_id":"vtext:doc-authoritative-channel",
		"channel_id":"not-the-vtext-doc-channel",
		"artifacts":["artifacts/authoritative.txt"],
		"tests":["verified authoritative channel routing"]
	}`))
	if err != nil {
		t.Fatalf("submit_coagent_update: %v", err)
	}
	var resp struct {
		ChannelID string `json:"channel_id"`
		Cursor    int64  `json:"cursor"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("decode submit_coagent_update response: %v", err)
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
	raw, err := superRegistry.Execute(WithToolExecutionContext(ctx, superRun), "submit_coagent_update", json.RawMessage(`{
		"update_id":"super-authoritative-parent",
		"agent_id":"researcher:decoy",
		"artifacts":["artifacts/parent.txt"],
		"tests":["verified parent target routing"]
	}`))
	if err != nil {
		t.Fatalf("submit_coagent_update: %v", err)
	}
	var resp struct {
		AgentID   string `json:"agent_id"`
		ChannelID string `json:"channel_id"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("decode submit_coagent_update response: %v", err)
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
	raw, err := superRegistry.Execute(WithToolExecutionContext(ctx, superRun), "submit_coagent_update", json.RawMessage(`{
		"update_id":"super-requester-target",
		"agent_id":"researcher:decoy-requester",
		"artifacts":["artifacts/requester.txt"],
		"tests":["verified requester target routing"]
	}`))
	if err != nil {
		t.Fatalf("submit_coagent_update: %v", err)
	}
	var resp struct {
		AgentID   string `json:"agent_id"`
		ChannelID string `json:"channel_id"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("decode submit_coagent_update response: %v", err)
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

func TestSubmitWorkerUpdateUsesVTextRequesterMetadataWhenAgentMissing(t *testing.T) {
	rt, s, cwd := testRuntimeWithTempCWD(t)
	if err := rt.InstallDefaultAgentTools(cwd); err != nil {
		t.Fatalf("install default agent tools: %v", err)
	}

	ctx := context.Background()
	ownerID := "user-alice"
	docID := "doc-remote-worker-requester"
	workerRun, err := rt.StartRunWithMetadata(ctx, "Report from isolated worker", ownerID, map[string]any{
		runMetadataAgentProfile: AgentProfileVSuper,
		runMetadataAgentRole:    AgentProfileVSuper,
		runMetadataAgentID:      "vsuper:remote-worker",
		runMetadataChannelID:    docID,
		runMetadataTrajectoryID: "traj-remote-worker-requester",
		"requested_by_agent_id": "vtext:" + docID,
		"requested_by_profile":  AgentProfileVText,
		"request_source":        "worker_vm_delegation",
	})
	if err != nil {
		t.Fatalf("start worker run: %v", err)
	}

	vSuperRegistry := rt.ToolRegistryForProfile(AgentProfileVSuper)
	raw, err := vSuperRegistry.Execute(WithToolExecutionContext(ctx, workerRun), "submit_coagent_update", json.RawMessage(`{
		"update_id":"remote-worker-update",
		"findings":["Remote worker update should route through inherited VText metadata."],
		"tests":["metadata-only requester routing passed"]
	}`))
	if err != nil {
		t.Fatalf("submit_coagent_update should not require local requester agent row: %v", err)
	}
	var resp struct {
		AgentID   string `json:"agent_id"`
		ChannelID string `json:"channel_id"`
		Status    string `json:"status"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("decode submit_coagent_update response: %v", err)
	}
	if resp.AgentID != "vtext:"+docID || resp.ChannelID != docID || resp.Status != "submitted" {
		t.Fatalf("response target/channel/status = %+v, want inherited vtext target", resp)
	}

	update, err := s.GetWorkerUpdate(ctx, ownerID, "remote-worker-update")
	if err != nil {
		t.Fatalf("get worker update: %v", err)
	}
	if update.TargetAgentID != "vtext:"+docID || update.ChannelID != docID || update.Role != AgentProfileVSuper {
		t.Fatalf("worker update target/channel/role = %+v", update)
	}
	deliveries, err := s.ListPendingInboxDeliveries(ctx, ownerID, "vtext:"+docID, 10)
	if err != nil {
		t.Fatalf("list vtext deliveries: %v", err)
	}
	if len(deliveries) != 1 || deliveries[0].ChannelID != docID {
		t.Fatalf("expected metadata-routed vtext delivery, got %+v", deliveries)
	}
}

func TestSubmitWorkerUpdateFallsBackToVTextChannelWhenExplicitTargetMissing(t *testing.T) {
	rt, s, cwd := testRuntimeWithTempCWD(t)
	if err := rt.InstallDefaultAgentTools(cwd); err != nil {
		t.Fatalf("install default agent tools: %v", err)
	}

	ctx := context.Background()
	ownerID := "user-alice"
	docID := "doc-terminal-blocker"
	superRun, err := rt.StartRunWithMetadata(ctx, "Report terminal worker blocker", ownerID, map[string]any{
		runMetadataAgentProfile: AgentProfileSuper,
		runMetadataAgentRole:    AgentProfileSuper,
		runMetadataAgentID:      "super:terminal-blocker",
		runMetadataChannelID:    docID,
		runMetadataTrajectoryID: "traj-terminal-blocker",
	})
	if err != nil {
		t.Fatalf("start super run: %v", err)
	}

	superRegistry := rt.ToolRegistryForProfile(AgentProfileSuper)
	for _, tc := range []struct {
		name            string
		explicitAgentID string
		updateID        string
	}{
		{name: "bare doc id", explicitAgentID: docID, updateID: "terminal-blocker-bare-doc"},
		{name: "missing vtext agent", explicitAgentID: "vtext:" + docID, updateID: "terminal-blocker-vtext-agent"},
		{name: "stale owner id", explicitAgentID: ownerID, updateID: "terminal-blocker-owner-id"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rawArgs := json.RawMessage(fmt.Sprintf(`{
				"update_id":%q,
				"agent_id":%q,
				"kind":"blocker",
				"summary":"Worker canceled after terminal evidence failed to publish.",
				"findings":["submit_coagent_update should route to the VText channel when the explicit target is stale."]
			}`, tc.updateID, tc.explicitAgentID))
			raw, err := superRegistry.Execute(WithToolExecutionContext(ctx, superRun), "submit_coagent_update", rawArgs)
			if err != nil {
				t.Fatalf("submit_coagent_update: %v", err)
			}
			var resp struct {
				AgentID   string `json:"agent_id"`
				ChannelID string `json:"channel_id"`
				Status    string `json:"status"`
			}
			if err := json.Unmarshal([]byte(raw), &resp); err != nil {
				t.Fatalf("decode submit_coagent_update response: %v", err)
			}
			if resp.AgentID != "vtext:"+docID || resp.ChannelID != docID || resp.Status != "submitted" {
				t.Fatalf("response target/channel/status = %+v, want vtext fallback", resp)
			}
			update, err := s.GetWorkerUpdate(ctx, ownerID, tc.updateID)
			if err != nil {
				t.Fatalf("get worker update: %v", err)
			}
			if update.TargetAgentID != "vtext:"+docID || update.ChannelID != docID {
				t.Fatalf("worker update target/channel = %+v, want vtext fallback", update)
			}
		})
	}

	messages, err := s.ListChannelMessages(ctx, ownerID, docID, 0, 10)
	if err != nil {
		t.Fatalf("list channel messages: %v", err)
	}
	if len(messages) != 3 {
		t.Fatalf("messages len = %d, want 3: %+v", len(messages), messages)
	}
	for _, message := range messages {
		if message.ToAgentID != "vtext:"+docID || message.ChannelID != docID {
			t.Fatalf("message did not route through vtext fallback: %+v", message)
		}
	}
}

func TestSuperFailureAfterDelegateSynthesizesWorkerUpdate(t *testing.T) {
	ctx := context.Background()
	rt, s, _ := testRuntimeWithTempCWD(t)
	ownerID := "user-delegate-fallback"
	docID := "doc-delegate-fallback"

	now := time.Now().UTC()
	superRun := &types.RunRecord{
		RunID:        "super-run-delegate-fallback",
		AgentID:      "super:" + ownerID,
		ChannelID:    docID,
		AgentProfile: AgentProfileSuper,
		AgentRole:    AgentProfileSuper,
		OwnerID:      ownerID,
		SandboxID:    "sandbox-test",
		State:        types.RunRunning,
		Prompt:       "Delegate worker then report",
		CreatedAt:    now,
		UpdatedAt:    now,
		Metadata: map[string]any{
			runMetadataAgentProfile: AgentProfileSuper,
			runMetadataAgentRole:    AgentProfileSuper,
			runMetadataAgentID:      "super:" + ownerID,
			runMetadataChannelID:    docID,
			runMetadataTrajectoryID: "traj-delegate-fallback",
			"requested_by_agent_id": "vtext:" + docID,
			"requested_by_profile":  AgentProfileVText,
		},
	}
	if err := s.CreateRun(ctx, *superRun); err != nil {
		t.Fatalf("create super run: %v", err)
	}
	if err := s.UpsertAgent(ctx, types.AgentRecord{
		AgentID:   "super:" + ownerID,
		OwnerID:   ownerID,
		SandboxID: "sandbox-test",
		Profile:   AgentProfileSuper,
		Role:      AgentProfileSuper,
		ChannelID: docID,
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("upsert super agent: %v", err)
	}

	delegateOutput := map[string]any{
		"status":                       "worker_run_timeout",
		"state":                        string(types.RunRunning),
		"loop_id":                      "worker-run-timeout",
		"worker_id":                    "worker-fallback",
		"worker_vm_id":                 "vm-fallback",
		"terminal_error":               "worker run worker-run-timeout did not finish within 15m0s",
		"event_count":                  12,
		"worker_channel_message_count": 2,
		"app_change_packages":          []map[string]any{},
	}
	outputJSON, _ := json.Marshal(delegateOutput)
	payload, _ := json.Marshal(map[string]any{
		"tool":     "delegate_worker_vm",
		"is_error": false,
		"output":   string(outputJSON),
	})
	if err := s.AppendEvent(ctx, &types.EventRecord{
		EventID:      "event-delegate-fallback",
		RunID:        superRun.RunID,
		AgentID:      agentIDForRun(superRun),
		ChannelID:    docID,
		OwnerID:      ownerID,
		TrajectoryID: "traj-delegate-fallback",
		Timestamp:    time.Now().UTC(),
		Kind:         types.EventToolResult,
		Payload:      payload,
	}); err != nil {
		t.Fatalf("append delegate event: %v", err)
	}

	rt.handleExecutionError(ctx, superRun, fmt.Errorf("gateway call failed: chatgpt: status 429 Too Many Requests (sanitized)"))

	storedRun, err := s.GetRun(ctx, superRun.RunID)
	if err != nil {
		t.Fatalf("get super run: %v", err)
	}
	if storedRun.State != types.RunBlocked {
		t.Fatalf("super state = %q, want blocked", storedRun.State)
	}

	updates, err := s.ListWorkerUpdatesByTrajectory(ctx, ownerID, "traj-delegate-fallback", 10)
	if err != nil {
		t.Fatalf("list worker updates: %v", err)
	}
	if len(updates) != 1 {
		t.Fatalf("worker updates = %d, want 1", len(updates))
	}
	update := updates[0]
	if update.UpdateID != "delegate-worker-vm-"+sanitizeExportPart(superRun.RunID) {
		t.Fatalf("update id = %q", update.UpdateID)
	}
	if update.TargetAgentID != "vtext:"+docID || update.ChannelID != docID {
		t.Fatalf("update target/channel = %q/%q", update.TargetAgentID, update.ChannelID)
	}
	if !containsString(update.EvidenceIDs, "event:event-delegate-fallback") || !containsString(update.EvidenceIDs, "worker_loop:worker-run-timeout") {
		t.Fatalf("evidence ids missing delegate refs: %+v", update.EvidenceIDs)
	}
	if !strings.Contains(strings.Join(update.Findings, "\n"), "no AppChangePackages") {
		t.Fatalf("findings missing export blocker: %+v", update.Findings)
	}
	if !strings.Contains(update.Content, "worker delegation returned") {
		t.Fatalf("worker update content missing delegate summary: %q", update.Content)
	}

	messages, err := s.ListChannelMessages(ctx, ownerID, docID, 0, 10)
	if err != nil {
		t.Fatalf("list channel messages: %v", err)
	}
	if len(messages) != 1 || messages[0].ToAgentID != "vtext:"+docID {
		t.Fatalf("channel messages = %+v", messages)
	}
	if !strings.Contains(messages[0].Content, "worker delegation returned") {
		t.Fatalf("message content missing delegate summary: %q", messages[0].Content)
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

func TestPublishAppChangePackageToolPublishesWithoutGitHubPush(t *testing.T) {
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

	superRun, err := rt.StartRunWithMetadata(context.Background(), "publish worker AppChangePackage", "user-alice", map[string]any{
		runMetadataAgentProfile: AgentProfileCoSuper,
		runMetadataAgentRole:    AgentProfileCoSuper,
		runMetadataTrajectoryID: "trace-export-tool",
	})
	if err != nil {
		t.Fatalf("start co-super run: %v", err)
	}
	registry := rt.ToolRegistryForProfile(AgentProfileCoSuper)
	raw, err := registry.Execute(WithToolExecutionContext(context.Background(), superRun), "publish_app_change_package", json.RawMessage(fmt.Sprintf(`{
			"repo_path": "repo",
			"base_sha": %q,
			"candidate_source_ref": "refs/heads/candidate/package-proof",
			"snapshot_id": "snapshot-tool",
			"summary": "worker package proof",
			"human_summary": "Owner-readable narrative for the worker package proof.",
			"recommendation": "review with the attached screenshot before install",
			"vtext_doc_id": "doc-tool-proof",
			"vtext_revision_id": "rev-tool-proof",
			"screenshot_refs": ["test-results/tool-proof.png"],
			"behavior_contract": "screenshot shows the changed README proof path",
			"checks": ["grep -q worker README.md"]
		}`, base)))
	if err != nil {
		t.Fatalf("publish_app_change_package: %v", err)
	}

	var result struct {
		Status                   string `json:"status"`
		PackageID                string `json:"package_id"`
		PackageManifestSHA256    string `json:"package_manifest_sha256"`
		RuntimeSourceDeltaSHA256 string `json:"runtime_source_delta_sha256"`
		CandidateHeadSHA         string `json:"candidate_head_sha"`
		CandidateSourceRef       string `json:"candidate_source_ref"`
		HumanProofState          string `json:"human_proof_state"`
		GitHubPush               bool   `json:"github_push"`
	}
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("decode package result: %v\n%s", err, raw)
	}
	if result.Status != "published_unlisted" || result.GitHubPush || result.PackageID == "" || result.PackageManifestSHA256 == "" || result.RuntimeSourceDeltaSHA256 == "" || result.CandidateHeadSHA == "" {
		t.Fatalf("unexpected package result: %+v", result)
	}
	if strings.Contains(result.CandidateSourceRef, "refs/heads/candidate/") || !strings.Contains(result.CandidateSourceRef, "/candidates/") {
		t.Fatalf("candidate_source_ref = %q, want canonical product candidate ref", result.CandidateSourceRef)
	}
	if result.HumanProofState != "human_reviewable" {
		t.Fatalf("human_proof_state = %q, want human_reviewable", result.HumanProofState)
	}
	pkg, err := rt.store.GetAppChangePackage(context.Background(), result.PackageID)
	if err != nil {
		t.Fatalf("get app change package: %v", err)
	}
	var manifest map[string]any
	if err := json.Unmarshal(pkg.ManifestJSON, &manifest); err != nil {
		t.Fatalf("decode package manifest: %v", err)
	}
	if manifest["source_ledger_candidate_ref"] != "refs/heads/candidate/package-proof" {
		t.Fatalf("source_ledger_candidate_ref = %q, want worker git branch", manifest["source_ledger_candidate_ref"])
	}
	var provenance map[string]any
	if err := json.Unmarshal(pkg.ProvenanceRefsJSON, &provenance); err != nil {
		t.Fatalf("decode package provenance refs: %v", err)
	}
	if provenance["vtext_doc_id"] != "doc-tool-proof" {
		t.Fatalf("vtext_doc_id = %q, want doc-tool-proof", provenance["vtext_doc_id"])
	}
	shots, _ := provenance["screenshot_refs"].([]any)
	if len(shots) != 1 || shots[0] != "test-results/tool-proof.png" {
		t.Fatalf("screenshot_refs = %+v", provenance["screenshot_refs"])
	}
	if !strings.Contains(string(pkg.VerifierContractsJSON), "human-behavior-proof") {
		t.Fatalf("verifier contracts missing human-behavior-proof: %s", string(pkg.VerifierContractsJSON))
	}
}

func TestDelegateWorkerVMToolRunsWorkerRuntimeAndCollectsExport(t *testing.T) {
	activeRT, activeStore, activeCWD := testRuntimeWithTempCWD(t)
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
				Name:      "publish_app_change_package",
				Arguments: json.RawMessage(exportArgs),
			}},
		},
		&ToolLoopResponse{
			StopReason: "end_turn",
			Text:       "Published background worker AppChangePackage.",
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
	mux.HandleFunc("/internal/runtime/app-change-packages/", workerHandler.HandleInternalAppChangePackageDetail)
	mux.HandleFunc("/internal/runtime/runs", workerHandler.HandleInternalRunSubmission)
	mux.HandleFunc("/internal/runtime/runs/", workerHandler.HandleInternalRuntimeRunRouter)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	superRun, err := activeRT.StartRunWithMetadata(context.Background(), "delegate to background worker", "user-alice", map[string]any{
		runMetadataAgentProfile: AgentProfileSuper,
		runMetadataAgentRole:    AgentProfileSuper,
		runMetadataTrajectoryID: "trace-worker-delegation",
		runMetadataChannelID:    "doc-worker-delegation",
		"requested_by_agent_id": "vtext:doc-worker-delegation",
		"requested_by_profile":  AgentProfileVText,
	})
	if err != nil {
		t.Fatalf("start active super run: %v", err)
	}
	registry := activeRT.ToolRegistryForProfile(AgentProfileSuper)
	raw, err := executeWorkerDelegationUntilSettled(t, registry, WithToolExecutionContext(context.Background(), superRun), json.RawMessage(fmt.Sprintf(`{
		"worker_sandbox_url": %q,
		"worker_id": "worker-proof",
		"vm_id": "vm-worker-proof",
		"objective": "Publish the committed worker AppChangePackage.",
		"profile": "co-super",
		"timeout_seconds": 10
	}`, srv.URL)))
	if err != nil {
		t.Fatalf("delegate_worker_vm: %v", err)
	}

	var result struct {
		State                               types.RunState   `json:"state"`
		WorkerVMID                          string           `json:"worker_vm_id"`
		AppChangePackages                   []map[string]any `json:"app_change_packages"`
		AppAdoptions                        []map[string]any `json:"app_adoptions"`
		ProductVisibleAppChangePackageCount int              `json:"product_visible_app_change_package_count"`
	}
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("decode delegate result: %v\n%s", err, raw)
	}
	if result.State != types.RunCompleted || result.WorkerVMID != "vm-worker-proof" || len(result.AppChangePackages) != 1 {
		t.Fatalf("unexpected delegate result: %+v\nraw=%s", result, raw)
	}
	if pushed, _ := result.AppChangePackages[0]["github_push"].(bool); pushed {
		t.Fatalf("worker export reported github push: %+v", result.AppChangePackages[0])
	}
	if got, _ := result.AppChangePackages[0]["candidate_head_sha"].(string); strings.TrimSpace(got) == "" {
		t.Fatalf("worker package missing candidate_head_sha: %+v", result.AppChangePackages[0])
	}
	if result.ProductVisibleAppChangePackageCount != 1 || result.AppChangePackages[0]["canonical_mirror_status"] != "mirrored" || result.AppChangePackages[0]["product_visible"] != true {
		t.Fatalf("worker package was not mirrored into active product store: %+v\nraw=%s", result.AppChangePackages, raw)
	}
	if len(result.AppAdoptions) != 0 {
		t.Fatalf("delegate_worker_vm must not create recipient adoptions during package collection, got %+v", result.AppAdoptions)
	}
	packageID, _ := result.AppChangePackages[0]["package_id"].(string)
	mirrored, err := activeStore.GetAppChangePackageForViewer(context.Background(), "user-bob", packageID)
	if err != nil {
		t.Fatalf("mirrored package should be visible to another authenticated viewer: %v", err)
	}
	if mirrored.PackageID != packageID || !strings.Contains(mirrored.RuntimeSourceDelta, "background worker change") {
		t.Fatalf("mirrored package missing full source delta: %+v", mirrored)
	}
	updates, err := activeStore.ListWorkerUpdatesByTrajectory(context.Background(), "user-alice", "trace-worker-delegation", 10)
	if err != nil {
		t.Fatalf("list worker updates: %v", err)
	}
	if len(updates) != 1 {
		t.Fatalf("worker update checkpoint count = %d, want 1; updates=%+v raw=%s", len(updates), updates, raw)
	}
	joinedFindings := strings.Join(updates[0].Findings, "\n")
	if !strings.Contains(joinedFindings, "returned 1 AppChangePackage") || strings.Contains(joinedFindings, "no AppChangePackages") {
		t.Fatalf("worker update checkpoint did not preserve successful package: %+v", updates[0])
	}
	if packageID == "" || !containsString(updates[0].EvidenceIDs, "app_change_package:"+packageID) {
		t.Fatalf("worker update checkpoint missing AppChangePackage evidence: %+v", updates[0])
	}
}

func TestStartWorkerDelegationPreloadsReferencedAppChangePackage(t *testing.T) {
	activeRT, _, activeCWD := testRuntimeWithTempCWD(t)
	if err := activeRT.InstallDefaultAgentTools(activeCWD); err != nil {
		t.Fatalf("install active tools: %v", err)
	}
	packageID := "37a05b90-1c85-483f-9cfc-6d4c4c129c1a"
	now := time.Now().UTC()
	_, err := activeRT.store.UpsertAppChangePackage(context.Background(), types.AppChangePackageRecord{
		PackageID:             packageID,
		OwnerID:               "user-alice",
		AppID:                 "human-proof-chyron",
		Status:                types.AppChangePackagePublishedUnlisted,
		Visibility:            "unlisted",
		SourceComputerID:      "source-computer",
		SourceCandidateID:     "candidate-chyron",
		SourceActiveRef:       "refs/computers/source/active",
		CandidateSourceRef:    "refs/computers/source/candidates/candidate-chyron",
		RuntimeSourceDelta:    "runtime delta",
		UISourceDelta:         "ui delta",
		PackageManifestSHA256: "manifest-sha",
		ManifestJSON:          json.RawMessage(`{"name":"human-proof-chyron"}`),
		ProvenanceRefsJSON:    json.RawMessage(`{"human_summary":"pending"}`),
		VerifierContractsJSON: json.RawMessage(`[]`),
		CreatedAt:             now,
		UpdatedAt:             now,
	})
	if err != nil {
		t.Fatalf("upsert package: %v", err)
	}

	var imported types.AppChangePackageRecord
	var submittedPrompt string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/internal/runtime/app-change-packages":
			if r.Header.Get("X-Internal-Caller") != "true" {
				t.Fatalf("missing internal caller header")
			}
			if err := json.NewDecoder(r.Body).Decode(&imported); err != nil {
				t.Fatalf("decode imported package: %v", err)
			}
			writeAPIJSON(w, http.StatusCreated, imported)
		case r.Method == http.MethodPost && r.URL.Path == "/internal/runtime/runs":
			var req internalRunSubmitRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode submitted run: %v", err)
			}
			submittedPrompt = req.Prompt
			writeAPIJSON(w, http.StatusAccepted, runStatusResponse{
				RunID:        "worker-run-preload",
				AgentID:      "worker-agent-preload",
				AgentProfile: AgentProfileVSuper,
				State:        types.RunPending,
				OwnerID:      "user-alice",
			})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
	}))
	defer srv.Close()

	superRun, err := activeRT.StartRunWithMetadata(context.Background(), "delegate package proof", "user-alice", map[string]any{
		runMetadataAgentProfile: AgentProfileSuper,
		runMetadataAgentRole:    AgentProfileSuper,
	})
	if err != nil {
		t.Fatalf("start active super run: %v", err)
	}
	registry := activeRT.ToolRegistryForProfile(AgentProfileSuper)
	raw, err := registry.Execute(WithToolExecutionContext(context.Background(), superRun), "start_worker_delegation", json.RawMessage(fmt.Sprintf(`{
		"worker_sandbox_url": %q,
		"worker_id": "worker-preload",
		"vm_id": "vm-preload",
		"objective": "Capture human proof for package id %s using product evidence.",
		"profile": "vsuper",
		"timeout_seconds": 10
	}`, srv.URL, packageID)))
	if err != nil {
		t.Fatalf("start worker delegation: %v", err)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("decode result: %v\n%s", err, raw)
	}
	if result["status"] != "worker_run_started" {
		t.Fatalf("status = %v, want worker_run_started: %s", result["status"], raw)
	}
	if imported.PackageID != packageID {
		t.Fatalf("preloaded package id = %q, want %q", imported.PackageID, packageID)
	}
	if !strings.Contains(submittedPrompt, "Referenced AppChangePackages have been preloaded") || !strings.Contains(submittedPrompt, packageID) {
		t.Fatalf("submitted prompt missing preload guidance:\n%s", submittedPrompt)
	}
}

func TestFinishWorkerDelegationMirrorsWorkerSubmitUpdateToActiveVText(t *testing.T) {
	activeRT, activeStore, activeCWD := testRuntimeWithTempCWD(t)
	if err := activeRT.InstallDefaultAgentTools(activeCWD); err != nil {
		t.Fatalf("install active tools: %v", err)
	}
	ctx := context.Background()
	ownerID := "user-alice"
	docID := "doc-worker-submit-mirror"
	now := time.Now().UTC()
	if err := activeStore.CreateDocument(ctx, types.Document{
		DocID:     docID,
		OwnerID:   ownerID,
		Title:     "Worker Submit Mirror",
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create active vtext document: %v", err)
	}
	if err := activeStore.UpsertAgent(ctx, types.AgentRecord{
		AgentID:   "vtext:" + docID,
		OwnerID:   ownerID,
		SandboxID: "active-sandbox",
		Profile:   AgentProfileVText,
		Role:      AgentProfileVText,
		ChannelID: docID,
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("upsert active vtext agent: %v", err)
	}

	workerDir := t.TempDir()
	workerDB, err := store.Open(filepath.Join(workerDir, "worker.db"))
	if err != nil {
		t.Fatalf("open worker store: %v", err)
	}
	workerProvider := newMockToolLoopProvider(
		&ToolLoopResponse{
			StopReason: "tool_use",
			ToolCalls: []types.ToolCall{{
				ID:   "call-worker-update",
				Name: "submit_coagent_update",
				Arguments: json.RawMessage(`{
					"update_id":"worker-direct-update",
					"findings":["WORKER_DIRECT_UPDATE: vsuper produced a substantive checkpoint."],
					"artifacts":["artifacts/chiron-proof.png"],
					"tests":["worker update routing verified"],
					"proposals":["Continue supervision from the active VText dashboard."]
				}`),
			}},
		},
		&ToolLoopResponse{
			StopReason: "end_turn",
			Text:       "Submitted direct worker update.",
		},
	)
	workerRT := New(Config{
		SandboxID:           "vm-worker-submit-mirror",
		StorePath:           filepath.Join(workerDir, "worker.db"),
		ProviderTimeout:     5 * time.Second,
		SupervisionInterval: time.Hour,
	}, workerDB, events.NewEventBus(), workerProvider)
	if err := workerRT.InstallDefaultAgentTools(workerDir); err != nil {
		t.Fatalf("install worker tools: %v", err)
	}
	workerCtx, cancelWorker := context.WithCancel(context.Background())
	defer cancelWorker()
	workerRT.Start(workerCtx)
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

	superRun, err := activeRT.StartRunWithMetadata(ctx, "delegate direct worker update proof", ownerID, map[string]any{
		runMetadataAgentProfile: AgentProfileSuper,
		runMetadataAgentRole:    AgentProfileSuper,
		runMetadataTrajectoryID: "traj-worker-submit-mirror",
		runMetadataChannelID:    docID,
		"requested_by_agent_id": "vtext:" + docID,
		"requested_by_profile":  AgentProfileVText,
	})
	if err != nil {
		t.Fatalf("start active super run: %v", err)
	}
	registry := activeRT.ToolRegistryForProfile(AgentProfileSuper)
	raw, err := executeWorkerDelegationUntilSettled(t, registry, WithToolExecutionContext(context.Background(), superRun), json.RawMessage(fmt.Sprintf(`{
		"worker_sandbox_url": %q,
		"worker_id": "worker-submit-mirror",
		"vm_id": "vm-worker-submit-mirror",
		"objective": "Submit one worker update for the VText dashboard. Do not publish an AppChangePackage.",
		"profile": "vsuper",
		"timeout_seconds": 10
	}`, srv.URL)))
	if err != nil {
		t.Fatalf("finish worker delegation: %v", err)
	}
	var result struct {
		Status                    string   `json:"status"`
		MirroredWorkerUpdateCount int      `json:"mirrored_worker_update_count"`
		MirroredWorkerUpdateIDs   []string `json:"mirrored_worker_update_ids"`
		WorkerUpdateCheckpoint    string   `json:"worker_update_checkpoint"`
	}
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("decode finish result: %v\n%s", err, raw)
	}
	if result.Status != "worker_run_completed" || result.MirroredWorkerUpdateCount != 1 || result.WorkerUpdateCheckpoint != "worker_submit_update_mirrored" {
		t.Fatalf("unexpected mirrored worker update result: %+v\nraw=%s", result, raw)
	}
	if len(result.MirroredWorkerUpdateIDs) != 1 {
		t.Fatalf("mirrored update ids = %+v", result.MirroredWorkerUpdateIDs)
	}

	updates, err := activeStore.ListWorkerUpdatesByTrajectory(ctx, ownerID, "traj-worker-submit-mirror", 10)
	if err != nil {
		t.Fatalf("list active worker updates: %v", err)
	}
	if len(updates) != 1 {
		t.Fatalf("active worker updates = %d, want only mirrored direct update; updates=%+v raw=%s", len(updates), updates, raw)
	}
	update := updates[0]
	if !strings.HasPrefix(update.UpdateID, "mirrored-worker-update-") || update.TargetAgentID != "vtext:"+docID || update.ChannelID != docID {
		t.Fatalf("unexpected mirrored update routing: %+v", update)
	}
	if !strings.Contains(strings.Join(update.Findings, "\n"), "WORKER_DIRECT_UPDATE") ||
		!containsString(update.Artifacts, "artifacts/chiron-proof.png") ||
		!containsString(update.Tests, "worker update routing verified") {
		t.Fatalf("mirrored update did not preserve worker payload: %+v", update)
	}
	messages, err := activeStore.ListChannelMessages(ctx, ownerID, docID, 0, 10)
	if err != nil {
		t.Fatalf("list active channel messages: %v", err)
	}
	if len(messages) != 1 || messages[0].FromRunID != superRun.RunID || messages[0].ToAgentID != "vtext:"+docID ||
		!strings.Contains(messages[0].Content, "WORKER_DIRECT_UPDATE") {
		t.Fatalf("mirrored update did not create active VText channel message: %+v", messages)
	}
}

func TestDelegateWorkerVMFollowsCompletedVSuperChildrenBeforeReturning(t *testing.T) {
	activeRT, _, activeCWD := testRuntimeWithTempCWD(t)
	if err := activeRT.InstallDefaultAgentTools(activeCWD); err != nil {
		t.Fatalf("install active tools: %v", err)
	}

	now := time.Now().UTC()
	rootEvents := []types.EventRecord{
		{
			EventID:   "root-spawn-child",
			RunID:     "worker-root",
			AgentID:   "agent-vsuper",
			OwnerID:   "user-alice",
			Timestamp: now,
			Kind:      types.EventToolResult,
			Payload: json.RawMessage(`{
				"tool":"spawn_agent",
				"is_error":false,
				"output":"{\"agent_id\":\"agent-child\",\"loop_id\":\"worker-child\",\"profile\":\"co-super\",\"state\":\"pending\"}"
			}`),
		},
	}
	childEvents := []types.EventRecord{
		{
			EventID:   "child-export",
			RunID:     "worker-child",
			AgentID:   "agent-child",
			OwnerID:   "user-alice",
			Timestamp: now.Add(time.Second),
			Kind:      types.EventToolResult,
			Payload: json.RawMessage(`{
				"tool":"publish_app_change_package",
				"is_error":false,
				"output":"{\"status\":\"published\",\"package_id\":\"pkg-child\",\"app_id\":\"trace-proof\",\"base_sha\":\"base-child\",\"candidate_head_sha\":\"head-child\",\"package_manifest_sha256\":\"manifest-child\",\"runtime_source_delta_sha256\":\"runtime-child\",\"ui_source_delta_sha256\":\"ui-child\"}"
			}`),
		},
	}

	var mu sync.Mutex
	childStatusPolled := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/internal/runtime/runs":
			writeAPIJSON(w, http.StatusAccepted, runStatusResponse{
				RunID:        "worker-root",
				AgentID:      "agent-vsuper",
				AgentProfile: AgentProfileVSuper,
				State:        types.RunPending,
				OwnerID:      "user-alice",
			})
		case r.Method == http.MethodGet && r.URL.Path == "/internal/runtime/runs/worker-root":
			writeAPIJSON(w, http.StatusOK, runStatusResponse{
				RunID:        "worker-root",
				AgentID:      "agent-vsuper",
				AgentProfile: AgentProfileVSuper,
				State:        types.RunCompleted,
				OwnerID:      "user-alice",
				Result:       "spawned children; waiting for export",
			})
		case r.Method == http.MethodGet && r.URL.Path == "/internal/runtime/runs/worker-root/events":
			writeAPIJSON(w, http.StatusOK, eventListResponse{Events: rootEvents})
		case r.Method == http.MethodGet && r.URL.Path == "/internal/runtime/runs/worker-child":
			mu.Lock()
			childStatusPolled = true
			mu.Unlock()
			writeAPIJSON(w, http.StatusOK, runStatusResponse{
				RunID:        "worker-child",
				AgentID:      "agent-child",
				AgentProfile: AgentProfileCoSuper,
				State:        types.RunCompleted,
				OwnerID:      "user-alice",
				Result:       "published child AppChangePackage",
			})
		case r.Method == http.MethodGet && r.URL.Path == "/internal/runtime/runs/worker-child/events":
			mu.Lock()
			ready := childStatusPolled
			mu.Unlock()
			if !ready {
				writeAPIJSON(w, http.StatusOK, eventListResponse{Events: nil})
				return
			}
			writeAPIJSON(w, http.StatusOK, eventListResponse{Events: childEvents})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
	}))
	defer srv.Close()

	superRun, err := activeRT.StartRunWithMetadata(context.Background(), "delegate child follow", "user-alice", map[string]any{
		runMetadataAgentProfile: AgentProfileSuper,
		runMetadataAgentRole:    AgentProfileSuper,
		runMetadataTrajectoryID: "trace-child-follow",
		runMetadataChannelID:    "doc-child-follow",
		"requested_by_profile":  AgentProfileVText,
	})
	if err != nil {
		t.Fatalf("start active super run: %v", err)
	}
	registry := activeRT.ToolRegistryForProfile(AgentProfileSuper)
	raw, err := executeWorkerDelegationUntilSettled(t, registry, WithToolExecutionContext(context.Background(), superRun), json.RawMessage(fmt.Sprintf(`{
		"worker_sandbox_url": %q,
		"worker_id": "worker-child-follow",
		"vm_id": "vm-child-follow",
		"objective": "spawn child and export",
		"profile": "vsuper",
		"timeout_seconds": 2
	}`, srv.URL)))
	if err != nil {
		t.Fatalf("delegate_worker_vm: %v", err)
	}

	var result struct {
		Status            string                    `json:"status"`
		State             types.RunState            `json:"state"`
		AppChangePackages []map[string]any          `json:"app_change_packages"`
		WorkerChildRunIDs []string                  `json:"worker_child_run_ids"`
		WorkerChildStates map[string]types.RunState `json:"worker_child_run_states"`
		CompletionBlocker string                    `json:"completion_blocker"`
		AppAdoptions      []map[string]any          `json:"app_adoptions"`
	}
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("decode delegate result: %v\n%s", err, raw)
	}
	if result.Status != "worker_run_completed" || result.State != types.RunCompleted {
		t.Fatalf("unexpected delegate status: %+v\nraw=%s", result, raw)
	}
	if result.CompletionBlocker != "" {
		t.Fatalf("completed child export should not be marked incomplete: %+v\nraw=%s", result, raw)
	}
	if len(result.AppChangePackages) != 1 || result.AppChangePackages[0]["loop_id"] != "worker-child" {
		t.Fatalf("child package was not collected after child follow-up: %+v\nraw=%s", result.AppChangePackages, raw)
	}
	if !containsString(result.WorkerChildRunIDs, "worker-child") || result.WorkerChildStates["worker-child"] != types.RunCompleted {
		t.Fatalf("child run follow-up evidence missing: %+v states=%+v raw=%s", result.WorkerChildRunIDs, result.WorkerChildStates, raw)
	}
	if len(result.AppAdoptions) != 0 {
		t.Fatalf("child package collection should not create recipient adoptions, got %+v\nraw=%s", result.AppAdoptions, raw)
	}
}

func TestDelegateWorkerVMMarksCompletedVSuperWithoutExportOrUpdateIncomplete(t *testing.T) {
	activeRT, _, activeCWD := testRuntimeWithTempCWD(t)
	if err := activeRT.InstallDefaultAgentTools(activeCWD); err != nil {
		t.Fatalf("install active tools: %v", err)
	}

	now := time.Now().UTC()
	rootEvents := []types.EventRecord{
		{
			EventID:   "root-spawn-child",
			RunID:     "worker-root-incomplete",
			AgentID:   "agent-vsuper-incomplete",
			OwnerID:   "user-alice",
			Timestamp: now,
			Kind:      types.EventToolResult,
			Payload: json.RawMessage(`{
				"tool":"spawn_agent",
				"is_error":false,
				"output":"{\"agent_id\":\"agent-child-incomplete\",\"loop_id\":\"worker-child-incomplete\",\"profile\":\"co-super\",\"state\":\"pending\"}"
			}`),
		},
	}
	childEvents := []types.EventRecord{
		{
			EventID:   "child-ack",
			RunID:     "worker-child-incomplete",
			AgentID:   "agent-child-incomplete",
			OwnerID:   "user-alice",
			Timestamp: now.Add(time.Second),
			Kind:      types.EventChannelMessage,
			Payload:   json.RawMessage(`{"from_agent_id":"agent-child-incomplete","role":"result","content":"Acknowledged; waiting for implementation evidence."}`),
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/internal/runtime/runs":
			writeAPIJSON(w, http.StatusAccepted, runStatusResponse{
				RunID:        "worker-root-incomplete",
				AgentID:      "agent-vsuper-incomplete",
				AgentProfile: AgentProfileVSuper,
				State:        types.RunPending,
				OwnerID:      "user-alice",
			})
		case r.Method == http.MethodGet && r.URL.Path == "/internal/runtime/runs/worker-root-incomplete":
			writeAPIJSON(w, http.StatusOK, runStatusResponse{
				RunID:        "worker-root-incomplete",
				AgentID:      "agent-vsuper-incomplete",
				AgentProfile: AgentProfileVSuper,
				State:        types.RunCompleted,
				OwnerID:      "user-alice",
				Result:       "spawned child and ended before export",
			})
		case r.Method == http.MethodGet && r.URL.Path == "/internal/runtime/runs/worker-root-incomplete/events":
			writeAPIJSON(w, http.StatusOK, eventListResponse{Events: rootEvents})
		case r.Method == http.MethodGet && r.URL.Path == "/internal/runtime/runs/worker-child-incomplete":
			writeAPIJSON(w, http.StatusOK, runStatusResponse{
				RunID:        "worker-child-incomplete",
				AgentID:      "agent-child-incomplete",
				AgentProfile: AgentProfileCoSuper,
				State:        types.RunCompleted,
				OwnerID:      "user-alice",
				Result:       "acknowledgement only",
			})
		case r.Method == http.MethodGet && r.URL.Path == "/internal/runtime/runs/worker-child-incomplete/events":
			writeAPIJSON(w, http.StatusOK, eventListResponse{Events: childEvents})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
	}))
	defer srv.Close()

	superRun, err := activeRT.StartRunWithMetadata(context.Background(), "delegate incomplete child", "user-alice", map[string]any{
		runMetadataAgentProfile: AgentProfileSuper,
		runMetadataAgentRole:    AgentProfileSuper,
		runMetadataTrajectoryID: "trace-child-incomplete",
		runMetadataChannelID:    "doc-child-incomplete",
		"requested_by_profile":  AgentProfileVText,
	})
	if err != nil {
		t.Fatalf("start active super run: %v", err)
	}
	registry := activeRT.ToolRegistryForProfile(AgentProfileSuper)
	raw, err := executeWorkerDelegationUntilSettled(t, registry, WithToolExecutionContext(context.Background(), superRun), json.RawMessage(fmt.Sprintf(`{
		"worker_sandbox_url": %q,
		"worker_id": "worker-child-incomplete",
		"vm_id": "vm-child-incomplete",
		"objective": "spawn child and either export or block",
		"profile": "vsuper",
		"timeout_seconds": 2
	}`, srv.URL)))
	if err != nil {
		t.Fatalf("delegate_worker_vm: %v", err)
	}

	var result struct {
		Status            string                    `json:"status"`
		State             types.RunState            `json:"state"`
		CompletionBlocker string                    `json:"completion_blocker"`
		TerminalError     string                    `json:"terminal_error"`
		AppChangePackages []map[string]any          `json:"app_change_packages"`
		WorkerChildStates map[string]types.RunState `json:"worker_child_run_states"`
	}
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("decode delegate result: %v\n%s", err, raw)
	}
	if result.Status != "worker_run_incomplete" || result.State != types.RunCompleted {
		t.Fatalf("unexpected incomplete delegate status: %+v\nraw=%s", result, raw)
	}
	if result.CompletionBlocker != "vsuper_completed_without_app_change_package_or_worker_update" || !strings.Contains(result.TerminalError, "completed after child coordination") {
		t.Fatalf("missing completion blocker evidence: %+v\nraw=%s", result, raw)
	}
	if len(result.AppChangePackages) != 0 {
		t.Fatalf("unexpected AppChangePackages for incomplete delegate: %+v\nraw=%s", result.AppChangePackages, raw)
	}
	if result.WorkerChildStates["worker-child-incomplete"] != types.RunCompleted {
		t.Fatalf("child completion state missing: %+v\nraw=%s", result.WorkerChildStates, raw)
	}
}

func TestDelegateWorkerVMMarksPackageRequiredVSuperWithoutPackageIncomplete(t *testing.T) {
	activeRT, _, activeCWD := testRuntimeWithTempCWD(t)
	if err := activeRT.InstallDefaultAgentTools(activeCWD); err != nil {
		t.Fatalf("install active tools: %v", err)
	}

	now := time.Now().UTC()
	workerEvents := []types.EventRecord{
		{
			EventID:   "worker-update-without-package",
			RunID:     "worker-package-required-no-package",
			AgentID:   "agent-vsuper-no-package",
			OwnerID:   "user-alice",
			Timestamp: now,
			Kind:      types.EventToolResult,
			Payload: json.RawMessage(`{
				"tool":"submit_coagent_update",
				"is_error":false,
				"output":"{\"status\":\"completed\",\"summary\":\"implemented candidate work but did not publish\"}"
			}`),
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/internal/runtime/runs":
			writeAPIJSON(w, http.StatusAccepted, runStatusResponse{
				RunID:        "worker-package-required-no-package",
				AgentID:      "agent-vsuper-no-package",
				AgentProfile: AgentProfileVSuper,
				State:        types.RunPending,
				OwnerID:      "user-alice",
			})
		case r.Method == http.MethodGet && r.URL.Path == "/internal/runtime/runs/worker-package-required-no-package":
			writeAPIJSON(w, http.StatusOK, runStatusResponse{
				RunID:        "worker-package-required-no-package",
				AgentID:      "agent-vsuper-no-package",
				AgentProfile: AgentProfileVSuper,
				State:        types.RunCompleted,
				OwnerID:      "user-alice",
				Result:       "completed candidate changes without package publication",
			})
		case r.Method == http.MethodGet && r.URL.Path == "/internal/runtime/runs/worker-package-required-no-package/events":
			writeAPIJSON(w, http.StatusOK, eventListResponse{Events: workerEvents})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
	}))
	defer srv.Close()

	superRun, err := activeRT.StartRunWithMetadata(context.Background(), "delegate package-required no-package", "user-alice", map[string]any{
		runMetadataAgentProfile: AgentProfileSuper,
		runMetadataAgentRole:    AgentProfileSuper,
		runMetadataTrajectoryID: "trace-package-required-no-package",
		runMetadataChannelID:    "doc-package-required-no-package",
		"requested_by_profile":  AgentProfileVText,
	})
	if err != nil {
		t.Fatalf("start active super run: %v", err)
	}
	registry := activeRT.ToolRegistryForProfile(AgentProfileSuper)
	raw, err := executeWorkerDelegationUntilSettled(t, registry, WithToolExecutionContext(context.Background(), superRun), json.RawMessage(fmt.Sprintf(`{
		"worker_sandbox_url": %q,
		"worker_id": "worker-package-required-no-package",
		"vm_id": "vm-package-required-no-package",
		"objective": "commit the candidate checkout and publish_app_change_package for an owner-pullable package",
		"profile": "vsuper",
		"timeout_seconds": 2
	}`, srv.URL)))
	if err != nil {
		t.Fatalf("delegate_worker_vm: %v", err)
	}

	var result struct {
		Status            string           `json:"status"`
		State             types.RunState   `json:"state"`
		CompletionBlocker string           `json:"completion_blocker"`
		TerminalError     string           `json:"terminal_error"`
		AppChangePackages []map[string]any `json:"app_change_packages"`
	}
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("decode delegate result: %v\n%s", err, raw)
	}
	if result.Status != "worker_run_incomplete" || result.State != types.RunCompleted {
		t.Fatalf("package-required delegate status = %+v, want incomplete\nraw=%s", result, raw)
	}
	if result.CompletionBlocker != "vsuper_completed_without_required_app_change_package" {
		t.Fatalf("completion blocker = %q, want required package blocker\nraw=%s", result.CompletionBlocker, raw)
	}
	if !strings.Contains(result.TerminalError, "without publish_app_change_package evidence") {
		t.Fatalf("terminal error missing package evidence blocker: %+v\nraw=%s", result, raw)
	}
	if len(result.AppChangePackages) != 0 {
		t.Fatalf("unexpected AppChangePackages: %+v\nraw=%s", result.AppChangePackages, raw)
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
	raw, err := executeWorkerDelegationUntilSettled(t, registry, WithToolExecutionContext(context.Background(), superRun), json.RawMessage(fmt.Sprintf(`{
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
		"git clone https://github.com/yusefmosiah/go-choir.git Source/candidate",
		"git checkout " + base,
		"Use repo_path \"Source/candidate\" and base_sha " + base,
		"Inspect the candidate checkout.",
	} {
		if !strings.Contains(workerRun.Prompt, want) {
			t.Fatalf("worker prompt missing %q in %q", want, workerRun.Prompt)
		}
	}
}

func TestPollInternalWorkerRunRetriesTransientStatusTimeout(t *testing.T) {
	var mu sync.Mutex
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls++
		call := calls
		mu.Unlock()

		if call == 1 {
			time.Sleep(60 * time.Millisecond)
			return
		}
		writeAPIJSON(w, http.StatusOK, runStatusResponse{
			RunID:        "run-transient-timeout",
			AgentID:      "agent-transient-timeout",
			AgentProfile: AgentProfileVSuper,
			State:        types.RunCompleted,
			OwnerID:      "user-alice",
		})
	}))
	defer srv.Close()

	client := &http.Client{Timeout: 20 * time.Millisecond}
	resp, err := pollInternalWorkerRun(context.Background(), client, srv.URL, "user-alice", "run-transient-timeout", time.Second)
	if err != nil {
		t.Fatalf("pollInternalWorkerRun: %v", err)
	}
	if resp.State != types.RunCompleted || resp.RunID != "run-transient-timeout" {
		t.Fatalf("unexpected status response: %+v", resp)
	}

	mu.Lock()
	defer mu.Unlock()
	if calls < 2 {
		t.Fatalf("expected retry after transient timeout, calls=%d", calls)
	}
}

func TestPollInternalWorkerRunRetriesTransientStatusCode(t *testing.T) {
	var mu sync.Mutex
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls++
		call := calls
		mu.Unlock()

		if call == 1 {
			writeAPIJSON(w, http.StatusBadGateway, apiError{Error: "worker runtime starting"})
			return
		}
		writeAPIJSON(w, http.StatusOK, runStatusResponse{
			RunID:        "run-transient-502",
			AgentID:      "agent-transient-502",
			AgentProfile: AgentProfileVSuper,
			State:        types.RunCompleted,
			OwnerID:      "user-alice",
		})
	}))
	defer srv.Close()

	resp, err := pollInternalWorkerRun(context.Background(), srv.Client(), srv.URL, "user-alice", "run-transient-502", time.Second)
	if err != nil {
		t.Fatalf("pollInternalWorkerRun: %v", err)
	}
	if resp.State != types.RunCompleted || resp.RunID != "run-transient-502" {
		t.Fatalf("unexpected status response: %+v", resp)
	}

	mu.Lock()
	defer mu.Unlock()
	if calls < 2 {
		t.Fatalf("expected retry after transient status code, calls=%d", calls)
	}
}

func TestPollInternalWorkerRunRetriesTransientNotFound(t *testing.T) {
	var mu sync.Mutex
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls++
		call := calls
		mu.Unlock()

		if call == 1 {
			writeAPIJSON(w, http.StatusNotFound, apiError{Error: "run not found yet"})
			return
		}
		writeAPIJSON(w, http.StatusOK, runStatusResponse{
			RunID:        "run-transient-404",
			AgentID:      "agent-transient-404",
			AgentProfile: AgentProfileCoSuper,
			State:        types.RunCompleted,
			OwnerID:      "user-alice",
		})
	}))
	defer srv.Close()

	resp, err := pollInternalWorkerRun(context.Background(), srv.Client(), srv.URL, "user-alice", "run-transient-404", time.Second)
	if err != nil {
		t.Fatalf("pollInternalWorkerRun: %v", err)
	}
	if resp.State != types.RunCompleted || resp.RunID != "run-transient-404" {
		t.Fatalf("unexpected status response: %+v", resp)
	}

	mu.Lock()
	defer mu.Unlock()
	if calls < 2 {
		t.Fatalf("expected retry after transient not found, calls=%d", calls)
	}
}

func TestMergeFollowedWorkerChildRunStatesPreservesRefreshedState(t *testing.T) {
	evidence := workerRunEvidence{
		ChildRunStates: map[string]types.RunState{
			"child-from-refresh": types.RunCompleted,
		},
		ChildStatusErrors: map[string]string{
			"child-from-refresh": "delegate_worker_vm status failed: 404 Not Found",
		},
	}

	got := mergeFollowedWorkerChildRunStates(evidence, map[string]types.RunState{
		"child-from-poll": types.RunCompleted,
	}, map[string]string{
		"child-from-refresh": "delegate_worker_vm status failed: 404 Not Found",
		"child-error":        "delegate worker child follow-up budget exhausted",
	})

	if got.ChildRunStates["child-from-refresh"] != types.RunCompleted {
		t.Fatalf("refreshed child state was discarded: %+v", got.ChildRunStates)
	}
	if got.ChildRunStates["child-from-poll"] != types.RunCompleted {
		t.Fatalf("polled child state was not merged: %+v", got.ChildRunStates)
	}
	if _, ok := got.ChildStatusErrors["child-from-refresh"]; ok {
		t.Fatalf("status error should not survive for child with refreshed state: %+v", got.ChildStatusErrors)
	}
	if got.ChildStatusErrors["child-error"] == "" {
		t.Fatalf("unresolved child status error missing: %+v", got.ChildStatusErrors)
	}
}

func TestPollInternalWorkerRunRetriesTransientConnectionReset(t *testing.T) {
	var mu sync.Mutex
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls++
		call := calls
		mu.Unlock()

		if call == 1 {
			hj, ok := w.(http.Hijacker)
			if !ok {
				t.Fatalf("response writer does not support hijack")
			}
			conn, _, err := hj.Hijack()
			if err != nil {
				t.Fatalf("hijack: %v", err)
			}
			_ = conn.Close()
			return
		}
		writeAPIJSON(w, http.StatusOK, runStatusResponse{
			RunID:        "run-transient-reset",
			AgentID:      "agent-transient-reset",
			AgentProfile: AgentProfileVSuper,
			State:        types.RunCompleted,
			OwnerID:      "user-alice",
		})
	}))
	defer srv.Close()

	resp, err := pollInternalWorkerRun(context.Background(), srv.Client(), srv.URL, "user-alice", "run-transient-reset", time.Second)
	if err != nil {
		t.Fatalf("pollInternalWorkerRun: %v", err)
	}
	if resp.State != types.RunCompleted || resp.RunID != "run-transient-reset" {
		t.Fatalf("unexpected status response: %+v", resp)
	}

	mu.Lock()
	defer mu.Unlock()
	if calls < 2 {
		t.Fatalf("expected retry after transient connection reset, calls=%d", calls)
	}
}

func TestDelegateWorkerVMRetriesInterruptedWorkerRunOnce(t *testing.T) {
	activeRT, _, activeCWD := testRuntimeWithTempCWD(t)
	if err := activeRT.InstallDefaultAgentTools(activeCWD); err != nil {
		t.Fatalf("install active tools: %v", err)
	}

	var mu sync.Mutex
	submitted := 0
	prompts := make([]string, 0, 2)
	metadata := make([]map[string]any, 0, 2)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/internal/runtime/runs":
			var req internalRunSubmitRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode submit: %v", err)
			}
			mu.Lock()
			submitted++
			call := submitted
			prompts = append(prompts, req.Prompt)
			metadata = append(metadata, req.Metadata)
			mu.Unlock()
			writeAPIJSON(w, http.StatusAccepted, runStatusResponse{
				RunID:        fmt.Sprintf("worker-run-%d", call),
				AgentID:      fmt.Sprintf("worker-agent-%d", call),
				AgentProfile: AgentProfileVSuper,
				State:        types.RunPending,
				OwnerID:      "user-alice",
			})
		case r.Method == http.MethodGet && r.URL.Path == "/internal/runtime/runs/worker-run-1":
			writeAPIJSON(w, http.StatusOK, runStatusResponse{
				RunID:        "worker-run-1",
				AgentID:      "worker-agent-1",
				AgentProfile: AgentProfileVSuper,
				State:        types.RunFailed,
				OwnerID:      "user-alice",
				Error:        "runtime restarted, run interrupted",
			})
		case r.Method == http.MethodGet && r.URL.Path == "/internal/runtime/runs/worker-run-1/events":
			writeAPIJSON(w, http.StatusOK, eventListResponse{Events: nil})
		case r.Method == http.MethodGet && r.URL.Path == "/internal/runtime/runs/worker-run-2":
			writeAPIJSON(w, http.StatusOK, runStatusResponse{
				RunID:        "worker-run-2",
				AgentID:      "worker-agent-2",
				AgentProfile: AgentProfileVSuper,
				State:        types.RunCompleted,
				OwnerID:      "user-alice",
			})
		case r.Method == http.MethodGet && r.URL.Path == "/internal/runtime/runs/worker-run-2/events":
			writeAPIJSON(w, http.StatusOK, eventListResponse{Events: nil})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
	}))
	defer srv.Close()

	superRun, err := activeRT.StartRunWithMetadata(context.Background(), "delegate retry", "user-alice", map[string]any{
		runMetadataAgentProfile: AgentProfileSuper,
		runMetadataAgentRole:    AgentProfileSuper,
		runMetadataTrajectoryID: "trace-worker-retry",
	})
	if err != nil {
		t.Fatalf("start active super run: %v", err)
	}
	registry := activeRT.ToolRegistryForProfile(AgentProfileSuper)
	raw, err := executeWorkerDelegationUntilSettled(t, registry, WithToolExecutionContext(context.Background(), superRun), json.RawMessage(fmt.Sprintf(`{
		"worker_sandbox_url": %q,
		"worker_id": "worker-retry",
		"vm_id": "vm-worker-retry",
		"objective": "export after retry",
		"profile": "vsuper",
		"timeout_seconds": 2
	}`, srv.URL)))
	if err != nil {
		t.Fatalf("delegate_worker_vm: %v", err)
	}
	var result struct {
		RunID string         `json:"loop_id"`
		State types.RunState `json:"state"`
	}
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("decode delegate result: %v\n%s", err, raw)
	}
	if result.RunID != "worker-run-1" || result.State != types.RunFailed {
		t.Fatalf("unexpected delegate result: %+v\nraw=%s", result, raw)
	}

	mu.Lock()
	defer mu.Unlock()
	if submitted != 1 {
		t.Fatalf("submitted worker runs = %d, want one async start; super should choose any retry", submitted)
	}
	if len(prompts) != 1 || !strings.Contains(prompts[0], "export after retry") {
		t.Fatalf("first prompt missing objective: %q", prompts)
	}
	if _, ok := metadata[0]["retry_of_run_id"]; ok {
		t.Fatalf("async start must not auto-submit a retry metadata=%+v", metadata[0])
	}
}

func TestDelegateWorkerVMReturnsFailedRunEvidence(t *testing.T) {
	activeRT, _, activeCWD := testRuntimeWithTempCWD(t)
	if err := activeRT.InstallDefaultAgentTools(activeCWD); err != nil {
		t.Fatalf("install active tools: %v", err)
	}

	now := time.Now().UTC()
	workerEvents := []types.EventRecord{
		{
			EventID:   "worker-event-spawn",
			RunID:     "worker-run-failed",
			AgentID:   "worker-agent-failed",
			OwnerID:   "user-alice",
			Timestamp: now,
			Kind:      types.EventToolResult,
			Payload: json.RawMessage(`{
				"tool":"spawn_agent",
				"is_error":false,
				"output":"{\"agent_id\":\"agent-worker-child\",\"profile\":\"co-super\"}"
			}`),
		},
		{
			EventID:   "worker-event-channel",
			RunID:     "worker-run-failed",
			AgentID:   "worker-agent-failed",
			OwnerID:   "user-alice",
			Timestamp: now.Add(time.Second),
			Kind:      types.EventChannelMessage,
			Payload:   json.RawMessage(`{"from_agent_id":"worker-agent-failed","to_agent_id":"agent-worker-child","content":"verification blocked before export"}`),
		},
		{
			EventID:   "worker-event-update",
			RunID:     "worker-run-failed",
			AgentID:   "worker-agent-failed",
			OwnerID:   "user-alice",
			Timestamp: now.Add(2 * time.Second),
			Kind:      types.EventToolResult,
			Payload:   json.RawMessage(`{"tool":"submit_coagent_update","is_error":false,"output":"{\"status\":\"blocked\",\"summary\":\"tool loop exhausted before export\"}"}`),
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/internal/runtime/runs":
			writeAPIJSON(w, http.StatusAccepted, runStatusResponse{
				RunID:        "worker-run-failed",
				AgentID:      "worker-agent-failed",
				AgentProfile: AgentProfileVSuper,
				State:        types.RunPending,
				OwnerID:      "user-alice",
			})
		case r.Method == http.MethodGet && r.URL.Path == "/internal/runtime/runs/worker-run-failed":
			writeAPIJSON(w, http.StatusOK, runStatusResponse{
				RunID:        "worker-run-failed",
				AgentID:      "worker-agent-failed",
				AgentProfile: AgentProfileVSuper,
				State:        types.RunFailed,
				OwnerID:      "user-alice",
				Error:        "tool loop: exceeded 200 iterations without end_turn",
			})
		case r.Method == http.MethodGet && r.URL.Path == "/internal/runtime/runs/worker-run-failed/events":
			writeAPIJSON(w, http.StatusOK, eventListResponse{Events: workerEvents})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
	}))
	defer srv.Close()

	superRun, err := activeRT.StartRunWithMetadata(context.Background(), "delegate failed worker", "user-alice", map[string]any{
		runMetadataAgentProfile: AgentProfileSuper,
		runMetadataAgentRole:    AgentProfileSuper,
		runMetadataTrajectoryID: "trace-worker-failed",
	})
	if err != nil {
		t.Fatalf("start active super run: %v", err)
	}
	registry := activeRT.ToolRegistryForProfile(AgentProfileSuper)
	raw, err := executeWorkerDelegationUntilSettled(t, registry, WithToolExecutionContext(context.Background(), superRun), json.RawMessage(fmt.Sprintf(`{
		"worker_sandbox_url": %q,
		"worker_id": "worker-failed",
		"vm_id": "vm-worker-failed",
		"objective": "export or return a precise blocker",
		"profile": "vsuper",
		"timeout_seconds": 2
	}`, srv.URL)))
	if err != nil {
		t.Fatalf("delegate_worker_vm should return structured failed-run evidence, got error: %v", err)
	}

	var result struct {
		Status                    string           `json:"status"`
		State                     types.RunState   `json:"state"`
		Error                     string           `json:"error"`
		TerminalError             string           `json:"terminal_error"`
		EventCount                int              `json:"event_count"`
		WorkerEventSummary        []map[string]any `json:"worker_event_summary"`
		WorkerSpawnedProfiles     []string         `json:"worker_spawned_profiles"`
		WorkerChannelMessageCount int              `json:"worker_channel_message_count"`
	}
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("decode delegate result: %v\n%s", err, raw)
	}
	if result.Status != "worker_run_failed" || result.State != types.RunFailed {
		t.Fatalf("unexpected failed worker result: %+v\nraw=%s", result, raw)
	}
	if !strings.Contains(result.Error, "exceeded 200 iterations") || !strings.Contains(result.TerminalError, "worker-run-failed") {
		t.Fatalf("missing terminal failure details: %+v\nraw=%s", result, raw)
	}
	if result.EventCount != len(workerEvents) || len(result.WorkerEventSummary) != len(workerEvents) {
		t.Fatalf("worker event evidence missing: %+v\nraw=%s", result, raw)
	}
	if len(result.WorkerSpawnedProfiles) != 1 || result.WorkerSpawnedProfiles[0] != "co-super" {
		t.Fatalf("spawn profile evidence missing: %+v\nraw=%s", result, raw)
	}
	if result.WorkerChannelMessageCount != 1 {
		t.Fatalf("channel evidence count = %d, want 1; raw=%s", result.WorkerChannelMessageCount, raw)
	}
}

func TestDelegateWorkerVMReturnsTimeoutRunEvidence(t *testing.T) {
	activeRT, s, activeCWD := testRuntimeWithTempCWD(t)
	if err := activeRT.InstallDefaultAgentTools(activeCWD); err != nil {
		t.Fatalf("install active tools: %v", err)
	}

	now := time.Now().UTC()
	workerEvents := []types.EventRecord{
		{
			EventID:   "worker-timeout-spawn",
			RunID:     "worker-run-timeout",
			AgentID:   "worker-agent-timeout",
			OwnerID:   "user-alice",
			Timestamp: now,
			Kind:      types.EventToolResult,
			Payload: json.RawMessage(`{
				"tool":"spawn_agent",
				"is_error":false,
				"output":"{\"agent_id\":\"agent-worker-verifier\",\"loop_id\":\"child-export-run\",\"profile\":\"co-super\"}"
			}`),
		},
		{
			EventID:   "worker-timeout-channel",
			RunID:     "worker-run-timeout",
			AgentID:   "worker-agent-timeout",
			OwnerID:   "user-alice",
			Timestamp: now.Add(time.Second),
			Kind:      types.EventChannelMessage,
			Payload:   json.RawMessage(`{"from_agent_id":"worker-agent-timeout","to_agent_id":"agent-worker-verifier","content":"verifier started before timeout"}`),
		},
	}
	childEvents := []types.EventRecord{
		{
			EventID:   "worker-timeout-child-export",
			RunID:     "child-export-run",
			AgentID:   "agent-worker-verifier",
			OwnerID:   "user-alice",
			Timestamp: now.Add(2 * time.Second),
			Kind:      types.EventToolResult,
			Payload: json.RawMessage(`{
				"tool":"publish_app_change_package",
				"is_error":false,
				"output":"{\"status\":\"published_unlisted\",\"package_id\":\"package-timeout\",\"package_manifest_sha256\":\"manifest-timeout\",\"base_sha\":\"base-timeout\",\"candidate_head_sha\":\"head-timeout\",\"recipient_build_required\":true}"
			}`),
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/internal/runtime/runs":
			writeAPIJSON(w, http.StatusAccepted, runStatusResponse{
				RunID:        "worker-run-timeout",
				AgentID:      "worker-agent-timeout",
				AgentProfile: AgentProfileVSuper,
				State:        types.RunPending,
				OwnerID:      "user-alice",
			})
		case r.Method == http.MethodGet && r.URL.Path == "/internal/runtime/runs/worker-run-timeout":
			writeAPIJSON(w, http.StatusOK, runStatusResponse{
				RunID:        "worker-run-timeout",
				AgentID:      "worker-agent-timeout",
				AgentProfile: AgentProfileVSuper,
				State:        types.RunRunning,
				OwnerID:      "user-alice",
			})
		case r.Method == http.MethodGet && r.URL.Path == "/internal/runtime/runs/worker-run-timeout/events":
			writeAPIJSON(w, http.StatusOK, eventListResponse{Events: workerEvents})
		case r.Method == http.MethodGet && r.URL.Path == "/internal/runtime/runs/child-export-run/events":
			writeAPIJSON(w, http.StatusOK, eventListResponse{Events: childEvents})
		case r.Method == http.MethodGet && r.URL.Path == "/internal/runtime/runs/child-export-run":
			writeAPIJSON(w, http.StatusOK, runStatusResponse{
				RunID:        "child-export-run",
				AgentID:      "agent-worker-verifier",
				AgentProfile: AgentProfileCoSuper,
				State:        types.RunCompleted,
				OwnerID:      "user-alice",
			})
		case r.Method == http.MethodGet && r.URL.Path == "/internal/runtime/app-change-packages/package-timeout":
			writeAPIJSON(w, http.StatusNotFound, apiError{Error: "app change package not found"})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
	}))
	defer srv.Close()

	superRun, err := activeRT.StartRunWithMetadata(context.Background(), "delegate timed-out worker", "user-alice", map[string]any{
		runMetadataAgentProfile: AgentProfileSuper,
		runMetadataAgentRole:    AgentProfileSuper,
		runMetadataTrajectoryID: "trace-worker-timeout",
		runMetadataChannelID:    "doc-worker-timeout",
		"requested_by_profile":  AgentProfileVText,
	})
	if err != nil {
		t.Fatalf("start active super run: %v", err)
	}
	registry := activeRT.ToolRegistryForProfile(AgentProfileSuper)
	toolCtx := WithToolExecutionContext(context.Background(), superRun)
	startRaw, err := registry.Execute(toolCtx, "delegate_worker_vm", json.RawMessage(fmt.Sprintf(`{
		"worker_sandbox_url": %q,
		"worker_id": "worker-timeout",
		"vm_id": "vm-worker-timeout",
		"objective": "export or return a precise blocker",
		"profile": "vsuper",
		"timeout_seconds": 1
	}`, srv.URL)))
	if err != nil {
		t.Fatalf("delegate_worker_vm start: %v", err)
	}
	var start map[string]any
	if err := json.Unmarshal([]byte(startRaw), &start); err != nil {
		t.Fatalf("decode async worker start: %v\n%s", err, startRaw)
	}
	raw, err := registry.Execute(toolCtx, "observe_worker_delegation", mustJSON(t, map[string]any{
		"worker_sandbox_url": stringMapValue(start, "worker_sandbox_url"),
		"worker_run_id":      stringMapValue(start, "worker_run_id"),
		"worker_id":          stringMapValue(start, "worker_id"),
		"vm_id":              stringMapValue(start, "worker_vm_id"),
		"profile":            AgentProfileVSuper,
	}))
	if err != nil {
		t.Fatalf("observe_worker_delegation should return structured active evidence, got error: %v", err)
	}

	var result struct {
		Status                    string            `json:"status"`
		RunID                     string            `json:"loop_id"`
		State                     types.RunState    `json:"state"`
		Error                     string            `json:"error"`
		TerminalError             string            `json:"terminal_error"`
		EventCount                int               `json:"event_count"`
		WorkerEventSummary        []map[string]any  `json:"worker_event_summary"`
		WorkerSpawnedProfiles     []string          `json:"worker_spawned_profiles"`
		WorkerChannelMessageCount int               `json:"worker_channel_message_count"`
		WorkerChildRunIDs         []string          `json:"worker_child_run_ids"`
		WorkerChildRunStates      map[string]string `json:"worker_child_run_states"`
		AppChangePackages         []map[string]any  `json:"app_change_packages"`
		AppAdoptions              []map[string]any  `json:"app_adoptions"`
		ReviewablePackageObserved bool              `json:"reviewable_package_observed"`
		CompletionBlocker         string            `json:"completion_blocker"`
	}
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("decode delegate result: %v\n%s", err, raw)
	}
	if result.Status != "worker_observed" || result.RunID != "worker-run-timeout" || result.State != types.RunRunning {
		t.Fatalf("unexpected active worker observation: %+v\nraw=%s", result, raw)
	}
	if result.EventCount != len(workerEvents)+len(childEvents) || len(result.WorkerEventSummary) != len(workerEvents)+len(childEvents) {
		t.Fatalf("worker event evidence missing: %+v\nraw=%s", result, raw)
	}
	if !containsString(result.WorkerChildRunIDs, "child-export-run") {
		t.Fatalf("child run evidence missing: %+v\nraw=%s", result, raw)
	}
	if len(result.AppChangePackages) != 1 ||
		result.AppChangePackages[0]["package_id"] != "package-timeout" ||
		result.AppChangePackages[0]["loop_id"] != "child-export-run" {
		t.Fatalf("child package evidence missing: %+v\nraw=%s", result.AppChangePackages, raw)
	}
	if !result.ReviewablePackageObserved || result.CompletionBlocker != "" {
		t.Fatalf("active export evidence should be reviewable without terminal blocker: %+v\nraw=%s", result, raw)
	}
	if result.WorkerChildRunStates["child-export-run"] != string(types.RunCompleted) {
		t.Fatalf("child status evidence missing: %+v\nraw=%s", result.WorkerChildRunStates, raw)
	}
	if len(result.AppAdoptions) != 0 {
		t.Fatalf("timeout package collection should not create recipient adoptions: %+v\nraw=%s", result.AppAdoptions, raw)
	}
	if len(result.WorkerSpawnedProfiles) != 1 || result.WorkerSpawnedProfiles[0] != "co-super" {
		t.Fatalf("spawn profile evidence missing: %+v\nraw=%s", result, raw)
	}
	if result.WorkerChannelMessageCount != 1 {
		t.Fatalf("channel evidence count = %d, want 1; raw=%s", result.WorkerChannelMessageCount, raw)
	}
	updates, err := s.ListWorkerUpdatesByTrajectory(context.Background(), "user-alice", "trace-worker-timeout", 10)
	if err != nil {
		t.Fatalf("list worker updates: %v", err)
	}
	if len(updates) != 1 {
		t.Fatalf("worker update checkpoint count = %d, want 1; updates=%+v raw=%s", len(updates), updates, raw)
	}
	if !strings.Contains(strings.Join(updates[0].Findings, "\n"), "worker_observed") ||
		!containsString(updates[0].Refs, "worker_vm:vm-worker-timeout") ||
		!containsString(updates[0].EvidenceIDs, "worker_loop:worker-run-timeout") {
		t.Fatalf("worker update checkpoint missing delegate evidence: %+v", updates[0])
	}
	joinedFindings := strings.Join(updates[0].Findings, "\n")
	if !strings.Contains(joinedFindings, "returned 1 AppChangePackage") || strings.Contains(joinedFindings, "no AppChangePackages") {
		t.Fatalf("worker update checkpoint did not preserve child package evidence: %+v", updates[0])
	}
	if !containsString(updates[0].Artifacts, "package-timeout") ||
		!containsString(updates[0].Artifacts, "manifest-timeout") {
		t.Fatalf("worker update checkpoint missing child export artifacts: %+v", updates[0])
	}
}

func TestFinishWorkerDelegationActiveIncludesWorkerEvidence(t *testing.T) {
	activeRT, s, activeCWD := testRuntimeWithTempCWD(t)
	if err := activeRT.InstallDefaultAgentTools(activeCWD); err != nil {
		t.Fatalf("install active tools: %v", err)
	}

	now := time.Now().UTC()
	workerEvents := []types.EventRecord{
		{
			EventID:   "worker-active-spawn",
			RunID:     "worker-run-active",
			AgentID:   "worker-agent-active",
			OwnerID:   "user-alice",
			Timestamp: now,
			Kind:      types.EventToolResult,
			Payload: json.RawMessage(`{
				"tool":"spawn_agent",
				"is_error":false,
				"output":"{\"agent_id\":\"agent-worker-impl\",\"loop_id\":\"child-implementation-active\",\"profile\":\"co-super\"}"
			}`),
		},
	}
	childEvents := []types.EventRecord{
		{
			EventID:   "worker-active-child-edit-failed",
			RunID:     "child-implementation-active",
			AgentID:   "agent-worker-impl",
			OwnerID:   "user-alice",
			Timestamp: now.Add(time.Second),
			Kind:      types.EventToolResult,
			Payload: json.RawMessage(`{
				"tool":"edit_file",
				"is_error":true,
				"output":"tool_error: old_string not found in frontend/src/lib/BottomBar.svelte"
			}`),
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/internal/runtime/runs/worker-run-active":
			writeAPIJSON(w, http.StatusOK, runStatusResponse{
				RunID:        "worker-run-active",
				AgentID:      "worker-agent-active",
				AgentProfile: AgentProfileVSuper,
				State:        types.RunRunning,
				OwnerID:      "user-alice",
			})
		case r.Method == http.MethodGet && r.URL.Path == "/internal/runtime/runs/worker-run-active/events":
			writeAPIJSON(w, http.StatusOK, eventListResponse{Events: workerEvents})
		case r.Method == http.MethodGet && r.URL.Path == "/internal/runtime/runs/child-implementation-active":
			writeAPIJSON(w, http.StatusOK, runStatusResponse{
				RunID:        "child-implementation-active",
				AgentID:      "agent-worker-impl",
				AgentProfile: AgentProfileCoSuper,
				State:        types.RunRunning,
				OwnerID:      "user-alice",
			})
		case r.Method == http.MethodGet && r.URL.Path == "/internal/runtime/runs/child-implementation-active/events":
			writeAPIJSON(w, http.StatusOK, eventListResponse{Events: childEvents})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
	}))
	defer srv.Close()

	superRun, err := activeRT.StartRunWithMetadata(context.Background(), "finish active worker", "user-alice", map[string]any{
		runMetadataAgentProfile: AgentProfileSuper,
		runMetadataAgentRole:    AgentProfileSuper,
		runMetadataTrajectoryID: "trace-finish-active",
		runMetadataChannelID:    "doc-finish-active",
		"requested_by_profile":  AgentProfileVText,
	})
	if err != nil {
		t.Fatalf("start active super run: %v", err)
	}
	registry := activeRT.ToolRegistryForProfile(AgentProfileSuper)
	toolCtx := WithToolExecutionContext(context.Background(), superRun)
	raw, err := registry.Execute(toolCtx, "finish_worker_delegation", mustJSON(t, map[string]any{
		"worker_sandbox_url": srv.URL,
		"worker_run_id":      "worker-run-active",
		"worker_id":          "worker-active",
		"vm_id":              "vm-worker-active",
		"profile":            AgentProfileVSuper,
		"objective":          "produce an AppChangePackage or precise blocker",
	}))
	if err != nil {
		t.Fatalf("finish_worker_delegation should return active evidence, got error: %v", err)
	}

	var result struct {
		Status                 string            `json:"status"`
		RunID                  string            `json:"loop_id"`
		State                  types.RunState    `json:"state"`
		FinishReady            bool              `json:"finish_ready"`
		EventCount             int               `json:"event_count"`
		WorkerEventSummary     []map[string]any  `json:"worker_event_summary"`
		WorkerChildRunIDs      []string          `json:"worker_child_run_ids"`
		WorkerChildRunStates   map[string]string `json:"worker_child_run_states"`
		AppChangePackages      []map[string]any  `json:"app_change_packages"`
		WorkerUpdateCheckpoint string            `json:"worker_update_checkpoint"`
	}
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("decode finish result: %v\n%s", err, raw)
	}
	if result.Status != "worker_run_active" || result.RunID != "worker-run-active" || result.State != types.RunRunning || result.FinishReady {
		t.Fatalf("unexpected active finish result: %+v\nraw=%s", result, raw)
	}
	if result.EventCount != len(workerEvents)+len(childEvents) || len(result.WorkerEventSummary) != len(workerEvents)+len(childEvents) {
		t.Fatalf("finish active evidence missing worker events: %+v\nraw=%s", result, raw)
	}
	if !containsString(result.WorkerChildRunIDs, "child-implementation-active") ||
		result.WorkerChildRunStates["child-implementation-active"] != string(types.RunRunning) {
		t.Fatalf("finish active evidence missing child run state: %+v\nraw=%s", result, raw)
	}
	if len(result.AppChangePackages) != 0 {
		t.Fatalf("active failed edit should not synthesize package evidence: %+v\nraw=%s", result.AppChangePackages, raw)
	}
	if !strings.Contains(raw, "old_string not found") {
		t.Fatalf("finish active result did not preserve child tool failure: %s", raw)
	}
	if result.WorkerUpdateCheckpoint != "submitted_or_existing" {
		t.Fatalf("finish active result did not checkpoint VText update: %+v\nraw=%s", result, raw)
	}

	updates, err := s.ListWorkerUpdatesByTrajectory(context.Background(), "user-alice", "trace-finish-active", 10)
	if err != nil {
		t.Fatalf("list worker updates: %v", err)
	}
	if len(updates) != 1 {
		t.Fatalf("worker update checkpoint count = %d, want 1; updates=%+v raw=%s", len(updates), updates, raw)
	}
	joinedFindings := strings.Join(updates[0].Findings, "\n")
	if !strings.Contains(joinedFindings, "worker event summary was preserved with 2 event") ||
		!strings.Contains(strings.Join(updates[0].Notes, "\n"), "checkpoint_source=async_finish_active") ||
		!strings.Contains(strings.Join(updates[0].Notes, "\n"), "active_worker_obligation=true") ||
		!containsString(updates[0].Refs, "worker_vm:vm-worker-active") ||
		!containsString(updates[0].EvidenceIDs, "worker_loop:worker-run-active") {
		t.Fatalf("worker update checkpoint missing active finish evidence: %+v", updates[0])
	}

	messages, err := s.ListChannelMessages(context.Background(), "user-alice", "doc-finish-active", 0, 10)
	if err != nil {
		t.Fatalf("list channel messages: %v", err)
	}
	var continuation string
	for _, message := range messages {
		if message.ToAgentID == persistentSuperAgentID("user-alice") {
			continuation = message.Content
			break
		}
	}
	if continuation == "" {
		t.Fatalf("active worker checkpoint should post a super continuation message, got %+v", messages)
	}
	for _, want := range []string{
		"Runtime supervision continuation required",
		"worker_run_id: worker-run-active",
		"worker_sandbox_url: " + srv.URL,
		"do not start a duplicate worker run",
		"observe_worker_delegation",
	} {
		if !strings.Contains(continuation, want) {
			t.Fatalf("super continuation missing %q:\n%s", want, continuation)
		}
	}
}

func TestDelegateWorkerVMReturnsSubmitFailureEvidence(t *testing.T) {
	activeRT, _, activeCWD := testRuntimeWithTempCWD(t)
	if err := activeRT.InstallDefaultAgentTools(activeCWD); err != nil {
		t.Fatalf("install active tools: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("closed server should not receive request")
	}))
	workerURL := srv.URL
	srv.Close()

	superRun, err := activeRT.StartRunWithMetadata(context.Background(), "delegate to unavailable worker", "user-alice", map[string]any{
		runMetadataAgentProfile: AgentProfileSuper,
		runMetadataAgentRole:    AgentProfileSuper,
		runMetadataTrajectoryID: "trace-worker-submit-failed",
	})
	if err != nil {
		t.Fatalf("start active super run: %v", err)
	}
	registry := activeRT.ToolRegistryForProfile(AgentProfileSuper)
	raw, err := executeWorkerDelegationUntilSettled(t, registry, WithToolExecutionContext(context.Background(), superRun), json.RawMessage(fmt.Sprintf(`{
		"worker_sandbox_url": %q,
		"worker_id": "worker-submit-failed",
		"vm_id": "vm-worker-submit-failed",
		"objective": "start a worker run",
		"profile": "vsuper",
		"timeout_seconds": 1
	}`, workerURL)))
	if err != nil {
		t.Fatalf("delegate_worker_vm should return structured submit failure evidence, got error: %v", err)
	}

	var result struct {
		Status        string `json:"status"`
		WorkerID      string `json:"worker_id"`
		WorkerVMID    string `json:"worker_vm_id"`
		Error         string `json:"error"`
		TerminalError string `json:"terminal_error"`
		EventCount    int    `json:"event_count"`
	}
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("decode delegate result: %v\n%s", err, raw)
	}
	if result.Status != "worker_run_submit_failed" || result.WorkerID != "worker-submit-failed" || result.WorkerVMID != "vm-worker-submit-failed" {
		t.Fatalf("unexpected submit failure result: %+v\nraw=%s", result, raw)
	}
	if !strings.Contains(result.Error, "delegate_worker_vm submit") || result.TerminalError == "" {
		t.Fatalf("missing submit failure details: %+v\nraw=%s", result, raw)
	}
	if result.EventCount != 0 {
		t.Fatalf("submit failure should not invent worker events: %+v\nraw=%s", result, raw)
	}
}

func TestPrepareRemoteWorkerRepoBootstrapUsesConfiguredSourceOutsideGit(t *testing.T) {
	cwd := t.TempDir()
	base := "5af8828e4e5087a2ce835d5d85de5d4acd936e7a"
	t.Setenv("RUNTIME_WORKER_REPO_REMOTE", "git@github.com:yusefmosiah/go-choir.git")
	t.Setenv("RUNTIME_WORKER_REPO_BASE_SHA", base+"-dirty")

	bootstrap, err := prepareRemoteWorkerRepoBootstrap(context.Background(), cwd, "http://172.27.0.2:8085", AgentProfileVSuper)
	if err != nil {
		t.Fatalf("prepare bootstrap: %v", err)
	}
	if !bootstrap.Enabled || bootstrap.Kind != "remote_git_clone" {
		t.Fatalf("bootstrap disabled or wrong kind: %+v", bootstrap)
	}
	if bootstrap.RemoteURL != "https://github.com/yusefmosiah/go-choir.git" || bootstrap.BaseSHA != base {
		t.Fatalf("bootstrap provenance mismatch: %+v", bootstrap)
	}
	for _, want := range []string{
		"mkdir -p Source/platform Source/user Source/candidate Build .choir",
		"git clone https://github.com/yusefmosiah/go-choir.git Source/platform",
		"git clone https://github.com/yusefmosiah/go-choir.git Source/candidate",
		"git config user.name \"Choir Worker\"",
		"git config user.email \"worker@choir.local\"",
		"git checkout " + base,
		"Use set -euo pipefail for multi-step bash commands",
		"Run gofmt, go test, node/npm, Obscura, and scripts directly from the checkout",
		"Do not run nix develop, nix build, or nix-store inside the worker VM",
		"Use repo_path \"Source/candidate\" and base_sha " + base,
	} {
		if !strings.Contains(bootstrap.WorkerPrompt, want) {
			t.Fatalf("worker prompt missing %q in %q", want, bootstrap.WorkerPrompt)
		}
	}
}

func TestPrepareRemoteWorkerRepoBootstrapPrefersConfiguredBaseOverGitHead(t *testing.T) {
	repo := t.TempDir()
	runGit(t, repo, "init")
	runGit(t, repo, "config", "user.name", "Choir Test")
	runGit(t, repo, "config", "user.email", "choir-test@example.com")
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("git head base\n"), 0o644); err != nil {
		t.Fatalf("write readme: %v", err)
	}
	runGit(t, repo, "add", "README.md")
	runGit(t, repo, "commit", "-m", "seed")
	runGit(t, repo, "remote", "add", "origin", "git@github.com:yusefmosiah/go-choir.git")

	envBase := "0d1527088c5774e74f5e4a796082652c5062eaa0"
	t.Setenv("RUNTIME_WORKER_REPO_BASE_SHA", envBase)

	bootstrap, err := prepareRemoteWorkerRepoBootstrap(context.Background(), repo, "http://172.27.0.2:8085", AgentProfileVSuper)
	if err != nil {
		t.Fatalf("prepare bootstrap: %v", err)
	}
	if !bootstrap.Enabled {
		t.Fatalf("bootstrap disabled: %+v", bootstrap)
	}
	if bootstrap.RemoteURL != "https://github.com/yusefmosiah/go-choir.git" {
		t.Fatalf("expected git remote to be retained, got %+v", bootstrap)
	}
	if bootstrap.BaseSHA != envBase {
		t.Fatalf("expected configured deployed base %s over git HEAD, got %+v", envBase, bootstrap)
	}
	if !strings.Contains(bootstrap.WorkerPrompt, "git checkout "+envBase) ||
		!strings.Contains(bootstrap.WorkerPrompt, "Use repo_path \"Source/candidate\" and base_sha "+envBase) {
		t.Fatalf("worker prompt did not use configured base: %q", bootstrap.WorkerPrompt)
	}
}

func TestWorkerVSuperDelegateContractPreventsCheckoutRaces(t *testing.T) {
	contract := workerVSuperDelegateContract(15 * time.Minute)
	for _, want := range []string{
		"Spawn the implementation co-super first",
		"Do not spawn slot=\"verifier\" until",
		"label that result stale",
		"exclusive writer for Source/candidate",
		"do not run reset, clean, edit, or commit commands",
		"verifier must inspect only after the implementation child has reported",
		"missing tools, failed tests, or package publication failure must end in submit_coagent_update",
		"both child runs finish without publish_app_change_package or submit_coagent_update",
		"publish_app_change_package",
		"human proof is still evidence_pending",
		"separate proof/adoption workers need a package id",
	} {
		if !strings.Contains(contract, want) {
			t.Fatalf("delegate contract missing %q in %q", want, contract)
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
				Name:      "publish_app_change_package",
				Arguments: exportArgs,
			}},
		},
		&ToolLoopResponse{
			StopReason: "end_turn",
			Text:       "Published local worktree AppChangePackage.",
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
	mux.HandleFunc("/internal/runtime/app-change-packages/", workerHandler.HandleInternalAppChangePackageDetail)
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
	raw, err := executeWorkerDelegationUntilSettled(t, registry, WithToolExecutionContext(context.Background(), superRun), json.RawMessage(fmt.Sprintf(`{
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
		State             types.RunState   `json:"state"`
		RunID             string           `json:"loop_id"`
		WorkerIsolation   string           `json:"worker_isolation"`
		WorkerWorktree    string           `json:"worker_worktree_path"`
		WorkerBranch      string           `json:"worker_branch"`
		WorkerBaseSHA     string           `json:"worker_base_sha"`
		AppChangePackages []map[string]any `json:"app_change_packages"`
		AppAdoptions      []map[string]any `json:"app_adoptions"`
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
	if len(result.AppChangePackages) != 1 || len(result.AppAdoptions) != 0 {
		t.Fatalf("expected one AppChangePackage and no old promotion queue entry: %+v", result)
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

type blockingExecuteProvider struct {
	started chan types.RunRecord
	release chan struct{}
	once    sync.Once
}

func newBlockingExecuteProvider() *blockingExecuteProvider {
	return &blockingExecuteProvider{
		started: make(chan types.RunRecord, 10),
		release: make(chan struct{}),
	}
}

func (p *blockingExecuteProvider) ProviderName() string { return "blocking-test" }

func (p *blockingExecuteProvider) RuntimeProviderPolicy() ProviderPolicy {
	return ProviderPolicy{ActiveProvider: "blocking-test"}
}

func (p *blockingExecuteProvider) Execute(ctx context.Context, task *types.RunRecord, emit EventEmitFunc) error {
	emit(types.EventRunProgress, "execution", json.RawMessage(`{"status":"started","provider":"blocking-test"}`))
	if !strings.Contains(task.Prompt, "Process the pending inbox deliveries addressed to you as the user's persistent super actor.") {
		task.Result = "non-super test run completed"
		return nil
	}
	select {
	case p.started <- *task:
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case <-p.release:
		task.Result = "super test run completed"
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (p *blockingExecuteProvider) waitForRun(t *testing.T, promptContains string) types.RunRecord {
	t.Helper()
	deadline := time.After(3 * time.Second)
	for {
		select {
		case rec := <-p.started:
			if strings.Contains(rec.Prompt, promptContains) {
				return rec
			}
		case <-deadline:
			t.Fatalf("timed out waiting for super run containing %q", promptContains)
		}
	}
}

func (p *blockingExecuteProvider) releaseOne() {
	select {
	case p.release <- struct{}{}:
	case <-time.After(3 * time.Second):
	}
}

func (p *blockingExecuteProvider) releaseAll() {
	p.once.Do(func() {
		close(p.release)
	})
}

func pendingDeliveriesForAgent(t *testing.T, s *store.Store, ownerID, agentID string) []types.InboxDelivery {
	t.Helper()
	deliveries, err := s.ListPendingInboxDeliveries(context.Background(), ownerID, agentID, 20)
	if err != nil {
		t.Fatalf("list pending deliveries for %s: %v", agentID, err)
	}
	return deliveries
}
