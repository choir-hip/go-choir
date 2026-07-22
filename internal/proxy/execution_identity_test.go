package proxy

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/buildinfo"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/computerversion"
	"github.com/yusefmosiah/go-choir/internal/routeledger"
	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

func TestExecutionIdentityJoinsGuestVMCTLRouteAndDeployReceipt(t *testing.T) {
	const commit = "1234567890abcdef1234567890abcdef12345678"
	var ownerID string
	const computerID = "computer-identity-join"
	const vmID = "vm-identity-join"
	const epoch = int64(42)
	const nonce = "nonce-bound-identity-join"
	handler, privateKey, sandbox, authStore := testProxyEnvWithAuthStore(t)
	defer sandbox.Close()
	user, apiSecret := createTestAPIKey(t, authStore, "identity-join", []string{"acceptance:read"}, nil)
	ownerID = user.ID

	publicKey, guestPrivateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer := computerevent.SigningKey{
		SignerRef:  computerevent.SignerRef{SignerDomain: "guest-core", KeyID: "guest-core-test"},
		PrivateKey: guestPrivateKey,
	}
	guest := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Authenticated-User") != ownerID {
			http.Error(w, "authenticated user unavailable", http.StatusUnauthorized)
			return
		}
		issuedAt := time.Now().UTC()
		expiresAt := issuedAt.Add(2 * time.Minute)
		identity := map[string]any{
			"schema": executionIdentitySchemaV1, "nonce": r.URL.Query().Get("nonce"),
			"audience":    executionIdentityAudience,
			"computer_id": computerID, "realization_id": vmID + "-epoch-42", "vm_epoch": "42",
			"executable":           map[string]string{"path": "/nix/store/sandbox/bin/choir", "sha256": "sha256:guest"},
			"guest_image_manifest": map[string]string{"path": "/nix/store/guest", "sha256": "sha256:image"},
			"kernel_configuration": map[string]string{"path": "/nix/store/kernel", "sha256": "sha256:kernel"},
			"build":                buildinfo.Info{Commit: commit, DeployedCommit: commit},
			"issued_at":            issuedAt.Format(time.RFC3339Nano), "expires_at": expiresAt.Format(time.RFC3339Nano),
		}
		fields := make(map[string]any, len(identity)-1)
		for key, value := range identity {
			if key != "issued_at" {
				fields[key] = value
			}
		}
		receipt, receiptErr := computerevent.NewSignedReceipt("ExecutionIdentity", "choir-sandbox", fields, []computerevent.SigningKey{signer}, issuedAt)
		if receiptErr != nil {
			t.Fatal(receiptErr)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"schema": executionIdentitySchemaV1, "identity": identity, "receipt": receipt,
			"signer_public_key": base64.RawStdEncoding.EncodeToString(publicKey),
		})
	}))
	defer guest.Close()

	createdAt := time.Date(2026, 7, 21, 0, 0, 0, 0, time.UTC)
	closure, err := computerversion.NewCodeClosure(commit, []computerversion.CodeArtifact{{Name: "sandbox", SHA256: strings.Repeat("a", 64), URI: "nix-store+sha256://" + strings.Repeat("a", 64) + "/nix/store/test-sandbox"}}, createdAt)
	if err != nil {
		t.Fatal(err)
	}
	program, err := computerversion.NewArtifactProgram([]computerversion.ArtifactProgramEntry{{Kind: "test", ContentSHA256: strings.Repeat("b", 64), ArtifactURI: "artifact+sha256://" + strings.Repeat("b", 64) + "/state"}}, createdAt)
	if err != nil {
		t.Fatal(err)
	}
	version := computerversion.ComputerVersion{CodeRef: closure.Ref, ArtifactProgramRef: program.Ref}
	slotID, _ := routeledger.RouteSlotID(ownerID, vmctl.PrimaryDesktopID)
	routeReceipt := routeledger.TransitionReceipt{
		ID: "11111111-1111-4111-8111-111111111111", RouteSlotID: slotID, Kind: routeledger.TransitionBootstrap,
		New: version, CommittedGeneration: 1, ApprovalRef: routeledger.ApprovalRef("approval:sha256:" + strings.Repeat("c", 64)),
		PromotionCertificateRef: routeledger.PromotionCertificateRef("certificate:sha256:" + strings.Repeat("d", 64)),
		IdempotencyKey:          routeledger.IdempotencyKey("idempotency:identity-join"), CommittedAt: createdAt,
	}
	vmctlServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/internal/vmctl/lookup":
			_ = json.NewEncoder(w).Encode(map[string]any{"vm_id": vmID, "computer_id": computerID, "user_id": ownerID, "desktop_id": vmctl.PrimaryDesktopID, "sandbox_url": guest.URL, "state": "active", "epoch": epoch})
		case "/internal/vmctl/computer-version-routes/resolve":
			_ = json.NewEncoder(w).Encode(vmctl.RouteResolution{Slot: routeledger.Slot{ID: slotID, Current: version, Generation: 1, LatestReceiptID: routeReceipt.ID}, LatestReceipt: routeReceipt, CodeClosure: closure, ArtifactProgram: program})
		default:
			http.NotFound(w, r)
		}
	}))
	defer vmctlServer.Close()

	platformPublicKey, platformPrivateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	platformKeyID := computerevent.DigestBytes(platformPublicKey)[:16]
	platformSigner := computerevent.SigningKey{
		SignerRef:  computerevent.SignerRef{SignerDomain: "platform-control", KeyID: platformKeyID},
		PrivateKey: platformPrivateKey,
	}
	platformServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/platform/execution-identity/attest" || r.Header.Get("X-Internal-Caller") != "true" {
			http.NotFound(w, r)
			return
		}
		var fields map[string]any
		if err := json.NewDecoder(r.Body).Decode(&fields); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		platformReceipt, signErr := computerevent.NewSignedReceipt("ExecutionIdentityJoin", "corpusd", fields, []computerevent.SigningKey{platformSigner}, time.Now().UTC())
		if signErr != nil {
			http.Error(w, signErr.Error(), http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"receipt": platformReceipt, "signer_public_key": base64.RawStdEncoding.EncodeToString(platformPublicKey),
		})
	}))
	defer platformServer.Close()
	handler.cfg.CorpusdURL = platformServer.URL

	receiptPath := filepath.Join(t.TempDir(), "deploy-receipt.json")
	receipt := `{"schema_version":1,"target_commit":"` + commit + `","activated_at":"2026-07-21T00:00:00Z","github":{"run_id":"123","run_attempt":"1"},"artifacts":{"proxy":{"commit":"` + commit + `","status":"active"}},"host_identity":{"canonical_ref":"refs/heads/main@` + commit + `","checkout_head":"` + commit + `","checkout_clean":true,"nixos_closure":"/nix/store/system","nixos_closure_digest":"sha256:nixos","services":{"proxy":{"package_path":"/nix/store/proxy","package_digest":"sha256:proxy","embedded_commit":"` + commit + `"}}}}`
	if err := os.WriteFile(receiptPath, []byte(receipt), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CHOIR_DEPLOY_RECEIPT_PATH", receiptPath)
	previousCommit, previousVersion, previousBuiltAt := buildinfo.Commit, buildinfo.Version, buildinfo.BuiltAt
	buildinfo.Commit, buildinfo.Version, buildinfo.BuiltAt = commit, "test", createdAt.Format(time.RFC3339)
	t.Cleanup(func() {
		buildinfo.Commit, buildinfo.Version, buildinfo.BuiltAt = previousCommit, previousVersion, previousBuiltAt
	})
	handler.vmctlClient = vmctl.NewClient(vmctlServer.URL)
	handler.platformSignerDigest = "sha256:" + computerevent.DigestBytes(platformPublicKey)
	request := httptest.NewRequest(http.MethodGet, "/api/acceptance/execution-identity?nonce="+nonce, nil)
	request.Header.Set("Authorization", "Bearer "+apiSecret)
	response := httptest.NewRecorder()
	handler.HandleAPI(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("identity join status=%d body=%s", response.Code, response.Body.String())
	}
	var joined joinedExecutionIdentity
	if err := json.NewDecoder(response.Body).Decode(&joined); err != nil {
		t.Fatal(err)
	}
	if !joined.Joined || joined.Guest.Identity.ComputerID != computerID || joined.VMCTL["epoch"] != float64(epoch) ||
		!strings.HasPrefix(joined.RouteDigest, "sha256:") || joined.HostBuild.Commit != commit {
		t.Fatalf("identity join = %+v", joined)
	}
	if joined.PlatformAttestation.Receipt.ReceiptKind != "ExecutionIdentityJoin" ||
		joined.PlatformAttestation.Receipt.Verify(executionIdentityKeyResolver{
			keyID:  joined.PlatformAttestation.Receipt.RequiredSigners[0].KeyID,
			domain: "platform-control",
			key:    platformPublicKey,
		}) != nil {
		t.Fatalf("platform identity attestation did not verify: %+v", joined.PlatformAttestation)
	}
	handler.platformSignerDigest = "sha256:wrong"
	badRequest := httptest.NewRequest(http.MethodGet, request.URL.String(), nil)
	badRequest.Header.Set("Authorization", "Bearer "+apiSecret)
	badResponse := httptest.NewRecorder()
	handler.HandleAPI(badResponse, badRequest)
	if badResponse.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d body=%s, want platform trust-anchor refusal", badResponse.Code, badResponse.Body.String())
	}
	handler.platformSignerDigest = "sha256:" + computerevent.DigestBytes(platformPublicKey)
	platformSigner.SignerRef.KeyID = "untrusted-key-id"
	untrustedIDRequest := httptest.NewRequest(http.MethodGet, request.URL.String(), nil)
	untrustedIDRequest.Header.Set("Authorization", "Bearer "+apiSecret)
	untrustedIDResponse := httptest.NewRecorder()
	handler.HandleAPI(untrustedIDResponse, untrustedIDRequest)
	if untrustedIDResponse.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d body=%s, want signer key-id refusal", untrustedIDResponse.Code, untrustedIDResponse.Body.String())
	}
	platformSigner.SignerRef.KeyID = platformKeyID
	cookieRequest := httptest.NewRequest(http.MethodGet, request.URL.String(), nil)
	cookieRequest.AddCookie(&http.Cookie{Name: "choir_access", Value: issueTestAccessJWT(privateKey, ownerID)})
	cookieResponse := httptest.NewRecorder()
	handler.HandleAPI(cookieResponse, cookieRequest)
	if cookieResponse.Code != http.StatusForbidden {
		t.Fatalf("cookie acceptance status=%d body=%s, want 403", cookieResponse.Code, cookieResponse.Body.String())
	}
	joined.Guest.Identity.Nonce = "tampered-nonce-value"
	if err := verifyGuestExecutionIdentity(joined.Guest); err == nil {
		t.Fatal("guest execution identity verifier accepted tampered signed fields")
	}
}

func TestExecutionIdentityCommitsJoinKeepsRouteIdentityIndependent(t *testing.T) {
	const commit = "1234567890abcdef1234567890abcdef12345678"
	const routeCommit = "abcdef1234567890abcdef1234567890abcdef12"
	matching := buildinfo.Info{Commit: commit, DeployedCommit: commit}
	if !executionIdentityCommitsJoin(commit, matching, commit, routeCommit, matching) {
		t.Fatal("independently versioned route identity was refused")
	}
	tests := map[string]struct {
		target, hostEmbedded, route string
		host, guest                 buildinfo.Info
	}{
		"target":         {target: "abcdef", host: matching, hostEmbedded: commit, route: routeCommit, guest: matching},
		"host build":     {target: commit, host: buildinfo.Info{Commit: strings.Repeat("a", 40), DeployedCommit: commit}, hostEmbedded: commit, route: routeCommit, guest: matching},
		"host deployed":  {target: commit, host: buildinfo.Info{Commit: commit, DeployedCommit: strings.Repeat("a", 40)}, hostEmbedded: commit, route: routeCommit, guest: matching},
		"host package":   {target: commit, host: matching, hostEmbedded: strings.Repeat("a", 40), route: routeCommit, guest: matching},
		"route missing":  {target: commit, host: matching, hostEmbedded: commit, route: "", guest: matching},
		"route invalid":  {target: commit, host: matching, hostEmbedded: commit, route: strings.Repeat("g", 40), guest: matching},
		"guest build":    {target: commit, host: matching, hostEmbedded: commit, route: routeCommit, guest: buildinfo.Info{Commit: strings.Repeat("a", 40), DeployedCommit: commit}},
		"guest deployed": {target: commit, host: matching, hostEmbedded: commit, route: routeCommit, guest: buildinfo.Info{Commit: commit, DeployedCommit: strings.Repeat("a", 40)}},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if executionIdentityCommitsJoin(test.target, test.host, test.hostEmbedded, test.route, test.guest) {
				t.Fatal("invalid identity join was accepted")
			}
		})
	}
}

func TestDeploymentIdentityReceiptRefusesMissingHostInventory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "deploy-receipt.json")
	if err := os.WriteFile(path, []byte(`{"schema_version":1,"target_commit":"1234567890abcdef1234567890abcdef12345678","github":{"run_id":"1"},"artifacts":{"proxy":{"commit":"1234567890abcdef1234567890abcdef12345678","status":"active"}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CHOIR_DEPLOY_RECEIPT_PATH", path)
	if _, _, err := readDeploymentIdentityReceipt(); err == nil {
		t.Fatal("deployment receipt without host inventory was accepted")
	}
}
