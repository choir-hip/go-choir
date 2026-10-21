package store

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// A whole Texture revision turn commits under the enforced writer gate
// (SDC ARCH-04, failure mode F1): the turn declares Texture as its writer, and
// its mixed batch (the revision with the runtime's inbox, event and wake
// bookkeeping) is admitted by the registry's classification. A wrong
// classification of any kind in that batch would refuse the turn here.
func TestTextureTurnCommitsUnderTheEnforcedWriterGate(t *testing.T) {
	s, start, caller, _ := setupLifecycleTextureTargetFixture(t)
	ctx := context.Background()
	req := textureTurnBaseRequest(t, s, start, caller, types.TextureTurnRevision)
	rev := start.InitialRevision
	rev.RevisionID = "writer-gate-revision-1"
	rev.AuthorKind = types.AuthorAppAgent
	rev.AuthorLabel = "appagent"
	rev.ParentRevisionID = start.InitialRevision.RevisionID
	rev.CreatedAt = time.Time{}
	doc := userAuthoredTextStructuredTextureDoc(rev.DocID, rev.RevisionID, "writer gate: a revision under the enforced gate")
	bodyDocJSON, _ := json.Marshal(doc)
	rev.BodyDoc = bodyDocJSON
	rev.Content = "writer gate: a revision under the enforced gate"
	req.Revision = rev
	setTextureTurnDigest(t, &req, TextureSourceGraphWriteSet{})

	// The fixture's own setup wrote documents without declaring a writer;
	// the gate is enforced only for the turn.
	defer objectgraph.SetWriterGateMode(objectgraph.WriterGateEnforce)()
	objectgraph.ResetWriterGateCounts()

	result, err := s.ApplyTextureTurn(ctx, req)
	if err != nil {
		t.Fatalf("a Texture turn was refused by the enforced writer gate: %v", err)
	}
	if result.Revision == nil || result.Revision.AuthorKind != types.AuthorAppAgent {
		t.Fatalf("revision = %+v", result.Revision)
	}
	if counts := objectgraph.WriterGateCounts(); len(counts) != 0 {
		t.Fatalf("the Texture turn's batch recorded writer violations: %v", counts)
	}
}
