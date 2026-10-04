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
	"github.com/yusefmosiah/go-choir/internal/storeschema"
)

// compatTestBase writes a guest-image-manifest that carries build_commit so
// the base-commit join has something to check, alongside the digest join.
func compatTestBase(t *testing.T, commit string) (manifestPath, digest string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "guest-image-manifest")
	content := "contract=choir-guest-image-v1\nbuild_commit=" + commit + "\nautoputer=/nix/store/aaaaaaaabbbbbbbbccccccccdddddddd-autoputer\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	d, err := DigestFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return path, d
}

func compatEngine(t *testing.T, root, baseManifestPath, schemaPath string) (*Updater, *fakeHealthProber) {
	t.Helper()
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	prober := &fakeHealthProber{}
	engine, err := NewWithBase(root, "computer-test", "realization-test", &fakeServiceManager{}, prober,
		testReceiptSigner{key: computerevent.SigningKey{SignerRef: computerevent.SignerRef{SignerDomain: "guest-core", KeyID: "updater-test"}, PrivateKey: privateKey}},
		baseManifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if schemaPath != "" {
		engine.WithStoreSchemaPath(schemaPath)
	}
	return engine, prober
}

func writeSchemaReceipt(t *testing.T, dir string, version uint64) string {
	t.Helper()
	path := filepath.Join(dir, storeschema.File)
	raw := `{"schema":"` + storeschema.Name + `","version":` + itoa(version) + `}`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func itoa(v uint64) string {
	if v == 0 {
		return "0"
	}
	var digits [20]byte
	i := len(digits)
	for v > 0 {
		i--
		digits[i] = byte('0' + v%10)
		v /= 10
	}
	return string(digits[i:])
}

// S2-d: a release whose declared store schema is older than the guest's
// persisted epoch is the vm-3dc68688 stale-binary class — refuse before any
// mutation: no staged release dir, no private-store replay, no swap.
func TestApplyRefusesReleaseOlderThanGuestStore(t *testing.T) {
	blob, err := os.ReadFile(filepath.Join("testdata", "closure-single.nar"))
	if err != nil {
		t.Skipf("closure fixture absent: %v", err)
	}
	root := filepath.Join(t.TempDir(), "updater")
	t.Cleanup(func() { makeTreeWritable(root) })
	basePath, baseDigest := compatTestBase(t, "commit-abc123")
	schemaDir := t.TempDir()
	// Guest store is at epoch 5; the release was built against epoch 3.
	schemaPath := writeSchemaReceipt(t, schemaDir, 5)
	engine, _ := compatEngine(t, root, basePath, schemaPath)

	request := layeredRequestFixture(t, root, "computer-test", "realization-test", "op-stale", baseDigest, blob, "0pvb33w34jr4243s1182511gxwrchf0c-payload.txt")
	request.Manifest.StoreSchemaVersion = 3
	request.Manifest.MinStoreSchemaVersion = 1
	request.Manifest.BaseCommit = "commit-abc123"
	if err := refinalizeRequest(&request); err != nil {
		t.Fatal(err)
	}
	result, err := engine.Apply(context.Background(), request)
	if err == nil || !strings.Contains(err.Error(), "newer than release") {
		t.Fatalf("stale-schema apply = %+v err=%v, want pre-mutation refusal", result, err)
	}
	if _, statErr := os.Lstat(filepath.Join(root, "releases", request.Manifest.ContentDigest)); !os.IsNotExist(statErr) {
		t.Fatalf("release dir staged despite refusal")
	}
	if _, statErr := os.Lstat(filepath.Join(root, "current")); !os.IsNotExist(statErr) {
		t.Fatalf("current pointer created despite refusal")
	}
}

// The inverse bound: a guest store older than the release's migratable floor
// refuses — the release cannot safely downgrade or run migrations backward.
func TestApplyRefusesStoreOlderThanReleaseMinimum(t *testing.T) {
	blob, err := os.ReadFile(filepath.Join("testdata", "closure-single.nar"))
	if err != nil {
		t.Skipf("closure fixture absent: %v", err)
	}
	root := filepath.Join(t.TempDir(), "updater")
	t.Cleanup(func() { makeTreeWritable(root) })
	basePath, baseDigest := compatTestBase(t, "commit-abc123")
	schemaDir := t.TempDir()
	schemaPath := writeSchemaReceipt(t, schemaDir, 1)
	engine, _ := compatEngine(t, root, basePath, schemaPath)

	request := layeredRequestFixture(t, root, "computer-test", "realization-test", "op-ancient", baseDigest, blob, "0pvb33w34jr4243s1182511gxwrchf0c-payload.txt")
	request.Manifest.StoreSchemaVersion = 5
	request.Manifest.MinStoreSchemaVersion = 4
	request.Manifest.BaseCommit = "commit-abc123"
	if err := refinalizeRequest(&request); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Apply(context.Background(), request); err == nil || !strings.Contains(err.Error(), "older than release's minimum") {
		t.Fatalf("ancient-store apply err=%v, want pre-mutation refusal", err)
	}
}

// A release declaring a base_commit that differs from the booted image's
// build_commit is refused pre-mutation — commit-level provenance beside the
// digest join.
func TestApplyRefusesMismatchedBaseCommit(t *testing.T) {
	blob, err := os.ReadFile(filepath.Join("testdata", "closure-single.nar"))
	if err != nil {
		t.Skipf("closure fixture absent: %v", err)
	}
	root := filepath.Join(t.TempDir(), "updater")
	t.Cleanup(func() { makeTreeWritable(root) })
	basePath, baseDigest := compatTestBase(t, "commit-abc123")
	schemaDir := t.TempDir()
	schemaPath := writeSchemaReceipt(t, schemaDir, 2)
	engine, _ := compatEngine(t, root, basePath, schemaPath)

	request := layeredRequestFixture(t, root, "computer-test", "realization-test", "op-wrongbase", baseDigest, blob, "0pvb33w34jr4243s1182511gxwrchf0c-payload.txt")
	request.Manifest.StoreSchemaVersion = 2
	request.Manifest.BaseCommit = "commit-WRONG"
	if err := refinalizeRequest(&request); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Apply(context.Background(), request); err == nil || !strings.Contains(err.Error(), "does not match booted base commit") {
		t.Fatalf("wrong-base-commit apply err=%v, want pre-mutation refusal", err)
	}
}

// Declared compat fields that match the guest pass the gate and the apply
// lands — the gate must not wedge the normal path.
func TestApplyAcceptsCompatibleDeclaredState(t *testing.T) {
	blob, err := os.ReadFile(filepath.Join("testdata", "closure-single.nar"))
	if err != nil {
		t.Skipf("closure fixture absent: %v", err)
	}
	root := filepath.Join(t.TempDir(), "updater")
	t.Cleanup(func() { makeTreeWritable(root) })
	basePath, baseDigest := compatTestBase(t, "commit-abc123")
	schemaDir := t.TempDir()
	schemaPath := writeSchemaReceipt(t, schemaDir, 2)
	engine, _ := compatEngine(t, root, basePath, schemaPath)

	request := layeredRequestFixture(t, root, "computer-test", "realization-test", "op-compat", baseDigest, blob, "0pvb33w34jr4243s1182511gxwrchf0c-payload.txt")
	request.Manifest.StoreSchemaVersion = 3
	request.Manifest.MinStoreSchemaVersion = 1
	request.Manifest.BaseCommit = "commit-abc123"
	if err := refinalizeRequest(&request); err != nil {
		t.Fatal(err)
	}
	result, err := engine.Apply(context.Background(), request)
	if err != nil || result.Outcome != "applied" {
		t.Fatalf("compatible apply = %+v err=%v", result, err)
	}
}

// A release that declares a schema window on a guest with no receipt wired
// fails closed — compatibility cannot be proven, so no mutation.
func TestApplyRefusesDeclaredWindowWithoutReceipt(t *testing.T) {
	blob, err := os.ReadFile(filepath.Join("testdata", "closure-single.nar"))
	if err != nil {
		t.Skipf("closure fixture absent: %v", err)
	}
	root := filepath.Join(t.TempDir(), "updater")
	t.Cleanup(func() { makeTreeWritable(root) })
	basePath, baseDigest := compatTestBase(t, "commit-abc123")
	engine, _ := compatEngine(t, root, basePath, "")

	request := layeredRequestFixture(t, root, "computer-test", "realization-test", "op-unwired", baseDigest, blob, "0pvb33w34jr4243s1182511gxwrchf0c-payload.txt")
	request.Manifest.StoreSchemaVersion = 2
	if err := refinalizeRequest(&request); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Apply(context.Background(), request); err == nil || !strings.Contains(err.Error(), "no store schema receipt") {
		t.Fatalf("unwired apply err=%v, want fail-closed refusal", err)
	}
}
