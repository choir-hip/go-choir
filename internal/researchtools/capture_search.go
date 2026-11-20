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
	// Query asks corpusd to search the pages' text as well (BM25 over
	// passages); Since, Until, Order and Limit shape that search.
	Query string `json:"query,omitempty"`
	Since string `json:"since,omitempty"`
	Until string `json:"until,omitempty"`
	Order string `json:"order,omitempty"`
	Limit int    `json:"limit,omitempty"`
	// Size is how many pages the ranked answer holds.
	Size int `json:"size,omitempty"`
	// ConversationID names the conversation whose earlier searches' leads the
	// sufficiency block reports separately; it filters nothing.
	ConversationID string `json:"conversation_id,omitempty"`
	// Sufficiency asks for the ranked answer's sufficiency block and leads.
	Sufficiency bool `json:"sufficiency,omitempty"`
}

// CaptureSearchAnswer is corpusd's candidate captures for the guest's own
// ranking.
type CaptureSearchAnswer struct {
	Objects  []objectgraph.Object `json:"objects"`
	Matched  int                  `json:"matched"`
	Capped   bool                 `json:"capped"`
	Searched int                  `json:"searched"`
	// Hits are the pages found by their text, best first; HitObjects are
	// those captures. TermsIncomplete: corpusd's title and label projection
	// is still building, so Objects is empty and the window covers that half.
	Hits            []CaptureTextHit     `json:"hits,omitempty"`
	HitObjects      []objectgraph.Object `json:"hit_objects,omitempty"`
	FullText        *CaptureTextInfo     `json:"fulltext,omitempty"`
	TermsIncomplete bool                 `json:"terms_incomplete,omitempty"`
	// Ranked is corpusd's one answer (platform.CaptureRanking); nil from a
	// corpusd that does not make one.
	Ranked *CaptureRanking `json:"ranked,omitempty"`
}

// CaptureRanking mirrors platform.CaptureRanking: the pages in the order the
// guest lists them, and which half answered.
type CaptureRanking struct {
	Entries           []RankedCapture `json:"entries"`
	Stories           []RankedStory   `json:"stories,omitempty"`
	VectorAnswered    bool            `json:"vector_answered"`
	VectorRows        int             `json:"vector_rows"`
	VectorUnavailable string          `json:"vector_unavailable,omitempty"`
	VectorState       string          `json:"vector_state,omitempty"`
	VectorPartial     string          `json:"vector_partial,omitempty"`
	BM25Kept          int             `json:"bm25_kept"`
	Note              string          `json:"note,omitempty"`
	// Sufficiency and Leads mirror platform.Sufficiency and platform.RankedLead
	// (landing L2 of the one search verb): measured, and no caller decides on
	// them yet.
	Sufficiency *Sufficiency `json:"sufficiency,omitempty"`
	Leads       []RankedLead `json:"leads,omitempty"`
}

// Sufficiency mirrors platform.Sufficiency.
type Sufficiency struct {
	BestCosine              float64               `json:"best_cosine"`
	Strong                  int                   `json:"strong"`
	NewestStrongAt          string                `json:"newest_strong_at,omitempty"`
	NewestStrongAgeSeconds  int64                 `json:"newest_strong_age_seconds,omitempty"`
	UnreadStrong            int                   `json:"unread_strong"`
	LeadsMatched            int                   `json:"leads_matched"`
	UnreadLeads             int                   `json:"unread_leads"`
	UnreadLeadsConversation int                   `json:"unread_leads_conversation"`
	UnreadLeadsWords        int                   `json:"unread_leads_words"`
	SameTerms               *bool                 `json:"same_terms,omitempty"`
	SameTermsAgeSeconds     int64                 `json:"same_terms_age_seconds,omitempty"`
	Thresholds              SufficiencyThresholds `json:"thresholds"`
	Note                    string                `json:"note,omitempty"`
}

// SufficiencyThresholds mirrors platform.SufficiencyThresholds.
type SufficiencyThresholds struct {
	StrongCosine float64 `json:"strong_cosine"`
	HorizonHours float64 `json:"horizon_hours"`
	LagHours     float64 `json:"lag_hours"`
	ReuseMinutes float64 `json:"reuse_minutes"`
}

// RankedLead mirrors platform.RankedLead.
type RankedLead struct {
	URL            string `json:"url"`
	Title          string `json:"title,omitempty"`
	Snippet        string `json:"snippet,omitempty"`
	PublishedAt    string `json:"published_at,omitempty"`
	LastReturnedAt string `json:"last_returned_at"`
	TimesReturned  int    `json:"times_returned"`
	Tried          bool   `json:"tried"`
	Conversation   bool   `json:"conversation,omitempty"`
	SameTerms      bool   `json:"same_terms,omitempty"`
}

// RankedCapture mirrors platform.RankedCapture.
type RankedCapture struct {
	CaptureID string   `json:"capture_id"`
	Reasons   []string `json:"reasons"`
	Stories   []string `json:"stories,omitempty"`
	Cosine    float64  `json:"cosine,omitempty"`
	At        string   `json:"at,omitempty"`
	Read      bool     `json:"read,omitempty"`
}

// RankedStory mirrors platform.RankedStory.
type RankedStory struct {
	PublicationID string   `json:"publication_id"`
	VersionID     string   `json:"version_id"`
	Revision      int      `json:"revision"`
	Title         string   `json:"title,omitempty"`
	Score         float64  `json:"score"`
	Day           string   `json:"day"`
	Captures      []string `json:"captures"`
}

// CaptureTextHit is one page corpusd found by its text, with the passage
// that found it (the mirror of platform.CaptureFullTextHit).
type CaptureTextHit struct {
	CaptureID   string  `json:"capture_id"`
	Title       string  `json:"title,omitempty"`
	Score       float64 `json:"score"`
	Ordinal     int     `json:"ordinal"`
	Passages    int     `json:"passages"`
	Snippet     string  `json:"snippet,omitempty"`
	FetchedAt   string  `json:"fetched_at,omitempty"`
	PublishedAt string  `json:"published_at,omitempty"`
	TitleMatch  bool    `json:"title_match"`
	BodyMatch   bool    `json:"body_match"`
}

// CaptureTextInfo says how corpusd's text search went (the mirror of
// platform.CaptureFullTextInfo).
type CaptureTextInfo struct {
	Used     bool   `json:"used"`
	Reason   string `json:"reason,omitempty"`
	Indexed  int64  `json:"indexed"`
	Captures int    `json:"captures"`
	Complete bool   `json:"complete"`
	Pages    int    `json:"pages"`
	Capped   bool   `json:"capped,omitempty"`
	// Words not searched in page text, and why (the mirror of
	// platform.CaptureFullTextInfo).
	Pruned          []string `json:"pruned,omitempty"`
	TermCapped      []string `json:"term_capped,omitempty"`
	Excluded        []string `json:"excluded,omitempty"`
	PhraseFallbacks []string `json:"phrase_fallbacks,omitempty"`
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
