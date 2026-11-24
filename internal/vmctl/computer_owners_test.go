package vmctl

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/hostroom"
)

// The auth service decides sign-up on which accounts already have a computer
// (their disk is in the free reading) and which are still promised one. vmctl
// is the only one that knows, and tells it through a file, with the disk each
// computer holds now. The file is SUBTRACTED from the footprint, so a figure
// too high opens a place; the disk is therefore counted only as far as it is
// measured exactly (the btrfs extent reading), and a file measured any other
// way counts 0. Failure modes guarded:
//  1. a fallback measure (FIEMAP bound, st_blocks, an error) counted: the
//     figure then runs high on a compressing disk and a hibernate opens a place;
//  2. a snapshot counted from its files with a measure that sees no
//     compression, instead of the disk_bytes its writer recorded (and counted
//     at all when that reading was not the exact one);
//  3. an account's second computer missing from the sum;
//  4. a walk of every computer's extents on every registry change;
//  5. the file lagging a change: a wake or a removal leaving the old disk in
//     the file while the free reading moves, which opens a place for the length
//     of the operation;
//  6. a failed write passing silently.
//
// One rule throughout: a file counts only as the exact btrfs reading, else 0
// and reported. A fully sparse file is not an exception: the measure reads it
// as the exact zero (vmmanager.btrfsReadingTrusted).

// exactAllocated is a measure that reports allocated bytes as the exact
// btrfs reading would (here st_blocks, the filesystem under the test being
// whatever it is).
func exactAllocated(path string) (int64, string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, "", err
	}
	if st, ok := info.Sys().(*syscall.Stat_t); ok {
		return int64(st.Blocks) * 512, "btrfs_extents", nil
	}
	return info.Size(), "btrfs_extents", nil
}

func writeFilled(t *testing.T, path string, n int) {
	t.Helper()
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(i%251 + 1)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatal(err)
	}
}

// writeSnapshot writes a memory file of logicalBytes and the meta.json its
// writer would, recording diskBytes by method.
func writeSnapshot(t *testing.T, stateDir, vmID string, logicalBytes int, diskBytes int64, method string) {
	t.Helper()
	writeFilled(t, filepath.Join(stateDir, vmID, "snapshot", "mem"), logicalBytes)
	meta, _ := json.Marshal(map[string]any{"disk_bytes": diskBytes, "disk_bytes_method": method})
	if err := os.WriteFile(filepath.Join(stateDir, vmID, "snapshot", "meta.json"), meta, 0o600); err != nil {
		t.Fatal(err)
	}
}

func ownerDisk(reg *OwnershipRegistry, user string, fresh bool) uint64 {
	for _, o := range reg.ComputerOwners(fresh) {
		if o.UserID == user {
			return o.DiskBytes
		}
	}
	return 1 << 62
}

func TestPublishedComputerOwnersAreTheAccountsThatHaveOne(t *testing.T) {
	stateDir := t.TempDir()
	reg := NewOwnershipRegistry("")
	reg.SetRetentionPruneConfig(RetentionPruneConfig{StateDir: stateDir})
	reg.diskMeasure = exactAllocated
	for _, u := range []string{"a", "b"} {
		addOwnership(reg, &VMOwnership{VMID: "vm-" + u, UserID: u, DesktopID: PrimaryDesktopID, Kind: VMKindInteractive, State: VMStateHibernated})
	}
	addOwnership(reg, &VMOwnership{VMID: "vm-c", UserID: "c", DesktopID: PrimaryDesktopID, Kind: VMKindInteractive, State: VMStateStopped})
	// a: a 4 MiB image, a snapshot the writer recorded as 2 MiB on disk, and a
	// second computer of 1 MiB.
	addOwnership(reg, &VMOwnership{VMID: "vm-a2", UserID: "a", DesktopID: "second", Kind: VMKindInteractive, State: VMStateHibernated})
	writeFilled(t, filepath.Join(stateDir, "vm-a", dataImageFileName), 4<<20)
	writeSnapshot(t, stateDir, "vm-a", 8<<20, 2<<20, "btrfs_extents")
	writeFilled(t, filepath.Join(stateDir, "vm-a2", dataImageFileName), 1<<20)
	path := filepath.Join(t.TempDir(), "host-room", "computer-owners.json")
	now := time.Now()
	if err := reg.PublishComputerOwners(path, now); err != nil {
		t.Fatal(err)
	}
	got, err := hostroom.ReadOwners(path, now.Add(time.Second), hostroom.MaxOwnersAge)
	if err != nil || len(got) != 3 {
		t.Fatalf("owners = %v, %v", got, err)
	}
	if _, ok := got["d"]; ok {
		t.Fatal("an account with no computer is listed")
	}
	// 4 MiB image + 2 MiB snapshot (its meta, not the 8 MiB of its files) + the
	// second computer's 1 MiB.
	if got["a"] < 7<<20 || got["a"] > 7<<20+256<<10 {
		t.Fatalf("a's disk = %d bytes, want about 7 MiB", got["a"])
	}
	if got["b"] != 0 || got["c"] != 0 {
		t.Fatalf("a computer with nothing readable holds %d and %d, want 0 (the whole footprint is held back)", got["b"], got["c"])
	}
	addOwnership(reg, &VMOwnership{VMID: "vm-d", UserID: "d", DesktopID: PrimaryDesktopID, Kind: VMKindInteractive, State: VMStateActive})
	if err := reg.PublishComputerOwners(path, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if got, err := hostroom.ReadOwners(path, now.Add(time.Minute), hostroom.MaxOwnersAge); err != nil {
		t.Fatal(err)
	} else if _, ok := got["d"]; !ok {
		t.Fatalf("owners after a new computer = %v", got)
	}
	if err := reg.PublishComputerOwners("/proc/no-such-dir/computer-owners.json", now); err == nil {
		t.Fatal("a failed write returned nil")
	}
}

// Failure 1 and 2: only the exact reading counts.
func TestOnlyTheExactMeasureCounts(t *testing.T) {
	for name, tc := range map[string]struct {
		measure func(string) (int64, string, error)
		meta    string
	}{
		"the FIEMAP bound":     {func(p string) (int64, string, error) { return 1 << 30, "fiemap_bound", nil }, "fiemap_bound"},
		"st_blocks":            {func(p string) (int64, string, error) { return 1 << 30, "st_blocks", nil }, "st_blocks"},
		"a mixed reading":      {func(p string) (int64, string, error) { return 1 << 30, "mixed", nil }, "mixed"},
		"a measure that fails": {func(p string) (int64, string, error) { return 0, "", os.ErrPermission }, ""},
	} {
		t.Run(name, func(t *testing.T) {
			stateDir := t.TempDir()
			reg := NewOwnershipRegistry("")
			reg.SetRetentionPruneConfig(RetentionPruneConfig{StateDir: stateDir})
			reg.diskMeasure = tc.measure
			addOwnership(reg, &VMOwnership{VMID: "vm-u", UserID: "u", DesktopID: PrimaryDesktopID, Kind: VMKindInteractive, State: VMStateHibernated})
			writeFilled(t, filepath.Join(stateDir, "vm-u", dataImageFileName), 4<<20)
			writeSnapshot(t, stateDir, "vm-u", 8<<20, 1<<30, tc.meta)
			if got := ownerDisk(reg, "u", true); got != 0 {
				t.Fatalf("a non-exact measure counted %d bytes, want 0 (the whole footprint held back)", got)
			}
			// The file says so, so the deployed proof can fail on it.
			var inexact int
			for _, o := range reg.ComputerOwners(true) {
				inexact += o.InexactFiles
			}
			if inexact == 0 {
				t.Fatal("files counted as 0 for want of an exact measure are not reported in the owners file")
			}
		})
	}
}

// A fallback cannot open a place: whatever a non-exact measure says, the
// places are those of a computer holding nothing, and never more than the
// exact reading would give.
func TestAFallbackMeasureCannotOpenAPlace(t *testing.T) {
	plan := hostroom.Plan{ReserveBytes: 8 << 20, FootprintBytes: 16 << 20}
	places := func(free uint64, reg *OwnershipRegistry) int {
		owners := map[string]uint64{"u": ownerDisk(reg, "u", true)}
		return plan.Places(free, plan.Held(owners))
	}
	build := func(asleep bool, measure func(string) (int64, string, error), meta string) *OwnershipRegistry {
		stateDir := t.TempDir()
		reg := NewOwnershipRegistry("")
		reg.SetRetentionPruneConfig(RetentionPruneConfig{StateDir: stateDir})
		reg.diskMeasure = measure
		addOwnership(reg, &VMOwnership{VMID: "vm-u", UserID: "u", DesktopID: PrimaryDesktopID, Kind: VMKindInteractive, State: VMStateHibernated})
		writeFilled(t, filepath.Join(stateDir, "vm-u", dataImageFileName), 4<<20)
		if asleep {
			writeSnapshot(t, stateDir, "vm-u", 8<<20, 8<<20, meta)
		}
		return reg
	}
	over := func(p string) (int64, string, error) { return 64 << 20, "fiemap_bound", nil } // far too high
	const free0 = 97 << 20
	awake, asleep := build(false, over, "fiemap_bound"), build(true, over, "fiemap_bound")
	a, b := places(free0, awake), places(free0-2<<20, asleep) // the snapshot took 2 MiB of the real disk
	zero := plan.Places(free0, plan.Held(map[string]uint64{"u": 0}))
	if a != zero || b > a {
		t.Fatalf("with a fallback measure the places went %d -> %d, want %d and no rise", a, b, zero)
	}
}

// A hibernate does not open a place when the figure is the exact reading and
// moves with the free reading.
func TestAHibernateDoesNotOpenAPlaceOnACompressingDisk(t *testing.T) {
	plan := hostroom.Plan{ReserveBytes: 8 << 20, FootprintBytes: 16 << 20}
	build := func(asleep bool) *OwnershipRegistry {
		stateDir := t.TempDir()
		reg := NewOwnershipRegistry("")
		reg.SetRetentionPruneConfig(RetentionPruneConfig{StateDir: stateDir})
		// The image is 4 MiB of logical bytes holding 1 MiB; the snapshot is 8
		// MiB holding 2 MiB (the writer's recorded disk_bytes).
		reg.diskMeasure = func(p string) (int64, string, error) { return 1 << 20, "btrfs_extents", nil }
		addOwnership(reg, &VMOwnership{VMID: "vm-u", UserID: "u", DesktopID: PrimaryDesktopID, Kind: VMKindInteractive, State: VMStateHibernated})
		writeFilled(t, filepath.Join(stateDir, "vm-u", dataImageFileName), 4<<20)
		if asleep {
			writeSnapshot(t, stateDir, "vm-u", 8<<20, 2<<20, "btrfs_extents")
		}
		return reg
	}
	places := func(free uint64, reg *OwnershipRegistry) int {
		return plan.Places(free, plan.Held(map[string]uint64{"u": ownerDisk(reg, "u", true)}))
	}
	const free0 = 97 << 20
	if a, b := places(free0, build(false)), places(free0-2<<20, build(true)); a != b {
		t.Fatalf("a hibernate moved the places from %d to %d", a, b)
	}
}

// Failure 4: a registry change reuses each computer's figure; the tick
// measures afresh; a state change measures that computer again.
func TestAChangePublishReusesTheDiskFigureAndTheTickMeasuresAgain(t *testing.T) {
	stateDir := t.TempDir()
	reg := NewOwnershipRegistry("")
	reg.SetRetentionPruneConfig(RetentionPruneConfig{StateDir: stateDir})
	var calls int32
	reg.diskMeasure = func(p string) (int64, string, error) {
		atomic.AddInt32(&calls, 1)
		return 1 << 20, "btrfs_extents", nil
	}
	own := &VMOwnership{VMID: "vm-u", UserID: "u", DesktopID: PrimaryDesktopID, Kind: VMKindInteractive, State: VMStateHibernated}
	addOwnership(reg, own)
	writeFilled(t, filepath.Join(stateDir, "vm-u", dataImageFileName), 1<<20)
	reg.ComputerOwners(false)
	first := atomic.LoadInt32(&calls)
	if first == 0 {
		t.Fatal("the first reading measured nothing")
	}
	reg.ComputerOwners(false)
	reg.ComputerOwners(false)
	if got := atomic.LoadInt32(&calls); got != first {
		t.Fatalf("a change-triggered publish measured again: %d calls after %d", got, first)
	}
	reg.ComputerOwners(true)
	if got := atomic.LoadInt32(&calls); got == first {
		t.Fatal("the tick did not measure afresh")
	}
	before := atomic.LoadInt32(&calls)
	reg.mu.Lock()
	own.State = VMStateActive
	reg.mu.Unlock()
	reg.ComputerOwners(false)
	if got := atomic.LoadInt32(&calls); got == before {
		t.Fatal("a state change did not measure that computer again")
	}
}

// Failure 5: before the snapshot is consumed or the state destroyed, the file
// already says so.
func TestTheFileSaysSoBeforeADiskIsReleased(t *testing.T) {
	stateDir := t.TempDir()
	reg := NewOwnershipRegistry("")
	reg.SetRetentionPruneConfig(RetentionPruneConfig{StateDir: stateDir})
	reg.diskMeasure = exactAllocated
	addOwnership(reg, &VMOwnership{VMID: "vm-u", UserID: "u", DesktopID: PrimaryDesktopID, Kind: VMKindInteractive, State: VMStateHibernated})
	writeFilled(t, filepath.Join(stateDir, "vm-u", dataImageFileName), 2<<20)
	writeSnapshot(t, stateDir, "vm-u", 4<<20, 3<<20, "btrfs_extents")
	path := filepath.Join(t.TempDir(), "computer-owners.json")
	ctx, cancel := context.WithCancel(context.Background())
	stopped := reg.StartComputerOwnersPublisher(ctx, path, time.Hour, nil)
	defer func() { cancel(); <-stopped }()
	read := func() uint64 {
		got, err := hostroom.ReadOwners(path, time.Now(), hostroom.MaxOwnersAge)
		if err != nil {
			t.Fatal(err)
		}
		return got["u"]
	}
	full := read()
	if full < 5<<20 {
		t.Fatalf("image and snapshot = %d, want about 5 MiB", full)
	}
	end := reg.beginDiskRelease("vm-u", diskReleaseSnapshot)
	if got := read(); got < 2<<20 || got >= full-(2<<20) {
		t.Fatalf("with the snapshot about to be consumed the file says %d, want only the image (about 2 MiB) already", got)
	}
	end()
	end = reg.beginDiskRelease("vm-u", diskReleaseWhole)
	if got := read(); got != 0 {
		t.Fatalf("with the state about to be destroyed the file says %d, want 0", got)
	}
	end()
}

// The real wake path releases the snapshot only after the file says so.
func TestAWakeTellsTheFileBeforeItConsumesTheSnapshot(t *testing.T) {
	stateDir := t.TempDir()
	mock := &snapshotMockVMManager{mockVMManager: &mockVMManager{getVMs: map[string]*VMInstanceInfo{}}}
	reg, initial := newSnapshotTestRegistry(t, mock, "user-wake")
	reg.SetRetentionPruneConfig(RetentionPruneConfig{StateDir: stateDir})
	reg.diskMeasure = exactAllocated
	mock.snapshotResp = &VMInstanceInfo{HostURL: initial.ComputerURL, Epoch: initial.Epoch, Healthy: true, State: "running"}
	writeFilled(t, filepath.Join(stateDir, initial.VMID, dataImageFileName), 2<<20)
	writeSnapshot(t, stateDir, initial.VMID, 4<<20, 3<<20, "btrfs_extents")
	path := filepath.Join(t.TempDir(), "computer-owners.json")
	ctx, cancel := context.WithCancel(context.Background())
	stopped := reg.StartComputerOwnersPublisher(ctx, path, time.Hour, nil)
	defer func() { cancel(); <-stopped }()
	var seen uint64 = 1 << 62
	mock.resumeHook = func(string) {
		if got, err := hostroom.ReadOwners(path, time.Now(), hostroom.MaxOwnersAge); err == nil {
			seen = got["user-wake"]
		}
	}
	if _, err := reg.ResumeVM("user-wake"); err != nil {
		t.Fatalf("resume: %v", err)
	}
	if seen > 3<<20 {
		t.Fatalf("at the moment the snapshot was consumed the file said %d bytes, want the snapshot already out of it (about 2 MiB)", seen)
	}
}

// Failure 5, the plain case: a registry change reaches the file without the tick.
func TestARegistryChangeRepublishesTheOwnersFileAtOnce(t *testing.T) {
	reg := NewOwnershipRegistry("")
	path := filepath.Join(t.TempDir(), "computer-owners.json")
	ctx, cancel := context.WithCancel(context.Background())
	stopped := reg.StartComputerOwnersPublisher(ctx, path, time.Hour, nil)
	defer func() { cancel(); <-stopped }()
	first, err := hostroom.ReadOwners(path, time.Now(), hostroom.MaxOwnersAge)
	if err != nil || len(first) != 0 {
		t.Fatalf("first publication = %v, %v", first, err)
	}
	addOwnership(reg, &VMOwnership{VMID: "vm-w", UserID: "woken", DesktopID: PrimaryDesktopID, Kind: VMKindInteractive, State: VMStateActive})
	// Every persistence path tells the publisher, not only saveLocked.
	reg.mu.Lock()
	_ = reg.writePersistenceLocked()
	reg.mu.Unlock()
	deadline := time.Now().Add(5 * time.Second)
	for {
		got, err := hostroom.ReadOwners(path, time.Now(), hostroom.MaxOwnersAge)
		if err == nil {
			if _, ok := got["woken"]; ok {
				return
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("the file did not follow the registry within 5 s (tick is an hour): %v, %v", got, err)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// A test-class computer pruned for idleness is a whole disk leaving: the file
// stops counting it before the state is destroyed, as the other prune does.
func TestTheTestIdlePruneTellsTheFileBeforeItDestroysState(t *testing.T) {
	f := newRetentionFixture(t, RetentionPruneModeActive, RetentionPruneModeActive, RetentionPruneModeOff)
	f.reg.diskMeasure = exactAllocated
	path := filepath.Join(t.TempDir(), "computer-owners.json")
	ctx, cancel := context.WithCancel(context.Background())
	stopped := f.reg.StartComputerOwnersPublisher(ctx, path, time.Hour, nil)
	defer func() { cancel(); <-stopped }()
	tester := f.own[rtTesterA]
	read := func() (uint64, bool) {
		got, err := hostroom.ReadOwners(path, time.Now(), hostroom.MaxOwnersAge)
		if err != nil {
			t.Fatal(err)
		}
		d, ok := got[rtTesterA]
		return d, ok
	}
	if d, ok := read(); !ok || d == 0 {
		t.Fatalf("before the prune the file says %d, %v, want the computer's disk", d, ok)
	}
	var atDestroy uint64 = 1 << 62
	f.mgr.destroyHook = func(vmID string) {
		if vmID == tester.VMID {
			atDestroy, _ = read()
		}
	}
	f.reg.PruneRetention(context.Background(), allowLifecycleRoute)
	if atDestroy != 0 {
		t.Fatalf("when the state was destroyed the file still said %d bytes for the computer, want 0", atDestroy)
	}
}

func TestWarmnessPolicyNormalisesTheOperatorProofSet(t *testing.T) {
	got := normalizeWarmnessPolicyConfig(WarmnessPolicyConfig{OperatorProofComputerIDs: map[string]bool{" proof-1 ": true, "off": false, "  ": true}})
	if len(got.OperatorProofComputerIDs) != 1 || !got.OperatorProofComputerIDs["proof-1"] {
		t.Fatalf("operator proof ids = %v, want only the trimmed enabled one", got.OperatorProofComputerIDs)
	}
}

// A data image the measure could not read exactly is reported on its own, with
// no snapshot to blame: the deployed proof must be able to see the image.
func TestAnInexactDataImageIsReported(t *testing.T) {
	stateDir := t.TempDir()
	reg := NewOwnershipRegistry("")
	reg.SetRetentionPruneConfig(RetentionPruneConfig{StateDir: stateDir})
	reg.diskMeasure = func(string) (int64, string, error) { return 1 << 30, "fiemap_bound", nil }
	addOwnership(reg, &VMOwnership{VMID: "vm-u", UserID: "u", DesktopID: PrimaryDesktopID, Kind: VMKindInteractive, State: VMStateActive})
	writeFilled(t, filepath.Join(stateDir, "vm-u", dataImageFileName), 1<<20)
	got := reg.ComputerOwners(true)
	if len(got) != 1 || got[0].DiskBytes != 0 || got[0].InexactFiles != 1 {
		t.Fatalf("owners = %+v, want 0 bytes and one inexact file", got)
	}
}

// A fully sparse image holds exactly 0, and the measure says so as the exact
// btrfs reading (vmmanager.btrfsReadingTrusted), so it counts 0 without being
// reported inexact; a measure that falls back for it would be reported.
func TestAFullySparseImageMeasuredExactlyIsNotInexact(t *testing.T) {
	stateDir := t.TempDir()
	reg := NewOwnershipRegistry("")
	reg.SetRetentionPruneConfig(RetentionPruneConfig{StateDir: stateDir})
	reg.diskMeasure = func(string) (int64, string, error) { return 0, "btrfs_extents", nil }
	addOwnership(reg, &VMOwnership{VMID: "vm-u", UserID: "u", DesktopID: PrimaryDesktopID, Kind: VMKindInteractive, State: VMStateActive})
	if err := os.MkdirAll(filepath.Join(stateDir, "vm-u"), 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(filepath.Join(stateDir, "vm-u", dataImageFileName))
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(32 << 20); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	got := reg.ComputerOwners(true)
	if len(got) != 1 || got[0].DiskBytes != 0 || got[0].InexactFiles != 0 {
		t.Fatalf("owners = %+v, want 0 bytes and no inexact file for an image measured as an exact 0", got)
	}
	reg.diskMeasure = func(string) (int64, string, error) { return 0, "fiemap_bound", nil }
	if got := reg.ComputerOwners(true); len(got) != 1 || got[0].InexactFiles != 1 {
		t.Fatalf("owners = %+v, want a fallback measure reported inexact", got)
	}
}
