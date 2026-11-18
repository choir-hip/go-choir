package capturefts

import (
	"unicode"
	"unicode/utf8"
)

const (
	// TargetPassageRunes is the size passages are packed to: about 300
	// English tokens, the unit a sentence-embedding model reads.
	TargetPassageRunes = 1800
	// MaxPassageRunes is the most a passage holds (a paragraph that fits
	// stays whole up to this).
	MaxPassageRunes = 2400
	// MaxIndexedBytes is the most of one page's text that is indexed. The
	// capture route already cuts a feed item's body at 1 MiB; a long
	// document is a few percent of pages, and its first 256 KiB (about 140
	// passages) is where its subject is. Past the cap the page is
	// still found by what comes before it, and Put says it was cut.
	MaxIndexedBytes = 256 << 10
)

// Span is a byte range of a page's text.
type Span struct{ Start, End int }

// Passages cuts text into passages of about TargetPassageRunes runes: whole
// paragraphs (the capture projection joins blocks with a blank line) packed
// together, and a paragraph longer than MaxPassageRunes split at the last
// sentence end or space of a window. Spans are in order, do not overlap, and
// are offsets into text; a text with no content has none.
func Passages(text string) []Span {
	var out []Span
	curStart, curEnd, curRunes := -1, 0, 0
	flush := func() {
		if curStart >= 0 {
			out = append(out, Span{curStart, curEnd})
		}
		curStart, curEnd, curRunes = -1, 0, 0
	}
	for _, p := range paragraphs(text) {
		n := utf8.RuneCountInString(text[p.Start:p.End])
		if n > MaxPassageRunes {
			flush()
			for _, s := range splitLong(text, p) {
				out = append(out, s)
			}
			continue
		}
		if curStart >= 0 && curRunes+n > TargetPassageRunes {
			flush()
		}
		if curStart < 0 {
			curStart = p.Start
			curRunes = 0
		}
		curEnd = p.End
		curRunes += n
	}
	flush()
	return out
}

// paragraphs are the blank-line-separated blocks of text, trimmed.
func paragraphs(text string) []Span {
	var out []Span
	i := 0
	for i < len(text) {
		// skip leading space
		for i < len(text) && isSpaceByte(text, i) {
			i++
		}
		start := i
		end := i
		for i < len(text) {
			if text[i] == '\n' {
				j := i + 1
				for j < len(text) && (text[j] == ' ' || text[j] == '\t' || text[j] == '\r') {
					j++
				}
				if j < len(text) && text[j] == '\n' {
					break
				}
			}
			if !isSpaceByte(text, i) {
				end = i + 1
			}
			i++
		}
		// extend end to a rune boundary
		for end < len(text) && !utf8.RuneStart(text[end]) {
			end++
		}
		if end > start {
			out = append(out, Span{start, end})
		}
	}
	return out
}

func isSpaceByte(s string, i int) bool {
	c := s[i]
	return c == ' ' || c == '\n' || c == '\t' || c == '\r' || c == '\v' || c == '\f'
}

// splitLong cuts one long paragraph into windows of TargetPassageRunes,
// each ending at a sentence end or space in its last quarter when there is
// one, else at the window's end.
func splitLong(text string, p Span) []Span {
	var out []Span
	pos := p.Start
	for pos < p.End {
		// advance TargetPassageRunes runes
		end := pos
		for n := 0; n < TargetPassageRunes && end < p.End; n++ {
			_, w := utf8.DecodeRuneInString(text[end:p.End])
			end += w
		}
		if end < p.End {
			// look back over the last quarter for a boundary
			limit := end
			cut := -1
			for back, n := end, 0; back > pos && n < TargetPassageRunes/4; n++ {
				r, w := utf8.DecodeLastRuneInString(text[pos:back])
				if isSentenceEnd(r) {
					cut = back
					break
				}
				if unicode.IsSpace(r) && cut < 0 {
					cut = back
				}
				back -= w
			}
			if cut > pos && cut < limit+1 {
				end = cut
			}
		}
		// trim
		s, e := pos, end
		for s < e && isSpaceByte(text, s) {
			s++
		}
		for e > s && isSpaceByte(text, e-1) {
			e--
		}
		if e > s {
			out = append(out, Span{s, e})
		}
		pos = end
	}
	return out
}

func isSentenceEnd(r rune) bool {
	switch r {
	case '.', '!', '?', '。', '！', '？', '؟', '।':
		return true
	}
	return false
}
