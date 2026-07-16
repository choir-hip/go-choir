package computerversion

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/diskinstantiation"
)

type constructionInputFixture struct {
	code    CodeClosure
	program ArtifactProgram
}

func (f constructionInputFixture) ResolveCode(_ context.Context, ref CodeRef) (CodeClosure, error) {
	if ref != f.code.Ref {
		return CodeClosure{}, ErrInputNotFound
	}
	return f.code, nil
}

func (f constructionInputFixture) ResolveArtifactProgram(_ context.Context, ref ArtifactProgramRef) (ArtifactProgram, error) {
	if ref != f.program.Ref {
		return ArtifactProgram{}, ErrInputNotFound
	}
	return f.program, nil
}

func bindTestConstructionCode(t *testing.T, version *ComputerVersion) CodeClosure {
	t.Helper()
	digest := strings.Repeat("a", 64)
	closure, err := NewCodeClosure(strings.Repeat("1", 40), []CodeArtifact{{Name: SandboxRuntimeArtifactName, SHA256: digest, URI: "nix-store+sha256://" + digest + "/nix/store/sandbox-runtime"}}, generatorFixedTime)
	if err != nil {
		t.Fatal(err)
	}
	version.CodeRef = closure.Ref
	return closure
}

type constructionArtifactFixture struct {
	payload []byte
}

func (f constructionArtifactFixture) VerifyArtifact(context.Context, string, string) error {
	return nil
}
func (f constructionArtifactFixture) ReadArtifact(context.Context, string, string) ([]byte, error) {
	return append([]byte(nil), f.payload...), nil
}

type constructionDiskFixture struct {
	root              string
	stateRoot         string
	reclaimed         bool
	receiptMutator    func(*diskinstantiation.Receipt)
	actualGeometry    diskinstantiation.GeometryReceipt
	inspectCount      int
	postBootAllocated uint64
}

func (f *constructionDiskFixture) Instantiate(ctx context.Context, plan diskinstantiation.Plan, populate diskinstantiation.Populate) (diskinstantiation.Receipt, error) {
	if err := plan.Validate(); err != nil {
		return diskinstantiation.Receipt{}, err
	}
	f.stateRoot = filepath.Join(f.root, "generated")
	if err := os.Mkdir(f.stateRoot, 0o700); err != nil {
		return diskinstantiation.Receipt{}, err
	}
	if err := populate(ctx, f.stateRoot); err != nil {
		return diskinstantiation.Receipt{}, err
	}
	devicePath := filepath.Join(f.root, "data.img")
	if err := os.WriteFile(devicePath, []byte("opaque-device-fixture"), 0o600); err != nil {
		return diskinstantiation.Receipt{}, err
	}
	receipt, err := diskinstantiation.FinalizeReceipt(diskinstantiation.Receipt{
		Backend:       "fixture",
		RealizationID: plan.RealizationID,
		DeviceID:      plan.DeviceID,
		DevicePath:    devicePath,
		Geometry: diskinstantiation.GeometryReceipt{
			FilesystemType:      diskinstantiation.FilesystemExt4,
			FilesystemLabel:     plan.Filesystem.Label,
			PartitionLayout:     diskinstantiation.PartitionLayoutNone,
			DeviceLogicalBytes:  plan.LogicalBytes,
			FilesystemBytes:     plan.LogicalBytes,
			FilesystemBlockSize: plan.Filesystem.BlockSizeBytes,
			FilesystemBlocks:    plan.LogicalBytes / plan.Filesystem.BlockSizeBytes,
			AllocatedBytes:      uint64(len("opaque-device-fixture")),
		},
		CreatedAt: generatorFixedTime,
	})
	if err != nil {
		return diskinstantiation.Receipt{}, err
	}
	f.actualGeometry = receipt.Geometry
	if f.receiptMutator != nil {
		f.receiptMutator(&receipt)
		receipt, err = diskinstantiation.FinalizeReceipt(receipt)
	}
	return receipt, err
}

func (f *constructionDiskFixture) Inspect(context.Context, diskinstantiation.Receipt) (diskinstantiation.GeometryReceipt, error) {
	f.inspectCount++
	geometry := f.actualGeometry
	if f.inspectCount > 1 && f.postBootAllocated != 0 {
		geometry.AllocatedBytes = f.postBootAllocated
	}
	return geometry, nil
}

func (f *constructionDiskFixture) Reclaim(_ context.Context, receipt diskinstantiation.Receipt) (diskinstantiation.ReclaimReceipt, error) {
	f.reclaimed = true
	_ = os.Remove(receipt.DevicePath)
	return diskinstantiation.ReclaimReceipt{InstantiationReceiptID: receipt.ID}, nil
}

type constructionLauncherFixture struct {
	disk       *constructionDiskFixture
	destroyed  bool
	mutateLive bool
	launchErr  error
	destroyErr error
}

func (f *constructionLauncherFixture) Launch(_ context.Context, request ConstructedLaunchRequest) (BootReceipt, error) {
	if request.Disk.DevicePath == "" || request.Version != (ComputerVersion{CodeRef: request.CodeClosure.Ref, ArtifactProgramRef: request.Version.ArtifactProgramRef}) {
		return BootReceipt{}, errors.New("unbound launch request")
	}
	boot := BootReceipt{VMID: request.Identity.RealizationID, HostURL: "http://guest.test", Epoch: 1, Healthy: true, BootedAt: generatorFixedTime}
	if f.launchErr != nil {
		return boot, f.launchErr
	}
	return boot, nil
}

func (f *constructionLauncherFixture) Observe(ctx context.Context, request ConstructedLaunchRequest, boot BootReceipt) (LiveConstructionObservation, error) {
	if request.Identity.RealizationID != boot.VMID || request.Disk.RealizationID != boot.VMID || request.Version != (ComputerVersion{CodeRef: request.CodeClosure.Ref, ArtifactProgramRef: request.Version.ArtifactProgramRef}) {
		return LiveConstructionObservation{}, errors.New("unbound observation request")
	}
	observed, err := ObserveConstructionState(ctx, f.disk.stateRoot, filepath.Join(f.disk.stateRoot, "files"), request.Version)
	if err != nil {
		return LiveConstructionObservation{}, err
	}
	if f.mutateLive && len(observed.Observations) > 0 {
		observed.Observations[0].Value = "seeded-corruption"
	}
	return LiveConstructionObservation{State: observed, Geometry: diskinstantiation.RuntimeGeometryReceipt{
		FilesystemBytes: f.disk.actualGeometry.FilesystemBytes, FilesystemBlockSize: f.disk.actualGeometry.FilesystemBlockSize,
		AvailableBytes: f.disk.actualGeometry.FilesystemBytes / 2,
	}}, nil
}

func (f *constructionLauncherFixture) Commit(context.Context, BootReceipt, ComputerVersion, diskinstantiation.Receipt) error {
	return nil
}

func (f *constructionLauncherFixture) Destroy(context.Context, BootReceipt) error {
	f.destroyed = true
	return f.destroyErr
}

func TestProductionMaterializerConstructsAndReadsBackLiveState(t *testing.T) {
	jrn, blobs, program, version := buildGeneratorFixture(t)
	payload, err := json.Marshal(jrn.Entries())
	if err != nil {
		t.Fatal(err)
	}
	disk := &constructionDiskFixture{root: t.TempDir(), postBootAllocated: 256 << 20}
	launcher := &constructionLauncherFixture{disk: disk}
	materializer := &ProductionMaterializer{
		Identity:  ConstructionIdentity{RealizationID: "candidate-1", ComputerKind: "candidate", OwnerID: "owner", DesktopID: "candidate", CandidateID: "candidate"},
		Inputs:    constructionInputFixture{code: bindTestConstructionCode(t, &version), program: program},
		Artifacts: constructionArtifactFixture{payload: payload},
		Blobs:     blobs,
		Disk:      disk,
		DiskPlan:  testConstructionDiskPlan(),
		Launcher:  launcher,
	}
	manifest := CapabilityManifest{Materializer: ProductionMaterializerName, Substrate: "firecracker/fixture", Supported: []ObservationKind{ObservationFileManifest, ObservationBlobSet, ObservationVMStateManifest}}
	result, err := materializer.Construct(context.Background(), version, manifest)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Equivalence.Equivalent() || !result.Boot.Healthy || result.Disk.ID == "" {
		t.Fatalf("incomplete construction result: %+v", result)
	}
	if disk.reclaimed || launcher.destroyed {
		t.Fatal("accepted realization was torn down")
	}
	if result.Disk.Geometry.AllocatedBytes != 256<<20 || disk.inspectCount != 2 {
		t.Fatalf("post-boot allocation was not joined: disk=%+v inspections=%d", result.Disk.Geometry, disk.inspectCount)
	}
	if got := result.Realization.Observations.RequiredKinds(); len(got) != 3 {
		t.Fatalf("required observation kinds = %v, want file, blob, and VM state", got)
	}
	independentDisk := &constructionDiskFixture{actualGeometry: result.Disk.Geometry}
	verifier := IndependentRealizationVerifier{Inputs: materializer.Inputs, Artifacts: materializer.Artifacts, Blobs: blobs, Disk: independentDisk, Launcher: launcher, Now: func() time.Time { return generatorFixedTime.Add(time.Minute) }}
	verification, err := verifier.Verify(t.Context(), materializer.DiskPlan, result)
	if err != nil {
		t.Fatalf("independent verification: %v", err)
	}
	if err := verification.Validate(); err != nil || verification.Version != version || verification.DiskReceiptID != result.Disk.ID || independentDisk.inspectCount != 1 {
		t.Fatalf("invalid verification receipt: %+v err=%v", verification, err)
	}
	forged := result
	forged.Inputs.Code.Artifacts = nil
	if _, err := verifier.Verify(t.Context(), materializer.DiskPlan, forged); err == nil {
		t.Fatal("independent verifier accepted forged construction inputs")
	}
}

func TestProductionMaterializerRefusesUnverifiedCodeClosureBeforeDiskMutation(t *testing.T) {
	jrn, blobs, program, version := buildGeneratorFixture(t)
	payload, err := json.Marshal(jrn.Entries())
	if err != nil {
		t.Fatal(err)
	}
	disk := &constructionDiskFixture{root: t.TempDir()}
	launcher := &constructionLauncherFixture{disk: disk}
	materializer := &ProductionMaterializer{
		Identity:  ConstructionIdentity{RealizationID: "candidate-invalid-closure", ComputerKind: "candidate"},
		Inputs:    constructionInputFixture{code: CodeClosure{Ref: version.CodeRef, Artifacts: []CodeArtifact{{Name: SandboxRuntimeArtifactName}}}, program: program},
		Artifacts: constructionArtifactFixture{payload: payload}, Blobs: blobs, Disk: disk,
		DiskPlan: testConstructionDiskPlan(), Launcher: launcher,
	}
	manifest := CapabilityManifest{Materializer: ProductionMaterializerName, Substrate: "firecracker/fixture", Supported: []ObservationKind{ObservationFileManifest, ObservationBlobSet, ObservationVMStateManifest}}
	if _, err := materializer.Construct(t.Context(), version, manifest); err == nil || !strings.Contains(err.Error(), "verify immutable CodeRef binding") {
		t.Fatalf("invalid CodeClosure error = %v", err)
	}
	if disk.stateRoot != "" || disk.inspectCount != 0 || launcher.destroyed {
		t.Fatalf("invalid CodeClosure reached mutation: disk=%+v launcher=%+v", disk, launcher)
	}
}

func TestProductionMaterializerDestroysMismatchedReadback(t *testing.T) {
	jrn, blobs, program, version := buildGeneratorFixture(t)
	payload, _ := json.Marshal(jrn.Entries())
	disk := &constructionDiskFixture{root: t.TempDir()}
	launcher := &constructionLauncherFixture{disk: disk, mutateLive: true}
	materializer := &ProductionMaterializer{
		Identity:  ConstructionIdentity{RealizationID: "candidate-corrupt", ComputerKind: "candidate"},
		Inputs:    constructionInputFixture{code: bindTestConstructionCode(t, &version), program: program},
		Artifacts: constructionArtifactFixture{payload: payload},
		Blobs:     blobs,
		Disk:      disk,
		DiskPlan:  testConstructionDiskPlan(),
		Launcher:  launcher,
	}
	manifest := CapabilityManifest{Materializer: ProductionMaterializerName, Substrate: "firecracker/fixture", Supported: []ObservationKind{ObservationFileManifest, ObservationBlobSet, ObservationVMStateManifest}}
	if _, err := materializer.Construct(context.Background(), version, manifest); err == nil {
		t.Fatal("expected mismatched live readback refusal")
	}
	if !disk.reclaimed || !launcher.destroyed {
		t.Fatalf("failed realization cleanup: disk=%v boot=%v", disk.reclaimed, launcher.destroyed)
	}
}

func TestProductionMaterializerRefusesPartiallyReplayedArtifactProgram(t *testing.T) {
	jrn, _, baseProgram, version := buildGeneratorFixture(t)
	unknownDigest := strings.Repeat("a", 64)
	program, err := NewArtifactProgram([]ArtifactProgramEntry{
		{Kind: ArtifactProgramKindBaseJournal, ContentSHA256: baseProgram.Entries[0].ContentSHA256, ArtifactURI: baseProgram.Entries[0].ArtifactURI},
		{Kind: "unsupported-state", ContentSHA256: unknownDigest, ArtifactURI: "artifact+sha256://" + unknownDigest + "/unsupported.json"},
	}, generatorFixedTime)
	if err != nil {
		t.Fatal(err)
	}
	version.ArtifactProgramRef = program.Ref
	payload, err := json.Marshal(jrn.Entries())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := loadVerifiedJournalEntries(context.Background(), constructionArtifactFixture{payload: payload}, program, version); err == nil || !strings.Contains(err.Error(), "unsupported artifact program entry") {
		t.Fatalf("expected partial replay refusal, got %v", err)
	}
}

func TestProductionMaterializerRefusesUnjoinedDiskGeometry(t *testing.T) {
	jrn, blobs, program, version := buildGeneratorFixture(t)
	payload, err := json.Marshal(jrn.Entries())
	if err != nil {
		t.Fatal(err)
	}
	disk := &constructionDiskFixture{root: t.TempDir(), receiptMutator: func(receipt *diskinstantiation.Receipt) {
		receipt.Geometry.DeviceLogicalBytes /= 2
		receipt.Geometry.FilesystemBytes /= 2
		receipt.Geometry.FilesystemBlocks /= 2
	}}
	launcher := &constructionLauncherFixture{disk: disk}
	materializer := &ProductionMaterializer{
		Identity:  ConstructionIdentity{RealizationID: "candidate-bad-geometry", ComputerKind: "candidate"},
		Inputs:    constructionInputFixture{code: bindTestConstructionCode(t, &version), program: program},
		Artifacts: constructionArtifactFixture{payload: payload}, Blobs: blobs, Disk: disk,
		DiskPlan: testConstructionDiskPlan(), Launcher: launcher,
	}
	manifest := CapabilityManifest{Materializer: ProductionMaterializerName, Substrate: "firecracker/fixture", Supported: []ObservationKind{ObservationFileManifest, ObservationBlobSet, ObservationVMStateManifest}}
	if _, err := materializer.Construct(context.Background(), version, manifest); err == nil {
		t.Fatal("expected disk geometry refusal")
	}
	if !disk.reclaimed {
		t.Fatal("unjoined disk was not reclaimed")
	}
}

func TestObserveConstructionStateRefusesSymlinkedParent(t *testing.T) {
	deviceRoot := t.TempDir()
	filesRoot := filepath.Join(deviceRoot, "files")
	if err := os.MkdirAll(filepath.Join(filesRoot, "safe"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filesRoot, "safe", "proof.txt"), []byte("expected"), 0o644); err != nil {
		t.Fatal(err)
	}
	version := ComputerVersion{CodeRef: "code:test", ArtifactProgramRef: "program:test"}
	expected, err := FilesystemProjectionObservationSet(context.Background(), "expected", version, filesRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteConstructionStateManifest(deviceRoot, version, expected); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "proof.txt"), []byte("outside"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(filesRoot, "safe")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(filesRoot, "safe")); err != nil {
		t.Fatal(err)
	}
	if _, err := ObserveConstructionState(context.Background(), deviceRoot, filesRoot, version); err == nil {
		t.Fatal("expected symlinked parent refusal")
	}
}

func testConstructionDiskPlan() diskinstantiation.Plan {
	return diskinstantiation.Plan{
		DeviceID:     "data",
		LogicalBytes: 32 << 30,
		Filesystem: diskinstantiation.FilesystemContract{
			Type: diskinstantiation.FilesystemExt4, Label: "choir-data", BlockSizeBytes: 4096,
		},
		Allocation: diskinstantiation.AllocationContract{Mode: diskinstantiation.AllocationSparse, MaxAllocatedBytes: 2 << 30, MinimumAvailableBytes: 2 << 30},
	}
}

func TestProductionMaterializerRetainsDiskWhenVMCleanupIsUncertain(t *testing.T) {
	for _, tc := range []struct {
		name       string
		launchErr  error
		destroyErr error
		mutateLive bool
	}{
		{name: "launch-cleanup", launchErr: errors.New("unregistered VM cleanup failed")},
		{name: "destroy-cleanup", destroyErr: errors.New("stop failed"), mutateLive: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			jrn, blobs, program, version := buildGeneratorFixture(t)
			payload, _ := json.Marshal(jrn.Entries())
			disk := &constructionDiskFixture{root: t.TempDir()}
			launcher := &constructionLauncherFixture{disk: disk, launchErr: tc.launchErr, destroyErr: tc.destroyErr, mutateLive: tc.mutateLive}
			materializer := &ProductionMaterializer{Identity: ConstructionIdentity{RealizationID: "candidate-cleanup", ComputerKind: "candidate"}, Inputs: constructionInputFixture{code: bindTestConstructionCode(t, &version), program: program}, Artifacts: constructionArtifactFixture{payload: payload}, Blobs: blobs, Disk: disk, DiskPlan: testConstructionDiskPlan(), Launcher: launcher}
			manifest := CapabilityManifest{Materializer: ProductionMaterializerName, Substrate: "firecracker/fixture", Supported: []ObservationKind{ObservationFileManifest, ObservationBlobSet, ObservationVMStateManifest}}
			if _, err := materializer.Construct(t.Context(), version, manifest); err == nil {
				t.Fatal("expected cleanup uncertainty refusal")
			}
			if disk.reclaimed {
				t.Fatal("backing disk reclaimed while VM cleanup remained uncertain")
			}
		})
	}
}
