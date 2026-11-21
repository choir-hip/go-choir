package researchtools

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

// Search leads (corpusd's search_leads table, platform/search_leads.go): every
// web-search result a computer is handed is written to corpusd, and a Read of
// a URL marks the matching lead. Both are best effort. A failure is logged with
// its count and dropped; it never fails or delays the search or the read.

// SearchLeadHit mirrors platform.SearchLeadHit.
type SearchLeadHit struct {
	URL         string `json:"url"`
	Title       string `json:"title"`
	Snippet     string `json:"snippet"`
	PublishedAt string `json:"published_at,omitempty"`
}

// SearchLeadsWrite mirrors platform.SearchLeadsWrite: one search: every provider's results.
type SearchLeadsWrite struct {
	OwnerID        string          `json:"owner_id"`
	ComputerID     string          `json:"computer_id"`
	Query          string          `json:"query,omitempty"`
	ConversationID string          `json:"conversation_id,omitempty"`
	Results        []SearchLeadHit `json:"results"`
}

// SearchLeadWriter is a capture sink that takes search leads. A sink that does
// not (a test fake) simply gets none.
type SearchLeadWriter interface {
	WriteSearchLeads(ctx context.Context, w SearchLeadsWrite) (rows int, err error)
	MarkSearchLeadsTried(ctx context.Context, ownerID, computerID string, urls []string, captureID string) error
}

// WriteSearchLeads sends one search's results to corpusd in one request.
func (c *CorpusdCaptureSink) WriteSearchLeads(ctx context.Context, w SearchLeadsWrite) (int, error) {
	if c == nil || c.BaseURL == "" {
		return 0, fmt.Errorf("search leads: corpusd URL is not configured")
	}
	raw, err := json.Marshal(w)
	if err != nil {
		return 0, err
	}
	body, err := c.do(ctx, http.MethodPost, c.BaseURL+"/internal/platform/search-leads", raw, 1<<20)
	if err != nil {
		return 0, fmt.Errorf("search leads: %w", err)
	}
	var out struct {
		Rows int `json:"rows"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return 0, fmt.Errorf("search leads: decode: %w", err)
	}
	return out.Rows, nil
}

// SearchLeadStoryReader is a capture sink that names, for captures, the
// conversations whose searches returned the lead each capture's page came
// from. A sink that does not (a test fake) gives the caller none.
type SearchLeadStoryReader interface {
	SearchLeadConversations(ctx context.Context, ownerID, computerID string, captureIDs []string) (map[string][]string, error)
}

// SearchLeadConversations asks corpusd, in one request, which conversations
// (story documents, for a story's research) searched their way to each capture.
func (c *CorpusdCaptureSink) SearchLeadConversations(ctx context.Context, ownerID, computerID string, captureIDs []string) (map[string][]string, error) {
	if c == nil || c.BaseURL == "" {
		return nil, fmt.Errorf("search leads: corpusd URL is not configured")
	}
	raw, err := json.Marshal(map[string]any{"owner_id": ownerID, "computer_id": computerID, "capture_ids": captureIDs})
	if err != nil {
		return nil, err
	}
	body, err := c.do(ctx, http.MethodPost, c.BaseURL+"/internal/platform/search-leads/by-capture", raw, 4<<20)
	if err != nil {
		return nil, fmt.Errorf("search leads: %w", err)
	}
	var out struct {
		Conversations map[string][]string `json:"conversations"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("search leads: decode: %w", err)
	}
	return out.Conversations, nil
}

// MarkSearchLeadsTried tells corpusd a run read these URLs.
func (c *CorpusdCaptureSink) MarkSearchLeadsTried(ctx context.Context, ownerID, computerID string, urls []string, captureID string) error {
	if c == nil || c.BaseURL == "" {
		return fmt.Errorf("search leads: corpusd URL is not configured")
	}
	raw, err := json.Marshal(map[string]any{"owner_id": ownerID, "computer_id": computerID, "urls": urls, "capture_id": captureID})
	if err != nil {
		return err
	}
	if _, err := c.do(ctx, http.MethodPost, c.BaseURL+"/internal/platform/search-leads/tried", raw, 1<<20); err != nil {
		return fmt.Errorf("search leads: %w", err)
	}
	return nil
}

// Lead calls run off the caller's path, each under a deadline. None is dropped
// or queued for volume: a search writes one batch of all its results, so the
// calls are as many as the searches, and the log line carries how many were in
// flight so a spike is seen. The deadline bounds a slow corpusd.
const leadCallTimeout = 5 * time.Second

var leadInFlight atomic.Int64

// RunLeadCall runs fn in the background under the lead deadline and logs
// `search leads op=<op> rows=<rows> ms=<ms> err=<err> inflight=<n>`. It returns
// a channel closed when the call has finished.
func RunLeadCall(op string, rows int, fn func(ctx context.Context) error) <-chan struct{} {
	inflight := leadInFlight.Add(1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer leadInFlight.Add(-1)
		start := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), leadCallTimeout)
		defer cancel()
		err := fn(ctx)
		msg := "none"
		if err != nil {
			msg = strings.ReplaceAll(err.Error(), "\n", " ")
		}
		log.Printf("search leads op=%s rows=%d ms=%d err=%s inflight=%d", op, rows, time.Since(start).Milliseconds(), msg, inflight)
	}()
	return done
}

// markLeadTried marks the leads of the URLs a Read touched, off its path.
func markLeadTried(deps Dependencies, ownerID, computerID, captureID string, urls ...string) {
	w, ok := deps.Captures.(SearchLeadWriter)
	if !ok || w == nil || strings.TrimSpace(ownerID) == "" || strings.TrimSpace(computerID) == "" {
		return
	}
	RunLeadCall("lead_tried", len(urls), func(ctx context.Context) error {
		return w.MarkSearchLeadsTried(ctx, ownerID, computerID, urls, captureID)
	})
}
