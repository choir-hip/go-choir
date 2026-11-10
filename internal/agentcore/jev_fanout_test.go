package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Failure modes of the Jev fan-out at the call sites (docs/evidence/
// jev-measurement-2026-11-10.md, "Build: failure list", Fan-out):
//   - the event layer's questions are asked ahead of the commits, and a later
//     chunk uses an answer asked without the events an earlier chunk minted
//     (TestEventLayerPrefetchIsUsedOnlyForTheSameQuestion; the serial result
//     is also pinned by TestEventLayerRetryIsIdempotentAndChunksSeeEarlierChunks);
//   - the sub-call, cast and inert judges stay behind a gate of one
//     (TestSubCallJudgeRunsConcurrentlyWithinItsBound);
//   - the scope a call names is dropped on the way to the gateway
//     (TestCallJudgeWithinPassesTheScope).

type slowSubCallJudge struct {
	cur, peak atomic.Int32
	calls     atomic.Int32
}

func (j *slowSubCallJudge) Judge(ctx context.Context, _ json.RawMessage, _ map[string]json.RawMessage) (json.RawMessage, error) {
	n := j.cur.Add(1)
	for p := j.peak.Load(); n > p && !j.peak.CompareAndSwap(p, n); p = j.peak.Load() {
	}
	j.calls.Add(1)
	time.Sleep(20 * time.Millisecond)
	j.cur.Add(-1)
	return json.RawMessage(`{"model":"m","answers":{}}`), nil
}

func TestSubCallJudgeRunsConcurrentlyWithinItsBound(t *testing.T) {
	rt, _ := testRuntime(t)
	judge := &slowSubCallJudge{}
	rt.SetCandidateJudge(judge)
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := rt.callSubCallJudge(context.Background(), json.RawMessage(`{}`), map[string]json.RawMessage{"q": json.RawMessage(`{}`)}); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if judge.calls.Load() != 12 {
		t.Fatalf("calls = %d", judge.calls.Load())
	}
	if peak := judge.peak.Load(); peak < 2 || int(peak) > subCallJudgeConcurrency {
		t.Fatalf("peak in flight = %d; want 2..%d", peak, subCallJudgeConcurrency)
	}
}

func TestEventLayerPrefetchAsksChunksTogetherWhenNothingChanges(t *testing.T) {
	// Every answer is omitted: no chunk mints an event, so the question a
	// later chunk would ask after the earlier commits is the one already
	// asked, and each chunk costs exactly one call.
	judge := &eventJudge{answers: map[string]eventAnswer{}, omit: map[string]bool{}, delay: 30 * time.Millisecond}
	rt, owner, computer := eventRuntime(t, judge)
	var inputs []eventInput
	for i := 0; i < eventChunkSize*3; i++ {
		title := fmt.Sprintf("Story number %d about topic", i)
		judge.omit[title] = true
		inputs = append(inputs, capItem(fmt.Sprintf("p%d", i), title, fmt.Sprintf("https://outlet%d.com/a", i), ""))
	}
	r, err := rt.resolveEvents(context.Background(), owner, computer, inputs)
	if err != nil {
		t.Fatal(err)
	}
	if judge.callCount() != 3 || r.Calls != 3 {
		t.Fatalf("judge calls = %d, report calls = %d; want 3 and 3 (no re-ask)", judge.callCount(), r.Calls)
	}
	judge.mu.Lock()
	peak := judge.maxIn
	judge.mu.Unlock()
	if peak < 2 {
		t.Fatalf("peak in flight = %d; the chunk questions were asked one at a time", peak)
	}
}

func TestEventLayerPrefetchIsUsedOnlyForTheSameQuestion(t *testing.T) {
	// The first chunk mints events, so the second chunk's question differs
	// (it now offers them): the prefetched answer is dropped and asked again,
	// and the cross-chunk join still happens, as in the serial order.
	judge := &eventJudge{answers: map[string]eventAnswer{}}
	rt, owner, computer := eventRuntime(t, judge)
	var inputs []eventInput
	for i := 0; i < eventChunkSize+3; i++ {
		inputs = append(inputs, capItem(fmt.Sprintf("y%d", i), fmt.Sprintf("Story number %d about topic", i), fmt.Sprintf("https://outlet%d.com/a", i), ""))
	}
	judge.answers[inputs[len(inputs)-1].Item.Title] = eventAnswer{target: inputs[0].Item.Title}
	r, err := rt.resolveEvents(context.Background(), owner, computer, inputs)
	if err != nil {
		t.Fatal(err)
	}
	if r.Calls != 2 || len(r.Corroborated) != 1 {
		t.Fatalf("report = %+v; want 2 chunks and the cross-chunk join", r)
	}
	if judge.callCount() != 3 {
		t.Fatalf("judge calls = %d; want 2 prefetched plus 1 re-ask", judge.callCount())
	}
}

type scopeJudge struct {
	mu     sync.Mutex
	scopes []string
}

func (j *scopeJudge) Judge(ctx context.Context, s json.RawMessage, q map[string]json.RawMessage) (json.RawMessage, error) {
	return j.JudgeScoped(ctx, "", s, q)
}

func (j *scopeJudge) JudgeScoped(_ context.Context, scope string, _ json.RawMessage, _ map[string]json.RawMessage) (json.RawMessage, error) {
	j.mu.Lock()
	j.scopes = append(j.scopes, scope)
	j.mu.Unlock()
	return json.RawMessage(`{"answers":{}}`), nil
}

func TestCallJudgeWithinPassesTheScope(t *testing.T) {
	judge := &scopeJudge{}
	for _, scope := range []string{judgmentScopeInteractive, "", judgmentScopeIntakeLabels} {
		if _, err := callJudgeWithin(context.Background(), judge, scope, time.Second, json.RawMessage(`{}`), nil); err != nil {
			t.Fatal(err)
		}
	}
	// An unscoped call goes through Judge, which asks with the empty scope.
	want := []string{judgmentScopeInteractive, "", judgmentScopeIntakeLabels}
	for i, got := range judge.scopes {
		if got != want[i] {
			t.Fatalf("scopes = %v, want %v", judge.scopes, want)
		}
	}
}
