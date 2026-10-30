package platform

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/recoveryplan"
)

// The projection job ledger: the deduplicated checkpoint/repair job document
// that vmctl's recovery admission and the proxy's recovery status enqueue and
// read through /internal/computers/projection-base/jobs. The worker that
// claimed and ran these jobs left with the projection bases (data system
// cutover, 2026-10-30); only the enqueue and read halves remain.

// checkpointVerifyReason is the reason prefix of a checkpoint verification
// job; the watermark advertisement never supersedes one.
const checkpointVerifyReason = "checkpoint-verify:"

const projectionJobColumns = `computer_id,generation,status,reason,target_sequence,target_head,watermark_sequence,seed_base_ref,key_digest,base_ref,error_text,genesis_repair,failures,lease_token,lease_until,updated_at`

type ProjectionJob struct {
	ComputerID        string     `json:"computer_id"`
	Generation        uint64     `json:"generation"`
	Status            string     `json:"status"`
	Reason            string     `json:"reason"`
	TargetSequence    uint64     `json:"target_sequence"`
	TargetHead        string     `json:"target_head"`
	WatermarkSequence uint64     `json:"watermark_sequence"`
	SeedBaseRef       string     `json:"seed_base_ref"`
	KeyDigest         string     `json:"-"`
	BaseRef           string     `json:"base_ref"`
	Error             string     `json:"error,omitempty"`
	GenesisRepair     bool       `json:"genesis_repair"`
	Failures          int        `json:"failures"`
	LeaseToken        string     `json:"-"`
	LeaseUntil        time.Time  `json:"-"`
	UpdatedAt         time.Time  `json:"updated_at"`
	WorkerHeartbeat   *time.Time `json:"worker_heartbeat,omitempty"`
	WorkerBuildCommit string     `json:"worker_build_commit,omitempty"`
	WorkerError       string     `json:"worker_error,omitempty"`
	ColdTail          uint64     `json:"cold_tail"`
	Alert             string     `json:"alert,omitempty"`
}

func scanProjectionJob(row interface{ Scan(...any) error }) (ProjectionJob, error) {
	var j ProjectionJob
	err := row.Scan(&j.ComputerID, &j.Generation, &j.Status, &j.Reason, &j.TargetSequence, &j.TargetHead, &j.WatermarkSequence, &j.SeedBaseRef, &j.KeyDigest, &j.BaseRef, &j.Error, &j.GenesisRepair, &j.Failures, &j.LeaseToken, &j.LeaseUntil, &j.UpdatedAt)
	return j, err
}

func (s *Store) ProjectionJob(ctx context.Context, computerID string) (ProjectionJob, error) {
	j, err := scanProjectionJob(s.db.QueryRowContext(ctx, `SELECT `+projectionJobColumns+` FROM computer_projection_jobs WHERE computer_id=?`, computerID))
	if err != nil {
		return j, err
	}
	var heartbeat time.Time
	if err := s.db.QueryRowContext(ctx, `SELECT heartbeat,build_commit,error_text FROM projection_worker_status WHERE worker_id='checkpoint'`).Scan(&heartbeat, &j.WorkerBuildCommit, &j.WorkerError); err == nil {
		j.WorkerHeartbeat = &heartbeat
	}
	var head, wm uint64
	if err := s.db.QueryRowContext(ctx, `SELECT h.sequence,COALESCE(w.watermark_sequence,0) FROM computer_event_heads h LEFT JOIN computer_replay_watermarks w ON w.computer_id=h.computer_id WHERE h.computer_id=?`, computerID).Scan(&head, &wm); err != nil {
		return j, err
	}
	j.ColdTail = 0
	if head > wm {
		j.ColdTail = head - wm
	}
	// Tail and failure alerts are reported together: a tail alert must never
	// mask the repeated-failure signal that explains why the tail grows.
	var alerts []string
	switch {
	case j.ColdTail >= 7500:
		alerts = append(alerts, "urgent_tail")
	case j.ColdTail >= 5000:
		alerts = append(alerts, "warning_tail")
	}
	if j.Failures >= 2 {
		alerts = append(alerts, "repeated_failure")
	}
	j.Alert = strings.Join(alerts, ",")
	return j, nil
}

// failedJobRetryBackoff is how long a twice-failed (non-deterministic) job
// waits before cadence or repair may re-admit it: 15m doubling per further
// failure, capped at 16h. Blocked jobs are deterministic refusals and still
// wait for changed inputs.
func failedJobRetryBackoff(failures int) time.Duration {
	shift := failures - 2
	if shift < 0 {
		shift = 0
	}
	if shift > 6 {
		shift = 6
	}
	return 15 * time.Minute << shift
}

// blockedByContextCancel reports whether a recorded job error names a context
// cancel or deadline: the exact Go error strings, since the schema has no
// failure-kind column. Only legacy `blocked` rows are matched this way; the
// worker now records such errors as `failed`.
func blockedByContextCancel(errorText string) bool {
	return strings.Contains(errorText, "context canceled") || strings.Contains(errorText, "context deadline exceeded")
}

type singleRowQuerier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// epochRoot returns the computer's epoch root (its base and sequence), or an
// empty ref when it has none.
func epochRoot(ctx context.Context, q singleRowQuerier, computerID string) (string, uint64, error) {
	var ref string
	var seq uint64
	err := q.QueryRowContext(ctx, `SELECT base_ref,sequence FROM computer_projection_epoch_roots WHERE computer_id=?`, computerID).Scan(&ref, &seq)
	if errors.Is(err, sql.ErrNoRows) {
		return "", 0, nil
	}
	return ref, seq, err
}

// EnqueueProjectionJob admits the computer's job at its current head, or
// returns the row already standing for it.
func (s *Store) EnqueueProjectionJob(ctx context.Context, computerID, reason string, genesisRepair bool) (ProjectionJob, error) {
	if !safeFileCASComponent(computerID) || len(computerID) > 128 || len(reason) > 255 {
		return ProjectionJob{}, fmt.Errorf("invalid checkpoint request")
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return ProjectionJob{}, err
	}
	defer tx.Rollback()
	var head uint64
	var target string
	if err := tx.QueryRowContext(ctx, `SELECT sequence,canonical_event_head FROM computer_event_heads WHERE computer_id=? FOR UPDATE`, computerID).Scan(&head, &target); err != nil {
		return ProjectionJob{}, err
	}
	var wm uint64
	var seed, keyDigest string
	err = tx.QueryRowContext(ctx, `SELECT watermark_sequence,base_ref FROM computer_replay_watermarks WHERE computer_id=?`, computerID).Scan(&wm, &seed)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return ProjectionJob{}, err
	}
	err = tx.QueryRowContext(ctx, `SELECT key_digest FROM computer_key_escrows WHERE computer_id=? AND protector='custodian'`, computerID).Scan(&keyDigest)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return ProjectionJob{}, err
	}
	// An epoch root is the floor of every replay this computer is admitted:
	// the tape below it is not replayed again (owner, 2026-10-27).
	rootRef, rootSeq, err := epochRoot(ctx, tx, computerID)
	if err != nil {
		return ProjectionJob{}, err
	}
	if rootRef != "" && (seed == "" || wm < rootSeq || genesisRepair) {
		// Never below the root: a genesis repair of this computer starts
		// from it.
		seed, wm = rootRef, rootSeq
	}
	old, err := scanProjectionJob(tx.QueryRowContext(ctx, `SELECT `+projectionJobColumns+` FROM computer_projection_jobs WHERE computer_id=? FOR UPDATE`, computerID))
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return ProjectionJob{}, err
	}
	now := time.Now().UTC()
	failures := old.Failures
	if err == nil {
		// Request traffic cannot reset a lease. A deterministic refusal waits for
		// changed inputs (seed/key); head growth alone does not admit it.
		sameInputs := old.SeedBaseRef == seed && old.KeyDigest == keyDigest && old.GenesisRepair == genesisRepair
		// A failed job is transient trouble (lease loss, store error), not a
		// deterministic refusal: it is re-admitted after a growing backoff so
		// two failures cannot silently end cadence for this computer.
		//
		// A `blocked` row whose recorded error is a context cancel or deadline
		// is also transient: before the worker stopped classifying wrapped
		// cancels as refusals, one cancelled read-back froze b@c's base for 30
		// hours. That branch exists only for rows written before the fix and
		// can be deleted once none remain.
		retriable := old.Status == "failed" || (old.Status == "blocked" && blockedByContextCancel(old.Error))
		failedBackingOff := retriable && sameInputs && now.Sub(old.UpdatedAt) < failedJobRetryBackoff(old.Failures)
		// A `blocked` base refusal is re-admitted: it waited for a second
		// refusal of the same seed to quarantine it, and nothing records base
		// refusals any more, so none is ever confirmed.
		readmit := old.Status == "blocked" && sameInputs && !retriable &&
			seed != "" && strings.Contains(old.Error, recoveryplan.ErrBaseRefused.Error())
		if old.Status == "queued" || old.Status == "running" || old.Status == "retry" || (old.Status == "succeeded" && head <= old.TargetSequence) || (old.Status == "blocked" && sameInputs && !retriable && !readmit) || failedBackingOff {
			return old, tx.Commit()
		}
		if old.SeedBaseRef != seed || old.KeyDigest != keyDigest {
			failures = 0
		}
	}
	j := ProjectionJob{ComputerID: computerID, Generation: old.Generation + 1, Status: "queued", Reason: reason, TargetSequence: head, TargetHead: target, WatermarkSequence: wm, SeedBaseRef: seed, KeyDigest: keyDigest, GenesisRepair: genesisRepair, Failures: failures, LeaseUntil: now, UpdatedAt: now}
	_, err = tx.ExecContext(ctx, `INSERT INTO computer_projection_jobs (`+projectionJobColumns+`) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT (computer_id) DO UPDATE SET generation=excluded.generation,status=excluded.status,reason=excluded.reason,target_sequence=excluded.target_sequence,target_head=excluded.target_head,watermark_sequence=excluded.watermark_sequence,seed_base_ref=excluded.seed_base_ref,key_digest=excluded.key_digest,base_ref='',error_text='',genesis_repair=excluded.genesis_repair,failures=excluded.failures,lease_token='',lease_until=excluded.lease_until,updated_at=excluded.updated_at`, j.ComputerID, j.Generation, j.Status, j.Reason, j.TargetSequence, j.TargetHead, j.WatermarkSequence, j.SeedBaseRef, j.KeyDigest, "", "", j.GenesisRepair, j.Failures, "", now, now)
	if err != nil {
		return j, err
	}
	if err = tx.Commit(); err != nil {
		return j, err
	}
	return j, nil
}
