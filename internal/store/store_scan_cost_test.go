package store

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// TestStoreScanCost is the before/after micro-benchmark for the guest store
// scan levers (docs/problems/guest-cpu-is-unindexed-store-scans-2026-10-14.md).
// It is opt-in because it builds a store of realistic size:
//
//	CHOIR_STORE_SCAN_BENCH=<commitment records>   (e.g. 10000)
//	CHOIR_STORE_SCAN_BENCH_OUT=<file.json>        write the measurements
//
// The fixture is the records, committed through the real inbox command path
// (28 sources noting management, 40 stories with management→texture and
// texture→research rounds, asks), with their wake rows, heads and command
// receipts, plus two choir.event rows per record over 400 runs.
//
// Every read is measured twice on the same data in one process: "before"
// reads through an object-graph store that never learned the generated
// columns, which issues exactly the JSON_EXTRACT SQL the old code issued;
// "after" reads through the live store. The run-end check's before is the
// record fold (OpenRounds), its after the head read (OpenRoundsAddressedTo).
// CPU is process CPU time (user+system) per call, which a loaded host does
// not inflate the way it inflates wall time; wall time is reported too.
//
// The schema migration on an existing store is measured by dropping the
// record columns from the built fixture, reopening it (Open leaves a store
// with rows alone) and timing EnsureObjectGraphIndexes, the boot's call.
func TestStoreScanCost(t *testing.T) {
	records, _ := strconv.Atoi(os.Getenv("CHOIR_STORE_SCAN_BENCH"))
	if records <= 0 {
		t.Skip("set CHOIR_STORE_SCAN_BENCH=<records> to run the store scan benchmark")
	}
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "scan-bench.db")
	out := map[string]any{"records": records}

	s, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	buildStart := time.Now()
	buildScanFixture(t, s, records)
	out["build_s"] = time.Since(buildStart).Seconds()
	objectCount := 0
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM og_objects`).Scan(&objectCount); err != nil {
		t.Fatal(err)
	}
	out["og_objects"] = objectCount
	t.Logf("fixture: %d og_objects built in %v", objectCount, time.Since(buildStart))

	// Migration: drop what this change adds, reopen, time the open.
	for _, stmt := range []string{
		`DROP INDEX idx_og_objects_og_rec_inbox ON og_objects`,
		`DROP INDEX idx_og_objects_og_rec_sender_inbox ON og_objects`,
		`DROP INDEX idx_og_objects_og_rec_conversation_id ON og_objects`,
		`DROP INDEX idx_og_objects_rec_inbox_seq ON og_objects`,
		`DROP INDEX idx_og_objects_rec_claim ON og_objects`,
		`ALTER TABLE og_objects DROP COLUMN og_rec_inbox_seq`,
		`ALTER TABLE og_objects DROP COLUMN og_rec_claim`,
		`ALTER TABLE og_objects DROP COLUMN og_rec_inbox`,
		`ALTER TABLE og_objects DROP COLUMN og_rec_sender_inbox`,
		`ALTER TABLE og_objects DROP COLUMN og_rec_conversation_id`,
	} {
		_, _ = s.db.ExecContext(ctx, stmt) // absent on the old code
	}
	workspace := s.TexturePath()
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if err := RunDoltGCWorkspace(workspace); err != nil {
		t.Logf("dolt gc: %v", err)
	}
	openCPU, openStart := cpuTime(), time.Now()
	s, err = Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer func() { _ = s.Close() }()
	out["reopen_wall_ms"] = ms(time.Since(openStart))
	out["reopen_cpu_ms"] = ms(cpuTime() - openCPU)
	t.Logf("reopen (no record migration in Open): wall %v cpu %v", time.Since(openStart), cpuTime()-openCPU)
	migrateCPU, migrateStart := cpuTime(), time.Now()
	var beats atomic.Int64
	if err := s.EnsureObjectGraphIndexes(ctx, func() { beats.Add(1) }); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	out["migration_wall_ms"] = ms(time.Since(migrateStart))
	out["migration_cpu_ms"] = ms(cpuTime() - migrateCPU)
	out["migration_heartbeats"] = beats.Load()
	t.Logf("record index migration: wall %v cpu %v heartbeats %d", time.Since(migrateStart), cpuTime()-migrateCPU, beats.Load())
	if size, err := dirSize(workspace); err == nil {
		out["workspace_mib"] = size >> 20
	}

	live := s.ogReadStore
	unmigrated := objectgraph.NewDoltStore(s.readDB)
	unmigrated.ShareEngineMutex(live)
	measure := func(name string, reps int, fn func() (int, error)) {
		t.Helper()
		for _, mode := range []string{"before", "after"} {
			if mode == "before" {
				s.ogReadStore = unmigrated
			} else {
				s.ogReadStore = live
			}
			var walls, cpus []time.Duration
			rows := 0
			for i := 0; i < reps; i++ {
				c0, w0 := cpuTime(), time.Now()
				n, err := fn()
				walls, cpus = append(walls, time.Since(w0)), append(cpus, cpuTime()-c0)
				if err != nil {
					t.Fatalf("%s %s: %v", name, mode, err)
				}
				rows = n
			}
			key := name + "_" + mode
			out[key+"_cpu_ms"], out[key+"_wall_ms"], out[key+"_rows"] = ms(median(cpus)), ms(median(walls)), rows
			t.Logf("%-34s %-6s cpu %10.2f ms  wall %10.2f ms  rows %d", name, mode, ms(median(cpus)), ms(median(walls)), rows)
		}
		s.ogReadStore = live
	}

	mgmt := "management:" + inboxTestOwner
	head, err := s.InboxHead(ctx, inboxTestOwner, inboxTestComputer, mgmt)
	if err != nil {
		t.Fatal(err)
	}
	out["management_inbox_head"] = head
	runEnd := func(inbox string) func() (int, error) {
		return func() (int, error) {
			if s.ogReadStore == unmigrated {
				rounds, err := s.OpenRounds(ctx, inboxTestOwner, inboxTestComputer, inbox)
				n := 0
				for _, r := range rounds {
					if r.RecipientInbox == inbox {
						n++
					}
				}
				return n, err
			}
			rounds, err := s.OpenRoundsAddressedTo(ctx, inboxTestOwner, inboxTestComputer, inbox)
			return len(rounds), err
		}
	}
	measure("run_end_check_management", 3, runEnd(mgmt))
	measure("run_end_check_texture", 3, runEnd("texture:doc-1"))
	measure("open_rounds_fold_management", 3, func() (int, error) {
		r, err := s.OpenRounds(ctx, inboxTestOwner, inboxTestComputer, mgmt)
		return len(r), err
	})
	measure("records_by_inbox_management", 3, func() (int, error) {
		r, err := s.ogListAllByMetadata(ctx, ogKindCommitmentRecord, "inbox", mgmt)
		return len(r), err
	})
	measure("conversation_state_source", 5, func() (int, error) {
		h, err := s.ConversationState(ctx, inboxTestOwner, inboxTestComputer, types.InboxConversationID(mgmt, "source:s1"))
		return h.Round, err
	})
	measure("cell_inbox_last_10", 5, func() (int, error) {
		r, err := s.ListInboxRecords(ctx, inboxTestOwner, inboxTestComputer, mgmt, head-10, head)
		return len(r), err
	})
	measure("claim_pack_256", 5, func() (int, error) {
		r, err := s.ListCommitmentClaimRecords(ctx, inboxTestOwner, inboxTestComputer, 256)
		return len(r), err
	})

	// AppendEvent as one run grows (C5): CPU per append by depth, and what
	// the old per-append sequence scan (every event of the run, decoded)
	// costs at the same depth.
	runID := "bench-run"
	buckets := []int{50, 200, 400, 800}
	var appendCPU, scanCPU time.Duration
	start := 0
	for i := 1; i <= buckets[len(buckets)-1]; i++ {
		ev := &types.EventRecord{EventID: fmt.Sprintf("%s-%d", runID, i), RunID: runID, OwnerID: inboxTestOwner,
			Kind: types.EventKind("tool.result"), Timestamp: time.Now().UTC(), Payload: benchPayload(i, 2048)}
		c0 := cpuTime()
		if err := s.AppendEvent(ctx, ev); err != nil {
			t.Fatalf("append event %d: %v", i, err)
		}
		appendCPU += cpuTime() - c0
		if ev.Seq != int64(i) {
			t.Fatalf("append %d got seq %d", i, ev.Seq)
		}
		c0 = cpuTime()
		existing, err := s.ogListByMetadata(ctx, ogKindEvent, "run_id", runID, 10000)
		if err != nil {
			t.Fatal(err)
		}
		for _, obj := range existing {
			var prior types.EventRecord
			_ = ogDecode(obj, &prior)
		}
		scanCPU += cpuTime() - c0
		for _, b := range buckets {
			if i == b {
				n := time.Duration(i - start)
				out[fmt.Sprintf("append_event_cpu_ms_%d_to_%d", start+1, b)] = ms(appendCPU / n)
				out[fmt.Sprintf("old_seq_scan_cpu_ms_%d_to_%d", start+1, b)] = ms(scanCPU / n)
				t.Logf("append events %4d..%4d: cpu %6.2f ms per append; old seq scan would add %6.2f ms", start+1, b, ms(appendCPU/n), ms(scanCPU/n))
				appendCPU, scanCPU, start = 0, 0, i
			}
		}
	}

	if outPath := os.Getenv("CHOIR_STORE_SCAN_BENCH_OUT"); outPath != "" {
		raw, _ := json.MarshalIndent(out, "", "  ")
		if err := os.WriteFile(outPath, raw, 0o644); err != nil {
			t.Fatalf("write out: %v", err)
		}
	}
}

func cpuTime() time.Duration {
	var usage syscall.Rusage
	_ = syscall.Getrusage(syscall.RUSAGE_SELF, &usage)
	return time.Duration(usage.Utime.Nano() + usage.Stime.Nano())
}

func median(ds []time.Duration) time.Duration {
	sorted := append([]time.Duration(nil), ds...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	return sorted[len(sorted)/2]
}

func ms(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }

func dirSize(dir string) (int64, error) {
	var total int64
	err := filepath.Walk(dir, func(_ string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	return total, err
}

func benchPayload(i, size int) json.RawMessage {
	raw, _ := json.Marshal(map[string]any{"i": i, "text": strings.Repeat("x", size)})
	return raw
}

// buildScanFixture commits `records` addressed records through the real
// inbox command path (heads, wakes and command receipts included) and two
// events per record directly into the object graph.
func buildScanFixture(t *testing.T, s *Store, records int) {
	t.Helper()
	ctx := context.Background()
	rng := rand.New(rand.NewSource(20261014))
	mgmt := "management:" + inboxTestOwner
	const docs, sources = 40, 28
	type conv struct{ open bool }
	convs := map[string]*conv{}
	// step returns the next record between two desks, keeping the round
	// protocol valid: a cast opens, the recipient reports, a final closes.
	step := func(id, caster, recipient string) types.CommitmentRecord {
		key := types.InboxConversationID(caster, recipient)
		if convs[key] == nil {
			convs[key] = &conv{}
		}
		c := convs[key]
		if !c.open {
			c.open = true
			return inboxCast(id, caster, recipient, types.CommitmentCast{Hypotheses: []types.CastHypothesis{hyp("h "+id, 0.5)}})
		}
		if rng.Intn(4) == 0 {
			c.open = false
			return inboxReport(id, recipient, caster, true, types.CommitmentReport{StopReason: "done", Claims: []types.ReportClaim{{Text: "claim " + id}}})
		}
		return inboxReport(id, recipient, caster, false, types.CommitmentReport{Claims: []types.ReportClaim{{Text: "claim " + id}}})
	}
	batch := make([]types.CommitmentRecord, 0, 250)
	commands := 0
	flush := func() {
		if len(batch) == 0 {
			return
		}
		commands++
		mustCommitInbox(t, s, fmt.Sprintf("bench-cmd-%d", commands), nil, batch...)
		batch = batch[:0]
	}
	for i := 0; i < records; i++ {
		id := fmt.Sprintf("bench-rec-%07d", i)
		doc := 1 + rng.Intn(docs)
		var rec types.CommitmentRecord
		switch r := rng.Intn(10); {
		case r < 5: // capture-style notes from a source to management
			rec = inboxNote(id, fmt.Sprintf("source:s%d", 1+rng.Intn(sources)), mgmt)
		case r < 8:
			rec = step(id, mgmt, fmt.Sprintf("texture:doc-%d", doc))
		case r < 9:
			rec = step(id, fmt.Sprintf("texture:doc-%d", doc), fmt.Sprintf("research:doc-%d", doc))
		default:
			rec = inboxAsk(id, fmt.Sprintf("research:doc-%d", doc), mgmt)
		}
		batch = append(batch, rec)
		if len(batch) == cap(batch) {
			flush()
		}
	}
	flush()

	events := 2 * records
	objects := make([]objectgraph.Object, 0, 2000)
	base := time.Now().UTC().Add(-time.Duration(events) * time.Second)
	for i := 0; i < events; i++ {
		ev := &types.EventRecord{EventID: fmt.Sprintf("fixture-ev-%d", i), RunID: fmt.Sprintf("fixture-run-%d", i%400), OwnerID: inboxTestOwner,
			Seq: int64(i/400 + 1), StreamSeq: int64(i + 1), Kind: types.EventKind("tool.result"),
			ChannelID: "chan-" + strconv.Itoa(i%50), TrajectoryID: "traj-" + strconv.Itoa(i%400),
			Timestamp: base.Add(time.Duration(i) * time.Second), Payload: benchPayload(i, 512)}
		obj, err := buildEventObject(ev, ev.Timestamp)
		if err != nil {
			t.Fatal(err)
		}
		objects = append(objects, obj)
		if len(objects) == cap(objects) || i == events-1 {
			if err := s.ogStore.PutBatch(ctx, objectgraph.Batch{Objects: objects}); err != nil {
				t.Fatalf("put events: %v", err)
			}
			objects = objects[:0]
		}
	}
}
