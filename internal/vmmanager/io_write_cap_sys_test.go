package vmmanager

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A stacked device is bounded at the whole disks under it: md127 with two
// partition members resolves to the two NVMe disks; a plain disk to itself.
func TestCapDevicesResolvesMirrorMembersToWholeDisks(t *testing.T) {
	sys := t.TempDir()
	write := func(p, content string) {
		t.Helper()
		full := filepath.Join(sys, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	link := func(from, to string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(filepath.Join(sys, from)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(to, filepath.Join(sys, from)); err != nil {
			t.Fatal(err)
		}
	}
	// devices/ holds the real nodes; class/block/<name> links to them.
	write("devices/virtual/block/md127/dev", "9:127\n")
	disks := map[string]string{"nvme0n1": "259:0", "nvme1n1": "259:1"}
	for disk, dev := range disks {
		write("devices/pci/"+disk+"/dev", dev+"\n")
		write("devices/pci/"+disk+"/"+disk+"p2/dev", "259:9\n")
		write("devices/pci/"+disk+"/"+disk+"p2/partition", "2\n")
		link("class/block/"+disk+"p2", filepath.Join(sys, "devices/pci", disk, disk+"p2"))
		link("class/block/"+disk, filepath.Join(sys, "devices/pci", disk))
		link("devices/virtual/block/md127/slaves/"+disk+"p2", filepath.Join(sys, "devices/pci", disk, disk+"p2"))
	}
	link("class/block/md127", filepath.Join(sys, "devices/virtual/block/md127"))
	write("dev/block/9:127/uevent", "MAJOR=9\nMINOR=127\nDEVNAME=md127\n")
	write("dev/block/259:0/uevent", "DEVNAME=nvme0n1\n")

	got := capDevices(sys, "9:127")
	if len(got) != 2 || got[0] == got[1] || strings.Join(got, ",") != "259:0,259:1" && strings.Join(got, ",") != "259:1,259:0" {
		t.Fatalf("mirror members = %v, want the two whole disks 259:0 and 259:1", got)
	}
	if got := capDevices(sys, "259:0"); strings.Join(got, ",") != "259:0" {
		t.Fatalf("plain disk = %v", got)
	}
	if got := capDevices(sys, "8:99"); len(got) != 0 {
		t.Fatalf("unknown device = %v", got)
	}
}
