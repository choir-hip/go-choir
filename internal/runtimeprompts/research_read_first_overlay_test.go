package runtimeprompts

import (
	"strings"
	"testing"
)

// The research desk reads before it searches again (owner, 2026-11-21: "on
// search, we should be reading much more and searching less";
// docs/design/semantic-layer-2026-11-20.md section 4). The overlay's job: a
// stop rule, a worked cell that sends one wording, reads its top pages and
// stops on a test, and the one way past a refusal. Failure list, written
// before the edit: the overlay still sends the desk "with the next round of
// searches"; its worked cell still teaches two wordings; it has no stop rule;
// it does not name SearchFresh or say what a refusal lists.
func TestResearchOverlayHasTheStopRuleAndTheOneWordingWorkedCell(t *testing.T) {
	overlay := RLMResearchOverlay()
	flat := strings.Join(strings.Fields(overlay), " ")
	for _, want := range []string{
		"your objective is met when you have read a page from a second independent outlet that supports the claim. Then report; do not search again",
		"A search is not sent while you hold pages you have not read",
		"the top pages of your earlier searches and the pages the corpus brief listed",
		"choir.SearchFresh(query, why)",
		"at least 20 characters, is recorded",
		"if two of these outlets support the claim, report now and do not search again",
		"outlets read:",
	} {
		if !strings.Contains(flat, want) {
			t.Errorf("research overlay lacks %q", want)
		}
	}
	for _, absent := range []string{"next round of searches", "Search, read and send an interim packet in one cell", "cabinet changes official statement", "choir.Search(query, true)"} {
		if strings.Contains(flat, absent) {
			t.Errorf("research overlay still says %q", absent)
		}
	}
	// The worked cell reads the first five results as listed and counts outlets by
	// registrable domain, not by hostname.
	for _, want := range []string{"(h.Held && h.CaptureID == \"\") || len(urls)+len(caps) == 5", "outlets[o] = true", "choir.ReadCapture(id, 0, 0)"} {
		if !strings.Contains(overlay, want) {
			t.Errorf("the worked cell lacks %q", want)
		}
	}
	// The worked cell sends one wording to SearchAll.
	if n := strings.Count(overlay, "choir.SearchAll("); n != 1 || !strings.Contains(overlay, `[]string{"ministry cabinet changes"})`) {
		t.Errorf("the worked cell must send one wording to one SearchAll call (found %d calls)", n)
	}
}
