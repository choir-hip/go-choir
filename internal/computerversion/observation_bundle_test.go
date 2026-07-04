package computerversion

import (
	"context"
	"testing"
)

func TestCombineObservationSetsBundlesBaseVMAndPromotionEvidenceUnderOneComputerVersion(t *testing.T) {
	version := observationBundleComputerVersion()
	baseSet, vmSet, promotionSet := observationBundleFixture(t, version)

	combined, err := CombineObservationSets("fixture-level combined evidence", version, baseSet, vmSet, promotionSet)
	if err != nil {
		t.Fatalf("combine observation sets: %v", err)
	}

	if combined.Name != "fixture-level combined evidence" {
		t.Fatalf("name = %q", combined.Name)
	}
	if combined.Version != version {
		t.Fatalf("version = %#v, want %#v", combined.Version, version)
	}
	assertObservationBundleKinds(t, combined.Required, []ObservationKind{
		ObservationBlobSet,
		ObservationFileManifest,
		ObservationPromotionCertificate,
		ObservationVMStateManifest,
	})
	assertObservationBundleKinds(t, combined.RequiredKinds(), []ObservationKind{
		ObservationBlobSet,
		ObservationFileManifest,
		ObservationPromotionCertificate,
		ObservationVMStateManifest,
	})

	wantObservations := []Observation{
		mustFindObservationBundleObservation(t, baseSet, ObservationBlobSet),
		mustFindObservationBundleObservation(t, baseSet, ObservationFileManifest),
		mustFindObservationBundleObservation(t, promotionSet, ObservationPromotionCertificate),
		mustFindObservationBundleObservation(t, vmSet, ObservationVMStateManifest),
	}
	assertObservationBundleObservations(t, combined.Observations, wantObservations)
}

func TestCombineObservationSetsComparesEquivalentWhenInputsAreSuppliedInDifferentOrder(t *testing.T) {
	version := observationBundleComputerVersion()
	baseSet, vmSet, promotionSet := observationBundleFixture(t, version)

	ordered, err := CombineObservationSets("ordered", version, baseSet, vmSet, promotionSet)
	if err != nil {
		t.Fatalf("combine ordered observation sets: %v", err)
	}
	shuffled, err := CombineObservationSets("shuffled", version, promotionSet, vmSet, baseSet)
	if err != nil {
		t.Fatalf("combine shuffled observation sets: %v", err)
	}

	result := (EquivalenceChecker{}).CheckObservationSets(ordered, shuffled)
	if !result.Equivalent() {
		t.Fatalf("expected equivalent combined observation sets, got %#v", result)
	}
}

func TestCombineObservationSetsRejectsDifferentComputerVersionInput(t *testing.T) {
	version := observationBundleComputerVersion()
	foreignVersion := ComputerVersion{CodeRef: version.CodeRef, ArtifactProgramRef: "tape:owner/other-computer"}

	matching := ObservationSet{
		Name:    "matching-files",
		Version: version,
		Observations: []Observation{
			FileManifestObservation("/notes/today.md", "sha256:file-manifest"),
		},
	}
	foreign := ObservationSet{
		Name:    "foreign-files",
		Version: foreignVersion,
		Observations: []Observation{
			FileManifestObservation("/notes/today.md", "sha256:file-manifest"),
		},
	}

	if _, err := CombineObservationSets("mixed versions", version, matching, foreign); err == nil {
		t.Fatal("expected different ComputerVersion input to be rejected")
	}
}

func TestCombineObservationSetsDedupesIdenticalDuplicateAndRejectsConflictingDuplicate(t *testing.T) {
	version := observationBundleComputerVersion()
	shared := Observation{Kind: ObservationBlobSet, Key: "blob:shared", Value: `{"blob_ref":"blob:shared","sha256":"aaa"}`}
	left := ObservationSet{
		Name:         "left-blob",
		Version:      version,
		Required:     []ObservationKind{ObservationBlobSet},
		Observations: []Observation{shared},
	}
	identical := ObservationSet{
		Name:         "identical-blob",
		Version:      version,
		Required:     []ObservationKind{ObservationBlobSet},
		Observations: []Observation{shared},
	}

	combined, err := CombineObservationSets("dedupe duplicate", version, left, identical)
	if err != nil {
		t.Fatalf("combine identical duplicate observations: %v", err)
	}
	assertObservationBundleKinds(t, combined.Required, []ObservationKind{ObservationBlobSet})
	assertObservationBundleObservations(t, combined.Observations, []Observation{shared})

	conflicting := ObservationSet{
		Name:    "conflicting-blob",
		Version: version,
		Observations: []Observation{{
			Kind:  shared.Kind,
			Key:   shared.Key,
			Value: `{"blob_ref":"blob:shared","sha256":"bbb"}`,
		}},
	}
	if _, err := CombineObservationSets("conflicting duplicate", version, left, conflicting); err == nil {
		t.Fatal("expected duplicate kind/key with a different value to be rejected")
	}
}

func observationBundleComputerVersion() ComputerVersion {
	return ComputerVersion{CodeRef: "git:combined-observation", ArtifactProgramRef: "tape:owner/combined@cursor-7"}
}

func observationBundleFixture(t *testing.T, version ComputerVersion) (ObservationSet, ObservationSet, ObservationSet) {
	t.Helper()

	baseSet := observationBundleBaseSet(t, version)
	vmSet, err := vmManagerBoundaryPath().ObservationSet("bundle-vmmanager", version)
	if err != nil {
		t.Fatalf("vmmanager observation set: %v", err)
	}
	certificate := promotionCertificateFixture()
	certificate.Candidate = version
	promotionSet, err := certificate.ObservationSet("bundle-promotion")
	if err != nil {
		t.Fatalf("promotion observation set: %v", err)
	}

	return baseSet, vmSet, promotionSet
}

func observationBundleBaseSet(t *testing.T, version ComputerVersion) ObservationSet {
	t.Helper()

	blobs := newBaseBlobStore(t, t.TempDir())
	ref, contentHash := putBaseBlob(t, blobs, []byte("combined observation base blob"))
	journal := newSQLiteJournalWithEvent(t, baseCreateEventWithBlob(7, ref, contentHash))
	observations, err := BaseCurrentStateObservationSet(context.Background(), "bundle-base", version, journal, blobs)
	if err != nil {
		t.Fatalf("base current state observation set: %v", err)
	}
	return observations
}

func mustFindObservationBundleObservation(t *testing.T, set ObservationSet, kind ObservationKind) Observation {
	t.Helper()

	for _, observation := range set.Observations {
		if observation.Kind == kind {
			return observation
		}
	}
	t.Fatalf("observation kind %q missing from %#v", kind, set.Observations)
	return Observation{}
}

func assertObservationBundleKinds(t *testing.T, got, want []ObservationKind) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("observation kinds = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("observation kinds = %#v, want %#v", got, want)
		}
	}
}

func assertObservationBundleObservations(t *testing.T, got, want []Observation) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("observations = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("observations = %#v, want %#v", got, want)
		}
	}
}
