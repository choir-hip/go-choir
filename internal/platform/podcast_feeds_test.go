package platform

// Subscriptions reported by phones (docs/design/podcast-clips-2026-11-15.md
// section 3.2). Ways it can fail, written first:
//   B1. two spellings of one feed are two feeds;
//   B2. an anonymous report counts toward popularity, or one account counts
//       twice for one feed, or the popular list shows a hash or a feed under
//       three accounts with a count;
//   B3. a report with a bad item writes the good ones before it, or more than
//       50 feeds are taken, or the day's new-feed bound is not a bound;
//   B4. the worker's queue is not now-playing first, then the most followed
//       feeds, then the rest;
//   B5. a transcript is not found when the phone spells the feed another way,
//       or an episode without one answers anything but nil;
//   B6. the clips feed filter drops clips instead of ordering them.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestB1_FeedSpellingsAreOneFeed(t *testing.T) {
	want := "https://feeds.example.test/news.xml"
	for _, in := range []string{"https://feeds.example.test/news.xml", "HTTPS://Feeds.Example.test:443/news.xml/#frag", " https://feeds.example.test/news.xml/ "} {
		got, err := NormalizeFeedURL(in)
		if err != nil || got != want {
			t.Errorf("NormalizeFeedURL(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "ftp://x.example/a", "https://u:p@x.example/a", "javascript:1", "https://" + strings.Repeat("a", 3000)} {
		if _, err := NormalizeFeedURL(bad); !errors.Is(err, ErrPodcastInvalid) {
			t.Errorf("NormalizeFeedURL(%q) accepted: %v", truncatePodcast(bad, 30), err)
		}
	}
}

func report(t *testing.T, f frontFixture, reporter string, urls ...string) *PodcastFeedsResponse {
	t.Helper()
	var feeds []PodcastFeedReport
	for _, u := range urls {
		feeds = append(feeds, PodcastFeedReport{FeedURL: u, Title: "Show " + u[len(u)-5:]})
	}
	resp, err := f.svc.ReportPodcastFeeds(context.Background(), reporter, PodcastFeedsRequest{Feeds: feeds})
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestB2_PopularCountsDistinctAccountsAndShowsNoOne(t *testing.T) {
	f := newFrontFixture(t)
	ctx := context.Background()
	a, b := "https://a.example.test/a.xml", "https://b.example.test/b.xml"
	// A seed feed arrives through the operator's intake.
	if _, _, err := f.svc.RecordPodcastEpisodes(ctx, []PodcastEpisodeIn{podcastFixtureEpisode("g1", 600)}); err != nil {
		t.Fatal(err)
	}
	report(t, f, "", a) // anonymous: in intake, counts for nothing
	resp := report(t, f, PodcastReporter("acct-1"), a, b)
	if resp.Accepted != 2 || resp.NewFeeds != 1 {
		t.Fatalf("B2: %+v (a was already known, b is new)", resp)
	}
	report(t, f, PodcastReporter("acct-1"), a) // the same account twice
	report(t, f, PodcastReporter("acct-2"), a)
	list, err := f.svc.ListPodcastPopular(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range list {
		if p.FeedURL == a {
			t.Fatalf("B2: a feed under %d accounts is listed with a count: %+v", podcastPopularMin, p)
		}
	}
	report(t, f, PodcastReporter("acct-3"), a)
	list, _ = f.svc.ListPodcastPopular(ctx, 0)
	if len(list) < 1 || list[0].FeedURL != a || list[0].Subscribers != 3 {
		t.Fatalf("B2: popular = %+v, want %s first with 3", list, a)
	}
	var sawFeatured, sawB bool
	for _, p := range list[1:] {
		sawFeatured = sawFeatured || (p.FeedURL == "https://feeds.example.test/news.xml" && p.Subscribers == 0)
		sawB = sawB || p.FeedURL == b
	}
	if !sawFeatured || sawB {
		t.Fatalf("B2: featured seed feed listed %v, a one-account feed listed %v: %+v", sawFeatured, sawB, list)
	}
	raw, _ := json.Marshal(list)
	for _, banned := range []string{"sub-", "acct-", "subscribers\":[", "hash"} {
		if strings.Contains(string(raw), banned) {
			t.Fatalf("B2: the popular list leaks %q: %s", banned, raw)
		}
	}
}

func TestB3_ReportBounds(t *testing.T) {
	f := newFrontFixture(t)
	ctx := context.Background()
	var many []PodcastFeedReport
	for i := 0; i <= podcastMaxFeedsPerReport; i++ {
		many = append(many, PodcastFeedReport{FeedURL: fmt.Sprintf("https://h%d.example.test/f.xml", i)})
	}
	if _, err := f.svc.ReportPodcastFeeds(ctx, "", PodcastFeedsRequest{Feeds: many}); !errors.Is(err, ErrPodcastInvalid) {
		t.Fatalf("B3: %d feeds taken: %v", len(many), err)
	}
	good := PodcastFeedReport{FeedURL: "https://good.example.test/f.xml"}
	for name, bad := range map[string]PodcastFeedsRequest{
		"bad url":       {Feeds: []PodcastFeedReport{good, {FeedURL: "ftp://x.example/a"}}},
		"long title":    {Feeds: []PodcastFeedReport{good, {FeedURL: "https://x.example.test/a", Title: strings.Repeat("t", 201)}}},
		"empty":         {},
		"now no guid":   {NowPlaying: &PodcastNowPlaying{FeedURL: "https://x.example.test/a", AudioURL: "https://x.example.test/a.mp3"}},
		"now bad audio": {NowPlaying: &PodcastNowPlaying{FeedURL: "https://x.example.test/a", EpisodeGUID: "g", AudioURL: "file:///etc/passwd"}},
	} {
		if _, err := f.svc.ReportPodcastFeeds(ctx, "", bad); !errors.Is(err, ErrPodcastInvalid) {
			t.Fatalf("B3 %s: %v", name, err)
		}
	}
	feeds, _ := f.svc.ListPodcastFeedsForWorker(ctx)
	if len(feeds) != 0 {
		t.Fatalf("B3: refused reports wrote %d feeds", len(feeds))
	}
	// The day's bound on new feeds: the rest are ignored, not refused.
	f.svc.podcastNewFeeds.mu.Lock()
	f.svc.podcastNewFeeds.day, f.svc.podcastNewFeeds.n = time.Now().UTC().Format("2006-01-02"), podcastMaxNewFeedsPerDay
	f.svc.podcastNewFeeds.mu.Unlock()
	resp, err := f.svc.ReportPodcastFeeds(ctx, "", PodcastFeedsRequest{Feeds: []PodcastFeedReport{good}})
	if err != nil || resp.NewFeeds != 0 {
		t.Fatalf("B3: past the day's bound: %+v %v", resp, err)
	}
	if feeds, _ := f.svc.ListPodcastFeedsForWorker(ctx); len(feeds) != 0 {
		t.Fatalf("B3: a feed past the bound was stored")
	}
}

func TestB4_TheWorkersQueueIsNowPlayingThenMostFollowedThenTheRest(t *testing.T) {
	f := newFrontFixture(t)
	ctx := context.Background()
	mk := func(feed, guid string, ago time.Duration) PodcastEpisodeIn {
		e := podcastFixtureEpisode(guid, 600)
		e.FeedURL = feed
		e.Published = time.Now().UTC().Add(-ago).Format(time.RFC3339)
		return e
	}
	seed, popular, niche := "https://seed.example.test/f.xml", "https://popular.example.test/f.xml", "https://niche.example.test/f.xml"
	_, _, err := f.svc.RecordPodcastEpisodes(ctx, []PodcastEpisodeIn{
		mk(seed, "seed-new", time.Hour), mk(popular, "pop-old", 5*time.Hour), mk(popular, "pop-new", 2*time.Hour), mk(niche, "niche-old", 9*time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, acct := range []string{"a", "b", "c"} {
		report(t, f, PodcastReporter(acct), popular)
	}
	report(t, f, PodcastReporter("a"), niche)
	order := func() string {
		list, err := f.svc.ListPodcastEpisodes(ctx, true, time.Time{}, 50)
		if err != nil {
			t.Fatal(err)
		}
		var g []string
		for _, e := range list {
			g = append(g, e.GUID)
		}
		return strings.Join(g, ",")
	}
	if got, want := order(), "pop-new,pop-old,niche-old,seed-new"; got != want {
		t.Fatalf("B4: queue %s, want %s", got, want)
	}
	// Someone presses play on the seed show's episode: it goes first.
	resp, err := f.svc.ReportPodcastFeeds(ctx, PodcastReporter("d"), PodcastFeedsRequest{NowPlaying: &PodcastNowPlaying{FeedURL: seed + "/", EpisodeGUID: "seed-new", AudioURL: "https://cdn.example.test/seed-new.mp3"}})
	if err != nil || !resp.NowPlayingQueued {
		t.Fatalf("B4: %+v %v", resp, err)
	}
	if got, want := order(), "seed-new,pop-new,pop-old,niche-old"; got != want {
		t.Fatalf("B4: queue %s, want %s", got, want)
	}
	// Playing an episode counts nobody as a subscriber of its feed.
	feeds, _ := f.svc.ListPodcastFeedsForWorker(ctx)
	for _, fd := range feeds {
		if fd.FeedURL == seed && (fd.Subscribers != 0 || len(fd.Wanted) != 1) {
			t.Fatalf("B4: seed feed = %+v", fd)
		}
	}
}

func TestB5_TranscriptByFeedAndGuidWhateverTheSpelling(t *testing.T) {
	f := newFrontFixture(t)
	ctx := context.Background()
	ep := podcastFixtureEpisode("g1", 600)
	if _, _, err := f.svc.RecordPodcastEpisodes(ctx, []PodcastEpisodeIn{ep}); err != nil {
		t.Fatal(err)
	}
	if tr, err := f.svc.GetPodcastTranscriptFor(ctx, ep.FeedURL, "g1"); err != nil || tr != nil {
		t.Fatalf("B5: an episode with no transcript answered %+v %v", tr, err)
	}
	id := PodcastEpisodeID(ep.FeedURL, "g1")
	if _, err := f.svc.PutPodcastTranscript(ctx, PodcastTranscript{EpisodeID: id, Engine: "parakeet.cpp test", AudioSeconds: 600, FileSeconds: 612.5,
		Segments: []PodcastSegment{{Start: 1, End: 5, Text: "first block"}, {Start: 5, End: 9, Text: "second block"}}}); err != nil {
		t.Fatal(err)
	}
	for _, spelling := range []string{ep.FeedURL, "HTTPS://FEEDS.EXAMPLE.TEST/news.xml/"} {
		tr, err := f.svc.GetPodcastTranscriptFor(ctx, spelling, "g1")
		if err != nil || tr == nil {
			t.Fatalf("B5: %q: %+v %v", spelling, tr, err)
		}
		if len(tr.Blocks) != 2 || tr.Blocks[1].Text != "second block" || tr.Source != "parakeet.cpp test" || !tr.Verified ||
			tr.Episode.Show != "Test Show" || tr.Episode.GUID != "g1" || tr.Episode.DurationSeconds != 612.5 || tr.Episode.AudioURL == "" {
			t.Fatalf("B5: transcript = %+v", tr)
		}
	}
	if tr, _ := f.svc.GetPodcastTranscriptFor(ctx, ep.FeedURL, "nope"); tr != nil {
		t.Fatalf("B5: an unknown guid answered %+v", tr)
	}
	if _, err := f.svc.GetPodcastTranscriptFor(ctx, "ftp://x.example/a", "g"); !errors.Is(err, ErrPodcastInvalid) {
		t.Fatalf("B5: bad feed: %v", err)
	}
}

func TestB6_TheClipsFeedFilterOrdersAndNeverDrops(t *testing.T) {
	f := newFrontFixture(t)
	ctx := context.Background()
	a, b := podcastFixtureEpisode("ga", 900), podcastFixtureEpisode("gb", 900)
	b.FeedURL = "https://other.example.test/b.xml"
	b.Show = "Other Show"
	if _, _, err := f.svc.RecordPodcastEpisodes(ctx, []PodcastEpisodeIn{a, b}); err != nil {
		t.Fatal(err)
	}
	idA, idB := PodcastEpisodeID(a.FeedURL, "ga"), PodcastEpisodeID(b.FeedURL, "gb")
	bridge, harbor := f.routes["doc-bridge"], f.routes["doc-harbor"]
	if _, err := f.svc.PutPodcastClips(ctx, []PodcastClipIn{
		{StoryRoute: harbor, EpisodeID: idA, Start: 10, End: 50, Quote: "from a", Support: 0.9},
		{StoryRoute: bridge, EpisodeID: idB, Start: 10, End: 50, Quote: "from b", Support: 0.9},
	}); err != nil {
		t.Fatal(err)
	}
	f.svc.podcastCache.reset()
	plain, _ := f.svc.ListPodcastClips(ctx, nil, nil, 0)
	mine, err := f.svc.ListPodcastClips(ctx, nil, []string{"HTTPS://other.example.test/b.xml/"}, 0)
	if err != nil || len(mine.Clips) != len(plain.Clips) || len(plain.Clips) != 2 {
		t.Fatalf("B6: %v plain %d filtered %d", err, len(plain.Clips), len(mine.Clips))
	}
	if mine.Clips[0].Show != "Other Show" {
		t.Fatalf("B6: the reader's own show is not first: %+v", mine.Clips)
	}
}
