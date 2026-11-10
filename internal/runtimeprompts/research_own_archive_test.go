package runtimeprompts

import (
	"strings"
	"testing"
)

// Research finds the paper's own earlier stories on a subject
// (docs/problems/published-search-recall-2026-11-10.md): before the web it
// searches the archive, reads the ones that bear on the subject, and cites a
// story's capture only for what that story itself established. The overlay
// stays inside its size budget (the v2 dedupe took it from 13.1k to about 5k).
func TestRLMResearchOverlayFindsAndReadsTheOwnArchive(t *testing.T) {
	overlay := RLMResearchOverlay()
	at := strings.Index(overlay, "Research Cell Surface")
	if at < 0 {
		t.Fatalf("overlay has no cell surface")
	}
	surface := overlay[at:]
	for _, want := range []string{"earlier stories", "SearchWireCorpus", "choir.Read", "read_url"} {
		if !strings.Contains(surface, want) {
			t.Errorf("research overlay does not say %q about the paper's own earlier stories", want)
		}
	}
	// The sentence must not teach research to rank or weigh by any reliance link.
	for _, absent := range []string{"relies_on", "materiality"} {
		if strings.Contains(overlay, absent) {
			t.Errorf("research overlay names %q: the reliance link and score are never shown to a desk", absent)
		}
	}
	if len(overlay) > 8000 {
		t.Errorf("research overlay is %d bytes, the budget is 8000", len(overlay))
	}
}
