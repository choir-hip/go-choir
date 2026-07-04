package computerversion

import "testing"

func TestComputerVersionValidRequiresBothRefs(t *testing.T) {
	valid := ComputerVersion{CodeRef: "git:abc123", ArtifactProgramRef: "tape:owner/main"}
	if !valid.Valid() {
		t.Fatal("valid computer version rejected")
	}

	missingCode := valid
	missingCode.CodeRef = ""
	if missingCode.Valid() {
		t.Fatal("computer version without code ref accepted")
	}

	missingProgram := valid
	missingProgram.ArtifactProgramRef = " "
	if missingProgram.Valid() {
		t.Fatal("computer version without artifact program ref accepted")
	}
}

func TestEquivalenceCheckerFileManifestPasses(t *testing.T) {
	version := ComputerVersion{CodeRef: "git:abc123", ArtifactProgramRef: "tape:owner/main"}
	left := Realization{
		ID:      "firecracker-fixture",
		Version: version,
		Capabilities: CapabilityManifest{
			Materializer: "firecracker-fixture",
			Substrate:    "firecracker",
			Supported:    []ObservationKind{ObservationFileManifest},
		},
		Observations: ObservationSet{
			Name:     "left-files",
			Version:  version,
			Required: []ObservationKind{ObservationFileManifest},
			Observations: []Observation{
				FileManifestObservation("/notes/today.md", "sha256:aaa"),
				FileManifestObservation("/notes/tomorrow.md", "sha256:bbb"),
			},
		},
	}
	right := left
	right.ID = "file-projection-fixture"
	right.Capabilities = CapabilityManifest{
		Materializer: "file-projection-fixture",
		Substrate:    "file-projection",
		Supported:    []ObservationKind{ObservationFileManifest},
	}

	result := EquivalenceChecker{}.CheckRealizations(left, right)
	if !result.Equivalent() {
		t.Fatalf("expected equivalent result, got %#v", result)
	}
}

func TestEquivalenceCheckerSeededMismatchFails(t *testing.T) {
	version := ComputerVersion{CodeRef: "git:abc123", ArtifactProgramRef: "tape:owner/main"}
	manifest := CapabilityManifest{
		Materializer: "fixture",
		Substrate:    "file-projection",
		Supported:    []ObservationKind{ObservationFileManifest},
	}
	left := Realization{
		ID:           "left",
		Version:      version,
		Capabilities: manifest,
		Observations: ObservationSet{
			Name:     "left-files",
			Version:  version,
			Required: []ObservationKind{ObservationFileManifest},
			Observations: []Observation{
				FileManifestObservation("/notes/today.md", "sha256:aaa"),
			},
		},
	}
	right := left
	right.ID = "right"
	right.Observations.Name = "right-files"
	right.Observations.Observations = []Observation{
		FileManifestObservation("/notes/today.md", "sha256:bbb"),
	}

	result := EquivalenceChecker{}.CheckRealizations(left, right)
	if result.Status != EquivalenceNotEquivalent {
		t.Fatalf("expected not_equivalent status, got %#v", result)
	}
	if len(result.Differences) != 1 {
		t.Fatalf("expected one difference, got %#v", result.Differences)
	}
	diff := result.Differences[0]
	if diff.Kind != ObservationFileManifest || diff.Key != "/notes/today.md" || diff.Left != "sha256:aaa" || diff.Right != "sha256:bbb" {
		t.Fatalf("unexpected difference: %#v", diff)
	}
}

func TestEquivalenceCheckerUnsupportedCapabilityNarrows(t *testing.T) {
	version := ComputerVersion{CodeRef: "git:abc123", ArtifactProgramRef: "tape:owner/main"}
	left := Realization{
		ID:      "left",
		Version: version,
		Capabilities: CapabilityManifest{
			Materializer: "left-materializer",
			Substrate:    "firecracker",
			Supported:    []ObservationKind{ObservationFileManifest},
		},
		Observations: ObservationSet{
			Name:     "left-files",
			Version:  version,
			Required: []ObservationKind{ObservationFileManifest},
			Observations: []Observation{
				FileManifestObservation("/notes/today.md", "sha256:aaa"),
			},
		},
	}
	right := left
	right.ID = "right"
	right.Capabilities = CapabilityManifest{
		Materializer: "right-materializer",
		Substrate:    "container",
		Supported:    nil,
		Unsupported: []UnsupportedCapability{{
			Kind:   ObservationFileManifest,
			Reason: "container fixture does not expose a durable file manifest",
		}},
	}

	result := EquivalenceChecker{}.CheckRealizations(left, right)
	if result.Status != EquivalenceNarrowed {
		t.Fatalf("expected narrowed status, got %#v", result)
	}
	if len(result.Unsupported) != 1 || result.Unsupported[0].Kind != ObservationFileManifest {
		t.Fatalf("expected file manifest unsupported capability, got %#v", result.Unsupported)
	}
}

func TestEquivalenceCheckerDifferentComputerVersionsFail(t *testing.T) {
	leftVersion := ComputerVersion{CodeRef: "git:abc123", ArtifactProgramRef: "tape:owner/main"}
	rightVersion := ComputerVersion{CodeRef: "git:def456", ArtifactProgramRef: "tape:owner/main"}
	left := ObservationSet{
		Version:      leftVersion,
		Observations: []Observation{FileManifestObservation("/notes/today.md", "sha256:aaa")},
	}
	right := ObservationSet{
		Version:      rightVersion,
		Observations: []Observation{FileManifestObservation("/notes/today.md", "sha256:aaa")},
	}

	result := EquivalenceChecker{}.CheckObservationSets(left, right)
	if result.Status != EquivalenceNotEquivalent {
		t.Fatalf("expected not_equivalent status, got %#v", result)
	}
}
