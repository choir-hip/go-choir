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
	defer objectgraph.SetWriterGateMode(objectgraph.WriterGateEnforce)()
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

// A crash between management's request and Texture's open leaves the Cast
// committed with no story. The command's retry repairs it: the receipt is
// found, and the open runs again exactly once.
func TestStoryOpeningRepairsAfterTheRequestCommittedAlone(t *testing.T) {
	s := openTestStore(t)
	defer objectgraph.SetWriterGateMode(objectgraph.WriterGateEnforce)()
	ctx := objectgraph.WithWriter(context.Background(), objectgraph.WriterManagement)
	req := managementOpeningCommand(t, "x19-repair", "x19:cast:r")
	req, err := normalizeInboxCommand(req)
	if err != nil {
		t.Fatal(err)
	}
	objects, conditions, openings, _, err := s.planInboxCommand(ctx, req, time.Now().UTC())
	if err != nil || len(openings) != 1 {
		t.Fatalf("plan: %v (%d openings)", err, len(openings))
	}
	if err := s.ogStore.PutBatchConditional(ctx, conditions, objectgraph.Batch{Objects: objects}); err != nil {
		t.Fatal(err)
	}
	docID := openings[0].docID
	texture := textureCustodianCtx(context.Background())
	if _, err := s.lifecycleGetObject(texture, ogKindTexDoc, inboxTestOwner, inboxTestComputer, docID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("story before the repair: err = %v; want not found", err)
	}
	receipt, err := s.CommitInboxRecords(ctx, req)
	if err != nil || !receipt.Replay {
		t.Fatalf("retry: replay=%v err=%v", receipt.Replay, err)
	}
	if _, err := s.lifecycleGetObject(texture, ogKindTexDoc, inboxTestOwner, inboxTestComputer, docID); err != nil {
		t.Fatalf("story after the repair: %v", err)
	}
	if head, _ := s.StoryLibraryHeadOf(textureCustodianCtx(context.Background()), inboxTestOwner, inboxTestComputer); head != 1 {
		t.Fatalf("library head after the repair = %d; want 1", head)
	}
	// A second retry opens nothing more.
	if _, err := s.CommitInboxRecords(ctx, req); err != nil {
		t.Fatal(err)
	}
	if head, _ := s.StoryLibraryHeadOf(textureCustodianCtx(context.Background()), inboxTestOwner, inboxTestComputer); head != 1 {
		t.Fatalf("library head after a second retry = %d; want 1", head)
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

	defer objectgraph.SetWriterGateMode(objectgraph.WriterGateEnforce)()
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
