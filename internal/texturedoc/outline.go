package texturedoc

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Outline renders a structured document body as one line per node: type,
// id, source attributes, heading level and the opening of its text. It is
// what Texture addresses edits by (block_id, after_block_id, node_id). The
// run prompt and the cell's choir.ReadDoc() both use it. maxNodes and
// maxBytes bound it (zero means unbounded); a bounded outline says when it
// stops early.
func Outline(raw json.RawMessage, maxNodes, maxBytes int) string {
	if len(strings.TrimSpace(string(raw))) == 0 {
		return ""
	}
	var doc StructuredTextureDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		return ""
	}
	var b strings.Builder
	count := 0
	stopped := appendOutline(&b, doc.Doc, 0, &count, maxNodes)
	out := strings.TrimSpace(b.String())
	if maxBytes > 0 && len(out) > maxBytes {
		out = out[:maxBytes]
		stopped = true
	}
	if stopped {
		out += "\n- ... outline truncated"
	}
	return out
}

func appendOutline(b *strings.Builder, node Node, depth int, count *int, maxNodes int) bool {
	if node.Type != "doc" {
		if maxNodes > 0 && *count >= maxNodes {
			return true
		}
		*count++
		b.WriteString(strings.Repeat("  ", depth))
		b.WriteString("- ")
		b.WriteString(node.Type)
		for _, key := range []string{"id", "source_entity_id", "display_mode"} {
			if value := stringAttr(node, key); value != "" {
				b.WriteString(" " + key + "=" + value)
			}
		}
		if node.Type == "heading" {
			if level, ok := node.Attrs["level"]; ok {
				b.WriteString(" level=" + fmt.Sprint(level))
			}
		}
		if preview := textPreview(node, 120); preview != "" {
			encoded, _ := json.Marshal(preview)
			b.WriteString(" text=" + string(encoded))
		}
		b.WriteString("\n")
	}
	for _, child := range node.Content {
		if appendOutline(b, child, depth+1, count, maxNodes) {
			return true
		}
	}
	return false
}

func textPreview(node Node, max int) string {
	var b strings.Builder
	collectText(&b, node)
	text := strings.Join(strings.Fields(b.String()), " ")
	if len(text) > max {
		text = strings.TrimSpace(text[:max]) + "..."
	}
	return text
}

func collectText(b *strings.Builder, node Node) {
	if node.Type == "text" && node.Text != "" {
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(node.Text)
	}
	for _, child := range node.Content {
		collectText(b, child)
	}
}
