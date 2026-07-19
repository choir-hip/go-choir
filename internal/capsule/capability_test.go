package capsule

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"
	"time"
)

func TestCapabilitySignAndVerify(t *testing.T) {
	// Generate Ed25519 key pair.
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	cap := &Capability{
		CapabilityID:  "cap-test-001",
		Handle:        "build-a",
		CapsuleID:     "capsule-uuid-001",
		AgentRunID:    "run-001",
		AgentRole:     RoleCoSuper,
		TargetCapsule: "capsule-uuid-001",
		Verbs:         RoleVerbSets[RoleCoSuper],
		ExpiresAt:     time.Now().Add(1 * time.Hour),
	}

	// Sign the capability.
	if err := SignCapability(cap, priv, "key-test-001"); err != nil {
		t.Fatalf("failed to sign capability: %v", err)
	}

	if cap.KeyID != "key-test-001" {
		t.Errorf("expected KeyID 'key-test-001', got '%s'", cap.KeyID)
	}
	if len(cap.Signature) != ed25519.SignatureSize {
		t.Errorf("expected signature size %d, got %d", ed25519.SignatureSize, len(cap.Signature))
	}

	// Verify with correct public key.
	if err := cap.Verify(pub); err != nil {
		t.Fatalf("verification failed with correct key: %v", err)
	}

	// Verify with wrong public key should fail.
	wrongPub, _, _ := ed25519.GenerateKey(rand.Reader)
	if err := cap.Verify(wrongPub); err == nil {
		t.Error("verification should fail with wrong key")
	}

	// Verify expired capability should fail.
	cap.ExpiresAt = time.Now().Add(-1 * time.Hour)
	if err := cap.Verify(pub); err == nil {
		t.Error("verification should fail for expired capability")
	}
}

func TestCapabilityRevocation(t *testing.T) {
	cap := &Capability{
		CapabilityID: "cap-revoke-test",
	}

	revoked := map[string]bool{"cap-revoke-test": true}
	if !cap.IsRevoked(revoked) {
		t.Error("capability should be revoked")
	}

	notRevoked := map[string]bool{"other-cap": true}
	if cap.IsRevoked(notRevoked) {
		t.Error("capability should not be revoked")
	}
}

func TestVerifyCapabilityWithKey(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)

	cap := &Capability{
		CapabilityID: "cap-combined-test",
		AgentRole:    RoleCoSuper,
		Verbs:        RoleVerbSets[RoleCoSuper],
		ExpiresAt:    time.Now().Add(1 * time.Hour),
	}

	SignCapability(cap, priv, "key-001")

	// Should pass with correct key and not revoked.
	revoked := map[string]bool{}
	if err := VerifyCapabilityWithKey(cap, pub, revoked); err != nil {
		t.Fatalf("verification should pass: %v", err)
	}

	// Should fail when revoked.
	revoked["cap-combined-test"] = true
	if err := VerifyCapabilityWithKey(cap, pub, revoked); err == nil {
		t.Error("verification should fail when revoked")
	}
}
