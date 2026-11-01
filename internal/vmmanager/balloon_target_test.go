package vmmanager

import (
	"encoding/json"
	"net/http"
	"reflect"
	"sync"
	"testing"
)

// D6 S6: SetGuestMemoryCap. Failure modes before code: it writes when the
// device already holds the target (needless PATCHes every minute); it
// inflates past base memory or under the floor; it touches a VM that is not
// running; it ignores a refused PATCH.

type fakeBalloonTargetAPI struct {
	mu      sync.Mutex
	amount  int
	calls   []string
	refuse  bool
	patched []int
}

func (f *fakeBalloonTargetAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, r.Method+" "+r.URL.Path)
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/machine-config":
		_ = json.NewEncoder(w).Encode(map[string]any{"vcpu_count": 2, "mem_size_mib": 8192})
	case r.Method == http.MethodGet && r.URL.Path == "/balloon":
		_ = json.NewEncoder(w).Encode(map[string]any{"amount_mib": f.amount, "deflate_on_oom": true, "free_page_reporting": true})
	case r.Method == http.MethodPatch && r.URL.Path == "/balloon":
		var body map[string]int
		_ = json.NewDecoder(r.Body).Decode(&body)
		if f.refuse {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"fault_message":"refused"}`))
			return
		}
		f.amount = body["amount_mib"]
		f.patched = append(f.patched, f.amount)
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func TestSetGuestMemoryCap(t *testing.T) {
	api := &fakeBalloonTargetAPI{}
	mgr := runningBalloonVM(t, nil, nil)
	// A reattached VM carries the manager default here; the VMM's 8192 wins.
	mgr.vms["vm-b"].Config.MachineMemSizeMib = 512
	serveUnix(t, mgr.apiSocketPath("vm-b"), api)

	res, err := mgr.SetGuestMemoryCap("vm-b", 3072)
	if err != nil || !res.Changed || res.BeforeMiB != 0 || res.AfterMiB != 5120 || res.BaseMiB != 8192 {
		t.Fatalf("cap 3072: %+v %v", res, err)
	}
	// Already at target: a GET, no PATCH.
	if res, err := mgr.SetGuestMemoryCap("vm-b", 3072); err != nil || res.Changed {
		t.Fatalf("repeat: %+v %v", res, err)
	}
	// A cap at or above base is no balloon.
	if res, err := mgr.SetGuestMemoryCap("vm-b", 16384); err != nil || res.AfterMiB != 0 {
		t.Fatalf("cap over base: %+v %v", res, err)
	}
	if _, err := mgr.SetGuestMemoryCap("vm-b", 512); err == nil {
		t.Fatal("cap under the floor accepted")
	}
	if want := []int{5120, 0}; !reflect.DeepEqual(api.patched, want) {
		t.Fatalf("patched = %v, want %v", api.patched, want)
	}
	api.refuse = true
	if _, err := mgr.SetGuestMemoryCap("vm-b", 4096); err == nil {
		t.Fatal("refused PATCH reported as success")
	}
	if _, err := mgr.SetGuestMemoryCap("vm-none", 4096); err == nil {
		t.Fatal("a VM that is not running was capped")
	}
}
