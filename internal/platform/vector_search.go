package platform

// The vector search of station V2: for a query, the nearest story passages and
// the nearest captures with their scores, and an optional day window. It does
// not fuse with BM25 or walk from a story to the pages it cites; the fused
// search (station V4) calls it.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/embedder"
)

const (
	vectorSearchDefaultLimit = 20
	vectorSearchMaxLimit     = 200
	// vectorEfSearch is the HNSW candidate list a search reads; it is raised
	// from pgvector's 40 (docs/reference/EMBEDDINGGEMMA.md 0.11.2).
	vectorEfSearch = 200
	// vectorSearchTimeout is the statement deadline of a search whose context
	// carries none. A caller that has a deadline (the ranked answer's, 4 s:
	// capture_rank.go) has that one only: each statement gets what is left of
	// it, so the two never race.
	vectorSearchTimeout = "5s"
	// vectorMinPgvector is the oldest pgvector the code runs on: iterative
	// index scans (hnsw.iterative_scan, hnsw.max_scan_tuples) are 0.8.0's.
	vectorMinPgvector = "0.8.0"
)

// vectorStatementTimeout is nine tenths of the time left to the context's
// deadline, in whole milliseconds (at least 1), or vectorSearchTimeout when it
// has none. It is the one deadline, kept a little short so that an exact read
// that passes it can still hand back the index's list as partial before the
// context ends.
func vectorStatementTimeout(ctx context.Context) string {
	d, ok := ctx.Deadline()
	if !ok {
		return vectorSearchTimeout
	}
	ms := time.Until(d).Milliseconds() * 9 / 10
	if ms < 1 {
		ms = 1
	}
	return fmt.Sprintf("%dms", ms)
}

// vectorMaxScanTuples is how many index tuples an iterative scan may visit
// before it gives up (pgvector's default is 20,000). A variable so a test can
// make the index scan end early.
var vectorMaxScanTuples = 100000

// vectorExactMaxRows is how many filtered rows a list may have and still be
// read exactly. Beyond it the list is the HNSW index's approximate nearest.
var vectorExactMaxRows = 100000

// vectorSearchStageHook runs inside a savepoint before the "count", "exact"
// and "index" statements (tests make them fail or pass the deadline).
var vectorSearchStageHook func(ctx context.Context, tx *sql.Tx, stage string) error

// vectorSearchTxHook runs inside each list's transaction, after the settings
// and before the first statement (tests steer the planner with it).
var vectorSearchTxHook func(ctx context.Context, tx *sql.Tx) error

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
	// ExactScan names the lists ("stories", "captures") answered by an exact
	// scan: their filtered rows number at most vectorExactMaxRows, or the
	// index came back short of the limit (filters apply after an approximate
	// index scan). Such a list is the true nearest of the rows it covers.
	// Approximate names the lists answered by the HNSW index alone: the
	// approximate nearest, as pgvector's index gives it (docs/reference/
	// EMBEDDINGGEMMA.md 0.11.2 says what that does and does not guarantee).
	ExactScan   []string `json:"exact_scan,omitempty"`
	Approximate []string `json:"approximate,omitempty"`
	// Partial is set when a list could not be completed (the exact scan
	// passed the statement deadline): the list holds what the index gave and
	// may lack nearer rows. A result without Partial is complete.
	Partial string `json:"partial,omitempty"`
}

// errVectorsUnsupported is a pgvector without iterative scans (below 0.8.0).
var errVectorsUnsupported = errors.New("the installed pgvector has no iterative index scans (needs 0.8.0 or later)")

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
	if why := s.vec.unsupportedReason(); why != "" {
		res.Unavailable = why
		return res, nil
	}
	// A search never runs DDL: the worker makes the tables and indexes.
	if !s.VectorsReady() {
		present, err := s.vectorTablesPresent(ctx)
		if err != nil || !present {
			res.Unavailable = "the vector tables do not exist yet"
			if err != nil {
				res.Unavailable += ": " + err.Error()
			}
			return res, nil
		}
		s.vec.ready.Store(true)
	}
	q := req.Vector
	if q == nil {
		// The query embedder when one is set (never the bulk one in that
		// case, even when it is down: see SetQueryEmbedder). A query longer
		// than the unit's slot is refused by the server (embedder.
		// InputRefusedError); like any other failure it is "unavailable", not
		// an error to the caller.
		emb := s.searchEmbedder()
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
	window := func(args []any) (string, []any) {
		var b strings.Builder
		if !req.Since.IsZero() {
			b.WriteString(` AND v.day >= ?::date`)
			args = append(args, req.Since.UTC().Format("2006-01-02"))
		}
		if !req.Until.IsZero() {
			b.WriteString(` AND v.day < ?::date`)
			args = append(args, req.Until.UTC().Format("2006-01-02"))
		}
		return b.String(), args
	}

	storyLimit := clamp(req.StoryLimit)
	doneStories := s.retrievalTimer(retrievalVectorSearch)
	sq, sargs := vectorStoryQuery(lit, window, storyLimit)
	cw, cargsS := window([]any{embedder.ModelHash})
	stories, mode, partial, err := runVectorList(ctx, s, sq, sargs, storyLimit, `SELECT COUNT(*) FROM vec_story_passages v WHERE v.model = ? AND v.embedding IS NOT NULL`+cw, cargsS, func(rows *sql.Rows) (VectorStoryHit, error) {
		var h VectorStoryHit
		err := rows.Scan(&h.PublicationID, &h.VersionID, &h.Ordinal, &h.Day, &h.TextSHA, &h.Score)
		return h, err
	})
	doneStories("vector_stories", len(stories), err)
	if errors.Is(err, errVectorsUnsupported) {
		res.Unavailable = err.Error()
		return res, nil
	}
	if err != nil {
		return res, err
	}
	sort.SliceStable(stories, func(i, j int) bool { return stories[i].Score > stories[j].Score })
	res.Stories = stories
	res.noteMode("stories", mode)
	res.Partial = joinPartial(res.Partial, "stories", partial)

	if strings.TrimSpace(req.Owner) != "" && strings.TrimSpace(req.Computer) != "" {
		capLimit := clamp(req.CaptureLimit)
		doneCaptures := s.retrievalTimer(retrievalVectorSearch)
		cq, cargs := vectorCaptureQuery(lit, req.Owner, req.Computer, window, capLimit)
		cw, cargsC := window([]any{embedder.ModelHash, req.Owner, req.Computer})
		caps, mode, partial, err := runVectorList(ctx, s, cq, cargs, capLimit, `SELECT COUNT(*) FROM vec_captures v WHERE v.model = ? AND v.embedding IS NOT NULL AND v.owner_id = ? AND v.computer_id = ?`+cw, cargsC, func(rows *sql.Rows) (VectorCaptureHit, error) {
			var h VectorCaptureHit
			err := rows.Scan(&h.CaptureID, &h.Day, &h.TextSHA, &h.Score)
			return h, err
		})
		doneCaptures("vector_captures", len(caps), err)
		if errors.Is(err, errVectorsUnsupported) {
			res.Unavailable = err.Error()
			return res, nil
		}
		if err != nil {
			return res, err
		}
		sort.SliceStable(caps, func(i, j int) bool { return caps[i].Score > caps[j].Score })
		res.Captures = caps
		res.noteMode("captures", mode)
		res.Partial = joinPartial(res.Partial, "captures", partial)
	}
	return res, nil
}

func (r *VectorSearchResult) noteMode(list, mode string) {
	if mode == vecModeIndex {
		r.Approximate = append(r.Approximate, list)
	} else {
		r.ExactScan = append(r.ExactScan, list)
	}
}

const (
	vecModeIndex = "index"
	vecModeExact = "exact"
)

func joinPartial(have, list, why string) string {
	if why == "" {
		return have
	}
	if have != "" {
		have += "; "
	}
	return have + list + ": " + why
}

// vectorStoryQuery is the story list's statement. The eligibility predicate
// is in it, so a passage whose text stopped being public is not returned
// before the sweep has removed it. ORDER BY is the bare distance so the
// planner can use the HNSW index's order; ties are broken in Go.
func vectorStoryQuery(lit string, window func([]any) (string, []any), limit int) (string, []any) {
	args := []any{lit, embedder.ModelHash}
	w, args := window(args)
	args = append(args, lit, limit)
	return `SELECT v.story_id, v.version_id, v.ordinal, to_char(v.day, 'YYYY-MM-DD'), v.text_sha, 1 - (v.embedding <=> ?::halfvec(256)) AS score
		FROM vec_story_passages v WHERE v.model = ? AND v.embedding IS NOT NULL` + w + ` AND ` + vecStoryEligibleSQL + `
		ORDER BY v.embedding <=> ?::halfvec(256) LIMIT ?`, args
}

// vectorCaptureQuery is the capture list's statement, scoped to one owner and
// computer and filtered by the same eligibility predicate.
func vectorCaptureQuery(lit, owner, computer string, window func([]any) (string, []any), limit int) (string, []any) {
	args := []any{lit, embedder.ModelHash, owner, computer}
	w, args := window(args)
	args = append(args, lit, limit)
	return `SELECT v.capture_id, to_char(v.day, 'YYYY-MM-DD'), v.text_sha, 1 - (v.embedding <=> ?::halfvec(256)) AS score
		FROM vec_captures v WHERE v.model = ? AND v.embedding IS NOT NULL AND v.owner_id = ? AND v.computer_id = ?` + w + ` AND ` + vecCaptureEligibleSQL + `
		ORDER BY v.embedding <=> ?::halfvec(256) LIMIT ?`, args
}

// runVectorList runs one list in its own transaction under a statement
// deadline and says how it was answered. pgvector applies a WHERE after the
// approximate HNSW scan: a filter that excludes most of the nearest rows
// leaves the scan short, and iterative scans stop at hnsw.max_scan_tuples or
// work_mem (pgvector 0.8.7 README, "Filtering", "Iterative Index Scans";
// src/hnswscan.c:256-264), with no signal when they stop. So:
//
//   - a list whose filtered rows number at most vectorExactMaxRows is read
//     exactly (index scans off: the planner reads the filtered rows by the
//     btree on scope and day or the table and sorts by distance): the true
//     nearest, a guarantee;
//   - a larger list is the index's answer with iterative scans on: ordered by
//     distance among the rows it found, approximate in which rows it found
//     (HNSW recall, widened by ef_search 200), and if it comes back short of the
//     limit it is read again exactly;
//   - an exact read that passes the statement deadline falls back to what the
//     index gave, and partial says so.
func runVectorList[T any](ctx context.Context, s *Store, query string, args []any, limit int, countQuery string, countArgs []any, scan func(*sql.Rows) (T, error)) (out []T, mode string, partial string, err error) {
	tx, err := s.corpus().BeginTx(ctx, nil)
	if err != nil {
		return nil, "", "", err
	}
	defer func() { _ = tx.Rollback() }()
	for _, stmt := range []string{
		`SET LOCAL statement_timeout = '` + vectorStatementTimeout(ctx) + `'`,
		// strict_order keeps the answer in distance order whatever the filter
		// drops; iterative scan keeps reading the graph until the limit is met.
		`SET LOCAL hnsw.iterative_scan = strict_order`,
		fmt.Sprintf(`SET LOCAL hnsw.ef_search = %d`, vectorEfSearch),
		fmt.Sprintf(`SET LOCAL hnsw.max_scan_tuples = %d`, vectorMaxScanTuples),
	} {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			if strings.Contains(err.Error(), "unrecognized configuration parameter") {
				return nil, "", "", errVectorsUnsupported
			}
			return nil, "", "", err
		}
	}
	if vectorSearchTxHook != nil {
		if err := vectorSearchTxHook(ctx, tx); err != nil {
			return nil, "", "", err
		}
	}
	collect := func() ([]T, error) {
		rows, err := tx.QueryContext(ctx, query, args...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var got []T
		for rows.Next() {
			item, err := scan(rows)
			if err != nil {
				return nil, err
			}
			got = append(got, item)
		}
		return got, rows.Err()
	}
	// The count and the exact read each run under a savepoint: a statement
	// that passes the deadline (or fails) is undone and the list falls back to
	// what the index gives, flagged partial; it never fails the search.
	attempt := func(stage string, fn func() error) (failure string) {
		if _, err := tx.ExecContext(ctx, `SAVEPOINT vec_try`); err != nil {
			return err.Error()
		}
		err := func() error {
			if vectorSearchStageHook != nil {
				if err := vectorSearchStageHook(ctx, tx, stage); err != nil {
					return err
				}
			}
			return fn()
		}()
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err().Error()
			}
			if _, rerr := tx.ExecContext(ctx, `ROLLBACK TO SAVEPOINT vec_try`); rerr != nil {
				return err.Error() + "; " + rerr.Error()
			}
			return err.Error()
		}
		_, _ = tx.ExecContext(ctx, `RELEASE SAVEPOINT vec_try`)
		return ""
	}
	// The index read is under a savepoint too: a deadline there returns an
	// empty list and says partial; no statement of a search is a hard failure
	// except a transaction that cannot begin or a cancelled caller.
	indexCollect := func() ([]T, string) {
		var got []T
		why := attempt("index", func() error {
			var err error
			got, err = collect()
			return err
		})
		if why != "" {
			return nil, "the index scan did not finish: " + why
		}
		return got, ""
	}
	indexList := func(partial string) ([]T, string, string, error) {
		got, p := indexCollect()
		if ctx.Err() != nil {
			return nil, "", "", ctx.Err()
		}
		return got, vecModeIndex, joinSentence(partial, p), nil
	}
	filtered := int64(-1)
	if why := attempt("count", func() error {
		return tx.QueryRowContext(ctx, countQuery, countArgs...).Scan(&filtered)
	}); why != "" {
		if ctx.Err() != nil {
			return nil, "", "", ctx.Err()
		}
		partial = "the filtered rows could not be counted: " + why
		filtered = int64(vectorExactMaxRows) + 1 // unknown: take the index
	}
	if filtered <= int64(vectorExactMaxRows) {
		var got []T
		why := attempt("exact", func() error {
			if _, err := tx.ExecContext(ctx, `SET LOCAL enable_indexscan = off`); err != nil {
				return err
			}
			var err error
			got, err = collect()
			return err
		})
		if why == "" {
			return got, vecModeExact, "", nil
		}
		if ctx.Err() != nil {
			return nil, "", "", ctx.Err()
		}
		return indexList(joinSentence(partial, "the exact scan did not finish: "+why))
	}
	out, partialIdx := indexCollect()
	if ctx.Err() != nil {
		return nil, "", "", ctx.Err()
	}
	if partialIdx != "" {
		return out, vecModeIndex, joinSentence(partial, partialIdx), nil
	}
	if len(out) >= limit {
		return out, vecModeIndex, partial, nil
	}
	var again []T
	why := attempt("exact", func() error {
		if _, err := tx.ExecContext(ctx, `SET LOCAL enable_indexscan = off`); err != nil {
			return err
		}
		var err error
		again, err = collect()
		return err
	})
	if why != "" {
		if ctx.Err() != nil {
			return nil, "", "", ctx.Err()
		}
		return out, vecModeIndex, joinSentence(partial, "the exact scan did not finish: "+why), nil
	}
	return again, vecModeExact, partial, nil
}

func joinSentence(a, b string) string {
	if a == "" {
		return b
	}
	return a + "; " + b
}
