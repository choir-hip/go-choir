package entitytext

import (
	"math"
	"unicode/utf8"
)

// trieNode is a node of a token trie: a key is a path of word keys, and the
// node at its end holds the ids filed under it.
type trieNode struct {
	next map[string]*trieNode
	ids  map[string]bool
}

func newTrieNode() *trieNode { return &trieNode{next: map[string]*trieNode{}} }

// insert files id under the key's words and reports whether the key had no
// id before.
func (n *trieNode) insert(words []string, id string) bool {
	for _, w := range words {
		c := n.next[w]
		if c == nil {
			c = newTrieNode()
			n.next[w] = c
		}
		n = c
	}
	if n.ids == nil {
		n.ids = map[string]bool{}
	}
	fresh := len(n.ids) == 0
	n.ids[id] = true
	return fresh
}

// remove drops id from the key and reports whether the key has no id left
// (and had one). Nodes stay; a removed key costs only its path.
func (n *trieNode) remove(words []string, id string) bool {
	for _, w := range words {
		n = n.next[w]
		if n == nil {
			return false
		}
	}
	if !n.ids[id] {
		return false
	}
	delete(n.ids, id)
	return len(n.ids) == 0
}

// span is one match: tokens [start, end) end at node.
type span struct {
	start, end int
	node       *trieNode
}

// scan matches the trie over toks in one left-to-right pass, longest match
// first, across joined tokens only (a comma or a sentence break ends a
// name). A one-word match whose key is under four letters matches only an
// acronym token ("FBI", never "fed" or "Fed"); a closed word never matches
// alone.
func scan(root *trieNode, toks []Token, emit func(span)) {
	for i := 0; i < len(toks); {
		n := root
		best := -1
		var bestNode *trieNode
		for j := i; j < len(toks); j++ {
			if j > i && !toks[j].joined {
				break
			}
			n = n.next[toks[j].Key]
			if n == nil {
				break
			}
			if len(n.ids) == 0 {
				continue
			}
			if j == i && !singleWordMatches(toks[i]) {
				continue
			}
			best, bestNode = j, n
		}
		if best < 0 {
			i++
			continue
		}
		emit(span{start: i, end: best + 1, node: bestNode})
		i = best + 1
	}
}

func singleWordMatches(t Token) bool {
	if closed(t) {
		return false
	}
	return t.Acronym || utf8.RuneCountInString(t.Key) >= 4
}

// Stats are a name's occurrences in one story and its score.
type Stats struct {
	// Count is the number of body occurrences.
	Count int
	// Sentences is the number of body sentences it occurs in (its spread).
	Sentences int
	// InTitle: it occurs in the title. InLead: in the first two sentences.
	InTitle, InLead bool
	// MidCapital: at least one occurrence is capitalised mid-sentence, or an
	// acronym.
	MidCapital bool
	// Rarity is the mean inverse document frequency of its words in the
	// computer's library (1 when unknown).
	Rarity float64
	// Score ranks names after YAKE: casing, position, frequency and spread,
	// weighted by rarity. Higher is better.
	Score float64

	sentenceSet map[int]bool
	capital     bool // any capitalised body occurrence
}

func (s *Stats) addBody(toks []Token, sp span) {
	s.Count++
	if s.sentenceSet == nil {
		s.sentenceSet = map[int]bool{}
	}
	sent := toks[sp.start].Sentence
	s.sentenceSet[sent] = true
	if sent < leadSentences {
		s.InLead = true
	}
	for k := sp.start; k < sp.end; k++ {
		t := toks[k]
		if t.Capital {
			s.capital = true
		}
		if t.Acronym || (t.Capital && !t.SentenceStart) {
			s.MidCapital = true
		}
	}
}

func (s *Stats) merge(o *Stats) {
	s.Count += o.Count
	if s.sentenceSet == nil {
		s.sentenceSet = map[int]bool{}
	}
	for k := range o.sentenceSet {
		s.sentenceSet[k] = true
	}
	s.InTitle = s.InTitle || o.InTitle
	s.InLead = s.InLead || o.InLead
	s.MidCapital = s.MidCapital || o.MidCapital
	s.capital = s.capital || o.capital
}

// finish computes the spread, rarity and score. idf gives a word key's
// inverse document frequency in the computer's library; nil means unknown.
func (s *Stats) finish(words []string, idf func(string) float64) {
	s.Sentences = len(s.sentenceSet)
	s.Rarity = 1
	if idf != nil {
		sum, n := 0.0, 0
		for _, w := range words {
			if closedWord(w) {
				continue
			}
			sum += idf(w)
			n++
		}
		if n > 0 {
			s.Rarity = sum / float64(n)
		}
	}
	score := 1 + math.Log1p(float64(s.Count)) + math.Log1p(float64(s.Sentences))
	if s.MidCapital {
		score++
	}
	if s.InLead {
		score += 0.5
	}
	s.Score = score * s.Rarity
}
