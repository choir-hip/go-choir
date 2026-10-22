package tracelog

import (
	"errors"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
	"time"
)

// openWith opens a log over the injectable disk.
func openWith(t *testing.T, dir string, d *disk, mut func(*Options)) *Log {
	t.Helper()
	l, _ := openT(t, dir, func(o *Options) {
		o.Hooks = d.hooks()
		if mut != nil {
			mut(o)
		}
	})
	return l
}

// F4. A torn tail is cut at every byte of the last record: the whole earlier
// records read, the partial one does not, nothing is truncated, and new
// writes go to a new segment.
func TestTornTailAtEveryByteOfTheLastRecord(t *testing.T) {
	src := t.TempDir()
	d := newDisk()
	l := openWith(t, src, d, nil)
	for i := 1; i <= 4; i++ {
		mustAppend(t, l, i)
	}
	if err := l.Sync(); err != nil {
		t.Fatal(err)
	}
	full := mustRead(t, filepath.Join(src, tlog(1)))
	// Where does the last record start? Open a static reader on the live dir.
	var lastOff int64
	_ = OpenReader(src).Scan(func(it Item) bool {
		if it.Boundary == nil && it.Meta.StreamSeq == 4 {
			lastOff = it.Offset
		}
		return true
	})
	if lastOff == 0 {
		t.Fatal("could not find the last record")
	}
	for cut := lastOff + 1; cut < int64(len(full)); cut++ {
		img := copyDir(t, src)
		if err := os.Truncate(filepath.Join(img, tlog(1)), cut); err != nil {
			t.Fatal(err)
		}
		l2, err := Open(Options{Dir: img, ComputerID: "computer-1", SyncInterval: -1, BootID: "boot-2"})
		if err != nil {
			t.Fatalf("cut %d: Open: %v", cut, err)
		}
		if st := l2.Stats(); st.TornTailsCut != 1 || st.DamagedTails != 0 {
			t.Fatalf("cut %d: stats %+v", cut, st)
		}
		if err := l2.Append(meta(5), lineFor(5)); err != nil {
			t.Fatalf("cut %d: Append: %v", cut, err)
		}
		if err := l2.Close(); err != nil {
			t.Fatal(err)
		}
		streams, reasons := scanDir(t, img)
		if !eqInts(streams, []int64{1, 2, 3, 5}) {
			t.Fatalf("cut %d: lines = %v", cut, streams)
		}
		if len(reasons) != 2 || reasons[0] != "open" || reasons[1] != "crash_recovered" {
			t.Fatalf("cut %d: boundaries = %v", cut, reasons)
		}
		// Nothing was truncated: the old segment is exactly the bytes the crash left.
		if got := mustRead(t, filepath.Join(img, tlog(1))); int64(len(got)) != cut {
			t.Fatalf("cut %d: segment 1 is now %d bytes", cut, len(got))
		}
		if _, err := os.Stat(filepath.Join(img, tidx(1))); err != nil {
			t.Fatalf("cut %d: the recovered segment was not sealed", cut)
		}
		if _, err := os.Stat(filepath.Join(img, tlog(2))); err != nil {
			t.Fatalf("cut %d: new writes did not go to a new segment", cut)
		}
	}
}

func TestZeroFilledAndGarbageTailsAreCut(t *testing.T) {
	for name, tail := range map[string][]byte{
		"zeros":   make([]byte, 4096),
		"garbage": []byte("this is not a record, it is just bytes that a crash left behind"),
	} {
		src := t.TempDir()
		l, _ := openT(t, src, nil)
		for i := 1; i <= 3; i++ {
			mustAppend(t, l, i)
		}
		if err := l.Sync(); err != nil {
			t.Fatal(err)
		}
		img := copyDir(t, src)
		f, err := os.OpenFile(filepath.Join(img, tlog(1)), os.O_WRONLY|os.O_APPEND, 0)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = f.Write(tail)
		f.Close()
		l2, err := Open(Options{Dir: img, ComputerID: "computer-1", SyncInterval: -1})
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		_ = l2.Close()
		streams, _ := scanDir(t, img)
		if !eqInts(streams, seq(1, 3)) {
			t.Fatalf("%s: lines = %v", name, streams)
		}
	}
}

// A bad record in the middle, followed by valid ones: the segment is cut at
// the first bad record and counted, never refused (design 9.2 item 4).
func TestDamageBeforeValidRecordsCutsAtTheFirstBadRecord(t *testing.T) {
	src := t.TempDir()
	l, _ := openT(t, src, nil)
	for i := 1; i <= 6; i++ {
		mustAppend(t, l, i)
	}
	if err := l.Sync(); err != nil {
		t.Fatal(err)
	}
	var off3 int64
	_ = OpenReader(src).Scan(func(it Item) bool {
		if it.Boundary == nil && it.Meta.StreamSeq == 3 {
			off3 = it.Offset
		}
		return true
	})
	img := copyDir(t, src)
	p := filepath.Join(img, tlog(1))
	raw := mustRead(t, p)
	raw[off3+12] ^= 0xff // a body byte of line 3
	if err := os.WriteFile(p, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	l2, err := Open(Options{Dir: img, ComputerID: "computer-1", SyncInterval: -1})
	if err != nil {
		t.Fatalf("Open refused damaged segment: %v", err)
	}
	if st := l2.Stats(); st.DamagedTails != 1 {
		t.Fatalf("DamagedTails = %d", st.DamagedTails)
	}
	mustAppend(t, l2, 7)
	_ = l2.Close()
	streams, _ := scanDir(t, img)
	if !eqInts(streams, []int64{1, 2, 7}) {
		t.Fatalf("lines = %v, want [1 2 7]", streams)
	}
}

func TestTornSegmentHeaderIsQuarantinedNotReused(t *testing.T) {
	src := t.TempDir()
	l, _ := openT(t, src, nil)
	mustAppend(t, l, 1)
	if err := l.Sync(); err != nil {
		t.Fatal(err)
	}
	full := mustRead(t, filepath.Join(src, tlog(1)))
	for cut := 0; cut < 40 && cut < len(full); cut++ {
		img := copyDir(t, src)
		if err := os.Truncate(filepath.Join(img, tlog(1)), int64(cut)); err != nil {
			t.Fatal(err)
		}
		l2, err := Open(Options{Dir: img, ComputerID: "computer-1", SyncInterval: -1})
		if err != nil {
			t.Fatalf("cut %d: %v", cut, err)
		}
		mustAppend(t, l2, 2)
		_ = l2.Close()
		if _, err := os.Stat(filepath.Join(img, tlog(2))); err != nil {
			t.Fatalf("cut %d: the next segment must be number 2 (a number is never reused)", cut)
		}
		streams, _ := scanDir(t, img)
		if !eqInts(streams, []int64{2}) {
			t.Fatalf("cut %d: lines = %v", cut, streams)
		}
	}
}

func TestHeaderThatIsNotOursIsQuarantined(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, tlog(3)), []byte("NOTATRACE-LOG-FILE-AT-ALL-0123456789abcdef-0123456789"), 0o600); err != nil {
		t.Fatal(err)
	}
	l, err := Open(Options{Dir: dir, ComputerID: "c", SyncInterval: -1})
	if err != nil {
		t.Fatalf("Open must not stay dead over a bad file: %v", err)
	}
	defer l.Close()
	if st := l.Stats(); st.Quarantined != 1 {
		t.Fatalf("Quarantined = %d", st.Quarantined)
	}
	if err := l.Append(meta(1), lineFor(1)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, tlog(3)+".bad")); err != nil {
		t.Fatal("the bad file was not set aside")
	}
	if _, err := os.Stat(filepath.Join(dir, tlog(4))); err != nil {
		t.Fatal("new segment must be numbered after the quarantined one")
	}
}

// F5. A failed fsync poisons; nothing is written, sealed or truncated again.
func TestFailedFsyncPoisonsAndTheLogTouchesNoFileAgain(t *testing.T) {
	dir := t.TempDir()
	d := newDisk()
	l := openWith(t, dir, d, nil)
	for i := 1; i <= 3; i++ {
		mustAppend(t, l, i)
	}
	if err := l.Sync(); err != nil {
		t.Fatal(err)
	}
	mustAppend(t, l, 4)
	mustAppend(t, l, 5)
	d.set(func(d *disk) { d.failSync = true })
	err := l.Sync()
	if err == nil || !errors.Is(err, syscall.EIO) {
		t.Fatalf("Sync with a failing disk = %v", err)
	}
	st := l.Stats()
	if !st.Poisoned || st.UnsyncedLinesAtPoison != 2 || st.SyncErrors != 1 {
		t.Fatalf("stats after the failure: %+v", st)
	}
	before := tree(t, dir)
	d.set(func(d *disk) { d.failSync = false }) // the disk "recovers": too late
	if err := l.Append(meta(6), lineFor(6)); !errors.Is(err, ErrPoisoned) {
		t.Fatalf("Append on a poisoned log = %v", err)
	}
	if err := l.Sync(); !errors.Is(err, ErrPoisoned) {
		t.Fatalf("Sync on a poisoned log = %v", err)
	}
	if err := l.AppendBoundary("restore", 1); !errors.Is(err, ErrPoisoned) {
		t.Fatalf("AppendBoundary on a poisoned log = %v", err)
	}
	if _, err := l.Maintain(); !errors.Is(err, ErrPoisoned) {
		t.Fatalf("Maintain on a poisoned log = %v", err)
	}
	if _, err := l.Retain(); !errors.Is(err, ErrPoisoned) {
		t.Fatalf("Retain on a poisoned log = %v", err)
	}
	if err := l.Close(); err != nil && !errors.Is(err, ErrPoisoned) {
		t.Fatalf("Close = %v", err)
	}
	sameTree(t, "poisoned log", before, tree(t, dir))
}

// The kernel may drop the dirty pages after a failed fsync. The next process
// reads back what is really there; every line a successful Sync covered is
// in it, exactly once.
func TestFailedFsyncThenRestartKeepsEverySyncedLine(t *testing.T) {
	dir := t.TempDir()
	d := newDisk()
	l := openWith(t, dir, d, nil)
	for i := 1; i <= 3; i++ {
		mustAppend(t, l, i)
	}
	if err := l.Sync(); err != nil {
		t.Fatal(err)
	}
	mustAppend(t, l, 4)
	d.set(func(d *disk) { d.failSync = true; d.lose = true })
	if err := l.Sync(); err == nil {
		t.Fatal("fsync should have failed")
	}
	_ = l.Close()
	l2, err := Open(Options{Dir: dir, ComputerID: "computer-1", SyncInterval: -1})
	if err != nil {
		t.Fatal(err)
	}
	// The poisoned process's tail is sealed by this one, which writes its own segment.
	mustAppend(t, l2, 5)
	if err := l2.Sync(); err != nil {
		t.Fatal(err)
	}
	_ = l2.Close()
	streams, _ := scanDir(t, dir)
	if !eqInts(streams, []int64{1, 2, 3, 5}) {
		t.Fatalf("lines = %v, want [1 2 3 5]", streams)
	}
}

func TestFailedAndShortWritesPoison(t *testing.T) {
	for name, set := range map[string]func(*disk){
		"failed": func(d *disk) { d.failWrite = true },
		"short":  func(d *disk) { d.shortWr = true },
	} {
		dir := t.TempDir()
		d := newDisk()
		l := openWith(t, dir, d, nil)
		mustAppend(t, l, 1)
		if err := l.Sync(); err != nil {
			t.Fatal(err)
		}
		d.set(set)
		if err := l.Append(meta(2), lineFor(2)); err == nil {
			t.Fatalf("%s: Append succeeded on a failing write", name)
		}
		if !l.Stats().Poisoned {
			t.Fatalf("%s: not poisoned", name)
		}
		d.set(func(d *disk) { d.failWrite, d.shortWr = false, false })
		before := tree(t, dir)
		if err := l.Append(meta(3), lineFor(3)); !errors.Is(err, ErrPoisoned) {
			t.Fatalf("%s: Append = %v", name, err)
		}
		_ = l.Close()
		sameTree(t, name, before, tree(t, dir))
		// The next process cuts the partial record; the earlier line survives.
		l2, err := Open(Options{Dir: dir, ComputerID: "computer-1", SyncInterval: -1})
		if err != nil {
			t.Fatal(err)
		}
		_ = l2.Close()
		streams, _ := scanDir(t, dir)
		if !eqInts(streams, []int64{1}) {
			t.Fatalf("%s: lines = %v", name, streams)
		}
	}
}

func TestFailedDirectoryFsyncPoisons(t *testing.T) {
	dir := t.TempDir()
	d := newDisk()
	d.failDir = true
	l := openWith(t, dir, d, nil)
	if err := l.Append(meta(1), lineFor(1)); err == nil {
		t.Fatal("Append acknowledged a segment whose directory entry may not be durable")
	}
	if !l.Stats().Poisoned {
		t.Fatal("a failed directory fsync must poison")
	}
}

// A failure while sealing for rotation poisons, and the line that triggered
// the rotation is not acknowledged (the caller sends it to the tape).
func TestFailedSealPoisonsAndRefusesTheTriggeringLine(t *testing.T) {
	for name, set := range map[string]func(*disk){
		"fsync of the segment": func(d *disk) { d.failSync = true },
		"fsync of a directory": func(d *disk) { d.failDir = true },
	} {
		dir := t.TempDir()
		d := newDisk()
		l := openWith(t, dir, d, func(o *Options) { o.SealRecords = 4 })
		var failedAt int
		for i := 1; i <= 6; i++ {
			if i == 4 {
				d.set(set)
			}
			if err := l.Append(meta(i), lineFor(i)); err != nil {
				failedAt = i
				break
			}
		}
		if failedAt == 0 {
			t.Fatalf("%s: rotation never failed", name)
		}
		if !l.Stats().Poisoned {
			t.Fatalf("%s: not poisoned", name)
		}
		before := tree(t, dir)
		_ = l.Append(meta(9), lineFor(9))
		_ = l.Close()
		sameTree(t, name, before, tree(t, dir))
	}
}

func TestTimerFsyncFailurePoisons(t *testing.T) {
	dir := t.TempDir()
	d := newDisk()
	l := openWith(t, dir, d, func(o *Options) { o.SyncInterval = 5 * time.Millisecond })
	mustAppend(t, l, 1)
	deadline := time.Now().Add(2 * time.Second)
	for l.Stats().Dirty && time.Now().Before(deadline) {
		time.Sleep(2 * time.Millisecond)
	}
	d.set(func(d *disk) { d.failSync = true })
	mustAppend(t, l, 2)
	for !l.Stats().Poisoned && time.Now().Before(deadline) {
		time.Sleep(2 * time.Millisecond)
	}
	if !l.Stats().Poisoned {
		t.Fatal("the timer's fsync failure did not poison the log")
	}
	if err := l.Append(meta(3), lineFor(3)); !errors.Is(err, ErrPoisoned) {
		t.Fatalf("Append = %v", err)
	}
}

// Round 3 of the host writer: a poisoned writer's memory outlived its lock
// and it later truncated another writer's acknowledged record. Here the
// poisoned log holds the directory lock until it closes, and after that it
// can change nothing.
func TestPoisonedWriterCannotDamageTheNextWriter(t *testing.T) {
	dir := t.TempDir()
	d := newDisk()
	a := openWith(t, dir, d, nil)
	mustAppend(t, a, 1)
	mustAppend(t, a, 2)
	d.set(func(d *disk) { d.failSync = true; d.lose = true })
	_ = a.Sync()
	d.set(func(d *disk) { d.failSync = false })
	// B cannot take the directory while A is alive, poisoned or not.
	if _, err := Open(Options{Dir: dir, ComputerID: "computer-1", SyncInterval: -1}); !errors.Is(err, ErrLocked) {
		t.Fatalf("Open beside a poisoned writer = %v", err)
	}
	_ = a.Close()
	b, err := Open(Options{Dir: dir, ComputerID: "computer-1", SyncInterval: -1})
	if err != nil {
		t.Fatal(err)
	}
	for i := 3; i <= 5; i++ {
		mustAppend(t, b, i)
	}
	if err := b.Sync(); err != nil {
		t.Fatal(err)
	}
	before := tree(t, dir)
	// A, still holding its state, tries everything.
	_ = a.Append(meta(6), lineFor(6))
	_ = a.Sync()
	_, _ = a.Maintain()
	_, _ = a.Retain()
	_ = a.Close()
	sameTree(t, "a stale poisoned writer", before, tree(t, dir))
	_ = b.Close()
	streams, _ := scanDir(t, dir)
	if !eqInts(streams, []int64{3, 4, 5}) {
		t.Fatalf("lines = %v, want [3 4 5]", streams)
	}
}

// No offset is reused: a sealed segment is never modified by a later open,
// recovery or close.
func TestSealedSegmentsAreNeverModified(t *testing.T) {
	dir := t.TempDir()
	d := newDisk()
	l := openWith(t, dir, d, func(o *Options) { o.SealRecords = 5 })
	for i := 1; i <= 12; i++ {
		mustAppend(t, l, i)
	}
	if err := l.Sync(); err != nil {
		t.Fatal(err)
	}
	img := crashImage(t, dir, d, nil)
	sealed := map[string]string{}
	for k, v := range tree(t, img) {
		if filepath.Ext(k) == ".tidx" || k == tlog(1) || k == tlog(2) {
			sealed[k] = v
		}
	}
	if len(sealed) < 4 {
		t.Fatalf("expected sealed segments, got %v", keys(sealed))
	}
	for round := 0; round < 3; round++ {
		l2, err := Open(Options{Dir: img, ComputerID: "computer-1", SyncInterval: -1, SealRecords: 5})
		if err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 7; i++ {
			mustAppend(t, l2, 100+round*10+i)
		}
		_ = l2.Close()
		now := tree(t, img)
		for k, v := range sealed {
			if now[k] != v {
				t.Fatalf("round %d: sealed file %s changed", round, k)
			}
		}
	}
}

// F8, as a property: after a power loss at a random point, the survivors are
// a prefix of what was appended, in order, with no repeats, and every line a
// successful Sync covered is among them.
func TestPowerLossKeepsAPrefixThatIncludesEverySyncedLine(t *testing.T) {
	for seed := int64(1); seed <= 60; seed++ {
		rng := rand.New(rand.NewSource(seed))
		dir := t.TempDir()
		d := newDisk()
		l, _ := openT(t, dir, func(o *Options) { o.Hooks = d.hooks(); o.SealRecords = 3 + rng.Intn(12) })
		n := 5 + rng.Intn(60)
		lastSynced := 0
		for i := 1; i <= n; i++ {
			mustAppend(t, l, i)
			if rng.Intn(6) == 0 {
				if err := l.Sync(); err != nil {
					t.Fatal(err)
				}
				lastSynced = i
			}
		}
		img := crashImage(t, dir, d, func(unsynced int64) int64 { return rng.Int63n(unsynced + 1) })
		l2, err := Open(Options{Dir: img, ComputerID: "computer-1", SyncInterval: -1})
		if err != nil {
			t.Fatalf("seed %d: Open after power loss: %v", seed, err)
		}
		_ = l2.Close()
		streams, _ := scanDir(t, img)
		for i, s := range streams {
			if s != int64(i+1) {
				t.Fatalf("seed %d: survivors are not a prefix: %v", seed, streams)
			}
		}
		if len(streams) < lastSynced {
			t.Fatalf("seed %d: lost synced lines: %d survive, %d were synced", seed, len(streams), lastSynced)
		}
		_ = l.Close()
	}
}

// Appenders racing an fsync failure: once a caller has seen the log poisoned,
// no Append succeeds and the open segment does not grow.
func TestConcurrentAppendsAndAFailingFsync(t *testing.T) {
	dir := t.TempDir()
	d := newDisk()
	l := openWith(t, dir, d, nil)
	var wg sync.WaitGroup
	stop := make(chan struct{})
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; ; i++ {
				select {
				case <-stop:
					return
				default:
				}
				n := w*100000 + i + 1
				if err := l.Append(meta(n), lineFor(n)); err != nil {
					if !errors.Is(err, ErrPoisoned) {
						t.Errorf("Append: %v", err)
					}
					return
				}
			}
		}(w)
	}
	time.Sleep(20 * time.Millisecond)
	d.set(func(d *disk) { d.failSync = true })
	if err := l.Sync(); err == nil {
		t.Error("fsync should have failed")
	}
	wg.Wait()
	close(stop)
	p := filepath.Join(dir, tlog(1))
	a, _ := os.Stat(p)
	time.Sleep(20 * time.Millisecond)
	b, _ := os.Stat(p)
	if a.Size() != b.Size() {
		t.Fatal("the open segment grew after the log was poisoned")
	}
	if err := l.Append(meta(1), lineFor(1)); !errors.Is(err, ErrPoisoned) {
		t.Fatalf("Append after poison = %v", err)
	}
}
