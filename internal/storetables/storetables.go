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
//   - one wall-time budget for the whole reading, shared out by weight: a
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

// Limits bounds one reading. Budget is the wall time of the whole reading.
type Limits struct {
	Budget time.Duration
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

// MaxTreeRead caps one tree's single read, and so the longest read
// transaction a reading holds.
const MaxTreeRead = 15 * time.Second

var maxTreeRead = MaxTreeRead

var (
	withoutRowidRE  = regexp.MustCompile(`(?i)\bWITHOUT\s+ROWID\b`)
	createVirtualRE = regexp.MustCompile(`(?i)^\s*CREATE\s+VIRTUAL\b`)
)

// headerBudget bounds the header pragmas and one tree's rowid extents, which
// are a few page reads each.
const headerBudget = 5 * time.Second

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
	Truncated bool         `json:"truncated"`
	Header    Header       `json:"header"`
	Trees     []Tree       `json:"trees"`
	Tables    []TableTotal `json:"tables"`
}

// Report is the store_tables section of guest observability.
type Report struct {
	// Reading is the last reading: this request's, or, when Busy, the one
	// before the reading that was running; absent before any reading.
	Reading *Reading `json:"reading,omitempty"`
	// Busy says another reading was running, so this request did none.
	Busy bool `json:"busy,omitempty"`
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
		if r != nil && (ctx.Err() == nil || s.last == nil) {
			s.last = r
		}
		s.mu.Unlock()
	}()
	r = s.read(ctx, db, clampBudget(lim.Budget))
	rep.Reading = r
	return rep
}

type schemaTree struct {
	name, typ, table string
	rowid            bool
}

func (s *Scanner) read(ctx context.Context, db *sql.DB, budget time.Duration) *Reading {
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
	weight := func(t schemaTree) float64 {
		if t.typ == "table" {
			return 4
		}
		return 1
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
		share = min(share, maxTreeRead)
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
	sortTrees(r.Trees)
	r.ElapsedMillis = time.Since(started).Milliseconds()
	return r
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
