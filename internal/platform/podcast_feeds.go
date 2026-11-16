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
	"errors"
	"fmt"
	"log"
	"net/url"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

const (
	ogKindPodcastFeed = objectgraph.ObjectKind("choir.podcast_feed")

	podcastMaxFeedsPerReport = 50
	// podcastMaxSubscriptionsPerAccount bounds what one account can add to
	// the platform's intake (review 1 of 2026-11-16: no stranger intake).
	podcastMaxSubscriptionsPerAccount = 500
	podcastMaxTitleBytes              = 200
	podcastMaxSubscribers             = 20000
	podcastMaxWanted                  = 20
	podcastWantedTTL                  = 24 * time.Hour
	// podcastListedMin is how many distinct signed-in accounts must report a
	// feed before it is public (clips, popular): a private, tokenised feed one
	// subscriber was given must not become a public address.
	podcastListedMin = 2
	// podcastPopularMin is how many distinct accounts must report a feed
	// before it is shown with a count (k-anonymity).
	podcastPopularMin   = 3
	podcastPopularLimit = 100
	podcastFeedScan     = 5000
)

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

// podcastFeedRow is a stored feed.
type podcastFeedRow struct {
	FeedURL string `json:"feed_url"`
	Norm    string `json:"norm"`
	Title   string `json:"title"`
	// Featured marks a feed the operator lists: the seed list, or an act of
	// the operator's. A feed is public (listed) when it is featured or when
	// at least podcastListedMin accounts report it.
	Featured    bool            `json:"featured,omitempty"`
	FirstSeen   string          `json:"first_seen"`
	Subscribers []string        `json:"subscribers,omitempty"` // hashes of accounts, never shown
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

// podcastSubscriberHash is what is kept of an account: a hash.
func podcastSubscriberHash(accountID string) string {
	return podcastHashID("sub-", "podcast-subscriber\x00"+accountID)
}

// PodcastReporter is the hash of an account (a fixed-context sha256, not a
// secret: it keeps the account id out of the store, nothing more), for the
// proxy to pass.
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
		"norm": row.Norm, "subscribers": len(row.Subscribers), "featured": row.Featured,
	}, now)
	if err != nil {
		return err
	}
	return og.PutObject(corpusdWriter(ctx), obj)
}

// ReportPodcastFeeds takes a phone's report from a signed-in account (reporter
// is the hash of the account; empty is refused). Feeds join the worker's
// intake, the account counts once per feed, and the playing episode is marked.
// A report that changes nothing writes nothing.
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
	type checked struct {
		url, norm, title string
		subscribe        bool
	}
	var feeds []checked
	index := map[string]int{}
	for i, f := range req.Feeds {
		norm, err := NormalizeFeedURL(f.FeedURL)
		if err != nil {
			return nil, fmt.Errorf("feed %d: %w", i, err)
		}
		title, err := boundField("title", f.Title, podcastMaxTitleBytes)
		if err != nil {
			return nil, fmt.Errorf("feed %d: %w", i, err)
		}
		if _, dup := index[norm]; !dup {
			index[norm] = len(feeds)
			feeds = append(feeds, checked{url: strings.TrimSpace(f.FeedURL), norm: norm, title: title, subscribe: true})
		}
	}
	var np *PodcastNowPlaying
	if req.NowPlaying != nil {
		npNorm, err := NormalizeFeedURL(req.NowPlaying.FeedURL)
		if err != nil {
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
		if _, dup := index[npNorm]; !dup {
			index[npNorm] = len(feeds)
			feeds = append(feeds, checked{url: strings.TrimSpace(req.NowPlaying.FeedURL), norm: npNorm})
		}
	}
	og, err := s.podcastStore()
	if err != nil {
		return nil, err
	}
	s.podcastFeedMu.Lock()
	defer s.podcastFeedMu.Unlock()
	// The bound on what one account can add: a stored cap of subscriptions.
	rows, err := s.allPodcastFeeds(ctx)
	if err != nil {
		return nil, err
	}
	has := map[string]bool{}
	mine := 0
	for _, r := range rows {
		if podcastContains(r.Subscribers, reporter) {
			has[r.Norm] = true
			mine++
		}
	}
	for _, f := range feeds {
		if f.subscribe && !has[f.norm] {
			mine++
		}
	}
	if mine > podcastMaxSubscriptionsPerAccount {
		return nil, podcastInvalid("an account holds at most %d subscriptions", podcastMaxSubscriptionsPerAccount)
	}
	now := time.Now().UTC()
	resp := &PodcastFeedsResponse{}
	changed := false
	for _, f := range feeds {
		row, err := s.getPodcastFeed(ctx, og, f.norm)
		if err != nil {
			return resp, err
		}
		dirty := false
		if row == nil {
			row = &podcastFeedRow{FeedURL: f.url, Norm: f.norm, Title: f.title, FirstSeen: now.Format(time.RFC3339)}
			resp.NewFeeds++
			dirty = true
		}
		if row.Title == "" && f.title != "" {
			row.Title, dirty = f.title, true
		}
		if f.subscribe {
			if !podcastContains(row.Subscribers, reporter) && len(row.Subscribers) < podcastMaxSubscribers {
				row.Subscribers = append(row.Subscribers, reporter)
				dirty = true
			}
			resp.Accepted++
		}
		if np != nil && f.norm == np.FeedURL {
			kept := pruneWanted(row.Wanted, now)
			if len(kept) != len(row.Wanted) {
				dirty = true
			}
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
			resp.NowPlayingQueued = true
		}
		if dirty {
			if err := s.putPodcastFeed(ctx, og, *row, now); err != nil {
				return resp, err
			}
			changed = true
		}
	}
	if changed {
		s.podcastCache.reset() // a feed may have just become public
	}
	return resp, nil
}

func podcastContains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// podcastFeedListed reports whether a feed is public: featured by the
// operator or reported by at least podcastListedMin accounts.
func podcastFeedListed(r podcastFeedRow) bool {
	return r.Featured || len(r.Subscribers) >= podcastListedMin
}

// listedFeedSet is the normalised URLs of the public feeds.
func (s *Service) listedFeedSet(ctx context.Context) (map[string]bool, error) {
	rows, err := s.allPodcastFeeds(ctx)
	if err != nil {
		return nil, err
	}
	out := map[string]bool{}
	for _, r := range rows {
		if podcastFeedListed(r) {
			out[r.Norm] = true
		}
	}
	return out, nil
}

// PodcastFeedListing is one feed the operator lists.
type PodcastFeedListing struct {
	FeedURL string `json:"feed_url"`
	Title   string `json:"title"`
}

// SetPodcastFeedsListed lists (or unlists) feeds as the operator's own: the
// seed list, or a feed the operator vouches for. Listing never touches the
// subscriber counts. An unchanged feed writes nothing.
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
	changed := 0
	for _, f := range list {
		row, err := s.getPodcastFeed(ctx, og, f.norm)
		if err != nil {
			return changed, err
		}
		dirty := false
		if row == nil {
			if !listed {
				continue
			}
			row = &podcastFeedRow{FeedURL: f.url, Norm: f.norm, Title: f.title, FirstSeen: now.Format(time.RFC3339), Featured: true}
			dirty = true
		}
		if row.Featured != listed {
			row.Featured, dirty = listed, true
		}
		if f.title != "" && row.Title != f.title {
			row.Title, dirty = f.title, true
		}
		if dirty {
			if err := s.putPodcastFeed(ctx, og, *row, now); err != nil {
				return changed, err
			}
			changed++
		}
	}
	if changed > 0 {
		s.podcastCache.reset()
	}
	return len(list), nil
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
	if len(objs) >= podcastFeedScan {
		log.Printf("corpusd: podcast feeds: the listing hit its scan bound of %d; older feeds are not seen", podcastFeedScan)
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
		// Only a public feed the worker has fetched and parsed at least once:
		// a phone cannot put an arbitrary name on the public list, and a
		// private feed never appears.
		e, fetched := latest[r.Norm]
		if !fetched || !podcastFeedListed(r) {
			continue
		}
		item.LatestEpisodeTitle, item.LatestPublished = e.Title, e.Published
		if item.Title == "" {
			item.Title = e.Show
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

// GetPodcastTranscriptFor reads an episode's transcript by feed and guid for
// the account reporter; nil when there is none or the caller may not see it.
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
	// An unlisted feed's transcript is the subscriber's: it answers only an
	// account that reported the feed, and "not found" for anyone else, so its
	// existence is not revealed.
	norm, err := NormalizeFeedURL(ep.FeedURL)
	if err != nil {
		return nil, nil
	}
	row, err := s.getPodcastFeed(ctx, og, norm)
	if err != nil {
		return nil, err
	}
	if row == nil || (!podcastFeedListed(*row) && (reporter == "" || !podcastContains(row.Subscribers, reporter))) {
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
