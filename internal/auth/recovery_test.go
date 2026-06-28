package auth

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// --- Recovery Token Store Tests ---

func TestCreateRecoveryTokenStoresHashNotPlaintext(t *testing.T) {
	store := TestStore(t)
	ctx := context.Background()

	user, err := store.CreateUser("rt-user-1", "rt1@example.com")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	emailHash := hashEmail("rt1@example.com")
	ipHash := hashIP("127.0.0.1")

	token, err := store.CreateRecoveryToken(ctx, user.ID, "rt1@example.com", emailHash, ipHash)
	if err != nil {
		t.Fatalf("create recovery token: %v", err)
	}

	if !strings.HasPrefix(token, RecoveryTokenPrefix) {
		t.Errorf("token: got %q, want prefix %q", token, RecoveryTokenPrefix)
	}

	// Verify the raw token is NOT stored in the database.
	var tokenCount int
	err = store.DB().QueryRow(
		"SELECT COUNT(*) FROM recovery_tokens WHERE token_hash = ?", token,
	).Scan(&tokenCount)
	if err != nil {
		t.Fatalf("query token: %v", err)
	}
	if tokenCount != 0 {
		t.Error("raw token should not be stored as the hash")
	}

	// Verify the SHA-256 hash IS stored.
	h := sha256SumHex(token)
	var hashCount int
	err = store.DB().QueryRow(
		"SELECT COUNT(*) FROM recovery_tokens WHERE token_hash = ?", h,
	).Scan(&hashCount)
	if err != nil {
		t.Fatalf("query hash: %v", err)
	}
	if hashCount != 1 {
		t.Errorf("expected 1 row with matching hash, got %d", hashCount)
	}
}

func TestConsumeRecoveryTokenValid(t *testing.T) {
	store := TestStore(t)
	ctx := context.Background()

	user, err := store.CreateUser("rt-user-2", "rt2@example.com")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	emailHash := hashEmail("rt2@example.com")
	ipHash := hashIP("127.0.0.1")

	token, err := store.CreateRecoveryToken(ctx, user.ID, "rt2@example.com", emailHash, ipHash)
	if err != nil {
		t.Fatalf("create recovery token: %v", err)
	}

	rt, err := store.ConsumeRecoveryToken(ctx, token)
	if err != nil {
		t.Fatalf("consume recovery token: %v", err)
	}
	if rt.UserID != user.ID {
		t.Errorf("user_id: got %q, want %q", rt.UserID, user.ID)
	}
	if rt.Email != "rt2@example.com" {
		t.Errorf("email: got %q, want %q", rt.Email, "rt2@example.com")
	}
}

func TestConsumeRecoveryTokenRejectsReuse(t *testing.T) {
	store := TestStore(t)
	ctx := context.Background()

	user, err := store.CreateUser("rt-user-3", "rt3@example.com")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	token, err := store.CreateRecoveryToken(ctx, user.ID, "rt3@example.com", hashEmail("rt3@example.com"), hashIP("127.0.0.1"))
	if err != nil {
		t.Fatalf("create recovery token: %v", err)
	}

	// First use should succeed.
	_, err = store.ConsumeRecoveryToken(ctx, token)
	if err != nil {
		t.Fatalf("first consume: %v", err)
	}

	// Second use should fail (single-use).
	_, err = store.ConsumeRecoveryToken(ctx, token)
	if err == nil {
		t.Fatal("expected error on reuse, got nil")
	}
	if !strings.Contains(err.Error(), "already used") {
		t.Errorf("expected 'already used' error, got %q", err.Error())
	}
}

func TestConsumeRecoveryTokenRejectsExpired(t *testing.T) {
	store := TestStore(t)
	ctx := context.Background()

	user, err := store.CreateUser("rt-user-4", "rt4@example.com")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	// Create a token and manually expire it.
	token, err := store.CreateRecoveryToken(ctx, user.ID, "rt4@example.com", hashEmail("rt4@example.com"), hashIP("127.0.0.1"))
	if err != nil {
		t.Fatalf("create recovery token: %v", err)
	}

	// Manually set expires_at to the past.
	h := sha256SumHex(token)
	_, err = store.DB().Exec(
		"UPDATE recovery_tokens SET expires_at = ? WHERE token_hash = ?",
		time.Now().UTC().Add(-1*time.Minute), h,
	)
	if err != nil {
		t.Fatalf("expire token: %v", err)
	}

	_, err = store.ConsumeRecoveryToken(ctx, token)
	if err == nil {
		t.Fatal("expected error for expired token, got nil")
	}
	if !strings.Contains(err.Error(), "expired") {
		t.Errorf("expected 'expired' error, got %q", err.Error())
	}
}

func TestConsumeRecoveryTokenRejectsUnknown(t *testing.T) {
	store := TestStore(t)
	ctx := context.Background()

	_, err := store.ConsumeRecoveryToken(ctx, "choir_rt_nonexistent_token_12345")
	if err == nil {
		t.Fatal("expected error for unknown token, got nil")
	}
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("expected sql.ErrNoRows, got %v", err)
	}
}

func TestConsumeRecoveryTokenRejectsDummyRecord(t *testing.T) {
	store := TestStore(t)
	ctx := context.Background()

	// Create a token with no user_id (anti-enumeration dummy).
	token, err := store.CreateRecoveryToken(ctx, "", "nonexistent@example.com", hashEmail("nonexistent@example.com"), hashIP("127.0.0.1"))
	if err != nil {
		t.Fatalf("create dummy recovery token: %v", err)
	}

	_, err = store.ConsumeRecoveryToken(ctx, token)
	if err == nil {
		t.Fatal("expected error for dummy token, got nil")
	}
	if !strings.Contains(err.Error(), "no associated user") {
		t.Errorf("expected 'no associated user' error, got %q", err.Error())
	}
}

// --- Rate Limiting Store Tests ---

func TestCountRecoveryTokensByEmailSince(t *testing.T) {
	store := TestStore(t)
	ctx := context.Background()

	emailHash := hashEmail("rate@example.com")
	ipHash := hashIP("127.0.0.1")

	// Create 3 tokens.
	for i := 0; i < 3; i++ {
		_, err := store.CreateRecoveryToken(ctx, "user-1", "rate@example.com", emailHash, ipHash)
		if err != nil {
			t.Fatalf("create token %d: %v", i, err)
		}
	}

	count, err := store.CountRecoveryTokensByEmailSince(ctx, emailHash, time.Now().UTC().Add(-1*time.Hour))
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 3 {
		t.Errorf("count: got %d, want 3", count)
	}

	// Count with a future cutoff should return 0.
	count, err = store.CountRecoveryTokensByEmailSince(ctx, emailHash, time.Now().UTC().Add(1*time.Hour))
	if err != nil {
		t.Fatalf("count future: %v", err)
	}
	if count != 0 {
		t.Errorf("count future: got %d, want 0", count)
	}
}

func TestCountRecoveryTokensByIPSince(t *testing.T) {
	store := TestStore(t)
	ctx := context.Background()

	emailHash := hashEmail("ip1@example.com")
	ipHash := hashIP("10.0.0.1")

	_, err := store.CreateRecoveryToken(ctx, "user-1", "ip1@example.com", emailHash, ipHash)
	if err != nil {
		t.Fatalf("create token: %v", err)
	}

	count, err := store.CountRecoveryTokensByIPSince(ctx, ipHash, time.Now().UTC().Add(-1*time.Hour))
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Errorf("count: got %d, want 1", count)
	}

	// Different IP should return 0.
	count, err = store.CountRecoveryTokensByIPSince(ctx, hashIP("10.0.0.2"), time.Now().UTC().Add(-1*time.Hour))
	if err != nil {
		t.Fatalf("count different IP: %v", err)
	}
	if count != 0 {
		t.Errorf("count different IP: got %d, want 0", count)
	}
}

// --- Recovery Request Handler Tests ---

func TestRecoveryRequestRejectsNonPost(t *testing.T) {
	h, _ := testHandlerEnv(t)

	req := httptest.NewRequest(http.MethodGet, "/auth/recovery/request", nil)
	rec := httptest.NewRecorder()
	h.HandleRecoveryRequest(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestRecoveryRequestRejectsEmptyEmail(t *testing.T) {
	h, _ := testHandlerEnv(t)

	body := `{}`
	req := httptest.NewRequest(http.MethodPost, "/auth/recovery/request", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.HandleRecoveryRequest(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestRecoveryRequestRejectsInvalidEmail(t *testing.T) {
	h, _ := testHandlerEnv(t)

	body := `{"email":"not-an-email"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/recovery/request", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.HandleRecoveryRequest(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestRecoveryRequestSucceedsForExistingUser(t *testing.T) {
	h, _ := testHandlerEnv(t)

	user, err := h.store.CreateUser("rec-user-1", "rec1@example.com")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	_ = user

	body := `{"email":"rec1@example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/recovery/request", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.HandleRecoveryRequest(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp recoveryResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.OK {
		t.Error("ok should be true")
	}
	if !strings.HasPrefix(resp.Token, RecoveryTokenPrefix) {
		t.Errorf("token: got %q, want prefix %q", resp.Token, RecoveryTokenPrefix)
	}

	// Verify the token hash is stored in the DB (not the raw token).
	h2 := sha256SumHex(resp.Token)
	var count int
	err = h.store.DB().QueryRow(
		"SELECT COUNT(*) FROM recovery_tokens WHERE token_hash = ?", h2,
	).Scan(&count)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 row with matching hash, got %d", count)
	}
}

func TestRecoveryRequestSucceedsForNonexistentUser(t *testing.T) {
	h, _ := testHandlerEnv(t)

	// Request recovery for a non-existent user — should still return OK
	// (anti-enumeration).
	body := `{"email":"nonexistent@example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/recovery/request", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.HandleRecoveryRequest(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp recoveryResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.OK {
		t.Error("ok should be true (anti-enumeration)")
	}

	// The token should not be usable (dummy record with no user_id).
	_, err := h.store.ConsumeRecoveryToken(context.Background(), resp.Token)
	if err == nil {
		t.Error("dummy token should not be consumable")
	}
}

func TestRecoveryRequestRateLimitsByEmail(t *testing.T) {
	h, _ := testHandlerEnv(t)

	h.store.CreateUser("rate-email-user", "rateemail@example.com")

	// Make 3 requests (the limit).
	for i := 0; i < RecoveryMaxPerEmail; i++ {
		body := `{"email":"rateemail@example.com"}`
		req := httptest.NewRequest(http.MethodPost, "/auth/recovery/request", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "192.168.1.100:12345"
		rec := httptest.NewRecorder()
		h.HandleRecoveryRequest(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: got %d, want %d; body: %s", i+1, rec.Code, http.StatusOK, rec.Body.String())
		}
	}

	// 4th request should be rate limited.
	body := `{"email":"rateemail@example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/recovery/request", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "192.168.1.100:12345"
	rec := httptest.NewRecorder()
	h.HandleRecoveryRequest(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("4th request: got %d, want %d; body: %s", rec.Code, http.StatusTooManyRequests, rec.Body.String())
	}
}

func TestRecoveryRequestRateLimitsByIP(t *testing.T) {
	h, _ := testHandlerEnv(t)

	// Make 5 requests from the same IP but different emails (IP limit is 5).
	for i := 0; i < RecoveryMaxPerIP; i++ {
		email := fmt.Sprintf("ip%d@example.com", i)
		body := fmt.Sprintf(`{"email":%q}`, email)
		req := httptest.NewRequest(http.MethodPost, "/auth/recovery/request", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "10.0.0.1:12345"
		rec := httptest.NewRecorder()
		h.HandleRecoveryRequest(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: got %d, want %d; body: %s", i+1, rec.Code, http.StatusOK, rec.Body.String())
		}
	}

	// 6th request from the same IP should be rate limited.
	body := `{"email":"ip6@example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/recovery/request", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "10.0.0.1:12345"
	rec := httptest.NewRecorder()
	h.HandleRecoveryRequest(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("6th request: got %d, want %d; body: %s", rec.Code, http.StatusTooManyRequests, rec.Body.String())
	}
}

// --- Recovery Verify Handler Tests ---

func TestRecoveryVerifyRejectsNonPost(t *testing.T) {
	h, _ := testHandlerEnv(t)

	req := httptest.NewRequest(http.MethodGet, "/auth/recovery/verify", nil)
	rec := httptest.NewRecorder()
	h.HandleRecoveryVerify(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestRecoveryVerifyRejectsEmptyToken(t *testing.T) {
	h, _ := testHandlerEnv(t)

	body := `{}`
	req := httptest.NewRequest(http.MethodPost, "/auth/recovery/verify", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.HandleRecoveryVerify(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestRecoveryVerifyRejectsUnknownToken(t *testing.T) {
	h, _ := testHandlerEnv(t)

	body := `{"token":"choir_rt_nonexistent"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/recovery/verify", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.HandleRecoveryVerify(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestRecoveryVerifyRejectsReusedToken(t *testing.T) {
	h, _ := testHandlerEnv(t)

	user, err := h.store.CreateUser("rv-user-2", "rv2@example.com")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	token, err := h.store.CreateRecoveryToken(context.Background(), user.ID, "rv2@example.com", hashEmail("rv2@example.com"), hashIP("127.0.0.1"))
	if err != nil {
		t.Fatalf("create token: %v", err)
	}

	// First verify should succeed.
	body := fmt.Sprintf(`{"token":%q}`, token)
	req := httptest.NewRequest(http.MethodPost, "/auth/recovery/verify", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.HandleRecoveryVerify(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("first verify: got %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	// Second verify should fail (single-use).
	req2 := httptest.NewRequest(http.MethodPost, "/auth/recovery/verify", bytes.NewBufferString(body))
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	h.HandleRecoveryVerify(rec2, req2)
	if rec2.Code != http.StatusBadRequest {
		t.Errorf("second verify: got %d, want %d", rec2.Code, http.StatusBadRequest)
	}
}

func TestRecoveryVerifyRejectsExpiredToken(t *testing.T) {
	h, _ := testHandlerEnv(t)

	user, err := h.store.CreateUser("rv-user-3", "rv3@example.com")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	token, err := h.store.CreateRecoveryToken(context.Background(), user.ID, "rv3@example.com", hashEmail("rv3@example.com"), hashIP("127.0.0.1"))
	if err != nil {
		t.Fatalf("create token: %v", err)
	}

	// Expire the token.
	h2 := sha256SumHex(token)
	_, err = h.store.DB().Exec(
		"UPDATE recovery_tokens SET expires_at = ? WHERE token_hash = ?",
		time.Now().UTC().Add(-1*time.Minute), h2,
	)
	if err != nil {
		t.Fatalf("expire token: %v", err)
	}

	body := fmt.Sprintf(`{"token":%q}`, token)
	req := httptest.NewRequest(http.MethodPost, "/auth/recovery/verify", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.HandleRecoveryVerify(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestRecoveryVerifyCreatesRegistrationChallenge(t *testing.T) {
	h, _ := testHandlerEnv(t)

	user, err := h.store.CreateUser("rv-user-4", "rv4@example.com")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	// Add an existing credential (user already has a passkey).
	cred := &Credential{
		ID:              "cred-rv-1",
		UserID:          user.ID,
		PublicKey:       make([]byte, 64),
		AttestationType: "none",
		Transport:       `["internal"]`,
		SignCount:       0,
		AAGUID:          make([]byte, 16),
		Flags:           `{"user_present":true,"user_verified":true,"backup_eligible":true,"backup_state":false}`,
		CreatedAt:       time.Now().UTC(),
	}
	if err := h.store.CreateCredential(cred); err != nil {
		t.Fatalf("create credential: %v", err)
	}

	token, err := h.store.CreateRecoveryToken(context.Background(), user.ID, "rv4@example.com", hashEmail("rv4@example.com"), hashIP("127.0.0.1"))
	if err != nil {
		t.Fatalf("create token: %v", err)
	}

	body := fmt.Sprintf(`{"token":%q}`, token)
	req := httptest.NewRequest(http.MethodPost, "/auth/recovery/verify", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.HandleRecoveryVerify(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	// The response should be WebAuthn registration options (same format as
	// register/begin).
	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	pk, ok := resp["publicKey"]
	if !ok {
		t.Fatal("response missing 'publicKey' field")
	}
	pkMap, ok := pk.(map[string]interface{})
	if !ok {
		t.Fatalf("publicKey is %T, not a map", pk)
	}
	challenge, ok := pkMap["challenge"].(string)
	if !ok || challenge == "" {
		t.Error("publicKey.challenge is missing or empty")
	}

	// Verify a registration challenge was stored in the DB.
	var challengeCount int
	err = h.store.DB().QueryRow(
		"SELECT COUNT(*) FROM challenge_state WHERE user_id = ? AND type = 'registration'",
		user.ID,
	).Scan(&challengeCount)
	if err != nil {
		t.Fatalf("query challenge: %v", err)
	}
	if challengeCount != 1 {
		t.Errorf("expected 1 registration challenge, got %d", challengeCount)
	}
}

// --- Credential Listing Handler Tests ---

func TestListCredentialsRejectsUnauthenticated(t *testing.T) {
	h, _ := testHandlerEnv(t)

	req := httptest.NewRequest(http.MethodGet, "/auth/credentials", nil)
	rec := httptest.NewRecorder()
	h.HandleListCredentials(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestListCredentialsReturnsCredentialsWithoutSecrets(t *testing.T) {
	h, priv := testHandlerEnv(t)

	user, err := h.store.CreateUser("cred-list-user", "credlist@example.com")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	// Create two credentials.
	for _, id := range []string{"cred-list-1", "cred-list-2"} {
		cred := &Credential{
			ID:              id,
			UserID:          user.ID,
			PublicKey:       []byte("secret-key-material-" + id),
			AttestationType: "none",
			Transport:       `["internal"]`,
			SignCount:       0,
			AAGUID:          make([]byte, 16),
			Flags:           "{}",
			CreatedAt:       time.Now().UTC(),
		}
		if err := h.store.CreateCredential(cred); err != nil {
			t.Fatalf("create credential %s: %v", id, err)
		}
	}

	req := authedAPIKeyReq(http.MethodGet, "/auth/credentials", nil, priv, user.ID)
	rec := httptest.NewRecorder()
	h.HandleListCredentials(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp listCredentialsResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Credentials) != 2 {
		t.Fatalf("expected 2 credentials, got %d", len(resp.Credentials))
	}

	// Verify no secret data is in the response.
	bodyStr := rec.Body.String()
	if strings.Contains(bodyStr, "secret-key-material") {
		t.Error("response leaks public key material")
	}
	if strings.Contains(bodyStr, "internal") {
		t.Error("response leaks transport info")
	}

	// Verify each credential has the expected fields.
	for _, c := range resp.Credentials {
		if c.ID == "" {
			t.Error("credential ID should be non-empty")
		}
		if c.CreatedAt == "" {
			t.Error("created_at should be non-empty")
		}
	}
}

// --- Credential Deletion Handler Tests ---

func TestDeleteCredentialRejectsUnauthenticated(t *testing.T) {
	h, _ := testHandlerEnv(t)

	req := httptest.NewRequest(http.MethodDelete, "/auth/credentials/cred-1", nil)
	rec := httptest.NewRecorder()
	h.HandleDeleteCredential(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestDeleteCredentialGuardsLastCredential(t *testing.T) {
	h, priv := testHandlerEnv(t)

	user, err := h.store.CreateUser("cred-del-last", "credlast@example.com")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	// Create only one credential.
	cred := &Credential{
		ID:              "cred-only-1",
		UserID:          user.ID,
		PublicKey:       make([]byte, 64),
		AttestationType: "none",
		Transport:       `["internal"]`,
		SignCount:       0,
		AAGUID:          make([]byte, 16),
		Flags:           "{}",
		CreatedAt:       time.Now().UTC(),
	}
	if err := h.store.CreateCredential(cred); err != nil {
		t.Fatalf("create credential: %v", err)
	}

	req := authedAPIKeyReq(http.MethodDelete, "/auth/credentials/cred-only-1", nil, priv, user.ID)
	rec := httptest.NewRecorder()
	h.HandleDeleteCredential(rec, req)

	if rec.Code != http.StatusConflict {
		t.Errorf("status: got %d, want %d (cannot delete last credential); body: %s", rec.Code, http.StatusConflict, rec.Body.String())
	}

	// Verify the credential still exists.
	creds, err := h.store.GetCredentialsByUserID(user.ID)
	if err != nil {
		t.Fatalf("get credentials: %v", err)
	}
	if len(creds) != 1 {
		t.Errorf("credential should still exist, got %d", len(creds))
	}
}

func TestDeleteCredentialSucceedsWithMultipleCredentials(t *testing.T) {
	h, priv := testHandlerEnv(t)

	user, err := h.store.CreateUser("cred-del-multi", "credmulti@example.com")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	// Create two credentials.
	for _, id := range []string{"cred-multi-1", "cred-multi-2"} {
		cred := &Credential{
			ID:              id,
			UserID:          user.ID,
			PublicKey:       make([]byte, 64),
			AttestationType: "none",
			Transport:       `["internal"]`,
			SignCount:       0,
			AAGUID:          make([]byte, 16),
			Flags:           "{}",
			CreatedAt:       time.Now().UTC(),
		}
		if err := h.store.CreateCredential(cred); err != nil {
			t.Fatalf("create credential %s: %v", id, err)
		}
	}

	req := authedAPIKeyReq(http.MethodDelete, "/auth/credentials/cred-multi-1", nil, priv, user.ID)
	rec := httptest.NewRecorder()
	h.HandleDeleteCredential(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status: got %d, want %d; body: %s", rec.Code, http.StatusNoContent, rec.Body.String())
	}

	// Verify only one credential remains.
	creds, err := h.store.GetCredentialsByUserID(user.ID)
	if err != nil {
		t.Fatalf("get credentials: %v", err)
	}
	if len(creds) != 1 {
		t.Errorf("expected 1 credential remaining, got %d", len(creds))
	}
	if creds[0].ID != "cred-multi-2" {
		t.Errorf("remaining credential: got %q, want %q", creds[0].ID, "cred-multi-2")
	}
}

func TestDeleteCredentialRejectsNonOwner(t *testing.T) {
	h, priv := testHandlerEnv(t)

	owner, err := h.store.CreateUser("cred-owner", "credowner@example.com")
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	other, err := h.store.CreateUser("cred-other", "credother@example.com")
	if err != nil {
		t.Fatalf("create other: %v", err)
	}

	// Owner has two credentials.
	for _, id := range []string{"cred-owner-1", "cred-owner-2"} {
		cred := &Credential{
			ID:              id,
			UserID:          owner.ID,
			PublicKey:       make([]byte, 64),
			AttestationType: "none",
			Transport:       `["internal"]`,
			SignCount:       0,
			AAGUID:          make([]byte, 16),
			Flags:           "{}",
			CreatedAt:       time.Now().UTC(),
		}
		if err := h.store.CreateCredential(cred); err != nil {
			t.Fatalf("create credential %s: %v", id, err)
		}
	}

	// Other user tries to delete owner's credential.
	req := authedAPIKeyReq(http.MethodDelete, "/auth/credentials/cred-owner-1", nil, priv, other.ID)
	rec := httptest.NewRecorder()
	h.HandleDeleteCredential(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d (non-owner)", rec.Code, http.StatusNotFound)
	}
}

// --- Credential Rename Handler Tests ---

func TestRenameCredentialRejectsUnauthenticated(t *testing.T) {
	h, _ := testHandlerEnv(t)

	body := `{"id":"cred-1","name":"My Phone"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/credentials/rename", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	h.HandleRenameCredential(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestRenameCredentialSucceeds(t *testing.T) {
	h, priv := testHandlerEnv(t)

	user, err := h.store.CreateUser("cred-rename-user", "credrename@example.com")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	cred := &Credential{
		ID:              "cred-rename-1",
		UserID:          user.ID,
		PublicKey:       make([]byte, 64),
		AttestationType: "none",
		Transport:       `["internal"]`,
		SignCount:       0,
		AAGUID:          make([]byte, 16),
		Flags:           "{}",
		CreatedAt:       time.Now().UTC(),
	}
	if err := h.store.CreateCredential(cred); err != nil {
		t.Fatalf("create credential: %v", err)
	}

	body := `{"id":"cred-rename-1","name":"My Laptop"}`
	req := authedAPIKeyReq(http.MethodPost, "/auth/credentials/rename", bytes.NewBufferString(body), priv, user.ID)
	rec := httptest.NewRecorder()
	h.HandleRenameCredential(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	// Verify the name was updated.
	creds, err := h.store.GetCredentialsByUserID(user.ID)
	if err != nil {
		t.Fatalf("get credentials: %v", err)
	}
	if len(creds) != 1 {
		t.Fatalf("expected 1 credential, got %d", len(creds))
	}
	if creds[0].Name != "My Laptop" {
		t.Errorf("name: got %q, want %q", creds[0].Name, "My Laptop")
	}
}

func TestRenameCredentialRejectsNonOwner(t *testing.T) {
	h, priv := testHandlerEnv(t)

	owner, err := h.store.CreateUser("rename-owner", "renameowner@example.com")
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	other, err := h.store.CreateUser("rename-other", "renameother@example.com")
	if err != nil {
		t.Fatalf("create other: %v", err)
	}

	cred := &Credential{
		ID:              "cred-rename-owner-1",
		UserID:          owner.ID,
		PublicKey:       make([]byte, 64),
		AttestationType: "none",
		Transport:       `["internal"]`,
		SignCount:       0,
		AAGUID:          make([]byte, 16),
		Flags:           "{}",
		CreatedAt:       time.Now().UTC(),
	}
	if err := h.store.CreateCredential(cred); err != nil {
		t.Fatalf("create credential: %v", err)
	}

	body := `{"id":"cred-rename-owner-1","name":"Hacked"}`
	req := authedAPIKeyReq(http.MethodPost, "/auth/credentials/rename", bytes.NewBufferString(body), priv, other.ID)
	rec := httptest.NewRecorder()
	h.HandleRenameCredential(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d (non-owner)", rec.Code, http.StatusNotFound)
	}
}

func TestRenameCredentialRejectsEmptyName(t *testing.T) {
	h, priv := testHandlerEnv(t)

	user, err := h.store.CreateUser("rename-empty", "renameempty@example.com")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	body := `{"id":"cred-1","name":""}`
	req := authedAPIKeyReq(http.MethodPost, "/auth/credentials/rename", bytes.NewBufferString(body), priv, user.ID)
	rec := httptest.NewRecorder()
	h.HandleRenameCredential(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

// --- Session Listing Handler Tests ---

func TestListSessionsRejectsUnauthenticated(t *testing.T) {
	h, _ := testHandlerEnv(t)

	req := httptest.NewRequest(http.MethodGet, "/auth/sessions", nil)
	rec := httptest.NewRecorder()
	h.HandleListSessions(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestListSessionsReturnsSessionsWithoutTokenHash(t *testing.T) {
	h, priv := testHandlerEnv(t)

	user, err := h.store.CreateUser("sess-list-user", "sesslist@example.com")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	// Create a session.
	rec := httptest.NewRecorder()
	_, err = h.issueSession(rec, httptest.NewRequest(http.MethodGet, "/", nil), user)
	if err != nil {
		t.Fatalf("issue session: %v", err)
	}

	req := authedAPIKeyReq(http.MethodGet, "/auth/sessions", nil, priv, user.ID)
	listRec := httptest.NewRecorder()
	h.HandleListSessions(listRec, req)

	if listRec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body: %s", listRec.Code, http.StatusOK, listRec.Body.String())
	}

	var resp listSessionsResponse
	if err := json.NewDecoder(listRec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(resp.Sessions))
	}

	// Verify no token hash is in the response.
	bodyStr := listRec.Body.String()
	if strings.Contains(bodyStr, "token_hash") {
		t.Error("response should not contain token_hash")
	}

	// Verify the session has the expected fields.
	s := resp.Sessions[0]
	if s.ID == "" {
		t.Error("session ID should be non-empty")
	}
	if s.CreatedAt == "" {
		t.Error("created_at should be non-empty")
	}
}

// --- Session Revocation Handler Tests ---

func TestRevokeSessionRejectsUnauthenticated(t *testing.T) {
	h, _ := testHandlerEnv(t)

	req := httptest.NewRequest(http.MethodDelete, "/auth/sessions/sess-1", nil)
	rec := httptest.NewRecorder()
	h.HandleRevokeSession(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestRevokeSessionSucceedsForOtherSession(t *testing.T) {
	h, priv := testHandlerEnv(t)

	user, err := h.store.CreateUser("sess-revoke-user", "sessrevoke@example.com")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	// Create two sessions (simulating two devices).
	rec1 := httptest.NewRecorder()
	_, err = h.issueSession(rec1, httptest.NewRequest(http.MethodGet, "/", nil), user)
	if err != nil {
		t.Fatalf("issue session 1: %v", err)
	}
	rec2 := httptest.NewRecorder()
	_, err = h.issueSession(rec2, httptest.NewRequest(http.MethodGet, "/", nil), user)
	if err != nil {
		t.Fatalf("issue session 2: %v", err)
	}

	// List sessions to get the IDs.
	listReq := authedAPIKeyReq(http.MethodGet, "/auth/sessions", nil, priv, user.ID)
	listRec := httptest.NewRecorder()
	h.HandleListSessions(listRec, listReq)
	var listResp listSessionsResponse
	if err := json.NewDecoder(listRec.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(listResp.Sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(listResp.Sessions))
	}

	// Revoke the second session (not the current one).
	targetID := listResp.Sessions[1].ID
	delReq := authedAPIKeyReq(http.MethodDelete, "/auth/sessions/"+targetID, nil, priv, user.ID)
	delRec := httptest.NewRecorder()
	h.HandleRevokeSession(delRec, delReq)

	if delRec.Code != http.StatusNoContent {
		t.Errorf("status: got %d, want %d; body: %s", delRec.Code, http.StatusNoContent, delRec.Body.String())
	}

	// Verify only one session remains.
	listReq2 := authedAPIKeyReq(http.MethodGet, "/auth/sessions", nil, priv, user.ID)
	listRec2 := httptest.NewRecorder()
	h.HandleListSessions(listRec2, listReq2)
	var listResp2 listSessionsResponse
	if err := json.NewDecoder(listRec2.Body).Decode(&listResp2); err != nil {
		t.Fatalf("decode list 2: %v", err)
	}
	if len(listResp2.Sessions) != 1 {
		t.Errorf("expected 1 session after revoke, got %d", len(listResp2.Sessions))
	}
}

func TestRevokeSessionRejectsCurrentSession(t *testing.T) {
	h, priv := testHandlerEnv(t)

	user, err := h.store.CreateUser("sess-current-user", "sesscurrent@example.com")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	// Create a session and get the refresh token cookie.
	rec := httptest.NewRecorder()
	_, err = h.issueSession(rec, httptest.NewRequest(http.MethodGet, "/", nil), user)
	if err != nil {
		t.Fatalf("issue session: %v", err)
	}

	// Extract the refresh cookie from the response.
	var refreshCookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == RefreshTokenCookieName {
			refreshCookie = c
			break
		}
	}
	if refreshCookie == nil {
		t.Fatal("no refresh cookie set")
	}

	// List sessions to find the current session ID.
	listReq := authedAPIKeyReq(http.MethodGet, "/auth/sessions", nil, priv, user.ID)
	listRec := httptest.NewRecorder()
	h.HandleListSessions(listRec, listReq)
	var listResp listSessionsResponse
	if err := json.NewDecoder(listRec.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(listResp.Sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(listResp.Sessions))
	}
	currentID := listResp.Sessions[0].ID

	// Try to revoke the current session (with the refresh cookie).
	delReq := authedAPIKeyReq(http.MethodDelete, "/auth/sessions/"+currentID, nil, priv, user.ID)
	delReq.AddCookie(refreshCookie)
	delRec := httptest.NewRecorder()
	h.HandleRevokeSession(delRec, delReq)

	if delRec.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d (cannot revoke current session); body: %s", delRec.Code, http.StatusBadRequest, delRec.Body.String())
	}
}

func TestRevokeSessionRejectsNonOwner(t *testing.T) {
	h, priv := testHandlerEnv(t)

	owner, err := h.store.CreateUser("sess-owner", "sessowner@example.com")
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	other, err := h.store.CreateUser("sess-other", "sessother@example.com")
	if err != nil {
		t.Fatalf("create other: %v", err)
	}

	// Owner creates a session.
	rec := httptest.NewRecorder()
	_, err = h.issueSession(rec, httptest.NewRequest(http.MethodGet, "/", nil), owner)
	if err != nil {
		t.Fatalf("issue session: %v", err)
	}

	// List owner's sessions.
	listReq := authedAPIKeyReq(http.MethodGet, "/auth/sessions", nil, priv, owner.ID)
	listRec := httptest.NewRecorder()
	h.HandleListSessions(listRec, listReq)
	var listResp listSessionsResponse
	if err := json.NewDecoder(listRec.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(listResp.Sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(listResp.Sessions))
	}
	targetID := listResp.Sessions[0].ID

	// Other user tries to revoke owner's session.
	delReq := authedAPIKeyReq(http.MethodDelete, "/auth/sessions/"+targetID, nil, priv, other.ID)
	delRec := httptest.NewRecorder()
	h.HandleRevokeSession(delRec, delReq)

	if delRec.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d (non-owner)", delRec.Code, http.StatusNotFound)
	}
}

// --- No Secrets in Logs Tests ---

func TestRecoveryRequestDoesNotLeakTokenInLogs(t *testing.T) {
	h, _ := testHandlerEnv(t)

	h.store.CreateUser("log-user", "loguser@example.com")

	var token string
	logs := captureLogs(t, func() {
		body := `{"email":"loguser@example.com"}`
		req := httptest.NewRequest(http.MethodPost, "/auth/recovery/request", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		h.HandleRecoveryRequest(rec, req)

		// Extract the token from the response.
		var resp recoveryResponse
		_ = json.NewDecoder(rec.Body).Decode(&resp)
		token = resp.Token
	})

	if token != "" && strings.Contains(logs, token) {
		t.Errorf("logs should not contain the recovery token:\n%s", logs)
	}
}

func TestRecoveryVerifyDoesNotLeakTokenInLogs(t *testing.T) {
	h, _ := testHandlerEnv(t)

	user, err := h.store.CreateUser("log-verify-user", "logverify@example.com")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	token, err := h.store.CreateRecoveryToken(context.Background(), user.ID, "logverify@example.com", hashEmail("logverify@example.com"), hashIP("127.0.0.1"))
	if err != nil {
		t.Fatalf("create token: %v", err)
	}

	logs := captureLogs(t, func() {
		body := fmt.Sprintf(`{"token":%q}`, token)
		req := httptest.NewRequest(http.MethodPost, "/auth/recovery/verify", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		h.HandleRecoveryVerify(rec, req)
	})

	if strings.Contains(logs, token) {
		t.Errorf("logs should not contain the recovery token:\n%s", logs)
	}
}

// --- Credential Store Methods Tests ---

func TestDeleteCredentialStoreRejectsNonOwner(t *testing.T) {
	store := TestStore(t)
	ctx := context.Background()

	owner, err := store.CreateUser("del-store-owner", "delstoreowner@example.com")
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}

	cred := &Credential{
		ID:              "cred-del-store-1",
		UserID:          owner.ID,
		PublicKey:       make([]byte, 64),
		AttestationType: "none",
		Transport:       `["internal"]`,
		SignCount:       0,
		AAGUID:          make([]byte, 16),
		Flags:           "{}",
		CreatedAt:       time.Now().UTC(),
	}
	if err := store.CreateCredential(cred); err != nil {
		t.Fatalf("create credential: %v", err)
	}

	// Non-owner delete should fail.
	err = store.DeleteCredential(ctx, "other-user", "cred-del-store-1")
	if err == nil {
		t.Fatal("expected error for non-owner delete")
	}
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("expected sql.ErrNoRows, got %v", err)
	}

	// Owner delete should succeed.
	err = store.DeleteCredential(ctx, owner.ID, "cred-del-store-1")
	if err != nil {
		t.Fatalf("owner delete: %v", err)
	}
}

func TestRenameCredentialStoreRejectsNonOwner(t *testing.T) {
	store := TestStore(t)
	ctx := context.Background()

	owner, err := store.CreateUser("rename-store-owner", "renamestoreowner@example.com")
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}

	cred := &Credential{
		ID:              "cred-rename-store-1",
		UserID:          owner.ID,
		PublicKey:       make([]byte, 64),
		AttestationType: "none",
		Transport:       `["internal"]`,
		SignCount:       0,
		AAGUID:          make([]byte, 16),
		Flags:           "{}",
		CreatedAt:       time.Now().UTC(),
	}
	if err := store.CreateCredential(cred); err != nil {
		t.Fatalf("create credential: %v", err)
	}

	// Non-owner rename should fail.
	err = store.RenameCredential(ctx, "other-user", "cred-rename-store-1", "Hacked")
	if err == nil {
		t.Fatal("expected error for non-owner rename")
	}
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("expected sql.ErrNoRows, got %v", err)
	}

	// Owner rename should succeed.
	err = store.RenameCredential(ctx, owner.ID, "cred-rename-store-1", "My Device")
	if err != nil {
		t.Fatalf("owner rename: %v", err)
	}
}

func TestTouchCredentialLastUsed(t *testing.T) {
	store := TestStore(t)

	user, err := store.CreateUser("touch-cred-user", "touchcred@example.com")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	cred := &Credential{
		ID:              "cred-touch-1",
		UserID:          user.ID,
		PublicKey:       make([]byte, 64),
		AttestationType: "none",
		Transport:       `["internal"]`,
		SignCount:       0,
		AAGUID:          make([]byte, 16),
		Flags:           "{}",
		CreatedAt:       time.Now().UTC(),
	}
	if err := store.CreateCredential(cred); err != nil {
		t.Fatalf("create credential: %v", err)
	}

	// Before touch, last_used_at should be nil.
	creds, err := store.GetCredentialsByUserID(user.ID)
	if err != nil {
		t.Fatalf("get credentials: %v", err)
	}
	if len(creds) != 1 || creds[0].LastUsedAt != nil {
		t.Fatal("last_used_at should be nil before touch")
	}

	// Touch.
	before := time.Now().UTC()
	if err := store.TouchCredentialLastUsed("cred-touch-1"); err != nil {
		t.Fatalf("touch: %v", err)
	}

	// After touch, last_used_at should be set.
	creds, err = store.GetCredentialsByUserID(user.ID)
	if err != nil {
		t.Fatalf("get credentials after touch: %v", err)
	}
	if len(creds) != 1 {
		t.Fatalf("expected 1 credential, got %d", len(creds))
	}
	if creds[0].LastUsedAt == nil {
		t.Fatal("last_used_at should be set after touch")
	}
	if creds[0].LastUsedAt.Before(before) {
		t.Errorf("last_used_at %v should be after %v", creds[0].LastUsedAt, before)
	}
}

// --- Session Store Methods Tests ---

func TestListRefreshSessionsByUserID(t *testing.T) {
	store := TestStore(t)

	user, err := store.CreateUser("list-sess-user", "listsess@example.com")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	// Create two sessions.
	for i := 0; i < 2; i++ {
		rs := &RefreshSession{
			ID:        fmt.Sprintf("rs-%d", i),
			UserID:    user.ID,
			TokenHash: fmt.Sprintf("hash-%d", i),
			CreatedAt: time.Now().UTC(),
			ExpiresAt: time.Now().UTC().Add(1 * time.Hour),
		}
		if err := store.CreateRefreshSession(rs); err != nil {
			t.Fatalf("create session %d: %v", i, err)
		}
	}

	sessions, err := store.ListRefreshSessionsByUserID(user.ID)
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessions))
	}
}

func TestGetRefreshSessionByID(t *testing.T) {
	store := TestStore(t)

	user, err := store.CreateUser("get-sess-user", "getsess@example.com")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	rs := &RefreshSession{
		ID:        "rs-get-1",
		UserID:    user.ID,
		TokenHash: "hash-get-1",
		CreatedAt: time.Now().UTC(),
		ExpiresAt: time.Now().UTC().Add(1 * time.Hour),
	}
	if err := store.CreateRefreshSession(rs); err != nil {
		t.Fatalf("create session: %v", err)
	}

	got, err := store.GetRefreshSessionByID("rs-get-1")
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if got.UserID != user.ID {
		t.Errorf("user_id: got %q, want %q", got.UserID, user.ID)
	}

	// Unknown ID should return ErrNoRows.
	_, err = store.GetRefreshSessionByID("nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown session ID")
	}
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("expected sql.ErrNoRows, got %v", err)
	}
}

// --- Clean Expired Recovery Tokens Test ---

func TestCleanExpiredRecoveryTokens(t *testing.T) {
	store := TestStore(t)
	ctx := context.Background()

	user, err := store.CreateUser("clean-user", "clean@example.com")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	// Create a valid (non-expired) token.
	_, err = store.CreateRecoveryToken(ctx, user.ID, "clean@example.com", hashEmail("clean@example.com"), hashIP("127.0.0.1"))
	if err != nil {
		t.Fatalf("create valid token: %v", err)
	}

	// Create an expired token.
	expiredToken, err := store.CreateRecoveryToken(ctx, user.ID, "clean@example.com", hashEmail("clean@example.com"), hashIP("127.0.0.1"))
	if err != nil {
		t.Fatalf("create expired token: %v", err)
	}
	h := sha256SumHex(expiredToken)
	_, err = store.DB().Exec(
		"UPDATE recovery_tokens SET expires_at = ? WHERE token_hash = ?",
		time.Now().UTC().Add(-1*time.Minute), h,
	)
	if err != nil {
		t.Fatalf("expire token: %v", err)
	}

	// Clean expired tokens.
	n, err := store.CleanExpiredRecoveryTokens()
	if err != nil {
		t.Fatalf("clean: %v", err)
	}
	if n != 1 {
		t.Errorf("expected 1 expired token cleaned, got %d", n)
	}

	// Verify only the valid token remains.
	var count int
	err = store.DB().QueryRow("SELECT COUNT(*) FROM recovery_tokens WHERE user_id = ?", user.ID).Scan(&count)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 remaining token, got %d", count)
	}
}
