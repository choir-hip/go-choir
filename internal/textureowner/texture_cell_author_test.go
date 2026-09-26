package textureowner

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// seedCellAuthorLifecycle starts a bound Texture document lifecycle (doc +
// initial user revision + texture agent + open work item) and projects the
// texture caller run onto it, then creates the pending agent mutation the
// activation opens. It returns the caller run record the cell commits under.
func seedCellAuthorLifecycle(t *testing.T, s *store.Store, ownerID, suffix string) types.RunRecord {
	t.Helper()
	ctx := context.Background()
	computerID := "autoputer-cell-author"
	docID := "doc-cell-author-" + suffix
	trajectoryID := "trajectory-cell-author-" + suffix
	textureAgentID := agentprofile.Texture + ":" + docID
	now := time.Now().UTC()
	start := types.StartLifecycleRequest{
		OwnerID: ownerID, ComputerID: computerID, CommandID: "start-cell-author-" + suffix,
		TrajectoryID: trajectoryID, Kind: types.TrajectoryKindDocument,
		SubjectRefs:    map[string]string{"artifact": "texture://documents/" + docID, "doc_id": docID},
		SettlementRule: types.SettlementRule{Version: types.LifecycleReducerVersion, RequireNoOpenWorkItems: true, RequiredSubjectRefs: []string{"artifact"}},
		InitialWork:    types.WorkItemRecord{WorkItemID: "work-cell-author-" + suffix, Objective: "author the supervision doc", AssignedAgentID: textureAgentID, AuthorityProfile: agentprofile.Texture},
		InitialDocument: types.Document{DocID: docID, OwnerID: ownerID, ComputerID: computerID, TrajectoryID: trajectoryID, Title: "Cell author " + suffix, CreatedAt: now, UpdatedAt: now},
		InitialRevision: types.Revision{RevisionID: "revision-cell-author-base-" + suffix, DocID: docID, OwnerID: ownerID, ComputerID: computerID, TrajectoryID: trajectoryID, AuthorKind: types.AuthorUser, AuthorLabel: ownerID, Content: "initial head content", CreatedAt: now},
		Agent:           types.AgentRecord{AgentID: textureAgentID, OwnerID: ownerID, ComputerID: computerID, Profile: agentprofile.Texture, Role: agentprofile.Texture, ChannelID: docID, CreatedAt: now, UpdatedAt: now},
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

// TestCommitCellTextureApplyCommitsAuthorRevision proves the R3d acceptance
// assertion: a staged choir.ApplyTexture apply body commits through
// ApplyTextureTurn as an AuthorAppAgent revision that advances the doc head,
// its metadata carries the texture_cell source — and no worker_updates legs.
// A replayed commit replays rather than minting a second head.
func TestCommitCellTextureApplyCommitsAuthorRevision(t *testing.T) {
	s, err := store.Open(t.TempDir() + "/cell-author.db")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()
	ownerID := "owner-cell-author"
	rec := seedCellAuthorLifecycle(t, s, ownerID, "one")

	h := &Handler{Store: s, Bus: events.NewEventBus()}
	h.createAgentMutationForRun(ctx, &rec)

	body := `{"op":"apply","content":"# Revised\n\ncell-authored head","rationale":"incorporate ledger finding"}`
	receipt, err := h.CommitCellTextureAuthor(ctx, &rec, body, "cell-author-idem-1")
	if err != nil {
		t.Fatalf("CommitCellTextureAuthor: %v", err)
	}
	var out struct {
		Op         string `json:"op"`
		DocID      string `json:"doc_id"`
		RevisionID string `json:"revision_id"`
	}
	if err := json.Unmarshal([]byte(receipt), &out); err != nil {
		t.Fatalf("decode receipt: %v", err)
	}
	if out.Op != "apply" || out.RevisionID == "" || out.DocID != rec.ChannelID {
		t.Fatalf("unexpected receipt %q", receipt)
	}

	doc, err := s.GetLifecycleDocument(ctx, ownerID, rec.ComputerID, rec.ChannelID)
	if err != nil {
		t.Fatalf("get lifecycle document: %v", err)
	}
	if doc.CurrentRevisionID != out.RevisionID {
		t.Fatalf("doc head did not advance: head=%s committed=%s", doc.CurrentRevisionID, out.RevisionID)
	}
	rev, err := s.GetLifecycleRevision(ctx, ownerID, rec.ComputerID, out.RevisionID)
	if err != nil {
		t.Fatalf("get committed revision: %v", err)
	}
	if rev.AuthorKind != types.AuthorAppAgent {
		t.Fatalf("cell-authored revision must be AuthorAppAgent, got %s", rev.AuthorKind)
	}
	meta := decodeRevisionMetadata(rev.Metadata)
	if metadataString(meta, "source") != "texture_cell" {
		t.Fatalf("revision source=%v, want texture_cell: %s", meta["source"], rev.Metadata)
	}
	for key := range meta {
		if strings.HasPrefix(key, "worker_updates") {
			t.Fatalf("revision metadata carries retired worker_updates leg %q", key)
		}
	}
	mutation, err := s.GetAgentMutationByRun(ctx, ownerID, rec.ComputerID, rec.RunID)
	if err != nil {
		t.Fatalf("get mutation: %v", err)
	}
	if mutation == nil || mutation.RevisionID != out.RevisionID {
		t.Fatalf("mutation did not record the committed revision: %+v vs %s", mutation, out.RevisionID)
	}

	// Replay: same staged body + same revision identity replays the commit —
	// no second head.
	receipt2, err := h.CommitCellTextureAuthor(ctx, &rec, body, "cell-author-idem-1")
	if err != nil {
		t.Fatalf("replayed CommitCellTextureAuthor: %v", err)
	}
	var out2 struct {
		RevisionID string `json:"revision_id"`
	}
	_ = json.Unmarshal([]byte(receipt2), &out2)
	if out2.RevisionID != out.RevisionID {
		t.Fatalf("replay minted a second head: %s vs %s", out2.RevisionID, out.RevisionID)
	}
	doc2, err := s.GetLifecycleDocument(ctx, ownerID, rec.ComputerID, rec.ChannelID)
	if err != nil {
		t.Fatalf("get lifecycle document after replay: %v", err)
	}
	if doc2.CurrentRevisionID != out.RevisionID {
		t.Fatalf("replay advanced head: %s", doc2.CurrentRevisionID)
	}
}

// TestCommitCellTextureDecideRecordsTurn covers the decide op: a non-revision
// lifecycle decision turn (record_texture_decision successor) commits through
// the same atomic path without advancing the doc head.
func TestCommitCellTextureDecideRecordsTurn(t *testing.T) {
	s, err := store.Open(t.TempDir() + "/cell-decide.db")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()
	ownerID := "owner-cell-author"
	rec := seedCellAuthorLifecycle(t, s, ownerID, "decide")

	h := &Handler{Store: s, Bus: events.NewEventBus()}
	h.createAgentMutationForRun(ctx, &rec)

	docBefore, err := s.GetLifecycleDocument(ctx, ownerID, rec.ComputerID, rec.ChannelID)
	if err != nil {
		t.Fatalf("get lifecycle document: %v", err)
	}
	receipt, err := h.CommitCellTextureAuthor(ctx, &rec,
		`{"op":"decide","decision_kind":"wait_for_evidence","reason":"no ledger evidence yet","next_action":"await child report","base_revision_id":"`+docBefore.CurrentRevisionID+`"}`,
		"cell-author-idem-2")
	if err != nil {
		t.Fatalf("CommitCellTextureAuthor decide: %v", err)
	}
	var out struct {
		Op          string `json:"op"`
		Outcome     string `json:"outcome"`
		HeadRevID   string `json:"head_revision_id"`
		DecisionKind string `json:"decision_kind"`
	}
	if err := json.Unmarshal([]byte(receipt), &out); err != nil {
		t.Fatalf("decode decide receipt: %v", err)
	}
	if out.Op != "decide" || out.DecisionKind != "wait_for_evidence" {
		t.Fatalf("unexpected decide receipt %q", receipt)
	}
	docAfter, err := s.GetLifecycleDocument(ctx, ownerID, rec.ComputerID, rec.ChannelID)
	if err != nil {
		t.Fatalf("get lifecycle document after decide: %v", err)
	}
	if docAfter.CurrentRevisionID != docBefore.CurrentRevisionID {
		t.Fatalf("decide op advanced doc head: %s -> %s", docBefore.CurrentRevisionID, docAfter.CurrentRevisionID)
	}
}
