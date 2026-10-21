package vmmanager

import "os"

// PENDING (FP9 snapshot vault): inert stubs so the failing tests compile and
// fail on behavior. The implementation replaces this file.

const snapshotVaultFloorBytes = int64(2) << 30

// vaultProbeEnv is where the probe reads the host from (tests point it at
// fixtures).
type vaultProbeEnv struct {
	Mountinfo string
	SysRoot   string
	OwnerOf   func(os.FileInfo) (uint32, bool)
}

func probeVaultAt(dir string, acceptAnyCrypt bool, env vaultProbeEnv) error { return nil }

func probeSnapshotVault(dir string, acceptAnyCrypt bool) error { return nil }

func (m *Manager) ensureSnapshotRoomAt(root, vmID string, need int64) bool {
	return m.ensureSnapshotRoom(vmID, need)
}
