package selfdev

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
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

func TestRecoveryHandoffSurvivesCrashUntilRevocationEventCompletes(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	expiresAt := time.Now().UTC().Add(5 * time.Minute).Truncate(time.Microsecond)
	mint := func(epoch uint64, nonce string) string {
		t.Helper()
		token, err := platform.MintComputerCapability(platform.ComputerCapability{
			Version: 1, ComputerID: "computer-stable", Scopes: []string{"event:read", "event:append"},
			ExpiresAt: expiresAt.Format(time.RFC3339Nano), RevocationEpoch: epoch, Nonce: nonce,
		}, privateKey)
		if err != nil {
			t.Fatal(err)
		}
		return token
	}
	current := mint(3, "current-token")
	next := mint(4, "post-revocation-token")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Header.Get("Authorization") {
		case "Bearer " + current:
			w.WriteHeader(http.StatusForbidden)
		case "Bearer " + next:
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusUnauthorized)
		}
	}))
	defer server.Close()

	credentials := &GuestCredentials{
		baseURL: server.URL, computerID: "computer-stable", realizationID: "realization-stable",
		http: server.Client(), token: current, postRevocationToken: next, expiresAt: expiresAt,
		keyID: "platform-test", publicKey: publicKey,
		pendingLifecycle: []computerevent.Receipt{{ReceiptID: "revocation-receipt"}},
	}
	recoveryPath := filepath.Join(t.TempDir(), "recovery-capability")
	restartPath := filepath.Join(t.TempDir(), "restart-capability")
	if err := credentials.ConfigureRecoveryHandoff(context.Background(), recoveryPath); err != nil {
		t.Fatal(err)
	}
	if err := credentials.WriteRestartHandoff(context.Background(), restartPath); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(recoveryPath); err != nil {
		t.Fatalf("recovery handoff missing after restart snapshot: %v", err)
	}
	restored, err := RestoreGuestCredentials(restartPath, server.URL, "computer-stable", "realization-stable")
	if err != nil {
		t.Fatal(err)
	}
	if restored.postRevocationToken != next || len(restored.pendingLifecycle) != 1 {
		t.Fatal("restart lost revocation transition state")
	}
	if err := restored.ConfigureRecoveryHandoff(context.Background(), recoveryPath); err != nil {
		t.Fatal(err)
	}
	recovered, err := restored.RecoverPostRevocationCapability(context.Background())
	if err != nil || !recovered {
		t.Fatalf("post-revocation recovery = %v, %v", recovered, err)
	}
	if restored.token != next || restored.postRevocationToken != next || len(restored.PendingLifecycleReceipts()) != 1 {
		t.Fatal("recovery did not preserve the pending revocation event")
	}
	if err := restored.CompletePostRevocation("revocation-receipt"); err != nil {
		t.Fatal(err)
	}
	if restored.token != next || restored.HasPostRevocationCapability() || len(restored.PendingLifecycleReceipts()) != 0 {
		t.Fatal("completed revocation transition retained stale handoff state")
	}
	final, err := RestoreGuestCredentials(recoveryPath, server.URL, "computer-stable", "realization-stable")
	if err != nil {
		t.Fatal(err)
	}
	if final.token != next || final.HasPostRevocationCapability() || len(final.PendingLifecycleReceipts()) != 0 {
		t.Fatal("durable recovery handoff did not record completed revocation state")
	}
}
