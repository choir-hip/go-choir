package rowcopy

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/stdlib"

	"github.com/yusefmosiah/go-choir/internal/dbx"
)

// Options tunes Run. Zero values take the defaults.
type Options struct {
	// Tables restricts the copy to these tables (default: every table in both).
	Tables []string
	// Exclude names tables to skip; a trailing * is a prefix match. Tables
	// starting with "dolt_" are always skipped (Dolt system tables).
	Exclude []string
	// DryRun counts source rows and intersects columns; it writes nothing.
	DryRun bool
	// BatchRows and BatchBytes bound the rows held per SQLite write
	// transaction (defaults 500 rows, 8 MiB).
	BatchRows  int
	BatchBytes int
	// PageRows bounds the rows one source query returns (default 50000): a
	// table with a primary key is read in key order, page by page, each page
	// resuming after the previous page's last key, inside the same source
	// transaction. One long scan of Store A's receipts (about 6M rows)
	// stopped returning rows after 2.75M and 5.5M at C5 while Dolt spun a
	// core (2026-10-30).
	PageRows int
	// Parallel is how many tables a PostgreSQL target copies at once
	// (default 1; ignored when the tables have foreign keys among them, and
	// for SQLite, which has one writer). The source needs that many
	// connections.
	Parallel int
	// LenientFilter makes a Tables entry that is not in both databases a
	// no-op instead of an error (for callers that run one filter against
	// several database pairs and check the union themselves).
	LenientFilter bool
	// Logf receives progress lines (default: discarded).
	Logf func(format string, args ...any)

	// beforeVerify is a test hook, called after a table is copied and before
	// it is verified.
	beforeVerify func(ctx context.Context, table string)
}

// TableReport is the outcome for one table.
type TableReport struct {
	Name              string           `json:"name"`
	Columns           []string         `json:"columns,omitempty"`
	SourceOnlyColumns []string         `json:"source_only_columns,omitempty"`
	TargetOnlyColumns []string         `json:"target_only_columns,omitempty"`
	SourceRows        int64            `json:"source_rows"`
	TargetRows        int64            `json:"target_rows"`
	SourceDigest      string           `json:"source_digest,omitempty"`
	TargetDigest      string           `json:"target_digest,omitempty"`
	NullCoerced       map[string]int64 `json:"null_coerced,omitempty"`
	ZeroDates         map[string]int64 `json:"zero_dates,omitempty"`
	// BadText (dry run, PostgreSQL target): per column and reason, the rows
	// whose text PostgreSQL cannot store (NUL byte, invalid UTF-8), with up
	// to five sample primary keys. A real run fails on the first such row.
	BadText        map[string]int64 `json:"bad_text,omitempty"`
	BadTextRows    int64            `json:"bad_text_rows,omitempty"`
	BadTextSamples []string         `json:"bad_text_samples,omitempty"`
	DurationMS     int64            `json:"duration_ms"`
	OK             bool             `json:"ok"`
	Error          string           `json:"error,omitempty"`
}

// Report is the outcome of a Run.
type Report struct {
	Target           string        `json:"target"`
	DryRun           bool          `json:"dry_run,omitempty"`
	StartedAt        time.Time     `json:"started_at"`
	FinishedAt       time.Time     `json:"finished_at"`
	Tables           []TableReport `json:"tables"`
	SourceOnlyTables []string      `json:"source_only_tables,omitempty"`
	TargetOnlyTables []string      `json:"target_only_tables,omitempty"`
	TotalRows        int64         `json:"total_rows"`
	OK               bool          `json:"ok"`
	Error            string        `json:"error,omitempty"`
}

// Failed lists the tables that did not verify.
func (r *Report) Failed() []string {
	var out []string
	for _, t := range r.Tables {
		if !t.OK {
			out = append(out, t.Name)
		}
	}
	return out
}

type runner struct {
	src  *sql.DB
	dst  *sql.DB
	kind dbx.Kind
	opts Options
	// hasFKs is set when the PostgreSQL tables in scope reference each other.
	hasFKs bool
}

// Run copies every table present in both src (a MySQL-protocol database) and
// dst (SQLite through dbx.SQLiteDB.Write, or PostgreSQL), then verifies each
// table. The target schema must exist. Each target table is emptied first, so
// a rerun starts clean. On the first table that fails, Run stops and returns
// the report so far with the error.
func Run(ctx context.Context, src, dst *sql.DB, kind dbx.Kind, opts Options) (*Report, error) {
	if opts.BatchRows <= 0 {
		opts.BatchRows = 500
	}
	if opts.BatchBytes <= 0 {
		opts.BatchBytes = 8 << 20
	}
	if opts.PageRows <= 0 {
		opts.PageRows = 50000
	}
	if opts.Logf == nil {
		opts.Logf = func(string, ...any) {}
	}
	r := &runner{src: src, dst: dst, kind: kind, opts: opts}
	rep := &Report{Target: string(kind), DryRun: opts.DryRun, StartedAt: time.Now().UTC()}
	err := r.run(ctx, rep)
	rep.FinishedAt = time.Now().UTC()
	for _, t := range rep.Tables {
		rep.TotalRows += t.TargetRows
	}
	if err != nil {
		rep.Error = err.Error()
		return rep, err
	}
	rep.OK = true
	return rep, nil
}

func (r *runner) excluded(name string) bool {
	if strings.HasPrefix(strings.ToLower(name), "dolt_") {
		return true
	}
	for _, e := range r.opts.Exclude {
		if p, ok := strings.CutSuffix(e, "*"); ok {
			if strings.HasPrefix(name, p) {
				return true
			}
		} else if e == name {
			return true
		}
	}
	return false
}

func (r *runner) run(ctx context.Context, rep *Report) error {
	srcTables, err := introspectSource(ctx, r.src)
	if err != nil {
		return err
	}
	dstTables, err := introspectTarget(ctx, r.dst, r.kind)
	if err != nil {
		return err
	}
	filter := map[string]bool{}
	for _, t := range r.opts.Tables {
		filter[strings.TrimSpace(t)] = true
	}
	var pairs []pair
	for name, d := range dstTables {
		if r.excluded(name) {
			continue
		}
		s, ok := srcTables[name]
		if !ok {
			rep.TargetOnlyTables = append(rep.TargetOnlyTables, name)
			continue
		}
		if len(filter) > 0 && !filter[name] {
			continue
		}
		if !identRe.MatchString(name) {
			return fmt.Errorf("rowcopy: unsupported table name %q", name)
		}
		p := intersect(s, d)
		if len(p.SrcCols) == 0 {
			rep.TargetOnlyTables = append(rep.TargetOnlyTables, name)
			continue
		}
		pairs = append(pairs, p)
	}
	for name := range srcTables {
		if _, ok := dstTables[name]; !ok && !r.excluded(name) {
			rep.SourceOnlyTables = append(rep.SourceOnlyTables, name)
		}
	}
	sort.Strings(rep.SourceOnlyTables)
	sort.Strings(rep.TargetOnlyTables)
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].Name < pairs[j].Name })
	for t := range filter {
		found := false
		for _, p := range pairs {
			if p.Name == t {
				found = true
			}
		}
		if !found && !r.opts.LenientFilter {
			return fmt.Errorf("rowcopy: requested table %q is not in both source and target", t)
		}
	}

	if r.opts.DryRun {
		var badErr error
		for _, p := range pairs {
			tr := TableReport{Name: p.Name, SourceOnlyColumns: p.SrcOnly, TargetOnlyColumns: p.DstOnly}
			for _, c := range p.DstCols {
				tr.Columns = append(tr.Columns, c.Name)
			}
			n, err := r.countSource(ctx, r.src, p.Name)
			if err != nil {
				tr.Error = err.Error()
				rep.Tables = append(rep.Tables, tr)
				return err
			}
			tr.SourceRows, tr.OK = n, true
			if r.kind == dbx.Postgres {
				if err := r.scanText(ctx, p, &tr); err != nil {
					tr.OK, tr.Error = false, err.Error()
					rep.Tables = append(rep.Tables, tr)
					return err
				}
				if tr.BadTextRows > 0 {
					tr.OK = false
					tr.Error = fmt.Sprintf("%d rows hold text PostgreSQL cannot store (%v)", tr.BadTextRows, tr.BadText)
					badErr = errors.Join(badErr, fmt.Errorf("table %s: %s", p.Name, tr.Error))
				}
			}
			rep.Tables = append(rep.Tables, tr)
		}
		return badErr
	}

	names := make([]string, len(pairs))
	for i, p := range pairs {
		names[i] = p.Name
	}
	switch r.kind {
	case dbx.SQLite:
		return r.runSQLite(ctx, rep, pairs)
	case dbx.Postgres:
		order, hasFKs, err := pgForeignOrder(ctx, r.dst, names)
		if err != nil {
			return err
		}
		r.hasFKs = hasFKs
		byName := map[string]pair{}
		for _, p := range pairs {
			byName[p.Name] = p
		}
		ordered := make([]pair, 0, len(pairs))
		for _, n := range order {
			ordered = append(ordered, byName[n])
		}
		return r.runPostgres(ctx, rep, ordered)
	}
	return fmt.Errorf("rowcopy: unsupported target kind %q", r.kind)
}

type srcQueryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// beginSource opens the read-only REPEATABLE READ transaction that one
// table's count, rows and source digest all come from, so they agree even
// while writers continue on a live source. It never writes.
func (r *runner) beginSource(ctx context.Context) (*sql.Tx, error) {
	// The MySQL protocol supports both options. The embedded Dolt driver
	// (the guest migrator, which reads a stopped workspace nobody writes)
	// rejects an isolation level and may reject read-only; fall back through
	// weaker option sets rather than fail.
	var firstErr error
	for _, o := range []*sql.TxOptions{
		{Isolation: sql.LevelRepeatableRead, ReadOnly: true},
		{ReadOnly: true},
		nil,
	} {
		tx, err := r.src.BeginTx(ctx, o)
		if err == nil {
			return tx, nil
		}
		if firstErr == nil {
			firstErr = err
		}
	}
	return nil, fmt.Errorf("rowcopy: begin source transaction: %w", firstErr)
}

func (r *runner) countSource(ctx context.Context, q srcQueryer, name string) (int64, error) {
	var n int64
	err := q.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+quoteMySQL(name)).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("rowcopy: count source %s: %w", name, err)
	}
	return n, nil
}

// sourceQuery selects the intersected columns in source primary-key order.
func sourceQuery(p pair) string { return sourceSelect(p) + sourceOrder(p) }

func sourceSelect(p pair) string {
	cols := make([]string, len(p.SrcCols))
	for i, c := range p.SrcCols {
		cols[i] = quoteMySQL(c.Name)
	}
	return "SELECT " + strings.Join(cols, ", ") + " FROM " + quoteMySQL(p.Name)
}

func sourceOrder(p pair) string {
	if len(p.PK) == 0 {
		return ""
	}
	pk := make([]string, len(p.PK))
	for i, c := range p.PK {
		pk[i] = quoteMySQL(c)
	}
	return " ORDER BY " + strings.Join(pk, ", ")
}

func targetQuery(p pair) string {
	cols := make([]string, len(p.DstCols))
	for i, c := range p.DstCols {
		cols[i] = quoteANSI(c.Name)
	}
	return "SELECT " + strings.Join(cols, ", ") + " FROM " + quoteANSI(p.Name)
}

// rowIter streams converted rows from the source, hashing what it yields.
type rowIter struct {
	rows    *sql.Rows
	p       pair
	pg      bool
	dig     digest
	raw     []any
	ptrs    []any
	n       int64
	coerced map[string]int64
	zero    map[string]int64
	err     error
	logf    func(string, ...any)
	start   time.Time
	// pkIdx indexes the source primary-key columns within the selected
	// columns, for error messages.
	pkIdx []int
	// Paged reads (page > 0): the queryer, the page size, the rows read
	// in the current page, and the last row's primary key.
	ctx    context.Context
	q      srcQueryer
	page   int
	inPage int
	last   []any
	// tolerant (dry run) counts text PostgreSQL cannot store instead of
	// failing on the first.
	tolerant   bool
	bad        map[string]int64
	badRows    int64
	badSamples []string
}

func (r *runner) openIter(ctx context.Context, q srcQueryer, p pair) (*rowIter, error) {
	it := &rowIter{p: p, pg: r.kind == dbx.Postgres,
		raw: make([]any, len(p.SrcCols)), ptrs: make([]any, len(p.SrcCols)),
		coerced: map[string]int64{}, zero: map[string]int64{}, logf: r.opts.Logf, start: time.Now()}
	for i := range it.raw {
		it.ptrs[i] = &it.raw[i]
	}
	it.bad = map[string]int64{}
	for _, k := range p.PK {
		for i, c := range p.SrcCols {
			if strings.EqualFold(c.Name, k) {
				it.pkIdx = append(it.pkIdx, i)
			}
		}
	}
	if len(p.PK) > 0 && len(it.pkIdx) == len(p.PK) {
		it.ctx, it.q, it.page = ctx, q, r.opts.PageRows
	}
	if err := it.query(ctx, q); err != nil {
		return nil, err
	}
	return it, nil
}

// query opens the next source query: the whole table, or (paged) the next
// page of up to it.page rows after it.last in primary-key order.
func (it *rowIter) query(ctx context.Context, q srcQueryer) error {
	sq, args := sourceQuery(it.p), []any(nil)
	if it.page > 0 {
		where := ""
		if it.last != nil {
			// (k1 > ?) OR (k1 = ? AND k2 > ?) OR ...: a form Dolt plans as
			// index ranges, unlike a row-value comparison.
			var ors []string
			for i := range it.p.PK {
				var ands []string
				for j := 0; j < i; j++ {
					ands = append(ands, quoteMySQL(it.p.PK[j])+" = ?")
					args = append(args, it.last[j])
				}
				ands = append(ands, quoteMySQL(it.p.PK[i])+" > ?")
				args = append(args, it.last[i])
				ors = append(ors, "("+strings.Join(ands, " AND ")+")")
			}
			where = " WHERE " + strings.Join(ors, " OR ")
		}
		sq = sourceSelect(it.p) + where + sourceOrder(it.p) + fmt.Sprintf(" LIMIT %d", it.page)
		it.inPage = 0
	}
	rows, err := q.QueryContext(ctx, sq, args...)
	if err != nil {
		return fmt.Errorf("rowcopy: read source %s: %w", it.p.Name, err)
	}
	it.rows = rows
	return nil
}

// next returns the next converted row, or nil at the end or on error (see err).
func (it *rowIter) next() []any {
	for {
		out, skipped := it.nextRow()
		if skipped {
			continue
		}
		return out
	}
}

// pkLabel names the current row's primary key for error messages.
func (it *rowIter) pkLabel() string {
	if len(it.pkIdx) == 0 {
		return fmt.Sprintf("row %d", it.n+1)
	}
	parts := make([]string, len(it.pkIdx))
	for i, ix := range it.pkIdx {
		s, _ := toText(it.raw[ix])
		parts[i] = clip(s)
	}
	return "pk=(" + strings.Join(parts, ", ") + ")"
}

func (it *rowIter) nextRow() (out []any, skipped bool) {
	if it.err != nil {
		return nil, false
	}
	if !it.rows.Next() {
		if it.err = it.rows.Err(); it.err != nil || it.page == 0 || it.inPage < it.page {
			return nil, false
		}
		// A full page: read the next one.
		_ = it.rows.Close()
		if it.err = it.query(it.ctx, it.q); it.err != nil || !it.rows.Next() {
			if it.err == nil {
				it.err = it.rows.Err()
			}
			return nil, false
		}
	}
	if err := it.rows.Scan(it.ptrs...); err != nil {
		it.err = fmt.Errorf("rowcopy: scan source %s row %d: %w", it.p.Name, it.n+1, err)
		return nil, false
	}
	if it.page > 0 {
		it.inPage++
		if it.last == nil {
			it.last = make([]any, len(it.pkIdx))
		}
		for i, ix := range it.pkIdx {
			// Bind text keys as strings so the comparison uses the
			// column's collation, the one ORDER BY used.
			if b, ok := it.raw[ix].([]byte); ok {
				it.last[i] = string(b)
			} else {
				it.last[i] = it.raw[ix]
			}
		}
	}
	out = make([]any, len(it.raw))
	it.dig.beginRow()
	for i, v := range it.raw {
		dc := it.p.DstCols[i]
		cv, res, err := convert(v, dc.Class, dc.NotNull, it.pg)
		if err != nil {
			var bt *badTextError
			if it.tolerant && errors.As(err, &bt) {
				// Dry run: count it and move on to the next row.
				it.bad[dc.Name+": "+bt.reason]++
				it.badRows++
				if len(it.badSamples) < 5 {
					it.badSamples = append(it.badSamples, dc.Name+" "+bt.reason+" at "+it.pkLabel())
				}
				it.n++
				return nil, true
			}
			it.err = fmt.Errorf("rowcopy: %s.%s %s: %w", it.p.Name, dc.Name, it.pkLabel(), err)
			return nil, false
		}
		if res.coerced {
			it.coerced[dc.Name]++
		}
		if res.zeroDate {
			it.zero[dc.Name]++
		}
		if err := it.dig.col(dc.Class, cv); err != nil {
			it.err = fmt.Errorf("rowcopy: %s.%s %s: %w", it.p.Name, dc.Name, it.pkLabel(), err)
			return nil, false
		}
		out[i] = cv
	}
	it.dig.endRow()
	it.n++
	if it.n%250000 == 0 {
		it.logf("rowcopy: %s: %d rows so far (%s)", it.p.Name, it.n, time.Since(it.start).Round(time.Second))
	}
	return out, false
}

func (it *rowIter) close() { _ = it.rows.Close() }

func tableReport(p pair) TableReport {
	tr := TableReport{Name: p.Name, SourceOnlyColumns: p.SrcOnly, TargetOnlyColumns: p.DstOnly}
	for _, c := range p.DstCols {
		tr.Columns = append(tr.Columns, c.Name)
	}
	return tr
}

// finish records the digests and decides OK.
func finishTable(tr *TableReport, srcCount int64, src, dst *digest, it *rowIter) {
	tr.SourceRows, tr.TargetRows = src.count, dst.count
	tr.SourceDigest, tr.TargetDigest = src.Hex(), dst.Hex()
	if len(it.coerced) > 0 {
		tr.NullCoerced = it.coerced
	}
	if len(it.zero) > 0 {
		tr.ZeroDates = it.zero
	}
	switch {
	case src.count != srcCount:
		tr.Error = fmt.Sprintf("source changed during copy: counted %d rows before, read %d", srcCount, src.count)
	case dst.count != src.count:
		tr.Error = fmt.Sprintf("row count mismatch: source %d, target %d", src.count, dst.count)
	case src.Hex() != dst.Hex():
		tr.Error = fmt.Sprintf("digest mismatch: source %s, target %s", src.Hex(), dst.Hex())
	default:
		tr.OK = true
	}
}

// readBack digests the target table as it now stands.
func readBack(ctx context.Context, q interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}, p pair) (*digest, error) {
	rows, err := q.QueryContext(ctx, targetQuery(p))
	if err != nil {
		return nil, fmt.Errorf("rowcopy: read back %s: %w", p.Name, err)
	}
	defer rows.Close()
	d := &digest{}
	vals := make([]any, len(p.DstCols))
	ptrs := make([]any, len(vals))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	for rows.Next() {
		if err := rows.Scan(ptrs...); err != nil {
			return nil, fmt.Errorf("rowcopy: read back %s: %w", p.Name, err)
		}
		d.beginRow()
		for i, v := range vals {
			if err := d.col(p.DstCols[i].Class, v); err != nil {
				return nil, fmt.Errorf("rowcopy: read back %s.%s: %w", p.Name, p.DstCols[i].Name, err)
			}
		}
		d.endRow()
	}
	return d, rows.Err()
}

// ---- SQLite target ----

func (r *runner) runSQLite(ctx context.Context, rep *Report, pairs []pair) (err error) {
	conn, err := r.dst.Conn(ctx)
	if err != nil {
		return fmt.Errorf("rowcopy: sqlite connection: %w", err)
	}
	defer conn.Close()
	// Speed: the target is a file nobody reads until the copy verifies; the
	// caller syncs it before use. Foreign keys are checked once at the end.
	for _, pragma := range []string{"PRAGMA synchronous = OFF", "PRAGMA foreign_keys = OFF"} {
		if _, err := conn.ExecContext(ctx, pragma); err != nil {
			return fmt.Errorf("rowcopy: %s: %w", pragma, err)
		}
	}
	defer func() {
		_, _ = conn.ExecContext(context.Background(), "PRAGMA synchronous = FULL")
		_, _ = conn.ExecContext(context.Background(), "PRAGMA foreign_keys = ON")
	}()
	for _, p := range pairs {
		if _, err := conn.ExecContext(ctx, "DELETE FROM "+quoteANSI(p.Name)); err != nil {
			return fmt.Errorf("rowcopy: empty target %s: %w", p.Name, err)
		}
	}
	for _, p := range pairs {
		start := time.Now()
		tr := tableReport(p)
		err := r.copySQLiteTable(ctx, conn, p, &tr)
		tr.DurationMS = time.Since(start).Milliseconds()
		if err != nil && tr.Error == "" {
			tr.Error = err.Error()
		}
		rep.Tables = append(rep.Tables, tr)
		r.opts.Logf("rowcopy: %s: %d rows ok=%v", p.Name, tr.TargetRows, tr.OK)
		if err != nil {
			return err
		}
	}
	rows, err := conn.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		return fmt.Errorf("rowcopy: foreign_key_check: %w", err)
	}
	violations := 0
	for rows.Next() {
		violations++
	}
	rows.Close()
	if violations > 0 {
		return fmt.Errorf("rowcopy: foreign_key_check found %d violations in the copied data", violations)
	}
	if _, err := conn.ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		return fmt.Errorf("rowcopy: checkpoint: %w", err)
	}
	return nil
}

func (r *runner) copySQLiteTable(ctx context.Context, conn *sql.Conn, p pair, tr *TableReport) error {
	stx, err := r.beginSource(ctx)
	if err != nil {
		return err
	}
	defer stx.Rollback()
	srcCount, err := r.countSource(ctx, stx, p.Name)
	if err != nil {
		return err
	}
	it, err := r.openIter(ctx, stx, p)
	if err != nil {
		return err
	}
	defer it.close()

	ncols := len(p.DstCols)
	perStmt := r.opts.BatchRows
	if max := 30000 / ncols; perStmt > max {
		perStmt = max
	}
	if perStmt < 1 {
		perStmt = 1
	}
	cols := make([]string, ncols)
	for i, c := range p.DstCols {
		cols[i] = quoteANSI(c.Name)
	}
	head := "INSERT INTO " + quoteANSI(p.Name) + " (" + strings.Join(cols, ", ") + ") VALUES "
	tuple := "(" + strings.TrimSuffix(strings.Repeat("?, ", ncols), ", ") + ")"
	build := func(n int) string {
		return head + strings.TrimSuffix(strings.Repeat(tuple+", ", n), ", ")
	}
	fullSQL := build(perStmt)
	full, err := conn.PrepareContext(ctx, fullSQL)
	if err != nil {
		return fmt.Errorf("rowcopy: prepare insert %s: %w", p.Name, err)
	}
	defer full.Close()

	var args []any
	var size int
	var pending int
	flush := func() error {
		if pending == 0 {
			return nil
		}
		tx, err := conn.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("rowcopy: begin %s: %w", p.Name, err)
		}
		// Rows are executed perStmt at a time; the last statement may be short.
		for off := 0; off < pending; {
			n := pending - off
			if n > perStmt {
				n = perStmt
			}
			slice := args[off*ncols : (off+n)*ncols]
			if n == perStmt {
				_, err = tx.StmtContext(ctx, full).ExecContext(ctx, slice...)
			} else {
				_, err = tx.ExecContext(ctx, build(n), slice...)
			}
			if err != nil {
				_ = tx.Rollback()
				return fmt.Errorf("rowcopy: insert into %s: %w", p.Name, err)
			}
			off += n
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("rowcopy: commit %s: %w", p.Name, err)
		}
		args, size, pending = args[:0], 0, 0
		return nil
	}
	for {
		row := it.next()
		if row == nil {
			break
		}
		args = append(args, row...)
		size += rowSize(row)
		pending++
		if pending >= r.opts.BatchRows || size >= r.opts.BatchBytes {
			if err := flush(); err != nil {
				return err
			}
		}
	}
	if it.err != nil {
		return it.err
	}
	if err := flush(); err != nil {
		return err
	}
	it.close()
	if h := r.opts.beforeVerify; h != nil {
		h(ctx, p.Name)
	}
	back, err := readBack(ctx, conn, p)
	if err != nil {
		return err
	}
	finishTable(tr, srcCount, &it.dig, back, it)
	if !tr.OK {
		return errors.New(tr.Error)
	}
	return nil
}

func rowSize(row []any) int {
	n := 16
	for _, v := range row {
		switch x := v.(type) {
		case string:
			n += len(x)
		case []byte:
			n += len(x)
		default:
			n += 8
		}
	}
	return n
}

// ---- PostgreSQL target ----

func (r *runner) runPostgres(ctx context.Context, rep *Report, pairs []pair) error {
	if len(pairs) == 0 {
		return nil
	}
	idents := make([]string, len(pairs))
	for i, p := range pairs {
		idents[i] = quoteANSI(p.Name)
	}
	if _, err := r.dst.ExecContext(ctx, "TRUNCATE TABLE "+strings.Join(idents, ", ")); err != nil {
		return fmt.Errorf("rowcopy: truncate targets: %w", err)
	}
	// Tables run Parallel at a time; with foreign keys among them, serially in
	// dependency order (pairs arrive ordered).
	par := r.opts.Parallel
	if par < 1 || r.hasFKs {
		par = 1
	}
	results := make([]TableReport, len(pairs))
	done := make([]bool, len(pairs))
	var mu sync.Mutex
	var firstErr error
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	sem := make(chan struct{}, par)
	var wg sync.WaitGroup
	for i, p := range pairs {
		sem <- struct{}{}
		mu.Lock()
		failed := firstErr != nil
		mu.Unlock()
		if failed {
			<-sem
			break
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			start := time.Now()
			tr := tableReport(p)
			err := r.copyPostgresTable(runCtx, p, &tr)
			tr.DurationMS = time.Since(start).Milliseconds()
			if err != nil && tr.Error == "" {
				tr.Error = err.Error()
			}
			r.opts.Logf("rowcopy: %s: %d rows in %s ok=%v", p.Name, tr.TargetRows, time.Since(start).Round(time.Millisecond), tr.OK)
			mu.Lock()
			results[i], done[i] = tr, true
			if err != nil && firstErr == nil {
				firstErr = fmt.Errorf("rowcopy: table %s: %w", p.Name, err)
				cancel()
			}
			mu.Unlock()
		}()
	}
	wg.Wait()
	for i := range results {
		if done[i] {
			rep.Tables = append(rep.Tables, results[i])
		}
	}
	return firstErr
}

// copySource adapts rowIter to pgx.CopyFromSource (true streaming: pgx pulls
// rows as it writes the COPY stream).
type copySource struct {
	it  *rowIter
	cur []any
	pg  []any
}

func (s *copySource) Next() bool {
	s.cur = s.it.next()
	if s.cur == nil {
		return false
	}
	if cap(s.pg) < len(s.cur) {
		s.pg = make([]any, len(s.cur))
	}
	s.pg = s.pg[:len(s.cur)]
	for i, v := range s.cur {
		if v != nil && s.it.p.DstCols[i].Class == classNumeric {
			var n pgtype.Numeric
			if err := n.Scan(v.(string)); err != nil {
				s.it.err = fmt.Errorf("rowcopy: %s.%s row %d: numeric %q: %w", s.it.p.Name, s.it.p.DstCols[i].Name, s.it.n, clip(v.(string)), err)
				return false
			}
			v = n
		}
		s.pg[i] = v
	}
	return true
}
func (s *copySource) Values() ([]any, error) { return s.pg, nil }
func (s *copySource) Err() error             { return s.it.err }

func (r *runner) copyPostgresTable(ctx context.Context, p pair, tr *TableReport) error {
	stx, err := r.beginSource(ctx)
	if err != nil {
		return err
	}
	defer stx.Rollback()
	srcCount, err := r.countSource(ctx, stx, p.Name)
	if err != nil {
		return err
	}
	it, err := r.openIter(ctx, stx, p)
	if err != nil {
		return err
	}
	defer it.close()
	names := make([]string, len(p.DstCols))
	for i, c := range p.DstCols {
		names[i] = c.Name
	}
	conn, err := r.dst.Conn(ctx)
	if err != nil {
		return fmt.Errorf("rowcopy: postgres connection: %w", err)
	}
	defer conn.Close()
	err = conn.Raw(func(dc any) error {
		pc, err := pgxConn(dc)
		if err != nil {
			return err
		}
		_, err = pc.CopyFrom(ctx, pgx.Identifier{p.Name}, names, &copySource{it: it})
		return err
	})
	if it.err != nil {
		return it.err
	}
	if err != nil {
		return fmt.Errorf("rowcopy: copy into %s: %w", p.Name, err)
	}
	it.close()
	if h := r.opts.beforeVerify; h != nil {
		h(ctx, p.Name)
	}
	back, err := readBack(ctx, conn, p)
	if err != nil {
		return err
	}
	finishTable(tr, srcCount, &it.dig, back, it)
	if !tr.OK {
		return errors.New(tr.Error)
	}
	// Fresh statistics for the planner; the load is the table's first.
	if _, err := r.dst.ExecContext(ctx, "ANALYZE "+quoteANSI(p.Name)); err != nil {
		r.opts.Logf("rowcopy: %s: ANALYZE failed (not fatal): %v", p.Name, err)
	}
	return nil
}

// scanText streams the table through the conversion without writing, and
// counts rows PostgreSQL could not store (dry run). Tables with no text or
// JSON column are skipped.
func (r *runner) scanText(ctx context.Context, p pair, tr *TableReport) error {
	has := false
	for _, c := range p.DstCols {
		if c.Class == classText || c.Class == classChar || c.Class == classJSON {
			has = true
		}
	}
	if !has {
		return nil
	}
	stx, err := r.beginSource(ctx)
	if err != nil {
		return err
	}
	defer stx.Rollback()
	it, err := r.openIter(ctx, stx, p)
	if err != nil {
		return err
	}
	defer it.close()
	it.tolerant = true
	for it.next() != nil {
	}
	if it.err != nil {
		return it.err
	}
	if it.badRows > 0 {
		tr.BadText, tr.BadTextRows, tr.BadTextSamples = it.bad, it.badRows, it.badSamples
	}
	return nil
}

// pgxConn finds the *pgx.Conn under a database/sql driver connection, through
// dbx's rebinding wrapper if present.
func pgxConn(dc any) (*pgx.Conn, error) {
	if w, ok := dc.(interface{ Raw() driver.Conn }); ok {
		dc = w.Raw()
	}
	if sc, ok := dc.(*stdlib.Conn); ok {
		return sc.Conn(), nil
	}
	return nil, fmt.Errorf("rowcopy: target is not a pgx connection (%T)", dc)
}
