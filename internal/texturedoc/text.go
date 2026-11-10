package texturedoc

import (
	"encoding/json"
	"strings"
	"unicode"
)

// CountWords counts whitespace-separated words, except that text written
// without spaces between words (Chinese, Japanese) counts one word per two
// ideographs or kana, so a CJK story is not read as a stub. The published
// front's depth and the publish word floor count the same way.
func CountWords(text string) int {
	n := 0
	for _, field := range strings.Fields(text) {
		dense, other := 0, false
		for _, r := range field {
			if unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r) {
				dense++
			} else if unicode.IsLetter(r) || unicode.IsDigit(r) {
				other = true
			}
		}
		n += (dense + 1) / 2
		if other || dense == 0 {
			n++
		}
	}
	return n
}

// TextBlock is one top-level block of a body and the text a reader sees in it.
type TextBlock struct {
	Type string
	Text string
}

// TextBlocks returns the text of each top-level block of a structured body,
// in order, whitespace collapsed; blocks with no text are left out. A body
// that cannot be read yields none.
func TextBlocks(raw json.RawMessage) []TextBlock {
	if len(strings.TrimSpace(string(raw))) == 0 {
		return nil
	}
	var doc StructuredTextureDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil
	}
	var out []TextBlock
	for _, block := range doc.Doc.Content {
		var b strings.Builder
		walkBlockText(&b, block)
		if text := strings.Join(strings.Fields(b.String()), " "); text != "" {
			out = append(out, TextBlock{Type: block.Type, Text: text})
		}
	}
	return out
}

// walkBlockText appends a block's text nodes; nested blocks (list items,
// paragraphs in a quote) are kept apart by a space.
func walkBlockText(b *strings.Builder, node Node) {
	switch node.Type {
	case "text":
		b.WriteString(node.Text)
	case "hard_break":
		b.WriteByte(' ')
	}
	for _, child := range node.Content {
		walkBlockText(b, child)
	}
	switch node.Type {
	case "paragraph", "list_item", "listItem", "heading":
		b.WriteByte(' ')
	}
}
