package vmctl

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// Deploy refresh through the guest's planned-restart marker
// (docs/problems/owner-computer-pinned-as-canary-2026-10-13.md). Owner rule:
// deploys never restart a busy computer; a planned restart for an update may
// resume work, keyed by a marker written before the restart.
//
// Ways this could fail, written first:
//   - vmctl kills the VM before the marker is durable (or without asking),
//     so the deploy boot is a crash that closes parked work;
//   - a busy guest (409) is refreshed anyway, or its refusal is reported as
//     a deploy failure instead of a deferral;
//   - a held computer is contacted or refreshed;
//   - the selection still drops constructed computers or always-on ones, so
//     the owner's computer never takes a deploy;
//   - a guest that cannot be asked (unreachable) is killed blind;
//   - a guest that predates the endpoint can never be updated (the first
//     deploy that ships the endpoint could not reach anyone), or is refreshed
//     while its /health reports work;
//   - a marker written for a refresh that then failed stays behind and turns
//     a later crash into a resume;
//   - the operator refresh endpoint starts depending on the marker (it must
//     keep its forced, crash-boot behavior).

func TestDeployRefreshSelectionIncludesConstructedAlwaysOnAndSkipsHeld(t *testing.T) {
	ownerships := []ownershipResponse{
		{VMID: "vm-owner", UserID: "owner", DesktopID: "primary", State: "active", Kind: VMKindInteractive,
			WarmnessClass: string(WarmnessClassPremiumAlwaysOn), SnapshotKind: "constructed-computer-version", DivergenceStatus: "canary"},
		{VMID: "vm-plain", UserID: "plain", DesktopID: "primary", State: "active"},
		{VMID: "vm-held", UserID: "held", DesktopID: "primary", State: "active", Kind: VMKindInteractive, Held: true},
		{VMID: "vm-stopped", UserID: "stopped", DesktopID: "primary", State: "stopped", Kind: VMKindInteractive},
		{VMID: "vm-worker", UserID: "worker", DesktopID: "primary", State: "active", Kind: VMKind("worker")},
	}
	plan := selectDeployRefresh(ownerships)
	got := map[string]bool{}
	for _, c := range plan.Candidates {
		got[c.VMID] = true
	}
	if !got["vm-owner"] {
		t.Fatalf("constructed premium_always_on computer not selected: %+v", plan)
	}
	if !got["vm-plain"] {
		t.Fatalf("ordinary active computer not selected: %+v", plan)
	}
	if got["vm-held"] || got["vm-stopped"] || got["vm-worker"] || len(plan.Candidates) != 2 {
		t.Fatalf("selection = %+v, want only vm-owner and vm-plain", plan.Candidates)
	}
	if len(plan.Skipped) != 1 || plan.Skipped[0].VMID != "vm-held" || plan.Skipped[0].Reason != "held" {
		t.Fatalf("skipped = %+v, want vm-held skipped as held", plan.Skipped)
	}
}

type fakeGuestMarker struct {
	mu              sync.Mutex
	outcome         guestMarkOutcome
	detail          string
	marks           []string
	clears          int
	refreshesAtMark []int
	mgr             *mockVMManager
}

func (f *fakeGuestMarker) mark(_ context.Context, own VMOwnership, target string) (guestMarkOutcome, string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.marks = append(f.marks, own.VMID+"@"+target)
	if f.mgr != nil {
		f.mgr.mu.Lock()
		f.refreshesAtMark = append(f.refreshesAtMark, len(f.mgr.refreshes))
		f.mgr.mu.Unlock()
	}
	return f.outcome, f.detail
}

func (f *fakeGuestMarker) clear(context.Context, VMOwnership) {
	f.mu.Lock()
	f.clears++
	f.mu.Unlock()
}

func newDeployRefreshFixture(t *testing.T, user string) (*Handler, *mockVMManager, *fakeGuestMarker) {
	t.Helper()
	mgr := &mockVMManager{}
	reg := NewOwnershipRegistry("")
	reg.SetCorpusdURL(testComputerCredentialIssuerURL(t))
	reg.SetVMManager(mgr)
	if _, err := reg.ResolveOrAssign(user); err != nil {
		t.Fatal(err)
	}
	h := NewHandler(reg)
	fake := &fakeGuestMarker{mgr: mgr}
	h.guestMarker = fake
	return h, mgr, fake
}

func postDeployRefresh(t *testing.T, h *Handler, body string) (int, deployRefreshResponse) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/internal/vmctl/deploy-refresh", bytes.NewBufferString(body))
	req.Header.Set("X-Internal-Caller", "true")
	rec := httptest.NewRecorder()
	h.HandleDeployRefresh(rec, req)
	var resp deployRefreshResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode %d %s: %v", rec.Code, rec.Body.String(), err)
	}
	return rec.Code, resp
}

func TestDeployRefreshDefersBusyComputer(t *testing.T) {
	h, mgr, fake := newDeployRefreshFixture(t, "owner-busy")
	fake.outcome, fake.detail = guestMarkBusy, "running_runs=1"
	code, resp := postDeployRefresh(t, h, `{"user_id":"owner-busy","desktop_id":"primary","target":"761d94c5"}`)
	if code != http.StatusConflict || resp.Outcome != deployRefreshDeferred || !strings.Contains(resp.Reason, "busy") {
		t.Fatalf("busy guest: %d %+v, want 409 deferred busy", code, resp)
	}
	if len(mgr.refreshes) != 0 {
		t.Fatalf("busy guest was refreshed: %v", mgr.refreshes)
	}
}

func TestDeployRefreshMarksThenRefreshesIdleComputer(t *testing.T) {
	h, mgr, fake := newDeployRefreshFixture(t, "owner-idle")
	fake.outcome = guestMarkWritten
	code, resp := postDeployRefresh(t, h, `{"user_id":"owner-idle","desktop_id":"primary","target":"761d94c5"}`)
	if code != http.StatusOK || resp.Outcome != deployRefreshRefreshed || !resp.PlannedRestart {
		t.Fatalf("idle guest: %d %+v, want 200 refreshed with planned_restart", code, resp)
	}
	if len(fake.marks) != 1 || !strings.HasSuffix(fake.marks[0], "@761d94c5") {
		t.Fatalf("marks = %v, want one for target 761d94c5", fake.marks)
	}
	if fake.refreshesAtMark[0] != 0 || len(mgr.refreshes) != 1 {
		t.Fatalf("order: refreshes at mark=%v, after=%d; the marker must precede the one refresh", fake.refreshesAtMark, len(mgr.refreshes))
	}
	if fake.clears != 0 {
		t.Fatalf("a successful refresh cleared its marker")
	}
}

func TestDeployRefreshClearsMarkerWhenRefreshFails(t *testing.T) {
	h, mgr, fake := newDeployRefreshFixture(t, "owner-refresh-fails")
	fake.outcome = guestMarkWritten
	mgr.refreshError = errors.New("firecracker refused")
	code, resp := postDeployRefresh(t, h, `{"user_id":"owner-refresh-fails","desktop_id":"primary","target":"761d94c5"}`)
	if code < 400 || resp.Outcome != deployRefreshFailed {
		t.Fatalf("failed refresh: %d %+v, want an error outcome failed", code, resp)
	}
	if fake.clears != 1 {
		t.Fatalf("clears = %d, want the marker cleared after a failed refresh", fake.clears)
	}
}

func TestDeployRefreshSkipsHeldComputerWithoutContactingGuest(t *testing.T) {
	h, mgr, fake := newDeployRefreshFixture(t, "owner-held")
	own := h.registry.GetOwnershipForDesktop("owner-held", PrimaryDesktopID)
	if err := h.registry.SetHold(own.ComputerID, "maintenance", "recovery"); err != nil {
		t.Fatal(err)
	}
	fake.outcome = guestMarkWritten
	code, resp := postDeployRefresh(t, h, `{"user_id":"owner-held","desktop_id":"primary","target":"761d94c5"}`)
	if code != http.StatusConflict || resp.Outcome != deployRefreshSkipped || resp.Reason != "held" {
		t.Fatalf("held: %d %+v, want 409 skipped held", code, resp)
	}
	if len(fake.marks) != 0 || len(mgr.refreshes) != 0 {
		t.Fatalf("held computer contacted (%v) or refreshed (%v)", fake.marks, mgr.refreshes)
	}
}

func TestDeployRefreshUnreachableGuestIsDeferred(t *testing.T) {
	h, mgr, fake := newDeployRefreshFixture(t, "owner-unreachable")
	fake.outcome, fake.detail = guestMarkUnavailable, "dial tcp: connection refused"
	code, resp := postDeployRefresh(t, h, `{"user_id":"owner-unreachable","desktop_id":"primary","target":"761d94c5"}`)
	if code != http.StatusConflict || resp.Outcome != deployRefreshDeferred {
		t.Fatalf("unreachable: %d %+v, want 409 deferred", code, resp)
	}
	if len(mgr.refreshes) != 0 {
		t.Fatalf("unreachable guest killed blind: %v", mgr.refreshes)
	}
}

func TestDeployRefreshLegacyGuestUsesIdleProbe(t *testing.T) {
	h, mgr, fake := newDeployRefreshFixture(t, "owner-legacy")
	fake.outcome = guestMarkUnsupported
	h.registry.setGuestBusyProbeForTest(func(*VMOwnership) bool { return true })
	code, resp := postDeployRefresh(t, h, `{"user_id":"owner-legacy","desktop_id":"primary","target":"761d94c5"}`)
	if code != http.StatusConflict || resp.Outcome != deployRefreshDeferred || len(mgr.refreshes) != 0 {
		t.Fatalf("busy legacy guest: %d %+v refreshes=%v, want deferred, not refreshed", code, resp, mgr.refreshes)
	}
	h.registry.setGuestBusyProbeForTest(func(*VMOwnership) bool { return false })
	code, resp = postDeployRefresh(t, h, `{"user_id":"owner-legacy","desktop_id":"primary","target":"761d94c5"}`)
	if code != http.StatusOK || resp.Outcome != deployRefreshRefreshed || resp.PlannedRestart || len(mgr.refreshes) != 1 {
		t.Fatalf("idle legacy guest: %d %+v refreshes=%v, want refreshed as a crash boot (planned_restart=false)", code, resp, mgr.refreshes)
	}
}

func TestOperatorRefreshDoesNotAskForMarker(t *testing.T) {
	h, mgr, fake := newDeployRefreshFixture(t, "owner-operator")
	fake.outcome = guestMarkBusy
	req := httptest.NewRequest(http.MethodPost, "/internal/vmctl/refresh", bytes.NewBufferString(`{"user_id":"owner-operator","desktop_id":"primary"}`))
	req.Header.Set("X-Internal-Caller", "true")
	rec := httptest.NewRecorder()
	h.HandleRefresh(rec, req)
	if rec.Code != http.StatusOK || len(mgr.refreshes) != 1 {
		t.Fatalf("operator refresh: %d %s refreshes=%v, want the forced refresh to keep working", rec.Code, rec.Body.String(), mgr.refreshes)
	}
	if len(fake.marks) != 0 {
		t.Fatalf("operator refresh asked the guest for a marker: %v", fake.marks)
	}
}

// The real host->guest client classifies the guest's answers.
func TestHTTPGuestMarkerClassifiesGuestAnswers(t *testing.T) {
	for _, c := range []struct {
		status int
		want   guestMarkOutcome
	}{
		{http.StatusOK, guestMarkWritten},
		{http.StatusConflict, guestMarkBusy},
		{http.StatusNotFound, guestMarkUnsupported},
		{http.StatusMethodNotAllowed, guestMarkUnsupported},
		{http.StatusServiceUnavailable, guestMarkUnavailable},
		{http.StatusForbidden, guestMarkUnavailable},
	} {
		var gotPath, gotTarget string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.Method + " " + r.URL.Path
			var body struct {
				Target string `json:"target"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			gotTarget = body.Target
			w.WriteHeader(c.status)
			_, _ = w.Write([]byte(`{}`))
		}))
		outcome, _ := httpGuestMarker{client: server.Client()}.mark(context.Background(), VMOwnership{ComputerURL: server.URL}, "761d94c5")
		server.Close()
		if outcome != c.want || gotPath != "POST /internal/runtime/planned-restart" || gotTarget != "761d94c5" {
			t.Fatalf("status %d: outcome=%v path=%q target=%q, want %v", c.status, outcome, gotPath, gotTarget, c.want)
		}
	}
	outcome, _ := httpGuestMarker{}.mark(context.Background(), VMOwnership{ComputerURL: "http://127.0.0.1:1"}, "761d94c5")
	if outcome != guestMarkUnavailable {
		t.Fatalf("unreachable guest outcome = %v, want unavailable", outcome)
	}
}
