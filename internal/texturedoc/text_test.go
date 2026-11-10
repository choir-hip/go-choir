package texturedoc

import (
	"strings"
	"testing"
)

// Adjacent citations of one source project as one marker, while every anchor
// stays in refs; a marker after prose keeps its own. Ways it can fail: "[1] [1]"
// survives; the second anchor is lost from refs; "[1]" after prose is swallowed.
func TestAdjacentCitationsOfOneSourceProjectAsOneMarker(t *testing.T) {
	entities := validEntities()[:1]
	doc := validDoc()
	ref := func(id string) Node {
		return Node{Type: "source_ref", Attrs: map[string]any{"id": id, "source_entity_id": "src-web", "display_mode": "numbered_ref"}}
	}
	text := func(s string) Node { return Node{Type: "text", Text: s} }
	doc.Doc.Content = []Node{{Type: "paragraph", Attrs: map[string]any{"id": "p1"}, Content: []Node{
		text("The council voted "), ref("r1"), text(" "), ref("r2"), text(" and the mayor agreed. "), ref("r3"),
	}}}
	p, err := Project(doc, entities)
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	if strings.Contains(p.Text, "[1] [1]") {
		t.Errorf("doubled marker survived: %q", p.Text)
	}
	if got := strings.Count(p.Text, "[1]"); got != 2 {
		t.Errorf("markers = %d in %q, want 2 (one merged pair, one after prose)", got, p.Text)
	}
	if len(p.SourceRefs) != 3 {
		t.Errorf("refs = %d, want all 3 anchors kept", len(p.SourceRefs))
	}
}
