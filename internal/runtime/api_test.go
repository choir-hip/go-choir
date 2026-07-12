//go:build comprehensive

package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// --- Task Submission Tests ---

func TestHandlePromptBarCreatesServerOwnedConductorRun(t *testing.T) {
	t.Parallel()
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
	if got := metadataStringValue(rec.Metadata, "requested_app"); got != AgentProfileTexture {
		t.Fatalf("requested_app: got %q, want %q", got, AgentProfileTexture)
	}
	if got := metadataStringValue(rec.Metadata, "seed_prompt"); got != "Draft a research plan" {
		t.Fatalf("seed_prompt: got %q", got)
	}
	var decision conductorDecision
	if err := json.Unmarshal([]byte(rec.Result), &decision); err != nil {
		t.Fatalf("decode prompt-bar decision: %v\n%s", err, rec.Result)
	}
	if decision.Action != "open_app" || decision.App != AgentProfileTexture || decision.DocID == "" {
		t.Fatalf("prompt-bar decision = %+v, want immediate Texture route", decision)
	}
	// Materialized Texture routes no longer carry conductor initial_content; the
	// prompt-bar text becomes the canonical V0 user revision instead.
	if decision.InitialContent != "" {
		t.Fatalf("initial_content = %q, want empty (texture owns canonical content)", decision.InitialContent)
	}
	doc, err := rt.store.GetDocument(context.Background(), decision.DocID, "user-alice")
	if err != nil {
		t.Fatalf("get materialized document: %v", err)
	}
	if doc.CurrentRevisionID == "" {
		t.Fatalf("materialized document has no current revision: %+v", doc)
	}
	rev, err := rt.store.GetRevision(context.Background(), doc.CurrentRevisionID, "user-alice")
	if err != nil {
		t.Fatalf("get seed prompt revision: %v", err)
	}
	if rev.Content != "Draft a research plan" {
		t.Fatalf("seed revision content = %q, want exact owner prompt as canonical V0", rev.Content)
	}
	revMeta := decodeRevisionMetadata(rev.Metadata)
	if metadataString(revMeta, "seed_prompt") != "Draft a research plan" {
		t.Fatalf("seed revision provenance metadata = %#v, want seed prompt", revMeta)
	}
	if metadataBoolValue(revMeta, "prompt_bar_instruction_revision") {
		t.Fatalf("seed revision must not carry prompt_bar_instruction_revision: %v", revMeta["prompt_bar_instruction_revision"])
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
	textureAgent, err := rt.store.GetAgent(context.Background(), currentTextureAgentID(decision.DocID))
	if err != nil {
		t.Fatalf("Texture agent missing: %v", err)
	}
	if textureAgent.Profile != AgentProfileTexture || textureAgent.Role != AgentProfileTexture {
		t.Fatalf("Texture agent identity = %q/%q, want %q/%q", textureAgent.Profile, textureAgent.Role, AgentProfileTexture, AgentProfileTexture)
	}
}

func TestPromptBarSubmissionDoesNotActivateIngestionEvent(t *testing.T) {
	t.Parallel()
	rt, handler := testAPISetup(t)

	req := authenticatedRequest(http.MethodPost, "/api/prompt-bar", `{"text":"Write a Universal Wire story about AI policy"}`, "user-alice")
	w := httptest.NewRecorder()
	handler.HandlePromptBar(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("status: got %d, want %d; body=%s", w.Code, http.StatusAccepted, w.Body.String())
	}
	var resp promptBarSubmitResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	rec, err := rt.GetRun(context.Background(), resp.SubmissionID, "user-alice")
	if err != nil {
		t.Fatalf("get run: %v", err)
	}
	if got := metadataStringValue(rec.Metadata, "input_source"); got != "prompt_bar" {
		t.Fatalf("input_source: got %q, want prompt_bar", got)
	}
	if got := metadataStringValue(rec.Metadata, "activation_origin"); got == "ingestion_event" {
		t.Fatalf("prompt-bar must not activate ingestion events, got activation_origin=%q", got)
	}
	if raw, ok := rec.Metadata["ingestion_event_ids"]; ok {
		switch ids := raw.(type) {
		case []any:
			if len(ids) > 0 {
				t.Fatalf("prompt-bar must not carry ingestion_event_ids, got %v", ids)
			}
		case []string:
			if len(ids) > 0 {
				t.Fatalf("prompt-bar must not carry ingestion_event_ids, got %v", ids)
			}
		default:
			t.Fatalf("unexpected ingestion_event_ids type %T", raw)
		}
	}
}

func TestHandlePromptBarRejectsBrowserRuntimeMetadata(t *testing.T) {
	t.Parallel()
	_, handler := testAPISetup(t)

	body := `{"text":"do work","metadata":{"agent_profile":"super"},"agent_role":"super","model":"x"}`
	req := authenticatedRequest(http.MethodPost, "/api/prompt-bar", body, "user-alice")
	w := httptest.NewRecorder()

	handler.HandlePromptBar(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleModelPolicyResolveUsesOverlayFile(t *testing.T) {
	t.Parallel()
	rt, handler := testAPISetup(t)
	policyPath := filepath.Join(t.TempDir(), "System", "model-policy.toml")
	overlayDir := filepath.Join(filepath.Dir(policyPath), "model-policy-overlays")
	if err := os.MkdirAll(overlayDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(policyPath, []byte(`
[defaults]
fallback_provider = "deepseek"
fallback_model = "deepseek-v4-flash"

[roles.researcher]
provider = "deepseek"
model = "deepseek-v4-flash"
reasoning = "medium"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	future := time.Now().UTC().Add(time.Hour).Format(time.RFC3339)
	if err := os.WriteFile(filepath.Join(overlayDir, "mimo-eval.toml"), []byte(`
[overlay]
expires_at = "`+future+`"

[roles.researcher]
provider = "xiaomi"
model = "mimo-v2.5-pro"
reasoning = "medium"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	rt.cfg.ModelPolicyPath = policyPath

	req := authenticatedRequest(http.MethodGet, "/api/model-policy/resolve?role=researcher&overlay_id=mimo-eval", "", "user-alice")
	w := httptest.NewRecorder()

	handler.HandleModelPolicyRouter(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d want %d body=%s", w.Code, http.StatusOK, w.Body.String())
	}
	var resp modelPolicyResolveResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Role != AgentProfileResearcher || resp.OverlayID != "mimo-eval" {
		t.Fatalf("route response identity = %+v", resp)
	}
	if resp.Provider != "xiaomi" || resp.Model != "mimo-v2.5-pro" || resp.ReasoningEffort != "medium" {
		t.Fatalf("selection = %+v", resp)
	}
	if resp.Source != filepath.Join(overlayDir, "mimo-eval.toml") {
		t.Fatalf("source = %q", resp.Source)
	}
	if resp.PolicyError != "" {
		t.Fatalf("policy_error = %q", resp.PolicyError)
	}
}

func TestHandleTexturePromptEvalPinsOverlayAcrossTextureRoute(t *testing.T) {
	t.Parallel()
	rt, handler := testAPISetup(t)
	policyPath := filepath.Join(t.TempDir(), "System", "model-policy.toml")
	overlayDir := filepath.Join(filepath.Dir(policyPath), "model-policy-overlays")
	if err := os.MkdirAll(overlayDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(policyPath, []byte(`
[defaults]
fallback_provider = "deepseek"
fallback_model = "deepseek-v4-flash"

[roles.texture]
provider = "xiaomi"
model = "mimo-v2.5"
reasoning = "medium"

[roles.researcher]
provider = "deepseek"
model = "deepseek-v4-flash"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	future := time.Now().UTC().Add(time.Hour).Format(time.RFC3339)
	if err := os.WriteFile(filepath.Join(overlayDir, "glm-medium.toml"), []byte(`
[overlay]
expires_at = "`+future+`"

[roles.texture]
provider = "zai"
model = "glm-5.2"
reasoning = "medium"

[roles.researcher]
provider = "zai"
model = "glm-5.2"
reasoning = "medium"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	rt.cfg.ModelPolicyPath = policyPath

	body := `{"text":"Write a briefing about new AI infra in 2026 with live evidence.","model_policy_overlay_id":"glm-medium"}`
	w := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/evals/texture-prompt", body, "user-alice")
	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202; body=%s", w.Code, w.Body.String())
	}
	var resp texturePromptEvalStartResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.SubmissionID == "" || resp.DocID == "" {
		t.Fatalf("response handles = %+v", resp)
	}
	if resp.Provider != "zai" || resp.Model != "glm-5.2" || resp.ReasoningEffort != "medium" {
		t.Fatalf("texture arm resolution = %+v", resp)
	}
	if resp.StatusURL != "/api/prompt-bar/submissions/"+resp.SubmissionID {
		t.Fatalf("status url = %q", resp.StatusURL)
	}

	// The conductor run carries the overlay for the trajectory.
	conductor, err := rt.GetRun(context.Background(), resp.SubmissionID, "user-alice")
	if err != nil {
		t.Fatalf("GetRun conductor: %v", err)
	}
	if got := metadataStringValue(conductor.Metadata, runMetadataLLMPolicyOverlayID); got != "glm-medium" {
		t.Fatalf("conductor overlay = %q; metadata=%+v", got, conductor.Metadata)
	}

	// The Texture revision run on the doc channel must resolve to the pinned arm.
	runs, err := rt.Store().ListRunsByChannel(context.Background(), "user-alice", resp.DocID, 20)
	if err != nil {
		t.Fatalf("ListRunsByChannel: %v", err)
	}
	var textureRun *types.RunRecord
	for i := range runs {
		if agentProfileForRun(&runs[i]) == AgentProfileTexture {
			textureRun = &runs[i]
			break
		}
	}
	if textureRun == nil {
		t.Fatalf("expected a texture run on doc channel %s; runs=%d", resp.DocID, len(runs))
	}
	if got := metadataStringValue(textureRun.Metadata, runMetadataLLMPolicyOverlayID); got != "glm-medium" {
		t.Fatalf("texture run overlay = %q; metadata=%+v", got, textureRun.Metadata)
	}
	if got := metadataStringValue(textureRun.Metadata, runMetadataLLMModel); got != "glm-5.2" {
		t.Fatalf("texture run model = %q, want glm-5.2; metadata=%+v", got, textureRun.Metadata)
	}
}

func TestHandleTexturePromptEvalRejectsMissingOverlay(t *testing.T) {
	t.Parallel()
	_, handler := testAPISetup(t)
	body := `{"text":"hello"}`
	w := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/evals/texture-prompt", body, "user-alice")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", w.Code, w.Body.String())
	}
}

func TestRunAcceptanceSynthesizeDerivesExportLevelRecord(t *testing.T) {
	t.Parallel()
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
	for _, want := range []string{"submitted", "texture_opened", "super_requested", "worker_leased", "worker_delegated", "app_package_published", "app_adoption_verified"} {
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

func TestRunAcceptanceSynthesizeDoesNotAcceptPromptTextureOnlySmoke(t *testing.T) {
	t.Parallel()
	_, handler := testAPISetup(t)

	prompt := "Mission lifecycle cutover staging smoke creates a prompt-bar Texture route."
	submitBody := fmt.Sprintf(`{"text":%q}`, prompt)
	submitW := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/prompt-bar", submitBody, "user-alice")
	if submitW.Code != http.StatusAccepted {
		t.Fatalf("prompt-bar status = %d, body=%s", submitW.Code, submitW.Body.String())
	}
	var submitted promptBarSubmitResponse
	if err := json.Unmarshal(submitW.Body.Bytes(), &submitted); err != nil {
		t.Fatalf("decode prompt-bar response: %v", err)
	}

	for _, targetMissionID := range []string{"mission-lifecycle-cutover-v0", "mission-lifecycle-cutover-m3.1-v0"} {
		t.Run(targetMissionID, func(t *testing.T) {
			body := fmt.Sprintf(`{"target_mission_id":%q,"trajectory_id":%q,"source_prompt_or_objective":%q}`, targetMissionID, submitted.SubmissionID, prompt)
			w := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/run-acceptances/synthesize", body, "user-alice")
			if w.Code != http.StatusAccepted {
				t.Fatalf("synthesize status = %d, body=%s", w.Code, w.Body.String())
			}
			var rec types.RunAcceptanceRecord
			if err := json.Unmarshal(w.Body.Bytes(), &rec); err != nil {
				t.Fatalf("decode acceptance: %v", err)
			}
			if rec.AcceptanceLevel != types.RunAcceptanceStagingSmokeLevel || rec.State != types.RunAcceptanceBlocked {
				t.Fatalf("acceptance = %s/%s, want staging-smoke-level/blocked for prompt/Texture-only smoke; checkpoints=%+v invariants=%+v risks=%+v", rec.AcceptanceLevel, rec.State, rec.Checkpoints, rec.InvariantChecks, rec.FailureResidualRisks)
			}
			for _, want := range []string{"submitted", "texture_opened"} {
				if !acceptanceHasCheckpoint(rec, want) {
					t.Fatalf("missing checkpoint %q in %+v", want, rec.Checkpoints)
				}
			}
			for _, check := range rec.InvariantChecks {
				if (check.Name == "product_path_observed" || check.Name == "worker_mutation_bounded") && check.State != "passed" {
					t.Fatalf("%s = %+v, want passed", check.Name, check)
				}
			}
			if strings.Contains(strings.Join(rec.FailureResidualRisks, "\n"), "acceptance invariant product_path_observed is blocked") ||
				strings.Contains(strings.Join(rec.FailureResidualRisks, "\n"), "acceptance invariant worker_mutation_bounded is blocked") {
				t.Fatalf("prompt/Texture smoke should not carry invariant blocker risks: %+v", rec.FailureResidualRisks)
			}
		})
	}
}

func TestRunAcceptanceSynthesizeCountsTimedOutDelegateWithReviewableExport(t *testing.T) {
	t.Parallel()
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

func TestRunAcceptanceSynthesizeAcceptsSourcePackageWithoutRecipientAdoption(t *testing.T) {
	t.Parallel()
	rt, handler := testAPISetup(t)
	seedRunAcceptanceSourcePackageOnlyTrajectory(t, rt)

	body := `{"target_mission_id":"mission-run-acceptance-source-package","trajectory_id":"traj-source-package"}`
	w := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/run-acceptances/synthesize", body, "user-alice")
	if w.Code != http.StatusAccepted {
		t.Fatalf("synthesize status = %d, body=%s", w.Code, w.Body.String())
	}
	var rec types.RunAcceptanceRecord
	if err := json.Unmarshal(w.Body.Bytes(), &rec); err != nil {
		t.Fatalf("decode acceptance: %v", err)
	}
	if rec.AcceptanceLevel != types.RunAcceptanceExportLevel || rec.State != types.RunAcceptanceAccepted {
		t.Fatalf("acceptance = %s/%s, want export-level/accepted; checkpoints=%+v invariants=%+v contracts=%+v", rec.AcceptanceLevel, rec.State, rec.Checkpoints, rec.InvariantChecks, rec.VerifierContracts)
	}
	for _, want := range []string{"submitted", "texture_opened", "super_requested", "worker_leased", "worker_delegated", "app_package_published"} {
		if !acceptanceHasCheckpoint(rec, want) {
			t.Fatalf("missing checkpoint %q in %+v", want, rec.Checkpoints)
		}
	}
	if acceptanceHasCheckpoint(rec, "app_adoption_verified") || acceptanceHasCheckpoint(rec, "app_adoption_promoted") {
		t.Fatalf("source package proof must not synthesize recipient adoption checkpoints: %+v", rec.Checkpoints)
	}
	for _, contract := range rec.VerifierContracts {
		if contract.Name == "export-level-product-path" && contract.State != "passed" {
			t.Fatalf("export-level-product-path contract = %+v, want passed", contract)
		}
	}
}

func TestRunAcceptanceSynthesizeAcceptsRuntimeSupervisionWithoutAppPackage(t *testing.T) {
	t.Parallel()
	rt, handler := testAPISetup(t)
	seedRunAcceptanceRuntimeSupervisionTrajectory(t, rt)

	body := `{"target_mission_id":"mission-runtime-supervision","trajectory_id":"traj-runtime-supervision"}`
	w := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/run-acceptances/synthesize", body, "user-alice")
	if w.Code != http.StatusAccepted {
		t.Fatalf("synthesize status = %d, body=%s", w.Code, w.Body.String())
	}
	var rec types.RunAcceptanceRecord
	if err := json.Unmarshal(w.Body.Bytes(), &rec); err != nil {
		t.Fatalf("decode acceptance: %v", err)
	}
	if rec.AcceptanceLevel != types.RunAcceptanceStagingSmokeLevel || rec.State != types.RunAcceptanceAccepted {
		t.Fatalf("acceptance = %s/%s, want staging-smoke-level/accepted; checkpoints=%+v invariants=%+v risks=%+v", rec.AcceptanceLevel, rec.State, rec.Checkpoints, rec.InvariantChecks, rec.FailureResidualRisks)
	}
	if !acceptanceHasCheckpoint(rec, "worker_delegated") || !acceptanceHasCheckpoint(rec, "worker_supervision_observed") {
		t.Fatalf("runtime supervision checkpoints missing: %+v", rec.Checkpoints)
	}
	if acceptanceHasCheckpoint(rec, "app_package_published") {
		t.Fatalf("runtime-only supervision proof must not synthesize app package checkpoint: %+v", rec.Checkpoints)
	}
	delegated := acceptanceCheckpoint(rec, "worker_delegated", "passed")
	if delegated == nil || delegated.Details["acceptance_contract"] != "runtime_supervision" {
		t.Fatalf("worker_delegated did not record runtime supervision contract: %+v", delegated)
	}
	for _, check := range rec.InvariantChecks {
		if check.Name == "worker_mutation_bounded" && check.State != "passed" {
			t.Fatalf("runtime supervision should satisfy bounded worker invariant, got %+v", check)
		}
	}
}

func TestRunAcceptanceSynthesizeRecordsWorkerDelegateBlocker(t *testing.T) {
	t.Parallel()
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
	if !strings.Contains(lastError, "worker exited before acceptance evidence") {
		t.Fatalf("last_error = %q, want worker exit detail", lastError)
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
	t.Parallel()
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
		"worker_event_summary":         []map[string]any{{"kind": "tool.result", "tool": "update_coagent", "output_excerpt": "precise blocker"}},
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
	t.Parallel()
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
	t.Parallel()
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
	if len(rec.RollbackRefs) != 1 {
		t.Fatalf("promoted adoption rollback refs = %+v, want one source ref", rec.RollbackRefs)
	}
	if rec.RollbackRefs[0].Kind != "source_ref" || rec.RollbackRefs[0].Ref != "refs/computers/computer-b/active-before-adoption" {
		t.Fatalf("promoted adoption rollback ref = %+v", rec.RollbackRefs[0])
	}
}

func TestRunAcceptanceSynthesizeAcceptsDirectProductAdoptionEvidence(t *testing.T) {
	t.Parallel()
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
	if rec.State != types.RunAcceptanceAccepted {
		t.Fatalf("state = %q, want accepted because product adoption evidence is trace-derived; invariants=%+v", rec.State, rec.InvariantChecks)
	}
	for _, want := range []string{"product_path_observed", "worker_mutation_bounded"} {
		found := false
		for _, check := range rec.InvariantChecks {
			if check.Name == want {
				found = true
				if check.State != "passed" {
					t.Fatalf("%s state = %q, want passed", want, check.State)
				}
			}
		}
		if !found {
			t.Fatalf("missing invariant check %q in %+v", want, rec.InvariantChecks)
		}
	}
	if strings.Contains(strings.Join(rec.FailureResidualRisks, "\n"), "acceptance invariant product_path_observed is blocked") {
		t.Fatalf("unexpected product path residual risk: %+v", rec.FailureResidualRisks)
	}
	if len(rec.RollbackRefs) != 1 {
		t.Fatalf("direct product adoption rollback refs = %+v, want one source ref", rec.RollbackRefs)
	}
	if rec.RollbackRefs[0].Kind != "source_ref" || rec.RollbackRefs[0].Ref != "refs/computers/computer-b/active-before-adoption" {
		t.Fatalf("direct product adoption rollback ref = %+v", rec.RollbackRefs[0])
	}
}

func seedRunAcceptanceTrajectory(t *testing.T, rt *Runtime) {
	seedRunAcceptanceTrajectoryWithDelegateStatus(t, rt, "worker_run_completed", string(types.RunCompleted), "")
}

func seedRunAcceptanceSourcePackageOnlyTrajectory(t *testing.T, rt *Runtime) {
	t.Helper()
	ctx := context.Background()
	now := time.Now().UTC()
	finishedAt := now.Add(15 * time.Second)
	runs := []types.RunRecord{
		{
			RunID:        "run-conductor-source-package",
			AgentID:      "agent-conductor-source-package",
			ChannelID:    "channel-source-package",
			AgentProfile: AgentProfileConductor,
			AgentRole:    AgentProfileConductor,
			OwnerID:      "user-alice",
			SandboxID:    "sandbox-test",
			State:        types.RunCompleted,
			Prompt:       "Publish one reviewable AppChangePackage.",
			Result:       `{"action":"open_app","app":"texture","doc_id":"doc-source-package"}`,
			CreatedAt:    now,
			UpdatedAt:    finishedAt,
			FinishedAt:   &finishedAt,
			Metadata: map[string]any{
				runMetadataAgentProfile: AgentProfileConductor,
				runMetadataAgentRole:    AgentProfileConductor,
				runMetadataTrajectoryID: "traj-source-package",
				runMetadataDesktopID:    types.PrimaryDesktopID,
				"input_source":          "prompt_bar",
			},
		},
		{
			RunID:            "run-texture-source-package",
			AgentID:          "agent-texture-source-package",
			ChannelID:        "channel-source-package",
			RequestedByRunID: "run-conductor-source-package",
			AgentProfile:     AgentProfileTexture,
			AgentRole:        AgentProfileTexture,
			OwnerID:          "user-alice",
			SandboxID:        "sandbox-test",
			State:            types.RunCompleted,
			Prompt:           "Own the package proof document.",
			CreatedAt:        now.Add(3 * time.Second),
			UpdatedAt:        now.Add(4 * time.Second),
			FinishedAt:       &finishedAt,
			Metadata: map[string]any{
				runMetadataAgentProfile: AgentProfileTexture,
				runMetadataAgentRole:    AgentProfileTexture,
				runMetadataTrajectoryID: "traj-source-package",
				runMetadataDesktopID:    types.PrimaryDesktopID,
			},
		},
		{
			RunID:            "run-super-source-package",
			AgentID:          "agent-super-source-package",
			ChannelID:        "channel-source-package",
			RequestedByRunID: "run-texture-source-package",
			AgentProfile:     AgentProfileSuper,
			AgentRole:        AgentProfileSuper,
			OwnerID:          "user-alice",
			SandboxID:        "sandbox-test",
			State:            types.RunCompleted,
			Prompt:           "Delegate a worker and publish one AppChangePackage.",
			CreatedAt:        now.Add(5 * time.Second),
			UpdatedAt:        now.Add(12 * time.Second),
			FinishedAt:       &finishedAt,
			Metadata: map[string]any{
				runMetadataAgentProfile: AgentProfileSuper,
				runMetadataAgentRole:    AgentProfileSuper,
				runMetadataTrajectoryID: "traj-source-package",
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
		EventID:      "event-submit-source-package",
		RunID:        "run-conductor-source-package",
		AgentID:      "agent-conductor-source-package",
		ChannelID:    "channel-source-package",
		OwnerID:      "user-alice",
		TrajectoryID: "traj-source-package",
		Timestamp:    now,
		Kind:         types.EventRunSubmitted,
		Payload:      json.RawMessage(`{"input_source":"prompt_bar"}`),
	})
	appendAcceptanceEvent(t, rt, types.EventRecord{
		EventID:      "event-texture-source-package",
		RunID:        "run-texture-source-package",
		AgentID:      "agent-texture-source-package",
		ChannelID:    "channel-source-package",
		OwnerID:      "user-alice",
		TrajectoryID: "traj-source-package",
		Timestamp:    now.Add(4 * time.Second),
		Kind:         types.EventTextureDocumentRevisionCreated,
		Payload:      json.RawMessage(`{"doc_id":"doc-source-package","revision_id":"rev-source-package"}`),
	})
	appendAcceptanceToolResultForTrajectory(t, rt, "event-super-source-package", "run-texture-source-package", "agent-texture-source-package", "traj-source-package", "channel-source-package", now.Add(5*time.Second), "request_super_execution", map[string]any{
		"agent_id": "agent-super-source-package",
		"loop_id":  "run-super-source-package",
		"state":    "running",
	})
	appendAcceptanceToolResultForTrajectory(t, rt, "event-worker-lease-source-package", "run-super-source-package", "agent-super-source-package", "traj-source-package", "channel-source-package", now.Add(6*time.Second), "request_worker_vm", map[string]any{
		"status": "worker_requested",
		"handle": map[string]any{
			"kind":          "worker",
			"worker_id":     "worker-source-package",
			"vm_id":         "vm-source-package",
			"desktop_id":    types.PrimaryDesktopID,
			"machine_class": "standard",
			"sandbox_url":   "http://127.0.0.1:8085",
		},
	})
	appendAcceptanceToolResultForTrajectory(t, rt, "event-delegate-source-package", "run-super-source-package", "agent-super-source-package", "traj-source-package", "channel-source-package", now.Add(10*time.Second), "delegate_worker_vm", map[string]any{
		"status":                  "worker_run_completed",
		"state":                   string(types.RunCompleted),
		"worker_vm_id":            "vm-source-package",
		"worker_id":               "worker-source-package",
		"worker_sandbox_url":      "http://127.0.0.1:8085",
		"loop_id":                 "run-worker-source-package",
		"event_count":             12,
		"worker_child_run_ids":    []string{"run-implementation-source-package", "run-verifier-source-package"},
		"worker_event_summary":    []map[string]any{{"kind": "tool.result", "tool": "publish_app_change_package", "output_excerpt": "published pkg-source-only"}},
		"worker_spawned_profiles": []string{AgentProfileCoSuper},
		"app_change_packages": []map[string]any{{
			"status":                         "published_unlisted",
			"package_id":                     "pkg-source-only",
			"app_id":                         "portfolio-source-package",
			"base_sha":                       "base-source-package",
			"candidate_head_sha":             "worker-head-source-package",
			"source_computer_id":             "computer-source",
			"source_candidate_id":            "candidate-source",
			"candidate_source_ref":           "refs/computers/computer-source/candidates/candidate-source",
			"package_manifest_sha256":        "sha256-manifest-source-package",
			"runtime_source_delta_sha256":    "sha256-runtime-delta-source-package",
			"ui_source_delta_sha256":         "sha256-ui-delta-source-package",
			"recipient_build_required":       true,
			"source_runtime_artifact_digest": "runtime-source-digest",
			"source_ui_artifact_digest":      "ui-source-digest",
		}},
	})
	appendAcceptanceEvent(t, rt, types.EventRecord{
		EventID:      "event-app-package-source-package",
		RunID:        "run-worker-source-package",
		AgentID:      "agent-super-source-package",
		ChannelID:    "channel-source-package",
		OwnerID:      "user-alice",
		TrajectoryID: "traj-source-package",
		Timestamp:    now.Add(11 * time.Second),
		Kind:         types.EventAppChangePackagePublished,
		Payload:      json.RawMessage(`{"package_id":"pkg-source-only","app_id":"portfolio-source-package","source_computer_id":"computer-source","source_candidate_id":"candidate-source","candidate_source_ref":"refs/computers/computer-source/candidates/candidate-source","package_manifest_sha":"sha256-manifest-source-package"}`),
	})
}

func seedRunAcceptanceRuntimeSupervisionTrajectory(t *testing.T, rt *Runtime) {
	t.Helper()
	ctx := context.Background()
	now := time.Now().UTC()
	finishedAt := now.Add(15 * time.Second)
	runs := []types.RunRecord{
		{
			RunID:        "run-conductor-runtime-supervision",
			AgentID:      "agent-conductor-runtime-supervision",
			ChannelID:    "channel-runtime-supervision",
			AgentProfile: AgentProfileConductor,
			AgentRole:    AgentProfileConductor,
			OwnerID:      "user-alice",
			SandboxID:    "sandbox-test",
			State:        types.RunCompleted,
			Prompt:       "Prove async worker supervision without publishing a package.",
			Result:       `{"action":"open_app","app":"texture","doc_id":"doc-runtime-supervision"}`,
			CreatedAt:    now,
			UpdatedAt:    finishedAt,
			FinishedAt:   &finishedAt,
			Metadata: map[string]any{
				runMetadataAgentProfile: AgentProfileConductor,
				runMetadataAgentRole:    AgentProfileConductor,
				runMetadataTrajectoryID: "traj-runtime-supervision",
				runMetadataDesktopID:    types.PrimaryDesktopID,
				"input_source":          "prompt_bar",
			},
		},
		{
			RunID:            "run-texture-runtime-supervision",
			AgentID:          "agent-texture-runtime-supervision",
			ChannelID:        "channel-runtime-supervision",
			RequestedByRunID: "run-conductor-runtime-supervision",
			AgentProfile:     AgentProfileTexture,
			AgentRole:        AgentProfileTexture,
			OwnerID:          "user-alice",
			SandboxID:        "sandbox-test",
			State:            types.RunCompleted,
			Prompt:           "Own the runtime supervision document.",
			CreatedAt:        now.Add(3 * time.Second),
			UpdatedAt:        now.Add(4 * time.Second),
			FinishedAt:       &finishedAt,
			Metadata: map[string]any{
				runMetadataAgentProfile: AgentProfileTexture,
				runMetadataAgentRole:    AgentProfileTexture,
				runMetadataTrajectoryID: "traj-runtime-supervision",
				runMetadataDesktopID:    types.PrimaryDesktopID,
			},
		},
		{
			RunID:            "run-super-runtime-supervision",
			AgentID:          "agent-super-runtime-supervision",
			ChannelID:        "channel-runtime-supervision",
			RequestedByRunID: "run-texture-runtime-supervision",
			AgentProfile:     AgentProfileSuper,
			AgentRole:        AgentProfileSuper,
			OwnerID:          "user-alice",
			SandboxID:        "sandbox-test",
			State:            types.RunCompleted,
			Prompt:           "Delegate a worker and collect a Texture worker update. Do not publish an AppChangePackage.",
			CreatedAt:        now.Add(5 * time.Second),
			UpdatedAt:        now.Add(12 * time.Second),
			FinishedAt:       &finishedAt,
			Metadata: map[string]any{
				runMetadataAgentProfile: AgentProfileSuper,
				runMetadataAgentRole:    AgentProfileSuper,
				runMetadataTrajectoryID: "traj-runtime-supervision",
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
		EventID:      "event-submit-runtime-supervision",
		RunID:        "run-conductor-runtime-supervision",
		AgentID:      "agent-conductor-runtime-supervision",
		ChannelID:    "channel-runtime-supervision",
		OwnerID:      "user-alice",
		TrajectoryID: "traj-runtime-supervision",
		Timestamp:    now,
		Kind:         types.EventRunSubmitted,
		Payload:      json.RawMessage(`{"input_source":"prompt_bar"}`),
	})
	appendAcceptanceEvent(t, rt, types.EventRecord{
		EventID:      "event-texture-runtime-supervision",
		RunID:        "run-texture-runtime-supervision",
		AgentID:      "agent-texture-runtime-supervision",
		ChannelID:    "channel-runtime-supervision",
		OwnerID:      "user-alice",
		TrajectoryID: "traj-runtime-supervision",
		Timestamp:    now.Add(4 * time.Second),
		Kind:         types.EventTextureDocumentRevisionCreated,
		Payload:      json.RawMessage(`{"doc_id":"doc-runtime-supervision","revision_id":"rev-runtime-supervision"}`),
	})
	appendAcceptanceToolResultForTrajectory(t, rt, "event-super-runtime-supervision", "run-texture-runtime-supervision", "agent-texture-runtime-supervision", "traj-runtime-supervision", "channel-runtime-supervision", now.Add(5*time.Second), "request_super_execution", map[string]any{
		"agent_id": "agent-super-runtime-supervision",
		"loop_id":  "run-super-runtime-supervision",
		"state":    "running",
	})
	appendAcceptanceToolResultForTrajectory(t, rt, "event-worker-lease-runtime-supervision", "run-super-runtime-supervision", "agent-super-runtime-supervision", "traj-runtime-supervision", "channel-runtime-supervision", now.Add(6*time.Second), "request_worker_vm", map[string]any{
		"status": "worker_requested",
		"handle": map[string]any{
			"kind":          "worker",
			"worker_id":     "worker-runtime-supervision",
			"vm_id":         "vm-runtime-supervision",
			"desktop_id":    types.PrimaryDesktopID,
			"machine_class": "worker-small",
			"sandbox_url":   "http://127.0.0.1:8085",
		},
	})
	appendAcceptanceToolResultForTrajectory(t, rt, "event-start-runtime-supervision", "run-super-runtime-supervision", "agent-super-runtime-supervision", "traj-runtime-supervision", "channel-runtime-supervision", now.Add(7*time.Second), "start_worker_delegation", map[string]any{
		"status":             "worker_run_active",
		"state":              string(types.RunRunning),
		"worker_vm_id":       "vm-runtime-supervision",
		"worker_id":          "worker-runtime-supervision",
		"worker_sandbox_url": "http://127.0.0.1:8085",
		"loop_id":            "run-worker-runtime-supervision",
		"event_count":        3,
	})
	appendAcceptanceToolErrorForTrajectory(t, rt, "event-duplicate-start-runtime-supervision", "run-super-runtime-supervision", "agent-super-runtime-supervision", "traj-runtime-supervision", "channel-runtime-supervision", now.Add(8*time.Second), "start_worker_delegation", "tool_error: duplicate start_worker_delegation payload already planned in this turn; wait for the first worker result instead of starting the same worker delegation twice")
	appendAcceptanceToolResultForTrajectory(t, rt, "event-observe-runtime-supervision", "run-super-runtime-supervision", "agent-super-runtime-supervision", "traj-runtime-supervision", "channel-runtime-supervision", now.Add(9*time.Second), "observe_worker_delegation", map[string]any{
		"status":                       "worker_observed",
		"state":                        string(types.RunRunning),
		"worker_vm_id":                 "vm-runtime-supervision",
		"worker_id":                    "worker-runtime-supervision",
		"worker_sandbox_url":           "http://127.0.0.1:8085",
		"loop_id":                      "run-worker-runtime-supervision",
		"event_count":                  9,
		"worker_channel_message_count": 1,
		"worker_update_checkpoint":     "worker_submit_update_mirrored",
		"mirrored_worker_update_count": 1,
		"mirrored_worker_update_ids":   []string{"mirrored-worker-update-run-super-runtime-supervision-worker-direct-update"},
	})
	appendAcceptanceToolResultForTrajectory(t, rt, "event-finish-runtime-supervision", "run-super-runtime-supervision", "agent-super-runtime-supervision", "traj-runtime-supervision", "channel-runtime-supervision", now.Add(10*time.Second), "finish_worker_delegation", map[string]any{
		"status":                       "worker_run_completed",
		"state":                        string(types.RunCompleted),
		"worker_vm_id":                 "vm-runtime-supervision",
		"worker_id":                    "worker-runtime-supervision",
		"worker_sandbox_url":           "http://127.0.0.1:8085",
		"loop_id":                      "run-worker-runtime-supervision",
		"event_count":                  9,
		"worker_channel_message_count": 1,
		"worker_update_checkpoint":     "worker_submit_update_mirrored",
		"mirrored_worker_update_count": 1,
		"mirrored_worker_update_ids":   []string{"mirrored-worker-update-run-super-runtime-supervision-worker-direct-update"},
		"worker_event_summary": []map[string]any{{
			"kind":           "tool.result",
			"tool":           "update_coagent",
			"is_error":       false,
			"output_excerpt": `{"status":"submitted","update_id":"worker-direct-update"}`,
		}},
		"app_change_packages": []map[string]any{},
	})
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
			Result:       `{"action":"open_app","app":"texture","doc_id":"doc-acceptance"}`,
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
			RunID:            "run-texture-acceptance",
			AgentID:          "agent-texture-acceptance",
			ChannelID:        "channel-acceptance",
			RequestedByRunID: "run-conductor-acceptance",
			AgentProfile:     AgentProfileTexture,
			AgentRole:        AgentProfileTexture,
			OwnerID:          "user-alice",
			SandboxID:        "sandbox-test",
			State:            types.RunCompleted,
			Prompt:           "Own the acceptance document.",
			CreatedAt:        now.Add(3 * time.Second),
			UpdatedAt:        now.Add(4 * time.Second),
			FinishedAt:       &finishedAt,
			Metadata: map[string]any{
				runMetadataAgentProfile: AgentProfileTexture,
				runMetadataAgentRole:    AgentProfileTexture,
				runMetadataTrajectoryID: "traj-acceptance",
				runMetadataDesktopID:    types.PrimaryDesktopID,
			},
		},
		{
			RunID:            "run-super-acceptance",
			AgentID:          "agent-super-acceptance",
			ChannelID:        "channel-acceptance",
			RequestedByRunID: "run-texture-acceptance",
			AgentProfile:     AgentProfileSuper,
			AgentRole:        AgentProfileSuper,
			OwnerID:          "user-alice",
			SandboxID:        "sandbox-test",
			State:            types.RunCompleted,
			Prompt:           "Delegate a worker and publish an AppChangePackage.",
			CreatedAt:        now.Add(5 * time.Second),
			UpdatedAt:        now.Add(12 * time.Second),
			FinishedAt:       &finishedAt,
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
		EventID:      "event-texture-acceptance",
		RunID:        "run-texture-acceptance",
		AgentID:      "agent-texture-acceptance",
		ChannelID:    "channel-acceptance",
		OwnerID:      "user-alice",
		TrajectoryID: "traj-acceptance",
		Timestamp:    now.Add(4 * time.Second),
		Kind:         types.EventTextureDocumentRevisionCreated,
		Payload:      json.RawMessage(`{"doc_id":"doc-acceptance","revision_id":"rev-1"}`),
	})
	appendAcceptanceToolResult(t, rt, "event-super-acceptance", "run-texture-acceptance", "agent-texture-acceptance", now.Add(5*time.Second), "request_super_execution", map[string]any{
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
			Result:       `{"action":"open_app","app":"texture","doc_id":"doc-acceptance"}`,
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
			RunID:            "run-texture-acceptance",
			AgentID:          "agent-texture-acceptance",
			ChannelID:        "channel-acceptance",
			RequestedByRunID: "run-conductor-acceptance",
			AgentProfile:     AgentProfileTexture,
			AgentRole:        AgentProfileTexture,
			OwnerID:          "user-alice",
			SandboxID:        "sandbox-test",
			State:            types.RunCompleted,
			Prompt:           "Own the acceptance document.",
			CreatedAt:        now.Add(3 * time.Second),
			UpdatedAt:        now.Add(4 * time.Second),
			FinishedAt:       &finishedAt,
			Metadata: map[string]any{
				runMetadataAgentProfile: AgentProfileTexture,
				runMetadataAgentRole:    AgentProfileTexture,
				runMetadataTrajectoryID: "traj-acceptance",
				runMetadataDesktopID:    types.PrimaryDesktopID,
			},
		},
		{
			RunID:            "run-super-acceptance",
			AgentID:          "agent-super-acceptance",
			ChannelID:        "channel-acceptance",
			RequestedByRunID: "run-texture-acceptance",
			AgentProfile:     AgentProfileSuper,
			AgentRole:        AgentProfileSuper,
			OwnerID:          "user-alice",
			SandboxID:        "sandbox-test",
			State:            types.RunCompleted,
			Prompt:           "Delegate a worker and publish an AppChangePackage.",
			CreatedAt:        now.Add(5 * time.Second),
			UpdatedAt:        now.Add(12 * time.Second),
			FinishedAt:       &finishedAt,
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
		EventID:      "event-texture-acceptance",
		RunID:        "run-texture-acceptance",
		AgentID:      "agent-texture-acceptance",
		ChannelID:    "channel-acceptance",
		OwnerID:      "user-alice",
		TrajectoryID: "traj-acceptance",
		Timestamp:    now.Add(4 * time.Second),
		Kind:         types.EventTextureDocumentRevisionCreated,
		Payload:      json.RawMessage(`{"doc_id":"doc-acceptance","revision_id":"rev-1"}`),
	})
	appendAcceptanceToolResult(t, rt, "event-super-acceptance", "run-texture-acceptance", "agent-texture-acceptance", now.Add(5*time.Second), "request_super_execution", map[string]any{
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
	appendAcceptanceToolError(t, rt, "event-delegate-worker-failed-acceptance", "run-super-acceptance", "agent-super-acceptance", now.Add(10*time.Second), "delegate_worker_vm", `tool_error: worker run run-worker-acceptance ended in state failed: worker exited before acceptance evidence`)
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
			Result:       `{"action":"open_app","app":"texture","doc_id":"doc-pending-delegate"}`,
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
			RunID:            "run-texture-pending-delegate",
			AgentID:          "agent-texture-pending-delegate",
			ChannelID:        "channel-pending-delegate",
			RequestedByRunID: "run-conductor-pending-delegate",
			AgentProfile:     AgentProfileTexture,
			AgentRole:        AgentProfileTexture,
			OwnerID:          "user-alice",
			SandboxID:        "sandbox-test",
			State:            types.RunCompleted,
			Prompt:           "Own the acceptance document.",
			CreatedAt:        now.Add(3 * time.Second),
			UpdatedAt:        now.Add(4 * time.Second),
			FinishedAt:       &finishedAt,
			Metadata: map[string]any{
				runMetadataAgentProfile: AgentProfileTexture,
				runMetadataAgentRole:    AgentProfileTexture,
				runMetadataTrajectoryID: "traj-pending-delegate",
				runMetadataDesktopID:    types.PrimaryDesktopID,
			},
		},
		{
			RunID:            "run-super-pending-delegate",
			AgentID:          "agent-super-pending-delegate",
			ChannelID:        "channel-pending-delegate",
			RequestedByRunID: "run-texture-pending-delegate",
			AgentProfile:     AgentProfileSuper,
			AgentRole:        AgentProfileSuper,
			OwnerID:          "user-alice",
			SandboxID:        "sandbox-test",
			State:            types.RunRunning,
			Prompt:           "Delegate a worker and publish an AppChangePackage.",
			CreatedAt:        now.Add(5 * time.Second),
			UpdatedAt:        now.Add(8 * time.Second),
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
		EventID:      "event-texture-pending-delegate",
		RunID:        "run-texture-pending-delegate",
		AgentID:      "agent-texture-pending-delegate",
		ChannelID:    "channel-pending-delegate",
		OwnerID:      "user-alice",
		TrajectoryID: "traj-pending-delegate",
		Timestamp:    now.Add(4 * time.Second),
		Kind:         types.EventTextureDocumentRevisionCreated,
		Payload:      json.RawMessage(`{"doc_id":"doc-pending-delegate","revision_id":"rev-1"}`),
	})
	appendAcceptanceToolResult(t, rt, "event-super-pending-delegate", "run-texture-pending-delegate", "agent-texture-pending-delegate", now.Add(5*time.Second), "request_super_execution", map[string]any{
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
	appendAcceptanceToolResultForTrajectory(t, rt, eventID, runID, agentID, "traj-acceptance", "channel-acceptance", at, tool, output)
}

func appendAcceptanceToolResultForTrajectory(t *testing.T, rt *Runtime, eventID, runID, agentID, trajectoryID, channelID string, at time.Time, tool string, output map[string]any) {
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
		ChannelID:    channelID,
		OwnerID:      "user-alice",
		TrajectoryID: trajectoryID,
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
	appendAcceptanceToolErrorForTrajectory(t, rt, eventID, runID, agentID, "traj-acceptance", "channel-acceptance", at, tool, output)
}

func appendAcceptanceToolErrorForTrajectory(t *testing.T, rt *Runtime, eventID, runID, agentID, trajectoryID, channelID string, at time.Time, tool, output string) {
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
		ChannelID:    channelID,
		OwnerID:      "user-alice",
		TrajectoryID: trajectoryID,
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
}

func TestBrowserSessionRejectsDirectWorldBinding(t *testing.T) {
	t.Parallel()
	_, handler := testAPISetup(t)
	forged := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/browser/sessions", `{"vm_id":"vm-forged"}`, "user-alice")
	if forged.Code != http.StatusBadRequest {
		t.Fatalf("forged vm_id status = %d, want 400; body=%s", forged.Code, forged.Body.String())
	}
}

func TestBrowserSessionNavigateKeepsTextWhenOptionalSnapshotDumpsFail(t *testing.T) {
	t.Parallel()
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
if [ "$mode" = "text" ]; then
  printf 'Readable Source Page\n\nPrimary source text from fake Obscura\n'
  exit 0
fi
printf 'fake optional %s dump failed\n' "$mode" >&2
exit 2
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

	navigateW := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/browser/sessions/"+created.SessionID+"/navigate", `{"url":"https://example.com/source"}`, "user-alice")
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
	if !strings.Contains(navigated.TextSnapshot, "Primary source text from fake Obscura") {
		t.Fatalf("text_snapshot missing primary text: %q", navigated.TextSnapshot)
	}
	if navigated.HTMLSnapshot != "" {
		t.Fatalf("html_snapshot = %q, want empty optional artifact", navigated.HTMLSnapshot)
	}
	if len(navigated.Links) != 0 {
		t.Fatalf("links = %+v, want none after optional dump failure", navigated.Links)
	}
	if len(navigated.SnapshotWarnings) != 2 {
		t.Fatalf("snapshot_warnings = %+v, want links/html warnings", navigated.SnapshotWarnings)
	}
	joinedWarnings := strings.Join(navigated.SnapshotWarnings, "\n")
	if !strings.Contains(joinedWarnings, "links") || !strings.Contains(joinedWarnings, "html") {
		t.Fatalf("snapshot_warnings = %+v, want links/html dump warnings", navigated.SnapshotWarnings)
	}

	events, err := rt.Store().ListEventsByTrajectory(context.Background(), "user-alice", browserSessionTraceID(created.SessionID), 10)
	if err != nil {
		t.Fatalf("list browser trace events: %v", err)
	}
	if len(events) != 2 || events[1].Kind != types.EventBrowserNavigationCompleted {
		t.Fatalf("browser trace events = %+v, want completed navigation", events)
	}
	var payload map[string]any
	if err := json.Unmarshal(events[1].Payload, &payload); err != nil {
		t.Fatalf("decode browser completion payload: %v", err)
	}
	if int(payload["snapshot_warning_count"].(float64)) != 2 {
		t.Fatalf("snapshot warning payload = %+v, want count 2", payload)
	}
}

func TestBrowserSessionNavigateUsesHTMLFallbackWhenTextSnapshotEmpty(t *testing.T) {
	t.Parallel()
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
if [ "$mode" = "text" ]; then
  exit 0
fi
if [ "$mode" = "html" ]; then
  printf '<!doctype html><title>Readable HTML Fallback</title><main><h1>Readable HTML Fallback</h1><p>Source text recovered from html. This fallback has enough article body text to prove that the HTML-derived source surface is useful without relying on a declared alternate. It includes a second sentence about citations, source windows, and durable inspection so the extraction quality check does not accept a skeletal page title alone.</p><p>The source reader should preserve this prose as the readable browser snapshot while still keeping the raw HTML artifact for debugging.</p><script>ignored()</script></main>'
  exit 0
fi
if [ "$mode" = "links" ]; then
  printf 'https://example.com/source\tSource link\n'
  exit 0
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

	navigateW := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/browser/sessions/"+created.SessionID+"/navigate", `{"url":"https://example.com/source"}`, "user-alice")
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
	if !strings.Contains(navigated.TextSnapshot, "Source text recovered from html.") {
		t.Fatalf("text_snapshot missing html fallback text: %q", navigated.TextSnapshot)
	}
	if !strings.Contains(navigated.HTMLSnapshot, "<title>Readable HTML Fallback</title>") {
		t.Fatalf("html_snapshot missing raw html: %q", navigated.HTMLSnapshot)
	}
	if len(navigated.Links) != 1 || navigated.Links[0].Text != "Source link" {
		t.Fatalf("links = %+v, want extracted fake link", navigated.Links)
	}
	if len(navigated.SnapshotWarnings) != 1 || !strings.Contains(navigated.SnapshotWarnings[0], "used html readable fallback") {
		t.Fatalf("snapshot_warnings = %+v, want html fallback warning", navigated.SnapshotWarnings)
	}

	events, err := rt.Store().ListEventsByTrajectory(context.Background(), "user-alice", browserSessionTraceID(created.SessionID), 10)
	if err != nil {
		t.Fatalf("list browser trace events: %v", err)
	}
	if len(events) != 2 || events[1].Kind != types.EventBrowserNavigationCompleted {
		t.Fatalf("browser trace events = %+v, want completed navigation", events)
	}
	var payload map[string]any
	if err := json.Unmarshal(events[1].Payload, &payload); err != nil {
		t.Fatalf("decode browser completion payload: %v", err)
	}
	if int(payload["snapshot_warning_count"].(float64)) != 1 {
		t.Fatalf("snapshot warning payload = %+v, want count 1", payload)
	}
}

func TestBrowserSessionNavigateUsesDeclaredMarkdownAlternateWhenHTMLFallbackLowContent(t *testing.T) {
	allowPrivateSourceFetchForTest(t)
	rt, handler := testAPISetup(t)
	markdown := strings.Repeat("Similarity search article text recovered from the declared Markdown alternate. ", 10)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/docs/index.md" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		_, _ = fmt.Fprintf(w, "# Search\n# Similarity search\n\n%s\n", markdown)
	}))
	t.Cleanup(server.Close)

	dir := t.TempDir()
	bin := filepath.Join(dir, "obscura")
	htmlShell := fmt.Sprintf(`<!doctype html><html><head><title>%s/docs/</title><link rel="canonical" href="%s/docs/"><link rel="alternate" type="text/markdown" href="index.md"></head><body></body></html>`, server.URL, server.URL)
	if err := os.WriteFile(bin, []byte(fmt.Sprintf(`#!/bin/sh
mode=text
while [ "$#" -gt 0 ]; do
  if [ "$1" = "--dump" ]; then
    mode="$2"
    shift 2
  else
    shift
  fi
done
if [ "$mode" = "text" ]; then
  exit 0
fi
if [ "$mode" = "html" ]; then
  printf %%s %q
  exit 0
fi
if [ "$mode" = "links" ]; then
  exit 0
fi
`, htmlShell)), 0o755); err != nil {
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

	navigateW := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/browser/sessions/"+created.SessionID+"/navigate", `{"url":"https://example.com/source"}`, "user-alice")
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
	if !strings.Contains(navigated.TextSnapshot, "Similarity search article text recovered") {
		t.Fatalf("text_snapshot missing markdown alternate text: %q", navigated.TextSnapshot)
	}
	if !strings.Contains(navigated.HTMLSnapshot, `rel="alternate"`) {
		t.Fatalf("html_snapshot missing original html shell: %q", navigated.HTMLSnapshot)
	}
	if len(navigated.SnapshotWarnings) != 1 || !strings.Contains(navigated.SnapshotWarnings[0], "used declared markdown alternate") {
		t.Fatalf("snapshot_warnings = %+v, want declared markdown alternate warning", navigated.SnapshotWarnings)
	}

	events, err := rt.Store().ListEventsByTrajectory(context.Background(), "user-alice", browserSessionTraceID(created.SessionID), 10)
	if err != nil {
		t.Fatalf("list browser trace events: %v", err)
	}
	if len(events) != 2 || events[1].Kind != types.EventBrowserNavigationCompleted {
		t.Fatalf("browser trace events = %+v, want completed navigation", events)
	}
	var payload map[string]any
	if err := json.Unmarshal(events[1].Payload, &payload); err != nil {
		t.Fatalf("decode browser completion payload: %v", err)
	}
	if int(payload["snapshot_warning_count"].(float64)) != 1 {
		t.Fatalf("snapshot warning payload = %+v, want count 1", payload)
	}
}

func TestBrowserSessionNavigateUsesDeclaredMarkdownAlternateFromCanonicalShell(t *testing.T) {
	allowPrivateSourceFetchForTest(t)
	rt, handler := testAPISetup(t)
	markdown := strings.Repeat("Similarity search article text recovered from the canonical page Markdown alternate. ", 10)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/docs/":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = fmt.Fprintf(w, `<!doctype html><html><head><title>Search - Qdrant</title><link rel="alternate" type="text/markdown" href="%s/docs/index.md"></head><body><main><h1>Search</h1></main></body></html>`, serverURLFromRequest(r))
		case "/docs/index.md":
			w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
			_, _ = fmt.Fprintf(w, "# Search\n# Similarity search\n\n%s\n", markdown)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	dir := t.TempDir()
	bin := filepath.Join(dir, "obscura")
	htmlShell := fmt.Sprintf(`<!doctype html><html><head><title>%s/source</title><link rel="canonical" href="%s/docs/"><meta http-equiv="refresh" content="0; url=%s/docs/"></head><body></body></html>`, server.URL, server.URL, server.URL)
	if err := os.WriteFile(bin, []byte(fmt.Sprintf(`#!/bin/sh
mode=text
while [ "$#" -gt 0 ]; do
  if [ "$1" = "--dump" ]; then
    mode="$2"
    shift 2
  else
    shift
  fi
done
if [ "$mode" = "text" ]; then
  exit 0
fi
if [ "$mode" = "html" ]; then
  printf %%s %q
  exit 0
fi
if [ "$mode" = "links" ]; then
  exit 0
fi
`, htmlShell)), 0o755); err != nil {
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

	navigateW := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/browser/sessions/"+created.SessionID+"/navigate", `{"url":"https://example.com/source"}`, "user-alice")
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
	if !strings.Contains(navigated.TextSnapshot, "canonical page Markdown alternate") {
		t.Fatalf("text_snapshot missing canonical markdown alternate text: %q", navigated.TextSnapshot)
	}
	if !strings.Contains(navigated.HTMLSnapshot, `http-equiv="refresh"`) {
		t.Fatalf("html_snapshot missing original redirect shell: %q", navigated.HTMLSnapshot)
	}
	if len(navigated.SnapshotWarnings) != 1 || !strings.Contains(navigated.SnapshotWarnings[0], "used declared markdown alternate") {
		t.Fatalf("snapshot_warnings = %+v, want declared markdown alternate warning", navigated.SnapshotWarnings)
	}
}

func serverURLFromRequest(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

func TestBrowserSessionNavigateFailsWhenTextSnapshotFails(t *testing.T) {
	t.Parallel()
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
if [ "$mode" = "text" ]; then
  printf 'fake text dump failed\n' >&2
  exit 2
fi
printf 'optional artifact should not matter\n'
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

	navigateW := registeredRuntimeRequest(t, handler, http.MethodPost, "/api/browser/sessions/"+created.SessionID+"/navigate", `{"url":"https://example.com/source"}`, "user-alice")
	if navigateW.Code != http.StatusBadGateway {
		t.Fatalf("navigate status = %d, want %d; body: %s", navigateW.Code, http.StatusBadGateway, navigateW.Body.String())
	}
	var navigated types.BrowserSessionRecord
	if err := json.NewDecoder(navigateW.Body).Decode(&navigated); err != nil {
		t.Fatalf("decode navigate: %v", err)
	}
	if navigated.State != types.BrowserSessionError {
		t.Fatalf("state = %q, want %q; session: %+v", navigated.State, types.BrowserSessionError, navigated)
	}
	if !strings.Contains(navigated.Error, "text fetch failed") {
		t.Fatalf("error = %q, want text fetch failure", navigated.Error)
	}
	if navigated.TextSnapshot != "" {
		t.Fatalf("text_snapshot = %q, want empty on text failure", navigated.TextSnapshot)
	}
}

func TestBrowserSessionNavigateFailsClosedWhenBackendUnavailable(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
	_, handler := testAPISetup(t)

	cases := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodPost, "/api/agent/loop", `{"prompt":"old"}`},
		{http.MethodPost, "/api/agent/spawn", `{"requested_by":"p","objective":"old"}`},
		{http.MethodGet, "/api/agent/status?loop_id=old", ""},
		{http.MethodGet, "/api/agent/loops", ""},
		{http.MethodGet, "/api/agent/events", ""},
		{http.MethodGet, "/api/agent/channel-messages?channel_id=doc", ""},
		{http.MethodGet, "/api/agent/topology", ""},
		{http.MethodGet, "/api/events", ""},
		{http.MethodGet, "/api/prompts", ""},
		{http.MethodPost, "/api/test/texture/worker-update", `{"doc_id":"doc","schema_version":"coagent_source_packet.v1","kind":"evidence_update"}`},
		{http.MethodPost, "/internal/runtime/objectgraph/web-captures", `{"items":[]}`},
		{http.MethodPost, "/api/texture/documents/doc-1/agent-revision", `{"intent":"revise"}`},
	}

	for _, tc := range cases {
		w := registeredRuntimeRequest(t, handler, tc.method, tc.path, tc.body, "user-alice")
		if w.Code != http.StatusNotFound {
			t.Fatalf("%s %s: got status %d, want 404; body=%s", tc.method, tc.path, w.Code, w.Body.String())
		}
	}
}

func TestRegisteredPromptBarRouteAcceptsIntentOnly(t *testing.T) {
	t.Parallel()
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

func TestInternalRuntimeRunRoutesRequireInternalCallerAndConstrainProfiles(t *testing.T) {
	t.Parallel()
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

	processorReq := httptest.NewRequest(http.MethodPost, "/internal/runtime/runs", strings.NewReader(`{"owner_id":"user-alice","prompt":"ingest source handoff","metadata":{"agent_profile":"processor","processor_key":"processor:global_firehose:global:gdelt"}}`))
	processorReq.Header.Set("X-Internal-Caller", "true")
	processorW := httptest.NewRecorder()
	handler.HandleInternalRunSubmission(processorW, processorReq)
	if processorW.Code != http.StatusAccepted {
		t.Fatalf("processor internal runtime status = %d, want 202; body=%s", processorW.Code, processorW.Body.String())
	}
	var processorResp runStatusResponse
	if err := json.NewDecoder(processorW.Body).Decode(&processorResp); err != nil {
		t.Fatalf("decode processor internal run response: %v", err)
	}
	if processorResp.AgentProfile != AgentProfileProcessor || processorResp.AgentID != "processor:processor-global_firehose-global-gdelt" {
		t.Fatalf("unexpected processor internal run response: %+v", processorResp)
	}

	reconcilerReq := httptest.NewRequest(http.MethodPost, "/internal/runtime/runs", strings.NewReader(`{"owner_id":"user-alice","prompt":"reconcile corpus","metadata":{"agent_profile":"reconciler","reconciler_scope":"story-corpus"}}`))
	reconcilerReq.Header.Set("X-Internal-Caller", "true")
	reconcilerW := httptest.NewRecorder()
	handler.HandleInternalRunSubmission(reconcilerW, reconcilerReq)
	if reconcilerW.Code != http.StatusAccepted {
		t.Fatalf("reconciler internal runtime status = %d, want 202; body=%s", reconcilerW.Code, reconcilerW.Body.String())
	}
	var reconcilerResp runStatusResponse
	if err := json.NewDecoder(reconcilerW.Body).Decode(&reconcilerResp); err != nil {
		t.Fatalf("decode reconciler internal run response: %v", err)
	}
	if reconcilerResp.AgentProfile != AgentProfileReconciler || reconcilerResp.AgentID != "reconciler:story-corpus" {
		t.Fatalf("unexpected reconciler internal run response: %+v", reconcilerResp)
	}
}

func TestHandleInternalRunSubmissionAdmitsProcessorAfterStoryRouteRequestResolutionCompletes(t *testing.T) {
	rt, handler := testAPISetup(t)
	t.Setenv("RUNTIME_MAX_PROCESSOR_RUNS", "1")

	rec, err := rt.createRunWithMetadata(context.Background(), "route a story to texture", "user-alice", map[string]any{
		runMetadataAgentProfile: AgentProfileProcessor,
		runMetadataAgentRole:    AgentProfileProcessor,
		runMetadataProcessorKey: "processor:global_firehose:global:gdelt",
		"source_item_ids":       []string{"source-item-1"},
		"source_count":          1,
	})
	if err != nil {
		t.Fatalf("create processor run: %v", err)
	}
	if _, err := rt.ensureCoagentTextureRevisionRoute(context.Background(), rec, coagentTextureRouteRequest{
		CallerProfile: AgentProfileProcessor,
		Role:          AgentProfileTexture,
		Profile:       AgentProfileTexture,
		Objective:     "Draft the article.",
		Title:         "Wire Story",
		SourceItemIDs: []string{"source-item-1"},
	}); err != nil {
		t.Fatalf("ensure processor texture route: %v", err)
	}

	requestItem, found, err := rt.Store().FindWorkItemByFingerprint(context.Background(), "user-alice", rec.TrajectoryID, wireProcessorDecisionWorkItemFingerprint(rec.TrajectoryID))
	if err != nil {
		t.Fatalf("find processor request work item: %v", err)
	}
	if !found {
		t.Fatal("processor request work item missing")
	}
	if requestItem.Status != types.WorkItemCompleted || requestItem.Details["resolution_state"] != "all_source_items_decided_with_story_route" {
		t.Fatalf("processor request work item = %+v, want completed story-route resolution", requestItem)
	}

	rec.State = types.RunRunning
	rec.UpdatedAt = time.Now().UTC()
	if err := rt.Store().UpdateRun(context.Background(), *rec); err != nil {
		t.Fatalf("mark processor run running: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/internal/runtime/runs", strings.NewReader(`{"owner_id":"user-alice","prompt":"ingest next source handoff","metadata":{"agent_profile":"processor","processor_key":"processor:global_firehose:global:gdelt"}}`))
	req.Header.Set("X-Internal-Caller", "true")
	w := httptest.NewRecorder()
	handler.HandleInternalRunSubmission(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("processor internal runtime status = %d, want 202; body=%s", w.Code, w.Body.String())
	}
	var resp runStatusResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode processor internal run response: %v", err)
	}
	if resp.AgentProfile != AgentProfileProcessor || resp.AgentID != "processor:processor-global_firehose-global-gdelt" {
		t.Fatalf("unexpected processor internal run response: %+v", resp)
	}
}

func TestHandleInternalRunStatusIncludesTrajectoryObligations(t *testing.T) {
	t.Parallel()
	rt, handler := testAPISetup(t)

	rec, err := rt.StartRunWithMetadata(context.Background(), "ingest source handoff", "user-alice", map[string]any{
		runMetadataAgentProfile:        AgentProfileProcessor,
		runMetadataAgentRole:           AgentProfileProcessor,
		"ingestion_handoff_request_id": "processor-request-status",
		runMetadataProcessorKey:        "processor:global_firehose:global:gdelt",
		"source_item_ids":              []string{"source-item-1"},
		"source_count":                 1,
		"source_network_request_id":    "source-request-status",
	})
	if err != nil {
		t.Fatalf("start processor run: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/internal/runtime/runs/"+rec.RunID+"?owner_id=user-alice", nil)
	req.Header.Set("X-Internal-Caller", "true")
	w := httptest.NewRecorder()
	handler.HandleInternalRunStatus(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("internal run status = %d, want 200; body=%s", w.Code, w.Body.String())
	}

	var resp runStatusResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode internal run status: %v", err)
	}
	if resp.Trajectory == nil {
		t.Fatal("internal run status trajectory is nil")
	}
	if resp.Trajectory.TrajectoryID != rec.TrajectoryID {
		t.Fatalf("trajectory_id = %q, want %q", resp.Trajectory.TrajectoryID, rec.TrajectoryID)
	}
	if resp.Trajectory.Status != types.TrajectoryLive {
		t.Fatalf("trajectory status = %q, want %q", resp.Trajectory.Status, types.TrajectoryLive)
	}
	if resp.Trajectory.SettlementReady {
		t.Fatalf("trajectory unexpectedly settlement-ready: %+v", resp.Trajectory)
	}
	if resp.Trajectory.OpenWorkItemCount == 0 {
		t.Fatalf("trajectory open work item count = 0, want > 0")
	}
	if len(resp.Trajectory.WaitingOn) == 0 {
		t.Fatalf("trajectory waiting_on empty, want obligations")
	}
	if resp.ProcessorResolution == nil {
		t.Fatal("internal run status processor_resolution is nil")
	}
	if resp.ProcessorResolution.Status != types.WorkItemOpen {
		t.Fatalf("processor resolution status = %q, want %q", resp.ProcessorResolution.Status, types.WorkItemOpen)
	}
	if resp.ProcessorResolution.ResolutionState != "awaiting_source_item_decisions" {
		t.Fatalf("processor resolution_state = %q, want awaiting_source_item_decisions", resp.ProcessorResolution.ResolutionState)
	}
	if resp.ProcessorResolution.SourceItemCount != 1 || resp.ProcessorResolution.ResolvedSourceItemCount != 0 {
		t.Fatalf("processor resolution counts = %+v", resp.ProcessorResolution)
	}
}

func TestHandleInternalRunStatusIncludesProcessorResolutionTerminalBranch(t *testing.T) {
	t.Parallel()
	rt, handler := testAPISetup(t)
	coveredByDocID := seedPublishedCoverageDoc(t, rt.Store(), "user-alice", "wire-status-covered")

	rec, err := rt.StartRunWithMetadata(context.Background(), "review this batch", "user-alice", map[string]any{
		runMetadataAgentProfile:        AgentProfileProcessor,
		runMetadataAgentRole:           AgentProfileProcessor,
		"ingestion_handoff_request_id": "processor-request-status-covered",
		runMetadataProcessorKey:        "processor:global_firehose:global:gdelt",
		"source_item_ids":              []string{"source-item-1"},
		"source_count":                 1,
		"source_network_request_id":    "source-request-status-covered",
	})
	if err != nil {
		t.Fatalf("start processor run: %v", err)
	}

	registry := NewToolRegistry()
	if err := RegisterWireProcessorTools(registry, rt); err != nil {
		t.Fatalf("register wire processor tools: %v", err)
	}
	if _, err := registry.Execute(WithToolExecutionContext(context.Background(), rec), "record_wire_processor_decision", json.RawMessage(`{
		"decision":"already_covered",
		"summary":"Published coverage already satisfies this source item.",
		"covered_by_doc_id":"`+coveredByDocID+`"
	}`)); err != nil {
		t.Fatalf("record_wire_processor_decision: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/internal/runtime/runs/"+rec.RunID+"?owner_id=user-alice", nil)
	req.Header.Set("X-Internal-Caller", "true")
	w := httptest.NewRecorder()
	handler.HandleInternalRunStatus(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("internal run status = %d, want 200; body=%s", w.Code, w.Body.String())
	}

	var resp runStatusResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode internal run status: %v", err)
	}
	if resp.Trajectory == nil || resp.Trajectory.Status != types.TrajectoryCancelled {
		t.Fatalf("trajectory status = %+v, want cancelled", resp.Trajectory)
	}
	if resp.ProcessorResolution == nil {
		t.Fatal("internal run status processor_resolution is nil")
	}
	if resp.ProcessorResolution.Status != types.WorkItemCompleted {
		t.Fatalf("processor resolution status = %q, want %q", resp.ProcessorResolution.Status, types.WorkItemCompleted)
	}
	if resp.ProcessorResolution.ResolutionState != "all_source_items_suppressed_against_published_corpus" {
		t.Fatalf("processor resolution_state = %q, want all_source_items_suppressed_against_published_corpus", resp.ProcessorResolution.ResolutionState)
	}
	if resp.ProcessorResolution.LastDecision != "already_covered" {
		t.Fatalf("processor last_decision = %q, want already_covered", resp.ProcessorResolution.LastDecision)
	}
	if resp.ProcessorResolution.CoveredByDocID != coveredByDocID {
		t.Fatalf("processor covered_by_doc_id = %q, want %q", resp.ProcessorResolution.CoveredByDocID, coveredByDocID)
	}
}

func TestHandleInternalRunStatusIncludesExplicitNoStoryTerminalBranch(t *testing.T) {
	t.Parallel()
	rt, handler := testAPISetup(t)

	rec, err := rt.StartRunWithMetadata(context.Background(), "review this batch", "user-alice", map[string]any{
		runMetadataAgentProfile:        AgentProfileProcessor,
		runMetadataAgentRole:           AgentProfileProcessor,
		"ingestion_handoff_request_id": "processor-request-status-no-story",
		runMetadataProcessorKey:        "processor:global_firehose:global:gdelt",
		"source_item_ids":              []string{"source-item-1"},
		"source_count":                 1,
		"source_network_request_id":    "source-request-status-no-story",
	})
	if err != nil {
		t.Fatalf("start processor run: %v", err)
	}

	registry := NewToolRegistry()
	if err := RegisterWireProcessorTools(registry, rt); err != nil {
		t.Fatalf("register wire processor tools: %v", err)
	}
	if _, err := registry.Execute(WithToolExecutionContext(context.Background(), rec), "record_wire_processor_decision", json.RawMessage(`{
		"decision":"not_newsworthy",
		"summary":"The batch does not justify a publication route."
	}`)); err != nil {
		t.Fatalf("record_wire_processor_decision: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/internal/runtime/runs/"+rec.RunID+"?owner_id=user-alice", nil)
	req.Header.Set("X-Internal-Caller", "true")
	w := httptest.NewRecorder()
	handler.HandleInternalRunStatus(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("internal run status = %d, want 200; body=%s", w.Code, w.Body.String())
	}

	var resp runStatusResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode internal run status: %v", err)
	}
	if resp.Trajectory == nil || resp.Trajectory.Status != types.TrajectoryCancelled {
		t.Fatalf("trajectory status = %+v, want cancelled", resp.Trajectory)
	}
	if resp.ProcessorResolution == nil {
		t.Fatal("internal run status processor_resolution is nil")
	}
	if resp.ProcessorResolution.Status != types.WorkItemCompleted {
		t.Fatalf("processor resolution status = %q, want %q", resp.ProcessorResolution.Status, types.WorkItemCompleted)
	}
	if resp.ProcessorResolution.ResolutionState != "all_source_items_decided_without_story_route" {
		t.Fatalf("processor resolution_state = %q, want all_source_items_decided_without_story_route", resp.ProcessorResolution.ResolutionState)
	}
	if resp.ProcessorResolution.LastDecision != "not_newsworthy" {
		t.Fatalf("processor last_decision = %q, want not_newsworthy", resp.ProcessorResolution.LastDecision)
	}
	if resp.ProcessorResolution.CoveredByDocID != "" {
		t.Fatalf("processor covered_by_doc_id = %q, want empty", resp.ProcessorResolution.CoveredByDocID)
	}
}

func TestHandleRunListOwnerScoped(t *testing.T) {
	t.Parallel()
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

// --- Health Tests ---

func TestHandleHealthReady(t *testing.T) {
	t.Parallel()
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
	if resp.PersistentDisk == nil {
		t.Fatal("expected persistent_disk in health response")
	}
	if resp.PersistentDisk.Source != "guest" {
		t.Fatalf("persistent_disk.source = %q, want guest", resp.PersistentDisk.Source)
	}
	if resp.PersistentDisk.DefaultCapBytes != 8*1024*1024*1024 {
		t.Fatalf("default_cap_bytes = %d, want 8GiB", resp.PersistentDisk.DefaultCapBytes)
	}
}

func TestHandleHealthDegraded(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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

func TestHandleTextureDocumentsRootUsesTextureRoutes(t *testing.T) {
	t.Parallel()
	_, handler := testAPISetup(t)

	createReqBody := `{"title":"texture route doc","content":"hello"}`
	createReq := authenticatedRequest(http.MethodPost, "/api/texture/documents", createReqBody, "user-alice")
	createW := httptest.NewRecorder()
	handler.HandleTextureDocumentsRoot(createW, createReq)

	if createW.Code != http.StatusCreated {
		t.Fatalf("create status: got %d, want %d", createW.Code, http.StatusCreated)
	}

	var createResp textureCreateDocResponse
	if err := json.NewDecoder(createW.Body).Decode(&createResp); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if createResp.DocID == "" {
		t.Fatal("doc_id should not be empty")
	}

	listReq := authenticatedRequest(http.MethodGet, "/api/texture/documents", "", "user-alice")
	listW := httptest.NewRecorder()
	handler.HandleTextureDocumentsRoot(listW, listReq)

	if listW.Code != http.StatusOK {
		t.Fatalf("list status: got %d, want %d", listW.Code, http.StatusOK)
	}

	var listResp textureListDocsResponse
	if err := json.NewDecoder(listW.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(listResp.Documents) != 1 {
		t.Fatalf("documents: got %d, want 1", len(listResp.Documents))
	}
	if listResp.Documents[0].Title != "texture route doc" {
		t.Errorf("title: got %q, want %q", listResp.Documents[0].Title, "texture route doc")
	}

	getReq := authenticatedRequest(http.MethodGet, "/api/texture/documents/"+url.PathEscape(createResp.DocID), "", "user-alice")
	getW := httptest.NewRecorder()
	handler.HandleTextureRouter(getW, getReq)

	if getW.Code != http.StatusOK {
		t.Fatalf("get status: got %d, want %d; body=%s", getW.Code, http.StatusOK, getW.Body.String())
	}
	var getResp textureDocumentResponse
	if err := json.NewDecoder(getW.Body).Decode(&getResp); err != nil {
		t.Fatalf("decode get response: %v", err)
	}
	if getResp.DocID != createResp.DocID {
		t.Errorf("get doc_id: got %q, want %q", getResp.DocID, createResp.DocID)
	}
}

// --- Supervisor Recovery Visibility Tests ---

func TestSupervisorRecoveryVisible(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
	dbPath := filepath.Join(t.TempDir(), "runtime.db")
	s, err := openTestStore(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	failProvider := &StubProvider{
		Delay:   10 * time.Millisecond,
		FailErr: errors.New("provider connection refused"),
	}
	rt := New(Config{
		SandboxID:           "sandbox-test",
		StorePath:           dbPath,
		ProviderTimeout:     time.Second,
		SupervisionInterval: time.Hour,
	}, s, events.NewEventBus(), failProvider)
	setTestDispatch(rt, s)
	t.Cleanup(func() {
		rt.Stop()
		_ = s.Close()
	})

	failing, err := rt.StartRun(context.Background(), "will fail", "user-alice")
	if err != nil {
		t.Fatalf("start failing run: %v", err)
	}
	failed := waitForRunTerminalState(t, rt, failing.RunID, "user-alice", 5*time.Second)
	if failed.State != types.RunFailed {
		t.Fatalf("failed run state = %q, want %q", failed.State, types.RunFailed)
	}

	rt.provider = NewStubProvider(50 * time.Millisecond)
	next, err := rt.StartRun(context.Background(), "after failure", "user-alice")
	if err != nil {
		t.Fatalf("runtime rejected run after provider failure: %v", err)
	}
	completed := waitForRunTerminalState(t, rt, next.RunID, "user-alice", 5*time.Second)
	if completed.State != types.RunCompleted {
		t.Fatalf("post-failure run state = %q, want %q", completed.State, types.RunCompleted)
	}
}

func intFromMetadata(metadata map[string]any, key string) int {
	switch value := metadata[key].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	default:
		return 0
	}
}

// --- AuthenticateUser Tests ---

func TestAuthenticateUserMissing(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "/api/agent/status", nil)
	_, err := authenticateUser(req)
	if err == nil {
		t.Error("expected error for missing auth header")
	}
}

func TestAuthenticateUserPresent(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
	// When a bridge provider is active, the health endpoint should report
	// its name (e.g., "bedrock" or "zai") instead of "stub", so operators
	// can distinguish real-provider paths from canned responses.

	dir := filepath.Join(os.TempDir(), "go-choir-m3-api-test")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	dbPath := filepath.Join(dir, t.Name()+".db")
	_ = os.Remove(dbPath)

	s, err := openTestStore(dbPath)
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
	setTestDispatch(rt, s)
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
