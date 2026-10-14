package agentcore

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Prompt-cache prefix stability
// (docs/problems/prompt-cache-breaks-across-turns-2026-10-14.md).
//
// Providers cache a prompt by its leading bytes: system prompt, tools,
// messages. Failure modes written before the code:
//   - F1 a volatile field (time, run id, agent/requester/channel ids,
//     assignment, task id, capture count, child time budget, seed prompt)
//     stays in the system prompt or the tool catalog;
//   - F2 the model loses a fact it needs because it moved nowhere;
//   - F3 a resumed activation's time turn is sent but not persisted, so
//     the next replay differs from what was sent;
//   - F5 a compaction drops the run's facts for good.

type prefixCase struct {
	name string
	a, b *types.RunRecord
	// volatile are per-run values that must not appear in the system
	// prompt; facts must appear in run A's run context.
	volatile []string
	facts    []string
}

func prefixCases() []prefixCase {
	run := func(runID, agentID, profile string, metadata map[string]any) *types.RunRecord {
		return &types.RunRecord{
			RunID: runID, AgentID: agentID, OwnerID: "owner-1", ComputerID: "vm-cache-test",
			AgentProfile: profile, AgentRole: profile, TrajectoryID: "traj-" + runID, ChannelID: "chan-" + runID,
			Metadata: metadata,
		}
	}
	return []prefixCase{
		{
			name:     "texture",
			a:        run("run-tex-a", "texture:doc-a", agentprofile.Texture, map[string]any{"doc_id": "doc-a"}),
			b:        run("run-tex-b", "texture:doc-b", agentprofile.Texture, map[string]any{"doc_id": "doc-b"}),
			volatile: []string{"run-tex-a", "texture:doc-a", "chan-run-tex-a"},
			facts:    []string{"Current agent id: texture:doc-a.", "Current coordination channel: chan-run-tex-a."},
		},
		{
			name:     "management",
			a:        run("run-man-a", "management:owner-1", agentprofile.Management, map[string]any{"requested_by_agent_id": "texture:doc-a"}),
			b:        run("run-man-b", "management:owner-1", agentprofile.Management, map[string]any{"requested_by_agent_id": "texture:doc-b"}),
			volatile: []string{"run-man-a", "texture:doc-a", "management:owner-1"},
			facts:    []string{"Requester agent id: texture:doc-a.", "Current agent id: management:owner-1."},
		},
		{
			name:     "research",
			a:        run("run-res-a", "research:owner-1", agentprofile.Research, map[string]any{"requested_by_agent_id": "texture:doc-a"}),
			b:        run("run-res-b", "research:owner-1", agentprofile.Research, map[string]any{"requested_by_agent_id": "management:owner-1"}),
			volatile: []string{"run-res-a", "texture:doc-a", "research:owner-1"},
			facts:    []string{"Requester agent id: texture:doc-a.", "Texture delivery scope: texture:doc-a."},
		},
		{
			name: "engineering",
			a: run("run-eng-a", "engineering:asg-a", agentprofile.Engineering, map[string]any{
				"assignment_id": "asg-a", "assignment_kind": string(types.EngineeringAssignmentImplementation), "subject_digest": "sha256:aaa",
			}),
			b: run("run-eng-b", "engineering:asg-b", agentprofile.Engineering, map[string]any{
				"assignment_id": "asg-b", "assignment_kind": string(types.EngineeringAssignmentImplementation), "subject_digest": "sha256:bbb",
			}),
			volatile: []string{"run-eng-a", "asg-a", "sha256:aaa"},
			facts:    []string{"assignment_id=asg-a kind=" + string(types.EngineeringAssignmentImplementation) + " subject_digest=sha256:aaa"},
		},
		{
			name:     "conductor",
			a:        run("run-con-a", "conductor:a", agentprofile.Conductor, map[string]any{"requested_app": "texture", "seed_prompt": "seed alpha"}),
			b:        run("run-con-b", "conductor:b", agentprofile.Conductor, map[string]any{"requested_app": "texture", "seed_prompt": "seed beta"}),
			volatile: []string{"seed alpha", "conductor:a"},
			facts:    []string{"Seed prompt: seed alpha.", "Requested default app: texture."},
		},
		{
			name:     "management task",
			a:        run("run-task-a", types.ManagementTaskInbox("task-a"), agentprofile.ManagementTask, map[string]any{"management_task_leased_count": 7}),
			b:        run("run-task-b", types.ManagementTaskInbox("task-b"), agentprofile.ManagementTask, map[string]any{"management_task_leased_count": 3}),
			volatile: []string{"task-a", "run-task-a"},
			facts:    []string{"You are management task task-a: management delegated 7 captures to you.", "you send as management:owner-1"},
		},
	}
}

// renderPrefix is what every provider call of the run starts with: the
// rendered system prompt (with its tool catalog) and the tool definitions.
func renderPrefix(t *testing.T, rt *Runtime, rec *types.RunRecord) (string, string) {
	t.Helper()
	system, err := rt.systemPromptForRun(rec)
	if err != nil {
		t.Fatalf("system prompt for %s: %v", rec.RunID, err)
	}
	registry := rt.toolRegistryForRun(rec)
	if registry == nil {
		return system, "[]"
	}
	defs, _ := json.Marshal(registry.Definitions())
	return toolregistry.BuildSystemPrompt(system, registry), string(defs)
}

func firstDiff(a, b string) string {
	n := min(len(a), len(b))
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			lo := max(0, i-80)
			return "at byte " + strconv.Itoa(i) + ":\nA: …" + a[lo:min(len(a), i+80)] + "\nB: …" + b[lo:min(len(b), i+80)]
		}
	}
	return "at byte " + strconv.Itoa(n) + " (one is a prefix of the other)"
}

// F1, F2: every desk's prefix is byte-identical for two different runs
// rendered at two different seconds, carries none of the run's own values,
// and the run's facts are in its run context instead.
func TestDeskPromptPrefixIsByteStableAcrossRunsAndTime(t *testing.T) {
	rt, _ := testRuntime(t)
	cases := prefixCases()
	type rendered struct{ system, tools string }
	first := make([]rendered, len(cases))
	for i, tc := range cases {
		first[i].system, first[i].tools = renderPrefix(t, rt, tc.a)
	}
	// The old timestamp was RFC3339 to the second.
	time.Sleep(1100 * time.Millisecond)
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			system, tools := renderPrefix(t, rt, tc.b)
			if system != first[i].system {
				t.Fatalf("system prompt differs between runs/times %s", firstDiff(first[i].system, system))
			}
			if tools != first[i].tools {
				t.Fatalf("tool definitions differ between runs/times %s", firstDiff(first[i].tools, tools))
			}
			for _, v := range append([]string{"Current UTC date/time", "Run context:"}, tc.volatile...) {
				if strings.Contains(system, v) {
					t.Fatalf("system prompt carries volatile %q", v)
				}
			}
			facts := runContextText(time.Now(), rt.runContextForRun(tc.a))
			for _, fact := range append([]string{"Current UTC date/time: "}, tc.facts...) {
				if !strings.Contains(facts, fact) {
					t.Fatalf("run context lacks %q:\n%s", fact, facts)
				}
			}
		})
	}
}

// B3: research children share one system prompt; each child's time budget
// is in its run-context turn.
func TestResearchChildSystemPromptIsStableAndBudgetIsAContextTurn(t *testing.T) {
	rt, _ := testRuntime(t)
	a, err := rt.researchChildSystemPrompt("owner-1")
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(1100 * time.Millisecond)
	b, err := rt.researchChildSystemPrompt("owner-1")
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Fatalf("research child system prompt differs over time %s", firstDiff(a, b))
	}
	if strings.Contains(a, "You have about") || strings.Contains(a, "Current UTC date/time") {
		t.Fatal("research child system prompt carries its time budget or the time")
	}
	turn := string(runContextMessage(time.Now(), researchChildTime(time.Now().Add(90*time.Second))))
	if !strings.Contains(turn, "You have about 1m30s for this objective") || !strings.Contains(turn, "Current UTC date/time") {
		t.Fatalf("child context turn = %s", turn)
	}
}

// F3: three activations of one run. Each activation's context starts with
// every byte the previous activation sent, then reads the time; the time
// turn is persisted, so the next replay holds it too.
func TestResumedActivationsReplayAppendOnlyWithTimeTurn(t *testing.T) {
	_, s := testRuntime(t)
	ctx := context.Background()
	now := time.Now().UTC()
	rec := types.RunRecord{
		RunID: "run-resume-cache", AgentID: "texture:doc-cache", OwnerID: "owner-1", ComputerID: "autoputer-test",
		AgentProfile: agentprofile.Texture, AgentRole: agentprofile.Texture, State: types.RunRunning, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.CreateRun(ctx, rec); err != nil {
		t.Fatal(err)
	}
	clock := now
	activation := func(reply string) []json.RawMessage {
		t.Helper()
		clock = clock.Add(7 * time.Minute)
		at := clock
		m := newRunMemoryManager(s, &rec, provideriface.Config{}, nil).withRunContext(
			func() json.RawMessage { return runContextMessage(at, "") },
			func() json.RawMessage { return runContextMessage(at, "facts") },
		)
		user, _ := json.Marshal(map[string]any{"role": "user", "content": []any{map[string]string{"type": "text", "text": "write the story"}}})
		msgs, err := m.initialize(ctx, []json.RawMessage{runContextMessage(at, "Current agent id: texture:doc-cache."), user})
		if err != nil {
			t.Fatalf("initialize: %v", err)
		}
		assistant, _ := json.Marshal(map[string]any{"role": "assistant", "content": []any{map[string]string{"type": "text", "text": reply}}})
		if err := m.afterAppendMessage(ctx, "assistant", assistant); err != nil {
			t.Fatal(err)
		}
		return append(msgs, assistant)
	}
	sent1 := activation("first")
	sent2 := activation("second")
	sent3 := activation("third")
	for i, pair := range [][2][]json.RawMessage{{sent1, sent2}, {sent2, sent3}} {
		prev, next := pair[0], pair[1]
		if len(next) != len(prev)+2 {
			t.Fatalf("activation %d: %d messages after %d, want previous + time turn + reply", i+2, len(next), len(prev))
		}
		for j := range prev {
			if string(prev[j]) != string(next[j]) {
				t.Fatalf("activation %d rewrote message %d:\nwas %s\nnow %s", i+2, j, prev[j], next[j])
			}
		}
		turn := string(next[len(prev)])
		if !strings.Contains(turn, "Run context:") || !strings.Contains(turn, "Current UTC date/time") || strings.Contains(turn, "texture:doc-cache") {
			t.Fatalf("activation %d time turn = %s", i+2, turn)
		}
	}
	if !strings.Contains(string(sent1[0]), "Current agent id: texture:doc-cache.") {
		t.Fatalf("first activation lost its run facts: %s", sent1[0])
	}
}

// F5: a compaction re-reads the run's facts and the time after the summary.
func TestCompactionReappendsRunContext(t *testing.T) {
	_, s := testRuntime(t)
	ctx := context.Background()
	now := time.Now().UTC()
	rec := types.RunRecord{
		RunID: "run-compact-cache", AgentID: "engineering:asg-c", OwnerID: "owner-1", ComputerID: "autoputer-test",
		AgentProfile: agentprofile.Engineering, AgentRole: agentprofile.Engineering, State: types.RunRunning, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.CreateRun(ctx, rec); err != nil {
		t.Fatal(err)
	}
	m := newRunMemoryManager(s, &rec, provideriface.Config{RunMemoryContextThresholdTokens: 1, RunMemoryKeepRecentTokens: 1}, func(types.EventKind, string, json.RawMessage) {}).withRunContext(
		nil,
		func() json.RawMessage {
			return runContextMessage(now, "Exact authenticated assignment: assignment_id=asg-c")
		},
	)
	big := strings.Repeat("evidence ", 400)
	var initial []json.RawMessage
	for _, role := range []string{"user", "assistant", "user", "assistant"} {
		msg, _ := json.Marshal(map[string]any{"role": role, "content": []any{map[string]string{"type": "text", "text": role + " " + big}}})
		initial = append(initial, msg)
	}
	if _, err := m.initialize(ctx, initial); err != nil {
		t.Fatal(err)
	}
	msgs, err := m.beforeProviderCall(ctx, nil)
	if err != nil {
		t.Fatalf("before provider call: %v", err)
	}
	last := string(msgs[len(msgs)-1])
	if !strings.Contains(last, "assignment_id=asg-c") || !strings.Contains(last, "Current UTC date/time") {
		t.Fatalf("compaction did not re-append the run context; last message = %s", last)
	}
	// The refresh must not defeat the guard against compacting twice in a
	// row: the next call, with nothing new but the refresh, compacts no
	// more and appends no second refresh.
	again, err := m.beforeProviderCall(ctx, nil)
	if err != nil {
		t.Fatalf("second before provider call: %v", err)
	}
	refreshes := 0
	for _, msg := range again {
		if strings.Contains(string(msg), "assignment_id=asg-c") {
			refreshes++
		}
	}
	if refreshes != 1 {
		t.Fatalf("run context appears %d times after one compaction, want 1", refreshes)
	}
	entries, err := s.ListRunMemoryEntries(ctx, rec.OwnerID, rec.RunID)
	if err != nil {
		t.Fatal(err)
	}
	compactions := 0
	for _, entry := range entries {
		if entry.Kind == types.RunMemoryEntryCompaction {
			compactions++
		}
	}
	if compactions != 1 {
		t.Fatalf("compactions = %d, want 1", compactions)
	}
}
