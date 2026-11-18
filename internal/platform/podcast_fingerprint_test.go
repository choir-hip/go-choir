package platform

// Clip fingerprints (docs/reference/PODCAST-CLIP-FINGERPRINT.md). Ways it can
// fail, written before the code:
//   1. a clip with a fingerprint loses it on the way to the public read;
//   2. a malformed fingerprint (wrong version, a truncated frame list, a
//      length that disagrees with the clip's) is stored;
//   3. a clip with none is refused, or the read shows a fingerprint for it;
//   4. a clip written again with a fingerprint keeps the old row;
//   5. a deleted clip is still shown, or deleting an unknown id is an error;
//   6. a delete with a bad id removes the good ones before it.

import (
	"context"
	"errors"
	"math"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/podcastfp"
)

func fingerprintFor(t *testing.T, start, end float64) *podcastfp.ClipRecord {
	t.Helper()
	pcm := podcastfp.FixturePCM(int(80 * podcastfp.SampleRate))
	rec, err := podcastfp.Cut(podcastfp.Frames(pcm), 0, start, end, podcastfp.FileInfo{Bytes: 5 << 20, FirstFrameOffset: 1024, BitrateKbps: 128, SeekableByBitrate: true, Seconds: 60})
	if err != nil {
		t.Fatal(err)
	}
	return rec
}

func TestPodcastClipFingerprintIsStoredValidatedAndServed(t *testing.T) {
	f := newFrontFixture(t)
	ctx := context.Background()
	listFeeds(t, f, "https://feeds.example.test/news.xml")
	if _, _, _, err := recordFeatured(f, ctx, []PodcastEpisodeIn{podcastFixtureEpisode("g1", 900)}); err != nil {
		t.Fatal(err)
	}
	id := PodcastEpisodeID("https://feeds.example.test/news.xml", "g1")
	seedTranscript(t, f, id, 900, "the harbor reopened after repairs", nil)
	harbor := f.routes["doc-harbor"]
	clip := func(start, end float64) PodcastClipIn {
		return PodcastClipIn{StoryRoute: harbor, EpisodeID: id, Start: start, End: end, Quote: "the harbor reopened", Support: 0.9}
	}
	plain := clip(300, 340)
	with := clip(100, 140)
	with.Fingerprint = fingerprintFor(t, 20, 60)

	// A bad fingerprint refuses the whole request, and nothing is stored.
	badVersion := clip(500, 540)
	badVersion.Fingerprint = fingerprintFor(t, 20, 60)
	badVersion.Fingerprint.Version = 2
	badLength := clip(600, 650)
	badLength.Fingerprint = fingerprintFor(t, 20, 60) // 40 s against 50 s
	badTail := clip(700, 740)
	badTail.Fingerprint = fingerprintFor(t, 20, 60)
	badTail.Fingerprint.Tail = badTail.Fingerprint.Tail[:10]
	nan := clip(800, 840)
	nan.Fingerprint = fingerprintFor(t, 20, 60)
	nan.Fingerprint.HeadAt = math.NaN()
	for name, bad := range map[string]PodcastClipIn{"version": badVersion, "length": badLength, "tail": badTail, "nan": nan} {
		if _, err := f.svc.PutPodcastClips(ctx, []PodcastClipIn{plain, bad}); !errors.Is(err, ErrPodcastInvalid) {
			t.Fatalf("%s: err = %v, want ErrPodcastInvalid", name, err)
		}
	}
	if got, _ := f.svc.ListPodcastClips(ctx, nil, nil, 0); len(got.Clips) != 0 {
		t.Fatalf("refused requests left %d clips", len(got.Clips))
	}

	if n, err := f.svc.PutPodcastClips(ctx, []PodcastClipIn{plain, with}); err != nil || n != 2 {
		t.Fatalf("put: %d %v", n, err)
	}
	f.svc.podcastCache.reset()
	got, err := f.svc.ListPodcastClips(ctx, []string{harbor}, nil, 0)
	if err != nil || len(got.Clips) != 2 {
		t.Fatalf("read: %d %v", len(got.Clips), err)
	}
	byStart := map[float64]PodcastClip{}
	for _, c := range got.Clips {
		byStart[c.Start] = c
	}
	if byStart[300].Fingerprint != nil {
		t.Fatalf("a clip with no fingerprint shows one: %+v", byStart[300].Fingerprint)
	}
	fp := byStart[100].Fingerprint
	if fp == nil || fp.Validate() != nil || fp.HeadAt != with.Fingerprint.HeadAt || string(fp.Head) != string(with.Fingerprint.Head) || !fp.SeekableByBitrate || fp.BitrateKbps != 128 {
		t.Fatalf("fingerprint did not survive: %+v", fp)
	}

	// The same clip written again with a fingerprint replaces the old row.
	again := clip(300, 340)
	again.Fingerprint = fingerprintFor(t, 30, 70)
	if _, err := f.svc.PutPodcastClips(ctx, []PodcastClipIn{again}); err != nil {
		t.Fatal(err)
	}
	f.svc.podcastCache.reset()
	got, _ = f.svc.ListPodcastClips(ctx, []string{harbor}, nil, 0)
	for _, c := range got.Clips {
		if c.Start == 300 && (c.Fingerprint == nil || c.Fingerprint.Start != 30) {
			t.Fatalf("the rewritten clip kept its old row: %+v", c.Fingerprint)
		}
	}

	// Delete: a bad id refuses the request whole; an unknown id is fine.
	var ids []string
	for _, c := range got.Clips {
		ids = append(ids, c.ClipID)
	}
	// Warm the cache, then refuse a request whose first id is good: the good
	// one must not have been deleted.
	f.svc.ListPodcastClips(ctx, []string{harbor}, nil, 0)
	if _, err := f.svc.DeletePodcastClips(ctx, []string{ids[0], "not-a-clip"}); !errors.Is(err, ErrPodcastInvalid) {
		t.Fatalf("bad id: %v", err)
	}
	f.svc.podcastCache.reset() // read the store, not the cache the delete should have emptied
	if again, _ := f.svc.ListPodcastClips(ctx, []string{harbor}, nil, 0); len(again.Clips) != 2 {
		t.Fatalf("a refused delete removed clips: %d left", len(again.Clips))
	}
	if n, err := f.svc.DeletePodcastClips(ctx, []string{ids[0], "clip-000000000000000000000000"}); err != nil || n != 2 {
		t.Fatalf("delete: %d %v", n, err)
	}
	left, _ := f.svc.ListPodcastClips(ctx, []string{harbor}, nil, 0) // no manual reset: the delete must have emptied the cache
	if len(left.Clips) != 1 || left.Clips[0].ClipID == ids[0] {
		t.Fatalf("after the delete: %+v", left.Clips)
	}
	if _, err := f.svc.DeletePodcastClips(ctx, nil); !errors.Is(err, ErrPodcastInvalid) {
		t.Fatalf("empty delete: %v", err)
	}
}
