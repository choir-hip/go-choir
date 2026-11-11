// Package speechsplit cuts a stream of text deltas into sentences for the ear.
//
// The Autoradio desk streams a model's answer to the phone one sentence at a
// time, so the phone can start speaking on the first. A sentence ends at ".",
// "!" or "?" (with any closing quote or bracket) followed by whitespace and
// then the start of the next sentence, or at a newline. A period is not an end
// inside a number ("3.5"), inside a dotted abbreviation ("U.S.", "e.g."),
// after a title or a common abbreviation ("Dr.", "Mrs.", "St."), after a
// single capital initial ("J. K. Rowling"), or before a lowercase word. A run
// of MaxRunes characters with no end at all is cut at the last natural pause (a
// transport safeguard, see MaxRunes).
package speechsplit

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// MaxRunes is a transport safeguard, not a steer on what the model says: a
// stream that runs this long with no terminator at all (a page of text with no
// full stop) is cut at its last pause so the phone is never starved of a
// speech event. It is far above any real spoken sentence; the length and shape
// of the answer are the prompt's to steer (owner 2026-11-11: no deterministic
// rules to shape a model).
const MaxRunes = 4000

// Splitter accumulates deltas and returns each sentence as it completes.
// The zero value is ready.
type Splitter struct {
	buf string
}

// Push adds a delta and returns the sentences it completed, in order.
func (s *Splitter) Push(delta string) []string {
	s.buf += delta
	return s.drain(false)
}

// Flush returns what is left at the end of the stream as a final sentence.
func (s *Splitter) Flush() []string { return s.drain(true) }

func (s *Splitter) drain(final bool) []string {
	var out []string
	for {
		end, rest, ok := nextBoundary(s.buf, final)
		if !ok {
			break
		}
		if sentence := clean(s.buf[:end]); sentence != "" {
			out = append(out, sentence)
		}
		s.buf = s.buf[rest:]
	}
	if final {
		if sentence := clean(s.buf); sentence != "" {
			out = append(out, sentence)
		}
		s.buf = ""
	}
	return out
}

// titles never end a sentence; a period after one belongs to it.
var titles = map[string]bool{
	"mr": true, "mrs": true, "ms": true, "dr": true, "prof": true, "sr": true, "jr": true, "st": true, "mt": true,
	"gen": true, "sen": true, "rep": true, "gov": true, "lt": true, "col": true, "capt": true, "sgt": true, "rev": true, "hon": true,
	"vs": true, "approx": true, "inc": true, "ltd": true, "co": true, "corp": true, "dept": true,
	"e.g": true, "i.e": true, "u.s": true, "u.k": true, "u.n": true, "d.c": true, "u.s.a": true,
}

// beforeNumber abbreviations are one only when a number follows ("No. 5",
// "Jan. 5"); otherwise the word ends the sentence ("The answer is no.").
var beforeNumber = map[string]bool{
	"no": true, "fig": true,
	"jan": true, "feb": true, "mar": true, "apr": true, "jun": true, "jul": true, "aug": true, "sep": true, "sept": true, "oct": true, "nov": true, "dec": true,
}

// softAbbreviations end a sentence only when an uppercase word follows.
var softAbbreviations = map[string]bool{"etc": true, "a.m": true, "p.m": true}

// nextBoundary finds the end of the first sentence in buf: end is where the
// sentence text stops, rest where the next one starts. With final false it
// does not decide a boundary it cannot yet see (a period at the very end of
// the buffer waits for the next character).
func nextBoundary(buf string, final bool) (end, rest int, ok bool) {
	runes := 0
	for i := 0; i < len(buf); {
		r, w := utf8.DecodeRuneInString(buf[i:])
		runes++
		switch {
		case r == '\n':
			if strings.TrimSpace(buf[:i]) != "" {
				return i, i + w, true
			}
		case isScriptTerminator(r):
			// Terminators of other scripts end a sentence with no abbreviation
			// question; closing quotes and brackets stay with it, and the
			// spaces after it are consumed.
			j := i + w
			for j < len(buf) {
				c, cw := utf8.DecodeRuneInString(buf[j:])
				if isScriptTerminator(c) || isCloser(c) {
					j += cw
					continue
				}
				break
			}
			if j >= len(buf) && !final {
				return 0, 0, false // a closing quote may still arrive
			}
			k := j
			for k < len(buf) {
				c, cw := utf8.DecodeRuneInString(buf[k:])
				if !unicode.IsSpace(c) {
					break
				}
				k += cw
			}
			return j, k, true
		case r == '.' || r == '!' || r == '?' || r == '…':
			j := i + w
			for j < len(buf) {
				c, cw := utf8.DecodeRuneInString(buf[j:])
				if c == '.' || c == '!' || c == '?' || c == '…' || isCloser(c) {
					j += cw
					continue
				}
				break
			}
			if j >= len(buf) {
				if final {
					return 0, 0, false
				}
				return 0, 0, false // wait for the next character
			}
			c, _ := utf8.DecodeRuneInString(buf[j:])
			if !unicode.IsSpace(c) {
				// "3.5", "U.S.A", "example.com": inside a token.
				i = j
				continue
			}
			k := j
			for k < len(buf) {
				c, cw := utf8.DecodeRuneInString(buf[k:])
				if !unicode.IsSpace(c) {
					break
				}
				k += cw
			}
			if k >= len(buf) {
				return 0, 0, false // the next sentence has not begun
			}
			next, _ := utf8.DecodeRuneInString(buf[k:])
			if r == '.' && !periodEnds(buf[:i], next) {
				i = j
				continue
			}
			return j, k, true
		}
		if runes >= MaxRunes {
			if cut, restAt, ok := forcedCut(buf, i+w); ok {
				return cut, restAt, true
			}
		}
		i += w
	}
	return 0, 0, false
}

// periodEnds reports whether a period after before ends a sentence, given the
// first character of the next word.
func periodEnds(before string, next rune) bool {
	word := before
	if i := strings.LastIndexFunc(before, unicode.IsSpace); i >= 0 {
		word = before[i+1:]
	}
	word = strings.ToLower(strings.TrimLeft(word, "\"'([“‘"))
	if titles[word] {
		return false
	}
	// A single letter is an initial ("J. K. Rowling").
	if utf8.RuneCountInString(word) == 1 && unicode.IsLetter([]rune(word)[0]) {
		return false
	}
	if beforeNumber[word] {
		return !unicode.IsDigit(next)
	}
	if softAbbreviations[word] {
		return unicode.IsUpper(next)
	}
	if unicode.IsLower(next) {
		return false
	}
	return true
}

// forcedCut cuts an over-long run at its last pause at or before limit (a byte
// offset just past the 200th rune): after a comma, semicolon or colon in the
// second half, else at the last space, else at limit.
func forcedCut(buf string, limit int) (end, rest int, ok bool) {
	head := buf[:limit]
	if i := strings.LastIndexAny(head, ",;:"); i >= len(head)/2 {
		return i + 1, i + 1, true
	}
	if i := strings.LastIndexFunc(head, unicode.IsSpace); i > 0 {
		_, w := utf8.DecodeRuneInString(head[i:]) // a space may be several bytes
		return i, i + w, true
	}
	return limit, limit, true
}

// emphasis is a paired run of asterisks hugging its text ("**bold**", "*word*").
var emphasis = regexp.MustCompile(`(\*+)([^*\s](?:[^*]*[^*\s])?)(\*+)`)

// stars is a leftover run of two or more asterisks.
var stars = regexp.MustCompile(`\*{2,}`)

// structure is a leading heading, bullet or quote mark.
var structure = regexp.MustCompile(`^(?:(?:#{1,6}|[-\x{2022}>])\s+)+`)

// clean makes a sentence plain for the ear without changing what it says:
// whitespace collapsed, emphasis marks and backticks dropped, and a leading
// heading, bullet or quote mark (always followed by a space) removed. A sign,
// a hash or a tilde inside the text is the text: "-5 degrees", "#1 story",
// "about ~3 million", "C#".
func clean(s string) string {
	// Paired emphasis marks around a word or phrase go; a lone "*" ("5 * 3")
	// is the text and stays. Leftover runs of two or more, and backticks, go.
	s = emphasis.ReplaceAllString(s, "$2")
	s = strings.NewReplacer("`", "").Replace(s)
	s = stars.ReplaceAllString(s, "")
	s = strings.Join(strings.Fields(s), " ")
	return strings.TrimSpace(structure.ReplaceAllString(s, ""))
}

// isScriptTerminator is a sentence terminator of a script other than Latin:
// CJK, Arabic and Urdu, Devanagari.
func isScriptTerminator(r rune) bool {
	switch r {
	case '。', '！', '？', '۔', '؟', '।', '॥':
		return true
	}
	return false
}

// isCloser is a closing quote or bracket that stays with the sentence it ends.
func isCloser(r rune) bool {
	switch r {
	case '"', '\'', ')', ']', '”', '’', '」', '』', '）', '】', '»':
		return true
	}
	return false
}
