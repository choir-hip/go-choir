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
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/embedder"
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
// Missing names the captures with no eligible vector (not embedded yet, or
// private, tombstoned or superseded) and, when State is partial, those the
// deadline did not reach. Unavailable is set, with no error, when the vectors
// cannot answer now; the caller proceeds without them.
type CaptureNeighboursResponse struct {
	Neighbours  map[string][]CaptureNeighbour `json:"neighbours"`
	Missing     []string                      `json:"missing,omitempty"`
	Unavailable string                        `json:"unavailable,omitempty"`
	// State is how the lists were answered: exact, approximate or partial.
	State string `json:"state,omitempty"`
}

var errBadNeighbours = errors.New("bad neighbours request")

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
		done("captures", rows, err, fmt.Sprintf(" ids=%d missing=%d state=%s", len(ids), len(res.Missing), firstNonEmptyString(res.State, "unavailable")))
	}()
	if !s.VectorsReady() {
		if err := s.ensureVectorTables(ctx); err != nil && !s.VectorsReady() {
			res.Unavailable = "the vector tables are not ready: " + err.Error()
			return res, nil
		}
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, captureNeighboursDeadline)
		defer cancel()
	}

	vectors, err := s.storedCaptureVectors(ctx, owner, computer, ids)
	if err != nil {
		return res, err
	}
	window := func(args []any) (string, []any) {
		if since.IsZero() {
			return "", args
		}
		return ` AND v.day >= ?::date`, append(args, since.UTC().Format("2006-01-02"))
	}
	state := "exact"
	for _, id := range ids {
		lit, ok := vectors[id]
		if !ok {
			res.Missing = append(res.Missing, id)
			continue
		}
		if ctx.Err() != nil {
			res.Missing = append(res.Missing, id)
			state = "partial"
			continue
		}
		cq, cargs := vectorCaptureQuery(lit, owner, computer, window, limit+1)
		cw, cargsC := window([]any{embedder.ModelHash, owner, computer})
		got, mode, partial, lerr := runVectorList(ctx, s, cq, cargs, limit+1,
			`SELECT COUNT(*) FROM vec_captures v WHERE v.model = ? AND v.owner_id = ? AND v.computer_id = ?`+cw, cargsC,
			func(r *sql.Rows) (VectorCaptureHit, error) {
				var h VectorCaptureHit
				err := r.Scan(&h.CaptureID, &h.Day, &h.TextSHA, &h.Score)
				return h, err
			})
		if lerr != nil {
			if ctx.Err() != nil {
				res.Missing = append(res.Missing, id)
				state = "partial"
				continue
			}
			return res, lerr
		}
		switch {
		case partial != "":
			state = "partial"
		case mode == vecModeIndex && state == "exact":
			state = "approximate"
		}
		sort.SliceStable(got, func(i, j int) bool { return got[i].Score > got[j].Score })
		var list []CaptureNeighbour
		for _, h := range got {
			if h.CaptureID == id || len(list) == limit {
				continue
			}
			list = append(list, CaptureNeighbour{CaptureID: h.CaptureID, Score: h.Score})
		}
		res.Neighbours[id] = list
		rows += len(list)
	}
	res.State = state
	return res, nil
}

// storedCaptureVectors reads the vectors (as pgvector text) of the named
// captures that are of this owner and computer and pass the eligibility
// predicate, in groups of 500 names.
func (s *Store) storedCaptureVectors(ctx context.Context, owner, computer string, ids []string) (map[string]string, error) {
	out := map[string]string{}
	for start := 0; start < len(ids); start += 500 {
		group := ids[start:min(start+500, len(ids))]
		marks := strings.TrimSuffix(strings.Repeat("?,", len(group)), ",")
		args := []any{embedder.ModelHash, owner, computer}
		for _, id := range group {
			args = append(args, id)
		}
		rows, err := s.corpus().QueryContext(ctx, `SELECT v.capture_id, v.embedding::text FROM `+vecCaptureTable+` v
			WHERE v.model = ? AND v.owner_id = ? AND v.computer_id = ? AND v.capture_id IN (`+marks+`) AND `+vecCaptureEligibleSQL, args...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id, lit string
			if err := rows.Scan(&id, &lit); err != nil {
				_ = rows.Close()
				return nil, err
			}
			out[id] = lit
		}
		err = rows.Err()
		_ = rows.Close()
		if err != nil {
			return nil, err
		}
	}
	return out, nil
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
