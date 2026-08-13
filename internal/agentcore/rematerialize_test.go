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
	rt := &Runtime{
		cfg:           provideriface.Config{ComputerID: computerID, StorePath: storePath},
		store:         live,
		eventAppender: appender,
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
	checkpoint := rematerializeTestCheckpoint(t, computerID, witness)

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
	if !result.WitnessMatched || !result.OriginalDenied || !result.StoreClosed || result.PinCheckoutUsed || result.FrontendRestaged {
		t.Fatalf("unexpected rematerialize report %#v", result)
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
	rt := &Runtime{
		cfg:           provideriface.Config{ComputerID: computerID, StorePath: storePath},
		store:         live,
		eventAppender: appender,
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
	checkpoint := rematerializeTestCheckpoint(t, computerID, witness)
	if _, err := rt.RematerializeFromTape(ctx, computerID, checkpoint); err == nil {
		t.Fatal("mismatched checkpoint witness was accepted")
	}
	if rt.store == nil {
		t.Fatal("mismatch closed the original realization")
	}
	if _, err := os.Stat(storePath); err != nil {
		t.Fatalf("original marker was moved on mismatch: %v", err)
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

func rematerializeTestCheckpoint(t *testing.T, computerID string, witness selfdevprotocol.VMLocalContentWitness) selfdevprotocol.Checkpoint {
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
		ReleaseDigest: digest('f'), Decision: "pass",
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
		FrontendIdentity:      selfdevprotocol.FrontendIdentity{Digest: digest('c'), Derivation: selfdevprotocol.FrontendDerivationExplicit},
	}
	checkpoint, _, err := selfdevprotocol.CheckpointFromRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	return checkpoint
}
