package offlineslice

import (
	"strings"
	"testing"
	"time"
)

func unit(v ...float32) []float32 { return normalize(v) }

func TestDek(t *testing.T) {
	got := Dek("# Title here\n\nFirst paragraph says things. [source 1: A page] More.\n\nSecond.")
	if got != "First paragraph says things. More." {
		t.Fatalf("dek %q", got)
	}
	if Dek("# Only a title") != "" {
		t.Fatal("no paragraph, no dek")
	}
	long := Dek("# T\n\n" + strings.Repeat("word ", 100))
	if len([]rune(long)) > DekRunes+1 {
		t.Fatalf("dek length %d", len([]rune(long)))
	}
}

func mkStories() []Story {
	return []Story{
		{ID: "S1", Versions: []StoryVersion{
			{ID: "S1v1", Created: ts("2026-11-14T00:00:00Z"), Title: "old title", Vecs: [][]float32{unit(1, 0)}},
			{ID: "S1v2", Created: ts("2026-11-16T12:00:00Z"), Title: "new title", Vecs: [][]float32{unit(0, 1)}},
		}},
		{ID: "S2", Versions: []StoryVersion{{ID: "S2v1", Created: ts("2026-11-16T10:00:00Z"), Title: "later story", Vecs: [][]float32{unit(1, 0.1), unit(0, 1)}}}},
	}
}

func TestNearestStoriesUsesOnlyTheVersionThatExistedAtArrival(t *testing.T) {
	st := mkStories()
	// arrival before S1v2 and before S2: only S1v1 exists
	c := Capture{ID: "c", Fetched: ts("2026-11-16T09:00:00Z"), Vec: unit(1, 0)}
	hits := NearestStories(c, st, 5)
	if len(hits) != 1 || hits[0].Story != 0 || hits[0].Version != 0 {
		t.Fatalf("hits %+v", hits)
	}
	// arrival after both: S1 is at v2 (vector 0,1), S2 exists; capture (1,0): S2 nearer
	c = Capture{ID: "c", Fetched: ts("2026-11-16T13:00:00Z"), Vec: unit(1, 0)}
	hits = NearestStories(c, st, 5)
	if len(hits) != 2 || hits[0].Story != 1 || hits[1].Story != 0 || hits[1].Version != 1 {
		t.Fatalf("hits %+v", hits)
	}
	if hits[0].Cos < hits[1].Cos {
		t.Fatal("not sorted")
	}
	// K bounds
	if len(NearestStories(c, st, 1)) != 1 {
		t.Fatal("K")
	}
	// no vector, no hits
	if len(NearestStories(Capture{Fetched: c.Fetched}, st, 5)) != 0 {
		t.Fatal("no vector")
	}
	// the max over passages is used: capture (0,1) matches S2's second passage
	c = Capture{ID: "c", Fetched: ts("2026-11-16T10:30:00Z"), Vec: unit(0, 1)}
	hits = NearestStories(c, st, 5)
	if hits[0].Cos < 0.99 {
		t.Fatalf("max over passages: %+v", hits)
	}
}

func TestOracleStatus(t *testing.T) {
	first := map[string]time.Time{"S1": ts("2026-11-14T00:00:00Z"), "S2": ts("2026-11-16T10:00:00Z")}
	at := ts("2026-11-16T09:00:00Z")
	if got := OracleStatus([]string{"S1"}, first, at); got.Status != "fed" || len(got.Existing) != 1 {
		t.Fatalf("%+v", got)
	}
	if got := OracleStatus([]string{"S2"}, first, at); got.Status != "new" {
		t.Fatalf("%+v", got)
	}
	if got := OracleStatus([]string{"S2", "S1"}, first, at); got.Status != "fed" || got.Existing[0] != "S1" {
		t.Fatalf("%+v", got)
	}
	if got := OracleStatus(nil, first, at); got.Status != "none" {
		t.Fatalf("%+v", got)
	}
}

func TestCleanerUsesEarlierDaysOnly(t *testing.T) {
	c := NewCleaner(0.10, 0.5, 3)
	m := func(k string) Mention { return Mention{Key: k, Phrase: k} }
	// day 1: "news" is in every capture; but day 1 sees no statistics
	c.StartDay()
	for i := 0; i < 10; i++ {
		if r := c.Drop(m("news"), ""); r != "" {
			t.Fatalf("day 1 dropped %q", r)
		}
		c.Observe([]Mention{m("news")}, "", "")
	}
	// day 2: "news" is now frequent in earlier days (10 of 10); "fresh" unseen
	c.StartDay()
	if r := c.Drop(m("news"), ""); r != "df" {
		t.Fatalf("day 2 news: %q", r)
	}
	if r := c.Drop(m("fresh"), ""); r != "" {
		t.Fatalf("fresh dropped: %q", r)
	}
	// counted today, not yet usable today
	c.Observe([]Mention{m("fresh")}, "", "")
	if r := c.Drop(m("fresh"), ""); r != "" {
		t.Fatalf("today's own counts leaked: %q", r)
	}
}

func TestCleanerOutletAndCommonNoun(t *testing.T) {
	c := NewCleaner(0.9, 0.5, 3)
	c.StartDay()
	for i := 0; i < 5; i++ { // "storm" is mostly lower case in running text, "Paris" capital
		c.Observe(nil, "A day", "The storm hit. We saw a storm and another storm. Then Paris. In Paris we left. Storm")
	}
	c.StartDay()
	if r := c.Drop(Mention{Key: "arab news", Phrase: "Arab News"}, "arabnews.com"); r != "outlet" {
		t.Fatalf("outlet %q", r)
	}
	if r := c.Drop(Mention{Key: "storm", Phrase: "Storm"}, ""); r != "commonnoun" {
		t.Fatalf("storm %q", r)
	}
	if r := c.Drop(Mention{Key: "paris", Phrase: "Paris"}, ""); r != "" {
		t.Fatalf("paris %q", r)
	}
	if r := c.Drop(Mention{Key: "nato", Phrase: "NATO", Acronym: true}, ""); r != "" {
		t.Fatalf("acronym %q", r)
	}
	if r := c.Drop(Mention{Key: "storm surge", Phrase: "Storm Surge"}, ""); r != "" {
		t.Fatalf("multiword is not a common noun: %q", r)
	}
}

func TestRankAndCap(t *testing.T) {
	in := ListInput{
		Stories: []StoryCand{{Title: "s1"}, {Title: "s2"}},
		Mentions: []Mention{
			{Key: "a", Phrase: "A", Hit: true, InTitle: true, Score: 1, Cands: []Cand{{EntityID: "e1", Name: "E1", Via: "exact"}}},
			{Key: "b", Phrase: "B", Hit: true, Count: 3, Score: 2, Cands: []Cand{{EntityID: "e2", Name: "E2", Via: "exact"}}},
			{Key: "n", Phrase: "N", InTitle: true, Score: 1},
			{Key: "m", Phrase: "M", Count: 1, Score: 5},
		},
		StoryEntities: [][]Cand{{{EntityID: "e1", Name: "E1"}, {EntityID: "e9", Name: "E9"}}},
	}
	list, pre := RankAndCap(in, 12)
	var labels []string
	for _, c := range list {
		labels = append(labels, c.Label)
	}
	want := []string{"s1", "s2", "E1", "N", "E2", "E9", "M"}
	if strings.Join(labels, ",") != strings.Join(want, ",") || pre != 7 {
		t.Fatalf("got %v pre %d", labels, pre)
	}
	list, pre = RankAndCap(in, 3)
	if len(list) != 3 || pre != 7 {
		t.Fatalf("cap: %d %d", len(list), pre)
	}
}
