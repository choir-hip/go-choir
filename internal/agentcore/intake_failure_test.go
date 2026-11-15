package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"
)

// One failed call must not lose a flush (third review round, Sonnet B2):
//
//	F1. The deadline is a fixed 5 s whatever the request; it scales with the
//	    request's estimated tokens (a deadline is not a call limit).
//	F2. The vendor refuses a request for size although the estimate said it
//	    fit: every capture of the call is unlabelled. It is halved and asked
//	    again instead.
//	F3. A timeout, 429 or 5xx leaves the flush unlabelled with no second try.
//	    It is asked once more; what fails twice is unlabelled and counted.

// scriptedJudge answers through inner, failing as the script says.
type scriptedJudge struct {
	inner CandidateJudge
	mu    sync.Mutex
	calls int
	// fail returns the error for call number n (1-based) over questions, or nil.
	fail func(n, questions int) error
}

func (s *scriptedJudge) Judge(ctx context.Context, state json.RawMessage, questions map[string]json.RawMessage) (json.RawMessage, error) {
	s.mu.Lock()
	s.calls++
	n := s.calls
	s.mu.Unlock()
	if err := s.fail(n, len(questions)); err != nil {
		return nil, err
	}
	return s.inner.Judge(ctx, state, questions)
}

func (s *scriptedJudge) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

func TestIntakeCallDeadlineScalesWithTheRequest(t *testing.T) {
	small := intakeCall{state: json.RawMessage(`{"a":"b"}`), questions: map[string]json.RawMessage{"q": json.RawMessage(`{}`)}}
	// 48k tokens of digits, one token each.
	big := intakeCall{state: json.RawMessage(`{"a":"` + string(make([]byte, 0)) + `"}`), questions: map[string]json.RawMessage{}}
	digits := make([]byte, 48000)
	for i := range digits {
		digits[i] = '7'
	}
	big.state = json.RawMessage(`{"a":"` + string(digits) + `"}`)
	if got := intakeCallTimeout(small); got > intakeLabelCallTimeout+2*intakeLabelTimeoutPerKToken {
		t.Fatalf("F1: a small call waits %v", got)
	}
	if got := intakeCallTimeout(big); got < intakeLabelCallTimeout+40*intakeLabelTimeoutPerKToken {
		t.Fatalf("F1: a 48k-token call waits only %v", got)
	}
}

func TestIntakeVendorSizeRefusalHalvesTheCallAndLosesNoCapture(t *testing.T) {
	rt, _ := testRuntime(t)
	inner := &fakeIntakeJudge{}
	judge := &scriptedJudge{inner: inner, fail: func(n, questions int) error {
		if questions > 12 {
			return errors.New("jev: 413 request entity too large: the context length exceeds the model's maximum")
		}
		return nil
	}}
	rt.SetCandidateJudge(judge)
	items := flushTestItems(12, "s")
	labels := mustLabels(t, rt.labelPoll(context.Background(), pollLib(rt, 5), "owner", "computer", "src", items, intakeBatchCaptures))
	if len(labels) != 12 {
		t.Fatalf("F2: %d of 12 captures labelled after a size refusal", len(labels))
	}
	// 24 questions refused, then two halves of 12.
	if got := judge.count(); got != 3 {
		t.Fatalf("F2: %d calls; want the refused one and its two halves", got)
	}
}

func TestIntakeTransientFailureIsAskedOnceMoreThenCountedUnlabelled(t *testing.T) {
	prev := intakeRetryBackoff
	intakeRetryBackoff = time.Millisecond
	t.Cleanup(func() { intakeRetryBackoff = prev })
	rt, _ := testRuntime(t)
	inner := &fakeIntakeJudge{}
	judge := &scriptedJudge{inner: inner, fail: func(n, questions int) error {
		if n == 1 {
			return errors.New("jev: 503 service unavailable")
		}
		return nil
	}}
	rt.SetCandidateJudge(judge)
	items := flushTestItems(3, "s")
	mustLabels(t, rt.labelPoll(context.Background(), pollLib(rt, 5), "owner", "computer", "src", items, intakeBatchCaptures))
	if got := judge.count(); got != 2 {
		t.Fatalf("F3: %d calls; want the failed one and one more", got)
	}

	always := &scriptedJudge{inner: inner, fail: func(n, questions int) error { return errors.New("jev: 429 rate limit") }}
	rt.SetCandidateJudge(always)
	results := rt.labelPoll(context.Background(), pollLib(rt, 5), "owner", "computer", "src", flushTestItems(3, "s"), intakeBatchCaptures)
	for i, r := range results {
		if r.Err == nil {
			t.Fatalf("F3: capture %d labelled though every try failed", i)
		}
	}
	if got := always.count(); got != 2 {
		t.Fatalf("F3: %d calls; a transient failure is asked once more, not forever", got)
	}
}
