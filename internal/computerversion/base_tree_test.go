package computerversion

import (
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/base/model"
	basetree "github.com/yusefmosiah/go-choir/internal/base/tree"
)

var baseSliceTime = time.Date(2026, 7, 4, 8, 0, 0, 0, time.UTC)

func TestBaseTreeObservationSetFeedsEquivalenceAcrossProjections(t *testing.T) {
	version := baseSliceComputerVersion()
	leftSet, err := BaseTreeObservationSet("base-tree", version, baseTreeFixture("a"))
	if err != nil {
		t.Fatalf("left observation set: %v", err)
	}
	rightSet, err := BaseTreeObservationSet("file-projection", version, baseTreeFixture("a"))
	if err != nil {
		t.Fatalf("right observation set: %v", err)
	}

	left := Realization{
		ID:      "base-tree-projection",
		Version: version,
		Capabilities: CapabilityManifest{
			Materializer: "base-tree-adapter",
			Substrate:    "base-tree",
			Supported:    []ObservationKind{ObservationFileManifest},
		},
		Observations: leftSet,
	}
	right := Realization{
		ID:      "file-manifest-projection",
		Version: version,
		Capabilities: CapabilityManifest{
			Materializer: "file-manifest-adapter",
			Substrate:    "file-projection",
			Supported:    []ObservationKind{ObservationFileManifest},
		},
		Observations: rightSet,
	}

	result := EquivalenceChecker{}.CheckRealizations(left, right)
	if !result.Equivalent() {
		t.Fatalf("expected equivalent base slice projections, got %#v", result)
	}
}

func TestBaseTreeObservationSetSeededCurrentStateMismatchFails(t *testing.T) {
	version := baseSliceComputerVersion()
	leftSet, err := BaseTreeObservationSet("base-tree", version, baseTreeFixture("a"))
	if err != nil {
		t.Fatalf("left observation set: %v", err)
	}
	rightSet, err := BaseTreeObservationSet("file-projection", version, baseTreeFixture("b"))
	if err != nil {
		t.Fatalf("right observation set: %v", err)
	}
	manifest := CapabilityManifest{
		Materializer: "fixture",
		Substrate:    "base-tree",
		Supported:    []ObservationKind{ObservationFileManifest},
	}

	result := EquivalenceChecker{}.CheckRealizations(
		Realization{ID: "left", Version: version, Capabilities: manifest, Observations: leftSet},
		Realization{ID: "right", Version: version, Capabilities: manifest, Observations: rightSet},
	)
	if result.Status != EquivalenceNotEquivalent {
		t.Fatalf("expected mismatch to fail equivalence, got %#v", result)
	}
	if len(result.Differences) != 1 {
		t.Fatalf("expected one base slice difference, got %#v", result.Differences)
	}
	if result.Differences[0].Key != "base_item_notes" {
		t.Fatalf("expected notes item difference, got %#v", result.Differences[0])
	}
}

func TestBaseTreeObservationSetRejectsUnlinkedLiveVersion(t *testing.T) {
	broken := baseTreeFixture("a")
	delete(broken.Versions, model.ItemID("base_item_notes"))

	_, err := BaseTreeObservationSet("broken", baseSliceComputerVersion(), broken)
	if err == nil {
		t.Fatal("expected missing live version to be rejected")
	}
}

func baseSliceComputerVersion() ComputerVersion {
	return ComputerVersion{CodeRef: "git:base-slice", ArtifactProgramRef: "base-journal:owner/main@cursor-2"}
}

func baseTreeFixture(suffix string) basetree.Tree {
	t := basetree.NewTree()
	contentHash := strings.Repeat(suffix, 64)
	t.Items["base_item_notes"] = model.Item{
		ItemID:         "base_item_notes",
		OwnerID:        "owner",
		ParentItemID:   "base_item_root",
		Name:           "notes.md",
		Kind:           model.KindFile,
		CurrentVersion: model.VersionID("base_ver_notes_" + suffix),
		CreatedAt:      baseSliceTime,
		UpdatedAt:      baseSliceTime,
	}
	t.Versions["base_item_notes"] = model.Version{
		VersionID:        model.VersionID("base_ver_notes_" + suffix),
		ItemID:           "base_item_notes",
		BlobRef:          model.BlobRef("sha256:" + contentHash),
		MediaType:        "text/markdown",
		ContentHash:      contentHash,
		ManifestJSON:     `{"mode":"0644","size":12}`,
		ProvenanceJSON:   `{"device":"dev1","subject":"user1"}`,
		CreatedByDevice:  "dev1",
		CreatedBySubject: "user1",
		CreatedAt:        baseSliceTime,
	}
	t.Items["base_item_old"] = model.Item{
		ItemID:       "base_item_old",
		OwnerID:      "owner",
		ParentItemID: "base_item_root",
		Name:         "old.md",
		Kind:         model.KindFile,
		DeletedAt:    &baseSliceTime,
		CreatedAt:    baseSliceTime,
		UpdatedAt:    baseSliceTime,
	}
	return t
}
