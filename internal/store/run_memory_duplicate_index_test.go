package store

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// R5 of docs/problems/paper-dolt-journal-grows-1-6-gib-per-10-minutes-2026-10-22.md,
// worked in docs/problems/run-memory-entries-hold-the-engine-2026-10-23.md
// ("R5: the duplicate loop_seq index"): idx_run_memory_entries_loop_seq is
// (loop_id, seq), and run_memory_entries already has UNIQUE(loop_id, seq), which
// Dolt names `loop_id`. Ways the drop could fail, written before the code:
//
//  1. The index is not a duplicate: a production statement plans differently
//     without it (a slower plan, or a statement that only it served).
//  2. The drop changes the content witness (SHOW COLUMNS Key of loop_id), so a
//     migrated store and a fresh store differ and replay eligibility records
//     SchemaDrift (the finding of bunch 160 on the kind index).
//  3. The drop runs at boot on a populated table, or schemaDDL still creates the
//     index at every boot, so the drop is undone or a boot builds it again.
//  4. A fresh store still gets the index, so a fresh store and a migrated store
//     disagree in structure.
//  5. The drop is run on a store without the covering unique key, which leaves
//     (loop_id, seq) unindexed.
//  6. The operation drops it twice, drops it before it is asked, or reports
//     nothing, so the operator cannot tell it ran.
//  7. The answer of a read changes.
//  8. A rollback is unsafe or unrepeatable: recreating the index fails, or the
//     old binary's schemaDDL does something other than rebuild it.
const testRunMemoryLoopSeqIndex = "idx_run_memory_entries_loop_seq"

// The statement that older binaries' schemaDDL ran for the index.
const testRunMemoryLoopSeqDDL = `CREATE INDEX IF NOT EXISTS idx_run_memory_entries_loop_seq ON run_memory_entries(loop_id, seq)`

// ensureRunMemoryLoopSeqIndex gives a store the duplicate index, as the paper's
// store has it (every store built since the table was created has it).
func ensureRunMemoryLoopSeqIndex(t *testing.T, s *Store) {
	t.Helper()
	if runMemoryIndexExists(t, s, testRunMemoryLoopSeqIndex) {
		return
	}
	if _, err := s.db.Exec(testRunMemoryLoopSeqDDL); err != nil {
		t.Fatal(err)
	}
}

func dropRunMemoryLoopSeqIndex(t *testing.T, s *Store) {
	t.Helper()
	if !runMemoryIndexExists(t, s, testRunMemoryLoopSeqIndex) {
		return
	}
	if _, err := s.db.Exec(`DROP INDEX ` + testRunMemoryLoopSeqIndex + ` ON run_memory_entries`); err != nil {
		t.Fatal(err)
	}
}

// loopSeqPopulatedStore is a reopened store whose run_memory_entries has rows,
// the duplicate loop_seq index and no kind index: the paper before the
// operation runs.
func loopSeqPopulatedStore(t *testing.T) (*Store, string) {
	t.Helper()
	path := testStorePath(t)
	cleanupTestStorePath(path)
	t.Cleanup(func() { cleanupTestStorePath(path) })
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	ensureRunMemoryLoopSeqIndex(t, s)
	// The paper's kind index is built by the same operation; start without it
	// so the step order (kind index, then the drop) is visible.
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

// runMemoryIndexColumns returns index name -> ordered columns, and the set of
// names that are unique.
func runMemoryIndexColumns(t *testing.T, s *Store) (cols map[string][]string, unique map[string]bool) {
	t.Helper()
	rows, err := s.db.Query(`SELECT INDEX_NAME, COLUMN_NAME, NON_UNIQUE FROM information_schema.STATISTICS
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'run_memory_entries' ORDER BY INDEX_NAME, SEQ_IN_INDEX`)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	cols, unique = map[string][]string{}, map[string]bool{}
	for rows.Next() {
		var name, col string
		var nonUnique int
		if err := rows.Scan(&name, &col, &nonUnique); err != nil {
			t.Fatal(err)
		}
		cols[name] = append(cols[name], col)
		unique[name] = nonUnique == 0
	}
	return cols, unique
}

// Every production statement over run_memory_entries, with where it is. Copies
// of the SQL: the statements are inline in their functions. A new statement
// that filters on loop_id must be added here.
var runMemoryProductionStatements = []struct {
	where string
	query string
	args  []any
}{
	{"run_memory.go:55 AppendRunMemoryEntry (tape path) seq allocation",
		`SELECT COALESCE(MAX(seq), 0) + 1 FROM run_memory_entries WHERE loop_id = ?`, []any{"run-agent-03-1"}},
	{"run_memory.go:64 AppendRunMemoryEntry (tape path) parent",
		`SELECT entry_id FROM run_memory_entries WHERE loop_id = ? ORDER BY seq DESC LIMIT 1`, []any{"run-agent-03-1"}},
	{"run_memory.go:108 AppendRunMemoryEntry seq allocation",
		`SELECT COALESCE(MAX(seq), 0) + 1 FROM run_memory_entries WHERE loop_id = ?`, []any{"run-agent-03-1"}},
	{"run_memory.go:119 AppendRunMemoryEntry parent",
		`SELECT entry_id FROM run_memory_entries WHERE loop_id = ? ORDER BY seq DESC LIMIT 1`, []any{"run-agent-03-1"}},
	{"run_memory.go:247 ListRunMemoryEntries (no owner)",
		`SELECT entry_id, loop_id, owner_id, agent_id, parent_entry_id, seq, kind, role, message_json, summary, first_kept_entry_id,
		        tokens_before, reason, model, details_json, created_at
		   FROM run_memory_entries WHERE loop_id = ? ORDER BY seq ASC`, []any{"run-agent-03-1"}},
	{"run_memory.go:257 ListRunMemoryEntries (owner)",
		`SELECT entry_id, loop_id, owner_id, agent_id, parent_entry_id, seq, kind, role, message_json, summary, first_kept_entry_id,
		        tokens_before, reason, model, details_json, created_at
		   FROM run_memory_entries WHERE owner_id = ? AND loop_id = ? ORDER BY seq ASC`, []any{"owner-1", "run-agent-03-1"}},
	{"run_memory.go:390 ListRunMemoryEntryFingerprints",
		`SELECT entry_id, loop_id, owner_id, agent_id, parent_entry_id, seq, kind, role, message_json, summary, first_kept_entry_id,
		        tokens_before, reason, model, details_json, created_at
		   FROM run_memory_entries ORDER BY entry_id ASC`, nil},
	{"run_memory.go:461 LatestRunMemoryEntry",
		`SELECT entry_id, loop_id, owner_id, agent_id, parent_entry_id, seq, kind, role, message_json, summary, first_kept_entry_id,
		        tokens_before, reason, model, details_json, created_at
		   FROM run_memory_entries WHERE loop_id = ? ORDER BY seq DESC LIMIT 1`, []any{"run-agent-03-1"}},
	{"run_memory.go:481 GetRunMemoryEntry (no owner)",
		`SELECT entry_id, loop_id FROM run_memory_entries WHERE entry_id = ?`, []any{"e-000001"}},
	{"run_memory.go:491 GetRunMemoryEntry (owner)",
		`SELECT entry_id, loop_id FROM run_memory_entries WHERE owner_id = ? AND entry_id = ?`, []any{"owner-1", "e-000001"}},
	{"project.go:313 projectRunMemoryEntry read",
		`SELECT entry_id, loop_id FROM run_memory_entries WHERE entry_id=? FOR UPDATE`, []any{"e-000001"}},
	{"residue_import.go:409 residue run memory",
		`SELECT e.entry_id FROM run_memory_entries e WHERE (e.owner_id=? AND e.loop_id=?) OR (e.owner_id=? AND e.loop_id=?)
		 ORDER BY e.owner_id, e.loop_id, e.seq`, []any{"owner-1", "run-agent-03-1", "owner-1", "run-agent-04-1"}},
}

// 1. Duplicate: the unique key is (loop_id, seq), and every statement plans the
// same without the index. Plans do not name the index; they name its columns,
// so the planner choosing the unique key is visible as equality of the texts
// and the loop_id,seq columns in the loop-keyed ones.
func TestRunMemoryLoopSeqIndexIsADuplicateOfTheUniqueKey(t *testing.T) {
	s, _ := loopSeqPopulatedStore(t)
	cols, unique := runMemoryIndexColumns(t, s)
	if !reflect.DeepEqual(cols[testRunMemoryLoopSeqIndex], []string{"loop_id", "seq"}) || unique[testRunMemoryLoopSeqIndex] {
		t.Fatalf("the index under test is not a plain (loop_id, seq): %v unique=%v", cols[testRunMemoryLoopSeqIndex], unique[testRunMemoryLoopSeqIndex])
	}
	var covering []string
	for name, c := range cols {
		if unique[name] && reflect.DeepEqual(c, []string{"loop_id", "seq"}) {
			covering = append(covering, name)
		}
	}
	if len(covering) != 1 {
		t.Fatalf("unique keys on (loop_id, seq) = %v, want exactly one (indexes: %v)", covering, cols)
	}

	with := map[string]string{}
	for _, st := range runMemoryProductionStatements {
		with[st.where] = explainEventTablePlan(t, s, st.query, st.args...)
	}
	// The agent read (run_memory.go:290) from its own builder.
	q, a := runMemoryAgentListQuery("owner-1", "agent-hot", []string{"rlm_inbox_cursor"})
	with["run_memory.go:290 ListRunMemoryEntriesForAgent"] = explainEventTablePlan(t, s, q, a...)

	dropRunMemoryLoopSeqIndex(t, s)
	if runMemoryIndexExists(t, s, testRunMemoryLoopSeqIndex) {
		t.Fatal("the index is still there after the drop")
	}
	for where, before := range with {
		var after string
		if where == "run_memory.go:290 ListRunMemoryEntriesForAgent" {
			after = explainEventTablePlan(t, s, q, a...)
		} else {
			for _, st := range runMemoryProductionStatements {
				if st.where == where {
					after = explainEventTablePlan(t, s, st.query, st.args...)
				}
			}
		}
		if before != after {
			t.Errorf("%s plans differently without the index:\nwith:\n%s\nwithout:\n%s", where, before, after)
		}
		if strings.Contains(where, "AppendRunMemoryEntry") || strings.Contains(where, "LatestRunMemoryEntry") || strings.Contains(where, "ListRunMemoryEntries (no owner)") {
			if !strings.Contains(after, "[run_memory_entries.loop_id,run_memory_entries.seq]") {
				t.Errorf("%s does not read the (loop_id, seq) key without the duplicate:\n%s", where, after)
			}
		}
	}
}

// 2. The witness: SHOW COLUMNS reads MUL for the first column of a non-unique
// index and of a composite unique one. loop_id is MUL through UNIQUE(loop_id,
// seq) already; the drop must leave every table's schema observation as it was,
// or a migrated and a fresh store differ and the replay report is SchemaDrift.
func TestRunMemoryLoopSeqIndexDropKeepsTheWitnessSchemaObservation(t *testing.T) {
	with, withPath := vocabUpcastOpenStore(t, "loopseq-schema-with")
	ensureRunMemoryLoopSeqIndex(t, with)
	without, withoutPath := vocabUpcastOpenStore(t, "loopseq-schema-without")
	dropRunMemoryLoopSeqIndex(t, without)
	if !runMemoryIndexExists(t, with, testRunMemoryLoopSeqIndex) || runMemoryIndexExists(t, without, testRunMemoryLoopSeqIndex) {
		t.Fatal("the two stores do not differ in the index; the comparison would prove nothing")
	}
	a := runMemorySchemaObservations(t, with, withPath)
	b := runMemorySchemaObservations(t, without, withoutPath)
	for key, want := range b {
		if got := a[key]; got != want {
			t.Errorf("schema observation %s differs with the loop_seq index:\n with:    %s\n without: %s", key, got, want)
		}
	}
	if len(a) != len(b) {
		t.Errorf("schema observation count %d with the index, %d without", len(a), len(b))
	}
	t.Logf("run_memory_entries schema observation, with and without: %s", a["dolt:texture:schema:run_memory_entries"])

	// The control that gives the test meaning, as for the kind index: an index
	// whose first column has no key yet does move a Key.
	control, controlPath := vocabUpcastOpenStore(t, "loopseq-schema-control")
	if _, err := control.db.Exec(`CREATE INDEX idx_control_model ON run_memory_entries(model, seq)`); err != nil {
		t.Fatal(err)
	}
	c := runMemorySchemaObservations(t, control, controlPath)
	if c["dolt:texture:schema:run_memory_entries"] == b["dolt:texture:schema:run_memory_entries"] {
		t.Error("an index on a keyless column left the schema observation unchanged: the test cannot see a drift")
	}
}

// 4. A fresh store is made without it, and with the covering unique key.
func TestRunMemoryLoopSeqIndexFreshStoreLacksIt(t *testing.T) {
	s := openTestStore(t)
	if runMemoryIndexExists(t, s, testRunMemoryLoopSeqIndex) {
		t.Fatalf("a fresh store has %s: the duplicate would be paid by every new computer", testRunMemoryLoopSeqIndex)
	}
	cols, unique := runMemoryIndexColumns(t, s)
	hasUnique := false
	for name, c := range cols {
		if unique[name] && reflect.DeepEqual(c, []string{"loop_id", "seq"}) {
			hasUnique = true
		}
	}
	if !hasUnique {
		t.Fatalf("a fresh store lacks UNIQUE(loop_id, seq): %v", cols)
	}
}

// 3. No boot drops it on a populated table, and no boot recreates it.
func TestRunMemoryLoopSeqIndexIsNotDroppedOrBuiltAtBoot(t *testing.T) {
	s, path := loopSeqPopulatedStore(t)
	if !runMemoryIndexExists(t, s, testRunMemoryLoopSeqIndex) {
		t.Fatal("Open dropped the loop_seq index on a populated run_memory_entries: boot-path DDL on a populated table")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.Reopen(path); err != nil {
		t.Fatal(err)
	}
	if !runMemoryIndexExists(t, s, testRunMemoryLoopSeqIndex) {
		t.Fatal("a second Open dropped the loop_seq index on a populated table")
	}
	if strings.Contains(schemaDDL, testRunMemoryLoopSeqIndex) {
		t.Fatal("schemaDDL names the loop_seq index: every boot would create it (a build on a dropped, populated table)")
	}
	// After the operation drops it, a populated reopen does not bring it back.
	if _, err := s.StartDerivedColumnMigration(); err != nil {
		t.Fatal(err)
	}
	if st := waitDerivedColumnMigration(t, s); st.State != DerivedColumnMigrationDone {
		t.Fatalf("migration %s: %s", st.State, st.Error)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.Reopen(path); err != nil {
		t.Fatal(err)
	}
	if runMemoryIndexExists(t, s, testRunMemoryLoopSeqIndex) {
		t.Fatal("a reopen after the drop recreated the loop_seq index at boot")
	}
}

// An empty table costs nothing, so a store that has the index and no rows
// drops it at open and agrees with a fresh store (as the kind index is made on
// an empty table).
func TestRunMemoryLoopSeqIndexIsDroppedAtOpenOnAnEmptyTable(t *testing.T) {
	path := testStorePath(t)
	cleanupTestStorePath(path)
	t.Cleanup(func() { cleanupTestStorePath(path) })
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	ensureRunMemoryLoopSeqIndex(t, s)
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.Reopen(path); err != nil {
		t.Fatal(err)
	}
	if runMemoryIndexExists(t, s, testRunMemoryLoopSeqIndex) {
		t.Fatal("an empty run_memory_entries kept the duplicate index at open")
	}
}

// 6 and 7. The operation drops it once, after the kind index, and the answer
// does not change.
func TestRunMemoryLoopSeqIndexMigrationDropsItOnce(t *testing.T) {
	ctx := context.Background()
	s, _ := loopSeqPopulatedStore(t)
	read := func() []string {
		t.Helper()
		var out []string
		es, err := s.ListRunMemoryEntries(ctx, "owner-1", "run-agent-03-1")
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range es {
			out = append(out, fmt.Sprintf("%s|%d", e.EntryID, e.Seq))
		}
		latest, err := s.LatestRunMemoryEntry(ctx, "run-agent-03-1")
		if err != nil {
			t.Fatal(err)
		}
		return append(out, "latest:"+latest.EntryID)
	}
	before := read()
	if len(before) < 3 {
		t.Fatalf("seed gives %d rows for the run; the comparison would prove nothing", len(before))
	}
	var rowsHash string
	if err := s.db.QueryRow(`SELECT dolt_hashof_table('run_memory_entries')`).Scan(&rowsHash); err != nil {
		t.Fatal(err)
	}
	if _, err := s.StartDerivedColumnMigration(); err != nil {
		t.Fatal(err)
	}
	done := waitDerivedColumnMigration(t, s)
	if done.State != DerivedColumnMigrationDone {
		t.Fatalf("migration %s: %s", done.State, done.Error)
	}
	if runMemoryIndexExists(t, s, testRunMemoryLoopSeqIndex) {
		t.Fatal("the migration did not drop the loop_seq index")
	}
	var names []string
	for _, step := range done.Steps {
		if step.Name == testRunMemoryLoopSeqIndex || step.Name == testRunMemoryKindIndex {
			names = append(names, step.Name+"/"+step.Kind)
			if step.Err != "" {
				t.Fatalf("step failed: %s", step.Err)
			}
		}
	}
	want := []string{testRunMemoryKindIndex + "/index", testRunMemoryLoopSeqIndex + "/drop_index"}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("steps = %v, want %v (build the kind index, then drop the duplicate)", names, want)
	}
	if !reflect.DeepEqual(before, read()) {
		t.Fatal("the answer changed with the drop")
	}
	var after string
	if err := s.db.QueryRow(`SELECT dolt_hashof_table('run_memory_entries')`).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if after != rowsHash {
		t.Logf("note: run_memory_entries row hash %s -> %s across the kind index build and the drop", rowsHash, after)
	}
	// A second run finds nothing to do for it.
	if _, err := s.StartDerivedColumnMigration(); err != nil {
		t.Fatal(err)
	}
	again := waitDerivedColumnMigration(t, s)
	for _, step := range again.Steps {
		if step.Name == testRunMemoryLoopSeqIndex {
			t.Fatalf("a second migration touched the dropped index: %+v", step)
		}
	}
	if again.State != DerivedColumnMigrationDone {
		t.Fatalf("second migration %s: %s", again.State, again.Error)
	}
}

// 5. Without the covering unique key the drop is refused, the migration says
// why, and the index stays.
func TestRunMemoryLoopSeqIndexIsNotDroppedWithoutTheUniqueKey(t *testing.T) {
	s, _ := loopSeqPopulatedStore(t)
	cols, unique := runMemoryIndexColumns(t, s)
	for name, c := range cols {
		if unique[name] && reflect.DeepEqual(c, []string{"loop_id", "seq"}) {
			if _, err := s.db.Exec("DROP INDEX `" + name + "` ON run_memory_entries"); err != nil {
				t.Fatalf("test setup cannot drop the unique key %q: %v", name, err)
			}
		}
	}
	if _, err := s.StartDerivedColumnMigration(); err != nil {
		t.Fatal(err)
	}
	done := waitDerivedColumnMigration(t, s)
	if done.State != DerivedColumnMigrationFailed || !strings.Contains(done.Error, "loop_seq") {
		t.Fatalf("migration state %q error %q, want failed naming the loop_seq index", done.State, done.Error)
	}
	if !runMemoryIndexExists(t, s, testRunMemoryLoopSeqIndex) {
		t.Fatal("the duplicate was dropped although nothing else indexes (loop_id, seq)")
	}
}

// 8. Rollback. A revert of the code leaves the dropped index dropped; the old
// binary's own schemaDDL line recreates it at the next boot (a build on the
// populated table, at boot); the recreate is also an explicit statement.
func TestRunMemoryLoopSeqIndexRollbackRecreatesIt(t *testing.T) {
	ctx := context.Background()
	s, _ := loopSeqPopulatedStore(t)
	if _, err := s.StartDerivedColumnMigration(); err != nil {
		t.Fatal(err)
	}
	if st := waitDerivedColumnMigration(t, s); st.State != DerivedColumnMigrationDone {
		t.Fatalf("migration %s: %s", st.State, st.Error)
	}
	if runMemoryIndexExists(t, s, testRunMemoryLoopSeqIndex) {
		t.Fatal("not dropped")
	}
	before, err := s.ListRunMemoryEntries(ctx, "owner-1", "run-agent-03-1")
	if err != nil {
		t.Fatal(err)
	}
	// What an older binary's boot does: its schemaDDL line, IF NOT EXISTS.
	if _, err := s.db.Exec(testRunMemoryLoopSeqDDL); err != nil {
		t.Fatalf("the old boot DDL line fails on the dropped store: %v", err)
	}
	if !runMemoryIndexExists(t, s, testRunMemoryLoopSeqIndex) {
		t.Fatal("the old boot DDL did not recreate the index")
	}
	cols, _ := runMemoryIndexColumns(t, s)
	if !reflect.DeepEqual(cols[testRunMemoryLoopSeqIndex], []string{"loop_id", "seq"}) {
		t.Fatalf("recreated index columns = %v", cols[testRunMemoryLoopSeqIndex])
	}
	after, err := s.ListRunMemoryEntries(ctx, "owner-1", "run-agent-03-1")
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("the answer changed across the recreate: %v", err)
	}
	// And the operation drops it again.
	if _, err := s.StartDerivedColumnMigration(); err != nil {
		t.Fatal(err)
	}
	if st := waitDerivedColumnMigration(t, s); st.State != DerivedColumnMigrationDone {
		t.Fatalf("migration %s: %s", st.State, st.Error)
	}
	if runMemoryIndexExists(t, s, testRunMemoryLoopSeqIndex) {
		t.Fatal("the operation did not drop the recreated index")
	}
}
