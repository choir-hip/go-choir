package cycle

import (
	"context"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/sources"
)

func TestSaveIngestionEventsRejectsPromptBarOrigin(t *testing.T) {
	store := openTestStorage(t)
	defer store.Close()

	err := store.SaveIngestionEvents(context.Background(), []IngestionEvent{{
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


func TestValidateProcessorRequestIngestionEvents(t *testing.T) {
	ctx := context.Background()
	store := openTestStorage(t)
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

