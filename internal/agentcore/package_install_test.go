package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/pkgrelease"
	"github.com/yusefmosiah/go-choir/internal/store"
)

// Package install (docs/design/package-install-2026-11-04.md), failure
// modes written before the code:
//
//	F1  a record whose bytes do not hash to the id is installed;
//	F3  tree bytes that are not the record's are staged;
//	F6  a first-party package or a local change is overwritten;
//	F10 a crash-closed attempt answers "already installed" forever;
//	F11 an uninstall removes something that is not the release;
//	F12 an install without the proxy's tree reaches the desks;
//	I5  a refusal leaves a trace: a prompt, an install record, a staging;
//	I6  the staged patch, applied to the computer's tree, is not the
//	    release's tree at the root, or touches anything else;
//	I7  "installed" is reported before the tree holds the release;
//	I8  another track's release of the same package cannot replace an
//	    earlier Library install (the owner's review of several tracks).

const installName = "demo-next"

func installManifest(title string) string {
	return `{
  "schema": "choir.app-package/v1",
  "name": "` + installName + `",
  "version": "1.0.0",
  "origin": "first_party",
  "display": {"title": "` + title + `", "icon": "D", "description": "Demo"},
  "app": {"entry": "App.svelte", "abi": "choir.app-frontend/svelte-inline/v1", "aliases": [],
    "launcher": {"menu": true}, "window": {"singleton": false}, "auth": {"preview": "private"}, "theme": {}},
  "opens": [],
  "kinds": [],
  "uses_kinds": [],
  "desk": null,
  "backend": null,
  "grants": []
}
`
}

// publishedPackage is a new-package release of demo-next whose files are
// files, on track, built on base in repo, the way a publisher makes one.
func publishedPackage(t *testing.T, repo, base, track string, files map[string]string) (pkgrelease.Record, string, json.RawMessage, []byte) {
	t.Helper()
	ctx := context.Background()
	root := pkgrelease.Root(installName)
	full := map[string]string{}
	for name, content := range files {
		full[root+"/"+name] = content
	}
	// The package tree's objects, written into repo the way a publisher's
	// own repository holds them; no work tree or ref changes.
	env := []string{"GIT_INDEX_FILE=" + filepath.Join(t.TempDir(), "index")}
	gitIn(t, repo, env, "", "read-tree", "--empty")
	for path, content := range full {
		sha := gitIn(t, repo, nil, content, "hash-object", "-w", "--stdin")
		gitIn(t, repo, env, "", "update-index", "--add", "--cacheinfo", "100644,"+sha+","+path)
	}
	fullTree := gitIn(t, repo, env, "", "write-tree")
	tree, present, err := pkgrelease.Subtree(ctx, repo, fullTree, root)
	if err != nil || !present {
		t.Fatalf("subtree: %v %v", present, err)
	}
	blobs, err := pkgrelease.BuildBlobs(ctx, repo, root, pkgrelease.EmptyTree, tree)
	if err != nil {
		t.Fatal(err)
	}
	manifest, version, err := pkgrelease.CheckManifest(installName, []byte(files["choir-package.json"]))
	if err != nil {
		t.Fatal(err)
	}
	rec := pkgrelease.Record{
		Schema: pkgrelease.Schema, Package: pkgrelease.Package{Name: installName, ManifestVersion: version}, Track: track,
		Parent: pkgrelease.Parent{Kind: pkgrelease.ParentBase, Commit: base, Tree: pkgrelease.EmptyTree},
		Source: pkgrelease.Source{
			Root: root, Tree: tree,
			TreePatch:    pkgrelease.Blob{SHA256: computerevent.DigestBytes(blobs.TreePatch), Bytes: len(blobs.TreePatch)},
			ReviewPatch:  pkgrelease.Blob{SHA256: computerevent.DigestBytes(blobs.ReviewPatch), Bytes: len(blobs.ReviewPatch)},
			ChangedPaths: []string{root + "/App.svelte", root + "/choir-package.json"}, BuiltOn: base,
		},
		Manifest:  manifest,
		Publisher: pkgrelease.Publisher{OwnerID: track, ComputerID: "computer-" + track},
		Operation: pkgrelease.Operation{OperationID: "op-" + track, BundleDigest: strings.Repeat("b", 64), DecisionEvent: strings.Repeat("e", 64)},
	}
	id, err := rec.ReleaseID()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := rec.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	return rec, id, raw, blobs.TreePatch
}

func gitIn(t *testing.T, dir string, env []string, stdin string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t"), env...)
	cmd.Stdin = strings.NewReader(stdin)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// releaseRouterCall sends one request through the computer's router as the
// proxy does, with the authenticated owner and computer headers.
func releaseRouterCall(t *testing.T, h *APIHandler, method, path, owner, computer string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader(string(body)))
	if owner != "" {
		r.Header.Set("X-Authenticated-User", owner)
	}
	r.Header.Set("X-Authenticated-Computer", computer)
	w := httptest.NewRecorder()
	h.HandleComputersRouter(w, r)
	return w
}

type fakePromptSubmitter struct {
	calls []string
}

func (f *fakePromptSubmitter) submit(_ context.Context, ownerID, text, commandID string) (PromptSubmission, error) {
	f.calls = append(f.calls, text)
	return PromptSubmission{SubmissionID: "submission-1", TrajectoryID: "traj-adopt-1", DocID: "doc-adopt-1"}, nil
}

func installBody(t *testing.T, id string, record json.RawMessage, treePatch []byte, action string) []byte {
	t.Helper()
	body := map[string]any{"release_id": id, "record": record}
	if treePatch != nil {
		body["tree_patch"] = treePatch
	}
	if action != "" {
		body["action"] = action
	}
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// commitStaged applies a staged install patch to repo and commits it: what
// the self-development apply and the source projection leave.
func commitStaged(t *testing.T, repo, staged string) {
	t.Helper()
	releaseGit(t, repo, "apply", filepath.Join(staged, "source.patch"))
	releaseGit(t, repo, "add", "-A")
	releaseGit(t, repo, "commit", "-q", "-m", "applied")
}

func TestPackageInstallRefusesWithoutTraceThenInstallsReplacesAndUninstalls(t *testing.T) {
	const computerID, owner = "computer-b", "owner-b"
	rt, _, updaterRoot := releaseTestRuntime(t, computerID)
	repo, base := releaseSourceRepo(t)
	rt.releaseSourceDir = repo
	submitter := &fakePromptSubmitter{}
	rt.SetPromptSubmitter(submitter.submit)
	h := NewAPIHandler(rt)
	route := "/api/computers/" + computerID + "/self-development/adoptions"
	root := pkgrelease.Root(installName)

	rec, id, raw, treePatch := publishedPackage(t, repo, base, "track-a", map[string]string{
		"choir-package.json": installManifest("Demo A"), "App.svelte": "<p>a</p>\n",
	})
	call := func(body []byte) (*ReleaseAdoptionView, int, string) {
		t.Helper()
		w := releaseRouterCall(t, h, http.MethodPost, route, owner, computerID, body)
		var got struct {
			ReleaseAdoptionView
			Reason string `json:"reason"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &got)
		return &got.ReleaseAdoptionView, w.Code, got.Reason
	}
	noTrace := func(leg string) {
		t.Helper()
		list, _ := rt.store.ListReleaseAdoptions(context.Background(), owner, computerID)
		entries, _ := os.ReadDir(filepath.Join(updaterRoot, "adoptions"))
		if len(submitter.calls) != 0 || len(list) != 0 || len(entries) != 0 {
			t.Fatalf("I5 %s: prompts %d, records %d, staged %d", leg, len(submitter.calls), len(list), len(entries))
		}
	}
	refused := func(leg string, body []byte, status int, reason string) {
		t.Helper()
		if _, code, got := call(body); code != status || got != reason {
			t.Fatalf("%s = %d %s, want %d %s", leg, code, got, status, reason)
		}
		noTrace(leg)
	}

	// F1: one field changed, the id kept.
	tampered := rec
	tampered.Track = "track-z"
	tamperedRaw, _ := tampered.Canonical()
	refused("F1", installBody(t, id, tamperedRaw, treePatch, ""), http.StatusUnprocessableEntity, "release_digest_mismatch")
	// F12: no tree from the proxy.
	refused("F12", installBody(t, id, raw, nil, ""), http.StatusConflict, "patch_unavailable")
	// F3: other bytes.
	refused("F3", installBody(t, id, raw, append(append([]byte{}, treePatch...), '\n'), ""), http.StatusConflict, "patch_digest_mismatch")
	// F11: nothing to uninstall.
	refused("F11 absent", installBody(t, id, raw, treePatch, "uninstall"), http.StatusConflict, "package_not_installed")
	// A software release no longer installs (P8 is deleted).
	refused("software release", installBody(t, strings.Repeat("a", 64), json.RawMessage(`{"schema":"choir.software_release.v1"}`), nil, ""), http.StatusUnprocessableEntity, "release_record_invalid")

	// F6: a package of that name this computer holds that is not the
	// release's parent, nor a Library install.
	local := t.TempDir()
	releaseGit(t, local, "clone", "-q", repo, ".")
	if err := os.MkdirAll(filepath.Join(local, filepath.FromSlash(root)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(local, filepath.FromSlash(root), "App.svelte"), []byte("<p>mine</p>\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	releaseGit(t, local, "add", "-A")
	releaseGit(t, local, "commit", "-q", "-m", "first party")
	rt.releaseSourceDir = local
	refused("F6", installBody(t, id, raw, treePatch, ""), http.StatusConflict, "package_name_taken")
	rt.releaseSourceDir = repo

	// Installed: staged once, the desks asked once, recorded as a package
	// install; not yet "in place" (I7).
	view, code, reason := call(installBody(t, id, raw, treePatch, ""))
	if code != http.StatusCreated || view.Kind != "package" || view.Package != installName || view.Action != "install" || view.Attempt != 1 || !view.SameBase || view.PatchMatchesRelease {
		t.Fatalf("install = %d %s %+v", code, reason, view)
	}
	if len(submitter.calls) != 1 || !strings.Contains(submitter.calls[0], "package:"+id) || !strings.Contains(submitter.calls[0], "check-packages.mjs") {
		t.Fatalf("desks' request = %q", submitter.calls)
	}
	staged := rt.stagedInstallDir(view.AdoptionID)
	stagedRecord, _ := os.ReadFile(filepath.Join(staged, "release.json"))
	if computerevent.DigestBytes(stagedRecord) != id {
		t.Fatal("staged record is not the release")
	}
	if dir, err := rt.adoptionInputDir(context.Background(), owner, computerID, view.TrajectoryID); err != nil || dir != staged {
		t.Fatalf("capsule input = %q %v", dir, err)
	}
	// A9: another trajectory gets nothing.
	if other, err := rt.adoptionInputDir(context.Background(), owner, computerID, "traj-other"); err != nil || other != "" {
		t.Fatalf("A9: another trajectory got %q, %v", other, err)
	}
	// A repeat answers the same install and asks nobody.
	if again, code, _ := call(installBody(t, id, raw, treePatch, "")); code != http.StatusOK || again.AdoptionID != view.AdoptionID || len(submitter.calls) != 1 {
		t.Fatalf("repeat = %d %+v prompts %d", code, again, len(submitter.calls))
	}

	// I6: the staged patch makes the release's tree at the root and nothing
	// else; then the install reads as in place (I7).
	before := releaseGit(t, repo, "rev-parse", "HEAD")
	commitStaged(t, repo, staged)
	if tree, present, _ := pkgrelease.Subtree(context.Background(), repo, "HEAD", root); !present || tree != rec.Source.Tree {
		t.Fatalf("I6: root = %s present %v, want %s", tree, present, rec.Source.Tree)
	}
	if paths, _ := pkgrelease.DiffPaths(context.Background(), repo, before, "HEAD"); len(pkgrelease.OutsideRoot(paths, root)) != 0 {
		t.Fatalf("I6: paths outside the root: %v", paths)
	}
	w := releaseRouterCall(t, h, http.MethodGet, route, owner, computerID, nil)
	var listed struct {
		Adoptions []ReleaseAdoptionView `json:"adoptions"`
	}
	if json.Unmarshal(w.Body.Bytes(), &listed) != nil || len(listed.Adoptions) != 1 || !listed.Adoptions[0].PatchMatchesRelease {
		t.Fatalf("I7: list = %s", w.Body.String())
	}

	// I8: another track's demo-next replaces the Library install.
	recB, idB, rawB, treePatchB := publishedPackage(t, repo, base, "track-b", map[string]string{
		"choir-package.json": installManifest("Demo B"), "App.svelte": "<p>b</p>\n",
	})
	viewB, code, reason := call(installBody(t, idB, rawB, treePatchB, ""))
	if code != http.StatusCreated || viewB.SameBase {
		t.Fatalf("I8: install B = %d %s %+v", code, reason, viewB)
	}
	commitStaged(t, repo, rt.stagedInstallDir(viewB.AdoptionID))
	if tree, _, _ := pkgrelease.Subtree(context.Background(), repo, "HEAD", root); tree != recB.Source.Tree {
		t.Fatalf("I8: root = %s, want B's %s", tree, recB.Source.Tree)
	}

	// F11: A is no longer what the computer holds, so uninstalling A is
	// refused; uninstalling B removes the root.
	refusedCalls := len(submitter.calls)
	if _, code, reason := call(installBody(t, id, raw, treePatch, "uninstall")); code != http.StatusConflict || reason != "package_not_installed" || len(submitter.calls) != refusedCalls {
		t.Fatalf("F11 = %d %s", code, reason)
	}
	un, code, reason := call(installBody(t, idB, rawB, treePatchB, "uninstall"))
	if code != http.StatusCreated || un.Action != "uninstall" {
		t.Fatalf("uninstall = %d %s", code, reason)
	}
	commitStaged(t, repo, rt.stagedInstallDir(un.AdoptionID))
	if _, present, _ := pkgrelease.Subtree(context.Background(), repo, "HEAD", root); present {
		t.Fatal("uninstall left the root")
	}

	// F10: an attempt with no operation that a crash left long ago does not
	// block a new one.
	stale := store.ReleaseAdoption{
		AdoptionID: packageInstallID(computerID, id, "install", 2), OwnerID: owner, ComputerID: computerID, ReleaseID: id,
		TrajectoryID: "traj-crashed", RequestedAt: time.Now().Add(-3 * time.Hour),
		Kind: "package", Package: installName, Action: "install", Attempt: 2, ReleaseTreeHash: rec.Source.Tree,
	}
	if err := rt.store.PutReleaseAdoption(context.Background(), stale); err != nil {
		t.Fatal(err)
	}
	retry, code, reason := call(installBody(t, id, raw, treePatch, ""))
	if code != http.StatusCreated || retry.Attempt != 3 {
		t.Fatalf("F10: retry = %d %s attempt %d", code, reason, retry.Attempt)
	}
}

// A8: the capsule input is refused once the staged bytes change.
func TestPackageInstallCapsuleInputRefusesTamperedStaging(t *testing.T) {
	const computerID, owner = "computer-b", "owner-b"
	rt, _, _ := releaseTestRuntime(t, computerID)
	repo, base := releaseSourceRepo(t)
	rt.releaseSourceDir = repo
	submitter := &fakePromptSubmitter{}
	rt.SetPromptSubmitter(submitter.submit)
	_, id, raw, treePatch := publishedPackage(t, repo, base, "track-a", map[string]string{
		"choir-package.json": installManifest("Demo"), "App.svelte": "<p>demo</p>\n",
	})
	w := releaseRouterCall(t, NewAPIHandler(rt), http.MethodPost, "/api/computers/"+computerID+"/self-development/adoptions", owner, computerID, installBody(t, id, raw, treePatch, ""))
	var view ReleaseAdoptionView
	if w.Code != http.StatusCreated || json.Unmarshal(w.Body.Bytes(), &view) != nil {
		t.Fatalf("install = %d %s", w.Code, w.Body.String())
	}
	patch := filepath.Join(rt.stagedInstallDir(view.AdoptionID), "source.patch")
	if err := os.Chmod(patch, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(patch, []byte("tampered"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := rt.adoptionInputDir(context.Background(), owner, computerID, view.TrajectoryID); !errors.Is(err, errAdoptionStagedTampered) {
		t.Fatalf("A8: tampered staged bytes = %v", err)
	}
}
