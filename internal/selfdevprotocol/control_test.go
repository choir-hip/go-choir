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
		VMLocalContentWitness: VMLocalContentWitness{
			Database: "choir", ContentRoot: digest('7'), DoltHead: digest('8'), DerivabilityDigest: digest('9'),
			Schema: map[string]string{"agents": digest('a')},
			Tables: map[string]string{"agents": digest('b')},
		},
		FrontendIdentity: FrontendIdentity{Digest: digest('c'), Derivation: FrontendDerivationExplicit},
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

func ownerRecoveryTestRequest() CheckpointRequest {
	digest := func(value byte) string { return strings.Repeat(string(value), 64) }
	return CheckpointRequest{
		ComputerID: "computer-owner-recovery", IdempotencyKey: "owner-recovery-test",
		ComputerVersion: computerversion.ComputerVersion{
			CodeRef:            computerversion.CodeRef("code:sha256:" + digest('d')),
			ArtifactProgramRef: computerversion.ArtifactProgramRef("artifact-program:sha256:" + digest('e')),
		},
		AcceptedEventHead: digest('1'), EffectiveEventHead: digest('2'), EffectiveStateCommitment: digest('2'),
		EventHeadReceiptID: "receipt-test", ReleaseDigest: digest('f'), ReconstructionDigest: digest('3'),
		ReducerVersion: 1, OwnerRecovery: true,
		VMLocalContentWitness: VMLocalContentWitness{
			Database: "texture", ContentRoot: digest('7'), DoltHead: digest('8'), DerivabilityDigest: digest('9'),
			Schema: map[string]string{"agents": digest('a')},
			Tables: map[string]string{"agents": digest('b')},
		},
		FrontendIdentity: FrontendIdentity{Digest: digest('c'), Derivation: FrontendDerivationRelease, ReleaseDigest: digest('f')},
	}
}

func TestOwnerRecoveryCheckpointAcceptsDistinctEvidenceClass(t *testing.T) {
	request := ownerRecoveryTestRequest()
	if _, _, err := CheckpointFromRequest(request); err != nil {
		t.Fatal(err)
	}
}

func TestOwnerRecoveryCheckpointAllowsCanonicalEffectiveDivergence(t *testing.T) {
	request := ownerRecoveryTestRequest()
	request.AcceptedEventHead = strings.Repeat("a", 64)
	request.EffectiveEventHead = strings.Repeat("b", 64)
	if _, _, err := CheckpointFromRequest(request); err != nil {
		t.Fatalf("owner-recovery must bind canonical and effective heads separately: %v", err)
	}
}

func TestOwnerRecoveryCheckpointRefusesVerifierEvidenceBlending(t *testing.T) {
	request := ownerRecoveryTestRequest()
	request.MaterializationReceiptDigest = strings.Repeat("c", 64)
	if _, _, err := CheckpointFromRequest(request); err == nil {
		t.Fatal("owner-recovery accepted a materialization receipt digest")
	}
	blend := ownerRecoveryTestRequest()
	blend.VerifierCertificateDigest = strings.Repeat("d", 64)
	if _, _, err := CheckpointFromRequest(blend); err == nil {
		t.Fatal("owner-recovery accepted a verifier certificate digest")
	}
	bootstrap := ownerRecoveryTestRequest()
	bootstrap.VerifierTrustBootstrap = true
	if _, _, err := CheckpointFromRequest(bootstrap); err == nil {
		t.Fatal("owner-recovery accepted verifier trust bootstrap")
	}
	certificate := ownerRecoveryTestRequest()
	certificate.VerifierCertificate.PublicKey = "not-empty"
	if _, _, err := CheckpointFromRequest(certificate); err == nil {
		t.Fatal("owner-recovery accepted a verifier public key")
	}
	sneak := ownerRecoveryTestRequest()
	sneak.VerifierCertificate.Request.ComputerID = "computer-sneak"
	if _, _, err := CheckpointFromRequest(sneak); err == nil {
		t.Fatal("owner-recovery accepted a non-empty verifier certificate request")
	}
}

func TestOwnerRecoveryCheckpointCannotAuthorizeRouteProjection(t *testing.T) {
	checkpoint, _, err := CheckpointFromRequest(ownerRecoveryTestRequest())
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = RouteProjectionFromRequest(RouteProjectionRequest{
		Checkpoint: CheckpointResponse{Checkpoint: checkpoint},
	}, time.Now().UTC())
	if err == nil {
		t.Fatal("owner-recovery checkpoint authorized route projection")
	}
	if !strings.Contains(err.Error(), "owner-recovery") {
		t.Fatalf("route projection refusal = %v", err)
	}
}
