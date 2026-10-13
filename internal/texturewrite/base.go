package texturewrite

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/texturedoc"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// DroppedMetadataKey is the revision metadata key that lists the citations a
// Write dropped; the next frame shows them.
const DroppedMetadataKey = "texture_dropped_citations"

// Base is a revision as a Write applies to it: its structured body and
// source entities. A revision with no structured body (an early owner
// draft) reads as markdown blocks of its content, with ids derived from the
// revision, so the frame and the commit see the same blocks.
func Base(rev types.Revision) (texturedoc.StructuredTextureDoc, []texturedoc.SourceEntity, error) {
	var entities []texturedoc.SourceEntity
	if raw := strings.TrimSpace(string(rev.SourceEntities)); raw != "" && raw != "null" {
		if err := json.Unmarshal(rev.SourceEntities, &entities); err != nil {
			return texturedoc.StructuredTextureDoc{}, nil, fmt.Errorf("revision %s source entities: %w", rev.RevisionID, err)
		}
	}
	if raw := strings.TrimSpace(string(rev.BodyDoc)); raw != "" && raw != "null" {
		var doc texturedoc.StructuredTextureDoc
		if err := json.Unmarshal(rev.BodyDoc, &doc); err != nil {
			return texturedoc.StructuredTextureDoc{}, nil, fmt.Errorf("revision %s body: %w", rev.RevisionID, err)
		}
		if !plainTextBody(doc) {
			return doc, entities, nil
		}
	}
	return PlainDoc(rev.DocID, rev.RevisionID, rev.Content), entities, nil
}

// plainTextBody reports the body the store derives for owner text: one
// paragraph of text lines joined by hard breaks. Read as markdown, it is
// the blocks the owner wrote.
func plainTextBody(doc texturedoc.StructuredTextureDoc) bool {
	if len(doc.Doc.Content) != 1 || doc.Doc.Content[0].Type != "paragraph" {
		return false
	}
	breaks := 0
	for _, n := range doc.Doc.Content[0].Content {
		switch n.Type {
		case "hard_break":
			breaks++
		case "text":
			if len(n.Marks) > 0 {
				return false
			}
		default:
			return false
		}
	}
	return breaks > 0
}

// PlainDoc reads plain content as markdown blocks with no citations.
func PlainDoc(docID, revisionID, content string) texturedoc.StructuredTextureDoc {
	var errs Errors
	var lines []Line
	for i, l := range strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n") {
		lines = append(lines, Line{N: i + 1, Text: l})
	}
	body := parseBody(lines, &errs)
	seq := 0
	id := func(prefix string) string {
		seq++
		sum := sha256.Sum256([]byte(docID + "\x00" + revisionID + "\x00" + strconv.Itoa(seq)))
		return prefix + "-" + hex.EncodeToString(sum[:6])
	}
	text := func(ls []Line) []texturedoc.Node {
		joined := joinLines(ls)
		if joined == "" {
			return nil
		}
		return []texturedoc.Node{{Type: "text", Text: joined}}
	}
	var blocks []texturedoc.Node
	for _, b := range body {
		switch b.Kind {
		case "heading":
			blocks = append(blocks, texturedoc.Node{Type: "heading", Attrs: map[string]any{"id": id("h"), "level": b.Level}, Content: text(b.Lines)})
		case "bullet_list", "ordered_list":
			attrs := map[string]any{"id": id("list")}
			if b.Kind == "ordered_list" {
				attrs["start"] = b.Start
			}
			var items []texturedoc.Node
			for _, l := range b.Lines {
				items = append(items, texturedoc.Node{Type: "list_item", Attrs: map[string]any{"id": id("li")}, Content: []texturedoc.Node{
					{Type: "paragraph", Attrs: map[string]any{"id": id("p")}, Content: text([]Line{l})},
				}})
			}
			blocks = append(blocks, texturedoc.Node{Type: b.Kind, Attrs: attrs, Content: items})
		case "blockquote":
			blocks = append(blocks, texturedoc.Node{Type: "blockquote", Attrs: map[string]any{"id": id("quote")}, Content: []texturedoc.Node{
				{Type: "paragraph", Attrs: map[string]any{"id": id("p")}, Content: text(b.Lines)},
			}})
		case "code_block":
			raw := make([]string, len(b.Lines))
			for i, l := range b.Lines {
				raw[i] = l.Text
			}
			var content []texturedoc.Node
			if joined := strings.Join(raw, "\n"); joined != "" {
				content = []texturedoc.Node{{Type: "text", Text: joined}}
			}
			blocks = append(blocks, texturedoc.Node{Type: "code_block", Attrs: map[string]any{"id": id("code")}, Content: content})
		case "horizontal_rule":
			blocks = append(blocks, texturedoc.Node{Type: "horizontal_rule", Attrs: map[string]any{"id": id("hr")}})
		default:
			blocks = append(blocks, texturedoc.Node{Type: "paragraph", Attrs: map[string]any{"id": id("p")}, Content: text(b.Lines)})
		}
	}
	return texturedoc.StructuredTextureDoc{Schema: texturedoc.SchemaV1, Doc: texturedoc.Node{
		Type: "doc", Attrs: map[string]any{"id": "doc-" + docID}, Content: blocks,
	}}
}

// DroppedFrom reads the citations a revision's Write dropped.
func DroppedFrom(rev types.Revision) []DroppedCitation {
	var meta map[string]json.RawMessage
	if json.Unmarshal(rev.Metadata, &meta) != nil {
		return nil
	}
	var out []DroppedCitation
	if raw, ok := meta[DroppedMetadataKey]; ok {
		_ = json.Unmarshal(raw, &out)
	}
	return out
}
