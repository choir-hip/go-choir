package sourceproj

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Three-way containment (docs/problems/promoted-stack-stays-base-moved-2026-10-26.md).
// For each path the patch modifies, the change base->after is merged into the
// booted blob; the path is contained only when that merge is clean and leaves
// the booted blob byte for byte, with the after-image's mode. Every case below
// except the first two is a "not contained" pin: the widening must never reach
// them.

// containedAt is Contained for a patch authored against base, judged against
// booted, with the patched tree computed the way reconcile computes it.
func containedAt(t *testing.T, repo, base string, patch []byte, booted string) (bool, error) {
	t.Helper()
	ctx := context.Background()
	tree, err := patchedTree(ctx, repo, base, patch, false)
	if err != nil {
		t.Fatalf("patch does not apply to its base: %v", err)
	}
	return Contained(ctx, repo, booted, base, patch, tree)
}

// insertAfter puts block after line n of rel.
func insertAfter(t *testing.T, root, rel string, n int, block string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(rel))
	raw, err := os.ReadFile(full)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.SplitAfter(string(raw), "\n")
	out := strings.Join(lines[:n], "") + block + strings.Join(lines[n:], "")
	if err := os.WriteFile(full, []byte(out), 0o644); err != nil {
		t.Fatal(err)
	}
}

// The evidence shape (Desktop.svelte, train 5 then round 4): the patch adds
// three lines to a.txt and edits b.txt. Main edited a.txt elsewhere before the
// promotion, then took the patch verbatim. b.txt is byte-identical to the
// after-image; a.txt is not, but it carries the patch's change.
func evidenceEdit(t *testing.T, r string) {
	insertAfter(t, r, "frontend/src/a.txt", 20, "resize anchor 1\nresize anchor 2\nresize anchor 3\n")
	editLine(t, r, "frontend/src/b.txt", 9, "b passes the anchor")
}

func TestThreeWayMainEditedTheSameFileElsewhereIsContained(t *testing.T) {
	repo, base := baseRepo(t)
	patch := patchFrom(t, repo, base, func(r string) { evidenceEdit(t, r) })
	booted := platformMoved(t, repo, base, func(r string) {
		editLine(t, r, "frontend/src/a.txt", 35, "the continue chip, from another lane")
		evidenceEdit(t, r)
	})
	if show(t, repo, booted, "frontend/src/a.txt") == show(t, repo, base, "frontend/src/a.txt") {
		t.Fatal("setup: booted must differ from base in a.txt")
	}
	ok, err := containedAt(t, repo, base, patch, booted)
	if err != nil || !ok {
		t.Fatalf("the patch's change is in booted, beside main's own edit: contained must be true, got %v %v", ok, err)
	}
	// The same, with main's edit just above the hunk's context but not in it.
	near := platformMoved(t, repo, base, func(r string) {
		editLine(t, r, "frontend/src/a.txt", 14, "an edit outside the hunk's context")
		evidenceEdit(t, r)
	})
	if ok, err := containedAt(t, repo, base, patch, near); err != nil || !ok {
		t.Fatalf("an edit outside the hunk is not a conflict: %v %v", ok, err)
	}
}

// A file with two identical sections. The patch inserts NEW into the second;
// booted inserted the very same block, with the very same context, into the
// first. Booted lacks the patch's change. A reverse-apply of the patch onto
// booted succeeds (git apply finds the hunk's after-image and context at the
// first section), which is why reverse-apply was never used. The three-way
// merge aligns base with the patch at the second section, applies the insert
// there, and the result differs from booted: not contained.
func TestThreeWayDuplicatedBlockIsNotContainedThoughReverseApplyIsFooled(t *testing.T) {
	repo, root := baseRepo(t)
	section := "begin section\n" + numbered("s", 6) + "end section\n"
	base := platformMoved(t, repo, root, func(r string) {
		write(t, r, "frontend/src/dup.txt", numbered("head", 4)+section+numbered("middle", 8)+section+numbered("tail", 4))
	})
	// Line numbers: head 1-4, first section 5-12 (s lines 6-11), middle 13-20,
	// second section 21-28 (s lines 22-27), tail 29-32.
	patch := patchFrom(t, repo, base, func(r string) { insertAfter(t, r, "frontend/src/dup.txt", 24, "NEW BLOCK\n") })
	booted := platformMoved(t, repo, base, func(r string) { insertAfter(t, r, "frontend/src/dup.txt", 8, "NEW BLOCK\n") })

	ctx := context.Background()
	if _, err := patchedTree(ctx, repo, booted, patch, true); err != nil {
		t.Fatalf("setup: the reverse-apply must succeed on booted (that is the trap), got %v", err)
	}
	ok, err := containedAt(t, repo, base, patch, booted)
	if err != nil || ok {
		t.Fatalf("a duplicated block retired a patch that is not in booted: %v %v", ok, err)
	}
}

func TestThreeWayPartlyUpstreamedIsNotContained(t *testing.T) {
	repo, base := baseRepo(t)
	patch := patchFrom(t, repo, base, func(r string) {
		editLine(t, r, "frontend/src/a.txt", 5, "first hunk")
		editLine(t, r, "frontend/src/a.txt", 30, "second hunk")
	})
	booted := platformMoved(t, repo, base, func(r string) {
		editLine(t, r, "frontend/src/a.txt", 5, "first hunk")
		editLine(t, r, "frontend/src/a.txt", 17, "main's own edit")
	})
	if ok, err := containedAt(t, repo, base, patch, booted); err != nil || ok {
		t.Fatalf("one hunk of two is in booted: not contained, got %v %v", ok, err)
	}
}

func TestThreeWayAdjacentEditConflictsAndIsNotContained(t *testing.T) {
	repo, base := baseRepo(t)
	patch := patchFrom(t, repo, base, func(r string) { editLine(t, r, "frontend/src/a.txt", 5, "the patch's line") })
	cases := map[string]func(r string){
		// The false negative the design accepts: the change is there, but main
		// also edited the next line, and diff3 calls that a conflict.
		"took the change and edited the next line": func(r string) {
			editLine(t, r, "frontend/src/a.txt", 5, "the patch's line")
			editLine(t, r, "frontend/src/a.txt", 6, "an adjacent edit")
		},
		"edited the next line only": func(r string) { editLine(t, r, "frontend/src/a.txt", 6, "an adjacent edit") },
		"the same line, otherwise":  func(r string) { editLine(t, r, "frontend/src/a.txt", 5, "main's line") },
	}
	for name, edit := range cases {
		booted := platformMoved(t, repo, base, edit)
		if ok, err := containedAt(t, repo, base, patch, booted); err != nil || ok {
			t.Fatalf("%s: not contained, got %v %v", name, ok, err)
		}
	}
}

func TestThreeWayModeDiffersIsNotContained(t *testing.T) {
	repo, base := baseRepo(t)
	patch := patchFrom(t, repo, base, func(r string) { evidenceEdit(t, r) })
	booted := platformMoved(t, repo, base, func(r string) {
		editLine(t, r, "frontend/src/a.txt", 35, "the continue chip, from another lane")
		evidenceEdit(t, r)
		if err := os.Chmod(filepath.Join(r, "frontend/src/a.txt"), 0o755); err != nil {
			t.Fatal(err)
		}
	})
	if mode := strings.Fields(tgit(t, repo, "ls-tree", booted, "--", "frontend/src/a.txt"))[0]; mode != "100755" {
		t.Fatalf("setup: booted a.txt mode %s, want 100755", mode)
	}
	if ok, err := containedAt(t, repo, base, patch, booted); err != nil || ok {
		t.Fatalf("the content merges but the mode is not the after-image's: not contained, got %v %v", ok, err)
	}
}

// patchFromBinary is patchFrom with binary hunks.
func patchFromBinary(t *testing.T, repo, rev string, edit func(root string)) []byte {
	t.Helper()
	scratch := t.TempDir()
	tgit(t, scratch, "clone", "-q", "--no-checkout", repo, ".")
	tgit(t, scratch, "checkout", "-q", rev)
	edit(scratch)
	tgit(t, scratch, "add", "-A")
	// Untrimmed: a binary hunk ends in a blank line that tgit would strip.
	out, err := exec.Command("git", "-C", scratch, "diff", "--cached", "--binary", "--no-color").Output()
	if err != nil {
		t.Fatalf("git diff --binary: %v", err)
	}
	return out
}

// A binary file is judged by the exact test only. The file is line-shaped, so
// a line merge would call it contained (the change is there beside main's
// own); git sees the NUL bytes and calls it binary, and binary is exact.
func TestThreeWayBinaryIsExactOnly(t *testing.T) {
	repo, root := baseRepo(t)
	bin := func(n int) string { return strings.ReplaceAll(numbered("bin", n), " line", "\x00line") }
	base := platformMoved(t, repo, root, func(r string) { write(t, r, "frontend/src/blob.bin", bin(40)) })
	patch := patchFromBinary(t, repo, base, func(r string) { editLine(t, r, "frontend/src/blob.bin", 5, "bin\x00changed") })
	booted := platformMoved(t, repo, base, func(r string) {
		editLine(t, r, "frontend/src/blob.bin", 5, "bin\x00changed")
		editLine(t, r, "frontend/src/blob.bin", 35, "bin\x00main's own")
	})
	if ok, err := containedAt(t, repo, base, patch, booted); err != nil || ok {
		t.Fatalf("a differing binary file is not contained (and is not an error): %v %v", ok, err)
	}
	// Exact still holds for binary.
	same := platformMoved(t, repo, base, func(r string) { editLine(t, r, "frontend/src/blob.bin", 5, "bin\x00changed") })
	if ok, err := containedAt(t, repo, base, patch, same); err != nil || !ok {
		t.Fatalf("an identical binary file is contained: %v %v", ok, err)
	}
}

// The binary rule reads the bytes, not the repository's attributes: a
// gitattributes "-diff" makes git's numstat report a binary blob as text, and
// that must not route a binary file through the line merge.
func TestThreeWayBinaryIgnoresAttributes(t *testing.T) {
	repo, root := baseRepo(t)
	if err := os.WriteFile(filepath.Join(repo, ".git", "info", "attributes"), []byte("* -diff\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bin := func(n int) string { return strings.ReplaceAll(numbered("bin", n), " line", "\x00line") }
	base := platformMoved(t, repo, root, func(r string) { write(t, r, "frontend/src/blob.bin", bin(40)) })
	patch := patchFromBinary(t, repo, base, func(r string) { editLine(t, r, "frontend/src/blob.bin", 5, "bin\x00changed") })
	booted := platformMoved(t, repo, base, func(r string) {
		editLine(t, r, "frontend/src/blob.bin", 5, "bin\x00changed")
		editLine(t, r, "frontend/src/blob.bin", 35, "bin\x00main's own")
	})
	if ok, err := containedAt(t, repo, base, patch, booted); err != nil || ok {
		t.Fatalf("a binary file marked -diff is still exact only: %v %v", ok, err)
	}
}

// Without the base blob there is no three-way merge; the test falls back to
// exact, never to anything looser, and that is an answer, not an error.
func TestThreeWayBaseBlobMissingIsExactOnly(t *testing.T) {
	repo, base := baseRepo(t)
	ctx := context.Background()
	patch := patchFrom(t, repo, base, func(r string) { evidenceEdit(t, r) })
	tree, err := patchedTree(ctx, repo, base, patch, false)
	if err != nil {
		t.Fatal(err)
	}
	merged := platformMoved(t, repo, base, func(r string) {
		editLine(t, r, "frontend/src/a.txt", 35, "the continue chip, from another lane")
		evidenceEdit(t, r)
	})
	exact := platformMoved(t, repo, base, func(r string) { evidenceEdit(t, r) })

	blob := tgit(t, repo, "rev-parse", base+":frontend/src/a.txt")
	loose := filepath.Join(repo, ".git", "objects", blob[:2], blob[2:])
	if err := os.Remove(loose); err != nil {
		t.Fatalf("setup: the base blob must be a loose object: %v", err)
	}
	if ok, err := Contained(ctx, repo, merged, base, patch, tree); err != nil || ok {
		t.Fatalf("no base blob, booted differs from the after-image: not contained, got %v %v", ok, err)
	}
	if ok, err := Contained(ctx, repo, exact, base, patch, tree); err != nil || !ok {
		t.Fatalf("no base blob, booted is the after-image: contained, got %v %v", ok, err)
	}
}
