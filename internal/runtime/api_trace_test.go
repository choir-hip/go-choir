//go:build comprehensive

package runtime

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func seedTraceTrajectory(t *testing.T, rt *Runtime) (*types.RunRecord, *types.RunRecord) {
	t.Helper()

	parent, err := rt.StartRunWithMetadata(context.Background(), "Investigate moss habitats", "user-alice", map[string]any{
		runMetadataAgentProfile: "conductor",
		runMetadataAgentRole:    "conductor",
	})
	if err != nil {
		t.Fatalf("start parent run: %v", err)
	}
	child, err := rt.StartChildRun(context.Background(), parent.RunID, "Research the best conditions for moss", "user-alice", map[string]any{
		runMetadataAgentProfile: "researcher",
		runMetadataAgentRole:    "researcher",
	})
	if err != nil {
		t.Fatalf("start child run: %v", err)
	}

	findingAt := time.Now().UTC()
	message := &types.ChannelMessage{
		ChannelID:    child.ChannelID,
		From:         "researcher",
		FromAgentID:  child.AgentID,
		FromRunID:    child.RunID,
		ToAgentID:    parent.AgentID,
		TrajectoryID: parent.RunID,
		Role:         "researcher",
		Content:      "Finding: moss thrives in damp shade with steady humidity.",
		Timestamp:    findingAt,
	}
	finding := types.ResearchFindingRecord{
		FindingID:     "finding-" + uuid.NewString(),
		OwnerID:       "user-alice",
		AgentID:       child.AgentID,
		TargetAgentID: parent.AgentID,
		ChannelID:     child.ChannelID,
		TrajectoryID:  parent.RunID,
		Findings:      []string{"Moss thrives in damp shade with steady humidity."},
		EvidenceIDs:   []string{"ev-moss-1"},
		Notes:         []string{"Humidity matters more than direct light."},
		Questions:     []string{"Which moss species tolerate brighter light?"},
		Content:       message.Content,
		CreatedAt:     findingAt,
	}
	delivery := types.InboxDelivery{
		DeliveryID:   "delivery-" + uuid.NewString(),
		OwnerID:      "user-alice",
		ToAgentID:    parent.AgentID,
		FromAgentID:  child.AgentID,
		FromRunID:    child.RunID,
		ChannelID:    child.ChannelID,
		Role:         "researcher",
		Content:      message.Content,
		TrajectoryID: parent.RunID,
		CreatedAt:    findingAt,
	}
	if _, created, err := rt.store.DispatchResearchFinding(context.Background(), finding, message, delivery); err != nil {
		t.Fatalf("dispatch research finding: %v", err)
	} else if created {
		rt.emitChannelMessageEvent(WithToolExecutionContext(context.Background(), child), *message, child.OwnerID)
	}

	time.Sleep(200 * time.Millisecond)
	return parent, child
}

func TestHandleTraceTrajectoryIndexOwnerScoped(t *testing.T) {
	rt, handler := testAPISetup(t)

	parent, _ := seedTraceTrajectory(t, rt)
	if _, err := rt.StartRun(context.Background(), "bob trajectory", "user-bob"); err != nil {
		t.Fatalf("start bob run: %v", err)
	}

	req := authenticatedRequest(http.MethodGet, "/api/trace/trajectories?limit=50", "", "user-alice")
	w := httptest.NewRecorder()
	handler.HandleTraceTrajectories(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", w.Code, http.StatusOK)
	}

	var resp traceTrajectoryListResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Trajectories) == 0 {
		t.Fatal("expected at least one trajectory")
	}
	if resp.Trajectories[0].TrajectoryID != parent.RunID {
		t.Fatalf("first trajectory_id: got %q, want %q", resp.Trajectories[0].TrajectoryID, parent.RunID)
	}
	for _, trajectory := range resp.Trajectories {
		if trajectory.Title == "" {
			t.Fatal("trajectory title should not be empty")
		}
		if trajectory.TrajectoryID == "" {
			t.Fatal("trajectory_id should not be empty")
		}
	}
}

func TestTraceTrajectorySummaryUsesEntryRunTitle(t *testing.T) {
	now := time.Now().UTC()
	parent := types.RunRecord{
		RunID:        "prompt-bar-root",
		AgentID:      "conductor:prompt-bar-root",
		AgentProfile: AgentProfileConductor,
		AgentRole:    AgentProfileConductor,
		OwnerID:      "user-alice",
		State:        types.RunCompleted,
		Prompt:       "Trace smoke user-visible title",
		CreatedAt:    now,
		UpdatedAt:    now,
		Metadata: map[string]any{
			runMetadataAgentProfile: AgentProfileConductor,
			runMetadataAgentRole:    AgentProfileConductor,
			runMetadataTrajectoryID: "prompt-bar-root",
			"input_source":          "prompt_bar",
		},
	}
	child := types.RunRecord{
		RunID:        "vtext-revision-child",
		AgentID:      "vtext:vtext-revision-child",
		ParentRunID:  parent.RunID,
		AgentProfile: AgentProfileVText,
		AgentRole:    AgentProfileVText,
		OwnerID:      "user-alice",
		State:        types.RunRunning,
		Prompt:       "A revise event was triggered for the current vtext document. Intent: regenerate text.",
		CreatedAt:    now.Add(time.Second),
		UpdatedAt:    now.Add(2 * time.Second),
		Metadata: map[string]any{
			runMetadataAgentProfile: AgentProfileVText,
			runMetadataAgentRole:    AgentProfileVText,
			runMetadataTrajectoryID: parent.RunID,
		},
	}

	runs := []types.RunRecord{parent, child}
	agents, _ := buildTraceAgentNodes(runs)
	summary := buildTraceTrajectorySummary(parent.RunID, runs, agents, buildTraceAgentEdges(runs), nil, nil, nil, traceSearchSummary{})

	if summary.Title != parent.Prompt {
		t.Fatalf("title = %q, want prompt-bar parent prompt %q", summary.Title, parent.Prompt)
	}
	if summary.LatestActivityAt != formatTraceTime(child.UpdatedAt) {
		t.Fatalf("latest activity = %q, want child update %q", summary.LatestActivityAt, formatTraceTime(child.UpdatedAt))
	}
	if !summary.Live {
		t.Fatal("summary should remain live because the child run is running")
	}
}

func TestRegisteredTraceRoutesAreReadOnly(t *testing.T) {
	_, handler := testAPISetup(t)

	cases := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/trace/trajectories"},
		{http.MethodPut, "/api/trace/trajectories/trajectory-1"},
		{http.MethodPost, "/api/trace/trajectories/trajectory-1/events"},
		{http.MethodDelete, "/api/trace/trajectories/trajectory-1/moments/moment-1"},
	}
	for _, tc := range cases {
		w := registeredRuntimeRequest(t, handler, tc.method, tc.path, `{}`, "user-alice")
		if w.Code != http.StatusMethodNotAllowed {
			t.Fatalf("%s %s: got status %d, want 405; body=%s", tc.method, tc.path, w.Code, w.Body.String())
		}
	}
}

func TestHandleTraceTrajectorySnapshotIncludesGraphAndMoments(t *testing.T) {
	rt, handler := testAPISetup(t)

	parent, child := seedTraceTrajectory(t, rt)
	modelPayload, _ := json.Marshal(map[string]any{
		"llm_provider":         "fireworks",
		"llm_model":            "accounts/fireworks/models/deepseek-v4-flash",
		"llm_reasoning_effort": "low",
		"model_policy":         "run_metadata",
	})
	rt.emitEvent(context.Background(), child, types.EventRunProgress, events.CauseProviderProgress, modelPayload)
	if _, err := rt.store.UpsertRunAcceptance(context.Background(), types.RunAcceptanceRecord{
		AcceptanceID:          "acceptance-trace-1",
		TargetMissionID:       "mission-trace-test",
		SourcePromptObjective: "prove trace acceptances are visible",
		OwnerID:               "user-alice",
		DesktopID:             types.PrimaryDesktopID,
		TrajectoryID:          parent.RunID,
		RunID:                 child.RunID,
		AuthorityProfile:      AgentProfileSuper,
		DeploymentCommit:      "commit-trace-test",
		AcceptanceLevel:       types.RunAcceptanceExportLevel,
		State:                 types.RunAcceptanceAccepted,
		Checkpoints: []types.RunAcceptanceCheckpoint{{
			Kind:           "worker_delegated",
			State:          "passed",
			At:             time.Now().UTC(),
			EvidenceRefIDs: []string{"event:trace-test"},
			Details:        map[string]any{"worker_loop_id": child.RunID},
		}},
		EvidenceRefs: []types.RunAcceptanceEvidenceRef{{
			RefID:   "event:trace-test",
			Kind:    "tool.result",
			Summary: "worker run exported concrete patchset evidence",
			RunID:   child.RunID,
			EventID: "event-trace-test",
			Details: map[string]any{
				"worker_child_run_ids": []string{"implementation-trace-test", "verifier-trace-test"},
				"export_count":         1,
			},
		}},
		RollbackRefs: []types.RunAcceptanceRollbackRef{{
			Kind:    "git_base",
			Ref:     "base-trace-test",
			Summary: "discard candidate",
		}},
	}); err != nil {
		t.Fatalf("seed run acceptance: %v", err)
	}

	req := authenticatedRequest(http.MethodGet, "/api/trace/trajectories/"+parent.RunID, "", "user-alice")
	w := httptest.NewRecorder()
	handler.HandleTraceTrajectories(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", w.Code, http.StatusOK)
	}

	var resp traceTrajectorySnapshotResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Trajectory.TrajectoryID != parent.RunID {
		t.Fatalf("trajectory_id: got %q, want %q", resp.Trajectory.TrajectoryID, parent.RunID)
	}
	if len(resp.Agents) < 2 {
		t.Fatalf("agents: got %d, want at least 2", len(resp.Agents))
	}
	if len(resp.Edges) == 0 {
		t.Fatal("expected at least one delegation edge")
	}
	if len(resp.Acceptances) != 1 || resp.Acceptances[0].AcceptanceID != "acceptance-trace-1" {
		t.Fatalf("acceptances: got %+v, want seeded acceptance", resp.Acceptances)
	}
	if len(resp.Acceptances[0].EvidenceRefs) != 1 || resp.Acceptances[0].EvidenceRefs[0].Details["worker_child_run_ids"] == nil {
		t.Fatalf("acceptance evidence missing structured details: %+v", resp.Acceptances[0].EvidenceRefs)
	}
	if resp.MobileSummary.AcceptanceID != "acceptance-trace-1" {
		t.Fatalf("mobile_summary acceptance_id = %q, want seeded acceptance", resp.MobileSummary.AcceptanceID)
	}
	if resp.MobileSummary.AcceptanceLevel != types.RunAcceptanceExportLevel || resp.MobileSummary.AcceptanceState != types.RunAcceptanceAccepted {
		t.Fatalf("mobile_summary acceptance state/level = %+v", resp.MobileSummary)
	}
	if resp.MobileSummary.AgentCount < 2 || resp.MobileSummary.EvidenceRefCount != 1 || resp.MobileSummary.RollbackRefCount != 1 {
		t.Fatalf("mobile_summary counts = %+v", resp.MobileSummary)
	}
	if len(resp.MobileSummary.ReadableEvidence) != 1 || !strings.Contains(resp.MobileSummary.ReadableEvidence[0], "exported concrete patchset") {
		t.Fatalf("mobile_summary evidence = %+v", resp.MobileSummary.ReadableEvidence)
	}
	if resp.MobileSummary.PrimaryRollbackRef == "" || !strings.Contains(resp.MobileSummary.PrimaryRollbackRef, "discard candidate") {
		t.Fatalf("mobile_summary rollback = %+v", resp.MobileSummary)
	}
	foundEdge := false
	for _, edge := range resp.Edges {
		if edge.FromAgentID == parent.AgentID && edge.ToAgentID == child.AgentID {
			foundEdge = true
		}
	}
	if !foundEdge {
		t.Fatalf("expected delegation edge from %s to %s", parent.AgentID, child.AgentID)
	}
	foundMessageMoment := false
	foundModelMoment := false
	for _, moment := range resp.Moments {
		if moment.Kind == types.EventChannelMessage && strings.Contains(moment.Summary, "damp shade") {
			foundMessageMoment = true
			if moment.MessageSeq == 0 {
				t.Fatal("channel.message moment should include message_seq")
			}
		}
		if moment.Kind == types.EventRunProgress && moment.LLMProvider == "fireworks" {
			foundModelMoment = true
			if moment.LLMModel != "accounts/fireworks/models/deepseek-v4-flash" {
				t.Fatalf("llm_model = %q", moment.LLMModel)
			}
			if moment.LLMReasoning != "low" {
				t.Fatalf("llm_reasoning_effort = %q", moment.LLMReasoning)
			}
			if moment.ModelPolicy != "run_metadata" {
				t.Fatalf("model_policy = %q", moment.ModelPolicy)
			}
		}
	}
	if !foundMessageMoment {
		t.Fatal("expected research channel.message moment")
	}
	if !foundModelMoment {
		t.Fatal("expected model policy moment in trace snapshot")
	}
}

func TestHandleTraceTrajectoryLogsReturnsDebugText(t *testing.T) {
	rt, handler := testAPISetup(t)

	parent, _ := seedTraceTrajectory(t, rt)

	req := authenticatedRequest(http.MethodGet, "/api/trace/trajectories/"+parent.RunID+"/logs", "", "user-alice")
	w := httptest.NewRecorder()
	handler.HandleTraceTrajectories(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body=%s", w.Code, http.StatusOK, w.Body.String())
	}
	if got := w.Header().Get("Content-Type"); !strings.Contains(got, "text/plain") {
		t.Fatalf("content-type = %q, want text/plain", got)
	}
	body := w.Body.String()
	for _, want := range []string{
		"Trajectory: Investigate moss habitats",
		"Agents",
		"Events",
		"Channel Messages",
		"Moss thrives in damp shade",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("log body missing %q:\n%s", want, body)
		}
	}
}

func TestTraceTrajectorySummaryStaysLiveWhileAnyRunIsActive(t *testing.T) {
	now := time.Now().UTC()
	finishedAt := now.Add(30 * time.Second)
	runs := []types.RunRecord{
		{
			RunID:        "run-conductor-live-summary",
			AgentID:      "agent-conductor-live-summary",
			AgentProfile: AgentProfileConductor,
			AgentRole:    AgentProfileConductor,
			State:        types.RunCompleted,
			Prompt:       "Open the mission VText.",
			CreatedAt:    now,
			UpdatedAt:    finishedAt,
			FinishedAt:   &finishedAt,
			Metadata: map[string]any{
				runMetadataAgentProfile: AgentProfileConductor,
				runMetadataAgentRole:    AgentProfileConductor,
				runMetadataTrajectoryID: "traj-live-summary",
			},
		},
		{
			RunID:        "run-super-live-summary",
			AgentID:      "agent-super-live-summary",
			ParentRunID:  "run-conductor-live-summary",
			AgentProfile: AgentProfileSuper,
			AgentRole:    AgentProfileSuper,
			State:        types.RunRunning,
			Prompt:       "Delegate to a worker VM.",
			CreatedAt:    now.Add(5 * time.Second),
			UpdatedAt:    now.Add(10 * time.Second),
			Metadata: map[string]any{
				runMetadataAgentProfile: AgentProfileSuper,
				runMetadataAgentRole:    AgentProfileSuper,
				runMetadataTrajectoryID: "traj-live-summary",
			},
		},
	}
	agents, _ := buildTraceAgentNodes(runs)

	summary := buildTraceTrajectorySummary("traj-live-summary", runs, agents, nil, nil, nil, nil, traceSearchSummary{})
	if summary.State != types.RunRunning {
		t.Fatalf("trajectory state = %q, want running", summary.State)
	}
	if !summary.Live {
		t.Fatal("trajectory live = false, want true while super run is active")
	}
}

func TestTraceRunGeometryMomentsHaveReadableSummaries(t *testing.T) {
	rt, handler := testAPISetup(t)

	parent, _ := seedTraceTrajectory(t, rt)
	ctx := context.Background()
	compactionEntry, err := rt.store.AppendRunMemoryEntry(ctx, types.RunMemoryEntry{
		RunID:        parent.RunID,
		OwnerID:      parent.OwnerID,
		AgentID:      parent.AgentID,
		Kind:         types.RunMemoryEntryCompaction,
		Summary:      "Kept the operational checkpoint.",
		TokensBefore: 240000,
		Reason:       "threshold",
		Details:      map[string]any{"tokens_after": 48000},
	})
	if err != nil {
		t.Fatalf("append compaction artifact: %v", err)
	}
	compactionPayload, _ := json.Marshal(map[string]any{
		"entry_id":      compactionEntry.EntryID,
		"reason":        "threshold",
		"tokens_before": 240000,
		"tokens_after":  48000,
	})
	rt.emitEvent(ctx, parent, types.EventRunCompactionCompleted, "run_memory", compactionPayload)
	continuation, err := rt.store.CreateRunContinuation(ctx, types.RunContinuationRecord{
		ContinuationID:   "cont-123",
		OwnerID:          parent.OwnerID,
		SourceRunID:      parent.RunID,
		Objective:        "Continue the run geometry slice",
		AuthorityProfile: AgentProfileVSuper,
		LeaseSeconds:     60,
		Status:           types.RunContinuationSelected,
		Details:          map[string]any{"objective_fingerprint": "abcdef1234567890"},
	})
	if err != nil {
		t.Fatalf("create continuation artifact: %v", err)
	}
	continuationPayload, _ := json.Marshal(map[string]any{
		"continuation_id":       continuation.ContinuationID,
		"objective_fingerprint": "abcdef1234567890",
		"next_loop_id":          "loop-next-123",
	})
	rt.emitEvent(ctx, parent, types.EventRunContinuationSelected, "run_memory", continuationPayload)
	appPackage, err := rt.PublishAppChangePackage(ctx, parent.OwnerID, publishAppChangePackageInput{
		PackageID:             "package-123456",
		AppID:                 "trace-artifact-app",
		Visibility:            "unlisted",
		SourceComputerID:      "source-computer-trace",
		SourceCandidateID:     "source-candidate-trace",
		SourceLedgerBaseRef:   "base-trace-artifact",
		CandidateSourceRef:    "refs/computers/source/candidates/trace-artifact",
		RuntimeSourceDelta:    "diff --git a/runtime.txt b/runtime.txt\nnew file mode 100644\n--- /dev/null\n+++ b/runtime.txt\n@@ -0,0 +1 @@\n+trace artifact\n",
		UISourceDelta:         "diff --git a/frontend/ui.txt b/frontend/ui.txt\nnew file mode 100644\n--- /dev/null\n+++ b/frontend/ui.txt\n@@ -0,0 +1 @@\n+trace artifact\n",
		AppProtocolContract:   "trace artifact app contract",
		SourceLedgerCommitSHA: "worker-trace-artifact",
		TraceID:               parent.RunID,
	})
	if err != nil {
		t.Fatalf("publish app package artifact: %v", err)
	}
	appAdoption, err := rt.CreateAppAdoption(ctx, parent.OwnerID, "target-computer-trace", createAppAdoptionInput{
		AdoptionID:         "adoption-123456",
		PackageID:          appPackage.PackageID,
		TargetCandidateID:  "target-candidate-trace",
		CandidateSourceRef: "refs/computers/target/candidates/trace-artifact",
		TraceID:            parent.RunID,
	})
	if err != nil {
		t.Fatalf("create app adoption artifact: %v", err)
	}

	req := authenticatedRequest(http.MethodGet, "/api/trace/trajectories/"+parent.RunID, "", "user-alice")
	w := httptest.NewRecorder()
	handler.HandleTraceTrajectories(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
	var resp traceTrajectorySnapshotResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	summaries := map[types.EventKind]traceMomentSummary{}
	for _, moment := range resp.Moments {
		summaries[moment.Kind] = moment
	}
	if got := summaries[types.EventRunCompactionCompleted].Summary; got != "compacted context 240000 -> 48000 tokens" {
		t.Fatalf("compaction summary = %q", got)
	}
	if got := summaries[types.EventRunContinuationSelected].Summary; !strings.Contains(got, "selected continuation") || !strings.Contains(got, "abcdef") {
		t.Fatalf("continuation summary = %q", got)
	}
	if got := summaries[types.EventAppChangePackagePublished].Summary; !strings.Contains(got, "published app package") {
		t.Fatalf("app package summary = %q", got)
	}
	if got := summaries[types.EventAppAdoptionProposed].Summary; !strings.Contains(got, "started app adoption") {
		t.Fatalf("app adoption summary = %q", got)
	}
	if summaries[types.EventRunCompactionCompleted].Tone != "success" ||
		summaries[types.EventRunContinuationSelected].Tone != "active" ||
		summaries[types.EventAppChangePackagePublished].Tone != "active" ||
		summaries[types.EventAppAdoptionProposed].Tone != "active" {
		t.Fatalf("unexpected tones: compaction=%q continuation=%q appPackage=%q appAdoption=%q",
			summaries[types.EventRunCompactionCompleted].Tone,
			summaries[types.EventRunContinuationSelected].Tone,
			summaries[types.EventAppChangePackagePublished].Tone,
			summaries[types.EventAppAdoptionProposed].Tone)
	}

	for kind, assertArtifact := range map[types.EventKind]func(traceMomentDetailResponse){
		types.EventRunCompactionCompleted: func(detail traceMomentDetailResponse) {
			if detail.References.RunMemoryEntryID != compactionEntry.EntryID {
				t.Fatalf("run memory reference = %q, want %q", detail.References.RunMemoryEntryID, compactionEntry.EntryID)
			}
			if detail.Artifacts.RunMemory == nil || detail.Artifacts.RunMemory.EntryID != compactionEntry.EntryID {
				t.Fatalf("run memory artifact = %+v, want %s", detail.Artifacts.RunMemory, compactionEntry.EntryID)
			}
		},
		types.EventRunContinuationSelected: func(detail traceMomentDetailResponse) {
			if detail.References.ContinuationID != continuation.ContinuationID {
				t.Fatalf("continuation reference = %q, want %q", detail.References.ContinuationID, continuation.ContinuationID)
			}
			if detail.Artifacts.Continuation == nil || detail.Artifacts.Continuation.ContinuationID != continuation.ContinuationID {
				t.Fatalf("continuation artifact = %+v, want %s", detail.Artifacts.Continuation, continuation.ContinuationID)
			}
		},
		types.EventAppChangePackagePublished: func(detail traceMomentDetailResponse) {
			if detail.References.AppChangePackageID != appPackage.PackageID {
				t.Fatalf("app package reference = %q, want %q", detail.References.AppChangePackageID, appPackage.PackageID)
			}
			if detail.Artifacts.AppChangePackage == nil || detail.Artifacts.AppChangePackage.PackageID != appPackage.PackageID {
				t.Fatalf("app package artifact = %+v, want %s", detail.Artifacts.AppChangePackage, appPackage.PackageID)
			}
		},
		types.EventAppAdoptionProposed: func(detail traceMomentDetailResponse) {
			if detail.References.AppAdoptionID != appAdoption.AdoptionID {
				t.Fatalf("app adoption reference = %q, want %q", detail.References.AppAdoptionID, appAdoption.AdoptionID)
			}
			if detail.Artifacts.AppAdoption == nil || detail.Artifacts.AppAdoption.AdoptionID != appAdoption.AdoptionID {
				t.Fatalf("app adoption artifact = %+v, want %s", detail.Artifacts.AppAdoption, appAdoption.AdoptionID)
			}
		},
	} {
		moment := summaries[kind]
		detailReq := authenticatedRequest(http.MethodGet, "/api/trace/trajectories/"+parent.RunID+"/moments/"+moment.MomentID, "", "user-alice")
		detailW := httptest.NewRecorder()
		handler.HandleTraceTrajectories(detailW, detailReq)
		if detailW.Code != http.StatusOK {
			t.Fatalf("detail status for %s = %d, body=%s", kind, detailW.Code, detailW.Body.String())
		}
		var detail traceMomentDetailResponse
		if err := json.Unmarshal(detailW.Body.Bytes(), &detail); err != nil {
			t.Fatalf("decode detail for %s: %v", kind, err)
		}
		assertArtifact(detail)
	}
}

func TestBuildTraceSearchSummaryAggregatesProviderAttempts(t *testing.T) {
	events := []types.EventRecord{
		{
			Kind: types.EventToolResult,
			Payload: json.RawMessage(`{
				"tool":"web_search",
				"is_error":false,
				"output":"{\"provider\":\"tavily\",\"providers\":[\"tavily\"],\"attempts\":[{\"provider\":\"tavily\",\"endpoint\":\"https://api.tavily.com/search\",\"status\":\"success\",\"latency_ms\":120,\"results\":3},{\"provider\":\"brave\",\"endpoint\":\"https://api.search.brave.com/res/v1/web/search\",\"status\":\"rate_limited\",\"latency_ms\":40,\"results\":0,\"error\":\"429 too many requests\"}],\"results\":[{\"title\":\"AI news\",\"url\":\"https://example.com\",\"provider\":\"tavily\"}]}"
			}`),
		},
		{
			Kind: types.EventToolResult,
			Payload: json.RawMessage(`{
				"tool":"web_search",
				"is_error":false,
				"output":"{\"provider\":\"exa\",\"providers\":[\"exa\"],\"attempts\":[{\"provider\":\"exa\",\"endpoint\":\"https://api.exa.ai/search\",\"status\":\"success\",\"latency_ms\":80,\"results\":2}],\"results\":[{\"title\":\"AI research\",\"url\":\"https://example.org\",\"provider\":\"exa\"}]}"
			}`),
		},
	}

	summary := buildTraceSearchSummary(events)
	if summary.Queries != 2 || summary.Attempts != 3 || summary.Successes != 2 || summary.RateLimits != 1 {
		t.Fatalf("summary = %+v, want queries=2 attempts=3 successes=2 rate_limits=1", summary)
	}
	byProvider := make(map[string]traceSearchProviderStats)
	for _, provider := range summary.Providers {
		byProvider[provider.Provider] = provider
	}
	if byProvider["tavily"].ResultCount != 3 || byProvider["tavily"].AvgLatencyMs != 120 {
		t.Fatalf("tavily stats = %+v", byProvider["tavily"])
	}
	if byProvider["brave"].RateLimits != 1 || !strings.Contains(byProvider["brave"].LastError, "429") {
		t.Fatalf("brave stats = %+v", byProvider["brave"])
	}
	if byProvider["exa"].ResultCount != 2 {
		t.Fatalf("exa stats = %+v", byProvider["exa"])
	}
}

func TestHandleTraceMomentDetailReturnsMessageAndFindings(t *testing.T) {
	rt, handler := testAPISetup(t)

	parent, _ := seedTraceTrajectory(t, rt)

	snapshotReq := authenticatedRequest(http.MethodGet, "/api/trace/trajectories/"+parent.RunID, "", "user-alice")
	snapshotW := httptest.NewRecorder()
	handler.HandleTraceTrajectories(snapshotW, snapshotReq)
	if snapshotW.Code != http.StatusOK {
		t.Fatalf("snapshot status: got %d, want %d", snapshotW.Code, http.StatusOK)
	}
	var snapshot traceTrajectorySnapshotResponse
	if err := json.NewDecoder(snapshotW.Body).Decode(&snapshot); err != nil {
		t.Fatalf("decode snapshot: %v", err)
	}

	var target traceMomentSummary
	for _, moment := range snapshot.Moments {
		if moment.Kind == types.EventChannelMessage && strings.Contains(moment.Summary, "damp shade") {
			target = moment
			break
		}
	}
	if target.MomentID == "" {
		t.Fatal("expected research channel.message moment in snapshot")
	}

	detailReq := authenticatedRequest(http.MethodGet, "/api/trace/trajectories/"+parent.RunID+"/moments/"+target.MomentID, "", "user-alice")
	detailW := httptest.NewRecorder()
	handler.HandleTraceTrajectories(detailW, detailReq)
	if detailW.Code != http.StatusOK {
		t.Fatalf("detail status: got %d, want %d", detailW.Code, http.StatusOK)
	}

	var detail traceMomentDetailResponse
	if err := json.NewDecoder(detailW.Body).Decode(&detail); err != nil {
		t.Fatalf("decode detail: %v", err)
	}
	if len(detail.Messages) != 1 {
		t.Fatalf("messages: got %d, want 1", len(detail.Messages))
	}
	if !strings.Contains(detail.Messages[0].Content, "damp shade") {
		t.Fatalf("unexpected message content: %q", detail.Messages[0].Content)
	}
	if len(detail.Findings) != 1 {
		t.Fatalf("findings: got %d, want 1", len(detail.Findings))
	}
	if detail.Findings[0].FindingID == "" {
		t.Fatal("finding_id should not be empty")
	}
	if len(detail.References.EvidenceIDs) != 1 || detail.References.EvidenceIDs[0] != "ev-moss-1" {
		t.Fatalf("unexpected evidence ids: %+v", detail.References.EvidenceIDs)
	}
}

func TestHandleTraceTrajectoryEventsStreamFiltersByTrajectory(t *testing.T) {
	rt, handler := testAPISetup(t)

	parent, child := seedTraceTrajectory(t, rt)
	otherParent, err := rt.StartRunWithMetadata(context.Background(), "Other trajectory", "user-alice", map[string]any{
		runMetadataAgentProfile: "conductor",
		runMetadataAgentRole:    "conductor",
	})
	if err != nil {
		t.Fatalf("start other trajectory: %v", err)
	}

	req := authenticatedRequest(http.MethodGet, "/api/trace/trajectories/"+parent.RunID+"/events?after_seq=0", "", "user-alice")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	go handler.HandleTraceTrajectories(w, req)
	time.Sleep(50 * time.Millisecond)

	if _, err := rt.ChannelPost(WithToolExecutionContext(context.Background(), child), child.ChannelID, "researcher", "researcher", "Second moss finding"); err != nil {
		t.Fatalf("trajectory channel post: %v", err)
	}
	if _, err := rt.ChannelPost(WithToolExecutionContext(context.Background(), otherParent), otherParent.ChannelID, "conductor", "conductor", "Other trajectory noise"); err != nil {
		t.Fatalf("other trajectory channel post: %v", err)
	}

	time.Sleep(150 * time.Millisecond)
	cancel()

	body := w.Body.String()
	scanner := bufio.NewScanner(strings.NewReader(body))
	foundTarget := false
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var ev types.EventRecord
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &ev); err != nil {
			continue
		}
		if ev.TrajectoryID != parent.RunID {
			t.Fatalf("unexpected trajectory in stream: got %q, want %q", ev.TrajectoryID, parent.RunID)
		}
		if ev.Kind == types.EventChannelMessage {
			foundTarget = true
		}
	}
	if !foundTarget {
		t.Fatal("expected channel.message event in trajectory stream")
	}
}
