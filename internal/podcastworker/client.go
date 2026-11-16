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
	"time"
)

// Client talks to the platform's operator podcast routes with an operator's
// API key. The key is held in memory and never logged or printed.
type Client struct {
	Base string
	key  string
	HTTP *http.Client
}

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
		return fmt.Errorf("%s %s: %d %s", method, path, resp.StatusCode, msg)
	}
	if out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			return fmt.Errorf("%s %s: decode answer: %w", method, path, err)
		}
	}
	return nil
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
	for start := 0; start < len(feeds); start += 200 {
		end := min(start+200, len(feeds))
		var items []item
		for _, f := range feeds[start:end] {
			items = append(items, item{FeedURL: f.URL, Title: f.Show})
		}
		if err := c.do(ctx, http.MethodPut, opBase+"feeds/listed", map[string]any{"feeds": items, "listed": true}, nil); err != nil {
			return err
		}
	}
	return nil
}

// MatchSummary is what a match run reports.
type MatchSummary struct {
	Stories  int `json:"stories"`
	Episodes int `json:"episodes"`
	Windows  int `json:"windows"`
	Pairs    int `json:"pairs"`
	JevCalls int `json:"jev_calls"`
	Clips    int `json:"clips"`
	Dropped  int `json:"dropped"`
	Covered  int `json:"stories_with_clips"`
}

// Match asks the platform to match the transcripts against the paper's
// stories of the last sinceHours hours.
func (c *Client) Match(ctx context.Context, sinceHours int) (*MatchSummary, error) {
	var sum MatchSummary
	if err := c.do(ctx, http.MethodPost, opBase+"match", map[string]int{"since_hours": sinceHours}, &sum); err != nil {
		return nil, err
	}
	return &sum, nil
}
