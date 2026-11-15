package store

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
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

// The writer's byte-check gate, end to end. A real command that only reads the
// head (a terminal settlement) is embedded and counted embedded_head; a batch
// whose revision object body is not the revision's own JSON stays embedded and
// counts embedded_mismatch. A writer that slims every command with a revision
// fails both.
func TestLifecycleWriterGateKeepsHeadAndMismatchedRevisionsEmbedded(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	start := lifecycleStartFixture()
	if _, err := s.StartLifecycle(ctx, start); err != nil {
		t.Fatal(err)
	}
	ready := settleInitialLifecycleWork(t, s, start, "stored-result-gate-work")
	settle := types.SettleLifecycleTrajectoryRequest{
		OwnerID: start.OwnerID, ComputerID: start.ComputerID,
		CommandID: "stored-result-gate-settle", TrajectoryID: start.TrajectoryID,
		ExpectedLifecycleVersion: ready.Trajectory.LifecycleVersion,
		ExpectedHeadRevisionID:   start.InitialRevision.RevisionID,
	}
	settle.CommandDigest, _ = ComputeSettleLifecycleTrajectoryDigest(settle)
	if _, err := s.SettleLifecycleTrajectory(ctx, settle); err != nil {
		t.Fatalf("settle trajectory: %v", err)
	}
	stored := storedResultKeys(t, storedResultCommandObject(t, s, start, settle.CommandID))
	if _, embedded := stored["revision"]; !embedded {
		t.Fatal("a head-only command was slimmed: its revision was not written by its own batch")
	}
	if _, named := stored["revision_id"]; named {
		t.Fatal("a head-only command names a revision id")
	}
	if forms := s.TelemetryStats().LifecycleCommandForms; forms["embedded_head"] != 1 || forms["slim"] != 1 {
		t.Fatalf("forms = %v, want embedded_head=1 (settle) and slim=1 (start)", forms)
	}

	// A batch whose revision body differs from the revision the result carries.
	rev := start.InitialRevision
	rev.RevisionID = "stored-result-gate-mismatch"
	rev.OwnerID, rev.ComputerID, rev.DocID = start.OwnerID, start.ComputerID, start.InitialDocument.DocID
	other := rev
	other.Content = "a body that is not the revision's own JSON"
	revObj, err := lifecycleObject(ogKindTexRev, start.OwnerID, start.ComputerID, rev.RevisionID, other, lifecycleMetadata("revision_id", rev.RevisionID, start.ComputerID, start.TrajectoryID, 90), rev.CreatedAt, rev.CreatedAt)
	if err != nil {
		t.Fatal(err)
	}
	receipt, receiptObj, err := s.lifecycleTransitionReceipt(rev.CreatedAt, start.OwnerID, start.ComputerID, start.TrajectoryID, "stored-result-gate-mismatch-cmd", "digest", types.LifecycleCommitArtifactHead, 90, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.commitLifecycleTransition(ctx, start.OwnerID, start.ComputerID, "stored-result-gate-mismatch-cmd", "digest", nil,
		[]objectgraph.Object{revObj, receiptObj}, types.LifecycleResult{Receipt: receipt, Revision: &rev}); err != nil {
		t.Fatalf("commit mismatching batch: %v", err)
	}
	stored = storedResultKeys(t, storedResultCommandObject(t, s, start, "stored-result-gate-mismatch-cmd"))
	if _, embedded := stored["revision"]; !embedded {
		t.Fatal("a command whose batch body differs from its revision was slimmed")
	}
	if forms := s.TelemetryStats().LifecycleCommandForms; forms["embedded_mismatch"] != 1 {
		t.Fatalf("forms = %v, want embedded_mismatch=1", forms)
	}
}

// A command that does not commit is not counted: a lost condition returns
// ErrConcurrentStateChange and leaves the counter where it was.
func TestLifecycleCommandFormsCountOnTheSuccessPathOnly(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	start := lifecycleStartFixture()
	if _, err := s.StartLifecycle(ctx, start); err != nil {
		t.Fatal(err)
	}
	before := s.TelemetryStats().LifecycleCommandForms["slim"]
	rev := start.InitialRevision
	rev.RevisionID = "stored-result-count-rev"
	rev.OwnerID, rev.ComputerID, rev.DocID = start.OwnerID, start.ComputerID, start.InitialDocument.DocID
	revObj, err := lifecycleObject(ogKindTexRev, start.OwnerID, start.ComputerID, rev.RevisionID, rev, lifecycleMetadata("revision_id", rev.RevisionID, start.ComputerID, start.TrajectoryID, 91), rev.CreatedAt, rev.CreatedAt)
	if err != nil {
		t.Fatal(err)
	}
	build := func(commandID string) (types.LifecycleResult, objectgraph.Object) {
		receipt, receiptObj, err := s.lifecycleTransitionReceipt(rev.CreatedAt, start.OwnerID, start.ComputerID, start.TrajectoryID, commandID, "digest", types.LifecycleCommitArtifactHead, 91, nil)
		if err != nil {
			t.Fatal(err)
		}
		return types.LifecycleResult{Receipt: receipt, Revision: &rev}, receiptObj
	}
	result, receiptObj := build("stored-result-count-lost")
	lost := []objectgraph.ObjectCondition{{CanonicalID: revObj.CanonicalID, Exists: true}} // the revision does not exist
	if _, err := s.commitLifecycleTransition(ctx, start.OwnerID, start.ComputerID, "stored-result-count-lost", "digest", lost,
		[]objectgraph.Object{revObj, receiptObj}, result); !errors.Is(err, ErrConcurrentStateChange) {
		t.Fatalf("lost condition error = %v, want ErrConcurrentStateChange", err)
	}
	if got := s.TelemetryStats().LifecycleCommandForms["slim"]; got != before {
		t.Fatalf("a command that did not commit was counted: slim %d -> %d", before, got)
	}
	result, receiptObj = build("stored-result-count-won")
	if _, err := s.commitLifecycleTransition(ctx, start.OwnerID, start.ComputerID, "stored-result-count-won", "digest", nil,
		[]objectgraph.Object{revObj, receiptObj}, result); err != nil {
		t.Fatal(err)
	}
	if got := s.TelemetryStats().LifecycleCommandForms["slim"]; got != before+1 {
		t.Fatalf("a committed command was not counted once: slim %d -> %d", before, got)
	}
}
