package autoputer

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/appdesk"
	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/store"
)

// MA4.4 on the real store: a computer restored to a head before a required
// package was installed boots with the package re-installed by a new event
// after that head, and the log survives a reopen.
func TestRestoreToAPreInstallHeadReinstallsTheRequiredSet(t *testing.T) {
	dir := t.TempDir()
	req := []appdesk.Required{{Package: "reqstore", Version: "1.0.0", Manifest: appdesk.Manifest{
		Package: "reqstore", Verbs: []string{"Cast", "ReadCapture"}, WorkerMaxBytes: 64 << 20,
		OwnedKinds: []string{"choir.app.reqstore.note"}}}}
	t.Cleanup(func() { _ = deskmanifest.RetireApp("app:reqstore") })

	boot := func(path string) (*store.Store, appdesk.Set) {
		st, err := store.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = st.Close() })
		set, refusals, err := appdesk.Boot(context.Background(), deskLog{st: st}, req)
		if err != nil || len(refusals) != 0 {
			t.Fatalf("boot: %v %+v", err, refusals)
		}
		return st, set
	}
	path := filepath.Join(dir, "state.db")
	st, set := boot(path)
	if _, ok := deskmanifest.Lookup("app:reqstore"); !ok || set.Versions["reqstore"] != "1.0.0" {
		t.Fatal("the required package is not installed after the first boot")
	}
	first, _ := st.ListDeskPackageEvents(context.Background())
	if len(first) != 1 {
		t.Fatalf("log after first boot: %d events", len(first))
	}
	// A second boot of the same store appends nothing.
	_ = deskmanifest.RetireApp("app:reqstore")
	st2, _ := boot(path)
	if again, _ := st2.ListDeskPackageEvents(context.Background()); len(again) != 1 {
		t.Fatalf("a second boot appended: %d events", len(again))
	}
	// Restore to a pre-install head: a store with no log.
	_ = deskmanifest.RetireApp("app:reqstore")
	restored, set := boot(filepath.Join(dir, "restored.db"))
	rows, _ := restored.ListDeskPackageEvents(context.Background())
	if len(rows) != 1 || set.Versions["reqstore"] != "1.0.0" {
		t.Fatalf("restored head: %d events, versions %v", len(rows), set.Versions)
	}
	if _, ok := deskmanifest.Lookup("app:reqstore"); !ok {
		t.Fatal("the restored computer booted without its required package")
	}
	if deskmanifest.CurrentEpoch("app:reqstore") != rows[0].Seq {
		t.Errorf("epoch %d, want the new install event's %d", deskmanifest.CurrentEpoch("app:reqstore"), rows[0].Seq)
	}
}

func TestTheImagesRequiredSetParsesAndBuildsAnOption(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if _, err := deskReconcileOption(st); err != nil {
		t.Fatal(err)
	}
}
