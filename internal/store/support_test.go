package store

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// Support judgement in the store
// (docs/archive/reports/support-judgement-to-jev-design-2026-10-14.md). A Texture
// commit lists its new pairs by their check memos; the runtime's Jev job
// records verdicts as a system act. Ways it can fail, written first:
//   - the commit still sends management a pairs record, so management's
//     serial turn is back in the path;
//   - an unchanged pair is listed again (its check memo rewritten);
//   - a verdict lands for a pair no Texture commit listed (a model-made
//     pair), or from a desk instead of the system judge;
//   - a second verdict for a judged pair sends Texture a second failure;
//   - a passing verdict sends Texture a record or touches the document;
//   - a failing verdict does not reach texture:<doc>, reaches it without
//     the pair it fails, or does not wake the Texture (system records wake
//     no one, D5).

const supportMatcher = "qm-test"

func supportPair(revisionID, blockID, text, quote string) types.CitationPair {
	blockHash := "sha256:block-" + blockID + "-" + text
	cited := types.Ref{Doc: "cap-1", Version: "v1", CID: "cid-" + quote, Hash: "sha256:cited-" + quote, Quote: quote}
	return types.CitationPair{
		Key: types.SupportPairKey(blockHash, cited.Hash, quote, supportMatcher), DocID: "document-texture-target", RevisionID: revisionID,
		BlockID: blockID, BlockHash: blockHash, Text: text, Cited: cited, Matcher: supportMatcher,
	}
}

func supportRevisionTurn(t *testing.T, s *Store, start types.StartLifecycleRequest, caller types.RunRecord, revisionID, content string, pairs ...types.CitationPair) (types.LifecycleResult, string) {
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
	return result, req.CommandID
}

func commitVerdicts(t *testing.T, s *Store, start types.StartLifecycleRequest, judge, commandID string, verdicts ...types.SupportVerdict) (types.InboxCommitReceipt, error) {
	t.Helper()
	req := types.CommitInboxRecordsRequest{OwnerID: start.OwnerID, ComputerID: start.ComputerID, CommandID: commandID,
		Judge: judge, Verdicts: verdicts}
	req.CommandDigest, _ = ComputeCommitInboxRecordsDigest(req)
	return s.CommitInboxRecords(context.Background(), req)
}

func TestSupportPairsAreListedByCheckAndVerdictsAreASystemAct(t *testing.T) {
	s, start, caller, _ := setupLifecycleTextureTargetFixture(t)
	ctx := context.Background()
	owner, computer := start.OwnerID, start.ComputerID
	management, texture := "management:"+owner, start.Agent.AgentID
	system := types.SupportSystemAgentID

	a := supportPair("support-rev-1", "p-1", "The merger may proceed.", "merger may proceed")
	b := supportPair("support-rev-1", "p-1", "The merger may proceed.", "appeal was filed")
	result, command := supportRevisionTurn(t, s, start, caller, "support-rev-1", "The merger may proceed.", a, b, a)
	if got := result.TextureTurn.InboxRecords; len(got) != 0 {
		t.Fatalf("turn's records = %+v; the commit must send no pairs record", got)
	}
	if head, _ := s.InboxHead(ctx, owner, computer, management); head != 0 {
		t.Fatalf("management inbox head = %d; management must not receive support pairs", head)
	}
	for _, p := range []types.CitationPair{a, b} {
		check, listed, err := s.CitationCheck(ctx, owner, computer, p.Key)
		if err != nil || !listed || check.ListedBy != command || check.Pair.Key != p.Key {
			t.Fatalf("check memo %s = %+v, %v, %v; want it listed by %s", p.Key, check, listed, err, command)
		}
	}

	// The same pairs on the next revision are listed already: their check
	// memos keep the commit that first listed them.
	a2, b2 := a, b
	a2.RevisionID, b2.RevisionID = "support-rev-2", "support-rev-2"
	supportRevisionTurn(t, s, start, caller, "support-rev-2", "The merger may proceed.", a2, b2)
	if check, _, _ := s.CitationCheck(ctx, owner, computer, a.Key); check.ListedBy != command {
		t.Fatalf("check memo listed by %s after an unchanged commit; want %s", check.ListedBy, command)
	}

	// A verdict for a pair no commit listed is refused, and so is a desk's
	// verdict: support judgement is the system's Jev job.
	forged := supportPair("support-rev-1", "p-9", "Made up.", "made up")
	if _, err := commitVerdicts(t, s, start, system, "verdict-forged", types.SupportVerdict{Key: forged.Key, Verdict: types.SupportSupported, Judge: "jev:typesafe/jev-1.13"}); !errors.Is(err, ErrInboxRefused) {
		t.Fatalf("verdict for an unlisted pair = %v; want ErrInboxRefused", err)
	}
	if _, err := commitVerdicts(t, s, start, management, "verdict-desk", types.SupportVerdict{Key: a.Key, Verdict: types.SupportSupported, Judge: "management"}); !errors.Is(err, ErrInboxRefused) {
		t.Fatalf("management's verdict = %v; want ErrInboxRefused", err)
	}

	docBefore, _ := s.GetLifecycleDocument(ctx, owner, computer, start.InitialDocument.DocID)
	receipt, err := commitVerdicts(t, s, start, system, "verdict-1",
		types.SupportVerdict{Key: a.Key, Verdict: types.SupportUnsupported, Why: "the source says the merger was blocked", Judge: "jev:typesafe/jev-1.13", PSupportedBP: 800},
		types.SupportVerdict{Key: b.Key, Verdict: types.SupportSupported, Judge: "jev:typesafe/jev-1.13", PSupportedBP: 9100})
	if err != nil {
		t.Fatalf("verdicts: %v", err)
	}
	if len(receipt.Verdicts) != 2 {
		t.Fatalf("receipt verdicts = %+v; want both recorded", receipt.Verdicts)
	}
	if len(receipt.Records) != 1 || receipt.Records[0].Inbox != texture || receipt.Records[0].WakeID == "" {
		t.Fatalf("receipt records = %+v; want one failure to %s that wakes it", receipt.Records, texture)
	}
	toTexture, err := s.ListInboxRecords(ctx, owner, computer, texture, 0, 10)
	if err != nil || len(toTexture) != 1 {
		t.Fatalf("texture inbox = %+v, %v; want one failure record", toTexture, err)
	}
	failure := toTexture[0]
	if failure.SenderInbox != system || failure.EmitterKind != types.CommitmentEmitterSystem || failure.Provenance.AgentID != system || failure.Support == nil ||
		len(failure.Support.Failures) != 1 || failure.Support.Failures[0].Key != a.Key || failure.Support.Failures[0].Pair == nil ||
		failure.Support.Failures[0].Pair.Cited.Quote != "merger may proceed" || failure.Support.Failures[0].Why == "" {
		t.Fatalf("failure record = %+v; want the system's failing verdict on pair a with its pair", failure)
	}
	for _, p := range []types.CitationPair{a, b} {
		v, found, err := s.SupportVerdict(ctx, owner, computer, p.Key)
		if err != nil || !found || v.Pair == nil || v.Pair.BlockID != p.BlockID {
			t.Fatalf("verdict memo %s = %+v, %v, %v", p.Key, v, found, err)
		}
	}

	// A second verdict on a judged pair is a memo hit: nothing new reaches
	// Texture.
	again, err := commitVerdicts(t, s, start, system, "verdict-2", types.SupportVerdict{Key: a.Key, Verdict: types.SupportUnsupported, Why: "still blocked", Judge: "jev:typesafe/jev-1.13"})
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
