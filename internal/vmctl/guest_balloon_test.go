package vmctl

import (
	"errors"
	"reflect"
	"sync"
	"testing"
)

// D6 S6 guest memory caps. Failure modes before code: a malformed entry caps
// a guest (dropped and named instead); a named computer misses its cap; an
// unnamed computer keeps a cap after its entry is dropped (every active
// computer is reconciled to 0); a stopped or hibernated computer is touched;
// one failing VM stops the pass for the rest.

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

func TestReconcileGuestMemoryCaps(t *testing.T) {
	mgr := &fakeCapManager{rssVMManager: newRSSVMManager(), calls: map[string]int{}, errs: map[string]error{}}
	reg, _ := hostCapacityRegistry(t, mgr)
	for _, own := range []*VMOwnership{
		{VMID: "vm-a", ComputerID: "computer-a", UserID: "qa-a", DesktopID: PrimaryDesktopID, Kind: VMKindInteractive, State: VMStateActive},
		{VMID: "vm-b", ComputerID: "computer-b", UserID: "qa-b", DesktopID: PrimaryDesktopID, Kind: VMKindInteractive, State: VMStateActive},
		{VMID: "vm-x", ComputerID: "computer-x", UserID: "qa-x", DesktopID: PrimaryDesktopID, Kind: VMKindInteractive, State: VMStateActive},
		{VMID: "vm-h", ComputerID: "computer-h", UserID: "qa-h", DesktopID: PrimaryDesktopID, Kind: VMKindInteractive, State: VMStateHibernated},
	} {
		addOwnership(reg, own)
	}
	mgr.errs["vm-x"] = errors.New("vm operation in progress")
	caps := map[string]int{"computer-a": 3072, "vm-b": 2048, "computer-h": 2048}
	if n := reg.reconcileGuestMemoryCaps(caps); n != 2 {
		t.Fatalf("changed = %d, want 2", n)
	}
	want := map[string]int{"vm-a": 3072, "vm-b": 2048, "vm-x": 0}
	if !reflect.DeepEqual(mgr.calls, want) {
		t.Fatalf("calls = %v, want %v (hibernated untouched, unnamed to 0)", mgr.calls, want)
	}
	// Dropping the entries reconciles every active computer to no cap.
	mgr.calls = map[string]int{}
	reg.reconcileGuestMemoryCaps(nil)
	if want := map[string]int{"vm-a": 0, "vm-b": 0, "vm-x": 0}; !reflect.DeepEqual(mgr.calls, want) {
		t.Fatalf("after drop calls = %v, want %v", mgr.calls, want)
	}
}
