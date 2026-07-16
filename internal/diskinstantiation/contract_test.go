package diskinstantiation

import (
	"strings"
	"testing"
	"time"
)

func testPlan() Plan {
	return Plan{
		RealizationID: "vm-candidate-1",
		DeviceID:      "data",
		LogicalBytes:  32 << 30,
		Filesystem: FilesystemContract{
			Type:           FilesystemExt4,
			Label:          "choir-data",
			BlockSizeBytes: 4096,
			ReservedPct:    0,
		},
		Allocation: AllocationContract{Mode: AllocationSparse, MaxAllocatedBytes: 2 << 30, MinimumAvailableBytes: 2 << 30},
	}
}

func TestPlanRequiresBoundedSparseGeometry(t *testing.T) {
	plan := testPlan()
	if err := plan.Validate(); err != nil {
		t.Fatal(err)
	}

	for name, mutate := range map[string]func(*Plan){
		"unbounded": func(p *Plan) { p.Allocation.MaxAllocatedBytes = 0 },
		"eager":     func(p *Plan) { p.Allocation.Mode = "eager" },
		"raw":       func(p *Plan) { p.Filesystem.Type = "raw" },
		"unsafe id": func(p *Plan) { p.RealizationID = "../owner" },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := plan
			mutate(&candidate)
			if err := candidate.Validate(); err == nil {
				t.Fatal("expected plan refusal")
			}
		})
	}
}

func TestFinalizeReceiptBindsGeometryAndIdentity(t *testing.T) {
	receipt := Receipt{
		Backend:       Ext4BackendName,
		RealizationID: "vm-candidate-1",
		DeviceID:      "data",
		DevicePath:    "/var/lib/go-choir/vm-state/vm-candidate-1/data.img",
		Geometry: GeometryReceipt{
			FilesystemType:      FilesystemExt4,
			PartitionLayout:     PartitionLayoutNone,
			DeviceLogicalBytes:  32 << 30,
			FilesystemBytes:     32 << 30,
			FilesystemBlockSize: 4096,
			FilesystemBlocks:    (32 << 30) / 4096,
			AllocatedBytes:      128 << 20,
		},
		CreatedAt: time.Date(2026, 7, 16, 9, 0, 0, 0, time.FixedZone("test", 3600)),
	}
	first, err := FinalizeReceipt(receipt)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(first.ID, "disk-instantiation:sha256:") || first.CreatedAt.Location() != time.UTC {
		t.Fatalf("unexpected finalized receipt: %+v", first)
	}
	second, err := FinalizeReceipt(first)
	if err != nil {
		t.Fatal(err)
	}
	if second.ID != first.ID {
		t.Fatalf("receipt id is not stable: %q != %q", second.ID, first.ID)
	}
	mutated := first
	mutated.Geometry.AllocatedBytes++
	mutated, err = FinalizeReceipt(mutated)
	if err != nil {
		t.Fatal(err)
	}
	if mutated.ID == first.ID {
		t.Fatal("geometry mutation did not change receipt identity")
	}
}

func TestFinalizeReceiptRefusesImpossibleGeometry(t *testing.T) {
	receipt := Receipt{
		Backend:       Ext4BackendName,
		RealizationID: "vm-candidate-1",
		DeviceID:      "data",
		DevicePath:    "/tmp/data.img",
		Geometry: GeometryReceipt{
			DeviceLogicalBytes: 1,
			FilesystemBytes:    2,
		},
		CreatedAt: time.Now(),
	}
	if _, err := FinalizeReceipt(receipt); err == nil {
		t.Fatal("expected impossible geometry refusal")
	}
}

func TestVerifyReceiptJoinsExactPlanGeometry(t *testing.T) {
	plan := testPlan()
	receipt, err := FinalizeReceipt(Receipt{
		Backend: Ext4BackendName, RealizationID: plan.RealizationID, DeviceID: plan.DeviceID,
		DevicePath: "/tmp/data.img", CreatedAt: time.Now(),
		Geometry: GeometryReceipt{
			FilesystemType: FilesystemExt4, FilesystemLabel: plan.Filesystem.Label,
			PartitionLayout: PartitionLayoutNone, DeviceLogicalBytes: plan.LogicalBytes,
			FilesystemBytes: plan.LogicalBytes, FilesystemBlockSize: plan.Filesystem.BlockSizeBytes,
			FilesystemBlocks: plan.LogicalBytes / plan.Filesystem.BlockSizeBytes, AllocatedBytes: 128 << 20,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyReceipt(plan, receipt); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*Receipt){
		"logical size": func(r *Receipt) { r.Geometry.DeviceLogicalBytes /= 2 },
		"filesystem span": func(r *Receipt) {
			r.Geometry.FilesystemBytes -= r.Geometry.FilesystemBlockSize
			r.Geometry.FilesystemBlocks--
		},
		"label":      func(r *Receipt) { r.Geometry.FilesystemLabel = "wrong" },
		"allocation": func(r *Receipt) { r.Geometry.AllocatedBytes = plan.Allocation.MaxAllocatedBytes + 1 },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := receipt
			mutate(&candidate)
			candidate, err = FinalizeReceipt(candidate)
			if err != nil {
				return
			}
			if err := VerifyReceipt(plan, candidate); err == nil {
				t.Fatal("expected geometry mismatch refusal")
			}
		})
	}
}

func TestVerifyRuntimeGeometryJoinsGuestStatfs(t *testing.T) {
	plan := testPlan()
	receipt, err := FinalizeReceipt(Receipt{
		Backend: Ext4BackendName, RealizationID: plan.RealizationID, DeviceID: plan.DeviceID,
		DevicePath: "/tmp/data.img", CreatedAt: time.Now(),
		Geometry: GeometryReceipt{FilesystemType: FilesystemExt4, FilesystemLabel: plan.Filesystem.Label,
			PartitionLayout: PartitionLayoutNone, DeviceLogicalBytes: plan.LogicalBytes, FilesystemBytes: plan.LogicalBytes,
			FilesystemBlockSize: plan.Filesystem.BlockSizeBytes, FilesystemBlocks: plan.LogicalBytes / plan.Filesystem.BlockSizeBytes,
			AllocatedBytes: 128 << 20},
	})
	if err != nil {
		t.Fatal(err)
	}
	runtime := RuntimeGeometryReceipt{FilesystemBytes: plan.LogicalBytes, FilesystemBlockSize: 4096, AvailableBytes: 31 << 30}
	if err := VerifyRuntimeGeometry(plan, receipt, runtime); err != nil {
		t.Fatal(err)
	}
	runtime.FilesystemBytes = 16 << 30
	if err := VerifyRuntimeGeometry(plan, receipt, runtime); err == nil {
		t.Fatal("expected 32 GiB versus 16 GiB statfs refusal")
	}
}

func TestRefreshAllocatedGeometryEnforcesPostBootBound(t *testing.T) {
	plan := testPlan()
	receipt, err := FinalizeReceipt(Receipt{Backend: Ext4BackendName, RealizationID: plan.RealizationID, DeviceID: plan.DeviceID, DevicePath: "/tmp/data.img", CreatedAt: time.Now(), Geometry: GeometryReceipt{FilesystemType: FilesystemExt4, FilesystemLabel: plan.Filesystem.Label, PartitionLayout: PartitionLayoutNone, DeviceLogicalBytes: plan.LogicalBytes, FilesystemBytes: plan.LogicalBytes, FilesystemBlockSize: 4096, FilesystemBlocks: plan.LogicalBytes / 4096, AllocatedBytes: 128 << 20}})
	if err != nil {
		t.Fatal(err)
	}
	postBoot := receipt.Geometry
	postBoot.AllocatedBytes = 256 << 20
	refreshed, err := RefreshAllocatedGeometry(plan, receipt, postBoot)
	if err != nil || refreshed.Geometry.AllocatedBytes != 256<<20 || refreshed.ID == receipt.ID {
		t.Fatalf("valid post-boot allocation refresh failed: receipt=%+v err=%v", refreshed, err)
	}
	postBoot.AllocatedBytes = plan.Allocation.MaxAllocatedBytes + 1
	if _, err := RefreshAllocatedGeometry(plan, receipt, postBoot); err == nil {
		t.Fatal("expected post-boot allocation bound refusal")
	}
}
