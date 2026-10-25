package textureowner

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/quotematch"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/texturedoc"
	"github.com/yusefmosiah/go-choir/internal/texturewrite"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// Texture's turn through the real store (Cut 2 2a, item iv). Ways it can
// fail, written first:
//   - a Write that adds one paragraph re-mints every block id, so citations
//     and judgements keyed on block ids churn (revision JSON diff);
//   - a cite of an uncaptured source commits (A2), or the refusal loses its
//     line;
//   - a turn written against a frame the owner has since edited commits over
//     the edit;
//   - Done leaves the caller's work open, or cannot close it without a
//     revision;
//   - Wait or Reject drops the dispatch fold: the Cast's record never
//     reaches research, the research desk is never minted, or the cursor
//     does not advance;
//   - a replayed cell commits twice.

// seedCellAuthorLifecycle starts a bound Texture document lifecycle (doc +
// initial owner revision + texture agent + open work item) and projects the
// texture caller run onto it. It returns the caller run record a cell
// commits under. shape, when given, edits the initial revision (a body with
// citations).
func seedCellAuthorLifecycle(t *testing.T, s *store.Store, ownerID, suffix, content string, shape ...func(*types.Revision)) types.RunRecord {
	t.Helper()
	return seedCellAuthorLifecycleOn(t, s, "autoputer-cell-author", ownerID, suffix, content, shape...)
}

// seedCellAuthorLifecycleOn is seedCellAuthorLifecycle on a named computer
// (the owner routes resolve documents on the runtime's own computer).
func seedCellAuthorLifecycleOn(t *testing.T, s *store.Store, computerID, ownerID, suffix, content string, shape ...func(*types.Revision)) types.RunRecord {
	t.Helper()
	ctx := context.Background()
	docID := "doc-cell-author-" + suffix
	trajectoryID := "trajectory-cell-author-" + suffix
	textureAgentID := agentprofile.Texture + ":" + docID
	now := time.Now().UTC()
	start := types.StartLifecycleRequest{
		OwnerID: ownerID, ComputerID: computerID, CommandID: "start-cell-author-" + suffix,
		TrajectoryID: trajectoryID, Kind: types.TrajectoryKindDocument,
		SubjectRefs:     map[string]string{"artifact": "texture://documents/" + docID, "doc_id": docID},
		SettlementRule:  types.SettlementRule{Version: types.LifecycleReducerVersion, RequireNoOpenWorkItems: true, RequiredSubjectRefs: []string{"artifact"}},
		InitialWork:     types.WorkItemRecord{WorkItemID: "work-cell-author-" + suffix, Objective: "author the supervision doc", AssignedAgentID: textureAgentID, AuthorityProfile: agentprofile.Texture},
		InitialDocument: types.Document{DocID: docID, OwnerID: ownerID, ComputerID: computerID, TrajectoryID: trajectoryID, Title: "Cell author " + suffix, CreatedAt: now, UpdatedAt: now},
		InitialRevision: types.Revision{RevisionID: "revision-cell-author-base-" + suffix, DocID: docID, OwnerID: ownerID, ComputerID: computerID, TrajectoryID: trajectoryID, AuthorKind: types.AuthorUser, AuthorLabel: ownerID, Content: content, CreatedAt: now},
		Agent:           types.AgentRecord{AgentID: textureAgentID, OwnerID: ownerID, ComputerID: computerID, Profile: agentprofile.Texture, Role: agentprofile.Texture, ChannelID: docID, CreatedAt: now, UpdatedAt: now},
	}
	for _, edit := range shape {
		edit(&start.InitialRevision)
	}
	start.StartRequestDigest, _ = store.ComputeStartLifecycleRequestDigest(start)
	if _, err := s.StartLifecycle(ctx, start); err != nil {
		t.Fatalf("start lifecycle: %v", err)
	}
	caller := types.RunRecord{
		RunID: "texture-run-cell-author-" + suffix, OwnerID: ownerID, ComputerID: computerID,
		AgentID: textureAgentID, AgentProfile: agentprofile.Texture, AgentRole: agentprofile.Texture,
		ChannelID: docID, TrajectoryID: trajectoryID, State: types.RunRunning,
		Metadata: map[string]any{
			"lifecycle_work_item_id": start.InitialWork.WorkItemID,
			"work_item_ids":          []string{start.InitialWork.WorkItemID},
			"doc_id":                 docID,
		},
		CreatedAt: now, UpdatedAt: now,
	}
	project := types.ReplaceLifecycleActivationRequest{
		OwnerID: ownerID, ComputerID: computerID, CommandID: "project-cell-author-" + suffix,
		TrajectoryID: trajectoryID, AgentID: textureAgentID, Run: caller,
	}
	project.CommandDigest, _ = store.ComputeReplaceLifecycleActivationDigest(project)
	if _, err := s.ReplaceLifecycleActivation(ctx, project); err != nil {
		t.Fatalf("project lifecycle activation: %v", err)
	}
	return caller
}

type fakeCaptureStore map[string]texturewrite.Capture

func (f fakeCaptureStore) CaptureBlocks(_ context.Context, _ string, handle string) (texturewrite.Capture, error) {
	if c, ok := f[handle]; ok {
		return c, nil
	}
	return texturewrite.Capture{}, texturewrite.ErrNotCaptured
}

var courtCapture = fakeCaptureStore{"https://example.org/court": {
	Doc: "cap-court", Version: "doc-hash-1", URL: "https://example.org/court", Title: "Court ruling",
	Blocks: []quotematch.Block{
		{ID: "cid-1", Text: "The court ruled that the merger may proceed."},
		{ID: "cid-2", Text: "The appeal was filed on Monday by the regulator."},
	},
}}

func cellAuthorSetup(t *testing.T, suffix string) (*Handler, *store.Store, types.RunRecord) {
	t.Helper()
	core, h := testAPISetup(t)
	core.SetTextureCaptures(courtCapture)
	s := core.Store()
	rec := seedCellAuthorLifecycle(t, s, "owner-cell-author", suffix, "# Merger\n\nOpening line.\n\nClosing line.")
	h.createAgentMutationForRun(context.Background(), &rec)
	return h, s, rec
}

func turnBody(t *testing.T, turn yaegikernel.TextureTurn) string {
	t.Helper()
	raw, err := json.Marshal(turn)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func headOf(t *testing.T, s *store.Store, rec types.RunRecord) types.Revision {
	t.Helper()
	doc, err := s.GetLifecycleDocument(context.Background(), rec.OwnerID, rec.ComputerID, rec.ChannelID)
	if err != nil {
		t.Fatalf("get document: %v", err)
	}
	rev, err := s.GetLifecycleRevision(context.Background(), rec.OwnerID, rec.ComputerID, doc.CurrentRevisionID)
	if err != nil {
		t.Fatalf("get head: %v", err)
	}
	return rev
}

func blocksByID(t *testing.T, rev types.Revision) map[string]string {
	t.Helper()
	var doc texturedoc.StructuredTextureDoc
	if err := json.Unmarshal(rev.BodyDoc, &doc); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	out := map[string]string{}
	for _, n := range doc.Doc.Content {
		raw, _ := json.Marshal(n)
		out[n.Attrs["id"].(string)] = string(raw)
	}
	return out
}

func TestWriteCommitsACitedParagraphAndKeepsUntouchedBlocks(t *testing.T) {
	h, s, rec := cellAuthorSetup(t, "write")
	ctx := context.Background()
	base := headOf(t, s, rec)
	first := turnBody(t, yaegikernel.TextureTurn{BaseRevisionID: base.RevisionID, Write: &yaegikernel.TextureWrite{
		Edits: "@replace b2\nThe court ruled that the merger may proceed {{cite https://example.org/court \"the court ruled that the merger may proceed\"}}.",
		Why:   "state the ruling",
	}})
	receipt, err := h.CommitCellTextureAuthor(ctx, &rec, first, "write-1")
	if err != nil {
		t.Fatalf("first Write: %v", err)
	}
	v1 := headOf(t, s, rec)
	if v1.RevisionID == base.RevisionID || v1.AuthorKind != types.AuthorAppAgent || !strings.Contains(receipt, v1.RevisionID) {
		t.Fatalf("first Write did not commit an appagent revision: %s", receipt)
	}
	if again, err := h.CommitCellTextureAuthor(ctx, &rec, first, "write-1"); err != nil || again != receipt {
		t.Fatalf("replay = %q, %v; want the first receipt", again, err)
	}

	second := turnBody(t, yaegikernel.TextureTurn{BaseRevisionID: v1.RevisionID, Write: &yaegikernel.TextureWrite{
		Edits: "@insert after b2\nThe regulator appealed {{cite https://example.org/court \"appeal was filed on Monday\"}}.",
		Why:   "add the appeal",
	}})
	if _, err := h.CommitCellTextureAuthor(ctx, &rec, second, "write-2"); err != nil {
		t.Fatalf("second Write: %v", err)
	}
	v2 := headOf(t, s, rec)
	before, after := blocksByID(t, v1), blocksByID(t, v2)
	if len(after) != len(before)+1 {
		t.Fatalf("blocks %d -> %d, want one more", len(before), len(after))
	}
	for id, raw := range before {
		if after[id] != raw {
			t.Fatalf("untouched block %s changed:\nv1 %s\nv2 %s", id, raw, after[id])
		}
	}
	var refs int
	for _, raw := range after {
		refs += strings.Count(raw, `"source_ref"`)
	}
	if refs != 2 || !strings.Contains(v2.Content, "The regulator appealed") {
		t.Fatalf("v2 citations = %d, content %q", refs, v2.Content)
	}
	var entities []texturedoc.SourceEntity
	if err := json.Unmarshal(v2.SourceEntities, &entities); err != nil || len(entities) != 1 || entities[0].Target.URI != "https://example.org/court" {
		t.Fatalf("source entities = %s", v2.SourceEntities)
	}
	if meta := decodeRevisionMetadata(v2.Metadata); metadataString(meta, "source") != "texture_cell" || metadataString(meta, "texture_write_why") != "add the appeal" {
		t.Fatalf("revision metadata = %s", v2.Metadata)
	}
}

func TestWriteRefusalsCommitNothing(t *testing.T) {
	h, s, rec := cellAuthorSetup(t, "refuse")
	ctx := context.Background()
	base := headOf(t, s, rec)
	uncaptured := turnBody(t, yaegikernel.TextureTurn{BaseRevisionID: base.RevisionID, Write: &yaegikernel.TextureWrite{
		Edits: "@insert end\nFirst.\n\nA claim {{cite https://example.org/snippet \"some words\"}}.", Why: "x",
	}})
	if _, err := h.CommitCellTextureAuthor(ctx, &rec, uncaptured, "refuse-1"); err == nil || !strings.Contains(err.Error(), "line 4") || !strings.Contains(err.Error(), "not captured") {
		t.Fatalf("uncaptured cite = %v, want a line-4 not-captured refusal", err)
	}
	split := turnBody(t, yaegikernel.TextureTurn{BaseRevisionID: base.RevisionID, Write: &yaegikernel.TextureWrite{
		Edits: "@insert end\nBoth {{cite https://example.org/court \"the merger may proceed ... filed on Monday\"}}.", Why: "x",
	}})
	if _, err := h.CommitCellTextureAuthor(ctx, &rec, split, "refuse-2"); err == nil || !strings.Contains(err.Error(), "two cites") {
		t.Fatalf("cross-block ellipsis = %v, want the split refusal", err)
	}
	stale := turnBody(t, yaegikernel.TextureTurn{BaseRevisionID: "an-older-revision", Write: &yaegikernel.TextureWrite{Edits: "@delete b3", Why: "x"}})
	if _, err := h.CommitCellTextureAuthor(ctx, &rec, stale, "refuse-3"); err == nil || !strings.Contains(err.Error(), "changed since your frame") {
		t.Fatalf("stale frame = %v, want the changed-since-your-frame refusal", err)
	}
	if head := headOf(t, s, rec); head.RevisionID != base.RevisionID {
		t.Fatalf("a refused Write moved the head to %s", head.RevisionID)
	}
}

func TestDoneAloneCompletesTheWorkAtTheCurrentRevision(t *testing.T) {
	h, s, rec := cellAuthorSetup(t, "done")
	ctx := context.Background()
	base := headOf(t, s, rec)
	if _, err := h.CommitCellTextureAuthor(ctx, &rec, turnBody(t, yaegikernel.TextureTurn{BaseRevisionID: base.RevisionID, Done: "the document already answers the request"}), "done-1"); err != nil {
		t.Fatalf("Done: %v", err)
	}
	snapshot, err := s.GetLifecycleSnapshot(ctx, rec.OwnerID, rec.ComputerID, rec.TrajectoryID)
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range snapshot.WorkItems {
		if w.WorkItemID == metadataStringValue(rec.Metadata, "lifecycle_work_item_id") && (w.Status != types.WorkItemCompleted || w.ResultRef != base.RevisionID) {
			t.Fatalf("work = %+v, want completed at %s", w, base.RevisionID)
		}
	}
	if head := headOf(t, s, rec); head.RevisionID != base.RevisionID {
		t.Fatalf("Done minted a revision")
	}

	// B5: a records-route run settles its work item only while it is open.
	// A later Done (or a Write with Done) on the completed item commits its
	// turn instead of failing on the closed work.
	rec.Metadata[textureRecordsRouteMetadata] = true
	if _, err := h.CommitCellTextureAuthor(ctx, &rec, turnBody(t, yaegikernel.TextureTurn{BaseRevisionID: base.RevisionID, Done: "still answered"}), "done-2"); err != nil {
		t.Fatalf("Done on a records-route run with closed work: %v", err)
	}
	write := turnBody(t, yaegikernel.TextureTurn{BaseRevisionID: base.RevisionID, Done: "answered with a fix",
		Write: &yaegikernel.TextureWrite{Edits: "@replace b3\nA corrected closing line.", Why: "correct the close"}})
	if _, err := h.CommitCellTextureAuthor(ctx, &rec, write, "done-3"); err != nil {
		t.Fatalf("Write and Done on a records-route run with closed work: %v", err)
	}
}

func TestWaitAndRejectCarryTheDispatchFoldAndTheCursor(t *testing.T) {
	h, s, rec := cellAuthorSetup(t, "wait")
	ctx := context.Background()
	base := headOf(t, s, rec)
	texture, research := rec.AgentID, agentprofile.Research+":"+rec.ChannelID
	cast := types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: "texture-cell:cast:tray-1", Kind: types.CommitmentKindDirective,
		Directive: &types.CommitmentDirective{Subtype: types.CommitmentDirectiveCast, Objective: "cover the ruling", Body: `{"kind":"question","summary":"ruling"}`},
		Cast:      &types.CommitmentCast{}, Inbox: research, SenderInbox: texture, Addressee: research, ComputerID: rec.ComputerID,
		EmitterKind: types.CommitmentEmitterAgent, Discrepancy: types.DiscrepancyUnresolved, Provenance: types.CommitmentProvenance{AgentID: texture},
	}
	wait := turnBody(t, yaegikernel.TextureTurn{BaseRevisionID: base.RevisionID, Wait: "asked research for the ruling", InboxRecords: []types.CommitmentRecord{cast}})
	if _, err := h.CommitCellTextureAuthor(ctx, &rec, wait, "wait-1"); err != nil {
		t.Fatalf("Wait with a Cast: %v", err)
	}
	if head, err := s.InboxHead(ctx, rec.OwnerID, rec.ComputerID, research); err != nil || head != 1 {
		t.Fatalf("research inbox head = %d, %v; want the Cast", head, err)
	}
	if _, err := s.GetAgentByScope(ctx, rec.OwnerID, rec.ComputerID, research); err != nil {
		t.Fatalf("the research desk was not minted with the Cast: %v", err)
	}

	report := types.CommitInboxRecordsRequest{OwnerID: rec.OwnerID, ComputerID: rec.ComputerID, CommandID: "seed-report", Records: []types.CommitmentRecord{{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: "research-cell:report:tray-1", Kind: types.CommitmentKindReport,
		Report: &types.CommitmentReport{Claims: []types.ReportClaim{{Text: "off topic"}}}, Inbox: texture, SenderInbox: research, Addressee: texture,
		ComputerID: rec.ComputerID, EmitterKind: types.CommitmentEmitterAgent, Discrepancy: types.DiscrepancyUnresolved, Provenance: types.CommitmentProvenance{AgentID: research},
	}}}
	report.CommandDigest, _ = store.ComputeCommitInboxRecordsDigest(report)
	if _, err := s.CommitInboxRecords(ctx, report); err != nil {
		t.Fatalf("seed report: %v", err)
	}
	reject := turnBody(t, yaegikernel.TextureTurn{BaseRevisionID: base.RevisionID,
		Rejects:     []yaegikernel.TextureReject{{RecordID: "research-cell:report:tray-1", Why: "it does not bear on the ruling"}},
		InboxCursor: &types.InboxCursorAdvance{ReaderAgentID: texture, Inbox: texture, ExpectedSeq: 0, Seq: 1}})
	if _, err := h.CommitCellTextureAuthor(ctx, &rec, reject, "reject-1"); err != nil {
		t.Fatalf("Reject: %v", err)
	}
	if cursor, err := s.ReaderCursor(ctx, rec.OwnerID, rec.ComputerID, texture, texture); err != nil || cursor != 1 {
		t.Fatalf("texture cursor = %d, %v; want 1", cursor, err)
	}
	snapshot, err := s.GetLifecycleSnapshot(ctx, rec.OwnerID, rec.ComputerID, rec.TrajectoryID)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, ev := range snapshot.Events {
		if ev.Kind == types.LifecycleTextureTurnCommitted && strings.Contains(ev.Reason, "rejected research-cell:report:tray-1: it does not bear on the ruling") {
			found = true
		}
	}
	if !found {
		t.Fatal("the rejection reason was not committed with the turn")
	}
	if head := headOf(t, s, rec); head.RevisionID != base.RevisionID {
		t.Fatal("Wait or Reject minted a revision")
	}
}
