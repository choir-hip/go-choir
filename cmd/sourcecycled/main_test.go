package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/cycle"
	"github.com/yusefmosiah/go-choir/internal/sourceapi"
	"github.com/yusefmosiah/go-choir/internal/sources"
)

func TestSourceServiceAPISearchAndResolveItems(t *testing.T) {
	store, err := cycle.NewStorage(filepath.Join(t.TempDir(), "sourcecycled.db"))
	if err != nil {
		t.Fatalf("new storage: %v", err)
	}
	defer store.Close()

	now := time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)
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
	if len(search.Results) != 1 {
		t.Fatalf("search results = %d, want 1", len(search.Results))
	}
	got := search.Results[0]
	if got.ItemID != item.ID || got.TargetKind != sourceapi.TargetKind || got.ContentHash != item.ContentHash {
		t.Fatalf("unexpected search result: %+v", got)
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

func TestSourceServiceAPISourceMaxxLatestReportsAgentHandoffs(t *testing.T) {
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
		ID:         "srcitem_source_maxx",
		SourceID:   "gdelt:15min",
		SourceType: sources.SourceTypeGDELT,
		Title:      "SourceMaxx event",
		Verticals:  []string{"supply_chain"},
		Region:     "global",
	}}
	handoff := cycle.BuildSourceMaxxHandoff(cycleID, items, now)
	if err := store.SaveProcessorRequests(ctx, handoff.ProcessorRequests); err != nil {
		t.Fatalf("save processors: %v", err)
	}
	if err := store.SaveReconcilerRequests(ctx, handoff.ReconcilerRequests); err != nil {
		t.Fatalf("save reconcilers: %v", err)
	}
	if err := store.FinishCycle(ctx, cycleID, "completed", len(items), 1, nil); err != nil {
		t.Fatalf("finish cycle: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/internal/source-service/sourcemaxx/latest", nil)
	rec := httptest.NewRecorder()
	handleSourceServiceSourceMaxxLatest(store).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("sourcemaxx latest status = %d body=%s", rec.Code, rec.Body.String())
	}
	var resp sourceapi.SourceMaxxResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode sourcemaxx latest: %v", err)
	}
	if resp.Provider != sourceapi.ProviderName || resp.Cycle.CycleID != cycleID {
		t.Fatalf("unexpected sourcemaxx identity: %+v", resp)
	}
	if len(resp.ProcessorRequests) != 1 || resp.ProcessorRequests[0].SourceItemIDs[0] != "srcitem_source_maxx" {
		t.Fatalf("unexpected processor requests: %+v", resp.ProcessorRequests)
	}
	if len(resp.ReconcilerRequests) != 1 || len(resp.ReconcilerRequests[0].ProcessorRequestIDs) != 1 {
		t.Fatalf("unexpected reconciler requests: %+v", resp.ReconcilerRequests)
	}
	if resp.Metadata.AuthorityRule == "" {
		t.Fatalf("missing authority metadata: %+v", resp.Metadata)
	}
}

func TestSourceMaxxRuntimeDispatcherSubmitsProcessorAndReconcilerProfiles(t *testing.T) {
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
	handoff := cycle.BuildSourceMaxxHandoff(cycleID, items, now)
	if err := store.SaveProcessorRequests(ctx, handoff.ProcessorRequests); err != nil {
		t.Fatalf("save processors: %v", err)
	}
	if err := store.SaveReconcilerRequests(ctx, handoff.ReconcilerRequests); err != nil {
		t.Fatalf("save reconcilers: %v", err)
	}

	var submissions []runtimeRunSubmitRequest
	runtimeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/internal/runtime/runs" {
			t.Fatalf("unexpected runtime request %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("X-Internal-Caller") != "true" {
			t.Fatalf("missing internal caller header")
		}
		var req runtimeRunSubmitRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode runtime request: %v", err)
		}
		submissions = append(submissions, req)
		profile, _ := req.Metadata["agent_profile"].(string)
		writeSourceServiceJSON(w, http.StatusAccepted, runtimeRunStatusResponse{
			RunID:        "run-" + profile + "-" + strings.TrimSpace(req.Metadata["source_maxx_request_id"].(string)),
			AgentID:      profile + ":agent",
			AgentProfile: profile,
			AgentRole:    profile,
			State:        "pending",
		})
	}))
	defer runtimeServer.Close()

	dispatcher := &sourceMaxxRuntimeDispatcher{
		baseURL:              runtimeServer.URL,
		ownerID:              "owner-global-wire",
		maxProcessorRequests: 1,
		client:               runtimeServer.Client(),
	}
	result := dispatcher.dispatch(ctx, store, handoff)
	if result.ProcessorSubmitted != 1 || result.ProcessorSkipped != len(handoff.ProcessorRequests)-1 || result.ReconcilerSubmitted != 1 {
		t.Fatalf("unexpected dispatch result: %+v", result)
	}
	if result.ProcessorFailed != 0 || result.ReconcilerFailed != 0 || len(result.Errors) != 0 {
		t.Fatalf("unexpected dispatch failures: %+v", result)
	}
	if len(submissions) != 2 {
		t.Fatalf("runtime submissions = %d, want processor + reconciler", len(submissions))
	}
	if submissions[0].OwnerID != "owner-global-wire" || submissions[0].Metadata["agent_profile"] != "processor" || submissions[0].Metadata["processor_key"] == "" {
		t.Fatalf("unexpected processor submission: %+v", submissions[0])
	}
	if !strings.Contains(submissions[0].Prompt, "Source item handles:") || !strings.Contains(submissions[0].Prompt, "Do not paste source bodies") {
		t.Fatalf("processor prompt missing source handle contract:\n%s", submissions[0].Prompt)
	}
	if submissions[1].Metadata["agent_profile"] != "reconciler" || submissions[1].Metadata["reconciler_scope"] != "story-corpus" {
		t.Fatalf("unexpected reconciler submission: %+v", submissions[1])
	}

	processors, err := store.ListProcessorRequests(ctx, cycleID, 10)
	if err != nil {
		t.Fatalf("list processors: %v", err)
	}
	var submitted, queued int
	for _, req := range processors {
		switch req.Status {
		case "submitted":
			submitted++
		case "queued":
			queued++
		}
	}
	if submitted != 1 || queued != len(handoff.ProcessorRequests)-1 {
		t.Fatalf("processor statuses submitted=%d queued=%d processors=%+v", submitted, queued, processors)
	}
	reconcilers, err := store.ListReconcilerRequests(ctx, cycleID, 10)
	if err != nil {
		t.Fatalf("list reconcilers: %v", err)
	}
	if len(reconcilers) != 1 || reconcilers[0].Status != "submitted" {
		t.Fatalf("reconciler status = %+v", reconcilers)
	}
}

func TestSourceMaxxRuntimeDispatcherRetriesTransientRuntimeUnavailable(t *testing.T) {
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

	dispatcher := &sourceMaxxRuntimeDispatcher{
		baseURL:       runtimeServer.URL,
		ownerID:       "owner-global-wire",
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
