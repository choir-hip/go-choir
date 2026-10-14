package texturedoc

import (
	"encoding/json"
	"testing"
)

// Headline is what a story is titled by and what its publish requires.
// Ways it can fail, written first:
//   - a body that leads with a paragraph (a V1 acknowledgment, a brief)
//     yields a headline, so the brief or the plan becomes the title;
//   - a later section heading ("Background") is taken as the headline;
//   - marks split the heading into several text nodes and the headline
//     loses or doubles spaces ("Cisco andNvidia");
//   - a citation inside the heading leaks its quote or id into the title;
//   - an empty leading block (an empty paragraph) hides the heading under it;
//   - an empty heading yields "" but counts as present.
func TestHeadline(t *testing.T) {
	body := func(blocks ...Node) json.RawMessage {
		raw, err := json.Marshal(StructuredTextureDoc{Schema: "x", Doc: Node{Type: "doc", Content: blocks}})
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	text := func(s string, marks ...string) Node {
		n := Node{Type: "text", Text: s}
		for _, m := range marks {
			n.Marks = append(n.Marks, Mark{Type: m})
		}
		return n
	}
	heading := func(level int, inline ...Node) Node {
		return Node{Type: "heading", Attrs: map[string]any{"level": level}, Content: inline}
	}
	para := func(inline ...Node) Node { return Node{Type: "paragraph", Content: inline} }

	cases := []struct {
		name string
		raw  json.RawMessage
		want string
	}{
		{"leading h1", body(heading(1, text("Uber begins robotaxi trial in Manchester")), para(text("Body."))), "Uber begins robotaxi trial in Manchester"},
		{"leading h2", body(heading(2, text("CBS editor warned staff")), para(text("Body."))), "CBS editor warned staff"},
		{"marks keep spacing", body(heading(1, text("Cisco and "), text("Nvidia", "strong"), text(" team up")), para(text("Body."))), "Cisco and Nvidia team up"},
		{"hard break is a space", body(heading(1, text("Line one"), Node{Type: "hard_break"}, text("line two")), para(text("x"))), "Line one line two"},
		{"citation adds nothing", body(heading(1, text("Court clears merger "), Node{Type: "source_ref", Attrs: map[string]any{"source_entity_id": "se-1"}}), para(text("x"))), "Court clears merger"},
		{"whitespace collapsed", body(heading(1, text("  Harbor \n reopens  "))), "Harbor reopens"},
		{"empty paragraph before heading", body(para(), heading(1, text("Harbor reopens")), para(text("x"))), "Harbor reopens"},
		{"leads with a paragraph", body(para(text("This document will explain the dispute.")), heading(2, text("Background"))), ""},
		{"empty heading then paragraph", body(heading(1), para(text("The owner wants an account."))), ""},
		{"list first", body(Node{Type: "bullet_list", Content: []Node{{Type: "list_item", Content: []Node{para(text("item"))}}}}), ""},
		{"empty body", body(), ""},
		{"not json", json.RawMessage(`{`), ""},
		{"nil", nil, ""},
	}
	for _, c := range cases {
		if got := Headline(c.raw); got != c.want {
			t.Errorf("%s: Headline = %q, want %q", c.name, got, c.want)
		}
	}
}
