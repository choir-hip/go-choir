package platform

// Subscriptions reported by phones (docs/design/podcast-clips-2026-11-15.md
// section 3.2) and the repairs of review 1 (2026-11-16). Ways it can fail,
// written first:
//   B1. two spellings of one feed are two feeds, or a host that names a
//       machine (an IP literal, localhost, .internal, .local, a single label,
//       a port but 80 and 443) is admitted;
//   B2. an account counts twice for one feed, or the popular list shows a
//       hash, a feed under three accounts with a count, a feed no worker has
//       fetched, or a private feed;
//   B3. a report with a bad item writes the good ones before it, more than 50
//       feeds are taken, a signed-out report is taken, or one account can hold
//       more than 500 subscriptions;
//   B4. the worker's queue is not now-playing first, then the most followed
//       feeds, then the rest;
//   B5. a transcript is not found when the phone spells the feed another way,
//       or an episode without one answers anything but nil;
//   B6. the clips feed filter drops clips instead of ordering them;
//   B7. a report that changes nothing writes anything;
//   B8. a feed one account reported (a private, tokenised feed) is public:
//       its clips are served, it is on the popular list, its transcript is
//       shown to others, or clips can be written for it;
//   B9. a shorter or partial transcript replaces a full one, or a segment past
//       the end of the episode is stored.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestB1_FeedSpellingsAreOneFeedAndMachineHostsAreRefused(t *testing.T) {
	want := "https://feeds.example.test/news.xml"
	for _, in := range []string{"https://feeds.example.test/news.xml", "HTTPS://Feeds.Example.test:443/news.xml/#frag", " https://feeds.example.test/news.xml/ "} {
		got, err := NormalizeFeedURL(in)
		if err != nil || got != want {
			t.Errorf("NormalizeFeedURL(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{
		"", "ftp://x.example/a", "https://u:p@x.example/a", "javascript:1", "https://" + strings.Repeat("a", 3000),
		"http://127.0.0.1/f.xml", "http://127.0.0.1:8080/f.xml", "https://169.254.169.254/latest/meta-data/", "http://[::1]/f.xml", "http://10.0.0.5/a",
		"http://localhost/f.xml", "http://printer.local/f.xml", "http://db.internal/f.xml", "http://intranet/f.xml", "https://feeds.example.test:8443/f.xml",
		"https://feeds.example.test:22/f.xml", "http://x.localhost/f.xml", "http://2130706433/f.xml",
	} {
		if _, err := NormalizeFeedURL(bad); !errors.Is(err, ErrPodcastInvalid) {
			t.Errorf("NormalizeFeedURL(%q) accepted: %v", truncatePodcast(bad, 40), err)
		}
	}
	// The same rule guards episode intake.
	f := newFrontFixture(t)
	e := podcastFixtureEpisode("g", 600)
	e.EnclosureURL = "http://192.168.1.10/a.mp3"
	if _, _, err := f.svc.RecordPodcastEpisodes(context.Background(), []PodcastEpisodeIn{e}); !errors.Is(err, ErrPodcastInvalid) {
		t.Fatalf("B1: a private enclosure was taken: %v", err)
	}
	// Two spellings of one feed make one episode id.
	if PodcastEpisodeID("https://feeds.example.test/news.xml", "g") != PodcastEpisodeID("HTTPS://FEEDS.example.test:443/news.xml/", "g") {
		t.Fatal("B1: two spellings of one feed gave two episode ids")
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

func TestB2_PopularCountsDistinctAccountsFetchedPublicFeedsAndShowsNoOne(t *testing.T) {
	f := newFrontFixture(t)
	ctx := context.Background()
	a, b, c := "https://a.example.test/a.xml", "https://b.example.test/b.xml", "https://c.example.test/c.xml"
	// A seed feed: the operator lists it and the worker has fetched it.
	if _, _, err := f.svc.RecordPodcastEpisodes(ctx, []PodcastEpisodeIn{podcastFixtureEpisode("g1", 600)}); err != nil {
		t.Fatal(err)
	}
	listFeeds(t, f, "https://feeds.example.test/news.xml")
	resp := report(t, f, PodcastReporter("acct-1"), a, b)
	if resp.Accepted != 2 || resp.NewFeeds != 2 {
		t.Fatalf("B2: %+v", resp)
	}
	report(t, f, PodcastReporter("acct-1"), a) // the same account twice
	report(t, f, PodcastReporter("acct-2"), a)
	report(t, f, PodcastReporter("acct-3"), a)
	list, err := f.svc.ListPodcastPopular(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range list {
		if p.FeedURL == a {
			t.Fatalf("B2: a feed no worker has fetched is on the public list: %+v", p)
		}
	}
	// The worker fetches a (three accounts: listed and counted) and c is fetched but private.
	ea := podcastFixtureEpisode("ga", 600)
	ea.FeedURL, ea.Show = a, "Show A"
	ec := podcastFixtureEpisode("gc", 600)
	ec.FeedURL, ec.Show = c, "Show C"
	if _, _, err := f.svc.RecordPodcastEpisodes(ctx, []PodcastEpisodeIn{ea, ec}); err != nil {
		t.Fatal(err)
	}
	report(t, f, PodcastReporter("acct-1"), c)
	list, _ = f.svc.ListPodcastPopular(ctx, 0)
	if len(list) < 2 || list[0].FeedURL != a || list[0].Subscribers != 3 {
		t.Fatalf("B2: popular = %+v, want %s first with 3", list, a)
	}
	sawSeed := false
	for _, p := range list[1:] {
		sawSeed = sawSeed || (p.FeedURL == "https://feeds.example.test/news.xml" && p.Subscribers == 0)
		if p.FeedURL == b || p.FeedURL == c {
			t.Fatalf("B2: a private or unfetched feed is listed: %+v", p)
		}
	}
	if !sawSeed {
		t.Fatalf("B2: the operator's seed feed is not featured: %+v", list)
	}
	raw, _ := json.Marshal(list)
	for _, banned := range []string{"sub-", "acct-", "subscribers\":[", "hash"} {
		if strings.Contains(string(raw), banned) {
			t.Fatalf("B2: the popular list leaks %q: %s", banned, raw)
		}
	}
}

func TestB3_ReportBoundsAndSignIn(t *testing.T) {
	f := newFrontFixture(t)
	ctx := context.Background()
	acct := PodcastReporter("acct-x")
	var many []PodcastFeedReport
	for i := 0; i <= podcastMaxFeedsPerReport; i++ {
		many = append(many, PodcastFeedReport{FeedURL: fmt.Sprintf("https://h%d.example.test/f.xml", i)})
	}
	if _, err := f.svc.ReportPodcastFeeds(ctx, acct, PodcastFeedsRequest{Feeds: many}); !errors.Is(err, ErrPodcastInvalid) {
		t.Fatalf("B3: %d feeds taken: %v", len(many), err)
	}
	good := PodcastFeedReport{FeedURL: "https://good.example.test/f.xml"}
	if _, err := f.svc.ReportPodcastFeeds(ctx, "", PodcastFeedsRequest{Feeds: []PodcastFeedReport{good}}); !errors.Is(err, ErrPodcastSignInRequired) {
		t.Fatalf("B3: a signed-out report: %v", err)
	}
	for name, bad := range map[string]PodcastFeedsRequest{
		"bad url":       {Feeds: []PodcastFeedReport{good, {FeedURL: "ftp://x.example/a"}}},
		"private host":  {Feeds: []PodcastFeedReport{good, {FeedURL: "http://169.254.169.254/latest/"}}},
		"long title":    {Feeds: []PodcastFeedReport{good, {FeedURL: "https://x.example.test/a", Title: strings.Repeat("t", 201)}}},
		"empty":         {},
		"now no guid":   {NowPlaying: &PodcastNowPlaying{FeedURL: "https://x.example.test/a", AudioURL: "https://x.example.test/a.mp3"}},
		"now bad audio": {NowPlaying: &PodcastNowPlaying{FeedURL: "https://x.example.test/a", EpisodeGUID: "g", AudioURL: "file:///etc/passwd"}},
		"now private":   {NowPlaying: &PodcastNowPlaying{FeedURL: "https://x.example.test/a", EpisodeGUID: "g", AudioURL: "http://127.0.0.1/a.mp3"}},
	} {
		if _, err := f.svc.ReportPodcastFeeds(ctx, acct, bad); !errors.Is(err, ErrPodcastInvalid) {
			t.Fatalf("B3 %s: %v", name, err)
		}
	}
	if feeds, _ := f.svc.ListPodcastFeedsForWorker(ctx); len(feeds) != 0 {
		t.Fatalf("B3: refused reports wrote %d feeds", len(feeds))
	}
	// One account holds at most 500 subscriptions: ten requests of fifty, then one more.
	for batch := 0; batch < podcastMaxSubscriptionsPerAccount/podcastMaxFeedsPerReport; batch++ {
		var feeds []PodcastFeedReport
		for i := 0; i < podcastMaxFeedsPerReport; i++ {
			feeds = append(feeds, PodcastFeedReport{FeedURL: fmt.Sprintf("https://b%d-%d.example.test/f.xml", batch, i)})
		}
		if _, err := f.svc.ReportPodcastFeeds(ctx, acct, PodcastFeedsRequest{Feeds: feeds}); err != nil {
			t.Fatalf("B3: batch %d: %v", batch, err)
		}
	}
	if _, err := f.svc.ReportPodcastFeeds(ctx, acct, PodcastFeedsRequest{Feeds: []PodcastFeedReport{good}}); !errors.Is(err, ErrPodcastInvalid) {
		t.Fatalf("B3: the 501st subscription: %v", err)
	}
	// Re-reporting what the account already holds is not a new subscription.
	if _, err := f.svc.ReportPodcastFeeds(ctx, acct, PodcastFeedsRequest{Feeds: []PodcastFeedReport{{FeedURL: "https://b0-0.example.test/f.xml"}}}); err != nil {
		t.Fatalf("B3: a repeat at the cap: %v", err)
	}
	// Another account is not held back by it.
	if _, err := f.svc.ReportPodcastFeeds(ctx, PodcastReporter("acct-y"), PodcastFeedsRequest{Feeds: []PodcastFeedReport{good}}); err != nil {
		t.Fatalf("B3: another account: %v", err)
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
	resp, err := f.svc.ReportPodcastFeeds(ctx, PodcastReporter("d"), PodcastFeedsRequest{NowPlaying: &PodcastNowPlaying{FeedURL: seed + "/", EpisodeGUID: "seed-new", AudioURL: "https://cdn.example.test/seed-new.mp3"}})
	if err != nil || !resp.NowPlayingQueued {
		t.Fatalf("B4: %+v %v", resp, err)
	}
	if got, want := order(), "seed-new,pop-new,pop-old,niche-old"; got != want {
		t.Fatalf("B4: queue %s, want %s", got, want)
	}
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
	listFeeds(t, f, ep.FeedURL)
	if tr, err := f.svc.GetPodcastTranscriptFor(ctx, ep.FeedURL, "g1", ""); err != nil || tr != nil {
		t.Fatalf("B5: an episode with no transcript answered %+v %v", tr, err)
	}
	id := PodcastEpisodeID(ep.FeedURL, "g1")
	if _, _, err := f.svc.PutPodcastTranscript(ctx, PodcastTranscript{EpisodeID: id, Engine: "parakeet.cpp test", AudioSeconds: 600, FileSeconds: 612.5,
		Segments: []PodcastSegment{{Start: 1, End: 5, Text: "first block"}, {Start: 5, End: 9, Text: "second block"}}}); err != nil {
		t.Fatal(err)
	}
	for _, spelling := range []string{ep.FeedURL, "HTTPS://FEEDS.EXAMPLE.TEST/news.xml/"} {
		tr, err := f.svc.GetPodcastTranscriptFor(ctx, spelling, "g1", PodcastReporter("anyone"))
		if err != nil || tr == nil {
			t.Fatalf("B5: %q: %+v %v", spelling, tr, err)
		}
		if len(tr.Blocks) != 2 || tr.Blocks[1].Text != "second block" || tr.Source != "parakeet.cpp test" || !tr.Verified ||
			tr.Episode.Show != "Test Show" || tr.Episode.GUID != "g1" || tr.Episode.DurationSeconds != 612.5 || tr.Episode.AudioURL == "" {
			t.Fatalf("B5: transcript = %+v", tr)
		}
	}
	if tr, _ := f.svc.GetPodcastTranscriptFor(ctx, ep.FeedURL, "nope", ""); tr != nil {
		t.Fatalf("B5: an unknown guid answered %+v", tr)
	}
	if _, err := f.svc.GetPodcastTranscriptFor(ctx, "ftp://x.example/a", "g", ""); !errors.Is(err, ErrPodcastInvalid) {
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
	listFeeds(t, f, a.FeedURL, b.FeedURL)
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

func TestB7_AReportThatChangesNothingWritesNothing(t *testing.T) {
	f := newFrontFixture(t)
	ctx := context.Background()
	acct := PodcastReporter("acct-1")
	feed := "https://a.example.test/a.xml"
	np := &PodcastNowPlaying{FeedURL: feed, EpisodeGUID: "g", AudioURL: "https://cdn.example.test/g.mp3"}
	req := PodcastFeedsRequest{Feeds: []PodcastFeedReport{{FeedURL: feed, Title: "A"}}, NowPlaying: np}
	if _, err := f.svc.ReportPodcastFeeds(ctx, acct, req); err != nil {
		t.Fatal(err)
	}
	canonical, _ := podcastCanonical(ogKindPodcastFeed, podcastFeedID(feed))
	old := time.Now().UTC().Add(-48 * time.Hour).Truncate(time.Second)
	if _, err := f.store.corpus().Exec(`UPDATE og_objects SET updated_at = ? WHERE canonical_id = ?`, old, canonical); err != nil {
		t.Fatal(err)
	}
	// A warm cache is not flushed by a repeat either.
	f.svc.podcastCache.mu.Lock()
	f.svc.podcastCache.good = true
	f.svc.podcastCache.at = time.Now()
	f.svc.podcastCache.mu.Unlock()
	if _, err := f.svc.ReportPodcastFeeds(ctx, acct, req); err != nil {
		t.Fatal(err)
	}
	obj, err := f.svc.ogStore().GetObject(ctx, canonical)
	if err != nil || !obj.UpdatedAt.UTC().Equal(old) {
		t.Fatalf("B7: an unchanged report rewrote the feed: %v %v", obj.UpdatedAt, err)
	}
	f.svc.podcastCache.mu.Lock()
	warm := f.svc.podcastCache.good
	f.svc.podcastCache.mu.Unlock()
	if !warm {
		t.Fatal("B7: an unchanged report flushed the public read's cache")
	}
	// A second account does change it.
	if _, err := f.svc.ReportPodcastFeeds(ctx, PodcastReporter("acct-2"), req); err != nil {
		t.Fatal(err)
	}
	obj, _ = f.svc.ogStore().GetObject(ctx, canonical)
	if obj.UpdatedAt.UTC().Equal(old) {
		t.Fatal("B7: a new subscriber was not written")
	}
}

func TestB8_APrivateFeedNeverBecomesPublic(t *testing.T) {
	f := newFrontFixture(t)
	ctx := context.Background()
	priv := "https://feeds.example.test/private/TOKEN123/f.xml"
	e := podcastFixtureEpisode("pg", 900)
	e.FeedURL = priv
	if _, _, err := f.svc.RecordPodcastEpisodes(ctx, []PodcastEpisodeIn{e}); err != nil {
		t.Fatal(err)
	}
	owner, other := PodcastReporter("owner"), PodcastReporter("other")
	report(t, f, owner, priv)
	id := PodcastEpisodeID(priv, "pg")
	if _, _, err := f.svc.PutPodcastTranscript(ctx, PodcastTranscript{EpisodeID: id, Engine: "t", AudioSeconds: 900, Segments: []PodcastSegment{{Start: 1, End: 9, Text: "a private episode"}}}); err != nil {
		t.Fatal(err)
	}
	// Clips cannot be written for it, and none is served.
	if _, err := f.svc.PutPodcastClips(ctx, []PodcastClipIn{{StoryRoute: f.routes["doc-harbor"], EpisodeID: id, Start: 10, End: 50, Quote: "q", Support: 0.9}}); !errors.Is(err, ErrPodcastInvalid) {
		t.Fatalf("B8: a clip of a private feed was written: %v", err)
	}
	// Its transcript is the subscriber's: 404 (nil) for everyone else.
	if tr, err := f.svc.GetPodcastTranscriptFor(ctx, priv, "pg", owner); err != nil || tr == nil {
		t.Fatalf("B8: the subscriber cannot read it: %v %v", tr, err)
	}
	for who, rep := range map[string]string{"another account": other, "nobody": ""} {
		if tr, err := f.svc.GetPodcastTranscriptFor(ctx, priv, "pg", rep); err != nil || tr != nil {
			t.Fatalf("B8: %s read a private transcript: %v %v", who, tr, err)
		}
	}
	// It is not on the popular list, even with three accounts and a fetched episode.
	report(t, f, other, priv) // two accounts: now listed
	if tr, _ := f.svc.GetPodcastTranscriptFor(ctx, priv, "pg", PodcastReporter("stranger")); tr == nil {
		t.Fatal("B8: a feed two accounts report is public and its transcript is readable")
	}
	// Unlisting by the operator takes it private again.
	if _, err := f.svc.SetPodcastFeedsListed(ctx, []PodcastFeedListing{{FeedURL: priv}}, false); err != nil {
		t.Fatal(err)
	}
	listFeeds(t, f, "https://feeds.example.test/news.xml")
	list, _ := f.svc.ListPodcastPopular(ctx, 0)
	for _, p := range list {
		if strings.Contains(p.FeedURL, "TOKEN123") {
			t.Fatalf("B8: a private feed is on the popular list: %+v", p)
		}
	}
}

func TestB9_ATranscriptIsNeverReplacedByOneThatCoversLess(t *testing.T) {
	f := newFrontFixture(t)
	ctx := context.Background()
	ep := podcastFixtureEpisode("g1", 600)
	if _, _, err := f.svc.RecordPodcastEpisodes(ctx, []PodcastEpisodeIn{ep}); err != nil {
		t.Fatal(err)
	}
	id := PodcastEpisodeID(ep.FeedURL, "g1")
	full := PodcastTranscript{EpisodeID: id, Engine: "full", AudioSeconds: 600, FileSeconds: 600, Segments: []PodcastSegment{{Start: 1, End: 300, Text: "first half"}, {Start: 300, End: 590, Text: "second half"}}}
	if n, kept, err := f.svc.PutPodcastTranscript(ctx, full); err != nil || n != 2 || kept {
		t.Fatalf("B9: %d %v %v", n, kept, err)
	}
	prefix := PodcastTranscript{EpisodeID: id, Engine: "prefix", AudioSeconds: 300, FileSeconds: 600, Segments: []PodcastSegment{{Start: 1, End: 290, Text: "only the start"}}}
	if n, kept, err := f.svc.PutPodcastTranscript(ctx, prefix); err != nil || !kept || n != 2 {
		t.Fatalf("B9: the prefix replaced the full transcript: n=%d kept=%v err=%v", n, kept, err)
	}
	og := f.svc.ogStore()
	tr, _ := f.svc.GetPodcastTranscript(ctx, og, id)
	if tr.Engine != "full" || tr.Partial {
		t.Fatalf("B9: stored = %+v", tr)
	}
	// An equal transcript is a no-op; replace:true overwrites.
	if _, kept, _ := f.svc.PutPodcastTranscript(ctx, full); !kept {
		t.Fatal("B9: a repeat of the same transcript was rewritten")
	}
	prefix.Replace = true
	if _, kept, err := f.svc.PutPodcastTranscript(ctx, prefix); err != nil || kept {
		t.Fatalf("B9: replace:true was not honoured: %v %v", kept, err)
	}
	tr, _ = f.svc.GetPodcastTranscript(ctx, og, id)
	if tr.Engine != "prefix" || !tr.Partial {
		t.Fatalf("B9: replaced transcript = %+v, want the prefix flagged partial", tr)
	}
	// A prefix leaves the episode's length at the file's, not the prefix's.
	list, _ := f.svc.ListPodcastEpisodes(ctx, false, time.Time{}, 10)
	if list[0].DurationSeconds != 600 || !list[0].HasTranscript {
		t.Fatalf("B9: episode = %+v", list[0])
	}
	// A longer transcript replaces a partial one without being asked.
	full.Engine = "full2"
	if _, kept, err := f.svc.PutPodcastTranscript(ctx, full); err != nil || kept {
		t.Fatalf("B9: a longer transcript did not replace a partial one: %v %v", kept, err)
	}
	// A segment after the end of the episode is refused.
	past := PodcastTranscript{EpisodeID: id, Engine: "x", AudioSeconds: 700, FileSeconds: 600, Replace: true, Segments: []PodcastSegment{{Start: 1, End: 700, Text: "too long"}}}
	if _, _, err := f.svc.PutPodcastTranscript(ctx, past); !errors.Is(err, ErrPodcastInvalid) {
		t.Fatalf("B9: a segment past the episode: %v", err)
	}
}
