package agentcore

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// runEndRetryDelays space the background retries of a run's end write:
// about 15 minutes in all, far inside the hour-long progress sweep that
// otherwise ends a run whose loop is gone.
var runEndRetryDelays = []time.Duration{
	2 * time.Second, 5 * time.Second, 15 * time.Second, 30 * time.Second,
	time.Minute, time.Minute, 2 * time.Minute, 2 * time.Minute,
	2 * time.Minute, 2 * time.Minute, 2 * time.Minute,
}

// retryRunEnd attempts persist until it lands, waiting delays[i] after
// failure i. It stops without error when persist reports the run was not
// persisted (another path already ended it), and returns the last error
// once the delays run out. after runs once, only when the write lands.
func retryRunEnd(persist func() (bool, error), after func(), wait func(time.Duration), delays []time.Duration) (bool, error) {
	for i := 0; ; i++ {
		persisted, err := persist()
		if err == nil {
			if persisted && after != nil {
				after()
			}
			return persisted, nil
		}
		if i >= len(delays) {
			return false, err
		}
		wait(delays[i])
	}
}

// persistRunEndInBackground keeps writing a run's end after the first
// write failed (a transient store or corpusd failure). The loop has ended,
// but a stored run still "running" absorbs its desk's wakes until the
// progress sweep, an hour later
// (problems/failed-terminal-run-write-blocks-desk-for-an-hour-2026-10-16.md).
// after runs the terminal follow-ups once the write lands.
func (rt *Runtime) persistRunEndInBackground(rec types.RunRecord, kind types.EventKind, cause events.EventCause, payload json.RawMessage, after func(context.Context, *types.RunRecord)) {
	go func() {
		ctx := context.Background()
		persisted, err := retryRunEnd(func() (bool, error) {
			return rt.persistActivationStateAndEmit(ctx, &rec, kind, cause, payload)
		}, func() {
			if after != nil {
				after(ctx, &rec)
			}
		}, time.Sleep, runEndRetryDelays)
		switch {
		case err != nil:
			log.Printf("runtime: run %s end (%s) still not persisted after retries: %v", rec.RunID, rec.State, err)
		case persisted:
			log.Printf("runtime: run %s end (%s) persisted on retry", rec.RunID, rec.State)
		}
	}()
}
