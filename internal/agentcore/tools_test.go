package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/sourceapi"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// --- Batch executor contract tests ---

func TestExecuteTools(t *testing.T) {
	registry := toolregistry.NewToolRegistry()

	echoTool := toolregistry.Tool{Name: "echo",
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			return string(args), nil
		}}
	if err := registry.Register(echoTool); err != nil {
		t.Fatalf("register: %v", err)
	}

	calls := []types.ToolCall{
		{ID: "call-1", Name: "echo", Arguments: json.RawMessage(`{"msg":"hello"}`)},
	}

	var emittedKinds []types.EventKind
	emit := func(kind types.EventKind, phase string, payload json.RawMessage) {
		emittedKinds = append(emittedKinds, kind)
	}

	results := toolregistry.ExecuteToolBatch(context.Background(), registry, calls, emit)

	if len(results) != 1 {
		t.Fatalf("results: got %d, want 1", len(results))
	}
	if results[0].CallID != "call-1" {
		t.Errorf("call_id: got %q, want call-1", results[0].CallID)
	}
	if results[0].Output != `{"msg":"hello"}` {
		t.Errorf("output: got %q, want echo result", results[0].Output)
	}
	if results[0].IsError {
		t.Error("should not be error")
	}

	// Should emit tool.invoked and tool.result events.
	if len(emittedKinds) != 2 {
		t.Fatalf("emitted events: got %d, want 2", len(emittedKinds))
	}
	if emittedKinds[0] != types.EventToolInvoked {
		t.Errorf("first event: got %q, want tool.invoked", emittedKinds[0])
	}
	if emittedKinds[1] != types.EventToolResult {
		t.Errorf("second event: got %q, want tool.result", emittedKinds[1])
	}
}

func TestExecuteToolsSkipsDuplicateTextureEditsInSameTurn(t *testing.T) {
	registry := toolregistry.NewToolRegistry()
	var executed int
	if err := registry.Register(toolregistry.Tool{Name: "patch_texture",
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			executed++
			return `{"status":"stored","revision_id":"rev-2"}`, nil
		}}); err != nil {
		t.Fatalf("register patch_texture: %v", err)
	}

	run := &types.RunRecord{
		RunID:        "run-texture",
		OwnerID:      "owner-1",
		AgentProfile: agentprofile.Texture,
		AgentRole:    agentprofile.Texture,
	}
	results := toolregistry.ExecuteToolBatch(toolregistry.WithExecutionContext(context.Background(), toolExecutionContextForRun(run)), registry, []types.ToolCall{
		{ID: "call-edit-1", Name: "patch_texture", Arguments: json.RawMessage(`{"doc_id":"doc-1"}`)},
		{ID: "call-edit-2", Name: "patch_texture", Arguments: json.RawMessage(`{"doc_id":"doc-1","content":"again"}`)},
	}, func(kind types.EventKind, phase string, payload json.RawMessage) {})

	if executed != 1 {
		t.Fatalf("executed patch_texture %d times, want 1", executed)
	}
	if len(results) != 2 {
		t.Fatalf("results = %d, want 2", len(results))
	}
	if results[0].IsError {
		t.Fatalf("first edit result = %#v, want success", results[0])
	}
	if results[1].IsError || !strings.Contains(results[1].Output, "duplicate Texture write tool patch_texture") {
		t.Fatalf("second edit result = %#v, want non-error duplicate notice", results[1])
	}
}

func TestExecuteToolsDoesNotSkipTextureEditAfterFailedAttempt(t *testing.T) {
	registry := toolregistry.NewToolRegistry()
	var executed int
	if err := registry.Register(toolregistry.Tool{Name: "patch_texture",
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			executed++
			if strings.Contains(string(args), "bad") {
				return "", fmt.Errorf("edit 0: find text not present")
			}
			return `{"status":"stored","revision_id":"rev-2"}`, nil
		}}); err != nil {
		t.Fatalf("register patch_texture: %v", err)
	}

	run := &types.RunRecord{
		RunID:        "run-texture",
		OwnerID:      "owner-1",
		AgentProfile: agentprofile.Texture,
		AgentRole:    agentprofile.Texture,
	}
	results := toolregistry.ExecuteToolBatch(toolregistry.WithExecutionContext(context.Background(), toolExecutionContextForRun(run)), registry, []types.ToolCall{
		{ID: "call-edit-1", Name: "patch_texture", Arguments: json.RawMessage(`{"doc_id":"doc-1","content":"bad"}`)},
		{ID: "call-edit-2", Name: "patch_texture", Arguments: json.RawMessage(`{"doc_id":"doc-1","content":"good"}`)},
	}, func(kind types.EventKind, phase string, payload json.RawMessage) {})

	if executed != 2 {
		t.Fatalf("executed patch_texture %d times, want 2", executed)
	}
	if len(results) != 2 {
		t.Fatalf("results = %d, want 2", len(results))
	}
	if !results[0].IsError {
		t.Fatalf("first edit result = %#v, want error", results[0])
	}
	if results[1].IsError || !strings.Contains(results[1].Output, `"status":"stored"`) {
		t.Fatalf("second edit result = %#v, want stored success", results[1])
	}
}

func TestExecuteToolsSkipsDuplicateTextureResearcherSpawnInSameTurn(t *testing.T) {
	registry := toolregistry.NewToolRegistry()
	var executed []string
	if err := registry.Register(toolregistry.Tool{Name: "spawn_agent",
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			var in struct {
				Role      string `json:"role"`
				Objective string `json:"objective"`
			}
			if err := json.Unmarshal(args, &in); err != nil {
				return "", err
			}
			executed = append(executed, in.Role+":"+in.Objective)
			return in.Role, nil
		}}); err != nil {
		t.Fatalf("register spawn_agent: %v", err)
	}

	run := &types.RunRecord{
		RunID:        "run-texture",
		OwnerID:      "owner-1",
		AgentProfile: agentprofile.Texture,
		AgentRole:    agentprofile.Texture,
	}
	results := toolregistry.ExecuteToolBatch(toolregistry.WithExecutionContext(context.Background(), toolExecutionContextForRun(run)), registry, []types.ToolCall{
		{ID: "research-1", Name: "spawn_agent", Arguments: json.RawMessage(`{"role":"researcher","channel_id":"doc-1","objective":"research current scores"}`)},
		{ID: "research-2", Name: "spawn_agent", Arguments: json.RawMessage(`{"role":"researcher","channel_id":"doc-1","objective":"research   current   scores"}`)},
		{ID: "research-3", Name: "spawn_agent", Arguments: json.RawMessage(`{"role":"researcher","channel_id":"doc-1","objective":"research injury notes"}`)},
	}, func(kind types.EventKind, phase string, payload json.RawMessage) {})

	if len(executed) != 2 {
		t.Fatalf("executed spawns = %#v, want duplicate skipped but distinct objective allowed", executed)
	}
	if len(results) != 3 {
		t.Fatalf("results = %d, want 3", len(results))
	}
	if results[0].IsError || results[0].Output != agentprofile.Researcher {
		t.Fatalf("first spawn result = %#v, want success", results[0])
	}
	if results[1].IsError || !strings.Contains(results[1].Output, "duplicate texture researcher spawn") {
		t.Fatalf("second spawn result = %#v, want non-error duplicate notice", results[1])
	}
	if results[2].IsError || results[2].Output != agentprofile.Researcher {
		t.Fatalf("third spawn result = %#v, want distinct-objective success", results[2])
	}
}

func TestExecuteToolsProjectionReturnsCompactOutputAndPreservesDurableEvidence(t *testing.T) {
	registry := toolregistry.NewToolRegistry()
	if err := registry.Register(toolregistry.Tool{Name: "projected",
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			return toolregistry.ProjectionResultJSON(map[string]any{"summary": "compact result", "results": []string{"a"}},
				map[string]any{"raw": strings.Repeat("full evidence ", 20)},
				map[string]any{"type": "test_projection"})
		}}); err != nil {
		t.Fatalf("register: %v", err)
	}

	var resultPayload map[string]any
	results := toolregistry.ExecuteToolBatch(context.Background(), registry, []types.ToolCall{
		{ID: "call-projected", Name: "projected", Arguments: json.RawMessage(`{}`)},
	}, func(kind types.EventKind, phase string, payload json.RawMessage) {
		if kind != types.EventToolResult {
			return
		}
		if err := json.Unmarshal(payload, &resultPayload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
	})

	if len(results) != 1 || results[0].IsError {
		t.Fatalf("results = %#v, want one successful projected result", results)
	}
	if strings.Contains(results[0].Output, "__choir_tool_projection") || strings.Contains(results[0].Output, "full evidence") {
		t.Fatalf("model-visible output leaked envelope/full evidence: %s", results[0].Output)
	}
	if !strings.Contains(results[0].Output, "compact result") {
		t.Fatalf("model-visible output = %s, want compact result", results[0].Output)
	}
	if resultPayload["full_output_sha256"] == "" || resultPayload["full_output"] == "" {
		t.Fatalf("result payload missing durable evidence fields: %#v", resultPayload)
	}
	if got := resultPayload["output"]; got != results[0].Output {
		t.Fatalf("event output = %#v, want model output %q", got, results[0].Output)
	}
	projection, ok := resultPayload["output_projection"].(map[string]any)
	if !ok || projection["type"] != "test_projection" {
		t.Fatalf("output_projection = %#v, want test_projection", resultPayload["output_projection"])
	}
}

func TestResearcherSourceSearchCallsSourceServiceAPI(t *testing.T) {
	ctx := context.Background()
	item := testSourceAPIItem()
	var sawSearch bool
	sourceServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/source-service/search" {
			t.Fatalf("unexpected source service path %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("q"); got != "rates" {
			t.Fatalf("source service query = %q, want rates", got)
		}
		if got := r.URL.Query().Get("max_results"); got != "5" {
			t.Fatalf("source service max_results = %q, want 5", got)
		}
		sawSearch = true
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(sourceapi.SearchResponse{
			Query:    "rates",
			Provider: sourceapi.ProviderName,
			Results:  []sourceapi.ItemResult{item},
			Metadata: sourceapi.Metadata{TargetKind: sourceapi.TargetKind},
		})
	}))
	defer sourceServer.Close()
	t.Setenv("SOURCE_SERVICE_BASE_URL", sourceServer.URL)
	t.Setenv("SOURCE_SERVICE_URL", "")
	t.Setenv("SOURCECYCLED_API_URL", "")
	t.Setenv("SOURCE_SERVICE_DB_PATH", "")
	t.Setenv("SOURCECYCLED_DB_PATH", "")

	rt, _ := testRuntime(t)
	if err := rt.InstallDefaultAgentTools(t.TempDir()); err != nil {
		t.Fatalf("install tools: %v", err)
	}
	researcherRegistry := rt.ToolRegistryForProfile(agentprofile.Researcher)
	if _, ok := researcherRegistry.Lookup("source_search"); !ok {
		t.Fatalf("researcher missing source_search")
	}
	textureRegistry := rt.ToolRegistryForProfile(agentprofile.Texture)
	if _, ok := textureRegistry.Lookup("source_search"); ok {
		t.Fatalf("texture should not have source_search")
	}

	rec := &types.RunRecord{
		RunID:        "researcher-source-search-run",
		OwnerID:      "owner-source-search",
		AgentProfile: agentprofile.Researcher,
		AgentRole:    agentprofile.Researcher,
	}
	raw, err := researcherRegistry.Execute(toolregistry.WithExecutionContext(ctx, toolExecutionContextForRun(rec)), "source_search", json.RawMessage(`{
		"query": "rates",
		"max_results": 5
	}`))
	if err != nil {
		t.Fatalf("source_search: %v", err)
	}
	var envelope struct {
		ModelOutput struct {
			Query           string `json:"query"`
			Provider        string `json:"provider"`
			ResultCount     int    `json:"result_count"`
			NextInstruction string `json:"next_instruction"`
			SourceIdentity  string `json:"source_identity"`
			Results         []struct {
				TargetKind      string   `json:"target_kind"`
				ItemID          string   `json:"item_id"`
				SourceID        string   `json:"source_id"`
				FetchID         string   `json:"fetch_id"`
				Title           string   `json:"title"`
				URL             string   `json:"url"`
				ContentHash     string   `json:"content_hash"`
				BodyKind        string   `json:"body_kind"`
				BodyLength      int      `json:"body_length"`
				ReaderSnapshot  bool     `json:"reader_snapshot"`
				EvidenceLevel   string   `json:"evidence_level"`
				VintagePolicy   string   `json:"vintage_policy"`
				LookaheadStatus string   `json:"lookahead_status"`
				Verticals       []string `json:"verticals"`
			} `json:"results"`
		} `json:"model_output"`
		DurableOutput struct {
			Results []map[string]any `json:"results"`
		} `json:"durable_output"`
		Projection struct {
			Type string `json:"type"`
		} `json:"projection"`
	}
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
		t.Fatalf("decode source_search projection: %v\nraw=%s", err, raw)
	}
	if !sawSearch {
		t.Fatalf("source_search did not call Source Service API")
	}
	if envelope.ModelOutput.Query != "rates" || envelope.ModelOutput.Provider != sourceapi.ProviderName {
		t.Fatalf("source_search model header = %+v", envelope.ModelOutput)
	}
	if envelope.ModelOutput.ResultCount != 1 || len(envelope.ModelOutput.Results) != 1 {
		t.Fatalf("source_search result count/model = %+v", envelope.ModelOutput)
	}
	got := envelope.ModelOutput.Results[0]
	if got.TargetKind != sourceapi.TargetKind || got.ItemID != item.ItemID || got.SourceID != item.SourceID || got.FetchID != item.FetchID {
		t.Fatalf("source identity = %+v, want item/source/fetch", got)
	}
	if got.ContentHash != item.ContentHash || got.EvidenceLevel != "official_release" || got.VintagePolicy != "release_snapshot" || got.LookaheadStatus != "no_lookahead" {
		t.Fatalf("source caveats/hash = %+v, want official caveats", got)
	}
	if got.BodyKind != item.BodyKind || got.BodyLength != item.BodyLength || got.ReaderSnapshot != item.ReaderSnapshot {
		t.Fatalf("source body classification = %+v, want %+v", got, item)
	}
	if got.Title != item.Title || got.URL != item.URL || len(got.Verticals) != 1 || got.Verticals[0] != "macro_policy" {
		t.Fatalf("source result projection = %+v", got)
	}
	if envelope.ModelOutput.NextInstruction == "" || !strings.Contains(envelope.ModelOutput.SourceIdentity, "source_service_item") {
		t.Fatalf("missing checkpoint/source identity guidance: %+v", envelope.ModelOutput)
	}
	if len(envelope.DurableOutput.Results) != 1 || envelope.Projection.Type != "source_search" {
		t.Fatalf("durable/projection output = %+v/%+v", envelope.DurableOutput, envelope.Projection)
	}
}

func TestResearcherSourceSearchWithoutConfiguredAPIIsUnavailable(t *testing.T) {
	t.Setenv("SOURCE_SERVICE_BASE_URL", "")
	t.Setenv("SOURCE_SERVICE_URL", "")
	t.Setenv("SOURCECYCLED_API_URL", "")
	t.Setenv("SOURCE_SERVICE_DB_PATH", "")
	t.Setenv("SOURCECYCLED_DB_PATH", "")
	rt, _ := testRuntime(t)
	if err := rt.InstallDefaultAgentTools(t.TempDir()); err != nil {
		t.Fatalf("install tools: %v", err)
	}
	researcherRegistry := rt.ToolRegistryForProfile(agentprofile.Researcher)
	_, err := researcherRegistry.Execute(toolregistry.WithExecutionContext(context.Background(), toolExecutionContextForRun(&types.RunRecord{
		RunID:        "researcher-source-search-unconfigured",
		OwnerID:      "owner-source-search",
		AgentProfile: agentprofile.Researcher,
	})), "source_search", json.RawMessage(`{"query":"rates"}`))
	if err == nil || !strings.Contains(err.Error(), "source search client not configured") {
		t.Fatalf("source_search err = %v, want source search client not configured", err)
	}
}

func testSourceAPIItem() sourceapi.ItemResult {
	return sourceapi.ItemResult{
		Rank:            1,
		TargetKind:      sourceapi.TargetKind,
		ItemID:          "srcitem_test_rates",
		SourceID:        "official:test",
		SourceType:      "rss",
		FetchID:         "fetch_test_rates",
		OriginalID:      "release-1",
		Title:           "Rate decision",
		Body:            "Rates held steady.",
		URL:             "https://example.test/release-1",
		CanonicalURL:    "https://example.test/release-1",
		PublishedAt:     "2026-06-04T12:00:00Z",
		FetchedAt:       "2026-06-04T12:01:00Z",
		Verticals:       []string{"macro_policy"},
		Language:        "en",
		Region:          "us",
		ContentHash:     "sha256-test-rates",
		BodyKind:        "reader_snapshot",
		BodyLength:      len("Rates held steady."),
		ReaderSnapshot:  true,
		EvidenceLevel:   "official_release",
		VintagePolicy:   "release_snapshot",
		LookaheadStatus: "no_lookahead",
		ReleaseDate:     "2026-06-04",
	}
}

func terminalResearcherRunFixture(runID, ownerID, docID, result string, state types.RunState, now time.Time) types.RunRecord {
	finishedAt := now
	rec := types.RunRecord{
		RunID:            runID,
		AgentID:          "researcher:" + runID,
		RequestedByRunID: "texture-parent:" + runID,
		ChannelID:        docID,
		OwnerID:          ownerID,
		AgentProfile:     agentprofile.Researcher,
		AgentRole:        agentprofile.Researcher,
		State:            state,
		Result:           result,
		CreatedAt:        now,
		UpdatedAt:        now,
		Metadata: map[string]any{
			runMetadataAgentProfile: agentprofile.Researcher,
			runMetadataAgentRole:    agentprofile.Researcher,
			runMetadataAgentID:      "researcher:" + runID,
			runMetadataChannelID:    docID,
			"requested_by_profile":  agentprofile.Texture,
			"requested_by_agent_id": "texture:" + docID,
		},
	}
	if state.Terminal() {
		rec.FinishedAt = &finishedAt
	}
	return rec
}

func TestRootTerminalRunSkipsOutcomeBindingWithoutStoreLookup(t *testing.T) {
	rt, _ := testRuntime(t)
	root := types.RunRecord{
		RunID:        "unpersisted-root-terminal",
		AgentID:      "co-super:unpersisted-root-terminal",
		AgentProfile: agentprofile.CoSuper,
		AgentRole:    agentprofile.CoSuper,
		State:        types.RunCompleted,
		Result:       "Root result has no requester.",
	}
	if err := rt.bindTerminalRunOutcome(context.Background(), &root, false); err != nil {
		t.Fatalf("root terminal run performed a binding store lookup: %v", err)
	}
}

func TestResearcherPlainTerminalResultBindsAddressedOutcome(t *testing.T) {
	for _, tc := range []struct {
		name   string
		marker func(map[string]any)
	}{
		{
			name: "plural legacy marker",
			marker: func(metadata map[string]any) {
				metadata["work_item_ids"] = []string{"legacy-work-item-researcher-plain-terminal"}
			},
		},
		{
			name: "singular legacy marker",
			marker: func(metadata map[string]any) {
				metadata["lifecycle_work_item_id"] = "legacy-work-item-researcher-plain-terminal"
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			rt, s := testRuntime(t)
			now := time.Now().UTC()
			suffix := strings.ReplaceAll(tc.name, " ", "-")
			rec := terminalResearcherRunFixture(
				"run-researcher-plain-terminal-"+suffix,
				"owner-researcher-plain-terminal-"+suffix,
				"doc-researcher-plain-terminal-"+suffix,
				"Plain terminal research result without any research-tool event.",
				types.RunCompleted,
				now,
			)
			rec.SandboxID = rt.TextureSandboxID()
			tc.marker(rec.Metadata)
			if err := s.CreateRun(ctx, rec); err != nil {
				t.Fatalf("create terminal researcher run: %v", err)
			}

			if err := rt.bindTerminalRunOutcome(ctx, &rec, false); err != nil {
				t.Fatalf("bind terminal researcher outcome: %v", err)
			}
			updates, err := s.ListPendingWorkerUpdates(ctx, rec.OwnerID, "texture:"+rec.ChannelID, 10)
			if err != nil {
				t.Fatalf("list bound terminal outcomes: %v", err)
			}
			if len(updates) != 1 {
				t.Fatalf("bound updates = %d, want exactly one: %+v", len(updates), updates)
			}
			update := updates[0]
			if update.SourceRunID != rec.RunID {
				t.Fatalf("source_run_id = %q, want %q", update.SourceRunID, rec.RunID)
			}
			wantDigest := types.TerminalRunOutcomeSHA256(rec.RunID, rec.State, rec.Result, rec.Error)
			if update.SourceOutcomeSHA256 != wantDigest {
				t.Fatalf("source_outcome_sha256 = %q, want recomputed %q", update.SourceOutcomeSHA256, wantDigest)
			}
			if strings.Contains(update.Content, rec.Result) {
				t.Fatalf("persisted reference envelope copied authoritative result: %q", update.Content)
			}
			projected, err := rt.projectTerminalOutcomeContent(ctx, updates)
			if err != nil {
				t.Fatalf("project authoritative terminal result: %v", err)
			}
			if len(projected) != 1 || !strings.Contains(projected[0].Content, rec.Result) {
				t.Fatalf("delivery projection omitted authoritative result: %+v", projected)
			}
			tampered := append([]types.CoagentSourcePacket(nil), updates...)
			tampered[0].SourceOutcomeSHA256 = "tampered"
			if _, err := rt.projectTerminalOutcomeContent(ctx, tampered); err == nil {
				t.Fatal("tampered terminal outcome digest projected without error")
			}
			persisted, err := s.GetWorkerUpdate(ctx, rec.OwnerID, update.UpdateID)
			if err != nil {
				t.Fatalf("reload terminal outcome reference: %v", err)
			}
			if strings.Contains(persisted.Content, rec.Result) {
				t.Fatalf("delivery projection mutated persisted envelope: %q", persisted.Content)
			}
		})
	}
}

func TestResearcherTerminalOutcomeRequiresDurableTerminalRun(t *testing.T) {
	ctx := context.Background()
	rt, s := testRuntime(t)
	now := time.Now().UTC()
	stored := terminalResearcherRunFixture(
		"run-researcher-terminal-order",
		"owner-researcher-terminal-order",
		"doc-researcher-terminal-order",
		"",
		types.RunRunning,
		now,
	)
	if err := s.CreateRun(ctx, stored); err != nil {
		t.Fatalf("create running researcher run: %v", err)
	}
	inMemoryTerminal := stored
	inMemoryTerminal.State = types.RunCompleted
	inMemoryTerminal.Result = "terminal result not durable yet"
	inMemoryTerminal.UpdatedAt = now.Add(time.Second)
	inMemoryTerminal.FinishedAt = &inMemoryTerminal.UpdatedAt

	if err := rt.bindTerminalRunOutcome(ctx, &inMemoryTerminal, false); err != nil {
		t.Fatalf("probe pre-persistence binding: %v", err)
	}
	updates, err := s.ListPendingWorkerUpdates(ctx, stored.OwnerID, "texture:"+stored.ChannelID, 10)
	if err != nil {
		t.Fatalf("list pre-persistence outcomes: %v", err)
	}
	if len(updates) != 0 {
		t.Fatalf("bound %d outcome(s) before terminal RunRecord persistence: %+v", len(updates), updates)
	}

	if err := s.UpdateRun(ctx, inMemoryTerminal); err != nil {
		t.Fatalf("persist terminal researcher run: %v", err)
	}
	if err := rt.bindTerminalRunOutcome(ctx, &inMemoryTerminal, false); err != nil {
		t.Fatalf("bind persisted terminal researcher outcome: %v", err)
	}
	updates, err = s.ListPendingWorkerUpdates(ctx, stored.OwnerID, "texture:"+stored.ChannelID, 10)
	if err != nil {
		t.Fatalf("list post-persistence outcomes: %v", err)
	}
	if len(updates) != 1 {
		t.Fatalf("bound outcomes after persistence = %d, want one: %+v", len(updates), updates)
	}
}

func TestBootBindsExplicitUpdateAcrossDispatchBeforeToolResultCrashWindow(t *testing.T) {
	ctx := context.Background()
	rt, s := testRuntime(t)
	now := time.Now().UTC()
	rec := terminalResearcherRunFixture(
		"run-researcher-explicit-terminal",
		"owner-researcher-explicit-terminal",
		"doc-researcher-explicit-terminal",
		"",
		types.RunRunning,
		now,
	)
	if err := s.CreateRun(ctx, rec); err != nil {
		t.Fatalf("create running researcher run: %v", err)
	}

	explicit := types.CoagentSourcePacket{
		OwnerID:       rec.OwnerID,
		AgentID:       rec.AgentID,
		TargetAgentID: "texture:" + rec.ChannelID,
		ChannelID:     rec.ChannelID,
		Role:          agentprofile.Researcher,
		SourceRunID:   rec.RunID,
		Packet: newCoagentPacket(
			"evidence_update",
			"Explicit final researcher update.",
			[]types.CoagentPacketClaim{coagentClaim("Explicit terminal evidence.")},
			nil,
			nil,
			nil,
			nil,
		),
		CreatedAt: now,
	}
	explicit.UpdateID = deriveWorkerUpdateID(explicit)
	explicit.Content = buildWorkerUpdateMessage(explicit)
	message := &types.ChannelMessage{
		ChannelID:   explicit.ChannelID,
		From:        rec.RunID,
		FromAgentID: rec.AgentID,
		FromRunID:   rec.RunID,
		ToAgentID:   explicit.TargetAgentID,
		Role:        explicit.Role,
		Content:     explicit.Content,
		Timestamp:   now,
	}
	storedExplicit, created, err := s.DispatchWorkerUpdate(ctx, explicit, message)
	if err != nil {
		t.Fatalf("dispatch explicit update: %v", err)
	}
	if !created {
		t.Fatal("explicit update unexpectedly already existed")
	}

	rec.State = types.RunCompleted
	rec.Result = "The explicit packet already delivered the terminal evidence."
	rec.UpdatedAt = now.Add(time.Second)
	rec.FinishedAt = &rec.UpdatedAt
	if err := s.UpdateRun(ctx, rec); err != nil {
		t.Fatalf("persist terminal researcher run: %v", err)
	}
	rt.SetDispatchActor(func(context.Context, string, string, string, string, string, string, string) error {
		return nil
	})
	rt.reconcileTerminalRunOutcomes(ctx)
	updates, err := s.ListWorkerUpdatesBySourceRun(ctx, rec.OwnerID, rec.RunID)
	if err != nil {
		t.Fatalf("list explicit bound update: %v", err)
	}
	if len(updates) != 1 {
		t.Fatalf("updates = %d, want original explicit update only: %+v", len(updates), updates)
	}
	bound := updates[0]
	if bound.UpdateID != storedExplicit.UpdateID || bound.MessageSeq != storedExplicit.MessageSeq {
		t.Fatalf("explicit identity changed from %s/%d to %s/%d", storedExplicit.UpdateID, storedExplicit.MessageSeq, bound.UpdateID, bound.MessageSeq)
	}
	wantDigest := types.TerminalRunOutcomeSHA256(rec.RunID, rec.State, rec.Result, rec.Error)
	if bound.SourceRunID != rec.RunID || bound.SourceOutcomeSHA256 != wantDigest {
		t.Fatalf("explicit update binding = run %q digest %q, want %q/%q", bound.SourceRunID, bound.SourceOutcomeSHA256, rec.RunID, wantDigest)
	}
}

func TestDerivedWorkerUpdateIDSurvivesReplaceableSourceRun(t *testing.T) {
	base := types.CoagentSourcePacket{
		OwnerID:       "owner-update-id",
		AgentID:       "researcher:update-id",
		TargetAgentID: "texture:update-id",
		ChannelID:     "doc-update-id",
		Role:          agentprofile.Researcher,
		Packet:        newCoagentPacket("evidence_update", "Same content.", nil, nil, nil, nil, nil),
	}
	first := base
	first.SourceRunID = "run-update-id-first"
	second := base
	second.SourceRunID = "run-update-id-second"

	if firstID, secondID := deriveWorkerUpdateID(first), deriveWorkerUpdateID(second); firstID != secondID {
		t.Fatalf("replaceable producer runs changed durable update ID: %q != %q", firstID, secondID)
	}
}

func TestSuperExplicitFinalUpdateIsBoundInPlace(t *testing.T) {
	ctx := context.Background()
	rt, s := testRuntime(t)
	now := time.Now().UTC()
	rec := types.RunRecord{
		RunID:            "run-super-explicit-terminal",
		AgentID:          "super:explicit-terminal",
		RequestedByRunID: "run-super-explicit-parent",
		ChannelID:        "doc-super-explicit-terminal",
		OwnerID:          "owner-super-explicit-terminal",
		AgentProfile:     agentprofile.Super,
		AgentRole:        agentprofile.Super,
		State:            types.RunRunning,
		CreatedAt:        now,
		UpdatedAt:        now,
		Metadata: map[string]any{
			runMetadataAgentProfile: agentprofile.Super,
			runMetadataAgentRole:    agentprofile.Super,
			runMetadataAgentID:      "super:explicit-terminal",
			runMetadataChannelID:    "doc-super-explicit-terminal",
			runMetadataTrajectoryID: "trajectory-super-explicit-terminal",
			"requested_by_agent_id": "co-super:explicit-terminal",
		},
	}
	if err := s.CreateRun(ctx, rec); err != nil {
		t.Fatalf("create running Super run: %v", err)
	}
	update := types.CoagentSourcePacket{
		OwnerID:       rec.OwnerID,
		SourceRunID:   rec.RunID,
		AgentID:       rec.AgentID,
		TargetAgentID: "co-super:explicit-terminal",
		ChannelID:     rec.ChannelID,
		TrajectoryID:  trajectoryIDForRun(&rec),
		Role:          agentprofile.Super,
		Packet: newCoagentPacket(
			"evidence_update",
			"Explicit final Super update.",
			[]types.CoagentPacketClaim{coagentClaim("Super terminal evidence.")},
			nil,
			nil,
			nil,
			nil,
		),
		CreatedAt: now,
	}
	update.UpdateID = deriveWorkerUpdateID(update)
	update.Content = buildWorkerUpdateMessage(update)
	message := &types.ChannelMessage{
		ChannelID:    update.ChannelID,
		From:         rec.RunID,
		FromAgentID:  rec.AgentID,
		FromRunID:    rec.RunID,
		ToAgentID:    update.TargetAgentID,
		TrajectoryID: update.TrajectoryID,
		Role:         update.Role,
		Content:      update.Content,
		Timestamp:    now,
	}
	storedUpdate, created, err := s.DispatchWorkerUpdate(ctx, update, message)
	if err != nil || !created {
		t.Fatalf("dispatch explicit Super update: created=%t err=%v", created, err)
	}
	output, _ := json.Marshal(map[string]any{
		"update_id":     storedUpdate.UpdateID,
		"agent_id":      storedUpdate.TargetAgentID,
		"channel_id":    storedUpdate.ChannelID,
		"trajectory_id": storedUpdate.TrajectoryID,
		"status":        "submitted",
	})
	eventPayload, _ := json.Marshal(map[string]any{
		"tool":     "update_coagent",
		"call_id":  "call-super-explicit-terminal",
		"is_error": false,
		"output":   string(output),
	})
	if err := s.AppendEvent(ctx, &types.EventRecord{
		EventID:      "event-super-explicit-terminal",
		RunID:        rec.RunID,
		AgentID:      rec.AgentID,
		ChannelID:    rec.ChannelID,
		OwnerID:      rec.OwnerID,
		TrajectoryID: rec.TrajectoryID,
		Timestamp:    now,
		Kind:         types.EventToolResult,
		Payload:      eventPayload,
	}); err != nil {
		t.Fatalf("append explicit Super update event: %v", err)
	}
	rec.State = types.RunCompleted
	rec.Result = "Super completed after its explicit terminal update."
	rec.UpdatedAt = now.Add(time.Second)
	rec.FinishedAt = &rec.UpdatedAt
	if err := s.UpdateRun(ctx, rec); err != nil {
		t.Fatalf("persist terminal Super run: %v", err)
	}
	if err := rt.bindTerminalRunOutcome(ctx, &rec, false); err != nil {
		t.Fatalf("bind explicit Super terminal update: %v", err)
	}
	bound, err := s.GetWorkerUpdate(ctx, rec.OwnerID, update.UpdateID)
	if err != nil {
		t.Fatalf("get bound Super update: %v", err)
	}
	wantDigest := types.TerminalRunOutcomeSHA256(rec.RunID, rec.State, rec.Result, rec.Error)
	if bound.SourceRunID != rec.RunID || bound.SourceOutcomeSHA256 != wantDigest || bound.MessageSeq != storedUpdate.MessageSeq {
		t.Fatalf("bound Super update = %+v, want run %q digest %q seq %d", bound, rec.RunID, wantDigest, storedUpdate.MessageSeq)
	}
}

func TestTerminalOutcomeIgnoresExplicitUpdateToNonRequester(t *testing.T) {
	ctx := context.Background()
	rt, s := testRuntime(t)
	now := time.Now().UTC()
	rec := terminalResearcherRunFixture(
		"run-terminal-nonrequester",
		"owner-terminal-nonrequester",
		"doc-terminal-nonrequester",
		"Authoritative result owed to the requesting Texture.",
		types.RunCompleted,
		now,
	)
	if err := s.CreateRun(ctx, rec); err != nil {
		t.Fatalf("create terminal child run: %v", err)
	}
	unrelated := types.CoagentSourcePacket{
		OwnerID:       rec.OwnerID,
		AgentID:       rec.AgentID,
		TargetAgentID: "co-super:unrelated-recipient",
		ChannelID:     rec.ChannelID,
		TrajectoryID:  trajectoryIDForRun(&rec),
		Role:          agentprofile.Researcher,
		SourceRunID:   rec.RunID,
		Packet:        newCoagentPacket("evidence_update", "Update for another actor.", nil, nil, nil, nil, nil),
		CreatedAt:     now,
	}
	unrelated.UpdateID = deriveWorkerUpdateID(unrelated)
	unrelated.Content = buildWorkerUpdateMessage(unrelated)
	if _, created, err := s.DispatchWorkerUpdate(ctx, unrelated, &types.ChannelMessage{
		ChannelID:    unrelated.ChannelID,
		From:         rec.RunID,
		FromAgentID:  rec.AgentID,
		FromRunID:    rec.RunID,
		ToAgentID:    unrelated.TargetAgentID,
		TrajectoryID: unrelated.TrajectoryID,
		Role:         unrelated.Role,
		Content:      unrelated.Content,
		Timestamp:    now,
	}); err != nil || !created {
		t.Fatalf("dispatch unrelated update: created=%t err=%v", created, err)
	}

	if err := rt.bindTerminalRunOutcome(ctx, &rec, false); err != nil {
		t.Fatalf("bind requester terminal outcome: %v", err)
	}
	requesterUpdates, err := s.ListPendingWorkerUpdates(ctx, rec.OwnerID, "texture:"+rec.ChannelID, 10)
	if err != nil {
		t.Fatalf("list requester terminal outcomes: %v", err)
	}
	if len(requesterUpdates) != 1 || requesterUpdates[0].TargetAgentID != "texture:"+rec.ChannelID {
		t.Fatalf("requester outcomes = %+v, want exactly one addressed synthetic outcome", requesterUpdates)
	}
	if requesterUpdates[0].SourceOutcomeSHA256 == "" {
		t.Fatalf("requester outcome lacks terminal digest: %+v", requesterUpdates[0])
	}
	storedUnrelated, err := s.GetWorkerUpdate(ctx, rec.OwnerID, unrelated.UpdateID)
	if err != nil {
		t.Fatalf("reload unrelated update: %v", err)
	}
	if storedUnrelated.SourceOutcomeSHA256 != "" {
		t.Fatalf("non-requester update was bound as terminal outcome: %+v", storedUnrelated)
	}
}

func TestBootReconciliationIsIdempotentForTerminalResearcherOutcome(t *testing.T) {
	ctx := context.Background()
	rt, s := testRuntime(t)
	now := time.Now().UTC()
	rec := terminalResearcherRunFixture(
		"run-researcher-restart-outcome",
		"owner-researcher-restart-outcome",
		"doc-researcher-restart-outcome",
		"Durable result written before the simulated restart gap.",
		types.RunCompleted,
		now,
	)
	if err := s.CreateRun(ctx, rec); err != nil {
		t.Fatalf("create terminal researcher restart fixture: %v", err)
	}
	rt.SetDispatchActor(func(context.Context, string, string, string, string, string, string, string) error {
		return nil
	})

	rt.reconcileTerminalRunOutcomes(ctx)
	first, err := s.ListPendingWorkerUpdates(ctx, rec.OwnerID, "texture:"+rec.ChannelID, 10)
	if err != nil {
		t.Fatalf("list first restart repair: %v", err)
	}
	if len(first) != 1 {
		t.Fatalf("first restart repair updates = %d, want one: %+v", len(first), first)
	}
	rt.reconcileTerminalRunOutcomes(ctx)
	second, err := s.ListPendingWorkerUpdates(ctx, rec.OwnerID, "texture:"+rec.ChannelID, 10)
	if err != nil {
		t.Fatalf("list replayed restart repair: %v", err)
	}
	if len(second) != 1 {
		t.Fatalf("replayed restart repair updates = %d, want one: %+v", len(second), second)
	}
	if second[0].UpdateID != first[0].UpdateID || second[0].MessageSeq != first[0].MessageSeq {
		t.Fatalf("restart replay changed update identity from %s/%d to %s/%d", first[0].UpdateID, first[0].MessageSeq, second[0].UpdateID, second[0].MessageSeq)
	}
	wantDigest := types.TerminalRunOutcomeSHA256(rec.RunID, rec.State, rec.Result, rec.Error)
	if second[0].SourceRunID != rec.RunID || second[0].SourceOutcomeSHA256 != wantDigest {
		t.Fatalf("restart binding = run %q digest %q, want %q/%q", second[0].SourceRunID, second[0].SourceOutcomeSHA256, rec.RunID, wantDigest)
	}
}

func TestBootTerminalRepairSynthesizesGenericChildrenAndWakesTargetOnce(t *testing.T) {
	ctx := context.Background()
	rt, s := testRuntime(t)
	now := time.Now().UTC()
	ownerID := "owner-generic-terminal-repair"
	targetAgentID := "super:generic-terminal-parent"
	channelID := "channel-generic-terminal-repair"
	for i := range 2 {
		runID := "run-cosuper-generic-terminal-" + string(rune('A'+i))
		finishedAt := now.Add(time.Duration(i) * time.Second)
		rec := types.RunRecord{
			RunID:            runID,
			AgentID:          "co-super:" + runID,
			RequestedByRunID: "parent-generic-terminal",
			ChannelID:        channelID,
			OwnerID:          ownerID,
			AgentProfile:     agentprofile.CoSuper,
			AgentRole:        agentprofile.CoSuper,
			State:            types.RunCompleted,
			Result:           "generic delegated child result",
			CreatedAt:        now,
			UpdatedAt:        finishedAt,
			FinishedAt:       &finishedAt,
			Metadata: map[string]any{
				runMetadataAgentProfile: agentprofile.CoSuper,
				runMetadataAgentRole:    agentprofile.CoSuper,
				runMetadataAgentID:      "co-super:" + runID,
				runMetadataChannelID:    channelID,
				runMetadataTrajectoryID: "trajectory-" + runID,
				"requested_by_agent_id": targetAgentID,
			},
		}
		if err := s.CreateRun(ctx, rec); err != nil {
			t.Fatalf("create generic terminal child %d: %v", i, err)
		}
	}
	var wakes atomic.Int32
	rt.SetDispatchActor(func(context.Context, string, string, string, string, string, string, string) error {
		wakes.Add(1)
		return nil
	})
	rt.reconcileTerminalRunOutcomes(ctx)
	if got := wakes.Load(); got != 1 {
		t.Fatalf("distinct repaired target wakes = %d, want one", got)
	}
	updates, err := s.ListPendingWorkerUpdates(ctx, ownerID, targetAgentID, 10)
	if err != nil {
		t.Fatalf("list generic terminal repairs: %v", err)
	}
	if len(updates) != 2 {
		t.Fatalf("generic terminal repairs = %d, want two: %+v", len(updates), updates)
	}
	for _, update := range updates {
		if update.SourceRunID == "" || update.SourceOutcomeSHA256 == "" {
			t.Fatalf("generic terminal repair lacks outcome binding: %+v", update)
		}
	}
}

func TestExecuteToolsParallel(t *testing.T) {
	registry := toolregistry.NewToolRegistry()

	slowTool := toolregistry.Tool{Name: "slow",
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			return "slow-result", nil
		}}
	fastTool := toolregistry.Tool{Name: "fast",
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			return "fast-result", nil
		}}
	if err := registry.Register(slowTool); err != nil {
		t.Fatalf("register slow: %v", err)
	}
	if err := registry.Register(fastTool); err != nil {
		t.Fatalf("register fast: %v", err)
	}

	calls := []types.ToolCall{
		{ID: "call-1", Name: "slow", Arguments: json.RawMessage(`{}`)},
		{ID: "call-2", Name: "fast", Arguments: json.RawMessage(`{}`)},
	}

	emit := func(kind types.EventKind, phase string, payload json.RawMessage) {}

	results := toolregistry.ExecuteToolBatch(context.Background(), registry, calls, emit)

	// Results should be in the same order as the calls.
	if results[0].CallID != "call-1" {
		t.Errorf("result[0] call_id: got %q, want call-1", results[0].CallID)
	}
	if results[0].Output != "slow-result" {
		t.Errorf("result[0] output: got %q, want slow-result", results[0].Output)
	}
	if results[1].CallID != "call-2" {
		t.Errorf("result[1] call_id: got %q, want call-2", results[1].CallID)
	}
	if results[1].Output != "fast-result" {
		t.Errorf("result[1] output: got %q, want fast-result", results[1].Output)
	}
}

func TestExecuteToolsSerializesHeavySideEffectTurns(t *testing.T) {
	registry := toolregistry.NewToolRegistry()
	var mu sync.Mutex
	var order []string
	register := func(name string) {
		t.Helper()
		if err := registry.Register(toolregistry.Tool{Name: name,
			Func: func(ctx context.Context, args json.RawMessage) (string, error) {
				mu.Lock()
				order = append(order, "start:"+name)
				mu.Unlock()
				time.Sleep(10 * time.Millisecond)
				mu.Lock()
				order = append(order, "end:"+name)
				mu.Unlock()
				return name + "-result", nil
			}}); err != nil {
			t.Fatalf("register %s: %v", name, err)
		}
	}
	register("bash")
	register("read_file")

	results := toolregistry.ExecuteToolBatch(context.Background(), registry, []types.ToolCall{
		{ID: "call-1", Name: "bash", Arguments: json.RawMessage(`{}`)},
		{ID: "call-2", Name: "read_file", Arguments: json.RawMessage(`{}`)},
	}, func(kind types.EventKind, phase string, payload json.RawMessage) {})

	if results[0].Output != "bash-result" || results[1].Output != "read_file-result" {
		t.Fatalf("results = %+v, want both tool outputs", results)
	}
	want := []string{"start:bash", "end:bash", "start:read_file", "end:read_file"}
	if strings.Join(order, ",") != strings.Join(want, ",") {
		t.Fatalf("execution order = %v, want %v", order, want)
	}
}

func TestExecuteToolsError(t *testing.T) {
	registry := toolregistry.NewToolRegistry()

	failTool := toolregistry.Tool{Name: "fail",
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			return "", fmt.Errorf("tool failure")
		}}
	if err := registry.Register(failTool); err != nil {
		t.Fatalf("register: %v", err)
	}

	calls := []types.ToolCall{
		{ID: "call-1", Name: "fail", Arguments: json.RawMessage(`{}`)},
	}

	emit := func(kind types.EventKind, phase string, payload json.RawMessage) {}

	results := toolregistry.ExecuteToolBatch(context.Background(), registry, calls, emit)

	if !results[0].IsError {
		t.Error("expected error result")
	}
	if results[0].Output == "" {
		t.Error("error output should contain error message")
	}
}

func TestExecuteToolsOutputTruncation(t *testing.T) {
	registry := toolregistry.NewToolRegistry()

	bigTool := toolregistry.Tool{Name: "big_output",
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			// Return output larger than 100KB.
			result := make([]byte, 150*1024)
			for i := range result {
				result[i] = 'x'
			}
			return string(result), nil
		}}
	if err := registry.Register(bigTool); err != nil {
		t.Fatalf("register: %v", err)
	}

	calls := []types.ToolCall{
		{ID: "call-1", Name: "big_output", Arguments: json.RawMessage(`{}`)},
	}

	emit := func(kind types.EventKind, phase string, payload json.RawMessage) {}

	results := toolregistry.ExecuteToolBatch(context.Background(), registry, calls, emit)

	// Output should be truncated to ~100KB + truncation notice.
	if len(results[0].Output) > 110*1024 {
		t.Errorf("output should be truncated, got %d bytes", len(results[0].Output))
	}
}

func TestExecuteToolsConductorTextureRouteSkipsOtherSpawn(t *testing.T) {
	registry := toolregistry.NewToolRegistry()
	executed := []string{}
	if err := registry.Register(toolregistry.Tool{Name: "spawn_agent",
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			var in struct {
				Role string `json:"role"`
			}
			if err := json.Unmarshal(args, &in); err != nil {
				return "", err
			}
			executed = append(executed, in.Role)
			return in.Role, nil
		}}); err != nil {
		t.Fatalf("register: %v", err)
	}

	ctx := toolregistry.WithExecutionContext(context.Background(), toolExecutionContextForRun(&types.RunRecord{
		RunID:        "conductor-run",
		OwnerID:      "owner",
		AgentProfile: agentprofile.Conductor,
	}))
	calls := []types.ToolCall{
		{ID: "research", Name: "spawn_agent", Arguments: json.RawMessage(`{"role":"researcher","objective":"research"}`)},
		{ID: "texture", Name: "spawn_agent", Arguments: json.RawMessage(`{"role":"texture","objective":"open document","initial_content":"# Draft"}`)},
	}

	results := toolregistry.ExecuteToolBatch(ctx, registry, calls, func(kind types.EventKind, phase string, payload json.RawMessage) {})

	if len(executed) != 1 || executed[0] != agentprofile.Texture {
		t.Fatalf("executed = %#v, want only texture", executed)
	}
	if results[0].IsError || !strings.Contains(results[0].Output, "texture owns downstream") {
		t.Fatalf("research spawn result = %#v, want non-error skipped downstream worker notice", results[0])
	}
	if results[1].IsError || results[1].Output != agentprofile.Texture {
		t.Fatalf("texture spawn result = %#v, want success", results[1])
	}
}

func TestExecuteToolsSuperSkipsDuplicateCoordinationSideEffects(t *testing.T) {
	registry := toolregistry.NewToolRegistry()
	var mu sync.Mutex
	counts := map[string]int{}
	registerCountingTool := func(name string) {
		t.Helper()
		if err := registry.Register(toolregistry.Tool{Name: name,
			Func: func(ctx context.Context, args json.RawMessage) (string, error) {
				mu.Lock()
				counts[name]++
				mu.Unlock()
				return name + " ok", nil
			}}); err != nil {
			t.Fatalf("register %s: %v", name, err)
		}
	}
	registerCountingTool("spawn_agent")
	registerCountingTool("update_coagent")

	ctx := toolregistry.WithExecutionContext(context.Background(), toolExecutionContextForRun(&types.RunRecord{
		RunID:        "super-run",
		OwnerID:      "owner",
		AgentProfile: agentprofile.Super,
	}))
	calls := []types.ToolCall{
		{ID: "spawn-implementation-1", Name: "spawn_agent", Arguments: json.RawMessage(`{"role":"co-super","slot":"implementation","channel_id":"doc-1","objective":"implement"}`)},
		{ID: "spawn-implementation-2", Name: "spawn_agent", Arguments: json.RawMessage(`{"role":"co-super","slot":"implementation","channel_id":"doc-1","objective":"implement again"}`)},
		{ID: "spawn-verifier", Name: "spawn_agent", Arguments: json.RawMessage(`{"role":"co-super","slot":"verifier","channel_id":"doc-1","objective":"verify"}`)},
		{ID: "cast-1", Name: "update_coagent", Arguments: json.RawMessage(`{"schema_version":"coagent_source_packet.v1","kind":"evidence_update","summary":"proceed with exact evidence","agent_id":"agent-impl","claims":[{"text":"proceed with exact evidence"}]}`)},
		{ID: "cast-2", Name: "update_coagent", Arguments: json.RawMessage(`{"schema_version":"coagent_source_packet.v1","kind":"evidence_update","summary":"proceed with exact evidence","agent_id":"agent-impl","claims":[{"text":"proceed with exact evidence"}]}`)},
	}

	results := toolregistry.ExecuteToolBatch(ctx, registry, calls, func(kind types.EventKind, phase string, payload json.RawMessage) {})

	mu.Lock()
	gotCounts := map[string]int{
		"spawn_agent":    counts["spawn_agent"],
		"update_coagent": counts["update_coagent"],
	}
	mu.Unlock()
	if gotCounts["spawn_agent"] != 2 || gotCounts["update_coagent"] != 1 {
		t.Fatalf("executed counts = %+v, want spawn=2 cast=1", gotCounts)
	}
	for _, idx := range []int{1, 4} {
		if !results[idx].IsError || !strings.Contains(results[idx].Output, "duplicate") {
			t.Fatalf("result[%d] = %#v, want duplicate skip error", idx, results[idx])
		}
	}
	for _, idx := range []int{0, 2, 3} {
		if results[idx].IsError {
			t.Fatalf("result[%d] = %#v, want successful execution", idx, results[idx])
		}
	}
}

func TestExecuteToolsCoSuperSkipsDuplicateBashCommand(t *testing.T) {
	registry := toolregistry.NewToolRegistry()
	var mu sync.Mutex
	executions := 0
	if err := registry.Register(toolregistry.Tool{Name: "bash",
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			mu.Lock()
			executions++
			mu.Unlock()
			return "bash ok", nil
		}}); err != nil {
		t.Fatalf("register bash: %v", err)
	}
	ctx := toolregistry.WithExecutionContext(context.Background(), toolExecutionContextForRun(&types.RunRecord{
		RunID:        "cosuper-run",
		OwnerID:      "owner",
		AgentProfile: agentprofile.CoSuper,
	}))
	calls := []types.ToolCall{
		{ID: "bash-1", Name: "bash", Arguments: json.RawMessage(`{"command":"go test ./internal/platform","timeout_ms":60000}`)},
		{ID: "bash-2", Name: "bash", Arguments: json.RawMessage(`{"command":"go test ./internal/platform","timeout_ms":60000}`)},
	}

	results := toolregistry.ExecuteToolBatch(ctx, registry, calls, func(kind types.EventKind, phase string, payload json.RawMessage) {})

	mu.Lock()
	gotExecutions := executions
	mu.Unlock()
	if gotExecutions != 1 {
		t.Fatalf("bash executions = %d, want one", gotExecutions)
	}
	if results[0].IsError {
		t.Fatalf("first bash = %#v, want success", results[0])
	}
	if !results[1].IsError || !strings.Contains(results[1].Output, "duplicate bash command") {
		t.Fatalf("second bash = %#v, want duplicate skip", results[1])
	}
}

func envValue(env []string, key string) string {
	prefix := key + "="
	for _, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			return strings.TrimPrefix(entry, prefix)
		}
	}
	return ""
}

func TestExecuteToolsDoesNotCapResearcherSearchBatch(t *testing.T) {
	registry := toolregistry.NewToolRegistry()
	var searches int32
	if err := registry.Register(toolregistry.Tool{Name: "web_search",
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			atomic.AddInt32(&searches, 1)
			return "search result", nil
		}}); err != nil {
		t.Fatalf("register web_search: %v", err)
	}
	if err := registry.Register(toolregistry.Tool{Name: "fetch_url",
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			return "fetch result", nil
		}}); err != nil {
		t.Fatalf("register fetch_url: %v", err)
	}

	ctx := toolregistry.WithExecutionContext(context.Background(), toolExecutionContextForRun(&types.RunRecord{
		RunID:        "researcher-run",
		OwnerID:      "owner",
		AgentProfile: agentprofile.Researcher,
	}))
	calls := []types.ToolCall{
		{ID: "search-1", Name: "web_search", Arguments: json.RawMessage(`{"query":"ai news may 2026"}`)},
		{ID: "search-2", Name: "web_search", Arguments: json.RawMessage(`{"query":"openai may 2026"}`)},
		{ID: "search-3", Name: "web_search", Arguments: json.RawMessage(`{"query":"google ai may 2026"}`)},
	}

	results := toolregistry.ExecuteToolBatch(ctx, registry, calls, func(kind types.EventKind, phase string, payload json.RawMessage) {})

	if got := atomic.LoadInt32(&searches); got != 3 {
		t.Fatalf("searches = %d, want 3", got)
	}
	for idx, result := range results {
		if result.IsError {
			t.Fatalf("result[%d] = %#v, want success", idx, result)
		}
	}
}
