package platform

// The floor (docs/problems/most-new-stories-show-no-related-stories-2026-10-27.md;
// owner, 2026-10-27: "There should always be 3+ related stories. Just a
// question of how related they are. But never no related stories."),
// written before the code. Ways it could fail:
//
//  F1 A story with no strong link still answers [] while the paper has
//     other stories.
//  F2 A weak link outranks a strong one, or displaces it, or the floor
//     drops a strong link to make room.
//  F3 The floor links the story to itself or lists a story twice (a strong
//     link repeated as a weak one).
//  F4 The weak tier ignores relatedness: an unrelated recent story is
//     taken over one that shares the story's terms; stopwords or short
//     words make everything "similar".
//  F5 The floor pads past MinLinks (it is a floor, not a fill), or past the
//     limit.
//  F6 With the floor off (CORPUSD_RELATED_FLOOR=off, or MinLinks 0) the
//     answer changes; with it on, the cache serves an answer computed with
//     it off (the mode, part of the key, must differ).
//  F7 The pool read runs on the request's 400 ms budget, so a cold pool
//     overruns and the story answers empty and partial.
//  F8 An opinion piece is offered as a related story.

import (
	"strings"
	"testing"
	"time"
)

func floorStory(id, title, section string, age time.Duration, now time.Time) FrontStory {
	return FrontStory{PublicationID: id, Title: title, Section: section, UpdatedAt: now.Add(-age),
		Route: FrontRoute{Bare: "/pub/texture/" + id, Permalink: "/pub/texture/" + id + "/v1"}}
}

func floorIDs(links []RelatedLink) []string {
	out := make([]string, len(links))
	for i, l := range links {
		out[i] = l.PublicationID
	}
	return out
}

func TestFloorFillsToMinLinksPreferringSharedTerms(t *testing.T) { // F1, F4, F5
	now := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	tune := defaultRelatedTuning()
	self := floorStory("self", "Israelis vote amid uncertainty over Netanyahu's future", "world", time.Hour, now)
	pool := []FrontStory{
		self,
		floorStory("r1", "Walter Koenig, Star Trek actor, dies at 90", "", 5*time.Minute, now),
		floorStory("r2", "Instagram announces long-form video feature", "tech_ai", 6*time.Minute, now),
		floorStory("e1", "Israel votes as polls point to Netanyahu coalition losing majority", "world", 3*time.Hour, now),
		floorStory("e2", "Israel's election campaign sidelines Palestinian future", "world", 5*time.Hour, now),
		floorStory("r3", "Valkyries lead WNBA finals after game 5 win", "", 7*time.Minute, now),
	}
	got := floorRelated(self, nil, pool, now, tune, 3)
	ids := floorIDs(got)
	if len(got) != 3 {
		t.Fatalf("floor of 3 from a pool of 5 others: got %v", ids)
	}
	if ids[0] != "e1" || ids[1] != "e2" {
		t.Fatalf("the two stories sharing the election terms must come first: %v", ids)
	}
	if got[0].Reason != relatedReasonSimilar || got[0].Shared.Terms < 2 {
		t.Fatalf("a term match is reason similar with its shared terms counted: %+v", got[0])
	}
	if got[2].Reason != relatedReasonRecent || got[2].Shared.Terms != 0 {
		t.Fatalf("the third, with no shared term, is the newest other story, reason recent: %+v", got[2])
	}
	if ids[2] != "r1" {
		t.Fatalf("with no shared term the newest story fills: %v", ids)
	}
}

func TestFloorKeepsStrongLinksFirstAndNeverRepeatsOrSelfLinks(t *testing.T) { // F2, F3
	now := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	tune := defaultRelatedTuning()
	self := floorStory("self", "Harbor reopens after storm", "world", time.Hour, now)
	strong := RelatedLink{FrontStory: floorStory("s1", "Ferries resume", "world", 2*time.Hour, now), Reason: relatedReasonCapture}
	pool := []FrontStory{
		self,
		strong.FrontStory,
		floorStory("h1", "Harbor storm damage counted", "world", time.Hour, now),
		floorStory("x1", "Unrelated market news", "business", time.Minute, now),
	}
	got := floorRelated(self, []RelatedLink{strong}, pool, now, tune, 3)
	ids := floorIDs(got)
	if strings.Join(ids, ",") != "s1,h1,x1" {
		t.Fatalf("strong link first, then the term match, then the newest; no repeats, no self: %v", ids)
	}
	full := []RelatedLink{strong, {FrontStory: pool[2], Reason: relatedReasonEntity}, {FrontStory: pool[3], Reason: relatedReasonEntity}}
	if got := floorRelated(self, full, pool, now, tune, 3); len(got) != 3 || got[0].Reason != relatedReasonCapture || got[2].Reason != relatedReasonEntity {
		t.Fatalf("three strong links already meet the floor; the list is unchanged: %v", floorIDs(got))
	}
}

func TestFloorStopwordsAndShortWordsAreNotSimilarity(t *testing.T) { // F4
	now := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	tune := defaultRelatedTuning()
	self := floorStory("self", "The war in the east and the talks of it", "", time.Hour, now)
	pool := []FrontStory{
		self,
		floorStory("old", "Glaciers retreat in the Alps", "", 30*time.Minute, now),
		floorStory("new", "And so it was in the end of it all", "", time.Minute, now),
	}
	got := floorRelated(self, nil, pool, now, tune, 1)
	if len(got) != 1 || got[0].Reason != relatedReasonRecent || got[0].PublicationID != "new" {
		t.Fatalf("stopwords alone are no match; the newest fills as recent: %+v", got)
	}
}

func TestFloorSkipsOpinionAndStopsAtThePool(t *testing.T) { // F8, F5
	now := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	tune := defaultRelatedTuning()
	self := floorStory("self", "Budget vote delayed", "", time.Hour, now)
	op := floorStory("op", "Budget vote delayed again, and why", "", time.Minute, now)
	op.Kind = "opinion"
	pool := []FrontStory{self, op, floorStory("a", "Budget talks resume", "", time.Hour, now)}
	got := floorRelated(self, nil, pool, now, tune, 3)
	if strings.Join(floorIDs(got), ",") != "a" {
		t.Fatalf("an opinion piece is not a related story, and the floor takes what exists: %v", floorIDs(got))
	}
}

func TestRelatedFloorThroughTheWalk(t *testing.T) { // F1, F6, F7
	l := newRealisticLab(t)
	l.svc.related.tuning.MinLinks = 3
	// The production budget: the pool read must not count against it.
	l.svc.related.tuning.Budget = 400 * time.Millisecond
	l.publish("doc-e1", 1, "Israel votes as polls point to Netanyahu coalition losing majority")
	l.publish("doc-e2", 1, "Israel's election campaign sidelines Palestinian future")
	self := l.publish("doc-self", 1, "Israelis vote amid uncertainty over Netanyahu's future")

	got := l.related(self.RoutePath)
	if len(got.Links) != 3 {
		t.Fatalf("a story with no strong link gets the floor of 3: %v", relatedRoutes(got))
	}
	if !strings.HasSuffix(got.Basis.Mode, relatedModeFloorSuffix) {
		t.Fatalf("the mode names the floor (it keys the cache): %q", got.Basis.Mode)
	}
	if got.Links[0].Reason != relatedReasonSimilar || !strings.Contains(got.Links[0].Title, "Netanyahu") {
		t.Fatalf("the shared-terms story leads: %+v", got.Links[0])
	}

	t.Setenv(relatedFloorEnv, "off")
	off := l.related(self.RoutePath)
	if len(off.Links) != 0 || strings.HasSuffix(off.Basis.Mode, relatedModeFloorSuffix) {
		t.Fatalf("the switch off restores the strong-only answer and mode: %v %q", relatedRoutes(off), off.Basis.Mode)
	}
}
