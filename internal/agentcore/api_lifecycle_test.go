package agentcore

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
	"strings"
	"time"
)

func TestLifecycleAPIOwnerScopedSnapshotAndRawStartRefusal(t *testing.T) {
	rt, handler := testAPISetup(t)
	request := types.StartLifecycleRequest{
		OwnerID: "owner-api", ComputerID: "sandbox-test",
		CommandID: "api-start-command-1", TrajectoryID: "api-trajectory-1",
		Kind:            types.TrajectoryKindTask,
		SubjectRefs:     map[string]string{"artifact": "texture://api/artifact", "doc_id": "api-document-1"},
		SettlementRule:  types.SettlementRule{RequireNoOpenWorkItems: true, RequiredSubjectRefs: []string{"artifact"}},
		InitialWork:     types.WorkItemRecord{WorkItemID: "api-work-1", Objective: "exercise public lifecycle"},
		InitialDocument: types.Document{DocID: "api-document-1", Title: "API lifecycle"},
		InitialRevision: types.Revision{RevisionID: "api-revision-v0", AuthorKind: types.AuthorAppAgent, AuthorLabel: "Choir", Content: "Initial API artifact"},
		Agent:           types.AgentRecord{AgentID: "texture:api-document-1", Profile: "texture", Role: "texture", ChannelID: "api-document-1"},
	}
	digest, err := store.ComputeStartLifecycleRequestDigest(request)
	if err != nil {
		t.Fatalf("compute start digest: %v", err)
	}
	request.StartRequestDigest = digest
	result, err := rt.Store().StartLifecycle(context.Background(), request)
	if err != nil {
		t.Fatalf("start lifecycle through internal authority: %v", err)
	}
	if result.Trajectory.OwnerID != "owner-api" || result.Trajectory.ComputerID != "sandbox-test" {
		t.Fatalf("unexpected lifecycle scope: %+v", result.Trajectory)
	}
	replay, err := rt.Store().StartLifecycle(context.Background(), request)
	if err != nil || !replay.Replay {
		t.Fatalf("unexpected internal replay: %+v, %v", replay, err)
	}
	body, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	rawStart := runtimeHandlerRequest(t, handler.HandleLifecycle, http.MethodPost, "/api/lifecycle/start", string(body), "owner-api")
	if rawStart.Code != http.StatusNotFound {
		t.Fatalf("raw public start status = %d, want 404; body=%s", rawStart.Code, rawStart.Body.String())
	}

	snapshot := runtimeHandlerRequest(t, handler.HandleTrajectoryDetail, http.MethodGet, "/api/trajectories/api-trajectory-1", "", "owner-api")
	if snapshot.Code != http.StatusOK {
		t.Fatalf("snapshot status = %d; body=%s", snapshot.Code, snapshot.Body.String())
	}
	var snapshotBody types.LifecycleSnapshot
	if err := json.Unmarshal(snapshot.Body.Bytes(), &snapshotBody); err != nil || snapshotBody.Schema != types.DurableWorkSchemaV1 || snapshotBody.SnapshotCursor != 1 || snapshotBody.Watermark != 1 {
		t.Fatalf("unexpected snapshot: %+v, %v", snapshotBody, err)
	}
	if snapshotBody.Activation.AgentID != request.Agent.AgentID || snapshotBody.Activation.State != types.RunPassivated {
		t.Fatalf("unexpected activation projection: %+v", snapshotBody.Activation)
	}
	eventPage := runtimeHandlerRequest(t, handler.HandleTrajectoryDetail, http.MethodGet, "/api/trajectories/api-trajectory-1/events?after=0&limit=1", "", "owner-api")
	if eventPage.Code != http.StatusOK {
		t.Fatalf("event page status = %d; body=%s", eventPage.Code, eventPage.Body.String())
	}
	var page types.LifecycleEventPage
	if err := json.Unmarshal(eventPage.Body.Bytes(), &page); err != nil || page.Schema != types.DurableWorkSchemaV1 || len(page.Events) != 1 || page.NextCursor != 1 || page.Watermark != 1 {
		t.Fatalf("unexpected event page: %+v, %v", page, err)
	}
	expired := runtimeHandlerRequest(t, handler.HandleTrajectoryDetail, http.MethodGet, "/api/trajectories/api-trajectory-1/events?after=99&limit=1", "", "owner-api")
	if expired.Code != http.StatusConflict {
		t.Fatalf("expired cursor status = %d, want 409; body=%s", expired.Code, expired.Body.String())
	}
	var expiredBody types.LifecycleEventPage
	if err := json.Unmarshal(expired.Body.Bytes(), &expiredBody); err != nil || expiredBody.Schema != types.DurableWorkSchemaV1 || !expiredBody.CursorExpired || !expiredBody.ReplayRequired {
		t.Fatalf("unexpected expired cursor response: %+v, %v", expiredBody, err)
	}
	streamContext, cancelStream := context.WithCancel(context.Background())
	streamRequest := authenticatedRequest(http.MethodGet, "/api/trajectories/api-trajectory-1/stream?after=0", "", "owner-api").WithContext(streamContext)
	streamResponse := httptest.NewRecorder()
	streamDone := make(chan struct{})
	go func() {
		handler.HandleTrajectoryDetail(streamResponse, streamRequest)
		close(streamDone)
	}()
	time.Sleep(50 * time.Millisecond)
	cancelStream()
	<-streamDone
	if streamResponse.Header().Get("Content-Type") != "text/event-stream" || !strings.Contains(streamResponse.Body.String(), `"schema":"choir.durable_work.v1"`) || !strings.Contains(streamResponse.Body.String(), "event: lifecycle") {
		t.Fatalf("unexpected lifecycle stream: headers=%v body=%s", streamResponse.Header(), streamResponse.Body.String())
	}
	otherOwner := runtimeHandlerRequest(t, handler.HandleTrajectoryDetail, http.MethodGet, "/api/trajectories/api-trajectory-1", "", "owner-other")
	if otherOwner.Code != http.StatusNotFound {
		t.Fatalf("cross-owner snapshot status = %d, want 404; body=%s", otherOwner.Code, otherOwner.Body.String())
	}
	cancelRequest := types.CancelLifecycleRequest{
		CommandID: "api-cancel-command-1", TrajectoryID: request.TrajectoryID, Reason: "owner completed lifecycle",
	}
	cancelRequest.CommandDigest, _ = store.ComputeCancelLifecycleDigest(cancelRequest)
	cancelBody, err := json.Marshal(cancelRequest)
	if err != nil {
		t.Fatalf("marshal cancel: %v", err)
	}
	cancelResponse := runtimeHandlerRequest(t, handler.HandleLifecycle, http.MethodPost, "/api/lifecycle/trajectories/cancel", string(cancelBody), "owner-api")
	if cancelResponse.Code != http.StatusOK {
		t.Fatalf("cancel status = %d; body=%s", cancelResponse.Code, cancelResponse.Body.String())
	}
	var cancelResult types.LifecycleResult
	if err := json.Unmarshal(cancelResponse.Body.Bytes(), &cancelResult); err != nil {
		t.Fatalf("decode cancel response: %v", err)
	}
	archive := types.ArchiveLifecycleArtifactRequest{
		CommandID: "api-archive-command-1", TrajectoryID: request.TrajectoryID,
		ExpectedLifecycleVersion: cancelResult.Trajectory.LifecycleVersion,
		ExpectedHeadRevisionID:   snapshotBody.HeadRevision.RevisionID,
		Reason:                   "owner archive",
	}
	archive.CommandDigest, _ = store.ComputeArchiveLifecycleArtifactDigest(archive)
	archiveBody, err := json.Marshal(archive)
	if err != nil {
		t.Fatalf("marshal archive: %v", err)
	}
	archiveResponse := runtimeHandlerRequest(t, handler.HandleLifecycle, http.MethodPost, "/api/lifecycle/artifacts/archive", string(archiveBody), "owner-api")
	if archiveResponse.Code != http.StatusOK {
		t.Fatalf("archive status = %d; body=%s", archiveResponse.Code, archiveResponse.Body.String())
	}
	var archiveResult types.LifecycleResult
	if err := json.Unmarshal(archiveResponse.Body.Bytes(), &archiveResult); err != nil || archiveResult.Schema != types.DurableWorkSchemaV1 || archiveResult.Document == nil || archiveResult.Document.ArchivedAt == nil {
		t.Fatalf("unexpected archive response: %+v, %v", archiveResult, err)
	}
}
