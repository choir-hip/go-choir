package platform

// The feeds readers subscribe to (docs/design/podcast-clips-2026-11-15.md
// sections 3.2 and 3.4): a signed-in phone reports the feeds its reader follows
// and the episode it plays; the worker transcribes the featured feeds first and
// then the subscribed ones, and now-playing before all. A transcript is stored
// once per episode. A feed is PUBLIC (its clips, its popularity, its transcript)
// only when the operator features it (the seed list, or an operator mark): a
// feed an account reports stays that account's own.
//
// Everything public is read by key: the featured set and the platform's row
// count live in one small index document, an account's subscriptions in
// another, a story's clips in a third. No public behaviour depends on a page of
// "the newest N rows".

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

const (
	ogKindPodcastFeed  = objectgraph.ObjectKind("choir.podcast_feed")
	ogKindPodcastIndex = objectgraph.ObjectKind("choir.podcast_index")

	podcastMaxFeedsPerReport          = 50
	podcastMaxSubscriptionsPerAccount = 500
	podcastMaxTitleBytes              = 200
	podcastMaxSubscribers             = 20000
	podcastMaxWanted                  = 20
	podcastWantedTTL                  = 24 * time.Hour
	// podcastRecentPerFeed is how many of a feed's newest episodes its row
	// remembers, for the match job to read by key.
	podcastRecentPerFeed = 40
	// podcastPopularMin is how many distinct accounts must report a featured
	// feed before its count is shown (k-anonymity).
	podcastPopularMin   = 3
	podcastPopularLimit = 100
)

// podcastMaxFeedRows is the most feed rows the platform holds: new rows beyond
// it are refused and logged, so no scan of them can truncate. A variable so a
// test can lower it.
var podcastMaxFeedRows = 20000

var errPodcastRowLimit = errors.New("the platform holds its limit of feeds")

// ErrPodcastSignInRequired marks a report with no signed-in account behind it.
var ErrPodcastSignInRequired = errors.New("signing in is required to report subscriptions")

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

type podcastRecent struct {
	EpisodeID string    `json:"episode_id"`
	Published time.Time `json:"published"`
}

// podcastFeedRow is a stored feed.
type podcastFeedRow struct {
	FeedURL string `json:"feed_url"`
	Norm    string `json:"norm"`
	// Title is what a reporter or the operator called it; Show is what the
	// worker parsed from the feed, and is what is shown publicly.
	Title       string          `json:"title"`
	Show        string          `json:"show,omitempty"`
	FirstSeen   string          `json:"first_seen"`
	Subscribers []string        `json:"subscribers,omitempty"` // hashes of accounts, never shown
	Wanted      []podcastWanted `json:"wanted,omitempty"`
	Recent      []podcastRecent `json:"recent,omitempty"`
	// LatestTitle and LatestPublished are the feed's newest parsed episode.
	LatestTitle     string    `json:"latest_title,omitempty"`
	LatestPublished time.Time `json:"latest_published,omitempty"`
}

// podcastIndexDoc is one small index document of kind choir.podcast_index; the
// key says which fields it uses.
type podcastIndexDoc struct {
	Featured []string `json:"featured,omitempty"` // "meta": normalised URLs of featured feeds
	Rows     int      `json:"rows,omitempty"`     // "meta": how many feed rows exist
	Subs     []string `json:"subs,omitempty"`     // "acct:<hash>": normalised URLs the account subscribes to
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
// and host, default port, fragment and trailing slash dropped.
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

// podcastSubscriberHash is what is kept of an account: sha256 of a fixed
// context and the account id. It keeps the id out of the store and is not a
// secret (it is neither salted nor keyed).
func podcastSubscriberHash(accountID string) string {
	return podcastHashID("sub-", "podcast-subscriber\x00"+accountID)
}

// PodcastReporter is the hash of an account, for the proxy to pass.
func PodcastReporter(accountID string) string {
	if strings.TrimSpace(accountID) == "" {
		return ""
	}
	return podcastSubscriberHash(accountID)
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
		"norm": row.Norm, "subscribers": len(row.Subscribers),
	}, now)
	if err != nil {
		return err
	}
	return og.PutObject(corpusdWriter(ctx), obj)
}

func podcastContains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func podcastSet(list []string) map[string]bool {
	out := make(map[string]bool, len(list))
	for _, x := range list {
		out[x] = true
	}
	return out
}

// featuredFeedSet is the normalised URLs of the featured feeds, read by key.
func (s *Service) featuredFeedSet(ctx context.Context, og *ObjectGraphStore) (map[string]bool, error) {
	meta, err := s.getPodcastIndex(ctx, og, "meta")
	if err != nil {
		return nil, err
	}
	return podcastSet(meta.Featured), nil
}

// ReportPodcastFeeds takes a phone's report from a signed-in account (reporter
// is the hash of the account; empty is refused). The account subscribes to the
// feeds it names, within its cap; a playing episode is marked only for a feed
// the account subscribes to. A report that changes nothing writes nothing.
func (s *Service) ReportPodcastFeeds(ctx context.Context, reporter string, req PodcastFeedsRequest) (*PodcastFeedsResponse, error) {
	if reporter == "" {
		return nil, ErrPodcastSignInRequired
	}
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
	}
	og, err := s.podcastStore()
	if err != nil {
		return nil, err
	}
	s.podcastFeedMu.Lock()
	defer s.podcastFeedMu.Unlock()
	now := time.Now().UTC()
	acctKey := "acct:" + reporter
	acct, err := s.getPodcastIndex(ctx, og, acctKey)
	if err != nil {
		return nil, err
	}
	have := podcastSet(acct.Subs)
	var added []string
	for _, f := range feeds {
		if !have[f.norm] {
			added = append(added, f.norm)
		}
	}
	// The bound on what one account can add: a stored cap of subscriptions.
	if len(acct.Subs)+len(added) > podcastMaxSubscriptionsPerAccount {
		return nil, podcastInvalid("an account holds at most %d subscriptions", podcastMaxSubscriptionsPerAccount)
	}
	meta, err := s.getPodcastIndex(ctx, og, "meta")
	if err != nil {
		return nil, err
	}
	resp := &PodcastFeedsResponse{}
	metaDirty := false
	changed := false
	for _, f := range feeds {
		row, err := s.getPodcastFeed(ctx, og, f.norm)
		if err != nil {
			return resp, err
		}
		dirty := false
		if row == nil {
			if meta.Rows >= podcastMaxFeedRows {
				log.Printf("corpusd: podcast feeds: the %d-row limit is reached; a new feed is refused", podcastMaxFeedRows)
				return resp, errPodcastRowLimit
			}
			row = &podcastFeedRow{FeedURL: f.url, Norm: f.norm, Title: f.title, FirstSeen: now.Format(time.RFC3339)}
			meta.Rows++
			metaDirty, dirty = true, true
			resp.NewFeeds++
		}
		if row.Title == "" && f.title != "" {
			row.Title, dirty = f.title, true
		}
		if !podcastContains(row.Subscribers, reporter) && len(row.Subscribers) < podcastMaxSubscribers {
			row.Subscribers = append(row.Subscribers, reporter)
			dirty = true
		}
		resp.Accepted++
		if dirty {
			if err := s.putPodcastFeed(ctx, og, *row, now); err != nil {
				return resp, err
			}
			changed = true
		}
	}
	if len(added) > 0 {
		acct.Subs = append(acct.Subs, added...)
		if err := s.putPodcastIndex(ctx, og, acctKey, acct, now); err != nil {
			return resp, err
		}
	}
	// A playing episode counts only for a feed the account subscribes to
	// (stored or just now); it never creates a feed row.
	if np != nil && (have[npNorm] || podcastContains(added, npNorm)) {
		row, err := s.getPodcastFeed(ctx, og, npNorm)
		if err != nil {
			return resp, err
		}
		if row != nil {
			kept := pruneWanted(row.Wanted, now)
			dirty := len(kept) != len(row.Wanted)
			row.Wanted = kept
			found := false
			for i := range row.Wanted {
				if row.Wanted[i].GUID == np.EpisodeGUID {
					found = true
					// The mark's time moves only when it is old, so the same
					// phone asking again writes nothing.
					if at, err := time.Parse(time.RFC3339, row.Wanted[i].At); err != nil || now.Sub(at) > time.Hour {
						row.Wanted[i].At, dirty = now.Format(time.RFC3339), true
					}
				}
			}
			if !found {
				row.Wanted = append(row.Wanted, podcastWanted{GUID: np.EpisodeGUID, AudioURL: np.AudioURL, At: now.Format(time.RFC3339)})
				if len(row.Wanted) > podcastMaxWanted {
					row.Wanted = row.Wanted[len(row.Wanted)-podcastMaxWanted:]
				}
				dirty = true
			}
			if dirty {
				if err := s.putPodcastFeed(ctx, og, *row, now); err != nil {
					return resp, err
				}
				changed = true
			}
			resp.NowPlayingQueued = true
		}
	}
	if metaDirty {
		if err := s.putPodcastIndex(ctx, og, "meta", meta, now); err != nil {
			return resp, err
		}
	}
	if changed || len(added) > 0 {
		s.podcastCache.reset()
	}
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

// PodcastFeedListing is one feed the operator features.
type PodcastFeedListing struct {
	FeedURL string `json:"feed_url"`
	Title   string `json:"title"`
}

// SetPodcastFeedsListed features (or unfeatures) feeds as the operator's own:
// the seed list, or a feed the operator vouches for. A featured feed is the
// only kind that is public. Featuring never touches the subscriber counts; an
// unchanged feed writes nothing.
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
			if meta.Rows >= podcastMaxFeedRows {
				log.Printf("corpusd: podcast feeds: the %d-row limit is reached; a featured feed is refused", podcastMaxFeedRows)
				return 0, errPodcastRowLimit
			}
			row = &podcastFeedRow{FeedURL: f.url, Norm: f.norm, Title: f.title, FirstSeen: now.Format(time.RFC3339)}
			meta.Rows++
			metaDirty = true
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
// name and the newest episodes, which the match job reads by key. A feed with
// no row (nobody subscribed, the operator did not feature it) is not tracked.
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

// PodcastFeedForWorker is a feed as the worker's intake sees it.
type PodcastFeedForWorker struct {
	FeedURL     string          `json:"feed_url"`
	Title       string          `json:"title"`
	Subscribers int             `json:"subscribers"`
	Wanted      []podcastWanted `json:"wanted,omitempty"`
}

// allPodcastFeeds reads every feed row (there are at most podcastMaxFeedRows,
// which is the scan's bound, so it never truncates).
func (s *Service) allPodcastFeeds(ctx context.Context) ([]podcastFeedRow, error) {
	og, err := s.podcastStore()
	if err != nil {
		return nil, err
	}
	objs, err := og.ListObjects(ctx, objectgraph.ListFilter{Kind: ogKindPodcastFeed, OwnerID: podcastOwner, Limit: podcastMaxFeedRows})
	if err != nil {
		return nil, err
	}
	if len(objs) >= podcastMaxFeedRows {
		log.Printf("corpusd: podcast feeds: the listing reached the %d-row limit", podcastMaxFeedRows)
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

// ListPodcastFeedsForWorker lists every feed, featured first, then the most
// subscribed. It is the operator's worker's list, never a public one.
func (s *Service) ListPodcastFeedsForWorker(ctx context.Context) ([]PodcastFeedForWorker, error) {
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
	sort.SliceStable(rows, func(i, j int) bool {
		fi, fj := featured[rows[i].Norm], featured[rows[j].Norm]
		if fi != fj {
			return fi
		}
		return len(rows[i].Subscribers) > len(rows[j].Subscribers)
	})
	out := make([]PodcastFeedForWorker, 0, len(rows))
	for _, r := range rows {
		out = append(out, PodcastFeedForWorker{FeedURL: r.FeedURL, Title: r.Title, Subscribers: len(r.Subscribers), Wanted: pruneWanted(r.Wanted, now)})
	}
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

// ListPodcastPopular lists the featured feeds the worker has fetched, with a
// subscriber count where at least podcastPopularMin accounts follow the feed
// and 0 otherwise, never who. The title is the show's name as the worker parsed
// it. Read by key: the featured set, then each feed's row.
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
	var counted, rest []PodcastPopularFeed
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
		if len(row.Subscribers) >= podcastPopularMin {
			item.Subscribers = len(row.Subscribers)
			counted = append(counted, item)
		} else {
			rest = append(rest, item)
		}
	}
	sort.SliceStable(counted, func(i, j int) bool { return counted[i].Subscribers > counted[j].Subscribers })
	sort.SliceStable(rest, func(i, j int) bool { return rest[i].Title < rest[j].Title })
	out := append(counted, rest...)
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

// GetPodcastTranscriptFor reads an episode's transcript by feed and guid for
// the account reporter; nil when there is none or the caller may not see it. A
// featured feed's transcript is for every signed-in account; any other feed's
// only for an account that subscribes to it.
func (s *Service) GetPodcastTranscriptFor(ctx context.Context, feedURL, guid, reporter string) (*PodcastTranscriptResponse, error) {
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
		row, err := s.getPodcastFeed(ctx, og, norm)
		if err != nil {
			return nil, err
		}
		if row == nil || reporter == "" || !podcastContains(row.Subscribers, reporter) {
			return nil, nil // 404 for everyone else: existence is not revealed
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
