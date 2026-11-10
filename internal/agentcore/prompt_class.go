package agentcore

import (
	"context"
	"log"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/promptroute"
)

// The input class at the prompt_router call site (lane D, handoff 4.D): the
// prompt bar is the universal input and Jev classifies the owner's text as an
// edit, a comment or a request. It is the router's second typed choice
// question, asked through the same System 1 service, so the tape and the Jev
// record carry every call, failures included. It performs no effect: the shell
// sends the text, and the one revise call and the fork route enforce ownership.

const (
	classHostInputRef   = system1HostRefPrefix + "class_input"
	classHostClassesRef = system1HostRefPrefix + "class_classes"
)

// PromptClass is one classification as the prompt bar reads it.
type PromptClass struct {
	Class        string             `json:"class"`
	Rule         string             `json:"rule"`
	Probability  float64            `json:"probability,omitempty"`
	Distribution map[string]float64 `json:"distribution,omitempty"`
	// Alternatives are the other classes the owner may re-send under, best
	// first; on an abstained class they are the judge's next choices, and
	// otherwise every other offered class.
	Alternatives []string `json:"alternatives,omitempty"`
	Reason       string   `json:"reason,omitempty"`
	Model        string   `json:"model,omitempty"`
	ReceiptRef   string   `json:"receipt_ref,omitempty"`
}

// ClassifyPromptInput classifies the owner's text for a prompt bar aimed at
// state. It never fails: every failure is the deterministic default (edit on
// a document the owner owns, comment on one they do not, request with none),
// with its cause as the rule. commandID only names the call's run; a retry asks again.
func (rt *Runtime) ClassifyPromptInput(ctx context.Context, ownerID, commandID, text string, state promptroute.DocState) (class PromptClass) {
	candidates := promptroute.ClassCandidates(state)
	defer func() {
		if p := recover(); p != nil {
			log.Printf("prompt class: panic: %v; taking the default class", p)
			d := promptroute.Fallback(candidates, promptroute.RuleJudgeFailed, "the classifier panicked")
			class = classOf(candidates, d, "", "")
		}
	}()
	if len(candidates) < 2 {
		return classOf(candidates, promptroute.Fallback(candidates, promptroute.RuleSoleCandidate, "nothing else fits"), "", "")
	}
	if rt == nil || rt.candidateJudge == nil {
		return classOf(candidates, promptroute.Fallback(candidates, promptroute.RuleNoJudge, "no decision model is bound on this computer"), "", "")
	}
	runID := rt.PromptBarRunID(ownerID, "class:"+strings.TrimSpace(commandID))
	asked := rt.askChoice(ctx, ownerID, runID, text, candidates, routerChoice{
		qid: promptroute.ClassQuestionID, text: promptroute.ClassQuestion, inputRef: classHostInputRef, listRef: classHostClassesRef,
		inputLead: "The owner's input to the prompt bar: ", list: promptroute.ClassDestinations(candidates),
	})
	return classOf(candidates, asked.Decision, asked.Model, asked.ReceiptRef)
}

func classOf(candidates []promptroute.Candidate, d promptroute.Decision, model, receipt string) PromptClass {
	alts := d.Alternatives
	if len(alts) == 0 {
		for _, c := range candidates {
			if c.Desk != d.Destination {
				alts = append(alts, c.Desk)
			}
		}
	}
	return PromptClass{Class: d.Destination, Rule: d.Rule, Probability: d.Probability, Distribution: d.Distribution,
		Alternatives: alts, Reason: d.Reason, Model: model, ReceiptRef: receipt}
}
