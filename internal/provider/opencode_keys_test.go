package provider

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// Failure modes (docs/archive/problems/opencode-key-exhausted-no-failover-2026-10-15.md):
// a 402 key is parked for hours and the call retried on another key; a 429
// key is parked briefly; a session stays on one healthy key; with every key
// parked no upstream call is made; key values never appear in errors.

func TestOpenCodeKeysFromEnvOrderAndDedup(t *testing.T) {
	t.Setenv("OPENCODE_API_KEY", "key-primary")
	t.Setenv("OPENCODE_GO_API_KEY", "key-primary")
	t.Setenv("OPENCODE_API_KEY_ALT_1", "key-alt-1")
	t.Setenv("OPENCODE_API_KEY_ALT_0", "key-alt-0")
	keys := openCodeKeysFromEnv()
	var names []string
	for _, k := range keys {
		names = append(names, k.name)
	}
	if got := strings.Join(names, ","); got != "OPENCODE_API_KEY,OPENCODE_API_KEY_ALT_0,OPENCODE_API_KEY_ALT_1" {
		t.Fatalf("names = %s", got)
	}
}

func TestOpenCodeKeyPoolParksAndRecovers(t *testing.T) {
	now := time.Unix(1000, 0)
	pool := newOpenCodeKeyPool([]openCodeKey{{name: "A", value: "a"}, {name: "B", value: "b"}})
	pool.now = func() time.Time { return now }

	first, ok := pool.pick("session-1", nil)
	if !ok {
		t.Fatal("no key")
	}
	again, _ := pool.pick("session-1", nil)
	if again.name != first.name {
		t.Fatalf("session not sticky: %s then %s", first.name, again.name)
	}
	pool.park(first.name, http.StatusPaymentRequired)
	other, ok := pool.pick("session-1", nil)
	if !ok || other.name == first.name {
		t.Fatalf("after 402 got %v %v", other.name, ok)
	}
	pool.park(other.name, http.StatusTooManyRequests)
	if _, ok := pool.pick("session-1", nil); ok {
		t.Fatal("every key parked, but pick returned one")
	}
	now = now.Add(openCodeRateLimitPark + time.Second)
	if k, ok := pool.pick("session-1", nil); !ok || k.name != other.name {
		t.Fatalf("429 key did not recover: %v %v", k.name, ok)
	}
	now = now.Add(openCodeFundsPark)
	if _, ok := pool.pick("session-1", nil); !ok {
		t.Fatal("402 key did not recover after its park")
	}
}

func TestOpenCodeProviderRetriesOnAnotherKeyAfter402(t *testing.T) {
	var mu sync.Mutex
	seen := map[string]int{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		mu.Lock()
		seen[auth]++
		mu.Unlock()
		if auth == "Bearer key-empty" {
			w.WriteHeader(http.StatusPaymentRequired)
			_, _ = io.WriteString(w, `{"error":{"type":"server_error","message":"Insufficient account funds"}}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"x","model":"deepseek-v4.1-flash","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`)
	}))
	defer srv.Close()

	pool := newOpenCodeKeyPool([]openCodeKey{{name: "OPENCODE_API_KEY", value: "key-empty"}, {name: "OPENCODE_API_KEY_ALT_0", value: "key-funded"}})
	p := &OpenCodeProvider{keys: pool, modelID: "deepseek-v4.1-flash", baseURL: srv.URL, providerName: "opencode-go", httpClient: srv.Client()}

	// Several sessions, so at least one starts on the empty key.
	for _, session := range []string{"s1", "s2", "s3", "s4", "s5", "s6"} {
		resp, err := p.Call(context.Background(), LLMRequest{ConversationID: session, Messages: []Message{{Role: "user", Content: []Block{{Type: "text", Text: "hi"}}}}})
		if err != nil {
			t.Fatalf("%s: %v", session, err)
		}
		if resp == nil {
			t.Fatalf("%s: nil response", session)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if seen["Bearer key-empty"] != 1 {
		t.Fatalf("empty key called %d times, want exactly 1 (then parked)", seen["Bearer key-empty"])
	}
}

func TestOpenCodeProviderAllKeysParkedMakesNoCall(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusPaymentRequired)
		_, _ = io.WriteString(w, `{"error":{"message":"Insufficient account funds"}}`)
	}))
	defer srv.Close()
	pool := newOpenCodeKeyPool([]openCodeKey{{name: "OPENCODE_API_KEY", value: "secret-value-1"}, {name: "OPENCODE_API_KEY_ALT_0", value: "secret-value-2"}})
	p := &OpenCodeProvider{keys: pool, modelID: "deepseek-v4.1-flash", baseURL: srv.URL, providerName: "opencode-go", httpClient: srv.Client()}

	_, err := p.Call(context.Background(), LLMRequest{ConversationID: "s", Messages: []Message{{Role: "user", Content: []Block{{Type: "text", Text: "hi"}}}}})
	if err == nil {
		t.Fatal("want an error with every key out of funds")
	}
	if calls != 2 {
		t.Fatalf("first call made %d upstream requests, want 2 (one per key)", calls)
	}
	_, err = p.Call(context.Background(), LLMRequest{ConversationID: "s", Messages: []Message{{Role: "user", Content: []Block{{Type: "text", Text: "hi"}}}}})
	if err == nil || calls != 2 {
		t.Fatalf("second call: err=%v calls=%d, want an error and no new upstream request", err, calls)
	}
	if strings.Contains(err.Error(), "secret-value") {
		t.Fatalf("error leaks a key: %v", err)
	}
}

// A revoked or expired credential answers 401 on every call: the key is parked
// like an empty account and the call retried on another key, so no session
// stays pinned to it (docs/problems/opencode-401-key-never-parked-2026-10-22.md).
func TestOpenCodeProviderRetriesOnAnotherKeyAfterDeadCredential(t *testing.T) {
	{
		status := http.StatusUnauthorized
		var mu sync.Mutex
		seen := map[string]int{}
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth := r.Header.Get("Authorization")
			mu.Lock()
			seen[auth]++
			mu.Unlock()
			if auth == "Bearer key-dead" {
				w.WriteHeader(status)
				_, _ = io.WriteString(w, `{"error":{"message":"invalid api key"}}`)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"id":"x","model":"deepseek-v4.1-flash","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`)
		}))
		pool := newOpenCodeKeyPool([]openCodeKey{{name: "OPENCODE_API_KEY", value: "key-dead"}, {name: "OPENCODE_API_KEY_ALT_0", value: "key-live"}})
		p := &OpenCodeProvider{keys: pool, modelID: "deepseek-v4.1-flash", baseURL: srv.URL, providerName: "opencode-go", httpClient: srv.Client()}
		for _, session := range []string{"s1", "s2", "s3", "s4", "s5", "s6"} {
			if _, err := p.Call(context.Background(), LLMRequest{ConversationID: session, Messages: []Message{{Role: "user", Content: []Block{{Type: "text", Text: "hi"}}}}}); err != nil {
				t.Fatalf("status %d, %s: %v", status, session, err)
			}
		}
		srv.Close()
		if seen["Bearer key-dead"] != 1 {
			t.Fatalf("status %d: dead key called %d times, want exactly 1 (then parked)", status, seen["Bearer key-dead"])
		}
	}
}

func TestOpenCodeKeyPoolParksDeadCredentialForHours(t *testing.T) {
	now := time.Unix(1000, 0)
	pool := newOpenCodeKeyPool([]openCodeKey{{name: "A", value: "a"}, {name: "B", value: "b"}})
	pool.now = func() time.Time { return now }
	if !pool.park("A", http.StatusUnauthorized) {
		t.Fatal("401 did not park")
	}
	now = now.Add(openCodeRateLimitPark + time.Second)
	if k, ok := pool.pick("s", map[string]bool{"B": true}); ok {
		t.Fatalf("a dead credential recovered after a rate-limit park: %s", k.name)
	}
	now = now.Add(openCodeCredentialPark)
	if _, ok := pool.pick("s", map[string]bool{"B": true}); !ok {
		t.Fatal("a dead credential never recovers (a renewed key must be tried again)")
	}
	// A 403 can be a model the plan does not cover; the pool is shared by
	// every model, so it must not bench the key.
	for _, status := range []int{http.StatusForbidden, http.StatusInternalServerError} {
		if pool.park("A", status) {
			t.Fatalf("status %d parked the key", status)
		}
	}
}

// A 401 that is not about one credential (an upstream auth outage, a proxy
// rejection) answers on every key: it must never bench the whole pool for
// hours, so the last healthy key is never parked on a 401 and the pool then
// behaves as it did before 401 parked at all.
func TestOpenCodeKeyPoolNeverParksTheLastKeyOn401(t *testing.T) {
	pool := newOpenCodeKeyPool([]openCodeKey{{name: "A", value: "a"}, {name: "B", value: "b"}})
	if !pool.park("A", http.StatusUnauthorized) {
		t.Fatal("the first 401 key was not parked")
	}
	if pool.park("B", http.StatusUnauthorized) {
		t.Fatal("a 401 parked the last healthy key")
	}
	if k, ok := pool.pick("s", nil); !ok || k.name != "B" {
		t.Fatalf("pick = %v %v, want B still in service", k.name, ok)
	}
	// 402 and 429 still park the last key: their meaning is per-account.
	if !pool.park("B", http.StatusPaymentRequired) {
		t.Fatal("402 must still park the last key")
	}
}

// Which OpenCode keys serve is gateway policy, GATEWAY_OPENCODE_KEYS (a list
// of key variable names), not a credential edit (owner, 2026-10-22). Failure
// modes: an unlisted key still serves; an unset policy drops keys; a policy
// naming no configured key falls back to every key instead of to none.
func TestOpenCodeKeysFromEnvHonorsThePolicy(t *testing.T) {
	t.Setenv("OPENCODE_API_KEY", "key-primary")
	t.Setenv("OPENCODE_API_KEY_ALT_0", "key-alt-0")
	t.Setenv("OPENCODE_API_KEY_ALT_1", "key-alt-1")
	names := func() string {
		var n []string
		for _, k := range openCodeKeysFromEnv() {
			n = append(n, k.name)
		}
		return strings.Join(n, ",")
	}
	if got := names(); got != "OPENCODE_API_KEY,OPENCODE_API_KEY_ALT_0,OPENCODE_API_KEY_ALT_1" {
		t.Fatalf("no policy: %s, want every key", got)
	}
	t.Setenv(openCodeKeysPolicyEnv, "OPENCODE_API_KEY_ALT_0")
	if got := names(); got != "OPENCODE_API_KEY_ALT_0" {
		t.Fatalf("policy ALT_0: %s", got)
	}
	t.Setenv(openCodeKeysPolicyEnv, " OPENCODE_API_KEY_ALT_1, OPENCODE_API_KEY_ALT_0 ")
	if got := names(); got != "OPENCODE_API_KEY_ALT_0,OPENCODE_API_KEY_ALT_1" {
		t.Fatalf("policy ALT_1,ALT_0: %s, want configured order", got)
	}
	t.Setenv(openCodeKeysPolicyEnv, "OPENCODE_API_KEY_ALT_7")
	if got := names(); got != "" {
		t.Fatalf("policy naming no configured key: %s, want none", got)
	}
	// Set but blank is a list naming no key: none serve (bunch 8b review:
	// a blank policy must not fail open to every key).
	t.Setenv(openCodeKeysPolicyEnv, " ")
	if got := names(); got != "" {
		t.Fatalf("blank policy: %s, want none", got)
	}
}
