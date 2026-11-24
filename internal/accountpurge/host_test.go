package accountpurge

// The runner and the Host over real HTTP against the real maild and gateway
// handlers (temporary stores), a fake vmctl and a fake corpusd that record
// what they receive. The real vmctl and corpusd handlers are tested in their
// own packages; this test pins the wire between them and the runner: what a
// request carries, and what a refusal looks like to the runner.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/auth"
	"github.com/yusefmosiah/go-choir/internal/gateway"
	"github.com/yusefmosiah/go-choir/internal/maild"
	"github.com/yusefmosiah/go-choir/internal/platform"
	"github.com/yusefmosiah/go-choir/internal/purgeguard"
	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

type recorded struct {
	mu   sync.Mutex
	reqs []string
	body map[string]map[string]any
}

// has reports whether the path was called.
func (r *recorded) has(path string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, p := range r.reqs {
		if p == path {
			return true
		}
	}
	return false
}

func (r *recorded) note(path string, req *http.Request) {
	var body map[string]any
	_ = json.NewDecoder(req.Body).Decode(&body)
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.body == nil {
		r.body = map[string]map[string]any{}
	}
	r.reqs = append(r.reqs, path)
	r.body[path] = body
}

func TestRunnerOverHTTPWithRealMaildAndGateway(t *testing.T) {
	// maild: real store in a temporary directory.
	dir := t.TempDir()
	mcfg := &maild.Config{Port: maild.DefaultPort, DBPath: filepath.Join(dir, "mail.db"), StorageRoot: filepath.Join(dir, "mail"),
		PrimaryDomain: "choir.news", RootOwnerID: "user-root", ResendBaseURL: maild.DefaultResendBaseURL,
		WebhookMaxBytes: maild.DefaultWebhookMaxBody, APIMaxBytes: maild.DefaultAPIMaxBody, ProviderMaxBytes: maild.DefaultProviderMaxBody}
	if err := mcfg.EnsureDirs(); err != nil {
		t.Fatal(err)
	}
	mstore, err := maild.OpenStore(mcfg.DBPath, mcfg.StorageRoot)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = mstore.Close() })
	if err := mstore.EnsureSchema(mcfg); err != nil {
		t.Fatal(err)
	}
	mh := maild.NewHandler(mcfg, mstore)
	mux := http.NewServeMux()
	mux.HandleFunc(maild.AuthPurgeAccountPath, mh.HandleAuthPurgeAccount)
	maildSrv := httptest.NewServer(mux)
	t.Cleanup(maildSrv.Close)

	// gateway: a real identity registry holding the doomed computer's identity.
	reg := gateway.NewIdentityRegistry(time.Hour)
	// The gateway keys an identity by the VM id; the product computer id is beside it.
	cred, err := reg.IssueCredentialForComputer("vm-1", "computer-abc")
	if err != nil {
		t.Fatal(err)
	}
	gh := gateway.NewHandlerWithRateLimit(reg, nil, nil)
	gmux := http.NewServeMux()
	gmux.HandleFunc("/provider/v1/credentials/forget", gh.HandleForgetCredential)
	gwSrv := httptest.NewServer(gmux)
	t.Cleanup(gwSrv.Close)

	// vmctl and corpusd: recording fakes.
	var vm, cd recorded
	vmSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		vm.note(r.URL.Path, r)
		switch r.URL.Path {
		case "/internal/vmctl/account/computers":
			// Like vmctl, a destroyed computer is no longer listed.
			list := []vmctl.AccountComputer{{ComputerID: "computer-abc", VMID: "vm-1", DesktopID: "primary", State: "stopped", Class: "primary"}}
			if vm.has("/internal/vmctl/account/destroy") {
				list = []vmctl.AccountComputer{}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"computers": list})
		case "/internal/vmctl/account/destroy":
			_ = json.NewEncoder(w).Encode(vmctl.AccountDestroyResult{Computers: []string{"computer-abc"}, VMIDs: []string{"vm-1"}, VMStates: 1})
		default:
			_ = json.NewEncoder(w).Encode(map[string]int{"stopped": 0})
		}
	}))
	t.Cleanup(vmSrv.Close)
	cdSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cd.note(r.URL.Path, r)
		switch r.URL.Path {
		case platform.AccountPurgeTapePath:
			_ = json.NewEncoder(w).Encode(platform.AccountTapeJob{State: "done", Deleted: 13, BytesGone: 1313, ComputersDeleted: 1})
		case platform.AccountPurgePath:
			_ = json.NewEncoder(w).Encode(platform.AccountPurgeCounts{TakesDeleted: 2, AuthorsDetached: 1, RemainingByKind: map[string]int64{"choir.publication": 1}})
		default:
			_ = json.NewEncoder(w).Encode(map[string]int{"takes": 2})
		}
	}))
	t.Cleanup(cdSrv.Close)

	s := auth.TestStore(t)
	u, err := s.CreateUser("e2e-user", "e2e-user@example.com")
	if err != nil {
		t.Fatal(err)
	}
	// The mail the account has.
	if _, err := mstore.AssignUserNumber(context.Background(), "choir.news", u.ID, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := mstore.MailboxForOwner(u.ID); err != nil {
		t.Fatal(err)
	}
	requested := time.Now().UTC().Add(-purgeguard.GracePeriod - time.Hour)
	if _, _, err := s.RequestAccountDeletion(u.ID, requested); err != nil {
		t.Fatal(err)
	}
	host := NewHost(vmSrv.URL, cdSrv.URL, maildSrv.URL, gwSrv.URL)
	r := New(s, host)
	if err := r.AtOnce(context.Background(), u.ID); err != nil {
		t.Fatalf("at once: %v", err)
	}
	r.Tick(context.Background())

	if _, err := s.GetUserByID(u.ID); err == nil {
		t.Fatal("the account was not purged")
	}
	if _, err := reg.ValidateCredential(cred.RawToken); err == nil {
		t.Fatal("the gateway identity survives")
	}
	if _, err := os.Stat(filepath.Join(mcfg.StorageRoot, "users", u.ID)); err == nil {
		t.Fatal("the mailbox directory survives")
	}
	// Every destructive request carried the deletion record's request time.
	want := requested.Format(time.RFC3339)
	for path, body := range cd.body {
		if path == platform.AccountSuspendPath {
			continue
		}
		if body["deletion_requested_at"] != want {
			t.Fatalf("corpusd %s carried %v, want %s", path, body["deletion_requested_at"], want)
		}
	}
	if cd.body[platform.AccountPurgePath]["user_id"] != u.ID {
		t.Fatal("the purge did not name the account")
	}
	ids, _ := cd.body[platform.AccountPurgeTapePath]["computer_ids"].([]any)
	if len(ids) != 1 || ids[0] != "computer-abc" {
		t.Fatalf("the tape purge named %v", ids)
	}
	if !strings.Contains(strings.Join(vm.reqs, ","), "/internal/vmctl/account/destroy") {
		t.Fatalf("vmctl was not asked to destroy: %v", vm.reqs)
	}
}

// A refusal by a service holding the data reaches the runner as an error,
// finalises nothing, and a young request time is refused by maild itself.
func TestServiceRefusalStopsThePurge(t *testing.T) {
	cd := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == platform.AccountPurgePath {
			http.Error(w, `{"error":"refused"}`, http.StatusForbidden)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]int{"takes": 0})
	}))
	defer cd.Close()
	vm := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"computers": []vmctl.AccountComputer{}})
	}))
	defer vm.Close()
	mailCalled := false
	ml := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mailCalled = true
		_ = json.NewEncoder(w).Encode(maild.AccountPurgeCounts{})
	}))
	defer ml.Close()
	s := auth.TestStore(t)
	u, _ := s.CreateUser("refused-user", "refused@example.com")
	if _, _, err := s.RequestAccountDeletion(u.ID, time.Now().Add(-100*time.Hour)); err != nil {
		t.Fatal(err)
	}
	r := New(s, NewHost(vm.URL, cd.URL, ml.URL, "http://127.0.0.1:1"))
	r.Tick(context.Background())
	if !mailCalled {
		t.Fatal("mail is the first step")
	}
	if _, err := s.GetUserByID(u.ID); err != nil {
		t.Fatal("finalised though corpusd refused")
	}
	rec, _ := s.AccountDeletionFor(u.ID)
	if rec == nil || !strings.Contains(rec.LastError, "refused") {
		t.Fatalf("the refusal is not on the record: %+v", rec)
	}
}
