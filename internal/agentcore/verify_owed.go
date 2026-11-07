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
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// verify_owed.go — management never ends with a frozen candidate
// unverified, silently
// (docs/problems/management-ends-without-verify-2026-11-04.md).
//
// Verify is owed on an assignment management cast when its current
// operation is frozen with a bundle and commitVerifyIntent would admit it:
// the assignment completed, or was cancelled by its build's end with the
// candidate retained (retained_candidate.go), or (engineer reuse) is open
// with its engineer asking for review. Then:
//   - the guard: a management turn that ends with a Verify owed is sent
//     back with "Verify owed on <assignment>: ..." at most
//     verifyOwedGuardReminders times, then accepted;
//   - the backstop: an activation that ends with a Verify owed, and an
//     engineer's Report on an assignment whose candidate is frozen, arm a
//     verify_owed continuation. If the Verify is still owed and management
//     is idle when it fires, a system notice (types.NewVerifyOwedNotice)
//     wakes management, at most types.VerifyOwedMaxNotices per operation,
//     and the Texture the round serves gets a wait line;
//   - management's pack shows each owed Verify as a typed wait
//     (types.WaitKindVerifyOwed).
//
// verify_owed is a restart work kind: a crash boot interrupts a pre-boot
// occurrence, and nothing at boot arms one (the restart rule). A later
// activation's end may arm again; that is new work on a booted computer.

const (
	verifyOwedUpdateKind = "verify_owed"
	// verifyOwedGuardReminders bounds the guard's reminders per activation.
	// It must not exceed the tool loop's maxCompletionGuardRetries (the loop
	// fails a run only past it), so the guard never fails the run.
	verifyOwedGuardReminders = 2
	// verifyOwedAfterRunEnd and verifyOwedAfterReport are how long after an
	// activation's end, and after an engineer's Report, the continuation
	// re-checks. The Report's own record wakes management first; the
	// continuation is for a wake that was lost or a turn that did not
	// Verify.
	verifyOwedAfterRunEnd = 75 * time.Second
	verifyOwedAfterReport = 90 * time.Second
	// verifyOwedNoticeSpacing is the least time between two notices about
	// one operation: the run the first one woke arms its own re-check.
	verifyOwedNoticeSpacing = 60 * time.Second
	// verifyOwedChainAfter is when a notice's own re-check fires.
	verifyOwedChainAfter = 3 * time.Minute
	verifyOwedEpisode    = "verify-owed"
)

// verifyOwed is one Verify management owes.
type verifyOwed struct {
	Assignment types.EngineeringAssignment
	Operation  selfdev.Operation
	// Why is the state that admits the Verify: completed, retained or
	// review.
	Why string
}

const (
	verifyOwedCompleted = "completed"
	verifyOwedRetained  = "retained"
	verifyOwedReview    = "review"
)

// Line is the harness line naming the act that settles it.
func (o verifyOwed) Line() string {
	id := o.Assignment.AssignmentID
	var state string
	switch o.Why {
	case verifyOwedRetained:
		state = "its build ended after its Freeze and the candidate was retained for your Verify"
	case verifyOwedReview:
		state = "its engineer asked for your review and waits with its capsule live"
	default:
		state = "its engineer completed"
	}
	return fmt.Sprintf("Verify owed on %s: call choir.Verify(%s, pass|fail, reasons) before ending. Candidate %s is frozen and unverified (%s); "+
		"the call is choir.Verify(%q, \"pass\" or \"fail\", verifierRefs, %q, reason). A check you ran, a Report or a Note is not a verdict: only choir.Verify moves the candidate.",
		id, id, o.Operation.OperationID, state, id, o.Operation.BundleDigest)
}

// freezeKeyAssignment names the assignment a freeze operation's
// idempotency key belongs to: freeze-open:<assignment>, with engineer
// reuse's ":rev:<n>" suffix on later revisions.
func freezeKeyAssignment(key string) (string, bool) {
	return store.EngineeringFreezeAssignmentOf(key)
}

// verifyOwedOn returns the Verify owed on assignmentID, if any: the states
// commitVerifyIntent admits, with the assignment's current candidate
// frozen.
func (rt *Runtime) verifyOwedOn(ctx context.Context, ownerID, computerID, assignmentID string) (verifyOwed, bool) {
	if rt == nil || rt.store == nil || rt.selfdevOperations == nil || strings.TrimSpace(assignmentID) == "" {
		return verifyOwed{}, false
	}
	// Attempt 1, as commitVerifyIntent reads it (records assignments have
	// no other attempt).
	assignment, err := rt.store.GetEngineeringAssignment(ctx, ownerID, computerID, assignmentID, 1)
	if err != nil {
		return verifyOwed{}, false
	}
	operation, frozen := rt.frozenCandidateOf(ctx, computerID, assignmentID)
	if !frozen {
		return verifyOwed{}, false
	}
	owed := verifyOwed{Assignment: assignment, Operation: operation}
	switch {
	case assignment.Disposition == types.EngineeringAssignmentCompleted:
		owed.Why = verifyOwedCompleted
	case assignment.Disposition == types.EngineeringAssignmentCancelled:
		if _, retained := rt.retainedFor(operation, assignmentID); !retained {
			return verifyOwed{}, false
		}
		owed.Why = verifyOwedRetained
	case !assignment.Disposition.Terminal() && rt.verifyOwedInReview(ctx, assignment, operation):
		owed.Why = verifyOwedReview
	default:
		return verifyOwed{}, false
	}
	return owed, true
}

// verifyOwedInReview reports whether an open assignment's frozen candidate
// waits for management's review: its engineer is parked on the verdict
// (engineer_reuse.go).
func (rt *Runtime) verifyOwedInReview(ctx context.Context, assignment types.EngineeringAssignment, operation selfdev.Operation) bool {
	return rt.assignmentInReview(ctx, assignment, operation)
}

// verifyOwedBy lists the Verifies deskAgentID owes on computerID, one per
// assignment, in assignment order. It reads the computer's frozen
// operations first, which are almost always none.
func (rt *Runtime) verifyOwedBy(ctx context.Context, ownerID, computerID, deskAgentID string) ([]verifyOwed, error) {
	if rt == nil || rt.store == nil || rt.selfdevOperations == nil || strings.TrimSpace(computerID) == "" {
		return nil, nil
	}
	frozen, err := rt.selfdevOperations.ListByStates(ctx, computerID, selfdev.StateFrozen)
	if err != nil {
		return nil, fmt.Errorf("verify owed: list frozen candidates: %w", err)
	}
	seen := map[string]bool{}
	var out []verifyOwed
	for _, operation := range frozen {
		assignmentID, ok := freezeKeyAssignment(operation.IdempotencyKey)
		if !ok || seen[assignmentID] {
			continue
		}
		owed, ok := rt.verifyOwedOn(ctx, ownerID, computerID, assignmentID)
		if !ok || owed.Operation.OperationID != operation.OperationID || owed.Assignment.Binding.ParentAgentID != strings.TrimSpace(deskAgentID) {
			continue
		}
		seen[assignmentID] = true
		out = append(out, owed)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Assignment.AssignmentID < out[j].Assignment.AssignmentID })
	return out, nil
}

// isPersistentManagementRun reports whether rec is a run of the owner's
// persistent management desk, the only desk that casts engineering.
func isPersistentManagementRun(rec *types.RunRecord) bool {
	return rec != nil && strings.TrimSpace(rec.OwnerID) != "" && agentProfileForRun(rec) == agentprofile.Management &&
		strings.TrimSpace(rec.AgentID) == persistentManagementAgentID(rec.OwnerID)
}

func (rt *Runtime) runComputerID(rec *types.RunRecord) string {
	if rec != nil && strings.TrimSpace(rec.ComputerID) != "" {
		return strings.TrimSpace(rec.ComputerID)
	}
	return strings.TrimSpace(rt.TextureComputerID())
}

// managementVerifyOwedGuard is the persistent management desk's completion
// guard: a turn that ends with a Verify owed resumes with the harness line,
// at most verifyOwedGuardReminders times per activation; then the turn is
// accepted and the backstop (armVerifyOwedAfterRun) takes over. A read
// failure accepts the turn: the guard is a prompt, never a gate.
func (rt *Runtime) managementVerifyOwedGuard(rec *types.RunRecord) toolregistry.ToolLoopCompletionGuardFunc {
	return func(ctx context.Context, state toolregistry.ToolLoopCompletionState) (toolregistry.ToolLoopCompletionGuardResult, error) {
		if state.Attempts >= verifyOwedGuardReminders {
			return toolregistry.ToolLoopCompletionGuardResult{}, nil
		}
		owed, err := rt.verifyOwedBy(ctx, rec.OwnerID, rt.runComputerID(rec), rec.AgentID)
		if err != nil {
			log.Printf("runtime: verify owed guard for run %s: %v", rec.RunID, err)
			return toolregistry.ToolLoopCompletionGuardResult{}, nil
		}
		lines := make([]string, 0, len(owed))
		for _, o := range owed {
			lines = append(lines, o.Line())
		}
		// The shots-owed sibling: an accepted operation owing shots and no
		// verdict is not ended over either.
		shots, err := rt.shotsOwedBy(ctx, rt.runComputerID(rec))
		if err != nil {
			log.Printf("runtime: shots owed guard for run %s: %v", rec.RunID, err)
		}
		for _, o := range shots {
			lines = append(lines, shotsOwedLine(o.Operation.OperationID))
		}
		if len(lines) == 0 {
			return toolregistry.ToolLoopCompletionGuardResult{}, nil
		}
		reason := "verify_owed"
		if len(owed) == 0 {
			reason = "shots_owed"
		}
		return toolregistry.ToolLoopCompletionGuardResult{Continue: true, Reason: reason, Instruction: strings.Join(lines, "\n")}, nil
	}
}

// verifyOwedWake is a verify_owed continuation's content. ArmedBy makes
// each arming its own occurrence (an activation's run id, a Report's id).
type verifyOwedWake struct {
	AssignmentID string `json:"assignment_id"`
	OperationID  string `json:"operation_id"`
	ArmedBy      string `json:"armed_by"`
}

// armVerifyOwed schedules the re-check of one owed Verify after d.
func (rt *Runtime) armVerifyOwed(ctx context.Context, owed verifyOwed, armedBy string, d time.Duration) {
	binding := owed.Assignment.Binding
	content, err := json.Marshal(verifyOwedWake{AssignmentID: owed.Assignment.AssignmentID, OperationID: owed.Operation.OperationID, ArmedBy: armedBy})
	if err != nil {
		return
	}
	rt.scheduleContinuation(context.WithoutCancel(ctx), binding.OwnerID, binding.ComputerID, binding.ParentAgentID,
		verifyOwedUpdateKind, string(content), "", "", rt.assignmentWaitNow().Add(d))
}

// armVerifyOwedAfterRun is the backstop at a management activation's end
// (any end: completed, failed, cancelled or parked). A run a crash ended
// arms nothing.
func (rt *Runtime) armVerifyOwedAfterRun(ctx context.Context, rec *types.RunRecord) {
	if rt == nil || !isPersistentManagementRun(rec) || !(rec.State.Terminal() || rec.State == types.RunPassivated) || rec.Error == runFateInterruptedByRestart {
		return
	}
	owed, err := rt.verifyOwedBy(ctx, rec.OwnerID, rt.runComputerID(rec), rec.AgentID)
	if err != nil {
		log.Printf("runtime: verify owed after run %s: %v", rec.RunID, err)
		return
	}
	for _, o := range owed {
		log.Printf("runtime: management run %s ended with the Verify of %s owed (candidate %s frozen, %s); re-check armed in %s",
			rec.RunID, o.Assignment.AssignmentID, o.Operation.OperationID, o.Why, verifyOwedAfterRunEnd)
		rt.armVerifyOwed(ctx, o, "run:"+rec.RunID, verifyOwedAfterRunEnd)
	}
}

// armVerifyOwedAfterReport is the backstop at an engineer's Report: when it
// leaves the assignment's candidate owed a Verify, the re-check is armed in
// case the Report's wake of management is lost or the turn it starts does
// not Verify.
func (rt *Runtime) armVerifyOwedAfterReport(ctx context.Context, assignment types.EngineeringAssignment, reportID string) {
	if rt == nil {
		return
	}
	binding := assignment.Binding
	owed, ok := rt.verifyOwedOn(ctx, binding.OwnerID, binding.ComputerID, assignment.AssignmentID)
	if !ok || binding.ParentAgentID != persistentManagementAgentID(binding.OwnerID) {
		return
	}
	rt.armVerifyOwed(ctx, owed, "report:"+strings.TrimSpace(reportID), verifyOwedAfterReport)
}

// HandleVerifyOwed is the verify_owed continuation: when the Verify is
// still owed on the same candidate and management is idle, it commits the
// next notice, which wakes management, and tells the Texture the round
// serves. A Verify since, a candidate that left frozen, a later revision, a
// running management turn or the notice bound makes it a no-op.
func (rt *Runtime) HandleVerifyOwed(ctx context.Context, ownerID, computerID, agentID, content string) error {
	if rt == nil || rt.store == nil {
		return fmt.Errorf("verify owed: store unavailable")
	}
	var wake verifyOwedWake
	if err := json.Unmarshal([]byte(content), &wake); err != nil || strings.TrimSpace(wake.AssignmentID) == "" || strings.TrimSpace(wake.OperationID) == "" {
		log.Printf("runtime: verify owed: unreadable continuation %q", content)
		return nil
	}
	if strings.TrimSpace(agentID) != persistentManagementAgentID(ownerID) {
		return nil
	}
	owed, ok := rt.verifyOwedOn(ctx, ownerID, computerID, wake.AssignmentID)
	if !ok || owed.Operation.OperationID != wake.OperationID || owed.Assignment.Binding.ParentAgentID != strings.TrimSpace(agentID) {
		return nil
	}
	now := rt.assignmentWaitNow()
	active, found, err := rt.activeRunByAgent(ctx, ownerID, agentID)
	if err != nil {
		return fmt.Errorf("verify owed: management's active run: %w", err)
	}
	if found && !rt.runHasNoLiveLoop(active, now) {
		// Its guard asks for the Verify, and its end re-checks. One more
		// re-check per run covers a run that never reaches an end that arms
		// (review r1b, DeepSeek N1); its content names the run, so it is
		// armed once.
		log.Printf("runtime: verify owed on %s: management run %s is working; its end re-checks", wake.AssignmentID, active.RunID)
		rt.armVerifyOwed(ctx, owed, "busy:"+active.RunID, deadRunGrace+verifyOwedAfterRunEnd)
		return nil
	}
	n, last, lastSeq, err := rt.verifyOwedNotices(ctx, ownerID, computerID, owed.Operation.OperationID)
	if err != nil {
		return err
	}
	// The last notice is still unread with management idle: its wake was
	// lost (a crash, a deferred delivery) or the turn it started ended
	// before reading it. Only the notice's own re-check (armed once, at its
	// commit, under content that names it) wakes management for it again,
	// so a notice gets at most one re-wake however many turns end unread
	// (review r1, Devin 1 and 2; review r1b, DeepSeek B1). Any other
	// re-check leaves it, and spends no notice on a turn that never read
	// the last one.
	if n > 0 && lastSeq > 0 {
		cursor, cerr := rt.store.ReaderCursor(ctx, ownerID, computerID, agentID, agentID)
		if cerr != nil {
			return fmt.Errorf("verify owed: management's read cursor: %w", cerr)
		}
		if cursor < lastSeq {
			noticeID := types.VerifyOwedNoticeID(owed.Operation.OperationID, n)
			if wake.ArmedBy != "notice:"+noticeID {
				log.Printf("runtime: verify owed on %s: notice %s is unread; left to that notice's own re-check", wake.AssignmentID, noticeID)
				return nil
			}
			log.Printf("runtime: verify owed on %s: notice %s is unread with management idle; waking management for it once", wake.AssignmentID, noticeID)
			if _, werr := rt.WakeRecordsInbox(ctx, ownerID, computerID, agentID); werr != nil {
				return fmt.Errorf("verify owed: wake management for %s: %w", noticeID, werr)
			}
			return nil
		}
	}
	if n >= types.VerifyOwedMaxNotices {
		log.Printf("runtime: verify owed on %s: candidate %s is still unverified after %d notices; it stays visible as a wait in management's pack",
			wake.AssignmentID, owed.Operation.OperationID, n)
		return nil
	}
	if !last.IsZero() && now.Sub(last) < verifyOwedNoticeSpacing {
		rt.armVerifyOwed(ctx, owed, "spaced:"+types.VerifyOwedNoticeID(owed.Operation.OperationID, n), last.Add(verifyOwedNoticeSpacing).Sub(now))
		return nil
	}
	n++
	binding := owed.Assignment.Binding
	wait := verifyOwedWait(owed, now)
	body := fmt.Sprintf("%s Notice %d of %d: %s.", owed.Line(), n, types.VerifyOwedMaxNotices, wait.Sentence)
	notice := types.NewVerifyOwedNotice(agentID, computerID, binding.TrajectoryID, owed.Assignment.AssignmentID, owed.Operation.OperationID, owed.Operation.BundleDigest, n, body)
	req := types.CommitInboxRecordsRequest{OwnerID: ownerID, ComputerID: computerID, CommandID: "verify-owed-notice:" + notice.RecordID, Records: []types.CommitmentRecord{notice}}
	if req.CommandDigest, err = store.ComputeCommitInboxRecordsDigest(req); err != nil {
		return err
	}
	if _, err := rt.store.CommitInboxRecords(ctx, req); err != nil {
		return fmt.Errorf("verify owed: notice %s: %w", notice.RecordID, err)
	}
	log.Printf("runtime: verify owed on %s: candidate %s frozen and unverified with management idle; notice %d of %d wakes management",
		wake.AssignmentID, owed.Operation.OperationID, n, types.VerifyOwedMaxNotices)
	rt.mintVerifyOwedRelay(ctx, owed, n, wait, now)
	// The chain does not hang on the wake it just sent: one re-check per
	// notice, which finds the Verify done, management working, the notice
	// unread (re-wake) or the next notice due.
	rt.armVerifyOwed(ctx, owed, "notice:"+notice.RecordID, verifyOwedChainAfter)
	return nil
}

// verifyOwedNotices counts the notices already committed about operationID
// and returns when the last one was committed and its inbox seq.
func (rt *Runtime) verifyOwedNotices(ctx context.Context, ownerID, computerID, operationID string) (int, time.Time, int64, error) {
	var last time.Time
	var seq int64
	n := 0
	for i := 1; i <= types.VerifyOwedMaxNotices; i++ {
		rec, err := rt.store.GetCommitmentRecord(ctx, ownerID, computerID, types.VerifyOwedNoticeID(operationID, i))
		if err != nil {
			return 0, time.Time{}, 0, fmt.Errorf("verify owed: read notice %d: %w", i, err)
		}
		if rec == nil {
			break
		}
		n, seq = i, rec.InboxSeq
		if at, perr := time.Parse(time.RFC3339Nano, rec.Provenance.CommittedAt); perr == nil {
			last = at
		}
	}
	return n, last, seq, nil
}

// verifyOwedWait is an owed Verify as a typed wait, since the Freeze.
func verifyOwedWait(o verifyOwed, now time.Time) *types.WaitState {
	since, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(o.Operation.UpdatedAt))
	if err != nil || since.IsZero() || since.After(now) {
		since = now
	}
	w := &types.WaitState{
		Kind: types.WaitKindVerifyOwed, Class: types.WaitClassPolicy,
		Subject:  "assignment " + o.Assignment.AssignmentID,
		Resource: "management's Verify of frozen candidate " + o.Operation.OperationID,
		Need:     fmt.Sprintf("choir.Verify(%q, pass|fail, verifierRefs, %q, reason)", o.Assignment.AssignmentID, o.Operation.BundleDigest),
		Holders:  []types.WaitHolder{{Who: "management", What: "Verify"}},
		Since:    since.UTC(),
		Detail:   strings.TrimSuffix(o.Line(), "."),
	}
	w.Stamp(now)
	return w
}

// mintVerifyOwedRelay tells the Texture the round serves that its
// candidate waits for management's Verify, through the wait relay's path
// and switch.
func (rt *Runtime) mintVerifyOwedRelay(ctx context.Context, o verifyOwed, n int, w *types.WaitState, now time.Time) {
	binding := o.Assignment.Binding
	textureInbox, ok := types.ConversationCounterpart(binding.ParentConversationID, binding.ParentAgentID)
	if !ok || !strings.HasPrefix(textureInbox, agentprofile.Texture+":") {
		return
	}
	body := fmt.Sprintf("%s: candidate %s is frozen and not yet verified, and management was woken to Verify it (notice %d of %d)",
		w.TextureLine(now), o.Operation.OperationID, n, types.VerifyOwedMaxNotices)
	relay, ok := types.NewEngineeringWaitRelay(binding.AssignedAgentID, textureInbox, binding.ComputerID, binding.Attempt,
		fmt.Sprintf("%s-%s-%d", verifyOwedEpisode, o.Operation.OperationID, n), types.WaitRelayStart, body, now,
		map[string]string{"wait_kind": string(types.WaitKindVerifyOwed), "operation_id": o.Operation.OperationID})
	if !ok {
		return
	}
	rt.mintWaitRelays(ctx, []waitRelayPlan{{ownerID: binding.OwnerID, computerID: binding.ComputerID, rec: relay}}, "")
}

// verifyOwedWaitViews is management's view of the Verifies it owes, one
// typed wait each. A retained candidate already shows as its decision wait
// (candidateDecisionWaitViews) and is not repeated. The pack is context,
// never a gate.
func (rt *Runtime) verifyOwedWaitViews(ctx context.Context, ownerID, computerID, deskAgentID string) []types.AssignmentWaitView {
	owed, err := rt.verifyOwedBy(ctx, ownerID, computerID, deskAgentID)
	if err != nil {
		log.Printf("desk pack: verify owed: %v", err)
		return nil
	}
	now := rt.assignmentWaitNow()
	var out []types.AssignmentWaitView
	for _, o := range owed {
		if o.Why == verifyOwedRetained {
			continue
		}
		out = append(out, types.AssignmentWaitView{AssignmentID: o.Assignment.AssignmentID, Attempt: o.Assignment.Binding.Attempt, Wait: verifyOwedWait(o, now)})
	}
	return out
}

// ---- the shots-owed sibling (docs/design/desk-shots-and-review-2026-11-07.md 3.4)
//
// An accepted platform operation whose host build is done owes management a
// shot verdict. The gate commits the shots-owed record once and keeps the
// clock (askManagementForShots: capped notices with fresh wake ids, then the
// deadline); this file is management's side of it: the completion guard
// (above, in managementVerifyOwedGuard), the end-of-run backstop and the
// typed wait. Nothing arms at boot: the backstop runs at a run's end and the
// gate's retry wake is the pending operation's own.

// shotsOwedBy lists the accepted operations of computerID that owe shots
// management has not judged.
func (rt *Runtime) shotsOwedBy(ctx context.Context, computerID string) ([]shotsOwedInfo, error) {
	if rt == nil || rt.store == nil || rt.selfdevOperations == nil || strings.TrimSpace(computerID) == "" {
		return nil, nil
	}
	accepted, err := rt.selfdevOperations.ListByStates(ctx, computerID, selfdev.StateAccepted)
	if err != nil {
		return nil, fmt.Errorf("shots owed: list accepted operations: %w", err)
	}
	var out []shotsOwedInfo
	for _, operation := range accepted {
		if operation.IsPackage() {
			continue
		}
		info, refusal, err := rt.shotsOwedFor(ctx, computerID, operation.OperationID)
		if err != nil {
			return out, err
		}
		if refusal == "" {
			out = append(out, info)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Operation.OperationID < out[j].Operation.OperationID })
	return out, nil
}

// armShotsOwedAfterRun is the backstop at a management activation's end: a
// run that ended with shots owed asks the materializer to re-run the gate,
// which notices management once it has been idle long enough. A run a crash
// ended asks nothing.
func (rt *Runtime) armShotsOwedAfterRun(ctx context.Context, rec *types.RunRecord) {
	if rt == nil || !isPersistentManagementRun(rec) || !(rec.State.Terminal() || rec.State == types.RunPassivated) || rec.Error == runFateInterruptedByRestart {
		return
	}
	owed, err := rt.shotsOwedBy(ctx, rt.runComputerID(rec))
	if err != nil {
		log.Printf("runtime: shots owed after run %s: %v", rec.RunID, err)
		return
	}
	for _, o := range owed {
		log.Printf("runtime: management run %s ended with the shots of %s owed; the gate re-checks", rec.RunID, o.Operation.OperationID)
	}
	if len(owed) > 0 {
		rt.TriggerSelfDevelopmentReconcile()
	}
}

// shotsOwedWait is an owed shot verdict as a typed wait, since the record.
func shotsOwedWait(o shotsOwedInfo, now time.Time) *types.WaitState {
	since := now
	if at, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(o.Record.Provenance.CommittedAt)); err == nil && !at.IsZero() && !at.After(now) {
		since = at
	}
	w := &types.WaitState{
		Kind: types.WaitKindShotsOwed, Class: types.WaitClassPolicy,
		Subject:  "operation " + o.Operation.OperationID,
		Resource: "management's shot verdict on the host build of " + o.Operation.OperationID,
		Need:     fmt.Sprintf("choir.ShotStart(%q), choir.ShotWait, then choir.ShotVerdict(%q, pass|fail, findings, reason)", o.Operation.OperationID, o.Operation.OperationID),
		Holders:  []types.WaitHolder{{Who: "management", What: "ShotVerdict"}},
		Since:    since.UTC(),
		Detail:   strings.TrimSuffix(shotsOwedLine(o.Operation.OperationID), "."),
	}
	w.Stamp(now)
	return w
}

// shotsOwedWaitViews is management's view of the shot verdicts it owes, one
// typed wait each. The pack is context, never a gate.
func (rt *Runtime) shotsOwedWaitViews(ctx context.Context, _, computerID string) []types.AssignmentWaitView {
	owed, err := rt.shotsOwedBy(ctx, computerID)
	if err != nil {
		log.Printf("desk pack: shots owed: %v", err)
		return nil
	}
	now := rt.assignmentWaitNow()
	var out []types.AssignmentWaitView
	for _, o := range owed {
		out = append(out, types.AssignmentWaitView{AssignmentID: o.Operation.OperationID, Wait: shotsOwedWait(o, now)})
	}
	return out
}
