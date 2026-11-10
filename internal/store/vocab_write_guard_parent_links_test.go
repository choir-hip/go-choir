package store

// After the vocabulary cutover the object-graph write guard reads every leaf
// named "role" in an object body as a desk role. A revision's provenance
// parent links used to carry a "role" key (base, source), so on a cut-over
// computer every commit of a revision with links was refused: "vocabfence:
// ...parent_links[0].role="base" not in v2 vocabulary" (staging, d@e,
// a71202161: fork, take, merge and an older-base comment all failed at the
// commit). Package tests never run with the cutover active, so nothing
// caught it; this one commits through the real lifecycle path with it on.

import (
	"context"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestCommitLifecycleHeadWithParentLinksAfterCutover(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	start := lifecycleStartFixture()
	if _, err := s.StartLifecycle(ctx, start); err != nil {
		t.Fatalf("start lifecycle: %v", err)
	}
	s.vocabCutover.Store(true)
	prov, err := types.Provenance{SchemaVersion: types.ProvenanceSchemaVersion, ParentLinks: []types.ParentLink{
		{Kind: types.ParentLinkBase, RevisionID: start.InitialRevision.RevisionID},
		{Kind: types.ParentLinkSource, PublicationVersionID: "pub-a-v1", ManifestHash: "mh", RoutePath: "/pub/texture/a/v1", AuthorLabel: "Autopaper"},
	}}.CanonicalJSON()
	if err != nil {
		t.Fatal(err)
	}
	commit := types.CommitLifecycleArtifactHeadRequest{
		OwnerID: start.OwnerID, ComputerID: start.ComputerID,
		CommandID: "commit-head-links", TrajectoryID: start.TrajectoryID,
		ExpectedLifecycleVersion: 1, ExpectedHeadRevisionID: start.InitialRevision.RevisionID,
		Revision: types.Revision{RevisionID: "revision-with-links", AuthorKind: types.AuthorUser, AuthorLabel: "owner", Content: "Owner revision", Provenance: prov},
	}
	commit.CommandDigest, _ = ComputeCommitLifecycleArtifactHeadDigest(commit)
	if _, err := s.CommitLifecycleArtifactHead(ctx, commit); err != nil {
		t.Fatalf("a revision with parent links was refused after the cutover: %v", err)
	}
	// The guard is live in this test: a desk role it does not know is refused.
	bad := commit
	bad.CommandID, bad.Revision.RevisionID = "commit-head-bad", "revision-bad"
	bad.ExpectedLifecycleVersion, bad.ExpectedHeadRevisionID = 2, commit.Revision.RevisionID
	bad.Revision.Provenance = []byte(`{"schema_version":1,"authoring_model":{},"authored_at":"0001-01-01T00:00:00Z","x":{"role":"boss"}}`)
	bad.CommandDigest, _ = ComputeCommitLifecycleArtifactHeadDigest(bad)
	if _, err := s.CommitLifecycleArtifactHead(ctx, bad); err == nil {
		t.Fatal("the write guard accepted an unknown role: it is not active in this test")
	}
}
