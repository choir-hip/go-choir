package runtime

import (
	"context"
	"encoding/json"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestVerifyVTextWorkflowDeterministicEventLog(t *testing.T) {
	provider := newVTextEditToolProvider(vtextReplaceAllResult("Moss habitats working document.\n\nInitial useful draft."))
	h, _, rt := vtextAPISetupWithProvider(t, provider, true)
	ctx := context.Background()
	ownerID := "user-1"
	artifactRelPath := filepath.ToSlash(filepath.Join("artifacts", "vtext-workflow-verifier", "moss-habitat.txt"))
	t.Cleanup(func() { _ = os.RemoveAll(filepath.Dir(artifactRelPath)) })

	promptText := "Write a testable note about moss habitat conditions."
	req := authenticatedRequest(http.MethodPost, "/api/prompt-bar", `{"text":"`+promptText+`"}`, ownerID)
	w := httptest.NewRecorder()
	h.HandlePromptBar(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("prompt-bar status = %d, want %d; body=%s", w.Code, http.StatusAccepted, w.Body.String())
	}
	var submission promptBarSubmitResponse
	if err := json.NewDecoder(w.Body).Decode(&submission); err != nil {
		t.Fatalf("decode prompt-bar response: %v", err)
	}
	if state := waitForTaskCompletion(t, h, submission.SubmissionID, 5*time.Second); state != types.RunCompleted {
		t.Fatalf("conductor state = %q, want completed", state)
	}
	conductor, err := rt.GetRun(ctx, submission.SubmissionID, ownerID)
	if err != nil {
		t.Fatalf("get conductor: %v", err)
	}
	var decision conductorDecision
	if err := json.Unmarshal([]byte(conductor.Result), &decision); err != nil {
		t.Fatalf("decode conductor decision: %v\n%s", err, conductor.Result)
	}
	if decision.DocID == "" || decision.InitialLoopID == "" {
		t.Fatalf("conductor did not open vtext: %+v", decision)
	}
	if state := waitForTaskCompletion(t, h, decision.InitialLoopID, 5*time.Second); state != types.RunCompleted {
		t.Fatalf("initial vtext state = %q, want completed", state)
	}
	initialVTextRun, err := rt.GetRun(ctx, decision.InitialLoopID, ownerID)
	if err != nil {
		t.Fatalf("get initial vtext run: %v", err)
	}

	vtextRegistry := rt.ToolRegistryForProfile(AgentProfileVText)
	researcherResults := executeVerifierTools(t, rt, initialVTextRun, vtextRegistry, []types.ToolCall{{
		ID:   "spawn-researcher",
		Name: "spawn_agent",
		Arguments: json.RawMessage(`{
			"objective":"Research moss habitat conditions",
			"role":"researcher",
			"channel_id":"` + decision.DocID + `"
		}`),
	}})
	var researcherResp struct {
		RunID string `json:"loop_id"`
	}
	if err := json.Unmarshal([]byte(researcherResults[0].Output), &researcherResp); err != nil {
		t.Fatalf("decode researcher spawn: %v\n%s", err, researcherResults[0].Output)
	}
	researcherRun, err := rt.GetRun(ctx, researcherResp.RunID, ownerID)
	if err != nil {
		t.Fatalf("get researcher: %v", err)
	}
	researcherRegistry := rt.ToolRegistryForProfile(AgentProfileResearcher)
	executeVerifierTools(t, rt, researcherRun, researcherRegistry, []types.ToolCall{
		{
			ID:   "research-findings",
			Name: "submit_research_findings",
			Arguments: json.RawMessage(`{
				"finding_id":"moss-finding-1",
				"findings":["Moss prefers damp shade and steady humidity."],
				"evidence":[{"kind":"web_page","source_uri":"https://example.test/moss","title":"Moss habitat","content":"Moss prefers damp shade and steady humidity."}],
				"notes":["Use this as a scoped habitat claim."]
			}`),
		},
	})

	superResults := executeVerifierTools(t, rt, initialVTextRun, vtextRegistry, []types.ToolCall{
		{
			ID:   "request-super",
			Name: "request_super_execution",
			Arguments: json.RawMessage(`{
				"objective":"Create and verify a moss habitat artifact, then report structured results.",
				"channel_id":"` + decision.DocID + `"
			}`),
		},
	})
	var superResp struct {
		RunID string `json:"loop_id"`
	}
	if err := json.Unmarshal([]byte(superResults[0].Output), &superResp); err != nil {
		t.Fatalf("decode request_super_execution result: %v\n%s", err, superResults[0].Output)
	}
	superRun, err := rt.GetRun(ctx, superResp.RunID, ownerID)
	if err != nil {
		t.Fatalf("get super run: %v", err)
	}
	superRegistry := rt.ToolRegistryForProfile(AgentProfileSuper)
	executeVerifierTools(t, rt, superRun, superRegistry, []types.ToolCall{
		{
			ID:        "write-artifact",
			Name:      "write_file",
			Arguments: json.RawMessage(`{"path":"` + artifactRelPath + `","content":"moss habitat artifact verified"}`),
		},
	})
	executeVerifierTools(t, rt, superRun, superRegistry, []types.ToolCall{
		{
			ID:        "verify-artifact",
			Name:      "bash",
			Arguments: json.RawMessage(`{"command":"test -f ` + artifactRelPath + ` && grep -q verified ` + artifactRelPath + `"}`),
		},
	})
	coSuperResults := executeVerifierTools(t, rt, superRun, superRegistry, []types.ToolCall{
		{
			ID:        "spawn-co-super",
			Name:      "spawn_agent",
			Arguments: json.RawMessage(`{"objective":"Check the moss artifact details","role":"co-super"}`),
		},
	})
	var coSuperResp struct {
		RunID   string `json:"loop_id"`
		Profile string `json:"profile"`
	}
	if err := json.Unmarshal([]byte(coSuperResults[0].Output), &coSuperResp); err != nil {
		t.Fatalf("decode co-super spawn result: %v\n%s", err, coSuperResults[0].Output)
	}
	if coSuperResp.Profile != AgentProfileCoSuper || coSuperResp.RunID == "" {
		t.Fatalf("unexpected co-super spawn: %+v", coSuperResp)
	}
	updateResults := executeVerifierTools(t, rt, superRun, superRegistry, []types.ToolCall{
		{
			ID:   "worker-update",
			Name: "submit_worker_update",
			Arguments: json.RawMessage(`{
					"update_id":"moss-worker-update-1",
					"agent_id":"vtext:` + decision.DocID + `",
					"channel_id":"` + decision.DocID + `",
					"artifacts":["` + artifactRelPath + `"],
					"tests":["test -f ` + artifactRelPath + ` && grep -q verified ` + artifactRelPath + `"],
					"proposals":["Include the verified artifact path in the current document."]
				}`),
		},
	})
	var updateResp struct {
		Cursor int64 `json:"cursor"`
	}
	if err := json.Unmarshal([]byte(updateResults[0].Output), &updateResp); err != nil {
		t.Fatalf("decode worker update result: %v\n%s", err, updateResults[0].Output)
	}
	waitForVerifierConsumedWorkerSeq(t, rt, ownerID, decision.DocID, updateResp.Cursor, 5*time.Second)

	report, err := rt.VerifyVTextWorkflow(ctx, VTextWorkflowVerificationOptions{
		OwnerID:                     ownerID,
		TrajectoryID:                submission.SubmissionID,
		PromptSubmissionID:          submission.SubmissionID,
		RequireResearchFindings:     true,
		RequireWorkerUpdates:        true,
		RequirePersistentSuper:      true,
		RequireCoSuper:              true,
		RequireArtifactWriteEvent:   true,
		RequireVerificationCmdEvent: true,
		RequireWorkerConsumption:    true,
		RequireToolBackedWorkerRuns: true,
	})
	if err != nil {
		t.Fatalf("verify vtext workflow: %v", err)
	}
	if len(report.Guarantees) < 10 {
		t.Fatalf("verification guarantees too small: %+v", report.Guarantees)
	}
}

func TestVerifyVTextWorkflowSeededStochasticOrdering(t *testing.T) {
	const ownerID = "user-1"
	rng := rand.New(rand.NewSource(20260501))
	provider := newVTextEditToolProvider(vtextReplaceAllResult("Stochastic ordering document.\n\nResearch and worker updates integrated."))
	h, s, rt := vtextAPISetupWithProvider(t, provider, true)
	ctx := context.Background()

	req := authenticatedRequest(http.MethodPost, "/api/prompt-bar", `{"text":"Build a stochastic ordering note."}`, ownerID)
	w := httptest.NewRecorder()
	h.HandlePromptBar(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("prompt-bar status = %d, want %d; body=%s", w.Code, http.StatusAccepted, w.Body.String())
	}
	var submission promptBarSubmitResponse
	if err := json.NewDecoder(w.Body).Decode(&submission); err != nil {
		t.Fatalf("decode prompt-bar response: %v", err)
	}
	if state := waitForTaskCompletion(t, h, submission.SubmissionID, 5*time.Second); state != types.RunCompleted {
		t.Fatalf("conductor state = %q, want completed", state)
	}
	conductor, err := rt.GetRun(ctx, submission.SubmissionID, ownerID)
	if err != nil {
		t.Fatalf("get conductor: %v", err)
	}
	var decision conductorDecision
	if err := json.Unmarshal([]byte(conductor.Result), &decision); err != nil {
		t.Fatalf("decode conductor decision: %v", err)
	}
	if state := waitForTaskCompletion(t, h, decision.InitialLoopID, 5*time.Second); state != types.RunCompleted {
		t.Fatalf("initial vtext state = %q, want completed", state)
	}
	initialVTextRun, err := rt.GetRun(ctx, decision.InitialLoopID, ownerID)
	if err != nil {
		t.Fatalf("get initial vtext run: %v", err)
	}

	vtextRegistry := rt.ToolRegistryForProfile(AgentProfileVText)
	researcherResults := executeVerifierTools(t, rt, initialVTextRun, vtextRegistry, []types.ToolCall{{
		ID:   "spawn-stochastic-researcher",
		Name: "spawn_agent",
		Arguments: json.RawMessage(`{
			"objective":"Research ordering evidence",
			"role":"researcher",
			"channel_id":"` + decision.DocID + `"
		}`),
	}})
	var researcherResp struct {
		RunID string `json:"loop_id"`
	}
	if err := json.Unmarshal([]byte(researcherResults[0].Output), &researcherResp); err != nil {
		t.Fatalf("decode stochastic researcher spawn: %v\n%s", err, researcherResults[0].Output)
	}
	researcherRun, err := rt.GetRun(ctx, researcherResp.RunID, ownerID)
	if err != nil {
		t.Fatalf("get stochastic researcher: %v", err)
	}
	superResults := executeVerifierTools(t, rt, initialVTextRun, vtextRegistry, []types.ToolCall{{
		ID:        "request-super-stochastic",
		Name:      "request_super_execution",
		Arguments: json.RawMessage(`{"objective":"Report a structured stochastic worker update.","channel_id":"` + decision.DocID + `"}`),
	}})
	var superResp struct {
		RunID string `json:"loop_id"`
	}
	if err := json.Unmarshal([]byte(superResults[0].Output), &superResp); err != nil {
		t.Fatalf("decode stochastic super request: %v", err)
	}
	superRun, err := rt.GetRun(ctx, superResp.RunID, ownerID)
	if err != nil {
		t.Fatalf("get stochastic super run: %v", err)
	}

	type action struct {
		at time.Duration
		fn func() int64
	}
	var workerSeq int64
	actions := []action{
		{at: time.Duration(5+rng.Intn(20)) * time.Millisecond, fn: func() int64 {
			executeVerifierTools(t, rt, researcherRun, rt.ToolRegistryForProfile(AgentProfileResearcher), []types.ToolCall{{
				ID:   "stochastic-research",
				Name: "submit_research_findings",
				Arguments: json.RawMessage(`{
					"finding_id":"stochastic-finding-1",
					"findings":["The stochastic order still preserves durable causality."],
					"evidence":[{"kind":"note","content":"seeded stochastic evidence"}]
				}`),
			}})
			return 0
		}},
		{at: time.Duration(5+rng.Intn(20)) * time.Millisecond, fn: func() int64 {
			results := executeVerifierTools(t, rt, superRun, rt.ToolRegistryForProfile(AgentProfileSuper), []types.ToolCall{{
				ID:   "stochastic-worker-update",
				Name: "submit_worker_update",
				Arguments: json.RawMessage(`{
					"update_id":"stochastic-worker-update-1",
					"agent_id":"vtext:` + decision.DocID + `",
					"channel_id":"` + decision.DocID + `",
					"tests":["seeded stochastic verification passed"],
					"proposals":["Preserve the latest stochastic worker update."]
				}`),
			}})
			var resp struct {
				Cursor int64 `json:"cursor"`
			}
			if err := json.Unmarshal([]byte(results[0].Output), &resp); err != nil {
				t.Fatalf("decode stochastic worker update: %v", err)
			}
			return resp.Cursor
		}},
		{at: time.Duration(5+rng.Intn(20)) * time.Millisecond, fn: func() int64 {
			createUserRevisionFromCurrentHead(t, h, s, decision.DocID, ownerID, "USER_STOCHASTIC_EDIT")
			return 0
		}},
	}
	for i := range actions {
		for j := i + 1; j < len(actions); j++ {
			if actions[j].at < actions[i].at {
				actions[i], actions[j] = actions[j], actions[i]
			}
		}
	}
	start := time.Now()
	for _, action := range actions {
		if sleep := action.at - time.Since(start); sleep > 0 {
			time.Sleep(sleep)
		}
		if seq := action.fn(); seq > 0 {
			workerSeq = seq
		}
	}
	if workerSeq == 0 {
		t.Fatal("stochastic worker update did not return a message seq")
	}
	waitForVerifierConsumedWorkerSeq(t, rt, ownerID, decision.DocID, workerSeq, 5*time.Second)

	if _, err := rt.VerifyVTextWorkflow(ctx, VTextWorkflowVerificationOptions{
		OwnerID:                     ownerID,
		TrajectoryID:                submission.SubmissionID,
		PromptSubmissionID:          submission.SubmissionID,
		RequireResearchFindings:     true,
		RequireWorkerUpdates:        true,
		RequirePersistentSuper:      true,
		RequireWorkerConsumption:    true,
		RequireToolBackedWorkerRuns: true,
	}); err != nil {
		t.Fatalf("verify stochastic vtext workflow: %v", err)
	}
}

func executeVerifierTools(t *testing.T, rt *Runtime, run *types.RunRecord, registry *ToolRegistry, calls []types.ToolCall) []types.ToolResult {
	t.Helper()
	if registry == nil {
		t.Fatal("tool registry is nil")
	}
	toolCtx := WithToolExecutionContext(context.Background(), run)
	emit := func(kind types.EventKind, phase string, payload json.RawMessage) {
		rt.emitEvent(context.Background(), run, kind, events.CauseToolExecution, payload)
	}
	results := executeTools(toolCtx, registry, calls, emit)
	for _, result := range results {
		if result.IsError {
			t.Fatalf("tool %s failed: %s", result.CallID, result.Output)
		}
	}
	return results
}

func waitForVerifierConsumedWorkerSeq(t *testing.T, rt *Runtime, ownerID, docID string, seq int64, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		revisions, err := rt.store.ListRevisionsByDoc(context.Background(), docID, ownerID, 50)
		if err == nil {
			for _, revision := range revisions {
				meta := decodeRevisionMetadata(revision.Metadata)
				for _, consumed := range consumedWorkerSeqs(meta) {
					if consumed == seq {
						loopID := metadataString(meta, "loop_id")
						if loopID == "" {
							return
						}
						eventsForRun, err := rt.store.ListEvents(context.Background(), loopID, 200)
						if err == nil && len(successfulToolResultPayloadsForRun(eventsForRun, loopID, "edit_vtext")) > 0 {
							return
						}
					}
				}
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	revisions, _ := rt.store.ListRevisionsByDoc(context.Background(), docID, ownerID, 50)
	t.Fatalf("timed out waiting for worker seq %d to be consumed; revisions=%+v", seq, revisions)
}
