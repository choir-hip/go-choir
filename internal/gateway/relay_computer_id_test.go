package gateway

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Problem 25 (docs/problems/claude-delegate-relay-compares-vm-id-with-computer-grant-2026-11-03.md):
// the bearer authenticates as the VM id, the delegate grant list names
// product computer ids, and the relay compared the two. The fix records the
// product computer id vmctl names at issuance on the identity, and the relay
// reads the grant, the budget and the in-flight bound by it; the bearer and
// its pin stay keyed by the VM id. Ways it can fail, written before the code:
//  1. a credential whose product computer id is granted is still withheld
//     (TestRelayGrantsByTheProductComputerID);
//  2. a VM id is compared with the grant list, so listing a VM id (or a VM
//     id that happens to equal a grant) opens the relay
//     (TestRelayNeverGrantsByTheVMID);
//  3. an identity issued before the fix, which has no product computer id,
//     is served, or is refused without saying a cold boot re-issues it
//     (TestRelayWithholdsALegacyIdentityWithTheColdBootReason);
//  4. the grant read and the relay call key different ids, so a computer the
//     grant read calls available is refused, or its spend is charged where
//     the grant read does not see it (TestRelayCallUsesTheGrantReadsID);
//  5. the host routes drop the product computer id, or the store forgets it
//     across a gateway restart (TestCredentialRoutesCarryTheProductComputerID);
//  6. the host's ensure rewrites an identity's product computer id
//     (TestEnsureNeverRewritesTheProductComputerID).

const relayTestOtherComputer = "computer-relay-other"

type grantAnswer struct {
	Available       bool   `json:"available"`
	Reason          string `json:"reason"`
	DailyTokensUsed int64  `json:"daily_tokens_used"`
}

func (rh *relayHarness) grantAs(bearer, peer string) (int, grantAnswer, string) {
	rh.t.Helper()
	w := rh.do(http.MethodGet, "/provider/anthropic/grant", "", peer, func(r *http.Request) {
		r.Header.Set("Authorization", "Bearer "+bearer)
	})
	var g grantAnswer
	_ = json.Unmarshal(w.Body.Bytes(), &g)
	return w.Code, g, w.Body.String()
}

func (rh *relayHarness) messageAs(bearer, peer string) *httptest.ResponseRecorder {
	rh.t.Helper()
	return rh.do(http.MethodPost, "/provider/anthropic/v1/messages", relayMessageBody(relayTestModel), peer, func(r *http.Request) {
		r.Header.Set("Authorization", "Bearer "+bearer)
	})
}

// issuePinned issues a bearer for vmID naming productComputerID ("" for a
// legacy issuance) and pins it to peer, as vmctl's issue and ensure do.
func (rh *relayHarness) issuePinned(vmID, productComputerID, peer string) string {
	rh.t.Helper()
	var cred *CredentialResult
	var err error
	if productComputerID == "" {
		cred, err = rh.reg.IssueCredential(vmID)
	} else {
		cred, err = rh.reg.IssueCredentialForComputer(vmID, productComputerID)
	}
	if err != nil {
		rh.t.Fatal(err)
	}
	if _, err := rh.reg.EnsureCredentialForPeer(cred.RawToken, peer); err != nil {
		rh.t.Fatal(err)
	}
	return cred.RawToken
}

func TestRelayGrantsByTheProductComputerID(t *testing.T) {
	rh := newRelayHarness(t)
	// The harness bearer authenticates as relayTestVM and names
	// relayTestIdentity, the product computer id on the grant list.
	if relayTestVM == relayTestIdentity {
		t.Fatal("the harness must use different VM and computer ids")
	}
	code, g, body := rh.grantAs(rh.bearer, peerA)
	if code != http.StatusOK || !g.Available || g.Reason != "" {
		t.Fatalf("a granted computer's grant read: %d %s", code, body)
	}
	if w := rh.messageAs(rh.bearer, peerA); w.Code != http.StatusOK {
		t.Fatalf("a granted computer's relay call: %d %s", w.Code, w.Body.String())
	}
	if strings.Contains(body, relayTestVM) || strings.Contains(body, relayTestIdentity) {
		t.Fatalf("the grant read names an id: %s", body)
	}
}

func TestRelayNeverGrantsByTheVMID(t *testing.T) {
	rh := newRelayHarness(t)
	other := rh.issuePinned("vm-relay-other", relayTestOtherComputer, peerB)
	// The grant list names the other computer's VM id, never its computer id.
	rh.relay.identities["vm-relay-other"] = true
	code, g, body := rh.grantAs(other, peerB)
	if code != http.StatusOK || g.Available || g.Reason != "this computer has no Claude delegate grant" {
		t.Fatalf("a VM id on the grant list opened the grant read: %d %s", code, body)
	}
	if w := rh.messageAs(other, peerB); w.Code != http.StatusForbidden {
		t.Fatalf("a VM id on the grant list opened the relay: %d %s", w.Code, w.Body.String())
	}
	// A VM id that equals another computer's grant opens nothing either.
	rh.relay.identities = map[string]bool{relayTestIdentity: true}
	same := rh.issuePinned(relayTestIdentity, relayTestOtherComputer, "10.200.2.2")
	if code, g, body := rh.grantAs(same, "10.200.2.2"); code != http.StatusOK || g.Available {
		t.Fatalf("a VM id equal to a grant opened the grant read: %d %s", code, body)
	}
	if w := rh.messageAs(same, "10.200.2.2"); w.Code != http.StatusForbidden {
		t.Fatalf("a VM id equal to a grant opened the relay: %d %s", w.Code, w.Body.String())
	}
	if n := len(rh.requests()); n != 0 {
		t.Fatalf("an ungranted computer reached the upstream %d times", n)
	}
}

func TestRelayWithholdsALegacyIdentityWithTheColdBootReason(t *testing.T) {
	rh := newRelayHarness(t)
	legacy := rh.issuePinned("vm-relay-legacy", "", peerB)
	// Even with its VM id on the list, an identity that names no computer is
	// withheld, and the reason says what re-issues it.
	rh.relay.identities["vm-relay-legacy"] = true
	code, g, body := rh.grantAs(legacy, peerB)
	if code != http.StatusOK || g.Available || !strings.Contains(g.Reason, "credential predates the computer id") || !strings.Contains(g.Reason, "cold boot") {
		t.Fatalf("a legacy identity's grant read: %d %s", code, body)
	}
	w := rh.messageAs(legacy, peerB)
	if w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), "cold boot") {
		t.Fatalf("a legacy identity's relay call: %d %s", w.Code, w.Body.String())
	}
	if n := len(rh.requests()); n != 0 {
		t.Fatalf("a legacy identity reached the upstream %d times", n)
	}
	// The legacy bearer still authenticates everywhere else as its VM id.
	if vm, err := rh.reg.ValidateCredential(legacy); err != nil || vm != "vm-relay-legacy" {
		t.Fatalf("legacy bearer validates as %q, %v", vm, err)
	}
	// A cold boot re-issues the credential for the VM, naming its computer.
	rh.relay.identities[relayTestOtherComputer] = true
	fresh := rh.issuePinned("vm-relay-legacy", relayTestOtherComputer, peerB)
	if code, g, body := rh.grantAs(fresh, peerB); code != http.StatusOK || !g.Available {
		t.Fatalf("after the cold boot's re-issue: %d %s", code, body)
	}
}

func TestRelayCallUsesTheGrantReadsID(t *testing.T) {
	rh := newRelayHarness(t)
	rh.relay.dailyTokens = 1000
	if w := rh.messageAs(rh.bearer, peerA); w.Code != http.StatusOK {
		t.Fatalf("relay call: %d %s", w.Code, w.Body.String())
	}
	if rh.relay.spentToday(relayTestIdentity) == 0 {
		t.Fatal("the relay call charged nothing to the computer id the grant reads")
	}
	if got := rh.relay.spentToday(relayTestVM); got != 0 {
		t.Fatalf("the relay call charged %d to the VM id", got)
	}
	_, g, body := rh.grantAs(rh.bearer, peerA)
	if g.DailyTokensUsed != rh.relay.spentToday(relayTestIdentity) {
		t.Fatalf("the grant read sees %d spent, the relay charged %d: %s", g.DailyTokensUsed, rh.relay.spentToday(relayTestIdentity), body)
	}
	// The budget a relay call checks is the one the grant read reports.
	rh.relay.dailyTokens = rh.relay.spentToday(relayTestIdentity)
	if _, g, body := rh.grantAs(rh.bearer, peerA); g.Available {
		t.Fatalf("a spent budget left the grant read available: %s", body)
	}
	if w := rh.messageAs(rh.bearer, peerA); w.Code != http.StatusTooManyRequests {
		t.Fatalf("a spent budget left the relay call open: %d", w.Code)
	}
	// Removing the computer id from the list closes both.
	rh.relay.dailyTokens = 0
	rh.relay.identities = map[string]bool{relayTestVM: true}
	if _, g, body := rh.grantAs(rh.bearer, peerA); g.Available {
		t.Fatalf("grant read open by VM id: %s", body)
	}
	if w := rh.messageAs(rh.bearer, peerA); w.Code != http.StatusForbidden {
		t.Fatalf("relay call open by VM id: %d", w.Code)
	}
}

func hostPost(t *testing.T, serve http.HandlerFunc, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Caller", "true")
	req.Host = "localhost:8084"
	req.RemoteAddr = "127.0.0.1:12345"
	w := httptest.NewRecorder()
	serve(w, req)
	return w
}

func TestCredentialRoutesCarryTheProductComputerID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gateway-identities.json")
	h, reg := setupHandlerNoProvider(t)
	if err := reg.SetPersistencePath(path); err != nil {
		t.Fatal(err)
	}
	w := hostPost(t, h.HandleIssueCredential, "/provider/v1/credentials/issue", `{"computer_id":"vm-issued","product_computer_id":"computer-issued"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("issue: %d %s", w.Code, w.Body.String())
	}
	var issued CredentialResult
	if err := json.Unmarshal(w.Body.Bytes(), &issued); err != nil {
		t.Fatal(err)
	}
	if issued.ComputerID != "vm-issued" || !strings.HasPrefix(issued.RawToken, "vm-issued:") {
		t.Fatalf("the bearer must stay keyed by the VM id: %q", issued.ComputerID)
	}
	// An import after the gateway lost its store also carries it.
	w = hostPost(t, h.HandleEnsureCredential, "/provider/v1/credentials/ensure", `{"raw_token":"vm-imported:host-held","product_computer_id":"computer-imported"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("ensure: %d %s", w.Code, w.Body.String())
	}
	// A product computer id that is not one is refused at issuance.
	for _, bad := range []string{"computer bad", `computer-\"x`, "computer-é", "computer:x"} {
		body, _ := json.Marshal(map[string]string{"computer_id": "vm-bad", "product_computer_id": bad})
		if w := hostPost(t, h.HandleIssueCredential, "/provider/v1/credentials/issue", string(body)); w.Code != http.StatusBadRequest {
			t.Fatalf("issue with product computer id %q: %d", bad, w.Code)
		}
		body, _ = json.Marshal(map[string]string{"raw_token": "vm-bad:host-held", "product_computer_id": bad})
		if w := hostPost(t, h.HandleEnsureCredential, "/provider/v1/credentials/ensure", string(body)); w.Code != http.StatusBadRequest {
			t.Fatalf("ensure with product computer id %q: %d", bad, w.Code)
		}
	}

	// A restarted gateway reads both back from its store.
	restarted := NewIdentityRegistry(reg.tokenTTL)
	if err := restarted.SetPersistencePath(path); err != nil {
		t.Fatal(err)
	}
	for raw, want := range map[string][2]string{
		issued.RawToken:         {"vm-issued", "computer-issued"},
		"vm-imported:host-held": {"vm-imported", "computer-imported"},
	} {
		vm, computer, err := restarted.ValidateCredentialIdentity(raw)
		if err != nil || vm != want[0] || computer != want[1] {
			t.Fatalf("after restart: %q %q %v, want %v", vm, computer, err, want)
		}
	}
}

func TestEnsureNeverRewritesTheProductComputerID(t *testing.T) {
	reg := NewIdentityRegistry(time.Hour)
	cred, err := reg.IssueCredentialForComputer("vm-ensure", "computer-ensure")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := reg.EnsureCredentialForComputer(cred.RawToken, peerA, "computer-someone-else"); err == nil {
		t.Fatal("an ensure naming another computer was accepted")
	}
	if _, err := reg.EnsureCredentialForComputer(cred.RawToken, peerA, "computer-ensure"); err != nil {
		t.Fatalf("an ensure naming the same computer: %v", err)
	}
	// An ensure never backfills a legacy identity: only a cold boot's
	// issuance names the computer.
	legacy, err := reg.IssueCredential("vm-legacy")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := reg.EnsureCredentialForComputer(legacy.RawToken, peerA, "computer-legacy"); err != nil {
		t.Fatalf("an ensure of a legacy identity: %v", err)
	}
	if _, computer, err := reg.ValidateCredentialIdentity(legacy.RawToken); err != nil || computer != "" {
		t.Fatalf("the ensure backfilled a legacy identity: %q %v", computer, err)
	}
	if _, computer, _ := reg.ValidateCredentialIdentity(cred.RawToken); computer != "computer-ensure" {
		t.Fatalf("the product computer id moved to %q", computer)
	}
}
