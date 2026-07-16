package diskinstantiation

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const Ext4BackendName = "linux-ext4-sparse-v1"

type Ext4Backend struct {
	WorkRoot       string
	MkfsBinary     string
	Dumpe2fsBinary string
	Now            func() time.Time
}

func (b Ext4Backend) Instantiate(ctx context.Context, plan Plan, populate Populate) (Receipt, error) {
	if err := plan.Validate(); err != nil {
		return Receipt{}, err
	}
	if populate == nil {
		return Receipt{}, fmt.Errorf("disk instantiation: populate function is required")
	}
	root, err := filepath.Abs(strings.TrimSpace(b.WorkRoot))
	if err != nil || strings.TrimSpace(b.WorkRoot) == "" {
		return Receipt{}, fmt.Errorf("disk instantiation: absolute work root is required")
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return Receipt{}, fmt.Errorf("disk instantiation: create work root: %w", err)
	}
	realizationDir := filepath.Join(root, plan.RealizationID)
	if err := os.Mkdir(realizationDir, 0o700); err != nil {
		if errors.Is(err, os.ErrExist) {
			return Receipt{}, fmt.Errorf("disk instantiation: realization %q already exists", plan.RealizationID)
		}
		return Receipt{}, fmt.Errorf("disk instantiation: create realization: %w", err)
	}
	keep := false
	defer func() {
		if !keep {
			_ = os.RemoveAll(realizationDir)
		}
	}()

	stagingRoot, err := os.MkdirTemp(root, ".populate-")
	if err != nil {
		return Receipt{}, fmt.Errorf("disk instantiation: create population root: %w", err)
	}
	defer os.RemoveAll(stagingRoot)
	if err := populate(ctx, stagingRoot); err != nil {
		return Receipt{}, fmt.Errorf("disk instantiation: populate filesystem: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return Receipt{}, err
	}

	devicePath := filepath.Join(realizationDir, plan.DeviceID+".img")
	file, err := os.OpenFile(devicePath, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
	if err != nil {
		return Receipt{}, fmt.Errorf("disk instantiation: create device: %w", err)
	}
	if err := file.Truncate(int64(plan.LogicalBytes)); err != nil {
		_ = file.Close()
		return Receipt{}, fmt.Errorf("disk instantiation: size device: %w", err)
	}
	if err := file.Close(); err != nil {
		return Receipt{}, fmt.Errorf("disk instantiation: close device: %w", err)
	}

	mkfs := strings.TrimSpace(b.MkfsBinary)
	if mkfs == "" {
		mkfs = "mkfs.ext4"
	}
	args := []string{
		"-F", "-q", "-b", strconv.FormatUint(plan.Filesystem.BlockSizeBytes, 10),
		"-m", strconv.FormatUint(uint64(plan.Filesystem.ReservedPct), 10),
		"-E", "lazy_itable_init=1,lazy_journal_init=1",
		"-d", stagingRoot,
	}
	if label := strings.TrimSpace(plan.Filesystem.Label); label != "" {
		args = append(args, "-L", label)
	}
	args = append(args, devicePath)
	if output, err := exec.CommandContext(ctx, mkfs, args...).CombinedOutput(); err != nil {
		return Receipt{}, fmt.Errorf("disk instantiation: format ext4: %w: %s", err, strings.TrimSpace(string(output)))
	}

	now := time.Now
	if b.Now != nil {
		now = b.Now
	}
	receipt := Receipt{
		Backend:       Ext4BackendName,
		RealizationID: plan.RealizationID,
		DeviceID:      plan.DeviceID,
		DevicePath:    devicePath,
		CreatedAt:     now().UTC(),
	}
	geometry, err := b.Inspect(ctx, receipt)
	if err != nil {
		return Receipt{}, err
	}
	if geometry.AllocatedBytes > plan.Allocation.MaxAllocatedBytes {
		return Receipt{}, fmt.Errorf("disk instantiation: allocated bytes %d exceed bound %d", geometry.AllocatedBytes, plan.Allocation.MaxAllocatedBytes)
	}
	receipt.Geometry = geometry
	receipt, err = FinalizeReceipt(receipt)
	if err != nil {
		return Receipt{}, err
	}
	if err := VerifyReceipt(plan, receipt); err != nil {
		return Receipt{}, err
	}
	keep = true
	return receipt, nil
}

func (b Ext4Backend) Inspect(ctx context.Context, receipt Receipt) (GeometryReceipt, error) {
	if receipt.Backend != "" && receipt.Backend != Ext4BackendName {
		return GeometryReceipt{}, fmt.Errorf("disk instantiation: receipt backend mismatch")
	}
	if err := b.authorizeDevicePath(receipt.DevicePath); err != nil {
		return GeometryReceipt{}, err
	}
	info, err := os.Stat(receipt.DevicePath)
	if err != nil {
		return GeometryReceipt{}, fmt.Errorf("disk instantiation: stat device: %w", err)
	}
	allocated, err := allocatedBytes(info)
	if err != nil {
		return GeometryReceipt{}, err
	}
	dumpe2fs := strings.TrimSpace(b.Dumpe2fsBinary)
	if dumpe2fs == "" {
		dumpe2fs = "dumpe2fs"
	}
	output, err := exec.CommandContext(ctx, dumpe2fs, "-h", receipt.DevicePath).CombinedOutput()
	if err != nil {
		return GeometryReceipt{}, fmt.Errorf("disk instantiation: inspect ext4: %w: %s", err, strings.TrimSpace(string(output)))
	}
	fields := parseDumpe2fsHeader(string(output))
	blocks, err := requiredUintField(fields, "Block count")
	if err != nil {
		return GeometryReceipt{}, err
	}
	blockSize, err := requiredUintField(fields, "Block size")
	if err != nil {
		return GeometryReceipt{}, err
	}
	if blocks > ^uint64(0)/blockSize {
		return GeometryReceipt{}, fmt.Errorf("disk instantiation: filesystem geometry overflows")
	}
	return GeometryReceipt{
		FilesystemType:      FilesystemExt4,
		FilesystemLabel:     fields["Filesystem volume name"],
		PartitionLayout:     PartitionLayoutNone,
		PartitionOffset:     0,
		DeviceLogicalBytes:  uint64(info.Size()),
		FilesystemBytes:     blocks * blockSize,
		FilesystemBlockSize: blockSize,
		FilesystemBlocks:    blocks,
		AllocatedBytes:      allocated,
	}, nil
}

func (b Ext4Backend) Reclaim(ctx context.Context, receipt Receipt) (ReclaimReceipt, error) {
	if err := VerifyReceiptIntegrity(receipt); err != nil {
		return ReclaimReceipt{}, fmt.Errorf("disk instantiation: refuse unverified reclaim receipt: %w", err)
	}
	if err := b.authorizeDevicePath(receipt.DevicePath); err != nil {
		return ReclaimReceipt{}, err
	}
	realizationDir := filepath.Dir(receipt.DevicePath)
	if filepath.Base(realizationDir) != receipt.RealizationID || filepath.Base(receipt.DevicePath) != receipt.DeviceID+".img" {
		return ReclaimReceipt{}, fmt.Errorf("disk instantiation: reclaim path does not match receipt identity")
	}
	if err := ctx.Err(); err != nil {
		return ReclaimReceipt{}, err
	}
	bytesReleased := uint64(0)
	info, err := os.Stat(receipt.DevicePath)
	if err == nil {
		if !info.Mode().IsRegular() {
			return ReclaimReceipt{}, fmt.Errorf("disk instantiation: reclaim target is not a regular file")
		}
		physicalBytes, allocationErr := allocatedBytes(info)
		if allocationErr != nil {
			return ReclaimReceipt{}, fmt.Errorf("disk instantiation: measure reclaim allocation: %w", allocationErr)
		}
		bytesReleased = physicalBytes
		if err := os.Remove(receipt.DevicePath); err != nil {
			return ReclaimReceipt{}, fmt.Errorf("disk instantiation: reclaim device: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return ReclaimReceipt{}, fmt.Errorf("disk instantiation: stat reclaim target: %w", err)
	}
	if err := os.Remove(realizationDir); err != nil && !errors.Is(err, os.ErrNotExist) {
		return ReclaimReceipt{}, fmt.Errorf("disk instantiation: reclaim realization directory: %w", err)
	}
	now := b.Now
	if now == nil {
		now = time.Now
	}
	return ReclaimReceipt{InstantiationReceiptID: receipt.ID, ReclaimedAt: now().UTC(), ReclaimedBytes: bytesReleased}, nil
}

func (b Ext4Backend) authorizeDevicePath(path string) error {
	root, err := filepath.Abs(strings.TrimSpace(b.WorkRoot))
	if err != nil || strings.TrimSpace(b.WorkRoot) == "" {
		return fmt.Errorf("disk instantiation: absolute work root is required")
	}
	candidate, err := filepath.Abs(strings.TrimSpace(path))
	if err != nil || strings.TrimSpace(path) == "" {
		return fmt.Errorf("disk instantiation: absolute device path is required")
	}
	rel, err := filepath.Rel(root, candidate)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("disk instantiation: device path escapes work root")
	}
	return nil
}

func allocatedBytes(info os.FileInfo) (uint64, error) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Blocks < 0 {
		return 0, fmt.Errorf("disk instantiation: allocated block accounting unavailable")
	}
	return uint64(stat.Blocks) * 512, nil
}

func parseDumpe2fsHeader(output string) map[string]string {
	fields := make(map[string]string)
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		key, value, ok := strings.Cut(scanner.Text(), ":")
		if ok {
			fields[strings.TrimSpace(key)] = strings.TrimSpace(value)
		}
	}
	return fields
}

func requiredUintField(fields map[string]string, name string) (uint64, error) {
	value := strings.TrimSpace(fields[name])
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil || parsed == 0 {
		return 0, fmt.Errorf("disk instantiation: invalid ext4 %s %q", strings.ToLower(name), value)
	}
	return parsed, nil
}
