package cycle

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/sources"
)

func TestStoragePersistsFetchItemsAndDedupsAcrossRestart(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "sourcecycled.db")
	registry := &sources.Registry{
		UserAgent: "ChoirTest/1.0",
		Sources: []sources.Source{{
			ID:                  "official:test",
			Type:                sources.SourceTypeRSS,
			Name:                "Official Test",
			URL:                 "https://example.test/feed.xml",
			Verticals:           []string{"macro_policy"},
			Languages:           []string{"en"},
			Regions:             []string{"us"},
			Tier:                "T0",
			PollIntervalSeconds: 3600,
			AuthPolicy:          "none",
			StoreBodyPolicy:     "bounded_release_text",
			Official:            true,
			SourceStanding:      "good_standing",
		}},
	}
	source := registry.Sources[0]
	item := sources.Item{
		ID:              sources.StableItemID(source, "release-1", "https://example.test/release-1", "Rate decision", "Rates held steady."),
		SourceID:        source.ID,
		SourceType:      source.Type,
		OriginalID:      "release-1",
		Title:           "Rate decision",
		Body:            "Rates held steady.",
		URL:             "https://example.test/release-1",
		CanonicalURL:    "https://example.test/release-1",
		Published:       time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC),
		FetchedAt:       time.Date(2026, 6, 4, 12, 1, 0, 0, time.UTC),
		Verticals:       []string{"macro_policy"},
		Language:        "en",
		Region:          "us",
		ContentHash:     sources.ContentHash("Rate decision", "Rates held steady."),
		EvidenceLevel:   "official_release",
		VintagePolicy:   "release_snapshot",
		LookaheadStatus: "no_lookahead",
		ReleaseDate:     "2026-06-04",
	}
	fetch := sources.NewFetchRecord(source, source.URL, time.Date(2026, 6, 4, 12, 1, 0, 0, time.UTC))
	fetch.Status = "ok"
	fetch.EndedAt = time.Date(2026, 6, 4, 12, 1, 1, 0, time.UTC)
	fetch.StatusCode = 200
	fetch.ItemCount = 1
	item.FetchID = fetch.FetchID

	store, err := NewStorage(dbPath)
	if err != nil {
		t.Fatalf("open storage: %v", err)
	}
	if err := store.SaveSources(registry); err != nil {
		t.Fatalf("save sources: %v", err)
	}
	if err := store.SaveFetches([]sources.FetchRecord{fetch}); err != nil {
		t.Fatalf("save fetches: %v", err)
	}
	if err := store.SaveItems([]sources.Item{item}); err != nil {
		t.Fatalf("save items: %v", err)
	}
	if err := store.SaveItems([]sources.Item{item}); err != nil {
		t.Fatalf("save duplicate items: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close storage: %v", err)
	}

	reopened, err := NewStorage(dbPath)
	if err != nil {
		t.Fatalf("reopen storage: %v", err)
	}
	defer reopened.Close()
	count, err := reopened.CountItems(ctx)
	if err != nil {
		t.Fatalf("count items: %v", err)
	}
	if count != 1 {
		t.Fatalf("item count after duplicate save/restart = %d, want 1", count)
	}
	fetchCount, err := reopened.CountFetches(ctx)
	if err != nil {
		t.Fatalf("count fetches: %v", err)
	}
	if fetchCount != 1 {
		t.Fatalf("fetch count = %d, want 1", fetchCount)
	}
	results, err := reopened.SearchItems(ctx, "rates", 10)
	if err != nil {
		t.Fatalf("search items: %v", err)
	}
	if len(results) != 1 || results[0].ID != item.ID {
		t.Fatalf("search results = %+v, want item %s", results, item.ID)
	}
	if results[0].VintagePolicy != "release_snapshot" || results[0].LookaheadStatus != "no_lookahead" {
		t.Fatalf("official caveats did not persist: %+v", results[0])
	}
}

func TestStorageRecordsCycleEvents(t *testing.T) {
	ctx := context.Background()
	store, err := NewStorage(filepath.Join(t.TempDir(), "sourcecycled.db"))
	if err != nil {
		t.Fatalf("open storage: %v", err)
	}
	defer store.Close()
	cycleID, err := store.StartCycle(ctx)
	if err != nil {
		t.Fatalf("start cycle: %v", err)
	}
	if err := store.RecordCycleEvent(ctx, cycleID, "rss:test", "fetch_completed", "fetch completed", map[string]any{"items": 2}); err != nil {
		t.Fatalf("record event: %v", err)
	}
	if err := store.FinishCycle(ctx, cycleID, "completed", 2, 1, nil); err != nil {
		t.Fatalf("finish cycle: %v", err)
	}
	var status string
	if err := store.DB.QueryRowContext(ctx, `SELECT status FROM cycles WHERE cycle_id = ?`, cycleID).Scan(&status); err != nil {
		t.Fatalf("query cycle: %v", err)
	}
	if status != "completed" {
		t.Fatalf("cycle status = %q, want completed", status)
	}
}
