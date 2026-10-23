package store

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestRunMemoryHoldCensus is the repeatable artifact of
// docs/problems/run-memory-entries-hold-the-engine-2026-10-23.md. It measures
// ListRunMemoryEntriesForAgent, the function the paper's store_stalled lines
// name most often, on a local store seeded to a size: its plan, its time and
// allocation as the desk's rows grow, how long it holds the write pool's one
// connection (a concurrent trivial read on that pool waits that long), and
// what two read shapes would cost instead (a narrow projection, and a
// (owner_id, agent_id, kind, created_at) index made on the scratch store only).
// Off by default (it writes tens of thousands of rows). Run it with
//
//	CHOIR_RUN_MEMORY_HOLD=20000:2000,100000:10000 go test ./internal/store -run TestRunMemoryHoldCensus -v -count=1 -timeout 60m
//
// Each size is TOTAL:HOT: TOTAL rows in run_memory_entries for one owner, HOT
// of them belonging to the measured desk agent ("agent-hot", spread over runs
// of 250 rows), the rest to 40 other agents. Kinds: 55% message (1.5 KB
// message_json), 20% cell_fate, 20% rlm_inbox_cursor, 5% compaction. The mix
// is a guess; the paper's is unknown (the record says so). It asserts only
// the plan facts the record rests on; the numbers are the artifact.
// CHOIR_RUN_MEMORY_HOLD_GC=1 adds a pass after the store's own online DOLT_GC
// and after a close and reopen (an empty engine chunk cache).
func TestRunMemoryHoldCensus(t *testing.T) {
	sizes := os.Getenv("CHOIR_RUN_MEMORY_HOLD")
	if sizes == "" {
		t.Skip("set CHOIR_RUN_MEMORY_HOLD=20000:2000 to run")
	}
	for _, field := range strings.Split(sizes, ",") {
		pair := strings.Split(strings.TrimSpace(field), ":")
		if len(pair) != 2 {
			t.Fatalf("bad size %q, want TOTAL:HOT", field)
		}
		total, err1 := strconv.Atoi(pair[0])
		hot, err2 := strconv.Atoi(pair[1])
		if err1 != nil || err2 != nil || hot <= 0 || total < hot {
			t.Fatalf("bad size %q", field)
		}
		t.Run(field, func(t *testing.T) { runMemoryHoldCensus(t, total, hot) })
	}
}

const rmOwner = "owner-rm-census"

func runMemoryHoldCensus(t *testing.T, total, hot int) {
	s := openTestStore(t)
	ctx := context.Background()
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	// ---- seed ---------------------------------------------------------------
	msg := `{"role":"assistant","content":"` + strings.Repeat("m", 1500) + `"}`
	comp := strings.Repeat("s", 3000)
	seedStart := time.Now()
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
		_, err := tx.ExecContext(ctx, `INSERT INTO run_memory_entries
			(entry_id, loop_id, owner_id, agent_id, parent_entry_id, seq, kind, role, message_json, summary,
			 first_kept_entry_id, tokens_before, reason, model, details_json, created_at) VALUES `+strings.Join(vals, ","), args...)
		if err != nil {
			t.Fatal(err)
		}
		vals, args = vals[:0], args[:0]
	}
	perRunSeq := map[string]int64{}
	perAgent := map[string]int64{}
	hotLeft, otherLeft := hot, total-hot
	for i := 0; hotLeft+otherLeft > 0; i++ {
		// Interleave: the hot agent's rows are spread evenly through the table.
		isHot := hotLeft > 0 && (otherLeft == 0 || i%max(1, total/hot) == 0)
		var agent, run string
		if isHot {
			hotLeft--
			agent = "agent-hot"
			run = fmt.Sprintf("run-hot-%05d", (hot-hotLeft-1)/250)
		} else {
			otherLeft--
			agent = fmt.Sprintf("agent-%02d", i%40)
			run = fmt.Sprintf("run-%s-%05d", agent, i/40/250)
		}
		perRunSeq[run]++
		perAgent[agent]++
		kind, message, summary, details := "message", msg, "", "{}"
		switch r := perAgent[agent] % 20; {
		case r < 11:
		case r < 15:
			kind, message = "cell_fate", ""
			details = `{"cell_id":"cell:` + run + `","fate":"success"}`
		case r < 19:
			kind, message = "rlm_inbox_cursor", ""
			summary = "rlm inbox cursor on ch"
			details = fmt.Sprintf(`{"channel_id":"ch-1","desk_agent_id":%q,"cursor":%d}`, agent, i)
		default:
			kind, message, summary = "compaction", "", comp
		}
		vals = append(vals, "(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)")
		args = append(args, fmt.Sprintf("e-%08d", i), run, rmOwner, agent, "", perRunSeq[run], kind, "", message, summary,
			"", 0, "", "", details, base.Add(time.Duration(i)*time.Second))
		if len(vals) == 100 {
			flush()
		}
	}
	flush()
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	t.Logf("seeded %d rows (hot agent %d, in %d runs of 250) in %v", total, hot, (hot+249)/250, time.Since(seedStart).Round(time.Second))

	// ---- plan ---------------------------------------------------------------
	sel := `SELECT entry_id, loop_id, owner_id, agent_id, parent_entry_id, seq,
	        kind, role, message_json, summary, first_kept_entry_id,
	        tokens_before, reason, model, details_json, created_at
	   FROM run_memory_entries
	  WHERE agent_id = ? AND owner_id = ? AND kind IN (?) ORDER BY created_at ASC, seq ASC`
	plan := explainEventTablePlan(t, s, sel, "agent-hot", rmOwner, "rlm_inbox_cursor")
	t.Logf("ListRunMemoryEntriesForAgent plan (owner given, kinds=[rlm_inbox_cursor]):\n%s", plan)
	if !strings.Contains(plan, "[run_memory_entries.owner_id,run_memory_entries.agent_id,run_memory_entries.created_at]") {
		t.Errorf("plan does not read the (owner_id, agent_id, created_at) index:\n%s", plan)
	}
	noOwner := strings.Replace(strings.Replace(sel, "AND owner_id = ? ", "", 1), "ORDER", "ORDER", 1)
	t.Logf("same, owner empty (a caller that passes ownerID \"\"), plan:\n%s", explainEventTablePlan(t, s, noOwner, "agent-hot", "rlm_inbox_cursor"))

	report := func(name string, first, best time.Duration, rows int, bytes uint64) {
		t.Logf("total=%d hot=%d %-62s first %9.1f ms  best %9.1f ms  rows %6d  alloc %7.1f MiB",
			total, hot, name, ms(first), ms(best), rows, float64(bytes)/(1<<20))
	}
	list := func(agent string) func() int {
		return func() int {
			es, err := s.ListRunMemoryEntriesForAgent(ctx, rmOwner, agent, []string{"rlm_inbox_cursor"})
			if err != nil {
				t.Fatal(err)
			}
			return len(es)
		}
	}
	f, b, r, by := rmTimeRead(t, list("agent-hot"))
	report("ListRunMemoryEntriesForAgent hot desk (cursor kind)", f, b, r, by)
	f, b, r, by = rmTimeRead(t, list("agent-07"))
	report("ListRunMemoryEntriesForAgent cold desk agent-07 (cursor kind)", f, b, r, by)
	f, b, r, by = rmTimeRead(t, func() int {
		es, err := s.ListRunMemoryEntriesForAgent(ctx, rmOwner, "agent-hot", nil)
		if err != nil {
			t.Fatal(err)
		}
		return len(es)
	})
	report("ListRunMemoryEntriesForAgent hot desk (all kinds, for scale)", f, b, r, by)
	f, b, r, by = rmTimeRead(t, func() int {
		es, err := s.ListRunMemoryEntries(ctx, rmOwner, "run-hot-00000")
		if err != nil {
			t.Fatal(err)
		}
		return len(es)
	})
	report("ListRunMemoryEntries one run of 250 rows", f, b, r, by)

	// Query time versus row iteration, same statement, on the pool's one connection.
	f, b, r, by = rmTimeRead(t, func() int {
		rows, err := s.db.QueryContext(ctx, sel, "agent-hot", rmOwner, "rlm_inbox_cursor")
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		n := 0
		for rows.Next() {
			n++
		}
		return n
	})
	report("  same statement, rows.Next only (no scan or decode)", f, b, r, by)

	// ---- after a Dolt GC (optional) -------------------------------------------
	// The paper's stall stacks are in archive reads (zstd with a dictionary),
	// which a fresh local journal does not have. CHOIR_RUN_MEMORY_HOLD_GC=1
	// runs the store's own online GC on the scratch store and measures the hot
	// desk's list again, so a GC'd table's read cost can be compared.
	if os.Getenv("CHOIR_RUN_MEMORY_HOLD_GC") != "" {
		gcStart := time.Now()
		if err := s.runOnlineDoltGC(ctx); err != nil {
			t.Fatalf("online gc: %v", err)
		}
		t.Logf("scratch online DOLT_GC took %v", time.Since(gcStart).Round(time.Millisecond))
		f, b, r, by := rmTimeRead(t, list("agent-hot"))
		report("after DOLT_GC: ListRunMemoryEntriesForAgent hot desk (cursor kind)", f, b, r, by)
		// A reopened engine has no chunk cache: the first call is the nearest a
		// local run gets to the paper's first read after a boot. The OS page
		// cache stays warm, so it is a floor.
		path := s.path
		if err := s.Close(); err != nil {
			t.Fatalf("close for reopen: %v", err)
		}
		if err := s.Reopen(path); err != nil {
			t.Fatalf("reopen: %v", err)
		}
		f, b, r, by = rmTimeRead(t, list("agent-hot"))
		report("after DOLT_GC and reopen: hot desk (first = engine cache cold)", f, b, r, by)
		f, b, r, by = rmTimeRead(t, list("agent-07"))
		report("after DOLT_GC: ListRunMemoryEntriesForAgent cold desk (cursor kind)", f, b, r, by)
	}

	// ---- the connection hold ------------------------------------------------
	// While the list runs, how long does a trivial read on the same pool wait?
	// This is /health's situation: its three reads use s.db, which has one
	// connection (configureEmbeddedDoltDB), with a 1 s budget.
	for _, c := range []struct {
		name string
		kind string
	}{{"hot desk", "agent-hot"}, {"cold desk", "agent-07"}} {
		var waits []float64
		for rep := 0; rep < 5; rep++ {
			var wg sync.WaitGroup
			started := make(chan struct{})
			wg.Add(1)
			go func() {
				defer wg.Done()
				close(started)
				list(c.kind)()
			}()
			<-started
			time.Sleep(2 * time.Millisecond) // let the list take the connection
			t0 := time.Now()
			var one int
			if err := s.db.QueryRowContext(ctx, `SELECT 1`).Scan(&one); err != nil {
				t.Fatal(err)
			}
			waits = append(waits, ms(time.Since(t0)))
			wg.Wait()
		}
		t.Logf("total=%d hot=%d trivial s.db read issued 2 ms into a list of the %s waits (ms, 5 runs): %v",
			total, hot, c.name, waits)
	}

	// ---- two read shapes that would cost less (scratch store only) ----------
	narrow := `SELECT details_json FROM run_memory_entries WHERE agent_id = ? AND owner_id = ? AND kind IN (?) ORDER BY created_at ASC, seq ASC`
	f, b, r, by = rmTimeRead(t, func() int {
		rows, err := s.db.QueryContext(ctx, narrow, "agent-hot", rmOwner, "rlm_inbox_cursor")
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		n := 0
		for rows.Next() {
			var d string
			if err := rows.Scan(&d); err != nil {
				t.Fatal(err)
			}
			n++
		}
		return n
	})
	report("variant A: narrow projection (details_json only), same index", f, b, r, by)

	idxStart := time.Now()
	if _, err := s.db.ExecContext(ctx, `CREATE INDEX idx_rm_census_owner_agent_kind_created ON run_memory_entries(owner_id, agent_id, kind, created_at)`); err != nil {
		t.Fatal(err)
	}
	t.Logf("scratch index (owner_id, agent_id, kind, created_at) built in %v", time.Since(idxStart).Round(time.Millisecond))
	t.Logf("variant B plan:\n%s", explainEventTablePlan(t, s, sel, "agent-hot", rmOwner, "rlm_inbox_cursor"))
	f, b, r, by = rmTimeRead(t, list("agent-hot"))
	report("variant B: ListRunMemoryEntriesForAgent with the kind index", f, b, r, by)
	f, b, r, by = rmTimeRead(t, func() int {
		rows, err := s.db.QueryContext(ctx, narrow, "agent-hot", rmOwner, "rlm_inbox_cursor")
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		n := 0
		for rows.Next() {
			var d string
			if err := rows.Scan(&d); err != nil {
				t.Fatal(err)
			}
			n++
		}
		return n
	})
	report("variant C: narrow projection with the kind index", f, b, r, by)
	latest := `SELECT details_json FROM run_memory_entries WHERE agent_id = ? AND owner_id = ? AND kind = ? ORDER BY created_at DESC LIMIT 64`
	f, b, r, by = rmTimeRead(t, func() int {
		rows, err := s.db.QueryContext(ctx, latest, "agent-hot", rmOwner, "rlm_inbox_cursor")
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		n := 0
		for rows.Next() {
			var d string
			if err := rows.Scan(&d); err != nil {
				t.Fatal(err)
			}
			n++
		}
		return n
	})
	report("variant D: newest 64 cursor rows, narrow, with the kind index", f, b, r, by)
	t.Logf("variant D plan:\n%s", explainEventTablePlan(t, s, latest, "agent-hot", rmOwner, "rlm_inbox_cursor"))
}

// rmTimeRead runs fn three times and returns the first and the best duration
// and the bytes the first call allocated (TotalAlloc, not peak heap). It is
// the same helper as TestBootReadCensus's, repeated so the two lanes do not
// collide.
func rmTimeRead(t *testing.T, fn func() int) (first, best time.Duration, rows int, bytes uint64) {
	t.Helper()
	for r := 0; r < 3; r++ {
		runtime.GC()
		var m0, m1 runtime.MemStats
		runtime.ReadMemStats(&m0)
		t0 := time.Now()
		rows = fn()
		d := time.Since(t0)
		if r == 0 {
			runtime.ReadMemStats(&m1)
			first, best, bytes = d, d, m1.TotalAlloc-m0.TotalAlloc
		} else if d < best {
			best = d
		}
	}
	return first, best, rows, bytes
}
