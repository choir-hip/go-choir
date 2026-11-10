package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/sourcegraph"
)

// Intake flush (docs/design/jev-batched-questions-2026-11-10.md, "Intake
// flush: ways it can fail"). F1 two flushes at once, F2 a capture enqueued
// meanwhile, F3 a failed call, F4 the byte guard, F5 shutdown with captures
// held, F8 a poll with no captures.

func flushTestItems(n int, summary string) []sourcegraph.CapturedItem {
	var items []sourcegraph.CapturedItem
	for i := 0; i < n; i++ {
		items = append(items, sourcegraph.CapturedItem{CaptureID: fmt.Sprintf("flush-%02d", i), Title: fmt.Sprintf("Flush item %d", i), Summary: summary, Outlet: "Wire", Language: "en"})
	}
	return items
}

// waitFor polls cond for up to five seconds.
func waitFlush(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestIntakeFlushTenSingleCapturePollsMakeOneFlush(t *testing.T) {
	f := newIntakeFixture(t, "owner-flush-ten")
	f.enable(t)
	t.Cleanup(f.rt.Stop)
	openTestStory(t, f.s, f.owner, f.computer, "Pope has lung surgery")
	judge := &fakeIntakeJudge{}
	f.rt.SetCandidateJudge(judge)
	items := flushTestItems(10, "s")
	for i := 0; i < 10; i++ {
		f.rt.enqueueIntakeLabels(f.owner, f.computer, fmt.Sprintf("src-%d", i), items[i:i+1])
	}
	if got := judge.callCount(); got != 0 {
		t.Fatalf("F2: %d calls before the flush is due; captures must be held", got)
	}
	l := &f.rt.intakeLabels
	l.mu.Lock()
	q := l.queues[eventKey(f.owner, f.computer)]
	held, armed := len(q.items), q.timer != nil
	l.mu.Unlock()
	if held != 10 || !armed {
		t.Fatalf("held=%d timer armed=%t; want 10 held under one timer", held, armed)
	}
	f.rt.drainIntakeLabelsNow(f.owner, f.computer)
	if got := len(judge.labelCalls()); got != 2 {
		t.Fatalf("%d label calls for one flush of 10 captures against one story; want 2 (6 + 4)", got)
	}
	if judge.maxSeen.Load() != 1 {
		t.Fatalf("max in flight = %d; want 1", judge.maxSeen.Load())
	}
	for i := 0; i < 10; i++ {
		labels, _ := f.s.CaptureLabels(context.Background(), f.owner, f.computer, fmt.Sprintf("flush-%02d", i))
		if len(labels) != 1 || labels[0].SourceID != fmt.Sprintf("src-%d", i) {
			t.Fatalf("capture %d labels = %+v; want one, from its own source", i, labels)
		}
	}
}

func TestIntakeFlushRunsAtTwelveHeldAndNeverTwoAtOnce(t *testing.T) {
	f := newIntakeFixture(t, "owner-flush-twelve")
	f.enable(t)
	judge := &fakeIntakeJudge{delay: 30 * time.Millisecond}
	f.rt.SetCandidateJudge(judge)
	t.Cleanup(f.rt.Stop)
	items := flushTestItems(30, "s")
	f.rt.enqueueIntakeLabels(f.owner, f.computer, "src", items[:12])
	// F2: more arrive while the flush runs; they are not a second stream.
	waitFlush(t, "the first flush's call", func() bool { return judge.callCount() >= 1 })
	f.rt.enqueueIntakeLabels(f.owner, f.computer, "src", items[12:24])
	waitFlush(t, "all 24 labelled", func() bool {
		for i := 0; i < 24; i++ {
			if l, _ := f.s.CaptureLabels(context.Background(), f.owner, f.computer, items[i].CaptureID); len(l) != 1 {
				return false
			}
		}
		return true
	})
	if got := judge.maxSeen.Load(); got != 1 {
		t.Fatalf("F1: %d Jev calls in flight at once; want 1", got)
	}
	if got := len(judge.labelCalls()); got != 4 {
		t.Fatalf("%d calls for two full flushes of no-story captures; want 4 (6 captures a call)", got)
	}
}

func TestIntakeFlushPacksToTheByteGuardNotACount(t *testing.T) {
	rt, _ := testRuntime(t)
	judge := &fakeIntakeJudge{}
	rt.SetCandidateJudge(judge)
	long := strings.Repeat("word ", 400)
	items := flushTestItems(12, long)
	labels := mustLabels(t, rt.labelPoll(context.Background(), pollLib(rt, 20), "owner", "computer", "src", items, intakeBatchCaptures))
	if len(labels) != 12 {
		t.Fatalf("F4: %d labels for 12 captures", len(labels))
	}
	calls := judge.labelCalls()
	total := 0
	for _, c := range calls {
		if c.bytes > intakeRequestMaxBytes {
			t.Errorf("F4: a call is %d bytes; the guard is %d", c.bytes, intakeRequestMaxBytes)
		}
		total += len(c.questions)
	}
	if len(calls) < 1 || total != 24 {
		t.Fatalf("calls=%d questions=%d; want 24 questions (two per capture, none split or dropped)", len(calls), total)
	}
	most := 0
	for _, c := range calls {
		most = max(most, len(c.questions))
	}
	if most > 2*intakeCallCaptures || most <= 8 {
		t.Fatalf("the largest call has %d questions; want more than 8 and at most %d", most, 2*intakeCallCaptures)
	}
	if judge.maxSeen.Load() != 1 {
		t.Fatalf("F1: max in flight = %d", judge.maxSeen.Load())
	}
}

func TestIntakeFlushFailedCallLeavesCapturesUnlabelledAndNothingRetries(t *testing.T) {
	f := newIntakeFixture(t, "owner-flush-fail")
	f.enable(t)
	judge := &fakeIntakeJudge{fail: fmt.Errorf("gateway: 429")}
	f.rt.SetCandidateJudge(judge)
	f.rt.enqueueIntakeLabels(f.owner, f.computer, "src", flushTestItems(3, "s"))
	f.rt.drainIntakeLabelsNow(f.owner, f.computer)
	f.rt.drainIntakeLabelsNow(f.owner, f.computer)
	for i := 0; i < 3; i++ {
		if l, _ := f.s.CaptureLabels(context.Background(), f.owner, f.computer, fmt.Sprintf("flush-%02d", i)); len(l) != 0 {
			t.Fatalf("F3: capture %d labelled by a failing Jev", i)
		}
	}
	if got := judge.callCount(); got != 1 {
		t.Fatalf("F3: %d calls; a failed capture is not retried", got)
	}
}

func TestIntakeFlushStopLabelsWhatIsHeldAndRefusesNew(t *testing.T) {
	f := newIntakeFixture(t, "owner-flush-stop")
	f.enable(t)
	judge := &fakeIntakeJudge{}
	f.rt.SetCandidateJudge(judge)
	f.rt.enqueueIntakeLabels(f.owner, f.computer, "src", flushTestItems(2, "s"))
	f.rt.Stop()
	for i := 0; i < 2; i++ {
		if l, _ := f.s.CaptureLabels(context.Background(), f.owner, f.computer, fmt.Sprintf("flush-%02d", i)); len(l) != 1 {
			t.Fatalf("F11: held capture %d not labelled by the drain on stop", i)
		}
	}
	before := judge.callCount()
	f.rt.enqueueIntakeLabels(f.owner, f.computer, "src", flushTestItems(14, "s"))
	f.rt.Stop() // idempotent
	if got := judge.callCount(); got != before {
		t.Fatalf("F11: %d calls after stop began; no enqueue is accepted", got-before)
	}
}

func TestIntakeFlushStopReturnsAtTheBoundWithASlowJudge(t *testing.T) {
	f := newIntakeFixture(t, "owner-flush-stop-slow")
	f.enable(t)
	f.rt.telemetryDrainDeadline = 300 * time.Millisecond
	judge := &fakeIntakeJudge{delay: 3 * time.Second}
	f.rt.SetCandidateJudge(judge)
	f.rt.enqueueIntakeLabels(f.owner, f.computer, "src", flushTestItems(2, "s"))
	start := time.Now()
	f.rt.Stop()
	if took := time.Since(start); took > 2*time.Second {
		t.Fatalf("F11: Stop took %v with a 300ms bound", took)
	}
	if l, _ := f.s.CaptureLabels(context.Background(), f.owner, f.computer, "flush-00"); len(l) != 0 {
		t.Fatalf("F11: a capture the judge did not answer in time is labelled: %+v", l)
	}
}

type panicOnceJudge struct {
	fakeIntakeJudge
	panicked atomic.Bool
}

func (j *panicOnceJudge) JudgeScoped(ctx context.Context, scope string, state json.RawMessage, questions map[string]json.RawMessage) (json.RawMessage, error) {
	if j.panicked.CompareAndSwap(false, true) {
		panic("judge panic")
	}
	return j.fakeIntakeJudge.JudgeScoped(ctx, scope, state, questions)
}

func (j *panicOnceJudge) Judge(ctx context.Context, state json.RawMessage, questions map[string]json.RawMessage) (json.RawMessage, error) {
	return j.JudgeScoped(ctx, "", state, questions)
}

func TestIntakeFlushSurvivesAPanicAndFlushesAgain(t *testing.T) {
	f := newIntakeFixture(t, "owner-flush-panic")
	f.enable(t)
	judge := &panicOnceJudge{}
	f.rt.SetCandidateJudge(judge)
	t.Cleanup(f.rt.Stop)
	items := flushTestItems(24, "s")
	f.rt.enqueueIntakeLabels(f.owner, f.computer, "src", items[:12])
	waitFlush(t, "the panicking flush to end", func() bool {
		l := &f.rt.intakeLabels
		l.mu.Lock()
		defer l.mu.Unlock()
		q := l.queues[eventKey(f.owner, f.computer)]
		return judge.panicked.Load() && q != nil && !q.inFlight
	})
	for i := 0; i < intakeCallCaptures; i++ {
		if l, _ := f.s.CaptureLabels(context.Background(), f.owner, f.computer, items[i].CaptureID); len(l) != 0 {
			t.Fatalf("F10: capture %d of the panicked call is labelled", i)
		}
	}
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

func TestIntakeFlushZeroCapturesArmsNothing(t *testing.T) {
	f := newIntakeFixture(t, "owner-flush-zero")
	f.enable(t)
	f.rt.SetCandidateJudge(&fakeIntakeJudge{})
	f.rt.enqueueIntakeLabels(f.owner, f.computer, "src", nil)
	f.rt.enqueueIntakeLabels(f.owner, f.computer, "src", []sourcegraph.CapturedItem{{Title: "no id"}})
	l := &f.rt.intakeLabels
	l.mu.Lock()
	defer l.mu.Unlock()
	if q := l.queues[eventKey(f.owner, f.computer)]; q != nil && (q.timer != nil || len(q.items) != 0) {
		t.Fatalf("F8: queue = %+v; want nothing held and no timer", q)
	}
}
