package computerversion

import (
	"context"
	"strings"
	"testing"
)

func TestBaseCurrentStateAndFileProjectionMaterializersCompare(t *testing.T) {
	version := baseSliceComputerVersion()
	blobs := newBaseBlobStore(t, t.TempDir())
	ref, contentHash := putBaseBlob(t, blobs, []byte("projection current state"))
	jr := newSQLiteJournalWithEvent(t, baseCreateEventWithBlob(1, ref, contentHash))

	currentState, err := BaseCurrentStateObservationSet(context.Background(), "base-current-state", version, jr, blobs)
	if err != nil {
		t.Fatalf("current-state observations: %v", err)
	}
	currentRealization, err := (ProjectionMaterializer{ID: "base-current-state-reader", Observations: currentState}).Materialize(
		context.Background(),
		version,
		BaseCurrentStateCapabilityManifest("base-current-state-reader", "base-sqlite-journal-blob"),
	)
	if err != nil {
		t.Fatalf("current-state materialize: %v", err)
	}

	fileProjectionSet := currentState
	fileProjectionSet.Name = "base-file-projection"
	fileProjection, err := (ProjectionMaterializer{ID: "base-file-projection", Observations: fileProjectionSet}).Materialize(
		context.Background(),
		version,
		BaseCurrentStateCapabilityManifest("base-file-projection", "non-firecracker-file-projection"),
	)
	if err != nil {
		t.Fatalf("file projection materialize: %v", err)
	}

	result := EquivalenceChecker{}.CheckRealizations(currentRealization, fileProjection)
	if !result.Equivalent() {
		t.Fatalf("expected current-state and file projection to compare equivalent, got %#v", result)
	}
}

func TestBaseCurrentStateProjectionMismatchFails(t *testing.T) {
	version := baseSliceComputerVersion()
	blobs := newBaseBlobStore(t, t.TempDir())
	ref, contentHash := putBaseBlob(t, blobs, []byte("projection mismatch source"))
	jr := newSQLiteJournalWithEvent(t, baseCreateEventWithBlob(1, ref, contentHash))

	currentState, err := BaseCurrentStateObservationSet(context.Background(), "base-current-state", version, jr, blobs)
	if err != nil {
		t.Fatalf("current-state observations: %v", err)
	}
	currentRealization, err := (ProjectionMaterializer{ID: "base-current-state-reader", Observations: currentState}).Materialize(
		context.Background(),
		version,
		BaseCurrentStateCapabilityManifest("base-current-state-reader", "base-sqlite-journal-blob"),
	)
	if err != nil {
		t.Fatalf("current-state materialize: %v", err)
	}

	mismatchedProjectionSet := currentState
	mismatchedProjectionSet.Name = "base-file-projection-mismatch"
	mismatchedProjectionSet.Observations = append([]Observation{}, currentState.Observations...)
	for i := range mismatchedProjectionSet.Observations {
		if mismatchedProjectionSet.Observations[i].Kind == ObservationBlobSet {
			mismatchedProjectionSet.Observations[i].Value = strings.Replace(mismatchedProjectionSet.Observations[i].Value, contentHash, strings.Repeat("0", 64), 1)
			break
		}
	}
	fileProjection, err := (ProjectionMaterializer{ID: "base-file-projection", Observations: mismatchedProjectionSet}).Materialize(
		context.Background(),
		version,
		BaseCurrentStateCapabilityManifest("base-file-projection", "non-firecracker-file-projection"),
	)
	if err != nil {
		t.Fatalf("file projection materialize: %v", err)
	}

	result := EquivalenceChecker{}.CheckRealizations(currentRealization, fileProjection)
	if result.Status != EquivalenceNotEquivalent {
		t.Fatalf("expected projection mismatch to fail equivalence, got %#v", result)
	}
}

func TestBaseCurrentStateCapabilityManifestNarrowsUnsupportedLiveProcess(t *testing.T) {
	manifest := BaseCurrentStateCapabilityManifest("base-current-state-reader", "base-sqlite-journal-blob")
	missing := manifest.MissingRequired([]ObservationKind{ObservationFileManifest, ObservationBlobSet, ObservationLiveProcessContinuity})
	if len(missing) != 1 || missing[0].Kind != ObservationLiveProcessContinuity {
		t.Fatalf("missing = %#v, want live-process continuity only", missing)
	}
}
