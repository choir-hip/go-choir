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

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/pkgrelease"
	"github.com/yusefmosiah/go-choir/internal/pkgruntime"
	"github.com/yusefmosiah/go-choir/internal/pkgruntime/runtimetest"
	"github.com/yusefmosiah/go-choir/internal/store"
)

// Runtime install (docs/design/s12-runtime-install-build-2026-11-04.md),
// failure modes written before the code:
//
//	F1  a record whose bytes do not hash to the id is installed;
//	F3  tree bytes that are not the record's are built;
//	S1  another owner's release runs in the shell origin (before S13's frame);
//	S2  an install changes the source tree, asks the desks, or records an
//	    install before its module is stored;
//	S3  a repeat install builds and records again;
//	S4  another track's release of the same package does not replace the
//	    earlier one, or the earlier record still reads as in force;
//	S5  an uninstall removes a release that is not the one in force, or
//	    leaves the package listed or its module served;
//	S6  an install whose module left the store (a restore onto a fresh
//	    store) reads as installed and is never rebuilt;
//	S7  a failed build leaves a record or a module;
//	S8  a computer without a builder or store half-installs;
//	S9  the module route serves bytes no install in force names, or bytes
//	    that do not match their digest;
//	S10 the installed list or a module is served without the owner.

const installName = "demo-next"

func installManifest(title string) string {
	return `{
  "schema": "choir.package/v2",
  "name": "` + installName + `",
  "version": "1.0.0",
  "display": {"title": "` + title + `", "icon": "D", "description": "Demo"},
  "app": {"entry": "App.svelte", "abi": "choir.app-frontend/svelte-inline/v1", "aliases": [],
    "launcher": {"menu": true}, "window": {"singleton": false}, "auth": {"preview": "private"}, "theme": {}},
  "opens": [],
  "kinds": [],
  "uses_kinds": [],
  "contributes": {"desks": []},
  "capabilities": []
}
`
}

// publishedPackage is a new-package release of demo-next whose files are
// files, on track (its publisher's owner id), built on base in repo, the way
// a publisher makes one.
func publishedPackage(t *testing.T, repo, base, track string, files map[string]string) (pkgrelease.Record, string, json.RawMessage, []byte) {
	t.Helper()
	ctx := context.Background()
	root := pkgrelease.Root(installName)
	full := map[string]string{}
	for name, content := range files {
		full[root+"/"+name] = content
	}
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
	manifest, version, err := pkgrelease.CheckManifest(installName, []byte(files["choir-package.json"]), nil)
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

func packagesCall(t *testing.T, h *APIHandler, path, owner string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodGet, path, nil)
	if owner != "" {
		r.Header.Set("X-Authenticated-User", owner)
	}
	w := httptest.NewRecorder()
	h.HandlePackages(w, r)
	return w
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

// fakeModuleBuilder verifies the release as the real builder does and
// returns a module naming the release tree, without node.
type fakeModuleBuilder struct {
	builds int
	fail   error
}

func (f *fakeModuleBuilder) Build(ctx context.Context, rec pkgrelease.Record, treePatch []byte) (pkgruntime.Module, error) {
	if err := pkgrelease.VerifyContent(ctx, rec, treePatch); err != nil {
		return pkgruntime.Module{}, err
	}
	if f.fail != nil {
		return pkgruntime.Module{}, f.fail
	}
	f.builds++
	raw := []byte("export const abi='choir.app-frontend/module/v1';export const requires_sdk='^1.0';export const sdk_imports=[];export function mount(){} // " + rec.Source.Tree + "\n")
	return pkgruntime.Module{SHA256: computerevent.DigestBytes(raw), Size: len(raw), ABI: pkgruntime.ModuleABI, RequiresSDK: "^1.0", Manifest: json.RawMessage(`{"name":"` + rec.Package.Name + `"}`), Bytes: raw}, nil
}

type installEnv struct {
	rt       *Runtime
	h        *APIHandler
	repo     string
	base     string
	builder  *fakeModuleBuilder
	computer string
	owner    string
}

func newInstallEnv(t *testing.T) *installEnv {
	t.Helper()
	const computerID, owner = "computer-b", "owner-b"
	rt, _, _ := releaseTestRuntime(t, computerID)
	repo, base := releaseSourceRepo(t)
	builder := &fakeModuleBuilder{}
	rt.packageBuilder = builder
	rt.packageModules = pkgruntime.Store{Dir: t.TempDir()}
	return &installEnv{rt: rt, h: NewAPIHandler(rt), repo: repo, base: base, builder: builder, computer: computerID, owner: owner}
}

func (e *installEnv) post(t *testing.T, body []byte) (int, PackageInstallView, string) {
	t.Helper()
	w := releaseRouterCall(t, e.h, http.MethodPost, "/api/computers/"+e.computer+"/packages/installs", e.owner, e.computer, body)
	var view PackageInstallView
	var refusal releaseAPIError
	_ = json.Unmarshal(w.Body.Bytes(), &view)
	_ = json.Unmarshal(w.Body.Bytes(), &refusal)
	return w.Code, view, refusal.Reason
}

func (e *installEnv) list(t *testing.T) []PackageInstallView {
	t.Helper()
	w := releaseRouterCall(t, e.h, http.MethodGet, "/api/computers/"+e.computer+"/packages/installs", e.owner, e.computer, nil)
	var body struct {
		Installs []PackageInstallView `json:"installs"`
	}
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &body) != nil {
		t.Fatalf("list = %d %s", w.Code, w.Body.String())
	}
	return body.Installs
}

func (e *installEnv) installed(t *testing.T) []InstalledPackage {
	t.Helper()
	w := packagesCall(t, e.h, packagesInstalledPath, e.owner)
	var body struct {
		Packages []InstalledPackage `json:"packages"`
	}
	if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &body) != nil {
		t.Fatalf("installed = %d %s", w.Code, w.Body.String())
	}
	return body.Packages
}

func (e *installEnv) records(t *testing.T) int {
	t.Helper()
	list, err := e.rt.packageInstalls(context.Background(), e.owner, e.computer)
	if err != nil {
		t.Fatal(err)
	}
	return len(list)
}

func TestRuntimeInstallInstallsReplacesUninstallsAndReinstalls(t *testing.T) {
	e := newInstallEnv(t)
	head := releaseGit(t, e.repo, "rev-parse", "HEAD")
	_, id, raw, tree := publishedPackage(t, e.repo, e.base, e.owner, map[string]string{
		"choir-package.json": installManifest("Demo"), "App.svelte": "<p>demo</p>\n",
	})

	// F1: a record that does not hash to the id.
	tampered := strings.Replace(string(raw), `"op-owner-b"`, `"op-owner-z"`, 1)
	if code, _, reason := e.post(t, installBody(t, id, json.RawMessage(tampered), tree, "")); code != http.StatusUnprocessableEntity || reason != "release_digest_mismatch" {
		t.Fatalf("F1 = %d %s", code, reason)
	}
	// F3: other bytes.
	if code, _, reason := e.post(t, installBody(t, id, raw, append(append([]byte{}, tree...), 'x'), "")); code != http.StatusConflict || reason != "patch_digest_mismatch" {
		t.Fatalf("F3 = %d %s", code, reason)
	}
	// S7: a failed build.
	e.builder.fail = errorsJoinBuild()
	if code, _, reason := e.post(t, installBody(t, id, raw, tree, "")); code != http.StatusUnprocessableEntity || reason != "package_build_failed" {
		t.Fatalf("S7 = %d %s", code, reason)
	}
	e.builder.fail = nil
	if n := e.records(t); n != 0 {
		t.Fatalf("refusals left %d records", n)
	}
	if entries, _ := os.ReadDir(filepath.Join(e.rt.packageModules.Dir, "modules")); len(entries) != 0 {
		t.Fatalf("refusals left modules %v", entries)
	}

	// S2: installed, with no source change.
	code, view, _ := e.post(t, installBody(t, id, raw, tree, ""))
	if code != http.StatusCreated || view.State != installStateInstalled || !view.PatchMatchesRelease || !view.ModuleAvailable || view.Mode != store.PackageInstallModeRuntime || view.TrajectoryID != "" {
		t.Fatalf("S2 = %d %+v", code, view)
	}
	if got := releaseGit(t, e.repo, "rev-parse", "HEAD"); got != head {
		t.Fatalf("S2: the source moved %s -> %s", head, got)
	}
	pkgs := e.installed(t)
	if len(pkgs) != 1 || pkgs[0].Name != installName || pkgs[0].ReleaseID != id || !pkgs[0].Available || pkgs[0].RequiresSDK != "^1.0" ||
		pkgs[0].Module.ABI != pkgruntime.ModuleABI || pkgs[0].Module.URL != packageModulesPrefix+view.ModuleSHA256+".js" {
		t.Fatalf("S2 installed = %+v", pkgs)
	}
	w := packagesCall(t, e.h, pkgs[0].Module.URL, e.owner)
	if w.Code != http.StatusOK || computerevent.DigestBytes(w.Body.Bytes()) != view.ModuleSHA256 || !strings.HasPrefix(w.Header().Get("Content-Type"), "text/javascript") {
		t.Fatalf("S2 module = %d %s", w.Code, w.Header())
	}

	// S3: a repeat answers the install, with no new build or record.
	code, again, _ := e.post(t, installBody(t, id, raw, tree, ""))
	if code != http.StatusOK || again.InstallID != view.InstallID || e.builder.builds != 1 || e.records(t) != 1 {
		t.Fatalf("S3 = %d %s builds %d", code, again.InstallID, e.builder.builds)
	}

	// S4: another release of the same package replaces it.
	_, id2, raw2, tree2 := publishedPackage(t, e.repo, e.base, e.owner, map[string]string{
		"choir-package.json": installManifest("Demo two"), "App.svelte": "<p>demo two</p>\n",
	})
	code, view2, _ := e.post(t, installBody(t, id2, raw2, tree2, ""))
	if code != http.StatusCreated || view2.State != installStateInstalled {
		t.Fatalf("S4 = %d %+v", code, view2)
	}
	list := e.list(t)
	if len(list) != 2 || list[0].ReleaseID != id2 || list[0].State != installStateInstalled || list[1].State != installStateReplaced || list[1].PatchMatchesRelease {
		t.Fatalf("S4 list = %+v", list)
	}
	if pkgs := e.installed(t); len(pkgs) != 1 || pkgs[0].ReleaseID != id2 {
		t.Fatalf("S4 installed = %+v", pkgs)
	}
	// S9: the replaced module is no longer served.
	if w := packagesCall(t, e.h, packageModulesPrefix+view.ModuleSHA256+".js", e.owner); w.Code != http.StatusNotFound {
		t.Fatalf("S9: a replaced module = %d", w.Code)
	}

	// S5: uninstall only what is in force.
	if code, _, reason := e.post(t, installBody(t, id, raw, nil, "uninstall")); code != http.StatusConflict || reason != "package_not_installed" {
		t.Fatalf("S5 replaced = %d %s", code, reason)
	}
	code, un, _ := e.post(t, installBody(t, id2, raw2, nil, "uninstall"))
	if code != http.StatusCreated || un.State != installStateUninstalled || !un.PatchMatchesRelease {
		t.Fatalf("S5 = %d %+v", code, un)
	}
	if pkgs := e.installed(t); len(pkgs) != 0 {
		t.Fatalf("S5 installed = %+v", pkgs)
	}
	if w := packagesCall(t, e.h, packageModulesPrefix+view2.ModuleSHA256+".js", e.owner); w.Code != http.StatusNotFound {
		t.Fatalf("S5: an uninstalled module = %d", w.Code)
	}

	// Reinstall: a new attempt, in force again.
	code, re, _ := e.post(t, installBody(t, id2, raw2, tree2, ""))
	if code != http.StatusCreated || re.Attempt != 2 || re.State != installStateInstalled {
		t.Fatalf("reinstall = %d %+v", code, re)
	}

	// S6: the module left the store; the repeat rebuilds it.
	if err := os.RemoveAll(e.rt.packageModules.Dir); err != nil {
		t.Fatal(err)
	}
	if pkgs := e.installed(t); len(pkgs) != 1 || pkgs[0].Available {
		t.Fatalf("S6 installed = %+v", pkgs)
	}
	code, rebuilt, _ := e.post(t, installBody(t, id2, raw2, tree2, ""))
	if code != http.StatusCreated || rebuilt.State != installStateInstalled || !rebuilt.ModuleAvailable {
		t.Fatalf("S6 = %d %+v", code, rebuilt)
	}
}

func errorsJoinBuild() error {
	return errors.Join(pkgruntime.ErrBuild, errors.New("vite: x"))
}

func TestRuntimeInstallNeedsItsBuilderAndOwner(t *testing.T) {
	e := newInstallEnv(t)
	_, id, raw, tree := publishedPackage(t, e.repo, e.base, e.owner, map[string]string{
		"choir-package.json": installManifest("Demo"), "App.svelte": "<p>demo</p>\n",
	})
	// S8
	e.rt.packageBuilder = nil
	if code, _, reason := e.post(t, installBody(t, id, raw, tree, "")); code != http.StatusServiceUnavailable || reason != "install_unavailable" {
		t.Fatalf("S8 = %d %s", code, reason)
	}
	e.rt.packageBuilder = e.builder
	if code, _, _ := e.post(t, installBody(t, id, raw, tree, "")); code != http.StatusCreated {
		t.Fatalf("install = %d", code)
	}
	// S10
	if w := packagesCall(t, e.h, packagesInstalledPath, ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("S10 list = %d", w.Code)
	}
	pkgs := e.installed(t)
	if w := packagesCall(t, e.h, pkgs[0].Module.URL, ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("S10 module = %d", w.Code)
	}
	// Another owner of this computer's store sees none of it.
	w := packagesCall(t, e.h, packagesInstalledPath, "owner-x")
	if w.Code != http.StatusOK || strings.Contains(w.Body.String(), installName) {
		t.Fatalf("another owner = %d %s", w.Code, w.Body.String())
	}
	// S9: a stored module whose bytes changed is not served.
	path := filepath.Join(e.rt.packageModules.Dir, "modules", pkgs[0].Module.SHA256+".js")
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("alert(1)"), 0o644); err != nil {
		t.Fatal(err)
	}
	if w := packagesCall(t, e.h, pkgs[0].Module.URL, e.owner); w.Code != http.StatusNotFound {
		t.Fatalf("S9 tampered = %d", w.Code)
	}
	if w := packagesCall(t, e.h, packageModulesPrefix+"../../etc/passwd", e.owner); w.Code != http.StatusNotFound {
		t.Fatalf("S9 path = %d", w.Code)
	}
}

// TestRuntimeInstallWithTheRealBuilder runs install to served module with the
// computer's builder (node and the frontend's locked dependencies), when they
// are installed: the module the shell fetches is the module/v1 build of the
// release.
func TestRuntimeInstallWithTheRealBuilder(t *testing.T) {
	deps := runtimetest.DepsDir(t)
	builder := pkgruntime.Builder{DepsDir: deps}
	if deps == "" || builder.Available() != nil {
		t.Skip("frontend/node_modules is not installed (npm ci in frontend)")
	}
	e := newInstallEnv(t)
	e.rt.packageBuilder = builder
	_, id, raw, tree := publishedPackage(t, e.repo, e.base, e.owner, map[string]string{
		"choir-package.json": installManifest("Demo"),
		"App.svelte":         "<script>\n  import { appTitle } from '../../lib/app-sdk';\n</script>\n<p data-demo>{appTitle('demo')}</p>\n",
	})
	code, view, reason := e.post(t, installBody(t, id, raw, tree, ""))
	if code != http.StatusCreated || view.State != installStateInstalled || view.RequiresSDK != "^1.0" {
		t.Fatalf("install = %d %s %+v", code, reason, view)
	}
	pkgs := e.installed(t)
	if len(pkgs) != 1 || !pkgs[0].Available {
		t.Fatalf("installed = %+v", pkgs)
	}
	w := packagesCall(t, e.h, pkgs[0].Module.URL, e.owner)
	body := w.Body.String()
	if w.Code != http.StatusOK || computerevent.DigestBytes(w.Body.Bytes()) != pkgs[0].Module.SHA256 ||
		!strings.Contains(body, `"choir.app-frontend/module/v1"`) || !strings.Contains(body, `["appTitle"]`) {
		t.Fatalf("module = %d (%d bytes)", w.Code, len(body))
	}
}

// B1 (review): an uninstall needs nothing from the catalog. A release that
// was withdrawn, suspended, yanked or deleted arrives with no record (the
// proxy reads nothing for an uninstall), and a computer whose builder is
// gone can still remove what it installed.
func TestRuntimeUninstallNeedsNoCatalogRecordOrBuilder(t *testing.T) {
	e := newInstallEnv(t)
	_, id, raw, tree := publishedPackage(t, e.repo, e.base, e.owner, map[string]string{
		"choir-package.json": installManifest("Demo"), "App.svelte": "<p>demo</p>\n",
	})
	if code, _, reason := e.post(t, installBody(t, id, raw, tree, "")); code != http.StatusCreated {
		t.Fatalf("install = %d %s", code, reason)
	}
	e.rt.packageBuilder = nil
	body := []byte(`{"release_id":"` + id + `","action":"uninstall"}`)
	code, view, reason := e.post(t, body)
	if code != http.StatusCreated || view.State != installStateUninstalled || view.Package != installName || view.ReleaseTreeHash == "" {
		t.Fatalf("uninstall without a record = %d %s %+v", code, reason, view)
	}
	if pkgs := e.installed(t); len(pkgs) != 0 {
		t.Fatalf("installed after uninstall = %+v", pkgs)
	}
	// Nothing in force now: a second uninstall is refused, not a 500.
	if code, _, reason := e.post(t, body); code != http.StatusConflict || reason != "package_not_installed" {
		t.Fatalf("second uninstall = %d %s", code, reason)
	}
}

// (review, DeepSeek) an attempt id that a record already holds is never
// reused: the next free id is taken, so a reinstall cannot 500 after its
// module is stored.
func TestRuntimeInstallNeverReusesAnInstallID(t *testing.T) {
	e := newInstallEnv(t)
	_, id, raw, tree := publishedPackage(t, e.repo, e.base, e.owner, map[string]string{
		"choir-package.json": installManifest("Demo"), "App.svelte": "<p>demo</p>\n",
	})
	// A record already holds attempt 1's id (as one the listing window once
	// missed would), but under another release, so the attempt count is 1.
	taken := packageInstallID(e.computer, id, packageActionInstall, 1)
	if err := e.rt.store.PutPackageInstall(context.Background(), store.PackageInstall{
		OwnerID: e.owner, ComputerID: e.computer, InstallID: taken, ReleaseID: strings.Repeat("f", 64),
		Kind: installKindPackage, Package: "other", Action: packageActionInstall, Attempt: 1, Mode: store.PackageInstallModeRuntime,
	}); err != nil {
		t.Fatal(err)
	}
	code, view, reason := e.post(t, installBody(t, id, raw, tree, ""))
	if code != http.StatusCreated || view.InstallID == taken || !strings.HasPrefix(view.InstallID, taken) {
		t.Fatalf("install = %d %s %q", code, reason, view.InstallID)
	}
}

// Open install (owner, 2026-11-04): a release another owner published
// installs like one's own (the Library is an app store); the guest holds no
// cross-owner wall. A forged record is still refused by digest, and uninstall
// still removes it.
func TestRuntimeInstallInstallsAnotherOwnersRelease(t *testing.T) {
	e := newInstallEnv(t)
	_, otherID, otherRaw, otherTree := publishedPackage(t, e.repo, e.base, "owner-x", map[string]string{
		"choir-package.json": installManifest("Theirs"), "App.svelte": "<p>theirs</p>\n",
	})
	forged := strings.Replace(string(otherRaw), `"op-owner-x"`, `"op-owner-z"`, 1)
	if forged == string(otherRaw) {
		t.Fatal("the forgery changed nothing")
	}
	if code, _, reason := e.post(t, installBody(t, otherID, json.RawMessage(forged), otherTree, "")); code != http.StatusUnprocessableEntity || reason != "release_digest_mismatch" {
		t.Fatalf("forged = %d %s", code, reason)
	}
	code, view, reason := e.post(t, installBody(t, otherID, otherRaw, otherTree, ""))
	if code != http.StatusCreated || view.State != installStateInstalled || !view.ModuleAvailable {
		t.Fatalf("another owner's release = %d %s %+v", code, reason, view)
	}
	if n := e.records(t); n != 1 {
		t.Fatalf("records = %d", n)
	}
	code, view, reason = e.post(t, []byte(`{"release_id":"`+otherID+`","action":"uninstall"}`))
	if code != http.StatusCreated || view.State != installStateUninstalled {
		t.Fatalf("uninstall = %d %s %+v", code, reason, view)
	}
}
