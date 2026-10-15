package vmctl

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// The guest's lease set (docs/reports/memory-hotplug-controller-design-
// 2026-10-14.md section 9.4): J1 unavailable vs J2 refused, J5 one summed
// lease per computer, J9 nothing renews after the last release, J10 the
// release at process start.

type fakeGuestMemoryRoute struct {
	mu      sync.Mutex
	base    int
	gets    int
	posts   []memoryRequestBody
	refuse  func(target int) (int, string) // status, reason; 0 admits
	srv     *httptest.Server
	current int
}

func newFakeGuestMemoryRoute(t *testing.T) *fakeGuestMemoryRoute {
	t.Helper()
	f := &fakeGuestMemoryRoute{base: 4096}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/vmctl/memory/guest" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		f.mu.Lock()
		defer f.mu.Unlock()
		if r.Method == http.MethodGet {
			f.gets++
			writeVMCTLJSON(w, http.StatusOK, MemoryHotplugView{VMID: "vm-g", BaseMiB: f.base, CurrentMiB: max(f.base, f.current), RequestedMiB: max(f.base, f.current)})
			return
		}
		var body memoryRequestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.VMID != "" || body.ComputerID != "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		f.posts = append(f.posts, body)
		if f.refuse != nil {
			if status, reason := f.refuse(body.TargetMiB); status != 0 {
				writeVMCTLJSON(w, status, vmctlErrorResponse{Error: reason})
				return
			}
		}
		f.current = body.TargetMiB
		target := max(body.TargetMiB, f.base)
		writeVMCTLJSON(w, http.StatusOK, MemoryHotplugView{VMID: "vm-g", BaseMiB: f.base, RequestedMiB: target, CurrentMiB: target, PluggedMiB: target - f.base})
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeGuestMemoryRoute) targets() []int {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]int, 0, len(f.posts))
	for _, p := range f.posts {
		out = append(out, p.TargetMiB)
	}
	return out
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// J5: two needs share the computer's one lease; each release steps the
// target down; the last one releases; the base is read once.
func TestGuestMemoryLeasesSumAndStepDown(t *testing.T) {
	f := newFakeGuestMemoryRoute(t)
	set := NewGuestMemoryLeases(NewClient(f.srv.URL), time.Hour, time.Second)
	ctx := context.Background()

	v1, release1, err := set.Acquire(ctx, 768)
	if err != nil || v1.CurrentMiB != 4096+768 {
		t.Fatalf("first need: %+v %v", v1, err)
	}
	v2, release2, err := set.Acquire(ctx, 512)
	if err != nil || v2.CurrentMiB != 4096+768+512 {
		t.Fatalf("second need: %+v %v", v2, err)
	}
	release1()
	release1() // a second call is a no-op
	release2()
	if got, want := f.targets(), []int{4864, 5376, 4608, 0}; !equalInts(got, want) {
		t.Fatalf("targets = %v, want %v", got, want)
	}
	f.mu.Lock()
	gets := f.gets
	f.mu.Unlock()
	if gets != 1 {
		t.Fatalf("base read %d times, want once", gets)
	}
	if n := set.LiveMiB(); n != 0 {
		t.Fatalf("live need after both releases = %d", n)
	}
}

// J1 and J2: hotplug off or the computer not selected is "unavailable"
// (the caller runs on the budget it has); a pressure refusal is not, and
// leaves the live needs as they were.
func TestGuestMemoryLeasesUnavailableAndRefused(t *testing.T) {
	f := newFakeGuestMemoryRoute(t)
	set := NewGuestMemoryLeases(NewClient(f.srv.URL), time.Hour, time.Second)
	for _, status := range []int{http.StatusNotFound, http.StatusConflict} {
		f.mu.Lock()
		f.refuse = func(int) (int, string) { return status, "memory hotplug is disabled" }
		f.mu.Unlock()
		_, _, err := set.Acquire(context.Background(), 768)
		if err == nil || !MemoryLeaseUnavailable(err) {
			t.Fatalf("status %d: %v, want unavailable", status, err)
		}
	}
	f.mu.Lock()
	f.refuse = func(target int) (int, string) {
		if target > 5000 {
			return http.StatusServiceUnavailable, "host memory PSI some avg10 2.00 is above 1.00"
		}
		return 0, ""
	}
	f.mu.Unlock()
	_, release, err := set.Acquire(context.Background(), 512)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = set.Acquire(context.Background(), 768)
	var leaseErr *MemoryLeaseError
	if !errors.As(err, &leaseErr) || leaseErr.Status != http.StatusServiceUnavailable || MemoryLeaseUnavailable(err) {
		t.Fatalf("refused grow = %v", err)
	}
	if n := set.LiveMiB(); n != 512 {
		t.Fatalf("live need after a refusal = %d, want 512", n)
	}
	release()
	if got := f.targets(); got[len(got)-1] != 0 {
		t.Fatalf("last target = %v, want a release", got)
	}
	var nilSet *GuestMemoryLeases
	if _, _, err := nilSet.Acquire(context.Background(), 1); !MemoryLeaseUnavailable(err) {
		t.Fatalf("no vmctl client: %v, want unavailable", err)
	}
}

// Renewal while a need is live, none after the last release (J9).
func TestGuestMemoryLeasesRenewOnlyWhileLive(t *testing.T) {
	f := newFakeGuestMemoryRoute(t)
	set := NewGuestMemoryLeases(NewClient(f.srv.URL), 80*time.Millisecond, 0)
	_, release, err := set.Acquire(context.Background(), 256)
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for len(f.targets()) < 3 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	got := f.targets()
	if len(got) < 3 || got[1] != 4096+256 || got[2] != 4096+256 {
		t.Fatalf("renewals = %v, want the same target again", got)
	}
	release()
	settled := len(f.targets())
	time.Sleep(300 * time.Millisecond)
	if after := f.targets(); len(after) != settled || after[len(after)-1] != 0 {
		t.Fatalf("requests after the last release = %v (settled at %d)", after, settled)
	}
}

// J10: at process start the guest releases whatever an earlier process
// held; unavailable is not an error there.
func TestGuestMemoryLeasesReleaseAtStart(t *testing.T) {
	f := newFakeGuestMemoryRoute(t)
	set := NewGuestMemoryLeases(NewClient(f.srv.URL), time.Hour, 0)
	if err := set.ReleaseStale(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := f.targets(); !equalInts(got, []int{0}) {
		t.Fatalf("start release = %v", got)
	}
	f.mu.Lock()
	f.refuse = func(int) (int, string) { return http.StatusNotFound, "memory hotplug is disabled" }
	f.mu.Unlock()
	if err := set.ReleaseStale(context.Background()); err != nil {
		t.Fatalf("unavailable at start = %v, want nil", err)
	}
}
