package platform

import (
	"crypto/ed25519"
	"path/filepath"
	"testing"
)

func TestSignRevisionRoundTrip(t *testing.T) {
	key := loadTestSigningKey(t)

	sig, err := key.signRevision("rev1:abcdef")
	if err != nil {
		t.Fatalf("signRevision: %v", err)
	}
	if sig == "" {
		t.Fatal("expected non-empty signature")
	}

	// The published public key verifies the signature over the revision hash.
	if !VerifyRevisionSignature(key.PublicKeyBase64(), "rev1:abcdef", sig) {
		t.Fatal("platform public key failed to verify its own signature")
	}

	// A different revision hash must NOT verify against this signature.
	if VerifyRevisionSignature(key.PublicKeyBase64(), "rev1:different", sig) {
		t.Fatal("signature must not verify against a tampered revision hash")
	}
}

func TestVerifyRevisionSignatureRejectsTamperedSignature(t *testing.T) {
	key := loadTestSigningKey(t)
	sig, err := key.signRevision("rev1:original")
	if err != nil {
		t.Fatalf("signRevision: %v", err)
	}

	// Flip a byte in the signature: verification must fail.
	tampered := []byte(sig)
	if tampered[0] == 'A' {
		tampered[0] = 'B'
	} else {
		tampered[0] = 'A'
	}
	if VerifyRevisionSignature(key.PublicKeyBase64(), "rev1:original", string(tampered)) {
		t.Fatal("tampered signature must not verify")
	}
}

func TestVerifyRevisionSignatureMalformed(t *testing.T) {
	key := loadTestSigningKey(t)
	sig, _ := key.signRevision("rev1:x")

	cases := []struct {
		name, pub, hash, sig string
	}{
		{"empty sig", key.PublicKeyBase64(), "rev1:x", ""},
		{"empty hash", key.PublicKeyBase64(), "", sig},
		{"empty pub", "", "rev1:x", sig},
		{"bad pub b64", "not-base64!!", "rev1:x", sig},
		{"bad sig b64", key.PublicKeyBase64(), "rev1:x", "not-base64!!"},
		{"wrong-size pub", "YWJjZA==", "rev1:x", sig}, // decodes but wrong length
	}
	for _, c := range cases {
		if VerifyRevisionSignature(c.pub, c.hash, c.sig) {
			t.Errorf("%s: expected verification to fail", c.name)
		}
	}
}

func TestLoadOrCreateSigningKeyPersistsAcrossLoads(t *testing.T) {
	path := filepath.Join(t.TempDir(), "signing-key")

	first, err := LoadOrCreateSigningKey(path)
	if err != nil {
		t.Fatalf("first load: %v", err)
	}
	// A second load must return the SAME key (persisted), not a fresh one.
	second, err := LoadOrCreateSigningKey(path)
	if err != nil {
		t.Fatalf("second load: %v", err)
	}
	if first.KeyID != second.KeyID {
		t.Fatalf("key id changed across loads: %q vs %q", first.KeyID, second.KeyID)
	}
	if !equalPub(first.Public, second.Public) {
		t.Fatal("public key changed across loads")
	}
}

func TestSignRevisionEmptyHashYieldsEmpty(t *testing.T) {
	key := loadTestSigningKey(t)
	sig, err := key.signRevision("")
	if err != nil {
		t.Fatalf("signRevision empty: %v", err)
	}
	if sig != "" {
		t.Fatalf("empty revision hash must yield empty signature, got %q", sig)
	}
}

// loadTestSigningKey builds an in-memory signing key for tests that do not need
// persistence.
func loadTestSigningKey(t *testing.T) *SigningKey {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	return newSigningKey(priv)
}

func equalPub(a, b ed25519.PublicKey) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
