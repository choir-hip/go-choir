package platform

// Research's search over published pieces (docs/problems/oped-not-connected-2026-10-20.md):
// the Autopaper stories and opinion pieces a reader can open, found by their
// words, newest first within a score, from Store B alone. A hit carries a
// plain-text URL that choir.Read captures, so research cites the capture and
// never the snippet.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
)

const (
	publishedPieceDefaultLimit = 10
	publishedPieceMaxLimit     = 25
	// publishedPieceMaxTerms bounds the query's words.
	publishedPieceMaxTerms = 8
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
	Kind    string `json:"kind"`
	Author  string `json:"author,omitempty"`
	ReadURL string `json:"read_url"`
	Score   int    `json:"score"`
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

func searchTerms(query string) []string {
	var out []string
	seen := map[string]bool{}
	for _, w := range strings.Fields(strings.ToLower(query)) {
		w = strings.Trim(w, ".,;:!?\"'()[]")
		if w == "" || seen[w] {
			continue
		}
		seen[w] = true
		out = append(out, w)
		if len(out) == publishedPieceMaxTerms {
			break
		}
	}
	return out
}

// SearchPublishedPieces finds published Autopaper stories and opinion pieces
// whose title or text holds every word of query (all pieces when query is
// empty), by their latest published version only. It walks every publication
// up to publishedPieceScanMax, newest first, and returns the limit strongest
// (default 10, at most 25).
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
	terms := searchTerms(query)
	rows, capped, err := s.scanPublicationRows(ctx, publishedPieceScanMax)
	if err != nil {
		return nil, fmt.Errorf("platform pieces: %w", err)
	}
	out := &PublishedPieceSearch{Query: query, Pieces: []PublishedPiece{}, Capped: capped}
	var hits []PublishedPiece
	for _, row := range rows {
		var meta frontPublicationMeta
		if err := json.Unmarshal(row.Metadata, &meta); err != nil {
			continue
		}
		if meta.State != "published" || strings.TrimSpace(meta.RoutePath) == "" {
			continue
		}
		kind := s.pieceKind(meta)
		if kind == "" {
			continue
		}
		versions, err := publishedVersions(ctx, og, row.CanonicalID)
		if err != nil || len(versions) == 0 {
			continue
		}
		versionObj, err := og.GetObject(ctx, versions[len(versions)-1].CanonicalID)
		if err != nil {
			continue
		}
		manifest, err := s.frontManifest(ctx, og, versionObj.CanonicalID)
		if err != nil || !publicationAccessPolicyAllowsPublicRoute(manifest.AccessPolicy) || manifest.StorageRef == "" {
			continue
		}
		contentBytes, err := s.readBlob(manifest.StorageRef)
		if err != nil {
			continue
		}
		content := string(contentBytes)
		title := strings.TrimSpace(firstNonEmpty(meta.Title, manifest.Title))
		score := 1
		if len(terms) > 0 {
			lowerContent, lowerTitle := strings.ToLower(content), strings.ToLower(title)
			score = 0
			for _, term := range terms {
				n := strings.Count(lowerContent, term)
				if strings.Contains(lowerTitle, term) {
					n += 3
				}
				if n == 0 {
					score = 0
					break
				}
				score += n
			}
			if score == 0 {
				continue
			}
		}
		route := normalizePublicationRoutePath(meta.RoutePath)
		hits = append(hits, PublishedPiece{
			PublicationID: meta.PublicationID,
			RoutePath:     route,
			Title:         title,
			Snippet:       snippet(content, firstNonEmpty(strings.Join(terms, " "), query)),
			Kind:          kind,
			Author:        meta.AuthorLabel,
			ReadURL:       s.publicationExportReadURL(route),
			Score:         score,
		})
	}
	// Newest first is the scan order; a stable sort keeps it within a score.
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].Score > hits[j].Score })
	if len(hits) > limit {
		hits = hits[:limit]
	}
	if hits != nil {
		out.Pieces = hits
	}
	return out, nil
}
