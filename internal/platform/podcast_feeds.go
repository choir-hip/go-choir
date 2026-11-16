package platform

// The feeds readers subscribe to (docs/design/podcast-clips-2026-11-15.md
// section 3.2): a phone reports the feeds its reader follows and the episode
// it is playing; the worker transcribes the union of those feeds and the seed
// list, most subscribed first, and now-playing first of all. A transcript is
// stored once per episode and shared by everyone. A feed row keeps a count of
// distinct reporting accounts as salted hashes, never an identity, and the
// popular list never says who subscribes to what.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

const (
	ogKindPodcastFeed = objectgraph.ObjectKind("choir.podcast_feed")

	podcastMaxFeedsPerReport = 50
	podcastMaxNewFeedsPerDay = 200
	podcastMaxTitleBytes     = 200
	podcastMaxSubscribers    = 20000
	podcastMaxWanted         = 20
	podcastWantedTTL         = 24 * time.Hour
	// podcastPopularMin is how many distinct accounts must report a feed
	// before it is listed with a count (k-anonymity).
	podcastPopularMin   = 3
	podcastPopularLimit = 100
	podcastFeedScan     = 5000
)

// PodcastFeedReport is one reported subscription.
type PodcastFeedReport struct {
	FeedURL string `json:"feed_url"`
	Title   string `json:"title"`
}

// PodcastNowPlaying is the episode a phone is playing.
type PodcastNowPlaying struct {
	FeedURL     string `json:"feed_url"`
	EpisodeGUID string `json:"episode_guid"`
	AudioURL    string `json:"audio_url"`
}

// PodcastFeedsRequest is the phone's report.
type PodcastFeedsRequest struct {
	Feeds      []PodcastFeedReport `json:"feeds"`
	NowPlaying *PodcastNowPlaying  `json:"now_playing,omitempty"`
}

// PodcastFeedsResponse is the answer to a report.
type PodcastFeedsResponse struct {
	Accepted         int  `json:"accepted"`
	NewFeeds         int  `json:"new_feeds"`
	NowPlayingQueued bool `json:"now_playing_queued"`
}

type podcastWanted struct {
	GUID     string `json:"guid"`
	AudioURL string `json:"audio_url"`
	At       string `json:"at"`
}

// podcastFeedRow is a stored feed.
type podcastFeedRow struct {
	FeedURL string `json:"feed_url"`
	Norm    string `json:"norm"`
	Title   string `json:"title"`
	// Featured marks a feed the operator's seed list brought in.
	Featured    bool            `json:"featured,omitempty"`
	FirstSeen   string          `json:"first_seen"`
	Subscribers []string        `json:"subscribers,omitempty"` // salted hashes, never shown
	Wanted      []podcastWanted `json:"wanted,omitempty"`
}

// NormalizeFeedURL makes two spellings of one feed one key: lower-case scheme
// and host, default port and fragment dropped, trailing slash trimmed.
func NormalizeFeedURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if err := checkPodcastURL("feed_url", raw); err != nil {
		return "", err
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", podcastInvalid("feed_url is not a URL")
	}
	u.Scheme, u.Host, u.Fragment = strings.ToLower(u.Scheme), strings.ToLower(u.Host), ""
	if (u.Scheme == "https" && strings.HasSuffix(u.Host, ":443")) || (u.Scheme == "http" && strings.HasSuffix(u.Host, ":80")) {
		u.Host = u.Host[:strings.LastIndex(u.Host, ":")]
	}
	if len(u.Path) > 1 {
		u.Path = strings.TrimRight(u.Path, "/")
	}
	return u.String(), nil
}

func podcastFeedID(norm string) string { return podcastHashID("feed-", norm) }

// podcastSubscriberHash is what is kept of an account: a salted hash.
func podcastSubscriberHash(accountID string) string {
	return podcastHashID("sub-", "podcast-subscriber\x00"+accountID)
}

// PodcastReporter is the salted hash of an account, for the proxy to pass.
func PodcastReporter(accountID string) string {
	if strings.TrimSpace(accountID) == "" {
		return ""
	}
	return podcastSubscriberHash(accountID)
}

type podcastDayCounter struct {
	mu  sync.Mutex
	day string
	n   int
}

func (c *podcastDayCounter) take() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	today := time.Now().UTC().Format("2006-01-02")
	if c.day != today {
		c.day, c.n = today, 0
	}
	if c.n >= podcastMaxNewFeedsPerDay {
		return false
	}
	c.n++
	return true
}

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
	obj, err := podcastObject(ogKindPodcastFeed, podcastFeedID(row.Norm), row, map[string]any{
		"norm": row.Norm, "subscribers": len(row.Subscribers), "featured": row.Featured,
	}, now)
	if err != nil {
		return err
	}
	return og.PutObject(corpusdWriter(ctx), obj)
}

// ReportPodcastFeeds takes a phone's report. reporter is the salted hash of a
// signed-in account, or "" for an anonymous report, which adds feeds to
// intake and counts toward nothing.
func (s *Service) ReportPodcastFeeds(ctx context.Context, reporter string, req PodcastFeedsRequest) (*PodcastFeedsResponse, error) {
	if len(req.Feeds) > podcastMaxFeedsPerReport {
		return nil, podcastInvalid("at most %d feeds", podcastMaxFeedsPerReport)
	}
	if len(req.Feeds) == 0 && req.NowPlaying == nil {
		return nil, podcastInvalid("nothing reported")
	}
	type checked struct{ url, norm, title string }
	var feeds []checked
	seen := map[string]bool{}
	for i, f := range req.Feeds {
		norm, err := NormalizeFeedURL(f.FeedURL)
		if err != nil {
			return nil, fmt.Errorf("feed %d: %w", i, err)
		}
		title, err := boundField("title", f.Title, podcastMaxTitleBytes)
		if err != nil {
			return nil, fmt.Errorf("feed %d: %w", i, err)
		}
		if !seen[norm] {
			seen[norm] = true
			feeds = append(feeds, checked{url: strings.TrimSpace(f.FeedURL), norm: norm, title: title})
		}
	}
	var np *PodcastNowPlaying
	var npNorm string
	if req.NowPlaying != nil {
		var err error
		if npNorm, err = NormalizeFeedURL(req.NowPlaying.FeedURL); err != nil {
			return nil, fmt.Errorf("now_playing: %w", err)
		}
		guid, err := boundField("episode_guid", req.NowPlaying.EpisodeGUID, podcastMaxURLBytes)
		if err != nil || guid == "" {
			return nil, podcastInvalid("now_playing needs an episode_guid")
		}
		if err := checkPodcastURL("audio_url", strings.TrimSpace(req.NowPlaying.AudioURL)); err != nil {
			return nil, fmt.Errorf("now_playing: %w", err)
		}
		np = &PodcastNowPlaying{FeedURL: npNorm, EpisodeGUID: guid, AudioURL: strings.TrimSpace(req.NowPlaying.AudioURL)}
		if !seen[npNorm] {
			seen[npNorm] = true
			feeds = append(feeds, checked{url: strings.TrimSpace(req.NowPlaying.FeedURL), norm: npNorm})
		}
	}
	og, err := s.podcastStore()
	if err != nil {
		return nil, err
	}
	s.podcastFeedMu.Lock()
	defer s.podcastFeedMu.Unlock()
	now := time.Now().UTC()
	resp := &PodcastFeedsResponse{}
	for _, f := range feeds {
		row, err := s.getPodcastFeed(ctx, og, f.norm)
		if err != nil {
			return resp, err
		}
		if row == nil {
			if !s.podcastNewFeeds.take() {
				continue // a bound on what strangers can cause: the rest of the day's new feeds are ignored
			}
			row = &podcastFeedRow{FeedURL: f.url, Norm: f.norm, Title: f.title, FirstSeen: now.Format(time.RFC3339)}
			resp.NewFeeds++
		}
		if row.Title == "" && f.title != "" {
			row.Title = f.title
		}
		isReport := false
		for _, rf := range req.Feeds {
			if n, err := NormalizeFeedURL(rf.FeedURL); err == nil && n == f.norm {
				isReport = true
				break
			}
		}
		if reporter != "" && isReport && len(row.Subscribers) < podcastMaxSubscribers {
			found := false
			for _, h := range row.Subscribers {
				if h == reporter {
					found = true
					break
				}
			}
			if !found {
				row.Subscribers = append(row.Subscribers, reporter)
			}
		}
		if np != nil && f.norm == np.FeedURL {
			row.Wanted = pruneWanted(row.Wanted, now)
			dup := false
			for i := range row.Wanted {
				if row.Wanted[i].GUID == np.EpisodeGUID {
					row.Wanted[i].At, dup = now.Format(time.RFC3339), true
				}
			}
			if !dup {
				row.Wanted = append(row.Wanted, podcastWanted{GUID: np.EpisodeGUID, AudioURL: np.AudioURL, At: now.Format(time.RFC3339)})
				if len(row.Wanted) > podcastMaxWanted {
					row.Wanted = row.Wanted[len(row.Wanted)-podcastMaxWanted:]
				}
			}
			resp.NowPlayingQueued = true
		}
		if err := s.putPodcastFeed(ctx, og, *row, now); err != nil {
			return resp, err
		}
		if isReport {
			resp.Accepted++
		}
	}
	s.podcastCache.reset()
	return resp, nil
}

func pruneWanted(w []podcastWanted, now time.Time) []podcastWanted {
	var out []podcastWanted
	for _, x := range w {
		if t, err := time.Parse(time.RFC3339, x.At); err == nil && now.Sub(t) < podcastWantedTTL {
			out = append(out, x)
		}
	}
	return out
}

// notePodcastSeedFeed records a feed the operator's intake brought in, so
// the popular list can feature it. It never removes subscriber counts.
func (s *Service) notePodcastSeedFeed(ctx context.Context, og *ObjectGraphStore, feedURL, title string, now time.Time) error {
	norm, err := NormalizeFeedURL(feedURL)
	if err != nil {
		return nil
	}
	s.podcastFeedMu.Lock()
	defer s.podcastFeedMu.Unlock()
	row, err := s.getPodcastFeed(ctx, og, norm)
	if err != nil {
		return err
	}
	if row == nil {
		row = &podcastFeedRow{FeedURL: feedURL, Norm: norm, Title: title, FirstSeen: now.Format(time.RFC3339), Featured: true}
	} else if row.Featured && (row.Title != "" || title == "") {
		return nil
	} else if row.Title == "" {
		row.Title = title
	}
	return s.putPodcastFeed(ctx, og, *row, now)
}

// PodcastFeedForWorker is a feed as the worker's intake sees it.
type PodcastFeedForWorker struct {
	FeedURL     string          `json:"feed_url"`
	Title       string          `json:"title"`
	Subscribers int             `json:"subscribers"`
	Wanted      []podcastWanted `json:"wanted,omitempty"`
}

func (s *Service) allPodcastFeeds(ctx context.Context) ([]podcastFeedRow, error) {
	og, err := s.podcastStore()
	if err != nil {
		return nil, err
	}
	objs, err := og.ListObjects(ctx, objectgraph.ListFilter{Kind: ogKindPodcastFeed, OwnerID: podcastOwner, Limit: podcastFeedScan})
	if err != nil {
		return nil, err
	}
	var out []podcastFeedRow
	for _, o := range objs {
		var row podcastFeedRow
		if !o.Tombstone && json.Unmarshal(o.Body, &row) == nil {
			out = append(out, row)
		}
	}
	return out, nil
}

// ListPodcastFeedsForWorker lists every feed with a subscriber, a seed mark or
// a wanted episode, most subscribed first.
func (s *Service) ListPodcastFeedsForWorker(ctx context.Context) ([]PodcastFeedForWorker, error) {
	rows, err := s.allPodcastFeeds(ctx)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	out := make([]PodcastFeedForWorker, 0, len(rows))
	for _, r := range rows {
		out = append(out, PodcastFeedForWorker{FeedURL: r.FeedURL, Title: r.Title, Subscribers: len(r.Subscribers), Wanted: pruneWanted(r.Wanted, now)})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Subscribers > out[j].Subscribers })
	return out, nil
}

// PodcastPopularFeed is one entry of the popular list.
type PodcastPopularFeed struct {
	FeedURL            string    `json:"feed_url"`
	Title              string    `json:"title"`
	Subscribers        int       `json:"subscribers"`
	LatestEpisodeTitle string    `json:"latest_episode_title,omitempty"`
	LatestPublished    time.Time `json:"latest_published,omitempty"`
}

// ListPodcastPopular lists the feeds readers follow, with counts and never
// who: a feed shows a count only when at least podcastPopularMin distinct
// accounts report it; the operator's featured feeds follow with a zero.
func (s *Service) ListPodcastPopular(ctx context.Context, limit int) ([]PodcastPopularFeed, error) {
	if limit <= 0 {
		limit = 30
	}
	if limit > podcastPopularLimit {
		limit = podcastPopularLimit
	}
	rows, err := s.allPodcastFeeds(ctx)
	if err != nil {
		return nil, err
	}
	eps, err := s.ListPodcastEpisodes(ctx, false, time.Time{}, 500)
	if err != nil {
		return nil, err
	}
	latest := map[string]PodcastEpisode{}
	for _, e := range eps {
		if n, err := NormalizeFeedURL(e.FeedURL); err == nil {
			if cur, ok := latest[n]; !ok || e.Published.After(cur.Published) {
				latest[n] = e
			}
		}
	}
	var counted, featured []PodcastPopularFeed
	for _, r := range rows {
		item := PodcastPopularFeed{FeedURL: r.FeedURL, Title: r.Title}
		if e, ok := latest[r.Norm]; ok {
			item.LatestEpisodeTitle, item.LatestPublished = e.Title, e.Published
			if item.Title == "" {
				item.Title = e.Show
			}
		}
		switch {
		case len(r.Subscribers) >= podcastPopularMin:
			item.Subscribers = len(r.Subscribers)
			counted = append(counted, item)
		case r.Featured:
			featured = append(featured, item)
		}
	}
	sort.SliceStable(counted, func(i, j int) bool { return counted[i].Subscribers > counted[j].Subscribers })
	sort.SliceStable(featured, func(i, j int) bool { return featured[i].Title < featured[j].Title })
	out := append(counted, featured...)
	if len(out) > limit {
		out = out[:limit]
	}
	for i := range out {
		if utf8.RuneCountInString(out[i].Title) > 200 {
			out[i].Title = string([]rune(out[i].Title)[:200])
		}
	}
	return out, nil
}

// subscriberCounts maps normalised feed URLs to their subscriber counts, for
// ordering the worker's queue.
func (s *Service) podcastFeedWeights(ctx context.Context) (counts map[string]int, wanted map[string]map[string]bool, err error) {
	rows, err := s.allPodcastFeeds(ctx)
	if err != nil {
		return nil, nil, err
	}
	now := time.Now().UTC()
	counts, wanted = map[string]int{}, map[string]map[string]bool{}
	for _, r := range rows {
		counts[r.Norm] = len(r.Subscribers)
		for _, w := range pruneWanted(r.Wanted, now) {
			if wanted[r.Norm] == nil {
				wanted[r.Norm] = map[string]bool{}
			}
			wanted[r.Norm][w.GUID] = true
		}
	}
	return counts, wanted, nil
}

// PodcastTranscriptResponse is the reader's transcript (design 4.4).
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
// when there is none.
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
	if err != nil {
		return nil, err
	}
	if ep == nil {
		// The phone may spell the feed another way than the worker did.
		norm, nerr := NormalizeFeedURL(feedURL)
		if nerr != nil {
			return nil, nil
		}
		all, err := s.ListPodcastEpisodes(ctx, false, time.Time{}, podcastMaxEpisodeListing)
		if err != nil {
			return nil, err
		}
		for i := range all {
			if n, err := NormalizeFeedURL(all[i].FeedURL); err == nil && n == norm && all[i].GUID == guid {
				ep = &all[i]
				break
			}
		}
		if ep == nil {
			return nil, nil
		}
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
