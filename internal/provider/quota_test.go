package provider

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Model-pool autoroute (docs/design/model-pool-autoroute-2026-11-02.md).
// Ways the 429 classification could fail, written before the code: a
// Synthetic quota 429 typed as a concurrency limit (the breaker never counts
// it: 2026-11-01, 11,917 of them); a concurrency 429 typed as quota (one
// burst demotes a healthy model); the weekly header ignored; a ChatGPT usage
// limit left untyped; the ChatGPT reason never logged; a secret in the log.

func synthetic429Server(t *testing.T, body, quotas string) *SyntheticProvider {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if quotas != "" {
			w.Header().Set("x-synthetic-quotas", quotas)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		fmt.Fprint(w, body)
	}))
	t.Cleanup(srv.Close)
	p, err := NewSyntheticProvider(SyntheticConfig{APIKey: "syn-test-key", ModelID: "hf:test", BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func callOnce(p Provider) error {
	_, err := p.Call(context.Background(), LLMRequest{Messages: []Message{{Role: "user", Content: []Block{{Type: "text", Text: "hi"}}}}})
	return err
}

func TestSynthetic429Classification(t *testing.T) {
	for name, tc := range map[string]struct {
		body, quotas string
		quota        bool
	}{
		"subscription limit body": {`{"error":"You've exceeded your subscription rate limits. Upgrade, or try again later"}`, "", true},
		"weekly plan at zero":     {`{"error":"rate limited"}`, `{"subscription":{"limit":135,"requests":3},"planWeeklyLimit":{"percentRemaining":0,"nextRegenAt":"2026-11-05T00:00:00Z"}}`, true},
		"concurrency":             {`{"error":"Too many concurrent requests"}`, `{"planWeeklyLimit":{"percentRemaining":42.5}}`, false},
		"unknown body":            {`{"error":"slow down"}`, "", false},
	} {
		t.Run(name, func(t *testing.T) {
			err := callOnce(synthetic429Server(t, tc.body, tc.quotas))
			if err == nil {
				t.Fatal("429 succeeded")
			}
			if got := IsQuotaExhausted(err); got != tc.quota {
				t.Fatalf("IsQuotaExhausted = %v, want %v (err %v)", got, tc.quota, err)
			}
			if got := errors.Is(err, ErrUpstreamConcurrencyLimit); got == tc.quota {
				t.Fatalf("concurrency typed = %v for quota=%v (err %v)", got, tc.quota, err)
			}
			if !strings.Contains(err.Error(), "status 429") {
				t.Fatalf("err = %v, want the sanitized status", err)
			}
			if tc.quota && !strings.Contains(err.Error(), "quota exhausted") {
				t.Fatalf("err = %v, want the quota named for the guest", err)
			}
			if !tc.quota && !strings.Contains(err.Error(), "concurrency limit") {
				t.Fatalf("err = %v, want the concurrency limit named for the guest", err)
			}
		})
	}
}

func chatgpt429Provider(t *testing.T, body string) *ChatGPTProvider {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		fmt.Fprint(w, body)
	}))
	t.Cleanup(srv.Close)
	authPath := writeTestCodexAuth(t, testJWTWithExpiry(t, time.Now().Add(time.Hour)), "refresh", time.Now())
	p, err := NewChatGPTProvider(ChatGPTConfig{ModelID: "gpt-6-luna", BaseURL: srv.URL + "/responses", AuthPath: authPath})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev, flags := log.Writer(), log.Flags()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(prev); log.SetFlags(flags) })
	return &buf
}

func TestChatGPT429ReasonIsLoggedAndQuotaTyped(t *testing.T) {
	logs := captureLog(t)
	p := chatgpt429Provider(t, `{"error":{"type":"usage_limit_reached","message":"The usage limit has been reached (key=sk-abcdefghijklmnop)"}}`)
	err := callOnce(p)
	if !IsQuotaExhausted(err) || !strings.Contains(err.Error(), "chatgpt: quota exhausted: status 429") {
		t.Fatalf("err = %v, want a typed chatgpt quota", err)
	}
	out := logs.String()
	if !strings.Contains(out, "chatgpt rejected request status=429") || !strings.Contains(out, "usage_limit_reached") {
		t.Fatalf("log = %q, want the ChatGPT 429 reason", out)
	}
	if strings.Contains(out, "sk-abcdefghijklmnop") {
		t.Fatalf("log leaked a secret: %q", out)
	}
}

func TestChatGPTPlainRateLimitStaysUntyped(t *testing.T) {
	captureLog(t)
	err := callOnce(chatgpt429Provider(t, `{"error":{"type":"rate_limit_exceeded","message":"Too many requests, slow down"}}`))
	if err == nil || IsQuotaExhausted(err) || !strings.Contains(err.Error(), "chatgpt: status 429") {
		t.Fatalf("err = %v, want the untyped sanitized 429", err)
	}
}

// The quota's reset time travels on its typed error
// (docs/problems/provider-quota-kills-desk-loops-2026-11-04.md): Retry-After
// first, then Synthetic's nextRegenAt; a past or absurd time is no time, and
// a Synthetic 429 that names a reset time is a limit to wait out, not a
// concurrency burst.
func TestQuotaCarriesItsResetTime(t *testing.T) {
	future := time.Now().Add(32 * time.Minute).UTC().Truncate(time.Second)
	retryAfter := func(secs string) http.Header { h := http.Header{}; h.Set("Retry-After", secs); return h }
	for name, tc := range map[string]struct {
		body    string
		headers http.Header
		want    func(time.Time) bool
		quota   bool
	}{
		"retry-after seconds": {`{"error":"You've exceeded your subscription rate limits"}`, retryAfter("120"),
			func(at time.Time) bool { d := time.Until(at); return d > 110*time.Second && d <= 120*time.Second }, true},
		"next regen": {`{"error":"You've exceeded your subscription rate limits"}`,
			http.Header{"X-Synthetic-Quotas": {fmt.Sprintf(`{"planWeeklyLimit":{"percentRemaining":0,"nextRegenAt":%q}}`, future.Format(time.RFC3339))}},
			func(at time.Time) bool { return at.Equal(future) }, true},
		"subscription window when the weekly plan has room": {`{"error":"You've exceeded your subscription rate limits"}`,
			http.Header{"X-Synthetic-Quotas": {fmt.Sprintf(`{"subscription":{"limit":500,"requests":500,"renewsAt":%q},"planWeeklyLimit":{"percentRemaining":40,"nextRegenAt":%q}}`,
				future.Add(-20*time.Minute).Format(time.RFC3339), future.Add(48*time.Hour).Format(time.RFC3339))}},
			func(at time.Time) bool { return at.Equal(future.Add(-20 * time.Minute)) }, true},
		"retry-after zero is no time": {`{"error":"You've exceeded your subscription rate limits"}`, retryAfter("0"),
			func(at time.Time) bool { return at.IsZero() }, true},
		"past regen is no time": {`{"error":"You've exceeded your subscription rate limits"}`,
			http.Header{"X-Synthetic-Quotas": {`{"planWeeklyLimit":{"percentRemaining":0,"nextRegenAt":"2020-01-01T00:00:00Z"}}`}},
			func(at time.Time) bool { return at.IsZero() }, true},
		"absurd retry-after is no time": {`{"error":"You've exceeded your subscription rate limits"}`, retryAfter("99999999"),
			func(at time.Time) bool { return at.IsZero() }, true},
		"unknown body with retry-after is a limit": {`{"error":"slow down"}`, retryAfter("60"),
			func(at time.Time) bool { return !at.IsZero() }, true},
		"concurrency with retry-after stays concurrency": {`{"error":"Too many concurrent requests"}`, retryAfter("5"), nil, false},
	} {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				for k, v := range tc.headers {
					w.Header()[k] = v
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				fmt.Fprint(w, tc.body)
			}))
			t.Cleanup(srv.Close)
			p, err := NewSyntheticProvider(SyntheticConfig{APIKey: "syn-test-key", ModelID: "hf:test", BaseURL: srv.URL})
			if err != nil {
				t.Fatal(err)
			}
			err = callOnce(p)
			q, ok := AsQuotaExhausted(err)
			if ok != tc.quota {
				t.Fatalf("typed quota = %v, want %v (err %v)", ok, tc.quota, err)
			}
			if ok && (q.Provider != "synthetic" || !tc.want(q.RetryAt)) {
				t.Fatalf("quota = %+v, unexpected reset time", q)
			}
		})
	}
}
