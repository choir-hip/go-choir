package store

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// Slice d (docs/problems/related-stories-no-entity-layer-2026-10-26.md): the
// backfill asks a story's own research desk to record what the story is
// about, a sibling of the reader's request on the same path. Failure modes:
// the request asks for new sources (a reader's request) or carries the
// story's text; its record id collides with a reader's request; it joins an
// open round; a replay writes again.
func TestStoryEntitiesRequestCastsTheRecordObjective(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	docID, head := storyResearchFixture(t, s, "re-1")
	ask := func(clientID string) (StoryResearchReceipt, error) {
		return s.CommitStoryResearchRequest(ctx, StoryResearchRequest{
			OwnerID: storyOwner, ComputerID: storyComputer, DocumentID: docID,
			ClientRequestID: clientID, ExpectedHeadRevisionID: head, Kind: StoryResearchKindEntities,
		})
	}
	receipt, err := ask("related-review-1:" + docID)
	if err != nil {
		t.Fatalf("ask: %v", err)
	}
	if receipt.Replay || receipt.WakeID == "" || receipt.RecordID != StoryEntitiesRecordID("related-review-1:"+docID) || receipt.RecordID == StoryResearchRecordID("related-review-1:"+docID) {
		t.Fatalf("receipt = %+v, want a wake and the entities record id", receipt)
	}
	rec, err := s.GetCommitmentRecord(ctx, storyOwner, storyComputer, receipt.RecordID)
	if err != nil || rec == nil {
		t.Fatalf("record: %v", err)
	}
	if rec.Inbox != "research:"+docID || rec.SenderInbox != "texture:"+docID || !rec.IsCast() || rec.Directive.Objective != storyEntitiesObjective {
		t.Fatalf("record = %+v", rec)
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(rec.Directive.Body), &body); err != nil {
		t.Fatalf("body: %v", err)
	}
	summary, _ := body["summary"].(string)
	for _, want := range []string{`choir.Relate("", entities)`, "choir.StoryAnalysis()", "empty"} {
		if !strings.Contains(summary, want) {
			t.Fatalf("summary %q misses %q", summary, want)
		}
	}
	if story, ok := body["story"].(map[string]any); ok && story["text"] != nil && story["text"] != "" {
		t.Fatalf("the entities request carries the story's text: %v", story["text"])
	}
	if strings.Contains(summary, "Search for new reporting") {
		t.Fatalf("the entities request asks for new sources: %q", summary)
	}
	if again, err := ask("related-review-1:" + docID); err != nil || !again.Replay {
		t.Fatalf("replay = %+v, %v", again, err)
	}
	if _, err := ask("related-review-2:" + docID); !errors.Is(err, ErrStoryResearchBusy) {
		t.Fatalf("a second request while the round is open: %v, want busy", err)
	}
}

// A backfill round costs no Texture turn: research's empty Reports in a
// round the owner's entities request opened neither wake Texture nor count
// as its unread work. A Report that carries something still wakes it, and
// so does an empty Report in a reader's round. Failure modes: the
// backfill's empty final Report wakes Texture (one model turn per story);
// the rule reaches a reader's round, or a Report with findings, and
// Texture misses what research found.
func TestStoryEntitiesRoundEmptyReportsDoNotWakeTexture(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	docID, head := storyResearchFixture(t, s, "re-quiet")
	texture, research := "texture:"+docID, "research:"+docID
	if _, err := s.CommitStoryResearchRequest(ctx, StoryResearchRequest{OwnerID: storyOwner, ComputerID: storyComputer, DocumentID: docID,
		ClientRequestID: "related-review-q:" + docID, ExpectedHeadRevisionID: head, Kind: StoryResearchKindEntities}); err != nil {
		t.Fatal(err)
	}
	// Texture has read its inbox through the story's opening.
	if cur, head := mustReaderCursor(t, s, texture), mustInboxHead(t, s, texture); head > cur {
		mustCommitInbox(t, s, "turn-records-read:q0", &types.InboxCursorAdvance{ReaderAgentID: texture, Inbox: texture, ExpectedSeq: cur, Seq: head})
	}
	interim := mustCommitInbox(t, s, "cell-records:q-interim", nil, inboxReport("q-1", research, texture, false, types.CommitmentReport{}))
	final := mustCommitInbox(t, s, "cell-records:q-final", nil, inboxReport("q-2", research, texture, true, types.CommitmentReport{StopReason: "empty"}))
	for _, r := range []types.InboxCommitReceipt{interim, final} {
		if r.Records[0].WakeID != "" {
			t.Fatalf("an empty Report of the entities round woke Texture: %+v", r.Records[0])
		}
	}
	if !final.Records[0].ClosedRound {
		t.Fatalf("the final Report did not close the round: %+v", final.Records[0])
	}
	if unread, err := s.TextureWorkUnread(ctx, storyOwner, storyComputer, texture); err != nil || unread {
		t.Fatalf("Texture work unread after the quiet round = %v, %v; want none", unread, err)
	}

	// A reader's round: the same empty final Report wakes Texture.
	if _, err := askStoryResearch(s, docID, "expand:q", head); err != nil {
		t.Fatal(err)
	}
	reader := mustCommitInbox(t, s, "cell-records:r-final", nil, inboxReport("r-2", research, texture, true, types.CommitmentReport{StopReason: "empty"}))
	if reader.Records[0].WakeID == "" {
		t.Fatal("an empty final Report of a reader's round did not wake Texture")
	}
	if unread, err := s.TextureWorkUnread(ctx, storyOwner, storyComputer, texture); err != nil || !unread {
		t.Fatalf("Texture work unread after a reader's round = %v, %v; want some", unread, err)
	}
	cur, _ := s.ReaderCursor(ctx, storyOwner, storyComputer, texture, texture)
	mustCommitInbox(t, s, "turn-records-read:q", &types.InboxCursorAdvance{ReaderAgentID: texture, Inbox: texture, ExpectedSeq: cur, Seq: reader.Records[0].InboxSeq})

	// An entities round whose Report carries a finding wakes Texture.
	if _, err := s.CommitStoryResearchRequest(ctx, StoryResearchRequest{OwnerID: storyOwner, ComputerID: storyComputer, DocumentID: docID,
		ClientRequestID: "related-review-f:" + docID, ExpectedHeadRevisionID: head, Kind: StoryResearchKindEntities}); err != nil {
		t.Fatal(err)
	}
	found := mustCommitInbox(t, s, "cell-records:f-final", nil, inboxReport("f-2", research, texture, true,
		types.CommitmentReport{Findings: []types.ReportFinding{{Text: "the story names the wrong minister", Outside: true}}, StopReason: "completed"}))
	if found.Records[0].WakeID == "" {
		t.Fatal("a Report with a finding in an entities round did not wake Texture")
	}
}

func mustReaderCursor(t *testing.T, s *Store, inbox string) int64 {
	t.Helper()
	cur, err := s.ReaderCursor(context.Background(), storyOwner, storyComputer, inbox, inbox)
	if err != nil {
		t.Fatal(err)
	}
	return cur
}

func mustInboxHead(t *testing.T, s *Store, inbox string) int64 {
	t.Helper()
	head, err := s.InboxHead(context.Background(), storyOwner, storyComputer, inbox)
	if err != nil {
		t.Fatal(err)
	}
	return head
}
