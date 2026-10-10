package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/capsule/transaction"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// A frozen self-development candidate is judged by Jev, a decision model, not
// by a second engineering model run (owner direction 2026-10-10: "get rid of
// the verification that we have. We should use Jev"). The judgment reads the
// owner's objective, the implementation's report and the frozen bundle, and
// answers typed questions. Its record is the same verification event the
// engineering verifier wrote, so the verifier certificate, approval and
// checkpoint chain are unchanged. Scoring precommitment records comes later.

// CandidateJudge asks the decision model typed questions about one state. It
// returns the model's full answer distribution.
type CandidateJudge interface {
	Judge(ctx context.Context, state json.RawMessage, questions map[string]json.RawMessage) (json.RawMessage, error)
}

// SetCandidateJudge binds the decision model used to judge frozen candidates.
func (rt *Runtime) SetCandidateJudge(judge CandidateJudge) {
	rt.candidateJudge = judge
}

// candidateJudgmentModel names the model the gateway pins for judgments.
const candidateJudgmentModel = "typesafe/jev-1.13"

// candidateJudgmentQuestions are the typed yes/no questions every frozen
// candidate must pass. All three must answer yes.
var candidateJudgmentQuestions = map[string]map[string]any{
	"does_what_was_asked": {
		"type":         "choice",
		"instructions": "Judge from the state whether the frozen change does what the owner's objective asked.",
		"criteria": map[string]string{
			"yes": "The implementation report and file effects show a change that accomplishes the objective.",
			"no":  "The change is missing, does something else, or the evidence does not show the objective was met.",
		},
		"choices": []string{"yes", "no"},
	},
	"limited_to_the_objective": {
		"type":         "choice",
		"instructions": "Judge from the state whether the frozen change is limited to what the objective needs.",
		"criteria": map[string]string{
			"yes": "Every file effect is plausibly needed for the objective; nothing unrelated is changed.",
			"no":  "The change touches files or behavior the objective did not ask for.",
		},
		"choices": []string{"yes", "no"},
	},
	"reversible": {
		"type":         "choice",
		"instructions": "Judge from the state whether the evidence shows the change can be reverted to the prior state.",
		"criteria": map[string]string{
			"yes": "The report or bundle shows how the prior state is restored, or the change is a pure addition that removal reverts.",
			"no":  "Nothing in the state shows how to return to the prior state.",
		},
		"choices": []string{"yes", "no"},
	},
}

// candidateJudgment is the parsed verdict.
type candidateJudgment struct {
	Decision string                     `json:"decision"`
	Model    string                     `json:"model"`
	Answers  map[string]candidateAnswer `json:"answers"`
}

type candidateAnswer struct {
	Choice     string             `json:"choice"`
	Yes        float64            `json:"p_yes"`
	Confidence float64            `json:"confidence"`
	Raw        map[string]float64 `json:"probabilities,omitempty"`
}

// parseCandidateJudgment reads the decision model's distribution. It passes
// only when every question was answered and each answer chose yes with at
// least even probability; anything missing or malformed fails closed.
func parseCandidateJudgment(raw json.RawMessage) (candidateJudgment, error) {
	var distribution struct {
		Model   string `json:"model"`
		Answers map[string]struct {
			Type          string             `json:"type"`
			Choice        string             `json:"choice"`
			Probabilities map[string]float64 `json:"probabilities"`
			Confidence    float64            `json:"confidence"`
		} `json:"answers"`
	}
	if err := json.Unmarshal(raw, &distribution); err != nil {
		return candidateJudgment{}, fmt.Errorf("candidate judgment: decode distribution: %w", err)
	}
	judgment := candidateJudgment{Decision: "pass", Model: strings.TrimSpace(distribution.Model), Answers: map[string]candidateAnswer{}}
	for name := range candidateJudgmentQuestions {
		answer, ok := distribution.Answers[name]
		if !ok || answer.Choice == "" {
			judgment.Decision = "fail"
			judgment.Answers[name] = candidateAnswer{Choice: "unanswered"}
			continue
		}
		yes := answer.Probabilities["yes"]
		judgment.Answers[name] = candidateAnswer{Choice: answer.Choice, Yes: yes, Confidence: answer.Confidence, Raw: answer.Probabilities}
		if answer.Choice != "yes" || yes < 0.5 {
			judgment.Decision = "fail"
		}
	}
	if judgment.Model == "" {
		return candidateJudgment{}, fmt.Errorf("candidate judgment: distribution names no model")
	}
	return judgment, nil
}

// candidateJudgmentState is what the decision model reads: the objective, the
// implementation's own account, and the frozen file effects.
func (rt *Runtime) candidateJudgmentState(ctx context.Context, operation selfdev.Operation, objective string, implementation types.EngineeringAssignment) (json.RawMessage, []string, error) {
	var summaries, evidenceRefs []string
	for _, ref := range implementation.ReportRefs {
		report, err := rt.store.GetEngineeringAssignmentReportByCanonicalID(ctx, ref)
		if err != nil || report.AssignmentID != implementation.AssignmentID {
			continue
		}
		if summary := strings.TrimSpace(report.Summary); summary != "" {
			summaries = append(summaries, summary)
		}
		evidenceRefs = append(evidenceRefs, ref)
	}
	if len(evidenceRefs) == 0 {
		return nil, nil, fmt.Errorf("candidate judgment: implementation %s has no report to judge", implementation.AssignmentID)
	}
	effects := []string{}
	var rejected bool
	rejectReason := ""
	sourcePatch := false
	if raw, err := os.ReadFile(filepath.Join(rt.selfdevUpdaterRoot, "incoming", operation.BundleDigest, "bundle.draft.json")); err == nil {
		var bundle transaction.CapsuleEffectBundle
		if json.Unmarshal(raw, &bundle) == nil {
			for _, effect := range bundle.OrderedFileEffects {
				effects = append(effects, effect.Kind+" "+effect.Path)
			}
			rejected, rejectReason = bundle.Rejected, bundle.RejectReason
			sourcePatch = bundle.SourcePatchSHA256 != ""
		}
	}
	sort.Strings(effects)
	if len(effects) > 60 {
		effects = append(effects[:60], fmt.Sprintf("... and %d more", len(effects)-60))
	}
	state, err := json.Marshal(map[string]any{
		"objective":              strings.TrimSpace(objective),
		"implementation_reports": summaries,
		"file_effects":           effects,
		"carries_source_patch":   sourcePatch,
		"freeze_rejected":        rejected,
		"freeze_reject_reason":   rejectReason,
	})
	if err != nil {
		return nil, nil, err
	}
	return state, evidenceRefs, nil
}

// judgeFrozenCandidate asks the decision model about a frozen candidate and
// records its verdict as the operation's verification. A transport failure
// returns an error so the reconcile retries; it never fails the operation.
func (rt *Runtime) judgeFrozenCandidate(ctx context.Context, doc types.Document, operation selfdev.Operation, objective string, implementation types.EngineeringAssignment) error {
	if rt.candidateJudge == nil {
		return fmt.Errorf("candidate judgment: no decision model is bound")
	}
	state, evidenceRefs, err := rt.candidateJudgmentState(ctx, operation, objective, implementation)
	if err != nil {
		return err
	}
	questions := make(map[string]json.RawMessage, len(candidateJudgmentQuestions))
	for name, question := range candidateJudgmentQuestions {
		if questions[name], err = json.Marshal(question); err != nil {
			return err
		}
	}
	raw, err := rt.candidateJudge.Judge(ctx, state, questions)
	if err != nil {
		return fmt.Errorf("candidate judgment: %w", err)
	}
	judgment, err := parseCandidateJudgment(raw)
	if err != nil {
		return err
	}
	toolCtx := &CapsuleToolCtx{
		AgentRunID: "decision-model:" + firstNonEmpty(judgment.Model, candidateJudgmentModel), ComputerID: doc.ComputerID, OwnerID: doc.OwnerID,
		EventAppender: rt.eventAppender, OperationStore: rt.selfdevOperations, UpdaterRoot: rt.selfdevUpdaterRoot,
	}
	if rt.store != nil {
		toolCtx.EventProjection = rt.store
		toolCtx.Ledger = rt.store
	}
	_, err = recordSelfDevelopmentVerdict(ctx, toolCtx, operation.TrajectoryID, operation.OperationID, operation.BundleDigest, judgment.Decision, evidenceRefs,
		map[string]any{"judgment": judgment, "judged_state": json.RawMessage(state)})
	return err
}
