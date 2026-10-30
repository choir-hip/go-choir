package vmmanager

import (
	"encoding/json"
	"testing"
)

// Failure modes this guards (docs/archive/reports/lr-guest-memory-return-2026-10-14.md):
//   - the balloon appears with the gate off (every cold boot changes shape);
//   - a field name Firecracker 1.15.1 rejects (deny_unknown_fields) fails
//     every boot on the host;
//   - a non-zero amount_mib inflates the balloon and takes guest memory;
//   - stats polling turned on without being asked.

func balloonTestManager(t *testing.T, mutate func(*ManagerConfig)) *Manager {
	t.Helper()
	cfg := DefaultManagerConfig()
	cfg.StateDir = t.TempDir()
	cfg.KernelImagePath = "/opt/go-choir/guest/vmlinux"
	cfg.RootfsPath = "/opt/go-choir/guest/rootfs.ext4"
	if mutate != nil {
		mutate(&cfg)
	}
	return NewManager(cfg)
}

func balloonTestVMConfig() VMConfig {
	return VMConfig{
		VMID:              "vm-balloon-1",
		KernelImagePath:   "/opt/go-choir/guest/vmlinux",
		RootfsPath:        "/opt/go-choir/guest/rootfs.ext4",
		GuestPort:         8085,
		MachineCPUCount:   2,
		MachineMemSizeMib: 8192,
		Epoch:             1,
	}
}

func TestBuildFirecrackerConfig_NoBalloonByDefault(t *testing.T) {
	mgr := balloonTestManager(t, nil)
	fcConfig := mgr.buildFirecrackerConfig(balloonTestVMConfig(), 9001)
	if _, ok := fcConfig["balloon"]; ok {
		t.Fatalf("balloon present with the gate off: %#v", fcConfig["balloon"])
	}
}

func TestBuildFirecrackerConfig_BalloonFreePageReportingWhenEnabled(t *testing.T) {
	mgr := balloonTestManager(t, func(c *ManagerConfig) { c.BalloonFreePageReporting = true })
	fcConfig := mgr.buildFirecrackerConfig(balloonTestVMConfig(), 9001)

	raw, err := json.Marshal(fcConfig)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	balloonRaw, ok := decoded["balloon"]
	if !ok {
		t.Fatalf("balloon missing with the gate on: %s", raw)
	}
	var balloon map[string]any
	if err := json.Unmarshal(balloonRaw, &balloon); err != nil {
		t.Fatalf("unmarshal balloon: %v", err)
	}
	// Exactly the BalloonDeviceConfig fields of Firecracker v1.15.1
	// (src/vmm/src/vmm_config/balloon.rs); free_page_hinting is never sent.
	want := map[string]any{
		"amount_mib":               float64(0),
		"deflate_on_oom":           true,
		"stats_polling_interval_s": float64(0),
		"free_page_reporting":      true,
	}
	if len(balloon) != len(want) {
		t.Fatalf("balloon fields = %v, want exactly %v", balloon, want)
	}
	for k, v := range want {
		if balloon[k] != v {
			t.Fatalf("balloon[%q] = %#v, want %#v (full: %v)", k, balloon[k], v, balloon)
		}
	}
	// The other top-level sections are untouched.
	for _, key := range []string{"boot-source", "drives", "machine-config", "network-interfaces"} {
		if _, ok := decoded[key]; !ok {
			t.Fatalf("%s missing beside balloon", key)
		}
	}
}

// The interval applies only to computers VM_BALLOON_STATS_ONLY names
// (RES-10; balloon_stats_test.go): set alone it turns nothing on.
func TestBuildFirecrackerConfig_BalloonStatsInterval(t *testing.T) {
	for _, tc := range []struct {
		only []string
		want int
	}{{nil, 0}, {[]string{"vm-balloon-1"}, 5}} {
		mgr := balloonTestManager(t, func(c *ManagerConfig) {
			c.BalloonFreePageReporting = true
			c.BalloonStatsPollingIntervalS = 5
			c.BalloonStatsOnly = tc.only
		})
		fcConfig := mgr.buildFirecrackerConfig(balloonTestVMConfig(), 9001)
		balloon, ok := fcConfig["balloon"].(map[string]any)
		if !ok {
			t.Fatalf("balloon missing: %#v", fcConfig["balloon"])
		}
		if got := balloon["stats_polling_interval_s"]; got != tc.want {
			t.Fatalf("only=%v: stats_polling_interval_s = %#v, want %d", tc.only, got, tc.want)
		}
	}
}

func TestLoadConfigFromEnv_BalloonGate(t *testing.T) {
	cases := []struct {
		value string
		want  bool
	}{
		{"", false},
		{"0", false},
		{"false", false},
		{"off", false},
		{"garbage", false},
		{"1", true},
		{"true", true},
		{"on", true},
		{"YES", true},
	}
	for _, tc := range cases {
		t.Run("value="+tc.value, func(t *testing.T) {
			t.Setenv("VM_BALLOON_FREE_PAGE_REPORTING", tc.value)
			if got := LoadConfigFromEnv().BalloonFreePageReporting; got != tc.want {
				t.Fatalf("BalloonFreePageReporting = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestLoadConfigFromEnv_BalloonStatsInterval(t *testing.T) {
	cases := []struct {
		value string
		want  int
	}{
		{"", 0},
		{"5", 5},
		{"-1", 0},
		{"x", 0},
		{"65535", 65535},
		{"65536", 0}, // Firecracker's field is u16; out of range is refused, not wrapped.
	}
	for _, tc := range cases {
		t.Run("value="+tc.value, func(t *testing.T) {
			t.Setenv("VM_BALLOON_STATS_POLLING_INTERVAL_S", tc.value)
			if got := LoadConfigFromEnv().BalloonStatsPollingIntervalS; got != tc.want {
				t.Fatalf("BalloonStatsPollingIntervalS = %d, want %d", got, tc.want)
			}
		})
	}
}
