package agentcore

// The round receipt reads the spawn and the job waits from the wait-summary
// record and puts one record, and one Texture line, at the round's end.
// Failure modes, written before the code:
//
//  P1 the spawn mark never reaches the ledger, or a round with no wait writes
//     no summary record (so the receipt has no spawn);
//  P2 the receipt is not in management's acting pack;
//  P3 a replayed act commits a second receipt, or a different Texture line;
//  P4 an operation that is not an engineering Freeze gets a receipt.

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func TestRoundReceiptIsOnTheLedgerAndInManagementsPack(t *testing.T) {
	rt, s, _ := c1eRuntime(t)
	ctx := context.Background()
	seed, err := store.SeedEngineeringAssignmentAuthority(s, "owner-round", rt.TextureComputerID(), 1)
	if err != nil {
		t.Fatal(err)
	}
	assignment, _ := c1eOpenBound(t, s, seed, 0)
	start := time.Now().UTC()
	now := start
	waitTestClock(rt, &now)
	busy := &engineeringAdmissionBusy{AssignmentID: assignment.AssignmentID, BlockedBy: "host memory", Why: engineeringWaitHostMemory, HostMemoryReason: "over the floor"}
	rt.observeAdmissionWait(ctx, assignment, busy, now, 30*time.Second)
	now = now.Add(2 * time.Minute)
	rt.endAssignmentWait(ctx, assignment.AssignmentID, assignment.Binding.Attempt, "admitted")
	rt.markAssignmentSpawn(assignment.AssignmentID, assignment.Binding.Attempt, 20*time.Second)
	rt.closeAssignmentWaits(ctx, assignment)
	summary, err := s.GetCommitmentRecord(ctx, seed.OwnerID, seed.ComputerID, assignmentWaitSummaryRecordID(assignment.AssignmentID, assignment.Binding.Attempt))
	if err != nil || summary == nil || summary.SubjectRefs[types.RoundRefSpawnS] != "20" || summary.SubjectRefs[types.RoundRefJobWaitedS] != "0" {
		t.Fatalf("P1: summary = %+v %v", summary, err)
	}

	now = start.Add(10 * time.Minute)
	op := selfdev.Operation{OperationID: "op-round-1", ComputerID: seed.ComputerID, IdempotencyKey: store.EngineeringFreezeIdempotencyKey(assignment.AssignmentID),
		CreatedAt: start.Add(4 * time.Minute).Format(time.RFC3339Nano)}
	rec := rt.recordRoundReceipt(ctx, seed.OwnerID, op, types.OwnerActReject)
	if rec == nil || rec.SubjectRefs["spawn_s"] != "20" || rec.SubjectRefs["waited_s:host_memory"] != "120" || rec.SubjectRefs["outcome"] != "reject" ||
		rec.SubjectRefs["total_s"] == "" || rec.SubjectRefs["platform_pct"] == "" {
		t.Fatalf("P1: receipt = %+v", rec)
	}
	if rec.SubjectRefs["freeze_to_decision_s"] != "360" {
		t.Fatalf("operation created_at is the Freeze when no prediction settled: %v", rec.SubjectRefs)
	}
	if rt.recordRoundReceipt(ctx, seed.OwnerID, op, types.OwnerActReject) == nil {
		t.Fatal("P3: replay returned nothing")
	}
	line := rt.roundLineSentence(ctx, seed.OwnerID, seed.ComputerID, op, types.OwnerActReject)
	if !strings.Contains(line, "round took ") || !strings.Contains(line, " platform (") || rt.roundLineSentence(ctx, seed.OwnerID, seed.ComputerID, op, types.OwnerActApprove) != "" {
		t.Fatalf("P3: line = %q", line)
	}
	packRecords, _ := s.ListCommitmentClaimRecords(ctx, seed.OwnerID, seed.ComputerID, 256)
	pack := types.BuildActingPack(packRecords, persistentManagementAgentID(seed.OwnerID), 50)
	found := false
	for _, it := range pack.Items {
		found = found || (it.RecordID == types.RoundReceiptRecordID("op-round-1") && strings.Contains(it.Claim, "round took"))
	}
	if !found {
		t.Fatalf("P2: not in management's pack: %+v", pack.Items)
	}
	other := op
	other.OperationID, other.IdempotencyKey = "op-other", "rebase:x"
	if rt.recordRoundReceipt(ctx, seed.OwnerID, other, types.OwnerActReject) != nil {
		t.Fatal("P4: a receipt for a non-engineering operation")
	}
}

// F1 the Freeze is the successful one the freeze_minutes prediction settled
// at, not the operation's creation (a refused first Freeze, a re-Freeze).
func TestRoundReceiptFreezeIsThePredictionsFreeze(t *testing.T) {
	rt, s, _ := c1eRuntime(t)
	ctx := context.Background()
	seed, err := store.SeedEngineeringAssignmentAuthority(s, "owner-round2", rt.TextureComputerID(), 1)
	if err != nil {
		t.Fatal(err)
	}
	assignment, _ := c1eOpenBound(t, s, seed, 0)
	start := time.Now().UTC()
	now := start.Add(10 * time.Minute)
	waitTestClock(rt, &now)
	freezeAt := start.Add(8 * time.Minute).Format(time.RFC3339Nano)
	resolveID := types.RuleResolveRecordID(roundPredictionRecordID(assignment.Binding.ParentControlID, types.PrecommitRuleFreezeMinutes))
	if _, err := s.AppendCommitmentRecord(ctx, seed.OwnerID, seed.ComputerID, types.CommitmentRecord{
		SchemaID: types.CommitmentRecordSchemaV1, RecordID: resolveID, Kind: types.CommitmentKindResolve,
		Discrepancy: types.DiscrepancyUnresolved, Provenance: types.CommitmentProvenance{AgentID: types.RuleScorerAgentID, CommittedAt: freezeAt},
		SubjectRefs: map[string]string{types.RuleRefFreezeAt: freezeAt},
	}); err != nil {
		t.Fatal(err)
	}
	op := selfdev.Operation{OperationID: "op-round-2", ComputerID: seed.ComputerID, IdempotencyKey: store.EngineeringFreezeIdempotencyKey(assignment.AssignmentID),
		CreatedAt: start.Add(4 * time.Minute).Format(time.RFC3339Nano)}
	rec := rt.recordRoundReceipt(ctx, seed.OwnerID, op, types.OwnerActReject)
	if rec == nil || rec.SubjectRefs["freeze_to_decision_s"] != "120" {
		t.Fatalf("F1: freeze_to_decision = %v", rec)
	}
}
