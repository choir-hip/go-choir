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

// Intake labels have their own rate scope (L2a R2-3,
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

func TestIntakeLabelJudgmentsHaveTheirOwnRateScope(t *testing.T) {
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
	// The limit is one request per bucket per minute.
	if code := judge(""); code != http.StatusOK {
		t.Fatalf("first support judgment = %d", code)
	}
	if code := judge(JudgmentScopeIntakeLabels); code != http.StatusOK {
		t.Fatalf("an intake label after a support judgment = %d; the support bucket starved labels", code)
	}
	if code := judge(JudgmentScopeIntakeLabels); code != http.StatusTooManyRequests {
		t.Fatalf("second intake label = %d; want the labels bucket limited", code)
	}
	if code := judge(""); code != http.StatusTooManyRequests {
		t.Fatalf("second support judgment = %d; want the support bucket limited", code)
	}
	if code := judge("bulk"); code != http.StatusBadRequest {
		t.Fatalf("unknown scope = %d; want refused", code)
	}

	// Labels exhausting their bucket never starve a support judgment.
	h2, reg2, _ := setupHandlerWithRateLimit(t, 1, time.Minute)
	h2.SetJevTransport(h.jevTransport)
	cred2, _ := reg2.IssueCredential("autoputer-labels-2")
	h, cred = h2, cred2
	_ = judge(JudgmentScopeIntakeLabels)
	_ = judge(JudgmentScopeIntakeLabels)
	if code := judge(""); code != http.StatusOK {
		t.Fatalf("support judgment after labels used their bucket = %d", code)
	}
	for _, body := range upstream.bodies {
		if strings.Contains(body, "scope") || strings.Contains(body, JudgmentScopeIntakeLabels) {
			t.Fatalf("the scope rode upstream: %s", body)
		}
	}
}
