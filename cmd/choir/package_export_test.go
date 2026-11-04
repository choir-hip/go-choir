package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/pkgrelease"
)

// choir package export (F1-F7 in package_export.go) against an httptest
// catalog and a throwaway git repository.

type exportWorld struct {
	t    *testing.T
	repo string
	base string
	// releases answers /api/catalog/releases/<id>; blobs answers .../blobs/<sha>.
	views map[string]json.RawMessage
	blobs map[string][]byte
}

func exportGit(t *testing.T, dir string, env []string, stdin string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdin = strings.NewReader(stdin)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func exportManifest(name, origin string) string {
	// origin is not a manifest field in v2: a non-first-party one is a
	// publisher-declared platform fact, which main's gate refuses.
	declared := ""
	if origin != "first_party" {
		declared = `"origin":"` + origin + `",`
	}
	return `{"schema":"choir.package/v2",` + declared + `"name":"` + name + `","version":"1.0.0",` +
		`"display":{"title":"` + name + `","icon":"","description":""},` +
		`"app":{"entry":"App.svelte","abi":"choir.app-frontend/svelte-inline/v1","aliases":[],"launcher":{"menu":true,"desktopIcon":false,"order":900},"window":{"singleton":false,"heavy":false},"auth":{"preview":"private"},"theme":{"surface":"standard","shellDataAttr":"data-x","contentClass":"x"}},` +
		`"opens":[],"kinds":[],"uses_kinds":[],"contributes":{"desks":[]},"capabilities":[]}` + "\n"
}

func newExportWorld(t *testing.T) *exportWorld {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	repo := t.TempDir()
	exportGit(t, repo, nil, "", "init", "-q", "-b", "main")
	exportGit(t, repo, nil, "", "config", "user.email", "export@test")
	exportGit(t, repo, nil, "", "config", "user.name", "Export Test")
	for path, body := range map[string]string{
		"other/x.txt": "x\n",
		"frontend/src/packages/reader/App.svelte":         "<p>v1</p>\n",
		"frontend/src/packages/reader/choir-package.json": exportManifest("reader", "first_party"),
	} {
		full := filepath.Join(repo, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	exportGit(t, repo, nil, "", "add", "-A")
	exportGit(t, repo, nil, "", "commit", "-q", "-m", "base")
	w := &exportWorld{t: t, repo: repo, base: exportGit(t, repo, nil, "", "rev-parse", "HEAD"), views: map[string]json.RawMessage{}, blobs: map[string][]byte{}}
	servePackageHost(t, func(rw http.ResponseWriter, r *http.Request) {
		rest := strings.TrimPrefix(r.URL.Path, "/api/catalog/releases/")
		if id, digest, ok := strings.Cut(rest, "/blobs/"); ok {
			raw, found := w.blobs[id+"/"+digest]
			if !found {
				rw.WriteHeader(http.StatusNotFound)
				return
			}
			rw.Header().Set("Content-Type", "application/octet-stream")
			_, _ = rw.Write(raw)
			return
		}
		view, found := w.views[rest]
		if !found {
			rw.WriteHeader(http.StatusNotFound)
			return
		}
		rw.Header().Set("Content-Type", "application/json")
		_, _ = rw.Write(view)
	})
	return w
}

// tree writes files under root into a full tree and returns the subtree id
// and the full tree.
func (w *exportWorld) tree(root string, files map[string]string, extra map[string]string) (string, string) {
	t := w.t
	env := []string{"GIT_INDEX_FILE=" + filepath.Join(t.TempDir(), "index")}
	exportGit(t, w.repo, env, "", "read-tree", "--empty")
	add := func(path, body string) {
		sha := exportGit(t, w.repo, nil, body, "hash-object", "-w", "--stdin")
		exportGit(t, w.repo, env, "", "update-index", "--add", "--cacheinfo", "100644,"+sha+","+path)
	}
	for name, body := range files {
		add(root+"/"+name, body)
	}
	for path, body := range extra {
		add(path, body)
	}
	full := exportGit(t, w.repo, env, "", "write-tree")
	return exportGit(t, w.repo, nil, "", "rev-parse", full+":"+root), full
}

type exportRelease struct {
	id     string
	record pkgrelease.Record
}

// release publishes (to the fake catalog) a release of name whose package
// files are files, on parentTree (EmptyTree for a new package). A non-nil
// treePatch replaces the honest one (a forged blob the record names).
func (w *exportWorld) release(name string, files map[string]string, parentTree string, treePatch []byte) exportRelease {
	t := w.t
	root := pkgrelease.Root(name)
	tree, _ := w.tree(root, files, nil)
	blobs, err := pkgrelease.BuildBlobs(context.Background(), w.repo, root, parentTree, tree)
	if err != nil {
		t.Fatal(err)
	}
	if treePatch != nil {
		blobs.TreePatch = treePatch
	}
	blob := func(raw []byte) pkgrelease.Blob {
		return pkgrelease.Blob{SHA256: computerevent.DigestBytes(raw), Bytes: len(raw)}
	}
	record := pkgrelease.Record{
		Schema: pkgrelease.Schema, Package: pkgrelease.Package{Name: name, ManifestVersion: "1.0.0"}, Track: "owner-a",
		Parent: pkgrelease.Parent{Kind: pkgrelease.ParentBase, Commit: w.base, Tree: parentTree},
		Source: pkgrelease.Source{
			Root: root, Tree: tree, TreePatch: blob(blobs.TreePatch), ReviewPatch: blob(blobs.ReviewPatch),
			ChangedPaths: []string{root + "/App.svelte"}, BuiltOn: w.base,
		},
		Manifest:  pkgrelease.Manifest{SHA256: strings.Repeat("4", 64), Schema: "choir.package/v2", ABI: "choir.app-frontend/svelte-inline/v1", Class: "app", Level: "default", BehaviourSurface: strings.Repeat("5", 64)},
		Publisher: pkgrelease.Publisher{OwnerID: "owner-a", ComputerID: "computer-a1"},
		Operation: pkgrelease.Operation{OperationID: "op-" + name, BundleDigest: strings.Repeat("b", 64), DecisionEvent: strings.Repeat("e", 64)},
	}
	if parentTree != pkgrelease.EmptyTree {
		p := blob(blobs.ParentPatch)
		record.ParentPatch = &p
	}
	id, err := record.ReleaseID()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := record.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	view, _ := json.Marshal(map[string]any{"release_id": id, "record": json.RawMessage(raw), "published_at": "2026-11-04T00:00:00Z"})
	w.views[id] = view
	w.blobs[id+"/"+record.Source.TreePatch.SHA256] = blobs.TreePatch
	return exportRelease{id: id, record: record}
}

func (w *exportWorld) export(args ...string) (int, map[string]any, string) {
	var stdout, stderr bytes.Buffer
	code := run(append([]string{"package", "export", "--worktree", w.repo}, args...), &stdout, &stderr)
	var out map[string]any
	_ = json.Unmarshal(stdout.Bytes(), &out)
	return code, out, stderr.String()
}

func (w *exportWorld) head() string { return exportGit(w.t, w.repo, nil, "", "rev-parse", "HEAD") }

func (w *exportWorld) wantRefused(reason string, args ...string) {
	w.t.Helper()
	before := w.head()
	code, out, stderr := w.export(args...)
	w.t.Logf("%s -> %s", reason, strings.TrimSpace(stderr))
	if code == 0 || !strings.Contains(stderr, reason) {
		w.t.Fatalf("want %s; exit %d %v %s", reason, code, out, stderr)
	}
	if w.head() != before || exportGit(w.t, w.repo, nil, "", "status", "--porcelain") != "" {
		w.t.Fatalf("a refused export (%s) changed the worktree", reason)
	}
}

func TestPackageExportNewPackageCommitsTheReleaseWithItsTrailers(t *testing.T) {
	w := newExportWorld(t)
	files := map[string]string{"App.svelte": "<p>notes</p>\n", "choir-package.json": exportManifest("notes", "first_party")}
	rel := w.release("notes", files, pkgrelease.EmptyTree, nil)

	// F5: a package main does not hold needs --new-package.
	w.wantRefused("not_first_party", "--release", rel.id)
	base := w.head()
	code, out, stderr := w.export("--release", rel.id, "--new-package", "--trailer", "Co-Authored-By: A <a@test>")
	if code != 0 {
		t.Fatalf("export exit %d: %s", code, stderr)
	}
	commit := w.head()
	if out["commit"] != commit || out["tree"] != rel.record.Source.Tree || out["new_package"] != true || out["parent_commit"] != base ||
		!strings.Contains(out["pr_body"].(string), rel.id) {
		t.Fatalf("export output = %v", out)
	}
	if got := exportGit(t, w.repo, nil, "", "rev-parse", "HEAD:frontend/src/packages/notes"); got != rel.record.Source.Tree {
		t.Fatalf("package tree = %s, want %s", got, rel.record.Source.Tree)
	}
	if changed := exportGit(t, w.repo, nil, "", "diff-tree", "--no-commit-id", "--name-only", "-r", base, commit); strings.Contains(changed, "other/") || strings.Contains(changed, "reader/") {
		t.Fatalf("the export touched %s", changed)
	}
	trailers := exportGit(t, w.repo, nil, "", "log", "-1", "--format=%(trailers:only,unfold)")
	for _, want := range []string{
		"Choir-Package-Release: " + rel.id, "Choir-Package-Track: owner-a", "Choir-Publisher-Computer: computer-a1",
		"Choir-Publisher-Operation: op-notes", "Choir-Tree: " + rel.record.Source.Tree, "Co-Authored-By: A <a@test>",
	} {
		if !strings.Contains(trailers, want) {
			t.Fatalf("trailers %q lack %q", trailers, want)
		}
	}
	// F4: the same new-package release again: main now holds the package.
	w.wantRefused("onto_moved", "--release", rel.id, "--new-package")
}

func TestPackageExportFirstPartyUpdateAndOntoMoved(t *testing.T) {
	w := newExportWorld(t)
	parent := exportGit(t, w.repo, nil, "", "rev-parse", "HEAD:frontend/src/packages/reader")
	manifest := exportManifest("reader", "first_party")
	v2 := w.release("reader", map[string]string{"App.svelte": "<p>v2</p>\n", "choir-package.json": manifest}, parent, nil)
	v2b := w.release("reader", map[string]string{"App.svelte": "<p>v2b</p>\n", "choir-package.json": manifest}, parent, nil)
	if code, _, stderr := w.export("--release", v2.id, "--onto", "main"); code != 0 {
		t.Fatalf("a first-party export exit %d: %s", code, stderr)
	}
	// F4: main moved under v2b (cut from the same parent).
	w.wantRefused("onto_moved", "--release", v2b.id, "--onto", "main")
	// F6: HEAD is not --onto's package.
	w.wantRefused("head_not_onto", "--release", v2b.id, "--onto", w.base)
}

func TestPackageExportRefusals(t *testing.T) {
	w := newExportWorld(t)
	files := map[string]string{"App.svelte": "<p>notes</p>\n", "choir-package.json": exportManifest("notes", "first_party")}
	good := w.release("notes", files, pkgrelease.EmptyTree, nil)

	// F6: a dirty worktree.
	if err := os.WriteFile(filepath.Join(w.repo, "stray.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, stderr := w.export("--release", good.id, "--new-package")
	if code == 0 || !strings.Contains(stderr, "worktree_dirty") {
		t.Fatalf("a dirty worktree = %d %s", code, stderr)
	}
	_ = os.Remove(filepath.Join(w.repo, "stray.txt"))

	// F1: a malformed id, an unknown id, and a record that is not its id.
	w.wantRefused("release_unverified", "--release", "../../etc", "--new-package")
	w.wantRefused("404", "--release", strings.Repeat("0", 64), "--new-package")
	forged := strings.Repeat("9", 64)
	w.views[forged] = bytes.Replace(w.views[good.id], []byte(good.id), []byte(forged), 1)
	w.wantRefused("does not verify", "--release", forged, "--new-package")

	// F2: blob bytes that are not the record's.
	honest := w.blobs[good.id+"/"+good.record.Source.TreePatch.SHA256]
	w.blobs[good.id+"/"+good.record.Source.TreePatch.SHA256] = append(append([]byte{}, honest...), '\n')
	w.wantRefused("blob", "--release", good.id, "--new-package")
	w.blobs[good.id+"/"+good.record.Source.TreePatch.SHA256] = honest

	// F3: a tree patch that also writes outside the package root; the
	// record names it, and its package subtree is the record's tree.
	_, full := w.tree("frontend/src/packages/notes", files, map[string]string{"other/evil.sh": "rm -rf /\n"})
	escape := []byte(exportGit(t, w.repo, nil, "", "diff-tree", "-p", "-r", "--binary", "--full-index", "--no-renames", pkgrelease.EmptyTree, full) + "\n")
	evil := w.release("notes", files, pkgrelease.EmptyTree, escape)
	w.wantRefused("content_not_confined", "--release", evil.id, "--new-package")

	// F5: a manifest whose origin main's build gate refuses.
	third := w.release("thirdp", map[string]string{"App.svelte": "<p>t</p>\n", "choir-package.json": exportManifest("thirdp", "third_party")}, pkgrelease.EmptyTree, nil)
	w.wantRefused("manifest_not_first_party", "--release", third.id, "--new-package")

	// The honest release still exports after all of that.
	if code, _, stderr := w.export("--release", good.id, "--new-package"); code != 0 {
		t.Fatalf("export exit %d: %s", code, stderr)
	}
}

func TestPackageExportFromASelfDevSourcePatch(t *testing.T) {
	w := newExportWorld(t)
	files := map[string]string{"App.svelte": "<p>notes</p>\n", "choir-package.json": exportManifest("notes", "first_party")}
	rel := w.release("notes", files, pkgrelease.EmptyTree, nil)
	delete(w.blobs, rel.id+"/"+rel.record.Source.TreePatch.SHA256)

	// The operation's source patch: the package and a change elsewhere,
	// from built_on.
	_, full := w.tree("frontend/src/packages/notes", files, nil)
	env := []string{"GIT_INDEX_FILE=" + filepath.Join(t.TempDir(), "index")}
	exportGit(t, w.repo, env, "", "read-tree", w.base)
	exportGit(t, w.repo, env, "", "read-tree", "--prefix=frontend/src/packages/notes/", full+":frontend/src/packages/notes")
	sha := exportGit(t, w.repo, nil, "y\n", "hash-object", "-w", "--stdin")
	exportGit(t, w.repo, env, "", "update-index", "--add", "--cacheinfo", "100644,"+sha+",other/y.txt")
	after := exportGit(t, w.repo, env, "", "write-tree")
	patch := exportGit(t, w.repo, nil, "", "diff-tree", "-p", "-r", "--binary", "--full-index", "--no-renames", w.base+"^{tree}", after) + "\n"
	dir := t.TempDir()
	good, bad := filepath.Join(dir, "source.patch"), filepath.Join(dir, "other.patch")
	if err := os.WriteFile(good, []byte(patch), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bad, []byte(strings.Replace(patch, "<p>notes</p>", "<p>NOTES</p>", 1)), 0o600); err != nil {
		t.Fatal(err)
	}
	// F2: a source patch that rebuilds another tree.
	w.wantRefused("tree_mismatch", "--release", rel.id, "--new-package", "--source-patch", bad)
	code, out, stderr := w.export("--release", rel.id, "--new-package", "--source-patch", good)
	if code != 0 || out["source"] != "self_dev_source_patch" || out["tree"] != rel.record.Source.Tree {
		t.Fatalf("source-patch export exit %d %v %s", code, out, stderr)
	}
	// Only the package is taken from the source patch.
	if _, err := os.Stat(filepath.Join(w.repo, "other/y.txt")); !os.IsNotExist(err) {
		t.Fatalf("the export carried a path outside the package: %v", err)
	}
}
