package platform

// Subscriptions reported by phones (docs/design/podcast-clips-2026-11-15.md
// sections 3.2 and 3.4) after review round 2 of 2026-11-16: an account can
// only cause what its own subscriptions are, a feed is public only when the
// operator features it, and nothing public is read from a page of the newest
// rows. Ways it can fail, written first:
//   B1. two spellings of one feed are two feeds, a host that names a machine is
//       admitted (an IP literal, localhost, a trailing-dot localhost, .internal,
//       a single label, odd ports), or an episode dated days ahead is taken;
//   B2. the popular list shows a hash, a feed no worker has fetched, a feed
//       the operator has not featured (two accounts, three, a hundred), or a
//       reporter's title in place of the one the worker parsed;
//   B3. a signed-out report is taken, a bad item leaves the good ones before it
//       written, one account holds more than 500 subscriptions, or the
//       platform holds more feed rows than its limit;
//   B4. now_playing creates a feed row, counts for a feed the account does not
//       subscribe to, or the queue is not now-playing, featured, then the most
//       followed;
//   B5. a transcript is not found in any spelling, or a feed that is not
//       featured shows its transcript to anyone but a subscriber;
//   B6. the clips feed filter drops clips instead of ordering them;
//   B7. a report that changes nothing writes anything, or flushes the cache;
//   B8. a feed reported by any number of accounts becomes public, or clips can
//       be written for it, or a match reads it;
//   B9. a shorter or partial transcript replaces a full one, or a segment past
//       the end of the episode is stored;
//   B10. the public read changes when strangers add feed rows (it must read by
//       key, not from the newest N).

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
		"http://localhost/f.xml", "http://localhost./f.xml", "http://printer.local/f.xml", "http://db.internal/f.xml", "http://db.internal./f.xml", "http://intranet/f.xml",
		"https://feeds.example.test:8443/f.xml", "https://feeds.example.test:22/f.xml", "http://x.localhost/f.xml", "http://2130706433/f.xml",
	} {
		if _, err := NormalizeFeedURL(bad); !errors.Is(err, ErrPodcastInvalid) {
			t.Errorf("NormalizeFeedURL(%q) accepted: %v", truncatePodcast(bad, 40), err)
		}
	}
	f := newFrontFixture(t)
	e := podcastFixtureEpisode("g", 600)
	e.EnclosureURL = "http://192.168.1.10/a.mp3"
	if _, _, err := f.svc.RecordPodcastEpisodes(context.Background(), []PodcastEpisodeIn{e}); !errors.Is(err, ErrPodcastInvalid) {
		t.Fatalf("B1: a private enclosure was taken: %v", err)
	}
	if PodcastEpisodeID("https://feeds.example.test/news.xml", "g") != PodcastEpisodeID("HTTPS://FEEDS.example.test:443/news.xml/", "g") {
		t.Fatal("B1: two spellings of one feed gave two episode ids")
	}
}

func report(t *testing.T, f frontFixture, reporter string, urls ...string) *PodcastFeedsResponse {
	t.Helper()
	var feeds []PodcastFeedReport
	for _, u := range urls {
		feeds = append(feeds, PodcastFeedReport{FeedURL: u, Title: "Reported " + u[len(u)-5:]})
	}
	resp, err := f.svc.ReportPodcastFeeds(context.Background(), reporter, PodcastFeedsRequest{Feeds: feeds})
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestB2_PopularShowsOnlyFeaturedFetchedFeedsWithTheWorkersTitle(t *testing.T) {
	f := newFrontFixture(t)
	ctx := context.Background()
	seed, loved, quiet := "https://seed.example.test/s.xml", "https://loved.example.test/l.xml", "https://quiet.example.test/q.xml"
	hundred, unfetched := "https://hundred.example.test/h.xml", "https://unfetched.example.test/u.xml"
	listFeeds(t, f, seed, loved, quiet, unfetched)
	for _, acct := range []string{"a", "b", "c"} {
		report(t, f, PodcastReporter(acct), loved, hundred)
	}
	for i := 0; i < 100; i++ {
		report(t, f, PodcastReporter(fmt.Sprint("sybil", i)), hundred) // not featured: never public
	}
	mk := func(feed, show, guid string) PodcastEpisodeIn {
		e := podcastFixtureEpisode(guid, 600)
		e.FeedURL, e.Show = feed, show
		return e
	}
	if _, _, err := f.svc.RecordPodcastEpisodes(ctx, []PodcastEpisodeIn{
		mk(seed, "Seed Show", "s1"), mk(loved, "Loved Show", "l1"), mk(quiet, "Quiet Show", "q1"), mk(hundred, "Hundred Show", "h1"),
	}); err != nil {
		t.Fatal(err)
	}
	list, err := f.svc.ListPodcastPopular(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]PodcastPopularFeed{}
	for _, p := range list {
		got[p.FeedURL] = p
	}
	if len(list) != 3 || list[0].FeedURL != loved || list[0].Subscribers != 3 {
		t.Fatalf("B2: popular = %+v, want the loved feed first with 3, then the seed and quiet feeds", list)
	}
	if got[seed].Subscribers != 0 || got[quiet].Subscribers != 0 {
		t.Fatalf("B2: a feed under three accounts shows a count: %+v", list)
	}
	if _, ok := got[hundred]; ok {
		t.Fatalf("B2: a feed a hundred accounts report but the operator did not feature is public")
	}
	if _, ok := got[unfetched]; ok {
		t.Fatalf("B2: a featured feed no worker has fetched is listed")
	}
	// The title is what the worker parsed, not what a reporter called it.
	if got[loved].Title != "Loved Show" || got[seed].Title != "Seed Show" {
		t.Fatalf("B2: titles = %q and %q", got[loved].Title, got[seed].Title)
	}
	raw, _ := json.Marshal(list)
	for _, banned := range []string{"sub-", "acct", "subscribers\":[", "hash", "sybil", "Reported"} {
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
	if _, err := f.svc.ReportPodcastFeeds(ctx, acct, PodcastFeedsRequest{Feeds: []PodcastFeedReport{{FeedURL: "https://b0-0.example.test/f.xml"}}}); err != nil {
		t.Fatalf("B3: a repeat at the cap: %v", err)
	}
	if _, err := f.svc.ReportPodcastFeeds(ctx, PodcastReporter("acct-y"), PodcastFeedsRequest{Feeds: []PodcastFeedReport{good}}); err != nil {
		t.Fatalf("B3: another account: %v", err)
	}
}

func TestB3_ThePlatformHoldsAtMostItsLimitOfFeedRows(t *testing.T) {
	f := newFrontFixture(t)
	ctx := context.Background()
	old := podcastMaxFeedRows
	podcastMaxFeedRows = 3
	defer func() { podcastMaxFeedRows = old }()
	report(t, f, PodcastReporter("a"), "https://one.example.test/f.xml", "https://two.example.test/f.xml")
	listFeeds(t, f, "https://seed.example.test/f.xml") // the third row
	if _, err := f.svc.ReportPodcastFeeds(ctx, PodcastReporter("b"), PodcastFeedsRequest{Feeds: []PodcastFeedReport{{FeedURL: "https://four.example.test/f.xml"}}}); !errors.Is(err, errPodcastRowLimit) {
		t.Fatalf("B3: a fourth row: %v", err)
	}
	// An existing feed is still joined at the limit.
	if _, err := f.svc.ReportPodcastFeeds(ctx, PodcastReporter("b"), PodcastFeedsRequest{Feeds: []PodcastFeedReport{{FeedURL: "https://one.example.test/f.xml"}}}); err != nil {
		t.Fatalf("B3: joining an existing feed at the limit: %v", err)
	}
	if feeds, _ := f.svc.ListPodcastFeedsForWorker(ctx); len(feeds) != 3 {
		t.Fatalf("B3: %d rows, want 3", len(feeds))
	}
}

func TestB4_NowPlayingNeverCreatesAFeedAndTheQueueIsPlayingFeaturedThenFollowed(t *testing.T) {
	f := newFrontFixture(t)
	ctx := context.Background()
	mk := func(feed, guid string, ago time.Duration) PodcastEpisodeIn {
		e := podcastFixtureEpisode(guid, 600)
		e.FeedURL = feed
		e.Published = time.Now().UTC().Add(-ago).Format(time.RFC3339)
		return e
	}
	seed, popular, niche, stranger := "https://seed.example.test/f.xml", "https://popular.example.test/f.xml", "https://niche.example.test/f.xml", "https://stranger.example.test/f.xml"
	listFeeds(t, f, seed)
	for _, acct := range []string{"a", "b", "c"} {
		report(t, f, PodcastReporter(acct), popular)
	}
	report(t, f, PodcastReporter("a"), niche)
	_, _, err := f.svc.RecordPodcastEpisodes(ctx, []PodcastEpisodeIn{
		mk(niche, "niche-new", time.Hour), mk(popular, "pop-old", 5*time.Hour), mk(popular, "pop-new", 2*time.Hour), mk(seed, "seed-old", 9*time.Hour), mk(stranger, "stranger-new", 30*time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}
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
	// Featured first, then the most followed, then the feed nobody follows.
	if got, want := order(), "seed-old,pop-new,pop-old,niche-new,stranger-new"; got != want {
		t.Fatalf("B4: queue %s, want %s", got, want)
	}
	// Playing an episode of a feed the account does not subscribe to is ignored.
	resp, err := f.svc.ReportPodcastFeeds(ctx, PodcastReporter("d"), PodcastFeedsRequest{NowPlaying: &PodcastNowPlaying{FeedURL: niche, EpisodeGUID: "niche-new", AudioURL: "https://cdn.example.test/n.mp3"}})
	if err != nil || resp.NowPlayingQueued || resp.NewFeeds != 0 {
		t.Fatalf("B4: an unsubscribed now_playing: %+v %v", resp, err)
	}
	// ... and one for an unknown feed makes no row at all.
	before, _ := f.svc.ListPodcastFeedsForWorker(ctx)
	resp, err = f.svc.ReportPodcastFeeds(ctx, PodcastReporter("d"), PodcastFeedsRequest{NowPlaying: &PodcastNowPlaying{FeedURL: "https://never.example.test/f.xml", EpisodeGUID: "g", AudioURL: "https://cdn.example.test/g.mp3"}})
	after, _ := f.svc.ListPodcastFeedsForWorker(ctx)
	if err != nil || resp.NowPlayingQueued || len(after) != len(before) {
		t.Fatalf("B4: now_playing made a feed row: %+v %v (%d to %d)", resp, err, len(before), len(after))
	}
	if got, want := order(), "seed-old,pop-new,pop-old,niche-new,stranger-new"; got != want {
		t.Fatalf("B4: queue moved after ignored now_playing: %s", got)
	}
	// A subscriber's now_playing goes first, in the same request as the subscription or after.
	resp, err = f.svc.ReportPodcastFeeds(ctx, PodcastReporter("a"), PodcastFeedsRequest{NowPlaying: &PodcastNowPlaying{FeedURL: niche + "/", EpisodeGUID: "niche-new", AudioURL: "https://cdn.example.test/n.mp3"}})
	if err != nil || !resp.NowPlayingQueued {
		t.Fatalf("B4: a subscriber's now_playing: %+v %v", resp, err)
	}
	if got, want := order(), "niche-new,seed-old,pop-new,pop-old,stranger-new"; got != want {
		t.Fatalf("B4: queue %s, want %s", got, want)
	}
	resp, err = f.svc.ReportPodcastFeeds(ctx, PodcastReporter("e"), PodcastFeedsRequest{Feeds: []PodcastFeedReport{{FeedURL: stranger}}, NowPlaying: &PodcastNowPlaying{FeedURL: stranger, EpisodeGUID: "stranger-new", AudioURL: "https://cdn.example.test/s.mp3"}})
	if err != nil || !resp.NowPlayingQueued {
		t.Fatalf("B4: subscribing and playing in one request: %+v %v", resp, err)
	}
}

func TestB5_TranscriptByFeedAndGuidAndWhoMaySeeIt(t *testing.T) {
	f := newFrontFixture(t)
	ctx := context.Background()
	ep := podcastFixtureEpisode("g1", 600)
	listFeeds(t, f, ep.FeedURL)
	if _, _, err := f.svc.RecordPodcastEpisodes(ctx, []PodcastEpisodeIn{ep}); err != nil {
		t.Fatal(err)
	}
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
	listFeeds(t, f, a.FeedURL, b.FeedURL)
	if _, _, err := f.svc.RecordPodcastEpisodes(ctx, []PodcastEpisodeIn{a, b}); err != nil {
		t.Fatal(err)
	}
	idA, idB := PodcastEpisodeID(a.FeedURL, "ga"), PodcastEpisodeID(b.FeedURL, "gb")
	seedTranscript(t, f, idA, 900, "from a", nil)
	seedTranscript(t, f, idB, 900, "from b", nil)
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

func TestB7_AReportThatChangesNothingWritesNothingAndKeepsTheCache(t *testing.T) {
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
	acctCanonical, _ := podcastCanonical(ogKindPodcastIndex, "acct:"+acct)
	old := time.Now().UTC().Add(-48 * time.Hour).Truncate(time.Second)
	for _, id := range []string{canonical, acctCanonical} {
		if _, err := f.store.corpus().Exec(`UPDATE og_objects SET updated_at = ? WHERE canonical_id = ?`, old, id); err != nil {
			t.Fatal(err)
		}
	}
	f.svc.podcastCache.put("warm", nil)
	if _, err := f.svc.ReportPodcastFeeds(ctx, acct, req); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{canonical, acctCanonical} {
		obj, err := f.svc.ogStore().GetObject(ctx, id)
		if err != nil || !obj.UpdatedAt.UTC().Equal(old) {
			t.Fatalf("B7: an unchanged report rewrote %s: %v %v", id, obj.UpdatedAt, err)
		}
	}
	if _, warm := f.svc.podcastCache.get("warm"); !warm {
		t.Fatal("B7: an unchanged report flushed the public read's cache")
	}
	if _, err := f.svc.ReportPodcastFeeds(ctx, PodcastReporter("acct-2"), req); err != nil {
		t.Fatal(err)
	}
	obj, _ := f.svc.ogStore().GetObject(ctx, canonical)
	if obj.UpdatedAt.UTC().Equal(old) {
		t.Fatal("B7: a new subscriber was not written")
	}
}

func TestB8_NoNumberOfAccountsMakesAFeedPublic(t *testing.T) {
	f := newFrontFixture(t)
	ctx := context.Background()
	priv := "https://feeds.example.test/private/TOKEN123/f.xml"
	for i := 0; i < 25; i++ {
		report(t, f, PodcastReporter(fmt.Sprint("acct", i)), priv)
	}
	e := podcastFixtureEpisode("pg", 900)
	e.FeedURL = priv
	if _, _, err := f.svc.RecordPodcastEpisodes(ctx, []PodcastEpisodeIn{e}); err != nil {
		t.Fatal(err)
	}
	id := PodcastEpisodeID(priv, "pg")
	seedTranscript(t, f, id, 900, "a private episode", nil)
	// Clips cannot be written for it.
	if _, err := f.svc.PutPodcastClips(ctx, []PodcastClipIn{{StoryRoute: f.routes["doc-harbor"], EpisodeID: id, Start: 10, End: 50, Quote: "a private episode", Support: 0.9}}); !errors.Is(err, ErrPodcastInvalid) {
		t.Fatalf("B8: a clip of an unfeatured feed was written: %v", err)
	}
	// Its transcript is the subscribers': 404 (nil) for everyone else, signed in or not.
	if tr, err := f.svc.GetPodcastTranscriptFor(ctx, priv, "pg", PodcastReporter("acct3")); err != nil || tr == nil {
		t.Fatalf("B8: a subscriber cannot read it: %v %v", tr, err)
	}
	for who, rep := range map[string]string{"another account": PodcastReporter("stranger"), "nobody": ""} {
		if tr, err := f.svc.GetPodcastTranscriptFor(ctx, priv, "pg", rep); err != nil || tr != nil {
			t.Fatalf("B8: %s read a private transcript: %v %v", who, tr, err)
		}
	}
	list, _ := f.svc.ListPodcastPopular(ctx, 0)
	for _, p := range list {
		if strings.Contains(p.FeedURL, "TOKEN123") {
			t.Fatalf("B8: a private feed is on the popular list: %+v", p)
		}
	}
	// The operator features it: now it is public; unfeaturing takes it back.
	listFeeds(t, f, priv)
	if tr, _ := f.svc.GetPodcastTranscriptFor(ctx, priv, "pg", PodcastReporter("stranger")); tr == nil {
		t.Fatal("B8: a featured feed's transcript is readable by any signed-in account")
	}
	if _, err := f.svc.SetPodcastFeedsListed(ctx, []PodcastFeedListing{{FeedURL: priv}}, false); err != nil {
		t.Fatal(err)
	}
	if tr, _ := f.svc.GetPodcastTranscriptFor(ctx, priv, "pg", PodcastReporter("stranger")); tr != nil {
		t.Fatal("B8: unfeaturing did not take the feed back")
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
	list, _ := f.svc.ListPodcastEpisodes(ctx, false, time.Time{}, 10)
	if list[0].DurationSeconds != 600 || !list[0].HasTranscript {
		t.Fatalf("B9: episode = %+v", list[0])
	}
	full.Engine = "full2"
	if _, kept, err := f.svc.PutPodcastTranscript(ctx, full); err != nil || kept {
		t.Fatalf("B9: a longer transcript did not replace a partial one: %v %v", kept, err)
	}
	past := PodcastTranscript{EpisodeID: id, Engine: "x", AudioSeconds: 700, FileSeconds: 600, Replace: true, Segments: []PodcastSegment{{Start: 1, End: 700, Text: "too long"}}}
	if _, _, err := f.svc.PutPodcastTranscript(ctx, past); !errors.Is(err, ErrPodcastInvalid) {
		t.Fatalf("B9: a segment past the episode: %v", err)
	}
}

// B10. The public clips read does not depend on a page of the newest rows: a
// seed feed featured long ago, and clips matched long ago, survive any number of
// feed rows and episodes written after them.
func TestB10_ThePublicReadSurvivesStrangersWritingAfterIt(t *testing.T) {
	f := newFrontFixture(t)
	ctx := context.Background()
	ep := podcastFixtureEpisode("g1", 900)
	listFeeds(t, f, ep.FeedURL)
	if _, _, err := f.svc.RecordPodcastEpisodes(ctx, []PodcastEpisodeIn{ep}); err != nil {
		t.Fatal(err)
	}
	id := PodcastEpisodeID(ep.FeedURL, "g1")
	seedTranscript(t, f, id, 900, "the harbor reopened", nil)
	harbor := f.routes["doc-harbor"]
	if _, err := f.svc.PutPodcastClips(ctx, []PodcastClipIn{{StoryRoute: harbor, EpisodeID: id, Start: 100, End: 140, Quote: "the harbor reopened", Support: 0.9}}); err != nil {
		t.Fatal(err)
	}
	// Age every row of the seed, then let accounts write many newer feed rows.
	if _, err := f.store.corpus().Exec(`UPDATE og_objects SET updated_at = ? WHERE object_kind LIKE 'choir.podcast_%'`, time.Now().UTC().Add(-72*time.Hour)); err != nil {
		t.Fatal(err)
	}
	old := podcastMaxFeedRows
	podcastMaxFeedRows = 300
	defer func() { podcastMaxFeedRows = old }()
	for batch := 0; batch < 4; batch++ {
		var feeds []PodcastFeedReport
		for i := 0; i < podcastMaxFeedsPerReport; i++ {
			feeds = append(feeds, PodcastFeedReport{FeedURL: fmt.Sprintf("https://n%d-%d.example.test/f.xml", batch, i)})
		}
		if _, err := f.svc.ReportPodcastFeeds(ctx, PodcastReporter(fmt.Sprint("w", batch)), PodcastFeedsRequest{Feeds: feeds}); err != nil {
			t.Fatal(err)
		}
	}
	f.svc.podcastCache.reset()
	got, err := f.svc.ListPodcastClips(ctx, []string{harbor}, nil, 0)
	if err != nil || len(got.Clips) != 1 {
		t.Fatalf("B10: the clip is gone after strangers wrote feed rows: %v %+v", err, got)
	}
	if tr, _ := f.svc.GetPodcastTranscriptFor(ctx, ep.FeedURL, "g1", ""); tr == nil {
		t.Fatal("B10: the featured feed's transcript is gone")
	}
}
