package platform

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/owneract"
	"github.com/yusefmosiah/go-choir/internal/pkgrelease"
	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

// Library S2 at corpusd (docs/design/library-package-propagation-2026-10-26.md
// 3.2, 3.4, 4.1, 4.2, 13, 14, 16), written before the code. Failure modes
// pinned here:
//
//	L1   a release whose parent is neither the base nor the track head is
//	     listed (it would carry the publisher's earlier, unpublished change);
//	L2   a forged tree_patch outside the root is stored or listed;
//	L4   another owner sees an owner-scoped listing, record or catalog entry
//	     (a single read must 404 like a missing one; a list is 200 and empty);
//	L6   a guest publishes, or writes either new kind through the generic
//	     object routes, or a publish lists without the owner's act receipt
//	     when the gate enforces;
//	L23  a withdrawn release is a track head;
//	L27  an oversize blob upload is stored, or refused without its size;
//	L33  rows of the new kinds disturb the routes an older binary serves
//	     (the skip test);
//	T1   track_moved: a stale parent (base after a first release, or a
//	     release that is no longer the head) is listed;
//	T2   seq is not 1, 2, 3 per (package, track), or a republish takes a
//	     new one;
//	B1   a base commit the host object store does not hold is believed
//	     (package_parent_unverifiable), and an unconfigured store is too;
//	H1   the store's shallow flag is missing from the health detail;
//	A1   a corpusd with no owner-act authority publishes (review 274 F3);
//	C1   a record stored without its listing is lost silently: it must be
//	     listed on republish while it can still be the head, and refused
//	     by name once the track moved; a listing that is not stored is
//	     reported as published (review 274 F6, Devin F7);
//	C2   past the listing cap a track's seq is counted from a truncated
//	     read (review 274 F7).

const pkgRoot = "frontend/src/packages/reader"

const pkgManifest = `{
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

func pkgGit(t *testing.T, dir string, env []string, stdin string, args ...string) string {
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

type pkgWorld struct {
	t      *testing.T
	h      *Handler
	repo   string
	base   string
	signer *owneract.Signer
}

// built is one release the way a publisher drafts it.
type built struct {
	id     string
	record pkgrelease.Record
	raw    []byte
	blobs  pkgrelease.Blobs
}

func newPkgWorld(t *testing.T) *pkgWorld {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	h, _ := newReleaseHandler(t)
	repo := t.TempDir()
	pkgGit(t, repo, nil, "", "init", "-q")
	for path, content := range map[string]string{
		pkgRoot + "/choir-package.json": pkgManifest,
		pkgRoot + "/App.svelte":         "<p>reader</p>\n",
		"frontend/src/app.css":          "body {}\n",
	} {
		full := filepath.Join(repo, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	pkgGit(t, repo, nil, "", "add", "-A")
	pkgGit(t, repo, nil, "", "commit", "-q", "-m", "base")
	w := &pkgWorld{t: t, h: h, repo: repo, base: pkgGit(t, repo, nil, "", "rev-parse", "HEAD")}
	h.ConfigurePackageBase(context.Background(), repo)
	// corpusd always configures an owner-act authority (cmd/corpusd); the
	// world runs it in observe with no key, so a publish without a receipt
	// lists as owner_act:unverified.
	h.ConfigurePackageOwnerActs(NewOwnerActAuthority(OwnerActObserve, nil))
	return w
}

// fullTree is the base tree with files replaced, written to the repo's
// objects only.
func (w *pkgWorld) fullTree(files map[string]string) string {
	t := w.t
	t.Helper()
	env := []string{"GIT_INDEX_FILE=" + filepath.Join(t.TempDir(), "index")}
	pkgGit(t, w.repo, env, "", "read-tree", w.base)
	for path, content := range files {
		sha := pkgGit(t, w.repo, nil, content, "hash-object", "-w", "--stdin")
		pkgGit(t, w.repo, env, "", "update-index", "--add", "--cacheinfo", "100644,"+sha+","+path)
	}
	return pkgGit(t, w.repo, env, "", "write-tree")
}

func (w *pkgWorld) subtree(full string) string {
	w.t.Helper()
	tree, present, err := pkgrelease.Subtree(context.Background(), w.repo, full, pkgRoot)
	if err != nil || !present {
		w.t.Fatalf("subtree: %v %v", present, err)
	}
	return tree
}

func pkgBlob(raw []byte) *pkgrelease.Blob {
	if raw == nil {
		return nil
	}
	return &pkgrelease.Blob{SHA256: computerevent.DigestBytes(raw), Bytes: len(raw)}
}

// build drafts a release of the reader with App.svelte set to content, on
// the base (parent nil) or on a parent release, published by owner.
func (w *pkgWorld) build(content, owner string, parent *built) built {
	t := w.t
	t.Helper()
	full := w.fullTree(map[string]string{pkgRoot + "/App.svelte": content})
	tree := w.subtree(full)
	parentTree := w.subtree(w.base)
	if parent != nil {
		parentTree = parent.record.Source.Tree
	}
	blobs, err := pkgrelease.BuildBlobs(context.Background(), w.repo, pkgRoot, parentTree, tree)
	if err != nil {
		t.Fatal(err)
	}
	manifest, version, err := pkgrelease.CheckManifest("reader", []byte(pkgManifest))
	if err != nil {
		t.Fatal(err)
	}
	record := pkgrelease.Record{
		Schema: pkgrelease.Schema, Package: pkgrelease.Package{Name: "reader", ManifestVersion: version}, Track: owner,
		Parent: pkgrelease.Parent{Kind: pkgrelease.ParentBase, Commit: w.base, Tree: parentTree},
		Source: pkgrelease.Source{
			Root: pkgRoot, Tree: tree, TreePatch: *pkgBlob(blobs.TreePatch), ReviewPatch: *pkgBlob(blobs.ReviewPatch),
			ChangedPaths: []string{pkgRoot + "/App.svelte"}, BuiltOn: w.base,
		},
		Manifest:  manifest,
		Publisher: pkgrelease.Publisher{OwnerID: owner, ComputerID: computerOf(owner)},
		Operation: pkgrelease.Operation{OperationID: "op-" + content[:4], BundleDigest: strings.Repeat("b", 64), DecisionEvent: strings.Repeat("e", 64)},
	}
	if parent != nil {
		record.Parent = pkgrelease.Parent{Kind: pkgrelease.ParentRelease, ReleaseID: parent.id, Tree: parentTree}
		blobs.ParentPatch = nil
	} else {
		record.ParentPatch = pkgBlob(blobs.ParentPatch)
	}
	return w.seal(record, blobs)
}

func (w *pkgWorld) seal(record pkgrelease.Record, blobs pkgrelease.Blobs) built {
	w.t.Helper()
	id, err := record.ReleaseID()
	if err != nil {
		w.t.Fatal(err)
	}
	raw, err := record.Canonical()
	if err != nil {
		w.t.Fatal(err)
	}
	return built{id: id, record: record, raw: raw, blobs: blobs}
}

func computerOf(owner string) string { return "computer-" + strings.TrimPrefix(owner, "owner-") + "1" }

type pkgCall struct {
	method, path, remote, receipt string
	internal                      bool
	body                          []byte
}

func (w *pkgWorld) do(c pkgCall) *httptest.ResponseRecorder {
	r := httptest.NewRequest(c.method, c.path, bytes.NewReader(c.body))
	if c.remote != "" {
		r.RemoteAddr = c.remote
	}
	if c.internal {
		r.Header.Set("X-Internal-Caller", "true")
	}
	if c.receipt != "" {
		r.Header.Set(owneract.Header, c.receipt)
	}
	rec := httptest.NewRecorder()
	w.h.HandlePackages(rec, r)
	return rec
}

func (w *pkgWorld) putBlobs(b built) {
	w.t.Helper()
	for _, raw := range [][]byte{b.blobs.TreePatch, b.blobs.ReviewPatch, b.blobs.ParentPatch} {
		if raw == nil {
			continue
		}
		if rec := w.do(pkgCall{method: http.MethodPut, path: packagesRoute + "/blobs/" + computerevent.DigestBytes(raw), internal: true, body: raw}); rec.Code != http.StatusCreated {
			w.t.Fatalf("blob upload = %d %s", rec.Code, rec.Body.String())
		}
	}
}

func (w *pkgWorld) receiptFor(b built) string {
	w.t.Helper()
	if w.signer == nil {
		return ""
	}
	receipt, err := w.signer.Mint(owneract.Act{
		Kind: owneract.ActPackagePublish, OwnerUserID: b.record.Publisher.OwnerID, AuthMethod: owneract.AuthMethodSession,
		CredentialRef: "session:test", ComputerID: b.record.Publisher.ComputerID, Subject: PackagePublishSubject(b.id),
	}, time.Now())
	if err != nil {
		w.t.Fatal(err)
	}
	encoded, err := owneract.Encode(receipt)
	if err != nil {
		w.t.Fatal(err)
	}
	return encoded
}

func (w *pkgWorld) publishWith(b built, receipt string) *httptest.ResponseRecorder {
	w.t.Helper()
	w.putBlobs(b)
	body, _ := json.Marshal(map[string]any{
		"release_id": b.id, "record": json.RawMessage(b.raw),
		"owner_id": b.record.Publisher.OwnerID, "computer_id": b.record.Publisher.ComputerID,
	})
	return w.do(pkgCall{method: http.MethodPost, path: packagesRoute + "/releases", internal: true, receipt: receipt, body: body})
}

func (w *pkgWorld) publish(b built) *httptest.ResponseRecorder {
	w.t.Helper()
	return w.publishWith(b, w.receiptFor(b))
}

func (w *pkgWorld) published(b built) PackageReleaseView {
	w.t.Helper()
	rec := w.publish(b)
	var view PackageReleaseView
	if (rec.Code != http.StatusCreated && rec.Code != http.StatusOK) || json.Unmarshal(rec.Body.Bytes(), &view) != nil {
		w.t.Fatalf("publish = %d %s", rec.Code, rec.Body.String())
	}
	return view
}

type pkgRefusal struct {
	Error  string `json:"error"`
	Reason string `json:"reason"`
}

func wantPkgRefusal(t *testing.T, rec *httptest.ResponseRecorder, status int, reason string) pkgRefusal {
	t.Helper()
	var got pkgRefusal
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if rec.Code != status || got.Reason != reason {
		t.Fatalf("= %d %s, want %d %s", rec.Code, rec.Body.String(), status, reason)
	}
	return got
}

func (w *pkgWorld) read(path string) *httptest.ResponseRecorder {
	return w.do(pkgCall{method: http.MethodGet, path: path, internal: true})
}

func TestPackageRoutesAreHostOnly(t *testing.T) {
	w := newPkgWorld(t)
	b := w.build("<p>v2</p>\n", "owner-a", nil)
	body, _ := json.Marshal(map[string]any{"release_id": b.id, "record": json.RawMessage(b.raw), "owner_id": "owner-a", "computer_id": "computer-a1"})
	sha := computerevent.DigestBytes(b.blobs.TreePatch)
	for _, c := range []pkgCall{
		{method: http.MethodPost, path: packagesRoute + "/releases", remote: guestTap, body: body},
		{method: http.MethodPut, path: packagesRoute + "/blobs/" + sha, remote: guestTap, body: b.blobs.TreePatch},
		{method: http.MethodGet, path: packagesRoute + "/catalog?owner_id=owner-a", remote: guestTap},
		{method: http.MethodGet, path: packagesRoute + "/releases/" + b.id + "?owner_id=owner-a", remote: guestTap},
		{method: http.MethodGet, path: packagesRoute + "/tracks/reader/head?owner_id=owner-a", remote: guestTap},
	} {
		if rec := w.do(c); rec.Code != http.StatusForbidden {
			t.Fatalf("L6 guest %s %s = %d, want 403", c.method, c.path, rec.Code)
		}
	}
}

func TestPackagePublishListsAtOwnerScope(t *testing.T) {
	w := newPkgWorld(t)
	b := w.build("<p>v2</p>\n", "owner-a", nil)
	rec := w.publish(b)
	var view PackageReleaseView
	if rec.Code != http.StatusCreated || json.Unmarshal(rec.Body.Bytes(), &view) != nil {
		t.Fatalf("publish = %d %s", rec.Code, rec.Body.String())
	}
	l := view.Listing
	if view.ReleaseID != b.id || !view.Published || l.RecordKind != pkgrelease.RecordKindPackageRelease || l.RecordID != b.id ||
		l.Package != "reader" || l.Track != "owner-a" || l.Seq != 1 || strings.Join(l.Scopes, ",") != pkgrelease.ScopeOwner ||
		l.State != pkgrelease.StateListed || l.Version != 1 || l.PrevVersionDigest != "" || !strings.HasPrefix(l.StateActor, "owner_act:") {
		t.Fatalf("listing = %+v", view)
	}
	// The stored record is the draft byte for byte.
	if !bytes.Equal(view.Record, b.raw) {
		t.Fatal("the stored record is not the published bytes")
	}

	// Reads: the owner sees it; another owner sees nothing (L4).
	if rec := w.read(packagesRoute + "/releases/" + b.id + "?owner_id=owner-a"); rec.Code != http.StatusOK {
		t.Fatalf("owner read = %d %s", rec.Code, rec.Body.String())
	}
	other := w.read(packagesRoute + "/releases/" + b.id + "?owner_id=owner-b")
	missing := w.read(packagesRoute + "/releases/" + strings.Repeat("0", 64) + "?owner_id=owner-b")
	if other.Code != http.StatusNotFound || other.Body.String() != missing.Body.String() {
		t.Fatalf("L4 another owner's read = %d %s, missing = %s", other.Code, other.Body.String(), missing.Body.String())
	}
	var catalog struct {
		Packages []struct {
			Package       string `json:"package"`
			Track         string `json:"track"`
			HeadReleaseID string `json:"head_release_id"`
			HeadSeq       int    `json:"head_seq"`
		} `json:"packages"`
	}
	if rec := w.read(packagesRoute + "/catalog?owner_id=owner-a"); rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &catalog) != nil ||
		len(catalog.Packages) != 1 || catalog.Packages[0].Package != "reader" || catalog.Packages[0].HeadReleaseID != b.id || catalog.Packages[0].HeadSeq != 1 {
		t.Fatalf("owner catalog = %d %s", rec.Code, rec.Body.String())
	}
	if rec := w.read(packagesRoute + "/catalog?owner_id=owner-a&q=read"); !strings.Contains(rec.Body.String(), b.id) {
		t.Fatalf("search by name = %s", rec.Body.String())
	}
	if rec := w.read(packagesRoute + "/catalog?owner_id=owner-a&q=image"); strings.Contains(rec.Body.String(), b.id) {
		t.Fatalf("search for another name found reader: %s", rec.Body.String())
	}
	if rec := w.read(packagesRoute + "/catalog?owner_id=owner-b"); rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), b.id) {
		t.Fatalf("L4 another owner's catalog = %d %s, want 200 and empty", rec.Code, rec.Body.String())
	}
	if rec := w.read(packagesRoute + "/catalog/reader/releases?owner_id=owner-a"); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), b.id) {
		t.Fatalf("owner history = %d %s", rec.Code, rec.Body.String())
	}
	if rec := w.read(packagesRoute + "/catalog/reader/releases?owner_id=owner-b"); rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), b.id) {
		t.Fatalf("L4 another owner's history = %d %s", rec.Code, rec.Body.String())
	}
	if rec := w.read(packagesRoute + "/tracks/reader/head?owner_id=owner-b"); rec.Code != http.StatusNotFound {
		t.Fatalf("L4 another owner's track head = %d", rec.Code)
	}
	// A read that names no owner is refused, never unscoped.
	for _, path := range []string{"/catalog", "/catalog/reader/releases", "/releases/" + b.id, "/tracks/reader/head"} {
		if rec := w.read(packagesRoute + path); rec.Code != http.StatusBadRequest {
			t.Fatalf("%s with no owner = %d, want 400", path, rec.Code)
		}
	}
}

func TestPackageRepublishIsIdempotent(t *testing.T) {
	w := newPkgWorld(t)
	b := w.build("<p>v2</p>\n", "owner-a", nil)
	first := w.published(b)
	rec := w.publish(b)
	var again PackageReleaseView
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &again) != nil || again.Published || again.Listing.Seq != 1 || again.Listing.Version != 1 {
		t.Fatalf("T2 republish = %d %s", rec.Code, rec.Body.String())
	}
	if again.CanonicalID != first.CanonicalID {
		t.Fatal("a republish named another object")
	}
}

func TestPackageTrackHasOneLinearHistory(t *testing.T) {
	w := newPkgWorld(t)
	r1 := w.build("<p>v2</p>\n", "owner-a", nil)
	if v := w.published(r1); v.Listing.Seq != 1 {
		t.Fatalf("first seq = %d", v.Listing.Seq)
	}
	// T1: a second release on the base after the track has a head.
	fork := w.build("<p>fork</p>\n", "owner-a", nil)
	wantPkgRefusal(t, w.publish(fork), http.StatusConflict, pkgrelease.ReasonTrackMoved)

	r2 := w.build("<p>v3</p>\n", "owner-a", &r1)
	if v := w.published(r2); v.Listing.Seq != 2 {
		t.Fatalf("second seq = %d", v.Listing.Seq)
	}
	// T1: a release on r1, which is no longer the head.
	stale := w.build("<p>v3b</p>\n", "owner-a", &r1)
	wantPkgRefusal(t, w.publish(stale), http.StatusConflict, pkgrelease.ReasonTrackMoved)
	if rec := w.read(packagesRoute + "/tracks/reader/head?owner_id=owner-a"); !strings.Contains(rec.Body.String(), r2.id) {
		t.Fatalf("head = %s, want r2", rec.Body.String())
	}

	// Another owner's track is its own line: its first release is on the base.
	b1 := w.build("<p>b</p>\n", "owner-b", nil)
	if v := w.published(b1); v.Listing.Seq != 1 || v.Listing.Track != "owner-b" {
		t.Fatalf("owner-b listing = %+v", v.Listing)
	}
	// L1: a parent that is another owner's release is not on this track.
	cross := w.build("<p>cross</p>\n", "owner-a", &b1)
	wantPkgRefusal(t, w.publish(cross), http.StatusConflict, pkgrelease.ReasonTrackMoved)
}

func TestWithdrawnReleaseIsNoTrackHead(t *testing.T) {
	w := newPkgWorld(t)
	r1 := w.build("<p>v2</p>\n", "owner-a", nil)
	w.published(r1)
	r2 := w.build("<p>v3</p>\n", "owner-a", &r1)
	v2 := w.published(r2)

	// The publisher withdraws r2 (S3's act; written here as corpusd would).
	withdrawn := v2.Listing
	prev, err := v2.Listing.Digest()
	if err != nil {
		t.Fatal(err)
	}
	withdrawn.Version, withdrawn.PrevVersionDigest = 2, prev
	withdrawn.State, withdrawn.StateReason, withdrawn.StateActor = pkgrelease.StateWithdrawn, "withdrawn", "owner_act:test"
	obj, err := listingObject("owner-a", "computer-a1", withdrawn, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.h.releaseStore().InsertObjectIfAbsent(context.Background(), obj); err != nil {
		t.Fatal(err)
	}

	// L23: r2 is no parent; r1 is the head again, and the next seq is 3.
	onWithdrawn := w.build("<p>v4</p>\n", "owner-a", &r2)
	wantPkgRefusal(t, w.publish(onWithdrawn), http.StatusConflict, pkgrelease.ReasonTrackMoved)
	r3 := w.build("<p>v4b</p>\n", "owner-a", &r1)
	if v := w.published(r3); v.Listing.Seq != 3 {
		t.Fatalf("seq after a withdrawn head = %d, want 3", v.Listing.Seq)
	}
	if rec := w.read(packagesRoute + "/releases/" + r2.id + "?owner_id=owner-a"); !strings.Contains(rec.Body.String(), `"state":"withdrawn"`) {
		t.Fatalf("a withdrawn release reads its current listing: %s", rec.Body.String())
	}
}

func TestPackagePublishRefusesForgedContent(t *testing.T) {
	w := newPkgWorld(t)
	b := w.build("<p>v2</p>\n", "owner-a", nil)
	full := w.fullTree(map[string]string{pkgRoot + "/App.svelte": "<p>v2</p>\n", "frontend/src/app.css": "body { color: red }\n"})
	forged := []byte(pkgGit(t, w.repo, nil, "", "diff-tree", "-p", "-r", "--binary", "--full-index", "--no-renames", "--no-color", pkgrelease.EmptyTree, full) + "\n")
	record := b.record
	record.Source.TreePatch = *pkgBlob(forged)
	blobs := b.blobs
	blobs.TreePatch = forged
	bad := w.seal(record, blobs)
	got := wantPkgRefusal(t, w.publish(bad), http.StatusUnprocessableEntity, pkgrelease.ReasonContentNotConfined)
	if !strings.Contains(got.Error, "frontend/src/app.css") {
		t.Fatalf("L2 refusal does not name the path: %s", got.Error)
	}
	if rec := w.read(packagesRoute + "/releases/" + bad.id + "?owner_id=owner-a"); rec.Code != http.StatusNotFound {
		t.Fatalf("L2 a refused release was stored: %d", rec.Code)
	}
	if rec := w.read(packagesRoute + "/catalog?owner_id=owner-a"); strings.Contains(rec.Body.String(), bad.id) {
		t.Fatalf("L2 a refused release was listed: %s", rec.Body.String())
	}
}

func TestPackagePublishRefusesABaseTheHostDoesNotHold(t *testing.T) {
	w := newPkgWorld(t)
	b := w.build("<p>v2</p>\n", "owner-a", nil)
	record := b.record
	record.Parent.Commit = strings.Repeat("7", 40)
	wantPkgRefusal(t, w.publish(w.seal(record, b.blobs)), http.StatusConflict, pkgrelease.ReasonParentUnverifiable)

	// B1: a corpusd with no base store believes no base parent.
	w.h.ConfigurePackageBase(context.Background(), "")
	wantPkgRefusal(t, w.publish(b), http.StatusConflict, pkgrelease.ReasonParentUnverifiable)
}

func TestPackagePublishBindsThePublisherAndTheBlobs(t *testing.T) {
	w := newPkgWorld(t)
	b := w.build("<p>v2</p>\n", "owner-a", nil)
	w.putBlobs(b)
	body, _ := json.Marshal(map[string]any{"release_id": b.id, "record": json.RawMessage(b.raw), "owner_id": "owner-b", "computer_id": "computer-a1"})
	if rec := w.do(pkgCall{method: http.MethodPost, path: packagesRoute + "/releases", internal: true, body: body}); rec.Code != http.StatusForbidden {
		t.Fatalf("a publisher that is not the authorized owner = %d %s", rec.Code, rec.Body.String())
	}
	body, _ = json.Marshal(map[string]any{"release_id": strings.Repeat("0", 64), "record": json.RawMessage(b.raw), "owner_id": "owner-a", "computer_id": "computer-a1"})
	wantPkgRefusal(t, w.do(pkgCall{method: http.MethodPost, path: packagesRoute + "/releases", internal: true, body: body}), http.StatusUnprocessableEntity, pkgrelease.ReasonInvalid)

	// A record naming a blob the store does not hold.
	other := w.build("<p>never uploaded</p>\n", "owner-a", nil)
	body, _ = json.Marshal(map[string]any{"release_id": other.id, "record": json.RawMessage(other.raw), "owner_id": "owner-a", "computer_id": "computer-a1"})
	wantPkgRefusal(t, w.do(pkgCall{method: http.MethodPost, path: packagesRoute + "/releases", internal: true, body: body}), http.StatusConflict, pkgrelease.ReasonBlobMismatch)
}

func TestPackageBlobUploadCapNamesTheSize(t *testing.T) {
	w := newPkgWorld(t)
	big := bytes.Repeat([]byte("x"), pkgrelease.MaxBlobBytes+1)
	rec := w.do(pkgCall{method: http.MethodPut, path: packagesRoute + "/blobs/" + computerevent.DigestBytes(big), internal: true, body: big})
	got := wantPkgRefusal(t, rec, http.StatusRequestEntityTooLarge, pkgrelease.ReasonBlobTooLarge)
	if !strings.Contains(got.Error, "2097153") {
		t.Fatalf("L27 refusal does not name the size: %s", got.Error)
	}
	if matches, _ := filepath.Glob(filepath.Join(w.h.service.artifactsRoot, "sha256", pkgrelease.ContentNamespace, "*")); len(matches) != 0 {
		t.Fatalf("L27 an oversize blob left %v", matches)
	}
	if rec := w.do(pkgCall{method: http.MethodPut, path: packagesRoute + "/blobs/" + strings.Repeat("0", 64), internal: true, body: []byte("abc")}); rec.Code != http.StatusBadRequest {
		t.Fatalf("a blob that does not hash to its name = %d", rec.Code)
	}
}

func TestPackagePublishVerifiesTheOwnersActReceipt(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := owneract.NewSigner(private)
	if err != nil {
		t.Fatal(err)
	}
	verifier, err := owneract.NewVerifier(public)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("enforce", func(t *testing.T) {
		w := newPkgWorld(t)
		w.h.ConfigurePackageOwnerActs(NewOwnerActAuthority(OwnerActEnforce, verifier))
		b := w.build("<p>v2</p>\n", "owner-a", nil)
		if rec := w.publishWith(b, ""); rec.Code != http.StatusForbidden {
			t.Fatalf("L6 no receipt under enforce = %d %s", rec.Code, rec.Body.String())
		}
		w.signer = signer
		other := w.build("<p>other</p>\n", "owner-a", nil)
		if rec := w.publishWith(b, w.receiptFor(other)); rec.Code != http.StatusForbidden {
			t.Fatalf("L6 a receipt for another release = %d %s", rec.Code, rec.Body.String())
		}
		view := w.published(b)
		if view.Listing.StateActor == "owner_act:unverified" || !strings.HasPrefix(view.Listing.StateActor, "owner_act:") {
			t.Fatalf("state actor = %q, want the receipt digest", view.Listing.StateActor)
		}
	})
	t.Run("observe", func(t *testing.T) {
		w := newPkgWorld(t)
		w.h.ConfigurePackageOwnerActs(NewOwnerActAuthority(OwnerActObserve, verifier))
		view := w.published(w.build("<p>v2</p>\n", "owner-a", nil))
		if view.Listing.StateActor != "owner_act:unverified" {
			t.Fatalf("observe without a receipt: state actor = %q", view.Listing.StateActor)
		}
	})
}

// L6: a bound guest cannot write either kind through the generic routes.
func TestGenericObjectRoutesRefusePackageKinds(t *testing.T) {
	mem := objectgraph.NewMemoryStore()
	svc := objectgraph.NewService(objectgraph.Config{Memory: mem, Durable: mem})
	defer svc.Close()
	h := NewObjectGraphHandler(svc, mem)
	if err := h.ConfigureGuestBinding(fakeGuestLookup{"10.200.1.2": {Found: true, UserID: "owner-a", ComputerID: "computer-a1"}}); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []objectgraph.ObjectKind{PackageReleaseKind, ReleaseListingKind} {
		raw, _ := json.Marshal(map[string]any{"kind": kind, "owner_id": "owner-a", "computer_id": "computer-a1", "identity_key": "k", "metadata": map[string]any{}})
		req := httptest.NewRequest(http.MethodPost, "/internal/platform/objects", bytes.NewReader(raw))
		req.RemoteAddr = "10.200.1.2:40000"
		rec := httptest.NewRecorder()
		h.HandleObjects(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("guest create of %s = %d, want 403", kind, rec.Code)
		}
		id, _ := objectgraph.BuildCanonicalID(kind, "owner-a", "forged")
		obj := objectgraph.Object{CanonicalID: id, ObjectKind: kind, OwnerID: "owner-a", ComputerID: "computer-a1", Body: []byte("{}"), Metadata: json.RawMessage(`{}`)}
		raw, _ = json.Marshal(obj)
		req = httptest.NewRequest(http.MethodPut, "/internal/platform/objects", bytes.NewReader(raw))
		req.RemoteAddr = "10.200.1.2:40000"
		rec = httptest.NewRecorder()
		h.HandleObjects(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("guest put of %s = %d, want 403", kind, rec.Code)
		}
	}
	_ = vmctl.GuestOwnership{}
}

func TestPackageKindsAreRegisteredForCorpusd(t *testing.T) {
	if string(PackageReleaseKind) != pkgrelease.ObjectKind || string(ReleaseListingKind) != pkgrelease.ListingKind {
		t.Fatal("the platform kinds and the pkgrelease kinds disagree")
	}
	for _, kind := range []objectgraph.ObjectKind{PackageReleaseKind, ReleaseListingKind} {
		reg, err := objectgraph.DefaultRegistry().LookupKind(kind)
		if err != nil || reg.Writer != objectgraph.WriterCorpusd || reg.IdentityMode != objectgraph.IdentityExternalKey {
			t.Fatalf("%s registration = %+v %v", kind, reg, err)
		}
	}
}

// L33, the skip test (design 13): rows of the new kinds are inert to the
// routes an older binary serves. Its release list, single read and the
// generic object list answer as before, and the store opens with them.
func TestPackageRowsAreInertToTheReleaseRoutes(t *testing.T) {
	w := newPkgWorld(t)
	w.published(w.build("<p>v2</p>\n", "owner-a", nil))
	if rec := (releaseCall{method: http.MethodGet, path: "/internal/platform/releases?limit=50", internal: true}).do(w.h); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"count":0`) {
		t.Fatalf("the release list beside package rows = %d %s", rec.Code, rec.Body.String())
	}
	if rec := (releaseCall{method: http.MethodGet, path: "/internal/platform/releases/" + strings.Repeat("0", 64) + "?computer_id=host", internal: true}).do(w.h); rec.Code != http.StatusNotFound {
		t.Fatalf("a release read beside package rows = %d", rec.Code)
	}
	objects, err := w.h.releaseStore().ListObjects(context.Background(), objectgraph.ListFilter{OwnerID: "owner-a", Limit: 50})
	if err != nil || len(objects) < 2 {
		t.Fatalf("the generic list beside package rows = %d %v", len(objects), err)
	}
	if err := w.h.service.Health(context.Background()); err != nil {
		t.Fatalf("store health beside package rows: %v", err)
	}
}

func TestHealthReportsThePackageBaseShallowFlag(t *testing.T) {
	w := newPkgWorld(t)
	health := func() healthResponse {
		rec := httptest.NewRecorder()
		w.h.HandleHealth(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
		var got healthResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		return got
	}
	if got := health(); got.Packages == nil || got.Packages.Shallow == nil || *got.Packages.Shallow || got.Packages.Store != w.repo {
		t.Fatalf("H1 full store = %+v", got.Packages)
	}
	clone := t.TempDir()
	pkgGit(t, clone, nil, "", "clone", "-q", "--depth", "1", "file://"+w.repo, ".")
	w.h.ConfigurePackageBase(context.Background(), clone)
	if got := health(); got.Packages == nil || got.Packages.Shallow == nil || !*got.Packages.Shallow {
		t.Fatalf("H1 shallow store = %+v", got.Packages)
	}
	w.h.ConfigurePackageBase(context.Background(), "")
	if got := health(); got.Packages == nil || got.Packages.Shallow != nil || got.Packages.Error == "" {
		t.Fatalf("H1 unconfigured store = %+v", got.Packages)
	}
}

func TestPackagePublishRefusesWithoutAnOwnerActAuthority(t *testing.T) {
	w := newPkgWorld(t)
	w.h.ConfigurePackageOwnerActs(nil)
	b := w.build("<p>v2</p>\n", "owner-a", nil)
	wantPkgRefusal(t, w.publish(b), http.StatusServiceUnavailable, "owner_act_unconfigured")
	if rec := w.read(packagesRoute + "/releases/" + b.id + "?owner_id=owner-a"); rec.Code != http.StatusNotFound {
		t.Fatalf("A1 a refused publish stored the release: %d", rec.Code)
	}
}

// storeRecordOnly files b's record the way publish does and stops before
// the listing: a crash between the two writes.
func (w *pkgWorld) storeRecordOnly(b built) {
	w.t.Helper()
	w.putBlobs(b)
	id, err := packageReleaseCanonicalID(b.record.Publisher.OwnerID, b.id)
	if err != nil {
		w.t.Fatal(err)
	}
	metadata, err := objectgraph.NormalizeMetadata(map[string]any{"release_id": b.id, "package": b.record.Package.Name, "track": b.record.Track})
	if err != nil {
		w.t.Fatal(err)
	}
	now := time.Now().UTC()
	obj := objectgraph.Object{
		CanonicalID: id, ObjectKind: PackageReleaseKind, OwnerID: b.record.Publisher.OwnerID, ComputerID: b.record.Publisher.ComputerID,
		VersionID: b.id, Body: b.raw, Metadata: metadata, ContentHash: objectgraph.ContentHash(PackageReleaseKind, b.raw, metadata), CreatedAt: now, UpdatedAt: now,
	}
	if inserted, err := w.h.releaseStore().InsertObjectIfAbsent(context.Background(), obj); err != nil || !inserted {
		w.t.Fatalf("store record only: %v %v", inserted, err)
	}
}

func TestPackageRecordWithoutItsListingIsListedOrRefusedByName(t *testing.T) {
	w := newPkgWorld(t)
	r1 := w.build("<p>v2</p>\n", "owner-a", nil)
	w.storeRecordOnly(r1)
	// C1: still head-eligible, the republish lists it.
	if v := w.published(r1); !v.Published || v.Listing.Seq != 1 {
		t.Fatalf("C1 republish of an unlisted record = %+v", v)
	}
	// r2's record is stored unlisted; meanwhile r2b takes the head.
	r2 := w.build("<p>v3</p>\n", "owner-a", &r1)
	w.storeRecordOnly(r2)
	r2b := w.build("<p>v3b</p>\n", "owner-a", &r1)
	w.published(r2b)
	got := wantPkgRefusal(t, w.publish(r2), http.StatusConflict, "package_record_unlisted")
	if !strings.Contains(got.Error, r2b.id) {
		t.Fatalf("C1 refusal does not name the head that moved: %s", got.Error)
	}
}

func TestPackageListingThatIsNotStoredIsNoPublish(t *testing.T) {
	w := newPkgWorld(t)
	b := w.build("<p>v2</p>\n", "owner-a", nil)
	// Something already holds the listing's canonical id and is no listing.
	id, err := listingCanonicalID("owner-a", b.id, 1)
	if err != nil {
		t.Fatal(err)
	}
	metadata, _ := objectgraph.NormalizeMetadata(map[string]any{"package": "reader"})
	body := []byte(`{"not":"a listing"}`)
	now := time.Now().UTC()
	junk := objectgraph.Object{CanonicalID: id, ObjectKind: ReleaseListingKind, OwnerID: "owner-a", ComputerID: "computer-a1", Body: body, Metadata: metadata,
		ContentHash: objectgraph.ContentHash(ReleaseListingKind, body, metadata), CreatedAt: now, UpdatedAt: now}
	if _, err := w.h.releaseStore().InsertObjectIfAbsent(context.Background(), junk); err != nil {
		t.Fatal(err)
	}
	wantPkgRefusal(t, w.publish(b), http.StatusConflict, "package_listing_conflict")
	if rec := w.read(packagesRoute + "/tracks/reader/head?owner_id=owner-a"); rec.Code != http.StatusNotFound {
		t.Fatalf("a listing that was not stored heads the track: %d %s", rec.Code, rec.Body.String())
	}
}

func TestPackageTrackRefusesPastTheListingCap(t *testing.T) {
	saved := maxTrackListings
	maxTrackListings = 2
	t.Cleanup(func() { maxTrackListings = saved })
	w := newPkgWorld(t)
	r1 := w.build("<p>v2</p>\n", "owner-a", nil)
	w.published(r1)
	r2 := w.build("<p>v3</p>\n", "owner-a", &r1)
	w.published(r2)
	// C2: a third listing would be counted from a read the cap truncates.
	r3 := w.build("<p>v4</p>\n", "owner-a", &r2)
	wantPkgRefusal(t, w.publish(r3), http.StatusConflict, "track_full")
	if rec := w.read(packagesRoute + "/tracks/reader/head?owner_id=owner-a"); !strings.Contains(rec.Body.String(), r2.id) {
		t.Fatalf("head at the cap = %d %s, want r2", rec.Code, rec.Body.String())
	}
	// A read past the cap refuses rather than answer from a truncated read.
	withdrawn := w.read(packagesRoute + "/releases/" + r2.id + "?owner_id=owner-a")
	var view PackageReleaseView
	if json.Unmarshal(withdrawn.Body.Bytes(), &view) != nil {
		t.Fatalf("read r2: %s", withdrawn.Body.String())
	}
	next := view.Listing
	prev, _ := view.Listing.Digest()
	next.Version, next.PrevVersionDigest, next.State, next.StateReason, next.StateActor = 2, prev, pkgrelease.StateWithdrawn, "withdrawn", "owner_act:test"
	obj, err := listingObject("owner-a", "computer-a1", next, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.h.releaseStore().InsertObjectIfAbsent(context.Background(), obj); err != nil {
		t.Fatal(err)
	}
	wantPkgRefusal(t, w.read(packagesRoute+"/catalog/reader/releases?owner_id=owner-a"), http.StatusConflict, "track_full")
	wantPkgRefusal(t, w.read(packagesRoute+"/catalog?owner_id=owner-a"), http.StatusConflict, "track_full")
}
