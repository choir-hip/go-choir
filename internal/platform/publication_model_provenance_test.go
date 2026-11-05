package platform

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// A published version pins the source document's model provenance into its
// manifest, so a reader of the paper sees which models wrote it; a version
// published without it carries none, and a malformed one is refused
// (docs/problems/model-provenance-unwired-2026-11-05.md).
func TestPublishedVersionPinsModelProvenance(t *testing.T) {
	svc, _ := newVersionTestService(t)
	ctx := context.Background()
	provenance := json.RawMessage(`{"schema":"choir.model_provenance.v1","runs":2,"runs_served":2,"roles":[{"role":"research","models":[{"provider":"chatgpt","model":"gpt-6-luna","calls":3,"runs":1,"via":["model_failover"]}]}]}`)
	resp, err := svc.PublishTexture(ctx, PublishTextureRequest{
		OwnerID: "owner-1", SourceDocID: "doc-models", SourceRevisionID: "rev-1", RevisionNumber: 1,
		Title: "Models Story", Content: "# Models\n\nA story.", RequestedBy: "owner-1", ModelProvenance: provenance,
	})
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	bundle, err := svc.GetPublicationBundleByRoute(ctx, resp.RoutePath)
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		ModelProvenance json.RawMessage `json:"model_provenance"`
	}
	if err := json.Unmarshal(bundle.Payload.Manifest, &manifest); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(manifest.ModelProvenance), `"gpt-6-luna"`) || !strings.Contains(string(manifest.ModelProvenance), `"model_failover"`) {
		t.Fatalf("manifest model_provenance = %s", manifest.ModelProvenance)
	}

	plain, err := svc.PublishTexture(ctx, PublishTextureRequest{
		OwnerID: "owner-1", SourceDocID: "doc-plain", SourceRevisionID: "rev-1", RevisionNumber: 1,
		Title: "Plain Story", Content: "# Plain", RequestedBy: "owner-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	plainBundle, err := svc.GetPublicationBundleByRoute(ctx, plain.RoutePath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(plainBundle.Payload.Manifest), "model_provenance") {
		t.Fatalf("a version published without provenance names one: %s", plainBundle.Payload.Manifest)
	}

	if _, err := svc.PublishTexture(ctx, PublishTextureRequest{
		OwnerID: "owner-1", SourceDocID: "doc-bad", SourceRevisionID: "rev-1", RevisionNumber: 1,
		Title: "Bad", Content: "# Bad", RequestedBy: "owner-1", ModelProvenance: json.RawMessage(`[1]`),
	}); err == nil {
		t.Fatal("a non-object model_provenance was published")
	}
}
