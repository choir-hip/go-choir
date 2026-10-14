package agentcore

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/sourceapi"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// --- Batch executor contract tests ---

func TestExecuteToolsProjectionReturnsCompactOutputAndPreservesDurableEvidence(t *testing.T) {
	registry := toolregistry.NewToolRegistry()
	if err := registry.Register(toolregistry.Tool{Name: "projected",
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			return toolregistry.ProjectionResultJSON(map[string]any{"summary": "compact result", "results": []string{"a"}},
				map[string]any{"raw": strings.Repeat("full evidence ", 20)},
				map[string]any{"type": "test_projection"})
		}}); err != nil {
		t.Fatalf("register: %v", err)
	}

	var resultPayload map[string]any
	results := toolregistry.ExecuteToolBatch(context.Background(), registry, []types.ToolCall{
		{ID: "call-projected", Name: "projected", Arguments: json.RawMessage(`{}`)},
	}, func(kind types.EventKind, phase string, payload json.RawMessage) {
		if kind != types.EventToolResult {
			return
		}
		if err := json.Unmarshal(payload, &resultPayload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
	})

	if len(results) != 1 || results[0].IsError {
		t.Fatalf("results = %#v, want one successful projected result", results)
	}
	if strings.Contains(results[0].Output, "__choir_tool_projection") || strings.Contains(results[0].Output, "full evidence") {
		t.Fatalf("model-visible output leaked envelope/full evidence: %s", results[0].Output)
	}
	if !strings.Contains(results[0].Output, "compact result") {
		t.Fatalf("model-visible output = %s, want compact result", results[0].Output)
	}
	if resultPayload["full_output_sha256"] == "" || resultPayload["full_output"] == "" {
		t.Fatalf("result payload missing durable evidence fields: %#v", resultPayload)
	}
	if got := resultPayload["output"]; got != results[0].Output {
		t.Fatalf("event output = %#v, want model output %q", got, results[0].Output)
	}
	projection, ok := resultPayload["output_projection"].(map[string]any)
	if !ok || projection["type"] != "test_projection" {
		t.Fatalf("output_projection = %#v, want test_projection", resultPayload["output_projection"])
	}
}

func testSourceAPIItem() sourceapi.ItemResult {
	return sourceapi.ItemResult{
		Rank:            1,
		TargetKind:      sourceapi.TargetKind,
		ItemID:          "srcitem_test_rates",
		SourceID:        "official:test",
		SourceType:      "rss",
		FetchID:         "fetch_test_rates",
		OriginalID:      "release-1",
		Title:           "Rate decision",
		Body:            "Rates held steady.",
		URL:             "https://example.test/release-1",
		CanonicalURL:    "https://example.test/release-1",
		PublishedAt:     "2026-06-04T12:00:00Z",
		FetchedAt:       "2026-06-04T12:01:00Z",
		Verticals:       []string{"macro_policy"},
		Language:        "en",
		Region:          "us",
		ContentHash:     "sha256-test-rates",
		BodyKind:        "reader_snapshot",
		BodyLength:      len("Rates held steady."),
		ReaderSnapshot:  true,
		EvidenceLevel:   "official_release",
		VintagePolicy:   "release_snapshot",
		LookaheadStatus: "no_lookahead",
		ReleaseDate:     "2026-06-04",
	}
}

func terminalResearchRunFixture(runID, ownerID, docID, result string, state types.RunState, now time.Time) types.RunRecord {
	finishedAt := now
	rec := types.RunRecord{
		RunID:            runID,
		AgentID:          "research:" + runID,
		RequestedByRunID: "texture-parent:" + runID,
		ChannelID:        docID,
		OwnerID:          ownerID,
		AgentProfile:     agentprofile.Research,
		AgentRole:        agentprofile.Research,
		State:            state,
		Result:           result,
		CreatedAt:        now,
		UpdatedAt:        now,
		Metadata: map[string]any{
			runMetadataAgentProfile: agentprofile.Research,
			runMetadataAgentRole:    agentprofile.Research,
			runMetadataAgentID:      "research:" + runID,
			runMetadataChannelID:    docID,
			"requested_by_profile":  agentprofile.Texture,
			"requested_by_agent_id": "texture:" + docID,
		},
	}
	if state.Terminal() {
		rec.FinishedAt = &finishedAt
	}
	return rec
}

func TestRootTerminalRunSkipsOutcomeBindingWithoutStoreLookup(t *testing.T) {
	rt, _ := testRuntime(t)
	root := types.RunRecord{
		RunID:        "unpersisted-root-terminal",
		AgentID:      "engineering:unpersisted-root-terminal",
		AgentProfile: agentprofile.Engineering,
		AgentRole:    agentprofile.Engineering,
		State:        types.RunCompleted,
		Result:       "Root result has no requester.",
	}
	if err := rt.bindTerminalRunOutcome(context.Background(), &root, false); err != nil {
		t.Fatalf("root terminal run performed a binding store lookup: %v", err)
	}
}

func TestFallbackAbstainsOnAssignmentRun(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	now := time.Now().UTC()
	rec := terminalResearchRunFixture(
		"owner",
		"computer",
		"run-assigned-child",
		"run-parent-requester",
		"channel-1",
		now,
	)
	rec.Metadata["assignment_id"] = "assignment-xyz"
	if err := s.CreateRun(ctx, rec); err != nil {
		t.Fatal(err)
	}
	if err := rt.bindTerminalRunOutcome(ctx, &rec, false); err != nil {
		t.Fatalf("bind terminal outcome on assignment run: %v", err)
	}
	updates, err := s.ListPendingWorkerUpdates(ctx, rec.OwnerID, "texture:"+rec.ChannelID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(updates) != 0 {
		t.Fatalf("fallback synthesized updates for assignment run: %+v", updates)
	}
}

func TestFallbackLeavesAssignedTerminalRunToFateReconcile(t *testing.T) {
	rt, s := testRuntime(t)
	ctx := context.Background()
	now := time.Now().UTC()
	seed, err := store.SeedEngineeringAssignmentAuthority(s, "owner-orphan-prod", rt.TextureComputerID(), 1)
	if err != nil {
		t.Fatal(err)
	}
	open := openBoundSeedAssignment(t, s, seed, 0, "opaque-orphan-prod")
	assignmentID, runID := open.AssignmentID, seed.AssignedRunIDs[0]
	persisted, err := s.GetLifecycleRun(ctx, seed.OwnerID, seed.ComputerID, runID)
	if err != nil {
		t.Fatal(err)
	}
	persisted.State, persisted.Result, persisted.UpdatedAt, persisted.FinishedAt =
		types.RunCompleted, "child exited without terminal packet", now, &now
	if err := s.UpdateRun(ctx, persisted); err != nil {
		t.Fatal(err)
	}
	probe := types.RunRecord{
		RunID: runID, OwnerID: seed.OwnerID, RequestedByRunID: seed.ParentRunID,
		AgentProfile: agentprofile.Engineering,
	}
	if err := rt.bindTerminalRunOutcome(ctx, &probe, false); err != nil {
		t.Fatalf("fallback on assignment run: %v", err)
	}
	// The fallback is no longer a fate authority for assignment runs: the
	// assignment stays bound until the trajectory/desk reconcile owns its
	// closure (recast for restart-class deaths, op-fail for real failures).
	// Asserting terminal here would re-bless the second-authority shape the
	// wedge cluster proved unsafe.
	assignment, err := s.GetEngineeringAssignment(ctx, seed.OwnerID, seed.ComputerID, assignmentID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if assignment.Disposition.Terminal() {
		t.Fatalf("assignment disposition = %v, want non-terminal (fate reconcile owns closure)", assignment.Disposition)
	}
	updates, err := s.ListPendingWorkerUpdates(ctx, seed.OwnerID, "texture:"+open.Binding.AssignedAgentID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(updates) != 0 {
		t.Fatalf("fallback synthesized updates for assignment run: %+v", updates)
	}
}

func TestResearchPlainTerminalResultBindsAddressedOutcome(t *testing.T) {
	for _, tc := range []struct {
		name   string
		marker func(map[string]any)
	}{
		{
			name: "plural legacy marker",
			marker: func(metadata map[string]any) {
				metadata["work_item_ids"] = []string{"legacy-work-item-researcher-plain-terminal"}
			},
		},
		{
			name: "singular legacy marker",
			marker: func(metadata map[string]any) {
				metadata["lifecycle_work_item_id"] = "legacy-work-item-researcher-plain-terminal"
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			rt, s := testRuntime(t)
			now := time.Now().UTC()
			suffix := strings.ReplaceAll(tc.name, " ", "-")
			rec := terminalResearchRunFixture(
				"run-researcher-plain-terminal-"+suffix,
				"owner-researcher-plain-terminal-"+suffix,
				"doc-researcher-plain-terminal-"+suffix,
				"Plain terminal research result without any research-tool event.",
				types.RunCompleted,
				now,
			)
			rec.ComputerID = rt.TextureComputerID()
			tc.marker(rec.Metadata)
			if err := s.CreateRun(ctx, rec); err != nil {
				t.Fatalf("create terminal researcher run: %v", err)
			}

			if err := rt.bindTerminalRunOutcome(ctx, &rec, false); err != nil {
				t.Fatalf("bind terminal researcher outcome: %v", err)
			}
			updates, err := s.ListPendingWorkerUpdates(ctx, rec.OwnerID, "texture:"+rec.ChannelID, 10)
			if err != nil {
				t.Fatalf("list bound terminal outcomes: %v", err)
			}
			if len(updates) != 1 {
				t.Fatalf("bound updates = %d, want exactly one: %+v", len(updates), updates)
			}
			update := updates[0]
			if update.SourceRunID != rec.RunID {
				t.Fatalf("source_run_id = %q, want %q", update.SourceRunID, rec.RunID)
			}
			wantDigest := types.TerminalRunOutcomeSHA256(rec.RunID, rec.State, rec.Result, rec.Error)
			if update.SourceOutcomeSHA256 != wantDigest {
				t.Fatalf("source_outcome_sha256 = %q, want recomputed %q", update.SourceOutcomeSHA256, wantDigest)
			}
			if strings.Contains(update.Content, rec.Result) {
				t.Fatalf("persisted reference envelope copied authoritative result: %q", update.Content)
			}
			projected, err := rt.projectTerminalOutcomeContent(ctx, updates)
			if err != nil {
				t.Fatalf("project authoritative terminal result: %v", err)
			}
			if len(projected) != 1 || !strings.Contains(projected[0].Content, rec.Result) {
				t.Fatalf("delivery projection omitted authoritative result: %+v", projected)
			}
			tampered := append([]types.CoagentSourcePacket(nil), updates...)
			tampered[0].SourceOutcomeSHA256 = "tampered"
			if _, err := rt.projectTerminalOutcomeContent(ctx, tampered); err == nil {
				t.Fatal("tampered terminal outcome digest projected without error")
			}
			persisted, err := s.GetWorkerUpdate(ctx, rec.OwnerID, update.UpdateID)
			if err != nil {
				t.Fatalf("reload terminal outcome reference: %v", err)
			}
			if strings.Contains(persisted.Content, rec.Result) {
				t.Fatalf("delivery projection mutated persisted envelope: %q", persisted.Content)
			}
		})
	}
}

func TestResearchTerminalOutcomeRequiresDurableTerminalRun(t *testing.T) {
	ctx := context.Background()
	rt, s := testRuntime(t)
	now := time.Now().UTC()
	stored := terminalResearchRunFixture(
		"run-researcher-terminal-order",
		"owner-researcher-terminal-order",
		"doc-researcher-terminal-order",
		"",
		types.RunRunning,
		now,
	)
	if err := s.CreateRun(ctx, stored); err != nil {
		t.Fatalf("create running researcher run: %v", err)
	}
	inMemoryTerminal := stored
	inMemoryTerminal.State = types.RunCompleted
	inMemoryTerminal.Result = "terminal result not durable yet"
	inMemoryTerminal.UpdatedAt = now.Add(time.Second)
	inMemoryTerminal.FinishedAt = &inMemoryTerminal.UpdatedAt

	if err := rt.bindTerminalRunOutcome(ctx, &inMemoryTerminal, false); err != nil {
		t.Fatalf("probe pre-persistence binding: %v", err)
	}
	updates, err := s.ListPendingWorkerUpdates(ctx, stored.OwnerID, "texture:"+stored.ChannelID, 10)
	if err != nil {
		t.Fatalf("list pre-persistence outcomes: %v", err)
	}
	if len(updates) != 0 {
		t.Fatalf("bound %d outcome(s) before terminal RunRecord persistence: %+v", len(updates), updates)
	}

	if err := s.UpdateRun(ctx, inMemoryTerminal); err != nil {
		t.Fatalf("persist terminal researcher run: %v", err)
	}
	if err := rt.bindTerminalRunOutcome(ctx, &inMemoryTerminal, false); err != nil {
		t.Fatalf("bind persisted terminal researcher outcome: %v", err)
	}
	updates, err = s.ListPendingWorkerUpdates(ctx, stored.OwnerID, "texture:"+stored.ChannelID, 10)
	if err != nil {
		t.Fatalf("list post-persistence outcomes: %v", err)
	}
	if len(updates) != 1 {
		t.Fatalf("bound outcomes after persistence = %d, want one: %+v", len(updates), updates)
	}
}

func TestDerivedWorkerUpdateIDSurvivesReplaceableSourceRun(t *testing.T) {
	base := types.CoagentSourcePacket{
		OwnerID:       "owner-update-id",
		AgentID:       "research:update-id",
		TargetAgentID: "texture:update-id",
		ChannelID:     "doc-update-id",
		Role:          agentprofile.Research,
		Packet:        newCoagentPacket("evidence_update", "Same content.", nil, nil, nil, nil, nil),
	}
	first := base
	first.SourceRunID = "run-update-id-first"
	second := base
	second.SourceRunID = "run-update-id-second"

	if firstID, secondID := deriveWorkerUpdateID(first), deriveWorkerUpdateID(second); firstID != secondID {
		t.Fatalf("replaceable producer runs changed durable update ID: %q != %q", firstID, secondID)
	}
}

func TestManagementExplicitFinalUpdateIsBoundInPlace(t *testing.T) {
	ctx := context.Background()
	rt, s := testRuntime(t)
	now := time.Now().UTC()
	rec := types.RunRecord{
		RunID:            "run-super-explicit-terminal",
		AgentID:          "management:explicit-terminal",
		RequestedByRunID: "run-super-explicit-parent",
		ChannelID:        "doc-super-explicit-terminal",
		OwnerID:          "owner-super-explicit-terminal",
		AgentProfile:     agentprofile.Management,
		AgentRole:        agentprofile.Management,
		State:            types.RunRunning,
		CreatedAt:        now,
		UpdatedAt:        now,
		Metadata: map[string]any{
			runMetadataAgentProfile: agentprofile.Management,
			runMetadataAgentRole:    agentprofile.Management,
			runMetadataAgentID:      "management:explicit-terminal",
			runMetadataChannelID:    "doc-super-explicit-terminal",
			runMetadataTrajectoryID: "trajectory-super-explicit-terminal",
			"requested_by_agent_id": "engineering:explicit-terminal",
		},
	}
	if err := s.CreateRun(ctx, rec); err != nil {
		t.Fatalf("create running Management run: %v", err)
	}
	update := types.CoagentSourcePacket{
		OwnerID:       rec.OwnerID,
		SourceRunID:   rec.RunID,
		AgentID:       rec.AgentID,
		TargetAgentID: "engineering:explicit-terminal",
		ChannelID:     rec.ChannelID,
		TrajectoryID:  trajectoryIDForRun(&rec),
		Role:          agentprofile.Management,
		Packet: newCoagentPacket(
			"evidence_update",
			"Explicit final Management update.",
			[]types.CoagentPacketClaim{coagentClaim("Management terminal evidence.")},
			nil,
			nil,
			nil,
			nil,
		),
		CreatedAt: now,
	}
	update.UpdateID = deriveWorkerUpdateID(update)
	update.Content = buildWorkerUpdateMessage(update)
	message := &types.ChannelMessage{
		ChannelID:    update.ChannelID,
		From:         rec.RunID,
		FromAgentID:  rec.AgentID,
		FromRunID:    rec.RunID,
		ToAgentID:    update.TargetAgentID,
		TrajectoryID: update.TrajectoryID,
		Role:         update.Role,
		Content:      update.Content,
		Timestamp:    now,
	}
	storedUpdate, created, err := s.DispatchWorkerUpdate(ctx, update, message)
	if err != nil || !created {
		t.Fatalf("dispatch explicit Management update: created=%t err=%v", created, err)
	}
	output, _ := json.Marshal(map[string]any{
		"update_id":     storedUpdate.UpdateID,
		"agent_id":      storedUpdate.TargetAgentID,
		"channel_id":    storedUpdate.ChannelID,
		"trajectory_id": storedUpdate.TrajectoryID,
		"status":        "submitted",
	})
	eventPayload, _ := json.Marshal(map[string]any{
		"tool":     "update_coagent",
		"call_id":  "call-super-explicit-terminal",
		"is_error": false,
		"output":   string(output),
	})
	if err := s.AppendEvent(ctx, &types.EventRecord{
		EventID:      "event-super-explicit-terminal",
		RunID:        rec.RunID,
		AgentID:      rec.AgentID,
		ChannelID:    rec.ChannelID,
		OwnerID:      rec.OwnerID,
		TrajectoryID: rec.TrajectoryID,
		Timestamp:    now,
		Kind:         types.EventToolResult,
		Payload:      eventPayload,
	}); err != nil {
		t.Fatalf("append explicit Management update event: %v", err)
	}
	rec.State = types.RunCompleted
	rec.Result = "Management completed after its explicit terminal update."
	rec.UpdatedAt = now.Add(time.Second)
	rec.FinishedAt = &rec.UpdatedAt
	if err := s.UpdateRun(ctx, rec); err != nil {
		t.Fatalf("persist terminal Management run: %v", err)
	}
	if err := rt.bindTerminalRunOutcome(ctx, &rec, false); err != nil {
		t.Fatalf("bind explicit Management terminal update: %v", err)
	}
	bound, err := s.GetWorkerUpdate(ctx, rec.OwnerID, update.UpdateID)
	if err != nil {
		t.Fatalf("get bound Management update: %v", err)
	}
	wantDigest := types.TerminalRunOutcomeSHA256(rec.RunID, rec.State, rec.Result, rec.Error)
	if bound.SourceRunID != rec.RunID || bound.SourceOutcomeSHA256 != wantDigest || bound.MessageSeq != storedUpdate.MessageSeq {
		t.Fatalf("bound Management update = %+v, want run %q digest %q seq %d", bound, rec.RunID, wantDigest, storedUpdate.MessageSeq)
	}
}

func TestTerminalOutcomeIgnoresExplicitUpdateToNonRequester(t *testing.T) {
	ctx := context.Background()
	rt, s := testRuntime(t)
	now := time.Now().UTC()
	rec := terminalResearchRunFixture(
		"run-terminal-nonrequester",
		"owner-terminal-nonrequester",
		"doc-terminal-nonrequester",
		"Authoritative result owed to the requesting Texture.",
		types.RunCompleted,
		now,
	)
	if err := s.CreateRun(ctx, rec); err != nil {
		t.Fatalf("create terminal child run: %v", err)
	}
	unrelated := types.CoagentSourcePacket{
		OwnerID:       rec.OwnerID,
		AgentID:       rec.AgentID,
		TargetAgentID: "engineering:unrelated-recipient",
		ChannelID:     rec.ChannelID,
		TrajectoryID:  trajectoryIDForRun(&rec),
		Role:          agentprofile.Research,
		SourceRunID:   rec.RunID,
		Packet:        newCoagentPacket("evidence_update", "Update for another actor.", nil, nil, nil, nil, nil),
		CreatedAt:     now,
	}
	unrelated.UpdateID = deriveWorkerUpdateID(unrelated)
	unrelated.Content = buildWorkerUpdateMessage(unrelated)
	if _, created, err := s.DispatchWorkerUpdate(ctx, unrelated, &types.ChannelMessage{
		ChannelID:    unrelated.ChannelID,
		From:         rec.RunID,
		FromAgentID:  rec.AgentID,
		FromRunID:    rec.RunID,
		ToAgentID:    unrelated.TargetAgentID,
		TrajectoryID: unrelated.TrajectoryID,
		Role:         unrelated.Role,
		Content:      unrelated.Content,
		Timestamp:    now,
	}); err != nil || !created {
		t.Fatalf("dispatch unrelated update: created=%t err=%v", created, err)
	}

	if err := rt.bindTerminalRunOutcome(ctx, &rec, false); err != nil {
		t.Fatalf("bind requester terminal outcome: %v", err)
	}
	requesterUpdates, err := s.ListPendingWorkerUpdates(ctx, rec.OwnerID, "texture:"+rec.ChannelID, 10)
	if err != nil {
		t.Fatalf("list requester terminal outcomes: %v", err)
	}
	if len(requesterUpdates) != 1 || requesterUpdates[0].TargetAgentID != "texture:"+rec.ChannelID {
		t.Fatalf("requester outcomes = %+v, want exactly one addressed synthetic outcome", requesterUpdates)
	}
	if requesterUpdates[0].SourceOutcomeSHA256 == "" {
		t.Fatalf("requester outcome lacks terminal digest: %+v", requesterUpdates[0])
	}
	storedUnrelated, err := s.GetWorkerUpdate(ctx, rec.OwnerID, unrelated.UpdateID)
	if err != nil {
		t.Fatalf("reload unrelated update: %v", err)
	}
	if storedUnrelated.SourceOutcomeSHA256 != "" {
		t.Fatalf("non-requester update was bound as terminal outcome: %+v", storedUnrelated)
	}
}

func TestExecuteToolsSerializesHeavySideEffectTurns(t *testing.T) {
	registry := toolregistry.NewToolRegistry()
	var mu sync.Mutex
	var order []string
	register := func(name string) {
		t.Helper()
		if err := registry.Register(toolregistry.Tool{Name: name,
			Func: func(ctx context.Context, args json.RawMessage) (string, error) {
				mu.Lock()
				order = append(order, "start:"+name)
				mu.Unlock()
				time.Sleep(10 * time.Millisecond)
				mu.Lock()
				order = append(order, "end:"+name)
				mu.Unlock()
				return name + "-result", nil
			}}); err != nil {
			t.Fatalf("register %s: %v", name, err)
		}
	}
	register("desk_go_eval")
	register("read_file")

	results := toolregistry.ExecuteToolBatch(context.Background(), registry, []types.ToolCall{
		{ID: "call-1", Name: "desk_go_eval", Arguments: json.RawMessage(`{}`)},
		{ID: "call-2", Name: "read_file", Arguments: json.RawMessage(`{}`)},
	}, func(kind types.EventKind, phase string, payload json.RawMessage) {})

	if results[0].Output != "desk_go_eval-result" || results[1].Output != "read_file-result" {
		t.Fatalf("results = %+v, want both tool outputs", results)
	}
	want := []string{"start:desk_go_eval", "end:desk_go_eval", "start:read_file", "end:read_file"}
	if strings.Join(order, ",") != strings.Join(want, ",") {
		t.Fatalf("execution order = %v, want %v", order, want)
	}
}

func TestExecuteToolsOutputTruncation(t *testing.T) {
	registry := toolregistry.NewToolRegistry()

	bigTool := toolregistry.Tool{Name: "big_output",
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			// Return output larger than 100KB.
			result := make([]byte, 150*1024)
			for i := range result {
				result[i] = 'x'
			}
			return string(result), nil
		}}
	if err := registry.Register(bigTool); err != nil {
		t.Fatalf("register: %v", err)
	}

	calls := []types.ToolCall{
		{ID: "call-1", Name: "big_output", Arguments: json.RawMessage(`{}`)},
	}

	emit := func(kind types.EventKind, phase string, payload json.RawMessage) {}

	results := toolregistry.ExecuteToolBatch(context.Background(), registry, calls, emit)

	// Output should be truncated to ~100KB + truncation notice.
	if len(results[0].Output) > 110*1024 {
		t.Errorf("output should be truncated, got %d bytes", len(results[0].Output))
	}
}

func TestExecuteToolsDoesNotCapResearchSearchBatch(t *testing.T) {
	registry := toolregistry.NewToolRegistry()
	var searches int32
	if err := registry.Register(toolregistry.Tool{Name: "web_search",
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			atomic.AddInt32(&searches, 1)
			return "search result", nil
		}}); err != nil {
		t.Fatalf("register web_search: %v", err)
	}
	if err := registry.Register(toolregistry.Tool{Name: "fetch_url",
		Func: func(ctx context.Context, args json.RawMessage) (string, error) {
			return "fetch result", nil
		}}); err != nil {
		t.Fatalf("register fetch_url: %v", err)
	}

	ctx := toolregistry.WithExecutionContext(context.Background(), toolExecutionContextForRun(&types.RunRecord{
		RunID:        "researcher-run",
		OwnerID:      "owner",
		AgentProfile: agentprofile.Research,
	}))
	calls := []types.ToolCall{
		{ID: "search-1", Name: "web_search", Arguments: json.RawMessage(`{"query":"ai news may 2026"}`)},
		{ID: "search-2", Name: "web_search", Arguments: json.RawMessage(`{"query":"openai may 2026"}`)},
		{ID: "search-3", Name: "web_search", Arguments: json.RawMessage(`{"query":"google ai may 2026"}`)},
	}

	results := toolregistry.ExecuteToolBatch(ctx, registry, calls, func(kind types.EventKind, phase string, payload json.RawMessage) {})

	if got := atomic.LoadInt32(&searches); got != 3 {
		t.Fatalf("searches = %d, want 3", got)
	}
	for idx, result := range results {
		if result.IsError {
			t.Fatalf("result[%d] = %#v, want success", idx, result)
		}
	}
}
