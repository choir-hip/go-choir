// Package datavolume reports what fills a computer's persistent data volume:
// the filesystem's own accounting (total, free, available to unprivileged
// users, so the reserved blocks show) and a bounded, read-only walk giving,
// for each top-level directory and for a fixed list of second-level ones,
// allocated bytes (st_blocks*512), apparent bytes and file count.
//
// The walk reads metadata only: it never opens a file, never follows a
// symlink, never crosses onto another filesystem, counts a hard-linked inode
// once, tolerates entries that vanish, and stops at a wall-time deadline or an
// entry bound and says so (truncated). Only one walk runs at a time; a request
// that arrives during one gets the last result. It names top-level entries and
// a fixed set of second-level directories, and never a user's file name:
// everything else under those is folded into one "(other)" row.
package datavolume

import (
	"context"
	"io"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
)

// OtherName is the row that holds every entry the report does not name.
const OtherName = "(other)"

// MaxChildren bounds the rows of one second-level group, besides OtherName.
const MaxChildren = 24

// Limits bounds one walk. Budget is a wall-time deadline for the whole walk;
// MaxEntries is the number of directory entries examined. Neither is a call
// limit: they bound the work one diagnostic request may cause.
type Limits struct {
	Budget     time.Duration
	MaxEntries int
}

// DefaultLimits keeps one walk to tens of seconds on a volume holding a full
// Go module cache, a build cache and a store.
var DefaultLimits = Limits{Budget: 25 * time.Second, MaxEntries: 3_000_000}

// Statfs is the filesystem's accounting. FreeBytes counts the blocks the root
// user could still use; AvailBytes those an unprivileged user can; the
// difference is the reserved blocks.
type Statfs struct {
	TotalBytes    uint64 `json:"total_bytes"`
	FreeBytes     uint64 `json:"free_bytes"`
	AvailBytes    uint64 `json:"avail_bytes"`
	ReservedBytes uint64 `json:"reserved_bytes"`
}

// Entry is one named row of the walk.
type Entry struct {
	Name           string `json:"name"`
	AllocatedBytes uint64 `json:"allocated_bytes"`
	ApparentBytes  uint64 `json:"apparent_bytes"`
	// Files counts inodes that are not directories (a symlink is one).
	Files uint64 `json:"files"`
	// Members is how many directory entries this row folds, when more than one.
	Members int `json:"members,omitempty"`
	// Truncated says the walk stopped inside this row: its numbers are a floor.
	Truncated bool `json:"truncated"`
	// Errors counts entries that could not be read for a reason other than
	// vanishing.
	Errors uint64 `json:"errors,omitempty"`
	// OtherFilesystem says a mount point sits inside and was not walked.
	OtherFilesystem bool `json:"other_filesystem,omitempty"`
}

// Group is the named children of one listed directory.
type Group struct {
	Path     string  `json:"path"`
	Children []Entry `json:"children"`
}

// Walk is the result of one walk.
type Walk struct {
	ReadAt          time.Time `json:"read_at"`
	ElapsedMillis   int64     `json:"elapsed_ms"`
	EntriesExamined uint64    `json:"entries_examined"`
	Truncated       bool      `json:"truncated"`
	Top             []Entry   `json:"top"`
	Detail          []Group   `json:"detail"`
}

// Report is the data_volume section of guest observability.
type Report struct {
	Root        string  `json:"root"`
	Statfs      *Statfs `json:"statfs,omitempty"`
	StatfsError string  `json:"statfs_error,omitempty"`
	// Walk is the last walk of this root: this request's, or, when Busy, the
	// one before the walk that was running; absent before any walk.
	Walk *Walk `json:"walk,omitempty"`
	// Busy says another walk was running, so this request did none.
	Busy bool `json:"busy,omitempty"`
}

// StatfsOf reads the filesystem accounting of the volume holding dir.
func StatfsOf(dir string) (Statfs, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(dir, &st); err != nil {
		return Statfs{}, err
	}
	bs := uint64(st.Bsize)
	out := Statfs{TotalBytes: uint64(st.Blocks) * bs, FreeBytes: uint64(st.Bfree) * bs, AvailBytes: uint64(st.Bavail) * bs}
	if out.FreeBytes > out.AvailBytes {
		out.ReservedBytes = out.FreeBytes - out.AvailBytes
	}
	return out, nil
}

// Scanner runs walks one at a time and keeps the last. The zero value works.
type Scanner struct {
	mu       sync.Mutex
	running  bool
	lastRoot string
	last     *Walk

	// testHook runs after a directory chunk is listed and before its entries
	// are examined; fakeDevice makes a path look like another filesystem.
	testHook   func(dir string)
	fakeDevice func(path string) bool
}

// Read reports the volume rooted at root. The statfs is always fresh. With
// wantWalk it walks, unless a walk is already running, in which case it
// answers with Busy and the last result. Without wantWalk it only attaches the
// last result, if there is one for this root.
func (s *Scanner) Read(ctx context.Context, root string, lim Limits, wantWalk bool) Report {
	rep := Report{Root: root}
	if st, err := StatfsOf(root); err == nil {
		rep.Statfs = &st
	} else {
		rep.StatfsError = err.Error()
	}
	s.mu.Lock()
	if s.lastRoot == root {
		rep.Walk = s.last
	}
	if !wantWalk {
		s.mu.Unlock()
		return rep
	}
	if s.running {
		s.mu.Unlock()
		rep.Busy = true
		return rep
	}
	s.running = true
	s.mu.Unlock()

	var w *Walk
	defer func() {
		s.mu.Lock()
		s.running = false
		// A walk cut short by the request's own cancellation does not
		// replace a result that others may read.
		if w != nil && (ctx.Err() == nil || s.lastRoot != root || s.last == nil) {
			s.last, s.lastRoot = w, root
		}
		s.mu.Unlock()
	}()
	w = s.walk(ctx, root, lim)
	rep.Walk = w
	return rep
}

var (
	topNameRE   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+-]{0,63}$`)
	digestRE    = regexp.MustCompile(`^[0-9a-f]{64}$`)
	systemName  = regexp.MustCompile(`^[a-z][a-z0-9-]{0,63}$`)
	cacheBucket = regexp.MustCompile(`^[0-9a-f]{2}$`)
)

// labeler maps a real child name to the row that holds it; "" folds it into
// OtherName.
type labeler func(name string) string

func fixed(names ...string) labeler {
	set := map[string]bool{}
	for _, n := range names {
		set[n] = true
	}
	return func(name string) string {
		if set[name] {
			return name
		}
		return ""
	}
}

// fixedTemp is fixed that also names a leftover temp tree of the program's
// own making. Only for directories a program writes, never a user's.
func fixedTemp(names ...string) labeler {
	f := fixed(names...)
	return func(name string) string {
		if l := f(name); l != "" {
			return l
		}
		return tempLabel(name)
	}
}

// tempLabel names a leftover of the form ".stage-1234" as ".stage-*": the
// updater, the capsule executor and the pointer swap make such names, and a
// stale one is a leak worth seeing.
func tempLabel(name string) string {
	if strings.HasPrefix(name, ".") && len(name) < 64 {
		if i := strings.Index(name[1:], "-"); i > 0 && systemName.MatchString(name[1:i+1]) {
			return name[:i+2] + "*"
		}
	}
	return ""
}

// detailSpecs are the directories listed one level deeper, with the names
// each may show. A user's file names (under files/) are never among them.
var detailSpecs = map[string]labeler{
	"choir-updater": fixed("releases", "store", "gc-roots", "operations", "incoming", "current"),
	"choir-updater/releases": func(n string) string {
		if digestRE.MatchString(n) {
			return n
		}
		return tempLabel(n)
	},
	"files":        fixed("Source"),
	"files/Source": fixed("platform"),
	"capsule-state": func(n string) string {
		if systemName.MatchString(n) {
			return n
		}
		return tempLabel(n)
	},
	"capsule-artifacts": func(n string) string {
		if systemName.MatchString(n) {
			return n
		}
		return tempLabel(n)
	},
	"go":         fixed("pkg", "bin"),
	"go/pkg/mod": fixed("cache"),
	"go-build-cache": func(n string) string {
		if cacheBucket.MatchString(n) {
			return "(objects)"
		}
		return ""
	},
}

// DetailPaths lists the directories the walk reports one level deeper.
func DetailPaths() []string {
	out := make([]string, 0, len(detailSpecs))
	for p := range detailSpecs {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

type inodeKey struct{ dev, ino uint64 }

type totals struct {
	alloc, apparent, files, errs uint64
	truncated, otherFS           bool
}

func (t *totals) add(o totals) {
	t.alloc += o.alloc
	t.apparent += o.apparent
	t.files += o.files
	t.errs += o.errs
	t.truncated = t.truncated || o.truncated
	t.otherFS = t.otherFS || o.otherFS
}

func (t totals) entry(name string, members int) Entry {
	e := Entry{Name: name, AllocatedBytes: t.alloc, ApparentBytes: t.apparent, Files: t.files, Truncated: t.truncated, Errors: t.errs, OtherFilesystem: t.otherFS}
	if members > 1 {
		e.Members = members
	}
	return e
}

type row struct {
	t       totals
	members int
}

type walker struct {
	s        *Scanner
	dev      uint64
	seen     map[inodeKey]struct{}
	examined uint64
	entryCap uint64
	deadline time.Time
	ctx      context.Context
	groups   map[string]map[string]*row
	stopped  bool
}

func (w *walker) stop() bool {
	if w.stopped {
		return true
	}
	if w.examined >= w.entryCap || (w.examined&127 == 0 && (time.Now().After(w.deadline) || w.ctx.Err() != nil)) {
		w.stopped = true
	}
	return w.stopped
}

func (s *Scanner) walk(ctx context.Context, root string, lim Limits) *Walk {
	start := time.Now()
	out := &Walk{ReadAt: start.UTC(), Top: []Entry{}, Detail: []Group{}}
	defer func() { out.ElapsedMillis = time.Since(start).Milliseconds() }()
	var rootSt syscall.Stat_t
	if err := syscall.Stat(root, &rootSt); err != nil {
		out.Top = append(out.Top, Entry{Name: OtherName, Errors: 1})
		return out
	}
	w := &walker{s: s, dev: uint64(rootSt.Dev), seen: map[inodeKey]struct{}{}, ctx: ctx, groups: map[string]map[string]*row{}}
	deadline := start.Add(lim.Budget)
	names, listErr := listNames(root)
	top := map[string]*row{}
	for i, name := range names {
		remaining := len(names) - i
		// Each entry may use an equal share of what is left; what an entry
		// does not use passes to the ones after it.
		left := uint64(0)
		if uint64(lim.MaxEntries) > w.examined {
			left = uint64(lim.MaxEntries) - w.examined
		}
		w.entryCap = w.examined + (left+uint64(remaining)-1)/uint64(remaining)
		w.deadline = time.Now().Add(deadline.Sub(time.Now()) / time.Duration(remaining))
		w.stopped = false
		label := name
		if !topNameRE.MatchString(name) {
			label = OtherName
		}
		t, ok := w.visit(root, name, name)
		if !ok {
			continue
		}
		r := top[label]
		if r == nil {
			r = &row{}
			top[label] = r
		}
		r.t.add(t)
		r.members++
		out.Truncated = out.Truncated || t.truncated
	}
	if listErr != nil {
		r := top[OtherName]
		if r == nil {
			r = &row{}
			top[OtherName] = r
		}
		r.t.errs++
	}
	out.EntriesExamined = w.examined
	out.Top = rows(top, 0)
	for _, p := range DetailPaths() {
		if g, ok := w.groups[p]; ok {
			out.Detail = append(out.Detail, Group{Path: p, Children: rows(g, MaxChildren)})
		}
	}
	return out
}

func listNames(dir string) ([]string, error) {
	f, err := os.Open(dir)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	names, err := f.Readdirnames(-1)
	sort.Strings(names)
	return names, err
}

// rows turns accumulated rows into a list, largest allocated first, with
// OtherName last. A positive max folds the rows past the largest max into
// OtherName.
func rows(m map[string]*row, max int) []Entry {
	type named struct {
		name string
		r    *row
	}
	var list []named
	var other *row
	for name, r := range m {
		if name == OtherName {
			other = r
			continue
		}
		list = append(list, named{name, r})
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].r.t.alloc != list[j].r.t.alloc {
			return list[i].r.t.alloc > list[j].r.t.alloc
		}
		return list[i].name < list[j].name
	})
	if max > 0 && len(list) > max {
		if other == nil {
			other = &row{}
		}
		for _, n := range list[max:] {
			other.t.add(n.r.t)
			other.members += n.r.members
		}
		list = list[:max]
	}
	out := make([]Entry, 0, len(list)+1)
	for _, n := range list {
		out = append(out, n.r.t.entry(n.name, n.r.members))
	}
	if other != nil {
		out = append(out, other.t.entry(OtherName, other.members))
	}
	return out
}

// visit examines the entry name in dir (rel is its path under the root) and,
// for a directory on the volume, everything beneath it. ok is false when the
// entry vanished.
func (w *walker) visit(dir, name, rel string) (totals, bool) {
	var t totals
	if w.stop() {
		t.truncated = true
		return t, true
	}
	w.examined++
	full := path.Join(dir, name)
	var st syscall.Stat_t
	if err := syscall.Lstat(full, &st); err != nil {
		if os.IsNotExist(err) {
			return t, false
		}
		t.errs++
		return t, true
	}
	return w.account(full, rel, &st)
}

func (w *walker) account(full, rel string, st *syscall.Stat_t) (totals, bool) {
	var t totals
	isDir := st.Mode&syscall.S_IFMT == syscall.S_IFDIR
	if uint64(st.Dev) != w.dev || (w.s.fakeDevice != nil && isDir && w.s.fakeDevice(full)) {
		t.otherFS = true
		return t, true
	}
	if !isDir && uint64(st.Nlink) > 1 {
		key := inodeKey{uint64(st.Dev), uint64(st.Ino)}
		if _, dup := w.seen[key]; dup {
			return t, true
		}
		w.seen[key] = struct{}{}
	}
	t.alloc = uint64(st.Blocks) * 512
	t.apparent = uint64(st.Size)
	if !isDir {
		t.files = 1
		return t, true
	}
	f, err := os.Open(full)
	if err != nil {
		if !os.IsNotExist(err) {
			t.errs++
		}
		return t, true
	}
	defer f.Close()
	spec := detailSpecs[rel]
	var g map[string]*row
	if spec != nil {
		g = w.groups[rel]
		if g == nil {
			g = map[string]*row{}
			w.groups[rel] = g
		}
	}
	for {
		entries, err := f.ReadDir(256)
		if w.s.testHook != nil && len(entries) > 0 {
			w.s.testHook(full)
		}
		for _, e := range entries {
			if w.stop() {
				t.truncated = true
				break
			}
			ct, ok := w.visit(full, e.Name(), rel+"/"+e.Name())
			if !ok {
				continue
			}
			t.add(ct)
			if g != nil {
				label := spec(e.Name())
				if label == "" {
					label = OtherName
				}
				r := g[label]
				if r == nil {
					r = &row{}
					g[label] = r
				}
				r.t.add(ct)
				r.members++
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			t.errs++
			break
		}
		if w.stop() {
			t.truncated = true
			break
		}
	}
	if w.stopped {
		t.truncated = true
	}
	return t, true
}
