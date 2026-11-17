package podcastmatch

// Failure list, written first (the matcher puts a clip on air, so it refuses):
//   M1. a near miss is accepted: an earlier event of the same kind, the story's
//       place or firm in unrelated news, the same subject on another day
//       (precision below 0.9 on the fixtures);
//   M2. a headline roundup that names five stories is accepted for one of them
//       (the margin rule);
//   M3. the right event discussed in an episode days before or after the story
//       is accepted (the time rule);
//   M4. a clip starts or ends mid-sentence, or is outside 20 to 75 seconds;
//   M5. a story with no distinctive term (no name, number or rare word) takes a
//       clip;
//   M6. two stories that are one event refuse each other (the margin rule must
//       not count a same-event story as a competitor);
//   M7. more than one clip per episode or more than PerStory per story;
//   M8. the counts do not add up (rejections are counted by the first rule).

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

type pr struct {
	tp, fp, fn int
}

func (x pr) precision() float64 {
	if x.tp+x.fp == 0 {
		return 1
	}
	return float64(x.tp) / float64(x.tp+x.fp)
}
func (x pr) recall() float64 {
	if x.tp+x.fn == 0 {
		return 1
	}
	return float64(x.tp) / float64(x.tp+x.fn)
}

func scoreFixture(t *testing.T, p Params) (pr, Result) {
	t.Helper()
	stories, eps, want := fixture()
	res, err := Match(context.Background(), stories, eps, p)
	if err != nil {
		t.Fatal(err)
	}
	var x pr
	got := map[string]bool{}
	for _, a := range res.Accepted {
		if want[a.EpisodeKey] == a.StoryID && !got[a.EpisodeKey] {
			x.tp++
			got[a.EpisodeKey] = true
		} else {
			x.fp++
			t.Logf("false positive: story %s took episode %s (%.0f-%.0f) %q", a.StoryID, a.EpisodeKey, a.Start, a.End, a.Quote)
		}
	}
	for k := range want {
		if !got[k] {
			x.fn++
		}
	}
	return x, res
}

func TestFixtureHasEnoughNegatives(t *testing.T) {
	neg, pos := 0, 0
	for _, c := range fixCases {
		if c.want == "" {
			neg++
		} else {
			pos++
		}
	}
	if neg < 40 || pos < 10 {
		t.Fatalf("the labelled set has %d negatives and %d positives; want at least 40 and 10", neg, pos)
	}
}

func TestPrecisionAndRecallTable(t *testing.T) {
	var table strings.Builder
	fmt.Fprintf(&table, "%-8s %5s %5s %5s %9s %7s  rejected by rule\n", "setting", "TP", "FP", "FN", "precision", "recall")
	for _, s := range []struct {
		name string
		p    Params
	}{{"strict", StrictParams}, {"default", DefaultParams}, {"loose", LooseParams}} {
		x, res := scoreFixture(t, s.p)
		fmt.Fprintf(&table, "%-8s %5d %5d %5d %9.3f %7.3f  %v\n", s.name, x.tp, x.fp, x.fn, x.precision(), x.recall(), res.Rejected)
		if s.name != "loose" && x.precision() < 0.9 {
			t.Errorf("M1: %s precision %.3f is below 0.9", s.name, x.precision())
		}
		if s.name == "default" && x.recall() < 0.6 {
			t.Errorf("default recall %.3f is below 0.6", x.recall())
		}
	}
	t.Log("\n" + table.String())
}

func TestRoundupsAndOldEpisodesAreRefused(t *testing.T) {
	_, res := scoreFixture(t, DefaultParams)
	for _, a := range res.Accepted {
		if strings.HasPrefix(a.EpisodeKey, "roundup") {
			t.Errorf("M2: roundup %s accepted for %s", a.EpisodeKey, a.StoryID)
		}
		if strings.HasSuffix(a.EpisodeKey, "-old") {
			t.Errorf("M3: old episode %s accepted for %s", a.EpisodeKey, a.StoryID)
		}
	}
	if res.Rejected[RuleTime] == 0 {
		t.Error("M3: no passage was refused by the time rule")
	}
	// The margin rule is the guard that is left when the entity gate is loose.
	if _, loose := scoreFixture(t, LooseParams); loose.Rejected[RuleMargin] == 0 {
		t.Error("M2: with the loose gate no roundup was refused by the margin rule")
	}
}

func TestAcceptedClipsAreShapedAndOnEdges(t *testing.T) {
	stories, eps, _ := fixture()
	res, err := Match(context.Background(), stories, eps, DefaultParams)
	if err != nil {
		t.Fatal(err)
	}
	byKey := map[string]Episode{}
	for _, e := range eps {
		byKey[e.Key] = e
	}
	perStory := map[string]int{}
	perEp := map[string]map[string]bool{}
	for _, a := range res.Accepted {
		d := a.End - a.Start
		if d < ClipMinSeconds || d > ClipMaxSeconds {
			t.Errorf("M4: clip of %s is %.1f s", a.EpisodeKey, d)
		}
		segs := byKey[a.EpisodeKey].Segs
		first, last := -1, -1
		for i, sg := range segs {
			if sg.Start == a.Start {
				first = i
			}
			if sg.End == a.End {
				last = i
			}
		}
		if first < 0 || last < 0 || !startsClean(segs, first) || !endsClean(segs, last) {
			t.Errorf("M4: clip %s %.1f-%.1f is not on segment and sentence edges", a.EpisodeKey, a.Start, a.End)
		}
		if !strings.Contains(a.Text, strings.Fields(a.Quote)[0]) || len([]rune(a.Quote)) > QuoteMaxRunes {
			t.Errorf("M4: quote %q is not words of its clip", a.Quote)
		}
		if a.Support <= 0 || a.Support > 0.95 {
			t.Errorf("support %v out of range", a.Support)
		}
		perStory[a.StoryID]++
		if perEp[a.StoryID] == nil {
			perEp[a.StoryID] = map[string]bool{}
		}
		if perEp[a.StoryID][a.EpisodeKey] {
			t.Errorf("M7: two clips of %s for %s", a.EpisodeKey, a.StoryID)
		}
		perEp[a.StoryID][a.EpisodeKey] = true
	}
	for id, n := range perStory {
		if n > DefaultParams.PerStory {
			t.Errorf("M7: %s has %d clips", id, n)
		}
	}
}

func TestMidSentenceEdgesAreNotUsed(t *testing.T) {
	// The sentence is cut by the segmenter into two segments with a small gap:
	// a clip must not start at the second half.
	segs := []Segment{
		{0, 6, "Filler about weather."},
		{7, 13, "The Marlin ferry hit a pier in Tessaly Bay and forty were hurt,"},
		{13.2, 19, "and the Harborview vessel lost steering, the coast guard said."},
		{20, 26, "Rescue boats were quick to arrive at the pier in Tessaly Bay."},
		{27, 33, "The ferry Marlin will be inspected."},
		{34, 40, "More filler."},
	}
	win := Windows("e", segs)[0]
	gist := "Harborview ferry Marlin strikes pier in Tessaly Bay, 40 hurt"
	clip, ok := ChooseSentenceClip(segs, win, gist)
	if !ok {
		t.Fatal("no clip")
	}
	if clip.First == 2 {
		t.Fatalf("M4: the clip starts mid-sentence at segment 2")
	}
	if !startsClean(segs, clip.First) || !endsClean(segs, clip.Last) {
		t.Fatalf("M4: edges not clean: %d-%d", clip.First, clip.Last)
	}
}

func TestStoryWithoutDistinctiveTermsTakesNothing(t *testing.T) {
	stories, eps, _ := fixture()
	stories = append(stories, Story{ID: "vague", Title: "Officials say things are going well", Dek: "More news later this week.", Updated: fixNow})
	res, err := Match(context.Background(), stories, eps, LooseParams)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range res.Accepted {
		if a.StoryID == "vague" {
			t.Fatalf("M5: a story with no distinctive term took %s", a.EpisodeKey)
		}
	}
}

func TestSameEventStoriesDoNotRefuseEachOther(t *testing.T) {
	stories, eps, _ := fixture()
	stories = append(stories, Story{ID: "ferry2", Title: "Marlin ferry collides with pier in Tessaly Bay; 40 injured", Dek: "Harborview vessel lost steering, coast guard says.", Updated: fixNow})
	res, err := Match(context.Background(), stories, eps, DefaultParams)
	if err != nil {
		t.Fatal(err)
	}
	have := map[string]bool{}
	for _, a := range res.Accepted {
		have[a.StoryID+"|"+a.EpisodeKey] = true
	}
	if !have["ferry|ferry+1"] || !have["ferry2|ferry+1"] {
		t.Fatalf("M6: two versions of one event did not both take their clip: %v", have)
	}
}

func TestCountsAddUp(t *testing.T) {
	_, res := scoreFixture(t, DefaultParams)
	rej := 0
	for _, n := range res.Rejected {
		rej += n
	}
	if res.Passages != len(res.Accepted)+rej {
		t.Fatalf("M8: %d passages, %d accepted, %d rejected", res.Passages, len(res.Accepted), rej)
	}
	if res.Stories != len(fixStories) || res.Scored == 0 {
		t.Fatalf("M8: stories %d scored %d", res.Stories, res.Scored)
	}
}

func TestRankStillEquivalent(t *testing.T) {
	_, eps, _ := fixture()
	var wins []Window
	for _, e := range eps {
		wins = append(wins, Windows(e.Key, e.Segs)...)
	}
	out, err := Rank(context.Background(), []Query{{ID: "q", Text: "Tessaly Bay ferry Marlin pier"}}, wins, 3)
	if err != nil || len(out["q"]) == 0 {
		t.Fatalf("rank: %v %v", out, err)
	}
}
