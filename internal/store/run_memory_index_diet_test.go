package store

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// The index diet of run_memory_entries: drop the secondary indexes no
// production statement needs, as R5 dropped the duplicate loop_seq index
// (docs/problems/run-memory-entries-hold-the-engine-2026-10-23.md, "The index
// diet"). Each secondary index is one prolly-tree path per append, 9 to 13 KiB
// of journal. The candidates are idx_..._owner_loop_seq (owner_id, loop_id,
// seq) and idx_..._owner_agent_created (owner_id, agent_id, created_at); the
// third, idx_..._parent, has no reader but moves the content witness and is
// left out. Ways the change could fail, written before the code:
//
//  1. A production statement needs a candidate. The audit lists every one
//     (TestRunMemoryProductionStatementCensus fails on a statement not in the
//     list) and pins the access each has without the candidates. The residue
//     import's statement is the one that does: as written it goes from a range
//     on (owner_id, loop_id, seq) to a full table scan, so the diet rewrites it
//     to filter loop_id IN (...) first, which the unique key serves; the two
//     forms return the same rows in the same order.
//  2. An ORDER BY created_at read loses its ordering when owner_agent_created
//     goes: the no-kinds agent read must still sort on (created_at, seq) and
//     return the same rows in the same order.
//  3. The drop moves the content witness (SHOW COLUMNS Key of owner_id) when
//     the kind index is not there to carry owner_id's MUL, so the drop is
//     refused unless the kind index exists with exactly (owner_id, agent_id,
//     kind, created_at). The control store (no kind index) shows the Key move.
//  4. The drop runs at boot on a populated table, or schemaDDL still creates
//     the indexes at every boot (an undone drop, a build at boot).
//  5. A fresh store still gets them, so fresh and migrated stores differ.
//  6. The operation drops one twice, drops before the kind index exists, or
//     leaves parent alone in some runs and not others.
//  7. parent is dropped: it moves parent_entry_id's Key from MUL to empty, so a
//     live store that dropped it, a fresh store, a base and a recorded
//     checkpoint witness would disagree by vintage. Pinned as the reason it is
//     out: the observation moves, and the operation leaves it.
//  8. A rollback is unsafe or unrepeatable: the old binary's boot DDL fails on
//     the dropped store, or the answer changes across the recreate.

const (
	testOwnerLoopSeqDDL     = `CREATE INDEX IF NOT EXISTS idx_run_memory_entries_owner_loop_seq ON run_memory_entries(owner_id, loop_id, seq)`
	testOwnerAgentCreateDDL = `CREATE INDEX IF NOT EXISTS idx_run_memory_entries_owner_agent_created ON run_memory_entries(owner_id, agent_id, created_at)`
)

// dietPopulatedStore is a reopened store with the paper's index set before the
// operation runs: the duplicate loop_seq, owner_loop_seq, owner_agent_created
// and parent, and no kind index (the operation builds it first), over rows.
func dietPopulatedStore(t *testing.T) (*Store, string) {
	t.Helper()
	path := testStorePath(t)
	cleanupTestStorePath(path)
	t.Cleanup(func() { cleanupTestStorePath(path) })
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	ensureRunMemoryLoopSeqIndex(t, s)
	auditSetIndexes(t, s, map[string]bool{idxOwnerLoopSeq: true, idxOwnerAgentCreate: true, idxParent: true})
	if runMemoryIndexExists(t, s, testRunMemoryKindIndex) {
		if _, err := s.db.Exec(`DROP INDEX ` + testRunMemoryKindIndex + ` ON run_memory_entries`); err != nil {
			t.Fatal(err)
		}
	}
	auditSeed(t, s, 3000)
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

func runDietMigration(t *testing.T, s *Store) DerivedColumnMigrationStatus {
	t.Helper()
	if _, err := s.StartDerivedColumnMigration(); err != nil {
		t.Fatal(err)
	}
	return waitDerivedColumnMigration(t, s)
}

// ---- 1. the audit: every reader, and the access each has without the diet ----

var runMemorySQLRE = regexp.MustCompile(`(?i)\b(FROM|INTO|UPDATE|JOIN)\s+run_memory_entries\b`)

// expectedRunMemorySQL is the number of statements that name the table, by
// file. A new statement must be added here AND to auditStatements, with a plan.
var expectedRunMemorySQL = map[string]int{
	"internal/store/run_memory.go":     12, // 11 reads (55, 64, 108, 119, 247, 257, 290, 390, 461, 481, 491) and the insert at 134
	"internal/store/project.go":        2,  // the FOR UPDATE read (313) and the insert (330)
	"internal/store/residue_import.go": 1,  // the residue read (409)
	// SELECT 1 FROM run_memory_entries LIMIT 1: the has-rows probes of the
	// index hooks; they read the first row of the primary key and need no index.
	"internal/store/run_memory_kind_index.go": 1,
	"internal/store/run_memory_index_diet.go": 1,
}

func TestRunMemoryProductionStatementCensus(t *testing.T) {
	root := filepath.Join("..", "..")
	got := map[string]int{}
	for _, dir := range []string{"internal", "cmd"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return err
			}
			b, rerr := os.ReadFile(path)
			if rerr != nil {
				return rerr
			}
			if n := len(runMemorySQLRE.FindAll(b, -1)); n > 0 {
				rel, _ := filepath.Rel(root, path)
				got[filepath.ToSlash(rel)] = n
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	for file, n := range got {
		if want := expectedRunMemorySQL[file]; n != want {
			t.Errorf("%s names run_memory_entries in %d statements, the audit knows %d: add the new statement to auditStatements and re-run the plan audit", file, n, want)
		}
	}
	for file, want := range expectedRunMemorySQL {
		if want > 0 && got[file] != want {
			t.Errorf("%s names run_memory_entries in %d statements, the audit knows %d", file, got[file], want)
		}
	}
	if t.Failed() {
		keys := make([]string, 0, len(got))
		for k := range got {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		t.Logf("found: %v", keys)
	}
}

// The access each statement has on the diet's set (primary key, UNIQUE(loop_id,
// seq), the kind index, parent). Written from the plans of the full set.
func TestRunMemoryIndexDietEveryStatementKeepsItsAccess(t *testing.T) {
	s := openTestStore(t)
	if !runMemoryIndexExists(t, s, RunMemoryKindIndexName) {
		t.Fatal("a fresh store lacks the kind index")
	}
	auditSeed(t, s, auditRows())
	auditSetIndexes(t, s, map[string]bool{idxOwnerLoopSeq: true, idxOwnerAgentCreate: true, idxParent: true})
	full := auditPlans(t, s)
	auditSetIndexes(t, s, map[string]bool{idxParent: true})
	diet := auditPlans(t, s)

	const (
		unique = "loop_id,seq"
		kind   = "owner_id,agent_id,kind,created_at"
	)
	want := map[string]string{
		"run_memory.go:55 AppendRunMemoryEntry (tape path) seq allocation":                                                      unique,
		"run_memory.go:64 AppendRunMemoryEntry (tape path) parent":                                                              unique,
		"run_memory.go:108 AppendRunMemoryEntry seq allocation":                                                                 unique,
		"run_memory.go:119 AppendRunMemoryEntry parent":                                                                         unique,
		"run_memory.go:247 ListRunMemoryEntries (no owner)":                                                                     unique,
		"run_memory.go:257 ListRunMemoryEntries (owner)":                                                                        unique,
		"run_memory.go:390 ListRunMemoryEntryFingerprints":                                                                      "entry_id",
		"run_memory.go:461 LatestRunMemoryEntry":                                                                                unique,
		"run_memory.go:481 GetRunMemoryEntry (no owner)":                                                                        "entry_id",
		"run_memory.go:491 GetRunMemoryEntry (owner)":                                                                           "entry_id",
		"project.go:313 projectRunMemoryEntry read":                                                                             "entry_id",
		"residue_import.go:424 residue run memory (loop_id IN first, batches of 100 scopes)":                                    unique,
		"run_memory.go:290 ListRunMemoryEntriesForAgent owner, agent, one kind (the only production caller, rlm_reduce.go:520)": kind,
		"run_memory.go:290 ListRunMemoryEntriesForAgent owner, agent, two kinds":                                                kind,
		"run_memory.go:290 ListRunMemoryEntriesForAgent owner, agent, no kinds (ORDER BY created_at)":                           kind,
		"run_memory.go:290 ListRunMemoryEntriesForAgent no owner, agent, one kind (no production caller)":                       "full scan",
	}
	for where, access := range want {
		plan, ok := diet[where]
		if !ok {
			t.Errorf("auditStatements has no %q", where)
			continue
		}
		if got := planAccess(plan); got != access {
			t.Errorf("%s: access without the candidates = %q, want %q\n%s", where, got, access, plan)
		}
		if got := planAccess(full[where]); got != access && !strings.Contains(where, "no kinds") {
			t.Errorf("%s: access with the candidates = %q, want %q (the plan was already this)", where, got, access)
		}
	}
	// 2. The ORDER BY created_at read still sorts on (created_at, seq) above a
	// range on (owner_id, agent_id): same rows, same order.
	noKinds := diet["run_memory.go:290 ListRunMemoryEntriesForAgent owner, agent, no kinds (ORDER BY created_at)"]
	if !strings.Contains(noKinds, "Sort(run_memory_entries.created_at ASC, run_memory_entries.seq ASC)") {
		t.Errorf("the no-kinds agent read lost its sort on (created_at, seq):\n%s", noKinds)
	}
	if !strings.Contains(full["run_memory.go:290 ListRunMemoryEntriesForAgent owner, agent, no kinds (ORDER BY created_at)"], "Sort(run_memory_entries.created_at ASC, run_memory_entries.seq ASC)") {
		t.Error("the baseline no-kinds read does not sort on (created_at, seq): the comparison proves nothing")
	}
}

// 1. The residue import's statement as written needs owner_loop_seq: without
// it the planner scans the table. That is why the diet rewrites it.
func TestRunMemoryResidueStatementAsWrittenNeedsOwnerLoopSeq(t *testing.T) {
	s := openTestStore(t)
	auditSeed(t, s, auditRows())
	auditSetIndexes(t, s, map[string]bool{idxOwnerLoopSeq: true, idxParent: true})
	old := `SELECT e.entry_id FROM run_memory_entries e WHERE (e.owner_id=? AND e.loop_id=?) OR (e.owner_id=? AND e.loop_id=?)
		 ORDER BY e.owner_id, e.loop_id, e.seq`
	args := []any{"owner-1", "run-agent-hot-0", "owner-2", "run-agent-01-1"}
	with := planAccess(explainEventTablePlan(t, s, old, args...))
	auditSetIndexes(t, s, map[string]bool{idxParent: true})
	without := planAccess(explainEventTablePlan(t, s, old, args...))
	if with != "owner_id,loop_id,seq" || without != "full scan" {
		t.Fatalf("residue statement as written: with owner_loop_seq %q, without %q; the premise of the rewrite changed", with, without)
	}
}

// The source of the residue read carries the rewrite: loop_id IN (...) ahead
// of the owner/loop pairs, in batches, and still orders by owner, loop, seq.
func TestRunMemoryResidueStatementIsSourcedFromTheUniqueKey(t *testing.T) {
	b, err := os.ReadFile("residue_import.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	if !strings.Contains(src, "e.loop_id IN (") {
		t.Fatal("residue_import.go does not filter e.loop_id IN (...): the statement still needs owner_loop_seq")
	}
	if !strings.Contains(src, "ORDER BY e.owner_id, e.loop_id, e.seq") {
		t.Fatal("residue_import.go lost the ORDER BY e.owner_id, e.loop_id, e.seq the residue snapshot depends on")
	}
}

// The rewrite returns the same rows in the same order as the statement as
// written, on the table with owner_loop_seq (old form) and on the diet's set
// (new form), for scopes that match, scopes under the wrong owner and scopes
// that do not exist.
func TestRunMemoryResidueRewriteReturnsTheSameRows(t *testing.T) {
	s := openTestStore(t)
	auditSeed(t, s, 6000)
	auditSetIndexes(t, s, map[string]bool{idxOwnerLoopSeq: true, idxParent: true})
	scopes := [][2]string{{"owner-1", "run-agent-hot-0"}, {"owner-1", "run-agent-hot-5"}, {"owner-2", "run-agent-01-1"}, {"owner-3", "run-agent-02-2"},
		{"owner-9", "run-agent-hot-0"}, /* a loop id under the wrong owner: no rows */
		{"owner-1", "run-no-such-run"}}
	query := func(newForm bool) []string {
		t.Helper()
		clauses := make([]string, len(scopes))
		var pairArgs, loopArgs []any
		marks := make([]string, len(scopes))
		for i, sc := range scopes {
			clauses[i] = `(e.owner_id=? AND e.loop_id=?)`
			pairArgs = append(pairArgs, sc[0], sc[1])
			loopArgs = append(loopArgs, sc[1])
			marks[i] = "?"
		}
		q := `SELECT e.entry_id FROM run_memory_entries e WHERE `
		args := pairArgs
		if newForm {
			q += `e.loop_id IN (` + strings.Join(marks, ",") + `) AND (` + strings.Join(clauses, ` OR `) + `)`
			args = append(append([]any{}, loopArgs...), pairArgs...)
		} else {
			q += strings.Join(clauses, ` OR `)
		}
		q += ` ORDER BY e.owner_id, e.loop_id, e.seq`
		rows, err := s.db.Query(q, args...)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var ids []string
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				t.Fatal(err)
			}
			ids = append(ids, id)
		}
		return ids
	}
	want := query(false)
	if len(want) < 100 {
		t.Fatalf("the seed gives %d rows for the scopes; the comparison would prove nothing", len(want))
	}
	if got := query(true); !reflect.DeepEqual(want, got) {
		t.Fatalf("the rewritten form returns %d rows, the statement as written %d (or in another order)", len(got), len(want))
	}
	auditSetIndexes(t, s, map[string]bool{idxParent: true})
	if got := query(true); !reflect.DeepEqual(want, got) {
		t.Fatalf("without owner_loop_seq the rewritten form returns %d rows, want %d", len(got), len(want))
	}
}

// The production function over more runs than one batch: every entry comes
// back once, in (owner, run, seq) order, with and without owner_loop_seq.
func TestSnapshotResidueRuntimeKeepsRunMemoryOrderAcrossManyRuns(t *testing.T) {
	ctx := context.Background()
	s := openProjectStore(t)
	computerID := "computer-residue-many-runs"
	now := time.Date(2026, 8, 18, 8, 0, 0, 0, time.UTC)
	const runs = 230
	for i := 0; i < runs; i++ {
		owner := fmt.Sprintf("owner-%d", 1+i%2)
		run := types.RunRecord{RunID: fmt.Sprintf("run-many-%03d", (i*37)%runs), AgentID: "agent-many", OwnerID: owner,
			ComputerID: computerID, State: types.RunCompleted, CreatedAt: now, UpdatedAt: now}
		if err := s.CreateRunOG(ctx, run); err != nil {
			t.Fatal(err)
		}
		for seq := 1; seq <= 3; seq++ {
			if _, err := s.db.ExecContext(ctx, `INSERT INTO run_memory_entries (entry_id, loop_id, owner_id, agent_id, seq, kind, created_at)
				VALUES (?, ?, ?, ?, ?, ?, ?)`, fmt.Sprintf("e-%s-%d", run.RunID, seq), run.RunID, owner, run.AgentID, seq, "message", now); err != nil {
				t.Fatal(err)
			}
		}
	}
	rows, err := s.db.QueryContext(ctx, `SELECT entry_id FROM run_memory_entries ORDER BY owner_id, loop_id, seq`)
	if err != nil {
		t.Fatal(err)
	}
	var want []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		want = append(want, id)
	}
	_ = rows.Close()
	if len(want) != runs*3 {
		t.Fatalf("seeded %d entries, want %d", len(want), runs*3)
	}
	for _, withIndex := range []bool{true, false} {
		keep := map[string]bool{idxParent: true}
		if withIndex {
			keep[idxOwnerLoopSeq] = true
		}
		auditSetIndexes(t, s, keep)
		objects, err := s.snapshotResidueObjects(ctx, "", computerID)
		if err != nil {
			t.Fatal(err)
		}
		ops, counts, err := s.snapshotResidueRuntime(ctx, "", computerID, objects)
		if err != nil {
			t.Fatal(err)
		}
		if counts.runMemory != len(want) {
			t.Fatalf("with owner_loop_seq=%v: %d residue entries, want %d", withIndex, counts.runMemory, len(want))
		}
		var got []string
		for _, op := range ops {
			if op.Kind == computerevent.ProjectionOpRunMemoryEntry {
				got = append(got, op.CanonicalID)
			}
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("with owner_loop_seq=%v: residue run memory is not the table in (owner, run, seq) order", withIndex)
		}
	}
}

// ---- 2/3. the answers do not change -----------------------------------------

func dietReads(t *testing.T, s *Store) []string {
	t.Helper()
	ctx := context.Background()
	var out []string
	add := func(label string, ids []string) { out = append(out, label+": "+strings.Join(ids, ",")) }
	idsOf := func(es []interface{ ID() string }) []string { return nil }
	_ = idsOf
	for _, c := range []struct {
		label string
		kinds []string
		owner string
	}{
		{"agent one kind", []string{"rlm_inbox_cursor"}, "owner-1"},
		{"agent two kinds", []string{"rlm_inbox_cursor", "cell_fate"}, "owner-1"},
		{"agent no kinds", nil, "owner-1"},
	} {
		es, err := s.ListRunMemoryEntriesForAgent(ctx, c.owner, "agent-hot", c.kinds)
		if err != nil {
			t.Fatal(err)
		}
		ids := make([]string, len(es))
		for i, e := range es {
			ids[i] = e.EntryID
		}
		add(c.label, ids)
	}
	for _, run := range []string{"run-agent-hot-0", "run-agent-03-3"} {
		es, err := s.ListRunMemoryEntries(ctx, "owner-1", run)
		if err != nil {
			t.Fatal(err)
		}
		ids := make([]string, len(es))
		for i, e := range es {
			ids[i] = fmt.Sprintf("%s/%d", e.EntryID, e.Seq)
		}
		add("run "+run, ids)
		latest, err := s.LatestRunMemoryEntry(ctx, run)
		if err == nil {
			add("latest "+run, []string{latest.EntryID})
		}
	}
	return out
}

func TestRunMemoryIndexDietKeepsEveryAnswer(t *testing.T) {
	s, _ := dietPopulatedStore(t)
	before := dietReads(t, s)
	for _, line := range before {
		if strings.HasSuffix(line, ": ") {
			t.Fatalf("a read returned nothing to compare: %q", line)
		}
	}
	st := runDietMigration(t, s)
	if st.State != DerivedColumnMigrationDone {
		t.Fatalf("migration %s: %s", st.State, st.Error)
	}
	for _, name := range []string{idxOwnerLoopSeq, idxOwnerAgentCreate} {
		if runMemoryIndexExists(t, s, name) {
			t.Fatalf("%s survived the operation", name)
		}
	}
	after := dietReads(t, s)
	if !reflect.DeepEqual(before, after) {
		for i := range before {
			if before[i] != after[i] {
				t.Errorf("answer %d changed with the diet:\n before %.200s\n after  %.200s", i, before[i], after[i])
			}
		}
	}
}

// ---- 3/7. the witness -------------------------------------------------------

func columnKeys(t *testing.T, observation string) map[string]string {
	t.Helper()
	var cols []struct {
		Field string `json:"field"`
		Key   string `json:"key"`
	}
	if err := json.Unmarshal([]byte(observation), &cols); err != nil {
		t.Fatalf("schema observation is not the column list: %v\n%s", err, observation)
	}
	keys := map[string]string{}
	for _, c := range cols {
		keys[c.Field] = c.Key
	}
	return keys
}

// A store with the full set and a store on the diet's set (kind index kept)
// have equal schema observations for every table: owner_id stays MUL through
// the kind index, loop_id through UNIQUE(loop_id, seq).
func TestRunMemoryIndexDietKeepsTheWitnessSchemaObservation(t *testing.T) {
	with, withPath := vocabUpcastOpenStore(t, "diet-schema-with")
	auditSetIndexes(t, with, map[string]bool{idxOwnerLoopSeq: true, idxOwnerAgentCreate: true, idxParent: true})
	ensureRunMemoryLoopSeqIndex(t, with)
	without, withoutPath := vocabUpcastOpenStore(t, "diet-schema-without")
	auditSetIndexes(t, without, map[string]bool{idxParent: true})
	dropRunMemoryLoopSeqIndex(t, without)
	if !runMemoryIndexExists(t, with, idxOwnerLoopSeq) || runMemoryIndexExists(t, without, idxOwnerLoopSeq) ||
		!runMemoryIndexExists(t, without, testRunMemoryKindIndex) {
		t.Fatal("the two stores do not differ as the test assumes; the comparison would prove nothing")
	}
	a := runMemorySchemaObservations(t, with, withPath)
	b := runMemorySchemaObservations(t, without, withoutPath)
	for key, want := range b {
		if got := a[key]; got != want {
			t.Errorf("schema observation %s differs across the diet:\n full: %s\n diet: %s", key, got, want)
		}
	}
	keys := columnKeys(t, a["dolt:texture:schema:run_memory_entries"])
	if keys["owner_id"] != "MUL" || keys["loop_id"] != "MUL" || keys["parent_entry_id"] != "MUL" || keys["agent_id"] != "" {
		t.Errorf("Keys on the diet's set: %v; want owner_id, loop_id, parent_entry_id MUL and agent_id empty", keys)
	}
}

// The control that gives the guard its meaning: the same drops without the
// kind index move owner_id's Key, so a drop before the kind index exists would
// make a live store differ from a fresh one.
func TestRunMemoryIndexDietWithoutTheKindIndexMovesTheWitness(t *testing.T) {
	full, fullPath := vocabUpcastOpenStore(t, "diet-control-full")
	auditSetIndexes(t, full, map[string]bool{idxOwnerLoopSeq: true, idxOwnerAgentCreate: true, idxParent: true})
	control, controlPath := vocabUpcastOpenStore(t, "diet-control-nokind")
	auditSetIndexes(t, control, map[string]bool{idxParent: true})
	if _, err := control.db.Exec(`DROP INDEX ` + testRunMemoryKindIndex + ` ON run_memory_entries`); err != nil {
		t.Fatal(err)
	}
	a := runMemorySchemaObservations(t, full, fullPath)
	c := runMemorySchemaObservations(t, control, controlPath)
	if a["dolt:texture:schema:run_memory_entries"] == c["dolt:texture:schema:run_memory_entries"] {
		t.Fatal("dropping the owner-leading indexes and the kind index left the observation unchanged: the guard has nothing to guard")
	}
	if got := columnKeys(t, c["dolt:texture:schema:run_memory_entries"])["owner_id"]; got != "" {
		t.Fatalf("owner_id Key without any owner-leading index = %q, want empty", got)
	}
}

// 7. Why parent is out: dropping it moves parent_entry_id's Key from MUL to
// empty, so a store that dropped it and one that did not hash different
// schemas for the same state (replay eligibility's SchemaDrift, a base's
// recorded witness, a checkpoint's recorded witness).
func TestRunMemoryParentIndexDropMovesTheWitnessSchemaObservation(t *testing.T) {
	with, withPath := vocabUpcastOpenStore(t, "parent-schema-with")
	without, withoutPath := vocabUpcastOpenStore(t, "parent-schema-without")
	if _, err := without.db.Exec(`DROP INDEX ` + idxParent + ` ON run_memory_entries`); err != nil {
		t.Fatal(err)
	}
	a := runMemorySchemaObservations(t, with, withPath)["dolt:texture:schema:run_memory_entries"]
	b := runMemorySchemaObservations(t, without, withoutPath)["dolt:texture:schema:run_memory_entries"]
	if a == b {
		t.Fatal("dropping the parent index left the observation unchanged: it can be dropped like loop_seq, the record's reason for leaving it out is gone")
	}
	ka, kb := columnKeys(t, a), columnKeys(t, b)
	if ka["parent_entry_id"] != "MUL" || kb["parent_entry_id"] != "" {
		t.Fatalf("parent_entry_id Key with the index %q, without %q; want MUL and empty", ka["parent_entry_id"], kb["parent_entry_id"])
	}
	for f, k := range ka {
		if f != "parent_entry_id" && kb[f] != k {
			t.Errorf("column %s Key moved too: %q -> %q", f, k, kb[f])
		}
	}
}

// ---- 4/5. boot, fresh stores ------------------------------------------------

func TestRunMemoryIndexDietFreshStoreLacksTheCandidatesAndKeepsParent(t *testing.T) {
	s := openTestStore(t)
	for _, name := range []string{idxOwnerLoopSeq, idxOwnerAgentCreate} {
		if runMemoryIndexExists(t, s, name) {
			t.Errorf("a fresh store has %s: every new computer would pay its tree", name)
		}
		if strings.Contains(schemaDDL, name) {
			t.Errorf("schemaDDL names %s: every boot would create it", name)
		}
	}
	if !runMemoryIndexExists(t, s, idxParent) || !strings.Contains(schemaDDL, idxParent) {
		t.Error("parent left the fresh schema: it moves the witness and is out of the diet")
	}
	if !runMemoryIndexExists(t, s, testRunMemoryKindIndex) {
		t.Error("a fresh store lacks the kind index, which the diet relies on")
	}
}

// An empty table costs nothing: a store with the old indexes and no rows drops
// them at open and agrees with a fresh store.
func TestRunMemoryIndexDietDropsFromAnEmptyTableAtOpen(t *testing.T) {
	path := testStorePath(t)
	cleanupTestStorePath(path)
	t.Cleanup(func() { cleanupTestStorePath(path) })
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	auditSetIndexes(t, s, map[string]bool{idxOwnerLoopSeq: true, idxOwnerAgentCreate: true, idxParent: true})
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.Reopen(path); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{idxOwnerLoopSeq, idxOwnerAgentCreate} {
		if runMemoryIndexExists(t, s, name) {
			t.Errorf("an empty run_memory_entries kept %s at open", name)
		}
	}
	if !runMemoryIndexExists(t, s, idxParent) || !runMemoryIndexExists(t, s, testRunMemoryKindIndex) {
		t.Error("open dropped parent or lost the kind index on the empty table")
	}
}

// 4. No boot drops them on a populated table, and none recreates them after
// the operation.
func TestRunMemoryIndexDietIsNotDroppedOrBuiltAtBoot(t *testing.T) {
	s, path := dietPopulatedStore(t)
	for _, name := range []string{idxOwnerLoopSeq, idxOwnerAgentCreate} {
		if !runMemoryIndexExists(t, s, name) {
			t.Fatalf("Open dropped %s on a populated run_memory_entries: boot-path DDL on a populated table", name)
		}
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.Reopen(path); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{idxOwnerLoopSeq, idxOwnerAgentCreate} {
		if !runMemoryIndexExists(t, s, name) {
			t.Fatalf("a second Open dropped %s on a populated table", name)
		}
	}
	if st := runDietMigration(t, s); st.State != DerivedColumnMigrationDone {
		t.Fatalf("migration %s: %s", st.State, st.Error)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.Reopen(path); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{idxOwnerLoopSeq, idxOwnerAgentCreate} {
		if runMemoryIndexExists(t, s, name) {
			t.Fatalf("a reopen after the drop recreated %s at boot", name)
		}
	}
}

// ---- 6. the operation -------------------------------------------------------

func TestRunMemoryIndexDietOperationDropsEachOnceAfterTheKindIndex(t *testing.T) {
	s, _ := dietPopulatedStore(t)
	done := runDietMigration(t, s)
	if done.State != DerivedColumnMigrationDone {
		t.Fatalf("migration %s: %s", done.State, done.Error)
	}
	var names []string
	watched := map[string]bool{testRunMemoryKindIndex: true, testRunMemoryLoopSeqIndex: true, idxOwnerLoopSeq: true, idxOwnerAgentCreate: true, idxParent: true}
	for _, step := range done.Steps {
		if watched[step.Name] {
			names = append(names, step.Name+"/"+step.Kind)
			if step.Err != "" {
				t.Fatalf("step failed: %s", step.Err)
			}
		}
	}
	want := []string{
		testRunMemoryKindIndex + "/index",
		testRunMemoryLoopSeqIndex + "/drop_index",
		idxOwnerLoopSeq + "/drop_index",
		idxOwnerAgentCreate + "/drop_index",
	}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("steps = %v, want %v (the kind index first, then the three drops; parent never)", names, want)
	}
	if !runMemoryIndexExists(t, s, idxParent) {
		t.Fatal("the operation dropped parent: it moves the witness and is out of the diet")
	}
	if !runMemoryIndexExists(t, s, testRunMemoryKindIndex) {
		t.Fatal("the operation lost the kind index")
	}
	// The table's structure is the fresh store's.
	cols, unique := runMemoryIndexColumns(t, s)
	fresh := openTestStore(t)
	freshCols, freshUnique := runMemoryIndexColumns(t, fresh)
	if !reflect.DeepEqual(cols, freshCols) || !reflect.DeepEqual(unique, freshUnique) {
		t.Fatalf("migrated indexes %v differ from a fresh store's %v", cols, freshCols)
	}
	// A second run finds nothing to do for any of them.
	again := runDietMigration(t, s)
	for _, step := range again.Steps {
		if watched[step.Name] {
			t.Fatalf("a second migration touched %s: %+v", step.Name, step)
		}
	}
	if again.State != DerivedColumnMigrationDone {
		t.Fatalf("second migration %s: %s", again.State, again.Error)
	}
}

// 3. The guard: the owner-leading drops are refused unless the kind index
// exists with exactly (owner_id, agent_id, kind, created_at). A kind index of
// the first-pass order (agent first) is present by name and does not count.
func TestRunMemoryIndexDietIsRefusedWithoutTheKindIndexColumns(t *testing.T) {
	s, _ := dietPopulatedStore(t)
	if _, err := s.db.Exec(`CREATE INDEX ` + testRunMemoryKindIndex + ` ON run_memory_entries(agent_id, kind, owner_id, created_at)`); err != nil {
		t.Fatal(err)
	}
	done := runDietMigration(t, s)
	if done.State != DerivedColumnMigrationFailed || !strings.Contains(done.Error, testRunMemoryKindIndex) {
		t.Fatalf("migration state %q error %q, want failed naming the kind index", done.State, done.Error)
	}
	for _, name := range []string{idxOwnerLoopSeq, idxOwnerAgentCreate} {
		if !runMemoryIndexExists(t, s, name) {
			t.Fatalf("%s was dropped although the kind index would not carry owner_id's Key and the agent read", name)
		}
	}
}

// 1. Without the covering unique key nothing is dropped.
func TestRunMemoryIndexDietIsRefusedWithoutTheUniqueKey(t *testing.T) {
	s, _ := dietPopulatedStore(t)
	// R5's own step would refuse first; take the duplicate out so only the
	// diet's guard is under test.
	dropRunMemoryLoopSeqIndex(t, s)
	cols, unique := runMemoryIndexColumns(t, s)
	for name, c := range cols {
		if unique[name] && reflect.DeepEqual(c, []string{"loop_id", "seq"}) {
			if _, err := s.db.Exec("DROP INDEX `" + name + "` ON run_memory_entries"); err != nil {
				t.Fatalf("test setup cannot drop the unique key %q: %v", name, err)
			}
		}
	}
	done := runDietMigration(t, s)
	if done.State != DerivedColumnMigrationFailed {
		t.Fatalf("migration state %q, want failed", done.State)
	}
	for _, name := range []string{idxOwnerLoopSeq, idxOwnerAgentCreate} {
		if !runMemoryIndexExists(t, s, name) {
			t.Fatalf("%s was dropped although nothing else indexes (loop_id, seq)", name)
		}
	}
}

// ---- 8. rollback ------------------------------------------------------------

// A revert of the code leaves the dropped indexes dropped; the old binary's own
// schemaDDL lines (IF NOT EXISTS) rebuild them at its next boot, on the
// populated table, before its listener. The recreate is also an explicit
// statement the operator can run first.
func TestRunMemoryIndexDietRollbackRecreatesThem(t *testing.T) {
	s, _ := dietPopulatedStore(t)
	if st := runDietMigration(t, s); st.State != DerivedColumnMigrationDone {
		t.Fatalf("migration %s: %s", st.State, st.Error)
	}
	before := dietReads(t, s)
	for _, c := range []struct {
		name, ddl string
		cols      []string
	}{
		{idxOwnerLoopSeq, testOwnerLoopSeqDDL, []string{"owner_id", "loop_id", "seq"}},
		{idxOwnerAgentCreate, testOwnerAgentCreateDDL, []string{"owner_id", "agent_id", "created_at"}},
	} {
		start := time.Now()
		if _, err := s.db.Exec(c.ddl); err != nil {
			t.Fatalf("the old boot DDL line fails on the dropped store: %v", err)
		}
		t.Logf("%s rebuilt in %v on %d rows", c.name, time.Since(start).Round(time.Millisecond), 3000)
		if !runMemoryIndexExists(t, s, c.name) {
			t.Fatalf("the old boot DDL did not recreate %s", c.name)
		}
		cols, _ := runMemoryIndexColumns(t, s)
		if !reflect.DeepEqual(cols[c.name], c.cols) {
			t.Fatalf("recreated %s columns = %v, want %v", c.name, cols[c.name], c.cols)
		}
	}
	if after := dietReads(t, s); !reflect.DeepEqual(before, after) {
		t.Fatal("the answers changed across the recreate")
	}
	if st := runDietMigration(t, s); st.State != DerivedColumnMigrationDone {
		t.Fatalf("migration %s: %s", st.State, st.Error)
	}
	for _, name := range []string{idxOwnerLoopSeq, idxOwnerAgentCreate} {
		if runMemoryIndexExists(t, s, name) {
			t.Fatalf("the operation did not drop the recreated %s", name)
		}
	}
}
