package agentcore

import (
	"context"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

func testReductionScope() ReductionScope {
	return ReductionScope{
		FromAgentID: "co-super:impl",
		FromRole:    "co-super",
		ChannelID:   "chan-reduce-test",
		RunID:       "run-reduce-test",
		OwnerID:     "user-alice",
		ReturnTo:    "super:root",
		Cursor:      0,
	}
}

// testReductionCtx installs the tool execution context production always
// provides: owner, agent, run, and channel identity for durable writes.
func testReductionCtx(scope ReductionScope) context.Context {
	return toolregistry.WithExecutionContext(context.Background(), toolregistry.ExecutionContext{
		RunID:     scope.RunID,
		AgentID:   scope.FromAgentID,
		OwnerID:   scope.OwnerID,
		ChannelID: scope.ChannelID,
	})
}

// TestReduceFailedCellDropsTray is the two-phase ack gate at the reduction
// boundary: a failed cell persists nothing and the durable cursor holds, so
// unread mail is never acknowledged for work that did not happen.
func TestReduceFailedCellDropsTray(t *testing.T) {
	rt, _ := testRuntime(t)
	scope := testReductionScope()
	ctx := testReductionCtx(scope)
	intents := []yaegikernel.StagedIntent{
		{LocalID: "tray-1", Kind: yaegikernel.IntentMessage, ToDesk: "super", Body: "lost"},
	}
	receipt, err := ReduceCellIntents(ctx, rt, scope, intents, false)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Committed || len(receipt.Intents) != 0 || receipt.Cursor != scope.Cursor {
		t.Fatalf("failed reduction = %+v, want inert", receipt)
	}
	msgs, _, err := rt.ChannelRead(scope.ChannelID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 0 {
		t.Fatalf("failed cell persisted %d messages", len(msgs))
	}
	if cursor, err := LoadInboxCursor(ctx, rt.store, scope.OwnerID, scope.RunID, scope.ChannelID); err != nil || cursor != 0 {
		t.Fatalf("failed cell cursor = %d, %v", cursor, err)
	}
}

// TestReduceSuccessPersistsAndCommits proves the success path: envelopes land
// in the Dolt channel log with durable seqs, the inbox reassembles them, and
// the cursor commits to run memory.
func TestReduceSuccessPersistsAndCommits(t *testing.T) {
	rt, _ := testRuntime(t)
	scope := testReductionScope()
	ctx := testReductionCtx(scope)
	intents := []yaegikernel.StagedIntent{
		{LocalID: "tray-1", Kind: yaegikernel.IntentMessage, ToDesk: "super", MsgKind: "evidence_update", Body: "built x"},
		{LocalID: "tray-2", Kind: yaegikernel.IntentComplete, Result: yaegikernel.CompleteCompleted, Verdict: "ok", Summary: "done", EvidenceRefs: []string{"ref-1"}},
	}
	receipt, err := ReduceCellIntents(ctx, rt, scope, intents, true)
	if err != nil {
		t.Fatal(err)
	}
	if !receipt.Committed || len(receipt.Intents) != 2 {
		t.Fatalf("success receipt = %+v", receipt)
	}
	inbox, highWater, err := AssembleCellInbox(ctx, rt, scope.ChannelID, scope.Cursor)
	if err != nil {
		t.Fatal(err)
	}
	if len(inbox) != 2 || highWater == 0 {
		t.Fatalf("assembled inbox = %+v, highWater %d", inbox, highWater)
	}
	if inbox[0].Kind != "evidence_update" || inbox[0].Body != "built x" || inbox[0].ToDesk != "super" {
		t.Fatalf("message envelope decoded = %+v", inbox[0])
	}
	if inbox[1].Kind != "complete" || inbox[1].Body != "done" {
		t.Fatalf("complete envelope decoded = %+v", inbox[1])
	}
	if err := CommitInboxCursor(ctx, rt.store, scope.OwnerID, scope.RunID, scope.ChannelID, highWater); err != nil {
		t.Fatal(err)
	}
	if cursor, err := LoadInboxCursor(ctx, rt.store, scope.OwnerID, scope.RunID, scope.ChannelID); err != nil || cursor != highWater {
		t.Fatalf("committed cursor = %d, want %d (%v)", cursor, highWater, err)
	}
	// A later cell observes only newer mail: the cursor is a real fence.
	if inbox2, _, err := AssembleCellInbox(ctx, rt, scope.ChannelID, highWater); err != nil || len(inbox2) != 0 {
		t.Fatalf("post-cursor inbox = %+v, %v", inbox2, err)
	}
}

// TestReduceEnforcesTrustBoundary proves worker output is re-validated:
// quota overflow, double complete, bad verdicts, unknown kinds, and
// role-escalating spawns all fail closed with the cursor held.
func TestReduceEnforcesTrustBoundary(t *testing.T) {
	rt, _ := testRuntime(t)
	ctx := testReductionCtx(testReductionScope())
	cases := map[string]struct {
		scope   ReductionScope
		intents []yaegikernel.StagedIntent
	}{
		"quota": {testReductionScope(), make([]yaegikernel.StagedIntent, yaegikernel.MaxIntentsPerCell+1)},
		"double complete": {testReductionScope(), []yaegikernel.StagedIntent{
			{LocalID: "a", Kind: yaegikernel.IntentComplete, Result: "completed"},
			{LocalID: "b", Kind: yaegikernel.IntentComplete, Result: "failed"},
		}},
		"bad verdict": {testReductionScope(), []yaegikernel.StagedIntent{
			{LocalID: "a", Kind: yaegikernel.IntentComplete, Result: "shipped"},
		}},
		"unknown kind": {testReductionScope(), []yaegikernel.StagedIntent{
			{LocalID: "a", Kind: "teleport"},
		}},
		"researcher spawns engineering": {func() ReductionScope {
			s := testReductionScope()
			s.FromRole = "researcher"
			return s
		}(), []yaegikernel.StagedIntent{
			{LocalID: "a", Kind: yaegikernel.IntentSpawn, Role: "co-super", Objective: "escalate"},
		}},
	}
	for name, tc := range cases {
		for i := range tc.intents {
			if tc.intents[i].Kind == "" {
				tc.intents[i].Kind = yaegikernel.IntentMessage
				tc.intents[i].ToDesk = "super"
				tc.intents[i].Body = "x"
			}
			if tc.intents[i].LocalID == "" {
				tc.intents[i].LocalID = "tray-fill"
			}
		}
		receipt, err := ReduceCellIntents(ctx, rt, tc.scope, tc.intents, true)
		if err == nil {
			t.Errorf("%s: invalid tray accepted: %+v", name, receipt)
		}
		if receipt.Cursor != tc.scope.Cursor {
			t.Errorf("%s: cursor moved on rejected tray", name)
		}
	}
	// Researcher-to-researcher fan-out is legitimate.
	researcher := testReductionScope()
	researcher.FromRole = "researcher"
	receipt, err := ReduceCellIntents(ctx, rt, researcher, []yaegikernel.StagedIntent{
		{LocalID: "a", Kind: yaegikernel.IntentSpawn, Role: "researcher", Objective: "survey"},
	}, true)
	if err != nil || !receipt.Committed {
		t.Fatalf("researcher fan-out = %+v, %v", receipt, err)
	}
}

// TestReduceAddressesScopedFanIn proves workers cannot broadcast: spawn
// requests and completion reports route exclusively to the durable return
// target, while mesh messages keep their addressed desk.
func TestReduceAddressesScopedFanIn(t *testing.T) {
	rt, _ := testRuntime(t)
	scope := testReductionScope()
	ctx := testReductionCtx(scope)
	intents := []yaegikernel.StagedIntent{
		{LocalID: "tray-1", Kind: yaegikernel.IntentSpawn, Role: "researcher", Objective: "survey"},
		{LocalID: "tray-2", Kind: yaegikernel.IntentMessage, ToDesk: "co-super:peer", Body: "mesh"},
		{LocalID: "tray-3", Kind: yaegikernel.IntentComplete, Result: yaegikernel.CompleteCompleted, Summary: "done"},
	}
	if _, err := ReduceCellIntents(ctx, rt, scope, intents, true); err != nil {
		t.Fatal(err)
	}
	msgs, _, err := rt.ChannelRead(scope.ChannelID, 0)
	if err != nil || len(msgs) != 3 {
		t.Fatalf("channel = %+v, %v", msgs, err)
	}
	bySeq := map[uint64]ChannelMessage{}
	for _, m := range msgs {
		bySeq[uint64(m.Seq)] = m
	}
	if bySeq[1].ToAgentID != scope.ReturnTo || bySeq[3].ToAgentID != scope.ReturnTo {
		t.Fatalf("spawn/complete escaped return target: %+v", msgs)
	}
	if bySeq[2].ToAgentID != "co-super:peer" {
		t.Fatalf("mesh message misrouted: %+v", msgs)
	}
}
