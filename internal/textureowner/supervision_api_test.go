package textureowner

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestTextureSupervisionImportRouteRequiresOwnerAndTrajectory(t *testing.T) {
	_, handler := testAPISetup(t)

	unauthorized := httptest.NewRequest(http.MethodPost, "/api/texture/supervision/import", nil)
	unauthorizedResult := httptest.NewRecorder()
	handler.HandleTextureRouter(unauthorizedResult, unauthorized)
	if unauthorizedResult.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized import status = %d body=%s", unauthorizedResult.Code, unauthorizedResult.Body.String())
	}

	missingTrajectory := textureRequest(t, http.MethodPost, "/api/texture/supervision/import", textureSupervisionImportRequest{})
	missingTrajectoryResult := httptest.NewRecorder()
	handler.HandleTextureRouter(missingTrajectoryResult, missingTrajectory)
	if missingTrajectoryResult.Code != http.StatusBadRequest {
		t.Fatalf("missing trajectory import status = %d body=%s", missingTrajectoryResult.Code, missingTrajectoryResult.Body.String())
	}
}

func TestTextureSupervisionImportDerivesStableCommandID(t *testing.T) {
	first := textureSupervisionImportCommandID("owner-a", "trajectory-a")
	if first == "" || first != textureSupervisionImportCommandID("owner-a", "trajectory-a") {
		t.Fatalf("derived import command must be stable: %q", first)
	}
	if first == textureSupervisionImportCommandID("owner-b", "trajectory-a") {
		t.Fatal("derived import command must remain owner scoped")
	}
}

func TestTextureSupervisionRebuildRouteRequiresOwner(t *testing.T) {
	_, handler := testAPISetup(t)
	request := httptest.NewRequest(http.MethodPost, "/api/texture/supervision/rebuild", nil)
	response := httptest.NewRecorder()
	handler.HandleTextureRouter(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized rebuild status = %d body=%s", response.Code, response.Body.String())
	}
}

func TestTextureSupervisionRebuildRejectsOrdinaryAuthenticatedUser(t *testing.T) {
	_, handler := testAPISetup(t)
	called := 0
	handler.rebuildSupervisionProjection = func(context.Context) error {
		called++
		return nil
	}
	request := authenticatedRequest(http.MethodPost, "/api/texture/supervision/rebuild", "", "user-ordinary")
	response := httptest.NewRecorder()
	handler.HandleTextureRouter(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("ordinary rebuild status = %d body=%s", response.Code, response.Body.String())
	}
	if called != 0 {
		t.Fatalf("ordinary rebuild invoked authority %d times, want 0", called)
	}
}

func TestTextureSupervisionRebuildAllowsTrustedInternalCaller(t *testing.T) {
	_, handler := testAPISetup(t)
	called := 0
	handler.rebuildSupervisionProjection = func(context.Context) error {
		called++
		return nil
	}
	request := authenticatedRequest(http.MethodPost, "/api/texture/supervision/rebuild", "", "user-operator")
	request.Header.Set("X-Internal-Caller", "true")
	response := httptest.NewRecorder()
	handler.HandleTextureRouter(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("trusted rebuild status = %d body=%s", response.Code, response.Body.String())
	}
	if called != 1 {
		t.Fatalf("trusted rebuild invoked authority %d times, want 1", called)
	}
}

func TestTextureColdWakeConsumesCanonicalSupervisionDelivery(t *testing.T) {
	core, handler := testAPISetup(t)
	ctx := context.Background()
	ownerID := "user-canonical-texture-wake"
	docID := "doc-canonical-texture-wake"
	trajectoryID := "trajectory-canonical-texture-wake"
	textureAgentID := currentTextureAgentID(docID)
	now := time.Now().UTC()
	start := types.StartLifecycleRequest{
		OwnerID: ownerID, ComputerID: core.TextureSandboxID(), CommandID: "start-canonical-texture-wake",
		TrajectoryID: trajectoryID, Kind: types.TrajectoryKindDocument,
		SettlementRule: types.SettlementRule{
			Version: types.LifecycleReducerVersion, RequireNoOpenWorkItems: true,
			RequiredSubjectRefs: []string{"artifact"},
		},
		SubjectRefs: map[string]string{"artifact": "texture://documents/" + docID},
		InitialWork: types.WorkItemRecord{
			WorkItemID: "work-canonical-texture-wake", Objective: "Integrate the supervised result.",
			AssignedAgentID: textureAgentID,
		},
		InitialDocument: types.Document{DocID: docID, Title: "Canonical wake"},
		InitialRevision: types.Revision{
			RevisionID: "revision-canonical-texture-wake", AuthorKind: types.AuthorUser,
			AuthorLabel: ownerID, Content: "Initial content",
		},
		Agent: types.AgentRecord{
			AgentID: textureAgentID, OwnerID: ownerID, ComputerID: core.TextureSandboxID(),
			SandboxID: core.TextureSandboxID(), Profile: agentprofile.Texture, Role: agentprofile.Texture,
			ChannelID: docID, CreatedAt: now, UpdatedAt: now,
		},
	}
	start.StartRequestDigest, _ = store.ComputeStartLifecycleRequestDigest(start)
	if _, err := handler.startSupervisionTrajectory(ctx, start); err != nil {
		t.Fatalf("start canonical Texture trajectory: %v", err)
	}
	superRun := &types.RunRecord{
		RunID: "run-canonical-texture-wake", OwnerID: ownerID, SandboxID: core.TextureSandboxID(),
		AgentID: "super:" + ownerID, ChannelID: docID, State: types.RunRunning,
		Metadata: map[string]any{
			runMetadataAgentProfile: agentprofile.Super,
			runMetadataAgentRole:    agentprofile.Super,
			"trajectory_id":         trajectoryID,
			runMetadataChannelID:    docID,
		},
	}
	raw := json.RawMessage(`{
		"agent_id":"texture:doc-canonical-texture-wake",
		"producer_update_id":"4f80bd1d-0557-4e3c-a594-75bbda6b72e6",
		"schema_version":"coagent_source_packet.v1",
		"kind":"execution_result",
		"summary":"The supervised command completed.",
		"sources":[{
			"source_id":"src-result",
			"kind":"command_output",
			"target":{"uri":"command_output:canonical-wake","title":"Canonical wake result"}
		}]
	}`)
	execCtx := toolregistry.WithExecutionContext(ctx, toolExecutionContextForRun(superRun))
	if _, err := core.ToolRegistryForProfile(agentprofile.Super).Execute(execCtx, "update_coagent", raw); err != nil {
		t.Fatalf("append canonical Super result: %v", err)
	}
	updates, err := core.ListPendingCanonicalSupervisionUpdates(ctx, ownerID, core.TextureSandboxID(), textureAgentID, trajectoryID, 10)
	if err != nil || len(updates) != 1 {
		t.Fatalf("canonical Texture updates = %+v err=%v, want one", updates, err)
	}
	rec, err := handler.ReconcileAgentWake(ctx, ownerID, docID)
	if err != nil {
		t.Fatalf("reconcile canonical Texture wake: %v", err)
	}
	if rec == nil || rec.AgentID != textureAgentID || rec.TrajectoryID != trajectoryID {
		t.Fatalf("canonical Texture wake run = %+v", rec)
	}
	if err := handler.ValidateActivationAuthority(ctx, ownerID, core.TextureSandboxID(), textureAgentID, rec.RunID); err != nil {
		t.Fatalf("validate canonical Texture wake authority: %v", err)
	}
	assertExecutionEntity(t, decodeAvailableTextureSourceEntities(rec.Metadata), "command_output", "canonical-wake", "")
	if ids := metadataStringSlice(rec.Metadata["supervision_delivery_ids"]); len(ids) != 1 {
		t.Fatalf("canonical Texture run delivery binding = %#v", rec.Metadata)
	}
	storedRevision, err := handler.commitTextureToolEdit(ctx, rec, editTextureArgs{
		DocID: docID, BaseRevisionID: start.InitialRevision.RevisionID,
		Operation: "replace_all", Content: "Initial content\n\nThe supervised command completed.",
		UpdateDispositions: []textureUpdateDisposition{{
			UpdateID:    metadataStringSlice(rec.Metadata["supervision_delivery_ids"])[0],
			Disposition: "incorporated",
		}},
	})
	if err != nil {
		t.Fatalf("commit canonical Texture delivery revision: %v", err)
	}
	if metadataStringValue(rec.Metadata, "canonical_texture_revision_id") != storedRevision.RevisionID {
		t.Fatalf("canonical Texture revision binding = %#v, revision=%s", rec.Metadata, storedRevision.RevisionID)
	}
	if mutation, err := handler.Store.GetAgentMutationByRun(ctx, ownerID, core.TextureSandboxID(), rec.RunID); err != nil || mutation != nil {
		t.Fatalf("canonical Texture delivery created legacy mutation authority: mutation=%+v err=%v", mutation, err)
	}
	snapshot, err := handler.Store.GetSupervisionProjectionSnapshot(ctx, ownerID, core.TextureSandboxID(), trajectoryID)
	if err != nil || snapshot.ArtifactHeadRevisionID != storedRevision.RevisionID {
		t.Fatalf("canonical Texture projection head=%q want=%q err=%v", snapshot.ArtifactHeadRevisionID, storedRevision.RevisionID, err)
	}
}
