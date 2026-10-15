package agentcore

import (
	"os"
	"path/filepath"
	"testing"
)

// The memory hotplug probe reads a lease from MemTotal. Failure modes: kB
// read as bytes, a missing MemTotal reported as zero memory, a malformed
// line aborting the parse.
func TestParseGuestMeminfo(t *testing.T) {
	got := parseGuestMeminfo("MemTotal:        4031484 kB\nMemFree:  100 kB\nbogus line\nMemAvailable:    3000000 kB\nHugePages_Total: 0\n")
	if got == nil || got.MemTotalBytes != 4031484<<10 || got.MemAvailableBytes != 3000000<<10 || got.MemFreeBytes != 100<<10 {
		t.Fatalf("parsed = %+v", got)
	}
	if parseGuestMeminfo("MemFree: 100 kB\n") != nil {
		t.Fatal("no MemTotal must yield nil")
	}
}

// virtio_mem load and bind, read from sysfs so the staging probe can tell a
// guest without the driver from one whose driver found no device. Failure
// modes: the driver's own files (bind, unbind, uevent, module) read as a
// bound device; a missing module read as loaded.
func TestGuestVirtioMemState(t *testing.T) {
	root := t.TempDir()
	if loaded, bound := guestVirtioMemState(root); loaded || bound {
		t.Fatalf("empty sysfs: loaded=%t bound=%t", loaded, bound)
	}
	mustMkdir(t, filepath.Join(root, "module", "virtio_mem"))
	driver := filepath.Join(root, "bus", "virtio", "drivers", "virtio_mem")
	mustMkdir(t, driver)
	for _, name := range []string{"bind", "unbind", "uevent", "module"} {
		if err := os.WriteFile(filepath.Join(driver, name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if loaded, bound := guestVirtioMemState(root); !loaded || bound {
		t.Fatalf("driver without a device: loaded=%t bound=%t", loaded, bound)
	}
	mustMkdir(t, filepath.Join(driver, "virtio5"))
	if loaded, bound := guestVirtioMemState(root); !loaded || !bound {
		t.Fatalf("bound device: loaded=%t bound=%t", loaded, bound)
	}
}

func mustMkdir(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
}
