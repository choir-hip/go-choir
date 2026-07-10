package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

// Store wraps a SQLite database connection and provides auth persistence for
// users, WebAuthn credentials, challenge/session state, and refresh/session
// records needed by later auth features.
type Store struct {
	db *sql.DB
}

// Schema DDL — all tables needed for Mission 2 Milestone 1 auth.
const schemaDDL = `
CREATE TABLE IF NOT EXISTS users (
	id         TEXT PRIMARY KEY,
	email      TEXT UNIQUE NOT NULL,
	created_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS credentials (
	id              TEXT PRIMARY KEY,
	user_id         TEXT    NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	public_key      BLOB    NOT NULL,
	attestation_type TEXT   NOT NULL,
	transport       TEXT    NOT NULL,
	sign_count      INTEGER NOT NULL DEFAULT 0,
	aaguid          BLOB    NOT NULL,
	flags           TEXT    NOT NULL DEFAULT '{}',
	name            TEXT    NOT NULL DEFAULT '',
	created_at      DATETIME NOT NULL,
	last_used_at    DATETIME
);

CREATE TABLE IF NOT EXISTS challenge_state (
	id                 TEXT PRIMARY KEY,
	user_id            TEXT    NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	challenge          TEXT    NOT NULL,
	type               TEXT    NOT NULL CHECK(type IN ('registration', 'login')),
	allowed_credentials TEXT,
	webauthn_session_data TEXT,
	created_at         DATETIME NOT NULL,
	expires_at         DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS refresh_sessions (
	id           TEXT PRIMARY KEY,
	user_id      TEXT    NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	token_hash   TEXT    NOT NULL,
	created_at   DATETIME NOT NULL,
	expires_at   DATETIME NOT NULL,
	rotated_from TEXT,
	device_info  TEXT,
	last_used_at DATETIME
);

CREATE INDEX IF NOT EXISTS idx_credentials_user_id ON credentials(user_id);
CREATE INDEX IF NOT EXISTS idx_challenge_state_user_id ON challenge_state(user_id);
CREATE INDEX IF NOT EXISTS idx_challenge_state_expires_at ON challenge_state(expires_at);
CREATE INDEX IF NOT EXISTS idx_refresh_sessions_user_id ON refresh_sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_refresh_sessions_expires_at ON refresh_sessions(expires_at);
CREATE INDEX IF NOT EXISTS idx_refresh_sessions_token_hash ON refresh_sessions(token_hash);

CREATE TABLE IF NOT EXISTS desktop_exchange_codes (
	code        TEXT PRIMARY KEY,
	user_id     TEXT    NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	access_token  TEXT  NOT NULL,
	refresh_token TEXT  NOT NULL,
	created_at  DATETIME NOT NULL,
	expires_at  DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_desktop_exchange_expires_at ON desktop_exchange_codes(expires_at);

CREATE TABLE IF NOT EXISTS api_keys (
	id           TEXT PRIMARY KEY,
	user_id      TEXT    NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	key_hash     TEXT    NOT NULL,
	label        TEXT    NOT NULL,
	scopes       TEXT    NOT NULL DEFAULT '[]',
	created_at   DATETIME NOT NULL,
	expires_at   DATETIME,
	last_used_at DATETIME,
	revoked_at   DATETIME
);
CREATE INDEX IF NOT EXISTS idx_api_keys_user_id ON api_keys(user_id);
CREATE INDEX IF NOT EXISTS idx_api_keys_key_hash ON api_keys(key_hash);
CREATE INDEX IF NOT EXISTS idx_api_keys_expires_at ON api_keys(expires_at);

CREATE TABLE IF NOT EXISTS recovery_tokens (
	id          TEXT PRIMARY KEY,
	user_id     TEXT,
	email       TEXT    NOT NULL,
	email_hash  TEXT    NOT NULL,
	ip_hash     TEXT    NOT NULL,
	token_hash  TEXT    NOT NULL,
	created_at  DATETIME NOT NULL,
	expires_at  DATETIME NOT NULL,
	used_at     DATETIME
);
CREATE INDEX IF NOT EXISTS idx_recovery_tokens_email_hash ON recovery_tokens(email_hash);
CREATE INDEX IF NOT EXISTS idx_recovery_tokens_ip_hash ON recovery_tokens(ip_hash);
CREATE INDEX IF NOT EXISTS idx_recovery_tokens_token_hash ON recovery_tokens(token_hash);
CREATE INDEX IF NOT EXISTS idx_recovery_tokens_expires_at ON recovery_tokens(expires_at);
CREATE INDEX IF NOT EXISTS idx_recovery_tokens_user_id ON recovery_tokens(user_id);
`

// schemaMigrations contains DDL statements that add columns to existing tables
// when the schema has evolved since the initial creation. These are run after
// the main DDL and are safe to repeat (they silently no-op if the column
// already exists due to the OR IGNORE / error-handling approach).
var schemaMigrations = []string{
	// Added webauthn_session_data column for storing serialized SessionData
	// needed by the finish handlers.
	`ALTER TABLE challenge_state ADD COLUMN webauthn_session_data TEXT`,
	// Added flags column for storing WebAuthn CredentialFlags (backup_eligible,
	// backup_state, user_present, user_verified) needed for re-login verification.
	`ALTER TABLE credentials ADD COLUMN flags TEXT NOT NULL DEFAULT '{}'`,
	// M7: Added name column for user-facing credential labels (multi-device).
	`ALTER TABLE credentials ADD COLUMN name TEXT NOT NULL DEFAULT ''`,
	// M7: Added last_used_at for credential usage tracking.
	`ALTER TABLE credentials ADD COLUMN last_used_at DATETIME`,
	// M7: Added device_info for session management (User-Agent at creation).
	`ALTER TABLE refresh_sessions ADD COLUMN device_info TEXT`,
	// M7: Added last_used_at for session usage tracking.
	`ALTER TABLE refresh_sessions ADD COLUMN last_used_at DATETIME`,
}

// User represents a row in the users table.
type User struct {
	ID        string
	Email     string
	CreatedAt time.Time
}

// Credential represents a WebAuthn passkey row in the credentials table.
type Credential struct {
	ID              string
	UserID          string
	PublicKey       []byte
	AttestationType string
	Transport       string
	SignCount       int64
	AAGUID          []byte
	Flags           string // JSON-encoded CredentialFlags: user_present, user_verified, backup_eligible, backup_state
	Name            string
	CreatedAt       time.Time
	LastUsedAt      *time.Time
}

// ChallengeState represents a WebAuthn ceremony challenge row in the
// challenge_state table.
type ChallengeState struct {
	ID                  string
	UserID              string
	Challenge           string
	Type                string // "registration" or "login"
	AllowedCredentials  string // JSON-encoded array (may be empty for registration)
	WebAuthnSessionData string // JSON-serialized webauthn.SessionData for finish handlers
	CreatedAt           time.Time
	ExpiresAt           time.Time
}

// RefreshSession represents a refresh/session record in the
// refresh_sessions table.
type RefreshSession struct {
	ID          string
	UserID      string
	TokenHash   string
	CreatedAt   time.Time
	ExpiresAt   time.Time
	RotatedFrom string
	DeviceInfo  string
	LastUsedAt  *time.Time
}

// OpenStore opens (or creates) the SQLite database at dbPath and applies the
// schema. It returns a Store ready for use.
func OpenStore(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("auth store: open %s: %w", dbPath, err)
	}

	// The auth service performs short challenge/session writes from request
	// handlers. SQLite permits many readers but only one writer; without an
	// explicit busy timeout concurrent login/register ceremonies can fail fast
	// with SQLITE_BUSY. Keep one writer connection and wait briefly instead of
	// surfacing a transient auth failure to the browser.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	// Enable WAL mode for better concurrent read performance and enable
	// foreign keys so that CASCADE works.
	if _, err := db.Exec("PRAGMA busy_timeout=10000"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("auth store: set busy timeout: %w", err)
	}
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("auth store: set WAL mode: %w", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("auth store: enable foreign keys: %w", err)
	}

	s := &Store{db: db}
	if err := s.bootstrap(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("auth store: bootstrap: %w", err)
	}

	return s, nil
}

// bootstrap applies the schema DDL to the database.
func (s *Store) bootstrap() error {
	_, err := s.db.Exec(schemaDDL)
	if err != nil {
		return fmt.Errorf("apply schema: %w", err)
	}

	// Apply migrations — each ALTER TABLE may fail harmlessly if the column
	// already exists (SQLite returns "duplicate column name"). We ignore those
	// specific errors so that re-running bootstrap is idempotent.
	for _, m := range schemaMigrations {
		_, err := s.db.Exec(m)
		if err != nil {
			// SQLite returns "duplicate column name" when a column already exists.
			// This is safe to ignore.
			if !isDuplicateColumnErr(err) {
				return fmt.Errorf("apply migration %q: %w", m, err)
			}
		}
	}

	// Hard cutover: remove username column by recreating the users table.
	// This is idempotent and safe to run multiple times.
	if err := s.migrateDropUsernameColumn(); err != nil {
		return fmt.Errorf("migrate drop username column: %w", err)
	}

	// The legacy desktop handoff schema carried raw access and refresh bearer
	// values. The redirect-only handoff now stores only a user reference, but
	// the NOT NULL columns remain until a later table migration. Scrub any
	// in-flight legacy values on startup and keep writing empty compatibility
	// values below so bearer credentials never persist in this table.
	if _, err := s.db.Exec(`UPDATE desktop_exchange_codes SET access_token = '', refresh_token = '' WHERE access_token <> '' OR refresh_token <> ''`); err != nil {
		return fmt.Errorf("scrub legacy desktop exchange tokens: %w", err)
	}

	return nil
}

// isDuplicateColumnErr returns true if the error is a SQLite "duplicate column
// name" error, which occurs when trying to add a column that already exists.
func isDuplicateColumnErr(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return containsString(msg, "duplicate column name")
}

// containsString reports whether substr is contained in s.
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

// searchSubstring is a simple substring search.
func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// migrateDropUsernameColumn performs a hard cutover from the old schema
// (with username column) to the new schema (just email). SQLite doesn't
// support DROP COLUMN, so we recreate the table.
// This is idempotent: safe to run multiple times.
func (s *Store) migrateDropUsernameColumn() error {
	// Check if users table has a username column (old schema).
	var hasUsernameCol bool
	err := s.db.QueryRow(
		"SELECT 1 FROM pragma_table_info('users') WHERE name = 'username'",
	).Scan(&hasUsernameCol)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("check for username column: %w", err)
	}

	// If no username column exists, we're already on the new schema — nothing to do.
	if !hasUsernameCol {
		return nil
	}

	// Also check if email column exists (for databases that have both columns).
	var hasEmailCol bool
	err = s.db.QueryRow(
		"SELECT 1 FROM pragma_table_info('users') WHERE name = 'email'",
	).Scan(&hasEmailCol)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("check for email column: %w", err)
	}

	// SQLite doesn't support DROP COLUMN, so we recreate the table:
	// 1. Create new table with correct schema (no username)
	// 2. Copy data from old table (username → email, or email if it exists)
	// 3. Drop old table
	// 4. Rename new table
	//
	// Note: Foreign keys are temporarily disabled during this migration
	// because we're recreating the users table that other tables reference.

	// Disable foreign keys during table recreation.
	if _, err := s.db.Exec("PRAGMA foreign_keys=OFF"); err != nil {
		return fmt.Errorf("disable foreign keys: %w", err)
	}
	// Re-enable foreign keys at the end (even on error paths).
	defer func() {
		_, _ = s.db.Exec("PRAGMA foreign_keys=ON")
	}()

	// Start transaction for atomicity.
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Create new users table with correct schema.
	_, err = tx.Exec(`
		CREATE TABLE users_new (
			id         TEXT PRIMARY KEY,
			email      TEXT UNIQUE NOT NULL,
			created_at DATETIME NOT NULL
		)
	`)
	if err != nil {
		return fmt.Errorf("create users_new table: %w", err)
	}

	// Copy data from old table.
	// Use COALESCE(email, username) if email column exists, otherwise just username.
	var copySQL string
	if hasEmailCol {
		// Has both columns: prefer email, fall back to username if email is NULL.
		copySQL = `
			INSERT INTO users_new (id, email, created_at)
			SELECT id, COALESCE(email, username), created_at FROM users
		`
	} else {
		// Only has username column: use it directly for email.
		copySQL = `
			INSERT INTO users_new (id, email, created_at)
			SELECT id, username, created_at FROM users
		`
	}
	_, err = tx.Exec(copySQL)
	if err != nil {
		return fmt.Errorf("copy data to users_new: %w", err)
	}

	// Drop old table.
	_, err = tx.Exec("DROP TABLE users")
	if err != nil {
		return fmt.Errorf("drop old users table: %w", err)
	}

	// Rename new table to users.
	_, err = tx.Exec("ALTER TABLE users_new RENAME TO users")
	if err != nil {
		return fmt.Errorf("rename users_new to users: %w", err)
	}

	// Commit transaction.
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration transaction: %w", err)
	}

	return nil
}

// Close closes the underlying database connection.
func (s *Store) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// DB returns the underlying *sql.DB for use by later auth features that need
// direct query access.
func (s *Store) DB() *sql.DB {
	return s.db
}

// CreateUser inserts a new user and returns it.
func (s *Store) CreateUser(id, email string) (*User, error) {
	now := time.Now().UTC()
	_, err := s.db.Exec(
		"INSERT INTO users (id, email, created_at) VALUES (?, ?, ?)",
		id, email, now,
	)
	if err != nil {
		return nil, fmt.Errorf("create user %q: %w", email, err)
	}
	return &User{ID: id, Email: email, CreatedAt: now}, nil
}

// GetUserByID returns the user with the given ID, or sql.ErrNoRows.
func (s *Store) GetUserByID(id string) (*User, error) {
	u := &User{}
	err := s.db.QueryRow(
		"SELECT id, email, created_at FROM users WHERE id = ?", id,
	).Scan(&u.ID, &u.Email, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return u, nil
}

// GetUserByEmail returns the user with the given email, or sql.ErrNoRows.
func (s *Store) GetUserByEmail(email string) (*User, error) {
	u := &User{}
	err := s.db.QueryRow(
		"SELECT id, email, created_at FROM users WHERE email = ?", email,
	).Scan(&u.ID, &u.Email, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return u, nil
}

// CreateCredential inserts a WebAuthn credential (passkey) record.
func (s *Store) CreateCredential(c *Credential) error {
	_, err := s.db.Exec(
		"INSERT INTO credentials (id, user_id, public_key, attestation_type, transport, sign_count, aaguid, flags, name, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		c.ID, c.UserID, c.PublicKey, c.AttestationType, c.Transport, c.SignCount, c.AAGUID, c.Flags, c.Name, c.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create credential %q: %w", c.ID, err)
	}
	return nil
}

// GetCredentialsByUserID returns all credentials for the given user.
func (s *Store) GetCredentialsByUserID(userID string) ([]Credential, error) {
	rows, err := s.db.Query(
		"SELECT id, user_id, public_key, attestation_type, transport, sign_count, aaguid, flags, name, created_at, last_used_at FROM credentials WHERE user_id = ?",
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var creds []Credential
	for rows.Next() {
		var c Credential
		var lastUsedAt sql.NullTime
		if err := rows.Scan(&c.ID, &c.UserID, &c.PublicKey, &c.AttestationType, &c.Transport, &c.SignCount, &c.AAGUID, &c.Flags, &c.Name, &c.CreatedAt, &lastUsedAt); err != nil {
			return nil, err
		}
		if lastUsedAt.Valid {
			c.LastUsedAt = &lastUsedAt.Time
		}
		creds = append(creds, c)
	}
	return creds, rows.Err()
}

// UpdateCredentialSignCount updates the sign_count for the given credential ID.
func (s *Store) UpdateCredentialSignCount(credID string, signCount int64) error {
	_, err := s.db.Exec(
		"UPDATE credentials SET sign_count = ? WHERE id = ?",
		signCount, credID,
	)
	if err != nil {
		return fmt.Errorf("update credential sign count %q: %w", credID, err)
	}
	return nil
}

// SaveChallengeState inserts a challenge/session record for a WebAuthn ceremony.
func (s *Store) SaveChallengeState(cs *ChallengeState) error {
	_, err := s.db.Exec(
		"INSERT INTO challenge_state (id, user_id, challenge, type, allowed_credentials, webauthn_session_data, created_at, expires_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		cs.ID, cs.UserID, cs.Challenge, cs.Type, cs.AllowedCredentials, cs.WebAuthnSessionData, cs.CreatedAt, cs.ExpiresAt,
	)
	if err != nil {
		return fmt.Errorf("save challenge state %q: %w", cs.ID, err)
	}
	return nil
}

// GetChallengeStateByID returns the challenge state with the given ID.
func (s *Store) GetChallengeStateByID(id string) (*ChallengeState, error) {
	cs := &ChallengeState{}
	err := s.db.QueryRow(
		"SELECT id, user_id, challenge, type, allowed_credentials, webauthn_session_data, created_at, expires_at FROM challenge_state WHERE id = ?",
		id,
	).Scan(&cs.ID, &cs.UserID, &cs.Challenge, &cs.Type, &cs.AllowedCredentials, &cs.WebAuthnSessionData, &cs.CreatedAt, &cs.ExpiresAt)
	if err != nil {
		return nil, err
	}
	return cs, nil
}

// DeleteChallengeStateByID removes a challenge state record (after finish or expiry).
func (s *Store) DeleteChallengeStateByID(id string) error {
	_, err := s.db.Exec("DELETE FROM challenge_state WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete challenge state %q: %w", id, err)
	}
	return nil
}

// GetChallengeStatesByUserID returns all challenge states for the given user,
// ordered by created_at descending (most recent first).
func (s *Store) GetChallengeStatesByUserID(userID string) ([]ChallengeState, error) {
	rows, err := s.db.Query(
		"SELECT id, user_id, challenge, type, allowed_credentials, webauthn_session_data, created_at, expires_at FROM challenge_state WHERE user_id = ? ORDER BY created_at DESC",
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var results []ChallengeState
	for rows.Next() {
		var cs ChallengeState
		if err := rows.Scan(&cs.ID, &cs.UserID, &cs.Challenge, &cs.Type, &cs.AllowedCredentials, &cs.WebAuthnSessionData, &cs.CreatedAt, &cs.ExpiresAt); err != nil {
			return nil, err
		}
		results = append(results, cs)
	}
	return results, rows.Err()
}

// CleanExpiredChallenges removes all challenge_state rows past their expires_at.
func (s *Store) CleanExpiredChallenges() (int64, error) {
	res, err := s.db.Exec("DELETE FROM challenge_state WHERE expires_at < ?", time.Now().UTC())
	if err != nil {
		return 0, fmt.Errorf("clean expired challenges: %w", err)
	}
	return res.RowsAffected()
}

// CreateRefreshSession inserts a new refresh/session record.
func (s *Store) CreateRefreshSession(rs *RefreshSession) error {
	_, err := s.db.Exec(
		"INSERT INTO refresh_sessions (id, user_id, token_hash, created_at, expires_at, rotated_from, device_info) VALUES (?, ?, ?, ?, ?, ?, ?)",
		rs.ID, rs.UserID, rs.TokenHash, rs.CreatedAt, rs.ExpiresAt, rs.RotatedFrom, rs.DeviceInfo,
	)
	if err != nil {
		return fmt.Errorf("create refresh session %q: %w", rs.ID, err)
	}
	return nil
}

// GetRefreshSessionByTokenHash returns the refresh session matching the given
// token hash, or sql.ErrNoRows.
func (s *Store) GetRefreshSessionByTokenHash(tokenHash string) (*RefreshSession, error) {
	rs := &RefreshSession{}
	var (
		deviceInfo sql.NullString
		lastUsedAt sql.NullTime
	)
	err := s.db.QueryRow(
		"SELECT id, user_id, token_hash, created_at, expires_at, rotated_from, device_info, last_used_at FROM refresh_sessions WHERE token_hash = ?",
		tokenHash,
	).Scan(&rs.ID, &rs.UserID, &rs.TokenHash, &rs.CreatedAt, &rs.ExpiresAt, &rs.RotatedFrom, &deviceInfo, &lastUsedAt)
	if err != nil {
		return nil, err
	}
	if deviceInfo.Valid {
		rs.DeviceInfo = deviceInfo.String
	}
	if lastUsedAt.Valid {
		rs.LastUsedAt = &lastUsedAt.Time
	}
	return rs, nil
}

// DeleteRefreshSessionByID removes a refresh session by ID (used during
// rotation or logout).
func (s *Store) DeleteRefreshSessionByID(id string) error {
	_, err := s.db.Exec("DELETE FROM refresh_sessions WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete refresh session %q: %w", id, err)
	}
	return nil
}

// DeleteRefreshSessionsByUserID removes all refresh sessions for a user
// (used during logout to fully invalidate).
func (s *Store) DeleteRefreshSessionsByUserID(userID string) error {
	_, err := s.db.Exec("DELETE FROM refresh_sessions WHERE user_id = ?", userID)
	if err != nil {
		return fmt.Errorf("delete refresh sessions for user %q: %w", userID, err)
	}
	return nil
}

// CleanExpiredRefreshSessions removes all refresh_sessions rows past their
// expires_at.
func (s *Store) CleanExpiredRefreshSessions() (int64, error) {
	res, err := s.db.Exec("DELETE FROM refresh_sessions WHERE expires_at < ?", time.Now().UTC())
	if err != nil {
		return 0, fmt.Errorf("clean expired refresh sessions: %w", err)
	}
	return res.RowsAffected()
}

// DesktopExchangeCode represents a one-time, user-bound native session
// handoff. It never contains access or refresh bearer credentials.
type DesktopExchangeCode struct {
	Code      string
	UserID    string
	CreatedAt time.Time
	ExpiresAt time.Time
}

func hashDesktopExchangeCode(code string) string {
	hash := sha256.Sum256([]byte(code))
	return fmt.Sprintf("%x", hash)
}

// CreateDesktopExchangeCode inserts a new one-time exchange code.
func (s *Store) CreateDesktopExchangeCode(c *DesktopExchangeCode) error {
	_, err := s.db.Exec(
		"INSERT INTO desktop_exchange_codes (code, user_id, access_token, refresh_token, created_at, expires_at) VALUES (?, ?, ?, ?, ?, ?)",
		hashDesktopExchangeCode(c.Code), c.UserID, "", "", c.CreatedAt, c.ExpiresAt,
	)
	if err != nil {
		return fmt.Errorf("create desktop exchange code: %w", err)
	}
	return nil
}

// ConsumeDesktopExchangeCode atomically retrieves and deletes a code.
// Returns sql.ErrNoRows if the code does not exist or is expired.
func (s *Store) ConsumeDesktopExchangeCode(code string) (*DesktopExchangeCode, error) {
	codeHash := hashDesktopExchangeCode(code)
	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	row := tx.QueryRow(
		"SELECT user_id, created_at, expires_at FROM desktop_exchange_codes WHERE code = ?",
		codeHash,
	)
	c := &DesktopExchangeCode{Code: code}
	if err := row.Scan(&c.UserID, &c.CreatedAt, &c.ExpiresAt); err != nil {
		return nil, err
	}
	if time.Now().UTC().After(c.ExpiresAt) {
		_, _ = tx.Exec("DELETE FROM desktop_exchange_codes WHERE code = ?", codeHash)
		return nil, fmt.Errorf("exchange code expired")
	}
	if _, err := tx.Exec("DELETE FROM desktop_exchange_codes WHERE code = ?", codeHash); err != nil {
		return nil, fmt.Errorf("delete exchange code: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return c, nil
}

// CleanExpiredDesktopExchangeCodes removes expired exchange codes.
func (s *Store) CleanExpiredDesktopExchangeCodes() (int64, error) {
	res, err := s.db.Exec("DELETE FROM desktop_exchange_codes WHERE expires_at < ?", time.Now().UTC())
	if err != nil {
		return 0, fmt.Errorf("clean expired desktop exchange codes: %w", err)
	}
	return res.RowsAffected()
}

// --- API Keys ---

// APIKey represents a row in the api_keys table. The secret (choir_sk_...) is
// only returned once at creation time and is never stored; only the SHA-256
// hash (key_hash) is persisted.
type APIKey struct {
	ID         string
	UserID     string
	Label      string
	Scopes     []string
	CreatedAt  time.Time
	ExpiresAt  *time.Time
	LastUsedAt *time.Time
	RevokedAt  *time.Time
}

// APIKeyPrefix is the prefix for all API key secrets.
const APIKeyPrefix = "choir_sk_"

// generateAPIKeySecret generates a new opaque API key secret of the form
// choir_sk_<32 bytes base64url>. It returns the raw secret (returned once to
// the caller) and its SHA-256 hex hash (stored in the database).
func generateAPIKeySecret() (secret, keyHash string, err error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", fmt.Errorf("generate api key secret: %w", err)
	}
	secret = APIKeyPrefix + base64.RawURLEncoding.EncodeToString(raw)
	h := sha256.Sum256([]byte(secret))
	keyHash = fmt.Sprintf("%x", h)
	return secret, keyHash, nil
}

// CreateAPIKey generates a new API key for the given user, stores only the
// SHA-256 hash of the secret, and returns the public key ID and the raw secret.
// The secret is only returned once at creation time and is never stored in
// plaintext.
func (s *Store) CreateAPIKey(ctx context.Context, userID, label string, scopes []string, expiresAt *time.Time) (id, secret string, err error) {
	if userID == "" {
		return "", "", errors.New("create api key: user_id is required")
	}
	if label == "" {
		return "", "", errors.New("create api key: label is required")
	}

	secret, keyHash, err := generateAPIKeySecret()
	if err != nil {
		return "", "", err
	}

	// Ensure scopes is a non-nil slice so json.Marshal produces "[]" not "null".
	if scopes == nil {
		scopes = []string{}
	}
	scopesJSON, err := json.Marshal(scopes)
	if err != nil {
		return "", "", fmt.Errorf("marshal scopes: %w", err)
	}

	id = "ak_" + uuid.NewString()
	now := time.Now().UTC()

	_, err = s.db.ExecContext(ctx,
		"INSERT INTO api_keys (id, user_id, key_hash, label, scopes, created_at, expires_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
		id, userID, keyHash, label, string(scopesJSON), now, expiresAt,
	)
	if err != nil {
		return "", "", fmt.Errorf("create api key: %w", err)
	}

	return id, secret, nil
}

// GetAPIKeyByHash looks up an API key by its SHA-256 hash. It excludes revoked
// keys (revoked_at IS NOT NULL) and returns sql.ErrNoRows if no active key
// matches. The caller is responsible for checking expiry.
func (s *Store) GetAPIKeyByHash(ctx context.Context, keyHash string) (*APIKey, error) {
	var (
		ak         APIKey
		scopesJSON string
		expiresAt  sql.NullTime
		lastUsedAt sql.NullTime
		revokedAt  sql.NullTime
	)
	err := s.db.QueryRowContext(ctx,
		"SELECT id, user_id, label, scopes, created_at, expires_at, last_used_at, revoked_at FROM api_keys WHERE key_hash = ? AND revoked_at IS NULL",
		keyHash,
	).Scan(&ak.ID, &ak.UserID, &ak.Label, &scopesJSON, &ak.CreatedAt, &expiresAt, &lastUsedAt, &revokedAt)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal([]byte(scopesJSON), &ak.Scopes); err != nil {
		return nil, fmt.Errorf("parse api key scopes: %w", err)
	}
	if expiresAt.Valid {
		ak.ExpiresAt = &expiresAt.Time
	}
	if lastUsedAt.Valid {
		ak.LastUsedAt = &lastUsedAt.Time
	}
	if revokedAt.Valid {
		ak.RevokedAt = &revokedAt.Time
	}

	return &ak, nil
}

// ListAPIKeys returns all API keys for the given user, ordered by created_at
// descending. Revoked keys are included (with revoked_at set) so the user can
// see their full key history. Secrets are never returned.
func (s *Store) ListAPIKeys(ctx context.Context, userID string) ([]APIKey, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, user_id, label, scopes, created_at, expires_at, last_used_at, revoked_at FROM api_keys WHERE user_id = ? ORDER BY created_at DESC",
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var keys []APIKey
	for rows.Next() {
		var (
			ak         APIKey
			scopesJSON string
			expiresAt  sql.NullTime
			lastUsedAt sql.NullTime
			revokedAt  sql.NullTime
		)
		if err := rows.Scan(&ak.ID, &ak.UserID, &ak.Label, &scopesJSON, &ak.CreatedAt, &expiresAt, &lastUsedAt, &revokedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(scopesJSON), &ak.Scopes); err != nil {
			return nil, fmt.Errorf("parse api key scopes: %w", err)
		}
		if expiresAt.Valid {
			ak.ExpiresAt = &expiresAt.Time
		}
		if lastUsedAt.Valid {
			ak.LastUsedAt = &lastUsedAt.Time
		}
		if revokedAt.Valid {
			ak.RevokedAt = &revokedAt.Time
		}
		keys = append(keys, ak)
	}
	return keys, rows.Err()
}

// RevokeAPIKey soft-deletes an API key by setting revoked_at to now. It only
// revokes keys belonging to the given user (ownership check) and returns
// sql.ErrNoRows if no matching active key is found.
func (s *Store) RevokeAPIKey(ctx context.Context, userID, keyID string) error {
	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx,
		"UPDATE api_keys SET revoked_at = ? WHERE id = ? AND user_id = ? AND revoked_at IS NULL",
		now, keyID, userID,
	)
	if err != nil {
		return fmt.Errorf("revoke api key: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("revoke api key: rows affected: %w", err)
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// TouchAPIKeyLastUsed updates the last_used_at timestamp for the given key ID.
// This is called on each successful API key validation. Errors are non-fatal
// (the caller may choose to ignore them).
func (s *Store) TouchAPIKeyLastUsed(ctx context.Context, keyID string) error {
	_, err := s.db.ExecContext(ctx,
		"UPDATE api_keys SET last_used_at = ? WHERE id = ?",
		time.Now().UTC(), keyID,
	)
	if err != nil {
		return fmt.Errorf("touch api key last_used: %w", err)
	}
	return nil
}

// BootstrapAdminAPIKeyUserID is the user ID for the bootstrap admin API key.
// It is a synthetic platform-admin user, not a WebAuthn-registered human. The
// bootstrap ensures this user row exists (idempotent) before seeding the key.
const BootstrapAdminAPIKeyUserID = "choir-bootstrap-admin"
const bootstrapAdminAPIKeyUserEmail = "bootstrap-admin@choir.local"
const bootstrapAdminAPIKeyLabel = "bootstrap-admin"

// SeedBootstrapAdminAPIKey seeds a single admin-scoped API key from rawKey
// when the auth DB has zero non-revoked API keys. This is the first-run
// escape hatch for the chicken-and-egg problem: a machine needs an API key
// to call the API, but only a WebAuthn-authenticated human can create one
// through the normal flow. The bootstrap lets an operator seed the first key
// from config (env var) so headless agents (the choir CLI) can authenticate
// before any human has provisioned a key.
//
// Safety properties:
//   - First-run-only: if any non-revoked API key exists, this is a no-op and
//     logs that it skipped. Reboots after first provisioning do not create
//     duplicate or shadow keys.
//   - Only the SHA-256 hash is stored (same as WebAuthn-provisioned keys);
//     the raw key is never persisted and never logged.
//   - The key has admin scope (full access) so it can provision other keys
//     and verify any route. It is revocable via the existing RevokeAPIKey
//     flow, identical to a WebAuthn-provisioned key.
//   - The activation is logged loudly (key ID + label) so operators can see
//     when the bootstrap fired.
//
// rawKey must already include the choir_sk_ prefix. If it does not, the
// function returns an error without touching the DB.
func (s *Store) SeedBootstrapAdminAPIKey(ctx context.Context, rawKey string) (keyID string, seeded bool, err error) {
	rawKey = strings.TrimSpace(rawKey)
	if rawKey == "" {
		return "", false, errors.New("seed bootstrap api key: raw key is required")
	}
	if !strings.HasPrefix(rawKey, APIKeyPrefix) {
		return "", false, fmt.Errorf("seed bootstrap api key: raw key must start with %q", APIKeyPrefix)
	}

	// First-run-only guard: count non-revoked API keys. If any exist, skip.
	var count int
	if err := s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM api_keys WHERE revoked_at IS NULL",
	).Scan(&count); err != nil {
		return "", false, fmt.Errorf("seed bootstrap api key: count existing: %w", err)
	}
	if count > 0 {
		log.Printf("auth: bootstrap admin api key skipped (%d api key(s) already exist)", count)
		return "", false, nil
	}

	// Ensure the synthetic bootstrap-admin user exists (idempotent). The
	// api_keys.user_id column has a FK to users(id), so the row must exist.
	if _, err := s.GetUserByID(BootstrapAdminAPIKeyUserID); err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return "", false, fmt.Errorf("seed bootstrap api key: lookup bootstrap user: %w", err)
		}
		if _, err := s.CreateUser(BootstrapAdminAPIKeyUserID, bootstrapAdminAPIKeyUserEmail); err != nil {
			return "", false, fmt.Errorf("seed bootstrap api key: create bootstrap user: %w", err)
		}
		log.Printf("auth: created bootstrap admin user %q", BootstrapAdminAPIKeyUserID)
	}

	// Hash the raw key (SHA-256) and insert the key row. No expiry: the
	// key is revocable by the operator the moment a WebAuthn-provisioned
	// key exists.
	h := sha256.Sum256([]byte(rawKey))
	keyHash := fmt.Sprintf("%x", h)
	keyID = "ak_" + uuid.NewString()
	scopesJSON, err := json.Marshal([]string{"admin"})
	if err != nil {
		return "", false, fmt.Errorf("seed bootstrap api key: marshal scopes: %w", err)
	}
	now := time.Now().UTC()
	if _, err := s.db.ExecContext(ctx,
		"INSERT INTO api_keys (id, user_id, key_hash, label, scopes, created_at, expires_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
		keyID, BootstrapAdminAPIKeyUserID, keyHash, bootstrapAdminAPIKeyLabel, string(scopesJSON), now, nil,
	); err != nil {
		return "", false, fmt.Errorf("seed bootstrap api key: insert: %w", err)
	}

	log.Printf("auth: bootstrap admin api key seeded (key_id=%s label=%s user=%s) — revoke it once a WebAuthn-provisioned key exists",
		keyID, bootstrapAdminAPIKeyLabel, BootstrapAdminAPIKeyUserID)
	return keyID, true, nil
}

// --- Recovery Tokens (M7) ---

// RecoveryTokenPrefix is the prefix for recovery token secrets.
const RecoveryTokenPrefix = "choir_rt_"

// RecoveryTokenTTL is how long a magic link recovery token remains valid.
const RecoveryTokenTTL = 15 * time.Minute

// RecoveryMaxPerEmail is the maximum recovery requests per email per hour.
const RecoveryMaxPerEmail = 3

// RecoveryMaxPerIP is the maximum recovery requests per IP per hour.
const RecoveryMaxPerIP = 5

// RecoveryToken represents a row in the recovery_tokens table. The raw token
// secret is never stored — only the SHA-256 hash (token_hash) is persisted.
type RecoveryToken struct {
	ID        string
	UserID    string // may be empty for anti-enumeration dummy records
	Email     string
	EmailHash string
	IPHash    string
	TokenHash string
	CreatedAt time.Time
	ExpiresAt time.Time
	UsedAt    *time.Time
}

// generateRecoveryTokenSecret generates a new opaque recovery token of the form
// choir_rt_<32 bytes base64url>. It returns the raw token (returned once to the
// caller) and its SHA-256 hex hash (stored in the database).
func generateRecoveryTokenSecret() (token, tokenHash string, err error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", fmt.Errorf("generate recovery token: %w", err)
	}
	token = RecoveryTokenPrefix + base64.RawURLEncoding.EncodeToString(raw)
	h := sha256.Sum256([]byte(token))
	tokenHash = fmt.Sprintf("%x", h)
	return token, tokenHash, nil
}

// CreateRecoveryToken generates a recovery token for the given email and IP,
// stores only the SHA-256 hash, and returns the raw token. The userID may be
// empty for anti-enumeration dummy records (when the email doesn't match a
// real user). The raw token is only returned once and never stored in plaintext.
func (s *Store) CreateRecoveryToken(ctx context.Context, userID, email, emailHash, ipHash string) (token string, err error) {
	token, tokenHash, err := generateRecoveryTokenSecret()
	if err != nil {
		return "", err
	}

	now := time.Now().UTC()
	rt := &RecoveryToken{
		ID:        uuid.NewString(),
		UserID:    userID,
		Email:     email,
		EmailHash: emailHash,
		IPHash:    ipHash,
		TokenHash: tokenHash,
		CreatedAt: now,
		ExpiresAt: now.Add(RecoveryTokenTTL),
	}

	_, err = s.db.ExecContext(ctx,
		"INSERT INTO recovery_tokens (id, user_id, email, email_hash, ip_hash, token_hash, created_at, expires_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		rt.ID, rt.UserID, rt.Email, rt.EmailHash, rt.IPHash, rt.TokenHash, rt.CreatedAt, rt.ExpiresAt,
	)
	if err != nil {
		return "", fmt.Errorf("create recovery token: %w", err)
	}

	return token, nil
}

// ConsumeRecoveryToken atomically validates and marks a recovery token as used.
// It checks the token hash, expiry, and single-use constraint (used_at IS NULL).
// Returns the recovery token record (including user_id) if valid, or an error
// if the token is not found, already used, expired, or has no associated user
// (anti-enumeration dummy record).
func (s *Store) ConsumeRecoveryToken(ctx context.Context, token string) (*RecoveryToken, error) {
	h := sha256.Sum256([]byte(token))
	tokenHash := fmt.Sprintf("%x", h)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	rt := &RecoveryToken{}
	var (
		userID sql.NullString
		usedAt sql.NullTime
	)
	err = tx.QueryRowContext(ctx,
		"SELECT id, user_id, email, email_hash, ip_hash, token_hash, created_at, expires_at, used_at FROM recovery_tokens WHERE token_hash = ?",
		tokenHash,
	).Scan(&rt.ID, &userID, &rt.Email, &rt.EmailHash, &rt.IPHash, &rt.TokenHash, &rt.CreatedAt, &rt.ExpiresAt, &usedAt)
	if err != nil {
		return nil, err
	}

	if userID.Valid {
		rt.UserID = userID.String
	}

	// Check single-use.
	if usedAt.Valid {
		return nil, errors.New("recovery token already used")
	}

	// Check expiry.
	if time.Now().UTC().After(rt.ExpiresAt) {
		// Clean up expired token.
		_, _ = tx.ExecContext(ctx, "DELETE FROM recovery_tokens WHERE id = ?", rt.ID)
		_ = tx.Commit()
		return nil, errors.New("recovery token expired")
	}

	// Check that this is a real user token (not an anti-enumeration dummy).
	if !userID.Valid || rt.UserID == "" {
		return nil, errors.New("recovery token has no associated user")
	}

	// Mark as used (single-use enforcement).
	now := time.Now().UTC()
	if _, err := tx.ExecContext(ctx, "UPDATE recovery_tokens SET used_at = ? WHERE id = ?", now, rt.ID); err != nil {
		return nil, fmt.Errorf("mark recovery token used: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return rt, nil
}

// CountRecoveryTokensByEmailSince returns the number of recovery tokens created
// for the given email hash since the given time. Used for rate limiting.
func (s *Store) CountRecoveryTokensByEmailSince(ctx context.Context, emailHash string, since time.Time) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM recovery_tokens WHERE email_hash = ? AND created_at >= ?",
		emailHash, since,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count recovery tokens by email: %w", err)
	}
	return count, nil
}

// CountRecoveryTokensByIPSince returns the number of recovery tokens created
// from the given IP hash since the given time. Used for rate limiting.
func (s *Store) CountRecoveryTokensByIPSince(ctx context.Context, ipHash string, since time.Time) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM recovery_tokens WHERE ip_hash = ? AND created_at >= ?",
		ipHash, since,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count recovery tokens by ip: %w", err)
	}
	return count, nil
}

// CleanExpiredRecoveryTokens removes all recovery_tokens rows past their
// expires_at, or that have been used more than 24 hours ago.
func (s *Store) CleanExpiredRecoveryTokens() (int64, error) {
	now := time.Now().UTC()
	res, err := s.db.Exec(
		"DELETE FROM recovery_tokens WHERE expires_at < ? OR (used_at IS NOT NULL AND used_at < ?)",
		now, now.Add(-24*time.Hour),
	)
	if err != nil {
		return 0, fmt.Errorf("clean expired recovery tokens: %w", err)
	}
	return res.RowsAffected()
}

// --- Credential Management (M7) ---

// DeleteCredential removes a WebAuthn credential. It only deletes credentials
// belonging to the given user (ownership check) and returns sql.ErrNoRows if
// no matching credential is found.
func (s *Store) DeleteCredential(ctx context.Context, userID, credID string) error {
	res, err := s.db.ExecContext(ctx,
		"DELETE FROM credentials WHERE id = ? AND user_id = ?",
		credID, userID,
	)
	if err != nil {
		return fmt.Errorf("delete credential: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete credential rows affected: %w", err)
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// RenameCredential updates the user-facing name of a credential. It only
// renames credentials belonging to the given user (ownership check) and
// returns sql.ErrNoRows if no matching credential is found.
func (s *Store) RenameCredential(ctx context.Context, userID, credID, name string) error {
	res, err := s.db.ExecContext(ctx,
		"UPDATE credentials SET name = ? WHERE id = ? AND user_id = ?",
		name, credID, userID,
	)
	if err != nil {
		return fmt.Errorf("rename credential: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rename credential rows affected: %w", err)
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// TouchCredentialLastUsed updates the last_used_at timestamp for the given
// credential ID. This is called on successful WebAuthn login. Errors are
// non-fatal (the caller may choose to ignore them).
func (s *Store) TouchCredentialLastUsed(credID string) error {
	_, err := s.db.Exec(
		"UPDATE credentials SET last_used_at = ? WHERE id = ?",
		time.Now().UTC(), credID,
	)
	if err != nil {
		return fmt.Errorf("touch credential last_used: %w", err)
	}
	return nil
}

// --- Session Management (M7) ---

// ListRefreshSessionsByUserID returns all refresh sessions for the given user,
// ordered by created_at descending (most recent first). Token hashes are
// included in the struct but should not be exposed in API responses.
func (s *Store) ListRefreshSessionsByUserID(userID string) ([]RefreshSession, error) {
	rows, err := s.db.Query(
		"SELECT id, user_id, token_hash, created_at, expires_at, rotated_from, device_info, last_used_at FROM refresh_sessions WHERE user_id = ? ORDER BY created_at DESC",
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var sessions []RefreshSession
	for rows.Next() {
		var rs RefreshSession
		var (
			rotatedFrom sql.NullString
			deviceInfo  sql.NullString
			lastUsedAt  sql.NullTime
		)
		if err := rows.Scan(&rs.ID, &rs.UserID, &rs.TokenHash, &rs.CreatedAt, &rs.ExpiresAt, &rotatedFrom, &deviceInfo, &lastUsedAt); err != nil {
			return nil, err
		}
		if rotatedFrom.Valid {
			rs.RotatedFrom = rotatedFrom.String
		}
		if deviceInfo.Valid {
			rs.DeviceInfo = deviceInfo.String
		}
		if lastUsedAt.Valid {
			rs.LastUsedAt = &lastUsedAt.Time
		}
		sessions = append(sessions, rs)
	}
	return sessions, rows.Err()
}

// GetRefreshSessionByID returns the refresh session with the given ID, or
// sql.ErrNoRows if not found.
func (s *Store) GetRefreshSessionByID(id string) (*RefreshSession, error) {
	rs := &RefreshSession{}
	var (
		rotatedFrom sql.NullString
		deviceInfo  sql.NullString
		lastUsedAt  sql.NullTime
	)
	err := s.db.QueryRow(
		"SELECT id, user_id, token_hash, created_at, expires_at, rotated_from, device_info, last_used_at FROM refresh_sessions WHERE id = ?",
		id,
	).Scan(&rs.ID, &rs.UserID, &rs.TokenHash, &rs.CreatedAt, &rs.ExpiresAt, &rotatedFrom, &deviceInfo, &lastUsedAt)
	if err != nil {
		return nil, err
	}
	if rotatedFrom.Valid {
		rs.RotatedFrom = rotatedFrom.String
	}
	if deviceInfo.Valid {
		rs.DeviceInfo = deviceInfo.String
	}
	if lastUsedAt.Valid {
		rs.LastUsedAt = &lastUsedAt.Time
	}
	return rs, nil
}

// TouchRefreshSessionLastUsed updates the last_used_at timestamp for the given
// session ID. This is called when a refresh token is validated (before
// rotation). Errors are non-fatal.
func (s *Store) TouchRefreshSessionLastUsed(id string) error {
	_, err := s.db.Exec(
		"UPDATE refresh_sessions SET last_used_at = ? WHERE id = ?",
		time.Now().UTC(), id,
	)
	if err != nil {
		return fmt.Errorf("touch refresh session last_used: %w", err)
	}
	return nil
}
