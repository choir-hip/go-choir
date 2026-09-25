package vmctl

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestColdRecoverRejectsUnknownFieldsBeforeStateChange(t *testing.T) {
	for _, field := range []string{"owner_id", "checkpoint_digest", "authorization_ref", "mode"} {
		t.Run(field, func(t *testing.T) {
			handler := NewHandler(NewOwnershipRegistry(""))
			mux := http.NewServeMux()
			mux.HandleFunc("/internal/vmctl/computers/{computerID}/cold-recover", handler.HandleColdRecover)

			body := `{"computer_id":"computer-1","expected_canonical_head":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","expected_route_generation":1,"idempotency_key":"key-1","` + field + `":"value"}`
			request := httptest.NewRequest(http.MethodPost, "/internal/vmctl/computers/computer-1/cold-recover", bytes.NewBufferString(body))
			request.Header.Set("X-Internal-Caller", "true")
			response := httptest.NewRecorder()
			mux.ServeHTTP(response, request)

			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusBadRequest, response.Body.String())
			}
			if handler.coldRecoveryState != nil {
				t.Fatal("unknown request field initialized cold recovery state")
			}
		})
	}
}

func TestRecoveryLeaseIsComputerAndGenerationBoundAndSingleUse(t *testing.T) {
	token := RecoveryFencingToken{
		ComputerID: "computer-a", RecoveryGeneration: 4,
		CanonicalHead: "head-a",
		Expiry:        time.Now().Add(time.Minute).UTC().Format(time.RFC3339Nano),
	}
	lease := &RecoveryLease{token: token}
	if lease.AllowAppend("computer-b", 4, "head-a") {
		t.Fatal("cross-computer append was allowed")
	}
	if lease.AllowAppend("computer-a", 3, "head-a") {
		t.Fatal("stale recovery generation was allowed")
	}
	if lease.AllowAppend("computer-a", 4, "head-b") {
		t.Fatal("head mismatch was allowed")
	}
	if !lease.AllowAppend("computer-a", 4, "head-a") {
		t.Fatal("matching boot append was refused")
	}
	if lease.AllowAppend("computer-a", 4, "head-a") {
		t.Fatal("single-use lease was reused")
	}
}

func TestRecoveryLeaseExpires(t *testing.T) {
	lease := &RecoveryLease{token: RecoveryFencingToken{
		ComputerID: "computer-a", RecoveryGeneration: 1,
		CanonicalHead: "head-a",
		Expiry:        time.Now().Add(-time.Second).UTC().Format(time.RFC3339Nano),
	}}
	if lease.AllowAppend("computer-a", 1, "head-a") {
		t.Fatal("expired recovery lease was allowed")
	}
}
