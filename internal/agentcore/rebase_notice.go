package agentcore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/sourceproj"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// U4 (docs/design/u4-stack-rebase-build-2026-10-26.md, route D, slices 1 and
// 2): when the applied self-development stack sits on a base the computer no
// longer booted (stack_base_moved), the runtime asks the root Texture RLM, with a
// rebase notice, to rebase it. This file is the trigger and the bounded
// re-mint. There is no switch (capsule overlay S4). With no live root RLM the
// notice goes to the Texture of the document the stack operation's own
// trajectory binds (rebaseTexture); with neither, the timer mints nothing and
// the Cast refusal says rebase_needs_root.
//
// Restart rules (CLAUDE.md, "Restarts End Work (Crash) Or Resume It"):
//   - Nothing here runs inside boot. Reconcile only arms an in-memory timer,
//     due at bootedAt + the stay-up interval; a crash forgets the timer and the
//     next boot recomputes everything from the tape.
//   - A crash never resumes an attempt. An attempt committed before a markerless
//     boot is closed for good (its wake is interrupted by the generic gate and
//     the cast gate and admission refuse it); the live timer may mint the NEXT
//     attempt, a new record under a new id, after the computer has stayed up
//     the interval: standing owner intents survive a crash, their occurrences
//     do not (recoverSourceSchedulesAtBoot is the precedent).
//   - A planned restart (marker present) parks and resumes a live round as the
//     existing rules do; a live attempt is not doubled.
//   - At most RebaseNoticeMaxAttempts attempts per pair, none once an assignment
//     claims the pair. State lives on the tape (the notice records and the claim
//     object) and nowhere else.

const (
	// defaultRebaseStayUp is how long the computer must have stayed up since
	// boot before the first notice (and a re-mint after a crash).
	defaultRebaseStayUp = 5 * time.Minute
	// defaultRebaseAnswerWindow is how long an attempt waits for the root's
	// Cast before it counts as unanswered.
	defaultRebaseAnswerWindow = 30 * time.Minute
	// rebaseMinDelay keeps a timer armed from a long-running boot from firing
	// inside the reconcile that armed it.
	rebaseMinDelay = time.Second
	// rebaseReconcileReason names the reconcile the timer runs; it never arms.
	rebaseReconcileReason = "rebase"
)

// rebaseTimerHandle is what stops an armed timer.
type rebaseTimerHandle interface{ Stop() bool }

// rebaseNoticeOutcome is what one timer fire decided.
type rebaseNoticeOutcome struct {
	// Minted is the notice id committed, or "".
	Minted  string
	Attempt int
	// Reason says why nothing was minted.
	Reason string
	// Rearm is when to fire again; zero ends the timer.
	Rearm time.Duration
}

func (rt *Runtime) rebaseClock() time.Time {
	if rt.rebaseNow != nil {
		return rt.rebaseNow().UTC()
	}
	return time.Now().UTC()
}

func (rt *Runtime) rebaseStayUpInterval() time.Duration {
	if rt.rebaseStayUp > 0 {
		return rt.rebaseStayUp
	}
	return defaultRebaseStayUp
}

func (rt *Runtime) rebaseAnswerWindowInterval() time.Duration {
	if rt.rebaseAnswerWindow > 0 {
		return rt.rebaseAnswerWindow
	}
	return defaultRebaseAnswerWindow
}

// rebaseRecheckInterval is how often the timer looks again at a live
// attempt: the stay-up interval, never longer than the answer window.
func (rt *Runtime) rebaseRecheckInterval() time.Duration {
	every := rt.rebaseStayUpInterval()
	if window := rt.rebaseAnswerWindowInterval(); window < every {
		every = window
	}
	return every + rebaseMinDelay
}

// rebaseScope is the owner and computer the notice is minted for.
func (rt *Runtime) rebaseScope() (ownerID, computerID string) {
	if rt == nil {
		return "", ""
	}
	return firstNonEmpty(strings.TrimSpace(rt.selfdevRouteOwnerID), strings.TrimSpace(os.Getenv("CHOIR_OWNER_ID"))), strings.TrimSpace(rt.cfg.ComputerID)
}

// armRebaseNotice is called after a reconcile. It arms the one timer when the
// stack sits on a moved base; it never mints, so it is safe at boot. A timer
// already armed is left as it is.
func (rt *Runtime) armRebaseNotice(ctx context.Context, result sourceproj.Result) {
	if rt == nil || result.State != sourceproj.StateBaseMoved {
		return
	}
	ownerID, computerID := rt.rebaseScope()
	if ownerID == "" || computerID == "" {
		return
	}
	rt.rebaseMu.Lock()
	defer rt.rebaseMu.Unlock()
	if rt.rebaseTimer != nil {
		return
	}
	now := rt.rebaseClock()
	due := rt.bootedAt.Add(rt.rebaseStayUpInterval())
	if min := now.Add(rebaseMinDelay); due.Before(min) {
		due = min
	}
	rt.armRebaseLocked(due, now)
}

// applyTextureRootDeskSetting is the owner's change of texture.root_desk at
// runtime. On, a reconcile arms the timer if the stack sits on a moved base, so
// a root that just became live is asked (the timer, not this call, mints, and
// only after the stay-up interval). Off changes nothing here: the timer finds
// no live root and mints nothing.
func (rt *Runtime) applyTextureRootDeskSetting(value bool) {
	if rt == nil || !value {
		return
	}
	go rt.reconcileSourceProjectionLogged(context.Background(), "setting")
}

// rearmRebaseNotice arms the timer again after a fire.
func (rt *Runtime) rearmRebaseNotice(after time.Duration) {
	rt.rebaseMu.Lock()
	defer rt.rebaseMu.Unlock()
	if rt.rebaseTimer != nil {
		return
	}
	now := rt.rebaseClock()
	rt.armRebaseLocked(now.Add(after), now)
}

func (rt *Runtime) armRebaseLocked(due, now time.Time) {
	rt.rebaseDue = due
	fire := func() { rt.fireRebaseNotice(context.Background()) }
	if rt.rebaseAfterFunc != nil {
		rt.rebaseTimer = rt.rebaseAfterFunc(due.Sub(now), fire)
		return
	}
	rt.rebaseTimer = time.AfterFunc(due.Sub(now), fire)
}

// rebaseNoticeDueAt reports the armed timer's due time.
func (rt *Runtime) rebaseNoticeDueAt() (time.Time, bool) {
	rt.rebaseMu.Lock()
	defer rt.rebaseMu.Unlock()
	return rt.rebaseDue, rt.rebaseTimer != nil
}

// stopRebaseNotice stops and forgets the timer.
func (rt *Runtime) stopRebaseNotice() {
	rt.rebaseMu.Lock()
	defer rt.rebaseMu.Unlock()
	if rt.rebaseTimer != nil {
		rt.rebaseTimer.Stop()
	}
	rt.rebaseTimer, rt.rebaseDue = nil, time.Time{}
}

// fireRebaseNotice is the timer's work: decide from the tape, mint at most one
// notice, and re-arm while the state holds and attempts remain.
func (rt *Runtime) fireRebaseNotice(ctx context.Context) rebaseNoticeOutcome {
	rt.stopRebaseNotice()
	out := rt.mintRebaseNotice(ctx)
	if out.Minted != "" {
		log.Printf("rebase notice: minted %s (attempt %d)", out.Minted, out.Attempt)
	} else if out.Reason != "" {
		log.Printf("rebase notice: none minted: %s", out.Reason)
	}
	if out.Rearm > 0 {
		rt.rearmRebaseNotice(out.Rearm)
	}
	return out
}

// transientRebase is a failure that may pass (a failed read or commit): the
// timer tries again after the stay-up interval instead of ending, so one
// hiccup on a quiet computer does not stop the rebase until unrelated activity
// reconciles again (review 321). A state that does not hold is not transient.
func (rt *Runtime) transientRebase(format string, args ...any) rebaseNoticeOutcome {
	return rebaseNoticeOutcome{Reason: fmt.Sprintf(format, args...), Rearm: rt.rebaseStayUpInterval()}
}

func (rt *Runtime) mintRebaseNotice(ctx context.Context) rebaseNoticeOutcome {
	if rt == nil || rt.store == nil {
		return rebaseNoticeOutcome{Reason: "no store"}
	}
	ownerID, computerID := rt.rebaseScope()
	if ownerID == "" || computerID == "" {
		return rebaseNoticeOutcome{Reason: "no owner or computer"}
	}
	result, err := rt.ReconcileSourceProjection(ctx, rebaseReconcileReason)
	if err != nil {
		return rt.transientRebase("reconcile failed: %v", err)
	}
	if result.State != sourceproj.StateBaseMoved {
		return rebaseNoticeOutcome{Reason: "the stack is " + string(result.State) + ", not stack_base_moved"}
	}
	booted, stackBase, operationID := strings.TrimSpace(result.BootedBase), strings.TrimSpace(result.Base), strings.TrimSpace(result.OperationID)
	if booted == "" || stackBase == "" || operationID == "" || strings.HasPrefix(operationID, "genesis-") {
		return rebaseNoticeOutcome{Reason: "the stack names no source operation to rebase"}
	}
	if !sourceproj.HasCommit(ctx, rt.releaseRepo(), booted) {
		return rebaseNoticeOutcome{Reason: "the booted base's objects are missing"}
	}
	// Owner ruling 2026-11-04: a change that re-applies cleanly and builds
	// applies without an owner act (clean_rebase.go). Only when it does not,
	// or the attempt fails, does the notice go to Texture.
	if hold, why := rt.cleanRebaseBeforeNotice(ctx, ownerID, computerID, operationID, stackBase, booted); hold {
		return rebaseNoticeOutcome{Reason: why, Rearm: rt.rebaseRecheckInterval()}
	} else if why != "" {
		log.Printf("rebase notice: no clean rebase of %s onto %s: %s", operationID, booted, why)
	}
	addr, err := rt.rebaseTexture(ctx, ownerID, computerID, operationID)
	if err != nil {
		return rt.transientRebase("rebase Texture: %v", err)
	}
	if !addr.Live {
		return rebaseNoticeOutcome{Reason: rebaseNeedsRootBecause(addr)}
	}
	rootInbox, docID := addr.Inbox, addr.DocID
	pair := types.StackRebasePairKey(operationID, booted)
	if _, err := rt.store.GetRebaseNoticeClaim(ctx, ownerID, computerID, pair); err == nil {
		return rebaseNoticeOutcome{Reason: "an assignment claimed the pair"}
	} else if !errors.Is(err, store.ErrNotFound) {
		return rt.transientRebase("read the claim: %v", err)
	}
	// Two rebase rounds of one operation never run at once: a claim for an
	// older base whose assignment is not terminal defers this pair's notice.
	claims, err := rt.store.ListRebaseNoticeClaims(ctx, ownerID, computerID, operationID)
	if err != nil {
		return rt.transientRebase("read the operation's claims: %v", err)
	}
	for _, other := range claims {
		if other.PairKey == pair {
			continue
		}
		assignment, err := rt.store.GetEngineeringAssignment(ctx, ownerID, computerID, other.AssignmentID, 1)
		if err == nil && !assignment.Disposition.Terminal() {
			return rebaseNoticeOutcome{
				Reason: fmt.Sprintf("a rebase of %s onto %s (assignment %s) still runs", operationID, carriedBaseLabel(other.BootedBase), other.AssignmentID),
				Rearm:  rt.rebaseStayUpInterval(),
			}
		}
	}
	var latest *types.CommitmentRecord
	attempt := 0
	for n := 1; n <= types.RebaseNoticeMaxAttempts; n++ {
		rec, err := rt.store.GetCommitmentRecord(ctx, ownerID, computerID, types.StackRebaseNoticeID(operationID, booted, n))
		if err != nil {
			return rt.transientRebase("read attempt %d: %v", n, err)
		}
		if rec != nil {
			latest, attempt = rec, n
		}
	}
	next := 1
	if latest != nil && addr.Continue {
		// A notice already on the tape whose document settled (or whose
		// continue failed after its commit): continue now, so the record
		// is read. Idempotent: one next trajectory per settled one.
		if err := rt.continueRebaseDocument(ctx, ownerID, computerID, addr, latest.RecordID); err != nil {
			return rt.transientRebase("%v", err)
		}
	}
	if latest != nil {
		closed, wait, err := rt.rebaseAttemptClosed(ctx, ownerID, computerID, *latest, rootInbox)
		if err != nil {
			return rt.transientRebase("attempt %d: %v", attempt, err)
		}
		if attempt >= types.RebaseNoticeMaxAttempts {
			return rebaseNoticeOutcome{Reason: fmt.Sprintf("attempt %d was the last", attempt)}
		}
		if !closed {
			return rebaseNoticeOutcome{Reason: fmt.Sprintf("attempt %d is still live", attempt), Rearm: wait}
		}
		next = attempt + 1
	}
	notice, ok := types.NewStackRebaseNotice(types.StackRebaseNotice{
		TextureInbox: rootInbox, ComputerID: computerID, OperationID: operationID, BootedBase: booted, StackBase: stackBase, Attempt: next, DocID: docID,
	})
	if !ok {
		return rebaseNoticeOutcome{Reason: "the notice does not bind"}
	}
	req := types.CommitInboxRecordsRequest{OwnerID: ownerID, ComputerID: computerID, CommandID: notice.RecordID, Records: []types.CommitmentRecord{notice}}
	digest, err := store.ComputeCommitInboxRecordsDigest(req)
	if err != nil {
		return rt.transientRebase("digest: %v", err)
	}
	req.CommandDigest = digest
	commit := rt.rebaseCommit
	if commit == nil {
		commit = func(ctx context.Context, req types.CommitInboxRecordsRequest) error {
			_, err := rt.store.CommitInboxRecords(ctx, req)
			return err
		}
	}
	if err := commit(ctx, req); err != nil {
		return rt.transientRebase("commit %s: %v", notice.RecordID, err)
	}
	out := rebaseNoticeOutcome{Minted: notice.RecordID, Attempt: next}
	// The notice first, then the continue: a failure between them leaves a
	// durable unread record that the next fire's continue (above) makes
	// readable, never a new episode with no notice in it.
	if addr.Continue {
		if err := rt.continueRebaseDocument(ctx, ownerID, computerID, addr, notice.RecordID); err != nil {
			log.Printf("rebase notice: %s minted; its document's next trajectory did not start: %v (the next fire retries)", notice.RecordID, err)
			out.Rearm = rt.rebaseStayUpInterval()
			return out
		}
	}
	if next < types.RebaseNoticeMaxAttempts {
		// Look again every stay-up interval, not once per answer window: a
		// round that answered the notice and closed without a claim (the
		// rebase was refused) is followed by the next attempt within minutes
		// (problem 16). An unanswered notice still waits out its window.
		out.Rearm = rt.rebaseRecheckInterval()
	}
	return out
}

// rebaseAddressee is the Texture a rebase notice goes to.
type rebaseAddressee struct {
	Inbox, DocID, TrajectoryID string
	// Live: the Texture can answer the notice.
	Live bool
	// Continue: the document's current trajectory (TrajectoryID) has settled.
	// A Texture wake on it runs nothing, so after the notice commits the
	// document is continued onto its next trajectory, as the owner's revise
	// does (problem 21, third shape).
	Continue bool
	// Why says, when it is not live, why the operation's Texture cannot take
	// the notice.
	Why string
}

// rebaseTexture resolves the Texture a rebase notice of operationID goes to:
// the root Texture RLM when one is live, and otherwise the Texture of the
// document the operation's own trajectory binds
// (docs/problems/selfdev-round-blockers-cluster-2026-11-03.md, fix 10). The
// root exists only where texture.root_desk is on, which is off by default, so
// without the fallback no computer but the root's ever got a notice. The
// store's admission is unchanged: the notice's inbox must be the sender of the
// Texture Cast the rebase answers, and the document Texture sends its own.
func (rt *Runtime) rebaseTexture(ctx context.Context, ownerID, computerID, operationID string) (rebaseAddressee, error) {
	inbox, docID, live, err := rt.rebaseRoot(ctx, ownerID, computerID)
	if err != nil {
		return rebaseAddressee{}, err
	}
	if live {
		return rebaseAddressee{Inbox: inbox, DocID: docID, Live: true}, nil
	}
	return rt.rebaseOperationTexture(ctx, ownerID, computerID, operationID)
}

// rebaseNeedsRootBecause is the timer's reason when no Texture can take the
// notice: rebase_needs_root, and why the operation's own document Texture
// cannot.
func rebaseNeedsRootBecause(addr rebaseAddressee) string {
	if strings.TrimSpace(addr.Why) == "" {
		return rebaseNeedsRoot
	}
	return rebaseNeedsRoot + ", and the applied operation's document Texture cannot take it: " + addr.Why
}

// rebaseOperationTexture is the fallback: the document the operation's
// trajectory names, on the document's current trajectory (not archived), live
// when that trajectory is live and has no cancellation intent, whether or not it has open work or a resident run. The
// notice is the wake: a Texture turn started (or a passivated run reactivated)
// for unread records needs no open work item (the records route, B5). Problem
// 21: the fallback used to require an open work item, which a document Texture
// settles at the end of each round, so it worked only inside a round (pool's
// Files Texture settled at 10:50Z, and every fire after logged
// rebase_needs_root). An operation without a document (genesis, rollback) has
// none, and the reason says so.
func (rt *Runtime) rebaseOperationTexture(ctx context.Context, ownerID, computerID, operationID string) (rebaseAddressee, error) {
	operationID = strings.TrimSpace(operationID)
	if operationID == "" {
		return rebaseAddressee{Why: "the stack names no operation"}, nil
	}
	if rt.selfdevOperations == nil {
		return rebaseAddressee{Why: "the runtime has no operation store"}, nil
	}
	operation, err := rt.selfdevOperations.Get(ctx, computerID, operationID)
	if err != nil {
		if errors.Is(err, selfdev.ErrOperationNotFound) || errors.Is(err, sql.ErrNoRows) {
			return rebaseAddressee{Why: "operation " + operationID + " is not in the operation store"}, nil
		}
		return rebaseAddressee{}, fmt.Errorf("operation %s: %w", operationID, err)
	}
	// The document, then its current trajectory: an owner revision on a
	// settled trajectory starts a new one for the same document, so the
	// operation's own trajectory may no longer bind it (problem 21, pool
	// 13:58Z: b1ad3952 had moved past 5dc6a6ff).
	doc, why, err := rt.operationDocument(ctx, ownerID, computerID, operation)
	if err != nil {
		return rebaseAddressee{}, err
	}
	if why != "" {
		return rebaseAddressee{Why: why}, nil
	}
	docID, trajectoryID := doc.DocID, strings.TrimSpace(doc.TrajectoryID)
	agentID := currentTextureAgentID(docID)
	state, err := rt.rebaseTextureRunState(ctx, ownerID, computerID, trajectoryID, agentID)
	if err != nil {
		return rebaseAddressee{}, err
	}
	switch state {
	case rebaseTextureOpen, rebaseTextureSettled:
		return rebaseAddressee{Inbox: agentID, DocID: docID, TrajectoryID: trajectoryID, Live: true}, nil
	case rebaseTextureTrajectorySettled:
		return rebaseAddressee{Inbox: agentID, DocID: docID, TrajectoryID: trajectoryID, Live: true, Continue: true}, nil
	case rebaseTextureTrajectoryGone:
		return rebaseAddressee{Why: "its document " + docID + "'s trajectory " + trajectoryID + " is not on the tape"}, nil
	case rebaseTextureCancelled:
		return rebaseAddressee{Why: "its document " + docID + "'s trajectory " + trajectoryID + " has a cancellation intent"}, nil
	default:
		return rebaseAddressee{Why: "its document " + docID + "'s trajectory " + trajectoryID + " is not live"}, nil
	}
}

// continueRebaseDocument starts the next trajectory of a document whose
// trajectory settled, so the notice's Texture can run: the document, its
// Texture desk and one open Texture work item move to a live trajectory
// (store.ContinueLifecycleDocument), with the owner's revise's identity
// (store.TextureContinueIdentity), so a revise racing it starts the same
// trajectory. A document already moved off the settled trajectory, by this
// or by anyone, is done.
func (rt *Runtime) continueRebaseDocument(ctx context.Context, ownerID, computerID string, addr rebaseAddressee, noticeID string) error {
	settledID := strings.TrimSpace(addr.TrajectoryID)
	doc, err := rt.store.GetLifecycleDocument(ctx, ownerID, computerID, addr.DocID)
	if err != nil {
		return fmt.Errorf("document %s: %w", addr.DocID, err)
	}
	if strings.TrimSpace(doc.TrajectoryID) != settledID {
		return nil
	}
	trajectoryID, commandID, workItemID := store.TextureContinueIdentity(ownerID, computerID, addr.DocID, settledID)
	req := types.ContinueLifecycleDocumentRequest{
		OwnerID: ownerID, ComputerID: computerID, CommandID: commandID,
		TrajectoryID: trajectoryID, PredecessorTrajectoryID: settledID, DocID: addr.DocID,
		ExpectedHeadRevisionID: doc.CurrentRevisionID,
		InitialWork: types.WorkItemRecord{
			WorkItemID: workItemID, Objective: "Answer the platform's rebase notice " + noticeID + " with the one Cast to management it gives.",
			AssignedAgentID: addr.Inbox, AuthorityProfile: agentprofile.Texture,
		},
	}
	digest, err := store.ComputeContinueLifecycleDocumentDigest(req)
	if err != nil {
		return err
	}
	req.CommandDigest = digest
	_, continueErr := rt.store.ContinueLifecycleDocument(ctx, req)
	// A lost race (the owner's revise, another attempt) has moved it too.
	current, err := rt.store.GetLifecycleDocument(ctx, ownerID, computerID, addr.DocID)
	if err != nil {
		return fmt.Errorf("document %s: %w", addr.DocID, err)
	}
	if strings.TrimSpace(current.TrajectoryID) != settledID {
		return nil
	}
	if continueErr == nil {
		continueErr = errors.New("the continue left the document on its settled trajectory")
	}
	return fmt.Errorf("continue document %s past settled trajectory %s: %w", addr.DocID, settledID, continueErr)
}

type rebaseTextureState int

const (
	rebaseTextureOpen rebaseTextureState = iota
	rebaseTextureSettled
	rebaseTextureTrajectoryGone
	rebaseTextureTrajectoryNotLive
	rebaseTextureCancelled
	// rebaseTextureTrajectorySettled: the trajectory ended its episode of work
	// (settled, not cancelled); the document continues onto a next one.
	rebaseTextureTrajectorySettled
)

// rebaseTextureRunState says whether agentID can take a run on trajectoryID:
// open (the trajectory is live, has no cancellation intent, and has an open
// work item assigned to the agent), settled (the same with no open work item),
// or why not. A trajectory that is gone is a state, not an error.
func (rt *Runtime) rebaseTextureRunState(ctx context.Context, ownerID, computerID, trajectoryID, agentID string) (rebaseTextureState, error) {
	snapshot, err := rt.store.GetLifecycleSnapshot(ctx, ownerID, computerID, trajectoryID)
	if errors.Is(err, store.ErrNotFound) {
		return rebaseTextureTrajectoryGone, nil
	}
	if err != nil {
		return rebaseTextureTrajectoryNotLive, err
	}
	if snapshot.Trajectory.Status == types.TrajectorySettled {
		return rebaseTextureTrajectorySettled, nil
	}
	if snapshot.Trajectory.Status != types.TrajectoryLive {
		return rebaseTextureTrajectoryNotLive, nil
	}
	if _, cancelErr := rt.store.GetLifecycleCancellationIntent(ctx, ownerID, computerID, trajectoryID); cancelErr == nil {
		return rebaseTextureCancelled, nil
	} else if !errors.Is(cancelErr, store.ErrNotFound) {
		return rebaseTextureTrajectoryNotLive, cancelErr
	}
	for _, work := range snapshot.WorkItems {
		if work.Status == types.WorkItemOpen && work.AssignedAgentID == agentID {
			return rebaseTextureOpen, nil
		}
	}
	return rebaseTextureSettled, nil
}

// rebaseTextureTakesRun reports whether agentID can take a run on
// trajectoryID now (the root's check: its standing item is always open).
func (rt *Runtime) rebaseTextureTakesRun(ctx context.Context, ownerID, computerID, trajectoryID, agentID string) (bool, error) {
	state, err := rt.rebaseTextureRunState(ctx, ownerID, computerID, trajectoryID, agentID)
	return err == nil && state == rebaseTextureOpen, err
}

// rebaseRoot resolves the current root Texture RLM: the marked master's inbox
// and document, and whether it can take a root run now (its trajectory is
// live, has no cancellation intent, and its standing item is open).
func (rt *Runtime) rebaseRoot(ctx context.Context, ownerID, computerID string) (inbox, docID string, live bool, err error) {
	marker, err := rt.store.GetTextureRoot(ctx, ownerID, computerID)
	if errors.Is(err, store.ErrNotFound) {
		return "", "", false, nil
	}
	if err != nil {
		return "", "", false, err
	}
	doc, err := rt.store.GetLifecycleDocument(ctx, ownerID, computerID, marker.DocumentID)
	if errors.Is(err, store.ErrNotFound) {
		return "", "", false, nil
	}
	if err != nil {
		return "", "", false, err
	}
	trajectoryID := strings.TrimSpace(doc.TrajectoryID)
	if trajectoryID == "" || doc.ArchivedAt != nil {
		return "", "", false, nil
	}
	// A marked master whose trajectory is gone is not live (and not an
	// error that would keep the fallback from being asked).
	agentID := currentTextureAgentID(doc.DocID)
	live, err = rt.rebaseTextureTakesRun(ctx, ownerID, computerID, trajectoryID, agentID)
	if err != nil || !live {
		return "", "", false, err
	}
	return agentID, doc.DocID, true, nil
}

// rebaseAttemptClosed decides, from the tape, whether attempt n can no longer
// be answered, so attempt n+1 may follow. It is closed when any of these holds:
//   - it predates a markerless boot (a crash closed it);
//   - its inbox is not the current addressee (the master was replaced, or a
//     root became live in place of the operation's Texture);
//   - no Cast from its addressee names it and the answer window has passed;
//   - a Cast named it but that round is closed (and no claim exists).
//
// When it is not closed, wait says when to look again.
func (rt *Runtime) rebaseAttemptClosed(ctx context.Context, ownerID, computerID string, notice types.CommitmentRecord, rootInbox string) (closed bool, wait time.Duration, err error) {
	now := rt.rebaseClock()
	committed, _ := time.Parse(time.RFC3339Nano, strings.TrimSpace(notice.Provenance.CommittedAt))
	if !rt.bootRestartPlanned && !rt.bootedAt.IsZero() && committed.Before(rt.bootedAt) {
		return true, 0, nil
	}
	if strings.TrimSpace(notice.Inbox) != rootInbox {
		return true, 0, nil
	}
	casts, err := rt.rebaseNoticeCasts(ctx, ownerID, computerID, notice)
	if err != nil {
		return false, 0, err
	}
	window := rt.rebaseAnswerWindowInterval()
	if len(casts) == 0 {
		if remaining := committed.Add(window).Sub(now); remaining > 0 {
			// An answer may come and its round close (a refused rebase)
			// well inside the window: look again by then.
			wait := remaining + rebaseMinDelay
			if every := rt.rebaseRecheckInterval(); every < wait {
				wait = every
			}
			return false, wait, nil
		}
		return true, 0, nil
	}
	for _, cast := range casts {
		head, err := rt.store.ConversationState(ctx, ownerID, computerID, cast.ConversationID)
		if err != nil {
			return false, 0, err
		}
		if head.Open && head.Round == cast.Round {
			return false, rt.rebaseRecheckInterval(), nil
		}
	}
	return true, 0, nil
}

// rebaseNoticeCasts are the addressee's Casts to management that name the
// notice (the root's, or the operation's document Texture's).
func (rt *Runtime) rebaseNoticeCasts(ctx context.Context, ownerID, computerID string, notice types.CommitmentRecord) ([]types.CommitmentRecord, error) {
	conversation := types.InboxConversationID(persistentManagementAgentID(ownerID), notice.Inbox)
	records, err := rt.store.ConversationRecords(ctx, ownerID, computerID, conversation)
	if err != nil {
		return nil, err
	}
	var out []types.CommitmentRecord
	for _, rec := range records {
		if rec.IsCast() && rec.SenderInbox == notice.Inbox && rec.Directive != nil && types.SameStackRebaseNotice(types.CastRebaseNotice(rec.Directive.Body), notice.RecordID) {
			out = append(out, rec)
		}
	}
	return out, nil
}
