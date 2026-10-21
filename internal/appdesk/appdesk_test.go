package appdesk

import (
	"testing"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
)

// The failure modes of docs/design/app-package-carries-app-desk-2026-10-21.md
// §5, written before the code. Numbers in test comments are that list's.

func goodManifest(pkg string) Manifest {
	return Manifest{
		Package:        pkg,
		Verbs:          []string{"Cast", "Note", "ReadCapture", "Done"},
		WorkerMaxBytes: 256 << 20,
		OwnedKinds:     []string{"choir.app." + pkg + ".episode_note"},
	}
}

func install(seq uint64, m Manifest) Event {
	return Event{Seq: seq, Action: ActionInstall, Package: m.Package, ManifestDigest: Digest(m), Manifest: &m}
}

func disable(seq uint64, pkg string) Event {
	return Event{Seq: seq, Action: ActionDisable, Package: pkg}
}

func hasCode(rs []Refusal, code Code) bool {
	for _, r := range rs {
		if r.Code == code {
			return true
		}
	}
	return false
}

func TestValidateAcceptsAMinimalAppDesk(t *testing.T) {
	if rs := Validate(goodManifest("reader")); len(rs) != 0 {
		t.Fatalf("a valid manifest was refused: %+v", rs)
	}
}

// 1. A package takes a system desk's name or a built-in kind.
// 2. The package grants itself authority.
func TestValidateRefusesWhatAPackageMayNotTake(t *testing.T) {
	cases := map[string]struct {
		mutate func(*Manifest)
		code   Code
	}{
		"system desk name":      {func(m *Manifest) { m.Package = "texture" }, CodeReservedName},
		"management":            {func(m *Manifest) { m.Package = "management" }, CodeReservedName},
		"static app name":       {func(m *Manifest) { m.Package = "podcast" }, CodeReservedName},
		"shell name":            {func(m *Manifest) { m.Package = "shell" }, CodeReservedName},
		"bad grammar":           {func(m *Manifest) { m.Package = "Reader App" }, CodeBadName},
		"one letter":            {func(m *Manifest) { m.Package = "r" }, CodeBadName},
		"empty":                 {func(m *Manifest) { m.Package = "" }, CodeBadName},
		"built-in kind":         {func(m *Manifest) { m.OwnedKinds = []string{"choir.texture_revision"} }, CodeKindNotNamespaced},
		"another package kind":  {func(m *Manifest) { m.OwnedKinds = []string{"choir.app.other.note"} }, CodeKindNotNamespaced},
		"prefix only":           {func(m *Manifest) { m.OwnedKinds = []string{"choir.app.reader."} }, CodeKindNotNamespaced},
		"kind with a dot":       {func(m *Manifest) { m.OwnedKinds = []string{"choir.app.reader.a.b"} }, CodeKindNotNamespaced},
		"duplicate kind":        {func(m *Manifest) { m.OwnedKinds = append(m.OwnedKinds, m.OwnedKinds[0]) }, CodeKindNotNamespaced},
		"exec":                  {func(m *Manifest) { m.Verbs = append(m.Verbs, "Exec") }, CodeVerbNotApp},
		"product api":           {func(m *Manifest) { m.Verbs = append(m.Verbs, "ProductAPI") }, CodeVerbNotApp},
		"freeze":                {func(m *Manifest) { m.Verbs = append(m.Verbs, "Freeze") }, CodeVerbNotApp},
		"world read":            {func(m *Manifest) { m.Verbs = append(m.Verbs, "Read") }, CodeVerbNotApp},
		"privileged taint":      {func(m *Manifest) { m.Taint = deskmanifest.TaintPrivileged }, CodeTaintBelowFloor},
		"unknown taint":         {func(m *Manifest) { m.Taint = "trusted" }, CodeTaintBelowFloor},
		"no memory":             {func(m *Manifest) { m.WorkerMaxBytes = 0 }, CodeMemoryOverCap},
		"memory over the cap":   {func(m *Manifest) { m.WorkerMaxBytes = 4 << 30 }, CodeMemoryOverCap},
		"egress with no grants": {func(m *Manifest) { m.Egress = []string{"https://example.com"} }, CodeEgressNotGranted},
	}
	for name, c := range cases {
		m := goodManifest("reader")
		c.mutate(&m)
		rs := Validate(m)
		if !hasCode(rs, c.code) {
			t.Errorf("%s: want refusal %s, got %+v", name, c.code, rs)
		}
	}
}

func TestDigestBindsEveryField(t *testing.T) {
	base := goodManifest("reader")
	d := Digest(base)
	if len(d) != len("sha256:")+64 || d[:7] != "sha256:" {
		t.Fatalf("digest %q is not sha256:<hex>", d)
	}
	if Digest(goodManifest("reader")) != d {
		t.Fatal("digest is not deterministic")
	}
	for name, mutate := range map[string]func(*Manifest){
		"verb":   func(m *Manifest) { m.Verbs = append(m.Verbs, "Ask") },
		"kind":   func(m *Manifest) { m.OwnedKinds = append(m.OwnedKinds, "choir.app.reader.x") },
		"memory": func(m *Manifest) { m.WorkerMaxBytes++ },
		"taint":  func(m *Manifest) { m.Taint = deskmanifest.TaintUntrustedInput },
		"egress": func(m *Manifest) { m.Egress = []string{"x"} },
	} {
		m := goodManifest("reader")
		mutate(&m)
		if Digest(m) == d {
			t.Errorf("changing %s left the digest unchanged", name)
		}
	}
}

func TestDeriveInstallsADesk(t *testing.T) {
	set, rs := Derive([]Event{install(1, goodManifest("reader"))})
	if len(rs) != 0 {
		t.Fatalf("refusals: %+v", rs)
	}
	if len(set.Desks) != 1 {
		t.Fatalf("want one desk, got %d", len(set.Desks))
	}
	d := set.Desks[0]
	if d.Name != "app:reader" || d.Kind != deskmanifest.KindApp || !PackageOrigin(d) {
		t.Fatalf("desk row %+v", d)
	}
	// The package trust floor: untrusted taint, whatever the manifest left empty.
	if d.Taint != deskmanifest.TaintUntrustedInput {
		t.Errorf("package desk taint %q, want the untrusted floor", d.Taint)
	}
	w, live, ok := set.Writer("choir.app.reader.episode_note")
	if !ok || !live || w != "desk:reader" {
		t.Errorf("kind writer %q live=%v ok=%v", w, live, ok)
	}
	if _, _, ok := set.Writer("choir.texture_revision"); ok {
		t.Error("the derived set claims a built-in kind")
	}
	if WriterFor("reader") != "desk:reader" || DeskName("reader") != "app:reader" {
		t.Error("writer and desk naming")
	}
}

// 3. What runs is not what was approved.
func TestDeriveRefusesADigestMismatch(t *testing.T) {
	ev := install(1, goodManifest("reader"))
	mutated := *ev.Manifest
	mutated.Verbs = append(mutated.Verbs, "Ask")
	ev.Manifest = &mutated
	set, rs := Derive([]Event{ev})
	if len(set.Desks) != 0 || !hasCode(rs, CodeDigestMismatch) {
		t.Fatalf("a mutated manifest under an unchanged digest was derived: desks=%d refusals=%+v", len(set.Desks), rs)
	}
	ev.Manifest = nil
	if set, rs := Derive([]Event{ev}); len(set.Desks) != 0 || !hasCode(rs, CodeDigestMismatch) {
		t.Fatalf("an install with no manifest was derived: %+v", rs)
	}
	bad := goodManifest("reader")
	bad.Verbs = append(bad.Verbs, "Exec")
	if set, rs := Derive([]Event{install(1, bad)}); len(set.Desks) != 0 || !hasCode(rs, CodeVerbNotApp) {
		t.Fatalf("an invalid manifest with a matching digest was derived: %+v", rs)
	}
}

// 4. A second authority for the desk set: Derive is a pure fold, so the same
// events give the same set and a prefix (a restore to a pre-install head)
// has no such desk.
func TestDeriveIsAPureFold(t *testing.T) {
	events := []Event{install(1, goodManifest("reader")), install(2, goodManifest("notes")), disable(3, "reader")}
	a, _ := Derive(events)
	b, _ := Derive(events)
	if len(a.Desks) != len(b.Desks) || a.Desks[0].Name != b.Desks[0].Name || a.Desks[0].Sandbox.UID != b.Desks[0].Sandbox.UID {
		t.Fatal("Derive is not deterministic")
	}
	pre, _ := Derive(events[:0])
	if len(pre.Desks) != 0 {
		t.Fatal("an empty tape derived a desk")
	}
	// Events out of order on input are folded by seq.
	rev := []Event{events[2], events[1], events[0]}
	c, rs := Derive(rev)
	if len(rs) != 0 || len(c.Desks) != 1 || c.Desks[0].Name != "app:notes" {
		t.Fatalf("fold is not by seq: desks=%+v refusals=%+v", c.Desks, rs)
	}
	// A duplicate seq is a broken chain: the second is refused.
	if _, rs := Derive([]Event{install(1, goodManifest("reader")), install(1, goodManifest("notes"))}); !hasCode(rs, CodeBadSequence) {
		t.Fatalf("a duplicate seq was not refused: %+v", rs)
	}
}

// 5. A retired desk writes.
func TestDisableRetiresTheDeskAndItsKinds(t *testing.T) {
	set, rs := Derive([]Event{install(1, goodManifest("reader")), disable(2, "reader")})
	if len(rs) != 0 {
		t.Fatalf("refusals: %+v", rs)
	}
	if len(set.Desks) != 0 {
		t.Fatal("a disabled desk is still live")
	}
	w, live, ok := set.Writer("choir.app.reader.episode_note")
	if !ok || live || w != "desk:reader" {
		t.Fatalf("retired kind: writer %q live=%v ok=%v; want known, not live", w, live, ok)
	}
	// A retired name is not reinstalled in this slice (upgrade and reinstall
	// need the re-approval path, design §5.10).
	_, rs = Derive([]Event{install(1, goodManifest("reader")), disable(2, "reader"), install(3, goodManifest("reader"))})
	if !hasCode(rs, CodeNameRetired) {
		t.Fatalf("a retired name was reinstalled: %+v", rs)
	}
	if _, rs := Derive([]Event{disable(1, "reader")}); !hasCode(rs, CodeNotInstalled) {
		t.Fatalf("disable of a package never installed: %+v", rs)
	}
}

// 6. Two packages claim one name or one kind: the earlier event wins.
func TestSecondClaimIsRefused(t *testing.T) {
	first := goodManifest("reader")
	second := goodManifest("reader")
	second.Verbs = []string{"Note"}
	set, rs := Derive([]Event{install(1, first), install(2, second)})
	if len(set.Desks) != 1 || !set.Desks[0].HasVerb("Cast") || !hasCode(rs, CodeNameTaken) {
		t.Fatalf("second install of a name: desks=%+v refusals=%+v", set.Desks, rs)
	}
	// One kind can only be claimed under its own package's namespace, so a
	// kind collision needs two installs of one name; Validate already refuses
	// a foreign namespace. Pin both.
	thief := goodManifest("notes")
	thief.OwnedKinds = []string{"choir.app.reader.episode_note"}
	if _, rs := Derive([]Event{install(1, first), install(2, thief)}); !hasCode(rs, CodeKindNotNamespaced) {
		t.Fatalf("a package claimed another's kind: %+v", rs)
	}
}

// 9. Taint crosses a worker: every package desk has its own uid, outside the
// system range, so its worker class is never a privileged desk's.
func TestPackageDeskWorkerClassIsItsOwn(t *testing.T) {
	set, _ := Derive([]Event{install(1, goodManifest("reader")), install(2, goodManifest("notes"))})
	seen := map[uint32]string{}
	for _, name := range deskmanifest.Names() {
		d, _ := deskmanifest.Lookup(name)
		if d.Kind != deskmanifest.KindApp && d.Sandbox.UID != 0 {
			seen[d.Sandbox.UID] = name
		}
	}
	for _, d := range set.Desks {
		if d.Sandbox.UID < 62000 {
			t.Errorf("%s uid %d is in the system range", d.Name, d.Sandbox.UID)
		}
		if other, dup := seen[d.Sandbox.UID]; dup {
			t.Errorf("%s shares uid %d with %s", d.Name, d.Sandbox.UID, other)
		}
		seen[d.Sandbox.UID] = d.Name
	}
}

func TestActivateIsOffByDefault(t *testing.T) {
	set, _ := Derive([]Event{install(1, goodManifest("offdefault"))})
	if err := Activate(ModeFromEnv(func(string) string { return "" }), set); err != nil {
		t.Fatal(err)
	}
	if _, ok := deskmanifest.Lookup("app:offdefault"); ok {
		t.Fatal("Activate registered a desk with CHOIR_APP_DESKS unset")
	}
	if ModeFromEnv(func(string) string { return "yes" }) != ModeOff {
		t.Fatal("an unknown mode must read as off")
	}
}

func TestActivateRegistersAndRetires(t *testing.T) {
	on := ModeFromEnv(func(k string) string {
		if k == EnvMode {
			return "on"
		}
		return ""
	})
	if on != ModeOn {
		t.Fatalf("mode %q", on)
	}
	events := []Event{install(1, goodManifest("activated"))}
	set, _ := Derive(events)
	if err := Activate(on, set); err != nil {
		t.Fatal(err)
	}
	if !deskmanifest.FrameAllowed("app:activated", "read_capture") {
		t.Fatal("an activated app desk is not served its frame")
	}
	// Activating the same set again is a no-op (every install event re-derives).
	if err := Activate(on, set); err != nil {
		t.Fatalf("re-activation of the same set: %v", err)
	}
	gone, _ := Derive(append(events, disable(2, "activated")))
	if err := Activate(on, gone); err != nil {
		t.Fatal(err)
	}
	if _, ok := deskmanifest.Lookup("app:activated"); ok {
		t.Fatal("a disabled app desk is still registered")
	}
	if deskmanifest.FrameAllowed("app:activated", "read_capture") {
		t.Fatal("a retired app desk is still served")
	}
}
