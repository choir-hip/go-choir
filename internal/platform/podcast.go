package platform

// Podcast clips in the Autopaper object graph (docs/design/podcast-clips-2026-11-15.md).
//
// Three kinds, all written by corpusd's podcast service and nobody else
// (residual R1: a desk should be the custodian): choir.podcast_episode (a
// feed item's metadata), choir.podcast_transcript (one per episode: timed
// segments, never words) and choir.podcast_clip (a time range of an episode
// that sources a paper story). Audio is never stored: a clip names the
// publisher's own enclosure URL, which the phone streams.
//
// The write routes are operator-only (the proxy gate) and the public read
// route serves clips alone: a short quote, never the transcript.

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

const (
	ogKindPodcastEpisode    = objectgraph.ObjectKind("choir.podcast_episode")
	ogKindPodcastTranscript = objectgraph.ObjectKind("choir.podcast_transcript")
	ogKindPodcastClip       = objectgraph.ObjectKind("choir.podcast_clip")

	// podcastOwner owns the rows: they are public-corpus data of the
	// platform, not of a user.
	podcastOwner = "platform"

	podcastMaxEpisodesPerRequest = 200
	podcastMaxNotesBytes         = 2000
	podcastMaxURLBytes           = 2048
	podcastMaxFieldBytes         = 300
	podcastMaxSegments           = 40000
	podcastMaxSegmentBytes       = 2000
	podcastMaxClipsPerRequest    = 200
	podcastClipMinSeconds        = 5.0
	podcastClipMaxSeconds        = 120.0
	podcastQuoteMaxRunes         = 280
	// podcastClipsPerStory is how many clips one story shows.
	podcastClipsPerStory = 3
	// PodcastClipsDefaultWindow is how far back the unfiltered read looks.
	PodcastClipsDefaultWindow = 72 * time.Hour
	podcastClipsScan          = 1000
	podcastClipsMaxRoutes     = 60
	podcastClipsDefaultLimit  = 120
	podcastClipsMaxLimit      = 200
	podcastReadCacheTTL       = 30 * time.Second
)

// PodcastEpisodeIn is one feed item as the operator's worker reports it.
type PodcastEpisodeIn struct {
	FeedURL         string  `json:"feed_url"`
	GUID            string  `json:"guid"`
	Show            string  `json:"show"`
	Title           string  `json:"title"`
	Published       string  `json:"published"`
	DurationSeconds float64 `json:"duration_seconds"`
	EnclosureURL    string  `json:"enclosure_url"`
	Notes           string  `json:"notes,omitempty"`
}

// PodcastEpisode is a stored episode.
type PodcastEpisode struct {
	EpisodeID       string    `json:"episode_id"`
	FeedURL         string    `json:"feed_url"`
	GUID            string    `json:"guid"`
	Show            string    `json:"show"`
	Title           string    `json:"title"`
	Published       time.Time `json:"published"`
	DurationSeconds float64   `json:"duration_seconds"`
	EnclosureURL    string    `json:"enclosure_url"`
	Notes           string    `json:"notes,omitempty"`
	HasTranscript   bool      `json:"has_transcript"`
	// StableFile is whether the worker saw the publisher serve one file
	// across the transcription; nil when it did not check. Clips of a stable
	// file are shown first (an ad-stitched file may not line up).
	StableFile *bool `json:"stable_file,omitempty"`
}

// PodcastSegment is one sentence or pause bounded stretch of a transcript.
type PodcastSegment struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Text  string  `json:"text"`
}

// PodcastTranscript is a stored transcript.
type PodcastTranscript struct {
	EpisodeID    string  `json:"episode_id"`
	Engine       string  `json:"engine"`
	AudioSeconds float64 `json:"audio_seconds"`
	// FileSeconds is the length of the whole file as it was downloaded, when
	// the worker measured it. A publisher that stitches ads in serves another
	// file on the next download, so this, not the feed's duration, is the
	// length a clip's time is valid for.
	FileSeconds float64          `json:"file_seconds,omitempty"`
	StableFile  *bool            `json:"stable_file,omitempty"`
	Segments    []PodcastSegment `json:"segments"`
}

// PodcastClipIn is one clip an operator, or the match job, proposes.
type PodcastClipIn struct {
	StoryRoute string  `json:"story_route"`
	EpisodeID  string  `json:"episode_id"`
	Start      float64 `json:"start"`
	End        float64 `json:"end"`
	Quote      string  `json:"quote"`
	Support    float64 `json:"support"`
}

// podcastClipRow is a stored clip.
type podcastClipRow struct {
	ClipID       string  `json:"clip_id"`
	StoryRoute   string  `json:"story_route"`
	StoryVersion int     `json:"story_version"`
	StoryTitle   string  `json:"story_title"`
	EpisodeID    string  `json:"episode_id"`
	Start        float64 `json:"start"`
	End          float64 `json:"end"`
	Quote        string  `json:"quote"`
	Support      float64 `json:"support"`
	MatchedAt    string  `json:"matched_at"`
}

// PodcastClipStory is the story a clip is for.
type PodcastClipStory struct {
	Route   string `json:"route"`
	Version int    `json:"version"`
	Title   string `json:"title"`
}

// PodcastClip is the public shape of a clip (design 4.1).
type PodcastClip struct {
	ClipID          string           `json:"clip_id"`
	Story           PodcastClipStory `json:"story"`
	Show            string           `json:"show"`
	EpisodeTitle    string           `json:"episode_title"`
	EpisodeGUID     string           `json:"episode_guid"`
	FeedURL         string           `json:"feed_url"`
	Published       time.Time        `json:"published"`
	AudioURL        string           `json:"audio_url"`
	DurationSeconds float64          `json:"duration_seconds"`
	Start           float64          `json:"start"`
	End             float64          `json:"end"`
	Quote           string           `json:"quote"`
	Support         float64          `json:"support"`
}

// PodcastClipsResponse is the public route's answer.
type PodcastClipsResponse struct {
	GeneratedAt time.Time     `json:"generated_at"`
	Clips       []PodcastClip `json:"clips"`
}

// ErrPodcastInvalid marks a request the podcast service refuses as malformed.
var ErrPodcastInvalid = errors.New("invalid podcast request")

func podcastInvalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrPodcastInvalid, fmt.Sprintf(format, args...))
}

func podcastHashID(prefix, key string) string {
	sum := sha256.Sum256([]byte(key))
	return prefix + hex.EncodeToString(sum[:])[:24]
}

// PodcastEpisodeID is an episode's id from its feed and guid.
func PodcastEpisodeID(feedURL, guid string) string {
	return podcastHashID("ep-", feedURL+"\x00"+guid)
}

func podcastClipID(route, episodeID string, start float64) string {
	return podcastHashID("clip-", fmt.Sprintf("%s\x00%s\x00%d", route, episodeID, int(math.Round(start))))
}

func podcastCanonical(kind objectgraph.ObjectKind, id string) (string, error) {
	return objectgraph.BuildCanonicalID(kind, podcastOwner, objectgraph.StableSuffixFromKey(id))
}

func isNotFound(err error) bool {
	return errors.Is(err, sql.ErrNoRows) || errors.Is(err, objectgraph.ErrNotFound)
}

func podcastObject(kind objectgraph.ObjectKind, id string, body any, metadata map[string]any, now time.Time) (objectgraph.Object, error) {
	canonical, err := podcastCanonical(kind, id)
	if err != nil {
		return objectgraph.Object{}, err
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return objectgraph.Object{}, err
	}
	meta, err := objectgraph.NormalizeMetadata(metadata)
	if err != nil {
		return objectgraph.Object{}, err
	}
	return objectgraph.Object{
		CanonicalID: canonical, ObjectKind: kind, OwnerID: podcastOwner,
		ContentHash: objectgraph.ContentHash(kind, raw, meta), Body: raw, Metadata: meta,
		CreatedAt: now, UpdatedAt: now,
	}, nil
}

// checkPodcastURL admits an http or https URL with no credentials. The
// service stores URLs and never fetches them.
func checkPodcastURL(name, raw string) error {
	if raw == "" || len(raw) > podcastMaxURLBytes {
		return podcastInvalid("%s is empty or over %d bytes", name, podcastMaxURLBytes)
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil {
		return podcastInvalid("%s is not an http or https URL without credentials", name)
	}
	return nil
}

func boundField(name, v string, max int) (string, error) {
	v = strings.TrimSpace(v)
	if !utf8.ValidString(v) || strings.ContainsAny(v, "\x00\r\n") {
		return "", podcastInvalid("%s is not one clean line", name)
	}
	if len(v) > max {
		return "", podcastInvalid("%s is over %d bytes", name, max)
	}
	return v, nil
}

func finiteSeconds(name string, v float64) error {
	if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > 24*3600 {
		return podcastInvalid("%s is not a time within a day", name)
	}
	return nil
}

func (s *Service) podcastStore() (*ObjectGraphStore, error) {
	og := s.ogStore()
	if og == nil {
		return nil, fmt.Errorf("platform service: object graph store unavailable")
	}
	return og, nil
}

func (s *Service) getPodcastEpisode(ctx context.Context, og *ObjectGraphStore, id string) (*PodcastEpisode, error) {
	canonical, err := podcastCanonical(ogKindPodcastEpisode, id)
	if err != nil {
		return nil, err
	}
	obj, err := og.GetObject(ctx, canonical)
	if err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	if obj.Tombstone {
		return nil, nil
	}
	var ep PodcastEpisode
	if err := json.Unmarshal(obj.Body, &ep); err != nil {
		return nil, fmt.Errorf("podcast episode %s: %w", id, err)
	}
	return &ep, nil
}

func (s *Service) putPodcastEpisode(ctx context.Context, og *ObjectGraphStore, ep PodcastEpisode, now time.Time) error {
	obj, err := podcastObject(ogKindPodcastEpisode, ep.EpisodeID, ep, map[string]any{
		"published": ep.Published.UTC().Format(time.RFC3339), "has_transcript": ep.HasTranscript,
	}, now)
	if err != nil {
		return err
	}
	return og.PutObject(corpusdWriter(ctx), obj)
}

// RecordPodcastEpisodes upserts feed items. It returns how many it took and
// how many were new. A request is checked whole before anything is written.
func (s *Service) RecordPodcastEpisodes(ctx context.Context, in []PodcastEpisodeIn) (recorded, fresh int, err error) {
	if len(in) == 0 || len(in) > podcastMaxEpisodesPerRequest {
		return 0, 0, podcastInvalid("a request carries 1 to %d episodes", podcastMaxEpisodesPerRequest)
	}
	checked := make([]PodcastEpisode, 0, len(in))
	for i, e := range in {
		ep, err := checkPodcastEpisode(e)
		if err != nil {
			return 0, 0, fmt.Errorf("episode %d: %w", i, err)
		}
		checked = append(checked, ep)
	}
	og, err := s.podcastStore()
	if err != nil {
		return 0, 0, err
	}
	now := time.Now().UTC()
	noted := map[string]bool{}
	for _, ep := range checked {
		prev, err := s.getPodcastEpisode(ctx, og, ep.EpisodeID)
		if err != nil {
			return recorded, fresh, err
		}
		if prev == nil {
			fresh++
		} else {
			// A re-poll never forgets that a transcript exists.
			ep.HasTranscript = prev.HasTranscript
			ep.StableFile = prev.StableFile
			if prev.HasTranscript || ep.DurationSeconds == 0 {
				// The length measured on the transcribed file stands over
				// whatever the feed says now.
				ep.DurationSeconds = prev.DurationSeconds
			}
		}
		if err := s.putPodcastEpisode(ctx, og, ep, now); err != nil {
			return recorded, fresh, err
		}
		recorded++
		if !noted[ep.FeedURL] {
			noted[ep.FeedURL] = true
			if err := s.notePodcastSeedFeed(ctx, og, ep.FeedURL, ep.Show, now); err != nil {
				return recorded, fresh, err
			}
		}
	}
	s.podcastCache.reset()
	return recorded, fresh, nil
}

func checkPodcastEpisode(e PodcastEpisodeIn) (PodcastEpisode, error) {
	var ep PodcastEpisode
	var err error
	if err = checkPodcastURL("feed_url", strings.TrimSpace(e.FeedURL)); err != nil {
		return ep, err
	}
	if err = checkPodcastURL("enclosure_url", strings.TrimSpace(e.EnclosureURL)); err != nil {
		return ep, err
	}
	ep.FeedURL, ep.EnclosureURL = strings.TrimSpace(e.FeedURL), strings.TrimSpace(e.EnclosureURL)
	if ep.GUID, err = boundField("guid", e.GUID, podcastMaxURLBytes); err != nil {
		return ep, err
	}
	if ep.GUID == "" {
		return ep, podcastInvalid("guid is required")
	}
	if ep.Show, err = boundField("show", e.Show, podcastMaxFieldBytes); err != nil {
		return ep, err
	}
	if ep.Title, err = boundField("title", e.Title, podcastMaxFieldBytes); err != nil {
		return ep, err
	}
	if ep.Show == "" || ep.Title == "" {
		return ep, podcastInvalid("show and title are required")
	}
	if ep.Published, err = time.Parse(time.RFC3339, strings.TrimSpace(e.Published)); err != nil {
		return ep, podcastInvalid("published is not RFC 3339")
	}
	ep.Published = ep.Published.UTC()
	if err = finiteSeconds("duration_seconds", e.DurationSeconds); err != nil {
		return ep, err
	}
	ep.DurationSeconds = e.DurationSeconds
	notes := strings.TrimSpace(e.Notes)
	if len(notes) > podcastMaxNotesBytes {
		cut := podcastMaxNotesBytes
		for cut > 0 && !utf8.RuneStart(notes[cut]) {
			cut--
		}
		notes = notes[:cut]
	}
	if !utf8.ValidString(notes) {
		return ep, podcastInvalid("notes are not valid UTF-8")
	}
	ep.Notes = notes
	ep.EpisodeID = PodcastEpisodeID(ep.FeedURL, ep.GUID)
	return ep, nil
}

// ListPodcastEpisodes lists episodes newest first by publication time. With
// needTranscript only those with no transcript. since zero means no bound.
func (s *Service) ListPodcastEpisodes(ctx context.Context, needTranscript bool, since time.Time, limit int) ([]PodcastEpisode, error) {
	og, err := s.podcastStore()
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	objs, err := og.ListObjects(ctx, objectgraph.ListFilter{Kind: ogKindPodcastEpisode, OwnerID: podcastOwner, Limit: 2000})
	if err != nil {
		return nil, err
	}
	var out []PodcastEpisode
	for _, o := range objs {
		if o.Tombstone {
			continue
		}
		var ep PodcastEpisode
		if json.Unmarshal(o.Body, &ep) != nil {
			continue
		}
		if needTranscript && ep.HasTranscript {
			continue
		}
		if !since.IsZero() && ep.Published.Before(since) {
			continue
		}
		out = append(out, ep)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Published.After(out[j].Published) })
	if needTranscript {
		// The worker's queue: what a phone is playing now, then the feeds
		// the most readers follow, then the rest, newest first within each.
		if counts, wanted, err := s.podcastFeedWeights(ctx); err == nil {
			rank := func(e PodcastEpisode) (bool, int) {
				n, _ := NormalizeFeedURL(e.FeedURL)
				return wanted[n][e.GUID], counts[n]
			}
			sort.SliceStable(out, func(i, j int) bool {
				wi, ci := rank(out[i])
				wj, cj := rank(out[j])
				if wi != wj {
					return wi
				}
				return ci > cj
			})
		}
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// PutPodcastTranscript stores an episode's transcript and marks the episode.
func (s *Service) PutPodcastTranscript(ctx context.Context, t PodcastTranscript) (int, error) {
	if t.EpisodeID == "" || len(t.Segments) == 0 || len(t.Segments) > podcastMaxSegments {
		return 0, podcastInvalid("a transcript needs an episode_id and 1 to %d segments", podcastMaxSegments)
	}
	engine, err := boundField("engine", t.Engine, 100)
	if err != nil {
		return 0, err
	}
	t.Engine = engine
	if err := finiteSeconds("audio_seconds", t.AudioSeconds); err != nil {
		return 0, err
	}
	if err := finiteSeconds("file_seconds", t.FileSeconds); err != nil {
		return 0, err
	}
	prevEnd := 0.0
	for i := range t.Segments {
		seg := &t.Segments[i]
		if err := finiteSeconds("segment start", seg.Start); err != nil {
			return 0, err
		}
		if err := finiteSeconds("segment end", seg.End); err != nil {
			return 0, err
		}
		seg.Text = strings.TrimSpace(seg.Text)
		if seg.End <= seg.Start || seg.Start < prevEnd-0.5 || seg.Text == "" || len(seg.Text) > podcastMaxSegmentBytes || !utf8.ValidString(seg.Text) {
			return 0, podcastInvalid("segment %d is empty, over %d bytes, ends before it starts, or is out of order", i, podcastMaxSegmentBytes)
		}
		prevEnd = seg.End
	}
	og, err := s.podcastStore()
	if err != nil {
		return 0, err
	}
	ep, err := s.getPodcastEpisode(ctx, og, t.EpisodeID)
	if err != nil {
		return 0, err
	}
	if ep == nil {
		return 0, podcastInvalid("episode %q is not known", t.EpisodeID)
	}
	now := time.Now().UTC()
	obj, err := podcastObject(ogKindPodcastTranscript, t.EpisodeID, t, map[string]any{
		"episode_id": t.EpisodeID, "engine": t.Engine, "segments": len(t.Segments),
	}, now)
	if err != nil {
		return 0, err
	}
	if err := og.PutObject(corpusdWriter(ctx), obj); err != nil {
		return 0, err
	}
	ep.HasTranscript = true
	if t.StableFile != nil {
		ep.StableFile = t.StableFile
	}
	switch {
	case t.FileSeconds > 0:
		ep.DurationSeconds = t.FileSeconds // what was downloaded beats what the feed said
	case ep.DurationSeconds == 0:
		ep.DurationSeconds = t.AudioSeconds
	}
	if err := s.putPodcastEpisode(ctx, og, *ep, now); err != nil {
		return 0, err
	}
	s.podcastCache.reset()
	return len(t.Segments), nil
}

// GetPodcastTranscript reads an episode's transcript; nil when it has none.
func (s *Service) GetPodcastTranscript(ctx context.Context, og *ObjectGraphStore, episodeID string) (*PodcastTranscript, error) {
	canonical, err := podcastCanonical(ogKindPodcastTranscript, episodeID)
	if err != nil {
		return nil, err
	}
	obj, err := og.GetObject(ctx, canonical)
	if err != nil {
		if isNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	var t PodcastTranscript
	if err := json.Unmarshal(obj.Body, &t); err != nil {
		return nil, err
	}
	return &t, nil
}

// podcastStory is a story a clip can be for: a recent front story.
type podcastStory struct {
	Route    string
	Title    string
	Dek      string
	Revision int
	Updated  time.Time
}

// recentPodcastStories lists the front stories updated within the window.
func (s *Service) recentPodcastStories(ctx context.Context, window time.Duration) ([]podcastStory, error) {
	page, err := s.ListFrontStories(ctx, 100, "")
	if err != nil {
		return nil, err
	}
	cutoff := time.Now().Add(-window)
	var out []podcastStory
	for _, st := range page.Stories {
		if st.Kind == "opinion" || st.UpdatedAt.Before(cutoff) || st.Route.Bare == "" {
			continue
		}
		out = append(out, podcastStory{Route: st.Route.Bare, Title: st.Title, Dek: st.Dek, Revision: st.LatestRevision, Updated: st.UpdatedAt})
	}
	return out, nil
}

// PutPodcastClips stores clips for recent front stories. A request is checked
// whole: every story must be on the front, every episode known, every range
// inside the episode and 5 to 120 seconds. It returns how many it stored.
func (s *Service) PutPodcastClips(ctx context.Context, in []PodcastClipIn) (int, error) {
	if len(in) == 0 || len(in) > podcastMaxClipsPerRequest {
		return 0, podcastInvalid("a request carries 1 to %d clips", podcastMaxClipsPerRequest)
	}
	stories, err := s.recentPodcastStories(ctx, 30*24*time.Hour)
	if err != nil {
		return 0, err
	}
	byRoute := map[string]podcastStory{}
	for _, st := range stories {
		byRoute[st.Route] = st
	}
	og, err := s.podcastStore()
	if err != nil {
		return 0, err
	}
	now := time.Now().UTC()
	rows := make([]podcastClipRow, 0, len(in))
	episodes := map[string]*PodcastEpisode{}
	for i, c := range in {
		st, ok := byRoute[strings.TrimSpace(c.StoryRoute)]
		if !ok {
			return 0, podcastInvalid("clip %d: story %q is not on the front list", i, truncatePodcast(c.StoryRoute, 80))
		}
		ep, seen := episodes[c.EpisodeID]
		if !seen {
			if ep, err = s.getPodcastEpisode(ctx, og, c.EpisodeID); err != nil {
				return 0, err
			}
			episodes[c.EpisodeID] = ep
		}
		if ep == nil {
			return 0, podcastInvalid("clip %d: episode %q is not known", i, truncatePodcast(c.EpisodeID, 80))
		}
		if err := finiteSeconds("start", c.Start); err != nil {
			return 0, err
		}
		if err := finiteSeconds("end", c.End); err != nil {
			return 0, err
		}
		if d := c.End - c.Start; d < podcastClipMinSeconds || d > podcastClipMaxSeconds {
			return 0, podcastInvalid("clip %d: %.1f seconds is outside %.0f to %.0f", i, d, podcastClipMinSeconds, podcastClipMaxSeconds)
		}
		if ep.DurationSeconds > 0 && c.End > ep.DurationSeconds+5 {
			return 0, podcastInvalid("clip %d: ends after the episode does", i)
		}
		if math.IsNaN(c.Support) || c.Support < 0 || c.Support > 1 {
			return 0, podcastInvalid("clip %d: support is outside [0, 1]", i)
		}
		quote := strings.Join(strings.Fields(c.Quote), " ")
		if quote == "" || !utf8.ValidString(quote) {
			return 0, podcastInvalid("clip %d: the quote is empty", i)
		}
		if utf8.RuneCountInString(quote) > podcastQuoteMaxRunes {
			quote = string([]rune(quote)[:podcastQuoteMaxRunes-1]) + "…"
		}
		rows = append(rows, podcastClipRow{
			ClipID: podcastClipID(st.Route, c.EpisodeID, c.Start), StoryRoute: st.Route, StoryVersion: st.Revision, StoryTitle: st.Title,
			EpisodeID: c.EpisodeID, Start: c.Start, End: c.End, Quote: quote, Support: c.Support, MatchedAt: now.Format(time.RFC3339),
		})
	}
	for _, row := range rows {
		obj, err := podcastObject(ogKindPodcastClip, row.ClipID, row, map[string]any{
			"story_route": row.StoryRoute, "episode_id": row.EpisodeID, "matched_at": row.MatchedAt,
		}, now)
		if err != nil {
			return 0, err
		}
		if err := og.PutObject(corpusdWriter(ctx), obj); err != nil {
			return 0, err
		}
	}
	s.podcastCache.reset()
	return len(rows), nil
}

func truncatePodcast(s string, n int) string {
	if len(s) <= n {
		return s
	}
	cut := n
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + "..."
}

// podcastReadCache holds the joined clip list for a short time, so the public
// route's cost is bounded whatever strangers ask.
type podcastReadCache struct {
	mu   sync.Mutex
	at   time.Time
	all  []PodcastClip
	good bool
}

func (c *podcastReadCache) reset() {
	c.mu.Lock()
	c.good = false
	c.mu.Unlock()
}

// allPodcastClips joins every stored clip with its episode, newest match
// first, best support first within a story.
func (s *Service) allPodcastClips(ctx context.Context) ([]PodcastClip, error) {
	c := &s.podcastCache
	c.mu.Lock()
	if c.good && time.Since(c.at) < podcastReadCacheTTL {
		out := c.all
		c.mu.Unlock()
		return out, nil
	}
	c.mu.Unlock()
	og, err := s.podcastStore()
	if err != nil {
		return nil, err
	}
	objs, err := og.ListObjects(ctx, objectgraph.ListFilter{Kind: ogKindPodcastClip, OwnerID: podcastOwner, Limit: podcastClipsScan})
	if err != nil {
		return nil, err
	}
	episodes := map[string]*PodcastEpisode{}
	type joined struct {
		clip    PodcastClip
		matched time.Time
		stable  bool
	}
	var rows []joined
	for _, o := range objs {
		if o.Tombstone {
			continue
		}
		var row podcastClipRow
		if json.Unmarshal(o.Body, &row) != nil {
			continue
		}
		ep, seen := episodes[row.EpisodeID]
		if !seen {
			if ep, err = s.getPodcastEpisode(ctx, og, row.EpisodeID); err != nil {
				return nil, err
			}
			episodes[row.EpisodeID] = ep
		}
		if ep == nil {
			continue
		}
		matched, _ := time.Parse(time.RFC3339, row.MatchedAt)
		rows = append(rows, joined{matched: matched, stable: ep.StableFile != nil && *ep.StableFile, clip: PodcastClip{
			ClipID: row.ClipID, Story: PodcastClipStory{Route: row.StoryRoute, Version: row.StoryVersion, Title: row.StoryTitle},
			Show: ep.Show, EpisodeTitle: ep.Title, EpisodeGUID: ep.GUID, FeedURL: ep.FeedURL, Published: ep.Published,
			AudioURL: ep.EnclosureURL, DurationSeconds: ep.DurationSeconds, Start: row.Start, End: row.End, Quote: row.Quote, Support: row.Support,
		}})
	}
	// Stories in the order of their newest match; within a story, clips of a
	// stable file first, then the best supported.
	newest := map[string]time.Time{}
	for _, r := range rows {
		if r.matched.After(newest[r.clip.Story.Route]) {
			newest[r.clip.Story.Route] = r.matched
		}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.clip.Story.Route != b.clip.Story.Route {
			na, nb := newest[a.clip.Story.Route], newest[b.clip.Story.Route]
			if !na.Equal(nb) {
				return na.After(nb)
			}
			return a.clip.Story.Route < b.clip.Story.Route
		}
		if a.stable != b.stable {
			return a.stable
		}
		return a.clip.Support > b.clip.Support
	})
	out := make([]PodcastClip, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.clip)
	}
	c.mu.Lock()
	c.all, c.at, c.good = out, time.Now(), true
	c.mu.Unlock()
	return out, nil
}

// ListPodcastClips is the public read (design 4.1). With routes it answers
// for those stories; without, for every clip matched within the window.
func (s *Service) ListPodcastClips(ctx context.Context, routes, feeds []string, limit int) (*PodcastClipsResponse, error) {
	if len(routes) > podcastClipsMaxRoutes || len(feeds) > podcastClipsMaxRoutes {
		return nil, podcastInvalid("at most %d routes and %d feeds", podcastClipsMaxRoutes, podcastClipsMaxRoutes)
	}
	preferred := map[string]bool{}
	for _, f := range feeds {
		if n, err := NormalizeFeedURL(f); err == nil {
			preferred[n] = true
		}
	}
	if limit <= 0 {
		limit = podcastClipsDefaultLimit
	}
	if limit > podcastClipsMaxLimit {
		limit = podcastClipsMaxLimit
	}
	all, err := s.allPodcastClips(ctx)
	if err != nil {
		return nil, err
	}
	want := map[string]bool{}
	for _, r := range routes {
		want[normalizePublicationRoutePath(r)] = true
	}
	perStory := map[string]int{}
	resp := &PodcastClipsResponse{GeneratedAt: time.Now().UTC(), Clips: []PodcastClip{}}
	var recent map[string]bool
	if len(want) == 0 {
		stories, err := s.recentPodcastStories(ctx, PodcastClipsDefaultWindow)
		if err != nil {
			return nil, err
		}
		recent = map[string]bool{}
		for _, st := range stories {
			recent[st.Route] = true
		}
	}
	for _, c := range all {
		if len(want) > 0 && !want[c.Story.Route] {
			continue
		}
		if recent != nil && !recent[c.Story.Route] {
			continue
		}
		if perStory[c.Story.Route] >= podcastClipsPerStory {
			continue
		}
		perStory[c.Story.Route]++
		resp.Clips = append(resp.Clips, c)
	}
	if len(preferred) > 0 {
		// The reader's own shows first, the order kept inside both groups.
		var mine, rest []PodcastClip
		for _, c := range resp.Clips {
			if n, err := NormalizeFeedURL(c.FeedURL); err == nil && preferred[n] {
				mine = append(mine, c)
			} else {
				rest = append(rest, c)
			}
		}
		resp.Clips = append(mine, rest...)
	}
	if len(resp.Clips) > limit {
		resp.Clips = resp.Clips[:limit]
	}
	return resp, nil
}
