package agentcore

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

func testReductionScope() ReductionScope {
	return ReductionScope{
		FromAgentID: "engineering:impl",
		DeskAgentID: "engineering:impl",
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

	// C1e D2: a document trajectory's assignment may open its own candidate,
	// so the refusal here is the incomplete freeze itself; it must land
	// before any executor effect and open no operation.
	_, err := reduction.commitFreezeIntent(ctx, yaegikernel.StagedIntent{Kind: yaegikernel.IntentFreeze})
	if err == nil {
		t.Fatal("incomplete document trajectory freeze was not refused")
	}
	if _, found, lookupErr := rt.selfdevOperations.GetByIdempotency(context.Background(), rec.ComputerID, store.EngineeringFreezeIdempotencyKey("assignment-document-freeze")); lookupErr != nil || found {
		t.Fatalf("refused freeze opened an operation: found=%v err=%v", found, lookupErr)
	}
}

// TestInboxCursorSurvivesDeskRespawn proves the (channel,desk) rekey: a
// cursor committed under run A is recovered when the desk respawns with a
// new run_id — the invariant the runID key could not hold. Commit stamps
// desk_agent_id; the ForDesk read finds it across the agent's runs.
func TestInboxCursorSurvivesDeskRespawn(t *testing.T) {
	rt, _ := testRuntime(t)
	scope := testReductionScope()
	ctx := testReductionCtx(scope)

	// Commit a cursor under the desk's first run.
	const watermark = 42
	if err := CommitInboxCursor(ctx, rt.store, scope.OwnerID, "run-respawn-A", scope.DeskAgentID, scope.ChannelID, watermark); err != nil {
		t.Fatalf("commit under run A: %v", err)
	}
	// Respawn: a different run_id under the same desk agent + channel must
	// resume at the watermark, not replay from zero.
	cursor, err := LoadInboxCursorForDesk(ctx, rt.store, scope.OwnerID, scope.DeskAgentID, scope.ChannelID)
	if err != nil {
		t.Fatalf("load after respawn: %v", err)
	}
	if cursor != watermark {
		t.Fatalf("respawned desk cursor = %d, want %d — (channel,desk) key not honored", cursor, watermark)
	}
	// A different desk agent on the same channel starts at zero: keys do not
	// collide.
	other, err := LoadInboxCursorForDesk(ctx, rt.store, scope.OwnerID, "texture:other", scope.ChannelID)
	if err != nil {
		t.Fatalf("load other desk: %v", err)
	}
	if other != 0 {
		t.Fatalf("other desk leaked cursor = %d, want 0", other)
	}
}

func TestCellTerminalDeadlineRecordsTimeoutForRestartPassivatedCell(t *testing.T) {
	rt, _ := testRuntime(t)
	scope := testReductionScope()
	scope.ComputerID = rt.TextureComputerID()
	scope.CellID = "cell-restart-passivated"
	now := time.Now().UTC()
	rec := &types.RunRecord{
		RunID: scope.RunID, OwnerID: scope.OwnerID, ComputerID: scope.ComputerID,
		AgentID: scope.FromAgentID, ChannelID: scope.ChannelID, State: types.RunPassivated,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := rt.store.CreateRun(context.Background(), *rec); err != nil {
		t.Fatal(err)
	}
	content, err := encodeCellTerminalDeadline(rec, scope)
	if err != nil {
		t.Fatal(err)
	}
	if err := rt.HandleCellTerminalDeadline(context.Background(), rec.OwnerID, rec.ComputerID, rec.AgentID, content); err != nil {
		t.Fatal(err)
	}
	fate, found, err := CellFate(context.Background(), rt.store, scope.OwnerID, scope.RunID, scope.CellID)
	if err != nil || !found || fate != cellFateTimeout {
		t.Fatalf("passivated deadline fate = %q, found=%t, err=%v", fate, found, err)
	}
	if err := rt.HandleCellTerminalDeadline(context.Background(), rec.OwnerID, rec.ComputerID, rec.AgentID, content); err != nil {
		t.Fatalf("duplicate deadline: %v", err)
	}
	entries, err := rt.store.ListRunMemoryEntries(context.Background(), scope.OwnerID, scope.RunID)
	if err != nil {
		t.Fatal(err)
	}
	fates := 0
	for _, entry := range entries {
		if entry.Kind == types.RunMemoryEntryCellFate {
			fates++
		}
	}
	if fates != 1 {
		t.Fatalf("cell fate entries = %d, want one", fates)
	}
}

func TestArmCellTerminalDeadlineCarriesReductionIdentity(t *testing.T) {
	rt, _ := testRuntime(t)
	rt.kernelMode = true
	deadline := time.Now().UTC().Add(time.Minute)
	var gotKind, gotContent string
	var gotNotBefore time.Time
	rt.scheduleActor = func(_ context.Context, _, _, _, kind, content, _, _ string, notBefore time.Time) error {
		gotKind, gotContent, gotNotBefore = kind, content, notBefore
		return nil
	}
	rec := &types.RunRecord{
		RunID: "run-cell-deadline", OwnerID: "user-alice", ComputerID: rt.TextureComputerID(),
		AgentID: "research:deadline", ChannelID: "channel-deadline",
	}
	reduction := &rlmCallReduction{
		active: true, rec: rec,
		scope: ReductionScope{
			RunID: rec.RunID, OwnerID: rec.OwnerID, ComputerID: rec.ComputerID,
			FromAgentID: rec.AgentID, ChannelID: rec.ChannelID, CellID: "cell-deadline", Cursor: 7,
		},
	}
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	rt.armCellTerminalDeadline(ctx, reduction)
	if gotKind != cellTerminalDeadlineUpdateKind || !gotNotBefore.Equal(deadline) {
		t.Fatalf("scheduled cell deadline = kind=%q not_before=%v", gotKind, gotNotBefore)
	}
	got, err := decodeCellTerminalDeadline(gotContent)
	if err != nil {
		t.Fatal(err)
	}
	if got.RunID != rec.RunID || got.CellID != reduction.scope.CellID || got.Cursor != reduction.scope.Cursor ||
		got.ChannelID != rec.ChannelID || got.AgentID != rec.AgentID {
		t.Fatalf("scheduled cell deadline payload = %+v", got)
	}
}

func TestCellTerminalDeadlineDoesNotCancelReactivatedRun(t *testing.T) {
	rt, _ := testRuntime(t)
	now := time.Now().UTC()
	scope := testReductionScope()
	scope.ComputerID = rt.TextureComputerID()
	scope.CellID = "cell-restarted-generation"
	rec := types.RunRecord{
		RunID: scope.RunID, OwnerID: scope.OwnerID, ComputerID: scope.ComputerID,
		AgentID: scope.FromAgentID, ChannelID: scope.ChannelID, State: types.RunRunning,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := rt.store.CreateRun(context.Background(), rec); err != nil {
		t.Fatal(err)
	}
	content, err := encodeCellTerminalDeadline(&rec, scope)
	if err != nil {
		t.Fatal(err)
	}
	rec.State = types.RunPassivated
	rec.UpdatedAt = now.Add(time.Second)
	if err := rt.store.UpdateRun(context.Background(), rec); err != nil {
		t.Fatal(err)
	}
	rec.State = types.RunRunning
	rec.UpdatedAt = now.Add(2 * time.Second)
	if err := rt.store.UpdateRun(context.Background(), rec); err != nil {
		t.Fatal(err)
	}
	if err := rt.HandleCellTerminalDeadline(context.Background(), rec.OwnerID, rec.ComputerID, rec.AgentID, content); err != nil {
		t.Fatal(err)
	}
	stored, err := rt.store.GetRunByOwner(context.Background(), rec.OwnerID, rec.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.State != types.RunRunning {
		t.Fatalf("reactivated run was cancelled by stale cell deadline: %+v", stored)
	}
	fate, found, err := CellFate(context.Background(), rt.store, scope.OwnerID, scope.RunID, scope.CellID)
	if err != nil || !found || fate != cellFateTimeout {
		t.Fatalf("reactivated cell fate = %q, found=%t, err=%v", fate, found, err)
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

// TestChannelCastRejectsTextureTarget proves the reduce-time refusal: channel
// mail addressed to a texture desk (agent-scoped or bare) is rejected at cast
// time, not persisted as a durable dead letter that the actor boundary would
// silently drop. Receipt: docs/problems/s0m-channel-mail-texture-dead-letter-2026-10-02.md.
func TestChannelCastRejectsTextureTarget(t *testing.T) {
	rt, s := testRuntime(t)
	scope := testReductionScope()
	ctx := testReductionCtx(scope)

	for _, target := range []string{"texture", "texture:doc-dead-letter"} {
		if _, err := rt.CastEnvelope(ctx, "chan-texture", target, scope.FromAgentID, "research", "body", "key-"+target); err == nil {
			t.Fatalf("CastEnvelope to %q succeeded; want reduce-time reject", target)
		} else if !strings.Contains(err.Error(), "not reachable by channel mail") {
			t.Fatalf("CastEnvelope to %q error = %v, want texture unreachable message", target, err)
		}
	}
	// Nothing was persisted: the texture channel log stays empty.
	msgs, err := s.ListChannelMessagesTo(ctx, scope.OwnerID, "texture:doc-dead-letter", 0, 10)
	if err != nil {
		t.Fatalf("list texture channel messages: %v", err)
	}
	if len(msgs) != 0 {
		t.Fatalf("texture-targeted cast persisted %d messages as dead letters", len(msgs))
	}
	// A normal addressed cast and a broadcast still succeed.
	if _, err := rt.CastEnvelope(ctx, "chan-ok", "research:doc-1", scope.FromAgentID, "research", "ok", "k1"); err != nil {
		t.Fatalf("legitimate addressed cast rejected: %v", err)
	}
	if _, err := rt.ChannelPost(ctx, "chan-ok", "engineering:impl", "engineering", "broadcast"); err != nil {
		t.Fatalf("broadcast rejected: %v", err)
	}
}

// Journal cut R2: the inbox cursor is a run-memory entry written after every
// cell, and the reader takes the maximum, so a cell whose inbox snapshot did
// not move the cursor wrote an entry that changes nothing and costs about 90
// KiB of journal. Skipped when the cursor did not move.
//
// Ways this could fail, written first: the entry is still written on an equal
// cursor; the entry is skipped when the cursor did move; a skipped write makes
// the durable cursor read lower than before.
func TestCommitSkipsInboxCursorEntryWhenCursorDidNotMove(t *testing.T) {
	rt, _ := testRuntime(t)
	scope := testReductionScope()
	ctx := testReductionCtx(scope)
	count := func() int {
		t.Helper()
		entries, err := rt.store.ListRunMemoryEntriesForAgent(ctx, scope.OwnerID, scope.DeskAgentID, []string{string(rlmInboxCursorKind)})
		if err != nil {
			t.Fatal(err)
		}
		return len(entries)
	}
	cell := func(cursor, highWater uint64, localID string) {
		t.Helper()
		sc := scope
		sc.Cursor = cursor
		r := &rlmCallReduction{active: true, mb: rt, st: rt.store, scope: sc, highWater: highWater}
		if err := r.commit(ctx, []yaegikernel.StagedIntent{
			{LocalID: localID, Kind: yaegikernel.IntentMessage, ToDesk: "management", Body: localID},
		}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := rt.ChannelCast(ctx, scope.ChannelID, scope.FromAgentID, "", "management", "management", "snapshot-mail"); err != nil {
		t.Fatal(err)
	}

	cell(0, 1, "moves-cursor") // 0 -> 1: one entry
	if got := count(); got != 1 {
		t.Fatalf("cursor entries after a cell that moved the cursor = %d, want 1", got)
	}
	cell(1, 1, "same-cursor-a") // inbox unchanged: no entry
	cell(1, 0, "same-cursor-b") // a snapshot below the cursor is clamped to it: no entry
	if got := count(); got != 1 {
		t.Fatalf("cursor entries after two cells that did not move it = %d, want still 1", got)
	}
	if cursor, err := LoadInboxCursorForDesk(ctx, rt.store, scope.OwnerID, scope.DeskAgentID, scope.ChannelID); err != nil || cursor != 1 {
		t.Fatalf("durable cursor = %d, %v; want 1", cursor, err)
	}
	cell(1, 2, "moves-again") // 1 -> 2: a second entry
	if got := count(); got != 2 {
		t.Fatalf("cursor entries after the cursor moved again = %d, want 2", got)
	}
	if cursor, err := LoadInboxCursorForDesk(ctx, rt.store, scope.OwnerID, scope.DeskAgentID, scope.ChannelID); err != nil || cursor != 2 {
		t.Fatalf("durable cursor = %d, %v; want 2", cursor, err)
	}
}

// The paper's top store_stalled holder is ListRunMemoryEntriesForAgent
// (docs/problems/run-memory-entries-hold-the-engine-2026-10-23.md). A desk
// cell read it twice: at cell start (LoadInboxCursorForDesk in
// rlmReductionForDeskCall) and again at commit (recoverPartialActCommit loaded
// the cursor before it knew it needed it). The common cell, a fresh act with
// no commitment record yet, needs only the first.
//
// Ways this could fail, written first: the commit still reads the cursor when
// no commitment record exists (2 calls per cell); the fix drops the read from
// the recovery path too, so a crash-recovered tray no longer sees the durable
// cursor (it replays the whole tray instead of only the missing part, or
// re-mails); the lazy load runs once per intent instead of once per commit.
func deskCellRuntime(t *testing.T) (*Runtime, ReductionScope, context.Context) {
	t.Helper()
	rt, _ := testRuntime(t)
	scope := testReductionScope()
	scope.ComputerID = rt.TextureComputerID()
	ctx := toolregistry.WithExecutionContext(context.Background(), toolregistry.ExecutionContext{
		RunID: scope.RunID, AgentID: scope.FromAgentID, OwnerID: scope.OwnerID,
		ChannelID: scope.ChannelID, ComputerID: scope.ComputerID,
	})
	if _, err := rt.ChannelCast(ctx, scope.ChannelID, scope.FromAgentID, "", "management", "management", "snapshot-mail"); err != nil {
		t.Fatal(err)
	}
	return rt, scope, ctx
}

func TestDeskCellReadsTheAgentCursorOnceInTheCommonCase(t *testing.T) {
	rt, _, ctx := deskCellRuntime(t)
	before := rt.store.RunMemoryAgentListCalls()
	reduction := rlmReductionForDeskCall(ctx, rt)
	if !reduction.active || reduction.highWater != 1 {
		t.Fatalf("cell start reduction = active %t, highWater %d; want active, 1", reduction.active, reduction.highWater)
	}
	atStart := rt.store.RunMemoryAgentListCalls() - before
	intent := yaegikernel.StagedIntent{LocalID: "ask-common", Kind: yaegikernel.IntentAsk, ToDesk: "management", Question: "is it ready?"}
	if err := reduction.commit(ctx, []yaegikernel.StagedIntent{intent}); err != nil {
		t.Fatal(err)
	}
	perCell := rt.store.RunMemoryAgentListCalls() - before
	t.Logf("ListRunMemoryEntriesForAgent calls: cell start %d, whole cell %d", atStart, perCell)
	if atStart != 1 {
		t.Fatalf("calls at cell start = %d, want 1", atStart)
	}
	if perCell != 1 {
		t.Fatalf("calls per desk cell = %d, want 1 (start only; the commit has no prior commitment record to recover)", perCell)
	}
	if cursor, err := LoadInboxCursorForDesk(ctx, rt.store, reduction.scope.OwnerID, reduction.scope.DeskAgentID, reduction.scope.ChannelID); err != nil || cursor != 1 {
		t.Fatalf("durable cursor after the cell = %d, %v; want 1", cursor, err)
	}
}

func TestCommitWithNoPriorCommitmentRecordDoesNotReadTheAgentCursor(t *testing.T) {
	rt, scope, ctx := deskCellRuntime(t)
	scope.CellID = "cell-no-prior-record"
	reduction := &rlmCallReduction{active: true, mb: rt, st: rt.store, ledger: rt.store, scope: scope, highWater: 1}
	intents := []yaegikernel.StagedIntent{
		{LocalID: "ask-a", Kind: yaegikernel.IntentAsk, ToDesk: "management", Question: "first?"},
		{LocalID: "ask-b", Kind: yaegikernel.IntentAsk, ToDesk: "management", Question: "second?"},
	}
	before := rt.store.RunMemoryAgentListCalls()
	if err := reduction.commit(ctx, intents); err != nil {
		t.Fatal(err)
	}
	if got := rt.store.RunMemoryAgentListCalls() - before; got != 0 {
		t.Fatalf("commit of two fresh acts read the agent cursor %d times, want 0", got)
	}
}

func TestPartialCommitRecoveryStillReadsTheAgentCursor(t *testing.T) {
	rt, scope, ctx := deskCellRuntime(t)
	scope.CellID = "cell-recovery-reads-cursor"
	first := yaegikernel.StagedIntent{LocalID: "ask-1", Kind: yaegikernel.IntentAsk, ToDesk: "management", Question: "is the cast ready?"}
	second := yaegikernel.StagedIntent{LocalID: "ask-2", Kind: yaegikernel.IntentAsk, ToDesk: "management", Question: "and the next?"}
	// A process death after the append-only ledger mint of both acts and
	// before the envelope and cursor writes.
	for _, in := range []yaegikernel.StagedIntent{first, second} {
		if _, err := rt.store.AppendCommitmentRecord(ctx, scope.OwnerID, scope.ComputerID, commitmentRecordForIntent(scope, in)); err != nil {
			t.Fatalf("stage commitment record: %v", err)
		}
	}
	reduction := &rlmCallReduction{active: true, mb: rt, st: rt.store, ledger: rt.store, scope: scope, highWater: 1}
	before := rt.store.RunMemoryAgentListCalls()
	if err := reduction.commit(ctx, []yaegikernel.StagedIntent{first, second}); err != nil {
		t.Fatalf("recover partial act commit: %v", err)
	}
	got := rt.store.RunMemoryAgentListCalls() - before
	// Once for the recovery decision, however many intents have records.
	if got != 1 {
		t.Fatalf("recovery read the agent cursor %d times, want exactly 1", got)
	}
	if !reduction.receipt.Committed || reduction.receipt.Cursor != 1 {
		t.Fatalf("recovery receipt = %+v", reduction.receipt)
	}
	// Now the cursor is durable at 1: a replay finds nothing behind it and
	// must read it again, and must still not duplicate mail.
	before = rt.store.RunMemoryAgentListCalls()
	if err := reduction.commit(ctx, []yaegikernel.StagedIntent{first, second}); err != nil {
		t.Fatalf("replay recovered tray: %v", err)
	}
	if got := rt.store.RunMemoryAgentListCalls() - before; got != 1 {
		t.Fatalf("replay read the agent cursor %d times, want 1", got)
	}
	to, content, mailed, err := stagedIntentEnvelope(scope, first)
	if err != nil || !mailed {
		t.Fatalf("envelope = (%q, %q, %t, %v)", to, content, mailed, err)
	}
	key := intentIdempotencyKey(scope, first.LocalID, to, content)
	messages, _, err := rt.ChannelRead(scope.ChannelID, 0)
	if err != nil {
		t.Fatal(err)
	}
	matched := 0
	for _, m := range messages {
		if m.IdempotencyKey == key {
			matched++
		}
	}
	if matched != 1 {
		t.Fatalf("recovered envelope count = %d, want 1", matched)
	}
}
