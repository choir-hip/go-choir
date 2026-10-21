package gateway

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A gateway bearer lives one TTL (720 h on staging) from the cold boot that
// issued it. A planned refresh and a recovery keep the token and never
// re-issue it, and the ensure route refused to touch its expiry, so a
// computer that stays up and is only ever refreshed loses its gateway 30 days
// after its last cold boot (docs/learn/03-gateway-and-providers.md 2.3, the
// "inference" bullet; docs/definitions/choir-launch-fp7-gateway-identity-2026-10-21.md,
// rotation). Renewal is the small part of rotation that is safe now: the host
// vouches for a live computer each time it ensures the credential, and that
// extends the window; nothing else can. Ways it can fail, written before the
// code:
//  1. the time bomb stays: an ensure never extends the expiry
//     (TestEnsureRenewsACredentialPastHalfItsLife);
//  2. an ensure revives what the gateway already refused: an expired or
//     revoked credential (TestEnsureNeverRevivesExpiredOrRevokedCredentials);
//  3. every ensure rewrites the store: renewal churn
//     (TestEnsureLeavesAFreshCredentialAlone);
//  4. a renewal that cannot be written is accepted, or leaves the memory
//     and the file disagreeing (TestEnsureRenewalThatCannotBeWrittenChangesNothing);
//  5. the renewal is lost on a gateway restart
//     (TestEnsureRenewsACredentialPastHalfItsLife);
//  6. a call, not the host, extends it: only EnsureCredentialForPeer renews;
//     validating or using a bearer never does
//     (TestUsingABearerNeverRenewsIt);
//  7. the lifetime is invisible: /health reports how many credentials expire
//     soon, without a secret (TestHealthReportsCredentialLifetime).

func TestEnsureRenewsACredentialPastHalfItsLife(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gateway-identities.json")
	reg := NewIdentityRegistry(10 * time.Hour)
	if err := reg.SetPersistencePath(path); err != nil {
		t.Fatal(err)
	}
	cred, _ := reg.IssueCredential("computer-renew")
	reg.identities["computer-renew"].ExpiresAt = time.Now().Add(2 * time.Hour) // 20% of the life left

	result, err := reg.EnsureCredentialForPeer(cred.RawToken, peerA)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Renewed {
		t.Fatal("the ensure did not report a renewal")
	}
	if left := time.Until(reg.identities["computer-renew"].ExpiresAt); left < 9*time.Hour+50*time.Minute {
		t.Fatalf("expiry is %s away after the renewal, want about a full 10h TTL", left)
	}
	if !result.ExpiresAt.Equal(reg.identities["computer-renew"].ExpiresAt) {
		t.Fatalf("the result reports %s, the identity holds %s", result.ExpiresAt, reg.identities["computer-renew"].ExpiresAt)
	}
	if reg.identities["computer-renew"].PeerIP != peerA {
		t.Fatal("the renewing ensure lost the pin")
	}
	restarted := NewIdentityRegistry(10 * time.Hour)
	if err := restarted.SetPersistencePath(path); err != nil {
		t.Fatal(err)
	}
	if left := time.Until(restarted.identities["computer-renew"].ExpiresAt); left < 9*time.Hour {
		t.Fatalf("a gateway restart lost the renewal: %s left", left)
	}
	if _, err := restarted.ValidateCredential(cred.RawToken); err != nil {
		t.Fatalf("the renewed credential does not validate: %v", err)
	}
}

func TestEnsureLeavesAFreshCredentialAlone(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gateway-identities.json")
	reg := NewIdentityRegistry(10 * time.Hour)
	if err := reg.SetPersistencePath(path); err != nil {
		t.Fatal(err)
	}
	cred, _ := reg.IssueCredential("computer-fresh")
	before := reg.identities["computer-fresh"].ExpiresAt
	info, _ := os.Stat(path)
	result, err := reg.EnsureCredentialForPeer(cred.RawToken, "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Renewed || !reg.identities["computer-fresh"].ExpiresAt.Equal(before) {
		t.Fatal("a credential with more than half its life left was renewed")
	}
	after, _ := os.Stat(path)
	if !after.ModTime().Equal(info.ModTime()) {
		t.Fatal("an ensure that changed nothing rewrote the store")
	}
}

func TestEnsureNeverRevivesExpiredOrRevokedCredentials(t *testing.T) {
	reg := NewIdentityRegistry(10 * time.Hour)
	cred, _ := reg.IssueCredential("computer-expired")
	reg.identities["computer-expired"].ExpiresAt = time.Now().Add(-time.Minute)
	if _, err := reg.EnsureCredentialForPeer(cred.RawToken, peerA); err == nil {
		t.Fatal("an expired credential was ensured")
	}
	if !reg.identities["computer-expired"].ExpiresAt.Before(time.Now()) {
		t.Fatal("a refused ensure extended the expiry")
	}
	if _, err := reg.ValidateCredential(cred.RawToken); err == nil {
		t.Fatal("an expired credential validates")
	}

	cred2, _ := reg.IssueCredential("computer-revoked")
	reg.identities["computer-revoked"].ExpiresAt = time.Now().Add(time.Hour)
	reg.RevokeCredential("computer-revoked")
	if _, err := reg.EnsureCredentialForPeer(cred2.RawToken, peerA); err == nil {
		t.Fatal("a revoked credential was ensured")
	}
	if time.Until(reg.identities["computer-revoked"].ExpiresAt) > 2*time.Hour {
		t.Fatal("a refused ensure extended a revoked credential")
	}
}

func TestEnsureRenewalThatCannotBeWrittenChangesNothing(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("a root process can write into a read-only directory")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "gateway-identities.json")
	reg := NewIdentityRegistry(10 * time.Hour)
	if err := reg.SetPersistencePath(path); err != nil {
		t.Fatal(err)
	}
	cred, _ := reg.IssueCredential("computer-disk")
	soon := time.Now().Add(time.Hour)
	reg.identities["computer-disk"].ExpiresAt = soon
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
	if _, err := reg.EnsureCredentialForPeer(cred.RawToken, peerA); err == nil {
		t.Fatal("a renewal that could not be written was accepted")
	}
	id := reg.identities["computer-disk"]
	if !id.ExpiresAt.Equal(soon) || id.PeerIP != "" {
		t.Fatalf("a failed write left memory changed: expires %s pin %q", id.ExpiresAt, id.PeerIP)
	}
}

func TestUsingABearerNeverRenewsIt(t *testing.T) {
	h, reg, _ := setupHandler(t)
	h.SetBearerBinding(BindingOff)
	cred, _ := reg.IssueCredential("computer-use")
	soon := time.Now().Add(time.Minute)
	reg.identities["computer-use"].ExpiresAt = soon
	for i := 0; i < 3; i++ {
		_ = inferFrom(h, cred.RawToken, peerA)
		if _, err := reg.ValidateCredential(cred.RawToken); err != nil {
			t.Fatal(err)
		}
	}
	if !reg.identities["computer-use"].ExpiresAt.Equal(soon) {
		t.Fatal("using a bearer extended its life: only the host's ensure may")
	}
}

func TestHealthReportsCredentialLifetime(t *testing.T) {
	h, reg, _ := setupHandler(t)
	a, _ := reg.IssueCredential("computer-a")
	b, _ := reg.IssueCredential("computer-b")
	_, _ = reg.IssueCredential("computer-c")
	reg.identities["computer-c"].ExpiresAt = time.Now().Add(60 * 24 * time.Hour)
	reg.identities["computer-a"].ExpiresAt = time.Now().Add(48 * time.Hour)
	reg.identities["computer-b"].ExpiresAt = time.Now().Add(30 * 24 * time.Hour)
	rec := httptest.NewRecorder()
	h.HandleHealth(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
	var body struct {
		Credentials struct {
			Active        int `json:"active"`
			Expiring7d    int `json:"expiring_within_7d"`
			SoonestHours  int `json:"soonest_expiry_hours"`
			RenewalsTotal int `json:"renewals"`
		} `json:"credentials"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	c := body.Credentials
	if c.Active != 3 || c.Expiring7d != 1 || c.SoonestHours < 46 || c.SoonestHours > 48 {
		t.Fatalf("credentials = %+v; body %s", c, rec.Body.String())
	}
	for _, secret := range []string{a.RawToken, b.RawToken, "computer-a", "computer-b"} {
		if strings.Contains(rec.Body.String(), secret) {
			t.Fatalf("health names %q", secret)
		}
	}
}
