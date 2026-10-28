package vmmanager

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

// S5 busy hibernate on a planned host stop
// (docs/problems/paper-never-checkpointed-2026-10-27.md, slice S5). Written
// before the code. Ways it can fail:
//   - F1 (owner 2026-10-28: on by default for every computer, no off switch)
//     a host stop of a busy guest is refused for want of a switch;
//   - F2 a hibernate that is not a host stop (idle sweep, make-room, operator)
//     captures a busy guest;
//   - F3 a guest busy with a self-development apply is captured;
//   - F4 a guest with a planned-restart marker pending is captured (a resumed
//     guest that crashed in the marker window would boot planned and resume);
//   - F5 a busy reply this host does not understand is captured;
//   - F6 a guest whose release is not the current release (or a legacy launch
//     with no release) is captured; its snapshot would be refused at wake, so
//     the stop would write gigabytes for nothing;
//   - F7 a busy snapshot is resumed onto a different guest release (a guest
//     code update must cold boot, never resume a busy process);
//   - F9 a busy snapshot is resumed past its own, shorter age bound;
//   - F10 a busy capture is not marked busy, so it cannot be told from a
//     fenced one at wake or in the timeline;
//   - F11 the age bound cannot be set from the environment.

func busyGuestBody() map[string]any {
	return map[string]any{"outcome": "busy", "error": "guest is busy; hibernate deferred", "running_runs": 2}
}

func hostStopReleaseFixture(t *testing.T) *releaseFixture {
	t.Helper()
	r := newReleaseFixture(t)
	r.serveLaunchAPI(t)
	r.guest.prepareStatus = http.StatusConflict
	r.guest.busyBody = busyGuestBody()
	return r
}

func assertLeftRunning(t *testing.T, f *snapshotFixture, err error, wantRefused string) {
	t.Helper()
	var busy interface{ HibernateGuestBusy() bool }
	if err == nil || !errors.As(err, &busy) || !busy.HibernateGuestBusy() {
		t.Fatalf("hibernate = %v, want a guest-busy skip", err)
	}
	f.mgr.mu.Lock()
	state := f.inst.State
	f.mgr.mu.Unlock()
	if state != StateRunning {
		t.Fatalf("state = %s, want the busy guest left running", state)
	}
	for _, call := range f.fc.paths() {
		if call != "GET /" {
			t.Fatalf("a refused busy guest must never be paused; calls=%v", f.fc.paths())
		}
	}
	if _, statErr := os.Stat(f.mgr.snapshotDir(f.vmID)); !os.IsNotExist(statErr) {
		t.Fatalf("no snapshot may be left: %v", statErr)
	}
	tl := readHibernateTimeline(t, f.mgr, f.vmID)
	if tl.Snapshot == nil || tl.Snapshot.Outcome != "skip" || tl.Snapshot.Reason != snapshotReasonGuestBusy {
		t.Fatalf("timeline = %+v, want a guest_busy skip", tl.Snapshot)
	}
	if tl.Snapshot.BusyRefused != wantRefused {
		t.Fatalf("busy_refused = %q, want %q", tl.Snapshot.BusyRefused, wantRefused)
	}
}

// F1: no configuration is needed; the default manager captures.
func TestBusyHibernateIsOnByDefault(t *testing.T) {
	r := hostStopReleaseFixture(t)
	if err := r.mgr.HibernateVMForHostStop(context.Background(), r.vmID, true); err != nil {
		t.Fatalf("HibernateVMForHostStop with the default config: %v", err)
	}
	if r.inst.State != StateHibernated {
		t.Fatalf("state = %s", r.inst.State)
	}
}

// F2.
func TestBusyHibernateOnlyForAHostStop(t *testing.T) {
	r := hostStopReleaseFixture(t)
	assertLeftRunning(t, r.snapshotFixture, r.mgr.HibernateVM(r.vmID), "")
}

// F3, F4, F5.
func TestBusyHibernateRefusesSelfdevMarkerAndUnknownReasons(t *testing.T) {
	for name, tc := range map[string]struct {
		body map[string]any
		want string
	}{
		"selfdev apply":           {map[string]any{"outcome": "busy", "running_runs": 1, "selfdev_active_operations": 1}, busyRefusedSelfdev},
		"planned restart pending": {map[string]any{"outcome": "busy", "reason": "planned_restart_pending"}, busyRefusedReason},
		"unknown reason":          {map[string]any{"outcome": "busy", "reason": "something_new"}, busyRefusedReason},
		"not a busy outcome":      {map[string]any{"error": "conflict"}, busyRefusedReason},
	} {
		t.Run(name, func(t *testing.T) {
			r := hostStopReleaseFixture(t)
			r.guest.busyBody = tc.body
			assertLeftRunning(t, r.snapshotFixture, r.mgr.HibernateVMForHostStop(context.Background(), r.vmID, true), tc.want)
		})
	}
}

// F6.
func TestBusyHibernateRefusesAGuestOffTheCurrentRelease(t *testing.T) {
	t.Run("behind", func(t *testing.T) {
		r := hostStopReleaseFixture(t)
		r.point(t, relR2) // a deploy moved the link; the guest still runs R1
		assertLeftRunning(t, r.snapshotFixture, r.mgr.HibernateVMForHostStop(context.Background(), r.vmID, true), busyRefusedRelease)
	})
	t.Run("legacy launch", func(t *testing.T) {
		f := newSnapshotFixture(t)
		f.serveLaunchAPI(t)
		f.guest.prepareStatus = http.StatusConflict
		f.guest.busyBody = busyGuestBody()
		assertLeftRunning(t, f, f.mgr.HibernateVMForHostStop(context.Background(), f.vmID, true), busyRefusedRelease)
	})
}

// The capture itself, and F10.
func TestBusyHibernateCapturesABusyGuestOnAHostStop(t *testing.T) {
	r := hostStopReleaseFixture(t)
	if err := r.mgr.HibernateVMForHostStop(context.Background(), r.vmID, true); err != nil {
		t.Fatalf("HibernateVMForHostStop: %v", err)
	}
	if r.inst.State != StateHibernated {
		t.Fatalf("state = %s, want hibernated", r.inst.State)
	}
	calls := strings.Join(r.fc.paths(), ",")
	if !strings.Contains(calls, "PATCH /vm") || !strings.Contains(calls, "PUT /snapshot/create") {
		t.Fatalf("calls = %s, want a pause and a snapshot", calls)
	}
	dir := r.mgr.snapshotDir(r.vmID)
	meta, err := readSnapshotMeta(dir)
	if err != nil {
		t.Fatalf("meta: %v", err)
	}
	if !meta.BusyHostStop || meta.GuestRelease != relR1 {
		t.Fatalf("meta busy=%v release=%q, want a busy snapshot of R1", meta.BusyHostStop, meta.GuestRelease)
	}
	// The busy reply carried no capability expiry and no credential_resume:
	// nothing is invented for it.
	if meta.CredentialResume || !meta.ResumeDeadline.IsZero() {
		t.Fatalf("meta credential_resume=%v resume_deadline=%s, want neither", meta.CredentialResume, meta.ResumeDeadline)
	}
	tl := readHibernateTimeline(t, r.mgr, r.vmID)
	if tl.Snapshot == nil || tl.Snapshot.Outcome != "snapshot" || !tl.Snapshot.Busy {
		t.Fatalf("timeline = %+v, want a busy snapshot", tl.Snapshot)
	}
	if r.guest.prepares != 1 {
		t.Fatalf("prepares = %d, want 1 (the fence is asked once)", r.guest.prepares)
	}
	if got := r.validate(dir, meta); got != "" {
		t.Fatalf("validate on the same release = %q, want a pass", got)
	}
	// It resumes once, as any snapshot: the guest is told it resumed.
	var args []string
	r.restoreStarter(t, &fakeFirecrackerAPI{}, &args)
	if _, err := r.mgr.ResumeVMFromSnapshot(r.vmID, r.expect, nil); err != nil {
		t.Fatalf("ResumeVMFromSnapshot: %v", err)
	}
	if r.guest.resumed != 1 {
		t.Fatalf("resumed calls = %d, want 1", r.guest.resumed)
	}
}

// A fenced (idle) guest on a host stop takes the ordinary snapshot, unmarked.
func TestHostStopOfAnIdleGuestIsAnOrdinarySnapshot(t *testing.T) {
	r := hostStopReleaseFixture(t)
	r.guest.prepareStatus = http.StatusOK
	if err := r.mgr.HibernateVMForHostStop(context.Background(), r.vmID, true); err != nil {
		t.Fatal(err)
	}
	meta, err := readSnapshotMeta(r.mgr.snapshotDir(r.vmID))
	if err != nil {
		t.Fatal(err)
	}
	if meta.BusyHostStop || meta.GuestBuild != "test-build" {
		t.Fatalf("meta busy=%v build=%q, want an ordinary fenced snapshot", meta.BusyHostStop, meta.GuestBuild)
	}
}

// F7, F9.
func TestBusySnapshotWakeRules(t *testing.T) {
	capture := func(t *testing.T) (*releaseFixture, string, *SnapshotMeta) {
		r := hostStopReleaseFixture(t)
		if err := r.mgr.HibernateVMForHostStop(context.Background(), r.vmID, true); err != nil {
			t.Fatal(err)
		}
		dir := r.mgr.snapshotDir(r.vmID)
		meta, err := readSnapshotMeta(dir)
		if err != nil {
			t.Fatal(err)
		}
		return r, dir, meta
	}
	t.Run("F7 release changed", func(t *testing.T) {
		r, dir, meta := capture(t)
		r.point(t, relR2)
		if got := r.validate(dir, meta); got != snapshotReasonBusyReleaseChanged {
			t.Fatalf("validate = %q, want %s (an idle snapshot would resume behind)", got, snapshotReasonBusyReleaseChanged)
		}
	})
	t.Run("F7 release missing from meta", func(t *testing.T) {
		r, dir, meta := capture(t)
		meta.GuestRelease = ""
		if got := r.validate(dir, meta); got != snapshotReasonBusyReleaseChanged {
			t.Fatalf("validate = %q, want %s", got, snapshotReasonBusyReleaseChanged)
		}
	})
	t.Run("F9 busy max age", func(t *testing.T) {
		r, dir, meta := capture(t)
		meta.CreatedAt = time.Now().Add(-DefaultBusyHibernateMaxAge - time.Minute)
		if got := r.validate(dir, meta); got != snapshotReasonBusyMaxAge {
			t.Fatalf("validate = %q, want %s", got, snapshotReasonBusyMaxAge)
		}
		r.mgr.cfg.BusyHibernateMaxAge = 3 * time.Hour
		if got := r.validate(dir, meta); got != "" {
			t.Fatalf("validate with a 3h bound = %q, want a pass", got)
		}
	})
}

// F11.
func TestBusyHibernateMaxAgeEnvConfig(t *testing.T) {
	t.Setenv("VM_BUSY_HIBERNATE_MAX_AGE", "20m")
	if got := LoadConfigFromEnv().BusyHibernateMaxAge; got != 20*time.Minute {
		t.Fatalf("max age = %s", got)
	}
	t.Setenv("VM_BUSY_HIBERNATE_MAX_AGE", "")
	if got := NewManager(LoadConfigFromEnv()).busyHibernateMaxAge(); got != DefaultBusyHibernateMaxAge {
		t.Fatalf("default max age = %s", got)
	}
}

// S5 review (both): the host-stop budget bounds a computer's fence, so a
// spent budget never reaches the pause or the snapshot of a stalled guest.
func TestBusyHibernateSpentBudgetNeverPausesTheGuest(t *testing.T) {
	r := hostStopReleaseFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_ = r.mgr.HibernateVMForHostStop(ctx, r.vmID, true)
	for _, call := range r.fc.paths() {
		if call != "GET /" {
			t.Fatalf("a spent budget must not pause or snapshot; calls=%v", r.fc.paths())
		}
	}
}

// S5 review (DeepSeek): a busy snapshot's capability was not renewed at the
// fence, so its default age stays inside the 15 minute renewal grace.
func TestBusyHibernateDefaultAgeIsInsideTheRenewalGrace(t *testing.T) {
	if DefaultBusyHibernateMaxAge > 15*time.Minute {
		t.Fatalf("DefaultBusyHibernateMaxAge = %s, want <= 15m", DefaultBusyHibernateMaxAge)
	}
}
