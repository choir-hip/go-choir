package store

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

// TestRunMemoryIndexDietCensus measures the run_memory_entries index diet
// (docs/problems/run-memory-entries-hold-the-engine-2026-10-23.md, "The index
// diet"): the journal per append before and after dropping the unread indexes,
// and the hold each DROP and each CREATE (the rollback's rebuild) has on the
// write connection. Off by default.
//
//	CHOIR_RUN_MEMORY_DIET=100000,425000 go test ./internal/store \
//	  -run TestRunMemoryIndexDietCensus -v -count=1 -timeout 60m
//
// It uses the journal census's meter (journal_amplification_census_test.go),
// which walks the chunk records written in a window, so the numbers are journal
// bytes, not the file system's. UUID ids and word-salad text, as there: the
// numbers are a warm local floor and differ by about 4 KiB between two windows
// of the same schema, so each schema is measured twice, alternating.
//
// The paper's schema today is the primary key, UNIQUE(loop_id, seq), and the
// five secondary indexes below. The diet drops the first three; the fourth
// (parent) is measured as a what-if only, because it moves the witness.
func TestRunMemoryIndexDietCensus(t *testing.T) {
	spec := os.Getenv("CHOIR_RUN_MEMORY_DIET")
	if spec == "" {
		t.Skip("set CHOIR_RUN_MEMORY_DIET=100000,425000 to run")
	}
	for _, f := range strings.Split(spec, ",") {
		var rows int
		if _, err := fmt.Sscan(strings.TrimSpace(f), &rows); err != nil || rows <= 0 {
			t.Fatalf("CHOIR_RUN_MEMORY_DIET %q: %v", spec, err)
		}
		t.Run(fmt.Sprintf("rows=%d", rows), func(t *testing.T) { runMemoryIndexDietCensus(t, rows) })
	}
}

var dietIndexes = []struct{ name, ddl string }{
	{"idx_run_memory_entries_loop_seq", `CREATE INDEX idx_run_memory_entries_loop_seq ON run_memory_entries(loop_id, seq)`},
	{idxOwnerLoopSeq, auditIndexDDL[idxOwnerLoopSeq]},
	{idxOwnerAgentCreate, auditIndexDDL[idxOwnerAgentCreate]},
}

func runMemoryIndexDietCensus(t *testing.T, seed int) {
	ctx := context.Background()
	s := openTestStore(t)
	m := newAmpMeter(t, s)
	g := newAmpGen("diet", 8, 1500)
	// The paper's schema today: loop_seq is not made by a fresh store.
	ampSetIndexes(t, s, "run_memory_entries", ampAllIndexes())
	ampSeedTable(t, s, "run_memory_entries", g, seed)
	n := ampN()

	cost := func(label string) {
		t0 := time.Now()
		w := ampAppendEach(t, s, m, "run_memory_entries", g, n)
		t.Logf("rows=%d %s: %s (%.1f ms an append)", seed, label, w.perOp(n), float64(time.Since(t0).Microseconds())/1000/float64(n))
	}
	// hold runs fn while a SELECT 1 on s.db is issued every 10 ms and reports
	// the whole time and the longest wait: what /health's reads would see.
	hold := func(label string, fn func() error) {
		var (
			longest time.Duration
			probes  int
			fnErr   error
		)
		done, probeDone := make(chan struct{}), make(chan struct{})
		start := time.Now()
		go func() { defer close(done); fnErr = fn() }()
		go func() {
			defer close(probeDone)
			for {
				select {
				case <-done:
					return
				default:
				}
				t1 := time.Now()
				var one int
				if err := s.db.QueryRowContext(ctx, `SELECT 1`).Scan(&one); err != nil {
					t.Error(err)
					return
				}
				if w := time.Since(t1); w > longest {
					longest = w
				}
				probes++
				time.Sleep(10 * time.Millisecond)
			}
		}()
		<-done
		<-probeDone
		if fnErr != nil {
			t.Fatalf("%s: %v", label, fnErr)
		}
		t.Logf("rows=%d HOLD %s: %v whole; %d probes, longest SELECT 1 wait %v", seed, label,
			time.Since(start).Round(time.Millisecond), probes, longest.Round(time.Millisecond))
	}
	drop := func(name string) {
		hold("DROP "+name, func() error {
			_, err := s.db.ExecContext(ctx, `DROP INDEX `+name+` ON run_memory_entries`)
			return err
		})
	}
	create := func(name, ddl string) {
		hold("CREATE "+name+" (the rollback's rebuild)", func() error {
			_, err := s.db.ExecContext(ctx, ddl)
			return err
		})
	}

	cost("today: pk, unique, loop_seq, owner_loop_seq, owner_agent_created, parent, kind (7 trees) (1)")
	for _, ix := range dietIndexes {
		drop(ix.name)
		cost("cumulative, without ..." + strings.TrimPrefix(ix.name, "idx_run_memory_entries"))
	}
	for _, ix := range dietIndexes {
		create(ix.name, ix.ddl)
	}
	cost("today (2)")
	for _, ix := range dietIndexes {
		drop(ix.name)
	}
	cost("the diet: pk, unique, parent, kind (4 trees) (2)")

	// What-if: parent too. Not built; it moves the witness.
	drop(idxParent)
	cost("what-if, parent also dropped: pk, unique, kind (3 trees)")
	create(idxParent, auditIndexDDL[idxParent])
	cost("the diet again, parent recreated (3)")

	// The production operation on the same table: the three indexes are back
	// (as the paper has them), then it drops them; its steps carry the hold.
	for _, ix := range dietIndexes {
		create(ix.name, ix.ddl)
	}
	hold("the operation (POST derived-columns?action=migrate), three drops", func() error {
		if _, err := s.StartDerivedColumnMigration(); err != nil {
			return err
		}
		st := waitDerivedColumnMigration(t, s)
		if st.State != DerivedColumnMigrationDone {
			return fmt.Errorf("migration %s: %s", st.State, st.Error)
		}
		for _, step := range st.Steps {
			t.Logf("rows=%d operation step %d/%d %s %s: %d ms", seed, step.Index, step.Total, step.Kind, step.Name, step.DurationMS)
		}
		return nil
	})
	cost("after the operation (4)")

	// The rollback: restore_run_memory_indexes, which a revert runs before the
	// old binary deploys. Its report carries each build's hold.
	hold("restore_run_memory_indexes (three builds)", func() error {
		rep, err := s.RestoreRunMemoryIndexes(ctx, nil)
		for _, st := range rep.Steps {
			t.Logf("rows=%d restore step %s %s: %d ms", seed, st.Name, st.Action, st.HoldMS)
		}
		t.Logf("rows=%d restore total hold %d ms", seed, rep.TotalHoldMS)
		return err
	})
	cost("after the restore (the paper's schema today) (5)")
	hold("restore again (idempotent)", func() error {
		rep, err := s.RestoreRunMemoryIndexes(ctx, nil)
		t.Logf("rows=%d second restore: %d steps, total hold %d ms", seed, len(rep.Steps), rep.TotalHoldMS)
		return err
	})
}
