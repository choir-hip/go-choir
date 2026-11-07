package apppackage

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The generator reads the committed allowlist, never the directory
// (docs/design/package-operation-decision-2026-11-06.md 2.6): a package a
// capsule added under src/packages, with kinds, is neither built-in nor
// registered; a listed name with no manifest is refused.
func TestGeneratorReadsOnlyTheBasePackageAllowlist(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not on PATH")
	}
	script, err := os.ReadFile(filepath.Join("..", "..", "frontend", "scripts", "gen-package-kinds.mjs"))
	if err != nil {
		t.Fatal(err)
	}
	write := func(root, rel, content string) {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	manifest := func(name, kinds string) string {
		return `{"schema":"choir.package/v2","name":"` + name + `","kinds":` + kinds + `}`
	}
	kind := `[{"kind":"choir.app.x.y","identity":"external_key","schema_version":1,"schema":{"type":"object"}}]`
	run := func(root string) (string, error) {
		cmd := exec.Command(node, filepath.Join(root, "frontend", "scripts", "gen-package-kinds.mjs"))
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	setup := func(list string) string {
		root := t.TempDir()
		write(root, "frontend/scripts/gen-package-kinds.mjs", string(script))
		write(root, "internal/apppackage/keep", "")
		write(root, "frontend/src/packages/base-packages.json", list)
		write(root, "frontend/src/packages/listed/choir-package.json", manifest("listed", kind))
		write(root, "frontend/src/packages/unlisted/choir-package.json", manifest("unlisted", kind))
		return root
	}

	root := setup(`{"schema":"choir.base-packages/v1","packages":["listed"]}`)
	if out, err := run(root); err != nil {
		t.Fatalf("generate: %v\n%s", err, out)
	}
	gen, err := os.ReadFile(filepath.Join(root, "internal", "apppackage", "firstparty_gen.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(gen), `"listed"`) || strings.Contains(string(gen), "unlisted") {
		t.Fatalf("generated table must name the listed package only:\n%s", gen)
	}

	root = setup(`{"schema":"choir.base-packages/v1","packages":["listed","ghost"]}`)
	if out, err := run(root); err == nil || !strings.Contains(out, "ghost") {
		t.Fatalf("a listed name with no manifest was accepted: %v\n%s", err, out)
	}
}
