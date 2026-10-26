package yaegikernel

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// choir.System1 is the research desk's closed-question verb
// (docs/problems/related-stories-thin-no-subject-nodes-2026-10-20.md). System1
// is the fast intuitive judge: it returns a distribution over the choices it
// is given. The desk's own reading and reasoning is System 2, and decides. The
// verb never names the backing model or transport (Jev today), so the judge
// behind it can change. The host re-validates everything here: this file is
// the one definition of a well-formed question, shared by the cell-side verb
// (fast refusal) and the host (authority).

// Bounds on one call. The context a call may carry is resolved by the host
// from references, never taken from the cell.
const (
	System1MaxQuestions       = 12
	System1MinChoices         = 2
	System1MaxChoices         = 6
	System1MaxTextRunes       = 400
	System1MaxChoiceRunes     = 80
	System1MaxRefsPerQuestion = 4
	System1MaxDistinctRefs    = 16
	system1MaxIDLen           = 40
	system1MaxRefLen          = 512
)

var system1IDRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)

// System1Question is one closed question: choose among Choices, judged only on
// the text the host resolves from ContextRefs (capture ids or story:<revision
// id>). Text is the desk's wording of the question, not evidence.
type System1Question struct {
	ID          string   `json:"id"`
	Text        string   `json:"text"`
	Choices     []string `json:"choices"`
	ContextRefs []string `json:"context_refs,omitempty"`
}

// System1Payload defines ActionSystem1.
type System1Payload struct {
	Questions []System1Question `json:"questions"`
}

// ValidateSystem1Questions checks a question list against the bounds and
// returns it normalized (fields trimmed). It names the first offending field.
func ValidateSystem1Questions(in []System1Question) ([]System1Question, error) {
	return validateSystem1Questions(in, System1MaxChoices)
}

// System1HostMaxChoices bounds the choices of a question the host asks
// itself: the story analysis's type question has the six entity types, "not
// an entity" and "none of these fit" (slice c0 §3).
const System1HostMaxChoices = 8

// ValidateSystem1HostQuestions checks questions the host builds, with the
// host's choice bound; every other bound is the cell's.
func ValidateSystem1HostQuestions(in []System1Question) ([]System1Question, error) {
	return validateSystem1Questions(in, System1HostMaxChoices)
}

func validateSystem1Questions(in []System1Question, maxChoices int) ([]System1Question, error) {
	if len(in) == 0 {
		return nil, fmt.Errorf("choir.System1 needs at least one question")
	}
	if len(in) > System1MaxQuestions {
		return nil, fmt.Errorf("choir.System1 takes at most %d questions per call, got %d", System1MaxQuestions, len(in))
	}
	out := make([]System1Question, 0, len(in))
	seenID := map[string]bool{}
	distinct := map[string]bool{}
	for i, q := range in {
		id := strings.TrimSpace(q.ID)
		if id == "" || len(id) > system1MaxIDLen || !system1IDRE.MatchString(id) {
			return nil, fmt.Errorf("question %d: id %q must be 1-%d letters, digits, '_', '.' or '-'", i+1, q.ID, system1MaxIDLen)
		}
		if seenID[id] {
			return nil, fmt.Errorf("question %d: duplicate id %q", i+1, id)
		}
		seenID[id] = true
		text := strings.TrimSpace(q.Text)
		if n := utf8.RuneCountInString(text); n == 0 || n > System1MaxTextRunes {
			return nil, fmt.Errorf("question %q: text must be 1-%d characters (it is your wording of the question; evidence goes in context_refs), got %d", id, System1MaxTextRunes, n)
		}
		if n := len(q.Choices); n < System1MinChoices || n > maxChoices {
			return nil, fmt.Errorf("question %q: choices must number %d-%d, got %d", id, System1MinChoices, maxChoices, n)
		}
		choices := make([]string, 0, len(q.Choices))
		seenChoice := map[string]bool{}
		for _, c := range q.Choices {
			c = strings.TrimSpace(c)
			if n := utf8.RuneCountInString(c); n == 0 || n > System1MaxChoiceRunes {
				return nil, fmt.Errorf("question %q: each choice must be 1-%d characters", id, System1MaxChoiceRunes)
			}
			if seenChoice[c] {
				return nil, fmt.Errorf("question %q: duplicate choice %q", id, c)
			}
			seenChoice[c] = true
			choices = append(choices, c)
		}
		if len(q.ContextRefs) > System1MaxRefsPerQuestion {
			return nil, fmt.Errorf("question %q: context_refs takes at most %d references, got %d", id, System1MaxRefsPerQuestion, len(q.ContextRefs))
		}
		var refs []string
		seenRef := map[string]bool{}
		for _, r := range q.ContextRefs {
			r = strings.TrimSpace(r)
			if r == "" || len(r) > system1MaxRefLen {
				return nil, fmt.Errorf("question %q: context_refs entries must be capture ids or story:<revision_id>, 1-%d characters", id, system1MaxRefLen)
			}
			if seenRef[r] {
				continue
			}
			seenRef[r] = true
			distinct[r] = true
			refs = append(refs, r)
		}
		out = append(out, System1Question{ID: id, Text: text, Choices: choices, ContextRefs: refs})
	}
	if len(distinct) > System1MaxDistinctRefs {
		return nil, fmt.Errorf("choir.System1 takes at most %d distinct context_refs per call, got %d", System1MaxDistinctRefs, len(distinct))
	}
	return out, nil
}

// system1Questions reads the cell's argument: a list of {id, text, choices,
// context_refs} maps (or the typed list). Unknown fields are refused, so text
// offered as evidence under another name is not silently dropped.
func system1Questions(arg any) ([]System1Question, error) {
	if typed, ok := arg.([]System1Question); ok {
		return ValidateSystem1Questions(typed)
	}
	raw, err := jsonCellArg(arg)
	if err != nil {
		return nil, err
	}
	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(raw, "[") {
		return nil, fmt.Errorf("choir.System1 takes a list of {id, text, choices, context_refs} maps")
	}
	dec := json.NewDecoder(bytes.NewReader([]byte(raw)))
	dec.DisallowUnknownFields()
	var qs []System1Question
	if err := dec.Decode(&qs); err != nil {
		return nil, fmt.Errorf("choir.System1 takes a list of {id, text, choices, context_refs} maps; evidence is named by context_refs (capture ids, story:<revision_id>), never pasted: %w", err)
	}
	return ValidateSystem1Questions(qs)
}

// System1 asks the judge closed questions and returns, per question, the
// probability of each choice as the judge gave it (no threshold, no pick):
// {receipt_ref, replayed, answers: [{id, choices, probabilities}]}.
// Read the distributions and decide yourself; a flat one means read the
// sources closely. context_refs name captures and story versions
// (story:<revision_id>) whose text the host reads itself.
func (s *ChoirScope) System1(questions any) (JSON, error) {
	qs, err := system1Questions(questions)
	if err != nil {
		return nil, err
	}
	var out JSON
	err = s.call(ActionSystem1, System1Payload{Questions: qs}, &out)
	return out, err
}
