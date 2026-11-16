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
	"log"
	"math"
	"net"
	"net/url"
	"sort"
	"strings"
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
	podcastEpisodeScan   = 5000
	podcastMaxFuture     = 24 * time.Hour
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
	FileSeconds float64 `json:"file_seconds,omitempty"`
	StableFile  *bool   `json:"stable_file,omitempty"`
	// Replace asks to overwrite a transcript that covers as much audio or
	// more; without it a transcript is never replaced by a shorter one.
	Replace bool `json:"replace,omitempty"`
	// Partial is set by the service, never believed from the request: the
	// transcript covers less of the file than the file holds (a worker that
	// transcribes only the first minutes). A partial transcript is stored and
	// its episode counts as transcribed; clips from it lie inside the part
	// that was covered, and a longer transcript replaces it.
	Partial  bool             `json:"partial,omitempty"`
	Segments []PodcastSegment `json:"segments"`
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
	if norm, err := NormalizeFeedURL(feedURL); err == nil {
		feedURL = norm // two spellings of one feed are one feed
	}
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

// checkPodcastURL admits an http or https URL with no credentials, on a named
// public-looking host: no IP literal, no localhost, no .local, .internal or
// .localhost name, no single-label name, and no port but 80 and 443. The
// service stores URLs and never fetches them, but the operator's worker does
// (review 1 of 2026-11-16, B1); the worker's dialer is the real guard and this
// is the first one.
func checkPodcastURL(name, raw string) error {
	if raw == "" || len(raw) > podcastMaxURLBytes {
		return podcastInvalid("%s is empty or over %d bytes", name, podcastMaxURLBytes)
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil {
		return podcastInvalid("%s is not an http or https URL without credentials", name)
	}
	host := strings.TrimRight(strings.ToLower(u.Hostname()), ".")
	if port := u.Port(); port != "" && port != "80" && port != "443" {
		return podcastInvalid("%s: only ports 80 and 443 are admitted", name)
	}
	if net.ParseIP(host) != nil {
		return podcastInvalid("%s: a host named by an address is not admitted", name)
	}
	if !strings.Contains(host, ".") || host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") ||
		strings.HasSuffix(host, ".internal") || strings.HasSuffix(host, ".localdomain") || strings.HasSuffix(host, ".home.arpa") {
		return podcastInvalid("%s: that host name is not admitted", name)
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
		// A repeat poll of an unchanged item writes nothing.
		if prev == nil || !sameEpisode(*prev, ep) {
			if err := s.putPodcastEpisode(ctx, og, ep, now); err != nil {
				return recorded, fresh, err
			}
		}
		recorded++
	}
	if err := s.notePodcastEpisodes(ctx, og, checked, now); err != nil {
		return recorded, fresh, err
	}
	s.podcastCache.reset()
	return recorded, fresh, nil
}

// sameEpisode reports whether two rows hold the same facts.
func sameEpisode(a, b PodcastEpisode) bool {
	return a.EpisodeID == b.EpisodeID && a.FeedURL == b.FeedURL && a.GUID == b.GUID && a.Show == b.Show && a.Title == b.Title &&
		a.Published.Equal(b.Published) && a.DurationSeconds == b.DurationSeconds && a.EnclosureURL == b.EnclosureURL && a.Notes == b.Notes &&
		a.HasTranscript == b.HasTranscript && ((a.StableFile == nil) == (b.StableFile == nil)) && (a.StableFile == nil || *a.StableFile == *b.StableFile)
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
	if ep.Published.After(time.Now().Add(podcastMaxFuture)) {
		return ep, podcastInvalid("published is more than %s in the future", podcastMaxFuture)
	}
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
	objs, err := og.ListObjects(ctx, objectgraph.ListFilter{Kind: ogKindPodcastEpisode, OwnerID: podcastOwner, Limit: podcastEpisodeScan})
	if err != nil {
		return nil, err
	}
	if len(objs) >= podcastEpisodeScan {
		// A bound that truncates is said, not silent (residual: no paging yet).
		log.Printf("corpusd: podcast episodes: the listing hit its scan bound of %d; older episodes are not seen", podcastEpisodeScan)
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
		// The worker's queue: what a signed-in phone is playing now, then the
		// operator's featured feeds, then the feeds most accounts follow, then
		// the rest; newest first within each (the sort is stable).
		if rank, err := s.podcastQueueRank(ctx); err == nil {
			sort.SliceStable(out, func(i, j int) bool { return rank(out[i]) < rank(out[j]) })
		}
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// podcastQueueRank ranks an episode for the worker: 0 playing now, 1 a
// featured feed, then 2 plus the inverse of the subscriber count.
func (s *Service) podcastQueueRank(ctx context.Context) (func(PodcastEpisode) int, error) {
	og, err := s.podcastStore()
	if err != nil {
		return nil, err
	}
	featured, err := s.featuredFeedSet(ctx, og)
	if err != nil {
		return nil, err
	}
	rows, err := s.allPodcastFeeds(ctx)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	subs := map[string]int{}
	wanted := map[string]map[string]bool{}
	for _, r := range rows {
		subs[r.Norm] = len(r.Subscribers)
		for _, w := range pruneWanted(r.Wanted, now) {
			if wanted[r.Norm] == nil {
				wanted[r.Norm] = map[string]bool{}
			}
			wanted[r.Norm][w.GUID] = true
		}
	}
	return func(e PodcastEpisode) int {
		n, _ := NormalizeFeedURL(e.FeedURL)
		switch {
		case wanted[n][e.GUID]:
			return 0
		case featured[n]:
			return 1
		}
		return 2 + podcastMaxSubscribers - subs[n]
	}, nil
}

// PutPodcastTranscript stores an episode's transcript and marks the episode.
func (s *Service) PutPodcastTranscript(ctx context.Context, t PodcastTranscript) (n int, kept bool, err error) {
	if t.EpisodeID == "" || len(t.Segments) == 0 || len(t.Segments) > podcastMaxSegments {
		return 0, false, podcastInvalid("a transcript needs an episode_id and 1 to %d segments", podcastMaxSegments)
	}
	engine, err := boundField("engine", t.Engine, 100)
	if err != nil {
		return 0, false, err
	}
	t.Engine = engine
	if err := finiteSeconds("audio_seconds", t.AudioSeconds); err != nil {
		return 0, false, err
	}
	if err := finiteSeconds("file_seconds", t.FileSeconds); err != nil {
		return 0, false, err
	}
	prevEnd, lastEnd := 0.0, 0.0
	for i := range t.Segments {
		seg := &t.Segments[i]
		if err := finiteSeconds("segment start", seg.Start); err != nil {
			return 0, false, err
		}
		if err := finiteSeconds("segment end", seg.End); err != nil {
			return 0, false, err
		}
		seg.Text = strings.TrimSpace(seg.Text)
		if seg.End <= seg.Start || seg.Start < prevEnd-0.5 || seg.Text == "" || len(seg.Text) > podcastMaxSegmentBytes || !utf8.ValidString(seg.Text) {
			return 0, false, podcastInvalid("segment %d is empty, over %d bytes, ends before it starts, or is out of order", i, podcastMaxSegmentBytes)
		}
		prevEnd, lastEnd = seg.End, seg.End
	}
	og, err := s.podcastStore()
	if err != nil {
		return 0, false, err
	}
	ep, err := s.getPodcastEpisode(ctx, og, t.EpisodeID)
	if err != nil {
		return 0, false, err
	}
	if ep == nil {
		return 0, false, podcastInvalid("episode %q is not known", t.EpisodeID)
	}
	// No segment may end after the file does (the feed's duration, or the one
	// measured on the download, whichever is longer, and a few seconds).
	if limit := math.Max(ep.DurationSeconds, t.FileSeconds); limit > 0 && lastEnd > limit+5 {
		return 0, false, podcastInvalid("a segment ends at %.0f s, after the episode's %.0f s", lastEnd, limit)
	}
	t.Partial = t.FileSeconds > 0 && t.AudioSeconds > 0 && t.AudioSeconds+5 < t.FileSeconds
	// A transcript is never replaced by one that covers no more audio.
	if existing, err := s.GetPodcastTranscript(ctx, og, t.EpisodeID); err != nil {
		return 0, false, err
	} else if existing != nil && !t.Replace && existing.AudioSeconds >= t.AudioSeconds {
		return len(existing.Segments), true, nil
	}
	now := time.Now().UTC()
	obj, err := podcastObject(ogKindPodcastTranscript, t.EpisodeID, t, map[string]any{
		"episode_id": t.EpisodeID, "engine": t.Engine, "segments": len(t.Segments), "partial": t.Partial,
	}, now)
	if err != nil {
		return 0, false, err
	}
	if err := og.PutObject(corpusdWriter(ctx), obj); err != nil {
		return 0, false, err
	}
	ep.HasTranscript = true
	if t.StableFile != nil {
		ep.StableFile = t.StableFile
	}
	switch {
	case t.FileSeconds > 0:
		ep.DurationSeconds = t.FileSeconds // what was downloaded beats what the feed said
	case ep.DurationSeconds == 0 && !t.Partial:
		ep.DurationSeconds = t.AudioSeconds
	}
	if err := s.putPodcastEpisode(ctx, og, *ep, now); err != nil {
		return 0, false, err
	}
	s.podcastCache.reset()
	return len(t.Segments), false, nil
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
	featured, err := s.featuredFeedSet(ctx, og)
	if err != nil {
		return 0, err
	}
	transcripts := map[string]*PodcastTranscript{}
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
		if n, nerr := NormalizeFeedURL(ep.FeedURL); nerr != nil || !featured[n] {
			return 0, podcastInvalid("clip %d: the episode's feed is not a featured feed", i)
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
		quote = CutQuote(quote, podcastQuoteMaxRunes)
		// The words must be in the transcript, inside the clip's times: the
		// operator route cannot publish words nobody said.
		tr, haveTr := transcripts[c.EpisodeID]
		if !haveTr {
			if tr, err = s.GetPodcastTranscript(ctx, og, c.EpisodeID); err != nil {
				return 0, err
			}
			transcripts[c.EpisodeID] = tr
		}
		if tr == nil || !quoteInTranscript(tr, c.Start, c.End, quote) {
			return 0, podcastInvalid("clip %d: the quote is not in the episode's transcript within the clip's times", i)
		}
		rows = append(rows, podcastClipRow{
			ClipID: podcastClipID(st.Route, c.EpisodeID, c.Start), StoryRoute: st.Route, StoryVersion: st.Revision, StoryTitle: st.Title,
			EpisodeID: c.EpisodeID, Start: c.Start, End: c.End, Quote: quote, Support: c.Support, MatchedAt: now.Format(time.RFC3339),
		})
	}
	// Two clips of one story, episode and second are one clip: the last wins.
	byID := map[string]int{}
	var distinct []podcastClipRow
	for _, row := range rows {
		if at, ok := byID[row.ClipID]; ok {
			distinct[at] = row
			continue
		}
		byID[row.ClipID] = len(distinct)
		distinct = append(distinct, row)
	}
	for _, row := range distinct {
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
	// Each story's clip index (read by key by the public read) keeps its 12 newest.
	clipsOfRoute := map[string][]string{}
	for _, row := range distinct {
		clipsOfRoute[row.StoryRoute] = append(clipsOfRoute[row.StoryRoute], row.ClipID)
	}
	s.podcastFeedMu.Lock()
	defer s.podcastFeedMu.Unlock()
	for route, ids := range clipsOfRoute {
		key := "clips:" + route
		idx, err := s.getPodcastIndex(ctx, og, key)
		if err != nil {
			return 0, err
		}
		have := podcastSet(idx.Clips)
		changed := false
		for _, id := range ids {
			if !have[id] {
				idx.Clips = append(idx.Clips, id)
				have[id] = true
				changed = true
			}
		}
		if len(idx.Clips) > podcastClipsPerStoryIndex {
			idx.Clips = idx.Clips[len(idx.Clips)-podcastClipsPerStoryIndex:]
		}
		if changed {
			if err := s.putPodcastIndex(ctx, og, key, idx, now); err != nil {
				return 0, err
			}
		}
	}
	s.podcastCache.reset()
	return len(distinct), nil
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

// CutQuote cuts a quote to at most max runes on a word boundary, never adding
// words: what is shown is a prefix of what was said.
func CutQuote(q string, max int) string {
	q = strings.Join(strings.Fields(q), " ")
	r := []rune(q)
	if len(r) <= max {
		return q
	}
	cut := max
	for cut > 0 && r[cut] != ' ' {
		cut--
	}
	if cut == 0 {
		cut = max
	}
	return strings.TrimSpace(string(r[:cut]))
}

// quoteInTranscript reports whether quote occurs, as words, in the segments of
// the transcript that overlap [start, end].
func quoteInTranscript(t *PodcastTranscript, start, end float64, quote string) bool {
	var parts []string
	for _, seg := range t.Segments {
		if seg.End > start-0.5 && seg.Start < end+0.5 {
			parts = append(parts, seg.Text)
		}
	}
	joined := strings.Join(strings.Fields(strings.Join(parts, " ")), " ")
	return strings.Contains(joined, strings.Join(strings.Fields(quote), " "))
}
