package platform

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
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

// B1 (review r1): the public manifest carries the projection only. A
// provenance built from runs selected by the computer's file-backed policy
// and a management-task overlay, with run and document ids, pins none of
// them.
func TestPublishedModelProvenanceCarriesNoInternalDetail(t *testing.T) {
	svc, _ := newVersionTestService(t)
	ctx := context.Background()
	const (
		policyPath  = "/home/choir/files/System/model-policy.toml"
		overlayPath = "/home/choir/files/System/model-policy-overlays/task_01JSECRETOVERLAY.toml"
		runID       = "run-7f3e9c1a-internal"
		otherRunID  = "run-b44d0e2f-internal"
		docID       = "doc-internal-4411"
	)
	var b types.ModelProvenanceBuilder
	b.AddServed(runID, "research", types.ModelServedRecord{Schema: types.ModelServedSchema, Purpose: types.ModelServedPurposeToolLoop,
		SelectedProvider: "opencode-go", SelectedModel: "deepseek-v4.1-flash", SelectedSource: policyPath,
		Models: []types.ModelServedUsage{
			{Provider: "opencode-go", Model: "deepseek-v4.1-flash", Calls: 2},
			{Provider: "chatgpt", Model: "gpt-6-luna", ReportedModel: "gpt-6-luna-2026-10", Calls: 3},
		},
		Switches: []types.ModelSwitch{{FromProvider: "opencode-go", FromModel: "deepseek-v4.1-flash", ToProvider: "chatgpt", ToModel: "gpt-6-luna", Reason: "provider_quota"}}})
	b.AddServed(otherRunID, "management_task", types.ModelServedRecord{Schema: types.ModelServedSchema, SelectedProvider: "chatgpt",
		SelectedModel: "gpt-6", SelectedSource: overlayPath, Models: []types.ModelServedUsage{{Provider: "chatgpt", Model: "gpt-6", Calls: 1}}})
	b.AddSelectionOnly("run-old-internal", "texture", "chatgpt", "gpt-6", policyPath+"/pool")
	b.AddDelegate(otherRunID, "engineering", "claude-sonnet-5-5", 2)
	full, _ := json.Marshal(b.Build(docID))
	for _, secret := range []string{policyPath, overlayPath, runID, docID} {
		if !strings.Contains(string(full), secret) {
			t.Fatalf("fixture: the owner's record does not carry %q, so the test proves nothing", secret)
		}
	}
	resp, err := svc.PublishTexture(ctx, PublishTextureRequest{
		OwnerID: "owner-1", SourceDocID: "doc-public-proj", SourceRevisionID: "rev-1", RevisionNumber: 1,
		Title: "Projection Story", Content: "# Projection\n\nA story.", RequestedBy: "owner-1", ModelProvenance: full,
	})
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := svc.GetPublicationBundleByRoute(ctx, resp.RoutePath)
	if err != nil {
		t.Fatal(err)
	}
	manifest := string(bundle.Payload.Manifest)
	for _, leak := range []string{"/home/", "model-policy", "task_01JSECRETOVERLAY", "run-", docID, `"source"`, `"run_id"`, `"doc_id"`, `"runs"`} {
		if strings.Contains(manifest, leak) {
			t.Fatalf("public manifest carries %q:\n%s", leak, manifest)
		}
	}
	for _, want := range []string{`"gpt-6-luna"`, `"gpt-6-luna-2026-10"`, `"provider_quota"`, `"claude-sonnet-5-5"`, `"delegate":true`, types.PublicModelProvenanceSchema} {
		if !strings.Contains(manifest, want) {
			t.Fatalf("public manifest lacks %s:\n%s", want, manifest)
		}
	}
}
