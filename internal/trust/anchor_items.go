package trust

import "time"

// The anchored sample as it is stored and served (MA7.10). One AnchorItem is
// one spot-checked item: every scorer's answer on it, and the truth the
// strongest judge settled. It carries ids, names, answers and the truth; never
// the item's text, the judge's prompt or the sampler's seed.

// ScorerAnswer is one scorer's answer on an item.
type ScorerAnswer struct {
	Scorer string             `json:"scorer"`
	Family string             `json:"family,omitempty"`
	Answer string             `json:"answer"`
	Probs  map[string]float64 `json:"probs,omitempty"`
}

// AnchorItem is a settled, anchored item.
type AnchorItem struct {
	ComputerID   string         `json:"computer_id,omitempty"`
	ItemID       string         `json:"item_id"`
	Agent        string         `json:"agent"`
	QuestionType string         `json:"question_type"`
	Choices      []string       `json:"choices"`
	Answers      []ScorerAnswer `json:"answers"`
	Truth        string         `json:"truth"`
	Kind         AnchorKind     `json:"kind"`
	Judges       []string       `json:"judges,omitempty"`
	Agreement    float64        `json:"agreement"`
	SettledAt    time.Time      `json:"settled_at"`
	// The sampler's log entry for the draw that chose the item.
	Draw      float64   `json:"draw"`
	Rate      float64   `json:"rate"`
	SeedID    string    `json:"seed_id"`
	SampledAt time.Time `json:"sampled_at"`
}

// CorpusRow is one settled item of the corpus the cheap labels cover, whether
// or not it was anchored: the agent's answer, the System 1 judge's answer,
// and, when anchored, the truth. PPI reads the cheap agreement on every row
// and the truth on the anchored ones.
type CorpusRow struct {
	Agent        string `json:"agent"`
	QuestionType string `json:"question_type"`
	ItemID       string `json:"item_id"`
	AgentAnswer  string `json:"agent_answer"`
	JudgeAnswer  string `json:"judge_answer"`
	Anchored     bool   `json:"anchored,omitempty"`
	Truth        string `json:"truth,omitempty"`
}

// AnchorSample is what the owner's API serves to the harness.
type AnchorSample struct {
	Items  []AnchorItem `json:"items"`
	Corpus []CorpusRow  `json:"corpus"`
	// Decisions is how many items the sampler has drawn on, Sampled how many
	// it chose, Unsettled how many chosen items the strongest judge did not
	// settle: a final split or out-of-set answer, or an item given up after
	// three deferrals (they stay unknown).
	Decisions int     `json:"decisions"`
	Sampled   int     `json:"sampled"`
	Unsettled int     `json:"unsettled"`
	Rate      float64 `json:"rate"`
	// Unreplayable is the decisions in logs set aside because they were
	// written under another seed or rate. LogNote says why the log could not
	// be read, when it could not.
	Unreplayable int `json:"unreplayable,omitempty"`
	// GaveUp is the sampled items whose deferrals reached the bound: decided
	// and unknown, counted in Unsettled.
	GaveUp  int    `json:"gave_up,omitempty"`
	LogNote string `json:"log_note,omitempty"`
}

// Anchor is the item's settled truth. ok is false for an item with none.
func (it AnchorItem) Anchor() (Anchor, bool) {
	if it.Truth == "" {
		return Anchor{}, false
	}
	return Anchor{ItemID: it.ItemID, Kind: it.Kind, Truth: it.Truth, Judges: it.Judges, Agreement: it.Agreement, SettledAt: it.SettledAt}, true
}

// Observations reads the sample as scorer observations and anchors, the
// inputs of ScorerProfiles and ErrorCorrelation. An item with no truth is
// left out.
func (s AnchorSample) Observations() ([]ScorerObservation, map[string]Anchor) {
	anchors := map[string]Anchor{}
	var obs []ScorerObservation
	for _, it := range s.Items {
		a, ok := it.Anchor()
		if !ok {
			continue
		}
		anchors[it.ItemID] = a
		for _, sa := range it.Answers {
			obs = append(obs, ScorerObservation{Scorer: sa.Scorer, Family: sa.Family, QuestionType: it.QuestionType, ItemID: it.ItemID,
				Answers: it.Choices, Probs: sa.Probs})
		}
	}
	return obs, anchors
}
