package researchtools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// Corpus search and URL citation lookup over all of a computer's captures
// (rungs 2-3 of docs/archive/reports/object-graph-retrieval-research-2026-10-14.md;
// problem record docs/archive/problems/corpus-retrieval-windowed-unindexed-untimed-2026-10-14.md).
// corpusd answers from its capture projection and term table; the guest
// still ranks with its own matcher. Any refusal (route absent on an older
// corpusd, projection still building, CORPUSD_CAPTURE_SEARCH=off on the
// host) falls back to the window of the newest captures, and
// CHOIR_CORPUS_SEARCH=window on the guest never asks.

// corpusSearchModeEnv selects the capture search: "window" keeps the
// newest-captures window only; anything else uses corpusd's index first.
const corpusSearchModeEnv = "CHOIR_CORPUS_SEARCH"

func corpusIndexEnabled() bool {
	return strings.TrimSpace(strings.ToLower(os.Getenv(corpusSearchModeEnv))) != "window"
}

// CaptureSearchQuery is one corpus search over a computer's captures
// (corpusd POST /internal/platform/captures/search).
type CaptureSearchQuery struct {
	OwnerID    string   `json:"owner_id"`
	ComputerID string   `json:"computer_id"`
	Words      []string `json:"words"`
	Need       int      `json:"need"`
	Candidates int      `json:"candidates"`
	IDs        []string `json:"ids,omitempty"`
}

// CaptureSearchAnswer is corpusd's candidate captures for the guest's own
// ranking.
type CaptureSearchAnswer struct {
	Objects  []objectgraph.Object `json:"objects"`
	Matched  int                  `json:"matched"`
	Capped   bool                 `json:"capped"`
	Searched int                  `json:"searched"`
}

// CaptureSearcher is a capture sink that searches the shared store's index.
type CaptureSearcher interface {
	SearchCaptures(ctx context.Context, q CaptureSearchQuery) (CaptureSearchAnswer, error)
}

// CaptureURLResolver is a capture sink that finds the newest capture of a
// URL over all of a computer's captures.
type CaptureURLResolver interface {
	NewestCaptureOfURL(ctx context.Context, ownerID, computerID, pageURL string, blocksOnly bool) (string, bool, error)
}

// SearchCaptures asks corpusd's capture index.
func (c *CorpusdCaptureSink) SearchCaptures(ctx context.Context, q CaptureSearchQuery) (CaptureSearchAnswer, error) {
	var out CaptureSearchAnswer
	if c == nil || c.BaseURL == "" {
		return out, fmt.Errorf("capture search: corpusd URL is not configured")
	}
	raw, err := json.Marshal(q)
	if err != nil {
		return out, fmt.Errorf("capture search: %w", err)
	}
	body, err := c.do(ctx, http.MethodPost, c.BaseURL+"/internal/platform/captures/search", raw, 32<<20)
	if err != nil {
		return out, fmt.Errorf("capture search: %w", err)
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return out, fmt.Errorf("capture search: decode: %w", err)
	}
	return out, nil
}

// NewestCaptureOfURL asks corpusd's capture index for the newest capture of
// pageURL among the computer's captures.
func (c *CorpusdCaptureSink) NewestCaptureOfURL(ctx context.Context, ownerID, computerID, pageURL string, blocksOnly bool) (string, bool, error) {
	if c == nil || c.BaseURL == "" {
		return "", false, fmt.Errorf("capture url lookup: corpusd URL is not configured")
	}
	query := url.Values{"owner": {ownerID}, "computer": {computerID}, "url": {pageURL}}
	if blocksOnly {
		query.Set("blocks", "1")
	}
	body, err := c.do(ctx, http.MethodGet, c.BaseURL+"/internal/platform/captures/by-url?"+query.Encode(), nil, 1<<20)
	if err != nil {
		return "", false, fmt.Errorf("capture url lookup: %w", err)
	}
	var got struct {
		Found     bool   `json:"found"`
		CaptureID string `json:"capture_id"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		return "", false, fmt.Errorf("capture url lookup: decode: %w", err)
	}
	return got.CaptureID, got.Found && got.CaptureID != "", nil
}

// errCaptureIndexRefused is any non-200 answer of the index routes: the
// caller falls back to the window.
var errCaptureIndexRefused = errors.New("capture index refused")

func (c *CorpusdCaptureSink) do(ctx context.Context, method, target string, payload []byte, limit int64) ([]byte, error) {
	var reader io.Reader
	if payload != nil {
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, reader)
	if err != nil {
		return nil, err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	client := c.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, limit))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: corpusd returned %d: %s", errCaptureIndexRefused, resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	return raw, nil
}
