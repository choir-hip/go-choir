package texturewrite

import (
	"encoding/json"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/texturedoc"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// A publish-auto story publishes only revisions that cite a source
// (docs/problems/free-model-429-opens-provider-circuit-stub-stories-
// 2026-10-14.md). Ways CitesSource can fail: a revision with only a
// citation made before Write ({{ref}} to a source entity) reads as
// uncited and is held; a revision with no citation reads as cited; a
// source_ref naming no entity and carrying no Ref counts.
func TestCitesSource(t *testing.T) {
	paragraph := func(inline ...texturedoc.Node) texturedoc.Node {
		return texturedoc.Node{Type: "paragraph", Attrs: map[string]any{"id": "p-1"}, Content: append([]texturedoc.Node{{Type: "text", Text: "The harbor reopens."}}, inline...)}
	}
	revision := func(body texturedoc.Node, entities []texturedoc.SourceEntity) types.Revision {
		doc, _ := json.Marshal(texturedoc.StructuredTextureDoc{Schema: texturedoc.SchemaV1, Doc: texturedoc.Node{
			Type: "doc", Attrs: map[string]any{"id": "doc-1"}, Content: []texturedoc.Node{
				{Type: "heading", Attrs: map[string]any{"id": "h-1", "level": 1}, Content: []texturedoc.Node{{Type: "text", Text: "Harbor"}}},
				body,
			},
		}})
		rev := types.Revision{RevisionID: "rev-1", DocID: "doc-1", BodyDoc: doc}
		if entities != nil {
			rev.SourceEntities, _ = json.Marshal(entities)
		}
		return rev
	}
	writeCite := texturedoc.Node{Type: "source_ref", Attrs: map[string]any{"id": "ref-1", "source_entity_id": "se-1", refAttr: map[string]any{"doc": "cap-court", "version": "v1"}}}
	legacyRef := texturedoc.Node{Type: "source_ref", Attrs: map[string]any{"id": "ref-2", "source_entity_id": "se-legacy"}}
	dangling := texturedoc.Node{Type: "source_ref", Attrs: map[string]any{"id": "ref-3", "source_entity_id": "se-gone"}}

	for _, tc := range []struct {
		name string
		rev  types.Revision
		want bool
	}{
		{"no citation", revision(paragraph(), nil), false},
		{"Write citation of a capture", revision(paragraph(writeCite), nil), true},
		{"citation made before Write", revision(paragraph(legacyRef), []texturedoc.SourceEntity{{SourceEntityID: "se-legacy"}}), true},
		{"source_ref naming no entity", revision(paragraph(dangling), nil), false},
		{"plain owner text", types.Revision{RevisionID: "rev-2", DocID: "doc-1", Content: "# Harbor\n\nNo sources yet."}, false},
	} {
		got, err := CitesSource(tc.rev)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if got != tc.want {
			t.Fatalf("%s: CitesSource = %v, want %v", tc.name, got, tc.want)
		}
	}
}
