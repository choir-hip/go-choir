package vmctl

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// A credential issuance that fails transiently under host load is retried
// with the same idempotency key (the issuer replays it) instead of failing
// the new computer (docs/problems/texture-zombie-activations-revising-forever-
// 2026-10-09.md, signup failure). Failure modes pinned: no retry after a
// transient 5xx; a retry after an explicit refusal (4xx); the retry changing
// the idempotency key.
func TestComputerCredentialIssueRetriesTransientFailureOnly(t *testing.T) {
	restore := credentialIssueRetryBackoff
	credentialIssueRetryBackoff = func(int) time.Duration { return time.Millisecond }
	t.Cleanup(func() { credentialIssueRetryBackoff = restore })

	var calls atomic.Int32
	keys := make(chan string, 4)
	flaky := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		var body struct {
			IdempotencyKey string `json:"idempotency_key"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		keys <- body.IdempotencyKey
		if n == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"envelope":{"version":1,"computer_id":"c"}}`))
	}))
	defer flaky.Close()
	if got := issueComputerCredentialEnvelope(flaky.URL, "c", "realization-1", 3); got == "" || calls.Load() != 2 {
		t.Fatalf("transient failure: envelope=%q calls=%d, want retried success", got, calls.Load())
	}
	if first, second := <-keys, <-keys; first == "" || first != second {
		t.Fatalf("retry changed the idempotency key: %q then %q", first, second)
	}

	var refusals atomic.Int32
	refusing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		refusals.Add(1)
		w.WriteHeader(http.StatusConflict)
	}))
	defer refusing.Close()
	if got := issueComputerCredentialEnvelope(refusing.URL, "c", "realization-1", 3); got != "" || refusals.Load() != 1 {
		t.Fatalf("refusal: envelope=%q calls=%d, want one call and no envelope", got, refusals.Load())
	}
}
