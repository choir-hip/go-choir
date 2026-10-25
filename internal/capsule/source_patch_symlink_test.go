package capsule

// A source patch never reads through a symbolic link (bunch221 review,
// DeepSeek F1). The candidate bytes come from the merged tree a cell can
// write, so a symlink there would carry bytes from outside the pinned
// source into source.patch, the bundle draft, the verifiers' mounts and
// the host builder.
//
// Ways this can fail, written before the fix:
//
//	S1. An added or modified source file that is itself a symlink to a file
//	    outside the tree: the renderer reads the target's bytes into a
//	    regular-file hunk.
//	S2. The change's recorded mode says regular but the merged path is a
//	    symlink (the manifest is a snapshot, the tree is read after it):
//	    reading by path follows the link anyway.
//	S3. The merged path passes through a symlinked directory: every
//	    component, not just the last, must be refused.
//	S4. The base (pinned source) side of a modification is a symlink, or
//	    passes through one: the base read follows it into the old side of
//	    the hunk.
//	S5. The refusal itself quotes the bytes it refused.
//
// This file runs on every platform: on Linux it exercises the openat2
// reader, elsewhere the Lstat-walk fallback.

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const outsideSecret = "OUTSIDE-THE-PINNED-SOURCE-7f3a9c\n"

func TestSourcePatchRefusesToReadThroughSymlinks(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T, f *overlayFixture, outsideFile, outsideDir string) []FileChange
	}{
		{"S1 added symlink to an outside file", func(t *testing.T, f *overlayFixture, outsideFile, _ string) []FileChange {
			mustSymlink(t, outsideFile, filepath.Join(f.merged, "workspace", "platform", "leak.go"))
			return []FileChange{{Path: platformRoot + "leak.go", Kind: ChangeAdded, Mode: os.ModeSymlink | 0o777}}
		}},
		{"S1 modified file replaced by a symlink", func(t *testing.T, f *overlayFixture, outsideFile, _ string) []FileChange {
			putFile(t, f.lower, "a.go", "base\n", 0o644)
			mustSymlink(t, outsideFile, filepath.Join(f.merged, "workspace", "platform", "a.go"))
			return []FileChange{{Path: platformRoot + "a.go", Kind: ChangeModified, Mode: os.ModeSymlink | 0o777}}
		}},
		{"S2 mode says regular, merged path is a symlink", func(t *testing.T, f *overlayFixture, outsideFile, _ string) []FileChange {
			mustSymlink(t, outsideFile, filepath.Join(f.merged, "workspace", "platform", "leak.go"))
			return []FileChange{{Path: platformRoot + "leak.go", Kind: ChangeAdded, Mode: 0o644}}
		}},
		{"S3 merged path passes through a symlinked directory", func(t *testing.T, f *overlayFixture, _, outsideDir string) []FileChange {
			mustSymlink(t, outsideDir, filepath.Join(f.merged, "workspace", "platform", "d"))
			return []FileChange{{Path: platformRoot + "d/secret.txt", Kind: ChangeAdded, Mode: 0o644}}
		}},
		{"S4 base of a modification is a symlink", func(t *testing.T, f *overlayFixture, outsideFile, _ string) []FileChange {
			mustSymlink(t, outsideFile, filepath.Join(f.lower, "CLAUDE.md"))
			putFile(t, filepath.Join(f.merged, "workspace", "platform"), "CLAUDE.md", "now a regular file\n", 0o644)
			return []FileChange{{Path: platformRoot + "CLAUDE.md", Kind: ChangeModified, Mode: 0o644}}
		}},
		{"S4 base passes through a symlinked directory", func(t *testing.T, f *overlayFixture, _, outsideDir string) []FileChange {
			mustSymlink(t, outsideDir, filepath.Join(f.lower, "d"))
			putFile(t, filepath.Join(f.merged, "workspace", "platform"), "d/secret.txt", "candidate\n", 0o644)
			return []FileChange{{Path: platformRoot + "d/secret.txt", Kind: ChangeModified, Mode: 0o644}}
		}},
		{"S4 deleted symlink", func(t *testing.T, f *overlayFixture, outsideFile, _ string) []FileChange {
			mustSymlink(t, outsideFile, filepath.Join(f.lower, "link"))
			return []FileChange{whiteoutChange("link")}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newOverlayFixture(t)
			outsideDir := t.TempDir()
			outsideFile := filepath.Join(outsideDir, "secret.txt")
			if err := os.WriteFile(outsideFile, []byte(outsideSecret), 0o644); err != nil {
				t.Fatal(err)
			}
			changes := tc.setup(t, f, outsideFile, outsideDir)
			files, err := collectSourcePatchFiles(f.merged, changes)
			if err == nil {
				patch, _ := renderSourcePatch(files)
				if strings.Contains(string(patch), strings.TrimSpace(outsideSecret)) {
					t.Fatalf("the source patch carries bytes from outside the pinned source:\n%s", patch)
				}
				t.Fatalf("collectSourcePatchFiles accepted a symlinked source change: %+v", files)
			}
			if !errors.Is(err, errSourcePatchSymlink) {
				t.Fatalf("err = %v, want a refusal naming errSourcePatchSymlink", err)
			}
			if files != nil {
				t.Fatalf("a refused collection returned files: %+v", files)
			}
			if strings.Contains(err.Error(), strings.TrimSpace(outsideSecret)) {
				t.Fatalf("the refusal quotes the outside bytes (S5): %v", err)
			}
		})
	}
}

// The readers still accept what they must: a regular candidate file, a
// base directory replaced by a file, and an absent base.
func TestSourcePatchReaderAcceptsRegularFiles(t *testing.T) {
	f := newOverlayFixture(t)
	f.lowerFile(t, "a.go", "old\n")
	f.upperFile(t, "a.go", "new\n")
	if err := os.MkdirAll(filepath.Join(f.lower, "was-dir", "x"), 0o755); err != nil {
		t.Fatal(err)
	}
	putFile(t, filepath.Join(f.merged, "workspace", "platform"), "was-dir", "file now\n", 0o644)
	putFile(t, filepath.Join(f.merged, "workspace", "platform"), "fresh/new.go", "fresh\n", 0o644)
	// A lower file replaced by a directory: the base has a file where the
	// candidate's path needs a directory (ENOTDIR), so the base is absent.
	putFile(t, f.lower, "was-file", "file then\n", 0o644)
	putFile(t, filepath.Join(f.merged, "workspace", "platform"), "was-file/inner.go", "inner\n", 0o644)
	files, err := collectSourcePatchFiles(f.merged, []FileChange{
		{Path: platformRoot + "a.go", Kind: ChangeModified, Mode: 0o644},
		{Path: platformRoot + "was-dir", Kind: ChangeModified, Mode: 0o644},
		{Path: platformRoot + "fresh/new.go", Kind: ChangeAdded, Mode: 0o644},
		{Path: platformRoot + "was-file/inner.go", Kind: ChangeAdded, Mode: 0o644},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 4 {
		t.Fatalf("files = %+v", files)
	}
	if string(files[0].Old) != "old\n" || string(files[0].New) != "new\n" || !files[0].OldExists {
		t.Fatalf("a.go = %+v", files[0])
	}
	if files[1].OldExists || string(files[1].New) != "file now\n" {
		t.Fatalf("was-dir = %+v, want no base and the new file", files[1])
	}
	if files[2].OldExists || string(files[2].New) != "fresh\n" {
		t.Fatalf("fresh/new.go = %+v", files[2])
	}
	if files[3].OldExists || string(files[3].New) != "inner\n" {
		t.Fatalf("was-file/inner.go = %+v, want no base and the new file", files[3])
	}
}

func mustSymlink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	_ = os.RemoveAll(link)
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
}
