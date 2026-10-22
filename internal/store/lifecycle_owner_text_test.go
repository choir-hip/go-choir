package store

import (
	"context"
	"errors"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// The initial revision of a new lifecycle that the owner authored as plain
// text is the owner's literal input: "[1]", "argv[1]" and "Source:" lines are
// not Texture citations and are not refused. Text an agent authors for the
// same start stays under the citation rules (receipt:
// docs/problems/prompt-bar-refuses-bracketed-index-as-server-error-2026-10-22.md).
func TestStartLifecycleKeepsOwnerLiteralInitialText(t *testing.T) {
	const ownerText = "Run it with sys.argv[1] set.\nSummarize the note.[1]\n\nSource: my notes\n{{source: pasted}}"

	t.Run("owner plain text with bracketed indices is stored exactly", func(t *testing.T) {
		s := openTestStore(t)
		ctx := context.Background()
		req := lifecycleStartFixture()
		req.InitialRevision = types.Revision{
			RevisionID: "revision-lifecycle-v0", AuthorKind: types.AuthorUser, AuthorLabel: req.OwnerID, Content: ownerText,
		}
		req.StartRequestDigest, _ = ComputeStartLifecycleRequestDigest(req)

		result, err := s.StartLifecycle(ctx, req)
		if err != nil {
			t.Fatalf("start lifecycle with owner text: %v", err)
		}
		if result.Revision == nil || result.Revision.Content != ownerText {
			t.Fatalf("initial revision = %+v, want the owner's text exactly", result.Revision)
		}
		replayed, err := s.StartLifecycle(ctx, req)
		if err != nil || !replayed.Replay || replayed.Revision == nil || replayed.Revision.Content != ownerText ||
			replayed.Revision.RevisionHash != result.Revision.RevisionHash {
			t.Fatalf("replay = %+v, %v; want the same revision", replayed, err)
		}
		snapshot, err := s.GetLifecycleSnapshot(ctx, req.OwnerID, req.ComputerID, req.TrajectoryID)
		if err != nil || snapshot.HeadRevision.Content != ownerText {
			t.Fatalf("snapshot head = %+v, %v", snapshot.HeadRevision, err)
		}
	})

	t.Run("agent-authored initial revision with the same text is still refused", func(t *testing.T) {
		s := openTestStore(t)
		ctx := context.Background()
		req := lifecycleStartFixture()
		req.InitialRevision.Content = ""
		req.InitialRevision.BodyDoc = lifecycleStructuredBodyDoc(req.InitialDocument.DocID, req.InitialRevision.RevisionID, "A claim.[1]")
		req.StartRequestDigest, _ = ComputeStartLifecycleRequestDigest(req)

		if _, err := s.StartLifecycle(ctx, req); !errors.Is(err, ErrInvalidTextureRevision) {
			t.Fatalf("agent-authored start with [1] error = %v, want ErrInvalidTextureRevision", err)
		}
		if _, err := s.GetLifecycleTrajectory(ctx, req.OwnerID, req.ComputerID, req.TrajectoryID); !errors.Is(err, ErrNotFound) {
			t.Fatalf("refused start persisted a trajectory: %v", err)
		}
	})
}
