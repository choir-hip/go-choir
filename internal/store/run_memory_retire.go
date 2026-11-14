package store

// Run-memory retirement is a tape operation (L1 step 2 of
// docs/definitions/choir-lean-system-2026-10-30.md). run_memory_entries is
// 3.8 GB on the paper (1.72M rows of message_json). Every entry reached the
// working store from a ProjectionOpRunMemoryEntry on the sealed tape, so the
// tape holds it; this op takes a finished run's entries out of the working
// store only. Nothing on the tape is touched.
//
// What reads run memory after a run finishes (checked at each caller):
//   - LatestActorRunMemoryEntries (agentcore/run_memory.go:142, :207,
//     texture_lifecycle_api.go:243): a Texture run continuing the same
//     document carries the newest completed or passivated run's conversation
//     (no other desk inherits a prior run since 2026-11-01). The job keeps the
//     newest runs of every owner/agent pair, so that read still finds its run.
//   - ListRunMemoryEntriesForAgent(rlm_inbox_cursor) (rlm_reduce.go:525): the
//     inbox cursor. Rows of that kind are never retired (KeepKinds).
//   - ListRunMemoryEntries(run) of a live or just-finished run (management,
//     Texture, engineering assignment, lifecycle SourceRunID resume): only a
//     run that is not terminal, or finished within the window, is read.
//   - get_run_memory_entry by id (agentcore/tools_evidence.go:57): an entry
//     of a retired run answers not-found, a visible refusal.
//
// A run is retired only when it is terminal (completed, failed, cancelled; a
// passivated run can be re-warmed, a blocked one resumed), finished more than
// runMemoryRetireWindow ago, has not written an entry in that window, and is
// not one of the newest runMemoryKeepPerAgent runs of its agent. The op names
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
	"github.com/yusefmosiah/go-choir/internal/types"
)

var (
	// runMemoryRetireWindow is how old a finished run's last entry must be.
	runMemoryRetireWindow = 7 * 24 * time.Hour
	// runMemoryKeepPerAgent is how many newest runs per owner/agent keep their
	// entries whatever their age.
	runMemoryKeepPerAgent = 2
	// runMemoryRetireMaxOps bounds the ops one run appends.
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
	keepMarks := make([]string, len(runMemoryRetireKeepKinds))
	args := make([]any, len(runMemoryRetireKeepKinds))
	for i, k := range runMemoryRetireKeepKinds {
		keepMarks[i], args[i] = "?", k
	}
	return `SELECT loop_id, owner_id, agent_id, MAX(created_at), SUM(CASE WHEN kind NOT IN (` + strings.Join(keepMarks, ",") + `) THEN 1 ELSE 0 END), COUNT(*) FROM run_memory_entries GROUP BY loop_id`, args
}

// SelectRunMemoryRetirements returns the loop ids whose entries may leave the
// working store at now. It reads only (read pool); the caller appends the op.
func (s *Store) SelectRunMemoryRetirements(ctx context.Context, now time.Time) ([]string, error) {
	ids, _, err := s.selectRunMemory(ctx, now)
	return ids, err
}

// selectRunMemory is the selection and the census of the groups it considered:
// each group lands in exactly one disposition, named by the first rule that
// keeps it, in the order the rules apply.
func (s *Store) selectRunMemory(ctx context.Context, now time.Time) ([]string, RunMemoryCensus, error) {
	census := newRunMemoryCensus()
	db := s.ReadDB()
	if db == nil {
		return nil, census, nil
	}
	cutoff := now.Add(-runMemoryRetireWindow).UTC()
	query, args := runMemoryRetireGroupQuery()
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, census, fmt.Errorf("run memory retire: group: %w", err)
	}
	var groups []runMemoryGroup
	for rows.Next() {
		var g runMemoryGroup
		var last any
		if err := rows.Scan(&g.runID, &g.ownerID, &g.agentID, &last, &g.retirable, &g.entries); err != nil {
			_ = rows.Close()
			return nil, census, err
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
		return nil, census, err
	}
	_ = rows.Close()

	// The newest runs of each owner/agent keep their entries.
	byAgent := map[string][]*runMemoryGroup{}
	for i := range groups {
		g := &groups[i]
		if g.agentID == "" {
			continue
		}
		key := g.ownerID + "\x00" + g.agentID
		byAgent[key] = append(byAgent[key], g)
	}
	protected := map[string]bool{}
	census.Agents = int64(len(byAgent))
	for _, list := range byAgent {
		if len(list) <= runMemoryKeepPerAgent {
			census.AgentsWithOneOrTwoRuns++
		}
		sort.Slice(list, func(i, j int) bool { return list[i].last.After(list[j].last) })
		for i := 0; i < len(list) && i < runMemoryKeepPerAgent; i++ {
			protected[list[i].runID] = true
		}
	}
	for _, g := range groups {
		if !protected[g.runID] {
			continue
		}
		age := now.Sub(g.last)
		switch {
		case age < runMemoryRetireWindow:
			census.NewestTwoOfAgent.Within7d.add(g)
		case age < 30*24*time.Hour:
			census.NewestTwoOfAgent.From7To30d.add(g)
		default:
			census.NewestTwoOfAgent.Over30d.add(g)
		}
	}

	maxRuns := runMemoryRetireMaxOps * computerevent.RunMemoryRetireMaxRuns
	var out []string
	for _, g := range groups {
		switch {
		case g.retirable == 0:
			census.add("kept_only_cursor_entries", g)
			continue
		case !g.last.Before(cutoff):
			census.add("kept_newest_entry_in_window", g)
			continue
		case protected[g.runID]:
			if now.Sub(g.last) < 30*24*time.Hour {
				census.add("kept_newest_two_of_agent_7_30d", g)
			} else {
				census.add("kept_newest_two_of_agent_over_30d", g)
			}
			continue
		}
		if len(out) >= maxRuns {
			census.add("not_reached_over_cap", g)
			continue
		}
		if err := ctx.Err(); err != nil {
			return out, census, err
		}
		run, err := s.GetRunByOwnerOG(ctx, g.ownerID, g.runID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				census.add("kept_run_record_not_found", g) // a run we cannot see is not proven finished
				continue
			}
			return out, census, err
		}
		switch run.State {
		case types.RunCompleted, types.RunFailed, types.RunCancelled:
		case types.RunPassivated:
			census.add("kept_state_passivated", g)
			continue
		case types.RunBlocked:
			census.add("kept_state_blocked", g)
			continue
		default:
			census.add("kept_state_other:"+string(run.State), g)
			continue
		}
		finished := run.UpdatedAt
		if run.FinishedAt != nil {
			finished = *run.FinishedAt
		}
		if finished.IsZero() || !finished.Before(cutoff) {
			census.add("kept_finished_in_window", g)
			continue
		}
		census.add("retired", g)
		out = append(out, g.runID)
	}
	sort.Strings(out)
	census.finish()
	return out, census, nil
}

// runMemoryRetireRunsPerOp is the runs one op names when this store writes
// it (at most computerevent.RunMemoryRetireMaxRuns): one op is one
// transaction under the event lock, and a run of the paper's can hold
// thousands of entries.
var runMemoryRetireRunsPerOp = 10

// RetireRunMemory appends the retire ops for the runs SelectRunMemoryRetirements
// names, runMemoryRetireRunsPerOp to an op. It returns the ops appended. An
// unbound store retires nothing.
func (s *Store) RetireRunMemory(ctx context.Context, now time.Time) (int, error) {
	if s == nil || s.projectionTape == nil {
		return 0, nil
	}
	started := time.Now().UTC()
	ids, census, err := s.selectRunMemory(ctx, now)
	if err != nil {
		return 0, err
	}
	// The census is of this pass's selection; the pass ends when the appends do.
	defer func() { s.recordRunMemoryCensus(census, started) }()
	appended := 0
	for start := 0; start < len(ids); start += runMemoryRetireRunsPerOp {
		chunk := ids[start:min(start+runMemoryRetireRunsPerOp, len(ids))]
		body, err := marshalJSON(computerevent.RunMemoryRetireProjection{
			RunIDs: chunk, KeepKinds: runMemoryRetireKeepKinds, Reason: "run memory retirement: finished runs older than the window",
		})
		if err != nil {
			return appended, err
		}
		// The floor first, durably: an older binary does not know the op.
		if err := s.ensureProjectionBatchTrimFloor(); err != nil {
			return appended, err
		}
		opStart := time.Now()
		if err := s.AppendProjectionOps(ctx, []computerevent.ProjectionOp{{Kind: computerevent.ProjectionOpRunMemoryRetire, Body: body}}); err != nil {
			return appended, err
		}
		appended++
		// The same duty-cycle bound as the telemetry ops (telemetry_retire.go).
		rest := max(telemetryRetireChunkPause, time.Duration(telemetryRetireRest*float64(time.Since(opStart))))
		select {
		case <-ctx.Done():
			return appended, ctx.Err()
		case <-time.After(rest):
		}
	}
	if appended > 0 {
		log.Printf("store: run memory retirement: %d runs in %d ops", len(ids), appended)
	}
	census.logLines()
	return appended, nil
}

// RetireRunMemoryBacklog is not built yet (test stub).
func (s *Store) RetireRunMemoryBacklog(ctx context.Context, now time.Time, gap time.Duration) (ops, passes int, err error) {
	return 0, 0, nil
}
