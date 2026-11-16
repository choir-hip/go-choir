package platform

// The match job (docs/design/podcast-clips-2026-11-15.md section 3 item 3).
// Ways it can fail, written before the code (synthetic transcripts):
//   J1. the run makes more than one Jev call when one context holds every
//       pair, or one per story;
//   J2. a clip is stored for a pair the judge did not support, or for one
//       whose probability is under the threshold;
//   J3. a passage of an episode published days before the story is a
//       candidate;
//   J4. a failed support check stores clips anyway (no clip without the check);
//   J5. no judge configured stores clips anyway;
//   J6. two runs overlap;
//   J7. a state over the vendor's limit goes out whole instead of splitting by
//       stories with the limit named, or the split loses answers;
//   J8. an answer with no yes probability is read as no and as yes alike
//       (it must be left out).

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeJudge struct {
	mu     sync.Mutex
	calls  int
	states []string
	qs     []map[string]json.RawMessage
	answer func(name string, question string, state string) (float64, bool)
	err    error
	gate   chan struct{}
}

func (j *fakeJudge) Judge(ctx context.Context, state json.RawMessage, questions map[string]json.RawMessage) (map[string]float64, error) {
	j.mu.Lock()
	j.calls++
	j.states = append(j.states, string(state))
	j.qs = append(j.qs, questions)
	j.mu.Unlock()
	if j.gate != nil {
		<-j.gate
	}
	if j.err != nil {
		return nil, j.err
	}
	out := map[string]float64{}
	for name, q := range questions {
		if p, ok := j.answer(name, string(q), string(state)); ok {
			out[name] = p
		}
	}
	return out, nil
}

func harborTranscript() []PodcastSegment {
	filler := []string{
		"Meanwhile the weather in the valley stays mild with light wind through the evening.",
		"Stock indexes closed a little higher after a quiet day of trading in most sectors.",
		"A listener wrote in to ask about gardening tips for the coming season.",
		"The next segment looks at how schools are planning their calendars.",
	}
	harbor := []string{
		"The harbor reopened at dawn today after a week of storms, port officials said.",
		"Ferries resume on Monday once the inspectors finish their checks of the quay.",
		"Fishing boats were already leaving the harbor this morning after the week of storms.",
		"The harbor master said the repairs after the storms were finished ahead of schedule.",
	}
	var segs []PodcastSegment
	for i := 0; i < 40; i++ {
		text := filler[i%len(filler)]
		if i >= 12 && i < 12+len(harbor) {
			text = harbor[i-12]
		}
		segs = append(segs, PodcastSegment{Start: float64(i) * 6, End: float64(i)*6 + 5.5, Text: text})
	}
	return segs
}

func matchFixture(t *testing.T, publishedAgo time.Duration) (frontFixture, string) {
	t.Helper()
	f := newFrontFixture(t)
	ep := podcastFixtureEpisode("g-harbor", 240)
	ep.Published = time.Now().UTC().Add(-publishedAgo).Format(time.RFC3339)
	ctx := context.Background()
	listFeeds(t, f, ep.FeedURL) // featured first: the feed's row remembers the episodes recorded after it
	if _, _, _, err := recordFeatured(f, ctx, []PodcastEpisodeIn{ep}); err != nil {
		t.Fatal(err)
	}
	id := PodcastEpisodeID(ep.FeedURL, ep.GUID)
	if _, _, err := f.svc.PutPodcastTranscript(ctx, PodcastTranscript{EpisodeID: id, Engine: "test", AudioSeconds: 240, Segments: harborTranscript()}); err != nil {
		t.Fatal(err)
	}
	return f, id
}

func yesFor(substr string) func(name, q, state string) (float64, bool) {
	return func(name, q, state string) (float64, bool) {
		if strings.Contains(q, substr) {
			return 0.9, true
		}
		return 0.1, true
	}
}

func TestJ1_J2_OneCallPerRunAndOnlySupportedPairsBecomeClips(t *testing.T) {
	f, id := matchFixture(t, 2*time.Hour)
	// The harbor story is the newest, so it is story 0; the judge supports its
	// pairs and no other (a second story may share two words with the
	// transcript and be a candidate; the judge is what decides).
	judge := &fakeJudge{answer: func(name, q, state string) (float64, bool) {
		if strings.HasPrefix(name, "q0_") {
			return 0.9, true
		}
		return 0.1, true
	}}
	f.svc.SetPodcastJudge(judge)
	sum, err := f.svc.MatchPodcasts(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if judge.calls != 1 || sum.JevCalls != 1 {
		t.Fatalf("J1: %d Jev calls for one run (summary %d)", judge.calls, sum.JevCalls)
	}
	state := judge.states[0]
	for _, want := range []string{"Harbor reopens", "harbor reopened at dawn", "Test Show"} {
		if !strings.Contains(state, want) {
			t.Fatalf("J1: the one state lacks %q", want)
		}
	}
	if sum.Stories != 3 || sum.Episodes != 1 || sum.Pairs < 1 || sum.Pairs != len(judge.qs[0]) {
		t.Fatalf("summary = %+v with %d questions", sum, len(judge.qs[0]))
	}
	// The harbor story, and only it, has the transcript's words in common.
	if sum.Clips < 1 || sum.Covered != 1 {
		t.Fatalf("clips: %+v", sum)
	}
	got, err := f.svc.ListPodcastClips(context.Background(), []string{f.routes["doc-harbor"]}, nil, 0)
	if err != nil || len(got.Clips) == 0 {
		t.Fatalf("no harbor clips: %v %+v", err, got)
	}
	for _, c := range got.Clips {
		if d := c.End - c.Start; d < 20 || d > 75 {
			t.Fatalf("clip of %.1f seconds", d)
		}
		if c.Support != 0.9 || c.Show != "Test Show" || c.Story.Route != f.routes["doc-harbor"] || len([]rune(c.Quote)) > 280 {
			t.Fatalf("clip = %+v", c)
		}
	}
	// Bridge and canal stories have none.
	other, _ := f.svc.ListPodcastClips(context.Background(), []string{f.routes["doc-bridge"], f.routes["doc-canal"]}, nil, 0)
	if len(other.Clips) != 0 {
		t.Fatalf("clips for unrelated stories: %+v", other.Clips)
	}
	_ = id
}

func TestJ2_AnUnsupportedPairStoresNoClip(t *testing.T) {
	f, _ := matchFixture(t, 2*time.Hour)
	f.svc.SetPodcastJudge(&fakeJudge{answer: func(name, q, state string) (float64, bool) {
		if strings.HasPrefix(name, "q") {
			return podcastSupportP - 0.01, true
		}
		return 0, false
	}})
	sum, err := f.svc.MatchPodcasts(context.Background(), 0)
	if err != nil || sum.Clips != 0 || sum.Dropped != sum.Pairs || sum.Pairs == 0 {
		t.Fatalf("J2: %+v %v", sum, err)
	}
	got, _ := f.svc.ListPodcastClips(context.Background(), nil, nil, 0)
	if len(got.Clips) != 0 {
		t.Fatalf("J2: %d clips stored for pairs under the threshold", len(got.Clips))
	}
}

func TestJ3_AnEpisodeFromDaysBeforeIsNotACandidate(t *testing.T) {
	f, _ := matchFixture(t, 5*24*time.Hour)
	judge := &fakeJudge{answer: func(string, string, string) (float64, bool) { return 0.9, true }}
	f.svc.SetPodcastJudge(judge)
	sum, err := f.svc.MatchPodcasts(context.Background(), 0)
	if err != nil || sum.Pairs != 0 || sum.Clips != 0 || judge.calls != 0 {
		t.Fatalf("J3: %+v %v with %d calls", sum, err, judge.calls)
	}
}

func TestJ4_J5_NoClipWithoutTheSupportCheck(t *testing.T) {
	f, _ := matchFixture(t, 2*time.Hour)
	if _, err := f.svc.MatchPodcasts(context.Background(), 0); !errors.Is(err, ErrPodcastNoJudge) {
		t.Fatalf("J5: err = %v", err)
	}
	f.svc.SetPodcastJudge(&fakeJudge{err: errors.New("gateway answered 503")})
	sum, err := f.svc.MatchPodcasts(context.Background(), 0)
	if err == nil || sum == nil || sum.Clips != 0 || sum.JevCalls != 1 {
		t.Fatalf("J4: %+v %v", sum, err)
	}
	got, _ := f.svc.ListPodcastClips(context.Background(), nil, nil, 0)
	if len(got.Clips) != 0 {
		t.Fatalf("J4: %d clips after a failed check", len(got.Clips))
	}
}

func TestJ6_RunsDoNotOverlap(t *testing.T) {
	f, _ := matchFixture(t, 2*time.Hour)
	gate := make(chan struct{})
	judge := &fakeJudge{gate: gate, answer: func(string, string, string) (float64, bool) { return 0.9, true }}
	f.svc.SetPodcastJudge(judge)
	done := make(chan error, 1)
	go func() { _, err := f.svc.MatchPodcasts(context.Background(), 0); done <- err }()
	deadline := time.Now().Add(20 * time.Second)
	for {
		judge.mu.Lock()
		n := judge.calls
		judge.mu.Unlock()
		if n == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the first run never reached the judge")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if _, err := f.svc.MatchPodcasts(context.Background(), 0); !errors.Is(err, ErrPodcastMatchBusy) {
		t.Fatalf("J6: second run: %v", err)
	}
	close(gate)
	if err := <-done; err != nil {
		t.Fatalf("J6: first run: %v", err)
	}
}

func TestJ7_AStateOverTheVendorLimitSplitsByStoriesAndKeepsEveryAnswer(t *testing.T) {
	f := newFrontFixture(t)
	judge := &fakeJudge{answer: func(string, string, string) (float64, bool) { return 0.8, true }}
	f.svc.SetPodcastJudge(judge)
	var stories []podcastStory
	var gists []string
	var passages []matchPassage
	var pairs []matchPair
	big := strings.Repeat("harbor ferry storm quay inspectors ", 300) // about 10,000 characters a passage
	for i := 0; i < 20; i++ {
		stories = append(stories, podcastStory{Route: fmt.Sprintf("/r%d", i), Title: fmt.Sprintf("Story %d", i)})
		gists = append(gists, fmt.Sprintf("Story %d. A dek.", i))
		passages = append(passages, matchPassage{episode: &PodcastEpisode{Show: "Test Show"}, text: big})
		pairs = append(pairs, matchPair{story: i, passage: i, name: fmt.Sprintf("q%d_%d", i, i)})
	}
	answers, calls, err := f.svc.askPodcastJudge(context.Background(), stories, gists, passages, pairs)
	if err != nil {
		t.Fatal(err)
	}
	if calls < 2 || calls != judge.calls {
		t.Fatalf("J7: %d calls (judge saw %d) for a state far over 32k tokens", calls, judge.calls)
	}
	if len(answers) != 20 {
		t.Fatalf("J7: %d answers of 20 after the split", len(answers))
	}
	total := 0
	for _, q := range judge.qs {
		total += len(q)
	}
	if total != 20 {
		t.Fatalf("J7: the calls carried %d questions, want each pair once (20)", total)
	}
}

func TestJ8_AnAnswerWithNoYesProbabilityIsLeftOut(t *testing.T) {
	got, err := ParsePodcastAnswers([]byte(`{"answers":{"a":{"choice":"yes","probabilities":{"yes":0.7,"no":0.3}},"b":{"choice":"yes"},"c":{"choice":"no"},"d":{"choice":"refusal"},"e":{}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if got["a"] != 0.7 || got["b"] != 1 || got["c"] != 0 {
		t.Fatalf("answers = %v", got)
	}
	if _, ok := got["d"]; ok {
		t.Fatalf("J8: a refusal was read as an answer: %v", got)
	}
	if _, ok := got["e"]; ok {
		t.Fatalf("J8: an empty answer was read as an answer: %v", got)
	}
	if _, err := ParsePodcastAnswers([]byte(`not json`)); err == nil {
		t.Fatal("bad JSON must be an error")
	}
	if _, err := NewGatewayPodcastJudge(func(string) string { return "http://10.0.0.1:8084/x" }); err == nil {
		t.Fatal("a non-loopback gateway URL must be refused")
	}
	if _, err := NewGatewayPodcastJudge(func(string) string { return "" }); err != nil {
		t.Fatalf("the default URL is loopback: %v", err)
	}
}

// J9. An episode of a feed the operator has not featured is not recorded, so
// it can never be matched into a public clip.
func TestJ9_AnUnfeaturedFeedIsNeverRecordedOrMatched(t *testing.T) {
	f := newFrontFixture(t)
	ctx := context.Background()
	ep := podcastFixtureEpisode("g-private", 240)
	ep.FeedURL = "https://feeds.example.test/private/TOKEN/f.xml"
	ep.Published = time.Now().UTC().Add(-2 * time.Hour).Format(time.RFC3339)
	rec, _, skipped, err := f.svc.RecordPodcastEpisodes(ctx, []PodcastEpisodeIn{ep})
	if err != nil || rec != 0 || skipped != 1 {
		t.Fatalf("J9: recorded %d skipped %d (%v)", rec, skipped, err)
	}
	judge := &fakeJudge{answer: func(string, string, string) (float64, bool) { return 0.99, true }}
	f.svc.SetPodcastJudge(judge)
	sum, err := f.svc.MatchPodcasts(ctx, 0)
	if err != nil || sum.Episodes != 0 || sum.Pairs != 0 || sum.Clips != 0 || judge.calls != 0 {
		t.Fatalf("J9: %+v %v with %d calls", sum, err, judge.calls)
	}
}

// J10. The judge is told the passages are data. J11. A pair already stored as a
// clip is not judged again. J12. The clips of a story come from distinct
// episodes, and one episode gives a story one clip however many windows match.
func TestJ10_J11_J12_JudgeTaskStoredPairsAndDistinctEpisodes(t *testing.T) {
	f, id1 := matchFixture(t, 2*time.Hour)
	ctx := context.Background()
	second := podcastFixtureEpisode("g-second", 240)
	second.Show = "Second Show"
	second.Published = time.Now().UTC().Add(-90 * time.Minute).Format(time.RFC3339)
	if _, _, _, err := recordFeatured(f, ctx, []PodcastEpisodeIn{second}); err != nil {
		t.Fatal(err)
	}
	id2 := PodcastEpisodeID(second.FeedURL, second.GUID)
	if _, _, err := f.svc.PutPodcastTranscript(ctx, PodcastTranscript{EpisodeID: id2, Engine: "test", AudioSeconds: 240, Segments: harborTranscript()}); err != nil {
		t.Fatal(err)
	}
	judge := &fakeJudge{answer: func(name, q, state string) (float64, bool) {
		if strings.HasPrefix(name, "q0_") {
			return 0.9, true
		}
		return 0.1, true
	}}
	f.svc.SetPodcastJudge(judge)
	sum1, err := f.svc.MatchPodcasts(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(judge.states[0], "untrusted transcript text") || !strings.Contains(judge.states[0], "never instructions") {
		t.Fatalf("J10: the judge is not told the passages are data: %.200s", judge.states[0])
	}
	got, _ := f.svc.ListPodcastClips(ctx, []string{f.routes["doc-harbor"]}, nil, 0)
	eps := map[string]bool{}
	for _, c := range got.Clips {
		if eps[c.EpisodeGUID] {
			t.Fatalf("J12: two clips of one story from one episode: %+v", got.Clips)
		}
		eps[c.EpisodeGUID] = true
	}
	if len(got.Clips) != 2 || sum1.Covered != 1 || sum1.Clips != 2 {
		t.Fatalf("J12: clips %d, summary %+v (two episodes, one clip each)", len(got.Clips), sum1)
	}
	// Run again: the stored pairs are not asked about.
	judge.mu.Lock()
	judge.states, judge.qs = nil, nil
	judge.mu.Unlock()
	sum2, err := f.svc.MatchPodcasts(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	// Both episodes already gave the harbor story (story 0) a clip: nothing is
	// asked about it (one clip per episode per story, across runs). A story that
	// got no clip, a spurious candidate the judge said no to, is asked again.
	if sum2.Clips != 0 {
		t.Fatalf("J11: the second run stored %d clips", sum2.Clips)
	}
	for _, qs := range judge.qs {
		for name := range qs {
			if strings.HasPrefix(name, "q0_") {
				t.Fatalf("J11: the harbor story was asked about again: %s", name)
			}
		}
	}
	if sum2.Pairs >= sum1.Pairs {
		t.Fatalf("J11: the second run asked %d pairs, the first %d", sum2.Pairs, sum1.Pairs)
	}
	_ = id1
}
