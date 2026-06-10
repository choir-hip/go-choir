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
		BodyKind:        sources.BodyKindSourceBody,
		BodyLength:      len("Rates held steady."),
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
	if results[0].BodyKind != sources.BodyKindSourceBody || results[0].BodyLength != len("Rates held steady.") || results[0].ReaderSnapshot {
		t.Fatalf("body classification did not persist: %+v", results[0])
	}
	if results[0].StoreBodyPolicy != "bounded_release_text" || results[0].SourceAuthPolicy != "none" {
		t.Fatalf("source policy fields did not resolve: %+v", results[0])
	}
}

func TestStorageDerivesBodyClassificationForLegacyRows(t *testing.T) {
	ctx := context.Background()
	store, err := NewStorage(filepath.Join(t.TempDir(), "sourcecycled.db"))
	if err != nil {
		t.Fatalf("open storage: %v", err)
	}
	defer store.Close()

	now := time.Date(2026, 6, 8, 8, 0, 0, 0, time.UTC)
	item := sources.Item{
		ID:         "srcitem_legacy_rss",
		SourceID:   "rss:legacy",
		SourceType: sources.SourceTypeRSS,
		Title:      "Legacy feed item",
		Body:       "A feed summary survived from before body classification columns were populated.",
		Published:  now,
		FetchedAt:  now,
	}
	if err := store.SaveItems([]sources.Item{item}); err != nil {
		t.Fatalf("save items: %v", err)
	}
	if _, err := store.DB.ExecContext(ctx, `UPDATE items SET body_kind = '', body_length = 0, reader_snapshot = 0 WHERE id = ?`, item.ID); err != nil {
		t.Fatalf("simulate legacy row: %v", err)
	}

	got, err := store.GetItem(ctx, item.ID)
	if err != nil {
		t.Fatalf("get item: %v", err)
	}
	if got.BodyKind != sources.BodyKindFeedSummary || got.BodyLength != len([]rune(item.Body)) || got.ReaderSnapshot {
		t.Fatalf("derived classification = %+v", got)
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

func TestStorageSupersedesQueuedProcessorContinuityAndDependentReconcilers(t *testing.T) {
	ctx := context.Background()
	store, err := NewStorage(filepath.Join(t.TempDir(), "sourcecycled.db"))
	if err != nil {
		t.Fatalf("open storage: %v", err)
	}
	defer store.Close()

	oldCycle, err := store.StartCycle(ctx)
	if err != nil {
		t.Fatalf("start old cycle: %v", err)
	}
	newCycle, err := store.StartCycle(ctx)
	if err != nil {
		t.Fatalf("start new cycle: %v", err)
	}
	oldTime := time.Date(2026, 6, 8, 7, 0, 0, 0, time.UTC)
	newTime := oldTime.Add(15 * time.Minute)
	continuityRef := "sourcecycled://processor/processor:technology:global:rss/latest"
	oldQueued := ProcessorRequest{
		RequestID:     "processor_old_queued",
		CycleID:       oldCycle,
		ProcessorKey:  "processor:technology:global:rss",
		Status:        "queued",
		SourceItemIDs: []string{"srcitem_old"},
		ContinuityRef: continuityRef,
		Prompt:        "old queued",
		CreatedAt:     oldTime,
		UpdatedAt:     oldTime,
	}
	oldSubmitted := ProcessorRequest{
		RequestID:     "processor_old_submitted",
		CycleID:       oldCycle,
		ProcessorKey:  "processor:technology:global:rss",
		Status:        "submitted",
		RuntimeRunID:  "run-old-submitted",
		SourceItemIDs: []string{"srcitem_old_submitted"},
		ContinuityRef: continuityRef,
		Prompt:        "old submitted",
		CreatedAt:     oldTime,
		UpdatedAt:     oldTime,
	}
	otherQueued := ProcessorRequest{
		RequestID:     "processor_other_queued",
		CycleID:       oldCycle,
		ProcessorKey:  "processor:finance:global:rss",
		Status:        "queued",
		SourceItemIDs: []string{"srcitem_other"},
		ContinuityRef: "sourcecycled://processor/processor:finance:global:rss/latest",
		Prompt:        "other queued",
		CreatedAt:     oldTime,
		UpdatedAt:     oldTime,
	}
	newQueued := ProcessorRequest{
		RequestID:     "processor_new_queued",
		CycleID:       newCycle,
		ProcessorKey:  "processor:technology:global:rss",
		Status:        "queued",
		SourceItemIDs: []string{"srcitem_new"},
		ContinuityRef: continuityRef,
		Prompt:        "new queued",
		CreatedAt:     newTime,
		UpdatedAt:     newTime,
	}
	if err := store.SaveProcessorRequests(ctx, []ProcessorRequest{oldQueued, oldSubmitted, otherQueued, newQueued}); err != nil {
		t.Fatalf("save processors: %v", err)
	}
	reconciler := ReconcilerRequest{
		RequestID:           "reconciler_old",
		CycleID:             oldCycle,
		Status:              "queued",
		Scope:               "story-corpus",
		ProcessorRequestIDs: []string{oldQueued.RequestID, otherQueued.RequestID},
		Prompt:              "old reconciler",
		CreatedAt:           oldTime,
		UpdatedAt:           oldTime,
	}
	if err := store.SaveReconcilerRequests(ctx, []ReconcilerRequest{reconciler}); err != nil {
		t.Fatalf("save reconciler: %v", err)
	}

	supersededProcessors, err := store.SupersedeQueuedProcessorRequests(ctx, []ProcessorRequest{newQueued})
	if err != nil {
		t.Fatalf("supersede processors: %v", err)
	}
	if supersededProcessors != 1 {
		t.Fatalf("superseded processors = %d, want 1", supersededProcessors)
	}
	supersededReconcilers, err := store.SupersedeQueuedReconcilersWithSupersededProcessors(ctx)
	if err != nil {
		t.Fatalf("supersede reconcilers: %v", err)
	}
	if supersededReconcilers != 1 {
		t.Fatalf("superseded reconcilers = %d, want 1", supersededReconcilers)
	}

	processors, err := store.ListProcessorRequests(ctx, "", 20)
	if err != nil {
		t.Fatalf("list processors: %v", err)
	}
	statusByID := map[string]string{}
	for _, req := range processors {
		statusByID[req.RequestID] = req.Status
	}
	if statusByID[oldQueued.RequestID] != "superseded" {
		t.Fatalf("old queued status = %q, want superseded; all=%+v", statusByID[oldQueued.RequestID], statusByID)
	}
	if statusByID[oldSubmitted.RequestID] != "submitted" {
		t.Fatalf("old submitted status = %q, want submitted; all=%+v", statusByID[oldSubmitted.RequestID], statusByID)
	}
	if statusByID[otherQueued.RequestID] != "queued" || statusByID[newQueued.RequestID] != "queued" {
		t.Fatalf("unrelated/new queued statuses wrong: %+v", statusByID)
	}
	reconcilers, err := store.ListReconcilerRequests(ctx, oldCycle, 10)
	if err != nil {
		t.Fatalf("list reconcilers: %v", err)
	}
	if len(reconcilers) != 1 || reconcilers[0].Status != "superseded" {
		t.Fatalf("dependent reconciler should be superseded: %+v", reconcilers)
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

	handoff := BuildSourceMaxxHandoff("cycle_source_maxx", items, BuildIngestionEventsFromItems("cycle_source_maxx", items, now), now)
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
	item := sources.Item{
		ID:         "srcitem_ai_policy",
		SourceID:   "rss:ai_policy",
		SourceType: sources.SourceTypeRSS,
		Title:      "AI policy update",
		Verticals:  []string{"ai", "policy"},
		Region:     "us",
	}
	events := BuildIngestionEventsFromItems(cycleID, []sources.Item{item}, now)
	if err := store.SaveIngestionEvents(ctx, events); err != nil {
		t.Fatalf("save ingestion events: %v", err)
	}
	handoff := BuildSourceMaxxHandoff(cycleID, []sources.Item{item}, events, now)
	if err := store.SaveProcessorRequests(ctx, handoff.ProcessorRequests); err != nil {
		t.Fatalf("save processor requests: %v", err)
	}
	if err := store.SaveReconcilerRequests(ctx, handoff.ReconcilerRequests); err != nil {
		t.Fatalf("save reconciler requests: %v", err)
	}
	fetch := sources.NewFetchRecord(sources.Source{ID: "rss:ai_policy", Type: sources.SourceTypeRSS, URL: "https://example.test/feed"}, "https://example.test/feed", now)
	fetch.Status = "ok"
	fetch.StatusCode = 200
	fetch.ItemCount = 1
	fetch.EndedAt = now.Add(time.Second)
	if err := store.SaveCycleFetches(cycleID, []sources.FetchRecord{fetch}); err != nil {
		t.Fatalf("save cycle fetches: %v", err)
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
	if len(summary.Fetches) != 1 || summary.Fetches[0].SourceID != "rss:ai_policy" {
		t.Fatalf("unexpected cycle fetches: %+v", summary.Fetches)
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
