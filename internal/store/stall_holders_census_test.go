package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// TestStallHoldersCensus is the repeatable artifact of
// docs/problems/paper-remaining-store-stall-holders-2026-10-23.md. It measures,
// on a local embedded Dolt store, the holders that remain in the paper's
// store_stalled lines once the run-memory kind index is built: the selfdev
// ListByStates read, the derived-key validation page (scanSideRows), the
// online DOLT_GC, and the GET /api/runs scans. It is off by default; pick
// sections with
//
//	CHOIR_STALL_HOLDERS=selfdev,runs,gc go test ./internal/store -run TestStallHoldersCensus -v -count=1 -timeout 60m
//
// CHOIR_STALL_RUNS sets the run-object count (default 34000, the paper's
// choir.run count at 12:33 on 2026-10-23). CHOIR_STALL_GC_MIB sets the journal
// sizes the GC section fills before each pass (default 128,512). The numbers
// are the artifact; the only assertions are the plan facts the record cites.
func TestStallHoldersCensus(t *testing.T) {
	sections := os.Getenv("CHOIR_STALL_HOLDERS")
	if sections == "" {
		t.Skip("set CHOIR_STALL_HOLDERS=selfdev,runs,gc to run")
	}
	want := map[string]bool{}
	for _, s := range strings.Split(sections, ",") {
		want[strings.TrimSpace(s)] = true
	}
	if want["selfdev"] {
		t.Run("selfdev", stallSelfdev)
	}
	if want["runs"] {
		t.Run("runs", stallRuns)
	}
	if want["gc"] {
		t.Run("gc", stallGC)
	}
}

// stallProbe issues SELECT 1 on db every 5 ms until stop is called and
// returns the longest wait seen: how long the work under it kept the
// connection from a /health-shaped read.
func stallProbe(ctx context.Context, db *sql.DB) (stop func() time.Duration) {
	return stallProbeQuery(ctx, db, `SELECT 1`)
}

// stallProbeQuery is stallProbe with a chosen single-value query.
func stallProbeQuery(ctx context.Context, db *sql.DB, query string) (stop func() time.Duration) {
	var (
		mu   sync.Mutex
		maxW time.Duration
		done = make(chan struct{})
		wg   sync.WaitGroup
	)
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-done:
				return
			default:
			}
			t0 := time.Now()
			var one int
			_ = db.QueryRowContext(ctx, query).Scan(&one)
			w := time.Since(t0)
			mu.Lock()
			if w > maxW {
				maxW = w
			}
			mu.Unlock()
			time.Sleep(5 * time.Millisecond)
		}
	}()
	return func() time.Duration {
		close(done)
		wg.Wait()
		mu.Lock()
		defer mu.Unlock()
		return maxW
	}
}

func stallAlloc() uint64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.TotalAlloc
}

// ---- selfdev.ListByStates ----------------------------------------------------

func stallSelfdev(t *testing.T) {
	ctx := context.Background()
	for _, n := range []int{200, 20000} {
		n := n
		t.Run(fmt.Sprintf("rows=%d", n), func(t *testing.T) {
			s := openTestStore(t)
			const computer = "computer-stall"
			base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
			tx, err := s.db.BeginTx(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			states := []string{selfdev.StateApplied, selfdev.StateFailed, selfdev.StateRolledBack, selfdev.StateApplied}
			for i := 0; i < n; i++ {
				state := states[i%len(states)]
				if i >= n-2 {
					state = selfdev.StateAwaitingApproval // two open operations, as a quiet paper has
				}
				at := base.Add(time.Duration(i) * time.Minute)
				if _, err := tx.ExecContext(ctx, `INSERT INTO self_development_operations
					(operation_id, computer_id, idempotency_key, request_commitment, trajectory_id, base_head, prompt_artifact_ref,
					 desired_head, effective_head, state, created_at, updated_at)
					VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
					fmt.Sprintf("op-%07d", i), computer, fmt.Sprintf("idem-%07d", i), strings.Repeat("a", 64), fmt.Sprintf("traj-%07d", i),
					strings.Repeat("b", 64), strings.Repeat("p", 400), strings.Repeat("c", 64), strings.Repeat("d", 64), state, at, at); err != nil {
					t.Fatal(err)
				}
			}
			if err := tx.Commit(); err != nil {
				t.Fatal(err)
			}
			ops, err := selfdev.NewStore(s, s)
			if err != nil {
				t.Fatal(err)
			}
			busy := []string{selfdev.StateExecuting, selfdev.StateAccepted, selfdev.StateMaterializing, selfdev.StateRollbackPending}
			applied := []string{selfdev.StateApplied}
			plan := explainEventTablePlan(t, s, `SELECT * FROM self_development_operations WHERE computer_id=? AND state IN (?,?,?,?) ORDER BY created_at`, computer, busy[0], busy[1], busy[2], busy[3])
			t.Logf("rows=%d busy-states plan (/health's selfdev_active_operations):\n%s", n, plan)
			planApplied := explainEventTablePlan(t, s, `SELECT * FROM self_development_operations WHERE computer_id=? AND state IN (?) ORDER BY created_at`, computer, applied[0])
			t.Logf("rows=%d applied-state plan (/health's effective_release path, appliedOperationForRelease):\n%s", n, planApplied)
			for _, c := range []struct {
				name   string
				states []string
			}{{"busy states (/health)", busy}, {"applied (effective_release)", applied}, {"open states", selfdev.OpenStates}} {
				first, best, rows, bytes := rmTimeRead(t, func() int {
					got, err := ops.ListByStates(ctx, computer, c.states...)
					if err != nil {
						t.Fatal(err)
					}
					return len(got)
				})
				t.Logf("rows=%d ListByStates %-30s first %8.2f ms  best %8.2f ms  rows returned %6d  alloc %7.2f MiB", n, c.name, ms(first), ms(best), rows, float64(bytes)/(1<<20))
			}
		})
	}
}

// ---- GET /api/runs and the derived-key validation page ----------------------

const stallOwner = "owner-stall"
const stallComputer = "computer-stall"

func stallSeedRuns(t *testing.T, s *Store, n int) {
	t.Helper()
	ctx := context.Background()
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	prompt := strings.Repeat("p", 1500)
	result := strings.Repeat("r", 1500)
	start := time.Now()
	const batch = 200
	for lo := 0; lo < n; lo += batch {
		var objs []objectgraph.Object
		for i := lo; i < lo+batch && i < n; i++ {
			runID := fmt.Sprintf("run-%07d", i)
			created := base.Add(time.Duration(i) * time.Second)
			rec := types.RunRecord{
				RunID: runID, AgentID: fmt.Sprintf("agent-%03d", i%60), ChannelID: "ch-1", TrajectoryID: fmt.Sprintf("traj-%06d", i/5),
				AgentProfile: "research", AgentRole: "research", OwnerID: stallOwner, ComputerID: stallComputer, State: types.RunState("completed"),
				Prompt: prompt, Result: result, CreatedAt: created, UpdatedAt: created,
			}
			body, err := json.Marshal(rec)
			if err != nil {
				t.Fatal(err)
			}
			meta, err := json.Marshal(map[string]any{
				"run_id": runID, "agent_id": rec.AgentID, "channel_id": rec.ChannelID, "trajectory_id": rec.TrajectoryID,
				"agent_profile": rec.AgentProfile, "agent_role": rec.AgentRole, "computer_id": stallComputer, "state": "completed",
				"created_at": created.Format(time.RFC3339Nano), "updated_at": created.Format(time.RFC3339Nano),
			})
			if err != nil {
				t.Fatal(err)
			}
			id, err := objectgraph.BuildCanonicalID(ogKindRun, stallOwner, objectgraph.StableSuffixFromKey(runID))
			if err != nil {
				t.Fatal(err)
			}
			objs = append(objs, objectgraph.Object{
				CanonicalID: id, ObjectKind: ogKindRun, OwnerID: stallOwner, ComputerID: stallComputer, VersionID: runID,
				ContentHash: objectgraph.ContentHash(ogKindRun, body, meta), Body: body, Metadata: meta, CreatedAt: created, UpdatedAt: created,
			})
		}
		if err := s.ogStore.PutBatch(ctx, objectgraph.Batch{Objects: objs}); err != nil {
			t.Fatalf("seed runs: %v", err)
		}
	}
	t.Logf("seeded %d choir.run objects (computer_id set, ~3 KiB body) in %v", n, time.Since(start).Round(time.Second))
}

func stallEngineOps(s *Store, prefix string) string {
	var lines []string
	for op, st := range s.EngineMutexStats() {
		if strings.Contains(op, prefix) {
			lines = append(lines, fmt.Sprintf("    %-70s calls %5d  max hold %9.1f ms  total hold %9.1f ms", op, st.Calls, float64(st.MaxHoldNanos)/1e6, float64(st.HoldNanos)/1e6))
		}
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

func stallRuns(t *testing.T) {
	ctx := context.Background()
	n := 34000
	if v := os.Getenv("CHOIR_STALL_RUNS"); v != "" {
		if _, err := fmt.Sscanf(v, "%d", &n); err != nil || n <= 0 {
			t.Fatalf("bad CHOIR_STALL_RUNS %q", v)
		}
	}
	s := openTestStore(t)
	stallSeedRuns(t, s, n)

	// ---- plans --------------------------------------------------------------
	t.Logf("ListObjectsPage (ogListAllObjectsByKind, ListRunsByOwnerOG) page plan:\n%s",
		explainEventTablePlan(t, s, `SELECT canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, metadata, created_at, updated_at, tombstone, superseded_by
		 FROM og_objects WHERE object_kind = ? AND canonical_id > ? ORDER BY canonical_id ASC LIMIT ?`, "choir.run", "", 256))
	t.Logf("ReadObjectSnapshotFiltered phase-1 refs plan (ListLifecycleRunsByOwner):\n%s",
		explainEventTablePlan(t, s, `SELECT canonical_id, object_kind, metadata FROM og_objects WHERE owner_id = ? AND computer_id = ? AND object_kind IN (?) ORDER BY canonical_id`, stallOwner, stallComputer, "choir.run"))

	// ---- GET /api/runs: the two scans, and what each holds ------------------
	for _, limit := range []int{100} {
		stop := stallProbe(ctx, s.db)
		stopRead := stallProbe(ctx, s.readDB)
		a0, t0 := stallAlloc(), time.Now()
		legacy, err := s.ListRunsByOwner(ctx, stallOwner, limit)
		dLegacy := time.Since(t0)
		aLegacy := stallAlloc() - a0
		if err != nil {
			t.Fatal(err)
		}
		a0, t0 = stallAlloc(), time.Now()
		life, err := s.ListLifecycleRunsByOwner(ctx, stallOwner, stallComputer, limit)
		dLife := time.Since(t0)
		aLife := stallAlloc() - a0
		if err != nil {
			t.Fatal(err)
		}
		readWait := stopRead()
		writeWait := stop()
		t.Logf("runs=%d limit=%d: ListRunsByOwner (legacy scan, skips every lifecycle row after decoding it) %8.1f ms, %7.1f MiB, returned %d",
			n, limit, ms(dLegacy), float64(aLegacy)/(1<<20), len(legacy))
		t.Logf("runs=%d limit=%d: ListLifecycleRunsByOwner (second scan, the rows that count)                %8.1f ms, %7.1f MiB, returned %d",
			n, limit, ms(dLife), float64(aLife)/(1<<20), len(life))
		t.Logf("runs=%d: longest SELECT 1 wait during both scans: on s.db (write connection) %.1f ms; on s.readDB (the scans' own connection) %.1f ms",
			n, ms(writeWait), ms(readWait))
		t.Logf("engine mutex ops during them:\n%s", stallEngineOps(s, "ListObjectsPage"))
		t.Logf("%s", stallEngineOps(s, "ReadObjectSnapshotFiltered"))
	}

	// A concurrent engine-mutex user (a point read of one object) during the
	// lifecycle scan: how long does it wait for the engine?
	probeID, err := objectgraph.BuildCanonicalID(ogKindRun, stallOwner, objectgraph.StableSuffixFromKey("run-0000001"))
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	wg.Add(1)
	started := make(chan struct{})
	go func() {
		defer wg.Done()
		close(started)
		if _, err := s.ListLifecycleRunsByOwner(ctx, stallOwner, stallComputer, 100); err != nil {
			t.Error(err)
		}
	}()
	<-started
	time.Sleep(20 * time.Millisecond)
	t0 := time.Now()
	if _, err := s.ogStore.GetObject(ctx, probeID); err != nil {
		t.Fatal(err)
	}
	t.Logf("runs=%d: a point GetObject issued 20 ms into ListLifecycleRunsByOwner waited %.1f ms (the engine mutex)", n, ms(time.Since(t0)))
	wg.Wait()

	// ---- the cheaper shape: the newest rows by an ordered ids-only read -----
	idsOnly := `SELECT canonical_id FROM og_objects WHERE object_kind = ? AND owner_id = ? AND computer_id = ? ORDER BY created_at DESC LIMIT ?`
	t.Logf("fix-shape probe, newest 100 run ids for the owner (ids only, no body), plan:\n%s", explainEventTablePlan(t, s, idsOnly, "choir.run", stallOwner, stallComputer, 100))
	first, best, rows, bytes := rmTimeRead(t, func() int {
		r, err := s.readDB.QueryContext(ctx, idsOnly, "choir.run", stallOwner, stallComputer, 100)
		if err != nil {
			t.Fatal(err)
		}
		defer r.Close()
		c := 0
		for r.Next() {
			var id string
			if err := r.Scan(&id); err != nil {
				t.Fatal(err)
			}
			c++
		}
		return c
	})
	t.Logf("runs=%d fix-shape ids-only newest 100: first %.1f ms best %.1f ms rows %d alloc %.2f MiB", n, ms(first), ms(best), rows, float64(bytes)/(1<<20))

	// ---- the derived-key validation page (scanSideRows) ---------------------
	head, sideQ := ogHeadPageSQLForTest()
	t.Logf("validation head page plan:\n%s", explainEventTablePlan(t, s, head, "obj:choir.run:", "obj:choir.run;", 200))
	t.Logf("validation side-table page plan:\n%s", explainEventTablePlan(t, s, sideQ, "run.agent_id", "obj:choir.run:", "obj:choir.run;"))
	// One mid-range side-table page (200 rows) as validatePage runs it, to see
	// whether a page's cost follows the family's size. (A FORCE INDEX (PRIMARY)
	// hint was tried and is ignored by the engine: the plan is unchanged.)
	var lo, hi string
	if err := s.db.QueryRowContext(ctx, `SELECT canonical_id FROM og_objects WHERE canonical_id > 'obj:choir.run:' AND canonical_id < 'obj:choir.run;' ORDER BY canonical_id LIMIT 1 OFFSET ?`, n/2).Scan(&lo); err != nil {
		t.Fatal(err)
	}
	if err := s.db.QueryRowContext(ctx, `SELECT canonical_id FROM og_objects WHERE canonical_id > ? AND canonical_id < 'obj:choir.run;' ORDER BY canonical_id LIMIT 1 OFFSET 199`, lo).Scan(&hi); err != nil {
		t.Fatal(err)
	}
	pageQuery := `SELECT canonical_id, owner_id, updated_at, content_hash, derive_ok FROM ` + objectgraph.DerivedKeysTable + `
		WHERE key_name = ? AND canonical_id > ? AND canonical_id < ? AND canonical_id <= ? ORDER BY canonical_id ASC`
	bestPage, pageRows := time.Hour, 0
	for rep := 0; rep < 5; rep++ {
		t0 := time.Now()
		r, err := s.db.QueryContext(ctx, pageQuery, "run.agent_id", lo, "obj:choir.run;", hi)
		if err != nil {
			t.Fatal(err)
		}
		pageRows = 0
		for r.Next() {
			var a, b, c string
			var d, e any
			_ = r.Scan(&a, &b, &d, &c, &e)
			pageRows++
		}
		r.Close()
		if w := time.Since(t0); w < bestPage {
			bestPage = w
		}
	}
	t.Logf("runs=%d one mid-range side page: %d rows, best of 5 %.2f ms", n, pageRows, ms(bestPage))

	var logs []string
	var logMu sync.Mutex
	stopW := stallProbe(ctx, s.db)
	v0 := time.Now()
	err = s.ValidateDerivedKeys(ctx, objectgraph.DerivedKeyValidation{PageSize: 200, DutyCycle: 0, Logf: func(f string, a ...any) {
		logMu.Lock()
		logs = append(logs, fmt.Sprintf(f, a...))
		logMu.Unlock()
	}})
	vd := time.Since(v0)
	waitV := stopW()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("runs=%d ValidateDerivedKeys (page 200, no pause) took %.1f s; longest SELECT 1 on s.db during it %.1f ms", n, vd.Seconds(), ms(waitV))
	t.Logf("%s", stallEngineOps(s, "ValidateDerivedKeys page"))
	logMu.Lock()
	for _, l := range logs {
		if strings.Contains(l, "validated") || strings.Contains(l, "starts") {
			t.Logf("  %s", l)
		}
	}
	logMu.Unlock()
}

// ogHeadPageSQLForTest and the side-table page text, as validatePage builds
// them (objectgraph/derived_keys.go), for EXPLAIN. Kept in step by hand: the
// numbers they print are the record's.
func ogHeadPageSQLForTest() (head, side string) {
	head = `SELECT canonical_id, owner_id, updated_at, content_hash, object_kind FROM og_objects
		WHERE canonical_id > ? AND canonical_id < ?
		ORDER BY canonical_id ASC LIMIT ?`
	side = `SELECT canonical_id, owner_id, updated_at, content_hash, derive_ok FROM ` + objectgraph.DerivedKeysTable + `
		WHERE key_name = ? AND canonical_id > ? AND canonical_id < ? ORDER BY canonical_id ASC`
	return head, side
}

// ---- the online GC ------------------------------------------------------------

func stallGC(t *testing.T) {
	ctx := context.Background()
	sizes := []int{128, 512}
	if v := os.Getenv("CHOIR_STALL_GC_MIB"); v != "" {
		sizes = nil
		for _, f := range strings.Split(v, ",") {
			var n int
			if _, err := fmt.Sscanf(strings.TrimSpace(f), "%d", &n); err != nil || n <= 0 {
				t.Fatalf("bad CHOIR_STALL_GC_MIB %q", v)
			}
			sizes = append(sizes, n)
		}
	}
	s := openTestStore(t)
	msg := `{"role":"assistant","content":"` + strings.Repeat("m", 1500) + `"}`
	seq := 0
	fill := func(targetMiB int) {
		t0 := time.Now()
		for int(doltJournalBytes(s.texturePath)>>20) < targetMiB {
			tx, err := s.db.BeginTx(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			for b := 0; b < 20; b++ {
				var vals []string
				var args []any
				for r := 0; r < 100; r++ {
					seq++
					vals = append(vals, "(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)")
					args = append(args, fmt.Sprintf("e-%09d", seq), fmt.Sprintf("run-%05d", seq/250), "owner-gc", fmt.Sprintf("agent-%02d", seq%40), "", seq%250, "message", "", msg, "", "", 0, "", "", "{}",
						time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(seq)*time.Second))
				}
				if _, err := tx.ExecContext(ctx, `INSERT INTO run_memory_entries
					(entry_id, loop_id, owner_id, agent_id, parent_entry_id, seq, kind, role, message_json, summary,
					 first_kept_entry_id, tokens_before, reason, model, details_json, created_at) VALUES `+strings.Join(vals, ","), args...); err != nil {
					t.Fatal(err)
				}
			}
			if err := tx.Commit(); err != nil {
				t.Fatal(err)
			}
		}
		t.Logf("filled the journal to %d MiB (%d rows) in %v", doltJournalBytes(s.texturePath)>>20, seq, time.Since(t0).Round(time.Second))
	}
	for _, mib := range sizes {
		// ---- the store's own pass: on the write connection -------------------
		fill(mib)
		journal := doltJournalBytes(s.texturePath) >> 20
		stopW := stallProbe(ctx, s.db)
		// /health-shaped reads on the read pool's connection during the same pass:
		// a trivial one and one that touches a table (the selfdev busy-state read).
		stopRead1 := stallProbe(ctx, s.readDB)
		stopRead2 := stallProbeQuery(ctx, s.readDB, `SELECT COUNT(*) FROM self_development_operations WHERE computer_id = 'x' AND state IN ('executing','accepted')`)
		g0 := time.Now()
		if err := s.runOnlineDoltGC(ctx); err != nil {
			t.Fatal(err)
		}
		d := time.Since(g0)
		r1, r2 := stopRead1(), stopRead2()
		t.Logf("journal %5d MiB: runOnlineDoltGC (write connection) %.2f s; longest SELECT 1 on s.db during it %.2f s; on s.readDB: SELECT 1 %.2f s, table read %.2f s; journal after %d MiB",
			journal, d.Seconds(), stopW().Seconds(), r1.Seconds(), r2.Seconds(), doltJournalBytes(s.texturePath)>>20)

		// ---- the same pass issued on the read pool's connection ---------------
		fill(mib)
		journal = doltJournalBytes(s.texturePath) >> 20
		stopW = stallProbe(ctx, s.db)
		stopR := stallProbe(ctx, s.readDB)
		// A writer on s.db during the pass: one small insert every 20 ms, each timed.
		var wmu sync.Mutex
		var worst time.Duration
		wstop := make(chan struct{})
		var wwg sync.WaitGroup
		wwg.Add(1)
		go func() {
			defer wwg.Done()
			for i := 0; ; i++ {
				select {
				case <-wstop:
					return
				default:
				}
				t0 := time.Now()
				_, _ = s.db.ExecContext(ctx, `INSERT INTO run_memory_entries (entry_id, loop_id, owner_id, agent_id, parent_entry_id, seq, kind, role, message_json, summary,
					first_kept_entry_id, tokens_before, reason, model, details_json, created_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
					fmt.Sprintf("w-%d-%d", mib, time.Now().UnixNano()), "run-w", "owner-gc", "agent-w", "", i, "message", "", "{}", "", "", 0, "", "", "{}", time.Now().UTC())
				if w := time.Since(t0); w > worst {
					wmu.Lock()
					worst = w
					wmu.Unlock()
				}
				time.Sleep(20 * time.Millisecond)
			}
		}()
		g0 = time.Now()
		_, err := s.readDB.ExecContext(ctx, "CALL DOLT_GC()")
		d = time.Since(g0)
		close(wstop)
		wwg.Wait()
		waitR, waitW := stopR(), stopW()
		if err != nil {
			t.Logf("journal %5d MiB: CALL DOLT_GC() on s.readDB failed after %.2f s: %v", journal, d.Seconds(), err)
			continue
		}
		wmu.Lock()
		t.Logf("journal %5d MiB: CALL DOLT_GC() on s.readDB %.2f s; longest SELECT 1 on s.db %.2f s, on s.readDB %.2f s; slowest 1-row INSERT on s.db %.2f s; journal after %d MiB",
			journal, d.Seconds(), waitW.Seconds(), waitR.Seconds(), worst.Seconds(), doltJournalBytes(s.texturePath)>>20)
		wmu.Unlock()
	}
}
