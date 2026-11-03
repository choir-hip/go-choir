package proxy

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/owneract"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

// The owner's verifier re-pin at the proxy
// (docs/problems/verifier-key-regenerated-locks-out-checkpoints-2026-11-02.md).
// Ways this could fail, written first:
//   - an agent key (admin included) without owner:acts reaches corpusd;
//   - the proxy pins a key corpusd's preflight did not see as the one the
//     guest presents, or a key the caller named in the body;
//   - the receipt does not bind the key, the replaced key and the reason;
//   - a computer whose key matches, or that has no pin, is re-pinned;
//   - the guest is not told, so its hold and /health stay stale.

type repinProxyEnv struct {
	handler *Handler
	user    string
	secret  func(t *testing.T, scopes ...string) string
	mu      sync.Mutex
	calls   []string
	receipt string
	repin   selfdevprotocol.VerifierKeyRepinRequest
	state   string
	// guestComputer is the computer_id the guest answers ("" is computer-a);
	// guestStatus a status for its key read (0 is 200).
	guestComputer string
	guestStatus   int
}

func (e *repinProxyEnv) record(call string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.calls = append(e.calls, call)
}

func (e *repinProxyEnv) joined() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return strings.Join(e.calls, "\n")
}

func newRepinProxyEnv(t *testing.T, presented, pinned string, presentedPublic string) *repinProxyEnv {
	t.Helper()
	env := &repinProxyEnv{state: selfdevprotocol.VerifierKeyMismatch}
	guest := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		env.record("guest " + r.Method + " " + r.URL.Path + " owner=" + r.Header.Get("X-Authenticated-User") + " " + string(body))
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/repinned") {
			_, _ = w.Write([]byte(`{"computer_id":"computer-a","event_digest":"e"}`))
			return
		}
		if env.guestStatus != 0 {
			w.WriteHeader(env.guestStatus)
			_, _ = w.Write([]byte(`{"error":"not found"}`))
			return
		}
		answered := "computer-a"
		if env.guestComputer != "" {
			answered = env.guestComputer
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"computer_id": answered, "key_id": presented, "public_key": presentedPublic})
	}))
	t.Cleanup(guest.Close)
	corpusd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		env.record("corpusd " + r.Method + " " + r.URL.Path + " internal=" + r.Header.Get("X-Internal-Caller"))
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/internal/computers/verifier-key/preflight":
			var request selfdevprotocol.VerifierKeyPreflightRequest
			_ = json.Unmarshal(body, &request)
			_ = json.NewEncoder(w).Encode(selfdevprotocol.VerifierKeyPin{ComputerID: request.ComputerID, State: env.state, PresentedKeyID: request.KeyID, PinnedKeyID: pinned})
		case "/internal/computers/verifier-key/repin":
			env.mu.Lock()
			env.receipt = r.Header.Get(owneract.Header)
			_ = json.Unmarshal(body, &env.repin)
			env.mu.Unlock()
			_ = json.NewEncoder(w).Encode(selfdevprotocol.VerifierKeyRepinResponse{Status: selfdevprotocol.VerifierKeyRepinned,
				Pin: selfdevprotocol.VerifierKeyPin{ComputerID: "computer-a", State: selfdevprotocol.VerifierKeyMatch, PinnedKeyID: presented}})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(corpusd.Close)
	handler, _, _, store := testProxyEnvWithAuthStore(t)
	handler.cfg.CorpusdURL = corpusd.URL
	_, private, _ := ed25519.GenerateKey(rand.Reader)
	signer, err := owneract.NewSigner(private)
	if err != nil {
		t.Fatal(err)
	}
	handler.ownerActSigner = signer
	user, err := store.CreateUser("repin-user", "repin@example.com")
	if err != nil {
		t.Fatal(err)
	}
	env.user = user.ID
	ownership := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"computer_id": "computer-a", "desktop_id": "primary", "user_id": user.ID,
			"state": "active", "computer_url": guest.URL,
		})
	}))
	t.Cleanup(ownership.Close)
	handler.vmctlClient = vmctl.NewClient(ownership.URL)
	env.handler = handler
	var n int
	env.secret = func(t *testing.T, scopes ...string) string {
		t.Helper()
		n++
		_, secret, err := store.CreateComputerScopedAPIKey(context.Background(), user.ID, "repin-key-"+strings.Repeat("x", n), scopes, "computer-a", nil)
		if err != nil {
			t.Fatal(err)
		}
		return secret
	}
	return env
}

func (e *repinProxyEnv) send(t *testing.T, body, secret string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/api/computers/computer-a/verifier-key/repin", strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+secret)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	e.handler.HandleAPI(response, request)
	return response
}

func TestVerifierRepinIsTheOwnersActAndPinsTheKeyTheGuestPresents(t *testing.T) {
	public, _, _ := ed25519.GenerateKey(rand.Reader)
	presented := selfdevprotocol.VerifierKeyIDFor(public)
	presentedPublic := base64.RawStdEncoding.EncodeToString(public)
	const pinned = "verifier-control-22033e8312994d59"
	env := newRepinProxyEnv(t, presented, pinned, presentedPublic)
	body := `{"reason":"the lost-disk drills regenerated d@e's verifier key"}`

	for _, scopes := range [][]string{{"computer:lifecycle"}, {"admin"}} {
		if rec := env.send(t, body, env.secret(t, scopes...)); rec.Code != http.StatusForbidden || env.joined() != "" {
			t.Fatalf("%v re-pin = %d, calls %v; want 403 and no upstream call", scopes, rec.Code, env.joined())
		}
	}
	owner := env.secret(t, "computer:lifecycle", OwnerActsScope)
	if rec := env.send(t, `{"reason":"  "}`, owner); rec.Code != http.StatusBadRequest {
		t.Fatalf("a re-pin without a reason = %d", rec.Code)
	}
	if rec := env.send(t, `{"reason":"x","key_id":"verifier-control-chosen"}`, owner); rec.Code != http.StatusBadRequest {
		t.Fatalf("a re-pin naming its own key = %d", rec.Code)
	}

	rec := env.send(t, body, owner)
	if rec.Code != http.StatusOK {
		t.Fatalf("owner re-pin = %d %s (calls %v)", rec.Code, rec.Body.String(), env.joined())
	}
	calls := env.joined()
	for _, want := range []string{"guest GET /api/computers/computer-a/self-development/verifier-key", "corpusd POST /internal/computers/verifier-key/preflight internal=true",
		"corpusd POST /internal/computers/verifier-key/repin internal=true", "guest POST /api/computers/computer-a/self-development/verifier-key/repinned"} {
		if !strings.Contains(calls, want) {
			t.Fatalf("calls %v lack %q", calls, want)
		}
	}
	if env.repin.KeyID != presented || env.repin.PublicKey != presentedPublic || env.repin.ReplacesKeyID != pinned || env.repin.OwnerID != env.user || !strings.Contains(env.repin.Reason, "lost-disk") {
		t.Fatalf("corpusd was asked to pin %+v", env.repin)
	}
	decoded, err := owneract.Decode(env.receipt)
	if err != nil {
		t.Fatal(err)
	}
	verifier, _ := owneract.NewVerifier(env.handler.ownerActSigner.PublicKey())
	verified, err := verifier.Verify(decoded, "computer-a", owneract.ActVerifierRepin, time.Now())
	if err != nil {
		t.Fatalf("receipt: %v", err)
	}
	for key, want := range selfdevprotocol.VerifierKeyRepinSubject(env.repin) {
		if verified.SubjectString(key) != want {
			t.Fatalf("receipt subject %s = %q, want %q", key, verified.SubjectString(key), want)
		}
	}

	// A matching key, or no pin, is not re-pinned.
	for _, state := range []string{selfdevprotocol.VerifierKeyMatch, selfdevprotocol.VerifierKeyUnpinned} {
		env.mu.Lock()
		env.calls, env.state = nil, state
		env.mu.Unlock()
		rec := env.send(t, body, owner)
		if strings.Contains(env.joined(), "/verifier-key/repin ") || strings.Contains(env.joined(), "repin internal") {
			t.Fatalf("%s computer was re-pinned: %v", state, env.joined())
		}
		if state == selfdevprotocol.VerifierKeyMatch && (rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), selfdevprotocol.VerifierKeyAlreadyPinned)) {
			t.Fatalf("matching key = %d %s", rec.Code, rec.Body.String())
		}
		if state == selfdevprotocol.VerifierKeyUnpinned && rec.Code != http.StatusConflict {
			t.Fatalf("unpinned computer = %d %s", rec.Code, rec.Body.String())
		}
	}
}

// Review of 9a465a7f2: the guest's answer must name the computer (#4), and
// an older guest release with no verifier-key route gets the owner's way
// out, not a bare 404 (#5).
func TestVerifierRepinRefusesAnotherComputersAnswerAndExplainsAnOldRelease(t *testing.T) {
	public, _, _ := ed25519.GenerateKey(rand.Reader)
	env := newRepinProxyEnv(t, selfdevprotocol.VerifierKeyIDFor(public), "verifier-control-22033e8312994d59", base64.RawStdEncoding.EncodeToString(public))
	owner := env.secret(t, "computer:lifecycle", OwnerActsScope)
	body := `{"reason":"lost disk"}`
	env.guestComputer = "computer-b"
	if rec := env.send(t, body, owner); rec.Code != http.StatusBadGateway || strings.Contains(env.joined(), "corpusd") {
		t.Fatalf("another computer's answer = %d %s, calls %v", rec.Code, rec.Body.String(), env.joined())
	}
	env.guestComputer, env.guestStatus = "", http.StatusNotFound
	rec := env.send(t, body, owner)
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "predates the verifier re-pin") || !strings.Contains(rec.Body.String(), "self-dev abandon") || strings.Contains(env.joined(), "corpusd") {
		t.Fatalf("old release = %d %s, calls %v", rec.Code, rec.Body.String(), env.joined())
	}
}
