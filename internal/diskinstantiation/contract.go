package diskinstantiation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

const (
	FilesystemExt4      = "ext4"
	AllocationSparse    = "sparse"
	PartitionLayoutNone = "none"
)

// Plan is a substrate-neutral request for one realization-local block device.
// It deliberately contains no ComputerVersion, semantic artifact, or route data.
type Plan struct {
	RealizationID string             `json:"realization_id"`
	DeviceID      string             `json:"device_id"`
	LogicalBytes  uint64             `json:"logical_bytes"`
	Filesystem    FilesystemContract `json:"filesystem"`
	Allocation    AllocationContract `json:"allocation"`
}

type FilesystemContract struct {
	Type           string `json:"type"`
	Label          string `json:"label"`
	BlockSizeBytes uint64 `json:"block_size_bytes"`
	ReservedPct    uint8  `json:"reserved_percent"`
}

type AllocationContract struct {
	Mode                  string `json:"mode"`
	MaxAllocatedBytes     uint64 `json:"max_allocated_bytes"`
	MinimumAvailableBytes uint64 `json:"minimum_available_bytes"`
}

type GeometryReceipt struct {
	FilesystemType      string `json:"filesystem_type"`
	FilesystemLabel     string `json:"filesystem_label"`
	PartitionLayout     string `json:"partition_layout"`
	PartitionOffset     uint64 `json:"partition_offset_bytes"`
	DeviceLogicalBytes  uint64 `json:"device_logical_bytes"`
	FilesystemBytes     uint64 `json:"filesystem_bytes"`
	FilesystemBlockSize uint64 `json:"filesystem_block_size_bytes"`
	FilesystemBlocks    uint64 `json:"filesystem_blocks"`
	AllocatedBytes      uint64 `json:"allocated_bytes"`
}

type Receipt struct {
	ID            string          `json:"receipt_id"`
	Backend       string          `json:"backend"`
	RealizationID string          `json:"realization_id"`
	DeviceID      string          `json:"device_id"`
	DevicePath    string          `json:"device_path"`
	Geometry      GeometryReceipt `json:"geometry"`
	CreatedAt     time.Time       `json:"created_at"`
}

type RuntimeGeometryReceipt struct {
	FilesystemBytes     uint64 `json:"filesystem_bytes"`
	FilesystemBlockSize uint64 `json:"filesystem_block_size_bytes"`
	AvailableBytes      uint64 `json:"available_bytes"`
}

type ReclaimReceipt struct {
	InstantiationReceiptID string    `json:"instantiation_receipt_id"`
	ReclaimedBytes         uint64    `json:"reclaimed_bytes"`
	ReclaimedAt            time.Time `json:"reclaimed_at"`
}

// Populate writes substrate-independent filesystem state below root. Backends
// may transport that state into any conforming block-device implementation but
// must not interpret it as ComputerVersion authority.
type Populate func(ctx context.Context, root string) error

// Backend owns only realization-local device creation, geometry, allocation,
// and reclaim. It cannot publish routes or resolve semantic computer state.
type Backend interface {
	Instantiate(context.Context, Plan, Populate) (Receipt, error)
	Inspect(context.Context, Receipt) (GeometryReceipt, error)
	Reclaim(context.Context, Receipt) (ReclaimReceipt, error)
}

func (p Plan) Validate() error {
	if strings.TrimSpace(p.RealizationID) == "" || strings.TrimSpace(p.DeviceID) == "" {
		return fmt.Errorf("disk instantiation: realization id and device id are required")
	}
	if filepath.Base(p.RealizationID) != p.RealizationID || filepath.Base(p.DeviceID) != p.DeviceID || p.RealizationID == "." || p.RealizationID == ".." || p.DeviceID == "." || p.DeviceID == ".." {
		return fmt.Errorf("disk instantiation: realization id and device id must be path-safe")
	}
	if p.LogicalBytes == 0 {
		return fmt.Errorf("disk instantiation: logical size is required")
	}
	if p.Filesystem.Type != FilesystemExt4 {
		return fmt.Errorf("disk instantiation: unsupported filesystem %q", p.Filesystem.Type)
	}
	if strings.TrimSpace(p.Filesystem.Label) == "" {
		return fmt.Errorf("disk instantiation: filesystem label is required")
	}
	if p.Filesystem.BlockSizeBytes != 4096 {
		return fmt.Errorf("disk instantiation: ext4 block size must be 4096 bytes")
	}
	if p.Filesystem.ReservedPct > 5 {
		return fmt.Errorf("disk instantiation: ext4 reserved percentage exceeds policy")
	}
	if p.Allocation.Mode != AllocationSparse || p.Allocation.MaxAllocatedBytes == 0 || p.Allocation.MinimumAvailableBytes == 0 || p.Allocation.MinimumAvailableBytes >= p.LogicalBytes {
		return fmt.Errorf("disk instantiation: bounded sparse allocation and minimum headroom are required")
	}
	if p.Allocation.MaxAllocatedBytes >= p.LogicalBytes {
		return fmt.Errorf("disk instantiation: allocation bound must be below logical size")
	}
	return nil
}

func VerifyReceipt(plan Plan, receipt Receipt) error {
	if err := plan.Validate(); err != nil {
		return err
	}
	finalized, err := FinalizeReceipt(receipt)
	if err != nil || receipt.ID == "" || finalized.ID != receipt.ID {
		return fmt.Errorf("disk instantiation: receipt integrity check failed")
	}
	geometry := receipt.Geometry
	if receipt.RealizationID != plan.RealizationID || receipt.DeviceID != plan.DeviceID {
		return fmt.Errorf("disk instantiation: receipt identity does not match plan")
	}
	if geometry.FilesystemType != plan.Filesystem.Type || geometry.FilesystemLabel != plan.Filesystem.Label {
		return fmt.Errorf("disk instantiation: filesystem identity does not match plan")
	}
	if geometry.PartitionLayout != PartitionLayoutNone || geometry.PartitionOffset != 0 {
		return fmt.Errorf("disk instantiation: unexpected partition geometry")
	}
	if geometry.DeviceLogicalBytes != plan.LogicalBytes || geometry.FilesystemBlockSize != plan.Filesystem.BlockSizeBytes {
		return fmt.Errorf("disk instantiation: device or block geometry does not match plan")
	}
	if geometry.FilesystemBlocks > ^uint64(0)/geometry.FilesystemBlockSize || geometry.FilesystemBlocks*geometry.FilesystemBlockSize != geometry.FilesystemBytes {
		return fmt.Errorf("disk instantiation: incoherent filesystem geometry")
	}
	if geometry.FilesystemBytes != plan.LogicalBytes {
		return fmt.Errorf("disk instantiation: filesystem does not span the planned device")
	}
	if geometry.AllocatedBytes > plan.Allocation.MaxAllocatedBytes {
		return fmt.Errorf("disk instantiation: allocated bytes exceed policy bound")
	}
	return nil
}

func VerifyReceiptIntegrity(receipt Receipt) error {
	finalized, err := FinalizeReceipt(receipt)
	if err != nil {
		return err
	}
	if receipt.ID == "" || finalized.ID != receipt.ID {
		return fmt.Errorf("disk instantiation: receipt integrity mismatch")
	}
	return nil
}

func RefreshAllocatedGeometry(plan Plan, receipt Receipt, inspected GeometryReceipt) (Receipt, error) {
	expected := receipt.Geometry
	expected.AllocatedBytes = inspected.AllocatedBytes
	if inspected != expected {
		return Receipt{}, fmt.Errorf("disk instantiation: post-boot geometry changed outside allocation")
	}
	refreshed := receipt
	refreshed.Geometry = inspected
	refreshed, err := FinalizeReceipt(refreshed)
	if err != nil {
		return Receipt{}, err
	}
	if err := VerifyReceipt(plan, refreshed); err != nil {
		return Receipt{}, err
	}
	return refreshed, nil
}

func VerifyRuntimeGeometry(plan Plan, disk Receipt, runtime RuntimeGeometryReceipt) error {
	if err := VerifyReceipt(plan, disk); err != nil {
		return err
	}
	if runtime.FilesystemBlockSize != disk.Geometry.FilesystemBlockSize || runtime.FilesystemBytes == 0 || runtime.FilesystemBytes > disk.Geometry.FilesystemBytes {
		return fmt.Errorf("disk instantiation: guest statfs geometry does not match device receipt")
	}
	// Linux ext4 statfs reports usable blocks rather than the raw filesystem
	// span recorded by dumpe2fs. Bound metadata overhead while still refusing a
	// 16 GiB filesystem presented as a 32 GiB device.
	minimumUsable := disk.Geometry.FilesystemBytes - disk.Geometry.FilesystemBytes/20
	if runtime.FilesystemBytes < minimumUsable {
		return fmt.Errorf("disk instantiation: guest statfs capacity is below the planned filesystem bound")
	}
	if runtime.AvailableBytes > runtime.FilesystemBytes || runtime.AvailableBytes < plan.Allocation.MinimumAvailableBytes {
		return fmt.Errorf("disk instantiation: guest statfs available bytes violate headroom policy")
	}
	return nil
}

func FinalizeReceipt(receipt Receipt) (Receipt, error) {
	if strings.TrimSpace(receipt.Backend) == "" || strings.TrimSpace(receipt.DevicePath) == "" {
		return Receipt{}, fmt.Errorf("disk instantiation: incomplete receipt")
	}
	if !filepath.IsAbs(receipt.DevicePath) {
		return Receipt{}, fmt.Errorf("disk instantiation: device path must be absolute")
	}
	if receipt.Geometry.DeviceLogicalBytes == 0 || receipt.Geometry.FilesystemBytes == 0 {
		return Receipt{}, fmt.Errorf("disk instantiation: geometry is incomplete")
	}
	if receipt.Geometry.FilesystemBytes > receipt.Geometry.DeviceLogicalBytes {
		return Receipt{}, fmt.Errorf("disk instantiation: filesystem exceeds device")
	}
	if receipt.CreatedAt.IsZero() {
		return Receipt{}, fmt.Errorf("disk instantiation: creation time is required")
	}
	receipt.CreatedAt = receipt.CreatedAt.UTC()
	receipt.ID = ""
	encoded, err := json.Marshal(receipt)
	if err != nil {
		return Receipt{}, fmt.Errorf("disk instantiation: encode receipt: %w", err)
	}
	digest := sha256.Sum256(encoded)
	receipt.ID = "disk-instantiation:sha256:" + hex.EncodeToString(digest[:])
	return receipt, nil
}
