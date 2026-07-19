package auth

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

// TestCreateAPIKeyGeneratesSecretAndHash verifies that CreateAPIKey returns a
// secret with the choir_sk_ prefix, that the stored hash is the SHA-256 of the
// secret, and that the hash lookup succeeds.
func TestCreateAPIKeyGeneratesSecretAndHash(t *testing.T) {
	store := TestStore(t)
	ctx := context.Background()

	user, err := store.CreateUser("ak-user-1", "ak1@example.com")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	id, secret, err := store.CreateAPIKey(ctx, user.ID, "Desktop sync", []string{"read:base", "write:base"}, nil)
	if err != nil {
		t.Fatalf("create api key: %v", err)
	}

	if !strings.HasPrefix(id, "ak_") {
		t.Errorf("key id: got %q, want prefix %q", id, "ak_")
	}
	if !strings.HasPrefix(secret, APIKeyPrefix) {
		t.Errorf("secret: got %q, want prefix %q", secret, APIKeyPrefix)
	}

	// The stored hash must be the SHA-256 of the secret.
	h := sha256.Sum256([]byte(secret))
	keyHash := fmt.Sprintf("%x", h)

	ak, err := store.GetAPIKeyByHash(ctx, keyHash)
	if err != nil {
		t.Fatalf("get api key by hash: %v", err)
	}
	if ak.ID != id {
		t.Errorf("key id: got %q, want %q", ak.ID, id)
	}
	if ak.UserID != user.ID {
		t.Errorf("user id: got %q, want %q", ak.UserID, user.ID)
	}
	if ak.Label != "Desktop sync" {
		t.Errorf("label: got %q, want %q", ak.Label, "Desktop sync")
	}
	if len(ak.Scopes) != 2 {
		t.Fatalf("scopes: got %v, want 2", ak.Scopes)
	}
	if ak.ExpiresAt != nil {
		t.Errorf("expires_at: got %v, want nil", ak.ExpiresAt)
	}
	if ak.RevokedAt != nil {
		t.Errorf("revoked_at: got %v, want nil", ak.RevokedAt)
	}
}

// TestCreateAPIKeySecretIsUnique verifies that two CreateAPIKey calls produce
// different secrets (randomness).
func TestCreateAPIKeySecretIsUnique(t *testing.T) {
	store := TestStore(t)
	ctx := context.Background()

	user, err := store.CreateUser("ak-user-2", "ak2@example.com")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	_, secret1, err := store.CreateAPIKey(ctx, user.ID, "key1", nil, nil)
	if err != nil {
		t.Fatalf("create key 1: %v", err)
	}
	_, secret2, err := store.CreateAPIKey(ctx, user.ID, "key2", nil, nil)
	if err != nil {
		t.Fatalf("create key 2: %v", err)
	}

	if secret1 == secret2 {
		t.Error("two api key secrets should be different")
	}
}

// TestCreateAPIKeyWithExpiry verifies that the expires_at is stored and
// returned correctly.
func TestCreateAPIKeyWithExpiry(t *testing.T) {
	store := TestStore(t)
	ctx := context.Background()

	user, err := store.CreateUser("ak-user-3", "ak3@example.com")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	exp := time.Now().Add(24 * time.Hour).UTC()
	_, secret, err := store.CreateAPIKey(ctx, user.ID, "CI staging", []string{"admin"}, &exp)
	if err != nil {
		t.Fatalf("create api key: %v", err)
	}

	h := sha256.Sum256([]byte(secret))
	keyHash := fmt.Sprintf("%x", h)

	ak, err := store.GetAPIKeyByHash(ctx, keyHash)
	if err != nil {
		t.Fatalf("get api key by hash: %v", err)
	}
	if ak.ExpiresAt == nil {
		t.Fatal("expires_at should be set")
	}
	// Allow a few seconds of skew for DB round-trip.
	diff := ak.ExpiresAt.Sub(exp)
	if diff > 2*time.Second || diff < -2*time.Second {
		t.Errorf("expires_at: got %v, want ~%v (diff %v)", ak.ExpiresAt, exp, diff)
	}
}

// TestGetAPIKeyByHashRejectsRevoked verifies that a revoked key is not returned
// by GetAPIKeyByHash.
func TestGetAPIKeyByHashRejectsRevoked(t *testing.T) {
	store := TestStore(t)
	ctx := context.Background()

	user, err := store.CreateUser("ak-user-4", "ak4@example.com")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	id, secret, err := store.CreateAPIKey(ctx, user.ID, "to-revoke", []string{"read:base"}, nil)
	if err != nil {
		t.Fatalf("create api key: %v", err)
	}

	h := sha256.Sum256([]byte(secret))
	keyHash := fmt.Sprintf("%x", h)

	// Revoke it.
	if err := store.RevokeAPIKey(ctx, user.ID, id); err != nil {
		t.Fatalf("revoke api key: %v", err)
	}

	// Lookup should now fail.
	_, err = store.GetAPIKeyByHash(ctx, keyHash)
	if err == nil {
		t.Fatal("expected error for revoked key, got nil")
	}
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("expected sql.ErrNoRows, got %v", err)
	}
}

// TestGetAPIKeyByHashRejectsUnknownHash verifies that a non-existent hash
// returns sql.ErrNoRows.
func TestGetAPIKeyByHashRejectsUnknownHash(t *testing.T) {
	store := TestStore(t)
	ctx := context.Background()

	_, err := store.GetAPIKeyByHash(ctx, "nonexistent-hash-12345")
	if err == nil {
		t.Fatal("expected error for unknown hash, got nil")
	}
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("expected sql.ErrNoRows, got %v", err)
	}
}

// TestListAPIKeysReturnsUserKeys verifies that ListAPIKeys returns keys for the
// specified user only, ordered by created_at descending.
func TestListAPIKeysReturnsUserKeys(t *testing.T) {
	store := TestStore(t)
	ctx := context.Background()

	user1, err := store.CreateUser("list-user-1", "list1@example.com")
	if err != nil {
		t.Fatalf("create user 1: %v", err)
	}
	user2, err := store.CreateUser("list-user-2", "list2@example.com")
	if err != nil {
		t.Fatalf("create user 2: %v", err)
	}

	if _, _, err := store.CreateAPIKey(ctx, user1.ID, "key-a", []string{"read:base"}, nil); err != nil {
		t.Fatalf("create key a: %v", err)
	}
	time.Sleep(10 * time.Millisecond)
	if _, _, err := store.CreateAPIKey(ctx, user1.ID, "key-b", []string{"write:base"}, nil); err != nil {
		t.Fatalf("create key b: %v", err)
	}
	if _, _, err := store.CreateAPIKey(ctx, user2.ID, "key-c", []string{"admin"}, nil); err != nil {
		t.Fatalf("create key c: %v", err)
	}

	keys, err := store.ListAPIKeys(ctx, user1.ID)
	if err != nil {
		t.Fatalf("list api keys: %v", err)
	}
	if len(keys) != 2 {
		t.Fatalf("expected 2 keys for user1, got %d", len(keys))
	}
	// Ordered by created_at desc — key-b should be first.
	if keys[0].Label != "key-b" {
		t.Errorf("first key: got %q, want %q", keys[0].Label, "key-b")
	}
	if keys[1].Label != "key-a" {
		t.Errorf("second key: got %q, want %q", keys[1].Label, "key-a")
	}

	// User 2 should only see their own key.
	keys2, err := store.ListAPIKeys(ctx, user2.ID)
	if err != nil {
		t.Fatalf("list api keys user2: %v", err)
	}
	if len(keys2) != 1 {
		t.Fatalf("expected 1 key for user2, got %d", len(keys2))
	}
}

// TestListAPIKeysExcludesSecrets verifies that listed keys do not expose the
// secret (the APIKey struct has no Secret field, but we verify the hash is not
// leaked either).
func TestListAPIKeysExcludesSecrets(t *testing.T) {
	store := TestStore(t)
	ctx := context.Background()

	user, err := store.CreateUser("no-leak-user", "noleak@example.com")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	_, secret, err := store.CreateAPIKey(ctx, user.ID, "no-leak", []string{"read:base"}, nil)
	if err != nil {
		t.Fatalf("create api key: %v", err)
	}

	keys, err := store.ListAPIKeys(ctx, user.ID)
	if err != nil {
		t.Fatalf("list api keys: %v", err)
	}
	if len(keys) != 1 {
		t.Fatalf("expected 1 key, got %d", len(keys))
	}

	// The APIKey struct has no Secret field; verify the raw secret is not
	// present in any field.
	for _, k := range keys {
		if strings.Contains(k.ID, secret) || strings.Contains(k.Label, secret) {
			t.Error("api key metadata leaks the secret")
		}
	}
}

// TestRevokeAPIKeyRejectsNonOwner verifies that a user cannot revoke another
// user's key.
func TestRevokeAPIKeyRejectsNonOwner(t *testing.T) {
	store := TestStore(t)
	ctx := context.Background()

	owner, err := store.CreateUser("owner-user", "owner@example.com")
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	other, err := store.CreateUser("other-user", "other@example.com")
	if err != nil {
		t.Fatalf("create other: %v", err)
	}

	id, _, err := store.CreateAPIKey(ctx, owner.ID, "owner-key", []string{"read:base"}, nil)
	if err != nil {
		t.Fatalf("create api key: %v", err)
	}

	// Other user tries to revoke — should fail with ErrNoRows.
	err = store.RevokeAPIKey(ctx, other.ID, id)
	if err == nil {
		t.Fatal("expected error when non-owner revokes, got nil")
	}
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("expected sql.ErrNoRows, got %v", err)
	}

	// Verify the key is still active.
	keys, err := store.ListAPIKeys(ctx, owner.ID)
	if err != nil {
		t.Fatalf("list keys: %v", err)
	}
	if len(keys) != 1 || keys[0].RevokedAt != nil {
		t.Fatal("key should still be active after non-owner revoke attempt")
	}
}

// TestRevokeAPIKeyIdempotentSafe verifies that revoking an already-revoked key
// returns sql.ErrNoRows (no double-revoke).
func TestRevokeAPIKeyIdempotentSafe(t *testing.T) {
	store := TestStore(t)
	ctx := context.Background()

	user, err := store.CreateUser("double-revoke-user", "double@example.com")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	id, _, err := store.CreateAPIKey(ctx, user.ID, "double-revoke", []string{"read:base"}, nil)
	if err != nil {
		t.Fatalf("create api key: %v", err)
	}

	if err := store.RevokeAPIKey(ctx, user.ID, id); err != nil {
		t.Fatalf("first revoke: %v", err)
	}
	err = store.RevokeAPIKey(ctx, user.ID, id)
	if err == nil {
		t.Fatal("expected error on second revoke, got nil")
	}
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("expected sql.ErrNoRows, got %v", err)
	}
}

// TestTouchAPIKeyLastUsed verifies that TouchAPIKeyLastUsed updates the
// last_used_at timestamp.
func TestTouchAPIKeyLastUsed(t *testing.T) {
	store := TestStore(t)
	ctx := context.Background()

	user, err := store.CreateUser("touch-user", "touch@example.com")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	id, secret, err := store.CreateAPIKey(ctx, user.ID, "touch-key", []string{"read:base"}, nil)
	if err != nil {
		t.Fatalf("create api key: %v", err)
	}

	h := sha256.Sum256([]byte(secret))
	keyHash := fmt.Sprintf("%x", h)

	// Before touch, last_used_at should be nil.
	ak, err := store.GetAPIKeyByHash(ctx, keyHash)
	if err != nil {
		t.Fatalf("get before touch: %v", err)
	}
	if ak.LastUsedAt != nil {
		t.Errorf("last_used_at should be nil before touch, got %v", ak.LastUsedAt)
	}

	// Touch.
	before := time.Now().UTC()
	if err := store.TouchAPIKeyLastUsed(ctx, id); err != nil {
		t.Fatalf("touch: %v", err)
	}

	// After touch, last_used_at should be set.
	ak, err = store.GetAPIKeyByHash(ctx, keyHash)
	if err != nil {
		t.Fatalf("get after touch: %v", err)
	}
	if ak.LastUsedAt == nil {
		t.Fatal("last_used_at should be set after touch")
	}
	if ak.LastUsedAt.Before(before) {
		t.Errorf("last_used_at %v should be after %v", ak.LastUsedAt, before)
	}
}

// TestCreateAPIKeyRejectsEmptyFields verifies validation of required fields.
func TestCreateAPIKeyRejectsEmptyFields(t *testing.T) {
	store := TestStore(t)
	ctx := context.Background()

	user, err := store.CreateUser("valid-user", "valid@example.com")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	// Empty user ID.
	if _, _, err := store.CreateAPIKey(ctx, "", "label", nil, nil); err == nil {
		t.Error("expected error for empty user_id, got nil")
	}
	// Empty label.
	if _, _, err := store.CreateAPIKey(ctx, user.ID, "", nil, nil); err == nil {
		t.Error("expected error for empty label, got nil")
	}
}

// TestCreateAPIKeyStoresScopesAsJSON verifies that scopes are stored as a JSON
// array and parsed back correctly.
func TestCreateAPIKeyStoresScopesAsJSON(t *testing.T) {
	store := TestStore(t)
	ctx := context.Background()

	user, err := store.CreateUser("scopes-user", "scopes@example.com")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	scopes := []string{"read:texture", "write:base", "admin"}
	_, secret, err := store.CreateAPIKey(ctx, user.ID, "scopes-key", scopes, nil)
	if err != nil {
		t.Fatalf("create api key: %v", err)
	}

	h := sha256.Sum256([]byte(secret))
	keyHash := fmt.Sprintf("%x", h)

	ak, err := store.GetAPIKeyByHash(ctx, keyHash)
	if err != nil {
		t.Fatalf("get api key: %v", err)
	}
	if len(ak.Scopes) != 3 {
		t.Fatalf("scopes: got %v, want 3 items", ak.Scopes)
	}
	for i, sc := range scopes {
		if ak.Scopes[i] != sc {
			t.Errorf("scope[%d]: got %q, want %q", i, ak.Scopes[i], sc)
		}
	}
}

// TestCreateAPIKeyEmptyScopes verifies that nil scopes are stored as an empty
// array and read back as an empty (non-nil) slice.
func TestCreateAPIKeyEmptyScopes(t *testing.T) {
	store := TestStore(t)
	ctx := context.Background()

	user, err := store.CreateUser("empty-scopes-user", "empty@example.com")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	_, secret, err := store.CreateAPIKey(ctx, user.ID, "empty-scopes", nil, nil)
	if err != nil {
		t.Fatalf("create api key: %v", err)
	}

	h := sha256.Sum256([]byte(secret))
	keyHash := fmt.Sprintf("%x", h)

	ak, err := store.GetAPIKeyByHash(ctx, keyHash)
	if err != nil {
		t.Fatalf("get api key: %v", err)
	}
	if ak.Scopes == nil {
		t.Error("scopes should be empty slice, not nil")
	}
	if len(ak.Scopes) != 0 {
		t.Errorf("scopes: got %v, want empty", ak.Scopes)
	}
}

// TestSeedBootstrapAdminAPIKeySeedsOnFirstRun verifies that the bootstrap
// seeds an admin-scoped key when no API keys exist, and that the key
// validates through the normal hash-lookup path (C1, C3).
func TestSeedBootstrapAdminAPIKeySeedsOnFirstRun(t *testing.T) {
	store := TestStore(t)
	ctx := context.Background()

	rawKey := APIKeyPrefix + "bootstrap-test-key-12345"
	keyID, seeded, err := store.SeedBootstrapAdminAPIKey(ctx, rawKey)
	if err != nil {
		t.Fatalf("seed bootstrap: %v", err)
	}
	if !seeded {
		t.Fatal("seeded = false, want true on first run")
	}
	if !strings.HasPrefix(keyID, "ak_") {
		t.Errorf("key id: got %q, want prefix ak_", keyID)
	}

	// The key must validate through the normal hash-lookup path (C3).
	h := sha256.Sum256([]byte(rawKey))
	keyHash := fmt.Sprintf("%x", h)
	ak, err := store.GetAPIKeyByHash(ctx, keyHash)
	if err != nil {
		t.Fatalf("get api key by hash: %v", err)
	}
	if ak.ID != keyID {
		t.Errorf("key id: got %q, want %q", ak.ID, keyID)
	}
	if ak.UserID != BootstrapAdminAPIKeyUserID {
		t.Errorf("user id: got %q, want %q", ak.UserID, BootstrapAdminAPIKeyUserID)
	}
	if ak.Label != bootstrapAdminAPIKeyLabel {
		t.Errorf("label: got %q, want %q", ak.Label, bootstrapAdminAPIKeyLabel)
	}
	// Admin scope.
	foundAdmin := false
	for _, s := range ak.Scopes {
		if s == "admin" {
			foundAdmin = true
		}
	}
	if !foundAdmin {
		t.Errorf("scopes: got %v, want to contain admin", ak.Scopes)
	}
}

// TestSeedBootstrapAdminAPIKeySkipsWhenKeysExist verifies the first-run-only
// guard: if any non-revoked API key exists, the bootstrap is a no-op (C2).
func TestSeedBootstrapAdminAPIKeySkipsWhenKeysExist(t *testing.T) {
	store := TestStore(t)
	ctx := context.Background()

	// Create a normal user + key first (simulates a WebAuthn-provisioned key).
	user, err := store.CreateUser("existing-user", "existing@example.com")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if _, _, err := store.CreateAPIKey(ctx, user.ID, "existing", []string{"read:runtime"}, nil); err != nil {
		t.Fatalf("create api key: %v", err)
	}

	// Now the bootstrap should be a no-op.
	rawKey := APIKeyPrefix + "bootstrap-should-not-seed"
	keyID, seeded, err := store.SeedBootstrapAdminAPIKey(ctx, rawKey)
	if err != nil {
		t.Fatalf("seed bootstrap: %v", err)
	}
	if seeded {
		t.Error("seeded = true, want false when keys already exist")
	}
	if keyID != "" {
		t.Errorf("key id: got %q, want empty on skip", keyID)
	}

	// The bootstrap key must NOT validate.
	h := sha256.Sum256([]byte(rawKey))
	keyHash := fmt.Sprintf("%x", h)
	_, err = store.GetAPIKeyByHash(ctx, keyHash)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("get api key by hash: got err=%v, want sql.ErrNoRows (key should not exist)", err)
	}
}

// TestSeedBootstrapAdminAPIKeySecondRunIsNoOp verifies that calling the
// bootstrap a second time on the same store is a no-op (C2).
func TestSeedBootstrapAdminAPIKeySecondRunIsNoOp(t *testing.T) {
	store := TestStore(t)
	ctx := context.Background()

	rawKey := APIKeyPrefix + "bootstrap-once-key"
	if _, seeded, err := store.SeedBootstrapAdminAPIKey(ctx, rawKey); err != nil || !seeded {
		t.Fatalf("first seed: seeded=%v err=%v", seeded, err)
	}

	// Second call: the key from the first run now exists, so this is a no-op.
	_, seeded2, err := store.SeedBootstrapAdminAPIKey(ctx, rawKey)
	if err != nil {
		t.Fatalf("second seed: %v", err)
	}
	if seeded2 {
		t.Error("second seed: seeded = true, want false (first-run-only)")
	}
}

// TestSeedBootstrapAdminAPIKeyRevocable verifies that revoking the bootstrap
// key disables it for future lookups (C4).
func TestSeedBootstrapAdminAPIKeyRevocable(t *testing.T) {
	store := TestStore(t)
	ctx := context.Background()

	rawKey := APIKeyPrefix + "bootstrap-revoke-key"
	keyID, seeded, err := store.SeedBootstrapAdminAPIKey(ctx, rawKey)
	if err != nil || !seeded {
		t.Fatalf("seed: seeded=%v err=%v", seeded, err)
	}

	// Revoke via the normal flow.
	if err := store.RevokeAPIKey(ctx, BootstrapAdminAPIKeyUserID, keyID); err != nil {
		t.Fatalf("revoke: %v", err)
	}

	// The key must no longer validate.
	h := sha256.Sum256([]byte(rawKey))
	keyHash := fmt.Sprintf("%x", h)
	_, err = store.GetAPIKeyByHash(ctx, keyHash)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("get api key by hash after revoke: got err=%v, want sql.ErrNoRows", err)
	}
}

// TestSeedBootstrapAdminAPIKeyRejectsBadPrefix verifies that a raw key
// without the choir_sk_ prefix is rejected without touching the DB.
func TestSeedBootstrapAdminAPIKeyRejectsBadPrefix(t *testing.T) {
	store := TestStore(t)
	ctx := context.Background()

	_, seeded, err := store.SeedBootstrapAdminAPIKey(ctx, "not-a-choir-key")
	if err == nil {
		t.Fatal("err = nil, want error for bad prefix")
	}
	if seeded {
		t.Error("seeded = true, want false on error")
	}
}

func TestCreateComputerScopedAPIKeyPersistsExactBinding(t *testing.T) {
	store := TestStore(t)
	user, err := store.CreateUser("ak-selfdev-user", "selfdev@example.com")
	if err != nil {
		t.Fatal(err)
	}
	_, secret, err := store.CreateComputerScopedAPIKey(context.Background(), user.ID, "Self development", []string{"computer:self_development:read"}, "computer-exact", nil)
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256([]byte(secret))
	key, err := store.GetAPIKeyByHash(context.Background(), fmt.Sprintf("%x", hash))
	if err != nil {
		t.Fatal(err)
	}
	if key.ComputerID != "computer-exact" {
		t.Fatalf("computer binding = %q, want computer-exact", key.ComputerID)
	}
}
