package texturedoc

// MergeMarkdown (merge_markdown.go): the owner's typed text plus the parent revision become the new body_doc, keeping untouched nodes exactly. Failure list, written before the code:
//   M1  the phone's fixture (testdata/merge_markdown.json: head, text, the body the phone wrote, content from Project): the server writes the same body shape and projects to the same content;
//   M2  an unchanged text returns the parent as it is (every node, byte for byte);
//   M3  an edit next to an embed leaves ONE embed and no marker paragraph (round 4 blocking 1: it grew by one line per save);
//   M4  an edit of a cited paragraph keeps its source_ref and the entity, drops the typed "[1]" (round 4 blocking 2, round 5 blocking 2: a stored text that differs from the field's);
//   M5  a paragraph typed where an embed's line was is KEPT and the embed goes (round 5 blocking 1: the paragraph was dropped and the embed came back);
//   M6  a changed embed line deletes the embed, it is never altered (the route refuses an altered embed);
//   M7  a deleted cited paragraph leaves its entity out (Validate refuses an entity nothing cites);
//   M8  saving twice does not grow the body: the second save of the first save's content is the first save's body;
//   M9  every result is valid for the server (Validate with the returned entities) and its projection is the text typed, for text without markup;
//   M10 one newline is a hard_break, a blank line a new paragraph; a code fence keeps its language; an empty parent body takes the text whole; no parent builds a fresh document.

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

func mdStripIDs(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := map[string]any{}
		for k, e := range x {
			if k == "id" {
				out[k] = "#"
			} else {
				out[k] = mdStripIDs(e)
			}
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = mdStripIDs(e)
		}
		return out
	}
	return v
}

func mdGeneric(t *testing.T, doc StructuredTextureDoc) any {
	t.Helper()
	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatal(err)
	}
	return mdStripIDs(v)
}

func mdPara(id string, content ...Node) Node {
	return Node{Type: "paragraph", Attrs: map[string]any{"id": id}, Content: content}
}
func mdText(s string) Node { return Node{Type: "text", Text: s} }
func mdRef(id, entity string) Node {
	return Node{Type: "source_ref", Attrs: map[string]any{"id": id, "source_entity_id": entity, "display_mode": "numbered_ref"}}
}
func mdEmbed(id, title string) Node {
	return Node{Type: "embed", Attrs: map[string]any{"id": id, "kind": "file", "title": title, "target": map[string]any{"path": "Files/" + title, "mime": "application/pdf"}}}
}
func mdEntity(id, title string) SourceEntity {
	return SourceEntity{SourceEntityID: id, Target: SourceTarget{Kind: "web_url", URI: "https://example.net/" + id}, Selectors: []SourceSelector{{Kind: "whole_resource"}},
		Display: SourceDisplay{Mode: "numbered_ref", Title: title}, Evidence: SourceEvidence{State: "available", OpenSurface: "source"}, Provenance: SourceEntityProvenance{CreatedBy: "test"}}
}
func mdDoc(content ...Node) StructuredTextureDoc {
	return StructuredTextureDoc{Schema: SchemaV1, Doc: Node{Type: "doc", Attrs: map[string]any{"id": "d"}, Content: content}}
}

// the cited and embedded document of the review: Intro / a cited paragraph / an embed / Last
func mdCited() (StructuredTextureDoc, []SourceEntity, string) {
	doc := mdDoc(mdPara("p1", mdText("Intro.")), mdPara("p2", mdText("The claim "), mdRef("r1", "src_a"), mdText(".")), mdEmbed("e1", "report.pdf"), mdPara("p3", mdText("Last.")))
	ents := []SourceEntity{mdEntity("src_a", "Report A")}
	p, err := Project(doc, ents)
	if err != nil {
		panic(err)
	}
	return doc, ents, p.Text
}

func mdCheck(t *testing.T, doc StructuredTextureDoc, ents []SourceEntity) string {
	t.Helper()
	p, err := Project(doc, ents)
	if err != nil {
		t.Fatalf("the server refuses the merged body: %v\n%s", err, mdJSON(doc))
	}
	return p.Text
}
func mdJSON(v any) string { b, _ := json.Marshal(v); return string(b) }
func mdTypes(doc StructuredTextureDoc) []string {
	var out []string
	for _, n := range doc.Doc.Content {
		out = append(out, n.Type)
	}
	return out
}

func TestMergeMarkdownPhoneFixture(t *testing.T) { // M1
	raw, err := os.ReadFile("testdata/merge_markdown.json")
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		Merge []struct {
			Name        string                `json:"name"`
			Text        string                `json:"text"`
			HeadContent string                `json:"head_content"`
			HeadBody    *StructuredTextureDoc `json:"head_body"`
			BodyDoc     StructuredTextureDoc  `json:"body_doc"`
			Content     string                `json:"content"`
		} `json:"merge"`
	}
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	if len(f.Merge) < 11 {
		t.Fatalf("the fixture has %d merge cases", len(f.Merge))
	}
	for _, c := range f.Merge {
		var parent StructuredTextureDoc
		if c.HeadBody != nil {
			parent = *c.HeadBody
		}
		got, ents := MergeMarkdown(parent, nil, c.HeadContent, c.Text)
		if text := mdCheck(t, got, ents); text != c.Content {
			t.Errorf("%s: projects %q, the fixture says %q", c.Name, text, c.Content)
		}
		if !reflect.DeepEqual(mdGeneric(t, got), mdGeneric(t, c.BodyDoc)) {
			t.Errorf("%s: the body differs from the phone's:\n server %s\n phone  %s", c.Name, mdJSON(mdGeneric(t, got)), mdJSON(mdGeneric(t, c.BodyDoc)))
		}
	}
}

func TestMergeMarkdownKeepsWhatTheTextDidNotTouch(t *testing.T) {
	doc, ents, text := mdCited()
	// M2
	got, gents := MergeMarkdown(doc, ents, text, text)
	if !reflect.DeepEqual(got, doc) || len(gents) != 1 {
		t.Errorf("an unchanged text changed the document: %s", mdJSON(got))
	}
	// M3
	edited := strings.Replace(text, "Last.", "Last, edited.", 1)
	got, gents = MergeMarkdown(doc, ents, text, edited)
	if want := []string{"paragraph", "paragraph", "embed", "paragraph"}; !reflect.DeepEqual(mdTypes(got), want) {
		t.Errorf("M3 node types %v, want %v", mdTypes(got), want)
	}
	if mdCheck(t, got, gents) != edited || !reflect.DeepEqual(got.Doc.Content[2], doc.Doc.Content[2]) || !reflect.DeepEqual(got.Doc.Content[1], doc.Doc.Content[1]) {
		t.Errorf("M3 the embed and the cited paragraph are not as they were: %s", mdJSON(got))
	}
	// M4
	edited = strings.Replace(text, "The claim [1].", "The claim, reworded [1].", 1)
	got, gents = MergeMarkdown(doc, ents, text, edited)
	if len(gents) != 1 || !hasSourceRef(got.Doc.Content[1]) || strings.Contains(nodePlainText(got.Doc.Content[1]), "[1]") || !strings.Contains(nodePlainText(got.Doc.Content[1]), "reworded") {
		t.Errorf("M4 the cited paragraph lost its ref or kept a typed marker: %s", mdJSON(got.Doc.Content[1]))
	}
	if got.Doc.Content[1].Attrs["id"] != "p2" {
		t.Errorf("M4 the edited paragraph lost its id")
	}
	mdCheck(t, got, gents)
	// M5
	edited = strings.Replace(text, "[file: report.pdf]", "New text typed here.", 1)
	got, gents = MergeMarkdown(doc, ents, text, edited)
	if want := []string{"paragraph", "paragraph", "paragraph", "paragraph"}; !reflect.DeepEqual(mdTypes(got), want) || nodePlainText(got.Doc.Content[2]) != "New text typed here." {
		t.Errorf("M5 the typed paragraph was dropped or the embed came back: %s", mdJSON(mdTypes(got)))
	}
	mdCheck(t, got, gents)
	// M6
	edited = strings.Replace(text, "[file: report.pdf]", "[file: renamed.pdf]", 1)
	got, gents = MergeMarkdown(doc, ents, text, edited)
	for _, n := range got.Doc.Content {
		if n.Type == "embed" {
			t.Errorf("M6 an embed whose line changed is still there: %s", mdJSON(n))
		}
	}
	mdCheck(t, got, gents)
	// M7
	edited = strings.Replace(text, "The claim [1].\n\n", "", 1)
	got, gents = MergeMarkdown(doc, ents, text, edited)
	if len(gents) != 0 {
		t.Errorf("M7 an entity nothing cites is kept: %v", gents)
	}
	mdCheck(t, got, gents)
}

func TestMergeMarkdownTwiceDoesNotGrow(t *testing.T) { // M8
	doc, ents, text := mdCited()
	edited := strings.Replace(text, "Last.", "Last, edited.", 1)
	first, fents := MergeMarkdown(doc, ents, text, edited)
	second, sents := MergeMarkdown(first, fents, mdCheck(t, first, fents), edited)
	if !reflect.DeepEqual(mdGeneric(t, first), mdGeneric(t, second)) || len(sents) != len(fents) {
		t.Errorf("M8 a second save changed the body:\n first  %s\n second %s", mdJSON(first), mdJSON(second))
	}
	third, tents := MergeMarkdown(second, sents, mdCheck(t, second, sents), strings.Replace(edited, "edited.", "edited twice.", 1))
	if want := []string{"paragraph", "paragraph", "embed", "paragraph"}; !reflect.DeepEqual(mdTypes(third), want) {
		t.Errorf("M8 types after three saves %v", mdTypes(third))
	}
	mdCheck(t, third, tents)
}

func TestMergeMarkdownShapes(t *testing.T) { // M9, M10
	nodes := func(doc StructuredTextureDoc) []string { return mdTypes(doc) }
	owner := "Hello\nThis is my first time using texture editing on autoradio 1"
	got, ents := MergeMarkdown(StructuredTextureDoc{}, nil, "", owner)
	if mdCheck(t, got, ents) != owner || len(got.Doc.Content) != 1 || len(got.Doc.Content[0].Content) != 3 || got.Doc.Content[0].Content[1].Type != "hard_break" {
		t.Errorf("M10 the owner's text: %s", mdJSON(got))
	}
	two, ents := MergeMarkdown(StructuredTextureDoc{}, nil, "", "One.\n\nTwo.")
	if !reflect.DeepEqual(nodes(two), []string{"paragraph", "paragraph"}) || mdCheck(t, two, ents) != "One.\n\nTwo." {
		t.Errorf("M10 two paragraphs: %s", mdJSON(two))
	}
	code, ents := MergeMarkdown(StructuredTextureDoc{}, nil, "", "```go\na := 1\n```")
	if mdCheck(t, code, ents) != "```go\na := 1\n```" {
		t.Errorf("M10 the code fence lost its language: %s", mdJSON(code))
	}
	empty, ents := MergeMarkdown(mdDoc(mdPara("p0")), nil, "", "Hello there.")
	if len(empty.Doc.Content) != 1 || mdCheck(t, empty, ents) != "Hello there." {
		t.Errorf("M10 an empty parent body must be taken whole: %s", mdJSON(empty))
	}
	rich := "# Title\n\n- a\n- b\n\n1. x\n2. y\n\n> q1\n> q2\n\n---\n\nplain"
	r, ents := MergeMarkdown(StructuredTextureDoc{}, nil, "", rich)
	if mdCheck(t, r, ents) != rich {
		t.Errorf("M9 a rich text does not project to itself: %q", mdCheck(t, r, ents))
	}
}
