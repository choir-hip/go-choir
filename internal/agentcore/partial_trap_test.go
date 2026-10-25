package agentcore

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// The partial trap (docs/evidence/selfdev-trace-reviews-2026-10-25.md,
// Round 2): an engineer froze a good candidate, then called
// Complete(result="partial") three times. Partial is non-terminal, the
// completion guard exhausted, the assignment was cancelled and the frozen
// candidate closed as failed before management could Verify it.
//
// Ways the fix can fail:
//  1. A partial Complete after the assignment's candidate froze is still
//     accepted, so the candidate strands again.
//  2. A partial Complete before any freeze is refused: today's meaning
//     (progress, the assignment stays open) is lost.
//  3. completed, failed or blocked after a freeze is refused: the only exits
//     are blocked.
//  4. A partial staged in the same cell as a Freeze is accepted: the freeze
//     commits first in the tray, then the partial strands it.
//  5. A candidate that is not open (still executing, or already failed)
//     blocks a partial.
//  6. The refusal leaves lifecycle state behind (a cell fate, a report, an
//     inbox cursor), so a later Complete conflicts with it.
//  7. A desk with no assignment or no operation store is refused.
//  8. The guard reminder after a committed partial does not name the cause,
//     or names it when no Complete committed, or is fooled by cell stdout
//     or a refused Complete's error text, or loses the budget line.
//  9. Re-sending an identical, already committed partial Complete errors
//     with a digest conflict instead of replaying the recorded report, or
//     the replay mints a second report.

func partialTrapOperation(t *testing.T, assignmentID string, states ...string) (*selfdev.Store, string) {
	t.Helper()
	ctx := context.Background()
	_, operations, computerID := freezeCandidateTestStore(t)
	if len(states) == 0 {
		return operations, computerID
	}
	op, err := openFreezeCandidate(ctx, operations, func(context.Context) error { return nil }, computerID, "trajectory-"+assignmentID, assignmentID, "reader")
	if err != nil {
		t.Fatal(err)
	}
	from := op.State
	for _, to := range states {
		if to == from {
			continue
		}
		if _, err := operations.Transition(ctx, computerID, op.OperationID, from, to, func(next *selfdev.Operation) error {
			if to == selfdev.StateFrozen {
				next.BundleDigest = strings.Repeat("d", 64)
			}
			return nil
		}); err != nil {
			t.Fatalf("transition %s -> %s: %v", from, to, err)
		}
		from = to
	}
	return operations, computerID
}

func completeIntent(result string) yaegikernel.StagedIntent {
	return yaegikernel.StagedIntent{Kind: yaegikernel.IntentComplete, LocalID: "tray-1", Result: result, Verdict: "none", Summary: "reader done"}
}

func TestPartialCompleteIsRefusedOnceTheAssignmentsCandidateFroze(t *testing.T) {
	ctx := context.Background()
	freeze := yaegikernel.StagedIntent{Kind: yaegikernel.IntentFreeze, LocalID: "tray-0"}
	cases := []struct {
		name    string
		states  []string
		intents []yaegikernel.StagedIntent
		refused bool
	}{
		{"frozen then partial (failure 1)", []string{selfdev.StateFrozen}, []yaegikernel.StagedIntent{completeIntent("partial")}, true},
		{"verified then partial (failure 1)", []string{selfdev.StateFrozen, selfdev.StateVerified}, []yaegikernel.StagedIntent{completeIntent("partial")}, true},
		{"no candidate, partial (failure 2)", nil, []yaegikernel.StagedIntent{completeIntent("partial")}, false},
		{"executing candidate, partial (failure 5)", []string{selfdev.StateExecuting}, []yaegikernel.StagedIntent{completeIntent("partial")}, false},
		{"failed candidate, partial (failure 5)", []string{selfdev.StateFailed}, []yaegikernel.StagedIntent{completeIntent("partial")}, false},
		{"frozen then completed (failure 3)", []string{selfdev.StateFrozen}, []yaegikernel.StagedIntent{completeIntent("completed")}, false},
		{"frozen then failed (failure 3)", []string{selfdev.StateFrozen}, []yaegikernel.StagedIntent{completeIntent("failed")}, false},
		{"frozen then blocked (failure 3)", []string{selfdev.StateFrozen}, []yaegikernel.StagedIntent{completeIntent("blocked")}, false},
		{"freeze and partial in one cell (failure 4)", nil, []yaegikernel.StagedIntent{freeze, completeIntent("partial")}, true},
		{"freeze and completed in one cell", nil, []yaegikernel.StagedIntent{freeze, completeIntent("completed")}, false},
		{"frozen, a cell with no Complete", []string{selfdev.StateFrozen}, []yaegikernel.StagedIntent{{Kind: yaegikernel.IntentNote, LocalID: "tray-1", ToDesk: "management:m", Body: "hi"}}, false},
	}
	// The checks only read, so cases in one candidate state share a store
	// (each store costs a Dolt open).
	type fixture struct {
		operations *selfdev.Store
		computerID string
	}
	fixtures := map[string]fixture{}
	for _, tc := range cases {
		if key := strings.Join(tc.states, ">"); fixtures[key].operations == nil {
			var f fixture
			f.operations, f.computerID = partialTrapOperation(t, "assignment-p", tc.states...)
			fixtures[key] = f
		}
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := fixtures[strings.Join(tc.states, ">")]
			operations, computerID := f.operations, f.computerID
			err := refusePartialCompleteAfterFreeze(ctx, operations, computerID, "assignment-p", tc.intents)
			if tc.refused {
				if err == nil {
					t.Fatal("partial Complete was accepted; the candidate would strand")
				}
				for _, want := range []string{"partial keeps the assignment open", "cannot be verified", "result=completed", "failed/blocked"} {
					if !strings.Contains(err.Error(), want) {
						t.Fatalf("refusal %q does not say %q", err, want)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("refused: %v", err)
			}
		})
	}
	// Failure 7: no store or no assignment is never refused.
	if err := refusePartialCompleteAfterFreeze(ctx, nil, "computer", "assignment-p", []yaegikernel.StagedIntent{completeIntent("partial")}); err != nil {
		t.Fatalf("no operation store: %v", err)
	}
	operations, computerID := fixtures[selfdev.StateFrozen].operations, fixtures[selfdev.StateFrozen].computerID
	if err := refusePartialCompleteAfterFreeze(ctx, operations, computerID, "", []yaegikernel.StagedIntent{completeIntent("partial")}); err != nil {
		t.Fatalf("no assignment: %v", err)
	}
	// Another assignment's frozen candidate does not count.
	if err := refusePartialCompleteAfterFreeze(ctx, operations, computerID, "assignment-other", []yaegikernel.StagedIntent{completeIntent("partial")}); err != nil {
		t.Fatalf("another assignment's candidate refused this one: %v", err)
	}
}

// Failure 6: the refusal runs before any side effect, through the real
// commit path: no cell fate, no committed receipt, no report.
func TestPartialCompleteRefusalLeavesNoLifecycleState(t *testing.T) {
	rt, st := testRuntime(t)
	operations, computerID := partialTrapOperation(t, "assignment-p", selfdev.StateFrozen)
	scope := ReductionScope{
		FromAgentID: "engineering:asg-p", DeskAgentID: "engineering:asg-p", FromRole: "engineering",
		ChannelID: "engineering:asg-p", RunID: "run-p", OwnerID: "owner-p", ComputerID: computerID,
		ReturnTo: "management:m", CellID: "cell-p",
	}
	r := &rlmCallReduction{
		active: true, mb: rt, st: st, ledger: st, scope: scope,
		rec:     &types.RunRecord{RunID: "run-p", AgentProfile: "engineering", Metadata: map[string]any{"assignment_id": "assignment-p", "assignment_attempt": 1}},
		toolCtx: &CapsuleToolCtx{ComputerID: computerID, AgentRunID: "run-p", OperationStore: operations},
	}
	err := r.commit(context.Background(), []yaegikernel.StagedIntent{completeIntent("partial")})
	if err == nil || !strings.Contains(err.Error(), "partial keeps the assignment open") {
		t.Fatalf("commit = %v, want the partial-after-freeze refusal", err)
	}
	if fate, found, ferr := CellFate(context.Background(), st, "owner-p", "run-p", "cell-p"); ferr != nil || found {
		t.Fatalf("refused cell recorded a fate %q (found %v, err %v)", fate, found, ferr)
	}
	if r.receipt.Committed || r.fateTerminal || len(r.receipt.Intents) != 0 {
		t.Fatalf("refused cell committed: %+v", r.receipt)
	}
}

func guardToolResult(t *testing.T, content string) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"role": "user", "content": []map[string]any{{"type": "tool_result", "content": content}}})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// Failure 8.
func TestCompletionGuardReminderNamesAPartialComplete(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	partialCell := guardToolResult(t, `{"stdout":"","stderr":"","exit_code":0,"staged_intent_ids":["rlm:report:0","rlm:complete:0"]}`)
	plainCell := guardToolResult(t, `{"stdout":"","stderr":"","exit_code":0,"staged_intent_ids":["rlm:report:0"]}`)
	spoofCell := guardToolResult(t, `{"stdout":"rlm:complete:0 staged_intent_ids","stderr":"","exit_code":0}`)
	refusedCell := guardToolResult(t, `tool_error: reduce: complete tray-1: rlm:complete:0 refused`)

	reason, instruction := engineeringCompletionGuardReminder(ctx, []json.RawMessage{plainCell, partialCell})
	if reason != engineeringTerminalFateRequired {
		t.Fatalf("reason = %q", reason)
	}
	for _, want := range []string{"result=partial", "keeps the assignment open", "not a terminal fate", "completed, failed or blocked", "Activation budget:"} {
		if !strings.Contains(instruction, want) {
			t.Fatalf("reminder after a partial Complete does not say %q:\n%s", want, instruction)
		}
	}
	for _, messages := range [][]json.RawMessage{nil, {plainCell}, {spoofCell}, {refusedCell}} {
		_, instruction := engineeringCompletionGuardReminder(ctx, messages)
		if strings.Contains(instruction, "result=partial") {
			t.Fatalf("reminder names a partial Complete that did not commit:\n%s", instruction)
		}
		if !strings.Contains(instruction, "Narrative text alone does not end the run") || !strings.Contains(instruction, "Activation budget:") {
			t.Fatalf("default reminder changed:\n%s", instruction)
		}
	}
}

// Failure 9: an identical partial Complete re-sent after it committed
// replays the recorded report instead of erroring with a digest conflict.
func TestIdenticalPartialCompleteReplays(t *testing.T) {
	rt, s, _ := c1eRuntime(t)
	ctx := context.Background()
	seed, err := store.SeedEngineeringAssignmentAuthority(s, "owner-partial-replay", rt.TextureComputerID(), 1)
	if err != nil {
		t.Fatal(err)
	}
	assignment, run := c1eOpenBound(t, s, seed, 0)
	partial := types.EngineeringAssignmentReport{Result: types.EngineeringResultPartial, Verdict: types.EngineeringVerdictNone, Summary: "reader done, pdf not restored"}
	first, err := rt.recordAssignedEngineeringReport(ctx, &run, "rlm-complete:run:same", partial)
	if err != nil {
		t.Fatalf("first partial Complete: %v", err)
	}
	second, err := rt.recordAssignedEngineeringReport(ctx, &run, "rlm-complete:run:same", partial)
	if err != nil {
		t.Fatalf("identical partial Complete re-sent: %v", err)
	}
	if !second.Replay || first.Report == nil || second.Report == nil || second.Report.ReportID != first.Report.ReportID {
		t.Fatalf("re-sent partial did not replay the recorded report: first %+v second %+v", first.Report, second.Report)
	}
	if recs := c1eFromEngineering(t, s, assignment); len(recs) != 1 {
		t.Fatalf("progress Reports after a replay = %d; want 1", len(recs))
	}
	// A different partial is new progress, not a replay.
	other := partial
	other.Summary = "more progress"
	if third, err := rt.recordAssignedEngineeringReport(ctx, &run, "rlm-complete:run:other", other); err != nil || third.Replay {
		t.Fatalf("a new partial = %+v, %v", third.Replay, err)
	}
}
