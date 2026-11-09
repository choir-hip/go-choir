package toolresidency

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestResidentCountsReadFilesAndSkipsSymlinks(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("mincore is Linux only")
	}
	root := t.TempDir()
	dir := LayerDir(root, "toolchain")
	if err := os.MkdirAll(filepath.Join(dir, "aaa-go", "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "aaa-go", "bin", "tool")
	if err := os.WriteFile(p, make([]byte, 1<<20), 0o755); err != nil {
		t.Fatal(err)
	}
	_ = os.Symlink("/etc/hostname", filepath.Join(dir, "link"))
	if _, err := os.ReadFile(p); err != nil {
		t.Fatal(err)
	}
	res, total, err := Resident(context.Background(), dir)
	if err != nil || total != 1<<20 {
		t.Fatalf("total %d err %v", total, err)
	}
	if res <= 0 {
		t.Fatalf("resident_bytes = %d after reading the file, want > 0", res)
	}
	if got := Layers(root); len(got) != 1 || got[0] != "toolchain" {
		t.Fatalf("layers %v", got)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := Resident(ctx, dir); err == nil {
		t.Fatal("cancelled walk reported no error")
	}
}

func TestStatusKeepsLastEviction(t *testing.T) {
	dir := t.TempDir()
	if s := ReadStatus(dir, "office"); !s.LastEvictedAt.IsZero() {
		t.Fatalf("absent status = %+v", s)
	}
	t0 := time.Date(2026, 11, 9, 10, 0, 0, 0, time.UTC)
	if err := RecordRun(dir, "office", t0, true); err != nil {
		t.Fatal(err)
	}
	if err := RecordRun(dir, "office", t0.Add(5*time.Minute), false); err != nil {
		t.Fatal(err)
	}
	s := ReadStatus(dir, "office")
	if !s.LastEvictedAt.Equal(t0) || !s.LastRunAt.Equal(t0.Add(5*time.Minute)) {
		t.Fatalf("status = %+v", s)
	}
}
