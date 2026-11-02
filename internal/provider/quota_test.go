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
