package platform

import (
	"context"
	"path/filepath"
	"testing"
)

func TestPublishTexturePersistsAndServesVersionHistory(t *testing.T) {
	store, root := openTestPlatformStore(t)
	svc := NewService(store, filepath.Join(root, "artifacts"))

	history := []PublishTextureRevision{
		{
			RevisionID:    "rev-1",
			VersionNumber: 1,
			AuthorKind:    "appagent",
			Content:       "# Draft\n\nFirst pass.",
			Provenance:    []byte(`{"schema_version":"v0","authoring_model":"m"}`),
			RevisionHash:  "rev1hash",
			CreatedAt:     "2026-01-01T00:00:00.000Z",
		},
		{
			RevisionID:       "rev-2",
			ParentRevisionID: "rev-1",
			VersionNumber:    2,
			AuthorKind:       "appagent",
			Content:          "# Draft\n\nSecond pass, deeper.",
			Provenance:       []byte(`{"schema_version":"v0","authoring_model":"m"}`),
			RevisionHash:     "rev2hash",
			CreatedAt:        "2026-01-02T00:00:00.000Z",
		},
	}

	resp, err := svc.PublishTexture(context.Background(), PublishTextureRequest{
		OwnerID:          "user-1",
		SourceDocID:      "doc-1",
		SourceRevisionID: "rev-2",
		Title:            "Draft",
		Content:          "# Draft\n\nSecond pass, deeper.",
		RequestedBy:      "user-1",
		History:          history,
	})
	if err != nil {
		t.Fatalf("PublishTexture: %v", err)
	}
	if resp.VersionCount != 2 {
		t.Fatalf("response version count = %d, want 2", resp.VersionCount)
	}
	if resp.VersionHistoryHash == "" {
		t.Fatal("response missing version history hash")
	}

	bundle, err := svc.GetPublicationBundleByRoute(context.Background(), resp.RoutePath)
	if err != nil {
		t.Fatalf("GetPublicationBundleByRoute: %v", err)
	}
	if bundle.VersionHistory == nil {
		t.Fatal("bundle missing version history")
	}
	if bundle.VersionHistory.RevisionCount != 2 {
		t.Fatalf("bundle history count = %d, want 2", bundle.VersionHistory.RevisionCount)
	}
	if bundle.VersionHistory.ManifestHash != resp.VersionHistoryHash {
		t.Fatalf("bundle history hash = %q, want %q", bundle.VersionHistory.ManifestHash, resp.VersionHistoryHash)
	}
	if bundle.VersionHistory.ChainHeadHash != "rev2hash" {
		t.Fatalf("chain head hash = %q, want rev2hash", bundle.VersionHistory.ChainHeadHash)
	}
	if got := bundle.VersionHistory.Revisions[0].RevisionID; got != "rev-1" {
		t.Fatalf("first history revision = %q, want rev-1 (oldest first)", got)
	}
	if bundle.VersionHistory.Revisions[0].Content != "# Draft\n\nFirst pass." {
		t.Fatalf("first revision content not preserved: %q", bundle.VersionHistory.Revisions[0].Content)
	}
	if len(bundle.VersionHistory.Revisions[1].Provenance) == 0 {
		t.Fatal("head revision provenance not preserved in history")
	}
}

func TestPublishTextureWithoutHistoryOmitsVersionHistory(t *testing.T) {
	store, root := openTestPlatformStore(t)
	svc := NewService(store, filepath.Join(root, "artifacts"))

	resp, err := svc.PublishTexture(context.Background(), PublishTextureRequest{
		OwnerID:          "user-1",
		SourceDocID:      "doc-1",
		SourceRevisionID: "rev-1",
		Title:            "Draft",
		Content:          "# Draft\n\nOnly head.",
		RequestedBy:      "user-1",
	})
	if err != nil {
		t.Fatalf("PublishTexture: %v", err)
	}
	if resp.VersionHistoryHash != "" || resp.VersionCount != 0 {
		t.Fatalf("head-only publish should not report history: hash=%q count=%d", resp.VersionHistoryHash, resp.VersionCount)
	}
	bundle, err := svc.GetPublicationBundleByRoute(context.Background(), resp.RoutePath)
	if err != nil {
		t.Fatalf("GetPublicationBundleByRoute: %v", err)
	}
	if bundle.VersionHistory != nil {
		t.Fatalf("head-only publish should have nil version history, got %#v", bundle.VersionHistory)
	}
}
