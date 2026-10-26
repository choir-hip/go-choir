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
//  D6 (review 325 F3) A story with no text yet (the seed management opens
//     it with) is asked, cannot be analysed, and is asked again by every
//     batch: it is skipped and counted.
//  D7 (review 325 F2) The scan is unbounded: an act reads at most
//     storyEntitiesBackfillScanMax stories and returns the cursor to go
//     on from.
//  D8 (review 325 F3, 324 F1) A story about none of its candidates, asked
//     by the backfill, is recorded and not asked again.

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// seedStory starts a story lifecycle whose head is text, as a story looks
// once Texture has written it, and returns its doc id.
func seedStory(t *testing.T, s *store.Store, owner, computer, docID, title, text string) string {
	t.Helper()
	trajectoryID, texture := "trajectory-"+docID, agentprofile.Texture+":"+docID
	now := time.Now().UTC()
	start := types.StartLifecycleRequest{
		OwnerID: owner, ComputerID: computer, CommandID: "start-" + docID, TrajectoryID: trajectoryID, Kind: types.TrajectoryKindStory,
		SubjectRefs:     map[string]string{"artifact": "texture://documents/" + docID, "doc_id": docID},
		SettlementRule:  types.SettlementRule{Version: types.LifecycleReducerVersion, RequireNoOpenWorkItems: true, RequiredSubjectRefs: []string{"artifact"}},
		InitialWork:     types.WorkItemRecord{WorkItemID: "work-" + docID, Objective: "write the story", AssignedAgentID: texture, AuthorityProfile: agentprofile.Texture},
		InitialDocument: types.Document{DocID: docID, OwnerID: owner, ComputerID: computer, TrajectoryID: trajectoryID, Title: title, CreatedAt: now, UpdatedAt: now},
		InitialRevision: types.Revision{RevisionID: "rev-" + docID, DocID: docID, OwnerID: owner, ComputerID: computer, TrajectoryID: trajectoryID, AuthorKind: types.AuthorUser, AuthorLabel: owner, Content: text, CreatedAt: now},
		Agent:           types.AgentRecord{AgentID: texture, OwnerID: owner, ComputerID: computer, Profile: agentprofile.Texture, Role: agentprofile.Texture, ChannelID: docID, CreatedAt: now, UpdatedAt: now},
	}
	start.StartRequestDigest, _ = store.ComputeStartLifecycleRequestDigest(start)
	if _, err := s.StartLifecycle(context.Background(), start); err != nil {
		t.Fatalf("start story %s: %v", docID, err)
	}
	return docID
}

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
		docs = append(docs, seedStory(t, f.s, f.owner, f.computer, fmt.Sprintf("doc-%d", i), fmt.Sprintf("Story %d", i), fmt.Sprintf("Story %d is about a harbor.", i)))
	}
	// D6: a story with only its seed is not asked.
	seedOnly := openStory(t, f.s, f.owner, f.computer, "open-seed", "A story not written yet")
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
	if len(got.Requested) != 2 || got.Recorded != 1 || got.Empty != 1 || got.Next != nil {
		t.Fatalf("backfill = %+v, want the two stories with no record asked, one recorded and one empty skipped", got)
	}
	for _, doc := range got.Requested {
		if doc == recorded || doc == seedOnly {
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
	doc := seedStory(t, f.s, f.owner, f.computer, "doc-quiet", "A quiet story", "A harbor reopens after the storm.")
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

func TestStoryEntitiesBackfillScanIsBoundedAndGoesOn(t *testing.T) { // D7
	f := newAnalysisFixture(t)
	f.turnOn(t)
	ctx := context.Background()
	defer func(n int) { storyEntitiesBackfillScanMax = n }(storyEntitiesBackfillScanMax)
	storyEntitiesBackfillScanMax = 2
	var docs []string
	for i := 0; i < 4; i++ {
		docs = append(docs, seedStory(t, f.s, f.owner, f.computer, fmt.Sprintf("doc-scan-%d", i), fmt.Sprintf("Story %d", i), "A harbor reopens."))
		time.Sleep(5 * time.Millisecond)
	}
	// The two newest have their records.
	for _, doc := range docs[2:] {
		if err := f.rt.appendStoryEntitiesEvent(ctx, f.exec("run-seed"), f.owner, f.computer, "run-seed", "seed-"+doc,
			storyEntitiesRecord{Computer: f.computer, Story: types.StoryPublicationID(f.owner, doc), Attachments: []storyEntityAttachment{{EntityID: "wd:Q1", Role: types.EntityRoleCentral}}}); err != nil {
			t.Fatal(err)
		}
	}
	first, err := f.rt.StoryEntitiesBackfill(ctx, f.owner, f.computer, "batch-scan", 10)
	if err != nil || first.Scanned != 2 || first.Recorded != 2 || len(first.Requested) != 0 || first.Next == nil || first.Next.BeforeDoc != docs[2] {
		t.Fatalf("first act = %+v, %v; want two scanned, both recorded, and the cursor after %s", first, err, docs[2])
	}
	next, err := f.rt.StoryEntitiesBackfillFrom(ctx, f.owner, f.computer, "batch-scan", 10, *first.Next)
	if err != nil || next.Scanned != 2 || len(next.Requested) != 2 || next.Next != nil {
		t.Fatalf("next act = %+v, %v; want the two older stories asked and no cursor", next, err)
	}
}

func TestStoryEntitiesBackfillRecordsAStoryAboutNone(t *testing.T) { // D8
	f := newAnalysisFixture(t)
	f.turnOn(t)
	ctx := context.Background()
	doc := seedStory(t, f.s, f.owner, f.computer, "doc-none", "An essay on patience", "Patience is a virtue that most of us forget.")
	got, err := f.rt.StoryEntitiesBackfill(ctx, f.owner, f.computer, "batch-none", 10)
	if err != nil || len(got.Requested) != 1 || got.Requested[0] != doc {
		t.Fatalf("backfill = %+v, %v", got, err)
	}
	// The research run the request starts: no candidates, so the story is
	// recorded as about none of them.
	f.cpub.resp = types.EntityCandidatesResponse{}
	if a := f.analyse(t, agentprofile.Research, "run-none", doc); a == nil || a.Error != "" || len(a.Candidates) != 0 {
		t.Fatalf("analysis = %+v", a)
	}
	payload, _ := json.Marshal(yaegikernel.RelatePayload{})
	if _, err := f.rt.deskEgress(ctx, agentprofile.Research, f.deskExec("run-none", doc), yaegikernel.ActionRelate, payload); err != nil {
		t.Fatalf("Relate(\"\", nil): %v", err)
	}
	if err := f.finishGate(t, "run-none", "research:"+doc, agentprofile.Research); err != nil {
		t.Fatalf("finish rule: %v", err)
	}
	again, err := f.rt.StoryEntitiesBackfill(ctx, f.owner, f.computer, "batch-none-2", 10)
	if err != nil || len(again.Requested) != 0 || again.Recorded != 1 {
		t.Fatalf("next batch = %+v, %v; want the story recorded, not asked", again, err)
	}
}

// Review 327 Nit 4: a scan bound under one still reads a story, and never
// panics.
func TestStoryEntitiesBackfillScanBoundIsAtLeastOne(t *testing.T) {
	f := newAnalysisFixture(t)
	f.turnOn(t)
	defer func(n int) { storyEntitiesBackfillScanMax = n }(storyEntitiesBackfillScanMax)
	storyEntitiesBackfillScanMax = 0
	for i := 0; i < 2; i++ {
		seedStory(t, f.s, f.owner, f.computer, fmt.Sprintf("doc-bound-%d", i), "A story", "A harbor reopens.")
	}
	got, err := f.rt.StoryEntitiesBackfill(context.Background(), f.owner, f.computer, "batch-bound", 5)
	if err != nil || got.Scanned != 1 || len(got.Requested) != 1 || got.Next == nil {
		t.Fatalf("act = %+v, %v; want one story read and the cursor", got, err)
	}
}
