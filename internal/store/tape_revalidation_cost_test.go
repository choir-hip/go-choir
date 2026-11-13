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

// What the head check costs under load, on the production shape (sealed
// batches, the store dry-runs outside the appender mutex). The harness counts
// every dry run the appender makes through its projection: with the sealed
// path the store's own dry run does not pass through it, so each count here is
// a revalidation (a second dry run under the mutex).
func measureRevalidations(t *testing.T, direct bool) (appends int64, revalidations int64) {
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
				case direct:
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
	events := h.store.TelemetryStats().TapeEvents - before
	return events, dry.Load()
}

func TestRevalidationsPerAppendStayNearZeroGrouped(t *testing.T) {
	events, reval := measureRevalidations(t, false)
	ratio := float64(reval) / float64(events)
	t.Logf("grouped: %d tape events, %d revalidations, %.3f per event", events, reval, ratio)
	if events == 0 || ratio > 0.1 {
		t.Fatalf("revalidations per grouped append = %.3f over %d events, want near zero", ratio, events)
	}
}

func TestRevalidationsPerAppendDirect(t *testing.T) {
	events, reval := measureRevalidations(t, true)
	ratio := float64(reval) / float64(events)
	t.Logf("direct: %d tape events, %d revalidations, %.3f per event", events, reval, ratio)
	if events == 0 || ratio > 0.1 {
		t.Fatalf("revalidations per direct append = %.3f over %d events, want near zero", ratio, events)
	}
}
