package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// --- Ceramic Provider ---

const ceramicSearchURL = "https://api.ceramic.ai/search"

// ceramicMaxResults and ceramicMaxWords are Ceramic's documented limits
// (docs/reference/CERAMIC.md); ceramicDescriptionLength is the smallest
// description it allows, since we use titles, URLs and a short excerpt.
const (
	ceramicMaxResults        = 20
	ceramicDefaultResults    = 10
	ceramicMaxWords          = 50
	ceramicDescriptionLength = 1000
)

// errCeramicQueryUnsupported is returned by a direct Search of a query the
// provider declines; the router asks DeclinesQuery first and never calls.
var errCeramicQueryUnsupported = errors.New("ceramic: query has no supported form")

// CeramicProvider implements search using Ceramic's Search API
// (docs/reference/CERAMIC.md): a lexical engine over English pages, prepaid.
// It is the provider every search starts with.
type CeramicProvider struct {
	httpClient *http.Client
	// endpoint overrides ceramicSearchURL (tests).
	endpoint string
}

func (p *CeramicProvider) Name() string { return "ceramic" }

func (p *CeramicProvider) IsAvailable() bool {
	return os.Getenv("CERAMIC_API_KEY") != ""
}

func (p *CeramicProvider) http() *http.Client {
	if p.httpClient != nil {
		return p.httpClient
	}
	return &http.Client{Timeout: 30 * time.Second}
}

// DeclinesQuery implements searchplane.QueryDecliner: a query Ceramic cannot
// take (non-Latin letters, or nothing left after preparation) is not sent.
func (p *CeramicProvider) DeclinesQuery(query string) bool {
	_, ok := prepareCeramicQuery(query)
	return !ok
}

var (
	ceramicSchemePattern = regexp.MustCompile(`(?i)\b[a-z][a-z0-9+.\-]*://`)
	ceramicNumberComma   = regexp.MustCompile(`(\d),(\d)`)
	ceramicQuoteReplacer = strings.NewReplacer("“", `"`, "”", `"`, "‘", "'", "’", "'")
)

// prepareCeramicQuery turns a caller's query into one Ceramic accepts, or
// reports ok=false when none exists. Ceramic takes 1 to 50 space-separated
// words and does not support slashes, numbers with commas or non-Latin
// characters. Quotes, OR and parentheses pass through; so does a
// site:domain operator's domain (its path, being slashes, becomes words).
func prepareCeramicQuery(query string) (string, bool) {
	for _, r := range query {
		if unicode.IsLetter(r) && !unicode.Is(unicode.Latin, r) {
			return "", false
		}
	}
	q := ceramicQuoteReplacer.Replace(query)
	q = ceramicSchemePattern.ReplaceAllString(q, "")
	q = strings.ReplaceAll(q, "/", " ")
	for {
		next := ceramicNumberComma.ReplaceAllString(q, "$1$2")
		if next == q {
			break
		}
		q = next
	}
	words := strings.Fields(q)
	if len(words) > ceramicMaxWords {
		words = words[:ceramicMaxWords]
		out := strings.Join(words, " ")
		if strings.Count(out, `"`)%2 == 1 {
			// The cut fell inside a quoted phrase: close it.
			words[len(words)-1] += `"`
		}
	}
	if len(words) == 0 {
		return "", false
	}
	return strings.Join(words, " "), true
}

// ceramicStatusError is a non-2xx answer. Its text is "status N Text:" and
// the body's title and code reduced to letters: a classifier that reads
// the message (searchplane.ClassifyCall) must not meet a digit or a phrase
// of the vendor's detail text, and a key echoed back must not survive.
type ceramicStatusError struct {
	text       string
	retryAfter time.Duration
}

func (e *ceramicStatusError) Error() string { return e.text }

// RetryAfter is the wait Ceramic asked for, or zero.
func (e *ceramicStatusError) RetryAfter() time.Duration { return e.retryAfter }

var ceramicLettersOnly = regexp.MustCompile(`[^A-Za-z _.\-]+`)

func newCeramicStatusError(resp *http.Response, body []byte, apiKey string) error {
	var parsed struct {
		Title             string  `json:"title"`
		Code              string  `json:"code"`
		RetryAfterSeconds float64 `json:"retry_after_seconds"`
	}
	_ = json.Unmarshal(body, &parsed)
	detail := strings.TrimSpace(ceramicLettersOnly.ReplaceAllString(parsed.Title+" "+parsed.Code, ""))
	if len(detail) > 80 {
		detail = detail[:80]
	}
	if apiKey != "" {
		detail = strings.ReplaceAll(detail, apiKey, "[redacted]")
	}
	text := fmt.Sprintf("status %d %s", resp.StatusCode, http.StatusText(resp.StatusCode))
	if detail != "" {
		text += ": " + detail
	}
	wait := time.Duration(parsed.RetryAfterSeconds * float64(time.Second))
	if wait <= 0 {
		// The header is a guess: the error page does not say where the
		// wait is carried (docs/reference/CERAMIC.md, Not known).
		if secs, err := strconv.Atoi(strings.TrimSpace(resp.Header.Get("Retry-After"))); err == nil && secs > 0 {
			wait = time.Duration(secs) * time.Second
		}
	}
	return &ceramicStatusError{text: text, retryAfter: wait}
}

func (p *CeramicProvider) Search(ctx context.Context, query string, maxResults int) ([]SearchResult, error) {
	apiKey := os.Getenv("CERAMIC_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("CERAMIC_API_KEY not set")
	}
	prepared, ok := prepareCeramicQuery(query)
	if !ok {
		return nil, errCeramicQueryUnsupported
	}
	if maxResults <= 0 {
		maxResults = ceramicDefaultResults
	}
	if maxResults > ceramicMaxResults {
		maxResults = ceramicMaxResults
	}
	// The body admits no unknown fields: exactly these three.
	reqBody, err := json.Marshal(struct {
		Query                string `json:"query"`
		MaxResults           int    `json:"maxResults"`
		MaxDescriptionLength int    `json:"maxDescriptionLength"`
	}{prepared, maxResults, ceramicDescriptionLength})
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	endpoint := p.endpoint
	if endpoint == "" {
		endpoint = ceramicSearchURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(string(reqBody)))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := p.http().Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, newCeramicStatusError(resp, body, apiKey)
	}
	var result struct {
		Result struct {
			Results []struct {
				Title       string `json:"title"`
				URL         string `json:"url"`
				Description string `json:"description"`
			} `json:"results"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	results := make([]SearchResult, 0, len(result.Result.Results))
	for _, r := range result.Result.Results {
		if r.URL == "" {
			continue
		}
		results = append(results, SearchResult{Title: r.Title, URL: r.URL, Snippet: r.Description})
	}
	return results, nil
}
