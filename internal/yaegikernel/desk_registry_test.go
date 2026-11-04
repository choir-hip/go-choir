package yaegikernel

import (
	"testing"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
)

// deskModuleSets once excluded KindApp and was a snapshot taken at init, so an
// app desk registered later had no module set. It is a live registry lookup.
func TestDeskModuleSetsIncludeRegisteredAppDesks(t *testing.T) {
	d, err := deskmanifest.NewAppDesk(deskmanifest.AppDeskSpec{Name: "app:modsets", Verbs: []string{"Cast", "Done"}, Taint: deskmanifest.TaintUntrustedInput, UID: 62502, WorkerMaxBytes: 64 << 20})
	if err != nil {
		t.Fatal(err)
	}
	if err := deskmanifest.RegisterApp(d); err != nil {
		t.Fatal(err)
	}
	defer deskmanifest.RetireApp("app:modsets")
	got, ok := deskModuleSets()["app:modsets"]
	if !ok || len(got) != 2 {
		t.Fatalf("app desk module set = %v, %v", got, ok)
	}
	if _, ok := deskModuleSets()[deskmanifest.Conductor]; ok {
		t.Fatal("a carrier-less desk has a module set")
	}
}

// The Updates observation is the module updates.read, granted by the row to
// texture and management only (F12).
func TestUpdatesObservationFollowsTheModule(t *testing.T) {
	for _, name := range deskmanifest.Names() {
		d, _ := deskmanifest.Lookup(name)
		s := &ChoirScope{desk: name}
		want := d.HasModule(deskmanifest.ModuleUpdatesRead)
		if got := s.legacyObservation("Updates"); got != want {
			t.Errorf("%s legacyObservation(Updates) = %v, want %v", name, got, want)
		}
		if s.legacyObservation("Inbox") {
			t.Errorf("%s reads the retired Inbox", name)
		}
		if want != (name == deskmanifest.Texture || name == deskmanifest.Management) {
			t.Errorf("%s updates.read = %v", name, want)
		}
	}
	var nilScope *ChoirScope
	if nilScope.legacyObservation("Updates") {
		t.Fatal("nil scope observes")
	}
}
