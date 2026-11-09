package agentcore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/apppackage"
	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/pkgrelease"
	"github.com/yusefmosiah/go-choir/internal/pkgruntime"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// Another publisher's desk installs only on a computer whose worker sandbox is
// ready (R24, Phase 6 acceptance), and then runs as its own release says.
//
// Failure modes, written before the code:
//
//	T1  the gate says admitted but nothing registers: the third-party desk is
//	    not in the desk set after the install;
//	T2  the registered row is wider than the ceiling the review judged
//	    (ReadMail, a lease grant, first-party), or is first-party (so it would
//	    run in a first-party sandbox class);
//	T3  the desk wakes on an image prompt, or on none, instead of its release's
//	    prompt;
//	T4  a not-ready computer installs it, registers it, or wakes it;
//	T5  a first-party or built-in name is taken by another publisher's desk;
//	T6  a sandbox that stops being ready leaves the desk registered.
const (
	thirdPartyPkg    = "notes-desk"
	thirdPartyPrompt = "You keep the owner's notes tidy. Use only your own records.\n"
	thirdPartyManif  = `{
  "schema": "choir.package/v2",
  "name": "notes-desk",
  "version": "1.0.0",
  "display": {"title": "Notes desk", "icon": "N", "description": "A desk from another publisher"},
  "kinds": [], "uses_kinds": [],
  "contributes": {"desks": [{"name": "notes-desk", "prompt": "prompt.md"}]},
  "capabilities": [{"grant": "desk.act"}]
}
`
)

// thirdPartyBuilder is the fake builder that, like the real one, keeps the
// desk's prompt as a data file of the module.
type thirdPartyBuilder struct{ fakeModuleBuilder }

func (b *thirdPartyBuilder) Build(ctx context.Context, rec pkgrelease.Record, treePatch []byte) (pkgruntime.Module, error) {
	m, err := b.fakeModuleBuilder.Build(ctx, rec, treePatch)
	sum := sha256.Sum256([]byte(thirdPartyPrompt))
	m.Manifest = json.RawMessage(thirdPartyManif)
	m.Files = []pkgruntime.File{{Path: "prompt.md", SHA256: hex.EncodeToString(sum[:]), Bytes: []byte(thirdPartyPrompt)}}
	return m, err
}

func readySandbox(t *testing.T) {
	t.Helper()
	apppackage.SetPlatformSource(GuestPlatform)
	t.Cleanup(func() { apppackage.SetPlatformSource(nil); yaegikernel.ResetDeskSandboxState() })
	yaegikernel.RecordDeskSandboxSelfTest(yaegikernel.DeskCellSelfTestReceipt{
		Mode:    string(yaegikernel.CellConfinementRequired),
		Workers: []yaegikernel.DeskCellSelfTestWorker{{Role: "research", Profile: "gvisor"}},
		Gvisor:  &yaegikernel.GvisorStatus{Available: true},
		OK:      true,
	}, "boot")
}

func TestThirdPartyDeskInstallsOnAReadySandboxAndWakesOnItsOwnPrompt(t *testing.T) {
	e := newDrillEnv(t)
	e.rt.packageBuilder = &thirdPartyBuilder{}
	e.files = map[string]string{"choir-package.json": thirdPartyManif, "prompt.md": thirdPartyPrompt}
	readySandbox(t)
	ctx := context.Background()

	publish := func(track string) (string, json.RawMessage, []byte) {
		_, id, raw, tree := publishedPackageNamed(t, thirdPartyPkg, e.repo, e.base, track, e.files)
		return id, raw, tree
	}
	id, raw, tree := publish("owner-publisher")

	// T4: a computer whose sandbox is not ready refuses the install and
	// registers nothing.
	yaegikernel.ResetDeskSandboxState()
	// (The record's own validation refuses first and the install route names
	// any record refusal release_digest_mismatch; the desk gate itself is
	// refuseDeskInstall, below.)
	if code, _, reason := e.post(t, installBody(t, id, raw, tree, "")); code != http.StatusUnprocessableEntity {
		t.Fatalf("T4 not ready = %d %s", code, reason)
	}
	if _, ok := deskmanifest.Lookup("app:" + thirdPartyPkg); ok {
		t.Fatal("T4: a not-ready computer registered the desk")
	}

	// T1, T2: ready, the install (with management's review, as for any desk)
	// registers the release's own row.
	readySandbox(t)
	e.install(t, id, raw, tree)
	d, ok := deskmanifest.Lookup("app:" + thirdPartyPkg)
	if !ok {
		t.Fatal("T1: the installed third-party desk is not in the desk set")
	}
	if d.FirstParty || d.HasVerb("ReadMail") || len(d.LeaseGrants) != 0 || !d.HasVerb("Cast") || !d.HasVerb("Done") {
		t.Fatalf("T2: the row is not capped by the manifest's ceiling: first_party=%t verbs=%v lease=%v", d.FirstParty, d.Verbs, d.LeaseGrants)
	}
	if cls, ok := yaegikernel.DeskTrustClassOf(d.Name); !ok || cls != yaegikernel.TrustClassPublisher {
		t.Fatalf("T2: trust class = %q %v, want the non-first-party class", cls, ok)
	}

	// T3: the wake runs the desk on its release's prompt.
	inbox := d.Name + ":" + e.owner
	if got, ok := e.rt.appDeskPrompt(d); !ok || got != thirdPartyPrompt {
		t.Fatalf("T3: prompt = %q %v", got, ok)
	}
	if _, err := e.rt.DeliverOwnerRequest(ctx, e.owner, "run-third-1", d.Name, "tidy my notes", nil, ""); err != nil {
		t.Fatal(err)
	}
	run, err := e.rt.WakeRecordsInbox(ctx, e.owner, e.computer, inbox)
	if err != nil || run == nil || run.AgentProfile != d.Name || run.AgentID != inbox {
		t.Fatalf("T3: the wake started %+v %v", run, err)
	}

	// T6: the sandbox stops being ready (a failing selftest): the next sync
	// retires the desk, and a wake refuses meanwhile.
	yaegikernel.RecordDeskSandboxSelfTest(yaegikernel.DeskCellSelfTestReceipt{Mode: string(yaegikernel.CellConfinementRequired)}, "api")
	if _, ok := e.rt.appInboxDesk(e.owner, inbox); ok {
		t.Fatal("T6: a not-ready computer would wake the desk")
	}
	e.rt.syncDesksAfterSandboxChange(ctx)
	if _, ok := deskmanifest.Lookup("app:" + thirdPartyPkg); ok {
		t.Fatal("T6: the desk is still registered on a computer that is no longer ready")
	}
	// ... and it comes back when the sandbox is ready again.
	readySandbox(t)
	e.rt.syncDesksAfterSandboxChange(ctx)
	if d, ok := deskmanifest.Lookup("app:" + thirdPartyPkg); !ok || d.FirstParty {
		t.Fatal("T6: the desk did not return when the sandbox was ready again")
	}
}

// T5: another publisher's desk never takes a built-in or first-party name.
func TestThirdPartyDeskNeverTakesAReservedName(t *testing.T) {
	readySandbox(t)
	for _, name := range []string{"texture", "management", "research", "autopaper"} {
		rec := pkgrelease.Record{Package: pkgrelease.Package{Name: name}, Track: "owner-x", Manifest: pkgrelease.Manifest{DesksCount: 1}}
		if refusal := refuseDeskInstall("owner-1", rec); refusal == nil || refusal.reason != pkgrelease.ReasonDeskNeedsSandbox {
			t.Errorf("%s: %+v", name, refusal)
		}
	}
	// A first-party slug stays the owner's own track's, with the image's row.
	rec := pkgrelease.Record{Package: pkgrelease.Package{Name: "mail"}, Track: "owner-x", Manifest: pkgrelease.Manifest{DesksCount: 1}}
	if refusal := refuseDeskInstall("owner-1", rec); refusal == nil || refusal.reason != "desk_track_not_yours" {
		t.Errorf("mail from a stranger: %+v", refusal)
	}
	// Two desks in one release are refused.
	two := pkgrelease.Record{Package: pkgrelease.Package{Name: "two-desks"}, Track: "owner-x", Manifest: pkgrelease.Manifest{DesksCount: 2}}
	if refusal := refuseDeskInstall("owner-1", two); refusal == nil {
		t.Error("two desks in one release were admitted")
	}
}
