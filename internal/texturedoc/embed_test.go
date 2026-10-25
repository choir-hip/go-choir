package texturedoc

// The embed block (docs/design/texture-evidence-embeds-te-2026-10-25.md
// section 1). Ways it could fail, written before the code:
//   - an embed is accepted inside a list item or a quote, where the Write
//     frame and the renderers do not address it;
//   - it carries content, text or marks;
//   - a writer chooses its privacy (attrs.privacy), or stores an unknown
//     kind, an unknown attribute or a reserved one (selector, poster);
//   - a capsule_evidence embed names a malformed digest, no operation, a
//     route off the origin, or no viewport, or a viewport out of bounds;
//   - caption and alt skip the authoring rules or are unbounded;
//   - a world document holds one;
//   - its projection is empty, so the revision's content loses it;
//   - capsule_evidence or image reads as publishable.

import (
	"strings"
	"testing"
)

const testDigest = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func evidenceNode(mutate func(attrs map[string]any, target map[string]any)) Node {
	target := map[string]any{
		"digest": testDigest, "operation_id": "selfdev-op-1", "computer_id": "computer-1",
		"route": "/reader", "width": float64(1280), "height": float64(800), "blank": false,
	}
	attrs := map[string]any{
		"id": "embed-1", "kind": EmbedKindCapsuleEvidence, "target": target,
		"caption": "Screenshot of /reader from candidate selfdev-op-1 (1280x800)", "alt": "Screenshot of /reader",
	}
	if mutate != nil {
		mutate(attrs, target)
	}
	return Node{Type: "embed", Attrs: attrs}
}

func docOf(blocks ...Node) StructuredTextureDoc {
	return StructuredTextureDoc{Schema: SchemaV1, Doc: Node{Type: "doc", Attrs: map[string]any{"id": "doc-1"}, Content: blocks}}
}

func para(id, text string) Node {
	return Node{Type: "paragraph", Attrs: map[string]any{"id": id}, Content: []Node{{Type: "text", Text: text}}}
}

func TestEmbedBlockValidates(t *testing.T) {
	image := Node{Type: "embed", Attrs: map[string]any{"id": "embed-2", "kind": EmbedKindImage, "target": map[string]any{"digest": testDigest}, "alt": "A diagram"}}
	doc := docOf(para("p1", "Candidate evidence."), evidenceNode(nil), image)
	if err := Validate(doc, nil); err != nil {
		t.Fatalf("valid embeds refused: %v", err)
	}
	projection, err := Project(doc, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := "Candidate evidence.\n\n[evidence: Screenshot of /reader from candidate selfdev-op-1 (1280x800)]\n\n[image: A diagram]"
	if projection.Text != want {
		t.Fatalf("projection = %q, want %q", projection.Text, want)
	}
	embeds := Embeds(doc)
	if len(embeds) != 2 || embeds[0].Digest != testDigest || embeds[0].OperationID != "selfdev-op-1" || embeds[0].Width != 1280 || embeds[0].Route != "/reader" {
		t.Fatalf("Embeds = %+v", embeds)
	}
	for _, e := range embeds {
		if e.Privacy() != EmbedPrivacyPrivate || e.Publishable() {
			t.Fatalf("%s embed reads as publishable: privacy %q", e.Kind, e.Privacy())
		}
	}
	if got := PrivateEmbeds(doc); len(got) != 2 {
		t.Fatalf("PrivateEmbeds = %d, want 2", len(got))
	}
	if got := EmbeddedDigests(doc); len(got) != 1 || got[0] != testDigest {
		t.Fatalf("EmbeddedDigests = %v, want the one digest once", got)
	}
}

func TestEmbedBlockRefusals(t *testing.T) {
	cases := map[string]StructuredTextureDoc{
		"nested in a list": docOf(Node{Type: "bullet_list", Attrs: map[string]any{"id": "l1"}, Content: []Node{
			{Type: "list_item", Attrs: map[string]any{"id": "li1"}, Content: []Node{evidenceNode(nil)}},
		}}),
		"nested in a quote": docOf(Node{Type: "blockquote", Attrs: map[string]any{"id": "q1"}, Content: []Node{evidenceNode(nil)}}),
		"no id":             docOf(evidenceNode(func(a, _ map[string]any) { delete(a, "id") })),
		"content":           docOf(func() Node { n := evidenceNode(nil); n.Content = []Node{{Type: "text", Text: "x"}}; return n }()),
		"text":              docOf(func() Node { n := evidenceNode(nil); n.Text = "x"; return n }()),
		"marks":             docOf(func() Node { n := evidenceNode(nil); n.Marks = []Mark{{Type: "strong"}}; return n }()),
		"privacy written":   docOf(evidenceNode(func(a, _ map[string]any) { a["privacy"] = "public" })),
		"unknown kind":      docOf(evidenceNode(func(a, _ map[string]any) { a["kind"] = "video" })),
		"no kind":           docOf(evidenceNode(func(a, _ map[string]any) { delete(a, "kind") })),
		"unknown attr":      docOf(evidenceNode(func(a, _ map[string]any) { a["style"] = "x" })),
		"reserved selector": docOf(evidenceNode(func(a, _ map[string]any) { a["selector"] = map[string]any{} })),
		"target not object": docOf(evidenceNode(func(a, _ map[string]any) { a["target"] = "sha256:" + testDigest })),
		"unknown target":    docOf(evidenceNode(func(_, tg map[string]any) { tg["bundle_digest"] = testDigest })),
		"uppercase digest":  docOf(evidenceNode(func(_, tg map[string]any) { tg["digest"] = strings.ToUpper(testDigest) })),
		"short digest":      docOf(evidenceNode(func(_, tg map[string]any) { tg["digest"] = testDigest[:63] })),
		"prefixed digest":   docOf(evidenceNode(func(_, tg map[string]any) { tg["digest"] = "sha256:" + testDigest })),
		"no operation":      docOf(evidenceNode(func(_, tg map[string]any) { delete(tg, "operation_id") })),
		"spaced operation":  docOf(evidenceNode(func(_, tg map[string]any) { tg["operation_id"] = "op 1" })),
		"no computer":       docOf(evidenceNode(func(_, tg map[string]any) { delete(tg, "computer_id") })),
		"absolute route":    docOf(evidenceNode(func(_, tg map[string]any) { tg["route"] = "https://evil.example/" })),
		"no route":          docOf(evidenceNode(func(_, tg map[string]any) { delete(tg, "route") })),
		"no width":          docOf(evidenceNode(func(_, tg map[string]any) { delete(tg, "width") })),
		"zero height":       docOf(evidenceNode(func(_, tg map[string]any) { tg["height"] = float64(0) })),
		"huge width":        docOf(evidenceNode(func(_, tg map[string]any) { tg["width"] = float64(MaxEmbedDimension + 1) })),
		"fraction width":    docOf(evidenceNode(func(_, tg map[string]any) { tg["width"] = 12.5 })),
		"blank not bool":    docOf(evidenceNode(func(_, tg map[string]any) { tg["blank"] = "yes" })),
		"no caption":        docOf(evidenceNode(func(a, _ map[string]any) { delete(a, "caption") })),
		"no alt":            docOf(evidenceNode(func(a, _ map[string]any) { a["alt"] = "  " })),
		"legacy citation":   docOf(evidenceNode(func(a, _ map[string]any) { a["caption"] = "Screenshot [3]" })),
		"long caption":      docOf(evidenceNode(func(a, _ map[string]any) { a["caption"] = strings.Repeat("x", MaxEmbedCaptionRunes+1) })),
		"image one dim": docOf(Node{Type: "embed", Attrs: map[string]any{"id": "e", "kind": EmbedKindImage, "alt": "a",
			"target": map[string]any{"digest": testDigest, "width": float64(10)}}}),
		"image operation": docOf(Node{Type: "embed", Attrs: map[string]any{"id": "e", "kind": EmbedKindImage, "alt": "a",
			"target": map[string]any{"digest": testDigest, "operation_id": "op"}}}),
	}
	cases["duplicate embed id"] = docOf(evidenceNode(nil), evidenceNode(nil))
	cases["duplicate id, other target"] = docOf(evidenceNode(nil), evidenceNode(func(_, tg map[string]any) { tg["route"] = "/other" }))
	for name, doc := range cases {
		if err := Validate(doc, nil); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	world := docOf(evidenceNode(nil))
	world.Schema = WorldSchemaV1
	if err := ValidateWorld(world); err == nil {
		t.Error("a world document held an embed")
	}
}
