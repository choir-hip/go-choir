package vmctl

import (
	"os"
	"path/filepath"
	"strings"
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
