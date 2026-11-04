package toolregistry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Provider quota means switch, never pause
// (docs/problems/provider-quota-kills-desk-loops-2026-11-04.md). Ways it
// could fail, written down before the code:
//   - a pinned model, or a pool's last member, that meets a quota stops or
//     sleeps instead of switching;
//   - the switch waits (any sleep before the next call);
//   - the switch leaves no run.retry model_switch event naming from and to;
//   - a model known to be out of quota is called again before its reset,
//     or never again after it;
//   - the chain walks back to a model the run just left in the same round;
//   - every model out of quota parks silently, forever, or fails at once
//     with no incident;
//   - the incident is never cleared, so desks read a working loop as stuck;
//   - a model failure (not a quota) switches or raises an incident.

var (
	routerMember = provideriface.LLMSelection{Provider: "openrouter", Model: "stealth/crouton-alpha"}
	zaiMember    = provideriface.LLMSelection{Provider: "zai", Model: "glm-5.2"}
)

func shortenQuota(t *testing.T) {
	t.Helper()
	hold, backoff, transient := providerQuotaUnknownHold, providerQuotaIncidentBackoff, transientProviderRetryDelay
	providerQuotaUnknownHold = 5 * time.Millisecond
	providerQuotaIncidentBackoff = []time.Duration{5 * time.Millisecond, 5 * time.Millisecond, 5 * time.Millisecond}
	transientProviderRetryDelay = time.Millisecond
	t.Cleanup(func() {
		providerQuotaUnknownHold, providerQuotaIncidentBackoff, transientProviderRetryDelay = hold, backoff, transient
	})
}

type quotaRun struct {
	mu        sync.Mutex
	incidents []*types.WaitState
	ends      []string
	phases    map[string]int
	switches  []map[string]any
}

func (r *quotaRun) record(_ context.Context, w *types.WaitState, ended string) {
	r.mu.Lock()
	r.incidents = append(r.incidents, w)
	if w == nil {
		r.ends = append(r.ends, ended)
	}
	r.mu.Unlock()
}

type quotaSetup struct {
	chain  []provideriface.LLMSelection
	ledger *QuotaLedger
}

func runQuota(ctx context.Context, p *poolProvider, r *quotaRun, s quotaSetup, primary provideriface.LLMSelection, fallbacks ...provideriface.LLMSelection) (string, error) {
	r.phases = map[string]int{}
	emit := func(_ types.EventKind, name string, payload json.RawMessage) {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.phases[name]++
		if name == "model_switch" {
			var m map[string]any
			_ = json.Unmarshal(payload, &m)
			r.switches = append(r.switches, m)
		}
	}
	opts := []ToolLoopOption{WithToolLoopLLMConfig(primary), WithToolLoopFallbackLLMConfigs(fallbacks), WithToolLoopConversationID("run-1"),
		WithProviderIncident(r.record)}
	if s.chain != nil {
		chain := s.chain
		opts = append(opts, WithContinuationChain(func(context.Context) []provideriface.LLMSelection { return chain }))
	}
	if s.ledger != nil {
		opts = append(opts, WithQuotaLedger(s.ledger))
	}
	text, _, err := RunToolLoop(ctx, p, nil, []json.RawMessage{json.RawMessage(`{"role":"user","content":"hello"}`)}, "system", 0, emit, nil, opts...)
	return text, err
}

func quotaErrFor(providerName string, retryAt time.Time) error {
	return fmt.Errorf("gateway call failed: %w", &provideriface.ProviderQuotaError{Provider: providerName, Model: "m",
		Reason: "provider quota exhausted (" + providerName + ")", Message: providerName + ": quota exhausted: status 429 Too Many Requests (sanitized)", RetryAt: retryAt})
}

func TestPinnedModelSwitchesOnQuotaWithoutWaiting(t *testing.T) {
	shortenQuota(t)
	providerQuotaUnknownHold = time.Hour // a sleep of any kind would show
	providerQuotaIncidentBackoff = []time.Duration{time.Hour}
	p := &poolProvider{errs: map[string][]error{"synthetic": {quotaErrFor("synthetic", time.Now().Add(time.Hour))}}}
	r := &quotaRun{}
	started := time.Now()
	text, err := runQuota(context.Background(), p, r, quotaSetup{chain: []provideriface.LLMSelection{lunaMember}}, synthMember)
	if err != nil || text != "done by chatgpt" {
		t.Fatalf("run = %q, %v; want the pinned role to switch to the chain and finish", text, err)
	}
	if time.Since(started) > 2*time.Second {
		t.Fatalf("the switch took %s, want no wait", time.Since(started))
	}
	if len(r.switches) != 1 || r.switches[0]["reason"] != "provider_quota" || r.switches[0]["from_provider"] != "synthetic" || r.switches[0]["to_provider"] != "chatgpt" || r.switches[0]["to_model"] != "gpt-6-luna" {
		t.Fatalf("switch events = %v, want one naming from, to and why", r.switches)
	}
	if len(r.incidents) != 0 || r.phases["provider_quota_incident"] != 0 {
		t.Fatalf("a switch is not an incident: %v %v", r.incidents, r.phases)
	}
}

func TestPoolLastMemberSwitchesToTheContinuationChain(t *testing.T) {
	shortenQuota(t)
	// luna's pool: synth, then luna. synth fails first, luna (the last
	// member) fails too, and the chain continues on OpenRouter.
	p := &poolProvider{errs: map[string][]error{
		"synthetic": {quotaErrFor("synthetic", time.Now().Add(time.Hour))},
		"chatgpt":   {quotaErrFor("chatgpt", time.Now().Add(time.Hour))},
	}}
	r := &quotaRun{}
	text, err := runQuota(context.Background(), p, r, quotaSetup{chain: []provideriface.LLMSelection{lunaMember, routerMember, zaiMember}}, synthMember, lunaMember)
	if err != nil || text != "done by openrouter" {
		t.Fatalf("run = %q, %v; want the chain's first usable model", text, err)
	}
	if len(r.switches) != 2 || r.switches[1]["to_provider"] != "openrouter" {
		t.Fatalf("switches = %v, want pool member then chain", r.switches)
	}
	if p.count("synthetic") != 1 || p.count("chatgpt") != 1 || p.count("openrouter") != 1 || p.count("zai") != 0 {
		t.Fatalf("calls synth %d luna %d router %d zai %d; each model once, in order", p.count("synthetic"), p.count("chatgpt"), p.count("openrouter"), p.count("zai"))
	}
}

func TestExhaustedModelIsSkippedUntilItsReset(t *testing.T) {
	shortenQuota(t)
	ledger := NewQuotaLedger()
	reset := time.Now().Add(150 * time.Millisecond)
	p := &poolProvider{errs: map[string][]error{"synthetic": {quotaErrFor("synthetic", reset)}}}
	r := &quotaRun{}
	setup := quotaSetup{chain: []provideriface.LLMSelection{lunaMember}, ledger: ledger}
	if text, err := runQuota(context.Background(), p, r, setup, synthMember); err != nil || text != "done by chatgpt" {
		t.Fatalf("first run = %q, %v", text, err)
	}
	// A second run on the same computer does not ask the exhausted model.
	r2 := &quotaRun{}
	text, err := runQuota(context.Background(), p, r2, setup, synthMember)
	if err != nil || text != "done by chatgpt" || p.count("synthetic") != 1 {
		t.Fatalf("second run = %q, %v, synthetic calls %d; want it skipped before the reset", text, err, p.count("synthetic"))
	}
	if len(r2.switches) != 1 || r2.switches[0]["reason"] != "known_exhausted" {
		t.Fatalf("switches = %v, want a known_exhausted switch", r2.switches)
	}
	// After the reset the model serves again.
	time.Sleep(time.Until(reset) + 20*time.Millisecond)
	p.mu.Lock()
	p.errs["synthetic"] = []error{nil}
	p.mu.Unlock()
	text, err = runQuota(context.Background(), p, &quotaRun{}, setup, synthMember)
	if err != nil || text != "done by synthetic" {
		t.Fatalf("run after the reset = %q, %v; want the model back in use", text, err)
	}
}

func TestEveryModelExhaustedRetriesBoundedAndRaisesAnIncident(t *testing.T) {
	shortenQuota(t)
	soon := func() error { return quotaErrFor("x", time.Now().Add(time.Hour)) }
	// Both models are out of quota for the first two chain passes, then
	// synthetic answers.
	p := &poolProvider{errs: map[string][]error{
		"synthetic": {soon(), soon(), nil},
		"chatgpt":   {soon(), soon(), soon()},
	}}
	r := &quotaRun{}
	text, err := runQuota(context.Background(), p, r, quotaSetup{chain: []provideriface.LLMSelection{lunaMember}}, synthMember)
	if err != nil || text != "done by synthetic" {
		t.Fatalf("run = %q, %v; want the run to continue once a model answers", text, err)
	}
	if r.phases["provider_quota_incident"] < 1 || r.phases["provider_quota_incident_cleared"] != 1 {
		t.Fatalf("phases = %v, want an incident and its clearing", r.phases)
	}
	w := r.incidents[0]
	if w == nil || w.Kind != types.WaitKindProviderQuota || w.Provider == "" || !strings.Contains(w.Detail, "every model of the chain is out of quota") {
		t.Fatalf("incident = %+v, want a typed provider_quota incident naming the chain", w)
	}
	if n := len(r.incidents); r.incidents[n-1] != nil || len(r.ends) != 1 || r.ends[0] != ProviderIncidentCleared {
		t.Fatalf("incidents %v ends %v, want the incident ended as cleared", r.incidents, r.ends)
	}
}

func TestEveryModelExhaustedForeverEndsTheRunBoundedAndVisibly(t *testing.T) {
	shortenQuota(t)
	forever := func(name string) []error { return []error{quotaErrFor(name, time.Now().Add(time.Hour))} }
	p := &poolProvider{errs: map[string][]error{"synthetic": forever("synthetic"), "chatgpt": forever("chatgpt")}}
	r := &quotaRun{}
	started := time.Now()
	_, err := runQuota(context.Background(), p, r, quotaSetup{chain: []provideriface.LLMSelection{lunaMember}}, synthMember)
	if err == nil || !strings.Contains(err.Error(), "every model is out of quota after 3 retry rounds") {
		t.Fatalf("err = %v, want a bounded, named end", err)
	}
	if time.Since(started) > 5*time.Second {
		t.Fatalf("took %s", time.Since(started))
	}
	if r.phases["provider_quota_incident"] != 3 || len(r.ends) != 1 || r.ends[0] != ProviderIncidentLoopEnded {
		t.Fatalf("phases %v ends %v, want 3 incident rounds and the incident ended with the loop", r.phases, r.ends)
	}
}

func TestCancelledRunLeavesTheIncident(t *testing.T) {
	shortenQuota(t)
	providerQuotaIncidentBackoff = []time.Duration{time.Hour}
	p := &poolProvider{errs: map[string][]error{"synthetic": {quotaErrFor("synthetic", time.Now().Add(time.Hour))}}}
	r := &quotaRun{}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := runQuota(ctx, p, r, quotaSetup{}, synthMember)
	if err == nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want the run's deadline", err)
	}
	if len(r.ends) != 1 || r.ends[0] != ProviderIncidentLoopEnded {
		t.Fatalf("ends = %v, want the incident ended as the loop's end", r.ends)
	}
}

func TestModelFailureDoesNotSwitchOrRaiseAnIncident(t *testing.T) {
	shortenQuota(t)
	outage := errors.New("gateway call failed: gateway client: provider synthetic model m: circuit open (model unhealthy) status 502 Bad Gateway")
	p := &poolProvider{errs: map[string][]error{"synthetic": {outage}}}
	r := &quotaRun{}
	_, err := runQuota(context.Background(), p, r, quotaSetup{chain: []provideriface.LLMSelection{lunaMember}}, synthMember)
	if err == nil || !strings.Contains(err.Error(), "model unhealthy") {
		t.Fatalf("err = %v, want today's failure after the outage retries", err)
	}
	if len(r.incidents) != 0 || len(r.switches) != 0 || r.phases["transient_provider"] != 3 {
		t.Fatalf("incidents %v switches %v phases %v", r.incidents, r.switches, r.phases)
	}
}
