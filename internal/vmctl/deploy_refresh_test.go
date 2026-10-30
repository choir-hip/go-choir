package vmctl

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// Deploy refresh through the guest's planned-restart marker
// (docs/archive/problems/owner-computer-pinned-as-canary-2026-10-13.md). Owner rule:
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
	allowBusy       []bool
	clears          int
	refreshesAtMark []int
	mgr             *mockVMManager
}

func (f *fakeGuestMarker) mark(_ context.Context, own VMOwnership, target string, allowBusy bool) (guestMarkOutcome, string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.marks = append(f.marks, own.VMID+"@"+target)
	f.allowBusy = append(f.allowBusy, allowBusy)
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
	h.guestServedCommit = func(context.Context, VMOwnership) string { return "" }
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
		var gotPath, gotTarget, gotQuery string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.Method + " " + r.URL.Path
			gotQuery = r.URL.RawQuery
			var body struct {
				Target string `json:"target"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			gotTarget = body.Target
			w.WriteHeader(c.status)
			_, _ = w.Write([]byte(`{}`))
		}))
		outcome, _ := httpGuestMarker{client: server.Client()}.mark(context.Background(), VMOwnership{ComputerURL: server.URL}, "761d94c5", false)
		if outcome != c.want || gotPath != "POST /internal/runtime/planned-restart" || gotTarget != "761d94c5" || gotQuery != "" {
			t.Fatalf("status %d: outcome=%v path=%q query=%q target=%q, want %v", c.status, outcome, gotPath, gotQuery, gotTarget, c.want)
		}
		// allow_busy travels as a query parameter: a guest built before it
		// ignores the parameter (its body decoder rejects unknown fields).
		outcome, _ = httpGuestMarker{client: server.Client()}.mark(context.Background(), VMOwnership{ComputerURL: server.URL}, "761d94c5", true)
		server.Close()
		if outcome != c.want || gotQuery != "allow_busy=1" || gotTarget != "761d94c5" {
			t.Fatalf("status %d allow_busy: outcome=%v query=%q target=%q, want %v with allow_busy=1", c.status, outcome, gotQuery, gotTarget, c.want)
		}
	}
	outcome, _ := httpGuestMarker{}.mark(context.Background(), VMOwnership{ComputerURL: "http://127.0.0.1:1"}, "761d94c5", false)
	if outcome != guestMarkUnavailable {
		t.Fatalf("unreachable guest outcome = %v, want unavailable", outcome)
	}
}

// Owner ruling 2026-10-15: "we need to update the Autopaper computer even
// when it is busy, because it's always going to be busy"
// (docs/problems/always-busy-computer-never-updates-2026-10-15.md). Ways this
// could fail, written first:
//   - a busy always-on platform computer (the paper, critical_protected) is
//     still deferred, so it never updates;
//   - a busy ordinary computer is refreshed (the ruling covers platform
//     computers, not every user's running turn);
//   - a guest refusal while busy-allowed (a self-development apply or a live
//     capsule) is refreshed anyway, or waits for the next deploy;
//   - a deferred computer waits for the next deploy instead of being retried
//     until it takes the update;
//   - the retry refreshes a computer twice, or keeps killing one whose
//     refresh fails;
//   - the retry and a deploy refresh the same computer at once, and the
//     loser's failure path clears the winner's marker;
//   - a vmctl restart loses the deferred set.

func makePlatformComputer(t *testing.T, h *Handler, user string) *VMOwnership {
	t.Helper()
	own := h.registry.GetOwnershipForDesktop(user, PrimaryDesktopID)
	if own == nil {
		t.Fatalf("no ownership for %s", user)
	}
	h.registry.SetWarmnessPolicyConfig(WarmnessPolicyConfig{PlatformComputerIDs: map[string]bool{own.ComputerID: true}})
	if got := h.registry.WarmnessClassForOwnership(own); got != WarmnessClassCriticalProtected {
		t.Fatalf("fixture class = %s, want critical_protected", got)
	}
	return own
}

func setFakeOutcome(f *fakeGuestMarker, outcome guestMarkOutcome) {
	f.mu.Lock()
	f.outcome = outcome
	f.mu.Unlock()
}

func TestDeployRefreshAllowsBusyForCriticalProtectedComputer(t *testing.T) {
	h, mgr, fake := newDeployRefreshFixture(t, "paper")
	makePlatformComputer(t, h, "paper")
	fake.outcome, fake.detail = guestMarkWritten, `{"interrupted_running_runs":2}`
	code, resp := postDeployRefresh(t, h, `{"user_id":"paper","desktop_id":"primary","target":"761d94c5"}`)
	if code != http.StatusOK || resp.Outcome != deployRefreshRefreshed || !resp.PlannedRestart || !resp.BusyAllowed {
		t.Fatalf("busy paper: %d %+v, want 200 refreshed, planned, busy_allowed", code, resp)
	}
	if len(fake.allowBusy) != 1 || !fake.allowBusy[0] {
		t.Fatalf("allow_busy sent = %v, want true for a critical_protected computer", fake.allowBusy)
	}
	if len(mgr.refreshes) != 1 {
		t.Fatalf("refreshes = %v, want one", mgr.refreshes)
	}
	if pending := h.pendingDeployRefreshes(); len(pending) != 0 {
		t.Fatalf("refreshed computer left pending: %+v", pending)
	}
}

func TestDeployRefreshBusyOrdinaryComputerStillDefersAndIsRecorded(t *testing.T) {
	h, mgr, fake := newDeployRefreshFixture(t, "ordinary")
	fake.outcome, fake.detail = guestMarkBusy, `{"outcome":"busy","running_runs":1}`
	code, resp := postDeployRefresh(t, h, `{"user_id":"ordinary","desktop_id":"primary","target":"761d94c5"}`)
	if code != http.StatusConflict || resp.Outcome != deployRefreshDeferred || resp.BusyAllowed {
		t.Fatalf("busy ordinary: %d %+v, want 409 deferred, busy not allowed", code, resp)
	}
	if len(fake.allowBusy) != 1 || fake.allowBusy[0] {
		t.Fatalf("allow_busy sent = %v, want false for a primary computer", fake.allowBusy)
	}
	if len(mgr.refreshes) != 0 {
		t.Fatalf("busy ordinary computer refreshed: %v", mgr.refreshes)
	}
	pending := h.pendingDeployRefreshes()
	if len(pending) != 1 || pending[0].UserID != "ordinary" || pending[0].Target != "761d94c5" {
		t.Fatalf("pending = %+v, want the deferred computer recorded for retry", pending)
	}
}

// The guest refuses even with allow_busy while a self-development apply is in
// flight or a capsule is live; vmctl defers and the retry sweep tries again
// shortly, not at the next deploy.
func TestDeployRefreshSelfDevelopmentApplyDefersEvenWhenBusyIsAllowed(t *testing.T) {
	h, mgr, fake := newDeployRefreshFixture(t, "paper-selfdev")
	makePlatformComputer(t, h, "paper-selfdev")
	h.guestServedCommit = func(context.Context, VMOwnership) string { return "0ld" }
	fake.outcome, fake.detail = guestMarkBusy, `{"outcome":"busy","allow_busy":true,"selfdev_active_operations":1}`
	code, resp := postDeployRefresh(t, h, `{"user_id":"paper-selfdev","desktop_id":"primary","target":"761d94c5"}`)
	if code != http.StatusConflict || resp.Outcome != deployRefreshDeferred || !strings.Contains(resp.Reason, "selfdev_active_operations") {
		t.Fatalf("selfdev in flight: %d %+v, want 409 deferred naming the guest's reason", code, resp)
	}
	if len(mgr.refreshes) != 0 {
		t.Fatalf("a self-development apply was cut: %v", mgr.refreshes)
	}
	if pending := h.pendingDeployRefreshes(); len(pending) != 1 {
		t.Fatalf("pending = %+v, want it recorded for the short retry", pending)
	}
	// Once the apply settles, the sweep refreshes it.
	setFakeOutcome(fake, guestMarkWritten)
	h.SweepDeployRefreshPending(context.Background())
	if len(mgr.refreshes) != 1 {
		t.Fatalf("sweep after the apply settled: refreshes=%v, want one", mgr.refreshes)
	}
}

func TestDeployRefreshSweepRetriesDeferredComputerUntilItUpdatesThenStops(t *testing.T) {
	h, mgr, fake := newDeployRefreshFixture(t, "retry")
	h.guestServedCommit = func(context.Context, VMOwnership) string { return "0ld" }
	fake.outcome = guestMarkBusy
	if code, resp := postDeployRefresh(t, h, `{"user_id":"retry","desktop_id":"primary","target":"761d94c5"}`); resp.Outcome != deployRefreshDeferred {
		t.Fatalf("setup: %d %+v, want deferred", code, resp)
	}
	// Still busy: the sweep asks again and keeps it.
	h.SweepDeployRefreshPending(context.Background())
	if len(mgr.refreshes) != 0 || len(h.pendingDeployRefreshes()) != 1 || len(fake.marks) != 2 {
		t.Fatalf("busy sweep: refreshes=%v pending=%+v marks=%v, want asked again and kept", mgr.refreshes, h.pendingDeployRefreshes(), fake.marks)
	}
	if got := h.pendingDeployRefreshes()[0].Attempts; got != 2 {
		t.Fatalf("attempts = %d, want 2 (the deploy and one sweep)", got)
	}
	// Idle at last: refreshed once, then forgotten.
	setFakeOutcome(fake, guestMarkWritten)
	h.SweepDeployRefreshPending(context.Background())
	if len(mgr.refreshes) != 1 || len(h.pendingDeployRefreshes()) != 0 {
		t.Fatalf("idle sweep: refreshes=%v pending=%+v, want one refresh and nothing pending", mgr.refreshes, h.pendingDeployRefreshes())
	}
	// A layered computer keeps serving its release commit after a refresh;
	// the sweep must not refresh it again.
	h.SweepDeployRefreshPending(context.Background())
	if len(mgr.refreshes) != 1 {
		t.Fatalf("refreshed twice: %v", mgr.refreshes)
	}
}

func TestDeployRefreshSweepDropsComputerAlreadyOnTarget(t *testing.T) {
	h, mgr, fake := newDeployRefreshFixture(t, "current")
	fake.outcome = guestMarkBusy
	postDeployRefresh(t, h, `{"user_id":"current","desktop_id":"primary","target":"761d94c5"}`)
	h.guestServedCommit = func(context.Context, VMOwnership) string { return "761d94c5" }
	setFakeOutcome(fake, guestMarkWritten)
	h.SweepDeployRefreshPending(context.Background())
	if len(mgr.refreshes) != 0 || len(h.pendingDeployRefreshes()) != 0 {
		t.Fatalf("current computer: refreshes=%v pending=%+v, want dropped without a refresh", mgr.refreshes, h.pendingDeployRefreshes())
	}
}

func TestDeployRefreshSweepDropsFailedRefreshInsteadOfRetryingIt(t *testing.T) {
	h, mgr, fake := newDeployRefreshFixture(t, "failing")
	h.guestServedCommit = func(context.Context, VMOwnership) string { return "" }
	fake.outcome = guestMarkBusy
	postDeployRefresh(t, h, `{"user_id":"failing","desktop_id":"primary","target":"761d94c5"}`)
	setFakeOutcome(fake, guestMarkWritten)
	mgr.mu.Lock()
	mgr.refreshError = errors.New("firecracker refused")
	mgr.mu.Unlock()
	h.SweepDeployRefreshPending(context.Background())
	h.SweepDeployRefreshPending(context.Background())
	if marks := len(fake.marks); marks != 2 || len(h.pendingDeployRefreshes()) != 0 {
		t.Fatalf("failed refresh: marks=%d pending=%+v, want one refresh attempt after the deploy and nothing pending", marks, h.pendingDeployRefreshes())
	}
	if fake.clears != 1 {
		t.Fatalf("clears = %d, want the failed refresh's marker cleared once", fake.clears)
	}
}

func TestDeployRefreshSweepDropsHeldComputer(t *testing.T) {
	h, mgr, fake := newDeployRefreshFixture(t, "held-later")
	fake.outcome = guestMarkBusy
	postDeployRefresh(t, h, `{"user_id":"held-later","desktop_id":"primary","target":"761d94c5"}`)
	own := h.registry.GetOwnershipForDesktop("held-later", PrimaryDesktopID)
	if err := h.registry.SetHold(own.ComputerID, "maintenance", "recovery"); err != nil {
		t.Fatal(err)
	}
	h.SweepDeployRefreshPending(context.Background())
	if len(mgr.refreshes) != 0 || len(h.pendingDeployRefreshes()) != 0 {
		t.Fatalf("held computer: refreshes=%v pending=%+v, want skipped and dropped", mgr.refreshes, h.pendingDeployRefreshes())
	}
}

// A refresh already in flight for a computer answers "deferred" before
// touching the guest: otherwise the second call's failure path ("refresh is
// already in progress") would clear the marker the first refresh relies on.
func TestDeployRefreshConcurrentRefreshDoesNotTouchTheMarker(t *testing.T) {
	h, mgr, fake := newDeployRefreshFixture(t, "concurrent")
	fake.outcome = guestMarkWritten
	release, ok := h.deployRefreshState().begin("concurrent", PrimaryDesktopID)
	if !ok {
		t.Fatal("could not take the in-flight slot")
	}
	code, resp := postDeployRefresh(t, h, `{"user_id":"concurrent","desktop_id":"primary","target":"761d94c5"}`)
	release()
	if code != http.StatusConflict || resp.Outcome != deployRefreshDeferred || !strings.Contains(resp.Reason, "in flight") {
		t.Fatalf("concurrent: %d %+v, want 409 deferred in flight", code, resp)
	}
	if len(fake.marks) != 0 || fake.clears != 0 || len(mgr.refreshes) != 0 {
		t.Fatalf("concurrent call touched the guest: marks=%v clears=%d refreshes=%v", fake.marks, fake.clears, mgr.refreshes)
	}
}

func TestDeployRefreshPendingSurvivesVMCTLRestartAndKeepsNewestTarget(t *testing.T) {
	path := filepath.Join(t.TempDir(), "deploy-refresh-pending.json")
	h, _, fake := newDeployRefreshFixture(t, "durable")
	if err := h.SetDeployRefreshPendingPath(path); err != nil {
		t.Fatal(err)
	}
	fake.outcome = guestMarkBusy
	postDeployRefresh(t, h, `{"user_id":"durable","desktop_id":"primary","target":"aaaa1111"}`)
	postDeployRefresh(t, h, `{"user_id":"durable","desktop_id":"primary","target":"bbbb2222"}`)

	h2 := NewHandler(h.registry)
	if err := h2.SetDeployRefreshPendingPath(path); err != nil {
		t.Fatal(err)
	}
	pending := h2.pendingDeployRefreshes()
	if len(pending) != 1 || pending[0].Target != "bbbb2222" || pending[0].Attempts != 2 {
		t.Fatalf("reloaded pending = %+v, want one entry for the newest target with 2 attempts", pending)
	}

	// The selection lists it, so an operator sees it without SSH.
	req := httptest.NewRequest(http.MethodGet, "/internal/vmctl/deploy-refresh", nil)
	req.Header.Set("X-Internal-Caller", "true")
	rec := httptest.NewRecorder()
	h2.HandleDeployRefresh(rec, req)
	var plan deployRefreshPlan
	if err := json.Unmarshal(rec.Body.Bytes(), &plan); err != nil || len(plan.Pending) != 1 || plan.Pending[0].UserID != "durable" {
		t.Fatalf("GET plan = %s (%v), want the pending computer listed", rec.Body.String(), err)
	}
}
