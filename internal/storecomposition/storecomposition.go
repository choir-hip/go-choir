// Package storecomposition reads what a SQLite state file is made of, in the
// background, in sizes and counts only: per table the rows and payload bytes;
// per object kind the objects, their bodies and the bytes of each, split by
// version state (current, superseded, tombstoned); the bytes of objects whose
// content hash was already seen; and run memory by kind and by age.
//
// It exists because the paper's store (51.5 GiB, 2026-11-21) cannot be
// measured by internal/storetables: that reading answers inside one HTTP
// request, stops at about 95 s with a third of the file attributed, and
// reports tables only
// (docs/problems/the-papers-store-doubles-in-eleven-days-2026-11-13.md). This
// reading is started by one request and read by later ones, and takes as long
// as it takes.
//
// What keeps it from hurting the running computer:
//   - every batch is one statement on the read pool (a read-only connection),
//     so no read transaction lasts longer than a batch (milliseconds to a few
//     hundred): a long one pins the WAL and the writer's checkpoints behind
//     it. The reading never holds a transaction across batches;
//   - a table is walked by rowid (or, for a WITHOUT ROWID table, by primary
//     key) in batches of BatchRows, the cursor being the last key seen;
//   - between batches it sleeps for as long as the batch took (YieldFactor),
//     so the reading uses at most about half of one reader, however slow the
//     disk;
//   - memory is the batch, the per-kind counters (at most MaxKinds each) and
//     one duplicate bitset (32 MiB, cleared between header/body pairs),
//     whatever the store's size;
//   - one reading at a time; a start while one runs answers the running one;
//   - a deadline for the whole reading (Deadline). A 51.5 GiB file at 20 to
//     100 MB/s of useful reading is 10 minutes to 3 hours of reading plus the
//     yields; 8 hours leaves every plausible disk room and still ends a
//     reading that has gone wrong. At the deadline it stops and reports what
//     it has, marked incomplete. This is a deadline on our own reading, not
//     a limit on any model call.
//
// Concurrency. The reading is not a snapshot: each batch is its own
// statement, and the computer keeps writing. A row inserted ahead of the
// cursor is counted; one inserted behind it is not; a row deleted after it was
// counted stays counted. A header deleted between its pair batch and the
// orphan batch for the same id range leaves its body counted twice in the
// body table's rows (once matched, once as an orphan). So the counts are a
// reading over a span of time, close for a store that changes slowly and
// never exact. When the store is replaced mid-reading (a Reopen swaps the
// pools; a rematerialize or a compaction swaps the file) the reading ends
// incomplete with reason store_replaced instead of blending two stores.
//
// Payload bytes, everywhere in the report, are one unit: the byte length of each
// stored column value, a text or blob by its length in bytes, an integer or a
// real as 8, a null as 0; stored generated columns count, virtual ones do not
// (they are computed, not stored). It is the payload, not the pages: no cell,
// page or index overhead, no free pages. File sizes (file_bytes, wal_bytes,
// store.bytes) are file sizes, a different thing. A running report has
// incomplete=false: state "running" is the partial signal.
//
// The report is kept in memory and in one small JSON file beside the store.
// A restart during a reading leaves the file saying "running"; opening the
// scanner turns that into "interrupted". It never resumes (CLAUDE.md,
// "Restarts").
//
// The report carries no content: the only strings that come from the store
// are table names and object, event and run-memory kinds, and a kind must
// look like a kind (kindOK) or it is counted under "other".
package storecomposition

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/bits"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Reading states.
const (
	StateRunning     = "running"
	StateDone        = "done"
	StateIncomplete  = "incomplete"
	StateInterrupted = "interrupted"
	StateFailed      = "failed"
)

// Why a reading is incomplete (a fixed vocabulary, never a database message).
const (
	ReasonDeadline  = "deadline"
	ReasonCancelled = "cancelled"
	ReasonRestart   = "restart"
	ReasonReadError = "read_error"
	// ReasonStoreReplaced: the store's pool or file changed under the reading.
	ReasonStoreReplaced = "store_replaced"
	// ReasonUnsupportedKey: a WITHOUT ROWID table whose key has a descending
	// column cannot be walked in key order without a sort per batch.
	ReasonUnsupportedKey = "unsupported_key"
)

// MaxPersistedBytes is the largest persisted report that is read back; a real
// report is about 100 KB.
const MaxPersistedBytes = 8 << 20

// maxPersistedTables bounds a loaded report's table list.
const maxPersistedTables = 4096

// Scope says what the report's numbers leave out.
const Scope = "the state file's main database file only: payload bytes of stored column values; the WAL (wal_bytes) and the telemetry partition files are not walked and not in these numbers"

// digestNote says what the duplicate columns are.
const digestNote = "duplicate_* counts objects whose content_hash (a hash of kind, body and metadata) was seen on an earlier row; duplicate_bytes is those objects' body bytes; bodies carry no digest column of their own and no body was hashed here; an upper bound (a 2^28-bit seen set, false positives about rows/2^28)"

// ReadingNote says what kind of numbers these are.
const ReadingNote = "a reading over a live store, not a snapshot: each batch is its own statement while the computer keeps writing, so a row inserted ahead of the cursor is counted, one behind it is not, a row deleted after it was counted stays counted, and a body whose header was deleted between two batches can be counted twice; close for a slow store, never exact"

var errStoreReplaced = errors.New("store replaced")

type tableFailure struct{ reason string }

func (e tableFailure) Error() string { return "table failure: " + e.reason }

// MaxKinds bounds the distinct kinds kept per breakdown; later ones are "other".
const MaxKinds = 256

// OtherKind is where a kind that does not look like a kind, or that arrives
// after MaxKinds, is counted.
const OtherKind = "other"

// dupBits is the duplicate-hash bitset size (2^28 bits, 32 MiB). A hash that
// lands on a set bit counts as seen before; a collision makes that a false
// positive at about rows/2^28 (2% at 5.4 million rows), so the duplicate
// bytes are an upper bound, said so in the report.
const dupBits = 1 << 28

// Config tunes a scanner. Zero values take the defaults.
type Config struct {
	// BatchRows is the rows one statement reads.
	BatchRows int
	// YieldFactor is the sleep after a batch, as a multiple of the batch's
	// own time (default 1: at most half of one reader).
	YieldFactor float64
	// MinYield is the least sleep between batches.
	MinYield time.Duration
	// Deadline ends the whole reading.
	Deadline time.Duration
	// PersistEvery is how often a running reading's progress is written to
	// the file.
	PersistEvery time.Duration
	// Now is the clock (tests).
	Now func() time.Time
	// StateFile is the state.db path. Its device and inode identify the
	// store: a reading ends when they change, and a persisted report of
	// another file is dropped. Empty skips both checks.
	StateFile string
}

// Defaults.
const (
	DefaultBatchRows    = 2000
	DefaultMinYield     = 2 * time.Millisecond
	DefaultDeadline     = 8 * time.Hour
	DefaultPersistEvery = 30 * time.Second
)

func (c Config) withDefaults() Config {
	if c.BatchRows <= 0 {
		c.BatchRows = DefaultBatchRows
	}
	if c.YieldFactor <= 0 {
		c.YieldFactor = 1
	}
	if c.MinYield <= 0 {
		c.MinYield = DefaultMinYield
	}
	if c.Deadline <= 0 {
		c.Deadline = DefaultDeadline
	}
	if c.PersistEvery <= 0 {
		c.PersistEvery = DefaultPersistEvery
	}
	if c.Now == nil {
		c.Now = time.Now
	}
	return c
}

// Report is the whole answer. Every field is a count, a size, a time, a
// state word or a table or kind name; nothing is copied from a row's text.
type Report struct {
	State string `json:"state"`
	// Incomplete is true when the reading stopped before the end of every
	// table; Reason says why (the Reason constants). The numbers are floors.
	Incomplete bool   `json:"incomplete,omitempty"`
	Reason     string `json:"reason,omitempty"`

	StartedAt  time.Time  `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`

	Progress Progress `json:"progress"`

	// Store is the state file as it was when the reading began; WALBytes the
	// size of its -wal file then (a size only). Scope is what the numbers
	// leave out.
	Store       StoreStamp `json:"store"`
	WALBytes    int64      `json:"wal_bytes,omitempty"`
	Scope       string     `json:"scope"`
	ReadingNote string     `json:"reading_note"`

	PageSize  int64 `json:"page_size,omitempty"`
	PageCount int64 `json:"page_count,omitempty"`
	FileBytes int64 `json:"file_bytes,omitempty"`

	// Tables is every table walked: rows and payload bytes (the byte length
	// of each stored column value, without page, cell or index overhead).
	Tables []TableStat `json:"tables,omitempty"`
	// TablesOmitted is how many tables a persisted report left out to stay
	// loadable (the smallest by payload bytes); a live report has all.
	TablesOmitted int64 `json:"tables_omitted,omitempty"`
	// Objects is og_objects joined to og_bodies by kind. Events is the same
	// for choir_events and choir_event_bodies.
	Objects *Objects `json:"objects,omitempty"`
	Events  *Objects `json:"events,omitempty"`
	// RunMemory is run_memory_entries by kind and by age.
	RunMemory *RunMemory `json:"run_memory,omitempty"`
}

// StoreStamp names a state file: device and inode (stable across writes,
// changed by a rematerialize or a compaction swap), and its size and
// modification time when the reading began.
type StoreStamp struct {
	Device     uint64    `json:"device,omitempty"`
	Inode      uint64    `json:"inode,omitempty"`
	Bytes      int64     `json:"bytes,omitempty"`
	ModifiedAt time.Time `json:"modified_at,omitempty"`
}

// stampOf stats path; ok is false when it cannot.
func stampOf(path string) (StoreStamp, bool) {
	if path == "" {
		return StoreStamp{}, false
	}
	fi, err := os.Stat(path)
	if err != nil {
		return StoreStamp{}, false
	}
	dev, ino := sysIdentity(fi)
	return StoreStamp{Device: dev, Inode: ino, Bytes: fi.Size(), ModifiedAt: fi.ModTime().UTC()}, true
}

// Progress is how far a reading has got.
type Progress struct {
	// Phase is the table being walked now (a table name), or empty.
	Phase       string `json:"phase,omitempty"`
	RowsWalked  int64  `json:"rows_walked"`
	Batches     int64  `json:"batches"`
	TablesDone  int    `json:"tables_done"`
	TablesTotal int    `json:"tables_total"`
}

// TableStat is one table.
type TableStat struct {
	Table        string `json:"table"`
	Rows         int64  `json:"rows"`
	PayloadBytes int64  `json:"payload_bytes"`
	// Complete is false for a table the reading did not finish.
	Complete bool `json:"complete"`
	// OrphanRows, on a body table, is its rows with no header row.
	OrphanRows  int64 `json:"orphan_rows,omitempty"`
	OrphanBytes int64 `json:"orphan_bytes,omitempty"`
	// Failure, when not empty, is why this table's reading stopped (a Reason
	// constant); the reading went on with the next table.
	Failure string `json:"failure,omitempty"`
}

// KindStat is one object kind. An object is tombstoned, else superseded (its
// superseded_by names a newer object), else current. Bytes are header column
// bytes plus body bytes.
type KindStat struct {
	Kind        string `json:"kind"`
	Objects     int64  `json:"objects"`
	Current     int64  `json:"current"`
	Superseded  int64  `json:"superseded"`
	Tombstoned  int64  `json:"tombstoned"`
	HeaderBytes int64  `json:"header_bytes"`
	BodyCount   int64  `json:"body_count"`
	BodyBytes   int64  `json:"body_bytes"`
	BodyMin     int64  `json:"body_min"`
	BodyMax     int64  `json:"body_max"`
	// BodyMedianAtMost is the upper edge of the power-of-two size bucket
	// holding the median body (the median is no more than this).
	BodyMedianAtMost int64 `json:"body_median_at_most"`
	CurrentBytes     int64 `json:"current_bytes"`
	SupersededBytes  int64 `json:"superseded_bytes"`
	TombstonedBytes  int64 `json:"tombstoned_bytes"`
	// DuplicateObjects counts objects whose content hash was already seen on
	// an earlier row; DuplicateBytes is those objects' body bytes (what a
	// body dedup would save, the first copy not counted). An upper bound:
	// see dupBits.
	DuplicateObjects int64 `json:"duplicate_objects"`
	DuplicateBytes   int64 `json:"duplicate_bytes"`
}

// Objects is a header table joined to its body table.
type Objects struct {
	HeaderTable string     `json:"header_table"`
	BodyTable   string     `json:"body_table"`
	Complete    bool       `json:"complete"`
	Kinds       []KindStat `json:"kinds"`
	Total       KindStat   `json:"total"`
	// OrphanBodyRows and OrphanBodyBytes are body rows with no header row,
	// reported on their own line (they are in the body table's totals too).
	OrphanBodyRows  int64 `json:"orphan_body_rows"`
	OrphanBodyBytes int64 `json:"orphan_body_bytes"`
	// DigestNote says what the duplicate columns are.
	DigestNote string `json:"digest_note"`
}

// RunMemory is run_memory_entries.
type RunMemory struct {
	Complete bool       `json:"complete"`
	ByKind   []RMCell   `json:"by_kind"`
	ByAge    []RMCell   `json:"by_age"`
	Cells    []RMKindAg `json:"cells"`
}

// RMCell is rows and bytes under one label (a kind or an age bucket).
type RMCell struct {
	Label string `json:"label"`
	Rows  int64  `json:"rows"`
	Bytes int64  `json:"bytes"`
}

// RMKindAg is rows and bytes of one kind in one age bucket.
type RMKindAg struct {
	Kind  string `json:"kind"`
	Age   string `json:"age"`
	Rows  int64  `json:"rows"`
	Bytes int64  `json:"bytes"`
}

// Age buckets, by created_at against the reading's start.
const (
	AgeDay     = "day"     // the last day (yesterday's date or later)
	AgeWeek    = "week"    // after the last day, within seven days
	AgeOlder   = "older"   // earlier
	AgeUnknown = "unknown" // created_at is not a date
)

// Scanner runs one reading at a time and keeps the last.
type Scanner struct {
	cfg  Config
	path string

	base     context.Context
	baseStop context.CancelFunc

	mu      sync.Mutex
	running *run
	last    *Report
	closed  bool
}

// Open returns a scanner that keeps its report in path ("" keeps it in
// memory only). A file left by a reading that was running is rewritten as
// interrupted: a restart never resumes.
func Open(path string, cfg Config) *Scanner {
	s := &Scanner{cfg: cfg.withDefaults(), path: path}
	s.base, s.baseStop = context.WithCancel(context.Background())
	if path == "" {
		return s
	}
	// Read through an opened descriptor, bounded: a file that is larger than
	// MaxPersistedBytes (whatever stat said earlier) is refused.
	f, err := os.Open(path)
	if err != nil {
		return s
	}
	data, err := io.ReadAll(io.LimitReader(f, MaxPersistedBytes+1))
	_ = f.Close()
	if err != nil || len(data) > MaxPersistedBytes {
		return s
	}
	// A file that is not this package's report (corrupt, another shape, a
	// string outside the report's vocabulary, too many entries) is ignored,
	// and the next reading replaces it.
	var rep Report
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.DisallowUnknownFields()
	if dec.Decode(&rep) != nil || !validReport(&rep) {
		return s
	}
	// A report of another state file (the file beside the store survives a
	// rematerialize and a compaction swap) is not this store's.
	if now, ok := stampOf(s.cfg.StateFile); ok && rep.Store.Inode != 0 && (rep.Store.Inode != now.Inode || rep.Store.Device != now.Device) {
		return s
	}
	if rep.State == StateRunning {
		now := s.cfg.Now().UTC()
		rep.State = StateInterrupted
		rep.Incomplete = true
		rep.Reason = ReasonRestart
		rep.FinishedAt = &now
		s.persist(&rep)
	}
	s.last = &rep
	return s
}

var (
	reasons = map[string]bool{"": true, ReasonDeadline: true, ReasonCancelled: true, ReasonRestart: true, ReasonReadError: true, ReasonStoreReplaced: true, ReasonUnsupportedKey: true}
	ages    = map[string]bool{AgeDay: true, AgeWeek: true, AgeOlder: true, AgeUnknown: true}
	states  = map[string]bool{StateRunning: true, StateDone: true, StateIncomplete: true, StateInterrupted: true, StateFailed: true}
)

// nameOK says a table or phase name is short and has no control character.
func nameOK(n string) bool {
	if len(n) > 128 {
		return false
	}
	for _, r := range n {
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	return true
}

func objectsOK(o *Objects) bool {
	if o == nil {
		return true
	}
	if len(o.Kinds) > MaxKinds+1 || !nameOK(o.HeaderTable) || !nameOK(o.BodyTable) || o.DigestNote != digestNote || !kindOK.MatchString(o.Total.Kind) {
		return false
	}
	for _, k := range o.Kinds {
		if !kindOK.MatchString(k.Kind) {
			return false
		}
	}
	return true
}

// validReport is whether every string and count in a loaded report is from
// the report's own vocabulary and within its bounds. A file written by this
// package always passes; anything else is not served.
func validReport(rep *Report) bool {
	if !states[rep.State] || !reasons[rep.Reason] || rep.Scope != Scope || rep.ReadingNote != ReadingNote || !nameOK(rep.Progress.Phase) {
		return false
	}
	if len(rep.Tables) > maxPersistedTables {
		return false
	}
	for _, t := range rep.Tables {
		if !nameOK(t.Table) || !reasons[t.Failure] {
			return false
		}
	}
	if !objectsOK(rep.Objects) || !objectsOK(rep.Events) {
		return false
	}
	if rm := rep.RunMemory; rm != nil {
		if len(rm.ByKind) > MaxKinds+1 || len(rm.ByAge) > len(ages) || len(rm.Cells) > (MaxKinds+1)*len(ages) {
			return false
		}
		for _, c := range rm.ByKind {
			if !kindOK.MatchString(c.Label) {
				return false
			}
		}
		for _, c := range rm.ByAge {
			if !ages[c.Label] {
				return false
			}
		}
		for _, c := range rm.Cells {
			if !kindOK.MatchString(c.Kind) || !ages[c.Age] {
				return false
			}
		}
	}
	return true
}

// Status is the running reading's progress, else the last report, else nil.
func (s *Scanner) Status() *Report {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running != nil {
		rep := s.running.snapshot()
		return &rep
	}
	if s.last == nil {
		return nil
	}
	rep := *s.last
	return &rep
}

// Start begins a reading unless one is running, in which case it returns the
// running one's status and started is false; after Close it starts nothing.
// db is asked for before each batch, not kept: a store that is reopened gives
// a new read pool (store.ReadDB's contract). The reading belongs to the
// scanner, not to the caller's request, and ends at the deadline or at Close.
func (s *Scanner) Start(db func() *sql.DB) (rep Report, started bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running != nil {
		return s.running.snapshot(), false
	}
	if s.closed {
		return Report{State: StateInterrupted, Incomplete: true, Reason: ReasonCancelled, StartedAt: s.cfg.Now().UTC()}, false
	}
	ctx, cancel := context.WithTimeout(s.base, s.cfg.Deadline)
	r := &run{s: s, source: db, cfg: s.cfg, cancel: cancel, started: s.cfg.Now().UTC(), lastSave: time.Now()}
	if st, ok := stampOf(s.cfg.StateFile); ok {
		r.stamp = st
		if fi, err := os.Stat(s.cfg.StateFile + "-wal"); err == nil {
			r.walBytes = fi.Size()
		}
	}
	s.running = r
	first := r.snapshot()
	s.persist(&first)
	go r.main(ctx)
	return first, true
}

// Cancel stops a running reading; it reports what it has.
func (s *Scanner) Cancel() {
	s.mu.Lock()
	r := s.running
	s.mu.Unlock()
	if r != nil {
		r.cancel()
	}
}

// Close stops a running reading, waits for it (bounded by wait), and refuses
// every later Start. Runtime.Stop calls it before the store closes.
func (s *Scanner) Close(wait time.Duration) {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
	s.baseStop()
	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()
	_ = s.Wait(ctx)
}

// Wait blocks until no reading runs (tests, shutdown).
func (s *Scanner) Wait(ctx context.Context) error {
	for {
		s.mu.Lock()
		r := s.running
		s.mu.Unlock()
		if r == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-r.done():
		}
	}
}

func (s *Scanner) persist(rep *Report) {
	if s.path == "" {
		return
	}
	out := *rep
	// The loader is strict about names; the writer never writes one it would
	// refuse: an out-of-vocabulary table or phase name becomes a placeholder.
	out.Tables = append([]TableStat(nil), out.Tables...)
	for i := range out.Tables {
		if !nameOK(out.Tables[i].Table) {
			out.Tables[i].Table = fmt.Sprintf("table_%d", i)
		}
	}
	if !nameOK(out.Progress.Phase) {
		out.Progress.Phase = "table"
	}
	for _, o := range []**Objects{&out.Objects, &out.Events} {
		if *o != nil && (!nameOK((*o).HeaderTable) || !nameOK((*o).BodyTable)) {
			c := **o
			if !nameOK(c.HeaderTable) {
				c.HeaderTable = "header_table"
			}
			if !nameOK(c.BodyTable) {
				c.BodyTable = "body_table"
			}
			*o = &c
		}
	}
	if len(out.Tables) > maxPersistedTables {
		// A file the loader would refuse helps no one: keep the largest
		// tables and say how many were left out.
		t := append([]TableStat(nil), out.Tables...)
		sort.SliceStable(t, func(i, j int) bool { return t[i].PayloadBytes > t[j].PayloadBytes })
		out.TablesOmitted = int64(len(t) - maxPersistedTables)
		out.Tables = t[:maxPersistedTables]
	}
	data, err := json.Marshal(&out)
	if err != nil {
		return
	}
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return
	}
	tmp, err := os.CreateTemp(dir, ".store-composition-*.tmp")
	if err != nil {
		return
	}
	name := tmp.Name()
	_, werr := tmp.Write(data)
	cerr := tmp.Close()
	if werr != nil || cerr != nil || os.Chmod(name, 0o640) != nil || os.Rename(name, s.path) != nil {
		_ = os.Remove(name)
	}
}

// kindOK says a string looks like a kind: short, lowercase letters, digits
// and . _ - only. Anything else is counted as "other", so a kind written by a
// stranger cannot carry text into the report.
var kindOK = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)

type kindAcc struct {
	KindStat
	hist [65]int64
}

type kindSet struct {
	m map[string]*kindAcc
}

func (k *kindSet) get(kind string) *kindAcc {
	if !kindOK.MatchString(kind) {
		kind = OtherKind
	}
	if k.m == nil {
		k.m = map[string]*kindAcc{}
	}
	a := k.m[kind]
	if a == nil {
		if len(k.m) >= MaxKinds && kind != OtherKind {
			return k.get(OtherKind)
		}
		a = &kindAcc{}
		a.Kind = kind
		a.BodyMin = -1
		k.m[kind] = a
	}
	return a
}

// run is one reading.
type run struct {
	s *Scanner
	// source is the store's read pool func; only pool() calls it.
	source func() *sql.DB
	cfg    Config
	cancel context.CancelFunc
	doneCh chan struct{}
	once   sync.Once

	started time.Time

	rows    atomic.Int64
	batches atomic.Int64

	mu         sync.Mutex // guards the fields below
	phase      string
	tablesDone int
	tablesAll  int
	tables     []TableStat // finished tables
	live       []TableStat // the table being walked, as far as it has got
	liveFn     func()      // rebuilds the partial composition (walker goroutine only)
	objects    *Objects
	events     *Objects
	runMem     *RunMemory
	pageSize   int64
	pageCount  int64
	lastSave   time.Time
	failed     bool
	failReason string // the one reason every failed table shares; read_error if they differ
	firstDB    *sql.DB
	stamp      StoreStamp
	walBytes   int64
	dup        []uint64 // one seen-set, cleared between pairs (walker goroutine only)
}

func (r *run) done() <-chan struct{} {
	r.once.Do(func() { r.doneCh = make(chan struct{}) })
	return r.doneCh
}

func (r *run) snapshot() Report {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.reportLocked(StateRunning, false, "", nil)
}

func (r *run) reportLocked(state string, incomplete bool, reason string, fin *time.Time) Report {
	rep := Report{
		State: state, Incomplete: incomplete, Reason: reason,
		StartedAt: r.started, FinishedAt: fin,
		Progress: Progress{Phase: r.phase, RowsWalked: r.rows.Load(), Batches: r.batches.Load(), TablesDone: r.tablesDone, TablesTotal: r.tablesAll},
		Store:    r.stamp, WALBytes: r.walBytes, Scope: Scope, ReadingNote: ReadingNote,
		PageSize: r.pageSize, PageCount: r.pageCount, FileBytes: r.pageSize * r.pageCount,
		Tables:  append(append([]TableStat(nil), r.tables...), r.live...),
		Objects: r.objects, Events: r.events, RunMemory: r.runMem,
	}
	return rep
}

func (r *run) main(ctx context.Context) {
	r.done()
	err := r.walk(ctx)
	if err == nil {
		// A swap that landed on the last statement is not "done".
		_, err = r.pool()
	}
	r.dup = nil
	fin := r.cfg.Now().UTC()
	r.mu.Lock()
	var rep Report
	switch {
	case err == nil && !r.failed:
		r.phase = ""
		rep = r.reportLocked(StateDone, false, "", &fin)
	case err == nil:
		// Every table was tried; at least one could not be read. The phase
		// of the last failure stays visible in the table's own line.
		r.phase = ""
		rep = r.reportLocked(StateIncomplete, true, r.failReason, &fin)
	case errors.Is(err, errStoreReplaced):
		rep = r.reportLocked(StateIncomplete, true, ReasonStoreReplaced, &fin)
	case errors.Is(err, context.DeadlineExceeded):
		rep = r.reportLocked(StateIncomplete, true, ReasonDeadline, &fin)
	case errors.Is(err, context.Canceled):
		rep = r.reportLocked(StateInterrupted, true, ReasonCancelled, &fin)
	default:
		rep = r.reportLocked(StateFailed, true, ReasonReadError, &fin)
	}
	r.mu.Unlock()
	r.s.persist(&rep)
	r.s.mu.Lock()
	r.s.last = &rep
	r.s.running = nil
	r.s.mu.Unlock()
	r.cancel()
	close(r.doneCh)
}

type tableInfo struct {
	name       string
	withoutRow bool
}

func quoteIdent(s string) string { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }

// valBytes is the SQL for the report's one byte unit over a column
// expression: an integer or real is 8, a null 0, text and blob their length
// in bytes. octet_length is the byte length without reading a value's
// overflow pages (measured: microseconds for 400 MB of 2 MB values), so a
// large body costs its row, not its bytes.
func valBytes(col string) string {
	return "CASE typeof(" + col + ") WHEN 'null' THEN 0 WHEN 'integer' THEN 8 WHEN 'real' THEN 8 ELSE octet_length(" + col + ") END"
}

// sumExpr is the byte total of a row's stored columns; alias prefixes them
// ("h" gives h."col"), empty for none.
func sumExpr(cols []string, alias string) string {
	parts := make([]string, 0, len(cols))
	for _, c := range cols {
		q := quoteIdent(c)
		if alias != "" {
			q = alias + "." + q
		}
		parts = append(parts, valBytes(q))
	}
	return strings.Join(parts, "+")
}

// step is one unit of the walk and the tables it reports.
type step struct {
	tables []string
	fn     func() error
}

// walk reads every table: the object pair first (the question the reading
// exists for), run memory, the event pair, then the rest by name. A table
// that cannot be read is recorded as failed and the walk goes on; only the
// deadline or a cancel ends it early.
func (r *run) walk(ctx context.Context) error {
	db, err := r.pool()
	if err != nil {
		return err
	}
	var pageSize, pageCount int64
	_ = db.QueryRowContext(ctx, `PRAGMA page_size`).Scan(&pageSize)
	_ = db.QueryRowContext(ctx, `PRAGMA page_count`).Scan(&pageCount)
	r.mu.Lock()
	r.pageSize, r.pageCount = pageSize, pageCount
	r.mu.Unlock()

	rows, err := db.QueryContext(ctx, `SELECT name, wr FROM pragma_table_list WHERE schema = 'main' AND type IN ('table', 'shadow') AND name NOT LIKE 'sqlite\_%' ESCAPE '\' ORDER BY name`)
	if err != nil {
		return err
	}
	have := map[string]tableInfo{}
	var names []string
	for rows.Next() {
		var name string
		var wr int
		if err := rows.Scan(&name, &wr); err != nil {
			_ = rows.Close()
			return err
		}
		have[name] = tableInfo{name: name, withoutRow: wr != 0}
		names = append(names, name)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	r.mu.Lock()
	r.tablesAll = len(names)
	r.mu.Unlock()

	special := map[string]bool{}
	var steps []step
	pair := func(header, body, id string) {
		if _, ok := have[header]; !ok {
			return
		}
		if _, ok := have[body]; !ok {
			return
		}
		special[header], special[body] = true, true
		steps = append(steps, step{[]string{header, body}, func() error { return r.walkPair(ctx, header, body, id) }})
	}
	pair("og_objects", "og_bodies", "obj_id")
	if _, ok := have["run_memory_entries"]; ok {
		special["run_memory_entries"] = true
		steps = append(steps, step{[]string{"run_memory_entries"}, func() error { return r.walkRunMemory(ctx) }})
	}
	pair("choir_events", "choir_event_bodies", "ev_id")
	for _, n := range names {
		if special[n] {
			continue
		}
		ti := have[n]
		steps = append(steps, step{[]string{n}, func() error { return r.walkTable(ctx, ti) }})
	}
	for _, st := range steps {
		// The pool is checked before every step: a closed or replaced one
		// ends the reading as store_replaced, not as a run of read errors.
		if _, perr := r.pool(); perr != nil {
			return perr
		}
		err := st.fn()
		if err == nil {
			continue
		}
		if cerr := ctx.Err(); cerr != nil {
			return cerr
		}
		if errors.Is(err, errStoreReplaced) {
			return err
		}
		if _, perr := r.pool(); perr != nil {
			return perr
		}
		reason := ReasonReadError
		var tf tableFailure
		if errors.As(err, &tf) {
			reason = tf.reason
		}
		r.fail(st.tables, reason)
	}
	return nil
}

// fail records that these tables could not be read (a fixed reason, never the
// database's message). The phase stays where it was.
func (r *run) fail(tables []string, reason string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.failed = true
	switch r.failReason {
	case "":
		r.failReason = reason
	case reason:
	default:
		r.failReason = ReasonReadError
	}
	for _, name := range tables {
		found := false
		for i := range r.tables {
			if r.tables[i].Table == name {
				r.tables[i].Failure, r.tables[i].Complete = reason, false
				found = true
			}
		}
		if !found {
			r.tables = append(r.tables, TableStat{Table: name, Failure: reason})
			r.tablesDone++
		}
	}
}

// yield sleeps as long as the work just done took (YieldFactor), at least
// MinYield; false when the reading's context ended first.
func (r *run) yield(ctx context.Context, start time.Time) error {
	pause := max(r.cfg.MinYield, time.Duration(float64(time.Since(start))*r.cfg.YieldFactor))
	t := time.NewTimer(pause)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// batchLoop runs fn once per batch until it reports done. The pool is
// fetched once per iteration, checked against the store the reading began on,
// and that exact handle is passed to fn: no statement runs on a pool the
// check did not see. The check is repeated after fn, even when it reports
// done. The partial composition is published after each batch, and the loop
// sleeps between batches. fn's statements are each bounded by BatchRows.
func (r *run) batchLoop(ctx context.Context, fn func(ctx context.Context, db *sql.DB) (done bool, err error)) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		db, err := r.pool()
		if err != nil {
			return err
		}
		start := time.Now()
		done, err := fn(ctx, db)
		if _, perr := r.pool(); perr != nil {
			return perr
		}
		if err != nil {
			if cerr := ctx.Err(); cerr != nil {
				return cerr
			}
			return err
		}
		r.batches.Add(1)
		if r.liveFn != nil {
			r.liveFn()
		}
		r.maybePersist()
		if done {
			return nil
		}
		if err := r.yield(ctx, start); err != nil {
			return err
		}
	}
}

// pool is the one place the store's pool is fetched. It returns errStoreReplaced
// when the pool is no longer the one the reading began on, or the state file
// is no longer the same file.
func (r *run) pool() (*sql.DB, error) {
	db := r.source()
	if r.firstDB == nil {
		r.firstDB = db
	}
	if db != r.firstDB {
		return nil, errStoreReplaced
	}
	if r.stamp.Inode != 0 {
		if now, ok := stampOf(r.cfg.StateFile); ok && (now.Inode != r.stamp.Inode || now.Device != r.stamp.Device) {
			return nil, errStoreReplaced
		}
	}
	return db, nil
}

func (r *run) maybePersist() {
	r.mu.Lock()
	now := time.Now()
	if now.Sub(r.lastSave) < r.cfg.PersistEvery {
		r.mu.Unlock()
		return
	}
	r.lastSave = now
	rep := r.reportLocked(StateRunning, false, "", nil)
	r.mu.Unlock()
	r.s.persist(&rep)
}

func (r *run) setPhase(name string) {
	r.mu.Lock()
	r.phase = name
	r.mu.Unlock()
}

// endTable moves a table's stat from live to finished.
func (r *run) endTable(st TableStat, complete bool) {
	st.Complete = complete
	r.mu.Lock()
	r.tables = append(r.tables, st)
	r.live = nil
	r.tablesDone++
	r.mu.Unlock()
	r.liveFn = nil
}

// walkTable reads one other table: rows and payload bytes.
func (r *run) walkTable(ctx context.Context, ti tableInfo) error {
	r.setPhase(ti.name)
	st := TableStat{Table: ti.name}
	r.liveFn = func() {
		r.mu.Lock()
		r.live = []TableStat{st}
		r.mu.Unlock()
	}
	finish := func(err error) error { r.endTable(st, err == nil); return err }
	db, err := r.pool()
	if err != nil {
		return finish(err)
	}
	cols, pk, err := r.columns(ctx, db, ti.name)
	if err != nil {
		return finish(err)
	}
	// A table cannot have only generated columns (SQLite refuses it), so cols
	// is never empty; were it, the rows are still counted, at 0 bytes each.
	expr := sumExpr(cols, "")
	if len(cols) == 0 {
		expr = "0"
	}
	t := quoteIdent(ti.name)
	if !ti.withoutRow {
		// next is the inclusive lower bound of the next batch, so a row whose
		// rowid is the smallest integer is read.
		q := `SELECT COUNT(*), COALESCE(SUM(x),0), COALESCE(MAX(r),0) FROM (SELECT rowid AS r, ` + expr + ` AS x FROM ` + t + ` WHERE rowid >= ? ORDER BY rowid LIMIT ?)`
		next := int64(math.MinInt64)
		return finish(r.batchLoop(ctx, func(ctx context.Context, db *sql.DB) (bool, error) {
			var n, bytes, last int64
			if err := db.QueryRowContext(ctx, q, next, r.cfg.BatchRows).Scan(&n, &bytes, &last); err != nil {
				return false, err
			}
			st.Rows += n
			st.PayloadBytes += bytes
			r.rows.Add(n)
			if last == math.MaxInt64 {
				return true, nil
			}
			next = last + 1
			return n < int64(r.cfg.BatchRows), nil
		}))
	}
	if len(pk) == 0 {
		return finish(errors.New("no key"))
	}
	var desc int
	if err := db.QueryRowContext(ctx, `SELECT COALESCE(SUM(x.desc), 0) FROM pragma_index_list(?) l, pragma_index_xinfo(l.name) x WHERE l.origin = 'pk' AND x.key = 1`, ti.name).Scan(&desc); err != nil {
		return finish(err)
	}
	if desc > 0 {
		// A descending key column makes "key > cursor" a range scan and a
		// sort per batch: quadratic. Not walked.
		return finish(tableFailure{ReasonUnsupportedKey})
	}
	var qc []string
	for _, c := range pk {
		qc = append(qc, quoteIdent(c))
	}
	keys := strings.Join(qc, ",")
	where := ` WHERE ` + keys + ` > ?`
	if len(pk) > 1 {
		where = ` WHERE (` + keys + `) > (` + strings.TrimSuffix(strings.Repeat("?,", len(pk)), ",") + `)`
	}
	first := `SELECT ` + keys + `, ` + expr + ` FROM ` + t + ` ORDER BY ` + keys + ` LIMIT ?`
	next := `SELECT ` + keys + `, ` + expr + ` FROM ` + t + where + ` ORDER BY ` + keys + ` LIMIT ?`
	var cursor []any
	return finish(r.batchLoop(ctx, func(ctx context.Context, db *sql.DB) (bool, error) {
		var rs *sql.Rows
		var err error
		if cursor == nil {
			rs, err = db.QueryContext(ctx, first, r.cfg.BatchRows)
		} else {
			rs, err = db.QueryContext(ctx, next, append(append([]any(nil), cursor...), r.cfg.BatchRows)...)
		}
		if err != nil {
			return false, err
		}
		defer rs.Close()
		n := 0
		cur := make([]any, len(pk))
		for rs.Next() {
			var b int64
			dest := make([]any, 0, len(pk)+1)
			for i := range cur {
				dest = append(dest, &cur[i])
			}
			dest = append(dest, &b)
			if err := rs.Scan(dest...); err != nil {
				return false, err
			}
			st.Rows++
			st.PayloadBytes += b
			n++
		}
		if err := rs.Err(); err != nil {
			return false, err
		}
		r.rows.Add(int64(n))
		if n > 0 {
			cursor = append([]any(nil), cur...)
		}
		return n < r.cfg.BatchRows, nil
	}))
}

// columns returns a table's stored columns (a virtual generated column is
// computed, not stored, and reading it would compute it; a stored generated
// column is stored and counts) and its primary key columns in key order.
func (r *run) columns(ctx context.Context, db *sql.DB, table string) (cols, pk []string, err error) {
	rs, err := db.QueryContext(ctx, `SELECT name, pk, hidden FROM pragma_table_xinfo(?)`, table)
	if err != nil {
		return nil, nil, err
	}
	defer rs.Close()
	type pkc struct {
		name string
		ord  int
	}
	var pks []pkc
	for rs.Next() {
		var name string
		var ord, hidden int
		if err := rs.Scan(&name, &ord, &hidden); err != nil {
			return nil, nil, err
		}
		if hidden != 0 && hidden != 3 {
			continue
		}
		cols = append(cols, name)
		if ord > 0 {
			pks = append(pks, pkc{name, ord})
		}
	}
	if err := rs.Err(); err != nil {
		return nil, nil, err
	}
	sort.Slice(pks, func(i, j int) bool { return pks[i].ord < pks[j].ord })
	for _, p := range pks {
		pk = append(pk, p.name)
	}
	return cols, pk, nil
}

// orphans counts the body rows in [lo, hi] that have no header row, and
// their bytes, in statements of at most BatchRows orphans each, so no
// statement is a snapshot that spans the table. The anti-join is one
// statement, so a header written while the walk runs is not mistaken for a
// missing one.
func (r *run) orphans(ctx context.Context, header, body, id string, lo, hi int64) (rows, bytes int64, err error) {
	q := `SELECT COUNT(*), COALESCE(SUM(x),0), COALESCE(MAX(i),0) FROM (SELECT b.` + id + ` AS i, ` + valBytes("b."+id) + `+` + valBytes("b.body") + ` AS x FROM ` + quoteIdent(body) + ` b WHERE b.` + id + ` >= ? AND b.` + id + ` <= ? AND NOT EXISTS (SELECT 1 FROM ` + quoteIdent(header) + ` h WHERE h.` + id + ` = b.` + id + `) ORDER BY b.` + id + ` LIMIT ?)`
	for {
		start := time.Now()
		// The pool is fetched and checked for each statement: a long orphan
		// tail does not outlive a replaced store.
		db, err := r.pool()
		if err != nil {
			return rows, bytes, err
		}
		var n, b, last int64
		if err := db.QueryRowContext(ctx, q, lo, hi, r.cfg.BatchRows).Scan(&n, &b, &last); err != nil {
			return rows, bytes, err
		}
		rows += n
		bytes += b
		if n < int64(r.cfg.BatchRows) || last == math.MaxInt64 {
			return rows, bytes, nil
		}
		lo = last + 1
		if err := r.yield(ctx, start); err != nil {
			return rows, bytes, err
		}
	}
}

// walkPair reads a header table with its body table: per kind, the objects,
// their version states, header and body bytes, body size spread, duplicate
// content hashes; and the body rows that have no header.
func (r *run) walkPair(ctx context.Context, header, body, id string) error {
	r.setPhase(header)
	ks := &kindSet{}
	var total kindAcc
	total.Kind = "total"
	total.BodyMin = -1
	hs := TableStat{Table: header}
	bs := TableStat{Table: body}
	var orphanRows, orphanBytes int64
	// The seen-set is this pair's: a hash in another table is no duplicate.
	if r.dup == nil {
		r.dup = make([]uint64, dupBits/64)
	}
	clear(r.dup)
	dup := r.dup

	build := func(complete bool) (*Objects, []TableStat) {
		o := &Objects{HeaderTable: header, BodyTable: body, Complete: complete,
			OrphanBodyRows: orphanRows, OrphanBodyBytes: orphanBytes,
			DigestNote: digestNote}
		for _, a := range ks.m {
			o.Kinds = append(o.Kinds, finishKind(a))
		}
		sort.Slice(o.Kinds, func(i, j int) bool {
			if o.Kinds[i].BodyBytes != o.Kinds[j].BodyBytes {
				return o.Kinds[i].BodyBytes > o.Kinds[j].BodyBytes
			}
			return o.Kinds[i].Kind < o.Kinds[j].Kind
		})
		o.Total = finishKind(&total)
		h, b := hs, bs
		h.Complete, b.Complete = complete, complete
		b.OrphanRows, b.OrphanBytes = orphanRows, orphanBytes
		return o, []TableStat{h, b}
	}
	publish := func(live bool, complete bool) {
		o, ts := build(complete)
		r.mu.Lock()
		if header == "og_objects" {
			r.objects = o
		} else {
			r.events = o
		}
		if live {
			r.live = ts
		} else {
			r.tables = append(r.tables, ts...)
			r.live = nil
			r.tablesDone += 2
		}
		r.mu.Unlock()
	}
	r.liveFn = func() { publish(true, false) }
	defer func() { r.liveFn = nil }()

	pdb, err := r.pool()
	if err != nil {
		publish(false, false)
		return err
	}
	hcols, _, err := r.columns(ctx, pdb, header)
	if err != nil {
		publish(false, false)
		return err
	}
	q := `SELECT h.` + id + `, h.object_kind, h.tombstone, h.superseded_by <> '', h.content_hash, ` + sumExpr(hcols, "h") + `, b.` + id + ` IS NOT NULL, ` + valBytes("b.body") + `
FROM ` + quoteIdent(header) + ` h LEFT JOIN ` + quoteIdent(body) + ` b ON b.` + id + ` = h.` + id + `
WHERE h.` + id + ` >= ? ORDER BY h.` + id + ` LIMIT ?`
	// next is the inclusive lower bound of the next header batch.
	next := int64(math.MinInt64)
	err = r.batchLoop(ctx, func(ctx context.Context, db *sql.DB) (bool, error) {
		rs, err := db.QueryContext(ctx, q, next, r.cfg.BatchRows)
		if err != nil {
			return false, err
		}
		defer rs.Close()
		n := 0
		after := next
		for rs.Next() {
			var rid int64
			var kind string
			var tomb, sup, hasBody bool
			var hash string
			var hb, bb int64
			if err := rs.Scan(&rid, &kind, &tomb, &sup, &hash, &hb, &hasBody, &bb); err != nil {
				return false, err
			}
			after = rid
			n++
			a := ks.get(kind)
			dupHit := hash != "" && seenBefore(dup, hash)
			for _, x := range []*kindAcc{a, &total} {
				x.Objects++
				x.HeaderBytes += hb
				bytes := hb + bb
				switch {
				case tomb:
					x.Tombstoned++
					x.TombstonedBytes += bytes
				case sup:
					x.Superseded++
					x.SupersededBytes += bytes
				default:
					x.Current++
					x.CurrentBytes += bytes
				}
				if hasBody {
					x.BodyCount++
					x.BodyBytes += bb
					if x.BodyMin < 0 || bb < x.BodyMin {
						x.BodyMin = bb
					}
					if bb > x.BodyMax {
						x.BodyMax = bb
					}
					x.hist[bits.Len64(uint64(bb))]++
				}
				if dupHit {
					x.DuplicateObjects++
					x.DuplicateBytes += bb
				}
			}
			hs.PayloadBytes += hb
			if hasBody {
				bs.Rows++
				bs.PayloadBytes += bb + 8 // the body row's id column: an integer is 8
			}
		}
		if err := rs.Err(); err != nil {
			return false, err
		}
		_ = rs.Close()
		hs.Rows += int64(n)
		r.rows.Add(int64(n))
		// Body rows with no header, in the id range this batch covered (the
		// last batch also covers everything past the last header).
		hi := after
		last := n < r.cfg.BatchRows || after == math.MaxInt64
		if last {
			hi = math.MaxInt64
		}
		oRows, oBytes, err := r.orphans(ctx, header, body, id, next, hi)
		orphanRows += oRows
		orphanBytes += oBytes
		bs.Rows += oRows
		bs.PayloadBytes += oBytes
		if err != nil {
			return false, err
		}
		if n > 0 && after < math.MaxInt64 {
			next = after + 1
		}
		return last, nil
	})
	if err != nil {
		publish(false, false)
		return err
	}
	publish(false, true)
	return nil
}

func finishKind(a *kindAcc) KindStat {
	s := a.KindStat
	if s.BodyMin < 0 {
		s.BodyMin = 0
	}
	if s.BodyCount > 0 {
		var cum int64
		for k, n := range a.hist {
			cum += n
			if cum*2 >= s.BodyCount {
				edge := int64(1)<<uint(k) - 1
				if k >= 63 {
					edge = s.BodyMax
				}
				s.BodyMedianAtMost = min(edge, s.BodyMax)
				break
			}
		}
	}
	return s
}

// seenBefore reports whether hash's slot was already set, and sets it. Ask once per row.
func seenBefore(bitset []uint64, hash string) bool {
	// FNV-1a, inline: no hasher and no byte slice per row.
	h := uint64(14695981039346656037)
	for i := 0; i < len(hash); i++ {
		h ^= uint64(hash[i])
		h *= 1099511628211
	}
	slot := h % dupBits
	w, m := slot/64, uint64(1)<<(slot%64)
	if bitset[w]&m != 0 {
		return true
	}
	bitset[w] |= m
	return false
}

// walkRunMemory reads run_memory_entries by kind and age.

// walkRunMemory reads run_memory_entries by kind and age.
func (r *run) walkRunMemory(ctx context.Context) error {
	const table = "run_memory_entries"
	r.setPhase(table)
	now := r.started
	dayCut := now.AddDate(0, 0, -1).Format("2006-01-02")
	weekCut := now.AddDate(0, 0, -7).Format("2006-01-02")
	type key struct{ kind, age string }
	cells := map[key]*RMCell{}
	kindsSeen := map[string]bool{}
	st := TableStat{Table: table}
	build := func(complete bool) (*RunMemory, TableStat) {
		rm := &RunMemory{Complete: complete}
		byKind, byAge := map[string]*RMCell{}, map[string]*RMCell{}
		for k, c := range cells {
			rm.Cells = append(rm.Cells, RMKindAg{Kind: k.kind, Age: k.age, Rows: c.Rows, Bytes: c.Bytes})
			for _, p := range []struct {
				m map[string]*RMCell
				l string
			}{{byKind, k.kind}, {byAge, k.age}} {
				e := p.m[p.l]
				if e == nil {
					e = &RMCell{Label: p.l}
					p.m[p.l] = e
				}
				e.Rows += c.Rows
				e.Bytes += c.Bytes
			}
		}
		for _, e := range byKind {
			rm.ByKind = append(rm.ByKind, *e)
		}
		for _, e := range byAge {
			rm.ByAge = append(rm.ByAge, *e)
		}
		less := func(s []RMCell) {
			sort.Slice(s, func(i, j int) bool {
				if s[i].Bytes != s[j].Bytes {
					return s[i].Bytes > s[j].Bytes
				}
				return s[i].Label < s[j].Label
			})
		}
		less(rm.ByKind)
		less(rm.ByAge)
		sort.Slice(rm.Cells, func(i, j int) bool {
			a, b := rm.Cells[i], rm.Cells[j]
			if a.Bytes != b.Bytes {
				return a.Bytes > b.Bytes
			}
			return a.Kind+a.Age < b.Kind+b.Age
		})
		t := st
		t.Complete = complete
		return rm, t
	}
	publish := func(live, complete bool) {
		rm, t := build(complete)
		r.mu.Lock()
		r.runMem = rm
		if live {
			r.live = []TableStat{t}
		} else {
			r.tables = append(r.tables, t)
			r.live = nil
			r.tablesDone++
		}
		r.mu.Unlock()
	}
	r.liveFn = func() { publish(true, false) }
	defer func() { r.liveFn = nil }()

	pdb, err := r.pool()
	if err != nil {
		publish(false, false)
		return err
	}
	cols, _, err := r.columns(ctx, pdb, table)
	if err != nil {
		publish(false, false)
		return err
	}
	q := `SELECT rowid, kind, substr(created_at,1,10), ` + sumExpr(cols, "") + ` FROM run_memory_entries WHERE rowid >= ? ORDER BY rowid LIMIT ?`
	dateRE := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	next := int64(math.MinInt64)
	hitMax := false
	err = r.batchLoop(ctx, func(ctx context.Context, db *sql.DB) (bool, error) {
		rs, err := db.QueryContext(ctx, q, next, r.cfg.BatchRows)
		if err != nil {
			return false, err
		}
		defer rs.Close()
		n := 0
		for rs.Next() {
			var rid, bytes int64
			var kind, date string
			if err := rs.Scan(&rid, &kind, &date, &bytes); err != nil {
				return false, err
			}
			if rid < math.MaxInt64 {
				next = rid + 1
			} else {
				hitMax = true
			}
			n++
			if !kindOK.MatchString(kind) || (!kindsSeen[kind] && len(kindsSeen) >= MaxKinds) {
				kind = OtherKind
			}
			kindsSeen[kind] = true
			age := AgeUnknown
			switch {
			case !dateRE.MatchString(date):
			case date >= dayCut:
				age = AgeDay
			case date >= weekCut:
				age = AgeWeek
			default:
				age = AgeOlder
			}
			c := cells[key{kind, age}]
			if c == nil {
				c = &RMCell{}
				cells[key{kind, age}] = c
			}
			c.Rows++
			c.Bytes += bytes
			st.Rows++
			st.PayloadBytes += bytes
		}
		if err := rs.Err(); err != nil {
			return false, err
		}
		r.rows.Add(int64(n))
		return n < r.cfg.BatchRows || hitMax, nil
	})
	if err != nil {
		publish(false, false)
		return err
	}
	publish(false, true)
	return nil
}
