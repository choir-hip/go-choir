package gateway

// Account deletion removes a destroyed computer's gateway identity, and only
// for a host caller that carries a deletion record's request time at least 72
// hours old.

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func forgetRequest(computerID string, requested time.Time, host bool) *http.Request {
	body := `{"computer_id":"` + computerID + `","deletion_requested_at":"` + requested.UTC().Format(time.RFC3339) + `"}`
	if requested.IsZero() {
		body = `{"computer_id":"` + computerID + `"}`
	}
	req := httptest.NewRequest(http.MethodPost, "/provider/v1/credentials/forget", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if host {
		req.Header.Set("X-Internal-Caller", "true")
		req.Host = "localhost:8084"
		req.RemoteAddr = "127.0.0.1:12345"
	} else {
		req.Host = "10.0.0.1:8084"
		req.RemoteAddr = "10.0.0.1:45678"
	}
	return req
}

func TestForgetCredentialNeedsHostAndAnOldRecord(t *testing.T) {
	h, reg := setupHandlerNoProvider(t)
	res, err := reg.IssueCredential("computer-doomed")
	if err != nil {
		t.Fatal(err)
	}
	other, _ := reg.IssueCredential("computer-other")
	now := time.Now()
	cases := []struct {
		name string
		req  *http.Request
		want int
	}{
		{"not the host", forgetRequest("computer-doomed", now.Add(-100*time.Hour), false), http.StatusForbidden},
		{"no time", forgetRequest("computer-doomed", time.Time{}, true), http.StatusForbidden},
		{"too young", forgetRequest("computer-doomed", now.Add(-71*time.Hour), true), http.StatusForbidden},
		{"future", forgetRequest("computer-doomed", now.Add(time.Hour), true), http.StatusForbidden},
	}
	for _, c := range cases {
		w := httptest.NewRecorder()
		h.HandleForgetCredential(w, c.req)
		if w.Code != c.want {
			t.Fatalf("%s: %d %s", c.name, w.Code, w.Body)
		}
		if _, err := reg.ValidateCredential(res.RawToken); err != nil {
			t.Fatalf("%s removed the identity: %v", c.name, err)
		}
	}
	w := httptest.NewRecorder()
	h.HandleForgetCredential(w, forgetRequest("computer-doomed", now.Add(-73*time.Hour), true))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"existed":true`) {
		t.Fatalf("forget: %d %s", w.Code, w.Body)
	}
	if _, err := reg.ValidateCredential(res.RawToken); err == nil {
		t.Fatal("the identity still validates")
	}
	if _, err := reg.ValidateCredential(other.RawToken); err != nil {
		t.Fatalf("another computer's identity was removed: %v", err)
	}
	w = httptest.NewRecorder()
	h.HandleForgetCredential(w, forgetRequest("computer-doomed", now.Add(-73*time.Hour), true))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"existed":false`) {
		t.Fatalf("second forget: %d %s", w.Code, w.Body)
	}
}

// Seat 1 #1: the gateway keys a credential by the VM id the bearer
// authenticates as; the product computer id (computer-...) is recorded beside
// it. A forget by either removes the identity, and by neither removes nothing.
func TestForgetCredentialFindsAnIdentityByItsVMOrItsProductComputer(t *testing.T) {
	reg := NewIdentityRegistry(time.Hour)
	byProduct, err := reg.IssueCredentialForComputer("vm-doomed-1", "computer-aaaa1111")
	if err != nil {
		t.Fatal(err)
	}
	byVM, err := reg.IssueCredentialForComputer("vm-doomed-2", "computer-bbbb2222")
	if err != nil {
		t.Fatal(err)
	}
	bystander, err := reg.IssueCredentialForComputer("vm-other", "computer-cccc3333")
	if err != nil {
		t.Fatal(err)
	}
	if existed, err := reg.ForgetIdentity("computer-aaaa1111"); err != nil || !existed {
		t.Fatalf("by product computer id: %v %v", existed, err)
	}
	if existed, err := reg.ForgetIdentity("vm-doomed-2"); err != nil || !existed {
		t.Fatalf("by vm id: %v %v", existed, err)
	}
	if _, err := reg.ValidateCredential(byProduct.RawToken); err == nil {
		t.Fatal("the identity forgotten by its product computer id still validates")
	}
	if _, err := reg.ValidateCredential(byVM.RawToken); err == nil {
		t.Fatal("the identity forgotten by its vm id still validates")
	}
	if _, err := reg.ValidateCredential(bystander.RawToken); err != nil {
		t.Fatalf("another computer's identity was removed: %v", err)
	}
	if existed, _ := reg.ForgetIdentity("computer-nobody"); existed {
		t.Fatal("an unknown id reported a removal")
	}
}

// Seat 2 S9: a forget whose write failed is finished by the retry, not
// reported as nothing to do while the file still holds the identity.
func TestForgetIsDurableAfterAFailedWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "identities.json")
	reg := NewIdentityRegistry(time.Hour)
	if err := reg.SetPersistencePath(path); err != nil {
		t.Fatal(err)
	}
	if _, err := reg.IssueCredentialForComputer("vm-doomed", "computer-dddd4444"); err != nil {
		t.Fatal(err)
	}
	// The write fails: the directory is gone and cannot be made (a file is in its place).
	if err := os.RemoveAll(filepath.Join(dir, "sub")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sub"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := reg.ForgetIdentity("computer-dddd4444"); err == nil {
		t.Fatal("a failed write was reported as done")
	}
	// The disk heals; the retry must write the removal even though the memory
	// no longer holds the identity.
	if err := os.Remove(filepath.Join(dir, "sub")); err != nil {
		t.Fatal(err)
	}
	if _, err := reg.ForgetIdentity("computer-dddd4444"); err != nil {
		t.Fatalf("retry: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "vm-doomed") {
		t.Fatalf("the file still holds the identity after the retry: %s", raw)
	}
}

func TestForgetCredentialRefusesTheHardList(t *testing.T) {
	h, reg := setupHandlerNoProvider(t)
	paper := "computer-1355b83bced1613eb048ee28d66688a8"
	if _, err := reg.IssueCredentialForComputer("vm-paper", paper); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	h.HandleForgetCredential(w, forgetRequest(paper, time.Now().Add(-100*time.Hour), true))
	if w.Code != http.StatusForbidden {
		t.Fatalf("the paper's computer: %d %s", w.Code, w.Body)
	}
}

// Round 2 (seat 1 #3, seat 2 F3): the hard list is applied to the identity the
// id resolves to, not only to the string named. The paper's VM id removes the
// paper's identity as surely as its product id does.
func TestForgetCredentialRefusesTheHardListByVMId(t *testing.T) {
	h, reg := setupHandlerNoProvider(t)
	paper := "computer-1355b83bced1613eb048ee28d66688a8"
	owner := "computer-03335285269bdba4f94377e56879f9e6"
	pc, err := reg.IssueCredentialForComputer("vm-paper-1", paper)
	if err != nil {
		t.Fatal(err)
	}
	oc, err := reg.IssueCredentialForComputer("vm-owner-1", owner)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"vm-paper-1", "vm-owner-1"} {
		w := httptest.NewRecorder()
		h.HandleForgetCredential(w, forgetRequest(name, time.Now().Add(-100*time.Hour), true))
		if w.Code != http.StatusForbidden {
			t.Fatalf("%s: %d %s", name, w.Code, w.Body)
		}
	}
	for _, c := range []*CredentialResult{pc, oc} {
		if _, err := reg.ValidateCredential(c.RawToken); err != nil {
			t.Fatalf("a protected identity was removed: %v", err)
		}
	}
	// An ordinary VM id still goes.
	if _, err := reg.IssueCredentialForComputer("vm-plain", "computer-aaaa5555"); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	h.HandleForgetCredential(w, forgetRequest("vm-plain", time.Now().Add(-100*time.Hour), true))
	if w.Code != http.StatusOK {
		t.Fatalf("an ordinary VM id: %d %s", w.Code, w.Body)
	}
}
