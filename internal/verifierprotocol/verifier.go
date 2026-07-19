package verifierprotocol

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

type Request struct {
	Version                 int      `json:"version"`
	ComputerID              string   `json:"computer_id"`
	OperationID             string   `json:"operation_id"`
	BundleDigest            string   `json:"bundle_digest"`
	VerificationEventDigest string   `json:"verification_event_digest"`
	VerifierEvidenceRefs    []string `json:"verifier_evidence_refs"`
	DecisionEventHead       string   `json:"decision_event_head"`
	CodeRef                 string   `json:"code_ref"`
	ArtifactProgramRef      string   `json:"artifact_program_ref"`
	ReleaseDigest           string   `json:"release_digest"`
	Decision                string   `json:"decision"`
}

type Response struct {
	Request     Request               `json:"request"`
	Certificate computerevent.Receipt `json:"certificate"`
	PublicKey   string                `json:"public_key"`
}
type VerifierCertificateRequest = Request

type VerifierCertificateResponse = Response

var NewVerifierCertificate = NewCertificate

var VerifyVerifierCertificate = Verify

func NewCertificate(request Request, key computerevent.SigningKey, now time.Time) (computerevent.Receipt, error) {
	if err := validateRequest(request); err != nil {
		return computerevent.Receipt{}, err
	}
	if key.SignerDomain != "verifier-control" || key.KeyID == "" || len(key.PrivateKey) != ed25519.PrivateKeySize {
		return computerevent.Receipt{}, fmt.Errorf("verifier certificate: independent signing key is required")
	}
	canonical, err := computerevent.CanonicalJSON(request)
	if err != nil {
		return computerevent.Receipt{}, err
	}
	return computerevent.NewSignedReceipt("VerifierCertificate", "choir-verifier", map[string]any{"request": json.RawMessage(canonical)}, []computerevent.SigningKey{key}, now.UTC())
}

func Verify(response Response) error {
	if err := validateRequest(response.Request); err != nil {
		return err
	}
	publicKey, err := base64.RawStdEncoding.DecodeString(response.PublicKey)
	if err != nil || len(publicKey) != ed25519.PublicKeySize {
		return fmt.Errorf("verifier certificate: invalid public key")
	}
	if response.Certificate.ReceiptKind != "VerifierCertificate" || response.Certificate.Issuer != "choir-verifier" ||
		len(response.Certificate.RequiredSigners) != 1 || response.Certificate.RequiredSigners[0].SignerDomain != "verifier-control" {
		return fmt.Errorf("verifier certificate: signature refused")
	}
	resolver := certificateKeyResolver{keyID: response.Certificate.RequiredSigners[0].KeyID, publicKey: ed25519.PublicKey(publicKey)}
	if response.Certificate.Verify(resolver) != nil || response.Certificate.RequireKindFields("request") != nil {
		return fmt.Errorf("verifier certificate: signature refused")
	}
	expected, _ := computerevent.CanonicalJSON(response.Request)
	actual, err := computerevent.CanonicalJSON(response.Certificate.KindFields["request"])
	if err != nil || !bytes.Equal(expected, actual) {
		return fmt.Errorf("verifier certificate: request binding mismatch")
	}
	return nil
}

func validateRequest(request Request) error {
	if request.Version != 1 || request.ComputerID == "" || request.OperationID == "" ||
		!computerevent.IsSHA256(request.BundleDigest) || !computerevent.IsSHA256(request.VerificationEventDigest) ||
		len(request.VerifierEvidenceRefs) == 0 || !computerevent.IsSHA256(request.DecisionEventHead) ||
		request.CodeRef == "" || request.ArtifactProgramRef == "" || !computerevent.IsSHA256(request.ReleaseDigest) ||
		(request.Decision != "pass" && request.Decision != "genesis_baseline" && request.Decision != "rollback_prior_verified") {
		return fmt.Errorf("verifier certificate: complete exact bindings are required")
	}
	return nil
}

type certificateKeyResolver struct {
	keyID     string
	publicKey ed25519.PublicKey
}

func (r certificateKeyResolver) ResolveReceiptKey(domain, _ string, keyID string, _ uint64, _ time.Time) (ed25519.PublicKey, error) {
	if domain != "verifier-control" || keyID != r.keyID {
		return nil, fmt.Errorf("verifier certificate: signing key refused")
	}
	return append(ed25519.PublicKey(nil), r.publicKey...), nil
}
