package sourceproj

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Patch identity (docs/problems/selfdev-blocked-behind-unperformed-rebase-2026-10-27.md,
// the correction): a stack promoted to main verbatim is upstreamed even when
// main then edits right beside it, which the three-way test calls a conflict.
// The answer is git's own: a non-merge commit in base..booted with the patch's
// stable patch-id, not undone by a later commit with the reverse patch-id.

// dropTail removes the last n lines of rel.
func dropTail(t *testing.T, root, rel string, n int) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(rel))
	raw, err := os.ReadFile(full)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.SplitAfter(string(raw), "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if err := os.WriteFile(full, []byte(strings.Join(lines[:len(lines)-n], "")), 0o644); err != nil {
		t.Fatal(err)
	}
}

// appendTo adds block at the end of rel.
func appendTo(t *testing.T, root, rel, block string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(rel))
	raw, err := os.ReadFile(full)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, append(raw, block...), 0o644); err != nil {
		t.Fatal(err)
	}
}

// round7 is the evidence shape: themes round 7 deleted the trailing block of
// app.css (and edited a second file); main took it verbatim, then appended the
// idle rule at the end of the same file.
func round7(t *testing.T, r string) {
	dropTail(t, r, "frontend/src/a.txt", 6)
	editLine(t, r, "frontend/src/b.txt", 9, "b without the drift")
}

const idleRule = "\n/* idle */\n:root[data-idle] * { animation-play-state: paused; }\n"

func TestPatchIDPromotedThenEditedAtTheSameEndIsContained(t *testing.T) {
	repo, base := baseRepo(t)
	patch := patchFrom(t, repo, base, func(r string) { round7(t, r) })
	promoted := platformMoved(t, repo, base, func(r string) { round7(t, r) })
	booted := platformMoved(t, repo, promoted, func(r string) { appendTo(t, r, "frontend/src/a.txt", idleRule) })
	if ok, err := containedAt(t, repo, base, patch, promoted); err != nil || !ok {
		t.Fatalf("setup: at the promotion itself the exact test holds: %v %v", ok, err)
	}
	ok, err := containedAt(t, repo, base, patch, booted)
	if err != nil || !ok {
		t.Fatalf("promoted verbatim, then main edited beside it: contained must be true, got %v %v", ok, err)
	}
}

func TestPatchIDPromotionInsideALargerCommitIsContained(t *testing.T) {
	repo, base := baseRepo(t)
	patch := patchFrom(t, repo, base, func(r string) { round7(t, r) })
	promoted := platformMoved(t, repo, base, func(r string) {
		round7(t, r)
		write(t, r, "internal/x.go", "package x\n\nfunc X() int { return 2 }\n")
	})
	booted := platformMoved(t, repo, promoted, func(r string) { appendTo(t, r, "frontend/src/a.txt", idleRule) })
	if ok, err := containedAt(t, repo, base, patch, booted); err != nil || !ok {
		t.Fatalf("the promotion also changed another path; on the patch's paths it is the patch: %v %v", ok, err)
	}
}

func TestPatchIDPromotedThenRevertedIsNotContained(t *testing.T) {
	repo, base := baseRepo(t)
	patch := patchFrom(t, repo, base, func(r string) { round7(t, r) })
	promoted := platformMoved(t, repo, base, func(r string) { round7(t, r) })
	reverted := platformMoved(t, repo, promoted, func(r string) {
		write(t, r, "frontend/src/a.txt", show(t, repo, base, "frontend/src/a.txt")+"\n")
		write(t, r, "frontend/src/b.txt", show(t, repo, base, "frontend/src/b.txt")+"\n")
	})
	if got, want := show(t, repo, reverted, "frontend/src/a.txt"), show(t, repo, base, "frontend/src/a.txt"); got != want {
		t.Fatal("setup: the revert must restore the base's a.txt")
	}
	booted := platformMoved(t, repo, reverted, func(r string) { appendTo(t, r, "frontend/src/a.txt", idleRule) })
	if ok, err := containedAt(t, repo, base, patch, booted); err != nil || ok {
		t.Fatalf("main reverted the promotion: the change is not in booted, got %v %v", ok, err)
	}
}

func TestPatchIDOutsideTheRangeIsNotContained(t *testing.T) {
	repo, base := baseRepo(t)
	patch := patchFrom(t, repo, base, func(r string) { round7(t, r) })
	// The change exists on a side branch the booted base does not descend from.
	_ = platformMoved(t, repo, base, func(r string) { round7(t, r) })
	booted := platformMoved(t, repo, base, func(r string) { appendTo(t, r, "frontend/src/a.txt", idleRule) })
	if ok, err := containedAt(t, repo, base, patch, booted); err != nil || ok {
		t.Fatalf("the promotion is on a branch booted does not contain: got %v %v", ok, err)
	}
}

func TestPatchIDBootedNotDescendedFromTheBaseFallsBack(t *testing.T) {
	repo, base := baseRepo(t)
	patch := patchFrom(t, repo, base, func(r string) { round7(t, r) })
	promoted := platformMoved(t, repo, base, func(r string) { round7(t, r) })
	edited := platformMoved(t, repo, promoted, func(r string) { appendTo(t, r, "frontend/src/a.txt", idleRule) })
	// The same tree as a parentless commit: no history joins it to the base.
	orphan := tgit(t, repo, "commit-tree", "-m", "orphan", edited+"^{tree}")
	if ok, err := containedAt(t, repo, base, patch, orphan); err != nil || ok {
		t.Fatalf("no history from the base to booted: the content answer stands (no), got %v %v", ok, err)
	}
}

// Review 353, F1: after the promoting commit, any later change that undoes
// part of the change (re-adds a line the patch removed, removes a line it
// added, deletes or re-modes the file) makes it not contained, whether or
// not the undo is an exact reverse; an unrelated edit elsewhere does not.
func TestPatchIDLaterPartialUndoIsNotContained(t *testing.T) {
	repo, base := baseRepo(t)
	patch := patchFrom(t, repo, base, func(r string) { round7(t, r) })
	promoted := platformMoved(t, repo, base, func(r string) { round7(t, r) })
	a := show(t, repo, base, "frontend/src/a.txt") + "\n"
	lines := strings.SplitAfter(a, "\n")
	cases := map[string]func(r string){
		"re-adds two of the six deleted lines": func(r string) {
			appendTo(t, r, "frontend/src/a.txt", lines[34]+lines[35])
		},
		"reverts and edits another line in the same commit": func(r string) {
			write(t, r, "frontend/src/a.txt", a)
			write(t, r, "frontend/src/b.txt", show(t, repo, base, "frontend/src/b.txt")+"\n")
			editLine(t, r, "frontend/src/a.txt", 2, "an unrelated line")
		},
		"deletes the file": func(r string) {
			if err := os.Remove(filepath.Join(r, "frontend/src/a.txt")); err != nil {
				t.Fatal(err)
			}
		},
		"removes the line the patch added": func(r string) {
			editLine(t, r, "frontend/src/b.txt", 9, "b line 09")
		},
	}
	for name, undo := range cases {
		booted := platformMoved(t, repo, promoted, undo)
		if name != "deletes the file" {
			booted = platformMoved(t, repo, booted, func(r string) { appendTo(t, r, "frontend/src/a.txt", idleRule) })
		}
		if ok, err := containedAt(t, repo, base, patch, booted); err != nil || ok {
			t.Fatalf("%s: the change is partly undone after the promotion: not contained, got %v %v", name, ok, err)
		}
	}
}

func TestPatchIDUnrelatedLaterEditsStayContained(t *testing.T) {
	repo, base := baseRepo(t)
	patch := patchFrom(t, repo, base, func(r string) { round7(t, r) })
	promoted := platformMoved(t, repo, base, func(r string) { round7(t, r) })
	booted := platformMoved(t, repo, promoted, func(r string) {
		editLine(t, r, "frontend/src/a.txt", 3, "main rewrote an unrelated line")
		editLine(t, r, "frontend/src/b.txt", 30, "and another, in the other file")
		appendTo(t, r, "frontend/src/a.txt", idleRule)
	})
	if ok, err := containedAt(t, repo, base, patch, booted); err != nil || !ok {
		t.Fatalf("main edited other lines after the promotion: still contained, got %v %v", ok, err)
	}
}

// A merge whose resolution drops the change: the merge commit is not walked
// for matches (--no-merges), but the later-change check reads trees, so it
// sees the drop.
func TestPatchIDMergeThatDropsTheChangeIsNotContained(t *testing.T) {
	repo, base := baseRepo(t)
	patch := patchFrom(t, repo, base, func(r string) { round7(t, r) })
	promoted := platformMoved(t, repo, base, func(r string) { round7(t, r) })
	side := platformMoved(t, repo, base, func(r string) { editLine(t, r, "frontend/src/b.txt", 30, "a side branch edit") })
	// The merge's tree is the side branch's: the promotion's change is gone.
	tree := tgit(t, repo, "rev-parse", side+"^{tree}")
	merge := tgit(t, repo, "commit-tree", "-p", promoted, "-p", side, "-m", "merge dropping the change", tree)
	booted := platformMoved(t, repo, merge, func(r string) { appendTo(t, r, "frontend/src/a.txt", idleRule) })
	if ok, err := containedAt(t, repo, base, patch, booted); err != nil || ok {
		t.Fatalf("a merge dropped the change: not contained, got %v %v", ok, err)
	}
}

// The evidence in its own syntax: the patch deletes a CSS block, main appends
// another; both have a line that is only "}". Punctuation-only lines, like
// blank ones, say nothing about the change; a real re-add brings back the
// block's lettered lines and is still caught.
func TestPatchIDPunctuationOnlyLinesAreNotEvidence(t *testing.T) {
	repo, root := baseRepo(t)
	css := ".a {\n  color: red;\n}\n\n/* drift */\n.drift {\n  animation: drift 9s;\n}\n"
	base := platformMoved(t, repo, root, func(r string) { write(t, r, "frontend/src/app.css", css) })
	drop := func(r string) { write(t, r, "frontend/src/app.css", ".a {\n  color: red;\n}\n") }
	patch := patchFrom(t, repo, base, drop)
	promoted := platformMoved(t, repo, base, drop)
	idle := platformMoved(t, repo, promoted, func(r string) {
		appendTo(t, r, "frontend/src/app.css", "\n:root[data-idle] * {\n  animation-play-state: paused;\n}\n")
	})
	if ok, err := containedAt(t, repo, base, patch, idle); err != nil || !ok {
		t.Fatalf("a new block beside the deletion shares only \"}\": contained, got %v %v", ok, err)
	}
	readd := platformMoved(t, repo, idle, func(r string) {
		appendTo(t, r, "frontend/src/app.css", ".drift {\n  animation: drift 9s;\n}\n")
	})
	if ok, err := containedAt(t, repo, base, patch, readd); err != nil || ok {
		t.Fatalf("the deleted block is back: not contained, got %v %v", ok, err)
	}
}
