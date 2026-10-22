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
//   - A capture opens the vault directory once and verifies that open
//     directory (repair round 1, finding B1): it is the probed directory and
//     it lives on the dm-crypt device the mount table names. Every capture
//     and commit path then resolves through it (/proc/self/fd/N for vmctl,
//     /proc/<vmctl pid>/fd/N for the VMM), so a vault unmounted or replaced
//     during the capture can never receive guest RAM on the main file
//     system: a plain umount fails with EBUSY while the directory is held,
//     and a lazy one leaves the held directory on the still-open encrypted
//     device. The commit verifies the handle again and that the captured
//     files are on the vault's device; a failed capture is removed through
//     the handle.
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
//     on or off never strands one (finding B2): a capture records the vault
//     root in <state>/snapshot-vault-roots, and discovery, listing, budget,
//     discard and the sweep use the configured root plus the recorded ones.
//     A recorded root that is no longer configured is never written to, its
//     snapshots load only while it still probes, and the sweep drops the
//     record once the root holds no snapshot; a plaintext snapshot of a computer that is
//     now a vault computer is discarded, never loaded.
//   - A host reboot makes a new empty vault; every hibernated computer cold
//     boots, which loses no durable data (the data image persists).
//
// The probe reads /proc/self/mountinfo and /sys/dev/block/<maj:min>/dm/uuid.
// For a plain-mode mapping cryptsetup writes the uuid "CRYPT-PLAIN-<name>";
// LUKS writes "CRYPT-LUKS1-" or "CRYPT-LUKS2-". That string is from
// cryptsetup's source, not observed on Node B; scripts/snapshot-vault.sh
// status prints it on the host before the switch is turned on. If it were
// wrong the probe would refuse every capture: the failure is closed.

import (
	"bufio"
	"fmt"
	"log"
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
	return checkCryptDevice(env.SysRoot, majmin, dir, acceptAnyCrypt)
}

// checkCryptDevice requires the block device majmin to be a dm-crypt target
// in plain mode (any dm-crypt mode with acceptAnyCrypt).
func checkCryptDevice(sysRoot, majmin, dir string, acceptAnyCrypt bool) error {
	uuidPath := filepath.Join(sysRoot, "dev", "block", majmin, "dm", "uuid")
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

// devMajmin formats a Linux dev_t as sysfs and mountinfo write it.
func devMajmin(dev uint64) string {
	major := ((dev >> 8) & 0x00000fff) | ((dev >> 32) & 0xfffff000)
	minor := (dev & 0x000000ff) | ((dev >> 12) & 0xffffff00)
	return fmt.Sprintf("%d:%d", major, minor)
}

func statOf(info os.FileInfo) (*syscall.Stat_t, bool) {
	st, ok := info.Sys().(*syscall.Stat_t)
	return st, ok
}

// sameOpenDirectory requires the open directory h to be the directory dir
// names now (same device and inode, dir not a link).
func sameOpenDirectory(h *os.File, dir string) error {
	held, err := h.Stat()
	if err != nil {
		return fmt.Errorf("snapshot vault: stat the held directory: %w", err)
	}
	now, err := os.Lstat(dir)
	if err != nil {
		return fmt.Errorf("snapshot vault: %w", err)
	}
	a, okA := statOf(held)
	b, okB := statOf(now)
	if !okA || !okB || !held.IsDir() || !now.IsDir() {
		return fmt.Errorf("snapshot vault: cannot compare the held directory with %s", dir)
	}
	if uint64(a.Dev) != uint64(b.Dev) || uint64(a.Ino) != uint64(b.Ino) {
		return fmt.Errorf("snapshot vault: %s no longer names the verified directory (unmounted or replaced)", dir)
	}
	return nil
}

// verifyVaultHandleAt requires the open directory h to be the directory at
// dir, on the device the mount table names for dir, and that device to be a
// dm-crypt plain target (any dm-crypt with acceptAnyCrypt).
func verifyVaultHandleAt(h *os.File, dir string, acceptAnyCrypt bool, env vaultProbeEnv) error {
	if err := sameOpenDirectory(h, dir); err != nil {
		return err
	}
	info, err := h.Stat()
	if err != nil {
		return fmt.Errorf("snapshot vault: stat the held directory: %w", err)
	}
	st, ok := statOf(info)
	if !ok {
		return fmt.Errorf("snapshot vault: no device number for the held directory")
	}
	held := devMajmin(uint64(st.Dev))
	mounted, err := lastMountDevice(env.Mountinfo, dir)
	if err != nil {
		return err
	}
	if held != mounted {
		return fmt.Errorf("snapshot vault: the held directory is on device %s, the mount at %s is %s", held, dir, mounted)
	}
	return checkCryptDevice(env.SysRoot, held, dir, acceptAnyCrypt)
}

// verifyVaultHandle is the production handle check: the real procfs and sysfs.
func verifyVaultHandle(h *os.File, dir string, acceptAnyCrypt bool) error {
	return verifyVaultHandleAt(h, dir, acceptAnyCrypt, vaultProbeEnv{Mountinfo: vaultMountinfoPath, SysRoot: vaultSysRoot, OwnerOf: statOwner})
}

// vaultHandle is the vault directory a capture verified and holds until its
// commit is synced. self and vmm are the paths that resolve through it for
// vmctl and for the (unjailed) VMM, which is vmctl's child.
type vaultHandle struct {
	dir  string
	f    *os.File
	dev  uint64
	self string
	vmm  string
}

// openVault opens dir, probes it, and verifies the open directory.
func (m *Manager) openVault(dir string) (*vaultHandle, error) {
	f, err := os.OpenFile(dir, os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return nil, fmt.Errorf("snapshot vault: open %s: %w", dir, err)
	}
	h := &vaultHandle{dir: dir, f: f}
	if err := h.verify(m); err != nil {
		h.close()
		return nil, err
	}
	info, err := f.Stat()
	if err != nil {
		h.close()
		return nil, fmt.Errorf("snapshot vault: stat the held directory: %w", err)
	}
	st, ok := statOf(info)
	if !ok {
		h.close()
		return nil, fmt.Errorf("snapshot vault: no device number for the held directory")
	}
	h.dev = uint64(st.Dev)
	fd := f.Fd()
	h.self = fmt.Sprintf("/proc/self/fd/%d", fd)
	h.vmm = fmt.Sprintf("/proc/%d/fd/%d", os.Getpid(), fd)
	return h, nil
}

// verify probes the path and checks the held directory against it.
func (h *vaultHandle) verify(m *Manager) error {
	if err := m.probeVaultDir(h.dir); err != nil {
		return err
	}
	check := m.vaultHandleCheck
	if check == nil {
		check = verifyVaultHandle
	}
	return check(h.f, h.dir, m.cfg.SnapshotVaultAcceptAnyCrypt)
}

// onDevice requires every path to be a regular file on the held vault's
// device.
func (h *vaultHandle) onDevice(paths ...string) error {
	for _, p := range paths {
		info, err := os.Lstat(p)
		if err != nil {
			return err
		}
		st, ok := statOf(info)
		if !ok || !info.Mode().IsRegular() || uint64(st.Dev) != h.dev {
			return fmt.Errorf("snapshot vault: %s is not a file on the vault device", filepath.Base(p))
		}
	}
	return nil
}

func (h *vaultHandle) close() {
	if h != nil && h.f != nil {
		_ = h.f.Close()
		h.f = nil
	}
}

// lastMountDevice returns the major:minor of the last mountinfo entry whose
// mount point is dir, and refuses unless that entry mounts the root of its
// file system. The last entry is the one that serves files: a later
// mount stacked on the path hides the earlier one.
func lastMountDevice(mountinfo, dir string) (string, error) {
	file, err := os.Open(mountinfo)
	if err != nil {
		return "", fmt.Errorf("snapshot vault: cannot read the mount table: %w", err)
	}
	defer file.Close()
	majmin, mountRoot := "", ""
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64<<10), 1<<20)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 5 {
			continue
		}
		if unescapeMountinfo(fields[4]) == dir {
			majmin, mountRoot = fields[2], unescapeMountinfo(fields[3])
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("snapshot vault: cannot read the mount table: %w", err)
	}
	if majmin == "" {
		return "", fmt.Errorf("snapshot vault: %s is not a mount point", dir)
	}
	// Field 4 is the root of the mount within its file system: the vault is
	// the whole file system, never a bind mount of a part of one.
	if mountRoot != "/" {
		return "", fmt.Errorf("snapshot vault: %s mounts %q of its file system, not the whole file system", dir, mountRoot)
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

// vaultPlacementUnknown reports a capture that cannot be placed: the vault
// list is set, the computer's id is empty and the list does not name its VM
// id, so whether the list means this computer cannot be told (repair round
// 2, L5). Such a capture is refused rather than written to the state
// directory.
func (m *Manager) vaultPlacementUnknown(vmID, computerID string) bool {
	if !m.vaultConfigured() || len(m.cfg.SnapshotVaultOnly) == 0 || computerID != "" {
		return false
	}
	for _, id := range m.cfg.SnapshotVaultOnly {
		if id == vmID {
			return false
		}
	}
	return true
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

// probeVaultDir runs the vault probe on a vault root. A manager built
// without one still probes, with the production probe, which refuses on
// anything it cannot verify.
func (m *Manager) probeVaultDir(dir string) error {
	probe := m.vaultProbe
	if probe == nil {
		probe = probeSnapshotVault
	}
	return probe(dir, m.cfg.SnapshotVaultAcceptAnyCrypt)
}

// snapshotVaultRootsFile records, in the state directory, every vault root a
// snapshot was written to (finding B2), one path per line.
const snapshotVaultRootsFile = "snapshot-vault-roots"

func (m *Manager) vaultRootsRecordPath() string {
	return filepath.Join(m.cfg.StateDir, snapshotVaultRootsFile)
}

// recordedVaultRoots reads the record; only absolute, clean paths count.
func (m *Manager) recordedVaultRoots() []string {
	if m.cfg.StateDir == "" {
		return nil
	}
	raw, err := os.ReadFile(m.vaultRootsRecordPath())
	if err != nil {
		return nil
	}
	var out []string
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && filepath.IsAbs(line) && filepath.Clean(line) == line {
			out = append(out, line)
		}
	}
	return out
}

// writeVaultRoots replaces the record atomically (0600).
func (m *Manager) writeVaultRoots(roots []string) error {
	path := m.vaultRootsRecordPath()
	if len(roots) == 0 {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	tmp := path + ".tmp"
	data := strings.Join(roots, "\n") + "\n"
	if err := os.WriteFile(tmp, []byte(data), 0o600); err != nil {
		return err
	}
	if err := os.Chmod(tmp, 0o600); err != nil {
		return err
	}
	if err := syncRegularFile(tmp); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	return syncDirectory(m.cfg.StateDir)
}

// recordVaultRoot adds root to the record before a capture writes into it,
// so a later rollback of the switch still finds the snapshot.
func (m *Manager) recordVaultRoot(root string) error {
	m.vaultRootsMu.Lock()
	defer m.vaultRootsMu.Unlock()
	roots := m.recordedVaultRoots()
	for _, r := range roots {
		if r == root {
			return nil
		}
	}
	return m.writeVaultRoots(append(roots, root))
}

// knownVaultRoots is the configured vault root, then every recorded one.
func (m *Manager) knownVaultRoots() []string {
	roots := make([]string, 0, 2)
	if m.vaultConfigured() {
		roots = append(roots, m.cfg.SnapshotVaultDir)
	}
	for _, r := range m.recordedVaultRoots() {
		if r != m.cfg.SnapshotVaultDir && r != m.cfg.StateDir {
			roots = append(roots, r)
		}
	}
	return roots
}

// dropEmptyVaultRoots removes from the record every root that is not
// configured and holds no committed or partial snapshot.
func (m *Manager) dropEmptyVaultRoots() {
	m.vaultRootsMu.Lock()
	defer m.vaultRootsMu.Unlock()
	roots := m.recordedVaultRoots()
	keep := make([]string, 0, len(roots))
	for _, r := range roots {
		if (m.vaultConfigured() && r == m.cfg.SnapshotVaultDir) || vaultRootHoldsSnapshots(r) {
			keep = append(keep, r)
		}
	}
	if len(keep) != len(roots) {
		if err := m.writeVaultRoots(keep); err != nil {
			log.Printf("vmmanager: snapshot vault record: %v", err)
		}
	}
}

// vaultRootHoldsSnapshots reports whether any <root>/<vm>/snapshot or
// snapshot.partial exists. An unreadable root counts as holding some.
func vaultRootHoldsSnapshots(root string) bool {
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return false
	}
	if err != nil {
		return true
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		for _, dir := range []string{snapshotDirIn(root, entry.Name()), snapshotPartialDirIn(root, entry.Name())} {
			if _, err := os.Lstat(dir); err == nil {
				return true
			}
		}
	}
	return false
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

// snapshotRoots lists the roots that can hold snapshots: the known vault
// roots (configured, then recorded), then the state directory.
func (m *Manager) snapshotRoots() []string {
	roots := m.knownVaultRoots()
	if m.cfg.StateDir != "" {
		roots = append(roots, m.cfg.StateDir)
	}
	return roots
}

func (m *Manager) isVaultRoot(root string) bool {
	for _, r := range m.knownVaultRoots() {
		if r == root {
			return true
		}
	}
	return false
}

// locateSnapshot returns where a VM's committed snapshot is: in a known vault
// root if one exists there (vaultRoot names it), else the state directory's
// path (which may not exist).
func (m *Manager) locateSnapshot(vmID string) (dir, vaultRoot string) {
	if safeSnapshotVMID(vmID) {
		for _, root := range m.knownVaultRoots() {
			vaultDir := snapshotDirIn(root, vmID)
			if _, err := os.Lstat(vaultDir); err == nil {
				return vaultDir, root
			}
		}
	}
	return m.snapshotDir(vmID), ""
}

// snapshotAnyDirs lists every committed and partial snapshot directory a VM
// could have, in every root.
func (m *Manager) snapshotAnyDirs(vmID string) []string {
	dirs := make([]string, 0, 4)
	if safeSnapshotVMID(vmID) {
		for _, root := range m.knownVaultRoots() {
			dirs = append(dirs, snapshotDirIn(root, vmID), snapshotPartialDirIn(root, vmID))
		}
	}
	return append(dirs, m.snapshotDir(vmID), m.snapshotPartialDir(vmID))
}

// removeVaultVMDir drops the VM's now-empty directory in the vault. It is a
// plain Remove: a directory that still holds anything stays.
func (m *Manager) removeVaultVMDir(vmID string) {
	if safeSnapshotVMID(vmID) {
		for _, root := range m.knownVaultRoots() {
			_ = os.Remove(filepath.Join(root, vmID))
		}
	}
}
