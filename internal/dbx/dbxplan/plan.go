// Package dbxplan holds query-plan assertions for SQLite statements over the
// split store (docs/design/store-header-body-split-2026-11-01.md section 4):
// a header read never touches a body table, and a body read reaches each body
// by its integer primary key with the header as the outer loop. It is
// imported only by tests. No statistics are assumed: ANALYZE never runs on a
// guest store.
package dbxplan

import (
	"database/sql"
	"regexp"
	"strings"
	"testing"
)

// BodyTables are the body tables of the split store.
var BodyTables = []string{"og_bodies", "choir_event_bodies"}

// Querier is what a plan is read through (*sql.DB, *sql.Tx, *sql.Conn).
type Querier interface {
	Query(query string, args ...any) (*sql.Rows, error)
}

// Plan returns the EXPLAIN QUERY PLAN detail lines of query, one per line.
func Plan(t testing.TB, db Querier, query string, args ...any) string {
	t.Helper()
	rows, err := db.Query("EXPLAIN QUERY PLAN "+query, args...)
	if err != nil {
		t.Fatalf("explain: %v\n%s", err, query)
	}
	defer rows.Close()
	var lines []string
	for rows.Next() {
		var id, parent, notused int
		var detail string
		if err := rows.Scan(&id, &parent, &notused, &detail); err != nil {
			t.Fatalf("explain scan: %v", err)
		}
		lines = append(lines, detail)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("explain rows: %v", err)
	}
	return strings.Join(lines, "\n")
}

var accessLine = regexp.MustCompile(`^(SCAN|SEARCH) (\S+)`)

// access is one table access of a plan: the verb (SCAN or SEARCH), the table
// or alias it names, and the whole line.
type access struct {
	verb, name, line string
}

func accesses(plan string) []access {
	var out []access
	for _, line := range strings.Split(plan, "\n") {
		line = strings.TrimSpace(line)
		if m := accessLine.FindStringSubmatch(line); m != nil {
			out = append(out, access{verb: m[1], name: m[2], line: line})
		}
	}
	return out
}

func namesBody(a access, extra ...string) bool {
	for _, name := range append(append([]string(nil), BodyTables...), extra...) {
		if a.name == name {
			return true
		}
	}
	return false
}

// HeaderOnly asserts that query's plan reads no body table (by table name, or
// by any of bodyAliases) and, when wantIndex is set, that it reads that index.
// It returns the plan.
func HeaderOnly(t testing.TB, db Querier, wantIndex, query string, args ...any) string {
	t.Helper()
	plan := Plan(t, db, query, args...)
	return AssertHeaderOnly(t, plan, wantIndex)
}

// AssertHeaderOnly is HeaderOnly over a plan already read.
func AssertHeaderOnly(t testing.TB, plan, wantIndex string, bodyAliases ...string) string {
	t.Helper()
	for _, a := range accesses(plan) {
		if namesBody(a, bodyAliases...) {
			t.Fatalf("header-only read touches a body table (%s):\n%s", a.line, plan)
		}
	}
	for _, body := range BodyTables {
		if strings.Contains(plan, body) {
			t.Fatalf("header-only read names body table %s:\n%s", body, plan)
		}
	}
	if wantIndex != "" && !regexp.MustCompile(`USING (COVERING )?INDEX `+regexp.QuoteMeta(wantIndex)+`\b`).MatchString(plan) {
		t.Fatalf("plan does not use index %s:\n%s", wantIndex, plan)
	}
	return plan
}

// bodySeek is the one admissible access of a body table: a rowid seek (the
// inner side of a LEFT JOIN reads "... LEFT-JOIN").
func bodySeek(name string) *regexp.Regexp {
	return regexp.MustCompile(`^SEARCH ` + regexp.QuoteMeta(name) + ` USING INTEGER PRIMARY KEY \(rowid=\?\)( LEFT-JOIN)?$`)
}

// BodyByRowid asserts that every access of the body table (named bodyAlias in
// query, or by its table name) is a rowid seek, that the body is never
// scanned, and that a header access comes before it: the header is the outer
// loop. A query that reads only the body table (a phase-2 fetch by obj_id) is
// admissible when every access is a seek. It returns the plan.
func BodyByRowid(t testing.TB, db Querier, bodyAlias, query string, args ...any) string {
	t.Helper()
	plan := Plan(t, db, query, args...)
	return AssertBodyByRowid(t, plan, bodyAlias)
}

// AssertBodyByRowid is BodyByRowid over a plan already read.
func AssertBodyByRowid(t testing.TB, plan, bodyAlias string) string {
	t.Helper()
	seek := bodySeek(bodyAlias)
	sawBody, sawHeader, headerFirst := false, false, false
	for _, a := range accesses(plan) {
		if a.name == bodyAlias || namesBody(a) {
			if !seek.MatchString(a.line) && !bodySeek(a.name).MatchString(a.line) {
				t.Fatalf("body access is not a rowid seek (%s):\n%s", a.line, plan)
			}
			sawBody = true
			continue
		}
		if !sawBody && !sawHeader {
			headerFirst = true
		}
		sawHeader = true
	}
	if !sawBody {
		t.Fatalf("plan reads no body through %s:\n%s", bodyAlias, plan)
	}
	if sawHeader && !headerFirst {
		t.Fatalf("plan reads the body before the header:\n%s", plan)
	}
	return plan
}

// NoIndexWalkCensus asserts that a whole-table header answer reads table by
// a plain scan or a covering-index scan, never by walking a non-covering
// index with a row lookup per entry. It returns the plan.
func NoIndexWalkCensus(t testing.TB, db Querier, table, query string, args ...any) string {
	t.Helper()
	plan := Plan(t, db, query, args...)
	walk := regexp.MustCompile(`^SCAN ` + regexp.QuoteMeta(table) + ` USING INDEX `)
	for _, a := range accesses(plan) {
		if walk.MatchString(a.line) {
			t.Fatalf("census walks an index with a lookup per row (%s):\n%s", a.line, plan)
		}
	}
	return plan
}
