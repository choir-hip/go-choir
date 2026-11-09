package agentcore

import (
	"context"
	"log"
	"strings"
	"sync"

	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// The updates a desk cell is shown are a window of its pending updates
// (yaegikernel.WindowUpdates): the frame is one transport frame, and the
// Autopaper's management desk had 1000 pending, which overflowed it
// (docs/evidence/paper-token-budget-2026-11-09.md section 1.3).
//
// One rule keeps the window safe: nothing is delivered, incorporated or
// consumed unless the desk was shown it whole. Three places consume updates,
// and each is held to the window.
//
//   - The cell's frame shows the oldest window of the desk's pending updates
//     (pendingCellUpdatesWindow).
//   - The wake turn names update ids, and a Management report consumes the ids
//     its run memory shows were named. wakeWindowIDs cuts the wake to the same
//     window, so the ids a wake names are ids the frame shows.
//   - A Texture turn delivers every pending report it was not told about.
//     The host records the ids the run's cells were shown (noteUpdatesShown),
//     and the turn delivers only those (UpdatesShownRestriction).
//
// The window is the oldest pending updates, so a desk that disposes what it is
// shown drains the backlog in order. A desk that leaves the first window
// pending is shown the same window again, not the updates past it: the
// backlog does not starve a window's updates, but a window nobody disposes
// holds the ones behind it.

// UpdateTooLargeToShow is the visible cause an update is settled with when it
// cannot be shown whole in a cell frame.
const UpdateTooLargeToShow = "too_large_to_show"

// unshowableUpdate reports whether an update is too large to be shown whole.
func unshowableUpdate(u types.CoagentSourcePacket) bool {
	return yaegikernel.UpdateSize(pendingUpdateView(u)) > yaegikernel.FrameUpdateMaxBytes
}

// windowUpdatePackets returns the oldest packets whose cell views fit the
// frame's updates budget, and how many were left out. An update too large to
// be shown whole is in neither: it is settled (settleUnshowableUpdates), and
// is never the head of the window, named by a wake, or consumed.
func windowUpdatePackets(packets []types.CoagentSourcePacket) (shown []types.CoagentSourcePacket, remaining int) {
	showable := make([]types.CoagentSourcePacket, 0, len(packets))
	views := make([]yaegikernel.PendingUpdate, 0, len(packets))
	for _, u := range packets {
		view := pendingUpdateView(u)
		if yaegikernel.UpdateSize(view) > yaegikernel.FrameUpdateMaxBytes {
			continue
		}
		showable = append(showable, u)
		views = append(views, view)
	}
	if len(showable) == 0 {
		return nil, 0
	}
	kept, remaining := yaegikernel.WindowUpdates(views, yaegikernel.FrameUpdatesBudget)
	return showable[:len(kept)], remaining
}

// settleUnshowableUpdates settles each update in packets that is too large to
// be shown whole, with the visible cause too_large_to_show, and returns how
// many it settled. It is the rule's fate for an item that cannot reach the
// desk: not held at the head of the window, not re-named by every wake, not
// consumed as if read. A lifecycle update terminalizes through the
// delivery-reconcile command (as delivery_attempts_exhausted does); a mailbox
// update through SettleWorkerUpdate.
func (rt *Runtime) settleUnshowableUpdates(ctx context.Context, rec *types.RunRecord, packets []types.CoagentSourcePacket) int {
	if rt == nil || rt.store == nil || rec == nil {
		return 0
	}
	settled := 0
	for _, u := range packets {
		if !unshowableUpdate(u) {
			continue
		}
		id := strings.TrimSpace(u.UpdateID)
		var err error
		if u.LifecycleVersion > 0 {
			req := types.ReconcileUpdateDeliveryRequest{
				OwnerID: rec.OwnerID, ComputerID: rec.ComputerID,
				CommandID:    "too-large-to-show:" + id,
				TrajectoryID: u.TrajectoryID, TargetAgentID: u.TargetAgentID, MaxAttempts: 1,
				Items: []types.ReconcileUpdateDeliveryItem{{
					UpdateID: id, ProducerAgentID: u.AgentID, ProducerUpdateID: u.ProducerUpdateID,
					ExpectedLifecycleVersion: u.LifecycleVersion, ExpectedRunID: u.DeliveredToRunID,
					Exhaust: true, Reason: UpdateTooLargeToShow,
				}},
			}
			if req.CommandDigest, err = store.ComputeReconcileUpdateDeliveryDigest(req); err == nil {
				_, err = rt.store.ReconcileUpdateDelivery(ctx, req)
			}
		} else {
			err = rt.store.SettleWorkerUpdate(ctx, rec.OwnerID, u.TargetAgentID, id, rec.RunID, UpdateTooLargeToShow)
		}
		if err != nil {
			log.Printf("desk cell: settle too-large update %s for %s: %v", id, rec.AgentID, err)
			continue
		}
		log.Printf("desk cell: update %s for %s is too large to show in a cell frame; settled as %s", id, rec.AgentID, UpdateTooLargeToShow)
		settled++
	}
	return settled
}

// wakeWindowIDs is the ids of the deliverable updates a cell's frame would
// show, so a wake turn names no update the desk cannot read in its cell: it
// uses the frame's own predicate (cellUpdateVisible, which adds the Texture
// turn boundary to the wake's deliverable rule).
// updates is the desk's pending list in the store's order, before the seen
// filter: an update an earlier wake named still holds its place in the
// window until a report consumes it.
func wakeWindowIDs(rec *types.RunRecord, updates []types.CoagentSourcePacket) map[string]bool {
	deliverable := make([]types.CoagentSourcePacket, 0, len(updates))
	for _, u := range updates {
		if strings.TrimSpace(u.UpdateID) == "" || !cellUpdateVisible(rec, u) {
			continue
		}
		deliverable = append(deliverable, u)
	}
	shown, _ := windowUpdatePackets(deliverable)
	ids := make(map[string]bool, len(shown))
	for _, u := range shown {
		ids[strings.TrimSpace(u.UpdateID)] = true
	}
	return ids
}

// updatesShown is what a run's cells were shown of its pending updates.
type updatesShown struct {
	mu         sync.Mutex
	restricted bool
	ids        map[string]bool
}

// noteUpdatesShown folds the update ids a cell's frame carried into its run's.
// cut says the window left pending updates out (or the transport cut the
// frame); from then on the run consumes only what it was shown. A run whose
// frames always carried every pending update is never restricted, and
// consumes as before.
func (rt *Runtime) noteUpdatesShown(runID string, ids []string, cut bool) {
	runID = strings.TrimSpace(runID)
	if rt == nil || runID == "" {
		return
	}
	v, loaded := rt.updatesRunShown.Load(runID)
	if !loaded {
		if !cut {
			return
		}
		v, _ = rt.updatesRunShown.LoadOrStore(runID, &updatesShown{ids: map[string]bool{}})
	}
	s := v.(*updatesShown)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.restricted = true
	for _, id := range ids {
		if id = strings.TrimSpace(id); id != "" {
			s.ids[id] = true
		}
	}
}

// UpdatesShownRestriction reports, for a run whose frames left pending updates
// out, the ids its cells were shown whole. restricted is false when every
// frame carried every pending update: the run consumes as it always did. It
// reports only what cells recorded; a run with no record is bounded by
// FirstUpdateWindow.
func (rt *Runtime) UpdatesShownRestriction(runID string) (shown map[string]bool, restricted bool) {
	if rt == nil {
		return nil, false
	}
	v, ok := rt.updatesRunShown.Load(strings.TrimSpace(runID))
	if !ok {
		return nil, false
	}
	s := v.(*updatesShown)
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.restricted {
		return nil, false
	}
	out := make(map[string]bool, len(s.ids))
	for id := range s.ids {
		out[id] = true
	}
	return out, true
}

// FirstUpdateWindow is the fail-safe for a run no cell recorded for (a commit
// with no cell, a run resumed after a planned restart, a parked run whose
// record was dropped): when its pending updates are more than one window, it
// consumes only the first window, the oldest within the frame's budget, never
// everything. restricted is false when they all fit one window.
func FirstUpdateWindow(pending []types.CoagentSourcePacket) (shown map[string]bool, restricted bool) {
	window, remaining := windowUpdatePackets(pending)
	if remaining == 0 {
		return nil, false
	}
	shown = make(map[string]bool, len(window))
	for _, u := range window {
		shown[strings.TrimSpace(u.UpdateID)] = true
	}
	return shown, true
}

// consumableUpdateIDs cuts ids (a run's pending updates, from the store) to
// what the run may consume: what its cells were shown, or, when no cell
// recorded, the first window.
func (rt *Runtime) consumableUpdateIDs(ctx context.Context, rec *types.RunRecord, ids []string) []string {
	shown, restricted := rt.UpdatesShownRestriction(rec.RunID)
	if !restricted && len(ids) > 1 {
		packets := make([]types.CoagentSourcePacket, 0, len(ids))
		for _, id := range ids {
			p, err := rt.store.GetWorkerUpdate(ctx, rec.OwnerID, strings.TrimSpace(id))
			if err != nil {
				// Unreadable: it is not consumed, and stays pending.
				continue
			}
			packets = append(packets, p)
		}
		shown, restricted = FirstUpdateWindow(packets)
		if !restricted {
			return ids
		}
	}
	if !restricted {
		return ids
	}
	kept := ids[:0:0]
	for _, id := range ids {
		if shown[strings.TrimSpace(id)] {
			kept = append(kept, id)
		}
	}
	return kept
}
