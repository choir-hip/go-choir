// Package quotematch checks that a quote is the cited source's own words and
// returns the source's span for it. It ports the cascade in mike's
// locateQuote (~/mike/backend/src/modules/chat/engine/verifyCitations.ts):
// exact, then whitespace and case tolerant, then punctuation tolerant, with
// the first tier that matches winning. Tolerant tiers compare text from
// internal/normtext, the same normalization the block cid hashes.
//
// Two rules are stricter than mike, because Choir quotes cite one block of a
// captured document rather than a whole page's text:
//
//   - A quote matches within a single block. It never spans blocks, and every
//     ellipsis segment must sit in the same block.
//   - Ellipsis segments must appear in quote order. mike checks each segment
//     on its own for a plain ellipsis quote and only orders the segments of a
//     cross-page quote; here order always holds.
//
// The rendered quote is always the source's own span (Result.Excerpt), never
// the quote as written, and a tolerant match is reported as such so the
// reader can show it with an approximate marker.
package quotematch

import (
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/yusefmosiah/go-choir/internal/normtext"
)

// Version names the matcher's rules, including the normalization it compares
// with. Memoized match results are keyed by it; bump it when any rule here or
// in normtext changes.
const Version = "qm1+" + normtext.Version

// Outcome is the verdict for one quote against one cited document.
type Outcome string

const (
	// Matched means the quote is in one block of the cited document.
	Matched Outcome = "matched"
	// NotFound means the document was captured and no block holds the quote.
	NotFound Outcome = "not_found"
	// NotCaptured means the cited document has no capture to check against.
	// Match never returns it; callers that find no capture report it, so a
	// quote research only saw in a search snippet is not read as a mismatch
	// (C8 quote census, implication 2).
	NotCaptured Outcome = "not_captured"
)

// Tier is the cascade step that matched.
type Tier string

const (
	// TierExact compares raw bytes.
	TierExact Tier = "exact"
	// TierWSCase compares at normtext.LevelFolded: NFKC, invisible characters
	// dropped, quotes and dashes folded, whitespace collapsed, lowercased.
	TierWSCase Tier = "tolerant_ws_case"
	// TierPunct compares at normtext.LevelLoose: LevelFolded with every rune
	// that is not a letter, digit or space dropped.
	TierPunct Tier = "tolerant_punct"
)

var tiers = []Tier{TierExact, TierWSCase, TierPunct}

// Block is one block of a captured document: its cid and its original text.
type Block struct {
	ID   string
	Text string
}

// Span is a byte range of a block's original Text and the bytes it covers.
type Span struct {
	Start int    `json:"start"`
	End   int    `json:"end"`
	Text  string `json:"text"`
}

// Result is the verdict for one quote. For a match, BlockID names the block,
// Spans holds one span per ellipsis segment (one for a plain quote), Start
// and End cover them all, and Excerpt is the source's own words: the segment
// spans joined by " ... ", as mike joins them.
type Result struct {
	Outcome Outcome `json:"outcome"`
	Tier    Tier    `json:"tier,omitempty"`
	BlockID string  `json:"block_id,omitempty"`
	Start   int     `json:"start"`
	End     int     `json:"end"`
	Excerpt string  `json:"excerpt,omitempty"`
	Spans   []Span  `json:"spans,omitempty"`
}

// Tolerant reports a match that was not byte-exact.
func (r Result) Tolerant() bool {
	return r.Outcome == Matched && r.Tier != TierExact
}

// ellipsis splits a quote into segments, as mike's ELLIPSIS_PATTERN does.
var ellipsis = regexp.MustCompile(`\.{3}|\x{2026}`)

// Match finds quote in one of blocks. Tiers are tried in cascade order, and
// within a tier blocks are tried in order, so an exact match in a later
// block beats a tolerant match in an earlier one. An empty quote, a quote
// with no letter or digit, or an empty block list is NotFound.
func Match(quote string, blocks []Block) Result {
	segments := splitSegments(quote)
	if len(segments) == 0 || len(blocks) == 0 {
		return Result{Outcome: NotFound}
	}
	hays := make([]haystack, len(blocks))
	for i := range blocks {
		hays[i].text = blocks[i].Text
	}
	for ceiling := range tiers {
		for i := range blocks {
			spans, used, ok := hays[i].chain(segments, ceiling)
			if !ok {
				continue
			}
			texts := make([]string, len(spans))
			for k, s := range spans {
				texts[k] = s.Text
			}
			return Result{
				Outcome: Matched,
				Tier:    tiers[used],
				BlockID: blocks[i].ID,
				Start:   spans[0].Start,
				End:     spans[len(spans)-1].End,
				Excerpt: strings.Join(texts, " ... "),
				Spans:   spans,
			}
		}
	}
	return Result{Outcome: NotFound}
}

// segment is one ellipsis-separated piece of a quote at each tier's level.
type segment struct {
	raw, folded, loose string
}

// splitSegments splits on "..." and U+2026, trims each piece and drops
// pieces with no letter or digit (mike drops punctuation-only remnants such
// as the fourth dot of "....").
func splitSegments(quote string) []segment {
	var out []segment
	for _, piece := range ellipsis.Split(quote, -1) {
		piece = strings.TrimSpace(piece)
		if !strings.ContainsFunc(piece, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsNumber(r) }) {
			continue
		}
		out = append(out, segment{raw: piece, folded: normtext.Fold(piece), loose: normtext.Loose(piece)})
	}
	return out
}

// haystack is one block's text with its tolerant forms built on first use.
type haystack struct {
	text   string
	folded *normtext.Mapped
	loose  *normtext.Mapped
}

// chain locates every segment in order within this block, each tier no
// weaker than ceiling. Each segment takes the candidate that ends earliest
// at or after the previous segment's end, which is optimal for an in-order
// chain with no gap bound, so no backtracking is needed. It returns the
// weakest tier any segment used.
func (h *haystack) chain(segments []segment, ceiling int) ([]Span, int, bool) {
	spans := make([]Span, 0, len(segments))
	cursor, used := 0, 0
	for _, seg := range segments {
		best, bestTier, found := Span{}, 0, false
		for tier := 0; tier <= ceiling; tier++ {
			start, end, ok := h.locate(seg, tier, cursor)
			if ok && (!found || end < best.End) {
				best, bestTier, found = Span{Start: start, End: end}, tier, true
			}
		}
		if !found {
			return nil, 0, false
		}
		best.Text = h.text[best.Start:best.End]
		spans = append(spans, best)
		cursor = best.End
		used = max(used, bestTier)
	}
	return spans, used, true
}

// locate finds the first occurrence of seg at the given tier whose source
// span starts at or after the source byte offset from.
func (h *haystack) locate(seg segment, tier, from int) (start, end int, ok bool) {
	if from > len(h.text) {
		return 0, 0, false
	}
	if tiers[tier] == TierExact {
		i := strings.Index(h.text[from:], seg.raw)
		if i < 0 {
			return 0, 0, false
		}
		return from + i, from + i + len(seg.raw), true
	}
	var m *normtext.Mapped
	needle := seg.folded
	if tiers[tier] == TierWSCase {
		if h.folded == nil {
			mapped := normtext.Map(h.text, normtext.LevelFolded)
			h.folded = &mapped
		}
		m = h.folded
	} else {
		if h.loose == nil {
			mapped := normtext.Map(h.text, normtext.LevelLoose)
			h.loose = &mapped
		}
		m, needle = h.loose, seg.loose
	}
	if needle == "" {
		return 0, 0, false
	}
	// Start is nondecreasing, so the first normalized byte at or after from
	// is a binary search away.
	n0 := sort.SearchInts(m.Start, from)
	i := strings.Index(m.Text[n0:], needle)
	if i < 0 {
		return 0, 0, false
	}
	start, end = m.Source(n0+i, n0+i+len(needle))
	return start, end, true
}
