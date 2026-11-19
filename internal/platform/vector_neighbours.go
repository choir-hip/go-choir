package platform

// The nearest held captures to named captures (station V5, the event layer's
// vector shortlist). The caller names captures it already holds; corpusd reads
// their stored vectors (no embedder, so an embedder that is down does not stop
// the answer) and returns, for each, the nearest other captures of the same
// owner and computer, under the same eligibility predicate and the same
// exact-or-index list as every other vector search. One request carries every
// capture of a flush.

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/embedder"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

const (
	retrievalVectorNeighbours = "vector_neighbours"
	// captureNeighboursMaxIDs bounds what one request may name (a bound on what
	// a caller can cause, not a limit on the event layer: a flush has at most
	// this many captures per request, and the guest sends the rest in the next).
	captureNeighboursMaxIDs       = 5000
	captureNeighboursDefaultLimit = 10
	captureNeighboursMaxLimit     = 50
	// captureNeighboursDeadline bounds the whole request; the captures it did
	// not reach come back as missing, with state partial.
	captureNeighboursDeadline = 30 * time.Second
)

// CaptureNeighboursRequest names captures of one owner and computer. Since is
// a day (YYYY-MM-DD or RFC 3339), inclusive, bounding the neighbours; Limit is
// how many neighbours each capture gets.
type CaptureNeighboursRequest struct {
	OwnerID    string   `json:"owner_id"`
	ComputerID string   `json:"computer_id"`
	CaptureIDs []string `json:"capture_ids"`
	Since      string   `json:"since,omitempty"`
	Limit      int      `json:"limit,omitempty"`
}

// CaptureNeighbour is one nearby capture and its cosine similarity.
type CaptureNeighbour struct {
	CaptureID string  `json:"capture_id"`
	Score     float64 `json:"score"`
}

// CaptureNeighboursResponse answers each named capture that has a vector.
// The captures it could not answer are counted by reason, never listed as one
// lump: Unembedded are eligible captures the worker has no vector for yet (the
// case a title-overlap shortlist used to cover); Excluded are the rest (private,
// tombstoned, superseded, another scope, not found); Cut were not reached
// because the deadline passed, which is State partial, not a fact about the
// captures. Unavailable is set, with no error, when the vectors cannot answer
// now; the caller proceeds without them.
type CaptureNeighboursResponse struct {
	Neighbours  map[string][]CaptureNeighbour `json:"neighbours"`
	Unembedded  int                           `json:"unembedded"`
	Excluded    int                           `json:"excluded"`
	Cut         int                           `json:"cut"`
	Unavailable string                        `json:"unavailable,omitempty"`
	// State is how the lists were answered: exact, approximate (the HNSW
	// index's answer on a large list) or partial (some were cut, or an exact
	// read did not finish).
	State string `json:"state,omitempty"`
}

var errBadNeighbours = errors.New("bad neighbours request")

const (
	// captureNeighboursGroup is how many named captures one SQL statement
	// answers (a lateral join over their stored vectors). A request of up to
	// this many is one statement; more is a statement a group, so a deadline
	// cuts a group and not the whole answer.
	captureNeighboursGroup = 500
)

// neighboursExactWork bounds the distance computations an exact answer may
// make (filtered rows times named captures); past it the HNSW index answers,
// and a list it returns short is read again exactly. A variable so a test can
// move it.
var neighboursExactWork int64 = 40_000_000

type neighbourRow struct {
	id, neighbour string
	score         float64
	has           bool
}

// CaptureNeighbours answers one request.
func (s *Store) CaptureNeighbours(ctx context.Context, req CaptureNeighboursRequest) (res CaptureNeighboursResponse, err error) {
	res.Neighbours = map[string][]CaptureNeighbour{}
	if s == nil {
		return res, errors.New("capture neighbours: nil store")
	}
	owner, computer := strings.TrimSpace(req.OwnerID), strings.TrimSpace(req.ComputerID)
	if owner == "" || computer == "" {
		return res, fmt.Errorf("%w: owner_id and computer_id are required", errBadNeighbours)
	}
	var ids []string
	seen := map[string]bool{}
	for _, id := range req.CaptureIDs {
		if id = strings.TrimSpace(id); id != "" && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	if len(ids) > captureNeighboursMaxIDs {
		return res, fmt.Errorf("%w: at most %d capture_ids", errBadNeighbours, captureNeighboursMaxIDs)
	}
	var since time.Time
	if v := strings.TrimSpace(req.Since); v != "" {
		for _, layout := range []string{time.RFC3339, "2006-01-02"} {
			if t, perr := time.Parse(layout, v); perr == nil {
				since = t
				break
			}
		}
		if since.IsZero() {
			return res, fmt.Errorf("%w: since %q is not a day or a time", errBadNeighbours, v)
		}
	}
	limit := req.Limit
	switch {
	case limit <= 0:
		limit = captureNeighboursDefaultLimit
	case limit > captureNeighboursMaxLimit:
		limit = captureNeighboursMaxLimit
	}
	if len(ids) == 0 {
		return res, nil
	}
	done := s.retrievalTimerExtra(retrievalVectorNeighbours, retrievalVectorNeighbours)
	rows := 0
	defer func() {
		done("captures", rows, err, fmt.Sprintf(" ids=%d unembedded=%d excluded=%d cut=%d state=%s", len(ids), res.Unembedded, res.Excluded, res.Cut, firstNonEmptyString(res.State, "unavailable")))
	}()
	if !s.VectorsReady() {
		if terr := s.ensureVectorTables(ctx); terr != nil && !s.VectorsReady() {
			// The detail (a database or extension error) is for the journal,
			// not for the guest.
			log.Printf("corpusd vectors: neighbours: the tables are not ready: %v", terr)
			res.Unavailable = "the vector tables are not ready"
			return res, nil
		}
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, captureNeighboursDeadline)
		defer cancel()
	}

	sinceArg := ""
	if !since.IsZero() {
		sinceArg = since.UTC().Format("2006-01-02")
	}
	// The same eligibility predicate in the count as in the query, so a scope
	// full of ineligible vectors is not taken for a large list.
	countQ := `SELECT COUNT(*) FROM ` + vecCaptureTable + ` v WHERE v.model = ? AND v.owner_id = ? AND v.computer_id = ?`
	countArgs := []any{embedder.ModelHash, owner, computer}
	if sinceArg != "" {
		countQ += ` AND v.day >= ?::date`
		countArgs = append(countArgs, sinceArg)
	}
	var filtered int64
	if cerr := s.corpus().QueryRowContext(ctx, countQ+` AND `+vecCaptureEligibleSQL, countArgs...).Scan(&filtered); cerr != nil {
		if ctx.Err() == nil {
			return res, cerr
		}
		res.Cut, res.State = len(ids), "partial"
		return res, nil
	}
	exact := filtered <= int64(vectorExactMaxRows) && filtered*int64(len(ids)) <= neighboursExactWork
	state := "exact"
	if !exact {
		state = "approximate"
	}

	answered := map[string]bool{}
	cut := map[string]bool{}
	for lo := 0; lo < len(ids); lo += captureNeighboursGroup {
		group := ids[lo:min(lo+captureNeighboursGroup, len(ids))]
		if ctx.Err() != nil {
			res.Cut += len(group)
			state = "partial"
			for _, id := range group {
				cut[id] = true
			}
			continue
		}
		got, serr := s.neighbourStatement(ctx, owner, computer, sinceArg, group, limit, exact)
		if serr != nil {
			if ctx.Err() != nil || isStatementTimeout(serr) {
				res.Cut += len(group)
				state = "partial"
				for _, id := range group {
					cut[id] = true
				}
				continue
			}
			return res, serr
		}
		lists := map[string][]CaptureNeighbour{}
		for _, r := range got {
			answered[r.id] = true
			if r.has {
				lists[r.id] = append(lists[r.id], CaptureNeighbour{CaptureID: r.neighbour, Score: r.score})
			}
		}
		if !exact {
			// An index list that came back short of the limit may lack nearer
			// rows (filters apply after an approximate scan): read those
			// again exactly.
			var short []string
			for _, id := range group {
				if answered[id] && len(lists[id]) < limit && filtered > int64(limit) {
					short = append(short, id)
				}
			}
			if len(short) > 0 {
				again, aerr := s.neighbourStatement(ctx, owner, computer, sinceArg, short, limit, true)
				if aerr != nil {
					if ctx.Err() == nil && !isStatementTimeout(aerr) {
						return res, aerr
					}
					state = "partial"
				} else {
					fixed := map[string][]CaptureNeighbour{}
					for _, r := range again {
						if r.has {
							fixed[r.id] = append(fixed[r.id], CaptureNeighbour{CaptureID: r.neighbour, Score: r.score})
						}
					}
					for _, id := range short {
						lists[id] = fixed[id]
					}
				}
			}
		}
		for id, list := range lists {
			res.Neighbours[id] = list
			rows += len(list)
		}
	}

	// Why the rest got no answer: an eligible capture with no vector yet is
	// unembedded; anything else (private, tombstoned, superseded, another
	// scope, unknown) is excluded.
	var rest []objectgraph.Object
	for _, id := range ids {
		if !answered[id] && !cut[id] {
			rest = append(rest, objectgraph.Object{CanonicalID: id, OwnerID: owner, ComputerID: computer})
		}
	}
	if len(rest) > 0 {
		eligible, eerr := s.sqlEligibleCaptures(ctx, rest)
		if eerr != nil {
			if ctx.Err() == nil {
				return res, eerr
			}
			res.Cut += len(rest)
			state = "partial"
		} else {
			for _, o := range rest {
				if eligible[o.CanonicalID] {
					res.Unembedded++
				} else {
					res.Excluded++
				}
			}
		}
	}
	res.State = state
	return res, nil
}

// isStatementTimeout is the database's own cut (SQLSTATE 57014, "canceling
// statement due to statement timeout"), which the deadline's 90% makes likely
// before the context ends.
func isStatementTimeout(err error) bool {
	return err != nil && (strings.Contains(err.Error(), "statement timeout") || strings.Contains(err.Error(), "57014"))
}

// neighbourStatement is one statement for every named capture: a lateral join
// of their stored vectors against the same eligible, scoped, windowed rows as
// every vector search. A named capture with no eligible vector gives no row; one
// with no neighbour gives a row with none.
func (s *Store) neighbourStatement(ctx context.Context, owner, computer, sinceArg string, ids []string, limit int, exact bool) ([]neighbourRow, error) {
	var b strings.Builder
	args := []any{}
	pool := vecCaptureTable
	if exact {
		// The eligible rows of the scope and window are found once, not once for
		// every named capture: the predicate is a lookup per row, and the
		// lateral join would repeat it for each pair. (The index path reads the
		// table, whose index orders the candidates; the predicate then runs on
		// the candidates only.)
		b.WriteString(`WITH pool AS MATERIALIZED (SELECT v.capture_id, v.embedding FROM ` + vecCaptureTable + ` v
			WHERE v.model = ? AND v.owner_id = ? AND v.computer_id = ?`)
		args = append(args, embedder.ModelHash, owner, computer)
		if sinceArg != "" {
			b.WriteString(` AND v.day >= ?::date`)
			args = append(args, sinceArg)
		}
		b.WriteString(` AND ` + vecCaptureEligibleSQL + `) `)
		pool = "pool"
	}
	b.WriteString(`SELECT q.capture_id, n.capture_id, n.score FROM ` + vecCaptureTable + ` q LEFT JOIN LATERAL (
		SELECT v.capture_id, 1 - (v.embedding <=> q.embedding) AS score FROM ` + pool + ` v
		WHERE v.capture_id <> q.capture_id`)
	if !exact {
		b.WriteString(` AND v.model = ? AND v.owner_id = ? AND v.computer_id = ?`)
		args = append(args, embedder.ModelHash, owner, computer)
		if sinceArg != "" {
			b.WriteString(` AND v.day >= ?::date`)
			args = append(args, sinceArg)
		}
		b.WriteString(` AND ` + vecCaptureEligibleSQL)
	}
	b.WriteString(`
		ORDER BY v.embedding <=> q.embedding LIMIT ?) n ON TRUE
		WHERE q.model = ? AND q.owner_id = ? AND q.computer_id = ? AND q.capture_id IN (` + strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",") + `)
		AND ` + strings.ReplaceAll(vecCaptureEligibleSQL, " v.", " q.") + `
		ORDER BY q.capture_id, n.score DESC`)
	args = append(args, limit, embedder.ModelHash, owner, computer)
	for _, id := range ids {
		args = append(args, id)
	}
	tx, err := s.corpus().BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	settings := []string{
		`SET LOCAL statement_timeout = '` + vectorStatementTimeout(ctx) + `'`,
		`SET LOCAL hnsw.iterative_scan = strict_order`,
		fmt.Sprintf(`SET LOCAL hnsw.ef_search = %d`, vectorEfSearch),
		fmt.Sprintf(`SET LOCAL hnsw.max_scan_tuples = %d`, vectorMaxScanTuples),
	}
	if exact {
		settings = append(settings, `SET LOCAL enable_indexscan = off`)
	}
	for _, stmt := range settings {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return nil, err
		}
	}
	if vectorSearchTxHook != nil {
		if err := vectorSearchTxHook(ctx, tx); err != nil {
			return nil, err
		}
	}
	rs, err := tx.QueryContext(ctx, b.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	var out []neighbourRow
	for rs.Next() {
		var r neighbourRow
		var nid sql.NullString
		var score sql.NullFloat64
		if err := rs.Scan(&r.id, &nid, &score); err != nil {
			return nil, err
		}
		r.neighbour, r.score, r.has = nid.String, score.Float64, nid.Valid
		out = append(out, r)
	}
	return out, rs.Err()
}

// HandleCaptureNeighbours handles POST /internal/platform/captures/neighbours,
// for a bound guest's own owner and computer or an internal caller.
func (h *ObjectGraphHandler) HandleCaptureNeighbours(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	var req CaptureNeighboursRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "invalid request body"})
		return
	}
	if strings.TrimSpace(req.OwnerID) == "" || strings.TrimSpace(req.ComputerID) == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "owner_id and computer_id are required"})
		return
	}
	if err := h.requireInternalOrBoundGuest(r, req.OwnerID, req.ComputerID, false); err != nil {
		writeJSON(w, http.StatusForbidden, apiError{Error: err.Error()})
		return
	}
	if status, msg := h.captureIndexRefusal(); status != 0 {
		writeJSON(w, status, apiError{Error: msg})
		return
	}
	res, err := h.captureIndex.CaptureNeighbours(r.Context(), req)
	if err != nil {
		if errors.Is(err, errBadNeighbours) {
			writeJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
			return
		}
		log.Printf("corpusd: capture neighbours for %s: %v", req.ComputerID, err)
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "capture neighbours failed"})
		return
	}
	writeJSON(w, http.StatusOK, res)
}
