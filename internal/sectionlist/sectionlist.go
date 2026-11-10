// Package sectionlist is the Autopaper section list as data: a versioned
// document of yes/no questions, each with a threshold, asked of Jev about a
// story (docs/design/sections-as-questions-2026-11-10.md). A section is a
// question; a story belongs to every section whose answer's probability of
// yes reaches the section's threshold, so it can be in several. The host
// scorer asks the questions and stores the probabilities; corpusd applies
// the thresholds at read time, so a threshold change re-sorts without
// asking again.
package sectionlist

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

//go:embed house.json
var houseJSON []byte

// MaxSections bounds a list so one story's section questions, with the
// importance and promo questions, fit one Jev call (System1MaxQuestions 12).
const MaxSections = 10

// Section is one question of a list.
type Section struct {
	ID        string  `json:"id"`
	Label     string  `json:"label"`
	Question  string  `json:"question"`
	Threshold float64 `json:"threshold"`
}

// List is a section list document.
type List struct {
	Version  int       `json:"version"`
	ID       string    `json:"id"`
	Sections []Section `json:"sections"`
}

var idPattern = regexp.MustCompile(`^[a-z0-9_]{1,32}$`)

// House returns the house list. It panics when the embedded document is
// invalid, which a test rules out.
func House() List {
	l, err := Parse(houseJSON)
	if err != nil {
		panic("sectionlist: house list: " + err.Error())
	}
	return l
}

// Parse decodes and validates a list document.
func Parse(raw []byte) (List, error) {
	var l List
	if err := json.Unmarshal(raw, &l); err != nil {
		return List{}, fmt.Errorf("decode: %w", err)
	}
	return l, l.Validate()
}

// Validate refuses a list the scorer or reader could not use.
func (l List) Validate() error {
	if l.Version < 1 {
		return fmt.Errorf("version %d", l.Version)
	}
	if len(l.Sections) == 0 || len(l.Sections) > MaxSections {
		return fmt.Errorf("%d sections, want 1..%d", len(l.Sections), MaxSections)
	}
	ids, hashes := map[string]bool{}, map[string]bool{}
	for _, s := range l.Sections {
		if !idPattern.MatchString(s.ID) || s.ID == "front" || s.ID == "opinion" {
			return fmt.Errorf("section id %q", s.ID)
		}
		if strings.TrimSpace(s.Label) == "" || Normalize(s.Question) == "" {
			return fmt.Errorf("section %q needs a label and a question", s.ID)
		}
		if !(s.Threshold > 0 && s.Threshold <= 1) {
			return fmt.Errorf("section %q threshold %v, want (0,1]", s.ID, s.Threshold)
		}
		h := Hash(s.Question)
		if ids[s.ID] || hashes[h] {
			return fmt.Errorf("section %q repeats an id or a question", s.ID)
		}
		ids[s.ID], hashes[h] = true, true
	}
	return nil
}

// Normalize is the question text a cache key is taken over: whitespace
// collapsed and trimmed.
func Normalize(q string) string { return strings.Join(strings.Fields(q), " ") }

// Hash is the hex SHA-256 of the normalized question text. It is the
// section's cache identity: a renamed section keeps its answers, an edited
// question does not.
func Hash(q string) string {
	sum := sha256.Sum256([]byte(Normalize(q)))
	return hex.EncodeToString(sum[:])
}

// Entry is a section as the scores file publishes it to corpusd.
type Entry struct {
	ID        string  `json:"id"`
	Label     string  `json:"label"`
	Hash      string  `json:"hash"`
	Threshold float64 `json:"threshold"`
}

// Published is the list as the host scorer writes it into the scores file;
// corpusd reads it, never a guest.
type Published struct {
	Version int     `json:"version"`
	ListID  string  `json:"list_id,omitempty"`
	Entries []Entry `json:"entries"`
}

// Published returns the list's publishable form.
func (l List) Published() Published {
	p := Published{Version: l.Version, ListID: l.ID}
	for _, s := range l.Sections {
		p.Entries = append(p.Entries, Entry{ID: s.ID, Label: strings.TrimSpace(s.Label), Hash: Hash(s.Question), Threshold: s.Threshold})
	}
	return p
}

// Valid reports whether a published list is usable (a corpusd guard against
// a corrupt file).
func (p Published) Valid() bool {
	if len(p.Entries) == 0 || len(p.Entries) > MaxSections {
		return false
	}
	seen := map[string]bool{}
	for _, e := range p.Entries {
		if !idPattern.MatchString(e.ID) || e.Hash == "" || !(e.Threshold > 0 && e.Threshold <= 1) || seen[e.ID] {
			return false
		}
		seen[e.ID] = true
	}
	return true
}

// Has reports whether id is a section of the list.
func (p Published) Has(id string) bool {
	for _, e := range p.Entries {
		if e.ID == id {
			return true
		}
	}
	return false
}

// Placement is where a story sits.
type Placement struct {
	// Sections are every section whose probability reaches its threshold,
	// in list order; when none does, the one highest-probability section
	// (Fallback true), so a story is never in no section.
	Sections []string
	// Primary is the member with the highest probability (ties go to the
	// earlier section).
	Primary  string
	Fallback bool
}

// Place applies the thresholds to a story's stored probabilities of yes,
// keyed by question hash. A section with no stored probability is not a
// member; a story with none stored is unplaced (zero Placement).
func (p Published) Place(probs map[string]float64) Placement {
	var out Placement
	bestMember, bestAny := -1.0, -1.0
	var anyID string
	for _, e := range p.Entries {
		pr, ok := probs[e.Hash]
		if !ok || pr != pr { // missing or NaN
			continue
		}
		if pr > bestAny {
			bestAny, anyID = pr, e.ID
		}
		if pr >= e.Threshold {
			out.Sections = append(out.Sections, e.ID)
			if pr > bestMember {
				bestMember, out.Primary = pr, e.ID
			}
		}
	}
	if len(out.Sections) == 0 && anyID != "" {
		out.Sections, out.Primary, out.Fallback = []string{anyID}, anyID, true
	}
	return out
}
