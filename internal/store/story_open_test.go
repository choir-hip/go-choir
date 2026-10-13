package store

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// Living stories in the store (Cut 2 2b; pre-flight review A6). Ways it can
// fail, written before the code:
//   - a capture arrival record is a system record, and system records wake
//     no one, so management never hears of the captures;
//   - opening a story mints an owner-authored seed revision (which the
//     owner_revision wake turns into a Texture run racing the Cast), or a
//     work item (whose lifecycle_work_assigned wake does the same), or no
//     desk origin;
//   - the Cast lands under "texture:new" instead of the new story's inbox,
//     so Texture's records wake never fires;
//   - a retried opening mints a second story; a different command under
//     the same id is accepted;
//   - a Note, or a Texture, opens a story;
//   - a story with no work items settles the first time its run ends, so
//     the next Cast or Note to it wakes nothing.

const storyOwner, storyComputer = inboxTestOwner, inboxTestComputer

func storyManagement() string { return "management:" + storyOwner }

func openingCast(id string) types.CommitmentRecord {
	rec := inboxCast(id, storyManagement(), types.StoryOpenInbox, types.CommitmentCast{
		Hypotheses: []types.CastHypothesis{hyp("the harbor reopens Monday", 0.7)},
		Captures:   []string{"capture-a", "capture-b"},
		Publish:    types.CastPublishAuto,
	})
	rec.Directive.Objective = "Harbor reopens after the storm"
	return rec
}

func storyWakes(t *testing.T, s *Store) []ActorWakeOutbox {
	t.Helper()
	wakes, err := s.ListUnprojectedActorWakes(context.Background())
	if err != nil {
		t.Fatalf("list wakes: %v", err)
	}
	return wakes
}

func TestCaptureArrivalRecordWakesManagement(t *testing.T) {
	s := openTestStore(t)
	arrival := types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1, Kind: types.CommitmentKindReport, Discrepancy: types.DiscrepancyUnresolved,
		RecordID:    types.CaptureArrivalRecordID("rss:port", []string{"capture-a"}),
		Provenance:  types.CommitmentProvenance{AgentID: types.CaptureArrivalSystemAgentID},
		EmitterKind: types.CommitmentEmitterSystem, Inbox: storyManagement(), SenderInbox: types.CaptureArrivalSenderInbox("rss:port"),
		Captures: &types.CommitmentCaptures{SourceID: "rss:port", Items: []types.CaptureArrival{{CaptureID: "capture-a", Title: "Port", URL: "https://port.example/a"}}},
	}
	receipt := mustCommitInbox(t, s, "arrival-1", nil, arrival)
	if len(receipt.Records) != 1 || receipt.Records[0].WakeID == "" || receipt.Records[0].Inbox != storyManagement() {
		t.Fatalf("arrival receipt = %+v; want one record to management that wakes it", receipt.Records)
	}
	// An empty arrival is not news: it is refused rather than waking anyone.
	empty := arrival
	empty.RecordID, empty.Captures = "capture-arrival:empty", &types.CommitmentCaptures{SourceID: "rss:port"}
	mustRefuseInbox(t, s, "arrival-empty", empty)
}

func TestOpeningCastCreatesOneStoryWithNoOwnerRevisionOrWork(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	cast := openingCast("cell-1:cast:c1")
	receipt := mustCommitInbox(t, s, "cell-records:cell-1", nil, cast)
	if len(receipt.Records) != 1 {
		t.Fatalf("receipt = %+v", receipt)
	}
	row := receipt.Records[0]
	docID := strings.TrimPrefix(row.Inbox, "texture:")
	if !row.OpenedStory || !strings.HasPrefix(row.Inbox, "texture:") || docID == "" || docID == "new" || row.WakeID == "" || !row.OpenedRound || len(row.HypothesisIDs) != 1 {
		t.Fatalf("opening row = %+v; want the new story's inbox, a wake, round 1 and H1", row)
	}

	doc, err := s.GetLifecycleDocument(ctx, storyOwner, storyComputer, docID)
	if err != nil || doc.Title != "Harbor reopens after the storm" || doc.TrajectoryID == "" || doc.CurrentRevisionID == "" {
		t.Fatalf("story document = %+v, %v", doc, err)
	}
	seed, err := s.GetLifecycleRevision(ctx, storyOwner, storyComputer, doc.CurrentRevisionID)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	var seedMeta map[string]any
	_ = json.Unmarshal(seed.Metadata, &seedMeta)
	if seed.AuthorKind == types.AuthorUser || strings.TrimSpace(seed.Content) != "" || seedMeta["revision_role"] != types.StorySeedRole || seedMeta["origin_desk"] != "management" {
		t.Fatalf("seed = author %s content %q metadata %s; want an empty system seed that is no owner's input", seed.AuthorKind, seed.Content, seed.Metadata)
	}
	snapshot, err := s.GetLifecycleSnapshot(ctx, storyOwner, storyComputer, doc.TrajectoryID)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	refs := snapshot.Trajectory.SubjectRefs
	if snapshot.Trajectory.Kind != types.TrajectoryKindStory || snapshot.Trajectory.Status != types.TrajectoryLive ||
		refs[types.StoryRefOriginDesk] != "management" || refs[types.StoryRefPublish] != types.CastPublishAuto ||
		refs[types.StoryRefOpeningRecord] != cast.RecordID || refs["doc_id"] != docID {
		t.Fatalf("story trajectory = %+v", snapshot.Trajectory)
	}
	if !types.StoryPublishesAuto(snapshot.Trajectory) {
		t.Fatal("a publish auto story does not report it")
	}
	if len(snapshot.WorkItems) != 0 {
		t.Fatalf("opening minted work items: %+v", snapshot.WorkItems)
	}
	if len(snapshot.Agents) != 1 || snapshot.Agents[0].AgentID != row.Inbox || snapshot.Agents[0].LifecycleVersion != 1 {
		t.Fatalf("story agents = %+v; want the Texture desk alone", snapshot.Agents)
	}
	// One wake, the Cast's record wake to the story's Texture: no
	// owner_revision and no lifecycle_work_assigned wake races it.
	wakes := storyWakes(t, s)
	if len(wakes) != 1 || wakes[0].TargetAgentID != row.Inbox || wakes[0].Kind != InboxRecordWakeKind {
		t.Fatalf("wakes after opening = %+v; want the one records wake to %s", wakes, row.Inbox)
	}
	records, err := s.ListInboxRecords(ctx, storyOwner, storyComputer, row.Inbox, 0, 0)
	if err != nil || len(records) != 1 || records[0].Cast == nil || records[0].Cast.Publish != types.CastPublishAuto ||
		len(records[0].Cast.Captures) != 2 || records[0].Addressee != row.Inbox {
		t.Fatalf("story inbox = %+v, %v", records, err)
	}
	if n, _ := s.InboxHead(ctx, storyOwner, storyComputer, types.StoryOpenInbox); n != 0 {
		t.Fatalf("a record was stored under %s", types.StoryOpenInbox)
	}

	// A retried opening replays: no second story.
	again := mustCommitInbox(t, s, "cell-records:cell-1", nil, cast)
	if !again.Replay || again.Records[0].Inbox != row.Inbox {
		t.Fatalf("retried opening = %+v; want a replay naming %s", again, row.Inbox)
	}
	changed := openingCast("cell-1:cast:c1")
	changed.Directive.Objective = "Another story"
	if _, err := commitInbox(t, s, "cell-records:cell-1", nil, changed); !errors.Is(err, ErrLifecycleCommandConflict) {
		t.Fatalf("a different command under the opening's id = %v; want a conflict", err)
	}
	if wakes := storyWakes(t, s); len(wakes) != 1 {
		t.Fatalf("retries minted wakes: %+v", wakes)
	}

	// A story never settles because its work drained.
	prospective := snapshot.Trajectory
	prospective.TerminalArtifactHeadRef = doc.CurrentRevisionID
	if ready, err := s.lifecycleSettlementReady(ctx, prospective, nil, nil); err != nil || ready {
		t.Fatalf("story settlement ready = %v, %v; want never", ready, err)
	}
}

func TestOnlyAManagementCastOpensAStory(t *testing.T) {
	s := openTestStore(t)
	note := inboxNote("note-new", storyManagement(), types.StoryOpenInbox)
	mustRefuseInbox(t, s, "cmd-note-new", note)
	texture := inboxCast("texture-new", "texture:doc-1", types.StoryOpenInbox, types.CommitmentCast{})
	mustRefuseInbox(t, s, "cmd-texture-new", texture)
	other := inboxCast("other-owner-new", "management:someone-else", types.StoryOpenInbox, types.CommitmentCast{})
	mustRefuseInbox(t, s, "cmd-other-new", other)
	if wakes := storyWakes(t, s); len(wakes) != 0 {
		t.Fatalf("refused openings minted wakes: %+v", wakes)
	}
}
