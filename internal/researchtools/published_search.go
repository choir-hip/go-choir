package researchtools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

// The published half of the corpus search (docs/problems/oped-not-connected-2026-10-20.md):
// the Autopaper stories and opinion pieces a reader can open, found by their
// words through corpusd's published search. A hit carries read_url, the
// plain-text export choir.Read captures; research cites the capture, never
// the snippet.

// PublishedPiece is one published story or opinion piece.
type PublishedPiece struct {
	PublicationID string `json:"publication_id,omitempty"`
	RoutePath     string `json:"route_path"`
	Title         string `json:"title"`
	Snippet       string `json:"snippet,omitempty"`
	// Kind is "story" or "opinion".
	Kind    string `json:"kind"`
	Author  string `json:"author,omitempty"`
	ReadURL string `json:"read_url"`
}

// PublishedAnswer is corpusd's published search answer. Capped says the
// search stopped at its scan cap before it saw every publication.
type PublishedAnswer struct {
	Pieces []PublishedPiece `json:"pieces"`
	Capped bool             `json:"capped,omitempty"`
}

// PublishedSearcher is a capture sink that also searches published pieces.
type PublishedSearcher interface {
	SearchPublished(ctx context.Context, query string, limit int) (PublishedAnswer, error)
}

// SearchPublished asks corpusd's published search.
func (c *CorpusdCaptureSink) SearchPublished(ctx context.Context, query string, limit int) (PublishedAnswer, error) {
	var out PublishedAnswer
	if c == nil || c.BaseURL == "" {
		return out, fmt.Errorf("published search: corpusd URL is not configured")
	}
	q := url.Values{"q": {query}, "limit": {strconv.Itoa(limit)}}
	body, err := c.do(ctx, http.MethodGet, c.BaseURL+"/internal/platform/published/search?"+q.Encode(), nil, 4<<20)
	if err != nil {
		return out, fmt.Errorf("published search: %w", err)
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return out, fmt.Errorf("published search: decode: %w", err)
	}
	return out, nil
}

// searchPublished returns the published pieces for the query and a note
// that is empty when the search was whole. A search that failed or cannot be
// made returns no pieces and says so; it never reads as "nothing published".
func searchPublished(ctx context.Context, deps Dependencies, query string, limit int) ([]PublishedPiece, string) {
	searcher, ok := deps.Captures.(PublishedSearcher)
	if !ok {
		return []PublishedPiece{}, "published pieces not searched: the shared store is not configured"
	}
	answer, err := searcher.SearchPublished(ctx, query, limit)
	if err != nil {
		return []PublishedPiece{}, "published pieces not searched: " + err.Error()
	}
	if answer.Pieces == nil {
		answer.Pieces = []PublishedPiece{}
	}
	note := ""
	if answer.Capped {
		note = "published search capped: it read the newest publications only, not every one; narrow the query if a piece you expect is missing"
	}
	return answer.Pieces, note
}
