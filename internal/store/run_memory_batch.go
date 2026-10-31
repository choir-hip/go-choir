package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// The run-memory window (owner, 2026-10-31; docs/design/tape-group-commit-2026-10-31.md
// section 9). With tape.group_commit on, a run's message entries are queued in
// the telemetry sealer like trace lines and reach the tape in the next sealed
// batch, usually riding the next acknowledged write. A crash may lose a run's
// newest messages; the crash boot closes that run, which is never resumed and
// never seeds a later one (LatestActorRunMemoryEntries reads completed and
// passivated runs only, and every such state change is an acknowledged write
// whose barrier seals the queue first). Every other kind stays acknowledged per
// write: rlm_inbox_cursor (a lost cursor re-reads the inbox: duplicate work),
// cell_fate (a lost fate re-dispatches a cell), compaction (the next run's
// checkpoint), texture_cell_author_receipt (Texture's idempotency receipt),
// and any kind added later.
//
// Reads see queued entries: the run's own list, its latest entry and a lookup
// by id merge the queue with the table, so a live run reads its own context
// without a seal. Reads across runs seal first.

// runMemoryKindBatched is the closed list of run-memory kinds that may wait in
// the window. Anything not named here is acknowledged.
func runMemoryKindBatched(kind types.RunMemoryEntryKind) bool {
	return kind == types.RunMemoryEntryMessage
}

// kindsMayBeWindowed reports whether a kind filter can match an entry that
// waits in the window. An empty filter means all kinds.
func kindsMayBeWindowed(kinds []string) bool {
	if len(kinds) == 0 {
		return true
	}
	for _, k := range kinds {
		if runMemoryKindBatched(types.RunMemoryEntryKind(k)) {
			return true
		}
	}
	return false
}

// RunMemoryDamaged reports whether a queued entry of the run was lost, so its
// log has a hole. The mark lives in memory only: the runtime fails such a run
// instead of completing it, which puts the fate on the tape.
func (s *Store) RunMemoryDamaged(runID string) bool {
	if s == nil {
		return false
	}
	s.runMem.mu.Lock()
	defer s.runMem.mu.Unlock()
	return s.runMem.damaged[runID]
}

// runMemoryTail is a run's last entry: the seq and parent of the next one.
type runMemoryTail struct {
	seq     int64
	entryID string
}

// runMemoryTailLimit bounds the tail cache; when full it is emptied, and a run
// that misses reads the table after sealing its queued entries.
const runMemoryTailLimit = 4096

// runMemoryState is the store's run-memory window bookkeeping.
type runMemoryState struct {
	mu      sync.Mutex // leaf
	tails   map[string]runMemoryTail
	damaged map[string]bool
}

func (s *Store) getRunMemoryTail(runID string) (runMemoryTail, bool) {
	s.runMem.mu.Lock()
	defer s.runMem.mu.Unlock()
	t, ok := s.runMem.tails[runID]
	return t, ok
}

func (s *Store) putRunMemoryTail(runID string, t runMemoryTail) {
	s.runMem.mu.Lock()
	defer s.runMem.mu.Unlock()
	if s.runMem.tails == nil || len(s.runMem.tails) >= runMemoryTailLimit {
		s.runMem.tails = map[string]runMemoryTail{}
	}
	s.runMem.tails[runID] = t
}

func (s *Store) forgetRunMemoryTail(runID string) {
	s.runMem.mu.Lock()
	defer s.runMem.mu.Unlock()
	delete(s.runMem.tails, runID)
}

// ErrRunMemoryLost is returned for a run one of whose queued entries could not
// be sealed: its context has a hole, so it must not go on as if whole.
var ErrRunMemoryLost = errors.New("run memory lost a queued entry; the run cannot continue on a partial context")

func (s *Store) markRunMemoryDamaged(runID string) {
	s.runMem.mu.Lock()
	defer s.runMem.mu.Unlock()
	if s.runMem.damaged == nil {
		s.runMem.damaged = map[string]bool{}
	}
	s.runMem.damaged[runID] = true
	delete(s.runMem.tails, runID)
}

func (s *Store) runMemoryDamagedErr(runID string) error {
	s.runMem.mu.Lock()
	defer s.runMem.mu.Unlock()
	if s.runMem.damaged[runID] {
		return fmt.Errorf("run %s: %w", runID, ErrRunMemoryLost)
	}
	return nil
}

// newRunMemoryRecord builds the queued record of one run-memory entry, or
// refuses a kind outside the window (design 3.4 layer 3, extended).
func newRunMemoryRecord(proj computerevent.RunMemoryEntryProjection, op computerevent.ProjectionOp) (telemetryRecord, error) {
	if !runMemoryKindBatched(types.RunMemoryEntryKind(proj.Kind)) || op.Kind != computerevent.ProjectionOpRunMemoryEntry {
		return telemetryRecord{}, fmt.Errorf("telemetry: run memory kind %q is not batched", proj.Kind)
	}
	p := proj
	return telemetryRecord{runID: proj.RunID, ownerID: proj.OwnerID, runMemory: &p, op: op, size: len(op.Body)}, nil
}

// recheckRunMemoryOp is the seal-time second look for a run-memory op.
func recheckRunMemoryOp(op computerevent.ProjectionOp) error {
	var p computerevent.RunMemoryEntryProjection
	if err := json.Unmarshal(op.Body, &p); err != nil {
		return fmt.Errorf("decode run memory entry: %w", err)
	}
	if !runMemoryKindBatched(types.RunMemoryEntryKind(p.Kind)) {
		return fmt.Errorf("run memory kind %q is not batched", p.Kind)
	}
	if op.Table != "" {
		return fmt.Errorf("run memory op is stamped for %q", op.Table)
	}
	return nil
}

// holdsRunMemory reports whether the queue (or the batch in flight) holds an
// entry of runID, or of any run when runID is empty.
func (sl *telemetrySealer) holdsRunMemory(runID string) bool {
	sl.mu.Lock()
	defer sl.mu.Unlock()
	for _, list := range [][]telemetryRecord{sl.queue, sl.inflightRecs} {
		for _, rec := range list {
			if rec.runMemory != nil && (runID == "" || rec.runID == runID) {
				return true
			}
		}
	}
	return false
}

// pendingRunMemory copies the queued and in-flight entries matching the
// filter (run, entry id; an empty filter matches all; ownerID scopes when set).
func (sl *telemetrySealer) pendingRunMemory(ownerID, runID, entryID string) []computerevent.RunMemoryEntryProjection {
	sl.mu.Lock()
	defer sl.mu.Unlock()
	var out []computerevent.RunMemoryEntryProjection
	for _, list := range [][]telemetryRecord{sl.inflightRecs, sl.queue} {
		for _, rec := range list {
			p := rec.runMemory
			if p == nil || (runID != "" && p.RunID != runID) || (entryID != "" && p.EntryID != entryID) || (ownerID != "" && p.OwnerID != ownerID) {
				continue
			}
			out = append(out, *p)
		}
	}
	return out
}

// noteDropped is called for records the sealer gave up on: a run-memory
// entry that was lost leaves its run damaged.
func (sl *telemetrySealer) noteDropped(recs ...telemetryRecord) {
	for _, rec := range recs {
		if rec.runMemory != nil {
			sl.s.markRunMemoryDamaged(rec.runID)
		}
	}
}

// projectionScanner hands a projection to scanRunMemoryEntry, so a queued
// entry reads exactly as the row it will become.
type projectionScanner struct {
	p computerevent.RunMemoryEntryProjection
}

func (r projectionScanner) Scan(dest ...any) error {
	p := r.p
	details := p.DetailsJSON
	if strings.TrimSpace(details) == "" {
		details = "{}"
	}
	vals := []any{p.EntryID, p.RunID, p.OwnerID, p.AgentID, p.ParentEntryID, p.Seq, p.Kind, p.Role,
		p.MessageJSON, p.Summary, p.FirstKeptEntryID, p.TokensBefore, p.Reason, p.Model, details, p.CreatedAt}
	if len(dest) != len(vals) {
		return fmt.Errorf("run memory scan: %d columns, want %d", len(dest), len(vals))
	}
	for i, d := range dest {
		switch d := d.(type) {
		case *string:
			*d = vals[i].(string)
		case *int64:
			*d = vals[i].(int64)
		case *int:
			*d = vals[i].(int)
		case *types.RunMemoryEntryKind:
			*d = types.RunMemoryEntryKind(vals[i].(string))
		default:
			return fmt.Errorf("run memory scan: column %d has type %T", i, d)
		}
	}
	return nil
}

// pendingRunMemoryEntries snapshots the queued entries as rows, before the
// table is read: an entry leaves the queue only after it is in the table, so
// taking the queue first and the table second misses nothing.
func (s *Store) pendingRunMemoryEntries(ownerID, runID, entryID string) ([]types.RunMemoryEntry, error) {
	sl := s.telemetry.Load()
	if sl == nil {
		return nil, nil
	}
	var out []types.RunMemoryEntry
	for _, p := range sl.pendingRunMemory(ownerID, runID, entryID) {
		e, err := scanRunMemoryEntry(projectionScanner{p})
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, nil
}

// mergeRunMemory adds the pending entries not yet in the table, in seq order.
func mergeRunMemory(table, pending []types.RunMemoryEntry) []types.RunMemoryEntry {
	if len(pending) == 0 {
		return table
	}
	seen := make(map[string]bool, len(table))
	for _, e := range table {
		seen[e.EntryID] = true
	}
	out := append([]types.RunMemoryEntry(nil), table...)
	for _, e := range pending {
		if !seen[e.EntryID] {
			out = append(out, e)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Seq < out[j].Seq })
	return out
}

// sealRunMemoryForRead seals queued run memory before a read across runs.
// AnyRunMemoryDamaged reports whether any run in this process lost a queued
// entry. A planned restart writes its marker as a failed drain then, so the
// next boot closes runs instead of parking damaged ones as seeds.
func (s *Store) AnyRunMemoryDamaged() bool {
	if s == nil {
		return false
	}
	s.runMem.mu.Lock()
	defer s.runMem.mu.Unlock()
	return len(s.runMem.damaged) > 0
}

// MarkRunMemoryDamaged records a run whose log is known to have a hole (the
// seam for the runtime and its tests).
func (s *Store) MarkRunMemoryDamaged(runID string) {
	if s != nil {
		s.markRunMemoryDamaged(runID)
	}
}

// SealRunMemory seals the run's queued entries and waits. A run calls it
// before it ends, so a seal that fails marks the run damaged while the run
// can still be failed, not after it has completed.
func (s *Store) SealRunMemory(runID string) {
	if sl := s.telemetry.Load(); sl != nil && sl.holdsRunMemory(runID) {
		sl.barrier(sealRead)
	}
}

func (s *Store) sealRunMemoryForRead(_ context.Context) {
	if sl := s.telemetry.Load(); sl != nil && sl.holdsRunMemory("") {
		sl.barrier(sealRead)
	}
}
