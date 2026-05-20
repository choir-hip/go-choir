package runtime

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/server"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// testAPISetup creates a fresh Runtime and APIHandler for HTTP handler tests.
func testAPISetup(t *testing.T) (*Runtime, *APIHandler) {
	t.Helper()

	dir := filepath.Join(os.TempDir(), "go-choir-m3-api-test")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	dbPath := filepath.Join(dir, t.Name()+".db")
	promptRoot := filepath.Join(dir, t.Name()+"-prompts")
	_ = os.Remove(dbPath)
	_ = os.RemoveAll(promptRoot)

	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	bus := events.NewEventBus()
	provider := NewStubProvider(50 * time.Millisecond)
	cfg := Config{
		SandboxID:           "sandbox-test",
		StorePath:           dbPath,
		PromptRoot:          promptRoot,
		ProviderTimeout:     time.Second,
		SupervisionInterval: 1 * time.Hour,
	}

	rt := New(cfg, s, bus, provider)
	handler := NewAPIHandler(rt)

	// Stop the runtime before closing the store to avoid "database is
	// closed" log noise from in-flight goroutines.
	t.Cleanup(func() {
		rt.Stop()
		_ = s.Close()
		_ = os.Remove(dbPath)
		_ = os.RemoveAll(promptRoot)
	})

	return rt, handler
}

// authenticatedRequest creates an HTTP request with the X-Authenticated-User
// header set, simulating the proxy's user-context injection.
func authenticatedRequest(method, path, body, user string) *http.Request {
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	if user != "" {
		req.Header.Set("X-Authenticated-User", user)
	}
	return req
}

func registeredRuntimeRequest(t *testing.T, handler *APIHandler, method, path, body, user string) *httptest.ResponseRecorder {
	t.Helper()
	srv := server.NewServer("runtime-api-test", "0")
	RegisterRoutes(srv, handler)
	req := authenticatedRequest(method, path, body, user)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	return w
}

// --- Task Submission Tests ---

func TestHandlePromptBarCreatesServerOwnedConductorRun(t *testing.T) {
	rt, handler := testAPISetup(t)

	req := authenticatedRequest(http.MethodPost, "/api/prompt-bar", `{"text":"Draft a research plan"}`, "user-alice")
	w := httptest.NewRecorder()

	handler.HandlePromptBar(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("status: got %d, want %d; body=%s", w.Code, http.StatusAccepted, w.Body.String())
	}

	var resp promptBarSubmitResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.SubmissionID == "" {
		t.Fatal("submission_id should not be empty")
	}
	if resp.StatusURL != "/api/prompt-bar/submissions/"+resp.SubmissionID {
		t.Fatalf("status_url: got %q", resp.StatusURL)
	}

	rec, err := rt.GetRun(context.Background(), resp.SubmissionID, "user-alice")
	if err != nil {
		t.Fatalf("get run: %v", err)
	}
	if rec.AgentProfile != AgentProfileConductor || rec.AgentRole != AgentProfileConductor {
		t.Fatalf("server-owned conductor identity not set: profile=%q role=%q", rec.AgentProfile, rec.AgentRole)
	}
	if got := metadataStringValue(rec.Metadata, "input_source"); got != "prompt_bar" {
		t.Fatalf("input_source: got %q, want prompt_bar", got)
	}
	if got := metadataStringValue(rec.Metadata, "requested_app"); got != AgentProfileVText {
		t.Fatalf("requested_app: got %q, want %q", got, AgentProfileVText)
	}
	if got := metadataStringValue(rec.Metadata, "seed_prompt"); got != "Draft a research plan" {
		t.Fatalf("seed_prompt: got %q", got)
	}
	var decision conductorDecision
	if err := json.Unmarshal([]byte(rec.Result), &decision); err != nil {
		t.Fatalf("decode prompt-bar decision: %v\n%s", err, rec.Result)
	}
	if decision.Action != "open_app" || decision.App != AgentProfileVText || decision.DocID == "" {
		t.Fatalf("prompt-bar decision = %+v, want immediate vtext route", decision)
	}
	if !strings.Contains(decision.InitialContent, "Draft a research plan") {
		t.Fatalf("initial_content = %q, want prompt-derived content", decision.InitialContent)
	}

	statusReq := authenticatedRequest(http.MethodGet, "/api/prompt-bar/submissions/"+resp.SubmissionID, "", "user-alice")
	statusW := httptest.NewRecorder()
	handler.HandlePromptBarSubmission(statusW, statusReq)
	if statusW.Code != http.StatusOK {
		t.Fatalf("status endpoint: got %d, want 200; body=%s", statusW.Code, statusW.Body.String())
	}
	var status promptBarSubmissionStatusResponse
	if err := json.NewDecoder(statusW.Body).Decode(&status); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if status.Decision == nil || status.Decision.DocID != decision.DocID {
		t.Fatalf("status decision = %+v, want doc_id %q", status.Decision, decision.DocID)
	}
	superAgent, err := rt.store.GetAgent(context.Background(), persistentSuperAgentID("user-alice"))
	if err != nil {
		t.Fatalf("persistent super agent missing: %v", err)
	}
	if superAgent.Profile != AgentProfileSuper || superAgent.Role != AgentProfileSuper {
		t.Fatalf("persistent super identity = %q/%q, want %q/%q", superAgent.Profile, superAgent.Role, AgentProfileSuper, AgentProfileSuper)
	}
}

func TestHandlePromptBarRejectsBrowserRuntimeMetadata(t *testing.T) {
	_, handler := testAPISetup(t)

	body := `{"text":"do work","metadata":{"agent_profile":"super"},"agent_role":"super","model":"x"}`
	req := authenticatedRequest(http.MethodPost, "/api/prompt-bar", body, "user-alice")
	w := httptest.NewRecorder()

	handler.HandlePromptBar(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestRunAcceptanceSynthesizeDerivesExportLevelRecord(t *testing.T) {
	rt, handler := testAPISetup(t)
	ctx := context.Background()
	seedRunAcceptanceTrajectory(t, rt)

	body := `{"target_mission_id":"mission-run-acceptance-v0","trajectory_id":"traj-acceptance"}`
	w := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/run-acceptances/synthesize", body, "user-alice")
	if w.Code != http.StatusAccepted {
		t.Fatalf("synthesize status = %d, body=%s", w.Code, w.Body.String())
	}
	var rec types.RunAcceptanceRecord
	if err := json.Unmarshal(w.Body.Bytes(), &rec); err != nil {
		t.Fatalf("decode acceptance: %v", err)
	}
	if rec.AcceptanceLevel != types.RunAcceptanceExportLevel {
		t.Fatalf("acceptance level = %q, want export-level; record=%+v", rec.AcceptanceLevel, rec)
	}
	if rec.State != types.RunAcceptanceAccepted {
		t.Fatalf("state = %q, want accepted", rec.State)
	}
	for _, want := range []string{"submitted", "vtext_opened", "super_requested", "worker_leased", "worker_delegated", "app_package_published", "app_adoption_verified"} {
		if !acceptanceHasCheckpoint(rec, want) {
			t.Fatalf("missing checkpoint %q in %+v", want, rec.Checkpoints)
		}
	}
	if rec.BaseSHA != "base-acceptance" {
		t.Fatalf("base sha = %q", rec.BaseSHA)
	}
	if rec.VMMode != "worker" {
		t.Fatalf("vm mode = %q", rec.VMMode)
	}
	if rec.GatewayProviderEvidence == "" || !strings.Contains(rec.GatewayProviderEvidence, "active_provider=") {
		t.Fatalf("gateway provider evidence missing: %q", rec.GatewayProviderEvidence)
	}
	if len(rec.EvidenceRefs) < 5 {
		t.Fatalf("expected structured evidence refs, got %+v", rec.EvidenceRefs)
	}
	delegated := acceptanceCheckpoint(rec, "worker_delegated", "passed")
	if delegated == nil {
		t.Fatalf("missing passed worker_delegated checkpoint: %+v", rec.Checkpoints)
	}
	if got, _ := delegated.Details["state"].(string); got != "completed" {
		t.Fatalf("worker_delegated state = %q, want completed; details=%+v", got, delegated.Details)
	}
	if delegated.Details["worker_child_run_ids"] == nil {
		t.Fatalf("worker_delegated missing child run ids: %+v", delegated.Details)
	}
	if delegated.Details["worker_event_summary"] == nil {
		t.Fatalf("worker_delegated missing worker event summary: %+v", delegated.Details)
	}
	if delegated.Details["app_change_packages"] == nil {
		t.Fatalf("worker_delegated missing AppChangePackage summaries: %+v", delegated.Details)
	}

	loaded, err := rt.store.GetRunAcceptance(ctx, "user-alice", rec.AcceptanceID)
	if err != nil {
		t.Fatalf("acceptance not durable: %v", err)
	}
	if loaded.AcceptanceID != rec.AcceptanceID {
		t.Fatalf("loaded acceptance id mismatch: %+v", loaded)
	}

	listW := registeredRuntimeRequest(t, handler, http.MethodGet, "/api/run-acceptances?trajectory_id=traj-acceptance", "", "user-alice")
	if listW.Code != http.StatusOK {
		t.Fatalf("list status = %d, body=%s", listW.Code, listW.Body.String())
	}
	var list runAcceptanceListResponse
	if err := json.Unmarshal(listW.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(list.Acceptances) != 1 || list.Acceptances[0].AcceptanceID != rec.AcceptanceID {
		t.Fatalf("list response mismatch: %+v", list)
	}

	otherW := registeredRuntimeRequest(t, handler, http.MethodGet, "/api/run-acceptances/"+rec.AcceptanceID, "", "user-bob")
	if otherW.Code != http.StatusNotFound {
		t.Fatalf("other owner status = %d, want 404", otherW.Code)
	}
}

func TestRunAcceptanceSynthesizeCountsTimedOutDelegateWithReviewableExport(t *testing.T) {
	rt, handler := testAPISetup(t)
	seedRunAcceptanceExportedTimeoutTrajectory(t, rt)

	body := `{"target_mission_id":"mission-run-acceptance-timeout-export","trajectory_id":"traj-acceptance"}`
	w := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/run-acceptances/synthesize", body, "user-alice")
	if w.Code != http.StatusAccepted {
		t.Fatalf("synthesize status = %d, body=%s", w.Code, w.Body.String())
	}
	var rec types.RunAcceptanceRecord
	if err := json.Unmarshal(w.Body.Bytes(), &rec); err != nil {
		t.Fatalf("decode acceptance: %v", err)
	}
	if rec.AcceptanceLevel != types.RunAcceptanceExportLevel || rec.State != types.RunAcceptanceAccepted {
		t.Fatalf("acceptance = %s/%s, want export-level/accepted; checkpoints=%+v", rec.AcceptanceLevel, rec.State, rec.Checkpoints)
	}
	delegated := acceptanceCheckpoint(rec, "worker_delegated", "passed")
	if delegated == nil {
		t.Fatalf("missing passed worker_delegated checkpoint: %+v", rec.Checkpoints)
	}
	if got, _ := delegated.Details["non_clean_delegate_status"].(string); got != "worker_run_timeout" {
		t.Fatalf("non-clean status = %q, want worker_run_timeout; details=%+v", got, delegated.Details)
	}
	if !strings.Contains(strings.Join(rec.FailureResidualRisks, "\n"), "non-clean status worker_run_timeout") {
		t.Fatalf("missing non-clean export residual risk: %+v", rec.FailureResidualRisks)
	}
}

func TestRunAcceptanceSynthesizeRecordsWorkerDelegateBlocker(t *testing.T) {
	rt, handler := testAPISetup(t)
	seedRunAcceptanceBlockedDelegationTrajectory(t, rt)

	body := `{"target_mission_id":"mission-run-acceptance-blocked","trajectory_id":"traj-acceptance"}`
	w := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/run-acceptances/synthesize", body, "user-alice")
	if w.Code != http.StatusAccepted {
		t.Fatalf("synthesize status = %d, body=%s", w.Code, w.Body.String())
	}
	var rec types.RunAcceptanceRecord
	if err := json.Unmarshal(w.Body.Bytes(), &rec); err != nil {
		t.Fatalf("decode acceptance: %v", err)
	}
	if rec.AcceptanceLevel != types.RunAcceptanceStagingSmokeLevel {
		t.Fatalf("acceptance level = %q, want staging-smoke-level; record=%+v", rec.AcceptanceLevel, rec)
	}
	if rec.State != types.RunAcceptanceBlocked {
		t.Fatalf("state = %q, want blocked", rec.State)
	}
	if acceptanceHasCheckpoint(rec, "worker_delegated") {
		t.Fatalf("failed delegation must not count as passed: %+v", rec.Checkpoints)
	}
	blocked := acceptanceCheckpoint(rec, "worker_delegated", "blocked")
	if blocked == nil {
		t.Fatalf("missing blocked worker_delegated checkpoint: %+v", rec.Checkpoints)
	}
	if len(blocked.EvidenceRefIDs) != 2 {
		t.Fatalf("blocked checkpoint evidence refs = %+v, want 2 refs", blocked.EvidenceRefIDs)
	}
	lastError, _ := blocked.Details["last_error"].(string)
	if !strings.Contains(lastError, "runtime restarted") {
		t.Fatalf("last_error = %q, want runtime restart detail", lastError)
	}
	if !strings.Contains(strings.Join(rec.FailureResidualRisks, "\n"), "worker VM delegation did not complete") {
		t.Fatalf("missing delegation residual risk: %+v", rec.FailureResidualRisks)
	}
	for _, check := range rec.InvariantChecks {
		if check.Name == "worker_mutation_bounded" && check.State != "blocked" {
			t.Fatalf("worker_mutation_bounded = %+v, want blocked", check)
		}
	}
}

func TestRunAcceptanceSynthesizePreservesStructuredFailedAndPendingDelegateEvidence(t *testing.T) {
	rt, handler := testAPISetup(t)
	seedRunAcceptanceBlockedDelegationTrajectory(t, rt)
	now := time.Now().UTC()
	appendAcceptanceToolResult(t, rt, "event-delegate-structured-failed", "run-super-acceptance", "agent-super-acceptance", now.Add(20*time.Second), "delegate_worker_vm", map[string]any{
		"status":                       "worker_run_failed",
		"state":                        "failed",
		"worker_id":                    "worker-acceptance",
		"worker_vm_id":                 "vm-acceptance",
		"worker_sandbox_url":           "http://127.0.0.1:8085",
		"loop_id":                      "run-worker-acceptance",
		"error":                        "tool loop: exceeded 200 iterations without end_turn",
		"terminal_error":               "worker run run-worker-acceptance ended in state failed: tool loop: exceeded 200 iterations without end_turn",
		"completion_blocker":           "vsuper_completed_without_export_or_worker_update",
		"event_count":                  3,
		"worker_event_summary":         []map[string]any{{"kind": "tool.result", "tool": "submit_worker_update", "output_excerpt": "precise blocker"}},
		"worker_spawned_profiles":      []string{"co-super"},
		"worker_child_run_ids":         []string{"run-implementation-acceptance"},
		"worker_child_run_states":      map[string]string{"run-implementation-acceptance": "completed"},
		"worker_child_status_errors":   map[string]string{"run-verifier-acceptance": "status unavailable"},
		"worker_channel_message_count": 1,
	})
	appendAcceptanceToolInvoked(t, rt, "event-delegate-invoked-after-fail", "run-super-acceptance", "agent-super-acceptance", "traj-acceptance", "channel-acceptance", now.Add(21*time.Second), "delegate_worker_vm", "call-pending-after-fail", map[string]any{
		"worker_id":          "worker-after-fail",
		"vm_id":              "vm-after-fail",
		"worker_sandbox_url": "http://127.0.0.1:8086",
		"profile":            AgentProfileVSuper,
		"objective":          "retry after failed worker",
	})

	body := `{"target_mission_id":"mission-run-acceptance-structured-failed","trajectory_id":"traj-acceptance"}`
	w := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/run-acceptances/synthesize", body, "user-alice")
	if w.Code != http.StatusAccepted {
		t.Fatalf("synthesize status = %d, body=%s", w.Code, w.Body.String())
	}
	var rec types.RunAcceptanceRecord
	if err := json.Unmarshal(w.Body.Bytes(), &rec); err != nil {
		t.Fatalf("decode acceptance: %v", err)
	}
	blocked := acceptanceCheckpoint(rec, "worker_delegated", "blocked")
	if blocked == nil {
		t.Fatalf("missing blocked worker_delegated checkpoint: %+v", rec.Checkpoints)
	}
	if len(blocked.EvidenceRefIDs) != 4 {
		t.Fatalf("blocked evidence refs = %+v, want structured result + two errors + pending invocation", blocked.EvidenceRefIDs)
	}
	if got, _ := blocked.Details["result_count"].(float64); got != 1 {
		t.Fatalf("result_count = %v, want 1; details=%+v", blocked.Details["result_count"], blocked.Details)
	}
	if got, _ := blocked.Details["error_count"].(float64); got != 2 {
		t.Fatalf("error_count = %v, want 2; details=%+v", blocked.Details["error_count"], blocked.Details)
	}
	if got, _ := blocked.Details["pending_invocation_count"].(float64); got != 1 {
		t.Fatalf("pending_invocation_count = %v, want 1; details=%+v", blocked.Details["pending_invocation_count"], blocked.Details)
	}
	if got, _ := blocked.Details["status"].(string); got != "worker_run_failed" {
		t.Fatalf("status = %q, want worker_run_failed; details=%+v", got, blocked.Details)
	}
	if got, _ := blocked.Details["pending_status"].(string); got != "invoked_without_terminal_result" {
		t.Fatalf("pending_status = %q, want invoked_without_terminal_result; details=%+v", got, blocked.Details)
	}
	if blocked.Details["worker_event_summary"] == nil {
		t.Fatalf("missing worker_event_summary in blocked details: %+v", blocked.Details)
	}
	if got, _ := blocked.Details["completion_blocker"].(string); got != "vsuper_completed_without_export_or_worker_update" {
		t.Fatalf("completion_blocker = %q; details=%+v", got, blocked.Details)
	}
	if blocked.Details["worker_child_run_states"] == nil || blocked.Details["worker_child_status_errors"] == nil {
		t.Fatalf("missing child status provenance in blocked details: %+v", blocked.Details)
	}
}

func TestRunAcceptanceSynthesizeRecordsPendingWorkerDelegateInvocation(t *testing.T) {
	rt, handler := testAPISetup(t)
	seedRunAcceptancePendingDelegationTrajectory(t, rt)

	body := `{"target_mission_id":"mission-run-acceptance-pending-delegate","trajectory_id":"traj-pending-delegate"}`
	w := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/run-acceptances/synthesize", body, "user-alice")
	if w.Code != http.StatusAccepted {
		t.Fatalf("synthesize status = %d, body=%s", w.Code, w.Body.String())
	}
	var rec types.RunAcceptanceRecord
	if err := json.Unmarshal(w.Body.Bytes(), &rec); err != nil {
		t.Fatalf("decode acceptance: %v", err)
	}
	if rec.AcceptanceLevel != types.RunAcceptanceStagingSmokeLevel {
		t.Fatalf("acceptance level = %q, want staging-smoke-level; record=%+v", rec.AcceptanceLevel, rec)
	}
	if rec.State != types.RunAcceptanceBlocked {
		t.Fatalf("state = %q, want blocked", rec.State)
	}
	blocked := acceptanceCheckpoint(rec, "worker_delegated", "blocked")
	if blocked == nil {
		t.Fatalf("missing blocked worker_delegated checkpoint: %+v", rec.Checkpoints)
	}
	if got, _ := blocked.Details["status"].(string); got != "invoked_without_terminal_result" {
		t.Fatalf("blocked status = %q, want invoked_without_terminal_result", got)
	}
	if got, _ := blocked.Details["worker_id"].(string); got != "worker-pending" {
		t.Fatalf("blocked worker_id = %q, want worker-pending", got)
	}
	if !strings.Contains(strings.Join(rec.FailureResidualRisks, "\n"), "worker VM delegation did not complete") {
		t.Fatalf("missing delegation residual risk: %+v", rec.FailureResidualRisks)
	}
}

func TestRunAcceptanceSynthesizeRequiresAdoptionPromotionForPromotionLevel(t *testing.T) {
	rt, handler := testAPISetup(t)
	seedRunAcceptanceTrajectory(t, rt)

	body := `{"target_mission_id":"mission-run-acceptance-v0","trajectory_id":"traj-acceptance"}`
	w := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/run-acceptances/synthesize", body, "user-alice")
	if w.Code != http.StatusAccepted {
		t.Fatalf("synthesize status = %d, body=%s", w.Code, w.Body.String())
	}
	var rec types.RunAcceptanceRecord
	if err := json.Unmarshal(w.Body.Bytes(), &rec); err != nil {
		t.Fatalf("decode acceptance: %v", err)
	}
	if rec.AcceptanceLevel != types.RunAcceptanceExportLevel {
		t.Fatalf("acceptance level before app adoption promotion = %q, want export-level", rec.AcceptanceLevel)
	}
	if !acceptanceHasCheckpoint(rec, "app_adoption_verified") {
		t.Fatalf("verified app adoption should create verifier checkpoint: %+v", rec.Checkpoints)
	}
	if acceptanceHasCheckpoint(rec, "app_adoption_promoted") {
		t.Fatalf("promotion checkpoint should not be present before durable adoption promotion: %+v", rec.Checkpoints)
	}

	appendAcceptanceEvent(t, rt, types.EventRecord{
		EventID:      "event-app-adoption-promoted-acceptance",
		RunID:        "run-worker-acceptance",
		AgentID:      "agent-super-acceptance",
		ChannelID:    "channel-acceptance",
		OwnerID:      "user-alice",
		TrajectoryID: "traj-acceptance",
		Timestamp:    time.Now().UTC(),
		Kind:         types.EventAppAdoptionPromoted,
		Payload:      json.RawMessage(`{"adoption_id":"adoption-acceptance","package_id":"pkg-acceptance","target_computer_id":"computer-b","candidate_source_ref":"refs/computers/computer-b/candidates/adoption-acceptance","runtime_artifact_digest":"runtime-recipient-digest-b","ui_artifact_digest":"ui-recipient-digest-b","route_profile":"primary","default_base_profile":"primary","rollback_source_ref":"refs/computers/computer-b/active-before-adoption"}`),
	})
	w = registeredRuntimeRequest(t, handler, http.MethodPost, "/api/run-acceptances/synthesize", body, "user-alice")
	if w.Code != http.StatusAccepted {
		t.Fatalf("synthesize promoted status = %d, body=%s", w.Code, w.Body.String())
	}
	if err := json.Unmarshal(w.Body.Bytes(), &rec); err != nil {
		t.Fatalf("decode promoted acceptance: %v", err)
	}
	if rec.AcceptanceLevel != types.RunAcceptancePromotionLevel {
		t.Fatalf("acceptance level with adoption promotion = %q, want promotion-level; checkpoints=%+v", rec.AcceptanceLevel, rec.Checkpoints)
	}
	if !acceptanceHasCheckpoint(rec, "app_adoption_promoted") {
		t.Fatalf("promoted adoption missing app_adoption_promoted checkpoint: %+v", rec.Checkpoints)
	}
}

func TestRunAcceptanceSynthesizeBlocksAcceptedStateWhenInvariantsAreBlocked(t *testing.T) {
	rt, handler := testAPISetup(t)
	now := time.Now().UTC()
	for i, ev := range []types.EventRecord{
		{
			EventID:      "event-direct-package",
			OwnerID:      "user-alice",
			TrajectoryID: "traj-direct-package",
			Timestamp:    now,
			Kind:         types.EventAppChangePackagePublished,
			Payload:      json.RawMessage(`{"package_id":"pkg-direct","app_id":"podcast","source_computer_id":"computer-a","source_candidate_id":"candidate-a","candidate_source_ref":"refs/computers/computer-a/candidates/candidate-a","package_manifest_sha":"sha256-manifest-direct"}`),
		},
		{
			EventID:      "event-direct-adoption-verified",
			OwnerID:      "user-alice",
			TrajectoryID: "traj-direct-package",
			Timestamp:    now.Add(time.Second),
			Kind:         types.EventAppAdoptionVerified,
			Payload:      json.RawMessage(`{"adoption_id":"adoption-direct","package_id":"pkg-direct","target_computer_id":"computer-b","runtime_artifact_digest":"runtime-recipient-digest-b","ui_artifact_digest":"ui-recipient-digest-b","foreground_tail_merge_result":"no-conflict"}`),
		},
		{
			EventID:      "event-direct-adoption-promoted",
			OwnerID:      "user-alice",
			TrajectoryID: "traj-direct-package",
			Timestamp:    now.Add(2 * time.Second),
			Kind:         types.EventAppAdoptionPromoted,
			Payload:      json.RawMessage(`{"adoption_id":"adoption-direct","package_id":"pkg-direct","target_computer_id":"computer-b","candidate_source_ref":"refs/computers/computer-b/candidates/adoption-direct","runtime_artifact_digest":"runtime-recipient-digest-b","ui_artifact_digest":"ui-recipient-digest-b","route_profile":"primary","rollback_source_ref":"refs/computers/computer-b/active-before-adoption"}`),
		},
	} {
		ev.StreamSeq = int64(i + 1)
		appendAcceptanceEvent(t, rt, ev)
	}

	body := `{"target_mission_id":"mission-direct-package","trajectory_id":"traj-direct-package"}`
	w := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/run-acceptances/synthesize", body, "user-alice")
	if w.Code != http.StatusAccepted {
		t.Fatalf("synthesize status = %d, body=%s", w.Code, w.Body.String())
	}
	var rec types.RunAcceptanceRecord
	if err := json.Unmarshal(w.Body.Bytes(), &rec); err != nil {
		t.Fatalf("decode acceptance: %v", err)
	}
	if rec.AcceptanceLevel != types.RunAcceptancePromotionLevel {
		t.Fatalf("acceptance level = %q, want promotion-level; checkpoints=%+v", rec.AcceptanceLevel, rec.Checkpoints)
	}
	if rec.State != types.RunAcceptanceBlocked {
		t.Fatalf("state = %q, want blocked because invariants are blocked; invariants=%+v", rec.State, rec.InvariantChecks)
	}
	for _, want := range []string{"product_path_observed", "worker_mutation_bounded"} {
		found := false
		for _, check := range rec.InvariantChecks {
			if check.Name == want {
				found = true
				if check.State != "blocked" {
					t.Fatalf("%s state = %q, want blocked", want, check.State)
				}
			}
		}
		if !found {
			t.Fatalf("missing invariant check %q in %+v", want, rec.InvariantChecks)
		}
	}
	if !strings.Contains(strings.Join(rec.FailureResidualRisks, "\n"), "acceptance invariant product_path_observed is blocked") {
		t.Fatalf("missing invariant residual risk: %+v", rec.FailureResidualRisks)
	}
}

func seedRunAcceptanceTrajectory(t *testing.T, rt *Runtime) {
	seedRunAcceptanceTrajectoryWithDelegateStatus(t, rt, "worker_run_completed", string(types.RunCompleted), "")
}

func seedRunAcceptanceExportedTimeoutTrajectory(t *testing.T, rt *Runtime) {
	seedRunAcceptanceTrajectoryWithDelegateStatus(t, rt, "worker_run_timeout", string(types.RunRunning), "worker run run-worker-acceptance did not finish within 15m0s; last state=running")
}

func seedRunAcceptanceTrajectoryWithDelegateStatus(t *testing.T, rt *Runtime, delegateStatus, delegateState, terminalError string) {
	t.Helper()
	ctx := context.Background()
	now := time.Now().UTC()
	finishedAt := now.Add(15 * time.Second)
	runs := []types.RunRecord{
		{
			RunID:        "run-conductor-acceptance",
			AgentID:      "agent-conductor-acceptance",
			ChannelID:    "channel-acceptance",
			AgentProfile: AgentProfileConductor,
			AgentRole:    AgentProfileConductor,
			OwnerID:      "user-alice",
			SandboxID:    "sandbox-test",
			State:        types.RunCompleted,
			Prompt:       "Build a tiny Choir-in-Choir verifier patch.",
			Result:       `{"action":"open_app","app":"vtext","doc_id":"doc-acceptance"}`,
			CreatedAt:    now,
			UpdatedAt:    finishedAt,
			FinishedAt:   &finishedAt,
			Metadata: map[string]any{
				runMetadataAgentProfile: AgentProfileConductor,
				runMetadataAgentRole:    AgentProfileConductor,
				runMetadataTrajectoryID: "traj-acceptance",
				runMetadataDesktopID:    types.PrimaryDesktopID,
				"input_source":          "prompt_bar",
			},
		},
		{
			RunID:        "run-vtext-acceptance",
			AgentID:      "agent-vtext-acceptance",
			ChannelID:    "channel-acceptance",
			ParentRunID:  "run-conductor-acceptance",
			AgentProfile: AgentProfileVText,
			AgentRole:    AgentProfileVText,
			OwnerID:      "user-alice",
			SandboxID:    "sandbox-test",
			State:        types.RunCompleted,
			Prompt:       "Own the acceptance document.",
			CreatedAt:    now.Add(3 * time.Second),
			UpdatedAt:    now.Add(4 * time.Second),
			FinishedAt:   &finishedAt,
			Metadata: map[string]any{
				runMetadataAgentProfile: AgentProfileVText,
				runMetadataAgentRole:    AgentProfileVText,
				runMetadataTrajectoryID: "traj-acceptance",
				runMetadataDesktopID:    types.PrimaryDesktopID,
			},
		},
		{
			RunID:        "run-super-acceptance",
			AgentID:      "agent-super-acceptance",
			ChannelID:    "channel-acceptance",
			ParentRunID:  "run-vtext-acceptance",
			AgentProfile: AgentProfileSuper,
			AgentRole:    AgentProfileSuper,
			OwnerID:      "user-alice",
			SandboxID:    "sandbox-test",
			State:        types.RunCompleted,
			Prompt:       "Delegate a worker and publish an AppChangePackage.",
			CreatedAt:    now.Add(5 * time.Second),
			UpdatedAt:    now.Add(12 * time.Second),
			FinishedAt:   &finishedAt,
			Metadata: map[string]any{
				runMetadataAgentProfile: AgentProfileSuper,
				runMetadataAgentRole:    AgentProfileSuper,
				runMetadataTrajectoryID: "traj-acceptance",
				runMetadataDesktopID:    types.PrimaryDesktopID,
			},
		},
	}
	for _, run := range runs {
		if err := rt.store.CreateRun(ctx, run); err != nil {
			t.Fatalf("create run %s: %v", run.RunID, err)
		}
	}

	appendAcceptanceEvent(t, rt, types.EventRecord{
		EventID:      "event-submit-acceptance",
		RunID:        "run-conductor-acceptance",
		AgentID:      "agent-conductor-acceptance",
		ChannelID:    "channel-acceptance",
		OwnerID:      "user-alice",
		TrajectoryID: "traj-acceptance",
		Timestamp:    now,
		Kind:         types.EventRunSubmitted,
		Payload:      json.RawMessage(`{"input_source":"prompt_bar"}`),
	})
	appendAcceptanceEvent(t, rt, types.EventRecord{
		EventID:      "event-vtext-acceptance",
		RunID:        "run-vtext-acceptance",
		AgentID:      "agent-vtext-acceptance",
		ChannelID:    "channel-acceptance",
		OwnerID:      "user-alice",
		TrajectoryID: "traj-acceptance",
		Timestamp:    now.Add(4 * time.Second),
		Kind:         types.EventVTextDocumentRevisionCreated,
		Payload:      json.RawMessage(`{"doc_id":"doc-acceptance","revision_id":"rev-1"}`),
	})
	appendAcceptanceToolResult(t, rt, "event-super-acceptance", "run-vtext-acceptance", "agent-vtext-acceptance", now.Add(5*time.Second), "request_super_execution", map[string]any{
		"agent_id": "agent-super-acceptance",
		"loop_id":  "run-super-acceptance",
		"state":    "running",
	})
	appendAcceptanceToolResult(t, rt, "event-worker-lease-acceptance", "run-super-acceptance", "agent-super-acceptance", now.Add(6*time.Second), "request_worker_vm", map[string]any{
		"status": "worker_requested",
		"handle": map[string]any{
			"kind":          "worker",
			"worker_id":     "worker-acceptance",
			"vm_id":         "vm-acceptance",
			"desktop_id":    types.PrimaryDesktopID,
			"machine_class": "standard",
			"sandbox_url":   "http://127.0.0.1:8085",
		},
	})
	appendAcceptanceToolResult(t, rt, "event-delegate-acceptance", "run-super-acceptance", "agent-super-acceptance", now.Add(10*time.Second), "delegate_worker_vm", map[string]any{
		"status":                       delegateStatus,
		"state":                        delegateState,
		"worker_vm_id":                 "vm-acceptance",
		"worker_id":                    "worker-acceptance",
		"worker_sandbox_url":           "http://127.0.0.1:8085",
		"loop_id":                      "run-worker-acceptance",
		"terminal_error":               terminalError,
		"completion_blocker":           map[bool]string{true: "vsuper_timed_out_after_reviewable_package", false: ""}[delegateStatus != "worker_run_completed"],
		"event_count":                  22,
		"worker_root_event_count":      9,
		"worker_child_run_ids":         []string{"run-implementation-acceptance", "run-verifier-acceptance"},
		"worker_child_event_counts":    map[string]int{"run-implementation-acceptance": 8, "run-verifier-acceptance": 5},
		"worker_channel_message_count": 4,
		"worker_spawned_profiles":      []string{AgentProfileCoSuper},
		"worker_event_summary": []map[string]any{
			{
				"kind":           "tool.result",
				"tool":           "spawn_agent",
				"output_excerpt": `{"agent_id":"agent-implementation-acceptance","loop_id":"run-implementation-acceptance","channel_id":"channel-implementation-acceptance","profile":"co-super","state":"completed"}`,
			},
			{
				"kind":            "channel.message",
				"role":            "result",
				"from_agent_id":   "agent-verifier-acceptance",
				"to_agent_id":     "agent-vsuper-acceptance",
				"content_excerpt": "Verifier observed the AppChangePackage manifest and rollback refs.",
			},
		},
		"app_change_packages": []map[string]any{{
			"status":                         "published_unlisted",
			"package_id":                     "pkg-acceptance",
			"app_id":                         "podcast",
			"base_sha":                       "base-acceptance",
			"candidate_head_sha":             "worker-head-acceptance",
			"source_computer_id":             "computer-a",
			"source_candidate_id":            "candidate-a",
			"candidate_source_ref":           "refs/computers/computer-a/candidates/candidate-a",
			"package_manifest_sha256":        "sha256-manifest-acceptance",
			"runtime_source_delta_sha256":    "sha256-runtime-delta-acceptance",
			"ui_source_delta_sha256":         "sha256-ui-delta-acceptance",
			"recipient_build_required":       true,
			"source_runtime_artifact_digest": "runtime-source-digest-a",
			"source_ui_artifact_digest":      "ui-source-digest-a",
		}},
	})
	appendAcceptanceEvent(t, rt, types.EventRecord{
		EventID:      "event-app-package-acceptance",
		RunID:        "run-worker-acceptance",
		AgentID:      "agent-super-acceptance",
		ChannelID:    "channel-acceptance",
		OwnerID:      "user-alice",
		TrajectoryID: "traj-acceptance",
		Timestamp:    now.Add(11 * time.Second),
		Kind:         types.EventAppChangePackagePublished,
		Payload:      json.RawMessage(`{"package_id":"pkg-acceptance","app_id":"podcast","source_computer_id":"computer-a","source_candidate_id":"candidate-a","candidate_source_ref":"refs/computers/computer-a/candidates/candidate-a","package_manifest_sha":"sha256-manifest-acceptance"}`),
	})
	appendAcceptanceEvent(t, rt, types.EventRecord{
		EventID:      "event-app-adoption-verify-acceptance",
		RunID:        "run-worker-acceptance",
		AgentID:      "agent-super-acceptance",
		ChannelID:    "channel-acceptance",
		OwnerID:      "user-alice",
		TrajectoryID: "traj-acceptance",
		Timestamp:    now.Add(12 * time.Second),
		Kind:         types.EventAppAdoptionVerified,
		Payload:      json.RawMessage(`{"adoption_id":"adoption-acceptance","package_id":"pkg-acceptance","target_computer_id":"computer-b","runtime_artifact_digest":"runtime-recipient-digest-b","ui_artifact_digest":"ui-recipient-digest-b","foreground_tail_merge_result":"no-conflict","recipient_build_required":true,"recipient_build_status":"passed"}`),
	})
}

func seedRunAcceptanceBlockedDelegationTrajectory(t *testing.T, rt *Runtime) {
	t.Helper()
	ctx := context.Background()
	now := time.Now().UTC()
	finishedAt := now.Add(15 * time.Second)
	runs := []types.RunRecord{
		{
			RunID:        "run-conductor-acceptance",
			AgentID:      "agent-conductor-acceptance",
			ChannelID:    "channel-acceptance",
			AgentProfile: AgentProfileConductor,
			AgentRole:    AgentProfileConductor,
			OwnerID:      "user-alice",
			SandboxID:    "sandbox-test",
			State:        types.RunCompleted,
			Prompt:       "Build a tiny Choir-in-Choir verifier patch.",
			Result:       `{"action":"open_app","app":"vtext","doc_id":"doc-acceptance"}`,
			CreatedAt:    now,
			UpdatedAt:    finishedAt,
			FinishedAt:   &finishedAt,
			Metadata: map[string]any{
				runMetadataAgentProfile: AgentProfileConductor,
				runMetadataAgentRole:    AgentProfileConductor,
				runMetadataTrajectoryID: "traj-acceptance",
				runMetadataDesktopID:    types.PrimaryDesktopID,
				"input_source":          "prompt_bar",
			},
		},
		{
			RunID:        "run-vtext-acceptance",
			AgentID:      "agent-vtext-acceptance",
			ChannelID:    "channel-acceptance",
			ParentRunID:  "run-conductor-acceptance",
			AgentProfile: AgentProfileVText,
			AgentRole:    AgentProfileVText,
			OwnerID:      "user-alice",
			SandboxID:    "sandbox-test",
			State:        types.RunCompleted,
			Prompt:       "Own the acceptance document.",
			CreatedAt:    now.Add(3 * time.Second),
			UpdatedAt:    now.Add(4 * time.Second),
			FinishedAt:   &finishedAt,
			Metadata: map[string]any{
				runMetadataAgentProfile: AgentProfileVText,
				runMetadataAgentRole:    AgentProfileVText,
				runMetadataTrajectoryID: "traj-acceptance",
				runMetadataDesktopID:    types.PrimaryDesktopID,
			},
		},
		{
			RunID:        "run-super-acceptance",
			AgentID:      "agent-super-acceptance",
			ChannelID:    "channel-acceptance",
			ParentRunID:  "run-vtext-acceptance",
			AgentProfile: AgentProfileSuper,
			AgentRole:    AgentProfileSuper,
			OwnerID:      "user-alice",
			SandboxID:    "sandbox-test",
			State:        types.RunCompleted,
			Prompt:       "Delegate a worker and publish an AppChangePackage.",
			CreatedAt:    now.Add(5 * time.Second),
			UpdatedAt:    now.Add(12 * time.Second),
			FinishedAt:   &finishedAt,
			Metadata: map[string]any{
				runMetadataAgentProfile: AgentProfileSuper,
				runMetadataAgentRole:    AgentProfileSuper,
				runMetadataTrajectoryID: "traj-acceptance",
				runMetadataDesktopID:    types.PrimaryDesktopID,
			},
		},
	}
	for _, run := range runs {
		if err := rt.store.CreateRun(ctx, run); err != nil {
			t.Fatalf("create run %s: %v", run.RunID, err)
		}
	}

	appendAcceptanceEvent(t, rt, types.EventRecord{
		EventID:      "event-submit-acceptance",
		RunID:        "run-conductor-acceptance",
		AgentID:      "agent-conductor-acceptance",
		ChannelID:    "channel-acceptance",
		OwnerID:      "user-alice",
		TrajectoryID: "traj-acceptance",
		Timestamp:    now,
		Kind:         types.EventRunSubmitted,
		Payload:      json.RawMessage(`{"input_source":"prompt_bar"}`),
	})
	appendAcceptanceEvent(t, rt, types.EventRecord{
		EventID:      "event-vtext-acceptance",
		RunID:        "run-vtext-acceptance",
		AgentID:      "agent-vtext-acceptance",
		ChannelID:    "channel-acceptance",
		OwnerID:      "user-alice",
		TrajectoryID: "traj-acceptance",
		Timestamp:    now.Add(4 * time.Second),
		Kind:         types.EventVTextDocumentRevisionCreated,
		Payload:      json.RawMessage(`{"doc_id":"doc-acceptance","revision_id":"rev-1"}`),
	})
	appendAcceptanceToolResult(t, rt, "event-super-acceptance", "run-vtext-acceptance", "agent-vtext-acceptance", now.Add(5*time.Second), "request_super_execution", map[string]any{
		"agent_id": "agent-super-acceptance",
		"loop_id":  "run-super-acceptance",
		"state":    "running",
	})
	appendAcceptanceToolResult(t, rt, "event-worker-lease-acceptance", "run-super-acceptance", "agent-super-acceptance", now.Add(6*time.Second), "request_worker_vm", map[string]any{
		"status": "worker_requested",
		"handle": map[string]any{
			"kind":          "worker",
			"worker_id":     "worker-acceptance",
			"vm_id":         "vm-acceptance",
			"desktop_id":    types.PrimaryDesktopID,
			"machine_class": "worker-medium",
			"sandbox_url":   "http://127.0.0.1:8085",
		},
	})
	appendAcceptanceToolError(t, rt, "event-delegate-timeout-acceptance", "run-super-acceptance", "agent-super-acceptance", now.Add(9*time.Second), "delegate_worker_vm", `tool_error: delegate_worker_vm status: context deadline exceeded`)
	appendAcceptanceToolError(t, rt, "event-delegate-restart-acceptance", "run-super-acceptance", "agent-super-acceptance", now.Add(10*time.Second), "delegate_worker_vm", `tool_error: worker run run-worker-acceptance ended in state failed: runtime restarted, run interrupted`)
}

func seedRunAcceptancePendingDelegationTrajectory(t *testing.T, rt *Runtime) {
	t.Helper()
	ctx := context.Background()
	now := time.Now().UTC()
	finishedAt := now.Add(15 * time.Second)
	runs := []types.RunRecord{
		{
			RunID:        "run-conductor-pending-delegate",
			AgentID:      "agent-conductor-pending-delegate",
			ChannelID:    "channel-pending-delegate",
			AgentProfile: AgentProfileConductor,
			AgentRole:    AgentProfileConductor,
			OwnerID:      "user-alice",
			SandboxID:    "sandbox-test",
			State:        types.RunCompleted,
			Prompt:       "Build a tiny Choir-in-Choir verifier patch.",
			Result:       `{"action":"open_app","app":"vtext","doc_id":"doc-pending-delegate"}`,
			CreatedAt:    now,
			UpdatedAt:    finishedAt,
			FinishedAt:   &finishedAt,
			Metadata: map[string]any{
				runMetadataAgentProfile: AgentProfileConductor,
				runMetadataAgentRole:    AgentProfileConductor,
				runMetadataTrajectoryID: "traj-pending-delegate",
				runMetadataDesktopID:    types.PrimaryDesktopID,
				"input_source":          "prompt_bar",
			},
		},
		{
			RunID:        "run-vtext-pending-delegate",
			AgentID:      "agent-vtext-pending-delegate",
			ChannelID:    "channel-pending-delegate",
			ParentRunID:  "run-conductor-pending-delegate",
			AgentProfile: AgentProfileVText,
			AgentRole:    AgentProfileVText,
			OwnerID:      "user-alice",
			SandboxID:    "sandbox-test",
			State:        types.RunCompleted,
			Prompt:       "Own the acceptance document.",
			CreatedAt:    now.Add(3 * time.Second),
			UpdatedAt:    now.Add(4 * time.Second),
			FinishedAt:   &finishedAt,
			Metadata: map[string]any{
				runMetadataAgentProfile: AgentProfileVText,
				runMetadataAgentRole:    AgentProfileVText,
				runMetadataTrajectoryID: "traj-pending-delegate",
				runMetadataDesktopID:    types.PrimaryDesktopID,
			},
		},
		{
			RunID:        "run-super-pending-delegate",
			AgentID:      "agent-super-pending-delegate",
			ChannelID:    "channel-pending-delegate",
			ParentRunID:  "run-vtext-pending-delegate",
			AgentProfile: AgentProfileSuper,
			AgentRole:    AgentProfileSuper,
			OwnerID:      "user-alice",
			SandboxID:    "sandbox-test",
			State:        types.RunRunning,
			Prompt:       "Delegate a worker and publish an AppChangePackage.",
			CreatedAt:    now.Add(5 * time.Second),
			UpdatedAt:    now.Add(8 * time.Second),
			Metadata: map[string]any{
				runMetadataAgentProfile: AgentProfileSuper,
				runMetadataAgentRole:    AgentProfileSuper,
				runMetadataTrajectoryID: "traj-pending-delegate",
				runMetadataDesktopID:    types.PrimaryDesktopID,
			},
		},
	}
	for _, run := range runs {
		if err := rt.store.CreateRun(ctx, run); err != nil {
			t.Fatalf("create run %s: %v", run.RunID, err)
		}
	}

	appendAcceptanceEvent(t, rt, types.EventRecord{
		EventID:      "event-submit-pending-delegate",
		RunID:        "run-conductor-pending-delegate",
		AgentID:      "agent-conductor-pending-delegate",
		ChannelID:    "channel-pending-delegate",
		OwnerID:      "user-alice",
		TrajectoryID: "traj-pending-delegate",
		Timestamp:    now,
		Kind:         types.EventRunSubmitted,
		Payload:      json.RawMessage(`{"input_source":"prompt_bar"}`),
	})
	appendAcceptanceEvent(t, rt, types.EventRecord{
		EventID:      "event-vtext-pending-delegate",
		RunID:        "run-vtext-pending-delegate",
		AgentID:      "agent-vtext-pending-delegate",
		ChannelID:    "channel-pending-delegate",
		OwnerID:      "user-alice",
		TrajectoryID: "traj-pending-delegate",
		Timestamp:    now.Add(4 * time.Second),
		Kind:         types.EventVTextDocumentRevisionCreated,
		Payload:      json.RawMessage(`{"doc_id":"doc-pending-delegate","revision_id":"rev-1"}`),
	})
	appendAcceptanceToolResult(t, rt, "event-super-pending-delegate", "run-vtext-pending-delegate", "agent-vtext-pending-delegate", now.Add(5*time.Second), "request_super_execution", map[string]any{
		"agent_id": "agent-super-pending-delegate",
		"loop_id":  "run-super-pending-delegate",
		"state":    "running",
	})
	appendAcceptanceToolResult(t, rt, "event-worker-lease-pending-delegate", "run-super-pending-delegate", "agent-super-pending-delegate", now.Add(6*time.Second), "request_worker_vm", map[string]any{
		"status": "worker_requested",
		"handle": map[string]any{
			"kind":          "worker",
			"worker_id":     "worker-pending",
			"vm_id":         "vm-pending",
			"desktop_id":    types.PrimaryDesktopID,
			"machine_class": "worker-medium",
			"sandbox_url":   "http://127.0.0.1:8085",
		},
	})
	appendAcceptanceToolInvoked(t, rt, "event-delegate-invoked-pending", "run-super-pending-delegate", "agent-super-pending-delegate", "traj-pending-delegate", "channel-pending-delegate", now.Add(7*time.Second), "delegate_worker_vm", "call-pending-delegate", map[string]any{
		"worker_id":          "worker-pending",
		"vm_id":              "vm-pending",
		"worker_sandbox_url": "http://127.0.0.1:8085",
		"profile":            AgentProfileVSuper,
		"objective":          "candidate-world task",
	})
}

func appendAcceptanceToolResult(t *testing.T, rt *Runtime, eventID, runID, agentID string, at time.Time, tool string, output map[string]any) {
	t.Helper()
	outputJSON, err := json.Marshal(output)
	if err != nil {
		t.Fatalf("marshal tool output: %v", err)
	}
	payload, err := json.Marshal(map[string]any{
		"tool":     tool,
		"call_id":  eventID + "-call",
		"is_error": false,
		"output":   string(outputJSON),
	})
	if err != nil {
		t.Fatalf("marshal tool payload: %v", err)
	}
	appendAcceptanceEvent(t, rt, types.EventRecord{
		EventID:      eventID,
		RunID:        runID,
		AgentID:      agentID,
		ChannelID:    "channel-acceptance",
		OwnerID:      "user-alice",
		TrajectoryID: "traj-acceptance",
		Timestamp:    at,
		Kind:         types.EventToolResult,
		Payload:      payload,
	})
}

func appendAcceptanceToolInvoked(t *testing.T, rt *Runtime, eventID, runID, agentID, trajectoryID, channelID string, at time.Time, tool, callID string, arguments map[string]any) {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"tool":      tool,
		"call_id":   callID,
		"arguments": arguments,
	})
	if err != nil {
		t.Fatalf("marshal tool invoked payload: %v", err)
	}
	appendAcceptanceEvent(t, rt, types.EventRecord{
		EventID:      eventID,
		RunID:        runID,
		AgentID:      agentID,
		ChannelID:    channelID,
		OwnerID:      "user-alice",
		TrajectoryID: trajectoryID,
		Timestamp:    at,
		Kind:         types.EventToolInvoked,
		Payload:      payload,
	})
}

func appendAcceptanceToolError(t *testing.T, rt *Runtime, eventID, runID, agentID string, at time.Time, tool, output string) {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"tool":       tool,
		"call_id":    eventID + "-call",
		"is_error":   true,
		"output_len": len(output),
		"output":     output,
	})
	if err != nil {
		t.Fatalf("marshal tool error payload: %v", err)
	}
	appendAcceptanceEvent(t, rt, types.EventRecord{
		EventID:      eventID,
		RunID:        runID,
		AgentID:      agentID,
		ChannelID:    "channel-acceptance",
		OwnerID:      "user-alice",
		TrajectoryID: "traj-acceptance",
		Timestamp:    at,
		Kind:         types.EventToolResult,
		Payload:      payload,
	})
}

func appendAcceptanceEvent(t *testing.T, rt *Runtime, rec types.EventRecord) {
	t.Helper()
	if err := rt.store.AppendEvent(context.Background(), &rec); err != nil {
		t.Fatalf("append event %s: %v", rec.EventID, err)
	}
}

func acceptanceHasCheckpoint(rec types.RunAcceptanceRecord, kind string) bool {
	for _, checkpoint := range rec.Checkpoints {
		if checkpoint.Kind == kind && checkpoint.State == "passed" {
			return true
		}
	}
	return false
}

func acceptanceCheckpoint(rec types.RunAcceptanceRecord, kind, state string) *types.RunAcceptanceCheckpoint {
	for i := range rec.Checkpoints {
		checkpoint := &rec.Checkpoints[i]
		if checkpoint.Kind == kind && checkpoint.State == state {
			return checkpoint
		}
	}
	return nil
}

func TestBrowserCapabilitiesRequireAuthAndReportUnavailable(t *testing.T) {
	_, handler := testAPISetup(t)

	unauth := authenticatedRequest(http.MethodGet, "/api/browser/capabilities", "", "")
	unauthW := httptest.NewRecorder()
	handler.HandleBrowserCapabilities(unauthW, unauth)
	if unauthW.Code != http.StatusUnauthorized {
		t.Fatalf("unauth status = %d, want %d", unauthW.Code, http.StatusUnauthorized)
	}

	req := authenticatedRequest(http.MethodGet, "/api/browser/capabilities", "", "user-alice")
	w := httptest.NewRecorder()
	handler.HandleBrowserCapabilities(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}
	var resp browserCapabilitiesResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Provider != "obscura" {
		t.Fatalf("provider = %q, want obscura", resp.Provider)
	}
	if resp.Available || resp.Configured || resp.Status != "not_configured" {
		t.Fatalf("unexpected unavailable response: %+v", resp)
	}
	if resp.Substrate != "frontend_iframe" {
		t.Fatalf("substrate = %q, want frontend_iframe", resp.Substrate)
	}
	if resp.Supports["navigate"] || resp.Supports["screenshot"] || resp.Supports["cdp_screenshot"] || resp.Supports["input"] || resp.Supports["cdp"] {
		t.Fatalf("unavailable support matrix should fail closed: %+v", resp.Supports)
	}
	if !resp.LegacyIframeAvailable {
		t.Fatalf("legacy iframe fallback should remain available until backend sessions are implemented")
	}
}

func TestBrowserCapabilitiesDetectConfiguredObscuraBinary(t *testing.T) {
	rt, handler := testAPISetup(t)
	dir := t.TempDir()
	bin := filepath.Join(dir, "obscura")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write fake obscura: %v", err)
	}
	rt.cfg.ObscuraPath = bin

	req := authenticatedRequest(http.MethodGet, "/api/browser/capabilities", "", "user-alice")
	w := httptest.NewRecorder()
	handler.HandleBrowserCapabilities(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}
	var resp browserCapabilitiesResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !resp.Available || !resp.Configured || resp.Mode != "backend" || resp.Status != "ready" {
		t.Fatalf("unexpected available response: %+v", resp)
	}
	if resp.Substrate != "obscura_cli_fetch" {
		t.Fatalf("substrate = %q, want obscura_cli_fetch", resp.Substrate)
	}
	if !resp.Supports["navigate"] || !resp.Supports["text"] || !resp.Supports["html"] || !resp.Supports["links"] {
		t.Fatalf("snapshot support matrix missing expected support: %+v", resp.Supports)
	}
	if resp.Supports["screenshot"] || resp.Supports["cdp_screenshot"] || resp.Supports["bounded_input"] || resp.Supports["input"] || resp.Supports["cdp"] {
		t.Fatalf("unexpected support matrix: %+v", resp.Supports)
	}
}

func TestBrowserCapabilitiesReportOptInCDPScreenshotSubstrate(t *testing.T) {
	rt, handler := testAPISetup(t)
	dir := t.TempDir()
	bin := filepath.Join(dir, "obscura")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write fake obscura: %v", err)
	}
	rt.cfg.ObscuraPath = bin
	rt.cfg.ObscuraCDPScreenshots = true

	req := authenticatedRequest(http.MethodGet, "/api/browser/capabilities", "", "user-alice")
	w := httptest.NewRecorder()
	handler.HandleBrowserCapabilities(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}
	var resp browserCapabilitiesResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Substrate != "obscura_cli_fetch+obscura_cdp_screenshot" {
		t.Fatalf("substrate = %q, want hybrid cdp screenshot substrate", resp.Substrate)
	}
	if !resp.Supports["screenshot"] || !resp.Supports["cdp_screenshot"] || !resp.Supports["bounded_input"] || !resp.Supports["fill"] || !resp.Supports["click"] {
		t.Fatalf("cdp bounded control support missing: %+v", resp.Supports)
	}
	if resp.Supports["input"] || resp.Supports["cdp"] {
		t.Fatalf("cdp screenshot mode must not claim generic input/cdp: %+v", resp.Supports)
	}
}

func TestBrowserSessionsNavigateThroughOwnerScopedBackendSnapshot(t *testing.T) {
	rt, handler := testAPISetup(t)
	dir := t.TempDir()
	bin := filepath.Join(dir, "obscura")
	if err := os.WriteFile(bin, []byte(`#!/bin/sh
mode=text
while [ "$#" -gt 0 ]; do
  if [ "$1" = "--dump" ]; then
    mode="$2"
    shift 2
  else
    shift
  fi
done
if [ "$mode" = "links" ]; then
  printf 'https://example.com/learn\tLearn more\n'
elif [ "$mode" = "html" ]; then
  printf '<!doctype html><title>Example Backend Page</title><h1>Example Backend Page</h1>'
else
  printf 'Example Backend Page\n\nSnapshot from fake Obscura\n'
fi
`), 0o755); err != nil {
		t.Fatalf("write fake obscura: %v", err)
	}
	rt.cfg.ObscuraPath = bin

	createW := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/browser/sessions", `{}`, "user-alice")
	if createW.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want %d; body: %s", createW.Code, http.StatusCreated, createW.Body.String())
	}
	var created types.BrowserSessionRecord
	if err := json.NewDecoder(createW.Body).Decode(&created); err != nil {
		t.Fatalf("decode create: %v", err)
	}
	if created.OwnerID != "user-alice" || created.Mode != "backend" || created.State != types.BrowserSessionIdle {
		t.Fatalf("unexpected created session: %+v", created)
	}

	otherUserW := registeredRuntimeRequest(t, handler, http.MethodGet, "/api/browser/sessions/"+created.SessionID, "", "user-bob")
	if otherUserW.Code != http.StatusNotFound {
		t.Fatalf("other user status = %d, want %d", otherUserW.Code, http.StatusNotFound)
	}

	navigateW := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/browser/sessions/"+created.SessionID+"/navigate", `{"url":"https://example.com/path#fragment"}`, "user-alice")
	if navigateW.Code != http.StatusOK {
		t.Fatalf("navigate status = %d, want %d; body: %s", navigateW.Code, http.StatusOK, navigateW.Body.String())
	}
	var navigated types.BrowserSessionRecord
	if err := json.NewDecoder(navigateW.Body).Decode(&navigated); err != nil {
		t.Fatalf("decode navigate: %v", err)
	}
	if navigated.State != types.BrowserSessionReady {
		t.Fatalf("state = %q, want %q; session: %+v", navigated.State, types.BrowserSessionReady, navigated)
	}
	if navigated.ExecutionScope != "host_process" {
		t.Fatalf("execution_scope = %q, want host_process", navigated.ExecutionScope)
	}
	if navigated.CurrentURL != "https://example.com/path" {
		t.Fatalf("current_url = %q, want normalized URL without fragment", navigated.CurrentURL)
	}
	if navigated.Title != "Example Backend Page" {
		t.Fatalf("title = %q, want first snapshot line", navigated.Title)
	}
	if !strings.Contains(navigated.TextSnapshot, "Snapshot from fake Obscura") {
		t.Fatalf("text_snapshot missing fake output: %q", navigated.TextSnapshot)
	}
	if !strings.Contains(navigated.HTMLSnapshot, "<title>Example Backend Page</title>") {
		t.Fatalf("html_snapshot missing fake output: %q", navigated.HTMLSnapshot)
	}
	if len(navigated.Links) != 1 || navigated.Links[0].URL != "https://example.com/learn" || navigated.Links[0].Text != "Learn more" {
		t.Fatalf("links = %+v, want extracted fake link", navigated.Links)
	}

	traceID := browserSessionTraceID(created.SessionID)
	events, err := rt.Store().ListEventsByTrajectory(context.Background(), "user-alice", traceID, 10)
	if err != nil {
		t.Fatalf("list browser trace events: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("browser trace event count = %d, want 2", len(events))
	}
	if events[0].Kind != types.EventBrowserSessionCreated || events[1].Kind != types.EventBrowserNavigationCompleted {
		t.Fatalf("browser trace kinds = %q, %q", events[0].Kind, events[1].Kind)
	}
	var payload map[string]any
	if err := json.Unmarshal(events[1].Payload, &payload); err != nil {
		t.Fatalf("decode browser completion payload: %v", err)
	}
	if int(payload["links_count"].(float64)) != 1 {
		t.Fatalf("links_count payload = %+v, want 1", payload)
	}
	if int(payload["html_snapshot_bytes"].(float64)) == 0 {
		t.Fatalf("html_snapshot_bytes payload = %+v, want nonzero", payload)
	}

	traceW := registeredRuntimeRequest(t, handler, http.MethodGet, "/api/trace/trajectories?limit=50", "", "user-alice")
	if traceW.Code != http.StatusOK {
		t.Fatalf("trace index status = %d, want %d; body: %s", traceW.Code, http.StatusOK, traceW.Body.String())
	}
	var traceResp traceTrajectoryListResponse
	if err := json.NewDecoder(traceW.Body).Decode(&traceResp); err != nil {
		t.Fatalf("decode trace index: %v", err)
	}
	foundTrace := false
	for _, trajectory := range traceResp.Trajectories {
		if trajectory.TrajectoryID == traceID {
			foundTrace = true
			if trajectory.MomentCount != 2 {
				t.Fatalf("browser trace moment count = %d, want 2", trajectory.MomentCount)
			}
		}
	}
	if !foundTrace {
		t.Fatalf("trace index missing event-only browser trajectory %q: %+v", traceID, traceResp.Trajectories)
	}
}

func TestBrowserSessionRejectsLegacyPromotionCandidateBinding(t *testing.T) {
	_, handler := testAPISetup(t)
	forged := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/browser/sessions", `{"vm_id":"vm-forged"}`, "user-alice")
	if forged.Code != http.StatusBadRequest {
		t.Fatalf("forged vm_id status = %d, want 400; body=%s", forged.Code, forged.Body.String())
	}

	legacy := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/browser/sessions", `{"promotion_candidate_id":"candidate-browser-world"}`, "user-alice")
	if legacy.Code != http.StatusBadRequest {
		t.Fatalf("legacy promotion_candidate_id status = %d, want 400; body=%s", legacy.Code, legacy.Body.String())
	}
}

func TestBrowserSessionNavigateFailsClosedWhenBackendUnavailable(t *testing.T) {
	_, handler := testAPISetup(t)

	createW := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/browser/sessions", `{}`, "user-alice")
	if createW.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want %d; body: %s", createW.Code, http.StatusCreated, createW.Body.String())
	}
	var created types.BrowserSessionRecord
	if err := json.NewDecoder(createW.Body).Decode(&created); err != nil {
		t.Fatalf("decode create: %v", err)
	}
	if created.State != types.BrowserSessionUnavailable {
		t.Fatalf("created state = %q, want unavailable", created.State)
	}

	navigateW := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/browser/sessions/"+created.SessionID+"/navigate", `{"url":"https://example.com"}`, "user-alice")
	if navigateW.Code != http.StatusServiceUnavailable {
		t.Fatalf("navigate status = %d, want %d; body: %s", navigateW.Code, http.StatusServiceUnavailable, navigateW.Body.String())
	}
	var blocked types.BrowserSessionRecord
	if err := json.NewDecoder(navigateW.Body).Decode(&blocked); err != nil {
		t.Fatalf("decode blocked: %v", err)
	}
	if blocked.State != types.BrowserSessionUnavailable || blocked.Error == "" {
		t.Fatalf("unexpected blocked session: %+v", blocked)
	}
	events, err := handler.rt.Store().ListEventsByTrajectory(context.Background(), "user-alice", browserSessionTraceID(created.SessionID), 10)
	if err != nil {
		t.Fatalf("list blocked browser trace events: %v", err)
	}
	if len(events) != 2 || events[1].Kind != types.EventBrowserNavigationFailed {
		t.Fatalf("blocked browser trace events = %+v, want create + navigation failed", events)
	}
}

func TestBrowserSessionCloseIsOwnerScopedIdempotentAndPreventsNavigation(t *testing.T) {
	rt, handler := testAPISetup(t)
	dir := t.TempDir()
	bin := filepath.Join(dir, "obscura")
	if err := os.WriteFile(bin, []byte(`#!/bin/sh
mode=text
while [ "$#" -gt 0 ]; do
  if [ "$1" = "--dump" ]; then
    mode="$2"
    shift 2
  else
    shift
  fi
done
case "$mode" in
  links) printf 'https://example.com/learn\tLearn more\n' ;;
  html) printf '<title>Example Backend Page</title>' ;;
  *) printf 'Example Backend Page\n' ;;
esac
`), 0o755); err != nil {
		t.Fatalf("write fake obscura: %v", err)
	}
	rt.cfg.ObscuraPath = bin

	createW := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/browser/sessions", `{}`, "user-alice")
	if createW.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want %d; body: %s", createW.Code, http.StatusCreated, createW.Body.String())
	}
	var created types.BrowserSessionRecord
	if err := json.NewDecoder(createW.Body).Decode(&created); err != nil {
		t.Fatalf("decode create: %v", err)
	}

	otherCloseW := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/browser/sessions/"+created.SessionID+"/close", `{}`, "user-bob")
	if otherCloseW.Code != http.StatusNotFound {
		t.Fatalf("other user close status = %d, want %d", otherCloseW.Code, http.StatusNotFound)
	}

	closeW := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/browser/sessions/"+created.SessionID+"/close", `{}`, "user-alice")
	if closeW.Code != http.StatusOK {
		t.Fatalf("close status = %d, want %d; body: %s", closeW.Code, http.StatusOK, closeW.Body.String())
	}
	var closed types.BrowserSessionRecord
	if err := json.NewDecoder(closeW.Body).Decode(&closed); err != nil {
		t.Fatalf("decode close: %v", err)
	}
	if closed.State != types.BrowserSessionClosed {
		t.Fatalf("closed state = %q, want %q", closed.State, types.BrowserSessionClosed)
	}

	closeAgainW := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/browser/sessions/"+created.SessionID+"/close", `{}`, "user-alice")
	if closeAgainW.Code != http.StatusOK {
		t.Fatalf("close again status = %d, want %d; body: %s", closeAgainW.Code, http.StatusOK, closeAgainW.Body.String())
	}

	navigateW := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/browser/sessions/"+created.SessionID+"/navigate", `{"url":"https://example.com"}`, "user-alice")
	if navigateW.Code != http.StatusConflict {
		t.Fatalf("navigate closed status = %d, want %d; body: %s", navigateW.Code, http.StatusConflict, navigateW.Body.String())
	}

	events, err := rt.Store().ListEventsByTrajectory(context.Background(), "user-alice", browserSessionTraceID(created.SessionID), 10)
	if err != nil {
		t.Fatalf("list close trace events: %v", err)
	}
	if len(events) != 2 || events[0].Kind != types.EventBrowserSessionCreated || events[1].Kind != types.EventBrowserSessionClosed {
		t.Fatalf("close trace events = %+v, want create + single close", events)
	}
}

func TestRegisteredPublicRoutesExcludeLegacyRuntimeAPIs(t *testing.T) {
	_, handler := testAPISetup(t)

	cases := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodPost, "/api/agent/loop", `{"prompt":"old"}`},
		{http.MethodPost, "/api/agent/spawn", `{"parent_id":"p","objective":"old"}`},
		{http.MethodGet, "/api/agent/status?loop_id=old", ""},
		{http.MethodGet, "/api/agent/loops", ""},
		{http.MethodGet, "/api/agent/events", ""},
		{http.MethodGet, "/api/agent/channel-messages?channel_id=doc", ""},
		{http.MethodGet, "/api/agent/topology", ""},
		{http.MethodGet, "/api/events", ""},
		{http.MethodGet, "/api/prompts", ""},
		{http.MethodPost, "/api/test/vtext/research-findings", `{"doc_id":"doc","finding_id":"f"}`},
		{http.MethodPost, "/api/vtext/documents/doc-1/agent-revision", `{"intent":"revise"}`},
	}

	for _, tc := range cases {
		w := registeredRuntimeRequest(t, handler, tc.method, tc.path, tc.body, "user-alice")
		if w.Code != http.StatusNotFound {
			t.Fatalf("%s %s: got status %d, want 404; body=%s", tc.method, tc.path, w.Code, w.Body.String())
		}
	}
}

func TestRegisteredPromptBarRouteAcceptsIntentOnly(t *testing.T) {
	_, handler := testAPISetup(t)

	ok := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/prompt-bar", `{"text":"build a document"}`, "user-alice")
	if ok.Code != http.StatusAccepted {
		t.Fatalf("prompt-bar status: got %d, want %d; body=%s", ok.Code, http.StatusAccepted, ok.Body.String())
	}

	bad := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/prompt-bar", `{"text":"build","agent_profile":"super"}`, "user-alice")
	if bad.Code != http.StatusBadRequest {
		t.Fatalf("prompt-bar metadata status: got %d, want %d", bad.Code, http.StatusBadRequest)
	}
}

func TestRunContinuationPublicSynthesizeListAndStartAreOwnerScoped(t *testing.T) {
	rt, handler := testAPISetup(t)
	source, err := rt.StartRunWithMetadata(context.Background(), "finish controller API source", "user-alice", map[string]any{
		runMetadataAgentProfile: AgentProfileSuper,
		runMetadataAgentRole:    AgentProfileSuper,
	})
	if err != nil {
		t.Fatalf("start source run: %v", err)
	}
	done := waitForRunTerminalState(t, rt, source.RunID, "user-alice", 5*time.Second)
	if done.State != types.RunCompleted {
		t.Fatalf("source state = %s", done.State)
	}
	adoptionID := "adoption-" + done.RunID
	if _, err := rt.store.UpsertAppAdoption(context.Background(), types.AppAdoptionRecord{
		AdoptionID:         adoptionID,
		OwnerID:            "user-alice",
		PackageID:          "pkg-continuation-api",
		AppID:              "podcast",
		TargetComputerID:   "computer-continuation-api",
		TargetComputerKind: "user",
		TargetCandidateID:  "candidate-continuation-api",
		Status:             types.AppAdoptionCandidateApplied,
		CandidateSourceRef: "refs/computers/computer-continuation-api/candidates/candidate-continuation-api",
		TraceID:            traceTrajectoryIDForRun(done),
	}); err != nil {
		t.Fatalf("upsert app adoption: %v", err)
	}

	unauth := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/continuations", `{"source_loop_id":"`+done.RunID+`"}`, "")
	if unauth.Code != http.StatusUnauthorized {
		t.Fatalf("unauth status = %d, want %d", unauth.Code, http.StatusUnauthorized)
	}

	selectW := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/continuations", `{"source_loop_id":"`+done.RunID+`"}`, "user-alice")
	if selectW.Code != http.StatusAccepted {
		t.Fatalf("select status = %d, want %d; body=%s", selectW.Code, http.StatusAccepted, selectW.Body.String())
	}
	var selected types.RunContinuationRecord
	if err := json.NewDecoder(selectW.Body).Decode(&selected); err != nil {
		t.Fatalf("decode selected: %v", err)
	}
	if selected.Status != types.RunContinuationSelected || selected.Details["adoption_id"] != adoptionID {
		t.Fatalf("unexpected selected continuation: %+v", selected)
	}
	sourceEvents, err := rt.store.ListEvents(context.Background(), done.RunID, 100)
	if err != nil {
		t.Fatalf("list source events: %v", err)
	}
	foundContinuationEvidence := false
	for _, ev := range sourceEvents {
		if ev.Kind != types.EventRunContinuationSelected {
			continue
		}
		payload := parseTracePayload(ev.Payload)
		details, _ := payload["details"].(map[string]any)
		if payloadString(payload, "compaction_status") != "" && payloadString(details, "adoption_id") == adoptionID {
			foundContinuationEvidence = true
			break
		}
	}
	if !foundContinuationEvidence {
		t.Fatalf("continuation selected event missing compaction/adoption evidence: %+v", sourceEvents)
	}

	listW := registeredRuntimeRequest(t, handler, http.MethodGet, "/api/continuations?source_loop_id="+done.RunID, "", "user-alice")
	if listW.Code != http.StatusOK {
		t.Fatalf("list status = %d, want %d; body=%s", listW.Code, http.StatusOK, listW.Body.String())
	}
	var listResp runContinuationListResponse
	if err := json.NewDecoder(listW.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(listResp.Continuations) != 1 || listResp.Continuations[0].ContinuationID != selected.ContinuationID {
		t.Fatalf("continuation list = %+v, want selected continuation", listResp.Continuations)
	}

	otherW := registeredRuntimeRequest(t, handler, http.MethodGet, "/api/continuations/"+selected.ContinuationID, "", "user-bob")
	if otherW.Code != http.StatusNotFound {
		t.Fatalf("other user detail status = %d, want %d", otherW.Code, http.StatusNotFound)
	}

	startW := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/continuations/"+selected.ContinuationID+"/start", `{}`, "user-alice")
	if startW.Code != http.StatusAccepted {
		t.Fatalf("start status = %d, want %d; body=%s", startW.Code, http.StatusAccepted, startW.Body.String())
	}
	var started types.RunContinuationRecord
	if err := json.NewDecoder(startW.Body).Decode(&started); err != nil {
		t.Fatalf("decode started: %v", err)
	}
	if started.Status != types.RunContinuationStarted || started.NextRunID == "" {
		t.Fatalf("unexpected started continuation: %+v", started)
	}
	child := waitForRunTerminalState(t, rt, started.NextRunID, "user-alice", 5*time.Second)
	if child.AgentProfile != AgentProfileVSuper {
		t.Fatalf("child agent profile = %q, want %q", child.AgentProfile, AgentProfileVSuper)
	}
}

func TestInternalRuntimeRunRoutesRequireInternalCallerAndConstrainProfiles(t *testing.T) {
	rt, handler := testAPISetup(t)

	body := `{"owner_id":"user-alice","prompt":"do worker work","metadata":{"agent_profile":"co-super"}}`
	publicReq := httptest.NewRequest(http.MethodPost, "/internal/runtime/runs", strings.NewReader(body))
	publicW := httptest.NewRecorder()
	handler.HandleInternalRunSubmission(publicW, publicReq)
	if publicW.Code != http.StatusForbidden {
		t.Fatalf("public internal runtime status = %d, want 403", publicW.Code)
	}

	badReq := httptest.NewRequest(http.MethodPost, "/internal/runtime/runs", strings.NewReader(`{"owner_id":"user-alice","prompt":"bad","metadata":{"agent_profile":"super"}}`))
	badReq.Header.Set("X-Internal-Caller", "true")
	badW := httptest.NewRecorder()
	handler.HandleInternalRunSubmission(badW, badReq)
	if badW.Code != http.StatusBadRequest {
		t.Fatalf("super internal runtime status = %d, want 400; body=%s", badW.Code, badW.Body.String())
	}

	req := httptest.NewRequest(http.MethodPost, "/internal/runtime/runs", strings.NewReader(body))
	req.Header.Set("X-Internal-Caller", "true")
	w := httptest.NewRecorder()
	handler.HandleInternalRunSubmission(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("internal runtime status = %d, want 202; body=%s", w.Code, w.Body.String())
	}
	var resp runStatusResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode internal run response: %v", err)
	}
	if resp.RunID == "" || resp.AgentProfile != AgentProfileCoSuper || resp.OwnerID != "user-alice" {
		t.Fatalf("unexpected internal run response: %+v", resp)
	}

	rec, err := rt.Store().GetRun(context.Background(), resp.RunID)
	if err != nil {
		t.Fatalf("get internal run: %v", err)
	}
	if metadataStringValue(rec.Metadata, "request_source") != "internal_worker_vm" {
		t.Fatalf("request_source = %q, want internal_worker_vm", metadataStringValue(rec.Metadata, "request_source"))
	}
}

func TestHandleRunSubmissionReturnsStableHandle(t *testing.T) {
	// VAL-RUNTIME-003: accepted run submission returns a stable handle.
	_, handler := testAPISetup(t)

	body := `{"prompt":"explain closures in Go"}`
	req := authenticatedRequest(http.MethodPost, "/api/agent/loop", body, "user-alice")
	w := httptest.NewRecorder()

	handler.HandleRunSubmission(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("status: got %d, want %d", w.Code, http.StatusAccepted)
	}

	var resp runSubmitResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.RunID == "" {
		t.Error("loop_id should not be empty")
	}
	if resp.State != types.RunPending {
		t.Errorf("state: got %q, want %q", resp.State, types.RunPending)
	}
	if resp.OwnerID != "user-alice" {
		t.Errorf("owner_id: got %q, want user-alice", resp.OwnerID)
	}
}

func TestHandleRunSubmissionPreservesMetadata(t *testing.T) {
	rt, handler := testAPISetup(t)

	body := `{"prompt":"route this into conductor","metadata":{"agent_profile":"conductor","agent_role":"conductor","input_source":"prompt_bar","requested_app":"vtext"}}`
	req := authenticatedRequest(http.MethodPost, "/api/agent/loop", body, "user-alice")
	w := httptest.NewRecorder()

	handler.HandleRunSubmission(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("status: got %d, want %d", w.Code, http.StatusAccepted)
	}

	var resp runSubmitResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	rec, err := rt.GetRun(context.Background(), resp.RunID, "user-alice")
	if err != nil {
		t.Fatalf("get task: %v", err)
	}

	if got, _ := rec.Metadata["agent_profile"].(string); got != AgentProfileConductor {
		t.Fatalf("agent_profile: got %q, want %q", got, AgentProfileConductor)
	}
	if got, _ := rec.Metadata["agent_role"].(string); got != AgentProfileConductor {
		t.Fatalf("agent_role: got %q, want %q", got, AgentProfileConductor)
	}
	if got, _ := rec.Metadata["input_source"].(string); got != "prompt_bar" {
		t.Fatalf("input_source: got %q, want prompt_bar", got)
	}
	if got, _ := rec.Metadata["requested_app"].(string); got != AgentProfileVText {
		t.Fatalf("requested_app: got %q, want %q", got, AgentProfileVText)
	}
}

func TestHandleRunSubmissionInjectsDesktopIDFromRequest(t *testing.T) {
	rt, handler := testAPISetup(t)

	body := `{"prompt":"fork this desktop later","metadata":{"agent_profile":"super","agent_role":"super"}}`
	req := authenticatedRequest(http.MethodPost, "/api/agent/loop?desktop_id=branch-a", body, "user-alice")
	w := httptest.NewRecorder()

	handler.HandleRunSubmission(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("status: got %d, want %d", w.Code, http.StatusAccepted)
	}

	var resp runSubmitResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	rec, err := rt.GetRun(context.Background(), resp.RunID, "user-alice")
	if err != nil {
		t.Fatalf("get run: %v", err)
	}
	if got, _ := rec.Metadata[runMetadataDesktopID].(string); got != "branch-a" {
		t.Fatalf("desktop_id: got %q, want %q", got, "branch-a")
	}
}

func TestHandleRunListOwnerScoped(t *testing.T) {
	rt, handler := testAPISetup(t)

	alice, err := rt.StartRunWithMetadata(context.Background(), "trace alice root", "user-alice", map[string]any{
		"agent_profile": "conductor",
		"agent_role":    "conductor",
	})
	if err != nil {
		t.Fatalf("submit alice task: %v", err)
	}
	if _, err := rt.StartRun(context.Background(), "trace bob root", "user-bob"); err != nil {
		t.Fatalf("submit bob task: %v", err)
	}

	req := authenticatedRequest(http.MethodGet, "/api/agent/loops?limit=20", "", "user-alice")
	w := httptest.NewRecorder()
	handler.HandleRunList(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", w.Code, http.StatusOK)
	}

	var resp runListResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(resp.Runs) == 0 {
		t.Fatal("expected at least one task")
	}
	for _, task := range resp.Runs {
		if task.OwnerID != "user-alice" {
			t.Fatalf("unexpected owner in task list: %q", task.OwnerID)
		}
	}
	if resp.Runs[0].RunID != alice.RunID {
		t.Errorf("first task id: got %q, want %q", resp.Runs[0].RunID, alice.RunID)
	}
	if profile, _ := resp.Runs[0].Metadata["agent_profile"].(string); profile != "conductor" {
		t.Errorf("metadata.agent_profile: got %q, want %q", profile, "conductor")
	}
}

func TestHandleEventListSupportsOwnerAndTaskHistory(t *testing.T) {
	rt, handler := testAPISetup(t)

	rec, err := rt.StartRunWithMetadata(context.Background(), "trace selected task", "user-alice", map[string]any{
		"agent_profile": "vtext",
		"agent_role":    "vtext",
	})
	if err != nil {
		t.Fatalf("submit task: %v", err)
	}

	time.Sleep(120 * time.Millisecond)

	ownerReq := authenticatedRequest(http.MethodGet, "/api/agent/events?limit=50", "", "user-alice")
	ownerW := httptest.NewRecorder()
	handler.HandleEventList(ownerW, ownerReq)

	if ownerW.Code != http.StatusOK {
		t.Fatalf("owner events status: got %d, want %d", ownerW.Code, http.StatusOK)
	}

	var ownerResp eventListResponse
	if err := json.NewDecoder(ownerW.Body).Decode(&ownerResp); err != nil {
		t.Fatalf("decode owner events: %v", err)
	}
	if len(ownerResp.Events) == 0 {
		t.Fatal("expected owner events")
	}

	taskReq := authenticatedRequest(http.MethodGet, "/api/agent/events?loop_id="+rec.RunID+"&limit=50", "", "user-alice")
	taskW := httptest.NewRecorder()
	handler.HandleEventList(taskW, taskReq)

	if taskW.Code != http.StatusOK {
		t.Fatalf("task events status: got %d, want %d", taskW.Code, http.StatusOK)
	}

	var taskResp eventListResponse
	if err := json.NewDecoder(taskW.Body).Decode(&taskResp); err != nil {
		t.Fatalf("decode task events: %v", err)
	}
	if len(taskResp.Events) == 0 {
		t.Fatal("expected task events")
	}
	for _, event := range taskResp.Events {
		if event.RunID != rec.RunID {
			t.Fatalf("unexpected loop_id in task-scoped events: %q", event.RunID)
		}
	}

	otherReq := authenticatedRequest(http.MethodGet, "/api/agent/events?loop_id="+rec.RunID, "", "user-bob")
	otherW := httptest.NewRecorder()
	handler.HandleEventList(otherW, otherReq)
	if otherW.Code != http.StatusNotFound {
		t.Fatalf("cross-owner task events status: got %d, want %d", otherW.Code, http.StatusNotFound)
	}
}

func TestHandleChannelMessageListOwnerScoped(t *testing.T) {
	rt, handler := testAPISetup(t)

	rec, err := rt.StartRunWithMetadata(context.Background(), "trace shared workflow", "user-alice", map[string]any{
		"agent_profile": "researcher",
		"agent_role":    "researcher",
		"channel_id":    "doc-123",
	})
	if err != nil {
		t.Fatalf("submit run: %v", err)
	}
	if _, err := rt.ChannelPost(WithToolExecutionContext(context.Background(), rec), "doc-123", "researcher-1", "researcher", "grounded finding"); err != nil {
		t.Fatalf("channel post: %v", err)
	}
	if _, err := rt.ChannelPost(WithToolExecutionContext(context.Background(), rec), "doc-123", "researcher-1", "researcher", "second grounded finding"); err != nil {
		t.Fatalf("channel post: %v", err)
	}

	req := authenticatedRequest(http.MethodGet, "/api/agent/channel-messages?channel_id=doc-123&limit=20", "", "user-alice")
	w := httptest.NewRecorder()
	handler.HandleChannelMessageList(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", w.Code, http.StatusOK)
	}

	var resp channelMessageListResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Messages) != 2 {
		t.Fatalf("messages: got %d, want 2", len(resp.Messages))
	}
	if resp.Messages[0].Content != "grounded finding" {
		t.Fatalf("first message content: got %q", resp.Messages[0].Content)
	}

	afterReq := authenticatedRequest(http.MethodGet, "/api/agent/channel-messages?channel_id=doc-123&after_seq=1&limit=20", "", "user-alice")
	afterW := httptest.NewRecorder()
	handler.HandleChannelMessageList(afterW, afterReq)
	if afterW.Code != http.StatusOK {
		t.Fatalf("after status: got %d, want %d", afterW.Code, http.StatusOK)
	}
	var afterResp channelMessageListResponse
	if err := json.NewDecoder(afterW.Body).Decode(&afterResp); err != nil {
		t.Fatalf("decode after response: %v", err)
	}
	if len(afterResp.Messages) != 1 || afterResp.Messages[0].Content != "second grounded finding" {
		t.Fatalf("after_seq messages: %+v", afterResp.Messages)
	}

	otherReq := authenticatedRequest(http.MethodGet, "/api/agent/channel-messages?channel_id=doc-123&limit=20", "", "user-bob")
	otherW := httptest.NewRecorder()
	handler.HandleChannelMessageList(otherW, otherReq)
	if otherW.Code != http.StatusOK {
		t.Fatalf("cross-owner status: got %d, want %d", otherW.Code, http.StatusOK)
	}
	var otherResp channelMessageListResponse
	if err := json.NewDecoder(otherW.Body).Decode(&otherResp); err != nil {
		t.Fatalf("decode cross-owner response: %v", err)
	}
	if len(otherResp.Messages) != 0 {
		t.Fatalf("cross-owner messages: got %d, want 0", len(otherResp.Messages))
	}
}

func TestHandleRunSubmissionAuthGated(t *testing.T) {
	// VAL-RUNTIME-002: task submission is auth-gated.
	_, handler := testAPISetup(t)

	// Request without auth header.
	body := `{"prompt":"test prompt"}`
	req := authenticatedRequest(http.MethodPost, "/api/agent/loop", body, "")
	w := httptest.NewRecorder()

	handler.HandleRunSubmission(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestHandleRunSubmissionMethodNotAllowed(t *testing.T) {
	_, handler := testAPISetup(t)

	req := authenticatedRequest(http.MethodGet, "/api/agent/loop", "", "user-alice")
	w := httptest.NewRecorder()

	handler.HandleRunSubmission(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusMethodNotAllowed)
	}
}

func TestHandleRunSubmissionEmptyPrompt(t *testing.T) {
	_, handler := testAPISetup(t)

	body := `{"prompt":""}`
	req := authenticatedRequest(http.MethodPost, "/api/agent/loop", body, "user-alice")
	w := httptest.NewRecorder()

	handler.HandleRunSubmission(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleRunSubmissionInvalidBody(t *testing.T) {
	_, handler := testAPISetup(t)

	req := authenticatedRequest(http.MethodPost, "/api/agent/loop", "not json", "user-alice")
	w := httptest.NewRecorder()

	handler.HandleRunSubmission(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// --- Task Status Tests ---

func TestHandleRunStatusReturnsCorrelatedHandle(t *testing.T) {
	// VAL-RUNTIME-004: status is correlated to the submitted handle.
	rt, handler := testAPISetup(t)

	rec, err := rt.StartRun(context.Background(), "test prompt", "user-alice")
	if err != nil {
		t.Fatalf("submit task: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	req := authenticatedRequest(http.MethodGet,
		fmt.Sprintf("/api/agent/status?loop_id=%s", rec.RunID), "", "user-alice")
	w := httptest.NewRecorder()

	handler.HandleRunStatus(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", w.Code, http.StatusOK)
	}

	var resp runStatusResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.RunID != rec.RunID {
		t.Errorf("loop_id: got %q, want %q", resp.RunID, rec.RunID)
	}
	if resp.State != types.RunCompleted {
		t.Errorf("state: got %q, want %q", resp.State, types.RunCompleted)
	}
	if resp.Result == "" {
		t.Error("result should not be empty for completed task")
	}
}

func TestHandleRunStatusAuthGated(t *testing.T) {
	// VAL-RUNTIME-006: status is auth-gated.
	_, handler := testAPISetup(t)

	req := authenticatedRequest(http.MethodGet, "/api/agent/status?loop_id=test", "", "")
	w := httptest.NewRecorder()

	handler.HandleRunStatus(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestHandleRunStatusCallerScoped(t *testing.T) {
	// VAL-RUNTIME-006: status is caller-scoped (user cannot see other users' runs).
	rt, handler := testAPISetup(t)

	rec, err := rt.StartRun(context.Background(), "alice task", "user-alice")
	if err != nil {
		t.Fatalf("submit task: %v", err)
	}

	// Eve tries to see Alice's task.
	req := authenticatedRequest(http.MethodGet,
		fmt.Sprintf("/api/agent/status?loop_id=%s", rec.RunID), "", "user-eve")
	w := httptest.NewRecorder()

	handler.HandleRunStatus(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d (caller-scoped denial)", w.Code, http.StatusNotFound)
	}
}

func TestHandleRunStatusMissingRunID(t *testing.T) {
	_, handler := testAPISetup(t)

	req := authenticatedRequest(http.MethodGet, "/api/agent/status", "", "user-alice")
	w := httptest.NewRecorder()

	handler.HandleRunStatus(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleRunStatusNotFound(t *testing.T) {
	_, handler := testAPISetup(t)

	req := authenticatedRequest(http.MethodGet, "/api/agent/status?loop_id=nonexistent", "", "user-alice")
	w := httptest.NewRecorder()

	handler.HandleRunStatus(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestHandleRunStatusFailedOutcome(t *testing.T) {
	// VAL-RUNTIME-004: status exposes non-happy-path outcomes.
	dir := filepath.Join(os.TempDir(), "go-choir-m3-api-test")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	dbPath := filepath.Join(dir, t.Name()+".db")
	_ = os.Remove(dbPath)

	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	bus := events.NewEventBus()
	provider := &StubProvider{
		Delay:   10 * time.Millisecond,
		FailErr: errors.New("provider timeout"),
	}

	cfg := Config{
		SandboxID:           "sandbox-test",
		StorePath:           dbPath,
		ProviderTimeout:     time.Second,
		SupervisionInterval: 1 * time.Hour,
	}

	rt := New(cfg, s, bus, provider)
	handler := NewAPIHandler(rt)

	t.Cleanup(func() {
		rt.Stop()
		_ = s.Close()
		_ = os.Remove(dbPath)
	})

	rec, err := rt.StartRun(context.Background(), "failing prompt", "user-alice")
	if err != nil {
		t.Fatalf("submit task: %v", err)
	}

	waitForRunTerminalState(t, rt, rec.RunID, "user-alice", 5*time.Second)

	req := authenticatedRequest(http.MethodGet,
		fmt.Sprintf("/api/agent/status?loop_id=%s", rec.RunID), "", "user-alice")
	w := httptest.NewRecorder()

	handler.HandleRunStatus(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", w.Code, http.StatusOK)
	}

	var resp runStatusResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.State != types.RunFailed {
		t.Errorf("state: got %q, want %q", resp.State, types.RunFailed)
	}
	if resp.Error == "" {
		t.Error("error should not be empty for failed task")
	}
}

// --- Task Status By Path ID Tests (VAL-CHOIR-002, VAL-CHOIR-005) ---
// GET /api/agent/{id}/status

func TestHandleRunStatusByIDReturnsTaskRecord(t *testing.T) {
	// VAL-CHOIR-002: GET /api/agent/{id}/status returns task record.
	rt, handler := testAPISetup(t)

	rec, err := rt.StartRun(context.Background(), "test status by id", "user-alice")
	if err != nil {
		t.Fatalf("submit task: %v", err)
	}

	// Wait for task to complete.
	time.Sleep(200 * time.Millisecond)

	req := authenticatedRequest(http.MethodGet,
		fmt.Sprintf("/api/agent/%s/status", rec.RunID), "", "user-alice")
	w := httptest.NewRecorder()

	handler.HandleRunStatusByID(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", w.Code, http.StatusOK)
	}

	var resp runStatusResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	// Response includes all required fields (VAL-CHOIR-002).
	if resp.RunID != rec.RunID {
		t.Errorf("loop_id: got %q, want %q", resp.RunID, rec.RunID)
	}
	if resp.OwnerID != "user-alice" {
		t.Errorf("owner_id: got %q, want user-alice", resp.OwnerID)
	}
	if resp.State == "" {
		t.Error("state should not be empty")
	}
	if resp.Prompt == "" {
		t.Error("prompt should not be empty")
	}
	if resp.CreatedAt == "" {
		t.Error("created_at should not be empty")
	}
	if resp.UpdatedAt == "" {
		t.Error("updated_at should not be empty")
	}
}

func TestHandleRunStatusByIDCompletedResult(t *testing.T) {
	// VAL-CHOIR-005: completed task has result and finished_at.
	rt, handler := testAPISetup(t)

	rec, err := rt.StartRun(context.Background(), "result check prompt", "user-alice")
	if err != nil {
		t.Fatalf("submit task: %v", err)
	}

	// Wait for task to complete.
	time.Sleep(200 * time.Millisecond)

	req := authenticatedRequest(http.MethodGet,
		fmt.Sprintf("/api/agent/%s/status", rec.RunID), "", "user-alice")
	w := httptest.NewRecorder()

	handler.HandleRunStatusByID(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", w.Code, http.StatusOK)
	}

	var resp runStatusResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.State != types.RunCompleted {
		t.Errorf("state: got %q, want %q", resp.State, types.RunCompleted)
	}
	if resp.Result == "" {
		t.Error("result should not be empty for completed task (VAL-CHOIR-005)")
	}
	if resp.FinishedAt == nil || *resp.FinishedAt == "" {
		t.Error("finished_at should be set for completed task (VAL-CHOIR-005)")
	}
}

func TestHandleRunStatusByIDAuthGated(t *testing.T) {
	// VAL-CHOIR-002: unauthenticated request returns 401.
	_, handler := testAPISetup(t)

	req := authenticatedRequest(http.MethodGet, "/api/agent/some-id/status", "", "")
	w := httptest.NewRecorder()

	handler.HandleRunStatusByID(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestHandleRunStatusByIDCallerScoped(t *testing.T) {
	// VAL-CHOIR-002: 404 for task owned by different user (403 in spec,
	// but we use 404 to prevent IDOR probing — same as query-param handler).
	rt, handler := testAPISetup(t)

	rec, err := rt.StartRun(context.Background(), "alice private task", "user-alice")
	if err != nil {
		t.Fatalf("submit task: %v", err)
	}

	// Eve tries to see Alice's task.
	req := authenticatedRequest(http.MethodGet,
		fmt.Sprintf("/api/agent/%s/status", rec.RunID), "", "user-eve")
	w := httptest.NewRecorder()

	handler.HandleRunStatusByID(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d (caller-scoped denial)", w.Code, http.StatusNotFound)
	}
}

func TestHandleRunStatusByIDNotFound(t *testing.T) {
	// VAL-CHOIR-002: 404 for non-existent task.
	_, handler := testAPISetup(t)

	req := authenticatedRequest(http.MethodGet,
		"/api/agent/nonexistent-task-id/status", "", "user-alice")
	w := httptest.NewRecorder()

	handler.HandleRunStatusByID(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestHandleRunStatusByIDFailedOutcome(t *testing.T) {
	// VAL-CHOIR-002: status exposes error information for failed runs.
	dir := filepath.Join(os.TempDir(), "go-choir-m3-api-test")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	dbPath := filepath.Join(dir, t.Name()+".db")
	_ = os.Remove(dbPath)

	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	bus := events.NewEventBus()
	provider := &StubProvider{
		Delay:   10 * time.Millisecond,
		FailErr: errors.New("provider timeout for by-id test"),
	}

	cfg := Config{
		SandboxID:           "sandbox-test",
		StorePath:           dbPath,
		ProviderTimeout:     time.Second,
		SupervisionInterval: 1 * time.Hour,
	}

	rt := New(cfg, s, bus, provider)
	handler := NewAPIHandler(rt)

	t.Cleanup(func() {
		rt.Stop()
		_ = s.Close()
		_ = os.Remove(dbPath)
	})

	rec, err := rt.StartRun(context.Background(), "failing by-id prompt", "user-alice")
	if err != nil {
		t.Fatalf("submit task: %v", err)
	}

	waitForRunTerminalState(t, rt, rec.RunID, "user-alice", 5*time.Second)

	req := authenticatedRequest(http.MethodGet,
		fmt.Sprintf("/api/agent/%s/status", rec.RunID), "", "user-alice")
	w := httptest.NewRecorder()

	handler.HandleRunStatusByID(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", w.Code, http.StatusOK)
	}

	var resp runStatusResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.State != types.RunFailed {
		t.Errorf("state: got %q, want %q", resp.State, types.RunFailed)
	}
	if resp.Error == "" {
		t.Error("error should not be empty for failed task")
	}
	if resp.FinishedAt == nil || *resp.FinishedAt == "" {
		t.Error("finished_at should be set for failed task")
	}
}

func TestHandleRunStatusByIDMethodNotAllowed(t *testing.T) {
	// Only GET is allowed.
	_, handler := testAPISetup(t)

	req := authenticatedRequest(http.MethodPost, "/api/agent/some-id/status", "", "user-alice")
	w := httptest.NewRecorder()

	handler.HandleRunStatusByID(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusMethodNotAllowed)
	}
}

func TestHandleRunStatusByIDSpawnedChildTask(t *testing.T) {
	// VAL-CHOIR-002: status works for spawned child runs too.
	rt, handler := testAPISetup(t)

	// Create a parent task first.
	parent, err := rt.StartRun(context.Background(), "parent task", "user-alice")
	if err != nil {
		t.Fatalf("submit parent task: %v", err)
	}

	// Spawn a child task.
	child, err := rt.StartChildRun(context.Background(), parent.RunID, "child objective", "user-alice", nil)
	if err != nil {
		t.Fatalf("spawn child task: %v", err)
	}

	// Wait for the child task to complete.
	time.Sleep(200 * time.Millisecond)

	req := authenticatedRequest(http.MethodGet,
		fmt.Sprintf("/api/agent/%s/status", child.RunID), "", "user-alice")
	w := httptest.NewRecorder()

	handler.HandleRunStatusByID(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", w.Code, http.StatusOK)
	}

	var resp runStatusResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.RunID != child.RunID {
		t.Errorf("loop_id: got %q, want %q", resp.RunID, child.RunID)
	}
	if resp.State == "" {
		t.Error("state should not be empty")
	}
	// Verify metadata includes parent_id.
	if resp.Metadata == nil {
		t.Error("metadata should not be nil for spawned task")
	} else if pid, _ := resp.Metadata["parent_id"].(string); pid != parent.RunID {
		t.Errorf("metadata.parent_id: got %q, want %q", pid, parent.RunID)
	}
}

func TestHandleRunStatusByIDStateTransitions(t *testing.T) {
	// VAL-CHOIR-002: state transitions reflected in status.
	// Verify that status shows different states as the task progresses.
	rt, handler := testAPISetup(t)

	rec, err := rt.StartRun(context.Background(), "state transition test", "user-alice")
	if err != nil {
		t.Fatalf("submit task: %v", err)
	}

	// Immediately check — should be at least pending (may already be running).
	req := authenticatedRequest(http.MethodGet,
		fmt.Sprintf("/api/agent/%s/status", rec.RunID), "", "user-alice")
	w := httptest.NewRecorder()
	handler.HandleRunStatusByID(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("initial status: got %d, want %d", w.Code, http.StatusOK)
	}

	var initialResp runStatusResponse
	if err := json.NewDecoder(w.Body).Decode(&initialResp); err != nil {
		t.Fatalf("decode initial response: %v", err)
	}

	// The initial state should be pending or running.
	if initialResp.State != types.RunPending && initialResp.State != types.RunRunning && initialResp.State != types.RunCompleted {
		t.Errorf("initial state: got %q, want pending/running/completed", initialResp.State)
	}

	// Wait for task to complete.
	time.Sleep(200 * time.Millisecond)

	req2 := authenticatedRequest(http.MethodGet,
		fmt.Sprintf("/api/agent/%s/status", rec.RunID), "", "user-alice")
	w2 := httptest.NewRecorder()
	handler.HandleRunStatusByID(w2, req2)

	var finalResp runStatusResponse
	if err := json.NewDecoder(w2.Body).Decode(&finalResp); err != nil {
		t.Fatalf("decode final response: %v", err)
	}

	if finalResp.State != types.RunCompleted {
		t.Errorf("final state: got %q, want %q", finalResp.State, types.RunCompleted)
	}

	// UpdatedAt should be >= CreatedAt.
	if finalResp.UpdatedAt < finalResp.CreatedAt {
		t.Errorf("updated_at %q should be >= created_at %q", finalResp.UpdatedAt, finalResp.CreatedAt)
	}
}

// --- Events Tests ---

func TestHandleEventsAuthGated(t *testing.T) {
	// VAL-RUNTIME-006: events are auth-gated.
	_, handler := testAPISetup(t)

	req := authenticatedRequest(http.MethodGet, "/api/events", "", "")
	w := httptest.NewRecorder()

	handler.HandleEvents(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestHandleEventsReturnsSSEStream(t *testing.T) {
	// VAL-RUNTIME-005: events stream is long-lived and incremental.
	rt, handler := testAPISetup(t)

	// Start the SSE connection in a goroutine.
	req := authenticatedRequest(http.MethodGet, "/api/events", "", "user-alice")
	req = req.WithContext(context.Background())
	w := httptest.NewRecorder()

	// SSE is a long-lived connection; we need to run it in a goroutine
	// and signal when we're done reading.
	done := make(chan struct{})
	go func() {
		handler.HandleEvents(w, req)
		close(done)
	}()

	// Submit a task to generate events.
	_, err := rt.StartRun(context.Background(), "test prompt for events", "user-alice")
	if err != nil {
		t.Fatalf("submit task: %v", err)
	}

	// Wait a bit for events to be emitted.
	time.Sleep(100 * time.Millisecond)

	// Read the response body so far.
	body := w.Body.String()

	// The response should have SSE headers.
	ct := w.Header().Get("Content-Type")
	if ct != "text/event-stream" {
		t.Errorf("content-type: got %q, want text/event-stream", ct)
	}

	// The body should contain at least one SSE data line.
	if !strings.Contains(body, "data: ") {
		t.Errorf("expected SSE data line in body, got: %s", body)
	}

	// Verify the SSE data contains event records.
	lines := strings.Split(body, "\n")
	var foundSubmitted bool
	for _, line := range lines {
		if strings.HasPrefix(line, "data: ") {
			var ev types.EventRecord
			data := strings.TrimPrefix(line, "data: ")
			if err := json.Unmarshal([]byte(data), &ev); err != nil {
				continue // skip malformed lines
			}
			if ev.Kind == types.EventRunSubmitted && ev.OwnerID == "user-alice" {
				foundSubmitted = true
			}
		}
	}
	if !foundSubmitted {
		t.Error("expected loop.submitted event in SSE stream")
	}
}

func TestHandleEventsCallerScoped(t *testing.T) {
	// VAL-RUNTIME-006: events are caller-scoped.
	rt, handler := testAPISetup(t)

	// Submit a task for alice.
	_, err := rt.StartRun(context.Background(), "alice task", "user-alice")
	if err != nil {
		t.Fatalf("submit task: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Connect as bob — should not see alice's events.
	req := authenticatedRequest(http.MethodGet, "/api/events", "", "user-bob")
	req = req.WithContext(context.Background())
	w := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		handler.HandleEvents(w, req)
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)

	body := w.Body.String()

	// Bob should not see any events for alice's task.
	lines := strings.Split(body, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "data: ") {
			var ev types.EventRecord
			data := strings.TrimPrefix(line, "data: ")
			if err := json.Unmarshal([]byte(data), &ev); err != nil {
				continue
			}
			if ev.OwnerID == "user-alice" {
				t.Errorf("bob should not see alice's events: %+v", ev)
			}
		}
	}
}

func TestHandleEventsIncremental(t *testing.T) {
	// VAL-RUNTIME-005: events arrive incrementally, not buffered.
	rt, handler := testAPISetup(t)

	req := authenticatedRequest(http.MethodGet, "/api/events", "", "user-alice")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()

	go func() {
		handler.HandleEvents(w, req)
	}()

	// Submit a task — should generate events incrementally.
	_, err := rt.StartRun(context.Background(), "incremental test", "user-alice")
	if err != nil {
		t.Fatalf("submit task: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	body := w.Body.String()

	// Parse SSE events and check that multiple different kinds arrived.
	kinds := make(map[types.EventKind]bool)
	scanner := bufio.NewScanner(strings.NewReader(body))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			var ev types.EventRecord
			data := strings.TrimPrefix(line, "data: ")
			if err := json.Unmarshal([]byte(data), &ev); err != nil {
				continue
			}
			kinds[ev.Kind] = true
		}
	}

	// Should see at least submitted + started (incremental, not buffered).
	if !kinds[types.EventRunSubmitted] {
		t.Error("expected loop.submitted event")
	}
	if !kinds[types.EventRunStarted] {
		t.Error("expected loop.started event (arrived incrementally)")
	}
}

// --- Health Tests ---

func TestHandleHealthReady(t *testing.T) {
	// VAL-RUNTIME-001: health reflects runtime readiness.
	_, handler := testAPISetup(t)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	handler.HandleHealth(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusOK)
	}

	var resp runtimeHealthResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Status != "ready" {
		t.Errorf("status: got %q, want ready", resp.Status)
	}
	if resp.RuntimeHealth != types.HealthReady {
		t.Errorf("runtime_health: got %q, want ready", resp.RuntimeHealth)
	}
	if resp.SandboxID != "sandbox-test" {
		t.Errorf("sandbox_id: got %q, want sandbox-test", resp.SandboxID)
	}
	if resp.ActiveProvider != "stub" {
		t.Errorf("active_provider: got %q, want stub (default test provider)", resp.ActiveProvider)
	}
	if resp.Build.Service != "sandbox" {
		t.Errorf("build.service: got %q, want sandbox", resp.Build.Service)
	}
	if resp.Build.Commit == "" {
		t.Error("build.commit should not be empty")
	}
}

func TestHandleHealthDegraded(t *testing.T) {
	// VAL-RUNTIME-001: degraded state is surfaced.
	rt, handler := testAPISetup(t)

	rt.SetHealth(types.HealthDegraded)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	handler.HandleHealth(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d (degraded is still serving)", w.Code, http.StatusOK)
	}

	var resp runtimeHealthResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Status != "degraded" {
		t.Errorf("status: got %q, want degraded", resp.Status)
	}
	if resp.RuntimeHealth != types.HealthDegraded {
		t.Errorf("runtime_health: got %q, want degraded", resp.RuntimeHealth)
	}
}

func TestHandleHealthFailed(t *testing.T) {
	// VAL-RUNTIME-001: failed state is surfaced with 503.
	rt, handler := testAPISetup(t)

	rt.SetHealth(types.HealthFailed)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	handler.HandleHealth(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusServiceUnavailable)
	}

	var resp runtimeHealthResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.RuntimeHealth != types.HealthFailed {
		t.Errorf("runtime_health: got %q, want failed", resp.RuntimeHealth)
	}
}

func TestHandleHealthReflectsRunningTasks(t *testing.T) {
	_, handler := testAPISetup(t)
	rt := handler.rt

	// No runs running initially.
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	handler.HandleHealth(w, req)

	var resp runtimeHealthResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.RunningRuns != 0 {
		t.Errorf("running_runs: got %d, want 0", resp.RunningRuns)
	}

	// Submit a task.
	_, err := rt.StartRun(context.Background(), "running task", "user-alice")
	if err != nil {
		t.Fatalf("submit task: %v", err)
	}

	w = httptest.NewRecorder()
	handler.HandleHealth(w, req)

	var resp2 runtimeHealthResponse
	if err := json.NewDecoder(w.Body).Decode(&resp2); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp2.RunningRuns < 1 {
		t.Errorf("running_runs: got %d, want >= 1", resp2.RunningRuns)
	}
}

func TestHandleTopologyReportsOrchestrationShape(t *testing.T) {
	rt, handler := testAPISetup(t)
	rt.cfg.ResearcherCount = 5

	if _, err := rt.ChannelManager().Channel("parent-1"); err != nil {
		t.Fatalf("create parent channel: %v", err)
	}
	if _, err := rt.ChannelManager().Channel("child-1"); err != nil {
		t.Fatalf("create child channel: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/agent/topology", nil)
	w := httptest.NewRecorder()

	handler.HandleTopology(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", w.Code, http.StatusOK)
	}

	var resp runtimeTopologyResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.ResearcherCount != 5 {
		t.Errorf("researcher_count: got %d, want 5", resp.ResearcherCount)
	}
	if resp.ChannelCount != 2 {
		t.Errorf("channel_count: got %d, want 2", resp.ChannelCount)
	}
}

func TestHandleVTextDocumentsRootUsesVTextRoutes(t *testing.T) {
	_, handler := testAPISetup(t)

	createReqBody := `{"title":"vtext alias doc","content":"hello"}`
	createReq := authenticatedRequest(http.MethodPost, "/api/vtext/documents", createReqBody, "user-alice")
	createW := httptest.NewRecorder()
	handler.HandleVTextDocumentsRoot(createW, createReq)

	if createW.Code != http.StatusCreated {
		t.Fatalf("create status: got %d, want %d", createW.Code, http.StatusCreated)
	}

	var createResp vtextCreateDocResponse
	if err := json.NewDecoder(createW.Body).Decode(&createResp); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if createResp.DocID == "" {
		t.Fatal("doc_id should not be empty")
	}

	listReq := authenticatedRequest(http.MethodGet, "/api/vtext/documents", "", "user-alice")
	listW := httptest.NewRecorder()
	handler.HandleVTextDocumentsRoot(listW, listReq)

	if listW.Code != http.StatusOK {
		t.Fatalf("list status: got %d, want %d", listW.Code, http.StatusOK)
	}

	var listResp vtextListDocsResponse
	if err := json.NewDecoder(listW.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(listResp.Documents) != 1 {
		t.Fatalf("documents: got %d, want 1", len(listResp.Documents))
	}
	if listResp.Documents[0].Title != "vtext alias doc" {
		t.Errorf("title: got %q, want %q", listResp.Documents[0].Title, "vtext alias doc")
	}
}

// --- Supervisor Recovery Visibility Tests ---

func TestSupervisorRecoveryVisible(t *testing.T) {
	// VAL-RUNTIME-009: supervisor recovery is externally visible.
	rt, _ := testAPISetup(t)

	// Subscribe to events.
	ch := rt.EventBus().Subscribe()
	defer rt.EventBus().Unsubscribe(ch)

	// Manually degrade and then recover the runtime.
	rt.SetHealth(types.HealthDegraded)

	// Should see degraded event.
	select {
	case ev := <-ch:
		if ev.Record.Kind != types.EventRuntimeDegraded {
			t.Errorf("event kind: got %q, want %q", ev.Record.Kind, types.EventRuntimeDegraded)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for degraded event")
	}

	// Recover to ready.
	rt.SetHealth(types.HealthReady)

	// Should see health event.
	select {
	case ev := <-ch:
		if ev.Record.Kind != types.EventRuntimeHealth {
			t.Errorf("event kind: got %q, want %q", ev.Record.Kind, types.EventRuntimeHealth)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for health event")
	}
}

func TestProviderFailureDoesNotCrashRuntime(t *testing.T) {
	// VAL-RUNTIME-008: provider failures surface without crashing the runtime.
	// Submit a failing task, verify the runtime still accepts new runs.
	dir := filepath.Join(os.TempDir(), "go-choir-m3-api-test")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	dbPath := filepath.Join(dir, t.Name()+".db")
	_ = os.Remove(dbPath)

	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	bus := events.NewEventBus()
	failProvider := &StubProvider{
		Delay:   10 * time.Millisecond,
		FailErr: errors.New("provider connection refused"),
	}

	cfg := Config{
		SandboxID:           "sandbox-test",
		StorePath:           dbPath,
		ProviderTimeout:     time.Second,
		SupervisionInterval: 1 * time.Hour,
	}

	rt := New(cfg, s, bus, failProvider)
	handler := NewAPIHandler(rt)

	t.Cleanup(func() {
		rt.Stop()
		_ = s.Close()
		_ = os.Remove(dbPath)
	})

	// Submit the failing task.
	body := `{"prompt":"will fail"}`
	req := authenticatedRequest(http.MethodPost, "/api/agent/loop", body, "user-alice")
	w := httptest.NewRecorder()
	handler.HandleRunSubmission(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("status: got %d, want %d", w.Code, http.StatusAccepted)
	}

	// Check the failed task status.
	var submitResp runSubmitResponse
	if err := json.NewDecoder(w.Body).Decode(&submitResp); err != nil {
		t.Fatalf("decode submit response: %v", err)
	}
	waitForRunTerminalState(t, rt, submitResp.RunID, "user-alice", 5*time.Second)

	statusReq := authenticatedRequest(http.MethodGet,
		fmt.Sprintf("/api/agent/status?loop_id=%s", submitResp.RunID), "", "user-alice")
	statusW := httptest.NewRecorder()
	handler.HandleRunStatus(statusW, statusReq)

	if statusW.Code != http.StatusOK {
		t.Fatalf("status code: got %d, want %d", statusW.Code, http.StatusOK)
	}

	var statusResp runStatusResponse
	if err := json.NewDecoder(statusW.Body).Decode(&statusResp); err != nil {
		t.Fatalf("decode status response: %v", err)
	}

	if statusResp.State != types.RunFailed {
		t.Errorf("state: got %q, want %q", statusResp.State, types.RunFailed)
	}

	// The runtime should still accept new runs.
	newBody := `{"prompt":"after failure"}`
	newReq := authenticatedRequest(http.MethodPost, "/api/agent/loop", newBody, "user-alice")
	newW := httptest.NewRecorder()

	// Replace the provider with a working one for the new task.
	rt.provider = NewStubProvider(50 * time.Millisecond)

	handler.HandleRunSubmission(newW, newReq)

	if newW.Code != http.StatusAccepted {
		t.Errorf("status after failure: got %d, want %d", newW.Code, http.StatusAccepted)
	}
}

// --- AuthenticateUser Tests ---

func TestAuthenticateUserMissing(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/agent/status", nil)
	_, err := authenticateUser(req)
	if err == nil {
		t.Error("expected error for missing auth header")
	}
}

func TestAuthenticateUserPresent(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/agent/status", nil)
	req.Header.Set("X-Authenticated-User", "user-alice")

	user, err := authenticateUser(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user != "user-alice" {
		t.Errorf("user: got %q, want user-alice", user)
	}
}

// --- Provider Bridge Health Visibility ---

func TestHandleHealthReportsBridgeProvider(t *testing.T) {
	// When a bridge provider is active, the health endpoint should report
	// its name (e.g., "bedrock" or "zai") instead of "stub", so operators
	// can distinguish real-provider paths from canned responses.

	dir := filepath.Join(os.TempDir(), "go-choir-m3-api-test")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	dbPath := filepath.Join(dir, t.Name()+".db")
	_ = os.Remove(dbPath)

	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	bus := events.NewEventBus()

	// Use a mock bridge provider instead of the stub.
	bridge := &mockBridgeProvider{name: "bedrock", result: "test"}

	cfg := Config{
		SandboxID:           "sandbox-test",
		StorePath:           dbPath,
		ProviderTimeout:     50 * time.Millisecond,
		SupervisionInterval: 1 * time.Hour,
	}

	rt := New(cfg, s, bus, bridge)
	handler := NewAPIHandler(rt)

	t.Cleanup(func() {
		rt.Stop()
		_ = s.Close()
		_ = os.Remove(dbPath)
	})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	handler.HandleHealth(w, req)

	var resp runtimeHealthResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.ActiveProvider != "bedrock" {
		t.Errorf("active_provider: got %q, want bedrock", resp.ActiveProvider)
	}
}
