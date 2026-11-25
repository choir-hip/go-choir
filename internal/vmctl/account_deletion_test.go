package vmctl

// Failure list: see the header of account_deletion.go (numbered 1 to 6). Every
// destructive call here goes to a mock VM manager and a registry in a
// temporary state directory; the deletion records are real rows of a real
// auth store in a temporary file.

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/auth"
	"github.com/yusefmosiah/go-choir/internal/purgeguard"
)

type acctEnv struct {
	reg   *OwnershipRegistry
	mgr   *mockVMManager
	store *auth.Store
	user  string
	own   *VMOwnership
}

func newAcctEnv(t *testing.T) *acctEnv {
	t.Helper()
	store := auth.TestStore(t)
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	mgr := &mockVMManager{}
	reg.SetVMManager(mgr)
	// The in-memory check reads the live store; the production read-only
	// file path is covered by TestAccountDestroyReadsTheRealAuthDatabaseReadOnly.
	reg.accountPurgeEligible = func(userID string, now time.Time) error { return store.PurgeEligible(userID, now) }
	u, err := store.CreateUser("acct-user", "acct@example.com")
	if err != nil {
		t.Fatal(err)
	}
	own, err := reg.ResolveOrAssign(u.ID)
	if err != nil {
		t.Fatal(err)
	}
	reg.mu.Lock()
	reg.ownerships[ownershipKey(u.ID, PrimaryDesktopID)].State = VMStateStopped
	reg.mu.Unlock()
	return &acctEnv{reg: reg, mgr: mgr, store: store, user: u.ID, own: own}
}

func (e *acctEnv) request(t *testing.T, at time.Time) {
	t.Helper()
	if _, _, err := e.store.RequestAccountDeletion(e.user, at); err != nil {
		t.Fatal(err)
	}
}

func (e *acctEnv) intact(t *testing.T) {
	t.Helper()
	if e.reg.GetOwnership(e.user) == nil || len(e.mgr.destroys) != 0 {
		t.Fatalf("something was destroyed: ownership=%v destroys=%v", e.reg.GetOwnership(e.user), e.mgr.destroys)
	}
}

// Failure 1.
func TestAccountDestroyNeedsAnOldEnoughRecord(t *testing.T) {
	e := newAcctEnv(t)
	now := time.Now().UTC()
	if _, err := e.reg.DestroyAccountComputers(e.user, now); !errors.Is(err, purgeguard.ErrNoRecord) {
		t.Fatalf("no record: %v", err)
	}
	e.intact(t)
	e.request(t, now)
	for _, at := range []time.Time{now, now.Add(24 * time.Hour), now.Add(purgeguard.GracePeriod - time.Minute)} {
		if _, err := e.reg.DestroyAccountComputers(e.user, at); !errors.Is(err, purgeguard.ErrTooEarly) {
			t.Fatalf("at +%s: %v", at.Sub(now), err)
		}
		e.intact(t)
	}
	// No auth database configured: refused as well, never allowed.
	bare := NewOwnershipRegistry("http://127.0.0.1:8085")
	if _, err := bare.DestroyAccountComputers(e.user, now.Add(100*time.Hour)); err == nil {
		t.Fatal("destroy ran with no auth database")
	}
}

func TestAccountDestroyAfterGracePeriodRemovesStateThenOwnership(t *testing.T) {
	e := newAcctEnv(t)
	now := time.Now().UTC()
	e.request(t, now.Add(-purgeguard.GracePeriod-time.Hour))
	res, err := e.reg.DestroyAccountComputers(e.user, now)
	if err != nil || res.VMStates != 1 || len(e.mgr.destroys) != 1 || e.mgr.destroys[0] != e.own.VMID {
		t.Fatalf("destroy: %+v %v %v", res, err, e.mgr.destroys)
	}
	if e.reg.GetOwnership(e.user) != nil || len(e.reg.AccountComputers(e.user)) != 0 {
		t.Fatal("ownership survives")
	}
	// Idempotent.
	if res, err := e.reg.DestroyAccountComputers(e.user, now); err != nil || res.VMStates != 0 {
		t.Fatalf("second destroy: %+v %v", res, err)
	}
}

// Failure 5: a failed state deletion keeps the ownership.
func TestAccountDestroyFailureKeepsOwnership(t *testing.T) {
	e := newAcctEnv(t)
	now := time.Now().UTC()
	e.request(t, now.Add(-purgeguard.GracePeriod-time.Hour))
	e.mgr.destroyError = errors.New("disk busy")
	if _, err := e.reg.DestroyAccountComputers(e.user, now); err == nil {
		t.Fatal("a failed destroy reported success")
	}
	if e.reg.GetOwnership(e.user) == nil {
		t.Fatal("ownership dropped though the state was not deleted")
	}
}

// Failure 2: the paper and the owner are never touched, alone or beside an
// ordinary computer of the same account; the whole account is refused.
func TestAccountDestroyRefusesProtectedComputers(t *testing.T) {
	for _, class := range []WarmnessClass{WarmnessClassCriticalProtected, WarmnessClassPublicPlatform} {
		e := newAcctEnv(t)
		now := time.Now().UTC()
		e.request(t, now.Add(-10*purgeguard.GracePeriod))
		// Another, ordinary computer of the same account.
		if _, err := e.reg.ResolveOrAssignDesktop(e.user, "second"); err != nil {
			t.Fatal(err)
		}
		e.reg.mu.Lock()
		e.reg.ownerships[ownershipKey(e.user, PrimaryDesktopID)].WarmnessClass = class
		e.reg.ownerships[ownershipKey(e.user, "second")].State = VMStateStopped
		e.reg.mu.Unlock()
		if _, err := e.reg.DestroyAccountComputers(e.user, now); !errors.Is(err, ErrAccountRefused) {
			t.Fatalf("%s: %v", class, err)
		}
		if len(e.mgr.destroys) != 0 || e.reg.GetOwnership(e.user) == nil || e.reg.GetOwnershipForDesktop(e.user, "second") == nil {
			t.Fatalf("%s: a computer of a protected account was destroyed: %v", class, e.mgr.destroys)
		}
		if n, err := e.reg.StopAccountComputers(e.user); n != 0 || !errors.Is(err, ErrAccountRefused) {
			t.Fatalf("%s: stop: %d %v", class, n, err)
		}
		if err := e.reg.AccountPrecheck(e.user); !errors.Is(err, ErrAccountRefused) {
			t.Fatalf("%s: precheck: %v", class, err)
		}
	}
	// A platform computer named by id in the policy.
	e := newAcctEnv(t)
	cfg := DefaultWarmnessPolicyConfig()
	cfg.PlatformComputerIDs[e.own.ComputerID] = true
	e.reg.SetWarmnessPolicyConfig(cfg)
	e.request(t, time.Now().Add(-10*purgeguard.GracePeriod))
	if _, err := e.reg.DestroyAccountComputers(e.user, time.Now()); !errors.Is(err, ErrAccountRefused) {
		t.Fatalf("platform id: %v", err)
	}
	e.intact(t)
	// The operator proof computer (computer-tiers.json): the tape sweep refuses
	// it, so vmctl refuses it before anything irreversible.
	e = newAcctEnv(t)
	cfg = DefaultWarmnessPolicyConfig()
	cfg.OperatorProofComputerIDs = map[string]bool{e.own.ComputerID: true}
	e.reg.SetWarmnessPolicyConfig(cfg)
	e.request(t, time.Now().Add(-10*purgeguard.GracePeriod))
	if err := e.reg.AccountPrecheck(e.user); !errors.Is(err, ErrAccountRefused) {
		t.Fatalf("operator proof precheck: %v", err)
	}
	if _, err := e.reg.DestroyAccountComputers(e.user, time.Now()); !errors.Is(err, ErrAccountRefused) {
		t.Fatalf("operator proof id: %v", err)
	}
	e.intact(t)
}

// Failure 3.
func TestAccountDestroyRefusesHeldAndLiveCapsule(t *testing.T) {
	e := newAcctEnv(t)
	e.request(t, time.Now().Add(-10*purgeguard.GracePeriod))
	if err := e.reg.SetHold(e.own.ComputerID, "maintenance", "test"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.reg.DestroyAccountComputers(e.user, time.Now()); !errors.Is(err, ErrAccountRefused) {
		t.Fatalf("held: %v", err)
	}
	e.intact(t)

	e2 := newAcctEnv(t)
	e2.request(t, time.Now().Add(-10*purgeguard.GracePeriod))
	if _, err := e2.reg.ResolveOrAssignDesktop(e2.user, "capsule-1"); err != nil {
		t.Fatal(err)
	}
	e2.reg.mu.Lock()
	e2.reg.ownerships[ownershipKey(e2.user, "capsule-1")].State = VMStateActive
	e2.reg.mu.Unlock()
	if _, err := e2.reg.DestroyAccountComputers(e2.user, time.Now()); !errors.Is(err, ErrAccountRefused) {
		t.Fatalf("live capsule: %v", err)
	}
	e2.intact(t)
}

func TestAccountStopStopsRunningComputers(t *testing.T) {
	e := newAcctEnv(t)
	e.reg.mu.Lock()
	e.reg.ownerships[ownershipKey(e.user, PrimaryDesktopID)].State = VMStateActive
	e.reg.mu.Unlock()
	n, err := e.reg.StopAccountComputers(e.user)
	if err != nil || n != 1 || len(e.mgr.stops) != 1 {
		t.Fatalf("stop: %d %v %v", n, err, e.mgr.stops)
	}
	if n, err := e.reg.StopAccountComputers(e.user); err != nil || n != 0 {
		t.Fatalf("second stop: %d %v", n, err)
	}
	// Another account's computer is not touched.
	other, err := e.reg.ResolveOrAssign("other-user")
	if err != nil {
		t.Fatal(err)
	}
	e.reg.mu.Lock()
	e.reg.ownerships[ownershipKey("other-user", PrimaryDesktopID)].State = VMStateActive
	e.reg.mu.Unlock()
	if _, err := e.reg.StopAccountComputers(e.user); err != nil {
		t.Fatal(err)
	}
	if got := e.reg.GetOwnership("other-user"); got.State != VMStateActive || strings.Contains(strings.Join(e.mgr.stops, ","), other.VMID) {
		t.Fatal("another account was stopped")
	}
}

// Failure 6.
func TestAccountRoutesRefuseNonHostCallers(t *testing.T) {
	e := newAcctEnv(t)
	h := NewHandler(e.reg)
	for _, tc := range []struct {
		path string
		fn   http.HandlerFunc
	}{{"/internal/vmctl/account/computers", h.HandleAccountComputers}, {"/internal/vmctl/account/stop", h.HandleAccountStop}, {"/internal/vmctl/account/destroy", h.HandleAccountDestroy}} {
		req := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(`{"user_id":"`+e.user+`"}`))
		req.RemoteAddr = "10.0.0.7:5555" // not loopback: a guest or a stranger
		rec := httptest.NewRecorder()
		tc.fn(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("%s from a non-host: %d", tc.path, rec.Code)
		}
		req = httptest.NewRequest(http.MethodGet, tc.path, nil)
		rec = httptest.NewRecorder()
		tc.fn(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("%s GET: %d", tc.path, rec.Code)
		}
	}
	e.intact(t)
	// Over the route, with the host transport, the record rule still holds.
	req := httptest.NewRequest(http.MethodPost, "/internal/vmctl/account/destroy", strings.NewReader(`{"user_id":"`+e.user+`"}`))
	req.RemoteAddr = "127.0.0.1:4000"
	rec := httptest.NewRecorder()
	h.HandleAccountDestroy(rec, req)
	if rec.Code != http.StatusForbidden || len(e.mgr.destroys) != 0 {
		t.Fatalf("destroy with no record over the route: %d %s", rec.Code, rec.Body)
	}
}

// The production read: the destroy route opens the auth database file
// read-only while the auth service holds it open and writes to it.
func TestAccountDestroyReadsTheRealAuthDatabaseReadOnly(t *testing.T) {
	dir := t.TempDir()
	store, err := auth.OpenStore(dir + "/auth.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	u, err := store.CreateUser("ro-user", "ro@example.com")
	if err != nil {
		t.Fatal(err)
	}
	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	mgr := &mockVMManager{}
	reg.SetVMManager(mgr)
	reg.SetAccountPurgeAuthDB(dir + "/auth.db")
	if _, err := reg.ResolveOrAssign(u.ID); err != nil {
		t.Fatal(err)
	}
	reg.mu.Lock()
	reg.ownerships[ownershipKey(u.ID, PrimaryDesktopID)].State = VMStateStopped
	reg.mu.Unlock()
	now := time.Now().UTC()
	if _, err := reg.DestroyAccountComputers(u.ID, now); !errors.Is(err, purgeguard.ErrNoRecord) {
		t.Fatalf("no record: %v", err)
	}
	if _, _, err := store.RequestAccountDeletion(u.ID, now); err != nil {
		t.Fatal(err)
	}
	if _, err := reg.DestroyAccountComputers(u.ID, now.Add(time.Hour)); !errors.Is(err, purgeguard.ErrTooEarly) {
		t.Fatalf("young record: %v", err)
	}
	if res, err := reg.DestroyAccountComputers(u.ID, now.Add(purgeguard.GracePeriod)); err != nil || res.VMStates != 1 {
		t.Fatalf("old record: %+v %v", res, err)
	}
}

// The hard list refuses an account whatever class vmctl computes: the paper's
// and the owner's computer ids, on an ordinary class.
func TestAccountDestroyRefusesTheHardListWhateverTheClass(t *testing.T) {
	for id := range purgeguard.ProtectedComputers {
		e := newAcctEnv(t)
		e.reg.mu.Lock()
		e.reg.ownerships[ownershipKey(e.user, PrimaryDesktopID)].ComputerID = id
		e.reg.mu.Unlock()
		e.request(t, time.Now().Add(-10*purgeguard.GracePeriod))
		if _, err := e.reg.DestroyAccountComputers(e.user, time.Now()); !errors.Is(err, ErrAccountRefused) {
			t.Fatalf("%s: %v", id, err)
		}
		if err := e.reg.AccountPrecheck(e.user); !errors.Is(err, ErrAccountRefused) {
			t.Fatalf("%s precheck: %v", id, err)
		}
		e.intact(t)
	}
}

// A computer that becomes refused (held) while the destroy is under way stops
// the destroy there: no further computer of the account is destroyed for a
// refusal the whole-account rule exists to honour.
func TestAccountDestroyStopsAtTheFirstRefusalMidway(t *testing.T) {
	e := newAcctEnv(t)
	e.request(t, time.Now().Add(-10*purgeguard.GracePeriod))
	for _, d := range []string{"second", "third"} {
		if _, err := e.reg.ResolveOrAssignDesktop(e.user, d); err != nil {
			t.Fatal(err)
		}
		e.reg.mu.Lock()
		e.reg.ownerships[ownershipKey(e.user, d)].State = VMStateStopped
		e.reg.mu.Unlock()
	}
	held := false
	e.mgr.destroyHook = func(vmID string) {
		if held {
			return
		}
		held = true
		for _, c := range e.reg.AccountComputers(e.user) {
			if c.VMID != vmID {
				if err := e.reg.SetHold(c.ComputerID, "maintenance", "test"); err != nil {
					t.Error(err)
				}
				return
			}
		}
	}
	if _, err := e.reg.DestroyAccountComputers(e.user, time.Now()); !errors.Is(err, ErrAccountRefused) {
		t.Fatalf("a refusal midway: %v", err)
	}
	if len(e.mgr.destroys) != 1 {
		t.Fatalf("the destroy went on past a refusal: %v", e.mgr.destroys)
	}
}
