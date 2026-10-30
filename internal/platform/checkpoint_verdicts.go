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

	"github.com/yusefmosiah/go-choir/internal/buildinfo"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/recoveryplan"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
)

// ErrNoVerificationBase: no base at or below a checkpoint's head exists to
// start the host's rebuild from, and genesis is beyond the replay bound.
var ErrNoVerificationBase = errors.New("no base at or below the checkpoint head to rebuild from")

// checkpointVerifyReason is the reason prefix of a verification job.
const checkpointVerifyReason = "checkpoint-verify:"

// CheckpointVerdictRecord is the host's judgment of one published checkpoint
// (docs/design/apply-checkpoint-without-replay-2026-10-26.md, slice 2). The
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

// ProjectionBaseAtSequence returns a base the platform holds for the computer
// at exactly the given sequence: a retained or superseded base, or the
// advertised one.
func (s *Store) ProjectionBaseAtSequence(ctx context.Context, computerID string, sequence uint64) (string, bool, error) {
	var ref string
	err := s.db.QueryRowContext(ctx, `SELECT base_ref FROM computer_projection_bases WHERE computer_id=? AND sequence=? ORDER BY created_at DESC LIMIT 1`, computerID, sequence).Scan(&ref)
	if err == nil {
		return ref, true, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", false, err
	}
	err = s.db.QueryRowContext(ctx, `SELECT base_ref FROM computer_replay_watermarks WHERE computer_id=? AND watermark_sequence=?`, computerID, sequence).Scan(&ref)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return ref, ref != "", nil
}

// EnqueueCheckpointVerificationJob asks the host to rebuild the projection at
// a checkpoint's own head, the head the checkpoint's witness was taken at, not
// the computer's head now. The job seeds from the advertised base when that is
// at or below the checkpoint, else from the newest base that is; with none and
// the checkpoint beyond the replay bound it refuses by name rather than queue
// a job that can only block the computer's cadence. A job in flight, or a
// refusal that still stands, is returned unchanged like any other request.
func (s *Store) EnqueueCheckpointVerificationJob(ctx context.Context, record CheckpointVerdictRecord) (ProjectionJob, error) {
	if !computerevent.IsSHA256(record.AcceptedHead) || record.Verdict.AcceptedSequence == 0 || !computerevent.IsSHA256(record.CheckpointDigest) {
		return ProjectionJob{}, fmt.Errorf("invalid checkpoint verification request")
	}
	target := &projectionTarget{sequence: record.Verdict.AcceptedSequence, head: record.AcceptedHead}
	return s.enqueueProjectionJob(ctx, record.ComputerID, checkpointVerifyReason+record.CheckpointDigest, false, target)
}

// checkpointBaseSeed picks the base a verification job at sequence seeds
// from: the advertised watermark when it is at or below the target, else the
// newest recorded base that is.
func checkpointBaseSeed(ctx context.Context, tx *sql.Tx, computerID string, sequence, watermark uint64, advertised string, floor uint64) (string, uint64, error) {
	// The advertised base is the seed when it is at or below the target and
	// at or above the epoch root (floor; review 365).
	if watermark <= sequence && watermark >= floor {
		quarantined := false
		if advertised != "" {
			var err error
			if quarantined, err = baseQuarantined(ctx, tx, computerID, advertised); err != nil {
				return "", 0, err
			}
		}
		if !quarantined {
			return advertised, watermarkOrZero(advertised, watermark), nil
		}
	}
	var ref string
	var seq uint64
	// Bases refused twice are skipped (paper-never-checkpointed S1), and so
	// is every base below the computer's epoch root (floor; 0 for none).
	err := tx.QueryRowContext(ctx, `SELECT base_ref,sequence FROM computer_projection_bases WHERE computer_id=? AND sequence<=? AND sequence>=? AND base_ref NOT IN (SELECT base_ref FROM computer_projection_base_refusals WHERE computer_id=? AND build_commit=? AND refusals>=2) ORDER BY sequence DESC, created_at DESC LIMIT 1`, computerID, sequence, floor, computerID, buildinfo.Commit).Scan(&ref, &seq)
	if errors.Is(err, sql.ErrNoRows) {
		return "", 0, nil
	}
	if err != nil {
		return "", 0, err
	}
	return ref, seq, nil
}

func watermarkOrZero(ref string, watermark uint64) uint64 {
	if ref == "" {
		return 0
	}
	return watermark
}

// refuseUnreachableBase reports ErrNoVerificationBase when a job at sequence
// would have no seed and genesis is beyond the replay bound.
func refuseUnreachableBase(seed string, sequence uint64) error {
	if seed == "" && sequence > recoveryplan.MaxRecoveryTailEvents {
		return fmt.Errorf("%w (checkpoint at %d, replay bound %d)", ErrNoVerificationBase, sequence, recoveryplan.MaxRecoveryTailEvents)
	}
	return nil
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
