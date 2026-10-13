package texturedoc

import (
	"fmt"
	"sort"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/sourcefetch"
)

// WorldSchemaV1 names a world document: a capture's blocks in the same node
// shape as a Texture document (Cut 2 unit 2a). A world document holds the
// page's own words, so it accepts footnote markers such as "[12]" and keeps
// links as marks; Texture-authored documents (SchemaV1) still refuse both.
const WorldSchemaV1 = "choir.world_doc.v1"

// WorldDocument renders a capture's blocks as a flat world document whose
// root id is the capture id. A heading block is a heading node; every other
// block is a paragraph node whose attrs.kind names the block kind (list_item,
// table_row, note, ...). Each node's attrs.id is the block's cid, and links
// become link marks on the text they cover.
func WorldDocument(docID string, blocks []sourcefetch.Block) StructuredTextureDoc {
	content := make([]Node, 0, len(blocks))
	for _, b := range blocks {
		node := Node{Type: "paragraph", Attrs: map[string]any{"id": b.ID, "kind": b.Kind}}
		if b.Kind == sourcefetch.KindHeading {
			node.Type = "heading"
			level := b.Level
			if level < 1 || level > 6 {
				level = 2
			}
			node.Attrs["level"] = level
		}
		if b.Page > 0 {
			node.Attrs["page"] = b.Page
		}
		node.Content = worldInlines(b)
		content = append(content, node)
	}
	return StructuredTextureDoc{
		Schema: WorldSchemaV1,
		Doc:    Node{Type: "doc", Attrs: map[string]any{"id": docID}, Content: content},
	}
}

// worldInlines splits a block's text into text nodes at its links' edges.
// Overlapping links keep the first.
func worldInlines(b sourcefetch.Block) []Node {
	links := append([]sourcefetch.Link(nil), b.Links...)
	sort.SliceStable(links, func(i, j int) bool { return links[i].Start < links[j].Start })
	var out []Node
	pos := 0
	for _, l := range links {
		if l.Start < pos || l.End > len(b.Text) || l.Start >= l.End {
			continue
		}
		if l.Start > pos {
			out = append(out, Node{Type: "text", Text: b.Text[pos:l.Start]})
		}
		out = append(out, Node{Type: "text", Text: b.Text[l.Start:l.End], Marks: []Mark{{Type: "link", Attrs: map[string]any{"href": l.Href}}}})
		pos = l.End
	}
	if pos < len(b.Text) {
		out = append(out, Node{Type: "text", Text: b.Text[pos:]})
	}
	return out
}

// ValidateWorld checks a world document: the node shape Validate checks,
// every block carrying its id, link marks carrying an href, and no
// source_ref. The Texture authoring rules on text (no "[12]", no "Source:"
// lines) do not apply to a page's own words.
func ValidateWorld(doc StructuredTextureDoc) error {
	if strings.TrimSpace(doc.Schema) != WorldSchemaV1 {
		return fmt.Errorf("schema %q is not %q", doc.Schema, WorldSchemaV1)
	}
	if doc.Doc.Type != "doc" {
		return fmt.Errorf("root node type %q is not doc", doc.Doc.Type)
	}
	if nodeID(doc.Doc) == "" {
		return fmt.Errorf("doc attrs.id is required")
	}
	v := &Validator{entities: map[string]SourceEntity{}, refs: map[string]bool{}, unused: map[string]bool{}, world: true}
	for i := range doc.Doc.Content {
		if err := v.validateBlock(doc.Doc.Content[i], fmt.Sprintf("doc.content[%d]", i)); err != nil {
			return err
		}
	}
	return nil
}
