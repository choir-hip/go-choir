package podcastmatch

// Review of 5cc739079, written first:
//   H1. caps are per run: a story that the server already holds 3 clips for
//       takes a fourth, or takes a second clip from an episode it has;
//   H2. one audio under two feed entries (two guids, one enclosure) gives two
//       clips of one story, and so does the same hourly item repeated in the
//       next hour's episode (13:00 and 14:00 briefs);
//   H3. an ad read that names the brand, and a Title Case headline, take clips.

import (
	"context"
	"strings"
	"testing"
	"time"
)

func ferryPassage(extra string) []Segment {
	_, eps, _ := fixture()
	for _, e := range eps {
		if e.Key == "ferry+1" {
			segs := append([]Segment(nil), e.Segs...)
			_ = extra
			return segs
		}
	}
	return nil
}

func withKey(e Episode, key, audio string, hoursAgo float64) Episode {
	e.Key, e.AudioKey = key, audio
	e.Published = fixNow.Add(-time.Duration(hoursAgo * float64(time.Hour)))
	return e
}

func runWith(t *testing.T, held map[string][]Held, extra ...Episode) Result {
	t.Helper()
	stories, eps, _ := fixture()
	var keep []Episode
	for _, e := range eps {
		if !strings.HasPrefix(e.Key, "ferry+") && !strings.HasPrefix(e.Key, "roundup") && !strings.HasSuffix(e.Key, "-old") {
			keep = append(keep, e)
		}
	}
	keep = append(keep, extra...)
	res, err := MatchSubjects(context.Background(), stories, map[string]bool{"ferry": true}, held, keep, DefaultParams)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func TestH2_OneAudioUnderTwoEntriesGivesOneClip(t *testing.T) {
	base := Episode{Segs: ferryPassage("")}
	a := withKey(base, "ep-a", "https://cdn.example.test/news-14.mp3", 3)
	b := withKey(base, "ep-b", "https://cdn.example.test/news-14.mp3", 3)
	res := runWith(t, nil, a, b)
	if len(res.Accepted) != 1 {
		t.Fatalf("H2: %d clips for one audio under two entries", len(res.Accepted))
	}
	// Two audios that carry the same item (an hourly brief repeated in the next one).
	c := withKey(base, "ep-c", "https://cdn.example.test/news-15.mp3", 2)
	res = runWith(t, nil, a, c)
	if len(res.Accepted) != 1 {
		t.Fatalf("H2: the same spoken item in two episodes gave %d clips", len(res.Accepted))
	}
}

func TestH1_CapsCountWhatTheServerHolds(t *testing.T) {
	base := Episode{Segs: ferryPassage("")}
	a := withKey(base, "ep-a", "a", 3)
	// The server holds a clip from this audio already: no second clip from it.
	res := runWith(t, map[string][]Held{"ferry": {{AudioKey: "a", Text: "Forty people were hurt."}}}, a)
	if len(res.Accepted) != 0 {
		t.Fatalf("H1: a second clip from an episode the story has: %v", res.Accepted)
	}
	// Three held clips fill the story.
	other := []Held{{AudioKey: "x", Text: "one"}, {AudioKey: "y", Text: "two"}, {AudioKey: "z", Text: "three"}}
	res = runWith(t, map[string][]Held{"ferry": other}, a)
	if len(res.Accepted) != 0 || res.Rejected[RuleCap] == 0 {
		t.Fatalf("H1: a fourth clip: %+v", res)
	}
	// Two held leave one slot.
	res = runWith(t, map[string][]Held{"ferry": other[:2]}, a, withKey(base, "ep-b", "b", 3))
	if len(res.Accepted) != 1 {
		t.Fatalf("H1: %d accepted with one slot", len(res.Accepted))
	}
}

func TestH3_AnAdReadNamingTheBrandTakesNothing(t *testing.T) {
	ad := fixCase{"ad", "", -3, s(
		"This programme is brought to you by Harborview Lines, the ferry company that takes you across Tessaly Bay.",
		"Book a Marlin class crossing today and hurry, because seats go fast.",
		"The Harborview Lines app shows every pier and every crossing.",
		"Visit the website for coast guard approved safety information.",
		"Terms apply and some crossings are seasonal.")}
	stories, eps, _ := fixture()
	var segs []Segment
	for i := 0; i < 8; i++ {
		segs = append(segs, Segment{Start: float64(i) * 7, End: float64(i)*7 + 6.5, Text: fixFiller[i]})
	}
	for _, t := range ad.text {
		at := float64(len(segs)) * 7
		segs = append(segs, Segment{Start: at, End: at + 6.5, Text: t})
	}
	eps = append(eps, Episode{Key: "ad", Published: fixNow.Add(-3 * time.Hour), Segs: segs})
	res, err := Match(context.Background(), stories, eps, DefaultParams)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range res.Accepted {
		if a.EpisodeKey == "ad" {
			t.Logf("accepted: %s %q", a.StoryID, a.Quote)
			t.Errorf("H3: an ad read took a clip for %s", a.StoryID)
		}
	}
}

func TestH3_TitleCaseHeadlinesDoNotTurnWordsIntoNames(t *testing.T) {
	stories, eps, want := fixture()
	for i := range stories {
		stories[i].Title = titleCase(stories[i].Title)
	}
	res, err := Match(context.Background(), stories, eps, DefaultParams)
	if err != nil {
		t.Fatal(err)
	}
	fp, tp := 0, 0
	for _, a := range res.Accepted {
		if want[a.EpisodeKey] == a.StoryID {
			tp++
		} else {
			fp++
			t.Logf("false positive: %s took %s", a.StoryID, a.EpisodeKey)
		}
	}
	if tp+fp == 0 || float64(tp)/float64(tp+fp) < 0.9 {
		t.Fatalf("H3: Title Case precision %d/%d", tp, tp+fp)
	}
}

func titleCase(s string) string {
	f := strings.Fields(s)
	for i, w := range f {
		f[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(f, " ")
}

// A known limit, left as it is (the thresholds are not changed here): an
// episode 40 h before an ongoing event, still inside the 48 h window, that
// talks about the run-up with the story's names and words is accepted. The
// test records what happens so a change in it is seen.
func TestKnownLimit_AnEpisodeBeforeAnOngoingEventInsideTheWindow(t *testing.T) {
	stories, eps, _ := fixture()
	var segs []Segment
	for i := 0; i < 8; i++ {
		segs = append(segs, Segment{Start: float64(i) * 7, End: float64(i)*7 + 6.5, Text: fixFiller[i]})
	}
	for _, x := range s("Unions at Ostrava Rail say they will ballot members on a strike next week.", "The company says talks about pay will continue on Monday.", "Freight customers in Kovnia have been told to expect possible delays.", "The union leader said members are angry about the offer of four percent.", "No date has been set for any action.") {
		at := float64(len(segs)) * 7
		segs = append(segs, Segment{Start: at, End: at + 6.5, Text: x})
	}
	eps = append(eps, Episode{Key: "rail-before", Published: fixNow.Add(-40 * time.Hour), Segs: segs})
	res, _ := Match(context.Background(), stories, eps, DefaultParams)
	taken := false
	for _, a := range res.Accepted {
		taken = taken || a.EpisodeKey == "rail-before"
	}
	t.Logf("a 40 h old run-up episode is accepted: %v (known limit)", taken)
	eps[len(eps)-1].Published = fixNow.Add(-60 * time.Hour)
	res, _ = Match(context.Background(), stories, eps, DefaultParams)
	for _, a := range res.Accepted {
		if a.EpisodeKey == "rail-before" {
			t.Fatal("a 60 h old episode passed the time rule")
		}
	}
}

// C2 of the re-review: editorial speech that contains an ad-like word is not an ad.
func TestC2_EditorialSpeechWithAdLikeWordsStillClips(t *testing.T) {
	for _, text := range []string{
		"The sponsors of the bill say the Marlin ferry crash in Tessaly Bay shows why it is needed.",
		"The finding that the Harborview vessel lost steering is supported by the data from the coast guard.",
		"Passengers had to book a second crossing after the Marlin hit the pier in Tessaly Bay.",
		"Subscribe to the view that forty hurt on the Marlin ferry in Tessaly Bay was avoidable, said the coast guard.",
	} {
		if isAdRead(text) {
			t.Errorf("C2: %q read as an ad", text)
		}
	}
	for _, text := range []string{
		"This episode is brought to you by Harborview Lines.",
		"Today's show is sponsored by Kestrel Dynamics.",
		"This episode is supported by Orbis bikes.",
		"Use code NEWS for a free trial, terms apply.",
	} {
		if !isAdRead(text) {
			t.Errorf("C2: %q not read as an ad", text)
		}
	}
}
