package computerevent

import (
	"bytes"
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
)

func externalTestCipher(t *testing.T, fill byte) *PrivateArtifactCipher {
	t.Helper()
	cipher, err := newPrivateArtifactCipher("computer-test", base64.RawStdEncoding.EncodeToString(bytes.Repeat([]byte{fill}, 32)))
	if err != nil {
		t.Fatal(err)
	}
	return cipher
}

func TestPrivateArtifactCipherUsesFreshNonceAndAuthenticatesMetadata(t *testing.T) {
	cipher := externalTestCipher(t, 0x11)
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
	cipher := externalTestCipher(t, 0x22)
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

func TestGuestPrivacyKeyringSurvivesTrustedCoreReconstruction(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keyring", "privacy-key")
	first, err := LoadGuestPrivateArtifactCipher(path, "computer-test", true)
	if err != nil {
		t.Fatal(err)
	}
	envelope, metadata, err := first.Encrypt(context.Background(), "computer-test", "event-restart", "text/plain", "private", []byte("restart durable"))
	if err != nil {
		t.Fatal(err)
	}
	reconstructed, err := LoadGuestPrivateArtifactCipher(path, "computer-test", false)
	if err != nil {
		t.Fatal(err)
	}
	plaintext, recoveredMetadata, err := reconstructed.Decrypt(context.Background(), envelope, "computer-test", "event-restart")
	if err != nil {
		t.Fatal(err)
	}
	if string(plaintext) != "restart durable" || recoveredMetadata.KeyVersionDigest != metadata.KeyVersionDigest {
		t.Fatalf("reconstructed private artifact = %q %+v", plaintext, recoveredMetadata)
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o400 {
		t.Fatalf("guest privacy key mode = %v, %v", info, err)
	}
	if _, err := LoadGuestPrivateArtifactCipher(path, "computer-other", false); err == nil {
		t.Fatal("guest privacy key accepted for another computer")
	}
}

func TestGuestPrivacyKeyringRefusesMissingKeyAfterGenesis(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keyring", "privacy-key")
	if _, err := LoadGuestPrivateArtifactCipher(path, "computer-test", false); err == nil {
		t.Fatal("missing post-genesis guest privacy key was recreated")
	}
}

// TestInstallGuestPrivacyKeyRestoresDecryption: a key delivered from escrow
// installs as the same canonical key file the guest would have created, so
// history encrypted by an earlier realization decrypts (SH slice 3, O21).
// An existing key file is never replaced.
func TestInstallGuestPrivacyKeyRestoresDecryption(t *testing.T) {
	original, err := LoadGuestPrivateArtifactCipher(filepath.Join(t.TempDir(), "old", "privacy-key"), "computer-test", true)
	if err != nil {
		t.Fatal(err)
	}
	envelope, _, err := original.Encrypt(context.Background(), "computer-test", "event-old", "text/plain", "private", []byte("before the disk was lost"))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := original.ExportKeyForEscrow(context.Background(), "computer-test")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "new", "privacy-key")
	if err := InstallGuestPrivacyKey(path, "computer-test", raw); err != nil {
		t.Fatal(err)
	}
	restored, err := LoadGuestPrivateArtifactCipher(path, "computer-test", false)
	if err != nil {
		t.Fatalf("installed key does not load: %v", err)
	}
	plaintext, _, err := restored.Decrypt(context.Background(), envelope, "computer-test", "event-old")
	if err != nil || string(plaintext) != "before the disk was lost" {
		t.Fatalf("installed key cannot decrypt earlier history: %q %v", plaintext, err)
	}
	if err := InstallGuestPrivacyKey(path, "computer-test", bytes.Repeat([]byte{9}, 32)); !os.IsExist(err) {
		t.Fatalf("existing key file must not be replaced, got %v", err)
	}
	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 1 {
		t.Fatalf("install left temporary files: %v", entries)
	}
}
