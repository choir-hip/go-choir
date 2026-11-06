package proxy

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/yusefmosiah/go-choir/internal/auth"
	"github.com/yusefmosiah/go-choir/internal/owneract"
	"github.com/yusefmosiah/go-choir/internal/platform"
)

// The operator account's yank at the proxy
// (docs/design/platform-operator-account-2026-11-06.md). Ways it could fail,
// written first:
//   - an API key, even one with owner:acts, reaches corpusd as the operator;
//   - the receipt names another account, a computer instead of the platform
//     scope, or does not bind the exact request corpusd receives;
//   - the proxy forwards without a receipt when it has no signing key;
//   - a malformed body reaches corpusd, or the proxy keeps its own roster
//     and answers for corpusd.

type ownerActsAPIKeys struct{}

func (ownerActsAPIKeys) GetAPIKeyByHash(context.Context, string) (*auth.APIKey, error) {
	return &auth.APIKey{ID: "key-1", UserID: "user-real", Scopes: []string{"admin", auth.OwnerActsScope}}, nil
}
func (ownerActsAPIKeys) TouchAPIKeyLastUsed(context.Context, string) error { return nil }
func (ownerActsAPIKeys) GetUserByID(id string) (*auth.User, error) {
	if id != "user-real" {
		return nil, sql.ErrNoRows
	}
	return &auth.User{ID: id, Email: "owner@example.com"}, nil
}

func TestPlatformOperatorYankIsASessionActWithAPlatformReceipt(t *testing.T) {
	var mu sync.Mutex
	var calls []string
	var gotReceipt string
	var gotBody platform.PlatformYankRequest
	corpusd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		mu.Lock()
		defer mu.Unlock()
		calls = append(calls, r.Method+" "+r.URL.Path+" internal="+r.Header.Get("X-Internal-Caller"))
		gotReceipt = r.Header.Get(owneract.Header)
		_ = json.Unmarshal(raw, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"owner act refused: account user-1 is not a platform operator"}`))
	}))
	defer corpusd.Close()
	autoputer := httptest.NewServer(http.NewServeMux())
	defer autoputer.Close()
	h, priv := opinionTestHandler(t, autoputer.URL, corpusd.URL)
	h.SetAPIKeyValidator(ownerActsAPIKeys{})
	_, signingKey, _ := ed25519.GenerateKey(rand.Reader)
	signer, err := owneract.NewSigner(signingKey)
	if err != nil {
		t.Fatal(err)
	}
	verifier, _ := owneract.NewVerifier(signer.PublicKey())

	release := strings.Repeat("a", 64)
	target := strings.Repeat("b", 64)
	post := func(body string, mutate func(*http.Request)) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "https://choir.news"+platformOperatorYankPath, strings.NewReader(body))
		if mutate != nil {
			mutate(req)
		}
		w := httptest.NewRecorder()
		h.HandleAPI(w, req)
		return w
	}
	session := func(req *http.Request) {
		req.AddCookie(&http.Cookie{Name: "choir_access", Value: issueTestAccessJWT(priv, "user-1")})
	}
	apiKey := func(req *http.Request) { req.Header.Set("Authorization", "Bearer "+auth.APIKeyPrefix+"test-secret") }
	good := `{"release_id":"` + strings.ToUpper(release) + `","reason":"  CVE-3  ","forced_release":"` + target + `"}`
	callCount := func() int {
		mu.Lock()
		defer mu.Unlock()
		return len(calls)
	}

	if w := post(good, nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous = %d", w.Code)
	}
	if w := post(good, apiKey); w.Code != http.StatusForbidden {
		t.Fatalf("owner:acts API key = %d %s", w.Code, w.Body)
	}
	for _, am := range []string{"email_code", "email_link", "something_new"} {
		emailSession := func(req *http.Request) {
			token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, jwt.MapClaims{
				"sub": "user-1", "iat": time.Now().Unix(), "exp": time.Now().Add(5 * time.Minute).Unix(),
				"scope": "access", "am": am,
			})
			signed, err := token.SignedString(priv)
			if err != nil {
				t.Fatal(err)
			}
			req.AddCookie(&http.Cookie{Name: "choir_access", Value: signed})
		}
		if w := post(good, emailSession); w.Code != http.StatusForbidden || callCount() != 0 {
			t.Fatalf("%s session = %d %s (%d calls)", am, w.Code, w.Body, callCount())
		}
	}
	if w := post(good, session); w.Code != http.StatusServiceUnavailable {
		t.Fatalf("no signer = %d %s", w.Code, w.Body)
	}
	h.ownerActSigner = signer
	for _, body := range []string{`{"release_id":"x","reason":"r"}`, `{"release_id":"` + release + `"}`, `{"release_id":"` + release + `","reason":"r","operator":"me"}`} {
		if w := post(body, session); w.Code != http.StatusBadRequest {
			t.Fatalf("body %s = %d", body, w.Code)
		}
	}
	if n := callCount(); n != 0 {
		t.Fatalf("refused requests reached corpusd: %d", n)
	}
	getReq := httptest.NewRequest(http.MethodGet, "https://choir.news"+platformOperatorYankPath, nil)
	session(getReq)
	getW := httptest.NewRecorder()
	h.HandleAPI(getW, getReq)
	if getW.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET = %d", getW.Code)
	}

	// The session's act reaches corpusd with a receipt; corpusd's answer
	// (here: not an operator) is relayed, not decided by the proxy.
	w := post(good, session)
	if w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), "not a platform operator") {
		t.Fatalf("relayed answer = %d %s", w.Code, w.Body)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(calls) != 1 || calls[0] != "POST /internal/platform/operator/packages/yank internal=true" {
		t.Fatalf("corpusd calls = %v", calls)
	}
	if gotBody.ReleaseID != release || gotBody.ForcedRelease != target || strings.TrimSpace(gotBody.Reason) != "CVE-3" {
		t.Fatalf("corpusd body = %+v", gotBody)
	}
	decoded, err := owneract.Decode(gotReceipt)
	if err != nil {
		t.Fatal(err)
	}
	verified, err := verifier.Verify(decoded, owneract.PlatformComputerID, owneract.ActPlatformYank, time.Now())
	if err != nil {
		t.Fatalf("receipt: %v", err)
	}
	if verified.OwnerUserID != "user-1" || verified.AuthMethod != owneract.AuthMethodSession || !strings.HasPrefix(verified.CredentialRef, "session-jwt-sha256:") {
		t.Fatalf("receipt principal = %+v", verified)
	}
	for key, want := range platform.PlatformYankSubject(gotBody) {
		if verified.SubjectString(key) != want {
			t.Fatalf("receipt subject %s = %q, want %q", key, verified.SubjectString(key), want)
		}
	}
}
