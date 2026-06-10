package vmctl

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDataImageStats(t *testing.T) {
	root := t.TempDir()
	vmID := "vm-test-disk"
	vmDir := filepath.Join(root, vmID)
	if err := os.MkdirAll(vmDir, 0o755); err != nil {
		t.Fatalf("mkdir vm dir: %v", err)
	}
	dataImg := filepath.Join(vmDir, dataImageFileName)
	if err := os.WriteFile(dataImg, make([]byte, 1024), 0o600); err != nil {
		t.Fatalf("write data.img: %v", err)
	}

	stats, ok := LookupDataImageStats(root, vmID)
	if !ok {
		t.Fatal("expected data image stats")
	}
	if stats.FileBytes != 1024 {
		t.Fatalf("file_bytes = %d, want 1024", stats.FileBytes)
	}
	if stats.CapBytes != 1024 {
		t.Fatalf("cap_bytes = %d, want 1024", stats.CapBytes)
	}
	if stats.StateDirBytes < 1024 {
		t.Fatalf("state_dir_bytes = %d, want >= 1024", stats.StateDirBytes)
	}
}

func TestDataImageStatsMissing(t *testing.T) {
	if _, ok := LookupDataImageStats(t.TempDir(), "missing-vm"); ok {
		t.Fatal("expected missing data image to return false")
	}
}

func TestOwnershipRegistryDataImageStatsForVM(t *testing.T) {
	root := t.TempDir()
	vmID := "vm-registry-disk"
	vmDir := filepath.Join(root, vmID)
	if err := os.MkdirAll(vmDir, 0o755); err != nil {
		t.Fatalf("mkdir vm dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(vmDir, dataImageFileName), []byte("disk"), 0o600); err != nil {
		t.Fatalf("write data.img: %v", err)
	}

	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	reg.SetPressureReclaimConfig(PressureReclaimConfig{StateDir: root})

	stats, ok := reg.DataImageStatsForVM(vmID)
	if !ok {
		t.Fatal("expected registry data image stats")
	}
	if stats.FileBytes != 4 {
		t.Fatalf("file_bytes = %d, want 4", stats.FileBytes)
	}
}
