//go:build linux

package capsule

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The manifest diff reports directories as changes. A unified diff cannot
// express a directory, and reading one as a file fails the whole freeze
// (problems/capsule-freeze-fails-on-new-source-directory-2026-10-09.md).
func TestEmitSourcePatchSkipsDirectoryChanges(t *testing.T) {
	root := t.TempDir()
	merged := filepath.Join(root, "merged")
	lower := filepath.Join(root, "source-lower", "workspace", "platform")
	mustMkdir := func(p string) {
		t.Helper()
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	mustWrite := func(p, body string) {
		t.Helper()
		mustMkdir(filepath.Dir(p))
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// Lower: docs/ exists with a file; old/ is a directory the change
	// replaces with a file; gone/ is a directory the change deletes.
	mustWrite(filepath.Join(lower, "docs", "README.md"), "readme\n")
	mustMkdir(filepath.Join(lower, "old"))
	mustMkdir(filepath.Join(lower, "gone"))
	// Merged: an existing directory gains a file, a new directory holds a
	// new file, old/ became a file.
	mergedPlatform := filepath.Join(merged, "workspace", "platform")
	mustWrite(filepath.Join(mergedPlatform, "docs", "README.md"), "readme\n")
	mustWrite(filepath.Join(mergedPlatform, "docs", "evidence", "m11.md"), "evidence\n")
	mustWrite(filepath.Join(mergedPlatform, "old"), "now a file\n")

	dir := os.ModeDir | 0o755
	changes := []FileChange{
		{Path: "workspace/platform/docs", Kind: ChangeModified, Mode: dir},
		{Path: "workspace/platform/docs/evidence", Kind: ChangeAdded, Mode: dir},
		{Path: "workspace/platform/docs/evidence/m11.md", Kind: ChangeAdded, Mode: 0o644},
		{Path: "workspace/platform/gone", Kind: ChangeDeleted, Mode: dir},
		{Path: "workspace/platform/old", Kind: ChangeModified, Mode: 0o644},
	}
	caps := &Capsule{MergedDir: merged}
	patchPath, sum, _, err := (&Executor{}).emitSourcePatch(caps, changes, t.TempDir())
	if err != nil {
		t.Fatalf("emitSourcePatch: %v", err)
	}
	if patchPath == "" || sum == "" {
		t.Fatalf("expected a patch, got path=%q sha=%q", patchPath, sum)
	}
	body, err := os.ReadFile(patchPath)
	if err != nil {
		t.Fatal(err)
	}
	patch := string(body)
	for _, want := range []string{
		"--- /dev/null\n+++ b/docs/evidence/m11.md",
		"+evidence",
		"--- /dev/null\n+++ b/old",
		"+now a file",
	} {
		if !strings.Contains(patch, want) {
			t.Fatalf("patch missing %q:\n%s", want, patch)
		}
	}
	for _, unwanted := range []string{"b/docs\n", "b/docs/evidence\n", "gone"} {
		if strings.Contains(patch, unwanted) {
			t.Fatalf("patch carries directory entry %q:\n%s", unwanted, patch)
		}
	}
}
