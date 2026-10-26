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
	"github.com/yusefmosiah/go-choir/internal/sourceproj"
	"github.com/yusefmosiah/go-choir/internal/store"
)

// Library S2 guest draft (docs/design/library-package-propagation-2026-10-26.md
// 3.2, publisher-side checks), written before the code. Failure modes pinned:
//
//	L1   an operation whose parent subtree is the publisher's earlier,
//	     unpublished change is drafted on the base (it would carry that
//	     change): package_parent_unpublished, naming the earlier operation;
//	L15  a mixed operation (the package and app.css) is drafted as a
//	     package; the refusal names the outside paths and points to the
//	     platform proposal;
//	L25  a manifest that declares kinds is drafted before S12;
//	G1   the setting packages.publish off still drafts;
//	G2   an unapplied operation is drafted;
//	G3   the draft's tree_patch carries a path outside the root, or the
//	     draft does not pass the host's own checks on the guest's base;
//	G4   the draft writes (it is GET only);
//	G5   an added secret is drafted;
//	L1b  the operation just before this one left no projection ref, and an
//	     older ref stands in for the source before (review 274 F2): the
//	     diff would carry the skipped operation's change inside the root.

const draftRoot = "frontend/src/packages/reader"

const draftManifest = `{
  "schema": "choir.app-package/v1",
  "name": "reader",
  "version": "1.0.0",
  "origin": "first_party",
  "display": {"title": "Reader", "icon": "R", "description": "Read"},
  "app": {"entry": "App.svelte", "abi": "choir.app-frontend/svelte-inline/v1", "aliases": [],
    "launcher": {"menu": true}, "window": {"singleton": false}, "auth": {"preview": "public-preview"}, "theme": {}},
  "opens": [],
  "kinds": [],
  "uses_kinds": [],
  "desk": null,
  "backend": null,
  "grants": []
}
`

// packageSourceRepo is a guest source repository whose one commit holds the
// reader package and a shell stylesheet beside it.
func packageSourceRepo(t *testing.T) (repo, base string) {
	t.Helper()
	repo, _ = releaseSourceRepo(t)
	for path, content := range map[string]string{
		draftRoot + "/choir-package.json": draftManifest,
		draftRoot + "/App.svelte":         "<p>reader</p>\n",
		"frontend/src/app.css":            "body {}\n",
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

// projectApplied leaves the projection's ref for an applied operation: a
// commit of base plus patch whose trailers name the stack base and the
// operation, at refs/choir/applied/<digest>, as sourceproj writes it.
func projectApplied(t *testing.T, repo, base, operationID string, patch []byte) {
	t.Helper()
	scratch := t.TempDir()
	releaseGit(t, scratch, "clone", "-q", repo, ".")
	releaseGit(t, scratch, "checkout", "-q", base)
	if err := os.WriteFile(filepath.Join(scratch, "op.patch"), patch, 0o644); err != nil {
		t.Fatal(err)
	}
	releaseGit(t, scratch, "apply", "--index", "op.patch")
	releaseGit(t, scratch, "commit", "-q", "-m", "choir: apply "+operationID+"\n\n"+sourceproj.TrailerStackBase+": "+base+"\nChoir-Operation: "+operationID)
	commit := releaseGit(t, scratch, "rev-parse", "HEAD")
	releaseGit(t, scratch, "push", "-q", repo, commit+":"+sourceproj.RefAppliedPrefix+strings.Repeat("a", 60)+operationID[len(operationID)-4:])
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
	w := &draftWorld{t: t, h: NewAPIHandler(rt), rt: rt, repo: repo, base: base, updaterRoot: updaterRoot, products: products, at: time.Now().UTC().Add(-time.Hour)}
	w.setting(true)
	return w
}

func (w *draftWorld) setting(on bool) {
	w.t.Helper()
	if err := w.rt.store.PutComputerSetting(context.Background(), store.ComputerSetting{OwnerID: "owner-a", ComputerID: draftComputer, Key: store.SettingPackagesPublish, Value: on}); err != nil {
		w.t.Fatal(err)
	}
}

// operation records an operation in state whose frozen bundle carries the
// base-relative patch of files, created after every earlier one.
func (w *draftWorld) operation(id, state string, files map[string]string) []byte {
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

func TestPackageDraftNeedsTheSettingAndAnAppliedOperation(t *testing.T) {
	w := newDraftWorld(t)
	w.operation("op-awaiting", selfdev.StateAwaitingApproval, map[string]string{draftRoot + "/App.svelte": "<p>v2</p>\n"})
	w.operation("op-applied", selfdev.StateApplied, map[string]string{draftRoot + "/App.svelte": "<p>v2</p>\n"})
	wantDraftReason(t, w.draft("op-awaiting", nil), http.StatusConflict, "operation_not_applied")
	w.setting(false)
	wantDraftReason(t, w.draft("op-applied", nil), http.StatusConflict, "packages_publish_off")
	w.setting(true)
	if rec := releaseRouterCall(t, w.h, http.MethodPost, "/api/computers/"+draftComputer+"/packages/operations/op-applied/release-draft?package=reader", "owner-a", draftComputer, []byte("{}")); rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("G4 draft write = %d, want 405", rec.Code)
	}
	if rec := w.draft("op-applied", url.Values{"package": {"../x"}}); rec.Code != http.StatusBadRequest {
		t.Fatalf("a package name that is not a name = %d, want 400", rec.Code)
	}
}

func TestPackageDraftRefusesAMixedOperationNamingItsPaths(t *testing.T) {
	w := newDraftWorld(t)
	mixed := w.operation("op-mixed", selfdev.StateApplied, map[string]string{
		draftRoot + "/App.svelte": "<p>v2</p>\n",
		"frontend/src/app.css":    "body { color: red }\n",
	})
	// The projection left op-mixed's ref, so the operation after it is
	// measured from it (without one it would refuse first, see L1b).
	projectApplied(t, w.repo, w.base, "op-mixed", mixed)
	got := wantDraftReason(t, w.draft("op-mixed", nil), http.StatusConflict, pkgrelease.ReasonChangeNotConfined)
	if !strings.Contains(got.Error, "frontend/src/app.css") || strings.Contains(got.Error, draftRoot+"/App.svelte") || !strings.Contains(got.Error, "choir platform publish") {
		t.Fatalf("L15 refusal = %q, want the outside path named and the platform proposal pointed to", got.Error)
	}
	// A confined change drafted under another package's name is not confined
	// to that package.
	w.operation("op-reader", selfdev.StateApplied, map[string]string{draftRoot + "/App.svelte": "<p>v3</p>\n"})
	wantDraftReason(t, w.draft("op-reader", url.Values{"package": {"image"}}), http.StatusConflict, pkgrelease.ReasonChangeNotConfined)
}

func TestPackageDraftIsTheConfinedSubtreeOnTheBase(t *testing.T) {
	w := newDraftWorld(t)
	w.operation("op-reader", selfdev.StateApplied, map[string]string{draftRoot + "/App.svelte": "<p>v2</p>\n"})
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

func TestPackageDraftRefusesAParentThatIsAnEarlierUnpublishedChange(t *testing.T) {
	w := newDraftWorld(t)
	// op-1 changed the reader and the shell; it applied, and the projection
	// left its ref. It was never published (it is mixed).
	first := map[string]string{draftRoot + "/App.svelte": "<p>v2</p>\n", "frontend/src/app.css": "body { color: red }\n"}
	projectApplied(t, w.repo, w.base, "op-0001", w.operation("op-0001", selfdev.StateApplied, first))
	// op-2 changes only the reader. Its bundle patch is cumulative from the
	// stack base (U1), so it carries op-1's change too.
	cumulative := map[string]string{draftRoot + "/App.svelte": "<p>v3</p>\n", "frontend/src/app.css": "body { color: red }\n"}
	w.operation("op-0002", selfdev.StateApplied, cumulative)

	got := wantDraftReason(t, w.draft("op-0002", nil), http.StatusConflict, pkgrelease.ReasonParentUnpublished)
	if !strings.Contains(got.Error, "op-0001") {
		t.Fatalf("L1 refusal does not name the earlier operation: %q", got.Error)
	}

	// Once the reader at op-1 is the track head (published from another
	// computer, say), op-2 drafts on it and carries only its own delta.
	headTree := strings.TrimSpace(releaseGit(t, w.repo, "rev-parse", "refs/choir/applied/"+strings.Repeat("a", 60)+"0001:"+draftRoot))
	headID := strings.Repeat("f", 64)
	record, _, blobs := decodeDraft(t, w.draft("op-0002", url.Values{"head_release_id": {headID}, "head_tree": {headTree}}))
	if record.Parent != (pkgrelease.Parent{Kind: pkgrelease.ParentRelease, ReleaseID: headID, Tree: headTree}) || record.ParentPatch != nil || blobs.ParentPatch != nil {
		t.Fatalf("draft on the head = %+v", record.Parent)
	}
	if strings.Contains(string(blobs.ReviewPatch), "app.css") || strings.Contains(string(blobs.TreePatch), "app.css") {
		t.Fatal("L1 the release carries the shell change")
	}
	if strings.Join(record.Source.ChangedPaths, ",") != draftRoot+"/App.svelte" {
		t.Fatalf("op-2's own paths = %v", record.Source.ChangedPaths)
	}
}

func TestPackageDraftRefusesKindsAndSecrets(t *testing.T) {
	w := newDraftWorld(t)
	withKinds := strings.Replace(draftManifest, `"kinds": []`, `"kinds": [{"kind": "choir.app.reader.note"}]`, 1)
	w.operation("op-kinds", selfdev.StateApplied, map[string]string{draftRoot + "/choir-package.json": withKinds})
	wantDraftReason(t, w.draft("op-kinds", nil), http.StatusUnprocessableEntity, pkgrelease.ReasonKindsNeedPlatform)
	// The secret on its own computer: after op-kinds without a ref, the
	// draft would refuse first (L1b).
	w = newDraftWorld(t)
	w.operation("op-secret", selfdev.StateApplied, map[string]string{draftRoot + "/App.svelte": "GITHUB_TOKEN=ghp_" + strings.Repeat("A", 36) + "\n"})
	wantDraftReason(t, w.draft("op-secret", nil), http.StatusUnprocessableEntity, pkgrelease.ReasonSecretFound)
}

func TestPackageDraftRefusesWhenThePrecedingOperationLeftNoRef(t *testing.T) {
	w := newDraftWorld(t)
	// op-1 changed the reader and the shell and left its projection ref.
	first := map[string]string{draftRoot + "/App.svelte": "<p>v2</p>\n", "frontend/src/app.css": "body { color: red }\n"}
	projectApplied(t, w.repo, w.base, "op-0001", w.operation("op-0001", selfdev.StateApplied, first))
	// op-2 changed the reader again; the projection never left its ref.
	w.operation("op-0002", selfdev.StateApplied, map[string]string{draftRoot + "/App.svelte": "<p>v3</p>\n", "frontend/src/app.css": "body { color: red }\n"})
	// op-3 changes the reader once more (cumulative from the stack base).
	w.operation("op-0003", selfdev.StateApplied, map[string]string{draftRoot + "/App.svelte": "<p>v4</p>\n", "frontend/src/app.css": "body { color: red }\n"})

	// The reader at op-1 is the track head. Measured from op-1's ref, op-3
	// would carry op-2's unpublished change to the reader as its own.
	headTree := strings.TrimSpace(releaseGit(t, w.repo, "rev-parse", "refs/choir/applied/"+strings.Repeat("a", 60)+"0001:"+draftRoot))
	got := wantDraftReason(t, w.draft("op-0003", url.Values{"head_release_id": {strings.Repeat("f", 64)}, "head_tree": {headTree}}), http.StatusConflict, pkgrelease.ReasonParentUnpublished)
	if !strings.Contains(got.Error, "op-0002") {
		t.Fatalf("L1b refusal does not name the operation without a ref: %q", got.Error)
	}
	// And with no head: the base cannot stand in either.
	wantDraftReason(t, w.draft("op-0003", nil), http.StatusConflict, pkgrelease.ReasonParentUnpublished)
}

// Library S3 (design 17 S3): the share gate is the guest's rollout switch,
// as packages.publish is for the draft. Failure modes pinned:
//
//	G6   the gate answers yes with packages.share off (the default), so a
//	     share, withdraw or reinstate act would reach corpusd from a stock
//	     computer; or the gate writes. Off must refuse packages_share_off;
//	N1   (S2 residual bunch279-N1) a store or ref read failure while
//	     drafting answers 409, as if it were a conflict; it is 503.
func TestPackageShareGateNeedsTheSetting(t *testing.T) {
	w := newDraftWorld(t)
	gate := func(method string) *httptest.ResponseRecorder {
		return releaseRouterCall(t, w.h, method, "/api/computers/"+draftComputer+"/packages/share-gate", "owner-a", draftComputer, nil)
	}
	wantDraftReason(t, gate(http.MethodGet), http.StatusConflict, "packages_share_off")
	if err := w.rt.store.PutComputerSetting(context.Background(), store.ComputerSetting{OwnerID: "owner-a", ComputerID: draftComputer, Key: store.SettingPackagesShare, Value: true}); err != nil {
		t.Fatal(err)
	}
	if rec := gate(http.MethodGet); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), store.SettingPackagesShare) {
		t.Fatalf("G6 the gate on = %d %s", rec.Code, rec.Body.String())
	}
	if rec := gate(http.MethodPost); rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("G6 a gate write = %d, want 405", rec.Code)
	}
	// packages.publish does not open the share gate, nor the reverse.
	if store.SettingPackagesShare == store.SettingPackagesPublish {
		t.Fatal("the share switch is the publish switch")
	}
}

func TestPackageDraftReadFailureIsUnavailable(t *testing.T) {
	w := newDraftWorld(t)
	w.operation("op-0001", selfdev.StateApplied, map[string]string{draftRoot + "/App.svelte": "<p>v2</p>\n"})
	w.operation("op-0002", selfdev.StateApplied, map[string]string{draftRoot + "/App.svelte": "<p>v3</p>\n"})
	// The source repository is gone: the applied refs cannot be read.
	w.rt.releaseSourceDir = t.TempDir()
	wantDraftReason(t, w.draft("op-0002", nil), http.StatusServiceUnavailable, "tree_unavailable")
}
