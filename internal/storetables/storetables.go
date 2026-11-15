// Package storetables reads what fills a SQLite state file: the file's own
// accounting from its header (pages, free pages, page size, auto_vacuum) and,
// for every table and index, the pages, bytes, cells and rowid range the
// dbstat virtual table reports, rolled up per table with its indexes.
//
// It exists because a guest store of 43.76 GiB had no per-table reading
// (docs/problems/the-papers-store-doubles-in-eleven-days-2026-11-13.md). A
// dbstat walk visits every page of the b-tree it names, so a full reading of
// a store that size reads the whole file, the same read a snapshot capture
// makes. The reading is therefore on request only, and bounded:
//
//   - one wall-time budget for the whole reading, shared out by weight (the
//     previous reading's bytes when there is one; a request may name the trees
//     to read, Limits.Only): a
//     table's b-tree weighs four times an index's, and each tree gets its
//     share of what is left when it starts, so one big tree cannot leave the
//     others unread (a tree that finishes early gives its time back);
//   - a tree cut at its share reports the pages it saw (a floor, the leftmost
//     part of the tree) and says Partial; a tree never reached says neither
//     Complete nor Partial; Truncated says either happened;
//   - each tree is its own short read transaction on the read pool, so no
//     transaction pins the WAL for the whole reading (a snapshot capture's
//     does, and grew it to 2.6 GB);
//   - one tree's single read is capped at MaxTreeRead (15 s), so the longest
//     read transaction the reading holds is 15 s, never the whole budget; a
//     tree cut at the cap with budget to spare is reported partial (there is
//     no resume);
//   - the header and the schema cost a few page reads and are always taken
//     (at most headerBudget); a rowid table's extents are two b-tree edge
//     reads, taken only while the budget lasts, so the whole reading, extents
//     included, ends within the budget plus the header's own few reads;
//   - only one reading runs at a time; a request that arrives during one gets
//     Busy and the last reading.
//
// The reading never writes and never takes the write connection. It does read
// pages from disk, which competes with the guest's own I/O for its duration:
// that is the stated cost of an on-request diagnostic, and no pacing is added.
// A table is
// named by its schema name and nothing else of its content is read: no row,
// no body, no key.
package storetables

import (
	"context"
	"database/sql"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// MaxOnly caps how many tree names one request may name.
const MaxOnly = 16

// Limits bounds one reading. Budget is the wall time of the whole reading.
// Only, when not empty, names the trees to read (tables or indexes, matched
// exactly against their schema names): the whole budget is shared among them
// in the order given, no other tree is walked or listed, and the reading is
// not kept as the scanner's last. At most MaxOnly names are used.
type Limits struct {
	Budget time.Duration
	Only   []string
}

// DefaultLimits is one minute: under the server's 120 s write timeout, and
// long enough to read a few GiB of the largest trees.
var DefaultLimits = Limits{Budget: 60 * time.Second}

// MaxBudget caps a requested budget below the server's 120 s write timeout,
// so a reading always answers before the connection is cut.
const MaxBudget = 100 * time.Second

func clampBudget(d time.Duration) time.Duration {
	if d <= 0 {
		return DefaultLimits.Budget
	}
	return min(d, MaxBudget)
}

// ClampBudget is the budget a request for d gets.
func ClampBudget(d time.Duration) time.Duration { return clampBudget(d) }

// MaxTreeRead caps one tree's single read in a default (unnamed) reading, and
// so the longest read transaction such a reading holds: an unasked dbstat
// query must not pin a WAL snapshot for the whole budget. A tree the request
// names (Limits.Only) is not capped by it: the caller asked for that read, and
// its cap is its share of the remaining budget (at most MaxBudget, 100 s,
// far below the snapshot capture's own WAL pin).
const MaxTreeRead = 15 * time.Second

var maxTreeRead = MaxTreeRead

var (
	withoutRowidRE  = regexp.MustCompile(`(?i)\bWITHOUT\s+ROWID\b`)
	createVirtualRE = regexp.MustCompile(`(?i)^\s*CREATE\s+VIRTUAL\b`)
)

// headerBudget bounds the header pragmas and one tree's rowid extents, which
// are a few page reads each.
const headerBudget = 5 * time.Second

// HeaderBudget is headerBudget, for a caller reserving time for it.
const HeaderBudget = headerBudget

// Header is the file's own accounting: what the file is, and how much of it
// is free pages a delete left and SQLite has not returned.
type Header struct {
	PageSize      int64 `json:"page_size"`
	PageCount     int64 `json:"page_count"`
	FreelistCount int64 `json:"freelist_count"`
	// AutoVacuum is the file's mode: 0 none, 1 full, 2 incremental. Only 2
	// lets incremental_vacuum return free pages to the filesystem.
	AutoVacuum int64  `json:"auto_vacuum"`
	FileBytes  int64  `json:"file_bytes"`
	FreeBytes  int64  `json:"free_bytes"`
	Error      string `json:"error,omitempty"`
}

// Tree is one b-tree, a table or an index.
type Tree struct {
	Name string `json:"name"`
	// Type is "table" or "index"; Table is the table an index belongs to
	// (the table's own name for a table).
	Type  string `json:"type"`
	Table string `json:"table"`
	// Complete says the walk finished: the numbers are the tree's. Partial
	// says it was cut: the numbers are a floor, the tree's leftmost part.
	// Neither says the reading never reached it.
	Complete bool `json:"complete"`
	Partial  bool `json:"partial,omitempty"`
	// Bytes is every page of the tree (leaf, internal and overflow) at the
	// page size.
	Bytes         int64 `json:"bytes"`
	LeafPages     int64 `json:"leaf_pages"`
	InternalPages int64 `json:"internal_pages"`
	OverflowPages int64 `json:"overflow_pages"`
	// Rows is a table's row count (its leaf cells) or an index's entry count
	// (every cell).
	Rows int64 `json:"rows"`
	// PayloadBytes is the bytes of keys and values the cells hold, and
	// UnusedBytes the free space left inside the pages.
	PayloadBytes int64 `json:"payload_bytes"`
	UnusedBytes  int64 `json:"unused_bytes"`
	// RowidMin and RowidMax are a rowid table's smallest and largest rowid
	// (two b-tree edge reads); absent for an index and a WITHOUT ROWID table.
	RowidMin  *int64 `json:"rowid_min,omitempty"`
	RowidMax  *int64 `json:"rowid_max,omitempty"`
	ElapsedMs int64  `json:"elapsed_ms"`
	Error     string `json:"error,omitempty"`

	// keyed says every cell of the tree, interior ones too, is a row: an
	// index, or the primary b-tree of a WITHOUT ROWID table.
	keyed bool
}

// TableTotal is one table with every index charged to it.
type TableTotal struct {
	Table      string `json:"table"`
	Bytes      int64  `json:"bytes"`
	TableBytes int64  `json:"table_bytes"`
	IndexBytes int64  `json:"index_bytes"`
	Indexes    int    `json:"indexes"`
	Rows       int64  `json:"rows"`
	// Complete says every tree of the table finished.
	Complete bool `json:"complete"`
}

// Reading is one reading of one file.
type Reading struct {
	ReadAt        time.Time `json:"read_at"`
	ElapsedMillis int64     `json:"elapsed_ms"`
	BudgetMillis  int64     `json:"budget_ms"`
	// Truncated says a tree was cut or never reached: some numbers are floors.
	Truncated bool `json:"truncated"`
	// Only, Unknown and OnlyDropped describe a restricted reading: the names
	// asked for (deduplicated, at most MaxOnly), those that match no tree,
	// and how many past the cap were ignored. Only a restricted reading's
	// Trees are the named ones; a table total is Complete only when every
	// tree of the table, named or not, was read.
	// UnattributedBytes is file_bytes - free_bytes - the bytes of the trees
	// read: pages no tree in this reading accounts for. It is not attributed
	// to anything here. When exactly one tree was cut, it is that tree's
	// missing bytes; in a restricted reading it is the trees not named plus
	// any cut one. It also holds pages dbstat does not report: the pointer-map
	// pages of an auto_vacuum file and the lock-byte page. (sqlite_schema is
	// left out by readSchema, not by dbstat.) Never negative.
	UnattributedBytes int64 `json:"unattributed_bytes"`

	Only        []string     `json:"only,omitempty"`
	Unknown     []string     `json:"unknown,omitempty"`
	OnlyDropped int          `json:"only_dropped,omitempty"`
	Header      Header       `json:"header"`
	Trees       []Tree       `json:"trees"`
	Tables      []TableTotal `json:"tables"`
}

// Report is the store_tables section of guest observability.
type Report struct {
	// Reading is the last reading: this request's, or, when Busy, the one
	// before the reading that was running; absent before any reading.
	Reading *Reading `json:"reading,omitempty"`
	// Busy says another reading was running, so this request did none.
	Busy bool `json:"busy,omitempty"`
	// Skipped says why no reading was taken though one was asked for.
	Skipped string `json:"skipped,omitempty"`
}

// Scanner runs readings one at a time and keeps the last. The zero value
// works.
type Scanner struct {
	mu      sync.Mutex
	running bool
	last    *Reading

	// testHook runs before a tree is read with the share it would get, and
	// returns the share to use.
	testHook func(name string, share time.Duration) time.Duration
}

// Read reports the file db holds. With want it reads, unless a reading is
// already running, in which case it answers Busy with the last one. Without
// want it only attaches the last reading, if there is one.
func (s *Scanner) Read(ctx context.Context, db *sql.DB, lim Limits, want bool) Report {
	s.mu.Lock()
	rep := Report{Reading: s.last}
	if !want {
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

	var r *Reading
	defer func() {
		s.mu.Lock()
		s.running = false
		// A reading cut by its request's own cancellation does not replace
		// one that others may read.
		// A restricted reading (decided from the request, not from the
		// reading, which an early failure leaves unmarked) is never kept: a
		// later plain read must not show one tree's numbers as the file's.
		// A reading that failed (a header error, no trees) never replaces a
		// good one either.
		failed := r != nil && r.Header.Error != "" && len(r.Trees) == 0
		if r != nil && len(lim.Only) == 0 && (ctx.Err() == nil || s.last == nil) && (!failed || s.last == nil) {
			s.last = r
		}
		s.mu.Unlock()
	}()
	r = s.read(ctx, db, clampBudget(lim.Budget), lim.Only)
	rep.Reading = r
	return rep
}

type schemaTree struct {
	name, typ, table string
	rowid            bool
}

func (s *Scanner) read(ctx context.Context, db *sql.DB, budget time.Duration, only []string) *Reading {
	started := time.Now()
	r := &Reading{ReadAt: started.UTC(), BudgetMillis: budget.Milliseconds(), Trees: []Tree{}, Tables: []TableTotal{}}
	if db == nil {
		r.Header.Error = "no database"
		r.Truncated = true
		return r
	}
	// The header and the schema are a few page reads and are taken even by a
	// cancelled request: the answer then says what was left unread.
	fixed, cancel := context.WithTimeout(context.WithoutCancel(ctx), headerBudget)
	r.Header = readHeader(fixed, db)
	list, err := readSchema(fixed, db)
	cancel()
	if err != nil {
		r.Header.Error = "schema: " + err.Error()
		r.Truncated = true
		r.ElapsedMillis = time.Since(started).Milliseconds()
		return r
	}

	deadline := started.Add(budget)
	weight := flatWeight
	treesOfTable := map[string]int{}
	for _, t := range list {
		treesOfTable[t.table]++
	}
	s.mu.Lock()
	prev := s.last
	s.mu.Unlock()
	if len(only) > 0 {
		list = selectOnly(r, list, only)
	} else if prev != nil {
		list, weight = orderByPrevious(list, prev)
	}
	remainingWeight := 0.0
	for _, t := range list {
		remainingWeight += weight(t)
	}
	for _, t := range list {
		w := weight(t)
		left := time.Until(deadline)
		share := time.Duration(float64(left) * w / remainingWeight)
		remainingWeight -= w
		if len(only) == 0 {
			share = min(share, maxTreeRead)
		}
		if s.testHook != nil {
			share = s.testHook(t.name, share)
		}
		tr := Tree{Name: t.name, Type: t.typ, Table: t.table, keyed: !t.rowid && t.typ == "table" || t.typ == "index"}
		if left > 0 && share > 0 && ctx.Err() == nil {
			walkTree(ctx, db, &tr, share)
			if t.rowid {
				readExtents(ctx, db, &tr, deadline)
			}
		}
		if !tr.Complete {
			r.Truncated = true
		}
		r.Trees = append(r.Trees, tr)
	}
	r.Tables = rollUp(r.Trees)
	if len(r.Only) > 0 {
		counts := map[string]int{}
		for _, tr := range r.Trees {
			counts[tr.Table]++
		}
		for i := range r.Tables {
			if counts[r.Tables[i].Table] < treesOfTable[r.Tables[i].Table] {
				r.Tables[i].Complete = false
			}
		}
	}
	var seen int64
	for _, tr := range r.Trees {
		seen += tr.Bytes
	}
	r.UnattributedBytes = unattributed(r.Header, seen)
	sortTrees(r.Trees)
	r.ElapsedMillis = time.Since(started).Milliseconds()
	return r
}

// selectOnly keeps the trees named, in the order named, and records the
// names and the ones that match nothing on r.
func selectOnly(r *Reading, list []schemaTree, only []string) []schemaTree {
	byName := map[string]schemaTree{}
	for _, t := range list {
		byName[t.name] = t
	}
	r.Only = []string{}
	seen := map[string]bool{}
	var out []schemaTree
	for _, n := range only {
		if n = strings.TrimSpace(n); n == "" {
			continue
		}
		if seen[n] {
			continue
		}
		if len(r.Only) >= MaxOnly {
			r.OnlyDropped++
			continue
		}
		seen[n] = true
		r.Only = append(r.Only, n)
		if t, ok := byName[n]; ok {
			out = append(out, t)
		} else {
			r.Unknown = append(r.Unknown, n)
		}
	}
	return out
}

// orderByPrevious orders trees by the bytes the last reading saw, largest
// first, and weighs each by them, so the budget goes where the bytes are. A
// tree the last reading cut counts double (its bytes are a floor); a tree it
// never saw weighs a minimum. Nothing cheaper than a walk gives a tree's
// size: dbstat has no root-only mode, and the schema holds no sizes.
func orderByPrevious(list []schemaTree, prev *Reading) ([]schemaTree, func(schemaTree) float64) {
	const minWeight = 64 << 10
	if len(prev.Trees) == 0 {
		return list, flatWeight
	}
	est := map[string]float64{}
	for _, tr := range prev.Trees {
		b := float64(tr.Bytes)
		if !tr.Complete {
			b *= 2
		}
		est[tr.Name] = b
	}
	w := func(t schemaTree) float64 { return max(est[t.name], minWeight) }
	out := append([]schemaTree(nil), list...)
	sort.SliceStable(out, func(i, j int) bool { return w(out[i]) > w(out[j]) })
	return out, w
}

func readHeader(ctx context.Context, db *sql.DB) Header {
	var h Header
	for _, p := range []struct {
		q   string
		dst *int64
	}{
		{"PRAGMA page_size", &h.PageSize},
		{"PRAGMA page_count", &h.PageCount},
		{"PRAGMA freelist_count", &h.FreelistCount},
		{"PRAGMA auto_vacuum", &h.AutoVacuum},
	} {
		if err := db.QueryRowContext(ctx, p.q).Scan(p.dst); err != nil && h.Error == "" {
			h.Error = p.q + ": " + err.Error()
		}
	}
	h.FileBytes = h.PageCount * h.PageSize
	h.FreeBytes = h.FreelistCount * h.PageSize
	return h
}

// readSchema lists the tables and indexes, tables first (they are weighted
// more, and the reading is read top to bottom).
func readSchema(ctx context.Context, db *sql.DB) ([]schemaTree, error) {
	rows, err := db.QueryContext(ctx, `SELECT type, name, tbl_name, COALESCE(sql, '') FROM sqlite_schema WHERE type IN ('table', 'index') ORDER BY type DESC, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []schemaTree
	for rows.Next() {
		var typ, name, table, ddl string
		if err := rows.Scan(&typ, &name, &table, &ddl); err != nil {
			return nil, err
		}
		out = append(out, schemaTree{
			name: name, typ: typ, table: table,
			rowid: typ == "table" && !withoutRowidRE.MatchString(ddl) && !createVirtualRE.MatchString(ddl),
		})
	}
	return out, rows.Err()
}

// walkTree reads one tree's pages through dbstat, for at most share. The
// pages seen before a cut are kept: Partial says they are a floor.
func walkTree(ctx context.Context, db *sql.DB, tr *Tree, share time.Duration) {
	started := time.Now()
	wctx, cancel := context.WithTimeout(ctx, share)
	defer cancel()
	defer func() { tr.ElapsedMs = time.Since(started).Milliseconds() }()

	rows, err := db.QueryContext(wctx, `SELECT pagetype, ncell, payload, unused, pgsize FROM dbstat WHERE name = ?`, tr.Name)
	if err != nil {
		finishWalk(wctx, tr, err)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var pageType string
		var ncell, payload, unused, pgsize int64
		if err := rows.Scan(&pageType, &ncell, &payload, &unused, &pgsize); err != nil {
			finishWalk(wctx, tr, err)
			return
		}
		tr.Bytes += pgsize
		tr.PayloadBytes += payload
		tr.UnusedBytes += unused
		switch pageType {
		case "leaf":
			tr.LeafPages++
			tr.Rows += ncell
		case "internal":
			tr.InternalPages++
			// An index's interior cells are entries, as are those of a
			// WITHOUT ROWID table; a rowid table's are only child keys.
			if tr.keyed {
				tr.Rows += ncell
			}
		case "overflow":
			tr.OverflowPages++
		}
	}
	finishWalk(wctx, tr, rows.Err())
}

func finishWalk(wctx context.Context, tr *Tree, err error) {
	switch {
	case err == nil:
		tr.Complete = true
	case wctx.Err() != nil:
		// The share ran out (or the request ended): what was seen is a floor.
		tr.Partial = true
	default:
		tr.Partial = tr.Bytes > 0
		tr.Error = err.Error()
	}
}

// readExtents reads a rowid table's smallest and largest rowid: two edge
// reads of its b-tree.
func readExtents(ctx context.Context, db *sql.DB, tr *Tree, deadline time.Time) {
	left := time.Until(deadline)
	if left <= 0 {
		return
	}
	ectx, cancel := context.WithTimeout(ctx, min(headerBudget, left))
	defer cancel()
	var lo, hi sql.NullInt64
	q := `SELECT min(rowid), max(rowid) FROM "` + strings.ReplaceAll(tr.Name, `"`, `""`) + `"`
	if err := db.QueryRowContext(ectx, q).Scan(&lo, &hi); err != nil || !lo.Valid || !hi.Valid {
		return
	}
	tr.RowidMin, tr.RowidMax = &lo.Int64, &hi.Int64
}

func rollUp(trees []Tree) []TableTotal {
	byTable := map[string]*TableTotal{}
	var order []string
	for _, tr := range trees {
		tt := byTable[tr.Table]
		if tt == nil {
			tt = &TableTotal{Table: tr.Table, Complete: true}
			byTable[tr.Table] = tt
			order = append(order, tr.Table)
		}
		tt.Bytes += tr.Bytes
		if tr.Type == "table" {
			tt.TableBytes += tr.Bytes
			tt.Rows = tr.Rows
		} else {
			tt.IndexBytes += tr.Bytes
			tt.Indexes++
		}
		tt.Complete = tt.Complete && tr.Complete
	}
	out := make([]TableTotal, 0, len(order))
	for _, name := range order {
		out = append(out, *byTable[name])
	}
	sortTotals(out)
	return out
}

// sortTrees puts the largest tree first, a name breaking ties so the order is
// stable between readings.
func sortTrees(trees []Tree) {
	sort.SliceStable(trees, func(i, j int) bool {
		if trees[i].Bytes != trees[j].Bytes {
			return trees[i].Bytes > trees[j].Bytes
		}
		return trees[i].Name < trees[j].Name
	})
}

func sortTotals(totals []TableTotal) {
	sort.SliceStable(totals, func(i, j int) bool {
		if totals[i].Bytes != totals[j].Bytes {
			return totals[i].Bytes > totals[j].Bytes
		}
		return totals[i].Table < totals[j].Table
	})
}

// unattributed is the pages no tree accounts for.
//
// The header is read first, so growth of the file during the read is not
// counted: the result is clamped at zero. It is zero when the header failed.
func unattributed(h Header, seen int64) int64 {
	if h.Error != "" {
		return 0
	}
	return max(0, h.FileBytes-h.FreeBytes-seen)
}

// flatWeight weighs a table four times an index.
func flatWeight(t schemaTree) float64 {
	if t.typ == "table" {
		return 4
	}
	return 1
}
