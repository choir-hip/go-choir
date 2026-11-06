package apppackage

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/gomodule"
)

// MA6.5: a package ships Go modules (docs/design/ma65-yaegi-go-modules-2026-11-06.md).
//
// Ways the publish gate could fail, written first:
//
//	G1 a module reached through a desk's go_imports imports os/exec, unsafe,
//	   syscall or net through a dependency, and a hash is issued;
//	G2 a module not on the curated list, or whose bytes differ from its
//	   curated (checksum database) hash, is accepted;
//	G3 a Go file under gomod/ that belongs to no declared module rides along;
//	G4 desk code imports a module package its desk did not declare;
//	G5 adding modules moves the hash of packages without them (the fields are
//	   omitted when empty), or a module version bump does not move the hash.

const (
	difflibMod = "github.com/pmezard/go-difflib"
	difflibVer = "v1.0.1-0.20181226105442-5d4384ee4fb2"
	difflibRef = difflibMod + "@" + difflibVer
	difflib    = difflibMod + "/difflib"
	difflibDir = GoModuleRoot + "/" + difflibRef + "/"
)

// difflibFiles is the real, curated go-difflib module content, at its release
// path.
func difflibFiles(t *testing.T) mapFiles {
	t.Helper()
	root := filepath.Join("..", "gomodule", "testdata", "gomod", "github.com", "pmezard", "go-difflib@"+difflibVer)
	out := mapFiles{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		b, err := os.ReadFile(p)
		out[difflibDir+filepath.ToSlash(rel)] = string(b)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func goDeskV(mods []string, imports ...string) func(map[string]any) {
	return func(v map[string]any) {
		deskV(v)
		d := v["contributes"].(map[string]any)["desks"].([]any)[0].(map[string]any)
		if len(imports) > 0 {
			d["go_imports"] = imports
		}
		if len(mods) > 0 {
			v["go_modules"] = mods
		}
	}
}

func deskFiles(code string) mapFiles {
	return mapFiles{"prompts/desk.md": "be careful", "desk/main.go": code}
}

func merge(fs ...mapFiles) mapFiles {
	out := mapFiles{}
	for _, f := range fs {
		for k, v := range f {
			out[k] = v
		}
	}
	return out
}

func TestGoModuleDifflibPassesTheGateAndBuildsTheBundle(t *testing.T) {
	m, err := ParseManifest(v2JSON(t, goDeskV([]string{difflibRef}, difflib)), OriginFirstParty)
	if err != nil {
		t.Fatal(err)
	}
	code := "package main\nimport (\"fmt\"; \"github.com/pmezard/go-difflib/difflib\")\nfunc main(){ fmt.Println(difflib.SplitLines(\"a\")) }\n"
	files := merge(deskFiles(code), difflibFiles(t))
	with, err := m.BehaviourSurface(files)
	if err != nil {
		t.Fatal(err)
	}
	b, err := m.DeskGoBundle("p2btest", files)
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Imports) != 1 || b.Imports[0] != difflib || len(b.Modules) != 1 || len(b.Modules[0].Files) == 0 {
		t.Fatalf("bundle %v %d", b.Imports, len(b.Modules))
	}
	cur, err := gomodule.Curated()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.Load(cur); err != nil {
		t.Fatal(err)
	}
	// G4: the same code in a desk that did not declare the module is refused.
	plain, err := ParseManifest(v2JSON(t, goDeskV([]string{difflibRef})), OriginFirstParty)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := plain.BehaviourSurface(files); !errors.Is(err, ErrInvalidManifest) || !strings.Contains(err.Error(), "outside the desk's import set") {
		t.Fatalf("undeclared module import: %v", err)
	}
	// G5: the module list and go_imports are in the hash.
	plainCode := deskFiles("package main\n")
	a, err := plain.BehaviourSurface(merge(plainCode, difflibFiles(t)))
	if err != nil {
		t.Fatal(err)
	}
	if a == with {
		t.Fatal("go_imports did not move the hash")
	}
}

func TestGoModuleFieldsAbsentKeepTheHash(t *testing.T) {
	m, err := ParseManifest(v2JSON(t, deskV), OriginFirstParty)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := m.surfaceDocument(deskFiles("package main\n"))
	if err != nil {
		t.Fatal(err)
	}
	if doc.GoModules != nil || doc.Desks[0].GoImports != nil {
		t.Fatal("the surface names go modules for a package with none")
	}
	// The hashes of packages without modules are the ones the tree before
	// MA6.5 (b1f3d58de) computed for the same bytes.
	d, err := m.BehaviourSurface(mapFiles{"prompts/desk.md": "be careful", "desk/main.go": "package main\nimport \"fmt\"\nvar _ = fmt.Sprint\n"})
	if err != nil || d != "1080083af7eb0e33e424100b13b7040fc504ade434796467958d6d70e5aed666" {
		t.Fatalf("desk package hash moved: %s %v", d, err)
	}
	a, err := fixture(t).BehaviourSurface(mapFiles{"App.svelte": "x"})
	if err != nil || a != "153a784137be07bc3250f2fe8f4e6a8594578db15aa71d45caf3a5d8a26d54a0" {
		t.Fatalf("app package hash moved: %s %v", a, err)
	}
}

// D1: the surface names each module with its curated h1:, sorted by path.
func TestGoModuleSurfaceCarriesTheSum(t *testing.T) {
	m, err := ParseManifest(v2JSON(t, goDeskV([]string{difflibRef}, difflib)), OriginFirstParty)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := m.surfaceDocument(merge(deskFiles("package main\n"), difflibFiles(t)))
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.GoModules) != 1 || doc.GoModules[0].Sum != "h1:Jamvg5psRIccs7FGNTlIRMkT8wgtp5eCXdBlqhYGL6U=" || doc.GoModules[0].Version != difflibVer {
		t.Fatalf("surface modules %+v", doc.GoModules)
	}
}

// G2
func TestGoModuleNotCuratedOrAlteredIsRefused(t *testing.T) {
	m, err := ParseManifest(v2JSON(t, goDeskV([]string{difflibRef}, difflib)), OriginFirstParty)
	if err != nil {
		t.Fatal(err)
	}
	files := merge(deskFiles("package main\n"), difflibFiles(t))
	files[difflibDir+"difflib/difflib.go"] += "\n// altered\n"
	_, err = m.BehaviourSurface(files)
	if !errors.Is(err, ErrGoModule) || gomodule.CodeOf(err) != gomodule.CodeSumMismatch {
		t.Fatalf("altered: %v", err)
	}
	other, err := ParseManifest(v2JSON(t, goDeskV([]string{"example.com/lib@v0.1.0"}, "example.com/lib")), OriginFirstParty)
	if err != nil {
		t.Fatal(err)
	}
	_, err = other.BehaviourSurface(merge(deskFiles("package main\n"), mapFiles{GoModuleRoot + "/example.com/lib@v0.1.0/lib.go": "package lib\n"}))
	if !errors.Is(err, ErrGoModule) || gomodule.CodeOf(err) != gomodule.CodeNotCurated {
		t.Fatalf("not curated: %v", err)
	}
}

// G3
func TestGoModuleStrayFileIsRefused(t *testing.T) {
	m, err := ParseManifest(v2JSON(t, deskV), OriginFirstParty)
	if err != nil {
		t.Fatal(err)
	}
	_, err = m.BehaviourSurface(merge(deskFiles("package main\n"), mapFiles{GoModuleRoot + "/example.com/x@v1.0.0/x.go": "package x\n"}))
	if gomodule.CodeOf(err) != gomodule.CodeStrayFile {
		t.Fatalf("%v", err)
	}
}

// curate admits fixture modules for the closure tests below: each refusal is
// the closure's, not the list's.
func curate(t *testing.T, files mapFiles, mods ...string) {
	t.Helper()
	var list []gomodule.Module
	for _, ref := range mods {
		mod, err := gomodule.ParseRef(ref)
		if err != nil {
			t.Fatal(err)
		}
		content := map[string][]byte{}
		prefix := GoModuleRoot + "/" + mod.Dir() + "/"
		for k, v := range files {
			if strings.HasPrefix(k, prefix) {
				content[strings.TrimPrefix(k, prefix)] = []byte(v)
			}
		}
		mod.Sum, err = gomodule.Hash1(mod.String(), content)
		if err != nil {
			t.Fatal(err)
		}
		list = append(list, mod)
	}
	old := curatedGoModules
	curatedGoModules = func() ([]gomodule.Module, error) { return list, nil }
	t.Cleanup(func() { curatedGoModules = old })
}

// G1: the publish gate refuses a banned import reached through a dependency,
// naming the code and the chain.
func TestGoModuleTransitiveBannedImportIsRefusedAtPublish(t *testing.T) {
	for _, banned := range []string{"os/exec", "unsafe", "syscall", "net"} {
		t.Run(banned, func(t *testing.T) {
			files := merge(deskFiles("package main\n"), mapFiles{
				GoModuleRoot + "/example.com/lib@v0.1.0/lib.go":      "package lib\nimport _ \"example.com/dep\"\n",
				GoModuleRoot + "/example.com/dep@v0.2.0/dep.go":      "package dep\nimport _ \"example.com/dep/inner\"\n",
				GoModuleRoot + "/example.com/dep@v0.2.0/inner/in.go": "package inner\nimport _ \"" + banned + "\"\n",
			})
			curate(t, files, "example.com/lib@v0.1.0", "example.com/dep@v0.2.0")
			m, err := ParseManifest(v2JSON(t, goDeskV([]string{"example.com/lib@v0.1.0", "example.com/dep@v0.2.0"}, "example.com/lib")), OriginFirstParty)
			if err != nil {
				t.Fatal(err)
			}
			_, err = m.BehaviourSurface(files)
			if !errors.Is(err, ErrGoModule) || gomodule.CodeOf(err) != gomodule.CodeBannedImport {
				t.Fatalf("%v", err)
			}
			if !strings.Contains(err.Error(), "example.com/lib -> example.com/dep -> example.com/dep/inner") || !strings.Contains(err.Error(), `"`+banned+`"`) {
				t.Fatalf("refusal does not name the chain and import: %v", err)
			}
		})
	}
}

func TestGoModuleCgoLinknameAndAssemblyAreRefusedAtPublish(t *testing.T) {
	for name, c := range map[string]struct {
		files mapFiles
		code  string
	}{
		"cgo":      {mapFiles{GoModuleRoot + "/example.com/lib@v0.1.0/lib.go": "package lib\nimport \"C\"\n"}, gomodule.CodeCgo},
		"linkname": {mapFiles{GoModuleRoot + "/example.com/lib@v0.1.0/lib.go": "package lib\n\n//go:linkname f runtime.f\nfunc f()\n"}, gomodule.CodeDirective},
		"assembly": {mapFiles{GoModuleRoot + "/example.com/lib@v0.1.0/lib.go": "package lib\nfunc F()\n", GoModuleRoot + "/example.com/lib@v0.1.0/f_amd64.s": "TEXT ·F(SB),$0\n"}, gomodule.CodeNonGoSource},
	} {
		t.Run(name, func(t *testing.T) {
			files := merge(deskFiles("package main\n"), c.files)
			curate(t, files, "example.com/lib@v0.1.0")
			m, err := ParseManifest(v2JSON(t, goDeskV([]string{"example.com/lib@v0.1.0"}, "example.com/lib")), OriginFirstParty)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := m.BehaviourSurface(files); gomodule.CodeOf(err) != c.code {
				t.Fatalf("%v", err)
			}
		})
	}
}

func TestGoModuleManifestShape(t *testing.T) {
	for name, mut := range map[string]func(map[string]any){
		"not path@version":      goDeskV([]string{difflibMod}),
		"not semver":            goDeskV([]string{difflibMod + "@1.0.1"}),
		"twice":                 goDeskV([]string{difflibRef, difflibMod + "@v1.0.2"}),
		"import outside module": goDeskV([]string{difflibRef}, "example.com/other"),
		"import with no module": goDeskV(nil, difflib),
	} {
		if _, err := ParseManifest(v2JSON(t, mut), OriginFirstParty); !errors.Is(err, ErrInvalidManifest) {
			t.Errorf("%s: %v", name, err)
		}
	}
}
