package agentcore

// A park (an engineer waiting for management's verdict) suspends the
// activation's progress deadline. Ways it could fail, written down for
// these tests:
//
//  D4 after the park the deadline is not charged again (a park is a free
//     extension of the run's own work);
//  D5 the durable deadline continuation closes a live parked run anyway;
//  D6 a provider quota touches the deadline (it must not: a quota switches
//     models, it never parks).

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
)

type quotaThenOK struct {
	provideriface.Provider
	mu    sync.Mutex
	calls map[string]int
}

// CallWithTools answers every synthetic call with a quota error and every
// other provider with a reply.
func (p *quotaThenOK) CallWithTools(_ context.Context, req provideriface.ToolLoopRequest) (*provideriface.ToolLoopResponse, error) {
	p.mu.Lock()
	if p.calls == nil {
		p.calls = map[string]int{}
	}
	p.calls[req.Provider]++
	p.mu.Unlock()
	if req.Provider == "synthetic" {
		return nil, &provideriface.ProviderQuotaError{Provider: "synthetic", Model: req.Model, Reason: "provider quota exhausted (synthetic)",
			Message: "synthetic: quota exhausted", RetryAt: time.Now().Add(time.Hour)}
	}
	return &provideriface.ToolLoopResponse{StopReason: "end_turn", Text: "done on " + req.Provider + "/" + req.Model}, nil
}

func withHardWall(t *testing.T, d time.Duration) {
	t.Helper()
	prev := activationHardWall
	activationHardWall = d
	t.Cleanup(func() { activationHardWall = prev })
}

// 2026-11-04: concurrent research children that met a quota each parked on
// the parent's one activation deadline (they inherit its context), and their
// suspend/resume pairs corrupted it. A quota no longer parks anything: the
// children switch models and the parent's deadline is not touched. Failure
// modes: a child's quota suspends or moves the parent's deadline; a child
// waits instead of switching; the children share one answer.
func TestConcurrentChildrenMeetingAQuotaDoNotTouchTheParentDeadline(t *testing.T) {
	withHardWall(t, time.Hour)
	rt := &Runtime{}
	ctx, cancel := newActivationDeadline(context.Background(), time.Minute)
	defer cancel()
	before, _ := ctx.Deadline()
	p := &quotaThenOK{}
	var wg sync.WaitGroup
	results := make([]string, 8)
	errs := make([]error, 8)
	started := time.Now()
	for i := range results {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rec := &types.RunRecord{RunID: "child-" + string(rune('a'+i)), OwnerID: "o", ComputerID: "c", AgentID: "research:o"}
			results[i], _, errs[i] = toolregistry.RunToolLoop(ctx, p, nil, []json.RawMessage{json.RawMessage(`{"role":"user","content":"hi"}`)}, "system", 0, func(types.EventKind, string, json.RawMessage) {}, nil,
				toolregistry.WithToolLoopLLMConfig(provideriface.LLMSelection{Provider: "synthetic", Model: "m"}),
				toolregistry.WithContinuationChain(func(context.Context) []provideriface.LLMSelection {
					return []provideriface.LLMSelection{{Provider: "chatgpt", Model: "gpt-6-luna"}}
				}),
				toolregistry.WithQuotaLedger(&rt.quotaLedger),
				toolregistry.WithProviderIncident(rt.providerQuotaIncidentHook(rec)))
		}()
	}
	wg.Wait()
	for i := range results {
		if errs[i] != nil || results[i] != "done on chatgpt/gpt-6-luna" {
			t.Fatalf("child %d = %q, %v; want a switch to the chain", i, results[i], errs[i])
		}
	}
	if time.Since(started) > 5*time.Second {
		t.Fatalf("the children took %s, want no wait", time.Since(started))
	}
	if after, _ := ctx.Deadline(); !after.Equal(before) {
		t.Fatalf("the parent's deadline moved: %s -> %s", before, after)
	}
	if _, over, moved := activationDeadlineFrom(ctx).endsAt(); over || moved {
		t.Fatalf("the parent's deadline was suspended or moved (over %v, moved %v)", over, moved)
	}
	if views := rt.providerWaitViews("o", "c"); len(views) != 0 {
		t.Fatalf("a switch left an incident: %+v", views)
	}
}

func TestLiveParkedActivationIsNotClosedByTheDurableDeadline(t *testing.T) {
	withHardWall(t, time.Hour)
	ctx, cancel := newActivationDeadline(context.Background(), 50*time.Millisecond)
	defer cancel()
	ctx.suspend(types.NewProviderQuotaWait("run", "synthetic", "m", time.Now(), time.Now().Add(time.Minute), false, time.Minute, ""))
	time.Sleep(100 * time.Millisecond)
	at, over, moved := ctx.endsAt()
	if over || !moved || !time.Now().Before(at) {
		t.Fatalf("D5: a parked activation reads as over (%v, ends %s)", over, at)
	}
	ctx.resume()
	if d, _ := ctx.Deadline(); !d.After(time.Now()) {
		t.Fatalf("D4: the deadline after a 100 ms park is %s, in the past", d)
	}
	cancel()
	if !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatalf("cancel: %v", ctx.Err())
	}
}

func TestGrantAtLeastLeavesFixTimeBoundedByTheWall(t *testing.T) {
	withHardWall(t, 2*time.Second)
	ctx, cancel := newActivationDeadline(context.Background(), 100*time.Millisecond)
	defer cancel()
	a := activationDeadlineFrom(ctx)
	a.grantAtLeast(time.Second)
	if d, _ := ctx.Deadline(); time.Until(d) < 900*time.Millisecond {
		t.Fatalf("after a 1 s grant the deadline is %s away", time.Until(d))
	}
	if _, _, moved := a.endsAt(); !moved {
		t.Fatal("a granted deadline is not marked moved, so the durable continuation would close the run at the old one")
	}
	a.grantAtLeast(time.Hour)
	if d, _ := ctx.Deadline(); d.After(a.hardWall) {
		t.Fatalf("a grant passed the hard wall: %s > %s", d, a.hardWall)
	}
	before, _ := ctx.Deadline()
	a.grantAtLeast(time.Millisecond)
	if after, _ := ctx.Deadline(); !after.Equal(before) {
		t.Fatal("a smaller grant pulled the deadline in")
	}
}

func TestReviewWaitIsNotChargedToTheBuild(t *testing.T) {
	withHardWall(t, time.Hour)
	ctx, cancel := newActivationDeadline(context.Background(), 200*time.Millisecond)
	defer cancel()
	a := activationDeadlineFrom(ctx)
	a.suspend(&types.WaitState{Kind: types.WaitKindVerifyOwed, Class: types.WaitClassPolicy, Resource: "management's verdict", Since: time.Now()})
	time.Sleep(400 * time.Millisecond)
	if ctx.Err() != nil {
		t.Fatalf("the activation ended while it waited on management's verdict: %v", ctx.Err())
	}
	a.resume()
	a.grantAtLeast(300 * time.Millisecond)
	if d, _ := ctx.Deadline(); time.Until(d) < 250*time.Millisecond {
		t.Fatalf("after a rejection the engineer has %s", time.Until(d))
	}
}
