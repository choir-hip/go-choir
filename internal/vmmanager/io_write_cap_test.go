package vmmanager

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testMountinfo = `22 1 0:21 /@ / rw,relatime shared:1 - btrfs /dev/md127 rw,ssd
23 22 0:5 / /dev rw shared:2 - devtmpfs devtmpfs rw
24 22 0:30 /@var /var/lib/go-choir rw - btrfs /dev/md127 rw
25 22 0:31 / /var/lib/go\040space\040dir rw - tmpfs tmpfs rw
`

func fakeRdev(p string) (uint64, bool) {
	if p == "/dev/md127" {
		return 9<<8 | 127, true // major 9, minor 127
	}
	return 0, false
}

func TestBlockDevOfMountPicksLongestPrefixAndSplitsRdev(t *testing.T) {
	for dir, want := range map[string]string{
		"/var/lib/go-choir/vm-state": "9:127",
		"/var/lib/go-choirx":         "9:127", // not under /var/lib/go-choir: falls back to /
		"/home/x":                    "9:127",
		"/var/lib/go space dir/a":    "", // tmpfs has no device node
	} {
		got := blockDevOfMount(bufio.NewScanner(strings.NewReader(testMountinfo)), dir, fakeRdev)
		if got != want {
			t.Errorf("blockDevOfMount(%q) = %q, want %q", dir, got, want)
		}
	}
	// A minor above 255 lives in the high bits of the 64-bit dev_t: 259:300.
	rdev := uint64(259)<<8 | uint64(300&0xff) | uint64(300&^0xff)<<12
	if got := blockDevOfMount(bufio.NewScanner(strings.NewReader(testMountinfo)), "/", func(string) (uint64, bool) { return rdev, true }); got != "259:300" {
		t.Errorf("minor split = %q", got)
	}
}

func TestIOMaxLine(t *testing.T) {
	if got := ioMaxLine("9:127", 400); got != "9:127 wbps=419430400" {
		t.Fatalf("ioMaxLine = %q", got)
	}
}

// The bound lands in the guest's leaf as a write-only io.max line, and a
// missing device or a zero rate writes nothing.
func TestApplyLeafWriteCap(t *testing.T) {
	mgr, _ := jailedManager(t, t.TempDir())
	mgr.cfg.Jailer.IOWriteMiBps = 400
	mgr.cfg.Jailer.IOMaxDevice = "9:127"
	leaf := filepath.Join(mgr.cgroupRoot, "system.slice", "system-choirvms.slice", "vms", "vm-a")
	if err := os.MkdirAll(leaf, 0o755); err != nil {
		t.Fatal(err)
	}
	mgr.applyLeafWriteCap("vm-a")
	data, err := os.ReadFile(filepath.Join(leaf, "io.max"))
	if err != nil || string(data) != "9:127 wbps=419430400" {
		t.Fatalf("io.max = %q, %v", data, err)
	}

	mgr2, _ := jailedManager(t, t.TempDir())
	mgr2.cfg.Jailer.IOWriteMiBps = 0
	mgr2.cfg.Jailer.IOMaxDevice = "9:127"
	leaf2 := filepath.Join(mgr2.cgroupRoot, "system.slice", "system-choirvms.slice", "vms", "vm-a")
	_ = os.MkdirAll(leaf2, 0o755)
	mgr2.applyLeafWriteCap("vm-a")
	if _, err := os.Stat(filepath.Join(leaf2, "io.max")); !os.IsNotExist(err) {
		t.Fatalf("a zero bound must write nothing: %v", err)
	}
}

func TestIOWriteConfigFromEnv(t *testing.T) {
	if got := defaultJailerConfig().IOWriteMiBps; got != DefaultJailerIOWriteMiBps || got <= 0 {
		t.Fatalf("default bound = %d", got)
	}
	t.Setenv("VMCTL_JAILER_IO_WRITE_MIBPS", "250")
	t.Setenv("VMCTL_JAILER_IO_MAX_DEVICE", "9:127")
	j := loadJailerConfigFromEnv("/x/firecracker")
	if j.IOWriteMiBps != 250 || j.IOMaxDevice != "9:127" {
		t.Fatalf("env = %d %q", j.IOWriteMiBps, j.IOMaxDevice)
	}
	t.Setenv("VMCTL_JAILER_IO_WRITE_MIBPS", "0")
	t.Setenv("VMCTL_JAILER_IO_MAX_DEVICE", "../x")
	j = loadJailerConfigFromEnv("/x/firecracker")
	if j.IOWriteMiBps != 0 || j.IOMaxDevice != "" {
		t.Fatalf("env off/invalid = %d %q", j.IOWriteMiBps, j.IOMaxDevice)
	}
}
