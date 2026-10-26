// Package entitytext is slice c0's deterministic entity extraction
// (docs/problems/related-stories-no-entity-layer-2026-10-26.md, "Slice c0",
// §1): standard keyword processing over a story's title and text, with no
// model. It proposes candidate names (capitalised spans, acronyms, short
// forms merged into the longest name) and matches known entities through a
// token-trie gazetteer (Aho-Corasick at the token level, longest match
// first). Corpusd and the guest share it; System 1 and the research desk
// judge what it finds.
//
// Identity is not here: an entity's id is still types.MintedEntityID over
// types.NormalizeEntityName, which keeps diacritics, so the ids slice a
// minted never move. Matching uses Key, which is that normalisation plus
// Unicode compatibility folding and diacritic folding, so "Vučić" in a
// story finds the entity named "Vucic" and the reverse.
package entitytext

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"

	"github.com/yusefmosiah/go-choir/internal/normtext"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Key is a name's match key: NFKC with quotes and dashes folded
// (normtext), then types.NormalizeEntityName (case, a possessive, periods
// and apostrophes, punctuation, a leading "the", company forms), then
// diacritics folded. "" when nothing is left.
func Key(name string) string {
	n := types.NormalizeEntityName(normtext.Normalize(name))
	if n == "" {
		return ""
	}
	return strings.Join(strings.Fields(foldDiacritics(n)), " ")
}

// tokenKey is one word's match key, the per-word form of Key: lower case,
// periods and apostrophes removed, diacritics folded.
func tokenKey(surface string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(surface) {
		if r == '.' || r == '\'' {
			continue
		}
		b.WriteRune(r)
	}
	return foldDiacritics(b.String())
}

// letterFolds are letters with no canonical decomposition that readers treat
// as their base letter.
var letterFolds = map[rune]string{
	'ł': "l", 'ø': "o", 'đ': "d", 'ð': "d", 'þ': "th", 'ß': "ss", 'æ': "ae", 'œ': "oe", 'ı': "i", 'ħ': "h",
}

// foldDiacritics drops combining marks after canonical decomposition and
// folds the few letters that have none. Stateless, so safe for concurrent use.
func foldDiacritics(s string) string {
	ascii := true
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			ascii = false
			break
		}
	}
	if ascii {
		return s
	}
	var b strings.Builder
	for _, r := range norm.NFD.String(s) {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		if f, ok := letterFolds[r]; ok {
			b.WriteString(f)
			continue
		}
		b.WriteRune(r)
	}
	return norm.NFC.String(b.String())
}
