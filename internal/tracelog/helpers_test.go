package tracelog

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"syscall"
	"testing"
	"time"
)

// fakeClock is the log's injectable clock.
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func newClock() *fakeClock { return &fakeClock{t: time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)} }

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

// disk is the injectable file layer for the crash-shaped tests. It wraps the
// real file calls and keeps, per segment file, the size its last successful
// fsync covered (what a power loss leaves). While failing, every fsync
// returns EIO; with lose set it first cuts the file back to that size, which
// is what a dropped dirty page leaves behind.
type disk struct {
	mu         sync.Mutex
	failSync   bool
	lose       bool
	failWrite  bool
	shortWr    bool
	failDir    bool
	dirOnce    chan struct{} // when non-nil, the first SyncDir waits for it to close, then clears it
	dirEntered chan struct{}
	fast       bool // skip the real fsync (the size is still recorded): for loops of many cases
	synced     map[string]int64
	syncs      int
	block      chan struct{} // when non-nil, SyncFile waits for it to close
	entered    chan struct{}
}

func newDisk() *disk { return &disk{synced: map[string]int64{}} }

func (d *disk) hooks() Hooks {
	return Hooks{WriteAt: d.writeAt, SyncFile: d.syncFile, SyncDir: d.syncDir}
}

func (d *disk) set(f func(*disk)) {
	d.mu.Lock()
	f(d)
	d.mu.Unlock()
}

func (d *disk) writeAt(f *os.File, b []byte, off int64) (int, error) {
	d.mu.Lock()
	fail, short := d.failWrite, d.shortWr
	d.mu.Unlock()
	if fail {
		return 0, syscall.EIO
	}
	if short {
		n, err := f.WriteAt(b[:len(b)/2], off)
		if err != nil {
			return n, err
		}
		return n, nil
	}
	return f.WriteAt(b, off)
}

func (d *disk) syncFile(f *os.File) error {
	d.mu.Lock()
	failing, lose, block, entered, fast := d.failSync, d.lose, d.block, d.entered, d.fast
	synced := d.synced[f.Name()]
	d.mu.Unlock()
	if entered != nil {
		select {
		case entered <- struct{}{}:
		default:
		}
	}
	if block != nil {
		<-block
	}
	if failing {
		if lose {
			_ = f.Truncate(synced)
		}
		return syscall.EIO
	}
	var err error
	if !fast {
		err = f.Sync()
	}
	if info, serr := f.Stat(); err == nil && serr == nil {
		d.mu.Lock()
		d.synced[f.Name()] = info.Size()
		d.syncs++
		d.mu.Unlock()
	}
	return err
}

func (d *disk) syncDir(path string) error {
	d.mu.Lock()
	fail := d.failDir
	once, entered := d.dirOnce, d.dirEntered
	d.dirOnce = nil
	d.mu.Unlock()
	if once != nil {
		if entered != nil {
			close(entered)
		}
		<-once
	}
	if fail {
		return syscall.EIO
	}
	d.mu.Lock()
	fast := d.fast
	d.mu.Unlock()
	if fast {
		return nil
	}
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

func (d *disk) syncedSize(path string) int64 {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.synced[path]
}

// crashImage copies the log directory as a power loss would leave it: every
// .tlog file is cut to what its last fsync covered plus extra bytes of the
// unsynced tail (the kernel may have written some of it); every other file is
// copied whole. The source is not touched, so a live log can stay open.
func crashImage(t *testing.T, dir string, d *disk, extra func(unsynced int64) int64) string {
	t.Helper()
	out := t.TempDir()
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range ents {
		src := filepath.Join(dir, e.Name())
		raw, err := os.ReadFile(src)
		if err != nil {
			t.Fatal(err)
		}
		if filepath.Ext(e.Name()) == ".tlog" && d != nil {
			synced := d.syncedSize(src)
			if synced > int64(len(raw)) {
				synced = int64(len(raw))
			}
			keep := synced
			if extra != nil {
				keep += extra(int64(len(raw)) - synced)
			}
			if keep > int64(len(raw)) {
				keep = int64(len(raw))
			}
			raw = raw[:keep]
		}
		if err := os.WriteFile(filepath.Join(out, e.Name()), raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return out
}

func copyDir(t *testing.T, dir string) string {
	t.Helper()
	return crashImage(t, dir, nil, nil)
}

// tree is every file under dir with its content hash, for "touched nothing".
func tree(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(dir, func(p string, de os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		if de.IsDir() {
			out[rel] = "dir"
			return nil
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		out[rel] = fmt.Sprintf("%x", sha256.Sum256(raw))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func sameTree(t *testing.T, what string, before, after map[string]string) {
	t.Helper()
	if len(before) != len(after) {
		t.Fatalf("%s: file set changed: before %v after %v", what, keys(before), keys(after))
	}
	for k, v := range before {
		if after[k] != v {
			t.Fatalf("%s: %s changed", what, k)
		}
	}
}

func keys(m map[string]string) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func meta(i int) Meta {
	return Meta{
		EventID: fmt.Sprintf("ev-%06d", i), RunID: "run-a", OwnerID: "owner-1", TrajectoryID: "traj-1",
		Kind: "tool.result", Seq: int64(i), StreamSeq: int64(i),
		Time: time.Date(2026, 10, 22, 12, 0, 0, i, time.UTC), AfterTapeSeq: int64(i / 2), BootID: "boot-1",
	}
}

func lineFor(i int) []byte {
	return []byte(fmt.Sprintf(`{"event_id":"ev-%06d","payload":{"n":%d}}`, i, i))
}

// openT opens a log with test defaults: a fake clock, no timer, small caps
// only where the test sets them.
func openT(t *testing.T, dir string, mut func(*Options)) (*Log, *fakeClock) {
	t.Helper()
	clk := newClock()
	opts := Options{Dir: dir, ComputerID: "computer-1", SyncInterval: -1, BootID: "boot-1", Now: clk.Now}
	if mut != nil {
		mut(&opts)
	}
	l, err := Open(opts)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = l.Close() })
	return l, clk
}

func mustAppend(t *testing.T, l *Log, i int) {
	t.Helper()
	if err := l.Append(meta(i), lineFor(i)); err != nil {
		t.Fatalf("Append(%d): %v", i, err)
	}
}

// lineStreams returns the stream_seq of every line a static reader finds, in
// scan order, and the boundary reasons.
func scanDir(t *testing.T, dir string) (streams []int64, reasons []string) {
	t.Helper()
	r := OpenReader(dir)
	err := r.Scan(func(it Item) bool {
		if it.Boundary != nil {
			reasons = append(reasons, it.Boundary.Reason)
		} else {
			streams = append(streams, it.Meta.StreamSeq)
			if !bytes.Equal(it.Line, lineFor(int(it.Meta.StreamSeq))) {
				t.Fatalf("line %d came back as %q", it.Meta.StreamSeq, it.Line)
			}
		}
		return true
	})
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	return streams, reasons
}

func seq(from, to int) []int64 {
	var out []int64
	for i := from; i <= to; i++ {
		out = append(out, int64(i))
	}
	return out
}

func eqInts(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func fileNames(t *testing.T, dir string) []string {
	t.Helper()
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range ents {
		out = append(out, e.Name())
	}
	return out
}

func mustRead(t *testing.T, p string) []byte {
	t.Helper()
	f, err := os.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	b, err := io.ReadAll(f)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func tlog(n uint64) string { return fmt.Sprintf("seg-%08d.tlog", n) }
func tidx(n uint64) string { return fmt.Sprintf("seg-%08d.tidx", n) }

// fastDisk is a disk whose fsyncs are no-ops, for tests that open and reopen
// a log hundreds of times. It still records synced sizes.
func fastDisk() *disk {
	d := newDisk()
	d.fast = true
	return d
}
