package podcastworker

// `podcast-worker fingerprint`: a read-only diagnostic. It downloads one
// episode as the worker does, finds the clip (by its quote as the repair pass
// does, or by trusting the given seconds) and returns one clip as the platform's
// clips list would show it, fingerprint included. No platform route is called
// and no key is needed; the audio is deleted when it returns.

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/yusefmosiah/go-choir/internal/podcastfp"
)

// LocalClip is shaped like an element of GET /api/platform/podcast-clips.
type LocalClip struct {
	ClipID string `json:"clip_id"`
	Story  struct {
		Route   string `json:"route"`
		Version int    `json:"version"`
		Title   string `json:"title"`
	} `json:"story"`
	Show            string                `json:"show"`
	EpisodeTitle    string                `json:"episode_title"`
	EpisodeGUID     string                `json:"episode_guid"`
	FeedURL         string                `json:"feed_url"`
	Published       time.Time             `json:"published"`
	AudioURL        string                `json:"audio_url"`
	DurationSeconds float64               `json:"duration_seconds"`
	Start           float64               `json:"start"`
	End             float64               `json:"end"`
	Quote           string                `json:"quote"`
	Support         float64               `json:"support"`
	Fingerprint     *podcastfp.ClipRecord `json:"fingerprint"`
}

// LocalFingerprint builds the clip. Without a quote, start and end are trusted
// as seconds of the download; with one, the quote is searched for (near start,
// then in the whole file) and is taken to begin where the clip does, so the
// fingerprint is cut at the clip's place in this download.
func LocalFingerprint(ctx context.Context, eng WindowEngine, audioURL string, start, end float64, quote string) (*LocalClip, error) {
	switch {
	case audioURL == "":
		return nil, errors.New("fingerprint needs --audio-url")
	case math.IsNaN(start) || math.IsNaN(end) || start < 0 || end-start < 2*podcastfp.ClipFrames/podcastfp.FPS:
		return nil, fmt.Errorf("fingerprint needs --start and --end at least %d s apart", 2*podcastfp.ClipFrames/podcastfp.FPS)
	}
	span := Span{math.Floor(math.Max(0, start-1)), math.Ceil(end + 1)}
	if quote != "" {
		span = Span{math.Floor(math.Max(0, start-RepairMargin)), math.Ceil(end + RepairMargin)}
	}
	file, audio, err := eng.Windows(ctx, audioURL, []Span{span})
	if err != nil {
		return nil, err
	}
	if len(audio) != 1 {
		return nil, fmt.Errorf("engine returned %d spans for 1", len(audio))
	}
	sa := &audio[0]
	shift := 0.0
	if quote != "" {
		at, score := FindQuote(sa.Words, quote, start)
		if score < QuoteFoundScore {
			file, audio, err = eng.Windows(ctx, audioURL, []Span{{}})
			if err != nil || len(audio) != 1 {
				return nil, fmt.Errorf("the quote is not within %.0f s of %.0f s and the whole file could not be searched: %v", RepairMargin, start, err)
			}
			sa = &audio[0]
			if at, score = FindQuote(sa.Words, quote, start); score < QuoteFoundScore {
				return nil, fmt.Errorf("the quote is not in the episode (best match %.2f of %.2f needed)", score, QuoteFoundScore)
			}
		}
		shift = at - start
	}
	rec, err := podcastfp.Cut(sa.Track, sa.T0, start+shift, end+shift, file)
	if err != nil {
		return nil, err
	}
	c := &LocalClip{ClipID: "local", AudioURL: audioURL, DurationSeconds: file.Seconds, Start: start, End: end, Quote: quote, Fingerprint: rec}
	return c, nil
}
