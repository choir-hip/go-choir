package vmctl

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// S3 re-credential on resume, vmctl side: the failure list is in the header
// of snapshot_hibernate_test.go (items 26-28 of the design note).

// resumeCorpusd records credential issuance requests; status overrides 201.
type resumeCorpusd struct {
	mu       sync.Mutex
	requests []map[string]string
	status   int
}

func (c *resumeCorpusd) server(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/computers/credentials/issue" || r.Header.Get("X-Internal-Caller") != "true" {
			http.Error(w, "unexpected", http.StatusBadRequest)
			return
		}
		var request map[string]string
		_ = json.NewDecoder(r.Body).Decode(&request)
		c.mu.Lock()
		c.requests = append(c.requests, request)
		status := c.status
		c.mu.Unlock()
		if status != 0 {
			w.WriteHeader(status)
			return
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"envelope": map[string]any{
			"computer_id": request["computer_id"], "realization_id": request["realization_id"], "nonce": request["idempotency_key"],
		}})
	}))
	t.Cleanup(srv.Close)
	return srv
}

func (c *resumeCorpusd) snapshot() []map[string]string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]map[string]string(nil), c.requests...)
}

func TestSnapshotResumeIssuesFreshCredentialForSameRealization(t *testing.T) {
	mock := &snapshotMockVMManager{mockVMManager: &mockVMManager{getVMs: make(map[string]*VMInstanceInfo)}, callIssuer: true}
	reg, initial := newSnapshotTestRegistry(t, mock, "user-snap-cred")
	corpusd := &resumeCorpusd{}
	reg.SetCorpusdURL(corpusd.server(t).URL)
	mock.snapshotResp = &VMInstanceInfo{HostURL: initial.ComputerURL, Epoch: initial.Epoch, Healthy: true, State: "running"}
	bootsBefore := len(mock.boots)

	resumed, err := reg.ResumeVM("user-snap-cred")
	if err != nil {
		t.Fatalf("ResumeVM: %v", err)
	}
	if resumed.Epoch != initial.Epoch || len(mock.boots) != bootsBefore {
		t.Fatalf("snapshot resume moved the epoch %d -> %d or cold booted", initial.Epoch, resumed.Epoch)
	}
	if len(mock.issued) != 1 || mock.issued[0] == "" || mock.issueErrs[0] != nil {
		t.Fatalf("issued envelopes = %v (%v), want one", mock.issued, mock.issueErrs)
	}
	requests := corpusd.snapshot()
	if len(requests) != 1 {
		t.Fatalf("issuance requests = %d, want 1", len(requests))
	}
	realization := realizationIDFor(initial.VMID, initial.Epoch)
	prefix := fmt.Sprintf("guest-credential-resume:%s:%d:", realization, initial.Epoch)
	if requests[0]["computer_id"] != initial.ComputerID || requests[0]["realization_id"] != realization || !strings.HasPrefix(requests[0]["idempotency_key"], prefix) {
		t.Fatalf("resume issuance = %v, want computer %s realization %s key %s*", requests[0], initial.ComputerID, realization, prefix)
	}
	if len(requests[0]["idempotency_key"]) < len(prefix)+16 {
		t.Fatalf("resume key %q carries no per-attempt nonce", requests[0]["idempotency_key"])
	}
}

func TestSnapshotResumeCredentialKeyPerAttempt(t *testing.T) {
	mock := &snapshotMockVMManager{mockVMManager: &mockVMManager{getVMs: make(map[string]*VMInstanceInfo)}}
	reg, _ := newSnapshotTestRegistry(t, mock, "user-snap-keys")
	corpusd := &resumeCorpusd{}
	reg.SetCorpusdURL(corpusd.server(t).URL)
	own := reg.GetOwnershipForDesktop("user-snap-keys", PrimaryDesktopID)
	first := reg.resumeCredentialIssuer(own)
	second := reg.resumeCredentialIssuer(own)
	for _, issue := range []func() (string, error){first, first, second} {
		if _, err := issue(); err != nil {
			t.Fatal(err)
		}
	}
	requests := corpusd.snapshot()
	if requests[0]["idempotency_key"] != requests[1]["idempotency_key"] {
		t.Fatal("a retry inside one resume attempt changed the key (a timed-out issuance would be duplicated)")
	}
	if requests[0]["idempotency_key"] == requests[2]["idempotency_key"] {
		t.Fatal("two resume attempts share one key (the second would replay a consumed envelope)")
	}
}

func TestSnapshotResumeIssuerRefusalIsFinal(t *testing.T) {
	mock := &snapshotMockVMManager{mockVMManager: &mockVMManager{getVMs: make(map[string]*VMInstanceInfo)}}
	reg, _ := newSnapshotTestRegistry(t, mock, "user-snap-refused")
	corpusd := &resumeCorpusd{status: http.StatusConflict}
	reg.SetCorpusdURL(corpusd.server(t).URL)
	issue := reg.resumeCredentialIssuer(reg.GetOwnershipForDesktop("user-snap-refused", PrimaryDesktopID))
	if issue == nil {
		t.Fatal("no issuer although corpusd is configured")
	}
	if envelope, err := issue(); err == nil || envelope != "" {
		t.Fatalf("refused issuance = %q, %v; want an error", envelope, err)
	}
	if n := len(corpusd.snapshot()); n != 1 {
		t.Fatalf("an explicit refusal was retried (%d requests)", n)
	}
}

func TestSnapshotResumeWithoutCorpusdOffersNoIssuer(t *testing.T) {
	mock := &snapshotMockVMManager{mockVMManager: &mockVMManager{getVMs: make(map[string]*VMInstanceInfo)}}
	reg, initial := newSnapshotTestRegistry(t, mock, "user-snap-nocorpusd")
	mock.snapshotResp = &VMInstanceInfo{HostURL: initial.ComputerURL, Epoch: initial.Epoch, Healthy: true, State: "running"}
	if _, err := reg.ResumeVM("user-snap-nocorpusd"); err != nil {
		t.Fatal(err)
	}
	if len(mock.issuers) != 1 || mock.issuers[0] != nil {
		t.Fatal("an issuer was offered although vmctl has no corpusd URL")
	}
}
