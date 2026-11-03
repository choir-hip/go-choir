package selfdevprotocol

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

// Verifier-key pin protocol
// (docs/problems/verifier-key-regenerated-locks-out-checkpoints-2026-11-02.md).
//
// The host pins one verifier-control key per computer (control_key_history)
// and refuses every verifier-class checkpoint signed by another. A guest
// whose data disk was lost generates a new verifier key at the next boot, so
// it is locked out of every later apply. The preflight asks the host, read
// only, whether the key the guest presents is the pinned one; the re-pin is
// the owner's act that replaces the pin with the presented key.

// VerifierKeyDomain is the verifier signer's domain.
const VerifierKeyDomain = "verifier-control"

// CheckpointRefusalVerifierKeyNotPinned is the checkpoint authority's refusal
// of a verifier certificate signed by a key other than the pinned one. The
// guest matches it to hold the operation instead of retrying.
const CheckpointRefusalVerifierKeyNotPinned = "checkpoint authority: verifier key is not the pinned computer key"

// CheckpointRefusalVerifierTrustSubstitution is the authority's refusal of a
// pin substitution inside the checkpoint transaction.
const CheckpointRefusalVerifierTrustSubstitution = "checkpoint authority: verifier trust substitution refused"

// Pin states the preflight answers.
const (
	// VerifierKeyMatch: the presented key is the active pin.
	VerifierKeyMatch = "match"
	// VerifierKeyMismatch: another key is pinned; every verifier checkpoint
	// will be refused until the owner re-pins.
	VerifierKeyMismatch = "mismatch"
	// VerifierKeyUnpinned: no pin, and the host holds the computer's
	// genesis: the next verifier checkpoint pins the presented key on first
	// use.
	VerifierKeyUnpinned = "unpinned"
	// VerifierKeyUnpinnedNoGenesis: no pin and no genesis on the host: a
	// verifier checkpoint is refused until the chain is bootstrapped.
	VerifierKeyUnpinnedNoGenesis = "unpinned_no_genesis"
	// VerifierKeyInvalid: the presented key id does not name the presented
	// public key.
	VerifierKeyInvalid = "invalid"
)

// VerifierKeyPreflightRequest names the key a guest's verifier presents.
type VerifierKeyPreflightRequest struct {
	ComputerID string `json:"computer_id"`
	KeyID      string `json:"key_id"`
	PublicKey  string `json:"public_key"`
}

// VerifierKeyPin is the host's answer: the pin state of a presented key.
type VerifierKeyPin struct {
	ComputerID     string `json:"computer_id"`
	State          string `json:"state"`
	PresentedKeyID string `json:"presented_key_id,omitempty"`
	PinnedKeyID    string `json:"pinned_key_id,omitempty"`
	PinnedAt       string `json:"pinned_at,omitempty"`
}

// Refuses reports whether a verifier checkpoint signed by the presented key
// is refused, deterministically, by the pin.
func (p VerifierKeyPin) Refuses() bool {
	return p.State == VerifierKeyMismatch || p.State == VerifierKeyInvalid || p.State == VerifierKeyUnpinnedNoGenesis
}

// VerifierKeyIDFor is the key id a verifier signer derives from its public
// key (internal/receiptsigner/signing.go).
func VerifierKeyIDFor(public ed25519.PublicKey) string {
	digest := sha256.Sum256(public)
	return VerifierKeyDomain + "-" + hex.EncodeToString(digest[:8])
}

// DecodeVerifierKey decodes a presented verifier public key and checks that
// the key id names it.
func DecodeVerifierKey(keyID, publicKey string) (ed25519.PublicKey, error) {
	raw, err := base64.RawStdEncoding.DecodeString(strings.TrimSpace(publicKey))
	if err != nil || len(raw) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("verifier key: invalid public key")
	}
	if strings.TrimSpace(keyID) != VerifierKeyIDFor(raw) {
		return nil, fmt.Errorf("verifier key: key id does not name the public key")
	}
	return ed25519.PublicKey(raw), nil
}

// VerifierKeyRepinRequest is the owner's re-pin as the proxy sends it to
// corpusd, with an owner-act receipt (owneract.ActVerifierRepin) whose
// subject is VerifierKeyRepinSubject of it.
type VerifierKeyRepinRequest struct {
	ComputerID    string `json:"computer_id"`
	OwnerID       string `json:"owner_id"`
	KeyID         string `json:"key_id"`
	PublicKey     string `json:"public_key"`
	ReplacesKeyID string `json:"replaces_key_id"`
	Reason        string `json:"reason"`
}

// VerifierKeyRepinSubject is the subject the owner-act receipt binds: the
// key pinned, the key it replaces and the owner's reason.
func VerifierKeyRepinSubject(request VerifierKeyRepinRequest) map[string]any {
	return map[string]any{
		"key_id":          strings.TrimSpace(request.KeyID),
		"public_key":      strings.TrimSpace(request.PublicKey),
		"replaces_key_id": strings.TrimSpace(request.ReplacesKeyID),
		"reason":          strings.TrimSpace(request.Reason),
	}
}

// Re-pin outcomes.
const (
	VerifierKeyRepinned      = "repinned"
	VerifierKeyAlreadyPinned = "already_pinned"
)

// VerifierKeyRepinResponse is corpusd's answer. PinReceipt is corpusd's
// signed VerifierKeyPinned receipt (repair "owner re-pin"), which the guest
// records on its tape.
type VerifierKeyRepinResponse struct {
	Status     string                `json:"status"`
	Pin        VerifierKeyPin        `json:"pin"`
	PinReceipt computerevent.Receipt `json:"pin_receipt"`
}

// VerifierKeyPinnedReceiptKind is the kind of corpusd's pin receipt.
const VerifierKeyPinnedReceiptKind = "VerifierKeyPinned"

// VerifierKeyRepinRepair is the repair field of an owner re-pin receipt.
const VerifierKeyRepinRepair = "owner re-pin"
