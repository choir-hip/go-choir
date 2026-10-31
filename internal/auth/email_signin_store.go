package auth

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Email sign-in persistence (docs/design/email-sign-in-2026-10-31.md, Data and
// Limits). Every limit counts and inserts in one transaction (the store runs
// with one open connection, so the transaction serializes), and the wrong-code
// count and the consumption are single conditional updates, as
// ConsumeRecoveryToken does: of two concurrent verifies exactly one wins.

const (
	// EmailAttemptPurposeSignIn is an attempt that signs a user in.
	EmailAttemptPurposeSignIn = "sign_in"
	// EmailAttemptPurposeVerifyAddress is an attempt that proves the inbox
	// of a signed-in (passkey) user.
	EmailAttemptPurposeVerifyAddress = "verify_address"

	// EmailAttemptTTL is the life of a code and a link.
	EmailAttemptTTL = 10 * time.Minute
	// EmailMaxWrongCodes kills an attempt.
	EmailMaxWrongCodes = 5
	// EmailMaxBeginPerEmail and EmailMaxBeginPerIP are the hourly begin limits.
	EmailMaxBeginPerEmail = 5
	EmailMaxBeginPerIP    = 10
	// EmailMaxVerifyPerIP is the hourly verify limit.
	EmailMaxVerifyPerIP = 30
	// EmailAttemptRetention is how long rows are kept: expired rows are the
	// rate-limit history, so this is at least the limit window.
	EmailAttemptRetention = 24 * time.Hour
	emailLimitWindow      = time.Hour
)

var (
	// ErrEmailRateLimited means a begin limit was reached.
	ErrEmailRateLimited = errors.New("email sign-in rate limited")
	// ErrEmailAttemptInvalid is every verify failure: wrong, expired, dead,
	// consumed, unknown or not this client's. One error, so no path tells an
	// attacker which it was.
	ErrEmailAttemptInvalid = errors.New("invalid or expired email sign-in code")
)

// EmailAttempt is a row of email_signin_attempts. UserID is empty when the
// address matched no eligible account (the row is still written: T2).
type EmailAttempt struct {
	ID               string
	UserID           string
	Purpose          string
	EmailHash        string
	CodeHash         string
	LinkTokenHash    string
	CookieHash       string
	IPHash           string
	UserAgentSummary string
	CreatedAt        time.Time
	ExpiresAt        time.Time
	Attempts         int
	Sent             bool
	ConsumedAt       *time.Time
}

func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// CreateEmailAttempt applies the per-email and per-IP begin limits, kills the
// user's earlier unconsumed attempts of the same purpose, and inserts the
// row, all in one transaction. The limits count every row, known address or
// not.
func (s *Store) CreateEmailAttempt(ctx context.Context, a *EmailAttempt) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("create email attempt: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	since := a.CreatedAt.Add(-emailLimitWindow)
	var n int
	if err := tx.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM email_signin_attempts WHERE email_hash = ? AND created_at >= ?",
		a.EmailHash, since).Scan(&n); err != nil {
		return fmt.Errorf("create email attempt: count email: %w", err)
	}
	if n >= EmailMaxBeginPerEmail {
		return ErrEmailRateLimited
	}
	if err := tx.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM email_signin_attempts WHERE ip_hash = ? AND created_at >= ?",
		a.IPHash, since).Scan(&n); err != nil {
		return fmt.Errorf("create email attempt: count ip: %w", err)
	}
	if n >= EmailMaxBeginPerIP {
		return ErrEmailRateLimited
	}

	// A new attempt supersedes the user's earlier live ones of its purpose:
	// they are expired now (not deleted: they remain rate-limit history).
	if a.UserID != "" {
		if _, err := tx.ExecContext(ctx,
			"UPDATE email_signin_attempts SET expires_at = ? WHERE user_id = ? AND purpose = ? AND consumed_at IS NULL AND expires_at > ?",
			a.CreatedAt, a.UserID, a.Purpose, a.CreatedAt); err != nil {
			return fmt.Errorf("create email attempt: supersede: %w", err)
		}
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO email_signin_attempts
		 (id, user_id, purpose, email_hash, code_hash, link_token_hash, cookie_hash, ip_hash, user_agent_summary, created_at, expires_at, attempts, sent)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, 0)`,
		a.ID, nullableString(a.UserID), a.Purpose, a.EmailHash, a.CodeHash, a.LinkTokenHash, a.CookieHash,
		a.IPHash, a.UserAgentSummary, a.CreatedAt, a.ExpiresAt); err != nil {
		return fmt.Errorf("create email attempt: insert: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("create email attempt: commit: %w", err)
	}
	return nil
}

// MarkEmailAttemptSent records that maild accepted the mail. A row that never
// gets it is the F9 "unsent" attempt.
func (s *Store) MarkEmailAttemptSent(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE email_signin_attempts SET sent = 1 WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("mark email attempt sent: %w", err)
	}
	return nil
}

// GetEmailAttempt returns a row by id (tests and audit), or sql.ErrNoRows.
func (s *Store) GetEmailAttempt(ctx context.Context, id string) (*EmailAttempt, error) {
	a := &EmailAttempt{}
	var (
		userID     sql.NullString
		consumedAt sql.NullTime
		sent       int
	)
	err := s.db.QueryRowContext(ctx,
		`SELECT id, user_id, purpose, email_hash, code_hash, link_token_hash, cookie_hash, ip_hash,
		        user_agent_summary, created_at, expires_at, attempts, sent, consumed_at
		   FROM email_signin_attempts WHERE id = ?`, id).Scan(
		&a.ID, &userID, &a.Purpose, &a.EmailHash, &a.CodeHash, &a.LinkTokenHash, &a.CookieHash, &a.IPHash,
		&a.UserAgentSummary, &a.CreatedAt, &a.ExpiresAt, &a.Attempts, &sent, &consumedAt)
	if err != nil {
		return nil, err
	}
	a.UserID = userID.String
	a.Sent = sent != 0
	if consumedAt.Valid {
		t := consumedAt.Time
		a.ConsumedAt = &t
	}
	return a, nil
}

// ConsumeEmailCode verifies a typed code. The attempt must carry this
// client's cookie hash (a code works only in the client that asked for it),
// must be of the purpose, and for a bound purpose must belong to wantUserID
// (empty: any user). The wrong-code count and the consumption are conditional
// single-statement updates; matches is called with the stored code hash only
// after the count is taken, and its result gates the consumption. Every
// failure is ErrEmailAttemptInvalid. It returns the attempt's user id.
func (s *Store) ConsumeEmailCode(ctx context.Context, id, cookieHash, purpose, wantUserID string, matches func(codeHash string) bool) (string, error) {
	now := time.Now().UTC()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("consume email code: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var (
		storedCookie, storedCode, storedPurpose string
		userID                                  sql.NullString
	)
	err = tx.QueryRowContext(ctx,
		"SELECT cookie_hash, code_hash, purpose, user_id FROM email_signin_attempts WHERE id = ?", id,
	).Scan(&storedCookie, &storedCode, &storedPurpose, &userID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrEmailAttemptInvalid
	}
	if err != nil {
		return "", fmt.Errorf("consume email code: load: %w", err)
	}
	// A stolen attempt id alone is useless (T8): without this client's
	// cookie there is no guess to count and no code to compare.
	if subtle.ConstantTimeCompare([]byte(storedCookie), []byte(cookieHash)) != 1 {
		return "", ErrEmailAttemptInvalid
	}
	if storedPurpose != purpose || !userID.Valid || userID.String == "" {
		return "", ErrEmailAttemptInvalid
	}
	if wantUserID != "" && userID.String != wantUserID {
		return "", ErrEmailAttemptInvalid
	}

	res, err := tx.ExecContext(ctx,
		"UPDATE email_signin_attempts SET attempts = attempts + 1 WHERE id = ? AND consumed_at IS NULL AND attempts < ? AND expires_at > ?",
		id, EmailMaxWrongCodes, now)
	if err != nil {
		return "", fmt.Errorf("consume email code: count: %w", err)
	}
	if n, err := res.RowsAffected(); err != nil || n != 1 {
		return "", ErrEmailAttemptInvalid
	}
	// The count must persist whether or not the code matches.
	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("consume email code: commit count: %w", err)
	}

	if !matches(storedCode) {
		return "", ErrEmailAttemptInvalid
	}
	res, err = s.db.ExecContext(ctx,
		"UPDATE email_signin_attempts SET consumed_at = ? WHERE id = ? AND consumed_at IS NULL AND expires_at > ?",
		now, id, now)
	if err != nil {
		return "", fmt.Errorf("consume email code: consume: %w", err)
	}
	if n, err := res.RowsAffected(); err != nil || n != 1 {
		return "", ErrEmailAttemptInvalid
	}
	return userID.String, nil
}

// ConsumeEmailLink consumes a sign-in link token (by its hash) with one
// conditional update and returns the attempt's user id. A link works for the
// sign_in purpose only, and dies with its attempt (five wrong codes, a newer
// begin, expiry).
func (s *Store) ConsumeEmailLink(ctx context.Context, tokenHash string) (string, error) {
	now := time.Now().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("consume email link: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.ExecContext(ctx,
		`UPDATE email_signin_attempts SET consumed_at = ?
		  WHERE link_token_hash = ? AND purpose = ? AND user_id IS NOT NULL
		    AND consumed_at IS NULL AND attempts < ? AND expires_at > ?`,
		now, tokenHash, EmailAttemptPurposeSignIn, EmailMaxWrongCodes, now)
	if err != nil {
		return "", fmt.Errorf("consume email link: %w", err)
	}
	if n, err := res.RowsAffected(); err != nil || n != 1 {
		return "", ErrEmailAttemptInvalid
	}
	var userID sql.NullString
	if err := tx.QueryRowContext(ctx,
		"SELECT user_id FROM email_signin_attempts WHERE link_token_hash = ? AND consumed_at = ?",
		tokenHash, now).Scan(&userID); err != nil || !userID.Valid {
		return "", ErrEmailAttemptInvalid
	}
	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("consume email link: commit: %w", err)
	}
	return userID.String, nil
}

// DeleteUnconsumedEmailAttempts removes every attempt not yet consumed, so no
// outstanding code or link can still mint a session (the rollback of the
// switch). It returns the number removed.
func (s *Store) DeleteUnconsumedEmailAttempts(ctx context.Context) (int64, error) {
	res, err := s.db.ExecContext(ctx, "DELETE FROM email_signin_attempts WHERE consumed_at IS NULL")
	if err != nil {
		return 0, fmt.Errorf("delete unconsumed email attempts: %w", err)
	}
	return res.RowsAffected()
}

// SweepEmailAttempts deletes rows created before the cutoff.
func (s *Store) SweepEmailAttempts(ctx context.Context, cutoff time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx, "DELETE FROM email_signin_attempts WHERE created_at < ?", cutoff)
	if err != nil {
		return 0, fmt.Errorf("sweep email attempts: %w", err)
	}
	return res.RowsAffected()
}
