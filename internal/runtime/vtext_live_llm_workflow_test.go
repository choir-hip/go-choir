//go:build integration || comprehensive

package runtime_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/provider"
	choirruntime "github.com/yusefmosiah/go-choir/internal/runtime"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

const liveLLMOwnerID = "user-live-llm"

func TestLiveLLMWorkflowWithFakeSearchGatewayResearchSuperVText(t *testing.T) {
	if os.Getenv("GO_CHOIR_LIVE_LLM_FAKE_SEARCH") != "1" {
		t.Skip("set GO_CHOIR_LIVE_LLM_FAKE_SEARCH=1 to run this live-LLM/fake-search dry-run")
	}
	if os.Getenv("GO_CHOIR_LIVE_LLM") != "1" {
		t.Skip("set GO_CHOIR_LIVE_LLM=1 with GO_CHOIR_LIVE_LLM_FAKE_SEARCH=1 to run this dry-run")
	}

	searchServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/provider/v1/search" {
			http.NotFound(w, r)
			return
		}
		if got := r.Header.Get("Authorization"); got != "Bearer live-llm-test-token" {
			http.Error(w, "bad auth", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"query":    "cellular automata biological evolution toy model",
			"provider": "live-test-search",
			"results": []map[string]any{
				{
					"title":   "Cellular automata and biological toy models",
					"url":     "https://example.test/cellular-automata-evolution",
					"snippet": "Toy cellular automata can model local inheritance, mutation, selection pressure, and population change when claims are scoped as illustrative rather than predictive biology.",
				},
			},
		})
	}))
	defer searchServer.Close()
	t.Setenv("RUNTIME_GATEWAY_URL", searchServer.URL)
	t.Setenv("RUNTIME_GATEWAY_TOKEN", "live-llm-test-token")

	model := strings.TrimSpace(os.Getenv("GO_CHOIR_LIVE_LLM_MODEL"))
	if model == "" {
		model = "gpt-5.4-mini"
	}
	reasoning := strings.TrimSpace(os.Getenv("GO_CHOIR_LIVE_LLM_REASONING_EFFORT"))
	if reasoning == "" {
		reasoning = "low"
	}
	chatgpt, err := provider.NewChatGPTProviderFromEnv(model, reasoning)
	if err != nil {
		t.Fatalf("create live ChatGPT provider: %v", err)
	}

	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "runtime.db"))
	if err != nil {
		t.Fatalf("open runtime store: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	rt := choirruntime.New(choirruntime.Config{
		SandboxID:         "sandbox-live-llm",
		StorePath:         filepath.Join(dir, "runtime.db"),
		PromptRoot:        filepath.Join(dir, "prompts"),
		ProviderTimeout:   180 * time.Second,
		VTextWakeDebounce: 500 * time.Millisecond,
	}, db, events.NewEventBus(), provider.NewBridgeProvider(chatgpt))
	if err := rt.InstallDefaultAgentTools(dir); err != nil {
		t.Fatalf("install agent tools: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	rt.Start(ctx)
	t.Cleanup(func() { rt.Stop() })
	h := choirruntime.NewAPIHandler(rt)

	conductorResp := postLiveJSON(t, h.HandlePromptBar, http.MethodPost, "/api/prompt-bar", map[string]any{
		"text": "Research cellular automata as a toy model for biological evolution, then produce a concise working document that can later mention an artifact and verification result.",
	})
	conductorID := stringField(t, conductorResp, "submission_id")
	conductorRun := waitLiveRun(t, rt, conductorID, liveLLMOwnerID, 180*time.Second)
	if conductorRun.State != types.RunCompleted {
		t.Fatalf("conductor state = %q error=%q", conductorRun.State, conductorRun.Error)
	}
	var decision struct {
		DocID             string `json:"doc_id"`
		UserRevisionID    string `json:"user_revision_id"`
		FramingRevisionID string `json:"framing_revision_id"`
		InitialLoopID     string `json:"initial_loop_id"`
	}
	if err := json.Unmarshal([]byte(conductorRun.Result), &decision); err != nil {
		t.Fatalf("decode conductor decision: %v\n%s", err, conductorRun.Result)
	}
	if decision.DocID == "" || decision.UserRevisionID == "" || decision.FramingRevisionID == "" {
		t.Fatalf("conductor did not materialize v0/v1: %+v result=%s", decision, conductorRun.Result)
	}
	if decision.InitialLoopID == "" {
		t.Fatalf("initial_loop_id is empty; conductor should start the product-path vtext run: %+v", decision)
	}

	revisions, err := db.ListRevisionsByDoc(context.Background(), decision.DocID, liveLLMOwnerID, 20)
	if err != nil {
		t.Fatalf("list conductor revisions: %v", err)
	}
	if len(revisions) < 2 {
		t.Fatalf("conductor revision count = %d, want at least v0/v1", len(revisions))
	}

	initialVTextID := decision.InitialLoopID
	initialVTextRun := waitLiveRun(t, rt, initialVTextID, liveLLMOwnerID, 180*time.Second)
	if initialVTextRun.State != types.RunCompleted {
		t.Fatalf("initial vtext state = %q error=%q", initialVTextRun.State, initialVTextRun.Error)
	}

	researchRun, err := rt.StartChildRun(context.Background(), initialVTextID,
		"Live verification: call web_search for cellular automata biological evolution toy model, then call submit_coagent_update with update_id live-research-ca and one concise evidence-backed checkpoint for the parent vtext agent.",
		liveLLMOwnerID,
		map[string]any{
			"agent_profile": "researcher",
			"agent_role":    "researcher",
			"channel_id":    decision.DocID,
			"model":         model,
		})
	if err != nil {
		t.Fatalf("start live researcher: %v", err)
	}
	researchDone := waitLiveRun(t, rt, researchRun.RunID, liveLLMOwnerID, 180*time.Second)
	if researchDone.State != types.RunCompleted {
		t.Fatalf("researcher state = %q error=%q result=%q", researchDone.State, researchDone.Error, researchDone.Result)
	}
	findings, err := db.ListResearchFindingsByTrajectory(context.Background(), liveLLMOwnerID, conductorID, 20)
	if err != nil {
		t.Fatalf("list live findings: %v", err)
	}
	var researchFinding *types.ResearchFindingRecord
	for i := range findings {
		if findings[i].FindingID == "live-research-ca" {
			researchFinding = &findings[i]
			break
		}
	}
	if researchFinding == nil {
		t.Fatalf("live researcher completed without the expected durable finding; findings=%+v result=%q", findings, researchDone.Result)
	}
	if researchFinding.ChannelID != decision.DocID || researchFinding.TargetAgentID != "vtext:"+decision.DocID || researchFinding.MessageSeq == 0 {
		t.Fatalf("live researcher finding routed to target=%q channel=%q seq=%d, want vtext:%s/%s with message cursor; finding=%+v result=%q", researchFinding.TargetAgentID, researchFinding.ChannelID, researchFinding.MessageSeq, decision.DocID, decision.DocID, researchFinding, researchDone.Result)
	}

	superUpdateID := "live-super-ca-artifact"
	vtextRegistry := rt.ToolRegistryForProfile(choirruntime.AgentProfileVText)
	superRequestRaw, err := vtextRegistry.Execute(choirruntime.WithToolExecutionContext(context.Background(), initialVTextRun), "request_super_execution", json.RawMessage(fmt.Sprintf(`{
		"objective":%q,
		"channel_id":%q,
		"model":%q
	}`, "Live verification: use write_file to create artifacts/live-evolution-ca.txt containing 'live deterministic CA artifact verified'. Then run bash to verify that the artifact exists and contains verified. Then call submit_coagent_update with update_id "+superUpdateID+", artifacts ['artifacts/live-evolution-ca.txt'], tests ['test -f artifacts/live-evolution-ca.txt && grep -q verified artifacts/live-evolution-ca.txt'], and one proposal for the parent vtext agent. Do not finish until submit_coagent_update returns.", decision.DocID, model)))
	if err != nil {
		t.Fatalf("request live super execution: %v", err)
	}
	var superRequest struct {
		RunID string `json:"loop_id"`
	}
	if err := json.Unmarshal([]byte(superRequestRaw), &superRequest); err != nil {
		t.Fatalf("decode live super request: %v\n%s", err, superRequestRaw)
	}
	superDone := waitLiveRun(t, rt, superRequest.RunID, liveLLMOwnerID, 180*time.Second)
	if superDone.State != types.RunCompleted {
		t.Fatalf("super state = %q error=%q result=%q", superDone.State, superDone.Error, superDone.Result)
	}
	updates, err := db.ListWorkerUpdatesByTrajectory(context.Background(), liveLLMOwnerID, conductorID, 20)
	if err != nil {
		t.Fatalf("list live worker updates: %v", err)
	}
	var superUpdate *types.WorkerUpdateRecord
	for i := range updates {
		if updates[i].UpdateID == superUpdateID || updates[i].Role == "super" {
			superUpdate = &updates[i]
			break
		}
	}
	if superUpdate == nil {
		t.Fatalf("live super completed without durable worker update; updates=%+v result=%q", updates, superDone.Result)
	}
	if superUpdate.ChannelID != decision.DocID || superUpdate.MessageSeq == 0 {
		t.Fatalf("live super update routed to channel=%q seq=%d, want channel=%q with message cursor; update=%+v result=%q", superUpdate.ChannelID, superUpdate.MessageSeq, decision.DocID, superUpdate, superDone.Result)
	}
	if superUpdate.TargetAgentID != "vtext:"+decision.DocID {
		t.Fatalf("live super update targeted %q, want vtext:%s; update=%+v result=%q", superUpdate.TargetAgentID, decision.DocID, superUpdate, superDone.Result)
	}
	if len(superUpdate.Artifacts) == 0 || len(superUpdate.Tests) == 0 {
		t.Fatalf("live super update missing artifact/test fields: %+v", superUpdate)
	}
	if _, err := os.Stat(filepath.Join(dir, "artifacts", "live-evolution-ca.txt")); err != nil {
		t.Fatalf("live super did not create artifact file: %v", err)
	}

	final := waitLiveConsumedSeqs(t, db, decision.DocID, conductorID, []int64{researchFinding.MessageSeq, superUpdate.MessageSeq}, 180*time.Second)
	if final.revision.RevisionID == "" {
		t.Fatal("missing final vtext revision")
	}
	if strings.TrimSpace(final.revision.Content) == "" {
		t.Fatalf("final vtext revision is empty: %+v", final.revision)
	}
	finalContent := final.revision.Content
	for _, want := range []string{"cellular automata", "artifacts/live-evolution-ca.txt"} {
		if !strings.Contains(strings.ToLower(finalContent), strings.ToLower(want)) {
			t.Fatalf("final vtext revision missing %q: %s", want, finalContent)
		}
	}
	if !strings.Contains(strings.ToLower(finalContent), "verification") || !strings.Contains(strings.ToLower(finalContent), "verified") {
		t.Fatalf("final vtext revision missing verification result: %s", finalContent)
	}
	if strings.Contains(finalContent, "Task completed successfully") ||
		strings.Contains(finalContent, "Worker update ready.") ||
		strings.Contains(finalContent, "Coagent update ready.") ||
		strings.Contains(finalContent, "Research findings ready.") {
		t.Fatalf("final vtext revision contains raw status/tool chatter: %s", finalContent)
	}

	eventsByTrajectory, err := db.ListEventsByTrajectory(context.Background(), liveLLMOwnerID, conductorID, 500)
	if err != nil {
		t.Fatalf("list live trace events: %v", err)
	}
	if !liveSuccessfulToolResult(eventsByTrajectory, "web_search") {
		t.Fatalf("trace missing successful web_search tool result")
	}
	if !liveSuccessfulToolResult(eventsByTrajectory, "submit_coagent_update") {
		t.Fatalf("trace missing successful submit_coagent_update tool result")
	}
	if !liveSuccessfulToolResult(eventsByTrajectory, "write_file") {
		t.Fatalf("trace missing successful write_file tool result")
	}
	if !liveSuccessfulBashResult(eventsByTrajectory) {
		t.Fatalf("trace missing successful bash verification result")
	}
	if !liveEventsContain(eventsByTrajectory, types.EventVTextAgentRevisionCompleted, "") {
		t.Fatalf("trace missing vtext revision completion")
	}
	if _, err := rt.VerifyVTextWorkflow(context.Background(), choirruntime.VTextWorkflowVerificationOptions{
		OwnerID:                     liveLLMOwnerID,
		TrajectoryID:                conductorID,
		PromptSubmissionID:          conductorID,
		RequireResearchFindings:     true,
		RequireWorkerUpdates:        true,
		RequirePersistentSuper:      true,
		RequireSearchToolEvent:      true,
		RequireArtifactWriteEvent:   true,
		RequireVerificationCmdEvent: true,
		RequireWorkerConsumption:    true,
	}); err != nil {
		t.Fatalf("event-log verifier rejected live workflow: %v", err)
	}
}

type liveFinalRevision struct {
	revision types.Revision
	consumed []map[string]any
}

func waitLiveConsumedSeqs(t *testing.T, db *store.Store, docID, trajectoryID string, seqs []int64, timeout time.Duration) liveFinalRevision {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		doc, err := db.GetDocument(context.Background(), docID, liveLLMOwnerID)
		if err != nil || doc.CurrentRevisionID == "" {
			time.Sleep(500 * time.Millisecond)
			continue
		}
		head, err := db.GetRevision(context.Background(), doc.CurrentRevisionID, liveLLMOwnerID)
		if err != nil || head.AuthorKind != types.AuthorAppAgent {
			time.Sleep(500 * time.Millisecond)
			continue
		}
		revisions, err := db.ListRevisionsByDoc(context.Background(), docID, liveLLMOwnerID, 50)
		if err != nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}
		allConsumed := []map[string]any{}
		for _, rev := range revisions {
			if rev.AuthorKind != types.AuthorAppAgent {
				continue
			}
			allConsumed = append(allConsumed, liveConsumedWorkerUpdates(rev.Metadata)...)
		}
		if liveHasSeqs(allConsumed, seqs) {
			return liveFinalRevision{revision: head, consumed: allConsumed}
		}
		time.Sleep(500 * time.Millisecond)
	}
	revs, _ := db.ListRevisionsByDoc(context.Background(), docID, liveLLMOwnerID, 20)
	t.Fatalf("timed out waiting for vtext to consume message seqs %v on trajectory %s; revisions=%+v", seqs, trajectoryID, revs)
	return liveFinalRevision{}
}

func liveConsumedWorkerUpdates(raw json.RawMessage) []map[string]any {
	var meta map[string]any
	_ = json.Unmarshal(raw, &meta)
	items, _ := meta["worker_updates_consumed"].([]any)
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		entry, ok := item.(map[string]any)
		if ok {
			out = append(out, entry)
		}
	}
	return out
}

func liveHasSeqs(consumed []map[string]any, seqs []int64) bool {
	for _, seq := range seqs {
		found := false
		for _, item := range consumed {
			if got, ok := liveMetadataSeq(item["seq"]); ok && got == seq {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func liveMetadataSeq(value any) (int64, bool) {
	switch v := value.(type) {
	case float64:
		return int64(v), true
	case int64:
		return v, true
	case int:
		return int64(v), true
	default:
		return 0, false
	}
}

func liveEventsContain(events []types.EventRecord, kind types.EventKind, text string) bool {
	for _, ev := range events {
		if ev.Kind != kind {
			continue
		}
		if text == "" || strings.Contains(string(ev.Payload), text) {
			return true
		}
	}
	return false
}

func liveSuccessfulToolResult(events []types.EventRecord, tool string) bool {
	for _, ev := range events {
		if ev.Kind != types.EventToolResult {
			continue
		}
		var payload map[string]any
		if err := json.Unmarshal(ev.Payload, &payload); err != nil {
			continue
		}
		got, _ := payload["tool"].(string)
		isError, _ := payload["is_error"].(bool)
		if got == tool && !isError {
			return true
		}
	}
	return false
}

func liveSuccessfulBashResult(events []types.EventRecord) bool {
	for _, ev := range events {
		if ev.Kind != types.EventToolResult {
			continue
		}
		var payload map[string]any
		if err := json.Unmarshal(ev.Payload, &payload); err != nil {
			continue
		}
		got, _ := payload["tool"].(string)
		isError, _ := payload["is_error"].(bool)
		if got != "bash" || isError {
			continue
		}
		rawOutput, _ := payload["output"].(string)
		var output struct {
			ExitCode int `json:"exit_code"`
		}
		if err := json.Unmarshal([]byte(rawOutput), &output); err == nil && output.ExitCode == 0 {
			return true
		}
	}
	return false
}

func postLiveJSON(t *testing.T, handler http.HandlerFunc, method, path string, body any) map[string]any {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal %s: %v", path, err)
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Authenticated-User", liveLLMOwnerID)
	w := httptest.NewRecorder()
	handler(w, req)
	if w.Code < 200 || w.Code >= 300 {
		t.Fatalf("%s %s status=%d body=%s", method, path, w.Code, w.Body.String())
	}
	var out map[string]any
	if err := json.NewDecoder(w.Body).Decode(&out); err != nil {
		t.Fatalf("decode %s response: %v\n%s", path, err, w.Body.String())
	}
	return out
}

func stringField(t *testing.T, data map[string]any, key string) string {
	t.Helper()
	value, _ := data[key].(string)
	value = strings.TrimSpace(value)
	if value == "" {
		t.Fatalf("response missing %s: %+v", key, data)
	}
	return value
}

func waitLiveRun(t *testing.T, rt *choirruntime.Runtime, runID, ownerID string, timeout time.Duration) *types.RunRecord {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		rec, err := rt.GetRun(context.Background(), runID, ownerID)
		if err == nil && rec.State.Terminal() {
			return rec
		}
		time.Sleep(500 * time.Millisecond)
	}
	rec, err := rt.GetRun(context.Background(), runID, ownerID)
	if err != nil {
		t.Fatalf("get run %s after timeout: %v", runID, err)
	}
	t.Fatalf("run %s did not finish within %s: state=%s error=%q", runID, timeout, rec.State, rec.Error)
	return nil
}

func TestLiveLLMProviderInfo(t *testing.T) {
	authPath := strings.TrimSpace(os.Getenv("CHATGPT_AUTH_PATH"))
	if authPath == "" {
		home, _ := os.UserHomeDir()
		authPath = filepath.Join(home, ".codex", "auth.json")
	}
	if _, err := os.Stat(authPath); err == nil {
		fmt.Fprintf(os.Stderr, "live ChatGPT auth file available at %s; set GO_CHOIR_LIVE_LLM=1 and GO_CHOIR_LIVE_LLM_FAKE_SEARCH=1 to run the live-LLM/fake-search dry-run\n", authPath)
		return
	}
	t.Log("No ChatGPT/Codex auth file found; live workflow test will be skipped unless CHATGPT_AUTH_PATH is configured.")
}
