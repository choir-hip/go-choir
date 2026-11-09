package toolregistry

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// An every-model-out-of-quota incident is bounded by time, not by rounds
// (docs/problems/paper-quota-incidents-2026-11-09.md: runs failed "after 8
// retry rounds over 5m28s" while a hold ended 14 s later). Ways it could
// fail, written down before the change:
//   - holds that reset after 6 minutes fail the run at 5m28s;
//   - short waits clamped to a near reset each spend a round, so a chain
//     that keeps recovering and re-exhausting dies after eight of them;
//   - a retry goes out before the earliest known reset;
//   - an incident that never clears runs past the schedule's total, or ends
//     with a different error than before;
//   - every run of the computer retries at the same instant;
//   - the ledger's holds are invisible.

type fakeQuotaClock struct {
	mu     sync.Mutex
	now    time.Time
	sleeps []time.Duration
}

func (c *fakeQuotaClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeQuotaClock) Sleep(_ context.Context, d time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sleeps = append(c.sleeps, d)
	c.now = c.now.Add(d)
	return nil
}

func useFakeQuotaClock(t *testing.T, jitter time.Duration) *fakeQuotaClock {
	t.Helper()
	c := &fakeQuotaClock{now: time.Date(2026, 11, 9, 16, 0, 0, 0, time.UTC)}
	now, sleep, j := providerQuotaNow, providerQuotaSleep, providerQuotaJitterMax
	providerQuotaNow, providerQuotaSleep, providerQuotaJitterMax = c.Now, c.Sleep, jitter
	t.Cleanup(func() {
		providerQuotaNow, providerQuotaSleep, providerQuotaJitterMax = now, sleep, j
	})
	return c
}

// stampedProvider records the clock at each call.
type stampedProvider struct {
	*poolProvider
	clock *fakeQuotaClock
	mu    sync.Mutex
	calls []time.Time
}

func (s *stampedProvider) CallWithTools(ctx context.Context, req provideriface.ToolLoopRequest) (*provideriface.ToolLoopResponse, error) {
	s.mu.Lock()
	s.calls = append(s.calls, s.clock.Now())
	s.mu.Unlock()
	return s.poolProvider.CallWithTools(ctx, req)
}

func runStamped(p provideriface.ToolLoopProvider) (string, error) {
	opts := []ToolLoopOption{WithToolLoopLLMConfig(synthMember), WithToolLoopConversationID("run-1"),
		WithContinuationChain(func(context.Context) []provideriface.LLMSelection { return []provideriface.LLMSelection{lunaMember} })}
	text, _, err := RunToolLoop(context.Background(), p, nil, []json.RawMessage{json.RawMessage(`{"role":"user","content":"hello"}`)}, "system", 0, func(types.EventKind, string, json.RawMessage) {}, nil, opts...)
	return text, err
}

func TestIncidentWhoseHoldsResetAfterSixMinutesSucceedsAndRetriesAtTheReset(t *testing.T) {
	clock := useFakeQuotaClock(t, 5*time.Second)
	t0 := clock.Now()
	reset := t0.Add(6 * time.Minute)
	pp := &poolProvider{errs: map[string][]error{
		"synthetic": {quotaErrFor("synthetic", reset), nil},
		"chatgpt":   {quotaErrFor("chatgpt", reset.Add(30*time.Second)), nil},
	}}
	p := &stampedProvider{poolProvider: pp, clock: clock}
	text, err := runStamped(p)
	if err != nil || !strings.HasPrefix(text, "done by") {
		t.Fatalf("run = %q, %v; want success once the holds reset at 6 min (it failed at 5m28s)", text, err)
	}
	if len(p.calls) != 3 {
		t.Fatalf("calls at %v, want two exhausted answers and one retry", p.calls)
	}
	if retry := p.calls[2]; retry.Before(reset) {
		t.Fatalf("retry at +%s, before the earliest reset at +6m", retry.Sub(t0))
	}
	if len(clock.sleeps) != 1 || clock.sleeps[0] < 6*time.Minute {
		t.Fatalf("sleeps = %v, want one wait of at least 6 min", clock.sleeps)
	}
}

func TestShortClampedWaitsDoNotExhaustTheIncident(t *testing.T) {
	clock := useFakeQuotaClock(t, 0)
	t0 := clock.Now()
	// Twelve rounds (the old bound was eight), each hold 10 s away.
	var syn, gpt []error
	for k := 1; k <= 12; k++ {
		at := t0.Add(time.Duration(k) * 10 * time.Second)
		syn = append(syn, quotaErrFor("synthetic", at))
		gpt = append(gpt, quotaErrFor("chatgpt", at))
	}
	pp := &poolProvider{errs: map[string][]error{"synthetic": append(syn, nil), "chatgpt": append(gpt, nil)}}
	p := &stampedProvider{poolProvider: pp, clock: clock}
	text, err := runStamped(p)
	if err != nil || !strings.HasPrefix(text, "done by") {
		t.Fatalf("run = %q, %v; want success after twelve short clamped rounds", text, err)
	}
	if len(clock.sleeps) < 12 {
		t.Fatalf("sleeps = %v, want at least twelve rounds", clock.sleeps)
	}
}

func TestIncidentLongerThanTheScheduleFailsWithTheExistingError(t *testing.T) {
	clock := useFakeQuotaClock(t, 0)
	t0 := clock.Now()
	far := t0.Add(24 * time.Hour)
	pp := &poolProvider{errs: map[string][]error{"synthetic": {quotaErrFor("synthetic", far)}, "chatgpt": {quotaErrFor("chatgpt", far)}}}
	p := &stampedProvider{poolProvider: pp, clock: clock}
	_, err := runStamped(p)
	if err == nil || !strings.Contains(err.Error(), "every model is out of quota after") || !strings.Contains(err.Error(), "retry rounds over 18m0s") {
		t.Fatalf("err = %v, want the bounded quota error over the schedule's 18m", err)
	}
	if elapsed := clock.Now().Sub(t0); elapsed != providerQuotaIncidentBound() {
		t.Fatalf("incident lasted %s, want exactly the bound %s", elapsed, providerQuotaIncidentBound())
	}
}

func TestQuotaRetryJitterIsDeterministicSmallAndSpreadsRuns(t *testing.T) {
	a1, a2 := quotaRetryJitter("run-a", "c", 1), quotaRetryJitter("run-a", "c", 1)
	if a1 != a2 {
		t.Fatalf("jitter not deterministic: %s vs %s", a1, a2)
	}
	seen := map[time.Duration]bool{}
	for _, id := range []string{"run-a", "run-b", "run-c", "run-d", "run-e", "run-f"} {
		j := quotaRetryJitter(id, "c", 1)
		if j < 0 || j >= providerQuotaJitterMax {
			t.Fatalf("jitter %s out of [0,%s)", j, providerQuotaJitterMax)
		}
		seen[j] = true
	}
	if len(seen) < 4 {
		t.Fatalf("six runs produced %d distinct jitters", len(seen))
	}
}

func TestQuotaLedgerHoldsNameMemberUntilAndSource(t *testing.T) {
	now := time.Date(2026, 11, 9, 16, 0, 0, 0, time.UTC)
	l := NewQuotaLedger()
	l.MarkSourced(synthMember, now.Add(time.Minute), false)
	l.MarkSourced(lunaMember, now.Add(2*time.Minute), true)
	l.Mark(routerMember, now.Add(-time.Minute))
	l.Demote(zaiMember, now)
	holds := l.Holds(now)
	got := map[string]string{}
	for _, h := range holds {
		got[h.Member] = h.Source
	}
	want := map[string]string{servingSelectionID(synthMember): QuotaHoldProviderReset, servingSelectionID(lunaMember): QuotaHoldEstimated, servingSelectionID(zaiMember): QuotaHoldFailure}
	if len(got) != len(want) {
		t.Fatalf("holds = %+v, want %v (the expired hold omitted)", holds, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("holds = %+v, want %v", holds, want)
		}
	}
}
