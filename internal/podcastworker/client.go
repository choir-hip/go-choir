package podcastworker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/yusefmosiah/go-choir/internal/podcastfp"
)

// Client talks to the platform's operator podcast routes with an operator's
// API key. The key is held in memory and never logged or printed.
type Client struct {
	Base string
	key  string
	HTTP *http.Client

	requests atomic.Int64
}

// Requests is how many HTTP requests this client has sent.
func (c *Client) Requests() int { return int(c.requests.Load()) }

// NewClient builds a client. base is the site, such as https://choir.news.
func NewClient(base, apiKey string) (*Client, error) {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	u, err := url.Parse(base)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return nil, fmt.Errorf("server %q is not an http(s) URL", base)
	}
	if u.Scheme == "http" && !isLoopbackHost(u.Hostname()) {
		return nil, fmt.Errorf("server %q: the operator key is never sent over cleartext to a host that is not this machine", base)
	}
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil, fmt.Errorf("the API key file is empty")
	}
	return &Client{Base: base, key: apiKey, HTTP: &http.Client{Timeout: 5 * time.Minute}}, nil
}

func (c *Client) do(ctx context.Context, method, path string, in, out any) error {
	var body io.Reader
	if in != nil {
		raw, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.Base+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.key)
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	c.requests.Add(1)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(raw))
		if len(msg) > 300 {
			msg = msg[:300]
		}
		return &HTTPError{Method: method, Path: path, Status: resp.StatusCode, Body: msg}
	}
	if out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			return fmt.Errorf("%s %s: decode answer: %w", method, path, err)
		}
	}
	return nil
}

// HTTPError is the platform's refusal of a request.
type HTTPError struct {
	Method, Path string
	Status       int
	Body         string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("%s %s: %d %s", e.Method, e.Path, e.Status, e.Body)
}

// Permanent reports whether repeating the same request cannot succeed: the
// platform refused its content (a 4xx other than a timeout or a rate limit).
func (e *HTTPError) Permanent() bool {
	return e.Status >= 400 && e.Status < 500 && e.Status != 401 && e.Status != 403 && e.Status != 408 && e.Status != 429
}

const opBase = "/api/platform/operator/podcasts/"

// PostEpisodes records feed items, in requests of at most 200. A chunk that
// fails does not stop the ones after it; the errors are returned together.
func (c *Client) PostEpisodes(ctx context.Context, eps []Episode) (recorded, fresh, skipped int, err error) {
	var errs []error
	for start := 0; start < len(eps); start += 200 {
		end := min(start+200, len(eps))
		var ans struct {
			Recorded int `json:"recorded"`
			New      int `json:"new"`
			Skipped  int `json:"skipped"`
		}
		if err := c.do(ctx, http.MethodPost, opBase+"episodes", map[string]any{"episodes": eps[start:end]}, &ans); err != nil {
			errs = append(errs, err)
			continue
		}
		recorded += ans.Recorded
		fresh += ans.New
		skipped += ans.Skipped
	}
	return recorded, fresh, skipped, errors.Join(errs...)
}

// PendingEpisode is an episode that needs a transcript.
type PendingEpisode struct {
	EpisodeID       string  `json:"episode_id"`
	FeedURL         string  `json:"feed_url"`
	GUID            string  `json:"guid"`
	Show            string  `json:"show"`
	Title           string  `json:"title"`
	Published       string  `json:"published"`
	DurationSeconds float64 `json:"duration_seconds"`
	EnclosureURL    string  `json:"enclosure_url"`
}

// PendingEpisodes lists episodes with no transcript, newest first.
func (c *Client) PendingEpisodes(ctx context.Context, since time.Time, limit int) ([]PendingEpisode, error) {
	q := url.Values{"need": {"transcript"}, "limit": {fmt.Sprint(limit)}}
	if !since.IsZero() {
		q.Set("since", since.UTC().Format(time.RFC3339))
	}
	var ans struct {
		Episodes []PendingEpisode `json:"episodes"`
	}
	if err := c.do(ctx, http.MethodGet, opBase+"episodes?"+q.Encode(), nil, &ans); err != nil {
		return nil, err
	}
	return ans.Episodes, nil
}

// PutTranscript stores one episode's transcript.
func (c *Client) PutTranscript(ctx context.Context, episodeID, engine string, r Result) error {
	return c.do(ctx, http.MethodPut, opBase+"transcripts", map[string]any{
		"episode_id": episodeID, "engine": engine, "audio_seconds": r.Seconds, "file_seconds": r.FileSeconds,
		"stable_file": r.StableFile, "segments": r.Segments,
	}, nil)
}

// ListFeeds tells the platform which feeds the operator lists as public (the
// seed list). It is idempotent and writes nothing for a feed already listed.
func (c *Client) ListFeeds(ctx context.Context, feeds []FeedSpec) error {
	type item struct {
		FeedURL string `json:"feed_url"`
		Title   string `json:"title"`
	}
	var errs []error
	for start := 0; start < len(feeds); start += 200 {
		end := min(start+200, len(feeds))
		var items []item
		for _, f := range feeds[start:end] {
			items = append(items, item{FeedURL: f.URL, Title: f.Show})
		}
		if err := c.do(ctx, http.MethodPut, opBase+"feeds/listed", map[string]any{"feeds": items, "listed": true}, nil); err != nil {
			errs = append(errs, err) // a failed chunk does not stop the next
		}
	}
	return errors.Join(errs...)
}

// PutClipsEach sends clips like PutClips and returns one error per clip, nil
// for a clip stored: a request the server refuses is retried clip by clip.
func (c *Client) PutClipsEach(ctx context.Context, clips []ClipIn) []error {
	errs := make([]error, len(clips))
	put := func(batch []ClipIn) error {
		return c.do(ctx, http.MethodPut, opBase+"clips", map[string]any{"clips": batch}, nil)
	}
	for start := 0; start < len(clips); start += 200 {
		end := min(start+200, len(clips))
		err := put(clips[start:end])
		if err == nil {
			continue
		}
		// A refused sign-in is the same for every clip: do not send them one by one.
		var he *HTTPError
		if errors.As(err, &he) && (he.Status == http.StatusUnauthorized || he.Status == http.StatusForbidden) {
			for i := start; i < end; i++ {
				errs[i] = err
			}
			continue
		}
		for i := start; i < end; i++ {
			errs[i] = put(clips[i : i+1])
		}
	}
	return errs
}

// DeleteClips removes clips by id (the operator route).
func (c *Client) DeleteClips(ctx context.Context, ids []string) error {
	for start := 0; start < len(ids); start += 200 {
		batch := ids[start:min(start+200, len(ids))]
		if err := c.do(ctx, http.MethodDelete, opBase+"clips", map[string]any{"clip_ids": batch}, nil); err != nil {
			return err
		}
	}
	return nil
}

// HeldClip is a clip the platform serves for a story.
type HeldClip struct {
	ClipID   string
	Route    string
	FeedURL  string
	GUID     string
	AudioURL string
	Start    float64
	End      float64
	Quote    string
	Support  float64
	Version  int
	// Fingerprint is nil for a clip stored without one.
	Fingerprint *podcastfp.ClipRecord
}

// Clips reads the clips the platform serves for the routes (the public read:
// at most three a story), in requests of at most 50 routes.
func (c *Client) Clips(ctx context.Context, routes []string) ([]HeldClip, error) {
	var out []HeldClip
	for start := 0; start < len(routes); start += 50 {
		q := url.Values{"limit": {"200"}}
		for _, r := range routes[start:min(start+50, len(routes))] {
			q.Add("route", r)
		}
		var ans struct {
			Clips []struct {
				Story struct {
					Route   string `json:"route"`
					Version int    `json:"version"`
				} `json:"story"`
				ClipID      string                `json:"clip_id"`
				FeedURL     string                `json:"feed_url"`
				GUID        string                `json:"episode_guid"`
				AudioURL    string                `json:"audio_url"`
				Start       float64               `json:"start"`
				End         float64               `json:"end"`
				Quote       string                `json:"quote"`
				Support     float64               `json:"support"`
				Fingerprint *podcastfp.ClipRecord `json:"fingerprint"`
			} `json:"clips"`
		}
		if err := c.do(ctx, http.MethodGet, "/api/platform/podcast-clips?"+q.Encode(), nil, &ans); err != nil {
			return nil, err
		}
		for _, k := range ans.Clips {
			out = append(out, HeldClip{ClipID: k.ClipID, Route: k.Story.Route, FeedURL: k.FeedURL, GUID: k.GUID, AudioURL: k.AudioURL, Start: k.Start, End: k.End, Quote: k.Quote, Support: k.Support, Version: k.Story.Version, Fingerprint: k.Fingerprint})
		}
	}
	return out, nil
}
