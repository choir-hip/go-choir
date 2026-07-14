package researchtools

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/search"
	"github.com/yusefmosiah/go-choir/internal/sourceapi"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestRegisterInstallsNineResearchToolsWithCanonicalSchemas(t *testing.T) {
	registry := toolregistry.NewToolRegistry()
	if err := Register(registry, Dependencies{}); err != nil {
		t.Fatalf("register research tools: %v", err)
	}

	wantNames := []string{
		"fetch_url",
		"import_document_content",
		"import_url_content",
		"list_content_item_selectors",
		"read_content_item",
		"read_content_item_selector",
		"search_wire_corpus",
		"source_search",
		"web_search",
	}
	tools := registry.Tools()
	gotNames := make([]string, 0, len(tools))
	for _, tool := range tools {
		gotNames = append(gotNames, tool.Name)
		if strings.TrimSpace(tool.Description) == "" {
			t.Fatalf("tool %q has empty description", tool.Name)
		}
	}
	if !reflect.DeepEqual(gotNames, wantNames) {
		t.Fatalf("registered tools = %v, want %v", gotNames, wantNames)
	}

	web, _ := registry.Lookup("web_search")
	webProperties := web.Parameters["properties"].(map[string]any)
	maxResults := webProperties["max_results"].(map[string]any)
	if maxResults["minimum"] != 1 || maxResults["maximum"] != 50 {
		t.Fatalf("web_search max_results schema = %#v", maxResults)
	}
	if !reflect.DeepEqual(web.Parameters["required"], []string{"query"}) {
		t.Fatalf("web_search required = %#v", web.Parameters["required"])
	}

	readItem, _ := registry.Lookup("read_content_item")
	readProperties := readItem.Parameters["properties"].(map[string]any)
	if readProperties["max_text_chars"].(map[string]any)["maximum"] != 100000 {
		t.Fatalf("read_content_item max_text_chars schema = %#v", readProperties["max_text_chars"])
	}
	if readProperties["max_segments"].(map[string]any)["maximum"] != 1000 {
		t.Fatalf("read_content_item max_segments schema = %#v", readProperties["max_segments"])
	}

	if err := Register(registry, Dependencies{}); err == nil || !strings.Contains(err.Error(), "already registered") {
		t.Fatalf("duplicate Register error = %v", err)
	}
}

func TestHTTPSourceClientSearchResolveAndHandoff(t *testing.T) {
	item := sourceapi.ItemResult{
		Rank:            1,
		TargetKind:      sourceapi.TargetKind,
		ItemID:          "srcitem_rates",
		SourceID:        "official:rates",
		SourceType:      "rss",
		FetchID:         "fetch_rates",
		Title:           "Rate decision",
		Body:            "Rates held steady.",
		URL:             "https://example.test/rates",
		CanonicalURL:    "https://example.test/rates",
		PublishedAt:     "2026-06-04T12:00:00Z",
		FetchedAt:       "2026-06-04T12:01:00Z",
		ContentHash:     "sha256-rates",
		BodyKind:        "reader_snapshot",
		BodyLength:      len("Rates held steady."),
		ReaderSnapshot:  true,
		EvidenceLevel:   "official_release",
		VintagePolicy:   "release_snapshot",
		LookaheadStatus: "no_lookahead",
	}
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.RequestURI())
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/internal/source-service/search":
			if r.URL.Query().Get("q") != "rates" || r.URL.Query().Get("max_results") != "5" {
				t.Fatalf("source search query = %q", r.URL.RawQuery)
			}
			_ = json.NewEncoder(w).Encode(sourceapi.SearchResponse{
				Query: "rates", Results: []sourceapi.ItemResult{item},
				Metadata: sourceapi.Metadata{TargetKind: sourceapi.TargetKind},
			})
		case "/internal/source-service/items/srcitem_rates":
			_ = json.NewEncoder(w).Encode(sourceapi.ResolveItemResponse{Item: item})
		case "/internal/source-service/ingestion-handoff/latest":
			_ = json.NewEncoder(w).Encode(sourceapi.IngestionHandoffResponse{})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewSourceClient(SourceClientConfig{BaseURL: server.URL + "/", HTTPClient: server.Client()})
	if client == nil {
		t.Fatal("NewSourceClient returned nil")
	}
	t.Setenv("SOURCE_SERVICE_BASE_URL", server.URL)
	t.Setenv("SOURCE_SERVICE_URL", "")
	t.Setenv("SOURCECYCLED_API_URL", "")
	if fromEnv := NewSourceClientFromEnv(); fromEnv == nil || fromEnv.baseURL != server.URL {
		t.Fatalf("NewSourceClientFromEnv = %#v", fromEnv)
	}
	response, err := client.SearchSources(context.Background(), " rates ", 5)
	if err != nil {
		t.Fatalf("SearchSources: %v", err)
	}
	if response.Provider != sourceapi.ProviderName || response.BaseURL != server.URL || len(response.Results) != 1 {
		t.Fatalf("source response = %#v", response)
	}
	got := response.Results[0]
	if got["item_id"] != item.ItemID || got["content_hash"] != item.ContentHash || got["evidence_level"] != item.EvidenceLevel {
		t.Fatalf("source item projection = %#v", got)
	}
	if response.Metadata["target_kind"] != sourceapi.TargetKind {
		t.Fatalf("source metadata = %#v", response.Metadata)
	}

	resolved, err := client.ResolveSourceItem(context.Background(), item.ItemID)
	if err != nil || resolved.ItemID != item.ItemID {
		t.Fatalf("ResolveSourceItem = %+v, %v", resolved, err)
	}
	if _, err := client.IngestionHandoffLatest(context.Background()); err != nil {
		t.Fatalf("IngestionHandoffLatest: %v", err)
	}

	registry := toolregistry.NewToolRegistry()
	if err := Register(registry, Dependencies{Source: client}); err != nil {
		t.Fatalf("register source tool: %v", err)
	}
	raw, err := registry.Execute(context.Background(), "source_search", json.RawMessage(`{"query":"rates","max_results":5}`))
	if err != nil {
		t.Fatalf("source_search: %v", err)
	}
	var envelope struct {
		ModelOutput struct {
			SourceIdentity string `json:"source_identity"`
			Results        []struct {
				ItemID        string `json:"item_id"`
				SourceID      string `json:"source_id"`
				FetchID       string `json:"fetch_id"`
				ContentHash   string `json:"content_hash"`
				EvidenceLevel string `json:"evidence_level"`
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
		t.Fatalf("decode source_search projection: %v", err)
	}
	if len(envelope.ModelOutput.Results) != 1 {
		t.Fatalf("source_search model results = %#v", envelope.ModelOutput)
	}
	projected := envelope.ModelOutput.Results[0]
	if projected.ItemID != item.ItemID || projected.SourceID != item.SourceID || projected.FetchID != item.FetchID ||
		projected.ContentHash != item.ContentHash || projected.EvidenceLevel != item.EvidenceLevel {
		t.Fatalf("source_search identity projection = %#v", projected)
	}
	if !strings.Contains(envelope.ModelOutput.SourceIdentity, "source_service_item") ||
		len(envelope.DurableOutput.Results) != 1 || envelope.Projection.Type != "source_search" {
		t.Fatalf("source_search envelope = %#v", envelope)
	}
	if len(paths) != 4 {
		t.Fatalf("source request paths = %v", paths)
	}
}

func TestHTTPSourceClientPreservesServiceErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "ledger unavailable", http.StatusBadGateway)
	}))
	defer server.Close()

	client := NewSourceClient(SourceClientConfig{BaseURL: server.URL, HTTPClient: server.Client()})
	_, err := client.SearchSources(context.Background(), "rates", 1)
	if err == nil || !strings.Contains(err.Error(), "source service search returned 502 Bad Gateway: ledger unavailable") {
		t.Fatalf("SearchSources service error = %v", err)
	}
	if NewSourceClient(SourceClientConfig{BaseURL: "://bad"}) != nil {
		t.Fatal("invalid source URL should not configure a client")
	}
}

type recordingSearchClient struct {
	query      string
	maxResults int
	response   *search.Response
	err        error
}

func (c *recordingSearchClient) Search(_ context.Context, query string, maxResults int) (*search.Response, error) {
	c.query = query
	c.maxResults = maxResults
	return c.response, c.err
}

func TestWebSearchPreservesBreadthProjectionAndGatewayOutage(t *testing.T) {
	client := &recordingSearchClient{response: &search.Response{
		Query: "current releases", Provider: "gateway", Providers: []string{"brave"},
		Results: []map[string]any{{
			"title": "Release", "url": "https://example.test/release",
			"snippet": strings.Repeat("evidence ", 100), "provider": "brave",
		}},
	}}
	registry := toolregistry.NewToolRegistry()
	if err := Register(registry, Dependencies{Search: client}); err != nil {
		t.Fatalf("register: %v", err)
	}
	raw, err := registry.Execute(context.Background(), "web_search", json.RawMessage(`{"query":" current releases ","max_results":5}`))
	if err != nil {
		t.Fatalf("web_search: %v", err)
	}
	if client.query != "current releases" || client.maxResults != webSearchAgentResultFloor {
		t.Fatalf("search call = query %q max %d", client.query, client.maxResults)
	}
	var envelope struct {
		ModelOutput struct {
			Results []struct {
				Snippet string `json:"snippet"`
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
		t.Fatalf("decode web projection: %v", err)
	}
	if len(envelope.ModelOutput.Results) != 1 || len(envelope.ModelOutput.Results[0].Snippet) != 403 || !strings.HasSuffix(envelope.ModelOutput.Results[0].Snippet, "...") {
		t.Fatalf("model projection = %#v", envelope.ModelOutput)
	}
	if len(envelope.DurableOutput.Results) != 1 || envelope.Projection.Type != "web_search" {
		t.Fatalf("durable projection = %#v / %#v", envelope.DurableOutput, envelope.Projection)
	}

	client.response = &search.Response{
		Query: "outage", Outage: true, Code: "search_outage", Error: "search_outage",
		Degraded:       true,
		ProviderHealth: map[string]any{"brave": map[string]any{"state": "cooling_down"}},
		Attempts:       []map[string]any{{"provider": "brave", "status": "rate_limited", "error": "429"}},
	}
	raw, err = registry.Execute(context.Background(), "web_search", json.RawMessage(`{"query":"outage"}`))
	if err != nil {
		t.Fatalf("outage web_search: %v", err)
	}
	var outage struct {
		ModelOutput map[string]any `json:"model_output"`
		Projection  map[string]any `json:"projection"`
	}
	if err := json.Unmarshal([]byte(raw), &outage); err != nil {
		t.Fatalf("decode outage projection: %v", err)
	}
	if outage.ModelOutput["search_outage"] != true || outage.ModelOutput["code"] != "search_outage" {
		t.Fatalf("outage model output = %#v", outage.ModelOutput)
	}
	if !strings.Contains(outage.ModelOutput["next_instruction"].(string), "precise blocker") || outage.Projection["search_outage"] != true {
		t.Fatalf("outage guidance/projection = %#v / %#v", outage.ModelOutput, outage.Projection)
	}

	client.err = errors.New("gateway transport failed")
	client.response = nil
	if _, err := registry.Execute(context.Background(), "web_search", json.RawMessage(`{"query":"failure"}`)); err == nil || err.Error() != "gateway transport failed" {
		t.Fatalf("gateway error = %v", err)
	}
}

func TestContentReadsPreserveBoundsSelectorsProvenanceAndCheckpointPressure(t *testing.T) {
	st := openTestStore(t)
	metadata, _ := json.Marshal(map[string]any{
		"segments": []any{map[string]any{"text": "first"}, map[string]any{"text": "second"}},
		"selectors": []any{
			map[string]any{"id": "page-1", "kind": "page", "label": "Page 1", "text": "alpha evidence"},
			map[string]any{"id": "page-2", "kind": "page", "label": "Page 2", "text": "beta evidence"},
		},
	})
	provenance := json.RawMessage(`{"source":"fixture","trusted":false}`)
	now := time.Now().UTC()
	item := types.ContentItem{
		ContentID: "content-1", OwnerID: "owner-1", SourceType: "document",
		MediaType: "application/pdf", AppHint: "document", Title: "Evidence",
		SourceURL: "https://example.test/evidence.pdf", CanonicalURL: "https://example.test/evidence.pdf",
		TextContent: "first segment and second segment", ContentHash: "sha256-content",
		Metadata: metadata, Provenance: provenance, CreatedAt: now, UpdatedAt: now,
	}
	if err := st.CreateContentItem(context.Background(), item); err != nil {
		t.Fatalf("create content item: %v", err)
	}

	registry := toolregistry.NewToolRegistry()
	if err := Register(registry, Dependencies{Store: st}); err != nil {
		t.Fatalf("register: %v", err)
	}
	toolCtx := toolregistry.WithExecutionContext(context.Background(), toolregistry.ExecutionContext{
		RunID: "research-run", OwnerID: item.OwnerID, Profile: agentprofile.Researcher,
	})
	raw, err := registry.Execute(toolCtx, "read_content_item", json.RawMessage(`{"content_id":"content-1","max_text_chars":12,"max_segments":1}`))
	if err != nil {
		t.Fatalf("read_content_item: %v", err)
	}
	var readResult map[string]any
	if err := json.Unmarshal([]byte(raw), &readResult); err != nil {
		t.Fatalf("decode read result: %v", err)
	}
	if readResult["text_content"] != "first segmen" || readResult["text_truncated"] != true {
		t.Fatalf("bounded text = %#v", readResult)
	}
	if readResult["segment_count"] != float64(2) || len(readResult["segments"].([]any)) != 1 || readResult["segments_truncated"] != true {
		t.Fatalf("bounded segments = %#v", readResult)
	}
	if readResult["provenance"].(map[string]any)["source"] != "fixture" {
		t.Fatalf("provenance = %#v", readResult["provenance"])
	}
	if !strings.Contains(readResult["next_instruction"].(string), "update_coagent") {
		t.Fatalf("initial checkpoint instruction = %#v", readResult["next_instruction"])
	}

	appendToolResult(t, st, "research-run", item.OwnerID, "read_content_item", 1)
	raw, err = registry.Execute(toolCtx, "list_content_item_selectors", json.RawMessage(`{"content_id":"content-1"}`))
	if err != nil {
		t.Fatalf("list_content_item_selectors: %v", err)
	}
	var listResult map[string]any
	_ = json.Unmarshal([]byte(raw), &listResult)
	if listResult["selector_count"] != float64(2) || listResult["next_instruction"] != nil {
		t.Fatalf("selector list/checkpoint = %#v", listResult)
	}
	appendToolResult(t, st, "research-run", item.OwnerID, "update_coagent", 2)

	raw, err = registry.Execute(toolCtx, "read_content_item_selector", json.RawMessage(`{"content_id":"content-1","selector_id":"page-2","max_text_chars":4}`))
	if err != nil {
		t.Fatalf("read_content_item_selector: %v", err)
	}
	var selectorResult map[string]any
	_ = json.Unmarshal([]byte(raw), &selectorResult)
	if selectorResult["selector_kind"] != "page" || selectorResult["text_content"] != "beta" || selectorResult["text_truncated"] != true {
		t.Fatalf("selector result = %#v", selectorResult)
	}
	if !strings.Contains(selectorResult["next_instruction"].(string), "update_coagent") {
		t.Fatalf("post-checkpoint instruction = %#v", selectorResult["next_instruction"])
	}
}

func TestFetchURLPreservesHTTPMetadataAndBoundedProjection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(strings.Repeat("abcdef", 1000)))
	}))
	defer server.Close()

	registry := toolregistry.NewToolRegistry()
	if err := Register(registry, Dependencies{HTTP: server.Client()}); err != nil {
		t.Fatalf("register: %v", err)
	}
	raw, err := registry.Execute(context.Background(), "fetch_url", json.RawMessage(`{"url":"`+server.URL+`","max_chars":5000}`))
	if err != nil {
		t.Fatalf("fetch_url: %v", err)
	}
	var envelope struct {
		ModelOutput struct {
			StatusCode   int    `json:"status_code"`
			Content      string `json:"content"`
			VisibleChars int    `json:"visible_chars"`
			OmittedChars int    `json:"omitted_chars"`
		} `json:"model_output"`
		DurableOutput struct {
			Content       string `json:"content"`
			ContentLength int    `json:"content_length"`
		} `json:"durable_output"`
	}
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
		t.Fatalf("decode fetch projection: %v", err)
	}
	if envelope.ModelOutput.StatusCode != http.StatusAccepted || envelope.ModelOutput.VisibleChars != 4003 || envelope.ModelOutput.OmittedChars != 997 {
		t.Fatalf("fetch model output = %#v", envelope.ModelOutput)
	}
	if len(envelope.ModelOutput.Content) != 4003 || !strings.HasSuffix(envelope.ModelOutput.Content, "...") || len(envelope.DurableOutput.Content) != 5000 || envelope.DurableOutput.ContentLength != 6000 {
		t.Fatalf("fetch durable/model bounds = model %d durable %d length %d", len(envelope.ModelOutput.Content), len(envelope.DurableOutput.Content), envelope.DurableOutput.ContentLength)
	}
}

func openTestStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "researchtools.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func appendToolResult(t *testing.T, st *store.Store, runID, ownerID, tool string, suffix int) {
	t.Helper()
	payload, err := json.Marshal(map[string]any{"tool": tool, "is_error": false})
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	if err := st.AppendEvent(context.Background(), &types.EventRecord{
		EventID: "event-" + tool + "-" + string(rune('0'+suffix)), RunID: runID, OwnerID: ownerID,
		Timestamp: time.Now().UTC(), Kind: types.EventToolResult, Phase: "tool_call", Payload: payload,
	}); err != nil {
		t.Fatalf("append tool result: %v", err)
	}
}
