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
//     texture_lifecycle_api.go:243): the newest completed or passivated run of
//     an actor seeds its next run. The job keeps the newest runs of every
//     owner/agent pair, so that read still finds its run.
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

// SelectRunMemoryRetirements returns the loop ids whose entries may leave the
// working store at now. It reads only (read pool); the caller appends the op.
func (s *Store) SelectRunMemoryRetirements(ctx context.Context, now time.Time) ([]string, error) {
	db := s.ReadDB()
	if db == nil {
		return nil, nil
	}
	cutoff := now.Add(-runMemoryRetireWindow).UTC()
	keepMarks := make([]string, len(runMemoryRetireKeepKinds))
	args := make([]any, len(runMemoryRetireKeepKinds))
	for i, k := range runMemoryRetireKeepKinds {
		keepMarks[i], args[i] = "?", k
	}
	rows, err := db.QueryContext(ctx, `SELECT loop_id, owner_id, agent_id, MAX(created_at), SUM(CASE WHEN kind NOT IN (`+strings.Join(keepMarks, ",")+`) THEN 1 ELSE 0 END) FROM run_memory_entries GROUP BY loop_id`, args...)
	if err != nil {
		return nil, fmt.Errorf("run memory retire: group: %w", err)
	}
	var groups []runMemoryGroup
	for rows.Next() {
		var g runMemoryGroup
		var last any
		if err := rows.Scan(&g.runID, &g.ownerID, &g.agentID, &last, &g.retirable); err != nil {
			_ = rows.Close()
			return nil, err
		}
		t, ok := parseStoreTime(last)
		if !ok {
			continue // an unreadable time is never proof of age
		}
		g.last = t
		groups = append(groups, g)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
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
	for _, list := range byAgent {
		sort.Slice(list, func(i, j int) bool { return list[i].last.After(list[j].last) })
		for i := 0; i < len(list) && i < runMemoryKeepPerAgent; i++ {
			protected[list[i].runID] = true
		}
	}

	maxRuns := runMemoryRetireMaxOps * computerevent.RunMemoryRetireMaxRuns
	var out []string
	for _, g := range groups {
		if len(out) >= maxRuns {
			break
		}
		if g.retirable == 0 || !g.last.Before(cutoff) || protected[g.runID] {
			continue
		}
		if err := ctx.Err(); err != nil {
			return out, err
		}
		run, err := s.GetRunByOwnerOG(ctx, g.ownerID, g.runID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				continue // a run we cannot see is not proven finished
			}
			return out, err
		}
		switch run.State {
		case types.RunCompleted, types.RunFailed, types.RunCancelled:
		default:
			continue
		}
		finished := run.UpdatedAt
		if run.FinishedAt != nil {
			finished = *run.FinishedAt
		}
		if finished.IsZero() || !finished.Before(cutoff) {
			continue
		}
		out = append(out, g.runID)
	}
	sort.Strings(out)
	return out, nil
}

// RetireRunMemory appends the retire ops for the runs SelectRunMemoryRetirements
// names, RunMemoryRetireMaxRuns to an op. It returns the ops appended. An
// unbound store retires nothing.
func (s *Store) RetireRunMemory(ctx context.Context, now time.Time) (int, error) {
	if s == nil || s.projectionTape == nil {
		return 0, nil
	}
	ids, err := s.SelectRunMemoryRetirements(ctx, now)
	if err != nil {
		return 0, err
	}
	appended := 0
	for start := 0; start < len(ids); start += computerevent.RunMemoryRetireMaxRuns {
		chunk := ids[start:min(start+computerevent.RunMemoryRetireMaxRuns, len(ids))]
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
		if err := s.AppendProjectionOps(ctx, []computerevent.ProjectionOp{{Kind: computerevent.ProjectionOpRunMemoryRetire, Body: body}}); err != nil {
			return appended, err
		}
		appended++
		select {
		case <-ctx.Done():
			return appended, ctx.Err()
		case <-time.After(telemetryRetireChunkPause):
		}
	}
	if appended > 0 {
		log.Printf("store: run memory retirement: %d runs in %d ops", len(ids), appended)
	}
	return appended, nil
}
