package entitytext

import (
	"strings"
	"unicode"

	"github.com/yusefmosiah/go-choir/internal/normtext"
)

// Bounds (slice c0 §1, G4). Text past MaxTextRunes is not read; the work is
// linear in tokens, so a full-size story costs a few milliseconds.
const (
	MaxTextRunes  = 20000
	MaxTitleRunes = 400
)

// Token is one word of a story.
type Token struct {
	// Surface is the word as written (normalised: NFKC, ASCII quotes and
	// dashes), without a trailing possessive.
	Surface string
	// Key is the word's match key (tokenKey).
	Key string
	// Sentence is the index of the token's sentence in its part (title or
	// body).
	Sentence int
	// SentenceStart: the first word of a sentence, or of a quotation or a
	// clause after a colon, where a capital says nothing about a name.
	SentenceStart bool
	// Capital: the first letter is upper case.
	Capital bool
	// Acronym: all capitals, 2 to 6 letters (NATO, U.S.).
	Acronym bool

	// joined: only spaces or a hyphen separate the token from the previous
	// one in the same sentence, so a name may run across them.
	joined bool
}

// Doc is a parsed story: its title and body tokens.
type Doc struct {
	Title     []Token
	Body      []Token
	sentences int
}

// Sentences is the number of body sentences that have a word.
func (d *Doc) Sentences() int { return d.sentences }

// leadSentences is how many opening sentences count as the lead.
const leadSentences = 2

// Parse splits a story's title and text into tokens and sentences, each
// bounded.
func Parse(title, text string) *Doc {
	d := &Doc{}
	d.Title, _ = tokenize(cut(title, MaxTitleRunes))
	d.Body, d.sentences = tokenize(cut(text, MaxTextRunes))
	return d
}

func cut(s string, max int) string {
	n := 0
	for i := range s {
		if n == max {
			return s[:i]
		}
		n++
	}
	return s
}

func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.Is(unicode.Mn, r)
}

// tokenize is a rule sentence splitter and word tokenizer. Each line is
// normalised by normtext (NFKC, quotes, dashes, spaces); a line break, or a
// stop, question or exclamation mark followed by a space or the line's end
// (after any closing quotes or brackets), ends a sentence, except after an
// abbreviation or a single initial.
func tokenize(text string) ([]Token, int) {
	var (
		toks         []Token
		sent         int
		sentHasTok   bool
		pendingBreak bool
		clauseStart  bool // the next word follows an opening quote or a colon
		gap          bool // punctuation since the previous word
	)
	for _, line := range strings.Split(text, "\n") {
		if sentHasTok {
			pendingBreak = true
		}
		rs := []rune(normtext.Normalize(line))
		n := len(rs)
		for i := 0; i < n; {
			r := rs[i]
			if !isWordRune(r) {
				switch r {
				case '.', '!', '?':
					if endsSentence(rs, i, toks, sentHasTok) {
						pendingBreak = true
					}
					gap = true
				case ':':
					clauseStart = true
					gap = true
				case '"', '\'', '(', '[':
					if (r == '"' || r == '\'') && (i == 0 || unicode.IsSpace(rs[i-1]) || rs[i-1] == '(' || rs[i-1] == ':') && i+1 < n && !unicode.IsSpace(rs[i+1]) {
						clauseStart = true
					}
					gap = true
				case ' ':
				case '-':
					if !(i > 0 && isWordRune(rs[i-1]) && i+1 < n && isWordRune(rs[i+1])) {
						gap = true
					}
				default:
					gap = true
				}
				i++
				continue
			}
			surface, next, acronym := readWord(rs, i)
			i = next
			key := tokenKey(surface)
			if key == "" {
				continue
			}
			if pendingBreak {
				sent++
				sentHasTok = false
				pendingBreak = false
			}
			first := []rune(surface)[0]
			t := Token{
				Surface:       surface,
				Key:           key,
				Sentence:      sent,
				SentenceStart: !sentHasTok || clauseStart,
				Capital:       unicode.IsUpper(first) || unicode.IsTitle(first),
				Acronym:       acronym,
				joined:        sentHasTok && !gap,
			}
			toks = append(toks, t)
			sentHasTok = true
			clauseStart = false
			gap = false
		}
		gap = true
	}
	if len(toks) == 0 {
		return nil, 0
	}
	return toks, sent + 1
}

// readWord reads the word at rs[i]: a dotted acronym (U.S.), or a run of
// letters, digits and marks with inner apostrophes (O'Brien), with a trailing
// possessive ('s, or s') dropped. It returns the surface, the index after
// the word (and its possessive), and whether the word is an acronym.
func readWord(rs []rune, i int) (string, int, bool) {
	n := len(rs)
	// Dotted acronym: two or more capital-period pairs.
	j, pairs := i, 0
	for j+1 < n && unicode.IsUpper(rs[j]) && rs[j+1] == '.' {
		j += 2
		pairs++
	}
	if pairs >= 2 && (j >= n || !isWordRune(rs[j])) {
		return string(rs[i:j]), j, pairs <= 6
	}
	j = i
	for j < n {
		r := rs[j]
		if isWordRune(r) {
			j++
			continue
		}
		if r == '\'' && j+1 < n && isWordRune(rs[j+1]) && j > i {
			// "'s" closing the word is a possessive, not an inner apostrophe.
			if (rs[j+1] == 's' || rs[j+1] == 'S') && (j+2 >= n || !isWordRune(rs[j+2])) {
				break
			}
			j++
			continue
		}
		break
	}
	surface := string(rs[i:j])
	next := j
	if next < n && rs[next] == '\'' {
		if next+1 < n && (rs[next+1] == 's' || rs[next+1] == 'S') && (next+2 >= n || !isWordRune(rs[next+2])) {
			next += 2 // Zelenskyy's
		} else if strings.HasSuffix(surface, "s") && (next+1 >= n || !isWordRune(rs[next+1])) {
			next++ // states'
		}
	}
	return surface, next, isAcronym(surface)
}

// isAcronym: all letters capitals, 2 to 6 of them, no lower case.
func isAcronym(s string) bool {
	letters := 0
	for _, r := range s {
		switch {
		case unicode.IsLetter(r):
			if !unicode.IsUpper(r) {
				return false
			}
			letters++
		case r == '.':
		default:
			return false
		}
	}
	return letters >= 2 && letters <= 6
}

// endsSentence decides whether the mark at rs[i] ends a sentence.
func endsSentence(rs []rune, i int, toks []Token, sentHasTok bool) bool {
	if !sentHasTok {
		return false
	}
	n := len(rs)
	j := i + 1
	for j < n && (rs[j] == '.' || rs[j] == '!' || rs[j] == '?' || rs[j] == '"' || rs[j] == '\'' || rs[j] == ')' || rs[j] == ']') {
		j++
	}
	if j < n && !unicode.IsSpace(rs[j]) {
		return false // 3.5, example.com
	}
	if rs[i] != '.' || i == 0 || !isWordRune(rs[i-1]) || len(toks) == 0 {
		return true
	}
	prev := toks[len(toks)-1]
	if abbreviations[prev.Key] {
		return false
	}
	pr := []rune(prev.Surface)
	if len(pr) == 1 && unicode.IsUpper(pr[0]) {
		return false // an initial: John F. Kennedy
	}
	return true
}
