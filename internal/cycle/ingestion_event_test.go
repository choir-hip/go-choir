package cycle

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/sources"
)

func TestSaveIngestionEventsRejectsPromptBarOrigin(t *testing.T) {
	store, err := NewStorage(filepath.Join(t.TempDir(), "sourcecycled.db"))
	if err != nil {
		t.Fatalf("new storage: %v", err)
	}
	defer store.Close()

	err = store.SaveIngestionEvents(context.Background(), []IngestionEvent{{
		EventID:    "ingestionevt_prompt",
		CycleID:    "cycle_prompt",
		ArtifactID: "srcitem_prompt",
		SourceID:   "rss:test",
		Origin:     IngestionOriginPromptBar,
		CreatedAt:  time.Now().UTC(),
	}})
	if err == nil {
		t.Fatal("expected prompt-bar ingestion event to be rejected")
	}
}

func TestBuildSourceMaxxHandoffRequiresIngestionEvents(t *testing.T) {
	now := time.Date(2026, 6, 10, 8, 0, 0, 0, time.UTC)
	items := []sources.Item{{
		ID:         "srcitem_rss_1",
		SourceID:   "rss:bbc_world",
		SourceType: sources.SourceTypeRSS,
		Title:      "Story",
		Region:     "global",
	}}
	handoff := BuildSourceMaxxHandoff("cycle_1", items, nil, now)
	if len(handoff.ProcessorRequests) != 0 {
		t.Fatalf("expected no processor requests without ingestion events, got %+v", handoff.ProcessorRequests)
	}
	events := BuildIngestionEventsFromItems("cycle_1", items, now)
	handoff = BuildSourceMaxxHandoff("cycle_1", items, events, now)
	if len(handoff.ProcessorRequests) != 1 || len(handoff.ProcessorRequests[0].IngestionEventIDs) != 1 {
		t.Fatalf("expected processor request with ingestion event ids, got %+v", handoff.ProcessorRequests)
	}
}

func TestValidateProcessorRequestIngestionEvents(t *testing.T) {
	ctx := context.Background()
	store, err := NewStorage(filepath.Join(t.TempDir(), "sourcecycled.db"))
	if err != nil {
		t.Fatalf("new storage: %v", err)
	}
	defer store.Close()

	cycleID := "cycle_validate"
	item := sources.Item{ID: "srcitem_1", SourceID: "rss:test", SourceType: sources.SourceTypeRSS, Title: "Story"}
	events := BuildIngestionEventsFromItems(cycleID, []sources.Item{item}, time.Now().UTC())
	if err := store.SaveIngestionEvents(ctx, events); err != nil {
		t.Fatalf("save ingestion events: %v", err)
	}
	okReq := ProcessorRequest{
		CycleID:           cycleID,
		SourceItemIDs:     []string{"srcitem_1"},
		IngestionEventIDs: []string{events[0].EventID},
	}
	ok, err := store.ValidateProcessorRequestIngestionEvents(ctx, okReq)
	if err != nil || !ok {
		t.Fatalf("ValidateProcessorRequestIngestionEvents(valid) = %v, %v", ok, err)
	}
	badReq := ProcessorRequest{CycleID: cycleID, SourceItemIDs: []string{"srcitem_1"}}
	ok, err = store.ValidateProcessorRequestIngestionEvents(ctx, badReq)
	if err != nil || ok {
		t.Fatalf("ValidateProcessorRequestIngestionEvents(missing ids) = %v, %v", ok, err)
	}
}

func TestRSSGDELTCurriculumEmitsIngestionEventsAndProcessorHandoff(t *testing.T) {
	ctx := context.Background()
	store, err := NewStorage(filepath.Join(t.TempDir(), "sourcecycled.db"))
	if err != nil {
		t.Fatalf("new storage: %v", err)
	}
	defer store.Close()

	now := time.Date(2026, 6, 10, 9, 0, 0, 0, time.UTC)
	cycleID := "cycle_curriculum"
	items := []sources.Item{
		{
			ID:         "srcitem_rss_hn",
			SourceID:   "rss:hn_best",
			SourceType: sources.SourceTypeRSS,
			FetchID:    "fetch_rss_1",
			Title:      "HN story",
			Region:     "global",
		},
		{
			ID:         "srcitem_gdelt_1",
			SourceID:   "gdelt:15min",
			SourceType: sources.SourceTypeGDELT,
			FetchID:    "fetch_gdelt_1",
			Title:      "GDELT mention",
			Region:     "global",
		},
	}
	events := BuildIngestionEventsFromItems(cycleID, items, now)
	if len(events) != 2 {
		t.Fatalf("ingestion events = %d, want 2", len(events))
	}
	if err := store.SaveIngestionEvents(ctx, events); err != nil {
		t.Fatalf("save ingestion events: %v", err)
	}
	handoff := BuildSourceMaxxHandoff(cycleID, items, events, now)
	if len(handoff.ProcessorRequests) != 2 {
		t.Fatalf("processor requests = %d, want rss + gdelt routes", len(handoff.ProcessorRequests))
	}
	for _, req := range handoff.ProcessorRequests {
		if len(req.IngestionEventIDs) == 0 {
			t.Fatalf("processor request missing ingestion refs: %+v", req)
		}
		ok, err := store.ValidateProcessorRequestIngestionEvents(ctx, req)
		if err != nil || !ok {
			t.Fatalf("validate processor request %+v: ok=%v err=%v", req, ok, err)
		}
	}
}
