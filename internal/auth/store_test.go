package auth

import (
	"bytes"
	"database/sql"
	"path/filepath"
	"testing"
	"time"
)

func TestOpenStoreCreatesSchema(t *testing.T) {
	store := TestStore(t)

	// Verify that all tables exist by querying them.
	tables := []string{"users", "credentials", "challenge_state", "refresh_sessions", "desktop_exchange_codes"}
	for _, table := range tables {
		var name string
		err := store.DB().QueryRow(
			"SELECT name FROM sqlite_master WHERE type='table' AND name=?", table,
		).Scan(&name)
		if err != nil {
			t.Errorf("table %q not found in schema: %v", table, err)
		}
	}
}

func TestOpenStoreIdempotentBootstrap(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	// Open and bootstrap once.
	store1, err := OpenStore(dbPath)
	if err != nil {
		t.Fatalf("first OpenStore: %v", err)
	}
	_ = store1.Close()

	// Reopen the same database — bootstrap should be idempotent (IF NOT EXISTS).
	store2, err := OpenStore(dbPath)
	if err != nil {
		t.Fatalf("second OpenStore: %v", err)
	}
	_ = store2.Close()
}

func TestOpenStoreInvalidPath(t *testing.T) {
	// Try to open a database in a directory that doesn't exist and can't be created.
	_, err := OpenStore("/nonexistent/path/that/cannot/be/created/auth.db")
	if err == nil {
		t.Error("expected error for invalid DB path, got nil")
	}
}

func TestOpenStoreSetsWALAndForeignKeys(t *testing.T) {
	store := TestStore(t)

	var journalMode string
	if err := store.DB().QueryRow("PRAGMA journal_mode").Scan(&journalMode); err != nil {
		t.Fatalf("query journal_mode: %v", err)
	}
	if journalMode != "wal" {
		t.Errorf("journal_mode: got %q, want %q", journalMode, "wal")
	}

	var fkEnabled bool
	if err := store.DB().QueryRow("PRAGMA foreign_keys").Scan(&fkEnabled); err != nil {
		t.Fatalf("query foreign_keys: %v", err)
	}
	if !fkEnabled {
		t.Error("foreign_keys: got false, want true")
	}
}

func TestOpenStoreSerializesWritesAndWaitsOnBusy(t *testing.T) {
	store := TestStore(t)

	if got := store.DB().Stats().MaxOpenConnections; got != 1 {
		t.Fatalf("MaxOpenConnections = %d, want 1", got)
	}

	var busyTimeout int
	if err := store.DB().QueryRow("PRAGMA busy_timeout").Scan(&busyTimeout); err != nil {
		t.Fatalf("query busy_timeout: %v", err)
	}
	if busyTimeout < 10000 {
		t.Fatalf("busy_timeout = %d, want at least 10000", busyTimeout)
	}
}
func TestCreateUserDuplicateEmail(t *testing.T) {
	store := TestStore(t)

	if _, err := store.CreateUser("user-1", "alice@example.com"); err != nil {
		t.Fatalf("first CreateUser: %v", err)
	}
	_, err := store.CreateUser("user-2", "alice@example.com") // same email
	if err == nil {
		t.Error("expected error for duplicate email, got nil")
	}
}
func TestGetUserByIDNotFound(t *testing.T) {
	store := TestStore(t)

	_, err := store.GetUserByID("nonexistent")
	if err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows, got: %v", err)
	}
}
func TestGetUserByEmailNotFound(t *testing.T) {
	store := TestStore(t)

	_, err := store.GetUserByEmail("nobody@example.com")
	if err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows, got: %v", err)
	}
}
func TestOwnerRootWrapPersistence(t *testing.T) {
	store := TestStore(t)
	if _, err := store.CreateUser("owner-1", "owner@example.com"); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	cred := &Credential{
		ID:              "prf-credential",
		UserID:          "owner-1",
		PublicKey:       []byte("fake-public-key"),
		AttestationType: "none",
		Transport:       `["internal"]`,
		AAGUID:          make([]byte, 16),
		Flags:           "{}",
		CreatedAt:       time.Now().UTC(),
	}
	if err := store.CreateCredential(cred); err != nil {
		t.Fatalf("CreateCredential: %v", err)
	}
	salt := bytes.Repeat([]byte{0x11}, 32)
	if err := store.SetCredentialPRFCapable(cred.ID, salt); err != nil {
		t.Fatalf("SetCredentialPRFCapable: %v", err)
	}
	capable, gotCredentialSalt, err := store.GetCredentialPRFInfo(cred.ID)
	if err != nil {
		t.Fatalf("GetCredentialPRFInfo: %v", err)
	}
	if !capable || !bytes.Equal(gotCredentialSalt, salt) {
		t.Fatal("credential PRF info was not persisted")
	}
	root, created, err := store.EnsureOwnerRoot("owner-1")
	if err != nil {
		t.Fatalf("EnsureOwnerRoot: %v", err)
	}
	if !created || len(root) != 32 {
		t.Fatal("expected a new 32-byte owner ROOT")
	}
	if err := store.PutOwnerRootWrap("owner-1", cred.ID, salt, []byte("sealed-root")); err != nil {
		t.Fatalf("PutOwnerRootWrap: %v", err)
	}
	gotSalt, gotWrap, err := store.GetOwnerRootWrap("owner-1", cred.ID)
	if err != nil {
		t.Fatalf("GetOwnerRootWrap: %v", err)
	}
	if !bytes.Equal(gotSalt, salt) || !bytes.Equal(gotWrap, []byte("sealed-root")) {
		t.Fatal("owner ROOT wrap was not persisted")
	}
	hasRoot, err := store.HasOwnerRoot("owner-1")
	if err != nil {
		t.Fatalf("HasOwnerRoot: %v", err)
	}
	if !hasRoot {
		t.Fatal("owner ROOT existence was not persisted")
	}
	root, created, err = store.EnsureOwnerRoot("owner-1")
	if err != nil {
		t.Fatalf("EnsureOwnerRoot repeat: %v", err)
	}
	if created || root != nil {
		t.Fatal("existing ROOT should not be regenerated")
	}
}

func TestCreateCredentialMissingUser(t *testing.T) {
	store := TestStore(t)

	cred := &Credential{
		ID:              "cred-1",
		UserID:          "nonexistent-user",
		PublicKey:       []byte("key"),
		AttestationType: "none",
		Transport:       `["internal"]`,
		SignCount:       0,
		AAGUID:          make([]byte, 16),
		Flags:           "{}",
		CreatedAt:       time.Now().UTC(),
	}
	err := store.CreateCredential(cred)
	if err == nil {
		t.Error("expected error for credential with missing user, got nil")
	}
}
func TestUpdateCredentialSignCount(t *testing.T) {
	store := TestStore(t)

	if _, err := store.CreateUser("user-1", "alice"); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	cred := &Credential{
		ID:              "cred-1",
		UserID:          "user-1",
		PublicKey:       []byte("key"),
		AttestationType: "none",
		Transport:       `["internal"]`,
		SignCount:       0,
		AAGUID:          make([]byte, 16),
		Flags:           "{}",
		CreatedAt:       time.Now().UTC(),
	}
	if err := store.CreateCredential(cred); err != nil {
		t.Fatalf("CreateCredential: %v", err)
	}

	if err := store.UpdateCredentialSignCount("cred-1", 42); err != nil {
		t.Fatalf("UpdateCredentialSignCount: %v", err)
	}

	creds, err := store.GetCredentialsByUserID("user-1")
	if err != nil {
		t.Fatalf("GetCredentialsByUserID: %v", err)
	}
	if creds[0].SignCount != 42 {
		t.Errorf("SignCount: got %d, want 42", creds[0].SignCount)
	}
}

func TestCredentialCascadeDelete(t *testing.T) {
	store := TestStore(t)

	if _, err := store.CreateUser("user-1", "alice"); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	cred := &Credential{
		ID:              "cred-1",
		UserID:          "user-1",
		PublicKey:       []byte("key"),
		AttestationType: "none",
		Transport:       `["internal"]`,
		SignCount:       0,
		AAGUID:          make([]byte, 16),
		Flags:           "{}",
		CreatedAt:       time.Now().UTC(),
	}
	if err := store.CreateCredential(cred); err != nil {
		t.Fatalf("CreateCredential: %v", err)
	}

	// Delete the user; credentials should cascade.
	_, err := store.DB().Exec("DELETE FROM users WHERE id = ?", "user-1")
	if err != nil {
		t.Fatalf("delete user: %v", err)
	}

	creds, err := store.GetCredentialsByUserID("user-1")
	if err != nil {
		t.Fatalf("GetCredentialsByUserID: %v", err)
	}
	if len(creds) != 0 {
		t.Errorf("expected 0 credentials after user delete (cascade), got %d", len(creds))
	}
}
func TestSaveChallengeStateInvalidType(t *testing.T) {
	store := TestStore(t)

	if _, err := store.CreateUser("user-1", "alice"); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	cs := &ChallengeState{
		ID:        "challenge-bad",
		UserID:    "user-1",
		Challenge: "challenge",
		Type:      "invalid-type", // not in CHECK constraint
		CreatedAt: time.Now().UTC(),
		ExpiresAt: time.Now().UTC().Add(5 * time.Minute),
	}
	err := store.SaveChallengeState(cs)
	if err == nil {
		t.Error("expected error for invalid challenge type, got nil")
	}
}
func TestCleanExpiredChallenges(t *testing.T) {
	store := TestStore(t)

	if _, err := store.CreateUser("user-1", "alice"); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	now := time.Now().UTC()

	// Insert an expired challenge.
	expired := &ChallengeState{
		ID:        "expired-1",
		UserID:    "user-1",
		Challenge: "expired",
		Type:      "registration",
		CreatedAt: now.Add(-10 * time.Minute),
		ExpiresAt: now.Add(-5 * time.Minute), // already expired
	}
	if err := store.SaveChallengeState(expired); err != nil {
		t.Fatalf("SaveChallengeState expired: %v", err)
	}

	// Insert a valid challenge.
	valid := &ChallengeState{
		ID:        "valid-1",
		UserID:    "user-1",
		Challenge: "valid",
		Type:      "registration",
		CreatedAt: now,
		ExpiresAt: now.Add(5 * time.Minute),
	}
	if err := store.SaveChallengeState(valid); err != nil {
		t.Fatalf("SaveChallengeState valid: %v", err)
	}

	n, err := store.CleanExpiredChallenges()
	if err != nil {
		t.Fatalf("CleanExpiredChallenges: %v", err)
	}
	if n != 1 {
		t.Errorf("rows affected: got %d, want 1", n)
	}

	// Valid challenge should still exist.
	got, err := store.GetChallengeStateByID("valid-1")
	if err != nil {
		t.Fatalf("GetChallengeStateByID valid: %v", err)
	}
	if got.Challenge != "valid" {
		t.Errorf("valid challenge: got %q, want %q", got.Challenge, "valid")
	}

	// Expired challenge should be gone.
	_, err = store.GetChallengeStateByID("expired-1")
	if err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows for expired challenge, got: %v", err)
	}
}

func TestChallengeStateCascadeDelete(t *testing.T) {
	store := TestStore(t)

	if _, err := store.CreateUser("user-1", "alice"); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	cs := &ChallengeState{
		ID:        "challenge-1",
		UserID:    "user-1",
		Challenge: "challenge",
		Type:      "registration",
		CreatedAt: time.Now().UTC(),
		ExpiresAt: time.Now().UTC().Add(5 * time.Minute),
	}
	if err := store.SaveChallengeState(cs); err != nil {
		t.Fatalf("SaveChallengeState: %v", err)
	}

	// Delete the user; challenge state should cascade.
	_, err := store.DB().Exec("DELETE FROM users WHERE id = ?", "user-1")
	if err != nil {
		t.Fatalf("delete user: %v", err)
	}

	_, err = store.GetChallengeStateByID("challenge-1")
	if err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows after user cascade, got: %v", err)
	}
}
func TestCreateRefreshSessionWithRotation(t *testing.T) {
	store := TestStore(t)

	if _, err := store.CreateUser("user-1", "alice"); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	rs1 := &RefreshSession{
		ID:        "rs-1",
		UserID:    "user-1",
		TokenHash: "hash-1",
		CreatedAt: time.Now().UTC(),
		ExpiresAt: time.Now().UTC().Add(720 * time.Hour),
	}
	if err := store.CreateRefreshSession(rs1); err != nil {
		t.Fatalf("CreateRefreshSession rs-1: %v", err)
	}

	rs2 := &RefreshSession{
		ID:          "rs-2",
		UserID:      "user-1",
		TokenHash:   "hash-2",
		CreatedAt:   time.Now().UTC(),
		ExpiresAt:   time.Now().UTC().Add(720 * time.Hour),
		RotatedFrom: "rs-1", // rotated from previous session
	}
	if err := store.CreateRefreshSession(rs2); err != nil {
		t.Fatalf("CreateRefreshSession rs-2: %v", err)
	}

	got, err := store.GetRefreshSessionByTokenHash("hash-2")
	if err != nil {
		t.Fatalf("GetRefreshSessionByTokenHash: %v", err)
	}
	if got.RotatedFrom != "rs-1" {
		t.Errorf("RotatedFrom: got %q, want %q", got.RotatedFrom, "rs-1")
	}
}
func TestCleanExpiredRefreshSessions(t *testing.T) {
	store := TestStore(t)

	if _, err := store.CreateUser("user-1", "alice"); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	now := time.Now().UTC()

	// Insert an expired session.
	expired := &RefreshSession{
		ID:        "expired-rs",
		UserID:    "user-1",
		TokenHash: "expired-hash",
		CreatedAt: now.Add(-800 * time.Hour),
		ExpiresAt: now.Add(-1 * time.Hour), // already expired
	}
	if err := store.CreateRefreshSession(expired); err != nil {
		t.Fatalf("CreateRefreshSession expired: %v", err)
	}

	// Insert a valid session.
	valid := &RefreshSession{
		ID:        "valid-rs",
		UserID:    "user-1",
		TokenHash: "valid-hash",
		CreatedAt: now,
		ExpiresAt: now.Add(720 * time.Hour),
	}
	if err := store.CreateRefreshSession(valid); err != nil {
		t.Fatalf("CreateRefreshSession valid: %v", err)
	}

	n, err := store.CleanExpiredRefreshSessions()
	if err != nil {
		t.Fatalf("CleanExpiredRefreshSessions: %v", err)
	}
	if n != 1 {
		t.Errorf("rows affected: got %d, want 1", n)
	}

	// Valid session should still exist.
	got, err := store.GetRefreshSessionByTokenHash("valid-hash")
	if err != nil {
		t.Fatalf("GetRefreshSessionByTokenHash valid: %v", err)
	}
	if got.ID != "valid-rs" {
		t.Errorf("valid session ID: got %q, want %q", got.ID, "valid-rs")
	}

	// Expired session should be gone.
	_, err = store.GetRefreshSessionByTokenHash("expired-hash")
	if err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows for expired session, got: %v", err)
	}
}

func TestRefreshSessionCascadeDelete(t *testing.T) {
	store := TestStore(t)

	if _, err := store.CreateUser("user-1", "alice"); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	rs := &RefreshSession{
		ID:        "rs-1",
		UserID:    "user-1",
		TokenHash: "hash-cascade",
		CreatedAt: time.Now().UTC(),
		ExpiresAt: time.Now().UTC().Add(720 * time.Hour),
	}
	if err := store.CreateRefreshSession(rs); err != nil {
		t.Fatalf("CreateRefreshSession: %v", err)
	}

	// Delete the user; refresh sessions should cascade.
	_, err := store.DB().Exec("DELETE FROM users WHERE id = ?", "user-1")
	if err != nil {
		t.Fatalf("delete user: %v", err)
	}

	_, err = store.GetRefreshSessionByTokenHash("hash-cascade")
	if err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows after user cascade, got: %v", err)
	}
}

func TestRefreshSessionMissingUser(t *testing.T) {
	store := TestStore(t)

	rs := &RefreshSession{
		ID:        "rs-orphan",
		UserID:    "nonexistent-user",
		TokenHash: "orphan-hash",
		CreatedAt: time.Now().UTC(),
		ExpiresAt: time.Now().UTC().Add(720 * time.Hour),
	}
	err := store.CreateRefreshSession(rs)
	if err == nil {
		t.Error("expected error for refresh session with missing user, got nil")
	}
}


// ======================================================================
// VAL-CROSS-118: Auth restart preserves session data
// ======================================================================

// TestSessionDataSurvivesAuthRestart verifies that auth session data
// persists across a simulated auth service restart. This is the key
// invariant for VAL-CROSS-118: after auth restarts, browser users can
// rehydrate via refresh-token rotation because their session data is
// persisted in SQLite rather than held only in memory.
//
// The test simulates a restart by closing the store and re-opening it
// against the same database file, then verifying that the previously
// created user, credential, and refresh session are still present and
// usable.
func TestSessionDataSurvivesAuthRestart(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "auth-restart-test.db")

	// --- Phase 1: First "run" of auth — create user, credential, refresh session.
	store1, err := OpenStore(dbPath)
	if err != nil {
		t.Fatalf("first OpenStore: %v", err)
	}

	// Create a user.
	user, err := store1.CreateUser("user-restart-001", "restart-tester@example.com")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	// Create a credential for the user.
	cred := &Credential{
		ID:              "cred-restart-001",
		UserID:          user.ID,
		PublicKey:       []byte("fake-pub-key-for-restart-test"),
		AttestationType: "none",
		Transport:       "[]",
		SignCount:       0,
		AAGUID:          []byte{},
		Flags:           `{"user_present":true,"user_verified":true,"backup_eligible":true,"backup_state":false}`,
		CreatedAt:       time.Now().UTC(),
	}
	if err := store1.CreateCredential(cred); err != nil {
		t.Fatalf("CreateCredential: %v", err)
	}

	// Create a refresh session for the user.
	rs := &RefreshSession{
		ID:        "rs-restart-001",
		UserID:    user.ID,
		TokenHash: "abc123restart",
		CreatedAt: time.Now().UTC(),
		ExpiresAt: time.Now().UTC().Add(720 * time.Hour),
	}
	if err := store1.CreateRefreshSession(rs); err != nil {
		t.Fatalf("CreateRefreshSession: %v", err)
	}

	// Close the store (simulate auth shutdown).
	_ = store1.Close()

	// --- Phase 2: Second "run" of auth --- reopen the same DB, verify data persists.
	store2, err := OpenStore(dbPath)
	if err != nil {
		t.Fatalf("second OpenStore (after restart): %v", err)
	}
	defer func() { _ = store2.Close() }()

	// Verify the user still exists.
	foundUser, err := store2.GetUserByID(user.ID)
	if err != nil {
		t.Fatalf("GetUserByID after restart: %v", err)
	}
	if foundUser.Email != "restart-tester@example.com" {
		t.Errorf("user after restart: got email %q, want %q", foundUser.Email, "restart-tester@example.com")
	}

	// Verify the credential still exists.
	creds, err := store2.GetCredentialsByUserID(user.ID)
	if err != nil {
		t.Fatalf("GetCredentialsByUserID after restart: %v", err)
	}
	if len(creds) != 1 {
		t.Fatalf("credentials after restart: got %d, want 1", len(creds))
	}
	if creds[0].ID != "cred-restart-001" {
		t.Errorf("credential ID after restart: got %q, want %q", creds[0].ID, "cred-restart-001")
	}

	// Verify the refresh session still exists and is usable.
	foundRS, err := store2.GetRefreshSessionByTokenHash("abc123restart")
	if err != nil {
		t.Fatalf("GetRefreshSessionByTokenHash after restart: %v", err)
	}
	if foundRS.ID != "rs-restart-001" {
		t.Errorf("refresh session ID after restart: got %q, want %q", foundRS.ID, "rs-restart-001")
	}
	if foundRS.UserID != user.ID {
		t.Errorf("refresh session user after restart: got %q, want %q", foundRS.UserID, user.ID)
	}

	// Verify the refresh session is not expired.
	if time.Now().UTC().After(foundRS.ExpiresAt) {
		t.Error("refresh session expired after restart — should still be valid")
	}
}

// TestRefreshRotationWorksAfterAuthRestart verifies that refresh token
// rotation (the mechanism used for browser rehydration) works correctly
// after a simulated auth restart. This directly exercises the
// VAL-CROSS-118 recovery path: after auth restarts, a browser user's
// expired access JWT is renewed by rotating their refresh token.
func TestRefreshRotationWorksAfterAuthRestart(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "auth-rotation-test.db")

	// --- Phase 1: Create user and initial refresh session.
	store1, err := OpenStore(dbPath)
	if err != nil {
		t.Fatalf("first OpenStore: %v", err)
	}

	user, err := store1.CreateUser("user-rotation-001", "rotation-tester@example.com")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	rs := &RefreshSession{
		ID:        "rs-rotation-001",
		UserID:    user.ID,
		TokenHash: "initial-token-hash",
		CreatedAt: time.Now().UTC(),
		ExpiresAt: time.Now().UTC().Add(720 * time.Hour),
	}
	if err := store1.CreateRefreshSession(rs); err != nil {
		t.Fatalf("CreateRefreshSession: %v", err)
	}

	_ = store1.Close()

	// --- Phase 2: After restart, rotate the refresh session.
	store2, err := OpenStore(dbPath)
	if err != nil {
		t.Fatalf("second OpenStore (after restart): %v", err)
	}
	defer func() { _ = store2.Close() }()

	// Look up the existing refresh session (simulating the browser's
	// refresh cookie being presented for renewal).
	foundRS, err := store2.GetRefreshSessionByTokenHash("initial-token-hash")
	if err != nil {
		t.Fatalf("GetRefreshSessionByTokenHash after restart: %v", err)
	}

	// Delete the old session (rotation).
	if err := store2.DeleteRefreshSessionByID(foundRS.ID); err != nil {
		t.Fatalf("DeleteRefreshSessionByID (rotation): %v", err)
	}

	// Create a new rotated session.
	newRS := &RefreshSession{
		ID:        "rs-rotation-002",
		UserID:    user.ID,
		TokenHash: "rotated-token-hash",
		CreatedAt: time.Now().UTC(),
		ExpiresAt: time.Now().UTC().Add(720 * time.Hour),
	}
	if err := store2.CreateRefreshSession(newRS); err != nil {
		t.Fatalf("CreateRefreshSession (rotated): %v", err)
	}

	// Verify the old session is gone.
	_, err = store2.GetRefreshSessionByTokenHash("initial-token-hash")
	if err == nil {
		t.Error("old refresh session should be deleted after rotation")
	}

	// Verify the new session is usable.
	foundNewRS, err := store2.GetRefreshSessionByTokenHash("rotated-token-hash")
	if err != nil {
		t.Fatalf("GetRefreshSessionByTokenHash (rotated): %v", err)
	}
	if foundNewRS.UserID != user.ID {
		t.Errorf("rotated session user: got %q, want %q", foundNewRS.UserID, user.ID)
	}

	// Verify user data is still accessible.
	foundUser, err := store2.GetUserByID(user.ID)
	if err != nil {
		t.Fatalf("GetUserByID after rotation: %v", err)
	}
	if foundUser.Email != "rotation-tester@example.com" {
		t.Errorf("user after rotation: got email %q, want %q", foundUser.Email, "rotation-tester@example.com")
	}
}

// --- DesktopExchangeCode CRUD ---

func TestCreateAndConsumeDesktopExchangeCode(t *testing.T) {
	store := TestStore(t)

	user, err := store.CreateUser("user-ex-1", "exchange@example.com")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	now := time.Now().UTC()
	code := &DesktopExchangeCode{
		Code:      "test-code-1",
		UserID:    user.ID,
		CreatedAt: now,
		ExpiresAt: now.Add(60 * time.Second),
	}
	if err := store.CreateDesktopExchangeCode(code); err != nil {
		t.Fatalf("CreateDesktopExchangeCode: %v", err)
	}

	var storedCode, storedAccess, storedRefresh string
	if err := store.DB().QueryRow(
		"SELECT code, access_token, refresh_token FROM desktop_exchange_codes WHERE user_id = ?",
		user.ID,
	).Scan(&storedCode, &storedAccess, &storedRefresh); err != nil {
		t.Fatalf("read persisted desktop handoff: %v", err)
	}
	if storedCode == code.Code || storedCode != hashDesktopExchangeCode(code.Code) {
		t.Fatalf("stored code = %q, want SHA-256 digest and not raw callback code", storedCode)
	}
	if storedAccess != "" || storedRefresh != "" {
		t.Fatalf("legacy bearer columns = (%q, %q), want empty", storedAccess, storedRefresh)
	}

	consumed, err := store.ConsumeDesktopExchangeCode(code.Code)
	if err != nil {
		t.Fatalf("ConsumeDesktopExchangeCode: %v", err)
	}
	if consumed.UserID != user.ID {
		t.Errorf("UserID: got %q, want %q", consumed.UserID, user.ID)
	}
	if consumed.Code != code.Code {
		t.Errorf("Code: got %q, want presented raw code", consumed.Code)
	}

	// Second consume should fail (code is deleted).
	_, err = store.ConsumeDesktopExchangeCode("test-code-1")
	if err == nil {
		t.Error("expected error on second consume, got nil")
	}
}

func TestConsumeExpiredDesktopExchangeCode(t *testing.T) {
	store := TestStore(t)

	user, err := store.CreateUser("user-ex-2", "expired@example.com")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	now := time.Now().UTC()
	code := &DesktopExchangeCode{
		Code:      "expired-code-1",
		UserID:    user.ID,
		CreatedAt: now.Add(-2 * time.Minute),
		ExpiresAt: now.Add(-1 * time.Minute), // already expired
	}
	if err := store.CreateDesktopExchangeCode(code); err != nil {
		t.Fatalf("CreateDesktopExchangeCode: %v", err)
	}

	_, err = store.ConsumeDesktopExchangeCode("expired-code-1")
	if err == nil {
		t.Error("expected error for expired code, got nil")
	}
}

func TestBootstrapScrubsLegacyDesktopExchangeBearerValues(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "legacy-desktop-exchange.db")
	store, err := OpenStore(dbPath)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	user, err := store.CreateUser("legacy-desktop-user", "legacy-desktop@example.com")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	now := time.Now().UTC()
	if _, err := store.DB().Exec(
		"INSERT INTO desktop_exchange_codes (code, user_id, access_token, refresh_token, created_at, expires_at) VALUES (?, ?, ?, ?, ?, ?)",
		"legacy-raw-code", user.ID, "legacy-access-bearer", "legacy-refresh-bearer", now, now.Add(time.Minute),
	); err != nil {
		t.Fatalf("insert legacy handoff: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close legacy store: %v", err)
	}

	reopened, err := OpenStore(dbPath)
	if err != nil {
		t.Fatalf("reopen legacy store: %v", err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	var accessValue, refreshValue string
	if err := reopened.DB().QueryRow(
		"SELECT access_token, refresh_token FROM desktop_exchange_codes WHERE code = ?",
		"legacy-raw-code",
	).Scan(&accessValue, &refreshValue); err != nil {
		t.Fatalf("read scrubbed legacy handoff: %v", err)
	}
	if accessValue != "" || refreshValue != "" {
		t.Fatalf("scrubbed legacy values = (%q, %q), want empty", accessValue, refreshValue)
	}
	if _, err := reopened.ConsumeDesktopExchangeCode("legacy-raw-code"); err == nil {
		t.Fatal("pre-upgrade raw code unexpectedly survived hashed-code cutover")
	}
}
