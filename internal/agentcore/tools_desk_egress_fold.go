package agentcore

import (
	"context"
	"sync"
)

// cellEgressLog collects the per-call egress rows of one desk cell so they
// are appended once, inside the cell's desk.cell event, instead of as one
// desk.egress event each (journal cut R4,
// docs/archive/problems/paper-dolt-journal-grows-1-6-gib-per-10-minutes-2026-10-22.md).
//
// The rows have the same fields the standalone desk.egress events had, so a
// reader that takes a row out of a cell event reads what it read before. A row
// that arrives after the cell event was built (the worker's frame was served
// late) is not lost: add reports false and the caller appends it standalone.
type cellEgressLog struct {
	mu     sync.Mutex
	sealed bool
	rows   []map[string]any
}

type cellEgressLogKey struct{}

func withCellEgressLog(ctx context.Context, l *cellEgressLog) context.Context {
	return context.WithValue(ctx, cellEgressLogKey{}, l)
}

// cellEgressLogFrom returns the log of the cell whose context ctx descends
// from, or nil.
func cellEgressLogFrom(ctx context.Context) *cellEgressLog {
	l, _ := ctx.Value(cellEgressLogKey{}).(*cellEgressLog)
	return l
}

// add records row and reports whether the cell event will carry it.
func (l *cellEgressLog) add(row map[string]any) bool {
	if l == nil {
		return false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.sealed {
		return false
	}
	l.rows = append(l.rows, row)
	return true
}

// seal returns the rows collected so far and refuses later ones.
func (l *cellEgressLog) seal() []map[string]any {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.sealed = true
	return l.rows
}
