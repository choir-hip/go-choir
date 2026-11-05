package store

import (
	"context"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// X19 (modular architecture plan, section 8.4) is counted, not fixed.
// Management's inbox command opens a story: planInboxCommand commits
// management's records with Texture's story document, revision,
// story_library_head and (when verdicts ride along) support_verdict in one
// batch. A built-in kind only observes until MA3.7 turns management's story
// opening into a Cast to Texture, so the write goes through; but the counters
// must read it as what it is, desk:management writing Texture's kinds. This
// test is MA3.7's target: when the crossing becomes a refusal, it turns from
// a counted crossing into an ErrWrongWriter.
func TestManagementStoryOpeningCountsAsManagementWritingTexturesKinds(t *testing.T) {
	s := openTestStore(t)
	defer objectgraph.SetWriterGateMode(objectgraph.WriterGateObserve)()
	objectgraph.ResetWriterGateCounts()
	ctx := objectgraph.WithWriter(context.Background(), objectgraph.WriterManagement)

	seen := int64(0)
	req := types.CommitInboxRecordsRequest{
		OwnerID: inboxTestOwner, ComputerID: inboxTestComputer, CommandID: "x19-open",
		Records:          []types.CommitmentRecord{openingCast("x19:cast:1")},
		StoryLibraryHead: &seen,
	}
	var err error
	if req.CommandDigest, err = ComputeCommitInboxRecordsDigest(req); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CommitInboxRecords(ctx, req); err != nil {
		t.Fatalf("management's story opening (observed, not refused): %v", err)
	}
	counts := objectgraph.WriterGateCounts()
	for _, kind := range []string{"choir.texture_document", "choir.texture_revision", "choir.story_library_head"} {
		key := kind + "|" + string(objectgraph.WriterManagement) + "|wrong_writer"
		if counts[key] == 0 {
			t.Errorf("counter %q = 0; want management's crossing into %s counted (counts: %v)", key, kind, counts)
		}
	}
	for key := range counts {
		if !strings.HasSuffix(key, "|"+string(objectgraph.WriterManagement)+"|wrong_writer") {
			t.Errorf("counter %q is not a management crossing: a declared writer must name who crossed", key)
		}
	}
}

// The same crossing for the support verdict: the verdicts ride in the batch of
// the command that carries them, and the command's declared writer is who the
// counters name.
func TestManagementCommandCarryingVerdictsCountsSupportVerdict(t *testing.T) {
	s, start, caller, _ := setupLifecycleTextureTargetFixture(t)
	system := types.SupportSystemAgentID
	a := supportPair("x19-rev-1", "p-1", "The merger may proceed.", "merger may proceed")
	supportRevisionTurn(t, s, start, caller, "x19-rev-1", "The merger may proceed.", a)

	defer objectgraph.SetWriterGateMode(objectgraph.WriterGateObserve)()
	objectgraph.ResetWriterGateCounts()
	req := types.CommitInboxRecordsRequest{OwnerID: start.OwnerID, ComputerID: start.ComputerID, CommandID: "x19-verdict",
		Judge: system, Verdicts: []types.SupportVerdict{{Key: a.Key, Verdict: types.SupportSupported, Judge: "jev:typesafe/jev-1.13", PSupportedBP: 9100}}}
	req.CommandDigest, _ = ComputeCommitInboxRecordsDigest(req)
	ctx := objectgraph.WithWriter(context.Background(), objectgraph.WriterManagement)
	if _, err := s.CommitInboxRecords(ctx, req); err != nil {
		t.Fatalf("verdicts under management (observed, not refused): %v", err)
	}
	key := "choir.support_verdict|" + string(objectgraph.WriterManagement) + "|wrong_writer"
	if got := objectgraph.WriterGateCounts()[key]; got == 0 {
		t.Fatalf("counter %q = 0 (counts: %v)", key, objectgraph.WriterGateCounts())
	}
}
