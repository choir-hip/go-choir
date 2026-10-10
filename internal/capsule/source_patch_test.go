package capsule

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Ways the capsule source patch can fail the builder (M11 rerun 10,
// docs/problems/selfdev-verifier-rejects-frozen-bundle-2026-10-10.md):
//
//  1. An added file is diffed against a phantom empty line, so its hunk
//     reads "@@ -1 +1,N @@" with a stray context line and git apply
//     rejects it ("depends on old contents").
//  2. A hunk touching the end of a file carries a phantom trailing
//     context line that the base file does not have.
//  3. A file without a final newline is given one, so the applied bytes
//     differ from the candidate's.
//  4. An empty added file produces no hunk and vanishes from the patch.
//  5. A deletion leaves the file behind, or deletes the wrong bytes.
//  6. A patch that does not reproduce the candidate bytes is frozen
//     anyway: the round-trip check must refuse it before any verifier.
//
// Every case is proven through real `git apply`, the builder's applier
// (internal/builder/util.go patchedFlakeRef).

func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
}

func applyWithGit(t *testing.T, patch []byte, files []sourcePatchFile) string {
	t.Helper()
	dir := t.TempDir()
	if out, err := exec.Command("git", "-C", dir, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	for _, f := range files {
		if !f.OldExists {
			continue
		}
		p := filepath.Join(dir, filepath.FromSlash(f.Rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, f.Old, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	patchPath := filepath.Join(t.TempDir(), "source.patch")
	if err := os.WriteFile(patchPath, patch, 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "apply", patchPath)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git apply rejected the patch: %v: %s\n--- patch ---\n%s", err, out, patch)
	}
	return dir
}

func TestSourcePatchRoundTripsThroughGitApply(t *testing.T) {
	requireGit(t)
	long := func(n int, tail string) string {
		var b strings.Builder
		for i := 0; i < n; i++ {
			b.WriteString("line ")
			b.WriteString(strings.Repeat("x", i%7))
			b.WriteString("\n")
		}
		return b.String() + tail
	}
	cases := []struct {
		name  string
		files []sourcePatchFile
	}{
		{"added file", []sourcePatchFile{{Rel: "internal/selfdevevidence/episode.go", New: []byte("package selfdevevidence\n\nconst X = 1\n"), NewExists: true}}},
		{"added file in new directory", []sourcePatchFile{{Rel: "docs/evidence/m11/episode.json", New: []byte("{\n  \"ok\": true\n}\n"), NewExists: true}}},
		{"empty added file", []sourcePatchFile{{Rel: "docs/evidence/.keep", New: []byte{}, NewExists: true}}},
		{"added file without final newline", []sourcePatchFile{{Rel: "a.txt", New: []byte("one\ntwo"), NewExists: true}}},
		{"edit at end of file", []sourcePatchFile{{Rel: "b.go", Old: []byte(long(20, "")), OldExists: true, New: []byte(long(20, "appended\n")), NewExists: true}}},
		{"drop final newline", []sourcePatchFile{{Rel: "c.txt", Old: []byte("one\ntwo\n"), OldExists: true, New: []byte("one\ntwo"), NewExists: true}}},
		{"add final newline", []sourcePatchFile{{Rel: "d.txt", Old: []byte("one\ntwo"), OldExists: true, New: []byte("one\ntwo\n"), NewExists: true}}},
		{"multi-hunk modify", []sourcePatchFile{{Rel: "e.go", Old: []byte(long(60, "")), OldExists: true, New: []byte(strings.Replace(strings.Replace(long(60, ""), "line \n", "first edit\n", 1), "line xxxxxx\n", "second edit\n", 1)), NewExists: true}}},
		{"deleted file", []sourcePatchFile{{Rel: "f.txt", Old: []byte("gone\n"), OldExists: true}}},
		{"deleted empty file", []sourcePatchFile{{Rel: "g.txt", Old: []byte{}, OldExists: true}}},
		{"several files at once", []sourcePatchFile{
			{Rel: "docs/evidence/m11.json", New: []byte("{}\n"), NewExists: true},
			{Rel: "internal/x/x.go", Old: []byte("package x\n"), OldExists: true, New: []byte("package x\n\nvar Y = 2\n"), NewExists: true},
			{Rel: "old.txt", Old: []byte("bye\n"), OldExists: true},
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			patch, err := renderSourcePatch(tc.files)
			if err != nil {
				t.Fatalf("renderSourcePatch: %v", err)
			}
			if len(patch) == 0 {
				t.Fatalf("empty patch for a real change")
			}
			if len(tc.files) == 1 && !tc.files[0].OldExists && strings.Contains(string(patch), "@@ -1 +1,") {
				t.Fatalf("creation hunk diffed against a phantom line:\n%s", patch)
			}
			dir := applyWithGit(t, patch, tc.files)
			for _, f := range tc.files {
				got, readErr := os.ReadFile(filepath.Join(dir, filepath.FromSlash(f.Rel)))
				if !f.NewExists {
					if !os.IsNotExist(readErr) {
						t.Fatalf("%s should be deleted, read err=%v", f.Rel, readErr)
					}
					continue
				}
				if readErr != nil {
					t.Fatalf("%s missing after apply: %v\n%s", f.Rel, readErr, patch)
				}
				if string(got) != string(f.New) {
					t.Fatalf("%s bytes differ after apply:\n got %q\nwant %q\n--- patch ---\n%s", f.Rel, got, f.New, patch)
				}
			}
			if err := verifySourcePatch(context.Background(), patch, tc.files); err != nil {
				t.Fatalf("verifySourcePatch refused a faithful patch: %v", err)
			}
		})
	}
}

func TestSourcePatchUnchangedFileEmitsNothing(t *testing.T) {
	patch, err := renderSourcePatch([]sourcePatchFile{{Rel: "same.txt", Old: []byte("x\n"), OldExists: true, New: []byte("x\n"), NewExists: true}})
	if err != nil {
		t.Fatal(err)
	}
	if len(patch) != 0 {
		t.Fatalf("unchanged file produced a patch:\n%s", patch)
	}
}

func TestVerifySourcePatchRefusesUnfaithfulPatch(t *testing.T) {
	requireGit(t)
	files := []sourcePatchFile{{Rel: "docs/evidence/m11.json", New: []byte("{}\n"), NewExists: true}}
	// The rerun-10 shape: a creation hunk against a phantom empty line.
	phantom := []byte("--- /dev/null\n+++ b/docs/evidence/m11.json\n@@ -1 +1,2 @@\n+{}\n \n")
	if err := verifySourcePatch(context.Background(), phantom, files); err == nil {
		t.Fatal("verifySourcePatch accepted the phantom-line creation hunk")
	}
	// A patch that applies but yields different bytes.
	wrong, err := renderSourcePatch([]sourcePatchFile{{Rel: "docs/evidence/m11.json", New: []byte("{ }\n"), NewExists: true}})
	if err != nil {
		t.Fatal(err)
	}
	if err := verifySourcePatch(context.Background(), wrong, files); err == nil || !strings.Contains(err.Error(), "docs/evidence/m11.json") {
		t.Fatalf("verifySourcePatch should name the mismatched file, got %v", err)
	}
}
