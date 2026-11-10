package deskjudge

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// The completion gate of desk_complete (owner ruling 4, 2026-11-09): the
// runtime runs the assignment's own checks in the capsule, Jev reads the
// change, and a completed pass needs both. The agent's word is a claim; the
// agent supplies no command that counts as acceptance (C1).

// ReadBoundTokens is the size of the change Jev reads: about 50,000 tokens of
// the diff and its context (ruling 4). The caller builds the read to this
// bound and says when it had to cut.
const ReadBoundTokens = 50_000

// CheckRun is the runtime's run of one assignment check: Command is the
// assignment's text, ExecutionRef the receipt the executor minted.
type CheckRun struct {
	Command      string
	Ran          bool
	ExitCode     int
	ExecutionRef string
}

// Change is what Jev reads at completion.
type Change struct {
	Text string
	// Tokens is the estimated size of Text; Truncated says the change was cut
	// to the bound, so Jev has not seen all of it (C6).
	Tokens    int
	Truncated bool
}

// Completion is the input to Admit.
type Completion struct {
	Result string
	// Checks are the assignment's checks, from the Cast: never the agent's.
	Checks []string
	Runs   []CheckRun
	// AgentRefs are execution refs the agent cited: advisory evidence that
	// rides along but never stands in for a run of the assignment's checks.
	AgentRefs []string
	Change    Change
}

// Verdict is the outcome of the gate.
type Verdict string

const (
	// Admitted: stage Complete as the agent asked.
	Admitted Verdict = "admit"
	// Refused: a check did not pass; the reason goes back as a tool result.
	Refused Verdict = "refuse"
	// Escalated: the checks are not enough, or Jev could not say; management's
	// model decides. The assignment stays open.
	Escalated Verdict = "escalate"
)

// Admission is the gate's answer and the evidence it cites (C9).
type Admission struct {
	Verdict Verdict
	Reason  string
	// ExecutionRefs are the receipts to cite on Complete: the runtime's runs
	// first, then the agent's, de-duplicated.
	ExecutionRefs []string
	// EvidenceRefs name the gate's own facts so the tape carries them.
	EvidenceRefs []string
	Model        string
}

const (
	completeQuestionID  = "suffice"
	completeTimeout     = 5 * time.Minute
	completeTaskPrompt  = "An engineering desk says its assignment is complete. The runtime ran the assignment's own checks and they passed. You are shown the change."
	completeInstruction = "Do the passing checks suffice to show this change does what the assignment asked and breaks nothing the checks cannot see (suffice), or should the management desk's model look at it first (escalate)? Choose escalate if the change touches behaviour the checks do not exercise."
)

// Admit decides one desk_complete. Only a completed result claims a pass; a
// failed, blocked or partial result is admitted without the gate (C7). A
// completed result needs the assignment to name checks (C2, C7), every check
// run by the runtime and green (C3), and Jev's "suffice" on a whole read
// (C4, C5, C6).
func Admit(ctx context.Context, j Judge, in Completion) Admission {
	if strings.TrimSpace(in.Result) != "completed" {
		return Admission{Verdict: Admitted, Reason: "result " + strings.TrimSpace(in.Result) + " claims no pass"}
	}
	if len(in.Checks) == 0 {
		return Admission{Verdict: Escalated, Reason: "the assignment names no checks, so a completed result cannot pass on the runtime's runs"}
	}
	byCmd := map[string]CheckRun{}
	for _, r := range in.Runs {
		byCmd[strings.TrimSpace(r.Command)] = r
	}
	var refs []string
	for _, c := range in.Checks {
		r, ok := byCmd[strings.TrimSpace(c)]
		switch {
		case !ok || !r.Ran:
			return Admission{Verdict: Refused, Reason: fmt.Sprintf("the runtime did not run the assignment's check %q", c)}
		case r.ExitCode != 0:
			return Admission{Verdict: Refused, Reason: fmt.Sprintf("the assignment's check %q failed with exit code %d; fix it and complete again", c, r.ExitCode),
				ExecutionRefs: dedupe([]string{r.ExecutionRef})}
		case strings.TrimSpace(r.ExecutionRef) == "":
			return Admission{Verdict: Refused, Reason: fmt.Sprintf("the assignment's check %q has no executor receipt", c)}
		}
		refs = append(refs, r.ExecutionRef)
	}
	refs = dedupe(append(refs, in.AgentRefs...))
	evidence := []string{fmt.Sprintf("completion-read-tokens:%d", in.Change.Tokens)}
	text, cut := bound(in.Change.Text, ReadBoundTokens*4)
	truncated := in.Change.Truncated || cut
	a, err := askChoice(ctx, j, completeTimeout, completeTaskPrompt, completeQuestionID, completeInstruction,
		[]string{"suffice", "escalate"}, map[string]string{"change": text})
	if err != nil {
		evidence = append(evidence, "completion-jev:none")
		return Admission{Verdict: Escalated, Reason: "no judgment ran, so management decides: " + err.Error(), ExecutionRefs: refs, EvidenceRefs: evidence}
	}
	evidence = append(evidence, "completion-jev:"+a.Choice)
	switch {
	case a.Choice != "suffice":
		return Admission{Verdict: Escalated, Reason: "Jev asks management to look at the change before it passes", ExecutionRefs: refs, EvidenceRefs: evidence, Model: a.Model}
	case truncated:
		evidence = append(evidence, "completion-read:truncated")
		return Admission{Verdict: Escalated, Reason: "the change is larger than the read bound, so Jev saw part of it; management decides", ExecutionRefs: refs, EvidenceRefs: evidence, Model: a.Model}
	}
	return Admission{Verdict: Admitted, Reason: "every assignment check passed and Jev read the whole change", ExecutionRefs: refs, EvidenceRefs: evidence, Model: a.Model}
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
