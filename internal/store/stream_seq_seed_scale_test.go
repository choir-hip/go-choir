package store

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// TestStreamSeqSeedScale is the repeatable artifact of the seed record: it
// builds a store with N events (1.5 KB payloads) and N/3 objects of another
// kind interleaved in time, then prints the plan and the time of the old
// seed's statement beside the new seed's, at each N. Off by default (it
// writes tens of thousands of rows); run it with
//
//	CHOIR_STREAM_SEQ_SEED_SCALE=2000,20000,60000 go test ./internal/store -run TestStreamSeqSeedScale -v -count=1
//
// It asserts the bound: the new read's time must not grow with N past a
// generous factor of its smallest-N time, and the new plan must not sort.
func TestStreamSeqSeedScale(t *testing.T) {
	sizes := os.Getenv("CHOIR_STREAM_SEQ_SEED_SCALE")
	if sizes == "" {
		t.Skip("set CHOIR_STREAM_SEQ_SEED_SCALE=2000,20000 to run")
	}
	var first time.Duration
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
			for i := 0; i < n; i++ {
				ts := base.Add(time.Duration(i) * time.Second)
				rec := &types.EventRecord{EventID: fmt.Sprintf("e-%d", i), RunID: fmt.Sprintf("run-%d", i%50), OwnerID: "alice",
					Seq: int64(i), StreamSeq: int64(i + 1), Timestamp: ts, Kind: types.EventRunProgress,
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

			oldSQL := `SELECT canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, metadata, created_at, updated_at, tombstone, superseded_by FROM og_objects WHERE 1=1 AND object_kind = ? ORDER BY updated_at DESC LIMIT ?`
			t.Logf("n=%d OLD plan:\n%s", n, explainEventTablePlan(t, s, oldSQL, "choir.event", 500))
			t.Logf("n=%d NEW plan:\n%s", n, explainEventTablePlan(t, s, objectgraph.NewestObjectHeadersSQL, streamSeedWindows[0]))
			var oldBest, newBest time.Duration
			for r := 0; r < 3; r++ {
				t0 := time.Now()
				if _, err := s.ogStore.ListObjects(ctx, objectgraph.ListFilter{Kind: ogKindEvent, Limit: 500}); err != nil {
					t.Fatal(err)
				}
				if d := time.Since(t0); r == 0 || d < oldBest {
					oldBest = d
				}
				t0 = time.Now()
				s.streamSeqSeeded, s.maxStreamSeq = false, 0
				if err := s.seedStreamSeq(ctx, nil); err != nil {
					t.Fatal(err)
				}
				if d := time.Since(t0); r == 0 || d < newBest {
					newBest = d
				}
				if s.maxStreamSeq != int64(n) {
					t.Fatalf("seed = %d, want %d", s.maxStreamSeq, n)
				}
			}
			t.Logf("n=%d old seed statement best-of-3 %v; new seed best-of-3 %v", n, oldBest, newBest)
			if first == 0 {
				first = newBest
			} else if newBest > 20*first+50*time.Millisecond {
				t.Fatalf("the seed grew with the event count: %v at n=%d, %v at the first size", newBest, n, first)
			}
		})
	}
}
