package yaegikernel

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// choir.RelateStories is the research desk's record of a judged pair relation
// (docs/problems/related-stories-thin-no-subject-nodes-2026-10-20.md, "Build:
// the related_story edge"). The desk has read two published stories and asked
// System 1 which relation they have; this verb records the desk's decision on
// the tape, and the computer exports it to Store B as one related_story edge.
// The probability is never the cell's: the host reads it from the System 1
// answer the pair cites. This file is the one definition of a well-formed pair,
// shared by the cell-side verb (fast refusal) and the host (authority).

// Bounds on one call.
const (
	RelateStoriesMaxPairs       = 12
	RelateStoriesMaxReasonRunes = 400
	// RelateStoriesMinOverrideReasonRunes is the least reason that goes with
	// a relation System 1 did not rank first; the host enforces it.
	RelateStoriesMinOverrideReasonRunes = 20
	relateStoriesMaxRefLen              = 128
)

// RelateStoriesPair is one judged pair: two publications (their canonical ids,
// as the published search returns them in publication_id), the relation the
// desk decided, and the System 1 answer it rests on (the receipt_ref of the
// choir.System1 call and the id of the question in it). Reason is the desk's
// one line; it is required when the relation is not System 1's top choice.
type RelateStoriesPair struct {
	StoryA      string `json:"story_a"`
	StoryB      string `json:"story_b"`
	Relation    string `json:"relation"`
	JudgmentRef string `json:"judgment_ref"`
	QuestionID  string `json:"question_id"`
	Reason      string `json:"reason,omitempty"`
}

// RelateStoriesPayload defines ActionRelateStories.
type RelateStoriesPayload struct {
	Pairs []RelateStoriesPair `json:"pairs"`
}

// ValidateRelateStoriesPairs checks a pair list against the bounds and returns
// it normalized: fields trimmed, the relation lower case, each pair in stored
// order (story_a < story_b), and a pair given twice (in either order, with the
// same relation, receipt and question) kept once. It names the first
// offending field.
func ValidateRelateStoriesPairs(in []RelateStoriesPair) ([]RelateStoriesPair, error) {
	if len(in) == 0 {
		return nil, fmt.Errorf("choir.RelateStories needs at least one pair")
	}
	if len(in) > RelateStoriesMaxPairs {
		return nil, fmt.Errorf("choir.RelateStories takes at most %d pairs per call, got %d", RelateStoriesMaxPairs, len(in))
	}
	out := make([]RelateStoriesPair, 0, len(in))
	seen := map[RelateStoriesPair]bool{}
	for i, p := range in {
		p.StoryA, p.StoryB = strings.TrimSpace(p.StoryA), strings.TrimSpace(p.StoryB)
		p.Relation = strings.ToLower(strings.TrimSpace(p.Relation))
		p.JudgmentRef, p.QuestionID = strings.TrimSpace(p.JudgmentRef), strings.TrimSpace(p.QuestionID)
		p.Reason = strings.TrimSpace(p.Reason)
		for _, id := range []string{p.StoryA, p.StoryB} {
			kind, _, _, err := objectgraph.ParseCanonicalID(id)
			if err != nil || kind != objectgraph.ObjectKind("choir.publication") {
				return nil, fmt.Errorf("pair %d: %q is not a publication id (use the publication_id of a published story from the published search)", i+1, id)
			}
		}
		if p.StoryA == p.StoryB {
			return nil, fmt.Errorf("pair %d: a story cannot be related to itself", i+1)
		}
		switch p.Relation {
		case types.StoryRelationSameIncident, types.StoryRelationLaterDevelopment, types.StoryRelationSameSubject:
		default:
			return nil, fmt.Errorf("pair %d: relation %q must be %s, %s or %s (a pair that is unrelated, or shares only an entity, is not recorded)", i+1, p.Relation,
				types.StoryRelationSameIncident, types.StoryRelationLaterDevelopment, types.StoryRelationSameSubject)
		}
		if !strings.HasPrefix(p.JudgmentRef, "system1:") || len(p.JudgmentRef) == len("system1:") || len(p.JudgmentRef) > relateStoriesMaxRefLen {
			return nil, fmt.Errorf("pair %d: judgment_ref must be the receipt_ref (system1:<id>) of the choir.System1 call this relation rests on", i+1)
		}
		if p.QuestionID == "" || len(p.QuestionID) > system1MaxIDLen || !system1IDRE.MatchString(p.QuestionID) {
			return nil, fmt.Errorf("pair %d: question_id must be the id of the question in that call that decided this pair", i+1)
		}
		if n := utf8.RuneCountInString(p.Reason); n > RelateStoriesMaxReasonRunes {
			return nil, fmt.Errorf("pair %d: reason is at most %d characters, got %d", i+1, RelateStoriesMaxReasonRunes, n)
		}
		if p.StoryA > p.StoryB {
			p.StoryA, p.StoryB = p.StoryB, p.StoryA
		}
		if seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return out, nil
}

// relateStoriesPairs reads the cell's argument: a list of pair maps (or the
// typed list). Unknown fields are refused, so a probability the cell supplies
// is an error, not silently dropped: the host reads it from the judgment.
func relateStoriesPairs(arg any) ([]RelateStoriesPair, error) {
	if typed, ok := arg.([]RelateStoriesPair); ok {
		return ValidateRelateStoriesPairs(typed)
	}
	raw, err := jsonCellArg(arg)
	if err != nil {
		return nil, err
	}
	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(raw, "[") {
		return nil, fmt.Errorf("choir.RelateStories takes a list of {story_a, story_b, relation, judgment_ref, question_id, reason} maps")
	}
	dec := json.NewDecoder(bytes.NewReader([]byte(raw)))
	dec.DisallowUnknownFields()
	var pairs []RelateStoriesPair
	if err := dec.Decode(&pairs); err != nil {
		return nil, fmt.Errorf("choir.RelateStories takes a list of {story_a, story_b, relation, judgment_ref, question_id, reason} maps; the probability is read from the System 1 answer, never supplied: %w", err)
	}
	return ValidateRelateStoriesPairs(pairs)
}

// RelateStories records judged pair relations between published stories:
// {recorded: [{story_a, story_b, relation, probability, top, overrode,
// replayed}], exported, export_error}. Each pair rests on a choir.System1
// answer of this run (receipt_ref and question id) whose choices include the
// relation (spelled with underscores or spaces); the host reads the
// probability from it. A relation System 1 did not rank first needs a reason
// of at least 20 characters. The relation is recorded on the tape, then
// exported to the public graph; if the export fails the record stays and
// repeating the pair exports it again. It may be off on this computer.
func (s *ChoirScope) RelateStories(pairs any) (JSON, error) {
	ps, err := relateStoriesPairs(pairs)
	if err != nil {
		return nil, err
	}
	var out JSON
	err = s.call(ActionRelateStories, RelateStoriesPayload{Pairs: ps}, &out)
	return out, err
}
