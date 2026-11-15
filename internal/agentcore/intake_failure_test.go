package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/gatewayruntime"
)

// A dial failure (the gateway's port refused: no request left the guest) fails
// the call at once, is never asked again or halved, and stops the flush from
// sending its other calls (docs/problems/jev-misuse-structural-assessment-
// 2026-11-15.md). Every halving seen on 2026-11-15 was "connection refused"
// while the gateway restarted. Every other failure keeps its treatment: the
// gateway answers every upstream failure as 502, so the guest cannot tell a
// size refusal from an outage, and those are asked once more and halved.
//
// Failure list for the dial change, written before the code:
//
//	D1. A closed port, through the production client, is jevTransportDown and
//	    jevNextStep fails it at any depth, with no retry and no halve.
//	D2. A dial error whose address holds the digits 429 or 529 (a port) is
//	    still a dial failure, not a rate refusal.
//	D3. A dial failure stops the flush: one call sent, the rest withheld.
//	D4. A 502 in one call does not stop the flush's other calls.
//	D5. A per-call timeout on intake is asked once more (it is not a dial
//	    failure), as on main.
//
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

// A deterministic failure (the gateway's 400 for an invalid request) is not
// transient: asking again or halving cannot change it, so it fails once,
// visibly, through the real client's Transient().
func TestIntakeGateway400FailsOnceAndIsNeitherRetriedNorHalved(t *testing.T) {
	fastRetries(t)
	rt, _ := testRuntime(t)
	gw, judge := newFakeGateway(t, &fakeIntakeJudge{}, func(n, questions int) int { return http.StatusBadRequest })
	rt.SetCandidateJudge(judge)
	results := rt.labelPoll(context.Background(), pollLib(rt, 5), "owner", "computer", "src", flushTestItems(12, "s"), intakeBatchCaptures)
	for i, r := range results {
		if r.Err == nil {
			t.Fatalf("a 400 labelled capture %d", i)
		}
	}
	if got := gw.count(); got != 1 {
		t.Fatalf("%d requests on a 400; want exactly 1", got)
	}
}

// The gateway 413 (a body over its transport cap) is the one 4xx that halving
// cures: it halves at once, and is not asked whole twice.
func TestIntakeGateway413HalvesAtOnceAndLosesNoCapture(t *testing.T) {
	fastRetries(t)
	rt, _ := testRuntime(t)
	gw, judge := newFakeGateway(t, &fakeIntakeJudge{}, func(n, questions int) int {
		if questions > 12 {
			return http.StatusRequestEntityTooLarge
		}
		return 0
	})
	rt.SetCandidateJudge(judge)
	labels := mustLabels(t, rt.labelPoll(context.Background(), pollLib(rt, 5), "owner", "computer", "src", flushTestItems(12, "s"), intakeBatchCaptures))
	if len(labels) != 12 {
		t.Fatalf("%d of 12 captures labelled after a 413", len(labels))
	}
	if got := gw.count(); got != 3 {
		t.Fatalf("%d requests; want the 413 once and two halves", got)
	}
}

// A 200 that parses but has the wrong shape is deterministic: it fails once.
func TestIntakeGatewayWrongShapedAnswerFailsOnce(t *testing.T) {
	fastRetries(t)
	rt, _ := testRuntime(t)
	gw, judge := newFakeGateway(t, &fakeIntakeJudge{}, func(n, questions int) int { return wrongShape })
	rt.SetCandidateJudge(judge)
	results := rt.labelPoll(context.Background(), pollLib(rt, 5), "owner", "computer", "src", flushTestItems(12, "s"), intakeBatchCaptures)
	for i, r := range results {
		if r.Err == nil {
			t.Fatalf("capture %d labelled from a wrong-shaped answer", i)
		}
	}
	if got := gw.count(); got != 1 {
		t.Fatalf("%d requests; a wrong-shaped answer fails once", got)
	}
}

// A cleanly closed short body (the client invalid JSON response) is a
// transient drop, so it gets the ordinary retry and halving.
func TestJevNextStepTreatsAShortBodyAsTransient(t *testing.T) {
	short := jevErr("judgment client: invalid JSON response")
	if act, _ := jevNextStep(short, 0, 0, true); act != jevRetry {
		t.Fatal("a short body is asked once more")
	}
}

// closedGatewayJudge is the production client pointed at a port nothing
// listens on, which is what a restarting gateway looks like from the guest.
// The port is drawn again until its digits hold neither 429 nor 529.
func closedGatewayJudge(t *testing.T) CandidateJudge {
	t.Helper()
	for {
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		addr := l.Addr().String()
		_ = l.Close()
		if strings.Contains(addr, "429") || strings.Contains(addr, "529") {
			continue
		}
		judge := gatewayJudge{client: gatewayruntime.NewJudgmentClient("http://"+addr, "test-token")}
		// Another process may have taken the port in the meantime: keep it
		// only if a call fails as a dial failure, else draw again.
		_, err = judge.Judge(context.Background(), json.RawMessage(`{}`), map[string]json.RawMessage{"q": json.RawMessage(`{}`)})
		if err != nil && jevTransportDown(err) {
			return judge
		}
	}
}

func TestJevNextStepATransportDialFailureFailsOnceNeverRetriedNeverHalved(t *testing.T) {
	judge := closedGatewayJudge(t)
	_, err := judge.Judge(context.Background(), json.RawMessage(`{}`), map[string]json.RawMessage{"q": json.RawMessage(`{}`)})
	if err == nil {
		t.Fatal("D1: a closed port answered")
	}
	if !jevTransportDown(err) {
		t.Fatalf("D1: the production client's dial failure is not jevTransportDown: %v", err)
	}
	for depth := 0; depth <= jevHalvingDepth; depth++ {
		if act, by := jevNextStep(err, 0, depth, true); act != jevFail || by != "" {
			t.Fatalf("D1: a dial failure at depth %d: %v %q; want jevFail", depth, act, by)
		}
	}
	if jevTransportDown(jevErr("gateway client: Jev judgment provider unavailable")) {
		t.Fatal("D1: a 502 is not a dial failure")
	}
}

func TestJevNextStepADialAddressWithRateDigitsIsStillADialFailure(t *testing.T) {
	for _, port := range []string{"42913", "52900", "8429"} {
		err := jevErr("judgment client: http call: Post \"http://x:" + port + "/provider/v1/judgments\": dial tcp 127.0.0.1:" + port + ": connect: connection refused")
		if act, _ := jevNextStep(err, 0, 0, true); act != jevFail {
			t.Fatalf("D2: a dial failure to port %s is not failed at once: %v", port, act)
		}
	}
	// A refusal that carries a rate signal and no dial is still a backoff.
	if act, _ := jevNextStep(jevErr("gateway client: status 429"), 0, 0, true); act != jevRetry {
		t.Fatal("D2: a 429 no longer backs off")
	}
}

// A closed gateway costs one call per flush, however many calls the flush
// planned: the rest are not sent to the closed port.
func TestIntakeDialFailureStopsTheRestOfTheFlush(t *testing.T) {
	fastRetries(t)
	rt, _ := testRuntime(t)
	rt.SetCandidateJudge(closedGatewayJudge(t))
	lib := pollLib(rt, 5)
	items := flushTestItems(6, "s")
	// The legacy byte policy with a batch of one plans one call per capture.
	results := rt.labelPollWith(context.Background(), lib, "owner", "computer", "src", items, 1, intakeLegacyBytePolicy)
	for i, r := range results {
		if r.Err == nil {
			t.Fatalf("D3: capture %d labelled through a closed port", i)
		}
	}
	if lib.stats.calls != 1 {
		t.Fatalf("D3: %d calls sent; want 1, then the rest withheld", lib.stats.calls)
	}
}

// With the plan proven to hold several calls, a 502 in the first still lets
// the others go: only a dial failure stops the flush.
func TestIntake502InOneCallDoesNotStopTheOthers(t *testing.T) {
	fastRetries(t)
	rt, _ := testRuntime(t)
	gw, judge := newFakeGateway(t, &fakeIntakeJudge{}, func(n, questions int) int {
		if n <= 2 {
			return http.StatusBadGateway
		}
		return 0
	})
	rt.SetCandidateJudge(judge)
	lib := pollLib(rt, 5)
	results := rt.labelPollWith(context.Background(), lib, "owner", "computer", "src", flushTestItems(6, "s"), 1, intakeLegacyBytePolicy)
	if gw.count() < 4 {
		t.Fatalf("D4: %d requests; a 502 (asked twice) must not stop the rest of the plan", gw.count())
	}
	if results[0].Err == nil {
		t.Fatal("D4: the capture of the failed call was labelled")
	}
	if results[len(results)-1].Err != nil {
		t.Fatalf("D4: a capture of a call that succeeded is unlabelled: %v", results[len(results)-1].Err)
	}
}

// timeoutOnceJudge fails its first call as the per-call deadline does.
type timeoutOnceJudge struct {
	*fakeIntakeJudge
	n atomic.Int32
}

func (j *timeoutOnceJudge) Judge(ctx context.Context, state json.RawMessage, questions map[string]json.RawMessage) (json.RawMessage, error) {
	return j.JudgeScoped(ctx, "", state, questions)
}

func (j *timeoutOnceJudge) JudgeScoped(ctx context.Context, scope string, state json.RawMessage, questions map[string]json.RawMessage) (json.RawMessage, error) {
	if j.n.Add(1) == 1 {
		return nil, fmt.Errorf("Jev: %w", context.DeadlineExceeded)
	}
	return j.fakeIntakeJudge.JudgeScoped(ctx, scope, state, questions)
}

// A per-call timeout is not a dial failure: intake asks the call once more, as
// before, and the captures are labelled by the second call.
func TestIntakePerCallTimeoutIsAskedOnceMore(t *testing.T) {
	fastRetries(t)
	rt, _ := testRuntime(t)
	judge := &timeoutOnceJudge{fakeIntakeJudge: &fakeIntakeJudge{}}
	rt.SetCandidateJudge(judge)
	lib := pollLib(rt, 5)
	results := rt.labelPoll(context.Background(), lib, "owner", "computer", "src", flushTestItems(3, "s"), intakeBatchCaptures)
	for i, r := range results {
		if r.Err != nil {
			t.Fatalf("D5: capture %d unlabelled after a timed-out call: %v", i, r.Err)
		}
	}
	if lib.stats.calls != 2 {
		t.Fatalf("D5: %d calls; want the timed-out call and its one retry", lib.stats.calls)
	}
	if act, _ := jevNextStep(fmt.Errorf("Jev: %w", context.DeadlineExceeded), 0, 0, true); act != jevRetry {
		t.Fatal("D5: a timeout is not asked once more")
	}
	if act, by := jevNextStep(fmt.Errorf("Jev: %w", context.DeadlineExceeded), 1, 0, true); act != jevHalve || by != "call_failed" {
		t.Fatalf("D5: a timeout that failed twice is not halved: %v %s", act, by)
	}
}
