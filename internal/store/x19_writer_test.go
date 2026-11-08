package store

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// X19 (modular architecture plan, section 8.4; MA3.7a) is resolved.
// Management used to open a story by writing Texture's story document,
// revision, story_library_head and support verdict in one batch with its own
// task and disposition rows. Now management's command writes its own rows and
// the Cast that asks Texture to open the story; Texture's host-side handler
// opens it as the Texture custodian. These tests run the gate in enforce mode,
// so a crossing would be an ErrWrongWriter, and require that no counter reads
// desk:management on a Texture kind.
func requireNoManagementCrossing(t *testing.T) {
	t.Helper()
	for key, n := range objectgraph.WriterGateCounts() {
		if n > 0 && (strings.Contains(key, "|wrong_writer") || strings.Contains(key, "|undeclared_writer")) {
			t.Errorf("writer gate counted %q = %d; the story opening must cross no custodian", key, n)
		}
	}
}

func managementOpeningCommand(t *testing.T, commandID, castID string) types.CommitInboxRecordsRequest {
	t.Helper()
	seen := int64(0)
	req := types.CommitInboxRecordsRequest{
		OwnerID: inboxTestOwner, ComputerID: inboxTestComputer, CommandID: commandID,
		Records:          []types.CommitmentRecord{openingCast(castID)},
		StoryLibraryHead: &seen,
	}
	var err error
	if req.CommandDigest, err = ComputeCommitInboxRecordsDigest(req); err != nil {
		t.Fatal(err)
	}
	return req
}

func TestManagementStoryOpeningCrossesNoCustodian(t *testing.T) {
	s := openTestStore(t)
	defer objectgraph.SetModeForTest(objectgraph.WriterGateEnforce)()
	objectgraph.ResetWriterGateCounts()
	ctx := objectgraph.WithWriter(context.Background(), objectgraph.WriterManagement)

	req := managementOpeningCommand(t, "x19-open", "x19:cast:1")
	receipt, err := s.CommitInboxRecords(ctx, req)
	if err != nil {
		t.Fatalf("management's story opening: %v", err)
	}
	requireNoManagementCrossing(t)
	if len(receipt.Records) != 1 || !receipt.Records[0].OpenedStory {
		t.Fatalf("receipt = %+v; want the Cast to have opened a story", receipt.Records)
	}
	docID, _, _ := storyOpeningIDs(inboxTestOwner, inboxTestComputer, "x19-open", "x19:cast:1")
	// The story exists in Texture's kinds, written by Texture.
	texture := textureCustodianCtx(context.Background())
	if _, err := s.lifecycleGetObject(texture, ogKindTexDoc, inboxTestOwner, inboxTestComputer, docID); err != nil {
		t.Fatalf("the opened story's document: %v", err)
	}
	head, err := s.StoryLibraryHeadOf(objectgraph.WithReader(context.Background(), objectgraph.WriterManagement), inboxTestOwner, inboxTestComputer)
	if err != nil || head != 1 || receipt.StoryLibraryHead != 1 {
		t.Fatalf("library head = %d (receipt %d), err %v; want 1", head, receipt.StoryLibraryHead, err)
	}
	// Management's view is the projection: the story is listed, and reading it
	// is no crossing into Texture's kinds.
	objectgraph.ResetReaderGateCounts()
	mgmt := objectgraph.WithReader(context.Background(), objectgraph.WriterManagement)
	rows, err := s.ListStoryIndex(mgmt, inboxTestOwner, inboxTestComputer, time.Time{}, "", 0)
	if err != nil || len(rows) != 1 || rows[0].DocID != docID || rows[0].Headline != "Harbor reopens after the storm" {
		t.Fatalf("story index = %+v, err %v", rows, err)
	}
	for key, n := range objectgraph.ReaderGateCounts() {
		if n > 0 {
			t.Errorf("reader gate counted %q = %d reading the story index as management", key, n)
		}
	}
	// A second open from a run that never saw the first is refused by the
	// projection's head, not Texture's.
	again := managementOpeningCommand(t, "x19-open-2", "x19:cast:2")
	if _, err := s.CommitInboxRecords(ctx, again); !errors.Is(err, ErrStoryLibraryMoved) {
		t.Fatalf("an open behind the library head: err = %v; want ErrStoryLibraryMoved", err)
	}
}

// B1 (Devin review): the open is ONE atomic commit. If Texture's part cannot
// commit (here its story's id is already taken), management's part does not
// either: no Cast delivered to a missing story, no wake for it, no receipt.
// Nothing is left to repair and no wake can be consumed against a missing
// document.
func TestStoryOpeningIsAtomicAcrossTheTwoCustodians(t *testing.T) {
	s := openTestStore(t)
	defer objectgraph.SetModeForTest(objectgraph.WriterGateEnforce)()
	ctx := objectgraph.WithWriter(context.Background(), objectgraph.WriterManagement)
	req, err := normalizeInboxCommand(managementOpeningCommand(t, "x19-atomic", "x19:cast:a"))
	if err != nil {
		t.Fatal(err)
	}
	_, openings, _, err := s.planStoryOpenings(req.OwnerID, req.ComputerID, req.CommandID, req.Records)
	if err != nil || len(openings) != 1 {
		t.Fatal(err)
	}
	o := openings[0]
	// Occupy the story's trajectory id, so Texture's part conflicts.
	objs, conds, edge, err := planStoryObjects(o.ownerID, o.computerID, o.docID, o.trajectoryID, o.seedID, o.cast, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	tex := textureCustodianCtx(context.Background())
	if err := s.ogStore.PutBatchConditional(tex, conds, objectgraph.Batch{Objects: objs[:1], Edges: nil}); err != nil {
		t.Fatal(err)
	}
	_ = edge
	wakesBefore := len(storyWakes(t, s))
	if _, err := s.CommitInboxRecords(ctx, req); err == nil {
		t.Fatal("the open committed over an occupied story id")
	}
	if recs, _ := s.ListInboxRecords(ctx, inboxTestOwner, inboxTestComputer, "texture:"+o.docID, 0, 0); len(recs) != 0 {
		t.Fatalf("the Cast was delivered to a story that was not opened: %d records", len(recs))
	}
	if _, found, _ := s.replayInboxCommand(ctx, inboxTestOwner, inboxTestComputer, req.CommandID, req.CommandDigest); found {
		t.Fatal("a receipt was committed for an open that did not happen")
	}
	if got := len(storyWakes(t, s)); got != wakesBefore {
		t.Fatalf("a wake row was minted for the failed open: %d -> %d", wakesBefore, got)
	}
}

// The success case: the Cast, its wake and the story are in the tape together
// after the one commit, so a wake can never run ahead of its document.
func TestStoryOpeningCastWakeAndStoryCommitTogether(t *testing.T) {
	s := openTestStore(t)
	ctx := objectgraph.WithWriter(context.Background(), objectgraph.WriterManagement)
	before := len(storyWakes(t, s))
	req := managementOpeningCommand(t, "x19-together", "x19:cast:t")
	if _, err := s.CommitInboxRecords(ctx, req); err != nil {
		t.Fatal(err)
	}
	docID, _, _ := storyOpeningIDs(inboxTestOwner, inboxTestComputer, "x19-together", "x19:cast:t")
	if _, err := s.lifecycleGetObject(textureCustodianCtx(context.Background()), ogKindTexDoc, inboxTestOwner, inboxTestComputer, docID); err != nil {
		t.Fatalf("story: %v", err)
	}
	if recs, _ := s.ListInboxRecords(ctx, inboxTestOwner, inboxTestComputer, "texture:"+docID, 0, 0); len(recs) != 1 {
		t.Fatalf("Cast records = %d", len(recs))
	}
	if got := len(storyWakes(t, s)); got <= before {
		t.Fatalf("no wake minted for the opening Cast (%d -> %d)", before, got)
	}
	// A retry of the committed command replays; it opens nothing more.
	if r, err := s.CommitInboxRecords(ctx, req); err != nil || !r.Replay {
		t.Fatalf("retry: %v replay=%v", err, r.Replay)
	}
	if head, _ := s.StoryLibraryHeadOf(textureCustodianCtx(context.Background()), inboxTestOwner, inboxTestComputer); head != 1 {
		t.Fatalf("library head = %d", head)
	}
}

// A withdrawn (tombstoned) index row reads as absent and is rewritten by the
// next publish.
func TestPublishStoryIndexRewritesAWithdrawnRow(t *testing.T) {
	s := openTestStore(t)
	ctx := objectgraph.WithWriter(context.Background(), objectgraph.WriterManagement)
	if _, err := s.CommitInboxRecords(ctx, managementOpeningCommand(t, "x19-tomb", "x19:cast:w")); err != nil {
		t.Fatal(err)
	}
	tex := textureCustodianCtx(context.Background())
	objs, err := s.ogListAllObjectsByKind(tex, ogKindStoryIndex)
	if err != nil || len(objs) == 0 {
		t.Fatalf("rows: %d %v", len(objs), err)
	}
	for _, obj := range objs {
		obj.Tombstone = true
		if err := s.ogStore.PutBatch(tex, objectgraph.Batch{Objects: []objectgraph.Object{obj}}); err != nil {
			t.Fatal(err)
		}
	}
	mgmt := objectgraph.WithReader(context.Background(), objectgraph.WriterManagement)
	if rows, err := s.ListStoryIndex(mgmt, inboxTestOwner, inboxTestComputer, time.Time{}, "", 0); err != nil || len(rows) != 0 {
		t.Fatalf("withdrawn rows listed: %d %v", len(rows), err)
	}
	if err := s.PublishStoryIndex(mgmt, inboxTestOwner, inboxTestComputer); err != nil {
		t.Fatal(err)
	}
	if rows, err := s.ListStoryIndex(mgmt, inboxTestOwner, inboxTestComputer, time.Time{}, "", 0); err != nil || len(rows) != 1 {
		t.Fatalf("after publish: %d %v", len(rows), err)
	}
	if head, _ := s.StoryLibraryHeadOf(mgmt, inboxTestOwner, inboxTestComputer); head != 1 {
		t.Fatalf("mirror head = %d", head)
	}
}

// An old tape has the library head and the stories but no projection: the
// custodian publishes the index on demand, and management's list reads it.
func TestPublishStoryIndexBackfillsAnOldTape(t *testing.T) {
	s := openTestStore(t)
	// A story written before the projection existed: its objects, no index row.
	req, err := normalizeInboxCommand(managementOpeningCommand(t, "x19-old", "x19:cast:o"))
	if err != nil {
		t.Fatal(err)
	}
	_, openings, _, err := s.planStoryOpenings(req.OwnerID, req.ComputerID, req.CommandID, req.Records)
	if err != nil || len(openings) != 1 {
		t.Fatalf("openings: %v %d", err, len(openings))
	}
	o := openings[0]
	objs, conds, edge, err := planStoryObjects(o.ownerID, o.computerID, o.docID, o.trajectoryID, o.seedID, o.cast, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	tex := textureCustodianCtx(context.Background())
	if err := s.ogStore.PutBatchConditional(tex, conds, objectgraph.Batch{Objects: objs, Edges: []objectgraph.Edge{edge}}); err != nil {
		t.Fatal(err)
	}
	mgmt := objectgraph.WithReader(context.Background(), objectgraph.WriterManagement)
	if rows, err := s.ListStoryIndex(mgmt, inboxTestOwner, inboxTestComputer, time.Time{}, "", 0); err != nil || len(rows) != 0 {
		t.Fatalf("index rows before the backfill = %d, %v", len(rows), err)
	}
	if err := s.PublishStoryIndex(mgmt, inboxTestOwner, inboxTestComputer); err != nil {
		t.Fatal(err)
	}
	rows, err := s.ListStoryIndex(mgmt, inboxTestOwner, inboxTestComputer, time.Time{}, "", 0)
	if err != nil || len(rows) != 1 || rows[0].DocID != o.docID {
		t.Fatalf("index after backfill = %+v, %v", rows, err)
	}
	// Publishing again writes nothing new.
	if err := s.PublishStoryIndex(mgmt, inboxTestOwner, inboxTestComputer); err != nil {
		t.Fatal(err)
	}
}

// The support verdict is Texture's kind. The Jev job writes it for Texture, so
// it declares the Texture custodian from the registry (production does,
// agentcore/support_judge.go); declared so, the command crosses nothing.
func TestSupportVerdictCommandAsTextureCustodianCrossesNothing(t *testing.T) {
	s, start, caller, _ := setupLifecycleTextureTargetFixture(t)
	system := types.SupportSystemAgentID
	a := supportPair("x19-rev-1", "p-1", "The merger may proceed.", "merger may proceed")
	supportRevisionTurn(t, s, start, caller, "x19-rev-1", "The merger may proceed.", a)

	defer objectgraph.SetModeForTest(objectgraph.WriterGateEnforce)()
	objectgraph.ResetWriterGateCounts()
	req := types.CommitInboxRecordsRequest{OwnerID: start.OwnerID, ComputerID: start.ComputerID, CommandID: "x19-verdict",
		Judge: system, Verdicts: []types.SupportVerdict{{Key: a.Key, Verdict: types.SupportSupported, Judge: "jev:typesafe/jev-1.13", PSupportedBP: 9100}}}
	req.CommandDigest, _ = ComputeCommitInboxRecordsDigest(req)
	custodian, ok := objectgraph.CustodianOf(ogKindSupportVerdict)
	if !ok {
		t.Fatal("support_verdict has no custodian")
	}
	if _, err := s.CommitInboxRecords(objectgraph.WithWriter(context.Background(), custodian), req); err != nil {
		t.Fatalf("verdicts as the custodian: %v", err)
	}
	requireNoManagementCrossing(t)
}
