// Command store-split-diff is the differential of the store header/body
// split (docs/design/store-header-body-split-2026-11-01.md section 8): it
// compares a pre-split store file (state.db.presplit) with the migrated file
// (state.db) and proves the migrated state answers like the old state.
//
//	store-split-diff --old state.db.presplit --new state.db [--as-of RFC3339] [--days 30]
//
// It is read-only (both files open read-only) and memory-bounded: every
// check is a SQL aggregate or a streaming digest, never a materialized
// table, so it runs on the paper's ~21 GB store copy (run it under nice and
// ionice). It prints one JSON report on stdout and exits 1 on any
// difference, 2 on a usage or open error.
//
// Checks:
//
//	rows        og_objects counts by (object_kind, tombstone), choir_events by
//	            event_kind; in the new file header count = body count, no
//	            orphan bodies, no missing bodies.
//	witness     the per-table witness digests (logical tables) are equal for
//	            every table; on a mismatch of og_objects or choir_events, a
//	            per-row fingerprint diff names the canonical ids.
//	derived     event_phase and og_rec_claim equal objectgraph.BodyDerivedSQL
//	            recomputed over the joined body, for every row; the old
//	            event_kind expression on the old file equals the new column;
//	            retire-selection counts (the last N days x each event kind x
//	            three phase modes) are equal.
//	keys        og_key_index is equal to the byte (row digest).
//	heads       computer_event_projection_heads is equal.
package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/yusefmosiah/go-choir/internal/computerversion"
	"github.com/yusefmosiah/go-choir/internal/dbx"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/store"
)

// Check is one result of the report.
type Check struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Detail any    `json:"detail,omitempty"`
}

// Report is the JSON printed on stdout.
type Report struct {
	Old        string    `json:"old"`
	New        string    `json:"new"`
	AsOf       time.Time `json:"as_of"`
	Days       int       `json:"days"`
	OK         bool      `json:"ok"`
	Checks     []Check   `json:"checks"`
	DurationMS int64     `json:"duration_ms"`
}

func (r *Report) add(name string, ok bool, detail any) {
	r.Checks = append(r.Checks, Check{Name: name, OK: ok, Detail: detail})
	if !ok {
		r.OK = false
	}
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("store-split-diff", flag.ContinueOnError)
	fs.SetOutput(stderr)
	oldPath := fs.String("old", "", "pre-split store file (state.db.presplit)")
	newPath := fs.String("new", "", "migrated store file (state.db)")
	asOf := fs.String("as-of", "", "end of the retire-equivalence window, RFC 3339 (default: now)")
	days := fs.Int("days", 30, "days of retire-equivalence window")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *oldPath == "" || *newPath == "" {
		fmt.Fprintln(stderr, "store-split-diff: --old and --new are required")
		return 2
	}
	end := time.Now().UTC()
	if *asOf != "" {
		t, err := time.Parse(time.RFC3339, *asOf)
		if err != nil {
			fmt.Fprintln(stderr, "store-split-diff: --as-of:", err)
			return 2
		}
		end = t.UTC()
	}
	oldDB, err := openRO(*oldPath)
	if err != nil {
		fmt.Fprintln(stderr, "store-split-diff:", err)
		return 2
	}
	defer oldDB.Close()
	newDB, err := openRO(*newPath)
	if err != nil {
		fmt.Fprintln(stderr, "store-split-diff:", err)
		return 2
	}
	defer newDB.Close()

	start := time.Now()
	report := diff(context.Background(), oldDB, newDB, end, *days)
	report.Old, report.New = *oldPath, *newPath
	report.DurationMS = time.Since(start).Milliseconds()
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(report)
	if !report.OK {
		return 1
	}
	return 0
}

func openRO(path string) (*sql.DB, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", dbx.SQLiteDSN(path, dbx.SQLiteOptions{ReadOnly: true}, true))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(2)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	return db, nil
}

func diff(ctx context.Context, oldDB, newDB *sql.DB, asOf time.Time, days int) *Report {
	r := &Report{OK: true, AsOf: asOf, Days: days}
	checkLayouts(ctx, r, oldDB, newDB)
	checkRows(ctx, r, oldDB, newDB)
	checkWitness(ctx, r, oldDB, newDB)
	checkDerived(ctx, r, oldDB, newDB)
	checkRetire(ctx, r, oldDB, newDB, asOf, days)
	for _, table := range []struct{ check, name string }{
		{"keys.og_key_index", objectgraph.DerivedKeysTable},
		{"heads.computer_event_projection_heads", "computer_event_projection_heads"},
	} {
		checkOneTable(ctx, r, oldDB, newDB, table.check, table.name)
	}
	return r
}

// ---- helpers ----

func tableExists(ctx context.Context, db *sql.DB, name string) bool {
	var n int
	_ = db.QueryRowContext(ctx, `SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?`, name).Scan(&n)
	return n > 0
}

func scalarInt(ctx context.Context, db *sql.DB, query string, args ...any) (int64, error) {
	var n int64
	err := db.QueryRowContext(ctx, query, args...).Scan(&n)
	return n, err
}

// groupCounts runs a "key columns..., count" query and returns "k1|k2" -> n.
func groupCounts(ctx context.Context, db *sql.DB, query string, args ...any) (map[string]int64, error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	out := map[string]int64{}
	vals := make([]any, len(cols))
	ptrs := make([]any, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	for rows.Next() {
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		parts := make([]string, 0, len(cols)-1)
		for _, v := range vals[:len(cols)-1] {
			parts = append(parts, fmt.Sprintf("%T:%v", v, v))
		}
		n, _ := vals[len(cols)-1].(int64)
		out[strings.Join(parts, "|")] += n
	}
	return out, rows.Err()
}

type countDiff struct {
	Key string `json:"key"`
	Old int64  `json:"old"`
	New int64  `json:"new"`
}

func diffCounts(a, b map[string]int64) []countDiff {
	var out []countDiff
	for k, v := range a {
		if b[k] != v {
			out = append(out, countDiff{k, v, b[k]})
		}
	}
	for k, v := range b {
		if _, ok := a[k]; !ok && v != 0 {
			out = append(out, countDiff{k, 0, v})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	if len(out) > 50 {
		out = out[:50]
	}
	return out
}

func compareGroups(ctx context.Context, r *Report, name string, oldDB *sql.DB, oldQ string, newDB *sql.DB, newQ string, args ...any) {
	a, err := groupCounts(ctx, oldDB, oldQ, args...)
	if err != nil {
		r.add(name, false, "old: "+err.Error())
		return
	}
	b, err := groupCounts(ctx, newDB, newQ, args...)
	if err != nil {
		r.add(name, false, "new: "+err.Error())
		return
	}
	d := diffCounts(a, b)
	if len(d) > 0 {
		r.add(name, false, d)
		return
	}
	r.add(name, true, map[string]int{"groups": len(a)})
}

// ---- layouts ----

func hasColumn(ctx context.Context, db *sql.DB, table, col string) bool {
	var n int
	_ = db.QueryRowContext(ctx, `SELECT count(*) FROM pragma_table_xinfo(?) WHERE name=?`, table, col).Scan(&n)
	return n > 0
}

func checkLayouts(ctx context.Context, r *Report, oldDB, newDB *sql.DB) {
	detail := map[string]bool{
		"old_has_og_body_column":     hasColumn(ctx, oldDB, "og_objects", "body"),
		"old_has_og_bodies":          tableExists(ctx, oldDB, "og_bodies"),
		"new_has_og_body_column":     hasColumn(ctx, newDB, "og_objects", "body"),
		"new_has_og_bodies":          tableExists(ctx, newDB, "og_bodies"),
		"new_has_choir_event_bodies": tableExists(ctx, newDB, "choir_event_bodies"),
	}
	ok := detail["old_has_og_body_column"] && !detail["old_has_og_bodies"] &&
		!detail["new_has_og_body_column"] && detail["new_has_og_bodies"] && detail["new_has_choir_event_bodies"]
	r.add("layouts", ok, detail)
}

// ---- rows ----

func checkRows(ctx context.Context, r *Report, oldDB, newDB *sql.DB) {
	compareGroups(ctx, r, "rows.og_objects_by_kind_tombstone", oldDB,
		`SELECT object_kind, tombstone, count(*) FROM og_objects GROUP BY 1, 2`, newDB,
		`SELECT object_kind, tombstone, count(*) FROM og_objects GROUP BY 1, 2`)
	compareGroups(ctx, r, "rows.choir_events_by_event_kind", oldDB,
		`SELECT event_kind, count(*) FROM choir_events GROUP BY 1`, newDB,
		`SELECT event_kind, count(*) FROM choir_events GROUP BY 1`)

	for _, p := range []struct{ name, header, body, key string }{
		{"rows.og_header_body_pairing", "og_objects", "og_bodies", "obj_id"},
		{"rows.event_header_body_pairing", "choir_events", "choir_event_bodies", "ev_id"},
	} {
		h, e1 := scalarInt(ctx, newDB, `SELECT count(*) FROM `+p.header)
		b, e2 := scalarInt(ctx, newDB, `SELECT count(*) FROM `+p.body)
		orphan, e3 := scalarInt(ctx, newDB, `SELECT count(*) FROM `+p.body+` b LEFT JOIN `+p.header+` h ON h.`+p.key+` = b.`+p.key+` WHERE h.`+p.key+` IS NULL`)
		missing, e4 := scalarInt(ctx, newDB, `SELECT count(*) FROM `+p.header+` h LEFT JOIN `+p.body+` b ON b.`+p.key+` = h.`+p.key+` WHERE b.`+p.key+` IS NULL`)
		if err := firstErr(e1, e2, e3, e4); err != nil {
			r.add(p.name, false, err.Error())
			continue
		}
		r.add(p.name, h == b && orphan == 0 && missing == 0, map[string]int64{"headers": h, "bodies": b, "orphan_bodies": orphan, "missing_bodies": missing})
	}
}

func firstErr(errs ...error) error {
	for _, e := range errs {
		if e != nil {
			return e
		}
	}
	return nil
}

// ---- witness ----

func extractor(db *sql.DB, ignore map[string]struct{}) computerversion.SQLiteStateExtractor {
	return computerversion.SQLiteStateExtractor{
		DB:            db,
		Database:      "texture",
		IgnoredTables: ignore,
		LogicalTables: store.WitnessLogicalTables(),
	}
}

func ignoreAllBut(ctx context.Context, db *sql.DB, keep string) (map[string]struct{}, error) {
	rows, err := db.QueryContext(ctx, `SELECT name FROM sqlite_master WHERE type='table'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]struct{}{}
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		if n != keep {
			out[n] = struct{}{}
		}
	}
	return out, rows.Err()
}

type tableDiff struct {
	Table string `json:"table"`
	Old   string `json:"old"`
	New   string `json:"new"`
}

func checkWitness(ctx context.Context, r *Report, oldDB, newDB *sql.DB) {
	// The derived-key index is checked on its own (keys.og_key_index); the
	// production witness ignores it too.
	ignore := map[string]struct{}{objectgraph.DerivedKeysTable: {}}
	od, err := extractor(oldDB, ignore).Digests(ctx)
	if err != nil {
		r.add("witness.tables", false, "old: "+err.Error())
		return
	}
	nd, err := extractor(newDB, ignore).Digests(ctx)
	if err != nil {
		r.add("witness.tables", false, "new: "+err.Error())
		return
	}
	var diffs []tableDiff
	seen := map[string]bool{}
	for t, d := range od.Tables {
		seen[t] = true
		if nd.Tables[t] != d {
			diffs = append(diffs, tableDiff{t, d, nd.Tables[t]})
		}
	}
	for t, d := range nd.Tables {
		if !seen[t] {
			diffs = append(diffs, tableDiff{t, "", d})
		}
	}
	sort.Slice(diffs, func(i, j int) bool { return diffs[i].Table < diffs[j].Table })
	// The migration writes store_meta.schema_version 2 (design §2); that one
	// row is the expected difference, and nothing else in store_meta may move.
	bumpOnly := storeMetaBumpOnly(ctx, oldDB, newDB)
	kept := diffs[:0]
	for _, d := range diffs {
		if d.Table == "store_meta" && bumpOnly {
			continue
		}
		kept = append(kept, d)
	}
	diffs = kept
	detail := map[string]any{
		"store_meta_schema_version_bump_only": bumpOnly,
		"tables":                              len(od.Tables),
		"old_content_root":                    od.ContentRoot,
		"new_content_root":                    nd.ContentRoot,
		"schema_hash_changes":                 od.SchemaHash != nd.SchemaHash,
		"mismatched_table_digest":             diffs,
	}
	if len(diffs) == 0 {
		r.add("witness.tables", true, detail)
		return
	}
	for _, d := range diffs {
		switch d.Table {
		case "og_objects":
			detail["og_objects_row_diff"] = rowDiff(ctx, oldDB, newDB, oldObjectsQuery, objectgraph.ObjectsWitnessQuery, 1)
		case "choir_events":
			detail["choir_events_row_diff"] = rowDiff(ctx, oldDB, newDB, oldEventsQuery, store.WitnessLogicalTables()["choir_events"].Query, 2)
		}
	}
	r.add("witness.tables", false, detail)
}

// storeMetaBumpOnly reports whether store_meta differs only in its
// schema_version row, from 1 in the old store to 2 in the new one.
func storeMetaBumpOnly(ctx context.Context, oldDB, newDB *sql.DB) bool {
	const rest = `SELECT coalesce(group_concat(key || '=' || value, char(10)), '') FROM (SELECT key, value FROM store_meta WHERE key <> 'schema_version' ORDER BY key)`
	const version = `SELECT value FROM store_meta WHERE key = 'schema_version'`
	var oldRest, newRest, oldV, newV string
	if oldDB.QueryRowContext(ctx, rest).Scan(&oldRest) != nil || newDB.QueryRowContext(ctx, rest).Scan(&newRest) != nil ||
		oldDB.QueryRowContext(ctx, version).Scan(&oldV) != nil || newDB.QueryRowContext(ctx, version).Scan(&newV) != nil {
		return false
	}
	return oldRest == newRest && oldV == "1" && newV == "2"
}

const oldObjectsQuery = `SELECT canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, metadata, created_at, updated_at, tombstone, superseded_by FROM og_objects ORDER BY canonical_id`
const oldEventsQuery = `SELECT tape_seq, op_index, canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, metadata, created_at, updated_at, tombstone, superseded_by, run_id, seq, stream_seq, trajectory_id, event_kind FROM choir_events ORDER BY tape_seq, op_index`

type fpRow struct {
	key []any  // sort key (the order of the query)
	id  string // canonical id naming the row
	fp  string
}

// fpCursor streams a query as (key, id, fingerprint) rows in query order.
// keyCols is how many leading columns form the ordering key.
type fpCursor struct {
	rows    *sql.Rows
	n       int
	keyCols int
	idCol   int
	vals    []any
	ptrs    []any
	cur     fpRow
	ok      bool
	err     error
}

func newFPCursor(ctx context.Context, db *sql.DB, query string, keyCols, idCol int) (*fpCursor, error) {
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	cols, err := rows.Columns()
	if err != nil {
		rows.Close()
		return nil, err
	}
	c := &fpCursor{rows: rows, n: len(cols), keyCols: keyCols, idCol: idCol, vals: make([]any, len(cols)), ptrs: make([]any, len(cols))}
	for i := range c.vals {
		c.ptrs[i] = &c.vals[i]
	}
	c.advance()
	return c, nil
}

func (c *fpCursor) advance() {
	if !c.rows.Next() {
		c.ok = false
		c.err = c.rows.Err()
		return
	}
	if err := c.rows.Scan(c.ptrs...); err != nil {
		c.ok, c.err = false, err
		return
	}
	h := sha256.New()
	key := make([]any, c.keyCols)
	for i, v := range c.vals {
		enc := encodeValue(v)
		var l [binary.MaxVarintLen64]byte
		h.Write(l[:binary.PutUvarint(l[:], uint64(len(enc)))])
		h.Write(enc)
		if i < c.keyCols {
			key[i] = v
		}
	}
	c.cur = fpRow{key: key, id: fmt.Sprint(c.vals[c.idCol]), fp: hex.EncodeToString(h.Sum(nil))}
	c.ok = true
}

func encodeValue(v any) []byte {
	switch x := v.(type) {
	case nil:
		return []byte{'n'}
	case int64:
		var b [9]byte
		b[0] = 'i'
		binary.BigEndian.PutUint64(b[1:], uint64(x))
		return b[:]
	case float64:
		return []byte(fmt.Sprintf("r%v", x))
	case string:
		return append([]byte{'t'}, x...)
	case []byte:
		return append([]byte{'b'}, x...)
	case time.Time:
		return append([]byte{'d'}, x.UTC().Format(time.RFC3339Nano)...)
	default:
		return []byte(fmt.Sprintf("?%T%v", v, v))
	}
}

type rowDiffEntry struct {
	ID     string `json:"id"`
	Reason string `json:"reason"`
}

// rowDiff merge-walks two ordered row streams (the old file's rows and the
// split file's logical rows) and names up to 50 differing rows. Both
// streams are ordered by the same logical key. Memory: two cursors.
func rowDiff(ctx context.Context, oldDB, newDB *sql.DB, oldQ, newQ string, keyCols int) any {
	idCol := 0
	if keyCols == 2 {
		idCol = 2 // canonical_id after (tape_seq, op_index)
	}
	a, err := newFPCursor(ctx, oldDB, oldQ, keyCols, idCol)
	if err != nil {
		return "old: " + err.Error()
	}
	defer a.rows.Close()
	b, err := newFPCursor(ctx, newDB, newQ, keyCols, idCol)
	if err != nil {
		return "new: " + err.Error()
	}
	defer b.rows.Close()
	var out []rowDiffEntry
	add := func(id, reason string) { out = append(out, rowDiffEntry{id, reason}) }
	for (a.ok || b.ok) && len(out) < 50 {
		if err := ctx.Err(); err != nil {
			return err.Error()
		}
		switch {
		case a.ok && b.ok && cmpKey(a.cur.key, b.cur.key) == 0:
			if a.cur.fp != b.cur.fp {
				add(a.cur.id, "row differs")
			}
			a.advance()
			b.advance()
		case a.ok && (!b.ok || cmpKey(a.cur.key, b.cur.key) < 0):
			add(a.cur.id, "missing in new")
			a.advance()
		default:
			add(b.cur.id, "missing in old")
			b.advance()
		}
	}
	if a.err != nil {
		out = append(out, rowDiffEntry{"", "old stream: " + a.err.Error()})
	}
	if b.err != nil {
		out = append(out, rowDiffEntry{"", "new stream: " + b.err.Error()})
	}
	return out
}

// ---- derived ----

func checkDerived(ctx context.Context, r *Report, oldDB, newDB *sql.DB) {
	claim, phase := objectgraph.BodyDerivedSQL("h.object_kind", "b.body", "h.metadata")
	for _, c := range []struct{ name, query string }{
		{"derived.og_rec_claim_recomputed",
			`SELECT h.canonical_id FROM og_objects h JOIN og_bodies b ON b.obj_id = h.obj_id WHERE h.og_rec_claim IS NOT (` + claim + `) LIMIT 20`},
		{"derived.og_event_phase_recomputed",
			`SELECT h.canonical_id FROM og_objects h JOIN og_bodies b ON b.obj_id = h.obj_id WHERE h.event_phase IS NOT (` + phase + `) LIMIT 20`},
		{"derived.choir_events_event_phase_recomputed",
			`SELECT h.canonical_id FROM choir_events h JOIN choir_event_bodies b ON b.ev_id = h.ev_id WHERE h.event_phase IS NOT (` + objectgraph.ChoirEventPhaseSQLFor("b.body", "h.metadata") + `) LIMIT 20`},
	} {
		ids, err := stringList(ctx, newDB, c.query)
		if err != nil {
			r.add(c.name, false, err.Error())
			continue
		}
		r.add(c.name, len(ids) == 0, map[string]any{"mismatched_ids_sample": ids})
	}
	// The old event_kind expression on the old file equals the new column.
	compareGroups(ctx, r, "derived.og_event_kind_expression", oldDB,
		`SELECT typeof(k), k, count(*) FROM (SELECT CASE WHEN object_kind = 'choir.event' THEN `+objectgraph.ChoirEventKindSQL+` END AS k FROM og_objects) GROUP BY 1, 2`,
		newDB,
		`SELECT typeof(event_kind), event_kind, count(*) FROM og_objects GROUP BY 1, 2`)
}

func stringList(ctx context.Context, db *sql.DB, query string, args ...any) ([]string, error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// checkRetire compares the retire selections: over the last days days, the
// old predicate's counts on the old file equal the new predicate's counts on
// the new file, per day, event kind and phase class (empty, set, NULL), on
// og_objects and on choir_events. The three retire modes (any, phaseless,
// phased) are sums of those classes. One grouped pass per side: the old
// expressions read bodies, so a query per day and mode re-read the window's
// bodies 90 times (S3, 2026-11-01: 84 minutes, 2.4 TB of logical reads).
func checkRetire(ctx context.Context, r *Report, oldDB, newDB *sql.DB, asOf time.Time, days int) {
	oldPhase := objectgraph.ChoirEventPhaseSQLFor("body", "metadata")
	lo := asOf.Truncate(24*time.Hour).AddDate(0, 0, -(days - 1))
	hi := asOf.Add(time.Nanosecond)
	const day = `substr(updated_at, 1, 10)`
	var bad []string
	compared := int64(0)
	for _, side := range []struct{ name, oldQ, newQ string }{
		{"og_objects",
			`SELECT ` + day + `, ` + objectgraph.ChoirEventKindSQL + `, (` + oldPhase + `) = '', count(*) FROM og_objects WHERE object_kind = 'choir.event' AND updated_at >= ?1 AND updated_at < ?2 GROUP BY 1, 2, 3`,
			`SELECT ` + day + `, event_kind, event_phase = '', count(*) FROM og_objects WHERE object_kind = 'choir.event' AND updated_at >= ?1 AND updated_at < ?2 GROUP BY 1, 2, 3`},
		{"choir_events",
			`SELECT ` + day + `, event_kind, (` + oldPhase + `) = '', count(*) FROM choir_events WHERE updated_at >= ?1 AND updated_at < ?2 GROUP BY 1, 2, 3`,
			`SELECT ` + day + `, event_kind, event_phase = '', count(*) FROM choir_events WHERE updated_at >= ?1 AND updated_at < ?2 GROUP BY 1, 2, 3`},
	} {
		a, err1 := groupCounts(ctx, oldDB, side.oldQ, lo, hi)
		b, err2 := groupCounts(ctx, newDB, side.newQ, lo, hi)
		if err := firstErr(err1, err2); err != nil {
			bad = append(bad, fmt.Sprintf("%s: %v", side.name, err))
			continue
		}
		for _, n := range a {
			compared += n
		}
		if dd := diffCounts(a, b); len(dd) > 0 {
			bad = append(bad, fmt.Sprintf("%s: %v", side.name, dd))
		}
	}
	if len(bad) > 50 {
		bad = bad[:50]
	}
	r.add("derived.retire_equivalence", len(bad) == 0, map[string]any{"days": days, "rows_compared": compared, "mismatches": bad})
}

// ---- single-table equality ----

func checkOneTable(ctx context.Context, r *Report, oldDB, newDB *sql.DB, check, table string) {
	oe, ne := tableExists(ctx, oldDB, table), tableExists(ctx, newDB, table)
	if !oe && !ne {
		r.add(check, true, "absent in both")
		return
	}
	if oe != ne {
		r.add(check, false, map[string]bool{"old_has": oe, "new_has": ne})
		return
	}
	digest := func(db *sql.DB) (string, error) {
		ignore, err := ignoreAllBut(ctx, db, table)
		if err != nil {
			return "", err
		}
		d, err := extractor(db, ignore).Digests(ctx)
		if err != nil {
			return "", err
		}
		return d.Tables[table], nil
	}
	a, err := digest(oldDB)
	if err != nil {
		r.add(check, false, "old: "+err.Error())
		return
	}
	b, err := digest(newDB)
	if err != nil {
		r.add(check, false, "new: "+err.Error())
		return
	}
	r.add(check, a == b && a != "", map[string]string{"old": a, "new": b})
}

// cmpKey orders keys as SQLite's binary collation does for the key types of
// the two logical tables (int64 and text).
func cmpKey(a, b []any) int {
	for i := range a {
		switch x := a[i].(type) {
		case int64:
			y, _ := b[i].(int64)
			if x != y {
				if x < y {
					return -1
				}
				return 1
			}
		default:
			if c := strings.Compare(fmt.Sprint(a[i]), fmt.Sprint(b[i])); c != 0 {
				return c
			}
		}
	}
	return 0
}
