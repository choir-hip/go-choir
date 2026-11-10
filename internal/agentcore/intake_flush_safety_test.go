package agentcore

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// F10: a panic inside labelHeld itself (not the judge) is recovered there.
func TestIntakeFlushPanicInsideLabelHeldIsRecovered(t *testing.T) {
	f := newIntakeFixture(t, "owner-flush-panic-held")
	f.enable(t)
	f.rt.SetCandidateJudge(&fakeIntakeJudge{})
	intakeFlushTestHook = func(stage string) {
		if stage == "labelHeld" {
			panic("labelHeld panic")
		}
	}
	t.Cleanup(func() { intakeFlushTestHook = nil })
	labelled, unlabelled := labelNow(f.rt, context.Background(), f.owner, f.computer, "src", flushTestItems(3, "s"))
	if labelled != 0 || unlabelled != 3 {
		t.Fatalf("F10: labelled=%d unlabelled=%d; want the 3 captures unlabelled and the process alive", labelled, unlabelled)
	}
}

// F10: a panic in the flusher goroutine outside labelHeld is recovered there,
// the layer is not left running, and a later enqueue flushes.
func TestIntakeFlushPanicInTheFlusherGoroutineIsRecovered(t *testing.T) {
	f := newIntakeFixture(t, "owner-flush-panic-flusher")
	f.enable(t)
	f.rt.SetCandidateJudge(&fakeIntakeJudge{})
	var fired atomic.Bool
	intakeFlushTestHook = func(stage string) {
		if stage == "flusher" && fired.CompareAndSwap(false, true) {
			panic("flusher panic")
		}
	}
	t.Cleanup(func() { intakeFlushTestHook = nil })
	t.Cleanup(f.rt.Stop)
	items := flushTestItems(24, "s")
	f.rt.enqueueIntakeLabels(f.owner, f.computer, "src", items[:12])
	waitFlush(t, "the flusher to panic and clear", func() bool {
		l := &f.rt.intakeLabels
		l.mu.Lock()
		defer l.mu.Unlock()
		return fired.Load() && !l.running
	})
	f.rt.enqueueIntakeLabels(f.owner, f.computer, "src", items[12:24])
	waitFlush(t, "a later flush to label", func() bool {
		for i := 12; i < 24; i++ {
			if l, _ := f.s.CaptureLabels(context.Background(), f.owner, f.computer, items[i].CaptureID); len(l) != 1 {
				return false
			}
		}
		return true
	})
}

// F13: a stale fired timer callback does not clobber a newer q.timer.
func TestIntakeFlushStaleTimerDoesNotClobberANewerOne(t *testing.T) {
	f := newIntakeFixture(t, "owner-flush-timer")
	f.enable(t)
	f.rt.SetCandidateJudge(&fakeIntakeJudge{})
	t.Cleanup(f.rt.Stop)
	f.rt.enqueueIntakeLabels(f.owner, f.computer, "src", flushTestItems(1, "s"))
	l := &f.rt.intakeLabels
	l.mu.Lock()
	q := l.queues[eventKey(f.owner, f.computer)]
	stale := q.timer
	newer := time.AfterFunc(time.Hour, func() {})
	q.timer = newer
	f.rt.intakeTimerFiredLocked(q, stale)
	kept := q.timer == newer
	l.mu.Unlock()
	newer.Stop()
	if !kept {
		t.Fatal("F13: a stale callback replaced or cleared a newer timer")
	}
}

// deadlineJudge records how much time each call has left when it arrives.
type deadlineJudge struct {
	fakeIntakeJudge
	mu2  sync.Mutex
	left []time.Duration
	noDL int
}

func (j *deadlineJudge) JudgeScoped(ctx context.Context, scope string, state json.RawMessage, questions map[string]json.RawMessage) (json.RawMessage, error) {
	j.mu2.Lock()
	if dl, ok := ctx.Deadline(); ok {
		j.left = append(j.left, time.Until(dl))
	} else {
		j.noDL++
	}
	j.mu2.Unlock()
	return j.fakeIntakeJudge.JudgeScoped(ctx, scope, state, questions)
}

func (j *deadlineJudge) Judge(ctx context.Context, state json.RawMessage, questions map[string]json.RawMessage) (json.RawMessage, error) {
	return j.JudgeScoped(ctx, "", state, questions)
}

// Many sequential calls share no deadline: every call is sent with a fresh
// per-call timeout and every capture is labelled, however many calls the
// flush makes (the old shared 60 s budget dropped the later ones).
func TestIntakeFlushCallsShareNoDeadline(t *testing.T) {
	rt, _ := testRuntime(t)
	judge := &deadlineJudge{}
	rt.SetCandidateJudge(judge)
	per := storiesPerChunk(t, rt)
	items := flushTestItems(12, "s")
	labels := mustLabels(t, rt.labelPoll(context.Background(), pollLib(rt, 6*per), "owner", "computer", "src", items, intakeBatchCaptures))
	if len(labels) != 12 {
		t.Fatalf("%d labels for 12 captures", len(labels))
	}
	judge.mu2.Lock()
	defer judge.mu2.Unlock()
	if len(judge.left) < 12 || judge.noDL != 0 {
		t.Fatalf("calls with a deadline=%d without=%d; want many sequential calls, each with its own", len(judge.left), judge.noDL)
	}
	for n, d := range judge.left {
		if d < intakeLabelCallTimeout-time.Second {
			t.Fatalf("call %d had %v left; each call starts a fresh %v timeout", n, d, intakeLabelCallTimeout)
		}
	}
}

// Two computers on one runtime are drained by one flusher, never two Jev
// calls at once.
func TestIntakeFlushTwoComputersNeverCallJevConcurrently(t *testing.T) {
	f := newIntakeFixture(t, "owner-flush-two")
	f.enable(t)
	judge := &fakeIntakeJudge{delay: 20 * time.Millisecond}
	f.rt.SetCandidateJudge(judge)
	t.Cleanup(f.rt.Stop)
	f.rt.enqueueIntakeLabels(f.owner, "computer-a", "src", flushTestItems(12, "s"))
	f.rt.enqueueIntakeLabels(f.owner, "computer-b", "src", flushTestItems(12, "s"))
	waitFlush(t, "both flushes", func() bool { return len(judge.labelCalls()) >= 4 })
	if got := judge.maxSeen.Load(); got != 1 {
		t.Fatalf("%d Jev calls in flight at once across computers; want 1", got)
	}
}
