package vmctl

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerversion"
	"github.com/yusefmosiah/go-choir/internal/diskinstantiation"
)

func TestVMConstructionLauncherBindsDeviceCodeAndProductReadback(t *testing.T) {
	version := computerversion.ComputerVersion{CodeRef: "code:sha256:test", ArtifactProgramRef: "artifact-program:sha256:test"}
	observations := computerversion.ObservationSet{
		Name:         "guest",
		Version:      version,
		Required:     []computerversion.ObservationKind{computerversion.ObservationFileManifest},
		Observations: []computerversion.Observation{{Kind: computerversion.ObservationFileManifest, Key: "proof.txt", Value: "hash"}},
	}
	guest := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/computer-version/observations" || r.Header.Get("X-Internal-Caller") != "true" {
			http.Error(w, "bad request", http.StatusForbidden)
			return
		}
		if r.URL.Query().Get("code_ref") != string(version.CodeRef) || r.URL.Query().Get("artifact_program_ref") != string(version.ArtifactProgramRef) {
			http.Error(w, "wrong version", http.StatusConflict)
			return
		}
		_ = json.NewEncoder(w).Encode(computerversion.LiveConstructionObservation{State: observations, Geometry: diskinstantiation.RuntimeGeometryReceipt{FilesystemBytes: 32 << 30, FilesystemBlockSize: 4096, AvailableBytes: 31 << 30}})
	}))
	defer guest.Close()

	manager := &mockVMManager{bootResponse: &VMInstanceInfo{HostURL: guest.URL, Epoch: 3, Healthy: true, State: "running", StartedAt: time.Now()}}
	registry := NewOwnershipRegistry("http://sandbox.test")
	persistencePath := t.TempDir() + "/ownerships.json"
	if err := registry.SetPersistencePath(persistencePath); err != nil {
		t.Fatal(err)
	}
	registry.SetVMManager(manager)
	launcher := NewVMConstructionLauncher(registry, guest.Client())
	disk, err := diskinstantiation.FinalizeReceipt(diskinstantiation.Receipt{
		Backend: diskinstantiation.Ext4BackendName, RealizationID: "candidate-construct", DeviceID: "data",
		DevicePath: "/var/lib/go-choir/vm-state/candidate-construct/data.img", CreatedAt: time.Now(),
		Geometry: diskinstantiation.GeometryReceipt{FilesystemType: diskinstantiation.FilesystemExt4, FilesystemLabel: "choir-data", PartitionLayout: diskinstantiation.PartitionLayoutNone, DeviceLogicalBytes: 32 << 30, FilesystemBytes: 32 << 30, FilesystemBlockSize: 4096, FilesystemBlocks: (32 << 30) / 4096, AllocatedBytes: 128 << 20},
	})
	if err != nil {
		t.Fatal(err)
	}
	request := computerversion.ConstructedLaunchRequest{
		Identity:    computerversion.ConstructionIdentity{RealizationID: "candidate-construct", ComputerKind: "candidate", OwnerID: "owner", DesktopID: "candidate", CandidateID: "candidate"},
		Version:     version,
		CodeClosure: computerversion.CodeClosure{Ref: version.CodeRef},
		Disk:        disk,
	}
	intentObserved := false
	manager.bootHook = func(VMManagerConfig) {
		own := registry.vmByID[request.Identity.RealizationID]
		intentObserved = own != nil && own.State == VMStateBooting && !own.ConstructionCommitted && own.ConstructionVersion != nil && *own.ConstructionVersion == version
	}
	boot, err := launcher.Launch(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	if !intentObserved {
		t.Fatal("construction intent was not durable before BootVM")
	}
	if len(manager.boots) != 1 {
		t.Fatalf("boot count = %d", len(manager.boots))
	}
	cfg := manager.boots[0]
	if cfg.DataDevicePath != request.Disk.DevicePath || cfg.CodeRef != string(version.CodeRef) {
		t.Fatalf("launch bindings = %+v", cfg)
	}
	if manager.getVMs == nil {
		manager.getVMs = make(map[string]*VMInstanceInfo)
	}
	manager.getVMs[boot.VMID] = manager.bootResponse
	redirected := boot
	redirected.HostURL = "http://forged.internal"
	if _, err := launcher.Observe(t.Context(), request, redirected); err == nil {
		t.Fatal("caller-supplied HostURL redirected independent product readback")
	}
	got, err := launcher.Observe(t.Context(), request, boot)
	if err != nil {
		t.Fatal(err)
	}
	if got.State.Version != version || len(got.State.Observations) != 1 || got.Geometry.FilesystemBytes != 32<<30 {
		t.Fatalf("product readback = %+v", got)
	}
	if err := launcher.Commit(t.Context(), boot, version, disk); err != nil {
		t.Fatalf("commit lifecycle evidence: %v", err)
	}
	if ownership := registry.vmByID[boot.VMID]; ownership == nil || ownership.Published || ownership.SnapshotKind != "constructed-computer-version" {
		t.Fatalf("constructed lifecycle ownership = %+v", ownership)
	}
	reloaded := NewOwnershipRegistry("http://sandbox.test")
	if err := reloaded.SetPersistencePath(persistencePath); err != nil {
		t.Fatal(err)
	}
	if ownership := reloaded.vmByID[boot.VMID]; ownership == nil || ownership.State != VMStateStopped || ownership.SnapshotKind != "constructed-computer-version" {
		t.Fatalf("reloaded constructed lifecycle ownership = %+v", ownership)
	}
	reloadedOwnership := reloaded.vmByID[boot.VMID]
	restartConfig := vmManagerConfigForOwnership(reloadedOwnership, "token")
	if restartConfig.DataDevicePath != disk.DevicePath || restartConfig.CodeRef != string(version.CodeRef) {
		t.Fatalf("restart lost constructed bindings: %+v", restartConfig)
	}
	reattachManager := &mockVMManager{}
	reloaded.SetVMManager(reattachManager)
	if got := reloaded.ReattachManagedVMs(t.Context(), func(context.Context, string, string) error { return nil }); got != 1 {
		t.Fatalf("configured constructed reattach count = %d", got)
	}
	if len(reattachManager.reattachCfgs) != 1 || reattachManager.reattachCfgs[0].DataDevicePath != disk.DevicePath || reattachManager.reattachCfgs[0].CodeRef != string(version.CodeRef) {
		t.Fatalf("configured reattach lost immutable bindings: %+v", reattachManager.reattachCfgs)
	}
	reloaded.mu.Lock()
	reloaded.vmByID[boot.VMID].State = VMStateFailed
	reloaded.mu.Unlock()
	if _, err := reloaded.ResolveOrAssignDesktopContext(t.Context(), "owner", "candidate"); err == nil {
		t.Fatal("failed constructed lifecycle fell through to legacy replacement")
	}
	if len(reattachManager.boots) != 0 {
		t.Fatalf("legacy replacement booted for failed constructed lifecycle: %+v", reattachManager.boots)
	}
}

func TestConstructedLifecyclePersistenceRejectsMissingBindings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ownerships.json")
	state := persistedOwnershipState{Ownerships: []*VMOwnership{{VMID: "candidate-corrupt", UserID: "owner", DesktopID: "candidate", Kind: VMKindInteractive, SnapshotKind: "constructed-computer-version", State: VMStateActive}}}
	encoded, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	registry := NewOwnershipRegistry("http://sandbox.test")
	if err := registry.SetPersistencePath(path); err == nil {
		t.Fatal("accepted constructed persistence without immutable bindings")
	}
}

func TestVMConstructionLauncherBootErrorPreservesDurableIdentityOnUnsafeCleanup(t *testing.T) {
	version := computerversion.ComputerVersion{CodeRef: "code:sha256:test", ArtifactProgramRef: "artifact-program:sha256:test"}
	disk, err := diskinstantiation.FinalizeReceipt(diskinstantiation.Receipt{Backend: diskinstantiation.Ext4BackendName, RealizationID: "candidate-boot-error", DeviceID: "data", DevicePath: "/var/lib/go-choir/vm-state/candidate-boot-error/data.img", CreatedAt: time.Now(), Geometry: diskinstantiation.GeometryReceipt{FilesystemType: diskinstantiation.FilesystemExt4, FilesystemLabel: "choir-data", PartitionLayout: diskinstantiation.PartitionLayoutNone, DeviceLogicalBytes: 32 << 30, FilesystemBytes: 32 << 30, FilesystemBlockSize: 4096, FilesystemBlocks: (32 << 30) / 4096, AllocatedBytes: 128 << 20}})
	if err != nil {
		t.Fatal(err)
	}
	manager := &mockVMManager{bootError: errors.New("readiness failed"), stopError: errors.New("stop failed"), getVMs: map[string]*VMInstanceInfo{"candidate-boot-error": {State: "failed"}}}
	registry := NewOwnershipRegistry("http://sandbox.test")
	if err := registry.SetPersistencePath(filepath.Join(t.TempDir(), "ownerships.json")); err != nil {
		t.Fatal(err)
	}
	registry.SetVMManager(manager)
	launcher := NewVMConstructionLauncher(registry, nil)
	request := computerversion.ConstructedLaunchRequest{Identity: computerversion.ConstructionIdentity{RealizationID: "candidate-boot-error", ComputerKind: "candidate", OwnerID: "owner", DesktopID: "candidate", CandidateID: "candidate"}, Version: version, CodeClosure: computerversion.CodeClosure{Ref: version.CodeRef}, Disk: disk}
	boot, err := launcher.Launch(t.Context(), request)
	if err == nil || boot.VMID != request.Identity.RealizationID {
		t.Fatalf("unsafe boot cleanup lost identity: boot=%+v err=%v", boot, err)
	}
	own := registry.vmByID[boot.VMID]
	if own == nil || own.State != VMStateFailed || own.ConstructionVersion == nil || own.ConstructionDisk == nil {
		t.Fatalf("unsafe boot cleanup lost durable intent: %+v", own)
	}
	if len(manager.destroys) != 0 {
		t.Fatalf("destroy ran after failed stop: %+v", manager.destroys)
	}
}
