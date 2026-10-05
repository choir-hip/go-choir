package updater

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

// closure-single.nar is a real `nix-store --export` of a one-file store path
// (payload.txt = "hello-layer\n"). It proves the export reader + NAR decoder
// handle the live wire format, not a hand-rolled stand-in.
func TestMaterializeClosureSinglePath(t *testing.T) {
	blob, err := os.ReadFile(filepath.Join("testdata", "closure-single.nar"))
	if err != nil {
		t.Skipf("closure fixture absent: %v", err)
	}
	store := filepath.Join(t.TempDir(), "store")
	t.Cleanup(func() { makeTreeWritable(store) })
	materialized, err := MaterializeClosure(blob, store)
	if err != nil {
		t.Fatalf("MaterializeClosure: %v", err)
	}
	if len(materialized) != 1 {
		t.Fatalf("materialized %d paths, want 1: %v", len(materialized), materialized)
	}
	base := materialized[0]
	if !strings.HasPrefix(base, "0pvb33w34jr4243s1182511gxwrchf0c-") || !strings.HasSuffix(base, "-payload.txt") {
		t.Fatalf("unexpected store basename %q", base)
	}
	got, err := os.ReadFile(filepath.Join(store, base))
	if err != nil {
		t.Fatalf("read materialized path: %v", err)
	}
	if string(got) != "hello-layer\n" {
		t.Fatalf("materialized payload = %q, want hello-layer", got)
	}
	// Re-materialize must be idempotent (content-addressed skip).
	if _, err := MaterializeClosure(blob, store); err != nil {
		t.Fatalf("re-materialize not idempotent: %v", err)
	}
}

// closure-dir.nar exports a store path that is a directory containing an
// executable, a data file, and a symlink — covering every NAR node kind.
func TestMaterializeClosureDirectory(t *testing.T) {
	blob, err := os.ReadFile(filepath.Join("testdata", "closure-dir.nar"))
	if err != nil {
		t.Skipf("closure fixture absent: %v", err)
	}
	store := filepath.Join(t.TempDir(), "store")
	t.Cleanup(func() { makeTreeWritable(store) })
	materialized, err := MaterializeClosure(blob, store)
	if err != nil {
		t.Fatalf("MaterializeClosure: %v", err)
	}
	if len(materialized) != 1 {
		t.Fatalf("materialized %d paths, want 1", len(materialized))
	}
	root := filepath.Join(store, materialized[0])

	execInfo, err := os.Stat(filepath.Join(root, "bin", "app"))
	if err != nil {
		t.Fatalf("exec missing: %v", err)
	}
	if execInfo.Mode().Perm()&0o111 == 0 {
		t.Fatalf("bin/app not executable (mode %v)", execInfo.Mode())
	}
	if got, _ := os.ReadFile(filepath.Join(root, "bin", "app")); !strings.Contains(string(got), "echo layered") {
		t.Fatalf("bin/app contents = %q", got)
	}
	if got, _ := os.ReadFile(filepath.Join(root, "share", "data.txt")); string(got) != "data-1\n" {
		t.Fatalf("share/data.txt = %q", got)
	}
	link, err := os.Readlink(filepath.Join(root, "share", "link.txt"))
	if err != nil {
		t.Fatalf("symlink missing: %v", err)
	}
	if !strings.HasPrefix(link, "/nix/store/") {
		t.Fatalf("symlink target = %q, want a /nix/store path", link)
	}
}

// A corrupt or non-export blob must fail closed, not produce a partial store.
func TestParseClosureRejectsNonExport(t *testing.T) {
	for name, blob := range map[string][]byte{
		"empty":      {},
		"bad-marker": append([]byte{0x02, 0, 0, 0, 0, 0, 0, 0}, []byte("nix-archive-1")...),
		"bad-magic":  append([]byte{0x01, 0, 0, 0, 0, 0, 0, 0, 0x0d, 0, 0, 0, 0, 0, 0, 0}, []byte("not-an-archive")...),
		"truncated":  []byte{0x01, 0, 0, 0, 0, 0, 0, 0},
		"non-store":  {0x01, 0, 0, 0, 0, 0, 0, 0, 0x0d, 0, 0, 0, 0, 0, 0, 0},
	} {
		if _, _, err := ParseClosure(blob); err == nil {
			t.Fatalf("ParseClosure(%s) succeeded on malformed input", name)
		}
	}
}

// A layered apply must replay closure.nar into the private store and GC-root
// it, while the file release still lands under releases/<digest>.
func TestApplyMaterializesLayeredClosureAndGcRoots(t *testing.T) {
	blob, err := os.ReadFile(filepath.Join("testdata", "closure-single.nar"))
	if err != nil {
		t.Skipf("closure fixture absent: %v", err)
	}
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "updater")
	t.Cleanup(func() { makeTreeWritable(root) })

	baseDir := t.TempDir()
	baseManifestPath := filepath.Join(baseDir, "guest-image-manifest.json")
	if err := os.WriteFile(baseManifestPath, []byte(`{"schema":"choir-guest-image-v1"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	bootedDigest, err := DigestFile(baseManifestPath)
	if err != nil {
		t.Fatal(err)
	}

	engine, err := NewWithBase(root, "computer-test", "realization-test", &fakeServiceManager{}, fakeHealthProber{},
		testReceiptSigner{key: computerevent.SigningKey{SignerRef: computerevent.SignerRef{SignerDomain: "guest-core", KeyID: "updater-test"}, PrivateKey: privateKey}},
		baseManifestPath)
	if err != nil {
		t.Fatal(err)
	}

	request := updaterRequestFixture(t, root, "computer-test", "realization-test", "op-layered", "idem-layered", "layered payload")
	closurePath := filepath.Join(request.SourceDir, "closure.nar")
	if err := os.WriteFile(closurePath, blob, 0o444); err != nil {
		t.Fatal(err)
	}
	closureSum, err := fileSHA256(closurePath)
	if err != nil {
		t.Fatal(err)
	}
	request.Manifest.Files = append(request.Manifest.Files, ManifestFile{Path: "closure.nar", SHA256: closureSum, Mode: 0o444})
	request.Manifest.ClosureDigest = closureSum
	request.Manifest.BaseImageManifestDigest = bootedDigest
	request.Manifest.LayeringEntrypoint = "0pvb33w34jr4243s1182511gxwrchf0c-payload.txt"
	if err := refinalizeRequest(&request); err != nil {
		t.Fatal(err)
	}

	result, err := engine.Apply(context.Background(), request)
	if err != nil {
		t.Fatalf("layered apply refused: %v", err)
	}
	if result.Outcome != "applied" {
		t.Fatalf("layered apply outcome = %q", result.Outcome)
	}
	materialized := filepath.Join(root, "store", "0pvb33w34jr4243s1182511gxwrchf0c-payload.txt")
	if _, err := os.Stat(materialized); err != nil {
		t.Fatalf("closure path not materialized: %v", err)
	}
	gcLink := filepath.Join(root, "gc-roots", result.ReleaseDigest, "0pvb33w34jr4243s1182511gxwrchf0c-payload.txt")
	if target, err := os.Readlink(gcLink); err != nil || target != materialized {
		t.Fatalf("gc-root link = %q err=%v, want %q", target, err, materialized)
	}
	// The exec pointer travels inside the release dir so the current/ swap
	// moves it atomically (S2-e); the wrapper reads current/layering-entrypoint.
	current, err := os.Readlink(filepath.Join(root, "current"))
	if err != nil {
		t.Fatalf("read current: %v", err)
	}
	entryBytes, err := os.ReadFile(filepath.Join(current, "layering-entrypoint"))
	if err != nil {
		t.Fatalf("layering entrypoint not recorded in release dir: %v", err)
	}
	if strings.TrimSpace(string(entryBytes)) != materialized {
		t.Fatalf("layering-entrypoint = %q, want %q", entryBytes, materialized)
	}
	if _, err := os.Lstat(filepath.Join(root, "layering-entrypoint")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("legacy root layering-entrypoint must not exist: %v", err)
	}
}

// A release whose materialized autoputer output root carries frontend/
// must stage it into the release dir, so the serving surface join
// (executable+frontend in one transaction) holds for releases whose offer
// carries no separate frontend file.
func TestApplyStagesReleaseFrontendFromOutputRoot(t *testing.T) {
	blob, err := os.ReadFile(filepath.Join("testdata", "closure-single.nar"))
	if err != nil {
		t.Skipf("closure fixture absent: %v", err)
	}
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "updater")
	t.Cleanup(func() { makeTreeWritable(root) })
	baseManifestPath, bootedDigest := layeredTestBase(t)
	engine, err := NewWithBase(root, "computer-test", "realization-test", &fakeServiceManager{}, fakeHealthProber{},
		testReceiptSigner{key: computerevent.SigningKey{SignerRef: computerevent.SignerRef{SignerDomain: "guest-core", KeyID: "updater-test"}, PrivateKey: privateKey}},
		baseManifestPath)
	if err != nil {
		t.Fatal(err)
	}
	request := layeredRequestFixture(t, root, "computer-test", "realization-test", "op-frontend", bootedDigest, blob, "0pvb33w34jr4243s1182511gxwrchf0c-payload.txt")
	result, err := engine.Apply(context.Background(), request)
	if err != nil {
		t.Fatalf("layered apply refused: %v", err)
	}
	if result.Outcome != "applied" {
		t.Fatalf("layered apply outcome = %q", result.Outcome)
	}
	current, err := os.Readlink(filepath.Join(root, "current"))
	if err != nil {
		t.Fatalf("read current: %v", err)
	}
	// The nar fixture carries no frontend tree, so no frontend dir stages.
	if _, err := os.Lstat(filepath.Join(current, "frontend")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("frontend staged without a release frontend tree")
	}
	// Plant a built frontend tree beside the materialized entrypoint and
	// re-stage under a fresh operation: the frontend must land in the dir.
	entryDir := filepath.Dir(filepath.Join(root, "store", "0pvb33w34jr4243s1182511gxwrchf0c-payload.txt"))
	feDir := filepath.Join(filepath.Dir(entryDir), "frontend")
	if err := os.MkdirAll(feDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(feDir, "index.html"), []byte("<title>staged</title>"), 0o444); err != nil {
		t.Fatal(err)
	}
	retry := layeredRequestFixture(t, root, "computer-test", "realization-test", "op-frontend-2", bootedDigest, blob, "0pvb33w34jr4243s1182511gxwrchf0c-payload.txt")
	if _, err := engine.Apply(context.Background(), retry); err != nil {
		t.Fatalf("frontend retry refused: %v", err)
	}
	current2, err := os.Readlink(filepath.Join(root, "current"))
	if err != nil {
		t.Fatalf("read current: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(current2, "frontend", "index.html"))
	if err != nil || string(got) != "<title>staged</title>" {
		t.Fatalf("staged frontend = %q err=%v, want staged bytes", got, err)
	}
}

// layeredRequestFixture builds a layered apply request carrying the given
// narchive blob and private-store-relative exec path. blobName makes each
// release's closure.nar digest distinct so two layered releases stage as
// different release dirs.
func layeredRequestFixture(t *testing.T, root, computerID, realizationID, operationID, baseDigest string, blob []byte, entrypoint string) ApplyRequest {
	t.Helper()
	request := updaterRequestFixture(t, root, computerID, realizationID, operationID, "idem-"+operationID, "layered payload")
	closurePath := filepath.Join(request.SourceDir, "closure.nar")
	if err := os.WriteFile(closurePath, blob, 0o444); err != nil {
		t.Fatal(err)
	}
	closureSum, err := fileSHA256(closurePath)
	if err != nil {
		t.Fatal(err)
	}
	request.Manifest.Files = append(request.Manifest.Files, ManifestFile{Path: "closure.nar", SHA256: closureSum, Mode: 0o444})
	request.Manifest.ClosureDigest = closureSum
	request.Manifest.BaseImageManifestDigest = baseDigest
	request.Manifest.LayeringEntrypoint = entrypoint
	if err := refinalizeRequest(&request); err != nil {
		t.Fatal(err)
	}
	return request
}

func layeredTestBase(t *testing.T) (string, string) {
	t.Helper()
	baseDir := t.TempDir()
	baseManifestPath := filepath.Join(baseDir, "guest-image-manifest.json")
	if err := os.WriteFile(baseManifestPath, []byte(`{"schema":"choir-guest-image-v1"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	digest, err := DigestFile(baseManifestPath)
	if err != nil {
		t.Fatal(err)
	}
	return baseManifestPath, digest
}

func currentEntrypoint(t *testing.T, root string) string {
	t.Helper()
	current, err := os.Readlink(filepath.Join(root, "current"))
	if err != nil {
		return ""
	}
	raw, err := os.ReadFile(filepath.Join(current, "layering-entrypoint"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(raw))
}

// S2-e rollback atomicity: the exec pointer lives inside the release dir,
// so a failed layered apply's recovery swap restores the prior release's
// exec entrypoint with the same pointer — no global file can keep pointing
// at the failed release.
func TestApplyRestoresPriorLayeredEntrypoint(t *testing.T) {
	blob, err := os.ReadFile(filepath.Join("testdata", "closure-single.nar"))
	if err != nil {
		t.Skipf("closure fixture absent: %v", err)
	}
	dirBlob, err := os.ReadFile(filepath.Join("testdata", "closure-dir.nar"))
	if err != nil {
		t.Skipf("dir closure fixture absent: %v", err)
	}
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "updater")
	t.Cleanup(func() { makeTreeWritable(root) })
	baseManifestPath, bootedDigest := layeredTestBase(t)
	prober := &fakeHealthProber{}
	engine, err := NewWithBase(root, "computer-test", "realization-test", &fakeServiceManager{}, prober,
		testReceiptSigner{key: computerevent.SigningKey{SignerRef: computerevent.SignerRef{SignerDomain: "guest-core", KeyID: "updater-test"}, PrivateKey: privateKey}},
		baseManifestPath)
	if err != nil {
		t.Fatal(err)
	}

	good := layeredRequestFixture(t, root, "computer-test", "realization-test", "op-good", bootedDigest, blob, "0pvb33w34jr4243s1182511gxwrchf0c-payload.txt")
	if result, err := engine.Apply(context.Background(), good); err != nil || result.Outcome != "applied" {
		t.Fatalf("prior layered apply = %+v, %v", result, err)
	}
	priorEntry := currentEntrypoint(t, root)
	wantPrior := filepath.Join(root, "store", "0pvb33w34jr4243s1182511gxwrchf0c-payload.txt")
	if priorEntry != wantPrior {
		t.Fatalf("prior entrypoint = %q, want %q", priorEntry, wantPrior)
	}

	// A health-failing layered apply whose own exec pointer resolves must be
	// rolled back to the prior release's entrypoint, not left pointing at the
	// failed release (the pre-S2-e global-file defect).
	dirPaths, err := MaterializeClosure(dirBlob, filepath.Join(root, "probe-store"))
	if err != nil {
		t.Fatal(err)
	}
	if len(dirPaths) == 0 {
		t.Fatal("dir fixture materialized no store paths")
	}
	// The dir fixture's store basename resolves under the private store, so
	// the apply stages and probes before failing — exercising recovery.
	bad := layeredRequestFixture(t, root, "computer-test", "realization-test", "op-bad", bootedDigest, dirBlob, dirPaths[0])
	prober.failDigest = bad.Manifest.ContentDigest
	result, err := engine.Apply(context.Background(), bad)
	if err == nil || result.Outcome != "failed" || result.RecoveryReceipt == nil {
		t.Fatalf("failed layered apply = %+v err=%v", result, err)
	}
	if got := currentEntrypoint(t, root); got != wantPrior {
		t.Fatalf("entrypoint after recovery = %q, want restored prior %q", got, wantPrior)
	}
}

// Pre-S2-e staged layered releases carry no in-dir entrypoint; restaging one
// must backfill the pointer so exec follows the swap.
func TestRestagePinnedBackfillsLegacyLayeredEntrypoint(t *testing.T) {
	blob, err := os.ReadFile(filepath.Join("testdata", "closure-single.nar"))
	if err != nil {
		t.Skipf("closure fixture absent: %v", err)
	}
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "updater")
	t.Cleanup(func() { makeTreeWritable(root) })
	baseManifestPath, bootedDigest := layeredTestBase(t)
	engine, err := NewWithBase(root, "computer-test", "realization-test", &fakeServiceManager{}, fakeHealthProber{},
		testReceiptSigner{key: computerevent.SigningKey{SignerRef: computerevent.SignerRef{SignerDomain: "guest-core", KeyID: "updater-test"}, PrivateKey: privateKey}},
		baseManifestPath)
	if err != nil {
		t.Fatal(err)
	}
	request := layeredRequestFixture(t, root, "computer-test", "realization-test", "op-legacy", bootedDigest, blob, "0pvb33w34jr4243s1182511gxwrchf0c-payload.txt")
	if _, err := engine.Apply(context.Background(), request); err != nil {
		t.Fatalf("apply: %v", err)
	}
	releaseDir := filepath.Join(root, "releases", request.Manifest.ContentDigest)
	entryPath := filepath.Join(releaseDir, "layering-entrypoint")
	// Simulate a pre-S2-e staged dir: remove the pointer file.
	makeTreeWritable(releaseDir)
	if err := os.Remove(entryPath); err != nil {
		t.Fatal(err)
	}
	if err := RestagePinnedRelease(root, request.Manifest.ContentDigest); err != nil {
		t.Fatalf("restage: %v", err)
	}
	raw, err := os.ReadFile(entryPath)
	if err != nil {
		t.Fatalf("backfilled entrypoint missing: %v", err)
	}
	want := filepath.Join(root, "store", "0pvb33w34jr4243s1182511gxwrchf0c-payload.txt")
	if strings.TrimSpace(string(raw)) != want {
		t.Fatalf("backfilled entrypoint = %q, want %q", raw, want)
	}
}
