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
	executionIdentityJoinsRoute(t, "1234567890abcdef1234567890abcdef12345678")
}

// A self-development promote stores a 64-hex source-tree digest as the route
// closure's source commit; the closure validator admits it, so the join must.
func TestExecutionIdentityJoinsSelfDevelopmentSourceTreeRoute(t *testing.T) {
	executionIdentityJoinsRoute(t, strings.Repeat("9f", 32))
}

func executionIdentityJoinsRoute(t *testing.T, routeSource string) {
	const commit = "1234567890abcdef1234567890abcdef12345678"
	const targetCommit = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	var ownerID string
	const computerID = "computer-identity-join"
	const vmID = "vm-identity-join"
	const epoch = int64(42)
	const nonce = "nonce-bound-identity-join"
	handler, privateKey, autoputer, authStore := testProxyEnvWithAuthStore(t)
	defer autoputer.Close()
	user, err := authStore.CreateUser("proxy-test-user-identity-join", "identity-join@example.com")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	_, apiSecret, err := authStore.CreateComputerScopedAPIKey(t.Context(), user.ID, "identity-join", []string{"acceptance:read"}, computerID, nil)
	if err != nil {
		t.Fatalf("create bounded api key: %v", err)
	}
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
		if r.URL.Query().Get("refuse") == "true" {
			writeJSON(w, http.StatusServiceUnavailable, errorResponse{
				Error:  "execution identity unavailable",
				Reason: "incomplete or conflicting executable, realization, epoch, closure, or deploy identity",
			})
			return
		}
		issuedAt := time.Now().UTC()
		expiresAt := issuedAt.Add(2 * time.Minute)
		identity := map[string]any{
			"schema": executionIdentitySchemaV1, "nonce": r.URL.Query().Get("nonce"),
			"audience":    executionIdentityAudience,
			"computer_id": computerID, "realization_id": vmID + "-epoch-42", "vm_epoch": "42",
			"executable":           map[string]string{"path": "/nix/store/autoputer/bin/choir", "sha256": "sha256:guest"},
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
		receipt, receiptErr := computerevent.NewSignedReceipt("ExecutionIdentity", "choir-autoputer", fields, []computerevent.SigningKey{signer}, issuedAt)
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
	closure, err := computerversion.NewCodeClosure(routeSource, []computerversion.CodeArtifact{{Name: "autoputer", SHA256: strings.Repeat("a", 64), URI: "nix-store+sha256://" + strings.Repeat("a", 64) + "/nix/store/test-autoputer"}}, createdAt)
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
			_ = json.NewEncoder(w).Encode(map[string]any{"vm_id": vmID, "computer_id": computerID, "user_id": ownerID, "desktop_id": vmctl.PrimaryDesktopID, "computer_url": guest.URL, "state": "active", "epoch": epoch})
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
	receipt := `{"schema_version":1,"target_commit":"` + targetCommit + `","activated_at":"2026-07-21T00:00:00Z","github":{"run_id":"123","run_attempt":"1"},"artifacts":{"gateway":{"commit":"` + targetCommit + `","status":"active"}},"host_identity":{"canonical_ref":"refs/heads/main@` + targetCommit + `","checkout_head":"` + targetCommit + `","checkout_clean":true,"nixos_closure":"/nix/store/system","nixos_closure_digest":"sha256:nixos","services":{"proxy":{"package_path":"/nix/store/proxy","package_digest":"sha256:proxy","embedded_commit":"` + commit + `"}}}}`
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
	if joined.PlatformAttestation.Receipt.KindFields["deployed_commit"] != targetCommit {
		t.Fatalf("platform deployed commit = %v, want coordinator target %s", joined.PlatformAttestation.Receipt.KindFields["deployed_commit"], targetCommit)
	}
	refusedRequest := httptest.NewRequest(http.MethodGet, request.URL.String()+"&refuse=true", nil)
	refusedRequest.Header.Set("Authorization", "Bearer "+apiSecret)
	refusedResponse := httptest.NewRecorder()
	handler.HandleAPI(refusedResponse, refusedRequest)
	if refusedResponse.Code != http.StatusBadGateway {
		t.Fatalf("guest refusal status=%d body=%s", refusedResponse.Code, refusedResponse.Body.String())
	}
	var refused errorResponse
	if err := json.NewDecoder(refusedResponse.Body).Decode(&refused); err != nil {
		t.Fatal(err)
	}
	if refused.UpstreamStatus != http.StatusServiceUnavailable ||
		refused.Reason != "execution identity unavailable: incomplete or conflicting executable, realization, epoch, closure, or deploy identity" {
		t.Fatalf("guest refusal diagnostic=%+v", refused)
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

func TestExecutionIdentityCommitsJoinAllowsBoundMixedGeneration(t *testing.T) {
	const targetCommit = "1234567890abcdef1234567890abcdef12345678"
	const hostCommit = "abcdef1234567890abcdef1234567890abcdef12"
	const guestCommit = "fedcba0987654321fedcba0987654321fedcba09"
	const routeCommit = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	mixed := deploymentIdentityReceipt{
		TargetCommit: targetCommit,
		Artifacts: map[string]json.RawMessage{
			"gateway": json.RawMessage(`{"commit":"1234567890abcdef1234567890abcdef12345678","status":"active"}`),
		},
		HostIdentity: hostDeploymentIdentity{Services: map[string]hostServiceIdentity{
			"proxy": {EmbeddedCommit: hostCommit},
		}},
	}
	host := buildinfo.Info{Service: "proxy", Commit: hostCommit}
	guest := buildinfo.Info{Commit: guestCommit, DeployedCommit: guestCommit}
	if !executionIdentityCommitsJoin(mixed, host, routeCommit, false, guest) {
		t.Fatal("bound mixed-generation identity was refused")
	}

	selected := mixed
	selected.Artifacts = map[string]json.RawMessage{
		"proxy": json.RawMessage(`{"commit":"1234567890abcdef1234567890abcdef12345678","status":"active"}`),
	}
	selected.HostIdentity.Services = map[string]hostServiceIdentity{
		"proxy": {EmbeddedCommit: targetCommit},
	}
	selectedHost := buildinfo.Info{Service: "proxy", Commit: targetCommit, DeployedCommit: targetCommit}
	if !executionIdentityCommitsJoin(selected, selectedHost, routeCommit, false, guest) {
		t.Fatal("selected proxy identity at target commit was refused")
	}
	routeAbsent := selected
	routeAbsent.Artifacts = map[string]json.RawMessage{
		"proxy":     json.RawMessage(`{"commit":"1234567890abcdef1234567890abcdef12345678","status":"active"}`),
		"autoputer": json.RawMessage(`{"commit":"1234567890abcdef1234567890abcdef12345678","status":"installed"}`),
	}
	ordinaryGuest := buildinfo.Info{Commit: targetCommit, DeployedCommit: targetCommit}
	if !executionIdentityCommitsJoin(routeAbsent, selectedHost, "", true, ordinaryGuest) {
		t.Fatal("same-commit route-absent identity was refused")
	}
	if executionIdentityCommitsJoin(routeAbsent, selectedHost, "", true, guest) {
		t.Fatal("route-absent identity accepted a guest outside the deployed boot closure")
	}
	routeAbsentNoProxy := routeAbsent
	routeAbsentNoProxy.Artifacts = map[string]json.RawMessage{
		"autoputer": json.RawMessage(`{"commit":"1234567890abcdef1234567890abcdef12345678","status":"installed"}`),
	}
	if executionIdentityCommitsJoin(routeAbsentNoProxy, selectedHost, "", true, ordinaryGuest) {
		t.Fatal("route-absent identity accepted a missing proxy artifact")
	}
	routeAbsentStaleHost := buildinfo.Info{Service: "proxy", Commit: hostCommit}
	if executionIdentityCommitsJoin(routeAbsent, routeAbsentStaleHost, "", true, ordinaryGuest) {
		t.Fatal("route-absent identity accepted a stale proxy host")
	}
	if executionIdentityCommitsJoin(routeAbsent, selectedHost, routeCommit, true, ordinaryGuest) {
		t.Fatal("route-absent identity accepted a route commit")
	}
	selectedStaleInventory := selected
	selectedStaleInventory.HostIdentity.Services = map[string]hostServiceIdentity{
		"proxy": {EmbeddedCommit: hostCommit},
	}

	tests := map[string]struct {
		receipt deploymentIdentityReceipt
		host    buildinfo.Info
		route   string
		guest   buildinfo.Info
	}{
		"invalid target":        {receipt: func() deploymentIdentityReceipt { r := mixed; r.TargetCommit = "abcdef"; return r }(), host: host, route: routeCommit, guest: guest},
		"host package mismatch": {receipt: mixed, host: buildinfo.Info{Service: "proxy", Commit: strings.Repeat("a", 40)}, route: routeCommit, guest: guest},
		"unselected deployed":   {receipt: mixed, host: buildinfo.Info{Service: "proxy", Commit: hostCommit, DeployedCommit: targetCommit}, route: routeCommit, guest: guest},
		"route invalid":         {receipt: mixed, host: host, route: strings.Repeat("g", 40), guest: guest},
		"guest invalid":         {receipt: mixed, host: host, route: routeCommit, guest: buildinfo.Info{Commit: strings.Repeat("g", 40), DeployedCommit: strings.Repeat("g", 40)}},
		"guest deployed":        {receipt: mixed, host: host, route: routeCommit, guest: buildinfo.Info{Commit: guestCommit, DeployedCommit: targetCommit}},
		"selected host stale":   {receipt: selected, host: host, route: routeCommit, guest: guest},
		"selected pair stale":   {receipt: selectedStaleInventory, host: buildinfo.Info{Service: "proxy", Commit: hostCommit, DeployedCommit: targetCommit}, route: routeCommit, guest: guest},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if executionIdentityCommitsJoin(test.receipt, test.host, test.route, false, test.guest) {
				t.Fatal("conflicting identity join was accepted")
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

// A merged receipt (docs/problems/partial-deploy-receipt-blanks-deployed-commit-2026-10-14.md)
// carries the platform deployed_commit; the strict identity decoder must
// accept it (R1) and refuse one that conflicts with the target (R2).
func TestDeploymentIdentityReceiptAcceptsMergedPlatformCommit(t *testing.T) {
	const target = "1234567890abcdef1234567890abcdef12345678"
	const carried = "abcdef1234567890abcdef1234567890abcdef12"
	receipt := func(platform string) string {
		return `{"schema_version":1,"target_commit":"` + target + `","deployed_commit":"` + platform + `","activated_at":"2026-10-14T00:00:00Z","github":{"run_id":"123","run_attempt":"1"},` +
			`"artifacts":{"vmctl":{"commit":"` + target + `","status":"active"},"proxy":{"commit":"` + carried + `","status":"active","carried_forward_from":"` + carried + `"}},` +
			`"host_identity":{"canonical_ref":"refs/heads/main@` + target + `","checkout_head":"` + target + `","checkout_clean":true,"nixos_closure":"/nix/store/system","nixos_closure_digest":"sha256:nixos","services":{"proxy":{"package_path":"/nix/store/proxy","package_digest":"sha256:proxy","embedded_commit":"` + carried + `"}}}}`
	}
	path := filepath.Join(t.TempDir(), "deploy-receipt.json")
	t.Setenv("CHOIR_DEPLOY_RECEIPT_PATH", path)
	if err := os.WriteFile(path, []byte(receipt(target)), 0o600); err != nil {
		t.Fatal(err)
	}
	public, parsed, err := readDeploymentIdentityReceipt()
	if err != nil {
		t.Fatalf("merged receipt refused: %v", err)
	}
	if parsed.DeployedCommit != target || !strings.Contains(string(public), `"deployed_commit":"`+target+`"`) {
		t.Fatalf("platform commit not carried: parsed=%q public=%s", parsed.DeployedCommit, public)
	}
	if !executionIdentityCommitsJoin(parsed, buildinfo.Info{Service: "proxy", Commit: carried},
		"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", false,
		buildinfo.Info{Commit: "fedcba0987654321fedcba0987654321fedcba09", DeployedCommit: "fedcba0987654321fedcba0987654321fedcba09"}) {
		t.Fatal("merged receipt with a carried proxy refused the bound identity join")
	}
	if err := os.WriteFile(path, []byte(receipt("cccccccccccccccccccccccccccccccccccccccc")), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := readDeploymentIdentityReceipt(); err == nil {
		t.Fatal("receipt with a conflicting platform commit was accepted")
	}
}

// /health answers the platform deployed commit after a partial deploy that
// did not move the proxy, beside (not inside) the proxy's per-service
// build identity, which stays blank (docs/problems/
// partial-deploy-receipt-blanks-deployed-commit-2026-10-14.md).
func TestProxyHealthReportsPlatformDeployedCommitAfterPartialDeploy(t *testing.T) {
	const target = "1234567890abcdef1234567890abcdef12345678"
	const carried = "abcdef1234567890abcdef1234567890abcdef12"
	path := filepath.Join(t.TempDir(), "deploy-receipt.json")
	if err := os.WriteFile(path, []byte(`{"schema_version":1,"target_commit":"`+target+`","deployed_commit":"`+target+`","activated_at":"2026-10-14T19:39:00Z",`+
		`"artifacts":{"vmctl":{"commit":"`+target+`","status":"active"},"proxy":{"commit":"`+carried+`","status":"active","carried_forward_from":"`+carried+`"}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CHOIR_DEPLOY_RECEIPT_PATH", path)
	h, _, _ := testProxyEnv(t)
	w := httptest.NewRecorder()
	h.HandleHealth(w, httptest.NewRequest(http.MethodGet, "/health", nil))
	var body struct {
		Build    map[string]any `json:"build"`
		Platform map[string]any `json:"platform"`
	}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Platform["deployed_commit"] != target || body.Platform["source"] != buildinfo.PlatformSourceReceipt {
		t.Fatalf("health platform = %v, want deployed_commit %s from the receipt", body.Platform, target)
	}
	if _, present := body.Build["deployed_commit"]; present {
		t.Fatalf("health build = %v, want no per-service deployed_commit for a carried proxy", body.Build)
	}
}

// A proxy artifact carried forward from an earlier deploy means this
// deploy did not move the proxy (R4): it joins like an unselected proxy.
// An unmarked proxy artifact whose commit is not the target stays a
// conflict.
func TestExecutionIdentityCommitsJoinTreatsCarriedProxyAsUnselected(t *testing.T) {
	const targetCommit = "1234567890abcdef1234567890abcdef12345678"
	const hostCommit = "abcdef1234567890abcdef1234567890abcdef12"
	const guestCommit = "fedcba0987654321fedcba0987654321fedcba09"
	const routeCommit = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	receipt := func(proxyArtifact string) deploymentIdentityReceipt {
		return deploymentIdentityReceipt{
			TargetCommit:   targetCommit,
			DeployedCommit: targetCommit,
			Artifacts: map[string]json.RawMessage{
				"vmctl": json.RawMessage(`{"commit":"` + targetCommit + `","status":"active"}`),
				"proxy": json.RawMessage(proxyArtifact),
			},
			HostIdentity: hostDeploymentIdentity{Services: map[string]hostServiceIdentity{
				"proxy": {EmbeddedCommit: hostCommit},
			}},
		}
	}
	carried := receipt(`{"commit":"` + hostCommit + `","status":"active","carried_forward_from":"` + hostCommit + `"}`)
	unmarked := receipt(`{"commit":"` + hostCommit + `","status":"active"}`)
	host := buildinfo.Info{Service: "proxy", Commit: hostCommit}
	guest := buildinfo.Info{Commit: guestCommit, DeployedCommit: guestCommit}

	if !executionIdentityCommitsJoin(carried, host, routeCommit, false, guest) {
		t.Fatal("carried proxy artifact refused the bound identity join")
	}
	if executionIdentityCommitsJoin(carried, buildinfo.Info{Service: "proxy", Commit: hostCommit, DeployedCommit: targetCommit}, routeCommit, false, guest) {
		t.Fatal("carried proxy accepted a host claiming the target")
	}
	if executionIdentityCommitsJoin(unmarked, host, routeCommit, false, guest) {
		t.Fatal("unmarked proxy artifact off the target was accepted")
	}
	routeAbsent := carried
	routeAbsent.Artifacts = map[string]json.RawMessage{
		"proxy":     carried.Artifacts["proxy"],
		"autoputer": json.RawMessage(`{"commit":"` + targetCommit + `","status":"installed"}`),
	}
	if executionIdentityCommitsJoin(routeAbsent, host, "", true, buildinfo.Info{Commit: targetCommit, DeployedCommit: targetCommit}) {
		t.Fatal("route-absent identity accepted a proxy this deploy did not move")
	}
}
