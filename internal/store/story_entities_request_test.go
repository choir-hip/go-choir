package store

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
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
