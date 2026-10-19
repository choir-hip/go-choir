package texturewrite

// {{embed}} printed a source's text as a block on the published page. The
// owner never asked for it and does not want it (2026-10-16): a story cites
// passages and never reprints its sources
// (docs/problems/published-embeds-republish-source-text-2026-10-16.md).
//
// Ways this could fail, written first:
//  1. a Write with {{embed}}, quoted or not, still stages a transclusion;
//  2. the refusal does not name its line or does not say what to write
//     instead;
//  3. a stored embed shows in the frame as {{embed}}, so the desk copies a
//     token that is now refused and cannot touch the block;
//  4. a block holding a stored embed, copied unchanged, loses or changes
//     the citation.
//
// A stored embed of a whole capture (no quote) grounds no sentence: it drew
// as a lone "1" under eight front-page headlines and counted its outlet
// toward the publish gate (docs/problems/articles-relay-outlets-instead-of-
// grounding-them-2026-10-19.md). Each Write now drops it. Ways that could
// fail:
//  5. it survives a Write in a block the Write did not touch;
//  6. the quoted stored embed is dropped with it, though it cites a passage;
//  7. a paragraph that held only the embed is left empty;
//  8. its source entity stays listed, so the reader still shows the source.

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/texturedoc"
)

// Failure modes 1 and 2.
func TestEmbedTokenIsRefusedWithACiteHint(t *testing.T) {
	for _, script := range []string{
		"@insert end\nFirst line.\n\n{{embed https://example.org/court}}",
		"@insert end\nFirst line.\n\nThe ruling {{embed https://example.org/court \"the merger may proceed\"}}.",
	} {
		_, err := Apply(context.Background(), Input{OwnerID: "owner", DocID: "doc", Seed: "rev-e",
			Base: baseDoc(heading("h1", 1, "Merger")), Script: script, Resolver: newFakeCaptures()})
		if err == nil {
			t.Fatalf("Write %q staged an embed", script)
		}
		if got := lineOf(t, err); got != 4 {
			t.Fatalf("refusal names line %d, want 4: %v", got, err)
		}
		if msg := err.Error(); !strings.Contains(msg, "{{cite") || !strings.Contains(msg, "exact words") {
			t.Fatalf("refusal does not say to cite the passage: %v", err)
		}
	}
}

// legacyEmbedBase is a document whose second block holds two citations
// stored as embeds before {{embed}} was retired: one quoted, one of the
// whole capture.
func legacyEmbedBase(t *testing.T) (texturedoc.StructuredTextureDoc, []texturedoc.SourceEntity) {
	t.Helper()
	res, err := Apply(context.Background(), Input{OwnerID: "owner", DocID: "doc", Seed: "rev-0",
		Base:     baseDoc(heading("h1", 1, "Merger"), paragraph("p1", "Opening line.")),
		Script:   "@replace b2\nThe court ruled {{cite https://example.org/court \"the merger may proceed\"}}; an appeal followed {{cite https://example.org/court \"The appeal was filed on Monday\"}}.",
		Resolver: newFakeCaptures()})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	refs := sourceRefs(res.Doc.Doc.Content[1])
	if len(refs) != 2 {
		t.Fatalf("seed refs = %d, want 2", len(refs))
	}
	for i, n := range refs {
		n.Attrs["display_mode"] = "expanded_ref"
		if i == 1 {
			ref, _ := RefOf(n)
			ref.Quote, ref.CID, ref.Hash, ref.Span = "", "", "", nil
			n.Attrs[refAttr] = refAttrValue(ref)
		}
	}
	return res.Doc, res.Entities
}

// Failure modes 3, 4 and 6.
func TestStoredEmbedsRoundTripWithoutTheEmbedToken(t *testing.T) {
	base, entities := legacyEmbedBase(t)
	blocks := DocBlocks(base)
	text := blocks[1].Text
	if strings.Contains(text, "{{embed") {
		t.Fatalf("frame shows a stored embed as {{embed}}: %q", text)
	}
	if !strings.Contains(text, `{{cite cap-court "the merger may proceed"}}`) || !strings.Contains(text, "{{ref ") {
		t.Fatalf("frame text = %q, want the quoted embed as a cite and the whole-capture embed as a ref", text)
	}
	before, _ := json.Marshal(base.Doc.Content[1])
	caps := newFakeCaptures()
	res, err := Apply(context.Background(), Input{OwnerID: "owner", DocID: "doc", Seed: "rev-1", Base: base, Entities: entities,
		Script: "@replace " + blocks[1].Alias + "\n" + text, Resolver: caps})
	if err != nil {
		t.Fatalf("copy unchanged: %v", err)
	}
	var want texturedoc.Node
	_ = json.Unmarshal(before, &want)
	want.Content = dropWholeCaptureEmbeds(want.Content)
	after, _ := json.Marshal(res.Doc.Doc.Content[1])
	if wantJSON, _ := json.Marshal(want); string(after) != string(wantJSON) {
		t.Fatalf("unchanged block changed beyond dropping the whole-capture embed:\nwant  %s\nafter %s", wantJSON, after)
	}
	if refs := sourceRefs(res.Doc.Doc.Content[1]); len(refs) != 1 || stringAttr(refs[0], "display_mode") != "expanded_ref" {
		t.Fatalf("refs after the Write = %+v; want only the quoted stored embed", refs)
	}
	if caps.calls != 0 {
		t.Fatalf("copying stored citations read the capture store %d times", caps.calls)
	}
}

// Failure modes 5, 7 and 8: a whole-capture embed alone in its paragraph,
// in a block the Write does not touch, goes with its paragraph and its
// source entity.
func TestWholeCaptureEmbedIsDroppedAtTheNextWrite(t *testing.T) {
	base, entities := legacyEmbedBase(t)
	refs := sourceRefs(base.Doc.Content[1])
	lone := texturedoc.Node{Type: "paragraph", Attrs: map[string]any{"id": "p-lone"}, Content: []texturedoc.Node{refs[1]}}
	base.Doc.Content = append([]texturedoc.Node{base.Doc.Content[0], lone}, base.Doc.Content[1:]...)
	base.Doc.Content[2].Content = dropWholeCaptureEmbeds(base.Doc.Content[2].Content)
	res, err := Apply(context.Background(), Input{OwnerID: "owner", DocID: "doc", Seed: "rev-2", Base: base, Entities: entities,
		Script: "@insert end\nA closing line.", Resolver: newFakeCaptures()})
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	for _, n := range res.Doc.Doc.Content {
		if nodeID(n) == "p-lone" {
			t.Fatalf("the paragraph that held only the embed is still there: %+v", n)
		}
		for _, r := range sourceRefs(n) {
			if ref, ok := RefOf(r); ok && ref.Quote == "" {
				t.Fatalf("a whole-capture embed survived the Write: %+v", r)
			}
		}
	}
	if len(res.Doc.Doc.Content) != 3 {
		t.Fatalf("blocks = %d; want heading, cited paragraph, closing line", len(res.Doc.Doc.Content))
	}
	if len(res.Entities) != 1 {
		t.Fatalf("entities = %+v; want only the quoted citation's source", res.Entities)
	}
}
