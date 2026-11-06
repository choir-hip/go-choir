package pkgrelease

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/apppackage"
	"github.com/yusefmosiah/go-choir/internal/gomodule"
)

// MA6.5 at the publish seam: a release whose desk loads a Go module is checked
// by the same CheckManifest corpusd runs on publish and the computer runs on
// install. A desk from a published origin waits for the R24 gate
// (ReasonDeskNeedsSandbox), so these state the Phase 6 platform to reach the
// module gate behind it.

const difflibDirName = "go-difflib@v1.0.1-0.20181226105442-5d4384ee4fb2"

func goModuleRelease(t *testing.T, alter func(map[string]string)) (raw []byte, files apppackage.Source) {
	t.Helper()
	raw = readerManifest(t, func(v map[string]any) {
		v["go_modules"] = []any{"github.com/pmezard/go-difflib@v1.0.1-0.20181226105442-5d4384ee4fb2"}
		v["contributes"] = map[string]any{"desks": []any{map[string]any{
			"name": "reader", "prompt": "p.md", "code": "desk/main.go",
			"go_imports": []any{"github.com/pmezard/go-difflib/difflib"},
		}}}
	})
	m := map[string]string{
		"App.svelte":   "<p>x</p>",
		"p.md":         "format sizes",
		"desk/main.go": "package main\nimport \"github.com/pmezard/go-difflib/difflib\"\nvar _ = difflib.SplitLines\n",
	}
	root := filepath.Join("..", "gomodule", "testdata", "gomod", "github.com", "pmezard", difflibDirName)
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		b, err := os.ReadFile(p)
		m[apppackage.GoModuleRoot+"/github.com/pmezard/"+difflibDirName+"/"+filepath.ToSlash(rel)] = string(b)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if alter != nil {
		alter(m)
	}
	return raw, apppackage.MapSource(m)
}

func TestPublishChecksTheGoModules(t *testing.T) {
	sandboxed := apppackage.Platform{DeskSandbox: true}
	raw, files := goModuleRelease(t, nil)
	sum, _, _, err := checkManifestOn("reader", raw, files, sandboxed)
	if err != nil {
		t.Fatalf("curated module refused: %v", err)
	}
	if sum.DesksCount != 1 || len(sum.BehaviourSurface) != 64 {
		t.Fatalf("summary %+v", sum)
	}
	// This build still holds R24's gate: the same release is refused for the
	// sandbox before the module gate runs.
	if _, _, err := CheckManifest("reader", raw, files); ReasonOf(err) != ReasonDeskNeedsSandbox {
		t.Fatalf("R24 gate: %v", err)
	}
	cases := map[string]struct {
		alter func(map[string]string)
		code  string
	}{
		"a module file altered (its checksum database hash no longer holds)": {func(m map[string]string) {
			f := apppackage.GoModuleRoot + "/github.com/pmezard/" + difflibDirName + "/difflib/difflib.go"
			m[f] = strings.Replace(m[f], "import (", "import (\n\t_ \"os/exec\"", 1)
		}, gomodule.CodeSumMismatch},
		"a stray Go file beside the module": {func(m map[string]string) {
			m[apppackage.GoModuleRoot+"/example.com/evil@v0.1.0/evil.go"] = "package evil\nimport _ \"os/exec\"\n"
		}, gomodule.CodeStrayFile},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			raw, files := goModuleRelease(t, c.alter)
			_, _, _, err := checkManifestOn("reader", raw, files, sandboxed)
			// The refusal's detail carries the gomodule code.
			if ReasonOf(err) != ReasonGoModuleRefused || !strings.Contains(err.Error(), c.code) {
				t.Fatalf("got %v, want %s naming %s", err, ReasonGoModuleRefused, c.code)
			}
		})
	}
	// A module not on the curated list.
	raw2 := readerManifest(t, func(v map[string]any) {
		v["go_modules"] = []any{"example.com/lib@v0.1.0"}
		v["contributes"] = map[string]any{"desks": []any{map[string]any{"name": "reader", "prompt": "p.md", "go_imports": []any{"example.com/lib"}}}}
	})
	files2 := apppackage.MapSource(map[string]string{"App.svelte": "<p>x</p>", "p.md": "x", apppackage.GoModuleRoot + "/example.com/lib@v0.1.0/lib.go": "package lib\n"})
	if _, _, _, err := checkManifestOn("reader", raw2, files2, sandboxed); ReasonOf(err) != ReasonGoModuleRefused || !strings.Contains(err.Error(), gomodule.CodeNotCurated) {
		t.Fatalf("not curated: %v", err)
	}
}
