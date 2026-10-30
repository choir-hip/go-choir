package platform

import (
	"context"
	"strconv"
	"time"
)

// Newest-N reads over Store B are time-bounded (§4 G2 of
// docs/archive/reports/object-graph-retrieval-research-2026-10-14.md): Dolt 2.1.9
// answers "ORDER BY updated_at DESC LIMIT n" over an equality prefix by
// sorting the whole range, so a computer with a million captures pays for a
// million rows to read a thousand. The ladder reads disjoint windows,
// newest first ([anchor-24h, ∞), then [anchor-7d, anchor-24h), …, then
// everything older), each with the rows still wanted, and stops when the
// limit is met. Windows are disjoint and each is read newest first, so the
// concatenation is exactly the unbounded query's answer.
var newestWindows = []time.Duration{
	24 * time.Hour,
	7 * 24 * time.Hour,
	30 * 24 * time.Hour,
	365 * 24 * time.Hour,
}

// newestWindow is one rung: rows with lo <= updated_at < hi, where a zero
// lo or hi is unbounded.
type newestWindow struct {
	lo, hi time.Time
}

// readNewestWindows calls read for each window, newest first, with the
// number of rows still wanted, until limit rows are read or the last
// (unbounded) window is done. read returns how many rows it got. The shape
// names the last window read ("window_24h", …, "window_all").
func readNewestWindows(ctx context.Context, anchor time.Time, limit int, read func(w newestWindow, want int) (int, error)) (string, error) {
	// DATETIME holds seconds; whole-second bounds keep one boundary row
	// in exactly one window.
	anchor = anchor.UTC().Truncate(time.Second)
	var hi time.Time
	got := 0
	for i := 0; ; i++ {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		w := newestWindow{hi: hi}
		shape := "window_all"
		if i < len(newestWindows) {
			w.lo = anchor.Add(-newestWindows[i])
			shape = "window_" + windowName(newestWindows[i])
		}
		n, err := read(w, limit-got)
		if err != nil {
			return shape, err
		}
		got += n
		if got >= limit || i >= len(newestWindows) {
			return shape, nil
		}
		hi = w.lo
	}
}

func windowName(d time.Duration) string {
	switch {
	case d%(24*time.Hour) == 0 && d >= 24*time.Hour:
		days := int(d / (24 * time.Hour))
		if days == 1 {
			return "24h"
		}
		return strconv.Itoa(days) + "d"
	default:
		return d.String()
	}
}

// windowSQL is the window's predicate on col and its arguments.
func (w newestWindow) windowSQL(col string) (string, []any) {
	var sql string
	var args []any
	if !w.lo.IsZero() {
		sql += " AND " + col + " >= ?"
		args = append(args, w.lo)
	}
	if !w.hi.IsZero() {
		sql += " AND " + col + " < ?"
		args = append(args, w.hi)
	}
	return sql, args
}
