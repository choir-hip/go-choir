package vmctl

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The owner update (docs/design/update-now-2026-11-09.md), failure modes 1-3,
// 5-7 at vmctl. Ways this could fail:
//   - the click on a computer already on its release marks or restarts it;
//   - a guest refusal (capsule, apply, candidate) is flattened or the
//     computer is restarted anyway;
//   - running runs alone defer the click (allow_busy not sent);
//   - the click releases a ring that has not opened, or targets the walking
//     release instead of the pin;
//   - a second click restarts twice;
//   - the signal shows for a ring-held, idle or already-current computer.

func postOwnerUpdate(t *testing.T, h *Handler, user string) (int, deployRefreshResponse) {
	t.Helper()
	// A caller-chosen target must be ignored.
	return postDeployRefresh(t, h, `{"user_id":"`+user+`","desktop_id":"primary","target":"attacker-chosen","owner_busy":true}`)
}

func getUpdateAvailable(t *testing.T, h *Handler, user string) UpdateAvailableResponse {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/internal/vmctl/update-available?user_id="+user+"&desktop_id=primary", nil)
	req.Header.Set("X-Internal-Caller", "true")
	rec := httptest.NewRecorder()
	h.HandleUpdateAvailable(rec, req)
	var out UpdateAvailableResponse
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &out) != nil {
		t.Fatalf("update-available %d %s", rec.Code, rec.Body.String())
	}
	return out
}

// FM1: already on the release, including a ring-held computer serving its
// held release: 200 current, no marker, no restart, no pending entry created,
// a stale one forgotten.
func TestOwnerUpdateAlreadyCurrentIsANoOp(t *testing.T) {
	f := newRingHandlerFixture(t)
	f.deploy("r2", "commit-r2")
	f.h.syncRingWalk()
	// o3 is ring-held on r1 and serves it.
	f.h.guestServedCommit = func(context.Context, VMOwnership) string { return "commit-r1" }
	f.h.deployRefreshState().enroll(deployRefreshPendingEntry{UserID: "ring-other", DesktopID: "primary", Target: "commit-r1"})
	code, resp := postOwnerUpdate(t, f.h, "ring-other")
	if code != http.StatusOK || resp.Outcome != deployRefreshCurrent || resp.Target != "commit-r1" {
		t.Fatalf("held computer on its held release: %d %+v, want 200 current", code, resp)
	}
	if len(f.fake.marks) != 0 || len(f.mgr.refreshes) != 0 {
		t.Fatalf("current marked or restarted: %v %v", f.fake.marks, f.mgr.refreshes)
	}
	if pending := f.h.pendingDeployRefreshes(); len(pending) != 0 {
		t.Fatalf("stale pending entry kept: %+v", pending)
	}
	// d@e is not pending and serves r2: current creates no entry either.
	f.h.guestServedCommit = func(context.Context, VMOwnership) string { return "commit-r2" }
	if code, resp = postOwnerUpdate(t, f.h, "ring-de"); code != http.StatusOK || resp.Outcome != deployRefreshCurrent {
		t.Fatalf("d@e on r2: %d %+v", code, resp)
	}
	if pending := f.h.pendingDeployRefreshes(); len(pending) != 0 {
		t.Fatalf("current created a pending entry: %+v", pending)
	}
	// A served commit that cannot be read is not current.
	f.h.guestServedCommit = func(context.Context, VMOwnership) string { return "" }
	if code, resp = postOwnerUpdate(t, f.h, "ring-de"); code != http.StatusOK || resp.Outcome != deployRefreshRefreshed {
		t.Fatalf("unreadable served commit: %d %+v, want the path to continue", code, resp)
	}
}

// FM3: running runs alone do not defer an owner update: allow_busy is sent
// and the marker-backed restart happens.
func TestOwnerUpdateRunningRunsRestartWithMarkerAndAllowBusy(t *testing.T) {
	h, mgr, fake := newRingOnlyFixture(t)
	h.guestServedCommit = func(context.Context, VMOwnership) string { return "old" }
	fake.outcome, fake.detail = guestMarkWritten, `{"interrupted_running_runs":3}`
	code, resp := postOwnerUpdate(t, h, "owner-up")
	if code != http.StatusOK || resp.Outcome != deployRefreshRefreshed || !resp.PlannedRestart || !resp.BusyAllowed {
		t.Fatalf("busy owner: %d %+v, want 200 refreshed, planned, busy_allowed", code, resp)
	}
	if len(fake.allowBusy) != 1 || !fake.allowBusy[0] {
		t.Fatalf("allow_busy sent = %v, want true", fake.allowBusy)
	}
	if len(fake.marks) != 1 || !strings.HasSuffix(fake.marks[0], "@commit-r1") {
		t.Fatalf("marks = %v, want the resolved target commit-r1, never the caller's", fake.marks)
	}
	if len(mgr.refreshes) != 1 {
		t.Fatalf("refreshes = %v, want one", mgr.refreshes)
	}
}

// The sweep and deploys keep the class policy: no owner_busy, no allow_busy.
func TestDeployRefreshWithoutOwnerBusyKeepsClassPolicy(t *testing.T) {
	h, _, fake := newRingOnlyFixture(t)
	fake.outcome, fake.detail = guestMarkBusy, `{"outcome":"busy","running_runs":1}`
	postDeployRefresh(t, h, `{"user_id":"owner-up","desktop_id":"primary","target":"commit-r1"}`)
	if len(fake.allowBusy) != 1 || fake.allowBusy[0] {
		t.Fatalf("allow_busy sent = %v, want false for a deploy of an ordinary computer", fake.allowBusy)
	}
}

// FM2: the guest's structured refusal passes through; nothing restarts; the
// pending entry stays so the notice stays.
func TestOwnerUpdateGuestRefusalPassesThroughStructured(t *testing.T) {
	cases := []struct{ name, detail, code string }{
		{"capsule", `{"outcome":"busy","allow_busy":true,"live_capsules":1}`, "live_capsule"},
		{"apply", `{"outcome":"busy","allow_busy":true,"selfdev_active_operations":1}`, "selfdev_active"},
		{"candidate", `{"outcome":"busy","reason":"selfdev_awaiting_decision","selfdev_awaiting_decision":1}`, "selfdev_awaiting_decision"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, mgr, fake := newRingOnlyFixture(t)
			h.guestServedCommit = func(context.Context, VMOwnership) string { return "old" }
			fake.outcome, fake.detail = guestMarkBusy, tc.detail
			code, resp := postOwnerUpdate(t, h, "owner-up")
			if code != http.StatusConflict || resp.Outcome != deployRefreshDeferred || resp.ReasonCode != tc.code {
				t.Fatalf("%d %+v, want 409 deferred code %s", code, resp, tc.code)
			}
			var refusal map[string]any
			if json.Unmarshal(resp.Refusal, &refusal) != nil || refusal["outcome"] != "busy" {
				t.Fatalf("refusal not passed through: %s", resp.Refusal)
			}
			if len(mgr.refreshes) != 0 {
				t.Fatalf("refused click restarted: %v", mgr.refreshes)
			}
			if got := getUpdateAvailable(t, h, "owner-up"); !got.Available || got.Target != "commit-r1" {
				t.Fatalf("notice must stay after a deferral: %+v", got)
			}
		})
	}
}

// FM5: the ring gate holds. A computer behind its held release restarts onto
// the held release (never the walking one); one serving it is current; a
// resolved target ahead of the pin is skipped "ring waits".
func TestOwnerUpdateRingHeldCatchesUpToThePinNeverTheWalkingRelease(t *testing.T) {
	f := newRingHandlerFixture(t)
	f.deploy("r2", "commit-r2")
	f.h.syncRingWalk()
	f.h.guestServedCommit = func(context.Context, VMOwnership) string { return "commit-r0-older" }
	code, resp := postOwnerUpdate(t, f.h, "ring-other")
	if code != http.StatusOK || resp.Outcome != deployRefreshRefreshed || resp.Target != "commit-r1" {
		t.Fatalf("behind the pin: %d %+v, want a catch-up refresh onto commit-r1", code, resp)
	}
	if len(f.fake.marks) != 1 || !strings.HasSuffix(f.fake.marks[0], "@commit-r1") {
		t.Fatalf("marks = %v, want the held release only", f.fake.marks)
	}
}

type aheadOfPinManager struct{ *ringManager }

// GuestReleaseInfoFor ignores the pin: the resolver names the walking release.
func (m *aheadOfPinManager) GuestReleaseInfoFor(string) (string, string) { return m.current, m.commit }

func TestOwnerUpdateTargetAheadOfThePinIsSkippedRingWaits(t *testing.T) {
	f := newRingHandlerFixture(t)
	f.deploy("r2", "commit-r2")
	f.h.syncRingWalk()
	f.reg.SetVMManager(&aheadOfPinManager{f.mgr})
	f.h.guestServedCommit = func(context.Context, VMOwnership) string { return "commit-r1" }
	code, resp := postOwnerUpdate(t, f.h, "ring-other")
	if code != http.StatusConflict || resp.Outcome != deployRefreshSkipped || !strings.Contains(resp.Reason, "waits") {
		t.Fatalf("ahead of the pin: %d %+v, want 409 skipped ring waits", code, resp)
	}
	if len(f.fake.marks) != 0 || len(f.mgr.refreshes) != 0 {
		t.Fatalf("an unopened ring was released: %v %v", f.fake.marks, f.mgr.refreshes)
	}
}

// FM6: a second click while one is in flight is deferred "in flight", not
// marked, and not recorded; one restart either way.
func TestOwnerUpdateSecondClickWhileInFlightIsDeferred(t *testing.T) {
	h, mgr, fake := newRingOnlyFixture(t)
	h.guestServedCommit = func(context.Context, VMOwnership) string { return "old" }
	release, ok := h.deployRefreshState().begin("owner-up", "primary")
	if !ok {
		t.Fatal("begin")
	}
	code, resp := postOwnerUpdate(t, h, "owner-up")
	release()
	if code != http.StatusConflict || resp.Outcome != deployRefreshDeferred || resp.Reason != deployRefreshInFlightReason {
		t.Fatalf("second click: %d %+v, want 409 in flight", code, resp)
	}
	if len(fake.marks) != 0 || len(mgr.refreshes) != 0 {
		t.Fatalf("second click marked or restarted: %v %v", fake.marks, mgr.refreshes)
	}
	if pending := h.pendingDeployRefreshes(); len(pending) != 0 {
		t.Fatalf("in-flight deferral recorded: %+v", pending)
	}
}

// FM7: the signal shows only for an active computer with a pending entry whose
// ring is open and whose served commit differs; it clears after the restart
// and survives a vmctl restart.
func TestUpdateAvailableSignal(t *testing.T) {
	f := newRingHandlerFixture(t)
	f.h.guestServedCommit = func(context.Context, VMOwnership) string { return "old" }
	if got := getUpdateAvailable(t, f.h, "ring-de"); got.Available {
		t.Fatalf("no pending entry: %+v", got)
	}
	f.deploy("r2", "commit-r2")
	f.h.syncRingWalk()
	// d@e's ring is open: a deferral by a deploy shows the notice.
	f.fake.outcome, f.fake.detail = guestMarkBusy, `{"outcome":"busy","running_runs":1}`
	postDeployRefresh(t, f.h, `{"user_id":"ring-de","desktop_id":"primary","target":"commit-r2"}`)
	got := getUpdateAvailable(t, f.h, "ring-de")
	if !got.Available || got.Target != "commit-r2" || got.Since == "" {
		t.Fatalf("pending d@e: %+v, want available commit-r2", got)
	}
	// A ring-held computer on its held release shows nothing even when an
	// entry exists.
	f.h.guestServedCommit = func(_ context.Context, own VMOwnership) string {
		if own.UserID == "ring-other" {
			return "commit-r1" // serving its held release
		}
		return "old"
	}
	f.h.deployRefreshState().enroll(deployRefreshPendingEntry{UserID: "ring-other", DesktopID: "primary", Target: "commit-r2"})
	if got := getUpdateAvailable(t, f.h, "ring-other"); got.Available {
		t.Fatalf("ring-held computer shows a notice: %+v", got)
	}
	// Once it serves the target the notice goes.
	f.h.guestServedCommit = func(context.Context, VMOwnership) string { return "commit-r2" }
	if got := getUpdateAvailable(t, f.h, "ring-de"); got.Available {
		t.Fatalf("notice after the restart: %+v", got)
	}
}

func TestUpdateAvailableSurvivesVMCTLRestart(t *testing.T) {
	h, _, fake := newRingOnlyFixture(t)
	path := t.TempDir() + "/pending.json"
	if err := h.SetDeployRefreshPendingPath(path); err != nil {
		t.Fatal(err)
	}
	h.guestServedCommit = func(context.Context, VMOwnership) string { return "old" }
	fake.outcome, fake.detail = guestMarkBusy, `{"outcome":"busy","reason":"hibernate_fenced"}`
	postOwnerUpdate(t, h, "owner-up")

	h2 := NewHandler(h.registry)
	h2.guestServedCommit = h.guestServedCommit
	h2.guestMarker = fake
	if err := h2.SetDeployRefreshPendingPath(path); err != nil {
		t.Fatal(err)
	}
	if got := getUpdateAvailable(t, h2, "owner-up"); !got.Available {
		t.Fatalf("notice lost across a vmctl restart (hibernate-fenced computer): %+v", got)
	}
}

func newRingOnlyFixture(t *testing.T) (*Handler, *mockVMManager, *fakeGuestMarker) {
	t.Helper()
	mock := &mockVMManager{}
	reg := NewOwnershipRegistry("")
	reg.SetCorpusdURL(testComputerCredentialIssuerURL(t))
	mgr := &ringManager{mockVMManager: mock, current: "r1", commit: "commit-r1", byVM: map[string]GuestReleaseVMStatus{}}
	reg.SetVMManager(mgr)
	if _, err := reg.ResolveOrAssign("owner-up"); err != nil {
		t.Fatal(err)
	}
	h := NewHandler(reg)
	mgr.pinner = h.RingWalkPinner()
	fake := &fakeGuestMarker{mgr: mock, outcome: guestMarkWritten}
	h.guestMarker = fake
	h.guestServedCommit = func(context.Context, VMOwnership) string { return "" }
	return h, mock, fake
}
