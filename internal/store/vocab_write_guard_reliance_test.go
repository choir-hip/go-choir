package store

// The reliance link (relies_on, a directed edge from a story to an earlier
// story of the paper) is written by corpusd, whose store has no vocabulary
// guard, but the Autopaper object graph is adopted onto computers' stores,
// where the guard reads every leaf named role, profile, agent_profile,
// agent_role, authority_profile, requested_by_profile, actor_profile or
// target_profile as a desk name. This commits the edge's metadata with the
// cutover on, so a key of that name in it would be caught here and not on a
// computer. The control refuses an unknown role, so the guard is live.

import (
	"context"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

func TestReliesOnEdgePassesTheVocabularyWriteGuardAfterCutover(t *testing.T) {
	s := openTestStore(t)
	s.vocabCutover.Store(true)
	edge := objectgraph.Edge{
		EdgeID: "e-reliance", FromID: "obj:choir.publication:o:later", ToID: "obj:choir.publication:o:earlier",
		Kind: objectgraph.EdgeReliesOn, Metadata: []byte(objectgraph.ReliesOnMetadata),
	}
	if err := s.vocabWriteGuard(context.Background(), nil, []objectgraph.Edge{edge}); err != nil {
		t.Fatalf("the write guard refused the relies_on edge after the cutover: %v", err)
	}
	if err := objectgraph.CheckWrite(nil, []objectgraph.Edge{edge}); err != nil {
		t.Fatalf("the registry refused the relies_on edge: %v", err)
	}
	bad := edge
	bad.Metadata = []byte(`{"relation":"relies_on","role":"boss"}`)
	if err := s.vocabWriteGuard(context.Background(), nil, []objectgraph.Edge{bad}); err == nil {
		t.Fatal("the write guard accepted an unknown role: it is not active in this test")
	}
}
