package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// An engineering cell's host wall clock
// (docs/problems/engineering-cell-never-returns-2026-11-03.md). Without
// timeout_ms a capsule_go_eval cell was bounded only by the activation
// deadline: the broker RPC's read deadline is the context's, so a broker
// that stopped answering hid the round for up to 60 minutes with no
// tool.result. Every engineering cell now ends at its limit, the model is
// told what the cell waited on and what to do instead, and a cell running
// past the notice is visible as a run progress event. The limit ends the
// host's wait only: it never stops the run's capsule job.

// engineeringCellRunningPhase is the run progress phase of a long cell.
const engineeringCellRunningPhase = "capsule_cell_running"

// engineeringCellDeadlineMargin is the room a cell's limit leaves before
// the activation deadline, so the model reads the limit's answer instead
// of the activation ending under it.
const engineeringCellDeadlineMargin = 30 * time.Second

// errEngineeringCellLimit marks a cell the host ended at its limit.
var errEngineeringCellLimit = errors.New("engineering cell limit reached")

// engineeringCellBounds is a cell's host limit and the interval of its
// "cell running" events.
type engineeringCellBounds struct {
	limit  time.Duration
	notice time.Duration
}

// defaultEngineeringCellBounds: five minutes, with a running event every
// two. A test may shorten them.
var defaultEngineeringCellBounds = engineeringCellBounds{limit: 5 * time.Minute, notice: 2 * time.Minute}

// engineeringCellTimeoutGrace is how long past the model's own timeout_ms
// the host waits: the broker enforces timeout_ms and answers with the
// cell's partial output, so the host must not race it.
const engineeringCellTimeoutGrace = 15 * time.Second

// forCell is the bound for one cell: the model's timeout_ms plus a grace
// when that is smaller than the limit, and never past the activation
// deadline less its margin (with less than the margin left, the
// activation's own deadline).
func (b engineeringCellBounds) forCell(ctx context.Context, timeoutMS int) engineeringCellBounds {
	if timeoutMS > 0 && int64(timeoutMS) < b.limit.Milliseconds() {
		if own := time.Duration(timeoutMS)*time.Millisecond + engineeringCellTimeoutGrace; own < b.limit {
			b.limit = own
		}
	}
	if deadline, ok := ctx.Deadline(); ok {
		left := time.Until(deadline)
		if room := left - engineeringCellDeadlineMargin; room > 0 && room < b.limit {
			b.limit = room
		} else if room <= 0 && left < b.limit {
			b.limit = left
		}
	}
	return b
}

// brokerTimeoutMS is the timeout_ms the broker gets: the model's, but never
// within the grace of the host's limit, so the broker's in-band answer (with
// the cell's partial output) always lands before the host stops waiting.
// Zero keeps the broker's default (60 s).
func (b engineeringCellBounds) brokerTimeoutMS(timeoutMS int) int {
	if timeoutMS == 0 {
		return 0
	}
	ceiling := max((b.limit - engineeringCellTimeoutGrace).Milliseconds(), 1)
	if timeoutMS < 0 || int64(timeoutMS) > ceiling {
		return int(ceiling)
	}
	return timeoutMS
}

// runEngineeringCell runs eval under the cell's bound. eval gets a context
// that ends at the limit; if eval does not return by then (a host call that
// ignores its context, such as a lock or a broker that stopped answering),
// the host stops waiting and answers with errEngineeringCellLimit, naming
// the run's running job. timedOut reports a cell ended by a deadline: its
// own limit or the activation's. The abandoned call finishes, or fails at
// the deadline, on its own goroutine; while a broker stays wedged the next
// cell may wait its own limit behind it on the shared connection. Nothing
// here touches the job.
func runEngineeringCell(ctx context.Context, b engineeringCellBounds, jobs capsuleJobTable, runID string, emit provideriface.EventEmitFunc,
	eval func(context.Context) (capsule.GoEvalResult, error)) (res capsule.GoEvalResult, err error, timedOut bool) {
	cellCtx, cancel := context.WithTimeout(ctx, b.limit)
	defer cancel()
	type outcome struct {
		res capsule.GoEvalResult
		err error
	}
	done := make(chan outcome, 1)
	go func() {
		// The call runs off the tool's stack; a panic in it is the cell's
		// error, not the process's.
		defer func() {
			if r := recover(); r != nil {
				done <- outcome{err: fmt.Errorf("capsule cell call panicked: %v", r)}
			}
		}()
		res, err := eval(cellCtx)
		done <- outcome{res, err}
	}()
	start := time.Now()
	var tick <-chan time.Time
	if b.notice > 0 && b.notice < b.limit {
		ticker := time.NewTicker(b.notice)
		defer ticker.Stop()
		tick = ticker.C
	}
	for {
		select {
		case out := <-done:
			if out.err != nil && ctx.Err() == nil && errors.Is(cellCtx.Err(), context.DeadlineExceeded) {
				// The call itself failed at the limit (the broker read
				// deadline): the same answer as a call that never returned.
				return capsule.GoEvalResult{}, engineeringCellLimitError(b.limit, time.Since(start), jobs, runID, out.err), true
			}
			if out.err != nil && ctx.Err() == nil && errors.Is(out.err, capsule.ErrBrokerNoAnswer) {
				// The broker read hit its own bound: the broker's cell bound
				// plus a slack (broker_answer.go).
				return capsule.GoEvalResult{}, engineeringCellLimitError(b.limit, time.Since(start), jobs, runID, out.err), true
			}
			parentDeadline := out.err != nil && errors.Is(ctx.Err(), context.DeadlineExceeded)
			return out.res, out.err, parentDeadline
		case <-tick:
			emitCellRunning(emit, runID, time.Since(start), b.limit, jobs)
		case <-cellCtx.Done():
			if err := ctx.Err(); err != nil {
				return capsule.GoEvalResult{}, err, errors.Is(err, context.DeadlineExceeded)
			}
			return capsule.GoEvalResult{}, engineeringCellLimitError(b.limit, time.Since(start), jobs, runID, nil), true
		}
	}
}

// emitCellRunning makes a long cell visible to tracers and Texture.
func emitCellRunning(emit provideriface.EventEmitFunc, runID string, elapsed, limit time.Duration, jobs capsuleJobTable) {
	if emit == nil {
		return
	}
	body := map[string]any{
		"run_id": runID, "elapsed_ms": elapsed.Milliseconds(), "limit_ms": limit.Milliseconds(),
		"note": fmt.Sprintf("cell running for %d min", int(elapsed.Round(time.Minute)/time.Minute)),
	}
	if jobs != nil {
		if view, _, running := jobs.RunningJob(runID); running {
			body["job_id"] = view.ID
		}
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return
	}
	emit(types.EventRunProgress, engineeringCellRunningPhase, payload)
}

// engineeringCellLimitError is what the model reads when its cell hit the
// host limit: what the cell was waiting on, that the job was left running,
// and the next step.
func engineeringCellLimitError(limit, elapsed time.Duration, jobs capsuleJobTable, runID string, cause error) error {
	waited := "it was waiting on the capsule's answer to the cell, which had not come"
	switch {
	case errors.Is(cause, capsule.ErrBrokerNoAnswer):
		waited = fmt.Sprintf("the capsule broker did not answer the cell (%v)", cause)
	case cause != nil:
		waited = fmt.Sprintf("it was waiting on the capsule's answer to the cell (%v)", cause)
	}
	head := fmt.Sprintf("%v: the host stopped waiting for this cell after %s (limit %s); %s. "+
		"The cell may still have run in part, so values it defines may not exist.",
		errEngineeringCellLimit, elapsed.Round(time.Millisecond), limit, waited)
	if jobs != nil {
		if view, _, running := jobs.RunningJob(runID); running {
			argv := strings.Join(view.Argv, " ")
			if len(argv) > 120 {
				argv = argv[:120] + "..."
			}
			return fmt.Errorf("%w%s Your capsule job %s (%s) is still running; it was not stopped. "+
				"Do not wait for a job inside a cell: use choir.Poll and end your turn; the platform wakes you when the job ends",
				errEngineeringCellLimit, strings.TrimPrefix(head, errEngineeringCellLimit.Error()), view.ID, argv)
		}
	}
	return fmt.Errorf("%w%s No capsule job of yours is running, so nothing will wake you if you end your turn. "+
		"Run a smaller step per cell, and start long work with choir.Start so it runs as a job",
		errEngineeringCellLimit, strings.TrimPrefix(head, errEngineeringCellLimit.Error()))
}
