package agentcore

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/visualcapture"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// Engineering's rule-settled precommitments (types/commitment_rules.go,
// failure modes R1-R12). Engineering stages choir.Precommit with a rule; the
// reducer stamps the assignment and its opening time and mints a ledger-only
// precommit record. The runtime then settles each one from receipts it
// holds: commitFreezeIntent (freeze_within, tests_pass, paths_confined),
// commitVerifyIntent (verify_pass) and every terminal transition
// (completes, and whatever is still open). Settlement is best effort (R6):
// it logs and never fails the act it observes.

// freezeSourcePathsKey carries the frozen change's source paths from
// freezeCapsuleEffectBundle to commitFreezeIntent; it is removed before the
// freeze result reaches the cell, so the model never sees it.
const freezeSourcePathsKey = "_rule_source_paths"

// capsuleSourcePrefix is where the platform source sits in a capsule diff.
const capsuleSourcePrefix = "workspace/platform/"

// capsuleSourcePaths lists the changed source files of a capsule diff,
// relative to /workspace/platform: build outputs outside it (the release
// tree, caches) and directories are not source changes (R9).
func capsuleSourcePaths(changes []capsule.FileChange) []string {
	seen := map[string]bool{}
	var out []string
	for _, change := range changes {
		if change.Mode.IsDir() {
			continue
		}
		rel, ok := strings.CutPrefix(strings.TrimPrefix(change.Path, "/"), capsuleSourcePrefix)
		if !ok || rel == "" || seen[rel] {
			continue
		}
		seen[rel] = true
		out = append(out, rel)
	}
	sort.Strings(out)
	return out
}

// validateEngineeringPrecommit is the reducer's pre-commit check of a staged
// Precommit (R3): an engineering Precommit must be rule-settled and may not
// share a cell with Freeze or Complete (R2); any other desk's Precommit may
// not carry a rule, because only the reducer can bind a rule to an
// assignment (R1).
func validateEngineeringPrecommit(fromDesk string, in yaegikernel.StagedIntent, intents []yaegikernel.StagedIntent) error {
	var precommit types.CommitmentPrecommit
	if err := json.Unmarshal([]byte(in.Precommit), &precommit); err != nil {
		return fmt.Errorf("reduce: precommit %s: invalid typed body: %w", in.LocalID, err)
	}
	if fromDesk != agentprofile.Engineering {
		if precommit.Rule != nil {
			return fmt.Errorf("reduce: precommit %s: rule-settled precommits are engineering's", in.LocalID)
		}
		return nil
	}
	if err := types.ValidateRulePrecommit(precommit); err != nil {
		return fmt.Errorf("reduce: precommit %s: %w", in.LocalID, err)
	}
	for _, other := range intents {
		if other.Kind == yaegikernel.IntentFreeze || other.Kind == yaegikernel.IntentComplete {
			return fmt.Errorf("reduce: precommit %s: precommit before the evidence, not in the cell that stages %s", in.LocalID, other.Kind)
		}
	}
	return nil
}

// engineeringRulePrecommitRecord derives the ledger record for an
// engineering Precommit: the generic precommit record, unaddressed
// (ledger-only), with the assignment and its opening time stamped onto the
// rule (R1) and the run's model recorded.
func engineeringRulePrecommitRecord(scope ReductionScope, in yaegikernel.StagedIntent, assignment types.EngineeringAssignment, modelID string) (types.CommitmentRecord, error) {
	rec := commitmentRecordForIntent(scope, in)
	if rec.Precommit == nil || rec.Precommit.Rule == nil {
		return types.CommitmentRecord{}, fmt.Errorf("reduce: precommit %s: a rule is required", in.LocalID)
	}
	if err := types.ValidateRulePrecommit(*rec.Precommit); err != nil {
		return types.CommitmentRecord{}, fmt.Errorf("reduce: precommit %s: %w", in.LocalID, err)
	}
	rule := *rec.Precommit.Rule
	rule.AssignmentID = assignment.AssignmentID
	rule.Since = assignment.CreatedAt.UTC().Format(time.RFC3339Nano)
	if len(rule.Paths) > 0 {
		paths := make([]string, 0, len(rule.Paths))
		for _, raw := range rule.Paths {
			clean, err := types.NormalizeRulePath(raw)
			if err != nil {
				return types.CommitmentRecord{}, fmt.Errorf("reduce: precommit %s: %w", in.LocalID, err)
			}
			paths = append(paths, clean)
		}
		rule.Paths = paths
	}
	if len(rule.Routes) > 0 {
		routes := make([]string, 0, len(rule.Routes))
		for _, raw := range rule.Routes {
			clean, err := visualcapture.NormalizeRoute(raw)
			if err != nil {
				return types.CommitmentRecord{}, fmt.Errorf("reduce: precommit %s: %w", in.LocalID, err)
			}
			routes = append(routes, clean)
		}
		rule.Routes = routes
	}
	rec.Precommit.Rule = &rule
	if len(rec.Prediction.Questions) == 1 {
		rec.Prediction.Questions[0].Answers = []string{types.PrecommitAnswerYes, types.PrecommitAnswerNo}
	}
	rec.Addressee = ""
	rec.Provenance.ModelID = strings.TrimSpace(modelID)
	rec.EmitterKind = types.CommitmentEmitterAgent
	rec.ComputerID = scope.ComputerID
	rec.SubjectRefs = map[string]string{"assignment": assignment.AssignmentID}
	return rec, nil
}

// commitEngineeringPrecommit mints an engineering run's rule precommit. It
// is refused once the assignment is terminal or its candidate froze (R2).
func (r *rlmCallReduction) commitEngineeringPrecommit(ctx context.Context, in yaegikernel.StagedIntent) (uint64, error) {
	rt := r.rt()
	if rt == nil || rt.store == nil || r.ledger == nil || r.rec == nil {
		return 0, fmt.Errorf("reduce: precommit %s without assignment authority", in.LocalID)
	}
	assignmentID := metadataStringValue(r.rec.Metadata, "assignment_id")
	attempt := uint64(metadataIntValue(r.rec.Metadata, "assignment_attempt"))
	if assignmentID == "" || attempt == 0 {
		return 0, fmt.Errorf("reduce: precommit %s: the run is bound to no assignment", in.LocalID)
	}
	assignment, err := rt.store.GetEngineeringAssignment(ctx, r.scope.OwnerID, r.scope.ComputerID, assignmentID, attempt)
	if err != nil {
		return 0, fmt.Errorf("reduce: precommit %s: assignment %s: %w", in.LocalID, assignmentID, err)
	}
	if assignment.Disposition.Terminal() {
		return 0, fmt.Errorf("reduce: precommit %s: assignment %s is %s", in.LocalID, assignmentID, assignment.Disposition)
	}
	if rt.selfdevOperations != nil {
		operation, found, opErr := rt.selfdevOperations.GetByIdempotency(ctx, r.scope.ComputerID, store.EngineeringFreezeIdempotencyKey(assignmentID))
		if opErr != nil {
			return 0, fmt.Errorf("reduce: precommit %s: %w", in.LocalID, opErr)
		}
		if found && strings.TrimSpace(operation.BundleDigest) != "" {
			return 0, fmt.Errorf("reduce: precommit %s: your candidate is already frozen; precommit before the evidence", in.LocalID)
		}
	}
	rec, err := engineeringRulePrecommitRecord(r.scope, in, assignment, metadataStringValue(r.rec.Metadata, runMetadataModel))
	if err != nil {
		return 0, err
	}
	if _, err := r.ledger.AppendCommitmentRecord(ctx, r.scope.OwnerID, r.scope.ComputerID, rec); err != nil {
		return 0, fmt.Errorf("reduce: precommit %s: %w", in.LocalID, err)
	}
	return 0, nil
}

// settleRulePrecommits runs one settlement pass for an assignment on a
// detached, bounded context and logs instead of failing (R6).
func (rt *Runtime) settleRulePrecommits(ctx context.Context, ownerID, computerID, assignmentID, cause string, at time.Time, settle func(types.PrecommitRule) (types.RuleSettlement, bool)) {
	if rt == nil || rt.store == nil || strings.TrimSpace(assignmentID) == "" {
		return
	}
	settleCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
	defer cancel()
	settled, err := rt.store.SettleRulePrecommits(settleCtx, ownerID, computerID, assignmentID, at, settle)
	if err != nil {
		log.Printf("runtime: assignment %s rule precommit settlement at %s: %v", assignmentID, cause, err)
		return
	}
	for _, rec := range settled {
		verdict := ""
		if rec.Resolve != nil {
			verdict = rec.Resolve.Verdict
		}
		log.Printf("runtime: assignment %s rule precommit %s settled %s at %s", assignmentID, rec.ParentID, verdict, cause)
	}
}

// settleFreezePrecommits settles what a fresh, successful Freeze decides.
// out is the freeze result; only a fresh freeze carries the source paths, so
// a replayed freeze (an already frozen operation) settles nothing.
func (r *rlmCallReduction) settleFreezePrecommits(ctx context.Context, in yaegikernel.StagedIntent, out map[string]any, assignmentID string, frozenAt time.Time) {
	paths, fresh := out[freezeSourcePathsKey].([]string)
	delete(out, freezeSourcePathsKey)
	var visual types.EngineeringFreezeObservation
	applyFreezeVisual(&visual, out)
	rt := r.rt()
	// Station TE: a fresh Freeze that observed captures asks Texture to
	// embed them; the operation is the Freeze's own result, never the cell's.
	if fresh && visual.VisualKnown && len(visual.VisualCaptures) > 0 {
		operationID, _ := out["operation_id"].(string)
		selfdevComputerID := ""
		if r.toolCtx != nil {
			selfdevComputerID = r.toolCtx.ComputerID
		}
		rt.requestCandidateEvidenceEmbeds(ctx, r.scope.OwnerID, r.scope.ComputerID, selfdevComputerID, operationID)
	}
	if !fresh || rt == nil || r.toolCtx == nil || r.toolCtx.Executor == nil {
		return
	}
	obs := types.EngineeringFreezeObservation{
		FrozenAt: frozenAt, SourcePaths: paths, SourcePathsKnown: true,
		VisualKnown: visual.VisualKnown, VisualReceipt: visual.VisualReceipt, VisualCaptures: visual.VisualCaptures,
	}
	obs.BundleDigest, _ = out["bundle_digest"].(string)
	obs.OperationID, _ = out["operation_id"].(string)
	refs := trimNonEmptyStrings(in.TestReceipts)
	receipts, err := r.toolCtx.Executor.ResolveExecutionReceipts(refs)
	if err != nil || len(receipts) != len(refs) {
		log.Printf("runtime: assignment %s rule precommit freeze settlement: test receipts unavailable: %v", assignmentID, err)
		return
	}
	for i, receipt := range receipts {
		obs.TestReceipts = append(obs.TestReceipts, types.EngineeringReceiptExit{Ref: refs[i], ExitCode: receipt.ExitCode})
	}
	rt.settleRulePrecommits(ctx, r.scope.OwnerID, r.scope.ComputerID, assignmentID, "freeze", frozenAt, func(rule types.PrecommitRule) (types.RuleSettlement, bool) {
		return types.SettleRuleOnFreeze(rule, obs)
	})
}

// settleEngineeringPrecommitsAtTerminal settles what an assignment's
// terminal disposition decides. A candidate the assignment froze keeps
// verify_pass open for management's Verify (R10) and marks a missing freeze
// settlement unresolved (R5).
func (rt *Runtime) settleEngineeringPrecommitsAtTerminal(ctx context.Context, assignment types.EngineeringAssignment, reason string) {
	if rt == nil || !assignment.Disposition.Terminal() {
		return
	}
	obs := types.EngineeringTerminalObservation{Disposition: assignment.Disposition, Reason: reason, At: time.Now().UTC()}
	if rt.selfdevOperations != nil {
		operation, found, err := rt.selfdevOperations.GetByIdempotency(context.WithoutCancel(ctx), assignment.Binding.ComputerID, store.EngineeringFreezeIdempotencyKey(assignment.AssignmentID))
		if err == nil && found && strings.TrimSpace(operation.BundleDigest) != "" {
			obs.CandidateFrozen = true
			obs.CandidateRef = "selfdev-operation:" + operation.OperationID
		}
	}
	rt.settleRulePrecommits(ctx, assignment.Binding.OwnerID, assignment.Binding.ComputerID, assignment.AssignmentID, "terminal "+string(assignment.Disposition), obs.At,
		func(rule types.PrecommitRule) (types.RuleSettlement, bool) {
			return types.SettleRuleAtTerminal(rule, obs)
		})
}
