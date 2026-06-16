//go:build comprehensive

package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestSubmitTaskReturnsStableHandle(t *testing.T) {
	t.Parallel()
	rt, _ := testRuntime(t)
	ctx := context.Background()

	rec, err := rt.StartRun(ctx, "explain closures in Go", "user-alice")
	if err != nil {
		t.Fatalf("submit task: %v", err)
	}

	// Task should have a stable UUID handle.
	if rec.RunID == "" {
		t.Error("loop_id should not be empty")
	}
	if rec.State != types.RunPending {
		t.Errorf("state: got %q, want %q", rec.State, types.RunPending)
	}
	if rec.OwnerID != "user-alice" {
		t.Errorf("owner_id: got %q, want user-alice", rec.OwnerID)
	}
	if rec.Prompt != "explain closures in Go" {
		t.Errorf("prompt: got %q, want original prompt", rec.Prompt)
	}
	if rec.SandboxID != "sandbox-test" {
		t.Errorf("sandbox_id: got %q, want sandbox-test", rec.SandboxID)
	}
	if rec.CreatedAt.IsZero() {
		t.Error("created_at should not be zero")
	}
}

func TestSubmitTaskPersistsToStore(t *testing.T) {
	t.Parallel()
	rt, s := testRuntime(t)
	ctx := context.Background()

	rec, err := rt.StartRun(ctx, "test prompt", "user-bob")
	if err != nil {
		t.Fatalf("submit task: %v", err)
	}

	// Verify the task is persisted in the store.
	stored, err := s.GetRun(ctx, rec.RunID)
	if err != nil {
		t.Fatalf("get task from store: %v", err)
	}
	if stored.RunID != rec.RunID {
		t.Errorf("loop_id: got %q, want %q", stored.RunID, rec.RunID)
	}
	if stored.OwnerID != "user-bob" {
		t.Errorf("owner_id: got %q, want user-bob", stored.OwnerID)
	}
}

func TestConductorTaskNormalizesStructuredRouteResult(t *testing.T) {
	t.Parallel()
	rt, s := testRuntime(t)
	ctx := context.Background()

	rec, err := rt.StartRunWithMetadata(ctx, "hi", "user-alice", map[string]any{
		runMetadataAgentProfile:  "conductor",
		runMetadataAgentRole:     "conductor",
		"input_source":           "prompt_bar",
		"requested_app":          AgentProfileTexture,
		"seed_prompt":            "hi",
		"initial_document_title": "hi",
	})
	if err != nil {
		t.Fatalf("submit conductor task: %v", err)
	}

	stored := waitForRunTerminalState(t, rt, rec.RunID, "user-alice", 5*time.Second)
	if stored.State != types.RunCompleted {
		t.Fatalf("state: got %q, want %q", stored.State, types.RunCompleted)
	}

	var result struct {
		Action               string `json:"action"`
		App                  string `json:"app"`
		Title                string `json:"title"`
		SeedPrompt           string `json:"seed_prompt"`
		CreateInitialVersion bool   `json:"create_initial_version"`
		DocID                string `json:"doc_id"`
		UserRevisionID       string `json:"user_revision_id"`
		FramingRevisionID    string `json:"framing_revision_id"`
		InitialRevisionID    string `json:"initial_revision_id"`
		InitialRunID         string `json:"initial_loop_id"`
	}
	if err := json.Unmarshal([]byte(stored.Result), &result); err != nil {
		t.Fatalf("decode result json: %v\nraw=%q", err, stored.Result)
	}
	if result.Action != "open_app" {
		t.Fatalf("action: got %q, want open_app", result.Action)
	}
	if result.App != AgentProfileTexture {
		t.Fatalf("app: got %q, want %q", result.App, AgentProfileTexture)
	}
	if result.SeedPrompt != "hi" {
		t.Fatalf("seed_prompt: got %q, want hi", result.SeedPrompt)
	}
	if result.CreateInitialVersion {
		t.Fatal("create_initial_version: got true, want false")
	}
	if result.DocID == "" {
		t.Fatal("doc_id should not be empty")
	}
	if result.UserRevisionID == "" {
		t.Fatal("user_revision_id should not be empty")
	}
	if result.FramingRevisionID != "" {
		t.Fatalf("framing_revision_id = %q, want empty because conductor cannot write appagent text", result.FramingRevisionID)
	}
	if result.InitialRevisionID != result.UserRevisionID {
		t.Fatalf("initial_revision_id: got %q, want user seed revision %q", result.InitialRevisionID, result.UserRevisionID)
	}
	if result.InitialRunID == "" {
		t.Fatal("initial_loop_id should point to the product-path texture run")
	}

	doc, err := s.GetDocument(ctx, result.DocID, "user-alice")
	if err != nil {
		t.Fatalf("get document: %v", err)
	}
	if doc.CurrentRevisionID != result.UserRevisionID {
		t.Fatalf("document head: got %q, want user seed revision %q before Texture writes", doc.CurrentRevisionID, result.UserRevisionID)
	}

	v0, err := s.GetRevision(ctx, result.UserRevisionID, "user-alice")
	if err != nil {
		t.Fatalf("get v0 revision: %v", err)
	}
	if v0.AuthorKind != types.AuthorUser {
		t.Fatalf("v0 author_kind: got %q, want %q", v0.AuthorKind, types.AuthorUser)
	}
	if v0.Content != "" {
		t.Fatalf("v0 content: got %q, want empty prompt-bar intake revision", v0.Content)
	}
	meta := decodeRevisionMetadata(v0.Metadata)
	if metadataString(meta, "conductor_loop_id") != rec.RunID {
		t.Fatalf("v0 conductor_loop_id: got %q, want %q", metadataString(meta, "conductor_loop_id"), rec.RunID)
	}
	if metadataString(meta, "seed_prompt") != "hi" {
		t.Fatalf("v0 seed_prompt metadata: got %q, want hi", metadataString(meta, "seed_prompt"))
	}
	if !metadataBoolValue(meta, "prompt_bar_instruction_revision") {
		t.Fatalf("v0 prompt_bar_instruction_revision = %v, want true", meta["prompt_bar_instruction_revision"])
	}
	if metadataString(meta, "texture_version") != "v0" {
		t.Fatalf("v0 metadata version: got %q, want v0", metadataString(meta, "texture_version"))
	}

	runs, err := s.ListRunsByOwner(ctx, "user-alice", 20)
	if err != nil {
		t.Fatalf("list runs: %v", err)
	}
	foundInitialTextureRun := false
	for _, run := range runs {
		if run.AgentProfile == AgentProfileTexture && run.RunID == result.InitialRunID {
			foundInitialTextureRun = true
		}
	}
	if !foundInitialTextureRun {
		t.Fatalf("prompt creation should start a product-path texture run %q; runs=%+v", result.InitialRunID, runs)
	}
	waitForRunTerminalState(t, rt, result.InitialRunID, "user-alice", 5*time.Second)
	if mutation, err := s.GetPendingAgentMutationByDoc(ctx, result.DocID, "user-alice"); err != nil {
		t.Fatalf("get pending mutation: %v", err)
	} else if mutation != nil {
		t.Fatalf("initial texture run should not leave a dangling pending mutation after completion, got %+v", mutation)
	}
}

func TestConductorDecisionNormalizesToastAfterMaterializedTextureRoute(t *testing.T) {
	t.Parallel()
	rec := &types.RunRecord{
		RunID:   "run-conductor-toast-after-route",
		OwnerID: "user-alice",
		Result:  `{"action":"toast","message":"Opened the document."}`,
		Metadata: map[string]any{
			runMetadataAgentProfile:  AgentProfileConductor,
			runMetadataAgentRole:     AgentProfileConductor,
			"requested_app":          AgentProfileTexture,
			"seed_prompt":            "create a texture document",
			"initial_document_title": "create a texture document",
			"doc_id":                 "doc-texture-route",
			"user_revision_id":       "rev-user",
			"framing_revision_id":    "rev-framing",
			"initial_revision_id":    "rev-framing",
		},
	}

	var result conductorDecision
	if err := json.Unmarshal([]byte(normalizeConductorDecision(rec)), &result); err != nil {
		t.Fatalf("decode normalized decision: %v", err)
	}
	if result.Action != "open_app" || result.App != AgentProfileTexture {
		t.Fatalf("normalized decision = action %q app %q, want open_app/%s", result.Action, result.App, AgentProfileTexture)
	}
	if result.DocID != "doc-texture-route" || result.InitialRevisionID != "rev-framing" {
		t.Fatalf("normalized decision lost route metadata: %+v", result)
	}
}

func TestConductorPromptBarStructuredDecisionMaterializesTextureRoute(t *testing.T) {
	t.Parallel()
	rt, s := testRuntime(t)
	provider := rt.provider.(*StubProvider)
	provider.Result = `{"action":"open_app","app":"texture","title":"Durable document","initial_content":"# Durable document\n\nInitial conductor-authored abstract."}`
	rt.Start(context.Background())

	rec, err := rt.StartRunWithMetadata(context.Background(), "make a durable document", "user-alice", map[string]any{
		runMetadataAgentProfile:  AgentProfileConductor,
		runMetadataAgentRole:     AgentProfileConductor,
		"input_source":           "prompt_bar",
		"requested_app":          AgentProfileTexture,
		"seed_prompt":            "make a durable document",
		"initial_document_title": "make a durable document",
	})
	if err != nil {
		t.Fatalf("start conductor run: %v", err)
	}

	stored := waitForRunTerminalState(t, rt, rec.RunID, "user-alice", 5*time.Second)
	if stored.State != types.RunCompleted {
		t.Fatalf("state = %q error=%q", stored.State, stored.Error)
	}
	var result conductorDecision
	if err := json.Unmarshal([]byte(stored.Result), &result); err != nil {
		t.Fatalf("decode result: %v\n%s", err, stored.Result)
	}
	if result.Action != "open_app" || result.App != AgentProfileTexture || result.DocID == "" {
		t.Fatalf("conductor result = %+v, want materialized Texture route", result)
	}
	// Conductor decisions no longer ship canonical document content; the
	// texture agent owns authoring, so any provider-supplied initial_content
	// must be stripped from the materialized route.
	if result.InitialContent != "" {
		t.Fatalf("initial_content = %q, want empty (texture owns canonical content)", result.InitialContent)
	}
	if result.CreateInitialVersion == nil || *result.CreateInitialVersion {
		t.Fatalf("create_initial_version = %v, want explicit false", result.CreateInitialVersion)
	}
	if result.InitialLoopID == "" {
		t.Fatalf("conductor result missing initial texture revision loop id: %+v", result)
	}
	doc, err := s.GetDocument(context.Background(), result.DocID, "user-alice")
	if err != nil {
		t.Fatalf("get materialized document: %v", err)
	}
	if doc.CurrentRevisionID == "" {
		t.Fatalf("materialized document has no current revision: %+v", doc)
	}
	rev, err := s.GetRevision(context.Background(), doc.CurrentRevisionID, "user-alice")
	if err != nil {
		t.Fatalf("get seed prompt revision: %v", err)
	}
	if rev.Content != "" {
		t.Fatalf("seed revision content = %q, want empty prompt-bar instruction revision", rev.Content)
	}
	revMeta := decodeRevisionMetadata(rev.Metadata)
	if metadataString(revMeta, "seed_prompt") != "make a durable document" {
		t.Fatalf("seed revision metadata = %#v, want seed prompt", revMeta)
	}
	if !metadataBoolValue(revMeta, "prompt_bar_instruction_revision") {
		t.Fatalf("prompt_bar_instruction_revision = %v, want true", revMeta["prompt_bar_instruction_revision"])
	}
}

func TestConductorPromptBarTextureRouteFallsBackToSeedPromptContent(t *testing.T) {
	t.Parallel()
	rt, s := testRuntime(t)
	provider := rt.provider.(*StubProvider)
	provider.Result = `{"action":"open_app","app":"texture","title":"Fallback document"}`
	rt.Start(context.Background())

	rec, err := rt.StartRunWithMetadata(context.Background(), "Draft fallback content", "user-alice", map[string]any{
		runMetadataAgentProfile:  AgentProfileConductor,
		runMetadataAgentRole:     AgentProfileConductor,
		"input_source":           "prompt_bar",
		"requested_app":          AgentProfileTexture,
		"seed_prompt":            "Draft fallback content",
		"initial_document_title": "Fallback document",
	})
	if err != nil {
		t.Fatalf("start conductor run: %v", err)
	}

	stored := waitForRunTerminalState(t, rt, rec.RunID, "user-alice", 5*time.Second)
	if stored.State != types.RunCompleted {
		t.Fatalf("state = %q error=%q", stored.State, stored.Error)
	}
	var result conductorDecision
	if err := json.Unmarshal([]byte(stored.Result), &result); err != nil {
		t.Fatalf("decode result: %v\n%s", err, stored.Result)
	}
	if result.Action != "open_app" || result.App != AgentProfileTexture || result.DocID == "" {
		t.Fatalf("conductor result = %+v, want materialized Texture route", result)
	}
	// initial_content is intentionally empty on materialized Texture routes;
	// prompt-bar text is preserved as instruction metadata until Texture writes
	// prose.
	if result.InitialContent != "" {
		t.Fatalf("initial_content = %q, want empty (seed prompt lives in metadata)", result.InitialContent)
	}
	doc, err := s.GetDocument(context.Background(), result.DocID, "user-alice")
	if err != nil {
		t.Fatalf("get materialized document: %v", err)
	}
	rev, err := s.GetRevision(context.Background(), doc.CurrentRevisionID, "user-alice")
	if err != nil {
		t.Fatalf("get fallback revision: %v", err)
	}
	if rev.Content != "" {
		t.Fatalf("fallback revision content = %q, want empty prompt-bar instruction revision", rev.Content)
	}
	revMeta := decodeRevisionMetadata(rev.Metadata)
	if metadataString(revMeta, "seed_prompt") != "Draft fallback content" {
		t.Fatalf("fallback revision metadata = %#v, want seed prompt", revMeta)
	}
	if !metadataBoolValue(revMeta, "prompt_bar_instruction_revision") {
		t.Fatalf("prompt_bar_instruction_revision = %v, want true", revMeta["prompt_bar_instruction_revision"])
	}
}

func TestProviderPromptUsesPromptOverride(t *testing.T) {
	t.Parallel()
	rt := testPromptRuntime(t)
	if _, err := rt.PromptStore().Save("user-alice", AgentProfileConductor, "Custom conductor prompt"); err != nil {
		t.Fatalf("save prompt override: %v", err)
	}

	rec := &types.RunRecord{
		RunID:    "task-1",
		OwnerID:  "user-alice",
		Prompt:   "route this request",
		Metadata: map[string]any{runMetadataAgentProfile: AgentProfileConductor},
	}
	prompt, err := rt.providerPromptForRun(rec)
	if err != nil {
		t.Fatalf("providerPromptForRun: %v", err)
	}
	if !strings.Contains(prompt, "Custom conductor prompt") {
		t.Fatalf("provider prompt should include prompt override, got %q", prompt)
	}
	if !strings.Contains(prompt, "route this request") {
		t.Fatalf("provider prompt should include task prompt, got %q", prompt)
	}
}

func TestSystemPromptForTextureDefaultsToResearch(t *testing.T) {
	t.Parallel()
	rt := testPromptRuntime(t)

	rec := &types.RunRecord{
		RunID:        "run-texture-1",
		AgentID:      "texture:doc-1",
		ChannelID:    "doc-1",
		OwnerID:      "user-alice",
		AgentProfile: AgentProfileTexture,
		Prompt:       "what's the latest with ai",
	}

	prompt, err := rt.systemPromptForRun(rec)
	if err != nil {
		t.Fatalf("systemPromptForRun: %v", err)
	}
	if !strings.Contains(prompt, "open worker work first") {
		t.Fatalf("texture system prompt should bias toward opening worker work first for factual requests, got %q", prompt)
	}
	if !strings.Contains(prompt, "`spawn_agent` with `role=\"researcher\"`") {
		t.Fatalf("texture system prompt should route research through spawn_agent researcher, got %q", prompt)
	}
	if !strings.Contains(prompt, "Choose researcher parallelism from the task shape") {
		t.Fatalf("texture system prompt should make researcher parallelism contextual, got %q", prompt)
	}
	if !strings.Contains(prompt, "Current coordination channel: doc-1.") {
		t.Fatalf("texture system prompt should include coordination channel, got %q", prompt)
	}
	if !strings.Contains(prompt, "ask super to lease a worker VM and delegate a `vsuper`") ||
		!strings.Contains(prompt, "For bounded local scratch work such as API calls") {
		t.Fatalf("texture system prompt should preserve sweep substrate topology in super requests, got %q", prompt)
	}
}

func TestSystemPromptForSuperDelegatesChoirDevButAllowsScratch(t *testing.T) {
	t.Parallel()
	rt := testPromptRuntime(t)

	rec := &types.RunRecord{
		RunID:        "run-super-sweep",
		AgentID:      "agent-super-user-alice",
		ChannelID:    "doc-sweep",
		OwnerID:      "user-alice",
		AgentProfile: AgentProfileSuper,
		Prompt:       "Run a MissionGradient sweep substrate proof with worker/verifier cosupers.",
	}

	prompt, err := rt.systemPromptForRun(rec)
	if err != nil {
		t.Fatalf("systemPromptForRun: %v", err)
	}
	for _, want := range []string{
		"Super authority boundary",
		"bounded local scratch work is allowed",
		"API calls, curl fetches",
		"product_api_request",
		"instead of asking a worker VM to impersonate a browser session",
		"Delegate work that changes Choir/app/harness behavior",
		"first call request_worker_vm",
		"start_worker_delegation` using the returned `start_args",
		"observe_worker_delegation",
		"finish_worker_delegation",
		"Do not answer that class of request only with update_coagent",
		"worker-small",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("super system prompt missing %q in %q", want, prompt)
		}
	}
}

func TestWorkerRepoBootstrapContextReachesVSuperAndCoSuper(t *testing.T) {
	t.Parallel()
	rt := testPromptRuntime(t)
	metadata := map[string]any{
		runMetadataWorkerRepoRemote:    "https://github.com/yusefmosiah/go-choir.git",
		runMetadataWorkerRepoBaseSHA:   "abc123",
		runMetadataWorkerRepoBootstrap: "remote_git_clone",
	}

	for _, tc := range []struct {
		name    string
		profile string
	}{
		{name: "vsuper", profile: AgentProfileVSuper},
		{name: "co-super", profile: AgentProfileCoSuper},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := &types.RunRecord{
				RunID:        "run-" + tc.name,
				AgentID:      tc.name + ":agent",
				ChannelID:    "doc-1",
				OwnerID:      "user-alice",
				AgentProfile: tc.profile,
				Metadata:     metadata,
				Prompt:       "candidate repo work",
			}
			prompt, err := rt.systemPromptForRun(rec)
			if err != nil {
				t.Fatalf("systemPromptForRun: %v", err)
			}
			for _, want := range []string{
				"Worker candidate repo bootstrap context",
				"repo_path: Source/candidate",
				"base_sha: abc123",
				"mkdir -p Source/platform Source/user Source/candidate Build .choir",
				"git clone https://github.com/yusefmosiah/go-choir.git Source/platform",
				"git clone https://github.com/yusefmosiah/go-choir.git Source/candidate",
				"git config user.name \"Choir Worker\"",
				"git reset --hard abc123",
				"Use set -euo pipefail",
			} {
				if !strings.Contains(prompt, want) {
					t.Fatalf("prompt missing %q in %q", want, prompt)
				}
			}
		})
	}
}

func TestStartChildRunInheritsWorkerRepoMetadata(t *testing.T) {
	t.Parallel()
	rt, _ := testRuntime(t)
	ctx := context.Background()
	parent, err := rt.StartRun(ctx, "delegate worker", "user-alice")
	if err != nil {
		t.Fatalf("start parent: %v", err)
	}
	parent.Metadata = map[string]any{
		runMetadataAgentProfile:        AgentProfileVSuper,
		runMetadataWorkerRepoRemote:    "https://github.com/yusefmosiah/go-choir.git",
		runMetadataWorkerRepoBaseSHA:   "abc123",
		runMetadataWorkerRepoBootstrap: "remote_git_clone",
	}
	if err := rt.store.UpdateRun(ctx, *parent); err != nil {
		t.Fatalf("update parent metadata: %v", err)
	}

	child, err := rt.StartChildRun(ctx, parent.RunID, "implementation child", "user-alice", map[string]any{
		runMetadataAgentProfile: AgentProfileCoSuper,
		runMetadataAgentRole:    AgentProfileCoSuper,
	})
	if err != nil {
		t.Fatalf("start child: %v", err)
	}
	for _, key := range []string{
		runMetadataWorkerRepoRemote,
		runMetadataWorkerRepoBaseSHA,
		runMetadataWorkerRepoBootstrap,
	} {
		if got, want := metadataStringValue(child.Metadata, key), metadataStringValue(parent.Metadata, key); got != want {
			t.Fatalf("child metadata %s = %q, want %q", key, got, want)
		}
	}
}

func TestSystemPromptForResearcherForcesEarlyHandoff(t *testing.T) {
	t.Parallel()
	rt := testPromptRuntime(t)

	rec := &types.RunRecord{
		RunID:        "run-researcher-1",
		AgentID:      "researcher:doc-1:1",
		ChannelID:    "doc-1",
		OwnerID:      "user-alice",
		AgentProfile: AgentProfileResearcher,
		Prompt:       "Find the latest Anthropic model release notes and summarize what matters for the doc.",
	}

	prompt, err := rt.systemPromptForRun(rec)
	if err != nil {
		t.Fatalf("systemPromptForRun: %v", err)
	}
	if !strings.Contains(prompt, "Use web_search and fetch_url with the parallelism appropriate") {
		t.Fatalf("researcher system prompt should make tool parallelism contextual, got %q", prompt)
	}
	if !strings.Contains(prompt, "prefer import_document_content, list_content_item_selectors, and read_content_item_selector") {
		t.Fatalf("researcher system prompt should prefer document import/selector tools, got %q", prompt)
	}
	if !strings.Contains(prompt, "call update_coagent as a durable checkpoint") {
		t.Fatalf("researcher system prompt should require early findings handoff, got %q", prompt)
	}
	if !strings.Contains(prompt, "persistent communicating coagent") {
		t.Fatalf("researcher system prompt should describe persistent coagent research, got %q", prompt)
	}
	if !strings.Contains(prompt, "provider endpoints, latency, errors, rate limits, and result counts") {
		t.Fatalf("researcher system prompt should allow sequential follow-up after findings checkpoints, got %q", prompt)
	}
	if !strings.Contains(prompt, "rate-limit errors as backpressure") {
		t.Fatalf("researcher system prompt should treat rate limits as backpressure, got %q", prompt)
	}
	if !strings.Contains(prompt, "For live scores, schedules, rankings, weather") {
		t.Fatalf("researcher system prompt should anchor time-sensitive lookups, got %q", prompt)
	}
	if !strings.Contains(prompt, "do not treat blocked HTML scoreboard pages as the only possible source") {
		t.Fatalf("researcher system prompt should encourage structured sports source fallback, got %q", prompt)
	}
	if !strings.Contains(prompt, "verified final scores from live, pending, scheduled, or snippet-only states") {
		t.Fatalf("researcher system prompt should distinguish final sports evidence from partial states, got %q", prompt)
	}
	if !strings.Contains(prompt, "send another update_coagent after each additional search/fetch batch") {
		t.Fatalf("researcher system prompt should require incremental checkpoints after continued research, got %q", prompt)
	}
}

func TestSystemPromptForUniversalWireProfilesLoadsSharedHarnessPrompts(t *testing.T) {
	t.Parallel()
	rt := testPromptRuntime(t)

	for _, tc := range []struct {
		name    string
		profile string
		want    []string
	}{
		{
			name:    "processor",
			profile: AgentProfileProcessor,
			want: []string{
				"Universal Wire source-understanding agent",
				"SourceItem batches",
				"`spawn_agent` with `role=texture`",
				"Texture owns canonical article prose and researcher follow-up on the document channel",
				"record_wire_processor_decision",
				"update_coagent",
			},
		},
		{
			name:    "reconciler",
			profile: AgentProfileReconciler,
			want: []string{
				"corpus-level Universal Wire story agent",
				"existing published Textures",
				"`spawn_agent` with `role=texture`",
				"Texture owns canonical article prose and researcher follow-up on the document channel",
				"update_coagent",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := &types.RunRecord{
				RunID:        "run-" + tc.name,
				AgentID:      tc.name + ":universal-wire",
				ChannelID:    "universal-wire-channel",
				OwnerID:      "universal-wire-platform",
				AgentProfile: tc.profile,
				AgentRole:    tc.profile,
				Prompt:       "Process Universal Wire handoff.",
			}
			prompt, err := rt.systemPromptForRun(rec)
			if err != nil {
				t.Fatalf("systemPromptForRun: %v", err)
			}
			for _, want := range tc.want {
				if !strings.Contains(prompt, want) {
					t.Fatalf("%s prompt missing %q in %q", tc.name, want, prompt)
				}
			}
		})
	}
}

func TestSystemPromptForUniversalWireTextureRunsRequiresArticleHead(t *testing.T) {
	t.Parallel()
	rt := testPromptRuntime(t)

	universalWireRec := &types.RunRecord{
		RunID:        "run-universal-wire-texture",
		AgentID:      "texture:doc-universal-wire",
		ChannelID:    "doc-universal-wire",
		OwnerID:      "universal-wire-platform",
		AgentProfile: AgentProfileTexture,
		AgentRole:    AgentProfileTexture,
		Prompt:       "Write the first publication-quality article revision for this Texture document.",
		Metadata: map[string]any{
			"type":                    "texture_agent_revision",
			"doc_id":                  "doc-universal-wire",
			"source_network_cycle_id": "cycle-test",
			"request_intent":          "universal_wire_reconciler_article_revision",
			"selected_style_sources":  []map[string]any{{"title": "Style.texture: Universal Wire"}},
		},
	}
	prompt, err := rt.systemPromptForRun(universalWireRec)
	if err != nil {
		t.Fatalf("systemPromptForRun Universal Wire Texture: %v", err)
	}
	for _, want := range []string{
		"For Universal Wire article revision runs",
		"processor or reconciler handoff is newsroom source context",
		"first patch_texture call must write a publishable article",
		"not a Source Brief, Working Revision, Evidence Gathering note, outline, or placeholder",
		"Use uncertainty and native source handles in reader-facing article prose",
		"cite a bounded set of distinct listed handles with [label](source:ENTITY_ID)",
		"source refs only in source inventories or metadata sections do not count",
		"Use selected Style.texture sources to shape voice, structure, and editorial judgment",
		"do not name the selected Style.texture, style rationale, source inventory, or handoff mechanics in reader-facing prose",
		"do not end the run with the document head still at a brief or status checkpoint",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("Universal Wire Texture prompt missing %q in %q", want, prompt)
		}
	}
	if strings.Contains(prompt, "first call patch_texture with a short owner-readable working response") {
		t.Fatalf("Universal Wire Texture prompt should not use generic working-response rule: %q", prompt)
	}

	ordinaryRec := &types.RunRecord{
		RunID:        "run-ordinary-texture",
		AgentID:      "texture:doc-ordinary",
		ChannelID:    "doc-ordinary",
		OwnerID:      "user-alice",
		AgentProfile: AgentProfileTexture,
		AgentRole:    AgentProfileTexture,
		Prompt:       "What is going on today?",
		Metadata: map[string]any{
			"type":   "texture_agent_revision",
			"doc_id": "doc-ordinary",
		},
	}
	ordinaryPrompt, err := rt.systemPromptForRun(ordinaryRec)
	if err != nil {
		t.Fatalf("systemPromptForRun ordinary Texture: %v", err)
	}
	if !strings.Contains(ordinaryPrompt, "write the first useful owner-readable Texture revision") {
		t.Fatalf("ordinary Texture prompt should preserve generic working-response rule: %q", ordinaryPrompt)
	}
	if strings.Contains(ordinaryPrompt, "processor or reconciler handoff is newsroom source context") {
		t.Fatalf("ordinary Texture prompt should not get Universal Wire article-head rule: %q", ordinaryPrompt)
	}
}

func TestSystemPromptIncludesRepoSkillContext(t *testing.T) {
	t.Parallel()
	rt := testPromptRuntime(t)
	skillsRoot := t.TempDir()
	for _, skill := range []struct {
		name        string
		description string
		body        string
	}{
		{
			name:        "mission-gradient",
			description: "Shape long-running work around invariants and evidence.",
			body:        "# MissionGradient\n\nUse homotopy, not ladder.",
		},
		{
			name:        "cognitive-transform-portfolio",
			description: "Use route-changing lenses before stopping.",
			body:        "# Cognitive Transform Portfolio\n\nA transform changes action.",
		},
	} {
		dir := filepath.Join(skillsRoot, skill.name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("create skill dir: %v", err)
		}
		content := fmt.Sprintf("---\nname: %s\ndescription: %s\n---\n\n%s\n", skill.name, skill.description, skill.body)
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
			t.Fatalf("write skill: %v", err)
		}
	}
	rt.cfg.SkillsRoot = skillsRoot

	rec := &types.RunRecord{
		RunID:        "run-vsuper-skills",
		AgentID:      "vsuper:worker-1",
		ChannelID:    "doc-1",
		OwnerID:      "user-alice",
		AgentProfile: AgentProfileVSuper,
		Prompt:       "run a sweep",
	}

	prompt, err := rt.systemPromptForRun(rec)
	if err != nil {
		t.Fatalf("systemPromptForRun: %v", err)
	}
	for _, want := range []string{
		"Available repo skills",
		"natural-language use; no slash commands",
		"mission-gradient: Shape long-running work around invariants and evidence.",
		"cognitive-transform-portfolio: Use route-changing lenses before stopping.",
		"Use homotopy, not ladder.",
		"A transform changes action.",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("skill prompt missing %q in %q", want, prompt)
		}
	}
}

func TestGetRunCallerScoped(t *testing.T) {
	t.Parallel()
	rt, _ := testRuntime(t)
	ctx := context.Background()

	rec, err := rt.StartRun(ctx, "test prompt", "user-alice")
	if err != nil {
		t.Fatalf("submit task: %v", err)
	}

	// Owner can see their own task.
	got, err := rt.GetRun(ctx, rec.RunID, "user-alice")
	if err != nil {
		t.Fatalf("get own task: %v", err)
	}
	if got.RunID != rec.RunID {
		t.Errorf("loop_id: got %q, want %q", got.RunID, rec.RunID)
	}

	// Another user cannot see the task (VAL-RUNTIME-006).
	_, err = rt.GetRun(ctx, rec.RunID, "user-eve")
	if err == nil {
		t.Error("expected error when getting another user's task")
	}
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestGetRunNotFound(t *testing.T) {
	t.Parallel()
	rt, _ := testRuntime(t)
	ctx := context.Background()

	_, err := rt.GetRun(ctx, "nonexistent-task-id", "user-alice")
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestTaskCompletesSuccessfully(t *testing.T) {
	t.Parallel()
	rt, _ := testRuntime(t)
	ctx := context.Background()

	rec, err := rt.StartRun(ctx, "test prompt", "user-alice")
	if err != nil {
		t.Fatalf("submit task: %v", err)
	}

	got := waitForRunTerminalState(t, rt, rec.RunID, "user-alice", 5*time.Second)

	if got.State != types.RunCompleted {
		t.Errorf("state: got %q, want %q", got.State, types.RunCompleted)
	}
	if got.Result == "" {
		t.Error("result should not be empty for completed task")
	}
	if got.FinishedAt == nil {
		t.Error("finished_at should be set for completed task")
	}
}

func TestProviderFailureSurfacesStructuredOutcome(t *testing.T) {
	t.Parallel()
	// VAL-RUNTIME-008: provider failures surface as structured task outcomes
	// without crashing the runtime.
	dir := filepath.Join(os.TempDir(), "go-choir-m3-runtime-test")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	dbPath := filepath.Join(dir, t.Name()+".db")
	_ = os.Remove(dbPath)

	s, err := openTestStore(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	bus := events.NewEventBus()
	// Create a provider that always fails.
	provider := &StubProvider{
		Delay:   10 * time.Millisecond,
		FailErr: errors.New("provider timeout after 30s"),
		Result:  "",
	}

	cfg := Config{
		SandboxID:           "sandbox-test",
		StorePath:           dbPath,
		ProviderTimeout:     time.Second,
		SupervisionInterval: 1 * time.Hour,
	}

	rt := New(cfg, s, bus, provider)

	t.Cleanup(func() {
		rt.Stop()
		_ = s.Close()
		_ = os.Remove(dbPath)
	})

	rec, err := rt.StartRun(context.Background(), "failing prompt", "user-alice")
	if err != nil {
		t.Fatalf("submit task: %v", err)
	}

	got := waitForRunTerminalState(t, rt, rec.RunID, "user-alice", 5*time.Second)

	if got.State != types.RunFailed {
		t.Errorf("state: got %q, want %q", got.State, types.RunFailed)
	}
	if got.Error == "" {
		t.Error("error should be set for failed task")
	}
	if got.FinishedAt == nil {
		t.Error("finished_at should be set for failed task")
	}

	// Runtime should remain available for new runs.
	nextRec, err := rt.StartRun(context.Background(), "next prompt", "user-alice")
	if err != nil {
		t.Fatalf("submit task after failure: %v", err)
	}
	if nextRec.RunID == "" {
		t.Error("loop_id should not be empty for task submitted after failure")
	}
}

func TestRuntimeRemainsAvailableAfterProviderFailure(t *testing.T) {
	t.Parallel()
	// Verify that after a provider failure, the runtime is still healthy
	// and can accept and complete new runs (VAL-RUNTIME-008).
	rt, _ := testRuntime(t)
	ctx := context.Background()

	// Submit and complete a normal task.
	rec, err := rt.StartRun(ctx, "normal task", "user-alice")
	if err != nil {
		t.Fatalf("submit task: %v", err)
	}
	got := waitForRunTerminalState(t, rt, rec.RunID, "user-alice", 5*time.Second)
	if got.State != types.RunCompleted {
		t.Errorf("state: got %q, want %q", got.State, types.RunCompleted)
	}

	// Runtime health should still be ready.
	if rt.HealthState() != types.HealthReady {
		t.Errorf("health: got %q, want %q", rt.HealthState(), types.HealthReady)
	}
}

func TestEventEmissionOnTaskSubmission(t *testing.T) {
	t.Parallel()
	rt, _ := testRuntime(t)
	ctx := context.Background()

	// Subscribe to events before submitting.
	ch := rt.EventBus().Subscribe()
	defer rt.EventBus().Unsubscribe(ch)

	_, err := rt.StartRun(ctx, "test prompt", "user-alice")
	if err != nil {
		t.Fatalf("submit task: %v", err)
	}

	// Should receive a loop.submitted event.
	select {
	case ev := <-ch:
		if ev.Record.Kind != types.EventRunSubmitted {
			t.Errorf("event kind: got %q, want %q", ev.Record.Kind, types.EventRunSubmitted)
		}
		if ev.Record.OwnerID != "user-alice" {
			t.Errorf("event owner_id: got %q, want user-alice", ev.Record.OwnerID)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for loop.submitted event")
	}
}

func TestEventsPersistedToStore(t *testing.T) {
	t.Parallel()
	rt, s := testRuntime(t)
	ctx := context.Background()

	rec, err := rt.StartRun(ctx, "test prompt", "user-alice")
	if err != nil {
		t.Fatalf("submit task: %v", err)
	}

	// Wait for the task to complete and events to be persisted.
	time.Sleep(200 * time.Millisecond)

	// Check that events were persisted.
	evts, err := s.ListEvents(ctx, rec.RunID, 20)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}

	if len(evts) == 0 {
		t.Fatal("expected events to be persisted")
	}

	// First event should be loop.submitted.
	if evts[0].Kind != types.EventRunSubmitted {
		t.Errorf("first event kind: got %q, want %q", evts[0].Kind, types.EventRunSubmitted)
	}
}

func TestTaskRecoveryAcrossRestart(t *testing.T) {
	t.Parallel()
	// VAL-RUNTIME-010: accepted task state remains recoverable after
	// sandbox restart.
	dir := filepath.Join(os.TempDir(), "go-choir-m3-runtime-test")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	dbPath := filepath.Join(dir, t.Name()+".db")
	_ = os.Remove(dbPath)

	// Open store, create runtime, submit a task, and stop.
	s1, err := openTestStore(dbPath)
	if err != nil {
		t.Fatalf("open store 1: %v", err)
	}

	bus1 := events.NewEventBus()
	cfg := Config{
		SandboxID:           "sandbox-test",
		StorePath:           dbPath,
		ProviderTimeout:     time.Second,
		SupervisionInterval: 1 * time.Hour,
	}
	provider1 := NewStubProvider(50 * time.Millisecond)
	rt1 := New(cfg, s1, bus1, provider1)

	rec, err := rt1.StartRun(context.Background(), "survive restart", "user-alice")
	if err != nil {
		t.Fatalf("submit task: %v", err)
	}

	waitForRunTerminalState(t, rt1, rec.RunID, "user-alice", 5*time.Second)

	// Stop the first runtime.
	rt1.Stop()
	_ = s1.Close()

	// Reopen the store and create a new runtime (simulates restart).
	s2, err := openTestStore(dbPath)
	if err != nil {
		t.Fatalf("open store 2: %v", err)
	}

	bus2 := events.NewEventBus()
	provider2 := NewStubProvider(50 * time.Millisecond)
	rt2 := New(cfg, s2, bus2, provider2)

	t.Cleanup(func() {
		rt2.Stop()
		_ = s2.Close()
		_ = os.Remove(dbPath)
	})

	// The previously completed task should be recoverable by handle.
	got, err := rt2.GetRun(context.Background(), rec.RunID, "user-alice")
	if err != nil {
		t.Fatalf("get task after restart: %v", err)
	}

	if got.RunID != rec.RunID {
		t.Errorf("loop_id: got %q, want %q", got.RunID, rec.RunID)
	}
	if got.State != types.RunCompleted {
		t.Errorf("state: got %q, want %q", got.State, types.RunCompleted)
	}
	if got.Prompt != "survive restart" {
		t.Errorf("prompt: got %q, want original", got.Prompt)
	}
}

func TestInterruptedRunningTasksPassivatedOnStart(t *testing.T) {
	t.Parallel()
	// When the sandbox restarts, runs that were running should be passivated:
	// the in-process activation is gone, but durable agent work is not failed.
	dir := filepath.Join(os.TempDir(), "go-choir-m3-runtime-test")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	dbPath := filepath.Join(dir, t.Name()+".db")
	_ = os.Remove(dbPath)

	ctx := context.Background()

	// Create a store with a running task that was interrupted.
	s1, err := openTestStore(dbPath)
	if err != nil {
		t.Fatalf("open store 1: %v", err)
	}

	now := time.Now().UTC()
	interruptedTask := types.RunRecord{
		RunID:     "interrupted-task-001",
		OwnerID:   "user-alice",
		SandboxID: "sandbox-test",
		State:     types.RunRunning, // was running when process exited
		Prompt:    "interrupted prompt",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s1.CreateRun(ctx, interruptedTask); err != nil {
		t.Fatalf("create interrupted task: %v", err)
	}
	_ = s1.Close()

	// Simulate restart: open new store and runtime, then call Start()
	// which should passivate interrupted activations.
	s2, err := openTestStore(dbPath)
	if err != nil {
		t.Fatalf("open store 2: %v", err)
	}

	bus := events.NewEventBus()
	cfg := Config{
		SandboxID:           "sandbox-test",
		StorePath:           dbPath,
		ProviderTimeout:     time.Second,
		SupervisionInterval: 1 * time.Hour,
	}
	provider := NewStubProvider(50 * time.Millisecond)
	rt := New(cfg, s2, bus, provider)

	t.Cleanup(func() {
		rt.Stop()
		_ = s2.Close()
		_ = os.Remove(dbPath)
	})
	rt.Start(ctx)

	// The interrupted run should now be passivated, not failed.
	got, err := rt.GetRun(ctx, "interrupted-task-001", "user-alice")
	if err != nil {
		t.Fatalf("get interrupted task: %v", err)
	}
	if got.State != types.RunPassivated {
		t.Errorf("state: got %q, want %q", got.State, types.RunPassivated)
	}
	if got.Error != "" {
		t.Errorf("error: got %q, want empty", got.Error)
	}
	if got.FinishedAt != nil {
		t.Errorf("finished_at = %v, want nil", got.FinishedAt)
	}
	if metadataStringValue(got.Metadata, "passivated_reason") != "runtime_restarted" {
		t.Errorf("passivated_reason = %q, want runtime_restarted", metadataStringValue(got.Metadata, "passivated_reason"))
	}
}

func TestInterruptedActivationPassivationDrainsBatches(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(os.TempDir(), "go-choir-m3-runtime-test")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	dbPath := filepath.Join(dir, t.Name()+".db")
	_ = os.Remove(dbPath)

	ctx := context.Background()
	s, err := openTestStore(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() {
		_ = s.Close()
		_ = os.Remove(dbPath)
	})

	now := time.Now().UTC()
	states := []types.RunState{types.RunPending, types.RunRunning}
	for _, state := range states {
		for i := 0; i < 105; i++ {
			rec := types.RunRecord{
				RunID:     fmt.Sprintf("interrupted-%s-%03d", state, i),
				OwnerID:   "user-alice",
				SandboxID: "sandbox-test",
				State:     state,
				Prompt:    "interrupted prompt",
				CreatedAt: now,
				UpdatedAt: now,
			}
			if err := s.CreateRun(ctx, rec); err != nil {
				t.Fatalf("create %s run %d: %v", state, i, err)
			}
		}
	}

	rt := New(Config{SandboxID: "sandbox-test"}, s, events.NewEventBus(), NewStubProvider(0))
	rt.passivateInterruptedActivations(ctx)

	for _, state := range states {
		remaining, err := s.ListRunsByState(ctx, state, 200)
		if err != nil {
			t.Fatalf("list remaining %s runs: %v", state, err)
		}
		if len(remaining) != 0 {
			t.Fatalf("remaining %s runs = %d, want 0", state, len(remaining))
		}
		for i := 0; i < 105; i++ {
			runID := fmt.Sprintf("interrupted-%s-%03d", state, i)
			got, err := s.GetRun(ctx, runID)
			if err != nil {
				t.Fatalf("get passivated run %s: %v", runID, err)
			}
			if got.State != types.RunPassivated {
				t.Fatalf("%s state = %q, want %q", runID, got.State, types.RunPassivated)
			}
		}
	}
}

func TestHealthStartsReady(t *testing.T) {
	t.Parallel()
	rt, _ := testRuntime(t)

	if rt.HealthState() != types.HealthReady {
		t.Errorf("initial health: got %q, want %q", rt.HealthState(), types.HealthReady)
	}
}

func TestSetHealthTransitionsVisible(t *testing.T) {
	t.Parallel()
	// VAL-RUNTIME-001: health transitions are visible.
	rt, _ := testRuntime(t)
	ctx := context.Background()

	// Subscribe to events before transitioning.
	ch := rt.EventBus().Subscribe()
	defer rt.EventBus().Unsubscribe(ch)

	// Transition to degraded.
	rt.SetHealth(types.HealthDegraded)
	if rt.HealthState() != types.HealthDegraded {
		t.Errorf("health after set degraded: got %q, want %q", rt.HealthState(), types.HealthDegraded)
	}

	// Should have received a degraded event.
	select {
	case ev := <-ch:
		if ev.Record.Kind != types.EventRuntimeDegraded {
			t.Errorf("event kind: got %q, want %q", ev.Record.Kind, types.EventRuntimeDegraded)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for degraded event")
	}

	// Transition back to ready.
	rt.SetHealth(types.HealthReady)
	if rt.HealthState() != types.HealthReady {
		t.Errorf("health after set ready: got %q, want %q", rt.HealthState(), types.HealthReady)
	}

	// The health events should also be persisted for post-restart visibility.
	evts, _ := rt.Store().ListEvents(ctx, "", 20)
	_ = evts // not critical for this test
}

func TestSetHealthNoOpForSameState(t *testing.T) {
	t.Parallel()
	rt, _ := testRuntime(t)

	// Set health to ready (already ready) — should not emit an event.
	ch := rt.EventBus().Subscribe()
	defer rt.EventBus().Unsubscribe(ch)

	rt.SetHealth(types.HealthReady)

	select {
	case <-ch:
		t.Error("should not emit event for same health state")
	case <-time.After(50 * time.Millisecond):
		// Expected: no event.
	}
}

func TestListRunsByOwner(t *testing.T) {
	t.Parallel()
	rt, _ := testRuntime(t)
	ctx := context.Background()

	// Submit runs for two owners.
	_, err := rt.StartRun(ctx, "alice task 1", "user-alice")
	if err != nil {
		t.Fatalf("submit alice task: %v", err)
	}
	_, err = rt.StartRun(ctx, "bob task 1", "user-bob")
	if err != nil {
		t.Fatalf("submit bob task: %v", err)
	}
	_, err = rt.StartRun(ctx, "alice task 2", "user-alice")
	if err != nil {
		t.Fatalf("submit alice task 2: %v", err)
	}

	aliceTasks, err := rt.ListRunsByOwner(ctx, "user-alice", 10)
	if err != nil {
		t.Fatalf("list alice runs: %v", err)
	}
	if len(aliceTasks) != 2 {
		t.Errorf("alice runs: got %d, want 2", len(aliceTasks))
	}

	bobTasks, err := rt.ListRunsByOwner(ctx, "user-bob", 10)
	if err != nil {
		t.Fatalf("list bob runs: %v", err)
	}
	if len(bobTasks) != 1 {
		t.Errorf("bob runs: got %d, want 1", len(bobTasks))
	}
}

func TestProviderStubEmitsProgress(t *testing.T) {
	t.Parallel()
	rt, _ := testRuntime(t)
	ctx := context.Background()

	ch := rt.EventBus().Subscribe()
	defer rt.EventBus().Unsubscribe(ch)

	_, err := rt.StartRun(ctx, "progress test", "user-alice")
	if err != nil {
		t.Fatalf("submit task: %v", err)
	}

	// Collect events until the run completes instead of sleeping for a fixed
	// wall-clock delay.
	var received []events.RuntimeEvent
	timer := time.After(2 * time.Second)
	for {
		select {
		case ev := <-ch:
			if ev.Record.OwnerID == "user-alice" {
				received = append(received, ev)
				if ev.Record.Kind == types.EventRunCompleted {
					goto done
				}
			}
		case <-timer:
			t.Fatal("timed out waiting for completed event")
			goto done
		}
	}
done:

	// Should have received at least submitted, started, progress, and completed.
	kinds := make(map[types.EventKind]bool)
	for _, ev := range received {
		kinds[ev.Record.Kind] = true
	}

	if !kinds[types.EventRunSubmitted] {
		t.Error("expected loop.submitted event")
	}
	if !kinds[types.EventRunStarted] {
		t.Error("expected loop.started event")
	}
	if !kinds[types.EventRunProgress] {
		t.Error("expected loop.progress event")
	}
	if !kinds[types.EventRunCompleted] {
		t.Error("expected loop.completed event")
	}
}

func TestProviderStubDeltaEvent(t *testing.T) {
	t.Parallel()
	rt, s := testRuntime(t)
	ctx := context.Background()

	rec, err := rt.StartRun(ctx, "delta test", "user-alice")
	if err != nil {
		t.Fatalf("submit task: %v", err)
	}

	done := waitForRunTerminalState(t, rt, rec.RunID, "user-alice", 5*time.Second)
	if done.State != types.RunCompleted {
		t.Fatalf("state: got %s, want completed", done.State)
	}

	evts, err := s.ListEvents(ctx, rec.RunID, 50)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}

	hasDelta := false
	for _, ev := range evts {
		if ev.Kind == types.EventRunDelta {
			hasDelta = true
			// Check that the payload contains provider info.
			var payload map[string]string
			if err := json.Unmarshal(ev.Payload, &payload); err == nil {
				if payload["provider"] != "stub" {
					t.Errorf("delta payload provider: got %q, want stub", payload["provider"])
				}
			}
		}
	}
	if !hasDelta {
		t.Error("expected loop.delta event from stub provider")
	}
}

// --- Bridge Provider Integration Tests ---

// mockBridgeProvider implements the runtime.Provider interface for testing
// the bridge provider integration with the runtime engine.
type mockBridgeProvider struct {
	name       string
	result     string
	execErr    error
	mu         sync.Mutex
	called     bool
	taskResult string // captures the result set by Execute on the RunRecord
}

func (m *mockBridgeProvider) Execute(ctx context.Context, task *types.RunRecord, emit EventEmitFunc) error {
	m.mu.Lock()
	m.called = true
	m.mu.Unlock()

	if m.execErr != nil {
		emit(types.EventRunProgress, "execution", json.RawMessage(`{"status":"failed","real":"true"}`))
		return m.execErr
	}

	emit(types.EventRunProgress, "execution", json.RawMessage(`{"status":"started","provider":"`+m.name+`","real":"true"}`))
	emit(types.EventRunDelta, "execution", json.RawMessage(`{"text":"`+m.result+`","provider":"`+m.name+`","real":"true"}`))
	task.Result = m.result
	m.mu.Lock()
	m.taskResult = m.result
	m.mu.Unlock()
	return nil
}

func (m *mockBridgeProvider) ProviderName() string { return m.name }

func testRuntimeWithBridge(t *testing.T, bridge Provider) (*Runtime, *store.Store) {
	t.Helper()

	dir := filepath.Join(os.TempDir(), "go-choir-m3-bridge-test")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	dbPath := filepath.Join(dir, t.Name()+".db")
	_ = os.Remove(dbPath)

	s, err := openTestStore(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	bus := events.NewEventBus()
	cfg := Config{
		SandboxID:           "sandbox-bridge-test",
		StorePath:           dbPath,
		ProviderTimeout:     50 * time.Millisecond,
		SupervisionInterval: 1 * time.Hour,
	}

	rt := New(cfg, s, bus, bridge)
	t.Cleanup(func() {
		rt.Stop()
		_ = s.Close()
		_ = os.Remove(dbPath)
	})

	return rt, s
}

func TestBridgeProviderSubmitsAndCompletes(t *testing.T) {
	t.Parallel()
	bridge := &mockBridgeProvider{
		name:   "bedrock",
		result: "Real Bedrock response with genuine inference!",
	}

	rt, _ := testRuntimeWithBridge(t, bridge)
	ctx := context.Background()

	rec, err := rt.StartRun(ctx, "What is the capital of France?", "user-bridge")
	if err != nil {
		t.Fatalf("submit task: %v", err)
	}

	// Verify task completed with the bridge provider result.
	stored := waitForRunTerminalState(t, rt, rec.RunID, "user-bridge", 5*time.Second)
	if stored.State != types.RunCompleted {
		t.Errorf("state: got %q, want completed", stored.State)
	}
	if stored.Result != "Real Bedrock response with genuine inference!" {
		t.Errorf("result: got %q, want bridge provider result", stored.Result)
	}

	// Verify the bridge was actually called.
	if !bridge.called {
		t.Error("bridge provider was not called")
	}
}

func TestBridgeProviderFailureSurfacesWithoutCrashing(t *testing.T) {
	t.Parallel()
	bridge := &mockBridgeProvider{
		name:    "zai",
		execErr: fmt.Errorf("upstream provider timeout"),
	}

	rt, _ := testRuntimeWithBridge(t, bridge)
	ctx := context.Background()

	rec, err := rt.StartRun(ctx, "This should fail at the provider", "user-fail")
	if err != nil {
		t.Fatalf("submit task: %v", err)
	}

	// The task should be in failed state, not crashing the runtime.
	stored := waitForRunTerminalState(t, rt, rec.RunID, "user-fail", 5*time.Second)
	if stored.State != types.RunFailed {
		t.Errorf("state: got %q, want failed", stored.State)
	}

	// The runtime should still be healthy for later runs.
	if rt.HealthState() == types.HealthFailed {
		t.Error("runtime should not be in failed state after a single provider error")
	}

	// Submit another task — should still work.
	rec2, err := rt.StartRun(ctx, "Another task after failure", "user-retry")
	if err != nil {
		t.Fatalf("submit task after failure: %v", err)
	}
	if rec2.RunID == "" {
		t.Error("second task should have a valid ID")
	}
}

func TestBridgeProviderEventsContainRealMarker(t *testing.T) {
	t.Parallel()
	bridge := &mockBridgeProvider{
		name:   "zai",
		result: "Z.AI generated text",
	}

	rt, s := testRuntimeWithBridge(t, bridge)
	ctx := context.Background()

	rec, err := rt.StartRun(ctx, "test event markers", "user-events")
	if err != nil {
		t.Fatalf("submit task: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	evts, err := s.ListEvents(ctx, rec.RunID, 20)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}

	// Look for events with the "real":"true" marker that distinguishes
	// bridge provider events from stub provider events.
	hasRealMarker := false
	for _, ev := range evts {
		if ev.Kind == types.EventRunDelta || ev.Kind == types.EventRunProgress {
			var payload map[string]string
			if err := json.Unmarshal(ev.Payload, &payload); err == nil {
				if payload["real"] == "true" {
					hasRealMarker = true
					if payload["provider"] == "stub" {
						t.Error("real provider event should not have provider=stub")
					}
				}
			}
		}
	}
	if !hasRealMarker {
		t.Error("expected at least one event with real=true marker from bridge provider")
	}
}

func TestHealthReportsActiveProvider(t *testing.T) {
	t.Parallel()
	bridge := &mockBridgeProvider{
		name:   "bedrock",
		result: "test",
	}

	rt, _ := testRuntimeWithBridge(t, bridge)

	// The runtime's provider should report its name.
	if rt.provider.ProviderName() != "bedrock" {
		t.Errorf("provider name: got %q, want bedrock", rt.provider.ProviderName())
	}
}

// TestBuildAppagentRevisionMetadataPreservesDurableKeys verifies that
// appagent revisions carry forward seed_prompt, source_path, and
// conductor_loop_id from the parent revision metadata so subsequent
// revise requests retain the original user context.
func TestBuildAppagentRevisionMetadataPreservesDurableKeys(t *testing.T) {
	t.Parallel()
	rt, s := testRuntime(t)

	ctx := context.Background()
	ownerID := "test-user"

	// Create a document with a user-authored revision that has durable metadata.
	doc := types.Document{
		DocID:   "doc-meta-test",
		OwnerID: ownerID,
		Title:   "metadata test",
	}
	if err := s.CreateDocument(ctx, doc); err != nil {
		t.Fatalf("create document: %v", err)
	}

	parentMeta, _ := json.Marshal(map[string]any{
		"seed_prompt":                         "write a haiku about cats",
		"source_path":                         "/notes/cats.md",
		canonicalTextureSourcePathMetadataKey: "/notes/cats.texture",
		"conductor_loop_id":                   "task-original-conductor",
	})
	parentRev := types.Revision{
		RevisionID: "rev-parent-meta",
		DocID:      "doc-meta-test",
		OwnerID:    ownerID,
		AuthorKind: types.AuthorUser,
		Content:    "cats are great",
		Citations:  json.RawMessage("[]"),
		Metadata:   parentMeta,
		CreatedAt:  time.Now().UTC(),
	}
	if err := s.CreateRevision(ctx, parentRev); err != nil {
		t.Fatalf("create parent revision: %v", err)
	}

	// Point the document at the parent revision.
	doc.CurrentRevisionID = parentRev.RevisionID

	// Build appagent metadata with a task record that has no durable keys.
	rec := &types.RunRecord{
		RunID:    "task-agent-1",
		Metadata: map[string]any{"type": "texture_agent_revision"},
	}

	result := rt.buildAppagentRevisionMetadata(ctx, rec, doc, ownerID, nil)
	var resultMap map[string]any
	if err := json.Unmarshal(result, &resultMap); err != nil {
		t.Fatalf("unmarshal result metadata: %v", err)
	}

	// Verify durable keys are carried forward.
	if resultMap["seed_prompt"] != "write a haiku about cats" {
		t.Errorf("seed_prompt: got %v, want 'write a haiku about cats'", resultMap["seed_prompt"])
	}
	if resultMap["source_path"] != "/notes/cats.md" {
		t.Errorf("source_path: got %v, want '/notes/cats.md'", resultMap["source_path"])
	}
	if resultMap[canonicalTextureSourcePathMetadataKey] != "/notes/cats.texture" {
		t.Errorf("%s: got %v, want '/notes/cats.texture'", canonicalTextureSourcePathMetadataKey, resultMap[canonicalTextureSourcePathMetadataKey])
	}
	if resultMap["conductor_loop_id"] != "task-original-conductor" {
		t.Errorf("conductor_loop_id: got %v, want 'task-original-conductor'", resultMap["conductor_loop_id"])
	}

	// Verify agent-specific fields are also present.
	if resultMap["source"] != "patch_texture" {
		t.Errorf("source: got %v, want 'patch_texture'", resultMap["source"])
	}
	if resultMap["loop_id"] != "task-agent-1" {
		t.Errorf("loop_id: got %v, want 'task-agent-1'", resultMap["loop_id"])
	}

	rec.Metadata = map[string]any{
		"type":                                "texture_agent_revision",
		canonicalTextureSourcePathMetadataKey: "/notes/run-metadata.texture",
	}
	result = rt.buildAppagentRevisionMetadata(ctx, rec, doc, ownerID, nil)
	if err := json.Unmarshal(result, &resultMap); err != nil {
		t.Fatalf("unmarshal run metadata result: %v", err)
	}
	if resultMap[canonicalTextureSourcePathMetadataKey] != "/notes/run-metadata.texture" {
		t.Errorf("%s from run metadata: got %v, want '/notes/run-metadata.texture'", canonicalTextureSourcePathMetadataKey, resultMap[canonicalTextureSourcePathMetadataKey])
	}
}
