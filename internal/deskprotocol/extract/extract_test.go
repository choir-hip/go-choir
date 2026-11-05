package extract

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/deskprotocol"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, _ := os.Getwd()
	for d := dir; d != filepath.Dir(d); d = filepath.Dir(d) {
		if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
			return d
		}
	}
	t.Fatal("no go.mod above " + dir)
	return ""
}

// F4: two extractions are byte-equal and carry no machine path.
func TestExtractionIsDeterministicAndPathFree(t *testing.T) {
	root := repoRoot(t)
	a, err := Surface(root)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Surface(root)
	if err != nil {
		t.Fatal(err)
	}
	ab, _ := a.Marshal()
	bb, _ := b.Marshal()
	if !bytes.Equal(ab, bb) {
		t.Fatal("two extractions differ")
	}
	if strings.Contains(string(ab), root) || strings.Contains(string(ab), "/Users/") || strings.Contains(string(ab), "/home/") {
		t.Fatal("the surface carries a machine path")
	}
}

// F1: the surface clears the floors, and every root is present.
func TestExtractionClearsFloorsAndCoversEveryRoot(t *testing.T) {
	s, err := Surface(repoRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Names) < deskprotocol.MinNames || len(s.Verbs) < deskprotocol.MinVerbs || len(s.Desks) < deskprotocol.MinDesks {
		t.Fatalf("below the floors: %d names, %d verbs, %d desks", len(s.Names), len(s.Verbs), len(s.Desks))
	}
	for _, r := range deskprotocol.Roots {
		if _, ok := s.Names[r.Name]; !ok {
			t.Errorf("root %s is not in the surface", r.Name)
		}
	}
	for _, want := range []string{"Cast", "Ask", "Report", "Delegate", "Precommit", "Pack"} {
		if v, ok := s.Verbs[want]; !ok || v.Sig == nil {
			t.Errorf("verb %s has no locked signature", want)
		}
	}
	// PSA coverage and the closed rule-kind enum.
	k := s.Names["types.PrecommitRuleKind"]
	if k.Kind != "enum" || k.Open || len(k.Values) < 10 {
		t.Errorf("PrecommitRuleKind must be a closed enum of the rule kinds, got %+v", k)
	}
	if s.Names["release.Precommitment"].Kind != "struct" {
		t.Error("release.Precommitment is not locked")
	}
}

// F8: a nonexistent root is an error, never a skip.
func TestUnresolvedRootIsAnError(t *testing.T) {
	saved := deskprotocol.Roots
	defer func() { deskprotocol.Roots = saved }()
	deskprotocol.Roots = append(append([]deskprotocol.Root{}, saved...), deskprotocol.Root{Name: "types.NoSuchThing", Direction: deskprotocol.Both})
	if _, err := Surface(repoRoot(t)); err == nil || !strings.Contains(err.Error(), "NoSuchThing") {
		t.Fatalf("want a root error, got %v", err)
	}
}

// The current source equals the lock for Version, and the lock set is sound.
func TestSurfaceMatchesLock(t *testing.T) {
	root := repoRoot(t)
	cur, err := Surface(root)
	if err != nil {
		t.Fatal(err)
	}
	locks, err := deskprotocol.LoadLocks(filepath.Join(root, filepath.FromSlash(deskprotocol.LockDir)))
	if err != nil {
		t.Fatal(err)
	}
	if p := deskprotocol.CheckLockSet(locks); len(p) > 0 {
		t.Fatal(strings.Join(p, "\n"))
	}
	for _, l := range locks {
		if l.Version == deskprotocol.Version {
			if !l.Equal(cur) {
				t.Fatalf("source differs from lock %s:\n%s", l.Version, deskprotocol.FormatChanges(deskprotocol.Classify(l, cur)))
			}
			return
		}
	}
	t.Fatalf("no lock for Version %s", deskprotocol.Version)
}

// B3: the lock's desk row carries exactly the field set deskmanifest.Desk.Equal
// compares. Every field of Desk is perturbed in turn; if Equal sees the change
// (it is authority by the manifest's own definition), the row must change too.
// Name is exempt: it is the map key, so a rename is a desk leaving and another
// arriving. A new Desk field of an unhandled kind fails the test instead of
// slipping through.
func TestDeskRowCarriesEveryFieldDeskEqualCompares(t *testing.T) {
	base, ok := deskmanifest.Lookup(deskmanifest.Engineering)
	if !ok {
		t.Fatal("no engineering desk")
	}
	var perturb func(v reflect.Value) bool
	perturb = func(v reflect.Value) bool {
		switch v.Kind() {
		case reflect.String:
			v.SetString(v.String() + "~")
		case reflect.Bool:
			v.SetBool(!v.Bool())
		case reflect.Int, reflect.Int64:
			v.SetInt(v.Int() + 1)
		case reflect.Uint, reflect.Uint32, reflect.Uint64:
			v.SetUint(v.Uint() + 1)
		case reflect.Slice:
			if v.Type().Elem().Kind() != reflect.String {
				return false
			}
			v.Set(reflect.Append(v, reflect.ValueOf("zz~").Convert(v.Type().Elem())))
		case reflect.Struct:
			if v.NumField() == 0 {
				return false
			}
			for i := 0; i < v.NumField(); i++ {
				if !perturb(v.Field(i)) {
					return false
				}
			}
		default:
			return false
		}
		return true
	}
	rt := reflect.TypeOf(base)
	compared := 0
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		if !f.IsExported() {
			continue
		}
		d := base
		d.Sandbox, d.Memory = base.Sandbox, base.Memory
		d.Verbs = append([]string{}, base.Verbs...)
		d.CapsuleVerbs = append([]string{}, base.CapsuleVerbs...)
		d.OwnedKinds = append([]string{}, base.OwnedKinds...)
		d.Modules = append([]string{}, base.Modules...)
		if !perturb(reflect.ValueOf(&d).Elem().Field(i)) {
			t.Errorf("Desk.%s has a kind this test cannot perturb; teach the test, and put the field in DeskRow if Equal compares it", f.Name)
			continue
		}
		if base.Equal(d) {
			continue // not authority by Equal's definition
		}
		if f.Name == "Name" {
			continue // the map key
		}
		compared++
		if reflect.DeepEqual(DeskRowOf(base), DeskRowOf(d)) {
			t.Errorf("Desk.%s is compared by Desk.Equal but changing it does not change the lock's DeskRow", f.Name)
		}
	}
	if compared < 15 {
		t.Fatalf("only %d fields compared; the test has stopped seeing Desk.Equal", compared)
	}
	// And the other way: every DeskRow field is fed by some Desk field (no
	// dead row fields): perturbing all of Desk changes every DeskRow field.
	d := base
	d.Verbs = append([]string{}, base.Verbs...)
	d.CapsuleVerbs = append([]string{}, base.CapsuleVerbs...)
	d.OwnedKinds = append([]string{}, base.OwnedKinds...)
	d.Modules = append([]string{}, base.Modules...)
	dv := reflect.ValueOf(&d).Elem()
	for i := 0; i < dv.NumField(); i++ {
		if dv.Type().Field(i).IsExported() {
			perturb(dv.Field(i))
		}
	}
	a, b := reflect.ValueOf(DeskRowOf(base)), reflect.ValueOf(DeskRowOf(d))
	for i := 0; i < a.NumField(); i++ {
		if reflect.DeepEqual(a.Field(i).Interface(), b.Field(i).Interface()) {
			t.Errorf("DeskRow.%s is not fed by any Desk field", a.Type().Field(i).Name)
		}
	}
}
