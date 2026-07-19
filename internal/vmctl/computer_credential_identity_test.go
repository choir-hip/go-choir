package vmctl

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStartExistingVMBindsCredentialToStableComputerAndRealization(t *testing.T) {
	var issued map[string]string
	corpusd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/computers/credentials/issue" || r.Header.Get("X-Internal-Caller") != "true" {
			t.Fatalf("credential request = %s %s headers=%v", r.Method, r.URL.Path, r.Header)
		}
		if err := json.NewDecoder(r.Body).Decode(&issued); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"envelope":{"schema_version":1,"computer_id":"computer-stable","realization_id":"vm-realization"}}`))
	}))
	defer corpusd.Close()

	registry := NewOwnershipRegistry("")
	registry.SetCorpusdURL(corpusd.URL)
	manager := &mockVMManager{resumeError: errors.New("not running")}
	ownership := &VMOwnership{VMID: "vm-realization", ComputerID: "computer-stable", DesktopID: "primary", UserID: "owner-1", Epoch: 7}
	if _, err := registry.startExistingVM(ownership, manager); err != nil {
		t.Fatal(err)
	}
	if issued["computer_id"] != ownership.ComputerID || issued["realization_id"] != "vm-realization-epoch-8" || issued["idempotency_key"] != "guest-credential:vm-realization-epoch-8:8" {
		t.Fatalf("issued credential identity = %#v", issued)
	}
	if len(manager.boots) != 1 || manager.boots[0].ComputerCredentialEnvelope == "" || manager.boots[0].DesktopID != ownership.DesktopID {
		t.Fatalf("boot config = %#v", manager.boots)
	}
}
