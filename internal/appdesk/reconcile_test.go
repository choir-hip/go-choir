package appdesk

import (
	"runtime"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
)

// Failure modes (MA4.2, written first):
//
//	R1 An update that narrows a verb set cuts a run in flight.
//	   -> Reconcile updates by epoch; the old row serves its holders.
//	R2 Reconcile twice, or after a crash between an event and its reconcile,
//	   registers a desk twice or changes it.
//	   -> idempotent: the second pass over the same set changes nothing, and a
//	      fresh registry rebuilt from the tape lands at the same epochs.
//	R3 The registry ahead of the tape (a row the events do not justify) is
//	   silently overwritten.
//	   -> an error, the row left as it is.
//	R4 Update, retire and reinstall cycles leak rows, holds or goroutines.
//	   -> the registry state and goroutine count return to baseline.
//	R5 An update changes the desk's uid or keeps a dropped kind's writer.
//	   -> the uid is the install's; a dropped kind is retired.
func update(seq uint64, m Manifest) Event {
	return Event{Seq: seq, Action: ActionUpdate, Package: m.Package, ManifestDigest: Digest(m), Manifest: &m}
}

func cleanupAll(names ...string) {
	for _, n := range names {
		_ = deskmanifest.RetireApp(n)
	}
}

func TestDeriveUpdateSetsTheEpochAndKeepsTheUID(t *testing.T) {
	m1 := goodManifest("epochpkg")
	m2 := goodManifest("epochpkg")
	m2.Verbs = []string{"ReadCapture"}
	m2.OwnedKinds = []string{"choir.app.epochpkg.other_note"}
	set, refusals := Derive([]Event{install(3, m1), update(9, m2)})
	if len(refusals) != 0 {
		t.Fatalf("refusals %+v", refusals)
	}
	if set.Epochs["app:epochpkg"] != 9 || len(set.Desks) != 1 {
		t.Fatalf("epochs %v desks %d", set.Epochs, len(set.Desks))
	}
	if got := set.Desks[0].Sandbox.UID; got != uidBase {
		t.Errorf("uid %d changed by the update", got)
	}
	if w, live, ok := set.Writer("choir.app.epochpkg.episode_note"); !ok || live || w == "" {
		t.Errorf("the dropped kind must be retired, got writer %q live %v ok %v", w, live, ok)
	}
	if w, live, ok := set.Writer("choir.app.epochpkg.other_note"); !ok || !live || w != "desk:epochpkg" {
		t.Errorf("the new kind must be live: %q %v %v", w, live, ok)
	}
	// An update of a package that is not installed, or with a bad digest, is refused.
	_, refusals = Derive([]Event{update(2, m2)})
	if !hasCode(refusals, CodeNotInstalled) {
		t.Errorf("update of an absent package: %+v", refusals)
	}
	bad := update(10, m2)
	bad.ManifestDigest = "sha256:00"
	if _, refusals = Derive([]Event{install(3, m1), bad}); !hasCode(refusals, CodeDigestMismatch) {
		t.Errorf("update with a wrong digest: %+v", refusals)
	}
	wide := m2
	wide.Verbs = []string{"Exec"}
	if _, refusals = Derive([]Event{install(3, m1), update(10, wide)}); !hasCode(refusals, CodeVerbNotApp) {
		t.Errorf("update with a privileged verb: %+v", refusals)
	}
}

func TestReconcileUpdateNarrowsWithoutCuttingARunInFlight(t *testing.T) {
	m1 := goodManifest("reconun")
	m1.Verbs = []string{"Cast", "ReadCapture", "RequestLease"}
	m2 := m1
	m2.Verbs = []string{"ReadCapture"}
	t.Cleanup(func() { cleanupAll("app:reconun") })

	set1, _ := Derive([]Event{install(4, m1)})
	if err := Reconcile(set1); err != nil {
		t.Fatal(err)
	}
	release, ok := deskmanifest.RetainEpoch("app:reconun", 4)
	if !ok {
		t.Fatal("cannot hold the installed epoch")
	}
	set2, _ := Derive([]Event{install(4, m1), update(8, m2)})
	if err := Reconcile(set2); err != nil {
		t.Fatal(err)
	}
	if !deskmanifest.FrameAllowedAt("app:reconun", 4, "request_lease") {
		t.Fatal("the run in flight lost a verb to the update")
	}
	if deskmanifest.FrameAllowedAt("app:reconun", 8, "request_lease") {
		t.Fatal("a new activation kept the dropped verb")
	}
	release()
	if deskmanifest.FrameAllowedAt("app:reconun", 4, "request_lease") {
		t.Fatal("the old row outlived its last holder")
	}
}

func TestReconcileIsIdempotentAndRebuildsFromTheTape(t *testing.T) {
	m := goodManifest("reconidem")
	events := []Event{install(5, m)}
	t.Cleanup(func() { cleanupAll("app:reconidem") })
	set, _ := Derive(events)
	for i := 0; i < 3; i++ {
		if err := Reconcile(set); err != nil {
			t.Fatalf("pass %d: %v", i, err)
		}
	}
	if deskmanifest.CurrentEpoch("app:reconidem") != 5 {
		t.Fatalf("epoch %d", deskmanifest.CurrentEpoch("app:reconidem"))
	}
	// A crash between the event and its reconcile: the process restarts with an
	// empty app registry and the tape alone rebuilds it at the same epoch.
	cleanupAll("app:reconidem")
	if _, ok := deskmanifest.Lookup("app:reconidem"); ok {
		t.Fatal("setup: desk still registered")
	}
	set, _ = Derive(events)
	if err := Reconcile(set); err != nil {
		t.Fatal(err)
	}
	if deskmanifest.CurrentEpoch("app:reconidem") != 5 {
		t.Fatalf("rebuilt epoch %d", deskmanifest.CurrentEpoch("app:reconidem"))
	}
}

func TestReconcileRefusesARegistryAheadOfTheTape(t *testing.T) {
	m1 := goodManifest("reconahead")
	m2 := m1
	m2.Verbs = []string{"ReadCapture"}
	t.Cleanup(func() { cleanupAll("app:reconahead") })
	set, _ := Derive([]Event{install(2, m1), update(6, m2)})
	if err := Reconcile(set); err != nil {
		t.Fatal(err)
	}
	older, _ := Derive([]Event{install(2, m1)})
	if err := Reconcile(older); err == nil {
		t.Fatal("a registry ahead of the tape was overwritten")
	}
	if d, _ := deskmanifest.Lookup("app:reconahead"); len(d.Verbs) != 1 {
		t.Errorf("the refused reconcile changed the live row: %v", d.Verbs)
	}
	// The same epoch with a different row is also an error, not a silent swap.
	forged, _ := Derive([]Event{install(2, m1), update(6, m1)})
	if err := Reconcile(forged); err == nil {
		t.Fatal("a different row at the live epoch was accepted")
	}
}

func TestReconcileCyclesLeakNothing(t *testing.T) {
	base := deskmanifest.EpochStateNow()
	goroutines := runtime.NumGoroutine()
	const pkg = "reconleak"
	for i := 0; i < 20; i++ {
		seq := uint64(10 * (i + 1))
		m1 := goodManifest(pkg)
		m2 := m1
		m2.Verbs = []string{"ReadCapture"}
		set, _ := Derive([]Event{install(seq, m1)})
		if err := Reconcile(set); err != nil {
			t.Fatal(err)
		}
		release, ok := deskmanifest.RetainEpoch("app:"+pkg, seq)
		if !ok {
			t.Fatal("hold failed")
		}
		set, _ = Derive([]Event{install(seq, m1), update(seq+1, m2)})
		if err := Reconcile(set); err != nil {
			t.Fatal(err)
		}
		release()
		set, _ = Derive([]Event{install(seq, m1), update(seq+1, m2), disable(seq+2, pkg)})
		if err := Reconcile(set); err != nil {
			t.Fatal(err)
		}
		// Each cycle reinstalls from a fresh tape: the retired name is a tape
		// fact, so the registry starts empty again.
		if _, ok := deskmanifest.Lookup("app:" + pkg); ok {
			t.Fatal("the disabled desk is still registered")
		}
	}
	time.Sleep(20 * time.Millisecond)
	if got := deskmanifest.EpochStateNow(); got != base {
		t.Fatalf("registry state %+v, baseline %+v", got, base)
	}
	if got := runtime.NumGoroutine(); got > goroutines+2 {
		t.Fatalf("goroutines %d, baseline %d", got, goroutines)
	}
}
