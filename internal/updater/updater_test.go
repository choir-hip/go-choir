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
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

type fakeServiceManager struct {
	restarts int
	cleanups int
}

func (s *fakeServiceManager) Restart(context.Context) error { s.restarts++; return nil }
func (s *fakeServiceManager) CleanupRestartHandoff() error  { s.cleanups++; return nil }

type fakeHealthProber struct{ failDigest string }

func (p fakeHealthProber) Probe(_ context.Context, digest string, _ ReleaseManifest) ([]string, error) {
	if digest == p.failDigest {
		return nil, errors.New("probe failed")
	}
	return []string{strings.Repeat("f", 64)}, nil
}

func TestUpdaterAppliesIdempotentlyAndRestoresPriorHealthyRelease(t *testing.T) {
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "updater")
	t.Cleanup(func() { makeTreeWritable(root) })
	service := &fakeServiceManager{}
	prober := &fakeHealthProber{}
	updater, err := New(root, "computer-test", "realization-test", service, prober, computerevent.SigningKey{
		SignerRef:  computerevent.SignerRef{SignerDomain: "guest-core", KeyID: "updater-test"},
		PrivateKey: privateKey,
	})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 18, 23, 45, 0, 0, time.UTC)
	updater.now = func() time.Time { return now }
	first := updaterRequestFixture(t, root, "computer-test", "realization-test", "operation-1", "idem-1", "first release")
	result, err := updater.Apply(context.Background(), first)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != "applied" || result.ReleaseDigest != first.Manifest.ContentDigest || result.MaterializationReceipt.ReceiptKind != "MaterializationReceipt" {
		t.Fatalf("first result = %+v", result)
	}
	if service.restarts != 1 || service.cleanups != 1 {
		t.Fatalf("first restart/cleanup count = %d/%d", service.restarts, service.cleanups)
	}
	retry, err := updater.Apply(context.Background(), first)
	if err != nil || retry.MaterializationReceipt.ReceiptID != result.MaterializationReceipt.ReceiptID || service.restarts != 1 {
		t.Fatalf("idempotent retry = %+v err=%v restarts=%d", retry, err, service.restarts)
	}

	second := updaterRequestFixture(t, root, "computer-test", "realization-test", "operation-2", "idem-2", "broken release")
	prober.failDigest = second.Manifest.ContentDigest
	failed, err := updater.Apply(context.Background(), second)
	if err == nil || failed.Outcome != "failed" || failed.RecoveryReceipt == nil {
		t.Fatalf("failed apply = %+v err=%v", failed, err)
	}
	currentDigest, _, err := updater.currentRelease()
	if err != nil || currentDigest != first.Manifest.ContentDigest {
		t.Fatalf("current release after recovery = %q, %v; want %q", currentDigest, err, first.Manifest.ContentDigest)
	}
	if service.restarts != 3 || service.cleanups != 2 {
		t.Fatalf("failure and restore restart/cleanup count = %d/%d, want 3/2", service.restarts, service.cleanups)
	}
}

func TestUpdaterRejectsSymlinkManifestEntryBeforePointerMutation(t *testing.T) {
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "updater")
	t.Cleanup(func() { makeTreeWritable(root) })
	updater, err := New(root, "computer-test", "realization-test", &fakeServiceManager{}, fakeHealthProber{}, computerevent.SigningKey{SignerRef: computerevent.SignerRef{SignerDomain: "guest-core", KeyID: "updater-test"}, PrivateKey: privateKey})
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(root, "incoming", "symlink-test")
	if err := os.MkdirAll(source, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/etc/passwd", filepath.Join(source, "choir")); err != nil {
		t.Fatal(err)
	}
	manifest := ReleaseManifest{Version: 1, ComputerID: "computer-test", AcceptedEventHead: strings.Repeat("a", 64), CodeRef: "code", ArtifactProgramRef: "program", EventSchemaVersion: 1, ReducerVersion: 1, Marker: "bad", Files: []ManifestFile{{Path: "choir", SHA256: strings.Repeat("b", 64), Mode: 0o555}}}
	unsigned, err := computerevent.CanonicalJSON(manifest)
	if err != nil {
		t.Fatal(err)
	}
	manifest.ContentDigest = computerevent.DigestBytes(unsigned)
	request := ApplyRequest{ComputerID: "computer-test", RealizationID: "realization-test", OperationID: "operation", IdempotencyKey: "idem", AcceptedEventHead: manifest.AcceptedEventHead, SourceDir: source, Manifest: manifest}
	request.RequestCommitment, err = validateApplyRequest(request.ComputerID, request.RealizationID, request)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := updater.Apply(context.Background(), request); err == nil {
		t.Fatal("symlink release was applied")
	}
	if _, err := os.Lstat(filepath.Join(root, "current")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("current pointer exists after refusal: %v", err)
	}
}

func TestRestartRequestManagerPublishesOnlyFixedTrigger(t *testing.T) {
	path := filepath.Join(t.TempDir(), "restart")
	manager := RestartRequestManager{Path: path}
	if err := manager.Restart(context.Background()); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil || string(raw) != "restart\n" || info.Mode().Perm() != 0o600 {
		t.Fatalf("restart request = %q, %#v, %v", raw, info, err)
	}
	if err := (RestartRequestManager{Path: filepath.Join(filepath.Dir(path), "arbitrary-unit")}).Restart(context.Background()); err == nil {
		t.Fatal("arbitrary restart target was accepted")
	}
}

func updaterRequestFixture(t *testing.T, updaterRoot, computerID, realizationID, operationID, idempotencyKey, content string) ApplyRequest {
	t.Helper()
	source := filepath.Join(updaterRoot, "incoming", operationID)
	if err := os.MkdirAll(source, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(source, "bin", "choir")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
	fileDigest, err := fileSHA256(path)
	if err != nil {
		t.Fatal(err)
	}
	manifest := ReleaseManifest{
		Version: 1, ComputerID: computerID, AcceptedEventHead: strings.Repeat("a", 64), CodeRef: "code:" + operationID,
		ArtifactProgramRef: "artifact-program:" + operationID, EventSchemaVersion: 1, ReducerVersion: 1, Marker: operationID,
		Files: []ManifestFile{{Path: "bin/choir", SHA256: fileDigest, Mode: 0o555}},
	}
	unsigned, err := computerevent.CanonicalJSON(manifest)
	if err != nil {
		t.Fatal(err)
	}
	manifest.ContentDigest = computerevent.DigestBytes(unsigned)
	request := ApplyRequest{ComputerID: computerID, RealizationID: realizationID, OperationID: operationID, IdempotencyKey: idempotencyKey, AcceptedEventHead: manifest.AcceptedEventHead, SourceDir: source, Manifest: manifest}
	request.RequestCommitment, err = validateApplyRequest(computerID, realizationID, request)
	if err != nil {
		t.Fatal(err)
	}
	return request
}

func makeTreeWritable(root string) {
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if entry.IsDir() {
			_ = os.Chmod(path, 0o700)
		} else {
			_ = os.Chmod(path, 0o600)
		}
		return nil
	})
}

func TestUpdaterImportsImmutableBaselineOnce(t *testing.T) {
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "updater")
	t.Cleanup(func() { makeTreeWritable(root) })
	engine, err := New(root, "computer-test", "realization-test", &fakeServiceManager{}, fakeHealthProber{}, computerevent.SigningKey{
		SignerRef: computerevent.SignerRef{SignerDomain: "guest-core", KeyID: "updater-test"}, PrivateKey: privateKey,
	})
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(root, "incoming", "genesis")
	if err := os.MkdirAll(filepath.Join(source, "bin"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "bin", "sandbox"), []byte("baseline"), 0o555); err != nil {
		t.Fatal(err)
	}
	manifest, err := BuildBaselineManifest(source, "computer-test", "code:baseline", "artifact-program:baseline")
	if err != nil {
		t.Fatal(err)
	}
	request := BaselineImportRequest{
		ComputerID: "computer-test", RealizationID: "realization-test", IdempotencyKey: "genesis-1",
		SourceDir: source, Manifest: manifest,
	}
	request.RequestCommitment, err = ComputeBaselineImportCommitment(request)
	if err != nil {
		t.Fatal(err)
	}
	imported, err := engine.ImportBaseline(request)
	if err != nil || imported.ContentDigest != manifest.ContentDigest {
		t.Fatalf("baseline import=%+v err=%v", imported, err)
	}
	replayed, err := engine.ImportBaseline(request)
	if err != nil || replayed.ContentDigest != imported.ContentDigest {
		t.Fatalf("baseline replay=%+v err=%v", replayed, err)
	}
	if current, err := ReadCurrentManifest(root); err != nil || current.ContentDigest != imported.ContentDigest {
		t.Fatalf("current baseline=%+v err=%v", current, err)
	}
	changed := request
	changed.Manifest.Marker = "changed"
	changed.Manifest, _ = FinalizeManifest(changed.Manifest)
	changed.RequestCommitment, _ = ComputeBaselineImportCommitment(changed)
	if _, err := engine.ImportBaseline(changed); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("changed baseline error=%v", err)
	}
}
