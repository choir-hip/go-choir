package platform

import (
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"testing"
	"time"
)

// An expired bootstrap envelope must be recognisable to the guest, which
// treats it as fatal for that boot instead of restarting into the same
// refusal (docs/problems/offline-maintenance-outlives-the-boot-credential-2026-10-24.md, item 2).
func TestVerifyBootstrapExpiredEnvelopeIsLifetimeError(t *testing.T) {
	issuedAt := time.Date(2026, 10, 24, 4, 0, 1, 0, time.UTC)
	envelope := ComputerCredentialEnvelope{
		Version: 1, ComputerID: "computer-paper", RealizationID: "vm-paper-epoch-9",
		Bearer: "bearer", Nonce: "nonce", RequestCommitment: "commitment", SigningKeyID: "key-1",
		SigningPublicKey: base64.RawStdEncoding.EncodeToString(make([]byte, ed25519.PublicKeySize)),
		IssuedAt:         issuedAt.Format(time.RFC3339Nano),
		ExpiresAt:        issuedAt.Add(10 * time.Minute).Format(time.RFC3339Nano),
	}
	_, err := envelope.VerifyBootstrap("computer-paper", "vm-paper-epoch-9", issuedAt.Add(11*time.Minute+27*time.Second))
	if !errors.Is(err, ErrCredentialEnvelopeLifetime) {
		t.Fatalf("expired envelope = %v, want ErrCredentialEnvelopeLifetime", err)
	}
	if err.Error() != "credential envelope: invalid bootstrap lifetime" {
		t.Fatalf("message changed: %q", err.Error())
	}
	_, err = envelope.VerifyBootstrap("computer-other", "vm-paper-epoch-9", issuedAt.Add(time.Minute))
	if err == nil || errors.Is(err, ErrCredentialEnvelopeLifetime) {
		t.Fatalf("binding mismatch = %v, must not read as a lifetime error", err)
	}
	_, err = envelope.VerifyBootstrap("computer-paper", "vm-paper-epoch-9", issuedAt.Add(time.Minute))
	if errors.Is(err, ErrCredentialEnvelopeLifetime) {
		t.Fatalf("a live envelope read as a lifetime error: %v", err)
	}
}
