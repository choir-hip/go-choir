package platform

// The operator's feeds (docs/design/podcast-clips-2026-11-15.md section 3.2).
// A feed row exists only because the operator featured the feed (the seed list
// the worker publishes, or an operator mark), and "featured" is the only kind
// of feed there is: nothing an account sends creates or changes one. Feeds
// reported by accounts are Part 2 and not built (the design note lists what it
// must answer).
//
// Everything public is read by key: the featured set is one small index
// document, a story's clips another, and each feed's row remembers its newest
// episodes. No public behaviour depends on a page of "the newest N rows".

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

const (
	ogKindPodcastFeed  = objectgraph.ObjectKind("choir.podcast_feed")
	ogKindPodcastIndex = objectgraph.ObjectKind("choir.podcast_index")

	podcastMaxTitleBytes = 200
	// podcastRecentPerFeed is how many of a feed's newest episodes its row
	// remembers, for the match job to read by key.
	podcastRecentPerFeed = 40
	podcastPopularLimit  = 100
	// podcastMaxFeatured bounds the featured set: the operator's own list.
	podcastMaxFeatured = 1000
)

type podcastRecent struct {
	EpisodeID string    `json:"episode_id"`
	Published time.Time `json:"published"`
}

// podcastFeedRow is a stored, featured feed.
type podcastFeedRow struct {
	FeedURL string `json:"feed_url"`
	Norm    string `json:"norm"`
	// Title is what the operator called it; Show is what the worker parsed
	// from the feed, and is what is shown publicly.
	Title     string          `json:"title"`
	Show      string          `json:"show,omitempty"`
	FirstSeen string          `json:"first_seen"`
	Recent    []podcastRecent `json:"recent,omitempty"`
	// LatestTitle and LatestPublished are the feed's newest parsed episode.
	LatestTitle     string    `json:"latest_title,omitempty"`
	LatestPublished time.Time `json:"latest_published,omitempty"`
}

// podcastIndexDoc is one small index document of kind choir.podcast_index; the
// key says which field it uses.
type podcastIndexDoc struct {
	Featured []string `json:"featured,omitempty"` // "meta": normalised URLs of the featured feeds
	Clips    []string `json:"clips,omitempty"`    // "clips:<route>": clip ids of the story, oldest first
}

const podcastClipsPerStoryIndex = 12

func (s *Service) getPodcastIndex(ctx context.Context, og *ObjectGraphStore, key string) (podcastIndexDoc, error) {
	canonical, err := podcastCanonical(ogKindPodcastIndex, key)
	if err != nil {
		return podcastIndexDoc{}, err
	}
	obj, err := og.GetObject(ctx, canonical)
	if err != nil {
		if isNotFound(err) {
			return podcastIndexDoc{}, nil
		}
		return podcastIndexDoc{}, err
	}
	var d podcastIndexDoc
	if err := json.Unmarshal(obj.Body, &d); err != nil {
		return podcastIndexDoc{}, err
	}
	return d, nil
}

func (s *Service) putPodcastIndex(ctx context.Context, og *ObjectGraphStore, key string, d podcastIndexDoc, now time.Time) error {
	obj, err := podcastObject(ogKindPodcastIndex, key, d, map[string]any{"key": key}, now)
	if err != nil {
		return err
	}
	return og.PutObject(corpusdWriter(ctx), obj)
}

// NormalizeFeedURL makes two spellings of one feed one key: lower-case scheme
// and host, a trailing dot on the host, a default port, the fragment and a
// trailing slash dropped.
func NormalizeFeedURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if err := checkPodcastURL("feed_url", raw); err != nil {
		return "", err
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", podcastInvalid("feed_url is not a URL")
	}
	host := strings.TrimRight(strings.ToLower(u.Hostname()), ".")
	port := u.Port()
	u.Scheme, u.Fragment = strings.ToLower(u.Scheme), ""
	if (u.Scheme == "https" && port == "443") || (u.Scheme == "http" && port == "80") {
		port = ""
	}
	u.Host = host
	if port != "" {
		u.Host = host + ":" + port
	}
	if len(u.Path) > 1 {
		u.Path = strings.TrimRight(u.Path, "/")
	}
	return u.String(), nil
}

func podcastFeedID(norm string) string { return podcastHashID("feed-", norm) }

func (s *Service) getPodcastFeed(ctx context.Context, og *ObjectGraphStore, norm string) (*podcastFeedRow, error) {
	canonical, err := podcastCanonical(ogKindPodcastFeed, podcastFeedID(norm))
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
	var row podcastFeedRow
	if err := json.Unmarshal(obj.Body, &row); err != nil {
		return nil, err
	}
	return &row, nil
}

func (s *Service) putPodcastFeed(ctx context.Context, og *ObjectGraphStore, row podcastFeedRow, now time.Time) error {
	obj, err := podcastObject(ogKindPodcastFeed, podcastFeedID(row.Norm), row, map[string]any{"norm": row.Norm}, now)
	if err != nil {
		return err
	}
	return og.PutObject(corpusdWriter(ctx), obj)
}

func podcastSet(list []string) map[string]bool {
	out := make(map[string]bool, len(list))
	for _, x := range list {
		out[x] = true
	}
	return out
}

var podcastEmptyIndexLogged sync.Once

// featuredFeedSet is the normalised URLs of the featured feeds, read by key. An
// empty index while feed rows exist is a lost document, not an empty list: it
// fails closed (nothing is public) and says so, once per process.
func (s *Service) featuredFeedSet(ctx context.Context, og *ObjectGraphStore) (map[string]bool, error) {
	meta, err := s.getPodcastIndex(ctx, og, "meta")
	if err != nil {
		return nil, err
	}
	if len(meta.Featured) == 0 {
		if rows, err := og.ListObjects(ctx, objectgraph.ListFilter{Kind: ogKindPodcastFeed, OwnerID: podcastOwner, Limit: 1}); err == nil && len(rows) > 0 {
			podcastEmptyIndexLogged.Do(func() {
				log.Printf("corpusd: podcast feeds: feed rows exist but the featured index is empty or missing; nothing is public until the operator lists the feeds again")
			})
		}
	}
	return podcastSet(meta.Featured), nil
}

// PodcastFeedListing is one feed the operator features.
type PodcastFeedListing struct {
	FeedURL string `json:"feed_url"`
	Title   string `json:"title"`
}

// SetPodcastFeedsListed features (or unfeatures) feeds as the operator's own:
// the seed list, or a feed the operator vouches for. An unchanged feed writes
// nothing. This is the only way a feed row comes to exist.
func (s *Service) SetPodcastFeedsListed(ctx context.Context, feeds []PodcastFeedListing, listed bool) (int, error) {
	if len(feeds) == 0 || len(feeds) > podcastMaxEpisodesPerRequest {
		return 0, podcastInvalid("a request lists 1 to %d feeds", podcastMaxEpisodesPerRequest)
	}
	type checked struct{ url, norm, title string }
	var list []checked
	for i, f := range feeds {
		norm, err := NormalizeFeedURL(f.FeedURL)
		if err != nil {
			return 0, fmt.Errorf("feed %d: %w", i, err)
		}
		title, err := boundField("title", f.Title, podcastMaxTitleBytes)
		if err != nil {
			return 0, fmt.Errorf("feed %d: %w", i, err)
		}
		list = append(list, checked{url: strings.TrimSpace(f.FeedURL), norm: norm, title: title})
	}
	og, err := s.podcastStore()
	if err != nil {
		return 0, err
	}
	s.podcastFeedMu.Lock()
	defer s.podcastFeedMu.Unlock()
	now := time.Now().UTC()
	meta, err := s.getPodcastIndex(ctx, og, "meta")
	if err != nil {
		return 0, err
	}
	featured := podcastSet(meta.Featured)
	metaDirty := false
	for _, f := range list {
		row, err := s.getPodcastFeed(ctx, og, f.norm)
		if err != nil {
			return 0, err
		}
		if row == nil && listed {
			row = &podcastFeedRow{FeedURL: f.url, Norm: f.norm, Title: f.title, FirstSeen: now.Format(time.RFC3339)}
			if err := s.putPodcastFeed(ctx, og, *row, now); err != nil {
				return 0, err
			}
		} else if row != nil && f.title != "" && row.Title != f.title {
			row.Title = f.title
			if err := s.putPodcastFeed(ctx, og, *row, now); err != nil {
				return 0, err
			}
		}
		if listed != featured[f.norm] {
			metaDirty = true
			if listed {
				featured[f.norm] = true
			} else {
				delete(featured, f.norm)
			}
		}
	}
	if len(featured) > podcastMaxFeatured {
		return 0, podcastInvalid("at most %d featured feeds", podcastMaxFeatured)
	}
	if metaDirty {
		meta.Featured = meta.Featured[:0]
		for n := range featured {
			meta.Featured = append(meta.Featured, n)
		}
		sort.Strings(meta.Featured)
		if err := s.putPodcastIndex(ctx, og, "meta", meta, now); err != nil {
			return 0, err
		}
		s.podcastCache.reset()
	}
	return len(list), nil
}

// notePodcastEpisodes tells the feed rows what the worker parsed: the show's
// name and the newest episodes, which the match job reads by key. Only a
// featured feed has a row; the caller has already skipped the rest.
func (s *Service) notePodcastEpisodes(ctx context.Context, og *ObjectGraphStore, eps []PodcastEpisode, now time.Time) error {
	byFeed := map[string][]PodcastEpisode{}
	for _, e := range eps {
		if n, err := NormalizeFeedURL(e.FeedURL); err == nil {
			byFeed[n] = append(byFeed[n], e)
		}
	}
	s.podcastFeedMu.Lock()
	defer s.podcastFeedMu.Unlock()
	for norm, list := range byFeed {
		row, err := s.getPodcastFeed(ctx, og, norm)
		if err != nil {
			return err
		}
		if row == nil {
			continue
		}
		before, _ := json.Marshal(row)
		recent := map[string]time.Time{}
		for _, r := range row.Recent {
			recent[r.EpisodeID] = r.Published
		}
		for _, e := range list {
			recent[e.EpisodeID] = e.Published
			row.Show = e.Show
			if e.Published.After(row.LatestPublished) {
				row.LatestPublished, row.LatestTitle = e.Published, e.Title
			}
		}
		row.Recent = row.Recent[:0]
		for id, p := range recent {
			row.Recent = append(row.Recent, podcastRecent{EpisodeID: id, Published: p})
		}
		sort.Slice(row.Recent, func(i, j int) bool {
			if !row.Recent[i].Published.Equal(row.Recent[j].Published) {
				return row.Recent[i].Published.After(row.Recent[j].Published)
			}
			return row.Recent[i].EpisodeID < row.Recent[j].EpisodeID
		})
		if len(row.Recent) > podcastRecentPerFeed {
			row.Recent = row.Recent[:podcastRecentPerFeed]
		}
		after, _ := json.Marshal(row)
		if string(before) == string(after) {
			continue
		}
		if err := s.putPodcastFeed(ctx, og, *row, now); err != nil {
			return err
		}
	}
	return nil
}

// PodcastPopularFeed is one entry of the popular list. The shape is the
// phone's decoder's; subscribers is always 0 until feeds reported by accounts
// exist (design Part 2).
type PodcastPopularFeed struct {
	FeedURL            string    `json:"feed_url"`
	Title              string    `json:"title"`
	Subscribers        int       `json:"subscribers"`
	LatestEpisodeTitle string    `json:"latest_episode_title,omitempty"`
	LatestPublished    time.Time `json:"latest_published,omitempty"`
}

// ListPodcastPopular lists the featured feeds the worker has fetched, the one
// with the newest episode first. The title is the show's name as the worker
// parsed it. Read by key: the featured set, then each feed's row.
func (s *Service) ListPodcastPopular(ctx context.Context, limit int) ([]PodcastPopularFeed, error) {
	if limit <= 0 {
		limit = 30
	}
	if limit > podcastPopularLimit {
		limit = podcastPopularLimit
	}
	og, err := s.podcastStore()
	if err != nil {
		return nil, err
	}
	meta, err := s.getPodcastIndex(ctx, og, "meta")
	if err != nil {
		return nil, err
	}
	var out []PodcastPopularFeed
	for _, norm := range meta.Featured {
		row, err := s.getPodcastFeed(ctx, og, norm)
		if err != nil {
			return nil, err
		}
		if row == nil || row.LatestTitle == "" {
			continue // not fetched and parsed yet
		}
		item := PodcastPopularFeed{FeedURL: row.FeedURL, Title: row.Show, LatestEpisodeTitle: row.LatestTitle, LatestPublished: row.LatestPublished}
		if item.Title == "" {
			item.Title = row.Title
		}
		out = append(out, item)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].LatestPublished.After(out[j].LatestPublished) })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// PodcastTranscriptResponse is the reader's transcript (design 4.7).
type PodcastTranscriptResponse struct {
	Episode  PodcastTranscriptEpisode `json:"episode"`
	Source   string                   `json:"source"`
	Verified bool                     `json:"verified"`
	Blocks   []PodcastSegment         `json:"blocks"`
}

// PodcastTranscriptEpisode names the episode a transcript is of.
type PodcastTranscriptEpisode struct {
	Show            string    `json:"show"`
	Title           string    `json:"title"`
	Published       time.Time `json:"published"`
	FeedURL         string    `json:"feed_url"`
	GUID            string    `json:"guid"`
	AudioURL        string    `json:"audio_url"`
	DurationSeconds float64   `json:"duration_seconds"`
}

// GetPodcastTranscriptFor reads an episode's transcript by feed and guid; nil
// when there is none or the feed is not featured (404 either way, so a feed's
// existence is not revealed).
func (s *Service) GetPodcastTranscriptFor(ctx context.Context, feedURL, guid string) (*PodcastTranscriptResponse, error) {
	feedURL, guid = strings.TrimSpace(feedURL), strings.TrimSpace(guid)
	if err := checkPodcastURL("feed", feedURL); err != nil || guid == "" || len(guid) > podcastMaxURLBytes {
		return nil, podcastInvalid("feed and guid are required")
	}
	og, err := s.podcastStore()
	if err != nil {
		return nil, err
	}
	ep, err := s.getPodcastEpisode(ctx, og, PodcastEpisodeID(feedURL, guid))
	if err != nil || ep == nil {
		return nil, err
	}
	norm, err := NormalizeFeedURL(ep.FeedURL)
	if err != nil {
		return nil, nil
	}
	featured, err := s.featuredFeedSet(ctx, og)
	if err != nil {
		return nil, err
	}
	if !featured[norm] {
		return nil, nil
	}
	tr, err := s.GetPodcastTranscript(ctx, og, ep.EpisodeID)
	if err != nil || tr == nil {
		return nil, err
	}
	return &PodcastTranscriptResponse{
		Episode: PodcastTranscriptEpisode{Show: ep.Show, Title: ep.Title, Published: ep.Published, FeedURL: ep.FeedURL, GUID: ep.GUID, AudioURL: ep.EnclosureURL, DurationSeconds: ep.DurationSeconds},
		Source:  tr.Engine, Verified: true, Blocks: tr.Segments,
	}, nil
}
