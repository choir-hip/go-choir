// Package runtimetest makes verified package releases for runtime install
// tests: a throwaway git repository whose base commit holds the package with
// a placeholder App.svelte, and a release whose tree is the given files.
package runtimetest

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/pkgrelease"
)

// Manifest is a valid first-party manifest for name.
func Manifest(name, extraApp string) string {
	return `{
  "schema": "choir.app-package/v1",
  "name": "` + name + `",
  "version": "1.0.0",
  "origin": "first_party",
  "display": {"title": "Fixture ` + name + `", "icon": "F", "description": "Runtime install fixture"},
  "app": {"entry": "App.svelte", "abi": "choir.app-frontend/svelte-inline/v1", "aliases": [],` + extraApp + `
    "launcher": {"menu": true, "desktopIcon": false, "order": 900},
    "window": {"singleton": false, "heavy": false},
    "auth": {"preview": "private"},
    "theme": {"surface": "standard", "shellDataAttr": "data-fixture-window", "contentClass": "fixture-content"}},
  "opens": [], "kinds": [], "uses_kinds": [], "desk": null, "backend": null, "grants": []
}
`
}

func git(t testing.TB, dir, stdin string, env []string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	cmd.Env = append(cmd.Env, env...)
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// Release returns a release of package name published by ownerID whose
// package files (paths relative to the package root) are files, and its
// tree_patch.
func Release(t testing.TB, ownerID, name string, files map[string]string) (pkgrelease.Record, []byte) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	root := pkgrelease.Root(name)
	repo := t.TempDir()
	git(t, repo, "", nil, "init", "-q")
	base := map[string]string{"choir-package.json": files["choir-package.json"], "App.svelte": "<p>base</p>\n"}
	for rel, content := range base {
		full := filepath.Join(repo, filepath.FromSlash(root+"/"+rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git(t, repo, "", nil, "add", "-A")
	git(t, repo, "", nil, "commit", "-q", "-m", "base")
	baseCommit := git(t, repo, "", nil, "rev-parse", "HEAD")
	env := []string{"GIT_INDEX_FILE=" + filepath.Join(t.TempDir(), "index")}
	git(t, repo, "", env, "read-tree", baseCommit)
	for rel, content := range files {
		sha := git(t, repo, content, nil, "hash-object", "-w", "--stdin")
		git(t, repo, "", env, "update-index", "--add", "--cacheinfo", "100644,"+sha+","+root+"/"+rel)
	}
	full := git(t, repo, "", env, "write-tree")
	ctx := context.Background()
	parent, _, err := pkgrelease.Subtree(ctx, repo, baseCommit, root)
	if err != nil {
		t.Fatal(err)
	}
	tree, _, err := pkgrelease.Subtree(ctx, repo, full, root)
	if err != nil {
		t.Fatal(err)
	}
	blobs, err := pkgrelease.BuildBlobs(ctx, repo, root, parent, tree)
	if err != nil {
		t.Fatal(err)
	}
	manifest, version, err := pkgrelease.CheckManifest(name, []byte(files["choir-package.json"]))
	if err != nil {
		t.Fatal(err)
	}
	blob := func(raw []byte) *pkgrelease.Blob {
		if raw == nil {
			return nil
		}
		return &pkgrelease.Blob{SHA256: computerevent.DigestBytes(raw), Bytes: len(raw)}
	}
	rec := pkgrelease.Record{
		Schema: pkgrelease.Schema, Package: pkgrelease.Package{Name: name, ManifestVersion: version}, Track: ownerID,
		Parent:      pkgrelease.Parent{Kind: pkgrelease.ParentBase, Commit: baseCommit, Tree: parent},
		ParentPatch: blob(blobs.ParentPatch),
		Source: pkgrelease.Source{
			Root: root, Tree: tree, TreePatch: *blob(blobs.TreePatch), ReviewPatch: *blob(blobs.ReviewPatch),
			ChangedPaths: []string{root + "/App.svelte"}, BuiltOn: baseCommit,
		},
		Manifest:  manifest,
		Publisher: pkgrelease.Publisher{OwnerID: ownerID, ComputerID: "computer-" + ownerID},
		Operation: pkgrelease.Operation{OperationID: "op-1", BundleDigest: strings.Repeat("b", 64), DecisionEvent: strings.Repeat("e", 64)},
	}
	if err := rec.Validate(); err != nil {
		t.Fatal(err)
	}
	return rec, blobs.TreePatch
}

// DepsDir is the repository's frontend directory when its node_modules are
// installed (npm ci in frontend), or "" so a build test can skip.
func DepsDir(t testing.TB) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	for dir := wd; dir != filepath.Dir(dir); dir = filepath.Dir(dir) {
		front := filepath.Join(dir, "frontend")
		if _, err := os.Stat(filepath.Join(front, "node_modules", "vite")); err == nil {
			if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
				return front
			}
		}
	}
	return ""
}
