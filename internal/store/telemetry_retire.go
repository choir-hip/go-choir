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
	"log"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/types"
)

var (
	// telemetryRetireFirstDelay is the wait after boot before the first run.
	telemetryRetireFirstDelay = 10 * time.Minute
	// telemetryRetireInterval is the gap between runs.
	telemetryRetireInterval = 24 * time.Hour
	// telemetryRetireChunkRows bounds the rows one retire op removes: the
	// apply holds the event lock, so the backlog goes in short transactions.
	telemetryRetireChunkRows = 20000
	// telemetryRetireChunkPause lets the desks' appends through between
	// chunks.
	telemetryRetireChunkPause = 250 * time.Millisecond
	// telemetryRetireMaxChunks bounds the ops one group appends in one run
	// (20000 rows each: 10 million rows); the next run continues.
	telemetryRetireMaxChunks = 500
	// telemetryRetireTraceWindow is how long the trace kinds stay (the
	// owner's default, section 11.6).
	telemetryRetireTraceWindow = 14 * 24 * time.Hour
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

// telemetryRetireGroups is the retirement policy (section 11.6).
//
// Kept on purpose, not listed:
//   - tool.result: run acceptance and the Texture verifier read it. TODO:
//     retire it once a run's acceptance can no longer read it (name the
//     window with the acceptance owner); until then it stays.
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
	{name: "phased loop.progress", kinds: []types.EventKind{types.EventRunProgress}, phase: computerevent.ObjectRetirePhaseSome, window: retireTrace},
}

func (g telemetryRetireGroup) projection(now time.Time) computerevent.ObjectRetireProjection {
	kinds := make([]string, len(g.kinds))
	for i, k := range g.kinds {
		kinds[i] = string(k)
	}
	return computerevent.ObjectRetireProjection{
		ObjectKind:    computerevent.ObjectRetireKindEvent,
		EventKinds:    kinds,
		UpdatedBefore: now.Add(-g.window()).UTC().Format(time.RFC3339Nano),
		Phase:         g.phase,
		MaxRows:       telemetryRetireChunkRows,
		Reason:        "telemetry retirement: " + g.name,
	}
}

// RetireTelemetry runs the retirement once: for each group, while a row
// matches, it appends one bounded retire op. It returns the ops appended. A
// store with no bound projection tape retires nothing (the test seam keeps no
// tape, and a delete outside the tape would break replay).
func (s *Store) RetireTelemetry(ctx context.Context, now time.Time) (int, error) {
	if s == nil || s.projectionTape == nil {
		return 0, nil
	}
	appended := 0
	for _, g := range telemetryRetireGroups {
		p := g.projection(now)
		for chunk := 0; chunk < telemetryRetireMaxChunks; chunk++ {
			if err := ctx.Err(); err != nil {
				return appended, err
			}
			found, err := s.retireMatchExists(ctx, p)
			if err != nil {
				return appended, err
			}
			if !found {
				break
			}
			body, err := json.Marshal(p)
			if err != nil {
				return appended, err
			}
			if err := s.AppendProjectionOps(ctx, []computerevent.ProjectionOp{{Kind: computerevent.ProjectionOpObjectRetire, Body: body}}); err != nil {
				return appended, err
			}
			appended++
			select {
			case <-ctx.Done():
				return appended, ctx.Err()
			case <-time.After(telemetryRetireChunkPause):
			}
		}
	}
	return appended, nil
}

// StartTelemetryRetirement runs RetireTelemetry about ten minutes after
// start, then every 24 hours, until ctx ends. Call it once the replay is done
// and the runtime is up. ready, when set, gates each run: a computer whose
// file-tree hydration is pending, or that is otherwise not settled, skips the
// run and tries at the next tick. It never fails the process.
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
			wait = telemetryRetireInterval
			if ready != nil && !ready() {
				log.Printf("store: telemetry retirement: not settled; skipping this run")
				continue
			}
			start := time.Now()
			ops, err := s.RetireTelemetry(ctx, time.Now().UTC())
			if err != nil {
				log.Printf("store: telemetry retirement: after %d ops: %v", ops, err)
				continue
			}
			log.Printf("store: telemetry retirement: %d ops in %s", ops, time.Since(start).Round(time.Millisecond))
		}
	}()
}
