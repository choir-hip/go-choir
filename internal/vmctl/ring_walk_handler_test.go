package vmctl

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

// The ring walk through vmctl's own paths: the CI plan lists only the open
// ring's computers, a refresh of a held-back computer is skipped, the manager
// pin holds it, and a breached guard at d@e enrolls d@e's return to the
// earlier release while the sweep refreshes nothing else.

type ringManager struct {
	*mockVMManager
	current, commit string
	byVM            map[string]GuestReleaseVMStatus
	pinner          RingWalkPinner
}

func (m *ringManager) GuestReleases() (json.RawMessage, error) { return json.RawMessage(`{}`), nil }
func (m *ringManager) GuestReleaseHealth() (string, map[string]GuestReleaseVMStatus) {
	return m.current, m.byVM
}
func (m *ringManager) CurrentGuestReleaseInfo() (string, string) { return m.current, m.commit }
func (m *ringManager) LayeredGuestReleaseInfo() (string, string) { return "", "" }
func (m *ringManager) GuestLayeredComputer(string) bool          { return false }
func (m *ringManager) GuestReleaseInfoFor(id string) (string, string) {
	if single, _ := m.pinner.Pin(id); single != "" {
		return single, "commit-" + single
	}
	return m.current, m.commit
}

type ringFixture struct {
	h    *Handler
	reg  *OwnershipRegistry
	mgr  *ringManager
	fake *fakeGuestMarker
	de   *VMOwnership
	o3   *VMOwnership
}

func newRingHandlerFixture(t *testing.T) *ringFixture {
	t.Helper()
	mock := &mockVMManager{}
	reg := NewOwnershipRegistry("")
	reg.SetCorpusdURL(testComputerCredentialIssuerURL(t))
	mgr := &ringManager{mockVMManager: mock, current: "r1", commit: "commit-r1", byVM: map[string]GuestReleaseVMStatus{}}
	reg.SetVMManager(mgr)
	f := &ringFixture{reg: reg, mgr: mgr}
	for _, user := range []string{"ring-de", "ring-other"} {
		if _, err := reg.ResolveOrAssign(user); err != nil {
			t.Fatal(err)
		}
	}
	reg.mu.Lock()
	for _, own := range reg.ownerships {
		if own.UserID == "ring-de" {
			own.ComputerID = ringProofComputer
		}
		mgr.byVM[own.VMID] = GuestReleaseVMStatus{GuestRelease: "r1"}
	}
	reg.mu.Unlock()
	f.h = NewHandler(reg)
	dir := t.TempDir()
	if err := f.h.SetRingWalkPath(filepath.Join(dir, "ring-walk.json"), filepath.Join(dir, "ring-walk-history.jsonl")); err != nil {
		t.Fatal(err)
	}
	mgr.pinner = f.h.RingWalkPinner()
	f.fake = &fakeGuestMarker{mgr: mock, outcome: guestMarkWritten}
	f.h.guestMarker = f.fake
	f.h.guestServedCommit = func(context.Context, VMOwnership) string { return "" }
	f.h.PrimeRingWalk()
	f.de = reg.GetOwnershipForDesktop("ring-de", "primary")
	f.o3 = reg.GetOwnershipForDesktop("ring-other", "primary")
	return f
}

func (f *ringFixture) deploy(id, commit string) {
	f.mgr.current, f.mgr.commit = id, commit
}

func getPlan(t *testing.T, h *Handler) deployRefreshPlan {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/internal/vmctl/deploy-refresh", nil)
	req.Header.Set("X-Internal-Caller", "true")
	rec := httptest.NewRecorder()
	h.HandleDeployRefresh(rec, req)
	var plan deployRefreshPlan
	if err := json.Unmarshal(rec.Body.Bytes(), &plan); err != nil {
		t.Fatalf("plan %d %s: %v", rec.Code, rec.Body.String(), err)
	}
	return plan
}

func TestRingPlanListsOnlyOpenRingAndGateSkipsTheRest(t *testing.T) {
	f := newRingHandlerFixture(t)
	if plan := getPlan(t, f.h); len(plan.Candidates) != 2 || len(plan.RingWaiting) != 0 {
		t.Fatalf("no walk: every active computer is a candidate: %+v", plan)
	}
	f.deploy("r2", "commit-r2")
	plan := getPlan(t, f.h)
	if len(plan.Candidates) != 1 || plan.Candidates[0].UserID != "ring-de" {
		t.Fatalf("a walk at R0: only d@e is a candidate: %+v", plan.Candidates)
	}
	if len(plan.RingWaiting) != 1 || plan.RingWaiting[0].UserID != "ring-other" || !strings.Contains(plan.RingWaiting[0].Reason, "R3 waits") {
		t.Fatalf("the R3 computer must be listed ring_waiting with its reason: %+v", plan.RingWaiting)
	}
	code, resp := postDeployRefresh(t, f.h, `{"user_id":"ring-other","desktop_id":"primary","target":"commit-r2"}`)
	if code != http.StatusConflict || resp.Outcome != deployRefreshSkipped || !strings.Contains(resp.Reason, "waits") {
		t.Fatalf("R3 refresh during R0: %d %+v, want skipped", code, resp)
	}
	if len(f.mgr.refreshes) != 0 || len(f.fake.marks) != 0 {
		t.Fatalf("a held-back computer was marked or refreshed: %v %v", f.fake.marks, f.mgr.refreshes)
	}
	if single, _ := f.h.RingWalkPinner().Pin(f.o3.ComputerID); single != "r1" {
		t.Fatalf("the R3 computer must be pinned to r1, got %q", single)
	}
	code, resp = postDeployRefresh(t, f.h, `{"user_id":"ring-de","desktop_id":"primary","target":"commit-r2"}`)
	if code != http.StatusOK || resp.Outcome != deployRefreshRefreshed {
		t.Fatalf("d@e refresh at R0: %d %+v", code, resp)
	}
}

func TestRingTickBreachEnrollsReturnAndHoldsTheRest(t *testing.T) {
	f := newRingHandlerFixture(t)
	f.deploy("r2", "commit-r2")
	f.h.syncRingWalk()
	f.mgr.byVM[f.de.VMID] = GuestReleaseVMStatus{GuestRelease: "r2"}
	f.h.ringHealthProbe = func(_ context.Context, own VMOwnership) ringHealth {
		if own.UserID != "ring-de" {
			t.Errorf("health read of %s, which is not in the open ring", own.UserID)
		}
		return ringHealth{Answered: true, Status: 200, Wedged: true, Detail: "projection_repair_required: planted"}
	}
	ctx := context.Background()
	f.h.TickRingWalk(ctx)
	f.h.TickRingWalk(ctx)
	st := f.h.ringWalkRunner().status(50)
	if st.State.Walk == nil || st.State.Walk.Status != ringWalkRolledBack {
		t.Fatalf("two wedged readings at d@e must roll back: %+v", st.State.Walk)
	}
	pending := f.h.pendingDeployRefreshes()
	if len(pending) != 1 || pending[0].UserID != "ring-de" || pending[0].Target != "commit-r1" {
		t.Fatalf("rollback must enroll d@e's return to commit-r1 only: %+v", pending)
	}
	f.h.SweepDeployRefreshPending(ctx)
	if len(f.fake.marks) != 1 || !strings.HasSuffix(f.fake.marks[0], "@commit-r1") || len(f.mgr.refreshes) != 1 {
		t.Fatalf("the return must be one marker-backed refresh of d@e: marks %v refreshes %v", f.fake.marks, f.mgr.refreshes)
	}
	// The R3 computer is never refreshed onto r2, now or later.
	code, resp := postDeployRefresh(t, f.h, `{"user_id":"ring-other","desktop_id":"primary","target":"commit-r2"}`)
	if code != http.StatusConflict || resp.Outcome != deployRefreshSkipped || !strings.Contains(resp.Reason, "rolled") {
		t.Fatalf("after rollback an R3 refresh to r2: %d %+v", code, resp)
	}
	if single, _ := f.h.RingWalkPinner().Pin(ringProofComputer); single != "r1" {
		t.Fatalf("after rollback d@e must boot r1, pinned %q", single)
	}
	var history []string
	for _, l := range st.History {
		var ev ringEvent
		_ = json.Unmarshal(l, &ev)
		history = append(history, ev.Event)
	}
	if got := strings.Join(history, ","); !strings.Contains(got, "walk_started,ring_opened,ring_closed,rolled_back") {
		t.Fatalf("history = %s", got)
	}
}

// The "resume, then update" sweep compares a computer with the release it
// launches now: a pinned computer resumed on its pin is not behind.
func TestRingPinnedResumeIsNotBehind(t *testing.T) {
	f := newRingHandlerFixture(t)
	f.deploy("r2", "commit-r2")
	f.h.syncRingWalk()
	f.mgr.byVM[f.o3.VMID] = GuestReleaseVMStatus{GuestRelease: "r1"}
	f.h.SetBehindRefresh(1, nil)
	f.h.deployRefreshState().enroll(deployRefreshPendingEntry{UserID: "ring-other", DesktopID: "primary", VMID: f.o3.VMID, Target: "commit-r2", Kind: deployRefreshKindWokeBehind, Release: "r1"})
	f.h.SweepDeployRefreshPending(context.Background())
	if len(f.mgr.refreshes) != 0 || len(f.h.pendingDeployRefreshes()) != 0 {
		t.Fatalf("a computer on its pinned release was refreshed or kept pending: %v %+v", f.mgr.refreshes, f.h.pendingDeployRefreshes())
	}
}

// Review 1 F5: a ring enroll of a computer that woke behind keeps its kind,
// and so the user-idle gate.
func TestRingEnsureKeepsWokeBehindKind(t *testing.T) {
	tr := newDeployRefreshTracker()
	tr.enroll(deployRefreshPendingEntry{UserID: "u", DesktopID: "primary", Target: "commit-r1", Kind: deployRefreshKindWokeBehind, Release: "r1"})
	tr.ensure(deployRefreshPendingEntry{UserID: "u", DesktopID: "primary", Target: "commit-r2"})
	got := tr.list()
	if len(got) != 1 || got[0].Kind != deployRefreshKindWokeBehind || got[0].Target != "commit-r2" {
		t.Fatalf("ensure dropped the woke_behind kind: %+v", got)
	}
}

// Staging 2026-11-03: the R0 wake must go through the real resolve with the
// observed VM id; an empty one is refused as "realized VM ownership changed".
func TestRingWakeResolvesStoppedProofComputer(t *testing.T) {
	f := newRingHandlerFixture(t)
	f.reg.mu.Lock()
	own := f.reg.ownerships[ownershipKey("ring-de", "primary")]
	own.State = VMStateStopped
	own.StoppedBy = "idle"
	vmID := own.VMID
	f.reg.mu.Unlock()
	f.h.ringWake(ringAction{Kind: ringActionWake, ComputerID: ringProofComputer, UserID: "ring-de", DesktopID: "primary", VMID: vmID, Reason: "test"})
	got := f.reg.GetOwnershipForDesktop("ring-de", "primary")
	if got == nil || got.State != VMStateActive {
		t.Fatalf("the wake did not start d@e: %+v", got)
	}
}

// The expedite endpoint opens every ring: the CI plan then lists every active
// computer as a candidate; the history records reason and who asked.
func TestRingExpediteEndpoint(t *testing.T) {
	f := newRingHandlerFixture(t)
	f.deploy("r2", "commit-r2")
	if plan := getPlan(t, f.h); len(plan.RingWaiting) != 1 {
		t.Fatalf("before expedite one computer waits: %+v", plan)
	}
	req := httptest.NewRequest(http.MethodPost, "/internal/vmctl/ring-walk/expedite", strings.NewReader(`{"reason":"key fix","by":"owner"}`))
	req.Header.Set("X-Internal-Caller", "true")
	rec := httptest.NewRecorder()
	f.h.HandleRingWalkExpedite(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"expedited":true`) {
		t.Fatalf("expedite: %d %s", rec.Code, rec.Body.String())
	}
	plan := getPlan(t, f.h)
	if len(plan.Candidates) != 2 || len(plan.RingWaiting) != 0 || plan.RingWalk == nil || !plan.RingWalk.Expedited {
		t.Fatalf("expedited plan must list every active computer: %+v", plan)
	}
	st := f.h.ringWalkRunner().status(50)
	found := false
	for _, l := range st.History {
		if strings.Contains(string(l), `"event":"expedited"`) && strings.Contains(string(l), "key fix") && strings.Contains(string(l), "owner") {
			found = true
		}
	}
	if !found || !st.Expedited {
		t.Fatalf("expedite not recorded: %+v", st.History)
	}
	// No by: refused.
	req = httptest.NewRequest(http.MethodPost, "/internal/vmctl/ring-walk/expedite", strings.NewReader(`{"reason":"x"}`))
	req.Header.Set("X-Internal-Caller", "true")
	rec = httptest.NewRecorder()
	f.h.HandleRingWalkExpedite(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expedite without by: %d", rec.Code)
	}
}

// Round 3: the endpoint refuses a caller that is not internal.
func TestRingExpediteEndpointInternalOnly(t *testing.T) {
	f := newRingHandlerFixture(t)
	f.deploy("r2", "commit-r2")
	req := httptest.NewRequest(http.MethodPost, "/internal/vmctl/ring-walk/expedite", strings.NewReader(`{"reason":"x","by":"y"}`))
	req.RemoteAddr = "203.0.113.7:4444"
	rec := httptest.NewRecorder()
	f.h.HandleRingWalkExpedite(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("an external caller reached expedite: %d %s", rec.Code, rec.Body.String())
	}
}
