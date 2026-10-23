package platform

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sort"
	"time"
)

// defaultTapeExtraRefsMax caps the unique digests the extra-refs route
// answers (about 136 MB of JSON at 68 bytes each). The real set is expected
// in the thousands to low hundreds of thousands; over the cap the route
// refuses rather than truncate.
const defaultTapeExtraRefsMax = 2_000_000

var errTapeExtraRefsTooLarge = errors.New("tape extra refs: the keep set is over the route's cap")

// TapeExtraRefs is the database-side keep set of the tape sweep: the digests
// the five tapeExtraRefQueries columns name.
type TapeExtraRefs struct {
	Digests     []string
	Queries     int
	RowsScanned int
	GeneratedAt time.Time
}

// TapeExtraRefDigests runs exactly the sweep's extra-ref queries (tapeExtraRefs)
// and extracts digests with the sweep's own tapeHexRefs. Read-only: it takes
// no run lock and writes nothing. Any query, scan or iteration error returns
// an error and no list; so does a set over the cap.
func (s *Service) TapeExtraRefDigests(ctx context.Context) (*TapeExtraRefs, error) {
	limit := s.tapeSweep.extraRefsMax
	if limit <= 0 {
		limit = defaultTapeExtraRefsMax
	}
	set := map[string]struct{}{}
	rows, over := 0, false
	if err := s.tapeExtraRefs(ctx, func(text string) {
		rows++
		for _, d := range tapeHexRefs(text) {
			if over {
				return
			}
			set[d] = struct{}{}
			if len(set) > limit {
				over = true
				set = nil // stop holding the set; the rest is only drained
				return
			}
		}
	}); err != nil {
		return nil, err
	}
	if over {
		return nil, fmt.Errorf("%w (%d digests)", errTapeExtraRefsTooLarge, limit)
	}
	digests := make([]string, 0, len(set))
	for d := range set {
		digests = append(digests, d)
	}
	sort.Strings(digests)
	return &TapeExtraRefs{Digests: digests, Queries: len(tapeExtraRefQueries), RowsScanned: rows, GeneratedAt: time.Now().UTC()}, nil
}

// HandleTapeExtraRefs answers GET /internal/platform/artifact-gc/tape/extra-refs:
// the digests the tape sweep keeps because of the database, for the host-side
// repack tool's keep file. Same authority as HandleTapeSweep (the host-internal
// credential; a guest is refused). The list is built whole before the first
// byte is written, then streamed, so a failure never leaves a partial list.
func (h *Handler) HandleTapeExtraRefs(w http.ResponseWriter, r *http.Request) {
	if !trustedInternalCaller(r) {
		writeJSON(w, http.StatusForbidden, apiError{Error: "internal caller required"})
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	refs, err := h.service.TapeExtraRefDigests(r.Context())
	switch {
	case errors.Is(err, errTapeExtraRefsTooLarge):
		log.Printf("corpusd: tape extra refs: %v", err)
		writeJSON(w, http.StatusInsufficientStorage, apiError{Error: err.Error()})
		return
	case err != nil:
		log.Printf("corpusd: tape extra refs: %v", err)
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "tape extra refs: a query failed; no list returned"})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	bw := bufio.NewWriterSize(w, 64<<10)
	_, _ = bw.WriteString(`{"digests":[`)
	for i, d := range refs.Digests {
		if i > 0 {
			_ = bw.WriteByte(',')
		}
		_ = bw.WriteByte('"')
		_, _ = bw.WriteString(d) // 64 lowercase hex: nothing to escape
		_ = bw.WriteByte('"')
	}
	generated, _ := json.Marshal(refs.GeneratedAt)
	fmt.Fprintf(bw, `],"count":%d,"queries":%d,"rows_scanned":%d,"generated_at":%s}`+"\n",
		len(refs.Digests), refs.Queries, refs.RowsScanned, generated)
	if err := bw.Flush(); err != nil {
		log.Printf("corpusd: tape extra refs: write: %v", err)
	}
}
