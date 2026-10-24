package objectgraph

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
)

// P2a (docs/design/app-packages-replan-2026-10-24.md section 2.2): the one
// exported seam through which an app package registers its data kinds.
//
// Failure modes, written before the code (CLAUDE.md "Testing"). A wrong check
// here silently admits a writer to a kind it must not write, so every mode
// names the way the check could be wrong:
//
//	F1  a kind outside choir.app.<pkg>. is registered: a package takes a
//	    built-in kind (choir.texture_revision) or another package's kind.
//	F2  the namespace check is a prefix check that another package's kind
//	    satisfies (pkg "slides" and kind choir.app.slides-x.deck), or that
//	    admits an empty or malformed kind name (dots, upper case, ':').
//	F3  the writer is anything but desk:<pkg>: empty, a built-in desk,
//	    system:runtime (admitted under every desk), owner, corpusd, another
//	    package's desk.
//	F4  a package named like a built-in desk (texture, management, ...) gets
//	    that desk's writer for its kinds.
//	F5  a kind already in DefaultRegistry is overwritten or shadowed.
//	F6  a registration smuggles a flag the gate trusts: Reserved, a memory
//	    store, a schema pointing at another package, an unknown identity mode.
//	F7  a second registration of the same kind silently changes it (OwnerEdits
//	    flipped, schema version bumped): idempotent means identical.
//	F8  a refused registration leaves a trace in the registry or the table.
//	F9  concurrent registration races or registers two different shapes.
//	F10 a registered kind is not writable by its desk and the owner (where
//	    OwnerEdits) and is writable by anyone else under enforce.
//	F11 the seam deregisters, or the table lets a caller mutate the registry
//	    through a returned slice.
//	F12 DefaultRegistry (and so the coverage tests over it) gains package kinds.

func pkgKind(pkg, kind string) KindRegistration {
	return KindRegistration{
		Kind:         ObjectKind("choir.app." + pkg + "." + kind),
		IdentityMode: IdentityExternalKey,
		Versioned:    true,
		OwnerEdits:   true,
		Schema:       "pkg:" + pkg + "@1",
		Writer:       Writer("desk:" + pkg),
	}
}

func newTestSeam() (*Registry, *packageKindTable, *Registry) {
	return NewRegistry(), newPackageKindTable(), DefaultRegistry()
}

func register(r *Registry, tbl *packageKindTable, builtin *Registry, pkg string, k KindRegistration) error {
	return registerPackageKindOn(r, tbl, builtin, pkg, k)
}

func TestRegisterPackageKindAdmitsTheDeclaredShape(t *testing.T) {
	r, tbl, builtin := newTestSeam()
	k := pkgKind("slides", "deck")
	if err := register(r, tbl, builtin, "slides", k); err != nil {
		t.Fatalf("register: %v", err)
	}
	got, err := r.LookupKind(k.Kind)
	if err != nil {
		t.Fatal(err)
	}
	if got.Writer != "desk:slides" || !got.OwnerEdits || got.IdentityMode != IdentityExternalKey || got.Store != StoreTypeDolt {
		t.Fatalf("registered shape = %+v", got)
	}
	if got.Owner != "package:slides" {
		t.Fatalf("owner = %q, want package:slides", got.Owner)
	}
	if reg, ok := tbl.lookup(k.Kind); !ok || reg.Kind != k.Kind {
		t.Fatalf("the table does not hold the kind: %+v %v", reg, ok)
	}
}

// F1, F2
func TestRegisterPackageKindRefusesAKindOutsideTheNamespace(t *testing.T) {
	cases := map[string]string{
		"a built-in kind":               "choir.texture_revision",
		"another package's kind":        "choir.app.mail.message",
		"a prefix-lookalike package":    "choir.app.slides-x.deck",
		"the namespace with no kind":    "choir.app.slides.",
		"the bare namespace":            "choir.app.slides",
		"a nested kind name":            "choir.app.slides.deck.v2",
		"an upper-case kind name":       "choir.app.slides.Deck",
		"a kind name with a colon":      "choir.app.slides.de:ck",
		"a kind name with a slash":      "choir.app.slides.de/ck",
		"a kind name with a space":      "choir.app.slides.de ck",
		"a kind name starting a digit":  "choir.app.slides.1deck",
		"an empty kind":                 "",
		"the namespace of a longer pkg": "choir.app.slides2.deck",
	}
	for name, kind := range cases {
		t.Run(name, func(t *testing.T) {
			r, tbl, builtin := newTestSeam()
			k := pkgKind("slides", "deck")
			k.Kind = ObjectKind(kind)
			err := register(r, tbl, builtin, "slides", k)
			if !errors.Is(err, ErrPackageKindRefused) {
				t.Fatalf("err = %v, want ErrPackageKindRefused", err)
			}
			if len(r.AllKinds()) != 0 || len(tbl.all()) != 0 {
				t.Fatalf("F8: a refused kind left a trace: %v %v", r.AllKinds(), tbl.all())
			}
		})
	}
}

// F3
func TestRegisterPackageKindRefusesAnyWriterButTheDeskOfThePackage(t *testing.T) {
	for _, w := range []Writer{
		"", WriterTexture, WriterManagement, WriterEngineering, WriterResearch, WriterAutoradio,
		WriterRuntime, WriterCorpusd, WriterStore, WriterOwner,
		"desk:mail", "desk:slides-x", "desk:Slides", "desk:slides ", "desk: slides",
	} {
		t.Run(string(w), func(t *testing.T) {
			r, tbl, builtin := newTestSeam()
			k := pkgKind("slides", "deck")
			k.Writer = w
			err := register(r, tbl, builtin, "slides", k)
			if !errors.Is(err, ErrPackageKindRefused) {
				t.Fatalf("writer %q: err = %v, want ErrPackageKindRefused", w, err)
			}
			if len(r.AllKinds()) != 0 {
				t.Fatalf("F8: a refused writer left a trace")
			}
		})
	}
}

// F4: desk:<pkg> for a built-in desk name is that desk.
func TestRegisterPackageKindRefusesAPackageNamedLikeABuiltinDesk(t *testing.T) {
	for _, pkg := range []string{"texture", "management", "engineering", "research", "autoradio"} {
		t.Run(pkg, func(t *testing.T) {
			r, tbl, builtin := newTestSeam()
			err := register(r, tbl, builtin, pkg, pkgKind(pkg, "thing"))
			if !errors.Is(err, ErrPackageKindRefused) {
				t.Fatalf("err = %v, want ErrPackageKindRefused", err)
			}
		})
	}
}

func TestRegisterPackageKindRefusesABadPackageName(t *testing.T) {
	long := strings.Repeat("a", 33) // desk:<name> allows 32 characters
	for _, pkg := range []string{"", "A", "1a", "a", "a_b", "a.b", "a:b", "a b", "-a", "has/slash", long} {
		t.Run(fmt.Sprintf("%q", pkg), func(t *testing.T) {
			r, tbl, builtin := newTestSeam()
			k := pkgKind("slides", "deck")
			k.Kind = ObjectKind("choir.app." + pkg + ".deck")
			k.Writer = Writer("desk:" + pkg)
			k.Schema = "pkg:" + pkg + "@1"
			if err := register(r, tbl, builtin, pkg, k); !errors.Is(err, ErrPackageKindRefused) {
				t.Fatalf("pkg %q: err = %v, want ErrPackageKindRefused", pkg, err)
			}
		})
	}
}

// F5: the namespace check alone keeps a package off a built-in kind; this is
// the belt on a registry that one day holds a choir.app.* kind.
func TestRegisterPackageKindRefusesAKindTheDefaultRegistryHolds(t *testing.T) {
	r, tbl, builtin := newTestSeam()
	k := pkgKind("slides", "deck")
	builtin.RegisterKind(KindRegistration{Kind: k.Kind, Writer: WriterTexture})
	err := register(r, tbl, builtin, "slides", k)
	if !errors.Is(err, ErrPackageKindRefused) || !strings.Contains(err.Error(), "built-in") {
		t.Fatalf("err = %v, want a built-in refusal", err)
	}
	if len(r.AllKinds()) != 0 {
		t.Fatal("F8: a refused kind left a trace")
	}
}

// F6
func TestRegisterPackageKindRefusesSmuggledFlags(t *testing.T) {
	cases := map[string]func(*KindRegistration){
		"reserved":                  func(k *KindRegistration) { k.Reserved = true },
		"a memory store":            func(k *KindRegistration) { k.Store = StoreTypeMemory },
		"a sqlite store":            func(k *KindRegistration) { k.Store = StoreTypeSQLite },
		"another package's schema":  func(k *KindRegistration) { k.Schema = "pkg:mail@1" },
		"a Go type as the schema":   func(k *KindRegistration) { k.Schema = "store.TextureRevision" },
		"no schema":                 func(k *KindRegistration) { k.Schema = "" },
		"schema version zero":       func(k *KindRegistration) { k.Schema = "pkg:slides@0" },
		"schema version not number": func(k *KindRegistration) { k.Schema = "pkg:slides@x" },
		"an unknown identity mode":  func(k *KindRegistration) { k.IdentityMode = "random" },
		"a claimed owner":           func(k *KindRegistration) { k.Owner = OwnerVMStore },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			r, tbl, builtin := newTestSeam()
			k := pkgKind("slides", "deck")
			mutate(&k)
			if err := register(r, tbl, builtin, "slides", k); !errors.Is(err, ErrPackageKindRefused) {
				t.Fatalf("err = %v, want ErrPackageKindRefused", err)
			}
			if len(r.AllKinds()) != 0 {
				t.Fatal("F8: a refused kind left a trace")
			}
		})
	}
}

func TestRegisterPackageKindAcceptsTheDefaultsItFills(t *testing.T) {
	r, tbl, builtin := newTestSeam()
	k := pkgKind("slides", "deck")
	k.IdentityMode = ""
	k.Store = ""
	k.Owner = "package:slides" // the value it would set itself
	if err := register(r, tbl, builtin, "slides", k); err != nil {
		t.Fatal(err)
	}
	got, _ := r.LookupKind(k.Kind)
	if got.IdentityMode != IdentityContentHash || got.Store != StoreTypeDolt {
		t.Fatalf("defaults = %+v", got)
	}
}

// F7
func TestRegisterPackageKindIsIdempotentOnlyForAnIdenticalShape(t *testing.T) {
	r, tbl, builtin := newTestSeam()
	k := pkgKind("slides", "deck")
	for i := 0; i < 3; i++ {
		if err := register(r, tbl, builtin, "slides", k); err != nil {
			t.Fatalf("identical re-registration %d: %v", i, err)
		}
	}
	if n := len(r.AllKinds()); n != 1 {
		t.Fatalf("kinds = %d, want 1", n)
	}
	changed := map[string]func(*KindRegistration){
		"owner edits flipped":   func(k *KindRegistration) { k.OwnerEdits = false },
		"schema version bumped": func(k *KindRegistration) { k.Schema = "pkg:slides@2" },
		"identity mode changed": func(k *KindRegistration) { k.IdentityMode = IdentityContentHash },
		"versioned flipped":     func(k *KindRegistration) { k.Versioned = false },
	}
	for name, mutate := range changed {
		t.Run(name, func(t *testing.T) {
			again := k
			mutate(&again)
			if err := register(r, tbl, builtin, "slides", again); !errors.Is(err, ErrPackageKindConflict) {
				t.Fatalf("err = %v, want ErrPackageKindConflict", err)
			}
			got, _ := r.LookupKind(k.Kind)
			if got.OwnerEdits != k.OwnerEdits || got.Schema != k.Schema || got.Versioned != k.Versioned || got.IdentityMode != k.IdentityMode {
				t.Fatalf("a refused change altered the registration: %+v", got)
			}
		})
	}
}

// F9
func TestRegisterPackageKindConcurrentRegistration(t *testing.T) {
	r, tbl, builtin := newTestSeam()
	same := pkgKind("slides", "deck")
	diff := same
	diff.OwnerEdits = false
	var wg sync.WaitGroup
	errs := make(chan error, 64)
	for i := 0; i < 32; i++ {
		wg.Add(2)
		go func() { defer wg.Done(); errs <- register(r, tbl, builtin, "slides", same) }()
		go func() { defer wg.Done(); errs <- register(r, tbl, builtin, "slides", diff) }()
	}
	wg.Wait()
	close(errs)
	ok, conflict := 0, 0
	for err := range errs {
		switch {
		case err == nil:
			ok++
		case errors.Is(err, ErrPackageKindConflict):
			conflict++
		default:
			t.Fatalf("unexpected error: %v", err)
		}
	}
	got, _ := r.LookupKind(same.Kind)
	stored, _ := tbl.lookup(same.Kind)
	if stored != got {
		t.Fatalf("registry and table disagree: %+v vs %+v", got, stored)
	}
	// Exactly one shape won: its 32 registrations pass, the other 32 conflict.
	if ok != 32 || conflict != 32 {
		t.Fatalf("ok=%d conflict=%d, want 32 and 32", ok, conflict)
	}
}

// F10: the gate admits the desk, and the owner only where the kind admits it.
func TestPackageKindGateAdmitsTheDeskAndOwnerEditsOnly(t *testing.T) {
	writerMode(t, WriterGateEnforce)
	pkg := "p2atest"
	withEdits, noEdits := pkgKind(pkg, "note"), pkgKind(pkg, "ledger")
	noEdits.OwnerEdits = false
	for _, k := range []KindRegistration{withEdits, noEdits} {
		if err := RegisterPackageKind(pkg, k); err != nil {
			t.Fatal(err)
		}
	}
	cases := []struct {
		name     string
		declared Writer
		kind     ObjectKind
		wantErr  error
	}{
		{"the package's desk writes its kind", Writer("desk:" + pkg), withEdits.Kind, nil},
		{"the owner writes a kind that admits owner edits", WriterOwner, withEdits.Kind, nil},
		{"the owner cannot write a kind that does not", WriterOwner, noEdits.Kind, ErrWrongWriter},
		{"texture cannot write a package kind", WriterTexture, withEdits.Kind, ErrWrongWriter},
		{"research cannot write a package kind", WriterResearch, withEdits.Kind, ErrWrongWriter},
		{"another package's desk cannot", "desk:mail", withEdits.Kind, ErrWrongWriter},
		{"corpusd cannot", WriterCorpusd, withEdits.Kind, ErrWrongWriter},
	}
	for _, tc := range cases {
		err := CheckWriteAs(WithWriter(context.Background(), tc.declared), []Object{gateObject(t, tc.kind, tc.name)}, nil)
		if !errors.Is(err, tc.wantErr) {
			t.Errorf("%s: err = %v, want %v", tc.name, err, tc.wantErr)
		}
	}
	if err := CheckWriteAs(context.Background(), []Object{gateObject(t, withEdits.Kind, "undeclared")}, nil); !errors.Is(err, ErrUndeclaredWriter) {
		t.Errorf("undeclared write of a package kind: err = %v, want ErrUndeclaredWriter", err)
	}
	// The package's own namespace is not a way onto a built-in kind.
	if err := RegisterPackageKind(pkg, KindRegistration{Kind: "choir.texture_revision", Writer: Writer("desk:" + pkg), Schema: "pkg:" + pkg + "@1"}); !errors.Is(err, ErrPackageKindRefused) {
		t.Errorf("built-in kind through the seam: err = %v", err)
	}
}

// The Service resolves a package kind (it holds its own registry, so the
// global table is its fallback): create, get and list work, and a kind that was
// never registered still does not.
func TestServiceResolvesARegisteredPackageKind(t *testing.T) {
	pkg := "p2aservice"
	k := pkgKind(pkg, "note")
	if err := RegisterPackageKind(pkg, k); err != nil {
		t.Fatal(err)
	}
	svc := NewService(Config{})
	ctx := context.Background()
	obj, err := svc.CreateObject(ctx, CreateObjectRequest{Kind: k.Kind, OwnerID: "user:p2a", IdentityKey: "n1", Body: []byte(`{"a":1}`)})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if obj.ObjectKind != k.Kind {
		t.Fatalf("kind = %s", obj.ObjectKind)
	}
	if _, err := svc.GetObject(ctx, obj.CanonicalID); err != nil {
		t.Fatalf("get: %v", err)
	}
	got, err := svc.ListObjects(ctx, ListFilter{Kind: k.Kind})
	if err != nil || len(got) != 1 {
		t.Fatalf("list = %v, %v", got, err)
	}
	if _, err := svc.CreateObject(ctx, CreateObjectRequest{Kind: "choir.app.p2aservice.never", OwnerID: "user:p2a", IdentityKey: "x"}); err == nil {
		t.Fatal("an unregistered package kind was created")
	}
}

// F11
func TestPackageKindTableReturnsCopies(t *testing.T) {
	r, tbl, builtin := newTestSeam()
	k := pkgKind("slides", "deck")
	if err := register(r, tbl, builtin, "slides", k); err != nil {
		t.Fatal(err)
	}
	list := tbl.ofPackage("slides")
	if len(list) != 1 {
		t.Fatalf("list = %v", list)
	}
	list[0].OwnerEdits = false
	list[0].Writer = WriterTexture
	again, _ := tbl.lookup(k.Kind)
	if !again.OwnerEdits || again.Writer != "desk:slides" {
		t.Fatalf("a returned slice mutated the table: %+v", again)
	}
	if len(tbl.ofPackage("mail")) != 0 {
		t.Fatal("another package's list is not empty")
	}
}

// F12
func TestDefaultRegistryHoldsNoPackageKinds(t *testing.T) {
	if err := RegisterPackageKind("p2adefault", pkgKind("p2adefault", "note")); err != nil {
		t.Fatal(err)
	}
	for _, reg := range DefaultRegistry().AllKinds() {
		if strings.HasPrefix(string(reg.Kind), packageKindNamespace) {
			t.Fatalf("DefaultRegistry holds package kind %s", reg.Kind)
		}
	}
}
