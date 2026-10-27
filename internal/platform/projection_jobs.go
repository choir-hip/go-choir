package platform

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/yusefmosiah/go-choir/internal/buildinfo"
	"github.com/yusefmosiah/go-choir/internal/recoveryplan"
)

// Operational projection work lives in Store A, never in the semantic tape.
const projectionJobSchemaDDL = `
CREATE TABLE IF NOT EXISTS computer_projection_jobs (
 computer_id VARCHAR(128) PRIMARY KEY,
 generation BIGINT UNSIGNED NOT NULL,
 status VARCHAR(32) NOT NULL,
 reason VARCHAR(255) NOT NULL,
 target_sequence BIGINT UNSIGNED NOT NULL,
 target_head CHAR(64) NOT NULL,
 watermark_sequence BIGINT UNSIGNED NOT NULL,
 seed_base_ref VARCHAR(255) NOT NULL,
 key_digest VARCHAR(128) NOT NULL,
 base_ref VARCHAR(255) NOT NULL,
 error_text TEXT NOT NULL,
 genesis_repair BOOLEAN NOT NULL,
 failures INT NOT NULL,
 lease_token VARCHAR(64) NOT NULL,
 lease_until DATETIME(6) NOT NULL,
 updated_at DATETIME(6) NOT NULL,
 build_commit VARCHAR(128) NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS computer_projection_base_refusals (
 computer_id VARCHAR(128) NOT NULL,
 base_ref VARCHAR(255) NOT NULL,
 refusals INT NOT NULL,
 last_error TEXT NOT NULL,
 updated_at DATETIME(6) NOT NULL,
 build_commit VARCHAR(128) NOT NULL DEFAULT '',
 PRIMARY KEY(computer_id,base_ref)
);
CREATE TABLE IF NOT EXISTS computer_projection_bases (
 computer_id VARCHAR(128) NOT NULL,
 base_ref VARCHAR(255) NOT NULL,
 sequence BIGINT UNSIGNED NOT NULL,
 created_at DATETIME(6) NOT NULL,
 PRIMARY KEY(computer_id,base_ref)
);
CREATE TABLE IF NOT EXISTS computer_projection_epoch_roots (
 computer_id VARCHAR(128) PRIMARY KEY,
 base_ref VARCHAR(255) NOT NULL,
 sequence BIGINT UNSIGNED NOT NULL,
 created_at DATETIME(6) NOT NULL
);
CREATE TABLE IF NOT EXISTS computer_projection_base_pins (
 computer_id VARCHAR(128) NOT NULL,
 base_ref VARCHAR(255) NOT NULL,
 reference_id VARCHAR(255) NOT NULL,
 created_at DATETIME(6) NOT NULL,
 PRIMARY KEY(computer_id,base_ref,reference_id)
);
CREATE TABLE IF NOT EXISTS computer_checkpoint_verdicts (
 computer_id VARCHAR(128) NOT NULL,
 checkpoint_digest CHAR(64) NOT NULL,
 accepted_sequence BIGINT UNSIGNED NOT NULL,
 accepted_head CHAR(64) NOT NULL,
 status VARCHAR(32) NOT NULL,
 reason VARCHAR(64) NOT NULL,
 detail TEXT NOT NULL,
 differing_tables TEXT NOT NULL,
 base_ref VARCHAR(255) NOT NULL,
 created_at DATETIME(6) NOT NULL,
 updated_at DATETIME(6) NOT NULL,
 PRIMARY KEY(computer_id,checkpoint_digest)
);
CREATE TABLE IF NOT EXISTS projection_worker_status (
 worker_id VARCHAR(64) PRIMARY KEY,
 heartbeat DATETIME(6) NOT NULL,
 build_commit VARCHAR(128) NOT NULL,
 error_text TEXT NOT NULL
);
`

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

func checkpointDue(head, watermark uint64, publishedAt, now time.Time) bool {
	if head <= watermark {
		return false
	}
	tail := head - watermark
	return tail >= 2500 || (tail >= 1000 && now.Sub(publishedAt) >= 24*time.Hour)
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

// ListProjectionJobs returns every projection job row (one per computer). It is
// a plain read: checkpointd's scratch sweep uses it to decide which scratch
// directories a queued, running or retrying job still holds.
func (s *Store) ListProjectionJobs(ctx context.Context) ([]ProjectionJob, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+projectionJobColumns+` FROM computer_projection_jobs`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ProjectionJob
	for rows.Next() {
		j, err := scanProjectionJob(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
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

// baseQuarantineRefusals is how many deterministic refusals of the same seed
// base quarantine it: the first could be an accident, the second confirms it.
const baseQuarantineRefusals = 2

// blockedLogInterval is the least time between two log lines for one blocked
// row, so a row blocked for days does not fill the journal.
const blockedLogInterval = time.Hour

// RecordBaseRefusal counts one deterministic refusal of a seed base for a
// computer and reports whether the base is now quarantined. Refusals are
// counted per worker build (a new build may open what the old one could not),
// and a successful publish for the computer clears them. A quarantined base
// is skipped by job enqueue and seed selection, which fall back to the next
// older base or to no seed (docs/problems/paper-never-checkpointed-2026-10-27.md).
func (s *Store) RecordBaseRefusal(ctx context.Context, computerID, baseRef, errorText string) (bool, error) {
	if !safeFileCASComponent(computerID) || baseRef == "" || len(baseRef) > 255 {
		return false, fmt.Errorf("invalid base refusal")
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO computer_projection_base_refusals (computer_id,base_ref,refusals,last_error,updated_at,build_commit) VALUES (?,?,1,?,?,?) ON DUPLICATE KEY UPDATE refusals=IF(build_commit=VALUES(build_commit),refusals+1,1),last_error=VALUES(last_error),updated_at=VALUES(updated_at),build_commit=VALUES(build_commit)`, computerID, baseRef, errorText, time.Now().UTC(), buildinfo.Commit); err != nil {
		return false, err
	}
	quarantined, err := baseQuarantined(ctx, s.db, computerID, baseRef)
	if err != nil {
		return false, err
	}
	return quarantined, s.commitBoundary(ctx, "record projection base refusal "+computerID)
}

type singleRowQuerier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func baseRefusals(ctx context.Context, q singleRowQuerier, computerID, baseRef string) (int, error) {
	var n int
	err := q.QueryRowContext(ctx, `SELECT refusals FROM computer_projection_base_refusals WHERE computer_id=? AND base_ref=? AND build_commit=?`, computerID, baseRef, buildinfo.Commit).Scan(&n)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return n, err
}

func baseQuarantined(ctx context.Context, q singleRowQuerier, computerID, baseRef string) (bool, error) {
	n, err := baseRefusals(ctx, q, computerID, baseRef)
	return n >= baseQuarantineRefusals, err
}

// EpochRoot returns the computer's epoch root (its base and sequence), or
// an empty ref when it has none.
func (s *Store) EpochRoot(ctx context.Context, computerID string) (string, uint64, error) {
	if s == nil || s.db == nil || !safeFileCASComponent(computerID) {
		return "", 0, fmt.Errorf("epoch root: computer ID is required")
	}
	return epochRoot(ctx, s.db, computerID)
}

func epochRoot(ctx context.Context, q singleRowQuerier, computerID string) (string, uint64, error) {
	var ref string
	var seq uint64
	err := q.QueryRowContext(ctx, `SELECT base_ref,sequence FROM computer_projection_epoch_roots WHERE computer_id=?`, computerID).Scan(&ref, &seq)
	if errors.Is(err, sql.ErrNoRows) {
		return "", 0, nil
	}
	return ref, seq, err
}

func (s *Store) EnqueueProjectionJob(ctx context.Context, computerID, reason string, genesisRepair bool) (ProjectionJob, error) {
	return s.enqueueProjectionJob(ctx, computerID, reason, genesisRepair, nil)
}

// projectionTarget freezes a job at a head other than the computer's current
// one: a checkpoint's own head, which the host rebuilds to compare witnesses.
type projectionTarget struct {
	sequence uint64
	head     string
}

func (s *Store) enqueueProjectionJob(ctx context.Context, computerID, reason string, genesisRepair bool, at *projectionTarget) (ProjectionJob, error) {
	return s.enqueueProjectionJobAtBuild(ctx, computerID, reason, genesisRepair, at, "")
}

// enqueueProjectionJobAtBuild is enqueueProjectionJob for a caller that is the
// worker's own build: a `blocked` row last written by another build is
// re-admitted once, because the refusal may be the old build's fault. An empty
// build never re-admits (request traffic from other services).
func (s *Store) enqueueProjectionJobAtBuild(ctx context.Context, computerID, reason string, genesisRepair bool, at *projectionTarget, build string) (ProjectionJob, error) {
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
	if at == nil && seed != "" {
		// A seed refused twice is quarantined: seed from the next older base
		// at or above the epoch root, else from none (genesis, or the root).
		quarantined, qerr := baseQuarantined(ctx, tx, computerID, seed)
		if qerr != nil {
			return ProjectionJob{}, qerr
		}
		if quarantined {
			if seed, wm, err = checkpointBaseSeed(ctx, tx, computerID, head, wm, seed, rootSeq); err != nil {
				return ProjectionJob{}, err
			}
		}
	}
	if at == nil && rootRef != "" && (seed == "" || wm < rootSeq || genesisRepair) {
		// Never below the root: a genesis repair of this computer starts
		// from it, and a quarantined root with nothing above it is kept (its
		// job fails closed and blocks) rather than replaced by the prefix.
		seed, wm = rootRef, rootSeq
	}
	if at != nil {
		// A checkpoint's job targets the checkpoint's head, and seeds from a
		// base at or below it, at or above the epoch root.
		if at.sequence > head {
			return ProjectionJob{}, fmt.Errorf("invalid checkpoint request: target %d is beyond the head %d", at.sequence, head)
		}
		if rootRef != "" && at.sequence < rootSeq {
			return ProjectionJob{}, fmt.Errorf("%w (checkpoint at %d is below the epoch root at %d; the tape below the root is not replayed)", ErrNoVerificationBase, at.sequence, rootSeq)
		}
		var seedSequence uint64
		if seed, seedSequence, err = checkpointBaseSeed(ctx, tx, computerID, at.sequence, wm, seed, rootSeq); err != nil {
			return ProjectionJob{}, err
		}
		if rootRef != "" && seed == "" {
			seed, seedSequence = rootRef, rootSeq
		}
		if err := refuseUnreachableBase(seed, at.sequence); err != nil {
			return ProjectionJob{}, err
		}
		wm, head, target = seedSequence, at.sequence, at.head
	}
	old, err := scanProjectionJob(tx.QueryRowContext(ctx, `SELECT `+projectionJobColumns+` FROM computer_projection_jobs WHERE computer_id=? FOR UPDATE`, computerID))
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return ProjectionJob{}, err
	}
	now := time.Now().UTC()
	failures := old.Failures
	if err == nil {
		// Request traffic cannot reset a lease. A deterministic refusal waits for
		// changed inputs (seed/key); head growth alone does not admit it. Two
		// exceptions below: the worker's own build changing, and a base refusal
		// not yet confirmed by a second one (request traffic can spend that, at
		// most baseQuarantineRefusals times per seed).
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
		// A checkpoint's rebuild asks for a head of its own. A refusal or a
		// backoff of the computer's job at another target says nothing about
		// it, and must not starve it (review 329); only a refusal at the same
		// target stands.
		if at != nil && (old.TargetSequence != at.sequence || old.TargetHead != at.head) {
			failedBackingOff = false
			sameInputs = false
		}
		// A `blocked` row is also re-admitted when the worker's build changed
		// since it was written, and once more when it is a base refusal not yet
		// confirmed by a second refusal of the same seed (quarantine).
		readmit := false
		if old.Status == "blocked" && sameInputs && !retriable {
			blockedBuild := ""
			if err := tx.QueryRowContext(ctx, `SELECT build_commit FROM computer_projection_jobs WHERE computer_id=?`, computerID).Scan(&blockedBuild); err != nil {
				return ProjectionJob{}, err
			}
			readmit = build != "" && blockedBuild != build
			if !readmit && seed != "" && strings.Contains(old.Error, recoveryplan.ErrBaseRefused.Error()) {
				n, rerr := baseRefusals(ctx, tx, computerID, seed)
				if rerr != nil {
					return ProjectionJob{}, rerr
				}
				readmit = n < baseQuarantineRefusals
			}
		}
		if old.Status == "queued" || old.Status == "running" || old.Status == "retry" || (at == nil && old.Status == "succeeded" && head <= old.TargetSequence) || (old.Status == "blocked" && sameInputs && !retriable && !readmit) || failedBackingOff {
			return old, tx.Commit()
		}
		if old.SeedBaseRef != seed || old.KeyDigest != keyDigest {
			failures = 0
		}
	}
	j := ProjectionJob{ComputerID: computerID, Generation: old.Generation + 1, Status: "queued", Reason: reason, TargetSequence: head, TargetHead: target, WatermarkSequence: wm, SeedBaseRef: seed, KeyDigest: keyDigest, GenesisRepair: genesisRepair, Failures: failures, LeaseUntil: now, UpdatedAt: now}
	_, err = tx.ExecContext(ctx, `INSERT INTO computer_projection_jobs (`+projectionJobColumns+`) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?) ON DUPLICATE KEY UPDATE generation=VALUES(generation),status=VALUES(status),reason=VALUES(reason),target_sequence=VALUES(target_sequence),target_head=VALUES(target_head),watermark_sequence=VALUES(watermark_sequence),seed_base_ref=VALUES(seed_base_ref),key_digest=VALUES(key_digest),base_ref='',error_text='',genesis_repair=VALUES(genesis_repair),failures=VALUES(failures),lease_token='',lease_until=VALUES(lease_until),updated_at=VALUES(updated_at)`, j.ComputerID, j.Generation, j.Status, j.Reason, j.TargetSequence, j.TargetHead, j.WatermarkSequence, j.SeedBaseRef, j.KeyDigest, "", "", j.GenesisRepair, j.Failures, "", now, now)
	if err != nil {
		return j, err
	}
	if err = tx.Commit(); err != nil {
		return j, err
	}
	return j, s.commitBoundary(ctx, "enqueue projection checkpoint "+computerID)
}

func (s *Store) ReconcileProjectionJobs(ctx context.Context) error {
	// Typed columns only: Dolt returns COALESCE over DATETIME as untyped bytes,
	// which parseTime does not convert (staging 0f7c58ba).
	rows, err := s.db.QueryContext(ctx, `SELECT h.computer_id,h.sequence,COALESCE(w.watermark_sequence,0),w.updated_at,h.created_at FROM computer_event_heads h LEFT JOIN computer_replay_watermarks w ON w.computer_id=h.computer_id`)
	if err != nil {
		return err
	}
	var due []string
	now := time.Now().UTC()
	for rows.Next() {
		var id string
		var h, w uint64
		var advertised sql.NullTime
		var created time.Time
		if err := rows.Scan(&id, &h, &w, &advertised, &created); err != nil {
			rows.Close()
			return err
		}
		published := created
		if advertised.Valid {
			published = advertised.Time
		}
		if checkpointDue(h, w, published, now) {
			due = append(due, id)
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	// One computer that cannot be enqueued must not stop cadence for the rest
	// of the fleet; every failure is returned for the worker's heartbeat.
	var failures []error
	for _, id := range due {
		if _, err := s.enqueueProjectionJobAtBuild(ctx, id, "cadence", false, nil, buildinfo.Commit); err != nil {
			failures = append(failures, fmt.Errorf("enqueue %s: %w", id, err))
		}
	}
	s.logBlockedJobs(ctx, now)
	return errors.Join(failures...)
}

// logBlockedJobs logs each blocked row at most once per blockedLogInterval: a
// blocked row never retries and otherwise says nothing (the paper's job sat
// blocked and silent from 2026-10-15). Best effort.
func (s *Store) logBlockedJobs(ctx context.Context, now time.Time) {
	jobs, err := s.ListProjectionJobs(ctx)
	if err != nil {
		return
	}
	s.blockedLogMu.Lock()
	defer s.blockedLogMu.Unlock()
	if s.blockedLogged == nil {
		s.blockedLogged = map[string]time.Time{}
	}
	for _, j := range jobs {
		if j.Status != "blocked" {
			delete(s.blockedLogged, j.ComputerID)
			continue
		}
		if last, ok := s.blockedLogged[j.ComputerID]; ok && now.Sub(last) < blockedLogInterval {
			continue
		}
		s.blockedLogged[j.ComputerID] = now
		log.Printf("checkpoint job blocked computer=%s reason=%s failures=%d error=%q", j.ComputerID, j.Reason, j.Failures, j.Error)
	}
}

func (s *Store) ClaimProjectionJob(ctx context.Context) (*ProjectionJob, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	now := time.Now().UTC()
	j, err := scanProjectionJob(tx.QueryRowContext(ctx, `SELECT `+projectionJobColumns+` FROM computer_projection_jobs WHERE status='queued' OR (status IN ('running','retry') AND lease_until<?) ORDER BY updated_at,computer_id LIMIT 1 FOR UPDATE`, now))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	j.Status = "running"
	j.LeaseToken = uuid.NewString()
	j.LeaseUntil = now.Add(5 * time.Minute)
	if _, err := tx.ExecContext(ctx, `UPDATE computer_projection_jobs SET status='running',lease_token=?,lease_until=?,updated_at=? WHERE computer_id=? AND generation=?`, j.LeaseToken, j.LeaseUntil, now, j.ComputerID, j.Generation); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &j, s.commitBoundary(ctx, "lease projection checkpoint "+j.ComputerID)
}

func (s *Store) RenewProjectionJob(ctx context.Context, j ProjectionJob) error {
	result, err := s.db.ExecContext(ctx, `UPDATE computer_projection_jobs SET lease_until=? WHERE computer_id=? AND generation=? AND lease_token=? AND status='running'`, time.Now().UTC().Add(5*time.Minute), j.ComputerID, j.Generation, j.LeaseToken)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return fmt.Errorf("checkpoint lease lost")
	}
	return nil
}

func (s *Store) FinishProjectionJob(ctx context.Context, j ProjectionJob, status, message string, watermark uint64, baseRef string) error {
	failures := j.Failures
	if status != "succeeded" {
		failures++
	}
	result, err := s.db.ExecContext(ctx, `UPDATE computer_projection_jobs SET status=?,error_text=?,watermark_sequence=?,base_ref=?,failures=?,lease_until=?,updated_at=?,build_commit=? WHERE computer_id=? AND generation=? AND lease_token=? AND status='running'`, status, message, watermark, baseRef, failures, time.Now().UTC().Add(time.Minute), time.Now().UTC(), buildinfo.Commit, j.ComputerID, j.Generation, j.LeaseToken)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return fmt.Errorf("checkpoint lease lost")
	}
	return s.commitBoundary(ctx, "finish projection checkpoint "+j.ComputerID+" "+status)
}

func (s *Store) PinProjectionBase(ctx context.Context, computerID, baseRef, reference string) error {
	if !safeFileCASComponent(computerID) || !validFileCASDigest(baseRef) || reference == "" || len(reference) > 255 {
		return fmt.Errorf("invalid checkpoint pin")
	}
	_, err := s.db.ExecContext(ctx, `INSERT IGNORE INTO computer_projection_base_pins (computer_id,base_ref,reference_id,created_at) VALUES (?,?,?,?)`, computerID, baseRef, reference, time.Now().UTC())
	if err != nil {
		return err
	}
	return s.commitBoundary(ctx, "pin projection base "+computerID)
}

func (s *Store) projectionBasePins(ctx context.Context) (map[string]struct{}, error) {
	live := map[string]struct{}{}
	rows, err := s.db.QueryContext(ctx, `SELECT base_ref FROM computer_projection_base_pins UNION SELECT p.base_ref FROM computer_projection_bases p WHERE (SELECT COUNT(*) FROM computer_projection_bases newer WHERE newer.computer_id=p.computer_id AND (newer.sequence>p.sequence OR (newer.sequence=p.sequence AND newer.created_at>p.created_at)))<2 UNION SELECT seed_base_ref FROM computer_projection_jobs WHERE status IN ('queued','running','retry') AND seed_base_ref<>''`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var ref string
		if err := rows.Scan(&ref); err != nil {
			return nil, err
		}
		live[ref] = struct{}{}
	}
	return live, rows.Err()
}

func (s *Store) CheckpointHeartbeat(ctx context.Context, problem string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO projection_worker_status (worker_id,heartbeat,build_commit,error_text) VALUES ('checkpoint',?,?,?) ON DUPLICATE KEY UPDATE heartbeat=VALUES(heartbeat),build_commit=VALUES(build_commit),error_text=VALUES(error_text)`, time.Now().UTC(), buildinfo.Commit, problem)
	return err
}

func (s *Store) AuditCheckpointKeyUse(ctx context.Context, j ProjectionJob, keyDigest string) error {
	raw, err := json.Marshal(map[string]any{"type": "projection_checkpoint_key_use", "computer_id": j.ComputerID, "job_generation": j.Generation, "target_head": j.TargetHead, "target_sequence": j.TargetSequence, "key_digest": keyDigest, "worker": "checkpointd", "build_commit": buildinfo.Commit})
	if err != nil {
		return err
	}
	if _, _, err := s.AppendKeyEscrowTransparency(ctx, raw); err != nil {
		return err
	}
	return s.commitBoundary(ctx, "checkpoint maintenance key use "+j.ComputerID)
}

func (s *Service) PublishCheckpointResult(ctx context.Context, j ProjectionJob, sequence uint64, blobSHA256, seedBaseRef string) error {
	if s == nil || s.store == nil {
		return fmt.Errorf("platform service unavailable")
	}
	tx, err := s.store.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var token, status string
	if err := tx.QueryRowContext(ctx, `SELECT lease_token,status FROM computer_projection_jobs WHERE computer_id=? AND generation=? FOR UPDATE`, j.ComputerID, j.Generation).Scan(&token, &status); err != nil {
		return err
	}
	if token != j.LeaseToken || status != "running" {
		return fmt.Errorf("checkpoint publication lease lost")
	}
	var priorSeq uint64
	var priorRef string
	err = tx.QueryRowContext(ctx, `SELECT watermark_sequence,base_ref FROM computer_replay_watermarks WHERE computer_id=? FOR UPDATE`, j.ComputerID).Scan(&priorSeq, &priorRef)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if priorSeq > sequence {
		// A base below the advertised one: a checkpoint's verification rebuild.
		// It is not advertised, but the platform records it so it can be found
		// by sequence (the checkpoint pins it against collection).
		if _, err := tx.ExecContext(ctx, `INSERT IGNORE INTO computer_projection_bases (computer_id,base_ref,sequence,created_at) VALUES (?,?,?,?)`, j.ComputerID, blobSHA256, sequence, time.Now().UTC()); err != nil {
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		return s.store.commitBoundary(ctx, "record verification base "+j.ComputerID)
	}
	if priorRef != seedBaseRef && priorRef != blobSHA256 {
		// A quarantined advertised base is replaced by a fallback seed on
		// purpose; any other difference is a concurrent advertisement.
		quarantined := false
		if priorRef != "" {
			if quarantined, err = baseQuarantined(ctx, tx, j.ComputerID, priorRef); err != nil {
				return err
			}
		}
		if !quarantined {
			return fmt.Errorf("checkpoint seed advertisement changed")
		}
	}
	now := time.Now().UTC()
	if priorRef != "" {
		if _, err := tx.ExecContext(ctx, `INSERT IGNORE INTO computer_projection_bases (computer_id,base_ref,sequence,created_at) VALUES (?,?,?,?)`, j.ComputerID, priorRef, priorSeq, now.Add(-time.Microsecond)); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `INSERT IGNORE INTO computer_projection_bases (computer_id,base_ref,sequence,created_at) VALUES (?,?,?,?)`, j.ComputerID, blobSHA256, sequence, now); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO computer_replay_watermarks (computer_id,watermark_sequence,base_ref,updated_at) VALUES (?,?,?,?) ON DUPLICATE KEY UPDATE watermark_sequence=VALUES(watermark_sequence),base_ref=VALUES(base_ref),updated_at=VALUES(updated_at)`, j.ComputerID, sequence, blobSHA256, now); err != nil {
		return err
	}
	// A verified publish ends the computer's refusal history: whatever was
	// quarantined is superseded, and a later refusal starts a new count.
	if _, err := tx.ExecContext(ctx, `DELETE FROM computer_projection_base_refusals WHERE computer_id=?`, j.ComputerID); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return s.store.commitBoundary(ctx, "publish verified checkpoint "+j.ComputerID)
}
