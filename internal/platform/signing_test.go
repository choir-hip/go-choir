package platform

import (
	"crypto/ed25519"
	"path/filepath"
	"testing"
)

func TestSignManifestRoundTrip(t *testing.T) {
	key := loadTestSigningKey(t)

	sig, err := key.signManifest("rev1:abcdef")
	if err != nil {
		t.Fatalf("signManifest: %v", err)
	}
	if sig == "" {
		t.Fatal("expected non-empty signature")
	}

	// The published public key verifies the signature over the manifest hash.
	if !VerifyManifestSignature(key.PublicKeyBase64(), "rev1:abcdef", sig) {
		t.Fatal("platform public key failed to verify its own signature")
	}

	// A different manifest hash must NOT verify against this signature.
	if VerifyManifestSignature(key.PublicKeyBase64(), "rev1:different", sig) {
		t.Fatal("signature must not verify against a tampered manifest hash")
	}
}

func TestVerifyManifestSignatureRejectsTamperedSignature(t *testing.T) {
	key := loadTestSigningKey(t)
	sig, err := key.signManifest("rev1:original")
	if err != nil {
		t.Fatalf("signManifest: %v", err)
	}

	// Flip a byte in the signature: verification must fail.
	tampered := []byte(sig)
	if tampered[0] == 'A' {
		tampered[0] = 'B'
	} else {
		tampered[0] = 'A'
	}
	if VerifyManifestSignature(key.PublicKeyBase64(), "rev1:original", string(tampered)) {
		t.Fatal("tampered signature must not verify")
	}
}

func TestVerifyManifestSignatureMalformed(t *testing.T) {
	key := loadTestSigningKey(t)
	sig, _ := key.signManifest("rev1:x")

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
		if VerifyManifestSignature(c.pub, c.hash, c.sig) {
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

func TestSignManifestEmptyHashYieldsEmpty(t *testing.T) {
	key := loadTestSigningKey(t)
	sig, err := key.signManifest("")
	if err != nil {
		t.Fatalf("signManifest empty: %v", err)
	}
	if sig != "" {
		t.Fatalf("empty manifest hash must yield empty signature, got %q", sig)
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
