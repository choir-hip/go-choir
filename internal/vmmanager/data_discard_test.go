package vmmanager

import (
	"os"
	"path/filepath"
	"testing"
)

// Failure modes this guards (docs/problems/guest-data-image-never-trims-
// 2026-10-20.md):
//   - "discard" sent to a Firecracker older than 1.17.0 (it denies unknown
//     fields; every boot of the computer fails);
//   - discard on the Async io_engine, or on the read-only store drive or the
//     credential disk (Firecracker refuses, or pointless);
//   - the gate on by accident (every cold boot changes shape and launch hash);
//   - the jailer rewrite dropping the field.

func driveOf(t *testing.T, fcConfig map[string]interface{}, id string) map[string]interface{} {
	t.Helper()
	drives, _ := fcConfig["drives"].([]map[string]interface{})
	for _, d := range drives {
		if d["drive_id"] == id {
			return d
		}
	}
	t.Fatalf("drive %q not in config: %#v", id, fcConfig["drives"])
	return nil
}

func TestBuildFirecrackerConfig_DataDriveHasNoDiscardByDefault(t *testing.T) {
	mgr := balloonTestManager(t, nil)
	data := driveOf(t, mgr.buildFirecrackerConfig(balloonTestVMConfig(), 9001), "data")
	for _, key := range []string{"discard", "io_engine"} {
		if _, ok := data[key]; ok {
			t.Fatalf("data drive carries %q with the gate off: %#v", key, data)
		}
	}
}

func TestBuildFirecrackerConfig_DataDiscardOnlyOnDataDriveWithSyncEngine(t *testing.T) {
	mgr := balloonTestManager(t, func(c *ManagerConfig) { c.DataDiscard = true })
	vm := balloonTestVMConfig()
	vm.StoreDiskPath = "/opt/go-choir/guest/store.erofs"
	vm.credentialDiskPath = "/var/lib/go-choir/vms/vm-balloon-1/cred.img"
	fc := mgr.buildFirecrackerConfig(vm, 9001)

	data := driveOf(t, fc, "data")
	if data["discard"] != true {
		t.Fatalf("data drive discard = %v, want true: %#v", data["discard"], data)
	}
	if data["io_engine"] != "Sync" {
		t.Fatalf("data drive io_engine = %v, want Sync (Async does not support discard)", data["io_engine"])
	}
	if data["is_read_only"] != false {
		t.Fatalf("data drive must be writable for discard: %#v", data)
	}
	for _, id := range []string{"store", "credential"} {
		d := driveOf(t, fc, id)
		for _, key := range []string{"discard", "io_engine"} {
			if _, ok := d[key]; ok {
				t.Fatalf("%s drive carries %q: %#v", id, key, d)
			}
		}
	}
}

func TestFirecrackerSupportsBlockDiscard(t *testing.T) {
	cases := []struct {
		banner string
		want   bool
	}{
		{"Firecracker v1.16.1", false},
		{"Firecracker v1.16.2", false},
		{"Firecracker v1.15.1", false},
		{"Firecracker v0.25.2", false},
		{"Firecracker v1.17.0", true},
		{"Firecracker v1.17.3", true},
		{"Firecracker v1.18.0", true},
		{"Firecracker v2.0.0", true},
		{"Firecracker v1.100.0", true},
		{"Firecracker v1.17.0-dev", true},
		{"v1.17.0", true},
		{"", false},
		{"Firecracker", false},
		{"Firecracker vX.Y.Z", false},
		{"Firecracker v1.17", false},
	}
	for _, tc := range cases {
		t.Run(tc.banner, func(t *testing.T) {
			if got := firecrackerSupportsBlockDiscard(tc.banner); got != tc.want {
				t.Fatalf("firecrackerSupportsBlockDiscard(%q) = %v, want %v", tc.banner, got, tc.want)
			}
		})
	}
}

func fakeFirecracker(t *testing.T, banner string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "firecracker")
	script := "#!/bin/sh\necho '" + banner + "'\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadConfigFromEnv_DataDiscardRespectsVersionGate(t *testing.T) {
	cases := []struct {
		name   string
		flag   string
		banner string
		want   bool
	}{
		{"flag unset, new binary", "", "Firecracker v1.17.0", false},
		{"flag off, new binary", "0", "Firecracker v1.17.0", false},
		{"flag on, pinned 1.16.2 refuses", "1", "Firecracker v1.16.2", false},
		{"flag on, new binary", "on", "Firecracker v1.17.0", true},
		{"flag on, newer binary", "true", "Firecracker v1.18.1", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("VM_FIRECRACKER_BIN", fakeFirecracker(t, tc.banner))
			t.Setenv("VM_DATA_DISCARD", tc.flag)
			if got := LoadConfigFromEnv().DataDiscard; got != tc.want {
				t.Fatalf("DataDiscard = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestLoadConfigFromEnv_DataDiscardOnUnreadableBinaryStaysOff(t *testing.T) {
	t.Setenv("VM_FIRECRACKER_BIN", filepath.Join(t.TempDir(), "missing"))
	t.Setenv("VM_DATA_DISCARD", "1")
	if LoadConfigFromEnv().DataDiscard {
		t.Fatal("DataDiscard on with a binary whose version cannot be read")
	}
}

func TestRewriteFirecrackerConfigForJail_KeepsDataDiscard(t *testing.T) {
	mgr := balloonTestManager(t, func(c *ManagerConfig) { c.DataDiscard = true })
	jail, _, err := rewriteFirecrackerConfigForJail(mgr.buildFirecrackerConfig(balloonTestVMConfig(), 9001))
	if err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	var data map[string]interface{}
	drives, _ := jail["drives"].([]interface{})
	for _, raw := range drives {
		if d, _ := raw.(map[string]interface{}); d != nil && d["drive_id"] == "data" {
			data = d
		}
	}
	if data == nil {
		t.Fatalf("no data drive in jail config: %#v", jail["drives"])
	}
	if data["discard"] != true || data["io_engine"] != "Sync" {
		t.Fatalf("jail rewrite dropped discard/io_engine: %#v", data)
	}
}
