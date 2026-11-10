package platform

// Research's search over published pieces (docs/problems/oped-not-connected-2026-10-20.md):
// the Autopaper stories and opinion pieces a reader can open, found by their
// words, newest first within a score, from Store B alone. A hit carries a
// plain-text URL that choir.Read captures, so research cites the capture and
// never the snippet.

import (
	"context"
	"fmt"
	"math"
	"net/url"
	"strings"
)

const (
	publishedPieceDefaultLimit = 10
	publishedPieceMaxLimit     = 25
	// DefaultPublicBaseURL is the host a read_url names when
	// CORPUSD_PUBLIC_BASE_URL is unset.
	DefaultPublicBaseURL = "https://choir.news"
)

// Variables so tests can shrink them. publishedPieceBatch is how many
// publications one window reads; publishedPieceScanMax is how many one search
// reads at most before it reports capped.
var (
	publishedPieceBatch   = 100
	publishedPieceScanMax = 2000
)

// PublishedPiece is one published Autopaper story or opinion piece a search
// found. ReadURL is the plain-text export choir.Read captures.
type PublishedPiece struct {
	PublicationID string `json:"publication_id"`
	RoutePath     string `json:"route_path"`
	Title         string `json:"title"`
	Snippet       string `json:"snippet"`
	// Kind is "story" (an Autopaper story) or "opinion".
	Kind    string  `json:"kind"`
	Author  string  `json:"author,omitempty"`
	ReadURL string  `json:"read_url"`
	Score   float64 `json:"score"`
}

// PublishedPieceSearch is the answer: at most the limit, strongest first.
// Capped says the scan stopped at its cap before it saw every publication.
type PublishedPieceSearch struct {
	Query  string           `json:"query"`
	Pieces []PublishedPiece `json:"pieces"`
	Capped bool             `json:"capped,omitempty"`
}

// SetPublicBaseURL sets the host read_urls name; empty restores the default.
func (s *Service) SetPublicBaseURL(base string) {
	s.frontMu.Lock()
	defer s.frontMu.Unlock()
	s.publicBaseURL = strings.TrimRight(strings.TrimSpace(base), "/")
}

func (s *Service) publicBase() string {
	s.frontMu.RLock()
	defer s.frontMu.RUnlock()
	if s.publicBaseURL == "" {
		return DefaultPublicBaseURL
	}
	return s.publicBaseURL
}

// publicationExportReadURL is the raw plain-text export of a route.
func (s *Service) publicationExportReadURL(routePath string) string {
	q := url.Values{"route": {routePath}, "format": {"txt"}, "raw": {"1"}}
	return s.publicBase() + "/api/platform/publications/export?" + q.Encode()
}

// scanPublicationRows reads publication rows newest updated first, in
// windows of publishedPieceBatch, up to max rows. capped is true when more
// remain past the last row read.
func (s *Service) scanPublicationRows(ctx context.Context, max int) (rows []frontPublicationRow, capped bool, err error) {
	var after *frontCursor
	batch := publishedPieceBatch
	if batch < 1 {
		batch = 1
	}
	for len(rows) < max {
		n := batch
		if left := max - len(rows); n > left {
			n = left
		}
		got, err := s.frontPublicationBatch(ctx, after, n)
		if err != nil {
			return nil, false, err
		}
		rows = append(rows, got...)
		if len(got) < n {
			return rows, false, nil
		}
		last := got[len(got)-1]
		after = &frontCursor{UpdatedAt: last.UpdatedAt.Unix(), CanonicalID: last.CanonicalID}
	}
	rest, err := s.frontPublicationBatch(ctx, after, 1)
	if err != nil {
		return nil, false, err
	}
	return rows, len(rest) > 0, nil
}

// pieceKind names what a publication is to research: "story" for an
// Autopaper story, "" for anything else research does not search.
func (s *Service) pieceKind(meta frontPublicationMeta) string {
	if s.isAutopaperStory(meta) {
		return "story"
	}
	if s.isOpinionPiece(meta) {
		return "opinion"
	}
	return ""
}

// SearchPublishedPieces finds published Autopaper stories and opinion pieces
// for query, BM25-ranked over the whole archive by their latest published
// version (publication_pieces_index.go): a piece that holds some of the
// query's words ranks below one that holds more of them, and rarer words
// count more. An empty query lists the newest pieces. It returns the limit
// strongest (default 10, at most 25). Capped says the index is still being
// built, so the answer is from the newest part of the archive.
func (s *Service) SearchPublishedPieces(ctx context.Context, query string, limit int) (*PublishedPieceSearch, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("platform service unavailable")
	}
	og := s.ogStore()
	if og == nil {
		return nil, fmt.Errorf("platform service: object graph store unavailable")
	}
	if limit <= 0 {
		limit = publishedPieceDefaultLimit
	}
	if limit > publishedPieceMaxLimit {
		limit = publishedPieceMaxLimit
	}
	query = strings.TrimSpace(query)
	complete, err := s.refreshPieceIndex(ctx, publishedPieceSliceBudget, false)
	if err != nil {
		return nil, fmt.Errorf("platform pieces: %w", err)
	}
	out := &PublishedPieceSearch{Query: query, Pieces: []PublishedPiece{}, Capped: !complete}
	terms := distinctQueryTerms(query)
	var cands []pieceCandidate
	var idf map[string]float64
	if len(terms) == 0 {
		cands = s.pieces.recent(limit * 3)
	} else {
		// Extra candidates: a hit is verified against the store below, and a
		// withdrawn or unreadable one is replaced by the next.
		cands, idf = s.pieces.rank(terms, limit*3)
	}
	var hits []PublishedPiece
	for _, c := range cands {
		if len(hits) == limit {
			break
		}
		obj, err := og.GetObject(ctx, c.id)
		if err != nil || obj.Tombstone {
			continue
		}
		load, ok := s.loadPiece(ctx, og, frontPublicationRow{CanonicalID: c.id, Metadata: obj.Metadata, UpdatedAt: obj.UpdatedAt})
		if !ok {
			continue
		}
		hits = append(hits, PublishedPiece{
			PublicationID: load.meta.PublicationID,
			RoutePath:     load.route,
			Title:         load.title,
			Snippet:       snippet(load.content, snippetTerm(load.content, terms, idf, query)),
			Kind:          load.kind,
			Author:        load.meta.AuthorLabel,
			ReadURL:       s.publicationExportReadURL(load.route),
			Score:         math.Round(c.score*1000) / 1000,
		})
	}
	if hits != nil {
		out.Pieces = hits
	}
	return out, nil
}

// snippetTerm is the query word to centre the snippet on: the rarest one the
// text holds, else the query itself.
func snippetTerm(content string, terms []string, idf map[string]float64, query string) string {
	lower := strings.ToLower(content)
	best, bestW := "", -1.0
	for _, t := range terms {
		if w := idf[t]; w > bestW && strings.Contains(lower, t) {
			best, bestW = t, w
		}
	}
	if best == "" {
		return query
	}
	return best
}
