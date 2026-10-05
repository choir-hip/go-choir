package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	embedded "github.com/dolthub/driver/v2"
	"github.com/yusefmosiah/go-choir/internal/cycle"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/sourceapi"
	"github.com/yusefmosiah/go-choir/internal/sources"
)

// newTestCycleStorage creates an embedded Dolt-backed cycle.Store for
// sourcecycled tests, mirroring the platform package's openTestPlatformStore.
func newTestCycleStorage(t *testing.T) cycle.Store {
	t.Helper()
	root := t.TempDir()
	rootDSN := fmt.Sprintf("file://%s?commitname=Choir&commitemail=system@choir.local&multistatements=true", root)
	rootCfg, err := embedded.ParseDSN(rootDSN)
	if err != nil {
		t.Fatalf("parse root dsn: %v", err)
	}
	rootConnector, err := embedded.NewConnector(rootCfg)
	if err != nil {
		t.Fatalf("new root connector: %v", err)
	}
	rootDB := sql.OpenDB(rootConnector)
	if _, err := rootDB.Exec("CREATE DATABASE IF NOT EXISTS platform"); err != nil {
		t.Fatalf("create database: %v", err)
	}
	_ = rootDB.Close()
	_ = rootConnector.Close()

	dbDSN := fmt.Sprintf("file://%s?commitname=Choir&commitemail=system@choir.local&database=platform&multistatements=true&clientfoundrows=true", root)
	dbCfg, err := embedded.ParseDSN(dbDSN)
	if err != nil {
		t.Fatalf("parse db dsn: %v", err)
	}
	dbConnector, err := embedded.NewConnector(dbCfg)
	if err != nil {
		t.Fatalf("new db connector: %v", err)
	}
	db := sql.OpenDB(dbConnector)
	store, err := cycle.NewStorageFromDB(db)
	if err != nil {
		t.Fatalf("bootstrap cycle storage: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
		_ = dbConnector.Close()
	})
	return store
}

func TestWriteSourceItemsToObjectGraphPublishesCanonicalObjectsAndEdges(t *testing.T) {
	t.Setenv("SOURCE_SERVICE_OBJECTGRAPH_OWNER_ID", "universal-wire-platform")
	t.Setenv("SOURCE_SERVICE_OBJECTGRAPH_COMPUTER_ID", "host-sourcecycled")
	objects := make(map[string]objectgraph.Object)
	var edges []objectgraph.Edge
	var requestPaths []string
	hostServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPaths = append(requestPaths, r.URL.Path)
		if strings.Contains(r.URL.Path, "/internal/runtime/objectgraph/web-captures") {
			t.Fatalf("capture publication targeted retired runtime route: %s", r.URL.Path)
		}
		if got := r.Header.Get("X-Internal-Caller"); got != "true" {
			t.Fatalf("X-Internal-Caller = %q, want true", got)
		}
		switch {
		case r.Method == http.MethodPut && r.URL.Path == "/internal/platform/objects":
			var obj objectgraph.Object
			if err := json.NewDecoder(r.Body).Decode(&obj); err != nil {
				t.Fatalf("decode canonical object: %v", err)
			}
			objects[obj.CanonicalID] = obj
			writeSourceServiceJSON(w, http.StatusOK, obj)
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/internal/platform/objects/"):
			id := strings.TrimPrefix(r.URL.Path, "/internal/platform/objects/")
			obj, ok := objects[id]
			if !ok {
				http.NotFound(w, r)
				return
			}
			writeSourceServiceJSON(w, http.StatusOK, obj)
		case r.Method == http.MethodPut && r.URL.Path == "/internal/platform/edges":
			var edge objectgraph.Edge
			if err := json.NewDecoder(r.Body).Decode(&edge); err != nil {
				t.Fatalf("decode canonical edge: %v", err)
			}
			edges = append(edges, edge)
			writeSourceServiceJSON(w, http.StatusOK, edge)
		default:
			t.Fatalf("unexpected canonical objectgraph request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer hostServer.Close()
	t.Setenv("SOURCE_SERVICE_OBJECTGRAPH_BASE_URL", hostServer.URL)

	store := newTestCycleStorage(t)
	defer store.Close()
	cycleID, err := store.StartCycle(context.Background())
	if err != nil {
		t.Fatalf("start cycle: %v", err)
	}
	now := time.Date(2026, 6, 26, 18, 45, 0, 0, time.UTC)
	item := sources.Item{
		ID:           "srcitem-canonical-target-1",
		SourceID:     "rss:canonical_target",
		SourceType:   sources.SourceTypeRSS,
		FetchID:      "fetch-canonical-target-1",
		OriginalID:   "https://example.com/canonical-target",
		Title:        "Canonical target story",
		Body:         "Sourcecycled publishes this capture directly to the host canonical object graph.",
		URL:          "https://example.com/canonical-target",
		CanonicalURL: "https://example.com/canonical-target",
		FetchedAt:    now,
		ContentHash:  sources.ContentHash("Canonical target story", "Sourcecycled publishes this capture directly to the host canonical object graph.", "https://example.com/canonical-target", "https://example.com/canonical-target"),
	}
	if err := writeSourceItemsToObjectGraph(context.Background(), store, cycleID, []sources.Item{item}, now, "web_captures_graph_written", "source items published to canonical objectgraph web captures"); err != nil {
		t.Fatalf("write source items to canonical objectgraph: %v", err)
	}
	if len(objects) != 2 {
		t.Fatalf("canonical objects = %d, want capture and source entity; paths=%v", len(objects), requestPaths)
	}
	var captureCount, sourceEntityCount int
	for _, obj := range objects {
		if obj.OwnerID != "universal-wire-platform" || obj.ComputerID != "host-sourcecycled" {
			t.Fatalf("canonical object authority = %+v", obj)
		}
		switch obj.ObjectKind {
		case objectgraph.WebCaptureObjectKind:
			captureCount++
		case objectgraph.ObjectKind("choir.source_entity"):
			sourceEntityCount++
		}
	}
	if captureCount != 1 || sourceEntityCount != 1 {
		t.Fatalf("canonical object kinds: captures=%d source_entities=%d", captureCount, sourceEntityCount)
	}
	if len(edges) != 1 || edges[0].Kind != objectgraph.EdgeKind("captured_from") {
		t.Fatalf("canonical edges = %+v, want one captured_from edge", edges)
	}
	for _, requestPath := range requestPaths {
		if strings.Contains(requestPath, "/internal/runtime/") {
			t.Fatalf("canonical publication made runtime request %q", requestPath)
		}
	}
	summary, err := store.LatestCycleSummary(context.Background())
	if err != nil {
		t.Fatalf("latest cycle summary: %v", err)
	}
	if len(summary.Events) != 1 {
		t.Fatalf("cycle events = %d, want 1", len(summary.Events))
	}
	event := summary.Events[0]
	if event.Kind != "web_captures_graph_written" ||
		event.Metadata["objectgraph_mode"] != "corpusd_api" ||
		event.Metadata["capture_count"] != float64(1) ||
		event.Metadata["captured_from_edges"] != float64(1) {
		t.Fatalf("cycle event = %+v", event)
	}
}

func TestProjectSourceItemsToObjectGraphNeverFallsBackToRuntime(t *testing.T) {
	t.Setenv("SOURCE_SERVICE_OBJECTGRAPH_BASE_URL", "")
	t.Setenv("SOURCECYCLED_OBJECTGRAPH_BASE_URL", "")
	runtimeRequests := 0
	runtimeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		runtimeRequests++
		http.Error(w, "runtime capture projection is retired", http.StatusGone)
	}))
	defer runtimeServer.Close()
	t.Setenv("SOURCE_SERVICE_RUNTIME_BASE_URL", runtimeServer.URL)

	_, err := projectSourceItemsToObjectGraph(context.Background(), []sources.Item{{
		ID:   "srcitem-no-runtime-fallback",
		Body: "This item must not be projected through a runtime.",
	}}, time.Now())
	if err == nil || !strings.Contains(err.Error(), "SOURCE_SERVICE_OBJECTGRAPH_BASE_URL") {
		t.Fatalf("missing canonical endpoint error = %v", err)
	}
	if runtimeRequests != 0 {
		t.Fatalf("runtime capture requests = %d, want zero", runtimeRequests)
	}
}

func TestSourceServiceIngestionHandoffLatestIncludesCycleEvents(t *testing.T) {
	ctx := context.Background()
	store := newTestCycleStorage(t)
	defer store.Close()

	cycleID, err := store.StartCycle(ctx)
	if err != nil {
		t.Fatalf("start cycle: %v", err)
	}
	if err := store.RecordCycleEvent(ctx, cycleID, "", "web_captures_graph_backfilled", "stored source items projected to empty objectgraph web captures", map[string]any{
		"capture_count":      3,
		"skipped_item_count": 1,
	}); err != nil {
		t.Fatalf("record cycle event: %v", err)
	}
	if err := store.FinishCycle(ctx, cycleID, "completed", 0, 0, nil); err != nil {
		t.Fatalf("finish cycle: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/internal/source-service/ingestion-handoff/latest", nil)
	rec := httptest.NewRecorder()
	handleSourceServiceIngestionHandoffLatest(store).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("latest handoff status = %d body=%s", rec.Code, rec.Body.String())
	}
	var resp sourceapi.IngestionHandoffResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode latest handoff response: %v", err)
	}
	if len(resp.Cycle.Events) != 1 {
		t.Fatalf("cycle events = %d, want 1: %+v", len(resp.Cycle.Events), resp.Cycle.Events)
	}
	event := resp.Cycle.Events[0]
	if event.Kind != "web_captures_graph_backfilled" ||
		event.Metadata["capture_count"] != float64(3) ||
		event.Metadata["skipped_item_count"] != float64(1) {
		t.Fatalf("unexpected cycle event in response: %+v", event)
	}
}

func TestSourceServiceAPISearchAndResolveItems(t *testing.T) {
	store := newTestCycleStorage(t)
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
	store := newTestCycleStorage(t)
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
	store := newTestCycleStorage(t)
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
	if err := store.SaveProcessorRequests(ctx, []cycle.ProcessorRequest{{
		RequestID:         "processor_ingestion_handoff",
		CycleID:           cycleID,
		ProcessorKey:      "processor:global_firehose:global:gdelt",
		Status:            "queued",
		RuntimeStatus:     "queued",
		SourceItemIDs:     []string{items[0].ID},
		SourceCount:       1,
		CreatedAt:         now,
		UpdatedAt:         now,
	}}); err != nil {
		t.Fatalf("save frozen processor request: %v", err)
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
	store := newTestCycleStorage(t)
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

