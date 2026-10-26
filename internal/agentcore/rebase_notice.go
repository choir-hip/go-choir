package agentcore

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/sourceproj"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// U4 (docs/design/u4-stack-rebase-build-2026-10-26.md, route D, slices 1 and
// 2): when the applied self-development stack sits on a base the computer no
// longer booted (stack_base_moved) and the owner has turned source.rebase_offer
// on, the runtime asks the root Texture RLM, with a rebase notice, to rebase it.
// This file is the trigger and the bounded re-mint.
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

// rebaseScope is the owner and computer the notice is minted for.
func (rt *Runtime) rebaseScope() (ownerID, computerID string) {
	if rt == nil {
		return "", ""
	}
	return firstNonEmpty(strings.TrimSpace(rt.selfdevRouteOwnerID), strings.TrimSpace(os.Getenv("CHOIR_OWNER_ID"))), strings.TrimSpace(rt.cfg.ComputerID)
}

// rebaseOfferOn is the effective switch: source.rebase_offer, and both
// source.projection and texture.root_desk, which it needs.
func (rt *Runtime) rebaseOfferOn(ctx context.Context, ownerID, computerID string) bool {
	return sourceproj.Enabled() &&
		rt.computerSettingOn(ctx, ownerID, computerID, store.SettingSourceRebaseOffer) &&
		rt.computerSettingOn(ctx, ownerID, computerID, store.SettingTextureRootDesk)
}

// armRebaseNotice is called after a reconcile. It arms the one timer when the
// stack sits on a moved base and the switches are on; it never mints, so it is
// safe at boot. A timer already armed is left as it is.
func (rt *Runtime) armRebaseNotice(ctx context.Context, result sourceproj.Result) {
	if rt == nil || result.State != sourceproj.StateBaseMoved {
		return
	}
	ownerID, computerID := rt.rebaseScope()
	if ownerID == "" || computerID == "" || !rt.rebaseOfferOn(ctx, ownerID, computerID) {
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

// applyRebaseOfferSetting is the owner's change of source.rebase_offer or
// texture.root_desk at runtime. On, a reconcile arms the timer if the stack
// sits on a moved base (the timer, not this call, mints, and only after the
// stay-up interval); off stops the timer, which is the live rollback.
func (rt *Runtime) applyRebaseOfferSetting(value bool) {
	if rt == nil {
		return
	}
	if !value {
		rt.stopRebaseNotice()
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
	if !rt.rebaseOfferOn(ctx, ownerID, computerID) {
		return rebaseNoticeOutcome{Reason: "a switch is off"}
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
	rootInbox, docID, live, err := rt.rebaseRoot(ctx, ownerID, computerID)
	if err != nil {
		return rt.transientRebase("root Texture RLM: %v", err)
	}
	if !live {
		return rebaseNoticeOutcome{Reason: "no live root Texture RLM"}
	}
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
	if next < types.RebaseNoticeMaxAttempts {
		out.Rearm = rt.rebaseAnswerWindowInterval() + rebaseMinDelay
	}
	return out
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
	snapshot, err := rt.store.GetLifecycleSnapshot(ctx, ownerID, computerID, trajectoryID)
	if err != nil {
		return "", "", false, err
	}
	if snapshot.Trajectory.Status != types.TrajectoryLive {
		return "", "", false, nil
	}
	if _, cancelErr := rt.store.GetLifecycleCancellationIntent(ctx, ownerID, computerID, trajectoryID); cancelErr == nil {
		return "", "", false, nil
	} else if !errors.Is(cancelErr, store.ErrNotFound) {
		return "", "", false, cancelErr
	}
	agentID := currentTextureAgentID(doc.DocID)
	for _, work := range snapshot.WorkItems {
		if work.Status == types.WorkItemOpen && work.AssignedAgentID == agentID {
			return agentID, doc.DocID, true, nil
		}
	}
	return "", "", false, nil
}

// rebaseAttemptClosed decides, from the tape, whether attempt n can no longer
// be answered, so attempt n+1 may follow. It is closed when any of these holds:
//   - it predates a markerless boot (a crash closed it);
//   - its inbox is not the current root (the master was replaced);
//   - no Cast from its root names it and the answer window has passed;
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
			return false, remaining + rebaseMinDelay, nil
		}
		return true, 0, nil
	}
	for _, cast := range casts {
		head, err := rt.store.ConversationState(ctx, ownerID, computerID, cast.ConversationID)
		if err != nil {
			return false, 0, err
		}
		if head.Open && head.Round == cast.Round {
			return false, window, nil
		}
	}
	return true, 0, nil
}

// rebaseNoticeCasts are the root's Casts to management that name the notice.
func (rt *Runtime) rebaseNoticeCasts(ctx context.Context, ownerID, computerID string, notice types.CommitmentRecord) ([]types.CommitmentRecord, error) {
	conversation := types.InboxConversationID(persistentManagementAgentID(ownerID), notice.Inbox)
	records, err := rt.store.ConversationRecords(ctx, ownerID, computerID, conversation)
	if err != nil {
		return nil, err
	}
	var out []types.CommitmentRecord
	for _, rec := range records {
		if rec.IsCast() && rec.SenderInbox == notice.Inbox && rec.Directive != nil && types.CastRebaseNotice(rec.Directive.Body) == notice.RecordID {
			out = append(out, rec)
		}
	}
	return out, nil
}
