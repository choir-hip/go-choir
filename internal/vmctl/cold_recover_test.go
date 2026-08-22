package vmctl

import (
	"bytes"
	"context"
	"encoding/json"
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

func TestClientColdRecoverUsesOnlyProtocolFields(t *testing.T) {
	var received ColdRecoverRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/internal/vmctl/computers/computer-1/cold-recover" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&received); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusAccepted)
		if err := json.NewEncoder(w).Encode(ColdRecoverResponse{RecoveryID: "rec-1-a", Status: "rematerializing"}); err != nil {
			t.Fatal(err)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL)
	result, err := client.ColdRecover(context.Background(), "computer-1", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", 7, "key-1")
	if err != nil {
		t.Fatal(err)
	}
	if result.RecoveryID != "rec-1-a" || result.Status != "rematerializing" {
		t.Fatalf("response = %#v", result)
	}
	if received.ComputerID != "computer-1" || received.ExpectedRouteGeneration != 7 || received.IdempotencyKey != "key-1" {
		t.Fatalf("request = %#v", received)
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
