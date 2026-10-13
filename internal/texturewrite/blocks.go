package texturewrite

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/texturedoc"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Block is one top-level block of a document as Texture addresses it: its
// alias in this frame (b1..bN), its durable id, its kind and its text in the
// Write grammar, citations as tokens. Writing Text back unchanged keeps the
// block, its id and its citations.
type Block struct {
	Alias string `json:"alias"`
	ID    string `json:"id"`
	Kind  string `json:"kind"`
	Text  string `json:"text"`
	// Node is the block itself; not part of a frame.
	Node texturedoc.Node `json:"-"`
}

// refAttr is the source_ref attribute carrying a Write citation's Ref.
const refAttr = "ref"

// tierAttr records the quote-match tier (exact or tolerant) of a citation.
const tierAttr = "quote_tier"

// DocBlocks lists a document's top-level blocks with their aliases.
func DocBlocks(doc texturedoc.StructuredTextureDoc) []Block {
	out := make([]Block, 0, len(doc.Doc.Content))
	for i, n := range doc.Doc.Content {
		out = append(out, Block{Alias: "b" + strconv.Itoa(i+1), ID: nodeID(n), Kind: n.Type, Text: renderBlock(n), Node: n})
	}
	return out
}

// renderBlock writes a block in the Write grammar.
func renderBlock(n texturedoc.Node) string {
	switch n.Type {
	case "heading":
		level := intAttr(n, "level")
		if level < 1 || level > 6 {
			level = 1
		}
		return strings.Repeat("#", level) + " " + renderInline(n.Content)
	case "bullet_list", "ordered_list":
		start := intAttr(n, "start")
		if start < 1 {
			start = 1
		}
		lines := make([]string, 0, len(n.Content))
		for i, item := range n.Content {
			marker := "- "
			if n.Type == "ordered_list" {
				marker = strconv.Itoa(start+i) + ". "
			}
			lines = append(lines, marker+renderFlow(item.Content))
		}
		return strings.Join(lines, "\n")
	case "blockquote":
		return "> " + renderFlow(n.Content)
	case "code_block":
		var b strings.Builder
		b.WriteString("```" + stringAttr(n, "language") + "\n")
		for i, c := range n.Content {
			if i > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(c.Text)
		}
		b.WriteString("\n```")
		return b.String()
	case "horizontal_rule":
		return "---"
	default:
		return renderInline(n.Content)
	}
}

// renderFlow writes nested blocks (a list item's or quote's) as one line.
func renderFlow(nodes []texturedoc.Node) string {
	parts := make([]string, 0, len(nodes))
	for _, n := range nodes {
		switch n.Type {
		case "paragraph", "heading":
			parts = append(parts, renderInline(n.Content))
		default:
			parts = append(parts, strings.ReplaceAll(renderBlock(n), "\n", " "))
		}
	}
	return strings.Join(parts, " ")
}

func renderInline(nodes []texturedoc.Node) string {
	var b strings.Builder
	for _, n := range nodes {
		switch n.Type {
		case "text":
			b.WriteString(n.Text)
		case "hard_break":
			b.WriteByte(' ')
		case "source_ref":
			b.WriteString(TokenText(tokenOf(n)))
		}
	}
	return b.String()
}

// tokenOf is the token a citation renders as.
func tokenOf(n texturedoc.Node) Token {
	if ref, ok := RefOf(n); ok {
		kind := TokenCite
		if stringAttr(n, "display_mode") == "expanded_ref" {
			kind = TokenEmbed
		}
		return Token{Kind: kind, Handle: ref.Doc, Quote: ref.Quote}
	}
	return Token{Kind: TokenRef, Handle: stringAttr(n, "source_entity_id")}
}

// RefOf reads the Ref a Write citation carries.
func RefOf(n texturedoc.Node) (types.Ref, bool) {
	if n.Type != "source_ref" || n.Attrs == nil {
		return types.Ref{}, false
	}
	raw, ok := n.Attrs[refAttr]
	if !ok || raw == nil {
		return types.Ref{}, false
	}
	var ref types.Ref
	switch v := raw.(type) {
	case types.Ref:
		ref = v
	default:
		data, err := json.Marshal(v)
		if err != nil || json.Unmarshal(data, &ref) != nil {
			return types.Ref{}, false
		}
	}
	return ref, ref.Doc != ""
}

// sourceRefs lists a node's citation nodes in document order.
func sourceRefs(n texturedoc.Node) []texturedoc.Node {
	var out []texturedoc.Node
	var walk func(texturedoc.Node)
	walk = func(n texturedoc.Node) {
		if n.Type == "source_ref" {
			out = append(out, n)
		}
		for _, c := range n.Content {
			walk(c)
		}
	}
	walk(n)
	return out
}

// plainText is a node's text without citations, for quote checks.
func plainText(n texturedoc.Node) string {
	var b strings.Builder
	var walk func(texturedoc.Node)
	walk = func(n texturedoc.Node) {
		switch n.Type {
		case "text":
			b.WriteString(n.Text)
		case "hard_break":
			b.WriteByte(' ')
		}
		for i, c := range n.Content {
			if i > 0 && isBlockType(c.Type) {
				b.WriteByte(' ')
			}
			walk(c)
		}
	}
	walk(n)
	return b.String()
}

func isBlockType(t string) bool {
	switch t {
	case "text", "hard_break", "source_ref":
		return false
	}
	return true
}

func nodeID(n texturedoc.Node) string { return stringAttr(n, "id") }

func stringAttr(n texturedoc.Node, key string) string {
	if n.Attrs == nil {
		return ""
	}
	if v, ok := n.Attrs[key].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

func intAttr(n texturedoc.Node, key string) int {
	if n.Attrs == nil {
		return 0
	}
	switch v := n.Attrs[key].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	}
	return 0
}

// normalizeText collapses whitespace, the comparison @all aligns blocks by.
func normalizeText(s string) string { return strings.Join(strings.Fields(s), " ") }

// ChangedBlocks names the blocks of head that are new or changed since
// parent, for the long-document frame.
func ChangedBlocks(head, parent texturedoc.StructuredTextureDoc) map[string]bool {
	before := map[string]string{}
	for _, b := range DocBlocks(parent) {
		before[b.ID] = b.Text
	}
	out := map[string]bool{}
	for _, b := range DocBlocks(head) {
		if text, ok := before[b.ID]; !ok || text != b.Text {
			out[b.ID] = true
		}
	}
	return out
}
