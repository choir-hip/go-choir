package vmctl

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/gateway"
)

// Problem 25 (docs/problems/claude-delegate-relay-compares-vm-id-with-computer-grant-2026-11-03.md):
// vmctl issues a VM's gateway bearer under the VM id, and the gateway's
// Claude delegate grant names product computer ids. vmctl is the authority
// that knows which computer a VM realizes, so it names that computer when it
// asks the gateway to issue or import the bearer. Ways it can fail, written
// before the code:
//  1. a boot's issuance names no computer, or names the VM id, so the relay
//     never grants it (TestGatewayIssuanceNamesTheProductComputer);
//  2. the bearer stops authenticating as the VM id, which every other
//     gateway route and the pin key by (same test);
//  3. an import after the gateway lost its store names no computer, so a
//     gateway restart withdraws every grant until a cold boot
//     (TestGatewayEnsureImportNamesTheProductComputer).

func realGateway(t *testing.T) (*httptest.Server, *gateway.IdentityRegistry) {
	t.Helper()
	ids := gateway.NewIdentityRegistry(time.Hour)
	h := gateway.NewHandlerWithRateLimit(ids, nil, nil)
	mux := http.NewServeMux()
	mux.HandleFunc("/provider/v1/credentials/issue", h.HandleIssueCredential)
	mux.HandleFunc("/provider/v1/credentials/ensure", h.HandleEnsureCredential)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, ids
}

func TestGatewayIssuanceNamesTheProductComputer(t *testing.T) {
	srv, ids := realGateway(t)
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	reg.SetGatewayURL(srv.URL)
	own := &VMOwnership{VMID: "vm-issue-product", UserID: "user-issue-product", DesktopID: PrimaryDesktopID}
	want := stableComputerID(own.UserID, own.DesktopID, own.ComputerID)

	token := reg.issueGatewayTokenFor(own)
	if token == "" {
		t.Fatal("no token issued")
	}
	vm, computer, err := ids.ValidateCredentialIdentity(token)
	if err != nil {
		t.Fatal(err)
	}
	if vm != own.VMID {
		t.Fatalf("the bearer authenticates as %q, want the VM id %q", vm, own.VMID)
	}
	if computer == "" || computer != want || computer == own.VMID {
		t.Fatalf("the identity names computer %q, want %q", computer, want)
	}

	// The platform computer's first boot names its computer the same way.
	platform := issueGatewayTokenAt(srv.URL, UniversalWirePlatformVMID, stableComputerID(UniversalWirePlatformOwnerID, UniversalWirePlatformDesktopID, ""))
	if _, computer, err := ids.ValidateCredentialIdentity(platform); err != nil || computer == "" || computer == UniversalWirePlatformVMID {
		t.Fatalf("platform identity names %q, %v", computer, err)
	}
}

func TestGatewayEnsureImportNamesTheProductComputer(t *testing.T) {
	srv, ids := realGateway(t)
	reg, own := registryWithVM(t, srv.URL, "http://10.200.0.2:8085")
	want := stableComputerID(own.UserID, own.DesktopID, own.ComputerID)

	reg.ensureExistingGatewayCredential(own.VMID)
	vm, computer, err := ids.ValidateCredentialIdentity("vm-jev-peer:host-held-token")
	if err != nil {
		t.Fatalf("the ensure did not import the bearer: %v", err)
	}
	if vm != own.VMID || computer != want {
		t.Fatalf("imported identity = (%q, %q), want (%q, %q)", vm, computer, own.VMID, want)
	}
}
