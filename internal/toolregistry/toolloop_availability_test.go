package toolregistry

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Availability-aware routing (docs/problems/pool-routing-walks-known-dead-
// members-2026-11-09.md). Ways it could fail, written down before the code:
//  1. a member one run saw fail is called again by the next run on the same
//     ledger, so every turn re-learns by failing;
//  2. a pool failover steps onto a member the ledger already holds;
//  3. a demotion never expires, or does not grow on repeated failure;
//  4. a success does not clear the hold, so a recovered member stays demoted;
//  5. our own cancellation demotes a healthy model;
//  6. every member held: the run is refused instead of trying the active one.

var routerSel = provideriface.LLMSelection{Provider: "openrouter", Model: "stealth/crouton-alpha"}

func circuitOpen(provider string) error {
	return &gatewayLikeError{"gateway call failed: gateway client: provider " + provider + " model m: circuit open (model unhealthy)", true}
}

func runWithLedger(p *poolProvider, ledger *QuotaLedger, primary provideriface.LLMSelection, fallbacks ...provideriface.LLMSelection) (string, error) {
	text, _, err := RunToolLoop(context.Background(), p, nil, []json.RawMessage{json.RawMessage(`{"role":"user","content":"hello"}`)}, "system", 0,
		func(types.EventKind, string, json.RawMessage) {}, nil,
		WithToolLoopLLMConfig(primary), WithToolLoopFallbackLLMConfigs(fallbacks), WithToolLoopConversationID("run-1"), WithQuotaLedger(ledger))
	return text, err
}

func TestFailedMemberIsSkippedByTheNextRunOnTheSameLedger(t *testing.T) {
	p := &poolProvider{errs: map[string][]error{"synthetic": {circuitOpen("synthetic")}}}
	ledger := NewQuotaLedger()
	for run := 1; run <= 3; run++ {
		text, err := runWithLedger(p, ledger, synthMember, lunaMember)
		if err != nil || text != "done by chatgpt" {
			t.Fatalf("run %d = %q, %v", run, text, err)
		}
	}
	if got := p.count("synthetic"); got != 1 {
		t.Fatalf("synthetic called %d times over three runs, want once (the first run found it down)", got)
	}
}

func TestPoolFailoverSkipsAMemberTheLedgerHolds(t *testing.T) {
	p := &poolProvider{errs: map[string][]error{"synthetic": {circuitOpen("synthetic")}}}
	ledger := NewQuotaLedger()
	ledger.Mark(lunaMember, time.Now().Add(time.Hour))
	text, err := runWithLedger(p, ledger, synthMember, lunaMember, routerSel)
	if err != nil || text != "done by openrouter" {
		t.Fatalf("run = %q, %v", text, err)
	}
	if p.count("chatgpt") != 0 {
		t.Fatalf("chatgpt called %d times though the ledger held it", p.count("chatgpt"))
	}
}

func TestDemotionGrowsExpiresAndClearsOnSuccess(t *testing.T) {
	l := NewQuotaLedger()
	now := time.Unix(1_000_000, 0)
	first := l.Demote(synthMember, now).Sub(now)
	second := l.Demote(synthMember, now).Sub(now)
	if first <= 0 || second != 2*first {
		t.Fatalf("holds %s then %s, want the second to double the first", first, second)
	}
	var last time.Duration
	for i := 0; i < 20; i++ {
		last = l.Demote(synthMember, now).Sub(now)
	}
	if last != failureHoldCap {
		t.Fatalf("hold after many failures = %s, want the cap %s", last, failureHoldCap)
	}
	if l.Until(synthMember, now.Add(last-time.Second)).IsZero() || !l.Until(synthMember, now.Add(last+time.Second)).IsZero() {
		t.Fatal("a demotion must hold until its time and no longer")
	}
	l.Demote(synthMember, now)
	l.Succeed(synthMember)
	if !l.Until(synthMember, now).IsZero() {
		t.Fatal("a success must clear the hold")
	}
	if got := l.Demote(synthMember, now).Sub(now); got != first {
		t.Fatalf("after a success the hold restarts at %s, got %s", first, got)
	}
}

func TestEveryMemberHeldStillTriesTheActiveOneAndClearsItOnSuccess(t *testing.T) {
	p := &poolProvider{}
	ledger := NewQuotaLedger()
	until := time.Now().Add(time.Hour)
	ledger.Mark(synthMember, until)
	ledger.Mark(lunaMember, until)
	text, err := runWithLedger(p, ledger, synthMember, lunaMember)
	if err != nil || text != "done by synthetic" {
		t.Fatalf("run = %q, %v; want the active member tried when nothing is free", text, err)
	}
	if !ledger.Until(synthMember, time.Now()).IsZero() {
		t.Fatal("a member that answered must leave the ledger")
	}
}

func TestOurOwnCancellationDoesNotDemote(t *testing.T) {
	p := &poolProvider{errs: map[string][]error{"synthetic": {context.Canceled}}}
	ledger := NewQuotaLedger()
	if _, err := runWithLedger(p, ledger, synthMember, lunaMember); err == nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want the cancellation", err)
	}
	if !ledger.Until(synthMember, time.Now()).IsZero() {
		t.Fatal("a cancellation must not demote the model")
	}
}
