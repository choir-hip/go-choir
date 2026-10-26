package agentcore

// Slice d (docs/problems/related-stories-no-entity-layer-2026-10-26.md): an
// owner act asks the research desks of stories with no entities record to
// record what their story is about, newest first, bounded, through the
// reader's request path. Failure modes, written first:
//
//  D1 A story that has its record is asked again.
//  D2 The act runs with the setting off, or unbounded.
//  D3 A repeat of the act casts again (it replays by its batch id).
//  D4 A busy story stops the batch: it is skipped and counted.

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// openStory opens a story the way management does, a Cast to texture:new,
// and returns its doc id.
func openStory(t *testing.T, s *store.Store, owner, computer, id, headline string) string {
	t.Helper()
	management := "management:" + owner
	rec := types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: id, Discrepancy: types.DiscrepancyUnresolved,
		Provenance:  types.CommitmentProvenance{AgentID: management, CommittedAt: time.Now().UTC().Format(time.RFC3339Nano)},
		EmitterKind: types.CommitmentEmitterAgent, Inbox: types.StoryOpenInbox, SenderInbox: management,
		Kind:      types.CommitmentKindDirective,
		Directive: &types.CommitmentDirective{Subtype: types.CommitmentDirectiveCast, Objective: headline},
		Cast: &types.CommitmentCast{
			Hypotheses: []types.CastHypothesis{{Statement: headline, Probability: 0.7, Stakes: "the lede", Crux: "a primary record"}},
			Captures:   []string{"capture-a", "capture-b"}, Publish: types.CastPublishAuto,
		},
	}
	req := types.CommitInboxRecordsRequest{OwnerID: owner, ComputerID: computer, CommandID: "cell-records:" + id, Records: []types.CommitmentRecord{rec}}
	req.CommandDigest, _ = store.ComputeCommitInboxRecordsDigest(req)
	receipt, err := s.CommitInboxRecords(context.Background(), req)
	if err != nil {
		t.Fatalf("open story %s: %v", id, err)
	}
	inbox := receipt.Records[0].Inbox
	return inbox[len("texture:"):]
}

func TestStoryEntitiesBackfillAsksOnlyStoriesWithNoRecord(t *testing.T) {
	f := newAnalysisFixture(t)
	ctx := context.Background()
	var docs []string
	for i := 0; i < 3; i++ {
		docs = append(docs, openStory(t, f.s, f.owner, f.computer, fmt.Sprintf("open-%d", i), fmt.Sprintf("Story %d", i)))
	}
	// D2: off refuses.
	if _, err := f.rt.StoryEntitiesBackfill(ctx, f.owner, f.computer, "batch-1", 10); err == nil {
		t.Fatal("the backfill ran with research.related_story off")
	}
	f.turnOn(t)
	// D1: one story already has its record.
	recorded := docs[1]
	if err := f.rt.appendStoryEntitiesEvent(ctx, f.exec("run-seed"), f.owner, f.computer, "run-seed", "seed-record",
		storyEntitiesRecord{Computer: f.computer, Story: types.StoryPublicationID(f.owner, recorded), Attachments: []storyEntityAttachment{{EntityID: "wd:Q1", Role: types.EntityRoleCentral}}}); err != nil {
		t.Fatal(err)
	}
	got, err := f.rt.StoryEntitiesBackfill(ctx, f.owner, f.computer, "batch-1", 10)
	if err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if len(got.Requested) != 2 || got.Recorded != 1 {
		t.Fatalf("backfill = %+v, want the two stories with no record asked and one skipped", got)
	}
	for _, doc := range got.Requested {
		if doc == recorded {
			t.Fatalf("the recorded story %s was asked again", doc)
		}
		rec, err := f.s.GetCommitmentRecord(ctx, f.owner, f.computer, store.StoryEntitiesRecordID("batch-1:"+doc))
		if err != nil || rec == nil || rec.Inbox != "research:"+doc {
			t.Fatalf("request of %s = %+v, %v", doc, rec, err)
		}
	}
	// D3: the same batch replays; D4: a new batch finds them busy.
	again, err := f.rt.StoryEntitiesBackfill(ctx, f.owner, f.computer, "batch-1", 10)
	if err != nil || len(again.Requested) != 2 || again.Replayed != 2 {
		t.Fatalf("repeat = %+v, %v; want both replayed", again, err)
	}
	next, err := f.rt.StoryEntitiesBackfill(ctx, f.owner, f.computer, "batch-2", 10)
	if err != nil || len(next.Requested) != 0 || next.Busy != 2 {
		t.Fatalf("new batch while their rounds are open = %+v, %v; want both busy", next, err)
	}
	// D2: the limit binds, and a limit past the bound is refused.
	if _, err := f.rt.StoryEntitiesBackfill(ctx, f.owner, f.computer, "batch-3", StoryEntitiesBackfillMax+1); err == nil {
		t.Fatal("an unbounded backfill was accepted")
	}
}

// The Texture controller reads the quiet round as no work (D5).
func TestStoryEntitiesBackfillRoundLeavesTextureNoWork(t *testing.T) {
	f := newAnalysisFixture(t)
	ctx := context.Background()
	doc := openStory(t, f.s, f.owner, f.computer, "open-quiet", "A quiet story")
	f.turnOn(t)
	got, err := f.rt.StoryEntitiesBackfill(ctx, f.owner, f.computer, "batch-q", 10)
	if err != nil || len(got.Requested) != 1 {
		t.Fatalf("backfill = %+v, %v", got, err)
	}
	texture, research := "texture:"+doc, "research:"+doc
	// Texture has read its inbox through the request.
	cur, _ := f.s.ReaderCursor(ctx, f.owner, f.computer, texture, texture)
	head, _ := f.s.InboxHead(ctx, f.owner, f.computer, texture)
	if head > cur {
		read := types.CommitInboxRecordsRequest{OwnerID: f.owner, ComputerID: f.computer, CommandID: "turn-records-read:quiet",
			Cursor: &types.InboxCursorAdvance{ReaderAgentID: texture, Inbox: texture, ExpectedSeq: cur, Seq: head}}
		read.CommandDigest, _ = store.ComputeCommitInboxRecordsDigest(read)
		if _, err := f.s.CommitInboxRecords(ctx, read); err != nil {
			t.Fatal(err)
		}
	}
	if unread, err := f.rt.InboxUnread(ctx, f.owner, f.computer, texture); err != nil || unread {
		t.Fatalf("Texture unread before the final Report = %v, %v", unread, err)
	}
	final := types.CommitmentRecord{SchemaID: types.CommitmentRecordSchemaV1, RecordID: "quiet-final", Kind: types.CommitmentKindReport,
		Discrepancy: types.DiscrepancyUnresolved, EmitterKind: types.CommitmentEmitterAgent, Provenance: types.CommitmentProvenance{AgentID: research},
		Inbox: texture, SenderInbox: research, Addressee: texture, Final: true, Report: &types.CommitmentReport{StopReason: "empty"}}
	req := types.CommitInboxRecordsRequest{OwnerID: f.owner, ComputerID: f.computer, CommandID: "cell-records:quiet-final", Records: []types.CommitmentRecord{final}}
	req.CommandDigest, _ = store.ComputeCommitInboxRecordsDigest(req)
	receipt, err := f.s.CommitInboxRecords(ctx, req)
	if err != nil || receipt.Records[0].WakeID != "" {
		t.Fatalf("final = %+v, %v; want no wake", receipt, err)
	}
	if unread, err := f.rt.InboxUnread(ctx, f.owner, f.computer, texture); err != nil || unread {
		t.Fatalf("Texture unread after the quiet final Report = %v, %v; want none", unread, err)
	}
}
