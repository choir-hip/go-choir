package platform

// The operator's feeds and the public reads (docs/design/podcast-clips-2026-11-15.md
// 3.2 and 3.4), after the review rounds of 2026-11-16 cut feeds reported by
// accounts out (Part 2, not built). Ways it can fail, written first:
//   B1. two spellings of one feed are two feeds (a trailing dot on the host, a
//       default port, case, a trailing slash), or a host that names a machine
//       is admitted;
//   B2. the popular list shows a feed no worker has fetched, a feed the
//       operator has not featured, a count other than 0, or a reporter's title;
//   B5. a transcript is not found in any spelling, or one of a feed that is not
//       featured is shown;
//   B6. the clips feed filter drops clips instead of ordering them;
//   B9. a shorter or partial transcript replaces a full one, or a segment past
//       the end of the episode is stored;
//   B10. the public read changes when other rows are written after it (it must
//       read by key, not from the newest N);
//   B11. a failure between a story's index and its clip hides the clip forever,
//       or a dangling id breaks the read;
//   B12. a lost featured index while feed rows exist is silent (it fails closed
//       and says so, once);
//   B13. the worker's queue is not newest first.

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
	for _, in := range []string{
		"https://feeds.example.test/news.xml", "HTTPS://Feeds.Example.test:443/news.xml/#frag", " https://feeds.example.test/news.xml/ ",
		"https://feeds.example.test./news.xml", "https://FEEDS.example.test.:443/news.xml/",
	} {
		got, err := NormalizeFeedURL(in)
		if err != nil || got != want {
			t.Errorf("NormalizeFeedURL(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	if got, _ := NormalizeFeedURL("http://feeds.example.test.:80/a"); got != "http://feeds.example.test/a" {
		t.Errorf("a default port and a trailing dot: %q", got)
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
	if PodcastEpisodeID("https://feeds.example.test/news.xml", "g") != PodcastEpisodeID("HTTPS://FEEDS.example.test.:443/news.xml/", "g") {
		t.Fatal("B1: two spellings of one feed gave two episode ids")
	}
}

func TestB2_PopularShowsOnlyFeaturedFetchedFeedsNewestFirstWithTheWorkersTitle(t *testing.T) {
	f := newFrontFixture(t)
	ctx := context.Background()
	older, newer, unfetched := "https://older.example.test/o.xml", "https://newer.example.test/n.xml", "https://unfetched.example.test/u.xml"
	listFeeds(t, f, older, newer, unfetched)
	mk := func(feed, show, guid string, ago time.Duration) PodcastEpisodeIn {
		e := podcastFixtureEpisode(guid, 600)
		e.FeedURL, e.Show = feed, show
		e.Published = time.Now().UTC().Add(-ago).Format(time.RFC3339)
		return e
	}
	if _, _, _, err := f.svc.RecordPodcastEpisodes(ctx, []PodcastEpisodeIn{mk(older, "Older Show", "o1", 10*time.Hour), mk(newer, "Newer Show", "n1", time.Hour)}); err != nil {
		t.Fatal(err)
	}
	list, err := f.svc.ListPodcastPopular(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].FeedURL != newer || list[1].FeedURL != older {
		t.Fatalf("B2: popular = %+v, want the newer feed first and no unfetched feed", list)
	}
	for _, p := range list {
		if p.Subscribers != 0 {
			t.Fatalf("B2: a count other than 0: %+v", p)
		}
	}
	if list[0].Title != "Newer Show" || list[0].LatestEpisodeTitle == "" || list[0].LatestPublished.IsZero() {
		t.Fatalf("B2: entry = %+v", list[0])
	}
	// A feed the operator unfeatures leaves the list.
	if _, err := f.svc.SetPodcastFeedsListed(ctx, []PodcastFeedListing{{FeedURL: newer}}, false); err != nil {
		t.Fatal(err)
	}
	list, _ = f.svc.ListPodcastPopular(ctx, 0)
	if len(list) != 1 || list[0].FeedURL != older {
		t.Fatalf("B2: after unfeaturing: %+v", list)
	}
	raw, _ := json.Marshal(list)
	for _, banned := range []string{"sub-", "hash", "Listed"} {
		if strings.Contains(string(raw), banned) {
			t.Fatalf("B2: the popular list leaks %q: %s", banned, raw)
		}
	}
}

func TestB5_TranscriptByFeedAndGuidOnlyForAFeaturedFeed(t *testing.T) {
	f := newFrontFixture(t)
	ctx := context.Background()
	ep := podcastFixtureEpisode("g1", 600)
	if _, _, _, err := recordFeatured(f, ctx, []PodcastEpisodeIn{ep}); err != nil {
		t.Fatal(err)
	}
	if tr, err := f.svc.GetPodcastTranscriptFor(ctx, ep.FeedURL, "g1"); err != nil || tr != nil {
		t.Fatalf("B5: an episode with no transcript answered %+v %v", tr, err)
	}
	id := PodcastEpisodeID(ep.FeedURL, "g1")
	if _, _, err := f.svc.PutPodcastTranscript(ctx, PodcastTranscript{EpisodeID: id, Engine: "parakeet.cpp test", AudioSeconds: 600, FileSeconds: 612.5,
		Segments: []PodcastSegment{{Start: 1, End: 5, Text: "first block"}, {Start: 5, End: 9, Text: "second block"}}}); err != nil {
		t.Fatal(err)
	}
	for _, spelling := range []string{ep.FeedURL, "HTTPS://FEEDS.EXAMPLE.TEST./news.xml/"} {
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
	// Unfeatured: not found, as if there were none.
	if _, err := f.svc.SetPodcastFeedsListed(ctx, []PodcastFeedListing{{FeedURL: ep.FeedURL}}, false); err != nil {
		t.Fatal(err)
	}
	if tr, err := f.svc.GetPodcastTranscriptFor(ctx, ep.FeedURL, "g1"); err != nil || tr != nil {
		t.Fatalf("B5: an unfeatured feed's transcript was shown: %+v %v", tr, err)
	}
}

func TestB6_TheClipsFeedFilterOrdersAndNeverDrops(t *testing.T) {
	f := newFrontFixture(t)
	ctx := context.Background()
	a, b := podcastFixtureEpisode("ga", 900), podcastFixtureEpisode("gb", 900)
	b.FeedURL = "https://other.example.test/b.xml"
	b.Show = "Other Show"
	if _, _, _, err := recordFeatured(f, ctx, []PodcastEpisodeIn{a, b}); err != nil {
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
	mine, err := f.svc.ListPodcastClips(ctx, nil, []string{"HTTPS://other.example.test./b.xml/"}, 0)
	if err != nil || len(mine.Clips) != len(plain.Clips) || len(plain.Clips) != 2 {
		t.Fatalf("B6: %v plain %d filtered %d", err, len(plain.Clips), len(mine.Clips))
	}
	if mine.Clips[0].Show != "Other Show" {
		t.Fatalf("B6: the reader's own show is not first: %+v", mine.Clips)
	}
}

func TestB9_ATranscriptIsNeverReplacedByOneThatCoversLess(t *testing.T) {
	f := newFrontFixture(t)
	ctx := context.Background()
	ep := podcastFixtureEpisode("g1", 600)
	if _, _, _, err := recordFeatured(f, ctx, []PodcastEpisodeIn{ep}); err != nil {
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

func TestB10_ThePublicReadSurvivesRowsWrittenAfterIt(t *testing.T) {
	f := newFrontFixture(t)
	ctx := context.Background()
	ep := podcastFixtureEpisode("g1", 900)
	if _, _, _, err := recordFeatured(f, ctx, []PodcastEpisodeIn{ep}); err != nil {
		t.Fatal(err)
	}
	id := PodcastEpisodeID(ep.FeedURL, "g1")
	seedTranscript(t, f, id, 900, "the harbor reopened", nil)
	harbor := f.routes["doc-harbor"]
	if _, err := f.svc.PutPodcastClips(ctx, []PodcastClipIn{{StoryRoute: harbor, EpisodeID: id, Start: 100, End: 140, Quote: "the harbor reopened", Support: 0.9}}); err != nil {
		t.Fatal(err)
	}
	// Age every podcast row, then write many newer ones (other feeds and episodes).
	if _, err := f.store.corpus().Exec(`UPDATE og_objects SET updated_at = ? WHERE object_kind LIKE 'choir.podcast_%'`, time.Now().UTC().Add(-72*time.Hour)); err != nil {
		t.Fatal(err)
	}
	var feeds []PodcastFeedListing
	for i := 0; i < 150; i++ {
		feeds = append(feeds, PodcastFeedListing{FeedURL: fmt.Sprintf("https://n%d.example.test/f.xml", i)})
	}
	if _, err := f.svc.SetPodcastFeedsListed(ctx, feeds[:100], true); err != nil {
		t.Fatal(err)
	}
	f.svc.podcastCache.reset()
	got, err := f.svc.ListPodcastClips(ctx, []string{harbor}, nil, 0)
	if err != nil || len(got.Clips) != 1 {
		t.Fatalf("B10: the clip is gone after other rows were written: %v %+v", err, got)
	}
	if tr, _ := f.svc.GetPodcastTranscriptFor(ctx, ep.FeedURL, "g1"); tr == nil {
		t.Fatal("B10: the featured feed's transcript is gone")
	}
}

// B11. The story's index is written before its clips; a failure between them
// leaves an id with no clip, which the read skips and a repeat completes.
func TestB11_AnIndexIdWithNoClipIsSkippedAndAWriteCompletesIt(t *testing.T) {
	f := newFrontFixture(t)
	ctx := context.Background()
	ep := podcastFixtureEpisode("g1", 900)
	if _, _, _, err := recordFeatured(f, ctx, []PodcastEpisodeIn{ep}); err != nil {
		t.Fatal(err)
	}
	id := PodcastEpisodeID(ep.FeedURL, "g1")
	seedTranscript(t, f, id, 900, "the harbor reopened", nil)
	harbor := f.routes["doc-harbor"]
	in := PodcastClipIn{StoryRoute: harbor, EpisodeID: id, Start: 100, End: 140, Quote: "the harbor reopened", Support: 0.9}
	// The failure: the index names the clip, the clip was never written.
	og := f.svc.ogStore()
	if err := f.svc.putPodcastIndex(ctx, og, "clips:"+harbor, podcastIndexDoc{Clips: []string{podcastClipID(harbor, id, 100), "clip-deadbeefdeadbeefdeadbeef"}}, time.Now()); err != nil {
		t.Fatal(err)
	}
	got, err := f.svc.ListPodcastClips(ctx, []string{harbor}, nil, 0)
	if err != nil || len(got.Clips) != 0 {
		t.Fatalf("B11: a dangling id broke or fed the read: %v %+v", err, got)
	}
	// The same write again completes it: the id is already in the index.
	if n, err := f.svc.PutPodcastClips(ctx, []PodcastClipIn{in}); err != nil || n != 1 {
		t.Fatalf("B11: %d %v", n, err)
	}
	f.svc.podcastCache.reset()
	got, err = f.svc.ListPodcastClips(ctx, []string{harbor}, nil, 0)
	if err != nil || len(got.Clips) != 1 {
		t.Fatalf("B11: the completed clip is not shown: %v %+v", err, got)
	}
	// And the order matters: the index is written first, so no clip can exist that
	// its story's index does not name.
	idx, _ := f.svc.getPodcastIndex(ctx, og, "clips:"+harbor)
	if !podcastContains(idx.Clips, got.Clips[0].ClipID) {
		t.Fatalf("B11: a stored clip is not in its story's index: %+v", idx)
	}
}

func podcastContains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// B12. A featured index that is lost while feed rows exist fails closed: nothing
// is public, and it is said (once per process).
func TestB12_ALostFeaturedIndexFailsClosedAndSaysSo(t *testing.T) {
	f := newFrontFixture(t)
	ctx := context.Background()
	ep := podcastFixtureEpisode("g1", 900)
	if _, _, _, err := recordFeatured(f, ctx, []PodcastEpisodeIn{ep}); err != nil {
		t.Fatal(err)
	}
	id := PodcastEpisodeID(ep.FeedURL, "g1")
	seedTranscript(t, f, id, 900, "the harbor reopened", nil)
	if _, err := f.store.corpus().Exec(`DELETE FROM og_objects WHERE object_kind = 'choir.podcast_index' AND canonical_id = ?`, mustIndexCanonical(t, "meta")); err != nil {
		t.Fatal(err)
	}
	podcastEmptyIndexLogged = onceReset()
	var logged strings.Builder
	restore := captureStdLog(&logged)
	defer restore()
	if tr, _ := f.svc.GetPodcastTranscriptFor(ctx, ep.FeedURL, "g1"); tr != nil {
		t.Fatal("B12: a transcript was public with the featured index lost")
	}
	_, _ = f.svc.GetPodcastTranscriptFor(ctx, ep.FeedURL, "g1")
	if n := strings.Count(logged.String(), "featured index is empty or missing"); n != 1 {
		t.Fatalf("B12: logged %d times, want once: %q", n, logged.String())
	}
}

// B13. The worker's queue is the episodes with no transcript, newest first.
func TestB13_TheQueueIsNewestFirst(t *testing.T) {
	f := newFrontFixture(t)
	ctx := context.Background()
	mk := func(guid string, ago time.Duration) PodcastEpisodeIn {
		e := podcastFixtureEpisode(guid, 600)
		e.Published = time.Now().UTC().Add(-ago).Format(time.RFC3339)
		return e
	}
	if _, _, _, err := recordFeatured(f, ctx, []PodcastEpisodeIn{mk("old", 9*time.Hour), mk("new", time.Hour), mk("mid", 5*time.Hour)}); err != nil {
		t.Fatal(err)
	}
	list, err := f.svc.ListPodcastEpisodes(ctx, true, time.Time{}, 50)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, e := range list {
		got = append(got, e.GUID)
	}
	if strings.Join(got, ",") != "new,mid,old" {
		t.Fatalf("B13: queue %v", got)
	}
}
