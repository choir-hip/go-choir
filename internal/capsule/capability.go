package capsule

import (
	"bytes"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Capability is an Ed25519-signed token minted by HostAuthority.
// The cosuper never sees the raw capsule ID — it gets an opaque handle.
type Capability struct {
	CapabilityID   string    `json:"capability_id"`    // stable unique ID (used in revocation + session binding)
	Handle         string    `json:"handle"`           // opaque handle, e.g. "build-a" (agent-facing)
	CapsuleID      string    `json:"capsule_id"`       // real capsule UUID, or "" for wildcard (researcher)
	AgentRunID     string    `json:"agent_run_id"`     // which agent run this cap is for
	AgentRole      AgentRole `json:"agent_role"`       // determines verb set
	TargetCapsule  string    `json:"target_capsule"`   // capsule ID, or "*" for all (researcher)
	Verbs          VerbSet   `json:"verbs"`            // role-defined verb set
	ExternalAccess []string  `json:"external_access"`  // e.g. ["dolt:write", "message:send"] for researcher
	CommitEpoch    uint64    `json:"commit_epoch"`     // audit metadata only (NOT enforced for exec/read/write)
	ExpiresAt      time.Time `json:"expires_at"`       // capability expiry
	KeyID          string    `json:"key_id"`           // which signing key was used (for rotation)
	Signature      []byte    `json:"signature"`        // Ed25519 signature over all fields
}

// signingPayload returns the canonical bytes that are signed by HostAuthority.
// The signature field itself is excluded from the payload.
func (c *Capability) signingPayload() ([]byte, error) {
	// Create a copy without the signature for signing.
	copy := *c
	copy.Signature = nil
	return json.Marshal(copy)
}

// Verify checks the Ed25519 signature against the provided public key
// and verifies the capability has not expired.
func (c *Capability) Verify(publicKey ed25519.PublicKey) error {
	if len(c.Signature) == 0 {
		return errors.New("capability has no signature")
	}
	if len(publicKey) != ed25519.PublicKeySize {
		return fmt.Errorf("invalid public key size: %d", len(publicKey))
	}

	payload, err := c.signingPayload()
	if err != nil {
		return fmt.Errorf("failed to marshal capability for verification: %w", err)
	}

	if !ed25519.Verify(publicKey, payload, c.Signature) {
		return errors.New("capability signature verification failed")
	}

	if time.Now().After(c.ExpiresAt) {
		return fmt.Errorf("capability expired at %s", c.ExpiresAt)
	}

	return nil
}

// IsRevoked checks if this capability's ID is in the revoked set.
func (c *Capability) IsRevoked(revokedCaps map[string]bool) bool {
	return revokedCaps[c.CapabilityID]
}

// MarshalForTransport serializes a capability for transmission over the wire.
func (c *Capability) MarshalForTransport() ([]byte, error) {
	return json.Marshal(c)
}

// UnmarshalFromTransport deserializes a capability from wire bytes.
func UnmarshalFromTransport(data []byte) (*Capability, error) {
	var cap Capability
	if err := json.Unmarshal(data, &cap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal capability: %w", err)
	}
	return &cap, nil
}

// SignCapability signs a capability with the provided Ed25519 private key.
// This is called by HostAuthority on the host. The Signature field is
// populated in-place.
func SignCapability(cap *Capability, privateKey ed25519.PrivateKey, keyID string) error {
	if len(privateKey) != ed25519.PrivateKeySize {
		return fmt.Errorf("invalid private key size: %d", len(privateKey))
	}

	cap.KeyID = keyID
	cap.Signature = nil // ensure signature is empty before computing payload

	payload, err := cap.signingPayload()
	if err != nil {
		return fmt.Errorf("failed to marshal capability for signing: %w", err)
	}

	cap.Signature = ed25519.Sign(privateKey, payload)
	return nil
}

// VerifyCapabilityWithKey is a convenience function that verifies a
// capability's signature and checks the revoked set in one call.
func VerifyCapabilityWithKey(cap *Capability, publicKey ed25519.PublicKey, revokedCaps map[string]bool) error {
	if err := cap.Verify(publicKey); err != nil {
		return err
	}
	if cap.IsRevoked(revokedCaps) {
		return fmt.Errorf("capability %s has been revoked", cap.CapabilityID)
	}
	return nil
}

// CapabilitiesEqual compares two capabilities by their CapabilityID.
func CapabilitiesEqual(a, b *Capability) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.CapabilityID == b.CapabilityID
}

// signingPayloadBytes is a helper for testing that returns the payload
// without the signature field.
func signingPayloadBytes(c *Capability) []byte {
	payload, _ := c.signingPayload()
	return payload
}

// ensure bytes import is used (for potential future use)
var _ = bytes.Equal
