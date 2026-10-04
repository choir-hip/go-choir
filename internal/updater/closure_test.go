package updater

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
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
	// The runtime wrapper reads the recorded entrypoint path.
	entryBytes, err := os.ReadFile(filepath.Join(root, "layering-entrypoint"))
	if err != nil {
		t.Fatalf("layering entrypoint not recorded: %v", err)
	}
	if strings.TrimSpace(string(entryBytes)) != materialized {
		t.Fatalf("layering-entrypoint = %q, want %q", entryBytes, materialized)
	}
}
