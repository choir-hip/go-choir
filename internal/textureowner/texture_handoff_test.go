package textureowner

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestEnsureTextureHandoffCorpusWakeRequiresChannelID(t *testing.T) {
	h := &Handler{}
	reconcilerRun := &types.RunRecord{
		OwnerID:      "user-alice",
		AgentProfile: agentprofile.Reconciler,
		AgentRole:    agentprofile.Reconciler,
	}

	_, err := h.EnsureTextureHandoff(context.Background(), reconcilerRun, HandoffRequest{
		Kind:          HandoffKindCorpusWake,
		CallerProfile: agentprofile.Reconciler,
		Objective:     "draft a corpus-wide correction without a target doc",
	})
	if err == nil {
		t.Fatal("corpus_wake handoff without channel_id should fail")
	}
}

func TestOwnerRevisionUsesOwnerActorAndRefusesStaleHead(t *testing.T) {
	core, handler := testAPISetup(t)
	now := time.Now().UTC()
	const (
		ownerID      = "user-1"
		docID        = "document-owner-actor"
		trajectoryID = "trajectory-owner-actor"
		revisionID   = "revision-owner-actor-v0"
	)
	start := types.StartLifecycleRequest{
		OwnerID: ownerID, ComputerID: core.TextureSandboxID(), CommandID: "start-owner-actor", TrajectoryID: trajectoryID,
		Kind:           types.TrajectoryKindDocument,
		SubjectRefs:    map[string]string{"artifact": "texture://documents/" + docID, "doc_id": docID},
		SettlementRule: types.SettlementRule{Version: types.LifecycleReducerVersion, RequireNoOpenWorkItems: true, RequiredSubjectRefs: []string{"artifact"}},
		InitialWork: types.WorkItemRecord{
			WorkItemID: "work-owner-actor", Objective: "revise document", AssignedAgentID: currentTextureAgentID(docID),
			AuthorityProfile: agentprofile.Texture,
		},
		InitialDocument: types.Document{DocID: docID, OwnerID: ownerID, ComputerID: core.TextureSandboxID(), TrajectoryID: trajectoryID, Title: "Owner actor", CreatedAt: now, UpdatedAt: now},
		InitialRevision: types.Revision{RevisionID: revisionID, DocID: docID, OwnerID: ownerID, ComputerID: core.TextureSandboxID(), TrajectoryID: trajectoryID, AuthorKind: types.AuthorUser, AuthorLabel: ownerID, Content: "Initial", CreatedAt: now},
		Agent:           types.AgentRecord{AgentID: currentTextureAgentID(docID), OwnerID: ownerID, ComputerID: core.TextureSandboxID(), SandboxID: core.TextureSandboxID(), Profile: agentprofile.Texture, Role: agentprofile.Texture, ChannelID: docID, CreatedAt: now, UpdatedAt: now},
	}
	start.StartRequestDigest, _ = store.ComputeStartLifecycleRequestDigest(start)
	if _, err := handler.startSupervisionTrajectory(t.Context(), start); err != nil {
		t.Fatalf("start supervision trajectory: %v", err)
	}
	activeDelete := httptest.NewRecorder()
	handler.handleTextureDeleteDocument(activeDelete, textureRequest(t, http.MethodDelete, "/api/texture/documents/"+docID, nil), docID)
	if activeDelete.Code != http.StatusConflict {
		t.Fatalf("unsettled delete status = %d body=%s", activeDelete.Code, activeDelete.Body.String())
	}

	save := textureRequest(t, http.MethodPost, "/api/texture/documents/"+docID+"/revisions", textureCreateRevisionRequest{
		Content: "Owner-authored", ParentRevisionID: revisionID, IdempotencyKey: "owner-actor-save", ExpectedLifecycleVersion: 1,
	})
	saveResponse := httptest.NewRecorder()
	handler.HandleTextureRevisions(saveResponse, save)
	if saveResponse.Code != http.StatusCreated {
		t.Fatalf("owner save status = %d body=%s", saveResponse.Code, saveResponse.Body.String())
	}
	snapshot, err := handler.Store.GetSupervisionProjectionSnapshot(t.Context(), ownerID, core.TextureSandboxID(), trajectoryID)
	if err != nil {
		t.Fatalf("load supervision snapshot: %v", err)
	}
	headRevision, err := handler.Store.GetLifecycleRevision(t.Context(), ownerID, core.TextureSandboxID(), snapshot.ArtifactHeadRevisionID)
	if err != nil {
		t.Fatalf("load canonical head revision: %v", err)
	}
	if headRevision.AuthorKind != types.AuthorUser || headRevision.AuthorLabel != ownerID {
		t.Fatalf("owner revision attribution = %s/%q, want user/%q", headRevision.AuthorKind, headRevision.AuthorLabel, ownerID)
	}

	stale := textureRequest(t, http.MethodPost, "/api/texture/documents/"+docID+"/revisions", textureCreateRevisionRequest{
		Content: "Stale write", ParentRevisionID: revisionID, IdempotencyKey: "owner-actor-stale", ExpectedLifecycleVersion: 1,
	})
	staleResponse := httptest.NewRecorder()
	handler.HandleTextureRevisions(staleResponse, stale)
	if staleResponse.Code != http.StatusConflict {
		t.Fatalf("stale owner save status = %d body=%s", staleResponse.Code, staleResponse.Body.String())
	}
}

func TestTextureToolAppendsCanonicalRevisionAndRefusesUnimportedLegacyDocument(t *testing.T) {
	core, handler := testAPISetup(t)
	now := time.Now().UTC()
	const (
		ownerID      = "user-1"
		docID        = "document-tool-actor"
		trajectoryID = "trajectory-tool-actor"
		baseID       = "revision-tool-actor-v0"
		workID       = "work-tool-actor"
		runID        = "run-tool-actor"
	)
	agentID := currentTextureAgentID(docID)
	start := types.StartLifecycleRequest{
		OwnerID: ownerID, ComputerID: core.TextureSandboxID(), CommandID: "start-tool-actor", TrajectoryID: trajectoryID,
		Kind:            types.TrajectoryKindDocument,
		SubjectRefs:     map[string]string{"artifact": "texture://documents/" + docID, "doc_id": docID},
		SettlementRule:  types.SettlementRule{Version: types.LifecycleReducerVersion, RequireNoOpenWorkItems: true, RequiredSubjectRefs: []string{"artifact"}},
		InitialWork:     types.WorkItemRecord{WorkItemID: workID, Objective: "revise document", AssignedAgentID: agentID, AuthorityProfile: agentprofile.Texture},
		InitialDocument: types.Document{DocID: docID, OwnerID: ownerID, ComputerID: core.TextureSandboxID(), TrajectoryID: trajectoryID, Title: "Tool actor", CreatedAt: now, UpdatedAt: now},
		InitialRevision: types.Revision{RevisionID: baseID, DocID: docID, OwnerID: ownerID, ComputerID: core.TextureSandboxID(), TrajectoryID: trajectoryID, AuthorKind: types.AuthorUser, AuthorLabel: ownerID, Content: "Initial", CreatedAt: now},
		Agent:           types.AgentRecord{AgentID: agentID, OwnerID: ownerID, ComputerID: core.TextureSandboxID(), SandboxID: core.TextureSandboxID(), Profile: agentprofile.Texture, Role: agentprofile.Texture, ChannelID: docID, CreatedAt: now, UpdatedAt: now},
	}
	start.StartRequestDigest, _ = store.ComputeStartLifecycleRequestDigest(start)
	if _, err := handler.startSupervisionTrajectory(t.Context(), start); err != nil {
		t.Fatalf("start supervision trajectory: %v", err)
	}
	if err := handler.Store.CreateAgentMutation(t.Context(), store.AgentMutation{
		DocID: docID, RunID: runID, OwnerID: ownerID, ComputerID: core.TextureSandboxID(), State: "pending", CreatedAt: now,
	}); err != nil {
		t.Fatalf("create agent mutation: %v", err)
	}
	run := &types.RunRecord{
		RunID: runID, AgentID: agentID, OwnerID: ownerID, SandboxID: core.TextureSandboxID(), ChannelID: docID, TrajectoryID: trajectoryID,
		State: types.RunRunning, AgentProfile: agentprofile.Texture, AgentRole: agentprofile.Texture,
		Metadata: map[string]any{"doc_id": docID, "trajectory_id": trajectoryID, "lifecycle_work_item_id": workID},
	}
	stored, err := handler.commitTextureToolEdit(t.Context(), run, editTextureArgs{
		DocID: docID, BaseRevisionID: baseID, Content: "Texture-authored", Operation: "replace_all",
	})
	if err != nil {
		t.Fatalf("commit Texture tool edit: %v", err)
	}
	snapshot, err := handler.Store.GetSupervisionProjectionSnapshot(t.Context(), ownerID, core.TextureSandboxID(), trajectoryID)
	if err != nil {
		t.Fatalf("load supervision snapshot: %v", err)
	}
	headRevision, err := handler.Store.GetLifecycleRevision(t.Context(), ownerID, core.TextureSandboxID(), snapshot.ArtifactHeadRevisionID)
	if err != nil {
		t.Fatalf("load canonical head revision: %v", err)
	}
	if headRevision.RevisionID != stored.RevisionID || headRevision.AuthorKind != types.AuthorAppAgent || headRevision.AuthorLabel != agentID {
		t.Fatalf("Texture tool revision projection = %+v, want Texture-authored canonical head", headRevision)
	}
	if _, err := handler.Store.GetRevision(t.Context(), stored.RevisionID, ownerID); !errors.Is(err, store.ErrLifecycleAuthorityRequired) {
		t.Fatalf("tool edit was exposed as a legacy revision: %v", err)
	}
	legacyDoc := types.Document{DocID: "document-tool-legacy", OwnerID: ownerID, Title: "Legacy", CreatedAt: now, UpdatedAt: now}
	if err := handler.Store.CreateDocument(t.Context(), legacyDoc); err != nil {
		t.Fatalf("create legacy document: %v", err)
	}
	legacyBase := types.Revision{RevisionID: "revision-tool-legacy-v0", DocID: legacyDoc.DocID, OwnerID: ownerID, AuthorKind: types.AuthorUser, AuthorLabel: ownerID, Content: "Initial", CreatedAt: now}
	if err := handler.Store.CreateRevision(t.Context(), legacyBase); err != nil {
		t.Fatalf("create legacy revision: %v", err)
	}
	if err := handler.Store.CreateAgentMutation(t.Context(), store.AgentMutation{
		DocID: legacyDoc.DocID, RunID: "run-tool-legacy", OwnerID: ownerID, ComputerID: core.TextureSandboxID(), State: "pending", CreatedAt: now,
	}); err != nil {
		t.Fatalf("create legacy mutation: %v", err)
	}
	_, err = handler.commitTextureToolEdit(t.Context(), &types.RunRecord{
		RunID: "run-tool-legacy", AgentID: currentTextureAgentID(legacyDoc.DocID), OwnerID: ownerID, SandboxID: core.TextureSandboxID(), ChannelID: legacyDoc.DocID,
		State: types.RunRunning, AgentProfile: agentprofile.Texture, AgentRole: agentprofile.Texture,
	}, editTextureArgs{DocID: legacyDoc.DocID, BaseRevisionID: legacyBase.RevisionID, Content: "Must refuse", Operation: "replace_all"})
	if err == nil {
		t.Fatal("unimported legacy document accepted a Texture tool edit")
	}
}

func TestTextureDeleteRefusesUnimportedLegacyDocument(t *testing.T) {
	_, handler := testAPISetup(t)
	now := time.Now().UTC()
	doc := types.Document{DocID: "document-delete-legacy", OwnerID: "user-1", Title: "Legacy", CreatedAt: now, UpdatedAt: now}
	if err := handler.Store.CreateDocument(t.Context(), doc); err != nil {
		t.Fatalf("create legacy document: %v", err)
	}
	response := httptest.NewRecorder()
	handler.handleTextureDeleteDocument(response, textureRequest(t, http.MethodDelete, "/api/texture/documents/"+doc.DocID, nil), doc.DocID)
	if response.Code != http.StatusConflict {
		t.Fatalf("legacy delete status = %d body=%s", response.Code, response.Body.String())
	}
	if _, err := handler.Store.GetDocument(t.Context(), doc.DocID, doc.OwnerID); err != nil {
		t.Fatalf("legacy document was deleted despite refusal: %v", err)
	}
}

type capturedTextureArchiveAppender struct {
	transaction computerevent.SupervisionTransaction
	payloads    []computerevent.PrivateSupervisionArtifactPayload
}

func (a *capturedTextureArchiveAppender) AppendSupervisionTransactionWithPrivateArtifacts(_ context.Context, transaction computerevent.SupervisionTransaction, payloads []computerevent.PrivateSupervisionArtifactPayload) (computerevent.Receipt, string, []computerevent.PrivateSupervisionArtifact, error) {
	a.transaction = transaction
	a.payloads = payloads
	return computerevent.Receipt{}, "", nil, nil
}

func TestAppendTextureArchiveUsesReservationBoundReasonPayload(t *testing.T) {
	const (
		ownerID      = "user-1"
		documentID   = "document-archive"
		trajectoryID = "trajectory-archive"
		headID       = "revision-archive"
	)
	appender := &capturedTextureArchiveAppender{}
	doc := types.Document{DocID: documentID, OwnerID: ownerID, ComputerID: "sandbox-test", TrajectoryID: trajectoryID, CurrentRevisionID: headID}
	snapshot := types.SupervisionProjectionSnapshot{
		OwnerID: ownerID, ComputerID: doc.ComputerID, TrajectoryID: trajectoryID, CanonicalEventHead: computerevent.DigestBytes([]byte("archive-head")),
		LifecycleVersion: 7, IntentRevisionID: "intent-archive", ArtifactHeadRevisionID: headID, Settled: true,
	}
	if err := appendTextureArchive(t.Context(), appender, doc, snapshot, ownerID); err != nil {
		t.Fatalf("append archive: %v", err)
	}
	if appender.transaction.TransactionClass != "archive_artifact" || appender.transaction.Actor.ActorID != ownerID ||
		appender.transaction.Actor.Role != "owner" || appender.transaction.Expected.CanonicalEventHead == nil ||
		*appender.transaction.Expected.CanonicalEventHead != snapshot.CanonicalEventHead {
		t.Fatalf("archive transaction authority/base = %+v", appender.transaction)
	}
	if len(appender.payloads) != 1 || appender.payloads[0].BindingID != "texture-archive-reason:"+trajectoryID+":"+headID ||
		len(appender.payloads[0].Plaintext) == 0 {
		t.Fatalf("archive private payload was not passed to reservation-first append: %+v", appender.payloads)
	}
	var body map[string]string
	if err := json.Unmarshal(appender.transaction.Mutations[0].Body, &body); err != nil {
		t.Fatalf("decode archive mutation: %v", err)
	}
	if body["artifact_id"] != documentID || body["head_revision_id"] != headID ||
		body["reason_artifact_ref"] != computerevent.SupervisionArtifactPlaceholder(appender.payloads[0].BindingID) {
		t.Fatalf("archive mutation = %#v", body)
	}
	var reason map[string]string
	if err := json.Unmarshal(appender.payloads[0].Plaintext, &reason); err != nil {
		t.Fatalf("decode archive reason: %v", err)
	}
	if reason["owner_id"] != ownerID || reason["document_id"] != documentID || reason["reason"] != "owner_requested_delete" {
		t.Fatalf("archive reason = %#v", reason)
	}
}
