package selfdevprotocol

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/computerversion"
)

func TestVerifierCertificateBindsIndependentDecisionAndCheckpoint(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	digest := func(value byte) string { return strings.Repeat(string(value), 64) }
	key := computerevent.SigningKey{SignerRef: computerevent.SignerRef{SignerDomain: "verifier-control", KeyID: "verifier-test"}, PrivateKey: privateKey}
	request := VerifierCertificateRequest{
		Version: 1, ComputerID: "computer-test", OperationID: "operation-test",
		BundleDigest: digest('a'), VerificationEventDigest: digest('b'), VerifierEvidenceRefs: []string{digest('b')},
		DecisionEventHead: digest('c'), CodeRef: "code:sha256:" + digest('d'), ArtifactProgramRef: "artifact-program:sha256:" + digest('e'),
		ReleaseDigest: digest('f'), Decision: "pass",
	}
	certificate, err := NewVerifierCertificate(request, key, time.Date(2026, 7, 19, 6, 30, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	response := VerifierCertificateResponse{Request: request, Certificate: certificate, PublicKey: base64.RawStdEncoding.EncodeToString(publicKey)}
	if err := VerifyVerifierCertificate(response); err != nil {
		t.Fatal(err)
	}
	certificateJSON, _ := computerevent.CanonicalJSON(certificate)
	checkpoint := CheckpointRequest{
		ComputerID: request.ComputerID, IdempotencyKey: "checkpoint-test",
		ComputerVersion:   computerversion.ComputerVersion{CodeRef: computerversion.CodeRef(request.CodeRef), ArtifactProgramRef: computerversion.ArtifactProgramRef(request.ArtifactProgramRef)},
		AcceptedEventHead: digest('1'), EffectiveEventHead: digest('1'), EffectiveStateCommitment: digest('2'), EventHeadReceiptID: "receipt-test",
		ReleaseDigest: request.ReleaseDigest, ReconstructionDigest: digest('3'), MaterializationReceiptDigest: digest('4'),
		VerifierCertificateDigest: computerevent.DigestBytes(certificateJSON), VerifierCertificate: response, ReducerVersion: 1,
	}
	if _, _, err := CheckpointFromRequest(checkpoint); err != nil {
		t.Fatal(err)
	}
	tampered := checkpoint
	tampered.VerifierCertificate.Request.ReleaseDigest = digest('9')
	if _, _, err := CheckpointFromRequest(tampered); err == nil {
		t.Fatal("checkpoint accepted a substituted verifier certificate request")
	}
}
