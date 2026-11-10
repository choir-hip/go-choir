package gateway

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Scopes name priority classes (intake labels were L2a R2-3,
// docs/archive/reports/l2a-synthesis-intake-design-2026-10-14.md). Ways it can fail,
// written before the code:
//   - labels draw on the shared judgments bucket, so a poll of many new
//     captures starves the support judge publication needs;
//   - support judgments starve labels the same way (the buckets must be
//     independent both ways);
//   - an unknown scope is accepted and mints a fresh, unlimited bucket per
//     spelling;
//   - the scope rides upstream to OpenRouter.

type decisionsRoundTripper struct {
	bodies []string
}

func (rt *decisionsRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	body, _ := io.ReadAll(r.Body)
	rt.bodies = append(rt.bodies, string(body))
	return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}},
		Body:    io.NopCloser(strings.NewReader(`{"model":"typesafe/jev-1.13","answers":{"q":{"choice":"yes"}}}`)),
		Request: r}, nil
}

func TestJudgmentScopesAreClassesNotBuckets(t *testing.T) {
	// The handler's own per-computer bucket is one request a minute; Jev no
	// longer draws on it (the shared limiter replaced it).
	h, reg, _ := setupHandlerWithRateLimit(t, 1, time.Minute)
	upstream := &decisionsRoundTripper{}
	h.SetJevTransport(&JevTransport{enabled: true, apiKey: "test-key", httpClient: &http.Client{Transport: upstream},
		receipts: newJevReceiptWriter(filepath.Join(t.TempDir(), "receipts.jsonl"))})
	cred, err := reg.IssueCredential("autoputer-labels")
	if err != nil {
		t.Fatal(err)
	}
	judge := func(scope string) int {
		t.Helper()
		body, _ := json.Marshal(map[string]any{
			"state":     map[string]any{"capture": "x"},
			"questions": map[string]any{"q": map[string]any{"type": "choice", "choices": []string{"yes", "no"}}},
			"scope":     scope,
		})
		req := httptest.NewRequest(http.MethodPost, "/provider/v1/judgments", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+cred.RawToken)
		rec := httptest.NewRecorder()
		h.HandleJudgment(rec, req)
		return rec.Code
	}
	for i := 0; i < 5; i++ {
		for _, scope := range []string{"", JudgmentScopeIntakeLabels, JudgmentScopeInteractive} {
			if code := judge(scope); code != http.StatusOK {
				t.Fatalf("judgment %d scope %q = %d; the old per-computer bucket still applies", i, scope, code)
			}
		}
	}
	if code := judge("bulk"); code != http.StatusBadRequest {
		t.Fatalf("unknown scope = %d; want refused", code)
	}
	for _, body := range upstream.bodies {
		if strings.Contains(body, "scope") || strings.Contains(body, JudgmentScopeIntakeLabels) || strings.Contains(body, JudgmentScopeInteractive) {
			t.Fatalf("the scope rode upstream: %s", body)
		}
	}
}
