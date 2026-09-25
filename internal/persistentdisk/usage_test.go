package persistentdisk

import (
	"os"
	"path/filepath"
	"testing"
)
func TestCriticalLowAvail(t *testing.T) {
	usage := Usage{TotalBytes: DefaultCapBytes, UsedBytes: DefaultCapBytes - (256 * 1024 * 1024), AvailBytes: 256 * 1024 * 1024}
	if !Critical(usage) {
		t.Fatal("expected critical for 256 MiB free")
	}
}

func TestStatusFromGuestUsageUsesTotalAsCap(t *testing.T) {
	usage := Usage{TotalBytes: 16 * 1024 * 1024 * 1024, UsedBytes: 2 * 1024 * 1024 * 1024, AvailBytes: 14 * 1024 * 1024 * 1024}
	status := StatusFromGuestUsage(usage)
	if status.CapBytes != usage.TotalBytes {
		t.Fatalf("cap = %d, want %d", status.CapBytes, usage.TotalBytes)
	}
	if status.Source != "guest" {
		t.Fatalf("source = %q, want guest", status.Source)
	}
}

func TestStatfsMissingDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "missing")
	if _, err := Statfs(dir); err == nil {
		t.Fatal("expected error for missing directory")
	}
	_ = os.Mkdir(dir, 0o755)
	if _, err := Statfs(dir); err != nil {
		t.Fatalf("Statfs existing dir: %v", err)
	}
}
