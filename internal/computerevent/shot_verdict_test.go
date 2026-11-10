package computerevent

import (
	"errors"
	"testing"
)

// shot_verdict (shot_verdict.go). Failure modes: the event unlocks a
// swap for a decision that is no longer pending (a stale worker); it moves a
// head; a payload with an unknown verdict, a pass with no shots, or a fail
// with no cause is accepted (FM8).
func TestReducerCandidateShotBindsThePendingDecisionAndMovesNoHead(t *testing.T) {
	for _, kind := range []EventKind{EventCandidateShot, EventShotVerdict, EventShotReuse} {
		t.Run(string(kind), func(t *testing.T) { reducerShotKindBindsThePendingDecision(t, kind) })
	}
}

func reducerShotKindBindsThePendingDecision(t *testing.T, kind EventKind) {
	genesis := testEvent(t, nil, EventGenesisImported)
	genesis.ResultingEffectiveCommitment = testDigestA
	head, err := Reduce(nil, genesis, TransitionInput{TargetStateCommitment: testDigestA})
	if err != nil {
		t.Fatal(err)
	}
	// Nothing pending: refused.
	idle := testEvent(t, &head, kind)
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
	foreign := testEvent(t, &pending, kind)
	foreign.DecisionRef, foreign.ProposedEffectRef = "decision:other", "artifact:bundle"
	if _, err := Reduce(&pending, foreign, TransitionInput{}); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("candidate_shot naming another decision: %v", err)
	}
	unbound := testEvent(t, &pending, kind)
	unbound.DecisionRef = pending.PendingTransitionRef
	if _, err := Reduce(&pending, unbound, TransitionInput{}); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("candidate_shot naming no bundle: %v", err)
	}
	shot := testEvent(t, &pending, kind)
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

func TestShotVerdictPayloadValidate(t *testing.T) {
	good := ShotVerdictPayload{
		OperationID: "op-1", BundleDigest: testDigestA, HostBuildJobID: "job-1", ShotJobID: "shot-1", PlanSHA256: testDigestB,
		Verdict: ShotVerdictPass, Findings: []ShotFindingRecord{{Pair: "01-home", BeforeSHA256: testDigestA, AfterSHA256: testDigestB}},
		Shots: []ShotFile{{Name: "01-before-home.png", SHA256: testDigestA, Bytes: 10}},
	}
	if err := good.Validate(); err != nil {
		t.Fatal(err)
	}
	for name, fn := range map[string]func(p *ShotVerdictPayload){
		"unknown verdict": func(p *ShotVerdictPayload) { p.Verdict = "maybe" },
		"pass no finding": func(p *ShotVerdictPayload) { p.Findings = nil },
		"fail no reason":  func(p *ShotVerdictPayload) { p.Verdict, p.Reason = ShotVerdictFail, " " },
		"no job":          func(p *ShotVerdictPayload) { p.HostBuildJobID = "" },
		"no shot job":     func(p *ShotVerdictPayload) { p.ShotJobID = "" },
		"bad plan digest": func(p *ShotVerdictPayload) { p.PlanSHA256 = "x" },
		"unbound shot":    func(p *ShotVerdictPayload) { p.Shots = []ShotFile{{Name: "a.png", SHA256: "x"}} },
		"unbound finding": func(p *ShotVerdictPayload) { p.Findings = []ShotFindingRecord{{Pair: "p", BeforeSHA256: "x", AfterSHA256: testDigestA}} },
	} {
		p := good
		fn(&p)
		if err := p.Validate(); err == nil {
			t.Errorf("%s: validated", name)
		}
	}
	fail := good
	fail.Verdict, fail.Reason, fail.Findings = ShotVerdictFail, "the list is empty", nil
	if err := fail.Validate(); err != nil {
		t.Fatalf("a fail with its reason: %v", err)
	}
	if k := ShotVerdictKey("op-1", "job-1"); k != "shot-verdict:op-1:job-1" {
		t.Fatalf("key = %s", k)
	}
	if ShotVerdictKey("op-1", "job-1") == ShotVerdictKey("op-1", "job-2") {
		t.Fatal("two host builds share a shot verdict key")
	}
}
