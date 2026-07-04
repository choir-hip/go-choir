package computerversion

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProductFixtureRootObservationSetCombinesProvisionedFixtureEvidence(t *testing.T) {
	version := productFixtureRootComputerVersion()
	fixture := productFixtureRootFixture(t, version)
	if fixture.ObjectGraph != nil || fixture.DoltHead != nil {
		t.Fatalf("fixture optional evidence = object_graph:%#v dolt_head:%#v, want nil for base product fixture evidence", fixture.ObjectGraph, fixture.DoltHead)
	}

	observations, err := fixture.ObservationSet(context.Background(), "product fixture")
	if err != nil {
		t.Fatalf("product fixture observations: %v", err)
	}
	if observations.Name != "product fixture" {
		t.Fatalf("name = %q", observations.Name)
	}
	if observations.Version != version {
		t.Fatalf("version = %#v, want %#v", observations.Version, version)
	}
	wantKinds := []ObservationKind{
		ObservationBlobSet,
		ObservationFileManifest,
		ObservationPromotionCertificate,
		ObservationVMStateManifest,
	}
	assertObservationBundleKinds(t, observations.Required, wantKinds)
	assertObservationBundleKinds(t, observations.RequiredKinds(), wantKinds)
	assertProductFixtureObservationKinds(t, observations.Observations, wantKinds)
}

func TestProductFixtureRootObservationSetIncludesObjectGraphHeadWhenProvided(t *testing.T) {
	version := productFixtureRootComputerVersion()
	fixture := productFixtureRootFixture(t, version)
	objectGraph := objectGraphSnapshotFixtureForVersion(t, version)
	fixture.ObjectGraph = &objectGraph

	observations, err := fixture.ObservationSet(context.Background(), "product fixture with objectgraph")
	if err != nil {
		t.Fatalf("product fixture observations: %v", err)
	}

	wantKinds := []ObservationKind{
		ObservationBlobSet,
		ObservationFileManifest,
		ObservationObjectGraphHead,
		ObservationPromotionCertificate,
		ObservationVMStateManifest,
	}
	assertObservationBundleKinds(t, observations.Required, wantKinds)
	assertObservationBundleKinds(t, observations.RequiredKinds(), wantKinds)
	assertProductFixtureObservationKinds(t, observations.Observations, wantKinds)

	objectGraphObservation := observations.Observations[2]
	if objectGraphObservation.Kind != ObservationObjectGraphHead || objectGraphObservation.Key != "objectgraph:head" {
		t.Fatalf("object graph observation = %#v, want objectgraph:head", objectGraphObservation)
	}
	payload := decodeObjectGraphHeadPayload(t, objectGraphObservation.Value)
	if payload.ObjectCount != len(objectGraph.Objects) || payload.EdgeCount != len(objectGraph.Edges) {
		t.Fatalf("object graph head counts = objects:%d edges:%d, want objects:%d edges:%d", payload.ObjectCount, payload.EdgeCount, len(objectGraph.Objects), len(objectGraph.Edges))
	}
}

func TestProductFixtureRootObservationSetIncludesDoltHeadWhenProvided(t *testing.T) {
	version := productFixtureRootComputerVersion()
	fixture := productFixtureRootFixture(t, version)
	objectGraph := objectGraphSnapshotFixtureForVersion(t, version)
	fixture.DoltHead = &DoltHeadSnapshot{
		RepoRoot:    t.TempDir(),
		Database:    "objectgraph",
		CommitHash:  "dolt-product-fixture-head",
		ObjectGraph: &objectGraph,
	}

	observations, err := fixture.ObservationSet(context.Background(), "product fixture with dolt")
	if err != nil {
		t.Fatalf("product fixture observations: %v", err)
	}

	wantKinds := []ObservationKind{
		ObservationBlobSet,
		ObservationDoltHead,
		ObservationFileManifest,
		ObservationPromotionCertificate,
		ObservationVMStateManifest,
	}
	assertObservationBundleKinds(t, observations.Required, wantKinds)
	assertObservationBundleKinds(t, observations.RequiredKinds(), wantKinds)
	assertProductFixtureObservationKinds(t, observations.Observations, wantKinds)

	doltObservation := observations.Observations[1]
	if doltObservation.Kind != ObservationDoltHead || doltObservation.Key != "dolt:objectgraph:head" {
		t.Fatalf("dolt observation = %#v, want dolt:objectgraph:head", doltObservation)
	}
	doltPayload := decodeDoltHeadPayload(t, doltObservation.Value)
	objectGraphPayload := decodeObjectGraphHeadPayload(t, mustObjectGraphCanonicalHead(t, objectGraph))
	if doltPayload.CommitHash != "dolt-product-fixture-head" || doltPayload.ObjectGraphHead != objectGraphPayload.Head || doltPayload.ObjectCount != objectGraphPayload.ObjectCount || doltPayload.EdgeCount != objectGraphPayload.EdgeCount {
		t.Fatalf("dolt payload = commit:%q head:%q objects:%d edges:%d, want commit dolt-product-fixture-head head:%q objects:%d edges:%d", doltPayload.CommitHash, doltPayload.ObjectGraphHead, doltPayload.ObjectCount, doltPayload.EdgeCount, objectGraphPayload.Head, objectGraphPayload.ObjectCount, objectGraphPayload.EdgeCount)
	}
}

func TestProductFixtureRootObservationSetRejectsPromotionCandidateMismatchBeforeEvidence(t *testing.T) {
	version := productFixtureRootComputerVersion()
	foreignCandidate := ComputerVersion{CodeRef: "git:other-candidate", ArtifactProgramRef: "tape:org/other-candidate"}
	fixture := ProductFixtureRoot{
		Version: version,
		Base: BaseCurrentStatePaths{
			JournalPath: filepath.Join(t.TempDir(), "missing-base.sqlite"),
			BlobRoot:    filepath.Join(t.TempDir(), "missing-blobs"),
		},
		VM:        vmManagerBoundaryPath(),
		Promotion: productFixtureRootPromotionCertificate(foreignCandidate),
	}

	observations, err := fixture.ObservationSet(context.Background(), "candidate mismatch")
	if err == nil || !strings.Contains(err.Error(), "promotion candidate does not match fixture version") {
		t.Fatalf("expected promotion candidate mismatch, got observations %#v error %v", observations, err)
	}
	assertNoProductFixtureObservations(t, observations)
}

func TestProductFixtureRootObservationSetRejectsMissingBaseRootsWithoutCreatingThem(t *testing.T) {
	version := productFixtureRootComputerVersion()

	t.Run("journal", func(t *testing.T) {
		root := t.TempDir()
		blobRoot := filepath.Join(root, "blobs")
		newBaseBlobStore(t, blobRoot)
		missingJournalPath := filepath.Join(root, "missing-base.sqlite")
		fixture := productFixtureRootWithBase(version, BaseCurrentStatePaths{
			JournalPath: missingJournalPath,
			BlobRoot:    blobRoot,
		})

		observations, err := fixture.ObservationSet(context.Background(), "missing base journal")
		if err == nil || !strings.Contains(err.Error(), "base current state source: open journal") {
			t.Fatalf("expected missing journal root rejection, got observations %#v error %v", observations, err)
		}
		assertNoProductFixtureObservations(t, observations)
		assertPathDoesNotExist(t, missingJournalPath)
	})

	t.Run("blob root", func(t *testing.T) {
		root := t.TempDir()
		journalPath := filepath.Join(root, "base.sqlite")
		journal := newSQLiteJournalAtPathWithEvent(t, journalPath, baseCreateEvent(1, "a"))
		if err := journal.Close(); err != nil {
			t.Fatalf("close writable journal: %v", err)
		}
		missingBlobRoot := filepath.Join(root, "missing-blobs")
		fixture := productFixtureRootWithBase(version, BaseCurrentStatePaths{
			JournalPath: journalPath,
			BlobRoot:    missingBlobRoot,
		})

		observations, err := fixture.ObservationSet(context.Background(), "missing base blobs")
		if err == nil || !strings.Contains(err.Error(), "base current state source: open blob store") {
			t.Fatalf("expected missing blob root rejection, got observations %#v error %v", observations, err)
		}
		assertNoProductFixtureObservations(t, observations)
		assertPathDoesNotExist(t, missingBlobRoot)
	})
}

func TestProductFixtureRootObservationSetRejectsVMFixtureWithoutPersistentDirOrDataImage(t *testing.T) {
	version := productFixtureRootComputerVersion()
	fixture := productFixtureRootFixture(t, version)
	fixture.VM.PersistentDir = ""
	fixture.VM.DataImagePath = ""

	observations, err := fixture.ObservationSet(context.Background(), "missing vm state path")
	if err == nil || !strings.Contains(err.Error(), "persistent dir or data image path is required") {
		t.Fatalf("expected missing vm state path rejection, got observations %#v error %v", observations, err)
	}
	assertNoProductFixtureObservations(t, observations)
}

func productFixtureRootComputerVersion() ComputerVersion {
	return ComputerVersion{CodeRef: "git:product-fixture-root", ArtifactProgramRef: "tape:org/product-fixture-root@cursor-11"}
}

func productFixtureRootFixture(t *testing.T, version ComputerVersion) ProductFixtureRoot {
	t.Helper()
	return productFixtureRootWithBase(version, productFixtureRootBasePaths(t))
}

func productFixtureRootWithBase(version ComputerVersion, base BaseCurrentStatePaths) ProductFixtureRoot {
	return ProductFixtureRoot{
		Version:   version,
		Base:      base,
		VM:        vmManagerBoundaryPath(),
		Promotion: productFixtureRootPromotionCertificate(version),
	}
}

func productFixtureRootBasePaths(t *testing.T) BaseCurrentStatePaths {
	t.Helper()
	root := t.TempDir()
	blobRoot := filepath.Join(root, "blobs")
	blobs := newBaseBlobStore(t, blobRoot)
	ref, contentHash := putBaseBlob(t, blobs, []byte("product fixture root base blob"))
	journalPath := filepath.Join(root, "base.sqlite")
	journal := newSQLiteJournalAtPathWithEvent(t, journalPath, baseCreateEventWithBlob(11, ref, contentHash))
	if err := journal.Close(); err != nil {
		t.Fatalf("close writable journal: %v", err)
	}
	return BaseCurrentStatePaths{JournalPath: journalPath, BlobRoot: blobRoot}
}

func productFixtureRootPromotionCertificate(candidate ComputerVersion) PromotionCertificate {
	certificate := promotionCertificateFixture()
	certificate.Candidate = candidate
	return certificate
}

func assertProductFixtureObservationKinds(t *testing.T, observations []Observation, want []ObservationKind) {
	t.Helper()
	if len(observations) != len(want) {
		t.Fatalf("observations = %#v, want kinds %#v", observations, want)
	}
	for i, kind := range want {
		if observations[i].Kind != kind {
			t.Fatalf("observation kinds = %#v, want %#v", observationKinds(observations), want)
		}
	}
}

func observationKinds(observations []Observation) []ObservationKind {
	kinds := make([]ObservationKind, 0, len(observations))
	for _, observation := range observations {
		kinds = append(kinds, observation.Kind)
	}
	return kinds
}

func assertNoProductFixtureObservations(t *testing.T, observations ObservationSet) {
	t.Helper()
	if observations.Name != "" || observations.Version.Valid() || len(observations.Required) != 0 || len(observations.Observations) != 0 {
		t.Fatalf("rejected product fixture emitted observations: %#v", observations)
	}
}

func assertPathDoesNotExist(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected %s not to exist, got stat error %v", path, err)
	}
}
