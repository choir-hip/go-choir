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
//   5. a clip whose quote is not found is deleted, or searched again at once;
//   6. an engine failure deletes clips, or is retried at once on the next run;
//   7. two clips of one episode cost two downloads;
//   8. a dry run writes or deletes.

import (
	"context"
	"encoding/json"
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
		t.Fatal("saving a result with no track failed; it should store nothing and succeed")
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
	s := &TrackStore{Dir: t.TempDir()}
	frames := podcastfp.Frames(podcastfp.FixturePCM(16000 * 400))
	if err := s.Save("ep-1", Result{Track: frames, File: podcastfp.FileInfo{Bytes: 9 << 20, BitrateKbps: 128, SeekableByBitrate: true, Seconds: 400}}); err != nil {
		t.Fatal(err)
	}
	var logs []string
	m := newMatcher(newFakeMatch(), &logs)
	m.Tracks = s
	fp := m.fingerprint("ep-1", 100, 140)
	if fp == nil || fp.Validate() != nil || fp.Start != 100 || fp.End != 140 || !fp.SeekableByBitrate || fp.BitrateKbps != 128 {
		t.Fatalf("fingerprint %+v logs %v", fp, logs)
	}
	if m.fingerprint("ep-2", 100, 140) != nil {
		t.Fatal("a fingerprint for an episode with no track")
	}
	// A clip under 24 s is written without one, with the reason logged.
	if m.fingerprint("ep-1", 100, 120.5) != nil || !strings.Contains(strings.Join(logs, "\n"), "shorter than 24 s") {
		t.Fatalf("a 20.5 s clip: logs %v", logs)
	}
	// Through a run: the clip is written whatever its length, fingerprint or not.
	b := newFakeMatch()
	m2 := newMatcher(b, &logs)
	m2.Tracks = s
	if _, err := m2.Run(context.Background()); err != nil || len(b.puts) != 1 {
		t.Fatalf("run: %v puts %v", err, b.puts)
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
	windows   int
	spansSeen [][]Span
	engineErr error
	wholeErr  error              // the engine fails on a whole-file span only
	said      map[float64]string // seconds -> words spoken there in the served file
	pcm       []int16
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
	for _, in := range c {
		for i := range f.held {
			if f.held[i].Route == in.StoryRoute && f.held[i].Start == in.Start {
				f.held[i].Fingerprint = in.Fingerprint
			}
		}
	}
	return make([]error, len(c))
}
func (f *fakeRepair) Windows(_ context.Context, _ string, spans []Span) (podcastfp.FileInfo, []SpanAudio, error) {
	f.windows++
	f.spansSeen = append(f.spansSeen, spans)
	if f.engineErr != nil {
		return podcastfp.FileInfo{}, nil, f.engineErr
	}
	filler := "Today we discuss gardens and weather and the long history of local fairs."
	var out []SpanAudio
	for _, sp := range spans {
		whole := sp.T0 == 0 && sp.T1 == 0
		if whole && f.wholeErr != nil {
			return podcastfp.FileInfo{}, nil, f.wholeErr
		}
		t1 := sp.T1
		if whole {
			t1 = float64(len(f.pcm)) / 16000
		}
		ws := spokenWords(filler, sp.T0, 0.5)
		for at, text := range f.said {
			if at >= sp.T0 && at+10 <= t1 {
				ws = append(ws, spokenWords(text, at, 0.4)...)
			}
		}
		n := min(int((t1-sp.T0)*16000), len(f.pcm)-int(sp.T0*16000))
		out = append(out, SpanAudio{T0: sp.T0, Words: ws, Track: podcastfp.Frames(f.pcm[int(sp.T0*16000) : int(sp.T0*16000)+n])})
	}
	return podcastfp.FileInfo{Bytes: 5 << 20, BitrateKbps: 128, SeekableByBitrate: true, Seconds: float64(len(f.pcm)) / 16000}, out, nil
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
	// The served file now is 3200 s long: 17 s of ad before the first quote;
	// the second quote is nowhere (edited out).
	f.pcm = podcastfp.FixturePCM(16000 * 3200)
	f.said = map[float64]string{417: q1}
	return f
}

func newRepairer(f *fakeRepair, logs *[]string) *Repairer {
	return &Repairer{B: f, Eng: f, Now: func() time.Time { return matchNow }, Logf: func(s string, a ...any) { *logs = append(*logs, fmt.Sprintf(s, a...)) }}
}

func TestRepairFingerprintsWhereTheQuoteIsNowAndLeavesAnUnfoundClipAlone(t *testing.T) {
	f := newFakeRepair()
	var logs []string
	r := newRepairer(f, &logs)
	rep, err := r.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// One download for the two windows, one more for the whole file (clip-b's
	// quote was not in its window).
	if f.windows != 2 || len(f.spansSeen[1]) != 1 || f.spansSeen[1][0] != (Span{}) {
		t.Fatalf("%d downloads, spans %v", f.windows, f.spansSeen)
	}
	if len(f.spansSeen[0]) != 2 || f.spansSeen[0][0].T0 != 95 || f.spansSeen[0][0].T1 != 735 {
		t.Fatalf("spans %v", f.spansSeen[0])
	}
	if rep.Fingerprinted != 1 || rep.NotFound != 1 || len(f.puts) != 1 {
		t.Fatalf("report %+v puts %d logs %v", rep, len(f.puts), logs)
	}
	p := f.puts[0]
	fp := p.Fingerprint
	// The clip's own stored seconds are unchanged; the fingerprint says where
	// the clip is in the file as it is served now: 17 s later.
	if p.Start != 395 || p.End != 435 || fp == nil || fp.Start != 412 || fp.End != 452 || fp.HeadAt != 412 || fp.Validate() != nil {
		t.Fatalf("put %+v fingerprint %+v", p, fp)
	}
	if !strings.Contains(strings.Join(logs, "\n"), "clip-b not found") {
		t.Fatalf("the reason is not logged: %v", logs)
	}
	// The clip not found is left alone for a day, then looked at again.
	before := f.windows
	if rep, _ = r.Run(context.Background()); f.windows != before || rep.Skipped != 1 {
		t.Fatalf("looked at again at once: %d downloads, %+v", f.windows, rep)
	}
	now := matchNow.Add(25 * time.Hour)
	r.Now = func() time.Time { return now }
	r.Run(context.Background())
	if f.windows == before {
		t.Fatal("never looked at again after a day")
	}
}

func TestRepairFindsAQuoteFarOutsideTheWindowInTheWholeFileAndDeletesNothing(t *testing.T) {
	f := newFakeRepair()
	f.said[3000] = q2 // 1700 s from where it was: a long block was added
	var logs []string
	rep, err := newRepairer(f, &logs).Run(context.Background())
	if err != nil || rep.Fingerprinted != 2 {
		t.Fatalf("%+v %v logs %v", rep, err, logs)
	}
	for _, p := range f.puts {
		if p.Quote == q2 && (p.Fingerprint.Start != 2995 || p.Start != 1295) {
			t.Fatalf("clip-b: %+v %+v", p, p.Fingerprint)
		}
	}
}

func TestRepairDoesNotSearchAFileLongerThanTheRecogniserBound(t *testing.T) {
	f := newFakeRepair()
	var logs []string
	r := newRepairer(f, &logs)
	r.MaxSeconds = 90 * 60 // the served file is 3200 s: within the bound
	if _, err := r.Run(context.Background()); err != nil || f.windows != 2 {
		t.Fatalf("a file within the bound: %d downloads, %v", f.windows, err)
	}
	f = newFakeRepair()
	logs = nil
	r = newRepairer(f, &logs)
	r.MaxSeconds = 1000
	rep, err := r.Run(context.Background())
	if err != nil || f.windows != 1 || rep.NotFound != 1 || rep.Fingerprinted != 1 {
		t.Fatalf("a file over the bound: %d downloads %+v %v logs %v", f.windows, rep, err, logs)
	}
	if !strings.Contains(strings.Join(logs, "\n"), "not searched whole") {
		t.Fatalf("the skip is not logged: %v", logs)
	}
}

func TestRepairWholeFileFailureLeavesTheClipAndBacksOff(t *testing.T) {
	f := newFakeRepair()
	f.wholeErr = errors.New("download: status 503")
	var logs []string
	r := newRepairer(f, &logs)
	rep, err := r.Run(context.Background())
	if err != nil || rep.NotFound != 0 || rep.Fingerprinted != 1 || rep.Skipped != 1 || len(f.puts) != 1 {
		t.Fatalf("%+v %v logs %v", rep, err, logs)
	}
	before := f.windows
	r.Run(context.Background())
	if f.windows != before {
		t.Fatalf("the skipped clip was fetched again (%d to %d downloads)", before, f.windows)
	}
}

func TestRepairLeavesAClipWhoseFingerprintCannotBeCutRatherThanDeletingIt(t *testing.T) {
	f := newFakeRepair()
	f.said = map[float64]string{3185: q1, 3190: q2} // found, but 2785 s later: the clip then runs past the end
	var logs []string
	rep, err := newRepairer(f, &logs).Run(context.Background())
	if err != nil || len(f.puts) != 0 {
		t.Fatalf("%+v %v puts %v logs %v", rep, err, f.puts, logs)
	}
	if rep.Skipped != 2 || rep.NotFound != 0 {
		t.Fatalf("both clips should be skipped: %+v", rep)
	}
}

func TestRepairBudgetEndsARunWithoutDeletingAnything(t *testing.T) {
	f := newFakeRepair()
	var logs []string
	r := newRepairer(f, &logs)
	r.Budget = time.Nanosecond
	time.Sleep(time.Millisecond)
	rep, err := r.Run(context.Background())
	if err != nil || f.windows != 0 || len(f.puts) != 0 {
		t.Fatalf("%+v %v downloads %d", rep, err, f.windows)
	}
}

func TestRepairRecordsASkipForAClipWhoseEpisodeIsNotListed(t *testing.T) {
	f := newFakeRepair()
	f.eps = nil
	var logs []string
	r := newRepairer(f, &logs)
	r.Run(context.Background())
	n := len(logs)
	r.Run(context.Background())
	for _, l := range logs[n:] {
		if strings.Contains(l, "not in the episode list") {
			t.Fatalf("logged again: %s", l)
		}
	}
}

func TestQuoteStartFindsAQuoteThatRunsAcrossTwoSegments(t *testing.T) {
	segs := []Segment{{Start: 10, End: 14, Text: "Intro words."}, {Start: 20, End: 24, Text: "The harbor reopened this"}, {Start: 25, End: 30, Text: "morning after weeks of repairs."}}
	h := HeldClip{Start: 15, End: 40, Quote: "The harbor reopened this morning after weeks"}
	if at := quoteStart(segs, h); at != 20 {
		t.Fatalf("quote start %v, want 20 (the segment holding its first word)", at)
	}
}

func TestRepairIgnoresFingerprintedClipsAndDryRunWritesNothing(t *testing.T) {
	f := newFakeRepair()
	f.held[0].Fingerprint = &podcastfp.ClipRecord{Version: 1}
	var logs []string
	r := newRepairer(f, &logs)
	r.DryRun = true
	rep, err := r.Run(context.Background())
	if err != nil || len(f.puts) != 0 || rep.Unfingerprinted != 1 || rep.NotFound != 1 || rep.Fingerprinted != 0 {
		t.Fatalf("%+v %v puts %v", rep, err, f.puts)
	}
}

func TestRepairLeavesClipsAloneOnAnEngineFailureAndBacksOff(t *testing.T) {
	f := newFakeRepair()
	f.engineErr = errors.New("download: status 503")
	var logs []string
	r := newRepairer(f, &logs)
	rep, err := r.Run(context.Background())
	if err != nil || rep.Failed != 2 || len(f.puts) != 0 {
		t.Fatalf("%+v %v puts %v", rep, err, f.puts)
	}
	rep, _ = r.Run(context.Background())
	if f.windows != 1 || rep.Skipped != 2 {
		t.Fatalf("retried at once: %d downloads, %+v", f.windows, rep)
	}
}

func TestRepairMergesNearbyClipsIntoOneSpan(t *testing.T) {
	f := newFakeRepair()
	f.held[1].Start, f.held[1].End = 500, 540
	f.segs["g1"][2] = Segment{Start: 505, End: 515, Text: q2}
	f.said[515] = q2
	var logs []string
	rep, err := newRepairer(f, &logs).Run(context.Background())
	if err != nil || len(f.spansSeen[0]) != 1 || rep.Fingerprinted != 2 || f.windows != 1 {
		t.Fatalf("%+v %v spans %v downloads %d", rep, err, f.spansSeen, f.windows)
	}
}

// B5: a new worker against an old platform.
func TestMatcherRetriesAClipWithoutItsFingerprintWhenThePlatformRefusesTheField(t *testing.T) {
	b := newFakeMatch()
	b.putErr = func(c ClipIn) error {
		if c.Fingerprint != nil {
			return &HTTPError{Method: "PUT", Path: "/clips", Status: 400, Body: "invalid request body"}
		}
		return nil
	}
	var logs []string
	m := newMatcher(b, &logs)
	rec, err := podcastfp.Cut(podcastfp.Frames(podcastfp.FixturePCM(16000*100)), 0, 10, 50, podcastfp.FileInfo{})
	if err != nil {
		t.Fatal(err)
	}
	fresh := []ClipIn{{StoryRoute: "/a", EpisodeID: "ep-1", Start: 10, End: 50, Quote: "q", Fingerprint: rec}, {StoryRoute: "/b", EpisodeID: "ep-1", Start: 10, End: 50, Quote: "q", Fingerprint: rec}}
	errs := m.putClips(context.Background(), fresh)
	if errs[0] != nil || errs[1] != nil || fresh[0].Fingerprint != nil || fresh[1].Fingerprint != nil {
		t.Fatalf("errors %v clips %+v", errs, fresh)
	}
	if n := strings.Count(strings.Join(logs, "\n"), "took it without one"); n != 1 {
		t.Fatalf("the downgrade is logged %d times, want once: %v", n, logs)
	}
	// A clip refused even without the fingerprint stays refused.
	b.putErr = func(ClipIn) error { return &HTTPError{Method: "PUT", Path: "/clips", Status: 400, Body: "no"} }
	withFP := []ClipIn{{StoryRoute: "/a", EpisodeID: "ep-1", Start: 10, End: 50, Quote: "q", Fingerprint: rec}}
	if errs := m.putClips(context.Background(), withFP); errs[0] == nil {
		t.Fatal("a refused clip counted as written")
	}
}

func TestRepairSkipsAClipUnder24SecondsAndDeletesNothing(t *testing.T) {
	f := newFakeRepair()
	f.held = f.held[:1]
	f.held[0].End = f.held[0].Start + 21
	var logs []string
	r := newRepairer(f, &logs)
	rep, err := r.Run(context.Background())
	if err != nil || f.windows != 0 || len(f.puts) != 0 || rep.Skipped != 1 {
		t.Fatalf("%+v %v downloads %d puts %v", rep, err, f.windows, f.puts)
	}
}

func TestLocalFingerprintShapeAndArguments(t *testing.T) {
	f := newFakeRepair()
	f.said = map[float64]string{417: q1}
	c, err := LocalFingerprint(context.Background(), f, "https://cdn.example.test/a.mp3", 400, 440, "")
	if err != nil {
		t.Fatal(err)
	}
	if c.ClipID != "local" || c.AudioURL != "https://cdn.example.test/a.mp3" || c.Start != 400 || c.End != 440 || c.Fingerprint == nil || c.Fingerprint.Start != 400 || f.windows != 1 {
		t.Fatalf("seconds trusted: %+v %+v downloads %d", c, c.Fingerprint, f.windows)
	}
	// With the quote the clip is found where it now is: 17 s later.
	f2 := newFakeRepair()
	c, err = LocalFingerprint(context.Background(), f2, "https://cdn.example.test/a.mp3", 400, 440, q1)
	if err != nil || c.Start != 400 || c.Fingerprint.Start != 417 || c.Fingerprint.End != 457 || c.Fingerprint.HeadAt != 417 || c.Fingerprint.Validate() != nil {
		t.Fatalf("quote: %v %+v", err, c)
	}
	raw, _ := json.Marshal(c)
	var shape map[string]any
	json.Unmarshal(raw, &shape)
	for _, k := range []string{"clip_id", "story", "show", "episode_title", "episode_guid", "feed_url", "published", "audio_url", "duration_seconds", "start", "end", "quote", "support", "fingerprint"} {
		if _, ok := shape[k]; !ok {
			t.Fatalf("field %q missing from %s", k, raw)
		}
	}
	for _, k := range []string{"head", "tail", "head_at", "tail_at", "start", "end", "version", "seekable_by_bitrate"} {
		if _, ok := shape["fingerprint"].(map[string]any)[k]; !ok {
			t.Fatalf("fingerprint field %q missing", k)
		}
	}
	for name, a := range map[string][4]any{"no url": {"", 1.0, 40.0, ""}, "short": {"u", 10.0, 20.0, ""}, "unset": {"u", -1.0, -1.0, ""}} {
		if _, err := LocalFingerprint(context.Background(), f, a[0].(string), a[1].(float64), a[2].(float64), a[3].(string)); err == nil {
			t.Fatalf("%s accepted", name)
		}
	}
	f3 := newFakeRepair()
	f3.said = nil
	if _, err := LocalFingerprint(context.Background(), f3, "u", 400, 440, q1); err == nil {
		t.Fatal("a quote that is not in the episode gave a clip")
	}
}
