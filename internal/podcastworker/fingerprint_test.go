package podcastworker

// Clip fingerprints in the worker. Ways it can fail, written before the code:
//   1. the matcher writes a clip with no fingerprint although the episode's
//      track is kept, or with one for an episode that has no track;
//   2. a track is saved for one episode and read back for another, or a bad
//      episode id writes outside the track directory;
//   3. the quote finder picks a repeated phrase far from where the clip was,
//      or accepts words that are not the quote;
//   4. the repair pass fingerprints a clip at the stored place instead of
//      where its quote is now (the shift is lost);
//   5. a clip whose quote is gone is kept, or one whose quote is found is deleted;
//   6. an engine failure deletes clips, or is retried at once on the next run;
//   7. two clips of one episode cost two downloads;
//   8. a dry run writes or deletes.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/podcastfp"
)

func TestTrackStoreRoundTripAndRefusesBadIDs(t *testing.T) {
	s := &TrackStore{Dir: filepath.Join(t.TempDir(), "tracks")}
	frames := podcastfp.Frames(podcastfp.FixturePCM(16000 * 5))
	file := podcastfp.FileInfo{Bytes: 1000, BitrateKbps: 64, SeekableByBitrate: true}
	if err := s.Save("ep-aaa", Result{Track: frames, File: file}); err != nil {
		t.Fatal(err)
	}
	got, err := s.Load("ep-aaa")
	if err != nil || string(got.Frames) != string(frames) || got.File != file {
		t.Fatalf("round trip: %v %+v", err, got)
	}
	if _, err := s.Load("ep-bbb"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("another episode's track: %v", err)
	}
	for _, id := range []string{"../x", "a/b", "", strings.Repeat("a", 90)} {
		if s.Save(id, Result{Track: frames}) == nil {
			t.Fatalf("id %q accepted", id)
		}
	}
	if s.Save("ep-none", Result{}) != nil {
		t.Fatal("a result with no track is an error")
	}
	if _, err := s.Load("ep-none"); err == nil {
		t.Fatal("a track appeared from nothing")
	}
	old := time.Now().Add(-48 * time.Hour)
	os.Chtimes(filepath.Join(s.Dir, "ep-aaa.fp"), old, old)
	if n := s.Prune(24 * time.Hour); n != 1 {
		t.Fatalf("pruned %d files, want 1 (the old .fp)", n)
	}
}

func TestMatcherFingerprintsAClipFromTheKeptTrack(t *testing.T) {
	b := newFakeMatch()
	s := &TrackStore{Dir: t.TempDir()}
	// ep-1 has a track of 400 s; the pad episodes have none.
	frames := podcastfp.Frames(podcastfp.FixturePCM(16000 * 400))
	if err := s.Save("ep-1", Result{Track: frames, File: podcastfp.FileInfo{Bytes: 9 << 20, BitrateKbps: 128, SeekableByBitrate: true, Seconds: 400}}); err != nil {
		t.Fatal(err)
	}
	var logs []string
	m := newMatcher(b, &logs)
	m.Tracks = s
	if _, err := m.Run(context.Background()); err != nil || len(b.puts) != 1 {
		t.Fatalf("run: %v puts %v logs %v", err, b.puts, logs)
	}
	c := b.puts[0][0]
	fp := c.Fingerprint
	if fp == nil || fp.Validate() != nil || fp.Start != c.Start || fp.End != c.End || !fp.SeekableByBitrate || fp.BitrateKbps != 128 {
		t.Fatalf("clip %+v fingerprint %+v", c, fp)
	}
	// No track: the clip is still written, with no fingerprint.
	b2 := newFakeMatch()
	m2 := newMatcher(b2, &logs)
	m2.Tracks = &TrackStore{Dir: t.TempDir()}
	if _, err := m2.Run(context.Background()); err != nil || len(b2.puts) != 1 || b2.puts[0][0].Fingerprint != nil {
		t.Fatalf("no track: %v %+v", err, b2.puts)
	}
}

func spokenWords(text string, from, per float64) []Word {
	var out []Word
	for i, f := range strings.Fields(text) {
		out = append(out, Word{W: f, Start: from + float64(i)*per, End: from + float64(i)*per + per*0.8})
	}
	return out
}

func TestFindQuote(t *testing.T) {
	quote := "The harbor reopened this morning after weeks of repairs."
	noise := "Today we discuss gardens and weather and the long history of local fairs. "
	ws := append(spokenWords(noise, 0, 0.4), spokenWords(quote+" Crews cheered.", 50, 0.4)...)
	ws = append(ws, spokenWords(noise, 90, 0.4)...)
	at, sc := FindQuote(ws, quote, 48)
	if sc < 0.99 || at != 50 {
		t.Fatalf("exact: at %v score %v", at, sc)
	}
	// A repeated phrase: the one nearest where the clip was wins.
	ws = append(ws, spokenWords(quote, 200, 0.4)...)
	if at, _ := FindQuote(ws, quote, 195); at != 200 {
		t.Fatalf("repeat: at %v, want 200", at)
	}
	if at, _ := FindQuote(ws, quote, 55); at != 50 {
		t.Fatalf("repeat: at %v, want 50", at)
	}
	// One word misheard, the first one: still found at the right place.
	mis := strings.Replace(quote, "The", "A", 1)
	if at, sc := FindQuote(spokenWords(noise+mis, 0, 0.4), quote, 5); sc < QuoteFoundScore || at < 5 {
		t.Fatalf("misheard first word: at %v score %v", at, sc)
	}
	// Other words: not found.
	if _, sc := FindQuote(spokenWords(noise+noise, 0, 0.4), quote, 5); sc >= QuoteFoundScore {
		t.Fatalf("absent quote scored %v", sc)
	}
	if _, sc := FindQuote(nil, quote, 0); sc != 0 {
		t.Fatalf("no words scored %v", sc)
	}
}

// fakeRepair is both the backend and the engine.
type fakeRepair struct {
	mu        sync.Mutex
	front     []FrontStory
	eps       []EpisodeInfo
	segs      map[string][]Segment
	held      []HeldClip
	puts      []ClipIn
	deleted   []string
	windows   int
	spansSeen [][]Span
	engineErr error
	audio     func(sp Span) SpanAudio
}

func (f *fakeRepair) Front(context.Context, int) ([]FrontStory, error) { return f.front, nil }
func (f *fakeRepair) Episodes(context.Context, time.Time, int) ([]EpisodeInfo, error) {
	return f.eps, nil
}
func (f *fakeRepair) Transcript(_ context.Context, _, guid string) ([]Segment, error) {
	return f.segs[guid], nil
}
func (f *fakeRepair) Clips(context.Context, []string) ([]HeldClip, error) { return f.held, nil }
func (f *fakeRepair) PutClipsEach(_ context.Context, c []ClipIn) []error {
	f.puts = append(f.puts, c...)
	return make([]error, len(c))
}
func (f *fakeRepair) DeleteClips(_ context.Context, ids []string) error {
	f.deleted = append(f.deleted, ids...)
	return nil
}
func (f *fakeRepair) Windows(_ context.Context, _ string, spans []Span) (podcastfp.FileInfo, []SpanAudio, error) {
	f.windows++
	f.spansSeen = append(f.spansSeen, spans)
	if f.engineErr != nil {
		return podcastfp.FileInfo{}, nil, f.engineErr
	}
	var out []SpanAudio
	for _, sp := range spans {
		out = append(out, f.audio(sp))
	}
	return podcastfp.FileInfo{Bytes: 5 << 20, BitrateKbps: 128, SeekableByBitrate: true, Seconds: 1500}, out, nil
}

const q1 = "The harbor reopened this morning after weeks of repairs."
const q2 = "Crews pulled the ferry clear and divers checked the damaged pier."

func newFakeRepair() *fakeRepair {
	f := &fakeRepair{segs: map[string][]Segment{}}
	f.front = []FrontStory{frontStory("/pub/texture/ferry", "t", "d", 1, matchNow.Add(-time.Hour))}
	f.eps = []EpisodeInfo{{EpisodeID: "ep-1", FeedURL: "f", GUID: "g1", EnclosureURL: "https://cdn.example.test/a.mp3", Show: "S"}}
	// Stored transcript: the quotes are said at 400 s and 1300 s (segment starts).
	f.segs["g1"] = []Segment{{Start: 0, End: 5, Text: "Hello."}, {Start: 400, End: 410, Text: q1 + " More words here."}, {Start: 1300, End: 1310, Text: q2}}
	f.held = []HeldClip{
		{ClipID: "clip-a", Route: "/pub/texture/ferry", FeedURL: "f", GUID: "g1", Start: 395, End: 435, Quote: q1, Support: 0.8},
		{ClipID: "clip-b", Route: "/pub/texture/ferry", FeedURL: "f", GUID: "g1", Start: 1295, End: 1330, Quote: q2, Support: 0.7},
	}
	// The served file now: 17 s of ad before the first quote, and the second
	// quote is gone (edited out).
	pcm := podcastfp.FixturePCM(16000 * 700)
	f.audio = func(sp Span) SpanAudio {
		var ws []Word
		filler := "Today we discuss gardens and weather and the long history of local fairs."
		ws = append(ws, spokenWords(filler, sp.T0, 0.5)...)
		if sp.T0 <= 417 && sp.T1 >= 440 {
			ws = append(ws, spokenWords(q1, 417, 0.4)...)
		}
		n := int((sp.T1 - sp.T0) * 16000)
		return SpanAudio{T0: sp.T0, Words: ws, Track: podcastfp.Frames(pcm[:min(n, len(pcm))])}
	}
	return f
}

func newRepairer(f *fakeRepair, logs *[]string) *Repairer {
	return &Repairer{B: f, Eng: f, Now: func() time.Time { return matchNow }, Logf: func(s string, a ...any) { *logs = append(*logs, fmt.Sprintf(s, a...)) }}
}

func TestRepairFingerprintsWhereTheQuoteIsNowAndDeletesTheEditedClip(t *testing.T) {
	f := newFakeRepair()
	var logs []string
	rep, err := newRepairer(f, &logs).Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if f.windows != 1 {
		t.Fatalf("%d downloads for one episode, want 1: %v", f.windows, f.spansSeen)
	}
	// Clips at 395-435 and 1295-1330 with a 300 s margin make two far-apart spans.
	if len(f.spansSeen[0]) != 2 || f.spansSeen[0][0].T0 != 95 || f.spansSeen[0][0].T1 != 735 {
		t.Fatalf("spans %v", f.spansSeen[0])
	}
	if rep.Fingerprinted != 1 || rep.Deleted != 1 {
		t.Fatalf("report %+v logs %v", rep, logs)
	}
	if len(f.deleted) != 1 || f.deleted[0] != "clip-b" {
		t.Fatalf("deleted %v", f.deleted)
	}
	if len(f.puts) != 1 {
		t.Fatalf("puts %+v", f.puts)
	}
	p := f.puts[0]
	fp := p.Fingerprint
	// The clip's own stored seconds are unchanged; the fingerprint says where
	// the clip is in the file as it is served now: 17 s later.
	if p.Start != 395 || p.End != 435 || fp == nil || fp.Start != 412 || fp.End != 452 || fp.HeadAt != 412 || fp.Validate() != nil {
		t.Fatalf("put %+v fingerprint %+v", p, fp)
	}
	if !strings.Contains(strings.Join(logs, "\n"), "clip-b deleted: its quote") {
		t.Fatalf("the deletion's reason is not logged: %v", logs)
	}
}

func TestRepairLeavesClipsAloneOnAnEngineFailureAndBacksOff(t *testing.T) {
	f := newFakeRepair()
	f.engineErr = errors.New("download: status 503")
	var logs []string
	r := newRepairer(f, &logs)
	rep, err := r.Run(context.Background())
	if err != nil || rep.Failed != 2 || len(f.deleted) != 0 || len(f.puts) != 0 {
		t.Fatalf("%+v %v deleted %v puts %v", rep, err, f.deleted, f.puts)
	}
	rep, _ = r.Run(context.Background())
	if f.windows != 1 || rep.Skipped != 2 {
		t.Fatalf("retried at once: %d downloads, %+v", f.windows, rep)
	}
}

func TestRepairIgnoresFingerprintedClipsAndDryRunWritesNothing(t *testing.T) {
	f := newFakeRepair()
	f.held[0].Fingerprint = &podcastfp.ClipRecord{Version: 1}
	var logs []string
	r := newRepairer(f, &logs)
	r.DryRun = true
	rep, err := r.Run(context.Background())
	if err != nil || len(f.puts) != 0 || len(f.deleted) != 0 || rep.Unfingerprinted != 1 || rep.Deleted != 1 {
		t.Fatalf("%+v %v puts %v deleted %v", rep, err, f.puts, f.deleted)
	}
}

func TestRepairMergesNearbyClipsIntoOneSpan(t *testing.T) {
	f := newFakeRepair()
	f.held[1].Start, f.held[1].End = 500, 540
	f.segs["g1"][2] = Segment{Start: 505, End: 515, Text: q2}
	var logs []string
	if _, err := newRepairer(f, &logs).Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(f.spansSeen[0]) != 1 {
		t.Fatalf("spans %v, want one", f.spansSeen[0])
	}
}
