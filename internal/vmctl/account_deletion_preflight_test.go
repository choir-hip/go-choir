package vmctl

// Round 2, seats 1 and 2: the purge must check every precondition of every
// step that can refuse BEFORE anything irreversible, and a destroy must report
// what it did even when it fails halfway, and must not leave an ownership on
// disk for state it destroyed.
//
// Failure list (written before the code):
//  P1 preflight touches something, or passes for an account that destroy would
//     refuse (no record, a young record, a protected computer, a hold);
//  P2 a destroy of two computers that fails on the second reports nothing of
//     the first, so the caller never learns it was destroyed;
//  P3 a destroy whose ownership write fails reports success (a restart would
//     bring the ownership back for state that is gone).

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/purgeguard"
)

func TestPreflightRefusesWhatDestroyRefusesAndTouchesNothing(t *testing.T) { // P1
	e := newAcctEnv(t)
	now := time.Now().UTC()
	if _, err := e.reg.PreflightAccountDestroy(e.user, now); !errors.Is(err, purgeguard.ErrNoRecord) {
		t.Fatalf("no record: %v", err)
	}
	e.request(t, now)
	if _, err := e.reg.PreflightAccountDestroy(e.user, now.Add(time.Hour)); !errors.Is(err, purgeguard.ErrTooEarly) {
		t.Fatalf("young record: %v", err)
	}
	list, err := e.reg.PreflightAccountDestroy(e.user, now.Add(purgeguard.GracePeriod))
	if err != nil || len(list) != 1 {
		t.Fatalf("old record: %v %v", list, err)
	}
	e.intact(t) // nothing destroyed, nothing stopped
	if len(e.mgr.stops) != 0 {
		t.Fatalf("preflight stopped something: %v", e.mgr.stops)
	}
	// A held computer: refused before anything is touched.
	if err := e.reg.SetHold(e.own.ComputerID, "maintenance", "test"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.reg.PreflightAccountDestroy(e.user, now.Add(purgeguard.GracePeriod)); !errors.Is(err, ErrAccountRefused) {
		t.Fatalf("held: %v", err)
	}
	// The route, host-only.
	h := NewHandler(e.reg)
	req := httptest.NewRequest(http.MethodPost, "/internal/vmctl/account/preflight", strings.NewReader(`{"user_id":"`+e.user+`"}`))
	req.RemoteAddr = "10.0.0.7:5555"
	rec := httptest.NewRecorder()
	h.HandleAccountPreflight(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("preflight from a non-host: %d", rec.Code)
	}
}

// failsSecond fails DestroyVMState for one VM id.
type failsSecond struct {
	*mockVMManager
	failVM string
}

func (f *failsSecond) DestroyVMState(vmID string) error {
	if vmID == f.failVM {
		return errors.New("disk busy")
	}
	return f.mockVMManager.DestroyVMState(vmID)
}

func TestAPartialDestroyReportsWhatWasDestroyed(t *testing.T) { // P2
	e := newAcctEnv(t)
	e.request(t, time.Now().Add(-10*purgeguard.GracePeriod))
	second, err := e.reg.ResolveOrAssignDesktop(e.user, "capsule-2")
	if err != nil {
		t.Fatal(err)
	}
	e.reg.mu.Lock()
	e.reg.ownerships[ownershipKey(e.user, "capsule-2")].State = VMStateStopped
	e.reg.mu.Unlock()
	e.reg.SetVMManager(&failsSecond{mockVMManager: e.mgr, failVM: second.VMID})

	res, err := e.reg.DestroyAccountComputers(e.user, time.Now())
	if err == nil {
		t.Fatal("a failed destroy reported success")
	}
	if len(res.Computers) != 1 || res.Computers[0] != e.own.ComputerID || len(res.VMIDs) != 1 || res.VMIDs[0] != e.own.VMID {
		t.Fatalf("what was destroyed is not reported: %+v", res)
	}
	// Over the route the same answer comes back beside the error.
	e2 := newAcctEnv(t)
	e2.request(t, time.Now().Add(-10*purgeguard.GracePeriod))
	second2, _ := e2.reg.ResolveOrAssignDesktop(e2.user, "capsule-2")
	e2.reg.mu.Lock()
	e2.reg.ownerships[ownershipKey(e2.user, "capsule-2")].State = VMStateStopped
	e2.reg.mu.Unlock()
	e2.reg.SetVMManager(&failsSecond{mockVMManager: e2.mgr, failVM: second2.VMID})
	h := NewHandler(e2.reg)
	req := httptest.NewRequest(http.MethodPost, "/internal/vmctl/account/destroy", strings.NewReader(`{"user_id":"`+e2.user+`"}`))
	req.RemoteAddr = "127.0.0.1:4000"
	rec := httptest.NewRecorder()
	h.HandleAccountDestroy(rec, req)
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), e2.own.ComputerID) || !strings.Contains(rec.Body.String(), e2.own.VMID) {
		t.Fatalf("partial destroy over the route: %d %s", rec.Code, rec.Body)
	}
}

func TestADestroyWhoseOwnershipWriteFailsIsAFailureNotASuccess(t *testing.T) { // P3
	e := newAcctEnv(t)
	e.request(t, time.Now().Add(-10*purgeguard.GracePeriod))
	dir := t.TempDir()
	// A regular file where the directory should be: the write cannot succeed.
	blocker := filepath.Join(dir, "blocked")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	e.reg.mu.Lock()
	e.reg.persistencePath = filepath.Join(blocker, "ownerships.json")
	e.reg.mu.Unlock()
	res, err := e.reg.DestroyAccountComputers(e.user, time.Now())
	if err == nil {
		t.Fatalf("a destroy whose ownership write failed reported success: %+v", res)
	}
	if e.reg.GetOwnership(e.user) == nil {
		t.Fatal("the in-memory ownership was dropped though the file still names it; a retry would not find it")
	}
	// The disk heals: the retry destroys again (idempotent) and succeeds.
	e.reg.mu.Lock()
	e.reg.persistencePath = filepath.Join(dir, "ok", "ownerships.json")
	e.reg.mu.Unlock()
	if res, err := e.reg.DestroyAccountComputers(e.user, time.Now()); err != nil || res.VMStates != 1 {
		t.Fatalf("retry: %+v %v", res, err)
	}
}
