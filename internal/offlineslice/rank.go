package offlineslice

import (
	"sort"
	"strings"
)

// StoryCand is a story offered as a subject.
type StoryCand struct {
	StoryID string  `json:"story_id"`
	Title   string  `json:"title"`
	Dek     string  `json:"dek,omitempty"`
	Cos     float64 `json:"cos"`
}

// FinalCand is one entry of the capped list.
type FinalCand struct {
	Label string `json:"label"`
	Kind  string `json:"kind"` // story | entity | name | story-entity | fuzzy
	Desc  string `json:"desc,omitempty"`
	Tier  int    `json:"tier"`
}

// ListInput is what the ranking reads: the top stories (best first), the
// capture's cleaned mentions, and the central entities of the top stories
// (one slice a story, in story order).
type ListInput struct {
	Stories       []StoryCand
	Mentions      []Mention
	StoryEntities [][]Cand
}

// RankAndCap orders the candidates by a fixed model-free rule and cuts the
// list: (1) stories by similarity; (2) known entities found in the title;
// (3) new names in the title; (4) other known entities found in the text, by
// the extractor's score; (5) the top stories' central entities; (6) other new
// names by score; (7) fuzzy matches. It returns the list of at most max and the
// number of distinct entries before the cut. A mention's first candidate only
// stands for it.
func RankAndCap(in ListInput, max int) ([]FinalCand, int) {
	var all []FinalCand
	seen := map[string]bool{}
	add := func(c FinalCand) {
		k := strings.ToLower(strings.TrimSpace(c.Label))
		if k == "" || seen[k] {
			return
		}
		seen[k] = true
		all = append(all, c)
	}
	for _, s := range in.Stories {
		add(FinalCand{Label: s.Title, Kind: "story", Desc: s.Dek, Tier: 1})
	}
	byScore := func(ms []Mention) []Mention {
		out := append([]Mention(nil), ms...)
		sort.SliceStable(out, func(i, j int) bool { return out[i].Score > out[j].Score })
		return out
	}
	ms := byScore(in.Mentions)
	for _, m := range ms {
		if m.Hit && len(m.Cands) > 0 && m.InTitle {
			add(FinalCand{Label: m.Cands[0].Name, Kind: "entity", Tier: 2})
		}
	}
	for _, m := range ms {
		if len(m.Cands) == 0 && m.InTitle {
			add(FinalCand{Label: m.Phrase, Kind: "name", Tier: 3})
		}
	}
	for _, m := range ms {
		if m.Hit && len(m.Cands) > 0 {
			add(FinalCand{Label: m.Cands[0].Name, Kind: "entity", Tier: 4})
		}
	}
	for _, es := range in.StoryEntities {
		for _, e := range es {
			add(FinalCand{Label: e.Name, Kind: "story-entity", Tier: 5})
		}
	}
	for _, m := range ms {
		if len(m.Cands) == 0 {
			add(FinalCand{Label: m.Phrase, Kind: "name", Tier: 6})
		}
	}
	for _, m := range ms {
		if !m.Hit && len(m.Cands) > 0 {
			add(FinalCand{Label: m.Cands[0].Name, Kind: "fuzzy", Tier: 7})
		}
	}
	pre := len(all)
	if len(all) > max {
		all = all[:max]
	}
	return all, pre
}
