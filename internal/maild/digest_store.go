package maild

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Digest state lives in maild's routing SQLite, beside email_owner_addresses
// (design section 4.5). The address is read from email_owner_addresses at send
// time, never copied here. The front list stays the authority for stories;
// losing these tables loses history and the subscription, not content.
const (
	digestKindScheduled = "scheduled"
	digestKindManual    = "manual"

	digestStatusStarted     = "started"
	digestStatusSent        = "sent"
	digestStatusSkipped     = "skipped"
	digestStatusFailed      = "failed"
	digestStatusInterrupted = "interrupted"

	// digestTimeFormat is fixed-width so it sorts and compares as text.
	digestTimeFormat = "2006-01-02T15:04:05.000000000Z"
)

var digestSchemaStatements = []string{
	`CREATE TABLE IF NOT EXISTS digest_subscriptions (
		owner_id text primary key,
		enabled integer not null,
		created_at text not null,
		updated_at text not null
	)`,
	`CREATE TABLE IF NOT EXISTS digest_runs (
		id text primary key,
		owner_id text not null,
		run_date text not null,
		kind text not null,
		status text not null,
		reason text not null default '',
		provider_message_id text not null default '',
		story_ids text not null default '[]',
		ranked_by text not null default '',
		created_at text not null,
		updated_at text not null
	)`,
	// One scheduled run per owner and UTC date, enforced by the table. Manual
	// (send-now) rows are limited in code to one an hour, not by this index.
	`CREATE UNIQUE INDEX IF NOT EXISTS digest_runs_scheduled_day
		ON digest_runs(owner_id, run_date) WHERE kind = 'scheduled'`,
	`CREATE INDEX IF NOT EXISTS digest_runs_owner_created ON digest_runs(owner_id, created_at)`,
	// The version of each story last sent in the scheduled series.
	`CREATE TABLE IF NOT EXISTS digest_sent (
		owner_id text not null,
		publication_id text not null,
		version integer not null,
		sent_at text not null,
		primary key (owner_id, publication_id)
	)`,
}

func digestStamp(t time.Time) string { return t.UTC().Format(digestTimeFormat) }

// DigestRun is one row of digest_runs.
type DigestRun struct {
	ID                string   `json:"-"`
	RunDate           string   `json:"run_date"`
	Kind              string   `json:"kind"`
	Status            string   `json:"status"`
	Reason            string   `json:"reason,omitempty"`
	ProviderMessageID string   `json:"provider_message_id,omitempty"`
	StoryIDs          []string `json:"story_ids"`
	RankedBy          string   `json:"ranked_by,omitempty"`
	UpdatedAt         string   `json:"updated_at"`
}

// SetDigestSubscription opts an owner in or out. Unsubscribing keeps the row
// (disabled) so the history of the choice is not lost.
func (s *Store) SetDigestSubscription(ctx context.Context, ownerID string, enabled bool, now time.Time) error {
	ownerID = strings.TrimSpace(ownerID)
	if ownerID == "" {
		return fmt.Errorf("owner_id is required")
	}
	flag := 0
	if enabled {
		flag = 1
	}
	stamp := digestStamp(now)
	_, err := s.routingDB.ExecContext(ctx, `INSERT INTO digest_subscriptions (owner_id, enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(owner_id) DO UPDATE SET enabled = excluded.enabled, updated_at = excluded.updated_at`,
		ownerID, flag, stamp, stamp)
	if err != nil {
		return fmt.Errorf("set digest subscription: %w", err)
	}
	return nil
}

// DigestSubscribed reports whether the owner has opted in.
func (s *Store) DigestSubscribed(ctx context.Context, ownerID string) (bool, error) {
	var enabled int
	err := s.routingDB.QueryRowContext(ctx, `SELECT enabled FROM digest_subscriptions WHERE owner_id = ?`,
		strings.TrimSpace(ownerID)).Scan(&enabled)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read digest subscription: %w", err)
	}
	return enabled == 1, nil
}

// ListDigestSubscribers returns the owners who are opted in, in a stable order.
func (s *Store) ListDigestSubscribers(ctx context.Context) ([]string, error) {
	rows, err := s.routingDB.QueryContext(ctx, `SELECT owner_id FROM digest_subscriptions WHERE enabled = 1 ORDER BY owner_id`)
	if err != nil {
		return nil, fmt.Errorf("list digest subscribers: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var owners []string
	for rows.Next() {
		var owner string
		if err := rows.Scan(&owner); err != nil {
			return nil, err
		}
		owners = append(owners, owner)
	}
	return owners, rows.Err()
}

// ClaimDigestRun opens a run row in status started. For a scheduled run it
// returns claimed=false when this owner already has a scheduled row for the
// date that is started or sent; a row that ended skipped, failed or
// interrupted is reopened (an explicit rerun the same day; Resend's key
// deduplicates a send that did land). Manual runs always get a new row.
func (s *Store) ClaimDigestRun(ctx context.Context, ownerID, runDate, kind string, now time.Time) (string, bool, error) {
	ownerID = strings.TrimSpace(ownerID)
	stamp := digestStamp(now)
	tx, err := s.routingDB.BeginTx(ctx, nil)
	if err != nil {
		return "", false, err
	}
	defer func() { _ = tx.Rollback() }()
	if kind == digestKindScheduled {
		var id, status string
		err := tx.QueryRowContext(ctx, `SELECT id, status FROM digest_runs WHERE owner_id = ? AND run_date = ? AND kind = 'scheduled'`,
			ownerID, runDate).Scan(&id, &status)
		switch {
		case err == nil:
			if status == digestStatusStarted || status == digestStatusSent {
				return id, false, nil
			}
			if _, err := tx.ExecContext(ctx, `UPDATE digest_runs SET status = ?, reason = '', provider_message_id = '', story_ids = '[]', ranked_by = '', updated_at = ? WHERE id = ?`,
				digestStatusStarted, stamp, id); err != nil {
				return "", false, err
			}
			return id, true, tx.Commit()
		case errors.Is(err, sql.ErrNoRows):
		default:
			return "", false, err
		}
	}
	id := newDigestRunID(ownerID, kind, now)
	if _, err := tx.ExecContext(ctx, `INSERT INTO digest_runs
		(id, owner_id, run_date, kind, status, reason, provider_message_id, story_ids, ranked_by, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, '', '', '[]', '', ?, ?)`,
		id, ownerID, runDate, kind, digestStatusStarted, stamp, stamp); err != nil {
		return "", false, fmt.Errorf("claim digest run: %w", err)
	}
	return id, true, tx.Commit()
}

// ScheduledDigestRunState returns how the owner's scheduled run for a UTC
// date last ended (exists=false when there is no row yet).
func (s *Store) ScheduledDigestRunState(ctx context.Context, ownerID, runDate string) (status, reason string, exists bool, err error) {
	err = s.routingDB.QueryRowContext(ctx, `SELECT status, reason FROM digest_runs WHERE owner_id = ? AND run_date = ? AND kind = 'scheduled'`,
		strings.TrimSpace(ownerID), runDate).Scan(&status, &reason)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return "", "", false, nil
	case err != nil:
		return "", "", false, fmt.Errorf("read scheduled digest run: %w", err)
	}
	return status, reason, true, nil
}

func newDigestRunID(ownerID, kind string, now time.Time) string {
	return fmt.Sprintf("dig_%s_%d_%s", kind, now.UTC().UnixNano(), authMailAddressHash(ownerID))
}

// FinishDigestRun records how a claimed run ended.
func (s *Store) FinishDigestRun(ctx context.Context, runID, status, reason, providerMessageID string, storyIDs []string, rankedBy string, now time.Time) error {
	if storyIDs == nil {
		storyIDs = []string{}
	}
	ids, _ := json.Marshal(storyIDs)
	_, err := s.routingDB.ExecContext(ctx, `UPDATE digest_runs
		SET status = ?, reason = ?, provider_message_id = ?, story_ids = ?, ranked_by = ?, updated_at = ?
		WHERE id = ?`, status, reason, providerMessageID, string(ids), rankedBy, digestStamp(now), runID)
	if err != nil {
		return fmt.Errorf("finish digest run: %w", err)
	}
	return nil
}

// CloseOpenDigestRuns marks every run still started as interrupted. Called
// once at boot: a crash never resumes work (CLAUDE.md "Restarts End Work"),
// it closes it with a visible fate. Nothing is sent.
func (s *Store) CloseOpenDigestRuns(ctx context.Context, now time.Time) (int, error) {
	res, err := s.routingDB.ExecContext(ctx, `UPDATE digest_runs SET status = ?, reason = 'interrupted by a restart', updated_at = ? WHERE status = ?`,
		digestStatusInterrupted, digestStamp(now), digestStatusStarted)
	if err != nil {
		return 0, fmt.Errorf("close open digest runs: %w", err)
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// LatestDigestRuns returns the owner's most recent runs, newest first.
func (s *Store) LatestDigestRuns(ctx context.Context, ownerID string, limit int) ([]DigestRun, error) {
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	rows, err := s.routingDB.QueryContext(ctx, `SELECT id, run_date, kind, status, reason, provider_message_id, story_ids, ranked_by, updated_at
		FROM digest_runs WHERE owner_id = ? ORDER BY created_at DESC, updated_at DESC LIMIT ?`, strings.TrimSpace(ownerID), limit)
	if err != nil {
		return nil, fmt.Errorf("list digest runs: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var runs []DigestRun
	for rows.Next() {
		var run DigestRun
		var ids string
		if err := rows.Scan(&run.ID, &run.RunDate, &run.Kind, &run.Status, &run.Reason, &run.ProviderMessageID, &ids, &run.RankedBy, &run.UpdatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(ids), &run.StoryIDs)
		if run.StoryIDs == nil {
			run.StoryIDs = []string{}
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

// RecentManualDigestSend reports whether the owner has a manual run that sent
// (or is sending) at or after since: the send-now rate limit.
func (s *Store) RecentManualDigestSend(ctx context.Context, ownerID string, since time.Time) (bool, error) {
	var n int
	err := s.routingDB.QueryRowContext(ctx, `SELECT count(*) FROM digest_runs
		WHERE owner_id = ? AND kind = 'manual' AND status IN ('started','sent') AND created_at >= ?`,
		strings.TrimSpace(ownerID), digestStamp(since)).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("read manual digest sends: %w", err)
	}
	return n > 0, nil
}

// DigestSentVersions returns the latest revision of each story already sent to
// the owner in the scheduled series.
func (s *Store) DigestSentVersions(ctx context.Context, ownerID string) (map[string]int, error) {
	rows, err := s.routingDB.QueryContext(ctx, `SELECT publication_id, version FROM digest_sent WHERE owner_id = ?`, strings.TrimSpace(ownerID))
	if err != nil {
		return nil, fmt.Errorf("read digest ledger: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := map[string]int{}
	for rows.Next() {
		var id string
		var version int
		if err := rows.Scan(&id, &version); err != nil {
			return nil, err
		}
		out[id] = version
	}
	return out, rows.Err()
}

// RecordDigestSent notes the revisions just sent. The ledger only moves up.
func (s *Store) RecordDigestSent(ctx context.Context, ownerID string, versions map[string]int, now time.Time) error {
	tx, err := s.routingDB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	stamp := digestStamp(now)
	for id, version := range versions {
		if _, err := tx.ExecContext(ctx, `INSERT INTO digest_sent (owner_id, publication_id, version, sent_at)
			VALUES (?, ?, ?, ?)
			ON CONFLICT(owner_id, publication_id) DO UPDATE SET
				version = max(digest_sent.version, excluded.version), sent_at = excluded.sent_at`,
			strings.TrimSpace(ownerID), id, version, stamp); err != nil {
			return fmt.Errorf("record digest sent: %w", err)
		}
	}
	return tx.Commit()
}
