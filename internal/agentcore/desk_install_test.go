package agentcore

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/appdesk"
	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/pkgrelease"
	"github.com/yusefmosiah/go-choir/internal/pkgruntime"
	"github.com/yusefmosiah/go-choir/internal/promptroute"
)

// A package install registers the desks its manifest contributes (drill P4;
// definition choir-modular-architecture-2026-11-04).
//
// Failure modes, written before the code:
//
//	D1  an installed first-party desk package does not join the desk set, so
//	    the drill desk is unreachable without a reboot;
//	D2  an uninstall leaves the desk registered, or a reinstall is refused as
//	    "disabled; needs re-approval";
//	D3  a restart forgets an install's desk (the log is empty, the install
//	    record is the fact);
//	D4  a stranger's release of a first-party desk package, or a desk of a
//	    package the image ships no desk for, registers a desk (R24);
//	D5  the desk row an install registers is anything but the image's own
//	    manifest (verbs, ceiling);
//	D6  the installed desk cannot be reached by the owner's prompt: it is not
//	    a router candidate, or the router's hand-off lands nowhere.

const drillPkg = "mail-reader-drill"

type memDeskLog struct {
	mu     sync.Mutex
	events []appdesk.Event
}

func (l *memDeskLog) Events(context.Context) ([]appdesk.Event, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]appdesk.Event(nil), l.events...), nil
}

func (l *memDeskLog) Append(_ context.Context, ev appdesk.Event) (uint64, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	ev.Seq = uint64(len(l.events) + 1)
	l.events = append(l.events, ev)
	return ev.Seq, nil
}

// drillManifestBuilder is the fake builder with the release's real manifest in the
// module, as the real builder carries it.
type drillManifestBuilder struct {
	fakeModuleBuilder
	manifest []byte
}

func (b *drillManifestBuilder) Build(ctx context.Context, rec pkgrelease.Record, treePatch []byte) (pkgruntime.Module, error) {
	m, err := b.fakeModuleBuilder.Build(ctx, rec, treePatch)
	m.Manifest = json.RawMessage(b.manifest)
	return m, err
}

type drillEnv struct {
	*installEnv
	log   *memDeskLog
	files map[string]string
}

func newDrillEnv(t *testing.T) *drillEnv {
	t.Helper()
	e := newInstallEnv(t)
	read := func(name string) string {
		raw, err := os.ReadFile("testdata/" + drillPkg + "/" + name)
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}
	files := map[string]string{"choir-package.json": read("choir-package.json"), "App.svelte": read("App.svelte"), "prompt.md": read("prompt.md")}
	e.rt.packageBuilder = &drillManifestBuilder{manifest: []byte(files["choir-package.json"])}
	// A desk-bearing release owes management's install review (its prompt is
	// behaviour); the drill judges it accepted, as management would.
	e.rt.installReviewSigner = newReviewSigner(t)
	now := time.Now()
	e.rt.installReviewNow = func() time.Time { return now }
	log := &memDeskLog{}
	required, err := appdesk.ImageRequired()
	if err != nil {
		t.Fatal(err)
	}
	e.rt.deskLog, e.rt.deskLogOwner = log, e.owner
	e.rt.deskReconciler = func(ctx context.Context) error {
		_, _, err := appdesk.Boot(ctx, e.rt.deskLog, required)
		return err
	}
	t.Cleanup(func() { _ = appdesk.Reconcile(appdesk.Set{}) })
	return &drillEnv{installEnv: e, log: log, files: files}
}

func (e *drillEnv) publish(t *testing.T, track string) (string, json.RawMessage, []byte) {
	t.Helper()
	_, id, raw, tree := publishedPackageNamed(t, drillPkg, e.repo, e.base, track, e.files)
	return id, raw, tree
}

// install posts the release; a desk owes management's install review, which
// the drill judges accepted, and the verdict installs it with no second owner
// act.
func (e *drillEnv) install(t *testing.T, id string, raw json.RawMessage, tree []byte) {
	t.Helper()
	code, view, reason := e.post(t, installBody(t, id, raw, tree, ""))
	if code == http.StatusAccepted && reason == reasonInstallReviewOwed {
		f := &reviewFixture{e: e.installEnv, id: id, raw: raw, tree: tree}
		if out := f.judge(t, true); out == nil {
			t.Fatal("the drill's accepting verdict was refused")
		}
		code, view, reason = e.post(t, installBody(t, id, raw, tree, ""))
		if code == http.StatusOK {
			code = http.StatusCreated
		}
	}
	if code != http.StatusCreated && code != http.StatusOK || view.State != installStateInstalled {
		t.Fatalf("install = %d %s %+v", code, reason, view.State)
	}
}

func hasDrillDesk() bool {
	d, ok := deskmanifest.Lookup("app:" + drillPkg)
	return ok && d.FirstParty && d.HasVerb("ReadMail") && len(d.LeaseGrants) == 1 && d.LeaseGrants[0] == "mail.read"
}

func TestInstallRegistersTheDeskItsManifestContributes(t *testing.T) {
	e := newDrillEnv(t)

	// D4: a stranger's release of the first-party desk package is refused and
	// registers nothing.
	sid, sraw, stree := e.publish(t, "owner-stranger")
	if code, _, reason := e.post(t, installBody(t, sid, sraw, stree, "")); code != http.StatusForbidden || reason != "desk_track_not_yours" {
		t.Fatalf("D4 stranger = %d %s", code, reason)
	}
	if _, ok := deskmanifest.Lookup("app:" + drillPkg); ok {
		t.Fatal("D4: a stranger's install registered the desk")
	}
	// D4: a desk of a package the image ships no desk for is refused.
	if refusal := refuseDeskInstall(e.owner, pkgrelease.Record{Package: pkgrelease.Package{Name: "notes-desk"}, Track: e.owner,
		Manifest: pkgrelease.Manifest{DesksCount: 1}}); refusal == nil || refusal.reason != pkgrelease.ReasonDeskNeedsSandbox {
		t.Fatalf("D4 other package's desk = %+v", refusal)
	}

	// D1, D5: the owner's own release registers the image's desk.
	id, raw, tree := e.publish(t, e.owner)
	e.install(t, id, raw, tree)
	if !hasDrillDesk() {
		t.Fatal("D1: the installed package's desk is not in the desk set")
	}
	fp, _ := appdesk.FirstPartyDesk(drillPkg)
	if d, _ := deskmanifest.Lookup("app:" + drillPkg); !d.HasVerb("RequestLease") || d.HasVerb("Cast") || len(d.Verbs) != len(fp.Manifest.Verbs) {
		t.Fatalf("D5: the registered row is not the image's manifest: %+v", d.Verbs)
	}

	// D6: the owner's prompt can reach it: the router lists it, and its
	// hand-off record lands in the desk's inbox.
	found := false
	for _, c := range promptroute.Candidates("texture") {
		found = found || c.Desk == "app:"+drillPkg
	}
	if !found {
		t.Fatal("D6: the installed desk is not a router candidate")
	}
	inbox := "app:" + drillPkg + ":" + e.owner
	if _, err := e.rt.DeliverOwnerRequest(context.Background(), e.owner, "run-drill-1", "app:"+drillPkg, "summarize the email msg-a", nil, ""); err != nil {
		t.Fatal(err)
	}
	rec, err := e.rt.store.GetCommitmentRecord(context.Background(), e.owner, e.computer, "owner-request:run-drill-1")
	if err != nil || rec == nil || rec.Inbox != inbox || rec.Addressee != inbox {
		t.Fatalf("D6: the hand-off is not in the desk's inbox: %+v %v", rec, err)
	}

	// D2: an uninstall removes the desk; a reinstall brings it back.
	if code, _, reason := e.post(t, installBody(t, id, raw, nil, "uninstall")); code != http.StatusCreated {
		t.Fatalf("D2 uninstall = %d %s", code, reason)
	}
	if _, ok := deskmanifest.Lookup("app:" + drillPkg); ok {
		t.Fatal("D2: the uninstalled package's desk is still registered")
	}
	e.install(t, id, raw, tree)
	if !hasDrillDesk() {
		t.Fatal("D2: the reinstalled package's desk is not in the desk set")
	}

	// D3: a restart with an empty log registers the desk from the install
	// record.
	_ = appdesk.Reconcile(appdesk.Set{})
	if _, ok := deskmanifest.Lookup("app:" + drillPkg); ok {
		t.Fatal("test setup: the registry was not cleared")
	}
	e.rt.deskLog = &memDeskLog{}
	e.rt.reconcileDesksAtBoot(context.Background())
	if !hasDrillDesk() {
		t.Fatal("D3: a restart did not register the installed package's desk")
	}
	// ... and a restart after an uninstall keeps it gone.
	if code, _, _ := e.post(t, installBody(t, id, raw, nil, "uninstall")); code != http.StatusCreated {
		t.Fatalf("D3 uninstall = %d", code)
	}
	_ = appdesk.Reconcile(appdesk.Set{})
	e.rt.deskLog = &memDeskLog{}
	e.rt.reconcileDesksAtBoot(context.Background())
	if _, ok := deskmanifest.Lookup("app:" + drillPkg); ok {
		t.Fatal("D3: a restart resurrected an uninstalled package's desk")
	}
}

// An unset owner id (the "local" placeholder) never disables an installed desk:
// the installs read under it prove nothing about which desks are gone.
func TestBootSyncUnderAPlaceholderOwnerDisablesNothing(t *testing.T) {
	e := newDrillEnv(t)
	id, raw, tree := e.publish(t, e.owner)
	e.install(t, id, raw, tree)
	if !hasDrillDesk() {
		t.Fatal("setup: the desk is not registered")
	}
	for _, owner := range []string{"local", ""} {
		e.rt.deskLogOwner = owner
		if err := e.rt.syncInstalledDesks(context.Background(), owner, e.computer); err != nil {
			t.Fatal(err)
		}
		if !hasDrillDesk() {
			t.Fatalf("a sync under owner %q disabled the installed desk", owner)
		}
	}
}
