package platform

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/yusefmosiah/go-choir/internal/buildinfo"
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
 updated_at DATETIME(6) NOT NULL
);
CREATE TABLE IF NOT EXISTS computer_projection_bases (
 computer_id VARCHAR(128) NOT NULL,
 base_ref VARCHAR(255) NOT NULL,
 sequence BIGINT UNSIGNED NOT NULL,
 created_at DATETIME(6) NOT NULL,
 PRIMARY KEY(computer_id,base_ref)
);
CREATE TABLE IF NOT EXISTS computer_projection_base_pins (
 computer_id VARCHAR(128) NOT NULL,
 base_ref VARCHAR(255) NOT NULL,
 reference_id VARCHAR(255) NOT NULL,
 created_at DATETIME(6) NOT NULL,
 PRIMARY KEY(computer_id,base_ref,reference_id)
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
	if err := s.db.QueryRowContext(ctx, `SELECT heartbeat,build_commit FROM projection_worker_status WHERE worker_id='checkpoint'`).Scan(&heartbeat, &j.WorkerBuildCommit); err == nil {
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
	switch {
	case j.ColdTail >= 7500:
		j.Alert = "urgent_tail"
	case j.ColdTail >= 5000:
		j.Alert = "warning_tail"
	case j.Failures >= 2:
		j.Alert = "repeated_failure"
	}
	return j, nil
}

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
	old, err := scanProjectionJob(tx.QueryRowContext(ctx, `SELECT `+projectionJobColumns+` FROM computer_projection_jobs WHERE computer_id=? FOR UPDATE`, computerID))
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return ProjectionJob{}, err
	}
	now := time.Now().UTC()
	failures := old.Failures
	if err == nil {
		// Request traffic cannot reset a lease or deterministic refusal. Dependency
		// changes (seed/key) admit a new attempt; head growth alone does not.
		if old.Status == "queued" || old.Status == "running" || old.Status == "retry" || (old.Status == "succeeded" && head <= old.TargetSequence) || ((old.Status == "blocked" || old.Status == "failed") && old.SeedBaseRef == seed && old.KeyDigest == keyDigest && old.GenesisRepair == genesisRepair) {
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
	rows, err := s.db.QueryContext(ctx, `SELECT h.computer_id,h.sequence,COALESCE(w.watermark_sequence,0),COALESCE(w.updated_at,h.created_at) FROM computer_event_heads h LEFT JOIN computer_replay_watermarks w ON w.computer_id=h.computer_id`)
	if err != nil {
		return err
	}
	var due []string
	now := time.Now().UTC()
	for rows.Next() {
		var id string
		var h, w uint64
		var published time.Time
		if err := rows.Scan(&id, &h, &w, &published); err != nil {
			rows.Close()
			return err
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
	for _, id := range due {
		if _, err := s.EnqueueProjectionJob(ctx, id, "cadence", false); err != nil {
			return err
		}
	}
	return nil
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
	result, err := s.db.ExecContext(ctx, `UPDATE computer_projection_jobs SET status=?,error_text=?,watermark_sequence=?,base_ref=?,failures=?,lease_until=?,updated_at=? WHERE computer_id=? AND generation=? AND lease_token=? AND status='running'`, status, message, watermark, baseRef, failures, time.Now().UTC().Add(time.Minute), time.Now().UTC(), j.ComputerID, j.Generation, j.LeaseToken)
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
		return tx.Commit()
	}
	if priorRef != seedBaseRef && priorRef != blobSHA256 {
		return fmt.Errorf("checkpoint seed advertisement changed")
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
	if err := tx.Commit(); err != nil {
		return err
	}
	return s.store.commitBoundary(ctx, "publish verified checkpoint "+j.ComputerID)
}
