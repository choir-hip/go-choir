package computerevent

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestPrivateArtifactCipherUsesFreshNonceAndAuthenticatesMetadata(t *testing.T) {
	keyring, err := NewFilePrivacyKeyring(filepath.Join(t.TempDir(), "keys"))
	if err != nil {
		t.Fatal(err)
	}
	cipher, err := NewPrivateArtifactCipher(keyring)
	if err != nil {
		t.Fatal(err)
	}
	plaintext := []byte("private model response")
	first, firstMetadata, err := cipher.Encrypt(context.Background(), "computer-test", "event-test", "text/plain", "private", plaintext)
	if err != nil {
		t.Fatal(err)
	}
	second, secondMetadata, err := cipher.Encrypt(context.Background(), "computer-test", "event-test", "text/plain", "private", plaintext)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(first, second) || firstMetadata.Nonce == secondMetadata.Nonce {
		t.Fatal("repeated encryption reused an XChaCha20 nonce")
	}
	decrypted, metadata, err := cipher.Decrypt(context.Background(), first, "computer-test", "event-test")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decrypted, plaintext) || metadata.KeyVersionDigest != firstMetadata.KeyVersionDigest {
		t.Fatalf("decrypted artifact mismatch: plaintext=%q metadata=%+v", decrypted, metadata)
	}
	if _, _, err := cipher.Decrypt(context.Background(), first, "computer-other", "event-test"); err == nil {
		t.Fatal("artifact decrypted under the wrong computer identity")
	}
	tampered := append([]byte(nil), first...)
	tampered[len(tampered)-2] ^= 1
	if _, _, err := cipher.Decrypt(context.Background(), tampered, "computer-test", "event-test"); err == nil {
		t.Fatal("tampered private artifact decrypted")
	}
}

func TestPrivateArtifactCipherRedactsSecretsBeforeImmutableEncryption(t *testing.T) {
	keyring, err := NewFilePrivacyKeyring(filepath.Join(t.TempDir(), "keys"))
	if err != nil {
		t.Fatal(err)
	}
	cipher, err := NewPrivateArtifactCipher(keyring)
	if err != nil {
		t.Fatal(err)
	}
	secret := []byte("Authorization: Bearer abcdefghijklmnopqrstuvwxyz123456")
	envelope, metadata, err := cipher.Encrypt(context.Background(), "computer-test", "event-secret", "text/plain", "private", secret)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(envelope, []byte("abcdefghijklmnopqrstuvwxyz123456")) {
		t.Fatal("secret appeared in immutable encrypted envelope")
	}
	redacted, _, err := cipher.Decrypt(context.Background(), envelope, "computer-test", "event-secret")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(redacted, []byte("abcdefghijklmnopqrstuvwxyz123456")) || !bytes.Contains(redacted, []byte("secret-handle:v1:authorization_bearer:")) {
		t.Fatalf("decrypted payload was not replaced by a typed secret handle: %q", redacted)
	}
	if len(metadata.SecretHandles) != 1 || metadata.SecretHandles[0].Kind != "authorization_bearer" {
		t.Fatalf("secret handles = %+v", metadata.SecretHandles)
	}
}

func TestFilePrivacyKeyringRefusesGroupReadableRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "keys")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := NewFilePrivacyKeyring(root); err == nil {
		t.Fatal("group-readable keyring root was accepted")
	}
}
