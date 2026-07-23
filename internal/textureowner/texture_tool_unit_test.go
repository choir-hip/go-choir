package textureowner

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/provider"
	"github.com/yusefmosiah/go-choir/internal/provideriface"

	"github.com/yusefmosiah/go-choir/internal/agentcore"
	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/sourcecontract"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/texturedoc"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
)

type semanticMergeTestProvider struct {
	response string
	req      provideriface.ToolLoopRequest
	calls    int
}

func (p *semanticMergeTestProvider) ProviderName() string { return "semantic-merge-test" }

func (p *semanticMergeTestProvider) Execute(ctx context.Context, task *types.RunRecord, emit provideriface.EventEmitFunc) error {
	task.Result = p.response
	return nil
}

func (p *semanticMergeTestProvider) CallWithTools(ctx context.Context, req provideriface.ToolLoopRequest) (*provideriface.ToolLoopResponse, error) {
	p.calls++
	p.req = req
	return &provideriface.ToolLoopResponse{
		ID:         "model-response-1",
		StopReason: "end_turn",
		Text:       p.response,
		Usage:      provideriface.TokenUsage{InputTokens: 321, OutputTokens: 123},
		Model:      "semantic-test-model",
	}, nil
}

func TestCleanTextureToolContentRemovesWrapperTags(t *testing.T) {
	input := " <payload>\nStaging smoke after RSS title extraction works.\n</payload> "
	if got := cleanTextureToolContent(input); got != "Staging smoke after RSS title extraction works." {
		t.Fatalf("cleanTextureToolContent() = %q", got)
	}
}

func TestCleanTextureToolContentRemovesDanglingClosingMarker(t *testing.T) {
	for _, input := range []string{
		"Texture wrapper cleanup works.</\n",
		"Texture wrapper cleanup works.</妮>",
	} {
		if got := cleanTextureToolContent(input); got != "Texture wrapper cleanup works." {
			t.Fatalf("cleanTextureToolContent(%q) = %q", input, got)
		}
	}
}

func TestCleanTextureToolContentPreservesOrdinaryText(t *testing.T) {
	input := "The paragraph mentions <payload> as literal text inside the body."
	if got := cleanTextureToolContent(input); got != input {
		t.Fatalf("cleanTextureToolContent() = %q, want original", got)
	}
}

func TestMaterializeTextureToolEditRequiresRationaleForLongRewrite(t *testing.T) {
	current := types.Revision{
		RevisionID: "rev-long",
		Content:    strings.Repeat("long section\n", 1300),
	}
	_, err := materializeTextureToolEdit(editTextureArgs{
		BaseRevisionID: "rev-long",
		Operation:      "replace_all",
		Content:        "short replacement",
	}, current)
	if err == nil || !strings.Contains(err.Error(), "requires rationale") {
		t.Fatalf("replace_all long doc err = %v, want rationale guard", err)
	}

	got, err := materializeTextureToolEdit(editTextureArgs{
		BaseRevisionID: "rev-long",
		Operation:      "replace_all",
		Content:        "short replacement",
		Rationale:      "Owner explicitly requested a full summary rewrite.",
	}, current)
	if err != nil {
		t.Fatalf("replace_all with rationale: %v", err)
	}
	if got.Operation != "replace_all" || got.Rationale == "" || got.EditCount != 1 {
		t.Fatalf("materialized rewrite metadata = %+v", got)
	}
}

func TestRewriteTextureParsesMarkdownIntoStructuredBlocks(t *testing.T) {
	current := types.Revision{RevisionID: "rev-markdown", DocID: "doc-markdown"}
	got, err := materializeTextureToolEdit(editTextureArgs{
		DocID:          "doc-markdown",
		BaseRevisionID: "rev-markdown",
		Operation:      "replace_all",
		Content: strings.Join([]string{
			"# Music brief",
			"",
			"Lead paragraph.",
			"",
			"## Signals",
			"",
			"- Touring revenue",
			"- Label licensing",
			"",
			"1. Watch releases",
			"2. Watch regulation",
		}, "\n"),
	}, current)
	if err != nil {
		t.Fatalf("replace_all markdown: %v", err)
	}
	var doc texturedoc.StructuredTextureDoc
	if err := json.Unmarshal(got.BodyDoc, &doc); err != nil {
		t.Fatalf("unmarshal body_doc: %v", err)
	}
	for _, nodeType := range []string{"heading", "paragraph", "bullet_list", "ordered_list"} {
		if !structuredDocHasType(doc.Doc, nodeType) {
			t.Fatalf("body_doc missing %s: %s", nodeType, got.BodyDoc)
		}
	}
	if len(doc.Doc.Content) == 0 || doc.Doc.Content[0].Type != "heading" {
		t.Fatalf("first structured block = %#v, want heading", doc.Doc.Content)
	}
	if firstText := structuredDocFirstText(doc.Doc.Content[0]); strings.Contains(firstText, "#") {
		t.Fatalf("heading text retained raw markdown marker: %q", firstText)
	}
}

func TestTextureToolStructuredUpdatePreservesSourceRefs(t *testing.T) {
	current := structuredTextureToolRevision(t)
	got, err := materializeTextureToolEdit(editTextureArgs{
		DocID:          current.DocID,
		BaseRevisionID: current.RevisionID,
		Operation:      "apply_edits",
		StructuredEdits: []textureStructuredEdit{{
			Op:      "update_block_text",
			BlockID: "p-1",
			Text:    "Updated grounded claim",
		}},
	}, current)
	if err != nil {
		t.Fatalf("materialize structured update: %v", err)
	}
	if got.Content != "Updated grounded claim[1]" {
		t.Fatalf("Content = %q, want source ref projection preserved", got.Content)
	}
	var doc texturedoc.StructuredTextureDoc
	if err := json.Unmarshal(got.BodyDoc, &doc); err != nil {
		t.Fatalf("unmarshal body_doc: %v", err)
	}
	if !structuredDocHasNode(doc.Doc, "source_ref", "ref-1") {
		t.Fatalf("body_doc lost source_ref node: %s", got.BodyDoc)
	}
	var entities []texturedoc.SourceEntity
	if err := json.Unmarshal(got.SourceEntities, &entities); err != nil {
		t.Fatalf("unmarshal source_entities: %v", err)
	}
	if len(entities) != 1 || entities[0].SourceEntityID != "src-web" {
		t.Fatalf("source_entities = %#v, want src-web preserved", entities)
	}
}

func TestTextureToolStructuredDeleteSourceNodeFiltersDetachedEntity(t *testing.T) {
	current := structuredTextureToolRevision(t)
	got, err := materializeTextureToolEdit(editTextureArgs{
		DocID:          current.DocID,
		BaseRevisionID: current.RevisionID,
		Operation:      "apply_edits",
		StructuredEdits: []textureStructuredEdit{{
			Op:     "delete_node",
			NodeID: "ref-1",
		}},
	}, current)
	if err != nil {
		t.Fatalf("materialize delete source node: %v", err)
	}
	if strings.Contains(got.Content, "[1]") {
		t.Fatalf("Content = %q, want source projection removed", got.Content)
	}
	var entities []texturedoc.SourceEntity
	if err := json.Unmarshal(got.SourceEntities, &entities); err != nil {
		t.Fatalf("unmarshal source_entities: %v", err)
	}
	if len(entities) != 0 {
		t.Fatalf("source_entities = %#v, want detached source filtered", entities)
	}
}

func TestTextureToolStructuredSourceInsertionWritesBodyNodesAndTopLevelEntities(t *testing.T) {
	current := plainTextureToolRevision(t, "Start")
	got, err := materializeTextureToolEdit(editTextureArgs{
		DocID:          current.DocID,
		BaseRevisionID: current.RevisionID,
		Operation:      "apply_edits",
		StructuredEdits: []textureStructuredEdit{
			{
				Op:      "insert_source_ref",
				BlockID: "p-1",
				SourceEntity: &texturedoc.SourceEntity{
					Target:  texturedoc.SourceTarget{Kind: "web_url", URI: "https://example.com/ref"},
					Display: texturedoc.SourceDisplay{Title: "Reference story"},
				},
			},
			{
				Op:          "insert_source_ref",
				BlockID:     "p-1",
				DisplayMode: "expanded_ref",
				SourceEntity: &texturedoc.SourceEntity{
					Target:  texturedoc.SourceTarget{Kind: "image", URI: "https://example.com/image.png"},
					Display: texturedoc.SourceDisplay{Title: "Launch image"},
				},
			},
		},
	}, current)
	if err != nil {
		t.Fatalf("materialize source insertion: %v", err)
	}
	var doc texturedoc.StructuredTextureDoc
	if err := json.Unmarshal(got.BodyDoc, &doc); err != nil {
		t.Fatalf("unmarshal body_doc: %v", err)
	}
	if !structuredDocHasType(doc.Doc, "source_ref") {
		t.Fatalf("body_doc missing inserted source_ref nodes: %s", got.BodyDoc)
	}
	var entities []texturedoc.SourceEntity
	if err := json.Unmarshal(got.SourceEntities, &entities); err != nil {
		t.Fatalf("unmarshal source_entities: %v", err)
	}
	if len(entities) != 2 {
		t.Fatalf("source_entities len = %d, want 2: %#v", len(entities), entities)
	}
	for _, entity := range entities {
		if strings.TrimSpace(entity.SourceEntityID) == "" || strings.Contains(entity.SourceEntityID, "example.com") {
			t.Fatalf("runtime did not mint opaque source id: %#v", entity)
		}
	}
}

func TestTextureToolMarkSourceUnusedAllowsUncitedEntity(t *testing.T) {
	current := plainTextureToolRevision(t, "Start")
	available := []texturedoc.SourceEntity{
		textureToolAvailableSource("src-cited"),
		textureToolAvailableSource("src-unused"),
	}
	got, err := materializeTextureToolEdit(editTextureArgs{
		DocID:          current.DocID,
		BaseRevisionID: current.RevisionID,
		Operation:      "apply_edits",
		StructuredEdits: []textureStructuredEdit{
			{Op: "insert_source_ref", BlockID: "p-1", SourceEntityID: "src-cited"},
			{Op: "mark_source_unused", SourceEntityID: "src-unused", Rationale: "Duplicate of src-cited; no distinct claim."},
		},
		AvailableSources: available,
	}, current)
	if err != nil {
		t.Fatalf("materialize mark_source_unused: %v", err)
	}
	var entities []texturedoc.SourceEntity
	if err := json.Unmarshal(got.SourceEntities, &entities); err != nil {
		t.Fatalf("unmarshal source_entities: %v", err)
	}
	if len(entities) != 2 {
		t.Fatalf("source_entities len = %d, want 2 (cited + marked-unused): %#v", len(entities), entities)
	}
}

func TestTextureToolMarkSourceUnusedRequiresRationale(t *testing.T) {
	current := plainTextureToolRevision(t, "Start")
	_, err := materializeTextureToolEdit(editTextureArgs{
		DocID:          current.DocID,
		BaseRevisionID: current.RevisionID,
		Operation:      "apply_edits",
		StructuredEdits: []textureStructuredEdit{
			{Op: "mark_source_unused", SourceEntityID: "src-web"},
		},
		AvailableSources: []texturedoc.SourceEntity{textureToolAvailableSource("src-web")},
	}, current)
	if err == nil || !strings.Contains(err.Error(), "requires a rationale") {
		t.Fatalf("mark_source_unused without rationale err = %v, want rationale rejection", err)
	}
}

func TestTextureToolSourceRefOffsetNormalizesOutOfWord(t *testing.T) {
	current := plainTextureToolRevision(t, "Frontier labs are still shipping, and OpenAI is active.")
	offset := len("Frontier labs are still s")
	got, err := materializeTextureToolEdit(editTextureArgs{
		DocID:          current.DocID,
		BaseRevisionID: current.RevisionID,
		Operation:      "apply_edits",
		StructuredEdits: []textureStructuredEdit{{
			Op:             "insert_source_ref",
			BlockID:        "p-1",
			SourceEntityID: "src-web",
			Offset:         &offset,
		}},
		AvailableSources: []texturedoc.SourceEntity{{
			SourceEntityID: "src-web",
			Target:         texturedoc.SourceTarget{Kind: "web_url", URI: "https://example.com/story"},
			Selectors:      []texturedoc.SourceSelector{{Kind: sourcecontract.SelectorKindWholeResource}},
			Display:        texturedoc.SourceDisplay{Mode: "numbered_ref", Title: "Example story"},
			Evidence:       texturedoc.SourceEvidence{State: sourcecontract.EvidenceStateConfirms, OpenSurface: sourcecontract.OpenSurfaceSource},
			Provenance:     texturedoc.SourceEntityProvenance{CreatedBy: "runtime"},
		}},
	}, current)
	if err != nil {
		t.Fatalf("materialize source ref insertion: %v", err)
	}
	if strings.Contains(got.Content, "s[1]hipping") {
		t.Fatalf("source ref landed inside word: %q", got.Content)
	}
	if !strings.Contains(got.Content, "shipping[1],") {
		t.Fatalf("source ref did not normalize to word boundary: %q", got.Content)
	}
}

func TestTextureToolRejectsWholeMarkdownInUpdateBlockText(t *testing.T) {
	current := plainTextureToolRevision(t, "Start")
	_, err := materializeTextureToolEdit(editTextureArgs{
		DocID:          current.DocID,
		BaseRevisionID: current.RevisionID,
		Operation:      "apply_edits",
		StructuredEdits: []textureStructuredEdit{{
			Op:      "update_block_text",
			BlockID: "p-1",
			Text:    "# Music brief\n\nLead paragraph.",
		}},
	}, current)
	if err == nil || !strings.Contains(err.Error(), "multi-paragraph or markdown-formatted text") {
		t.Fatalf("update_block_text markdown err = %v, want multi-paragraph/markdown rejection", err)
	}
}

func TestTextureToolRejectsOffsetZeroSourceRefInNonEmptyBlock(t *testing.T) {
	current := plainTextureToolRevision(t, "Start")
	offset := 0
	_, err := materializeTextureToolEdit(editTextureArgs{
		DocID:          current.DocID,
		BaseRevisionID: current.RevisionID,
		Operation:      "apply_edits",
		StructuredEdits: []textureStructuredEdit{{
			Op:             "insert_source_ref",
			BlockID:        "p-1",
			SourceEntityID: "src-web",
			Offset:         &offset,
		}},
		AvailableSources: []texturedoc.SourceEntity{textureToolAvailableSource("src-web")},
	}, current)
	if err == nil || !strings.Contains(err.Error(), "offset 0") {
		t.Fatalf("source ref offset zero err = %v, want offset 0 rejection", err)
	}
}

func TestTextureToolRejectsDuplicateSourceRefOffsets(t *testing.T) {
	current := plainTextureToolRevision(t, "Start")
	offset := len([]rune("Start"))
	_, err := materializeTextureToolEdit(editTextureArgs{
		DocID:          current.DocID,
		BaseRevisionID: current.RevisionID,
		Operation:      "apply_edits",
		StructuredEdits: []textureStructuredEdit{
			{Op: "insert_source_ref", BlockID: "p-1", SourceEntityID: "src-web-1", Offset: &offset},
			{Op: "insert_source_ref", BlockID: "p-1", SourceEntityID: "src-web-2", Offset: &offset},
		},
		AvailableSources: []texturedoc.SourceEntity{
			textureToolAvailableSource("src-web-1"),
			textureToolAvailableSource("src-web-2"),
		},
	}, current)
	if err == nil || !strings.Contains(err.Error(), "same block_id and offset") {
		t.Fatalf("duplicate source ref offset err = %v, want rejection", err)
	}
}

func TestTextureToolRejectsLegacyEditsAndSourceSyntax(t *testing.T) {
	current := plainTextureToolRevision(t, "Start")
	if _, err := materializeTextureToolEdit(editTextureArgs{
		DocID:          current.DocID,
		BaseRevisionID: current.RevisionID,
		Operation:      "apply_edits",
		StructuredEdits: []textureStructuredEdit{{
			Op:   "append",
			Text: "legacy",
		}},
	}, current); err == nil || !strings.Contains(err.Error(), `op = "append", want update_block_text`) {
		t.Fatalf("legacy edit err = %v, want rejection", err)
	}
	if _, err := materializeTextureToolEdit(editTextureArgs{
		DocID:          current.DocID,
		BaseRevisionID: current.RevisionID,
		Operation:      "apply_edits",
		StructuredEdits: []textureStructuredEdit{{
			Op:      "update_block_text",
			BlockID: "p-1",
			Text:    "[Story](source:src-web)",
		}},
	}, current); err == nil || !strings.Contains(err.Error(), "legacy markdown source link") {
		t.Fatalf("legacy source syntax err = %v, want rejection", err)
	}
}

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
		}, &types.RunRecord{RunID: "run-shared", SandboxID: computerID})
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

func TestPatchTextureSourceRefFailureDoesNotAdvanceDocumentHead(t *testing.T) {
	s, _, _ := textureToolCommitRuntime(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Millisecond)
	doc := types.Document{DocID: "doc-source-ref-failure", OwnerID: "user-1", Title: "Source Ref Failure", CreatedAt: now, UpdatedAt: now}
	if err := s.CreateDocument(ctx, doc); err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}
	base := plainTextureToolRevision(t, "Base")
	base.RevisionID = "rev-source-ref-failure-base"
	base.DocID = doc.DocID
	base.OwnerID = doc.OwnerID
	base.CreatedAt = now
	if err := s.CreateRevision(ctx, base); err != nil {
		t.Fatalf("CreateRevision base: %v", err)
	}
	bodyDoc, sourceEntities := structuredTextureToolPayload(t)
	rev := types.Revision{
		RevisionID:       "rev-source-ref-failure-next",
		DocID:            doc.DocID,
		OwnerID:          doc.OwnerID,
		AuthorKind:       types.AuthorAppAgent,
		AuthorLabel:      "appagent",
		BodyDoc:          bodyDoc,
		SourceEntities:   sourceEntities,
		ParentRevisionID: base.RevisionID,
		CreatedAt:        now.Add(time.Second),
	}
	graph, err := textureToolSourceGraphWriteSet(rev, materializedTextureEdit{
		BodyDoc:        bodyDoc,
		SourceEntities: sourceEntities,
	}, &types.RunRecord{RunID: "run-source-ref-failure"})
	if err != nil {
		t.Fatalf("textureToolSourceGraphWriteSet: %v", err)
	}
	if len(graph.SourceRefs) != 1 {
		t.Fatalf("source refs len = %d, want 1", len(graph.SourceRefs))
	}
	graph.SourceEntities = nil
	err = s.CreateRevisionWithSourceGraph(ctx, rev, graph)
	if err == nil || !strings.Contains(err.Error(), "missing source entity version") {
		t.Fatalf("CreateRevisionWithSourceGraph error = %v, want missing source entity version", err)
	}
	gotDoc, err := s.GetDocument(ctx, doc.DocID, doc.OwnerID)
	if err != nil {
		t.Fatalf("GetDocument: %v", err)
	}
	if gotDoc.CurrentRevisionID != base.RevisionID {
		t.Fatalf("current_revision_id = %q, want unchanged base %q", gotDoc.CurrentRevisionID, base.RevisionID)
	}
	if _, err := s.GetRevision(ctx, rev.RevisionID, doc.OwnerID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("GetRevision after failed graph write = %v, want ErrNotFound", err)
	}
}

func TestTextureToolCommitWritesStructuredRevisionAndRejectsStaleBase(t *testing.T) {
	s, h, registry := textureToolCommitRuntime(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Millisecond)
	doc := types.Document{DocID: "doc-d4-tool", OwnerID: "user-1", Title: "D4 Tool", CreatedAt: now, UpdatedAt: now}
	if err := s.CreateDocument(ctx, doc); err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}
	base := structuredTextureToolRevision(t)
	base.DocID = doc.DocID
	base.OwnerID = doc.OwnerID
	base.CreatedAt = now
	if err := s.CreateRevision(ctx, base); err != nil {
		t.Fatalf("CreateRevision base: %v", err)
	}
	runID := "run-d4-tool"
	if err := s.CreateAgentMutation(ctx, store.AgentMutation{
		DocID:     doc.DocID,
		RunID:     runID,
		OwnerID:   doc.OwnerID,
		State:     "pending",
		CreatedAt: now,
	}); err != nil {
		t.Fatalf("CreateAgentMutation: %v", err)
	}
	run := &types.RunRecord{
		RunID:        runID,
		AgentID:      currentTextureAgentID(doc.DocID),
		ChannelID:    doc.DocID,
		OwnerID:      doc.OwnerID,
		SandboxID:    "sandbox-texture-test",
		State:        types.RunRunning,
		Prompt:       "Patch structured Texture.",
		CreatedAt:    now,
		UpdatedAt:    now,
		AgentProfile: agentprofile.Texture,
		AgentRole:    agentprofile.Texture,
		Metadata: map[string]any{
			"type":                     textureAgentRevisionTaskType,
			"doc_id":                   doc.DocID,
			"source_entities":          []textureSourceEntity{{EntityID: "legacy-sidecar", Kind: "content_item"}},
			"source_ref_normalization": map[string]any{"legacy_count": 1},
			runMetadataAgentID:         currentTextureAgentID(doc.DocID),
			runMetadataAgentProfile:    agentprofile.Texture,
			runMetadataAgentRole:       agentprofile.Texture,
			runMetadataChannelID:       doc.DocID,
		},
	}
	rawArgs, err := json.Marshal(editTextureArgs{
		DocID:          doc.DocID,
		BaseRevisionID: base.RevisionID,
		StructuredEdits: []textureStructuredEdit{{
			Op:      "update_block_text",
			BlockID: "p-1",
			Text:    "Committed structured claim",
		}},
	})
	if err != nil {
		t.Fatalf("marshal patch args: %v", err)
	}
	if _, err := registry.Execute(toolregistry.WithExecutionContext(ctx, textureToolExecutionContext(run)), "patch_texture", rawArgs); err != nil {
		t.Fatalf("patch_texture: %v", err)
	}
	revs, err := s.ListRevisionsByDoc(ctx, doc.DocID, doc.OwnerID, 10)
	if err != nil {
		t.Fatalf("ListRevisionsByDoc: %v", err)
	}
	if len(revs) != 2 {
		t.Fatalf("revisions len = %d, want 2", len(revs))
	}
	appRev := revs[0]
	if len(appRev.BodyDoc) == 0 || len(appRev.SourceEntities) == 0 {
		t.Fatalf("app revision missing structured fields: body_doc=%s source_entities=%s", appRev.BodyDoc, appRev.SourceEntities)
	}
	var legacyEntities []texturedoc.SourceEntity
	if err := json.Unmarshal(appRev.SourceEntities, &legacyEntities); err != nil {
		t.Fatalf("unmarshal legacy source_entities: %v", err)
	}
	if len(legacyEntities) != 1 || legacyEntities[0].SourceEntityID != "src-web" {
		t.Fatalf("legacy source_entities = %#v, want src-web preserved", legacyEntities)
	}
	graphEntities, err := s.ListTextureSourceEntities(ctx, doc.OwnerID)
	if err != nil {
		t.Fatalf("ListTextureSourceEntities: %v", err)
	}
	if len(graphEntities) != 1 {
		t.Fatalf("graph source entities len = %d, want 1: %#v", len(graphEntities), graphEntities)
	}
	expectedCanonicalID, err := store.BuildTextureSourceEntityCanonicalID(doc.OwnerID, doc.OwnerID, "web_url", "https://example.com/story")
	if err != nil {
		t.Fatalf("BuildTextureSourceEntityCanonicalID: %v", err)
	}
	if graphEntities[0].CanonicalID != expectedCanonicalID || graphEntities[0].LegacySourceEntityID != "src-web" {
		t.Fatalf("graph source entity = %#v, want target-derived canonical ID and legacy src-web", graphEntities[0])
	}
	graphRefs, err := s.ListTextureSourceRefsForRevision(ctx, doc.OwnerID, doc.DocID, appRev.RevisionID)
	if err != nil {
		t.Fatalf("ListTextureSourceRefsForRevision: %v", err)
	}
	if len(graphRefs) != 1 {
		t.Fatalf("graph source refs len = %d, want 1: %#v", len(graphRefs), graphRefs)
	}
	if graphRefs[0].SourceEntityCanonicalID != graphEntities[0].CanonicalID || graphRefs[0].SourceEntityVersionID != graphEntities[0].VersionID {
		t.Fatalf("graph source ref = %#v, want pin to graph entity %#v", graphRefs[0], graphEntities[0])
	}
	apiResp := h.revisionResponseFromRecord(ctx, appRev)
	if string(apiResp.SourceEntities) != string(appRev.SourceEntities) {
		t.Fatalf("legacy source_entities changed in API response: got %s want %s", apiResp.SourceEntities, appRev.SourceEntities)
	}
	if len(apiResp.SourceEntityObjects) != 1 {
		t.Fatalf("source_entity_objects len = %d, want 1: %#v", len(apiResp.SourceEntityObjects), apiResp.SourceEntityObjects)
	}
	if apiResp.SourceEntityObjects[0].ObjectKind != string(store.TextureSourceEntityObjectKind) ||
		apiResp.SourceEntityObjects[0].CanonicalID != graphEntities[0].CanonicalID ||
		apiResp.SourceEntityObjects[0].LegacySourceEntityID != "src-web" {
		t.Fatalf("source_entity_objects[0] = %#v, want graph entity wrapper", apiResp.SourceEntityObjects[0])
	}
	if len(apiResp.SourceRefs) != 1 {
		t.Fatalf("source_refs len = %d, want 1: %#v", len(apiResp.SourceRefs), apiResp.SourceRefs)
	}
	if apiResp.SourceRefs[0].ObjectKind != string(store.TextureSourceRefObjectKind) ||
		apiResp.SourceRefs[0].SourceEntityCanonicalID != graphEntities[0].CanonicalID ||
		apiResp.SourceRefs[0].SourceEntityVersionID != graphEntities[0].VersionID ||
		apiResp.SourceRefs[0].DisplayMode != store.TextureSourceRefDisplayNumbered {
		t.Fatalf("source_refs[0] = %#v, want pinned source_ref wrapper", apiResp.SourceRefs[0])
	}
	listResp := h.revisionResponsesFromRecords(ctx, revs, doc.OwnerID, doc.DocID)
	if len(listResp) != 2 {
		t.Fatalf("revisionResponsesFromRecords len = %d, want 2", len(listResp))
	}
	if listResp[0].RevisionID != appRev.RevisionID || len(listResp[0].SourceEntityObjects) != 1 || len(listResp[0].SourceRefs) != 1 {
		t.Fatalf("listed app revision response = %#v, want source wrappers", listResp[0])
	}
	if string(listResp[0].SourceEntities) != string(appRev.SourceEntities) {
		t.Fatalf("listed response changed legacy source_entities: got %s want %s", listResp[0].SourceEntities, appRev.SourceEntities)
	}
	if listResp[1].RevisionID != base.RevisionID {
		t.Fatalf("listed base revision id = %q, want %q", listResp[1].RevisionID, base.RevisionID)
	}
	if len(listResp[1].SourceEntityObjects) != 0 || len(listResp[1].SourceRefs) != 0 {
		t.Fatalf("listed base revision response = %#v, want no graph wrappers", listResp[1])
	}
	meta := decodeRevisionMetadata(appRev.Metadata)
	for _, key := range []string{"source_entities", "media_source_refs", "source_ref_normalization", "citations_json"} {
		if _, ok := meta[key]; ok {
			t.Fatalf("metadata retained legacy source key %q: %#v", key, meta)
		}
	}
	if _, err := registry.Execute(toolregistry.WithExecutionContext(ctx, textureToolExecutionContext(run)), "patch_texture", rawArgs); err == nil ||
		!strings.Contains(err.Error(), "stale") {
		t.Fatalf("stale base err = %v, want stale rejection", err)
	}
}

func TestTextureEditRevisionMetadataRecordsOperationEvidence(t *testing.T) {
	now := time.Now().UTC()
	raw := addTextureEditRevisionMetadata(json.RawMessage(`{"existing":"kept"}`), materializedTextureEdit{
		Operation:      "apply_edits",
		SourceTool:     "patch_texture",
		BaseRevisionID: "rev-1",
		EditCount:      2,
		BaseChars:      100,
		ResultChars:    124,
		DeltaChars:     24,
	}, &types.RunRecord{
		RunID:     "run-1",
		Prompt:    "revise paragraph",
		CreatedAt: now.Add(-1500 * time.Millisecond),
		UpdatedAt: now,
	})
	var meta map[string]any
	if err := json.Unmarshal(raw, &meta); err != nil {
		t.Fatalf("metadata json: %v", err)
	}
	if meta["existing"] != "kept" {
		t.Fatalf("existing metadata not preserved: %+v", meta)
	}
	if meta["source"] != "patch_texture" || meta["texture_edit_tool"] != "patch_texture" {
		t.Fatalf("texture write source metadata missing: %+v", meta)
	}
	if meta["texture_edit_operation"] != "apply_edits" || int(meta["texture_edit_count"].(float64)) != 2 {
		t.Fatalf("edit operation metadata missing: %+v", meta)
	}
	if int(meta["texture_run_prompt_chars"].(float64)) != len("revise paragraph") {
		t.Fatalf("prompt chars metadata missing: %+v", meta)
	}
	if int(meta["texture_edit_delta_chars"].(float64)) != 24 {
		t.Fatalf("delta metadata missing: %+v", meta)
	}
}

func TestTextureSemanticMergeUsesProviderBackedJSON(t *testing.T) {
	provider := &semanticMergeTestProvider{response: `{
		"summary": ["Older version has a stronger client-control framing."],
		"suggestions": [{
			"id": "client_control_frame",
			"label": "Restore client-control framing",
			"description": "Bring the sharper control argument into the Primary draft while keeping current evidence.",
			"status": "Clean merge",
			"source": "rev-source",
			"preview": "client control"
		}]
	}`}
	core := agentcore.New(provideriface.Config{}, nil, nil, provider)
	h := NewHandler(core)
	source := types.Revision{RevisionID: "rev-source", Content: "# Proposal\n\nClients control the system."}
	target := types.Revision{RevisionID: "rev-target", Content: "# Proposal\n\nThe system has current evidence."}

	result, evidence, err := h.callTextureSemanticMergeModel(context.Background(), "owner-1", source, target, types.DiffResult{AddedLines: 1, RemovedLines: 1}, "compare", nil, "v4", "v5")
	if err != nil {
		t.Fatalf("model semantic compare: %v", err)
	}
	if provider.calls != 1 {
		t.Fatalf("provider calls = %d, want 1", provider.calls)
	}
	if provider.req.ToolChoice != "none" {
		t.Fatalf("tool choice = %q, want none", provider.req.ToolChoice)
	}
	if provider.req.Model == "" || provider.req.Provider == "" {
		t.Fatalf("provider/model not resolved in request: %+v", provider.req)
	}
	if got := result.Suggestions[0].Label; got != "Restore client-control framing" {
		t.Fatalf("suggestion label = %q", got)
	}
	if strings.Contains(strings.ToLower(result.Suggestions[0].ID+" "+result.Suggestions[0].Label), "restore_glossary") {
		t.Fatalf("semantic merge returned old hard-coded suggestion: %+v", result.Suggestions[0])
	}
	if evidence["model_input_tokens"] != 321 || evidence["model_output_tokens"] != 123 {
		t.Fatalf("token evidence missing: %+v", evidence)
	}
}

func TestTextureSemanticMergePromotesModelSummaryToSuggestion(t *testing.T) {
	result, err := normalizeModelSemanticMergeResult(textureModelSemanticMergeResult{
		Summary: []string{"Earlier draft has a sharper ownership argument."},
	}, types.Revision{RevisionID: "rev-source"}, types.Revision{RevisionID: "rev-target"}, false)
	if err != nil {
		t.Fatalf("normalize summary-only model result: %v", err)
	}
	if len(result.Suggestions) != 1 {
		t.Fatalf("suggestions = %d, want 1", len(result.Suggestions))
	}
	if result.Suggestions[0].ID != "model_finding_1" || result.Suggestions[0].Description != result.Summary[0] {
		t.Fatalf("summary-derived suggestion mismatch: %+v", result.Suggestions[0])
	}
	if strings.Contains(strings.ToLower(result.Suggestions[0].Label), "glossary") {
		t.Fatalf("summary fallback reintroduced domain stub: %+v", result.Suggestions[0])
	}
}

func TestApplyTextureModelMergeEditsStripsVisibleProvenance(t *testing.T) {
	target := "# Proposal\n\nCurrent paragraph.\n\n<!-- Texture merge preview provenance\n- leaked metadata\n-->\n"
	content, applied, err := applyTextureModelMergeEdits(target, []textureModelMergeEdit{{
		SuggestionID: "client_control_frame",
		Operation:    "replace_exact",
		OldText:      "Current paragraph.",
		NewText:      "Current paragraph with restored client-control framing.",
		Rationale:    "Selected source concept improves framing.",
	}})
	if err != nil {
		t.Fatalf("apply edits: %v", err)
	}
	if strings.Contains(content, "Texture merge preview provenance") || strings.Contains(content, "<!--") {
		t.Fatalf("visible provenance leaked into content: %q", content)
	}
	if !strings.Contains(content, "restored client-control framing") {
		t.Fatalf("model edit not applied: %q", content)
	}
	if len(applied) != 1 || applied[0]["operation"] != "replace_exact" {
		t.Fatalf("applied edit evidence mismatch: %+v", applied)
	}
}

func structuredTextureToolRevision(t *testing.T) types.Revision {
	t.Helper()
	bodyDoc, sourceEntities := structuredTextureToolPayload(t)
	return types.Revision{
		RevisionID:     "rev-structured-tool",
		DocID:          "doc-structured-tool",
		OwnerID:        "user-1",
		AuthorKind:     types.AuthorUser,
		AuthorLabel:    "user",
		Content:        "Grounded[1].",
		BodyDoc:        bodyDoc,
		SourceEntities: sourceEntities,
		CreatedAt:      time.Now().UTC().Truncate(time.Millisecond),
	}
}

func plainTextureToolRevision(t *testing.T, content string) types.Revision {
	t.Helper()
	doc := texturedoc.StructuredTextureDoc{
		Schema: texturedoc.SchemaV1,
		Doc: texturedoc.Node{
			Type:  "doc",
			Attrs: map[string]any{"id": "doc-node"},
			Content: []texturedoc.Node{{
				Type:    "paragraph",
				Attrs:   map[string]any{"id": "p-1"},
				Content: []texturedoc.Node{{Type: "text", Text: content}},
			}},
		},
	}
	bodyDoc, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal body_doc: %v", err)
	}
	return types.Revision{
		RevisionID:     "rev-plain-tool",
		DocID:          "doc-plain-tool",
		OwnerID:        "user-1",
		AuthorKind:     types.AuthorUser,
		AuthorLabel:    "user",
		Content:        content,
		BodyDoc:        bodyDoc,
		SourceEntities: json.RawMessage("[]"),
		CreatedAt:      time.Now().UTC().Truncate(time.Millisecond),
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

func structuredDocFirstText(node texturedoc.Node) string {
	if node.Type == "text" {
		return node.Text
	}
	for _, child := range node.Content {
		if text := structuredDocFirstText(child); text != "" {
			return text
		}
	}
	return ""
}

func structuredDocHasNode(node texturedoc.Node, nodeType, nodeID string) bool {
	if node.Type == nodeType && textureNodeStringAttr(node, "id") == nodeID {
		return true
	}
	for _, child := range node.Content {
		if structuredDocHasNode(child, nodeType, nodeID) {
			return true
		}
	}
	return false
}

func TestTextureLifecycleRevisionKeepsWorkOpenUntilExplicitCompletion(t *testing.T) {
	s, _, registry := textureToolCommitRuntime(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Millisecond)
	const (
		ownerID      = "owner-explicit-work"
		computerID   = "sandbox-texture-test"
		docID        = "doc-explicit-work"
		trajectoryID = "trajectory-explicit-work"
		workItemID   = "work-explicit-work"
	)
	agentID := currentTextureAgentID(docID)
	start := types.StartLifecycleRequest{
		OwnerID: ownerID, ComputerID: computerID, CommandID: "start-explicit-work", TrajectoryID: trajectoryID,
		Kind:            types.TrajectoryKindDocument,
		SubjectRefs:     map[string]string{"artifact": "texture://documents/" + docID, "doc_id": docID},
		SettlementRule:  types.SettlementRule{Version: types.LifecycleReducerVersion, RequireNoOpenWorkItems: true, RequiredSubjectRefs: []string{"artifact"}},
		InitialWork:     types.WorkItemRecord{WorkItemID: workItemID, Objective: "Produce a grounded artifact", AssignedAgentID: agentID, AuthorityProfile: agentprofile.Texture},
		InitialDocument: types.Document{DocID: docID, Title: "Explicit work disposition"},
		InitialRevision: types.Revision{RevisionID: "revision-explicit-work-v0", AuthorKind: types.AuthorUser, AuthorLabel: ownerID, Content: "Produce a grounded artifact."},
		Agent:           types.AgentRecord{AgentID: agentID, Profile: agentprofile.Texture, Role: agentprofile.Texture, ChannelID: docID},
	}
	start.StartRequestDigest, _ = store.ComputeStartLifecycleRequestDigest(start)
	if _, err := s.StartLifecycle(ctx, start); err != nil {
		t.Fatalf("start lifecycle: %v", err)
	}
	packet := types.CoagentSourcePacketPayload{
		SchemaVersion: types.CoagentSourcePacketSchemaV1,
		Kind:          "evidence_update",
		Summary:       "Interim evidence checkpoint; material work remains.",
	}
	payloadDigest, err := store.ComputeLifecycleUpdatePayloadDigest(packet, "Interim evidence checkpoint")
	if err != nil {
		t.Fatalf("digest interim update: %v", err)
	}
	queue := types.QueueLifecycleUpdateRequest{
		OwnerID: ownerID, ComputerID: computerID, CommandID: "queue-explicit-work-interim",
		TrajectoryID: trajectoryID, TargetAgentID: agentID, ProducerAgentID: agentID,
		ProducerUpdateID: "update-explicit-work-interim", UpdateID: "update-explicit-work-interim",
		ChannelID: docID, Packet: packet, Content: "Interim evidence checkpoint", PayloadDigest: payloadDigest,
		WorkDisposition: types.WorkItemOpen,
	}
	queue.CommandDigest, _ = store.ComputeQueueLifecycleUpdateDigest(queue)
	if _, err := s.QueueLifecycleUpdate(ctx, queue); err != nil {
		t.Fatalf("queue interim update: %v", err)
	}
	newRun := func(runID, baseRevisionID string) *types.RunRecord {
		run := &types.RunRecord{
			RunID: runID, AgentID: agentID, ChannelID: docID, OwnerID: ownerID, SandboxID: computerID,
			State: types.RunRunning, Prompt: "Revise the artifact.", CreatedAt: now, UpdatedAt: now,
			AgentProfile: agentprofile.Texture, AgentRole: agentprofile.Texture,
			Metadata: map[string]any{
				"type": textureAgentRevisionTaskType, "doc_id": docID, "current_revision_id": baseRevisionID,
				"trajectory_id": trajectoryID, "lifecycle_work_item_id": workItemID,
				runMetadataAgentID: agentID, runMetadataAgentProfile: agentprofile.Texture,
				runMetadataAgentRole: agentprofile.Texture, runMetadataChannelID: docID,
			},
		}
		if err := s.CreateRun(ctx, *run); err != nil {
			t.Fatalf("create run %s: %v", runID, err)
		}
		if err := s.CreateAgentMutation(ctx, store.AgentMutation{DocID: docID, RunID: runID, OwnerID: ownerID, ComputerID: computerID, State: "pending", CreatedAt: now}); err != nil {
			t.Fatalf("create mutation %s: %v", runID, err)
		}
		return run
	}
	omittedRun := newRun("run-explicit-work-omitted", start.InitialRevision.RevisionID)
	omittedArgs, _ := json.Marshal(editTextureArgs{
		DocID: docID, BaseRevisionID: start.InitialRevision.RevisionID,
		Content: "# Ambiguous\n\nNo native work consequence was declared.", Rationale: "Exercise authority refusal.",
	})
	omittedRaw, err := registry.Execute(toolregistry.WithExecutionContext(ctx, textureToolExecutionContext(omittedRun)), "rewrite_texture", omittedArgs)
	if err != nil {
		t.Fatalf("commit omitted disposition revision: %v", err)
	}
	var omittedResult map[string]any
	if err := json.Unmarshal([]byte(omittedRaw), &omittedResult); err != nil || omittedResult["work_disposition"] != "open" {
		t.Fatalf("omitted disposition result = %s, err=%v", omittedRaw, err)
	}
	omittedHead, _ := omittedResult["revision_id"].(string)
	omittedSnapshot, err := s.GetLifecycleSnapshot(ctx, ownerID, computerID, trajectoryID)
	if err != nil || len(omittedSnapshot.WorkItems) != 1 ||
		omittedSnapshot.WorkItems[0].Status != types.WorkItemOpen || omittedSnapshot.HeadRevision.RevisionID != omittedHead {
		t.Fatalf("omitted disposition did not preserve open work: %+v, %v", omittedSnapshot, err)
	}
	openRun := newRun("run-explicit-work-open", omittedHead)
	openRun.Metadata["worker_update_ids"] = []string{queue.UpdateID}
	openArgs, _ := json.Marshal(editTextureArgs{
		DocID: docID, BaseRevisionID: omittedHead,
		Content: "# Interim\n\nOfficial evidence is still required.", Rationale: "Preserve an honest interim artifact.",
		WorkDisposition:    "open",
		UpdateDispositions: []textureUpdateDisposition{{UpdateID: queue.UpdateID, Disposition: "incorporated"}},
	})
	openRaw, err := registry.Execute(toolregistry.WithExecutionContext(ctx, textureToolExecutionContext(openRun)), "rewrite_texture", openArgs)
	if err != nil {
		t.Fatalf("commit open revision: %v", err)
	}
	var openResult map[string]any
	if err := json.Unmarshal([]byte(openRaw), &openResult); err != nil || openResult["work_disposition"] != "open" {
		t.Fatalf("open result = %s, err=%v", openRaw, err)
	}
	openSnapshot, err := s.GetLifecycleSnapshot(ctx, ownerID, computerID, trajectoryID)
	if err != nil {
		t.Fatalf("snapshot open revision: %v", err)
	}
	if len(openSnapshot.WorkItems) != 1 || openSnapshot.WorkItems[0].Status != types.WorkItemOpen ||
		openSnapshot.HeadRevision.RevisionID == start.InitialRevision.RevisionID {
		t.Fatalf("interim revision settled or failed to advance: %+v", openSnapshot)
	}
	if _, err := s.GetDocumentAliasSourcePath(ctx, ownerID, docID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("lifecycle revision created legacy projection alias: %v", err)
	}
	var incorporatedInterim, boundOpenRevision bool
	for _, update := range openSnapshot.Updates {
		if update.UpdateID == queue.UpdateID {
			incorporatedInterim = update.Disposition == types.UpdateIncorporated
		}
		if update.Packet.Kind == "artifact_revision" {
			boundOpenRevision = update.Disposition == types.UpdateIncorporated &&
				update.WorkDisposition == types.WorkItemOpen && update.WorkItemID == start.InitialWork.WorkItemID
		}
	}
	if !incorporatedInterim || !boundOpenRevision {
		t.Fatalf("interim updates omitted incorporation or assigned open work binding: %+v", openSnapshot.Updates)
	}
	rejectedWork := types.OpenLifecycleWorkRequest{
		OwnerID: ownerID, ComputerID: computerID, CommandID: "command-open-rejected-evidence",
		TrajectoryID: trajectoryID, WorkItem: types.WorkItemRecord{
			WorkItemID: "work-rejected-evidence", Objective: "verify rejected evidence",
			AssignedAgentID: start.Agent.AgentID, AuthorityProfile: agentprofile.Texture,
		},
	}
	rejectedWork.CommandDigest, _ = store.ComputeOpenLifecycleWorkDigest(rejectedWork)
	if _, err := s.OpenLifecycleWork(ctx, rejectedWork); err != nil {
		t.Fatalf("open rejected evidence work: %v", err)
	}
	rejectedUpdate := queue
	rejectedUpdate.CommandID = "command-queue-rejected-evidence"
	rejectedUpdate.ProducerUpdateID, rejectedUpdate.UpdateID = "producer-rejected-evidence", "update-rejected-evidence"
	rejectedUpdate.Packet.Summary, rejectedUpdate.Content = "Evidence failed verification.", "Evidence failed verification."
	rejectedUpdate.PayloadDigest, _ = store.ComputeLifecycleUpdatePayloadDigest(rejectedUpdate.Packet, rejectedUpdate.Content)
	rejectedUpdate.WorkDisposition, rejectedUpdate.WorkItemID = types.WorkItemCompleted, rejectedWork.WorkItem.WorkItemID
	rejectedUpdate.CommandDigest, _ = store.ComputeQueueLifecycleUpdateDigest(rejectedUpdate)
	if _, err := s.QueueLifecycleUpdate(ctx, rejectedUpdate); err != nil {
		t.Fatalf("queue rejected evidence: %v", err)
	}
	openRejectedWork := types.OpenLifecycleWorkRequest{
		OwnerID: ownerID, ComputerID: computerID, CommandID: "command-open-rejected-interim",
		TrajectoryID: trajectoryID, WorkItem: types.WorkItemRecord{
			WorkItemID: "work-rejected-interim", Objective: "continue after a rejected interim checkpoint",
			AssignedAgentID: start.Agent.AgentID, AuthorityProfile: agentprofile.Texture,
		},
	}
	openRejectedWork.CommandDigest, _ = store.ComputeOpenLifecycleWorkDigest(openRejectedWork)
	if _, err := s.OpenLifecycleWork(ctx, openRejectedWork); err != nil {
		t.Fatalf("open rejected interim work: %v", err)
	}
	openRejectedUpdate := queue
	openRejectedUpdate.CommandID = "command-queue-rejected-interim"
	openRejectedUpdate.ProducerUpdateID, openRejectedUpdate.UpdateID = "producer-rejected-interim", "update-rejected-interim"
	openRejectedUpdate.Packet.Summary, openRejectedUpdate.Content = "Interim evidence was unusable.", "Interim evidence was unusable."
	openRejectedUpdate.PayloadDigest, _ = store.ComputeLifecycleUpdatePayloadDigest(openRejectedUpdate.Packet, openRejectedUpdate.Content)
	openRejectedUpdate.WorkDisposition, openRejectedUpdate.WorkItemID = types.WorkItemOpen, openRejectedWork.WorkItem.WorkItemID
	openRejectedUpdate.CommandDigest, _ = store.ComputeQueueLifecycleUpdateDigest(openRejectedUpdate)
	if _, err := s.QueueLifecycleUpdate(ctx, openRejectedUpdate); err != nil {
		t.Fatalf("queue rejected interim: %v", err)
	}
	ignoredUpdate := queue
	ignoredUpdate.CommandID = "command-queue-ignored-evidence"
	ignoredUpdate.ProducerUpdateID, ignoredUpdate.UpdateID = "producer-ignored-evidence", "update-ignored-evidence"
	ignoredUpdate.Packet.Summary, ignoredUpdate.Content = "Evidence remains undecided.", "Evidence remains undecided."
	ignoredUpdate.PayloadDigest, _ = store.ComputeLifecycleUpdatePayloadDigest(ignoredUpdate.Packet, ignoredUpdate.Content)
	ignoredUpdate.WorkDisposition, ignoredUpdate.WorkItemID = types.WorkItemOpen, ""
	ignoredUpdate.CommandDigest, _ = store.ComputeQueueLifecycleUpdateDigest(ignoredUpdate)
	if _, err := s.QueueLifecycleUpdate(ctx, ignoredUpdate); err != nil {
		t.Fatalf("queue ignored evidence: %v", err)
	}
	completedRun := newRun("run-explicit-work-completed", openSnapshot.HeadRevision.RevisionID)
	completedArgs, _ := json.Marshal(editTextureArgs{
		DocID: docID, BaseRevisionID: openSnapshot.HeadRevision.RevisionID,
		Content: "# Grounded result\n\nThe required evidence is incorporated.", Rationale: "Complete the assigned artifact.",
		WorkDisposition: "completed",
		UpdateDispositions: []textureUpdateDisposition{
			{UpdateID: rejectedUpdate.UpdateID, Disposition: "rejected", Reason: "evidence failed verification"},
			{UpdateID: openRejectedUpdate.UpdateID, Disposition: "rejected", Reason: "interim evidence was unusable"},
		},
	})
	completedRaw, err := registry.Execute(toolregistry.WithExecutionContext(ctx, textureToolExecutionContext(completedRun)), "rewrite_texture", completedArgs)
	if err != nil {
		t.Fatalf("commit completed revision: %v", err)
	}
	var completedResult map[string]any
	if err := json.Unmarshal([]byte(completedRaw), &completedResult); err != nil || completedResult["work_disposition"] != "completed" {
		t.Fatalf("completed result = %s, err=%v", completedRaw, err)
	}
	completedSnapshot, err := s.GetLifecycleSnapshot(ctx, ownerID, computerID, trajectoryID)
	if err != nil {
		t.Fatalf("snapshot completed revision: %v", err)
	}
	if completedSnapshot.WorkItems[0].Status != types.WorkItemCompleted ||
		completedSnapshot.WorkItems[0].ResultRef != completedSnapshot.HeadRevision.RevisionID {
		t.Fatalf("explicit completion did not settle work: %+v", completedSnapshot)
	}
	var rejectedWorkState, openRejectedWorkState, ignoredUpdateState bool
	for _, work := range completedSnapshot.WorkItems {
		switch work.WorkItemID {
		case rejectedWork.WorkItem.WorkItemID:
			rejectedWorkState = work.Status == types.WorkItemRefused && work.ResultRef == completedSnapshot.HeadRevision.RevisionID
		case openRejectedWork.WorkItem.WorkItemID:
			openRejectedWorkState = work.Status == types.WorkItemOpen && work.ResultRef == ""
		}
	}
	for _, update := range completedSnapshot.Updates {
		if update.UpdateID == ignoredUpdate.UpdateID {
			ignoredUpdateState = update.Disposition == types.UpdatePending
		}
	}
	if !rejectedWorkState || !openRejectedWorkState || !ignoredUpdateState {
		t.Fatalf("explicit rejection or omitted-pending consequence lost: works=%+v updates=%+v", completedSnapshot.WorkItems, completedSnapshot.Updates)
	}
}

func TestTextureEditToolsRefusePresentInvalidWorkDisposition(t *testing.T) {
	_, _, registry := textureToolCommitRuntime(t)
	run := &types.RunRecord{
		RunID: "run-invalid-work-disposition", OwnerID: "owner-invalid-work-disposition",
		AgentID: "texture:doc-invalid-work-disposition", AgentProfile: agentprofile.Texture, AgentRole: agentprofile.Texture,
		Metadata: map[string]any{"type": textureAgentRevisionTaskType},
	}
	ctx := toolregistry.WithExecutionContext(context.Background(), textureToolExecutionContext(run))
	for name, raw := range map[string]json.RawMessage{
		"null":                     json.RawMessage(`{"doc_id":"doc","base_revision_id":"rev","content":"next","rationale":"test","work_disposition":null}`),
		"blank":                    json.RawMessage(`{"doc_id":"doc","base_revision_id":"rev","content":"next","rationale":"test","work_disposition":" "}`),
		"unknown":                  json.RawMessage(`{"doc_id":"doc","base_revision_id":"rev","content":"next","rationale":"test","work_disposition":"done"}`),
		"misspelled":               json.RawMessage(`{"doc_id":"doc","base_revision_id":"rev","content":"next","rationale":"test","work_dispositon":"completed"}`),
		"updates_null":             json.RawMessage(`{"doc_id":"doc","base_revision_id":"rev","content":"next","rationale":"test","work_disposition":"open","update_dispositions":null}`),
		"updates_object":           json.RawMessage(`{"doc_id":"doc","base_revision_id":"rev","content":"next","rationale":"test","work_disposition":"open","update_dispositions":{}}`),
		"updates_unknown":          json.RawMessage(`{"doc_id":"doc","base_revision_id":"rev","content":"next","rationale":"test","work_disposition":"open","update_dispositions":[{"update_id":"u","disposition":"incorporated","receipt":"fake"}]}`),
		"rejection_without_reason": json.RawMessage(`{"doc_id":"doc","base_revision_id":"rev","content":"next","rationale":"test","work_disposition":"open","update_dispositions":[{"update_id":"u","disposition":"rejected"}]}`),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := registry.Execute(ctx, "rewrite_texture", raw); err == nil {
				t.Fatalf("rewrite_texture accepted invalid work disposition: %s", raw)
			}
		})
	}
}

func TestLifecycleTextureEditsAndInjectionAreComputerScopedAcrossRestart(t *testing.T) {
	ctx := context.Background()
	dir := filepath.Join(os.TempDir(), "go-choir-texture-computer-scope-test")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	dbPath := filepath.Join(dir, t.Name()+".db")
	promptRoot := filepath.Join(dir, t.Name()+"-prompts")
	_ = os.Remove(dbPath)
	_ = os.RemoveAll(promptRoot)
	t.Cleanup(func() {
		_ = os.Remove(dbPath)
		_ = os.RemoveAll(promptRoot)
	})
	openRuntime := func() (*store.Store, *agentcore.Runtime, *Handler, *toolregistry.ToolRegistry) {
		t.Helper()
		s, err := store.Open(dbPath)
		if err != nil {
			t.Fatalf("open store: %v", err)
		}
		core := agentcore.New(provideriface.Config{
			SandboxID: "computer-b", StorePath: dbPath, PromptRoot: promptRoot,
			ProviderTimeout: time.Second, SupervisionInterval: time.Hour,
		}, s, events.NewEventBus(), provider.NewStubProvider(0))
		handler := NewHandler(core)
		registry := toolregistry.NewToolRegistry()
		if err := RegisterTools(registry, handler); err != nil {
			t.Fatalf("register tools: %v", err)
		}
		return s, core, handler, registry
	}
	s, core, handler, registry := openRuntime()
	const (
		ownerID = "owner-computer-collision"
		docID   = "doc-computer-collision"
	)
	agentID := currentTextureAgentID(docID)
	starts := make(map[string]types.StartLifecycleRequest)
	for _, computerID := range []string{"computer-a", "computer-b"} {
		start := types.StartLifecycleRequest{
			OwnerID: ownerID, ComputerID: computerID, CommandID: "start-" + computerID,
			TrajectoryID: "trajectory-" + computerID, Kind: types.TrajectoryKindDocument,
			SubjectRefs:    map[string]string{"artifact": "texture://documents/" + docID, "doc_id": docID},
			SettlementRule: types.SettlementRule{Version: types.LifecycleReducerVersion, RequireNoOpenWorkItems: true, RequiredSubjectRefs: []string{"artifact"}},
			InitialWork: types.WorkItemRecord{
				WorkItemID: "work-" + computerID, Objective: "revise scoped document",
				AssignedAgentID: agentID, AuthorityProfile: agentprofile.Texture,
			},
			InitialDocument: types.Document{DocID: docID, Title: "Scoped " + computerID},
			InitialRevision: types.Revision{
				RevisionID: "revision-" + computerID + "-v0", AuthorKind: types.AuthorUser,
				AuthorLabel: ownerID, Content: "Initial " + computerID,
			},
			Agent: types.AgentRecord{AgentID: agentID, Profile: agentprofile.Texture, Role: agentprofile.Texture, ChannelID: docID},
		}
		start.StartRequestDigest, _ = store.ComputeStartLifecycleRequestDigest(start)
		if _, err := s.StartLifecycle(ctx, start); err != nil {
			t.Fatalf("start %s lifecycle: %v", computerID, err)
		}
		packet := types.CoagentSourcePacketPayload{
			SchemaVersion: types.CoagentSourcePacketSchemaV1, Kind: "evidence_update",
			Summary: "pending " + computerID,
		}
		content := "pending " + computerID
		payloadDigest, _ := store.ComputeLifecycleUpdatePayloadDigest(packet, content)
		queue := types.QueueLifecycleUpdateRequest{
			OwnerID: ownerID, ComputerID: computerID, CommandID: "queue-" + computerID,
			TrajectoryID: start.TrajectoryID, TargetAgentID: agentID, ProducerAgentID: agentID,
			ProducerUpdateID: "producer-" + computerID, UpdateID: "update-" + computerID,
			ChannelID: docID, Packet: packet, Content: content, PayloadDigest: payloadDigest,
			WorkDisposition: types.WorkItemOpen, WorkItemID: start.InitialWork.WorkItemID,
		}
		queue.CommandDigest, _ = store.ComputeQueueLifecycleUpdateDigest(queue)
		if _, err := s.QueueLifecycleUpdate(ctx, queue); err != nil {
			t.Fatalf("queue %s update: %v", computerID, err)
		}
		starts[computerID] = start
	}
	if unscoped, err := s.ListCoagentMailboxBacklog(ctx, ownerID, agentID, 100); err != nil || len(unscoped) != 0 {
		t.Fatalf("owner-only mailbox exposed lifecycle updates: %+v, %v", unscoped, err)
	}
	if docs, err := s.ListDocumentsByOwner(ctx, ownerID, 100); err != nil || len(docs) != 0 {
		t.Fatalf("owner-only document list exposed lifecycle documents: %+v, %v", docs, err)
	}
	if revisions, err := s.ListRevisionsByDoc(ctx, docID, ownerID, 100); err != nil || len(revisions) != 0 {
		t.Fatalf("owner-only revision list exposed lifecycle revisions: %+v, %v", revisions, err)
	}
	for _, computerID := range []string{"computer-a", "computer-b"} {
		docs, err := s.ListDocumentsByScope(ctx, ownerID, computerID, 100)
		if err != nil || len(docs) != 1 || docs[0].ComputerID != computerID || docs[0].Title != "Scoped "+computerID {
			t.Fatalf("scoped documents on %s = %+v, %v", computerID, docs, err)
		}
		revisions, err := s.ListRevisionsByScope(ctx, docID, ownerID, computerID, 100)
		if err != nil || len(revisions) != 1 || revisions[0].ComputerID != computerID || revisions[0].Content != "Initial "+computerID {
			t.Fatalf("scoped revisions on %s = %+v, %v", computerID, revisions, err)
		}
	}
	docs, err := handler.listTextureDocuments(ctx, ownerID, 100)
	if err != nil || len(docs) != 1 || docs[0].ComputerID != "computer-b" {
		t.Fatalf("computer-b product document list = %+v, %v", docs, err)
	}
	revisions, err := handler.listTextureRevisions(ctx, ownerID, docID, 100)
	if err != nil || len(revisions) != 1 || revisions[0].ComputerID != "computer-b" {
		t.Fatalf("computer-b product revision list = %+v, %v", revisions, err)
	}
	history, err := handler.getTextureHistory(ctx, ownerID, docID, 100)
	if err != nil || len(history) != 1 || history[0].RevisionID != "revision-computer-b-v0" {
		t.Fatalf("computer-b product history = %+v, %v", history, err)
	}
	blameRequest := httptest.NewRequest(http.MethodGet, "/api/texture/revisions/revision-computer-b-v0/blame", nil)
	blameRequest.Header.Set("X-Authenticated-User", ownerID)
	blameRecorder := httptest.NewRecorder()
	handler.HandleTextureBlame(blameRecorder, blameRequest)
	if blameRecorder.Code != http.StatusOK {
		t.Fatalf("computer-b blame status = %d, body=%s", blameRecorder.Code, blameRecorder.Body.String())
	}
	var blame textureBlameResponse
	if err := json.Unmarshal(blameRecorder.Body.Bytes(), &blame); err != nil ||
		blame.RevisionID != "revision-computer-b-v0" || blame.DocID != docID {
		t.Fatalf("computer-b blame = %+v, %v", blame, err)
	}
	newRun := func(runID, baseRevisionID string) *types.RunRecord {
		return &types.RunRecord{
			RunID: runID, AgentID: agentID, ChannelID: docID, OwnerID: ownerID, SandboxID: "computer-b",
			State: types.RunRunning, Prompt: "Revise the scoped artifact.", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
			AgentProfile: agentprofile.Texture, AgentRole: agentprofile.Texture,
			Metadata: map[string]any{
				"type": textureAgentRevisionTaskType, "doc_id": docID, "current_revision_id": baseRevisionID,
				"trajectory_id":          starts["computer-b"].TrajectoryID,
				"lifecycle_work_item_id": starts["computer-b"].InitialWork.WorkItemID,
				runMetadataAgentID:       agentID, runMetadataAgentProfile: agentprofile.Texture,
				runMetadataAgentRole: agentprofile.Texture, runMetadataChannelID: docID,
			},
		}
	}
	assertScopedInjection := func(h *Handler, rec *types.RunRecord) {
		t.Helper()
		inject := h.coagentUpdateTurnInjector(rec)
		if inject == nil {
			t.Fatal("scoped lifecycle injector is nil")
		}
		messages, err := inject(false)
		if err != nil || len(messages) != 1 {
			t.Fatalf("inject scoped updates: %d messages, %v", len(messages), err)
		}
		if !messageTextContains(t, messages[0], "update-computer-b") ||
			messageTextContains(t, messages[0], "update-computer-a") {
			t.Fatalf("cross-computer update injection: %s", string(messages[0]))
		}
	}
	commitScopedEdit := func(s *store.Store, registry *toolregistry.ToolRegistry, runID, baseRevisionID, content string) string {
		t.Helper()
		run := newRun(runID, baseRevisionID)
		if err := s.CreateRun(ctx, *run); err != nil {
			t.Fatalf("create scoped edit run: %v", err)
		}
		if err := s.CreateAgentMutation(ctx, store.AgentMutation{DocID: docID, RunID: runID, OwnerID: ownerID, ComputerID: "computer-b", State: "pending", CreatedAt: time.Now().UTC()}); err != nil {
			t.Fatalf("create scoped mutation: %v", err)
		}
		args, _ := json.Marshal(editTextureArgs{
			DocID: docID, BaseRevisionID: baseRevisionID, Content: content,
			Rationale: "Prove owner/computer scoped lifecycle edit.",
		})
		if _, err := registry.Execute(toolregistry.WithExecutionContext(ctx, textureToolExecutionContext(run)), "rewrite_texture", args); err != nil {
			t.Fatalf("commit scoped edit: %v", err)
		}
		snapshot, err := s.GetLifecycleSnapshot(ctx, ownerID, "computer-b", starts["computer-b"].TrajectoryID)
		if err != nil {
			t.Fatalf("load computer-b snapshot: %v", err)
		}
		if len(snapshot.WorkItems) != 1 || snapshot.WorkItems[0].Status != types.WorkItemOpen {
			t.Fatalf("omitted work_disposition changed assigned work: %+v", snapshot.WorkItems)
		}
		foundOpenUpdate := false
		for _, update := range snapshot.Updates {
			if update.Disposition == types.UpdateIncorporated && update.WorkDisposition == types.WorkItemOpen {
				foundOpenUpdate = true
			}
		}
		if !foundOpenUpdate {
			t.Fatalf("omitted work_disposition did not fail open: %+v", snapshot.Updates)
		}
		other, err := s.GetLifecycleSnapshot(ctx, ownerID, "computer-a", starts["computer-a"].TrajectoryID)
		if err != nil || other.HeadRevision.RevisionID != starts["computer-a"].InitialRevision.RevisionID {
			t.Fatalf("computer-a artifact changed: %+v, %v", other.HeadRevision, err)
		}
		return snapshot.HeadRevision.RevisionID
	}

	injectionRun := newRun("run-inject-before-restart", starts["computer-b"].InitialRevision.RevisionID)
	assertScopedInjection(handler, injectionRun)
	firstHead := commitScopedEdit(s, registry, "run-edit-before-restart", starts["computer-b"].InitialRevision.RevisionID, "# Computer B v1\n\nScoped before restart.")
	core.Stop()
	if err := s.Close(); err != nil {
		t.Fatalf("close pre-restart store: %v", err)
	}

	s, core, handler, registry = openRuntime()
	defer core.Stop()
	defer s.Close()
	assertScopedInjection(handler, newRun("run-inject-after-restart", firstHead))
	secondHead := commitScopedEdit(s, registry, "run-edit-after-restart", firstHead, "# Computer B v2\n\nScoped after restart.")
	if secondHead == firstHead {
		t.Fatalf("computer-b head did not advance after restart: %s", secondHead)
	}
}

func textureToolCommitRuntime(t *testing.T) (*store.Store, *Handler, *toolregistry.ToolRegistry) {
	t.Helper()
	dir := filepath.Join(os.TempDir(), "go-choir-d4-texture-tool-test")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	dbPath := filepath.Join(dir, t.Name()+".db")
	promptRoot := filepath.Join(dir, t.Name()+"-prompts")
	_ = os.Remove(dbPath)
	_ = os.RemoveAll(promptRoot)
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	core := agentcore.New(provideriface.Config{
		SandboxID:           "sandbox-texture-test",
		StorePath:           dbPath,
		PromptRoot:          promptRoot,
		ProviderTimeout:     time.Second,
		SupervisionInterval: time.Hour,
	}, s, events.NewEventBus(), provider.NewStubProvider(0))
	h := NewHandler(core)
	registry := toolregistry.NewToolRegistry()
	if err := RegisterTools(registry, h); err != nil {
		t.Fatalf("RegisterTools: %v", err)
	}
	t.Cleanup(func() {
		core.Stop()
		_ = s.Close()
		_ = os.Remove(dbPath)
		_ = os.RemoveAll(promptRoot)
	})
	return s, h, registry
}

func textureToolExecutionContext(run *types.RunRecord) toolregistry.ExecutionContext {
	if run == nil {
		return toolregistry.ExecutionContext{}
	}
	return toolregistry.ExecutionContext{
		RunID:     run.RunID,
		AgentID:   run.AgentID,
		OwnerID:   run.OwnerID,
		Profile:   run.AgentProfile,
		Role:      run.AgentRole,
		ChannelID: run.ChannelID,
		SandboxID: run.SandboxID,
		RunRecord: run,
	}
}
