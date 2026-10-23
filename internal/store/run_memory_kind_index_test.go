package store

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

// The kind index of run_memory_entries (shape 3 of
// docs/problems/run-memory-entries-hold-the-engine-2026-10-23.md). The
// paper's first slow-read line held the write connection 16.6 s before the
// first row, reading the management desk's whole range to keep 14,965 cursor
// rows. Ways the change could fail, written before the code:
//
//  1. The index goes into schemaDDL, so the paper builds it at boot, before
//     its listener, on a populated table (the shape of
//     boot-alters-og-objects-when-older-columns-missing-2026-10-21).
//  2. The explicit operation does not build it, builds it twice, or reports
//     nothing, so the operator cannot tell it ran.
//  3. A fresh store lacks it, so a new computer starts on the slow plan.
//  4. The plan still walks the desk's whole range with kind filtered above
//     the index, so the index exists and is not used (owner given, owner
//     empty, two kinds).
//  5. The answer changes: the index changes which rows or what order the
//     cursor read returns.
//  6. A rollback is unsafe: an older binary's boot DDL fails on the extra
//     index, or a reopen drops or rebuilds it.
const testRunMemoryKindIndex = "idx_run_memory_entries_owner_agent_kind_created"

func runMemoryIndexExists(t *testing.T, s *Store, name string) bool {
	t.Helper()
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM information_schema.STATISTICS
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'run_memory_entries' AND INDEX_NAME = ?`, name).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n > 0
}

// seedRunMemory inserts n rows of one owner: a hot desk (every fifth row) with
// the kind mix of the census, and 40 other agents.
func seedRunMemory(t *testing.T, s *Store, n int) {
	t.Helper()
	ctx := context.Background()
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	tx, err := s.db.BeginTx(ctx, nil)
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
		if _, err := tx.ExecContext(ctx, `INSERT INTO run_memory_entries
			(entry_id, loop_id, owner_id, agent_id, parent_entry_id, seq, kind, role, message_json, summary,
			 first_kept_entry_id, tokens_before, reason, model, details_json, created_at) VALUES `+strings.Join(vals, ","), args...); err != nil {
			t.Fatal(err)
		}
		vals, args = vals[:0], args[:0]
	}
	kinds := []string{"message", "message", "cell_fate", "rlm_inbox_cursor", "rlm_inbox_cursor", "compaction"}
	hotSeen := 0
	for i := 0; i < n; i++ {
		agent := fmt.Sprintf("agent-%02d", i%40)
		kind := kinds[i%len(kinds)]
		if i%5 == 0 {
			agent = "agent-hot"
			kind = kinds[hotSeen%len(kinds)]
			hotSeen++
		}
		vals = append(vals, "(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)")
		args = append(args, fmt.Sprintf("e-%06d", i), fmt.Sprintf("run-%s-%d", agent, i/500), "owner-1", agent, "", i, kind, "",
			strings.Repeat("m", 200), "", "", 0, "", "",
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

// populatedRunMemoryStore is a reopened runtime store whose run_memory_entries
// has rows and no kind index: the paper before the operation.
func populatedRunMemoryStore(t *testing.T) (*Store, string) {
	t.Helper()
	path := testStorePath(t)
	cleanupTestStorePath(path)
	t.Cleanup(func() { cleanupTestStorePath(path) })
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if runMemoryIndexExists(t, s, testRunMemoryKindIndex) {
		if _, err := s.db.Exec(`DROP INDEX ` + testRunMemoryKindIndex + ` ON run_memory_entries`); err != nil {
			t.Fatal(err)
		}
	}
	seedRunMemory(t, s, 1200)
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s, path
}

func TestRunMemoryKindIndexFreshStoreHasIt(t *testing.T) {
	s := openTestStore(t)
	if !runMemoryIndexExists(t, s, testRunMemoryKindIndex) {
		t.Fatalf("a fresh store lacks %s: a new computer would start on the slow plan", testRunMemoryKindIndex)
	}
}

func TestRunMemoryKindIndexIsNotBuiltAtBootOnAPopulatedTable(t *testing.T) {
	s, path := populatedRunMemoryStore(t)
	if runMemoryIndexExists(t, s, testRunMemoryKindIndex) {
		t.Fatal("Open built the kind index on a populated run_memory_entries: a boot-path build")
	}
	// A second reopen, with the table still populated, still does not.
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.Reopen(path); err != nil {
		t.Fatal(err)
	}
	if runMemoryIndexExists(t, s, testRunMemoryKindIndex) {
		t.Fatal("a second Open built the kind index on a populated run_memory_entries")
	}
	if strings.Contains(schemaDDL, testRunMemoryKindIndex) {
		t.Fatal("schemaDDL names the kind index: it would build at every boot")
	}
}

func TestRunMemoryKindIndexMigrationBuildsItOnce(t *testing.T) {
	s, _ := populatedRunMemoryStore(t)
	idle := s.DerivedColumnMigrationStatus(context.Background())
	if idle.RunMemoryKindIndex == nil || idle.RunMemoryKindIndex.Present || idle.RunMemoryKindIndex.Name != testRunMemoryKindIndex {
		t.Fatalf("idle status does not report the missing index: %+v", idle.RunMemoryKindIndex)
	}
	if _, err := s.StartDerivedColumnMigration(); err != nil {
		t.Fatal(err)
	}
	done := waitDerivedColumnMigration(t, s)
	if done.State != DerivedColumnMigrationDone {
		t.Fatalf("migration state %q: %s", done.State, done.Error)
	}
	if !runMemoryIndexExists(t, s, testRunMemoryKindIndex) {
		t.Fatal("the migration did not build the kind index")
	}
	var steps []string
	for _, step := range done.Steps {
		if step.Name == testRunMemoryKindIndex {
			steps = append(steps, step.Kind)
			if step.Err != "" {
				t.Fatalf("step failed: %s", step.Err)
			}
		}
	}
	if !reflect.DeepEqual(steps, []string{"index"}) {
		t.Fatalf("steps naming the index = %v, want one index step", steps)
	}
	if done.RunMemoryKindIndex == nil || !done.RunMemoryKindIndex.Present {
		t.Fatalf("final status does not report the index present: %+v", done.RunMemoryKindIndex)
	}
	// A second run finds nothing to do for it.
	if _, err := s.StartDerivedColumnMigration(); err != nil {
		t.Fatal(err)
	}
	again := waitDerivedColumnMigration(t, s)
	for _, step := range again.Steps {
		if step.Name == testRunMemoryKindIndex {
			t.Fatalf("a second migration rebuilt the index: %+v", step)
		}
	}
}

// runMemoryListShapes are the shapes of ListRunMemoryEntriesForAgent's
// arguments. The only production caller (LoadInboxCursorForDesk) sends an
// owner and one kind; owner and two kinds is the same index with two ranges.
// No owner is not sent by any production caller and the owner-leading index
// does not serve it (unserved: its plan is not pinned). With all kinds the
// planner keeps the old index.
var runMemoryListShapes = []struct {
	name   string
	owner  string
	kinds  []string
	expect string // "kind", "old" or "unserved"
}{
	{"owner+1kind", "owner-1", []string{"rlm_inbox_cursor"}, "kind"},
	{"owner+2kinds", "owner-1", []string{"rlm_inbox_cursor", "cell_fate"}, "kind"},
	{"noowner+1kind", "", []string{"rlm_inbox_cursor"}, "unserved"},
	{"owner+allkinds", "owner-1", nil, "old"},
}

// The plan is read from the production statement (runMemoryAgentListQuery),
// not a copy of its text.
func TestRunMemoryKindIndexPlanHasNoFullRangeWalk(t *testing.T) {
	s, _ := populatedRunMemoryStore(t)
	plan := func(owner string, kinds []string) string {
		q, args := runMemoryAgentListQuery(owner, "agent-hot", kinds)
		return explainEventTablePlan(t, s, q, args...)
	}
	// Before: the kind filter sits above the desk's whole index range.
	if before := plan("owner-1", []string{"rlm_inbox_cursor"}); !strings.Contains(before, "Filter") {
		t.Fatalf("plan before the index changed shape; the test's premise is gone:\n%s", before)
	}
	if _, err := s.StartDerivedColumnMigration(); err != nil {
		t.Fatal(err)
	}
	if st := waitDerivedColumnMigration(t, s); st.State != DerivedColumnMigrationDone {
		t.Fatalf("migration %s: %s", st.State, st.Error)
	}
	const kindIndexCols = "[run_memory_entries.owner_id,run_memory_entries.agent_id,run_memory_entries.kind,run_memory_entries.created_at]"
	for _, shape := range runMemoryListShapes {
		p := plan(shape.owner, shape.kinds)
		switch shape.expect {
		case "kind":
			if !strings.Contains(p, kindIndexCols) {
				t.Errorf("%s does not read the kind index:\n%s", shape.name, p)
			}
			if strings.Contains(p, "Filter") {
				t.Errorf("%s still filters above the index (a full-range walk):\n%s", shape.name, p)
			}
		case "old":
			if strings.Contains(p, kindIndexCols) {
				t.Errorf("%s moved to the kind index:\n%s", shape.name, p)
			}
		}
	}
}

func TestRunMemoryKindIndexKeepsTheAnswerAndRollsBackSafely(t *testing.T) {
	ctx := context.Background()
	s, path := populatedRunMemoryStore(t)
	cursorKinds := []string{"rlm_inbox_cursor"}
	read := func() (cursor, mixed, all []string) {
		t.Helper()
		for _, c := range []struct {
			into  *[]string
			owner string
			kinds []string
		}{{&cursor, "owner-1", cursorKinds}, {&mixed, "owner-1", []string{"rlm_inbox_cursor", "cell_fate"}}, {&all, "", cursorKinds}} {
			es, err := s.ListRunMemoryEntriesForAgent(ctx, c.owner, "agent-hot", c.kinds)
			if err != nil {
				t.Fatal(err)
			}
			for _, e := range es {
				*c.into = append(*c.into, fmt.Sprintf("%s|%s|%d|%v", e.EntryID, e.Kind, e.Seq, e.Details["cursor"]))
			}
		}
		return
	}
	c0, m0, a0 := read()
	if len(c0) == 0 || len(m0) <= len(c0) || len(a0) != len(c0) {
		t.Fatalf("seed gives cursor %d, mixed %d, no-owner %d rows; the comparison would prove nothing", len(c0), len(m0), len(a0))
	}
	var rowsHash string
	if err := s.db.QueryRow(`SELECT dolt_hashof_table('run_memory_entries')`).Scan(&rowsHash); err != nil {
		t.Fatal(err)
	}

	if _, err := s.StartDerivedColumnMigration(); err != nil {
		t.Fatal(err)
	}
	if st := waitDerivedColumnMigration(t, s); st.State != DerivedColumnMigrationDone {
		t.Fatalf("migration %s: %s", st.State, st.Error)
	}
	c1, m1, a1 := read()
	if !reflect.DeepEqual(c0, c1) || !reflect.DeepEqual(m0, m1) || !reflect.DeepEqual(a0, a1) {
		t.Fatalf("the answer changed with the index:\ncursor %v -> %v", c0, c1)
	}

	// Rollback: an older binary's boot applies schemaDDL (its own three
	// indexes, IF NOT EXISTS) over the store and keeps appending; the extra
	// index must not stop either, and a reopen must neither drop nor rebuild it.
	if _, err := s.db.Exec(schemaDDL); err != nil {
		t.Fatalf("an older binary's boot DDL fails on the extra index: %v", err)
	}
	if _, err := s.db.Exec(`INSERT INTO run_memory_entries
		(entry_id, loop_id, owner_id, agent_id, parent_entry_id, seq, kind, role, message_json, summary, first_kept_entry_id, tokens_before, reason, model, details_json, created_at)
		VALUES ('e-after', 'run-after', 'owner-1', 'agent-hot', '', 1, 'rlm_inbox_cursor', '', '', '', '', 0, '', '', '{"channel_id":"ch-1","cursor":99999}', NOW())`); err != nil {
		t.Fatalf("an append on the indexed table: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.Reopen(path); err != nil {
		t.Fatal(err)
	}
	if !runMemoryIndexExists(t, s, testRunMemoryKindIndex) {
		t.Fatal("a reopen dropped the kind index")
	}
	es, err := s.ListRunMemoryEntriesForAgent(ctx, "owner-1", "agent-hot", cursorKinds)
	if err != nil || len(es) != len(c0)+1 {
		t.Fatalf("after rollback-shaped use: %d cursor rows, %v; want %d", len(es), err, len(c0)+1)
	}
	// Dropping it (the full rollback) restores the table to its row hash.
	if _, err := s.db.Exec(`DELETE FROM run_memory_entries WHERE entry_id = 'e-after'`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`DROP INDEX ` + testRunMemoryKindIndex + ` ON run_memory_entries`); err != nil {
		t.Fatal(err)
	}
	var back string
	if err := s.db.QueryRow(`SELECT dolt_hashof_table('run_memory_entries')`).Scan(&back); err != nil {
		t.Fatal(err)
	}
	if back != rowsHash {
		t.Fatalf("run_memory_entries row hash after DROP INDEX = %s, before the index = %s", back, rowsHash)
	}
	c2, _, _ := read()
	if !reflect.DeepEqual(c0, c2) {
		t.Fatalf("after DROP INDEX the cursor answer differs:\n%v\n%v", c0, c2)
	}
}

// The content witness hashes a per-table schema observation as well as the
// rows: queryTableSchemaHash is SHOW COLUMNS, and its Key column reads MUL for
// the first column of a non-unique index
// (computerversion/dolt_state_extractor.go). A live store that gained the
// kind index and a fresh store (which makes it at open, on the empty table)
// must hash the same schema, or replay eligibility records SchemaDrift and
// checkpoint_restore_bindings.go refuses a self-development checkpoint.
// bunch 160 (DeepSeek, HIGH) found the agent-leading order did exactly that:
// agent_id was not MUL before. Ways this could fail, written first: the index
// changes any Key; the test reads rows only and misses the schema observation;
// the test passes because it compares two stores that both have the index.
func runMemorySchemaObservations(t *testing.T, s *Store, path string) map[string]string {
	t.Helper()
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	set, _ := eventTableWitness(t, path)
	out := map[string]string{}
	for _, o := range set.Observations {
		if strings.Contains(o.Key, ":schema:") {
			out[o.Key] = o.Value
		}
	}
	if out["dolt:texture:schema:run_memory_entries"] == "" {
		t.Fatalf("no schema observation for run_memory_entries in %d keys", len(out))
	}
	return out
}

func TestRunMemoryKindIndexKeepsTheWitnessSchemaObservation(t *testing.T) {
	with, withPath := vocabUpcastOpenStore(t, "schema-with-index")
	if !runMemoryIndexExists(t, with, testRunMemoryKindIndex) {
		t.Fatal("a fresh store lacks the kind index")
	}
	without, withoutPath := vocabUpcastOpenStore(t, "schema-without-index")
	if _, err := without.db.Exec(`DROP INDEX ` + testRunMemoryKindIndex + ` ON run_memory_entries`); err != nil {
		t.Fatal(err)
	}
	a := runMemorySchemaObservations(t, with, withPath)
	b := runMemorySchemaObservations(t, without, withoutPath)
	for key, want := range b {
		if got := a[key]; got != want {
			t.Errorf("schema observation %s differs with the kind index:\n with:    %s\n without: %s", key, got, want)
		}
	}
	if len(a) != len(b) {
		t.Errorf("schema observation count %d with the index, %d without", len(a), len(b))
	}

	// The control that gives the test its meaning: an agent-leading index (the
	// order of the first pass of this change) does move a Key, so a drift is
	// detectable by this extractor. owner_id is already MUL through the
	// (owner_id, agent_id, created_at) index; agent_id is not.
	control, controlPath := vocabUpcastOpenStore(t, "schema-agent-leading")
	if _, err := control.db.Exec(`DROP INDEX ` + testRunMemoryKindIndex + ` ON run_memory_entries`); err != nil {
		t.Fatal(err)
	}
	if _, err := control.db.Exec(`CREATE INDEX idx_control_agent_leading ON run_memory_entries(owner_id, agent_id, kind, created_at)`); err != nil {
		t.Fatal(err)
	}
	c := runMemorySchemaObservations(t, control, controlPath)
	if c["dolt:texture:schema:run_memory_entries"] == b["dolt:texture:schema:run_memory_entries"] {
		t.Error("an agent-leading index left the schema observation unchanged: the test cannot see a drift")
	}
}

// Lows 3 and 4 of bunch 160 (DeepSeek) and its nit 6.
//
// 3. A cancel after the og_objects steps and before the index step reported
//    done with the index absent.
// 4. A status read during a run omitted run_memory_kind_index.
// 6. The open hook had no deadline and returned nothing, so a failed query
//    was invisible in the boot phase log.
func TestRunMemoryKindIndexCancelBeforeItsStepIsCancelledNotDone(t *testing.T) {
	s, _ := populatedRunMemoryStore(t)
	// Nothing is missing on og_objects, so its part returns at once without
	// looking at the context; the cancel lands before the index step.
	release := make(chan struct{})
	derivedColumnStepHook = func() { <-release }
	t.Cleanup(func() { derivedColumnStepHook = nil })
	if _, err := s.StartDerivedColumnMigration(); err != nil {
		t.Fatal(err)
	}
	running := s.DerivedColumnMigrationStatus(context.Background())
	if running.State != DerivedColumnMigrationRunning {
		t.Fatalf("state %q, want running", running.State)
	}
	if running.RunMemoryKindIndex == nil || running.RunMemoryKindIndex.Present || running.RunMemoryKindIndex.Name != testRunMemoryKindIndex {
		t.Fatalf("a status read during a run does not report the index: %+v", running.RunMemoryKindIndex)
	}
	if !s.CancelDerivedColumnMigration() {
		t.Fatal("cancel found nothing running")
	}
	close(release)
	done := waitDerivedColumnMigration(t, s)
	if done.State != DerivedColumnMigrationCancelled {
		t.Fatalf("state %q after a cancel before the index step, want cancelled (index present=%t)", done.State, runMemoryIndexExists(t, s, testRunMemoryKindIndex))
	}
	if runMemoryIndexExists(t, s, testRunMemoryKindIndex) {
		t.Fatal("a cancelled operation built the index")
	}
	if done.RunMemoryKindIndex == nil || done.RunMemoryKindIndex.Present {
		t.Fatalf("final status = %+v, want the index reported absent", done.RunMemoryKindIndex)
	}
}

func TestRunMemoryKindIndexOpenHookHasADeadlineAndReportsErrors(t *testing.T) {
	s, _ := populatedRunMemoryStore(t)
	expired, cancel := context.WithCancel(context.Background())
	cancel()
	if err := s.ensureRunMemoryKindIndexAtOpen(expired); err == nil {
		t.Fatal("the open hook swallowed an error from a cancelled context")
	}
	if runMemoryIndexExists(t, s, testRunMemoryKindIndex) {
		t.Fatal("the open hook built the index on a populated table")
	}
	// A populated table without the index is a report, not an error, and not a build.
	if err := s.ensureRunMemoryKindIndexAtOpen(context.Background()); err != nil {
		t.Fatalf("populated table: %v", err)
	}
	if runMemoryIndexExists(t, s, testRunMemoryKindIndex) {
		t.Fatal("the open hook built the index on a populated table")
	}
}
