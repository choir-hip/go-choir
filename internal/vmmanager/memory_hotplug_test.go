package vmmanager

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// Memory hotplug tests. The failure list is in
// docs/reports/memory-hotplug-controller-design-2026-10-14.md section 5;
// each test names the items it pins.

// fakeHotplugAPI emulates Firecracker 1.15.1's /hotplug/memory and passes
// every other path to the snapshot fake. The guest plugs or unplugs
// toward the request by step MiB per GET; floor is the most it can unplug
// down to (memory in use).
type fakeHotplugAPI struct {
	*fakeFirecrackerAPI
	hmu       sync.Mutex
	total     int
	requested int
	plugged   int
	step      int
	floor     int
	patches   []int
	noDevice  bool
	patchFail string
}

func (f *fakeHotplugAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/hotplug/memory" {
		f.fakeFirecrackerAPI.ServeHTTP(w, r)
		return
	}
	f.fakeFirecrackerAPI.mu.Lock()
	f.fakeFirecrackerAPI.calls = append(f.fakeFirecrackerAPI.calls, fcCall{Method: r.Method, Path: r.URL.Path})
	f.fakeFirecrackerAPI.mu.Unlock()
	f.hmu.Lock()
	defer f.hmu.Unlock()
	if f.noDevice {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"fault_message":"The memory hotplug device was not configured"}`))
		return
	}
	switch r.Method {
	case http.MethodGet:
		f.advanceLocked()
		_ = json.NewEncoder(w).Encode(map[string]int{
			"total_size_mib": f.total, "slot_size_mib": 128, "block_size_mib": 2,
			"plugged_size_mib": f.plugged, "requested_size_mib": f.requested,
		})
	case http.MethodPatch:
		raw, _ := io.ReadAll(r.Body)
		var body map[string]int
		if err := json.Unmarshal(raw, &body); err != nil || len(body) != 1 {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"fault_message":"bad body"}`))
			return
		}
		if f.patchFail != "" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"fault_message":"` + f.patchFail + `"}`))
			return
		}
		req := body["requested_size_mib"]
		if req%2 != 0 || req > f.total {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"fault_message":"Size is invalid"}`))
			return
		}
		f.requested = req
		f.patches = append(f.patches, req)
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (f *fakeHotplugAPI) advanceLocked() {
	step := f.step
	if step <= 0 {
		step = 1 << 30
	}
	switch {
	case f.plugged < f.requested:
		f.plugged += min(step, f.requested-f.plugged)
	case f.plugged > f.requested:
		target := max(f.requested, f.floor)
		f.plugged -= min(step, max(f.plugged-target, 0))
	}
}

func hotplugTestManager(t *testing.T, mutate func(*ManagerConfig)) *Manager {
	t.Helper()
	return balloonTestManager(t, func(cfg *ManagerConfig) {
		cfg.MemoryHotplug = true
		if mutate != nil {
			mutate(cfg)
		}
	})
}

// F1, F2: the device and its kernel parameter appear only when both the
// manager flag and vmctl's selection say so, with Firecracker's exact field
// names, and a size Firecracker would reject omits the device instead of
// failing the boot.
func TestMemoryHotplugConfigGeneration(t *testing.T) {
	selected := balloonTestVMConfig()
	selected.MemoryHotplugMiB = 4096

	off := balloonTestManager(t, nil)
	if _, ok := off.buildFirecrackerConfig(selected, 9001)["memory-hotplug"]; ok {
		t.Fatal("flag off: memory-hotplug must not appear")
	}
	if strings.Contains(bootArgsOf(t, off.buildFirecrackerConfig(selected, 9001)), "memhp_default_state") {
		t.Fatal("flag off: kernel parameter must not appear")
	}

	on := hotplugTestManager(t, nil)
	if _, ok := on.buildFirecrackerConfig(balloonTestVMConfig(), 9001)["memory-hotplug"]; ok {
		t.Fatal("unselected computer got the device")
	}
	fc := on.buildFirecrackerConfig(selected, 9001)
	raw, err := json.Marshal(fc["memory-hotplug"])
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != `{"block_size_mib":2,"slot_size_mib":128,"total_size_mib":4096}` {
		t.Fatalf("memory-hotplug = %s", raw)
	}
	if !strings.HasSuffix(bootArgsOf(t, fc), " memhp_default_state=online_movable") {
		t.Fatalf("boot args lack the online_movable parameter: %s", bootArgsOf(t, fc))
	}
	if mc := fc["machine-config"].(map[string]interface{}); mc["mem_size_mib"] != selected.MachineMemSizeMib {
		t.Fatalf("base memory changed: %v", mc)
	}

	for name, tc := range map[string]struct {
		total, block, slot int
	}{
		"total not a slot multiple": {total: 1000},
		"total below slot":          {total: 64},
		"block not power of two":    {total: 4096, block: 6, slot: 132},
		"slot below 128":            {total: 4096, slot: 64},
	} {
		mgr := hotplugTestManager(t, func(c *ManagerConfig) {
			c.MemoryHotplugBlockMiB, c.MemoryHotplugSlotMiB = tc.block, tc.slot
		})
		cfg := balloonTestVMConfig()
		cfg.MemoryHotplugMiB = tc.total
		if _, ok := mgr.buildFirecrackerConfig(cfg, 9001)["memory-hotplug"]; ok {
			t.Fatalf("%s: invalid sizes must omit the device", name)
		}
	}
}

func bootArgsOf(t *testing.T, fc map[string]interface{}) string {
	t.Helper()
	boot, _ := fc["boot-source"].(map[string]interface{})
	args, _ := boot["boot_args"].(string)
	return args
}

// The validation mirrors Firecracker 1.15.1 vmm_config/memory_hotplug.rs.
func TestValidateMemoryHotplugSizesMatchesFirecracker(t *testing.T) {
	for _, ok := range [][3]int{{1024, 2, 128}, {4096, 2, 128}, {256, 128, 128}, {4096, 4, 256}} {
		if err := validateMemoryHotplugSizes(ok[0], ok[1], ok[2]); err != nil {
			t.Fatalf("%v rejected: %v", ok, err)
		}
	}
	for _, bad := range [][3]int{{1024, 1, 128}, {1024, 3, 129}, {1024, 2, 64}, {100, 2, 128}, {1000, 2, 128}, {1024, 256, 128}} {
		if err := validateMemoryHotplugSizes(bad[0], bad[1], bad[2]); err == nil {
			t.Fatalf("%v accepted", bad)
		}
	}
}

// F3: a selected computer serves the API even in kill hibernate mode; an
// unselected one launches exactly as before.
func TestMemoryHotplugLaunchServesAPIInKillMode(t *testing.T) {
	stateDir := shortStateDir(t)
	cfg := DefaultManagerConfig()
	cfg.StateDir = stateDir
	cfg.MemoryHotplug = true
	mgr := NewManager(cfg)
	plain := VMConfig{VMID: "vm-a"}
	if mgr.wantsAPISocket(plain) {
		t.Fatal("unselected computer in kill mode must stay --no-api")
	}
	if got := strings.Join(mgr.firecrackerLaunchArgsWithAPI("vm-a", "/cfg.json", mgr.wantsAPISocket(plain)), " "); got != "--no-api --enable-pci --id vm-a --config-file /cfg.json" {
		t.Fatalf("unselected launch args changed: %s", got)
	}
	selected := VMConfig{VMID: "vm-a", MemoryHotplugMiB: 1024}
	if !mgr.wantsAPISocket(selected) {
		t.Fatal("selected computer must get the API socket")
	}
	if got := strings.Join(mgr.firecrackerLaunchArgsWithAPI("vm-a", "/cfg.json", true), " "); !strings.HasPrefix(got, "--api-sock ") {
		t.Fatalf("selected launch args = %s", got)
	}
	if got := strings.Join(jailFirecrackerLaunchArgs(mgr.wantsAPISocket(selected)), " "); !strings.HasPrefix(got, "--api-sock ") {
		t.Fatalf("selected jailed launch args = %s", got)
	}
}

// The selection is authoritative per call: a recover or refresh without it
// boots without the device.
func TestMemoryHotplugSelectionIsAuthoritativePerCall(t *testing.T) {
	stored := VMConfig{VMID: "vm-a", MemoryHotplugMiB: 4096}
	if got := mergeVMConfigOverrides(stored, VMConfig{}).MemoryHotplugMiB; got != 0 {
		t.Fatalf("empty override kept a stale selection: %d", got)
	}
	if got := mergeVMConfigOverrides(VMConfig{VMID: "vm-a"}, VMConfig{MemoryHotplugMiB: 2048}).MemoryHotplugMiB; got != 2048 {
		t.Fatalf("override selection = %d", got)
	}
}

func runningHotplugVM(t *testing.T) (*Manager, *fakeHotplugAPI) {
	t.Helper()
	stateDir := shortStateDir(t)
	cfg := DefaultManagerConfig()
	cfg.StateDir = stateDir
	cfg.MemoryHotplug = true
	mgr := NewManager(cfg)
	mgr.vms["vm-h"] = &VMInstance{
		Config: VMConfig{VMID: "vm-h", MachineMemSizeMib: 4096, MemoryHotplugMiB: 4096},
		State:  StateRunning, done: make(chan struct{}),
	}
	api := &fakeHotplugAPI{fakeFirecrackerAPI: &fakeFirecrackerAPI{}, total: 4096}
	serveUnix(t, mgr.apiSocketPath("vm-h"), api)
	return mgr, api
}

// Run-time API over the VM's socket: status carries the base, PATCH sends
// exactly {"requested_size_mib": N}, and errors are classified.
func TestMemoryHotplugRuntimeAPI(t *testing.T) {
	mgr, api := runningHotplugVM(t)
	st, err := mgr.MemoryHotplugStatus("vm-h")
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if st != (MemoryHotplugStatus{BaseMiB: 4096, TotalMiB: 4096, BlockMiB: 2, SlotMiB: 128}) {
		t.Fatalf("status = %+v", st)
	}
	if err := mgr.SetMemoryHotplugRequestedMiB("vm-h", 1024); err != nil {
		t.Fatalf("patch: %v", err)
	}
	if st, _ = mgr.MemoryHotplugStatus("vm-h"); st.RequestedMiB != 1024 || st.PluggedMiB != 1024 {
		t.Fatalf("after grow = %+v", st)
	}
	if err := mgr.SetMemoryHotplugRequestedMiB("vm-h", 3); !errors.Is(err, ErrMemoryHotplugRejected) {
		t.Fatalf("unaligned request = %v, want rejected", err)
	}
	if err := mgr.SetMemoryHotplugRequestedMiB("vm-h", -2); !errors.Is(err, ErrMemoryHotplugRejected) {
		t.Fatalf("negative request = %v, want rejected", err)
	}

	// F4: the guest driver is not bound yet.
	api.hmu.Lock()
	api.patchFail = "Device is not active"
	api.hmu.Unlock()
	if err := mgr.SetMemoryHotplugRequestedMiB("vm-h", 512); !errors.Is(err, ErrMemoryHotplugRejected) || !strings.Contains(err.Error(), "not active") {
		t.Fatalf("inactive device = %v", err)
	}

	// F14: another operation holds the VM.
	unlock := mgr.lockVMOperation("vm-h")
	if _, err := mgr.MemoryHotplugStatus("vm-h"); !errors.Is(err, ErrMemoryHotplugBusy) {
		t.Fatalf("status while busy = %v", err)
	}
	if err := mgr.SetMemoryHotplugRequestedMiB("vm-h", 0); !errors.Is(err, ErrMemoryHotplugBusy) {
		t.Fatalf("patch while busy = %v", err)
	}
	unlock()

	// F4: a VM launched without the device.
	api.hmu.Lock()
	api.noDevice = true
	api.hmu.Unlock()
	if _, err := mgr.MemoryHotplugStatus("vm-h"); !errors.Is(err, ErrMemoryHotplugUnavailable) {
		t.Fatalf("no device = %v, want unavailable", err)
	}

	// F15: not running.
	mgr.mu.Lock()
	mgr.vms["vm-h"].State = StateHibernated
	mgr.mu.Unlock()
	if _, err := mgr.MemoryHotplugStatus("vm-h"); !errors.Is(err, ErrMemoryHotplugUnavailable) {
		t.Fatalf("hibernated = %v, want unavailable", err)
	}
	if _, err := mgr.MemoryHotplugStatus("vm-none"); !errors.Is(err, ErrMemoryHotplugUnavailable) {
		t.Fatalf("unknown VM = %v, want unavailable", err)
	}
}

// F13: a hibernate shrinks a hotplugged computer to its base after the
// guest fence and before the pause, and records it.
func TestSnapshotHibernateUnplugsBeforePause(t *testing.T) {
	f := newSnapshotFixture(t)
	f.mgr.cfg.MemoryHotplug = true
	f.cfg.MemoryHotplugMiB = 1024
	f.inst.Config = f.cfg
	hostPort, _ := f.mgr.hostPortFromHostURL(f.guestS.URL)
	writeLaunchConfig(t, f, hostPort)
	api := &fakeHotplugAPI{fakeFirecrackerAPI: f.fc, total: 1024, requested: 512, plugged: 512, step: 256}
	serveUnix(t, f.mgr.apiSocketPath(f.vmID), api)
	if err := f.mgr.HibernateVM(f.vmID); err != nil {
		t.Fatalf("HibernateVM: %v", err)
	}
	calls := strings.Join(f.fc.paths(), ",")
	patch := strings.Index(calls, "PATCH /hotplug/memory")
	pause := strings.Index(calls, "PATCH /vm")
	if patch < 0 || pause < 0 || patch > pause {
		t.Fatalf("unplug must precede the pause: %s", calls)
	}
	if len(api.patches) != 1 || api.patches[0] != 0 {
		t.Fatalf("patches = %v, want [0]", api.patches)
	}
	tl := readHibernateTimeline(t, f.mgr, f.vmID)
	found := false
	for _, mark := range tl.Marks {
		if mark.Phase == "hotplug_unplug" && strings.Contains(mark.Detail, "plugged_before_mib=512 plugged_after_mib=0") {
			found = true
		}
	}
	if !found || tl.Snapshot.Outcome != "snapshot" {
		t.Fatalf("timeline lacks the unplug or did not snapshot: %+v", tl)
	}
}

func writeLaunchConfig(t *testing.T, f *snapshotFixture, hostPort int) {
	t.Helper()
	raw, err := json.Marshal(f.mgr.buildFirecrackerConfig(f.cfg, hostPort))
	if err != nil {
		t.Fatal(err)
	}
	if !launchConfigHasMemoryHotplug(raw) {
		t.Fatal("fixture launch config lacks the device")
	}
	if err := os.WriteFile(filepath.Join(f.mgr.vmStateDir(f.vmID), "fc-config.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

// F13 partial: a guest that cannot unplug everything does not block the
// snapshot past the bound.
func TestUnplugBeforeSnapshotIsBoundedOnPartialUnplug(t *testing.T) {
	mgr, api := runningHotplugVM(t)
	api.hmu.Lock()
	api.requested, api.plugged, api.floor, api.step = 1024, 1024, 384, 512
	api.hmu.Unlock()
	sock, base, err := mgr.runningAPISocket("vm-h")
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	detail := unplugBeforeSnapshot(newFCAPIClient(sock), "vm-h", base, 700*time.Millisecond)
	if time.Since(start) > 3*time.Second {
		t.Fatalf("unplug was not bounded: %s", time.Since(start))
	}
	if !strings.Contains(detail, "plugged_before_mib=1024 plugged_after_mib=384 timeout=true") {
		t.Fatalf("detail = %q", detail)
	}
	// No device or nothing plugged: no call beyond the status read.
	api.hmu.Lock()
	api.requested, api.plugged, api.floor = 0, 0, 0
	api.patches = nil
	api.hmu.Unlock()
	if detail := unplugBeforeSnapshot(newFCAPIClient(sock), "vm-h", base, time.Second); detail != "" || len(api.patches) != 0 {
		t.Fatalf("nothing plugged: detail %q patches %v", detail, api.patches)
	}
}

// A snapshot of a computer launched without the device makes no hotplug
// call (the S3 call sequence is unchanged).
func TestLaunchConfigHasMemoryHotplug(t *testing.T) {
	if launchConfigHasMemoryHotplug([]byte(`{"machine-config":{}}`)) || launchConfigHasMemoryHotplug([]byte(`not json`)) {
		t.Fatal("false positive")
	}
	if !launchConfigHasMemoryHotplug([]byte(`{"memory-hotplug":{"total_size_mib":1024}}`)) {
		t.Fatal("false negative")
	}
}
