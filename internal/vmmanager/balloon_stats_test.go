package vmmanager

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// RES-10 steps 1 and 2: balloon statistics on for selected computers only,
// and a read-only, bounded reader. The failure list (1-14) is in
// docs/problems/balloon-stats-unread-no-emergency-inflation-2026-10-21.md,
// "Failure modes before code"; each test names the items it pins.

// balloonGoldenConfig is today's (58e705802) Firecracker config for
// balloonTestVMConfig with free page reporting on, StateDir replaced by
// STATEDIR.
func balloonGoldenConfig(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "balloon-unselected-fc-config-58e705802.json"))
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	return strings.TrimSpace(string(raw))
}

func fcConfigJSON(t *testing.T, mgr *Manager, cfg VMConfig) string {
	t.Helper()
	raw, err := json.Marshal(mgr.buildFirecrackerConfig(cfg, 9001))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return strings.ReplaceAll(string(raw), mgr.cfg.StateDir, "STATEDIR")
}

// 1, 2: a computer the list does not name boots with exactly today's bytes,
// whatever the interval variable says, so no snapshot hash moves.
func TestBalloonStatsUnselectedConfigIsTodaysBytes(t *testing.T) {
	golden := balloonGoldenConfig(t)
	cases := map[string]func(*ManagerConfig){
		"no selector":      func(c *ManagerConfig) {},
		"interval alone":   func(c *ManagerConfig) { c.BalloonStatsPollingIntervalS = 7 },
		"another computer": func(c *ManagerConfig) { c.BalloonStatsOnly = []string{"vm-other", "computer-other"} },
		"another class": func(c *ManagerConfig) {
			c.BalloonStatsOnly = []string{"class:test"}
			c.BalloonStatsPollingIntervalS = 5
		},
		"empty tokens":               func(c *ManagerConfig) { c.BalloonStatsOnly = []string{"", " "} },
		"class prefix with no class": func(c *ManagerConfig) { c.BalloonStatsOnly = []string{"class:"} },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			mgr := balloonTestManager(t, func(c *ManagerConfig) {
				c.BalloonFreePageReporting = true
				mutate(c)
			})
			if got := fcConfigJSON(t, mgr, balloonTestVMConfig()); got != golden {
				t.Fatalf("unselected config changed:\n got %s\nwant %s", got, golden)
			}
		})
	}
}

// 1, 3: a selected computer gets the interval (default 5), and nothing else
// in its config changes.
func TestBalloonStatsSelectedConfigHasInterval(t *testing.T) {
	golden := balloonGoldenConfig(t)
	cases := []struct {
		name     string
		only     []string
		class    string
		interval int
		want     int
	}{
		{"vm id, default interval", []string{"vm-balloon-1"}, "", 0, 5},
		{"vm id, set interval", []string{"vm-balloon-1"}, "", 30, 30},
		{"class:test", []string{"class:test"}, "test", 0, 5},
		{"star", []string{"*"}, "", 0, 5},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mgr := balloonTestManager(t, func(c *ManagerConfig) {
				c.BalloonFreePageReporting = true
				c.BalloonStatsOnly = tc.only
				c.BalloonStatsPollingIntervalS = tc.interval
			})
			vm := balloonTestVMConfig()
			vm.WarmnessClass = tc.class
			fc := mgr.buildFirecrackerConfig(vm, 9001)
			balloon, ok := fc["balloon"].(map[string]interface{})
			if !ok {
				t.Fatalf("balloon missing: %#v", fc["balloon"])
			}
			if got := balloon["stats_polling_interval_s"]; got != tc.want {
				t.Fatalf("stats_polling_interval_s = %#v, want %d", got, tc.want)
			}
			if len(balloon) != 4 {
				t.Fatalf("balloon fields = %v, want the four BalloonDeviceConfig fields", balloon)
			}
			got := fcConfigJSON(t, mgr, vm)
			want := strings.Replace(golden, `"stats_polling_interval_s":0`, `"stats_polling_interval_s":`+itoaTest(tc.want), 1)
			if got != want {
				t.Fatalf("selected config differs beyond the interval:\n got %s\nwant %s", got, want)
			}
		})
	}
}

func itoaTest(n int) string {
	raw, _ := json.Marshal(n)
	return string(raw)
}

// 4: selection never adds a balloon device.
func TestBalloonStatsSelectedWithoutBalloonAddsNoDevice(t *testing.T) {
	mgr := balloonTestManager(t, func(c *ManagerConfig) {
		c.BalloonStatsOnly = []string{"*"}
		c.BalloonStatsPollingIntervalS = 5
	})
	if b, ok := mgr.buildFirecrackerConfig(balloonTestVMConfig(), 9001)["balloon"]; ok {
		t.Fatalf("balloon added by the stats selector alone: %#v", b)
	}
}

// 1: the selector variable is the FP8 selector type.
func TestLoadConfigFromEnv_BalloonStatsSelector(t *testing.T) {
	t.Setenv("VM_BALLOON_STATS_ONLY", "computer-test-1, class:test")
	got := LoadConfigFromEnv().BalloonStatsOnly
	if len(got) != 2 || got[0] != "computer-test-1" || got[1] != "class:test" {
		t.Fatalf("BalloonStatsOnly = %#v", got)
	}
	t.Setenv("VM_BALLOON_STATS_ONLY", "")
	if got := LoadConfigFromEnv().BalloonStatsOnly; len(got) != 0 {
		t.Fatalf("empty VM_BALLOON_STATS_ONLY = %#v, want none", got)
	}
}

// fakeBalloonAPI is Firecracker's /balloon and /balloon/statistics. It fails
// the test on any method but GET (11).
type fakeBalloonAPI struct {
	t        *testing.T
	mu       sync.Mutex
	calls    []string
	noDevice bool
	interval int
	stats    string // raw /balloon/statistics body; "" with interval > 0 answers 400
	delay    time.Duration
}

func (f *fakeBalloonAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	f.calls = append(f.calls, r.Method+" "+r.URL.Path)
	noDevice, interval, stats, delay := f.noDevice, f.interval, f.stats, f.delay
	f.mu.Unlock()
	if r.Method != http.MethodGet {
		f.t.Errorf("balloon reader sent %s %s; it must only GET", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-r.Context().Done():
			return
		}
	}
	if noDevice {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"fault_message":"No balloon device found."}`))
		return
	}
	switch r.URL.Path {
	case "/balloon":
		_ = json.NewEncoder(w).Encode(map[string]any{
			"amount_mib": 0, "deflate_on_oom": true, "stats_polling_interval_s": interval, "free_page_reporting": true,
		})
	case "/balloon/statistics":
		if interval == 0 || stats == "" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"fault_message":"Statistics for the balloon device are not enabled"}`))
			return
		}
		_, _ = w.Write([]byte(stats))
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func (f *fakeBalloonAPI) callLog() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.calls...)
}

// A guest reporting every tag, plus fields newer Firecracker builds add.
const fakeBalloonStatsJSON = `{"target_pages":0,"actual_pages":0,"target_mib":0,"actual_mib":0,
"swap_in":0,"swap_out":0,"major_faults":1200,"minor_faults":3400000,
"free_memory":1073741824,"total_memory":8589934592,"available_memory":6012954214,
"disk_caches":4831838208,"hugetlb_allocations":0,"hugetlb_failures":0,"oom_kill":0}`

func runningBalloonVM(t *testing.T, api *fakeBalloonAPI, mutate func(*ManagerConfig)) *Manager {
	t.Helper()
	cfg := DefaultManagerConfig()
	cfg.StateDir = shortStateDir(t)
	cfg.BalloonFreePageReporting = true
	if mutate != nil {
		mutate(&cfg)
	}
	mgr := NewManager(cfg)
	mgr.vms["vm-b"] = &VMInstance{
		Config: VMConfig{VMID: "vm-b", ComputerID: "computer-test-b", WarmnessClass: "test", MachineMemSizeMib: 8192},
		State:  StateRunning, done: make(chan struct{}),
	}
	if api != nil {
		api.t = t
		serveUnix(t, mgr.apiSocketPath("vm-b"), api)
	}
	return mgr
}

// 6, 12: the client decodes Firecracker's statistics with its names and
// units; an absent optional field stays absent, not 0.
func TestBalloonStatisticsDecodesFakeSocket(t *testing.T) {
	api := &fakeBalloonAPI{interval: 5, stats: fakeBalloonStatsJSON}
	mgr := runningBalloonVM(t, api, nil)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	st, err := newFCAPIClient(mgr.apiSocketPath("vm-b")).BalloonStatistics(ctx)
	if err != nil {
		t.Fatalf("BalloonStatistics: %v", err)
	}
	checks := map[string]struct {
		got  *uint64
		want uint64
	}{
		"available_memory": {st.AvailableMemory, 6012954214},
		"total_memory":     {st.TotalMemory, 8589934592},
		"free_memory":      {st.FreeMemory, 1073741824},
		"disk_caches":      {st.DiskCaches, 4831838208},
		"major_faults":     {st.MajorFaults, 1200},
		"minor_faults":     {st.MinorFaults, 3400000},
		"swap_in":          {st.SwapIn, 0},
		"swap_out":         {st.SwapOut, 0},
	}
	for name, c := range checks {
		if c.got == nil || *c.got != c.want {
			t.Fatalf("%s = %v, want %d", name, c.got, c.want)
		}
	}
	if st.TargetPages != 0 || st.ActualPages != 0 {
		t.Fatalf("pages = %d/%d, want 0/0", st.TargetPages, st.ActualPages)
	}

	api.mu.Lock()
	api.stats = `{"target_pages":0,"actual_pages":0,"target_mib":0,"actual_mib":0}`
	api.mu.Unlock()
	st, err = newFCAPIClient(mgr.apiSocketPath("vm-b")).BalloonStatistics(ctx)
	if err != nil {
		t.Fatalf("BalloonStatistics (no guest report yet): %v", err)
	}
	if st.AvailableMemory != nil || st.TotalMemory != nil || st.DiskCaches != nil {
		t.Fatalf("absent fields decoded as values: %+v", st)
	}
}

// 5, 11, 13: the manager reading carries the device's own interval and the
// statistics, and talks GET only.
func TestGuestMemoryStatsReading(t *testing.T) {
	api := &fakeBalloonAPI{interval: 5, stats: fakeBalloonStatsJSON}
	mgr := runningBalloonVM(t, api, func(c *ManagerConfig) { c.BalloonStatsOnly = []string{"class:test"} })
	st, err := mgr.GuestMemoryStats("vm-b")
	if err != nil {
		t.Fatalf("GuestMemoryStats: %v", err)
	}
	if st.VMID != "vm-b" || st.StatsPollingIntervalS != 5 || st.ObservedAt.IsZero() {
		t.Fatalf("reading = %+v", st)
	}
	if st.Stats.AvailableMemory == nil || *st.Stats.AvailableMemory != 6012954214 {
		t.Fatalf("available_memory = %v", st.Stats.AvailableMemory)
	}
	calls := api.callLog()
	if len(calls) != 2 || calls[0] != "GET /balloon" || calls[1] != "GET /balloon/statistics" {
		t.Fatalf("calls = %v, want GET /balloon then GET /balloon/statistics", calls)
	}
}

func statsReason(t *testing.T, err error) string {
	t.Helper()
	var se *GuestMemoryStatsError
	if !errors.As(err, &se) {
		t.Fatalf("error %v (%T) is not a GuestMemoryStatsError", err, err)
	}
	return se.Reason
}

// 5: a guest restored from a snapshot taken without stats keeps its saved
// device. vmctl now selects it, but the device says interval 0: the reading
// is "stats disabled", never zeros, and statistics are not even asked for.
func TestGuestMemoryStatsRestoredWithoutStatsReportsUnavailable(t *testing.T) {
	api := &fakeBalloonAPI{interval: 0}
	mgr := runningBalloonVM(t, api, func(c *ManagerConfig) {
		c.BalloonStatsOnly = []string{"vm-b"}
		c.BalloonStatsPollingIntervalS = 5
	})
	mgr.vms["vm-b"].Config.BootKind = "snapshot_resume"
	st, err := mgr.GuestMemoryStats("vm-b")
	if err == nil {
		t.Fatalf("restored guest without stats gave a reading: %+v", st)
	}
	if got := statsReason(t, err); got != GuestMemoryStatsReasonDisabled {
		t.Fatalf("reason = %q, want %q (%v)", got, GuestMemoryStatsReasonDisabled, err)
	}
	if st.Stats.AvailableMemory != nil || st.Stats.TotalMemory != nil {
		t.Fatalf("numbers beside a reason: %+v", st)
	}
	for _, c := range api.callLog() {
		if c == "GET /balloon/statistics" {
			t.Fatalf("statistics read on a device with stats off: %v", api.callLog())
		}
	}
}

// 6, 7, 8, 9: every failure is a reason.
func TestGuestMemoryStatsReasons(t *testing.T) {
	t.Run("not running", func(t *testing.T) {
		mgr := runningBalloonVM(t, nil, nil)
		_, err := mgr.GuestMemoryStats("vm-missing")
		if got := statsReason(t, err); got != GuestMemoryStatsReasonNotRunning {
			t.Fatalf("reason = %q (%v)", got, err)
		}
	})
	t.Run("no api socket", func(t *testing.T) {
		mgr := runningBalloonVM(t, nil, nil)
		_, err := mgr.GuestMemoryStats("vm-b")
		if got := statsReason(t, err); got != GuestMemoryStatsReasonNoAPISocket {
			t.Fatalf("reason = %q (%v)", got, err)
		}
	})
	t.Run("no balloon device", func(t *testing.T) {
		mgr := runningBalloonVM(t, &fakeBalloonAPI{noDevice: true}, nil)
		_, err := mgr.GuestMemoryStats("vm-b")
		if got := statsReason(t, err); got != GuestMemoryStatsReasonNoBalloon {
			t.Fatalf("reason = %q (%v)", got, err)
		}
	})
	t.Run("statistics refused", func(t *testing.T) {
		mgr := runningBalloonVM(t, &fakeBalloonAPI{interval: 5}, nil)
		_, err := mgr.GuestMemoryStats("vm-b")
		if got := statsReason(t, err); got != GuestMemoryStatsReasonDisabled {
			t.Fatalf("reason = %q (%v)", got, err)
		}
	})
	t.Run("guest has not reported", func(t *testing.T) {
		mgr := runningBalloonVM(t, &fakeBalloonAPI{interval: 5, stats: `{"target_pages":0,"actual_pages":0,"target_mib":0,"actual_mib":0}`}, nil)
		st, err := mgr.GuestMemoryStats("vm-b")
		if got := statsReason(t, err); got != GuestMemoryStatsReasonNotReported {
			t.Fatalf("reason = %q (%v, %+v)", got, err, st)
		}
	})
	t.Run("busy", func(t *testing.T) {
		mgr := runningBalloonVM(t, &fakeBalloonAPI{interval: 5, stats: fakeBalloonStatsJSON}, nil)
		unlock, ok := mgr.tryLockVMOperation("vm-b")
		if !ok {
			t.Fatal("could not hold the VM operation lock")
		}
		defer unlock()
		start := time.Now()
		_, err := mgr.GuestMemoryStats("vm-b")
		if got := statsReason(t, err); got != GuestMemoryStatsReasonBusy {
			t.Fatalf("reason = %q (%v)", got, err)
		}
		if time.Since(start) > 500*time.Millisecond {
			t.Fatalf("busy read waited %v; it must not wait", time.Since(start))
		}
	})
	t.Run("timeout", func(t *testing.T) {
		old := guestMemoryStatsTimeout
		guestMemoryStatsTimeout = 100 * time.Millisecond
		defer func() { guestMemoryStatsTimeout = old }()
		mgr := runningBalloonVM(t, &fakeBalloonAPI{interval: 5, stats: fakeBalloonStatsJSON, delay: 5 * time.Second}, nil)
		start := time.Now()
		_, err := mgr.GuestMemoryStats("vm-b")
		if got := statsReason(t, err); got != GuestMemoryStatsReasonTimeout {
			t.Fatalf("reason = %q (%v)", got, err)
		}
		if el := time.Since(start); el > 2*time.Second {
			t.Fatalf("stuck VMM held the read %v", el)
		}
	})
}
