//go:build comprehensive

package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/provider"
	"github.com/yusefmosiah/go-choir/internal/provideriface"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestSubmitTaskPersistsToStore(t *testing.T) {
	t.Parallel()
	rt, s := testRuntime(t)
	ctx := context.Background()

	rec, err := rt.StartRun(ctx, "test prompt", "user-bob")
	if err != nil {
		t.Fatalf("submit task: %v", err)
	}

	// Verify the task is persisted in the store.
	stored, err := s.GetRun(ctx, rec.RunID)
	if err != nil {
		t.Fatalf("get task from store: %v", err)
	}
	if stored.RunID != rec.RunID {
		t.Errorf("loop_id: got %q, want %q", stored.RunID, rec.RunID)
	}
	if stored.OwnerID != "user-bob" {
		t.Errorf("owner_id: got %q, want user-bob", stored.OwnerID)
	}
}

func TestConductorTaskNormalizesStructuredRouteResult(t *testing.T) {
	t.Parallel()
	rt, s := testRuntime(t)
	ctx := context.Background()

	rec, err := rt.StartRunWithMetadata(ctx, "hi", "user-alice", map[string]any{
		runMetadataAgentProfile:  "conductor",
		runMetadataAgentRole:     "conductor",
		"input_source":           "prompt_bar",
		"requested_app":          agentprofile.Texture,
		"seed_prompt":            "hi",
		"initial_document_title": "hi",
	})
	if err != nil {
		t.Fatalf("submit conductor task: %v", err)
	}

	stored := waitForRunTerminalState(t, rt, rec.RunID, "user-alice", 5*time.Second)
	if stored.State != types.RunCompleted {
		t.Fatalf("state: got %q, want %q", stored.State, types.RunCompleted)
	}

	var result struct {
		Action               string `json:"action"`
		App                  string `json:"app"`
		Title                string `json:"title"`
		SeedPrompt           string `json:"seed_prompt"`
		CreateInitialVersion bool   `json:"create_initial_version"`
		DocID                string `json:"doc_id"`
		UserRevisionID       string `json:"user_revision_id"`
		FramingRevisionID    string `json:"framing_revision_id"`
		InitialRevisionID    string `json:"initial_revision_id"`
		InitialRunID         string `json:"initial_loop_id"`
	}
	if err := json.Unmarshal([]byte(stored.Result), &result); err != nil {
		t.Fatalf("decode result json: %v\nraw=%q", err, stored.Result)
	}
	if result.Action != "open_app" {
		t.Fatalf("action: got %q, want open_app", result.Action)
	}
	if result.App != agentprofile.Texture {
		t.Fatalf("app: got %q, want %q", result.App, agentprofile.Texture)
	}
	if result.SeedPrompt != "hi" {
		t.Fatalf("seed_prompt: got %q, want hi", result.SeedPrompt)
	}
	if result.CreateInitialVersion {
		t.Fatal("create_initial_version: got true, want false")
	}
	if result.DocID == "" {
		t.Fatal("doc_id should not be empty")
	}
	if result.UserRevisionID == "" {
		t.Fatal("user_revision_id should not be empty")
	}
	if result.FramingRevisionID != "" {
		t.Fatalf("framing_revision_id = %q, want empty because conductor cannot write appagent text", result.FramingRevisionID)
	}
	if result.InitialRevisionID != result.UserRevisionID {
		t.Fatalf("initial_revision_id: got %q, want user seed revision %q", result.InitialRevisionID, result.UserRevisionID)
	}
	if result.InitialRunID == "" {
		t.Fatal("initial_loop_id should point to the product-path texture run")
	}

	doc, err := s.GetDocument(ctx, result.DocID, "user-alice")
	if err != nil {
		t.Fatalf("get document: %v", err)
	}
	if doc.CurrentRevisionID != result.UserRevisionID {
		t.Fatalf("document head: got %q, want user seed revision %q before Texture writes", doc.CurrentRevisionID, result.UserRevisionID)
	}

	v0, err := s.GetRevision(ctx, result.UserRevisionID, "user-alice")
	if err != nil {
		t.Fatalf("get v0 revision: %v", err)
	}
	if v0.AuthorKind != types.AuthorUser {
		t.Fatalf("v0 author_kind: got %q, want %q", v0.AuthorKind, types.AuthorUser)
	}
	if v0.Content != "hi" {
		t.Fatalf("v0 content: got %q, want exact owner prompt %q as canonical Texture V0", v0.Content, "hi")
	}
	meta := decodeRevisionMetadata(v0.Metadata)
	if metadataString(meta, "conductor_loop_id") != rec.RunID {
		t.Fatalf("v0 conductor_loop_id: got %q, want %q", metadataString(meta, "conductor_loop_id"), rec.RunID)
	}
	if metadataString(meta, "seed_prompt") != "hi" {
		t.Fatalf("v0 seed_prompt provenance metadata: got %q, want hi", metadataString(meta, "seed_prompt"))
	}
	if metadataBoolValue(meta, "prompt_bar_instruction_revision") {
		t.Fatalf("v0 must not carry prompt_bar_instruction_revision marker: %v", meta["prompt_bar_instruction_revision"])
	}
	if metadataString(meta, "input_origin") != textureInputOriginUserPrompt {
		t.Fatalf("v0 input_origin provenance: got %q, want %q", metadataString(meta, "input_origin"), textureInputOriginUserPrompt)
	}
	if metadataString(meta, "texture_version") != "v0" {
		t.Fatalf("v0 metadata version: got %q, want v0", metadataString(meta, "texture_version"))
	}
	promptUnixTS := metadataIntValue(meta, textureMetadataPromptUnixTS)
	if promptUnixTS <= 0 {
		t.Fatalf("v0 prompt_unix_ts: got %d, want positive unix timestamp", promptUnixTS)
	}
	if delta := time.Since(time.Unix(int64(promptUnixTS), 0)); delta < 0 || delta > 2*time.Minute {
		t.Fatalf("v0 prompt_unix_ts %d too far from now: delta=%v", promptUnixTS, delta)
	}

	runs, err := s.ListRunsByOwner(ctx, "user-alice", 20)
	if err != nil {
		t.Fatalf("list runs: %v", err)
	}
	foundInitialTextureRun := false
	for _, run := range runs {
		if run.AgentProfile == agentprofile.Texture && run.RunID == result.InitialRunID {
			foundInitialTextureRun = true
		}
	}
	if !foundInitialTextureRun {
		t.Fatalf("prompt creation should start a product-path texture run %q; runs=%+v", result.InitialRunID, runs)
	}
	waitForRunTerminalState(t, rt, result.InitialRunID, "user-alice", 5*time.Second)
	if mutation, err := s.GetPendingAgentMutationByDoc(ctx, "user-alice", "autoputer-test", result.DocID); err != nil {
		t.Fatalf("get pending mutation: %v", err)
	} else if mutation != nil {
		t.Fatalf("initial texture run should not leave a dangling pending mutation after completion, got %+v", mutation)
	}
}

func TestConductorPromptBarStructuredDecisionMaterializesTextureRoute(t *testing.T) {
	t.Parallel()
	rt, s := testRuntime(t)
	provider := rt.provider.(*provider.StubProvider)
	provider.Result = `{"action":"open_app","app":"texture","title":"Durable document","initial_content":"# Durable document\n\nInitial conductor-authored abstract."}`
	rt.Start(context.Background())

	rec, err := rt.StartRunWithMetadata(context.Background(), "make a durable document", "user-alice", map[string]any{
		runMetadataAgentProfile:  agentprofile.Conductor,
		runMetadataAgentRole:     agentprofile.Conductor,
		"input_source":           "prompt_bar",
		"requested_app":          agentprofile.Texture,
		"seed_prompt":            "make a durable document",
		"initial_document_title": "make a durable document",
	})
	if err != nil {
		t.Fatalf("start conductor run: %v", err)
	}

	stored := waitForRunTerminalState(t, rt, rec.RunID, "user-alice", 5*time.Second)
	if stored.State != types.RunCompleted {
		t.Fatalf("state = %q error=%q", stored.State, stored.Error)
	}
	var result conductorDecision
	if err := json.Unmarshal([]byte(stored.Result), &result); err != nil {
		t.Fatalf("decode result: %v\n%s", err, stored.Result)
	}
	if result.Action != "open_app" || result.App != agentprofile.Texture || result.DocID == "" {
		t.Fatalf("conductor result = %+v, want materialized Texture route", result)
	}
	// Conductor decisions no longer ship canonical document content; the
	// texture agent owns authoring, so any provider-supplied initial_content
	// must be stripped from the materialized route.
	if result.InitialContent != "" {
		t.Fatalf("initial_content = %q, want empty (texture owns canonical content)", result.InitialContent)
	}
	if result.CreateInitialVersion == nil || *result.CreateInitialVersion {
		t.Fatalf("create_initial_version = %v, want explicit false", result.CreateInitialVersion)
	}
	if result.InitialLoopID == "" {
		t.Fatalf("conductor result missing initial texture revision loop id: %+v", result)
	}
	doc, err := s.GetDocument(context.Background(), result.DocID, "user-alice")
	if err != nil {
		t.Fatalf("get materialized document: %v", err)
	}
	if doc.CurrentRevisionID == "" {
		t.Fatalf("materialized document has no current revision: %+v", doc)
	}
	rev, err := s.GetRevision(context.Background(), doc.CurrentRevisionID, "user-alice")
	if err != nil {
		t.Fatalf("get seed prompt revision: %v", err)
	}
	if rev.Content != "make a durable document" {
		t.Fatalf("seed revision content = %q, want exact owner prompt as canonical V0", rev.Content)
	}
	revMeta := decodeRevisionMetadata(rev.Metadata)
	if metadataString(revMeta, "seed_prompt") != "make a durable document" {
		t.Fatalf("seed revision provenance metadata = %#v, want seed prompt", revMeta)
	}
	if metadataBoolValue(revMeta, "prompt_bar_instruction_revision") {
		t.Fatalf("seed revision must not carry prompt_bar_instruction_revision: %v", revMeta["prompt_bar_instruction_revision"])
	}
}

func TestGetRunCallerScoped(t *testing.T) {
	t.Parallel()
	rt, _ := testRuntime(t)
	ctx := context.Background()

	rec, err := rt.StartRun(ctx, "test prompt", "user-alice")
	if err != nil {
		t.Fatalf("submit task: %v", err)
	}

	// Owner can see their own task.
	got, err := rt.GetRun(ctx, rec.RunID, "user-alice")
	if err != nil {
		t.Fatalf("get own task: %v", err)
	}
	if got.RunID != rec.RunID {
		t.Errorf("loop_id: got %q, want %q", got.RunID, rec.RunID)
	}

	// Another user cannot see the task (VAL-RUNTIME-006).
	_, err = rt.GetRun(ctx, rec.RunID, "user-eve")
	if err == nil {
		t.Error("expected error when getting another user's task")
	}
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestGetRunNotFound(t *testing.T) {
	t.Parallel()
	rt, _ := testRuntime(t)
	ctx := context.Background()

	_, err := rt.GetRun(ctx, "nonexistent-task-id", "user-alice")
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestProviderFailureSurfacesStructuredOutcome(t *testing.T) {
	t.Parallel()
	// VAL-RUNTIME-008: provider failures surface as structured task outcomes
	// without crashing the runtime.
	dir := filepath.Join(os.TempDir(), "go-choir-m3-runtime-test")
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
	// Create a provider that always fails.
	provider := &provider.StubProvider{
		Delay:   10 * time.Millisecond,
		FailErr: errors.New("provider timeout after 30s"),
		Result:  "",
	}

	cfg := provideriface.Config{
		ComputerID:          "autoputer-test",
		StorePath:           dbPath,
		ProviderTimeout:     time.Second,
		SupervisionInterval: 1 * time.Hour,
	}

	rt := New(cfg, s, bus, provider)
	setTestDispatch(rt, s)

	t.Cleanup(func() {
		rt.Stop()
		_ = s.Close()
		_ = os.Remove(dbPath)
	})

	rec, err := rt.StartRun(context.Background(), "failing prompt", "user-alice")
	if err != nil {
		t.Fatalf("submit task: %v", err)
	}

	got := waitForRunTerminalState(t, rt, rec.RunID, "user-alice", 5*time.Second)

	if got.State != types.RunFailed {
		t.Errorf("state: got %q, want %q", got.State, types.RunFailed)
	}
	if got.Error == "" {
		t.Error("error should be set for failed task")
	}
	if got.FinishedAt == nil {
		t.Error("finished_at should be set for failed task")
	}

	// Runtime should remain available for new runs.
	nextRec, err := rt.StartRun(context.Background(), "next prompt", "user-alice")
	if err != nil {
		t.Fatalf("submit task after failure: %v", err)
	}
	if nextRec.RunID == "" {
		t.Error("loop_id should not be empty for task submitted after failure")
	}
}

func TestTaskRecoveryAcrossRestart(t *testing.T) {
	t.Parallel()
	// VAL-RUNTIME-010: accepted task state remains recoverable after
	// autoputer restart.
	dir := filepath.Join(os.TempDir(), "go-choir-m3-runtime-test")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	dbPath := filepath.Join(dir, t.Name()+".db")
	_ = os.Remove(dbPath)

	// Open store, create runtime, submit a task, and stop.
	s1, err := openTestStore(dbPath)
	if err != nil {
		t.Fatalf("open store 1: %v", err)
	}

	bus1 := events.NewEventBus()
	cfg := provideriface.Config{
		ComputerID:          "autoputer-test",
		StorePath:           dbPath,
		ProviderTimeout:     time.Second,
		SupervisionInterval: 1 * time.Hour,
	}
	provider1 := provider.NewStubProvider(50 * time.Millisecond)
	rt1 := New(cfg, s1, bus1, provider1)

	rec, err := rt1.StartRun(context.Background(), "survive restart", "user-alice")
	if err != nil {
		t.Fatalf("submit task: %v", err)
	}

	waitForRunTerminalState(t, rt1, rec.RunID, "user-alice", 5*time.Second)

	// Stop the first runtime.
	rt1.Stop()
	_ = s1.Close()

	// Reopen the store and create a new runtime (simulates restart).
	s2, err := openTestStore(dbPath)
	if err != nil {
		t.Fatalf("open store 2: %v", err)
	}

	bus2 := events.NewEventBus()
	provider2 := provider.NewStubProvider(50 * time.Millisecond)
	rt2 := New(cfg, s2, bus2, provider2)
	setTestDispatch(rt2, s2)

	t.Cleanup(func() {
		rt2.Stop()
		_ = s2.Close()
		_ = os.Remove(dbPath)
	})

	// The previously completed task should be recoverable by handle.
	got, err := rt2.GetRun(context.Background(), rec.RunID, "user-alice")
	if err != nil {
		t.Fatalf("get task after restart: %v", err)
	}

	if got.RunID != rec.RunID {
		t.Errorf("loop_id: got %q, want %q", got.RunID, rec.RunID)
	}
	if got.State != types.RunCompleted {
		t.Errorf("state: got %q, want %q", got.State, types.RunCompleted)
	}
	if got.Prompt != "survive restart" {
		t.Errorf("prompt: got %q, want original", got.Prompt)
	}
}

func TestInterruptedRunningTasksPassivatedOnStart(t *testing.T) {
	t.Parallel()
	// When the autoputer restarts, runs that were running should be passivated:
	// the in-process activation is gone, but durable agent work is not failed.
	dir := filepath.Join(os.TempDir(), "go-choir-m3-runtime-test")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	dbPath := filepath.Join(dir, t.Name()+".db")
	_ = os.Remove(dbPath)

	ctx := context.Background()

	// Create a store with a running task that was interrupted.
	s1, err := openTestStore(dbPath)
	if err != nil {
		t.Fatalf("open store 1: %v", err)
	}

	now := time.Now().UTC()
	interruptedTask := types.RunRecord{
		RunID:      "interrupted-task-001",
		OwnerID:    "user-alice",
		ComputerID: "autoputer-test",
		State:      types.RunRunning, // was running when process exited
		Prompt:     "interrupted prompt",
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := s1.CreateRun(ctx, interruptedTask); err != nil {
		t.Fatalf("create interrupted task: %v", err)
	}
	_ = s1.Close()

	// Simulate restart: open new store and runtime, then call Start()
	// which should passivate interrupted activations.
	s2, err := openTestStore(dbPath)
	if err != nil {
		t.Fatalf("open store 2: %v", err)
	}

	bus := events.NewEventBus()
	cfg := provideriface.Config{
		ComputerID:          "autoputer-test",
		StorePath:           dbPath,
		ProviderTimeout:     time.Second,
		SupervisionInterval: 1 * time.Hour,
	}
	provider := provider.NewStubProvider(50 * time.Millisecond)
	rt := New(cfg, s2, bus, provider)
	setTestDispatch(rt, s2)

	t.Cleanup(func() {
		rt.Stop()
		_ = s2.Close()
		_ = os.Remove(dbPath)
	})
	rt.Start(ctx)

	// The interrupted run should now be passivated, not failed.
	got, err := rt.GetRun(ctx, "interrupted-task-001", "user-alice")
	if err != nil {
		t.Fatalf("get interrupted task: %v", err)
	}
	if got.State != types.RunPassivated {
		t.Errorf("state: got %q, want %q", got.State, types.RunPassivated)
	}
	if got.Error != "" {
		t.Errorf("error: got %q, want empty", got.Error)
	}
	if got.FinishedAt != nil {
		t.Errorf("finished_at = %v, want nil", got.FinishedAt)
	}
	if metadataStringValue(got.Metadata, "passivated_reason") != "runtime_restarted" {
		t.Errorf("passivated_reason = %q, want runtime_restarted", metadataStringValue(got.Metadata, "passivated_reason"))
	}
}

func TestInterruptedActivationPassivationDrainsBatches(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(os.TempDir(), "go-choir-m3-runtime-test")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	dbPath := filepath.Join(dir, t.Name()+".db")
	_ = os.Remove(dbPath)

	ctx := context.Background()
	s, err := openTestStore(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() {
		_ = s.Close()
		_ = os.Remove(dbPath)
	})

	now := time.Now().UTC()
	states := []types.RunState{types.RunPending, types.RunRunning}
	for _, state := range states {
		for i := 0; i < 105; i++ {
			rec := types.RunRecord{
				RunID:      fmt.Sprintf("interrupted-%s-%03d", state, i),
				OwnerID:    "user-alice",
				ComputerID: "autoputer-test",
				State:      state,
				Prompt:     "interrupted prompt",
				CreatedAt:  now,
				UpdatedAt:  now,
			}
			if err := s.CreateRun(ctx, rec); err != nil {
				t.Fatalf("create %s run %d: %v", state, i, err)
			}
		}
	}

	rt := New(provideriface.Config{ComputerID: "autoputer-test"}, s, events.NewEventBus(), provider.NewStubProvider(0))
	setTestDispatch(rt, s)
	rt.passivateInterruptedActivations(ctx)

	for _, state := range states {
		remaining, err := s.ListRunsByState(ctx, state, 200)
		if err != nil {
			t.Fatalf("list remaining %s runs: %v", state, err)
		}
		if len(remaining) != 0 {
			t.Fatalf("remaining %s runs = %d, want 0", state, len(remaining))
		}
		for i := 0; i < 105; i++ {
			runID := fmt.Sprintf("interrupted-%s-%03d", state, i)
			got, err := s.GetRun(ctx, runID)
			if err != nil {
				t.Fatalf("get passivated run %s: %v", runID, err)
			}
			if got.State != types.RunPassivated {
				t.Fatalf("%s state = %q, want %q", runID, got.State, types.RunPassivated)
			}
		}
	}
}
func TestRestartReDispatchesProjectedLifecycleActivation(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	ownerID, docID := "user-lifecycle-restart", "doc-lifecycle-restart"
	trajectoryID := seedDurableTextureSubject(t, s, ownerID, docID)
	workID := "test-work:" + ownerID + ":" + docID
	var dispatched []string
	rt.SetDispatchActor(func(_ context.Context, gotOwnerID, gotComputerID, targetAgentID, kind, content, gotTrajectoryID, _ string) error {
		if kind == "lifecycle_work_assigned" && gotOwnerID == ownerID && gotComputerID == rt.TextureComputerID() &&
			targetAgentID == currentTextureAgentID(docID) && gotTrajectoryID == trajectoryID {
			dispatched = append(dispatched, content)
		}
		return nil
	})
	rt.SetKernelMode()
	rt.sweepActorWakeOutbox(ctx)
	if len(dispatched) != 1 || !strings.Contains(dispatched[0], workID) {
		t.Fatalf("restart lifecycle work wakes = %v, want one wake for %s", dispatched, workID)
	}
}

func TestRestartReconcilesDurableTerminalLifecycleSettlementTrigger(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	ownerID, docID := "user-terminal-settlement-restart", "doc-terminal-settlement-restart"
	trajectoryID := seedDurableTextureSubject(t, s, ownerID, docID)
	now := time.Now().UTC()
	run := types.RunRecord{
		RunID: "run-terminal-settlement-before-restart", AgentID: currentTextureAgentID(docID),
		OwnerID: ownerID, ComputerID: rt.TextureComputerID(), ChannelID: docID, TrajectoryID: trajectoryID,
		State: types.RunPending, Prompt: "complete durable lifecycle", AgentProfile: "texture", AgentRole: "texture",
		CreatedAt: now, UpdatedAt: now,
		Metadata: map[string]any{
			runMetadataAgentID: currentTextureAgentID(docID), runMetadataAgentProfile: "texture",
			runMetadataAgentRole: "texture", runMetadataTrajectoryID: trajectoryID,
			"lifecycle_work_item_id": "test-work:" + ownerID + ":" + docID,
		},
	}
	if err := s.CreateRun(ctx, run); err != nil {
		t.Fatalf("create lifecycle activation: %v", err)
	}
	settleWork := types.SettleLifecycleWorkRequest{
		OwnerID: ownerID, ComputerID: rt.TextureComputerID(),
		CommandID: "command-terminal-settlement-work", TrajectoryID: trajectoryID,
		WorkItemID: "test-work:" + ownerID + ":" + docID, ActingAgentID: currentTextureAgentID(docID),
		ResultRef: "test-revision:" + ownerID + ":" + docID,
	}
	settleWork.CommandDigest, _ = store.ComputeSettleLifecycleWorkDigest(settleWork)
	if _, err := s.SettleLifecycleWork(ctx, settleWork); err != nil {
		t.Fatalf("settle lifecycle work: %v", err)
	}
	run.State = types.RunCompleted
	run.UpdatedAt = time.Now().UTC()
	run.FinishedAt = &run.UpdatedAt
	project := types.ReplaceLifecycleActivationRequest{
		OwnerID: ownerID, ComputerID: rt.TextureComputerID(),
		CommandID: "command-terminal-projection-before-restart", TrajectoryID: trajectoryID,
		AgentID: run.AgentID, Run: run,
	}
	project.CommandDigest, _ = store.ComputeReplaceLifecycleActivationDigest(project)
	if _, err := s.ProjectTerminalLifecycleRun(ctx, project); err != nil {
		t.Fatalf("project terminal lifecycle run: %v", err)
	}
	before, err := s.GetLifecycleSnapshot(ctx, ownerID, rt.TextureComputerID(), trajectoryID)
	if err != nil || before.Trajectory.Status != types.TrajectoryLive {
		t.Fatalf("terminal projection settled without reducer caller: %+v, %v", before, err)
	}
	genericRuns, err := s.ListAllRunsByState(ctx, types.RunCompleted)
	if err != nil {
		t.Fatalf("list generic terminal runs: %v", err)
	}
	for _, genericRun := range genericRuns {
		if genericRun.RunID == run.RunID {
			t.Fatalf("lifecycle run leaked into generic boot listing: %+v", genericRun)
		}
	}
	lifecycleRuns, err := s.ListLifecycleRunsByState(ctx, ownerID, rt.TextureComputerID(), types.RunCompleted)
	if err != nil || len(lifecycleRuns) != 1 || lifecycleRuns[0].RunID != run.RunID {
		t.Fatalf("lifecycle boot listing = %+v, %v", lifecycleRuns, err)
	}
	rt.reconcileTerminalRunOutcomes(ctx)
	after, err := s.GetLifecycleSnapshot(ctx, ownerID, rt.TextureComputerID(), trajectoryID)
	if err != nil || after.Trajectory.Status != types.TrajectorySettled ||
		after.Activation.State != types.RunCompleted {
		t.Fatalf("boot settlement reconciliation = %+v, %v", after, err)
	}
}

func TestRuntimeTerminalPersistenceSettlesReadyLifecycle(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	ownerID, docID := "user-runtime-terminal-settlement", "doc-runtime-terminal-settlement"
	trajectoryID := seedDurableTextureSubject(t, s, ownerID, docID)
	now := time.Now().UTC()
	run := types.RunRecord{
		RunID: "run-runtime-terminal-settlement", AgentID: currentTextureAgentID(docID),
		OwnerID: ownerID, ComputerID: rt.TextureComputerID(), ChannelID: docID, TrajectoryID: trajectoryID,
		State: types.RunPending, Prompt: "complete durable lifecycle", AgentProfile: "texture", AgentRole: "texture",
		CreatedAt: now, UpdatedAt: now,
		Metadata: map[string]any{
			runMetadataAgentID: currentTextureAgentID(docID), runMetadataAgentProfile: "texture",
			runMetadataAgentRole: "texture", runMetadataTrajectoryID: trajectoryID,
			"lifecycle_work_item_id": "test-work:" + ownerID + ":" + docID,
		},
	}
	if err := s.CreateRun(ctx, run); err != nil {
		t.Fatalf("create lifecycle activation: %v", err)
	}
	settleWork := types.SettleLifecycleWorkRequest{
		OwnerID: ownerID, ComputerID: rt.TextureComputerID(),
		CommandID: "command-runtime-terminal-work", TrajectoryID: trajectoryID,
		WorkItemID: "test-work:" + ownerID + ":" + docID, ActingAgentID: currentTextureAgentID(docID),
		ResultRef: "test-revision:" + ownerID + ":" + docID,
	}
	settleWork.CommandDigest, _ = store.ComputeSettleLifecycleWorkDigest(settleWork)
	if _, err := s.SettleLifecycleWork(ctx, settleWork); err != nil {
		t.Fatalf("settle lifecycle work: %v", err)
	}
	run.State = types.RunCompleted
	run.UpdatedAt = time.Now().UTC()
	run.FinishedAt = &run.UpdatedAt
	persisted, err := rt.persistActivationState(ctx, &run)
	if err != nil || !persisted {
		t.Fatalf("persist runtime terminal activation: persisted=%t err=%v", persisted, err)
	}
	snapshot, err := s.GetLifecycleSnapshot(ctx, ownerID, rt.TextureComputerID(), trajectoryID)
	if err != nil || snapshot.Trajectory.Status != types.TrajectorySettled ||
		snapshot.Activation.State != types.RunCompleted {
		t.Fatalf("runtime terminal settlement = %+v, %v", snapshot, err)
	}
}

func TestListRunsByOwner(t *testing.T) {
	t.Parallel()
	rt, _ := testRuntime(t)
	ctx := context.Background()

	// Submit runs for two owners.
	_, err := rt.StartRun(ctx, "alice task 1", "user-alice")
	if err != nil {
		t.Fatalf("submit alice task: %v", err)
	}
	_, err = rt.StartRun(ctx, "bob task 1", "user-bob")
	if err != nil {
		t.Fatalf("submit bob task: %v", err)
	}
	_, err = rt.StartRun(ctx, "alice task 2", "user-alice")
	if err != nil {
		t.Fatalf("submit alice task 2: %v", err)
	}

	aliceTasks, err := rt.ListRunsByOwner(ctx, "user-alice", 10)
	if err != nil {
		t.Fatalf("list alice runs: %v", err)
	}
	if len(aliceTasks) != 2 {
		t.Errorf("alice runs: got %d, want 2", len(aliceTasks))
	}

	bobTasks, err := rt.ListRunsByOwner(ctx, "user-bob", 10)
	if err != nil {
		t.Fatalf("list bob runs: %v", err)
	}
	if len(bobTasks) != 1 {
		t.Errorf("bob runs: got %d, want 1", len(bobTasks))
	}
}

// --- Bridge Provider Integration Tests ---

// mockBridgeProvider implements the runtime.Provider interface for testing
// the bridge provider integration with the runtime engine.
type mockBridgeProvider struct {
	name       string
	result     string
	execErr    error
	mu         sync.Mutex
	called     bool
	taskResult string // captures the result set by Execute on the RunRecord
}

func (m *mockBridgeProvider) Execute(ctx context.Context, task *types.RunRecord, emit provideriface.EventEmitFunc) error {
	m.mu.Lock()
	m.called = true
	m.mu.Unlock()

	if m.execErr != nil {
		emit(types.EventRunProgress, "execution", json.RawMessage(`{"status":"failed","real":"true"}`))
		return m.execErr
	}

	emit(types.EventRunProgress, "execution", json.RawMessage(`{"status":"started","provider":"`+m.name+`","real":"true"}`))
	emit(types.EventRunDelta, "execution", json.RawMessage(`{"text":"`+m.result+`","provider":"`+m.name+`","real":"true"}`))
	task.Result = m.result
	m.mu.Lock()
	m.taskResult = m.result
	m.mu.Unlock()
	return nil
}

func (m *mockBridgeProvider) ProviderName() string { return m.name }

func testRuntimeWithBridge(t *testing.T, bridge provideriface.Provider) (*Runtime, *store.Store) {
	t.Helper()

	dir := filepath.Join(os.TempDir(), "go-choir-m3-bridge-test")
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
	cfg := provideriface.Config{
		ComputerID:          "autoputer-bridge-test",
		StorePath:           dbPath,
		ProviderTimeout:     50 * time.Millisecond,
		SupervisionInterval: 1 * time.Hour,
	}

	rt := New(cfg, s, bus, bridge)
	setTestDispatch(rt, s)
	t.Cleanup(func() {
		rt.Stop()
		_ = s.Close()
		_ = os.Remove(dbPath)
	})

	return rt, s
}

func TestBridgeProviderFailureSurfacesWithoutCrashing(t *testing.T) {
	t.Parallel()
	bridge := &mockBridgeProvider{
		name:    "zai",
		execErr: fmt.Errorf("upstream provider timeout"),
	}

	rt, _ := testRuntimeWithBridge(t, bridge)
	ctx := context.Background()

	rec, err := rt.StartRun(ctx, "This should fail at the provider", "user-fail")
	if err != nil {
		t.Fatalf("submit task: %v", err)
	}

	// The task should be in failed state, not crashing the runtime.
	stored := waitForRunTerminalState(t, rt, rec.RunID, "user-fail", 5*time.Second)
	if stored.State != types.RunFailed {
		t.Errorf("state: got %q, want failed", stored.State)
	}

	// The runtime should still be healthy for later runs.
	if rt.HealthState() == types.HealthFailed {
		t.Error("runtime should not be in failed state after a single provider error")
	}

	// Submit another task — should still work.
	rec2, err := rt.StartRun(ctx, "Another task after failure", "user-retry")
	if err != nil {
		t.Fatalf("submit task after failure: %v", err)
	}
	if rec2.RunID == "" {
		t.Error("second task should have a valid ID")
	}
}
