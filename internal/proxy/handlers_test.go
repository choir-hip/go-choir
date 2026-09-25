package proxy

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"github.com/yusefmosiah/go-choir/internal/auth"
	"github.com/yusefmosiah/go-choir/internal/computerversion"
	"github.com/yusefmosiah/go-choir/internal/routeledger"
	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

// testProxyEnv sets up a proxy Handler with a real backend autoputer and
// Ed25519 key material for JWT validation. The autoputer backend includes
// HTTP bootstrap and WebSocket echo endpoints matching the real autoputer
// surface used in production.
func testProxyEnv(t *testing.T) (*Handler, ed25519.PrivateKey, *httptest.Server) {
	t.Helper()

	// Generate a real Ed25519 key pair for JWT signing/verification.
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate ed25519 key: %v", err)
	}

	// Create a fake autoputer backend that echoes request data and supports WS.
	autoputerMux := http.NewServeMux()
	autoputerMux.HandleFunc("/api/shell/bootstrap", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		user := r.Header.Get("X-Authenticated-User")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"computer_id": "autoputer-test",
			"user":        user,
			"bootstrap":   "placeholder-shell-v1",
			"path":        r.URL.Path,
			"method":      r.Method,
			"query":       r.URL.RawQuery,
			"status_code": 200,
		})
	})
	autoputerMux.HandleFunc("/api/shell/error", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"computer_id": "autoputer-test",
			"status_code": 500,
			"error":       "deliberate autoputer error",
		})
	})
	autoputerMux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "service": "autoputer"})
	})
	// WebSocket echo endpoint matching the real autoputer surface.
	autoputerMux.HandleFunc("/api/ws", func(w http.ResponseWriter, r *http.Request) {
		user := r.Header.Get("X-Authenticated-User")
		upgrader := websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()

		// Send initial connected message.
		connected := map[string]interface{}{
			"computer_id": "autoputer-test",
			"user":        user,
			"type":        "connected",
			"payload":     "websocket channel open",
		}
		if err := conn.WriteJSON(connected); err != nil {
			return
		}

		// Echo loop.
		for {
			mt, msg, err := conn.ReadMessage()
			if err != nil {
				break
			}
			// Parse the incoming JSON to extract payload, then echo back.
			var incoming map[string]interface{}
			if json.Unmarshal(msg, &incoming) == nil {
				echo := map[string]interface{}{
					"computer_id": "autoputer-test",
					"user":        user,
					"type":        "echo",
					"payload":     incoming["payload"],
				}
				if err := conn.WriteJSON(echo); err != nil {
					break
				}
			} else {
				// Non-JSON: echo as raw text message.
				if err := conn.WriteMessage(mt, msg); err != nil {
					break
				}
			}
		}
	})

	autoputerServer := httptest.NewServer(autoputerMux)
	t.Cleanup(func() { autoputerServer.Close() })

	cfg := &Config{AllowDirectAutoputerForTests: true, Port: "0",
		ComputerURL:       autoputerServer.URL,
		AuthPublicKeyPath: "/unused/in/test"}

	handler, err := NewHandler(cfg, pub)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	return handler, priv, autoputerServer
}

// testWSProxyEnv sets up a full proxy server with WS support and returns
// the proxy test server URL for WebSocket dialing, the signing key, and
// a cleanup function.
func testWSProxyEnv(t *testing.T) (*httptest.Server, ed25519.PrivateKey) {
	t.Helper()

	handler, priv, _ := testProxyEnv(t)

	// Build a mux that routes both HTTP and WS through the proxy handler.
	mux := http.NewServeMux()
	mux.HandleFunc("/api/shell/bootstrap", handler.HandleBootstrap)
	mux.HandleFunc("/api/ws", handler.HandleWS)
	mux.HandleFunc("/api/", handler.HandleAPI)

	proxyServer := httptest.NewServer(mux)
	t.Cleanup(func() { proxyServer.Close() })

	return proxyServer, priv
}

// wsDialWithCookie dials the proxy's /api/ws endpoint with a valid access
// JWT cookie. Returns the websocket connection.
func wsDialWithCookie(t *testing.T, proxyURL string, accessToken string) *websocket.Conn {
	t.Helper()

	wsURL := "ws" + strings.TrimPrefix(proxyURL, "http") + "/api/ws"
	header := http.Header{}
	header.Set("Cookie", "choir_access="+accessToken)

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("dial proxy WS: %v", err)
	}
	return conn
}

// issueTestAccessJWT creates a signed Ed25519 JWT for the given user ID
// with a 5-minute TTL and "access" scope.
func issueTestAccessJWT(priv ed25519.PrivateKey, userID string) string {
	return issueTestAccessJWTWithTTL(priv, userID, 5*time.Minute)
}

func issueTestAccessJWTWithEmail(priv ed25519.PrivateKey, userID, email string) string {
	return issueTestAccessJWTWithClaims(priv, userID, email, 5*time.Minute)
}

// issueTestAccessJWTWithTTL creates a signed Ed25519 JWT for the given user
// ID with the specified TTL and "access" scope.
func issueTestAccessJWTWithTTL(priv ed25519.PrivateKey, userID string, ttl time.Duration) string {
	return issueTestAccessJWTWithClaims(priv, userID, "", ttl)
}

func issueTestAccessJWTWithClaims(priv ed25519.PrivateKey, userID, email string, ttl time.Duration) string {
	now := time.Now().UTC()
	claims := jwt.MapClaims{
		"sub":   userID,
		"iat":   now.Unix(),
		"exp":   now.Add(ttl).Unix(),
		"scope": "access",
	}
	if email != "" {
		claims["email"] = email
	}
	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	signed, err := token.SignedString(priv)
	if err != nil {
		panic(fmt.Sprintf("sign test JWT: %v", err))
	}
	return signed
}

func TestBootstrapDeniesMissingAuth(t *testing.T) {
	h, _, _ := testProxyEnv(t)

	req := httptest.NewRequest(http.MethodGet, "/api/shell/bootstrap", nil)
	w := httptest.NewRecorder()
	h.HandleBootstrap(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("missing auth: got status %d, want %d", w.Code, http.StatusUnauthorized)
	}

	var resp errorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if resp.Error == "" {
		t.Error("expected non-empty error message")
	}
}

func TestBootstrapDeniesExpiredAuth(t *testing.T) {
	h, priv, _ := testProxyEnv(t)

	// Issue an access JWT that is already expired.
	expiredToken := issueTestAccessJWTWithTTL(priv, "user-123", -1*time.Minute)

	req := httptest.NewRequest(http.MethodGet, "/api/shell/bootstrap", nil)
	req.AddCookie(&http.Cookie{
		Name:  "choir_access",
		Value: expiredToken,
	})
	w := httptest.NewRecorder()
	h.HandleBootstrap(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expired auth: got status %d, want %d", w.Code, http.StatusUnauthorized)
	}

	var resp errorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if resp.Error == "" {
		t.Error("expected non-empty error message")
	}
}

func TestBootstrapDeniesNonAccessToken(t *testing.T) {
	h, priv, _ := testProxyEnv(t)

	// Issue a JWT with a different scope (e.g., "refresh").
	now := time.Now().UTC()
	claims := jwt.MapClaims{
		"sub":   "user-123",
		"iat":   now.Unix(),
		"exp":   now.Add(5 * time.Minute).Unix(),
		"scope": "refresh",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	refreshToken, err := token.SignedString(priv)
	if err != nil {
		t.Fatalf("sign refresh JWT: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/shell/bootstrap", nil)
	req.AddCookie(&http.Cookie{
		Name:  "choir_access",
		Value: refreshToken,
	})
	w := httptest.NewRecorder()
	h.HandleBootstrap(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("non-access token: got status %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestBootstrapDeniesWrongSigningKey(t *testing.T) {
	_, priv, _ := testProxyEnv(t)

	// Generate a different key pair.
	_, wrongPriv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate wrong key: %v", err)
	}

	// Create a handler with the original public key.
	origPub := priv.Public().(ed25519.PublicKey)

	autoputerServer := httptest.NewServer(http.NewServeMux())
	defer autoputerServer.Close()

	cfg := &Config{AllowDirectAutoputerForTests: true, Port: "0", ComputerURL: autoputerServer.URL, AuthPublicKeyPath: "/unused"}
	handler, err := NewHandler(cfg, origPub)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	// Sign a JWT with the wrong key.
	wrongToken := issueTestAccessJWT(wrongPriv, "user-123")

	req := httptest.NewRequest(http.MethodGet, "/api/shell/bootstrap", nil)
	req.AddCookie(&http.Cookie{
		Name:  "choir_access",
		Value: wrongToken,
	})
	w := httptest.NewRecorder()
	handler.HandleBootstrap(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("wrong signing key: got status %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestBootstrapIgnoresClientSuppliedUserContext(t *testing.T) {
	h, priv, _ := testProxyEnv(t)

	accessToken := issueTestAccessJWT(priv, "user-real-identity")

	req := httptest.NewRequest(http.MethodGet, "/api/shell/bootstrap", nil)
	req.AddCookie(&http.Cookie{
		Name:  "choir_access",
		Value: accessToken,
	})
	// Client tries to spoof identity.
	req.Header.Set("X-Authenticated-User", "spoofed-attacker-identity")
	w := httptest.NewRecorder()
	h.HandleBootstrap(w, req)

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode bootstrap response: %v", err)
	}

	// The proxy should inject the JWT-verified user, not the client-supplied value.
	if resp["user"] != "user-real-identity" {
		t.Errorf("spoofed identity: got %v, want %q (JWT identity)", resp["user"], "user-real-identity")
	}
}

func TestBootstrapProxyDoesNotLeakToSignedOutUsers(t *testing.T) {
	h, _, autoputer := testProxyEnv(t)
	_ = autoputer

	// Request without auth should not reach the autoputer (no autoputer data in response).
	req := httptest.NewRequest(http.MethodGet, "/api/shell/bootstrap", nil)
	w := httptest.NewRecorder()
	h.HandleBootstrap(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("signed-out request: got status %d, want %d", w.Code, http.StatusUnauthorized)
	}

	// The response should be an auth error, not autoputer data.
	var resp errorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error response: %v", err)
	}

	if resp.Error == "" {
		t.Error("expected non-empty auth error")
	}

	// Verify the error is not a autoputer payload (no computer_id field).
	var raw map[string]interface{}
	_ = json.NewDecoder(w.Body).Decode(&raw) // decode again from already-consumed body
	// The error response should only have "error", not autoputer fields.
	_, hasComputerID := raw["computer_id"]
	if hasComputerID {
		t.Error("signed-out response should not contain computer_id")
	}
}

func TestBootstrapRejectsNonGet(t *testing.T) {
	h, priv, _ := testProxyEnv(t)

	accessToken := issueTestAccessJWT(priv, "user-123")

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch} {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/shell/bootstrap", nil)
			req.AddCookie(&http.Cookie{
				Name:  "choir_access",
				Value: accessToken,
			})
			w := httptest.NewRecorder()
			h.HandleBootstrap(w, req)

			if w.Code != http.StatusMethodNotAllowed {
				t.Errorf("method %s: got status %d, want %d", method, w.Code, http.StatusMethodNotAllowed)
			}
		})
	}
}

// --- HandleAPI routing test ---

func TestHandleAPIReturnsNotFoundForUnknownRoutes(t *testing.T) {
	h, priv, _ := testProxyEnv(t)

	accessToken := issueTestAccessJWT(priv, "user-123")

	req := httptest.NewRequest(http.MethodGet, "/api/unknown/route", nil)
	req.AddCookie(&http.Cookie{
		Name:  "choir_access",
		Value: accessToken,
	})
	w := httptest.NewRecorder()
	h.HandleAPI(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("unknown API route: got status %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestHandlePulseSummaryIsPublicAndAggregateOnly(t *testing.T) {
	h, _, _ := testProxyEnv(t)
	vmctlSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/vmctl/pulse" {
			t.Fatalf("vmctl path = %s, want /internal/vmctl/pulse", r.URL.Path)
		}
		if r.Header.Get("X-Internal-Caller") != "true" {
			t.Fatalf("missing internal caller header")
		}
		_ = json.NewEncoder(w).Encode(vmctl.PulseSummary{
			Status:      "ok",
			GeneratedAt: "2026-06-14T12:00:00Z",
			Privacy: vmctl.PulsePrivacyStatement{
				Surface:              "public-readonly",
				DataMode:             "aggregate-only",
				NoPrivateSuperset:    true,
				NoRowLevelAnalytics:  true,
				NoUserIdentityOutput: true,
			},
			Accounts: vmctl.PulseAccountSummary{
				Total: 3,
				ByClass: map[string]int{
					vmctl.PulseAccountReal:             1,
					vmctl.PulseAccountCodexAgenticTest: 1,
					vmctl.PulseAccountProtectedTest:    1,
				},
				AuthDataAvailable: true,
			},
		})
	}))
	t.Cleanup(func() { vmctlSrv.Close() })
	h.vmctlClient = vmctl.NewClient(vmctlSrv.URL)

	req := httptest.NewRequest(http.MethodGet, "/api/pulse/summary", nil)
	w := httptest.NewRecorder()
	h.HandleAPI(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("pulse summary status = %d, want 200 body=%s", w.Code, w.Body.String())
	}
	var summary vmctl.PulseSummary
	if err := json.NewDecoder(w.Body).Decode(&summary); err != nil {
		t.Fatalf("decode pulse summary: %v", err)
	}
	if summary.Accounts.ByClass[vmctl.PulseAccountReal] != 1 {
		t.Fatalf("real users = %d, want 1", summary.Accounts.ByClass[vmctl.PulseAccountReal])
	}
	body := w.Body.String()
	for _, forbidden := range []string{"@", "user_id", "ip_address"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("pulse public response leaked forbidden marker %q in %s", forbidden, body)
		}
	}
}

// --- Edge cases ---

func TestBootstrapWithEmptyCookieValue(t *testing.T) {
	h, _, _ := testProxyEnv(t)

	req := httptest.NewRequest(http.MethodGet, "/api/shell/bootstrap", nil)
	req.AddCookie(&http.Cookie{
		Name:  "choir_access",
		Value: "",
	})
	w := httptest.NewRecorder()
	h.HandleBootstrap(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("empty cookie value: got status %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

// --- VAL-PROXY-004: Missing or invalid auth cannot open GET /api/ws ---

func TestWSDeniesMissingAuth(t *testing.T) {
	h, _, _ := testProxyEnv(t)

	// Use httptest.NewServer so we can attempt a real WS dial.
	mux := http.NewServeMux()
	mux.HandleFunc("/api/ws", h.HandleWS)
	proxyServer := httptest.NewServer(mux)
	defer proxyServer.Close()

	wsURL := "ws" + strings.TrimPrefix(proxyServer.URL, "http") + "/api/ws"
	_, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err == nil {
		t.Fatal("expected WS dial to fail without auth, but it succeeded")
	}
	// The response should be a 401, not a successful upgrade.
	if resp != nil && resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 status, got %d", resp.StatusCode)
	}
}

func TestWSDeniesExpiredAuth(t *testing.T) {
	h, priv, _ := testProxyEnv(t)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/ws", h.HandleWS)
	proxyServer := httptest.NewServer(mux)
	defer proxyServer.Close()

	expiredToken := issueTestAccessJWTWithTTL(priv, "user-expired", -1*time.Minute)
	wsURL := "ws" + strings.TrimPrefix(proxyServer.URL, "http") + "/api/ws"
	header := http.Header{}
	header.Set("Cookie", "choir_access="+expiredToken)

	_, resp, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err == nil {
		t.Fatal("expected WS dial to fail with expired auth, but it succeeded")
	}
	if resp != nil && resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 status, got %d", resp.StatusCode)
	}
}

func TestWSDeniesNonAccessToken(t *testing.T) {
	h, priv, _ := testProxyEnv(t)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/ws", h.HandleWS)
	proxyServer := httptest.NewServer(mux)
	defer proxyServer.Close()

	// Issue a JWT with a different scope (e.g., "refresh").
	now := time.Now().UTC()
	claims := jwt.MapClaims{
		"sub":   "user-123",
		"iat":   now.Unix(),
		"exp":   now.Add(5 * time.Minute).Unix(),
		"scope": "refresh",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	refreshToken, err := token.SignedString(priv)
	if err != nil {
		t.Fatalf("sign refresh JWT: %v", err)
	}

	wsURL := "ws" + strings.TrimPrefix(proxyServer.URL, "http") + "/api/ws"
	header := http.Header{}
	header.Set("Cookie", "choir_access="+refreshToken)

	_, resp, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err == nil {
		t.Fatal("expected WS dial to fail with non-access token, but it succeeded")
	}
	if resp != nil && resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 status, got %d", resp.StatusCode)
	}
}

// --- VAL-PROXY-003: Authenticated WS upgrade succeeds and relays frames bidirectionally ---

func TestWSAuthenticatedUpgradesAndRelays(t *testing.T) {
	proxyServer, priv := testWSProxyEnv(t)

	accessToken := issueTestAccessJWT(priv, "user-ws-relay")
	conn := wsDialWithCookie(t, proxyServer.URL, accessToken)
	defer func() { _ = conn.Close() }()

	// Read the initial connected message from the autoputer (relayed through proxy).
	var connected map[string]interface{}
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	if err := conn.ReadJSON(&connected); err != nil {
		t.Fatalf("read connected message: %v", err)
	}

	if connected["type"] != "connected" {
		t.Errorf("connected type: got %v, want %q", connected["type"], "connected")
	}
	if connected["computer_id"] != "autoputer-test" {
		t.Errorf("connected computer_id: got %v, want %q", connected["computer_id"], "autoputer-test")
	}

	// Send a message and verify it is echoed back via the proxy relay.
	msg := map[string]interface{}{
		"type":    "test",
		"payload": "hello-through-proxy",
	}
	if err := conn.WriteJSON(msg); err != nil {
		t.Fatalf("write test message: %v", err)
	}

	var echo map[string]interface{}
	if err := conn.ReadJSON(&echo); err != nil {
		t.Fatalf("read echo message: %v", err)
	}

	if echo["type"] != "echo" {
		t.Errorf("echo type: got %v, want %q", echo["type"], "echo")
	}
	if echo["payload"] != "hello-through-proxy" {
		t.Errorf("echo payload: got %v, want %q", echo["payload"], "hello-through-proxy")
	}
	if echo["computer_id"] != "autoputer-test" {
		t.Errorf("echo computer_id: got %v, want %q", echo["computer_id"], "autoputer-test")
	}
}

func TestWSIgnoresClientSuppliedUserContext(t *testing.T) {
	proxyServer, priv := testWSProxyEnv(t)

	accessToken := issueTestAccessJWT(priv, "user-real-identity")

	// Attempt to spoof the X-Authenticated-User header via the WS handshake.
	wsURL := "ws" + strings.TrimPrefix(proxyServer.URL, "http") + "/api/ws"
	header := http.Header{}
	header.Set("Cookie", "choir_access="+accessToken)
	header.Set("X-Authenticated-User", "spoofed-attacker-identity")

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("dial proxy WS: %v", err)
	}
	defer func() { _ = conn.Close() }()

	// The autoputer should see the JWT-verified user, not the spoofed header.
	var connected map[string]interface{}
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	if err := conn.ReadJSON(&connected); err != nil {
		t.Fatalf("read connected message: %v", err)
	}

	if connected["user"] != "user-real-identity" {
		t.Errorf("spoofed identity: got %v, want %q (JWT identity)", connected["user"], "user-real-identity")
	}
}

// testWSDeniesAuthWithHTTPCheck verifies that a plain HTTP request to the WS
// endpoint without valid auth returns 401 JSON without upgrading.
func TestWSAuthDenialReturnsJSONWithoutUpgrade(t *testing.T) {
	h, _, _ := testProxyEnv(t)

	req := httptest.NewRequest(http.MethodGet, "/api/ws", nil)
	// Add WebSocket upgrade headers to simulate a WS handshake attempt.
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")

	w := httptest.NewRecorder()
	h.HandleWS(w, req)

	// Auth denial should return 401 before any upgrade happens.
	if w.Code != http.StatusUnauthorized {
		t.Errorf("WS auth denial: got status %d, want %d", w.Code, http.StatusUnauthorized)
	}

	// Response should be JSON, not a WS upgrade.
	ct := w.Header().Get("Content-Type")
	if ct == "" {
		t.Error("Content-Type header is missing on WS auth failure")
	}

	// Verify the response body is a JSON error, not a WS frame.
	var resp errorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode WS auth error response: %v", err)
	}
	if resp.Error == "" {
		t.Error("expected non-empty error message for WS auth denial")
	}

	// The Upgrade header should NOT be set (no WS upgrade occurred).
	if upgrade := w.Header().Get("Upgrade"); upgrade == "websocket" {
		t.Error("Upgrade header should not be set on auth denial")
	}
}

// --- VAL-PROXY-005: Spoofed identity headers, same autoputer, distinct user context ---

func TestBootstrapTwoDistinctUsersSameAutoputerDifferentContext(t *testing.T) {
	h, priv, _ := testProxyEnv(t)

	// User A requests bootstrap.
	accessTokenA := issueTestAccessJWT(priv, "user-alice")
	reqA := httptest.NewRequest(http.MethodGet, "/api/shell/bootstrap", nil)
	reqA.AddCookie(&http.Cookie{Name: "choir_access", Value: accessTokenA})
	wA := httptest.NewRecorder()
	h.HandleBootstrap(wA, reqA)

	if wA.Code != http.StatusOK {
		t.Fatalf("user A: got status %d, want %d", wA.Code, http.StatusOK)
	}

	var respA map[string]interface{}
	if err := json.NewDecoder(wA.Body).Decode(&respA); err != nil {
		t.Fatalf("decode user A bootstrap: %v", err)
	}

	// User B requests bootstrap.
	accessTokenB := issueTestAccessJWT(priv, "user-bob")
	reqB := httptest.NewRequest(http.MethodGet, "/api/shell/bootstrap", nil)
	reqB.AddCookie(&http.Cookie{Name: "choir_access", Value: accessTokenB})
	wB := httptest.NewRecorder()
	h.HandleBootstrap(wB, reqB)

	if wB.Code != http.StatusOK {
		t.Fatalf("user B: got status %d, want %d", wB.Code, http.StatusOK)
	}

	var respB map[string]interface{}
	if err := json.NewDecoder(wB.Body).Decode(&respB); err != nil {
		t.Fatalf("decode user B bootstrap: %v", err)
	}

	// Both users must reach the same autoputer instance.
	if respA["computer_id"] != respB["computer_id"] {
		t.Errorf("autoputer identity mismatch: user A saw %v, user B saw %v", respA["computer_id"], respB["computer_id"])
	}

	// Each user must see their own authenticated context.
	if respA["user"] != "user-alice" {
		t.Errorf("user A context: got %v, want %q", respA["user"], "user-alice")
	}
	if respB["user"] != "user-bob" {
		t.Errorf("user B context: got %v, want %q", respB["user"], "user-bob")
	}

	// The contexts must be distinct.
	if respA["user"] == respB["user"] {
		t.Errorf("user A and user B should have different context, both got %v", respA["user"])
	}
}

func TestBootstrapStripsAdditionalSpoofedIdentityHeaders(t *testing.T) {
	// Verify that the proxy strips common identity-spoofing headers
	// beyond just X-Authenticated-User.
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	// Create a autoputer that echoes all received identity headers.
	autoputerMux := http.NewServeMux()
	autoputerMux.HandleFunc("/api/shell/bootstrap", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"computer_id":      "autoputer-test",
			"user":             r.Header.Get("X-Authenticated-User"),
			"x_user_id":        r.Header.Get("X-User-Id"),
			"x_forwarded_user": r.Header.Get("X-Forwarded-User"),
			"x_remote_user":    r.Header.Get("X-Remote-User"),
			"x_auth_user":      r.Header.Get("X-Auth-User"),
			"x_user_name":      r.Header.Get("X-User-Name"),
			"path":             r.URL.Path,
		})
	})
	autoputerServer := httptest.NewServer(autoputerMux)
	defer autoputerServer.Close()

	cfg := &Config{AllowDirectAutoputerForTests: true, Port: "0", ComputerURL: autoputerServer.URL, AuthPublicKeyPath: "/unused"}
	handler, err := NewHandler(cfg, pub)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	accessToken := issueTestAccessJWT(priv, "user-real-identity")

	req := httptest.NewRequest(http.MethodGet, "/api/shell/bootstrap", nil)
	req.AddCookie(&http.Cookie{Name: "choir_access", Value: accessToken})
	// Spoof multiple identity headers.
	req.Header.Set("X-Authenticated-User", "spoofed-auth-user")
	req.Header.Set("X-User-Id", "spoofed-user-id")
	req.Header.Set("X-Forwarded-User", "spoofed-forwarded-user")
	req.Header.Set("X-Remote-User", "spoofed-remote-user")
	req.Header.Set("X-Auth-User", "spoofed-auth-user-header")
	req.Header.Set("X-User-Name", "spoofed-user-name")

	w := httptest.NewRecorder()
	handler.HandleBootstrap(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d", w.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	// The trusted X-Authenticated-User must be the JWT-verified identity.
	if resp["user"] != "user-real-identity" {
		t.Errorf("X-Authenticated-User: got %v, want %q", resp["user"], "user-real-identity")
	}

	// All other identity headers must be stripped — not forwarded to autoputer.
	for _, header := range []string{"x_user_id", "x_forwarded_user", "x_remote_user", "x_auth_user", "x_user_name"} {
		if resp[header] != "" {
			t.Errorf("spoofed header %q leaked to autoputer: got %v", header, resp[header])
		}
	}
}

func TestWSAuthenticatedTwoDistinctUsersSameAutoputerDifferentContext(t *testing.T) {
	proxyServer, priv := testWSProxyEnv(t)

	// User A connects via WS.
	accessTokenA := issueTestAccessJWT(priv, "user-ws-alice")
	connA := wsDialWithCookie(t, proxyServer.URL, accessTokenA)
	defer func() { _ = connA.Close() }()

	var connectedA map[string]interface{}
	_ = connA.SetReadDeadline(time.Now().Add(3 * time.Second))
	if err := connA.ReadJSON(&connectedA); err != nil {
		t.Fatalf("user A: read connected: %v", err)
	}

	// User B connects via WS.
	accessTokenB := issueTestAccessJWT(priv, "user-ws-bob")
	connB := wsDialWithCookie(t, proxyServer.URL, accessTokenB)
	defer func() { _ = connB.Close() }()

	var connectedB map[string]interface{}
	_ = connB.SetReadDeadline(time.Now().Add(3 * time.Second))
	if err := connB.ReadJSON(&connectedB); err != nil {
		t.Fatalf("user B: read connected: %v", err)
	}

	// Both users must reach the same autoputer instance.
	if connectedA["computer_id"] != connectedB["computer_id"] {
		t.Errorf("autoputer identity mismatch: user A saw %v, user B saw %v", connectedA["computer_id"], connectedB["computer_id"])
	}

	// Each user must see their own authenticated context.
	if connectedA["user"] != "user-ws-alice" {
		t.Errorf("user A context: got %v, want %q", connectedA["user"], "user-ws-alice")
	}
	if connectedB["user"] != "user-ws-bob" {
		t.Errorf("user B context: got %v, want %q", connectedB["user"], "user-ws-bob")
	}

	// The contexts must be distinct.
	if connectedA["user"] == connectedB["user"] {
		t.Errorf("user A and user B should have different context, both got %v", connectedA["user"])
	}
}

func TestWSSpoofedIdentityHeadersDoNotReachAutoputer(t *testing.T) {
	// Create a autoputer that echoes all received identity headers over WS.
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	autoputerMux := http.NewServeMux()
	autoputerMux.HandleFunc("/api/ws", func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()

		// Echo all identity headers we received.
		connected := map[string]interface{}{
			"computer_id":      "autoputer-test",
			"user":             r.Header.Get("X-Authenticated-User"),
			"x_user_id":        r.Header.Get("X-User-Id"),
			"x_forwarded_user": r.Header.Get("X-Forwarded-User"),
			"x_remote_user":    r.Header.Get("X-Remote-User"),
			"type":             "connected",
		}
		if err := conn.WriteJSON(connected); err != nil {
			return
		}
	})
	autoputerServer := httptest.NewServer(autoputerMux)
	defer autoputerServer.Close()

	cfg := &Config{AllowDirectAutoputerForTests: true, Port: "0", ComputerURL: autoputerServer.URL, AuthPublicKeyPath: "/unused"}
	handler, err := NewHandler(cfg, pub)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/ws", handler.HandleWS)
	proxyServer := httptest.NewServer(mux)
	defer proxyServer.Close()

	accessToken := issueTestAccessJWT(priv, "user-real-ws")

	// Dial with spoofed identity headers on the handshake.
	wsURL := "ws" + strings.TrimPrefix(proxyServer.URL, "http") + "/api/ws"
	header := http.Header{}
	header.Set("Cookie", "choir_access="+accessToken)
	header.Set("X-Authenticated-User", "spoofed-auth-user")
	header.Set("X-User-Id", "spoofed-user-id")
	header.Set("X-Forwarded-User", "spoofed-forwarded")
	header.Set("X-Remote-User", "spoofed-remote")

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("dial proxy WS: %v", err)
	}
	defer func() { _ = conn.Close() }()

	var connected map[string]interface{}
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	if err := conn.ReadJSON(&connected); err != nil {
		t.Fatalf("read connected: %v", err)
	}

	// Trusted identity must match JWT.
	if connected["user"] != "user-real-ws" {
		t.Errorf("X-Authenticated-User: got %v, want %q", connected["user"], "user-real-ws")
	}

	// All other identity headers must NOT reach the autoputer.
	for _, header := range []string{"x_user_id", "x_forwarded_user", "x_remote_user"} {
		if connected[header] != "" {
			t.Errorf("spoofed header %q leaked to autoputer via WS: got %v", header, connected[header])
		}
	}
}

func TestWSDeniesWrongSigningKey(t *testing.T) {
	h, _, _ := testProxyEnv(t)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/ws", h.HandleWS)
	proxyServer := httptest.NewServer(mux)
	defer proxyServer.Close()

	// Generate a different key pair.
	_, wrongPriv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate wrong key: %v", err)
	}

	// Sign a JWT with the wrong key.
	wrongToken := issueTestAccessJWT(wrongPriv, "user-attacker")

	wsURL := "ws" + strings.TrimPrefix(proxyServer.URL, "http") + "/api/ws"
	header := http.Header{}
	header.Set("Cookie", "choir_access="+wrongToken)

	_, resp, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err == nil {
		t.Fatal("expected WS dial to fail with wrong signing key, but it succeeded")
	}
	if resp != nil && resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 status, got %d", resp.StatusCode)
	}
}

// TestAllAPIRoutesDenySignedOutCallers verifies that every /api/* route
// denies signed-out callers with 401. This covers both explicitly-handled
// protected routes (bootstrap, ws) and the default catch-all for unknown
// /api/* paths. No /api/* route should ever return 200 or expose data
// without valid auth.
//
// VAL-DEPLOY-005: "Protected shell routes deny signed-out callers before
// shell data or live state are exposed"
func TestAllAPIRoutesDenySignedOutCallers(t *testing.T) {
	h, _, _ := testProxyEnv(t)

	// Test a variety of /api/* paths — both known protected routes and
	// unknown future routes. All must deny without auth.
	paths := []struct {
		path       string
		method     string
		wantStatus int
	}{
		{"/api/shell/bootstrap", http.MethodGet, http.StatusUnauthorized},
		{"/api/ws", http.MethodGet, http.StatusUnauthorized},
		{"/api/prompt-bar", http.MethodPost, http.StatusUnauthorized},
		{"/api/prompt-bar/submissions/run-1", http.MethodGet, http.StatusUnauthorized},
		{"/api/trace/trajectories", http.MethodGet, http.StatusUnauthorized},
		{"/api/unknown", http.MethodGet, http.StatusUnauthorized},
		{"/api/shell/some-future-route", http.MethodGet, http.StatusUnauthorized},
	}

	for _, tt := range paths {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()
			h.HandleAPI(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("signed-out %s %s: got status %d, want %d", tt.method, tt.path, w.Code, tt.wantStatus)
			}

			// All denials must be machine-readable JSON.
			ct := w.Header().Get("Content-Type")
			if ct != "application/json" {
				t.Errorf("Content-Type: got %q, want %q", ct, "application/json")
			}

			var resp errorResponse
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Fatalf("decode denial for %s: %v", tt.path, err)
			}
			if resp.Error == "" {
				t.Errorf("denial for %s should have a non-empty error message", tt.path)
			}

			// Must not contain autoputer data.
			body := w.Body.String()
			if strings.Contains(body, "computer_id") {
				t.Errorf("denial for %s should not contain computer_id", tt.path)
			}
		})
	}
}

// TestSignedOutCallersNeverSeeAutoputerData is a comprehensive test verifying
// that no proxy response to a signed-out caller ever contains autoputer-origin
// data (computer_id, bootstrap payloads, user context from the upstream).
//
// VAL-DEPLOY-005: "Protected shell routes deny signed-out callers before
// shell data or live state are exposed"
func TestSignedOutCallersNeverSeeAutoputerData(t *testing.T) {
	h, _, _ := testProxyEnv(t)

	paths := []string{
		"/api/shell/bootstrap",
		"/api/ws",
		"/api/prompt-bar",
		"/api/anything",
	}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			w := httptest.NewRecorder()
			h.HandleAPI(w, req)

			body := w.Body.String()

			// Must not contain any autoputer-origin data.
			for _, field := range []string{"computer_id", "placeholder-shell", "websocket channel"} {
				if strings.Contains(body, field) {
					t.Errorf("signed-out response for %s contains autoputer data field %q", path, field)
				}
			}

			// Response must be a denial (401 or 404), never 200.
			if w.Code == http.StatusOK {
				t.Errorf("signed-out caller got 200 for %s — this is a fail-open bug", path)
			}
		})
	}
}

// ======================================================================
// VAL-DEPLOY-008 / VAL-CROSS-118: Proxy health and restart readiness
// ======================================================================

// TestProxyHealthReportsOkWhenUpstreamIsHealthy verifies that the proxy
// /health endpoint returns "ok" status with "ok" upstream when the
// autoputer backend is reachable.
//
// VAL-DEPLOY-008: "protected-request backend health is observable"
func TestProxyHealthReportsOkWhenUpstreamIsHealthy(t *testing.T) {
	h, _, _ := testProxyEnv(t)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	h.HandleHealth(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("health endpoint: got status %d, want %d", w.Code, http.StatusOK)
	}

	var resp proxyHealthResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode health response: %v", err)
	}

	if resp.Status != "ok" {
		t.Errorf("status: got %q, want %q", resp.Status, "ok")
	}
	if resp.Service != "proxy" {
		t.Errorf("service: got %q, want %q", resp.Service, "proxy")
	}
	if resp.Upstream != "ok" {
		t.Errorf("upstream: got %q, want %q", resp.Upstream, "ok")
	}
	if resp.Build.Service != "proxy" {
		t.Errorf("build.service: got %q, want proxy", resp.Build.Service)
	}
	if resp.Build.Commit == "" {
		t.Error("build.commit should not be empty")
	}
}

// TestProxyHealthReportsDegradedWhenUpstreamIsUnreachable verifies that
// the proxy /health endpoint returns "degraded" status with "unreachable"
// upstream when the autoputer backend is not available. This makes it
// possible for operators and monitoring to distinguish between a healthy
// proxy and a degraded proxy whose backend is down.
//
// VAL-DEPLOY-008: "protected-request backend health is observable and restartable"
func TestProxyHealthReportsDegradedWhenUpstreamIsUnreachable(t *testing.T) {
	pub, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	// Create a autoputer server that we can close to simulate unreachability.
	autoputerMux := http.NewServeMux()
	autoputerMux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "service": "autoputer"})
	})
	autoputerServer := httptest.NewServer(autoputerMux)

	cfg := &Config{AllowDirectAutoputerForTests: true, Port: "0", ComputerURL: autoputerServer.URL, AuthPublicKeyPath: "/unused"}
	handler, err := NewHandler(cfg, pub)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	// Verify health is ok while autoputer is up.
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	handler.HandleHealth(w, req)

	var respOk proxyHealthResponse
	if err := json.NewDecoder(w.Body).Decode(&respOk); err != nil {
		t.Fatalf("decode healthy response: %v", err)
	}
	if respOk.Status != "ok" {
		t.Errorf("status before shutdown: got %q, want %q", respOk.Status, "ok")
	}
	if respOk.Upstream != "ok" {
		t.Errorf("upstream before shutdown: got %q, want %q", respOk.Upstream, "ok")
	}

	// Shut down the autoputer to simulate an upstream failure.
	autoputerServer.Close()

	// Wait briefly for connections to drain.
	time.Sleep(100 * time.Millisecond)

	// Verify health reports degraded when upstream is unreachable.
	req2 := httptest.NewRequest(http.MethodGet, "/health", nil)
	w2 := httptest.NewRecorder()
	handler.HandleHealth(w2, req2)

	var respDegraded proxyHealthResponse
	if err := json.NewDecoder(w2.Body).Decode(&respDegraded); err != nil {
		t.Fatalf("decode degraded response: %v", err)
	}
	if respDegraded.Status != "degraded" {
		t.Errorf("status after shutdown: got %q, want %q", respDegraded.Status, "degraded")
	}
	if respDegraded.Upstream != "unreachable" {
		t.Errorf("upstream after shutdown: got %q, want %q", respDegraded.Upstream, "unreachable")
	}
}

// TestProxyHealthRejectsNonGet verifies that the health endpoint only
// accepts GET requests.
func TestProxyHealthRejectsNonGet(t *testing.T) {
	h, _, _ := testProxyEnv(t)

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch} {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/health", nil)
			w := httptest.NewRecorder()
			h.HandleHealth(w, req)

			if w.Code != http.StatusMethodNotAllowed {
				t.Errorf("health %s: got status %d, want %d", method, w.Code, http.StatusMethodNotAllowed)
			}
		})
	}
}

// TestProviderRoutesDenied verifies that the proxy denies all browser access
// to /provider/* routes (VAL-GATEWAY-002). Browser callers must never use
// /provider/* as a raw inference bypass around the runtime/proxy boundary.
func TestProviderRoutesDenied(t *testing.T) {
	handler, _, _ := testProxyEnv(t)

	providerPaths := []string{
		"/provider/v1/inference",
		"/provider/v1/credentials/issue",
		"/provider/v1/credentials/revoke",
		"/provider/v1/credentials/rotate",
		"/provider/anything",
	}

	for _, path := range providerPaths {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"messages":[]}`))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			handler.HandleProviderDeny(w, req)

			if w.Code != http.StatusForbidden {
				t.Errorf("status = %d, want %d for path %s", w.Code, http.StatusForbidden, path)
			}

			var errResp errorResponse
			if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
				t.Fatalf("decode error response: %v", err)
			}
			if !strings.Contains(errResp.Error, "provider routes") {
				t.Errorf("error = %q, want provider routes denial message", errResp.Error)
			}
		})
	}
}

// TestProviderRouteDeniedWithAuth verifies that even authenticated users
// cannot access /provider/* routes through the proxy (VAL-GATEWAY-002).
func TestProviderRouteDeniedWithAuth(t *testing.T) {
	handler, priv, _ := testProxyEnv(t)

	// Create a valid access JWT.
	token := issueTestAccessJWTWithTTL(priv, "user-1", 5*time.Minute)
	req := httptest.NewRequest(http.MethodPost, "/provider/v1/inference", strings.NewReader(`{"messages":[]}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "choir_access", Value: token})

	w := httptest.NewRecorder()
	handler.HandleProviderDeny(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d (even with auth)", w.Code, http.StatusForbidden)
	}
}

// --- VAL-VM-001, VAL-VM-002: vmctl-backed routing tests ---

// testVMctlProxyEnv sets up a proxy Handler with a vmctl service backend,
// a fake autoputer backend, and Ed25519 key material. Returns the handler,
// signing key, autoputer server, and vmctl test server.
func testVMctlProxyEnv(t *testing.T) (*Handler, ed25519.PrivateKey, *httptest.Server, *httptest.Server) {
	t.Helper()

	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate ed25519 key: %v", err)
	}

	// Create a fake autoputer backend.
	autoputerMux := http.NewServeMux()
	autoputerMux.HandleFunc("/api/shell/bootstrap", func(w http.ResponseWriter, r *http.Request) {
		user := r.Header.Get("X-Authenticated-User")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"computer_id": "autoputer-vmctl-test",
			"user":        user,
			"bootstrap":   "vm-routed",
			"path":        r.URL.Path,
		})
	})
	autoputerMux.HandleFunc("/api/prompt-bar", func(w http.ResponseWriter, r *http.Request) {
		user := r.Header.Get("X-Authenticated-User")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"submission_id": "task-123",
			"owner_id":      user,
			"state":         "accepted",
		})
	})
	autoputerMux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status":           "ready",
			"service":          "autoputer",
			"runtime_health":   "ready",
			"running_runs":     0,
			"researcher_count": 1,
			"persistent_disk": map[string]interface{}{
				"source":            "guest",
				"used_bytes":        2 * 1024 * 1024 * 1024,
				"total_bytes":       8 * 1024 * 1024 * 1024,
				"avail_bytes":       6 * 1024 * 1024 * 1024,
				"cap_bytes":         8 * 1024 * 1024 * 1024,
				"used_percent":      25,
				"warning":           false,
				"critical":          false,
				"default_cap_bytes": 8 * 1024 * 1024 * 1024,
			},
		})
	})

	autoputerServer := httptest.NewServer(autoputerMux)
	t.Cleanup(func() { autoputerServer.Close() })

	// Create a vmctl service.
	reg := vmctl.NewOwnershipRegistry(autoputerServer.URL)
	vmctlHandler := vmctl.NewHandler(reg)

	vmctlMux := http.NewServeMux()
	vmctlMux.HandleFunc("/internal/vmctl/resolve", vmctlHandler.HandleResolve)
	vmctlMux.HandleFunc("/internal/vmctl/lookup", vmctlHandler.HandleLookup)
	vmctlMux.HandleFunc("/internal/vmctl/stop", vmctlHandler.HandleStop)
	vmctlMux.HandleFunc("/internal/vmctl/list", vmctlHandler.HandleList)
	vmctlMux.HandleFunc("/internal/vmctl/refresh", vmctlHandler.HandleRefresh)
	vmctlMux.HandleFunc("/health", vmctlHandler.HandleHealth)

	vmctlServer := httptest.NewServer(vmctlMux)
	t.Cleanup(func() { vmctlServer.Close() })

	// Create proxy config with vmctl routing enabled.
	cfg := &Config{AllowDirectAutoputerForTests: true, Port: "0",
		ComputerURL:       autoputerServer.URL,
		AuthPublicKeyPath: "/unused/in/test",
		VmctlURL:          vmctlServer.URL}

	if !cfg.VmctlRoutingEnabled() {
		t.Fatal("expected vmctl routing to be enabled")
	}

	handler, err := NewHandler(cfg, pub)
	if err != nil {
		t.Fatalf("NewHandler with vmctl: %v", err)
	}

	return handler, priv, autoputerServer, vmctlServer
}

func TestResolveComputerURLHasNoProductionStaticFallback(t *testing.T) {
	var backendCalls atomic.Int64
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		backendCalls.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()
	h, err := NewHandler(&Config{ComputerURL: backend.URL}, make(ed25519.PublicKey, ed25519.PublicKeySize))
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}
	if _, err := h.resolveComputerURL(context.Background(), "owner", "primary"); err == nil {
		t.Fatal("production route used static autoputer without vmctl")
	}
	if backendCalls.Load() != 0 {
		t.Fatalf("static autoputer calls = %d, want zero", backendCalls.Load())
	}
}

func TestResolveComputerURLRequiresJoinedComputerVersionRoute(t *testing.T) {
	createdAt := time.Date(2026, 7, 16, 1, 0, 0, 0, time.UTC)
	closure, err := computerversion.NewCodeClosure(strings.Repeat("1", 40), []computerversion.CodeArtifact{{
		Name: "autoputer", SHA256: strings.Repeat("a", 64), URI: "nix-store+sha256://" + strings.Repeat("a", 64) + "/nix/store/test-autoputer",
	}}, createdAt)
	if err != nil {
		t.Fatal(err)
	}
	program, err := computerversion.NewArtifactProgram([]computerversion.ArtifactProgramEntry{{
		Kind: "test", ContentSHA256: strings.Repeat("b", 64), ArtifactURI: "artifact+sha256://" + strings.Repeat("b", 64) + "/test/state",
	}}, createdAt)
	if err != nil {
		t.Fatal(err)
	}
	version := computerversion.ComputerVersion{CodeRef: closure.Ref, ArtifactProgramRef: program.Ref}
	var routeCalls, resolveCalls atomic.Int64
	autoputer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	defer autoputer.Close()
	mux := http.NewServeMux()
	mux.HandleFunc("/internal/vmctl/computer-version-routes/resolve", func(w http.ResponseWriter, r *http.Request) {
		routeCalls.Add(1)
		slotID := r.URL.Query().Get("route_slot_id")
		_ = json.NewEncoder(w).Encode(vmctl.RouteResolution{
			Slot:            routeledger.Slot{ID: slotID, Current: version, Generation: 1, LatestReceiptID: "receipt-1"},
			LatestReceipt:   routeledger.TransitionReceipt{ID: "receipt-1", RouteSlotID: slotID, New: version, CommittedGeneration: 1},
			CodeClosure:     closure,
			ArtifactProgram: program,
		})
	})
	mux.HandleFunc("/internal/vmctl/resolve", func(w http.ResponseWriter, _ *http.Request) {
		resolveCalls.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"vm_id": "vm-joined", "user_id": "owner", "desktop_id": "primary",
			"published": true, "computer_url": autoputer.URL, "state": "active",
		})
	})
	vmctlServer := httptest.NewServer(mux)
	defer vmctlServer.Close()
	h, err := NewHandler(&Config{ComputerURL: autoputer.URL, VmctlURL: vmctlServer.URL}, make(ed25519.PublicKey, ed25519.PublicKeySize))
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}
	got, err := h.resolveComputerURL(context.Background(), "owner", "primary")
	if err != nil {
		t.Fatalf("resolve joined route: %v", err)
	}
	if got != autoputer.URL || routeCalls.Load() != 1 || resolveCalls.Load() != 1 {
		t.Fatalf("joined route result url=%q route_calls=%d resolve_calls=%d", got, routeCalls.Load(), resolveCalls.Load())
	}
}

func TestResolveComputerURLRefusesBeforeVMResolutionWhenRouteMissing(t *testing.T) {
	var resolveCalls atomic.Int64
	mux := http.NewServeMux()
	mux.HandleFunc("/internal/vmctl/computer-version-routes/resolve", func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"error":"slot not found"}`, http.StatusNotFound)
	})
	mux.HandleFunc("/internal/vmctl/resolve", func(w http.ResponseWriter, _ *http.Request) {
		resolveCalls.Add(1)
		w.WriteHeader(http.StatusOK)
	})
	vmctlServer := httptest.NewServer(mux)
	defer vmctlServer.Close()
	h, err := NewHandler(&Config{ComputerURL: "http://invalid", VmctlURL: vmctlServer.URL}, make(ed25519.PublicKey, ed25519.PublicKeySize))
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}
	if _, err := h.resolveComputerURL(context.Background(), "owner", "primary"); err == nil {
		t.Fatal("missing D-ROUTE reached VM resolution")
	}
	if resolveCalls.Load() != 0 {
		t.Fatalf("VM resolution calls = %d, want zero", resolveCalls.Load())
	}
}

// TestVMctlRouting_BootstrapRoutesThroughVM tests that protected bootstrap
// routes resolve through vmctl ownership (VAL-VM-001, VAL-VM-002).
func TestVMctlRouting_BootstrapRoutesThroughVM(t *testing.T) {
	handler, priv, _, vmctlSrv := testVMctlProxyEnv(t)
	_ = vmctlSrv

	accessToken := issueTestAccessJWT(priv, "user-vm-1")

	req := httptest.NewRequest(http.MethodGet, "/api/shell/bootstrap", nil)
	req.Header.Set("Cookie", "choir_access="+accessToken)
	w := httptest.NewRecorder()
	handler.HandleBootstrap(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("decode result: %v", err)
	}

	// The autoputer should have received the user context.
	if result["user"] != "user-vm-1" {
		t.Errorf("expected user user-vm-1, got %v", result["user"])
	}
	if result["bootstrap"] != "vm-routed" {
		t.Errorf("expected vm-routed bootstrap, got %v", result["bootstrap"])
	}
}

func TestVMctlRouting_BootstrapResolveSurvivesCanceledRequestContext(t *testing.T) {
	handler, priv, _, vmctlSrv := testVMctlProxyEnv(t)
	accessToken := issueTestAccessJWT(priv, "user-vm-cancel")
	req := httptest.NewRequest(http.MethodGet, "/api/shell/bootstrap", nil)
	req.Header.Set("Cookie", "choir_access="+accessToken)
	ctx, cancel := context.WithCancel(req.Context())
	cancel()
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	handler.HandleBootstrap(w, req)
	client := vmctl.NewClient(vmctlSrv.URL)
	lookup, err := client.Lookup("user-vm-cancel")
	if err != nil || lookup == nil || strings.TrimSpace(lookup.ComputerURL) == "" {
		t.Fatalf("vmctl resolve must complete despite canceled request: status=%d lookup=%+v err=%v", w.Code, lookup, err)
	}
}

// TestVMctlRouting_DifferentUsersGetDifferentVMs tests that different users
// receive distinct VMs (VAL-VM-005, VAL-CROSS-113).
func TestVMctlRouting_DifferentUsersGetDifferentVMs(t *testing.T) {
	handler, priv, _, vmctlSrv := testVMctlProxyEnv(t)
	_ = vmctlSrv

	user1Token := issueTestAccessJWT(priv, "alice")
	user2Token := issueTestAccessJWT(priv, "bob")

	// User 1 request.
	req1 := httptest.NewRequest(http.MethodGet, "/api/shell/bootstrap", nil)
	req1.Header.Set("Cookie", "choir_access="+user1Token)
	w1 := httptest.NewRecorder()
	handler.HandleBootstrap(w1, req1)

	// User 2 request.
	req2 := httptest.NewRequest(http.MethodGet, "/api/shell/bootstrap", nil)
	req2.Header.Set("Cookie", "choir_access="+user2Token)
	w2 := httptest.NewRecorder()
	handler.HandleBootstrap(w2, req2)

	if w1.Code != http.StatusOK || w2.Code != http.StatusOK {
		t.Fatalf("expected both 200, got %d and %d", w1.Code, w2.Code)
	}

	// Verify the vmctl registry created distinct VMs for each user.
	client := vmctl.NewClient(vmctlSrv.URL)
	lookup1, _ := client.Lookup("alice")
	lookup2, _ := client.Lookup("bob")

	if lookup1 == nil || lookup2 == nil {
		t.Fatal("expected both users to have VM ownership")
	}
	if lookup1.VMID == lookup2.VMID {
		t.Error("expected different VM IDs for different users (VAL-VM-005)")
	}
}

func TestVMctlRouting_UnknownDesktopSelectorDoesNotMintVM(t *testing.T) {
	handler, priv, _, vmctlSrv := testVMctlProxyEnv(t)

	client := vmctl.NewClient(vmctlSrv.URL)
	if _, err := client.ResolveDesktop("alice", vmctl.PrimaryDesktopID); err != nil {
		t.Fatalf("ResolveDesktop primary: %v", err)
	}

	accessToken := issueTestAccessJWT(priv, "alice")
	req := httptest.NewRequest(http.MethodGet, "/api/shell/bootstrap?desktop_id=branch-a", nil)
	req.Header.Set("Cookie", "choir_access="+accessToken)
	w := httptest.NewRecorder()
	handler.HandleBootstrap(w, req)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 for unknown desktop, got %d", w.Code)
	}

	branch, err := client.LookupDesktop("alice", "branch-a")
	if err != nil {
		t.Fatalf("LookupDesktop branch-a: %v", err)
	}
	if branch != nil {
		t.Fatalf("browser-selected unknown desktop minted VM ownership: %+v", branch)
	}
}

func TestResolveComputerURLRetriesTransientVMctlFailure(t *testing.T) {
	oldWindow := autoputerResolveRetryWindow
	oldBaseDelay := autoputerResolveRetryBaseDelay
	oldMaxDelay := autoputerResolveRetryMaxDelay
	autoputerResolveRetryWindow = 100 * time.Millisecond
	autoputerResolveRetryBaseDelay = time.Millisecond
	autoputerResolveRetryMaxDelay = 5 * time.Millisecond
	t.Cleanup(func() {
		autoputerResolveRetryWindow = oldWindow
		autoputerResolveRetryBaseDelay = oldBaseDelay
		autoputerResolveRetryMaxDelay = oldMaxDelay
	})

	attempts := 0
	vmctlSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			http.Error(w, "vmctl warming up", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"vm_id":        "vm-retry",
			"user_id":      "alice",
			"desktop_id":   vmctl.PrimaryDesktopID,
			"kind":         string(vmctl.VMKindInteractive),
			"published":    true,
			"computer_url": "http://127.0.0.1:8085",
			"state":        string(vmctl.VMStateActive),
		})
	}))
	defer vmctlSrv.Close()

	handler := &Handler{cfg: &Config{AllowDirectAutoputerForTests: true}, vmctlClient: vmctl.NewClient(vmctlSrv.URL)}
	got, err := handler.resolveComputerURL(context.Background(), "alice", vmctl.PrimaryDesktopID)
	if err != nil {
		t.Fatalf("resolveComputerURL: %v", err)
	}
	if got != "http://127.0.0.1:8085" {
		t.Fatalf("autoputer URL = %q, want proxy target", got)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
}

func TestResolveComputerURLUsesResolveForUniversalWirePlatformComputer(t *testing.T) {
	resolveCalls := 0
	lookupCalls := 0
	vmctlSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/internal/vmctl/resolve":
			resolveCalls++
			var req struct {
				UserID    string `json:"user_id"`
				DesktopID string `json:"desktop_id"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode resolve request: %v", err)
			}
			if req.UserID != vmctl.UniversalWirePlatformOwnerID || req.DesktopID != vmctl.UniversalWirePlatformDesktopID {
				t.Fatalf("resolve request = %+v, want Universal Wire platform computer", req)
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"vm_id":          vmctl.UniversalWirePlatformVMID,
				"user_id":        vmctl.UniversalWirePlatformOwnerID,
				"desktop_id":     vmctl.UniversalWirePlatformDesktopID,
				"kind":           string(vmctl.VMKindInteractive),
				"published":      true,
				"computer_url":   "http://10.203.141.2:8085",
				"state":          string(vmctl.VMStateActive),
				"warmness_class": "public_platform",
			})
		case "/internal/vmctl/lookup":
			lookupCalls++
			http.Error(w, "lookup should not serve platform computer", http.StatusInternalServerError)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(func() { vmctlSrv.Close() })

	handler := &Handler{cfg: &Config{AllowDirectAutoputerForTests: true}, vmctlClient: vmctl.NewClient(vmctlSrv.URL)}
	got, err := handler.resolveComputerURL(context.Background(), vmctl.UniversalWirePlatformOwnerID, vmctl.UniversalWirePlatformDesktopID)
	if err != nil {
		t.Fatalf("resolveComputerURL: %v", err)
	}
	if got != "http://10.203.141.2:8085" {
		t.Fatalf("autoputer URL = %q, want platform autoputer", got)
	}
	if resolveCalls != 1 || lookupCalls != 0 {
		t.Fatalf("vmctl calls: resolve=%d lookup=%d, want resolve-only", resolveCalls, lookupCalls)
	}
}

// TestVMctlRouting_InvalidAuthDeniedBeforeVMSideEffects tests that invalid
// auth is denied before VM ownership changes or runtime side effects
// (VAL-CROSS-110).
func TestVMctlRouting_InvalidAuthDeniedBeforeVMSideEffects(t *testing.T) {
	handler, priv, _, vmctlSrv := testVMctlProxyEnv(t)
	_ = vmctlSrv

	// Issue a token then expire it.
	expiredToken := issueTestAccessJWTWithTTL(priv, "user-would-be", -1*time.Minute)

	req := httptest.NewRequest(http.MethodGet, "/api/shell/bootstrap", nil)
	req.Header.Set("Cookie", "choir_access="+expiredToken)
	w := httptest.NewRecorder()
	handler.HandleBootstrap(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for expired token, got %d", w.Code)
	}

	// The user should NOT have a VM assignment since auth was denied
	// before the resolve step (VAL-CROSS-110).
	client := vmctl.NewClient(vmctlSrv.URL)
	lookup, _ := client.Lookup("user-would-be")
	if lookup != nil {
		t.Error("expected no VM assignment when auth is denied (VAL-CROSS-110)")
	}
}

// TestVMctlRouting_SameUserPinnedToSameVM tests that repeated requests
// from the same user stay pinned to the same VM (VAL-VM-003).
func TestVMctlRouting_SameUserPinnedToSameVM(t *testing.T) {
	handler, priv, _, vmctlSrv := testVMctlProxyEnv(t)
	_ = vmctlSrv

	accessToken := issueTestAccessJWT(priv, "user-pinned")

	// Make two requests.
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/shell/bootstrap", nil)
		req.Header.Set("Cookie", "choir_access="+accessToken)
		w := httptest.NewRecorder()
		handler.HandleBootstrap(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i+1, w.Code)
		}
	}

	// Should still have exactly one VM for this user.
	client := vmctl.NewClient(vmctlSrv.URL)
	lookup, _ := client.Lookup("user-pinned")
	if lookup == nil {
		t.Fatal("expected user to have a VM")
	}
	// The VM ID should be stable.
	vmID := lookup.VMID

	// Make another request and verify the VM hasn't changed.
	req := httptest.NewRequest(http.MethodGet, "/api/shell/bootstrap", nil)
	req.Header.Set("Cookie", "choir_access="+accessToken)
	w := httptest.NewRecorder()
	handler.HandleBootstrap(w, req)

	lookup2, _ := client.Lookup("user-pinned")
	if lookup2.VMID != vmID {
		t.Errorf("expected pinned VM %s, got %s (VAL-VM-003)", vmID, lookup2.VMID)
	}
}

func TestHandlePlatformTextureReadForwardsCurrentRevisionID(t *testing.T) {
	handler, priv, _ := testProxyEnv(t)

	corpusd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/platform/texture/documents/doc-wire-1" {
			t.Fatalf("corpusd path = %q, want document read", r.URL.Path)
		}
		if r.Header.Get("X-Internal-Caller") != "true" {
			t.Fatalf("missing internal caller header")
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"doc_id":              "doc-wire-1",
			"owner_id":            vmctl.UniversalWirePlatformOwnerID,
			"title":               "Wire article",
			"current_revision_id": "rev-wire-head",
		})
	}))
	t.Cleanup(corpusd.Close)
	handler.cfg.CorpusdURL = corpusd.URL

	req := httptest.NewRequest(http.MethodGet, "/api/texture/documents/doc-wire-1?read_owner="+vmctl.UniversalWirePlatformOwnerID, nil)
	req.AddCookie(&http.Cookie{Name: "choir_access", Value: issueTestAccessJWT(priv, "reader-1")})
	rr := httptest.NewRecorder()

	handler.HandlePlatformTextureRead(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var got map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got["current_revision_id"] != "rev-wire-head" {
		t.Fatalf("current_revision_id = %q, want rev-wire-head; body=%s", got["current_revision_id"], rr.Body.String())
	}
}

func TestHandlePlatformTextureReadForwardsRevisionListEnvelope(t *testing.T) {
	handler, priv, _ := testProxyEnv(t)

	corpusd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/platform/texture/documents/doc-wire-1/revisions" {
			t.Fatalf("corpusd path = %q, want revision list read", r.URL.Path)
		}
		if r.Header.Get("X-Internal-Caller") != "true" {
			t.Fatalf("missing internal caller header")
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"revisions": []map[string]any{{
				"revision_id": "rev-wire-head",
				"doc_id":      "doc-wire-1",
				"owner_id":    vmctl.UniversalWirePlatformOwnerID,
				"content":     "Wire article body",
				"created_at":  "2026-06-26T22:00:00Z",
			}},
		})
	}))
	t.Cleanup(corpusd.Close)
	handler.cfg.CorpusdURL = corpusd.URL

	req := httptest.NewRequest(http.MethodGet, "/api/texture/documents/doc-wire-1/revisions?limit=10000&read_owner="+vmctl.UniversalWirePlatformOwnerID, nil)
	req.AddCookie(&http.Cookie{Name: "choir_access", Value: issueTestAccessJWT(priv, "reader-1")})
	rr := httptest.NewRecorder()

	handler.HandlePlatformTextureRead(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rr.Code, rr.Body.String())
	}
	var got struct {
		Revisions []struct {
			RevisionID string `json:"revision_id"`
			Content    string `json:"content"`
		} `json:"revisions"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(got.Revisions) != 1 || got.Revisions[0].RevisionID != "rev-wire-head" || got.Revisions[0].Content != "Wire article body" {
		t.Fatalf("revision list response = %+v, want wrapped platform revision", got)
	}
}

func TestVMctlRouting_ProtectedAPIThroughVM(t *testing.T) {
	handler, priv, _, vmctlSrv := testVMctlProxyEnv(t)
	_ = vmctlSrv

	accessToken := issueTestAccessJWT(priv, "user-runtime")

	req := httptest.NewRequest(http.MethodPost, "/api/prompt-bar", strings.NewReader(`{"text":"test"}`))
	req.Header.Set("Cookie", "choir_access="+accessToken)
	w := httptest.NewRecorder()
	handler.HandleProtectedAPI(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if result["owner_id"] != "user-runtime" {
		t.Errorf("expected owner_id user-runtime, got %v", result["owner_id"])
	}
}

// TestVMctlDeny_PublicVMctlBlocked tests that /internal/vmctl/* routes are
// denied to browser callers (VAL-VM-012).
func TestVMctlDeny_PublicVMctlBlocked(t *testing.T) {
	handler, _, _, _ := testVMctlProxyEnv(t)

	req := httptest.NewRequest(http.MethodPost, "/internal/vmctl/resolve", nil)
	w := httptest.NewRecorder()
	handler.HandleVMctlDeny(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}

	var result errorResponse
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if !strings.Contains(result.Error, "not publicly accessible") {
		t.Errorf("expected public denial message, got: %s", result.Error)
	}
}

// TestVMctlRouting_HealthReportsRedactedVMctlStatus tests that proxy health
// includes only coarse vmctl status when routing is enabled.
func TestVMctlRouting_HealthReportsRedactedVMctlStatus(t *testing.T) {
	handler, _, _, _ := testVMctlProxyEnv(t)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	handler.HandleHealth(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var result proxyHealthResponse
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if result.VMctlRouting != "enabled" {
		t.Errorf("expected vmctl_routing=enabled, got %s", result.VMctlRouting)
	}
	if result.VMctlStatus != "ok" {
		t.Errorf("expected vmctl_status=ok, got %s", result.VMctlStatus)
	}
	body := w.Body.String()
	for _, forbidden := range []string{"vmctl_url", "vmctl_health", "active_vms", "total_ownerships", "memory_available_bytes", "computer_url", "reclaim", "warmness"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("health leaked %q in %s", forbidden, body)
		}
	}
}

func TestCurrentImmutableIdentityJoinsRedactedProductEvidence(t *testing.T) {
	createdAt := time.Date(2026, 7, 17, 19, 0, 0, 0, time.UTC)
	closure, err := computerversion.NewCodeClosure(strings.Repeat("1", 40), []computerversion.CodeArtifact{{
		Name: "autoputer", SHA256: strings.Repeat("a", 64), URI: "nix-store+sha256://" + strings.Repeat("a", 64) + "/nix/store/test-autoputer",
	}}, createdAt)
	if err != nil {
		t.Fatal(err)
	}
	program, err := computerversion.NewArtifactProgram([]computerversion.ArtifactProgramEntry{{
		Kind: "test", ContentSHA256: strings.Repeat("b", 64), ArtifactURI: "artifact+sha256://" + strings.Repeat("b", 64) + "/test/state",
	}}, createdAt)
	if err != nil {
		t.Fatal(err)
	}
	version := computerversion.ComputerVersion{CodeRef: closure.Ref, ArtifactProgramRef: program.Ref}
	slotID, _ := routeledger.RouteSlotID("owner-private", "primary")
	receipt := routeledger.TransitionReceipt{
		ID: "11111111-1111-4111-8111-111111111111", RouteSlotID: slotID, Kind: routeledger.TransitionBootstrap,
		New: version, CommittedGeneration: 1, ApprovalRef: routeledger.ApprovalRef("approval:sha256:" + strings.Repeat("c", 64)),
		PromotionCertificateRef: routeledger.PromotionCertificateRef("certificate:sha256:" + strings.Repeat("d", 64)),
		IdempotencyKey:          routeledger.IdempotencyKey("idempotency:product-inspection"), CommittedAt: createdAt,
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(vmctl.RouteResolution{
			Slot:          routeledger.Slot{ID: slotID, Current: version, Generation: 1, LatestReceiptID: receipt.ID},
			LatestReceipt: receipt, CodeClosure: closure, ArtifactProgram: program,
		})
	}))
	defer srv.Close()
	h := &Handler{vmctlClient: vmctl.NewClient(srv.URL)}
	identity, err := h.currentImmutableIdentity(t.Context(), "owner-private", "primary")
	if err != nil {
		t.Fatal(err)
	}
	if identity == nil || !identity.Joined || identity.ComputerVersion != version || identity.RouteReceiptID != string(receipt.ID) || identity.RouteGeneration != 1 || identity.ApprovalRef != string(receipt.ApprovalRef) {
		t.Fatalf("immutable identity = %+v", identity)
	}
	body, err := json.Marshal(identity)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"owner-private", slotID, "vm_id", "computer_url", "device_path", "construction_disk_receipt_id"} {
		if strings.Contains(string(body), forbidden) {
			t.Fatalf("immutable identity leaked %q in %s", forbidden, body)
		}
	}
}

func TestCurrentImmutableIdentityAcceptsCanonicalRouteAbsence(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(vmctl.RouteResolution{RouteAbsent: true})
	}))
	defer server.Close()
	handler := &Handler{vmctlClient: vmctl.NewClient(server.URL)}

	if err := handler.ensureComputerVersionRoute(t.Context(), "ordinary-owner", "primary"); err != nil {
		t.Fatalf("canonical route absence blocked ordinary boot: %v", err)
	}
	if err := handler.ensureComputerVersionRoute(
		t.Context(), vmctl.UniversalWirePlatformOwnerID, vmctl.UniversalWirePlatformDesktopID,
	); err == nil {
		t.Fatal("canonical route absence authorized the universal-wire platform path")
	}
	identity, err := handler.currentImmutableIdentity(t.Context(), "ordinary-owner", "primary")
	if err != nil {
		t.Fatal(err)
	}
	if identity == nil || !identity.RouteAbsent || identity.Joined || identity.CodeCommit != "" {
		t.Fatalf("route-absent identity=%+v", identity)
	}
}

func TestComputeStatusDoesNotCreateOwnershipAndRedactsIdentity(t *testing.T) {
	handler, priv, _, vmctlSrv := testVMctlProxyEnv(t)
	client := vmctl.NewClient(vmctlSrv.URL)
	token := issueTestAccessJWT(priv, "compute-status-user")

	req := httptest.NewRequest(http.MethodGet, "/api/compute/status", nil)
	req.AddCookie(&http.Cookie{Name: "choir_access", Value: token})
	w := httptest.NewRecorder()
	handler.HandleAPI(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("compute status = %d, want 200 body=%s", w.Code, w.Body.String())
	}

	var result computeStatusResponse
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("decode compute status: %v", err)
	}
	if result.Service != "compute-monitor" {
		t.Fatalf("service = %s, want compute-monitor", result.Service)
	}
	if result.CurrentComputer.LookupStatus != "not_found" {
		t.Fatalf("lookup status = %s, want not_found", result.CurrentComputer.LookupStatus)
	}
	if result.CurrentComputer.State != "not_started" {
		t.Fatalf("computer state = %s, want not_started", result.CurrentComputer.State)
	}
	own, err := client.LookupDesktop("compute-status-user", vmctl.PrimaryDesktopID)
	if err != nil {
		t.Fatalf("lookup after status: %v", err)
	}
	if own != nil {
		t.Fatalf("compute status should not create ownership, got %+v", own)
	}

	body := w.Body.String()
	for _, forbidden := range []string{
		"compute-status-user",
		"vm_id",
		"computer_url",
		"user_id",
		"state_dir",
		"vmctl",
		"active_vms",
		"total_ownerships",
		"memory_available_bytes",
		"lifecycle",
		"build",
	} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("compute status leaked %q in %s", forbidden, body)
		}
	}
}

func TestSystemMonitorRoutesAreHardCutOver(t *testing.T) {
	handler, priv, _, vmctlSrv := testVMctlProxyEnv(t)
	client := vmctl.NewClient(vmctlSrv.URL)
	token := issueTestAccessJWT(priv, "legacy-system-user")

	req := httptest.NewRequest(http.MethodGet, "/api/system/status", nil)
	req.AddCookie(&http.Cookie{Name: "choir_access", Value: token})
	w := httptest.NewRecorder()
	handler.HandleAPI(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("legacy system route = %d, want 404 body=%s", w.Code, w.Body.String())
	}

	own, err := client.LookupDesktop("legacy-system-user", vmctl.PrimaryDesktopID)
	if err != nil {
		t.Fatalf("lookup after legacy system route: %v", err)
	}
	if own != nil {
		t.Fatalf("legacy system route should not resolve or create ownership, got %+v", own)
	}
}

func TestComputeStatusListsOnlyUserComputers(t *testing.T) {
	handler, priv, _, vmctlSrv := testVMctlProxyEnv(t)
	client := vmctl.NewClient(vmctlSrv.URL)
	token := issueTestAccessJWT(priv, "compute-list-user")

	if _, err := client.ResolveDesktop("compute-list-user", vmctl.PrimaryDesktopID); err != nil {
		t.Fatalf("resolve primary: %v", err)
	}
	if _, err := client.ResolveDesktop("other-compute-user", vmctl.PrimaryDesktopID); err != nil {
		t.Fatalf("resolve other user: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/compute/status", nil)
	req.AddCookie(&http.Cookie{Name: "choir_access", Value: token})
	w := httptest.NewRecorder()
	handler.HandleAPI(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("compute status = %d, want 200 body=%s", w.Code, w.Body.String())
	}
	var result computeStatusResponse
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("decode compute status: %v", err)
	}
	if len(result.Computers) != 1 {
		t.Fatalf("computers len = %d, want 1: %+v", len(result.Computers), result.Computers)
	}
	var sawPrimary bool
	for _, computer := range result.Computers {
		if computer.DesktopID == vmctl.PrimaryDesktopID && computer.Role == "primary" {
			sawPrimary = true
		}
		if !strings.HasPrefix(computer.ComputerID, "computer-") {
			t.Fatalf("computer %s missing stable computer_id: %+v", computer.DesktopID, computer)
		}
	}
	if !sawPrimary {
		t.Fatalf("missing primary computer: %+v", result.Computers)
	}
	if result.CurrentComputer.ComputerID != result.Computers[0].ComputerID {
		t.Fatalf("current computer_id %q != listed %q", result.CurrentComputer.ComputerID, result.Computers[0].ComputerID)
	}
	body := w.Body.String()
	for _, forbidden := range []string{"other-compute-user", "compute-list-user", "vm_id", "computer_url", "user_id"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("compute status leaked %q in %s", forbidden, body)
		}
	}
}

func TestComputeStatusReportsPersistentDiskFromRuntimeHealth(t *testing.T) {
	handler, priv, _, vmctlSrv := testVMctlProxyEnv(t)
	client := vmctl.NewClient(vmctlSrv.URL)
	if _, err := client.ResolveDesktop("compute-disk-user", vmctl.PrimaryDesktopID); err != nil {
		t.Fatalf("resolve desktop: %v", err)
	}
	token := issueTestAccessJWT(priv, "compute-disk-user")

	req := httptest.NewRequest(http.MethodGet, "/api/compute/status", nil)
	req.AddCookie(&http.Cookie{Name: "choir_access", Value: token})
	w := httptest.NewRecorder()
	handler.HandleAPI(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("compute status = %d, want 200 body=%s", w.Code, w.Body.String())
	}

	var result computeStatusResponse
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("decode compute status: %v", err)
	}
	if result.PersistentDisk == nil {
		t.Fatalf("expected persistent_disk in compute status: %+v", result)
	}
	if result.PersistentDisk.Source != "guest" {
		t.Fatalf("persistent_disk.source = %q, want guest", result.PersistentDisk.Source)
	}
	if result.PersistentDisk.CapBytes != 8*1024*1024*1024 {
		t.Fatalf("cap_bytes = %d, want 8GiB", result.PersistentDisk.CapBytes)
	}
}

func TestComputeRecoveryWakeCreatesRedactedCurrentComputer(t *testing.T) {
	handler, priv, _, vmctlSrv := testVMctlProxyEnv(t)
	client := vmctl.NewClient(vmctlSrv.URL)
	token := issueTestAccessJWT(priv, "compute-recovery-user")

	req := httptest.NewRequest(http.MethodPost, "/api/compute/recovery", strings.NewReader(`{"action":"wake_current_computer"}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "choir_access", Value: token})
	w := httptest.NewRecorder()
	handler.HandleAPI(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("compute recovery = %d, want 200 body=%s", w.Code, w.Body.String())
	}

	var result computeRecoveryResponse
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("decode recovery response: %v", err)
	}
	if !result.OK {
		t.Fatal("recovery response ok=false")
	}
	if result.CurrentComputer.LookupStatus != "ok" {
		t.Fatalf("lookup status = %s, want ok", result.CurrentComputer.LookupStatus)
	}
	if result.CurrentComputer.State != string(vmctl.VMStateActive) {
		t.Fatalf("computer state = %s, want active", result.CurrentComputer.State)
	}
	own, err := client.LookupDesktop("compute-recovery-user", vmctl.PrimaryDesktopID)
	if err != nil {
		t.Fatalf("lookup after recovery: %v", err)
	}
	if own == nil {
		t.Fatal("wake recovery did not create/resume ownership")
	}

	body := w.Body.String()
	for _, forbidden := range []string{"compute-recovery-user", "vm_id", "computer_url", "user_id", "state_dir", "build", "active_provider"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("compute recovery leaked %q in %s", forbidden, body)
		}
	}
}

func TestComputeRecoveryStopUsesOwnerScopedVMCTL(t *testing.T) {
	handler, priv, _, vmctlSrv := testVMctlProxyEnv(t)
	client := vmctl.NewClient(vmctlSrv.URL)
	token := issueTestAccessJWT(priv, "compute-stop-user")

	wakeReq := httptest.NewRequest(http.MethodPost, "/api/compute/recovery", strings.NewReader(`{"action":"wake_current_computer"}`))
	wakeReq.Header.Set("Content-Type", "application/json")
	wakeReq.AddCookie(&http.Cookie{Name: "choir_access", Value: token})
	wakeW := httptest.NewRecorder()
	handler.HandleAPI(wakeW, wakeReq)
	if wakeW.Code != http.StatusOK {
		t.Fatalf("wake current computer = %d body=%s", wakeW.Code, wakeW.Body.String())
	}

	stopReq := httptest.NewRequest(http.MethodPost, "/api/compute/recovery", strings.NewReader(`{"action":"stop_current_computer"}`))
	stopReq.Header.Set("Content-Type", "application/json")
	stopReq.AddCookie(&http.Cookie{Name: "choir_access", Value: token})
	stopW := httptest.NewRecorder()
	handler.HandleAPI(stopW, stopReq)
	if stopW.Code != http.StatusOK {
		t.Fatalf("stop current computer = %d body=%s", stopW.Code, stopW.Body.String())
	}
	var result computeRecoveryResponse
	if err := json.NewDecoder(stopW.Body).Decode(&result); err != nil {
		t.Fatalf("decode stop response: %v", err)
	}
	if !result.OK || result.Action != "stop_current_computer" || result.CurrentComputer.State != string(vmctl.VMStateStopped) {
		t.Fatalf("stop response = %+v", result)
	}
	own, err := client.LookupDesktop("compute-stop-user", vmctl.PrimaryDesktopID)
	if err != nil {
		t.Fatalf("lookup stopped computer: %v", err)
	}
	if own == nil || own.State != string(vmctl.VMStateStopped) || own.UserID != "compute-stop-user" {
		t.Fatalf("stopped ownership = %+v", own)
	}
}

func TestComputeRecoveryWakeRefreshesUnreachableCurrentComputer(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate ed25519 key: %v", err)
	}

	var refreshed atomic.Bool
	autoputerMux := http.NewServeMux()
	autoputerMux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if !refreshed.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "boot_failed"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "service": "autoputer"})
	})
	autoputerSrv := httptest.NewServer(autoputerMux)
	t.Cleanup(func() { autoputerSrv.Close() })

	reg := vmctl.NewOwnershipRegistry(autoputerSrv.URL)
	vmctlHandler := vmctl.NewHandler(reg)
	vmctlMux := http.NewServeMux()
	vmctlMux.HandleFunc("/internal/vmctl/resolve", vmctlHandler.HandleResolve)
	vmctlMux.HandleFunc("/internal/vmctl/lookup", vmctlHandler.HandleLookup)
	vmctlMux.HandleFunc("/internal/vmctl/list", vmctlHandler.HandleList)
	vmctlMux.HandleFunc("/internal/vmctl/refresh", func(w http.ResponseWriter, r *http.Request) {
		refreshed.Store(true)
		vmctlHandler.HandleRefresh(w, r)
	})
	vmctlMux.HandleFunc("/health", vmctlHandler.HandleHealth)
	vmctlSrv := httptest.NewServer(vmctlMux)
	t.Cleanup(func() { vmctlSrv.Close() })

	cfg := &Config{AllowDirectAutoputerForTests: true, Port: "0",
		ComputerURL:       autoputerSrv.URL,
		AuthPublicKeyPath: "/unused/in/test",
		VmctlURL:          vmctlSrv.URL}
	handler, err := NewHandler(cfg, pub)
	if err != nil {
		t.Fatalf("NewHandler with vmctl: %v", err)
	}

	client := vmctl.NewClient(vmctlSrv.URL)
	if _, err := client.ResolveDesktop("compute-stale-user", vmctl.PrimaryDesktopID); err != nil {
		t.Fatalf("precreate ownership: %v", err)
	}
	token := issueTestAccessJWT(priv, "compute-stale-user")
	req := httptest.NewRequest(http.MethodPost, "/api/compute/recovery", strings.NewReader(`{"action":"wake_current_computer"}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "choir_access", Value: token})
	w := httptest.NewRecorder()
	handler.HandleAPI(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("compute recovery = %d, want 200 body=%s", w.Code, w.Body.String())
	}
	if !refreshed.Load() {
		t.Fatal("wake recovery did not refresh unreachable current computer")
	}

	var result computeRecoveryResponse
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("decode recovery response: %v", err)
	}
	if result.Runtime == nil || !result.Runtime.Reachable {
		t.Fatalf("runtime after recovery = %+v, want reachable", result.Runtime)
	}
	if result.CurrentComputer.LookupStatus != "ok" || result.CurrentComputer.State != string(vmctl.VMStateActive) {
		t.Fatalf("current computer after recovery = %+v", result.CurrentComputer)
	}
}

func TestComputeRecoveryWakeReportsUnreachableRefreshFailure(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate ed25519 key: %v", err)
	}

	autoputerMux := http.NewServeMux()
	autoputerMux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "boot_pending"})
	})
	autoputerSrv := httptest.NewServer(autoputerMux)
	t.Cleanup(func() { autoputerSrv.Close() })

	var refreshCalled atomic.Bool
	writeOwnership := func(w http.ResponseWriter) {
		writeJSON(w, http.StatusOK, map[string]any{
			"vm_id":          "vm-refresh-fails",
			"user_id":        "compute-refresh-fails-user",
			"desktop_id":     vmctl.PrimaryDesktopID,
			"kind":           string(vmctl.VMKindInteractive),
			"warmness_class": "primary",
			"published":      true,
			"computer_url":   autoputerSrv.URL,
			"state":          string(vmctl.VMStateActive),
			"created_at":     "2026-06-15T10:00:00.000Z",
			"last_active_at": "2026-06-15T10:01:00.000Z",
			"epoch":          8,
		})
	}

	vmctlMux := http.NewServeMux()
	vmctlMux.HandleFunc("/internal/vmctl/lookup", func(w http.ResponseWriter, r *http.Request) {
		writeOwnership(w)
	})
	vmctlMux.HandleFunc("/internal/vmctl/refresh", func(w http.ResponseWriter, r *http.Request) {
		refreshCalled.Store(true)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "refresh failed"})
	})
	vmctlMux.HandleFunc("/internal/vmctl/list", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"ownerships": []any{}})
	})
	vmctlSrv := httptest.NewServer(vmctlMux)
	t.Cleanup(func() { vmctlSrv.Close() })

	cfg := &Config{AllowDirectAutoputerForTests: true, Port: "0",
		ComputerURL:       autoputerSrv.URL,
		AuthPublicKeyPath: "/unused/in/test",
		VmctlURL:          vmctlSrv.URL}
	handler, err := NewHandler(cfg, pub)
	if err != nil {
		t.Fatalf("NewHandler with vmctl: %v", err)
	}

	token := issueTestAccessJWT(priv, "compute-refresh-fails-user")
	req := httptest.NewRequest(http.MethodPost, "/api/compute/recovery", strings.NewReader(`{"action":"wake_current_computer"}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "choir_access", Value: token})
	w := httptest.NewRecorder()
	handler.HandleAPI(w, req)
	if w.Code != http.StatusBadGateway {
		t.Fatalf("compute recovery = %d, want 502 body=%s", w.Code, w.Body.String())
	}
	if !refreshCalled.Load() {
		t.Fatal("expected recovery to attempt refresh")
	}
	statusReq := httptest.NewRequest(http.MethodGet, "/api/compute/status", nil)
	statusReq.AddCookie(&http.Cookie{Name: "choir_access", Value: token})
	statusW := httptest.NewRecorder()
	handler.HandleAPI(statusW, statusReq)
	if statusW.Code != http.StatusOK {
		t.Fatalf("compute status = %d, want 200 body=%s", statusW.Code, statusW.Body.String())
	}
	var status computeStatusResponse
	if err := json.NewDecoder(statusW.Body).Decode(&status); err != nil {
		t.Fatalf("decode compute status: %v", err)
	}
	if status.Recovery == nil || status.Recovery.Active || status.Recovery.Status != "failed" ||
		status.Recovery.Code != "refresh_failed" || status.Recovery.Message != "The retained computer could not be refreshed." {
		t.Fatalf("recovery status = %+v, want bounded failed refresh diagnostic", status.Recovery)
	}
	if strings.Contains(strings.ToLower(status.Recovery.Message), "vmctl") {
		t.Fatalf("recovery status leaked raw vmctl error: %+v", status.Recovery)
	}
	if status.Runtime == nil || status.Runtime.Reachable {
		t.Fatalf("runtime after failed refresh = %+v, want unreachable observation", status.Runtime)
	}
	if status.CurrentComputer.State != string(vmctl.VMStateActive) {
		t.Fatalf("current computer state = %s, want retained active observation", status.CurrentComputer.State)
	}
}

func TestRunComputeRecoveryRejectsStalePreRefreshHealth(t *testing.T) {
	oldAutoputer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}))
	t.Cleanup(oldAutoputer.Close)

	vmctlMux := http.NewServeMux()
	vmctlMux.HandleFunc("/internal/vmctl/lookup", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"vm_id": "vm-unready", "user_id": "owner-unready", "desktop_id": vmctl.PrimaryDesktopID,
			"kind": string(vmctl.VMKindInteractive), "warmness_class": "primary",
			"computer_url": oldAutoputer.URL, "state": string(vmctl.VMStateFailed), "epoch": 8,
		})
	})
	vmctlMux.HandleFunc("/internal/vmctl/refresh", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"vm_id": "vm-unready", "user_id": "owner-unready", "desktop_id": vmctl.PrimaryDesktopID,
			"kind": string(vmctl.VMKindInteractive), "warmness_class": "primary",
			"state": string(vmctl.VMStateActive), "epoch": 9,
		})
	})
	vmctlServer := httptest.NewServer(vmctlMux)
	t.Cleanup(vmctlServer.Close)

	handler, err := NewHandler(&Config{
		AllowDirectAutoputerForTests: true, Port: "0", ComputerURL: oldAutoputer.URL,
		AuthPublicKeyPath: "/unused/in/test", VmctlURL: vmctlServer.URL,
	}, nil)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}
	current, runtimeStatus, recoveryErr := handler.runComputeRecovery(context.Background(), "owner-unready", vmctl.PrimaryDesktopID, "")
	if recoveryErr == nil || !strings.Contains(recoveryErr.Error(), "guest health unavailable") {
		t.Fatalf("recovery error = %v, want guest health refusal", recoveryErr)
	}
	if current.State != string(vmctl.VMStateActive) || runtimeStatus != nil {
		t.Fatalf("recovery observation = current=%+v runtime=%+v, want active with no new-realization health", current, runtimeStatus)
	}
}

func TestComputeRecoveryDoesNotJoinDifferentStableComputerAuthority(t *testing.T) {
	tracker := newComputeRecoveryTracker()
	release := make(chan struct{})
	first := tracker.startOrJoin("owner", vmctl.PrimaryDesktopID, "computer-a", "wake_current_computer", func(context.Context) computeRecoveryRunResult {
		<-release
		return computeRecoveryRunResult{}
	})
	if first == nil {
		t.Fatal("first recovery was not admitted")
	}
	var wrongRun atomic.Bool
	second := tracker.startOrJoin("owner", vmctl.PrimaryDesktopID, "computer-b", "wake_current_computer", func(context.Context) computeRecoveryRunResult {
		wrongRun.Store(true)
		return computeRecoveryRunResult{}
	})
	if second != nil || wrongRun.Load() {
		t.Fatalf("different stable-computer authority joined or ran: second=%v ran=%v", second, wrongRun.Load())
	}
	close(release)
	select {
	case <-first.done:
	case <-time.After(10 * time.Second):
		t.Fatal("first recovery did not finish within 10s after release")
	}
}

func TestComputeRecoveryWaiterSnapshotsOriginalOperation(t *testing.T) {
	tracker := newComputeRecoveryTracker()
	first := tracker.startOrJoin("owner", vmctl.PrimaryDesktopID, "", "wake_current_computer", func(context.Context) computeRecoveryRunResult {
		return computeRecoveryRunResult{Err: errors.New("first refresh failed")}
	})
	select {
	case <-first.done:
	case <-time.After(10 * time.Second):
		t.Fatal("first recovery did not finish within 10s")
	}

	releaseSecond := make(chan struct{})
	second := tracker.startOrJoin("owner", vmctl.PrimaryDesktopID, "", "wake_current_computer", func(context.Context) computeRecoveryRunResult {
		<-releaseSecond
		return computeRecoveryRunResult{}
	})
	defer func() {
		close(releaseSecond)
		select {
		case <-second.done:
		case <-time.After(10 * time.Second):
			t.Error("second recovery did not finish within 10s after release")
		}
	}()

	recovery, _, _, recoveryErr, ok := tracker.snapshotOperation(first)
	if !ok || recoveryErr == nil || recovery == nil || recovery.Status != "failed" || recovery.Code != "refresh_failed" {
		t.Fatalf("first operation snapshot = %+v err=%v ok=%v, want original failure", recovery, recoveryErr, ok)
	}
	current, _, _, currentErr, ok := tracker.snapshot("owner", vmctl.PrimaryDesktopID)
	if !ok || currentErr != nil || current == nil || !current.Active || current.Status != "refreshing" {
		t.Fatalf("current operation snapshot = %+v err=%v ok=%v, want replacement in progress", current, currentErr, ok)
	}
}

func TestComputeRecoveryWakeRefreshesCurrentComputerWithoutBlockingResolve(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate ed25519 key: %v", err)
	}

	var refreshed atomic.Bool
	var resolveCalled atomic.Bool
	autoputerMux := http.NewServeMux()
	autoputerMux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if !refreshed.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "boot_pending"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "service": "autoputer"})
	})
	autoputerSrv := httptest.NewServer(autoputerMux)
	t.Cleanup(func() { autoputerSrv.Close() })

	reg := vmctl.NewOwnershipRegistry(autoputerSrv.URL)
	vmctlHandler := vmctl.NewHandler(reg)

	vmctlMux := http.NewServeMux()
	vmctlMux.HandleFunc("/internal/vmctl/resolve", func(w http.ResponseWriter, r *http.Request) {
		resolveCalled.Store(true)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "resolve should not be called"})
	})
	vmctlMux.HandleFunc("/internal/vmctl/lookup", vmctlHandler.HandleLookup)
	vmctlMux.HandleFunc("/internal/vmctl/list", vmctlHandler.HandleList)
	vmctlMux.HandleFunc("/internal/vmctl/refresh", func(w http.ResponseWriter, r *http.Request) {
		refreshed.Store(true)
		vmctlHandler.HandleRefresh(w, r)
	})
	vmctlMux.HandleFunc("/health", vmctlHandler.HandleHealth)
	vmctlSrv := httptest.NewServer(vmctlMux)
	t.Cleanup(func() { vmctlSrv.Close() })
	client := vmctl.NewClient(vmctlSrv.URL)

	if _, err := reg.ResolveOrAssignDesktop("compute-lookup-recovery-user", vmctl.PrimaryDesktopID); err != nil {
		t.Fatalf("precreate ownership: %v", err)
	}
	if own, err := client.LookupDesktop("compute-lookup-recovery-user", vmctl.PrimaryDesktopID); err != nil || own == nil {
		t.Fatalf("precreated ownership lookup = %+v, err=%v", own, err)
	}

	cfg := &Config{AllowDirectAutoputerForTests: true, Port: "0",
		ComputerURL:       autoputerSrv.URL,
		AuthPublicKeyPath: "/unused/in/test",
		VmctlURL:          vmctlSrv.URL}
	handler, err := NewHandler(cfg, pub)
	if err != nil {
		t.Fatalf("NewHandler with vmctl: %v", err)
	}

	token := issueTestAccessJWT(priv, "compute-lookup-recovery-user")
	req := httptest.NewRequest(http.MethodPost, "/api/compute/recovery", strings.NewReader(`{"action":"wake_current_computer"}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "choir_access", Value: token})
	w := httptest.NewRecorder()
	handler.HandleAPI(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("compute recovery = %d, want 200 body=%s", w.Code, w.Body.String())
	}
	if resolveCalled.Load() {
		t.Fatal("recovery called resolve before refreshing an existing current computer")
	}
	if !refreshed.Load() {
		t.Fatal("wake recovery did not refresh from lookup-only current computer")
	}

	var result computeRecoveryResponse
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("decode recovery response: %v", err)
	}
	if result.Runtime == nil || !result.Runtime.Reachable {
		t.Fatalf("runtime after lookup-first recovery = %+v, want reachable", result.Runtime)
	}
	if result.CurrentComputer.State != string(vmctl.VMStateActive) {
		t.Fatalf("current computer state = %s, want active", result.CurrentComputer.State)
	}
}

func TestComputeRecoveryWakeRefreshesStoppedCurrentComputerWhenResolveFails(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate ed25519 key: %v", err)
	}

	autoputerMux := http.NewServeMux()
	autoputerMux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "service": "autoputer"})
	})
	autoputerSrv := httptest.NewServer(autoputerMux)
	t.Cleanup(func() { autoputerSrv.Close() })

	var resolveCalled atomic.Bool
	var refreshCalled atomic.Bool
	vmctlMux := http.NewServeMux()
	vmctlMux.HandleFunc("/internal/vmctl/lookup", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"vm_id":          "vm-stopped-current",
			"user_id":        "compute-stopped-recovery-user",
			"desktop_id":     vmctl.PrimaryDesktopID,
			"kind":           string(vmctl.VMKindInteractive),
			"warmness_class": "primary",
			"published":      true,
			"computer_url":   "http://10.203.109.2:8085",
			"state":          string(vmctl.VMStateStopped),
			"created_at":     "2026-06-09T20:00:00.000Z",
			"last_active_at": "2026-06-09T20:30:00.000Z",
			"epoch":          159,
			"stopped_by":     "vmctl-restart",
		})
	})
	vmctlMux.HandleFunc("/internal/vmctl/resolve", func(w http.ResponseWriter, r *http.Request) {
		resolveCalled.Store(true)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "stale resume failed"})
	})
	vmctlMux.HandleFunc("/internal/vmctl/refresh", func(w http.ResponseWriter, r *http.Request) {
		refreshCalled.Store(true)
		writeJSON(w, http.StatusOK, map[string]any{
			"vm_id":          "vm-stopped-current",
			"user_id":        "compute-stopped-recovery-user",
			"desktop_id":     vmctl.PrimaryDesktopID,
			"kind":           string(vmctl.VMKindInteractive),
			"warmness_class": "primary",
			"published":      true,
			"computer_url":   autoputerSrv.URL,
			"state":          string(vmctl.VMStateActive),
		})
	})
	vmctlMux.HandleFunc("/internal/vmctl/list", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"ownerships": []any{}})
	})
	vmctlSrv := httptest.NewServer(vmctlMux)
	t.Cleanup(func() { vmctlSrv.Close() })

	cfg := &Config{AllowDirectAutoputerForTests: true, Port: "0",
		ComputerURL:       autoputerSrv.URL,
		AuthPublicKeyPath: "/unused/in/test",
		VmctlURL:          vmctlSrv.URL}
	handler, err := NewHandler(cfg, pub)
	if err != nil {
		t.Fatalf("NewHandler with vmctl: %v", err)
	}

	token := issueTestAccessJWT(priv, "compute-stopped-recovery-user")
	req := httptest.NewRequest(http.MethodPost, "/api/compute/recovery", strings.NewReader(`{"action":"wake_current_computer"}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "choir_access", Value: token})
	w := httptest.NewRecorder()
	handler.HandleAPI(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("compute recovery = %d, want 200 body=%s", w.Code, w.Body.String())
	}
	if !resolveCalled.Load() {
		t.Fatal("expected recovery to try normal wake before refresh fallback")
	}
	if !refreshCalled.Load() {
		t.Fatal("expected recovery to refresh stopped current computer after wake failed")
	}

	var result computeRecoveryResponse
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("decode recovery response: %v", err)
	}
	if result.CurrentComputer.State != string(vmctl.VMStateActive) {
		t.Fatalf("current computer state = %s, want active", result.CurrentComputer.State)
	}
	if result.Runtime == nil || !result.Runtime.Reachable {
		t.Fatalf("runtime after stopped fallback refresh = %+v, want reachable", result.Runtime)
	}
}

func TestComputeRecoveryContinuesAfterClientCancelAndStatusBootstrapObserveReady(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate ed25519 key: %v", err)
	}

	var refreshed atomic.Bool
	autoputerMux := http.NewServeMux()
	autoputerMux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if !refreshed.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "boot_pending"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "service": "autoputer"})
	})
	autoputerMux.HandleFunc("/api/shell/bootstrap", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if !refreshed.Load() {
			w.WriteHeader(http.StatusBadGateway)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "route not ready"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"computer_id": "vm-cancel-recovered"})
	})
	autoputerSrv := httptest.NewServer(autoputerMux)
	t.Cleanup(func() { autoputerSrv.Close() })

	refreshStarted := make(chan struct{})
	releaseRefresh := make(chan struct{})
	refreshDone := make(chan struct{})
	var refreshStartedClosed atomic.Bool
	var refreshContextCanceled atomic.Bool

	writeOwnership := func(w http.ResponseWriter, state string) {
		writeJSON(w, http.StatusOK, map[string]any{
			"vm_id":          "vm-cancel-recovered",
			"user_id":        "compute-cancel-recovery-user",
			"desktop_id":     vmctl.PrimaryDesktopID,
			"kind":           string(vmctl.VMKindInteractive),
			"warmness_class": "primary",
			"published":      true,
			"computer_url":   autoputerSrv.URL,
			"state":          state,
			"created_at":     "2026-06-15T10:00:00.000Z",
			"last_active_at": "2026-06-15T10:01:00.000Z",
			"epoch":          7,
		})
	}

	vmctlMux := http.NewServeMux()
	vmctlMux.HandleFunc("/internal/vmctl/lookup", func(w http.ResponseWriter, r *http.Request) {
		writeOwnership(w, string(vmctl.VMStateActive))
	})
	vmctlMux.HandleFunc("/internal/vmctl/resolve", func(w http.ResponseWriter, r *http.Request) {
		writeOwnership(w, string(vmctl.VMStateActive))
	})
	vmctlMux.HandleFunc("/internal/vmctl/refresh", func(w http.ResponseWriter, r *http.Request) {
		if refreshStartedClosed.CompareAndSwap(false, true) {
			close(refreshStarted)
		}
		defer close(refreshDone)
		select {
		case <-r.Context().Done():
			refreshContextCanceled.Store(true)
			return
		case <-releaseRefresh:
		}
		refreshed.Store(true)
		writeOwnership(w, string(vmctl.VMStateActive))
	})
	vmctlMux.HandleFunc("/internal/vmctl/list", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"ownerships": []any{}})
	})
	vmctlMux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "service": "vmctl"})
	})
	vmctlSrv := httptest.NewServer(vmctlMux)
	t.Cleanup(func() { vmctlSrv.Close() })

	cfg := &Config{AllowDirectAutoputerForTests: true, Port: "0",
		ComputerURL:       autoputerSrv.URL,
		AuthPublicKeyPath: "/unused/in/test",
		VmctlURL:          vmctlSrv.URL}
	handler, err := NewHandler(cfg, pub)
	if err != nil {
		t.Fatalf("NewHandler with vmctl: %v", err)
	}

	token := issueTestAccessJWT(priv, "compute-cancel-recovery-user")
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodPost, "/api/compute/recovery", strings.NewReader(`{"action":"wake_current_computer"}`)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "choir_access", Value: token})
	w := httptest.NewRecorder()
	handlerDone := make(chan struct{})
	go func() {
		handler.HandleAPI(w, req)
		close(handlerDone)
	}()

	select {
	case <-refreshStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("recovery did not reach vmctl refresh")
	}
	cancel()
	select {
	case <-handlerDone:
	case <-time.After(2 * time.Second):
		t.Fatal("canceled recovery request did not return")
	}

	statusReq := httptest.NewRequest(http.MethodGet, "/api/compute/status", nil)
	statusReq.AddCookie(&http.Cookie{Name: "choir_access", Value: token})
	statusW := httptest.NewRecorder()
	handler.HandleAPI(statusW, statusReq)
	if statusW.Code != http.StatusOK {
		t.Fatalf("compute status during recovery = %d, want 200 body=%s", statusW.Code, statusW.Body.String())
	}
	var statusDuring computeStatusResponse
	if err := json.NewDecoder(statusW.Body).Decode(&statusDuring); err != nil {
		t.Fatalf("decode status during recovery: %v", err)
	}
	if statusDuring.Recovery == nil || !statusDuring.Recovery.Active || statusDuring.Recovery.Status != "refreshing" {
		t.Fatalf("status during recovery = %+v, want active refreshing", statusDuring.Recovery)
	}

	select {
	case <-refreshDone:
		t.Fatal("vmctl refresh ended before release; likely inherited the canceled browser context")
	case <-time.After(50 * time.Millisecond):
	}
	if refreshContextCanceled.Load() {
		t.Fatal("vmctl refresh inherited canceled browser request context")
	}
	close(releaseRefresh)
	select {
	case <-refreshDone:
	case <-time.After(2 * time.Second):
		t.Fatal("detached vmctl refresh did not complete")
	}

	readyDeadline := time.Now().Add(2 * time.Second)
	var statusAfter computeStatusResponse
	for time.Now().Before(readyDeadline) {
		statusReq := httptest.NewRequest(http.MethodGet, "/api/compute/status", nil)
		statusReq.AddCookie(&http.Cookie{Name: "choir_access", Value: token})
		statusW := httptest.NewRecorder()
		handler.HandleAPI(statusW, statusReq)
		if statusW.Code != http.StatusOK {
			time.Sleep(10 * time.Millisecond)
			continue
		}
		if err := json.NewDecoder(statusW.Body).Decode(&statusAfter); err != nil {
			time.Sleep(10 * time.Millisecond)
			continue
		}
		if statusAfter.Recovery != nil &&
			!statusAfter.Recovery.Active &&
			statusAfter.Recovery.Status == "ready" &&
			statusAfter.CurrentComputer.State == string(vmctl.VMStateActive) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if statusAfter.Recovery == nil ||
		statusAfter.Recovery.Active ||
		statusAfter.Recovery.Status != "ready" ||
		statusAfter.CurrentComputer.State != string(vmctl.VMStateActive) {
		t.Fatalf("status after recovery = recovery:%+v current:%+v, want ready active", statusAfter.Recovery, statusAfter.CurrentComputer)
	}

	bootstrapReq := httptest.NewRequest(http.MethodGet, "/api/shell/bootstrap", nil)
	bootstrapReq.AddCookie(&http.Cookie{Name: "choir_access", Value: token})
	bootstrapW := httptest.NewRecorder()
	handler.HandleBootstrap(bootstrapW, bootstrapReq)
	if bootstrapW.Code != http.StatusOK {
		t.Fatalf("bootstrap after detached recovery = %d, want 200 body=%s", bootstrapW.Code, bootstrapW.Body.String())
	}
	var bootstrap map[string]string
	if err := json.NewDecoder(bootstrapW.Body).Decode(&bootstrap); err != nil {
		t.Fatalf("decode bootstrap after detached recovery: %v", err)
	}
	if bootstrap["computer_id"] != "vm-cancel-recovered" {
		t.Fatalf("bootstrap computer_id = %q, want vm-cancel-recovered", bootstrap["computer_id"])
	}
}

func TestProxyHealthReportsRedactedLifecycleAggregates(t *testing.T) {
	handler, priv, _, _ := testVMctlProxyEnv(t)
	token := issueTestAccessJWT(priv, "lifecycle-user")

	bootstrapReq := httptest.NewRequest(http.MethodGet, "/api/shell/bootstrap", nil)
	bootstrapReq.AddCookie(&http.Cookie{Name: "choir_access", Value: token})
	bootstrapW := httptest.NewRecorder()
	handler.HandleBootstrap(bootstrapW, bootstrapReq)
	if bootstrapW.Code != http.StatusOK {
		t.Fatalf("bootstrap status = %d, want 200 body=%s", bootstrapW.Code, bootstrapW.Body.String())
	}

	healthReq := httptest.NewRequest(http.MethodGet, "/health", nil)
	healthW := httptest.NewRecorder()
	handler.HandleHealth(healthW, healthReq)
	if healthW.Code != http.StatusOK {
		t.Fatalf("health status = %d, want 200", healthW.Code)
	}
	var result proxyHealthResponse
	if err := json.NewDecoder(healthW.Body).Decode(&result); err != nil {
		t.Fatalf("decode health: %v", err)
	}
	if len(result.Lifecycle.Stages) == 0 {
		t.Fatalf("expected lifecycle stages in health")
	}
	var sawResolve, sawTotal bool
	for _, stage := range result.Lifecycle.Stages {
		if strings.Contains(stage.Stage, "lifecycle-user") {
			t.Fatalf("lifecycle stage leaked user id: %+v", stage)
		}
		if stage.Stage == "bootstrap.resolve" && stage.Count > 0 {
			sawResolve = true
		}
		if stage.Stage == "bootstrap.total" && stage.ByStatus["http_200"] > 0 {
			sawTotal = true
		}
	}
	if !sawResolve || !sawTotal {
		t.Fatalf("lifecycle stages = %+v, want bootstrap.resolve and bootstrap.total http_200", result.Lifecycle.Stages)
	}
}

// TestVMctlRouting_GracefulDegradation tests that when vmctl is unreachable,
// the proxy falls back to the static autoputer URL.
func TestVMctlRouting_GracefulDegradation(t *testing.T) {
	oldWindow := autoputerResolveRetryWindow
	oldBaseDelay := autoputerResolveRetryBaseDelay
	oldMaxDelay := autoputerResolveRetryMaxDelay
	autoputerResolveRetryWindow = 100 * time.Millisecond
	autoputerResolveRetryBaseDelay = time.Millisecond
	autoputerResolveRetryMaxDelay = 5 * time.Millisecond
	t.Cleanup(func() {
		autoputerResolveRetryWindow = oldWindow
		autoputerResolveRetryBaseDelay = oldBaseDelay
		autoputerResolveRetryMaxDelay = oldMaxDelay
	})

	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	// Create a autoputer backend.
	autoputerMux := http.NewServeMux()
	autoputerMux.HandleFunc("/api/shell/bootstrap", func(w http.ResponseWriter, r *http.Request) {
		user := r.Header.Get("X-Authenticated-User")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"computer_id": "autoputer-fallback",
			"user":        user,
		})
	})
	autoputerMux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	autoputerServer := httptest.NewServer(autoputerMux)
	t.Cleanup(func() { autoputerServer.Close() })

	// Create proxy pointing at an unreachable vmctl.
	cfg := &Config{AllowDirectAutoputerForTests: true, Port: "0",
		ComputerURL:       autoputerServer.URL,
		AuthPublicKeyPath: "/unused",
		VmctlURL:          "http://127.0.0.1:1", // unreachable port
	}

	handler, err := NewHandler(cfg, pub)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	accessToken := issueTestAccessJWT(priv, "user-fallback")

	req := httptest.NewRequest(http.MethodGet, "/api/shell/bootstrap", nil)
	req.Header.Set("Cookie", "choir_access="+accessToken)
	w := httptest.NewRecorder()
	handler.HandleBootstrap(w, req)

	// Multi-desktop routing must fail closed. Falling back to a different autoputer
	// would land the user on the wrong desktop.
	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 when vmctl is unavailable, got %d", w.Code)
	}
}

// --- Bearer Token (API Key) Auth Tests ---

// testProxyEnvWithAuthStore sets up a proxy Handler with a real backend autoputer
// and an auth store for API key validation. It returns the handler, signing
// key, autoputer server, and the auth store.
func testProxyEnvWithAuthStore(t *testing.T) (*Handler, ed25519.PrivateKey, *httptest.Server, *auth.Store) {
	t.Helper()

	handler, priv, autoputer, _ := testVMctlProxyEnv(t)

	// Create an auth store and wire it into the handler for API key validation.
	dbDir := t.TempDir()
	store, err := auth.OpenStore(filepath.Join(dbDir, "test-auth.db"))
	if err != nil {
		t.Fatalf("open auth store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	handler.SetAPIKeyValidator(store)
	return handler, priv, autoputer, store
}

// createTestAPIKey creates a user and an API key in the auth store, returning
// the raw secret and the user.
func createTestAPIKey(t *testing.T, handler *Handler, store *auth.Store, label string, scopes []string, expiresAt *time.Time) (*auth.User, string) {
	t.Helper()
	ctx := context.Background()
	user, err := store.CreateUser("proxy-test-user-"+label, label+"@example.com")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	computer, err := handler.vmctlClient.ResolveDesktopContext(ctx, user.ID, "primary")
	if err != nil {
		t.Fatalf("resolve owned computer: %v", err)
	}
	_, secret, err := store.CreateComputerScopedAPIKey(ctx, user.ID, label, scopes, computer.ComputerID, expiresAt)
	if err != nil {
		t.Fatalf("create api key: %v", err)
	}
	return user, secret
}

// hashSecretForTest computes the SHA-256 hex hash of an API key secret.
func hashSecretForTest(secret string) string {
	h := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(h[:])
}

func TestBearerTokenAuthAcceptsValidAPIKey(t *testing.T) {
	handler, _, _, store := testProxyEnvWithAuthStore(t)

	user, secret := createTestAPIKey(t, handler, store, "valid-key", []string{"read:runtime"}, nil)

	// Make a request with the Bearer token to the bootstrap endpoint.
	req := httptest.NewRequest(http.MethodGet, "/api/shell/bootstrap", nil)
	req.Header.Set("Authorization", "Bearer "+secret)
	rec := httptest.NewRecorder()
	handler.HandleBootstrap(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	// Verify the autoputer received the authenticated user header.
	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got := resp["user"]; got != user.ID {
		t.Errorf("autoputer user: got %v, want %q", got, user.ID)
	}

	// Verify scopes were injected as a header to the upstream.
	// The autoputer test backend doesn't echo scopes, but we can verify the
	// auth result had scopes by checking last_used_at was updated.
	ctx := context.Background()
	ak, err := store.GetAPIKeyByHash(ctx, hashSecretForTest(secret))
	if err != nil {
		t.Fatalf("get api key: %v", err)
	}
	if ak.LastUsedAt == nil {
		t.Error("last_used_at should be updated after successful validation")
	}
}

func TestBearerTokenAuthRejectsInvalidToken(t *testing.T) {
	handler, _, _, _ := testProxyEnvWithAuthStore(t)

	req := httptest.NewRequest(http.MethodGet, "/api/shell/bootstrap", nil)
	req.Header.Set("Authorization", "Bearer choir_sk_bogustoken123")
	rec := httptest.NewRecorder()
	handler.HandleBootstrap(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestBearerTokenAuthRejectsRevokedKey(t *testing.T) {
	handler, _, _, store := testProxyEnvWithAuthStore(t)

	user, secret := createTestAPIKey(t, handler, store, "revoked-key", []string{"read:base"}, nil)

	// Revoke the key.
	ctx := context.Background()
	// Look up the key ID via hash.
	ak, err := store.GetAPIKeyByHash(ctx, hashSecretForTest(secret))
	if err != nil {
		t.Fatalf("get api key: %v", err)
	}
	if err := store.RevokeAPIKey(ctx, user.ID, ak.ID); err != nil {
		t.Fatalf("revoke: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/shell/bootstrap", nil)
	req.Header.Set("Authorization", "Bearer "+secret)
	rec := httptest.NewRecorder()
	handler.HandleBootstrap(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want %d (revoked key)", rec.Code, http.StatusUnauthorized)
	}
}

func TestBearerTokenAuthRejectsExpiredKey(t *testing.T) {
	handler, _, _, store := testProxyEnvWithAuthStore(t)

	// Create a key that expired 1 hour ago.
	past := time.Now().Add(-1 * time.Hour)
	// We can't create an expired key via CreateAPIKey (it rejects past expiry
	// at the handler level, but the store itself doesn't check). Use the store
	// directly.
	ctx := context.Background()
	user, err := store.CreateUser("expired-user", "expired@example.com")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	_, secret, err := store.CreateAPIKey(ctx, user.ID, "expired-key", []string{"read:base"}, &past)
	if err != nil {
		t.Fatalf("create expired api key: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/shell/bootstrap", nil)
	req.Header.Set("Authorization", "Bearer "+secret)
	rec := httptest.NewRecorder()
	handler.HandleBootstrap(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want %d (expired key)", rec.Code, http.StatusUnauthorized)
	}
}

func TestCookieAuthPreferredOverBearerToken(t *testing.T) {
	handler, priv, _, store := testProxyEnvWithAuthStore(t)

	// Create an API key for a different user.
	_, secret := createTestAPIKey(t, handler, store, "other-key", []string{"read:base"}, nil)

	// Make a request with BOTH a valid cookie and a valid Bearer token.
	// Cookie auth should win (it's tried first).
	req := httptest.NewRequest(http.MethodGet, "/api/shell/bootstrap", nil)
	req.AddCookie(&http.Cookie{Name: "choir_access", Value: issueTestAccessJWT(priv, "cookie-priority-user")})
	req.Header.Set("Authorization", "Bearer "+secret)
	rec := httptest.NewRecorder()
	handler.HandleBootstrap(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got := resp["user"]; got != "cookie-priority-user" {
		t.Errorf("user: got %v, want %q (cookie should take priority)", got, "cookie-priority-user")
	}
}

func TestBearerTokenAuthRejectsMissingScope_whenProtectedAPIRouteRequiresRuntimeWrite(t *testing.T) {
	handler, _, autoputer, store := testProxyEnvWithAuthStore(t)

	var upstreamReached bool
	autoputerMux := http.NewServeMux()
	autoputerMux.HandleFunc("/api/test", func(w http.ResponseWriter, r *http.Request) {
		upstreamReached = true
		w.WriteHeader(http.StatusNoContent)
	})
	autoputerMux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	autoputer.Config.Handler = autoputerMux

	_, secret := createTestAPIKey(t, handler, store, "runtime-read-only", []string{"read:runtime"}, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/test", nil)
	req.Header.Set("Authorization", "Bearer "+secret)
	rec := httptest.NewRecorder()
	handler.HandleProtectedAPI(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status: got %d, want %d; body: %s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
	if upstreamReached {
		t.Fatal("upstream reached with API key missing required write:runtime scope")
	}
}

func TestBearerTokenAuthRejectsMissingScope_whenComputeRecoveryRequiresRuntimeWrite(t *testing.T) {
	handler, _, _, store := testProxyEnvWithAuthStore(t)

	_, secret := createTestAPIKey(t, handler, store, "compute-read-only", []string{"read:runtime"}, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/compute/recovery", strings.NewReader(`{"action":"wake_current_computer"}`))
	req.Header.Set("Authorization", "Bearer "+secret)
	rec := httptest.NewRecorder()
	handler.HandleComputeRecovery(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status: got %d, want %d; body: %s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
}

func TestBearerTokenAuthAcceptsBaseReadScope_whenProtectedAPIRouteIsBaseRead(t *testing.T) {
	handler, _, autoputer, store := testProxyEnvWithAuthStore(t)

	autoputerMux := http.NewServeMux()
	autoputerMux.HandleFunc("/api/base/delta", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"user":   r.Header.Get("X-Authenticated-User"),
			"scopes": r.Header.Get("X-Authenticated-Scopes"),
		})
	})
	autoputerMux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	autoputer.Config.Handler = autoputerMux

	user, secret := createTestAPIKey(t, handler, store, "base-read", []string{"read:base"}, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/base/delta?cursor=0", nil)
	req.Header.Set("Authorization", "Bearer "+secret)
	rec := httptest.NewRecorder()
	handler.HandleProtectedAPI(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got := resp["user"]; got != user.ID {
		t.Errorf("user: got %v, want %q", got, user.ID)
	}
	if got := resp["scopes"]; got != "read:base" {
		t.Errorf("scopes: got %v, want %q", got, "read:base")
	}
}

func TestBearerTokenAuthWithoutValidatorSkipsAPIKey(t *testing.T) {
	// When no API key validator is configured, Bearer tokens should be
	// rejected (only cookie auth works). This verifies the fallback path
	// doesn't accidentally accept unknown tokens.
	handler, _, _ := testProxyEnv(t)

	req := httptest.NewRequest(http.MethodGet, "/api/shell/bootstrap", nil)
	req.Header.Set("Authorization", "Bearer choir_sk_sometoken")
	rec := httptest.NewRecorder()
	handler.HandleBootstrap(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want %d (no validator configured)", rec.Code, http.StatusUnauthorized)
	}
}

func TestBearerTokenAuthStripsClientSuppliedScopes(t *testing.T) {
	handler, _, autoputer, store := testProxyEnvWithAuthStore(t)

	// Use a autoputer that echoes the scopes header.
	autoputerMux := http.NewServeMux()
	autoputerMux.HandleFunc("/api/shell/bootstrap", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"scopes": r.Header.Get("X-Authenticated-Scopes"),
		})
	})
	autoputerMux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	autoputer.Config.Handler = autoputerMux

	scopes := []string{"read:runtime"}
	_, secret := createTestAPIKey(t, handler, store, "strip-key", scopes, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/shell/bootstrap", nil)
	req.Header.Set("Authorization", "Bearer "+secret)
	// Client tries to inject fake scopes — should be stripped.
	req.Header.Set("X-Authenticated-Scopes", "admin,write:runtime")
	rec := httptest.NewRecorder()
	handler.HandleBootstrap(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	// Should be the key's scopes, not the client-supplied ones.
	if got := resp["scopes"]; got != "read:runtime" {
		t.Errorf("scopes: got %v, want %q (client-supplied scopes should be stripped)", got, "read:runtime")
	}
}

func TestProtectedAPIReverseProxyPreservesEscapedPathForBackendValidation(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	var gotEscapedPath, gotRawQuery string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotEscapedPath = r.URL.EscapedPath()
		gotRawQuery = r.URL.RawQuery
		if r.Header.Get(proxyOriginalEscapedPathHeader) != "" || r.Header.Get(proxyOriginalRawQueryHeader) != "" || r.Header.Get("X-Original-Path") != "" {
			t.Error("internal original-target header leaked upstream")
		}
		switch {
		case gotEscapedPath == "/api/trajectories/trajectory-one/capsule-evidence/assignment-one" && gotRawQuery == "attempt=1":
			w.WriteHeader(http.StatusNoContent)
		case gotEscapedPath == "/api/ordinary%20route" && gotRawQuery == "value=one%20two":
			w.WriteHeader(http.StatusAccepted)
		default:
			http.Error(w, "noncanonical capsule evidence route", http.StatusBadRequest)
		}
	}))
	defer upstream.Close()

	h, err := NewHandler(&Config{AllowDirectAutoputerForTests: true, ComputerURL: upstream.URL, AuthPublicKeyPath: "/unused"}, pub)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}
	token := issueTestAccessJWT(priv, "escaped-path-owner")

	tests := []struct {
		name        string
		target      string
		spoofedPath string
		wantPath    string
		wantStatus  int
	}{
		{
			name:       "canonical route remains canonical",
			target:     "/api/trajectories/trajectory-one/capsule-evidence/assignment-one?attempt=1",
			wantPath:   "/api/trajectories/trajectory-one/capsule-evidence/assignment-one",
			wantStatus: http.StatusNoContent,
		},
		{
			name:        "encoded unreserved assignment remains visible and client header cannot normalize it",
			target:      "/api/trajectories/trajectory-one/capsule-evidence/%61ssignment-one?attempt=1",
			spoofedPath: "/api/trajectories/trajectory-one/capsule-evidence/assignment-one",
			wantPath:    "/api/trajectories/trajectory-one/capsule-evidence/%61ssignment-one",
			wantStatus:  http.StatusBadRequest,
		},
		{
			name:       "encoded slash fails closed",
			target:     "/api/trajectories/trajectory-one/capsule-evidence/assignment%2Fone?attempt=1",
			wantPath:   "/api/trajectories/trajectory-one/capsule-evidence/assignment%2Fone",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "encoded backslash fails closed",
			target:     "/api/trajectories/trajectory-one/capsule-evidence/assignment%5Cone?attempt=1",
			wantPath:   "/api/trajectories/trajectory-one/capsule-evidence/assignment%5Cone",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "encoded dot fails closed",
			target:     "/api/trajectories/trajectory-one/capsule-evidence/%2e?attempt=1",
			wantPath:   "/api/trajectories/trajectory-one/capsule-evidence/%2e",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "unrelated route keeps prefix and query",
			target:     "/api/ordinary%20route?value=one%20two",
			wantPath:   "/api/ordinary%20route",
			wantStatus: http.StatusAccepted,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotEscapedPath, gotRawQuery = "", ""
			req := httptest.NewRequest(http.MethodGet, tt.target, nil)
			req.AddCookie(&http.Cookie{Name: "choir_access", Value: token})
			if tt.spoofedPath != "" {
				req.Header.Set(proxyOriginalEscapedPathHeader, tt.spoofedPath)
			}
			rec := httptest.NewRecorder()
			h.HandleProtectedAPI(rec, req)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status=%d want=%d body=%s", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if gotEscapedPath != tt.wantPath {
				t.Fatalf("upstream EscapedPath=%q want=%q", gotEscapedPath, tt.wantPath)
			}
		})
	}
}

func TestProtectedAPIRejectsMalformedRawPathBeforeUpstream(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	upstreamReached := false
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamReached = true
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()
	h, err := NewHandler(&Config{AllowDirectAutoputerForTests: true, ComputerURL: upstream.URL, AuthPublicKeyPath: "/unused"}, pub)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/trajectories/t/capsule-evidence/a?attempt=1", nil)
	req.URL.RawPath = "/api/trajectories/t/capsule-evidence/%zz"
	req.AddCookie(&http.Cookie{Name: "choir_access", Value: issueTestAccessJWT(priv, "malformed-path-owner")})
	rec := httptest.NewRecorder()
	h.HandleProtectedAPI(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want=%d body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if upstreamReached {
		t.Fatal("malformed RawPath reached upstream")
	}
}
