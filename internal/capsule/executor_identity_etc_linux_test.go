//go:build linux

package capsule

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteCapsuleIdentityEtc(t *testing.T) {
	upper := t.TempDir()
	if err := writeCapsuleIdentityEtc(upper); err != nil {
		t.Fatal(err)
	}
	hosts := filepath.Join(upper, "etc", "hosts")
	data, err := os.ReadFile(hosts)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "127.0.0.1 localhost\n::1 localhost\n" {
		t.Fatalf("hosts=%q", data)
	}
	info, err := os.Lstat(hosts)
	if err != nil || !info.Mode().IsRegular() {
		t.Fatalf("hosts mode=%v err=%v", info, err)
	}
}

func TestWriteCapsuleIdentityEtcHidesLowerHostsSymlink(t *testing.T) {
	lower := t.TempDir()
	upper := t.TempDir()
	work := t.TempDir()
	merged := t.TempDir()
	storeLike := t.TempDir()
	target := filepath.Join(storeLike, "hosts")
	if err := os.WriteFile(target, []byte("from-nix-store\n"), 0o444); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(lower, "etc"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(lower, "etc", "hosts")); err != nil {
		t.Fatal(err)
	}
	if err := MountOverlayFS(merged, upper, work, lower); err != nil {
		t.Skipf("overlay mount unavailable: %v", err)
	}
	t.Cleanup(func() { _ = UnmountOverlayFS(merged) })
	if err := writeCapsuleIdentityEtc(upper); err != nil {
		t.Fatal(err)
	}
	mergedHosts := filepath.Join(merged, "etc", "hosts")
	info, err := os.Lstat(mergedHosts)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Fatal("merged hosts remained a lower symlink into a read-only store")
	}
	data, err := os.ReadFile(mergedHosts)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "127.0.0.1 localhost\n::1 localhost\n" {
		t.Fatalf("merged hosts=%q", data)
	}
	storeData, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(storeData) != "from-nix-store\n" {
		t.Fatalf("lower store hosts mutated: %q", storeData)
	}
}

func TestBrokerReadinessTimeoutErrorIncludesLastProbe(t *testing.T) {
	err := brokerReadinessTimeoutError(os.ErrPermission)
	if err == nil || !strings.Contains(err.Error(), "capsule broker readiness timed out") || !strings.Contains(err.Error(), os.ErrPermission.Error()) {
		t.Fatalf("timeout error=%v", err)
	}
}

func TestPrepareCapsuleRootMasksGuestProc(t *testing.T) {
	lower := t.TempDir()
	upper := t.TempDir()
	work := t.TempDir()
	merged := t.TempDir()
	if err := os.MkdirAll(filepath.Join(lower, "proc", "guest"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(lower, "proc", "guest", "cmdline"), []byte("guest-proc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := MountOverlayFS(merged, upper, work, lower); err != nil {
		t.Skipf("overlay mount unavailable: %v", err)
	}
	t.Cleanup(func() { _ = UnmountOverlayFS(merged) })
	if err := prepareCapsuleRoot(merged, upper); err != nil {
		t.Skipf("prepareCapsuleRoot unavailable: %v", err)
	}
	t.Cleanup(func() { _ = unmountCapsuleRoot(merged) })
	if _, err := os.Stat(filepath.Join(merged, "proc", "guest", "cmdline")); !os.IsNotExist(err) {
		t.Fatalf("guest proc leaked through capsule root: err=%v", err)
	}
}
