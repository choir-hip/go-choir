package vmctl

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"
)

// D6 S6 guest memory ceilings. Failure modes before code: a malformed entry
// caps a guest (dropped and named instead); a named computer is not asked
// for its ceiling act; an unnamed computer is asked; a balloon the fixed cap
// left stays inflated (every active computer is reconciled to 0); a stopped
// or hibernated computer is touched; one failing VM stops the pass for the
// rest; VMCTL_GUEST_CACHE_RETURN=off does not stop the acts; an old guest
// without ceiling mode is asked every minute.

type fakeCapManager struct {
	*rssVMManager
	mu    sync.Mutex
	calls map[string]int
	errs  map[string]error
}

func (m *fakeCapManager) SetGuestMemoryCap(vmID string, capMiB int) (GuestMemoryCapResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls[vmID] = capMiB
	if err := m.errs[vmID]; err != nil {
		return GuestMemoryCapResult{}, err
	}
	return GuestMemoryCapResult{BaseMiB: 8192, AfterMiB: 8192 - capMiB, Changed: capMiB > 0}, nil
}

func TestParseGuestMemoryCaps(t *testing.T) {
	caps, bad := ParseGuestMemoryCaps(" computer-a=3072, vm-b=2048,junk,c=0,d=-1,e=x,=5,f=512,")
	if want := map[string]int{"computer-a": 3072, "vm-b": 2048}; !reflect.DeepEqual(caps, want) {
		t.Fatalf("caps = %v, want %v", caps, want)
	}
	if want := []string{"junk", "c=0", "d=-1", "e=x", "=5", "f=512"}; !reflect.DeepEqual(bad, want) {
		t.Fatalf("bad = %v, want %v", bad, want)
	}
	if caps, bad := ParseGuestMemoryCaps(""); caps != nil || bad != nil {
		t.Fatalf("empty = %v %v", caps, bad)
	}
}

// Class entries (the ceiling on every computer, widened ring by ring). Ways
// it can fail, written first:
//   - a class entry overrides a computer's own entry (the paper's 3072 must
//     win over any class it is in);
//   - a computer of another class, or of none, gets the class's ceiling;
//   - "class:" with no name, or an unknown spelling, selects every computer.
func TestGuestMemoryCapForClass(t *testing.T) {
	caps, bad := ParseGuestMemoryCaps("class:primary=3072,computer-a=4096,class:=2048")
	if want := []string{"class:=2048"}; !reflect.DeepEqual(bad, want) {
		t.Fatalf("bad = %v, want %v", bad, want)
	}
	for _, tc := range []struct {
		name  string
		own   *VMOwnership
		class WarmnessClass
		want  int
	}{
		{"own entry wins over its class", &VMOwnership{VMID: "vm-a", ComputerID: "computer-a"}, WarmnessClassPrimary, 4096},
		{"class entry", &VMOwnership{VMID: "vm-b", ComputerID: "computer-b"}, WarmnessClassPrimary, 3072},
		{"another class", &VMOwnership{VMID: "vm-c", ComputerID: "computer-c"}, WarmnessClassTest, 0},
		{"no class", &VMOwnership{VMID: "vm-d"}, "", 0},
	} {
		if got := guestMemoryCapFor(caps, tc.own, tc.class); got != tc.want {
			t.Errorf("%s: cap = %d, want %d", tc.name, got, tc.want)
		}
	}
}

func TestReconcileGuestMemoryCaps(t *testing.T) {
	mgr := &fakeCapManager{rssVMManager: newRSSVMManager(), calls: map[string]int{}, errs: map[string]error{}}
	reg, _ := hostCapacityRegistry(t, mgr)
	reg.SetGuestCacheReturnConfig(GuestCacheReturnConfig{Mode: GuestCacheReturnActive, Scope: "daemon"})

	var mu sync.Mutex
	asked := map[string]map[string]any{}
	guest := func(name string, status int, steps int) *httptest.Server {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			mu.Lock()
			asked[name] = body
			mu.Unlock()
			w.WriteHeader(status)
			_ = json.NewEncoder(w).Encode(map[string]any{"outcome": "returned", "steps": steps, "reclaimed_bytes": 1 << 30, "used_bytes": 4 << 30})
		}))
		t.Cleanup(srv.Close)
		return srv
	}
	a, b, x := guest("a", http.StatusOK, 16), guest("b", http.StatusConflict, 0), guest("x", http.StatusOK, 0)
	for _, own := range []*VMOwnership{
		{VMID: "vm-a", ComputerID: "computer-a", UserID: "qa-a", DesktopID: PrimaryDesktopID, Kind: VMKindInteractive, State: VMStateActive, ComputerURL: a.URL},
		{VMID: "vm-b", ComputerID: "computer-b", UserID: "qa-b", DesktopID: PrimaryDesktopID, Kind: VMKindInteractive, State: VMStateActive, ComputerURL: b.URL},
		{VMID: "vm-x", ComputerID: "computer-x", UserID: "qa-x", DesktopID: PrimaryDesktopID, Kind: VMKindInteractive, State: VMStateActive, ComputerURL: x.URL},
		{VMID: "vm-h", ComputerID: "computer-h", UserID: "qa-h", DesktopID: PrimaryDesktopID, Kind: VMKindInteractive, State: VMStateHibernated},
	} {
		addOwnership(reg, own)
	}
	mgr.errs["vm-x"] = errors.New("vm operation in progress")
	caps := map[string]int{"computer-a": 3072, "vm-b": 2048, "computer-h": 2048}
	if n := reg.reconcileGuestMemoryCaps(context.Background(), caps); n != 1 {
		t.Fatalf("acts that wrote = %d, want 1", n)
	}
	// Every active balloon goes to 0: the ceiling never holds memory.
	if want := map[string]int{"vm-a": 0, "vm-b": 0, "vm-x": 0}; !reflect.DeepEqual(mgr.calls, want) {
		t.Fatalf("balloon calls = %v, want %v (hibernated untouched)", mgr.calls, want)
	}
	mu.Lock()
	if got := asked["a"]; got["mode"] != "ceiling" || got["scope"] != "root" || got["ceiling_mib"] != float64(3072) || got["dry_run"] != false {
		t.Fatalf("a asked %v", got)
	}
	if asked["b"]["ceiling_mib"] != float64(2048) {
		t.Fatalf("b asked %v", asked["b"])
	}
	if _, ok := asked["x"]; ok {
		t.Fatal("an unnamed computer was asked for a ceiling act")
	}
	mu.Unlock()

	// observe makes the acts dry runs; off stops them.
	reg.SetGuestCacheReturnConfig(GuestCacheReturnConfig{Mode: GuestCacheReturnObserve, Scope: "daemon"})
	reg.reconcileGuestMemoryCaps(context.Background(), caps)
	mu.Lock()
	if asked["a"]["dry_run"] != true {
		t.Fatalf("observe asked %v", asked["a"])
	}
	asked = map[string]map[string]any{}
	mu.Unlock()
	reg.SetGuestCacheReturnConfig(GuestCacheReturnConfig{Mode: GuestCacheReturnOff, Scope: "daemon"})
	reg.reconcileGuestMemoryCaps(context.Background(), caps)
	mu.Lock()
	defer mu.Unlock()
	if len(asked) != 0 {
		t.Fatalf("off still asked %v", asked)
	}
}

func TestReconcileGuestMemoryCapsOldGuestIsAskedOncePerBoot(t *testing.T) {
	mgr := &fakeCapManager{rssVMManager: newRSSVMManager(), calls: map[string]int{}, errs: map[string]error{}}
	reg, _ := hostCapacityRegistry(t, mgr)
	reg.SetGuestCacheReturnConfig(GuestCacheReturnConfig{Mode: GuestCacheReturnActive, Scope: "daemon"})
	var mu sync.Mutex
	n := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		n++
		mu.Unlock()
		w.WriteHeader(http.StatusBadRequest)
	}))
	t.Cleanup(srv.Close)
	addOwnership(reg, &VMOwnership{VMID: "vm-a", ComputerID: "computer-a", UserID: "qa-a", DesktopID: PrimaryDesktopID, Kind: VMKindInteractive, State: VMStateActive, ComputerURL: srv.URL, Epoch: 1})
	caps := map[string]int{"computer-a": 3072}
	reg.reconcileGuestMemoryCaps(context.Background(), caps)
	reg.reconcileGuestMemoryCaps(context.Background(), caps)
	mu.Lock()
	defer mu.Unlock()
	if n != 1 {
		t.Fatalf("an old guest was asked %d times, want 1", n)
	}
}
