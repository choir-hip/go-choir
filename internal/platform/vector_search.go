package platform

// The vector search of station V2: for a query, the nearest story passages and
// the nearest captures with their scores, and an optional day window. It does
// not fuse with BM25 or walk from a story to the pages it cites; the fused
// search (station V4) calls it.

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/embedder"
)

const (
	vectorSearchDefaultLimit = 20
	vectorSearchMaxLimit     = 200
	// vectorEfSearch is the HNSW candidate list a search reads; it is raised
	// from pgvector's 40 so a window or a scope that filters most of the
	// nearest does not leave the answer short (iterative scan does the rest).
	vectorEfSearch = 200
)

// VectorSearchRequest is one vector search. Give Query (the Node B embedder
// makes its vector with the query prompt) or Vector (a 256-value unit vector,
// for a caller that has one). Since is inclusive and Until exclusive, as days
// (UTC); either may be zero.
type VectorSearchRequest struct {
	Query  string
	Vector []float32
	Since  time.Time
	Until  time.Time
	// Owner and Computer scope the capture half: a capture of another scope
	// is never returned. With both empty the capture half is skipped, since
	// captures are not public.
	Owner, Computer string
	StoryLimit      int
	CaptureLimit    int
}

// VectorStoryHit is one story passage. PublicationID is the story (the
// publication) and VersionID the version the passage is from, so a caller
// can keep a publication's latest version and drop the others.
type VectorStoryHit struct {
	PublicationID string  `json:"publication_id"`
	VersionID     string  `json:"version_id"`
	Ordinal       int     `json:"ordinal"`
	Score         float64 `json:"score"`
	Day           string  `json:"day"`
	TextSHA       string  `json:"text_sha"`
}

// VectorCaptureHit is one capture.
type VectorCaptureHit struct {
	CaptureID string  `json:"capture_id"`
	Score     float64 `json:"score"`
	Day       string  `json:"day"`
	TextSHA   string  `json:"text_sha"`
}

// VectorSearchResult holds the two lists, each best first. Score is the
// cosine similarity (1 minus pgvector's cosine distance).
type VectorSearchResult struct {
	Stories  []VectorStoryHit   `json:"stories"`
	Captures []VectorCaptureHit `json:"captures"`
	// Unavailable is set, with a nil error, when the vectors cannot answer now
	// (no embedder, the unit down, the tables not ready): the lists are empty
	// and the caller answers from what it has, naming the gap. A search for
	// meaning never fails the caller's search.
	Unavailable string `json:"unavailable,omitempty"`
}

// ErrVectorsUnavailable says the vector tables do not exist yet.
var ErrVectorsUnavailable = errors.New("vector index is not ready")

// SearchVectors answers one vector search. A request that cannot be answered
// now because the vectors are unavailable (no embedder, the unit down, the
// tables not ready) returns no error and a result whose Unavailable says why;
// only a malformed request or a failed query is an error.
func (s *Store) SearchVectors(ctx context.Context, req VectorSearchRequest) (res VectorSearchResult, err error) {
	if s == nil {
		return res, errors.New("vector search: nil store")
	}
	if req.Vector == nil && strings.TrimSpace(req.Query) == "" {
		return res, errors.New("vector search: a query or a vector is required")
	}
	if req.Vector != nil && len(req.Vector) != embedder.Dim {
		return res, fmt.Errorf("vector search: vector of %d values, want %d", len(req.Vector), embedder.Dim)
	}
	if !s.VectorsReady() {
		if err := s.ensureVectorTables(ctx); err != nil && !s.VectorsReady() {
			res.Unavailable = "the vector tables are not ready: " + err.Error()
			return res, nil
		}
	}
	q := req.Vector
	if q == nil {
		emb := s.embedder()
		if emb == nil {
			res.Unavailable = "no embedder is configured"
			return res, nil
		}
		doneEmbed := s.retrievalTimer(retrievalVectorEmbed)
		vs, err := emb.Embed(ctx, []string{embedder.QueryInput(req.Query)})
		doneEmbed("query", 1, err)
		if err != nil {
			res.Unavailable = "the embedder did not answer: " + err.Error()
			return res, nil
		}
		q = vs[0]
	}
	lit := vecLiteral(q)
	clamp := func(n int) int {
		if n <= 0 {
			return vectorSearchDefaultLimit
		}
		return min(n, vectorSearchMaxLimit)
	}
	tx, err := s.corpus().BeginTx(ctx, nil)
	if err != nil {
		return res, err
	}
	defer func() { _ = tx.Rollback() }()
	// strict_order keeps the answer in distance order whatever the filter
	// drops; iterative scan keeps reading the graph until the limit is met.
	for _, stmt := range []string{
		`SET LOCAL hnsw.iterative_scan = strict_order`,
		fmt.Sprintf(`SET LOCAL hnsw.ef_search = %d`, vectorEfSearch),
	} {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return res, err
		}
	}
	window := func(args []any) (string, []any) {
		var b strings.Builder
		if !req.Since.IsZero() {
			b.WriteString(` AND day >= ?::date`)
			args = append(args, req.Since.UTC().Format("2006-01-02"))
		}
		if !req.Until.IsZero() {
			b.WriteString(` AND day < ?::date`)
			args = append(args, req.Until.UTC().Format("2006-01-02"))
		}
		return b.String(), args
	}

	doneStories := s.retrievalTimer(retrievalVectorSearch)
	args := []any{lit, embedder.ModelHash}
	w, args := window(args)
	args = append(args, lit, clamp(req.StoryLimit))
	rows, err := tx.QueryContext(ctx, `SELECT story_id, version_id, ordinal, to_char(day, 'YYYY-MM-DD'), text_sha, 1 - (embedding <=> ?::halfvec(256)) AS score
		FROM vec_story_passages WHERE model = ?`+w+` ORDER BY embedding <=> ?::halfvec(256), story_id, version_id, ordinal LIMIT ?`, args...)
	if err == nil {
		for rows.Next() {
			var h VectorStoryHit
			if err = rows.Scan(&h.PublicationID, &h.VersionID, &h.Ordinal, &h.Day, &h.TextSHA, &h.Score); err != nil {
				break
			}
			res.Stories = append(res.Stories, h)
		}
		if err == nil {
			err = rows.Err()
		}
		_ = rows.Close()
	}
	doneStories("vector_stories", len(res.Stories), err)
	if err != nil {
		return res, err
	}

	if strings.TrimSpace(req.Owner) != "" && strings.TrimSpace(req.Computer) != "" {
		doneCaptures := s.retrievalTimer(retrievalVectorSearch)
		args := []any{lit, embedder.ModelHash, req.Owner, req.Computer}
		w, args := window(args)
		args = append(args, lit, clamp(req.CaptureLimit))
		rows, err := tx.QueryContext(ctx, `SELECT capture_id, to_char(day, 'YYYY-MM-DD'), text_sha, 1 - (embedding <=> ?::halfvec(256)) AS score
			FROM vec_captures WHERE model = ? AND owner_id = ? AND computer_id = ?`+w+` ORDER BY embedding <=> ?::halfvec(256), capture_id LIMIT ?`, args...)
		if err == nil {
			for rows.Next() {
				var h VectorCaptureHit
				if err = rows.Scan(&h.CaptureID, &h.Day, &h.TextSHA, &h.Score); err != nil {
					break
				}
				res.Captures = append(res.Captures, h)
			}
			if err == nil {
				err = rows.Err()
			}
			_ = rows.Close()
		}
		doneCaptures("vector_captures", len(res.Captures), err)
		if err != nil {
			return res, err
		}
	}
	return res, tx.Commit()
}
