package sourceproj

import (
	"context"
	"testing"
)

// ObjectIDs is the source identity the shot verdict reuse compares (design
// section 12, F3, F13). Ways it can fail, written before the code:
//  1. a Go-only commit changes the frontend tree id (no reuse, ever)
//  2. a frontend commit leaves the id equal (reuse of a changed frontend)
//  3. a missing path or commit yields an empty id that compares equal to
//     another empty id instead of an error
func TestInputIDsRealGit(t *testing.T) {
	repo, base := baseRepo(t)
	scratch := t.TempDir()
	tgit(t, scratch, "clone", "-q", "--no-checkout", repo, ".")
	tgit(t, scratch, "checkout", "-q", base)

	write(t, scratch, "internal/other/x.go", "package other\n")
	write(t, scratch, "docs/note.md", "note\n")
	tgit(t, scratch, "add", "-A")
	tgit(t, scratch, "commit", "-q", "-m", "go only")
	goOnly := tgit(t, scratch, "rev-parse", "HEAD")

	editLine(t, scratch, "frontend/src/a.txt", 3, "a line 03 CHANGED")
	tgit(t, scratch, "add", "-A")
	tgit(t, scratch, "commit", "-q", "-m", "frontend")
	frontend := tgit(t, scratch, "rev-parse", "HEAD")
	tgit(t, repo, "fetch", "-q", scratch, "HEAD:refs/platform/ids")

	ctx := context.Background()
	paths := []string{"frontend", "frontend/src/a.txt"}
	atBase, err := ObjectIDs(ctx, repo, base, paths...)
	if err != nil {
		t.Fatal(err)
	}
	atGoOnly, err := ObjectIDs(ctx, repo, goOnly, paths...)
	if err != nil {
		t.Fatal(err)
	}
	atFrontend, err := ObjectIDs(ctx, repo, frontend, paths...)
	if err != nil {
		t.Fatal(err)
	}
	if atBase[0] == "" || atBase[0] != atGoOnly[0] || atBase[1] != atGoOnly[1] {
		t.Fatalf("a Go-only commit moved the frontend ids: %v -> %v", atBase, atGoOnly)
	}
	if atFrontend[0] == atBase[0] || atFrontend[1] == atBase[1] {
		t.Fatalf("a frontend commit left the ids equal: %v -> %v", atBase, atFrontend)
	}
}

func TestInputIDsMissingPathIsAnError(t *testing.T) {
	repo, base := baseRepo(t)
	ctx := context.Background()
	for _, paths := range [][]string{{"nix"}, {"frontend", "no/such/file"}, {""}, {"../x"}, {"-x"}} {
		if ids, err := ObjectIDs(ctx, repo, base, paths...); err == nil {
			t.Errorf("ObjectIDs(%v) = %v, want an error", paths, ids)
		}
	}
	for _, commit := range []string{"", "HEAD", "0000000000000000000000000000000000000000", "not-a-commit"} {
		if ids, err := ObjectIDs(ctx, repo, commit, "frontend"); err == nil {
			t.Errorf("ObjectIDs(commit %q) = %v, want an error", commit, ids)
		}
	}
}
