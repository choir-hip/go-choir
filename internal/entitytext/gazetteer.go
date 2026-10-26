package entitytext

import (
	"sort"
	"strings"
	"sync"
)

// Gazetteer is the known-entity side of extraction: a token trie over the
// match keys of every entity name and recorded alias, matched in one pass,
// longest match first. It is safe for concurrent use and is updated in place,
// so a mint or an alias is matched by the next request (slice c0 §2, §6).
type Gazetteer struct {
	mu   sync.RWMutex
	root *trieNode
	// byID maps an entity id to the keys filed under it.
	byID map[string]map[string]bool
	keys int
}

// NewGazetteer returns an empty gazetteer.
func NewGazetteer() *Gazetteer {
	return &Gazetteer{root: newTrieNode(), byID: map[string]map[string]bool{}}
}

// Matchable reports whether a name can be a gazetteer key: its key is not
// empty, and a one-word key is not a closed word, unless the name is written
// as an acronym ("US", "IT").
func Matchable(name string) bool {
	key := Key(name)
	if key == "" {
		return false
	}
	if strings.Contains(key, " ") {
		return true
	}
	return !closed(Token{Key: key, Acronym: isAcronym(strings.TrimSpace(name))})
}

// Add files name (an entity's name or alias) under id and reports whether it
// can match at all (Matchable).
func (g *Gazetteer) Add(id, name string) bool {
	if id == "" || !Matchable(name) {
		return false
	}
	key := Key(name)
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.root.insert(strings.Fields(key), id) {
		g.keys++
	}
	if g.byID[id] == nil {
		g.byID[id] = map[string]bool{}
	}
	g.byID[id][key] = true
	return true
}

// Remove drops one name of id (a revoked alias).
func (g *Gazetteer) Remove(id, name string) {
	key := Key(name)
	g.mu.Lock()
	defer g.mu.Unlock()
	g.removeKey(id, key)
}

// RemoveEntity drops every name of id.
func (g *Gazetteer) RemoveEntity(id string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	for key := range g.byID[id] {
		g.removeKey(id, key)
	}
}

func (g *Gazetteer) removeKey(id, key string) {
	if !g.byID[id][key] {
		return
	}
	if g.root.remove(strings.Fields(key), id) {
		g.keys--
	}
	delete(g.byID[id], key)
	if len(g.byID[id]) == 0 {
		delete(g.byID, id)
	}
}

// Size is the number of entities and of distinct keys filed.
func (g *Gazetteer) Size() (entities, keys int) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.byID), g.keys
}

// Hit is a known entity found in a story.
type Hit struct {
	EntityID string
	// Phrase is the entity's most frequent form in the story; Key its key.
	Phrase, Key string
	// Ambiguous: a key matched is filed under more than one entity.
	Ambiguous bool
	Stats
}

// Match finds the gazetteer's entities in a story. An entity is a hit when
// the body has a capitalised occurrence of it (then lower-case ones count
// too) or the title names it. idf is as for Propose. Hits rank by title,
// then score, then id.
func (g *Gazetteer) Match(d *Doc, idf func(string) float64) []Hit {
	type acc struct {
		Hit
		surfaces map[string]int
		keyCount map[string]int
	}
	accs := map[string]*acc{}
	get := func(id string) *acc {
		a := accs[id]
		if a == nil {
			a = &acc{Hit: Hit{EntityID: id}, surfaces: map[string]int{}, keyCount: map[string]int{}}
			accs[id] = a
		}
		return a
	}
	g.mu.RLock()
	scan(g.root, d.Body, func(sp span) {
		var words, keys []string
		for i := sp.start; i < sp.end; i++ {
			words = append(words, d.Body[i].Surface)
			keys = append(keys, d.Body[i].Key)
		}
		for id := range sp.node.ids {
			a := get(id)
			a.addBody(d.Body, sp)
			a.surfaces[strings.Join(words, " ")]++
			a.keyCount[strings.Join(keys, " ")]++
			if len(sp.node.ids) > 1 {
				a.Ambiguous = true
			}
		}
	})
	scan(g.root, d.Title, func(sp span) {
		var keys []string
		for i := sp.start; i < sp.end; i++ {
			keys = append(keys, d.Title[i].Key)
		}
		for id := range sp.node.ids {
			a := get(id)
			a.InTitle = true
			a.keyCount[strings.Join(keys, " ")] += 0
			if len(sp.node.ids) > 1 {
				a.Ambiguous = true
			}
		}
	})
	g.mu.RUnlock()

	out := make([]Hit, 0, len(accs))
	for _, a := range accs {
		if !a.capital && !a.InTitle {
			continue
		}
		if !a.capital {
			// A title-only hit: no body occurrence counts.
			a.Count, a.sentenceSet, a.InLead = 0, nil, false
		}
		a.Phrase = topSurface(a.surfaces)
		a.Key = topSurface(a.keyCount)
		if a.Phrase == "" {
			a.Phrase = a.Key
		}
		a.finish(strings.Fields(a.Key), idf)
		out = append(out, a.Hit)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].InTitle != out[j].InTitle {
			return out[i].InTitle
		}
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].EntityID < out[j].EntityID
	})
	return out
}

// Candidate is a name to rank for System 1's questions: a hit or a
// proposal, with its title flag, score and whether it is a hub (central to
// many stories).
type Candidate struct {
	Key     string
	InTitle bool
	Hub     bool
	Score   float64
}

// Rank orders candidates by title, then score, then key, and keeps at most
// max of them, of which at most hubSlots are hubs (slice c0 §1, G5): hubs
// neither crowd out the rest nor vanish.
func Rank(cs []Candidate, max, hubSlots int) []Candidate {
	sorted := append([]Candidate(nil), cs...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].InTitle != sorted[j].InTitle {
			return sorted[i].InTitle
		}
		if sorted[i].Score != sorted[j].Score {
			return sorted[i].Score > sorted[j].Score
		}
		return sorted[i].Key < sorted[j].Key
	})
	out := make([]Candidate, 0, max)
	hubs := 0
	for _, c := range sorted {
		if len(out) == max {
			break
		}
		if c.Hub {
			if hubs == hubSlots {
				continue
			}
			hubs++
		}
		out = append(out, c)
	}
	return out
}
