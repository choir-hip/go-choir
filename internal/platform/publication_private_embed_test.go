package platform

// Station TE (docs/design/texture-evidence-embeds-te-2026-10-25.md E6):
// capsule_evidence is never published, and no embed kind is publishable
// until public asset pinning exists. Ways the publish path could leak one,
// written before the code:
//   - the platform publishes a document that holds a private embed;
//   - it strips the embed and publishes a document whose bytes differ from
//     the revision it names, silently;
//   - the refusal does not say why, so the owner cannot act on it;
//   - a document with no embed is refused.

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestPublishTextureRefusesAPrivateEmbed(t *testing.T) {
	store, _ := openTestPlatformStore(t)
	svc := NewService(store, t.TempDir(), "")
	digest := strings.Repeat("a", 64)
	for name, embed := range map[string]string{
		"capsule_evidence": `{"type":"embed","attrs":{"id":"embed-1","kind":"capsule_evidence","target":{"digest":"` + digest + `","operation_id":"selfdev-op-1","computer_id":"computer-1","route":"/reader","width":1280,"height":800,"blank":false},"caption":"Screenshot of /reader","alt":"Screenshot of /reader"}}`,
		"image":            `{"type":"embed","attrs":{"id":"embed-2","kind":"image","target":{"digest":"` + digest + `"},"alt":"A diagram"}}`,
	} {
		body := json.RawMessage(`{"schema":"choir.texture_doc.v1","doc":{"type":"doc","attrs":{"id":"doc-embed"},"content":[` +
			`{"type":"paragraph","attrs":{"id":"p1"},"content":[{"type":"text","text":"Evidence follows."}]},` + embed + `]}}`)
		_, err := svc.PublishTexture(context.Background(), PublishTextureRequest{
			OwnerID: "user-1", SourceDocID: "doc-embed-" + name, SourceRevisionID: "rev-1", RevisionNumber: 1,
			Title: "Embed", BodyDoc: body, SourceEntities: json.RawMessage(`[]`), RequestedBy: "user-1", RevisionHash: "revhash-1",
		})
		if err == nil || !strings.Contains(err.Error(), "private embed") || !strings.Contains(err.Error(), name) {
			t.Fatalf("%s: publish = %v; want a refusal naming the private embed", name, err)
		}
	}
	plain := json.RawMessage(`{"schema":"choir.texture_doc.v1","doc":{"type":"doc","attrs":{"id":"doc-plain"},"content":[{"type":"paragraph","attrs":{"id":"p1"},"content":[{"type":"text","text":"No evidence."}]}]}}`)
	if _, err := svc.PublishTexture(context.Background(), PublishTextureRequest{
		OwnerID: "user-1", SourceDocID: "doc-plain", SourceRevisionID: "rev-1", RevisionNumber: 1,
		Title: "Plain", BodyDoc: plain, SourceEntities: json.RawMessage(`[]`), RequestedBy: "user-1", RevisionHash: "revhash-1",
	}); err != nil {
		t.Fatalf("a document with no embed was refused: %v", err)
	}
}
