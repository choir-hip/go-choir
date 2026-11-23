package yaegikernel

import (
	"testing"
)

// What a search store keeps of the run's question (search_need.go): a search is tagged
// with the question it answers, and the tag says when it could not be applied. The need
// rules that once used the tags (the 15-unread line of 2026-11-18, the run-wide read-first
// refusal of 2026-11-21) are removed: owner, 2026-11-22: "we shouldnt need rules"; the run
// is given the graph and told to use it first (docs/design/semantic-layer-2026-11-20.md
// section 4). What these tests keep: the tag and the outlet and page counts the run's
// summary line reports.

func TestTagQuestionSaysWhenItCannotTag(t *testing.T) {
	st := newSearchStore()
	if st.tagQuestion("s99", "harbor strike") {
		t.Fatal("tagQuestion of a search the store does not hold must say it did not tag")
	}
	st.apply(SearchFrame{ID: "s1", Kind: SearchFrameStart, Query: "x one"})
	if !st.tagQuestion("s1", "harbor strike") {
		t.Fatal("tagQuestion of a held search must say it tagged")
	}
	var a SearchStats
	a.Add(SearchStats{Untagged: 2})
	a.Add(SearchStats{Untagged: 1})
	if a.Untagged != 3 || a.IsZero() {
		t.Fatalf("stats = %+v, want untagged summed", a)
	}
}
