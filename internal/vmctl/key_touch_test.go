package vmctl

// Gap K (docs/problems/idle-sweep-hibernates-a-computer-just-after-its-apply-restart-2026-10-24.md):
// the API-key path never refreshed LastActiveAt, so the idle sweep hibernated
// a computer a CLI was driving. The touch moves the idle clock of an active
// computer and nothing else.

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func touchFixture(state VMState, lastActive time.Time) (*OwnershipRegistry, *VMOwnership) {
	reg := NewOwnershipRegistry("")
	own := &VMOwnership{
		VMID: "vm-touch-1", ComputerID: "computer-touch-1", UserID: "user-touch", DesktopID: PrimaryDesktopID,
		State: state, ComputerURL: "http://10.0.0.9:8080", Epoch: 4, LastActiveAt: lastActive,
	}
	reg.ownerships[ownershipKey("user-touch", PrimaryDesktopID)] = own
	return reg, own
}

func TestKeyOnlyTrafficKeepsAComputerOutOfTheIdleSweep(t *testing.T) {
	now := time.Now()
	idleTimeout := 30 * time.Minute
	reg, own := touchFixture(VMStateActive, now.Add(-31*time.Minute))
	sweep := func() int {
		return len(idleOwnershipCandidates([]*VMOwnership{own}, WarmnessPolicyConfig{}, HostPressureSample{}, idleTimeout, now, nil))
	}
	if sweep() != 1 {
		t.Fatal("precondition: a computer idle for 31 minutes must be a sweep candidate")
	}
	if r := reg.TouchComputerActivity("user-touch", "computer-touch-1", now); !r.Found || !r.Touched {
		t.Fatalf("touch = %+v", r)
	}
	if sweep() != 0 {
		t.Fatal("a touched computer was still an idle sweep candidate")
	}
}

func TestTouchNeverWakesOrChangesANonActiveComputer(t *testing.T) {
	for _, state := range []VMState{VMStateHibernated, VMStateStopped, VMStateStopping, VMStateBooting} {
		old := time.Now().Add(-2 * time.Hour)
		reg, own := touchFixture(state, old)
		before := *own
		r := reg.TouchComputerActivity("user-touch", "computer-touch-1", time.Now())
		if !r.Found || r.Touched || r.State != state {
			t.Errorf("%s: touch = %+v", state, r)
		}
		if own.State != before.State || !own.LastActiveAt.Equal(old) || own.ComputerURL != before.ComputerURL || own.Epoch != before.Epoch || own.VMID != before.VMID {
			t.Errorf("%s: ownership changed: %+v -> %+v", state, before, *own)
		}
	}
}

func TestTouchHonorsTheFloorAndTheExactUserJoin(t *testing.T) {
	now := time.Now()
	recent := now.Add(-10 * time.Second)
	reg, own := touchFixture(VMStateActive, recent)
	if r := reg.TouchComputerActivity("user-touch", "computer-touch-1", now); r.Touched || !own.LastActiveAt.Equal(recent) {
		t.Fatalf("touch inside the %s floor wrote: %+v", keyTouchMinInterval, r)
	}
	later := now.Add(keyTouchMinInterval)
	if r := reg.TouchComputerActivity("user-touch", "computer-touch-1", later); !r.Touched || !own.LastActiveAt.Equal(later) {
		t.Fatalf("touch past the floor did not write: %+v", r)
	}
	stale := later.Add(-time.Hour)
	own.LastActiveAt = stale
	for _, pair := range [][2]string{{"user-other", "computer-touch-1"}, {"user-touch", "computer-other"}, {"", "computer-touch-1"}, {"user-touch", ""}} {
		if r := reg.TouchComputerActivity(pair[0], pair[1], later); r.Found || r.Touched || !own.LastActiveAt.Equal(stale) {
			t.Errorf("touch %v = %+v, last active moved to %v", pair, r, own.LastActiveAt)
		}
	}
}

func touchRequestFor(body string) *http.Request {
	request := httptest.NewRequest(http.MethodPost, "/internal/vmctl/touch", strings.NewReader(body))
	request.RemoteAddr = "127.0.0.1:4444"
	return request
}

func TestHandleTouchIsInternalOnlyAdditiveAndSwitchable(t *testing.T) {
	old := time.Now().Add(-time.Hour)
	reg, own := touchFixture(VMStateActive, old)
	handler := NewHandler(reg)
	body := `{"user_id":"user-touch","computer_id":"computer-touch-1"}`

	// A non-loopback caller is refused and nothing moves.
	remote := touchRequestFor(body)
	remote.RemoteAddr = "10.0.0.1:4444"
	rec := httptest.NewRecorder()
	handler.HandleTouch(rec, remote)
	if rec.Code != http.StatusForbidden || !own.LastActiveAt.Equal(old) {
		t.Fatalf("remote caller: %d, last active %v", rec.Code, own.LastActiveAt)
	}

	// GET is refused.
	get := httptest.NewRequest(http.MethodGet, "/internal/vmctl/touch", nil)
	get.RemoteAddr = "127.0.0.1:4444"
	rec = httptest.NewRecorder()
	handler.HandleTouch(rec, get)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET status = %d", rec.Code)
	}

	// A malformed or incomplete body is a 400.
	for _, bad := range []string{`{`, `{"user_id":"user-touch"}`, `{"computer_id":"computer-touch-1"}`} {
		rec = httptest.NewRecorder()
		handler.HandleTouch(rec, touchRequestFor(bad))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("body %q status = %d", bad, rec.Code)
		}
	}

	// An unknown pair is a 404; another user's pair does not touch.
	rec = httptest.NewRecorder()
	handler.HandleTouch(rec, touchRequestFor(`{"user_id":"user-other","computer_id":"computer-touch-1"}`))
	if rec.Code != http.StatusNotFound || !own.LastActiveAt.Equal(old) {
		t.Fatalf("foreign user: %d, last active %v", rec.Code, own.LastActiveAt)
	}

	// The switch turns the route into a no-op that still answers 200.
	t.Setenv("VMCTL_KEY_TOUCH", "off")
	rec = httptest.NewRecorder()
	handler.HandleTouch(rec, touchRequestFor(body))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"reason":"disabled"`) || !own.LastActiveAt.Equal(old) {
		t.Fatalf("switch off: %d %s, last active %v", rec.Code, rec.Body.String(), own.LastActiveAt)
	}

	// On (the default), it moves the clock and reports it.
	t.Setenv("VMCTL_KEY_TOUCH", "")
	rec = httptest.NewRecorder()
	handler.HandleTouch(rec, touchRequestFor(body))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"touched":true`) || !own.LastActiveAt.After(old) {
		t.Fatalf("switch on: %d %s, last active %v", rec.Code, rec.Body.String(), own.LastActiveAt)
	}
}
