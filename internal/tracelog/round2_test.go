package tracelog

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"testing"
	"time"
)

// Round 2 (review bunch 132): resync past a damaged record, unknown versions
// never served around, and the smaller findings.

type logCapture struct {
	mu   sync.Mutex
	msgs []string
}

func (c *logCapture) logf(format string, args ...any) {
	c.mu.Lock()
	c.msgs = append(c.msgs, fmt.Sprintf(format, args...))
	c.mu.Unlock()
}

func (c *logCapture) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.msgs)
}

// offsets maps stream_seq to the record's offset and length in a segment,
// reading the intact file.
func offsets(t *testing.T, dir string, segno uint64) map[int64][2]int64 {
	t.Helper()
	var starts []int64
	byOff := map[int64]int64{}
	_ = OpenReader(dir).Scan(func(it Item) bool {
		if it.Segment == segno {
			starts = append(starts, it.Offset)
			if it.Boundary == nil {
				byOff[it.Offset] = it.Meta.StreamSeq
			}
		}
		return true
	})
	info, err := os.Stat(filepath.Join(dir, tlog(segno)))
	if err != nil {
		t.Fatal(err)
	}
	sort.Slice(starts, func(i, j int) bool { return starts[i] < starts[j] })
	out := map[int64][2]int64{}
	for i, off := range starts {
		end := info.Size()
		if i+1 < len(starts) {
			end = starts[i+1]
		}
		if s, ok := byOff[off]; ok {
			out[s] = [2]int64{off, end - off}
		}
	}
	return out
}

func syncedSix(t *testing.T) (string, map[int64][2]int64) {
	t.Helper()
	src := t.TempDir()
	l, _ := openT(t, src, nil)
	for i := 1; i <= 6; i++ {
		mustAppend(t, l, i)
	}
	if err := l.Sync(); err != nil {
		t.Fatal(err)
	}
	return copyDir(t, src), offsets(t, src, 1)
}

func flipAt(t *testing.T, path string, at int64) {
	t.Helper()
	raw := mustRead(t, path)
	raw[at] ^= 0xff
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
}

// Required 1. One flipped byte in record 3 of a synced segment costs record 3
// and nothing else.
func TestFlippedByteInRecord3KeepsRecords1_2_And4Onward(t *testing.T) {
	img, offs := syncedSix(t)
	flipAt(t, filepath.Join(img, tlog(1)), offs[3][0]+12)

	// An unsealed segment read as it lies.
	streams, _ := scanDir(t, img)
	if !eqInts(streams, []int64{1, 2, 4, 5, 6}) {
		t.Fatalf("unsealed scan = %v, want [1 2 4 5 6]", streams)
	}

	var cap logCapture
	l, err := Open(Options{Dir: img, ComputerID: "computer-1", SyncInterval: -1, Logf: cap.logf})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	st := l.Stats()
	if st.DamagedRecords != 1 || st.DamagedBytes != offs[3][1] || st.TornTailsCut != 0 {
		t.Fatalf("stats = damaged_records %d damaged_bytes %d (want 1, %d) torn %d", st.DamagedRecords, st.DamagedBytes, offs[3][1], st.TornTailsCut)
	}
	if cap.count() != 1 {
		t.Fatalf("damage was logged %d times, want once per segment: %v", cap.count(), cap.msgs)
	}
	mustAppend(t, l, 7)
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	// The sealed segment: nothing truncated, the damage is in the index, and
	// every reader path returns every verified record.
	if got := mustRead(t, filepath.Join(img, tlog(1))); int64(len(got)) != offs[6][0]+offs[6][1] {
		t.Fatalf("segment 1 changed size to %d", len(got))
	}
	streams, _ = scanDir(t, img)
	if !eqInts(streams, []int64{1, 2, 4, 5, 6, 7}) {
		t.Fatalf("sealed scan = %v", streams)
	}
	r := OpenReader(img)
	recs, err := r.ByRun("owner-1", "run-a", 0, 100)
	if err != nil || len(recs) != 6 {
		t.Fatalf("ByRun = %d records, %v", len(recs), err)
	}
	segs, _ := r.Segments()
	if segs[0].DamagedRecords != 1 || segs[0].DamagedBytes != offs[3][1] {
		t.Fatalf("segment info = %+v", segs[0])
	}
	// The index loses nothing when it is rebuilt from the file.
	if err := os.Remove(filepath.Join(img, tidx(1))); err != nil {
		t.Fatal(err)
	}
	if recs, _ := OpenReader(img).ByRun("owner-1", "run-a", 0, 100); len(recs) != 6 {
		t.Fatalf("ByRun without the index = %d records", len(recs))
	}
}

func TestResyncFindsTheNextVerifiedRecord(t *testing.T) {
	img, offs := syncedSix(t)
	path := filepath.Join(img, tlog(1))
	flipAt(t, path, offs[3][0]+12)
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	info, _ := f.Stat()
	next, found, err := resync(f, offs[3][0], info.Size())
	if err != nil || !found || next != offs[4][0] {
		t.Fatalf("resync = %d %v %v, want %d", next, found, err, offs[4][0])
	}
	// Nothing valid after the last record's damage: not found.
	flipAt(t, path, offs[6][0]+12)
	f2, _ := os.Open(path)
	defer f2.Close()
	if _, found, err := resync(f2, offs[6][0], info.Size()); err != nil || found {
		t.Fatalf("resync past the end = found %v err %v", found, err)
	}
}

// A payload that embeds a header-shaped pattern: the record's own CRC decides.
func fakeHeader(bodyLen uint32) []byte {
	h := make([]byte, recHeaderSize+int(bodyLen))
	binary.BigEndian.PutUint32(h[0:4], bodyLen)
	binary.BigEndian.PutUint32(h[4:8], 0xdeadbeef) // wrong checksum
	h[8] = typeLine
	return h
}

func TestResyncIgnoresAFakeHeaderInsideAPayload(t *testing.T) {
	src := t.TempDir()
	l, _ := openT(t, src, nil)
	mustAppend(t, l, 1)
	mustAppend(t, l, 2)
	if err := l.Append(meta(3), append([]byte("pre-"), fakeHeader(20)...)); err != nil {
		t.Fatal(err)
	}
	for i := 4; i <= 6; i++ {
		mustAppend(t, l, i)
	}
	if err := l.Sync(); err != nil {
		t.Fatal(err)
	}
	offs := offsets(t, src, 1)
	img := copyDir(t, src)

	// Intact: the line with the embedded pattern is read whole, no damage.
	var got []byte
	_ = OpenReader(img).Scan(func(it Item) bool {
		if it.Boundary == nil && it.Meta.StreamSeq == 3 {
			got = it.Line
		}
		return true
	})
	if !bytes.Contains(got, fakeHeader(20)) {
		t.Fatal("a line holding a header-shaped pattern was not read whole")
	}

	// Damaged: the search steps over the fake header and finds record 4.
	flipAt(t, filepath.Join(img, tlog(1)), offs[3][0]+12)
	streams, _ := scanDir2(t, img)
	if !eqInts(streams, []int64{1, 2, 4, 5, 6}) {
		t.Fatalf("scan = %v, want [1 2 4 5 6]", streams)
	}
}

// scanDir2 is scanDir without the line-content check (line 3 is not lineFor(3)).
func scanDir2(t *testing.T, dir string) ([]int64, []string) {
	t.Helper()
	var streams []int64
	var reasons []string
	err := OpenReader(dir).Scan(func(it Item) bool {
		if it.Boundary != nil {
			reasons = append(reasons, it.Boundary.Reason)
		} else {
			streams = append(streams, it.Meta.StreamSeq)
		}
		return true
	})
	if err != nil {
		t.Fatal(err)
	}
	return streams, reasons
}

// The one case a CRC cannot separate: a damaged record whose payload embeds a
// whole valid record. The valid one is returned (an extra line); none is lost.
func TestResyncNeverLosesALineWhenAPayloadEmbedsAValidRecord(t *testing.T) {
	embedded, err := encodeLine(nil, &Meta{EventID: "ev-embedded", RunID: "run-a", OwnerID: "owner-1", TrajectoryID: "traj-1", Kind: "tool.result", Seq: 99, StreamSeq: 99}, []byte("embedded"))
	if err != nil {
		t.Fatal(err)
	}
	src := t.TempDir()
	l, _ := openT(t, src, nil)
	mustAppend(t, l, 1)
	mustAppend(t, l, 2)
	if err := l.Append(meta(3), append([]byte("pre-"), embedded...)); err != nil {
		t.Fatal(err)
	}
	for i := 4; i <= 6; i++ {
		mustAppend(t, l, i)
	}
	if err := l.Sync(); err != nil {
		t.Fatal(err)
	}
	offs := offsets(t, src, 1)
	img := copyDir(t, src)
	flipAt(t, filepath.Join(img, tlog(1)), offs[3][0]+12)
	streams, _ := scanDir2(t, img)
	if !eqInts(streams, []int64{1, 2, 99, 4, 5, 6}) {
		t.Fatalf("scan = %v, want [1 2 99 4 5 6]", streams)
	}
}

func TestMultipleDamagedSpansAreEachCounted(t *testing.T) {
	img, offs := syncedSix(t)
	path := filepath.Join(img, tlog(1))
	flipAt(t, path, offs[2][0]+12)
	flipAt(t, path, offs[5][0]+12)
	l, err := Open(Options{Dir: img, ComputerID: "computer-1", SyncInterval: -1, Logf: func(string, ...any) {}})
	if err != nil {
		t.Fatal(err)
	}
	st := l.Stats()
	_ = l.Close()
	if st.DamagedRecords != 2 || st.DamagedBytes != offs[2][1]+offs[5][1] {
		t.Fatalf("damaged_records %d damaged_bytes %d", st.DamagedRecords, st.DamagedBytes)
	}
	streams, _ := scanDir(t, img)
	if !eqInts(streams, []int64{1, 3, 4, 6}) {
		t.Fatalf("lines = %v", streams)
	}
}

// --- Required 2: unknown versions ------------------------------------------

func patchTidxVersion(t *testing.T, path string, v uint32) {
	t.Helper()
	b := mustRead(t, path)
	binary.BigEndian.PutUint32(b[8:12], v)
	binary.BigEndian.PutUint32(b[idxHeaderSize-4:idxHeaderSize], crc32.Checksum(b[:idxHeaderSize-4], crcTable))
	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatal(err)
	}
}

func patchTlogVersion(t *testing.T, path string, v uint32) {
	t.Helper()
	b := mustRead(t, path)
	binary.BigEndian.PutUint32(b[8:12], v)
	cidLen := int(binary.BigEndian.Uint16(b[segHeaderFixed-2 : segHeaderFixed]))
	total := segHeaderFixed + cidLen + 4
	binary.BigEndian.PutUint32(b[total-4:total], crc32.Checksum(b[:total-4], crcTable))
	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatal(err)
	}
}

func patchEnvelopeVersion(t *testing.T, path string, off int64) {
	t.Helper()
	b := mustRead(t, path)
	b[off+recHeaderSize] = envVersion + 1
	n := binary.BigEndian.Uint32(b[off : off+4])
	rec := b[off : off+recHeaderSize+int64(n)]
	binary.BigEndian.PutUint32(rec[4:8], recordCRC(rec))
	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatal(err)
	}
}

// twoSealed builds segments 1 and 2, sealed, holding runs run-a (1..5) and run-b (6..10).
func twoSealed(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	l, _ := openT(t, dir, func(o *Options) { o.SealRecords = 7 })
	for i := 1; i <= 12; i++ {
		m := meta(i)
		if i > 5 {
			m.RunID = "run-b"
		}
		if err := l.Append(m, lineFor(i)); err != nil {
			t.Fatal(err)
		}
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestNewerIndexVersionIsAnErrorForThatSegmentOnly(t *testing.T) {
	dir := twoSealed(t)
	patchTidxVersion(t, filepath.Join(dir, tidx(1)), formatVersion+1)
	r := OpenReader(dir)
	recs, err := r.ByOwnerAfter("owner-1", 0, 100)
	if !errors.Is(err, ErrUnsupportedVersion) {
		t.Fatalf("ByOwnerAfter error = %v, want ErrUnsupportedVersion", err)
	}
	if len(recs) == 0 {
		t.Fatal("the other segments were not read")
	}
	for _, rec := range recs {
		if rec.Segment == 1 {
			t.Fatal("a record of the unsupported segment was served")
		}
	}
	if r.IndexRebuilds() != 0 {
		t.Fatalf("the unsupported index was scanned around (%d rebuilds)", r.IndexRebuilds())
	}
	// The summary path (used to skip segments) refuses too.
	if _, err := OpenReader(dir).ByRun("owner-1", "run-b", 0, 100); !errors.Is(err, ErrUnsupportedVersion) {
		t.Fatalf("ByRun error = %v", err)
	}
	if _, err := OpenReader(dir).Segments(); !errors.Is(err, ErrUnsupportedVersion) {
		t.Fatalf("Segments error = %v", err)
	}
}

func TestNewerSegmentHeaderIsAnErrorForThatSegmentOnly(t *testing.T) {
	dir := twoSealed(t)
	// Segment 1 loses its index (so it reads as unsealed) and gets a newer header.
	if err := os.Remove(filepath.Join(dir, tidx(1))); err != nil {
		t.Fatal(err)
	}
	patchTlogVersion(t, filepath.Join(dir, tlog(1)), formatVersion+1)
	r := OpenReader(dir)
	recs, err := r.ByOwnerAfter("owner-1", 0, 100)
	if !errors.Is(err, ErrUnsupportedVersion) || len(recs) == 0 {
		t.Fatalf("ByOwnerAfter = %d records, %v", len(recs), err)
	}
	// Scan: the same.
	var seen int
	err = OpenReader(dir).Scan(func(Item) bool { seen++; return true })
	if !errors.Is(err, ErrUnsupportedVersion) || seen == 0 {
		t.Fatalf("Scan = %d items, %v", seen, err)
	}
	// Open refuses and does not set the file aside.
	_, err = Open(Options{Dir: dir, ComputerID: "computer-1", SyncInterval: -1})
	if !errors.Is(err, ErrUnsupportedVersion) {
		t.Fatalf("Open = %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, tlog(1)+".bad")); err == nil {
		t.Fatal("an unsupported segment was set aside as .bad")
	}
}

// A header whose magic is ours with another version, older or newer, is not
// foreign: Open fails, nothing is renamed.
func TestOlderAndNewerHeaderVersionsAreNeverQuarantined(t *testing.T) {
	for _, v := range []uint32{0, formatVersion + 1} {
		src := t.TempDir()
		l, _ := openT(t, src, nil)
		mustAppend(t, l, 1)
		if err := l.Sync(); err != nil {
			t.Fatal(err)
		}
		img := copyDir(t, src)
		patchTlogVersion(t, filepath.Join(img, tlog(1)), v)
		if _, err := Open(Options{Dir: img, ComputerID: "computer-1", SyncInterval: -1}); !errors.Is(err, ErrUnsupportedVersion) {
			t.Fatalf("version %d: Open = %v", v, err)
		}
		if _, err := os.Stat(filepath.Join(img, tlog(1)+".bad")); err == nil {
			t.Fatalf("version %d: set aside as .bad", v)
		}
	}
}

func TestNewerRecordEnvelopeIsAnErrorNotADamagedSpan(t *testing.T) {
	src := t.TempDir()
	l, _ := openT(t, src, func(o *Options) { o.SealRecords = 7 })
	for i := 1; i <= 12; i++ {
		mustAppend(t, l, i)
	}
	if err := l.Sync(); err != nil {
		t.Fatal(err)
	}
	offs := offsets(t, src, 2) // the open, unsealed segment
	img := copyDir(t, src)
	var first int64 = 1<<62 - 1
	for s, o := range offs {
		if s < first {
			first = s
		}
		_ = o
	}
	patchEnvelopeVersion(t, filepath.Join(img, tlog(2)), offs[first][0])
	// A reader serves the other segment and reports this one.
	recs, err := OpenReader(img).ByOwnerAfter("owner-1", 0, 100)
	if !errors.Is(err, ErrUnsupportedVersion) || len(recs) == 0 {
		t.Fatalf("ByOwnerAfter = %d records, %v", len(recs), err)
	}
	// Open refuses rather than sealing it as damaged.
	if _, err := Open(Options{Dir: img, ComputerID: "computer-1", SyncInterval: -1}); !errors.Is(err, ErrUnsupportedVersion) {
		t.Fatalf("Open = %v", err)
	}
	if _, err := os.Stat(filepath.Join(img, tidx(2))); err == nil {
		t.Fatal("a segment with an unknown record version was sealed")
	}
}

// --- smaller findings ------------------------------------------------------

// Finding 3: a rebuild for a summary keeps no entries; scans of unsealed
// segments are bounded like loaded indexes.
func TestRebuiltAndScannedIndexesStayWithinTheCacheBound(t *testing.T) {
	dir := t.TempDir()
	l, _ := openT(t, dir, func(o *Options) { o.SealRecords = 4 })
	for i := 1; i <= 40; i++ {
		mustAppend(t, l, i)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	segs, _ := OpenReader(dir).Segments()
	if len(segs) < 8 {
		t.Fatalf("%d segments", len(segs))
	}
	for _, s := range segs {
		flipAt(t, filepath.Join(dir, tidx(s.Segno)), 60)
	}
	r := OpenReader(dir)
	if _, err := r.Segments(); err != nil {
		t.Fatal(err)
	}
	if n := entriesHeld(r); n != 0 {
		t.Fatalf("a summary-only pass kept %d full indexes in memory", n)
	}
	if _, err := r.ByOwnerAfter("owner-1", 0, 1000); err != nil {
		t.Fatal(err)
	}
	if n := entriesHeld(r); n > maxLoadedIndexes {
		t.Fatalf("%d indexes held, bound is %d", n, maxLoadedIndexes)
	}
	// Unsealed scans: remove every index.
	for _, s := range segs {
		_ = os.Remove(filepath.Join(dir, tidx(s.Segno)))
	}
	r2 := OpenReader(dir)
	if _, err := r2.ByOwnerAfter("owner-1", 0, 1000); err != nil {
		t.Fatal(err)
	}
	if n := entriesHeld(r2); n > maxLoadedIndexes {
		t.Fatalf("%d scans held, bound is %d", n, maxLoadedIndexes)
	}
}

func entriesHeld(r *Reader) int {
	r.mu.Lock()
	states := make([]*segState, 0, len(r.states))
	for _, st := range r.states {
		states = append(states, st)
	}
	r.mu.Unlock()
	n := 0
	for _, st := range states {
		st.mu.Lock()
		if st.idx != nil || st.scan != nil {
			n++
		}
		st.mu.Unlock()
	}
	return n
}

// Finding 4: Close waits for a Retain in progress.
func TestCloseWaitsForARetainInProgress(t *testing.T) {
	dir := t.TempDir()
	d := newDisk()
	l, clk := openT(t, dir, func(o *Options) { o.Hooks = d.hooks(); o.SealRecords = 5; o.MaxAge = time.Hour })
	for i := 1; i <= 14; i++ {
		mustAppend(t, l, i)
	}
	clk.Advance(3 * time.Hour)
	release := make(chan struct{})
	entered := make(chan struct{})
	d.set(func(d *disk) { d.dirOnce, d.dirEntered = release, entered })
	retained := make(chan error, 1)
	go func() { _, err := l.Retain(); retained <- err }()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("Retain never reached its directory fsync")
	}
	closed := make(chan error, 1)
	go func() { closed <- l.Close() }()
	select {
	case <-closed:
		t.Fatal("Close finished while a Retain was still unlinking")
	case <-time.After(200 * time.Millisecond):
	}
	close(release)
	if err := <-retained; err != nil {
		t.Fatalf("Retain: %v", err)
	}
	if err := <-closed; err != nil {
		t.Fatalf("Close: %v", err)
	}
	if _, err := l.Retain(); !errors.Is(err, ErrClosed) {
		t.Fatalf("Retain after Close = %v", err)
	}
}

// Finding 5a.
func TestOversizedBoundaryIsRefusedAndHarmless(t *testing.T) {
	dir := t.TempDir()
	l, _ := openT(t, dir, nil)
	mustAppend(t, l, 1)
	if err := l.Sync(); err != nil {
		t.Fatal(err)
	}
	before := tree(t, dir)
	big := string(make([]byte, MaxRecordBytes+1))
	if err := l.AppendBoundary(big, 1); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("AppendBoundary = %v, want ErrTooLarge", err)
	}
	sameTree(t, "oversized boundary", before, tree(t, dir))
	if l.Stats().Poisoned {
		t.Fatal("poisoned by a refused boundary")
	}
	mustAppend(t, l, 2)
}

// Finding 5b: a summary length larger than its file is not allocated.
func TestHugeSummaryLengthIsRefusedBeforeItIsAllocated(t *testing.T) {
	dir := twoSealed(t)
	p := filepath.Join(dir, tidx(1))
	b := mustRead(t, p)
	binary.BigEndian.PutUint32(b[36:40], 200<<20)
	binary.BigEndian.PutUint32(b[idxHeaderSize-4:idxHeaderSize], crc32.Checksum(b[:idxHeaderSize-4], crcTable))
	if err := os.WriteFile(p, b, 0o600); err != nil {
		t.Fatal(err)
	}
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	r := OpenReader(dir)
	for i := 0; i < 3; i++ {
		_, _ = r.ByRun("owner-1", "run-a", 0, 10)
		r = OpenReader(dir)
	}
	runtime.ReadMemStats(&after)
	if grew := after.TotalAlloc - before.TotalAlloc; grew > 64<<20 {
		t.Fatalf("reading a bad index allocated %d MiB", grew>>20)
	}
}

// Finding 5c: an fsync failure that has returned but not yet taken the log's
// mutex stops a seal from installing an index.
func TestSealDoesNotInstallAnIndexOnceAFsyncHasFailed(t *testing.T) {
	dir := t.TempDir()
	var mu sync.Mutex
	calls := 0
	failed := make(chan struct{})
	hold := make(chan struct{})
	var l *Log
	hooks := Hooks{SyncFile: func(f *os.File) error {
		mu.Lock()
		calls++
		n := calls
		mu.Unlock()
		switch {
		case n == 3: // the Sync below (calls 1 and 2 are the segment header and its first boundary path)
			<-hold
			return fmt.Errorf("EIO")
		case n == 4: // the seal's own fsync: let the failing Sync return, and wait until it has flagged the log
			close(hold)
			deadline := time.Now().Add(5 * time.Second)
			for !l.failing.Load() && time.Now().Before(deadline) {
				time.Sleep(time.Millisecond)
			}
			close(failed)
		}
		return f.Sync()
	}}
	l, _ = openT(t, dir, func(o *Options) { o.Hooks = hooks; o.SealRecords = 4 })
	mustAppend(t, l, 1)
	mu.Lock()
	calls = 2 // the next fsync is call 3
	mu.Unlock()
	syncErr := make(chan error, 1)
	go func() { syncErr <- l.Sync() }()
	time.Sleep(50 * time.Millisecond) // the Sync is inside its fsync, parked on hold
	var appendErr error
	for i := 2; i <= 6 && appendErr == nil; i++ {
		appendErr = l.Append(meta(i), lineFor(i)) // the one that rotates runs the seal
	}
	<-failed
	if err := <-syncErr; err == nil {
		t.Fatal("the Sync should have failed")
	}
	if appendErr == nil {
		t.Fatal("the rotating append succeeded although an fsync had failed")
	}
	if _, err := os.Stat(filepath.Join(dir, tidx(1))); err == nil {
		t.Fatal("a seal installed an index after an fsync had failed")
	}
	if !l.Stats().Poisoned {
		t.Fatal("not poisoned")
	}
}
