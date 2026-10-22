package vmmanager

import (
	"bufio"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

// L3a: Diff snapshots of cold-booted guests (RES-13;
// docs/design/fp10-snapshot-levers-and-host-slab-2026-10-22.md §2 L3a).
//
// With dirty-page tracking off (vmmanager never sets track_dirty_pages),
// Firecracker 1.13 and later write a Diff snapshot from mincore(2): only
// resident pages, into a sparse file of the full memory length. For a
// process that was cold booted, a non-resident page of anonymous guest
// memory that cannot swap was never touched, or was discarded (free page
// reporting, virtio-mem unplug), and reads as zero, which is what a hole
// reads as. So the Diff file is a complete image and loads unchanged.
//
// Every condition below must hold, else the hibernate takes Full as
// before. VM_SNAPSHOT_TYPE defaults to full and VM_SNAPSHOT_DIFF_ONLY to
// no computer, so a deploy of this code changes no computer.

const (
	// SnapshotTypeConfigFull and SnapshotTypeConfigDiff are the values of
	// VM_SNAPSHOT_TYPE. Anything else reads as full.
	SnapshotTypeConfigFull = "full"
	SnapshotTypeConfigDiff = "diff"

	// snapshotTypeFull and snapshotTypeDiff are Firecracker's snapshot_type
	// values, recorded in SnapshotMeta.SnapshotType.
	snapshotTypeFull = "Full"
	snapshotTypeDiff = "Diff"

	// Firecracker 1.15.0 fixed a Diff corruption bug (#5705).
	diffMinFirecrackerMajor = 1
	diffMinFirecrackerMinor = 15

	// diffGuestMappingMinKB is the size from which a mapping of the
	// Firecracker process counts as guest memory. Firecracker's own
	// mappings are a few MiB at most.
	diffGuestMappingMinKB = 32 << 10
)

// validateSnapshot's Diff refusals (diffSnapshotRefusalReason).
const (
	snapshotReasonSnapshotType = "snapshot_type_unknown"
	snapshotReasonDiffNotCold  = "diff_not_cold_origin"
	snapshotReasonDiffLength   = "diff_memory_length_mismatch"
	snapshotReasonDiffLeverOff = "diff_lever_off"
)

// Why a hibernate took Full instead of Diff (SnapshotTimeline.DiffRefused).
const (
	diffRefusedOff           = "diff_off"
	diffRefusedNotSelected   = "diff_not_selected"
	diffRefusedNotColdOrigin = "not_cold_origin"
	diffRefusedNoPID         = "no_firecracker_pid"
	diffRefusedFirecracker   = "firecracker_below_1_15"
	diffRefusedMaySwap       = "guest_memory_may_swap"
	diffRefusedTHP           = "guest_memory_thp_or_unknown"
	diffRefusedCreateFailed  = "diff_create_failed"
	// diffRefusedHotplugShape: the launch's virtio-mem device and vmctl's
	// hotplug selection disagree, so the memory file's length would not be
	// the one the meta expects (round 2, F1).
	diffRefusedHotplugShape = "hotplug_shape_mismatch"
	// diffRefusedLengthAtCreate: the Diff file was not (mem + hotplug) MiB
	// long after the create; the hibernate took Full instead.
	diffRefusedLengthAtCreate = "diff_length_mismatch_at_create"
)

// normalizeSnapshotTypeConfig reads VM_SNAPSHOT_TYPE: diff, or full.
func normalizeSnapshotTypeConfig(raw string) string {
	if strings.EqualFold(strings.TrimSpace(raw), SnapshotTypeConfigDiff) {
		return SnapshotTypeConfigDiff
	}
	return SnapshotTypeConfigFull
}

// diffSnapshotRefusal returns "" when a hibernate of this process may take
// a Diff snapshot, else why not. It only reads, and runs before the pause.
// firecrackerBanner is called only once the cheap checks pass, so a
// hibernate in full mode never runs `firecracker --version` for it.
func (m *Manager) diffSnapshotRefusal(cfg VMConfig, coldOrigin bool, pid int, launchHasHotplug bool, firecrackerBanner func() string) string {
	if normalizeSnapshotTypeConfig(m.cfg.SnapshotType) != SnapshotTypeConfigDiff {
		return diffRefusedOff
	}
	if !selectsComputer(m.cfg.SnapshotDiffOnly, cfg) {
		return diffRefusedNotSelected
	}
	if !coldOrigin {
		return diffRefusedNotColdOrigin
	}
	if pid <= 0 {
		return diffRefusedNoPID
	}
	if launchHasHotplug != (cfg.MemoryHotplugMiB > 0) {
		return diffRefusedHotplugShape
	}
	if !firecrackerAtLeast(firecrackerBanner(), diffMinFirecrackerMajor, diffMinFirecrackerMinor) {
		return diffRefusedFirecracker
	}
	if !guestMemoryCannotSwap(m.procRootDir(), m.cgroupRootDir(), pid) {
		return diffRefusedMaySwap
	}
	if !guestMemoryHasNoTHP(m.procRootDir(), pid) {
		return diffRefusedTHP
	}
	return ""
}

func (m *Manager) procRootDir() string {
	if m.procRoot != "" {
		return m.procRoot
	}
	return "/proc"
}

func (m *Manager) cgroupRootDir() string {
	if m.cgroupRoot != "" {
		return m.cgroupRoot
	}
	return defaultCgroupRoot
}

// firecrackerAtLeast reports whether a `firecracker --version` banner
// ("Firecracker v1.16.2") is at least major.minor. Anything it cannot
// parse is "no".
func firecrackerAtLeast(banner string, major, minor int) bool {
	fields := strings.Fields(banner)
	if len(fields) == 0 {
		return false
	}
	parts := strings.SplitN(strings.TrimPrefix(fields[len(fields)-1], "v"), ".", 3)
	if len(parts) != 3 || parts[2] == "" || parts[2][0] < '0' || parts[2][0] > '9' {
		return false
	}
	gotMajor, err := strconv.Atoi(parts[0])
	if err != nil {
		return false
	}
	gotMinor, err := strconv.Atoi(parts[1])
	if err != nil {
		return false
	}
	if gotMajor != major {
		return gotMajor > major
	}
	return gotMinor >= minor
}

// guestMemoryCannotSwap reports whether the process has no swapped memory
// and some cgroup from its own up to the root reads memory.swap.max 0. The
// cgroup v2 limit is hierarchical, so the nearest ancestor that sets 0
// bounds the leaf (reading R2: the leaf go-choir-vmctl.service/supervisor
// has no memory controller; the service reads 0; under the jailer the leaf
// sits under the same service). Anything unreadable is "may swap".
func guestMemoryCannotSwap(procRoot, cgroupRoot string, pid int) bool {
	pidDir := filepath.Join(procRoot, strconv.Itoa(pid))
	if swap, ok := procStatusKB(filepath.Join(pidDir, "status"), "VmSwap:"); !ok || swap != 0 {
		return false
	}
	data, err := os.ReadFile(filepath.Join(pidDir, "cgroup"))
	if err != nil {
		return false
	}
	rel := ""
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "0::") {
			rel = strings.TrimSpace(strings.TrimPrefix(line, "0::"))
			break
		}
	}
	if !strings.HasPrefix(rel, "/") {
		return false
	}
	for p := path.Clean(rel); ; p = path.Dir(p) {
		if raw, err := os.ReadFile(filepath.Join(cgroupRoot, filepath.FromSlash(p), "memory.swap.max")); err == nil && strings.TrimSpace(string(raw)) == "0" {
			return true
		}
		if p == "/" {
			return false
		}
	}
}

// procStatusKB reads one "<key> <n> kB" line of /proc/<pid>/status.
func procStatusKB(file, key string) (int64, bool) {
	data, err := os.ReadFile(file)
	if err != nil {
		return 0, false
	}
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, key) {
			continue
		}
		fields := strings.Fields(strings.TrimPrefix(line, key))
		if len(fields) == 0 {
			return 0, false
		}
		n, err := strconv.ParseInt(fields[0], 10, 64)
		return n, err == nil
	}
	return 0, false
}

// guestMemoryHasNoTHP reports whether every guest memory mapping of the
// process (every mapping of diffGuestMappingMinKB or more) reads
// THPeligible 0, and at least one exists (reading R5). The host's global
// THP mode is not the test: Node B is madvise and Firecracker does not
// madvise guest memory.
func guestMemoryHasNoTHP(procRoot string, pid int) bool {
	f, err := os.Open(filepath.Join(procRoot, strconv.Itoa(pid), "smaps"))
	if err != nil {
		return false
	}
	defer f.Close()
	found := 0
	inGuest, sawTHP := false, false
	closeMapping := func() bool { return !inGuest || sawTHP }
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if isSmapsHeader(fields[0]) {
			if !closeMapping() {
				return false
			}
			inGuest, sawTHP = false, false
			continue
		}
		switch fields[0] {
		case "Size:":
			if len(fields) >= 2 {
				if kb, err := strconv.ParseInt(fields[1], 10, 64); err == nil && kb >= diffGuestMappingMinKB {
					inGuest = true
					found++
				}
			}
		case "THPeligible:":
			if inGuest {
				if len(fields) < 2 || fields[1] != "0" {
					return false
				}
				sawTHP = true
			}
		}
	}
	if sc.Err() != nil || !closeMapping() {
		return false
	}
	return found > 0
}

// isSmapsHeader reports whether a smaps field is a mapping's address range.
func isSmapsHeader(field string) bool {
	lo, hi, ok := strings.Cut(field, "-")
	if !ok || lo == "" || hi == "" {
		return false
	}
	for _, s := range []string{lo, hi} {
		if _, err := strconv.ParseUint(s, 16, 64); err != nil {
			return false
		}
	}
	return true
}

// diffSnapshotRefusalReason is validateSnapshot's Diff rule: "" for a Full
// snapshot (or one from before L3a, which has no type), and for a Diff whose
// capture was cold origin and whose memory file is exactly the guest's
// memory length, base plus the virtio-mem region (reading R3: 8 + 4 GiB).
func (m *Manager) diffSnapshotRefusalReason(dir string, meta *SnapshotMeta) string {
	switch meta.SnapshotType {
	case "", snapshotTypeFull:
		return ""
	case snapshotTypeDiff:
	default:
		return snapshotReasonSnapshotType
	}
	if !meta.ColdOrigin {
		return snapshotReasonDiffNotCold
	}
	// The lever must still select the computer: turning it off (the
	// rollback) cold boots a pending Diff instead of loading it (F4).
	if normalizeSnapshotTypeConfig(m.cfg.SnapshotType) != SnapshotTypeConfigDiff || !selectsComputer(m.cfg.SnapshotDiffOnly, meta.Config) {
		return snapshotReasonDiffLeverOff
	}
	info, err := os.Stat(filepath.Join(dir, snapshotMemName))
	if err != nil {
		return snapshotReasonFilesMissing
	}
	want := int64(meta.MemSizeMiB+meta.Config.MemoryHotplugMiB) << 20
	if meta.MemSizeMiB <= 0 || info.Size() != want {
		return snapshotReasonDiffLength
	}
	return ""
}

// diffMemoryLength is the length a complete memory file of cfg's guest has:
// base memory plus the virtio-mem region (reading R3: 8 + 4 GiB).
func diffMemoryLength(cfg VMConfig) int64 {
	return int64(cfg.MachineMemSizeMib+cfg.MemoryHotplugMiB) << 20
}
