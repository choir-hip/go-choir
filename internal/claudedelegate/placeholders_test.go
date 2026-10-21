package claudedelegate

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

// Failure 29 (docs/problems/engineering-completion-path-cluster-2026-10-20.md):
// with the subprocess env scrub on, Claude Code creates empty placeholder
// files (package.json, lockfiles, .npmrc, .env*), node_modules/.bin and a
// stub .git in its working directory, and they stayed in the source tree.
// The server removes, after the run, the placeholders that were absent
// before it. Failure modes: a placeholder survives (the build reads an
// empty package.json); a file that existed before the run is removed or
// changed; the delegate's own work (a new non-placeholder file, a
// placeholder name it filled with content) is removed; a .git or
// node_modules that existed before is removed; the cleanup follows a
// symlink out of the tree.
func TestServerRemovesClaudeCodePlaceholdersItDidNotFind(t *testing.T) {
	tree := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "victim"), []byte("outside"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tree, ".npmrc"), []byte("keep=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tree, "frontend", "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	script := `
read prompt
cd ` + tree + `
for f in package.json package-lock.json .npmrc .yarnrc .yarnrc.yml bunfig.toml .gitmodules yarn.lock .env .env.local .env.production.local; do : >> "$f"; done
mkdir -p node_modules/.bin .git/info .git/modules .git/worktrees .git/glab-cli
printf '\n# claude\n' >> .git/info/exclude
printf . > .git/commondir
: >> .git/config.worktree
printf 'real lock\n' > pnpm-lock.yaml
ln -s ` + outside + ` .env.test
printf 'notes\n' > notes.md
printf '<script></script>\n' > frontend/src/X.svelte
echo '{"type":"result","subtype":"success","is_error":false,"num_turns":1,"session_id":"123e4567-e89b-12d3-a456-426614174000"}'
`
	srv := NewServer(t.TempDir(), fakeToken, []string{"PATH=" + os.Getenv("PATH")}, fakeClaude(t, script))
	srv.Workdir = tree
	if _, err := srv.Start(Spec{Objective: "write", MaxTurns: 1, MaxBudgetUSD: 1, WallClockMin: 1}); err != nil {
		t.Fatal(err)
	}
	final, _ := srv.Wait(10 * time.Second)
	if final.State != StateDone {
		t.Fatalf("final = %+v", final)
	}
	var left []string
	entries, _ := os.ReadDir(tree)
	for _, e := range entries {
		left = append(left, e.Name())
	}
	if want := []string{".npmrc", "frontend", "notes.md", "pnpm-lock.yaml"}; !slices.Equal(left, want) {
		t.Fatalf("tree after the run = %v; want %v", left, want)
	}
	if b, _ := os.ReadFile(filepath.Join(tree, ".npmrc")); string(b) != "keep=1\n" {
		t.Fatalf("a file that existed before the run changed: %q", b)
	}
	if b, _ := os.ReadFile(filepath.Join(outside, "victim")); string(b) != "outside" {
		t.Fatal("the cleanup followed a symlink out of the tree")
	}
	if _, err := os.Stat(filepath.Join(tree, "frontend", "src", "X.svelte")); err != nil {
		t.Fatal("the delegate's own file was removed")
	}
	if len(final.PlaceholdersRemoved) == 0 || !slices.Contains(final.PlaceholdersRemoved, "package.json") || !slices.Contains(final.PlaceholdersRemoved, ".git") {
		t.Fatalf("placeholders_removed = %v", final.PlaceholdersRemoved)
	}
}

// A .git and a node_modules that were there before the run stay.
func TestServerKeepsAGitAndNodeModulesThatExistedBefore(t *testing.T) {
	tree := t.TempDir()
	for _, d := range []string{".git/info", "node_modules/vite"} {
		if err := os.MkdirAll(filepath.Join(tree, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	script := `
read prompt
cd ` + tree + `
mkdir -p node_modules/.bin .git/modules
: >> package.json
echo '{"type":"result","subtype":"success","is_error":false,"num_turns":1,"session_id":"123e4567-e89b-12d3-a456-426614174000"}'
`
	srv := NewServer(t.TempDir(), fakeToken, []string{"PATH=" + os.Getenv("PATH")}, fakeClaude(t, script))
	srv.Workdir = tree
	if _, err := srv.Start(Spec{Objective: "write", MaxTurns: 1, MaxBudgetUSD: 1, WallClockMin: 1}); err != nil {
		t.Fatal(err)
	}
	if final, _ := srv.Wait(10 * time.Second); final.State != StateDone {
		t.Fatalf("final = %+v", final)
	}
	for _, d := range []string{".git/info", "node_modules/vite"} {
		if _, err := os.Stat(filepath.Join(tree, d)); err != nil {
			t.Fatalf("%s existed before the run and is gone: %v", d, err)
		}
	}
	if _, err := os.Lstat(filepath.Join(tree, "package.json")); !os.IsNotExist(err) {
		t.Fatalf("the package.json placeholder survived: %v", err)
	}
}
