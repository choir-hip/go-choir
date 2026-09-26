package agentcore

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

func testReductionScope() ReductionScope {
	return ReductionScope{
		FromAgentID: "engineering:impl",
		FromRole:    "engineering",
		ChannelID:   "chan-reduce-test",
		RunID:       "run-reduce-test",
		OwnerID:     "user-alice",
		ReturnTo:    "management:root",
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

func TestCommitFreezeIntentRejectsDocumentTrajectoryBeforeExecutorEffect(t *testing.T) {
	rt, _ := testRuntime(t)
	rec := &types.RunRecord{
		RunID: "run-document-freeze", OwnerID: "user-alice", ComputerID: "autoputer-test", TrajectoryID: "document-trajectory",
		Metadata: map[string]any{
			"assignment_id": "assignment-document-freeze", "assignment_attempt": 1,
			"assignment_kind": string(types.EngineeringAssignmentImplementation),
		},
	}
	toolCtx := &CapsuleToolCtx{
		Executor: new(capsule.Executor), AgentRunID: rec.RunID, ComputerID: rec.ComputerID,
		Role: capsule.RoleEngineering, CapsuleHandle: "bound-handle", OperationStore: rt.selfdevOperations,
		ValidateCurrentObligation: func(context.Context) error { return nil },
	}
	ctx := WithCapsuleCtx(context.Background(), toolCtx)
	ctx = toolregistry.WithExecutionContext(ctx, toolregistry.ExecutionContext{RunID: rec.RunID, RunRecord: rec})
	reduction := &rlmCallReduction{rec: rec, toolCtx: toolCtx}

	_, err := reduction.commitFreezeIntent(ctx, yaegikernel.StagedIntent{Kind: yaegikernel.IntentFreeze})
	if err == nil || !strings.Contains(err.Error(), "resolve self-development operation") {
		t.Fatalf("document trajectory freeze error = %v, want self-development operation refusal", err)
	}
}

// TestReduceFailedCellDropsTray is the two-phase ack gate at the reduction
// boundary: a failed cell persists nothing and the durable cursor holds, so
// unread mail is never acknowledged for work that did not happen.
func TestReduceFailedCellDropsTray(t *testing.T) {
	rt, _ := testRuntime(t)
	scope := testReductionScope()
	ctx := testReductionCtx(scope)
	intents := []yaegikernel.StagedIntent{
		{LocalID: "tray-1", Kind: yaegikernel.IntentMessage, ToDesk: "management", Body: "lost"},
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
		{LocalID: "tray-1", Kind: yaegikernel.IntentMessage, ToDesk: "management", MsgKind: "evidence_update", Body: "built x"},
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
	if inbox[0].Kind != "evidence_update" || inbox[0].Body != "built x" || inbox[0].ToDesk != "management" {
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
			s.FromRole = "research"
			return s
		}(), []yaegikernel.StagedIntent{
			{LocalID: "a", Kind: yaegikernel.IntentSpawn, Role: "engineering", Objective: "escalate"},
		}},
		"complete not last": {testReductionScope(), []yaegikernel.StagedIntent{
			{LocalID: "a", Kind: yaegikernel.IntentComplete, Result: "completed"},
			{LocalID: "b", Kind: yaegikernel.IntentMessage, ToDesk: "management", Body: "x"},
		}},
		"verify missing digest": {testReductionScope(), []yaegikernel.StagedIntent{
			{LocalID: "a", Kind: yaegikernel.IntentVerify, Decision: "pass", VerifierRefs: []string{"ref"}},
		}},
	}
	for name, tc := range cases {
		for i := range tc.intents {
			if tc.intents[i].Kind == "" {
				tc.intents[i].Kind = yaegikernel.IntentMessage
				tc.intents[i].ToDesk = "management"
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
	// Research-to-researcher fan-out is legitimate.
	research := testReductionScope()
	research.FromRole = "research"
	receipt, err := ReduceCellIntents(ctx, rt, research, []yaegikernel.StagedIntent{
		{LocalID: "a", Kind: yaegikernel.IntentSpawn, Role: "research", Objective: "survey"},
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
		{LocalID: "tray-1", Kind: yaegikernel.IntentSpawn, Role: "research", Objective: "survey"},
		{LocalID: "tray-2", Kind: yaegikernel.IntentMessage, ToDesk: "engineering:peer", Body: "mesh"},
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
	if bySeq[2].ToAgentID != "engineering:peer" {
		t.Fatalf("mesh message misrouted: %+v", msgs)
	}
}

func TestCommitAdvancesOnlyInboxHighWater(t *testing.T) {
	rt, _ := testRuntime(t)
	scope := testReductionScope()
	ctx := testReductionCtx(scope)

	if _, err := rt.ChannelCast(ctx, scope.ChannelID, scope.FromAgentID, "", "management", "management", "snapshot-mail"); err != nil {
		t.Fatal(err)
	}
	reduction := &rlmCallReduction{
		active:    true,
		mb:        rt,
		st:        rt.store,
		scope:     scope,
		highWater: 1,
	}
	if _, err := rt.ChannelCast(ctx, scope.ChannelID, scope.FromAgentID, "", "peer", "research", "late-inbound"); err != nil {
		t.Fatal(err)
	}
	if err := reduction.commit(ctx, []yaegikernel.StagedIntent{
		{LocalID: "tray-1", Kind: yaegikernel.IntentMessage, ToDesk: "management", Body: "outbound"},
	}); err != nil {
		t.Fatal(err)
	}
	cursor, err := LoadInboxCursor(ctx, rt.store, scope.OwnerID, scope.RunID, scope.ChannelID)
	if err != nil {
		t.Fatal(err)
	}
	if cursor != 1 {
		t.Fatalf("cursor = %d, want snapshot high-water 1 (must not skip concurrent inbound)", cursor)
	}
	inbox, _, err := AssembleCellInbox(ctx, rt, scope.ChannelID, cursor)
	if err != nil {
		t.Fatal(err)
	}
	if len(inbox) < 1 {
		t.Fatal("late inbound must remain visible after commit")
	}
	foundLate := false
	for _, msg := range inbox {
		if msg.Body == "late-inbound" || msg.FromDesk != "" && msg.Kind == "channel" {
			if msg.Body == "late-inbound" {
				foundLate = true
			}
		}
	}
	for _, msg := range inbox {
		if msg.Body == "late-inbound" {
			foundLate = true
		}
	}
	if !foundLate {
		t.Fatalf("late inbound dropped from next inbox: %+v", inbox)
	}
}

func TestCommitRecoversPartialActTray(t *testing.T) {
	rt, _ := testRuntime(t)
	scope := testReductionScope()
	scope.CellID = "cell-partial-act-recovery"
	scope.ComputerID = rt.TextureComputerID()
	ctx := testReductionCtx(scope)
	if _, err := rt.ChannelCast(ctx, scope.ChannelID, scope.FromAgentID, "", "management", "management", "snapshot-mail"); err != nil {
		t.Fatal(err)
	}
	intent := yaegikernel.StagedIntent{LocalID: "ask-1", Kind: yaegikernel.IntentAsk, ToDesk: "management", Question: "is the cast ready?"}

	// Model a process death after the append-only ledger mint and before the
	// envelope/cursor writes.
	if _, err := rt.store.AppendCommitmentRecord(ctx, scope.OwnerID, scope.ComputerID, commitmentRecordForIntent(scope, intent)); err != nil {
		t.Fatalf("stage commitment record: %v", err)
	}
	reduction := &rlmCallReduction{
		active: true, mb: rt, st: rt.store, ledger: rt.store, scope: scope, highWater: 1,
	}
	if err := reduction.commit(ctx, []yaegikernel.StagedIntent{intent}); err != nil {
		t.Fatalf("recover partial act commit: %v", err)
	}
	if !reduction.receipt.Committed || reduction.receipt.Cursor != 1 {
		t.Fatalf("recovery receipt = %+v", reduction.receipt)
	}
	if cursor, err := LoadInboxCursor(ctx, rt.store, scope.OwnerID, scope.RunID, scope.ChannelID); err != nil || cursor != 1 {
		t.Fatalf("recovered cursor = %d, %v; want 1", cursor, err)
	}
	to, content, mailed, err := stagedIntentEnvelope(scope, intent)
	if err != nil || !mailed {
		t.Fatalf("recovery envelope = (%q, %q, %t, %v)", to, content, mailed, err)
	}
	key := intentIdempotencyKey(scope, intent.LocalID, to, content)
	messages, _, err := rt.ChannelRead(scope.ChannelID, 0)
	if err != nil {
		t.Fatal(err)
	}
	matched := 0
	for _, message := range messages {
		if message.IdempotencyKey == key {
			matched++
		}
	}
	if matched != 1 {
		t.Fatalf("recovered envelope count = %d, want 1", matched)
	}

	// Retrying the same post-crash tray converges without duplicate mail.
	if err := reduction.commit(ctx, []yaegikernel.StagedIntent{intent}); err != nil {
		t.Fatalf("replay recovered tray: %v", err)
	}
	messages, _, err = rt.ChannelRead(scope.ChannelID, 0)
	if err != nil {
		t.Fatal(err)
	}
	matched = 0
	for _, message := range messages {
		if message.IdempotencyKey == key {
			matched++
		}
	}
	if matched != 1 {
		t.Fatalf("replayed recovered envelope count = %d, want 1", matched)
	}
}

func TestCommitRecoversUnmailedActWithoutInboxAdvance(t *testing.T) {
	rt, _ := testRuntime(t)
	scope := testReductionScope()
	scope.CellID = "cell-unmailed-act-recovery"
	scope.ComputerID = rt.TextureComputerID()
	ctx := testReductionCtx(scope)
	intent := yaegikernel.StagedIntent{LocalID: "note-1", Kind: yaegikernel.IntentNote, ToDesk: "management", Body: "recover this mail"}

	if _, err := rt.store.AppendCommitmentRecord(ctx, scope.OwnerID, scope.ComputerID, commitmentRecordForIntent(scope, intent)); err != nil {
		t.Fatalf("stage commitment record: %v", err)
	}
	reduction := &rlmCallReduction{active: true, mb: rt, st: rt.store, ledger: rt.store, scope: scope}
	if err := reduction.commit(ctx, []yaegikernel.StagedIntent{intent}); err != nil {
		t.Fatalf("recover unmailed act: %v", err)
	}
	to, content, mailed, err := stagedIntentEnvelope(scope, intent)
	if err != nil || !mailed {
		t.Fatalf("recovery envelope = (%q, %q, %t, %v)", to, content, mailed, err)
	}
	key := intentIdempotencyKey(scope, intent.LocalID, to, content)
	messages, _, err := rt.ChannelRead(scope.ChannelID, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, message := range messages {
		if message.IdempotencyKey == key {
			return
		}
	}
	t.Fatal("partial act recovery did not mail the missing envelope")
}

func TestReduceCellIntentsIdempotentReplay(t *testing.T) {
	rt, _ := testRuntime(t)
	scope := testReductionScope()
	scope.CellID = "cell-replay"
	ctx := testReductionCtx(scope)
	intents := []yaegikernel.StagedIntent{
		{LocalID: "tray-1", Kind: yaegikernel.IntentMessage, ToDesk: "management", Body: "same"},
	}
	first, err := ReduceCellIntents(ctx, rt, scope, intents, true)
	if err != nil || !first.Committed || len(first.Intents) != 1 {
		t.Fatalf("first reduce = %+v, %v", first, err)
	}
	second, err := ReduceCellIntents(ctx, rt, scope, intents, true)
	if err != nil || !second.Committed || len(second.Intents) != 1 {
		t.Fatalf("replay reduce = %+v, %v", second, err)
	}
	if first.Intents[0].Seq != second.Intents[0].Seq {
		t.Fatalf("replay assigned new seq %d, want %d", second.Intents[0].Seq, first.Intents[0].Seq)
	}
	msgs, _, err := rt.ChannelRead(scope.ChannelID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 {
		t.Fatalf("replay duplicated envelopes: %d messages", len(msgs))
	}
}

func TestReduceCellIntentsSequentialCellsAtSameCursor(t *testing.T) {
	rt, _ := testRuntime(t)
	scope := testReductionScope()
	ctx := testReductionCtx(scope)
	first, err := ReduceCellIntents(ctx, rt, scope, []yaegikernel.StagedIntent{
		{LocalID: "tray-1", Kind: yaegikernel.IntentMessage, ToDesk: "management", Body: "cell-a"},
	}, true)
	if err != nil || !first.Committed {
		t.Fatalf("first cell = %+v, %v", first, err)
	}
	second, err := ReduceCellIntents(ctx, rt, scope, []yaegikernel.StagedIntent{
		{LocalID: "tray-1", Kind: yaegikernel.IntentMessage, ToDesk: "management", Body: "cell-b"},
	}, true)
	if err != nil || !second.Committed {
		t.Fatalf("second cell = %+v, %v", second, err)
	}
	if first.Intents[0].Seq == second.Intents[0].Seq {
		t.Fatalf("sequential cells at the same cursor reused seq %d", first.Intents[0].Seq)
	}
	msgs, _, err := rt.ChannelRead(scope.ChannelID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 2 {
		t.Fatalf("sequential cells = %d messages, want 2", len(msgs))
	}
}

// TestCommitActIntentEscalateActionsRejectsUnsafeActions is the R2 carrier gate
// for the privileged-execution verb: an escalate carrying an actions payload
// must satisfy the same per-action safety contract the retired
// execution_request packet enforced, or the cell reduces nothing.
func TestCommitActIntentEscalateActionsRejectsUnsafeActions(t *testing.T) {
	rt, _ := testRuntime(t)
	scope := testReductionScope()
	scope.CellID = "cell-escalate-actions"
	ctx := testReductionCtx(scope)

	cases := []struct {
		name      string
		actions   string
		wantError string
	}{
		{"malformed json", `[{`, "not a valid actions array"},
		{"empty actions", `[]`, "at least one action"},
		{"missing type", `[{"objective":"x","safety":{"mutation_class":"green","network":"forbidden","file_mutation":"forbidden"}}]`, "type"},
		{"missing safety", `[{"type":"run_tests","objective":"x"}]`, "safety"},
		{"bad mutation class", `[{"type":"run_tests","objective":"x","safety":{"mutation_class":"nonsense","network":"forbidden","file_mutation":"forbidden"}}]`, "mutation_class"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reduction := &rlmCallReduction{active: true, mb: rt, st: rt.store, scope: scope}
			_, err := reduction.commitActIntent(ctx, yaegikernel.StagedIntent{
				LocalID: "esc-1", Kind: yaegikernel.IntentEscalate,
				ToDesk: "management", Body: "need exec", Actions: tc.actions,
			})
			if err == nil || !strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("escalate_actions err = %v, want %q", err, tc.wantError)
			}
		})
	}

	// A fully-guarded actions payload reduces: the commitment mints and the
	// escalation envelope mails to the target desk.
	safe := `[{"type":"run_tests","objective":"re-run the failing shard","inputs":{"shard":"0"},"safety":{"mutation_class":"green","network":"forbidden","file_mutation":"forbidden"}}]`
	reduction := &rlmCallReduction{active: true, mb: rt, st: rt.store, scope: scope}
	seq, err := reduction.commitActIntent(ctx, yaegikernel.StagedIntent{
		LocalID: "esc-ok", Kind: yaegikernel.IntentEscalate,
		ToDesk: "management", Body: "re-run shard 0", Actions: safe,
	})
	if err != nil {
		t.Fatalf("guarded escalate_actions: %v", err)
	}
	if seq == 0 {
		t.Fatal("guarded escalate_actions mailed no envelope")
	}
}

// TestCommitActIntentReportPacketBody is the R2 Report-as-packet contract: a
// report carrying the full coagent source-packet body validates against the
// same payload schema the retired update_coagent enforced, and mails the
// packet through the envelope so the target desk reads it.
func TestCommitActIntentReportPacketBody(t *testing.T) {
	rt, _ := testRuntime(t)
	scope := testReductionScope()
	scope.CellID = "cell-report-packet"
	ctx := testReductionCtx(scope)

	// A packet failing the payload contract reduces nothing.
	bad := `{"schema_version":"coagent_source_packet.v1","kind":"evidence_update","summary":""}`
	reduction := &rlmCallReduction{active: true, mb: rt, st: rt.store, scope: scope}
	if _, err := reduction.commitActIntent(ctx, yaegikernel.StagedIntent{
		LocalID: "rep-bad", Kind: yaegikernel.IntentReport, ToDesk: "management",
		Packet: bad, ResolverID: "management:root",
	}); err == nil || !strings.Contains(err.Error(), "packet invalid") {
		t.Fatalf("bad packet err = %v, want packet invalid", err)
	}

	// A valid packet commits and mails through the envelope.
	good := `{"schema_version":"coagent_source_packet.v1","kind":"evidence_update","summary":"source ready","claims":[{"text":"official source confirms"}],"sources":[{"source_id":"src-1","kind":"content_item","target":{"uri":"https://example.test/x"},"excerpt":"excerpt"}]}`
	reduction = &rlmCallReduction{active: true, mb: rt, st: rt.store, scope: scope}
	seq, err := reduction.commitActIntent(ctx, yaegikernel.StagedIntent{
		LocalID: "rep-ok", Kind: yaegikernel.IntentReport, ToDesk: "management",
		Packet: good, ResolverID: "management:root",
	})
	if err != nil {
		t.Fatalf("valid report packet: %v", err)
	}
	if seq == 0 {
		t.Fatal("report packet mailed no envelope")
	}
	msgs, _, err := rt.ChannelRead(scope.ChannelID, 0)
	if err != nil {
		t.Fatal(err)
	}
	foundPacket := false
	for _, m := range msgs {
		if strings.Contains(m.Content, `"packet"`) && strings.Contains(m.Content, "src-1") {
			foundPacket = true
		}
	}
	if !foundPacket {
		t.Fatalf("report packet body not on the wire: %+v", msgs)
	}
}

func TestReduceCellIntentsSequentialCellsDistinctDestinations(t *testing.T) {
	rt, _ := testRuntime(t)
	scope := testReductionScope()
	ctx := testReductionCtx(scope)
	first, err := ReduceCellIntents(ctx, rt, scope, []yaegikernel.StagedIntent{
		{LocalID: "tray-1", Kind: yaegikernel.IntentMessage, ToDesk: "peer-a", Body: "same"},
	}, true)
	if err != nil || !first.Committed {
		t.Fatalf("first dest = %+v, %v", first, err)
	}
	second, err := ReduceCellIntents(ctx, rt, scope, []yaegikernel.StagedIntent{
		{LocalID: "tray-1", Kind: yaegikernel.IntentMessage, ToDesk: "peer-b", Body: "same"},
	}, true)
	if err != nil || !second.Committed {
		t.Fatalf("second dest = %+v, %v", second, err)
	}
	if first.Intents[0].Seq == second.Intents[0].Seq {
		t.Fatalf("distinct destinations reused seq %d", first.Intents[0].Seq)
	}
	msgs, _, err := rt.ChannelRead(scope.ChannelID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 2 {
		t.Fatalf("distinct destinations = %d messages, want 2", len(msgs))
	}
	if msgs[0].ToAgentID == msgs[1].ToAgentID {
		t.Fatalf("destination-distinct envelopes collapsed: %+v", msgs)
	}
}

func TestReduceCellIntentsReplaySkipsEventAndWake(t *testing.T) {
	rt, st := testRuntime(t)
	scope := testReductionScope()
	scope.CellID = "cell-replay-side-effect"
	ctx := testReductionCtx(scope)
	var mu sync.Mutex
	var wakes int
	rt.SetDispatchActor(func(_ context.Context, _, _, toAgentID, kind, _, _, _ string) error {
		mu.Lock()
		wakes++
		mu.Unlock()
		return nil
	})
	intents := []yaegikernel.StagedIntent{
		{LocalID: "tray-1", Kind: yaegikernel.IntentMessage, ToDesk: "management", Body: "same"},
	}
	if _, err := ReduceCellIntents(ctx, rt, scope, intents, true); err != nil {
		t.Fatal(err)
	}
	if _, err := ReduceCellIntents(ctx, rt, scope, intents, true); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	gotWakes := wakes
	mu.Unlock()
	if gotWakes != 2 {
		t.Fatalf("wakes = %d, want 2 (replay re-issues wake; actor.Send collapses UpdateID)", gotWakes)
	}
	events, err := st.ListEventsByChannel(ctx, scope.OwnerID, scope.ChannelID, 20)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, ev := range events {
		if ev.Kind == types.EventChannelMessage {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("channel events = %d, want 1 (replay must not emit)", count)
	}
}

func TestChannelCastWakesAddressedActor(t *testing.T) {
	rt, _ := testRuntime(t)
	var mu sync.Mutex
	var wakes []string
	rt.SetDispatchActor(func(_ context.Context, _, _, toAgentID, kind, content, _, _ string) error {
		mu.Lock()
		wakes = append(wakes, kind+":"+toAgentID+":"+content)
		mu.Unlock()
		return nil
	})
	ctx := testReductionCtx(testReductionScope())
	if _, err := rt.ChannelCast(ctx, "chan-wake", "management:root", "", "engineering:impl", "engineering", "hi"); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	if len(wakes) != 1 || wakes[0] != "channel_message:management:root:chan-wake:1" {
		mu.Unlock()
		t.Fatalf("wakes = %v, want channel_message to management:root with chan-wake:1", wakes)
	}
	mu.Unlock()
	if _, err := rt.ChannelCast(ctx, "chan-wake", "management:root", "", "engineering:impl", "engineering", "hi"); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	if len(wakes) != 2 || wakes[1] != "channel_message:management:root:chan-wake:2" {
		mu.Unlock()
		t.Fatalf("same-body second envelope collapsed: %v", wakes)
	}
	mu.Unlock()
	if _, err := rt.ChannelCast(ctx, "chan-wake", "", "", "engineering:impl", "engineering", "broadcast"); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(wakes) != 2 {
		t.Fatalf("broadcast must not wake: %v", wakes)
	}
}
