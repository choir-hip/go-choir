package store

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

const (
	costGrouped = iota
	costDirect
	costMixed
)

// What the head check costs under load, on the production shape (sealed
// batches). A grouped append is dry-run by the store outside the appender mutex
// (the leader's dry run and its turn are adjacent); the appender repeats the
// dry run only if the head moved: those are revalidations
// (TelemetryStats.AppenderRevalidations). A direct append is dry-run once, by
// the appender, under its mutex. The harness counts every dry run that passes
// through the appender's projection.
func measureRevalidations(t *testing.T, mode int) (events, appenderDryRuns, revalidations int64) {
	h := newTapeHarness(t)
	h.useSealing()
	var dry atomic.Int64
	h.dryRunHook = func(*computerevent.ProjectionBatch) error { dry.Add(1); return nil }
	h.warm()
	dry.Store(0)
	before := h.store.TelemetryStats().TapeEvents
	const workers, each = 8, 25
	runs := distinctRunMemoryRuns(h.store, workers)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < each; i++ {
				id := fmt.Sprintf("cost-%d-%d", w, i)
				var err error
				switch {
				case mode == costDirect || (mode == costMixed && w%2 == 0):
					now := time.Now().UTC()
					obj, oerr := lifecycleObject(ogKindMigrationMarker, "owner-1", h.computerID, id, map[string]string{"x": "y"}, map[string]any{}, now, now)
					if oerr != nil {
						t.Error(oerr)
						return
					}
					err = h.store.ogStore.PutBatchConditional(context.Background(),
						[]objectgraph.ObjectCondition{{CanonicalID: obj.CanonicalID}}, objectgraph.Batch{Objects: []objectgraph.Object{obj}})
				case i%2 == 0:
					_, err = h.store.AppendRunMemoryEntry(context.Background(), rmEntry(id, runs[w]))
				default:
					err = h.store.AppendEvent(context.Background(), h.event(id, "run-"+id, "owner-1", types.EventRunCompleted, telemetryBase))
				}
				if err != nil {
					t.Errorf("%s: %v", id, err)
				}
			}
		}(w)
	}
	wg.Wait()
	stats := h.store.TelemetryStats()
	return stats.TapeEvents - before, dry.Load(), stats.AppenderRevalidations
}

func TestRevalidationsPerAppendStayNearZeroGrouped(t *testing.T) {
	events, dry, reval := measureRevalidations(t, costGrouped)
	t.Logf("grouped: %d tape events, %d dry runs under the mutex, %d revalidations (%.3f per event)", events, dry, reval, float64(reval)/float64(events))
	if events == 0 || float64(reval)/float64(events) > 0.1 {
		t.Fatalf("revalidations per grouped event = %d/%d, want near zero", reval, events)
	}
	if dry != reval {
		t.Fatalf("%d dry runs under the mutex but %d revalidations: a grouped append is dry-run outside it", dry, reval)
	}
}

// A direct append is dry-run once, under the mutex: no repeat, one dry run an
// event.
func TestDirectAppendsAreDryRunOnceUnderTheMutex(t *testing.T) {
	events, dry, reval := measureRevalidations(t, costDirect)
	t.Logf("direct: %d tape events, %d dry runs under the mutex, %d revalidations", events, dry, reval)
	if events == 0 || reval != 0 || dry != events {
		t.Fatalf("direct: %d events, %d dry runs under the mutex, %d revalidations; want one dry run per event and no revalidation", events, dry, reval)
	}
}

// Grouped and direct writers at once, as production has them (direct appends
// are the minority there; here half the workers). A direct writer can take the
// mutex between a leader's dry run and its turn, which is when a revalidation
// happens. The ratio is logged and loosely bounded.
func TestRevalidationsWithGroupedAndDirectWritersMixed(t *testing.T) {
	events, dry, reval := measureRevalidations(t, costMixed)
	t.Logf("mixed: %d tape events, %d dry runs under the mutex, %d revalidations (%.3f per event)", events, dry, reval, float64(reval)/float64(events))
	if events == 0 || float64(reval)/float64(events) > 0.5 {
		t.Fatalf("revalidations per event = %d/%d under a mixed load", reval, events)
	}
}
