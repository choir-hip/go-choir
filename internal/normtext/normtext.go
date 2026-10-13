// Package normtext is the one text normalization shared by block content ids
// (cid) and the quote matcher (internal/quotematch). Keeping both on one
// function is what stops a block's address and a quote's match from
// disagreeing about the same words (Cut 2 unit 2a, "Normalization differs
// between cid and matching").
//
// There are three levels, each the previous one plus more folding:
//
//   - LevelCanonical (Normalize): Unicode NFKC; zero-width and other
//     invisible format characters dropped; curly quotes, apostrophes and
//     dashes folded to ASCII; every whitespace run (newlines, NBSP, ideographic
//     space) collapsed to one space; leading and trailing space trimmed. Case
//     is kept. The cid hashes this level.
//   - LevelFolded (Fold): LevelCanonical, lowercased. The matcher's
//     tolerant_ws_case tier compares at this level.
//   - LevelLoose (Loose): LevelFolded with every rune that is not a letter,
//     digit or space dropped, without disturbing the whitespace collapse (so
//     "U.S." becomes "us" and "foo, bar" becomes "foo bar", as in mike's
//     normalizeWithMap). The matcher's tolerant_punct tier compares here.
//
// The matcher's exact tier compares raw bytes and uses no level.
//
// Changing any rule here re-addresses every cid and changes match results, so
// it must bump Version. The NFKC tables come from golang.org/x/text; a module
// upgrade that changes them for characters in captured text is also a Version
// bump.
package normtext

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// Version names the normalization rules. quotematch.Version embeds it.
const Version = "nt1"

// CIDLength is the number of hex characters in a cid: the first 64 bits of
// the sha256. A cid only needs to be unique within one document version (a
// Ref also names the document), so 64 bits is ample.
const CIDLength = 16

// Level selects how much folding Map applies; see the package comment.
type Level int

const (
	LevelCanonical Level = iota
	LevelFolded
	LevelLoose
)

// Normalize returns s at LevelCanonical. It is the text the cid hashes.
func Normalize(s string) string { return Map(s, LevelCanonical).Text }

// Fold returns s at LevelFolded: Normalize, lowercased.
func Fold(s string) string { return Map(s, LevelFolded).Text }

// Loose returns s at LevelLoose: Fold with punctuation and symbols dropped.
func Loose(s string) string { return Map(s, LevelLoose).Text }

// Mapped is normalized text with, for each of its bytes, the byte range of
// the source text it came from. Start[i] and End[i] bound the source bytes
// behind Text[i]. A range always covers whole source runes; when one source
// rune expands to several (a ligature, an ellipsis) or several compose into
// one (a combining accent), every output byte points at the whole group.
type Mapped struct {
	Text  string
	Start []int
	End   []int
}

// Source maps the normalized byte range [i, j) back to the source byte range
// that produced it. It requires 0 <= i < j <= len(m.Text).
func (m Mapped) Source(i, j int) (start, end int) {
	return m.Start[i], m.End[j-1]
}

// Map normalizes s at the given level and records where each output byte
// came from.
func Map(s string, level Level) Mapped {
	var (
		out          strings.Builder
		starts, ends []int
		pending      []rune // NFKC output whose source range is not yet known
		groupStart   int
		spacePending bool
		spaceStart   int
		spaceEnd     int
		buf          [utf8.UTFMax]byte
	)
	emit := func(r rune, start, end int) {
		n := utf8.EncodeRune(buf[:], r)
		out.Write(buf[:n])
		for k := 0; k < n; k++ {
			starts = append(starts, start)
			ends = append(ends, end)
		}
	}
	flush := func(start, end int) {
		for _, r := range pending {
			r = foldRune(r)
			if isInvisible(r) {
				continue
			}
			if unicode.IsSpace(r) {
				if !spacePending {
					spacePending = true
					spaceStart, spaceEnd = start, end
				}
				continue
			}
			if level >= LevelLoose && !unicode.IsLetter(r) && !unicode.IsNumber(r) {
				// Dropped without touching spacePending, as in mike.
				continue
			}
			if level >= LevelFolded {
				r = unicode.ToLower(r)
			}
			if spacePending && out.Len() > 0 {
				emit(' ', spaceStart, spaceEnd)
			}
			spacePending = false
			emit(r, start, end)
		}
		pending = pending[:0]
	}

	var it norm.Iter
	it.InitString(norm.NFKC, s)
	for !it.Done() {
		// Next's buffer is only valid until the next call, so decode now.
		for _, r := range string(it.Next()) {
			pending = append(pending, r)
		}
		// An expansion (a ligature, an ellipsis) yields several segments
		// before Pos moves; they all belong to the source runes Pos then
		// passes over.
		if pos := it.Pos(); pos > groupStart {
			flush(groupStart, pos)
			groupStart = pos
		}
	}
	if len(pending) > 0 {
		flush(groupStart, len(s))
	}
	return Mapped{Text: out.String(), Start: starts, End: ends}
}

// foldRune folds the quote, apostrophe and dash variants NFKC leaves alone.
func foldRune(r rune) rune {
	switch r {
	case '\u2018', '\u2019', '\u201a', '\u201b', '\u02bc':
		return '\''
	case '\u201c', '\u201d', '\u201e', '\u201f':
		return '"'
	case '\u2010', '\u2011', '\u2012', '\u2013', '\u2014', '\u2015', '\u2212', '\ufe58', '\ufe63':
		return '-'
	}
	return r
}

// isInvisible reports the zero-width and format characters that render as
// nothing: they are dropped, not treated as spaces, so "co\u200bop" reads as
// the word the reader sees.
func isInvisible(r rune) bool {
	switch r {
	case '\u200b', '\u200c', '\u200d', '\u2060', '\ufeff', '\u00ad', '\u034f':
		return true
	}
	return false
}

// Block is a block's kind and text, the input to cid assignment.
type Block struct {
	Kind string
	Text string
}

// CID returns a block's content id: the first CIDLength hex characters of
// sha256(kind, NUL, Normalize(text)). Whitespace changes keep the cid; case,
// word, punctuation and kind changes change it.
func CID(kind, text string) string {
	sum := sha256.Sum256([]byte(kind + "\x00" + Normalize(text)))
	return hex.EncodeToString(sum[:])[:CIDLength]
}

// AssignCIDs returns the cid of each block in document order. The first block
// with a given cid keeps it; the k-th later duplicate gets "cid.k" (the second
// occurrence is cid.1). The result depends only on the blocks and their order.
func AssignCIDs(blocks []Block) []string {
	ids := make([]string, len(blocks))
	seen := make(map[string]int, len(blocks))
	for i, b := range blocks {
		cid := CID(b.Kind, b.Text)
		if k := seen[cid]; k > 0 {
			ids[i] = cid + "." + strconv.Itoa(k)
		} else {
			ids[i] = cid
		}
		seen[cid]++
	}
	return ids
}
