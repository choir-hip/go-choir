package auth

// Account deletion: the durable record (docs/design/account-create-delete-
// 2026-11-23.md section C, with the owner's rulings of 2026-11-23).
//
// One row of account_deletions per account that asked to be deleted. It is
// written in the same transaction that revokes every session and key of the
// account, so "access is revoked at once" and "a deletion record exists" are
// one fact. The irreversible purge waits purgeguard.GracePeriod (72 hours)
// after requested_at; signing in with a passkey inside that time offers the
// cancel path and nothing else; the purge itself (internal/accountpurge) is a
// loop that re-reads this table, never a memory of a boot.
//
// States: pending (access revoked, nothing destroyed, cancellable) and
// purging (the destructive steps have begun; cancel is refused). The row and
// the account's user row go in one transaction at the end, which also writes
// the receipt.

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/yusefmosiah/go-choir/internal/purgeguard"
)

const deletionSchemaDDL = `
CREATE TABLE IF NOT EXISTS account_deletions (
	user_id              TEXT PRIMARY KEY,
	job_id               TEXT NOT NULL UNIQUE,
	state                TEXT NOT NULL CHECK(state IN ('pending', 'purging')),
	requested_at         DATETIME NOT NULL,
	purge_after          DATETIME NOT NULL,
	computers            TEXT NOT NULL DEFAULT '[]',
	steps                TEXT NOT NULL DEFAULT '{}',
	cancel_token_hash    TEXT NOT NULL DEFAULT '',
	cancel_token_expires DATETIME,
	attempt_started_at   DATETIME,
	attempts             INTEGER NOT NULL DEFAULT 0,
	last_error           TEXT NOT NULL DEFAULT '',
	updated_at           DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_account_deletions_purge_after ON account_deletions(purge_after);

-- A fresh passkey assertion starts a deletion. The ceremony has its own table
-- so a deletion challenge can never be finished as a sign-in (login finish
-- reads challenge_state only) and a sign-in challenge can never start a
-- deletion.
CREATE TABLE IF NOT EXISTS deletion_challenges (
	id           TEXT PRIMARY KEY,
	user_id      TEXT NOT NULL,
	session_data TEXT NOT NULL,
	created_at   DATETIME NOT NULL,
	expires_at   DATETIME NOT NULL
);

-- What is kept of a deleted account: an id, times and counts. user_hash is a
-- hash of the opaque user id; no email, no content.
CREATE TABLE IF NOT EXISTS account_deletion_receipts (
	id           TEXT PRIMARY KEY,
	job_id       TEXT NOT NULL,
	user_hash    TEXT NOT NULL,
	requested_at DATETIME NOT NULL,
	purged_at    DATETIME NOT NULL,
	summary      TEXT NOT NULL
);
`

// Deletion states.
const (
	DeletionPending = "pending"
	DeletionPurging = "purging"
)

// Errors of the deletion store.
var (
	// ErrDeletionNotCancellable means the purge has begun.
	ErrDeletionNotCancellable = errors.New("auth: the deletion can no longer be cancelled")
	// ErrNoDeletion means the account has no deletion record.
	ErrNoDeletion = errors.New("auth: the account has no deletion record")
	// ErrDeletionPending is returned wherever a session or key would be
	// issued for an account whose deletion is pending.
	ErrDeletionPending = errors.New("auth: the account is being deleted")
)

// DeletionStep is one step's record in the job.
type DeletionStep struct {
	DoneAt  time.Time        `json:"done_at"`
	Counts  map[string]int64 `json:"counts,omitempty"`
	Refused string           `json:"refused,omitempty"` // named, not hidden
}

// AccountDeletion is the job row.
type AccountDeletion struct {
	UserID       string
	JobID        string
	State        string
	RequestedAt  time.Time
	PurgeAfter   time.Time
	Computers    []string
	Steps        map[string]DeletionStep
	AttemptStart *time.Time
	Attempts     int
	LastError    string
	UpdatedAt    time.Time
}

// DeletionReceipt is what remains of a purged account.
type DeletionReceipt struct {
	ID          string
	JobID       string
	UserHash    string
	RequestedAt time.Time
	PurgedAt    time.Time
	Summary     string
}

// DeletionUserHash is the hash a receipt keeps in place of the user id.
func DeletionUserHash(userID string) string {
	sum := sha256.Sum256([]byte("account-deletion-receipt\x00" + strings.TrimSpace(userID)))
	return hex.EncodeToString(sum[:])
}

func (s *Store) migrateDeletion() error {
	if _, err := s.db.Exec(deletionSchemaDDL); err != nil {
		return fmt.Errorf("apply deletion schema: %w", err)
	}
	return nil
}

const deletionColumns = `user_id, job_id, state, requested_at, purge_after, computers, steps, attempt_started_at, attempts, last_error, updated_at`

func scanDeletion(row interface{ Scan(...any) error }) (*AccountDeletion, error) {
	d := &AccountDeletion{}
	var computers, steps string
	var attempt sql.NullTime
	if err := row.Scan(&d.UserID, &d.JobID, &d.State, &d.RequestedAt, &d.PurgeAfter, &computers, &steps, &attempt, &d.Attempts, &d.LastError, &d.UpdatedAt); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(computers), &d.Computers); err != nil {
		return nil, fmt.Errorf("deletion computers: %w", err)
	}
	d.Steps = map[string]DeletionStep{}
	if err := json.Unmarshal([]byte(steps), &d.Steps); err != nil {
		return nil, fmt.Errorf("deletion steps: %w", err)
	}
	if attempt.Valid {
		t := attempt.Time
		d.AttemptStart = &t
	}
	return d, nil
}

// AccountDeletionFor returns the account's deletion record, or nil.
func (s *Store) AccountDeletionFor(userID string) (*AccountDeletion, error) {
	d, err := scanDeletion(s.db.QueryRow(`SELECT `+deletionColumns+` FROM account_deletions WHERE user_id = ?`, userID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return d, err
}

// AccountAccessRevoked reports whether the account has a deletion record,
// pending or purging. It is the one question every session, key and token
// check asks. A read error answers true: the check fails closed.
func (s *Store) AccountAccessRevoked(userID string) bool {
	if strings.TrimSpace(userID) == "" {
		return false
	}
	var n int
	if err := s.db.QueryRow(`SELECT 1 FROM account_deletions WHERE user_id = ?`, userID).Scan(&n); err != nil {
		return !errors.Is(err, sql.ErrNoRows)
	}
	return true
}

// RequestAccountDeletion writes the deletion record and revokes the account's
// access in one transaction: every refresh session, desktop exchange code,
// device authorisation, email sign-in attempt, recovery token, sign-in
// ceremony and account-switcher membership is deleted and every API key is
// revoked. The passkeys stay: they are how the person comes back to cancel.
// A second request returns the record that exists (idempotent).
func (s *Store) RequestAccountDeletion(userID string, now time.Time) (*AccountDeletion, bool, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, false, errors.New("auth: deletion needs a user id")
	}
	now = now.UTC()
	tx, err := s.db.Begin()
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = tx.Rollback() }()
	var exists int
	if err := tx.QueryRow(`SELECT 1 FROM users WHERE id = ?`, userID).Scan(&exists); err != nil {
		return nil, false, fmt.Errorf("auth: deletion: user: %w", err)
	}
	if d, err := scanDeletion(tx.QueryRow(`SELECT `+deletionColumns+` FROM account_deletions WHERE user_id = ?`, userID)); err == nil {
		return d, false, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, false, err
	}
	d := &AccountDeletion{
		UserID: userID, JobID: "del-" + uuid.NewString(), State: DeletionPending,
		RequestedAt: now, PurgeAfter: now.Add(purgeguard.GracePeriod),
		Computers: []string{}, Steps: map[string]DeletionStep{}, UpdatedAt: now,
	}
	if _, err := tx.Exec(`INSERT INTO account_deletions (user_id, job_id, state, requested_at, purge_after, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		d.UserID, d.JobID, d.State, d.RequestedAt, d.PurgeAfter, d.UpdatedAt); err != nil {
		return nil, false, fmt.Errorf("auth: deletion: insert: %w", err)
	}
	for _, stmt := range []string{
		`DELETE FROM refresh_sessions WHERE user_id = ?`,
		`DELETE FROM desktop_exchange_codes WHERE user_id = ?`,
		`DELETE FROM device_authorizations WHERE user_id = ?`,
		`DELETE FROM email_signin_attempts WHERE user_id = ?`,
		`DELETE FROM recovery_tokens WHERE user_id = ?`,
		`DELETE FROM challenge_state WHERE user_id = ?`,
		`DELETE FROM deletion_challenges WHERE user_id = ?`,
		`DELETE FROM account_set_member WHERE user_id = ?`,
	} {
		if _, err := tx.Exec(stmt, userID); err != nil {
			return nil, false, fmt.Errorf("auth: deletion: revoke: %w", err)
		}
	}
	if _, err := tx.Exec(`UPDATE api_keys SET revoked_at = ? WHERE user_id = ? AND revoked_at IS NULL`, now, userID); err != nil {
		return nil, false, fmt.Errorf("auth: deletion: revoke keys: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, false, err
	}
	return d, true, nil
}

// SaveDeletionChallenge stores a deletion ceremony's challenge.
func (s *Store) SaveDeletionChallenge(id, userID, sessionData string, created, expires time.Time) error {
	_, err := s.db.Exec(`INSERT INTO deletion_challenges (id, user_id, session_data, created_at, expires_at) VALUES (?, ?, ?, ?, ?)`,
		id, userID, sessionData, created.UTC(), expires.UTC())
	return err
}

// TakeDeletionChallenge reads and deletes a deletion challenge in one
// transaction: a challenge answers once.
func (s *Store) TakeDeletionChallenge(id string) (userID, sessionData string, created, expires time.Time, err error) {
	tx, err := s.db.Begin()
	if err != nil {
		return "", "", time.Time{}, time.Time{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if err = tx.QueryRow(`SELECT user_id, session_data, created_at, expires_at FROM deletion_challenges WHERE id = ?`, id).
		Scan(&userID, &sessionData, &created, &expires); err != nil {
		return "", "", time.Time{}, time.Time{}, err
	}
	if _, err = tx.Exec(`DELETE FROM deletion_challenges WHERE id = ?`, id); err != nil {
		return "", "", time.Time{}, time.Time{}, err
	}
	if err = tx.Commit(); err != nil {
		return "", "", time.Time{}, time.Time{}, err
	}
	return userID, sessionData, created, expires, nil
}

// DeletionCancelTokenTTL is how long the cancel offer made by a passkey
// sign-in stays open.
const DeletionCancelTokenTTL = 10 * time.Minute

func hashCancelToken(token string) string {
	sum := sha256.Sum256([]byte("deletion-cancel\x00" + token))
	return hex.EncodeToString(sum[:])
}

// OfferDeletionCancel stores a one-use cancel token for a pending deletion
// and returns it. Only a successful passkey assertion may call it.
func (s *Store) OfferDeletionCancel(userID string, now time.Time) (token string, err error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	token = base64.RawURLEncoding.EncodeToString(buf)
	res, err := s.db.Exec(`UPDATE account_deletions SET cancel_token_hash = ?, cancel_token_expires = ?, updated_at = ? WHERE user_id = ? AND state = ?`,
		hashCancelToken(token), now.UTC().Add(DeletionCancelTokenTTL), now.UTC(), userID, DeletionPending)
	if err != nil {
		return "", err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return "", ErrDeletionNotCancellable
	}
	return token, nil
}

// PeekCancelOffer checks a cancel token without consuming it and returns the
// pending record it belongs to. The cancel handler restores the account's
// takes between the peek and the cancel, so a failed restore leaves the
// record, and the person can try again, instead of a cancelled record with
// takes still held back.
func (s *Store) PeekCancelOffer(token string, now time.Time) (*AccountDeletion, error) {
	if strings.TrimSpace(token) == "" {
		return nil, ErrNoDeletion
	}
	var exp sql.NullTime
	var userID, state string
	err := s.db.QueryRow(`SELECT user_id, state, cancel_token_expires FROM account_deletions WHERE cancel_token_hash = ? AND cancel_token_hash != ''`, hashCancelToken(token)).
		Scan(&userID, &state, &exp)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNoDeletion
	}
	if err != nil {
		return nil, err
	}
	if !exp.Valid || now.UTC().After(exp.Time) {
		return nil, ErrNoDeletion
	}
	if state != DeletionPending {
		return nil, ErrDeletionNotCancellable
	}
	return s.AccountDeletionFor(userID)
}

// CancelAccountDeletion consumes a cancel token and removes the pending
// record. Nothing was purged, so nothing is restored here: the account's
// passkeys and rows are as they were; its sessions and keys stay revoked and
// the caller signs in again. It returns the user id.
func (s *Store) CancelAccountDeletion(token string, now time.Time) (*AccountDeletion, error) {
	if strings.TrimSpace(token) == "" {
		return nil, ErrNoDeletion
	}
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	var exp sql.NullTime
	var userID, state string
	err = tx.QueryRow(`SELECT user_id, state, cancel_token_expires FROM account_deletions WHERE cancel_token_hash = ? AND cancel_token_hash != ''`, hashCancelToken(token)).
		Scan(&userID, &state, &exp)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNoDeletion
	}
	if err != nil {
		return nil, err
	}
	if !exp.Valid || now.UTC().After(exp.Time) {
		return nil, ErrNoDeletion
	}
	if state != DeletionPending {
		return nil, ErrDeletionNotCancellable
	}
	d, err := scanDeletion(tx.QueryRow(`SELECT `+deletionColumns+` FROM account_deletions WHERE user_id = ?`, userID))
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(`DELETE FROM account_deletions WHERE user_id = ? AND state = ?`, userID, DeletionPending); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return d, nil
}

// DueAccountDeletions lists records whose grace period has ended.
func (s *Store) DueAccountDeletions(now time.Time, limit int) ([]*AccountDeletion, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.Query(`SELECT `+deletionColumns+` FROM account_deletions WHERE purge_after <= ? ORDER BY purge_after ASC LIMIT ?`, now.UTC(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*AccountDeletion
	for rows.Next() {
		d, err := scanDeletion(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// OpenAccountDeletions lists every record, due or not (the at-once steps
// retry until they are recorded).
func (s *Store) OpenAccountDeletions(limit int) ([]*AccountDeletion, error) {
	if limit <= 0 {
		limit = 200
	}
	rows, err := s.db.Query(`SELECT `+deletionColumns+` FROM account_deletions ORDER BY requested_at ASC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*AccountDeletion
	for rows.Next() {
		d, err := scanDeletion(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// PurgeEligible is the rule every destructive step asks: the record exists
// and is at least the grace period old.
func (s *Store) PurgeEligible(userID string, now time.Time) error {
	return purgeguard.Eligible(s.db, userID, now)
}

// BeginPurge moves a record whose grace period has ended to purging, after
// which it can no longer be cancelled. It records the attempt: an attempt
// still open from before (a restart interrupted it) is closed as such by the
// caller, never resumed silently.
func (s *Store) BeginPurge(userID string, now time.Time) (*AccountDeletion, error) {
	if err := s.PurgeEligible(userID, now); err != nil {
		return nil, err
	}
	res, err := s.db.Exec(`UPDATE account_deletions SET state = ?, cancel_token_hash = '', cancel_token_expires = NULL, attempt_started_at = ?, attempts = attempts + 1, updated_at = ? WHERE user_id = ? AND purge_after <= ?`,
		DeletionPurging, now.UTC(), now.UTC(), userID, now.UTC())
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, purgeguard.ErrTooEarly
	}
	return s.AccountDeletionFor(userID)
}

// EndAttempt closes the open attempt, recording an error if there was one.
func (s *Store) EndAttempt(userID string, lastErr string, now time.Time) error {
	if len(lastErr) > 500 {
		lastErr = lastErr[:500]
	}
	_, err := s.db.Exec(`UPDATE account_deletions SET attempt_started_at = NULL, last_error = ?, updated_at = ? WHERE user_id = ?`, lastErr, now.UTC(), userID)
	return err
}

// RecordDeletionComputers stores the computers found for the account.
func (s *Store) RecordDeletionComputers(userID string, computers []string, now time.Time) error {
	raw, _ := json.Marshal(computers)
	_, err := s.db.Exec(`UPDATE account_deletions SET computers = ?, updated_at = ? WHERE user_id = ?`, string(raw), now.UTC(), userID)
	return err
}

// RecordDeletionStep stores one step's result.
func (s *Store) RecordDeletionStep(userID, step string, rec DeletionStep) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var raw string
	if err := tx.QueryRow(`SELECT steps FROM account_deletions WHERE user_id = ?`, userID).Scan(&raw); err != nil {
		return err
	}
	steps := map[string]DeletionStep{}
	if err := json.Unmarshal([]byte(raw), &steps); err != nil {
		return err
	}
	steps[step] = rec
	out, _ := json.Marshal(steps)
	if _, err := tx.Exec(`UPDATE account_deletions SET steps = ?, updated_at = ? WHERE user_id = ?`, string(out), rec.DoneAt.UTC(), userID); err != nil {
		return err
	}
	return tx.Commit()
}

// FinalizeAccountDeletion is the last step of a purge: in one transaction it
// re-checks the rule, deletes everything auth holds for the account (the
// owner root key and its wraps, recovery tokens by address, the waitlist
// entry, the user row and every row that cascades from it, the deletion
// job), and writes the receipt. The sign-up address is free from this
// commit. summary is JSON of counts only.
func (s *Store) FinalizeAccountDeletion(userID, summary string, now time.Time) (*DeletionReceipt, error) {
	if err := s.PurgeEligible(userID, now); err != nil {
		return nil, err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	d, err := scanDeletion(tx.QueryRow(`SELECT `+deletionColumns+` FROM account_deletions WHERE user_id = ?`, userID))
	if err != nil {
		return nil, err
	}
	if d.State != DeletionPurging {
		return nil, fmt.Errorf("auth: finalize: the deletion is %s, not purging", d.State)
	}
	var email string
	if err := tx.QueryRow(`SELECT email FROM users WHERE id = ?`, userID).Scan(&email); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if email != "" {
		for _, stmt := range []string{
			`DELETE FROM recovery_tokens WHERE email = ?`,
			`DELETE FROM registration_waitlist WHERE email = ?`,
		} {
			if _, err := tx.Exec(stmt, email); err != nil {
				return nil, fmt.Errorf("auth: finalize: %w", err)
			}
		}
	}
	for _, stmt := range []string{
		`DELETE FROM owner_root_key_wraps WHERE owner_id = ?`,
		`DELETE FROM owner_root_keys WHERE owner_id = ?`,
		`DELETE FROM users WHERE id = ?`,
		`DELETE FROM account_deletions WHERE user_id = ?`,
	} {
		if _, err := tx.Exec(stmt, userID); err != nil {
			return nil, fmt.Errorf("auth: finalize: %w", err)
		}
	}
	rec := &DeletionReceipt{ID: "rcpt-" + uuid.NewString(), JobID: d.JobID, UserHash: DeletionUserHash(userID),
		RequestedAt: d.RequestedAt, PurgedAt: now.UTC(), Summary: summary}
	if _, err := tx.Exec(`INSERT INTO account_deletion_receipts (id, job_id, user_hash, requested_at, purged_at, summary) VALUES (?, ?, ?, ?, ?, ?)`,
		rec.ID, rec.JobID, rec.UserHash, rec.RequestedAt, rec.PurgedAt, rec.Summary); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return rec, nil
}

// DeletionReceiptByJob returns the receipt of a purged job.
func (s *Store) DeletionReceiptByJob(jobID string) (*DeletionReceipt, error) {
	r := &DeletionReceipt{}
	err := s.db.QueryRow(`SELECT id, job_id, user_hash, requested_at, purged_at, summary FROM account_deletion_receipts WHERE job_id = ?`, jobID).
		Scan(&r.ID, &r.JobID, &r.UserHash, &r.RequestedAt, &r.PurgedAt, &r.Summary)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return r, err
}

// UserCredentialIDs is a convenience for tests and the finish handler.
func (s *Store) deletionCredentialCount(ctx context.Context, userID string) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM credentials WHERE user_id = ?`, userID).Scan(&n)
	return n, err
}
