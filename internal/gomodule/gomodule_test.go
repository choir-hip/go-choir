package gomodule

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Ways the gate could fail, written first (design section 6): a banned import
// reached through a dependency or hidden behind a build tag passes; cgo,
// linkname, embed or assembly passes; an unlisted, re-versioned or altered
// module passes; our h1 differs from Go's; a stray or internal package is
// loadable; a tampered bundle passes the worker's re-check.

const humanizePath = "github.com/dustin/go-humanize"

func readModule(t *testing.T, dir string) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		b, err := os.ReadFile(p)
		out[filepath.ToSlash(rel)] = b
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func humanize(t *testing.T) ModuleSource {
	t.Helper()
	m := Module{Path: humanizePath, Version: "v1.0.1"}
	return ModuleSource{Module: m, Files: readModule(t, filepath.Join("testdata", "gomod", m.Dir()))}
}

// Failure mode 5: the h1 we compute is the Go checksum database's. The lines
// are the ones in Choir's own go.sum, which every build has verified against
// sum.golang.org. Every curated entry has its module content in testdata and
// hashes to its line.
func TestHash1MatchesGoSum(t *testing.T) {
	goSum, err := os.ReadFile(filepath.Join("..", "..", "go.sum"))
	if err != nil {
		t.Fatal(err)
	}
	h := humanize(t)
	sum, err := Hash1(h.Module.String(), h.Files)
	if err != nil {
		t.Fatal(err)
	}
	if want := humanizePath + " v1.0.1 " + sum + "\n"; !strings.Contains(string(goSum), want) {
		t.Fatalf("h1 %s is not go.sum's line for %s", sum, h.Module)
	}
	cur, err := Curated()
	if err != nil || len(cur) == 0 {
		t.Fatalf("curated list: %v %v", cur, err)
	}
	for _, c := range cur {
		files := readModule(t, filepath.Join("testdata", "gomod", c.Dir()))
		sum, err := Hash1(c.String(), files)
		if err != nil {
			t.Fatal(err)
		}
		if sum != c.Sum {
			t.Errorf("curated %s says %s; its testdata copy hashes to %s", c, c.Sum, sum)
		}
		if !strings.Contains(string(goSum), c.Path+" "+c.Version+" "+c.Sum+"\n") {
			t.Logf("%s is not in Choir's go.sum; scripts/check-curated-go-modules checks it against GOSUMDB", c)
		}
	}
}

func TestCuratedListFormat(t *testing.T) {
	good := "# c\n\nexample.com/a v1.2.3 h1:GzkhY7T5VNhEkwH0PVJgjz+fX1rhBrR7pRT3mDkpeCY=\n"
	if m, err := ParseCurated([]byte(good)); err != nil || len(m) != 1 || m[0].Path != "example.com/a" {
		t.Fatalf("%v %v", m, err)
	}
	for name, bad := range map[string]string{
		"two fields":   "example.com/a v1.2.3\n",
		"bad version":  "example.com/a 1.2.3 h1:GzkhY7T5VNhEkwH0PVJgjz+fX1rhBrR7pRT3mDkpeCY=\n",
		"bad hash":     "example.com/a v1.2.3 h1:xyz\n",
		"no domain":    "examplecom/a v1.2.3 h1:GzkhY7T5VNhEkwH0PVJgjz+fX1rhBrR7pRT3mDkpeCY=\n",
		"listed twice": good + "example.com/a v1.2.4 h1:GzkhY7T5VNhEkwH0PVJgjz+fX1rhBrR7pRT3mDkpeCY=\n",
	} {
		if _, err := ParseCurated([]byte(bad)); CodeOf(err) != CodeCuratedFormat {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestResolveHumanizeLoadsOnlyWhatTheDeskReaches(t *testing.T) {
	// go-humanize is not curated (yaegi cannot interpret it); a list naming
	// it with its go.sum hash tests the closure on a real module.
	cur := []Module{{Path: humanizePath, Version: "v1.0.1", Sum: "h1:GzkhY7T5VNhEkwH0PVJgjz+fX1rhBrR7pRT3mDkpeCY="}}
	b, err := Resolve(cur, []ModuleSource{humanize(t)}, []string{humanizePath})
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Imports) != 1 || len(b.Modules) != 1 || b.Modules[0].Sum == "" {
		t.Fatalf("bundle %+v", b.Modules)
	}
	raw, err := b.Encode()
	if err != nil {
		t.Fatal(err)
	}
	back, err := DecodeBundle(raw)
	if err != nil {
		t.Fatal(err)
	}
	if again, _ := back.Encode(); string(again) != string(raw) {
		t.Fatal("encoding is not canonical")
	}
	l, err := back.Load(cur)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fs.Stat(l.FS, "src/"+humanizePath+"/comma.go"); err != nil {
		t.Fatal(err)
	}
	// Tests ride in the bundle (the worker re-hashes the whole module) but
	// are never in the interpreter's filesystem.
	if _, err := fs.Stat(l.FS, "src/"+humanizePath+"/comma_test.go"); err == nil {
		t.Fatal("a test file is in the interpreter's filesystem")
	}
	// english is a package of the module no desk import reaches: not loaded.
	if _, err := fs.Stat(l.FS, "src/"+humanizePath+"/english/words.go"); err == nil {
		t.Fatal("an unreached package is in the interpreter's filesystem")
	}
	// Reached when imported, with what it imports (english imports
	// humanize itself).
	b2, err := Resolve(cur, []ModuleSource{humanize(t)}, []string{humanizePath + "/english"})
	if err != nil {
		t.Fatal(err)
	}
	l2, err := b2.Load(cur)
	if err != nil {
		t.Fatal(err)
	}
	if len(l2.Packages) != 2 {
		t.Fatalf("english reaches %v", l2.Packages)
	}
}

// fixture builds in-memory modules and a curated list that admits them, so
// each refusal below is the closure's, not the list's.
type fixture struct {
	mods []ModuleSource
}

func (f *fixture) add(path string, files map[string]string) {
	m := ModuleSource{Module: Module{Path: path, Version: "v0.1.0"}, Files: map[string][]byte{}}
	for k, v := range files {
		m.Files[k] = []byte(v)
	}
	f.mods = append(f.mods, m)
}

func (f *fixture) curated(t *testing.T) []Module {
	t.Helper()
	var out []Module
	for _, m := range f.mods {
		sum, err := Hash1(m.Module.String(), m.Files)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, Module{Path: m.Module.Path, Version: m.Module.Version, Sum: sum})
	}
	return out
}

func (f *fixture) resolve(t *testing.T, imports ...string) (Bundle, error) {
	t.Helper()
	return Resolve(f.curated(t), f.mods, imports)
}

func lib(imports ...string) string {
	s := "package lib\n"
	for _, i := range imports {
		s += "import _ \"" + i + "\"\n"
	}
	return s + "func F() int { return 1 }\n"
}

// Failure modes 1 to 3: every refusal, each through a dependency where that
// is how it would hide.
func TestResolveRefusesTheClosure(t *testing.T) {
	cases := []struct {
		name  string
		build func(f *fixture)
		code  string
		chain string
	}{
		{"os/exec via a dependency", func(f *fixture) {
			f.add("example.com/lib", map[string]string{"lib.go": lib("example.com/lib/dep"), "dep/dep.go": strings.Replace(lib("os/exec"), "package lib", "package dep", 1)})
		}, CodeBannedImport, "example.com/lib -> example.com/lib/dep"},
		{"unsafe via a dependency", func(f *fixture) {
			f.add("example.com/lib", map[string]string{"lib.go": lib("example.com/lib/dep"), "dep/dep.go": "package dep\nimport \"unsafe\"\nvar X = unsafe.Sizeof(0)\n"})
		}, CodeBannedImport, "example.com/lib/dep"},
		{"syscall via a second module", func(f *fixture) {
			f.add("example.com/lib", map[string]string{"lib.go": lib("example.com/other")})
			f.add("example.com/other", map[string]string{"other.go": "package other\nimport _ \"syscall\"\n"})
		}, CodeBannedImport, "example.com/lib -> example.com/other"},
		{"net two levels down", func(f *fixture) {
			f.add("example.com/lib", map[string]string{"lib.go": lib("example.com/lib/a"), "a/a.go": "package a\nimport _ \"example.com/lib/b\"\n", "b/b.go": "package b\nimport _ \"net\"\n"})
		}, CodeBannedImport, "example.com/lib -> example.com/lib/a -> example.com/lib/b"},
		{"banned import behind a build tag", func(f *fixture) {
			f.add("example.com/lib", map[string]string{"lib.go": lib(), "lib_linux.go": "//go:build linux\n\npackage lib\nimport _ \"syscall\"\n"})
		}, CodeBannedImport, ""},
		{"runtime/debug", func(f *fixture) {
			f.add("example.com/lib", map[string]string{"lib.go": lib("runtime/debug")})
		}, CodeBannedImport, ""},
		{"stdlib outside the set", func(f *fixture) {
			f.add("example.com/lib", map[string]string{"lib.go": lib("path/filepath")})
		}, CodeOutsideSet, ""},
		{"the choir verbs", func(f *fixture) {
			f.add("example.com/lib", map[string]string{"lib.go": lib("choir")})
		}, CodeOutsideSet, ""},
		{"cgo", func(f *fixture) {
			f.add("example.com/lib", map[string]string{"lib.go": lib("example.com/lib/c"), "c/c.go": "package c\n// #include <stdio.h>\nimport \"C\"\n"})
		}, CodeCgo, ""},
		{"linkname", func(f *fixture) {
			f.add("example.com/lib", map[string]string{"lib.go": lib() + "\n//go:linkname now time.now\nfunc now() int64\n"})
		}, CodeDirective, ""},
		{"embed directive", func(f *fixture) {
			f.add("example.com/lib", map[string]string{"lib.go": "package lib\n\n//go:embed x.txt\nvar s string\n", "x.txt": "x"})
		}, CodeDirective, ""},
		{"assembly", func(f *fixture) {
			f.add("example.com/lib", map[string]string{"lib.go": lib(), "lib_amd64.s": "TEXT ·F(SB),$0\n"})
		}, CodeNonGoSource, ""},
		{"objective-c++ (cgo input)", func(f *fixture) {
			f.add("example.com/lib", map[string]string{"lib.go": lib(), "x.mm": "int f(void);\n"})
		}, CodeNonGoSource, ""},
		{"an undeclared module", func(f *fixture) {
			f.add("example.com/lib", map[string]string{"lib.go": lib("example.com/elsewhere")})
		}, CodeUnresolved, ""},
		{"a relative import", func(f *fixture) {
			f.add("example.com/lib", map[string]string{"lib.go": lib("./x")})
		}, CodeUnresolved, ""},
		{"another tree's internal package", func(f *fixture) {
			f.add("example.com/lib", map[string]string{"lib.go": lib("example.com/other/internal/x")})
			f.add("example.com/other", map[string]string{"internal/x/x.go": "package x\n"})
		}, CodeInternal, ""},
		{"not Go", func(f *fixture) {
			f.add("example.com/lib", map[string]string{"lib.go": "package lib\nfunc {"})
		}, CodeParse, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := &fixture{}
			c.build(f)
			_, err := f.resolve(t, "example.com/lib")
			if !errors.Is(err, ErrRefused) || CodeOf(err) != c.code {
				t.Fatalf("got %v, want %s", err, c.code)
			}
			if c.chain != "" && !strings.Contains(err.Error(), c.chain) {
				t.Fatalf("refusal %q does not name the chain %q", err, c.chain)
			}
		})
	}
}

func TestResolveAdmitsAPureClosureAndItsOwnInternal(t *testing.T) {
	f := &fixture{}
	f.add("example.com/lib", map[string]string{
		"lib.go": lib("example.com/lib/internal/x", "strings", "sync"), "internal/x/x.go": "package x\n",
		"cmd/tool/main.go": "package main\nimport _ \"os\"\n", // unreached: not checked, not loaded
		"lib_test.go":      "package lib\nimport _ \"os\"\n",  // a test: not built
		"testdata/a.go":    "package a\nimport _ \"os\"\n",    // testdata: never built
	})
	b, err := f.resolve(t, "example.com/lib")
	if err != nil {
		t.Fatal(err)
	}
	l, err := b.Load(f.curated(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(l.Packages) != 2 {
		t.Fatalf("packages %v", l.Packages)
	}
	for _, p := range []string{"src/example.com/lib/cmd/tool/main.go", "src/example.com/lib/lib_test.go", "src/example.com/lib/testdata/a.go"} {
		if _, err := fs.Stat(l.FS, p); err == nil {
			t.Errorf("%s is in the interpreter's filesystem", p)
		}
	}
}

// Failure mode 4: the list is the gate.
func TestResolveRefusesWhatTheListDoesNotHold(t *testing.T) {
	f := &fixture{}
	f.add("example.com/lib", map[string]string{"lib.go": lib()})
	cur := f.curated(t)
	if _, err := Resolve(nil, f.mods, []string{"example.com/lib"}); CodeOf(err) != CodeNotCurated {
		t.Fatalf("unlisted: %v", err)
	}
	other := []Module{{Path: "example.com/lib", Version: "v0.2.0", Sum: cur[0].Sum}}
	if _, err := Resolve(other, f.mods, []string{"example.com/lib"}); CodeOf(err) != CodeNotCurated {
		t.Fatalf("other version: %v", err)
	}
	f.mods[0].Files["lib.go"] = []byte(lib() + "// one more line\n")
	if _, err := Resolve(cur, f.mods, []string{"example.com/lib"}); CodeOf(err) != CodeSumMismatch {
		t.Fatalf("altered: %v", err)
	}
	// A file added to the module (not a Go file, not reached) still moves the hash.
	f.mods[0].Files["lib.go"] = []byte(lib())
	f.mods[0].Files["NOTES"] = []byte("x")
	if _, err := Resolve(cur, f.mods, []string{"example.com/lib"}); CodeOf(err) != CodeSumMismatch {
		t.Fatalf("file added: %v", err)
	}
}

// Failure mode 7: cells import only declared, non-internal packages.
func TestResolveRefusesACellImportOfAnInternalPackage(t *testing.T) {
	f := &fixture{}
	f.add("example.com/lib", map[string]string{"lib.go": lib(), "internal/x/x.go": "package x\n"})
	if _, err := f.resolve(t, "example.com/lib/internal/x"); CodeOf(err) != CodeInternal {
		t.Fatalf("%v", err)
	}
}

func TestResolveCaps(t *testing.T) {
	f := &fixture{}
	big := "package lib\nvar S = `" + strings.Repeat("x", MaxBundleBytes) + "`\n"
	f.add("example.com/lib", map[string]string{"lib.go": big})
	if _, err := f.resolve(t, "example.com/lib"); CodeOf(err) != CodeTooLarge {
		t.Fatalf("%v", err)
	}
}

// Failure mode 8's other half: the worker's re-check (Load) refuses a bundle
// that was altered after the publish check, re-hashing every module.
func TestBundleLoadRefusesTampering(t *testing.T) {
	f := &fixture{}
	f.add("example.com/lib", map[string]string{"lib.go": lib("example.com/lib/dep"), "dep/dep.go": "package dep\n", "lib_test.go": "package lib\n"})
	cur := f.curated(t)
	good, err := f.resolve(t, "example.com/lib")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := good.Load(cur); err != nil {
		t.Fatal(err)
	}
	mut := func(fn func(b *Bundle)) Bundle {
		raw, _ := good.Encode()
		b, err := DecodeBundle(raw)
		if err != nil {
			t.Fatal(err)
		}
		fn(&b)
		return b
	}
	file := func(b *Bundle, name string) *BundleFile {
		for i := range b.Modules[0].Files {
			if b.Modules[0].Files[i].Name == name {
				return &b.Modules[0].Files[i]
			}
		}
		t.Fatalf("no %s", name)
		return nil
	}
	cases := map[string]struct {
		b    Bundle
		code string
	}{
		"banned import injected": {mut(func(b *Bundle) { file(b, "dep/dep.go").Data = []byte("package dep\nimport _ \"os\"\n") }), CodeSumMismatch},
		// A test file's bytes are hashed too: a change to one fails the
		// re-hash even though the interpreter never sees tests.
		"test file altered": {mut(func(b *Bundle) { file(b, "lib_test.go").Data = []byte("package lib\nimport _ \"os\"\n") }), CodeSumMismatch},
		"file added": {mut(func(b *Bundle) {
			b.Modules[0].Files = append(b.Modules[0].Files, BundleFile{Name: "zz.go", Data: []byte("package lib\n")})
		}), CodeSumMismatch},
		"sum swapped":          {mut(func(b *Bundle) { b.Modules[0].Sum = "h1:GzkhY7T5VNhEkwH0PVJgjz+fX1rhBrR7pRT3mDkpeCY=" }), CodeSumMismatch},
		"not curated":          {mut(func(b *Bundle) { b.Modules[0].Version = "v0.2.0" }), CodeNotCurated},
		"internal cell import": {mut(func(b *Bundle) { b.Imports = []string{"example.com/lib/internal/x"} }), CodeInternal},
		"path traversal":       {mut(func(b *Bundle) { file(b, "dep/dep.go").Name = "../x.go" }), CodeBadBundle},
		"unreached module": {mut(func(b *Bundle) {
			b.Imports = []string{}
		}), CodeBadBundle},
	}
	for name, c := range cases {
		if _, err := c.b.Load(cur); CodeOf(err) != c.code {
			t.Errorf("%s: %v, want %s", name, err, c.code)
		}
	}
	// The re-check reads the curated list it is given: the worker's own.
	if _, err := good.Load(nil); CodeOf(err) != CodeNotCurated {
		t.Errorf("empty list: %v", err)
	}
	raw, _ := good.Encode()
	if _, err := DecodeBundle(append(raw, []byte(`{}`)...)); err == nil {
		t.Error("trailing data accepted")
	}
	if _, err := DecodeBundle([]byte(strings.Replace(string(raw), `"modules"`, `"modules_x":[],"modules"`, 1))); err == nil {
		t.Error("unknown field accepted")
	}
}

// D6: a module inside another's path would make the provider of a package
// ambiguous.
func TestResolveRefusesOverlappingModules(t *testing.T) {
	f := &fixture{}
	f.add("example.com/lib", map[string]string{"lib.go": lib("example.com/lib/sub")})
	f.add("example.com/lib/sub", map[string]string{"sub.go": "package sub\n"})
	if _, err := f.resolve(t, "example.com/lib"); CodeOf(err) != CodeOverlap {
		t.Fatalf("%v", err)
	}
}

// D8: a package named choir, or whose path ends in choir, would be bound to
// the desk's host verbs by the cell's import aliasing.
func TestResolveRefusesAChoirNamedPackage(t *testing.T) {
	f := &fixture{}
	f.add("example.com/lib", map[string]string{"choir/c.go": "package choir\n", "x/x.go": "package choir\n", "lib.go": lib()})
	if _, err := f.resolve(t, "example.com/lib/choir"); CodeOf(err) != CodeReservedName {
		t.Fatalf("leaf: %v", err)
	}
	if _, err := f.resolve(t, "example.com/lib/x"); CodeOf(err) != CodeReservedName {
		t.Fatalf("package clause: %v", err)
	}
}

func TestStdlibSetHoldsNothingBanned(t *testing.T) {
	for _, p := range Stdlib() {
		if _, banned := BannedReason(p); banned {
			t.Errorf("%s is both in the set and banned", p)
		}
	}
	for _, p := range []string{"os", "os/exec", "unsafe", "syscall", "net", "net/http", "reflect", "runtime", "plugin", "path/filepath", "text/template", "log", "crypto/rand", "io/fs"} {
		if InStdlib(p) {
			t.Errorf("%s is in the desk set", p)
		}
	}
	if len(Stdlib()) != 54 {
		t.Errorf("the set has %d packages; the design names 54", len(Stdlib()))
	}
}
