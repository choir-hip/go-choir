package store

import (
	"bytes"
	"context"
	"errors"
	"log"
	"reflect"
	"strings"
	"sync"
	"testing"
)

// The restore half of the index diet: POST /internal/store/derived-columns?
// action=restore_run_memory_indexes recreates, online, the indexes R5 and the
// diet dropped, so that a revert of the binary does not rebuild them at boot on
// the populated table (the old schemaDDL's CREATE INDEX IF NOT EXISTS is then a
// no-op). Ways it could fail, written before the code:
//
//  1. The restored store differs from the pre-diet store: another index set, a
//     schema observation that moved, a plan or an answer that changed.
//  2. It recreates in the wrong order, so owner_id's Key (MUL, carried by the
//     kind index alone after the diet) can move in between: owner_agent_created
//     goes first.
//  3. A second restore, or one on a store that never dieted, builds an index
//     again (a hold for nothing) or reports nothing.
//  4. A reverted binary's boot DDL still builds something after a restore.
//  5. It runs beside a migration and the two race (a drop and a create of the
//     same index).
//  6. It reports no per-index hold, so the operator cannot size a retry.
//  7. An unknown index name is accepted and silently ignored.
//  8. Open says nothing when the store has neither the kind index nor
//     owner_agent_created: owner_id has no leading index, the agent read is a
//     full scan and the witness differs from a fresh store's.

// dietRestoreRef is the state the paper has today and a restore must return to:
// every legacy index and the kind index, over the same rows.
func dietRestoreRef(t *testing.T) (*Store, string) {
	t.Helper()
	s, path := dietPopulatedStore(t)
	if _, err := s.db.Exec(runMemoryKindIndexDDL); err != nil {
		t.Fatal(err)
	}
	return s, path
}

func restoreStepsOf(rep RunMemoryIndexRestoreReport) (created, present []string) {
	for _, st := range rep.Steps {
		switch st.Action {
		case RunMemoryIndexRestoreCreated:
			created = append(created, st.Name)
		case RunMemoryIndexRestorePresent:
			present = append(present, st.Name)
		}
	}
	return created, present
}

var restoreOrder = []string{RunMemoryOwnerAgentCreatedIndexName, RunMemoryOwnerLoopSeqIndexName, RunMemoryLoopSeqIndexName}

// 1, 2, 6. After the diet, a restore gives the pre-diet store back.
func TestRunMemoryIndexRestoreAfterTheDietEqualsThePreDietStore(t *testing.T) {
	ctx := context.Background()
	ref, refPath := dietRestoreRef(t)
	refCols, refUnique := runMemoryIndexColumns(t, ref)
	refPlans := auditPlans(t, ref)
	refReads := dietReads(t, ref)

	s, path := dietPopulatedStore(t)
	if st := runDietMigration(t, s); st.State != DerivedColumnMigrationDone {
		t.Fatalf("migration %s: %s", st.State, st.Error)
	}
	for _, name := range restoreOrder {
		if runMemoryIndexExists(t, s, name) {
			t.Fatalf("%s survived the diet; the restore would prove nothing", name)
		}
	}
	rep, err := s.RestoreRunMemoryIndexes(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	created, present := restoreStepsOf(rep)
	if !reflect.DeepEqual(created, restoreOrder) || len(present) != 0 {
		t.Fatalf("restore created %v and found %v, want %v created in that order (owner_agent_created first: it carries owner_id's Key when the kind index is the only other owner index)", created, present, restoreOrder)
	}
	for _, st := range rep.Steps {
		if st.HoldMS < 0 || st.Err != "" {
			t.Fatalf("step %+v", st)
		}
	}
	if rep.TotalHoldMS < 0 {
		t.Fatalf("total hold %d", rep.TotalHoldMS)
	}
	cols, unique := runMemoryIndexColumns(t, s)
	if !reflect.DeepEqual(cols, refCols) || !reflect.DeepEqual(unique, refUnique) {
		t.Fatalf("restored indexes %v differ from the pre-diet store's %v", cols, refCols)
	}
	plans := auditPlans(t, s)
	for where, want := range refPlans {
		if plans[where] != want {
			t.Errorf("%s plans differently after the restore:\nwant:\n%s\ngot:\n%s", where, want, plans[where])
		}
	}
	if got := dietReads(t, s); !reflect.DeepEqual(got, refReads) {
		t.Error("the answers differ after the restore")
	}
	a := runMemorySchemaObservations(t, ref, refPath)
	b := runMemorySchemaObservations(t, s, path)
	for key, want := range a {
		if b[key] != want {
			t.Errorf("schema observation %s differs after the restore:\n pre-diet: %s\n restored: %s", key, want, b[key])
		}
	}
}

// 3. A second restore builds nothing.
func TestRunMemoryIndexRestoreTwiceIsANoOp(t *testing.T) {
	ctx := context.Background()
	s, _ := dietPopulatedStore(t)
	if st := runDietMigration(t, s); st.State != DerivedColumnMigrationDone {
		t.Fatalf("migration %s: %s", st.State, st.Error)
	}
	if _, err := s.RestoreRunMemoryIndexes(ctx, nil); err != nil {
		t.Fatal(err)
	}
	var before string
	if err := s.db.QueryRow(`SELECT dolt_hashof_table('run_memory_entries')`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	rep, err := s.RestoreRunMemoryIndexes(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	created, present := restoreStepsOf(rep)
	if len(created) != 0 || !reflect.DeepEqual(present, restoreOrder) {
		t.Fatalf("second restore created %v, found %v; want nothing created and all three present", created, present)
	}
	var after string
	if err := s.db.QueryRow(`SELECT dolt_hashof_table('run_memory_entries')`).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatal("a second restore changed the table")
	}
}

// 3. A store that never dieted (every legacy index present) has nothing to
// restore.
func TestRunMemoryIndexRestoreOnAStoreThatNeverDietedIsANoOp(t *testing.T) {
	s, _ := dietRestoreRef(t)
	rep, err := s.RestoreRunMemoryIndexes(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	created, present := restoreStepsOf(rep)
	if len(created) != 0 || len(present) != 3 {
		t.Fatalf("restore on the pre-diet store created %v, found %v", created, present)
	}
}

// 4. After a restore an old binary's boot DDL builds nothing: its lines are
// IF NOT EXISTS over indexes that exist, the table is untouched.
func TestRunMemoryIndexRestoreLeavesAnOldBootDDLANoOp(t *testing.T) {
	s, _ := dietPopulatedStore(t)
	if st := runDietMigration(t, s); st.State != DerivedColumnMigrationDone {
		t.Fatalf("migration %s: %s", st.State, st.Error)
	}
	if _, err := s.RestoreRunMemoryIndexes(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	var before string
	if err := s.db.QueryRow(`SELECT dolt_hashof_table('run_memory_entries')`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	for _, ddl := range []string{testRunMemoryLoopSeqDDL, testOwnerLoopSeqDDL, testOwnerAgentCreateDDL,
		`CREATE INDEX IF NOT EXISTS idx_run_memory_entries_parent ON run_memory_entries(parent_entry_id)`} {
		if _, err := s.db.Exec(ddl); err != nil {
			t.Fatalf("the old boot DDL fails after a restore: %v\n%s", err, ddl)
		}
	}
	var after string
	if err := s.db.QueryRow(`SELECT dolt_hashof_table('run_memory_entries')`).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatal("the old boot DDL changed the table after a restore: it built an index")
	}
	rep, err := s.RestoreRunMemoryIndexes(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if created, _ := restoreStepsOf(rep); len(created) != 0 {
		t.Fatalf("a restore after the old boot DDL created %v", created)
	}
}

// The indexes argument restores a subset (a revert of the diet but not of R5
// does not need loop_seq), and an unknown name is refused before anything runs.
func TestRunMemoryIndexRestoreTakesASubsetAndRefusesUnknownNames(t *testing.T) {
	ctx := context.Background()
	s, _ := dietPopulatedStore(t)
	if st := runDietMigration(t, s); st.State != DerivedColumnMigrationDone {
		t.Fatalf("migration %s: %s", st.State, st.Error)
	}
	if _, err := s.RestoreRunMemoryIndexes(ctx, []string{RunMemoryOwnerAgentCreatedIndexName, "idx_nope"}); err == nil ||
		!strings.Contains(err.Error(), "idx_nope") {
		t.Fatalf("unknown index error = %v, want it to name idx_nope", err)
	}
	if runMemoryIndexExists(t, s, RunMemoryOwnerAgentCreatedIndexName) {
		t.Fatal("a refused restore still built an index")
	}
	rep, err := s.RestoreRunMemoryIndexes(ctx, []string{RunMemoryOwnerAgentCreatedIndexName, RunMemoryOwnerLoopSeqIndexName})
	if err != nil {
		t.Fatal(err)
	}
	created, _ := restoreStepsOf(rep)
	if !reflect.DeepEqual(created, restoreOrder[:2]) {
		t.Fatalf("subset restore created %v, want %v", created, restoreOrder[:2])
	}
	if runMemoryIndexExists(t, s, RunMemoryLoopSeqIndexName) {
		t.Fatal("the subset restore built loop_seq")
	}
}

// 5. A restore does not run beside a migration, and a migration does not
// start beside a restore.
func TestRunMemoryIndexRestoreRefusesWhileAMigrationRuns(t *testing.T) {
	s, _ := dietPopulatedStore(t)
	release := make(chan struct{})
	derivedColumnStepHook = func() { <-release }
	t.Cleanup(func() { derivedColumnStepHook = nil })
	if _, err := s.StartDerivedColumnMigration(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RestoreRunMemoryIndexes(context.Background(), nil); !errors.Is(err, ErrDerivedColumnMigrationRunning) {
		t.Fatalf("restore during a migration = %v, want ErrDerivedColumnMigrationRunning", err)
	}
	close(release)
	if st := waitDerivedColumnMigration(t, s); st.State != DerivedColumnMigrationDone {
		t.Fatalf("migration %s: %s", st.State, st.Error)
	}
}

// 8. Open says so, loudly, when neither the kind index nor owner_agent_created
// exists, and says nothing in any state that has one of them.
func TestRunMemoryOpenWarnsWhenNoOwnerAgentIndexExists(t *testing.T) {
	capture := func() (*bytes.Buffer, func()) {
		var buf bytes.Buffer
		prev := log.Writer()
		log.SetOutput(&buf)
		return &buf, func() { log.SetOutput(prev) }
	}
	const marker = "run-memory: no (owner_id, agent_id) index"

	// The dangerous state: populated, kind index not built, owner_agent_created gone.
	s, path := dietPopulatedStore(t)
	if _, err := s.db.Exec(`DROP INDEX ` + RunMemoryOwnerAgentCreatedIndexName + ` ON run_memory_entries`); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	buf, restore := capture()
	if err := s.Reopen(path); err != nil {
		restore()
		t.Fatal(err)
	}
	restore()
	if !strings.Contains(buf.String(), marker) {
		t.Fatalf("Open logged nothing loud for a store with neither the kind index nor owner_agent_created:\n%s", buf.String())
	}

	// Quiet states: the pre-diet store (owner_agent_created, no kind index yet),
	// and a fresh dieted store (the kind index only).
	for name, mk := range map[string]func() (*Store, string){
		"pre-diet populated": func() (*Store, string) { return dietPopulatedStore(t) },
		"fresh": func() (*Store, string) {
			p := testStorePath(t)
			cleanupTestStorePath(p)
			t.Cleanup(func() { cleanupTestStorePath(p) })
			st, err := Open(p)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = st.Close() })
			return st, p
		},
	} {
		q, p := mk()
		if err := q.Close(); err != nil {
			t.Fatal(err)
		}
		b, restore := capture()
		err := q.Reopen(p)
		restore()
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(b.String(), marker) {
			t.Errorf("%s: Open warned although an (owner_id, agent_id) index exists:\n%s", name, b.String())
		}
	}
}

// The status after a restore. Ways it could fail, written before the code:
//
//  1. Stale after success: the migration's last status says the dieted indexes
//     are absent (present=false), and the restore never refreshes it, so GET
//     contradicts the rollback step that says to read it after the restore.
//  2. Stale after a failure: a restore that built the first index and failed on
//     the second leaves the first one reported absent.
//  3. A refresh that fails (a probe error) clobbers the last good status.
//  4. restoring stays true after the restore returns (a leaked flag refuses
//     every later migration), or is never true while one runs.
//  5. A concurrent status read during the build sees a torn status: the refresh
//     is published under m.mu together with restoring=false, and the database
//     probes run outside m.mu so a GET is never blocked on them.

func dietIndexPresence(st DerivedColumnMigrationStatus) map[string]bool {
	out := map[string]bool{}
	for _, d := range st.RunMemoryDietIndexes {
		out[d.Name] = d.Present
	}
	if st.RunMemoryLoopSeqIndex != nil {
		out[RunMemoryLoopSeqIndexName] = st.RunMemoryLoopSeqIndex.Present
	}
	return out
}

// 1, 4. migrate -> restore -> status shows every restored index present.
func TestRunMemoryIndexRestoreRefreshesTheMigrationStatus(t *testing.T) {
	s, _ := dietPopulatedStore(t)
	done := runDietMigration(t, s)
	if done.State != DerivedColumnMigrationDone {
		t.Fatalf("migration %s: %s", done.State, done.Error)
	}
	for name, present := range dietIndexPresence(done) {
		if present {
			t.Fatalf("after the migration %s is still reported present", name)
		}
	}
	if done.Restoring {
		t.Fatal("restoring is true with no restore running")
	}
	if _, err := s.RestoreRunMemoryIndexes(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	st := s.DerivedColumnMigrationStatus(context.Background())
	if st.State != DerivedColumnMigrationDone {
		t.Fatalf("a restore changed the migration state to %s", st.State)
	}
	if st.Restoring {
		t.Fatal("restoring is still true after the restore returned")
	}
	got := dietIndexPresence(st)
	want := map[string]bool{RunMemoryOwnerLoopSeqIndexName: true, RunMemoryOwnerAgentCreatedIndexName: true, RunMemoryLoopSeqIndexName: true}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("status after migrate then restore reports %v, want %v", got, want)
	}
	if st.RunMemoryKindIndex == nil || !st.RunMemoryKindIndex.Present {
		t.Fatalf("kind index status after a restore = %+v", st.RunMemoryKindIndex)
	}
	// The migration can run again, and its status goes back to absent.
	again := runDietMigration(t, s)
	if again.State != DerivedColumnMigrationDone {
		t.Fatalf("second migration %s: %s", again.State, again.Error)
	}
	for name, present := range dietIndexPresence(again) {
		if present {
			t.Fatalf("after the second migration %s is reported present", name)
		}
	}
}

// 1. A restore of a subset refreshes the whole set from the inventory.
func TestRunMemoryIndexRestoreSubsetRefreshesOnlyWhatExists(t *testing.T) {
	s, _ := dietPopulatedStore(t)
	if st := runDietMigration(t, s); st.State != DerivedColumnMigrationDone {
		t.Fatalf("migration %s: %s", st.State, st.Error)
	}
	if _, err := s.RestoreRunMemoryIndexes(context.Background(), []string{RunMemoryOwnerAgentCreatedIndexName}); err != nil {
		t.Fatal(err)
	}
	got := dietIndexPresence(s.DerivedColumnMigrationStatus(context.Background()))
	want := map[string]bool{RunMemoryOwnerLoopSeqIndexName: false, RunMemoryOwnerAgentCreatedIndexName: true, RunMemoryLoopSeqIndexName: false}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("status after a subset restore = %v, want %v", got, want)
	}
}

// 2. A restore that fails part-way still reports what it built: the hook
// cancels the context after the first index, so the second step fails.
func TestRunMemoryIndexRestoreRefreshesTheStatusAfterAFailure(t *testing.T) {
	s, _ := dietPopulatedStore(t)
	if st := runDietMigration(t, s); st.State != DerivedColumnMigrationDone {
		t.Fatalf("migration %s: %s", st.State, st.Error)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runMemoryRestoreStepHook = func(step int) {
		if step == 1 {
			cancel()
		}
	}
	t.Cleanup(func() { runMemoryRestoreStepHook = nil })
	rep, err := s.RestoreRunMemoryIndexes(ctx, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("restore error = %v, want context.Canceled", err)
	}
	if created, _ := restoreStepsOf(rep); !reflect.DeepEqual(created, restoreOrder[:1]) {
		t.Fatalf("restore built %v before the cancel, want %v", created, restoreOrder[:1])
	}
	st := s.DerivedColumnMigrationStatus(context.Background())
	if st.Restoring {
		t.Fatal("restoring leaked after a failed restore")
	}
	got := dietIndexPresence(st)
	want := map[string]bool{RunMemoryOwnerLoopSeqIndexName: false, RunMemoryOwnerAgentCreatedIndexName: true, RunMemoryLoopSeqIndexName: false}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("status after a failed restore = %v, want %v (what it built is reported)", got, want)
	}
}

// 4, 5. restoring is true while a restore runs, a status read does not wait
// for it, a migration is refused beside it, and the flag clears with the
// refresh.
func TestRunMemoryIndexRestoreReportsRestoringWhileItRuns(t *testing.T) {
	s, _ := dietPopulatedStore(t)
	if st := runDietMigration(t, s); st.State != DerivedColumnMigrationDone {
		t.Fatalf("migration %s: %s", st.State, st.Error)
	}
	reached := make(chan struct{})
	release := make(chan struct{})
	runMemoryRestoreStepHook = func(step int) {
		if step == 0 {
			close(reached)
			<-release
		}
	}
	t.Cleanup(func() { runMemoryRestoreStepHook = nil })
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	// A t.Fatal below still unblocks the restore goroutine.
	t.Cleanup(unblock)
	errc := make(chan error, 1)
	go func() {
		_, err := s.RestoreRunMemoryIndexes(context.Background(), nil)
		errc <- err
	}()
	<-reached
	st := s.DerivedColumnMigrationStatus(context.Background())
	if !st.Restoring || st.State != DerivedColumnMigrationDone {
		t.Fatalf("status during a restore: restoring=%t state=%s, want restoring over the last migration state", st.Restoring, st.State)
	}
	if _, err := s.StartDerivedColumnMigration(); !errors.Is(err, ErrDerivedColumnMigrationRunning) {
		t.Fatalf("migration beside a restore = %v, want ErrDerivedColumnMigrationRunning", err)
	}
	unblock()
	if err := <-errc; err != nil {
		t.Fatal(err)
	}
	if st := s.DerivedColumnMigrationStatus(context.Background()); st.Restoring {
		t.Fatal("restoring leaked after the restore")
	}
}
