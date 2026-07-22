package proxy

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/buildinfo"
)

const executionIdentitySchemaV1 = "choir.execution_identity.v1"

type guestExecutionIdentityEnvelope struct {
	Schema   string `json:"schema"`
	Identity struct {
		Schema              string          `json:"schema"`
		Nonce               string          `json:"nonce"`
		ComputerID          string          `json:"computer_id"`
		RealizationID       string          `json:"realization_id"`
		VMEpoch             string          `json:"vm_epoch"`
		Executable          json.RawMessage `json:"executable"`
		GuestImageManifest  json.RawMessage `json:"guest_image_manifest"`
		KernelConfiguration json.RawMessage `json:"kernel_configuration"`
		Build               buildinfo.Info  `json:"build"`
		IssuedAt            string          `json:"issued_at"`
		ExpiresAt           string          `json:"expires_at"`
	} `json:"identity"`
	Receipt         json.RawMessage `json:"receipt"`
	SignerPublicKey string          `json:"signer_public_key"`
}

type hostServiceIdentity struct {
	PackagePath    string `json:"package_path"`
	EmbeddedCommit string `json:"embedded_commit"`
}

type hostDeploymentIdentity struct {
	CanonicalRef  string                         `json:"canonical_ref"`
	CheckoutHead  string                         `json:"checkout_head"`
	CheckoutClean bool                           `json:"checkout_clean"`
	NixOSClosure  string                         `json:"nixos_closure"`
	Services      map[string]hostServiceIdentity `json:"services"`
}

type deploymentIdentityReceipt struct {
	SchemaVersion int    `json:"schema_version"`
	TargetCommit  string `json:"target_commit"`
	ActivatedAt   string `json:"activated_at"`
	Github        struct {
		RunID      string `json:"run_id"`
		RunAttempt string `json:"run_attempt"`
	} `json:"github"`
	Artifacts    map[string]json.RawMessage `json:"artifacts"`
	HostIdentity hostDeploymentIdentity     `json:"host_identity"`
}

type joinedExecutionIdentity struct {
	Schema            string                         `json:"schema"`
	Joined            bool                           `json:"joined"`
	Guest             guestExecutionIdentityEnvelope `json:"guest"`
	VMCTL             map[string]any                 `json:"vmctl"`
	Route             *computeImmutableIdentity      `json:"route"`
	HostBuild         buildinfo.Info                 `json:"host_build"`
	DeploymentReceipt json.RawMessage                `json:"deployment_receipt"`
}

func deploymentReceiptPath() string {
	if path := strings.TrimSpace(os.Getenv("CHOIR_DEPLOY_RECEIPT_PATH")); path != "" {
		return path
	}
	return "/var/lib/go-choir/deploy-receipt.json"
}

func readDeploymentIdentityReceipt() (json.RawMessage, deploymentIdentityReceipt, error) {
	path, err := filepath.Abs(deploymentReceiptPath())
	if err != nil {
		return nil, deploymentIdentityReceipt{}, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, deploymentIdentityReceipt{}, err
	}
	var receipt deploymentIdentityReceipt
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&receipt); err != nil {
		return nil, deploymentIdentityReceipt{}, err
	}
	proxy := receipt.HostIdentity.Services["proxy"]
	if receipt.SchemaVersion != 1 || len(receipt.TargetCommit) != 40 || receipt.Github.RunID == "" ||
		receipt.HostIdentity.CanonicalRef != "refs/heads/main@"+receipt.TargetCommit ||
		receipt.HostIdentity.CheckoutHead != receipt.TargetCommit || !receipt.HostIdentity.CheckoutClean ||
		!strings.HasPrefix(receipt.HostIdentity.NixOSClosure, "/nix/store/") ||
		!strings.HasPrefix(proxy.PackagePath, "/nix/store/") || len(proxy.EmbeddedCommit) != 40 {
		return nil, deploymentIdentityReceipt{}, fmt.Errorf("deployment receipt is incomplete or conflicting")
	}
	return json.RawMessage(raw), receipt, nil
}

// HandleExecutionIdentity joins guest-core evidence to the independently
// resolved vmctl realization, immutable ComputerVersion route, host build, CI
// deployment receipt, NixOS closure, and service executable inventory.
func (h *Handler) HandleExecutionIdentity(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	authResult, err := h.authenticate(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "authentication required"})
		return
	}
	if !h.authorizeAPIKeyScope(w, r, authResult) {
		return
	}
	if h.vmctlClient == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "execution identity authority unavailable"})
		return
	}
	desktopID := requestDesktopID(r)
	ownership, err := h.vmctlClient.LookupDesktopContext(r.Context(), authResult.UserID, desktopID)
	if err != nil || ownership == nil || ownership.State != "active" || ownership.ComputerID == "" || ownership.VMID == "" || ownership.Epoch <= 0 || ownership.SandboxURL == "" {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "execution identity authority unavailable"})
		return
	}
	routeIdentity, err := h.currentImmutableIdentity(r.Context(), authResult.UserID, desktopID)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "execution identity route join unavailable"})
		return
	}
	upstreamURL, err := joinBasePath(ownership.SandboxURL, r.URL.Path)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "execution identity guest join unavailable"})
		return
	}
	upstreamRequest, err := http.NewRequestWithContext(r.Context(), http.MethodGet, upstreamURL+"?"+r.URL.RawQuery, nil)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "execution identity guest join unavailable"})
		return
	}
	h.setTrustedAuthHeaders(upstreamRequest, authResult)
	response, err := http.DefaultClient.Do(upstreamRequest)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "execution identity guest unavailable"})
		return
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil || response.StatusCode != http.StatusOK {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "execution identity guest refused"})
		return
	}
	var guest guestExecutionIdentityEnvelope
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&guest); err != nil || guest.Schema != executionIdentitySchemaV1 || guest.Identity.Schema != executionIdentitySchemaV1 || guest.Identity.Nonce != strings.TrimSpace(r.URL.Query().Get("nonce")) || len(guest.Receipt) == 0 || guest.SignerPublicKey == "" {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "execution identity guest evidence invalid"})
		return
	}
	expectedRealization := fmt.Sprintf("%s-epoch-%d", ownership.VMID, ownership.Epoch)
	if guest.Identity.ComputerID != ownership.ComputerID || guest.Identity.RealizationID != expectedRealization || guest.Identity.VMEpoch != strconv.FormatInt(ownership.Epoch, 10) {
		writeJSON(w, http.StatusConflict, errorResponse{Error: "execution identity guest and vmctl conflict"})
		return
	}
	receiptRaw, receipt, err := readDeploymentIdentityReceipt()
	hostBuild := buildinfo.Snapshot("proxy")
	hostProxy := receipt.HostIdentity.Services["proxy"]
	if err != nil || hostBuild.Commit != hostProxy.EmbeddedCommit ||
		guest.Identity.Build.Commit != routeIdentity.CodeCommit ||
		guest.Identity.Build.DeployedCommit != guest.Identity.Build.Commit {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "execution identity host, guest, route, and CI join unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, joinedExecutionIdentity{
		Schema: executionIdentitySchemaV1, Joined: true, Guest: guest,
		VMCTL: map[string]any{"computer_id": ownership.ComputerID, "vm_id": ownership.VMID, "epoch": ownership.Epoch, "state": ownership.State},
		Route: routeIdentity, HostBuild: hostBuild, DeploymentReceipt: receiptRaw,
	})
}
