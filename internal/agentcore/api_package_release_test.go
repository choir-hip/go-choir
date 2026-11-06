package agentcore

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/pkgrelease"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/store"
)

// Library S2 guest draft (docs/design/library-package-propagation-2026-10-26.md
// 3.2), as forced modularity leaves it
// (docs/design/forced-modularity-build-2026-11-04.md sections 1 and 2): the
// draft reads one package operation's frozen bundle. Failure modes pinned:
//
//	L15  a mixed operation (the package and app.css), or one that also
//	     changes a sibling package, is drafted as a package; the refusal
//	     names the outside paths and points to the platform proposal;
//	L25  a manifest that declares kinds is drafted before S12;
//	G1   a packages.publish setting can still refuse a draft (the switch is
//	     deleted: publishing is the only way an app exists on its builder);
//	G2   an undecided operation is drafted (a release names its decision);
//	G3   the draft's tree_patch carries a path outside the root, or the
//	     draft does not pass the host's own checks on the guest's base;
//	G4   the draft writes (it is GET only);
//	G5   an added secret is drafted.

const draftRoot = "frontend/src/packages/reader"

const draftManifest = `{
  "schema": "choir.package/v2",
  "name": "reader",
  "version": "1.0.0",
  "display": {"title": "Reader", "icon": "R", "description": "Read"},
  "app": {"entry": "App.svelte", "abi": "choir.app-frontend/svelte-inline/v1", "aliases": [],
    "launcher": {"menu": true, "desktopIcon": false, "order": 1}, "window": {"singleton": false, "heavy": false}, "auth": {"preview": "public-preview"}, "theme": {"surface": "standard", "shellDataAttr": "data-x-window", "contentClass": "x-content"}},
    "kinds": [],
  "uses_kinds": [],
  "contributes": {"desks": []},
  "capabilities": []
}
`

// packageSourceRepo is a guest source repository whose one commit holds the
// reader package and a shell stylesheet beside it.
func packageSourceRepo(t *testing.T) (repo, base string) {
	t.Helper()
	repo, _ = releaseSourceRepo(t)
	for path, content := range map[string]string{
		draftRoot + "/choir-package.json":  draftManifest,
		draftRoot + "/App.svelte":          "<p>reader</p>\n",
		draftRoot + "/choir-contract.json": `{"schema": "choir.package-contract/v1", "ready": "[data-reader]"}` + "\n",
		"frontend/src/app.css":             "body {}\n",
	} {
		full := filepath.Join(repo, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	releaseGit(t, repo, "add", "-A")
	releaseGit(t, repo, "commit", "-q", "-m", "packages")
	return repo, releaseGit(t, repo, "rev-parse", "HEAD")
}

type draftWorld struct {
	t           *testing.T
	h           *APIHandler
	rt          *Runtime
	repo, base  string
	updaterRoot string
	products    *store.Store
	at          time.Time
}

const draftComputer = "computer-publisher"

func newDraftWorld(t *testing.T) *draftWorld {
	t.Helper()
	rt, products, updaterRoot := releaseTestRuntime(t, draftComputer)
	repo, base := packageSourceRepo(t)
	rt.releaseSourceDir = repo
	return &draftWorld{t: t, h: NewAPIHandler(rt), rt: rt, repo: repo, base: base, updaterRoot: updaterRoot, products: products, at: time.Now().UTC().Add(-time.Hour)}
}

// operation records an operation in state whose frozen bundle carries the
// base-relative patch of files, created after every earlier one.
// operation records an operation; an applied one carries one accepted
// screenshot in its frozen bundle, since a release is drafted only with one.
func (w *draftWorld) operation(id, state string, files map[string]string) []byte {
	w.t.Helper()
	if state == selfdev.StateApplied {
		w.mediaOperation(id, files, map[string][]byte{"home.png": mediaPNG(w.t, 64, 40, 3, false)}, nil)
		patch, _ := releasePatch(w.t, w.repo, files)
		return patch
	}
	return w.bareOperation(id, state, files)
}

// bareOperation is an operation whose bundle names no screens.
func (w *draftWorld) bareOperation(id, state string, files map[string]string) []byte {
	w.t.Helper()
	patch, _ := releasePatch(w.t, w.repo, files)
	digest := releaseFrozenBundle(w.t, w.updaterRoot, draftComputer, id, w.base, patch)
	w.at = w.at.Add(time.Second)
	if _, err := w.products.DB().ExecContext(context.Background(), `INSERT INTO self_development_operations
		(operation_id,computer_id,idempotency_key,request_commitment,trajectory_id,capsule_id,base_head,prompt_artifact_ref,bundle_digest,release_digest,code_ref,verifier_refs_json,decision_event,desired_head,effective_head,checkpoint_ref,route_certificate,state,created_at,updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		id, draftComputer, "key-"+id, strings.Repeat("d", 64), "trajectory-"+id, "capsule-"+id, strings.Repeat("a", 64),
		"artifact:sha256:"+strings.Repeat("b", 64), digest, strings.Repeat("3", 64), "code:sha256:"+strings.Repeat("9", 64), `["verifier:`+id+`"]`,
		strings.Repeat("e", 64), strings.Repeat("c", 64), strings.Repeat("c", 64), "checkpoint:"+id, "route-cert:"+id, state, w.at, w.at); err != nil {
		w.t.Fatal(err)
	}
	return patch
}

type packageDraftBody struct {
	ReleaseID         string          `json:"release_id"`
	Record            json.RawMessage `json:"record"`
	TreePatchBase64   string          `json:"tree_patch_base64"`
	ReviewPatchBase64 string          `json:"review_patch_base64"`
	ParentPatchBase64 string          `json:"parent_patch_base64"`
}

func (w *draftWorld) draft(op string, query url.Values) *httptest.ResponseRecorder {
	if query == nil {
		query = url.Values{}
	}
	if query.Get("package") == "" {
		query.Set("package", "reader")
	}
	return releaseRouterCall(w.t, w.h, http.MethodGet, "/api/computers/"+draftComputer+"/packages/operations/"+op+"/release-draft?"+query.Encode(), "owner-a", draftComputer, nil)
}

func wantDraftReason(t *testing.T, rec *httptest.ResponseRecorder, status int, reason string) releaseAPIError {
	t.Helper()
	var got releaseAPIError
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if rec.Code != status || got.Reason != reason {
		t.Fatalf("draft = %d %s, want %d %s", rec.Code, rec.Body.String(), status, reason)
	}
	return got
}

func decodeDraft(t *testing.T, rec *httptest.ResponseRecorder) (pkgrelease.Record, string, pkgrelease.Blobs) {
	t.Helper()
	var body packageDraftBody
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &body) != nil {
		t.Fatalf("draft = %d %s", rec.Code, rec.Body.String())
	}
	record, err := pkgrelease.Decode(body.Record)
	if err != nil || pkgrelease.VerifyID(record, body.ReleaseID) != nil {
		t.Fatalf("the draft record does not verify against its id: %v", err)
	}
	var blobs pkgrelease.Blobs
	for _, b := range []struct {
		in  string
		out *[]byte
	}{{body.TreePatchBase64, &blobs.TreePatch}, {body.ReviewPatchBase64, &blobs.ReviewPatch}, {body.ParentPatchBase64, &blobs.ParentPatch}} {
		if b.in == "" {
			continue
		}
		raw, err := base64.StdEncoding.DecodeString(b.in)
		if err != nil {
			t.Fatal(err)
		}
		*b.out = raw
	}
	return record, body.ReleaseID, blobs
}

func TestPackageDraftNeedsADecidedOperationAndNoSetting(t *testing.T) {
	w := newDraftWorld(t)
	w.operation("op-awaiting", selfdev.StateAwaitingApproval, map[string]string{draftRoot + "/App.svelte": "<p>v2</p>\n"})
	w.paired("op-applied", map[string]string{draftRoot + "/App.svelte": "<p>v2</p>\n"})
	wantDraftReason(t, w.draft("op-awaiting", nil), http.StatusConflict, "operation_not_decided")
	// G1: the setting is gone; a computer cannot store it.
	if _, known := store.KnownComputerSettings["packages.publish"]; known {
		t.Fatal("G1 packages.publish is still a computer setting")
	}
	if err := w.rt.store.PutComputerSetting(context.Background(), store.ComputerSetting{OwnerID: "owner-a", ComputerID: draftComputer, Key: "packages.publish", Value: false}); err == nil {
		t.Fatal("G1 a computer stored the deleted packages.publish")
	}
	if rec := w.draft("op-applied", nil); rec.Code != http.StatusOK {
		t.Fatalf("an applied package operation drafts: %d %s", rec.Code, rec.Body.String())
	}
	if rec := releaseRouterCall(t, w.h, http.MethodPost, "/api/computers/"+draftComputer+"/packages/operations/op-applied/release-draft?package=reader", "owner-a", draftComputer, []byte("{}")); rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("G4 draft write = %d, want 405", rec.Code)
	}
	if rec := w.draft("op-applied", url.Values{"package": {"../x"}}); rec.Code != http.StatusBadRequest {
		t.Fatalf("a package name that is not a name = %d, want 400", rec.Code)
	}
}

func TestPackageDraftRefusesAMixedOperationNamingItsPaths(t *testing.T) {
	w := newDraftWorld(t)
	w.operation("op-mixed", selfdev.StateApplied, map[string]string{
		draftRoot + "/App.svelte": "<p>v2</p>\n",
		"frontend/src/app.css":    "body { color: red }\n",
	})
	got := wantDraftReason(t, w.draft("op-mixed", nil), http.StatusConflict, pkgrelease.ReasonChangeNotConfined)
	if !strings.Contains(got.Error, "frontend/src/app.css") || strings.Contains(got.Error, draftRoot+"/App.svelte") || !strings.Contains(got.Error, "choir platform publish") {
		t.Fatalf("L15 refusal = %q, want the outside path named and the platform proposal pointed to", got.Error)
	}
	// One operation, one package: a change to the reader drafted under
	// another package's name is outside that package.
	w.operation("op-reader", selfdev.StateApplied, map[string]string{draftRoot + "/App.svelte": "<p>v3</p>\n"})
	wantDraftReason(t, w.draft("op-reader", url.Values{"package": {"image"}}), http.StatusConflict, pkgrelease.ReasonChangeNotConfined)
}

func TestPackageDraftIsTheConfinedSubtreeOnTheBase(t *testing.T) {
	w := newDraftWorld(t)
	w.paired("op-reader", map[string]string{draftRoot + "/App.svelte": "<p>v2</p>\n"})
	record, id, blobs := decodeDraft(t, w.draft("op-reader", url.Values{"test_command": {"npm test"}}))
	baseTree := strings.TrimSpace(releaseGit(t, w.repo, "rev-parse", w.base+":"+draftRoot))
	if record.Package.Name != "reader" || record.Package.ManifestVersion != "1.0.0" || record.Track != "owner-a" ||
		record.Parent != (pkgrelease.Parent{Kind: pkgrelease.ParentBase, Commit: w.base, Tree: baseTree}) || record.ParentPatch == nil ||
		record.Source.Root != draftRoot || record.Source.BuiltOn != w.base || strings.Join(record.Source.ChangedPaths, ",") != draftRoot+"/App.svelte" ||
		record.Publisher != (pkgrelease.Publisher{OwnerID: "owner-a", ComputerID: draftComputer}) || record.Operation.OperationID != "op-reader" ||
		record.Declarations.TestCommand != "npm test" {
		t.Fatalf("draft record = %+v", record)
	}
	// G3: the draft passes the host's own checks against this base.
	if err := pkgrelease.VerifyHost(context.Background(), pkgrelease.HostInput{Record: record, TreePatch: blobs.TreePatch, ReviewPatch: blobs.ReviewPatch, ParentPatch: blobs.ParentPatch, Base: pkgrelease.GitStore{Dir: w.repo}}); err != nil {
		t.Fatalf("G3 the draft fails the host checks: %v", err)
	}
	// Deterministic: the same operation drafts the same release.
	if _, again, _ := decodeDraft(t, w.draft("op-reader", url.Values{"test_command": {"npm test"}})); again != id {
		t.Fatalf("second draft %s, first %s", again, id)
	}
}

func TestPackageDraftRefusesKindsAndSecrets(t *testing.T) {
	w := newDraftWorld(t)
	withKinds := strings.Replace(draftManifest, `"kinds": []`, `"kinds": [{"kind": "choir.app.reader.note"}]`, 1)
	w.operation("op-kinds", selfdev.StateApplied, map[string]string{draftRoot + "/choir-package.json": withKinds})
	wantDraftReason(t, w.draft("op-kinds", nil), http.StatusUnprocessableEntity, pkgrelease.ReasonKindsNeedPlatform)
	w = newDraftWorld(t)
	w.operation("op-secret", selfdev.StateApplied, map[string]string{draftRoot + "/App.svelte": "GITHUB_TOKEN=ghp_" + strings.Repeat("A", 36) + "\n"})
	wantDraftReason(t, w.draft("op-secret", nil), http.StatusUnprocessableEntity, pkgrelease.ReasonSecretFound)
}

// The owner's ruling of 2026-11-03: a published release is public, so
// there is no share act and no switch for it. Failure modes pinned:
//
//	P5   the share gate still answers, or packages.share is still a setting
//	     a computer stores (a hidden switch: nothing is off by default);
//	N1   (S2 residual bunch279-N1) a store or ref read failure while
//	     drafting answers 409, as if it were a conflict; it is 503.
func TestPackageShareGateAndSettingAreRetired(t *testing.T) {
	w := newDraftWorld(t)
	rec := releaseRouterCall(t, w.h, http.MethodGet, "/api/computers/"+draftComputer+"/packages/share-gate", "owner-a", draftComputer, nil)
	if rec.Code == http.StatusOK || strings.Contains(rec.Body.String(), "packages.share") {
		t.Fatalf("P5 the retired share gate answered %d %s", rec.Code, rec.Body.String())
	}
	if _, known := store.KnownComputerSettings["packages.share"]; known {
		t.Fatal("P5 packages.share is still a computer setting")
	}
	if err := w.rt.store.PutComputerSetting(context.Background(), store.ComputerSetting{OwnerID: "owner-a", ComputerID: draftComputer, Key: "packages.share", Value: true}); err == nil {
		t.Fatal("P5 a computer stored the retired packages.share")
	}
}

func TestPackageDraftReadFailureIsUnavailable(t *testing.T) {
	w := newDraftWorld(t)
	w.operation("op-0002", selfdev.StateApplied, map[string]string{draftRoot + "/App.svelte": "<p>v3</p>\n"})
	// The source repository is gone: the operation's base cannot be read.
	w.rt.releaseSourceDir = t.TempDir()
	wantDraftReason(t, w.draft("op-0002", nil), http.StatusServiceUnavailable, "tree_unavailable")
}
