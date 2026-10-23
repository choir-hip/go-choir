package store

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Test 37 (measurement) and the plan test of open question 3 of the stream_seq
// lease record (docs/problems/stream-seq-high-water-mark-design-2026-10-23.md).
// Ways it can fail, written before the code:
//  1. The exact seed's og half is not a keyset scan of the kind: its page
//     query sorts the kind, or reads the body, and the cost grows faster than
//     the history.
//  2. The table half is not a per-owner seek: the plan reads every row of
//     choir_events, or sorts them.
//  3. The seed misses the maximum: the answer is the newest-by-tape-position
//     row, or a band.
//  4. The measurement is not written down, so the seeding deadline (3x the
//     measured exact seed) is a guess.

// TestExactStreamSeqSeedScale is the repeatable artifact of test 37. Off by
// default (it writes tens of thousands of rows); run it with
//
//	CHOIR_STREAM_SEQ_SEED_SCALE=20000,60000,150000 go test ./internal/store -run TestExactStreamSeqSeedScale -v -count=1 -timeout 60m
//
// It prints the plan and the best-of-3 time of the exact seed's two halves at
// each size, and asserts the answer is the maximum.
func TestExactStreamSeqSeedScale(t *testing.T) {
	sizes := os.Getenv("CHOIR_STREAM_SEQ_SEED_SCALE")
	if sizes == "" {
		t.Skip("set CHOIR_STREAM_SEQ_SEED_SCALE=20000,60000 to run")
	}
	for _, field := range strings.Split(sizes, ",") {
		n, err := strconv.Atoi(strings.TrimSpace(field))
		if err != nil || n <= 0 {
			t.Fatalf("bad size %q", field)
		}
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			s := openTestStore(t)
			ctx := context.Background()
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
			// og half: n events (1.5 KB payloads) over 50 runs, n/3 objects of
			// another kind interleaved, and the maximum placed at the OLDEST
			// timestamp: the newest-by-updated_at heuristics cannot find it.
			maxStream := int64(n) + 777
			for i := 0; i < n; i++ {
				ts := base.Add(time.Duration(i) * time.Second)
				stream := int64(i + 1)
				if i == 0 {
					stream = maxStream
				}
				rec := &types.EventRecord{EventID: fmt.Sprintf("e-%d", i), RunID: fmt.Sprintf("run-%d", i%50), OwnerID: "alice",
					Seq: int64(i), StreamSeq: stream, Timestamp: ts, Kind: types.EventRunProgress,
					Payload: []byte(`{"x":"` + strings.Repeat("p", 1500) + `"}`)}
				obj, err := buildEventObject(rec, ts)
				if err != nil {
					t.Fatal(err)
				}
				batch.Objects = append(batch.Objects, obj)
				if i%3 == 0 {
					source := objectgraph.Object{OwnerID: scanOwner, ComputerID: scanComputer, CreatedAt: ts, UpdatedAt: ts}
					_, o, err := actorWakeOutbox(source, fmt.Sprintf("src-%06d", i), "texture:doc", "traj", "agent", "coagent_result",
						strings.Repeat("x", 600)+fmt.Sprint(i), time.Time{}, fmt.Sprintf("wake-%06d", i))
					if err != nil {
						t.Fatal(err)
					}
					o.UpdatedAt = ts
					batch.Objects = append(batch.Objects, o)
				}
				if len(batch.Objects) >= 250 {
					flush()
				}
			}
			flush()
			// table half: n/10 rows over 20 owners, maximum 5 above the og one.
			tableRows := n / 10
			tx, err := s.db.BeginTx(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			for i := 0; i < tableRows; i++ {
				stream := int64(i + 1)
				if i == tableRows-1 {
					stream = maxStream + 5
				}
				if _, err := tx.ExecContext(ctx, `INSERT INTO choir_events (tape_seq, op_index, canonical_id, object_kind, owner_id, content_hash, body, metadata, created_at, updated_at, stream_seq)
					VALUES (?, 0, ?, 'choir.event', ?, 'h', '{}', '{}', ?, ?, ?)`,
					uint64(i+1), fmt.Sprintf("obj:choir.event:o:%d", i), fmt.Sprintf("owner-%02d", i%20), base, base, stream); err != nil {
					t.Fatal(err)
				}
			}
			if err := tx.Commit(); err != nil {
				t.Fatal(err)
			}

			t.Logf("n=%d og page plan:\n%s", n, explainEventTablePlan(t, s, objectgraph.KindMetadataPageSQL, "choir.event", "", leaseScanPage))
			sql, args := eventTableOwnerMaxStreamSQL("owner-07")
			t.Logf("n=%d table per-owner plan:\n%s", n, explainEventTablePlan(t, s, sql, args...))

			// The machine's context travels with the number (bunch 169 nit):
			// the same seed on a different disk is a different number.
			t.Logf("MACHINE %s/%s cpus=%d go=%s disk:\n%s", runtime.GOOS, runtime.GOARCH, runtime.NumCPU(), runtime.Version(), diskContext(s.TexturePath()))
			var ogBest, tableBest, bothBest time.Duration
			for r := 0; r < 3; r++ {
				t0 := time.Now()
				og, err := s.exactOGStreamSeqMax(ctx)
				if err != nil {
					t.Fatal(err)
				}
				d := time.Since(t0)
				if r == 0 || d < ogBest {
					ogBest = d
				}
				if og != maxStream {
					t.Fatalf("og half = %d, want the oldest event's %d", og, maxStream)
				}
				t0 = time.Now()
				table, err := s.exactTableStreamSeqMax(ctx)
				if err != nil {
					t.Fatal(err)
				}
				d = time.Since(t0)
				if r == 0 || d < tableBest {
					tableBest = d
				}
				if table != maxStream+5 {
					t.Fatalf("table half = %d, want %d", table, maxStream+5)
				}
				t0 = time.Now()
				if _, _, err := s.exactStreamSeqMax(ctx); err != nil {
					t.Fatal(err)
				}
				d = time.Since(t0)
				if r == 0 || d < bothBest {
					bothBest = d
				}
			}
			t.Logf("EXACT SEED n=%d events (+%d other-kind objects, %d table rows): og half best-of-3 %v; table half best-of-3 %v; both %v",
				n, n/3, tableRows, ogBest, tableBest, bothBest)
		})
	}
}

// Open question 3: the per-owner skip-seek plan on choir_events. The queries
// must be served by idx_choir_events_owner_stream, with no sort and no scan of
// the table.
func TestExactSeedTablePlanSeeksPerOwner(t *testing.T) {
	h := newTapeHarness(t)
	h.appendStamped(
		eventTableRecord("a-1", "run-a", "alpha", "traj-a", types.EventToolResult, 1, 5, eventTableBase.Add(time.Second)),
		eventTableRecord("b-1", "run-b", "bravo", "traj-b", types.EventToolResult, 1, 6, eventTableBase.Add(2*time.Second)),
	)
	nextSQL, nextArgs := eventTableNextOwnerSQL("")
	nextPlan := strings.ToLower(explainEventTablePlan(t, h.store, nextSQL, nextArgs...))
	maxSQL, maxArgs := eventTableOwnerMaxStreamSQL("alpha")
	maxPlan := strings.ToLower(explainEventTablePlan(t, h.store, maxSQL, maxArgs...))
	for name, plan := range map[string]string{"next owner": nextPlan, "owner max": maxPlan} {
		if strings.Contains(plan, "sort") {
			t.Errorf("%s plan sorts:\n%s", name, plan)
		}
		// Dolt names the index by its columns in the plan.
		if !strings.Contains(plan, "indexedtableaccess(choir_events)") || !strings.Contains(plan, "choir_events.owner_id,choir_events.stream_seq") {
			t.Errorf("%s plan is not an index access on (owner_id, stream_seq):\n%s", name, plan)
		}
	}
}

// The og page query: the kind index, no sort on a column the index does not
// carry, no body.
func TestExactSeedPagePlanIsAKeysetRangeOfTheKind(t *testing.T) {
	s := openTestStore(t)
	putStoredEvent(t, s, "e-1", "run", 1, time.Now().UTC())
	pageSQL := objectgraph.KindMetadataPageSQL
	plan := strings.ToLower(explainEventTablePlan(t, s, pageSQL, "choir.event", "", 100))
	if strings.Contains(plan, "sort") && !strings.Contains(plan, "limit") {
		t.Errorf("the page query sorts the kind without a limit:\n%s", plan)
	}
	if strings.Contains(strings.ToLower(pageSQL), "body") {
		t.Errorf("the page query reads the body:\n%s", pageSQL)
	}
	t.Logf("page plan:\n%s", plan)
}

// diskContext is the filesystem the workspace sits on: df for the mount, and
// the device's type where the platform names it.
func diskContext(path string) string {
	out, err := exec.Command("df", "-h", path).CombinedOutput()
	if err != nil {
		return "df failed: " + err.Error()
	}
	text := strings.TrimSpace(string(out))
	if mount, mErr := exec.Command("mount").CombinedOutput(); mErr == nil {
		fields := strings.Fields(strings.SplitN(text, "\n", 2)[len(strings.SplitN(text, "\n", 2))-1])
		if len(fields) > 0 {
			for _, line := range strings.Split(string(mount), "\n") {
				if strings.HasPrefix(line, fields[0]+" ") {
					text += "\n" + line
					break
				}
			}
		}
	}
	return text
}
