//go:build linux

package capsule

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// TestExecutorInheritedBrokerListenerEndToEnd is opt-in because it needs root,
// cgroup v2, overlayfs, namespaces, Landlock, seccomp, and the immutable broker.
func TestExecutorInheritedBrokerListenerEndToEnd(t *testing.T) {
	if os.Getenv("CHOIR_CAPSULE_INTEGRATION") != "1" {
		t.Skip("set CHOIR_CAPSULE_INTEGRATION=1 on the designated Linux harness")
	}
	if os.Geteuid() != 0 {
		t.Fatal("capsule integration requires root")
	}
	brokerPath := filepath.Clean(os.Getenv("CHOIR_CAPSULE_BROKER"))
	if info, err := os.Stat(brokerPath); err != nil || !info.Mode().IsRegular() {
		t.Fatalf("immutable capsule broker unavailable: %s", brokerPath)
	}
	sourceDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(sourceDir, "README"), []byte("capsule integration source\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.email", "capsule-integration@choir.invalid"},
		{"config", "user.name", "Capsule Integration"},
		{"add", "README"},
		{"commit", "-q", "-m", "freeze integration source"},
	} {
		command := exec.Command("git", args...)
		command.Dir = sourceDir
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("freeze integration source: %v: %s", err, output)
		}
	}
	lowerDir := t.TempDir()
	for _, path := range []string{"dev/pts", "proc"} {
		if err := os.MkdirAll(filepath.Join(lowerDir, path), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	stateDir, err := os.MkdirTemp("/tmp", "choir-capsule-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(stateDir) })
	executor := NewExecutorWithSource(stateDir, lowerDir, sourceDir, brokerPath, 512<<20)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	capsuleID := "g1-listener-" + strconv.Itoa(os.Getpid())
	caps, err := executor.Spawn(ctx, SpawnSpec{
		CapsuleID: capsuleID, OwnerRunID: "g1-listener-integration",
		MemoryMax: 256 << 20, CpuQuota: 50000, CpuPeriod: 100000, PidsMax: 128,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = executor.ForceDestroy(context.Background(), capsuleID) })
	if caps.State != StateActive || caps.PID <= 0 || caps.listener == nil || caps.broker == nil {
		t.Fatalf("capsule broker did not become active through inherited listener: %+v", caps)
	}
	capability, err := executor.MintCapability("g1-listener-reconnect", RoleCoSuper, capsuleID, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	socketPath := filepath.Join(caps.MergedDir, "run", "capsule", "broker.sock")
	reconnected := NewBrokerClient(socketPath, executor.publicKey)
	if err := reconnected.Connect(ctx); err != nil {
		t.Fatalf("reconnect inherited broker listener: %v", err)
	}
	if _, err := reconnected.Stat(ctx, capability, "."); err != nil {
		_ = reconnected.Close()
		t.Fatalf("authenticated reconnect readiness: %v", err)
	}
	_ = reconnected.Close()
	detachedPath := filepath.Join(caps.MergedDir, "var/lib/artifact/release/detached-writer")
	if err := os.MkdirAll(filepath.Dir(detachedPath), 0o700); err != nil {
		t.Fatal(err)
	}
	writer := exec.Command("sh", "-c", `while :; do printf x >> "$DETACHED_PATH"; done`)
	writer.Env = append(os.Environ(), "DETACHED_PATH="+detachedPath)
	if err := writer.Start(); err != nil {
		t.Fatalf("start detached writer: %v", err)
	}
	t.Cleanup(func() { _ = writer.Process.Kill() })
	cgroup, ok := caps.Cgroup.(*CgroupManager)
	if !ok {
		t.Fatal("production capsule cgroup manager unavailable")
	}
	if err := cgroup.AddPID(writer.Process.Pid); err != nil {
		t.Fatalf("join detached writer to capsule cgroup: %v", err)
	}
	var before os.FileInfo
	deadline := time.Now().Add(time.Second)
	for before == nil || before.Size() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("detached writer did not produce proof file")
		}
		before, _ = os.Stat(detachedPath)
		time.Sleep(time.Millisecond)
	}
	if err := caps.Quiesce(ctx); err != nil {
		t.Fatalf("freeze capsule cgroup: %v", err)
	}
	before, err = os.Stat(detachedPath)
	if err != nil || before.Size() == 0 {
		t.Fatalf("detached writer did not produce proof file: info=%v err=%v", before, err)
	}
	time.Sleep(50 * time.Millisecond)
	after, err := os.Stat(detachedPath)
	if err != nil {
		t.Fatal(err)
	}
	if before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
		t.Fatalf("detached writer mutated after freeze: before=%+v after=%+v", before, after)
	}
	if err := caps.Thaw(ctx); err != nil {
		t.Fatalf("thaw capsule cgroup: %v", err)
	}
	_ = writer.Process.Kill()
	_ = writer.Wait()
	if err := executor.ForceDestroy(ctx, capsuleID); err != nil {
		t.Fatalf("destroy integrated capsule: %v", err)
	}
	for name, path := range map[string]string{
		"socket": socketPath,
		"cgroup": filepath.Join("/sys/fs/cgroup/capsule", capsuleID),
		"state":  filepath.Join(stateDir, capsuleID),
	} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatalf("%s leaked after capsule destroy: path=%s err=%v", name, path, err)
		}
	}
	if caps.listener != nil {
		t.Fatal("parent listener descriptor remained live after destroy")
	}
	select {
	case <-caps.processDone:
	default:
		t.Fatal("broker launcher remained live after destroy")
	}
}
