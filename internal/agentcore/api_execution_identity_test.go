package agentcore

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/buildinfo"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/receiptsigner"
)

type executionIdentityTestResolver struct {
	ref computerevent.SignerRef
	key ed25519.PublicKey
}

func (r executionIdentityTestResolver) ResolveReceiptKey(domain, _ string, keyID string, _ uint64, _ time.Time) (ed25519.PublicKey, error) {
	if domain != r.ref.SignerDomain || keyID != r.ref.KeyID {
		return nil, os.ErrPermission
	}
	return r.key, nil
}

func TestExecutionIdentityReturnsNonceBoundGuestSignature(t *testing.T) {
	_, handler := testAPISetup(t)
	dir, err := os.MkdirTemp("/tmp", "choir-identity-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	key := computerevent.SigningKey{SignerRef: computerevent.SignerRef{SignerDomain: receiptsigner.ModeGuestCore, KeyID: "guest-test"}, PrivateKey: privateKey}
	signer, err := receiptsigner.NewHandler(receiptsigner.ModeGuestCore, "computer-test", filepath.Join(dir, "receipts"), key)
	if err != nil {
		t.Fatal(err)
	}
	socket := filepath.Join(dir, "signer.sock")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: signer}
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { _ = server.Shutdown(context.Background()) })

	manifest := filepath.Join(dir, "guest-manifest")
	kernel := filepath.Join(dir, "kernel-config")
	if err := os.WriteFile(manifest, []byte("guest-closure"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(kernel, []byte("kernel-closure"), 0o600); err != nil {
		t.Fatal(err)
	}
	commit := "0123456789abcdef0123456789abcdef01234567"
	deployReceipt := filepath.Join(dir, "deploy-receipt.json")
	receiptBody := map[string]any{
		"schema_version": 1, "target_commit": commit, "activated_at": time.Now().UTC().Format(time.RFC3339),
		"artifacts": map[string]any{"sandbox": map[string]any{"commit": commit, "status": "active"}},
	}
	raw, _ := json.Marshal(receiptBody)
	if err := os.WriteFile(deployReceipt, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CHOIR_GUEST_SIGNER_SOCKET", socket)
	t.Setenv("CHOIR_GUEST_IMAGE_MANIFEST", manifest)
	t.Setenv("CHOIR_KERNEL_CONFIG", kernel)
	t.Setenv("CHOIR_COMPUTER_ID", "computer-test")
	t.Setenv("CHOIR_REALIZATION_ID", "realization-test")
	t.Setenv("VM_EPOCH", "epoch-test")
	t.Setenv("CHOIR_DEPLOY_RECEIPT_PATH", deployReceipt)
	previousCommit, previousVersion, previousBuiltAt := buildinfo.Commit, buildinfo.Version, buildinfo.BuiltAt
	buildinfo.Commit, buildinfo.Version, buildinfo.BuiltAt = commit, "test", time.Now().UTC().Format(time.RFC3339)
	t.Cleanup(func() {
		buildinfo.Commit, buildinfo.Version, buildinfo.BuiltAt = previousCommit, previousVersion, previousBuiltAt
	})

	nonce := "nonce-bound-client-challenge"
	response := runtimeHandlerRequest(t, handler.HandleExecutionIdentity, http.MethodGet, "/api/acceptance/execution-identity?nonce="+nonce, "", "owner-identity")
	if response.Code != http.StatusOK {
		t.Fatalf("identity status = %d; body=%s", response.Code, response.Body.String())
	}
	var envelope executionIdentityEnvelope
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	publicKey := privateKey.Public().(ed25519.PublicKey)
	if envelope.Schema != executionIdentitySchemaV1 || envelope.Identity.Nonce != nonce || envelope.Identity.Build.Commit != commit || envelope.Identity.Build.DeployedCommit != commit {
		t.Fatalf("identity binding mismatch: %+v", envelope.Identity)
	}
	if err := envelope.Receipt.Verify(executionIdentityTestResolver{ref: key.SignerRef, key: publicKey}); err != nil {
		t.Fatalf("identity signature invalid: %v", err)
	}
}

func TestExecutionIdentityFailsClosedWithoutExactDeploymentInputs(t *testing.T) {
	_, handler := testAPISetup(t)
	for _, name := range []string{"CHOIR_COMPUTER_ID", "CHOIR_GUEST_IMAGE_MANIFEST", "CHOIR_KERNEL_CONFIG", "CHOIR_REALIZATION_ID", "VM_EPOCH", "CHOIR_GUEST_SIGNER_SOCKET"} {
		t.Setenv(name, "")
	}
	response := runtimeHandlerRequest(t, handler.HandleExecutionIdentity, http.MethodGet, "/api/acceptance/execution-identity?nonce=nonce-bound-client-challenge", "", "owner-identity")
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("identity status = %d, want 503; body=%s", response.Code, response.Body.String())
	}
}

func TestExecutionIdentityRequiresAuthenticationAndSafeNonce(t *testing.T) {
	_, handler := testAPISetup(t)
	unauthenticated := runtimeHandlerRequest(t, handler.HandleExecutionIdentity, http.MethodGet, "/api/acceptance/execution-identity?nonce=nonce-bound-client-challenge", "", "")
	if unauthenticated.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status = %d, want 401", unauthenticated.Code)
	}
	unsafe := runtimeHandlerRequest(t, handler.HandleExecutionIdentity, http.MethodGet, "/api/acceptance/execution-identity?nonce=short", "", "owner-identity")
	if unsafe.Code != http.StatusBadRequest {
		t.Fatalf("unsafe nonce status = %d, want 400", unsafe.Code)
	}
}
