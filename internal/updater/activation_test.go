package updater

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

func TestAdmitCurrentRequiresSignedReceiptAndVerifiedFiles(t *testing.T) {
	engine, root := activationTestUpdater(t)
	request := updaterRequestFixture(t, root, "computer-test", "realization-test", "operation-1", "idem-1", "#!/bin/sh\nexit 0\n")
	result, err := engine.Apply(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	admitted, err := engine.AdmitCurrent(context.Background())
	if err != nil || admitted.ReleaseDigest != request.Manifest.ContentDigest || admitted.ReceiptKind != "MaterializationReceipt" {
		t.Fatalf("admitted=%+v err=%v", admitted, err)
	}
	if err := exec.Command(admitted.SandboxPath).Run(); err != nil {
		t.Fatalf("admitted sandbox is not directly executable: %v", err)
	}
	reconstructed, err := New(root, "computer-test", "realization-reconstructed", &fakeServiceManager{}, fakeHealthProber{}, engine.signer)
	if err != nil {
		t.Fatal(err)
	}
	if rebuilt, rebuildErr := reconstructed.AdmitCurrent(context.Background()); rebuildErr != nil || rebuilt.ReleaseDigest != admitted.ReleaseDigest {
		t.Fatalf("reconstructed admission=%+v err=%v", rebuilt, rebuildErr)
	}
	if err := os.Remove(activationReceiptPath(root, request.Manifest.ContentDigest)); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.AdmitCurrent(context.Background()); !errors.Is(err, ErrNoAuthorizedRelease) {
		t.Fatalf("unsigned current admitted: %v", err)
	}
	if err := writeActivationReceipt(root, request.Manifest.ContentDigest, result.MaterializationReceipt); err != nil {
		t.Fatal(err)
	}
	makeTreeWritable(filepath.Join(root, "releases", request.Manifest.ContentDigest))
	if err := os.WriteFile(filepath.Join(root, "releases", request.Manifest.ContentDigest, "bin", "sandbox"), []byte("tampered"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.AdmitCurrent(context.Background()); !errors.Is(err, ErrNoAuthorizedRelease) {
		t.Fatalf("tampered current admitted: %v", err)
	}
}
func TestImportBaselineReplacesOnlyUnauthenticatedLegacyCurrent(t *testing.T) {
	engine, root := activationTestUpdater(t)
	legacy := updaterRequestFixture(t, root, "computer-test", "realization-test", "legacy", "legacy-apply", "legacy release")
	legacyImport := BaselineImportRequest{
		ComputerID: "computer-test", RealizationID: "realization-test", IdempotencyKey: "legacy-import",
		SourceDir: legacy.SourceDir, Manifest: legacy.Manifest,
	}
	legacyImport.RequestCommitment, _ = ComputeBaselineImportCommitment(legacyImport)
	if _, err := engine.ImportBaseline(context.Background(), legacyImport); err != nil {
		t.Fatal(err)
	}
	replacement := updaterRequestFixture(t, root, "computer-test", "realization-test", "replacement", "replacement-apply", "replacement release")
	replacementImport := BaselineImportRequest{
		ComputerID: "computer-test", RealizationID: "realization-test", IdempotencyKey: "replacement-import",
		SourceDir: replacement.SourceDir, Manifest: replacement.Manifest, ReplaceUnauthenticatedCurrent: true,
	}
	replacementImport.RequestCommitment, _ = ComputeBaselineImportCommitment(replacementImport)
	if imported, err := engine.ImportBaseline(context.Background(), replacementImport); err != nil || imported.ContentDigest != replacement.Manifest.ContentDigest {
		t.Fatalf("replace legacy current=%+v err=%v", imported, err)
	}
	authorized := updaterRequestFixture(t, root, "computer-test", "realization-test", "authorized", "authorized-apply", "authorized release")
	if _, err := engine.Apply(context.Background(), authorized); err != nil {
		t.Fatal(err)
	}
	legacyImport.ReplaceUnauthenticatedCurrent = true
	legacyImport.RequestCommitment, _ = ComputeBaselineImportCommitment(legacyImport)
	if _, err := engine.ImportBaseline(context.Background(), legacyImport); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("authorized current replaced: %v", err)
	}
}

func TestApplyPersistsIntentBeforeRestartAndRecoveryAuthority(t *testing.T) {
	engine, root := activationTestUpdater(t)
	service := engine.service.(*activationInspectingService)
	service.engine = engine
	first := updaterRequestFixture(t, root, "computer-test", "realization-test", "operation-1", "idem-1", "first release")
	if _, err := engine.Apply(context.Background(), first); err != nil {
		t.Fatal(err)
	}
	if service.admittedKind != ActivationIntentReceiptKind {
		t.Fatalf("restart admission kind=%q", service.admittedKind)
	}
	second := updaterRequestFixture(t, root, "computer-test", "realization-test", "operation-2", "idem-2", "second release")
	engine.health = fakeHealthProber{failDigest: second.Manifest.ContentDigest}
	failed, err := engine.Apply(context.Background(), second)
	if err == nil || failed.RecoveryReceipt == nil {
		t.Fatalf("failed apply=%+v err=%v", failed, err)
	}
	admitted, admitErr := engine.AdmitCurrent(context.Background())
	if admitErr != nil || admitted.ReleaseDigest != first.Manifest.ContentDigest || admitted.ReceiptKind != "UpdaterRecoveryReceipt" {
		t.Fatalf("recovery admission=%+v err=%v", admitted, admitErr)
	}
}

type activationInspectingService struct {
	engine       *Updater
	admittedKind string
}

func (s *activationInspectingService) Restart(ctx context.Context) error {
	admitted, err := s.engine.AdmitCurrent(ctx)
	if err != nil {
		return err
	}
	s.admittedKind = admitted.ReceiptKind
	return nil
}
func (s *activationInspectingService) RecoveryRestart(context.Context) error           { return nil }
func (s *activationInspectingService) CleanupRecoveryCredential(context.Context) error { return nil }

func activationTestUpdater(t *testing.T) (*Updater, string) {
	t.Helper()
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(t.TempDir(), "updater")
	t.Cleanup(func() { makeTreeWritable(root) })
	service := &activationInspectingService{}
	engine, err := New(root, "computer-test", "realization-test", service, fakeHealthProber{}, testReceiptSigner{key: computerevent.SigningKey{
		SignerRef: computerevent.SignerRef{SignerDomain: "guest-core", KeyID: "activation-test"}, PrivateKey: privateKey,
	}})
	if err != nil {
		t.Fatal(err)
	}
	service.engine = engine
	engine.now = func() time.Time { return time.Date(2026, 7, 21, 16, 0, 0, 0, time.UTC) }
	return engine, root
}
