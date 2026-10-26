package pkgrelease

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

// Library S2 host checks (docs/design/library-package-propagation-2026-10-26.md
// 3.2 and section 16), written before the code. Failure modes pinned:
//
//	L2   a guest-forged tree_patch reaches outside the root (frontend/src/app.css);
//	L3   a symlink, a submodule or a .gitattributes rides inside the root;
//	L25  a package declares kinds before S12;
//	L27  a blob over the cap is stored or refused without naming its size;
//	P1   source.tree is not the subtree the stored tree_patch makes;
//	P2   the review patch does not turn the parent subtree into the subtree;
//	P3   a parent_patch whose tree differs from the host's own base tree is
//	     believed (both when it disagrees with parent.tree, and when it
//	     agrees with a parent.tree the base does not hold);
//	P4   a base commit the host does not hold is believed
//	     (package_parent_unverifiable), or a host with no store believes any;
//	P5   a release parent whose stored tree is not parent.tree is believed;
//	P6   an added secret rides the tree_patch;
//	P7   a release that changes nothing is listed;
//	P8   the store's shallow flag is not reported truthfully.

const testRoot = "frontend/src/packages/reader"

const testManifest = `{
  "schema": "choir.app-package/v1",
  "name": "reader",
  "version": "1.0.0",
  "origin": "first_party",
  "display": {"title": "Reader", "icon": "R", "description": "Read"},
  "app": {"entry": "App.svelte", "abi": "choir.app-frontend/svelte-inline/v1", "aliases": [],
    "launcher": {"menu": true}, "window": {"singleton": false}, "auth": {"preview": "public-preview"}, "theme": {}},
  "opens": [{"media_type": "application/pdf", "extensions": ["pdf"]}],
  "kinds": [],
  "uses_kinds": [],
  "desk": null,
  "backend": null,
  "grants": []
}
`

func gitT(t *testing.T, dir string, env []string, stdin string, args ...string) string {
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

// fixture is a source repository with one base commit: the reader package
// and a shell file beside it.
type fixture struct {
	t    *testing.T
	repo string
	base string // base commit
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	repo := t.TempDir()
	gitT(t, repo, nil, "", "init", "-q")
	for path, content := range map[string]string{
		testRoot + "/choir-package.json": testManifest,
		testRoot + "/App.svelte":         "<p>reader</p>\n",
		"frontend/src/app.css":           "body {}\n",
	} {
		full := filepath.Join(repo, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	gitT(t, repo, nil, "", "add", "-A")
	gitT(t, repo, nil, "", "commit", "-q", "-m", "base")
	return &fixture{t: t, repo: repo, base: gitT(t, repo, nil, "", "rev-parse", "HEAD")}
}

// entry is one index entry a tree is built from: a mode and content (for a
// gitlink, the content is the commit id).
type entry struct{ mode, content string }

// tree writes the base tree with the given entries replaced or added (an
// empty content with mode "-" deletes), into the repository's objects, and
// returns the full tree id. The work tree is never touched.
func (f *fixture) tree(changes map[string]entry) string {
	t := f.t
	t.Helper()
	env := []string{"GIT_INDEX_FILE=" + filepath.Join(t.TempDir(), "index")}
	gitT(t, f.repo, env, "", "read-tree", f.base)
	for path, e := range changes {
		switch e.mode {
		case "-":
			gitT(t, f.repo, env, "", "update-index", "--force-remove", path)
		case "160000":
			gitT(t, f.repo, env, "", "update-index", "--add", "--cacheinfo", "160000,"+e.content+","+path)
		default:
			sha := gitT(t, f.repo, nil, e.content, "hash-object", "-w", "--stdin")
			gitT(t, f.repo, env, "", "update-index", "--add", "--cacheinfo", e.mode+","+sha+","+path)
		}
	}
	return gitT(t, f.repo, env, "", "write-tree")
}

func (f *fixture) subtree(full string) string {
	f.t.Helper()
	tree, present, err := Subtree(context.Background(), f.repo, full, testRoot)
	if err != nil || !present {
		f.t.Fatalf("subtree of %s: present=%v %v", full, present, err)
	}
	return tree
}

func (f *fixture) baseSubtree() string { return f.subtree(f.base) }

// release builds a valid base-parent release of the reader whose subtree is
// the one in full, with its blobs, the way a publisher does.
func (f *fixture) release(full string) (Record, Blobs) {
	t := f.t
	t.Helper()
	parent, tree := f.baseSubtree(), f.subtree(full)
	blobs, err := BuildBlobs(context.Background(), f.repo, testRoot, parent, tree)
	if err != nil {
		t.Fatal(err)
	}
	manifestRaw, err := ReadFile(context.Background(), f.repo, full, testRoot+"/choir-package.json")
	if err != nil {
		t.Fatal(err)
	}
	manifest, version, err := CheckManifest("reader", manifestRaw)
	if err != nil {
		t.Fatal(err)
	}
	record := Record{
		Schema: Schema, Package: Package{Name: "reader", ManifestVersion: version}, Track: "owner-a",
		Parent:      Parent{Kind: ParentBase, Commit: f.base, Tree: parent},
		ParentPatch: blobOf(blobs.ParentPatch),
		Source: Source{
			Root: testRoot, Tree: tree, TreePatch: *blobOf(blobs.TreePatch), ReviewPatch: *blobOf(blobs.ReviewPatch),
			ChangedPaths: []string{testRoot + "/App.svelte"}, BuiltOn: f.base,
		},
		Manifest:  manifest,
		Publisher: Publisher{OwnerID: "owner-a", ComputerID: "computer-a1"},
		Operation: Operation{OperationID: "op-1", BundleDigest: strings.Repeat("b", 64), DecisionEvent: strings.Repeat("e", 64)},
	}
	return record, blobs
}

func blobOf(raw []byte) *Blob {
	if raw == nil {
		return nil
	}
	return &Blob{SHA256: computerevent.DigestBytes(raw), Bytes: len(raw)}
}

func (f *fixture) input(record Record, blobs Blobs) HostInput {
	return HostInput{Record: record, TreePatch: blobs.TreePatch, ReviewPatch: blobs.ReviewPatch, ParentPatch: blobs.ParentPatch, Base: GitStore{Dir: f.repo}}
}

// fullDiff is a binary diff between two trees of the fixture repository.
func (f *fixture) fullDiff(a, b string) []byte {
	f.t.Helper()
	return []byte(gitT(f.t, f.repo, nil, "", "diff-tree", "-p", "-r", "--binary", "--full-index", "--no-renames", "--no-color", a, b) + "\n")
}

func wantReason(t *testing.T, err error, reason string) {
	t.Helper()
	if ReasonOf(err) != reason {
		t.Fatalf("error = %v, want reason %s", err, reason)
	}
}

func changed(content string) map[string]entry {
	return map[string]entry{testRoot + "/App.svelte": {"100644", content}}
}

func TestHostAcceptsAPackageConfinedRelease(t *testing.T) {
	f := newFixture(t)
	record, blobs := f.release(f.tree(changed("<p>reader v2</p>\n")))
	if err := record.Validate(); err != nil {
		t.Fatal(err)
	}
	if err := VerifyHost(context.Background(), f.input(record, blobs)); err != nil {
		t.Fatalf("a confined release is refused: %v", err)
	}
}

func TestHostRefusesATreePatchOutsideTheRoot(t *testing.T) {
	f := newFixture(t)
	full := f.tree(map[string]entry{
		testRoot + "/App.svelte": {"100644", "<p>reader v2</p>\n"},
		"frontend/src/app.css":   {"100644", "body { color: red }\n"},
	})
	record, blobs := f.release(full)
	// L2: the guest forges the tree_patch as the whole tree, app.css and all;
	// the subtree id still matches.
	forged := f.fullDiff(EmptyTree, full)
	blobs.TreePatch = forged
	record.Source.TreePatch = *blobOf(forged)
	err := VerifyHost(context.Background(), f.input(record, blobs))
	wantReason(t, err, ReasonContentNotConfined)
	if !strings.Contains(err.Error(), "frontend/src/app.css") {
		t.Fatalf("L2 refusal does not name the outside path: %v", err)
	}
}

func TestHostRefusesSymlinksSubmodulesAndGitattributes(t *testing.T) {
	f := newFixture(t)
	for name, e := range map[string]map[string]entry{
		"symlink":        {testRoot + "/link": {"120000", "../../app.css"}},
		"submodule":      {testRoot + "/sub": {"160000", f.base}},
		"gitattributes":  {testRoot + "/.gitattributes": {"100644", "*.svelte filter=x\n"}},
		"nested symlink": {testRoot + "/lib/link": {"120000", "/etc/passwd"}},
	} {
		t.Run(name, func(t *testing.T) {
			f.t = t
			record, blobs := f.release(f.tree(e))
			wantReason(t, VerifyHost(context.Background(), f.input(record, blobs)), ReasonContentNotConfined)
		})
	}
}

func TestHostRefusesASourceTreeThatIsNotTheStoredSubtree(t *testing.T) {
	f := newFixture(t)
	record, blobs := f.release(f.tree(changed("<p>v2</p>\n")))
	other, _ := f.release(f.tree(changed("<p>v3</p>\n")))
	record.Source.Tree = other.Source.Tree
	wantReason(t, VerifyHost(context.Background(), f.input(record, blobs)), ReasonTreeMismatch)
}

func TestHostRefusesAReviewPatchThatDoesNotMakeTheSubtree(t *testing.T) {
	f := newFixture(t)
	record, blobs := f.release(f.tree(changed("<p>v2</p>\n")))
	_, other := f.release(f.tree(changed("<p>v3</p>\n")))
	blobs.ReviewPatch = other.ReviewPatch
	record.Source.ReviewPatch = *blobOf(other.ReviewPatch)
	wantReason(t, VerifyHost(context.Background(), f.input(record, blobs)), ReasonReviewMismatch)
}

func TestHostRefusesAParentPatchWhoseTreeIsNotTheBaseTree(t *testing.T) {
	f := newFixture(t)
	full := f.tree(changed("<p>v2</p>\n"))
	record, blobs := f.release(full)

	// The publisher claims a parent subtree the base does not hold: a reader
	// it changed earlier and never published.
	earlier := f.subtree(f.tree(changed("<p>unpublished</p>\n")))
	forged, err := BuildBlobs(context.Background(), f.repo, testRoot, earlier, record.Source.Tree)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("parent_patch disagrees with parent.tree", func(t *testing.T) {
		r, b := record, blobs
		b.ParentPatch = forged.ParentPatch
		r.ParentPatch = blobOf(forged.ParentPatch)
		wantReason(t, VerifyHost(context.Background(), f.input(r, b)), ReasonParentMismatch)
	})
	t.Run("parent_patch and parent.tree agree, the base does not", func(t *testing.T) {
		r, b := record, forged
		r.Parent.Tree = earlier
		r.ParentPatch = blobOf(forged.ParentPatch)
		r.Source.ReviewPatch = *blobOf(forged.ReviewPatch)
		r.Source.TreePatch = *blobOf(forged.TreePatch)
		wantReason(t, VerifyHost(context.Background(), f.input(r, b)), ReasonParentMismatch)
	})
}

func TestHostRefusesABaseCommitItDoesNotHold(t *testing.T) {
	f := newFixture(t)
	record, blobs := f.release(f.tree(changed("<p>v2</p>\n")))
	record.Parent.Commit = strings.Repeat("7", 40)
	wantReason(t, VerifyHost(context.Background(), f.input(record, blobs)), ReasonParentUnverifiable)

	record, blobs = f.release(f.tree(changed("<p>v2</p>\n")))
	in := f.input(record, blobs)
	in.Base = nil
	wantReason(t, VerifyHost(context.Background(), in), ReasonParentUnverifiable)

	in.Base = GitStore{Dir: filepath.Join(t.TempDir(), "missing")}
	wantReason(t, VerifyHost(context.Background(), in), ReasonParentUnverifiable)
}

func TestHostAcceptsANewPackageOnlyWhenTheBaseLacksIt(t *testing.T) {
	f := newFixture(t)
	const root = "frontend/src/packages/notes"
	manifest := strings.Replace(testManifest, `"name": "reader"`, `"name": "notes"`, 1)
	full := f.tree(map[string]entry{root + "/choir-package.json": {"100644", manifest}, root + "/App.svelte": {"100644", "<p>notes</p>\n"}})
	tree, _, err := Subtree(context.Background(), f.repo, full, root)
	if err != nil {
		t.Fatal(err)
	}
	blobs, err := BuildBlobs(context.Background(), f.repo, root, EmptyTree, tree)
	if err != nil || blobs.ParentPatch != nil {
		t.Fatalf("new package blobs: parent_patch=%d bytes, %v", len(blobs.ParentPatch), err)
	}
	summary, version, err := CheckManifest("notes", []byte(manifest))
	if err != nil {
		t.Fatal(err)
	}
	record := Record{
		Schema: Schema, Package: Package{Name: "notes", ManifestVersion: version}, Track: "owner-a",
		Parent: Parent{Kind: ParentBase, Commit: f.base, Tree: EmptyTree},
		Source: Source{Root: root, Tree: tree, TreePatch: *blobOf(blobs.TreePatch), ReviewPatch: *blobOf(blobs.ReviewPatch),
			ChangedPaths: []string{root + "/App.svelte", root + "/choir-package.json"}, BuiltOn: f.base},
		Manifest: summary, Publisher: Publisher{OwnerID: "owner-a", ComputerID: "computer-a1"},
		Operation: Operation{OperationID: "op-1", BundleDigest: strings.Repeat("b", 64), DecisionEvent: strings.Repeat("e", 64)},
	}
	if err := VerifyHost(context.Background(), HostInput{Record: record, TreePatch: blobs.TreePatch, ReviewPatch: blobs.ReviewPatch, Base: GitStore{Dir: f.repo}}); err != nil {
		t.Fatalf("a new package is refused: %v", err)
	}
	// The same claim of an empty parent for a package the base holds is a
	// lie about the base.
	r, b := f.release(f.tree(changed("<p>v2</p>\n")))
	r.Parent.Tree, r.ParentPatch = EmptyTree, nil
	reviewed, err := BuildBlobs(context.Background(), f.repo, testRoot, EmptyTree, r.Source.Tree)
	if err != nil {
		t.Fatal(err)
	}
	r.Source.ReviewPatch = *blobOf(reviewed.ReviewPatch)
	wantReason(t, VerifyHost(context.Background(), HostInput{Record: r, TreePatch: b.TreePatch, ReviewPatch: reviewed.ReviewPatch, Base: GitStore{Dir: f.repo}}), ReasonParentMismatch)
}

func TestHostChecksAReleaseParentAgainstItsStoredTree(t *testing.T) {
	f := newFixture(t)
	first, firstBlobs := f.release(f.tree(changed("<p>v2</p>\n")))
	second := f.subtree(f.tree(changed("<p>v3</p>\n")))
	blobs, err := BuildBlobs(context.Background(), f.repo, testRoot, first.Source.Tree, second)
	if err != nil {
		t.Fatal(err)
	}
	record := first
	record.Parent = Parent{Kind: ParentRelease, ReleaseID: strings.Repeat("a", 64), Tree: first.Source.Tree}
	record.ParentPatch = nil
	record.Source.Tree = second
	record.Source.TreePatch, record.Source.ReviewPatch = *blobOf(blobs.TreePatch), *blobOf(blobs.ReviewPatch)
	in := HostInput{Record: record, TreePatch: blobs.TreePatch, ReviewPatch: blobs.ReviewPatch, ParentRelease: &first, ParentTreePatch: firstBlobs.TreePatch, Base: GitStore{Dir: f.repo}}
	if err := VerifyHost(context.Background(), in); err != nil {
		t.Fatalf("a release on its track head is refused: %v", err)
	}
	// P5: the parent release's stored tree is not the tree the record names.
	in.Record.Parent.Tree = f.baseSubtree()
	wantReason(t, VerifyHost(context.Background(), in), ReasonParentMismatch)
	// A release parent with no parent release at all.
	in = HostInput{Record: record, TreePatch: blobs.TreePatch, ReviewPatch: blobs.ReviewPatch, Base: GitStore{Dir: f.repo}}
	wantReason(t, VerifyHost(context.Background(), in), ReasonParentMismatch)
}

func TestHostRefusesKindsBeforeS12(t *testing.T) {
	f := newFixture(t)
	withKinds := strings.Replace(testManifest, `"kinds": []`, `"kinds": [{"kind": "choir.app.reader.note"}]`, 1)
	full := f.tree(map[string]entry{testRoot + "/choir-package.json": {"100644", withKinds}})
	if _, _, err := CheckManifest("reader", []byte(withKinds)); ReasonOf(err) != ReasonKindsNeedPlatform {
		t.Fatalf("L25 publisher side = %v", err)
	}
	record, blobs := f.release(f.tree(changed("<p>v2</p>\n")))
	// The publisher's summary says no kinds; the stored bytes declare one.
	tree := f.subtree(full)
	forged, err := BuildBlobs(context.Background(), f.repo, testRoot, record.Parent.Tree, tree)
	if err != nil {
		t.Fatal(err)
	}
	record.Source.Tree = tree
	record.Source.TreePatch, record.Source.ReviewPatch = *blobOf(forged.TreePatch), *blobOf(forged.ReviewPatch)
	record.Manifest.SHA256 = computerevent.DigestBytes([]byte(withKinds))
	blobs.TreePatch, blobs.ReviewPatch = forged.TreePatch, forged.ReviewPatch
	wantReason(t, VerifyHost(context.Background(), f.input(record, blobs)), ReasonKindsNeedPlatform)
}

func TestHostRefusesAManifestSummaryThatIsNotTheBytes(t *testing.T) {
	f := newFixture(t)
	record, blobs := f.release(f.tree(changed("<p>v2</p>\n")))
	record.Manifest.OpensCount = 7
	wantReason(t, VerifyHost(context.Background(), f.input(record, blobs)), ReasonManifestInvalid)
}

func TestHostRefusesAnAddedSecret(t *testing.T) {
	f := newFixture(t)
	record, blobs := f.release(f.tree(changed("GITHUB_TOKEN=ghp_" + strings.Repeat("A", 36) + "\n")))
	wantReason(t, VerifyHost(context.Background(), f.input(record, blobs)), ReasonSecretFound)
}

func TestRecordRefusesAnUnchangedPackage(t *testing.T) {
	f := newFixture(t)
	record, _ := f.release(f.tree(changed("<p>v2</p>\n")))
	record.Source.Tree = record.Parent.Tree
	wantReason(t, record.Validate(), ReasonUnchanged)
}

func TestBlobCapNamesTheSize(t *testing.T) {
	big := make([]byte, MaxBlobBytes+1)
	err := VerifyBlob(Blob{SHA256: computerevent.DigestBytes(big), Bytes: len(big)}, big)
	wantReason(t, err, ReasonBlobTooLarge)
	if !strings.Contains(err.Error(), "2097153") {
		t.Fatalf("L27: the refusal does not name the size: %v", err)
	}
	f := newFixture(t)
	record, _ := f.release(f.tree(changed("<p>v2</p>\n")))
	record.Source.TreePatch.Bytes = MaxBlobBytes + 1
	wantReason(t, record.Validate(), ReasonBlobTooLarge)
	if err := VerifyBlob(Blob{SHA256: strings.Repeat("0", 64), Bytes: 3}, []byte("abc")); ReasonOf(err) != ReasonBlobMismatch {
		t.Fatalf("a blob that does not hash to its name = %v", err)
	}
}

func TestRecordShapeAndIdentity(t *testing.T) {
	f := newFixture(t)
	record, _ := f.release(f.tree(changed("<p>v2</p>\n")))
	id, err := record.ReleaseID()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := record.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := Decode(raw)
	if err != nil || VerifyID(decoded, id) != nil {
		t.Fatalf("a canonical record does not round trip: %v", err)
	}
	if VerifyID(decoded, strings.Repeat("0", 64)) == nil {
		t.Fatal("a record verified against another id")
	}
	if _, err := Decode([]byte(strings.Replace(string(raw), `"schema"`, `"build":{},"schema"`, 1))); err == nil {
		t.Fatal("an unknown field (build is S12's) was accepted")
	}
	for name, mutate := range map[string]func(*Record){
		"track is not the publisher": func(r *Record) { r.Track = "owner-b" },
		"root is not the package's":  func(r *Record) { r.Source.Root = "frontend/src/packages/image" },
		"parent_patch on a release": func(r *Record) {
			r.Parent = Parent{Kind: ParentRelease, ReleaseID: strings.Repeat("a", 64), Tree: r.Parent.Tree}
		},
		"release parent names a commit": func(r *Record) {
			r.Parent = Parent{Kind: ParentRelease, ReleaseID: strings.Repeat("a", 64), Commit: f.base, Tree: r.Parent.Tree}
			r.ParentPatch = nil
		},
		"unknown parent kind":                   func(r *Record) { r.Parent.Kind = "branch" },
		"changed path outside the root":         func(r *Record) { r.Source.ChangedPaths = []string{"frontend/src/app.css"} },
		"no changed paths":                      func(r *Record) { r.Source.ChangedPaths = nil },
		"a reserved package name":               func(r *Record) { r.Package.Name = "desktop"; r.Source.Root = Root("desktop") },
		"a two-line test command":               func(r *Record) { r.Declarations.TestCommand = "a\nb" },
		"no operation":                          func(r *Record) { r.Operation.OperationID = "" },
		"a manifest version that is not semver": func(r *Record) { r.Package.ManifestVersion = "v1" },
	} {
		r := record
		mutate(&r)
		if err := r.Validate(); err == nil {
			t.Errorf("%s: validated", name)
		}
	}
}

func TestGitStoreReadsTheBaseAndReportsShallow(t *testing.T) {
	f := newFixture(t)
	store := GitStore{Dir: f.repo}
	tree, present, err := store.PackageTree(context.Background(), f.base, testRoot)
	if err != nil || !present || tree != f.baseSubtree() {
		t.Fatalf("base package tree = %s %v %v", tree, present, err)
	}
	if _, present, err := store.PackageTree(context.Background(), f.base, "frontend/src/packages/notes"); err != nil || present {
		t.Fatalf("an absent package = present %v %v", present, err)
	}
	if _, _, err := store.PackageTree(context.Background(), strings.Repeat("7", 40), testRoot); err == nil {
		t.Fatal("an unknown commit was read")
	}
	if shallow, err := store.Shallow(context.Background()); err != nil || shallow {
		t.Fatalf("full clone shallow = %v %v", shallow, err)
	}
	clone := t.TempDir()
	gitT(t, clone, nil, "", "clone", "-q", "--depth", "1", "file://"+f.repo, ".")
	if shallow, err := (GitStore{Dir: clone}).Shallow(context.Background()); err != nil || !shallow {
		t.Fatalf("depth-1 clone shallow = %v %v", shallow, err)
	}
}

func TestOutsideRootAndDiffPaths(t *testing.T) {
	got := OutsideRoot([]string{testRoot + "/App.svelte", testRoot, "frontend/src/packages/readerx/a", "frontend/src/app.css"}, testRoot)
	if strings.Join(got, ",") != "frontend/src/packages/readerx/a,frontend/src/app.css" {
		t.Fatalf("outside = %v", got)
	}
	f := newFixture(t)
	full := f.tree(map[string]entry{testRoot + "/App.svelte": {"100644", "x\n"}, "frontend/src/app.css": {"-", ""}})
	paths, err := DiffPaths(context.Background(), f.repo, f.base, full)
	if err != nil || strings.Join(paths, ",") != "frontend/src/app.css,"+testRoot+"/App.svelte" {
		t.Fatalf("diff paths = %v %v", paths, err)
	}
}
