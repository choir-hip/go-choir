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
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Intake flush (docs/design/jev-batched-questions-2026-11-10.md, "Intake
// flush: ways it can fail"). A group commit with no timer: the first capture
// is asked at once, captures arriving during a call go out together in the
// next; the poll waits on a handle that every exit path closes.
// F1 two flushes at once, F2 a capture enqueued meanwhile, F3 a failed call,
// F4 the byte guard, F5 shutdown with captures held, F8 a poll with no
// captures, F10 panics, F15 a handle that never closes.

func flushTestItems(n int, summary string) []sourcegraph.CapturedItem {
	var items []sourcegraph.CapturedItem
	for i := 0; i < n; i++ {
		items = append(items, sourcegraph.CapturedItem{CaptureID: fmt.Sprintf("flush-%02d", i), Title: fmt.Sprintf("Flush item %d", i), Summary: summary, Outlet: "Wire", Language: "en"})
	}
	return items
}

// waitFlush polls cond for up to five seconds.
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

// waitClosed fails unless ch closes within five seconds.
func waitClosed(t *testing.T, what string, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatalf("F15: the wait handle did not close: %s", what)
	}
}

func labelCount(t *testing.T, f *intakeFixture, id string) int {
	t.Helper()
	l, _ := f.s.CaptureLabels(context.Background(), f.owner, f.computer, id)
	return len(l)
}

// gatedJudge blocks its first call until the test releases it, and says when
// that call has arrived. The wait is bounded so a failing test cannot hang.
type gatedJudge struct {
	fakeIntakeJudge
	first   atomic.Bool
	entered chan struct{}
	gate    chan struct{}
}

func newGatedJudge() *gatedJudge {
	return &gatedJudge{entered: make(chan struct{}), gate: make(chan struct{})}
}

func (j *gatedJudge) JudgeScoped(ctx context.Context, scope string, state json.RawMessage, questions map[string]json.RawMessage) (json.RawMessage, error) {
	if j.first.CompareAndSwap(false, true) {
		close(j.entered)
		select {
		case <-j.gate:
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(10 * time.Second):
		}
	}
	return j.fakeIntakeJudge.JudgeScoped(ctx, scope, state, questions)
}

func (j *gatedJudge) Judge(ctx context.Context, state json.RawMessage, questions map[string]json.RawMessage) (json.RawMessage, error) {
	return j.JudgeScoped(ctx, "", state, questions)
}

func TestIntakeFlushFirstCaptureIsAskedAtOnce(t *testing.T) {
	f := newIntakeFixture(t, "owner-flush-first")
	f.enable(t)
	openTestStory(t, f.s, f.owner, f.computer, "Pope has lung surgery")
	judge := &fakeIntakeJudge{}
	f.rt.SetCandidateJudge(judge)
	t.Cleanup(f.rt.Stop)
	items := flushTestItems(1, "s")
	waitClosed(t, "one capture", f.rt.enqueueIntakeLabels(f.owner, f.computer, "src", items))
	if labelCount(t, f, items[0].CaptureID) != 1 {
		t.Fatal("the first capture was not labelled before its handle closed")
	}
	if got := len(judge.labelCalls()); got != 1 {
		t.Fatalf("%d calls for one capture; want 1", got)
	}
}

// Burst: N enqueues made while a call is in flight go out together in the
// next flush, at most 6 captures a call, one call at a time.
func TestIntakeFlushBurstDuringACallBatchesIntoCeilNOverSix(t *testing.T) {
	f := newIntakeFixture(t, "owner-flush-burst")
	f.enable(t)
	openTestStory(t, f.s, f.owner, f.computer, "Pope has lung surgery")
	judge := newGatedJudge()
	f.rt.SetCandidateJudge(judge)
	t.Cleanup(f.rt.Stop)
	all := flushTestItems(15, "s")
	first := f.rt.enqueueIntakeLabels(f.owner, f.computer, "src", all[:1])
	waitClosed(t, "the first call to arrive", judge.entered)
	var handles []<-chan struct{}
	for i := 1; i < 15; i++ { // 14 single-capture polls while the call is held
		handles = append(handles, f.rt.enqueueIntakeLabels(f.owner, f.computer, fmt.Sprintf("src-%d", i), all[i:i+1]))
	}
	// The held call is not recorded until it is released; no second call may
	// have been made meanwhile.
	if got := len(judge.labelCalls()); got != 0 {
		t.Fatalf("%d calls recorded while the first is held; want none", got)
	}
	close(judge.gate)
	waitClosed(t, "the first capture", first)
	for i, h := range handles {
		waitClosed(t, fmt.Sprintf("capture %d", i+1), h)
	}
	if got, want := len(judge.labelCalls()), 1+(14+intakeCallCaptures-1)/intakeCallCaptures; got != want {
		t.Fatalf("%d calls; want %d (the held one, then ceil(14/6) for the burst)", got, want)
	}
	if got := judge.maxSeen.Load(); got != 1 {
		t.Fatalf("F1: %d Jev calls in flight at once; want 1", got)
	}
	for i := 0; i < 15; i++ {
		if labelCount(t, f, all[i].CaptureID) != 1 {
			t.Fatalf("capture %d not labelled", i)
		}
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

// The poll announces after the labels exist, and still announces when the
// label call fails.
func TestIntakeFlushPollAnnouncesAfterLabelsAndEvenWhenTheCallFails(t *testing.T) {
	for _, c := range []struct {
		name       string
		judge      CandidateJudge
		wantLabels int
	}{
		{"labels", &fakeIntakeJudge{}, 3},
		{"failed call", &fakeIntakeJudge{fail: fmt.Errorf("gateway: 429")}, 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := newIntakeFixture(t, "owner-flush-order-"+strings.ReplaceAll(c.name, " ", "-"))
			f.enable(t)
			openTestStory(t, f.s, f.owner, f.computer, "Pope has lung surgery")
			f.rt.SetCandidateJudge(c.judge)
			t.Cleanup(f.rt.Stop)
			var labelsAtCommit atomic.Int32
			commits := atomic.Int32{}
			f.rt.commitCaptureArrival = func(ctx context.Context, req types.CommitInboxRecordsRequest) error {
				commits.Add(1)
				n := 0
				for i := 0; i < 3; i++ {
					if labelCount(t, f, f.captureID(i)) > 0 {
						n++
					}
				}
				labelsAtCommit.Store(int32(n))
				_, err := f.rt.store.CommitInboxRecords(ctx, req)
				return err
			}
			for i := 0; i < 3; i++ {
				*f.items = append(*f.items, labelFeedItem{fmt.Sprintf("Order item %d", i), "Summary"})
			}
			f.poll(t)
			if commits.Load() != 1 {
				t.Fatalf("%d arrivals committed; want 1 (a failed label still announces)", commits.Load())
			}
			if got := int(labelsAtCommit.Load()); got != c.wantLabels {
				t.Fatalf("%d labels existed when the arrival was committed; want %d", got, c.wantLabels)
			}
		})
	}
}

func TestIntakeFlushFailedCallLeavesCapturesUnlabelledAndNothingRetries(t *testing.T) {
	f := newIntakeFixture(t, "owner-flush-fail")
	f.enable(t)
	openTestStory(t, f.s, f.owner, f.computer, "Pope has lung surgery")
	judge := &fakeIntakeJudge{fail: fmt.Errorf("gateway: 429")}
	f.rt.SetCandidateJudge(judge)
	t.Cleanup(f.rt.Stop)
	items := flushTestItems(3, "s")
	waitClosed(t, "a failed flush", f.rt.enqueueIntakeLabels(f.owner, f.computer, "src", items))
	for i := range items {
		if labelCount(t, f, items[i].CaptureID) != 0 {
			t.Fatalf("F3: capture %d labelled by a failing Jev", i)
		}
	}
	if got := judge.callCount(); got != 1 {
		t.Fatalf("F3: %d calls; a failed capture is not retried", got)
	}
}

// Stop: what is held is labelled by the drain, and the handles close.
func TestIntakeFlushStopLabelsWhatIsHeldAndClosesHandles(t *testing.T) {
	f := newIntakeFixture(t, "owner-flush-stop")
	f.enable(t)
	openTestStory(t, f.s, f.owner, f.computer, "Pope has lung surgery")
	judge := newGatedJudge()
	f.rt.SetCandidateJudge(judge)
	items := flushTestItems(2, "s")
	a := f.rt.enqueueIntakeLabels(f.owner, f.computer, "src", items[:1])
	waitClosed(t, "the first call to arrive", judge.entered)
	b := f.rt.enqueueIntakeLabels(f.owner, f.computer, "src", items[1:2]) // held behind the call
	stopped := make(chan struct{})
	go func() { f.rt.Stop(); close(stopped) }()
	waitFlush(t, "stop to begin", func() bool {
		f.rt.intakeLabels.mu.Lock()
		defer f.rt.intakeLabels.mu.Unlock()
		return f.rt.intakeLabels.stopped
	})
	close(judge.gate)
	waitClosed(t, "Stop", stopped)
	waitClosed(t, "the in-flight capture", a)
	waitClosed(t, "the held capture", b)
	for i := range items {
		if labelCount(t, f, items[i].CaptureID) != 1 {
			t.Fatalf("F5: capture %d not labelled by the drain on stop", i)
		}
	}
	before := judge.callCount()
	waitClosed(t, "a refused enqueue", f.rt.enqueueIntakeLabels(f.owner, f.computer, "src", flushTestItems(14, "s")))
	if got := judge.callCount(); got != before {
		t.Fatalf("F5: %d calls after stop began; no enqueue is accepted", got-before)
	}
}

// Stop with a judge slower than the bound returns at the bound, leaves the
// captures unlabelled and closes their handles.
func TestIntakeFlushStopReturnsAtTheBoundWithASlowJudge(t *testing.T) {
	f := newIntakeFixture(t, "owner-flush-stop-slow")
	f.enable(t)
	openTestStory(t, f.s, f.owner, f.computer, "Pope has lung surgery")
	f.rt.telemetryDrainDeadline = 300 * time.Millisecond
	judge := newGatedJudge() // never released: only the cancellation ends the call
	f.rt.SetCandidateJudge(judge)
	items := flushTestItems(2, "s")
	a := f.rt.enqueueIntakeLabels(f.owner, f.computer, "src", items[:1])
	waitClosed(t, "the call to arrive", judge.entered)
	b := f.rt.enqueueIntakeLabels(f.owner, f.computer, "src", items[1:2])
	start := time.Now()
	f.rt.Stop()
	if took := time.Since(start); took > 2*time.Second {
		t.Fatalf("F5: Stop took %v with a 300ms bound", took)
	}
	waitClosed(t, "the in-flight capture", a)
	waitClosed(t, "the abandoned capture", b)
	for i := range items {
		if labelCount(t, f, items[i].CaptureID) != 0 {
			t.Fatalf("F5: capture %d labelled although the judge never answered in time", i)
		}
	}
}

func TestIntakeFlushZeroCapturesCloseTheHandleAtOnce(t *testing.T) {
	f := newIntakeFixture(t, "owner-flush-zero")
	f.enable(t)
	f.rt.SetCandidateJudge(&fakeIntakeJudge{})
	waitClosed(t, "no captures", f.rt.enqueueIntakeLabels(f.owner, f.computer, "src", nil))
	waitClosed(t, "a capture with no id", f.rt.enqueueIntakeLabels(f.owner, f.computer, "src", []sourcegraph.CapturedItem{{Title: "no id"}}))
	l := &f.rt.intakeLabels
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.running || l.anyHeldLocked() {
		t.Fatalf("F8: running=%t held=%t; want nothing started", l.running, l.anyHeldLocked())
	}
}
