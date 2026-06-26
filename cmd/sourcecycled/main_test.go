package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/cycle"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/sourceapi"
	"github.com/yusefmosiah/go-choir/internal/sourcefetch"
	"github.com/yusefmosiah/go-choir/internal/sources"
)

func TestUniversalWireSourceRegistryConfigKeepsBroadUntieredCoverage(t *testing.T) {
	path := filepath.Join("..", "..", "configs", "sources.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read source registry config: %v", err)
	}
	var raw struct {
		Sources []map[string]any `json:"sources"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("decode raw source registry config: %v", err)
	}
	var registry sources.Registry
	if err := json.Unmarshal(data, &registry); err != nil {
		t.Fatalf("decode source registry config: %v", err)
	}
	if len(registry.Sources) < 200 {
		t.Fatalf("source registry has %d sources, want broad coverage >= 200", len(registry.Sources))
	}

	seen := map[string]bool{}
	byType := map[sources.SourceType]int{}
	verticals := map[string]int{}
	languages := map[string]int{}
	regions := map[string]int{}
	for i, source := range registry.Sources {
		if strings.TrimSpace(source.ID) == "" {
			t.Fatalf("source %d has empty id", i)
		}
		if seen[source.ID] {
			t.Fatalf("duplicate source id %q", source.ID)
		}
		seen[source.ID] = true
		byType[source.Type]++
		for _, vertical := range source.Verticals {
			verticals[vertical]++
		}
		for _, language := range source.Languages {
			languages[language]++
		}
		for _, region := range source.Regions {
			regions[region]++
		}
		if strings.TrimSpace(source.Tier) != "" || strings.TrimSpace(source.SourceStanding) != "" {
			t.Fatalf("source %s hardcodes source trust tier/standing: tier=%q standing=%q", source.ID, source.Tier, source.SourceStanding)
		}
		if _, ok := raw.Sources[i]["tier"]; ok {
			t.Fatalf("source %s contains forbidden static tier field", source.ID)
		}
		if _, ok := raw.Sources[i]["source_standing"]; ok {
			t.Fatalf("source %s contains forbidden static source_standing field", source.ID)
		}
	}
	if byType[sources.SourceTypeGDELT] < 1 || byType[sources.SourceTypeRSS] < 130 || byType[sources.SourceTypeTelegram] < 65 {
		t.Fatalf("source type coverage too narrow: %+v", byType)
	}
	for _, required := range []string{"technology", "science", "finance", "industry", "regional_sentiment", "supply_chain", "energy", "agriculture", "open_source"} {
		if verticals[required] == 0 {
			t.Fatalf("missing required source vertical %q; verticals=%+v", required, verticals)
		}
	}
	for _, required := range []string{"en", "de", "fr", "es", "ru", "uk", "zh", "ja"} {
		if languages[required] == 0 {
			t.Fatalf("missing required source language %q; languages=%+v", required, languages)
		}
	}
	for _, required := range []string{"global", "us", "europe", "asia", "africa", "latin_america", "middle_east"} {
		if regions[required] == 0 {
			t.Fatalf("missing required source region %q; regions=%+v", required, regions)
		}
	}
	for _, required := range []string{"rss:hn_best", "rss:hn_ask", "telegram:androidpolice", "telegram:sciencealert", "rss:semiconductor_digest"} {
		if !seen[required] {
			t.Fatalf("missing required newly validated source %q", required)
		}
	}
}

func TestRunCycleWritesSourceItemsToObjectGraphWebCaptures(t *testing.T) {
	previous := sourcefetch.SetAllowPrivateNetworkForTests(true)
	t.Cleanup(func() { sourcefetch.SetAllowPrivateNetworkForTests(previous) })
	t.Setenv("SOURCE_SERVICE_RUNTIME_BASE_URL", "")
	t.Setenv("SOURCECYCLED_RUNTIME_BASE_URL", "")
	t.Setenv("RUNTIME_STORE_PATH", "")
	graphPath := filepath.Join(t.TempDir(), "runtime.objectgraph.db")
	t.Setenv("SOURCE_SERVICE_OBJECTGRAPH_DB_PATH", graphPath)
	t.Setenv("SOURCE_SERVICE_OBJECTGRAPH_OWNER_ID", "universal-wire-platform")
	t.Setenv("SOURCE_SERVICE_OBJECTGRAPH_COMPUTER_ID", "computer-universal-wire-platform")

	feedServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(`<?xml version="1.0"?>
<rss version="2.0">
  <channel>
    <title>Wire Test Feed</title>
    <item>
      <guid>wire-graph-1</guid>
      <title>Graph ingest story</title>
      <link>` + "https://example.com/wire-graph-1#frag" + `</link>
      <description>Sourcecycled persisted this feed item and projected it into objectgraph.</description>
      <pubDate>Fri, 26 Jun 2026 12:00:00 +0000</pubDate>
    </item>
  </channel>
</rss>`))
	}))
	defer feedServer.Close()

	store, err := cycle.NewStorage(filepath.Join(t.TempDir(), "sourcecycled.db"))
	if err != nil {
		t.Fatalf("new storage: %v", err)
	}
	defer store.Close()
	registry := &sources.Registry{
		UserAgent: "ChoirTest/1.0",
		Sources: []sources.Source{{
			ID:        "rss:wire-test",
			Type:      sources.SourceTypeRSS,
			Name:      "Wire Test",
			URL:       feedServer.URL,
			Verticals: []string{"wire"},
			Regions:   []string{"global"},
		}},
	}
	if err := store.SaveSources(registry); err != nil {
		t.Fatalf("save sources: %v", err)
	}
	engine = nil
	t.Cleanup(func() { engine = nil })

	runCycle(context.Background(), registry, store)

	graphStore, err := objectgraph.NewSQLiteStore(graphPath)
	if err != nil {
		t.Fatalf("open objectgraph store: %v", err)
	}
	graph := objectgraph.NewService(objectgraph.Config{SQLite: graphStore})
	defer graph.Close()
	notTombstoned := false
	captures, err := graph.ListObjects(context.Background(), objectgraph.ListFilter{
		Kind:      objectgraph.WebCaptureObjectKind,
		OwnerID:   "universal-wire-platform",
		Tombstone: &notTombstoned,
	})
	if err != nil {
		t.Fatalf("list web captures: %v", err)
	}
	if len(captures) != 1 {
		t.Fatalf("captures = %d, want 1: %+v", len(captures), captures)
	}
	meta, err := objectgraph.WebCaptureMetadataFromObject(captures[0])
	if err != nil {
		t.Fatalf("decode capture metadata: %v", err)
	}
	if meta.Title != "Graph ingest story" || meta.CanonicalURL != "https://example.com/wire-graph-1" ||
		!strings.Contains(string(captures[0].Body), "Sourcecycled persisted this feed item") {
		t.Fatalf("capture metadata/body = %+v body=%q", meta, captures[0].Body)
	}
	edges, err := graph.ListEdges(context.Background(), objectgraph.EdgeFilter{FromID: captures[0].CanonicalID, Kind: "captured_from"})
	if err != nil {
		t.Fatalf("list captured_from edges: %v", err)
	}
	if len(edges) != 1 {
		t.Fatalf("captured_from edges = %+v, want one source item edge", edges)
	}
}

func TestSourceServiceAPISearchAndResolveItems(t *testing.T) {
	store, err := cycle.NewStorage(filepath.Join(t.TempDir(), "sourcecycled.db"))
	if err != nil {
		t.Fatalf("new storage: %v", err)
	}
	defer store.Close()

	now := time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)
	if err := store.SaveSources(&sources.Registry{Sources: []sources.Source{{
		ID:                  "official-fed",
		Type:                sources.SourceTypeRSS,
		Name:                "Federal Reserve",
		URL:                 "https://example.test/feed.xml",
		PollIntervalSeconds: 3600,
		AuthPolicy:          "none",
		StoreBodyPolicy:     "bounded_release_text",
	}}}); err != nil {
		t.Fatalf("save source: %v", err)
	}
	item := sources.Item{
		ID:              "srcitem_test_rates",
		SourceID:        "official-fed",
		SourceType:      sources.SourceTypeRSS,
		FetchID:         "fetch-rates-1",
		OriginalID:      "rates-2026-06-04",
		Title:           "Federal Reserve rate statement",
		Body:            "The committee held rates steady.",
		URL:             "https://example.test/rates",
		CanonicalURL:    "https://example.test/rates",
		Published:       now,
		FetchedAt:       now.Add(2 * time.Minute),
		Verticals:       []string{"macro", "official"},
		Language:        "en",
		Region:          "US",
		ContentHash:     "hash-rates",
		BodyKind:        sources.BodyKindSourceBody,
		BodyLength:      len("The committee held rates steady."),
		EvidenceLevel:   "official-source",
		VintagePolicy:   "point-in-time",
		LookaheadStatus: "safe",
		ReleaseDate:     "2026-06-04",
	}
	if err := store.SaveItems([]sources.Item{item}); err != nil {
		t.Fatalf("save item: %v", err)
	}

	searchReq := httptest.NewRequest(http.MethodGet, "/internal/source-service/search?q=rates&max_results=5", nil)
	searchRec := httptest.NewRecorder()
	handleSourceServiceSearch(store).ServeHTTP(searchRec, searchReq)
	if searchRec.Code != http.StatusOK {
		t.Fatalf("search status = %d body=%s", searchRec.Code, searchRec.Body.String())
	}
	var search sourceapi.SearchResponse
	if err := json.Unmarshal(searchRec.Body.Bytes(), &search); err != nil {
		t.Fatalf("decode search: %v", err)
	}
	if search.Provider != sourceapi.ProviderName || search.Metadata.TargetKind != sourceapi.TargetKind {
		t.Fatalf("unexpected search identity: %+v", search)
	}
	if !strings.Contains(searchRec.Body.String(), `"reader_snapshot":false`) || !strings.Contains(searchRec.Body.String(), `"body_length":32`) {
		t.Fatalf("search body classification fields not explicit: %s", searchRec.Body.String())
	}
	if len(search.Results) != 1 {
		t.Fatalf("search results = %d, want 1", len(search.Results))
	}
	got := search.Results[0]
	if got.ItemID != item.ID || got.TargetKind != sourceapi.TargetKind || got.ContentHash != item.ContentHash {
		t.Fatalf("unexpected search result: %+v", got)
	}
	if got.BodyKind != item.BodyKind || got.BodyLength != item.BodyLength || got.ReaderSnapshot {
		t.Fatalf("unexpected search body classification: %+v", got)
	}
	if got.StoreBodyPolicy != "bounded_release_text" || got.SourceAuthPolicy != "none" {
		t.Fatalf("unexpected search source policy fields: %+v", got)
	}

	handleReq := httptest.NewRequest(http.MethodGet, "/internal/source-service/search?q=source_service_item:"+item.ID+"&max_results=5", nil)
	handleRec := httptest.NewRecorder()
	handleSourceServiceSearch(store).ServeHTTP(handleRec, handleReq)
	if handleRec.Code != http.StatusOK {
		t.Fatalf("handle search status = %d body=%s", handleRec.Code, handleRec.Body.String())
	}
	var handleSearch sourceapi.SearchResponse
	if err := json.Unmarshal(handleRec.Body.Bytes(), &handleSearch); err != nil {
		t.Fatalf("decode handle search: %v", err)
	}
	if len(handleSearch.Results) != 1 || handleSearch.Results[0].ItemID != item.ID {
		t.Fatalf("handle search results = %+v, want exact source item", handleSearch.Results)
	}

	resolveReq := httptest.NewRequest(http.MethodGet, "/internal/source-service/items/"+item.ID, nil)
	resolveRec := httptest.NewRecorder()
	handleSourceServiceItem(store).ServeHTTP(resolveRec, resolveReq)
	if resolveRec.Code != http.StatusOK {
		t.Fatalf("resolve status = %d body=%s", resolveRec.Code, resolveRec.Body.String())
	}
	var resolved sourceapi.ResolveItemResponse
	if err := json.Unmarshal(resolveRec.Body.Bytes(), &resolved); err != nil {
		t.Fatalf("decode resolve: %v", err)
	}
	if resolved.Provider != sourceapi.ProviderName || resolved.Item.ItemID != item.ID {
		t.Fatalf("unexpected resolved item: %+v", resolved)
	}
	if resolved.Item.BodyKind != item.BodyKind || resolved.Item.BodyLength != item.BodyLength || resolved.Item.ReaderSnapshot {
		t.Fatalf("unexpected resolved body classification: %+v", resolved.Item)
	}
	if resolved.Item.StoreBodyPolicy != "bounded_release_text" || resolved.Item.SourceAuthPolicy != "none" {
		t.Fatalf("unexpected resolved source policy fields: %+v", resolved.Item)
	}
}

func TestSourceServiceAPIHealthReportsLedgerCounts(t *testing.T) {
	store, err := cycle.NewStorage(filepath.Join(t.TempDir(), "sourcecycled.db"))
	if err != nil {
		t.Fatalf("new storage: %v", err)
	}
	defer store.Close()

	now := time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)
	if err := store.SaveFetches([]sources.FetchRecord{{
		FetchID:    "fetch-health-1",
		SourceID:   "source-health",
		SourceType: sources.SourceTypeRSS,
		RequestURL: "https://example.test/feed",
		Status:     "ok",
		StartedAt:  now,
		EndedAt:    now.Add(time.Second),
		ItemCount:  1,
	}}); err != nil {
		t.Fatalf("save fetch: %v", err)
	}
	if err := store.SaveItems([]sources.Item{{
		ID:        "srcitem_health",
		SourceID:  "source-health",
		Title:     "Health item",
		Published: now,
		FetchedAt: now,
	}}); err != nil {
		t.Fatalf("save item: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/internal/source-service/health", nil)
	rec := httptest.NewRecorder()
	handleSourceServiceHealth(store).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("health status = %d body=%s", rec.Code, rec.Body.String())
	}
	var health sourceapi.HealthResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &health); err != nil {
		t.Fatalf("decode health: %v", err)
	}
	if health.Status != "ok" || health.ItemCount != 1 || health.FetchCount != 1 {
		t.Fatalf("unexpected health: %+v", health)
	}
}

func TestSourceServiceAPIIngestionHandoffLatestReportsAgentHandoffs(t *testing.T) {
	ctx := context.Background()
	store, err := cycle.NewStorage(filepath.Join(t.TempDir(), "sourcecycled.db"))
	if err != nil {
		t.Fatalf("new storage: %v", err)
	}
	defer store.Close()

	cycleID, err := store.StartCycle(ctx)
	if err != nil {
		t.Fatalf("start cycle: %v", err)
	}
	now := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)
	items := []sources.Item{{
		ID:         "srcitem_ingestion_handoff",
		SourceID:   "gdelt:15min",
		SourceType: sources.SourceTypeGDELT,
		Title:      "Ingestion handoff event",
		Verticals:  []string{"supply_chain"},
		Region:     "global",
	}}
	events := cycle.BuildIngestionEventsFromItems(cycleID, items, now)
	if err := store.SaveIngestionEvents(ctx, events); err != nil {
		t.Fatalf("save ingestion events: %v", err)
	}
	handoff := cycle.BuildIngestionHandoff(cycleID, items, events, now)
	if err := store.SaveProcessorRequests(ctx, handoff.ProcessorRequests); err != nil {
		t.Fatalf("save processors: %v", err)
	}
	if err := store.SaveReconcilerRequests(ctx, handoff.ReconcilerRequests); err != nil {
		t.Fatalf("save reconcilers: %v", err)
	}
	if err := store.FinishCycle(ctx, cycleID, "completed", len(items), 1, nil); err != nil {
		t.Fatalf("finish cycle: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/internal/source-service/ingestion-handoff/latest", nil)
	rec := httptest.NewRecorder()
	handleSourceServiceIngestionHandoffLatest(store).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("ingestion handoff latest status = %d body=%s", rec.Code, rec.Body.String())
	}
	var resp sourceapi.IngestionHandoffResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode ingestion handoff latest: %v", err)
	}
	if resp.Provider != sourceapi.ProviderName || resp.Cycle.CycleID != cycleID {
		t.Fatalf("unexpected ingestion handoff identity: %+v", resp)
	}
	if len(resp.ProcessorRequests) != 1 || resp.ProcessorRequests[0].SourceItemIDs[0] != "srcitem_ingestion_handoff" {
		t.Fatalf("unexpected processor requests: %+v", resp.ProcessorRequests)
	}
	if resp.ProcessorRequests[0].Status != "queued" || resp.ProcessorRequests[0].RuntimeStatus != "queued" {
		t.Fatalf("unexpected processor status projection: %+v", resp.ProcessorRequests[0])
	}
	if len(resp.ReconcilerRequests) != 0 {
		t.Fatalf("unexpected reconciler requests: %+v", resp.ReconcilerRequests)
	}
	if resp.Metadata.AuthorityRule == "" {
		t.Fatalf("missing authority metadata: %+v", resp.Metadata)
	}
}

func TestSourceServiceAPIIngestionHandoffLatestTreatsNotModifiedAsSuccessfulFetch(t *testing.T) {
	ctx := context.Background()
	store, err := cycle.NewStorage(filepath.Join(t.TempDir(), "sourcecycled.db"))
	if err != nil {
		t.Fatalf("new storage: %v", err)
	}
	defer store.Close()

	cycleID, err := store.StartCycle(ctx)
	if err != nil {
		t.Fatalf("start cycle: %v", err)
	}
	now := time.Date(2026, 6, 8, 0, 26, 0, 0, time.UTC)
	if err := store.SaveCycleFetches(cycleID, []sources.FetchRecord{
		{
			FetchID:    "fetch-ok",
			SourceID:   "rss:active",
			SourceType: sources.SourceTypeRSS,
			RequestURL: "https://example.test/active.xml",
			Status:     "ok",
			StatusCode: http.StatusOK,
			StartedAt:  now,
			EndedAt:    now.Add(time.Second),
			ItemCount:  3,
		},
		{
			FetchID:    "fetch-not-modified",
			SourceID:   "rss:cached",
			SourceType: sources.SourceTypeRSS,
			RequestURL: "https://example.test/cached.xml",
			Status:     "not_modified",
			StatusCode: http.StatusNotModified,
			StartedAt:  now,
			EndedAt:    now.Add(time.Second),
		},
		{
			FetchID:    "fetch-error",
			SourceID:   "rss:blocked",
			SourceType: sources.SourceTypeRSS,
			RequestURL: "https://example.test/blocked.xml",
			Status:     "http_error",
			StatusCode: http.StatusForbidden,
			ErrorClass: "http_error",
			Error:      "unexpected status code: 403",
			StartedAt:  now,
			EndedAt:    now.Add(time.Second),
		},
	}); err != nil {
		t.Fatalf("save fetches: %v", err)
	}
	if err := store.FinishCycle(ctx, cycleID, "completed", 3, 3, nil); err != nil {
		t.Fatalf("finish cycle: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/internal/source-service/ingestion-handoff/latest", nil)
	rec := httptest.NewRecorder()
	handleSourceServiceIngestionHandoffLatest(store).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("latest status = %d body=%s", rec.Code, rec.Body.String())
	}
	var resp sourceapi.IngestionHandoffResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode latest: %v", err)
	}
	if resp.SourceHealth.ConfiguredSourceCount != 3 ||
		resp.SourceHealth.SuccessFetchCount != 2 ||
		resp.SourceHealth.FailedFetchCount != 1 {
		t.Fatalf("unexpected source health counts: %+v", resp.SourceHealth)
	}
	if len(resp.SourceHealth.Failures) != 1 || resp.SourceHealth.Failures[0].SourceID != "rss:blocked" {
		t.Fatalf("not_modified fetch should not appear as failure: %+v", resp.SourceHealth.Failures)
	}
	if resp.SourceHealth.ItemProducingSourceCount != 1 || resp.SourceHealth.ItemCount != 3 {
		t.Fatalf("unexpected source item counts: %+v", resp.SourceHealth)
	}
}

func TestIngestionRuntimeDispatcherSubmitsProcessorProfilesOnly(t *testing.T) {
	ctx := context.Background()
	store, err := cycle.NewStorage(filepath.Join(t.TempDir(), "sourcecycled.db"))
	if err != nil {
		t.Fatalf("new storage: %v", err)
	}
	defer store.Close()

	cycleID, err := store.StartCycle(ctx)
	if err != nil {
		t.Fatalf("start cycle: %v", err)
	}
	now := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)
	items := []sources.Item{
		{ID: "srcitem_gdelt_1", SourceID: "gdelt:15min", SourceType: sources.SourceTypeGDELT, Title: "GDELT 1", Region: "global"},
		{ID: "srcitem_rss_1", SourceID: "rss:bbc_world", SourceType: sources.SourceTypeRSS, Title: "BBC 1", Verticals: []string{"conflict"}, Region: "global"},
	}
	events := cycle.BuildIngestionEventsFromItems(cycleID, items, now)
	if err := store.SaveIngestionEvents(ctx, events); err != nil {
		t.Fatalf("save ingestion events: %v", err)
	}
	handoff := cycle.BuildIngestionHandoff(cycleID, items, events, now)
	if err := store.SaveProcessorRequests(ctx, handoff.ProcessorRequests); err != nil {
		t.Fatalf("save processors: %v", err)
	}
	if err := store.SaveReconcilerRequests(ctx, handoff.ReconcilerRequests); err != nil {
		t.Fatalf("save reconcilers: %v", err)
	}

	var submissions []runtimeRunSubmitRequest
	runtimeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Internal-Caller") != "true" {
			t.Fatalf("missing internal caller header")
		}
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/internal/runtime/runs":
			var req runtimeRunSubmitRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode runtime request: %v", err)
			}
			submissions = append(submissions, req)
			profile, _ := req.Metadata["agent_profile"].(string)
			writeSourceServiceJSON(w, http.StatusAccepted, runtimeRunStatusResponse{
				RunID:        "run-" + profile + "-" + strings.TrimSpace(req.Metadata["ingestion_handoff_request_id"].(string)),
				AgentID:      profile + ":agent",
				AgentProfile: profile,
				AgentRole:    profile,
				State:        "pending",
			})
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/internal/runtime/runs/"):
			writeSourceServiceJSON(w, http.StatusOK, runtimeRunStatusResponse{
				RunID:   path.Base(r.URL.Path),
				AgentID: "processor:agent",
				State:   "completed",
			})
		default:
			t.Fatalf("unexpected runtime request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer runtimeServer.Close()

	dispatcher := &ingestionRuntimeDispatcher{
		baseURL:              runtimeServer.URL,
		ownerID:              "owner-universal-wire",
		maxProcessorRequests: 1,
		client:               runtimeServer.Client(),
	}
	result := dispatcher.dispatch(ctx, store, handoff)
	if result.ProcessorSubmitted != 1 || result.ProcessorSkipped != len(handoff.ProcessorRequests)-1 || result.ReconcilerSubmitted != 0 || result.ReconcilerSkipped != 0 {
		t.Fatalf("unexpected dispatch result: %+v", result)
	}
	if result.ProcessorFailed != 0 || result.ReconcilerFailed != 0 || len(result.Errors) != 0 {
		t.Fatalf("unexpected dispatch failures: %+v", result)
	}
	if len(submissions) != 1 {
		t.Fatalf("runtime submissions = %d, want one processor before all processor handoffs are submitted", len(submissions))
	}
	if submissions[0].OwnerID != "owner-universal-wire" || submissions[0].Metadata["agent_profile"] != "processor" || submissions[0].Metadata["processor_key"] == "" {
		t.Fatalf("unexpected processor submission: %+v", submissions[0])
	}
	if !strings.Contains(submissions[0].Prompt, "Source item handles:") || !strings.Contains(submissions[0].Prompt, "Do not paste source bodies") {
		t.Fatalf("processor prompt missing source handle contract:\n%s", submissions[0].Prompt)
	}

	processors, err := store.ListProcessorRequests(ctx, cycleID, 10)
	if err != nil {
		t.Fatalf("list processors: %v", err)
	}
	var submitted, queued int
	var submittedRunID string
	for _, req := range processors {
		switch req.Status {
		case "submitted":
			submitted++
			submittedRunID = req.RuntimeRunID
		case "queued":
			queued++
		}
	}
	if submitted != 1 || queued != len(handoff.ProcessorRequests)-1 {
		t.Fatalf("processor statuses submitted=%d queued=%d processors=%+v", submitted, queued, processors)
	}
	if submittedRunID == "" {
		t.Fatalf("submitted processor missing runtime run id: %+v", processors)
	}
	dispatcher.maxProcessorRequests = 10
	secondResult := dispatcher.dispatch(ctx, store, cycle.IngestionHandoff{})
	if secondResult.ProcessorSubmitted != len(handoff.ProcessorRequests)-1 || secondResult.ProcessorSkipped != 0 || secondResult.ReconcilerSubmitted != 0 {
		t.Fatalf("unexpected second dispatch result: %+v", secondResult)
	}
	if len(submissions) != len(handoff.ProcessorRequests) {
		t.Fatalf("runtime submissions after drain = %d, want one per processor", len(submissions))
	}
}

func TestIngestionRuntimeDispatcherSkipsProcessorWithoutIngestionEvents(t *testing.T) {
	ctx := context.Background()
	store, err := cycle.NewStorage(filepath.Join(t.TempDir(), "sourcecycled.db"))
	if err != nil {
		t.Fatalf("new storage: %v", err)
	}
	defer store.Close()

	now := time.Date(2026, 6, 10, 9, 0, 0, 0, time.UTC)
	cycleID, err := store.StartCycle(ctx)
	if err != nil {
		t.Fatalf("start cycle: %v", err)
	}
	req := cycle.ProcessorRequest{
		RequestID:     "processor_no_ingestion",
		CycleID:       cycleID,
		ProcessorKey:  "processor:general:global:rss",
		Status:        "queued",
		SourceItemIDs: []string{"srcitem_orphan"},
		SourceCount:   1,
		Prompt:        "orphan processor",
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := store.SaveProcessorRequests(ctx, []cycle.ProcessorRequest{req}); err != nil {
		t.Fatalf("save processor request: %v", err)
	}

	dispatcher := &ingestionRuntimeDispatcher{
		baseURL:              "http://127.0.0.1:1",
		ownerID:              "universal-wire-platform",
		maxProcessorRequests: 1,
		client:               http.DefaultClient,
	}
	result := dispatcher.dispatch(ctx, store, cycle.IngestionHandoff{})
	if result.ProcessorSubmitted != 0 || result.ProcessorSkipped != 1 {
		t.Fatalf("unexpected dispatch result: %+v", result)
	}
}

func TestIngestionRuntimeDispatcherRetriesTransientRuntimeUnavailable(t *testing.T) {
	ctx := context.Background()
	req := cycle.ProcessorRequest{
		RequestID:     "processor_retry",
		CycleID:       "cycle_retry",
		ProcessorKey:  "processor:global_firehose:global:gdelt",
		Status:        "queued",
		SourceItemIDs: []string{"srcitem_retry_1"},
		SourceCount:   1,
		ContinuityRef: "sourcecycled://processor/processor:global_firehose:global:gdelt/latest",
		Prompt:        "Processor retry",
	}

	var attempts int
	runtimeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if r.Method != http.MethodPost || r.URL.Path != "/internal/runtime/runs" {
			t.Fatalf("unexpected runtime request %s %s", r.Method, r.URL.Path)
		}
		if attempts < 3 {
			writeSourceServiceJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "runtime warming"})
			return
		}
		writeSourceServiceJSON(w, http.StatusAccepted, runtimeRunStatusResponse{
			RunID:        "run-processor-retry",
			AgentID:      "processor:retry",
			AgentProfile: "processor",
			AgentRole:    "processor",
			State:        "pending",
		})
	}))
	defer runtimeServer.Close()

	dispatcher := &ingestionRuntimeDispatcher{
		baseURL:       runtimeServer.URL,
		ownerID:       "owner-universal-wire",
		client:        runtimeServer.Client(),
		retryAttempts: 4,
		retryDelay:    time.Millisecond,
	}
	run, err := dispatcher.submitProcessor(ctx, req)
	if err != nil {
		t.Fatalf("submit processor with transient retries: %v", err)
	}
	if attempts != 3 {
		t.Fatalf("runtime attempts = %d, want 3", attempts)
	}
	if run.RunID != "run-processor-retry" || run.AgentProfile != "processor" {
		t.Fatalf("unexpected run response: %+v", run)
	}
}

func TestIngestionRuntimeDispatcherKeepsQueuedRequestOnTransientRuntimeFailure(t *testing.T) {
	ctx := context.Background()
	store, err := cycle.NewStorage(filepath.Join(t.TempDir(), "sourcecycled.db"))
	if err != nil {
		t.Fatalf("new storage: %v", err)
	}
	defer store.Close()

	now := time.Date(2026, 6, 8, 7, 40, 0, 0, time.UTC)
	cycleID, err := store.StartCycle(ctx)
	if err != nil {
		t.Fatalf("start cycle: %v", err)
	}
	item := sources.Item{
		ID:         "srcitem_transient_1",
		SourceID:   "gdelt:15min",
		SourceType: sources.SourceTypeGDELT,
		Title:      "Transient queue item",
		Region:     "global",
	}
	events := cycle.BuildIngestionEventsFromItems(cycleID, []sources.Item{item}, now)
	if err := store.SaveIngestionEvents(ctx, events); err != nil {
		t.Fatalf("save ingestion events: %v", err)
	}
	req := cycle.ProcessorRequest{
		RequestID:         "processor_transient_queue",
		CycleID:           cycleID,
		ProcessorKey:      "processor:global_firehose:global:gdelt",
		Status:            "queued",
		SourceItemIDs:     []string{item.ID},
		IngestionEventIDs: []string{events[0].EventID},
		SourceCount:       1,
		ContinuityRef:     "sourcecycled://processor/processor:global_firehose:global:gdelt/latest",
		Prompt:            "Processor transient queue",
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := store.SaveProcessorRequests(ctx, []cycle.ProcessorRequest{req}); err != nil {
		t.Fatalf("save processor request: %v", err)
	}

	runtimeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeSourceServiceJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "runtime warming"})
	}))
	defer runtimeServer.Close()

	dispatcher := &ingestionRuntimeDispatcher{
		baseURL:              runtimeServer.URL,
		ownerID:              "owner-universal-wire",
		maxProcessorRequests: 1,
		client:               runtimeServer.Client(),
		retryAttempts:        1,
		retryDelay:           time.Millisecond,
	}
	result := dispatcher.dispatch(ctx, store, cycle.IngestionHandoff{})
	if result.ProcessorSubmitted != 0 || result.ProcessorFailed != 0 || len(result.Errors) != 1 {
		t.Fatalf("unexpected transient dispatch result: %+v", result)
	}

	processors, err := store.ListProcessorRequests(ctx, cycleID, 10)
	if err != nil {
		t.Fatalf("list processors: %v", err)
	}
	if len(processors) != 1 || processors[0].Status != "queued" || processors[0].RuntimeRunID != "" {
		t.Fatalf("transient runtime failure should leave request queued: %+v", processors)
	}
}

// dispatcherReconcileFixture is the shared two-request shape every dispatcher
// reconcile test starts from: one queued processor request waiting on
// admission, and one live request already submitted to the runtime whose
// status projection drives the test.
type dispatcherReconcileFixture struct {
	store   *cycle.Storage
	cycleID string
	queued  cycle.ProcessorRequest
	live    cycle.ProcessorRequest
}

// newDispatcherReconcileFixture seeds storage with one ingestion item plus the
// queued/live processor request pair. slug namespaces all generated IDs;
// liveStatus is the live request's verdict column ("submitted" or
// "completed") — its runtime_status is always "submitted". Rows are stamped
// with the current wall clock: dispatch backpressure counts in-flight
// requests by updated_at recency, so fixed timestamps would silently expire.
func newDispatcherReconcileFixture(t *testing.T, slug string, liveStatus string) dispatcherReconcileFixture {
	t.Helper()
	ctx := context.Background()
	now := time.Now().UTC()
	store, err := cycle.NewStorage(filepath.Join(t.TempDir(), "sourcecycled.db"))
	if err != nil {
		t.Fatalf("new storage: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	cycleID, err := store.StartCycle(ctx)
	if err != nil {
		t.Fatalf("start cycle: %v", err)
	}
	item := sources.Item{
		ID:         "srcitem_" + slug + "_1",
		SourceID:   "gdelt:15min",
		SourceType: sources.SourceTypeGDELT,
		Title:      slug + " item",
		Region:     "global",
	}
	events := cycle.BuildIngestionEventsFromItems(cycleID, []sources.Item{item}, now)
	if err := store.SaveIngestionEvents(ctx, events); err != nil {
		t.Fatalf("save ingestion events: %v", err)
	}
	queued := cycle.ProcessorRequest{
		RequestID:         "processor_" + slug + "_queue",
		CycleID:           cycleID,
		ProcessorKey:      "processor:global_firehose:global:gdelt",
		Status:            "queued",
		RuntimeStatus:     "queued",
		SourceItemIDs:     []string{item.ID},
		IngestionEventIDs: []string{events[0].EventID},
		SourceCount:       1,
		ContinuityRef:     "sourcecycled://processor/processor:global_firehose:global:gdelt/latest",
		Prompt:            "Queued " + slug + " processor",
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	live := cycle.ProcessorRequest{
		RequestID:         "processor_" + slug + "_live",
		CycleID:           cycleID,
		ProcessorKey:      "processor:global_firehose:global:gdelt",
		Status:            liveStatus,
		RuntimeStatus:     "submitted",
		RuntimeRunID:      "run-" + strings.ReplaceAll(slug, "_", "-") + "-live",
		SourceItemIDs:     []string{"srcitem-live"},
		IngestionEventIDs: []string{"ingestion-event-live"},
		SourceCount:       1,
		ContinuityRef:     "sourcecycled://processor/processor:global_firehose:global:gdelt/latest",
		Prompt:            "Live " + slug + " processor",
		CreatedAt:         now.Add(-time.Minute),
		UpdatedAt:         now,
	}
	if err := store.SaveProcessorRequests(ctx, []cycle.ProcessorRequest{queued, live}); err != nil {
		t.Fatalf("save processor requests: %v", err)
	}
	return dispatcherReconcileFixture{store: store, cycleID: cycleID, queued: queued, live: live}
}

// dispatcher returns the standard single-slot dispatcher pointed at the test
// runtime server. The in-flight window must cover the fixture's row
// timestamps: with the default zero window, backpressure only counts rows
// updated at or after time.Now() and the fixture's rows would never qualify.
func (f dispatcherReconcileFixture) dispatcher(server *httptest.Server) *ingestionRuntimeDispatcher {
	return &ingestionRuntimeDispatcher{
		baseURL:              server.URL,
		ownerID:              "owner-universal-wire",
		maxProcessorRequests: 1,
		client:               server.Client(),
		inFlightWindow:       time.Hour,
	}
}

func (f dispatcherReconcileFixture) requestsByID(t *testing.T, ctx context.Context) map[string]cycle.ProcessorRequest {
	t.Helper()
	processors, err := f.store.ListProcessorRequests(ctx, f.cycleID, 10)
	if err != nil {
		t.Fatalf("list processors: %v", err)
	}
	byID := map[string]cycle.ProcessorRequest{}
	for _, req := range processors {
		byID[req.RequestID] = req
	}
	return byID
}

func TestIngestionRuntimeDispatcherKeepsRuntimeSubmittedRequestsInFlightAfterVerdictCompletion(t *testing.T) {
	ctx := context.Background()
	fx := newDispatcherReconcileFixture(t, "budget", "completed")

	runtimeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/internal/runtime/runs/"):
			writeSourceServiceJSON(w, http.StatusOK, runtimeRunStatusResponse{
				RunID:   path.Base(r.URL.Path),
				AgentID: "processor:agent",
				State:   "running",
			})
		case r.Method == http.MethodPost && r.URL.Path == "/internal/runtime/runs":
			t.Fatalf("unexpected processor submission while runtime_status=request submitted remains in flight")
		default:
			t.Fatalf("unexpected runtime request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer runtimeServer.Close()

	result := fx.dispatcher(runtimeServer).dispatch(ctx, fx.store, cycle.IngestionHandoff{})
	if result.ProcessorSubmitted != 0 || result.ProcessorSkipped != 1 {
		t.Fatalf("unexpected dispatch result: %+v", result)
	}

	statusByID := fx.requestsByID(t, ctx)
	if statusByID[fx.live.RequestID].Status != "completed" || statusByID[fx.live.RequestID].RuntimeStatus != "submitted" {
		t.Fatalf("live request runtime/verdict split lost: %+v", statusByID[fx.live.RequestID])
	}
	if statusByID[fx.queued.RequestID].Status != "queued" || statusByID[fx.queued.RequestID].RuntimeStatus != "queued" {
		t.Fatalf("queued request changed unexpectedly: %+v", statusByID[fx.queued.RequestID])
	}
}

func TestIngestionRuntimeDispatcherProjectsPublishedCorpusCoverageWithoutReleasingBudget(t *testing.T) {
	ctx := context.Background()
	fx := newDispatcherReconcileFixture(t, "coverage", "submitted")

	runtimeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/internal/runtime/runs/"):
			writeSourceServiceJSON(w, http.StatusOK, runtimeRunStatusResponse{
				RunID:   path.Base(r.URL.Path),
				AgentID: "processor:agent",
				State:   "running",
				Trajectory: &runtimeTrajectoryStatusResponse{
					TrajectoryID:      "traj-coverage-live",
					Status:            "cancelled",
					SettlementReady:   false,
					OpenWorkItemCount: 0,
				},
				ProcessorResolution: &runtimeProcessorResolutionStatusResponse{
					WorkItemID:              "work-coverage-live",
					Status:                  "completed",
					ResolutionState:         "all_source_items_suppressed_against_published_corpus",
					SourceItemCount:         1,
					ResolvedSourceItemCount: 1,
					LastDecision:            "already_covered",
					CoveredByDocID:          "doc-covered-live",
				},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/internal/runtime/runs":
			t.Fatalf("unexpected processor submission while runtime_status submitted still occupies the slot")
		default:
			t.Fatalf("unexpected runtime request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer runtimeServer.Close()

	result := fx.dispatcher(runtimeServer).dispatch(ctx, fx.store, cycle.IngestionHandoff{})
	if result.ProcessorSubmitted != 0 || result.ProcessorSkipped != 1 {
		t.Fatalf("unexpected dispatch result: %+v", result)
	}

	statusByID := fx.requestsByID(t, ctx)
	if statusByID[fx.live.RequestID].Status != "completed" || statusByID[fx.live.RequestID].RuntimeStatus != "submitted" {
		t.Fatalf("coverage live request projection wrong: %+v", statusByID[fx.live.RequestID])
	}
	if statusByID[fx.queued.RequestID].Status != "queued" || statusByID[fx.queued.RequestID].RuntimeStatus != "queued" {
		t.Fatalf("queued request changed unexpectedly: %+v", statusByID[fx.queued.RequestID])
	}
}

func TestIngestionRuntimeDispatcherProjectsExplicitNoStoryTerminalWithoutReleasingBudget(t *testing.T) {
	ctx := context.Background()
	fx := newDispatcherReconcileFixture(t, "no_story", "submitted")

	runtimeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/internal/runtime/runs/"):
			writeSourceServiceJSON(w, http.StatusOK, runtimeRunStatusResponse{
				RunID:   path.Base(r.URL.Path),
				AgentID: "processor:agent",
				State:   "running",
				Trajectory: &runtimeTrajectoryStatusResponse{
					TrajectoryID:      "traj-no-story-live",
					Status:            "cancelled",
					SettlementReady:   false,
					OpenWorkItemCount: 0,
				},
				ProcessorResolution: &runtimeProcessorResolutionStatusResponse{
					WorkItemID:              "work-no-story-live",
					Status:                  "completed",
					ResolutionState:         "all_source_items_decided_without_story_route",
					SourceItemCount:         1,
					ResolvedSourceItemCount: 1,
					LastDecision:            "not_newsworthy",
				},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/internal/runtime/runs":
			t.Fatalf("unexpected processor submission while runtime_status submitted still occupies the slot")
		default:
			t.Fatalf("unexpected runtime request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer runtimeServer.Close()

	result := fx.dispatcher(runtimeServer).dispatch(ctx, fx.store, cycle.IngestionHandoff{})
	if result.ProcessorSubmitted != 0 || result.ProcessorSkipped != 1 {
		t.Fatalf("unexpected dispatch result: %+v", result)
	}

	statusByID := fx.requestsByID(t, ctx)
	if statusByID[fx.live.RequestID].Status != "completed" || statusByID[fx.live.RequestID].RuntimeStatus != "submitted" {
		t.Fatalf("explicit no-story live request projection wrong: %+v", statusByID[fx.live.RequestID])
	}
	if statusByID[fx.queued.RequestID].Status != "queued" || statusByID[fx.queued.RequestID].RuntimeStatus != "queued" {
		t.Fatalf("queued request changed unexpectedly: %+v", statusByID[fx.queued.RequestID])
	}
}

func TestIngestionRuntimeDispatcherProjectsSettledTrajectoryWithoutWaitingForRunTree(t *testing.T) {
	ctx := context.Background()
	fx := newDispatcherReconcileFixture(t, "settled", "submitted")

	runtimeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/internal/runtime/runs/"):
			writeSourceServiceJSON(w, http.StatusOK, runtimeRunStatusResponse{
				RunID:           path.Base(r.URL.Path),
				AgentID:         "processor:agent",
				State:           "running",
				ActiveChildRuns: 2,
				Trajectory: &runtimeTrajectoryStatusResponse{
					TrajectoryID:      "traj-settled-live",
					Status:            "settled",
					SettlementReady:   false,
					OpenWorkItemCount: 0,
				},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/internal/runtime/runs":
			t.Fatalf("unexpected processor submission while runtime_status submitted still occupies the slot")
		default:
			t.Fatalf("unexpected runtime request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer runtimeServer.Close()

	result := fx.dispatcher(runtimeServer).dispatch(ctx, fx.store, cycle.IngestionHandoff{})
	if result.ProcessorSubmitted != 0 || result.ProcessorSkipped != 1 {
		t.Fatalf("unexpected dispatch result: %+v", result)
	}

	statusByID := fx.requestsByID(t, ctx)
	if statusByID[fx.live.RequestID].Status != "completed" || statusByID[fx.live.RequestID].RuntimeStatus != "submitted" {
		t.Fatalf("settled live request projection wrong: %+v", statusByID[fx.live.RequestID])
	}
	if statusByID[fx.queued.RequestID].Status != "queued" || statusByID[fx.queued.RequestID].RuntimeStatus != "queued" {
		t.Fatalf("queued request changed unexpectedly: %+v", statusByID[fx.queued.RequestID])
	}
}

func TestIngestionRuntimeDispatcherMarksDeferredBranchWithoutRepollingForever(t *testing.T) {
	ctx := context.Background()
	fx := newDispatcherReconcileFixture(t, "deferred", "submitted")

	var deferredPolls int
	runtimeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/internal/runtime/runs/"):
			runID := path.Base(r.URL.Path)
			switch runID {
			case fx.live.RuntimeRunID:
				deferredPolls++
				writeSourceServiceJSON(w, http.StatusOK, runtimeRunStatusResponse{
					RunID:   runID,
					AgentID: "processor:deferred",
					State:   "completed",
					Trajectory: &runtimeTrajectoryStatusResponse{
						TrajectoryID:      "traj-deferred-live",
						Status:            "live",
						SettlementReady:   false,
						OpenWorkItemCount: 1,
					},
					ProcessorResolution: &runtimeProcessorResolutionStatusResponse{
						WorkItemID:              "work-deferred-live",
						Status:                  "open",
						ResolutionState:         "all_source_items_deferred_without_story_route",
						SourceItemCount:         1,
						ResolvedSourceItemCount: 1,
						LastDecision:            "deferred",
					},
				})
			case "run-deferred-queued":
				writeSourceServiceJSON(w, http.StatusOK, runtimeRunStatusResponse{
					RunID:   runID,
					AgentID: "processor:queued",
					State:   "running",
				})
			default:
				t.Fatalf("unexpected runtime status run %s", runID)
			}
		case r.Method == http.MethodPost && r.URL.Path == "/internal/runtime/runs":
			writeSourceServiceJSON(w, http.StatusAccepted, runtimeRunStatusResponse{
				RunID:        "run-deferred-queued",
				AgentID:      "processor:queued",
				AgentProfile: "processor",
				AgentRole:    "processor",
				State:        "pending",
			})
		default:
			t.Fatalf("unexpected runtime request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer runtimeServer.Close()

	dispatcher := fx.dispatcher(runtimeServer)

	first := dispatcher.dispatch(ctx, fx.store, cycle.IngestionHandoff{})
	if first.ProcessorSubmitted != 1 {
		t.Fatalf("first dispatch should submit queued request after deferred runtime releases capacity: %+v", first)
	}
	byID := fx.requestsByID(t, ctx)
	if got := byID[fx.live.RequestID]; got.Status != "deferred" || got.RuntimeStatus != "completed" {
		t.Fatalf("live deferred request after first dispatch = %+v, want deferred verdict + completed runtime", got)
	}
	if got := byID[fx.queued.RequestID]; got.Status != "submitted" || got.RuntimeStatus != "submitted" || got.RuntimeRunID != "run-deferred-queued" {
		t.Fatalf("queued request after first dispatch = %+v, want submitted/submitted with runtime run id", got)
	}

	second := dispatcher.dispatch(ctx, fx.store, cycle.IngestionHandoff{})
	if second.ProcessorSubmitted != 0 {
		t.Fatalf("second dispatch should not submit more work: %+v", second)
	}
	byID = fx.requestsByID(t, ctx)
	if got := byID[fx.live.RequestID]; got.Status != "deferred" || got.RuntimeStatus != "completed" {
		t.Fatalf("live deferred request after second dispatch = %+v, want deferred verdict + completed runtime", got)
	}
	if deferredPolls != 1 {
		t.Fatalf("deferred run should stop being repolled once request leaves submitted; deferredPolls=%d", deferredPolls)
	}
}

func TestIngestionRuntimeDispatcherKeepsPollingSubmittedVerdictAfterRuntimeCompletes(t *testing.T) {
	ctx := context.Background()
	fx := newDispatcherReconcileFixture(t, "settlement", "submitted")

	var livePolls int
	var submissions int
	runtimeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/internal/runtime/runs/"):
			runID := path.Base(r.URL.Path)
			switch runID {
			case fx.live.RuntimeRunID:
				livePolls++
				settled := livePolls >= 2
				activeChildren, openItems, trajStatus := 2, 1, "live"
				if settled {
					activeChildren, openItems, trajStatus = 1, 0, "settled"
				}
				writeSourceServiceJSON(w, http.StatusOK, runtimeRunStatusResponse{
					RunID:           runID,
					AgentID:         "processor:settlement",
					State:           "completed",
					ActiveChildRuns: activeChildren,
					Trajectory: &runtimeTrajectoryStatusResponse{
						TrajectoryID:      "traj-settlement-live",
						Status:            trajStatus,
						SettlementReady:   false,
						OpenWorkItemCount: openItems,
					},
				})
			case "run-queued-submitted":
				writeSourceServiceJSON(w, http.StatusOK, runtimeRunStatusResponse{
					RunID:   runID,
					AgentID: "processor:queued",
					State:   "running",
				})
			default:
				t.Fatalf("unexpected runtime status run %s", runID)
			}
		case r.Method == http.MethodPost && r.URL.Path == "/internal/runtime/runs":
			submissions++
			writeSourceServiceJSON(w, http.StatusAccepted, runtimeRunStatusResponse{
				RunID:        "run-queued-submitted",
				AgentID:      "processor:queued",
				AgentProfile: "processor",
				AgentRole:    "processor",
				State:        "pending",
			})
		default:
			t.Fatalf("unexpected runtime request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer runtimeServer.Close()

	dispatcher := fx.dispatcher(runtimeServer)

	first := dispatcher.dispatch(ctx, fx.store, cycle.IngestionHandoff{})
	if first.ProcessorSubmitted != 1 {
		t.Fatalf("first dispatch should submit queued request after runtime capacity releases: %+v", first)
	}
	byID := fx.requestsByID(t, ctx)
	if got := byID[fx.live.RequestID]; got.Status != "submitted" || got.RuntimeStatus != "completed" {
		t.Fatalf("live request after first dispatch = %+v, want submitted verdict + completed runtime", got)
	}
	if got := byID[fx.queued.RequestID]; got.Status != "submitted" || got.RuntimeStatus != "submitted" || got.RuntimeRunID != "run-queued-submitted" {
		t.Fatalf("queued request after first dispatch = %+v, want submitted/submitted with runtime run id", got)
	}

	second := dispatcher.dispatch(ctx, fx.store, cycle.IngestionHandoff{})
	if second.ProcessorSubmitted != 0 {
		t.Fatalf("second dispatch should not submit more work: %+v", second)
	}
	byID = fx.requestsByID(t, ctx)
	if got := byID[fx.live.RequestID]; got.Status != "completed" || got.RuntimeStatus != "completed" {
		t.Fatalf("live request after second dispatch = %+v, want completed verdict + completed runtime", got)
	}
	if livePolls < 2 {
		t.Fatalf("live request was not repolled after runtime completion; livePolls=%d submissions=%d", livePolls, submissions)
	}
}

func TestIngestionRuntimeDispatcherStopsReadingActiveChildRunsForCompletedProcessorCapacity(t *testing.T) {
	ctx := context.Background()
	fx := newDispatcherReconcileFixture(t, "childrun_capacity", "submitted")

	var submissions int
	runtimeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/internal/runtime/runs/"):
			runID := path.Base(r.URL.Path)
			switch runID {
			case fx.live.RuntimeRunID:
				writeSourceServiceJSON(w, http.StatusOK, runtimeRunStatusResponse{
					RunID:           runID,
					AgentID:         "processor:childrun-capacity",
					State:           "running",
					ActiveChildRuns: 3,
					Trajectory: &runtimeTrajectoryStatusResponse{
						TrajectoryID:      "traj-childrun-capacity-live",
						Status:            "live",
						SettlementReady:   false,
						OpenWorkItemCount: 1,
					},
					ProcessorResolution: &runtimeProcessorResolutionStatusResponse{
						WorkItemID:              "work-childrun-capacity-live",
						Status:                  "completed",
						ResolutionState:         "all_source_items_decided_with_story_route",
						SourceItemCount:         1,
						ResolvedSourceItemCount: 1,
						LastDecision:            "opened_texture",
						StoryDocID:              "doc-childrun-capacity-live",
					},
				})
			case "run-childrun-capacity-queued":
				writeSourceServiceJSON(w, http.StatusOK, runtimeRunStatusResponse{
					RunID:   runID,
					AgentID: "processor:queued",
					State:   "running",
				})
			default:
				t.Fatalf("unexpected runtime status run %s", runID)
			}
		case r.Method == http.MethodPost && r.URL.Path == "/internal/runtime/runs":
			submissions++
			writeSourceServiceJSON(w, http.StatusAccepted, runtimeRunStatusResponse{
				RunID:        "run-childrun-capacity-queued",
				AgentID:      "processor:queued",
				AgentProfile: "processor",
				AgentRole:    "processor",
				State:        "pending",
			})
		default:
			t.Fatalf("unexpected runtime request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer runtimeServer.Close()

	result := fx.dispatcher(runtimeServer).dispatch(ctx, fx.store, cycle.IngestionHandoff{})
	if result.ProcessorSubmitted != 1 {
		t.Fatalf("dispatch should submit queued request once completed processor-resolution frees capacity despite active children: %+v", result)
	}

	byID := fx.requestsByID(t, ctx)
	if got := byID[fx.live.RequestID]; got.Status != "submitted" || got.RuntimeStatus != "completed" {
		t.Fatalf("live request after dispatch = %+v, want submitted verdict + completed runtime", got)
	}
	if got := byID[fx.queued.RequestID]; got.Status != "submitted" || got.RuntimeStatus != "submitted" || got.RuntimeRunID != "run-childrun-capacity-queued" {
		t.Fatalf("queued request after dispatch = %+v, want submitted/submitted with runtime run id", got)
	}
	if submissions != 1 {
		t.Fatalf("queued request should have submitted exactly once; submissions=%d", submissions)
	}
}

func TestIngestionRuntimeDispatcherStoryRouteCapacityReleaseAlignsWithRuntimeAdmission(t *testing.T) {
	ctx := context.Background()
	fx := newDispatcherReconcileFixture(t, "story_route_429", "submitted")

	var submissions int
	runtimeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/internal/runtime/runs/"):
			writeSourceServiceJSON(w, http.StatusOK, runtimeRunStatusResponse{
				RunID:           path.Base(r.URL.Path),
				AgentID:         "processor:story-route-429",
				State:           "running",
				ActiveChildRuns: 2,
				Trajectory: &runtimeTrajectoryStatusResponse{
					TrajectoryID:      "traj-story-route-429-live",
					Status:            "live",
					SettlementReady:   false,
					OpenWorkItemCount: 1,
				},
				ProcessorResolution: &runtimeProcessorResolutionStatusResponse{
					WorkItemID:              "work-story-route-429-live",
					Status:                  "completed",
					ResolutionState:         "all_source_items_decided_with_story_route",
					SourceItemCount:         1,
					ResolvedSourceItemCount: 1,
					LastDecision:            "opened_texture",
					StoryDocID:              "doc-story-route-429-live",
				},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/internal/runtime/runs":
			submissions++
			writeSourceServiceJSON(w, http.StatusAccepted, runtimeRunStatusResponse{
				RunID:        "run-story-route-429-queued",
				AgentID:      "processor:queued",
				AgentProfile: "processor",
				AgentRole:    "processor",
				State:        "pending",
			})
		default:
			t.Fatalf("unexpected runtime request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer runtimeServer.Close()

	result := fx.dispatcher(runtimeServer).dispatch(ctx, fx.store, cycle.IngestionHandoff{})
	if result.ProcessorSubmitted != 1 {
		t.Fatalf("dispatch should submit queued request once runtime admission aligns with story-route release: %+v", result)
	}
	if len(result.Errors) != 0 {
		t.Fatalf("dispatch errors = %+v, want none", result.Errors)
	}

	byID := fx.requestsByID(t, ctx)
	if got := byID[fx.live.RequestID]; got.Status != "submitted" || got.RuntimeStatus != "completed" {
		t.Fatalf("live request after dispatch = %+v, want submitted verdict + completed runtime", got)
	}
	if got := byID[fx.queued.RequestID]; got.Status != "submitted" || got.RuntimeStatus != "submitted" || got.RuntimeRunID != "run-story-route-429-queued" {
		t.Fatalf("queued request after dispatch = %+v, want submitted/submitted with runtime run id", got)
	}
	if submissions != 1 {
		t.Fatalf("queued request should have submitted exactly once; submissions=%d", submissions)
	}
}
