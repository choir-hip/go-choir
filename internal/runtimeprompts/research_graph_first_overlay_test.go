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
		`The counts are hints, not verdicts: you decide from the pages you read whether the corpus holds the answer, you go to the web when it does not, and you write "missing:" only for a lack you established`,
		"Search the web whenever the graph does not hold what the question needs; what to stop is rewording a query against pages you already hold",
		"A detail you cannot confirm is a finding to report, not a task to keep searching for",
		"a reworded query that returns the pages you already hold tells you this wording does not find it; say what you looked for and stop",
		"The platform holds a story that cites fewer than two outlets; the second outlet may come from the graph's event record (its outlets and capture ids), not only the web",
		"Worked cell (graph, then the web for what is missing",
		"decide from the pages you read whether the corpus holds the answer; report when a second independent outlet supports the claim",
		"outlets read without error:",
		`its notes carry "adds: ..." (what this adds to the corpus) and "missing: ..." (what the corpus lacked, so you went to the web, or "missing: none")`,
	} {
		if !strings.Contains(flat, want) {
			t.Errorf("research overlay lacks %q", want)
		}
	}
	for _, absent := range []string{"do not search again", "report now only if two independent outlets", "missing: what the graph lacked", "SearchFresh", "A search is not sent while you hold pages", "next round of searches", "Search, read and send an interim packet in one cell", "choir.Search(query, true)"} {
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
	if !strings.Contains(cell, "choir.ReadCapture(c.CaptureID, 0, 0)") {
		t.Error("the worked cell reads the graph's pages by capture id")
	}
	// The stop rule sits above the worked cell. The cell decides nothing from the counts or
	// from word matches (whether pages answer a question is the model's judgement; three review
	// rounds on a lexical gate in the example ended that): it prints the counts and the note,
	// and its web search keeps the control flow it had.
	if rule, cellAt := strings.Index(overlay, "A detail you cannot confirm is a finding"), strings.Index(overlay, "Worked cell"); rule < 0 || cellAt < 0 || rule > cellAt {
		t.Errorf("the stop rule (at %d) must come before the worked cell (at %d)", rule, cellAt)
	}
	block, _, _ := strings.Cut(strings.TrimPrefix(cell, "```go\n"), "```")
	for _, want := range []string{"captures_matched_words", "words_note", "g.Matched", "g.WordsNote"} {
		if !strings.Contains(block, want) {
			t.Errorf("the worked cell does not print the corpus counts: lacks %q", want)
		}
	}
	for _, absent := range []string{"lacks :=", "answered", "strings.Contains", `"missing:`} {
		if strings.Contains(block, absent) {
			t.Errorf("the worked cell decides a lack in code (%q)", absent)
		}
	}
	if n := strings.Count(overlay, "choir.SearchAll("); n != 1 {
		t.Errorf("the worked cell sends one wording to one SearchAll call (found %d calls)", n)
	}
}
