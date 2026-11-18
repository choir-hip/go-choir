package podcastworker

// The fingerprint repair pass. A clip the platform holds with no fingerprint
// had its audio deleted long ago, and the publisher serves another file now
// (other ads). So: download the episode again, transcribe the minutes around
// the clip's stored time (the recogniser is the worker's own), find the clip's
// quote in that audio, and fingerprint the clip where it now is. If the quote
// is not found there, the episode was re-edited and the clip no longer says
// what its quote says: it is deleted, with the reason logged. The audio is
// deleted after use; nothing here calls a model service.

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/podcastfp"
)

const (
	// RepairMargin is how much audio on each side of the stored clip is
	// transcribed: offsets between two downloads of one episode measured -37 to
	// +130 s, all within 240 s.
	RepairMargin = 300.0
	// QuoteFoundScore is the share of the quote's words that must be heard, in
	// order, for the clip to count as found.
	QuoteFoundScore = 0.75
	repairSkip      = 6 * time.Hour
)

// Span is a stretch of an episode's file, in whole seconds from its start.
// {0, 0} is the whole file.
type Span struct{ T0, T1 float64 }

// SpanAudio is a transcribed span: words with times in file seconds, and the
// fingerprint track of the span (frame 0 at T0).
type SpanAudio struct {
	T0    float64
	Words []Word
	Track []byte
	// Complete: the span was the whole file (T1 0) and it was transcribed to
	// within 3 s of the file's length. Only then does a missing quote prove
	// the episode no longer says it.
	Complete bool
}

// WindowEngine fetches an episode once and transcribes spans of it.
type WindowEngine interface {
	Windows(ctx context.Context, enclosureURL string, spans []Span) (podcastfp.FileInfo, []SpanAudio, error)
}

// RepairBackend is what the pass can reach on the platform.
type RepairBackend interface {
	Front(ctx context.Context, limit int) ([]FrontStory, error)
	Episodes(ctx context.Context, since time.Time, limit int) ([]EpisodeInfo, error)
	Transcript(ctx context.Context, feedURL, guid string) ([]Segment, error)
	Clips(ctx context.Context, routes []string) ([]HeldClip, error)
	PutClipsEach(ctx context.Context, clips []ClipIn) []error
	DeleteClips(ctx context.Context, ids []string) error
}

// Repairer is the pass.
type Repairer struct {
	B      RepairBackend
	Eng    WindowEngine
	DryRun bool
	// Budget is the wall time one Run may spend (default 20 minutes): when it
	// is spent the episode in hand is dropped (nothing from it is written or
	// deleted) and the rest wait for the next run.
	Budget time.Duration
	Now    func() time.Time
	Logf   func(string, ...any)

	skipUntil map[string]time.Time // episode id or clip id -> not before
}

// RepairReport is one run's counts.
type RepairReport struct {
	Unfingerprinted, Fingerprinted, Deleted, Failed, Skipped int
}

func (r *Repairer) logf(f string, a ...any) {
	if r.Logf != nil {
		r.Logf(f, a...)
	}
}

func (r *Repairer) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}

type repairClip struct {
	held    HeldClip
	oldAt   float64 // where the quote starts in the stored transcript
	span    int     // index into the episode's spans
	episode EpisodeInfo
}

// Run repairs once.
func (r *Repairer) Run(ctx context.Context) (RepairReport, error) {
	var rep RepairReport
	if r.skipUntil == nil {
		r.skipUntil = map[string]time.Time{}
	}
	front, err := r.B.Front(ctx, 100)
	if err != nil {
		return rep, fmt.Errorf("front list: %w", err)
	}
	var routes []string
	for _, st := range front {
		if st.Kind != "opinion" && st.Route.Bare != "" && r.now().Sub(st.UpdatedAt) <= StoryWindow {
			routes = append(routes, st.Route.Bare)
		}
	}
	if len(routes) == 0 {
		return rep, nil
	}
	held, err := r.B.Clips(ctx, routes)
	if err != nil {
		return rep, fmt.Errorf("read the clips the platform holds: %w", err)
	}
	var todo []HeldClip
	for _, h := range held {
		if h.Fingerprint == nil && h.ClipID != "" {
			rep.Unfingerprinted++
			if r.now().Before(r.skipUntil[h.ClipID]) {
				rep.Skipped++
				continue
			}
			if h.End-h.Start < 2*podcastfp.ClipFrames/podcastfp.FPS {
				// Too short to carry a fingerprint: valid, left as it is, never retried.
				r.skipUntil[h.ClipID] = r.now().Add(100 * 365 * 24 * time.Hour)
				rep.Skipped++
				continue
			}
			todo = append(todo, h)
		}
	}
	if len(todo) == 0 {
		return rep, nil
	}
	eps, err := r.B.Episodes(ctx, r.now().Add(-14*24*time.Hour), 500)
	if err != nil {
		return rep, fmt.Errorf("episodes: %w", err)
	}
	byKey := map[string]EpisodeInfo{}
	for _, e := range eps {
		byKey[e.FeedURL+"\x00"+e.GUID] = e
	}
	byEpisode := map[string][]HeldClip{}
	var order []string
	for _, h := range todo {
		e, ok := byKey[h.FeedURL+"\x00"+h.GUID]
		if !ok {
			r.logf("podcast-worker repair: clip %s: its episode is not in the episode list; left as it is, not looked at again for %s", h.ClipID, repairSkip)
			r.skipUntil[h.ClipID] = r.now().Add(repairSkip)
			rep.Skipped++
			continue
		}
		if _, seen := byEpisode[e.EpisodeID]; !seen {
			order = append(order, e.EpisodeID)
		}
		byEpisode[e.EpisodeID] = append(byEpisode[e.EpisodeID], h)
	}
	budget := r.Budget
	if budget <= 0 {
		budget = 20 * time.Minute
	}
	bctx, cancel := context.WithTimeout(ctx, budget)
	defer cancel()
	var puts []ClipIn
	var putIDs []string
	var gone []string
	for _, id := range order {
		if err := ctx.Err(); err != nil {
			return rep, err
		}
		if bctx.Err() != nil {
			r.logf("podcast-worker repair: the run's %s is spent; the remaining episodes wait for the next run", budget)
			break
		}
		clips := byEpisode[id]
		e := byKey[clips[0].FeedURL+"\x00"+clips[0].GUID]
		if r.now().Before(r.skipUntil[id]) {
			rep.Skipped += len(clips)
			continue
		}
		in, ids, del, skipped, err := r.repairEpisode(bctx, e, clips)
		if err != nil {
			if ctx.Err() != nil {
				return rep, ctx.Err()
			}
			if bctx.Err() != nil {
				continue // the budget ended it: nothing from this episode is used
			}
			rep.Failed += len(clips)
			r.skipUntil[id] = r.now().Add(repairSkip)
			r.logf("podcast-worker repair: episode %s (%s): %d clips left as they are, retried after %s: %v", id, e.Show, len(clips), repairSkip, err)
			continue
		}
		for _, c := range skipped {
			r.skipUntil[c] = r.now().Add(repairSkip)
			rep.Skipped++
		}
		puts, putIDs, gone = append(puts, in...), append(putIDs, ids...), append(gone, del...)
	}
	if r.DryRun {
		rep.Fingerprinted, rep.Deleted = len(puts), len(gone)
		return rep, nil
	}
	if len(puts) > 0 {
		for i, err := range r.B.PutClipsEach(ctx, puts) {
			if err != nil {
				rep.Failed++
				r.skipUntil[putIDs[i]] = r.now().Add(repairSkip)
				r.logf("podcast-worker repair: clip %s not updated: %v", putIDs[i], err)
				continue
			}
			rep.Fingerprinted++
		}
	}
	if len(gone) > 0 {
		if err := r.B.DeleteClips(ctx, gone); err != nil {
			rep.Failed += len(gone)
			return rep, fmt.Errorf("delete clips: %w", err)
		}
		rep.Deleted = len(gone)
	}
	r.logf("podcast-worker repair: %d clips without a fingerprint, %d fingerprinted, %d deleted, %d failed, %d skipped", rep.Unfingerprinted, rep.Fingerprinted, rep.Deleted, rep.Failed, rep.Skipped)
	return rep, nil
}

// repairEpisode makes the clip writes, the deletions and the skips for one
// episode. A clip is deleted only when the whole served file was transcribed
// completely and its quote is not in it; a quote missed in the window around
// the stored place gets one search of the whole file (a second download), and
// any failure leaves the clip as it is.
func (r *Repairer) repairEpisode(ctx context.Context, e EpisodeInfo, clips []HeldClip) (puts []ClipIn, putIDs, gone, skipped []string, err error) {
	segs, err := r.B.Transcript(ctx, e.FeedURL, e.GUID)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("stored transcript: %w", err)
	}
	// Where each clip's quote begins in the stored transcript: the start of the
	// segment holding it (a segment starts at its first word).
	type item struct {
		h     HeldClip
		oldAt float64
		span  Span
	}
	items := make([]item, len(clips))
	for i, h := range clips {
		items[i] = item{h: h, oldAt: quoteStart(segs, h), span: Span{math.Floor(math.Max(0, h.Start-RepairMargin)), math.Ceil(h.End + RepairMargin)}}
	}
	spans := mergeSpans(items, func(i int) Span { return items[i].span })
	file, audio, err := r.Eng.Windows(ctx, e.EnclosureURL, spans)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	if len(audio) != len(spans) {
		return nil, nil, nil, nil, fmt.Errorf("engine returned %d spans for %d", len(audio), len(spans))
	}
	// place fingerprints a clip whose quote was heard at time at in sa.
	place := func(it item, sa *SpanAudio, file podcastfp.FileInfo, at, score float64) {
		offset := at - it.oldAt
		start, end := it.h.Start+offset, it.h.End+offset
		rec, cerr := podcastfp.Cut(sa.Track, sa.T0, start, end, file)
		if cerr != nil {
			// Not evidence that the quote is gone: the clip stays as it is.
			r.logf("podcast-worker repair: clip %s left as it is: its quote is found %+.1f s from where it was, but it cannot be fingerprinted there: %v", it.h.ClipID, offset, cerr)
			skipped = append(skipped, it.h.ClipID)
			return
		}
		r.logf("podcast-worker repair: clip %s: quote found %+.1f s from its stored place (match %.2f), fingerprinted at %.1f-%.1f s", it.h.ClipID, offset, score, start, end)
		puts = append(puts, ClipIn{StoryRoute: it.h.Route, EpisodeID: e.EpisodeID, Start: it.h.Start, End: it.h.End, Quote: it.h.Quote, Support: it.h.Support, Fingerprint: rec})
		putIDs = append(putIDs, it.h.ClipID)
	}
	var missed []item
	for _, it := range items {
		var sa *SpanAudio
		for i := range spans {
			if it.span.T0 >= spans[i].T0 && it.span.T1 <= spans[i].T1 {
				sa = &audio[i]
				break
			}
		}
		if sa == nil {
			return nil, nil, nil, nil, errors.New("a clip fits none of the spans")
		}
		at, score := FindQuote(sa.Words, it.h.Quote, it.oldAt)
		if score < QuoteFoundScore {
			missed = append(missed, it)
			continue
		}
		place(it, sa, file, at, score)
	}
	if len(missed) == 0 {
		return puts, putIDs, gone, skipped, nil
	}
	file2, whole, werr := r.Eng.Windows(ctx, e.EnclosureURL, []Span{{}})
	if werr != nil || len(whole) != 1 {
		if ctx.Err() != nil {
			return nil, nil, nil, nil, ctx.Err()
		}
		r.logf("podcast-worker repair: episode %s: %d clips not found near their stored place and the whole file could not be searched (%v); left as they are", e.EpisodeID, len(missed), werr)
		for _, it := range missed {
			skipped = append(skipped, it.h.ClipID)
		}
		return puts, putIDs, gone, skipped, nil
	}
	sa := &whole[0]
	for _, it := range missed {
		at, score := FindQuote(sa.Words, it.h.Quote, it.oldAt)
		switch {
		case score >= QuoteFoundScore:
			place(it, sa, file2, at, score)
		case sa.Complete:
			r.logf("podcast-worker repair: clip %s deleted: its quote %q is not anywhere in the %.0f s of the episode as served now (best match %.2f of %.2f needed; story %s, stored %.0f-%.0f s): the episode was edited",
				it.h.ClipID, truncateRunes(it.h.Quote, 60), file2.Seconds, score, QuoteFoundScore, it.h.Route, it.h.Start, it.h.End)
			gone = append(gone, it.h.ClipID)
		default:
			r.logf("podcast-worker repair: clip %s left as it is: its quote is not in the audio transcribed, which did not cover the whole file", it.h.ClipID)
			skipped = append(skipped, it.h.ClipID)
		}
	}
	return puts, putIDs, gone, skipped, nil
}

func mergeSpans[T any](items []T, span func(i int) Span) []Span {
	ss := make([]Span, len(items))
	for i := range items {
		ss[i] = span(i)
	}
	sort.Slice(ss, func(i, j int) bool { return ss[i].T0 < ss[j].T0 })
	var out []Span
	for _, s := range ss {
		if n := len(out); n > 0 && s.T0 <= out[n-1].T1 {
			out[n-1].T1 = math.Max(out[n-1].T1, s.T1)
			continue
		}
		out = append(out, s)
	}
	return out
}

// quoteStart is where the clip's quote begins in the stored transcript: the
// start of the segment holding its first word, found over the clip's segments
// joined (a quote may run across two).
func quoteStart(segs []Segment, h HeldClip) float64 {
	quote := strings.Join(strings.Fields(h.Quote), " ")
	var joined strings.Builder
	var starts []int
	var times []float64
	for _, s := range segs {
		if s.End > h.Start-0.5 && s.Start < h.End+0.5 {
			if joined.Len() > 0 {
				joined.WriteByte(' ')
			}
			starts = append(starts, joined.Len())
			times = append(times, s.Start)
			joined.WriteString(strings.Join(strings.Fields(s.Text), " "))
		}
	}
	idx := strings.Index(joined.String(), quote)
	if idx < 0 || quote == "" {
		return h.Start
	}
	at := h.Start
	for i, off := range starts {
		if off <= idx {
			at = times[i]
		}
	}
	return at
}

func truncateRunes(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n]) + "..."
	}
	return s
}

// Windows downloads the enclosure once, then for each span cuts the audio,
// transcribes it and builds its fingerprint track. The download and every cut
// are deleted before it returns.
func (t *Transcriber) Windows(ctx context.Context, enclosureURL string, spans []Span) (podcastfp.FileInfo, []SpanAudio, error) {
	if err := t.checkDisk(); err != nil {
		return podcastfp.FileInfo{}, nil, err
	}
	dir, err := os.MkdirTemp(t.WorkDir, "episode-")
	if err != nil {
		return podcastfp.FileInfo{}, nil, err
	}
	defer os.RemoveAll(dir)
	src := dir + "/in.audio"
	if err := t.download(ctx, enclosureURL, src); err != nil {
		return podcastfp.FileInfo{}, nil, err
	}
	if head, err := readHead(src, 512); err == nil && looksLikePlaylist(head) {
		return podcastfp.FileInfo{}, nil, errors.New("the download is a playlist, not audio: refused")
	}
	file, _ := podcastfp.AnalyzeMP3(src)
	file.Seconds = probeSeconds(ctx, t.FFmpeg, src)
	var out []SpanAudio
	for i, sp := range spans {
		wav := fmt.Sprintf("%s/span%d.wav", dir, i)
		args := []string{"-nostdin", "-hide_banner", "-loglevel", "error", "-y", "-protocol_whitelist", "file", "-ss", fmt.Sprint(sp.T0)}
		if sp.T1 > 0 {
			args = append(args, "-t", fmt.Sprint(sp.T1-sp.T0))
		}
		args = append(args, "-i", src, "-ar", "16000", "-ac", "1", "-c:a", "pcm_s16le", wav)
		if o, err := exec.CommandContext(ctx, t.FFmpeg, args...).CombinedOutput(); err != nil {
			return file, nil, fmt.Errorf("ffmpeg: %v: %s", err, tail(o))
		}
		track := wavTrack(wav)
		if track == nil {
			return file, nil, errors.New("the cut audio could not be fingerprinted")
		}
		parsed, err := t.runParakeet(ctx, wav)
		if err != nil {
			return file, nil, err
		}
		os.Remove(wav)
		words := make([]Word, len(parsed.Words))
		for j, w := range parsed.Words {
			words[j] = Word{W: w.W, Start: w.Start + sp.T0, End: w.End + sp.T0}
		}
		complete := sp.T0 == 0 && sp.T1 == 0 && file.Seconds > 0 && float64(len(track)/podcastfp.Bands)/podcastfp.FPS >= file.Seconds-3
		out = append(out, SpanAudio{T0: sp.T0, Words: words, Track: track, Complete: complete})
	}
	return file, out, nil
}
