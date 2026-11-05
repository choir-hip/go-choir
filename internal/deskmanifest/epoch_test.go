package deskmanifest

import "testing"

// Failure modes (MA4.2, written first):
//
//	E1 An update that narrows a verb set cuts a run in flight.
//	   -> FrameAllowedAt resolves the row of the run's epoch.
//	E2 A retained row leaks after its last activation ends.
//	   -> EpochStateNow returns to baseline after the release.
//	E3 An activation adopts an epoch that was dropped.
//	   -> RetainEpoch fails for it.
//	E4 An update replays an old epoch, or carries authority an app lacks.
//	   -> UpdateApp refuses and the live row is unchanged.
//	E5 A retired desk keeps serving new activations.
//	   -> a retired name has no live row; only holds taken earlier resolve.
//	E6 A release called twice drops another holder's row.
//	   -> release is idempotent.
func appDeskFor(t *testing.T, name string, verbs ...string) Desk {
	t.Helper()
	d, err := NewAppDesk(AppDeskSpec{Name: name, Verbs: verbs, Taint: TaintUntrustedInput, UID: 62900, WorkerMaxBytes: 64 * mib})
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func cleanupApp(name string) {
	_ = RetireApp(name)
	mu.Lock()
	delete(held, name)
	mu.Unlock()
}

func TestUpdateNarrowingDoesNotCutARunInFlight(t *testing.T) {
	const name = "app:epochnarrow"
	t.Cleanup(func() { cleanupApp(name) })
	base := EpochStateNow()
	if err := RegisterAppAt(appDeskFor(t, name, "Cast", "ReadCapture", "RequestLease"), 10); err != nil {
		t.Fatal(err)
	}
	if CurrentEpoch(name) != 10 {
		t.Fatalf("epoch %d", CurrentEpoch(name))
	}
	release, ok := RetainEpoch(name, 10)
	if !ok {
		t.Fatal("cannot retain the live epoch")
	}
	if err := UpdateApp(appDeskFor(t, name, "ReadCapture"), 20); err != nil {
		t.Fatal(err)
	}
	// The run that started at epoch 10 finishes under its row.
	if !FrameAllowedAt(name, 10, "request_lease") || !FrameAllowedAt(name, 10, "read_capture") {
		t.Fatal("the old epoch's frame was refused after the update")
	}
	// A new activation, at epoch 20, has the narrowed row.
	if FrameAllowedAt(name, 20, "request_lease") || !FrameAllowedAt(name, 20, "read_capture") {
		t.Fatal("the new epoch must hold exactly the narrowed row")
	}
	if FrameAllowedAt(name, 15, "read_capture") || FrameAllowedAt(name, 0, "read_capture") {
		t.Fatal("an epoch no row was held at must serve nothing")
	}
	if got := EpochStateNow(); got.Retained != base.Retained+1 || got.Holders != 1 {
		t.Fatalf("while held: %+v from %+v", got, base)
	}
	release()
	release() // E6
	if FrameAllowedAt(name, 10, "request_lease") {
		t.Fatal("the old row outlived its last holder")
	}
	if _, ok := RetainEpoch(name, 10); ok {
		t.Fatal("a dropped epoch was adopted") // E3
	}
	cleanupApp(name)
	if got := EpochStateNow(); got != base {
		t.Fatalf("registry state %+v did not return to baseline %+v", got, base) // E2
	}
}

func TestUpdateRefusals(t *testing.T) {
	const name = "app:epochrefuse"
	t.Cleanup(func() { cleanupApp(name) })
	if err := RegisterAppAt(appDeskFor(t, name, "Cast"), 5); err != nil {
		t.Fatal(err)
	}
	if err := UpdateApp(appDeskFor(t, name, "Cast", "Ask"), 5); err == nil {
		t.Error("an update at the live epoch was accepted")
	}
	if err := UpdateApp(appDeskFor(t, name, "Cast", "Ask"), 4); err == nil {
		t.Error("an update at an older epoch was accepted")
	}
	wide := appDeskFor(t, name, "Cast")
	wide.Verbs = append(wide.Verbs, "Exec")
	wide.Grants = append(wide.Grants, G("capsule.mutate"))
	if err := UpdateApp(wide, 6); err == nil {
		t.Error("an update that carries a privileged grant was accepted")
	}
	if err := UpdateApp(appDeskFor(t, "app:epochnone", "Cast"), 6); err == nil {
		t.Error("an update of an unregistered desk was accepted")
	}
	if d, _ := Lookup(name); len(d.Verbs) != 1 || CurrentEpoch(name) != 5 {
		t.Errorf("a refused update changed the live row: %v@%d", d.Verbs, CurrentEpoch(name))
	}
	if err := RegisterAppAt(appDeskFor(t, "app:epochzero", "Cast"), 0); err == nil {
		t.Error("epoch 0 is the built-in epoch")
	}
}

func TestRetiredDeskServesOnlyEarlierHolds(t *testing.T) {
	const name = "app:epochretire"
	t.Cleanup(func() { cleanupApp(name) })
	base := EpochStateNow()
	if err := RegisterAppAt(appDeskFor(t, name, "ReadCapture"), 7); err != nil {
		t.Fatal(err)
	}
	release, _ := RetainEpoch(name, 7)
	if err := RetireApp(name); err != nil {
		t.Fatal(err)
	}
	if _, ok := Lookup(name); ok {
		t.Fatal("a retired desk still has a live row")
	}
	// A second activation of the same epoch, already admitted, still resolves.
	if _, ok := RetainEpoch(name, 7); !ok {
		t.Fatal("the hold taken before the retire must still resolve")
	}
	if !FrameAllowedAt(name, 7, "read_capture") {
		t.Fatal("the in-flight worker was cut by the retire")
	}
	release()
	// The second hold above is still out; drop the retained row directly.
	mu.Lock()
	delete(held, name)
	mu.Unlock()
	if FrameAllowedAt(name, 7, "read_capture") {
		t.Fatal("a retired desk serves after its holds ended")
	}
	if got := EpochStateNow(); got != base {
		t.Fatalf("state %+v, baseline %+v", got, base)
	}
}

func TestBuiltinEpochIsZeroAndRetainable(t *testing.T) {
	for _, name := range Names() {
		d, _ := Lookup(name)
		if d.Kind == KindApp {
			continue
		}
		if CurrentEpoch(name) != 0 {
			t.Errorf("%s epoch %d", name, CurrentEpoch(name))
		}
		release, ok := RetainEpoch(name, 0)
		if !ok {
			t.Errorf("%s: cannot retain epoch 0", name)
			continue
		}
		release()
	}
	if !FrameAllowedAt("research", 0, "search_start") || FrameAllowedAt("research", 1, "search_start") {
		t.Error("FrameAllowedAt must match FrameAllowed at the live epoch and deny others")
	}
	if FrameAllowedAt("nobody", 0, "read") {
		t.Error("an unknown desk is never served")
	}
}
