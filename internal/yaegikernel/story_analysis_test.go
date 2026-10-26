package yaegikernel

import (
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// The story analysis preload (docs/problems/related-stories-no-entity-layer-
// 2026-10-26.md, slice c0 §4): the host builds a story's entity analysis at
// the research run's first cell and every cell's frame carries it, read by
// choir.StoryAnalysis(). Ways it can fail at the cell, written first:
//
//  G9a Another desk reads it: only the research desk has the accessor.
//  G9b It leaks past its cell: End clears it with the rest of the frame.
//  G9c An unbound cell reads a stale or zero analysis that looks real: with
//      no analysis in the frame the accessor says there is none.

func TestStoryAnalysisBindsFromTheCellFrameAndClearsAtEnd(t *testing.T) { // G9b
	scope, _ := system1Scope(t, SessionRoleResearch, "")
	analysis := &StoryAnalysis{
		Story: StoryAnalysisStory{PublicationID: "pub-0123456789abcdef0123456789abcdef", DocID: "doc-1", RevisionID: "rev-1", Title: "Merger"},
		Candidates: []StoryAnalysisCandidate{{
			EntityCandidate: types.EntityCandidate{Source: types.EntityCandidateExisting, EntityID: "wd:Q312", Name: "Apple", Phrase: "Apple", Count: 3},
			Role:            &StoryAnalysisAnswer{ReceiptRef: "system1:e1", QuestionID: "role-1", Probabilities: map[string]float64{"central": 0.8}, Top: "central"},
		}},
	}
	hooks := scope.BindCell()
	hooks.Begin(SessionFrame{ID: "f1", Source: "cell", Analysis: analysis})
	got := scope.StoryAnalysis()
	if got.Story.PublicationID != analysis.Story.PublicationID || len(got.Candidates) != 1 || got.Candidates[0].Role.QuestionID != "role-1" {
		t.Fatalf("analysis = %+v", got)
	}
	hooks.End()
	if after := scope.StoryAnalysis(); after.Story.PublicationID != "" || len(after.Candidates) != 0 || after.Error == "" {
		t.Fatalf("analysis leaked past the cell: %+v", after)
	}
}

func TestStoryAnalysisAbsentSaysSo(t *testing.T) { // G9c
	scope, _ := system1Scope(t, SessionRoleResearch, "")
	hooks := scope.BindCell()
	hooks.Begin(SessionFrame{ID: "f1", Source: "cell"})
	defer hooks.End()
	got := scope.StoryAnalysis()
	if got.Story.PublicationID != "" || len(got.Candidates) != 0 || !strings.Contains(got.Error, "no story analysis") {
		t.Fatalf("absent analysis = %+v, want an empty one whose error says there is none", got)
	}
}

func TestStoryAnalysisIsExportedToTheResearchDeskOnly(t *testing.T) { // G9a
	cases := []struct {
		role, slot string
		want       bool
	}{
		{SessionRoleResearch, "", true},
		{"management", "", false},
		{"engineering", "implementation", false},
		{"texture", "", false},
		{SessionRoleResearchChild, "", false},
		{SessionRoleManagementTask, "", false},
		{"", "", false},
	}
	for _, tc := range cases {
		scope, _ := system1Scope(t, tc.role, tc.slot)
		if _, got := scope.ChoirExports()["choir/choir"]["StoryAnalysis"]; got != tc.want {
			t.Errorf("role %q slot %q: StoryAnalysis exported = %v, want %v", tc.role, tc.slot, got, tc.want)
		}
	}
}

// The surface teaches Relate's own-story form and the analysis it cites.
func TestRelateSurfaceNamesTheOwnStoryAndTheAnalysis(t *testing.T) {
	doc := choirFunctionDocs["Relate"].Summary
	for _, want := range []string{`""`, "choir.StoryAnalysis()", "novelty"} {
		if !strings.Contains(doc, want) {
			t.Fatalf("Relate's surface doc misses %q: %s", want, doc)
		}
	}
}
