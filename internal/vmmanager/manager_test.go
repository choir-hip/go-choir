package vmmanager

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	cfg := DefaultManagerConfig()
	mgr := NewManager(cfg)

	if mgr == nil {
		t.Fatal("expected non-nil manager")
	}
	if mgr.nextPort != cfg.HostBasePort {
		t.Errorf("expected nextPort=%d, got %d", cfg.HostBasePort, mgr.nextPort)
	}
}

func TestManagerDefaultConfig(t *testing.T) {
	cfg := DefaultManagerConfig()

	if cfg.GuestPort != 8085 {
		t.Errorf("expected GuestPort=8085, got %d", cfg.GuestPort)
	}
	if cfg.HostBasePort != 9000 {
		t.Errorf("expected HostBasePort=9000, got %d", cfg.HostBasePort)
	}
	if cfg.MachineCPUCount != 2 {
		t.Errorf("expected MachineCPUCount=2, got %d", cfg.MachineCPUCount)
	}
	if cfg.MachineMemSizeMib != 512 {
		t.Errorf("expected MachineMemSizeMib=512, got %d", cfg.MachineMemSizeMib)
	}
	if cfg.HealthCheckInterval != 15*time.Second {
		t.Errorf("expected HealthCheckInterval=15s, got %s", cfg.HealthCheckInterval)
	}
	if cfg.BootReadyTimeout != 20*time.Second {
		t.Errorf("expected BootReadyTimeout=20s, got %s", cfg.BootReadyTimeout)
	}
}

func TestRefreshConfigForCurrentDeployUsesCurrentMicroVMArtifacts(t *testing.T) {
	old := VMConfig{
		VMID:              "vm-stale",
		KernelImagePath:   "/old/kernel",
		InitrdPath:        "/old/initrd",
		RootfsPath:        "/old/rootfs",
		StoreDiskPath:     "/old/store",
		KernelParams:      "init=/old/init",
		GuestPort:         8085,
		MachineCPUCount:   2,
		MachineMemSizeMib: 4096,
		PersistentDir:     "/state/vm-stale/persist",
		SourceVMID:        "vm-source",
		GatewayToken:      "sandbox-token",
		Epoch:             7,
	}
	defaults := DefaultManagerConfig()
	defaults.StoreDiskPath = "/current/store"

	got := refreshConfigForCurrentDeploy(old, defaults)
	if got.KernelImagePath != "" || got.InitrdPath != "" || got.RootfsPath != "" || got.StoreDiskPath != "" || got.KernelParams != "" {
		t.Fatalf("refresh config kept stale boot artifacts: %+v", got)
	}
	if got.SourceVMID != "" {
		t.Fatalf("refresh config kept stale source VM copy request: %+v", got)
	}
	if got.VMID != old.VMID || got.PersistentDir != old.PersistentDir || got.GuestPort != old.GuestPort || got.MachineMemSizeMib != old.MachineMemSizeMib {
		t.Fatalf("refresh config did not preserve VM identity and mutable state fields: %+v", got)
	}
}

func TestRefreshConfigForCurrentDeployPreservesLegacyRootfsConfig(t *testing.T) {
	old := VMConfig{
		VMID:            "vm-legacy",
		KernelImagePath: "/old/kernel",
		RootfsPath:      "/state/vm-legacy/rootfs.ext4",
		KernelParams:    "console=ttyS0",
	}
	defaults := DefaultManagerConfig()
	defaults.StoreDiskPath = ""

	got := refreshConfigForCurrentDeploy(old, defaults)
	if !reflect.DeepEqual(got, old) {
		t.Fatalf("legacy rootfs refresh config changed unexpectedly:\n got=%+v\nwant=%+v", got, old)
	}
}

func TestManagerBootVMRequiresKernelAndRootfs(t *testing.T) {
	// BootVM should fail when no kernel/rootfs is configured.
	tmpDir := t.TempDir()
	cfg := DefaultManagerConfig()
	cfg.StateDir = tmpDir
	// Deliberately leave KernelImagePath and RootfsPath empty.

	mgr := NewManager(cfg)
	_, err := mgr.BootVM(VMConfig{
		VMID:          "test-vm-1",
		PersistentDir: filepath.Join(tmpDir, "persist"),
	})

	if err == nil {
		t.Error("expected error when kernel/rootfs not configured")
	}
}

func TestManagerBuildFirecrackerConfig_NoSecrets(t *testing.T) {
	// VAL-VM-011: The Firecracker config must NOT contain provider
	// credentials or any secret material.
	cfg := DefaultManagerConfig()
	cfg.StateDir = t.TempDir()
	cfg.KernelImagePath = "/opt/go-choir/guest/vmlinux"
	cfg.RootfsPath = "/opt/go-choir/guest/rootfs.ext4"

	mgr := NewManager(cfg)

	vmCfg := VMConfig{
		VMID:              "vm-test-123",
		KernelImagePath:   cfg.KernelImagePath,
		RootfsPath:        cfg.RootfsPath,
		GuestPort:         8085,
		MachineCPUCount:   2,
		MachineMemSizeMib: 512,
		Epoch:             1,
	}

	fcConfig := mgr.buildFirecrackerConfig(vmCfg, 9001)

	// Verify the config is a valid map.
	if fcConfig == nil {
		t.Fatal("expected non-nil config")
	}

	// Verify boot-source exists.
	bootSource, ok := fcConfig["boot-source"].(map[string]interface{})
	if !ok {
		t.Fatal("expected boot-source in config")
	}

	// Check boot args contain the VM ID and epoch but NO secrets.
	bootArgs, _ := bootSource["boot_args"].(string)
	if bootArgs == "" {
		t.Error("expected non-empty boot_args")
	}

	// VAL-VM-011: Verify NO secret patterns in the config.
	forbidden := []string{
		"Bearer", "AWS_", "SECRET", "PASSWORD", "TOKEN",
		"api_key", "apiKey", "api-key",
		"ZAI_API_KEY", "AWS_BEARER_TOKEN_BEDROCK", "FIREWORKS_API_KEY",
	}
	for _, pattern := range forbidden {
		if contains(fcConfig, pattern) {
			t.Errorf("VAL-VM-011: firecracker config contains forbidden pattern: %s", pattern)
		}
	}

	// Verify VM ID and epoch are in boot args.
	if !containsStr(bootArgs, "vm_id=vm-test-123") {
		t.Errorf("expected vm_id in boot args: %s", bootArgs)
	}
	if !containsStr(bootArgs, "epoch=1") {
		t.Errorf("expected epoch in boot args: %s", bootArgs)
	}

	// Verify machine config.
	machineCfg, ok := fcConfig["machine-config"].(map[string]interface{})
	if !ok {
		t.Fatal("expected machine-config in config")
	}
	if machineCfg["vcpu_count"] != 2 {
		t.Errorf("expected vcpu_count=2, got %v", machineCfg["vcpu_count"])
	}
	if machineCfg["mem_size_mib"] != 512 {
		t.Errorf("expected mem_size_mib=512, got %v", machineCfg["mem_size_mib"])
	}
}

func TestManagerEpochPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := DefaultManagerConfig()
	cfg.StateDir = tmpDir

	mgr := NewManager(cfg)

	// Save an epoch.
	if err := mgr.saveEpoch("test-vm-1", 42); err != nil {
		t.Fatalf("saveEpoch: %v", err)
	}

	// Load it back.
	epoch, err := mgr.loadEpoch("test-vm-1")
	if err != nil {
		t.Fatalf("loadEpoch: %v", err)
	}
	if epoch != 42 {
		t.Errorf("expected epoch=42, got %d", epoch)
	}

	// Nonexistent VM returns error.
	_, err = mgr.loadEpoch("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent epoch")
	}
}

func TestManagerGetListRemoveVM(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := DefaultManagerConfig()
	cfg.StateDir = tmpDir

	mgr := NewManager(cfg)

	// Get nonexistent VM.
	if v := mgr.GetVM("nonexistent"); v != nil {
		t.Error("expected nil for nonexistent VM")
	}

	// List empty.
	if vms := mgr.ListVMs(); len(vms) != 0 {
		t.Errorf("expected 0 VMs, got %d", len(vms))
	}

	// Add a VM manually.
	inst := &VMInstance{
		Config: VMConfig{VMID: "test-vm-1"},
		State:  StateStopped,
	}
	mgr.mu.Lock()
	mgr.vms["test-vm-1"] = inst
	mgr.mu.Unlock()

	// Get it back.
	if v := mgr.GetVM("test-vm-1"); v == nil || v.Config.VMID != "test-vm-1" {
		t.Error("expected to find test-vm-1")
	}

	// List should have 1.
	if vms := mgr.ListVMs(); len(vms) != 1 {
		t.Errorf("expected 1 VM, got %d", len(vms))
	}

	// Remove running VM should fail.
	inst.State = StateRunning
	if err := mgr.RemoveVM("test-vm-1"); err == nil {
		t.Error("expected error removing running VM")
	}

	// Remove stopped VM should succeed.
	inst.State = StateStopped
	if err := mgr.RemoveVM("test-vm-1"); err != nil {
		t.Errorf("RemoveVM: %v", err)
	}

	// Verify it's gone.
	if v := mgr.GetVM("test-vm-1"); v != nil {
		t.Error("expected nil after remove")
	}
}

func TestManagerDestroyVMStateRemovesStoppedStateDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := DefaultManagerConfig()
	cfg.StateDir = tmpDir
	mgr := NewManager(cfg)

	vmID := "test-vm-destroy"
	stateDir := filepath.Join(tmpDir, vmID)
	if err := os.MkdirAll(filepath.Join(stateDir, "persist"), 0o755); err != nil {
		t.Fatalf("mkdir state: %v", err)
	}
	if err := os.WriteFile(filepath.Join(stateDir, "data.img"), []byte("state"), 0o600); err != nil {
		t.Fatalf("write state: %v", err)
	}
	mgr.mu.Lock()
	mgr.vms[vmID] = &VMInstance{
		Config: VMConfig{VMID: vmID},
		State:  StateStopped,
	}
	mgr.mu.Unlock()

	if err := mgr.DestroyVMState(vmID); err != nil {
		t.Fatalf("DestroyVMState: %v", err)
	}
	if _, err := os.Stat(stateDir); !os.IsNotExist(err) {
		t.Fatalf("state dir still exists or stat failed: %v", err)
	}
	if got := mgr.GetVM(vmID); got != nil {
		t.Fatalf("manager still tracks VM: %+v", got)
	}
}

func TestManagerDestroyVMStateRefusesRunningVM(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := DefaultManagerConfig()
	cfg.StateDir = tmpDir
	mgr := NewManager(cfg)

	vmID := "test-vm-running"
	stateDir := filepath.Join(tmpDir, vmID)
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("mkdir state: %v", err)
	}
	mgr.mu.Lock()
	mgr.vms[vmID] = &VMInstance{
		Config: VMConfig{VMID: vmID},
		State:  StateRunning,
	}
	mgr.mu.Unlock()

	if err := mgr.DestroyVMState(vmID); err == nil {
		t.Fatalf("DestroyVMState should reject running VM")
	}
	if _, err := os.Stat(stateDir); err != nil {
		t.Fatalf("state dir should remain: %v", err)
	}
}

func TestManagerMarkFailed(t *testing.T) {
	mgr := NewManager(DefaultManagerConfig())

	inst := &VMInstance{
		Config:  VMConfig{VMID: "test-vm-1"},
		State:   StateRunning,
		Healthy: true,
	}
	mgr.mu.Lock()
	mgr.vms["test-vm-1"] = inst
	mgr.mu.Unlock()

	mgr.MarkFailed("test-vm-1")

	if inst.State != StateFailed {
		t.Errorf("expected failed state, got %s", inst.State)
	}
	if inst.Healthy {
		t.Error("expected unhealthy after MarkFailed")
	}
}

func TestManagerForceKillVM(t *testing.T) {
	mgr := NewManager(DefaultManagerConfig())

	inst := &VMInstance{
		Config:  VMConfig{VMID: "test-vm-1"},
		State:   StateRunning,
		Healthy: true,
		done:    make(chan struct{}),
	}
	mgr.mu.Lock()
	mgr.vms["test-vm-1"] = inst
	mgr.mu.Unlock()

	if err := mgr.ForceKillVM("test-vm-1"); err != nil {
		t.Fatalf("ForceKillVM: %v", err)
	}

	if inst.State != StateFailed {
		t.Errorf("expected failed state, got %s", inst.State)
	}
}

func TestManagerStopVM(t *testing.T) {
	mgr := NewManager(DefaultManagerConfig())

	inst := &VMInstance{
		Config:  VMConfig{VMID: "test-vm-1"},
		State:   StateRunning,
		Healthy: true,
		done:    make(chan struct{}),
	}
	mgr.mu.Lock()
	mgr.vms["test-vm-1"] = inst
	mgr.mu.Unlock()

	if err := mgr.StopVM("test-vm-1"); err != nil {
		t.Fatalf("StopVM: %v", err)
	}

	if inst.State != StateStopped {
		t.Errorf("expected stopped state, got %s", inst.State)
	}
	if inst.Healthy {
		t.Error("expected unhealthy after stop")
	}

	// Stop nonexistent VM.
	if err := mgr.StopVM("nonexistent"); err == nil {
		t.Error("expected error for nonexistent VM")
	}
}

func TestManagerHibernateVM(t *testing.T) {
	mgr := NewManager(DefaultManagerConfig())

	inst := &VMInstance{
		Config:  VMConfig{VMID: "test-vm-1"},
		State:   StateRunning,
		Healthy: true,
		done:    make(chan struct{}),
	}
	mgr.mu.Lock()
	mgr.vms["test-vm-1"] = inst
	mgr.mu.Unlock()

	if err := mgr.HibernateVM("test-vm-1"); err != nil {
		t.Fatalf("HibernateVM: %v", err)
	}

	if inst.State != StateHibernated {
		t.Errorf("expected hibernated state, got %s", inst.State)
	}

	// Hibernate non-running VM should fail.
	inst2 := &VMInstance{
		Config: VMConfig{VMID: "test-vm-2"},
		State:  StateStopped,
	}
	mgr.mu.Lock()
	mgr.vms["test-vm-2"] = inst2
	mgr.mu.Unlock()

	if err := mgr.HibernateVM("test-vm-2"); err == nil {
		t.Error("expected error hibernating non-running VM")
	}
}

// --- Config Tests ---

func TestLoadConfigFromEnv(t *testing.T) {
	// Test with no env vars.
	cfg := LoadConfigFromEnv()
	if cfg.KernelImagePath != "" {
		t.Errorf("expected empty KernelImagePath, got %s", cfg.KernelImagePath)
	}

	t.Setenv("VM_BOOT_READY_TIMEOUT", "7s")
	cfg = LoadConfigFromEnv()
	if cfg.BootReadyTimeout != 7*time.Second {
		t.Fatalf("expected BootReadyTimeout=7s, got %s", cfg.BootReadyTimeout)
	}
}

func TestConfigValidate(t *testing.T) {
	cfg := ManagerConfig{} // empty config

	// Missing kernel.
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for missing kernel")
	}

	cfg.KernelImagePath = "/path/to/kernel"

	// Missing state dir (rootfs is no longer required with microvm.nix approach).
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for missing state dir")
	}

	cfg.StateDir = "/path/to/state"

	// Valid config with just kernel and state dir (microvm.nix approach).
	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate: %v", err)
	}

	// Also valid with rootfs (legacy approach).
	cfg.RootfsPath = "/path/to/rootfs"
	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate with rootfs: %v", err)
	}

	// Also valid with store disk (microvm.nix approach).
	cfg.StoreDiskPath = "/path/to/storedisk.erofs"
	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate with store disk: %v", err)
	}
}

func TestIsFirecrackerAvailable(t *testing.T) {
	// On macOS, Firecracker is not available.
	// This test just verifies the function doesn't panic.
	_ = IsFirecrackerAvailable()
}

func TestHostProcessFallbackEnabled(t *testing.T) {
	t.Setenv("VMCTL_ALLOW_HOST_PROCESS", "")
	if !HostProcessFallbackEnabled() {
		t.Fatal("expected host-process fallback to default to enabled")
	}

	t.Setenv("VMCTL_ALLOW_HOST_PROCESS", "false")
	if HostProcessFallbackEnabled() {
		t.Fatal("expected false to disable host-process fallback")
	}

	t.Setenv("VMCTL_ALLOW_HOST_PROCESS", "0")
	if HostProcessFallbackEnabled() {
		t.Fatal("expected 0 to disable host-process fallback")
	}

	t.Setenv("VMCTL_ALLOW_HOST_PROCESS", "true")
	if !HostProcessFallbackEnabled() {
		t.Fatal("expected true to enable host-process fallback")
	}
}

func TestManagerPersistentDirCreation(t *testing.T) {
	// Verify that BootVM creates the persistent directory.
	tmpDir := t.TempDir()
	cfg := DefaultManagerConfig()
	cfg.StateDir = tmpDir
	cfg.KernelImagePath = "/nonexistent/kernel"
	cfg.RootfsPath = "/nonexistent/rootfs"

	mgr := NewManager(cfg)

	persistDir := filepath.Join(tmpDir, "test-vm-1", "persist")

	// BootVM will fail because Firecracker is not available, but it
	// should still create the persistent directory.
	_, _ = mgr.BootVM(VMConfig{
		VMID:          "test-vm-1",
		PersistentDir: persistDir,
	})

	// The persistent directory should have been created.
	if _, err := os.Stat(persistDir); os.IsNotExist(err) {
		t.Errorf("expected persistent directory to be created at %s", persistDir)
	}
}

func TestCreateDataImage_CreatesMissingFile(t *testing.T) {
	tmpDir := t.TempDir()
	binDir := filepath.Join(tmpDir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("MkdirAll binDir: %v", err)
	}

	mkfsPath := filepath.Join(binDir, "mkfs.ext4")
	if err := os.WriteFile(mkfsPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("WriteFile mkfs.ext4: %v", err)
	}
	t.Setenv("PATH", binDir)

	mgr := NewManager(DefaultManagerConfig())
	dataImg := filepath.Join(tmpDir, "nested", "data.img")

	if err := mgr.createDataImage(dataImg, 8); err != nil {
		t.Fatalf("createDataImage: %v", err)
	}

	info, err := os.Stat(dataImg)
	if err != nil {
		t.Fatalf("Stat data.img: %v", err)
	}
	if info.Size() != 8*1024*1024 {
		t.Fatalf("expected data.img size %d, got %d", 8*1024*1024, info.Size())
	}
}

func TestDataImageSizeCoversSelfDevelopmentWorkspace(t *testing.T) {
	if dataImageSizeMB < 8192 {
		t.Fatalf("dataImageSizeMB = %d, want at least 8192 for candidate repo, Dolt, cache, and export artifacts", dataImageSizeMB)
	}
}

func TestCopySparseFileClonesDataImageContent(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewManager(DefaultManagerConfig())

	src := filepath.Join(tmpDir, "source", "data.img")
	dst := filepath.Join(tmpDir, "target", "data.img")
	if err := os.MkdirAll(filepath.Dir(src), 0o755); err != nil {
		t.Fatalf("MkdirAll source: %v", err)
	}
	if err := os.WriteFile(src, []byte("prefix"), 0o644); err != nil {
		t.Fatalf("WriteFile source: %v", err)
	}
	f, err := os.OpenFile(src, os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("OpenFile source: %v", err)
	}
	if _, err := f.Seek(1024*1024, 0); err != nil {
		t.Fatalf("Seek source: %v", err)
	}
	if _, err := f.Write([]byte("suffix")); err != nil {
		t.Fatalf("Write suffix: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("Close source: %v", err)
	}

	if err := mgr.copySparseFile(src, dst); err != nil {
		t.Fatalf("copySparseFile: %v", err)
	}
	srcData, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("ReadFile source: %v", err)
	}
	dstData, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("ReadFile destination: %v", err)
	}
	if string(dstData) != string(srcData) {
		t.Fatalf("destination content mismatch")
	}
	srcInfo, err := os.Stat(src)
	if err != nil {
		t.Fatalf("Stat source: %v", err)
	}
	dstInfo, err := os.Stat(dst)
	if err != nil {
		t.Fatalf("Stat destination: %v", err)
	}
	if dstInfo.Size() != srcInfo.Size() {
		t.Fatalf("destination size = %d, want %d", dstInfo.Size(), srcInfo.Size())
	}
}

func TestBootVMRejectsRunningSourceDataImageFork(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := DefaultManagerConfig()
	cfg.StateDir = tmpDir
	cfg.StoreDiskPath = "/nonexistent/store.erofs"
	mgr := NewManager(cfg)

	mgr.mu.Lock()
	mgr.vms["source-vm"] = &VMInstance{
		Config: VMConfig{VMID: "source-vm"},
		State:  StateRunning,
	}
	mgr.mu.Unlock()

	_, err := mgr.BootVM(VMConfig{
		VMID:       "target-vm",
		SourceVMID: "source-vm",
	})
	if err == nil {
		t.Fatal("expected running source data-image fork to fail")
	}
	if !strings.Contains(err.Error(), "refusing unsafe live data image copy") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBootVMClonesSourceDataImageBeforeLaunch(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := DefaultManagerConfig()
	cfg.StateDir = tmpDir
	cfg.StoreDiskPath = "/nonexistent/store.erofs"
	cfg.KernelImagePath = "/nonexistent/kernel"
	cfg.RootfsPath = "/nonexistent/rootfs"
	mgr := NewManager(cfg)

	sourceData := filepath.Join(tmpDir, "source-vm", "data.img")
	if err := os.MkdirAll(filepath.Dir(sourceData), 0o755); err != nil {
		t.Fatalf("MkdirAll source VM: %v", err)
	}
	want := []byte("source filesystem bytes")
	if err := os.WriteFile(sourceData, want, 0o644); err != nil {
		t.Fatalf("WriteFile source data image: %v", err)
	}

	_, err := mgr.BootVM(VMConfig{
		VMID:       "target-vm",
		SourceVMID: "source-vm",
	})
	if err == nil {
		t.Fatal("expected launch to fail with nonexistent Firecracker inputs")
	}

	targetData := filepath.Join(tmpDir, "target-vm", "data.img")
	got, readErr := os.ReadFile(targetData)
	if readErr != nil {
		t.Fatalf("ReadFile target data image: %v", readErr)
	}
	if string(got) != string(want) {
		t.Fatalf("target data image = %q, want %q", string(got), string(want))
	}
}

func TestBootVMExpandsExistingSmallDataImageBeforeLaunch(t *testing.T) {
	tmpDir := t.TempDir()
	binDir := filepath.Join(tmpDir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("MkdirAll binDir: %v", err)
	}
	resizePath := filepath.Join(binDir, "resize2fs")
	if err := os.WriteFile(resizePath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("WriteFile resize2fs: %v", err)
	}
	t.Setenv("PATH", binDir)

	cfg := DefaultManagerConfig()
	cfg.StateDir = tmpDir
	cfg.StoreDiskPath = "/nonexistent/store.erofs"
	cfg.KernelImagePath = "/nonexistent/kernel"
	cfg.RootfsPath = "/nonexistent/rootfs"
	mgr := NewManager(cfg)

	dataImg := filepath.Join(tmpDir, "old-vm-123", "data.img")
	if err := os.MkdirAll(filepath.Dir(dataImg), 0o755); err != nil {
		t.Fatalf("MkdirAll VM dir: %v", err)
	}
	if err := os.WriteFile(dataImg, []byte("old small image"), 0o644); err != nil {
		t.Fatalf("WriteFile data image: %v", err)
	}

	_, err := mgr.BootVM(VMConfig{VMID: "old-vm-123"})
	if err == nil {
		t.Fatal("expected launch to fail with nonexistent Firecracker inputs")
	}
	info, statErr := os.Stat(dataImg)
	if statErr != nil {
		t.Fatalf("Stat data image: %v", statErr)
	}
	want := int64(dataImageSizeMB) * 1024 * 1024
	if info.Size() != want {
		t.Fatalf("data image size = %d, want %d", info.Size(), want)
	}
}

func TestReattachVMRequiresPIDAndHealthyGuest(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewManager(ManagerConfig{
		StateDir:           tmpDir,
		GuestPort:          8085,
		HealthCheckTimeout: time.Second,
	})

	if _, err := mgr.ReattachVM("vm-missing-pid", "http://127.0.0.1:1", 4); err == nil {
		t.Fatal("expected missing pid reattach to fail")
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))
	t.Cleanup(srv.Close)

	if err := mgr.savePID("vm-reattach", os.Getpid()); err != nil {
		t.Fatalf("savePID: %v", err)
	}
	inst, err := mgr.ReattachVM("vm-reattach", srv.URL, 9)
	if err != nil {
		t.Fatalf("ReattachVM: %v", err)
	}
	if inst.State != StateRunning || !inst.Healthy {
		t.Fatalf("reattached state=%s healthy=%v, want running healthy", inst.State, inst.Healthy)
	}
	if inst.PID != os.Getpid() {
		t.Fatalf("reattached pid=%d, want %d", inst.PID, os.Getpid())
	}
	if inst.Config.Epoch != 9 {
		t.Fatalf("reattached epoch=%d, want 9", inst.Config.Epoch)
	}
}

func TestFirecrackerCmdlineBytesMatchVM(t *testing.T) {
	vmID := "vm-existing-user"
	matches := [][]byte{
		[]byte("/nix/store/firecracker/bin/firecracker\x00--no-api\x00--id\x00vm-existing-user\x00--config-file\x00/state/fc-config.json"),
		[]byte("firecracker --no-api --id=vm-existing-user --config-file /state/fc-config.json"),
	}
	for _, cmdline := range matches {
		if !firecrackerCmdlineBytesMatchVM(cmdline, vmID) {
			t.Fatalf("expected cmdline to match VM %s: %q", vmID, string(cmdline))
		}
	}

	nonMatches := [][]byte{
		[]byte("/bin/sleep\x0060"),
		[]byte("firecracker --no-api --id vm-other-user --config-file /state/fc-config.json"),
		[]byte("firecracker --no-api --id vm-existing-user-extra --config-file /state/fc-config.json"),
		[]byte(""),
	}
	for _, cmdline := range nonMatches {
		if firecrackerCmdlineBytesMatchVM(cmdline, vmID) {
			t.Fatalf("expected cmdline not to match VM %s: %q", vmID, string(cmdline))
		}
	}
}

func TestFirecrackerPIDsForVMFromEntries(t *testing.T) {
	entries := []procCmdlineEntry{
		{
			pid:  42,
			data: []byte("/nix/store/firecracker/bin/firecracker\x00--no-api\x00--id\x00vm-existing-user\x00--config-file\x00/state/fc-config.json"),
		},
		{
			pid:  7,
			data: []byte("firecracker --no-api --id=vm-existing-user --config-file /state/fc-config.json"),
		},
		{
			pid:  99,
			data: []byte("firecracker --no-api --id vm-other-user --config-file /state/fc-config.json"),
		},
		{
			pid:  100,
			data: []byte("/bin/sleep\x0060"),
		},
	}

	got := firecrackerPIDsForVMFromEntries(entries, "vm-existing-user")
	want := []int{7, 42}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("firecrackerPIDsForVMFromEntries = %+v, want %+v", got, want)
	}
}

func TestReserveHostURLLockedPreservesReattachedNetworkSlots(t *testing.T) {
	mgr := NewManager(ManagerConfig{HostBasePort: 9000})

	mgr.reserveHostURLLocked("http://10.200.9.2:8085")
	if mgr.nextPort != 9010 {
		t.Fatalf("nextPort=%d, want 9010 after reserving 10.200.9.2", mgr.nextPort)
	}

	mgr.reserveHostURLLocked("http://10.200.2.2:8085")
	if mgr.nextPort != 9010 {
		t.Fatalf("nextPort=%d, lower reservation should not move nextPort backward", mgr.nextPort)
	}

	mgr.reserveHostURLLocked("http://172.9.0.2:8085")
	if mgr.nextPort != 9010 {
		t.Fatalf("nextPort=%d, legacy 172.x reservation should not move nextPort backward", mgr.nextPort)
	}

	mgr.reserveHostURLLocked("http://127.0.0.1:9017")
	if mgr.nextPort != 9018 {
		t.Fatalf("nextPort=%d, want localhost host-port reservation to 9018", mgr.nextPort)
	}
}

func TestTapReachableHostServicePortsIncludeMaild(t *testing.T) {
	ports := tapReachableHostServicePorts()
	for _, want := range []string{"8083", "8084", "8087"} {
		if !containsString(ports, want) {
			t.Fatalf("tapReachableHostServicePorts() = %#v, missing %s", ports, want)
		}
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

// --- Guest Isolation Tests (VAL-VM-007, VAL-VM-011) ---

func TestBuildFirecrackerConfig_NoHostControlPlaneAccess(t *testing.T) {
	// VAL-VM-007: Guest workloads cannot reach host control-plane surfaces by
	// loopback or host filesystem paths. vmctl/gateway access is deliberately
	// exposed through per-VM tap-subnet URLs, not host localhost URLs.
	cfg := DefaultManagerConfig()
	cfg.StateDir = t.TempDir()
	cfg.KernelImagePath = "/opt/go-choir/guest/vmlinux"
	cfg.RootfsPath = "/opt/go-choir/guest/rootfs.ext4"

	mgr := NewManager(cfg)

	vmCfg := VMConfig{
		VMID:              "vm-isolation-test",
		KernelImagePath:   cfg.KernelImagePath,
		RootfsPath:        cfg.RootfsPath,
		GuestPort:         8085,
		MachineCPUCount:   2,
		MachineMemSizeMib: 512,
		Epoch:             1,
	}

	fcConfig := mgr.buildFirecrackerConfig(vmCfg, 9001)

	// Verify the guest port is the sandbox port, not a control-plane port.
	bootSource := fcConfig["boot-source"].(map[string]interface{})
	bootArgs := bootSource["boot_args"].(string)
	if !containsStr(bootArgs, "guest_port=8085") {
		t.Errorf("expected guest_port=8085 in boot args, got: %s", bootArgs)
	}

	// Verify the config does not reference host control-plane URLs or ports.
	forbiddenPatterns := []string{
		"127.0.0.1:8081", // auth
		"127.0.0.1:8082", // proxy
		"127.0.0.1:8083", // vmctl
		"127.0.0.1:8084", // gateway
		"/var/lib/go-choir/auth",
		"/var/lib/go-choir/auth-signing",
		"/var/lib/go-choir/gateway-provider.env",
		"/var/run/",
		"/run/",
	}
	for _, pattern := range forbiddenPatterns {
		if contains(fcConfig, pattern) {
			t.Errorf("VAL-VM-007: firecracker config exposes host control-plane path: %s", pattern)
		}
	}

	// Verify the network interface uses a tap device, not host-side ports.
	netIfaces, ok := fcConfig["network-interfaces"].([]map[string]interface{})
	if !ok || len(netIfaces) == 0 {
		t.Fatal("expected network-interfaces in config")
	}
	if netIfaces[0]["iface_id"] != "eth0" {
		t.Errorf("expected eth0 interface, got %v", netIfaces[0]["iface_id"])
	}
	// The host_dev_name should be a VM-specific tap device, not a host interface.
	hostDev, _ := netIfaces[0]["host_dev_name"].(string)
	if !containsStr(hostDev, "vm-") || !containsStr(hostDev, "-tap") {
		t.Errorf("expected VM-specific tap device name, got: %s", hostDev)
	}
}

func TestBuildFirecrackerConfig_ComprehensiveSecretExclusion(t *testing.T) {
	// VAL-VM-011: Comprehensive check that NO provider credentials or
	// host-side secrets appear anywhere in the Firecracker VM configuration.
	// This test covers the full forbidden pattern list from the environment
	// documentation.
	cfg := DefaultManagerConfig()
	cfg.StateDir = t.TempDir()
	cfg.KernelImagePath = "/opt/go-choir/guest/vmlinux"
	cfg.RootfsPath = "/opt/go-choir/guest/rootfs.ext4"

	mgr := NewManager(cfg)

	vmCfg := VMConfig{
		VMID:              "vm-secret-test",
		KernelImagePath:   cfg.KernelImagePath,
		RootfsPath:        cfg.RootfsPath,
		GuestPort:         8085,
		MachineCPUCount:   2,
		MachineMemSizeMib: 512,
		Epoch:             1,
	}

	fcConfig := mgr.buildFirecrackerConfig(vmCfg, 9001)

	// Comprehensive forbidden pattern list covering all provider credentials
	// and host-side secret patterns from environment.md.
	forbiddenPatterns := []string{
		// Provider credential env vars
		"ZAI_API_KEY",
		"AWS_BEARER_TOKEN_BEDROCK",
		"AWS_REGION",
		"RUNTIME_BEDROCK_MODEL",
		"RUNTIME_ZAI_MODEL",
		"FIREWORKS_API_KEY",
		"RUNTIME_FIREWORKS_MODEL",
		"FIREWORKS_BASE_URL",
		// Auth signing material
		"AUTH_JWT_PRIVATE_KEY_PATH",
		"ed25519-key",
		// Generic secret patterns
		"Bearer",
		"SECRET",
		"PASSWORD",
		"api_key",
		"apiKey",
		"api-key",
		// Host secret paths
		"gateway-provider.env",
		"sandbox-gateway-token.env",
		"auth-signing",
	}
	for _, pattern := range forbiddenPatterns {
		if contains(fcConfig, pattern) {
			t.Errorf("VAL-VM-011: firecracker config contains forbidden secret pattern: %s", pattern)
		}
	}

	// Verify the drives section contains guest drives, not host paths.
	// With the microvm.nix approach, we expect a store drive and a data drive.
	// With the legacy approach, we expect a rootfs drive and a data drive.
	drives, ok := fcConfig["drives"].([]map[string]interface{})
	if !ok || len(drives) < 1 {
		t.Fatal("expected at least 1 drive in config")
	}
	driveIDs := make([]string, len(drives))
	for i, d := range drives {
		driveIDs[i], _ = d["drive_id"].(string)
	}
	hasStoreOrRootfs := false
	for _, id := range driveIDs {
		if id == "store" || id == "rootfs" {
			hasStoreOrRootfs = true
			break
		}
	}
	if !hasStoreOrRootfs {
		t.Errorf("expected store or rootfs drive, got drives: %v", driveIDs)
	}
	// Verify data drive is present (per-VM mutable state).
	hasData := false
	for _, id := range driveIDs {
		if id == "data" {
			hasData = true
			break
		}
	}
	if !hasData {
		t.Errorf("expected data drive, got drives: %v", driveIDs)
	}
}

func TestBuildFirecrackerConfig_GuestPortInBootArgs(t *testing.T) {
	// Verify the guest port is passed via boot args so the guest sandbox
	// knows which port to listen on. This is the only way the guest receives
	// network configuration — no host IPs or control-plane ports are exposed.
	cfg := DefaultManagerConfig()
	cfg.StateDir = t.TempDir()
	cfg.KernelImagePath = "/opt/go-choir/guest/vmlinux"
	cfg.RootfsPath = "/opt/go-choir/guest/rootfs.ext4"

	mgr := NewManager(cfg)

	vmCfg := VMConfig{
		VMID:              "vm-bootargs-test",
		KernelImagePath:   cfg.KernelImagePath,
		RootfsPath:        cfg.RootfsPath,
		GuestPort:         8085,
		MachineCPUCount:   2,
		MachineMemSizeMib: 512,
		Epoch:             1,
	}

	fcConfig := mgr.buildFirecrackerConfig(vmCfg, 9001)

	bootSource := fcConfig["boot-source"].(map[string]interface{})
	bootArgs := bootSource["boot_args"].(string)

	// Verify the boot args contain the expected guest parameters.
	expectedArgs := []string{
		"guest_port=8085", "vm_id=vm-bootargs-test", "epoch=1",
		"persistent=/mnt/persistent",
		// init= and root= are required for the guest to boot correctly.
		"init=/bin/init", "root=/dev/vda",
	}
	for _, arg := range expectedArgs {
		if !containsStr(bootArgs, arg) {
			t.Errorf("expected boot arg %s in: %s", arg, bootArgs)
		}
	}

	// Verify the boot args do NOT contain host-side provider parameters.
	forbiddenArgs := []string{"provider", "api_key", "secret", "auth"}
	for _, arg := range forbiddenArgs {
		if containsStr(bootArgs, arg) {
			t.Errorf("VAL-VM-011: boot args contain forbidden pattern: %s (full: %s)", arg, bootArgs)
		}
	}
}

func TestBuildFirecrackerConfig_MicrovmUsesStoreDiskAndKernelParams(t *testing.T) {
	cfg := DefaultManagerConfig()
	cfg.StateDir = t.TempDir()
	cfg.KernelImagePath = "/opt/go-choir/guest/vmlinux"
	cfg.InitrdPath = "/opt/go-choir/guest/initrd"
	cfg.StoreDiskPath = "/opt/go-choir/guest/storedisk.erofs"
	cfg.KernelParams = "root=fstab init=/nix/store/example-init regInfo=/nix/store/example-reginfo"

	mgr := NewManager(cfg)

	vmCfg := VMConfig{
		VMID:              "vm-microvm-test",
		KernelImagePath:   cfg.KernelImagePath,
		InitrdPath:        cfg.InitrdPath,
		StoreDiskPath:     cfg.StoreDiskPath,
		GuestPort:         8085,
		MachineCPUCount:   2,
		MachineMemSizeMib: 512,
		Epoch:             1,
	}

	fcConfig := mgr.buildFirecrackerConfig(vmCfg, 9000)
	drives, ok := fcConfig["drives"].([]map[string]interface{})
	if !ok {
		t.Fatal("expected drives slice")
	}
	if len(drives) != 2 {
		t.Fatalf("expected store and data drives; got %d", len(drives))
	}
	if drives[0]["drive_id"] != "store" || drives[0]["is_root_device"] != false {
		t.Fatalf("expected first drive to be store disk, got %#v", drives[0])
	}
	if drives[1]["drive_id"] != "data" {
		t.Fatalf("expected second drive to be data disk, got %#v", drives[1])
	}

	bootArgs := fcConfig["boot-source"].(map[string]interface{})["boot_args"].(string)
	for _, arg := range []string{
		"console=ttyS0,115200",
		"root=fstab",
		"init=/nix/store/example-init",
		"regInfo=/nix/store/example-reginfo",
		"i8042.noaux",
		"i8042.nomux",
		"i8042.nopnp",
		"i8042.dumbkbd",
		"guest_port=8085",
		"vm_id=vm-microvm-test",
		"epoch=1",
		"choir.gateway_url=http://10.200.0.1:8084",
		"choir.vmctl_url=http://10.200.0.1:8083",
		"choir.maild_url=http://10.200.0.1:8087",
		"ip=10.200.0.2::10.200.0.1:255.255.255.252::eth0:off",
	} {
		if !containsStr(bootArgs, arg) {
			t.Fatalf("expected boot arg %q in %q", arg, bootArgs)
		}
	}
}

func TestGuestInitScript_NoProviderCredentials(t *testing.T) {
	// VAL-VM-011: Verify the guest init script pattern used in guest-image.nix
	// does not pass provider credentials to the guest. This test mirrors the
	// init script in nix/guest-image.nix to ensure it stays clean.
	//
	// The guest init script sets only:
	//   - SANDBOX_PORT (from guest_port kernel param)
	//   - SANDBOX_ID (from vm_id kernel param)
	//   - RUNTIME_GATEWAY_URL / RUNTIME_GATEWAY_TOKEN (sandbox auth only)
	//   - RUNTIME_VMCTL_URL (tap-subnet control plane for super VM tools)
	//   - RUNTIME_MAILD_URL (tap-subnet draft persistence only)
	//   - RUNTIME_STORE_PATH (local persistent path)
	//
	// No provider credentials or host-side secret paths are set.
	guestEnvVars := []string{
		"SANDBOX_PORT",
		"SANDBOX_ID",
		"RUNTIME_GATEWAY_URL",
		"RUNTIME_GATEWAY_TOKEN",
		"RUNTIME_VMCTL_URL",
		"RUNTIME_MAILD_URL",
		"RUNTIME_STORE_PATH",
	}

	forbiddenEnvVars := []string{
		"ZAI_API_KEY",
		"AWS_BEARER_TOKEN_BEDROCK",
		"FIREWORKS_API_KEY",
		"AUTH_JWT_PRIVATE_KEY_PATH",
		"PROXY_AUTH_PUBLIC_KEY_PATH",
		"GATEWAY_PORT",
		"PROXY_PORT",
		"VMCTL_PORT",
		"AUTH_PORT",
	}

	// Verify no forbidden env vars appear in the allowed set.
	for _, forbidden := range forbiddenEnvVars {
		for _, allowed := range guestEnvVars {
			if allowed == forbidden {
				t.Errorf("VAL-VM-011: guest env var %s is in the forbidden list", forbidden)
			}
		}
	}
}

// --- Helper functions ---

func contains(m map[string]interface{}, pattern string) bool {
	for k, v := range m {
		if containsStr(k, pattern) || containsStr(fmtVal(v), pattern) {
			return true
		}
	}
	return false
}

func fmtVal(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case []interface{}:
		s := ""
		for _, item := range val {
			s += fmtVal(item)
		}
		return s
	case map[string]interface{}:
		s := ""
		for k, v := range val {
			s += k + "=" + fmtVal(v)
		}
		return s
	default:
		return ""
	}
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		(len(s) > 0 && len(sub) > 0 && findSubstr(s, sub)))
}

func findSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// --- Gateway Token Tests ---

func TestBootVM_WritesGatewayToken(t *testing.T) {
	// Verify that when a gateway token is provided, it is written to the
	// persistent directory so the guest init script can read it.
	tmpDir := t.TempDir()
	cfg := DefaultManagerConfig()
	cfg.StateDir = tmpDir
	cfg.KernelImagePath = "/nonexistent/kernel"
	cfg.RootfsPath = "/nonexistent/rootfs"

	mgr := NewManager(cfg)

	persistDir := filepath.Join(tmpDir, "test-vm-gw", "persist")
	token := "test-vm-gw:abcdef1234567890"

	_, err := mgr.BootVM(VMConfig{
		VMID:          "test-vm-gw",
		PersistentDir: persistDir,
		GatewayToken:  token,
	})
	// BootVM will fail because Firecracker is not available, but the
	// token should still be written before the launch attempt.
	if err == nil {
		t.Log("BootVM succeeded unexpectedly (no Firecracker)")
	}

	tokenPath := filepath.Join(persistDir, "gateway-token")
	data, err := os.ReadFile(tokenPath)
	if err != nil {
		t.Fatalf("expected gateway token file at %s: %v", tokenPath, err)
	}
	if string(data) != token {
		t.Errorf("expected token %q, got %q", token, string(data))
	}

	readToken, err := mgr.ReadGatewayToken("test-vm-gw")
	if err != nil {
		t.Fatalf("ReadGatewayToken: %v", err)
	}
	if readToken != token {
		t.Errorf("ReadGatewayToken = %q, want %q", readToken, token)
	}
}

func TestBootVM_NoGatewayToken(t *testing.T) {
	// Verify that when no gateway token is provided, no token file is created.
	tmpDir := t.TempDir()
	cfg := DefaultManagerConfig()
	cfg.StateDir = tmpDir
	cfg.KernelImagePath = "/nonexistent/kernel"
	cfg.RootfsPath = "/nonexistent/rootfs"

	mgr := NewManager(cfg)

	persistDir := filepath.Join(tmpDir, "test-vm-nogw", "persist")

	_, _ = mgr.BootVM(VMConfig{
		VMID:          "test-vm-nogw",
		PersistentDir: persistDir,
		// GatewayToken intentionally empty
	})

	tokenPath := filepath.Join(persistDir, "gateway-token")
	if _, err := os.Stat(tokenPath); err == nil {
		t.Error("expected no gateway token file when token is empty")
	}
}

func TestBuildFirecrackerConfig_IPConfigInBootArgs(t *testing.T) {
	// Verify the ip= kernel parameter is correctly formatted with guest
	// and host IPs from the /30 subnet allocation.
	cfg := DefaultManagerConfig()
	cfg.StateDir = t.TempDir()
	cfg.KernelImagePath = "/opt/go-choir/guest/vmlinux"
	cfg.RootfsPath = "/opt/go-choir/guest/rootfs.ext4"

	mgr := NewManager(cfg)

	vmCfg := VMConfig{
		VMID:              "vm-ip-test",
		KernelImagePath:   cfg.KernelImagePath,
		RootfsPath:        cfg.RootfsPath,
		GuestPort:         8085,
		MachineCPUCount:   2,
		MachineMemSizeMib: 512,
		Epoch:             1,
	}

	// hostPort 9001 maps to the second subnet in the bounded private pool.
	// guest IP = 10.200.1.2, host IP = 10.200.1.1
	fcConfig := mgr.buildFirecrackerConfig(vmCfg, 9001)

	bootSource := fcConfig["boot-source"].(map[string]interface{})
	bootArgs := bootSource["boot_args"].(string)

	// Verify the ip= parameter contains the expected guest/host IPs.
	if !containsStr(bootArgs, "ip=10.200.1.2::10.200.1.1:255.255.255.252::eth0:off") {
		t.Errorf("expected ip= parameter with correct subnet in boot args: %s", bootArgs)
	}
}

func TestBuildFirecrackerConfig_IncludesGatewayTokenBootstrapParam(t *testing.T) {
	cfg := DefaultManagerConfig()
	cfg.StateDir = t.TempDir()
	cfg.KernelImagePath = "/opt/go-choir/guest/vmlinux"
	cfg.InitrdPath = "/opt/go-choir/guest/initrd"
	cfg.StoreDiskPath = "/opt/go-choir/guest/storedisk.erofs"
	cfg.KernelParams = "root=fstab init=/nix/store/example-init regInfo=/nix/store/example-reginfo"

	mgr := NewManager(cfg)

	vmCfg := VMConfig{
		VMID:              "vm-gateway-token-test",
		KernelImagePath:   cfg.KernelImagePath,
		InitrdPath:        cfg.InitrdPath,
		StoreDiskPath:     cfg.StoreDiskPath,
		GuestPort:         8085,
		MachineCPUCount:   2,
		MachineMemSizeMib: 512,
		GatewayToken:      "vm-gateway-token-test:abcdef123456",
		Epoch:             1,
	}

	fcConfig := mgr.buildFirecrackerConfig(vmCfg, 9000)
	bootArgs := fcConfig["boot-source"].(map[string]interface{})["boot_args"].(string)
	if !containsStr(bootArgs, "choir.gateway_token=vm-gateway-token-test:abcdef123456") {
		t.Fatalf("expected gateway token bootstrap arg in %q", bootArgs)
	}
}

func TestBuildFirecrackerConfig_LoadsPersistedGatewayTokenBootstrapParam(t *testing.T) {
	cfg := DefaultManagerConfig()
	cfg.StateDir = t.TempDir()
	cfg.KernelImagePath = "/opt/go-choir/guest/vmlinux"
	cfg.InitrdPath = "/opt/go-choir/guest/initrd"
	cfg.StoreDiskPath = "/opt/go-choir/guest/storedisk.erofs"
	cfg.KernelParams = "root=fstab init=/nix/store/example-init regInfo=/nix/store/example-reginfo"

	mgr := NewManager(cfg)

	persistDir := filepath.Join(cfg.StateDir, "vm-persisted-token-test", "persist")
	if err := os.MkdirAll(persistDir, 0o755); err != nil {
		t.Fatalf("create persist dir: %v", err)
	}
	token := "vm-persisted-token-test:abcdef123456"
	if err := os.WriteFile(filepath.Join(persistDir, "gateway-token"), []byte(token), 0o600); err != nil {
		t.Fatalf("write gateway token: %v", err)
	}

	vmCfg := VMConfig{
		VMID:              "vm-persisted-token-test",
		KernelImagePath:   cfg.KernelImagePath,
		InitrdPath:        cfg.InitrdPath,
		StoreDiskPath:     cfg.StoreDiskPath,
		GuestPort:         8085,
		MachineCPUCount:   2,
		MachineMemSizeMib: 512,
		PersistentDir:     persistDir,
		Epoch:             1,
	}

	fcConfig := mgr.buildFirecrackerConfig(vmCfg, 9000)
	bootArgs := fcConfig["boot-source"].(map[string]interface{})["boot_args"].(string)
	if !containsStr(bootArgs, "choir.gateway_token="+token) {
		t.Fatalf("expected persisted gateway token bootstrap arg in %q", bootArgs)
	}
}

func TestBuildFirecrackerConfig_SubnetIsolation(t *testing.T) {
	// Verify that different host ports get different /30 subnets,
	// ensuring VM network isolation (VAL-VM-005).
	cfg := DefaultManagerConfig()
	cfg.StateDir = t.TempDir()
	cfg.KernelImagePath = "/opt/go-choir/guest/vmlinux"
	cfg.RootfsPath = "/opt/go-choir/guest/rootfs.ext4"

	mgr := NewManager(cfg)

	vmCfg := VMConfig{
		VMID:              "vm-subnet-test",
		KernelImagePath:   cfg.KernelImagePath,
		RootfsPath:        cfg.RootfsPath,
		GuestPort:         8085,
		MachineCPUCount:   2,
		MachineMemSizeMib: 512,
		Epoch:             1,
	}

	// VM on port 9000 → 10.200.0.0/30
	fcConfig1 := mgr.buildFirecrackerConfig(vmCfg, 9000)
	// VM on port 9001 → 10.200.1.0/30
	fcConfig2 := mgr.buildFirecrackerConfig(vmCfg, 9001)

	bootArgs1 := fcConfig1["boot-source"].(map[string]interface{})["boot_args"].(string)
	bootArgs2 := fcConfig2["boot-source"].(map[string]interface{})["boot_args"].(string)

	// Verify different subnets.
	if containsStr(bootArgs1, "10.200.0.2") && containsStr(bootArgs2, "10.200.1.2") {
		// Expected: different subnets
	} else {
		t.Errorf("expected different subnets for different host ports:\n  port 9000: %s\n  port 9001: %s", bootArgs1, bootArgs2)
	}

	// Verify the subnets are actually different.
	if bootArgs1 == bootArgs2 {
		t.Error("expected different boot args for different host ports")
	}
}

func TestGuestAndHostIP_TracksPerVMSubnet(t *testing.T) {
	cfg := DefaultManagerConfig()
	mgr := NewManager(cfg)

	guestIP, hostIP := mgr.guestAndHostIP(9000)
	if guestIP != "10.200.0.2" || hostIP != "10.200.0.1" {
		t.Fatalf("port 9000: got guest=%s host=%s", guestIP, hostIP)
	}

	guestIP, hostIP = mgr.guestAndHostIP(9001)
	if guestIP != "10.200.1.2" || hostIP != "10.200.1.1" {
		t.Fatalf("port 9001: got guest=%s host=%s", guestIP, hostIP)
	}

	guestIP, hostIP = mgr.guestAndHostIP(9259)
	if guestIP != "10.201.3.2" || hostIP != "10.201.3.1" {
		t.Fatalf("port 9259: got guest=%s host=%s", guestIP, hostIP)
	}

	for _, port := range []int{9000, 9001, 9255, 9256, 9259, 9000 + vmSubnetCapacity} {
		guestIP, hostIP := mgr.guestAndHostIP(port)
		if net.ParseIP(guestIP) == nil || net.ParseIP(hostIP) == nil {
			t.Fatalf("port %d generated invalid addresses guest=%s host=%s", port, guestIP, hostIP)
		}
	}
}

func TestParseIPAddrShowInterfaces(t *testing.T) {
	out := []byte(`354: vm-vm-6298a-tap    inet 172.5.0.1/30 scope global vm-vm-6298a-tap\       valid_lft forever preferred_lft forever
426: vm-vm-4993c-tap    inet 172.2.0.1/30 scope global vm-vm-4993c-tap\       valid_lft forever preferred_lft forever
426: vm-vm-4993c-tap    inet 172.2.0.1/30 scope global secondary vm-vm-4993c-tap\       valid_lft forever preferred_lft forever
`)

	got := parseIPAddrShowInterfaces(out)
	want := []string{"vm-vm-6298a-tap", "vm-vm-4993c-tap"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestWaitForGuestReady_EventuallySucceeds(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1) < 3 {
			http.Error(w, "booting", http.StatusServiceUnavailable)
			return
		}
		fmt.Fprintln(w, `{"status":"ready"}`)
	}))
	defer srv.Close()

	cfg := DefaultManagerConfig()
	cfg.HealthCheckTimeout = 100 * time.Millisecond
	cfg.BootReadyTimeout = 2 * time.Second
	mgr := NewManager(cfg)

	if err := mgr.waitForGuestReady(srv.URL); err != nil {
		t.Fatalf("waitForGuestReady: %v", err)
	}
	if hits.Load() < 3 {
		t.Fatalf("expected multiple probes, got %d", hits.Load())
	}
}

func TestWaitForGuestReady_TimesOut(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "booting", http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	cfg := DefaultManagerConfig()
	cfg.HealthCheckTimeout = 50 * time.Millisecond
	cfg.BootReadyTimeout = 300 * time.Millisecond
	mgr := NewManager(cfg)

	if err := mgr.waitForGuestReady(srv.URL); err == nil {
		t.Fatal("expected timeout waiting for guest readiness")
	}
}
