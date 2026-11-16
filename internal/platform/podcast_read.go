package platform

// The public clips read (docs/design/podcast-clips-2026-11-15.md 4.1). It reads
// by key: a story's clip index, each clip, its episode, the featured set. It
// never pages through "the newest N rows", so no stranger's writes can push a
// clip out of it. Only a featured feed's clips are shown.

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	podcastClipsMaxRoutes    = 60
	podcastClipsDefaultLimit = 120
	podcastClipsMaxLimit     = 200
	podcastReadCacheTTL      = 30 * time.Second
	podcastReadCacheEntries  = 256
	// PodcastClipsDefaultWindow is how far back the unfiltered read looks.
	PodcastClipsDefaultWindow = 72 * time.Hour
)

// podcastReadCache holds recent answers for a short time, so the public
// route's cost is bounded whatever strangers ask.
type podcastReadCache struct {
	mu      sync.Mutex
	entries map[string]podcastCachedClips
}

type podcastCachedClips struct {
	at    time.Time
	clips []PodcastClip
}

func (c *podcastReadCache) reset() {
	c.mu.Lock()
	c.entries = nil
	c.mu.Unlock()
}

func (c *podcastReadCache) get(key string) ([]PodcastClip, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok || time.Since(e.at) > podcastReadCacheTTL {
		return nil, false
	}
	return e.clips, true
}

func (c *podcastReadCache) put(key string, clips []PodcastClip) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entries == nil || len(c.entries) >= podcastReadCacheEntries {
		c.entries = map[string]podcastCachedClips{}
	}
	c.entries[key] = podcastCachedClips{at: time.Now(), clips: clips}
}

// storyClips reads one story's clips by key, best first: clips of a stable
// file before the others, then the best supported; at most three.
func (s *Service) storyClips(ctx context.Context, og *ObjectGraphStore, route string, featured map[string]bool, episodes map[string]*PodcastEpisode) ([]PodcastClip, error) {
	idx, err := s.getPodcastIndex(ctx, og, "clips:"+route)
	if err != nil {
		return nil, err
	}
	type joined struct {
		clip   PodcastClip
		stable bool
	}
	var rows []joined
	for i := len(idx.Clips) - 1; i >= 0; i-- {
		canonical, err := podcastCanonical(ogKindPodcastClip, idx.Clips[i])
		if err != nil {
			continue
		}
		obj, err := og.GetObject(ctx, canonical)
		if err != nil {
			if isNotFound(err) {
				continue
			}
			return nil, err
		}
		var row podcastClipRow
		if obj.Tombstone || json.Unmarshal(obj.Body, &row) != nil {
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
		// Only a featured feed's clips are public: a private feed a reader
		// reported must never reach this read.
		if n, nerr := NormalizeFeedURL(ep.FeedURL); nerr != nil || !featured[n] {
			continue
		}
		rows = append(rows, joined{stable: ep.StableFile != nil && *ep.StableFile, clip: PodcastClip{
			ClipID: row.ClipID, Story: PodcastClipStory{Route: row.StoryRoute, Version: row.StoryVersion, Title: row.StoryTitle},
			Show: ep.Show, EpisodeTitle: ep.Title, EpisodeGUID: ep.GUID, FeedURL: ep.FeedURL, Published: ep.Published,
			AudioURL: ep.EnclosureURL, DurationSeconds: ep.DurationSeconds, Start: row.Start, End: row.End, Quote: row.Quote, Support: row.Support,
		}})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].stable != rows[j].stable {
			return rows[i].stable
		}
		return rows[i].clip.Support > rows[j].clip.Support
	})
	var out []PodcastClip
	for _, r := range rows {
		if len(out) == podcastClipsPerStory {
			break
		}
		out = append(out, r.clip)
	}
	return out, nil
}

// ListPodcastClips is the public read. With routes it answers for those
// stories; without, for the stories of the front updated within the window.
// feeds, when given, put those feeds' clips first.
func (s *Service) ListPodcastClips(ctx context.Context, routes, feeds []string, limit int) (*PodcastClipsResponse, error) {
	if len(routes) > podcastClipsMaxRoutes || len(feeds) > podcastClipsMaxRoutes {
		return nil, podcastInvalid("at most %d routes and %d feeds", podcastClipsMaxRoutes, podcastClipsMaxRoutes)
	}
	if limit <= 0 {
		limit = podcastClipsDefaultLimit
	}
	if limit > podcastClipsMaxLimit {
		limit = podcastClipsMaxLimit
	}
	preferred := map[string]bool{}
	for _, f := range feeds {
		if n, err := NormalizeFeedURL(f); err == nil {
			preferred[n] = true
		}
	}
	var want []string
	seen := map[string]bool{}
	for _, r := range routes {
		if n := normalizePublicationRoutePath(r); !seen[n] {
			seen[n] = true
			want = append(want, n)
		}
	}
	cacheKey := "*"
	if len(want) > 0 {
		cacheKey = strings.Join(want, "\n")
	}
	clips, ok := s.podcastCache.get(cacheKey)
	if !ok {
		og, err := s.podcastStore()
		if err != nil {
			return nil, err
		}
		featured, err := s.featuredFeedSet(ctx, og)
		if err != nil {
			return nil, err
		}
		if len(want) == 0 {
			stories, err := s.recentPodcastStories(ctx, PodcastClipsDefaultWindow)
			if err != nil {
				return nil, err
			}
			for _, st := range stories {
				want = append(want, st.Route)
			}
		}
		episodes := map[string]*PodcastEpisode{}
		for _, route := range want {
			got, err := s.storyClips(ctx, og, route, featured, episodes)
			if err != nil {
				return nil, err
			}
			clips = append(clips, got...)
		}
		s.podcastCache.put(cacheKey, clips)
	}
	resp := &PodcastClipsResponse{GeneratedAt: time.Now().UTC(), Clips: []PodcastClip{}}
	if len(preferred) > 0 {
		// The reader's own shows first, the order kept inside both groups.
		var mine, rest []PodcastClip
		for _, c := range clips {
			if n, err := NormalizeFeedURL(c.FeedURL); err == nil && preferred[n] {
				mine = append(mine, c)
			} else {
				rest = append(rest, c)
			}
		}
		clips = append(append([]PodcastClip{}, mine...), rest...)
	}
	if len(clips) > limit {
		clips = clips[:limit]
	}
	resp.Clips = append(resp.Clips, clips...)
	return resp, nil
}
