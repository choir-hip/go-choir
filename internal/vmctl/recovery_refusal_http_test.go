package vmctl

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestHandleResolveReturnsStructured503ForDurableRefusal locks the
// product-visible admission contract: a known blocked computer fails promptly
// with a structured 503, Retry-After: 60, the reason, the input witness and
// the repair job status — and never launches a VM.
func TestHandleResolveReturnsStructured503ForDurableRefusal(t *testing.T) {
	corpusd := &recoveryEvidenceFake{headSequence: 572151, watermarkSequence: 148431, baseRef: "base-stale"}
	corpusdServer := httptest.NewServer(corpusd)
	t.Cleanup(corpusdServer.Close)
	statePath := filepath.Join(t.TempDir(), "recovery-conditions.json")
	writeRecoveryConditions(t, statePath, &RecoveryCondition{
		ComputerID: "computer-blocked",
		OwnerID:    "user-blocked",
		Kind:       RecoveryRefusalTailExcess,
		Reason:     "recovery tail 423720 events exceeds 10000; publish a fresher base (local=0 W=148431 H=572151)",
		Witness: RecoveryInputWitness{
			LocalSequence:     0,
			WatermarkSequence: 148431,
			TargetSequence:    572151,
			BaseRef:           "base-stale",
			EmptyStore:        true,
			ChainExists:       true,
			HasLocalWitness:   true,
		},
		ObservedAt: time.Now().UTC().Add(-time.Hour),
	})

	reg := newRecoveryAdmissionRegistry(t, statePath, corpusdServer.URL, "")
	mgr := &mockVMManager{}
	reg.SetVMManager(mgr)
	seedRecoveryOwnership(reg, "user-blocked", "computer-blocked", VMStateStopped)
	handler := NewHandler(reg)

	request := httptest.NewRequest(http.MethodPost, "/internal/vmctl/resolve",
		strings.NewReader(`{"user_id":"user-blocked","desktop_id":"primary"}`))
	request.Header.Set("X-Internal-Caller", "true")
	recorder := httptest.NewRecorder()
	start := time.Now()
	handler.HandleResolve(recorder, request)
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("known blocked resolve took %s, want prompt refusal", elapsed)
	}

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503; body=%s", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Header().Get("Retry-After"); got != "60" {
		t.Fatalf("Retry-After = %q, want 60", got)
	}
	var body struct {
		Error             string                `json:"error"`
		Reason            string                `json:"reason"`
		Kind              string                `json:"kind"`
		RetryAfterSeconds int                   `json:"retry_after_seconds"`
		Witness           *RecoveryInputWitness `json:"witness"`
		Repair            *CheckpointJobStatus  `json:"repair"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode refusal body: %v body=%s", err, recorder.Body.String())
	}
	if body.Error != "computer recovery blocked" {
		t.Fatalf("error = %q", body.Error)
	}
	if body.Kind != string(RecoveryRefusalTailExcess) {
		t.Fatalf("kind = %q, want %q", body.Kind, RecoveryRefusalTailExcess)
	}
	if body.RetryAfterSeconds != recoveryRetryAfterSeconds {
		t.Fatalf("retry_after_seconds = %d, want %d", body.RetryAfterSeconds, recoveryRetryAfterSeconds)
	}
	if body.Witness == nil || body.Witness.WatermarkSequence != 148431 || body.Witness.TargetSequence != 572151 {
		t.Fatalf("witness = %+v", body.Witness)
	}
	if !strings.Contains(body.Reason, "recovery tail 423720 events exceeds 10000") {
		t.Fatalf("reason = %q", body.Reason)
	}
	if len(mgr.boots) != 0 || len(mgr.recovers) != 0 {
		t.Fatalf("blocked resolve launched resources: boots=%d recovers=%d", len(mgr.boots), len(mgr.recovers))
	}
}
