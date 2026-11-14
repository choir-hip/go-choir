package store

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// Step 2: the writer names the revision its batch writes instead of embedding
// a copy. Step 1's tests (lifecycle_stored_result_test.go) pin the reader.

func TestLifecycleCommandNamesItsRevisionInsteadOfEmbeddingIt(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	start := lifecycleStartFixture()
	if _, err := s.StartLifecycle(ctx, start); err != nil {
		t.Fatal(err)
	}
	commit := storedResultCommit(start, "stored-result-commit-1", "stored-result-rev-1", 1, start.InitialRevision.RevisionID)
	if _, err := s.CommitLifecycleArtifactHead(ctx, commit); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ id, revision string }{
		{start.CommandID, start.InitialRevision.RevisionID},
		{commit.CommandID, commit.Revision.RevisionID},
	} {
		stored := storedResultKeys(t, storedResultCommandObject(t, s, start, c.id))
		if _, embedded := stored["revision"]; embedded {
			t.Fatalf("%s still embeds its revision", c.id)
		}
		var named string
		if err := json.Unmarshal(stored["revision_id"], &named); err != nil || named != c.revision {
			t.Fatalf("%s names revision %q, want %q (%v)", c.id, named, c.revision, err)
		}
	}
	// The measured size: the command op body no longer carries the revision body.
	cmd := storedResultCommandObject(t, s, start, commit.CommandID)
	rev, err := s.lifecycleGetObject(ctx, ogKindTexRev, start.OwnerID, start.ComputerID, commit.Revision.RevisionID)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(cmd.Body, []byte(storedResultMarker)) {
		t.Fatal("command body still contains the revision body")
	}
	if len(cmd.Body)*4 > len(rev.Body) {
		t.Fatalf("command body %d bytes is not far below the revision body %d bytes", len(cmd.Body), len(rev.Body))
	}
	t.Logf("command body %d bytes, revision body %d bytes", len(cmd.Body), len(rev.Body))
	// A batch of both writes carries each body once.
	if n := bytes.Count(cmd.Body, []byte("body text")); n != 0 {
		t.Fatalf("command body carries %d copies of the revision text", n)
	}
}

func TestLifecycleCommandFormsAreCounted(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	start := lifecycleStartFixture()
	if _, err := s.StartLifecycle(ctx, start); err != nil {
		t.Fatal(err)
	}
	commit := storedResultCommit(start, "stored-result-commit-1", "stored-result-rev-1", 1, start.InitialRevision.RevisionID)
	if _, err := s.CommitLifecycleArtifactHead(ctx, commit); err != nil {
		t.Fatal(err)
	}
	// A work settlement carries no revision.
	settleInitialLifecycleWork(t, s, start, "stored-result-settle-work")
	forms := s.TelemetryStats().LifecycleCommandForms
	if forms["slim"] != 2 || forms["no_revision"] < 1 {
		t.Fatalf("lifecycle command forms = %v, want slim=2 (start, commit) and no_revision>=1", forms)
	}
	// The byte check refuses a revision whose object body is not its own JSON.
	rev := commit.Revision
	rev.RevisionID = "stored-result-mismatch"
	obj, err := lifecycleObject(ogKindTexRev, start.OwnerID, start.ComputerID, rev.RevisionID, rev, nil, rev.CreatedAt, rev.CreatedAt)
	if err != nil {
		t.Fatal(err)
	}
	changed := rev
	changed.Content = "different"
	if got := lifecycleBatchRevisionForm(nil, start.OwnerID, start.ComputerID, &changed); got != lifecycleFormEmbeddedHead {
		t.Fatalf("revision absent from the batch = %q", got)
	}
	if got := lifecycleBatchRevisionForm([]objectgraph.Object{obj}, start.OwnerID, start.ComputerID, &changed); got != lifecycleFormEmbeddedMismatch {
		t.Fatalf("revision whose body differs = %q, want embedded_mismatch", got)
	}
	if got := lifecycleBatchRevisionForm([]objectgraph.Object{obj}, start.OwnerID, start.ComputerID, &rev); got != lifecycleFormSlim {
		t.Fatalf("revision whose body is its own = %q, want slim", got)
	}
}

func TestTextureTurnCommandNamesItsRevision(t *testing.T) {
	s, start, req, _ := applyTextureRevisionTurn(t)
	stored := storedResultKeys(t, storedResultCommandObject(t, s, start, req.CommandID))
	if _, embedded := stored["revision"]; embedded {
		t.Fatal("texture turn command embeds its revision")
	}
	if _, named := stored["revision_id"]; !named {
		t.Fatal("texture turn command does not name its revision")
	}
	cmd := storedResultCommandObject(t, s, start, req.CommandID)
	if bytes.Contains(cmd.Body, []byte(storedResultMarker)) {
		t.Fatal("texture turn command body still contains the revision body")
	}
}
