package store

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// docs/design/research-more-goes-to-research-2026-10-21.md: a reader's ask
// is a Cast to the story's research desk, admitted in one transaction.
// Failure modes (sections 4 and 8): research not woken or its desk missing
// (1); Texture woken by the ask (2); the round not the story's own
// conversation (3); a replay writes again (4); another or a non-story
// document targeted (5); an open round joined (9); a replay after the head
// moved refused (10); the expansion instruction lost once Texture read
// past it (11); a crash-closed request reads as accepted (12).

func storyResearchFixture(t *testing.T, s *Store, open string) (docID string, head string) {
	t.Helper()
	receipt := mustCommitInbox(t, s, "cell-records:"+open, nil, openingCast(open+":cast:c1"))
	docID = strings.TrimPrefix(receipt.Records[0].Inbox, "texture:")
	doc, err := s.GetLifecycleDocument(context.Background(), storyOwner, storyComputer, docID)
	if err != nil {
		t.Fatalf("story document: %v", err)
	}
	return docID, doc.CurrentRevisionID
}

func askStoryResearch(s *Store, docID, clientID, head string) (StoryResearchReceipt, error) {
	return s.CommitStoryResearchRequest(context.Background(), StoryResearchRequest{
		OwnerID: storyOwner, ComputerID: storyComputer, DocumentID: docID,
		ClientRequestID: clientID, ExpectedHeadRevisionID: head,
	})
}

func TestStoryResearchRequestCastsTheStorysResearchDeskOnce(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	docID, head := storyResearchFixture(t, s, "rr-1")
	texture, research := "texture:"+docID, "research:"+docID
	textureHead, _ := s.InboxHead(ctx, storyOwner, storyComputer, texture)
	wakesBefore := len(storyWakes(t, s))

	receipt, err := askStoryResearch(s, docID, "expand:pub-1:rev-1", head)
	if err != nil {
		t.Fatalf("ask: %v", err)
	}
	if receipt.Replay || receipt.Round != 1 || receipt.WakeID == "" || receipt.RecordID != StoryResearchRecordID("expand:pub-1:rev-1") {
		t.Fatalf("receipt = %+v; want round 1, a wake, the derived record id", receipt)
	}
	rec, err := s.GetCommitmentRecord(ctx, storyOwner, storyComputer, receipt.RecordID)
	if err != nil || rec == nil {
		t.Fatalf("record: %v", err)
	}
	// 3: the story's own conversation, so research's Report closes it.
	if rec.Inbox != research || rec.SenderInbox != texture || !rec.IsCast() {
		t.Fatalf("record routes %s -> %s cast=%v; want %s -> %s cast", rec.SenderInbox, rec.Inbox, rec.IsCast(), texture, research)
	}
	// The owner is the speaker; the ledger never claims Texture cast it.
	if rec.EmitterKind != types.CommitmentEmitterOwner || rec.Provenance.AgentID != "owner:"+storyOwner {
		t.Fatalf("emitter %q agent %q; want the owner", rec.EmitterKind, rec.Provenance.AgentID)
	}
	// Research is given the story it expands, from the head the ask checked.
	if !strings.Contains(rec.Directive.Body, head) || !strings.Contains(rec.Directive.Body, "expand:pub-1:rev-1") {
		t.Fatalf("cast body does not name the head and the request: %s", rec.Directive.Body)
	}
	// 1: research's desk exists so its wake starts a run.
	if _, err := s.GetAgentByScope(ctx, storyOwner, storyComputer, research); err != nil {
		t.Fatalf("research desk: %v", err)
	}
	// 2: nothing addressed to Texture, and exactly one new wake (research's).
	if after, _ := s.InboxHead(ctx, storyOwner, storyComputer, texture); after != textureHead {
		t.Fatalf("texture inbox head %d -> %d; the ask must not write to Texture", textureHead, after)
	}
	wakes := storyWakes(t, s)
	if len(wakes) != wakesBefore+1 || wakes[len(wakes)-1].TargetAgentID != research {
		t.Fatalf("wakes %d -> %d (last %+v); want one wake for %s", wakesBefore, len(wakes), wakes[len(wakes)-1], research)
	}

	// 4 and 10: the same request replays, even after the head moved.
	replay, err := askStoryResearch(s, docID, "expand:pub-1:rev-1", "some-later-head")
	if err != nil || !replay.Replay || replay.RecordID != receipt.RecordID {
		t.Fatalf("replay = %+v, %v; want the stored receipt", replay, err)
	}
	if head, _ := s.InboxHead(ctx, storyOwner, storyComputer, research); head != 1 {
		t.Fatalf("research inbox head %d after a replay, want 1", head)
	}
}

func TestStoryResearchRequestRefusals(t *testing.T) {
	s := openTestStore(t)
	docID, head := storyResearchFixture(t, s, "rr-2")
	texture, research := "texture:"+docID, "research:"+docID

	// The head the reader saw moved.
	if _, err := askStoryResearch(s, docID, "expand:a", "not-the-head"); !errors.Is(err, ErrStoryResearchChanged) {
		t.Fatalf("stale head = %v, want ErrStoryResearchChanged", err)
	}
	// 5: no such story.
	if _, err := askStoryResearch(s, "no-such-doc", "expand:b", head); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown document = %v, want ErrNotFound", err)
	}
	// 9: Texture's own round with research is open; the ask is busy, not joined.
	mustCommitInbox(t, s, "cell-records:texture-cast", nil, inboxCast("tc-1", texture, research, types.CommitmentCast{}))
	if _, err := askStoryResearch(s, docID, "expand:c", head); !errors.Is(err, ErrStoryResearchBusy) {
		t.Fatalf("open round = %v, want ErrStoryResearchBusy", err)
	}
}

func TestStoryResearchRequestStateAndPending(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	docID, head := storyResearchFixture(t, s, "rr-3")
	texture, research := "texture:"+docID, "research:"+docID
	if _, err := askStoryResearch(s, docID, "expand:d", head); err != nil {
		t.Fatal(err)
	}
	state, err := s.StoryResearchRequestState(ctx, storyOwner, storyComputer, docID, "expand:d")
	if err != nil || !state.Open || state.Reports != 0 || state.Interrupted {
		t.Fatalf("admitted state = %+v, %v", state, err)
	}
	// 11: Texture is told while the round is open.
	if id, pending, err := s.PendingStoryResearchRequest(ctx, storyOwner, storyComputer, docID); err != nil || !pending || id != "expand:d" {
		t.Fatalf("pending while open = %q %v %v", id, pending, err)
	}
	mustCommitInbox(t, s, "cell-records:interim", nil, inboxReport("rep-1", research, texture, false, types.CommitmentReport{}))
	final := inboxReport("rep-2", research, texture, true, types.CommitmentReport{StopReason: "completed"})
	receipt := mustCommitInbox(t, s, "cell-records:final", nil, final)
	state, _ = s.StoryResearchRequestState(ctx, storyOwner, storyComputer, docID, "expand:d")
	if state.Open || state.Reports != 2 || state.StopReason != "completed" || state.Interrupted {
		t.Fatalf("closed state = %+v", state)
	}
	// Still pending until Texture has read the final Report.
	if _, pending, _ := s.PendingStoryResearchRequest(ctx, storyOwner, storyComputer, docID); !pending {
		t.Fatal("not pending before Texture read the final Report")
	}
	cur, _ := s.ReaderCursor(ctx, storyOwner, storyComputer, texture, texture)
	mustCommitInbox(t, s, "turn-records-read:t", &types.InboxCursorAdvance{ReaderAgentID: texture, Inbox: texture, ExpectedSeq: cur, Seq: receipt.Records[0].InboxSeq})
	if _, pending, _ := s.PendingStoryResearchRequest(ctx, storyOwner, storyComputer, docID); pending {
		t.Fatal("still pending after Texture read the final Report")
	}
}

// 12: a crash closes the round with no research Report; the request reads
// interrupted, not accepted.
func TestStoryResearchRequestInterruptedByRestart(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	docID, head := storyResearchFixture(t, s, "rr-4")
	texture, research := "texture:"+docID, "research:"+docID
	if _, err := askStoryResearch(s, docID, "expand:e", head); err != nil {
		t.Fatal(err)
	}
	closure := types.NewRoundClosureRecord(texture, research, 1, types.ClosureReasonInterruptedByRestart, "")
	mustCommitInbox(t, s, "boot-closure:test", nil, closure)
	state, err := s.StoryResearchRequestState(ctx, storyOwner, storyComputer, docID, "expand:e")
	if err != nil || state.Open || !state.Interrupted || state.Reports != 0 {
		t.Fatalf("crash-closed state = %+v, %v; want interrupted", state, err)
	}
}
