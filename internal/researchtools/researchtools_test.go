package researchtools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/sourceapi"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
)

func TestRegisterInstallsResearchToolsWithCanonicalSchemas(t *testing.T) {
	registry := toolregistry.NewToolRegistry()
	if err := Register(registry, Dependencies{}); err != nil {
		t.Fatalf("register research tools: %v", err)
	}

	wantNames := []string{
		"read",
		"search_wire_corpus",
		"source_search",
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

	read, _ := registry.Lookup("read")
	if !reflect.DeepEqual(read.Parameters["required"], []string{"url"}) {
		t.Fatalf("read required = %#v", read.Parameters["required"])
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
