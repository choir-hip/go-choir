package proxy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/buildinfo"
	"github.com/yusefmosiah/go-choir/internal/computerversion"
	"github.com/yusefmosiah/go-choir/internal/routeledger"
	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

func TestExecutionIdentityJoinsGuestVMCTLRouteAndDeployReceipt(t *testing.T) {
	const commit = "1234567890abcdef1234567890abcdef12345678"
	const ownerID = "owner-identity-join"
	const computerID = "computer-identity-join"
	const vmID = "vm-identity-join"
	const epoch = int64(42)
	const nonce = "nonce-bound-identity-join"

	guest := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"schema": executionIdentitySchemaV1,
			"identity": map[string]any{
				"schema": executionIdentitySchemaV1, "nonce": r.URL.Query().Get("nonce"),
				"computer_id": computerID, "realization_id": vmID + "-epoch-42", "vm_epoch": "42",
				"executable":           map[string]string{"path": "/nix/store/sandbox/bin/choir", "sha256": "sha256:guest"},
				"guest_image_manifest": map[string]string{"path": "/nix/store/guest", "sha256": "sha256:image"},
				"kernel_configuration": map[string]string{"path": "/nix/store/kernel", "sha256": "sha256:kernel"},
				"build":                map[string]string{"commit": commit, "deployed_commit": commit},
				"issued_at":            "2026-07-21T00:00:00Z", "expires_at": "2026-07-21T00:02:00Z",
			},
			"receipt": map[string]any{"receipt_kind": "ExecutionIdentity"}, "signer_public_key": "guest-public-key",
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

	receiptPath := filepath.Join(t.TempDir(), "deploy-receipt.json")
	receipt := `{"schema_version":1,"target_commit":"` + commit + `","activated_at":"2026-07-21T00:00:00Z","github":{"run_id":"123","run_attempt":"1"},"artifacts":{"proxy":{"commit":"` + commit + `","status":"active"}},"host_identity":{"canonical_ref":"refs/heads/main@` + commit + `","checkout_head":"` + commit + `","checkout_clean":true,"nixos_closure":"/nix/store/system","services":{"proxy":{"package_path":"/nix/store/proxy","embedded_commit":"` + commit + `"}}}}`
	if err := os.WriteFile(receiptPath, []byte(receipt), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CHOIR_DEPLOY_RECEIPT_PATH", receiptPath)
	previousCommit, previousVersion, previousBuiltAt := buildinfo.Commit, buildinfo.Version, buildinfo.BuiltAt
	buildinfo.Commit, buildinfo.Version, buildinfo.BuiltAt = commit, "test", createdAt.Format(time.RFC3339)
	t.Cleanup(func() {
		buildinfo.Commit, buildinfo.Version, buildinfo.BuiltAt = previousCommit, previousVersion, previousBuiltAt
	})

	handler, privateKey, sandbox := testProxyEnv(t)
	defer sandbox.Close()
	handler.vmctlClient = vmctl.NewClient(vmctlServer.URL)
	request := httptest.NewRequest(http.MethodGet, "/api/acceptance/execution-identity?nonce="+nonce, nil)
	request.AddCookie(&http.Cookie{Name: "choir_access", Value: issueTestAccessJWT(privateKey, ownerID)})
	response := httptest.NewRecorder()
	handler.HandleAPI(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("identity join status=%d body=%s", response.Code, response.Body.String())
	}
	var joined joinedExecutionIdentity
	if err := json.NewDecoder(response.Body).Decode(&joined); err != nil {
		t.Fatal(err)
	}
	if !joined.Joined || joined.Guest.Identity.ComputerID != computerID || joined.VMCTL["epoch"] != float64(epoch) || joined.Route == nil || joined.Route.ComputerVersion != version || joined.HostBuild.Commit != commit {
		t.Fatalf("identity join = %+v", joined)
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
