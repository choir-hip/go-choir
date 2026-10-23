package vmmanager

import (
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// VMM confinement, slice 1 (J1): each Firecracker runs under the Firecracker
// jailer as its own uid/gid, chrooted to a per-VM directory, in its own mount
// and PID namespaces and cgroup v2 leaf, with no IP connectivity. Design and
// failure modes: docs/problems/firecracker-runs-as-root-without-jailer-
// 2026-10-14.md ("Design: slice 1"). VMCTL_JAILER=on enables it (default
// off); VMCTL_JAILER_ONLY narrows it to named VMs or computers.

const (
	// ConfinementDirect is today's launch: Firecracker as a root child of
	// vmctl, its bounding set cut to CAP_NET_ADMIN.
	ConfinementDirect = "direct"
	// ConfinementJailer is the jailer launch.
	ConfinementJailer = "jailer"

	DefaultJailerUIDBase   = 900000
	DefaultJailerUIDCount  = 4096
	DefaultJailerPidsMax   = 128
	DefaultJailerCPUWeight = 100
	DefaultJailerNoFile    = 2048

	jailBaseDirName     = ".jail"
	vmmUIDFileName      = "vmm-uid"
	confinementFileName = "confinement.json"
	jailConfigName      = "fc-config.json"
	jailRootDirName     = "root"
	jailAPISocket       = "/run/fc.sock"
	jailPIDFileSuffix   = ".pid"
	vmmNoIPComment      = "go-choir-vmm-noip"
	defaultCgroupRoot   = "/sys/fs/cgroup"
	jailerExitTimeout   = 30 * time.Second
	jailedExecTimeout   = 5 * time.Second

	confinementReasonOff            = "jailer_off"
	confinementReasonNotSelected    = "not_selected"
	confinementReasonHostUnprepared = "jailer_host_unprepared"
	confinementReasonInvalidID      = "invalid_jailer_id"
	confinementReasonNextNotStatic  = "next_build_not_static"
)

// Linux capability numbers (include/uapi/linux/capability.h). Kept here so
// the policy compiles on every platform.
const (
	capChown       = 0
	capDACOverride = 1
	capFowner      = 3
	capKill        = 5
	capSetgid      = 6
	capSetuid      = 7
	capSetpcap     = 8
	capNetAdmin    = 12
	capSysPtrace   = 19
	capSysAdmin    = 21
	capMknod       = 27
)

var capNames = map[int]string{
	capChown: "chown", capDACOverride: "dac_override", capFowner: "fowner", capKill: "kill",
	capSetgid: "setgid", capSetuid: "setuid", capSetpcap: "setpcap", capNetAdmin: "net_admin",
	capSysPtrace: "sys_ptrace", capSysAdmin: "sys_admin", capMknod: "mknod",
}

func capName(c int) string {
	if name, ok := capNames[c]; ok {
		return name
	}
	return "cap_" + strconv.Itoa(c)
}

// directKeepCaps is a direct VMM's whole bounding set: it attaches to an
// ownerless tap. legacyCaps were vmctl's bounding set before slice 1; a
// failure to drop one of them does not fail a launch (an old unit).
var (
	directKeepCaps = []int{capNetAdmin}
	legacyCaps     = []int{capNetAdmin, capSysPtrace}
	// jailerKeepCaps is what the jailer needs before it drops to the VM's
	// uid: mount namespace and pivot_root, device nodes, chown of the jail,
	// setuid/setgid, and writing inside the jail root it has just chowned.
	jailerKeepCaps = []int{capSysAdmin, capMknod, capChown, capSetuid, capSetgid, capDACOverride}
)

// JailerConfig is the jailer flag and its parameters (config.go reads them
// from the environment).
type JailerConfig struct {
	// Enabled is VMCTL_JAILER=on.
	Enabled bool
	// Only, when non-empty, limits the jailer to these VM ids or computer
	// ids (VMCTL_JAILER_ONLY).
	Only []string
	// BinPath is the jailer binary (VM_JAILER_BIN; default: beside
	// Firecracker).
	BinPath string
	// FirecrackerBinPath is the jailer's --exec-file (VM_JAILER_FIRECRACKER_BIN;
	// default: the direct Firecracker). The jailer copies it into the chroot,
	// which holds no loader, so it must be a static build.
	FirecrackerBinPath string
	// ChrootBase is the jailer's --chroot-base-dir. It must be on the state
	// dir's mount so data.img can be hard-linked (default <state>/.jail).
	ChrootBase string
	// UIDBase and UIDCount bound the per-VM uids (gid = uid).
	UIDBase  int
	UIDCount int
	// CgroupParent is the VMs' parent cgroup relative to the cgroup2 root;
	// empty means derive it from vmctl's own cgroup at Start.
	CgroupParent string
	// PidsMax and CPUWeight are written into each VM's leaf; 0 omits them.
	PidsMax   int
	CPUWeight int
	// NoFile is the VMM's RLIMIT_NOFILE; 0 leaves the jailer default.
	NoFile int
	// SpeculationDirect applies the speculation barriers to direct launches
	// too (VMCTL_VMM_SPECULATION_BARRIERS=on). Jailed launches always get them.
	SpeculationDirect bool
}

// ConfinementReceipt records how one launch confined its VMM. It is part of
// the boot timeline and persisted as <state>/<vm>/confinement.json.
type ConfinementReceipt struct {
	Mode         string              `json:"mode"`
	Reason       string              `json:"reason,omitempty"`
	UID          int                 `json:"uid,omitempty"`
	GID          int                 `json:"gid,omitempty"`
	JailRoot     string              `json:"jail_root,omitempty"`
	Cgroup       string              `json:"cgroup,omitempty"`
	PID          int                 `json:"pid,omitempty"`
	JailFiles    []string            `json:"jail_files,omitempty"`
	LaunchThread *LaunchThreadReport `json:"launch_thread,omitempty"`
	At           string              `json:"at,omitempty"`
}

func (r *ConfinementReceipt) jailed() bool {
	return r != nil && r.Mode == ConfinementJailer
}

func (r *ConfinementReceipt) clone() *ConfinementReceipt {
	if r == nil {
		return nil
	}
	c := *r
	c.JailFiles = append([]string(nil), r.JailFiles...)
	if r.LaunchThread != nil {
		lt := *r.LaunchThread
		lt.Speculation = append([]string(nil), r.LaunchThread.Speculation...)
		lt.BoundingKept = append([]string(nil), r.LaunchThread.BoundingKept...)
		lt.Errors = append([]string(nil), r.LaunchThread.Errors...)
		c.LaunchThread = &lt
	}
	return &c
}

// LaunchThreadReport is what the launch thread did before the fork.
type LaunchThreadReport struct {
	// Supported is false where the thread policy cannot run (not Linux).
	Supported bool `json:"supported"`
	// NoNewPrivs records PR_SET_NO_NEW_PRIVS on the launch thread.
	NoNewPrivs bool `json:"no_new_privs,omitempty"`
	// Speculation lists "indirect_branch=force_disabled" style results.
	Speculation []string `json:"speculation,omitempty"`
	// BoundingKept names the capabilities left in the bounding set; empty
	// with BoundingSkipped set when vmctl is not root.
	BoundingKept    []string `json:"bounding_kept,omitempty"`
	BoundingDropped int      `json:"bounding_dropped,omitempty"`
	BoundingSkipped string   `json:"bounding_skipped,omitempty"`
	// Errors are non-fatal failures (a speculation control the CPU does not
	// offer, a legacy capability an old unit cannot drop).
	Errors []string `json:"errors,omitempty"`
}

// threadPolicy is applied on the launch thread before the fork.
type threadPolicy struct {
	// keepCaps, when non-nil, is the bounding set after the drop.
	keepCaps []int
	// tolerateCaps may fail to drop without failing the launch.
	tolerateCaps []int
	speculation  bool
}

var errJailerHostNotPrepared = errors.New("jailer host not prepared (manager not started)")

// --- configuration and decision -------------------------------------------

func defaultJailerConfig() JailerConfig {
	return JailerConfig{
		UIDBase:   DefaultJailerUIDBase,
		UIDCount:  DefaultJailerUIDCount,
		PidsMax:   DefaultJailerPidsMax,
		CPUWeight: DefaultJailerCPUWeight,
		NoFile:    DefaultJailerNoFile,
	}
}

var jailerIDPattern = regexp.MustCompile(`^[A-Za-z0-9-]{1,64}$`)

// validJailerID mirrors the jailer's validate_instance_id.
func validJailerID(id string) bool {
	return jailerIDPattern.MatchString(id)
}

func (m *Manager) jailerHostError() error {
	m.jailerMu.RLock()
	defer m.jailerMu.RUnlock()
	return m.jailerHostErr
}

// decideConfinement names the confinement the next launch of cfg uses and,
// for a direct launch, why. Validation of a snapshot and its restore use the
// same decision as a launch, so they cannot disagree.
func (m *Manager) decideConfinement(cfg VMConfig) (mode, reason string) {
	j := m.cfg.Jailer
	if !j.Enabled {
		return ConfinementDirect, confinementReasonOff
	}
	if len(j.Only) > 0 && !jailerSelects(j.Only, cfg) {
		return ConfinementDirect, confinementReasonNotSelected
	}
	if err := m.jailerHostError(); err != nil {
		return ConfinementDirect, confinementReasonHostUnprepared + ": " + err.Error()
	}
	if !validJailerID(cfg.VMID) {
		return ConfinementDirect, confinementReasonInvalidID
	}
	return ConfinementJailer, ""
}

// jailerSelects is VMCTL_JAILER_ONLY: VM ids, computer ids, class:<class>
// or * (computer_select.go).
func jailerSelects(only []string, cfg VMConfig) bool {
	return selectsComputer(only, cfg)
}

// --- paths -----------------------------------------------------------------

func (m *Manager) jailChrootBase() string {
	if base := strings.TrimSpace(m.cfg.Jailer.ChrootBase); base != "" {
		return filepath.Clean(base)
	}
	return filepath.Join(m.cfg.StateDir, jailBaseDirName)
}

// jailExecName is the directory the jailer names after the exec file: the
// basename of its canonical path.
func (m *Manager) jailExecName() string {
	bin := m.cfg.FirecrackerBinPath
	if bin == "" {
		bin = "firecracker"
	}
	if real, err := filepath.EvalSymlinks(bin); err == nil {
		bin = real
	}
	return filepath.Base(bin)
}

func (m *Manager) jailDir(vmID string) string {
	return filepath.Join(m.jailChrootBase(), m.jailExecName(), vmID)
}

func (m *Manager) jailRoot(vmID string) string {
	return filepath.Join(m.jailDir(vmID), jailRootDirName)
}

// jailAPISocketHostPath is the jailed VMM's API socket as vmctl reaches it.
func (m *Manager) jailAPISocketHostPath(vmID string) string {
	return filepath.Join(m.jailRoot(vmID), strings.TrimPrefix(jailAPISocket, "/"))
}

func (m *Manager) cgroupLeafRel(vmID string) string {
	parent := strings.Trim(m.cfg.Jailer.CgroupParent, "/")
	if parent == "" {
		return ""
	}
	return parent + "/" + vmID
}

// --- arguments -------------------------------------------------------------

// jailFirecrackerLaunchArgs are Firecracker's arguments after "--" for a
// jailed cold boot. The jailer prepends --id itself.
func jailFirecrackerLaunchArgs(apiSocket bool) []string {
	if apiSocket {
		return []string{"--api-sock", jailAPISocket, "--enable-pci", "--config-file", "/" + jailConfigName}
	}
	return []string{"--no-api", "--enable-pci", "--config-file", "/" + jailConfigName}
}

// jailFirecrackerRestoreArgs are Firecracker's arguments for a jailed
// snapshot restore: the API socket only.
func jailFirecrackerRestoreArgs() []string {
	return []string{"--api-sock", jailAPISocket, "--enable-pci"}
}

// jailerArgs is the jailer's command line (without argv[0]).
func (m *Manager) jailerArgs(vmID string, uid int, firecrackerArgs []string) []string {
	j := m.cfg.Jailer
	bin := m.launchedVMM(vmID).Firecracker
	if bin == "" {
		bin = "firecracker"
	}
	args := []string{
		"--id", vmID,
		"--exec-file", bin,
		"--uid", strconv.Itoa(uid),
		"--gid", strconv.Itoa(uid),
		"--chroot-base-dir", m.jailChrootBase(),
		"--new-pid-ns",
		"--cgroup-version", "2",
	}
	if parent := strings.Trim(j.CgroupParent, "/"); parent != "" {
		args = append(args, "--parent-cgroup", parent)
	}
	if j.PidsMax > 0 {
		args = append(args, "--cgroup", "pids.max="+strconv.Itoa(j.PidsMax))
	}
	if j.CPUWeight > 0 {
		args = append(args, "--cgroup", "cpu.weight="+strconv.Itoa(j.CPUWeight))
	}
	if j.NoFile > 0 {
		args = append(args, "--resource-limit", "no-file="+strconv.Itoa(j.NoFile))
	}
	args = append(args, "--")
	return append(args, firecrackerArgs...)
}

// --- launch config rewriting -------------------------------------------------

// jailFile is one host file the jailed VMM opens, and its name in the jail.
type jailFile struct {
	HostPath string
	Name     string
	// Writable files are hard links of the host file owned by the VMM uid;
	// the rest are copies (reflinks where the filesystem allows) vmctl owns.
	Writable bool
}

var driveIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,32}$`)

// rewriteFirecrackerConfigForJail returns a copy of a launch config with every
// host path replaced by its jail path, and the files to place in the jail.
// The input is not changed.
func rewriteFirecrackerConfigForJail(host map[string]interface{}) (map[string]interface{}, []jailFile, error) {
	raw, err := json.Marshal(host)
	if err != nil {
		return nil, nil, err
	}
	var cfg map[string]interface{}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, nil, err
	}
	files := make([]jailFile, 0, 5)
	boot, _ := cfg["boot-source"].(map[string]interface{})
	if boot == nil {
		return nil, nil, fmt.Errorf("launch config has no boot-source")
	}
	kernel, _ := boot["kernel_image_path"].(string)
	if kernel == "" {
		return nil, nil, fmt.Errorf("launch config has no kernel image")
	}
	files = append(files, jailFile{HostPath: kernel, Name: "vmlinux"})
	boot["kernel_image_path"] = "/vmlinux"
	if initrd, _ := boot["initrd_path"].(string); initrd != "" {
		files = append(files, jailFile{HostPath: initrd, Name: "initrd"})
		boot["initrd_path"] = "/initrd"
	}
	drives, _ := cfg["drives"].([]interface{})
	for _, rawDrive := range drives {
		drive, _ := rawDrive.(map[string]interface{})
		if drive == nil {
			return nil, nil, fmt.Errorf("launch config drive is not an object")
		}
		id, _ := drive["drive_id"].(string)
		if !driveIDPattern.MatchString(id) {
			return nil, nil, fmt.Errorf("drive id %q cannot name a jail file", id)
		}
		path, _ := drive["path_on_host"].(string)
		if path == "" {
			return nil, nil, fmt.Errorf("drive %s has no path", id)
		}
		readOnly, _ := drive["is_read_only"].(bool)
		name := id + ".img"
		files = append(files, jailFile{HostPath: path, Name: name, Writable: !readOnly})
		drive["path_on_host"] = "/" + name
	}
	return cfg, files, nil
}

// --- per-VM uid --------------------------------------------------------------

func fnv32(s string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(s))
	return h.Sum32()
}

func (m *Manager) inVMMUIDRange(uid int) bool {
	j := m.cfg.Jailer
	return j.UIDCount > 0 && uid >= j.UIDBase && uid < j.UIDBase+j.UIDCount
}

func (m *Manager) parseVMMUID(data []byte) (int, bool) {
	uid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || !m.inVMMUIDRange(uid) {
		return 0, false
	}
	return uid, true
}

// readVMMUID returns the VM's recorded uid, if it has a valid one.
func (m *Manager) readVMMUID(vmID string) (int, bool) {
	data, err := os.ReadFile(filepath.Join(m.vmStateDir(vmID), vmmUIDFileName))
	if err != nil {
		return 0, false
	}
	return m.parseVMMUID(data)
}

// allocateVMMUID returns the VM's uid, allocating it on first use. The record
// lives in the VM's state dir, so the uid is held exactly as long as the VM's
// files exist. A record that cannot be parsed is never overwritten: files may
// be owned by whatever uid it once named.
func (m *Manager) allocateVMMUID(vmID string) (int, error) {
	j := m.cfg.Jailer
	if j.UIDCount <= 0 || j.UIDBase <= 0 {
		return 0, fmt.Errorf("VMM uid range is not configured")
	}
	m.uidMu.Lock()
	defer m.uidMu.Unlock()
	vmDir := m.vmStateDir(vmID)
	path := filepath.Join(vmDir, vmmUIDFileName)
	data, err := os.ReadFile(path)
	switch {
	case err == nil:
		uid, ok := m.parseVMMUID(data)
		if !ok {
			return 0, fmt.Errorf("VMM uid record %s is invalid (%q); refusing to reallocate", path, strings.TrimSpace(string(data)))
		}
		return uid, nil
	case !os.IsNotExist(err):
		return 0, fmt.Errorf("read VMM uid record %s: %w", path, err)
	}
	used := m.usedVMMUIDs(vmID)
	start := int(fnv32(vmID) % uint32(j.UIDCount))
	for i := 0; i < j.UIDCount; i++ {
		uid := j.UIDBase + (start+i)%j.UIDCount
		if used[uid] {
			continue
		}
		if err := writeFileDurably(vmDir, vmmUIDFileName, []byte(strconv.Itoa(uid)+"\n"), 0o644); err != nil {
			return 0, fmt.Errorf("record VMM uid for %s: %w", vmID, err)
		}
		log.Printf("vmmanager: allocated VMM uid %d to VM %s", uid, vmID)
		return uid, nil
	}
	return 0, fmt.Errorf("no free VMM uid in [%d,%d)", j.UIDBase, j.UIDBase+j.UIDCount)
}

func (m *Manager) usedVMMUIDs(except string) map[int]bool {
	used := map[int]bool{}
	entries, err := os.ReadDir(m.cfg.StateDir)
	if err != nil {
		return used
	}
	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == except {
			continue
		}
		if uid, ok := m.readVMMUID(entry.Name()); ok {
			used[uid] = true
		}
	}
	return used
}

// writeFileDurably writes dir/name by temp file, fsync, rename, dir fsync.
func writeFileDurably(dir, name string, data []byte, mode os.FileMode) error {
	tmp, err := os.CreateTemp(dir, "."+name+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, filepath.Join(dir, name)); err != nil {
		return err
	}
	return syncDirectory(dir)
}

// --- jail layout -------------------------------------------------------------

// ensurePrivateDir makes dir a 0700 directory owned by this process, refusing
// a symlink or another owner at that path (CVE-2026-1386: the jailer once
// followed a symlink planted in a writable jail path).
func ensurePrivateDir(dir string) error {
	info, err := os.Lstat(dir)
	if os.IsNotExist(err) {
		if err := os.Mkdir(dir, 0o700); err != nil && !os.IsExist(err) {
			return fmt.Errorf("create %s: %w", dir, err)
		}
		info, err = os.Lstat(dir)
	}
	if err != nil {
		return fmt.Errorf("stat %s: %w", dir, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("%s must be a real directory, not %s", dir, info.Mode().Type())
	}
	if st, ok := info.Sys().(*syscall.Stat_t); ok && int(st.Uid) != os.Geteuid() {
		return fmt.Errorf("%s is owned by uid %d, not vmctl", dir, st.Uid)
	}
	if info.Mode().Perm() != 0o700 {
		if err := os.Chmod(dir, 0o700); err != nil {
			return fmt.Errorf("chmod %s: %w", dir, err)
		}
	}
	return nil
}

// prepareJail rebuilds the VM's jail from nothing: the jailer refuses its own
// leftovers (pid file, device nodes) and copies its binary without
// truncating. Writable drives are hard links owned by uid (the host file is
// the same inode); everything else is a hard link of vmctl's one shared,
// read-only copy of the artifact (jail_shared.go), so every jailed VM reads
// the store image through one inode and one host page-cache copy.
// config, when non-nil, becomes the jail's fc-config.json. It returns how
// each file was placed, for the receipt.
func (m *Manager) prepareJail(vmID string, uid int, files []jailFile, config []byte) ([]string, error) {
	if !validJailerID(vmID) {
		return nil, fmt.Errorf("VM id %q is not a valid jailer id", vmID)
	}
	base := m.jailChrootBase()
	execDir := filepath.Join(base, m.jailExecName())
	for _, dir := range []string{base, execDir} {
		if err := ensurePrivateDir(dir); err != nil {
			return nil, fmt.Errorf("jail base: %w", err)
		}
	}
	dir := filepath.Join(execDir, vmID)
	if err := os.RemoveAll(dir); err != nil {
		return nil, fmt.Errorf("remove previous jail %s: %w", dir, err)
	}
	if err := ensurePrivateDir(dir); err != nil {
		return nil, err
	}
	root := filepath.Join(dir, jailRootDirName)
	if err := ensurePrivateDir(root); err != nil {
		return nil, err
	}
	notes := make([]string, 0, len(files))
	for _, f := range files {
		if f.Name == "" || strings.ContainsRune(f.Name, os.PathSeparator) || f.Name == "." || f.Name == ".." {
			return nil, fmt.Errorf("invalid jail file name %q", f.Name)
		}
		dst := filepath.Join(root, f.Name)
		if f.Writable {
			if err := os.Link(f.HostPath, dst); err != nil {
				return nil, fmt.Errorf("hard-link writable drive %s into the jail (a writable disk is never copied; the jail must share its mount): %w", f.HostPath, err)
			}
			if err := m.chownFn(dst, uid, uid); err != nil {
				return nil, fmt.Errorf("chown %s to VMM uid %d: %w", dst, uid, err)
			}
			if err := os.Chmod(dst, 0o600); err != nil {
				return nil, fmt.Errorf("chmod %s: %w", dst, err)
			}
			notes = append(notes, f.Name+"=hardlink")
			continue
		}
		method, err := m.linkSharedReadOnly(f.HostPath, dst)
		if err != nil {
			return nil, fmt.Errorf("place %s in the jail: %w", f.HostPath, err)
		}
		notes = append(notes, f.Name+"="+method)
	}
	if config != nil {
		if err := writeNewFile(filepath.Join(root, jailConfigName), config, 0o444); err != nil {
			return nil, fmt.Errorf("write jail launch config: %w", err)
		}
	}
	return notes, nil
}

func writeNewFile(path string, data []byte, mode os.FileMode) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, mode)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Chmod(mode); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

// placeReadOnlyCopy puts a private copy of src at dst: a reflink where the
// filesystem shares extents (btrfs: no data copied), else a byte copy.
func placeReadOnlyCopy(src, dst string) (string, error) {
	in, err := os.Open(src)
	if err != nil {
		return "", err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("%s is not a regular file", src)
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, 0o600)
	if err != nil {
		return "", err
	}
	method := "reflink"
	if err := cloneFile(out, in); err != nil {
		method = "copy"
		if _, err := io.Copy(out, in); err != nil {
			_ = out.Close()
			return "", err
		}
		if info.Size() > 64<<20 {
			log.Printf("vmmanager: jail: reflink of %s unavailable; made a full %d-byte copy", src, info.Size())
		}
	}
	if err := out.Chmod(0o444); err != nil {
		_ = out.Close()
		return "", err
	}
	return method, out.Close()
}

// linkSnapshotIntoJail hard-links a committed snapshot's files into the
// jail's snapshot/ directory, owned by the VMM uid, for a jailed restore.
func (m *Manager) linkSnapshotIntoJail(vmID, snapshotDir string, uid int) error {
	dir := filepath.Join(m.jailRoot(vmID), snapshotDirName)
	if err := os.Mkdir(dir, 0o700); err != nil {
		return err
	}
	if err := m.chownFn(dir, uid, uid); err != nil {
		return err
	}
	for _, name := range []string{snapshotStateName, snapshotMemName} {
		if err := os.Link(filepath.Join(snapshotDir, name), filepath.Join(dir, name)); err != nil {
			return fmt.Errorf("hard-link snapshot %s into the jail: %w", name, err)
		}
	}
	return nil
}

// removeJailSnapshotLinks drops snapshot names left in the VM's jail. The
// names pin the memory file's blocks after the host copy is discarded.
func (m *Manager) removeJailSnapshotLinks(vmID string) {
	if !validJailerID(vmID) {
		return
	}
	for _, name := range []string{snapshotDirName, snapshotPartialDirName} {
		dir := filepath.Join(m.jailRoot(vmID), name)
		if _, err := os.Lstat(dir); err == nil {
			if err := os.RemoveAll(dir); err != nil {
				log.Printf("vmmanager: remove jail snapshot dir %s: %v", dir, err)
			}
		}
	}
}

// reclaimDirectOwnership prepares a direct launch of a VM that last ran in a
// jail: its disks go back to root (a root VMM without CAP_DAC_OVERRIDE cannot
// open a file another uid owns 0600) and the stale jail is removed.
func (m *Manager) reclaimDirectOwnership(vmID string) {
	for _, name := range []string{"data.img", "credential.img", "rootfs.ext4"} {
		path := filepath.Join(m.vmStateDir(vmID), name)
		uid, ok := m.fileOwnerFn(path)
		if !ok || !m.inVMMUIDRange(uid) {
			continue
		}
		if err := m.chownFn(path, 0, 0); err != nil {
			log.Printf("vmmanager: return %s of VM %s to root: %v", name, vmID, err)
			continue
		}
		log.Printf("vmmanager: returned %s of VM %s from VMM uid %d to root for a direct launch", name, vmID, uid)
	}
	if !validJailerID(vmID) {
		return
	}
	if _, err := os.Lstat(m.jailDir(vmID)); err == nil {
		if err := os.RemoveAll(m.jailDir(vmID)); err != nil {
			log.Printf("vmmanager: remove stale jail of VM %s: %v", vmID, err)
		}
	}
}

func defaultFileOwner(path string) (int, bool) {
	info, err := os.Lstat(path)
	if err != nil {
		return 0, false
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, false
	}
	return int(st.Uid), true
}

// --- receipts --------------------------------------------------------------

// recordConfinementLocked attaches a launch's confinement receipt to the
// tracked instance and its boot timeline and persists it. Callers hold m.mu.
func (m *Manager) recordConfinementLocked(vmID string, receipt *ConfinementReceipt) {
	if receipt == nil {
		return
	}
	if receipt.At == "" {
		receipt.At = time.Now().UTC().Format(time.RFC3339Nano)
	}
	if inst, ok := m.vms[vmID]; ok {
		inst.confinement = receipt
		if inst.BootTimeline != nil {
			inst.BootTimeline.setConfinement(receipt.clone())
		}
	}
	m.persistConfinement(vmID, receipt)
}

func (m *Manager) persistConfinement(vmID string, receipt *ConfinementReceipt) {
	data, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return
	}
	dir := m.vmStateDir(vmID)
	if _, err := os.Stat(dir); err != nil {
		return
	}
	if err := writeFileDurably(dir, confinementFileName, data, 0o644); err != nil {
		log.Printf("vmmanager: persist confinement receipt for VM %s: %v", vmID, err)
	}
}

func (m *Manager) loadConfinement(vmID string) *ConfinementReceipt {
	data, err := os.ReadFile(filepath.Join(m.vmStateDir(vmID), confinementFileName))
	if err != nil {
		return nil
	}
	var receipt ConfinementReceipt
	if json.Unmarshal(data, &receipt) != nil {
		return nil
	}
	return &receipt
}

// --- host preparation --------------------------------------------------------

// parseCgroupV2Path returns the unified-hierarchy path in /proc/<pid>/cgroup
// content, relative to the cgroup2 root ("" for the root or none).
func parseCgroupV2Path(content string) string {
	for _, line := range strings.Split(content, "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "0::"); ok {
			return strings.Trim(rest, "/")
		}
	}
	return ""
}

// deriveJailerCgroupParent places VM leaves beside vmctl's own subgroup: the
// unit runs with Delegate=yes and DelegateSubgroup=supervisor, so vmctl is in
// <unit>/supervisor and VMs go to <unit>/vms/<vm>, under the unit's memory
// protection.
func deriveJailerCgroupParent(own string) (string, error) {
	own = strings.Trim(own, "/")
	if own == "" || filepath.Base(own) != "supervisor" {
		return "", fmt.Errorf("vmctl runs in cgroup %q, not a delegated 'supervisor' subgroup (DelegateSubgroup=supervisor); set VMCTL_JAILER_CGROUP_PARENT", own)
	}
	return filepath.Dir(own) + "/vms", nil
}

// prepareJailerHost readies the host for jailed launches once, at Start. Any
// failure leaves the host unprepared: every launch is direct and its receipt
// says why (failure item 25).
func (m *Manager) prepareJailerHost() error {
	j := &m.cfg.Jailer
	info, err := os.Stat(j.BinPath)
	if err != nil {
		return fmt.Errorf("jailer binary: %w", err)
	}
	if info.IsDir() || info.Mode()&0o111 == 0 {
		return fmt.Errorf("jailer binary %s is not executable", j.BinPath)
	}
	if j.UIDBase <= 0 || j.UIDCount <= 0 {
		return fmt.Errorf("VMM uid range [%d,+%d) is invalid", j.UIDBase, j.UIDCount)
	}
	if user := hostUserInRange("/etc/passwd", j.UIDBase, j.UIDCount); user != "" {
		return fmt.Errorf("host user %s holds a uid in the VMM range [%d,%d)", user, j.UIDBase, j.UIDBase+j.UIDCount)
	}
	for _, dir := range []string{m.jailChrootBase(), filepath.Join(m.jailChrootBase(), m.jailExecName())} {
		if err := ensurePrivateDir(dir); err != nil {
			return err
		}
	}
	ownData, err := os.ReadFile("/proc/self/cgroup")
	if err != nil {
		return fmt.Errorf("read own cgroup: %w", err)
	}
	own := parseCgroupV2Path(string(ownData))
	if strings.TrimSpace(j.CgroupParent) == "" {
		parent, err := deriveJailerCgroupParent(own)
		if err != nil {
			return err
		}
		j.CgroupParent = parent
	}
	if err := m.prepareJailerCgroups(own); err != nil {
		return err
	}
	return m.ensureVMMNoIPRules()
}

// prepareJailerCgroups empties the VMs' common ancestor of processes (cgroup
// v2 refuses to enable controllers in a cgroup that holds any) by moving them
// into vmctl's own subgroup, then creates the VMs' parent.
func (m *Manager) prepareJailerCgroups(own string) error {
	root := m.cgroupRoot
	parent := filepath.Join(root, strings.Trim(m.cfg.Jailer.CgroupParent, "/"))
	common := filepath.Dir(parent)
	ownDir := filepath.Join(root, own)
	if own != "" && filepath.Dir(ownDir) == common {
		data, err := os.ReadFile(filepath.Join(common, "cgroup.procs"))
		if err != nil {
			return fmt.Errorf("read %s/cgroup.procs: %w", common, err)
		}
		for _, field := range strings.Fields(string(data)) {
			if _, err := strconv.Atoi(field); err != nil {
				continue
			}
			err := os.WriteFile(filepath.Join(ownDir, "cgroup.procs"), []byte(field), 0o644)
			if err != nil && !errors.Is(err, syscall.ESRCH) {
				return fmt.Errorf("move pid %s from %s into %s: %w", field, common, ownDir, err)
			}
			if err == nil {
				log.Printf("vmmanager: moved pid %s from %s into %s so VM cgroups can enable controllers", field, common, ownDir)
			}
		}
	}
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("create VM cgroup parent %s: %w", parent, err)
	}
	return nil
}

func (m *Manager) removeCgroupLeaf(vmID string) {
	rel := m.cgroupLeafRel(vmID)
	if rel == "" || !validJailerID(vmID) {
		return
	}
	if err := os.Remove(filepath.Join(m.cgroupRoot, rel)); err != nil && !os.IsNotExist(err) {
		log.Printf("vmmanager: remove cgroup leaf %s: %v", rel, err)
	}
}

// hostUserInRange returns a passwd entry whose uid falls in [base, base+count).
func hostUserInRange(passwd string, base, count int) string {
	data, err := os.ReadFile(passwd)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Split(line, ":")
		if len(fields) < 3 {
			continue
		}
		uid, err := strconv.Atoi(fields[2])
		if err == nil && uid >= base && uid < base+count {
			return fields[0] + ":" + fields[2]
		}
	}
	return ""
}

// vmmNoIPRuleSpec drops every IP packet a VMM uid originates: tap I/O is a
// file descriptor and the API is a unix socket, so a VMM needs no IP.
func (m *Manager) vmmNoIPRuleSpec() []string {
	j := m.cfg.Jailer
	return []string{
		"-m", "owner", "--uid-owner", fmt.Sprintf("%d-%d", j.UIDBase, j.UIDBase+j.UIDCount-1),
		"-m", "comment", "--comment", vmmNoIPComment,
		"-j", "DROP",
	}
}

func (m *Manager) ensureVMMNoIPRules() error {
	spec := m.vmmNoIPRuleSpec()
	h := m.net()
	if _, err := h.ipt("filter", append([]string{"-C", "OUTPUT"}, spec...)...); err != nil {
		if _, err := h.ipt("filter", append([]string{"-I", "OUTPUT", "1"}, spec...)...); err != nil {
			return fmt.Errorf("iptables OUTPUT no-IP rule for VMM uids: %w", err)
		}
		log.Printf("vmmanager: installed iptables OUTPUT DROP for VMM uids %d-%d", m.cfg.Jailer.UIDBase, m.cfg.Jailer.UIDBase+m.cfg.Jailer.UIDCount-1)
	}
	// Guests have no IPv6 today; the v6 rule is belt and braces.
	ip6 := findBinary("ip6tables", "/run/current-system/sw/bin/ip6tables")
	if _, err := runHostCommand(ip6, append([]string{"-C", "OUTPUT"}, spec...)...); err != nil {
		if _, err := runHostCommand(ip6, append([]string{"-I", "OUTPUT", "1"}, spec...)...); err != nil {
			log.Printf("vmmanager: ip6tables OUTPUT no-IP rule for VMM uids not installed: %v", err)
		}
	}
	return nil
}

// staticELFError is nil when path is an ELF with no program interpreter.
func staticELFError(path string) error { return nil }

// jailedExecError checks the jailed exec file against the direct binary.
func jailedExecError(direct, jailed string, version func(string) string) error { return nil }

// --- jailed process start ----------------------------------------------------

// startJailedFirecracker runs the jailer for vmID and tracks the Firecracker
// it leaves behind in its own PID namespace. Callers hold m.mu.
func (m *Manager) startJailedFirecracker(vmID string, receipt *ConfinementReceipt, firecrackerArgs []string) error {
	args := m.jailerArgs(vmID, receipt.UID, firecrackerArgs)
	if m.firecrackerStarter != nil {
		return m.firecrackerStarter(vmID, args)
	}
	consolePath := consoleLogPath(m.cfg.StateDir, vmID)
	console, err := openConsoleSink(consolePath, consoleLogMaxBytes, consoleLogGenerations)
	if err != nil {
		return fmt.Errorf("open console sink for VM %s: %w", vmID, err)
	}
	cmd := exec.Command(m.launchedVMM(vmID).Jailer, args...)
	cmd.Stdout = console
	cmd.Stderr = os.Stderr
	cmd.Env = []string{}
	report, err := startWithThreadPolicy(cmd, threadPolicy{keepCaps: jailerKeepCaps, speculation: true})
	receipt.LaunchThread = &report
	_ = console.Close()
	if err != nil {
		return fmt.Errorf("start jailer for VM %s: %w", vmID, err)
	}
	// With --new-pid-ns the jailer parent exits once Firecracker is forked
	// into its namespace; its exit status is the jail setup's.
	if err := m.waitJailer(vmID, cmd, jailerExitTimeout); err != nil {
		return fmt.Errorf("jailer for VM %s: %w", vmID, err)
	}
	pidPath := filepath.Join(m.jailRoot(vmID), m.jailExecName()+jailPIDFileSuffix)
	data, err := os.ReadFile(pidPath)
	if err != nil {
		return fmt.Errorf("read jailed firecracker pid for VM %s: %w", vmID, err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 0 {
		return fmt.Errorf("jailed firecracker pid file for VM %s holds %q", vmID, strings.TrimSpace(string(data)))
	}
	// The exit watch opens a pidfd before the identity check, so a recycled
	// pid can never be adopted.
	exitCh, err := watchProcessExit(pid)
	if err != nil {
		return fmt.Errorf("watch jailed firecracker pid %d for VM %s: %w", pid, vmID, err)
	}
	if !waitForJailedExec(pid, vmID, exitCh) {
		if proc, err := os.FindProcess(pid); err == nil {
			m.killProcessWithReceipt(proc, DestructionReceipt{VMID: vmID, Cause: "jailer-exec-unverified"})
		}
		return fmt.Errorf("jailed pid %d did not become Firecracker for VM %s", pid, vmID)
	}
	receipt.PID = pid
	applyLaunchOOMScoreAdj(vmID, pid)
	exited := make(chan struct{})
	go watchConsoleLog(vmID, pid, consolePath, exited)

	var expected *VMInstance
	if inst, ok := m.vms[vmID]; ok {
		expected = inst
		inst.PID = pid
		inst.cmd = nil
		inst.done = make(chan struct{})
		if err := m.savePID(vmID, pid); err != nil {
			log.Printf("vmmanager: warning: could not save pid for VM %s: %v", vmID, err)
		}
	}
	m.recordConfinementLocked(vmID, receipt)
	log.Printf("vmmanager: jailed firecracker for VM %s: pid=%d uid=%d root=%s cgroup=%s", vmID, pid, receipt.UID, receipt.JailRoot, receipt.Cgroup)

	go func() {
		<-exitCh
		close(exited)
		if expected != nil {
			close(expected.done)
			m.markJailedExit(vmID, expected, pid)
		}
		m.removeCgroupLeaf(vmID)
	}()
	return nil
}

// markJailedExit handles a jailed Firecracker's exit: an instance that should
// be running is marked failed, and, while the exited process is still the
// instance's current one (no kill or relaunch has moved it on), its tap
// network is torn down, as releaseExitedVMNetwork does for a direct VMM.
func (m *Manager) markJailedExit(vmID string, expected *VMInstance, pid int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	current, ok := m.vms[vmID]
	if !ok || current != expected {
		return
	}
	if current.State == StateRunning || current.State == StatePending {
		current.State = StateFailed
		current.Healthy = false
		log.Printf("vmmanager: jailed firecracker for VM %s exited; marked failed", vmID)
	}
	if current.cmd == nil && current.PID == pid {
		log.Printf("vmmanager: jailed Firecracker for VM %s exited; removing its tap network", vmID)
		m.deleteTapDevice(tapNameForVMID(vmID))
	}
}

// waitJailer waits for the jailer parent to exit; a jailer that hangs is
// killed (with a receipt) and the launch fails.
func (m *Manager) waitJailer(vmID string, cmd *exec.Cmd, timeout time.Duration) error {
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		return err
	case <-time.After(timeout):
		m.killProcessWithReceipt(cmd.Process, DestructionReceipt{VMID: vmID, Cause: "jailer-timeout"})
		<-done
		return fmt.Errorf("did not exit within %s", timeout)
	}
}

// waitForJailedExec waits for the namespaced child to exec Firecracker: the
// jailer writes the pid file before its child execs.
func waitForJailedExec(pid int, vmID string, exited <-chan struct{}) bool {
	deadline := time.Now().Add(jailedExecTimeout)
	for {
		if firecrackerCmdlineMatchesVM(pid, vmID) {
			return true
		}
		select {
		case <-exited:
			return false
		default:
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(10 * time.Millisecond)
	}
}
