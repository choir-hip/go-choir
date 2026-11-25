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
// States: pending (access revoked, nothing destroyed, cancellable), purging
// (the destructive steps have begun; cancel is refused) and cancelled. A
// cancel does not delete the row: it leaves this tombstone, which answers as
// no record to every check (AccountDeletionFor, AccountAccessRevoked,
// purgeguard) and tells the runner that the account's takes must be served
// again; the runner deletes the row once they are. The row and the account's
// user row go in one transaction at the end of a purge, which also writes the
// receipt.

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/yusefmosiah/go-choir/internal/purgeguard"
)

const deletionSchemaDDL = `
CREATE TABLE IF NOT EXISTS account_deletions (
	user_id              TEXT PRIMARY KEY,
	job_id               TEXT NOT NULL UNIQUE,
	state                TEXT NOT NULL CHECK(state IN ('pending', 'purging', 'cancelled')),
	requested_at         DATETIME NOT NULL,
	purge_after          DATETIME NOT NULL,
	computers            TEXT NOT NULL DEFAULT '[]',
	vm_ids               TEXT NOT NULL DEFAULT '[]',
	destroy_computers    TEXT NOT NULL DEFAULT '[]',
	destroy_vm_ids       TEXT NOT NULL DEFAULT '[]',
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
	DeletionPending   = "pending"
	DeletionPurging   = "purging"
	DeletionCancelled = "cancelled"
)

// liveDeletion is the SQL for a record that counts: pending or purging.
const liveDeletion = "state IN ('pending', 'purging')"

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
	UserID      string
	JobID       string
	State       string
	RequestedAt time.Time
	PurgeAfter  time.Time
	Computers   []string
	// VMIDs are the VM ids of those computers: the gateway keys a credential by
	// the VM it authenticates, not by the product computer.
	VMIDs []string
	// DestroyComputers and DestroyVMIDs are the computers (and their VM ids)
	// the purge acts on: the ones vmctl listed as the account's own when the
	// purge began, plus any it reported destroying. The gateway and the tape
	// are driven from this set alone, never from Computers (the list at the
	// time of the request): a computer that left the account inside the 72
	// hours is not in it, so its gateway bearer and tape are not touched.
	DestroyComputers []string
	DestroyVMIDs     []string
	Steps            map[string]DeletionStep
	AttemptStart     *time.Time
	Attempts         int
	LastError        string
	UpdatedAt        time.Time
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

const deletionColumns = `user_id, job_id, state, requested_at, purge_after, computers, vm_ids, destroy_computers, destroy_vm_ids, steps, attempt_started_at, attempts, last_error, updated_at`

func scanDeletion(row interface{ Scan(...any) error }) (*AccountDeletion, error) {
	d := &AccountDeletion{}
	var computers, vmIDs, destroyComputers, destroyVMIDs, steps string
	var attempt sql.NullTime
	if err := row.Scan(&d.UserID, &d.JobID, &d.State, &d.RequestedAt, &d.PurgeAfter, &computers, &vmIDs, &destroyComputers, &destroyVMIDs, &steps, &attempt, &d.Attempts, &d.LastError, &d.UpdatedAt); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(computers), &d.Computers); err != nil {
		return nil, fmt.Errorf("deletion computers: %w", err)
	}
	if err := json.Unmarshal([]byte(vmIDs), &d.VMIDs); err != nil {
		return nil, fmt.Errorf("deletion vm ids: %w", err)
	}
	if err := json.Unmarshal([]byte(destroyComputers), &d.DestroyComputers); err != nil {
		return nil, fmt.Errorf("deletion destroy computers: %w", err)
	}
	if err := json.Unmarshal([]byte(destroyVMIDs), &d.DestroyVMIDs); err != nil {
		return nil, fmt.Errorf("deletion destroy vm ids: %w", err)
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

// AccountDeletionFor returns the account's live deletion record (pending or
// purging), or nil. A cancelled tombstone is no record.
func (s *Store) AccountDeletionFor(userID string) (*AccountDeletion, error) {
	d, err := scanDeletion(s.db.QueryRow(`SELECT `+deletionColumns+` FROM account_deletions WHERE user_id = ? AND `+liveDeletion, userID))
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
	if err := s.db.QueryRow(`SELECT 1 FROM account_deletions WHERE user_id = ? AND `+liveDeletion, userID).Scan(&n); err != nil {
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
	if d, err := scanDeletion(tx.QueryRow(`SELECT `+deletionColumns+` FROM account_deletions WHERE user_id = ? AND `+liveDeletion, userID)); err == nil {
		return d, false, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, false, err
	}
	// A tombstone of an earlier cancel is replaced: the new request is the
	// account's only record, and it suspends the takes itself.
	if _, err := tx.Exec(`DELETE FROM account_deletions WHERE user_id = ? AND state = ?`, userID, DeletionCancelled); err != nil {
		return nil, false, fmt.Errorf("auth: deletion: replace tombstone: %w", err)
	}
	d := &AccountDeletion{
		UserID: userID, JobID: "del-" + uuid.NewString(), State: DeletionPending,
		RequestedAt: now, PurgeAfter: now.Add(purgeguard.GracePeriod),
		Computers: []string{}, VMIDs: []string{}, DestroyComputers: []string{}, DestroyVMIDs: []string{}, Steps: map[string]DeletionStep{}, UpdatedAt: now,
	}
	if _, err := tx.Exec(`INSERT INTO account_deletions (user_id, job_id, state, requested_at, purge_after, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		d.UserID, d.JobID, d.State, d.RequestedAt, d.PurgeAfter, d.UpdatedAt); err != nil {
		return nil, false, fmt.Errorf("auth: deletion: insert: %w", err)
	}
	// The browsers' account switchers: remember which sets the account was in,
	// so the set's active account is repaired after the membership goes.
	var sets []string
	if rows, err := tx.Query(`SELECT set_id FROM account_set_member WHERE user_id = ?`, userID); err != nil {
		return nil, false, fmt.Errorf("auth: deletion: sets: %w", err)
	} else {
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err == nil {
				sets = append(sets, id)
			}
		}
		_ = rows.Close()
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
	for _, setID := range sets {
		if err := repairSetTx(tx, setID); err != nil {
			return nil, false, fmt.Errorf("auth: deletion: repair account set: %w", err)
		}
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
// and returns it. Only a successful passkey assertion may call it. The state
// is the boundary, not the clock: a pending record can be cancelled, a
// purging one cannot. A record past its purge time that the runner has not
// begun (a preflight that cannot pass, a runner that is behind) has had
// nothing removed, so it is still offered the cancel; BeginPurge, which needs
// the deadline and the state in one UPDATE, is what closes it, and of a cancel
// and a BeginPurge racing for the same pending row exactly one wins.
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

// cancellableByToken reads the pending record a token belongs to and applies
// the two rules of a cancel: the token is unexpired and the record is pending
// (the state is the boundary; see OfferDeletionCancel).
func cancellableByToken(q interface {
	QueryRow(string, ...any) *sql.Row
}, token string, now time.Time) (*AccountDeletion, error) {
	if strings.TrimSpace(token) == "" {
		return nil, ErrNoDeletion
	}
	var exp sql.NullTime
	var userID, state string
	err := q.QueryRow(`SELECT user_id, state, cancel_token_expires FROM account_deletions WHERE cancel_token_hash = ? AND cancel_token_hash != '' AND `+liveDeletion, hashCancelToken(token)).
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
	d, err := scanDeletion(q.QueryRow(`SELECT `+deletionColumns+` FROM account_deletions WHERE user_id = ? AND `+liveDeletion, userID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNoDeletion // a concurrent cancel took it
	}
	if err != nil {
		return nil, err
	}
	return d, nil
}

// PeekCancelOffer checks a cancel token without consuming it and returns the
// pending record it belongs to; never (nil, nil).
func (s *Store) PeekCancelOffer(token string, now time.Time) (*AccountDeletion, error) {
	return cancellableByToken(s.db, token, now)
}

// CancelAccountDeletion consumes a cancel token. It does not delete the
// record: it moves it to cancelled, a tombstone that answers as no record
// everywhere (the account is signed in again at once) and that the purge
// runner finishes by serving the account's takes again and then deleting the
// row (FinishCancelledDeletion). A pass that was holding the takes back when
// the cancel landed therefore always finds the tombstone and undoes itself.
// Nothing was purged, so nothing else is restored: the account's sessions and
// keys stay revoked and the caller signs in again. It returns the record as
// it was.
func (s *Store) CancelAccountDeletion(token string, now time.Time) (*AccountDeletion, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	d, err := cancellableByToken(tx, token, now)
	if err != nil {
		return nil, err
	}
	res, err := tx.Exec(`UPDATE account_deletions SET state = ?, cancel_token_hash = '', cancel_token_expires = NULL, updated_at = ? WHERE user_id = ? AND state = ?`,
		DeletionCancelled, now.UTC(), d.UserID, DeletionPending)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrNoDeletion
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return d, nil
}

// FinishCancelledDeletion deletes the tombstone of a cancelled deletion once
// the account's takes are served again.
func (s *Store) FinishCancelledDeletion(userID string) error {
	_, err := s.db.Exec(`DELETE FROM account_deletions WHERE user_id = ? AND state = ?`, userID, DeletionCancelled)
	return err
}

// DueAccountDeletions lists records whose grace period has ended.
func (s *Store) DueAccountDeletions(now time.Time, limit int) ([]*AccountDeletion, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.Query(`SELECT `+deletionColumns+` FROM account_deletions WHERE purge_after <= ? AND `+liveDeletion+` ORDER BY purge_after ASC LIMIT ?`, now.UTC(), limit)
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

// OpenAccountDeletions lists every record, due or not, tombstones included
// (the at-once steps retry until they are recorded). A limit of zero is no
// limit: a fixed batch would let a stuck record at the front starve every
// record behind it.
func (s *Store) OpenAccountDeletions(limit int) ([]*AccountDeletion, error) {
	if limit <= 0 {
		limit = -1 // SQLite: no limit
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
	res, err := s.db.Exec(`UPDATE account_deletions SET state = ?, cancel_token_hash = '', cancel_token_expires = NULL, attempt_started_at = ?, attempts = attempts + 1, updated_at = ? WHERE user_id = ? AND purge_after <= ? AND `+liveDeletion,
		DeletionPurging, now.UTC(), now.UTC(), userID, now.UTC())
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, purgeguard.ErrTooEarly
	}
	return s.AccountDeletionFor(userID)
}

// ReturnToPending undoes BeginPurge for a purge that halted before its first
// removal: the record goes back to pending, so the person can cancel again.
// It refuses when any step other than the reversible at-once one is recorded,
// since then something has been removed and the purge must only go forward.
func (s *Store) ReturnToPending(userID string, now time.Time) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var raw string
	if err := tx.QueryRow(`SELECT steps FROM account_deletions WHERE user_id = ? AND state = ?`, userID, DeletionPurging).Scan(&raw); err != nil {
		return err
	}
	steps := map[string]DeletionStep{}
	if err := json.Unmarshal([]byte(raw), &steps); err != nil {
		return err
	}
	for name := range steps {
		if name != "at_once" {
			return fmt.Errorf("auth: the purge has done step %q; it cannot return to pending", name)
		}
	}
	if _, err := tx.Exec(`UPDATE account_deletions SET state = ?, updated_at = ? WHERE user_id = ? AND state = ?`, DeletionPending, now.UTC(), userID, DeletionPurging); err != nil {
		return err
	}
	return tx.Commit()
}

// EndAttempt closes the open attempt, recording an error if there was one.
func (s *Store) EndAttempt(userID string, lastErr string, now time.Time) error {
	if len(lastErr) > 500 {
		lastErr = lastErr[:500]
	}
	_, err := s.db.Exec(`UPDATE account_deletions SET attempt_started_at = NULL, last_error = ?, updated_at = ? WHERE user_id = ?`, lastErr, now.UTC(), userID)
	return err
}

// RecordDeletionComputers stores the computers (and their VM ids) found for
// the account.
func (s *Store) RecordDeletionComputers(userID string, computers, vmIDs []string, now time.Time) error {
	if computers == nil {
		computers = []string{}
	}
	if vmIDs == nil {
		vmIDs = []string{}
	}
	raw, _ := json.Marshal(computers)
	rawVM, _ := json.Marshal(vmIDs)
	_, err := s.db.Exec(`UPDATE account_deletions SET computers = ?, vm_ids = ?, updated_at = ? WHERE user_id = ? AND `+liveDeletion, string(raw), string(rawVM), now.UTC(), userID)
	return err
}

func unionStrings(a, b []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, id := range append(append([]string{}, a...), b...) {
		id = strings.TrimSpace(id)
		if id != "" && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out
}

// RecordDeletionDestroySet adds computers (and VM ids) to the set the purge
// acts on. It only ever grows: a crash between vmctl's destroy and this
// record leaves the id recorded from before the destroy (the runner records
// the list first), and a retry never forgets one.
func (s *Store) RecordDeletionDestroySet(userID string, computers, vmIDs []string, now time.Time) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var rc, rv string
	if err := tx.QueryRow(`SELECT destroy_computers, destroy_vm_ids FROM account_deletions WHERE user_id = ? AND `+liveDeletion, userID).Scan(&rc, &rv); err != nil {
		return err
	}
	var oc, ov []string
	_ = json.Unmarshal([]byte(rc), &oc)
	_ = json.Unmarshal([]byte(rv), &ov)
	nc, _ := json.Marshal(unionStrings(oc, computers))
	nv, _ := json.Marshal(unionStrings(ov, vmIDs))
	if _, err := tx.Exec(`UPDATE account_deletions SET destroy_computers = ?, destroy_vm_ids = ?, updated_at = ? WHERE user_id = ?`, string(nc), string(nv), now.UTC(), userID); err != nil {
		return err
	}
	return tx.Commit()
}

// ReplaceDeletionDestroySet sets the destroy set to exactly these ids while
// the record is still pending, that is while nothing has been destroyed: a
// stale id from an earlier listing (a computer that has since left the
// account) is dropped rather than carried into the purge. Once the record is
// purging the set only grows (RecordDeletionDestroySet).
func (s *Store) ReplaceDeletionDestroySet(userID string, computers, vmIDs []string, now time.Time) error {
	nc, _ := json.Marshal(unionStrings(nil, computers))
	nv, _ := json.Marshal(unionStrings(nil, vmIDs))
	res, err := s.db.Exec(`UPDATE account_deletions SET destroy_computers = ?, destroy_vm_ids = ?, updated_at = ? WHERE user_id = ? AND state = ?`,
		string(nc), string(nv), now.UTC(), userID, DeletionPending)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrDeletionNotCancellable
	}
	return nil
}

// ClearDeletionSteps forgets the named steps, so the runner does them again.
func (s *Store) ClearDeletionSteps(userID string, names ...string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var raw string
	if err := tx.QueryRow(`SELECT steps FROM account_deletions WHERE user_id = ? AND `+liveDeletion, userID).Scan(&raw); err != nil {
		return err
	}
	steps := map[string]DeletionStep{}
	if err := json.Unmarshal([]byte(raw), &steps); err != nil {
		return err
	}
	for _, n := range names {
		delete(steps, n)
	}
	out, _ := json.Marshal(steps)
	if _, err := tx.Exec(`UPDATE account_deletions SET steps = ? WHERE user_id = ?`, string(out), userID); err != nil {
		return err
	}
	return tx.Commit()
}

// RecordDeletionStep stores one step's result.
func (s *Store) RecordDeletionStep(userID, step string, rec DeletionStep) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var raw string
	if err := tx.QueryRow(`SELECT steps FROM account_deletions WHERE user_id = ? AND `+liveDeletion, userID).Scan(&raw); err != nil {
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
	d, err := scanDeletion(tx.QueryRow(`SELECT `+deletionColumns+` FROM account_deletions WHERE user_id = ? AND `+liveDeletion, userID))
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
			`DELETE FROM recovery_tokens WHERE lower(email) = lower(?)`,
			`DELETE FROM registration_waitlist WHERE lower(email) = lower(?)`,
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
