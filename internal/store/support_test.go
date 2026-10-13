package store

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// Support judgement in the store (Cut 2 2a (v)). Ways it can fail, written
// first:
//   - the pairs list is a system record and wakes no one, so management
//     never judges (D5's rule is for closures);
//   - the list repeats pairs a verdict or an earlier list already covers;
//   - a verdict lands for a pair no Texture commit listed (a model-made
//     pair), or a second verdict for a judged pair sends Texture a second
//     failure;
//   - a passing verdict sends Texture a record or touches the document;
//   - a failing verdict does not reach texture:<doc>, or reaches it without
//     the pair it fails.

const supportMatcher = "qm-test"

func supportPair(revisionID, blockID, text, quote string) types.CitationPair {
	blockHash := "sha256:block-" + blockID + "-" + text
	cited := types.Ref{Doc: "cap-1", Version: "v1", CID: "cid-" + quote, Hash: "sha256:cited-" + quote, Quote: quote}
	return types.CitationPair{
		Key: types.SupportPairKey(blockHash, cited.Hash, quote, supportMatcher), DocID: "document-texture-target", RevisionID: revisionID,
		BlockID: blockID, BlockHash: blockHash, Text: text, Cited: cited, Matcher: supportMatcher,
	}
}

func supportRevisionTurn(t *testing.T, s *Store, start types.StartLifecycleRequest, caller types.RunRecord, revisionID, content string, pairs ...types.CitationPair) types.LifecycleResult {
	t.Helper()
	req := textureTurnBaseRequest(t, s, start, caller, types.TextureTurnRevision)
	req.CommandID = "support-turn-" + revisionID
	rev := start.InitialRevision
	rev.RevisionID, rev.AuthorKind, rev.AuthorLabel, rev.CreatedAt = revisionID, types.AuthorAppAgent, "appagent", time.Time{}
	rev.ParentRevisionID = req.ExpectedHeadRevisionID
	rev.BodyDoc, _ = json.Marshal(userAuthoredTextStructuredTextureDoc(rev.DocID, rev.RevisionID, content))
	rev.Content = content
	req.Revision = rev
	req.Citations = pairs
	setTextureTurnDigest(t, &req, TextureSourceGraphWriteSet{})
	result, err := s.ApplyTextureTurn(context.Background(), req)
	if err != nil {
		t.Fatalf("revision turn %s: %v", revisionID, err)
	}
	return result
}

func commitVerdicts(t *testing.T, s *Store, start types.StartLifecycleRequest, commandID string, verdicts ...types.SupportVerdict) (types.InboxCommitReceipt, error) {
	t.Helper()
	req := types.CommitInboxRecordsRequest{OwnerID: start.OwnerID, ComputerID: start.ComputerID, CommandID: commandID,
		Judge: "management:" + start.OwnerID, Verdicts: verdicts}
	req.CommandDigest, _ = ComputeCommitInboxRecordsDigest(req)
	return s.CommitInboxRecords(context.Background(), req)
}

func TestSupportPairsListWakeManagementAndVerdictsMemoize(t *testing.T) {
	s, start, caller, _ := setupLifecycleTextureTargetFixture(t)
	ctx := context.Background()
	owner, computer := start.OwnerID, start.ComputerID
	management, texture := "management:"+owner, start.Agent.AgentID

	a := supportPair("support-rev-1", "p-1", "The merger may proceed.", "merger may proceed")
	b := supportPair("support-rev-1", "p-1", "The merger may proceed.", "appeal was filed")
	result := supportRevisionTurn(t, s, start, caller, "support-rev-1", "The merger may proceed.", a, b, a)
	if got := result.TextureTurn.InboxRecords; len(got) != 1 || got[0].Inbox != management || got[0].WakeID == "" {
		t.Fatalf("turn's records = %+v; want one list to %s that wakes it", got, management)
	}
	listed, err := s.ListInboxRecords(ctx, owner, computer, management, 0, 10)
	if err != nil || len(listed) != 1 || listed[0].Support == nil || len(listed[0].Support.Pairs) != 2 || listed[0].EmitterKind != types.CommitmentEmitterSystem {
		t.Fatalf("management inbox = %+v, %v; want one system list of two pairs", listed, err)
	}

	// The same pairs on the next revision are listed already.
	a2, b2 := a, b
	a2.RevisionID, b2.RevisionID = "support-rev-2", "support-rev-2"
	supportRevisionTurn(t, s, start, caller, "support-rev-2", "The merger may proceed.", a2, b2)
	if head, _ := s.InboxHead(ctx, owner, computer, management); head != 1 {
		t.Fatalf("management inbox head = %d; listed pairs were listed again", head)
	}

	// A verdict for a pair no commit listed is refused.
	forged := supportPair("support-rev-1", "p-9", "Made up.", "made up")
	if _, err := commitVerdicts(t, s, start, "verdict-forged", types.SupportVerdict{Key: forged.Key, Verdict: types.SupportSupported, Judge: "management"}); !errors.Is(err, ErrInboxRefused) {
		t.Fatalf("verdict for an unlisted pair = %v; want ErrInboxRefused", err)
	}

	docBefore, _ := s.GetLifecycleDocument(ctx, owner, computer, start.InitialDocument.DocID)
	receipt, err := commitVerdicts(t, s, start, "verdict-1",
		types.SupportVerdict{Key: a.Key, Verdict: types.SupportUnsupported, Why: "the source says the merger was blocked", Judge: "management"},
		types.SupportVerdict{Key: b.Key, Verdict: types.SupportSupported, Judge: "jev:typesafe/jev-1.13", PSupportedBP: 9100})
	if err != nil {
		t.Fatalf("verdicts: %v", err)
	}
	if len(receipt.Verdicts) != 2 {
		t.Fatalf("receipt verdicts = %+v; want both recorded", receipt.Verdicts)
	}
	toTexture, err := s.ListInboxRecords(ctx, owner, computer, texture, 0, 10)
	if err != nil || len(toTexture) != 1 {
		t.Fatalf("texture inbox = %+v, %v; want one failure record", toTexture, err)
	}
	failure := toTexture[0]
	if failure.SenderInbox != management || failure.EmitterKind != types.CommitmentEmitterAgent || failure.Support == nil ||
		len(failure.Support.Failures) != 1 || failure.Support.Failures[0].Key != a.Key || failure.Support.Failures[0].Pair == nil ||
		failure.Support.Failures[0].Pair.Cited.Quote != "merger may proceed" || failure.Support.Failures[0].Why == "" {
		t.Fatalf("failure record = %+v; want management's failing verdict on pair a with its pair", failure)
	}
	for _, p := range []types.CitationPair{a, b} {
		v, found, err := s.SupportVerdict(ctx, owner, computer, p.Key)
		if err != nil || !found || v.Pair == nil || v.Pair.BlockID != p.BlockID {
			t.Fatalf("verdict memo %s = %+v, %v, %v", p.Key, v, found, err)
		}
	}

	// A second verdict on a judged pair is a memo hit: nothing new reaches
	// Texture.
	again, err := commitVerdicts(t, s, start, "verdict-2", types.SupportVerdict{Key: a.Key, Verdict: types.SupportUnsupported, Why: "still blocked", Judge: "management"})
	if err != nil || len(again.Verdicts) != 0 {
		t.Fatalf("second verdict = %+v, %v; want a memo hit", again, err)
	}
	if head, _ := s.InboxHead(ctx, owner, computer, texture); head != 1 {
		t.Fatalf("texture inbox head = %d; a judged pair failed twice", head)
	}
	if docAfter, _ := s.GetLifecycleDocument(ctx, owner, computer, start.InitialDocument.DocID); docAfter.CurrentRevisionID != docBefore.CurrentRevisionID {
		t.Fatal("a verdict moved the document head")
	}
}
