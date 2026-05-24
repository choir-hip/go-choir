//go:build comprehensive

package runtime

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestRunContinuationCompactsAndStartsBoundedNextGoal(t *testing.T) {
	ctx := context.Background()
	rt, s := testRuntime(t)
	rt.cfg.RunMemoryKeepRecentTokens = 1
	rt.cfg.RunMemoryContextThresholdTokens = 1
	if err := rt.InstallDefaultAgentTools(t.TempDir()); err != nil {
		t.Fatalf("install agent tools: %v", err)
	}

	source, err := rt.StartRunWithMetadata(ctx, "finish the current mission slice", "user-alice", map[string]any{
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

	selected, err := rt.SelectRunContinuation(ctx, done.RunID, "user-alice", ContinuationProposal{
		Objective:        "continue with the next candidate-world product patch",
		Reason:           "mission gradient selects the next verifier-dense product increment",
		AuthorityProfile: AgentProfileVSuper,
		LeaseSeconds:     60,
		Details:          map[string]any{"mission_doc": "docs/mission-choir-in-choir-deformation-v0.md"},
	})
	if err != nil {
		t.Fatalf("select continuation: %v", err)
	}
	if selected.Status != types.RunContinuationSelected || selected.AuthorityProfile != AgentProfileVSuper {
		t.Fatalf("selected continuation mismatch: %+v", selected)
	}
	if selected.Details["compaction_status"] != "completed" {
		t.Fatalf("continuation did not compact first: %+v", selected.Details)
	}
	fingerprint, _ := selected.Details["objective_fingerprint"].(string)
	if fingerprint == "" {
		t.Fatalf("continuation missing objective fingerprint: %+v", selected.Details)
	}
	duplicate, err := rt.SelectRunContinuation(ctx, done.RunID, "user-alice", ContinuationProposal{
		Objective:        " continue WITH the next/candidate world product patch!! ",
		Reason:           "same objective repeated by the controller",
		AuthorityProfile: AgentProfileVSuper,
		LeaseSeconds:     60,
	})
	if err != nil {
		t.Fatalf("select duplicate continuation: %v", err)
	}
	if duplicate.ContinuationID != selected.ContinuationID {
		t.Fatalf("duplicate continuation = %s, want existing %s", duplicate.ContinuationID, selected.ContinuationID)
	}
	continuations, err := s.ListRunContinuationsBySource(ctx, "user-alice", done.RunID)
	if err != nil {
		t.Fatalf("list continuations: %v", err)
	}
	if len(continuations) != 1 {
		t.Fatalf("continuations = %d, want one deduped continuation: %+v", len(continuations), continuations)
	}

	started, err := rt.StartRunContinuation(ctx, "user-alice", selected.ContinuationID)
	if err != nil {
		t.Fatalf("start continuation: %v", err)
	}
	if started.Status != types.RunContinuationStarted || started.NextRunID == "" {
		t.Fatalf("started continuation mismatch: %+v", started)
	}
	child := waitForRunTerminalState(t, rt, started.NextRunID, "user-alice", 5*time.Second)
	if child.AgentProfile != AgentProfileVSuper {
		t.Fatalf("child profile = %q, want %q", child.AgentProfile, AgentProfileVSuper)
	}
	if child.Metadata["objective_fingerprint"] != fingerprint {
		t.Fatalf("child objective_fingerprint = %v, want %q", child.Metadata["objective_fingerprint"], fingerprint)
	}

	events, err := s.ListEvents(ctx, done.RunID, 100)
	if err != nil {
		t.Fatalf("list source events: %v", err)
	}
	kinds := map[types.EventKind]bool{}
	for _, ev := range events {
		kinds[ev.Kind] = true
	}
	for _, kind := range []types.EventKind{
		types.EventRunCompactionCompleted,
		types.EventRunContinuationSelected,
		types.EventRunContinuationStarted,
	} {
		if !kinds[kind] {
			t.Fatalf("source run missing event %s; got %+v", kind, kinds)
		}
	}
}

func TestRunControlSynthesizesContinuationFromAppAdoption(t *testing.T) {
	ctx := context.Background()
	rt, s := testRuntime(t)
	rt.cfg.RunMemoryKeepRecentTokens = 1
	rt.cfg.RunMemoryContextThresholdTokens = 1
	if err := rt.InstallDefaultAgentTools(t.TempDir()); err != nil {
		t.Fatalf("install agent tools: %v", err)
	}

	source, err := rt.StartRunWithMetadata(ctx, "finish candidate-producing work", "user-alice", map[string]any{
		runMetadataAgentProfile: AgentProfileSuper,
		runMetadataAgentRole:    AgentProfileSuper,
		runMetadataTrajectoryID: "trace-run-control",
	})
	if err != nil {
		t.Fatalf("start source run: %v", err)
	}
	done := waitForRunTerminalState(t, rt, source.RunID, "user-alice", 5*time.Second)
	if done.State != types.RunCompleted {
		t.Fatalf("source state = %s", done.State)
	}

	if _, err := rt.store.UpsertAppAdoption(ctx, types.AppAdoptionRecord{
		AdoptionID:         "adoption-" + done.RunID,
		OwnerID:            "user-alice",
		PackageID:          "pkg-run-control",
		AppID:              "podcast",
		TargetComputerID:   "computer-run-control",
		TargetComputerKind: "user",
		TargetCandidateID:  "candidate-run-control",
		Status:             types.AppAdoptionCandidateApplied,
		CandidateSourceRef: "refs/computers/computer-run-control/candidates/candidate-run-control",
		TraceID:            "trace-run-control",
	}); err != nil {
		t.Fatalf("upsert app adoption: %v", err)
	}

	selected, err := rt.SelectSynthesizedRunContinuation(ctx, done.RunID, "user-alice")
	if err != nil {
		t.Fatalf("select synthesized continuation: %v", err)
	}
	if selected.AuthorityProfile != AgentProfileVSuper {
		t.Fatalf("authority profile = %q, want %q", selected.AuthorityProfile, AgentProfileVSuper)
	}
	if !strings.Contains(selected.Objective, "adoption-"+done.RunID) || !strings.Contains(selected.Objective, "Verify app adoption") {
		t.Fatalf("objective = %q, want app adoption verification objective", selected.Objective)
	}
	if selected.Details["signal"] != "app_adoption" || selected.Details["adoption_id"] != "adoption-"+done.RunID {
		t.Fatalf("details missing app adoption signal: %+v", selected.Details)
	}
	if selected.Details["compaction_status"] != "completed" {
		t.Fatalf("synthesized continuation did not compact first: %+v", selected.Details)
	}

	duplicate, err := rt.SelectSynthesizedRunContinuation(ctx, done.RunID, "user-alice")
	if err != nil {
		t.Fatalf("select duplicate synthesized continuation: %v", err)
	}
	if duplicate.ContinuationID != selected.ContinuationID {
		t.Fatalf("duplicate synthesized continuation = %s, want %s", duplicate.ContinuationID, selected.ContinuationID)
	}
	continuations, err := s.ListRunContinuationsBySource(ctx, "user-alice", done.RunID)
	if err != nil {
		t.Fatalf("list continuations: %v", err)
	}
	if len(continuations) != 1 {
		t.Fatalf("continuations = %d, want one synthesized continuation", len(continuations))
	}
}

func TestRunControlCompactsEventLedgerWhenSourceHasNoProviderMemory(t *testing.T) {
	ctx := context.Background()
	rt, s := testRuntime(t)
	now := time.Now().UTC()
	source := types.RunRecord{
		RunID:        "source-no-provider-memory",
		AgentID:      "agent-no-provider-memory",
		ChannelID:    "channel-no-provider-memory",
		AgentProfile: AgentProfileSuper,
		AgentRole:    AgentProfileSuper,
		OwnerID:      "user-alice",
		SandboxID:    "sandbox-test",
		State:        types.RunCompleted,
		Prompt:       "finish candidate-producing control-plane work",
		Result:       "queued a reviewable candidate without a provider transcript",
		CreatedAt:    now,
		UpdatedAt:    now,
		FinishedAt:   &now,
		Metadata: map[string]any{
			runMetadataAgentProfile: AgentProfileSuper,
			runMetadataAgentRole:    AgentProfileSuper,
			runMetadataTrajectoryID: "trace-no-provider-memory",
		},
	}
	if err := s.CreateRun(ctx, source); err != nil {
		t.Fatalf("create source run: %v", err)
	}
	rt.emitEvent(ctx, &source, types.EventRunSubmitted, events.CauseTaskLifecycle, json.RawMessage(`{"prompt_length":46}`))
	rt.emitEvent(ctx, &source, types.EventRunCompleted, events.CauseTaskLifecycle, json.RawMessage(`{"result_length":57}`))

	if _, err := rt.store.UpsertAppAdoption(ctx, types.AppAdoptionRecord{
		AdoptionID:         "adoption-" + source.RunID,
		OwnerID:            "user-alice",
		PackageID:          "pkg-no-provider-memory",
		AppID:              "podcast",
		TargetComputerID:   "computer-no-provider-memory",
		TargetComputerKind: "user",
		TargetCandidateID:  "candidate-no-provider-memory",
		Status:             types.AppAdoptionCandidateApplied,
		CandidateSourceRef: "refs/computers/computer-no-provider-memory/candidates/candidate-no-provider-memory",
		TraceID:            "trace-no-provider-memory",
	}); err != nil {
		t.Fatalf("upsert app adoption: %v", err)
	}

	selected, err := rt.SelectSynthesizedRunContinuation(ctx, source.RunID, "user-alice")
	if err != nil {
		t.Fatalf("select synthesized continuation: %v", err)
	}
	if selected.Details["compaction_status"] != "completed" {
		t.Fatalf("synthesized continuation did not record event-ledger compaction: %+v", selected.Details)
	}

	entries, err := s.ListRunMemoryEntries(ctx, "user-alice", source.RunID)
	if err != nil {
		t.Fatalf("list run memory entries: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("run memory entries = %d, want event-ledger checkpoint only: %+v", len(entries), entries)
	}
	checkpoint := entries[0]
	if checkpoint.Kind != types.RunMemoryEntryCompaction || checkpoint.Reason != "continuation_selection" {
		t.Fatalf("checkpoint = %+v, want continuation compaction entry", checkpoint)
	}
	if checkpoint.Details["source"] != "run_event_ledger" {
		t.Fatalf("checkpoint source = %v, want run_event_ledger: %+v", checkpoint.Details["source"], checkpoint.Details)
	}
	if checkpoint.TokensBefore <= 0 || !strings.Contains(checkpoint.Summary, "durable run record and event ledger") {
		t.Fatalf("checkpoint summary/tokens did not preserve event ledger: tokens=%d summary=%q", checkpoint.TokensBefore, checkpoint.Summary)
	}

	eventsForRun, err := s.ListEvents(ctx, source.RunID, 100)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	var completedPayload map[string]any
	for _, ev := range eventsForRun {
		if ev.Kind != types.EventRunCompactionCompleted {
			continue
		}
		if err := json.Unmarshal(ev.Payload, &completedPayload); err != nil {
			t.Fatalf("decode compaction payload: %v", err)
		}
		break
	}
	if completedPayload["source"] != "run_event_ledger" || completedPayload["entry_id"] != checkpoint.EntryID {
		t.Fatalf("compaction event payload = %+v, want event-ledger checkpoint ref %s", completedPayload, checkpoint.EntryID)
	}
}

func TestRunControlSynthesizesMissionFallbackWhenNoCandidateSignals(t *testing.T) {
	ctx := context.Background()
	rt, _ := testRuntime(t)
	rt.cfg.RunMemoryKeepRecentTokens = 1
	rt.cfg.RunMemoryContextThresholdTokens = 1
	if err := rt.InstallDefaultAgentTools(t.TempDir()); err != nil {
		t.Fatalf("install agent tools: %v", err)
	}

	source, err := rt.StartRunWithMetadata(ctx, "finish without candidates", "user-alice", map[string]any{
		runMetadataAgentProfile: AgentProfileSuper,
		runMetadataAgentRole:    AgentProfileSuper,
		"mission_doc":           "docs/mission-choir-grand-deformation-v0.md",
	})
	if err != nil {
		t.Fatalf("start source run: %v", err)
	}
	done := waitForRunTerminalState(t, rt, source.RunID, "user-alice", 5*time.Second)

	selected, err := rt.SelectSynthesizedRunContinuation(ctx, done.RunID, "user-alice")
	if err != nil {
		t.Fatalf("select synthesized fallback: %v", err)
	}
	if selected.AuthorityProfile != AgentProfileVSuper {
		t.Fatalf("authority profile = %q, want %q", selected.AuthorityProfile, AgentProfileVSuper)
	}
	if !strings.Contains(selected.Objective, "docs/mission-choir-grand-deformation-v0.md") {
		t.Fatalf("objective = %q, want mission doc fallback", selected.Objective)
	}
	if selected.Details["signal"] != "mission_gradient" || selected.Details["selection_source"] != "run_control_memory" {
		t.Fatalf("details missing run-control fallback signal: %+v", selected.Details)
	}
}

func TestRunCompletionCanAutoStartConfiguredContinuation(t *testing.T) {
	ctx := context.Background()
	rt, s := testRuntime(t)
	rt.cfg.RunMemoryKeepRecentTokens = 1
	rt.cfg.RunMemoryContextThresholdTokens = 1
	if err := rt.InstallDefaultAgentTools(t.TempDir()); err != nil {
		t.Fatalf("install agent tools: %v", err)
	}

	source, err := rt.StartRunWithMetadata(ctx, "finish and continue", "user-alice", map[string]any{
		runMetadataAgentProfile:     AgentProfileSuper,
		runMetadataAgentRole:        AgentProfileSuper,
		runMetadataContObjective:    "continue automatically in a bounded candidate worker",
		runMetadataContReason:       "configured next objective after completed run",
		runMetadataContAuthority:    AgentProfileVSuper,
		runMetadataContLeaseSeconds: 60,
		runMetadataContAutoStart:    true,
	})
	if err != nil {
		t.Fatalf("start source run: %v", err)
	}
	done := waitForRunTerminalState(t, rt, source.RunID, "user-alice", 5*time.Second)

	continuations := waitForContinuationsBySource(t, s, "user-alice", done.RunID, 5*time.Second)
	if len(continuations) != 1 {
		t.Fatalf("continuations = %d, want 1", len(continuations))
	}
	if continuations[0].Status != types.RunContinuationStarted || continuations[0].NextRunID == "" {
		t.Fatalf("auto continuation was not started: %+v", continuations[0])
	}
	child := waitForRunTerminalState(t, rt, continuations[0].NextRunID, "user-alice", 5*time.Second)
	if child.AgentProfile != AgentProfileVSuper {
		t.Fatalf("auto child profile = %q, want %q", child.AgentProfile, AgentProfileVSuper)
	}
}

func waitForContinuationsBySource(t *testing.T, s interface {
	ListRunContinuationsBySource(context.Context, string, string) ([]types.RunContinuationRecord, error)
}, ownerID, sourceRunID string, timeout time.Duration) []types.RunContinuationRecord {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		continuations, err := s.ListRunContinuationsBySource(context.Background(), ownerID, sourceRunID)
		if err != nil {
			t.Fatalf("list continuations: %v", err)
		}
		if len(continuations) > 0 &&
			continuations[0].Status == types.RunContinuationStarted &&
			continuations[0].NextRunID != "" {
			return continuations
		}
		time.Sleep(25 * time.Millisecond)
	}
	continuations, err := s.ListRunContinuationsBySource(context.Background(), ownerID, sourceRunID)
	if err != nil {
		t.Fatalf("list continuations after timeout: %v", err)
	}
	t.Fatalf("timeout waiting for continuations from %s", sourceRunID)
	return continuations
}
