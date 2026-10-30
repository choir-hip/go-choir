package platform

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
)

// CheckpointVerdictRecord is the host's judgment of one published checkpoint
// (docs/archive/design/apply-checkpoint-without-replay-2026-10-26.md, slice 2). The
// record is operational state in Store A, never part of the semantic tape.
type CheckpointVerdictRecord struct {
	ComputerID       string                            `json:"computer_id"`
	CheckpointDigest string                            `json:"checkpoint_digest"`
	AcceptedHead     string                            `json:"accepted_head"`
	CreatedAt        time.Time                         `json:"created_at"`
	Verdict          selfdevprotocol.CheckpointVerdict `json:"verdict"`
}

const checkpointVerdictColumns = `computer_id,checkpoint_digest,accepted_sequence,accepted_head,status,reason,detail,differing_tables,base_ref,created_at,updated_at`

func scanCheckpointVerdict(row interface{ Scan(...any) error }) (CheckpointVerdictRecord, error) {
	var (
		r         CheckpointVerdictRecord
		tables    string
		updatedAt time.Time
	)
	v := &r.Verdict
	if err := row.Scan(&r.ComputerID, &r.CheckpointDigest, &v.AcceptedSequence, &r.AcceptedHead, &v.Status, &v.Reason, &v.Detail, &tables, &v.BaseRef, &r.CreatedAt, &updatedAt); err != nil {
		return r, err
	}
	if strings.TrimSpace(tables) != "" {
		if err := json.Unmarshal([]byte(tables), &v.DifferingTables); err != nil {
			return r, fmt.Errorf("checkpoint verdict: decode differing tables: %w", err)
		}
	}
	v.UpdatedAt = updatedAt.UTC().Format(time.RFC3339Nano)
	return r, nil
}

// CheckpointVerdict returns the verdict row of one checkpoint, sql.ErrNoRows
// when the checkpoint has none (published before the host judged them).
func (s *Store) CheckpointVerdict(ctx context.Context, computerID, checkpointDigest string) (CheckpointVerdictRecord, error) {
	return scanCheckpointVerdict(s.db.QueryRowContext(ctx, `SELECT `+checkpointVerdictColumns+` FROM computer_checkpoint_verdicts WHERE computer_id=? AND checkpoint_digest=?`, computerID, checkpointDigest))
}

// PendingCheckpointVerdicts lists every checkpoint the host has not judged,
// oldest accepted head first.
func (s *Store) PendingCheckpointVerdicts(ctx context.Context) ([]CheckpointVerdictRecord, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+checkpointVerdictColumns+` FROM computer_checkpoint_verdicts WHERE status=? ORDER BY accepted_sequence, computer_id, checkpoint_digest`, selfdevprotocol.CheckpointVerdictPending)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CheckpointVerdictRecord
	for rows.Next() {
		r, err := scanCheckpointVerdict(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ResolveCheckpointVerdict records the host's judgment of a pending
// checkpoint. A verdict is judged once: a checkpoint that already has one is
// left as it is, and false is returned.
func (s *Store) ResolveCheckpointVerdict(ctx context.Context, computerID, checkpointDigest string, verdict selfdevprotocol.CheckpointVerdict) (bool, error) {
	switch verdict.Status {
	case selfdevprotocol.CheckpointVerdictVerified, selfdevprotocol.CheckpointVerdictNotDerivable, selfdevprotocol.CheckpointVerdictUnverified:
	default:
		return false, fmt.Errorf("checkpoint verdict: %q is not a judgment", verdict.Status)
	}
	tables, err := json.Marshal(verdict.DifferingTables)
	if err != nil {
		return false, err
	}
	if verdict.DifferingTables == nil {
		tables = []byte("[]")
	}
	detail := verdict.Detail
	if len(detail) > 4000 {
		detail = detail[:4000]
	}
	result, err := s.db.ExecContext(ctx, `UPDATE computer_checkpoint_verdicts SET status=?,reason=?,detail=?,differing_tables=?,base_ref=?,updated_at=? WHERE computer_id=? AND checkpoint_digest=? AND status=?`,
		verdict.Status, verdict.Reason, detail, string(tables), verdict.BaseRef, time.Now().UTC(), computerID, checkpointDigest, selfdevprotocol.CheckpointVerdictPending)
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	if n != 1 {
		return false, nil
	}
	return true, nil
}

// StoredCheckpoint returns a published checkpoint by digest, scoped to its
// computer.
func (s *Store) StoredCheckpoint(ctx context.Context, computerID, checkpointDigest string) (selfdevprotocol.Checkpoint, error) {
	var raw string
	if err := s.db.QueryRowContext(ctx, `SELECT checkpoint_json FROM computer_checkpoints WHERE computer_id=? AND checkpoint_digest=?`, computerID, checkpointDigest).Scan(&raw); err != nil {
		return selfdevprotocol.Checkpoint{}, err
	}
	var checkpoint selfdevprotocol.Checkpoint
	if err := json.Unmarshal([]byte(raw), &checkpoint); err != nil {
		return selfdevprotocol.Checkpoint{}, fmt.Errorf("stored checkpoint is invalid: %w", err)
	}
	if checkpoint.Digest != checkpointDigest || checkpoint.Request.ComputerID != computerID {
		return selfdevprotocol.Checkpoint{}, fmt.Errorf("stored checkpoint does not match its key")
	}
	return checkpoint, nil
}

// HandleCheckpointVerdict serves the host's verdict on one checkpoint:
// GET ?computer_id=&checkpoint_digest=. A checkpoint with no verdict row
// (published before the host judged them) is a 404, not a pending one.
func (h *Handler) HandleCheckpointVerdict(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if h == nil || h.service == nil || h.service.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, apiError{Error: "checkpoint control unavailable"})
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	computerID := strings.TrimSpace(r.URL.Query().Get("computer_id"))
	digest := strings.TrimSpace(r.URL.Query().Get("checkpoint_digest"))
	if !safeFileCASComponent(computerID) || !computerevent.IsSHA256(digest) {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "invalid verdict request"})
		return
	}
	if !h.authorizeFileCAS(r, computerID, "event:read") {
		writeJSON(w, http.StatusForbidden, apiError{Error: "computer capability required"})
		return
	}
	record, err := h.service.store.CheckpointVerdict(r.Context(), computerID, digest)
	if errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, apiError{Error: "checkpoint verdict unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, record.Verdict)
}
