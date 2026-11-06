package appdesk

import (
	"testing"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// The Mail desk is the package desk of the first-party Mail package (plan
// MA4.7, doctrine I26). Failure modes, written before the code:
//
//  1. a system desk named mail exists beside the package desk (two Mail
//     desks), or the old email row survives;
//  2. a third-party package named mail registers a desk under the reserved
//     slug, or the image's package with any other manifest does;
//  3. the first-party manifest is refused, so the Mail desk cannot exist;
//  4. the Mail desk's writer is not the package writer desk:mail;
//  5. the mail package's kinds are refused because mail is a desk name;
//  6. FirstParty admits a slug that is not a reserved first-party slug.

func TestTheMailDeskIsTheFirstPartyMailPackagesDesk(t *testing.T) {
	// (1) No system desk named mail or email.
	for _, n := range []string{"mail", "email"} {
		if _, ok := deskmanifest.Lookup(n); ok {
			t.Fatalf("a system desk %q is registered", n)
		}
	}
	fp, ok := FirstPartyDesk("mail")
	if !ok {
		t.Fatal("the image ships no Mail desk manifest")
	}
	if rs := Validate(fp.Manifest); len(rs) != 0 {
		t.Fatalf("(3) the first-party Mail desk manifest is refused: %+v", rs)
	}
	set, rs := Derive([]Event{install(1, fp.Manifest)})
	if len(rs) != 0 || len(set.Desks) != 1 {
		t.Fatalf("(3) installing the first-party Mail package: desks %d, refusals %+v", len(set.Desks), rs)
	}
	d := set.Desks[0]
	if d.Name != deskmanifest.Mail || d.Kind != deskmanifest.KindApp || !d.FirstParty {
		t.Fatalf("the Mail desk row: %+v", d)
	}
	// (4) Its writer is the package writer.
	if err := Reconcile(set); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = Reconcile(Set{}) })
	if w, ok := objectgraph.DeskWriter(deskmanifest.Mail); !ok || string(w) != WriterFor("mail") {
		t.Fatalf("(4) the Mail desk's writer is %q (%v), want %s", w, ok, WriterFor("mail"))
	}
	// (5) The mail package may own kinds.
	if !objectgraph.ValidPackageName("mail") {
		t.Fatal("(5) mail is refused as a package name for kinds")
	}
}

func TestOnlyTheImagesManifestTakesTheMailSlug(t *testing.T) {
	fp, _ := FirstPartyDesk("mail")
	// (2) A third party's manifest under the name, or the image's widened by
	// one verb or one lease grant, is refused.
	other := goodManifest("mail")
	wider := fp.Manifest
	wider.Verbs = append(append([]string{}, wider.Verbs...), "Cast")
	ceiling := fp.Manifest
	ceiling.LeaseGrants = []string{"mail.read"}
	for name, m := range map[string]Manifest{"third party": other, "wider verbs": wider, "lease ceiling": ceiling} {
		if rs := Validate(m); !hasCode(rs, CodeReservedName) {
			t.Errorf("(2) %s: Validate = %+v, want reserved_name", name, rs)
		}
		if set, _ := Derive([]Event{install(1, m)}); len(set.Desks) != 0 {
			t.Errorf("(2) %s registered %+v", name, set.Desks)
		}
	}
	// (6) FirstParty never admits an ordinary or another reserved slug.
	for _, name := range []string{"app:notes", "app:texture", "app:calendar"} {
		if _, err := deskmanifest.NewAppDesk(deskmanifest.AppDeskSpec{Name: name, Taint: deskmanifest.TaintUntrustedInput,
			UID: 62990, WorkerMaxBytes: 1 << 20, FirstParty: true}); err == nil {
			t.Errorf("(6) FirstParty admitted %s", name)
		}
	}
	if _, err := deskmanifest.NewAppDesk(deskmanifest.AppDeskSpec{Name: deskmanifest.Mail, Taint: deskmanifest.TaintUntrustedInput,
		UID: 62990, WorkerMaxBytes: 1 << 20}); err == nil {
		t.Error("(2) app:mail registered without the first-party mark")
	}
}
