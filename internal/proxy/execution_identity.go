package proxy

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/buildinfo"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

const (
	executionIdentitySchemaV1 = "choir.execution_identity.v1"
	executionIdentityAudience = "choir.news/acceptance/execution-identity"
)

type guestExecutionIdentityEnvelope struct {
	Schema   string `json:"schema"`
	Identity struct {
		Schema              string          `json:"schema"`
		Nonce               string          `json:"nonce"`
		Audience            string          `json:"audience"`
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
	Receipt         computerevent.Receipt `json:"receipt"`
	SignerPublicKey string                `json:"signer_public_key"`
}

type hostServiceIdentity struct {
	Role           string `json:"role,omitempty"`
	PackagePath    string `json:"package_path,omitempty"`
	PackageDigest  string `json:"package_digest"`
	EmbeddedCommit string `json:"embedded_commit"`
}

type hostDeploymentIdentity struct {
	CanonicalRef       string                         `json:"canonical_ref"`
	CheckoutHead       string                         `json:"checkout_head,omitempty"`
	CheckoutClean      bool                           `json:"checkout_clean"`
	NixOSClosure       string                         `json:"nixos_closure,omitempty"`
	NixOSClosureDigest string                         `json:"nixos_closure_digest"`
	Services           map[string]hostServiceIdentity `json:"services"`
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
	Schema              string                               `json:"schema"`
	Joined              bool                                 `json:"joined"`
	RouteDigest         string                               `json:"route_digest"`
	Guest               guestExecutionIdentityEnvelope       `json:"guest"`
	VMCTL               map[string]any                       `json:"vmctl"`
	HostBuild           buildinfo.Info                       `json:"host_build"`
	DeploymentReceipt   json.RawMessage                      `json:"deployment_receipt"`
	PlatformAttestation executionIdentityPlatformAttestation `json:"platform_attestation"`
}

type executionIdentityPlatformAttestation struct {
	Receipt         computerevent.Receipt `json:"receipt"`
	SignerPublicKey string                `json:"signer_public_key"`
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
	if receipt.SchemaVersion != 1 || !executionIdentityFullCommit(receipt.TargetCommit) || receipt.Github.RunID == "" ||
		receipt.HostIdentity.CanonicalRef != "refs/heads/main@"+receipt.TargetCommit ||
		receipt.HostIdentity.CheckoutHead != receipt.TargetCommit || !receipt.HostIdentity.CheckoutClean ||
		!strings.HasPrefix(receipt.HostIdentity.NixOSClosure, "/nix/store/") ||
		!strings.HasPrefix(receipt.HostIdentity.NixOSClosureDigest, "sha256:") ||
		!strings.HasPrefix(proxy.PackagePath, "/nix/store/") || !strings.HasPrefix(proxy.PackageDigest, "sha256:") ||
		!executionIdentityFullCommit(proxy.EmbeddedCommit) {
		return nil, deploymentIdentityReceipt{}, fmt.Errorf("deployment receipt is incomplete or conflicting")
	}
	publicReceipt, err := publicDeploymentIdentityReceipt(receipt)
	if err != nil {
		return nil, deploymentIdentityReceipt{}, err
	}
	return publicReceipt, receipt, nil
}

func publicDeploymentIdentityReceipt(receipt deploymentIdentityReceipt) (json.RawMessage, error) {
	receipt.HostIdentity.CheckoutHead = ""
	receipt.HostIdentity.NixOSClosure = ""
	for role, service := range receipt.HostIdentity.Services {
		service.Role = role
		service.PackagePath = ""
		receipt.HostIdentity.Services[role] = service
	}
	raw, err := json.Marshal(receipt)
	return json.RawMessage(raw), err
}

type executionIdentityKeyResolver struct {
	keyID  string
	domain string
	key    ed25519.PublicKey
}

func (r executionIdentityKeyResolver) ResolveReceiptKey(domain, _ string, keyID string, _ uint64, _ time.Time) (ed25519.PublicKey, error) {
	expectedDomain := r.domain
	if expectedDomain == "" {
		expectedDomain = "guest-core"
	}
	if domain != expectedDomain || keyID != r.keyID || len(r.key) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("untrusted execution identity signer")
	}
	return r.key, nil
}

func verifyGuestExecutionIdentity(guest guestExecutionIdentityEnvelope) error {
	publicKey, err := base64.RawStdEncoding.DecodeString(guest.SignerPublicKey)
	if err != nil || len(publicKey) != ed25519.PublicKeySize || guest.Receipt.ReceiptKind != "ExecutionIdentity" ||
		guest.Receipt.Issuer != "choir-sandbox" || len(guest.Receipt.RequiredSigners) != 1 ||
		guest.Receipt.RequiredSigners[0].SignerDomain != "guest-core" {
		return fmt.Errorf("invalid guest execution identity signer")
	}
	expectedFields := map[string]any{
		"schema": guest.Identity.Schema, "nonce": guest.Identity.Nonce,
		"audience":    guest.Identity.Audience,
		"computer_id": guest.Identity.ComputerID, "realization_id": guest.Identity.RealizationID,
		"vm_epoch": guest.Identity.VMEpoch, "executable": guest.Identity.Executable,
		"guest_image_manifest": guest.Identity.GuestImageManifest,
		"kernel_configuration": guest.Identity.KernelConfiguration, "build": guest.Identity.Build,
		"expires_at": guest.Identity.ExpiresAt,
	}
	got, gotErr := computerevent.CanonicalJSON(guest.Receipt.KindFields)
	want, wantErr := computerevent.CanonicalJSON(expectedFields)
	if gotErr != nil || wantErr != nil || !bytes.Equal(got, want) {
		return fmt.Errorf("guest execution identity receipt fields do not match")
	}
	issuedAt, issuedErr := time.Parse(time.RFC3339Nano, guest.Identity.IssuedAt)
	expiresAt, expiresErr := time.Parse(time.RFC3339Nano, guest.Identity.ExpiresAt)
	now := time.Now().UTC()
	if issuedErr != nil || expiresErr != nil || guest.Receipt.IssuedAt != guest.Identity.IssuedAt ||
		guest.Identity.Audience != executionIdentityAudience ||
		issuedAt.After(now.Add(5*time.Second)) || expiresAt.After(issuedAt.Add(2*time.Minute)) || !expiresAt.After(now) {
		return fmt.Errorf("guest execution identity time or audience binding invalid")
	}
	return guest.Receipt.Verify(executionIdentityKeyResolver{
		keyID: guest.Receipt.RequiredSigners[0].KeyID,
		key:   ed25519.PublicKey(publicKey),
	})
}

func canonicalIdentityDigest(value any) (string, error) {
	canonical, err := computerevent.CanonicalJSON(value)
	if err != nil {
		return "", err
	}
	return "sha256:" + computerevent.DigestBytes(canonical), nil
}

func verifyPlatformExecutionIdentity(attestation executionIdentityPlatformAttestation, expectedFields map[string]any, expectedSignerDigest string) error {
	publicKey, err := base64.RawStdEncoding.DecodeString(attestation.SignerPublicKey)
	if err != nil || len(publicKey) != ed25519.PublicKeySize ||
		attestation.Receipt.ReceiptKind != "ExecutionIdentityJoin" || attestation.Receipt.Issuer != "corpusd" ||
		len(attestation.Receipt.RequiredSigners) != 1 || attestation.Receipt.RequiredSigners[0].SignerDomain != "platform-control" {
		return fmt.Errorf("invalid platform execution identity signer")
	}
	if !strings.EqualFold(strings.TrimSpace(expectedSignerDigest), "sha256:"+computerevent.DigestBytes(publicKey)) {
		return fmt.Errorf("platform execution identity signer is not pinned")
	}
	got, gotErr := computerevent.CanonicalJSON(attestation.Receipt.KindFields)
	want, wantErr := computerevent.CanonicalJSON(expectedFields)
	if gotErr != nil || wantErr != nil || !bytes.Equal(got, want) {
		return fmt.Errorf("platform execution identity receipt fields do not match")
	}
	ref := attestation.Receipt.RequiredSigners[0]
	expectedKeyID := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(expectedSignerDigest)), "sha256:")
	if len(expectedKeyID) < 16 || !strings.EqualFold(ref.KeyID, expectedKeyID[:16]) {
		return fmt.Errorf("platform execution identity signer key id is not pinned")
	}
	return attestation.Receipt.Verify(executionIdentityKeyResolver{
		keyID: ref.KeyID, domain: "platform-control", key: ed25519.PublicKey(publicKey),
	})
}

func (h *Handler) attestExecutionIdentity(
	r *http.Request,
	guest guestExecutionIdentityEnvelope,
	vmctlIdentity map[string]any,
	route *computeImmutableIdentity,
	hostBuild buildinfo.Info,
	deploymentReceipt json.RawMessage,
	deployedCommit string,
	expectedPlatformSignerDigest string,
) (executionIdentityPlatformAttestation, error) {
	guestDigest, err := canonicalIdentityDigest(guest.Receipt)
	if err != nil {
		return executionIdentityPlatformAttestation{}, err
	}
	routeDigest, err := canonicalIdentityDigest(route)
	if err != nil {
		return executionIdentityPlatformAttestation{}, err
	}
	hostBuildDigest, err := canonicalIdentityDigest(hostBuild)
	if err != nil {
		return executionIdentityPlatformAttestation{}, err
	}
	deploymentDigest, err := canonicalIdentityDigest(deploymentReceipt)
	if err != nil {
		return executionIdentityPlatformAttestation{}, err
	}
	guestSignerKey, err := base64.RawStdEncoding.DecodeString(guest.SignerPublicKey)
	if err != nil || len(guestSignerKey) != ed25519.PublicKeySize {
		return executionIdentityPlatformAttestation{}, fmt.Errorf("guest signer key unavailable")
	}
	requestBody := map[string]any{
		"schema": executionIdentitySchemaV1, "nonce": guest.Identity.Nonce,
		"deployed_commit": deployedCommit,
		"computer_id":     guest.Identity.ComputerID, "realization_id": guest.Identity.RealizationID,
		"vm_epoch": guest.Identity.VMEpoch, "guest_receipt_digest": guestDigest,
		"audience": executionIdentityAudience,
		"vmctl":    vmctlIdentity, "route_digest": routeDigest, "host_build_digest": hostBuildDigest,
		"guest_signer_key_digest":   "sha256:" + computerevent.DigestBytes(guestSignerKey),
		"deployment_receipt_digest": deploymentDigest,
	}
	raw, err := json.Marshal(requestBody)
	if err != nil {
		return executionIdentityPlatformAttestation{}, err
	}
	endpoint, err := joinBasePath(h.cfg.CorpusdURL, "/internal/platform/execution-identity/attest")
	if err != nil {
		return executionIdentityPlatformAttestation{}, err
	}
	request, err := http.NewRequestWithContext(r.Context(), http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return executionIdentityPlatformAttestation{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Internal-Caller", "true")
	response, err := h.corpusd.Do(request)
	if err != nil {
		return executionIdentityPlatformAttestation{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return executionIdentityPlatformAttestation{}, fmt.Errorf("platform signer status %d", response.StatusCode)
	}
	var attestation executionIdentityPlatformAttestation
	decoder := json.NewDecoder(io.LimitReader(response.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&attestation); err != nil {
		return executionIdentityPlatformAttestation{}, err
	}
	if err := verifyPlatformExecutionIdentity(attestation, requestBody, expectedPlatformSignerDigest); err != nil {
		return executionIdentityPlatformAttestation{}, err
	}
	return attestation, nil
}

// executionIdentityFullCommit validates a Git source identity without
// conflating independently versioned platform and ComputerVersion commits.
func executionIdentityFullCommit(commit string) bool {
	if len(commit) != 40 {
		return false
	}
	for _, r := range commit {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}

func executionIdentityCommitsJoin(receipt deploymentIdentityReceipt, host buildinfo.Info, routeCommit string, guest buildinfo.Info) bool {
	target := strings.TrimSpace(receipt.TargetCommit)
	routeCommit = strings.TrimSpace(routeCommit)
	proxy := receipt.HostIdentity.Services["proxy"]
	if !executionIdentityFullCommit(target) ||
		!executionIdentityFullCommit(routeCommit) ||
		!executionIdentityFullCommit(host.Commit) ||
		!executionIdentityFullCommit(proxy.EmbeddedCommit) ||
		host.Service != "proxy" ||
		host.Commit != proxy.EmbeddedCommit ||
		!executionIdentityFullCommit(guest.Commit) ||
		guest.DeployedCommit != guest.Commit {
		return false
	}
	rawArtifact, selected := receipt.Artifacts["proxy"]
	if !selected {
		return strings.TrimSpace(host.DeployedCommit) == ""
	}
	var artifact struct {
		Commit string `json:"commit"`
		Status string `json:"status"`
	}
	if json.Unmarshal(rawArtifact, &artifact) != nil {
		return false
	}
	return artifact.Commit == target && artifact.Status == "active" &&
		host.Commit == target && host.DeployedCommit == target
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
	if authResult.AuthMethod != "api_key" || (!hasAPIKeyScope(authResult.Scopes, "admin") && !hasAPIKeyScope(authResult.Scopes, "acceptance:read")) {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "acceptance API key scope required"})
		return
	}
	expectedPlatformSignerDigest := h.platformSignerDigest
	if !strings.HasPrefix(expectedPlatformSignerDigest, "sha256:") {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "execution identity trust configuration unavailable"})
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
	upstreamRequest.Header.Set("X-Authenticated-User", authResult.UserID)
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
	if err := decoder.Decode(&guest); err != nil || guest.Schema != executionIdentitySchemaV1 ||
		guest.Identity.Schema != executionIdentitySchemaV1 || guest.Identity.Audience != executionIdentityAudience ||
		guest.Identity.Nonce != strings.TrimSpace(r.URL.Query().Get("nonce")) ||
		guest.Receipt.ReceiptKind == "" || guest.SignerPublicKey == "" {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "execution identity guest evidence invalid"})
		return
	}
	if err := verifyGuestExecutionIdentity(guest); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "execution identity guest signature invalid"})
		return
	}
	expectedRealization := fmt.Sprintf("%s-epoch-%d", ownership.VMID, ownership.Epoch)
	if guest.Identity.ComputerID != ownership.ComputerID || guest.Identity.RealizationID != expectedRealization || guest.Identity.VMEpoch != strconv.FormatInt(ownership.Epoch, 10) {
		writeJSON(w, http.StatusConflict, errorResponse{Error: "execution identity guest and vmctl conflict"})
		return
	}
	receiptRaw, receipt, err := readDeploymentIdentityReceipt()
	hostBuild := buildinfo.Snapshot("proxy")
	if err != nil || !executionIdentityCommitsJoin(receipt, hostBuild, routeIdentity.CodeCommit, guest.Identity.Build) {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "execution identity host, guest, route, and CI join unavailable"})
		return
	}
	vmctlIdentity := map[string]any{"computer_id": ownership.ComputerID, "vm_id": ownership.VMID, "epoch": ownership.Epoch, "state": ownership.State}
	platformAttestation, err := h.attestExecutionIdentity(r, guest, vmctlIdentity, routeIdentity, hostBuild, receiptRaw, receipt.TargetCommit, expectedPlatformSignerDigest)
	if err != nil {
		log.Printf("proxy execution identity: platform attestation: %v", err)
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "execution identity platform attestation unavailable"})
		return
	}
	routeDigest, err := canonicalIdentityDigest(routeIdentity)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "execution identity route digest unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, joinedExecutionIdentity{
		Schema: executionIdentitySchemaV1, Joined: true, Guest: guest,
		VMCTL: vmctlIdentity, RouteDigest: routeDigest, HostBuild: hostBuild, DeploymentReceipt: receiptRaw,
		PlatformAttestation: platformAttestation,
	})
}
