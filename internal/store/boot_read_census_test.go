package store

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// TestBootReadCensus is the repeatable artifact of
// docs/problems/boot-reads-with-the-seed-shape-2026-10-23.md. It measures the
// three reads the stream_seq seed record named as having the seed's shape (a
// read of a whole kind or a whole owner under the engine mutex at boot or on
// first use), on one local store seeded to a size, and prints each one's plan
// and time. Off by default (it writes tens of thousands of rows). Run it with
//
//	CHOIR_BOOT_READ_CENSUS=20000:5000,60000:20000 go test ./internal/store -run TestBootReadCensus -v -count=1 -timeout 30m
//
// Each size is EVENTS:RUNS: EVENTS events for one owner (1.5 KB payloads,
// spread over EVENTS/200 runs, plus three runs of 100, 1,000 and 10,000
// events), EVENTS/3 actor-wake-outbox rows, and RUNS run headers of which 21
// are pending or running. It asserts nothing about time: a time bound on the
// embedded engine is flaky; the numbers and the plans are the artifact. It
// does assert the plan facts the record rests on, so a change of plan shows.
func TestBootReadCensus(t *testing.T) {
	sizes := os.Getenv("CHOIR_BOOT_READ_CENSUS")
	if sizes == "" {
		t.Skip("set CHOIR_BOOT_READ_CENSUS=20000:5000 to run")
	}
	for _, field := range strings.Split(sizes, ",") {
		pair := strings.Split(strings.TrimSpace(field), ":")
		if len(pair) != 2 {
			t.Fatalf("bad size %q, want EVENTS:RUNS", field)
		}
		events, err1 := strconv.Atoi(pair[0])
		runs, err2 := strconv.Atoi(pair[1])
		if err1 != nil || err2 != nil || events <= 0 || runs < 21 {
			t.Fatalf("bad size %q", field)
		}
		t.Run(field, func(t *testing.T) { bootReadCensus(t, events, runs) })
	}
}

const censusOwner, censusComputer = "owner-census", "computer-census"

type censusAlloc struct{ bytes, mallocs uint64 }

func censusMem() censusAlloc {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return censusAlloc{m.TotalAlloc, m.Mallocs}
}

// timeRead runs fn three times and returns the first (coldest in-process) and
// the best duration, with the bytes the first call allocated.
func timeRead(t *testing.T, fn func() int) (first, best time.Duration, rows int, bytes uint64) {
	t.Helper()
	for r := 0; r < 3; r++ {
		runtime.GC()
		m0, t0 := censusMem(), time.Now()
		rows = fn()
		d := time.Since(t0)
		if r == 0 {
			first, best, bytes = d, d, censusMem().bytes-m0.bytes
		} else if d < best {
			best = d
		}
	}
	return first, best, rows, bytes
}

func bootReadCensus(t *testing.T, nEvents, nRuns int) {
	s := openTestStore(t)
	ctx := context.Background()
	// The generated columns and indexes a migrated guest has live. A fresh
	// Open may not have them yet; the paper logs "derived access paths
	// complete (live: ... og_meta_run_id ...)" at every boot.
	if _, err := s.ogStore.MigrateAccessPaths(ctx, nil); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	s.inheritAccessPaths()
	t.Logf("live access paths: %v", s.ogStore.LiveAccessPaths())

	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var batch objectgraph.Batch
	flush := func() {
		if len(batch.Objects) > 0 {
			if err := s.ogStore.PutBatch(ctx, batch); err != nil {
				t.Fatal(err)
			}
			batch = objectgraph.Batch{}
		}
	}
	payload := []byte(`{"x":"` + strings.Repeat("p", 1500) + `"}`)
	streamSeq := int64(0)
	addEvent := func(runID string, seq int64, ts time.Time) {
		streamSeq++
		rec := &types.EventRecord{EventID: fmt.Sprintf("e-%s-%d", runID, seq), RunID: runID, OwnerID: censusOwner,
			Seq: seq, StreamSeq: streamSeq, Timestamp: ts, Kind: types.EventRunProgress, Payload: payload}
		obj, err := buildEventObject(rec, ts)
		if err != nil {
			t.Fatal(err)
		}
		obj.ComputerID = censusComputer
		batch.Objects = append(batch.Objects, obj)
		if len(batch.Objects) >= 250 {
			flush()
		}
	}
	seedStart := time.Now()
	spread := max(1, nEvents/200)
	perRun := map[string]int64{}
	for i := 0; i < nEvents; i++ {
		ts := base.Add(time.Duration(i) * time.Second)
		runID := fmt.Sprintf("run-bg-%d", i%spread)
		perRun[runID]++
		addEvent(runID, perRun[runID], ts)
		if i%3 == 0 {
			source := objectgraph.Object{OwnerID: censusOwner, ComputerID: censusComputer, CreatedAt: ts, UpdatedAt: ts}
			_, o, err := actorWakeOutbox(source, fmt.Sprintf("src-%06d", i), "texture:doc", "traj", "agent", "coagent_result",
				strings.Repeat("x", 600)+fmt.Sprint(i), time.Time{}, fmt.Sprintf("wake-%06d", i))
			if err != nil {
				t.Fatal(err)
			}
			o.UpdatedAt = ts
			batch.Objects = append(batch.Objects, o)
		}
	}
	// The three runs whose per-run seed is measured, newest in time.
	sized := map[string]int{"run-k100": 100, "run-k1000": 1000, "run-k10000": 10000}
	at := nEvents
	for _, name := range []string{"run-k100", "run-k1000", "run-k10000"} {
		for j := 1; j <= sized[name]; j++ {
			addEvent(name, int64(j), base.Add(time.Duration(at)*time.Second))
			at++
		}
	}
	flush()
	totalEvents := int(streamSeq)
	// Run headers. 21 are live (pending or running), the rest finished, as on
	// the paper at a planned restart (candidates=21).
	runBody := []byte(`{"prompt":"` + strings.Repeat("r", 1800) + `"}`)
	for i := 0; i < nRuns; i++ {
		state := types.RunCompleted
		if i%(nRuns/21) == 0 && i/(nRuns/21) < 21 {
			state = types.RunRunning
		}
		runID := fmt.Sprintf("run-hdr-%06d", i)
		ts := base.Add(time.Duration(i) * time.Minute)
		rec := types.RunRecord{RunID: runID, AgentID: "agent-" + strconv.Itoa(i%40), OwnerID: censusOwner, ComputerID: censusComputer,
			State: state, CreatedAt: ts, UpdatedAt: ts, AgentProfile: "texture"}
		body, err := json.Marshal(rec)
		if err != nil {
			t.Fatal(err)
		}
		body = append(body[:len(body)-1], []byte(`,"pad":`+string(runBody)+`}`)...)
		meta, err := objectgraph.NormalizeMetadata(map[string]any{
			"run_id": runID, "agent_id": rec.AgentID, "computer_id": censusComputer, "state": string(state),
			"agent_profile": "texture", "created_at": ts.Format(time.RFC3339Nano), "updated_at": ts.Format(time.RFC3339Nano)})
		if err != nil {
			t.Fatal(err)
		}
		hash := objectgraph.ContentHash(ogKindRun, body, meta)
		id, err := objectgraph.BuildCanonicalID(ogKindRun, censusOwner, objectgraph.StableSuffixFromKey(runID))
		if err != nil {
			t.Fatal(err)
		}
		batch.Objects = append(batch.Objects, objectgraph.Object{CanonicalID: id, ObjectKind: ogKindRun, OwnerID: censusOwner,
			ComputerID: censusComputer, ContentHash: hash, Body: body, Metadata: meta, CreatedAt: ts, UpdatedAt: ts})
		if len(batch.Objects) >= 250 {
			flush()
		}
	}
	flush()
	t.Logf("seeded %d events (%d runs of %d, plus runs of 100/1000/10000), %d outbox rows, %d run headers in %v",
		totalEvents, spread, nEvents/spread, (nEvents+2)/3, nRuns, time.Since(seedStart).Round(time.Second))

	report := func(name string, first, best time.Duration, rows int, bytes uint64) {
		t.Logf("n=%d runs=%d %-58s first %9.1f ms  best %9.1f ms  rows %6d  alloc %7.1f MiB",
			nEvents, nRuns, name, ms(first), ms(best), rows, float64(bytes)/(1<<20))
	}

	// ---- Suspect 1: the per-run seq seed (allocateEventSeqLocked) ----------
	// The block as it is on main: ogListByMetadata(event, run_id, run, 10000),
	// then a decode of every body for the maximum seq.
	liveRead := s.ogReadStore
	unmigrated := objectgraph.NewDoltStore(s.readDB)
	unmigrated.ShareEngineMutex(liveRead)
	perRunSeed := func(runID string) func() int {
		return func() int {
			objs, err := s.ogListByMetadata(ctx, ogKindEvent, "run_id", runID, 10000)
			if err != nil {
				t.Fatal(err)
			}
			var maxSeq int64
			for _, obj := range objs {
				var ev types.EventRecord
				if ogDecode(obj, &ev) == nil && ev.OwnerID == censusOwner && ev.Seq > maxSeq {
					maxSeq = ev.Seq
				}
			}
			return len(objs)
		}
	}
	runSeedSQLLive := `SELECT canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, metadata, created_at, updated_at, tombstone, superseded_by FROM og_objects
		 WHERE object_kind = ? AND og_meta_run_id = ?
		 ORDER BY updated_at DESC LIMIT ?`
	runSeedSQLJSON := `SELECT canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, metadata, created_at, updated_at, tombstone, superseded_by FROM og_objects
		 WHERE object_kind = ? AND JSON_UNQUOTE(JSON_EXTRACT(CAST(metadata AS JSON), ?)) = ?
		 ORDER BY updated_at DESC LIMIT ?`
	plan := explainEventTablePlan(t, s, runSeedSQLLive, "choir.event", "run-k1000", 10000)
	t.Logf("per-run seed, column live, plan:\n%s", plan)
	if !strings.Contains(plan, "idx_og_objects_og_meta_run_id") && !strings.Contains(plan, "og_meta_run_id") {
		t.Errorf("per-run seed does not read the run_id index:\n%s", plan)
	}
	t.Logf("per-run seed, JSON_EXTRACT fallback, plan:\n%s", explainEventTablePlan(t, s, runSeedSQLJSON, "choir.event", "$.run_id", "run-k1000", 10000))
	for _, name := range []string{"run-k100", "run-k1000", "run-k10000", "run-bg-0", "run-never-seen"} {
		f, b, r, by := timeRead(t, perRunSeed(name))
		report("per-run seed, live column: "+name, f, b, r, by)
	}
	// The engine-mutex part alone (the query and its row scan), without the
	// decode that follows it under eventMu only.
	for _, name := range []string{"run-k1000", "run-k10000"} {
		name := name
		f, b, r, by := timeRead(t, func() int {
			objs, err := s.ogListByMetadata(ctx, ogKindEvent, "run_id", name, 10000)
			if err != nil {
				t.Fatal(err)
			}
			return len(objs)
		})
		report("per-run seed ENGINE HOLD (query only): "+name, f, b, r, by)
	}
	s.ogReadStore = unmigrated
	for _, name := range []string{"run-k1000", "run-k10000"} {
		f, b, r, by := timeRead(t, perRunSeed(name))
		report("per-run seed, JSON_EXTRACT (column not live): "+name, f, b, r, by)
	}
	s.ogReadStore = liveRead

	// ---- Suspect 2: ListEventsByOwnerAfter -----------------------------------
	ownerAfterSQL := `SELECT canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, metadata, created_at, updated_at, tombstone, superseded_by FROM og_objects WHERE 1=1 AND object_kind = ? AND owner_id = ? ORDER BY updated_at DESC LIMIT ?`
	t.Logf("ListEventsByOwnerAfter og half, plan:\n%s", explainEventTablePlan(t, s, ownerAfterSQL, "choir.event", censusOwner, 100000))
	// A reconnecting client is a little behind the head; a client that is a
	// long way behind asks for more but the read is the same.
	after := func(cursor int64) func() int {
		return func() int {
			recs, err := s.ListEventsByOwnerAfter(ctx, censusOwner, cursor, 500)
			if err != nil {
				t.Fatal(err)
			}
			return len(recs)
		}
	}
	f, b, r, by := timeRead(t, after(int64(totalEvents)-50))
	report("ListEventsByOwnerAfter(head-50, 500) whole call", f, b, r, by)
	f, b, r, by = timeRead(t, func() int {
		objs, err := s.og.ListObjects(ctx, objectgraph.ListFilter{Kind: ogKindEvent, OwnerID: censusOwner, Limit: 100000})
		if err != nil {
			t.Fatal(err)
		}
		return len(objs)
	})
	report("ListEventsByOwnerAfter ENGINE HOLD (ListObjects only)", f, b, r, by)
	f, b, r, by = timeRead(t, func() int {
		objs, err := s.og.ListObjects(ctx, objectgraph.ListFilter{Kind: ogKindEvent, OwnerID: censusOwner, Limit: 200})
		if err != nil {
			t.Fatal(err)
		}
		return len(objs)
	})
	report("ListEventsByOwner og half (limit 200, for scale)", f, b, r, by)

	// ---- Suspect 3: the boot run-header list ---------------------------------
	hdrSQL := `SELECT canonical_id, computer_id, metadata, updated_at, tombstone FROM og_objects WHERE object_kind = ? AND owner_id = ? ORDER BY updated_at DESC`
	t.Logf("boot run-header list, plan:\n%s", explainEventTablePlan(t, s, hdrSQL, "choir.run", censusOwner))
	f, b, r, by = timeRead(t, func() int {
		runs, err := s.ListRunsByOwnerStates(ctx, censusOwner, censusComputer,
			[]types.RunState{types.RunPending, types.RunRunning}, ListAllRunHeaders)
		if err != nil {
			t.Fatal(err)
		}
		return len(runs)
	})
	report("ListRunsByOwnerStates(pending,running, ListAllRunHeaders)", f, b, r, by)
	f, b, r, by = timeRead(t, func() int {
		refs, err := s.ogReadStore.ListAllObjectRefsByKindOwner(ctx, "choir.run", censusOwner)
		if err != nil {
			t.Fatal(err)
		}
		return len(refs)
	})
	report("  of which ListAllObjectRefsByKindOwner alone", f, b, r, by)

	// ---- For scale: the seed statement the fixed lane replaced ---------------
	f, b, r, by = timeRead(t, func() int {
		objs, err := s.ogStore.ListObjects(ctx, objectgraph.ListFilter{Kind: ogKindEvent, Limit: 500})
		if err != nil {
			t.Fatal(err)
		}
		return len(objs)
	})
	report("stream_seq seed statement (fixed on lane, still on main)", f, b, r, by)
}
