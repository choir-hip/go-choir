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

// F10/F15: a panic in the flusher goroutine outside labelHeld is recovered
// there, the layer is not left running, the poll's handle closes, and a later
// enqueue flushes.
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
	items := flushTestItems(2, "s")
	waitClosed(t, "the panicked flush's handle", f.rt.enqueueIntakeLabels(f.owner, f.computer, "src", items[:1]))
	if labelCount(t, f, items[0].CaptureID) != 0 {
		t.Fatal("F10: the panicked flush's capture is labelled")
	}
	waitFlush(t, "the flusher to clear", func() bool {
		l := &f.rt.intakeLabels
		l.mu.Lock()
		defer l.mu.Unlock()
		return !l.running
	})
	waitClosed(t, "a later flush", f.rt.enqueueIntakeLabels(f.owner, f.computer, "src", items[1:2]))
	if labelCount(t, f, items[1].CaptureID) != 1 {
		t.Fatal("F10: the flusher did not flush again after the panic")
	}
}

// F10/F15: a panic inside labelHeld closes the poll's handle too.
func TestIntakeFlushPanicInsideLabelHeldClosesTheHandle(t *testing.T) {
	f := newIntakeFixture(t, "owner-flush-panic-held-handle")
	f.enable(t)
	f.rt.SetCandidateJudge(&fakeIntakeJudge{})
	intakeFlushTestHook = func(stage string) {
		if stage == "labelHeld" {
			panic("labelHeld panic")
		}
	}
	t.Cleanup(func() { intakeFlushTestHook = nil })
	t.Cleanup(f.rt.Stop)
	items := flushTestItems(1, "s")
	waitClosed(t, "the handle after a labelHeld panic", f.rt.enqueueIntakeLabels(f.owner, f.computer, "src", items))
	if labelCount(t, f, items[0].CaptureID) != 0 {
		t.Fatal("F10: a capture is labelled after the panic")
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
