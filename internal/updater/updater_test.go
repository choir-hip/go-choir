package updater

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

type fakeServiceManager struct {
	restarts   int
	recoveries int
	cleanups   int
	restartErr error
}

func (s *fakeServiceManager) Restart(context.Context) error         { s.restarts++; return s.restartErr }
func (s *fakeServiceManager) RecoveryRestart(context.Context) error { s.recoveries++; return nil }
func (s *fakeServiceManager) CleanupRecoveryCredential(context.Context) error {
	s.cleanups++
	return nil
}

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
	updater, err := New(root, "computer-test", "realization-test", service, prober, testReceiptSigner{key: computerevent.SigningKey{SignerRef: computerevent.SignerRef{SignerDomain: "guest-core", KeyID: "updater-test"},
		PrivateKey: privateKey}})
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
	if service.restarts != 2 || service.recoveries != 1 || service.cleanups != 2 {
		t.Fatalf("failure recovery counts restart=%d recovery=%d cleanup=%d", service.restarts, service.recoveries, service.cleanups)
	}
	service.restartErr = errors.New("restart failed")
	third := updaterRequestFixture(t, root, "computer-test", "realization-test", "operation-3", "idem-3", "restart failure")
	restartFailed, err := updater.Apply(context.Background(), third)
	if err == nil || restartFailed.Outcome != "failed" || restartFailed.RecoveryReceipt == nil {
		t.Fatalf("restart failed apply = %+v err=%v", restartFailed, err)
	}
	currentDigest, _, err = updater.currentRelease()
	if err != nil || currentDigest != first.Manifest.ContentDigest {
		t.Fatalf("current release after restart recovery = %q, %v; want %q", currentDigest, err, first.Manifest.ContentDigest)
	}
	if service.restarts != 3 || service.recoveries != 2 || service.cleanups != 3 {
		t.Fatalf("restart failure recovery counts restart=%d recovery=%d cleanup=%d", service.restarts, service.recoveries, service.cleanups)
	}
}

type processRestartManager struct {
	root    string
	output  string
	process *os.Process
}

func (m *processRestartManager) Restart(context.Context) error {
	if m.process != nil {
		_ = m.process.Kill()
		_, _ = m.process.Wait()
	}
	command := exec.Command("/bin/sh", filepath.Join(m.root, "current", "bin", "choir"))
	command.Env = append(os.Environ(), "CHOIR_TEST_OUTPUT="+m.output)
	if err := command.Start(); err != nil {
		return err
	}
	m.process = command.Process
	return nil
}
func (m *processRestartManager) RecoveryRestart(ctx context.Context) error       { return m.Restart(ctx) }
func (m *processRestartManager) CleanupRecoveryCredential(context.Context) error { return nil }

type processHealthProber struct{ output string }

func (p processHealthProber) Probe(ctx context.Context, _ string, manifest ReleaseManifest) ([]string, error) {
	for {
		raw, err := os.ReadFile(p.output)
		if err == nil && strings.TrimSpace(string(raw)) == manifest.Marker {
			return []string{computerevent.DigestBytes(raw)}, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func TestUpdaterRestartsRealReleaseProcessAcrossPointerSwap(t *testing.T) {
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "updater")
	t.Cleanup(func() { makeTreeWritable(root) })
	manager := &processRestartManager{root: root, output: filepath.Join(t.TempDir(), "running-release")}
	t.Cleanup(func() {
		if manager.process != nil {
			_ = manager.process.Kill()
			_, _ = manager.process.Wait()
		}
	})
	engine, err := New(root, "computer-test", "realization-test", manager, processHealthProber{output: manager.output}, testReceiptSigner{key: computerevent.SigningKey{SignerRef: computerevent.SignerRef{SignerDomain: "guest-core", KeyID: "process-restart-test"}, PrivateKey: privateKey}})
	if err != nil {
		t.Fatal(err)
	}
	script := func(marker string) string {
		return fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' %q > \"$CHOIR_TEST_OUTPUT\"\nexec sleep 30\n", marker)
	}
	first := updaterRequestFixture(t, root, "computer-test", "realization-test", "process-v1", "process-idem-v1", script("process-v1"))
	if result, err := engine.Apply(t.Context(), first); err != nil || result.Outcome != "applied" {
		t.Fatalf("first process apply = %+v, %v", result, err)
	}
	firstPID := manager.process.Pid
	second := updaterRequestFixture(t, root, "computer-test", "realization-test", "process-v2", "process-idem-v2", script("process-v2"))
	if result, err := engine.Apply(t.Context(), second); err != nil || result.Outcome != "applied" {
		t.Fatalf("second process apply = %+v, %v", result, err)
	}
	if manager.process.Pid == firstPID {
		t.Fatalf("release process PID did not change: %d", firstPID)
	}
	raw, err := os.ReadFile(manager.output)
	if err != nil || strings.TrimSpace(string(raw)) != "process-v2" {
		t.Fatalf("running release marker = %q, %v", raw, err)
	}
}

func TestUpdaterRejectsSymlinkManifestEntryBeforePointerMutation(t *testing.T) {
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "updater")
	t.Cleanup(func() { makeTreeWritable(root) })
	updater, err := New(root, "computer-test", "realization-test", &fakeServiceManager{}, fakeHealthProber{}, testReceiptSigner{key: computerevent.SigningKey{SignerRef: computerevent.SignerRef{SignerDomain: "guest-core", KeyID: "updater-test"}, PrivateKey: privateKey}})
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
	recoveryPath := filepath.Join(filepath.Dir(path), "recover")
	cleanupPath := filepath.Join(filepath.Dir(path), "cleanup")
	manager.RecoveryPath, manager.CleanupPath = recoveryPath, cleanupPath
	if err := manager.RecoveryRestart(context.Background()); err != nil {
		t.Fatal(err)
	}
	if raw, err := os.ReadFile(recoveryPath); err != nil || string(raw) != "recover\n" {
		t.Fatalf("recovery request = %q, %v", raw, err)
	}
	if err := manager.CleanupRecoveryCredential(context.Background()); err != nil {
		t.Fatal(err)
	}
	if raw, err := os.ReadFile(cleanupPath); err != nil || string(raw) != "cleanup\n" {
		t.Fatalf("cleanup request = %q, %v", raw, err)
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
	choirPath := filepath.Join(source, "bin", "choir")
	sandboxPath := filepath.Join(source, "bin", "sandbox")
	skillPath := filepath.Join(source, "share", "go-choir", "skills", "default.txt")
	if err := os.MkdirAll(filepath.Dir(choirPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(skillPath), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{choirPath, sandboxPath} {
		if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(skillPath, []byte("skill"), 0o644); err != nil {
		t.Fatal(err)
	}
	fileDigest, err := fileSHA256(choirPath)
	if err != nil {
		t.Fatal(err)
	}
	skillDigest, err := fileSHA256(skillPath)
	if err != nil {
		t.Fatal(err)
	}
	manifest := ReleaseManifest{
		Version: 1, ComputerID: computerID, AcceptedEventHead: strings.Repeat("a", 64), CodeRef: "code:" + operationID,
		ArtifactProgramRef: "artifact-program:" + operationID, EventSchemaVersion: 1, ReducerVersion: 1, Marker: operationID,
		Files: []ManifestFile{{Path: "bin/choir", SHA256: fileDigest, Mode: 0o555}, {Path: "bin/sandbox", SHA256: fileDigest, Mode: 0o555}, {Path: "share/go-choir/skills/default.txt", SHA256: skillDigest, Mode: 0o444}},
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
	engine, err := New(root, "computer-test", "realization-test", &fakeServiceManager{}, fakeHealthProber{}, testReceiptSigner{key: computerevent.SigningKey{SignerRef: computerevent.SignerRef{SignerDomain: "guest-core", KeyID: "updater-test"}, PrivateKey: privateKey}})
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
	imported, err := engine.ImportBaseline(context.Background(), request)
	if err != nil || imported.ContentDigest != manifest.ContentDigest {
		t.Fatalf("baseline import=%+v err=%v", imported, err)
	}
	replayed, err := engine.ImportBaseline(context.Background(), request)
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
	if _, err := engine.ImportBaseline(context.Background(), changed); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("changed baseline error=%v", err)
	}
}
