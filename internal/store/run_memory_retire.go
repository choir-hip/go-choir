package store

// Run-memory retirement is a tape operation (L1 step 2 of
// docs/definitions/choir-lean-system-2026-10-30.md). run_memory_entries is
// 11.2 GB and 5.1M rows on the paper (2026-11-14). Every entry reached the
// working store from a ProjectionOpRunMemoryEntry on the sealed tape, so the
// tape holds it; this op takes a finished run's entries out of the working
// store only. Nothing on the tape is touched.
//
// What reads run memory after a run finishes (checked at each caller):
//   - LatestActorRunMemoryEntries (agentcore/run_memory.go carryActorConversation,
//     texture_lifecycle_api.go): a Texture run continuing the same document
//     carries the newest completed or passivated run's conversation that holds
//     entries (no other desk inherits a prior run since 2026-11-01). Only a
//     document custodian's agent (deskmanifest.IsDocumentInbox) is ever carried
//     from, and the source needs a run record. The invariant this job keeps
//     (docs/design/bounding-the-guest-store-2026-11-14.md): the newest
//     runMemoryKeepPerAgent completed or passivated runs of every document
//     custodian's agent keep their entries whatever their age, ordered as that
//     reader orders them (FinishedAt, else UpdatedAt). A failed, cancelled or
//     record-less run takes no slot; a record that does not decode is kept.
//     No other agent's old run is read by anything.
//   - ListRunMemoryEntriesForAgent(rlm_inbox_cursor) (rlm_reduce.go:525): the
//     inbox cursor. Rows of that kind are never retired (KeepKinds).
//   - ListRunMemoryEntries(run) of a live or just-finished run (management,
//     Texture, engineering assignment, lifecycle SourceRunID resume): only a
//     run that is not terminal, or finished within the window, is read. The
//     late management report reads a producer run's memory only for a live
//     trajectory (lifecycle.go, inside !lateEvidenceOnly), where the producer
//     run must be pending, running or passivated
//     (persistentManagementRunStateAllowed, engineering_assignments.go:680);
//     none of those is ever retired, and a terminal producer run is refused or
//     read for no memory whatever retirement did
//     (TestManagementReportOutcomeDoesNotDependOnRetiredRunMemory).
//   - get_run_memory_entry by id (agentcore/tools_evidence.go:57): an entry
//     of a retired run answers not-found, a visible refusal.
//
// A run is retired only when it has not written an entry in
// runMemoryRetireWindow, is not one of the carry sources above, and either has
// a record that says it is terminal (completed, failed, cancelled; a
// passivated run can be re-warmed, a blocked one resumed) and finished more
// than the window ago, or has no record at all and its newest entry is older
// than runMemoryRetireNoRecordWindow (nothing can carry from, resume or name a
// run with no record, but absence is not proof of state, so the hold is
// longer). The
// record is read by its owner-scoped id and, failing that, by its run id: a
// run the lifecycle writes lives under a computer-scoped id, which the
// owner-scoped read never finds (docs/problems/
// run-memory-retirement-keeps-nearly-everything-2026-11-14.md). The op names
// the runs explicitly, so replay removes exactly what the live apply removed.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

var (
	// runMemoryRetireWindow is how old a finished run's last entry must be.
	runMemoryRetireWindow = 7 * 24 * time.Hour
	// runMemoryKeepPerAgent is how many newest completed or passivated runs per
	// owner and document custodian agent keep their entries whatever their age.
	runMemoryKeepPerAgent = 2
	// runMemoryRetireNoRecordWindow is how old the newest entry of a run with no
	// record must be. It can be lowered once a census on the paper shows what
	// remains of the runs the old lookup could not see.
	runMemoryRetireNoRecordWindow = 30 * 24 * time.Hour
	// runMemoryRetireMaxOps times computerevent.RunMemoryRetireMaxRuns (100) is
	// the runs one pass names: 50 x 100 = 5,000. At runMemoryRetireRunsPerOp (10)
	// runs an op that is up to 500 ops, each at least 250 ms apart (rest of four
	// times the op's own time when longer), so a full pass takes at least two
	// minutes of rest and appends about 500 tape events.
	runMemoryRetireMaxOps = 50
	// runMemoryRetireKeepKinds are the kinds that outlive their run.
	runMemoryRetireKeepKinds = []string{"rlm_inbox_cursor"}
)

// projectRunMemoryRetire applies a run-memory retire op in the batch
// transaction.
func projectRunMemoryRetire(ctx context.Context, tx *sql.Tx, body []byte) error {
	p, err := computerevent.DecodeRunMemoryRetire(body)
	if err != nil {
		return fmt.Errorf("%w: %v", computerevent.ErrProjectionBatchInvalid, err)
	}
	args := make([]any, 0, len(p.RunIDs)+len(p.KeepKinds))
	marks := make([]string, len(p.RunIDs))
	for i, id := range p.RunIDs {
		marks[i] = "?"
		args = append(args, id)
	}
	query := `DELETE FROM run_memory_entries WHERE loop_id IN (` + strings.Join(marks, ",") + `)`
	if len(p.KeepKinds) > 0 {
		kmarks := make([]string, len(p.KeepKinds))
		for i, k := range p.KeepKinds {
			kmarks[i] = "?"
			args = append(args, k)
		}
		query += ` AND kind NOT IN (` + strings.Join(kmarks, ",") + `)`
	}
	if _, err := tx.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("run memory retire: delete: %w", err)
	}
	return nil
}

type runMemoryGroup struct {
	runID, ownerID, agentID string
	last                    time.Time
	retirable               int64
	// carry is the rows a carry can use (runMemoryCarryKinds).
	carry int64
	// mixedOwner is set when the run's document-agent rows name more than one owner.
	mixedOwner bool
	// entries is every row of the run, cursors included (census only).
	entries int64
}

// parseStoreTime reads a time that MAX(created_at) returns as a time.Time or
// as text in one of the layouts the driver writes.
func parseStoreTime(v any) (time.Time, bool) {
	switch x := v.(type) {
	case time.Time:
		return x.UTC(), true
	case string:
		for _, layout := range []string{time.RFC3339Nano, "2006-01-02 15:04:05.999999999-07:00", "2006-01-02 15:04:05.999999999 -0700 MST", "2006-01-02 15:04:05.999999999", "2006-01-02T15:04:05.999999999"} {
			if t, err := time.Parse(layout, x); err == nil {
				return t.UTC(), true
			}
		}
	case []byte:
		return parseStoreTime(string(x))
	}
	return time.Time{}, false
}

// runMemoryRetireGroupQuery is the per-run summary the selection reads. It
// is served by idx_run_memory_entries_loop_retire alone: every column it
// names is in that index, so it never reads a row, whose created_at and kind
// sit after message_json (a table scan read the whole 3.8 GB table). COUNT(*)
// is the census's row count and is free in the same index. The body's size is
// not: length(message_json) is in the row, after which overflow pages hold a
// large body, so summing it would read the table; the census keeps rows only.
func runMemoryRetireGroupQuery() (string, []any) {
	marks := func(vals []string) string {
		m := make([]string, len(vals))
		for i := range vals {
			m[i] = "?"
		}
		return strings.Join(m, ",")
	}
	var args []any
	for _, k := range runMemoryRetireKeepKinds {
		args = append(args, k)
	}
	for _, k := range runMemoryCarryKinds {
		args = append(args, k)
	}
	// The document custodian's agent id and its owner are aggregated apart from
	// the plain MAX: a row of another agent (a worker's) can sort above
	// "texture:<doc>" and must not hide the run's document agent.
	prefix := deskmanifest.DocumentCustodianName() + ":"
	n := len([]rune(prefix))
	args = append(args, n, prefix, n, prefix, n, prefix)
	return `SELECT loop_id, MAX(owner_id), MAX(agent_id), MAX(created_at),
		SUM(CASE WHEN kind NOT IN (` + marks(runMemoryRetireKeepKinds) + `) THEN 1 ELSE 0 END), COUNT(*),
		SUM(CASE WHEN kind IN (` + marks(runMemoryCarryKinds) + `) THEN 1 ELSE 0 END),
		MAX(CASE WHEN substr(agent_id, 1, ?) = ? THEN agent_id END),
		MAX(CASE WHEN substr(agent_id, 1, ?) = ? THEN owner_id END),
		MIN(CASE WHEN substr(agent_id, 1, ?) = ? THEN owner_id END)
		FROM run_memory_entries GROUP BY loop_id`, args
}

// SelectRunMemoryRetirements returns the loop ids whose entries may leave the
// working store at now. It reads only (read pool); the caller appends the op.
func (s *Store) SelectRunMemoryRetirements(ctx context.Context, now time.Time) ([]string, error) {
	ids, _, err := s.selectRunMemory(ctx, now)
	return ids, err
}

// runMemoryPlan is the runs a pass retires, by why: runs whose record says they
// are finished, and runs with no record. groups holds each planned run's group,
// for the census to say what was applied.
type runMemoryPlan struct {
	recorded, noRecord []string
	groups             map[string]runMemoryGroup
}

// selectRunMemory is the plan's runs sorted, with the census.
func (s *Store) selectRunMemory(ctx context.Context, now time.Time) ([]string, RunMemoryCensus, error) {
	plan, census, err := s.selectRunMemoryPlan(ctx, now)
	ids := append(append([]string(nil), plan.recorded...), plan.noRecord...)
	sort.Strings(ids)
	return ids, census, err
}

// runRecordMemo reads each carry candidate's record once per pass (the slot
// pre-pass and the main loop both need it). A record of any other run is read
// once and not kept: a pass holds at most the document agents' runs. Nothing is
// kept across passes: a run's state changes while a wake runs (a passivated run
// is reactivated, a run is cancelled) and a delete is irreversible, so every
// pass reads every record fresh. The cost of that is counted (record_reads) and
// measured on the paper's first wake.
type runRecordMemo struct {
	s *Store
	m map[string]runRecordResult
}

type runRecordResult struct {
	rec types.RunRecord
	err error
	// copiesDisagree is set when the run's live copies name more than one
	// computer or more than one agent.
	copiesDisagree bool
}

// get returns the run's record result; reads counts the lookups actually made.
func (m *runRecordMemo) get(ctx context.Context, g *runMemoryGroup, keep bool, reads *int64) runRecordResult {
	key := g.ownerID + "\x00" + g.runID
	if r, ok := m.m[key]; ok {
		return r
	}
	*reads++
	rec, disagree, err := m.s.runRecordCopiesForRetirement(ctx, g.ownerID, g.runID)
	r := runRecordResult{rec, err, disagree}
	if keep {
		m.m[key] = r
	}
	return r
}

// selectRunMemoryPlan is the selection and the census of the groups it
// considered: each group lands in exactly one disposition, named by the first
// rule that keeps it, in the order the rules apply. The census is finished on
// every return.
func (s *Store) selectRunMemoryPlan(ctx context.Context, now time.Time) (plan runMemoryPlan, census RunMemoryCensus, err error) {
	census = newRunMemoryCensus()
	plan.groups = map[string]runMemoryGroup{}
	defer func() { census.finish() }()
	db := s.ReadDB()
	if db == nil {
		return plan, census, nil
	}
	cutoff := now.Add(-runMemoryRetireWindow).UTC()
	query, args := runMemoryRetireGroupQuery()
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return plan, census, fmt.Errorf("run memory retire: group: %w", err)
	}
	var groups []runMemoryGroup
	for rows.Next() {
		var g runMemoryGroup
		var last any
		var docAgent, docOwner, docOwnerMin sql.NullString
		if err := rows.Scan(&g.runID, &g.ownerID, &g.agentID, &last, &g.retirable, &g.entries, &g.carry, &docAgent, &docOwner, &docOwnerMin); err != nil {
			_ = rows.Close()
			return plan, census, err
		}
		// A loop id the retire op would refuse (blank or padded) cannot be named
		// in one: skip it here, never trim it (the delete is by exact loop id).
		if g.runID == "" || g.runID != strings.TrimSpace(g.runID) {
			census.add("kept_loop_id_invalid", g)
			continue
		}
		// A run with document custodian rows is that agent's, whatever other
		// agent or owner ids its other rows carry.
		if docAgent.Valid && docAgent.String != "" {
			g.agentID = docAgent.String
			if docOwner.Valid && docOwner.String != "" {
				g.ownerID = docOwner.String
				g.mixedOwner = docOwnerMin.Valid && docOwnerMin.String != docOwner.String
			}
		}
		t, ok := parseStoreTime(last)
		if !ok {
			census.add("kept_unreadable_time", g) // an unreadable time is never proof of age
			continue
		}
		g.last = t
		groups = append(groups, g)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return plan, census, err
	}
	_ = rows.Close()

	memo := &runRecordMemo{s: s, m: map[string]runRecordResult{}}

	// The carry sources: each owner's document custodian agent on one computer
	// keeps its newest runMemoryKeepPerAgent completed or passivated runs that
	// hold something a carry uses, in the order LatestActorRunMemoryEntries takes
	// them (carryRunBefore, resolveRunCopies: the one rule, run_memory_carry.go).
	// A failed, cancelled, blocked, running or record-less run takes no slot, and
	// neither does a run with only cursors or receipts; a record that does not
	// decode is protected, and so is a run whose rows and record disagree about
	// its agent.
	type slot struct {
		runID string
		at    time.Time
	}
	agents := map[string]bool{}
	perAgent := map[string]int{}
	slots := map[string][]slot{}
	protected := map[string]bool{}
	for i := range groups {
		g := &groups[i]
		if g.agentID == "" {
			continue
		}
		agents[g.ownerID+"\x00"+g.agentID] = true
		perAgent[g.ownerID+"\x00"+g.agentID]++
		if !deskmanifest.IsDocumentInbox(g.agentID) || g.carry == 0 || g.mixedOwner {
			continue
		}
		if err := ctx.Err(); err != nil {
			return plan, census, err
		}
		r := memo.get(ctx, g, true, &census.RecordReads.SlotPrePass)
		switch {
		case r.err == nil:
			if r.rec.AgentID != g.agentID || r.copiesDisagree {
				protected[g.ownerID+"\x00"+g.runID] = true // fail closed
				continue
			}
			if carrySourceState(r.rec.State) {
				key := g.ownerID + "\x00" + r.rec.ComputerID + "\x00" + g.agentID
				slots[key] = append(slots[key], slot{g.runID, runCarryFinishedAt(r.rec)})
			}
		case errors.Is(r.err, errRunRecordUnreadable):
			protected[g.ownerID+"\x00"+g.runID] = true
		case errors.Is(r.err, ErrNotFound):
		default:
			return plan, census, r.err
		}
	}
	census.Agents = int64(len(agents))
	for _, n := range perAgent {
		if n <= runMemoryKeepPerAgent {
			census.AgentsWithOneOrTwoRuns++
		}
	}
	for key, list := range slots {
		sort.Slice(list, func(i, j int) bool {
			return carryRunBefore(list[i].at, list[i].runID, list[j].at, list[j].runID)
		})
		owner, _, _ := strings.Cut(key, "\x00")
		for i := 0; i < len(list) && i < runMemoryKeepPerAgent; i++ {
			protected[owner+"\x00"+list[i].runID] = true
		}
	}
	for _, g := range groups {
		if !protected[g.ownerID+"\x00"+g.runID] {
			continue
		}
		age := now.Sub(g.last)
		switch {
		case age < runMemoryRetireWindow:
			census.CarrySources.Within7d.add(g)
		case age < runMemoryRetireNoRecordWindow:
			census.CarrySources.From7To30d.add(g)
		default:
			census.CarrySources.Over30d.add(g)
		}
	}

	maxRuns := runMemoryRetireMaxOps * computerevent.RunMemoryRetireMaxRuns
	for i := range groups {
		g := &groups[i]
		switch {
		case g.retirable == 0:
			census.add("kept_only_cursor_entries", *g)
			continue
		case !g.last.Before(cutoff):
			census.add("kept_newest_entry_in_window", *g)
			continue
		case g.mixedOwner:
			// The record cannot be read under one owner and the delete is by loop id.
			census.add("kept_mixed_owner_rows", *g)
			continue
		case protected[g.ownerID+"\x00"+g.runID]:
			if now.Sub(g.last) < runMemoryRetireNoRecordWindow {
				census.add("kept_carry_source_7_30d", *g)
			} else {
				census.add("kept_carry_source_over_30d", *g)
			}
			continue
		}
		if len(plan.recorded)+len(plan.noRecord) >= maxRuns {
			census.add("not_reached_over_cap", *g)
			continue
		}
		if err := ctx.Err(); err != nil {
			return plan, census, err
		}
		r := memo.get(ctx, g, false, &census.RecordReads.Selection)
		if r.err != nil {
			switch {
			case errors.Is(r.err, ErrNotFound):
				// No record under either id: nothing can carry from, resume or
				// name this run. Absence is not proof of state, so the hold is
				// longer than for a run whose record says it finished.
				if now.Sub(g.last) < runMemoryRetireNoRecordWindow {
					census.add("kept_no_run_record_in_hold", *g)
					continue
				}
				census.add("retired_no_run_record", *g)
				plan.noRecord = append(plan.noRecord, g.runID)
				plan.groups[g.runID] = *g
			case errors.Is(r.err, errRunRecordUnreadable):
				census.add("kept_run_record_unreadable", *g) // an unreadable record is never proof of state
			default:
				return plan, census, r.err
			}
			continue
		}
		run := r.rec
		if r.copiesDisagree {
			// Live copies of the run name more than one computer or agent; the
			// reader reads one computer's by one agent, so which copy decides is
			// not known.
			census.add("kept_run_copies_disagree", *g)
			continue
		}
		// The rows said this run is not a document agent's and the record says it
		// is (or the other way): the carry reads records, so keep the run.
		if deskmanifest.IsDocumentInbox(run.AgentID) != deskmanifest.IsDocumentInbox(g.agentID) {
			census.add("kept_run_agent_disagrees", *g)
			continue
		}
		switch run.State {
		case types.RunCompleted, types.RunFailed, types.RunCancelled:
		case types.RunPassivated:
			census.add("kept_state_passivated", *g)
			continue
		case types.RunBlocked:
			census.add("kept_state_blocked", *g)
			continue
		default:
			census.add("kept_state_other:"+string(run.State), *g)
			continue
		}
		finished := runCarryFinishedAt(run)
		if finished.IsZero() || !finished.Before(cutoff) {
			census.add("kept_finished_in_window", *g)
			continue
		}
		census.add("retired", *g)
		plan.recorded = append(plan.recorded, g.runID)
		plan.groups[g.runID] = *g
	}
	sort.Strings(plan.recorded)
	sort.Strings(plan.noRecord)
	return plan, census, nil
}

// errRunRecordUnreadable is a run record that exists and does not decode.
var errRunRecordUnreadable = errors.New("run record unreadable")

// runRecordForRetirement is the run's record as the selection needs it, read
// through the read pool (never the single write connection): the owner-scoped
// id, and every copy the run id in the record's metadata (an indexed column)
// names, which finds the computer-scoped lifecycle records. Only the owner's
// live copies whose own run id matches count. When more than one exists
// resolveRunCopies picks (the carry reader picks the same way): a copy that is
// not terminal keeps the run, else the newest finish.
// A copy that does not decode makes the record unreadable.
func (s *Store) runRecordForRetirement(ctx context.Context, ownerID, runID string) (types.RunRecord, error) {
	rec, _, err := s.runRecordCopiesForRetirement(ctx, ownerID, runID)
	return rec, err
}

// runRecordCopiesForRetirement is runRecordForRetirement and whether the run's
// live copies name more than one computer or more than one agent (the reader
// reads by both, so which copy decides is not known).
func (s *Store) runRecordCopiesForRetirement(ctx context.Context, ownerID, runID string) (types.RunRecord, bool, error) {
	if hook := runRecordLookupHook; hook != nil {
		if err := hook(ownerID, runID); err != nil {
			return types.RunRecord{}, false, err
		}
	}
	graph := s.ogReadStore
	if graph == nil {
		graph = s.ogStore
	}
	if graph == nil {
		return types.RunRecord{}, false, fmt.Errorf("store: object graph not initialized")
	}
	var copies []objectgraph.Object
	if id, err := objectgraph.BuildCanonicalID(ogKindRun, ownerID, objectgraph.StableSuffixFromKey(runID)); err == nil {
		obj, err := graph.GetObject(ctx, id)
		switch {
		case err == nil:
			copies = append(copies, obj)
		case !errors.Is(err, objectgraph.ErrNotFound):
			// A read that failed (context, I/O) says nothing about the record.
			return types.RunRecord{}, false, err
		}
	}
	// A run id has a few copies at most: one page, and a second only when the
	// first was full (the usual run costs one read, not two).
	after := ""
	for {
		page, err := graph.ListObjectsByMetadataPage(ctx, string(ogKindRun), "$.run_id", runID, after, ogMetadataPageSize)
		if err != nil {
			return types.RunRecord{}, false, err
		}
		copies = append(copies, page...)
		if len(page) < ogMetadataPageSize {
			break
		}
		if page[len(page)-1].CanonicalID <= after {
			return types.RunRecord{}, false, fmt.Errorf("run record lookup: metadata page did not advance")
		}
		after = page[len(page)-1].CanonicalID
	}
	seen := map[string]bool{}
	var best types.RunRecord
	found, disagree := false, false
	computer, agent := "", ""
	for _, obj := range copies {
		if seen[obj.CanonicalID] {
			continue
		}
		seen[obj.CanonicalID] = true
		if obj.Tombstone || obj.OwnerID != ownerID {
			continue
		}
		var r types.RunRecord
		if err := ogDecode(obj, &r); err != nil {
			return types.RunRecord{}, false, fmt.Errorf("%w: %v", errRunRecordUnreadable, err)
		}
		if r.RunID != runID {
			continue
		}
		if found && (r.ComputerID != computer || r.AgentID != agent) {
			disagree = true
		}
		computer, agent = r.ComputerID, r.AgentID
		best, found = resolveRunCopies(best, found, r), true
	}
	if !found {
		return types.RunRecord{}, false, ErrNotFound
	}
	return best, disagree, nil
}

// runMemoryRetireRunsPerOp is the runs one op names when this store writes
// it (at most computerevent.RunMemoryRetireMaxRuns): one op is one
// transaction under the event lock, and a run of the paper's can hold
// thousands of entries.
var runMemoryRetireRunsPerOp = 10

const (
	runMemoryRetireReason         = "run memory retirement: finished runs older than the window"
	runMemoryRetireNoRecordReason = "run memory retirement: runs with no run record, newest entry older than the no-record hold"
)

// RetireRunMemory appends the retire ops for the runs the selection names,
// runMemoryRetireRunsPerOp to an op, each op with the reason its runs are
// retired. It returns the ops appended. An unbound store retires nothing.
func (s *Store) RetireRunMemory(ctx context.Context, now time.Time) (int, error) {
	if s == nil || s.projectionTape == nil {
		return 0, nil
	}
	started := time.Now().UTC()
	plan, census, err := s.selectRunMemoryPlan(ctx, now)
	if err != nil {
		s.recordRunMemoryCensus(census, started)
		return 0, err
	}
	// The census is of what this pass applied: runs it planned and did not reach
	// (an append failed, the context ended) are moved out of "retired" before it
	// is recorded, so the wake's totals count applied runs only.
	applied := map[string]bool{}
	defer func() {
		for _, id := range plan.recorded {
			if !applied[id] {
				census.unretire("retired", plan.groups[id])
			}
		}
		for _, id := range plan.noRecord {
			if !applied[id] {
				census.unretire("retired_no_run_record", plan.groups[id])
			}
		}
		s.recordRunMemoryCensus(census, started)
	}()
	appended := 0
	appendRuns := func(ids []string, reason string) error {
		for start := 0; start < len(ids); start += runMemoryRetireRunsPerOp {
			chunk := ids[start:min(start+runMemoryRetireRunsPerOp, len(ids))]
			body, err := marshalJSON(computerevent.RunMemoryRetireProjection{
				RunIDs: chunk, KeepKinds: runMemoryRetireKeepKinds, Reason: reason,
			})
			if err != nil {
				return err
			}
			// The floor first, durably: an older binary does not know the op.
			if err := s.ensureProjectionBatchTrimFloor(); err != nil {
				return err
			}
			opStart := time.Now()
			if err := s.AppendProjectionOps(ctx, []computerevent.ProjectionOp{{Kind: computerevent.ProjectionOpRunMemoryRetire, Body: body}}); err != nil {
				return err
			}
			appended++
			for _, id := range chunk {
				applied[id] = true
			}
			// The same duty-cycle bound as the telemetry ops (telemetry_retire.go).
			rest := max(telemetryRetireChunkPause, time.Duration(telemetryRetireRest*float64(time.Since(opStart))))
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(rest):
			}
		}
		return nil
	}
	if err := appendRuns(plan.recorded, runMemoryRetireReason); err != nil {
		return appended, err
	}
	if err := appendRuns(plan.noRecord, runMemoryRetireNoRecordReason); err != nil {
		return appended, err
	}
	if appended > 0 {
		log.Printf("store: run memory retirement: %d runs in %d ops", len(plan.recorded)+len(plan.noRecord), appended)
	}
	return appended, nil
}

// runMemoryCatchUpGap is the rest between back-to-back passes of a backlog.
// Each pass already rests after every op (a duty cycle of one in five at
// most); the gap lets a pass's group read and the desks' work through.
var runMemoryCatchUpGap = time.Minute

// runMemoryWakeMaxPasses bounds the passes of one wake. The paper's backlog is
// 107,711 runs, 22 passes of 5,000; 32 passes is 160,000 runs, half as many
// again, and what is left waits for the next day's wake.
var runMemoryWakeMaxPasses = 32

// RetireRunMemoryBacklog runs passes until one leaves no run over the per-pass
// cap, retires nothing, ready says no, or the wake has run
// runMemoryWakeMaxPasses, resting gap between them. A backlog the size of the
// paper's drains in one wake, not one pass a day. now is the start; a pass that
// runs long judges age by that start, which only keeps more. It returns the ops
// and passes done. The census keeps the wake's first pass and its totals.
func (s *Store) RetireRunMemoryBacklog(ctx context.Context, now time.Time, gap time.Duration, ready func() bool) (ops, passes int, err error) {
	if s == nil {
		return 0, 0, nil
	}
	s.runMemoryCensus.beginWake()
	wakeStarted := time.Now()
	defer func() {
		s.runMemoryCensus.endWake()
		// One census in the log per wake: the last pass's, and the wake's totals
		// (a catch-up of 22 passes would log 300 lines otherwise).
		c := s.RunMemoryRetirementCensus()
		c.logLines()
		if c.Wake != nil {
			for _, d := range c.Wake.Retired {
				log.Printf("store: run memory census: wake of %d passes: %s: %d runs, %d entries", c.Wake.Passes, d.Disposition, d.Runs, d.Entries)
			}
			log.Printf("store: run memory census: wake of %d passes took %s; record reads: %d in the slot pre-pass, %d in the selection",
				c.Wake.Passes, time.Since(wakeStarted).Round(time.Millisecond), c.Wake.RecordReads.SlotPrePass, c.Wake.RecordReads.Selection)
		}
	}()
	for {
		n, err := s.RetireRunMemory(ctx, now)
		passes++
		ops += n
		if err != nil {
			return ops, passes, err
		}
		if n == 0 || s.runMemoryOverCap() == 0 || passes >= runMemoryWakeMaxPasses {
			return ops, passes, nil
		}
		if ready != nil && !ready() {
			return ops, passes, nil
		}
		select {
		case <-ctx.Done():
			return ops, passes, ctx.Err()
		case <-time.After(gap):
		}
	}
}

// runMemoryOverCap is the runs the last pass left unreached because of the cap.
func (s *Store) runMemoryOverCap() int64 {
	for _, d := range s.RunMemoryRetirementCensus().Dispositions {
		if d.Disposition == "not_reached_over_cap" {
			return d.Runs
		}
	}
	return 0
}

// runRecordLookupHook, when set, is consulted at the start of every record
// lookup; an error it returns is the lookup's error (a seam for the test that a
// read error aborts the pass, in the slot pre-pass and in the selection).
var runRecordLookupHook func(ownerID, runID string) error
