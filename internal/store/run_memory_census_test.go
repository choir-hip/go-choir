package store

// The census the run-memory retirement pass takes of every run group it
// considers (run_memory_retire.go). Ways it can fail, written before the code:
//
//   - C1: a run group is counted twice, or not at all, so the dispositions do
//     not sum to the groups (or the rows to the table);
//   - C2: the census changes what is retired: the selected set differs from the
//     selection before the census existed (legacySelectRunMemoryRetirements);
//   - C3: a run past the per-pass cap is silently dropped instead of counted
//     as not reached;
//   - C4: the census needs a wider read than the covering index;
//   - C5: the last pass's census is not kept, or is lost between passes.

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// legacySelectRunMemoryRetirements is an independent copy of the R-1 selection
// (protection only for document custodians, no record retires), with its own
// copy of the group query, over fixtures whose runs all have legacy ids, so the
// census is held to the same set.
func legacySelectRunMemoryRetirements(ctx context.Context, s *Store, now time.Time) ([]string, error) {
	db := s.ReadDB()
	cutoff := now.Add(-runMemoryRetireWindow).UTC()
	rows, err := db.QueryContext(ctx, `SELECT loop_id, owner_id, agent_id, MAX(created_at), SUM(CASE WHEN kind NOT IN (?) THEN 1 ELSE 0 END) FROM run_memory_entries GROUP BY loop_id`, runMemoryRetireKeepKinds[0])
	if err != nil {
		return nil, err
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
			continue
		}
		g.last = t
		groups = append(groups, g)
	}
	_ = rows.Close()
	byAgent := map[string][]*runMemoryGroup{}
	for i := range groups {
		g := &groups[i]
		if !deskmanifest.IsDocumentInbox(g.agentID) {
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
		run, err := s.GetRunByOwnerOG(ctx, g.ownerID, g.runID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				out = append(out, g.runID)
				continue
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

// memSeedEntries writes entries of the given kinds for a run with no run
// record unless record is true (then through memSeed's state).
func memSeedEntries(t *testing.T, h *tapeHarness, runID, agentID string, kinds []types.RunMemoryEntryKind, entryAgo time.Duration, now time.Time) {
	t.Helper()
	for i, kind := range kinds {
		_, err := h.store.AppendRunMemoryEntry(context.Background(), types.RunMemoryEntry{
			RunID: runID, OwnerID: "owner-1", AgentID: agentID, Kind: kind, Role: "user",
			Message: json.RawMessage(`{"text":"hello"}`), CreatedAt: now.Add(-entryAgo + time.Duration(i)*time.Second),
		})
		if err != nil {
			t.Fatalf("append %s: %v", runID, err)
		}
	}
}

// memCensusScript is memScript plus a run in each remaining disposition.
func memCensusScript(t *testing.T, h *tapeHarness, now time.Time) {
	day := 24 * time.Hour
	memScript(t, h, now)
	memSeed(t, h, "z-pass", docAgent("z"), types.RunPassivated, 40*day, 40*day, now)
	memSeed(t, h, "z-blk", docAgent("z"), types.RunBlocked, 40*day, 40*day, now)
	memSeed(t, h, "z-run", docAgent("z"), types.RunRunning, 0, 40*day, now)
	memSeed(t, h, "z-fin", docAgent("z"), types.RunCompleted, 2*day, 40*day, now)
	memSeedEntries(t, h, "z-nf", docAgent("z"), []types.RunMemoryEntryKind{"message", "message", "rlm_inbox_cursor"}, 41*day, now)
	// A finished run holding only its cursor (retirable = 0).
	if err := h.store.CreateRunOG(context.Background(), types.RunRecord{RunID: "z-cur", AgentID: docAgent("z"), OwnerID: "owner-1", ComputerID: "computer-1", State: types.RunCompleted, CreatedAt: now.Add(-51 * day), UpdatedAt: now.Add(-50 * day)}); err != nil {
		t.Fatal(err)
	}
	memSeedEntries(t, h, "z-cur", docAgent("z"), []types.RunMemoryEntryKind{"rlm_inbox_cursor"}, 50*day, now)
	memSeed(t, h, "z-new1", docAgent("z"), types.RunCompleted, 10*day, 10*day, now)
	memSeed(t, h, "z-new2", docAgent("z"), types.RunCompleted, 9*day, 9*day, now)
}

func censusByName(c RunMemoryCensus) map[string]RunMemoryDisposition {
	out := map[string]RunMemoryDisposition{}
	for _, d := range c.Dispositions {
		out[d.Disposition] = d
	}
	return out
}

func TestRunMemoryCensusCountsEveryGroupExactlyOnce(t *testing.T) {
	h := newTapeHarness(t)
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	memCensusScript(t, h, now)

	ids, c, err := h.store.selectRunMemory(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"v1", "v2", "v3", "v4", "x-old", "z-nf"}; !reflect.DeepEqual(ids, want) {
		t.Fatalf("selected %v, want %v", ids, want)
	}
	got := censusByName(c)
	want := map[string][2]int64{ // runs, entries
		"retired":                           {5, 15},
		"retired_no_run_record":             {1, 3},
		"kept_only_cursor_entries":          {1, 1},
		"kept_newest_entry_in_window":       {1, 3},
		"kept_newest_two_of_agent_7_30d":    {4, 12},
		"kept_newest_two_of_agent_over_30d": {0, 0},
		"kept_state_passivated":             {2, 6},
		"kept_state_blocked":                {2, 6},
		"kept_state_other:running":          {2, 6},
		"kept_finished_in_window":           {1, 3},
		"not_reached_over_cap":              {0, 0},
		"kept_unreadable_time":              {0, 0},
		"kept_run_record_unreadable":        {0, 0},
	}
	var runs, entries int64
	for _, d := range c.Dispositions {
		runs += d.Runs
		entries += d.Entries
	}
	for name, w := range want {
		g, ok := got[name]
		if !ok {
			t.Errorf("no disposition %s in %+v", name, c.Dispositions)
			continue
		}
		if g.Runs != w[0] || g.Entries != w[1] {
			t.Errorf("%s = %d runs %d entries, want %d, %d", name, g.Runs, g.Entries, w[0], w[1])
		}
	}
	if len(got) != len(want) {
		t.Errorf("dispositions %v, want exactly %d", c.Dispositions, len(want))
	}
	if runs != 19 || c.Groups != 19 {
		t.Errorf("dispositions sum to %d runs, groups %d, want 19", runs, c.Groups)
	}
	var tableRows int
	for _, n := range memRunsWithRows(t, h.store) {
		tableRows += n
	}
	if entries != int64(tableRows) || c.Entries != int64(tableRows) {
		t.Errorf("dispositions sum to %d entries, census %d, table %d", entries, c.Entries, tableRows)
	}
	if got["retired"].RetirableEntries != 10 {
		t.Errorf("retired retirable entries = %d, want 10", got["retired"].RetirableEntries)
	}
	if c.Agents != 7 || c.AgentsWithOneOrTwoRuns != 4 {
		t.Errorf("agents = %d (%d with 1-2 runs), want 7 (4)", c.Agents, c.AgentsWithOneOrTwoRuns)
	}
	// The protected set (document custodians only) regardless of the order
	// the rules apply in.
	ov := c.NewestTwoOfAgent
	if ov.Within7d.Runs != 0 || ov.From7To30d.Runs != 4 || ov.Over30d.Runs != 0 {
		t.Errorf("newest-two overlap = %+v, want 0/4/0", ov)
	}
}

// C2: the retire set is what it was before the census, on a fixture with
// every disposition.
func TestRunMemoryCensusLeavesTheSelectionAsItWas(t *testing.T) {
	h := newTapeHarness(t)
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	memCensusScript(t, h, now)
	old, err := legacySelectRunMemoryRetirements(context.Background(), h.store, now)
	if err != nil {
		t.Fatal(err)
	}
	got, err := h.store.SelectRunMemoryRetirements(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(old, got) || len(got) == 0 {
		t.Fatalf("selection %v, before the census %v", got, old)
	}
}

// C3: runs beyond the cap are counted as not reached, and the set is still the
// legacy set.
func TestRunMemoryCensusCountsRunsBeyondTheCap(t *testing.T) {
	h := newTapeHarness(t)
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	day := 24 * time.Hour
	ops := runMemoryRetireMaxOps
	runMemoryRetireMaxOps = 1
	t.Cleanup(func() { runMemoryRetireMaxOps = ops })
	total := computerevent.RunMemoryRetireMaxRuns + 5 + runMemoryKeepPerAgent
	for i := 0; i < total; i++ {
		memSeed(t, h, "c"+strconv.Itoa(1000+i), docAgent("c"), types.RunCompleted, 40*day, 40*day-time.Duration(i)*time.Minute, now)
	}
	old, err := legacySelectRunMemoryRetirements(context.Background(), h.store, now)
	if err != nil {
		t.Fatal(err)
	}
	ids, c, err := h.store.selectRunMemory(context.Background(), now)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(ids, old) || len(ids) != computerevent.RunMemoryRetireMaxRuns {
		t.Fatalf("selected %d, legacy %d, want %d", len(ids), len(old), computerevent.RunMemoryRetireMaxRuns)
	}
	got := censusByName(c)
	if got["retired"].Runs != int64(len(ids)) || got["not_reached_over_cap"].Runs != 5 || got["kept_newest_two_of_agent_over_30d"].Runs != 2 {
		t.Fatalf("census %+v", c.Dispositions)
	}
	if c.Groups != int64(total) {
		t.Fatalf("groups %d, want %d", c.Groups, total)
	}
}

// C5: a pass records its census with its times; none before the first pass.
func TestRunMemoryCensusIsKeptFromTheLastPass(t *testing.T) {
	ctx := context.Background()
	withRetireKnobs(t)
	h := newTapeHarness(t)
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	before := h.store.RunMemoryRetirementCensus()
	if before.Passes != 0 || before.StartedAt != nil || before.FinishedAt != nil || before.Dispositions == nil || len(before.Dispositions) != 0 {
		t.Fatalf("before any pass: %+v", before)
	}
	memCensusScript(t, h, now)
	if _, err := h.store.RetireRunMemory(ctx, now); err != nil {
		t.Fatal(err)
	}
	c := h.store.RunMemoryRetirementCensus()
	if c.Passes != 1 || c.StartedAt == nil || c.FinishedAt == nil || c.FinishedAt.Before(*c.StartedAt) {
		t.Fatalf("after a pass: %+v", c)
	}
	if censusByName(c)["retired"].Runs != 5 || censusByName(c)["retired_no_run_record"].Runs != 1 {
		t.Fatalf("census %+v", c.Dispositions)
	}
	// The second pass finds the retired runs holding only cursors.
	if _, err := h.store.RetireRunMemory(ctx, now); err != nil {
		t.Fatal(err)
	}
	c = h.store.RunMemoryRetirementCensus()
	if c.Passes != 2 || censusByName(c)["retired"].Runs != 0 {
		t.Fatalf("second pass: %+v", c)
	}
}
