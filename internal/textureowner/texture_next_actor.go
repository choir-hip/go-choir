package textureowner

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentcore"
	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// texture_next_actor.go — a Texture turn never leaves the owner's request
// open with no next actor, silently
// (docs/problems/texture-write-leaves-owner-request-unattended-2026-11-04.md).
//
// A request has a next actor when the turn names one (a Cast, Ask or Note
// in the cell, a Wait saying what it waits on, or Done), or when something
// already works on it: an open round Texture cast to research or management,
// a live engineering assignment on the trajectory, or a live
// self-development operation. Without one:
//   - a desk turn is refused at commit, nothing committed, with
//     unattendedRequestRefusal (the guard; the model fixes it in the same
//     activation);
//   - the runtime's Idle turn for an activation that staged nothing commits,
//     and the request's work item is refused with UnattendedRequestReason,
//     which the owner sees on the document and management in its inbox (the
//     backstop). The owner's next revise reopens work.
//
// Stories and publications keep their Texture work open by design (new
// captures wake them), and the Texture root's standing item is clamped
// open, so neither is checked.

// UnattendedRequestReason prefixes the fate of a Texture request the
// runtime closed because nothing was working on it. Like
// TriggerNotDisposedReason, the runtime closed it, not the desk, so the
// owner's next revise reopens work.
const UnattendedRequestReason = "no one is working on this request"

// unattendedRequestRefusal is the guard's refusal, worded for the model.
const unattendedRequestRefusal = "the owner's request is still open and nothing is working on it: Cast to start the work, Wait(why) if it waits on something named, or Done(why) if this revision answers it. Nothing was committed; put the same Write in the cell with one of them"

// turnNamesNextActor reports whether the turn itself says what happens to
// the request next.
func turnNamesNextActor(turn yaegikernel.TextureTurn) bool {
	return strings.TrimSpace(turn.Done) != "" || strings.TrimSpace(turn.Wait) != "" || len(turn.InboxRecords) > 0
}

// openTextureRequest returns the open Texture work item assigned to agentID
// on a trajectory whose Texture work is an owner request, or false.
func openTextureRequest(snapshot types.LifecycleSnapshot, agentID string) (types.WorkItemRecord, bool) {
	switch snapshot.Trajectory.Kind {
	case types.TrajectoryKindStory, types.TrajectoryKindPublication:
		return types.WorkItemRecord{}, false
	}
	agentID = strings.TrimSpace(agentID)
	for _, work := range snapshot.WorkItems {
		if work.Status == types.WorkItemOpen && work.AssignedAgentID == agentID && strings.TrimSpace(work.AuthorityProfile) == deskmanifest.DocumentCustodianName() {
			return work, true
		}
	}
	return types.WorkItemRecord{}, false
}

// liveEngineeringAssignment names a trajectory engineering assignment that
// is not terminal, or "".
func liveEngineeringAssignment(snapshot types.LifecycleSnapshot) string {
	for _, assignment := range snapshot.EngineeringAssignments {
		if !assignment.Disposition.Terminal() {
			return assignment.AssignmentID
		}
	}
	return ""
}

// unattendedTextureRequest returns the open owner request on doc that
// nothing works on, or false when there is none or something works on it.
// It does not look at the turn.
func (h *Handler) unattendedTextureRequest(ctx context.Context, rec *types.RunRecord, doc types.Document) (types.WorkItemRecord, bool, error) {
	if h == nil || h.Store == nil || rec == nil || strings.TrimSpace(doc.TrajectoryID) == "" {
		return types.WorkItemRecord{}, false, nil
	}
	if _, isRoot := h.textureRootFor(ctx, rec.OwnerID, doc.ComputerID, doc.DocID); isRoot {
		return types.WorkItemRecord{}, false, nil
	}
	snapshot, err := h.Store.GetLifecycleSnapshot(ctx, rec.OwnerID, doc.ComputerID, doc.TrajectoryID)
	if err != nil {
		return types.WorkItemRecord{}, false, fmt.Errorf("load the Texture request's trajectory: %w", err)
	}
	work, open := openTextureRequest(snapshot, rec.AgentID)
	if !open {
		return types.WorkItemRecord{}, false, nil
	}
	textureInbox := strings.TrimSpace(rec.AgentID)
	rounds, err := h.Store.OpenRoundsCastBy(ctx, rec.OwnerID, doc.ComputerID, textureInbox)
	if err != nil {
		return types.WorkItemRecord{}, false, fmt.Errorf("read the Texture desk's open rounds: %w", err)
	}
	operationID := ""
	if h.Core != nil {
		operationID, _ = h.Core.LiveSelfDevelopmentOperation(doc.ComputerID, doc.TrajectoryID)
	}
	if textureRequestServer(snapshot, len(rounds), operationID) != "" {
		return types.WorkItemRecord{}, false, nil
	}
	return work, true, nil
}

// textureRequestServer names what works on an open Texture request, or "":
// a live engineering assignment (which carries a running capsule job), an
// open round the Texture desk cast (a Cast to research or
// management, its child runs included), or a live self-development operation.
func textureRequestServer(snapshot types.LifecycleSnapshot, openRounds int, operationID string) string {
	switch {
	case liveEngineeringAssignment(snapshot) != "":
		return "engineering assignment"
	case openRounds > 0:
		return "open round"
	case strings.TrimSpace(operationID) != "":
		return "self-development operation"
	}
	return ""
}

// refuseUnattendedTurn is the guard: a desk turn that names no next actor
// while the owner's request is open and nothing works on it is refused.
// The runtime's Idle turn is not a desk turn; the backstop answers it.
func (h *Handler) refuseUnattendedTurn(ctx context.Context, rec *types.RunRecord, doc types.Document, turn yaegikernel.TextureTurn) error {
	if turn.Idle != "" || turnNamesNextActor(turn) {
		return nil
	}
	_, unattended, err := h.unattendedTextureRequest(ctx, rec, doc)
	if err != nil {
		return err
	}
	if unattended {
		return fmt.Errorf("choir: turn refused: %s", unattendedRequestRefusal)
	}
	return nil
}

// closeUnattendedRequest is the backstop, run after the runtime's Idle turn
// for an activation that staged nothing: when the owner's request is still
// open and nothing works on it, its work item is refused with
// UnattendedRequestReason and management gets a notice. Both commits are
// idempotent in the work item and the run.
func (h *Handler) closeUnattendedRequest(ctx context.Context, rec *types.RunRecord, docID string, turn yaegikernel.TextureTurn) error {
	if h == nil || rec == nil || strings.TrimSpace(turn.Idle) != agentcore.TextureIdleNoActReason {
		return nil
	}
	cause := fmt.Sprintf("Texture's turn (run %s) ended without a Cast, a Wait naming what it waits on, or Done, and no round, engineering assignment or self-development operation is working on it; revise again to continue", strings.TrimSpace(rec.RunID))
	return h.closeUnattendedWith(ctx, rec, docID, strings.TrimSpace(turn.BaseRevisionID), cause, nil)
}

// closeUnservedWait is the Wait leg of the same backstop. A Wait is accepted
// as a next actor for any text (turnNamesNextActor) but nothing arms a wake
// for it, so once the turn has committed the request is asked again and, when
// nothing serves it, closed with the Wait's text on the fate. It never
// re-dispatches or arms anything. A Cast in the same turn commits with the
// turn, so its round is already visible here.
func (h *Handler) closeUnservedWait(ctx context.Context, rec *types.RunRecord, docID string, turn yaegikernel.TextureTurn) error {
	if h == nil || rec == nil || strings.TrimSpace(turn.Wait) == "" || strings.TrimSpace(turn.Done) != "" || turn.Idle != "" {
		return nil
	}
	waited := strings.Join(strings.Fields(turn.Wait), " ")
	if r := []rune(waited); len(r) > 240 {
		waited = string(r[:240]) + "..."
	}
	cause := fmt.Sprintf("Texture's turn (run %s) waits (%q), but no round, engineering assignment, self-development operation or unread record is working on it, and nothing wakes it; revise again to continue", strings.TrimSpace(rec.RunID), waited)
	return h.closeUnattendedWith(ctx, rec, docID, "", cause, func(ctx context.Context, doc types.Document) (bool, error) {
		return h.waitServedByWake(ctx, rec, doc, turn)
	})
}

// waitServedByWake reports whether something will wake the Texture desk
// after a committed Wait turn, beyond what unattendedTextureRequest reads: a
// later head (the owner revised, which opened work and woke the desk), another
// live activation of the same desk, a pending lifecycle update packet not yet
// bound to a run, an Ask or Escalate still being worked, or an unread waking record in the desk's own inbox (the
// run-end records re-check wakes it).
func (h *Handler) waitServedByWake(ctx context.Context, rec *types.RunRecord, doc types.Document, turn yaegikernel.TextureTurn) (bool, error) {
	head, err := h.Store.GetLifecycleRevision(ctx, rec.OwnerID, doc.ComputerID, doc.CurrentRevisionID)
	if err != nil {
		return false, fmt.Errorf("load the Texture head: %w", err)
	}
	if turn.Write == nil {
		if base := strings.TrimSpace(turn.BaseRevisionID); base != "" && base != head.RevisionID {
			return true, nil
		}
	} else if head.AuthorKind != types.AuthorAppAgent {
		return true, nil
	}
	snapshot, err := h.Store.GetLifecycleSnapshot(ctx, rec.OwnerID, doc.ComputerID, doc.TrajectoryID)
	if err != nil {
		return false, fmt.Errorf("load the Texture request's trajectory: %w", err)
	}
	if a := snapshot.Activation; a.RunID != "" && a.RunID != rec.RunID && (a.State == types.RunPending || a.State == types.RunRunning) {
		return true, nil
	}
	agentID := strings.TrimSpace(rec.AgentID)
	if pendingTextureUpdate(snapshot, agentID) {
		return true, nil
	}
	// An Ask or Escalate opens no round, but its resolver was woken holding
	// the question and its Reply will wake this desk.
	outstanding, err := h.Store.OutstandingDispatches(ctx, rec.OwnerID, doc.ComputerID, agentID)
	if err != nil {
		return false, fmt.Errorf("read the Texture desk's outstanding asks: %w", err)
	}
	if len(outstanding) > 0 {
		return true, nil
	}
	cursor, err := h.Store.ReaderCursor(ctx, rec.OwnerID, doc.ComputerID, agentID, agentID)
	if err != nil {
		return false, fmt.Errorf("read the Texture inbox cursor: %w", err)
	}
	inboxHead, err := h.Store.InboxHead(ctx, rec.OwnerID, doc.ComputerID, agentID)
	if err != nil {
		return false, fmt.Errorf("read the Texture inbox head: %w", err)
	}
	if inboxHead <= cursor {
		return false, nil
	}
	unread, err := h.Store.ListInboxRecords(ctx, rec.OwnerID, doc.ComputerID, agentID, cursor, inboxHead)
	if err != nil {
		return false, fmt.Errorf("read the Texture inbox: %w", err)
	}
	for _, r := range unread {
		if store.RecordWakesReader(r) {
			return true, nil
		}
	}
	return false, nil
}

// pendingTextureUpdate reports whether a lifecycle update packet addressed
// to agentID is pending and not yet bound to a run: durable input the owner's
// recovery delivers to the desk, at a boot too.
func pendingTextureUpdate(snapshot types.LifecycleSnapshot, agentID string) bool {
	for _, u := range snapshot.Updates {
		pending := u.Disposition == "" || u.Disposition == types.UpdatePending
		if pending && strings.TrimSpace(u.TargetAgentID) == agentID && strings.TrimSpace(u.DeliveredToRunID) == "" && u.DeliveredAt == nil {
			return true
		}
	}
	return false
}

// closeUnservedAtBoot is the crash boot's leg: work is an open Texture work
// item whose run the boot cancelled. The fate is "interrupted by a restart";
// nothing is resumed, re-dispatched or armed.
func (h *Handler) closeUnservedAtBoot(ctx context.Context, work types.WorkItemRecord) error {
	if h == nil || h.Store == nil {
		return nil
	}
	agentID := strings.TrimSpace(work.AssignedAgentID)
	docID := strings.TrimPrefix(agentID, deskmanifest.DocumentCustodianName()+":")
	if docID == agentID || docID == "" || strings.TrimSpace(work.AuthorityProfile) != deskmanifest.DocumentCustodianName() {
		return nil
	}
	rec := &types.RunRecord{RunID: "crash-boot", OwnerID: work.OwnerID, ComputerID: work.ComputerID, AgentID: agentID, ChannelID: docID, TrajectoryID: work.TrajectoryID}
	return h.closeUnattendedWith(ctx, rec, docID, "", "Texture's run ended with a restart (interrupted by a restart), and no round, engineering assignment or self-development operation is working on it; revise again to continue",
		// A pending update packet is input the owner's recovery still
		// delivers after a boot, so its work is served.
		func(ctx context.Context, doc types.Document) (bool, error) {
			snapshot, err := h.Store.GetLifecycleSnapshot(ctx, work.OwnerID, doc.ComputerID, doc.TrajectoryID)
			if err != nil {
				return false, fmt.Errorf("load the Texture request's trajectory: %w", err)
			}
			return pendingTextureUpdate(snapshot, agentID), nil
		})
}

// closeUnattendedWith refuses the owner's open request on docID with
// UnattendedRequestReason and cause, and notifies management. baseRevisionID,
// when set, is the head the closing turn answered: a moved head means an
// owner revision opened work this turn never saw. served, when set, is asked
// last and keeps the request open when it says something will wake the desk.
func (h *Handler) closeUnattendedWith(ctx context.Context, rec *types.RunRecord, docID, baseRevisionID, cause string, served func(context.Context, types.Document) (bool, error)) error {
	ctx = asTextureReader(ctx)
	if h == nil || h.Store == nil || rec == nil || strings.TrimSpace(docID) == "" {
		return nil
	}
	doc, err := h.Store.GetLifecycleDocument(ctx, rec.OwnerID, strings.TrimSpace(rec.ComputerID), docID)
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("load the Texture document: %w", err)
	}
	// Only the request the turn answered: an owner revision since then moved
	// the head and opened work this activation never saw.
	if base := baseRevisionID; base != "" && base != strings.TrimSpace(doc.CurrentRevisionID) {
		return nil
	}
	work, unattended, err := h.unattendedTextureRequest(ctx, rec, doc)
	if err != nil || !unattended {
		return err
	}
	if served != nil {
		if ok, err := served(ctx, doc); err != nil || ok {
			return err
		}
	}
	runID := strings.TrimSpace(rec.RunID)
	reason := UnattendedRequestReason + ": " + cause
	sum := sha256.Sum256([]byte(strings.Join([]string{rec.OwnerID, doc.ComputerID, doc.TrajectoryID, work.WorkItemID, runID}, "\x00")))
	key := hex.EncodeToString(sum[:16])
	refuse := types.RefuseLifecycleWorkRequest{
		OwnerID: rec.OwnerID, ComputerID: doc.ComputerID, CommandID: "texture-request-unattended:" + key,
		TrajectoryID: doc.TrajectoryID, WorkItemID: work.WorkItemID, ActingAgentID: rec.AgentID,
		RefusalRef: "unattended:" + runID, Reason: reason,
	}
	if refuse.CommandDigest, err = store.ComputeRefuseLifecycleWorkDigest(refuse); err != nil {
		return err
	}
	if _, err := h.Store.RefuseLifecycleWork(ctx, refuse); err != nil {
		return fmt.Errorf("refuse unattended Texture request %s: %w", work.WorkItemID, err)
	}
	if owner := strings.TrimSpace(rec.OwnerID); owner != "" {
		wait := unattendedRequestWait(work, reason, time.Now())
		body := fmt.Sprintf("No one is working on the owner's request on document %s (Texture work %s, %q): %s. Wait record: %s.", doc.DocID, work.WorkItemID, work.Objective, reason, wait.Sentence)
		notice := types.NewUnattendedRequestNotice(deskmanifest.AdmitterInbox(owner), doc.ComputerID, doc.DocID, doc.TrajectoryID, work.WorkItemID, runID, body)
		req := types.CommitInboxRecordsRequest{OwnerID: owner, ComputerID: doc.ComputerID, CommandID: "unattended-notice:" + key, Records: []types.CommitmentRecord{notice}}
		if req.CommandDigest, err = store.ComputeCommitInboxRecordsDigest(req); err != nil {
			return err
		}
		if _, err := h.Store.CommitInboxRecords(ctx, req); err != nil {
			// The refusal is the owner-visible state; the notice is
			// management's copy of it, so its failure is logged, not fatal.
			log.Printf("texture: unattended request %s: notice to management: %v", work.WorkItemID, err)
		}
	}
	h.NoteTextureRootChange(context.WithoutCancel(ctx), rec.OwnerID, doc.ComputerID, doc.DocID)
	log.Printf("texture: %s (document %s, work %s)", reason, doc.DocID, work.WorkItemID)
	return nil
}

// unattendedRequestWait renders an unattended request through the typed
// wait record (types.WaitState): a policy wait of kind no_next_actor, since
// the request opened, that only the owner's next revise ends.
func unattendedRequestWait(work types.WorkItemRecord, reason string, now time.Time) *types.WaitState {
	wait := &types.WaitState{
		Kind: types.WaitKindNoNextActor, Class: types.WaitClassPolicy,
		Subject:  "request " + work.WorkItemID,
		Resource: "the owner's next revise; no one is working on this request",
		Need:     "a Cast, a Wait naming what it waits on, or Done from the Texture desk",
		Since:    work.CreatedAt, Detail: strings.TrimSpace(reason),
	}
	wait.Stamp(now)
	return wait
}

// textureRequestUnattendedWait is textureRequestUnattended as the typed wait
// record, or nil.
func textureRequestUnattendedWait(snapshot types.LifecycleSnapshot, docID string, now time.Time) *types.WaitState {
	work, ok := latestTextureWork(snapshot, docID)
	if !ok || work.Status != types.WorkItemRefused || !strings.HasPrefix(strings.TrimSpace(work.Reason), UnattendedRequestReason) {
		return nil
	}
	return unattendedRequestWait(work, work.Reason, now)
}

func latestTextureWork(snapshot types.LifecycleSnapshot, docID string) (types.WorkItemRecord, bool) {
	textureAgentID := deskmanifest.DocumentInbox(docID)
	var latest *types.WorkItemRecord
	for i := range snapshot.WorkItems {
		work := &snapshot.WorkItems[i]
		if work.AssignedAgentID != textureAgentID || strings.TrimSpace(work.AuthorityProfile) != deskmanifest.DocumentCustodianName() {
			continue
		}
		if latest == nil || work.UpdatedAt.After(latest.UpdatedAt) {
			latest = work
		}
	}
	if latest == nil {
		return types.WorkItemRecord{}, false
	}
	return *latest, true
}

// textureRequestUnattended returns the reason when the document's latest
// Texture work item was closed by the backstop, or "".
func textureRequestUnattended(snapshot types.LifecycleSnapshot, docID string) string {
	work, ok := latestTextureWork(snapshot, docID)
	if ok && work.Status == types.WorkItemRefused && strings.HasPrefix(strings.TrimSpace(work.Reason), UnattendedRequestReason) {
		return strings.TrimSpace(work.Reason)
	}
	return ""
}
