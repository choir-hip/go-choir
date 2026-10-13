package textureowner

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/sourcecontract"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/texturedoc"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestTextureToolSourceGraphUsesTargetIdentityNotGeneratedLegacyID(t *testing.T) {
	rev := types.Revision{
		RevisionID: "rev-source-graph-identity",
		DocID:      "doc-source-graph-identity",
		OwnerID:    "user-1",
		CreatedAt:  time.Now().UTC().Truncate(time.Millisecond),
	}
	entity := texturedoc.SourceEntity{
		SourceEntityID: "src_model_generated_opaque",
		Target:         texturedoc.SourceTarget{Kind: "web_url", URI: "https://example.com/grounded-story"},
		Selectors:      []texturedoc.SourceSelector{{Kind: sourcecontract.SelectorKindWholeResource}},
		Display:        texturedoc.SourceDisplay{Mode: "numbered_ref", Title: "Grounded story"},
		Evidence:       texturedoc.SourceEvidence{State: sourcecontract.EvidenceStateAvailable, OpenSurface: sourcecontract.OpenSurfaceSource},
		Provenance:     texturedoc.SourceEntityProvenance{CreatedBy: "runtime", SourceSystem: "test"},
	}
	sourceEntities, err := json.Marshal([]texturedoc.SourceEntity{entity})
	if err != nil {
		t.Fatalf("marshal source entities: %v", err)
	}
	graph, err := textureToolSourceGraphWriteSet(rev, materializedTextureEdit{SourceEntities: sourceEntities}, &types.RunRecord{RunID: "run-source-graph"})
	if err != nil {
		t.Fatalf("textureToolSourceGraphWriteSet: %v", err)
	}
	if len(graph.SourceEntities) != 1 {
		t.Fatalf("source graph entities len = %d, want 1", len(graph.SourceEntities))
	}
	expectedCanonicalID, err := store.BuildTextureSourceEntityCanonicalID(rev.OwnerID, rev.OwnerID, "web_url", "https://example.com/grounded-story")
	if err != nil {
		t.Fatalf("BuildTextureSourceEntityCanonicalID: %v", err)
	}
	got := graph.SourceEntities[0]
	if got.CanonicalID != expectedCanonicalID {
		t.Fatalf("canonical_id = %q, want target-derived %q", got.CanonicalID, expectedCanonicalID)
	}
	if got.LegacySourceEntityID != "src_model_generated_opaque" {
		t.Fatalf("legacy_source_entity_id = %q, want compatibility id", got.LegacySourceEntityID)
	}
	if strings.Contains(got.CanonicalID, "src_model_generated_opaque") {
		t.Fatalf("canonical graph id leaked model-generated source_entity_id: %q", got.CanonicalID)
	}
}

func TestTextureToolSourceGraphUsesComputerScopedIdentity(t *testing.T) {
	bodyDoc, sourceEntities := structuredTextureToolPayload(t)
	canonicalEntities := map[string]string{}
	canonicalRefs := map[string]string{}
	for _, computerID := range []string{"computer-a", "computer-b"} {
		rev := types.Revision{
			RevisionID: "revision-shared", DocID: "document-shared", OwnerID: "owner-shared",
			ComputerID: computerID, CreatedAt: time.Now().UTC().Truncate(time.Millisecond),
		}
		graph, err := textureToolSourceGraphWriteSet(rev, materializedTextureEdit{
			BodyDoc: bodyDoc, SourceEntities: sourceEntities,
		}, &types.RunRecord{RunID: "run-shared", ComputerID: computerID})
		if err != nil {
			t.Fatalf("build %s source graph: %v", computerID, err)
		}
		if len(graph.SourceEntities) != 1 || len(graph.SourceRefs) != 1 {
			t.Fatalf("%s source graph = %+v", computerID, graph)
		}
		if graph.SourceEntities[0].ComputerID != computerID || graph.SourceRefs[0].ComputerID != computerID {
			t.Fatalf("%s source graph lost computer identity: %+v", computerID, graph)
		}
		canonicalEntities[computerID] = graph.SourceEntities[0].CanonicalID
		canonicalRefs[computerID] = graph.SourceRefs[0].CanonicalID
	}
	if canonicalEntities["computer-a"] == canonicalEntities["computer-b"] {
		t.Fatalf("source entity canonical IDs collided: %q", canonicalEntities["computer-a"])
	}
	if canonicalRefs["computer-a"] == canonicalRefs["computer-b"] {
		t.Fatalf("source ref canonical IDs collided: %q", canonicalRefs["computer-a"])
	}
}

func TestTextureToolSourceGraphWritesSourceRefEdgesPinnedToRevisionAndSourceVersion(t *testing.T) {
	bodyDoc, sourceEntities := structuredTextureToolPayload(t)
	rev := types.Revision{
		RevisionID:       "rev-source-ref-edges",
		DocID:            "doc-source-ref-edges",
		OwnerID:          "user-1",
		ParentRevisionID: "rev-base",
		CreatedAt:        time.Now().UTC().Truncate(time.Millisecond),
	}
	graph, err := textureToolSourceGraphWriteSet(rev, materializedTextureEdit{
		BodyDoc:        bodyDoc,
		SourceEntities: sourceEntities,
	}, &types.RunRecord{RunID: "run-source-ref-edges"})
	if err != nil {
		t.Fatalf("textureToolSourceGraphWriteSet: %v", err)
	}
	if len(graph.SourceEntities) != 1 {
		t.Fatalf("source graph entities len = %d, want 1", len(graph.SourceEntities))
	}
	if len(graph.SourceRefs) != 1 {
		t.Fatalf("source graph refs len = %d, want 1: %#v", len(graph.SourceRefs), graph.SourceRefs)
	}
	entity := graph.SourceEntities[0]
	ref := graph.SourceRefs[0]
	if ref.LegacySourceEntityID != "src-web" || ref.BodyNodeID != "ref-1" {
		t.Fatalf("source ref legacy/body identity = %#v, want src-web/ref-1", ref)
	}
	if ref.SourceEntityCanonicalID != entity.CanonicalID || ref.SourceEntityVersionID != entity.VersionID {
		t.Fatalf("source ref pins %s/%s, want %s/%s", ref.SourceEntityCanonicalID, ref.SourceEntityVersionID, entity.CanonicalID, entity.VersionID)
	}
	if ref.DisplayMode != store.TextureSourceRefDisplayNumbered || ref.CitationState != "cited" {
		t.Fatalf("source ref mode/state = %s/%s", ref.DisplayMode, ref.CitationState)
	}
	if ref.BodyNodePathHash == "" || !strings.HasPrefix(ref.BodyNodePathHash, "sha256:") {
		t.Fatalf("body node path hash = %q, want sha256 hash", ref.BodyNodePathHash)
	}
	var meta map[string]any
	if err := json.Unmarshal(ref.Metadata, &meta); err != nil {
		t.Fatalf("unmarshal source ref metadata: %v", err)
	}
	if meta["source_entity_canonical_id"] != entity.CanonicalID || meta["source_entity_version_id"] != entity.VersionID || meta["created_run_id"] != "run-source-ref-edges" {
		t.Fatalf("source ref metadata missing pin/run evidence: %#v", meta)
	}
}

func TestTextureToolSourceGraphDuplicateLegacyIDsResolveToSharedGraphEntity(t *testing.T) {
	doc := texturedoc.StructuredTextureDoc{
		Schema: texturedoc.SchemaV1,
		Doc: texturedoc.Node{
			Type:  "doc",
			Attrs: map[string]any{"id": "doc-node"},
			Content: []texturedoc.Node{{
				Type:  "paragraph",
				Attrs: map[string]any{"id": "p-1"},
				Content: []texturedoc.Node{
					{Type: "text", Text: "First"},
					{Type: "source_ref", Attrs: map[string]any{"id": "ref-a", "source_entity_id": "src-a", "display_mode": "numbered_ref"}},
					{Type: "text", Text: " and second"},
					{Type: "source_ref", Attrs: map[string]any{"id": "ref-b", "source_entity_id": "src-b", "display_mode": "expanded_ref"}},
					{Type: "text", Text: "."},
				},
			}},
		},
	}
	entities := []texturedoc.SourceEntity{
		textureToolAvailableSource("src-a"),
		textureToolAvailableSource("src-b"),
	}
	bodyDoc, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal body_doc: %v", err)
	}
	sourceEntities, err := json.Marshal(entities)
	if err != nil {
		t.Fatalf("marshal source_entities: %v", err)
	}
	rev := types.Revision{
		RevisionID: "rev-duplicate-source-ids",
		DocID:      "doc-duplicate-source-ids",
		OwnerID:    "user-1",
		CreatedAt:  time.Now().UTC().Truncate(time.Millisecond),
	}
	graph, err := textureToolSourceGraphWriteSet(rev, materializedTextureEdit{
		BodyDoc:        bodyDoc,
		SourceEntities: sourceEntities,
	}, &types.RunRecord{RunID: "run-duplicate-source-ids"})
	if err != nil {
		t.Fatalf("textureToolSourceGraphWriteSet: %v", err)
	}
	if len(graph.SourceEntities) != 1 {
		t.Fatalf("source graph entities len = %d, want 1 shared graph record: %#v", len(graph.SourceEntities), graph.SourceEntities)
	}
	if len(graph.SourceRefs) != 2 {
		t.Fatalf("source refs len = %d, want 2: %#v", len(graph.SourceRefs), graph.SourceRefs)
	}
	shared := graph.SourceEntities[0]
	seenLegacyRefs := map[string]bool{}
	for _, ref := range graph.SourceRefs {
		if ref.SourceEntityCanonicalID != shared.CanonicalID || ref.SourceEntityVersionID != shared.VersionID {
			t.Fatalf("source ref %#v did not pin shared graph entity %#v", ref, shared)
		}
		seenLegacyRefs[ref.LegacySourceEntityID] = true
	}
	if !seenLegacyRefs["src-a"] || !seenLegacyRefs["src-b"] {
		t.Fatalf("source refs legacy ids = %#v, want src-a and src-b", seenLegacyRefs)
	}
}

func structuredTextureToolPayload(t *testing.T) (json.RawMessage, json.RawMessage) {
	t.Helper()
	doc := texturedoc.StructuredTextureDoc{
		Schema: texturedoc.SchemaV1,
		Doc: texturedoc.Node{
			Type:  "doc",
			Attrs: map[string]any{"id": "doc-node"},
			Content: []texturedoc.Node{{
				Type:  "paragraph",
				Attrs: map[string]any{"id": "p-1"},
				Content: []texturedoc.Node{
					{Type: "text", Text: "Grounded"},
					{
						Type: "source_ref",
						Attrs: map[string]any{
							"id":               "ref-1",
							"source_entity_id": "src-web",
							"display_mode":     "numbered_ref",
						},
					},
					{Type: "text", Text: "."},
				},
			}},
		},
	}
	entities := []texturedoc.SourceEntity{{
		SourceEntityID: "src-web",
		Target:         texturedoc.SourceTarget{Kind: "web_url", URI: "https://example.com/story"},
		Selectors:      []texturedoc.SourceSelector{{Kind: sourcecontract.SelectorKindTextQuote, Data: map[string]any{"exact": "Grounded"}}},
		Display:        texturedoc.SourceDisplay{Mode: "numbered_ref", Title: "Example story"},
		Evidence:       texturedoc.SourceEvidence{State: sourcecontract.EvidenceStateConfirms, OpenSurface: sourcecontract.OpenSurfaceSource},
		Provenance:     texturedoc.SourceEntityProvenance{CreatedBy: "runtime", SourceSystem: "test"},
	}}
	bodyDoc, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal body_doc: %v", err)
	}
	sourceEntities, err := json.Marshal(entities)
	if err != nil {
		t.Fatalf("marshal source_entities: %v", err)
	}
	return bodyDoc, sourceEntities
}

func textureToolAvailableSource(sourceEntityID string) texturedoc.SourceEntity {
	return texturedoc.SourceEntity{
		SourceEntityID: sourceEntityID,
		Target:         texturedoc.SourceTarget{Kind: "web_url", URI: "https://example.com/story"},
		Selectors:      []texturedoc.SourceSelector{{Kind: sourcecontract.SelectorKindWholeResource}},
		Display:        texturedoc.SourceDisplay{Mode: "numbered_ref", Title: "Example story"},
		Evidence:       texturedoc.SourceEvidence{State: sourcecontract.EvidenceStateConfirms, OpenSurface: sourcecontract.OpenSurfaceSource},
		Provenance:     texturedoc.SourceEntityProvenance{CreatedBy: "runtime", SourceSystem: "test"},
	}
}

func structuredDocHasType(node texturedoc.Node, nodeType string) bool {
	if node.Type == nodeType {
		return true
	}
	for _, child := range node.Content {
		if structuredDocHasType(child, nodeType) {
			return true
		}
	}
	return false
}

