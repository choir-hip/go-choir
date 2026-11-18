package platform

// Podcast clips in Store B (docs/design/podcast-clips-2026-11-15.md). Ways
// it can fail, written before the code:
//   1. an episode with a URL that carries credentials, or is not http(s), is
//      stored (the service stores URLs and the phone streams them);
//   2. a bad item in a request leaves the good ones before it written;
//   3. a re-poll of a feed forgets that an episode has a transcript, so it is
//      transcribed twice;
//   4. a transcript for an unknown episode is accepted;
//   5. a clip for a story that is not on the front, a range under 5 s or over
//      120 s, or one past the end of the episode is accepted;
//   6. the public read leaks transcript text, or a quote over 280 runes;
//   7. the public read shows more than three clips for one story, or clips
//      of a story that was asked for by another route;
//   8. a clip survives the same (story, episode, start) written twice as two
//      clips;
//   9. the write routes answer a caller that is not the host.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func podcastFixtureEpisode(guid string, secs float64) PodcastEpisodeIn {
	return PodcastEpisodeIn{
		FeedURL: "https://feeds.example.test/news.xml", GUID: guid, Show: "Test Show", Title: "Episode " + guid,
		Published: time.Now().UTC().Add(-2 * time.Hour).Format(time.RFC3339), DurationSeconds: secs,
		EnclosureURL: "https://cdn.example.test/" + guid + ".mp3", Notes: "show notes",
	}
}

// listFeeds makes feeds public the way the operator does.
func listFeeds(t *testing.T, f frontFixture, feedURLs ...string) {
	t.Helper()
	var list []PodcastFeedListing
	for _, u := range feedURLs {
		list = append(list, PodcastFeedListing{FeedURL: u, Title: "Listed"})
	}
	if _, err := f.svc.SetPodcastFeedsListed(context.Background(), list, true); err != nil {
		t.Fatal(err)
	}
}

// seedTranscript stores an episode's transcript of one segment every 10 s up to
// seconds, each with text (so any quote of that text sits inside any clip).
func seedTranscript(t *testing.T, f frontFixture, episodeID string, seconds float64, text string, override map[int]string) {
	t.Helper()
	var segs []PodcastSegment
	for i := 0; float64(i)*10+9.5 <= seconds; i++ {
		txt := text
		if o, ok := override[i]; ok {
			txt = o
		}
		segs = append(segs, PodcastSegment{Start: float64(i) * 10, End: float64(i)*10 + 9.5, Text: txt})
	}
	if _, _, err := f.svc.PutPodcastTranscript(context.Background(), PodcastTranscript{EpisodeID: episodeID, Engine: "test", AudioSeconds: seconds, Segments: segs}); err != nil {
		t.Fatal(err)
	}
}

// recordFeatured features the feeds of the episodes (the operator's act) and
// records them: a feed row, and so an episode, exists only for a featured feed.
func recordFeatured(f frontFixture, ctx context.Context, eps []PodcastEpisodeIn) (recorded, fresh, skipped int, err error) {
	seen := map[string]bool{}
	var list []PodcastFeedListing
	for _, e := range eps {
		if _, nerr := NormalizeFeedURL(e.FeedURL); nerr == nil && !seen[e.FeedURL] {
			seen[e.FeedURL] = true
			list = append(list, PodcastFeedListing{FeedURL: e.FeedURL, Title: "Listed"})
		}
	}
	if len(list) > 0 {
		if _, err := f.svc.SetPodcastFeedsListed(ctx, list, true); err != nil {
			return 0, 0, 0, err
		}
	}
	return f.svc.RecordPodcastEpisodes(ctx, eps)
}

// One bad item costs only itself; only a malformed envelope is refused.
func TestPodcastEpisodesSkipBadItemsAndKeepTheRest(t *testing.T) {
	f := newFrontFixture(t)
	ctx := context.Background()
	listFeeds(t, f, "https://feeds.example.test/news.xml")
	good := podcastFixtureEpisode("g1", 600)
	bads := map[string]func(*PodcastEpisodeIn){
		"credentials":  func(e *PodcastEpisodeIn) { e.EnclosureURL = "https://user:pw@cdn.example.test/a.mp3" },
		"ftp":          func(e *PodcastEpisodeIn) { e.EnclosureURL = "ftp://cdn.example.test/a.mp3" },
		"private":      func(e *PodcastEpisodeIn) { e.EnclosureURL = "http://192.168.1.10/a.mp3" },
		"feed":         func(e *PodcastEpisodeIn) { e.FeedURL = "javascript:alert(1)" },
		"no guid":      func(e *PodcastEpisodeIn) { e.GUID = "" },
		"bad date":     func(e *PodcastEpisodeIn) { e.Published = "yesterday" },
		"huge":         func(e *PodcastEpisodeIn) { e.EnclosureURL = "https://x.example.test/" + strings.Repeat("a", 3000) },
		"future":       func(e *PodcastEpisodeIn) { e.Published = time.Now().UTC().Add(48 * time.Hour).Format(time.RFC3339) },
		"not featured": func(e *PodcastEpisodeIn) { e.FeedURL = "https://private.example.test/token/f.xml" },
	}
	var in []PodcastEpisodeIn
	for name, mutate := range bads {
		bad := podcastFixtureEpisode("bad-"+strings.ReplaceAll(name, " ", "-"), 600)
		mutate(&bad)
		in = append(in, bad)
	}
	in = append(in, good)
	rec, fresh, skipped, err := f.svc.RecordPodcastEpisodes(ctx, in)
	if err != nil || rec != 1 || fresh != 1 || skipped != len(bads) {
		t.Fatalf("recorded %d new %d skipped %d (%v); want 1, 1 and %d", rec, fresh, skipped, err, len(bads))
	}
	list, _ := f.svc.ListPodcastEpisodes(ctx, false, time.Time{}, 50)
	if len(list) != 1 || list[0].GUID != "g1" {
		t.Fatalf("the store holds %+v, want the one good episode", list)
	}
	// Only the envelope can be refused whole.
	if _, _, _, err := f.svc.RecordPodcastEpisodes(ctx, nil); !errors.Is(err, ErrPodcastInvalid) {
		t.Fatalf("an empty request: %v", err)
	}
	if _, _, _, err := f.svc.RecordPodcastEpisodes(ctx, make([]PodcastEpisodeIn, podcastMaxEpisodesPerRequest+1)); !errors.Is(err, ErrPodcastInvalid) {
		t.Fatalf("an oversized request: %v", err)
	}
}

func TestPodcastRepollKeepsTranscriptFlag(t *testing.T) {
	f := newFrontFixture(t)
	ctx := context.Background()
	ep := podcastFixtureEpisode("g1", 600)
	if rec, fresh, _, err := recordFeatured(f, ctx, []PodcastEpisodeIn{ep}); err != nil || rec != 1 || fresh != 1 {
		t.Fatalf("first record: %d %d %v", rec, fresh, err)
	}
	id := PodcastEpisodeID(ep.FeedURL, ep.GUID)
	if _, _, err := f.svc.PutPodcastTranscript(ctx, PodcastTranscript{EpisodeID: "ep-unknown", Engine: "t", AudioSeconds: 60, Segments: []PodcastSegment{{Start: 0, End: 5, Text: "x"}}}); !errors.Is(err, ErrPodcastInvalid) {
		t.Fatalf("a transcript of an unknown episode: err = %v", err)
	}
	if n, kept, err := f.svc.PutPodcastTranscript(ctx, PodcastTranscript{EpisodeID: id, Engine: "t", AudioSeconds: 600, FileSeconds: 655.5, Segments: []PodcastSegment{{Start: 0, End: 5, Text: "one"}, {Start: 5, End: 9, Text: "two"}}}); err != nil || n != 2 || kept {
		t.Fatalf("put transcript: %d %v", n, err)
	}
	need, _ := f.svc.ListPodcastEpisodes(ctx, true, time.Time{}, 50)
	if len(need) != 0 {
		t.Fatalf("an episode with a transcript is still listed as needing one: %+v", need)
	}
	if rec, fresh, _, err := recordFeatured(f, ctx, []PodcastEpisodeIn{ep}); err != nil || rec != 1 || fresh != 0 {
		t.Fatalf("re-record: %d %d %v", rec, fresh, err)
	}
	need, _ = f.svc.ListPodcastEpisodes(ctx, true, time.Time{}, 50)
	if len(need) != 0 {
		t.Fatalf("a re-poll forgot the transcript: %+v", need)
	}
	all, _ := f.svc.ListPodcastEpisodes(ctx, false, time.Time{}, 50)
	if len(all) != 1 || !all[0].HasTranscript {
		t.Fatalf("episodes = %+v", all)
	}
	// Ad insertion: the length measured on the transcribed file (655.5 s) is
	// the one a clip is valid for, not the feed's 600 s, and a re-poll keeps it.
	if all[0].DurationSeconds != 655.5 {
		t.Fatalf("duration_seconds = %v, want the measured 655.5", all[0].DurationSeconds)
	}
	// Out of order or empty segments are refused.
	for name, segs := range map[string][]PodcastSegment{
		"empty text":   {{Start: 0, End: 5, Text: " "}},
		"backwards":    {{Start: 5, End: 4, Text: "x"}},
		"out of order": {{Start: 10, End: 20, Text: "x"}, {Start: 1, End: 3, Text: "y"}},
		"no segments":  nil,
		"past a day":   {{Start: 0, End: 90000, Text: "x"}},
	} {
		if _, _, err := f.svc.PutPodcastTranscript(ctx, PodcastTranscript{EpisodeID: id, Engine: "t", Segments: segs}); !errors.Is(err, ErrPodcastInvalid) {
			t.Fatalf("%s: err = %v, want ErrPodcastInvalid", name, err)
		}
	}
}

func TestPodcastClipsValidationAndPublicRead(t *testing.T) {
	f := newFrontFixture(t)
	ctx := context.Background()
	listFeeds(t, f, "https://feeds.example.test/news.xml")
	if _, _, _, err := recordFeatured(f, ctx, []PodcastEpisodeIn{podcastFixtureEpisode("g1", 900)}); err != nil {
		t.Fatal(err)
	}
	id := PodcastEpisodeID("https://feeds.example.test/news.xml", "g1")
	seedTranscript(t, f, id, 900, "the harbor reopened after repairs", map[int]string{50: strings.Repeat("harbor ", 250)})
	harbor, bridge := f.routes["doc-harbor"], f.routes["doc-bridge"]
	long := strings.Repeat("harbor ", 100)
	clip := func(route string, start, end float64) PodcastClipIn {
		return PodcastClipIn{StoryRoute: route, EpisodeID: id, Start: start, End: end, Quote: "the harbor reopened", Support: 0.9}
	}
	for name, in := range map[string]PodcastClipIn{
		"not on the front":  clip("/nope", 10, 50),
		"under five":        clip(harbor, 10, 12),
		"over two minutes":  clip(harbor, 10, 200),
		"past the episode":  clip(harbor, 880, 930),
		"unknown episode":   {StoryRoute: harbor, EpisodeID: "ep-x", Start: 1, End: 30, Quote: "q", Support: 0.9},
		"support too high":  {StoryRoute: harbor, EpisodeID: id, Start: 1, End: 30, Quote: "the harbor", Support: 1.5},
		"empty quote":       {StoryRoute: harbor, EpisodeID: id, Start: 1, End: 30, Quote: "  ", Support: 0.9},
		"words nobody said": {StoryRoute: harbor, EpisodeID: id, Start: 100, End: 140, Quote: "the minister resigned", Support: 0.9},
		"said elsewhere":    {StoryRoute: harbor, EpisodeID: id, Start: 100, End: 140, Quote: long, Support: 0.9}, // the long run of harbor is at 500 s
	} {
		if _, err := f.svc.PutPodcastClips(ctx, []PodcastClipIn{clip(harbor, 100, 140), in}); !errors.Is(err, ErrPodcastInvalid) {
			t.Fatalf("%s: err = %v, want ErrPodcastInvalid", name, err)
		}
	}
	got, err := f.svc.ListPodcastClips(ctx, nil, nil, 0)
	if err != nil || len(got.Clips) != 0 {
		t.Fatalf("refused requests left %d clips (err %v)", len(got.Clips), err)
	}

	in := []PodcastClipIn{clip(harbor, 100, 140), clip(harbor, 100.2, 140), clip(harbor, 300, 350), clip(harbor, 500, 540), clip(harbor, 700, 740)}
	in[3].Quote = long
	// 100 and 100.2 round to the same second: one clip. Five proposed, four stored.
	if n, err := f.svc.PutPodcastClips(ctx, in); err != nil || n != 4 {
		t.Fatalf("put clips: %d %v", n, err)
	}
	if n, err := f.svc.PutPodcastClips(ctx, []PodcastClipIn{clip(bridge, 20, 60)}); err != nil || n != 1 {
		t.Fatalf("put bridge clip: %d %v", n, err)
	}
	f.svc.podcastCache.reset()

	// One story asked for: at most three clips, and only its own.
	one, err := f.svc.ListPodcastClips(ctx, []string{harbor}, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(one.Clips) != podcastClipsPerStory {
		t.Fatalf("harbor shows %d clips, want %d", len(one.Clips), podcastClipsPerStory)
	}
	seen := map[string]bool{}
	for _, c := range one.Clips {
		if c.Story.Route != harbor || c.Story.Title != "Harbor reopens" || c.Story.Version != 3 {
			t.Fatalf("clip story = %+v", c.Story)
		}
		if c.Show != "Test Show" || c.AudioURL != "https://cdn.example.test/g1.mp3" || c.EpisodeGUID != "g1" || c.DurationSeconds != 900 || c.FeedURL == "" || c.Published.IsZero() {
			t.Fatalf("clip is not joined with its episode: %+v", c)
		}
		if seen[c.ClipID] {
			t.Fatalf("clip %s listed twice", c.ClipID)
		}
		seen[c.ClipID] = true
		if len([]rune(c.Quote)) > podcastQuoteMaxRunes || strings.Contains(c.Quote, "…") {
			t.Fatalf("quote is %d runes or carries an ellipsis: %q", len([]rune(c.Quote)), c.Quote)
		}
	}
	// No routes: the recent front, both stories.
	bulk, err := f.svc.ListPodcastClips(ctx, nil, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	routes := map[string]int{}
	for _, c := range bulk.Clips {
		routes[c.Story.Route]++
	}
	if routes[harbor] != 3 || routes[bridge] != 1 || len(routes) != 2 {
		t.Fatalf("bulk read by story = %v", routes)
	}
	// The public shape carries no transcript field at all.
	raw, _ := json.Marshal(bulk)
	if bytes.Contains(raw, []byte("segments")) || bytes.Contains(raw, []byte("transcript")) {
		t.Fatalf("public clips leak the transcript: %s", raw)
	}
	// Bounds on what a stranger can ask.
	many := make([]string, podcastClipsMaxRoutes+1)
	for i := range many {
		many[i] = "/r"
	}
	if _, err := f.svc.ListPodcastClips(ctx, many, nil, 0); !errors.Is(err, ErrPodcastInvalid) {
		t.Fatalf("too many routes: err = %v", err)
	}
	capped, _ := f.svc.ListPodcastClips(ctx, nil, nil, 1)
	if len(capped.Clips) != 1 {
		t.Fatalf("limit 1 gave %d clips", len(capped.Clips))
	}
}

func TestCutQuoteIsAPrefixOnAWordBoundary(t *testing.T) {
	long := strings.Repeat("word ", 100)
	q := CutQuote(long, 280)
	if len([]rune(q)) > 280 || strings.Contains(q, "…") || !strings.HasPrefix(strings.Join(strings.Fields(long), " "), q) || strings.HasSuffix(q, " ") {
		t.Fatalf("CutQuote = %q", q)
	}
	if got := CutQuote("  short   quote ", 280); got != "short quote" {
		t.Fatalf("CutQuote = %q", got)
	}
	if got := CutQuote(strings.Repeat("x", 400), 280); len(got) != 280 {
		t.Fatalf("a word longer than the bound is cut at the bound: %d", len(got))
	}
}

func TestPodcastRoutesRefuseAnyoneButTheHost(t *testing.T) {
	f := newFrontFixture(t)
	h := NewHandler(f.svc)
	for _, tc := range []struct {
		method, path string
		fn           http.HandlerFunc
	}{
		{http.MethodGet, PodcastClipsPath, h.HandlePodcastClips},
		{http.MethodPost, PodcastEpisodesPath, h.HandlePodcastEpisodes},
		{http.MethodGet, PodcastEpisodesPath, h.HandlePodcastEpisodes},
		{http.MethodPut, PodcastTranscriptsPath, h.HandlePodcastTranscripts},
		{http.MethodPut, PodcastClipsWritePath, h.HandlePodcastClipsWrite},
	} {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(`{}`))
		req.RemoteAddr = "10.200.0.9:4444"
		rec := httptest.NewRecorder()
		tc.fn(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("%s %s from a guest address answered %d, want 403", tc.method, tc.path, rec.Code)
		}
		wrong := httptest.NewRequest(http.MethodPatch, tc.path, nil)
		rec = httptest.NewRecorder()
		tc.fn(rec, wrong)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("PATCH %s answered %d, want 405", tc.path, rec.Code)
		}
	}
}

// A clip of a file the publisher served unchanged is shown before a clip of an
// ad-stitched file, whatever their support (docs/design/podcast-clips-2026-11-15.md R4).
func TestPodcastClipsOfAStableFileComeFirst(t *testing.T) {
	f := newFrontFixture(t)
	ctx := context.Background()
	stable, unstable := podcastFixtureEpisode("stable", 900), podcastFixtureEpisode("unstable", 900)
	listFeeds(t, f, stable.FeedURL)
	if _, _, _, err := recordFeatured(f, ctx, []PodcastEpisodeIn{stable, unstable}); err != nil {
		t.Fatal(err)
	}
	yes, no := true, false
	seg := []PodcastSegment{}
	for i := 0; i < 90; i++ {
		seg = append(seg, PodcastSegment{Start: float64(i) * 10, End: float64(i)*10 + 9.5, Text: "u1 u2 s1 s2"})
	}
	idS, idU := PodcastEpisodeID(stable.FeedURL, "stable"), PodcastEpisodeID(unstable.FeedURL, "unstable")
	if _, _, err := f.svc.PutPodcastTranscript(ctx, PodcastTranscript{EpisodeID: idS, Engine: "t", AudioSeconds: 900, StableFile: &yes, Segments: seg}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := f.svc.PutPodcastTranscript(ctx, PodcastTranscript{EpisodeID: idU, Engine: "t", AudioSeconds: 900, StableFile: &no, Segments: seg}); err != nil {
		t.Fatal(err)
	}
	harbor := f.routes["doc-harbor"]
	clips := []PodcastClipIn{
		{StoryRoute: harbor, EpisodeID: idU, Start: 10, End: 50, Quote: "u1", Support: 0.99},
		{StoryRoute: harbor, EpisodeID: idU, Start: 100, End: 140, Quote: "u2", Support: 0.98},
		{StoryRoute: harbor, EpisodeID: idS, Start: 200, End: 240, Quote: "s1", Support: 0.7},
		{StoryRoute: harbor, EpisodeID: idS, Start: 300, End: 340, Quote: "s2", Support: 0.65},
	}
	if _, err := f.svc.PutPodcastClips(ctx, clips); err != nil {
		t.Fatal(err)
	}
	f.svc.podcastCache.reset()
	got, err := f.svc.ListPodcastClips(ctx, []string{harbor}, nil, 0)
	if err != nil || len(got.Clips) != 3 {
		t.Fatalf("clips: %v %+v", err, got)
	}
	order := got.Clips[0].Quote + "," + got.Clips[1].Quote + "," + got.Clips[2].Quote
	if order != "s1,s2,u1" {
		t.Fatalf("order = %s, want the stable file's clips first (s1,s2,u1)", order)
	}
	// A re-poll of the feed keeps what the worker saw.
	if _, _, _, err := recordFeatured(f, ctx, []PodcastEpisodeIn{stable}); err != nil {
		t.Fatal(err)
	}
	list, _ := f.svc.ListPodcastEpisodes(ctx, false, time.Time{}, 10)
	for _, e := range list {
		if e.EpisodeID == idS && (e.StableFile == nil || !*e.StableFile) {
			t.Fatalf("a re-poll forgot the stable flag: %+v", e)
		}
	}
}

// A poll that finds nothing new writes nothing: the feeds are polled every
// half hour and a rewrite of every item each time would be churn.
func TestPodcastRepeatPollWritesNothingUnlessSomethingChanged(t *testing.T) {
	f := newFrontFixture(t)
	ctx := context.Background()
	ep := podcastFixtureEpisode("g1", 600)
	if _, _, _, err := recordFeatured(f, ctx, []PodcastEpisodeIn{ep}); err != nil {
		t.Fatal(err)
	}
	canonical, _ := podcastCanonical(ogKindPodcastEpisode, PodcastEpisodeID(ep.FeedURL, ep.GUID))
	old := time.Now().UTC().Add(-48 * time.Hour).Truncate(time.Second)
	if _, err := f.store.corpus().Exec(`UPDATE og_objects SET updated_at = ? WHERE canonical_id = ?`, old, canonical); err != nil {
		t.Fatal(err)
	}
	updatedAt := func() time.Time {
		obj, err := f.svc.ogStore().GetObject(ctx, canonical)
		if err != nil {
			t.Fatal(err)
		}
		return obj.UpdatedAt.UTC()
	}
	if _, _, _, err := recordFeatured(f, ctx, []PodcastEpisodeIn{ep}); err != nil {
		t.Fatal(err)
	}
	if !updatedAt().Equal(old) {
		t.Fatalf("an unchanged item was rewritten: updated_at %v, want %v", updatedAt(), old)
	}
	ep.Title = "A new title"
	if _, _, _, err := recordFeatured(f, ctx, []PodcastEpisodeIn{ep}); err != nil {
		t.Fatal(err)
	}
	if updatedAt().Equal(old) {
		t.Fatal("a changed item was not written")
	}
}

// Test seams for the lost-index test.
func mustIndexCanonical(t *testing.T, key string) string {
	t.Helper()
	id, err := podcastCanonical(ogKindPodcastIndex, key)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func onceReset() sync.Once { return sync.Once{} }

// captureStdLog sends the standard logger's output to w until restored.
func captureStdLog(w *strings.Builder) func() {
	prev := log.Writer()
	log.SetOutput(w)
	return func() { log.SetOutput(prev) }
}
