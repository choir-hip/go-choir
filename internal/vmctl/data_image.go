package vmctl

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/yusefmosiah/go-choir/internal/vmmanager"
)

const dataImageFileName = "data.img"

// DataImageStats reports host-side persistent disk image stats for a VM.
// FileBytes is allocated host storage, not the image's virtual capacity.
// CapBytes is the virtual data.img size exposed to the guest filesystem.
type DataImageStats struct {
	FileBytes     uint64 `json:"file_bytes"`
	CapBytes      uint64 `json:"cap_bytes"`
	StateDirBytes uint64 `json:"state_dir_bytes"`
}

type dataImageResponse struct {
	FileBytes     uint64 `json:"file_bytes"`
	CapBytes      uint64 `json:"cap_bytes"`
	StateDirBytes uint64 `json:"state_dir_bytes"`
}

// LookupDataImageStats returns host-side data image stats when stateDir/vmID are valid
// and data.img exists.
func LookupDataImageStats(stateDir, vmID string) (DataImageStats, bool) {
	vmID = strings.TrimSpace(vmID)
	stateDir = strings.TrimSpace(stateDir)
	if vmID == "" || stateDir == "" {
		return DataImageStats{}, false
	}
	root := filepath.Clean(stateDir)
	if root == "." || root == string(os.PathSeparator) {
		return DataImageStats{}, false
	}
	vmDir := filepath.Clean(filepath.Join(root, vmID))
	if vmDir == root || !strings.HasPrefix(vmDir, root+string(os.PathSeparator)) {
		return DataImageStats{}, false
	}
	dataImg := filepath.Join(vmDir, dataImageFileName)
	info, err := os.Stat(dataImg)
	if err != nil {
		return DataImageStats{}, false
	}
	capBytes := uint64(info.Size())
	if capBytes == 0 {
		return DataImageStats{}, false
	}
	stateDirBytes := uint64(vmStateDirUsageBytes(stateDir, vmID))
	return DataImageStats{
		FileBytes:     stateDirBytes,
		CapBytes:      capBytes,
		StateDirBytes: stateDirBytes,
	}, true
}

// DataImageStateDir is the directory holding each VM's data image.
func (r *OwnershipRegistry) DataImageStateDir() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return normalizePressureReclaimConfig(r.pressureReclaim).StateDir
}

// DataImageStatsForVM returns stats using the registry pressure-reclaim state dir.
func (r *OwnershipRegistry) DataImageStatsForVM(vmID string) (DataImageStats, bool) {
	cfg := normalizePressureReclaimConfig(r.pressureReclaim)
	return LookupDataImageStats(cfg.StateDir, vmID)
}

func dataImageResponseFromStats(stats DataImageStats) *dataImageResponse {
	if stats.CapBytes == 0 {
		return nil
	}
	return &dataImageResponse{
		FileBytes:     stats.FileBytes,
		CapBytes:      stats.CapBytes,
		StateDirBytes: stats.StateDirBytes,
	}
}

func vmStateDirUsageBytes(stateDir, vmID string) int64 {
	vmID = strings.TrimSpace(vmID)
	if vmID == "" {
		return 0
	}
	root := filepath.Clean(stateDir)
	if root == "." || root == string(os.PathSeparator) {
		return 0
	}
	dir := filepath.Clean(filepath.Join(root, vmID))
	if dir == root || !strings.HasPrefix(dir, root+string(os.PathSeparator)) {
		return 0
	}
	var total int64
	_ = filepath.WalkDir(dir, func(_ string, d os.DirEntry, err error) error {
		if err != nil || d == nil {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		if stat, ok := info.Sys().(*syscall.Stat_t); ok && stat.Blocks > 0 {
			total += stat.Blocks * 512
		} else if info.Mode().IsRegular() {
			total += info.Size()
		}
		return nil
	})
	return total
}

// vmStateDirDiskBytes is the bytes a VM's state directory occupies on disk,
// counting only what is measured exactly. Each regular file is measured by
// measure (vmmanager.FileDiskBytes in production), and counts only when the
// measure is the exact btrfs extent reading; a file measured any other way
// (the FIEMAP bound, st_blocks, an error) counts 0. The snapshot directory is
// counted from its meta.json (disk_bytes, written by the snapshot budget's
// measure after the snapshot is fsynced), again only when that reading was the
// exact one, and not at all when skipSnapshot (the snapshot is about to be
// consumed). Counting less than the disk holds makes the room hold more back,
// which is the safe direction: the owners file is subtracted from the
// footprint, so a figure that is too high would open a place.
func vmStateDirDiskBytes(stateDir, vmID string, measure func(path string) (int64, string, error), skipSnapshot bool) (int64, int) {
	vmID = strings.TrimSpace(vmID)
	if vmID == "" {
		return 0, 0
	}
	root := filepath.Clean(stateDir)
	if root == "." || root == string(os.PathSeparator) {
		return 0, 0
	}
	dir := filepath.Clean(filepath.Join(root, vmID))
	if dir == root || !strings.HasPrefix(dir, root+string(os.PathSeparator)) {
		return 0, 0
	}
	snapshotDir := filepath.Join(dir, "snapshot")
	var total int64
	inexact := 0
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d == nil {
			return nil
		}
		if d.IsDir() {
			if path == snapshotDir {
				if !skipSnapshot {
					n, ok := snapshotMetaDiskBytes(filepath.Join(snapshotDir, "meta.json"))
					total += n
					if !ok {
						inexact++
					}
				}
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() || measure == nil {
			return nil
		}
		if n, method, merr := measure(path); merr == nil && method == vmmanager.DiskMethodBtrfsExtents {
			total += n
		} else {
			inexact++
		}
		return nil
	})
	return total, inexact
}

// snapshotMetaDiskBytes is the on-disk bytes the snapshot writer recorded for
// a snapshot, and whether that was an exact reading (0, false when there is
// none or it was not the exact one).
func snapshotMetaDiskBytes(path string) (int64, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	var meta struct {
		DiskBytes       int64  `json:"disk_bytes"`
		DiskBytesMethod string `json:"disk_bytes_method"`
	}
	if json.Unmarshal(raw, &meta) != nil || meta.DiskBytesMethod != vmmanager.DiskMethodBtrfsExtents || meta.DiskBytes < 0 {
		return 0, false
	}
	return meta.DiskBytes, true
}
