package gateway

import (
	"bytes"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/provider"
)

// SEC-05: a computer's gateway bearer is accepted only from that computer's
// own address (docs/problems/gateway-bearer-works-from-any-computer-2026-10-21.md).
// Ways the repair can fail, written before the code (numbering follows the
// problem record):
//  1. a bearer pinned to computer A is accepted from B's address, on any
//     route (TestBearerBindingRefusesAnotherComputersAddressOnEveryRoute);
//  2. A's own calls are refused: after the host moves the pin, after a
//     gateway restart (TestBearerBindingHostMovesThePin,
//     TestBearerBindingFirstUseSurvivesARestart);
//  3. the guest moves the pin: only EnsureCredentialForPeer does, and a call
//     never rewrites a pin (TestBearerBindingACallNeverMovesAPin);
//  4. observe mode refuses something, or writes a pin
//     (TestBearerBindingObserveCountsAndNeverRefuses);
//  5. strict mode lets an unpinned bearer through, or enforce does not bind
//     it (TestBearerBindingStrictRefusesUnpinned, ...FirstUseBinds...);
//  6. a failed pin write is accepted in enforce
//     (TestBearerBindingFailedPinWriteRefusesAndLeavesNoPin);
//  7. two first uses from different addresses both pass
//     (TestBearerBindingConcurrentFirstUseHasOneWinner);
//  8. loopback presents a guest's bearer and passes
//     (TestBearerBindingLoopbackCannotUseAGuestsBearer);
//  9. the mismatch log floods or names the token
//     (TestBearerBindingLogIsBoundedAndNeverNamesTheToken);
//  10. the comparison is not constant time (TestConstantTimeHashCompare);
//  11. the pin key changes and a rollback cannot read it
//     (TestPeerPinKeepsTheLegacyJSONKey);
//  12. a mode value widens a route: an unknown value must enforce
//     (TestBearerBindingModeParsing).

const (
	peerA = "10.200.0.2"
	peerB = "10.200.1.2"
)

func bindingRequest(rawToken, peer string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/provider/v1/inference", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer "+rawToken)
	req.RemoteAddr = peer + ":45678"
	return req
}

func inferenceBody() string {
	body, _ := json.Marshal(ProviderRequest{
		Messages: []provider.Message{{Role: "user", Content: []provider.Block{{Type: "text", Text: "hi"}}}},
	})
	return string(body)
}

// pinnedHandler is a handler in the given mode with computer-a issued and
// pinned to peerA by the host's ensure route.
func pinnedHandler(t *testing.T, mode BindingMode) (*Handler, *IdentityRegistry, string) {
	t.Helper()
	h, reg, _ := setupHandler(t)
	h.SetBearerBinding(mode)
	cred, err := reg.IssueCredential("computer-a")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := reg.EnsureCredentialForPeer(cred.RawToken, peerA); err != nil {
		t.Fatal(err)
	}
	return h, reg, cred.RawToken
}

func inferFrom(h *Handler, raw, peer string) int {
	req := httptest.NewRequest(http.MethodPost, "/provider/v1/inference", strings.NewReader(inferenceBody()))
	req.Header.Set("Authorization", "Bearer "+raw)
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = net.JoinHostPort(peer, "45678")
	w := httptest.NewRecorder()
	h.HandleInference(w, req)
	return w.Code
}

func TestBearerBindingRefusesAnotherComputersAddressOnEveryRoute(t *testing.T) {
	h, _, raw := pinnedHandler(t, BindingEnforce)
	routes := []struct {
		name    string
		method  string
		path    string
		handler http.HandlerFunc
	}{
		{"inference", http.MethodPost, "/provider/v1/inference", h.HandleInference},
		{"openai chat", http.MethodPost, "/provider/openai/v1/chat/completions", h.HandleOpenAIChatCompletions},
		{"openai models", http.MethodGet, "/provider/openai/v1/models", h.HandleOpenAIModels},
		{"models", http.MethodGet, "/provider/v1/models", h.HandleModels},
		{"search", http.MethodPost, "/provider/v1/search", h.HandleSearch},
		{"source api", http.MethodPost, "/provider/v1/source-api", h.HandleSourceAPI},
	}
	for _, rt := range routes {
		t.Run(rt.name, func(t *testing.T) {
			call := func(peer string) (int, string) {
				req := httptest.NewRequest(rt.method, rt.path, strings.NewReader(`{}`))
				req.Header.Set("Authorization", "Bearer "+raw)
				req.RemoteAddr = peer + ":45678"
				w := httptest.NewRecorder()
				rt.handler(w, req)
				return w.Code, w.Header().Get("Retry-After")
			}
			if code, retry := call(peerB); code != http.StatusTooManyRequests || retry == "" {
				t.Fatalf("a bearer pinned to %s was not refused (with a retry hint) from %s: status %d retry-after %q", peerA, peerB, code, retry)
			}
			if code, _ := call(peerA); code == http.StatusUnauthorized || code == http.StatusForbidden || code == http.StatusTooManyRequests {
				t.Fatalf("the computer's own address was refused: status %d", code)
			}
		})
	}
}

func TestBearerBindingOwnCallsPassAndOthersFailEndToEnd(t *testing.T) {
	h, _, raw := pinnedHandler(t, BindingEnforce)
	if code := inferFrom(h, raw, peerA); code != http.StatusOK {
		t.Fatalf("own address: status %d, want 200", code)
	}
	if code := inferFrom(h, raw, peerB); code != http.StatusTooManyRequests {
		t.Fatalf("other address: status %d, want 429 with a retry hint", code)
	}
	if code := inferFrom(h, raw, peerA); code != http.StatusOK {
		t.Fatalf("own address after a refused attempt: status %d, want 200 (a refusal must not unpin or block the owner)", code)
	}
}

func TestBearerBindingHostMovesThePin(t *testing.T) {
	h, reg, raw := pinnedHandler(t, BindingEnforce)
	if code := inferFrom(h, raw, peerB); code != http.StatusTooManyRequests {
		t.Fatalf("before the host moves the pin, the new boot's address must be refused: %d", code)
	}
	if _, err := reg.EnsureCredentialForPeer(raw, peerB); err != nil {
		t.Fatal(err)
	}
	if code := inferFrom(h, raw, peerB); code != http.StatusOK {
		t.Fatalf("after the host moved the pin the computer's new address must pass: %d", code)
	}
	if code := inferFrom(h, raw, peerA); code != http.StatusTooManyRequests {
		t.Fatalf("the old address must be refused after the move: %d", code)
	}
}

func TestBearerBindingACallNeverMovesAPin(t *testing.T) {
	h, reg, raw := pinnedHandler(t, BindingEnforce)
	for i := 0; i < 5; i++ {
		_ = inferFrom(h, raw, peerB)
	}
	if got := reg.identities["computer-a"].PeerIP; got != peerA {
		t.Fatalf("a refused call moved the pin to %q", got)
	}
}

func TestBearerBindingFirstUseBindsInEnforce(t *testing.T) {
	h, reg, _ := setupHandler(t)
	h.SetBearerBinding(BindingEnforce)
	cred, _ := reg.IssueCredential("computer-new")
	if code := inferFrom(h, cred.RawToken, peerA); code != http.StatusOK {
		t.Fatalf("first use: %d", code)
	}
	if got := reg.identities["computer-new"].PeerIP; got != peerA {
		t.Fatalf("first use did not bind: pin %q", got)
	}
	if code := inferFrom(h, cred.RawToken, peerB); code != http.StatusTooManyRequests {
		t.Fatalf("a second address after first use must be refused: %d", code)
	}
	// An attacker who wins the first-use race loses it back to the host's
	// attested address.
	if _, err := reg.EnsureCredentialForPeer(cred.RawToken, peerB); err != nil {
		t.Fatal(err)
	}
	if code := inferFrom(h, cred.RawToken, peerA); code != http.StatusTooManyRequests {
		t.Fatalf("the host's attestation must override the first-use pin: %d", code)
	}
}

func TestBearerBindingFirstUseSurvivesARestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gateway-identities.json")
	reg := NewIdentityRegistry(time.Hour)
	if err := reg.SetPersistencePath(path); err != nil {
		t.Fatal(err)
	}
	h := NewHandlerWithRateLimit(reg, &mockProvider{name: "bedrock", real: true, response: &provider.LLMResponse{ID: "r", Text: "ok", ProviderName: "bedrock"}}, nil)
	h.SetBearerBinding(BindingEnforce)
	cred, _ := reg.IssueCredential("computer-restart")
	if code := inferFrom(h, cred.RawToken, peerA); code != http.StatusOK {
		t.Fatalf("first use: %d", code)
	}

	restarted := NewIdentityRegistry(time.Hour)
	if err := restarted.SetPersistencePath(path); err != nil {
		t.Fatal(err)
	}
	h2 := NewHandlerWithRateLimit(restarted, &mockProvider{name: "bedrock", real: true, response: &provider.LLMResponse{ID: "r", Text: "ok", ProviderName: "bedrock"}}, nil)
	h2.SetBearerBinding(BindingEnforce)
	if code := inferFrom(h2, cred.RawToken, peerB); code != http.StatusTooManyRequests {
		t.Fatalf("a gateway restart widened the bearer: %d", code)
	}
	if code := inferFrom(h2, cred.RawToken, peerA); code != http.StatusOK {
		t.Fatalf("a gateway restart refused the computer's own address: %d", code)
	}
}

func TestBearerBindingStrictRefusesUnpinned(t *testing.T) {
	h, reg, _ := setupHandler(t)
	h.SetBearerBinding(BindingStrict)
	cred, _ := reg.IssueCredential("computer-unpinned")
	if code := inferFrom(h, cred.RawToken, peerA); code != http.StatusTooManyRequests {
		t.Fatalf("strict mode passed an unpinned bearer: %d", code)
	}
	if reg.identities["computer-unpinned"].PeerIP != "" {
		t.Fatal("strict mode bound an unpinned bearer on first use")
	}
	if _, err := reg.EnsureCredentialForPeer(cred.RawToken, peerA); err != nil {
		t.Fatal(err)
	}
	if code := inferFrom(h, cred.RawToken, peerA); code != http.StatusOK {
		t.Fatalf("strict mode refused a host-pinned bearer at its own address: %d", code)
	}
}

func TestBearerBindingObserveCountsAndNeverRefuses(t *testing.T) {
	h, reg, raw := pinnedHandler(t, BindingObserve)
	unpinned, _ := reg.IssueCredential("computer-unpinned")

	if code := inferFrom(h, raw, peerB); code != http.StatusOK {
		t.Fatalf("observe refused a mismatch: %d", code)
	}
	if code := inferFrom(h, unpinned.RawToken, peerA); code != http.StatusOK {
		t.Fatalf("observe refused an unpinned bearer: %d", code)
	}
	if reg.identities["computer-a"].PeerIP != peerA {
		t.Fatal("observe moved a pin")
	}
	if reg.identities["computer-unpinned"].PeerIP != "" {
		t.Fatal("observe wrote a first-use pin")
	}
	stats := h.bindingStatsSnapshot()
	if stats.WouldRefuse != 1 || stats.Refused != 0 || stats.Bound != 0 {
		t.Fatalf("observe counters = %+v, want would_refuse 1, refused 0, bound 0", stats)
	}
}

func TestBearerBindingOffWritesNothing(t *testing.T) {
	h, reg, _ := setupHandler(t)
	h.SetBearerBinding(BindingOff)
	cred, _ := reg.IssueCredential("computer-off")
	if code := inferFrom(h, cred.RawToken, peerA); code != http.StatusOK {
		t.Fatalf("off mode: %d", code)
	}
	if code := inferFrom(h, cred.RawToken, peerB); code != http.StatusOK {
		t.Fatalf("off mode must restore the old behavior (any guest passes): %d", code)
	}
	if reg.identities["computer-off"].PeerIP != "" {
		t.Fatal("off mode wrote a pin")
	}
}

func TestBearerBindingLoopbackCannotUseAGuestsBearer(t *testing.T) {
	h, _, raw := pinnedHandler(t, BindingEnforce)
	for _, peer := range []string{"127.0.0.1", "::1"} {
		if code := inferFrom(h, raw, peer); code != http.StatusTooManyRequests {
			t.Fatalf("a host process at %s used a guest's bearer: %d", peer, code)
		}
	}
}

func TestBearerBindingConcurrentFirstUseHasOneWinner(t *testing.T) {
	h, reg, _ := setupHandler(t)
	h.SetBearerBinding(BindingEnforce)
	cred, _ := reg.IssueCredential("computer-race")
	peers := []string{"10.200.0.2", "10.200.1.2", "10.200.2.2", "10.200.3.2", "10.200.4.2", "10.200.5.2", "10.200.6.2", "10.200.7.2"}
	var wg sync.WaitGroup
	var mu sync.Mutex
	passed := 0
	for _, p := range peers {
		wg.Add(1)
		go func(peer string) {
			defer wg.Done()
			if _, err := h.authenticateAutoputer(bindingRequest(cred.RawToken, peer)); err == nil {
				mu.Lock()
				passed++
				mu.Unlock()
			}
		}(p)
	}
	wg.Wait()
	if passed != 1 {
		t.Fatalf("%d first uses from different addresses passed, want exactly 1", passed)
	}
}

func TestBearerBindingFailedPinWriteRefusesAndLeavesNoPin(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("a root process can write into a read-only directory")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "gateway-identities.json")
	reg := NewIdentityRegistry(time.Hour)
	if err := reg.SetPersistencePath(path); err != nil {
		t.Fatal(err)
	}
	cred, err := reg.IssueCredential("computer-disk")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	h := NewHandlerWithRateLimit(reg, &mockProvider{name: "bedrock", real: true, response: &provider.LLMResponse{ID: "r", Text: "ok", ProviderName: "bedrock"}}, nil)
	h.SetBearerBinding(BindingEnforce)
	if code := inferFrom(h, cred.RawToken, peerA); code != http.StatusServiceUnavailable {
		t.Fatalf("enforce accepted a first use whose pin could not be written, or called it an authentication failure: %d", code)
	}
	if reg.identities["computer-disk"].PeerIP != "" {
		t.Fatal("a failed write left an in-memory pin that the file does not have")
	}
}

func TestBearerBindingLogIsBoundedAndNeverNamesTheToken(t *testing.T) {
	h, _, raw := pinnedHandler(t, BindingEnforce)
	var buf bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(prev)
	for i := 0; i < 200; i++ {
		_ = inferFrom(h, raw, peerB)
	}
	out := buf.String()
	secret := raw[strings.Index(raw, ":")+1:]
	if strings.Contains(out, secret) {
		t.Fatal("the mismatch log names the bearer's secret")
	}
	lines := strings.Count(out, "bearer binding")
	if lines < 1 {
		t.Fatalf("a refused mismatch left no log line; log: %q", out)
	}
	if lines > 5 {
		t.Fatalf("200 retries of one mismatch wrote %d binding log lines; want it bounded", lines)
	}
	if got := h.bindingStatsSnapshot().Refused; got != 200 {
		t.Fatalf("refused counter = %d, want 200 (the counter counts every refusal; only the log is bounded)", got)
	}
}

func TestBearerBindingModeParsing(t *testing.T) {
	cases := map[string]BindingMode{
		"":         BindingEnforce,
		"enforce":  BindingEnforce,
		"ENFORCE ": BindingEnforce,
		"observe":  BindingObserve,
		"Observe":  BindingObserve,
		"off":      BindingOff,
		"strict":   BindingStrict,
		"garbage":  BindingEnforce,
		"on":       BindingEnforce,
		"disabled": BindingEnforce,
	}
	for raw, want := range cases {
		if got := ParseBindingMode(raw); got != want {
			t.Errorf("ParseBindingMode(%q) = %q, want %q", raw, got, want)
		}
	}
	if (&Handler{}).bearerBindingMode() != BindingEnforce {
		t.Fatal("a handler nobody configured must enforce")
	}
}

func TestBearerBindingHealthReportsModeAndCounts(t *testing.T) {
	h, reg, raw := pinnedHandler(t, BindingObserve)
	_, _ = reg.IssueCredential("computer-unpinned")
	_ = inferFrom(h, raw, peerB)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	h.HandleHealth(w, req)
	var body struct {
		BearerBinding struct {
			Mode        string `json:"mode"`
			Pinned      int    `json:"pinned"`
			Unpinned    int    `json:"unpinned"`
			WouldRefuse int64  `json:"would_refuse"`
			Refused     int64  `json:"refused"`
		} `json:"bearer_binding"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	b := body.BearerBinding
	if b.Mode != "observe" || b.Pinned != 1 || b.Unpinned != 1 || b.WouldRefuse != 1 || b.Refused != 0 {
		t.Fatalf("bearer_binding = %+v; body: %s", b, w.Body.String())
	}
	if strings.Contains(w.Body.String(), peerA) {
		t.Fatal("the public health endpoint names a pinned address")
	}
}

func TestPeerPinKeepsTheLegacyJSONKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gateway-identities.json")
	reg := NewIdentityRegistry(time.Hour)
	if err := reg.SetPersistencePath(path); err != nil {
		t.Fatal(err)
	}
	cred, _ := reg.IssueCredential("computer-compat")
	if _, err := reg.EnsureCredentialForPeer(cred.RawToken, peerA); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"JevPeerIP": "`+peerA+`"`) {
		t.Fatalf("the pin is not stored under the key an older gateway reads: %s", data)
	}
	// A store written before this change (only Jev-pinned identities had a
	// pin) loads as a pin.
	legacy := filepath.Join(t.TempDir(), "legacy.json")
	if err := os.WriteFile(legacy, []byte(`{"c":{"ComputerID":"c","TokenHash":"00","Active":true,"ExpiresAt":"2999-01-01T00:00:00Z","JevPeerIP":"`+peerB+`"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	old := NewIdentityRegistry(time.Hour)
	if err := old.SetPersistencePath(legacy); err != nil {
		t.Fatal(err)
	}
	if got := old.identities["c"].PeerIP; got != peerB {
		t.Fatalf("legacy pin loaded as %q", got)
	}
}

func TestConstantTimeHashCompare(t *testing.T) {
	a := strings.Repeat("ab", 32)
	if !hashesEqual(a, a) {
		t.Fatal("equal hashes compared unequal")
	}
	if hashesEqual(a, strings.Repeat("ab", 31)+"ac") {
		t.Fatal("hashes differing in the last byte compared equal")
	}
	if hashesEqual(a, a[:62]) || hashesEqual("", a) || hashesEqual(a, "") {
		t.Fatal("hashes of different length compared equal")
	}
	if hashesEqual("", "") {
		t.Fatal("two empty hashes compared equal; an identity with no hash must match nothing")
	}
}

func TestBearerBindingKeepsRefusingWhatWasRefused(t *testing.T) {
	// Binding only narrows. Every refusal that existed before it still holds
	// in every mode: no header, wrong scheme, revoked, an outside address.
	for _, mode := range []BindingMode{BindingOff, BindingObserve, BindingEnforce, BindingStrict} {
		h, reg, raw := pinnedHandler(t, mode)
		req := httptest.NewRequest(http.MethodPost, "/provider/v1/inference", strings.NewReader(inferenceBody()))
		w := httptest.NewRecorder()
		h.HandleInference(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("%s: no header: %d", mode, w.Code)
		}
		if code := inferFrom(h, raw, "8.8.8.8"); code != http.StatusUnauthorized {
			t.Fatalf("%s: outside address: %d", mode, code)
		}
		reg.RevokeCredential("computer-a")
		if code := inferFrom(h, raw, peerA); code != http.StatusUnauthorized {
			t.Fatalf("%s: revoked: %d", mode, code)
		}
	}
}

// A binding refusal tells a legitimate guest to come back. Pinning every
// route means a computer rebooted onto a new address (a deploy refresh or a
// recovery keeps its token and takes a new address from the next host port)
// is refused until vmctl re-ensures its credential, which happens right after
// the boot returns while a resumed guest may already be calling. The guest
// waits out a 429 Retry-After (internal/gatewayruntime/provider.go), so the
// refusal heals itself once the host moves the pin; a genuine mismatch stays
// refused and every refusal is still a refusal.
func TestBearerBindingRefusalAsksTheGuestToRetryAndHealsWhenTheHostMovesThePin(t *testing.T) {
	h, reg, raw := pinnedHandler(t, BindingEnforce)
	mp := h.provider.(*mockProvider)
	sawRequest := func() bool {
		mp.mu.Lock()
		defer mp.mu.Unlock()
		return mp.lastReq != nil
	}

	req := httptest.NewRequest(http.MethodPost, "/provider/v1/inference", strings.NewReader(inferenceBody()))
	req.Header.Set("Authorization", "Bearer "+raw)
	req.RemoteAddr = peerB + ":45678"
	w := httptest.NewRecorder()
	h.HandleInference(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("status %d, want 429", w.Code)
	}
	retry := w.Header().Get("Retry-After")
	if secs, err := time.ParseDuration(retry + "s"); err != nil || secs < time.Second || secs > 10*time.Second {
		t.Fatalf("Retry-After %q is not a short wait", retry)
	}
	if strings.Contains(w.Body.String(), peerA) || strings.Contains(w.Body.String(), peerB) {
		t.Fatalf("the refusal body names an address: %s", w.Body.String())
	}
	if sawRequest() {
		t.Fatal("a refused call reached the provider")
	}

	if _, err := reg.EnsureCredentialForPeer(raw, peerB); err != nil {
		t.Fatal(err)
	}
	if code := inferFrom(h, raw, peerB); code != http.StatusOK {
		t.Fatalf("after the host moved the pin, the retry must pass: %d", code)
	}
}

// Failures that are not a binding refusal stay 401: no header, a wrong or
// revoked credential, an address outside the runtime peers.
func TestBearerBindingLeavesOtherAuthFailuresAt401(t *testing.T) {
	h, reg, raw := pinnedHandler(t, BindingEnforce)
	if code := inferFrom(h, "computer-a:not-the-token", peerA); code != http.StatusUnauthorized {
		t.Fatalf("wrong token: %d", code)
	}
	if code := inferFrom(h, raw, "8.8.8.8"); code != http.StatusUnauthorized {
		t.Fatalf("outside address: %d", code)
	}
	reg.RevokeCredential("computer-a")
	if code := inferFrom(h, raw, peerA); code != http.StatusUnauthorized {
		t.Fatalf("revoked: %d", code)
	}
}
