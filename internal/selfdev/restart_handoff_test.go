package selfdev

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/platform"
)

func TestRestartHandoffRestoresExactTransientCapability(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	expiresAt := time.Now().UTC().Add(5 * time.Minute).Truncate(time.Microsecond)
	token, err := platform.MintComputerCapability(platform.ComputerCapability{
		Version: 1, ComputerID: "computer-stable", Scopes: []string{"event:append"},
		ExpiresAt: expiresAt.Format(time.RFC3339Nano), RevocationEpoch: 3, Nonce: "restart-test-nonce",
	}, privateKey)
	if err != nil {
		t.Fatal(err)
	}
	credentials := &GuestCredentials{
		baseURL: "https://platform.test", computerID: "computer-stable", realizationID: "realization-stable", token: token,
		expiresAt: expiresAt, keyID: "platform-test", publicKey: publicKey,
	}
	path := filepath.Join(t.TempDir(), "restart-capability")
	if err := credentials.WriteRestartHandoff(context.Background(), path); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o400 {
		t.Fatalf("handoff mode = %v, %v", info, err)
	}
	if _, err := RestoreGuestCredentials(path, "https://platform.test", "computer-stable", "realization-other"); err == nil {
		t.Fatal("cross-realization handoff was accepted")
	}
	restored, err := RestoreGuestCredentials(path, "https://platform.test", "computer-stable", "realization-stable")
	if err != nil {
		t.Fatal(err)
	}
	if restored.token != token || restored.computerID != credentials.computerID || restored.realizationID != credentials.realizationID ||
		!restored.expiresAt.Equal(expiresAt) {
		t.Fatalf("restored handoff changed capability binding: %+v", restored)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("consumed handoff remains readable: %v", err)
	}
	if _, err := RestoreGuestCredentials(path, "https://platform.test", "computer-stable", "realization-stable"); err == nil {
		t.Fatal("consumed handoff replay was accepted")
	}
}
