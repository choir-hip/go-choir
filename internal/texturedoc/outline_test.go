package texturedoc

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// The outline names every block's id, type and opening text, so a Texture
// cell can address an edit without guessing ids
// (docs/archive/problems/texture-probes-canonical-write-to-learn-schema-2026-10-11.md).
// With no node bound, a long document is listed in full.
func TestOutlineListsEveryBlockID(t *testing.T) {
	doc := StructuredTextureDoc{Schema: "texture.doc.v1", Doc: Node{Type: "doc"}}
	doc.Doc.Content = append(doc.Doc.Content, Node{Type: "heading", Attrs: map[string]any{"id": "h-1", "level": 1}, Content: []Node{{Type: "text", Text: "AI news"}}})
	for i := 0; i < 100; i++ {
		doc.Doc.Content = append(doc.Doc.Content, Node{Type: "paragraph", Attrs: map[string]any{"id": fmt.Sprintf("p-%d", i)}, Content: []Node{{Type: "text", Text: fmt.Sprintf("item %d", i)}}})
	}
	raw, _ := json.Marshal(doc)

	full := Outline(raw, 0, 0)
	for _, want := range []string{"heading id=h-1 level=1 text=\"AI news\"", "paragraph id=p-0 text=\"item 0\"", "paragraph id=p-99 text=\"item 99\""} {
		if !strings.Contains(full, want) {
			t.Fatalf("outline missing %q:\n%s", want, full)
		}
	}
	bounded := Outline(raw, 64, 4000)
	if strings.Contains(bounded, "id=p-99") || !strings.Contains(bounded, "truncated") {
		t.Fatalf("a bounded outline must stop at its node bound and say so:\n%s", bounded[len(bounded)-200:])
	}
	if Outline(nil, 0, 0) != "" || Outline(json.RawMessage(`not json`), 0, 0) != "" {
		t.Fatalf("an absent or unreadable body has no outline")
	}
}
