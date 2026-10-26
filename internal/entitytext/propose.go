package entitytext

import (
	"sort"
	"strings"
	"unicode"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// MaxProposals bounds the phrases a story proposes.
const MaxProposals = 48

// maxSpanWords bounds a proposed name's length in words other than
// connectors, and maxSpanTokens in all words.
const (
	maxSpanWords  = 4
	maxSpanTokens = 7
)

// Proposal is a candidate name a story proposes: not yet an entity, for
// System 1 to type and research to judge.
type Proposal struct {
	// Phrase is the name's most frequent capitalised form in the story.
	Phrase string
	// Key is its match key (the words' keys; equal to Key(Phrase)).
	Key string
	// Acronym: a one-word name written in capitals.
	Acronym bool
	// Aliases are shorter forms merged into this name in the story
	// ("Zelenskyy" into "Volodymyr Zelenskyy"): alias candidates.
	Aliases []string
	Stats
}

// companySuffixes end no proposed name, as types.NormalizeEntityName drops
// them from an entity's name.
var companySuffixes = set("inc", "incorporated", "corp", "corporation", "co", "ltd", "llc", "plc", "pbc", "ag", "sa", "gmbh")

// Propose finds candidate names in a story's body (slice c0 §1): capitalised
// spans of one to four words that a sentence-start capital alone does not
// explain, and acronyms; it counts every occurrence of each, in any case,
// merges a short form into the one longest name it begins or ends, and ranks
// by title, then score. idf is the computer's library's inverse document
// frequency per word key (nil when unknown).
func Propose(d *Doc, idf func(string) float64) []Proposal {
	body := d.Body
	pieces := spansOf(body)

	confirmed := map[string]bool{}
	startSingles := map[string]int{}
	for _, p := range pieces {
		if !body[p[0]].SentenceStart || body[p[0]].Acronym {
			confirmed[keyOf(body, p)] = true
		} else if p[1]-p[0] == 1 {
			startSingles[body[p[0]].Key]++
		}
	}
	inTitle := map[string]bool{}
	for _, t := range d.Title {
		inTitle[t.Key] = true
	}
	keys := map[string][]string{}
	add := func(p [2]int) {
		if p[1]-p[0] == 1 && !singleWordMatches(body[p[0]]) {
			return
		}
		k := keyOf(body, p)
		if _, ok := keys[k]; !ok {
			keys[k] = strings.Fields(k)
		}
	}
	for _, p := range pieces {
		first := body[p[0]]
		if !first.SentenceStart || first.Acronym || confirmed[keyOf(body, p)] {
			add(p)
			continue
		}
		if p[1]-p[0] == 1 {
			// A sentence's first word alone says nothing, unless it starts
			// sentences again, carries a possessive, or is in the title.
			if k := first.Key; startSingles[k] >= 2 || first.possessive || inTitle[k] {
				add(p)
			}
			continue
		}
		if connectors[body[p[0]+1].Key] {
			// "Leaders of NATO": the opener is not part of the name.
			if q, ok := trim(body, [2]int{p[0] + 1, p[1]}); ok {
				add(q)
			}
			continue
		}
		add(p)
	}
	if len(keys) == 0 {
		return nil
	}

	root := newTrieNode()
	for k, words := range keys {
		root.insert(words, k)
	}
	type acc struct {
		Proposal
		order     int
		surfaces  map[string]int
		alive     bool
		mixedCase bool // a one-word name also written other than in capitals
	}
	accs := map[string]*acc{}
	order := 0
	scan(root, body, func(sp span) {
		for k := range sp.node.ids {
			a := accs[k]
			if a == nil {
				a = &acc{Proposal: Proposal{Key: k}, order: order, surfaces: map[string]int{}, alive: true}
				order++
				accs[k] = a
			}
			a.addBody(body, sp)
			capital := false
			var words []string
			for i := sp.start; i < sp.end; i++ {
				words = append(words, body[i].Surface)
				if body[i].Capital {
					capital = true
				}
			}
			if capital {
				a.surfaces[strings.Join(words, " ")]++
				if sp.end-sp.start == 1 && !body[sp.start].Acronym {
					a.mixedCase = true
				}
			}
		}
	})
	scan(root, d.Title, func(sp span) {
		for k := range sp.node.ids {
			if a := accs[k]; a != nil {
				a.InTitle = true
			}
		}
	})
	list := make([]*acc, 0, len(accs))
	for _, a := range accs {
		if !a.capital {
			a.alive = false
			continue
		}
		a.Phrase = topSurface(a.surfaces)
		a.Acronym = len(keys[a.Key]) == 1 && !a.mixedCase
		list = append(list, a)
	}

	// Merge short forms into the one longest name, longest first so a chain
	// lands on the longest form.
	sort.Slice(list, func(i, j int) bool {
		li, lj := len(keys[list[i].Key]), len(keys[list[j].Key])
		if li != lj {
			return li > lj
		}
		return list[i].Key < list[j].Key
	})
	for _, s := range list {
		sw := keys[s.Key]
		var target *acc
		n := 0
		for _, l := range list {
			if !l.alive || l == s || len(keys[l.Key]) <= len(sw) {
				continue
			}
			if affix(keys[l.Key], sw) {
				target = l
				n++
			}
		}
		if n != 1 {
			continue
		}
		target.merge(&s.Stats)
		target.Aliases = append(target.Aliases, s.Phrase)
		target.Aliases = append(target.Aliases, s.Aliases...)
		s.alive = false
	}

	out := make([]Proposal, 0, len(list))
	for _, a := range list {
		if !a.alive {
			continue
		}
		a.finish(keys[a.Key], idf)
		out = append(out, a.Proposal)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].InTitle != out[j].InTitle {
			return out[i].InTitle
		}
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].Key < out[j].Key
	})
	if len(out) > MaxProposals {
		out = out[:MaxProposals]
	}
	return out
}

// spansOf returns the candidate spans of a body: runs of joined capitalised
// words, acronyms and inner connectors, split at closed words, trimmed of
// connectors and company forms at the ends, at most maxSpanWords long.
func spansOf(toks []Token) [][2]int {
	var out [][2]int
	emit := func(s, e int) {
		if p, ok := trim(toks, [2]int{s, e}); ok {
			out = append(out, p)
		}
	}
	start := -1
	for i := 0; i <= len(toks); i++ {
		in := i < len(toks) && eligible(toks, i) && (start < 0 || toks[i].joined)
		if in && start < 0 {
			start = i
			continue
		}
		if in {
			continue
		}
		if start >= 0 {
			// Split the run at closed words (a connector, and "the" after
			// one, stay: "Department of the Treasury").
			s := start
			for k := start; k < i; k++ {
				if splitter(toks, k, start, i) {
					emit(s, k)
					s = k + 1
				}
			}
			emit(s, i)
			start = -1
		}
		if i < len(toks) && eligible(toks, i) {
			start = i
		}
	}
	return out
}

func eligible(toks []Token, i int) bool {
	t := toks[i]
	return t.Capital || t.Acronym || connectors[t.Key] || (t.Key == "the" && i > 0 && connectors[toks[i-1].Key])
}

func splitter(toks []Token, k, start, end int) bool {
	t := toks[k]
	if !closed(t) || connectors[t.Key] {
		return false
	}
	return !(t.Key == "the" && k > start && connectors[toks[k-1].Key] && k+1 < end)
}

// trim drops connectors, "the" and company forms from a span's ends and
// refuses one that is empty or too long.
func trim(toks []Token, p [2]int) ([2]int, bool) {
	s, e := p[0], p[1]
	// A capitalised connector belongs to the name ("El-Masri", "Van Dijk");
	// a lower-case one never starts or ends one.
	inner := func(t Token) bool { return connectors[t.Key] && t.Capital }
	for s < e && !inner(toks[s]) && (connectors[toks[s].Key] || toks[s].Key == "the" || closed(toks[s])) {
		s++
	}
	for e > s && (connectors[toks[e-1].Key] || toks[e-1].Key == "the" || closed(toks[e-1]) || (e-1 > s && companySuffixes[toks[e-1].Key])) {
		e--
	}
	content := 0
	for i := s; i < e; i++ {
		if !connectors[toks[i].Key] && toks[i].Key != "the" {
			content++
		}
	}
	if s >= e || content > maxSpanWords || e-s > maxSpanTokens {
		return p, false
	}
	return [2]int{s, e}, true
}

func keyOf(toks []Token, p [2]int) string {
	words := make([]string, 0, p[1]-p[0])
	for i := p[0]; i < p[1]; i++ {
		words = append(words, toks[i].Key)
	}
	return strings.Join(words, " ")
}

// affix reports whether short is a word prefix or suffix of long.
func affix(long, short []string) bool {
	if len(short) >= len(long) {
		return false
	}
	pre, suf := true, true
	for i, w := range short {
		if long[i] != w {
			pre = false
		}
		if long[len(long)-len(short)+i] != w {
			suf = false
		}
	}
	return pre || suf
}

// topSurface is the most frequent form; on a tie, a form not all in
// capitals ("Kyiv" over a dateline's "KYIV"), then the first in byte order.
func topSurface(m map[string]int) string {
	best, bestN := "", -1
	for s, n := range m {
		if n > bestN || (n == bestN && better(s, best)) {
			best, bestN = s, n
		}
	}
	return best
}

func better(a, b string) bool {
	ua, ub := isAcronymish(a), isAcronymish(b)
	if ua != ub {
		return !ua
	}
	return a < b
}

// isAcronymish: no lower-case letter.
func isAcronymish(s string) bool {
	for _, r := range s {
		if unicode.IsLower(r) {
			return false
		}
	}
	return true
}

// WireProposals puts proposals on the wire.
func WireProposals(ps []Proposal) []types.EntityProposal {
	out := make([]types.EntityProposal, 0, len(ps))
	for _, p := range ps {
		out = append(out, types.EntityProposal{
			Phrase: p.Phrase, Count: p.Count, Sentences: p.Sentences, InTitle: p.InTitle, InLead: p.InLead,
			Acronym: p.Acronym, Rarity: p.Rarity, Score: p.Score, Aliases: p.Aliases,
		})
	}
	return out
}
