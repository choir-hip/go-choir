package apppackage

import (
	"reflect"
	"slices"
	"testing"
)

// MA6.6: a package desk's import set is derived from what the behaviour
// surface hashes (the base set, a platform fact, plus the desk's verified
// go_imports), so one hash means one set on one platform version, and a change
// to the set moves the hash. Failure modes, written first:
//
//	S1 the set depends on something the hash does not cover, so two releases
//	   with one hash give a desk different imports;
//	S2 a go_imports change moves the set but not the hash (an update the owner
//	   never approves);
//	S3 authoring order (capability or go_imports order) moves the set or the
//	   hash;
//	S4 a desk's set holds a package with effects.
func TestMA66DeskImportSetIsStableUnderTheSurfaceHash(t *testing.T) {
	code := "package main\nimport (\"fmt\"; \"github.com/pmezard/go-difflib/difflib\")\nfunc main(){ fmt.Println(difflib.SplitLines(\"a\")) }\n"
	files := merge(deskFiles(code), difflibFiles(t))
	parse := func(mut func(map[string]any)) Manifest {
		t.Helper()
		m, err := ParseManifest(v2JSON(t, mut), OriginFirstParty)
		if err != nil {
			t.Fatal(err)
		}
		return m
	}
	with := parse(goDeskV([]string{difflibRef}, difflib))
	h1, err := with.BehaviourSurface(files)
	if err != nil {
		t.Fatal(err)
	}
	s1, err := with.DeskImportSets(files)
	if err != nil {
		t.Fatal(err)
	}
	if len(s1) != 1 {
		t.Fatalf("sets = %v", s1)
	}
	var desk string
	var set []string
	for desk, set = range s1 {
	}
	if !slices.Contains(set, difflib) || !slices.Contains(set, "choir") || !slices.Contains(set, "math") {
		t.Fatalf("desk %s set lacks the base or its go_imports: %v", desk, set)
	}
	// S4.
	for _, p := range set {
		switch p {
		case "os", "os/exec", "net", "net/http", "syscall", "unsafe", "reflect", "runtime", "plugin":
			t.Fatalf("desk %s set holds %s", desk, p)
		}
	}
	// S1/S3: recomputed, and with the capability list reversed, the hash and
	// the set hold.
	reversed := parse(func(v map[string]any) {
		goDeskV([]string{difflibRef}, difflib)(v)
		if caps, ok := v["capabilities"].([]any); ok {
			slices.Reverse(caps)
		}
	})
	h2, err := reversed.BehaviourSurface(files)
	if err != nil {
		t.Fatal(err)
	}
	s2, err := reversed.DeskImportSets(files)
	if err != nil {
		t.Fatal(err)
	}
	if h1 != h2 || !reflect.DeepEqual(s1, s2) {
		t.Fatalf("authoring order moved the hash (%v) or the set (%v)", h1 != h2, !reflect.DeepEqual(s1, s2))
	}
	// S2: dropping the go_import (and the code that used it) moves both.
	plain := parse(goDeskV([]string{difflibRef}))
	plainFiles := merge(deskFiles("package main\n"), difflibFiles(t))
	h3, err := plain.BehaviourSurface(plainFiles)
	if err != nil {
		t.Fatal(err)
	}
	s3, err := plain.DeskImportSets(plainFiles)
	if err != nil {
		t.Fatal(err)
	}
	if h3 == h1 || reflect.DeepEqual(s3, s1) {
		t.Fatalf("a go_imports change: hash moved %v, set moved %v", h3 != h1, !reflect.DeepEqual(s3, s1))
	}
	base := plain.ImportSet()
	slices.Sort(base)
	if got := s3[desk]; !reflect.DeepEqual(got, base) {
		t.Fatalf("a desk with no go_imports does not have the base set: %v", got)
	}
	// The sets refuse what the hash refuses: the module's import in a desk
	// that did not declare it.
	if _, err := plain.DeskImportSets(files); err == nil {
		t.Fatal("DeskImportSets accepted a desk importing outside its set")
	}
}
