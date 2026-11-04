package agentcore

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/store"
)

// Owner ruling 2026-11-04: "Rejected rounds don't complete but get sent
// back." (docs/problems/texture-settles-rejected-round-as-complete-2026-11-04.md)
//
// Ways the fix can fail, written before it:
//  1. A rejected last candidate reads as nothing, so Texture's Done settles it.
//  2. The rejection's reasons are lost, so the send-back carries none.
//  3. A rejection with attempts left reads as exhausted (ends early), or the
//     exhausted budget reads as sendable (retries forever).
//  4. An owner abandon, a crash close or a live candidate reads as a rejection.
//  5. A pass after rejections still reads as a rejected round.

func rejectedOp(id string, n int, state, terminal string, verifiers ...string) selfdev.Operation {
	return selfdev.Operation{
		OperationID: id, BundleDigest: "bundle-" + id, State: state, TerminalError: terminal, VerifierRefs: verifiers,
		CreatedAt: time.Date(2026, 11, 4, 10, n, 0, 0, time.UTC).Format(time.RFC3339Nano),
	}
}

func judged(id string, n int, reason string) selfdev.Operation {
	return rejectedOp(id, n, selfdev.StateFailed, selfDevelopmentJudgmentRejected+": "+reason, "ref-"+id)
}

func TestRejectedLastCandidateIsASendBackWithItsReasons(t *testing.T) {
	round, ok := rejectedRoundFrom([]selfdev.Operation{judged("op1", 1, "shot 3 clipped: question 2 no")})
	if !ok {
		t.Fatal("a judgment-rejected last candidate is not a rejected round")
	}
	if round.Exhausted || round.Attempts != 1 || round.Budget != store.EngineeringFreezeMaxRevisions {
		t.Fatalf("round = %+v, want attempt 1 of the budget, not exhausted", round)
	}
	if !strings.Contains(round.Reasons, "shot 3 clipped") || strings.Contains(round.Reasons, selfDevelopmentJudgmentRejected) {
		t.Fatalf("reasons = %q, want the judge's reason without the fixed prefix", round.Reasons)
	}
}

func TestAttemptBudgetEndsTheRoundRejectedNeverComplete(t *testing.T) {
	var ops []selfdev.Operation
	for i := 1; i <= store.EngineeringFreezeMaxRevisions; i++ {
		ops = append(ops, judged("op"+string(rune('0'+i)), i, "still wrong"))
	}
	round, ok := rejectedRoundFrom(ops[:len(ops)-1])
	if !ok || round.Exhausted {
		t.Fatalf("one attempt short of the budget = %+v, %v; want sendable", round, ok)
	}
	round, ok = rejectedRoundFrom(ops)
	if !ok || !round.Exhausted || round.Attempts != store.EngineeringFreezeMaxRevisions {
		t.Fatalf("budget spent = %+v, %v; want exhausted", round, ok)
	}
	if got := round.Fate(); !strings.HasPrefix(got, "rejected after 6 attempts") || !strings.Contains(got, "still wrong") {
		t.Fatalf("fate = %q, want \"rejected after 6 attempts\" with the reason", got)
	}
}

func TestOwnerAndConsensusRejectionIsSentBackToo(t *testing.T) {
	op := rejectedOp("op1", 1, selfdev.StateRejected, "")
	op.DecisionActor = "consensus"
	round, ok := rejectedRoundFrom([]selfdev.Operation{op})
	if !ok || round.Exhausted || round.Reasons == "" {
		t.Fatalf("rejected = %+v, %v; want a sendable round that names where the note is", round, ok)
	}
}

func TestOnlyRealRejectionsCountAsRejectedRounds(t *testing.T) {
	cases := map[string][]selfdev.Operation{
		"no operations":  nil,
		"live candidate": {judged("op1", 1, "bad"), rejectedOp("op2", 2, selfdev.StateExecuting, "")},
		// An owner abandon and a crash close fail the operation with no
		// verdict: no verifier refs, and not the judgment's words.
		"owner abandon": {rejectedOp("op1", 1, selfdev.StateFailed, "abandoned by the owner: stop")},
		"crash close":   {rejectedOp("op1", 1, selfdev.StateFailed, "interrupted by a restart")},
		// A close whose reason merely begins with the verdict's words.
		"look-alike close":   {rejectedOp("op1", 1, selfdev.StateFailed, selfDevelopmentJudgmentRejected+": closed")},
		"pass after rejects": {judged("op1", 1, "bad"), rejectedOp("op2", 2, selfdev.StateAwaitingApproval, "", "ref-2")},
		"applied":            {rejectedOp("op1", 1, selfdev.StateApplied, "", "ref")},
	}
	for name, ops := range cases {
		if round, ok := rejectedRoundFrom(ops); ok {
			t.Errorf("%s read as a rejected round: %+v", name, round)
		}
	}
}

func TestRejectedRoundOfReadsTheTrajectoryAndHonorsAnOwnerAbandon(t *testing.T) {
	rt, productStore := testRuntime(t)
	rt.cfg.ComputerID = "computer-sendback"
	operations, err := selfdev.NewStore(productStore, productStore)
	if err != nil {
		t.Fatal(err)
	}
	rt.selfdevOperations = operations
	now := time.Now().UTC()
	if _, err := rt.store.DB().Exec(`INSERT INTO self_development_operations (operation_id, computer_id, idempotency_key, request_commitment, trajectory_id, base_head, prompt_artifact_ref, bundle_digest, verifier_refs_json, desired_head, effective_head, state, terminal_error, created_at, updated_at) VALUES ('op-r', 'computer-sendback', 'key-r', '', 'traj-r', '', '', 'b1', '["ref"]', '', '', ?, ?, ?, ?)`,
		selfdev.StateFailed, selfDevelopmentJudgmentRejected+": shot 1 no", now, now); err != nil {
		t.Fatal(err)
	}
	round, ok := rt.RejectedRoundOf(context.Background(), "owner", "computer-sendback", "traj-r")
	if !ok || round.OperationID != "op-r" || round.Exhausted {
		t.Fatalf("RejectedRoundOf = %+v, %v; want op-r sendable", round, ok)
	}
	if _, ok := rt.RejectedRoundOf(context.Background(), "owner", "computer-sendback", "traj-other"); ok {
		t.Fatal("another trajectory read as rejected")
	}
}
