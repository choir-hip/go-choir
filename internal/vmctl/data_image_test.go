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
	f, err := os.OpenFile(dataImg, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatalf("create data.img: %v", err)
	}
	if err := f.Truncate(1024 * 1024); err != nil {
		_ = f.Close()
		t.Fatalf("truncate data.img: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close data.img: %v", err)
	}
	if err := os.WriteFile(filepath.Join(vmDir, "allocation-marker"), make([]byte, 8192), 0o600); err != nil {
		t.Fatalf("write allocation marker: %v", err)
	}

	stats, ok := LookupDataImageStats(root, vmID)
	if !ok {
		t.Fatal("expected data image stats")
	}
	if stats.CapBytes != 1024*1024 {
		t.Fatalf("cap_bytes = %d, want %d", stats.CapBytes, 1024*1024)
	}
	if stats.FileBytes != stats.StateDirBytes {
		t.Fatalf("file_bytes = %d, want state_dir_bytes %d", stats.FileBytes, stats.StateDirBytes)
	}
	if stats.FileBytes == stats.CapBytes {
		t.Fatalf("file_bytes = %d should report host allocation/state-dir usage, not virtual cap_bytes", stats.FileBytes)
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
	dataImg := filepath.Join(vmDir, dataImageFileName)
	f, err := os.OpenFile(dataImg, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatalf("create data.img: %v", err)
	}
	if err := f.Truncate(4 * 1024 * 1024); err != nil {
		_ = f.Close()
		t.Fatalf("truncate data.img: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close data.img: %v", err)
	}

	reg := NewOwnershipRegistry("http://127.0.0.1:8085")
	reg.SetPressureReclaimConfig(PressureReclaimConfig{StateDir: root})

	stats, ok := reg.DataImageStatsForVM(vmID)
	if !ok {
		t.Fatal("expected registry data image stats")
	}
	if stats.CapBytes != 4*1024*1024 {
		t.Fatalf("cap_bytes = %d, want %d", stats.CapBytes, 4*1024*1024)
	}
	if stats.FileBytes != stats.StateDirBytes {
		t.Fatalf("file_bytes = %d, want state_dir_bytes %d", stats.FileBytes, stats.StateDirBytes)
	}
}
