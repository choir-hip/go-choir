package agentcore

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/buildinfo"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/receiptsigner"
)

const (
	executionIdentitySchemaV1 = "choir.execution_identity.v1"
	executionIdentityAudience = "choir.news/acceptance/execution-identity"
)

type executionIdentityArtifact struct {
	Role   string `json:"role"`
	SHA256 string `json:"sha256"`
}

type executionIdentityPayload struct {
	Schema              string                    `json:"schema"`
	Nonce               string                    `json:"nonce"`
	Audience            string                    `json:"audience"`
	ComputerID          string                    `json:"computer_id"`
	RealizationID       string                    `json:"realization_id"`
	VMEpoch             string                    `json:"vm_epoch"`
	Executable          executionIdentityArtifact `json:"executable"`
	GuestImageManifest  executionIdentityArtifact `json:"guest_image_manifest"`
	KernelConfiguration executionIdentityArtifact `json:"kernel_configuration"`
	Build               buildinfo.Info            `json:"build"`
	IssuedAt            string                    `json:"issued_at"`
	ExpiresAt           string                    `json:"expires_at"`
}

type executionIdentityEnvelope struct {
	Schema          string                   `json:"schema"`
	Identity        executionIdentityPayload `json:"identity"`
	Receipt         computerevent.Receipt    `json:"receipt"`
	SignerPublicKey string                   `json:"signer_public_key"`
}

func digestIdentityArtifact(role, path string) (executionIdentityArtifact, error) {
	role, path = strings.TrimSpace(role), strings.TrimSpace(path)
	if role == "" || path == "" {
		return executionIdentityArtifact{}, fmt.Errorf("identity artifact role and path are required")
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err == nil {
		path = resolved
	}
	file, err := os.Open(path)
	if err != nil {
		return executionIdentityArtifact{}, err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return executionIdentityArtifact{}, err
	}
	return executionIdentityArtifact{Role: role, SHA256: fmt.Sprintf("sha256:%x", hash.Sum(nil))}, nil
}

// HandleExecutionIdentity returns a nonce-bound, guest-core-signed, least-
// disclosure identity envelope for the exact process serving this request.
func (h *APIHandler) HandleExecutionIdentity(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if _, err := authenticateUser(r); err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	nonce := strings.TrimSpace(r.URL.Query().Get("nonce"))
	if len(nonce) < 16 || len(nonce) > 256 || strings.ContainsAny(nonce, "\r\n\x00") {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "nonce must be 16-256 safe characters"})
		return
	}
	executablePath, err := os.Executable()
	if err != nil {
		writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "execution identity unavailable"})
		return
	}
	executable, executableErr := digestIdentityArtifact("sandbox-service", executablePath)
	guestManifest, guestErr := digestIdentityArtifact("guest-image-manifest", os.Getenv("CHOIR_GUEST_IMAGE_MANIFEST"))
	kernelConfig, kernelErr := digestIdentityArtifact("guest-kernel-configuration", os.Getenv("CHOIR_KERNEL_CONFIG"))
	computerID := strings.TrimSpace(os.Getenv("CHOIR_COMPUTER_ID"))
	realizationID := strings.TrimSpace(os.Getenv("CHOIR_REALIZATION_ID"))
	epoch := strings.TrimSpace(os.Getenv("VM_EPOCH"))
	build := buildinfo.Snapshot("sandbox")
	if executableErr != nil || guestErr != nil || kernelErr != nil || computerID == "" || realizationID == "" || epoch == "" || build.Commit == "" || build.Commit == "local" || build.DeployedCommit == "" || build.DeployedCommit != build.Commit {
		writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "execution identity unavailable", Reason: "incomplete or conflicting executable, realization, epoch, closure, or deploy identity"})
		return
	}
	now := time.Now().UTC()
	identity := executionIdentityPayload{
		Schema: executionIdentitySchemaV1, Nonce: nonce, Audience: executionIdentityAudience, ComputerID: computerID,
		RealizationID: realizationID, VMEpoch: epoch, Executable: executable,
		GuestImageManifest: guestManifest, KernelConfiguration: kernelConfig, Build: build,
		IssuedAt: now.Format(time.RFC3339Nano), ExpiresAt: now.Add(2 * time.Minute).Format(time.RFC3339Nano),
	}
	socket := strings.TrimSpace(os.Getenv("CHOIR_GUEST_SIGNER_SOCKET"))
	if socket == "" {
		socket = "/run/choir-signers/guest-core/signer.sock"
	}
	signer, err := receiptsigner.NewClient(socket, receiptsigner.ModeGuestCore)
	if err != nil {
		writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "execution identity signer unavailable"})
		return
	}
	_, publicKey, err := signer.PublicKey(r.Context())
	if err != nil {
		writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "execution identity signer unavailable"})
		return
	}
	fields := map[string]any{
		"schema": identity.Schema, "nonce": identity.Nonce, "audience": identity.Audience,
		"computer_id": identity.ComputerID,
		"realization_id": identity.RealizationID, "vm_epoch": identity.VMEpoch,
		"executable": identity.Executable, "guest_image_manifest": identity.GuestImageManifest,
		"kernel_configuration": identity.KernelConfiguration, "build": identity.Build,
		"expires_at": identity.ExpiresAt,
	}
	receipt, err := signer.SignReceipt(r.Context(), "ExecutionIdentity", "choir-sandbox", fields, now)
	if err != nil {
		log.Printf("execution identity: sign receipt: %v", err)
		writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "execution identity signature unavailable"})
		return
	}
	writeAPIJSON(w, http.StatusOK, executionIdentityEnvelope{
		Schema: executionIdentitySchemaV1, Identity: identity, Receipt: receipt,
		SignerPublicKey: base64.RawStdEncoding.EncodeToString(publicKey),
	})
}
