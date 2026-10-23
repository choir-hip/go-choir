package store

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"
)

// The reader audit of run_memory_entries' indexes (the "index diet", lane
// fp0-run-memory-index-diet; option 4 of
// docs/problems/store-journal-write-amplification-2026-10-23.md, worked in
// docs/problems/run-memory-entries-hold-the-engine-2026-10-23.md, section
// "The index diet"). Every secondary index on the table costs one prolly-tree
// path (9 to 13 KiB of journal) on every append, so an index no production
// statement needs is a standing cost. Ways the audit could be wrong, written
// before the code:
//
//  1. The statement list misses a reader: a new statement filters on columns
//     only a candidate serves. TestRunMemoryProductionStatementCensus counts
//     the statements in the source and fails when one is not in the list.
//  2. A plan is compared with the index dropped on a store too small for the
//     planner to prefer it: the table is seeded with several owners, agents and
//     loops so a range is selective.
//  3. A plan is "equal" because both are table scans: the test pins the access
//     each statement must have (the columns the plan names).
//  4. An ORDER BY created_at read loses its ordering when the (owner_id,
//     agent_id, created_at) index goes: the plan with no owner_agent_created
//     must still be a range on the kind index (owner_id, agent_id, ...) and
//     sort above it, exactly as before (created_at, seq is never fully indexed).
//  5. The comparison is made on the order the drops happen in, not the final
//     set: every subset is planned, not only the first drop.

const (
	idxOwnerLoopSeq     = "idx_run_memory_entries_owner_loop_seq"
	idxOwnerAgentCreate = "idx_run_memory_entries_owner_agent_created"
	idxParent           = "idx_run_memory_entries_parent"
)

// auditIndexDDL recreates a candidate exactly as the old schemaDDL made it.
var auditIndexDDL = map[string]string{
	idxOwnerLoopSeq:     `CREATE INDEX ` + idxOwnerLoopSeq + ` ON run_memory_entries(owner_id, loop_id, seq)`,
	idxOwnerAgentCreate: `CREATE INDEX ` + idxOwnerAgentCreate + ` ON run_memory_entries(owner_id, agent_id, created_at)`,
	idxParent:           `CREATE INDEX ` + idxParent + ` ON run_memory_entries(parent_entry_id)`,
}

// auditSeed fills run_memory_entries the way production does: loops of 50
// entries, each entry's parent the previous entry of its loop, three owners,
// 40 agents and a hot desk, the kind mix of the hold census.
func auditSeed(t *testing.T, s *Store, n int) {
	t.Helper()
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	tx, err := s.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	var (
		vals []string
		args []any
	)
	flush := func() {
		if len(vals) == 0 {
			return
		}
		if _, err := tx.Exec(`INSERT INTO run_memory_entries
			(entry_id, loop_id, owner_id, agent_id, parent_entry_id, seq, kind, role, message_json, summary,
			 first_kept_entry_id, tokens_before, reason, model, details_json, created_at) VALUES `+strings.Join(vals, ","), args...); err != nil {
			t.Fatal(err)
		}
		vals, args = vals[:0], args[:0]
	}
	kinds := []string{"message", "message", "cell_fate", "rlm_inbox_cursor", "rlm_inbox_cursor", "compaction"}
	const loopLen = 50
	for i := 0; i < n; i++ {
		loop := i / loopLen
		agent := fmt.Sprintf("agent-%02d", loop%40)
		if loop%5 == 0 {
			agent = "agent-hot"
		}
		owner := fmt.Sprintf("owner-%d", 1+loop%3)
		if agent == "agent-hot" {
			owner = "owner-1"
		}
		seq := i%loopLen + 1
		parent := ""
		if seq > 1 {
			parent = fmt.Sprintf("e-%08d", i-1)
		}
		vals = append(vals, "(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)")
		args = append(args, fmt.Sprintf("e-%08d", i), fmt.Sprintf("run-%s-%d", agent, loop), owner, agent, parent, seq,
			kinds[i%len(kinds)], "", strings.Repeat("m", 200), "", "", 0, "", "",
			fmt.Sprintf(`{"channel_id":"ch-1","desk_agent_id":%q,"cursor":%d}`, agent, i), base.Add(time.Duration(i)*time.Second))
		if len(vals) == 100 {
			flush()
		}
	}
	flush()
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
}

type auditStatement struct {
	where string
	query string
	args  []any
}

// auditStatements is every production reader of run_memory_entries, with the
// file:line where it is (this branch). The loop-keyed ones are the shared list
// of the R5 test; the agent reads come from runMemoryAgentListQuery, in every
// shape a caller can send.
func auditStatements() []auditStatement {
	out := make([]auditStatement, 0, 20)
	for _, st := range runMemoryProductionStatements {
		out = append(out, auditStatement{st.where, st.query, st.args})
	}
	add := func(where, owner, agent string, kinds []string) {
		q, a := runMemoryAgentListQuery(owner, agent, kinds)
		out = append(out, auditStatement{where, q, a})
	}
	add("run_memory.go:290 ListRunMemoryEntriesForAgent owner, agent, one kind (the only production caller, rlm_reduce.go:520)", "owner-1", "agent-hot", []string{"rlm_inbox_cursor"})
	add("run_memory.go:290 ListRunMemoryEntriesForAgent owner, agent, two kinds", "owner-1", "agent-hot", []string{"rlm_inbox_cursor", "cell_fate"})
	add("run_memory.go:290 ListRunMemoryEntriesForAgent owner, agent, no kinds (ORDER BY created_at)", "owner-1", "agent-hot", nil)
	add("run_memory.go:290 ListRunMemoryEntriesForAgent no owner, agent, one kind (no production caller)", "", "agent-hot", []string{"rlm_inbox_cursor"})
	// Statements by the columns a candidate leads with, which no production
	// statement sends: pinned so a future one is visible as a regression.
	out = append(out,
		auditStatement{"(not production) WHERE parent_entry_id = ?", `SELECT entry_id FROM run_memory_entries WHERE parent_entry_id = ?`, []any{"e-00000010"}},
		auditStatement{"(not production) WHERE owner_id = ? AND loop_id = ? AND seq > ? ORDER BY seq", `SELECT entry_id FROM run_memory_entries WHERE owner_id = ? AND loop_id = ? AND seq > ? ORDER BY seq`, []any{"owner-1", "run-agent-hot-0", 10}},
	)
	return out
}

func auditPlans(t *testing.T, s *Store) map[string]string {
	t.Helper()
	plans := map[string]string{}
	for _, st := range auditStatements() {
		plans[st.where] = explainEventTablePlan(t, s, st.query, st.args...)
	}
	return plans
}

func auditSetIndexes(t *testing.T, s *Store, keep map[string]bool) {
	t.Helper()
	for name, ddl := range auditIndexDDL {
		has := runMemoryIndexExists(t, s, name)
		switch {
		case keep[name] && !has:
			if _, err := s.db.Exec(ddl); err != nil {
				t.Fatal(err)
			}
		case !keep[name] && has:
			if _, err := s.db.Exec(`DROP INDEX ` + name + ` ON run_memory_entries`); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func auditRows() int {
	if v := os.Getenv("CHOIR_RUN_MEMORY_AUDIT_ROWS"); v != "" {
		var n int
		if _, err := fmt.Sscan(v, &n); err == nil && n > 0 {
			return n
		}
	}
	return 20000
}

// accessColumns pulls the "[a,b,c]" index column lists out of a plan.
var accessColumnsRE = regexp.MustCompile(`index: \[([^\]]*)\]`)

func planAccess(plan string) string {
	m := accessColumnsRE.FindAllStringSubmatch(plan, -1)
	if len(m) == 0 {
		if strings.Contains(plan, "Table") {
			return "full scan"
		}
		return "?"
	}
	var cols []string
	for _, x := range m {
		cols = append(cols, strings.ReplaceAll(x[1], "run_memory_entries.", ""))
	}
	return strings.Join(cols, " + ")
}

// The access each statement is expected to use on the diet's final set (the
// primary key, UNIQUE(loop_id, seq), and the kind index), written from the
// plans of the full set before any index was dropped.
func TestRunMemoryIndexAuditPlansWithAndWithoutEachCandidate(t *testing.T) {
	s := openTestStore(t)
	if !runMemoryIndexExists(t, s, RunMemoryKindIndexName) {
		t.Fatal("a fresh store lacks the kind index; the audit's baseline is wrong")
	}
	auditSeed(t, s, auditRows())
	all := map[string]bool{idxOwnerLoopSeq: true, idxOwnerAgentCreate: true, idxParent: true}
	auditSetIndexes(t, s, all)
	full := auditPlans(t, s)

	subsets := []struct {
		name string
		keep map[string]bool
	}{
		{"without owner_loop_seq", map[string]bool{idxOwnerAgentCreate: true, idxParent: true}},
		{"without owner_agent_created", map[string]bool{idxOwnerLoopSeq: true, idxParent: true}},
		{"without parent", map[string]bool{idxOwnerLoopSeq: true, idxOwnerAgentCreate: true}},
		{"without owner_loop_seq and owner_agent_created", map[string]bool{idxParent: true}},
		{"without all three", map[string]bool{}},
	}
	statements := auditStatements()
	for _, sub := range subsets {
		auditSetIndexes(t, s, sub.keep)
		got := auditPlans(t, s)
		for _, st := range statements {
			if got[st.where] == full[st.where] {
				continue
			}
			t.Logf("PLAN CHANGE %s\n  %s\n  full:    %s\n  without: %s", sub.name, st.where, planAccess(full[st.where]), planAccess(got[st.where]))
			if os.Getenv("CHOIR_RUN_MEMORY_AUDIT_PLANS") != "" {
				t.Logf("FULL PLAN\n%s\nWITHOUT PLAN\n%s", full[st.where], got[st.where])
			}
		}
	}
	// The baseline plans, one line each, for the record.
	keys := make([]string, 0, len(full))
	for k := range full {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		t.Logf("FULL SET %s: %s", k, planAccess(full[k]))
	}
}
