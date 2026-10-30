package store

// The telemetry retirement job: once a day the guest appends the retire ops
// (object_retire.go) that take aged, unread choir.event lines out of the
// working store (docs/design/object-graph-redesign-2026-10-30.md sections
// 11.3 and 11.6). It appends through the bound projection tape like every
// other guest-originated projection batch, so the retirement is a tape event
// and replay agrees on what was dropped. It never writes SQL around the tape.
//
// Space: SQLite here has no auto_vacuum, so a delete frees pages inside the
// file and does not shrink it. The job does not VACUUM; a reclaim of file
// size is its own act.

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/types"
)

var (
	// telemetryRetireFirstDelay is the wait after boot before the first run.
	telemetryRetireFirstDelay = 10 * time.Minute
	// telemetryRetireInterval is the gap between runs.
	telemetryRetireInterval = 24 * time.Hour
	// telemetryRetireChunkPause lets the desks' appends through between
	// chunks.
	telemetryRetireChunkPause = 250 * time.Millisecond
	// telemetryRetireMaxSlices bounds the ops one group appends in one run
	// (one per hour slice that has a row); the next run continues.
	telemetryRetireMaxSlices = 2000
	// telemetryRetireSlice is the slice one op covers.
	telemetryRetireSlice = time.Hour
	// telemetryRetireTraceWindow is how long the trace kinds stay (the
	// owner's default, section 11.6).
	telemetryRetireTraceWindow = 14 * 24 * time.Hour
	// telemetryRetireToolResultWindow is how long tool.result lines stay.
	telemetryRetireToolResultWindow = 7 * 24 * time.Hour
)

// telemetryRetireGroup is one retirement the job runs: the kinds, the phase
// filter, and how long the lines stay (0 retires every existing one).
type telemetryRetireGroup struct {
	name   string
	kinds  []types.EventKind
	phase  string
	window func() time.Duration
}

func retireNow() time.Duration   { return 0 }
func retireTrace() time.Duration { return telemetryRetireTraceWindow }
func retireToolResult() time.Duration {
	return telemetryRetireToolResultWindow
}

// telemetryRetireGroups is the retirement policy (section 11.6).
//
// tool.result goes after 7 days (L1 step 2). Its readers are the run
// acceptance synthesis (agentcore/run_acceptance.go, an on-demand POST over the
// newest 3000 events of a trajectory, run after a deploy proof), the
// search-provider summary built inside it (trace_helpers.go) and the Texture
// workflow verifier (textureowner/texture_workflow_verifier.go, at the end of
// a run); all three read a run's results in the session that proves it, and a
// stored acceptance record is not recomputed. A week is far past that. The
// row stays on the sealed tape in the batch that recorded it.
//
// Kept on purpose, not listed:
//   - the run lifecycle, Texture revisions, judgments and channel lines:
//     product reads.
//
// loop.progress is split by phase, as the cost page needs: /api/costs reads
// the newest events by owner (at most 10,000) and extracts the "tool_loop",
// "jev_judgment" and "execution" lines, all phased. A phaseless progress line
// has no reader, so it goes at once; a phased one stays 14 days, far longer
// than the cost page's newest-10,000 window reaches.
var telemetryRetireGroups = []telemetryRetireGroup{
	{name: "unread kinds", kinds: []types.EventKind{types.EventToolInvoked, types.EventTextureAgentRevisionProgress}, window: retireNow},
	{name: "phaseless loop.progress", kinds: []types.EventKind{types.EventRunProgress}, phase: computerevent.ObjectRetirePhaseNone, window: retireNow},
	{name: "trace kinds", kinds: []types.EventKind{types.EventDeskCell, types.EventDeskEgress}, window: retireTrace},
	{name: "tool results", kinds: []types.EventKind{types.EventToolResult}, window: retireToolResult},
	{name: "phased loop.progress", kinds: []types.EventKind{types.EventRunProgress}, phase: computerevent.ObjectRetirePhaseSome, window: retireTrace},
}

func (g telemetryRetireGroup) projection(from, before time.Time) computerevent.ObjectRetireProjection {
	kinds := make([]string, len(g.kinds))
	for i, k := range g.kinds {
		kinds[i] = string(k)
	}
	return computerevent.ObjectRetireProjection{
		ObjectKind:    computerevent.ObjectRetireKindEvent,
		EventKinds:    kinds,
		UpdatedFrom:   from.UTC().Format(time.RFC3339Nano),
		UpdatedBefore: before.UTC().Format(time.RFC3339Nano),
		Phase:         g.phase,
		Reason:        "telemetry retirement: " + g.name,
	}
}

// retireCleared is, per group, the time below which this process has retired
// every matching line: a later run starts an hour before it, not at the
// oldest row, which a kept kind (tool.result) holds back forever. A restart
// forgets it and the first run starts from the oldest row again.
var retireCleared sync.Map // group name -> time.Time

// RetireTelemetry runs the retirement once. For each group it finds the
// oldest candidate row (read pool, outside any write transaction), then
// appends one retire op per hour slice from that hour up to the group's
// cutoff, skipping a slice with no matching row. It returns the ops
// appended. A store with no bound projection tape retires nothing (the test
// seam keeps no tape, and a delete outside the tape would break replay).
func (s *Store) RetireTelemetry(ctx context.Context, now time.Time) (int, error) {
	if s == nil || s.projectionTape == nil {
		return 0, nil
	}
	appended := 0
	for _, g := range telemetryRetireGroups {
		cutoff := now.Add(-g.window()).UTC()
		var start time.Time
		if v, ok := retireCleared.Load(s.retireClearedKey(g)); ok {
			start = v.(time.Time).Add(-time.Hour)
		} else {
			oldest, err := s.retireOldest(ctx, cutoff)
			if err != nil {
				return appended, err
			}
			start = oldest
		}
		if start.IsZero() || !start.Before(cutoff) {
			continue
		}
		t := start.Truncate(telemetryRetireSlice)
		ops := 0
		for t.Before(cutoff) && ops < telemetryRetireMaxSlices {
			if err := ctx.Err(); err != nil {
				return appended, err
			}
			end := t.Add(telemetryRetireSlice)
			if end.After(cutoff) {
				end = cutoff
			}
			p := g.projection(t, end)
			found, err := s.retireMatchExists(ctx, p)
			if err != nil {
				return appended, err
			}
			if found {
				body, err := json.Marshal(p)
				if err != nil {
					return appended, err
				}
				if err := s.AppendProjectionOps(ctx, []computerevent.ProjectionOp{{Kind: computerevent.ProjectionOpObjectRetire, Body: body}}); err != nil {
					return appended, err
				}
				appended++
				ops++
				select {
				case <-ctx.Done():
					return appended, ctx.Err()
				case <-time.After(telemetryRetireChunkPause):
				}
			}
			t = end
		}
		// Everything before t is retired for this group.
		retireCleared.Store(s.retireClearedKey(g), t)
	}
	return appended, nil
}

// retireClearedKey scopes the watermark to this store and group.
func (s *Store) retireClearedKey(g telemetryRetireGroup) string {
	return fmt.Sprintf("%p/%s", s, g.name)
}

// StartTelemetryRetirement runs RetireTelemetry about ten minutes after
// start, then every 24 hours, until ctx ends. Call it once the replay is done
// and the runtime is up. ready, when set, gates each run: a computer whose
// file-tree hydration is pending, or whose store.telemetry_retire setting is
// off, skips the run and looks again ten minutes later. It never fails the
// process.
func (s *Store) StartTelemetryRetirement(ctx context.Context, ready func() bool) {
	if s == nil {
		return
	}
	go func() {
		wait := telemetryRetireFirstDelay
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(wait):
			}
			if ready != nil && !ready() {
				// Not settled, or the setting is off: look again soon, so
				// turning it on takes effect without a restart.
				wait = telemetryRetireFirstDelay
				continue
			}
			wait = telemetryRetireInterval
			start := time.Now()
			ops, err := s.RetireTelemetry(ctx, time.Now().UTC())
			if err != nil {
				log.Printf("store: telemetry retirement: after %d ops: %v", ops, err)
				continue
			}
			log.Printf("store: telemetry retirement: %d ops in %s", ops, time.Since(start).Round(time.Millisecond))
			memStart := time.Now()
			memOps, memErr := s.RetireRunMemory(ctx, time.Now().UTC())
			if memErr != nil {
				log.Printf("store: run memory retirement: after %d ops: %v", memOps, memErr)
				continue
			}
			log.Printf("store: run memory retirement: %d ops in %s", memOps, time.Since(memStart).Round(time.Millisecond))
			// Pages the retirements freed go back to the filesystem, bounded
			// (compact.go; only a compacted file has incremental auto_vacuum).
			if freed, vacErr := s.reclaimFreePages(ctx); vacErr != nil {
				log.Printf("store: incremental vacuum: after %d pages: %v", freed, vacErr)
			} else if freed > 0 {
				log.Printf("store: incremental vacuum: %d pages returned", freed)
			}
		}
	}()
}

// TelemetryRetireOn reports whether the store.telemetry_retire setting is on
// for this computer. It is read at each run; off on any read failure.
func (s *Store) TelemetryRetireOn(ctx context.Context, ownerID string) bool {
	return s.graphHomeOn(ctx, ownerID, SettingTelemetryRetire)
}
