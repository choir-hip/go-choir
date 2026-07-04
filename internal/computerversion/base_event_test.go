package computerversion

import (
	"context"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/base/model"
	basetree "github.com/yusefmosiah/go-choir/internal/base/tree"
)

func TestBaseEventExtractorOrdersTapeByCursor(t *testing.T) {
	version := baseSliceComputerVersion()
	ordered := BaseEventExtractor{Events: []model.Event{
		baseCreateEvent(1, "a"),
		baseUpdateEvent(2, "b"),
	}}
	shuffled := BaseEventExtractor{Events: []model.Event{
		baseUpdateEvent(2, "b"),
		baseCreateEvent(1, "a"),
	}}

	leftSet, err := ordered.Extract(context.Background(), ExtractRequest{Name: "ordered", Version: version})
	if err != nil {
		t.Fatalf("ordered extract: %v", err)
	}
	rightSet, err := shuffled.Extract(context.Background(), ExtractRequest{Name: "shuffled", Version: version})
	if err != nil {
		t.Fatalf("shuffled extract: %v", err)
	}
	manifest := CapabilityManifest{
		Materializer: "base-event-extractor",
		Substrate:    "base-event-tape",
		Supported:    []ObservationKind{ObservationFileManifest},
	}

	result := EquivalenceChecker{}.CheckRealizations(
		Realization{ID: "ordered", Version: version, Capabilities: manifest, Observations: leftSet},
		Realization{ID: "shuffled", Version: version, Capabilities: manifest, Observations: rightSet},
	)
	if !result.Equivalent() {
		t.Fatalf("expected cursor-ordered event extracts to match, got %#v", result)
	}
}

func TestBaseEventExtractorSeededTapeMismatchFails(t *testing.T) {
	version := baseSliceComputerVersion()
	leftSet, err := BaseEventJournalObservationSet("left", version, []model.Event{
		baseCreateEvent(1, "a"),
		baseUpdateEvent(2, "b"),
	})
	if err != nil {
		t.Fatalf("left extract: %v", err)
	}
	rightSet, err := BaseEventJournalObservationSet("right", version, []model.Event{
		baseCreateEvent(1, "a"),
		baseUpdateEvent(2, "c"),
	})
	if err != nil {
		t.Fatalf("right extract: %v", err)
	}
	manifest := CapabilityManifest{
		Materializer: "base-event-extractor",
		Substrate:    "base-event-tape",
		Supported:    []ObservationKind{ObservationFileManifest},
	}

	result := EquivalenceChecker{}.CheckRealizations(
		Realization{ID: "left", Version: version, Capabilities: manifest, Observations: leftSet},
		Realization{ID: "right", Version: version, Capabilities: manifest, Observations: rightSet},
	)
	if result.Status != EquivalenceNotEquivalent {
		t.Fatalf("expected seeded tape mismatch to fail, got %#v", result)
	}
	if len(result.Differences) != 1 || result.Differences[0].Key != "base_item_notes" {
		t.Fatalf("expected one notes item difference, got %#v", result.Differences)
	}
}

func TestBaseEventExtractorRejectsInvalidTape(t *testing.T) {
	invalid := baseCreateEvent(0, "a")
	if _, err := BaseEventJournalObservationSet("invalid", baseSliceComputerVersion(), []model.Event{invalid}); err == nil {
		t.Fatal("expected non-committed cursor to be rejected")
	}

	duplicateCursor := []model.Event{baseCreateEvent(1, "a"), baseUpdateEvent(1, "b")}
	if _, err := BaseEventJournalObservationSet("duplicate", baseSliceComputerVersion(), duplicateCursor); err == nil {
		t.Fatal("expected duplicate cursor to be rejected")
	}
}

func baseCreateEvent(seq int64, hashSuffix string) model.Event {
	hash := strings.Repeat(hashSuffix, 64)
	payload := basetree.Payload{
		Name:         "notes.md",
		ParentItemID: "base_item_root",
		Kind:         model.KindFile,
		VersionID:    model.VersionID("base_ver_notes_create_" + hashSuffix),
		BlobRef:      model.BlobRef("sha256:" + hash),
		ContentHash:  hash,
	}
	return model.Event{
		EventID:     model.EventID("base_evt_create_" + hashSuffix),
		OwnerID:     "owner",
		ItemID:      "base_item_notes",
		DeviceID:    "dev1",
		SubjectID:   "user1",
		EventType:   model.EventCreate,
		Kind:        model.KindFile,
		CursorSeq:   seq,
		PayloadJSON: payload.JSON(),
		CreatedAt:   baseSliceTime,
	}
}

func baseUpdateEvent(seq int64, hashSuffix string) model.Event {
	hash := strings.Repeat(hashSuffix, 64)
	payload := basetree.Payload{
		VersionID:   model.VersionID("base_ver_notes_update_" + hashSuffix),
		BlobRef:     model.BlobRef("sha256:" + hash),
		ContentHash: hash,
	}
	return model.Event{
		EventID:     model.EventID("base_evt_update_" + hashSuffix),
		OwnerID:     "owner",
		ItemID:      "base_item_notes",
		DeviceID:    "dev1",
		SubjectID:   "user1",
		EventType:   model.EventUpdate,
		Kind:        model.KindFile,
		CursorSeq:   seq,
		PayloadJSON: payload.JSON(),
		CreatedAt:   baseSliceTime,
	}
}
