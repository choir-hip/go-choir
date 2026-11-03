package auth

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Device authorization persistence (docs/design/cli-agent-onboarding-
// 2026-11-03.md). Both codes are stored only as SHA-256 hashes. Approve and
// deny are one conditional update from pending; collecting the key marks the
// row consumed and inserts the key in one transaction, so a row yields at
// most one key, and the key's secret is generated then and never stored.

const (
	DeviceStatusPending  = "pending"
	DeviceStatusApproved = "approved"
	DeviceStatusDenied   = "denied"
	DeviceStatusConsumed = "consumed"
	// DeviceStatusExpired is reported, never stored: a row past expires_at.
	DeviceStatusExpired = "expired"

	// DeviceAuthorizationTTL is the life of a request, approved or not.
	DeviceAuthorizationTTL = 10 * time.Minute
	// DevicePollIntervalSeconds is the poll interval the CLI is told.
	DevicePollIntervalSeconds = 5
	// DeviceMaxBeginPerIP is the hourly begin limit.
	DeviceMaxBeginPerIP = 10
	// DeviceAuthorizationRetention is how long rows are kept (the begin
	// limit's history).
	DeviceAuthorizationRetention = 24 * time.Hour
	deviceLimitWindow            = time.Hour
)

var (
	// ErrDeviceRateLimited means the begin limit was reached.
	ErrDeviceRateLimited = errors.New("device login rate limited")
	// ErrDeviceNotFound is every lookup and decide failure: unknown,
	// expired or no longer pending.
	ErrDeviceNotFound = errors.New("no pending login request with that code")
)

// DeviceAuthorization is a row of device_authorizations.
type DeviceAuthorization struct {
	ID               string
	DeviceCodeHash   string
	UserCodeHash     string
	Label            string
	UserAgentSummary string
	PlaceSummary     string
	IPHash           string
	CreatedAt        time.Time
	ExpiresAt        time.Time
	Status           string
	UserID           string
	DecidedAt        *time.Time
	ConsumedAt       *time.Time
	APIKeyID         string
	LastPollAt       *time.Time
}

// EffectiveStatus reads an unconsumed row past its expiry as expired.
func (a *DeviceAuthorization) EffectiveStatus(now time.Time) string {
	if a.Status != DeviceStatusConsumed && !a.ExpiresAt.After(now) {
		return DeviceStatusExpired
	}
	return a.Status
}

// CreateDeviceAuthorization applies the per-IP begin limit and inserts the
// row in one transaction.
func (s *Store) CreateDeviceAuthorization(ctx context.Context, a *DeviceAuthorization) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("create device authorization: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	var n int
	if err := tx.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM device_authorizations WHERE ip_hash = ? AND created_at >= ?",
		a.IPHash, a.CreatedAt.Add(-deviceLimitWindow)).Scan(&n); err != nil {
		return fmt.Errorf("create device authorization: count ip: %w", err)
	}
	if n >= DeviceMaxBeginPerIP {
		return ErrDeviceRateLimited
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO device_authorizations
		 (id, device_code_hash, user_code_hash, label, user_agent_summary, place_summary, ip_hash, created_at, expires_at, status)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID, a.DeviceCodeHash, a.UserCodeHash, a.Label, a.UserAgentSummary, a.PlaceSummary, a.IPHash,
		a.CreatedAt, a.ExpiresAt, DeviceStatusPending); err != nil {
		return fmt.Errorf("create device authorization: insert: %w", err)
	}
	return tx.Commit()
}

const deviceAuthorizationColumns = `id, device_code_hash, user_code_hash, label, user_agent_summary, place_summary, ip_hash,
	created_at, expires_at, status, user_id, decided_at, consumed_at, api_key_id, last_poll_at`

type rowScanner interface{ Scan(dest ...any) error }

func scanDeviceAuthorization(row rowScanner) (*DeviceAuthorization, error) {
	a := &DeviceAuthorization{}
	var (
		userID, keyID                 sql.NullString
		decidedAt, consumedAt, polled sql.NullTime
	)
	if err := row.Scan(&a.ID, &a.DeviceCodeHash, &a.UserCodeHash, &a.Label, &a.UserAgentSummary, &a.PlaceSummary, &a.IPHash,
		&a.CreatedAt, &a.ExpiresAt, &a.Status, &userID, &decidedAt, &consumedAt, &keyID, &polled); err != nil {
		return nil, err
	}
	a.UserID, a.APIKeyID = userID.String, keyID.String
	if decidedAt.Valid {
		t := decidedAt.Time
		a.DecidedAt = &t
	}
	if consumedAt.Valid {
		t := consumedAt.Time
		a.ConsumedAt = &t
	}
	if polled.Valid {
		t := polled.Time
		a.LastPollAt = &t
	}
	return a, nil
}

// GetDeviceAuthorizationByUserCode returns the newest row for a user code
// hash, or sql.ErrNoRows.
func (s *Store) GetDeviceAuthorizationByUserCode(ctx context.Context, userCodeHash string) (*DeviceAuthorization, error) {
	return scanDeviceAuthorization(s.db.QueryRowContext(ctx,
		"SELECT "+deviceAuthorizationColumns+" FROM device_authorizations WHERE user_code_hash = ? ORDER BY created_at DESC LIMIT 1",
		userCodeHash))
}

// DecideDeviceAuthorization approves or denies a pending, live request for
// userID with one conditional update. Anything else is ErrDeviceNotFound.
func (s *Store) DecideDeviceAuthorization(ctx context.Context, userCodeHash, userID string, approve bool, now time.Time) error {
	status := DeviceStatusDenied
	if approve {
		status = DeviceStatusApproved
	}
	res, err := s.db.ExecContext(ctx,
		`UPDATE device_authorizations SET status = ?, user_id = ?, decided_at = ?
		  WHERE user_code_hash = ? AND status = ? AND expires_at > ?`,
		status, userID, now, userCodeHash, DeviceStatusPending, now)
	if err != nil {
		return fmt.Errorf("decide device authorization: %w", err)
	}
	if n, err := res.RowsAffected(); err != nil || n != 1 {
		return ErrDeviceNotFound
	}
	return nil
}

// DevicePollResult is what one poll found. Status is one of the
// DeviceStatus values, or "slow_down" or "" (unknown device code). When the
// poll collected the key, Secret and KeyID are set.
type DevicePollResult struct {
	Status string
	Secret string
	KeyID  string
	Label  string
	Scopes []string
}

// PollDeviceAuthorization answers a poll. An approved, live request is
// collected: the row becomes consumed and the key is inserted in the same
// transaction, with scopes and the label "choir login: <label>".
func (s *Store) PollDeviceAuthorization(ctx context.Context, deviceCodeHash string, scopes []string, minInterval time.Duration, now time.Time) (*DevicePollResult, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("poll device authorization: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	a, err := scanDeviceAuthorization(tx.QueryRowContext(ctx,
		"SELECT "+deviceAuthorizationColumns+" FROM device_authorizations WHERE device_code_hash = ?", deviceCodeHash))
	if errors.Is(err, sql.ErrNoRows) {
		return &DevicePollResult{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("poll device authorization: load: %w", err)
	}
	switch st := a.EffectiveStatus(now); st {
	case DeviceStatusConsumed, DeviceStatusExpired, DeviceStatusDenied:
		return &DevicePollResult{Status: st}, nil
	}
	tooSoon := a.LastPollAt != nil && now.Sub(*a.LastPollAt) < minInterval
	if _, err := tx.ExecContext(ctx, "UPDATE device_authorizations SET last_poll_at = ? WHERE id = ?", now, a.ID); err != nil {
		return nil, fmt.Errorf("poll device authorization: touch: %w", err)
	}
	if tooSoon {
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return &DevicePollResult{Status: "slow_down"}, nil
	}
	if a.Status == DeviceStatusPending {
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return &DevicePollResult{Status: DeviceStatusPending}, nil
	}
	if a.Status != DeviceStatusApproved || a.UserID == "" {
		return nil, fmt.Errorf("poll device authorization: unexpected status %q", a.Status)
	}

	secret, keyHash, err := generateAPIKeySecret()
	if err != nil {
		return nil, err
	}
	keyID := "ak_" + uuid.NewString()
	label := "choir login: " + a.Label
	res, err := tx.ExecContext(ctx,
		`UPDATE device_authorizations SET status = ?, consumed_at = ?, api_key_id = ?
		  WHERE id = ? AND status = ? AND consumed_at IS NULL AND expires_at > ?`,
		DeviceStatusConsumed, now, keyID, a.ID, DeviceStatusApproved, now)
	if err != nil {
		return nil, fmt.Errorf("poll device authorization: consume: %w", err)
	}
	if n, err := res.RowsAffected(); err != nil || n != 1 {
		return &DevicePollResult{Status: DeviceStatusConsumed}, nil
	}
	scopesJSON, err := json.Marshal(scopes)
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx,
		"INSERT INTO api_keys (id, user_id, key_hash, label, scopes, computer_id, created_at, expires_at) VALUES (?, ?, ?, ?, ?, NULL, ?, NULL)",
		keyID, a.UserID, keyHash, label, string(scopesJSON), now); err != nil {
		return nil, fmt.Errorf("poll device authorization: insert key: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("poll device authorization: commit: %w", err)
	}
	return &DevicePollResult{Status: DeviceStatusApproved, Secret: secret, KeyID: keyID, Label: label, Scopes: scopes}, nil
}

// SweepDeviceAuthorizations deletes rows created before the cutoff.
func (s *Store) SweepDeviceAuthorizations(ctx context.Context, cutoff time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx, "DELETE FROM device_authorizations WHERE created_at < ?", cutoff)
	if err != nil {
		return 0, fmt.Errorf("sweep device authorizations: %w", err)
	}
	return res.RowsAffected()
}
