package keyescrow

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSealOpenRoundTrip(t *testing.T) {
	priv, pub, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	dek := make([]byte, 32)
	for i := range dek {
		dek[i] = byte(i)
	}
	record, err := SealDEK(pub, "computer-abc", dek)
	if err != nil {
		t.Fatal(err)
	}
	if record.Protector != ProtectorCustodian || record.ComputerID != "computer-abc" {
		t.Fatalf("unexpected record header: %+v", record)
	}
	// Storage round trip through JSON.
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseWrappedKey(data)
	if err != nil {
		t.Fatal(err)
	}
	got, err := OpenDEK(priv, parsed, "computer-abc")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, dek) {
		t.Fatal("recovered DEK mismatch")
	}
}

func TestPasskeyPRFRootWrap(t *testing.T) {
	prfSecret := bytes.Repeat([]byte{0x11}, 32)
	root := bytes.Repeat([]byte{0x22}, 32)
	wrapKey := DerivePRFWrapKey(prfSecret, "owner-a")
	if len(wrapKey) != 32 {
		t.Fatalf("wrap key length = %d, want 32", len(wrapKey))
	}
	blob, err := SealRoot(wrapKey, "owner-a", root)
	if err != nil {
		t.Fatal(err)
	}
	got, err := OpenRoot(wrapKey, "owner-a", blob)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, root) {
		t.Fatal("recovered ROOT mismatch")
	}
	if _, err := OpenRoot(wrapKey, "owner-b", blob); err == nil {
		t.Fatal("expected wrong owner rejection")
	}
	if _, err := OpenRoot(DerivePRFWrapKey(bytes.Repeat([]byte{0x33}, 32), "owner-a"), "owner-a", blob); err == nil {
		t.Fatal("expected wrong key rejection")
	}
	blob[len(blob)-1] ^= 1
	if _, err := OpenRoot(wrapKey, "owner-a", blob); err == nil {
		t.Fatal("expected tamper rejection")
	}
}

func TestOpenRejectsWrongComputerBinding(t *testing.T) {
	priv, pub, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	dek := make([]byte, 32)
	record, err := SealDEK(pub, "computer-a", dek)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := OpenDEK(priv, record, "computer-b"); err == nil {
		t.Fatal("expected binding mismatch rejection")
	}
}

func TestOpenRejectsWrongKeyAndTampering(t *testing.T) {
	priv, pub, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	otherPriv, _, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	dek := make([]byte, 32)
	record, err := SealDEK(pub, "computer-a", dek)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := OpenDEK(otherPriv, record, "computer-a"); err == nil {
		t.Fatal("expected wrong-key rejection")
	}
	record.Ciphertext = record.Ciphertext[:len(record.Ciphertext)-4] + "AAAA"
	if _, err := OpenDEK(priv, record, "computer-a"); err == nil {
		t.Fatal("expected tamper rejection")
	}
}

func TestSealRejectsBadDEKSize(t *testing.T) {
	_, pub, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := SealDEK(pub, "computer-a", make([]byte, 16)); err == nil {
		t.Fatal("expected short DEK rejection")
	}
}

func TestLoadOrGeneratePrivateKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "escrow", "key")
	first, err := LoadOrGeneratePrivateKey(path)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("private key mode = %v, want 0600", info.Mode().Perm())
	}
	second, err := LoadOrGeneratePrivateKey(path)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatal("reload produced a different key")
	}
	if _, err := LoadOrGeneratePrivateKey("relative/key"); err == nil {
		t.Fatal("expected relative path rejection")
	}
}
