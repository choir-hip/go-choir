package platform

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
)

const (
	defaultCheckpointListLimit = 20
	maxCheckpointListLimit     = 100
)

// clampCheckpointListLimit bounds a listing request.
func clampCheckpointListLimit(limit int) int {
	switch {
	case limit <= 0:
		return defaultCheckpointListLimit
	case limit > maxCheckpointListLimit:
		return maxCheckpointListLimit
	default:
		return limit
	}
}

// ListCheckpointSummaries lists a computer's checkpoints, newest first, with
// the host's verdict on each. A checkpoint published before the host judged
// checkpoints has no verdict row: it reads "unjudged", and its sequence is
// placed from the tape's receipts.
func (s *Store) ListCheckpointSummaries(ctx context.Context, computerID string, limit int) ([]selfdevprotocol.CheckpointSummary, error) {
	limit = clampCheckpointListLimit(limit)
	rows, err := s.db.QueryContext(ctx, `SELECT c.checkpoint_digest,c.checkpoint_json,c.created_at,
 v.accepted_sequence,v.status,v.reason,v.detail,v.differing_tables,v.base_ref,v.updated_at
 FROM computer_checkpoints c
 LEFT JOIN computer_checkpoint_verdicts v ON v.computer_id=c.computer_id AND v.checkpoint_digest=c.checkpoint_digest
 WHERE c.computer_id=? ORDER BY c.created_at DESC, c.checkpoint_digest DESC LIMIT ?`, computerID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []selfdevprotocol.CheckpointSummary{}
	for rows.Next() {
		var (
			digest, raw                    string
			createdAt                      time.Time
			sequence                       sql.NullInt64
			status, reason, detail, tables sql.NullString
			baseRef                        sql.NullString
			updatedAt                      sql.NullTime
			checkpoint                     selfdevprotocol.Checkpoint
		)
		if err := rows.Scan(&digest, &raw, &createdAt, &sequence, &status, &reason, &detail, &tables, &baseRef, &updatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(raw), &checkpoint); err != nil {
			return nil, fmt.Errorf("stored checkpoint %s is invalid: %w", digest, err)
		}
		request := checkpoint.Request
		row := selfdevprotocol.CheckpointSummary{
			CheckpointDigest: digest, AcceptedHead: request.AcceptedEventHead, ReleaseDigest: request.ReleaseDigest,
			EvidenceClass: checkpointEvidenceClass(request), CreatedAt: createdAt.UTC().Format(time.RFC3339Nano),
		}
		if status.Valid {
			row.Verdict = selfdevprotocol.CheckpointVerdict{
				Status: status.String, Reason: reason.String, Detail: detail.String, BaseRef: baseRef.String,
				AcceptedSequence: uint64(sequence.Int64),
			}
			if updatedAt.Valid {
				row.Verdict.UpdatedAt = updatedAt.Time.UTC().Format(time.RFC3339Nano)
			}
			if strings.TrimSpace(tables.String) != "" {
				_ = json.Unmarshal([]byte(tables.String), &row.Verdict.DifferingTables)
			}
			row.AcceptedSequence = uint64(sequence.Int64)
		} else {
			row.Verdict = selfdevprotocol.CheckpointVerdict{Status: selfdevprotocol.CheckpointVerdictUnjudged}
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	for i := range out {
		if out[i].AcceptedSequence != 0 || !computerevent.IsSHA256(out[i].AcceptedHead) {
			continue
		}
		var sequence uint64
		err := s.db.QueryRowContext(ctx, `SELECT sequence FROM computer_event_append_receipts WHERE computer_id=? AND event_digest=?`, computerID, out[i].AcceptedHead).Scan(&sequence)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		out[i].AcceptedSequence = sequence
		if out[i].Verdict.Status == selfdevprotocol.CheckpointVerdictUnjudged {
			out[i].Verdict.AcceptedSequence = sequence
		}
	}
	return out, nil
}

func checkpointEvidenceClass(request selfdevprotocol.CheckpointRequest) string {
	switch {
	case request.OwnerRecovery:
		return selfdevprotocol.CheckpointClassOwnerRecovery
	case request.PlatformFollow:
		return selfdevprotocol.CheckpointClassPlatformFollow
	default:
		return selfdevprotocol.CheckpointClassVerifier
	}
}

// StoredCheckpointResponse returns a published checkpoint with its receipt,
// scoped to its computer: the artifact a restore takes.
func (s *Store) StoredCheckpointResponse(ctx context.Context, computerID, checkpointDigest string) (selfdevprotocol.CheckpointResponse, error) {
	var checkpointJSON, receiptJSON string
	if err := s.db.QueryRowContext(ctx, `SELECT checkpoint_json,receipt_json FROM computer_checkpoints WHERE computer_id=? AND checkpoint_digest=?`, computerID, checkpointDigest).Scan(&checkpointJSON, &receiptJSON); err != nil {
		return selfdevprotocol.CheckpointResponse{}, err
	}
	var response selfdevprotocol.CheckpointResponse
	if json.Unmarshal([]byte(checkpointJSON), &response.Checkpoint) != nil || json.Unmarshal([]byte(receiptJSON), &response.Receipt) != nil {
		return selfdevprotocol.CheckpointResponse{}, fmt.Errorf("stored checkpoint response is invalid")
	}
	return response, nil
}

// HandleCheckpointList serves a computer's checkpoint listing, newest first
// with the host's verdicts, or one checkpoint with its receipt when
// checkpoint_digest is given: GET ?computer_id=&limit=&checkpoint_digest=.
func (h *Handler) HandleCheckpointList(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if h == nil || h.service == nil || h.service.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, apiError{Error: "checkpoint control unavailable"})
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	query := r.URL.Query()
	computerID := strings.TrimSpace(query.Get("computer_id"))
	digest := strings.TrimSpace(query.Get("checkpoint_digest"))
	limit := 0
	if raw := strings.TrimSpace(query.Get("limit")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, apiError{Error: "invalid limit"})
			return
		}
		limit = n
	}
	if !safeFileCASComponent(computerID) || (digest != "" && !computerevent.IsSHA256(digest)) {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "invalid checkpoint listing request"})
		return
	}
	if !h.authorizeFileCAS(r, computerID, "event:read") {
		writeJSON(w, http.StatusForbidden, apiError{Error: "computer capability required"})
		return
	}
	if digest != "" {
		response, err := h.service.store.StoredCheckpointResponse(r.Context(), computerID, digest)
		if errors.Is(err, sql.ErrNoRows) {
			http.NotFound(w, r)
			return
		}
		if err != nil {
			writeJSON(w, http.StatusServiceUnavailable, apiError{Error: "checkpoint unavailable"})
			return
		}
		writeJSON(w, http.StatusOK, response)
		return
	}
	list, err := h.service.store.ListCheckpointSummaries(r.Context(), computerID, limit)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, apiError{Error: "checkpoint listing unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, selfdevprotocol.CheckpointList{Checkpoints: list})
}
