// Package taperepack copies legacy tape files (one file per digest under
// sha256/computer-event, computer-event-payload and pin-receipts) into the
// per-computer segment store, and proves, file by file, which of them have a
// verified segment copy. It is steps 2 to 4 of the repack in
// docs/reports/tape-volume-forward-fix-and-surgery-2026-10-19.md section 6.1,
// built for the size Node B has (10 million files in three flat
// directories) and for the delete that follows the census
// (docs/problems/platform-artifact-small-file-growth-2026-10-19.md, "Repack
// tool v2").
//
// Run is a streaming, bucketed join with bounded memory:
//
//  1. List. The three directories are streamed in batches and the names are
//     spilled to 256 files by the first byte of the digest; each pin receipt
//     is read once and its edge (computer, artifact, namespace, length,
//     eligibility) is spilled by the artifact's first byte. The pin receipts
//     and the event bodies in every computer's segments (sealed and unsealed)
//     are read for edges too: a segmented repin of an artifact whose legacy
//     file exists writes its receipt to the segment only (review round 2).
//  2. For each of the 256 buckets, the bucket's names and edges are loaded, and
//     for each computer a tapestore.SegmentView supplies the segment records
//     of that byte. Every legacy file of the bucket is read, hashed against
//     its name, attributed to the computers that need it by the signed
//     receipts, and compared with the segment copies. Copy mode appends the
//     missing records through the production tapestore.Appender (the
//     per-computer lock, one fsync per batch, fail-stop) and reads each back.
//
// Nothing here deletes, moves or edits a legacy file. The Verdict type is the
// r4 guard: only a verified copy makes a file deletable.
package taperepack

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/yusefmosiah/go-choir/internal/tapestore"
)

const (
	buckets = 256

	// pendingCapBytes bounds the record bodies held for appends.
	pendingCapBytes = 64 << 20
	// pendingLowBytes is where a forced flush stops.
	pendingLowBytes = 32 << 20
)

// Options configure Run.
type Options struct {
	// Root is the artifacts root (the directory holding sha256/ and tape-seg/).
	Root string
	// WorkDir holds spill files, state and reports. It must be outside Root.
	WorkDir string
	// Computers limits the run to these computers. Apply requires it: a
	// computer is never copied without being named.
	Computers []string
	// Copy appends missing records to the segments. Without Apply it is a dry
	// run: it counts what it would write and writes nothing.
	Copy bool
	// Apply makes Copy write. It needs Copy and at least one computer.
	Apply bool
	// Fresh discards the saved state of an interrupted apply run.
	Fresh bool
	// Rate limits legacy file reads per second; zero is unlimited.
	Rate int
	// Progress receives a line about every ten seconds.
	Progress func(string)
	// OnVerdict is called once for every listed legacy file, with its verdict,
	// right after the engine compared it. It may stop the run by returning an
	// error. A delete step runs here and unlinks only if v.Deletable().
	OnVerdict func(Verdict) error

	// Test seams.
	testAfterAppend func(bucket int) error // after a bucket's appends, before its result is saved
	testAfterList   func()                 // after the listing phase
	testSyncHook    func(*os.File) error   // the appender's fsync
}

type state struct {
	Version   int              `json:"version"`
	Root      string           `json:"root"`
	Computers []string         `json:"computers"`
	Copy      bool             `json:"copy"`
	Apply     bool             `json:"apply"`
	ListDone  bool             `json:"list_done"`
	Table     []string         `json:"computer_table"`
	Listing   map[string]int64 `json:"listing"`
	Done      []bool           `json:"done"`
	// ListStart is when the list phase began; AttrErrors are computers whose
	// segments could not be read for their receipts (review round 2).
	ListStart  time.Time         `json:"list_start"`
	AttrErrors map[string]string `json:"attr_errors,omitempty"`
}

func (s *state) matches(o Options, root string, filter []string) bool {
	if s.Version != reportVersion || s.Root != root || s.Copy != o.Copy || s.Apply != o.Apply || len(s.Computers) != len(filter) {
		return false
	}
	for i := range filter {
		if s.Computers[i] != filter[i] {
			return false
		}
	}
	return s.ListDone && len(s.Done) == buckets
}

type engine struct {
	opts   Options
	root   string
	want   map[string]bool // nil: every computer
	filter []string
	dir    string // state directory
	st     *state
	table  *computerTable

	reader *tapestore.Reader
	app    *tapestore.Appender

	buf     []byte
	pace    pacer
	touched map[string]bool

	progress   time.Time
	filesSeen  int64
	bucketsRun int
}

// Run lists the legacy files, joins them against the segment store and
// returns the report. See the package comment, and Options for the modes:
// census (read only), Copy (dry run) and Copy with Apply (writes).
func Run(opts Options) (*Report, error) {
	if opts.Root == "" {
		return nil, errors.New("taperepack: --root is required")
	}
	root := filepath.Clean(opts.Root)
	if fi, err := os.Stat(root); err != nil || !fi.IsDir() {
		return nil, fmt.Errorf("taperepack: artifacts root %s is not a directory", root)
	}
	if err := checkWorkDir(opts.WorkDir, root); err != nil {
		return nil, err
	}
	if opts.Apply && !opts.Copy {
		return nil, errors.New("taperepack: apply is a modifier of copy")
	}
	e := &engine{opts: opts, root: root, touched: map[string]bool{}}
	if len(opts.Computers) > 0 {
		want, filter, err := computerSet(opts.Computers)
		if err != nil {
			return nil, err
		}
		e.want, e.filter = want, filter
	}
	if opts.Apply && len(e.want) == 0 {
		return nil, errors.New("taperepack: copy --apply needs at least one --computer; a computer is never copied without being named")
	}
	if err := os.MkdirAll(opts.WorkDir, 0o700); err != nil {
		return nil, err
	}
	release, err := lockWorkDir(filepath.Join(opts.WorkDir, "lock"))
	if err != nil {
		return nil, err
	}
	defer release()

	mode := "census"
	switch {
	case opts.Apply:
		mode = "copy-apply"
	case opts.Copy:
		mode = "copy-dry-run"
	}
	e.dir = filepath.Join(opts.WorkDir, "scan")
	if opts.Apply {
		e.dir = filepath.Join(opts.WorkDir, "apply")
	}
	e.reader = tapestore.NewReader(root)
	if opts.Apply {
		e.app = tapestore.NewAppender(root)
		if opts.testSyncHook != nil {
			e.app.SetSyncHookForTest(opts.testSyncHook)
		}
	}
	e.pace = newPacer(opts.Rate)
	e.progress = time.Now()

	if err := e.prepare(); err != nil {
		return nil, err
	}
	if !e.st.ListDone {
		e.st.ListStart = time.Now()
		if err := e.list(); err != nil {
			return nil, err
		}
		e.st.ListDone = true
		e.st.Table = e.table.names
		if err := e.saveState(); err != nil {
			return nil, err
		}
		if opts.testAfterList != nil {
			opts.testAfterList()
		}
	} else {
		e.table = newComputerTable(e.st.Table)
	}
	for b := 0; b < buckets; b++ {
		if e.st.Done[b] {
			continue
		}
		part, err := e.bucket(b)
		if err != nil {
			return nil, fmt.Errorf("taperepack: bucket %02x: %w", b, err)
		}
		if err := writeJSONAtomic(e.resultPath(b), part); err != nil {
			return nil, err
		}
		e.st.Done[b] = true
		if err := e.saveState(); err != nil {
			return nil, err
		}
		e.bucketsRun++
		e.report(fmt.Sprintf("bucket %d of %d done", b+1, buckets))
	}
	if e.app != nil {
		for _, c := range sortedKeys(e.touched) {
			if err := e.app.Seal(c); err != nil {
				return nil, fmt.Errorf("taperepack: seal computer %s: %w", c, err)
			}
		}
	}

	rep := newReport()
	rep.Mode, rep.Root, rep.Filter = mode, root, e.filter
	for k, v := range e.st.Listing {
		rep.Listing[k] = v
	}
	for b := 0; b < buckets; b++ {
		part := newReport()
		if err := readJSON(e.resultPath(b), part); err != nil {
			return nil, err
		}
		rep.merge(part)
	}
	for c, msg := range e.st.AttrErrors {
		if _, ok := rep.SegmentErrors[c]; !ok {
			rep.SegmentErrors[c] = msg
		}
	}
	rep.Complete = true
	rep.Finished = time.Now().UTC().Format(time.RFC3339)
	raw, digest, err := rep.canonical()
	if err != nil {
		return nil, err
	}
	rep.Receipt = digest
	prefix := "census-"
	if opts.Copy {
		prefix = "copy-"
	}
	if err := writeFileAtomic(filepath.Join(opts.WorkDir, prefix+digest+".json"), raw); err != nil {
		return nil, err
	}
	if err := wipeDir(opts.WorkDir, e.dir); err != nil {
		return nil, err
	}
	return rep, nil
}

// prepare loads a matching saved state (apply runs only) or starts a new one.
func (e *engine) prepare() error {
	if e.opts.Apply && !e.opts.Fresh {
		var st state
		if err := readJSON(filepath.Join(e.dir, "state.json"), &st); err == nil {
			if st.matches(e.opts, e.root, e.filter) {
				e.st = &st
				e.report("resuming an interrupted copy run")
				return nil
			}
			return fmt.Errorf("taperepack: %s holds the state of a different run; rerun with --fresh to discard it", e.dir)
		}
	}
	if err := wipeDir(e.opts.WorkDir, e.dir); err != nil {
		return err
	}
	for _, sub := range []string{"list", "edge"} {
		if err := os.MkdirAll(filepath.Join(e.dir, sub), 0o700); err != nil {
			return err
		}
	}
	e.st = &state{Version: reportVersion, Root: e.root, Computers: e.filter, Copy: e.opts.Copy, Apply: e.opts.Apply, Listing: map[string]int64{}, Done: make([]bool, buckets)}
	e.table = newComputerTable(nil)
	return nil
}

func (e *engine) saveState() error { return writeJSONAtomic(filepath.Join(e.dir, "state.json"), e.st) }

func (e *engine) resultPath(b int) string {
	return filepath.Join(e.dir, fmt.Sprintf("res-%02x.json", b))
}

func (e *engine) report(msg string) {
	if e.opts.Progress == nil {
		return
	}
	if time.Since(e.progress) < 10*time.Second && msg != "" && e.bucketsRun%32 != 0 {
		return
	}
	e.progress = time.Now()
	e.opts.Progress(fmt.Sprintf("taperepack: %s (files read %d)", msg, e.filesSeen))
}

func computerSet(ids []string) (map[string]bool, []string, error) {
	set := map[string]bool{}
	for _, id := range ids {
		if id == "" {
			continue
		}
		if !tapestore.ValidComputerID(id) {
			return nil, nil, fmt.Errorf("taperepack: invalid computer id %q", id)
		}
		set[id] = true
	}
	if len(set) == 0 {
		return nil, nil, errors.New("taperepack: no computer named")
	}
	out := make([]string, 0, len(set))
	for id := range set {
		out = append(out, id)
	}
	sort.Strings(out)
	return set, out, nil
}

// pacer spaces legacy file reads to a rate.
type pacer struct {
	interval time.Duration
	next     time.Time
}

func newPacer(perSecond int) pacer {
	if perSecond <= 0 {
		return pacer{}
	}
	return pacer{interval: time.Second / time.Duration(perSecond)}
}

func (p *pacer) wait() {
	if p.interval == 0 {
		return
	}
	now := time.Now()
	if p.next.After(now) {
		time.Sleep(p.next.Sub(now))
		now = p.next
	}
	p.next = now.Add(p.interval)
}
