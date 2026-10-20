package transaction

import (
	"testing"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/visualcapture"
)

// A capture job's output must not make Freeze reject the candidate: the
// classifier rejects unknown paths at commit time (the v7 decision), and the
// evidence directory is a new place in the capsule. It belongs to the
// artifact ledger beside the release tree, in either path spelling the diff
// uses.
func TestVisualEvidencePathsClassifyAsArtifactNotUnknown(t *testing.T) {
	classifier := NewClassifier()
	for _, prefix := range []string{"", "/"} {
		changes := []capsule.FileChange{
			{Path: prefix + visualcapture.EvidencePrefix + visualcapture.ManifestFile, Kind: capsule.ChangeAdded, Mode: 0o644},
			{Path: prefix + visualcapture.EvidencePrefix + repeatHex('a') + ".png", Kind: capsule.ChangeAdded, Mode: 0o644},
		}
		result := classifier.Classify(changes)
		if result.HasUnknown() {
			t.Fatalf("evidence under %q is unknown, so Freeze would reject the candidate: %+v", prefix, result.Unknown)
		}
		if len(result.Groups[LedgerArtifact]) != 2 {
			t.Fatalf("evidence under %q groups as %+v, want both in the artifact ledger", prefix, result.Groups)
		}
	}
}
