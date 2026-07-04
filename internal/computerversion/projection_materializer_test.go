package computerversion

import (
	"context"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/base/model"
)

func TestProjectionMaterializerComparesExtractedBaseObservations(t *testing.T) {
	version := baseSliceComputerVersion()
	leftSet, err := BaseEventJournalObservationSet("base-event", version, []model.Event{
		baseCreateEvent(1, "a"),
		baseUpdateEvent(2, "b"),
	})
	if err != nil {
		t.Fatalf("left extract: %v", err)
	}
	rightSet, err := BaseEventJournalObservationSet("file-projection", version, []model.Event{
		baseCreateEvent(1, "a"),
		baseUpdateEvent(2, "b"),
	})
	if err != nil {
		t.Fatalf("right extract: %v", err)
	}

	left, err := (ProjectionMaterializer{ID: "base-event-materializer", Observations: leftSet}).Materialize(context.Background(), version, CapabilityManifest{
		Materializer: "base-event-materializer",
		Substrate:    "base-event-tape",
		Supported:    []ObservationKind{ObservationFileManifest},
	})
	if err != nil {
		t.Fatalf("left materialize: %v", err)
	}
	right, err := (ProjectionMaterializer{ID: "file-projection-materializer", Observations: rightSet}).Materialize(context.Background(), version, CapabilityManifest{
		Materializer: "file-projection-materializer",
		Substrate:    "file-projection",
		Supported:    []ObservationKind{ObservationFileManifest},
	})
	if err != nil {
		t.Fatalf("right materialize: %v", err)
	}

	result := EquivalenceChecker{}.CheckRealizations(left, right)
	if !result.Equivalent() {
		t.Fatalf("expected materialized projections to be equivalent, got %#v", result)
	}
}

func TestProjectionMaterializerMismatchStillFailsChecker(t *testing.T) {
	version := baseSliceComputerVersion()
	leftSet, err := BaseEventJournalObservationSet("base-event", version, []model.Event{
		baseCreateEvent(1, "a"),
		baseUpdateEvent(2, "b"),
	})
	if err != nil {
		t.Fatalf("left extract: %v", err)
	}
	rightSet, err := BaseEventJournalObservationSet("file-projection", version, []model.Event{
		baseCreateEvent(1, "a"),
		baseUpdateEvent(2, "c"),
	})
	if err != nil {
		t.Fatalf("right extract: %v", err)
	}
	manifest := CapabilityManifest{
		Materializer: "projection-materializer",
		Substrate:    "projection",
		Supported:    []ObservationKind{ObservationFileManifest},
	}
	left, err := (ProjectionMaterializer{ID: "left", Observations: leftSet}).Materialize(context.Background(), version, manifest)
	if err != nil {
		t.Fatalf("left materialize: %v", err)
	}
	right, err := (ProjectionMaterializer{ID: "right", Observations: rightSet}).Materialize(context.Background(), version, manifest)
	if err != nil {
		t.Fatalf("right materialize: %v", err)
	}

	result := EquivalenceChecker{}.CheckRealizations(left, right)
	if result.Status != EquivalenceNotEquivalent {
		t.Fatalf("expected mismatch to fail, got %#v", result)
	}
}

func TestProjectionMaterializerRejectsUnsupportedManifest(t *testing.T) {
	version := baseSliceComputerVersion()
	observations, err := BaseEventJournalObservationSet("base-event", version, []model.Event{baseCreateEvent(1, "a")})
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	_, err = (ProjectionMaterializer{ID: "unsupported", Observations: observations}).Materialize(context.Background(), version, CapabilityManifest{
		Materializer: "unsupported",
		Substrate:    "projection",
		Unsupported: []UnsupportedCapability{{
			Kind:   ObservationFileManifest,
			Reason: "projection cannot expose file manifest",
		}},
	})
	if err == nil {
		t.Fatal("expected unsupported manifest to be rejected")
	}
}
