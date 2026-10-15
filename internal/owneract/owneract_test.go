package owneract

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// The host generates the key with the same two openssl commands as the
// activation script in nix/node-b.nix; the proxy and corpusd must load
// what it writes, through the systemd credential names.
func TestKeysGeneratedLikeActivationLoadAndVerify(t *testing.T) {
	openssl, err := exec.LookPath("openssl")
	if err != nil {
		t.Skip("openssl not on PATH")
	}
	dir := t.TempDir()
	key := filepath.Join(dir, CredentialName)
	pub := filepath.Join(dir, PublicCredentialName)
	if out, err := exec.Command(openssl, "genpkey", "-algorithm", "ed25519", "-out", key).CombinedOutput(); err != nil {
		t.Skipf("openssl cannot generate ed25519: %v %s", err, out)
	}
	if out, err := exec.Command(openssl, "pkey", "-in", key, "-pubout", "-out", pub).CombinedOutput(); err != nil {
		t.Fatalf("openssl pubout: %v %s", err, out)
	}
	t.Setenv(EnvKeyFile, "")
	t.Setenv(EnvPublicKeyFile, "")
	t.Setenv("CREDENTIALS_DIRECTORY", dir)
	signer, path, err := LoadSignerFromEnv()
	if err != nil || signer == nil || path != key {
		t.Fatalf("load signer: %v %q", err, path)
	}
	verifier, path, err := LoadVerifierFromEnv()
	if err != nil || verifier == nil || path != pub {
		t.Fatalf("load verifier: %v %q", err, path)
	}
	if signer.KeyID() != verifier.KeyID() {
		t.Fatalf("key ids differ: %s %s", signer.KeyID(), verifier.KeyID())
	}
	receipt, err := signer.Mint(Act{
		Kind: ActSelfDevModeSet, OwnerUserID: "u", AuthMethod: AuthMethodSession, CredentialRef: "session-jwt-sha256:x",
		ComputerID: "computer-a", Subject: map[string]any{"mode": "off", "expected_generation": uint64(3)},
	}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := Encode(receipt)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := verifier.Verify(decoded, "computer-a", ActSelfDevModeSet, time.Now()); err != nil {
		t.Fatalf("round-tripped receipt does not verify: %v", err)
	}
	if _, err := verifier.Verify(decoded, "computer-a", ActSelfDevModeSet, time.Now().Add(TTL+time.Second)); err == nil {
		t.Fatal("expired receipt verified")
	}
	// No key file: the proxy runs without receipts rather than failing.
	if err := os.Remove(key); err != nil {
		t.Fatal(err)
	}
	if signer, _, err := LoadSignerFromEnv(); signer != nil || err != nil {
		t.Fatalf("absent key: %v %v", signer, err)
	}
}
