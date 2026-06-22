package runtime

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/texturedoc"
)

func TestMarkdownLineageStructuredRevisionParsesHeadingsListsAndRefs(t *testing.T) {
	bodyDoc, sourceEntities, projected, err := markdownLineageStructuredRevision(
		"doc-markdown-structure",
		"rev-markdown-structure",
		"# What's new\n\n## Short answer\n\nThe story is shipping [1].\n\n- First point\n- Second point",
		[]textureSourceEntity{{
			EntityID: "src-news",
			Kind:     "web_url",
			Label:    "Newsroom",
			Target:   textureSourceEntityTarget{TargetKind: "web_url", URL: "https://example.com/news", CanonicalURL: "https://example.com/news"},
		}},
		[]textureCitationMarkerResolution{{Marker: "[1]", EntityID: "src-news"}},
	)
	if err != nil {
		t.Fatalf("markdownLineageStructuredRevision: %v", err)
	}
	var doc texturedoc.StructuredTextureDoc
	if err := json.Unmarshal(bodyDoc, &doc); err != nil {
		t.Fatalf("unmarshal body_doc: %v", err)
	}
	if len(doc.Doc.Content) != 4 {
		t.Fatalf("block count = %d, want 4: %#v", len(doc.Doc.Content), doc.Doc.Content)
	}
	if doc.Doc.Content[0].Type != "heading" || doc.Doc.Content[1].Type != "heading" || doc.Doc.Content[2].Type != "paragraph" || doc.Doc.Content[3].Type != "bullet_list" {
		t.Fatalf("unexpected block types: %#v", doc.Doc.Content)
	}
	if strings.Contains(firstTextNode(doc.Doc.Content[0]), "#") || strings.Contains(firstTextNode(doc.Doc.Content[1]), "##") {
		t.Fatalf("heading markdown marker leaked into text nodes: %#v", doc.Doc.Content[:2])
	}
	if !structuredDocHasType(doc.Doc, "source_ref") {
		t.Fatalf("structured doc missing native source_ref: %s", bodyDoc)
	}
	var entities []texturedoc.SourceEntity
	if err := json.Unmarshal(sourceEntities, &entities); err != nil {
		t.Fatalf("unmarshal source_entities: %v", err)
	}
	if len(entities) != 1 || entities[0].SourceEntityID != "src-news" {
		t.Fatalf("source_entities = %#v, want src-news", entities)
	}
	if !strings.Contains(projected, "# What's new\n\n## Short answer") || !strings.Contains(projected, "- First point") {
		t.Fatalf("projection did not preserve markdown-compatible structure: %q", projected)
	}
}

func firstTextNode(node texturedoc.Node) string {
	if node.Type == "text" {
		return node.Text
	}
	for _, child := range node.Content {
		if text := firstTextNode(child); text != "" {
			return text
		}
	}
	return ""
}
