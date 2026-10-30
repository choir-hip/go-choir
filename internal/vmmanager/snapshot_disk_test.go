package vmmanager

import (
	"crypto/rand"
	"encoding/binary"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

// Real-disk snapshot budget and dead-snapshot cleanup
// (docs/archive/reports/s3-snapshot-hibernate-design-2026-10-14.md, items 39-48).
//
// Ways this could fail, written first:
//   - the budget still counts logical bytes, so btrfs-compressed snapshots
//     (~6% of logical on Node B) are evicted or refused far too early;
//   - snapshots committed before this change (no disk_bytes in metadata)
//     count as zero, so the budget is silently unbounded;
//   - the new snapshot is reserved at its compressed guess instead of its
//     worst case, so incompressible memory overruns the budget;
//   - the btrfs extent parser double counts a shared extent, counts holes,
//     misreads inline extents, or reads items of another inode;
//   - the measure is not recorded, so staging cannot compare it with
//     compsize;
//   - a stop-time discard blocks behind a manager operation, or deletes the
//     snapshot a resume is loading;
//   - the start sweep deletes a hibernated computer's snapshot, follows a
//     symlink out of the state dir, or keeps interrupted captures.

// fakeDiskBytes makes every snapshot file occupy pct percent of its logical
// size on disk, as btrfs zstd does with guest memory.
func fakeDiskBytes(t *testing.T, pct int64) {
	t.Helper()
	prev := fileDiskBytes
	fileDiskBytes = func(path string) (int64, string, error) {
		info, err := os.Stat(path)
		if err != nil {
			return 0, "", err
		}
		return info.Size() * pct / 100, diskBytesMethodBtrfs, nil
	}
	t.Cleanup(func() { fileDiskBytes = prev })
}

func writeFakeSnapshot(t *testing.T, mgr *Manager, vmID string, logical int64, created time.Time, diskBytes int64, port int) {
	t.Helper()
	dir := mgr.snapshotDir(vmID)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	mem, err := os.Create(filepath.Join(dir, snapshotMemName))
	if err != nil {
		t.Fatal(err)
	}
	if err := mem.Truncate(logical); err != nil {
		t.Fatal(err)
	}
	_ = mem.Close()
	meta := &SnapshotMeta{Version: snapshotMetaVersion, VMID: vmID, HostPort: port, CreatedAt: created, Bytes: logical, DiskBytes: diskBytes}
	if diskBytes > 0 {
		meta.DiskBytesMethod = diskBytesMethodBtrfs
	}
	if err := writeSnapshotMeta(dir, meta); err != nil {
		t.Fatal(err)
	}
	mgr.reserveSnapshotPort(vmID, port)
}

// Items 39, 41, 42: committed snapshots count at their on-disk bytes (from
// metadata, or measured for snapshots that predate the field); the new one
// is reserved at its logical worst case.
func TestSnapshotBudgetCountsOnDiskBytes(t *testing.T) {
	stateDir := shortStateDir(t)
	cfg := DefaultManagerConfig()
	cfg.StateDir = stateDir
	cfg.SnapshotBudgetGiB = 48
	mgr := NewManager(cfg)
	prevStatfs := statfsAvailable
	statfsAvailable = func(string) (int64, error) { return 1 << 50, nil }
	t.Cleanup(func() { statfsAvailable = prevStatfs })
	fakeDiskBytes(t, 6)

	const logical = int64(8) << 30
	const onDisk = int64(600) << 20
	need := logical + snapshotStateOverheadBytes
	base := time.Now().Add(-time.Hour)
	vms := make([]string, 0, 70)
	for i := 0; i < 70; i++ {
		vm := "vm-" + strings.Repeat("0", 3-len(itoa(i))) + itoa(i)
		vms = append(vms, vm)
		recorded := onDisk
		if i%2 == 1 {
			// Committed before this change: no disk_bytes; measured live
			// (6% of 8 GiB = 491.5 MiB).
			recorded = 0
		}
		writeFakeSnapshot(t, mgr, vm, logical, base.Add(time.Duration(i)*time.Second), recorded, 100+i)
	}
	// 35*600 MiB + 35*491.5 MiB = 37.3 GiB; plus 8.06 GiB worst case for
	// the new one = 45.4 GiB <= 48 GiB: no eviction. A logical budget would
	// have admitted 5.
	if !mgr.ensureSnapshotRoom("vm-x", need) {
		t.Fatal("70 compressed snapshots fit a 48 GiB budget by real disk use")
	}
	for _, vm := range vms {
		if _, err := os.Stat(mgr.snapshotDir(vm)); err != nil {
			t.Fatalf("%s evicted although the budget had room: %v", vm, err)
		}
	}

	// Eight more recorded snapshots: 37.3 + 4.7 = 42.0 GiB, + 8.06 = 50.1:
	// the oldest go until the worst-case reservation fits.
	for i := 70; i < 78; i++ {
		vm := "vm-" + itoa(i)
		vms = append(vms, vm)
		writeFakeSnapshot(t, mgr, vm, logical, base.Add(time.Duration(i)*time.Second), onDisk, 100+i)
	}
	if !mgr.ensureSnapshotRoom("vm-x", need) {
		t.Fatal("expected room after evicting the oldest snapshots")
	}
	evicted := 0
	for i, vm := range vms {
		_, err := os.Stat(mgr.snapshotDir(vm))
		if os.IsNotExist(err) {
			evicted++
			if i > 4 {
				t.Fatalf("%s evicted out of age order", vm)
			}
		}
	}
	if evicted < 2 || evicted > 5 {
		t.Fatalf("evicted %d snapshots, want the few oldest needed to fit", evicted)
	}
	var used int64
	for _, entry := range mgr.listSnapshots() {
		used += entry.bytes
	}
	if used+need > mgr.snapshotBudgetBytes() {
		t.Fatalf("used %d + need %d exceeds budget %d", used, need, mgr.snapshotBudgetBytes())
	}

	// Item 41: the new snapshot is reserved at its worst case, never at a
	// compressed guess: a single snapshot larger than the budget is refused.
	if mgr.ensureSnapshotRoom("vm-x", int64(49)<<30) {
		t.Fatal("a worst case larger than the whole budget must be refused")
	}
	// Item 43: the free-space floor still guards the worst case.
	statfsAvailable = func(string) (int64, error) { return snapshotDiskFloorBytes + need - 1, nil }
	if mgr.ensureSnapshotRoom("vm-x", need) {
		t.Fatal("the disk floor must refuse a worst case that would cross it")
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	return string(b)
}

// Item 40: a capture records logical and on-disk bytes, and the method, in
// metadata and in the hibernate timeline.
func TestHibernateRecordsLogicalAndDiskBytes(t *testing.T) {
	f := newSnapshotFixture(t)
	f.fc.memBytes = 1 << 20
	fakeDiskBytes(t, 6)
	f.serveLaunchAPI(t)
	if err := f.mgr.HibernateVM(f.vmID); err != nil {
		t.Fatalf("HibernateVM: %v", err)
	}
	meta, err := readSnapshotMeta(f.mgr.snapshotDir(f.vmID))
	if err != nil {
		t.Fatalf("committed snapshot: %v", err)
	}
	if meta.Bytes < 1<<20 || meta.DiskBytes <= 0 || meta.DiskBytes >= meta.Bytes || meta.DiskBytesMethod != diskBytesMethodBtrfs {
		t.Fatalf("meta bytes=%d disk_bytes=%d method=%q", meta.Bytes, meta.DiskBytes, meta.DiskBytesMethod)
	}
	tl := readHibernateTimeline(t, f.mgr, f.vmID)
	if tl.Snapshot.Bytes != meta.Bytes || tl.Snapshot.DiskBytes != meta.DiskBytes || tl.Snapshot.DiskBytesMethod != diskBytesMethodBtrfs {
		t.Fatalf("hibernate timeline snapshot = %+v", tl.Snapshot)
	}
	entries := f.mgr.listSnapshots()
	if len(entries) != 1 || entries[0].bytes != meta.DiskBytes {
		t.Fatalf("budget entry = %+v, want disk bytes %d", entries, meta.DiskBytes)
	}
}

// Item 40: the real measure on this host's filesystem returns a positive
// size for written data. On btrfs without CAP_SYS_ADMIN it is the FIEMAP
// bound; on every other filesystem, st_blocks*512.
func TestFileDiskBytesMeasuresWrittenData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mem")
	data := make([]byte, 1<<20)
	if _, err := rand.Read(data); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := syncRegularFile(path); err != nil {
		t.Fatal(err)
	}
	n, method, err := fileDiskBytes(path)
	if err != nil {
		t.Fatalf("fileDiskBytes: %v", err)
	}
	if method != diskBytesMethodBtrfs && method != diskBytesMethodFiemapBound && method != diskBytesMethodStBlocks {
		t.Fatalf("method = %q", method)
	}
	// Random data does not compress: on-disk is at least most of it.
	if n < int64(len(data))/2 {
		t.Fatalf("disk bytes = %d for %d random bytes (method %s)", n, len(data), method)
	}
}

// btrfsItem appends one search header and item to buf.
func btrfsItem(buf []byte, objectid, offset uint64, typ uint32, item []byte) []byte {
	hdr := make([]byte, btrfsSearchHeaderSize)
	binary.NativeEndian.PutUint64(hdr[8:], objectid)
	binary.NativeEndian.PutUint64(hdr[16:], offset)
	binary.NativeEndian.PutUint32(hdr[24:], typ)
	binary.NativeEndian.PutUint32(hdr[28:], uint32(len(item)))
	return append(append(buf, hdr...), item...)
}

func btrfsRegularExtent(extentType byte, compression byte, bytenr, diskNum, numBytes uint64) []byte {
	item := make([]byte, btrfsFileExtentRegularSize)
	binary.LittleEndian.PutUint64(item[8:], numBytes) // ram_bytes
	item[16] = compression
	item[20] = extentType
	binary.LittleEndian.PutUint64(item[21:], bytenr)
	binary.LittleEndian.PutUint64(item[29:], diskNum)
	binary.LittleEndian.PutUint64(item[45:], numBytes)
	return item
}

// Item 40: the extent parser sums each distinct extent's disk bytes once,
// counts inline extents by their inline length, skips holes and items of
// other inodes or types, and reports the last key offset for paging.
func TestParseBtrfsExtentItems(t *testing.T) {
	const ino = 257
	var buf []byte
	buf = btrfsItem(buf, ino, 0, btrfsExtentDataKey, btrfsRegularExtent(1, 3, 1<<30, 4096, 128<<10))        // zstd: 4 KiB on disk
	buf = btrfsItem(buf, ino, 128<<10, btrfsExtentDataKey, btrfsRegularExtent(1, 3, 1<<30, 4096, 64<<10))   // same extent, referenced again
	buf = btrfsItem(buf, ino, 192<<10, btrfsExtentDataKey, btrfsRegularExtent(1, 0, 0, 0, 1<<20))           // hole
	buf = btrfsItem(buf, ino, 2<<20, btrfsExtentDataKey, btrfsRegularExtent(1, 0, 2<<30, 128<<10, 128<<10)) // uncompressed
	buf = btrfsItem(buf, ino, 3<<20, btrfsExtentDataKey, btrfsRegularExtent(2, 0, 3<<30, 64<<10, 64<<10))   // prealloc
	inline := make([]byte, btrfsFileExtentInlineHeaderSize+100)
	inline[20] = 0
	buf = btrfsItem(buf, ino, 4<<20, btrfsExtentDataKey, inline)
	buf = btrfsItem(buf, ino+1, 0, btrfsExtentDataKey, btrfsRegularExtent(1, 0, 4<<30, 1<<20, 1<<20)) // another inode
	buf = btrfsItem(buf, ino, 5<<20, 1, []byte{1, 2, 3})                                              // another item type

	seen := map[uint64]struct{}{}
	sum, last, err := parseBtrfsExtentItems(buf, 8, ino, seen)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	want := int64(4096 + 128<<10 + 64<<10 + 100)
	if sum != want {
		t.Fatalf("sum = %d, want %d", sum, want)
	}
	if last != 5<<20 {
		t.Fatalf("last offset = %d, want %d", last, 5<<20)
	}
	// A later page that references an extent already counted adds nothing.
	page2 := btrfsItem(nil, ino, 6<<20, btrfsExtentDataKey, btrfsRegularExtent(1, 3, 1<<30, 4096, 4096))
	sum2, _, err := parseBtrfsExtentItems(page2, 1, ino, seen)
	if err != nil || sum2 != 0 {
		t.Fatalf("page 2 sum = %d (%v), want 0", sum2, err)
	}
	if _, _, err := parseBtrfsExtentItems(buf[:40], 2, ino, map[uint64]struct{}{}); err == nil {
		t.Fatal("a truncated buffer must be an error, never a short count")
	}
}

// Items 44, 45: a stop-time discard removes the snapshot and its port, and
// never waits for or races a manager operation on the same VM.
func TestDiscardSnapshotIsNonBlocking(t *testing.T) {
	cfg := DefaultManagerConfig()
	cfg.StateDir = shortStateDir(t)
	mgr := NewManager(cfg)
	writeFakeSnapshot(t, mgr, "vm-a", 4096, time.Now(), 4096, 101)

	unlock := mgr.lockVMOperation("vm-a")
	done := make(chan bool, 1)
	go func() { done <- mgr.DiscardSnapshot("vm-a", "stopped") }()
	select {
	case removed := <-done:
		if removed {
			t.Fatal("a discard must not remove a snapshot while the VM is mid-operation")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("DiscardSnapshot blocked behind the VM's operation lock")
	}
	unlock()
	if _, err := os.Stat(mgr.snapshotDir("vm-a")); err != nil {
		t.Fatal("snapshot of a VM mid-operation must survive the discard")
	}

	if !mgr.DiscardSnapshot("vm-a", "stopped") {
		t.Fatal("expected the snapshot to be discarded")
	}
	if _, err := os.Stat(mgr.snapshotDir("vm-a")); !os.IsNotExist(err) {
		t.Fatal("snapshot dir must be gone after the discard")
	}
	if mgr.snapshotPortHeldByOther("vm-other", 101) {
		t.Fatal("a discarded snapshot must release its host port")
	}
	if mgr.DiscardSnapshot("vm-a", "stopped") {
		t.Fatal("a second discard has nothing to remove")
	}
}

// Items 46, 47: the start sweep removes committed snapshots of VMs that are
// not hibernated and every interrupted capture; it never removes a
// hibernated computer's snapshot, a VM mid-operation, or anything behind a
// symlink.
func TestSweepOrphanSnapshotsNeverDeletesHibernated(t *testing.T) {
	cfg := DefaultManagerConfig()
	cfg.StateDir = shortStateDir(t)
	mgr := NewManager(cfg)
	now := time.Now()
	writeFakeSnapshot(t, mgr, "vm-hibernated", 4096, now, 4096, 101)
	writeFakeSnapshot(t, mgr, "vm-held-hibernated", 4096, now, 4096, 102)
	writeFakeSnapshot(t, mgr, "vm-stopped", 4096, now, 4096, 103)
	writeFakeSnapshot(t, mgr, "vm-unknown", 4096, now, 4096, 104)
	writeFakeSnapshot(t, mgr, "vm-busy", 4096, now, 4096, 105)
	if err := os.MkdirAll(mgr.snapshotPartialDir("vm-hibernated"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(mgr.snapshotPartialDir("vm-running"), 0o700); err != nil {
		t.Fatal(err)
	}
	// A symlinked VM dir pointing outside the state dir is never followed.
	outside := t.TempDir()
	if err := os.MkdirAll(filepath.Join(outside, snapshotDirName), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(cfg.StateDir, "vm-link")); err != nil {
		t.Fatal(err)
	}

	hibernated := map[string]bool{"vm-hibernated": true, "vm-held-hibernated": true}
	unlockBusy := mgr.lockVMOperation("vm-busy")
	removed := mgr.SweepOrphanSnapshots(func(vmID string) bool { return hibernated[vmID] })
	unlockBusy()
	sort.Strings(removed)

	want := []string{
		mgr.snapshotPartialDir("vm-hibernated"),
		mgr.snapshotPartialDir("vm-running"),
		mgr.snapshotDir("vm-stopped"),
		mgr.snapshotDir("vm-unknown"),
	}
	sort.Strings(want)
	if strings.Join(removed, ",") != strings.Join(want, ",") {
		t.Fatalf("removed = %v, want %v", removed, want)
	}
	for _, vm := range []string{"vm-hibernated", "vm-held-hibernated", "vm-busy"} {
		if _, err := os.Stat(mgr.snapshotDir(vm)); err != nil {
			t.Fatalf("%s snapshot must survive the sweep: %v", vm, err)
		}
	}
	if _, err := os.Stat(filepath.Join(outside, snapshotDirName)); err != nil {
		t.Fatal("the sweep followed a symlink out of the state dir")
	}
	if !mgr.snapshotPortHeldByOther("vm-x", 101) || mgr.snapshotPortHeldByOther("vm-x", 103) {
		t.Fatal("the sweep must keep hibernated reservations and release orphaned ones")
	}

	// No keep function: nothing committed is removed.
	writeFakeSnapshot(t, mgr, "vm-stopped", 4096, now, 4096, 103)
	if got := mgr.SweepOrphanSnapshots(nil); len(got) != 0 {
		t.Fatalf("a sweep without a keep set removed %v", got)
	}
}
