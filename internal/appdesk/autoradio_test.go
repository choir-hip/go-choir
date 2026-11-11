package appdesk

import (
	"testing"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/modelpolicy"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// The Autoradio desk is a package desk registered like Mail (design
// docs/design/autoradio-desk-fast-turns-2026-11-11.md 3.1). Failure modes,
// written before the code:
//
//  1. the image ships no manifest, or the manifest is refused;
//  2. it has accepts, so the web prompt router changes (it must not);
//  3. a third party's package named autoradio, or the image's widened by a
//     verb, takes the reserved slug;
//  4. the desk is not in the image's required set, so a computer never has it;
//  5. its writer is not desk:autoradio, or it is not first-party;
//  6. the image ships no prompt for it;
//  7. the policy role cannot be named for the package desk (recorded by the
//     last assertion: it does not resolve as a registry profile, and the role
//     autoradio is the one that does).

func TestTheAutoradioDeskIsAFirstPartyPackageDeskWithNoAccepts(t *testing.T) {
	fp, ok := FirstPartyDesk("autoradio")
	if !ok {
		t.Fatal("(1) the image ships no autoradio manifest")
	}
	if rs := Validate(fp.Manifest); len(rs) != 0 {
		t.Fatalf("(1) refused: %+v", rs)
	}
	if len(fp.Manifest.Accepts) != 0 {
		t.Fatalf("(2) accepts %v would change the web prompt router", fp.Manifest.Accepts)
	}
	set, rs := Derive([]Event{install(1, fp.Manifest)})
	if len(rs) != 0 || len(set.Desks) != 1 {
		t.Fatalf("installing it: desks %d, refusals %+v", len(set.Desks), rs)
	}
	d := set.Desks[0]
	if d.Name != "app:autoradio" || d.Kind != deskmanifest.KindApp || !d.FirstParty || len(d.Accepts) != 0 {
		t.Fatalf("(2)(5) desk row %+v", d)
	}
	if err := Reconcile(set); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = Reconcile(Set{}) })
	if w, ok := objectgraph.DeskWriter("app:autoradio"); !ok || string(w) != "desk:autoradio" {
		t.Fatalf("(5) writer %q (%v)", w, ok)
	}
	if prompt, ok := FirstPartyPrompt("autoradio"); !ok || len(prompt) < 200 {
		t.Fatal("(6) the image ships no prompt for the autoradio desk")
	}
	// (7) Registered, the package desk IS a registry profile, so its own name
	// normalizes; the policy role the answers run under is the bare
	// "autoradio" (modelpolicy.AutoradioRole), which resolves without the
	// registry and so before the desk is registered.
	if got := modelpolicy.NormalizeRole("app:autoradio"); got != "app:autoradio" {
		t.Fatalf("(7) NormalizeRole(app:autoradio) = %q", got)
	}
	if got := modelpolicy.NormalizeRole("autoradio"); got != modelpolicy.AutoradioRole {
		t.Fatalf("(7) NormalizeRole(autoradio) = %q", got)
	}
}

func TestOnlyTheImagesManifestTakesTheAutoradioSlug(t *testing.T) {
	fp, _ := FirstPartyDesk("autoradio")
	wider := fp.Manifest
	wider.Verbs = append(append([]string{}, wider.Verbs...), "Cast")
	for name, m := range map[string]Manifest{"third party": goodManifest("autoradio"), "wider verbs": wider} {
		if rs := Validate(m); !hasCode(rs, CodeReservedName) {
			t.Errorf("(3) %s: Validate = %+v, want reserved_name", name, rs)
		}
		if set, _ := Derive([]Event{install(1, m)}); len(set.Desks) != 0 {
			t.Errorf("(3) %s registered %+v", name, set.Desks)
		}
	}
}

func TestTheImageRequiresTheAutoradioDesk(t *testing.T) {
	required, err := ImageRequired()
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range required {
		if r.Package == "autoradio" {
			if Digest(r.Manifest) != Digest(mustFirstPartyManifest(t)) {
				t.Fatal("(4) the required manifest is not the first-party one")
			}
			return
		}
	}
	t.Fatal("(4) the image's required set lacks the autoradio desk")
}

func mustFirstPartyManifest(t *testing.T) Manifest {
	t.Helper()
	fp, ok := FirstPartyDesk("autoradio")
	if !ok {
		t.Fatal("no first-party autoradio")
	}
	return fp.Manifest
}
