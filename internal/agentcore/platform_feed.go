package agentcore

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// A computer's reads and posts to corpusd for the op-ed path (autopaper
// contracts 2 and 3): the op-ed feed of its stories, the notice feed of its
// owner, one published version, and the paper's verdict post. The owner is
// the one vmctl binds to the calling guest; no call names one.

// OpedFeedItemWire is one op-ed on one of this computer's stories.
type OpedFeedItemWire struct {
	VersionID    string               `json:"version_id"`
	RoutePath    string               `json:"route_path"`
	AuthorLabel  string               `json:"author_label"`
	ManifestHash string               `json:"manifest_hash"`
	PublishedAt  string               `json:"published_at"`
	RespondsTo   types.OpedRespondsTo `json:"responds_to"`
	NoticeID     string               `json:"notice_id"`
	StoryDocID   string               `json:"story_doc_id"`
	// PublisherID is the publishing owner (from corpusd's graph); the paper's
	// per-publisher bounds key on it, never on the byline.
	PublisherID string `json:"publisher_id"`
}

// FeedSkippedWire is a row corpusd named and passed over.
type FeedSkippedWire struct {
	ID     string `json:"id"`
	Reason string `json:"reason"`
}

// OpedFeedWire is a page of the op-ed feed.
type OpedFeedWire struct {
	Items      []OpedFeedItemWire `json:"items"`
	NextCursor string             `json:"next_cursor"`
	Skipped    []FeedSkippedWire  `json:"skipped,omitempty"`
}

// NoticeItemWire is one notice for this computer's owner.
type NoticeItemWire struct {
	ID        string                    `json:"id"`
	CreatedAt string                    `json:"created_at"`
	Notice    types.CommitmentChangelog `json:"notice"`
}

// NoticeFeedWire is a page of the notice feed.
type NoticeFeedWire struct {
	Items      []NoticeItemWire  `json:"items"`
	NextCursor string            `json:"next_cursor"`
	Skipped    []FeedSkippedWire `json:"skipped,omitempty"`
}

// OpedVerdictWire is the courier copy of a gate verdict.
type OpedVerdictWire struct {
	VersionID    string `json:"version_id"`
	Run          int    `json:"run"`
	Verdict      string `json:"verdict"`
	Reason       string `json:"reason,omitempty"`
	VerdictHash  string `json:"verdict_hash"`
	TapeRecordID string `json:"tape_record_id"`
}

// PublicationVersionWire is a published version as the gate reads it.
type PublicationVersionWire struct {
	PublicationVersionID string `json:"publication_version_id"`
	OwnerID              string `json:"owner_id"`
	AuthorLabel          string `json:"author_label,omitempty"`
	Title                string `json:"title"`
	RoutePath            string `json:"route_path"`
	ManifestHash         string `json:"manifest_hash"`
	Content              string `json:"content"`
}

// PlatformFeed is this computer's door to the corpusd feeds. Tests install a
// fake with SetPlatformFeed.
type PlatformFeed interface {
	Opeds(ctx context.Context, after string, limit int) (OpedFeedWire, error)
	Notices(ctx context.Context, after string, limit int) (NoticeFeedWire, error)
	ReadVersion(ctx context.Context, versionID string) (PublicationVersionWire, error)
	PostOpedVerdict(ctx context.Context, post OpedVerdictWire) error
}

// SetPlatformFeed installs the feed client (tests; nil removes it).
func (rt *Runtime) SetPlatformFeed(feed PlatformFeed) {
	if rt == nil {
		return
	}
	rt.polls.mu.Lock()
	defer rt.polls.mu.Unlock()
	rt.polls.feed, rt.polls.feedSet = feed, true
}

func (rt *Runtime) platformFeed() PlatformFeed {
	rt.polls.mu.Lock()
	defer rt.polls.mu.Unlock()
	if !rt.polls.feedSet {
		if base := strings.TrimRight(strings.TrimSpace(rt.cfg.CorpusdURL), "/"); base != "" {
			rt.polls.feed = httpPlatformFeed{baseURL: base, client: &http.Client{Timeout: 30 * time.Second}}
		}
		rt.polls.feedSet = true
	}
	return rt.polls.feed
}

type httpPlatformFeed struct {
	baseURL string
	client  *http.Client
}

func (f httpPlatformFeed) do(ctx context.Context, method, path string, query url.Values, body any, out any) error {
	target := f.baseURL + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	var rdr io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rdr = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, rdr)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := f.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%s %s answered %d", method, path, resp.StatusCode)
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(raw, out)
}

func feedQuery(after string, limit int) url.Values {
	q := url.Values{}
	if after != "" {
		q.Set("after", after)
	}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	return q
}

func (f httpPlatformFeed) Opeds(ctx context.Context, after string, limit int) (out OpedFeedWire, err error) {
	err = f.do(ctx, http.MethodGet, "/internal/platform/opeds", feedQuery(after, limit), nil, &out)
	return
}

func (f httpPlatformFeed) Notices(ctx context.Context, after string, limit int) (out NoticeFeedWire, err error) {
	err = f.do(ctx, http.MethodGet, "/internal/platform/notices", feedQuery(after, limit), nil, &out)
	return
}

func (f httpPlatformFeed) ReadVersion(ctx context.Context, versionID string) (out PublicationVersionWire, err error) {
	err = f.do(ctx, http.MethodGet, "/internal/platform/publications/versions/"+url.PathEscape(versionID), nil, nil, &out)
	return
}

func (f httpPlatformFeed) PostOpedVerdict(ctx context.Context, post OpedVerdictWire) error {
	return f.do(ctx, http.MethodPost, "/internal/platform/oped-verdicts", nil, post, nil)
}
