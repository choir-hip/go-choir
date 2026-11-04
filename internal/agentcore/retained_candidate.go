package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Freeze ends the build budget
// (docs/problems/frozen-candidate-dies-with-build-deadline-2026-11-04.md).
//
// An engineering round's run carries its build's wall clock: the 60-minute
// activation budget and the assignment deadline. A candidate the round
// froze inside that clock is finished work, and its decision (management's
// Verify, then the gate) is a separate phase. Before this, the build's
// clock ending after the Freeze cancelled the assignment and failed the
// frozen operation through failBoundSelfdevOperation, and management could
// not Verify it anyway because Verify admitted only a completed
// assignment. So a good candidate died waiting for its own decision.
//
// Now, when the build ends the round (withBuildBudgetEnd: a context mark the
// runtime sets on every end of the engineer's run and at the assignment
// deadline, never a reason text) and its operation is frozen, the runtime writes a retention
// record (<updater root>/retained/<operation>.json) before the cancel, and:
//   - the operation stays frozen; the assignment is cancelled with a reason
//     that names the candidate and its bundle, and that reason is the final
//     Report management receives;
//   - management's Verify admits a cancelled round's candidate only when its
//     retention record names that assignment and bundle (commitVerifyIntent),
//     so a cancel whose best-effort close failed never becomes verifiable;
//   - the round's verify_pass precommit stays open for that Verify;
//   - the wait for the decision is a typed wait (WaitKindCandidateDecision)
//     with its own budget, from the record, in management's pack, and a
//     start and end Note to the Texture the round serves. When the budget
//     runs out the candidate stays frozen and retained, and the wait says it
//     is at its bound;
//   - a crash restart fails every retained candidate still open, from the
//     records (closeRetainedCandidatesAtCrashBoot): the restart rule.

const (
	// activationBudgetExceededReason is the run's fate when its activation
	// budget ends it (the context deadline, or the durable
	// activation_budget_deadline wake).
	activationBudgetExceededReason = "activation budget exceeded: progress deadline reached"
	// assignmentDeadlineExpiredPrefix begins the assignment deadline's
	// cancellation reason (HandleAssignedEngineeringFateDeadline).
	assignmentDeadlineExpiredPrefix = "assignment deadline expired after "
	// candidateDecisionBudget bounds a retained candidate's decision wait,
	// from its retention. It is the length of the deploy-refresh hold a
	// parked candidate keeps (selfdevDecisionHoldCap); that hold counts
	// from the operation's last transition (the Freeze), so it can end
	// before the decision budget does.
	candidateDecisionBudget = selfdevDecisionHoldCap
	// candidateDecisionEpisode names the decision wait's relay episode; an
	// attempt has at most one retained candidate.
	candidateDecisionEpisode = "candidate-decision"

	retainedCandidateSchema = "choir.retained_candidate.v1"
)

// buildBudgetEndKey marks a context whose cancellation is the build ending:
// the engineer's run ended (terminalizeRun sets it for every run end: its
// activation budget, a loop or cell failure, a run cancel, a drain, no live
// loop) or the assignment deadline. After a Freeze none of these fails the
// candidate. Only the runtime sets it; no model text reaches it.
type buildBudgetEndKey struct{}

// withBuildBudgetEnd marks ctx as the build's clock ending the round.
func withBuildBudgetEnd(ctx context.Context) context.Context {
	return context.WithValue(ctx, buildBudgetEndKey{}, true)
}

// buildBudgetEnded reports whether ctx carries the build-end mark.
func buildBudgetEnded(ctx context.Context) bool {
	marked, _ := ctx.Value(buildBudgetEndKey{}).(bool)
	return marked
}

// retainedCandidate is the durable record that a frozen candidate was kept
// when its round's build clock ended.
type retainedCandidate struct {
	Schema       string    `json:"schema"`
	OperationID  string    `json:"operation_id"`
	ComputerID   string    `json:"computer_id"`
	OwnerID      string    `json:"owner_id"`
	TrajectoryID string    `json:"trajectory_id"`
	AssignmentID string    `json:"assignment_id"`
	Attempt      uint64    `json:"attempt"`
	BundleDigest string    `json:"bundle_digest"`
	TextureInbox string    `json:"texture_inbox,omitempty"`
	RetainedAt   time.Time `json:"retained_at"`
}

func retainedDir(root string) string { return filepath.Join(root, "retained") }

func retainedPath(root, operationID string) string {
	return filepath.Join(retainedDir(root), url.PathEscape(operationID)+".json")
}

// writeRetainedCandidate writes the record atomically (temp file, rename).
func writeRetainedCandidate(root string, rec retainedCandidate) error {
	if strings.TrimSpace(root) == "" || strings.TrimSpace(rec.OperationID) == "" {
		return fmt.Errorf("retained candidate: no updater root or operation")
	}
	if err := os.MkdirAll(retainedDir(root), 0o700); err != nil {
		return err
	}
	body, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(retainedDir(root), ".retained-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(body); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), retainedPath(root, rec.OperationID))
}

// readRetainedCandidate reads one record; found is false when there is none.
func readRetainedCandidate(root, operationID string) (retainedCandidate, bool, error) {
	if strings.TrimSpace(root) == "" || strings.TrimSpace(operationID) == "" {
		return retainedCandidate{}, false, nil
	}
	body, err := os.ReadFile(retainedPath(root, operationID))
	if errors.Is(err, os.ErrNotExist) {
		return retainedCandidate{}, false, nil
	}
	if err != nil {
		return retainedCandidate{}, false, err
	}
	var rec retainedCandidate
	if err := json.Unmarshal(body, &rec); err != nil || rec.Schema != retainedCandidateSchema || rec.OperationID != operationID {
		return retainedCandidate{}, false, fmt.Errorf("retained candidate %s: unreadable record", operationID)
	}
	return rec, true, nil
}

// listRetainedCandidates reads every record; an unreadable one is logged
// and skipped.
func listRetainedCandidates(root string) []retainedCandidate {
	if strings.TrimSpace(root) == "" {
		return nil
	}
	entries, err := os.ReadDir(retainedDir(root))
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			log.Printf("selfdev: list retained candidates: %v", err)
		}
		return nil
	}
	var out []retainedCandidate
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || strings.HasPrefix(name, ".") || !strings.HasSuffix(name, ".json") {
			continue
		}
		operationID, err := url.PathUnescape(strings.TrimSuffix(name, ".json"))
		if err != nil {
			continue
		}
		rec, found, err := readRetainedCandidate(root, operationID)
		if err != nil {
			log.Printf("selfdev: %v", err)
			continue
		}
		if found {
			out = append(out, rec)
		}
	}
	return out
}

// retainedFor is the retention record of the assignment's candidate, when it
// names that assignment and bundle.
func (rt *Runtime) retainedFor(operation selfdev.Operation, assignmentID string) (retainedCandidate, bool) {
	if rt == nil {
		return retainedCandidate{}, false
	}
	rec, found, err := readRetainedCandidate(rt.selfdevUpdaterRoot, operation.OperationID)
	if err != nil {
		log.Printf("selfdev: %v", err)
		return retainedCandidate{}, false
	}
	if !found || rec.AssignmentID != strings.TrimSpace(assignmentID) || rec.BundleDigest != operation.BundleDigest || rec.ComputerID != operation.ComputerID {
		return retainedCandidate{}, false
	}
	return rec, true
}

// frozenCandidateOf is the assignment's own operation when it is frozen.
func (rt *Runtime) frozenCandidateOf(ctx context.Context, computerID, assignmentID string) (selfdev.Operation, bool) {
	if rt == nil || rt.selfdevOperations == nil || strings.TrimSpace(computerID) == "" || strings.TrimSpace(assignmentID) == "" {
		return selfdev.Operation{}, false
	}
	operation, found, err := rt.selfdevOperations.GetByIdempotency(ctx, computerID, store.EngineeringFreezeIdempotencyKey(assignmentID))
	if err != nil {
		log.Printf("selfdev: read the candidate of assignment %s: %v", assignmentID, err)
		return selfdev.Operation{}, false
	}
	if !found || operation.State != selfdev.StateFrozen || strings.TrimSpace(operation.BundleDigest) == "" {
		return selfdev.Operation{}, false
	}
	return operation, true
}

// decideRetention decides, before a build-end cancel is sent, whether the
// round's frozen candidate is retained, and returns the reason the cancel
// carries. It writes nothing: only the cancel's committer writes the record
// (recordRetention), so no uncommitted or lost cancel can leave one behind
// (review r3, devin F1 F2).
func (rt *Runtime) decideRetention(ctx context.Context, assignment types.EngineeringAssignment, reason string) (selfdev.Operation, string, bool) {
	if !buildBudgetEnded(ctx) || rt == nil || strings.TrimSpace(rt.selfdevUpdaterRoot) == "" {
		return selfdev.Operation{}, reason, false
	}
	operation, frozen := rt.frozenCandidateOf(ctx, assignment.Binding.ComputerID, assignment.AssignmentID)
	if !frozen {
		return selfdev.Operation{}, reason, false
	}
	return operation, retainedCandidateReason(reason, operation, assignment.AssignmentID), true
}

// recordRetention writes the retention record after the cancel that names
// the candidate committed. A record already there keeps its time. A write
// that fails leaves the candidate frozen and unverifiable; the crash boot
// closes it (closeRetainedCandidatesAtCrashBoot) and the owner can abandon
// it.
func (rt *Runtime) recordRetention(assignment types.EngineeringAssignment, operation selfdev.Operation) bool {
	if _, ok := rt.retainedFor(operation, assignment.AssignmentID); ok {
		return true
	}
	binding := assignment.Binding
	textureInbox, _ := types.ConversationCounterpart(binding.ParentConversationID, binding.ParentAgentID)
	rec := retainedCandidate{
		Schema: retainedCandidateSchema, OperationID: operation.OperationID, ComputerID: binding.ComputerID, OwnerID: binding.OwnerID,
		TrajectoryID: binding.TrajectoryID, AssignmentID: assignment.AssignmentID, Attempt: binding.Attempt,
		BundleDigest: operation.BundleDigest, TextureInbox: textureInbox, RetainedAt: rt.assignmentWaitNow().UTC(),
	}
	if err := writeRetainedCandidate(rt.selfdevUpdaterRoot, rec); err != nil {
		log.Printf("selfdev: %s: retention record not written; the candidate stays frozen and cannot be verified: %v", operation.OperationID, err)
		return false
	}
	return true
}

// retainedCandidateReason is the cancellation reason of a round whose build
// ended after its Freeze: it names the retained candidate, its bundle, and
// the act that decides it.
func retainedCandidateReason(reason string, operation selfdev.Operation, assignmentID string) string {
	return fmt.Sprintf("%s; the build ended after its Freeze, so its frozen candidate %s (bundle %s) is retained for Verify, not failed: "+
		"choir.Verify(%q, ...) with that bundle digest decides it, within its decision budget of %s",
		strings.TrimSpace(reason), operation.OperationID, operation.BundleDigest, assignmentID, candidateDecisionBudget)
}

// retainCandidateAtBuildEnd is the build-end alternative to
// failBoundSelfdevOperation, after the cancel committed: the frozen
// operation is left frozen, and the Texture the round serves is told its
// decision wait began. A candidate that left frozen in the meantime (an
// owner abandon) is not announced.
func (rt *Runtime) retainCandidateAtBuildEnd(ctx context.Context, assignment types.EngineeringAssignment, operation selfdev.Operation) {
	if !rt.recordRetention(assignment, operation) {
		return
	}
	rec, ok := rt.retainedFor(operation, assignment.AssignmentID)
	if !ok {
		return
	}
	if _, frozen := rt.frozenCandidateOf(ctx, assignment.Binding.ComputerID, assignment.AssignmentID); !frozen {
		log.Printf("selfdev: %s left frozen before its retention was announced", operation.OperationID)
		return
	}
	log.Printf("selfdev: %s retained frozen: assignment %s ended by its build budget after its Freeze; its decision has its own budget (%s)",
		operation.OperationID, assignment.AssignmentID, candidateDecisionBudget)
	now := rt.assignmentWaitNow()
	w := candidateDecisionWait(rec, now)
	rt.mintCandidateDecisionRelay(ctx, rec, types.WaitRelayStart, w.TextureLine(now)+"; the build ended after its Freeze and the candidate is kept", now)
}

// candidateDecisionWait is a retained candidate's wait for its decision, as
// a typed wait.
func candidateDecisionWait(rec retainedCandidate, now time.Time) *types.WaitState {
	since := rec.RetainedAt.UTC()
	if since.IsZero() {
		since = now.UTC()
	}
	w := &types.WaitState{
		Kind: types.WaitKindCandidateDecision, Class: types.WaitClassPolicy,
		Subject:  "operation " + rec.OperationID,
		Resource: "management's Verify of frozen candidate " + rec.OperationID,
		Holders:  []types.WaitHolder{{Who: "management", What: "Verify"}},
		Since:    since, Until: since.Add(candidateDecisionBudget),
		Detail: fmt.Sprintf("assignment %s ended by its build budget after its Freeze; the candidate (bundle %s) is retained frozen for Verify",
			rec.AssignmentID, rec.BundleDigest),
	}
	if !now.Before(w.Until) {
		w.Detail += "; the decision budget ran out, and the candidate stays frozen and retained until management verifies it or the owner abandons it"
	}
	return w
}

// mintCandidateDecisionRelay tells the Texture the round serves that its
// candidate's decision wait began or ended, through the wait relay's path
// and switch.
func (rt *Runtime) mintCandidateDecisionRelay(ctx context.Context, rec retainedCandidate, phase, body string, now time.Time) {
	if rec.TextureInbox == "" {
		return
	}
	relay, ok := types.NewEngineeringWaitRelay(agentprofile.Engineering+":"+rec.AssignmentID, rec.TextureInbox, rec.ComputerID, rec.Attempt,
		candidateDecisionEpisode, phase, body, now, map[string]string{"wait_kind": string(types.WaitKindCandidateDecision), "operation_id": rec.OperationID})
	if !ok {
		return
	}
	rt.mintWaitRelays(ctx, []waitRelayPlan{{ownerID: rec.OwnerID, computerID: rec.ComputerID, rec: relay}}, "")
}

// endRetainedCandidateWait closes a retained candidate's decision wait for
// Texture once management's verdict committed.
func (rt *Runtime) endRetainedCandidateWait(ctx context.Context, rec retainedCandidate, decision string) {
	now := rt.assignmentWaitNow()
	waited := now.Sub(rec.RetainedAt)
	body := fmt.Sprintf("Waited %d min for management's Verify of frozen candidate %s; the verdict is %s.",
		max(int((waited+30*time.Second)/time.Minute), 1), rec.OperationID, strings.TrimSpace(decision))
	rt.mintCandidateDecisionRelay(ctx, rec, types.WaitRelayEnd, body, now)
}

// candidateDecisionWaitViews is management's view of the computer's
// retained candidates still frozen, one per candidate, from the records, so
// it survives a planned restart. The pack is context, never a gate.
func (rt *Runtime) candidateDecisionWaitViews(ctx context.Context, ownerID, computerID string) []types.AssignmentWaitView {
	if rt == nil || rt.selfdevOperations == nil {
		return nil
	}
	now := rt.assignmentWaitNow()
	var out []types.AssignmentWaitView
	for _, rec := range listRetainedCandidates(rt.selfdevUpdaterRoot) {
		if rec.OwnerID != ownerID || rec.ComputerID != computerID {
			continue
		}
		operation, err := rt.selfdevOperations.Get(ctx, computerID, rec.OperationID)
		if err != nil || operation.State != selfdev.StateFrozen || operation.BundleDigest != rec.BundleDigest {
			continue
		}
		w := candidateDecisionWait(rec, now)
		w.Stamp(now)
		out = append(out, types.AssignmentWaitView{AssignmentID: rec.AssignmentID, Attempt: rec.Attempt, Wait: w})
	}
	return out
}

// closeRetainedCandidatesAtCrashBoot fails, at a crash boot, every retained
// candidate still waiting for Verify (frozen), and removes the records of
// those past it. A crash never resumes work, and a retained candidate's decision wait was
// work in flight. It reads the records, not the assignments, so no
// assignment state can hide one. A completed round's candidate has no record
// and survives a restart on disk as before.
func (rt *Runtime) closeRetainedCandidatesAtCrashBoot(ctx context.Context) {
	if rt == nil || rt.selfdevOperations == nil {
		return
	}
	rt.closeOrphanedFrozenCandidatesAtCrashBoot(ctx)
	for _, rec := range listRetainedCandidates(rt.selfdevUpdaterRoot) {
		operation, err := rt.selfdevOperations.Get(ctx, rec.ComputerID, rec.OperationID)
		if err != nil {
			log.Printf("selfdev: crash boot: retained candidate %s: read: %v", rec.OperationID, err)
			continue
		}
		// Only the wait for Verify is the retention's; a candidate management
		// already verified waits for the owner like any other and is left.
		if operation.State == selfdev.StateFrozen {
			log.Printf("selfdev: crash boot: %s was retained for its decision when assignment %s's build ended; a crash closes it", rec.OperationID, rec.AssignmentID)
			rt.failSelfdevOperation(ctx, operation, engineeringInterruptedByRestart+": the computer crashed while candidate "+rec.OperationID+" waited for its decision")
			if current, err := rt.selfdevOperations.Get(ctx, rec.ComputerID, rec.OperationID); err != nil || current.State == selfdev.StateFrozen {
				continue
			}
		}
		if err := os.Remove(retainedPath(rt.selfdevUpdaterRoot, rec.OperationID)); err != nil && !errors.Is(err, os.ErrNotExist) {
			log.Printf("selfdev: crash boot: remove the record of %s: %v", rec.OperationID, err)
		}
	}
}

// closeOrphanedFrozenCandidatesAtCrashBoot fails, at a crash boot, every
// frozen candidate whose round ended without completing and that has no
// retention record: a cancel whose best-effort close did not land, or a
// retention whose record was never written. Nothing can verify one, and it
// holds every engineering admission on the computer.
func (rt *Runtime) closeOrphanedFrozenCandidatesAtCrashBoot(ctx context.Context) {
	computerID := strings.TrimSpace(rt.TextureComputerID())
	if computerID == "" || rt.store == nil {
		return
	}
	frozen, err := rt.selfdevOperations.ListByStates(ctx, computerID, selfdev.StateFrozen)
	if err != nil || len(frozen) == 0 {
		if err != nil {
			log.Printf("selfdev: crash boot: list frozen candidates: %v", err)
		}
		return
	}
	assignments, err := rt.store.ListEngineeringAssignmentsForComputer(ctx, computerID)
	if err != nil {
		log.Printf("selfdev: crash boot: list assignments for frozen candidates: %v", err)
		return
	}
	newest := map[string]types.EngineeringAssignment{}
	for _, a := range assignments {
		if cur, ok := newest[a.AssignmentID]; !ok || a.Binding.Attempt > cur.Binding.Attempt {
			newest[a.AssignmentID] = a
		}
	}
	prefix := store.EngineeringFreezeIdempotencyKey("")
	for _, operation := range frozen {
		assignmentID, ok := strings.CutPrefix(operation.IdempotencyKey, prefix)
		if !ok {
			continue
		}
		assignment, found := newest[strings.TrimSpace(assignmentID)]
		if !found || !assignment.Disposition.Terminal() || assignment.Disposition == types.EngineeringAssignmentCompleted {
			continue
		}
		if _, retained := rt.retainedFor(operation, assignment.AssignmentID); retained {
			continue // the record pass below closes it
		}
		log.Printf("selfdev: crash boot: %s is frozen but its assignment %s ended %s; nothing can verify it, so it closes", operation.OperationID, assignment.AssignmentID, assignment.Disposition)
		rt.failSelfdevOperation(ctx, operation, engineeringInterruptedByRestart+": assignment "+assignment.AssignmentID+" ended "+string(assignment.Disposition)+" and its frozen candidate could not be verified")
	}
}
