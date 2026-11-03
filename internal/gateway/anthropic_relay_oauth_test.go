package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The relay holds the owner's Claude token on the host
// (docs/problems/claude-delegate-token-on-three-computers-2026-11-03.md,
// design section). Failure modes 13, 14, 21 and the grant read:
//  13. the token reaches a log, an error body, /health or the grant read
//      (TestAnthropicRelayOAuthNeverLeaksTheToken);
//  14. a missing or bad token file falls back per call, or the kind is mixed
//      (TestAnthropicRelayOAuthConfigFromEnv);
//  21. the spend cap is charged to the wrong computer, not charged on a
//      stream, or lets a call start once spent
//      (TestAnthropicRelayDailyTokenCap);
//  grant: the grant read answers only the caller, after the same checks,
//      and writes no identity state (TestAnthropicRelayGrantRead).

const relayTestOAuthToken = "sk-ant-oat01-FAKE-oauth-test-token-0123456789abcdef"

func useOAuth(rh *relayHarness) {
	rh.relay.key = relayTestOAuthToken
	rh.relay.kind = relayCredentialOAuth
}

func TestAnthropicRelayOAuthSendsTheTokenAsABearer(t *testing.T) {
	rh := newRelayHarness(t)
	useOAuth(rh)
	w := rh.do(http.MethodPost, "/provider/anthropic/v1/messages?beta=true", relayMessageBody(relayTestModel), peerA, func(r *http.Request) {
		r.Header.Set("X-Api-Key", "sk-ant-the-callers-own-key")
		r.Header.Set("Anthropic-Beta", "claude-code-20250219,fine-grained-tool-streaming-2025-05-14")
		r.Header.Set("User-Agent", "claude-cli/2.1.0 (external, sdk-cli)")
		r.Header.Set("X-App", "cli")
	})
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	got := rh.requests()[0]
	if got.Header.Get("Authorization") != "Bearer "+relayTestOAuthToken {
		t.Fatalf("upstream authorization is not the host-held token")
	}
	if got.Header.Get("X-Api-Key") != "" {
		t.Fatalf("an OAuth call also sent x-api-key")
	}
	beta := got.Header.Get("Anthropic-Beta")
	if !strings.Contains(beta, "oauth-2025-04-20") || !strings.Contains(beta, "claude-code-20250219") {
		t.Fatalf("anthropic-beta = %q, want the caller's betas plus the OAuth beta", beta)
	}
	if got.Header.Get("User-Agent") != "claude-cli/2.1.0 (external, sdk-cli)" || got.Header.Get("X-App") != "cli" {
		t.Fatalf("client identification not passed: ua=%q x-app=%q", got.Header.Get("User-Agent"), got.Header.Get("X-App"))
	}
	// A header-splitting or oversized user agent is replaced, not passed.
	w = rh.do(http.MethodPost, "/provider/anthropic/v1/messages", relayMessageBody(relayTestModel), peerA, func(r *http.Request) {
		r.Header.Set("User-Agent", strings.Repeat("x", 300))
	})
	if w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
	if ua := rh.requests()[1].Header.Get("User-Agent"); ua != "choir-gateway-anthropic-relay" {
		t.Fatalf("oversized user agent passed: %q", ua)
	}
	// The API-key kind adds no OAuth beta.
	rh.relay.kind = relayCredentialAPIKey
	rh.relay.key = relayTestKey
	_ = rh.do(http.MethodPost, "/provider/anthropic/v1/messages", relayMessageBody(relayTestModel), peerA, nil)
	if beta := rh.requests()[2].Header.Get("Anthropic-Beta"); strings.Contains(beta, "oauth") {
		t.Fatalf("an API-key call carries the OAuth beta: %q", beta)
	}
}

func TestAnthropicRelayRedactsASuccessHeaderAndChargesAnAbort(t *testing.T) {
	rh := newRelayHarness(t)
	useOAuth(rh)
	rh.setUpstream(func(w http.ResponseWriter, r *http.Request, body []byte) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Anthropic-Echo", r.Header.Get("Authorization"))
		_, _ = w.Write([]byte(`{"usage":{"input_tokens":1,"output_tokens":1}}`))
	})
	w := rh.do(http.MethodPost, "/provider/anthropic/v1/messages", relayMessageBody(relayTestModel), peerA, nil)
	if strings.Contains(w.Header().Get("Anthropic-Echo"), relayTestOAuthToken) {
		t.Fatal("a success header carried the token to the caller")
	}
	// A caller that goes away before the answer is still charged.
	rh.setUpstream(func(w http.ResponseWriter, r *http.Request, body []byte) { <-r.Context().Done() }) // hang-guard: ends with the cancelled request
	before := rh.relay.spentToday(relayTestIdentity)
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodPost, "/provider/anthropic/v1/messages", strings.NewReader(relayMessageBody(relayTestModel))).WithContext(ctx)
	req.RemoteAddr = peerA + ":45678"
	req.Header.Set("Authorization", "Bearer "+rh.bearer)
	req.Header.Set("Content-Type", "application/json")
	go func() { time.Sleep(100 * time.Millisecond); cancel() }()
	rh.h.HandleAnthropicRelay(httptest.NewRecorder(), req)
	if rh.relay.spentToday(relayTestIdentity) <= before {
		t.Fatal("an aborted call was not charged")
	}
}

func TestAnthropicRelayOAuthNeverLeaksTheToken(t *testing.T) {
	rh := newRelayHarness(t)
	useOAuth(rh)
	var logs bytes.Buffer
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	rh.setUpstream(func(w http.ResponseWriter, r *http.Request, body []byte) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Echo", r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"type":"error","error":{"type":"authentication_error","message":"bad token ` + r.Header.Get("Authorization") + `"}}`))
	})
	w := rh.do(http.MethodPost, "/provider/anthropic/v1/messages", relayMessageBody(relayTestModel), peerA, nil)
	if strings.Contains(w.Body.String(), relayTestOAuthToken) || strings.Contains(w.Header().Get("X-Echo"), relayTestOAuthToken) {
		t.Fatalf("the token reached the caller: %s", w.Body.String())
	}
	grant := rh.do(http.MethodGet, "/provider/anthropic/grant", "", peerA, nil)
	health := httptest.NewRecorder()
	rh.h.HandleHealth(health, httptest.NewRequest(http.MethodGet, "/health", nil))
	for name, text := range map[string]string{"log": logs.String(), "grant": grant.Body.String(), "health": health.Body.String()} {
		if strings.Contains(text, relayTestOAuthToken) {
			t.Fatalf("the token reached the %s", name)
		}
	}
	if strings.Contains(health.Body.String(), "oauth") {
		t.Fatalf("health names the credential kind (it is for the startup log): %s", health.Body.String())
	}
}

func TestAnthropicRelayOAuthConfigFromEnv(t *testing.T) {
	dir := t.TempDir()
	tokenFile := filepath.Join(dir, "claude-oauth-token")
	if err := os.WriteFile(tokenFile, []byte(relayTestOAuthToken+"\n"), 0o400); err != nil {
		t.Fatal(err)
	}
	get := func(m map[string]string) func(string) string { return func(k string) string { return m[k] } }
	base := map[string]string{
		"GATEWAY_ANTHROPIC_OAUTH_TOKEN_FILE": tokenFile,
		"GATEWAY_ANTHROPIC_RELAY_IDENTITIES": "vm-a",
		"GATEWAY_ANTHROPIC_RELAY_EXPIRES":    "2999-01-01",
	}
	r := newAnthropicRelayFromGetenv(get(base))
	if r.status() != "on" || r.kind != relayCredentialOAuth || r.key != relayTestOAuthToken {
		t.Fatalf("token file: status %q kind %q", r.status(), r.kind)
	}
	if r.dailyTokens != anthropicRelayDailyTokens {
		t.Fatalf("default daily cap = %d", r.dailyTokens)
	}
	// The token file wins over a key; the choice is one kind, made here.
	withKey := map[string]string{"ANTHROPIC_API_KEY": relayTestKey}
	for k, v := range base {
		withKey[k] = v
	}
	if r := newAnthropicRelayFromGetenv(get(withKey)); r.kind != relayCredentialOAuth || r.key != relayTestOAuthToken {
		t.Fatalf("token and key: kind %q", r.kind)
	}
	// A named but missing, empty or malformed token file leaves the relay
	// off: it does not fall back to the key.
	open := filepath.Join(dir, "open")
	_ = os.WriteFile(open, []byte(relayTestOAuthToken+"\n"), 0o444)
	link := filepath.Join(dir, "link")
	_ = os.Symlink(tokenFile, link)
	empty := filepath.Join(dir, "empty")
	_ = os.WriteFile(empty, nil, 0o400)
	bad := filepath.Join(dir, "bad")
	_ = os.WriteFile(bad, []byte("two words\n"), 0o400)
	for name, path := range map[string]string{"missing": filepath.Join(dir, "absent"), "empty": empty, "malformed": bad, "a directory": dir, "group-readable": open, "a symlink": link} {
		m := map[string]string{}
		for k, v := range withKey {
			m[k] = v
		}
		m["GATEWAY_ANTHROPIC_OAUTH_TOKEN_FILE"] = path
		if r := newAnthropicRelayFromGetenv(get(m)); r.status() != "off" || r.key != "" {
			t.Fatalf("%s token file: status %q kind %q, want off with no credential", name, r.status(), r.kind)
		}
	}
	// No token file named: the key, as FP7 built it.
	keyOnly := map[string]string{}
	for k, v := range withKey {
		keyOnly[k] = v
	}
	delete(keyOnly, "GATEWAY_ANTHROPIC_OAUTH_TOKEN_FILE")
	if r := newAnthropicRelayFromGetenv(get(keyOnly)); r.kind != relayCredentialAPIKey || r.status() != "on" {
		t.Fatalf("key only: kind %q status %q", r.kind, r.status())
	}
	cap := map[string]string{"GATEWAY_ANTHROPIC_RELAY_DAILY_TOKENS": "1234"}
	for k, v := range base {
		cap[k] = v
	}
	if r := newAnthropicRelayFromGetenv(get(cap)); r.dailyTokens != 1234 {
		t.Fatalf("daily cap = %d, want 1234", r.dailyTokens)
	}
}

func TestAnthropicRelayDailyTokenCap(t *testing.T) {
	rh := newRelayHarness(t)
	day := time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC)
	rh.relay.now = func() time.Time { return day }
	rh.relay.expires = day.Add(48 * time.Hour)
	rh.relay.dailyTokens = 30
	// Each reply: 11 input + 7 output (+3 cached reads, not charged) = 18.
	if w := rh.do(http.MethodPost, "/provider/anthropic/v1/messages", relayMessageBody(relayTestModel), peerA, nil); w.Code != http.StatusOK {
		t.Fatalf("first call: %d", w.Code)
	}
	if got := rh.relay.spentToday(relayTestIdentity); got != 18 {
		t.Fatalf("charged %d, want 18", got)
	}
	// Under the cap, a call starts (and may overshoot it once).
	if w := rh.do(http.MethodPost, "/provider/anthropic/v1/messages", relayMessageBody(relayTestModel), peerA, nil); w.Code != http.StatusOK {
		t.Fatalf("second call: %d", w.Code)
	}
	before := len(rh.requests())
	w := rh.do(http.MethodPost, "/provider/anthropic/v1/messages", relayMessageBody(relayTestModel), peerA, nil)
	if w.Code != http.StatusTooManyRequests || !strings.Contains(w.Body.String(), "rate_limit_error") {
		t.Fatalf("spent: %d %s, want 429", w.Code, w.Body.String())
	}
	if len(rh.requests()) != before {
		t.Fatal("a call was relayed after the budget was spent")
	}
	// Another computer's budget is its own.
	if got := rh.relay.spentToday("vm-other"); got != 0 {
		t.Fatalf("another computer was charged %d", got)
	}
	// A streamed reply is charged too.
	day = day.Add(24 * time.Hour)
	rh.setUpstream(func(w http.ResponseWriter, r *http.Request, body []byte) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":5,\"output_tokens\":1,\"cache_creation_input_tokens\":4}}}\n\n"))
		_, _ = w.Write([]byte("event: message_delta\ndata: {\"type\":\"message_delta\",\"usage\":{\"output_tokens\":9}}\n\n"))
	})
	if w := rh.do(http.MethodPost, "/provider/anthropic/v1/messages", relayMessageBody(relayTestModel), peerA, func(r *http.Request) { r.Header.Set("Accept", "text/event-stream") }); w.Code != http.StatusOK {
		t.Fatalf("next day: %d (the budget did not renew)", w.Code)
	}
	if got := rh.relay.spentToday(relayTestIdentity); got != 18 {
		t.Fatalf("stream charged %d, want 5+9+4=18", got)
	}
}

func TestAnthropicRelaySpendSurvivesARestart(t *testing.T) {
	dir := t.TempDir()
	spend := filepath.Join(dir, "spend.json")
	get := func(k string) string {
		return map[string]string{
			"ANTHROPIC_API_KEY":                    relayTestKey,
			"GATEWAY_ANTHROPIC_RELAY_IDENTITIES":   "vm-a",
			"GATEWAY_ANTHROPIC_RELAY_EXPIRES":      "2999-01-01",
			"GATEWAY_ANTHROPIC_RELAY_DAILY_TOKENS": "100",
			"GATEWAY_ANTHROPIC_RELAY_SPEND_FILE":   spend,
		}[k]
	}
	r := newAnthropicRelayFromGetenv(get)
	r.charge("vm-a", relayUsage{InputTokens: 60, OutputTokens: 50})
	if !r.budgetSpent("vm-a") {
		t.Fatal("110 of 100 is not spent")
	}
	again := newAnthropicRelayFromGetenv(get)
	if got := again.spentToday("vm-a"); got != 110 || !again.budgetSpent("vm-a") {
		t.Fatalf("after a restart the day's spend is %d", got)
	}
	// Another day's record starts at zero.
	again.now = func() time.Time { return time.Now().Add(48 * time.Hour) }
	if again.budgetSpent("vm-a") {
		t.Fatal("yesterday's spend counts today")
	}
	// An unreadable cap is the default cap, never none.
	bad := newAnthropicRelayFromGetenv(func(k string) string {
		if k == "GATEWAY_ANTHROPIC_RELAY_DAILY_TOKENS" {
			return "-5"
		}
		return get(k)
	})
	if bad.dailyTokens != anthropicRelayDailyTokens {
		t.Fatalf("a bad cap gave %d", bad.dailyTokens)
	}
}

func TestAnthropicRelayGrantRead(t *testing.T) {
	rh := newRelayHarness(t)
	useOAuth(rh)
	type grant struct {
		Available       bool   `json:"available"`
		Reason          string `json:"reason"`
		ExpiresAt       string `json:"expires_at"`
		Model           string `json:"model"`
		DailyTokenCap   int64  `json:"daily_token_cap"`
		DailyTokensUsed int64  `json:"daily_tokens_used"`
	}
	read := func(peer string, mutate func(*http.Request)) (int, grant, string) {
		w := rh.do(http.MethodGet, "/provider/anthropic/grant", "", peer, mutate)
		var g grant
		_ = json.Unmarshal(w.Body.Bytes(), &g)
		return w.Code, g, w.Body.String()
	}
	code, g, body := read(peerA, nil)
	if code != http.StatusOK || !g.Available || g.Model != relayTestModel || g.ExpiresAt == "" || g.DailyTokenCap != rh.relay.dailyTokens {
		t.Fatalf("listed computer: %d %s", code, body)
	}
	if strings.Contains(body, relayTestIdentity) || strings.Contains(body, "oauth") {
		t.Fatalf("the grant read names an identity or the credential: %s", body)
	}
	// Not listed, closed window, no credential: 200 and unavailable, with a
	// reason; nothing about other computers.
	for name, mutate := range map[string]func(){
		"unlisted":      func() { rh.relay.identities = map[string]bool{"vm-other": true} },
		"closed window": func() { rh.relay.expires = time.Now().Add(-time.Minute) },
		"no credential": func() { rh.relay.key = "" },
	} {
		identities, expires, key := rh.relay.identities, rh.relay.expires, rh.relay.key
		mutate()
		code, g, body := read(peerA, nil)
		if code != http.StatusOK || g.Available || g.Reason == "" || strings.Contains(body, "vm-other") {
			t.Fatalf("%s: %d %s", name, code, body)
		}
		rh.relay.identities, rh.relay.expires, rh.relay.key = identities, expires, key
	}
	// No bearer, or the bearer from another address: refused.
	if code, _, _ := read(peerA, func(r *http.Request) { r.Header.Del("Authorization") }); code != http.StatusUnauthorized {
		t.Fatalf("no bearer: %d", code)
	}
	if code, _, body := read("10.200.7.7", nil); code == http.StatusOK {
		t.Fatalf("a copied bearer read the grant: %s", body)
	}
	rh.relay.identities = map[string]bool{"vm-other": true}
	if code, _, body := read("10.200.7.7", nil); code == http.StatusOK {
		t.Fatalf("a copied bearer learned an unlisted answer: %s", body)
	}
	rh.relay.identities = map[string]bool{relayTestIdentity: true}
	// Only GET.
	if w := rh.do(http.MethodPost, "/provider/anthropic/grant", "{}", peerA, nil); w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST grant: %d", w.Code)
	}
	if len(rh.requests()) != 0 {
		t.Fatal("a grant read called the upstream")
	}
}
