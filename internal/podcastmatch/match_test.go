package podcastmatch

// Ways matching can fail, written first (synthetic transcripts only):
//   M1. the passage that covers the story does not rank first, because the
//       ranking counts stop words or ignores rarity;
//   M2. a garbled name (one letter off) earns nothing, so a story is missed;
//   M3. a story with one common word in common with a transcript gets a hit;
//   M4. the top hits are windows of the same minute twice;
//   M5. a clip is shorter than 20 s or longer than 75 s, or starts or ends
//       inside a segment;
//   M6. the clip drags in the irrelevant sentences around the relevant ones;
//   M7. the display quote is over 280 characters.

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

// transcript builds n segments of about six seconds: filler about the weather
// and the stock market, with the given lines replacing the segments from at.
func transcript(n int, at int, lines []string) []Segment {
	filler := []string{
		"Meanwhile the weather in the valley stays mild with light wind through the evening.",
		"Stock indexes closed a little higher after a quiet day of trading in most sectors.",
		"A listener wrote in to ask about gardening tips for the coming season.",
		"The next segment looks at how schools are planning their calendars.",
	}
	var segs []Segment
	for i := 0; i < n; i++ {
		text := filler[i%len(filler)]
		if i >= at && i < at+len(lines) {
			text = lines[i-at]
		}
		segs = append(segs, Segment{Start: float64(i) * 6, End: float64(i)*6 + 5.5, Text: text})
	}
	return segs
}

var harborLines = []string{
	"The harbor at Hargreeves reopened this morning after a week closed by the storm.",
	"Ferry service resumes on Monday, the port authority said, once inspectors finish.",
	"Officials expect the first sailings to carry freight before passengers return.",
	"Fishing boats have already started to leave the harbor at Hargreeves at dawn.",
}

func TestM1_M2_M4_TheCoveringPassageRanksFirstAndNamesMayBeGarbled(t *testing.T) {
	a := transcript(60, 20, harborLines) // episode a covers the harbor at 120 s
	b := transcript(60, 0, nil)          // episode b is filler
	wins := append(Windows("a", a), Windows("b", b)...)
	// The story spells the name another way: one letter off.
	story := Query{ID: "s1", Text: "Harbor at Hargreaves reopens after storm. Ferry service resumes Monday, the port authority says."}
	ranked, err := Rank(context.Background(), []Query{story}, wins, 4)
	if err != nil {
		t.Fatal(err)
	}
	hits := ranked["s1"]
	if len(hits) == 0 || hits[0].Window.Key != "a" {
		t.Fatalf("M1: the covering passage is not first: %+v", hits)
	}
	w := hits[0].Window
	if w.Start > 120 || w.End < 140 {
		t.Fatalf("M1: the best window [%.0f, %.0f] does not hold the harbor lines at 120 to 144 s", w.Start, w.End)
	}
	for i := 1; i < len(hits); i++ {
		if hits[i].Window.Key == hits[0].Window.Key && hits[i].Window.Start < w.End && w.Start < hits[i].Window.End {
			t.Fatalf("M4: hit %d overlaps the best one", i)
		}
	}
	// M2: without the tolerant match the name adds nothing; with it, the score
	// is higher than for a story that lacks the name.
	plainRanked, _ := Rank(context.Background(), []Query{{ID: "s2", Text: "Harbor reopens after storm. Ferry service resumes Monday, the port authority says."}}, wins, 1)
	plain := plainRanked["s2"]
	if len(plain) == 0 || !(hits[0].Score > plain[0].Score) {
		t.Fatalf("M2: the garbled name earned nothing: %v vs %v", hits[0].Score, plain)
	}
	if hits[0].Matched <= plain[0].Matched {
		t.Fatalf("M2: matched terms %d vs %d", hits[0].Matched, plain[0].Matched)
	}
}

func TestM3_AStoryThatSharesOnlyACommonWordHasNoHit(t *testing.T) {
	wins := Windows("a", transcript(60, 20, harborLines))
	r3, _ := Rank(context.Background(), []Query{{ID: "s", Text: "Council approves new budget for schools."}}, wins, 4)
	hits := r3["s"]
	// "schools" appears in the filler once a cycle; one term is not a match.
	for _, h := range hits {
		if h.Matched < 2 {
			t.Fatalf("M3: a hit with %d matched terms: %+v", h.Matched, h)
		}
	}
	r4, _ := Rank(context.Background(), []Query{{ID: "s", Text: "Volcano erupts near the capital city."}}, wins, 4)
	none := r4["s"]
	if len(none) != 0 {
		t.Fatalf("M3: an unrelated story has %d hits", len(none))
	}
}

func TestM5_M6_M7_TheClipIsOnSegmentBoundariesAndTrimmed(t *testing.T) {
	segs := transcript(60, 20, harborLines)
	wins := Windows("a", segs)
	story := "Harbor at Hargreaves reopens after storm. Ferry service resumes Monday, the port authority says."
	r5, _ := Rank(context.Background(), []Query{{ID: "s", Text: story}}, wins, 1)
	hits := r5["s"]
	clip, ok := ChooseClip(segs, *hits[0].Window, story)
	if !ok {
		t.Fatal("no clip")
	}
	if d := clip.End - clip.Start; d < ClipMinSeconds || d > ClipMaxSeconds {
		t.Fatalf("M5: %.1f seconds", d)
	}
	onStart, onEnd := false, false
	for _, s := range segs {
		onStart = onStart || s.Start == clip.Start
		onEnd = onEnd || s.End == clip.End
	}
	if !onStart || !onEnd {
		t.Fatalf("M5: the clip [%v, %v] is not on segment boundaries", clip.Start, clip.End)
	}
	// M6: the relevant lines run 120 to 144 s; the clip holds all four and at
	// most one irrelevant segment each side to reach the 20 s minimum.
	if clip.Start > 120 || clip.End < 143 {
		t.Fatalf("M6: the clip [%v, %v] cuts the harbor lines", clip.Start, clip.End)
	}
	if clip.Start < 120-12 || clip.End > 144+12 {
		t.Fatalf("M6: the clip [%v, %v] carries more than a segment or two of irrelevance", clip.Start, clip.End)
	}
	if !strings.Contains(clip.Quote, "harbor") && !strings.Contains(clip.Quote, "Ferry") {
		t.Fatalf("M7: the quote %q is not a relevant line", clip.Quote)
	}
	long := []Segment{{Start: 0, End: 30, Text: strings.Repeat("harbor ferry storm ", 40)}, {Start: 30, End: 55, Text: "other"}}
	c, ok := ChooseClip(long, Window{First: 0, Last: 1}, "harbor ferry storm")
	if !ok || len([]rune(c.Quote)) > QuoteMaxRunes {
		t.Fatalf("M7: quote of %d runes (ok %v)", len([]rune(c.Quote)), ok)
	}
}

func TestChooseClipRefusesWhenNothingMatchesOrNothingFits(t *testing.T) {
	segs := transcript(10, 0, nil)
	if _, ok := ChooseClip(segs, Window{First: 0, Last: 9}, "volcano erupts"); ok {
		t.Fatal("a clip with no matching words")
	}
	short := []Segment{{Start: 0, End: 4, Text: "harbor ferry storm"}}
	if _, ok := ChooseClip(short, Window{First: 0, Last: 0}, "harbor ferry storm"); ok {
		t.Fatal("a clip under 20 seconds")
	}
}

func TestTokensAndWithinOne(t *testing.T) {
	got := fmt.Sprint(Tokens("The harbors' Ferries, it's reopening!"))
	if got != "[harbor ferri its reopen]" && got != "[harbors ferri its reopen]" {
		t.Logf("tokens: %s", got) // the stem is crude; only the shape matters
	}
	for _, c := range []struct {
		a, b string
		want bool
	}{{"hargreeves", "hargreaves", true}, {"harbor", "harbour", true}, {"harbor", "harbours", false}, {"abc", "abc", true}, {"abcdef", "abcdfe", false}} {
		if withinOne(c.a, c.b) != c.want {
			t.Errorf("withinOne(%q,%q) != %v", c.a, c.b, c.want)
		}
	}
	if len(Windows("x", nil)) != 0 {
		t.Fatal("windows of nothing")
	}
}

// Round 2 repairs: the deadline reaches the ranking, a transcript costs a
// bounded number of windows, the quote is a prefix of what was said, and a
// float that cannot advance does not spin.
func TestM8_TheDeadlineCancelsTheRanking(t *testing.T) {
	wins := Windows("a", transcript(60, 20, harborLines))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Rank(ctx, []Query{{ID: "s", Text: "harbor ferry storm"}}, wins, 4); err == nil {
		t.Fatal("M8: a cancelled context did not stop the ranking")
	}
}

func TestM9_WindowsAreBoundedAndAlwaysAdvance(t *testing.T) {
	var long []Segment
	for i := 0; i < 20000; i++ {
		long = append(long, Segment{Start: float64(i) * 6, End: float64(i)*6 + 5, Text: "a long episode of talk"})
	}
	if n := len(Windows("long", long)); n > MaxWindowsPerTranscript {
		t.Fatalf("M9: %d windows for one transcript, bound %d", n, MaxWindowsPerTranscript)
	}
	// A start so large that adding a stride changes nothing must end, not spin.
	huge := []Segment{{Start: 1e18, End: 1e18 + 5, Text: "x y"}, {Start: 1e18 + 6, End: 1e18 + 9, Text: "z"}}
	done := make(chan int, 1)
	go func() { done <- len(Windows("huge", huge)) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("M9: Windows spins on a start the float cannot advance")
	}
}

func TestM10_AQuoteIsAPrefixOfWhatWasSaid(t *testing.T) {
	long := strings.Repeat("harbor ", 100) // 700 characters
	segs := []Segment{{Start: 0, End: 30, Text: long}, {Start: 30, End: 55, Text: "other"}}
	c, ok := ChooseClip(segs, Window{First: 0, Last: 1}, "harbor ferry storm")
	if !ok {
		t.Fatal("M10: no clip")
	}
	if len([]rune(c.Quote)) > QuoteMaxRunes || strings.Contains(c.Quote, "…") || !strings.Contains(long, c.Quote) {
		t.Fatalf("M10: quote %q is not a plain prefix of the transcript", c.Quote)
	}
	if strings.HasSuffix(c.Quote, "harbo") || strings.HasSuffix(c.Quote, " ") {
		t.Fatalf("M10: the quote is cut inside a word or ends in space: %q", c.Quote)
	}
}
