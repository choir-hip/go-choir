package yaegikernel

import (
	"strings"
	"testing"
)

// Failure mode F7 of docs/problems/oped-not-connected-2026-10-20.md: research
// is not told that SearchWireCorpus also finds published stories and opinion
// pieces, or that a piece is cited only after its read_url is Read into a
// capture, so it would cite a snippet.
func TestSearchWireCorpusSummaryTellsResearchToReadAPublishedPiece(t *testing.T) {
	summary := choirFunctionDocs["SearchWireCorpus"].Summary
	for _, want := range []string{"published", "opinion", "read_url", "Read", "capture_id"} {
		if !strings.Contains(summary, want) {
			t.Fatalf("SearchWireCorpus summary lacks %q: %s", want, summary)
		}
	}
	if !strings.Contains(summary, "never cite") {
		t.Fatalf("the summary must forbid citing a snippet alone: %s", summary)
	}
}
