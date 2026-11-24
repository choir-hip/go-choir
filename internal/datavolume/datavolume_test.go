package datavolume

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// What can go wrong in a data volume report, each covered below:
//   - the reserved blocks hidden: free and available reported as one number;
//   - st_blocks counted as bytes, or a sparse file's apparent size reported
//     as the space it holds;
//   - a hard-linked inode counted once per name, doubling a store;
//   - a symlink followed out of the volume (a link to a huge tree, or a loop);
//   - a mount point under the root walked into, adding another filesystem;
//   - file contents read (a file the walker cannot open must still count);
//   - a file that vanishes between the directory read and the stat aborting
//     the walk;
//   - a walk that overruns its entry or time bound instead of stopping and
//     saying truncated, or one huge directory leaving the others unwalked;
//   - two requests walking at once, each paying the full cost;
//   - an arbitrary user file name (under files/) in the report;
//   - a second-level directory the report was asked to name missing, or an
//     unbounded list of children;
//   - a top-level name that is not a plain name echoed back.

func write(t *testing.T, path string, size int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, make([]byte, size), 0o644); err != nil {
		t.Fatal(err)
	}
}

func topByName(t *testing.T, w *Walk, name string) Entry {
	t.Helper()
	for _, e := range w.Top {
		if e.Name == name {
			return e
		}
	}
	t.Fatalf("no top-level entry %q in %+v", name, w.Top)
	return Entry{}
}

func groupByPath(t *testing.T, w *Walk, path string) Group {
	t.Helper()
	for _, g := range w.Detail {
		if g.Path == path {
			return g
		}
	}
	t.Fatalf("no detail group %q in %+v", path, w.Detail)
	return Group{}
}

func child(t *testing.T, g Group, name string) Entry {
	t.Helper()
	for _, c := range g.Children {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("group %s has no child %q: %+v", g.Path, name, g.Children)
	return Entry{}
}

func scan(t *testing.T, root string, lim Limits) *Walk {
	t.Helper()
	var s Scanner
	rep := s.Read(context.Background(), root, lim, true)
	if rep.Walk == nil {
		t.Fatalf("no walk: %+v", rep)
	}
	return rep.Walk
}

func TestStatfsShowsReservedBlocks(t *testing.T) {
	rep := (&Scanner{}).Read(context.Background(), t.TempDir(), DefaultLimits, false)
	if rep.Statfs == nil || rep.Statfs.TotalBytes == 0 {
		t.Fatalf("statfs = %+v", rep.Statfs)
	}
	s := rep.Statfs
	if s.FreeBytes < s.AvailBytes || s.ReservedBytes != s.FreeBytes-s.AvailBytes {
		t.Fatalf("free=%d avail=%d reserved=%d", s.FreeBytes, s.AvailBytes, s.ReservedBytes)
	}
	if s.FreeBytes > s.TotalBytes {
		t.Fatalf("free %d > total %d", s.FreeBytes, s.TotalBytes)
	}
}

func TestWalkCountsAllocatedApparentAndFiles(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "go", "a.bin"), 10000)
	write(t, filepath.Join(root, "go", "sub", "b.bin"), 5000)
	write(t, filepath.Join(root, "state.db"), 3)
	// A sparse file: a large apparent size, almost nothing allocated.
	sparse := filepath.Join(root, "go", "sparse")
	f, err := os.Create(sparse)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(1 << 30); err != nil {
		t.Fatal(err)
	}
	f.Close()
	w := scan(t, root, DefaultLimits)
	g := topByName(t, w, "go")
	if g.Files != 3 {
		t.Fatalf("files = %d, want 3", g.Files)
	}
	if g.ApparentBytes < (1<<30)+15000 {
		t.Fatalf("apparent = %d, want the sparse size counted", g.ApparentBytes)
	}
	var want uint64
	filepath.Walk(filepath.Join(root, "go"), func(p string, info os.FileInfo, err error) error {
		if err == nil {
			want += uint64(info.Sys().(*syscall.Stat_t).Blocks) * 512
		}
		return nil
	})
	if g.AllocatedBytes != want {
		t.Fatalf("allocated = %d, want exactly st_blocks*512 summed = %d", g.AllocatedBytes, want)
	}
	if g.AllocatedBytes%512 != 0 {
		t.Fatalf("allocated %d is not a multiple of the 512-byte block unit", g.AllocatedBytes)
	}
	if topByName(t, w, "state.db").Files != 1 {
		t.Fatal("a top-level file is one file")
	}
	if w.Truncated {
		t.Fatal("a small tree is truncated")
	}
}

func TestHardLinkedInodeCountsOnce(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "choir-updater", "store", "x", "lib"), 40000)
	for i, name := range []string{"a", "b", "c"} {
		if err := os.MkdirAll(filepath.Join(root, "choir-updater", "releases", name), 0o755); err != nil {
			t.Fatal(err)
		}
		_ = i
		if err := os.Link(filepath.Join(root, "choir-updater", "store", "x", "lib"), filepath.Join(root, "choir-updater", "releases", name, "lib")); err != nil {
			t.Fatal(err)
		}
	}
	w := scan(t, root, DefaultLimits)
	cu := topByName(t, w, "choir-updater")
	if cu.Files != 1 {
		t.Fatalf("files = %d, want the one inode counted once", cu.Files)
	}
	if cu.ApparentBytes >= 80000 {
		t.Fatalf("apparent = %d, the 40000 byte inode was counted more than once", cu.ApparentBytes)
	}
}

func TestSymlinksAreNotFollowed(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	write(t, filepath.Join(outside, "huge"), 1<<20)
	write(t, filepath.Join(root, "files", "keep"), 10)
	if err := os.Symlink(outside, filepath.Join(root, "files", "escape")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(root, filepath.Join(root, "files", "loop")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "linktop")); err != nil {
		t.Fatal(err)
	}
	w := scan(t, root, DefaultLimits)
	if got := topByName(t, w, "files"); got.ApparentBytes >= 1<<20 || got.Files != 3 {
		t.Fatalf("files = %+v: a symlink was followed or not counted as an entry", got)
	}
	if got := topByName(t, w, "linktop"); got.ApparentBytes >= 1<<20 || got.Files != 1 {
		t.Fatalf("top symlink = %+v", got)
	}
}

func TestUnreadableFileStillCountsAndNothingIsRead(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "go", "secret")
	write(t, p, 4096)
	if err := os.Chmod(p, 0); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(p, 0o644)
	w := scan(t, root, DefaultLimits)
	if g := topByName(t, w, "go"); g.Files != 1 || g.Errors != 0 {
		t.Fatalf("go = %+v: a file with no read permission is metadata only", g)
	}
}

func TestVanishingFilesAreTolerated(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 50; i++ {
		write(t, filepath.Join(root, "capsule-state", "boot-x", string(rune('a'+i%26))+string(rune('a'+i/26))), 100)
	}
	removed := false
	w := scanWith(t, root, DefaultLimits, func(path string) {
		// The first time the walker lists the boot directory, delete half
		// of its files before it stats them.
		if removed || !strings.HasSuffix(path, "boot-x") {
			return
		}
		removed = true
		es, _ := os.ReadDir(path)
		for i, e := range es {
			if i%2 == 0 {
				os.Remove(filepath.Join(path, e.Name()))
			}
		}
	})
	if !removed {
		t.Fatal("the hook never ran")
	}
	c := topByName(t, w, "capsule-state")
	if c.Files != 25 || c.Errors != 0 {
		t.Fatalf("capsule-state = %+v, want 25 files and no errors", c)
	}
}

func TestEntryBoundTruncatesAndSaysSo(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 200; i++ {
		write(t, filepath.Join(root, "aaa-huge", "00", string(rune('a'+i%26))+string(rune('a'+i/26))), 10)
	}
	write(t, filepath.Join(root, "files", "x"), 10)
	w := scan(t, root, Limits{Budget: time.Minute, MaxEntries: 40})
	if !w.Truncated {
		t.Fatal("walk is not marked truncated")
	}
	if !topByName(t, w, "aaa-huge").Truncated {
		t.Fatal("the cut entry is not marked truncated")
	}
	if got := topByName(t, w, "files"); got.Truncated || got.Files != 1 {
		t.Fatalf("one huge directory starved the next: %+v", got)
	}
	if w.EntriesExamined > 60 {
		t.Fatalf("examined %d entries against a bound of 40", w.EntriesExamined)
	}
}

func TestDeadlineTruncates(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 600; i++ {
		write(t, filepath.Join(root, "go", string(rune('a'+i%26)), string(rune('a'+i/26))), 1)
	}
	w := scan(t, root, Limits{Budget: time.Nanosecond, MaxEntries: 1 << 30})
	if !w.Truncated || !topByName(t, w, "go").Truncated {
		t.Fatalf("a spent deadline did not truncate: %+v", w)
	}
}

func TestSecondRequestGetsTheLastResultWhileOneIsRunning(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "go", "a"), 100)
	var s Scanner
	first := s.Read(context.Background(), root, DefaultLimits, true)
	if first.Busy || first.Walk == nil {
		t.Fatalf("first = %+v", first)
	}
	entered := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	s.testHook = func(string) {
		once.Do(func() { close(entered); <-release })
	}
	done := make(chan Report)
	go func() { done <- s.Read(context.Background(), root, DefaultLimits, true) }()
	<-entered
	second := s.Read(context.Background(), root, DefaultLimits, true)
	if !second.Busy {
		t.Fatalf("a concurrent request ran its own walk: %+v", second)
	}
	if second.Walk == nil || !second.Walk.ReadAt.Equal(first.Walk.ReadAt) {
		t.Fatalf("a concurrent request did not get the last result: %+v", second.Walk)
	}
	if second.Statfs == nil {
		t.Fatal("statfs is missing from a busy answer")
	}
	close(release)
	third := <-done
	if third.Busy || third.Walk == nil || !third.Walk.ReadAt.After(first.Walk.ReadAt) {
		t.Fatalf("the running walk did not finish and replace the result: %+v", third)
	}
}

func TestPlainReadDoesNotWalkAndShowsTheLastWalk(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "go", "a"), 100)
	var s Scanner
	if rep := s.Read(context.Background(), root, DefaultLimits, false); rep.Walk != nil {
		t.Fatalf("a plain read walked: %+v", rep.Walk)
	}
	s.Read(context.Background(), root, DefaultLimits, true)
	rep := s.Read(context.Background(), root, DefaultLimits, false)
	if rep.Walk == nil || rep.Walk.ReadAt.IsZero() {
		t.Fatalf("a plain read hides the last walk: %+v", rep)
	}
	// A different root is not answered with this root's walk.
	if rep := s.Read(context.Background(), t.TempDir(), DefaultLimits, false); rep.Walk != nil {
		t.Fatal("another root got this root's walk")
	}
}

func TestSecondLevelNamesAreAllowlistedAndUserNamesNeverAppear(t *testing.T) {
	root := t.TempDir()
	digestA := strings.Repeat("a", 64)
	write(t, filepath.Join(root, "choir-updater", "releases", digestA, "bin"), 1000)
	write(t, filepath.Join(root, "choir-updater", "releases", ".stage-123", "x"), 500)
	write(t, filepath.Join(root, "choir-updater", "releases", "my private notes", "x"), 500)
	write(t, filepath.Join(root, "choir-updater", "store", "abc-glibc", "lib"), 2000)
	write(t, filepath.Join(root, "choir-updater", "gc-roots", "r"), 10)
	write(t, filepath.Join(root, "files", "Source", "platform", ".git", "pack"), 3000)
	write(t, filepath.Join(root, "files", "Source", "secret-project", "a"), 100)
	write(t, filepath.Join(root, "files", "taxes-2025.pdf"), 100)
	write(t, filepath.Join(root, "files", "Documents", "diary.txt"), 100)
	write(t, filepath.Join(root, "files", ".hidden-memoir", "a"), 100)
	write(t, filepath.Join(root, "files", "Source", ".hidden-memoir", "a"), 100)
	write(t, filepath.Join(root, "capsule-artifacts", "subjects", strings.Repeat("b", 64), "workspace", "x"), 100)
	write(t, filepath.Join(root, "capsule-state", "boot-abc", "cap1", "x"), 100)
	write(t, filepath.Join(root, "go", "pkg", "mod", "cache", "download", "z"), 700)
	write(t, filepath.Join(root, "go", "pkg", "mod", "github.com", "someone", "repo", "f"), 300)
	write(t, filepath.Join(root, "go", "bin", "tool"), 50)
	write(t, filepath.Join(root, "go-build-cache", "ab", "obj-a"), 80)
	write(t, filepath.Join(root, "go-build-cache", "cd", "obj-b"), 80)
	write(t, filepath.Join(root, "go-build-cache", "trim.txt"), 5)
	write(t, filepath.Join(root, "weird name with spaces", "x"), 10)
	w := scan(t, root, DefaultLimits)

	rel := groupByPath(t, w, "choir-updater/releases")
	if child(t, rel, digestA).Files != 1 {
		t.Fatalf("release digest row: %+v", rel)
	}
	if child(t, rel, ".stage-*").AllocatedBytes == 0 {
		t.Fatal("stage leftovers are not named")
	}
	cu := groupByPath(t, w, "choir-updater")
	for _, name := range []string{"releases", "store", "gc-roots"} {
		child(t, cu, name)
	}
	src := groupByPath(t, w, "files/Source")
	if child(t, src, "platform").Files != 1 {
		t.Fatalf("files/Source: %+v", src)
	}
	child(t, groupByPath(t, w, "files"), "Source")
	child(t, groupByPath(t, w, "go"), "pkg")
	child(t, groupByPath(t, w, "go/pkg/mod"), "cache")
	child(t, groupByPath(t, w, "go-build-cache"), "(objects)")
	child(t, groupByPath(t, w, "capsule-artifacts"), "subjects")
	child(t, groupByPath(t, w, "capsule-state"), "boot-abc")

	raw, _ := json.Marshal(w)
	for _, banned := range []string{"taxes", "Documents", "diary", "secret-project", "private notes", "memoir", "weird name", "someone", "github.com"} {
		if strings.Contains(string(raw), banned) {
			t.Fatalf("report names %q:\n%s", banned, raw)
		}
	}
	if topByName(t, w, "(other)").Files != 1 {
		t.Fatalf("an odd top-level name was not folded: %+v", w.Top)
	}
}

func TestChildListIsBounded(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 60; i++ {
		write(t, filepath.Join(root, "choir-updater", "releases", strings.Repeat(string(rune('a'+i%6)), 62)+string(rune('a'+i/6%6))+string(rune('a'+i/36)), "f"), 100*(i+1))
	}
	w := scan(t, root, DefaultLimits)
	rel := groupByPath(t, w, "choir-updater/releases")
	if len(rel.Children) > MaxChildren+1 {
		t.Fatalf("%d children, bound is %d plus the (other) row", len(rel.Children), MaxChildren)
	}
	var sum uint64
	for _, c := range rel.Children {
		sum += c.Files
	}
	if sum != 60 {
		t.Fatalf("children hold %d files, the folded rows lost some of 60", sum)
	}
	for i := 1; i < len(rel.Children); i++ {
		if rel.Children[i-1].Name != OtherName && rel.Children[i].Name != OtherName && rel.Children[i-1].AllocatedBytes < rel.Children[i].AllocatedBytes {
			t.Fatal("children are not largest first")
		}
	}
}

func TestOtherFilesystemIsNotCrossed(t *testing.T) {
	// A real mount cannot be made in a test. The device comparison is the
	// code under test, so the walker exposes it and the test lies about the
	// device of one directory.
	root := t.TempDir()
	write(t, filepath.Join(root, "capsule-state", "boot-x", "mnt", "huge"), 9000)
	write(t, filepath.Join(root, "capsule-state", "boot-x", "ok"), 10)
	var s Scanner
	s.fakeDevice = func(path string) bool { return strings.HasSuffix(path, "/mnt") }
	rep := s.Read(context.Background(), root, DefaultLimits, true)
	c := topByName(t, rep.Walk, "capsule-state")
	if !c.OtherFilesystem || c.Files != 1 || c.ApparentBytes >= 9000 {
		t.Fatalf("capsule-state = %+v: the mount was walked or not reported", c)
	}
}

func scanWith(t *testing.T, root string, lim Limits, hook func(path string)) *Walk {
	t.Helper()
	var s Scanner
	s.testHook = hook
	rep := s.Read(context.Background(), root, lim, true)
	if rep.Walk == nil {
		t.Fatalf("no walk: %+v", rep)
	}
	return rep.Walk
}

func TestMidWalkDeadlineTruncatesOneEntryAndSparesTheNext(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 300; i++ {
		write(t, filepath.Join(root, "aaa-slow", "d", string(rune('a'+i%26))+string(rune('a'+i/26%26))+string(rune('a'+i/676))), 1)
	}
	write(t, filepath.Join(root, "zzz-small", "x"), 1)
	// A clock the test owns: no wall-clock margin. The slow entry's listing
	// takes a simulated 350 ms of a 400 ms budget, so it passes its share
	// (400 ms less the 50 ms promised to the next entry) and is cut, and the
	// next entry still gets its promised share.
	var mu sync.Mutex
	now := time.Unix(1_800_000_000, 0)
	var s Scanner
	s.clock = func() time.Time { mu.Lock(); defer mu.Unlock(); return now }
	var once sync.Once
	s.testHook = func(path string) {
		if strings.HasSuffix(path, "/d") {
			once.Do(func() { mu.Lock(); now = now.Add(360 * time.Millisecond); mu.Unlock() })
		}
	}
	rep := s.Read(context.Background(), root, Limits{Budget: 400 * time.Millisecond, MaxEntries: 1 << 30}, true)
	w := rep.Walk
	if w == nil {
		t.Fatal("no walk")
	}
	if !topByName(t, w, "aaa-slow").Truncated {
		t.Fatal("the entry that ran past its share of the deadline is not truncated")
	}
	if z := topByName(t, w, "zzz-small"); z.Truncated || z.Files != 1 {
		t.Fatalf("the next entry was starved: %+v", z)
	}
}

func TestCancelledRequestStopsTheWalkAndKeepsTheCompleteResult(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 300; i++ {
		write(t, filepath.Join(root, "go", string(rune('a'+i%26)), string(rune('a'+i/26))), 1)
	}
	var s Scanner
	first := s.Read(context.Background(), root, DefaultLimits, true)
	if first.Walk == nil || first.Walk.Truncated {
		t.Fatalf("first walk: %+v", first.Walk)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	done := make(chan Report)
	go func() { done <- s.Read(ctx, root, DefaultLimits, true) }()
	select {
	case rep := <-done:
		if rep.Walk == nil || !rep.Walk.Truncated {
			t.Fatalf("a cancelled walk is not reported truncated: %+v", rep.Walk)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("a cancelled walk did not stop")
	}
	again := s.Read(context.Background(), root, DefaultLimits, false)
	if again.Walk == nil || !again.Walk.ReadAt.Equal(first.Walk.ReadAt) || again.Walk.Truncated {
		t.Fatalf("a cancelled walk replaced the complete one: %+v", again.Walk)
	}
	// running was cleared: the next walk runs.
	if next := s.Read(context.Background(), root, DefaultLimits, true); next.Busy {
		t.Fatal("running stayed set after a cancelled walk")
	}
}

func TestUnreadableDirectoryCountsAnErrorAndTheWalkContinues(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads any directory")
	}
	root := t.TempDir()
	write(t, filepath.Join(root, "go", "locked", "a"), 10)
	write(t, filepath.Join(root, "go", "open", "b"), 10)
	locked := filepath.Join(root, "go", "locked")
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(locked, 0o755)
	g := topByName(t, scan(t, root, DefaultLimits), "go")
	if g.Errors != 1 || g.Files != 1 {
		t.Fatalf("go = %+v, want 1 error and the open file counted", g)
	}
}

func TestUnreadableRootIsAnError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads any directory")
	}
	root := t.TempDir()
	write(t, filepath.Join(root, "go", "a"), 1)
	if err := os.Chmod(root, 0o111); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(root, 0o755)
	w := scan(t, root, DefaultLimits)
	if len(w.Top) != 1 || w.Top[0].Errors == 0 {
		t.Fatalf("an unreadable root reads as an empty volume: %+v", w.Top)
	}
}

func TestStoppedWalkEmitsNoRowsForEntriesNeverExamined(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 300; i++ {
		write(t, filepath.Join(root, "choir-updater", "releases", strings.Repeat("a", 60)+string(rune('a'+i%6))+string(rune('a'+i/6%6))+string(rune('a'+i/36%6))+string(rune('a'+i/216))), 1)
	}
	w := scan(t, root, Limits{Budget: time.Minute, MaxEntries: 50})
	var members int
	for _, c := range groupByPath(t, w, "choir-updater/releases").Children {
		members += max(c.Members, 1)
	}
	if uint64(members) > w.EntriesExamined {
		t.Fatalf("%d rows for %d examined entries: rows were made for entries never stat'd", members, w.EntriesExamined)
	}
}

func TestSmallEntryBoundStillExaminesTheFirstEntries(t *testing.T) {
	root := t.TempDir()
	for _, n := range []string{"a", "b", "c", "d", "e"} {
		write(t, filepath.Join(root, n), 1)
	}
	w := scan(t, root, Limits{Budget: time.Minute, MaxEntries: 2})
	if a := topByName(t, w, "a"); a.Truncated || a.Files != 1 {
		t.Fatalf("the first entry got a zero share: %+v", a)
	}
}

// What can go wrong in the leftovers report, each covered below:
//   - the candidate source copies folded into one row, so a reader cannot say
//     how many there are or which is large (the 2026-11-24 drill record);
//   - a row with a size and no age, so a collector cannot tell a copy a week
//     old from one made a minute ago;
//   - the one large directory cut at an equal share of the walk while its
//     small siblings used almost none of theirs (the walk reported a floor);
//   - a large first directory spending what later entries were promised;
//   - a user's file name appearing under the new groups.

func TestSubjectsAreNamedByDigestWithSizesCountsAndAges(t *testing.T) {
	root := t.TempDir()
	before := time.Now().Add(-time.Second)
	var digests []string
	for i := 0; i < 40; i++ {
		d := strings.Repeat(string(rune('a'+i%6)), 60) + string(rune('a'+i/6%6)) + string(rune('a'+i/36)) + "00"
		digests = append(digests, d)
		write(t, filepath.Join(root, "capsule-artifacts", "subjects", d, "workspace", "platform", "main.go"), 4096*(i+1))
		write(t, filepath.Join(root, "capsule-artifacts", "subjects", d, "workspace", "platform", "go.mod"), 10)
	}
	write(t, filepath.Join(root, "capsule-artifacts", "subjects", "not a digest", "x"), 10)
	time.Sleep(20 * time.Millisecond)
	late := strings.Repeat("f", 64)
	write(t, filepath.Join(root, "capsule-artifacts", "subjects", late, "workspace", "platform", "main.go"), 100)
	w := scan(t, root, DefaultLimits)
	if w.Truncated {
		t.Fatal("a small volume's walk was cut")
	}
	g := groupByPath(t, w, "capsule-artifacts/subjects")
	// All forty-one digests are named: more than MaxChildren, none folded.
	var named int
	for _, c := range g.Children {
		if c.Name != OtherName {
			named++
		}
	}
	if named != 41 {
		t.Fatalf("%d subject rows named, want 41: %+v", named, g.Children)
	}
	big := child(t, g, digests[39])
	if big.Files != 2 || big.AllocatedBytes < 4096*40 {
		t.Fatalf("subject row has no size or count: %+v", big)
	}
	if big.OldestChanged == nil || big.NewestChanged == nil || big.OldestChanged.Before(before) || big.NewestChanged.Before(*big.OldestChanged) {
		t.Fatalf("subject row has no usable age: %+v", big)
	}
	if !child(t, g, late).NewestChanged.After(*big.NewestChanged) {
		t.Fatal("a copy written later does not read as newer")
	}
	other := child(t, g, OtherName)
	if other.Files != 1 {
		t.Fatalf("a name that is not a digest was not folded: %+v", other)
	}
	if strings.Contains(string(mustJSON(t, w)), "not a digest") {
		t.Fatal("the report echoes a name that is not a digest")
	}
	for i := 1; i < len(g.Children); i++ {
		a, b := g.Children[i-1], g.Children[i]
		if a.Name != OtherName && b.Name != OtherName && a.AllocatedBytes < b.AllocatedBytes {
			t.Fatal("subjects are not largest first")
		}
	}
	// The top row for capsule-artifacts carries the whole and its age too.
	if top := topByName(t, w, "capsule-artifacts"); top.Files != 82 || top.OldestChanged == nil {
		t.Fatalf("capsule-artifacts top row: %+v", top)
	}
}

func TestReceiptsAreRowsByKindAndTheLogDirectoriesCarryAges(t *testing.T) {
	root := t.TempDir()
	for _, k := range []string{"execution", "fate", "granted"} {
		for i := 0; i < 3; i++ {
			write(t, filepath.Join(root, "capsule-artifacts", "receipts", k, strings.Repeat("c", 60)+string(rune('a'+i))+".json"), 300)
		}
	}
	write(t, filepath.Join(root, "capsule-artifacts", "receipts", "private-kind", "x"), 5)
	write(t, filepath.Join(root, "capsule-artifacts", "egress", "capsule-1.jsonl"), 800)
	write(t, filepath.Join(root, "capsule-artifacts", "jobs", "j1.json"), 200)
	write(t, filepath.Join(root, "capsule-artifacts", "job-logs", strings.Repeat("d", 64)+".log"), 5000)
	w := scan(t, root, DefaultLimits)
	rec := groupByPath(t, w, "capsule-artifacts/receipts")
	for _, k := range []string{"execution", "fate", "granted"} {
		if c := child(t, rec, k); c.Files != 3 || c.OldestChanged == nil {
			t.Fatalf("receipts/%s: %+v", k, c)
		}
	}
	if child(t, rec, OtherName).Files != 1 {
		t.Fatalf("an unknown receipt kind was named: %+v", rec.Children)
	}
	ca := groupByPath(t, w, "capsule-artifacts")
	for name, files := range map[string]uint64{"egress": 1, "jobs": 1, "job-logs": 1, "receipts": 10} {
		c := child(t, ca, name)
		if c.Files != files || c.OldestChanged == nil || c.NewestChanged == nil || c.AllocatedBytes == 0 {
			t.Fatalf("capsule-artifacts/%s has no size, count and age: %+v", name, c)
		}
	}
}

func TestQuarantinesAreNamedRowsWithAges(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "rematerialize-quarantine-20261115T041047Z", "state.db"), 20000)
	write(t, filepath.Join(root, "rematerialize-quarantine-20261115T054339Z", "state.db"), 30000)
	write(t, filepath.Join(root, "workspace-replaced-20261101T000000Z", "x"), 100)
	w := scan(t, root, DefaultLimits)
	for _, n := range []string{"rematerialize-quarantine-20261115T041047Z", "rematerialize-quarantine-20261115T054339Z", "workspace-replaced-20261101T000000Z"} {
		e := topByName(t, w, n)
		if e.AllocatedBytes == 0 || e.Files == 0 || e.OldestChanged == nil || e.NewestChanged == nil {
			t.Fatalf("%s: %+v", n, e)
		}
	}
}

func TestOneLargeDirectoryFinishesWhenItsSiblingsAreSmall(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 9000; i++ {
		write(t, filepath.Join(root, "capsule-artifacts", "subjects", strings.Repeat("a", 64), "d"+string(rune('a'+i%26)), "f"+strconv.Itoa(i)), 1)
	}
	for i := 0; i < 10; i++ {
		write(t, filepath.Join(root, "small"+strconv.Itoa(i), "x"), 1)
	}
	// An equal share of 20,000 entries over eleven top entries is 1,819: the
	// old walk cut the large one there and reported a floor.
	w := scan(t, root, Limits{Budget: time.Minute, MaxEntries: 20000})
	if w.Truncated || topByName(t, w, "capsule-artifacts").Truncated {
		t.Fatalf("the large directory was cut with room to spare: %+v", topByName(t, w, "capsule-artifacts"))
	}
	if got := topByName(t, w, "capsule-artifacts").Files; got != 9000 {
		t.Fatalf("%d files counted, want 9000", got)
	}
}

func TestALargeFirstDirectoryDoesNotStarveTheEntriesAfterIt(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 15000; i++ {
		write(t, filepath.Join(root, "aaa-huge", "d"+string(rune('a'+i%26)), "f"+strconv.Itoa(i)), 1)
	}
	for i := 0; i < 10; i++ {
		write(t, filepath.Join(root, "small"+strconv.Itoa(i), "x"), 1)
	}
	w := scan(t, root, Limits{Budget: time.Minute, MaxEntries: 20000})
	if !topByName(t, w, "aaa-huge").Truncated {
		t.Fatal("the bound did not apply to the large directory")
	}
	for i := 0; i < 10; i++ {
		if s := topByName(t, w, "small"+strconv.Itoa(i)); s.Truncated || s.Files != 1 {
			t.Fatalf("an entry after the large one was starved: %+v", s)
		}
	}
}

func TestLeftoversReportSample(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "state.db"), 200000)
	write(t, filepath.Join(root, "rematerialize-quarantine-20261115T041047Z", "state.db"), 90000)
	for i, d := range []string{strings.Repeat("a", 64), strings.Repeat("b", 64)} {
		write(t, filepath.Join(root, "capsule-artifacts", "subjects", d, "workspace", "platform", "main.go"), 40000*(i+1))
	}
	write(t, filepath.Join(root, "capsule-artifacts", "receipts", "fate", "r1.json"), 300)
	write(t, filepath.Join(root, "capsule-artifacts", "egress", "capsule-1.jsonl"), 800)
	var s Scanner
	rep := s.Read(context.Background(), root, DefaultLimits, true)
	raw, _ := json.MarshalIndent(rep.Walk, "", " ")
	t.Logf("sample report:\n%s", raw)
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
