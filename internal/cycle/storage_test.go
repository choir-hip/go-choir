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

func TestSearchItemsTokenizesNaturalQueriesAndRanksMatches(t *testing.T) {
	ctx := context.Background()
	store, err := NewStorage(filepath.Join(t.TempDir(), "sourcecycled.db"))
	if err != nil {
		t.Fatalf("open storage: %v", err)
	}
	defer store.Close()

	now := time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)
	items := []sources.Item{
		{
			ID:        "srcitem_economy_inflation",
			SourceID:  "gdelt:15min",
			Title:     "GDELT Event: macro update",
			Body:      "Themes: EPU_ECONOMY; inflation; employment report.",
			Published: now,
			FetchedAt: now,
		},
		{
			ID:        "srcitem_economy_only",
			SourceID:  "gdelt:15min",
			Title:     "GDELT Event: economy",
			Body:      "Themes: EPU_ECONOMY_HISTORIC.",
			Published: now.Add(time.Hour),
			FetchedAt: now.Add(time.Hour),
		},
	}
	if err := store.SaveItems(items); err != nil {
		t.Fatalf("save items: %v", err)
	}

	results, err := store.SearchItems(ctx, "economy inflation GDP employment 2026", 10)
	if err != nil {
		t.Fatalf("search items: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("search results = %d, want 2: %+v", len(results), results)
	}
	if results[0].ID != "srcitem_economy_inflation" {
		t.Fatalf("top result = %s, want multi-term match first: %+v", results[0].ID, results)
	}
}

func TestSearchItemsResolvesDurableSourceItemHandles(t *testing.T) {
	ctx := context.Background()
	store, err := NewStorage(filepath.Join(t.TempDir(), "sourcecycled.db"))
	if err != nil {
		t.Fatalf("open storage: %v", err)
	}
	defer store.Close()

	now := time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)
	items := []sources.Item{
		{
			ID:        "srcitem_target_handle",
			SourceID:  "gdelt:15min",
			Title:     "Target event",
			Body:      "Body does not contain the durable id.",
			Published: now,
			FetchedAt: now,
		},
		{
			ID:        "srcitem_other_handle",
			SourceID:  "rss:other",
			Title:     "Other event",
			Body:      "Also does not contain the target id.",
			Published: now.Add(time.Minute),
			FetchedAt: now.Add(time.Minute),
		},
	}
	if err := store.SaveItems(items); err != nil {
		t.Fatalf("save items: %v", err)
	}

	for _, query := range []string{
		"srcitem_target_handle",
		"source_service_item:srcitem_target_handle",
		"Please inspect srcitem_target_handle before publication.",
	} {
		results, err := store.SearchItems(ctx, query, 10)
		if err != nil {
			t.Fatalf("search items for %q: %v", query, err)
		}
		if len(results) != 1 || results[0].ID != "srcitem_target_handle" {
			t.Fatalf("search %q results = %+v, want exact durable handle", query, results)
		}
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

func TestBuildSourceMaxxHandoffRoutesSourceItemsToProcessorsAndReconciler(t *testing.T) {
	now := time.Date(2026, 6, 7, 10, 30, 0, 0, time.UTC)
	items := []sources.Item{
		{
			ID:         "srcitem_gdelt_supply",
			SourceID:   "gdelt:15min",
			SourceType: sources.SourceTypeGDELT,
			Title:      "Port disruption mention",
			Verticals:  []string{"supply_chain"},
			Region:     "global",
		},
		{
			ID:         "srcitem_rss_supply",
			SourceID:   "rss:logistics",
			SourceType: sources.SourceTypeRSS,
			Title:      "Carrier advisory",
			Verticals:  []string{"supply_chain"},
			Region:     "global",
		},
		{
			ID:         "srcitem_telegram_conflict",
			SourceID:   "telegram:conflict",
			SourceType: sources.SourceTypeTelegram,
			Title:      "Field report",
			Verticals:  []string{"conflict"},
			Region:     "mena",
		},
	}

	handoff := BuildSourceMaxxHandoff("cycle_source_maxx", items, now)
	if len(handoff.ProcessorRequests) != 3 {
		t.Fatalf("processor requests = %d, want one per source-class route: %+v", len(handoff.ProcessorRequests), handoff.ProcessorRequests)
	}
	if len(handoff.ReconcilerRequests) != 1 {
		t.Fatalf("reconciler requests = %d, want 1", len(handoff.ReconcilerRequests))
	}
	for _, req := range handoff.ProcessorRequests {
		if req.Status != "queued" || req.CycleID != "cycle_source_maxx" || req.ContinuityRef == "" {
			t.Fatalf("processor request missing durable handoff fields: %+v", req)
		}
		if len(req.SourceItemIDs) == 0 || req.Prompt == "" {
			t.Fatalf("processor request missing source handles or prompt: %+v", req)
		}
	}
	foundGDELT := false
	for _, req := range handoff.ProcessorRequests {
		if req.ProcessorKey == "processor:global_firehose:global:gdelt" {
			foundGDELT = true
		}
	}
	if !foundGDELT {
		t.Fatalf("GDELT route missing global firehose processor: %+v", handoff.ProcessorRequests)
	}
	reconciler := handoff.ReconcilerRequests[0]
	if reconciler.Scope != "story-corpus" || len(reconciler.SourceItemIDs) != 3 || len(reconciler.ProcessorRequestIDs) != len(handoff.ProcessorRequests) {
		t.Fatalf("unexpected reconciler request: %+v", reconciler)
	}
}

func TestStoragePersistsSourceMaxxHandoffsAndLatestCycleSummary(t *testing.T) {
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
	now := time.Date(2026, 6, 7, 11, 0, 0, 0, time.UTC)
	handoff := BuildSourceMaxxHandoff(cycleID, []sources.Item{{
		ID:         "srcitem_ai_policy",
		SourceID:   "rss:ai_policy",
		SourceType: sources.SourceTypeRSS,
		Title:      "AI policy update",
		Verticals:  []string{"ai", "policy"},
		Region:     "us",
	}}, now)
	if err := store.SaveProcessorRequests(ctx, handoff.ProcessorRequests); err != nil {
		t.Fatalf("save processor requests: %v", err)
	}
	if err := store.SaveReconcilerRequests(ctx, handoff.ReconcilerRequests); err != nil {
		t.Fatalf("save reconciler requests: %v", err)
	}
	if err := store.FinishCycle(ctx, cycleID, "completed", 1, 1, nil); err != nil {
		t.Fatalf("finish cycle: %v", err)
	}

	summary, err := store.LatestCycleSummary(ctx)
	if err != nil {
		t.Fatalf("latest cycle summary: %v", err)
	}
	if summary.CycleID != cycleID || summary.ItemCount != 1 || summary.FetchCount != 1 {
		t.Fatalf("unexpected cycle summary: %+v", summary)
	}
	if len(summary.ProcessorRequests) != 1 || summary.ProcessorRequests[0].ProcessorKey != "processor:ai:us:rss" {
		t.Fatalf("unexpected processor summary: %+v", summary.ProcessorRequests)
	}
	if err := store.UpdateProcessorRequestRuntimeRun(ctx, summary.ProcessorRequests[0].RequestID, "submitted", "processor-run-1"); err != nil {
		t.Fatalf("update processor runtime run: %v", err)
	}
	if len(summary.ReconcilerRequests) != 1 || summary.ReconcilerRequests[0].Scope != "story-corpus" {
		t.Fatalf("unexpected reconciler summary: %+v", summary.ReconcilerRequests)
	}
	if err := store.UpdateReconcilerRequestRuntimeRun(ctx, summary.ReconcilerRequests[0].RequestID, "submitted", "reconciler-run-1"); err != nil {
		t.Fatalf("update reconciler runtime run: %v", err)
	}
	summary, err = store.LatestCycleSummary(ctx)
	if err != nil {
		t.Fatalf("latest cycle summary after runtime run ids: %v", err)
	}
	if summary.ProcessorRequests[0].RuntimeRunID != "processor-run-1" || summary.ReconcilerRequests[0].RuntimeRunID != "reconciler-run-1" {
		t.Fatalf("runtime run ids not persisted: processors=%+v reconcilers=%+v", summary.ProcessorRequests, summary.ReconcilerRequests)
	}
}
