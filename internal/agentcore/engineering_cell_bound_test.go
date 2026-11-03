package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// docs/problems/engineering-cell-never-returns-2026-11-03.md: an engineering
// cell whose host call never returned hid the round until its 60-minute
// activation deadline. Failure modes written before the code:
//   - the cell's call ignores its context and never returns: the host must
//     still answer at its limit, not at the activation deadline;
//   - the answer must name what the cell waited on (the running job) and
//     the next step (choir.Poll, end the turn, be woken);
//   - the limit must not stop the job: it keeps running after the cell ends;
//   - a long cell must be visible before the limit (a "cell running" event);
//   - an activation with less time left than the limit bounds the cell first;
//   - a model timeout_ms below the limit is kept, one above it is capped;
//   - a cell that returns in time is passed through untouched.

type cellRunningEvents struct {
	mu     sync.Mutex
	bodies []map[string]any
}

func (e *cellRunningEvents) emit(kind types.EventKind, phase string, payload json.RawMessage) {
	if kind != types.EventRunProgress || phase != engineeringCellRunningPhase {
		return
	}
	var body map[string]any
	_ = json.Unmarshal(payload, &body)
	e.mu.Lock()
	defer e.mu.Unlock()
	e.bodies = append(e.bodies, body)
}

func (e *cellRunningEvents) count() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.bodies)
}

func TestEngineeringCellThatNeverReturnsEndsAtItsLimitAndTheJobRuns(t *testing.T) {
	jobs := runningBuild(time.Now().Add(10 * time.Minute))
	events := &cellRunningEvents{}
	release := make(chan struct{})
	defer close(release)
	bound := engineeringCellBounds{limit: 300 * time.Millisecond, notice: 100 * time.Millisecond}

	start := time.Now()
	type out struct {
		res capsule.GoEvalResult
		err error
	}
	done := make(chan out, 1)
	go func() {
		res, err, timedOut := runEngineeringCell(context.Background(), bound, jobs, "run-1", events.emit,
			func(context.Context) (capsule.GoEvalResult, error) {
				<-release // hang-guard: the cell ignores its context, as the stuck one did
				return capsule.GoEvalResult{}, nil
			})
		if !timedOut {
			t.Error("the limit was not reported as a timeout")
		}
		done <- out{res, err}
	}()
	var got out
	select {
	case got = <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the cell did not return at its limit")
	}
	if elapsed := time.Since(start); elapsed < bound.limit || elapsed > bound.limit+2*time.Second {
		t.Fatalf("returned after %s, want about %s", elapsed, bound.limit)
	}
	if got.err == nil || !errors.Is(got.err, errEngineeringCellLimit) {
		t.Fatalf("err = %v, want the cell limit", got.err)
	}
	msg := got.err.Error()
	for _, want := range []string{"job-build", "still running", "choir.Poll", "end your turn", "wakes you"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("message %q lacks %q", msg, want)
		}
	}
	if _, _, running := jobs.RunningJob("run-1"); !running {
		t.Fatal("the cell's limit stopped the job")
	}
	if events.count() == 0 {
		t.Fatal("no cell-running event before the limit")
	}
}

func TestEngineeringCellReturningInTimePassesThrough(t *testing.T) {
	events := &cellRunningEvents{}
	bound := engineeringCellBounds{limit: 2 * time.Second, notice: time.Second}
	res, err, timedOut := runEngineeringCell(context.Background(), bound, nil, "run-1", events.emit,
		func(ctx context.Context) (capsule.GoEvalResult, error) {
			if _, ok := ctx.Deadline(); !ok {
				t.Error("the cell's context has no deadline")
			}
			return capsule.GoEvalResult{Stdout: "ok"}, nil
		})
	if err != nil || res.Stdout != "ok" || timedOut {
		t.Fatalf("got %+v, %v", res, err)
	}
	if events.count() != 0 {
		t.Fatal("a quick cell emitted a running event")
	}
}

func TestEngineeringCellParentCancellationIsNotTheLimit(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	release := make(chan struct{})
	defer close(release)
	go func() { time.Sleep(50 * time.Millisecond); cancel() }()
	_, err, _ := runEngineeringCell(ctx, engineeringCellBounds{limit: 5 * time.Second, notice: time.Second}, nil, "run-1", nil,
		func(context.Context) (capsule.GoEvalResult, error) {
			<-release // hang-guard: the cell ignores its context
			return capsule.GoEvalResult{}, nil
		})
	if !errors.Is(err, context.Canceled) || errors.Is(err, errEngineeringCellLimit) {
		t.Fatalf("err = %v, want the parent's cancellation", err)
	}
}

func TestEngineeringCellBoundsFollowTheActivationAndTimeoutMS(t *testing.T) {
	base := engineeringCellBounds{limit: 5 * time.Minute, notice: 2 * time.Minute}
	if got := base.forCell(context.Background(), 0); got.limit != 5*time.Minute {
		t.Fatalf("no deadline, no timeout_ms: limit %s", got.limit)
	}
	// The broker enforces timeout_ms and answers with partial output; the
	// host waits a grace past it so it does not race the broker.
	if got := base.forCell(context.Background(), 1000); got.limit != time.Second+engineeringCellTimeoutGrace {
		t.Fatalf("timeout_ms 1000: limit %s", got.limit)
	}
	if got := base.forCell(context.Background(), int((5 * time.Minute).Milliseconds())); got.limit != 5*time.Minute {
		t.Fatalf("timeout_ms at the limit: limit %s", got.limit)
	}
	if got := base.forCell(context.Background(), int((20 * time.Minute).Milliseconds())); got.limit != 5*time.Minute {
		t.Fatalf("timeout_ms above the limit: limit %s", got.limit)
	}
	// The broker always answers inside the host's limit, by the grace.
	for _, tc := range []struct{ in, want int }{
		{0, 0}, {1000, 1000},
		{int((5 * time.Minute).Milliseconds()), int((5*time.Minute - engineeringCellTimeoutGrace).Milliseconds())},
		{int((20 * time.Minute).Milliseconds()), int((5*time.Minute - engineeringCellTimeoutGrace).Milliseconds())},
		{-5, int((5*time.Minute - engineeringCellTimeoutGrace).Milliseconds())},
	} {
		b := base.forCell(context.Background(), tc.in)
		if got := b.brokerTimeoutMS(tc.in); got != tc.want {
			t.Fatalf("timeout_ms %d: broker %d, want %d", tc.in, got, tc.want)
		}
		if tc.in != 0 && time.Duration(b.brokerTimeoutMS(tc.in))*time.Millisecond > b.limit-engineeringCellTimeoutGrace {
			t.Fatalf("timeout_ms %d: broker bound not inside the host limit %s", tc.in, b.limit)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	got := base.forCell(ctx, 0)
	if got.limit > 3*time.Minute-engineeringCellDeadlineMargin || got.limit < 2*time.Minute {
		t.Fatalf("activation with 3m left: limit %s", got.limit)
	}
}

func TestEngineeringCellLimitWithNoJobSaysNothingWillWake(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	_, err, timedOut := runEngineeringCell(context.Background(), engineeringCellBounds{limit: 100 * time.Millisecond, notice: time.Second}, nil, "run-1", nil,
		func(context.Context) (capsule.GoEvalResult, error) {
			<-release // hang-guard: the cell ignores its context
			return capsule.GoEvalResult{}, nil
		})
	if !timedOut || !errors.Is(err, errEngineeringCellLimit) {
		t.Fatalf("err = %v, timedOut = %v", err, timedOut)
	}
	msg := err.Error()
	if strings.Contains(msg, "wakes you") || !strings.Contains(msg, "nothing will wake you") || !strings.Contains(msg, "choir.Start") {
		t.Fatalf("message %q", msg)
	}
}

func TestEngineeringCellActivationDeadlineIsATimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	release := make(chan struct{})
	defer close(release)
	_, err, timedOut := runEngineeringCell(ctx, engineeringCellBounds{limit: 5 * time.Second, notice: time.Second}, nil, "run-1", nil,
		func(context.Context) (capsule.GoEvalResult, error) {
			<-release // hang-guard: the cell ignores its context
			return capsule.GoEvalResult{}, nil
		})
	if !timedOut || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, timedOut = %v; want the activation deadline as a timeout", err, timedOut)
	}
}

// The crouton cell (2026-11-03) failed with a broker read timeout after 46
// minutes. A silent broker must read as one, naming the running job.
func TestEngineeringCellSilentBrokerNamesTheJob(t *testing.T) {
	jobs := runningBuild(time.Now().Add(10 * time.Minute))
	_, err, timedOut := runEngineeringCell(context.Background(), engineeringCellBounds{limit: 5 * time.Second, notice: time.Second}, jobs, "run-1", nil,
		func(context.Context) (capsule.GoEvalResult, error) {
			return capsule.GoEvalResult{}, fmt.Errorf("%w go_eval within 1m15s: read unix: i/o timeout", capsule.ErrBrokerNoAnswer)
		})
	if !timedOut || err == nil {
		t.Fatalf("err = %v, timedOut = %v", err, timedOut)
	}
	for _, want := range []string{"the capsule broker did not answer", "job-build", "choir.Poll", "end your turn"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("message %q lacks %q", err.Error(), want)
		}
	}
}
