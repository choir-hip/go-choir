package agentcore

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

// One failed call must not lose a flush, whatever the reason (the gateway does
// not relay the vendor's reason; docs/problems/jev-misuse-structural-
// assessment-2026-11-15.md, "The lane after three review rounds"):
//
//	F1. The deadline is a fixed 5 s whatever the request; it scales with the
//	    request's estimated tokens (a deadline is not a call limit).
//	F2. A call the gateway fails with 502 twice (the vendor refused it for size
//	    or anything else) is halved, to two levels, so the loss on a failure
//	    that only a smaller request avoids is none.
//	F3. A persistent failure makes a small fixed number of calls (2 for the
//	    call, 2 for its halves, 4 for the quarters) and no more.
//	F4. A 429 backs off and is asked again, and is never halved.
//	F5. The flush counters count the calls actually made.

func TestIntakeCallDeadlineScalesWithTheRequest(t *testing.T) {
	small := intakeCall{state: json.RawMessage(`{"a":"b"}`), questions: map[string]json.RawMessage{"q": json.RawMessage(`{}`)}}
	digits := make([]byte, 48000)
	for i := range digits {
		digits[i] = '7'
	}
	big := intakeCall{state: json.RawMessage(`{"a":"` + string(digits) + `"}`), questions: map[string]json.RawMessage{}}
	if got := intakeCallTimeout(small); got > intakeLabelCallTimeout+2*intakeLabelTimeoutPerKToken {
		t.Fatalf("F1: a small call waits %v", got)
	}
	if got := intakeCallTimeout(big); got < intakeLabelCallTimeout+40*intakeLabelTimeoutPerKToken {
		t.Fatalf("F1: a 48k-token call waits only %v", got)
	}
}

func TestIntakeGateway502OnABigCallIsHalvedAndLosesNoCapture(t *testing.T) {
	fastRetries(t)
	rt, _ := testRuntime(t)
	// The gateway fails every request of more than 12 questions with the 502
	// it gives for any upstream failure, a size refusal included.
	gw, judge := newFakeGateway(t, &fakeIntakeJudge{}, func(n, questions int) int {
		if questions > 12 {
			return http.StatusBadGateway
		}
		return 0
	})
	rt.SetCandidateJudge(judge)
	lib := pollLib(rt, 5)
	items := flushTestItems(12, "s")
	labels := mustLabels(t, rt.labelPoll(context.Background(), lib, "owner", "computer", "src", items, intakeBatchCaptures))
	if len(labels) != 12 {
		t.Fatalf("F2: %d of 12 captures labelled", len(labels))
	}
	// 24 questions: the call, its retry, then two halves of 12.
	if got := gw.count(); got != 4 {
		t.Fatalf("F2: %d gateway requests; want the failed call, its retry and two halves", got)
	}
	if lib.stats.calls != 4 {
		t.Fatalf("F5: the counters say %d calls; %d were made", lib.stats.calls, gw.count())
	}
}

func TestIntakeGatewayPersistentFailureIsAFixedSmallNumberOfCalls(t *testing.T) {
	fastRetries(t)
	rt, _ := testRuntime(t)
	gw, judge := newFakeGateway(t, &fakeIntakeJudge{}, func(n, questions int) int { return http.StatusBadGateway })
	rt.SetCandidateJudge(judge)
	results := rt.labelPoll(context.Background(), pollLib(rt, 5), "owner", "computer", "src", flushTestItems(12, "s"), intakeBatchCaptures)
	for i, r := range results {
		if r.Err == nil {
			t.Fatalf("F3: capture %d labelled though every request failed", i)
		}
	}
	// The call twice, its two halves, their four quarters: eight, not more.
	if got := gw.count(); got != 8 {
		t.Fatalf("F3: %d gateway requests on a persistent failure; want 8", got)
	}
}

func TestIntakeGateway429BacksOffAndIsNeverHalved(t *testing.T) {
	fastRetries(t)
	rt, _ := testRuntime(t)
	gw, judge := newFakeGateway(t, &fakeIntakeJudge{}, func(n, questions int) int {
		if n == 1 {
			return http.StatusTooManyRequests
		}
		return 0
	})
	rt.SetCandidateJudge(judge)
	items := flushTestItems(12, "s")
	mustLabels(t, rt.labelPoll(context.Background(), pollLib(rt, 5), "owner", "computer", "src", items, intakeBatchCaptures))
	if got := gw.count(); got != 2 {
		t.Fatalf("F4: %d requests; a 429 is asked again, whole", got)
	}
	always, judge2 := newFakeGateway(t, &fakeIntakeJudge{}, func(n, questions int) int { return http.StatusTooManyRequests })
	rt.SetCandidateJudge(judge2)
	results := rt.labelPoll(context.Background(), pollLib(rt, 5), "owner", "computer", "src", flushTestItems(12, "s"), intakeBatchCaptures)
	for i, r := range results {
		if r.Err == nil {
			t.Fatalf("F4: capture %d labelled through a persistent 429", i)
		}
	}
	if got := always.count(); got != 3 {
		t.Fatalf("F4: %d requests on a persistent 429; want the call and two backed-off tries, never halved", got)
	}
}

func TestJevNextStepOrdersTheChecks(t *testing.T) {
	rate := jevErr("gateway client: Jev judgment rate limit exceeded")
	if act, _ := jevNextStep(rate, 0, 0, true); act != jevRetry {
		t.Fatal("a 429 backs off")
	}
	if act, _ := jevNextStep(rate, 2, 0, true); act != jevFail {
		t.Fatal("a 429 is not halved when the tries are spent")
	}
	size := jevErr("openrouter: max_tokens_exceeded")
	if act, by := jevNextStep(size, 0, 0, true); act != jevHalve || by != "vendor_refused" {
		t.Fatalf("a known size refusal halves at once: %v %s", act, by)
	}
	gw := jevErr("gateway client: Jev judgment provider unavailable")
	if act, _ := jevNextStep(gw, 0, 0, true); act != jevRetry {
		t.Fatal("an unknown failure is asked once more")
	}
	if act, by := jevNextStep(gw, 1, 0, true); act != jevHalve || by != "call_failed" {
		t.Fatalf("an unknown failure that failed twice halves: %v %s", act, by)
	}
	if act, _ := jevNextStep(gw, 1, jevHalvingDepth, true); act != jevFail {
		t.Fatal("halving stops at the depth")
	}
	if act, _ := jevNextStep(gw, 1, 0, false); act != jevFail {
		t.Fatal("a single unit cannot be halved")
	}
}

type jevErr string

func (e jevErr) Error() string { return string(e) }
