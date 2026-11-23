package runtimeprompts

import (
	"strings"
	"testing"
)

// The research overlay states the objective, sends the run to the graph first, and its
// FIRST worked cell does it (owner, 2026-11-22: "we shouldnt need rules. we should be
// able with prompts to encourage models to search the object graph first, and then the
// web"; "the goal of any run shouldnt just be to write the best story, but to be building
// on and using our corpus, and then the story adds to our corpus"). Failure list, written
// before the edit: the overlay still teaches read-first refusals and SearchFresh; its
// worked cell searches the web first; it states no objective of building on the corpus;
// the Report is not told to name what it adds and what the corpus lacked.
func TestResearchOverlayStatesTheObjectiveAndIsGraphFirst(t *testing.T) {
	overlay := RLMResearchOverlay()
	flat := strings.Join(strings.Fields(overlay), " ")
	for _, want := range []string{
		"Your job is to build on and use the paper's corpus, and to leave it better: your Report is an addition to it",
		"The graph is the better first source: its pages are already read, deduplicated, dated, attributed and tied to our stories",
		"Your first message opens with a digest of it",
		"Read what it names by capture id (choir.ReadCapture(id, 0, 0)): no web fetch",
		"search the graph (choir.SearchWireCorpus(query, 10)) before the web",
		"The web is for what the graph lacks: the newest facts, a second independent outlet",
		"Your objective is met when a page from a second independent outlet supports the claim, from the graph or the web; then report",
		"The counts say where to look; the pages you read decide whether the graph holds the answer, and a gap is written only for a lack you established",
		"Search the web whenever the graph does not hold what the question needs; what to stop is rewording a query against pages you already hold",
		"A detail you cannot confirm is a finding to report, not a task to keep searching for",
		"a reworded query that returns the pages you already hold tells you the web lacks it; say what you looked for and stop",
		"The platform holds a story that cites fewer than two outlets; the second outlet may come from the graph's event record (its outlets and capture ids), not only the web",
		"Worked cell (graph, then the web for what is missing",
		"report when a page from a second independent outlet supports the claim",
		"outlets read without error:",
		`its notes carry "adds: ..." (what this adds to the corpus) and "missing: ..." (what the corpus lacked, so you went to the web, or "missing: none")`,
	} {
		if !strings.Contains(flat, want) {
			t.Errorf("research overlay lacks %q", want)
		}
	}
	for _, absent := range []string{"do not search again", "report now only if two independent outlets", "missing: what the graph lacked", "len(outlets) < 2", "SearchFresh", "A search is not sent while you hold pages", "next round of searches", "Search, read and send an interim packet in one cell", "choir.Search(query, true)"} {
		if strings.Contains(flat, absent) {
			t.Errorf("research overlay still says %q", absent)
		}
	}
	// The worked cell reads the graph before it searches the web, and by capture id.
	cell := overlay[strings.Index(overlay, "```go"):]
	graph, web := strings.Index(cell, "choir.SearchWireCorpus("), strings.Index(cell, "choir.SearchAll(")
	if graph < 0 || web < 0 || graph > web {
		t.Errorf("the worked cell must search the graph (at %d) before the web (at %d)", graph, web)
	}
	if !strings.Contains(cell, "choir.ReadCapture(id, 0, 0)") {
		t.Error("the worked cell reads the graph's pages by capture id")
	}
	// The stop rule sits above the worked cell, and the cell sends a web search only for
	// what the run names as lacking, never because the top hits share an outlet.
	if rule, cellAt := strings.Index(overlay, "A detail you cannot confirm is a finding"), strings.Index(overlay, "Worked cell"); rule < 0 || cellAt < 0 || rule > cellAt {
		t.Errorf("the stop rule (at %d) must come before the worked cell (at %d)", rule, cellAt)
	}
	if !strings.Contains(cell, "lacks := \"\"") || !strings.Contains(cell, "if lacks != \"\"") {
		t.Error("the worked cell goes to the web only for a named lack")
	}
	for _, want := range []string{"e.Outlets", "e.CaptureIDs", "captures_matched_words"} {
		if !strings.Contains(cell, want) {
			t.Errorf("the worked cell does not use the graph's event record and counts: lacks %q", want)
		}
	}
	if n := strings.Count(overlay, "choir.SearchAll("); n != 1 {
		t.Errorf("the worked cell sends one wording to one SearchAll call (found %d calls)", n)
	}
}
