package agentcore

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/computerversion"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
	choirstore "github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/updater"
)

func TestRematerializeFromTapeQuarantinesOriginalAndDropsLiveOnlyRows(t *testing.T) {
	computerID := "computer-rematerialize"
	storePath := filepath.Join(t.TempDir(), "runtime.db")
	live, err := choirstore.Open(storePath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if live != nil {
			_ = live.Close()
		}
	}()

	authority := emptyReplayAuthority{}
	appender, err := computerevent.NewComputerEventAppender(computerID, authority, live, authority, authority)
	if err != nil {
		t.Fatal(err)
	}
	updaterRoot := filepath.Join(t.TempDir(), "updater")
	priorDigest, _ := pinFrontendRelease(t, updaterRoot, computerID, "<html>live</html>")
	targetDigest, targetIdentity := pinFrontendRelease(t, updaterRoot, computerID, "<html>checkpoint</html>")
	pointCurrent(t, updaterRoot, priorDigest)
	rt := &Runtime{
		cfg:                provideriface.Config{ComputerID: computerID, StorePath: storePath},
		store:              live,
		eventAppender:      appender,
		selfdevUpdaterRoot: updaterRoot,
	}
	ctx := context.Background()
	report, err := rt.ReplayCompleteness(ctx, computerID)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Result.Equivalent() {
		t.Fatalf("clean projections were not equivalent: %#v", report.Result)
	}
	witness, err := selfdevprotocol.WitnessFromObservationSets(report.Live, report.Replay, report.Result)
	if err != nil {
		t.Fatal(err)
	}
	checkpoint := rematerializeTestCheckpoint(t, computerID, witness, targetDigest, targetIdentity)

	if _, err := live.DB().ExecContext(ctx, "CREATE TABLE rematerialize_live_only (id VARCHAR(64) PRIMARY KEY, value VARCHAR(64) NOT NULL)"); err != nil {
		t.Fatal(err)
	}
	if _, err := live.DB().ExecContext(ctx, "INSERT INTO rematerialize_live_only (id, value) VALUES ('drift', 'live-only')"); err != nil {
		t.Fatal(err)
	}

	result, err := rt.RematerializeFromTape(ctx, computerID, checkpoint)
	if err != nil {
		t.Fatalf("tape rematerialize refused: %v", err)
	}
	live = nil
	if rt.store != nil {
		t.Fatal("runtime kept the original store handle")
	}
	if !result.WitnessMatched || !result.OriginalDenied || !result.StoreClosed || result.PinCheckoutUsed || !result.FrontendRestaged {
		t.Fatalf("unexpected rematerialize report %#v", result)
	}
	served, err := os.ReadFile(filepath.Join(updaterRoot, "current", "frontend", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if string(served) != "<html>checkpoint</html>" {
		t.Fatalf("served SPA = %q", served)
	}
	if _, err := os.Stat(result.QuarantinedMarker); err != nil {
		t.Fatalf("quarantined marker missing: %v", err)
	}

	quarantine, err := choirstore.Open(result.QuarantinedMarker)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = quarantine.Close() })
	var quarantined int
	if err := quarantine.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'rematerialize_live_only'`).Scan(&quarantined); err != nil {
		t.Fatal(err)
	}
	if quarantined == 0 {
		t.Fatal("original live-only rows were not preserved in quarantine")
	}

	restored, err := choirstore.Open(storePath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = restored.Close() })
	var surviving int
	if err := restored.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'rematerialize_live_only'`).Scan(&surviving); err != nil {
		t.Fatal(err)
	}
	if surviving != 0 {
		t.Fatal("rematerialize copied surviving live-only rows instead of reconstructing from the tape")
	}
}

func TestRematerializeFromTapeKeepsOriginalOnWitnessMismatch(t *testing.T) {
	computerID := "computer-rematerialize-mismatch"
	storePath := filepath.Join(t.TempDir(), "runtime.db")
	live, err := choirstore.Open(storePath)
	if err != nil {
		t.Fatal(err)
	}
	defer live.Close()

	authority := emptyReplayAuthority{}
	appender, err := computerevent.NewComputerEventAppender(computerID, authority, live, authority, authority)
	if err != nil {
		t.Fatal(err)
	}
	updaterRoot := filepath.Join(t.TempDir(), "updater")
	liveDigest, _ := pinFrontendRelease(t, updaterRoot, computerID, "<html>live</html>")
	targetDigest, targetIdentity := pinFrontendRelease(t, updaterRoot, computerID, "<html>checkpoint</html>")
	pointCurrent(t, updaterRoot, liveDigest)
	rt := &Runtime{
		cfg:                provideriface.Config{ComputerID: computerID, StorePath: storePath},
		store:              live,
		eventAppender:      appender,
		selfdevUpdaterRoot: updaterRoot,
	}
	ctx := context.Background()
	report, err := rt.ReplayCompleteness(ctx, computerID)
	if err != nil {
		t.Fatal(err)
	}
	witness, err := selfdevprotocol.WitnessFromObservationSets(report.Live, report.Replay, report.Result)
	if err != nil {
		t.Fatal(err)
	}
	witness.ContentRoot = strings.Repeat("0", 64)
	checkpoint := rematerializeTestCheckpoint(t, computerID, witness, targetDigest, targetIdentity)
	if _, err := rt.RematerializeFromTape(ctx, computerID, checkpoint); err == nil {
		t.Fatal("mismatched checkpoint witness was accepted")
	}
	if rt.store == nil {
		t.Fatal("mismatch closed the original realization")
	}
	if _, err := os.Stat(storePath); err != nil {
		t.Fatalf("original marker was moved on mismatch: %v", err)
	}
	served, err := os.ReadFile(filepath.Join(updaterRoot, "current", "frontend", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if string(served) != "<html>live</html>" {
		t.Fatalf("mismatch restaged SPA = %q", served)
	}
}

func TestRematerializeFromTapeProductPath(t *testing.T) {
	computerID := "computer-rematerialize-api"
	rt := &Runtime{cfg: provideriface.Config{ComputerID: computerID}}
	body, _ := json.Marshal(rematerializeAPIRequest{})
	request := httptest.NewRequest(http.MethodPost, "/api/computers/"+computerID+"/lifecycle/rematerialize-from-tape", bytes.NewReader(body))
	request.Header.Set("X-Authenticated-User", "owner-rematerialize")
	request.Header.Set("X-Authenticated-Computer", computerID)
	response := httptest.NewRecorder()
	NewAPIHandler(rt).HandleComputersRouter(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("rematerialize status=%d body=%s", response.Code, response.Body.String())
	}
	if bytes.Contains(response.Body.Bytes(), []byte("genesis")) {
		t.Fatal("rematerialize used genesis")
	}
}

func rematerializeTestCheckpoint(t *testing.T, computerID string, witness selfdevprotocol.VMLocalContentWitness, releaseDigest string, frontend selfdevprotocol.FrontendIdentity) selfdevprotocol.Checkpoint {
	t.Helper()
	digest := func(value byte) string { return strings.Repeat(string(value), 64) }
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	key := computerevent.SigningKey{SignerRef: computerevent.SignerRef{SignerDomain: "verifier-control", KeyID: "verifier-test"}, PrivateKey: privateKey}
	verifierRequest := selfdevprotocol.VerifierCertificateRequest{
		Version: 1, ComputerID: computerID, OperationID: "operation-test",
		BundleDigest: digest('a'), VerificationEventDigest: digest('b'), VerifierEvidenceRefs: []string{digest('b')},
		DecisionEventHead: digest('c'), CodeRef: "code:sha256:" + digest('d'), ArtifactProgramRef: "artifact-program:sha256:" + digest('e'),
		ReleaseDigest: releaseDigest, Decision: "pass",
	}
	certificate, err := selfdevprotocol.NewVerifierCertificate(verifierRequest, key, time.Date(2026, 7, 19, 6, 30, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	response := selfdevprotocol.VerifierCertificateResponse{
		Request: verifierRequest, Certificate: certificate, PublicKey: base64.RawStdEncoding.EncodeToString(publicKey),
	}
	certificateJSON, _ := computerevent.CanonicalJSON(certificate)
	request := selfdevprotocol.CheckpointRequest{
		ComputerID: computerID, IdempotencyKey: "checkpoint-test",
		ComputerVersion: computerversion.ComputerVersion{
			CodeRef: computerversion.CodeRef(verifierRequest.CodeRef), ArtifactProgramRef: computerversion.ArtifactProgramRef(verifierRequest.ArtifactProgramRef),
		},
		AcceptedEventHead: digest('1'), EffectiveEventHead: digest('1'), EffectiveStateCommitment: digest('2'), EventHeadReceiptID: "receipt-test",
		ReleaseDigest: verifierRequest.ReleaseDigest, ReconstructionDigest: digest('3'), MaterializationReceiptDigest: digest('4'),
		VerifierCertificateDigest: computerevent.DigestBytes(certificateJSON), VerifierCertificate: response, ReducerVersion: 1,
		VMLocalContentWitness: witness,
		FrontendIdentity:      frontend,
	}
	checkpoint, _, err := selfdevprotocol.CheckpointFromRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	return checkpoint
}

func TestRematerializeFromTapeRefusesMissingFrontend(t *testing.T) {
	computerID := "computer-rematerialize-missing-spa"
	storePath := filepath.Join(t.TempDir(), "runtime.db")
	live, err := choirstore.Open(storePath)
	if err != nil {
		t.Fatal(err)
	}
	defer live.Close()
	authority := emptyReplayAuthority{}
	appender, err := computerevent.NewComputerEventAppender(computerID, authority, live, authority, authority)
	if err != nil {
		t.Fatal(err)
	}
	updaterRoot := filepath.Join(t.TempDir(), "updater")
	liveDigest, liveIdentity := pinFrontendRelease(t, updaterRoot, computerID, "<html>live</html>")
	pointCurrent(t, updaterRoot, liveDigest)
	rt := &Runtime{
		cfg:                provideriface.Config{ComputerID: computerID, StorePath: storePath},
		store:              live,
		eventAppender:      appender,
		selfdevUpdaterRoot: updaterRoot,
	}
	ctx := context.Background()
	report, err := rt.ReplayCompleteness(ctx, computerID)
	if err != nil {
		t.Fatal(err)
	}
	witness, err := selfdevprotocol.WitnessFromObservationSets(report.Live, report.Replay, report.Result)
	if err != nil {
		t.Fatal(err)
	}
	missingDigest := strings.Repeat("a", 64)
	checkpoint := rematerializeTestCheckpoint(t, computerID, witness, missingDigest, selfdevprotocol.FrontendIdentity{
		Digest: liveIdentity.Digest, Derivation: selfdevprotocol.FrontendDerivationRelease, ReleaseDigest: missingDigest,
	})
	if _, err := rt.RematerializeFromTape(ctx, computerID, checkpoint); err == nil {
		t.Fatal("missing pinned frontend was accepted")
	}
	if rt.store == nil {
		t.Fatal("missing frontend closed the original realization")
	}
	served, err := os.ReadFile(filepath.Join(updaterRoot, "current", "frontend", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if string(served) != "<html>live</html>" {
		t.Fatalf("missing frontend restaged SPA = %q", served)
	}
}

func TestRestoreFromTapeProductPath(t *testing.T) {
	computerID := "computer-restore-api"
	rt := &Runtime{cfg: provideriface.Config{ComputerID: computerID}}
	body, _ := json.Marshal(restoreAPIRequest{
		OperandScopes: []string{selfdevprotocol.RestoreScopeVMLocal, selfdevprotocol.RestoreScopeFrontend},
	})
	request := httptest.NewRequest(http.MethodPost, "/api/computers/"+computerID+"/lifecycle/restore", bytes.NewReader(body))
	request.Header.Set("X-Authenticated-User", "owner-restore")
	request.Header.Set("X-Authenticated-Computer", computerID)
	response := httptest.NewRecorder()
	NewAPIHandler(rt).HandleComputersRouter(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("restore status=%d body=%s", response.Code, response.Body.String())
	}
	if bytes.Contains(response.Body.Bytes(), []byte("genesis")) {
		t.Fatal("restore used genesis")
	}
}

func TestRestoreFromTapeRefusesPlatformOperand(t *testing.T) {
	computerID := "computer-restore-platform"
	rt := &Runtime{cfg: provideriface.Config{ComputerID: computerID}}
	body, _ := json.Marshal(restoreAPIRequest{
		OperandScopes: []string{selfdevprotocol.RestoreScopeVMLocal, selfdevprotocol.RestoreScopeFrontend, selfdevprotocol.RestoreScopePlatform},
	})
	request := httptest.NewRequest(http.MethodPost, "/api/computers/"+computerID+"/lifecycle/restore", bytes.NewReader(body))
	request.Header.Set("X-Authenticated-User", "owner-restore")
	request.Header.Set("X-Authenticated-Computer", computerID)
	response := httptest.NewRecorder()
	NewAPIHandler(rt).HandleComputersRouter(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("platform restore status=%d body=%s", response.Code, response.Body.String())
	}
}

func pinFrontendRelease(t *testing.T, updaterRoot, computerID, spaHTML string) (string, selfdevprotocol.FrontendIdentity) {
	t.Helper()
	source := t.TempDir()
	files := map[string]string{
		"bin/choir":           "choir",
		"frontend/index.html": spaHTML,
	}
	for path, body := range files {
		full := filepath.Join(source, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	manifest, err := updater.BuildBaselineManifest(source, computerID, "code:sha256:"+strings.Repeat("d", 64), "artifact-program:sha256:"+strings.Repeat("e", 64))
	if err != nil {
		t.Fatal(err)
	}
	releaseDir := filepath.Join(updaterRoot, "releases", manifest.ContentDigest)
	if err := os.MkdirAll(releaseDir, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, file := range manifest.Files {
		src := filepath.Join(source, filepath.FromSlash(file.Path))
		dst := filepath.Join(releaseDir, filepath.FromSlash(file.Path))
		if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
			t.Fatal(err)
		}
		raw, err := os.ReadFile(src)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dst, raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	encoded, err := computerevent.CanonicalJSON(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(releaseDir, "release-manifest.json"), encoded, 0o644); err != nil {
		t.Fatal(err)
	}
	releaseFiles := make([]selfdevprotocol.ReleaseFile, 0, len(manifest.Files))
	for _, file := range manifest.Files {
		releaseFiles = append(releaseFiles, selfdevprotocol.ReleaseFile{Path: file.Path, SHA256: file.SHA256})
	}
	identity, err := selfdevprotocol.FrontendIdentityFromReleaseFiles(manifest.ContentDigest, releaseFiles)
	if err != nil {
		t.Fatal(err)
	}
	return manifest.ContentDigest, identity
}

func pointCurrent(t *testing.T, updaterRoot, digest string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(updaterRoot, "releases"), 0o700); err != nil {
		t.Fatal(err)
	}
	current := filepath.Join(updaterRoot, "current")
	_ = os.Remove(current)
	if err := os.Symlink(filepath.Join(updaterRoot, "releases", digest), current); err != nil {
		t.Fatal(err)
	}
}
