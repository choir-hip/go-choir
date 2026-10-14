package texturedoc

import (
	"encoding/json"
	"strings"
)

// Headline returns the headline a structured body leads with: the text of
// its first block that has text, when that block is a heading of any
// level. It returns "" when the body leads with anything else (a
// paragraph, a list, a quote), when the leading heading has no text, or
// when the body cannot be read.
//
// A story's title is the headline Texture writes (docs/problems/
// instruction-text-published-as-story-titles-2026-10-14.md): the store
// titles a story from it in the commit of each Texture revision, and
// automatic publishing holds a story revision that has none.
//
// Only text nodes count; a citation (source_ref) inside the heading adds
// nothing and a hard break reads as a space. Whitespace is collapsed.
func Headline(raw json.RawMessage) string {
	if len(strings.TrimSpace(string(raw))) == 0 {
		return ""
	}
	var doc StructuredTextureDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		return ""
	}
	for _, block := range doc.Doc.Content {
		var b strings.Builder
		inlineText(&b, block)
		text := strings.Join(strings.Fields(b.String()), " ")
		if text == "" {
			continue
		}
		if block.Type != "heading" {
			return ""
		}
		return text
	}
	return ""
}

// inlineText appends a block's text as a reader sees it: text nodes run
// together (marks split a word into several), a hard break is a space.
func inlineText(b *strings.Builder, node Node) {
	switch node.Type {
	case "text":
		b.WriteString(node.Text)
	case "hard_break":
		b.WriteByte(' ')
	}
	for _, child := range node.Content {
		inlineText(b, child)
	}
}
