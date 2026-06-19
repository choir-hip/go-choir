package platform

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// revisionAttestationSchema identifies the per-revision signed payload. The
// signature attests the platform published the revision identified by
// RevisionHash. Because RevisionHash (mission D2) is a content-addressed
// commitment over canonical(body + citations + provenance + parent_hash), and
// provenance carries the system-attributed timestamp + authoring model, signing
// RevisionHash attests exactly "this revision, at this chain position, authored
// by this model at this time" — tamperproof and attributable. Bump only on a
// breaking payload shape change.
const revisionAttestationSchema = "choir.platform.revision_attestation.v0"

// revisionAttestation is the canonical, self-describing signed payload for one
// published revision. It is marshaled to deterministic JSON (struct fields in
// declaration order, no maps) and then Ed25519-signed.
type revisionAttestation struct {
	Schema       string `json:"schema"`
	RevisionHash string `json:"revision_hash"`
}

// SigningKey is the platform Ed25519 keypair that attests published revisions.
// The public half is published inside the version_history manifest so any
// reader/verifier can check per-revision signatures independently. KeyID is a
// short content-addressed id of the public key so signatures stay verifiable
// across a future key rotation (rotation is out of scope for this slice).
type SigningKey struct {
	Private ed25519.PrivateKey
	Public  ed25519.PublicKey
	KeyID   string
}

// LoadOrCreateSigningKey loads the platform signing key from path; if it does
// not exist it generates a fresh Ed25519 keypair and persists it (0600). The key
// is stored as raw Ed25519 private-key bytes. Auto-generation makes signing work
// out of the box on staging; production should override the path with a
// provisioned, backed-up key (PLATFORM_SIGNING_KEY_PATH).
func LoadOrCreateSigningKey(path string) (*SigningKey, error) {
	if path == "" {
		return nil, fmt.Errorf("signing key: path must not be empty")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("signing key: create dir %s: %w", filepath.Dir(path), err)
	}
	data, err := os.ReadFile(path)
	if err == nil {
		// Existing key: raw Ed25519 private-key bytes (64 bytes).
		priv := ed25519.PrivateKey(data)
		if len(priv) != ed25519.PrivateKeySize {
			return nil, fmt.Errorf("signing key: %s is %d bytes, want %d", path, len(priv), ed25519.PrivateKeySize)
		}
		return newSigningKey(priv), nil
	}
	if !os.IsNotExist(err) {
		return nil, fmt.Errorf("signing key: read %s: %w", path, err)
	}

	// No key yet: generate and persist.
	_, priv, gerr := ed25519.GenerateKey(nil)
	if gerr != nil {
		return nil, fmt.Errorf("signing key: generate: %w", gerr)
	}
	if werr := os.WriteFile(path, priv, 0o600); werr != nil {
		return nil, fmt.Errorf("signing key: write %s: %w", path, werr)
	}
	return newSigningKey(priv), nil
}

func newSigningKey(priv ed25519.PrivateKey) *SigningKey {
	pub := priv.Public().(ed25519.PublicKey)
	id := sha256.Sum256(pub)
	return &SigningKey{
		Private: priv,
		Public:  pub,
		KeyID:   hex.EncodeToString(id[:])[:16],
	}
}

// PublicKeyBase64 returns the platform public key in base64 (standard encoding)
// for publication inside the version_history manifest.
func (k *SigningKey) PublicKeyBase64() string {
	return base64.StdEncoding.EncodeToString(k.Public)
}

// signRevision returns a base64 Ed25519 signature over the canonical
// revision-attestation envelope for the given revision hash. Empty revisionHash
// yields an empty signature (a revision without a hash cannot be attested).
func (k *SigningKey) signRevision(revisionHash string) (string, error) {
	if revisionHash == "" {
		return "", nil
	}
	payload, err := attestationBytes(revisionHash)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(ed25519.Sign(k.Private, payload)), nil
}

// attestationBytes returns the deterministic signed payload for a revision hash.
// Exported via a function (not a method) so verification reconstructs the exact
// bytes the signer used.
func attestationBytes(revisionHash string) ([]byte, error) {
	return json.Marshal(revisionAttestation{Schema: revisionAttestationSchema, RevisionHash: revisionHash})
}

// VerifyRevisionSignature checks a per-revision platform signature. It returns
// true only when pub verifies the signature over the canonical attestation of
// revisionHash. Any malformed input returns false (never an error) so callers
// can treat verification as a pure boolean.
func VerifyRevisionSignature(pubBase64, revisionHash, signatureBase64 string) bool {
	if pubBase64 == "" || revisionHash == "" || signatureBase64 == "" {
		return false
	}
	pub, err := base64.StdEncoding.DecodeString(pubBase64)
	if err != nil || len(pub) != ed25519.PublicKeySize {
		return false
	}
	sig, err := base64.StdEncoding.DecodeString(signatureBase64)
	if err != nil || len(sig) != ed25519.SignatureSize {
		return false
	}
	payload, err := attestationBytes(revisionHash)
	if err != nil {
		return false
	}
	return ed25519.Verify(ed25519.PublicKey(pub), payload, sig)
}
