package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
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
