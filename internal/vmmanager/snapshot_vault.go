package vmmanager

// Snapshot vault (SDC NET-06, review F12; docs/definitions/choir-launch-fp9-
// keys-recovery-2026-10-21.md).
//
// A snapshot's mem file is the guest's RAM. Without a vault it rests in
// plaintext on the main file system for up to the snapshot age limit. The
// vault is a volume whose key is read from /dev/urandom when the host boots
// and is never written down (dm-crypt plain mode has no header and no key
// slot), mounted at one path (nix/snapshot-vault.nix). With
// VM_SNAPSHOT_VAULT_DIR set, the snapshots of the selected computers are
// written under <vault>/<vm>/ and nowhere else:
//
//   - Before a capture, again before its commit, and before a resume, the
//     path is probed: it must be a mount point (not a subdirectory, not a
//     link) whose device is a device-mapper crypt target in plain mode, with
//     a root-only mount root. Anything else, including a host that cannot be
//     read or is not Linux, refuses with a named reason and the computer takes
//     the kill path (a cold wake, the crash rule). There is no fallback to the
//     state directory.
//   - A jailed VMM captures inside its jail on the main file system, so a
//     vault computer under the jailer is refused (confinement.go: the jail
//     base must share the state directory's mount to hard-link data.img).
//   - A snapshot is found where it exists (vault first), so turning the vault
//     on or off never strands one; a plaintext snapshot of a computer that is
//     now a vault computer is discarded, never loaded.
//   - A host reboot makes a new empty vault; every hibernated computer cold
//     boots, which loses no durable data (the data image persists).
//
// The probe reads /proc/self/mountinfo and /sys/dev/block/<maj:min>/dm/uuid.
// For a plain-mode mapping cryptsetup writes the uuid "CRYPT-PLAIN-<name>";
// LUKS writes "CRYPT-LUKS1-" or "CRYPT-LUKS2-". That string is from
// cryptsetup's source, not observed on Node B; scripts/snapshot-vault-
// verify.sh prints it on the host before the switch is turned on. If it were
// wrong the probe would refuse every capture: the failure is closed.

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

const (
	// snapshotVaultFloorBytes is the free space a vault must keep after a
	// snapshot. Only snapshots live there, so it is far below the state
	// directory's floor; the vault must still be sized for the budget plus it.
	snapshotVaultFloorBytes = int64(2) << 30

	vaultUUIDPlainPrefix = "CRYPT-PLAIN-"
	vaultUUIDCryptPrefix = "CRYPT-"
	vaultMountinfoPath   = "/proc/self/mountinfo"
	vaultSysRoot         = "/sys"
)

// vaultProbeEnv is where the probe reads the host from; tests point it at
// fixtures.
type vaultProbeEnv struct {
	// Mountinfo is the path of /proc/self/mountinfo.
	Mountinfo string
	// SysRoot is /sys.
	SysRoot string
	// OwnerOf reports a file's owning uid.
	OwnerOf func(os.FileInfo) (uint32, bool)
}

func statOwner(info os.FileInfo) (uint32, bool) {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, false
	}
	return st.Uid, true
}

// probeSnapshotVault is the production probe: the real procfs and sysfs.
func probeSnapshotVault(dir string, acceptAnyCrypt bool) error {
	return probeVaultAt(dir, acceptAnyCrypt, vaultProbeEnv{Mountinfo: vaultMountinfoPath, SysRoot: vaultSysRoot, OwnerOf: statOwner})
}

// probeVaultAt returns nil only when dir is an absolute, clean, link-free
// path to a root-only directory that is the root of a mount whose device is
// a dm-crypt target (plain mode unless acceptAnyCrypt). Every other outcome,
// including a host it cannot read, is an error.
func probeVaultAt(dir string, acceptAnyCrypt bool, env vaultProbeEnv) error {
	if strings.TrimSpace(dir) == "" {
		return fmt.Errorf("snapshot vault: no path configured")
	}
	if !filepath.IsAbs(dir) || filepath.Clean(dir) != dir {
		return fmt.Errorf("snapshot vault: %q is not an absolute, clean path", dir)
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return fmt.Errorf("snapshot vault: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("snapshot vault: %s is not a directory (links are refused)", dir)
	}
	if real, err := filepath.EvalSymlinks(dir); err != nil || real != dir {
		return fmt.Errorf("snapshot vault: %s is not its own real path (a component is a link)", dir)
	}
	if info.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("snapshot vault: %s is accessible to group or others (%o)", dir, info.Mode().Perm())
	}
	if env.OwnerOf == nil {
		return fmt.Errorf("snapshot vault: no way to read the owner of %s", dir)
	}
	if uid, ok := env.OwnerOf(info); !ok || uid != 0 {
		return fmt.Errorf("snapshot vault: %s is not owned by root", dir)
	}
	majmin, err := lastMountDevice(env.Mountinfo, dir)
	if err != nil {
		return err
	}
	uuidPath := filepath.Join(env.SysRoot, "dev", "block", majmin, "dm", "uuid")
	raw, err := os.ReadFile(uuidPath)
	if err != nil {
		return fmt.Errorf("snapshot vault: %s is mounted from device %s, which is not a device-mapper device: %w", dir, majmin, err)
	}
	uuid := strings.TrimSpace(string(raw))
	switch {
	case strings.HasPrefix(uuid, vaultUUIDPlainPrefix):
		return nil
	case strings.HasPrefix(uuid, vaultUUIDCryptPrefix) && acceptAnyCrypt:
		return nil
	case strings.HasPrefix(uuid, vaultUUIDCryptPrefix):
		return fmt.Errorf("snapshot vault: %s is a dm-crypt mapping that is not plain mode (a LUKS key slot is on disk); VM_SNAPSHOT_VAULT_ACCEPT_ANY_CRYPT=on accepts it", dir)
	default:
		return fmt.Errorf("snapshot vault: %s is mounted from a device-mapper target that is not dm-crypt", dir)
	}
}

// lastMountDevice returns the major:minor of the last mountinfo entry whose
// mount point is dir. The last entry is the one that serves files: a later
// mount stacked on the path hides the earlier one.
func lastMountDevice(mountinfo, dir string) (string, error) {
	file, err := os.Open(mountinfo)
	if err != nil {
		return "", fmt.Errorf("snapshot vault: cannot read the mount table: %w", err)
	}
	defer file.Close()
	majmin := ""
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64<<10), 1<<20)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 5 {
			continue
		}
		if unescapeMountinfo(fields[4]) == dir {
			majmin = fields[2]
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("snapshot vault: cannot read the mount table: %w", err)
	}
	if majmin == "" {
		return "", fmt.Errorf("snapshot vault: %s is not a mount point", dir)
	}
	return majmin, nil
}

// unescapeMountinfo decodes the octal escapes (\040 for a space) the kernel
// writes in mountinfo paths.
func unescapeMountinfo(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+4 <= len(s) {
			if v, ok := octal3(s[i+1 : i+4]); ok {
				b.WriteByte(v)
				i += 3
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func octal3(s string) (byte, bool) {
	if len(s) != 3 {
		return 0, false
	}
	v := 0
	for i := 0; i < 3; i++ {
		if s[i] < '0' || s[i] > '7' {
			return 0, false
		}
		v = v*8 + int(s[i]-'0')
	}
	if v > 255 {
		return 0, false
	}
	return byte(v), true
}

// --- manager side -----------------------------------------------------------

func (m *Manager) vaultConfigured() bool {
	return strings.TrimSpace(m.cfg.SnapshotVaultDir) != ""
}

// vaultSelected reports whether a computer's snapshots belong in the vault:
// the vault is configured and either no ONLY list is set or it names the VM
// or the computer.
func (m *Manager) vaultSelected(vmID, computerID string) bool {
	if !m.vaultConfigured() {
		return false
	}
	if len(m.cfg.SnapshotVaultOnly) == 0 {
		return true
	}
	for _, id := range m.cfg.SnapshotVaultOnly {
		if id == vmID || (computerID != "" && id == computerID) {
			return true
		}
	}
	return false
}

// probeVault runs the vault probe. A manager built without one still probes,
// with the production probe, which refuses on anything it cannot verify.
func (m *Manager) probeVault() error {
	probe := m.vaultProbe
	if probe == nil {
		probe = probeSnapshotVault
	}
	return probe(m.cfg.SnapshotVaultDir, m.cfg.SnapshotVaultAcceptAnyCrypt)
}

func snapshotDirIn(root, vmID string) string {
	return filepath.Join(root, vmID, snapshotDirName)
}

func snapshotPartialDirIn(root, vmID string) string {
	return filepath.Join(root, vmID, snapshotPartialDirName)
}

// safeSnapshotVMID guards the paths built under the vault from a VM id.
func safeSnapshotVMID(vmID string) bool {
	return vmID != "" && vmID != "." && vmID != ".." && !strings.ContainsRune(vmID, os.PathSeparator)
}

// snapshotRoots lists the roots that can hold snapshots: the vault when it is
// configured, then the state directory.
func (m *Manager) snapshotRoots() []string {
	roots := make([]string, 0, 2)
	if m.vaultConfigured() {
		roots = append(roots, m.cfg.SnapshotVaultDir)
	}
	if m.cfg.StateDir != "" {
		roots = append(roots, m.cfg.StateDir)
	}
	return roots
}

func (m *Manager) isVaultRoot(root string) bool {
	return m.vaultConfigured() && root == m.cfg.SnapshotVaultDir
}

// locateSnapshot returns where a VM's committed snapshot is: in the vault if
// one exists there, else the state directory's path (which may not exist).
func (m *Manager) locateSnapshot(vmID string) (dir string, inVault bool) {
	if m.vaultConfigured() && safeSnapshotVMID(vmID) {
		vaultDir := snapshotDirIn(m.cfg.SnapshotVaultDir, vmID)
		if _, err := os.Lstat(vaultDir); err == nil {
			return vaultDir, true
		}
	}
	return m.snapshotDir(vmID), false
}

// snapshotAnyDirs lists every committed and partial snapshot directory a VM
// could have, in every root.
func (m *Manager) snapshotAnyDirs(vmID string) []string {
	dirs := make([]string, 0, 4)
	if m.vaultConfigured() && safeSnapshotVMID(vmID) {
		dirs = append(dirs, snapshotDirIn(m.cfg.SnapshotVaultDir, vmID), snapshotPartialDirIn(m.cfg.SnapshotVaultDir, vmID))
	}
	return append(dirs, m.snapshotDir(vmID), m.snapshotPartialDir(vmID))
}

// removeVaultVMDir drops the VM's now-empty directory in the vault. It is a
// plain Remove: a directory that still holds anything stays.
func (m *Manager) removeVaultVMDir(vmID string) {
	if m.vaultConfigured() && safeSnapshotVMID(vmID) {
		_ = os.Remove(filepath.Join(m.cfg.SnapshotVaultDir, vmID))
	}
}
