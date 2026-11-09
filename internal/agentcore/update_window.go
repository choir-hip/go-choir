package agentcore

import (
	"strings"
	"sync"

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

// windowUpdatePackets returns the oldest packets whose cell views fit the
// frame's updates budget, and how many were left out.
func windowUpdatePackets(packets []types.CoagentSourcePacket) (shown []types.CoagentSourcePacket, remaining int) {
	if len(packets) == 0 {
		return nil, 0
	}
	views := make([]yaegikernel.PendingUpdate, len(packets))
	for i, u := range packets {
		views[i] = pendingUpdateView(u)
	}
	kept, remaining := yaegikernel.WindowUpdates(views, yaegikernel.FrameUpdatesBudget)
	return packets[:len(kept)], remaining
}

// wakeWindowIDs is the ids of the deliverable updates a cell's frame would
// show, so a wake turn names no update the desk cannot read in its cell. A
// cell's frame shows a subset of the deliverable updates (cellUpdateVisible
// adds the Texture turn boundary), so a budget prefix of this list is never
// longer than the frame's.
// updates is the desk's pending list in the store's order, before the seen
// filter: an update an earlier wake named still holds its place in the
// window until a report consumes it.
func wakeWindowIDs(rec *types.RunRecord, updates []types.CoagentSourcePacket) map[string]bool {
	deliverable := make([]types.CoagentSourcePacket, 0, len(updates))
	for _, u := range updates {
		if strings.TrimSpace(u.UpdateID) == "" || !coagentUpdateDeliverableForRun(rec, u) {
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
// frame carried every pending update: the run consumes as it always did.
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
