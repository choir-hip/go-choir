package agentcore

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

// Failure modes pinned for the decision-model judgment of a frozen candidate:
//   - a "no" on any question passes;
//   - an unanswered or malformed question passes;
//   - a "yes" with less than even probability passes;
//   - a distribution with no model identity is recorded;
//   - all three yes answers fail.
func TestParseCandidateJudgmentFailsClosed(t *testing.T) {
	answer := func(choice string, yes float64) map[string]any {
		return map[string]any{"type": "choice", "choice": choice, "probabilities": map[string]float64{"yes": yes, "no": 1 - yes}, "confidence": 0.3}
	}
	dist := func(model string, answers map[string]any) json.RawMessage {
		raw, _ := json.Marshal(map[string]any{"model": model, "answers": answers})
		return raw
	}
	allYes := map[string]any{"does_what_was_asked": answer("yes", 0.8), "limited_to_the_objective": answer("yes", 0.7), "reversible": answer("yes", 0.9)}
	if j, err := parseCandidateJudgment(dist("typesafe/jev-1.13-20260917", allYes)); err != nil || j.Decision != "pass" || j.Model == "" {
		t.Fatalf("all yes = %+v, %v; want pass", j, err)
	}
	for name, answers := range map[string]map[string]any{
		"one no":     {"does_what_was_asked": answer("yes", 0.8), "limited_to_the_objective": answer("no", 0.3), "reversible": answer("yes", 0.9)},
		"unanswered": {"does_what_was_asked": answer("yes", 0.8), "reversible": answer("yes", 0.9)},
		"weak yes":   {"does_what_was_asked": answer("yes", 0.4), "limited_to_the_objective": answer("yes", 0.7), "reversible": answer("yes", 0.9)},
	} {
		if j, err := parseCandidateJudgment(dist("typesafe/jev-1.13-20260917", answers)); err != nil || j.Decision != "fail" {
			t.Errorf("%s = %+v, %v; want fail", name, j, err)
		}
	}
	if _, err := parseCandidateJudgment(dist("", allYes)); err == nil {
		t.Error("a distribution with no model was accepted")
	}
	if _, err := parseCandidateJudgment(json.RawMessage(`not json`)); err == nil {
		t.Error("a malformed distribution was accepted")
	}
}

type recordingCandidateJudge struct {
	state     map[string]any
	questions []string
	reply     json.RawMessage
	err       error
}

func (j *recordingCandidateJudge) Judge(_ context.Context, state json.RawMessage, questions map[string]json.RawMessage) (json.RawMessage, error) {
	_ = json.Unmarshal(state, &j.state)
	for name := range questions {
		j.questions = append(j.questions, name)
	}
	return j.reply, j.err
}

// Rerun 14: the verdict is recorded in an event payload, which is canonical
// JSON and refuses floats ("non-integral number 0.82 is forbidden"). Failure
// modes pinned: the recorded judgment carries a float; the basis points round
// the wrong way or lose the decision.
func TestCandidateJudgmentRecordsAsCanonicalJSON(t *testing.T) {
	raw := json.RawMessage(`{"model":"typesafe/jev-1.13","answers":{
		"does_what_was_asked":{"type":"choice","choice":"yes","probabilities":{"yes":0.82,"no":0.18},"confidence":0.6449},
		"limited_to_the_objective":{"type":"choice","choice":"yes","probabilities":{"yes":0.71,"no":0.29},"confidence":0.42},
		"reversible":{"type":"choice","choice":"yes","probabilities":{"yes":0.93,"no":0.07},"confidence":0.86}}}`)
	judgment, err := parseCandidateJudgment(raw)
	if err != nil || judgment.Decision != "pass" {
		t.Fatalf("judgment = %+v, %v", judgment, err)
	}
	if _, err := computerevent.CanonicalJSON(map[string]any{"judgment": judgment}); err != nil {
		t.Fatalf("recorded judgment is not canonical JSON: %v", err)
	}
	answer := judgment.Answers["does_what_was_asked"]
	if answer.YesBasisPoints != 8200 || answer.ConfidenceBasisPoints != 6449 || answer.BasisPoints["no"] != 1800 {
		t.Fatalf("basis points = %+v", answer)
	}
}
