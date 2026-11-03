package transaction

import (
	"testing"

	"github.com/yusefmosiah/go-choir/internal/capsule"
)

// Writes under the computer-state layer are scratch: never a Freeze
// rejection and never carried
// (docs/design/capsule-overlay-computer-state-2026-11-03.md, S2 failure mode 7).
func TestClassifierIgnoresTheComputerStateLayer(t *testing.T) {
	result := NewClassifier().Classify([]capsule.FileChange{
		{Path: "computer/files/notes/a.md", Kind: capsule.ChangeModified},
		{Path: "computer/state.db", Kind: capsule.ChangeAdded},
		{Path: "computer/state.db-wal", Kind: capsule.ChangeAdded},
		{Path: "workspace/platform/main.go", Kind: capsule.ChangeModified},
	})
	if len(result.Unknown) != 0 {
		t.Fatalf("unknown = %+v", result.Unknown)
	}
	if len(result.Ignored) != 3 {
		t.Fatalf("ignored = %+v", result.Ignored)
	}
	if got := result.Groups[LedgerSource]; len(got) != 1 || got[0].Path != "workspace/platform/main.go" {
		t.Fatalf("source group = %+v", got)
	}
	// A path that only starts with the word is not the layer.
	if r := NewClassifier().Classify([]capsule.FileChange{{Path: "computerfoo/x", Kind: capsule.ChangeAdded}}); len(r.Unknown) != 1 {
		t.Fatalf("computerfoo/x classified as %+v", r)
	}
}
