package computerevent

import (
	"errors"
	"strings"
	"testing"
)

// candidate_shot (candidate_shot.go). Failure modes: the event unlocks a
// swap for a decision that is no longer pending (a stale worker); it moves a
// head; a payload with an unknown verdict, a pass with no shots, or a fail
// with no cause is accepted (FM8).
func TestReducerCandidateShotBindsThePendingDecisionAndMovesNoHead(t *testing.T) {
	genesis := testEvent(t, nil, EventGenesisImported)
	genesis.ResultingEffectiveCommitment = testDigestA
	head, err := Reduce(nil, genesis, TransitionInput{TargetStateCommitment: testDigestA})
	if err != nil {
		t.Fatal(err)
	}
	// Nothing pending: refused.
	idle := testEvent(t, &head, EventCandidateShot)
	idle.DecisionRef, idle.ProposedEffectRef = "decision:x", "artifact:bundle"
	if _, err := Reduce(&head, idle, TransitionInput{}); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("candidate_shot with nothing pending: %v", err)
	}
	accepted := testEvent(t, &head, EventEffectAccepted)
	accepted.ProposedEffectRef, accepted.DecisionRef, accepted.VerifierRefs = "artifact:bundle", "decision:consensus", []string{"artifact:verifier"}
	pending, err := Reduce(&head, accepted, TransitionInput{TargetStateCommitment: testDigestB})
	if err != nil {
		t.Fatal(err)
	}
	foreign := testEvent(t, &pending, EventCandidateShot)
	foreign.DecisionRef, foreign.ProposedEffectRef = "decision:other", "artifact:bundle"
	if _, err := Reduce(&pending, foreign, TransitionInput{}); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("candidate_shot naming another decision: %v", err)
	}
	unbound := testEvent(t, &pending, EventCandidateShot)
	unbound.DecisionRef = pending.PendingTransitionRef
	if _, err := Reduce(&pending, unbound, TransitionInput{}); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("candidate_shot naming no bundle: %v", err)
	}
	shot := testEvent(t, &pending, EventCandidateShot)
	shot.DecisionRef, shot.ProposedEffectRef = pending.PendingTransitionRef, "artifact:bundle"
	next, err := Reduce(&pending, shot, TransitionInput{})
	if err != nil {
		t.Fatal(err)
	}
	if next.DesiredEventHead != pending.DesiredEventHead || next.EffectiveEventHead != pending.EffectiveEventHead ||
		next.PendingTransitionRef != pending.PendingTransitionRef || next.DesiredStateCommitment != pending.DesiredStateCommitment ||
		next.EffectiveStateCommitment != pending.EffectiveStateCommitment {
		t.Fatalf("candidate_shot moved a head: before=%+v after=%+v", pending, next)
	}
}

func TestCandidateShotPayloadValidate(t *testing.T) {
	good := CandidateShotPayload{
		OperationID: "op-1", BundleDigest: testDigestA, HostBuildJobID: "job-1", PlanSHA256: testDigestB,
		Verdict: CandidateShotPass, Shots: []CandidateShotFile{{Name: "01-before-home.png", SHA256: testDigestA, Bytes: 10}},
	}
	if err := good.Validate(); err != nil {
		t.Fatal(err)
	}
	for name, fn := range map[string]func(p *CandidateShotPayload){
		"unknown verdict": func(p *CandidateShotPayload) { p.Verdict = "maybe" },
		"pass no shots":   func(p *CandidateShotPayload) { p.Shots = nil },
		"fail no cause":   func(p *CandidateShotPayload) { p.Verdict, p.Cause = CandidateShotFail, " " },
		"no job":          func(p *CandidateShotPayload) { p.HostBuildJobID = "" },
		"bad plan digest": func(p *CandidateShotPayload) { p.PlanSHA256 = "x" },
		"unbound shot":    func(p *CandidateShotPayload) { p.Shots[0].SHA256 = "x" },
	} {
		p := good
		p.Shots = append([]CandidateShotFile(nil), good.Shots...)
		fn(&p)
		if err := p.Validate(); err == nil {
			t.Errorf("%s: validated", name)
		}
	}
	fail := good
	fail.Verdict, fail.Cause, fail.Shots = CandidateShotFail, "shot worker died", nil
	if err := fail.Validate(); err != nil {
		t.Fatalf("a fail with its cause: %v", err)
	}
	if k := CandidateShotKey("op-1", "job-1"); !strings.Contains(k, "op-1") || !strings.Contains(k, "job-1") {
		t.Fatalf("the key must name the operation and the job: %s", k)
	}
	if CandidateShotKey("op-1", "job-1") == CandidateShotKey("op-1", "job-2") {
		t.Fatal("two host builds share a candidate_shot key")
	}
}
