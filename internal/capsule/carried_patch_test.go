package capsule

// Failure 15, problem 1, option A
// (docs/problems/engineering-completion-path-cluster-2026-10-20.md): a failed
// round's candidate tree is carried to the retry as an inert source patch.
// Ways the derivation can fail (written before the code):
//
//  1. The patch does not reproduce the candidate from the base tree.
//  2. An identical candidate yields a patch anyway (empty must stage nothing).
//  3. A huge candidate yields a truncated or unbounded patch (must refuse).
//  4. Non-source content (node_modules, .git, symlinks, binaries) leaks into
//     the patch.
//  5. A patch naming a path outside the source tree is accepted.
//  6. A patch that no longer applies to a moved base is called applicable,
//     or one that does apply is refused for an unrelated base change.
//  7. The same inputs give different bytes (the digest must be stable so a
//     restart re-derives the same offer).
//
// Failure 16 (the first staging drill, 2026-10-21): the delegate's sandbox
// leaves empty files named .env, .npmrc, package.json and the like at the
// source root, and the patch carried all of them.
//
//  8. A secret-named path (one the release refuses) is carried, with its
//     content, into the next capsule.
//  9. A new empty file (a sandbox mask) is carried as a new source file.
// 10. Leaving those out also drops a real change: a file emptied by the
//     candidate is still a change and must be carried.
// 11. The carried-patch rule and the release rule drift apart.
//
// Iteration on a rejected candidate (owner, 2026-10-25):
//
// 12. A patch that conflicts on a moved base in some files is refused without
//     naming which files, so the engineer redoes all of them; or a file that
//     still applies is named as a conflict.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestDiffTreesToPatchReproducesCandidate(t *testing.T) {
	requireGit(t)
	base, cand := t.TempDir(), t.TempDir()
	writeTree(t, base, map[string]string{"a.go": "package a\n", "b.go": "package b\n", "gone.go": "package gone\n"})
	writeTree(t, cand, map[string]string{"a.go": "package a\n\nfunc A() {}\n", "b.go": "package b\n", "new/c.go": "package c\n"})
	got, err := DiffTreesToPatch(context.Background(), base, cand, CarriedPatchMaxBytes)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(got.Patch)
	if got.SHA256 != hex.EncodeToString(sum[:]) {
		t.Fatalf("digest %q is not the patch's sha256", got.SHA256)
	}
	if strings.Join(got.Files, ",") != "a.go,gone.go,new/c.go" {
		t.Fatalf("files = %v, want the changed, deleted and added files in order", got.Files)
	}
	// Applying the patch to a copy of the base gives the candidate (mode 1).
	if err := CheckPatchAppliesToTree(context.Background(), got.Patch, got.Files, base); err != nil {
		t.Fatalf("patch does not apply to its own base: %v", err)
	}
	// Mode 7: the same trees give the same bytes.
	again, err := DiffTreesToPatch(context.Background(), base, cand, CarriedPatchMaxBytes)
	if err != nil || again.SHA256 != got.SHA256 {
		t.Fatalf("second derivation = %v, %v; want the same digest %s", again.SHA256, err, got.SHA256)
	}
}

func TestDiffTreesToPatchEmptyIsRefused(t *testing.T) {
	requireGit(t)
	base, cand := t.TempDir(), t.TempDir()
	same := map[string]string{"a.go": "package a\n"}
	writeTree(t, base, same)
	writeTree(t, cand, same)
	if _, err := DiffTreesToPatch(context.Background(), base, cand, CarriedPatchMaxBytes); !errors.Is(err, ErrCarriedPatchEmpty) {
		t.Fatalf("identical trees = %v, want ErrCarriedPatchEmpty", err)
	}
}

func TestDiffTreesToPatchOversizeIsRefusedNotTruncated(t *testing.T) {
	requireGit(t)
	base, cand := t.TempDir(), t.TempDir()
	writeTree(t, base, map[string]string{"a.go": "package a\n"})
	writeTree(t, cand, map[string]string{"a.go": "package a\n" + strings.Repeat("// padding line\n", 400)})
	_, err := DiffTreesToPatch(context.Background(), base, cand, 1024)
	if !errors.Is(err, ErrCarriedPatchTooLarge) {
		t.Fatalf("oversize = %v, want ErrCarriedPatchTooLarge", err)
	}
	if !strings.Contains(err.Error(), "1024") {
		t.Fatalf("refusal %q does not name the bound", err)
	}
	if _, err := DiffTreesToPatch(context.Background(), base, cand, CarriedPatchMaxBytes); err != nil {
		t.Fatalf("under the real bound: %v", err)
	}
}

func TestDiffTreesToPatchCarriesSourceOnly(t *testing.T) {
	requireGit(t)
	base, cand := t.TempDir(), t.TempDir()
	writeTree(t, base, map[string]string{"a.go": "package a\n"})
	writeTree(t, cand, map[string]string{
		"a.go":                                "package a\n// edited\n",
		"frontend/node_modules/left-pad/i.js": "module.exports = 1\n",
		".git/config":                         "[core]\n",
		"bin/tool":                            "\x00\x01\x02binary",
		"frontend/.cache/x":                   "cache\n",
	})
	if err := os.Symlink("../../etc/passwd", filepath.Join(cand, "link")); err != nil {
		t.Fatal(err)
	}
	got, err := DiffTreesToPatch(context.Background(), base, cand, CarriedPatchMaxBytes)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got.Files, ",") != "a.go" {
		t.Fatalf("files = %v, want only a.go", got.Files)
	}
	for _, leak := range []string{"node_modules", ".git/config", "binary", "passwd", ".cache"} {
		if strings.Contains(string(got.Patch), leak) {
			t.Fatalf("patch carries %q", leak)
		}
	}
	omitted := strings.Join(got.Omitted, "\n")
	for _, want := range []string{"node_modules", ".git", "bin/tool", "link", ".cache"} {
		if !strings.Contains(omitted, want) {
			t.Fatalf("omitted list %q does not name %q", omitted, want)
		}
	}
}

func TestCarriedPatchPathsRefusesOutsideTheSourceTree(t *testing.T) {
	ok := "diff --git a/x/y.go b/x/y.go\n--- a/x/y.go\n+++ b/x/y.go\n@@ -1 +1 @@\n-a\n+b\n"
	if paths, err := CarriedPatchPaths([]byte(ok)); err != nil || len(paths) != 1 || paths[0] != "x/y.go" {
		t.Fatalf("a source path = %v, %v", paths, err)
	}
	for _, bad := range []string{"../x", "/etc/x", ".git/config", "a/../../x", "x/.git/hooks/pre-commit"} {
		patch := "diff --git a/" + bad + " b/" + bad + "\n--- a/" + bad + "\n+++ b/" + bad + "\n@@ -1 +1 @@\n-a\n+b\n"
		if _, err := CarriedPatchPaths([]byte(patch)); !errors.Is(err, ErrCarriedPatchOutsideSource) {
			t.Fatalf("path %q = %v, want ErrCarriedPatchOutsideSource", bad, err)
		}
	}
}

func TestCheckPatchAppliesToMovedBase(t *testing.T) {
	requireGit(t)
	base, cand := t.TempDir(), t.TempDir()
	writeTree(t, base, map[string]string{"a.go": "package a\n\nfunc A() {}\n", "other.go": "package o\n"})
	writeTree(t, cand, map[string]string{"a.go": "package a\n\nfunc A() { println(1) }\n", "other.go": "package o\n"})
	got, err := DiffTreesToPatch(context.Background(), base, cand, CarriedPatchMaxBytes)
	if err != nil {
		t.Fatal(err)
	}
	// An unrelated file changed in the moved base: the patch still applies.
	unrelated := t.TempDir()
	writeTree(t, unrelated, map[string]string{"a.go": "package a\n\nfunc A() {}\n", "other.go": "package o\n// moved\n"})
	if err := CheckPatchAppliesToTree(context.Background(), got.Patch, got.Files, unrelated); err != nil {
		t.Fatalf("unrelated base change refused the patch: %v", err)
	}
	// The file the patch touches changed in the moved base: refused, naming it.
	conflict := t.TempDir()
	writeTree(t, conflict, map[string]string{"a.go": "package a\n\nfunc A() { return }\n", "other.go": "package o\n"})
	err = CheckPatchAppliesToTree(context.Background(), got.Patch, got.Files, conflict)
	if err == nil || !strings.Contains(err.Error(), "a.go") {
		t.Fatalf("conflicting base = %v, want a refusal naming a.go", err)
	}
}

// Mode 12 (iteration, 2026-10-25): a patch over several files that conflicts
// in one of them on a moved base names exactly the conflicting paths, so the
// engineer can redo only those; a file that still applies is not named as a
// conflict. The patch carries no blob ids, so a 3-way merge has no preimage
// to merge from and is not attempted.
func TestCheckPatchAppliesNamesOnlyTheConflictingPaths(t *testing.T) {
	requireGit(t)
	base, cand := t.TempDir(), t.TempDir()
	writeTree(t, base, map[string]string{"a.go": "package a\n\nfunc A() {}\n", "b.go": "package b\n\nfunc B() {}\n", "c.go": "package c\n"})
	writeTree(t, cand, map[string]string{"a.go": "package a\n\nfunc A() { println(1) }\n", "b.go": "package b\n\nfunc B() { println(2) }\n", "c.go": "package c\n\nfunc C() {}\n"})
	got, err := DiffTreesToPatch(context.Background(), base, cand, CarriedPatchMaxBytes)
	if err != nil {
		t.Fatal(err)
	}
	moved := t.TempDir()
	writeTree(t, moved, map[string]string{"a.go": "package a\n\nfunc A() { return }\n", "b.go": "package b\n\nfunc B() {}\n", "c.go": "package c\n// moved\n"})
	err = CheckPatchAppliesToTree(context.Background(), got.Patch, got.Files, moved)
	var conflict *CarriedPatchConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("conflicting base = %v (%T), want a *CarriedPatchConflictError", err, err)
	}
	if strings.Join(conflict.Paths, ",") != "a.go,c.go" {
		t.Fatalf("conflicting paths = %v, want exactly a.go and c.go", conflict.Paths)
	}
	if strings.Join(conflict.Clean, ",") != "b.go" {
		t.Fatalf("clean paths = %v, want b.go", conflict.Clean)
	}
	if !strings.Contains(err.Error(), "a.go, c.go") || strings.Contains(err.Error(), "conflict in b.go") {
		t.Fatalf("refusal %q does not name exactly the conflicting paths", err)
	}
}

func TestDiffTreesToPatchLeavesOutSecretNamedAndMaskFiles(t *testing.T) {
	requireGit(t)
	base, cand := t.TempDir(), t.TempDir()
	writeTree(t, base, map[string]string{
		"a.go":                    "package a\n",
		".npmrc":                  "registry=https://example.invalid/\n",
		"frontend/src/emptied.js": "export const x = 1\n",
	})
	writeTree(t, cand, map[string]string{
		"a.go":                     "package a\n// edited\n",
		".npmrc":                   "registry=https://example.invalid/\n//example.invalid/:_authToken=npm-secret\n",
		".env":                     "",
		".env.local":               "TOKEN=env-secret\n",
		"frontend/.ENV.production": "KEY=upper-secret\n",
		"certs/server.pem":         "pem-secret\n",
		"package.json":             "",
		"bunfig.toml":              "",
		"frontend/src/emptied.js":  "",
	})
	got, err := DiffTreesToPatch(context.Background(), base, cand, CarriedPatchMaxBytes)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got.Files, ",") != "a.go,frontend/src/emptied.js" {
		t.Fatalf("files = %v, want a.go and the emptied file only (modes 8-10)", got.Files)
	}
	for _, leak := range []string{"secret", ".env", ".npmrc", "server.pem", "package.json", "bunfig.toml"} {
		if strings.Contains(string(got.Patch), leak) {
			t.Fatalf("patch carries %q (modes 8-9)", leak)
		}
	}
	omitted := strings.Join(got.Omitted, "\n")
	for _, want := range []string{
		".env (secret-bearing path)", ".env.local (secret-bearing path)", "frontend/.ENV.production (secret-bearing path)",
		".npmrc (secret-bearing path)", "certs/server.pem (secret-bearing path)",
		"package.json (new empty file)", "bunfig.toml (new empty file)",
	} {
		if !strings.Contains(omitted, want) {
			t.Fatalf("omitted list %q does not name %q", omitted, want)
		}
	}
	if err := CheckPatchAppliesToTree(context.Background(), got.Patch, got.Files, base); err != nil {
		t.Fatalf("the filtered patch must still apply to its base: %v", err)
	}
}

func TestDiffTreesToPatchOfOnlyMasksIsEmpty(t *testing.T) {
	requireGit(t)
	base, cand := t.TempDir(), t.TempDir()
	writeTree(t, base, map[string]string{"a.go": "package a\n"})
	writeTree(t, cand, map[string]string{"a.go": "package a\n", ".env": "", ".yarnrc.yml": "", "yarn.lock": ""})
	got, err := DiffTreesToPatch(context.Background(), base, cand, CarriedPatchMaxBytes)
	if !errors.Is(err, ErrCarriedPatchEmpty) {
		t.Fatalf("err = %v, want ErrCarriedPatchEmpty when only masks changed", err)
	}
	if len(got.Omitted) != 3 {
		t.Fatalf("omitted = %v, want the three masks named", got.Omitted)
	}
}

func TestSecretBearingPathIsTheReleaseRule(t *testing.T) {
	for path, want := range map[string]bool{
		".env": true, "a/.env.local": true, "frontend/.ENV.production": true, ".npmrc": true, ".netrc": true,
		"x/credentials.json": true, "auth.json": true, "home/id_rsa": true, "id_ed25519": true,
		"certs/a.PEM": true, "tls.key": true, "a.p12": true, "b.pfx": true, ".env/inner.txt": true,
		"workspace/platform/frontend/src/App.svelte": false, ".envrc": false, "env.go": false,
		"keys.go": false, "id_rsa.pub": false, "frontend/package.json": false,
	} {
		if got := secretBearingPath(path); got != want {
			t.Errorf("secretBearingPath(%q) = %v, want %v", path, got, want)
		}
	}
}
