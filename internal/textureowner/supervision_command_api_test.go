package textureowner

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestTextureSupervisionCommandRequiresAuthenticationAndRejectsAuthorityInjection(t *testing.T) {
	_, handler := testAPISetup(t)

	unauthenticated := httptest.NewRequest(http.MethodPost, "/api/texture/supervision/command", nil)
	unauthenticatedResult := httptest.NewRecorder()
	handler.HandleTextureRouter(unauthenticatedResult, unauthenticated)
	if unauthenticatedResult.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status = %d body=%s", unauthenticatedResult.Code, unauthenticatedResult.Body.String())
	}

	for _, body := range []string{
		`{"trajectory_id":"trajectory-1","actor":{"actor_id":"other"}}`,
		`{"trajectory_id":"trajectory-1","owner_id":"other"}`,
	} {
		injected := authenticatedRequest(http.MethodPost, "/api/texture/supervision/command", body, "user-1")
		injectedResult := httptest.NewRecorder()
		handler.HandleTextureRouter(injectedResult, injected)
		if injectedResult.Code != http.StatusBadRequest {
			t.Fatalf("authority injection status = %d body=%s", injectedResult.Code, injectedResult.Body.String())
		}
	}
}

func TestTextureSupervisionCommandPinsLogicalOwnerDecisionAndRecoversRetryArtifacts(t *testing.T) {
	handler, ownerID, trajectoryID, intentID, expected := setupOwnerCommandTrajectory(t)
	request := ownerDecisionCommandRequest(t, handler, ownerID, trajectoryID, intentID, expected, "owner-command-1", "owner-decision-1")

	first := appendOwnerCommand(t, handler, request)
	if first.ArtifactDigest == "" || first.Receipt.ReceiptID == "" || len(first.PrivateArtifacts) != 1 {
		t.Fatalf("canonical append result = %+v", first)
	}
	if first.PrivateArtifacts[0].BindingID != request.PrivateArtifacts[0].BindingID || first.PrivateArtifacts[0].Ref.String() == "" {
		t.Fatalf("pinned logical artifact = %+v", first.PrivateArtifacts[0])
	}

	retry := appendOwnerCommand(t, handler, request)
	if len(retry.PrivateArtifacts) != 1 || retry.PrivateArtifacts[0].ArtifactDigest != first.PrivateArtifacts[0].ArtifactDigest ||
		retry.PrivateArtifacts[0].Ref.String() != first.PrivateArtifacts[0].Ref.String() {
		t.Fatalf("retry artifacts = %+v, want %+v", retry.PrivateArtifacts, first.PrivateArtifacts)
	}
}

func TestTextureSupervisionCommandMapsStaleExpectedToConflict(t *testing.T) {
	handler, ownerID, trajectoryID, intentID, expected := setupOwnerCommandTrajectory(t)
	staleHead := strings.Repeat("a", 64)
	expected.CanonicalEventHead = &staleHead
	request := ownerDecisionCommandRequest(t, handler, ownerID, trajectoryID, intentID, expected, "owner-command-stale", "owner-decision-stale")

	response := httptest.NewRecorder()
	handler.HandleTextureRouter(response, textureRequest(t, http.MethodPost, "/api/texture/supervision/command", request))
	if response.Code != http.StatusConflict {
		t.Fatalf("stale expected status = %d body=%s", response.Code, response.Body.String())
	}
}

func TestTextureSupervisionCommandRejectsChangedBodyForCommandID(t *testing.T) {
	handler, ownerID, trajectoryID, intentID, expected := setupOwnerCommandTrajectory(t)
	first := ownerDecisionCommandRequest(t, handler, ownerID, trajectoryID, intentID, expected, "owner-command-retry", "owner-decision-1")
	if result := appendOwnerCommand(t, handler, first); result.ArtifactDigest == "" {
		t.Fatalf("first append result = %+v", result)
	}

	changed := ownerDecisionCommandRequest(t, handler, ownerID, trajectoryID, intentID, expected, "owner-command-retry", "owner-decision-changed")
	response := httptest.NewRecorder()
	handler.HandleTextureRouter(response, textureRequest(t, http.MethodPost, "/api/texture/supervision/command", changed))
	if response.Code != http.StatusConflict {
		t.Fatalf("changed retry status = %d body=%s", response.Code, response.Body.String())
	}
}

func TestTextureSupervisionCommandRejectsDigestMismatchBeforeAppend(t *testing.T) {
	handler, ownerID, trajectoryID, intentID, expected := setupOwnerCommandTrajectory(t)
	request := ownerDecisionCommandRequest(t, handler, ownerID, trajectoryID, intentID, expected, "owner-command-mismatch", "owner-decision-1")
	request.CommandDigest = strings.Repeat("0", 64)
	before, err := handler.Store.Head(context.Background(), handler.Core.TextureSandboxID())
	if err != nil {
		t.Fatal(err)
	}

	response := httptest.NewRecorder()
	handler.HandleTextureRouter(response, textureRequest(t, http.MethodPost, "/api/texture/supervision/command", request))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("digest mismatch status = %d body=%s", response.Code, response.Body.String())
	}
	after, err := handler.Store.Head(context.Background(), handler.Core.TextureSandboxID())
	if err != nil {
		t.Fatal(err)
	}
	if after.CanonicalEventHead != before.CanonicalEventHead || after.Sequence != before.Sequence {
		t.Fatalf("digest mismatch appended: before=%+v after=%+v", before, after)
	}
}

func TestTextureSupervisionCommandRejectsPrePinnedArtifactReference(t *testing.T) {
	handler, ownerID, trajectoryID, intentID, expected := setupOwnerCommandTrajectory(t)
	request := ownerDecisionCommandRequest(t, handler, ownerID, trajectoryID, intentID, expected, "owner-command-prepinned", "owner-decision-1")
	var body map[string]any
	if err := json.Unmarshal(request.Mutations[0].Body, &body); err != nil {
		t.Fatal(err)
	}
	body["decision_artifact_ref"] = "artifact:sha256:" + strings.Repeat("a", 64)
	rewritten, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	request.Mutations[0].Body = rewritten
	request.CommandDigest = ownerCommandDigest(t, handler, ownerID, request)

	response := httptest.NewRecorder()
	handler.HandleTextureRouter(response, textureRequest(t, http.MethodPost, "/api/texture/supervision/command", request))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("pre-pinned ref status = %d body=%s", response.Code, response.Body.String())
	}
}

func appendOwnerCommand(t *testing.T, handler *Handler, request textureSupervisionCommandRequest) textureSupervisionCommandResponse {
	t.Helper()
	response := httptest.NewRecorder()
	handler.HandleTextureRouter(response, textureRequest(t, http.MethodPost, "/api/texture/supervision/command", request))
	if response.Code != http.StatusOK {
		t.Fatalf("command status = %d body=%s", response.Code, response.Body.String())
	}
	var result textureSupervisionCommandResponse
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func setupOwnerCommandTrajectory(t *testing.T) (*Handler, string, string, string, computerevent.SupervisionExpected) {
	t.Helper()
	_, handler := testAPISetup(t)
	const ownerID = "user-1"
	const trajectoryID = "owner-command-trajectory"
	const initialCommandID = "owner-command-initial"
	const documentID = "owner-command-document"
	const revisionID = "owner-command-revision"
	const workItemID = "owner-command-work"
	const actorID = "texture:" + documentID
	start := types.StartLifecycleRequest{
		OwnerID: ownerID, ComputerID: handler.Core.TextureSandboxID(), CommandID: initialCommandID,
		TrajectoryID: trajectoryID, Kind: types.TrajectoryKindTask,
		SubjectRefs:     map[string]string{"artifact": "texture://documents/" + documentID},
		SettlementRule:  types.SettlementRule{Version: types.LifecycleReducerVersion, RequireNoOpenWorkItems: true},
		InitialWork:     types.WorkItemRecord{WorkItemID: workItemID, Objective: "Initial supervised document.", AssignedAgentID: actorID, AuthorityProfile: "texture"},
		InitialDocument: types.Document{DocID: documentID, OwnerID: ownerID, ComputerID: handler.Core.TextureSandboxID(), TrajectoryID: trajectoryID, Title: "Owner command"},
		InitialRevision: types.Revision{RevisionID: revisionID, DocID: documentID, OwnerID: ownerID, ComputerID: handler.Core.TextureSandboxID(), TrajectoryID: trajectoryID, AuthorKind: types.AuthorUser, AuthorLabel: ownerID, Content: "Initial content."},
		Agent:           types.AgentRecord{AgentID: actorID, OwnerID: ownerID, ComputerID: handler.Core.TextureSandboxID(), SandboxID: handler.Core.TextureSandboxID(), Profile: "texture", Role: "texture", ChannelID: documentID},
	}
	if _, err := handler.startSupervisionTrajectory(context.Background(), start); err != nil {
		t.Fatalf("start supervision trajectory: %v", err)
	}
	head, err := handler.Store.Head(context.Background(), handler.Core.TextureSandboxID())
	if err != nil {
		t.Fatal(err)
	}
	intentID := uuid.NewSHA1(uuid.NameSpaceOID, []byte(initialCommandID+":intent:v1")).String()
	return handler, ownerID, trajectoryID, intentID, computerevent.SupervisionExpected{CanonicalEventHead: &head.CanonicalEventHead}
}

func ownerDecisionCommandRequest(t *testing.T, handler *Handler, ownerID, trajectoryID, parentIntentID string, expected computerevent.SupervisionExpected, commandID, decisionID string) textureSupervisionCommandRequest {
	t.Helper()
	bindingID := commandID + ":owner-decision"
	body, err := json.Marshal(map[string]any{
		"decision_id": decisionID, "proposal_id": "proposal-1", "owner_actor_id": ownerID,
		"decision_artifact_ref": computerevent.SupervisionArtifactPlaceholder(bindingID), "scope_digest": computerevent.DigestBytes([]byte(parentIntentID)),
	})
	if err != nil {
		t.Fatal(err)
	}
	request := textureSupervisionCommandRequest{
		TrajectoryID: trajectoryID, TransactionID: commandID, TransactionClass: "record_owner_decision",
		CommandID: commandID, Expected: &expected,
		Mutations:        []computerevent.SupervisionMutation{{Kind: "owner_decision_recorded", Body: body}},
		PrivateArtifacts: []textureSupervisionPrivateArtifact{{BindingID: bindingID, Plaintext: "owner decision " + decisionID, MediaType: "text/plain"}},
	}
	request.CommandDigest = ownerCommandDigest(t, handler, ownerID, request)
	return request
}

func ownerCommandDigest(t *testing.T, handler *Handler, ownerID string, request textureSupervisionCommandRequest) string {
	t.Helper()
	_, artifacts, err := textureSupervisionPrivatePayloads(request.PrivateArtifacts)
	if err != nil {
		t.Fatal(err)
	}
	transaction := computerevent.SupervisionTransaction{
		Schema: computerevent.SupervisionSchemaV1, Reducer: computerevent.SupervisionReducerV1,
		DigestRecipe: computerevent.SupervisionDigestRecipeV1, TransactionID: request.TransactionID,
		TransactionClass: request.TransactionClass, OwnerID: ownerID, ComputerID: handler.Core.TextureSandboxID(),
		TrajectoryID: request.TrajectoryID, CommandID: request.CommandID,
		Actor:    computerevent.SupervisionActor{ActorID: ownerID, Role: "owner", AuthorityRef: "owner:" + ownerID},
		Expected: *request.Expected, Mutations: request.Mutations, ReferencedArtifacts: artifacts,
	}
	digest, err := transaction.ComputeCommandDigest()
	if err != nil {
		t.Fatal(err)
	}
	return digest
}
