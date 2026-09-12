package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// The scheduling contract (I26) requires every execution_request bound for the
// persistent Super to carry a durable computer-scoped arrival ordinal. Ordinals
// must be monotonic across turns, restarts, and trajectories on one computer;
// FIFO selection orders by (ordinal, update_id).
func TestApplyTextureTurnAssignsComputerScopedArrivalOrdinalsToSuperExecutionRequests(t *testing.T) {
	s, start, caller, _ := setupLifecycleTextureTargetFixture(t)
	ctx := context.Background()
	now := time.Now().UTC()
	superID := "super:" + start.OwnerID
	if err := s.UpsertAgent(ctx, types.AgentRecord{AgentID: superID, OwnerID: start.OwnerID, ComputerID: start.ComputerID,
		Profile: "super", Role: "super", ChannelID: superID, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}

	req := textureTurnBaseRequest(t, s, start, caller, types.TextureTurnWait)
	req.CommandID, req.Reason = "texture-turn-ordinal-first", "first opener"
	control := textureTurnControl(t, "control-ordinal-a", superID, "work-super-target")
	control.OpenWork = &types.WorkItemRecord{WorkItemID: "work-super-target", Objective: "coordinate exact implementation",
		AuthorityProfile: "super", AssignedAgentID: superID, StepBudget: 8}
	req.Controls = []types.TextureTurnControl{control}
	setTextureTurnDigest(t, &req, TextureSourceGraphWriteSet{})
	first, err := s.ApplyTextureTurn(ctx, req)
	if err != nil {
		t.Fatalf("apply first Super opener: %v", err)
	}
	if len(first.Controls) != 1 || first.Controls[0].ArrivalOrdinal != 1 {
		t.Fatalf("first Super execution request ordinal = %+v, want 1", first.Controls)
	}

	second := textureTurnBaseRequest(t, s, start, caller, types.TextureTurnWait)
	second.CommandID, second.Reason = "texture-turn-ordinal-second", "continuation"
	secondControl := textureTurnControl(t, "control-ordinal-b", superID, control.TargetWorkItemID)
	secondControl.OpenWork = control.OpenWork
	second.Controls = []types.TextureTurnControl{secondControl}
	setTextureTurnDigest(t, &second, TextureSourceGraphWriteSet{})
	if _, err := s.ApplyTextureTurn(ctx, second); err != nil {
		t.Fatalf("apply second control: %v", err)
	}

	pending, err := s.ListAllPendingLifecycleUpdates(ctx, start.OwnerID, start.ComputerID, superID)
	if err != nil || len(pending) != 2 {
		t.Fatalf("pending Super controls = %+v, %v", pending, err)
	}
	if pending[0].UpdateID != "control-ordinal-a" || pending[0].ArrivalOrdinal != 1 ||
		pending[1].UpdateID != "control-ordinal-b" || pending[1].ArrivalOrdinal != 2 {
		t.Fatalf("arrival ordinals not monotonic across turns: %+v", pending)
	}
	for _, update := range pending {
		if update.ReducerSeq == 0 || update.LifecycleVersion <= 0 {
			t.Fatalf("control %s lacks lifecycle identity: %+v", update.UpdateID, update)
		}
	}
}

// A non-execution control or a Researcher-targeted control must NOT consume an
// arrival ordinal: the scheduler counts only Super-bound execution requests.
func TestApplyTextureTurnDoesNotSpendArrivalOrdinalOnResearcherControls(t *testing.T) {
	s, start, caller, _ := setupLifecycleTextureTargetFixture(t)
	ctx := context.Background()

	req := textureTurnBaseRequest(t, s, start, caller, types.TextureTurnWait)
	req.CommandID, req.Reason = "texture-turn-researcher-no-spend", "research direction"
	researcherAgentID := "researcher:ordinal-check"
	researcherWorkID := "research-work-ordinal-check"
	control := textureTurnControl(t, "control-researcher-ordinal", researcherAgentID, researcherWorkID)
	control.Packet.Kind = "question"
	control.PayloadDigest, _ = ComputeLifecycleUpdatePayloadDigest(control.Packet, control.Content)
	control.OpenAgent = &types.AgentRecord{AgentID: researcherAgentID, OwnerID: start.OwnerID, ComputerID: start.ComputerID,
		Profile: "researcher", Role: "researcher", ChannelID: start.InitialDocument.DocID}
	control.OpenWork = &types.WorkItemRecord{WorkItemID: researcherWorkID, Objective: "research exact gap",
		AuthorityProfile: "researcher", AssignedAgentID: researcherAgentID}
	req.Controls = []types.TextureTurnControl{control}
	setTextureTurnDigest(t, &req, TextureSourceGraphWriteSet{})
	result, err := s.ApplyTextureTurn(ctx, req)
	if err != nil || len(result.Controls) != 1 {
		t.Fatalf("researcher control turn = %+v, %v", result, err)
	}
	if result.Controls[0].ArrivalOrdinal != 0 {
		t.Fatalf("researcher control consumed an arrival ordinal: %+v", result.Controls[0])
	}

	_, found, err := s.peekArrivalOrdinal(ctx, start.OwnerID, start.ComputerID)
	if err != nil {
		t.Fatal(err)
	}
	if found {
		t.Fatalf("arrival sequence object was created for non-Super control")
	}
}

// FIFO selection ordering: among two computers the ordinals are independent.
// Ordinals are independent per computer: two computers in one store each start
// their own sequence at 1 (computer-scoped FIFO, not owner-global).
func TestArrivalOrdinalIsComputerScoped(t *testing.T) {
	s, start, _, _ := setupLifecycleTextureTargetFixture(t)
	ctx := context.Background()

	a, err := s.nextArrivalOrdinal(ctx, start.OwnerID, start.ComputerID)
	if err != nil || a != 1 {
		t.Fatalf("computer A first ordinal = %d, %v", a, err)
	}
	a2, err := s.nextArrivalOrdinal(ctx, start.OwnerID, start.ComputerID)
	if err != nil || a2 != 2 {
		t.Fatalf("computer A second ordinal = %d, %v", a2, err)
	}
	computerB := start.ComputerID + "-b"
	b, err := s.nextArrivalOrdinal(ctx, start.OwnerID, computerB)
	if err != nil || b != 1 {
		t.Fatalf("computer B first ordinal = %d, %v; want independent sequence starting at 1", b, err)
	}
	if _, found, err := s.peekArrivalOrdinal(ctx, start.OwnerID, start.ComputerID); err != nil || !found || found == false {
		t.Fatalf("computer A sequence missing after B allocation: found=%t err=%v", found, err)
	}
}

// Concurrent ordinal allocation must fail closed with a conflict rather than
// issuing duplicate ordinals; the losing command replays after reload.
func TestArrivalOrdinalAllocationConflictsInsteadOfReusing(t *testing.T) {
	s, start, _, _ := setupLifecycleTextureTargetFixture(t)
	ctx := context.Background()
	now := time.Now().UTC()
	superID := "super:" + start.OwnerID
	if err := s.UpsertAgent(ctx, types.AgentRecord{AgentID: superID, OwnerID: start.OwnerID, ComputerID: start.ComputerID,
		Profile: "super", Role: "super", ChannelID: superID, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	// Directly allocate one ordinal to advance the counter...
	if _, err := s.nextArrivalOrdinal(ctx, start.OwnerID, start.ComputerID); err != nil {
		t.Fatalf("seed ordinal: %v", err)
	}
	obj, err := s.lifecycleGetObject(ctx, ogKindSchedulerSeq, start.OwnerID, start.ComputerID, arrivalSequenceKey)
	if err != nil {
		t.Fatalf("load arrival sequence: %v", err)
	}
	// ...then simulate a stale allocator that read a DIFFERENT row state: the
	// condition compares the stored row's hash against the allocator's expected
	// hash, so a wrong expected hash must conflict the whole allocation.
	stale := obj.ContentHash + "stale"
	if _, conflictErr := s.nextArrivalOrdinalFrom(ctx, start.OwnerID, start.ComputerID, stale); !errors.Is(conflictErr, ErrConcurrentStateChange) {
		t.Fatalf("stale allocation error = %v, want ErrConcurrentStateChange", conflictErr)
	}
	fresh, freshErr := s.nextArrivalOrdinal(ctx, start.OwnerID, start.ComputerID)
	if freshErr != nil || fresh != 2 {
		t.Fatalf("fresh allocation after conflict = %d, %v", fresh, freshErr)
	}
}
