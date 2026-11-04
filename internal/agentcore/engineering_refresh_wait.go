package agentcore

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// A deploy refresh deferred behind an engineer's own candidate
// (docs/problems/selfdev-waits-untyped-and-invisible-2026-11-04.md, live7
// trace review). The guest knows the update is waiting from the host's last
// request (refresh_hold.go); this renders that knowledge as the one typed
// wait (kind platform_update) for the engineer, in its Freeze result and
// run context, and through the existing tracker for management and Texture.
// No new mechanism: the record, the render and the relay are the waits'.

// pendingRefreshWait is the wait of the update the host last asked for, nil
// when no ask is fresh. frozen says the engineer's candidate is the thing it
// waits on; otherwise it waits on the round to finish.
func (rt *Runtime) pendingRefreshWait(assignmentID string, frozen bool, now time.Time) *types.WaitState {
	if rt == nil {
		return nil
	}
	h := &rt.refreshHold
	h.mu.Lock()
	fresh, target, since := h.freshLocked(now), h.target, h.askedSince
	h.mu.Unlock()
	if !fresh {
		return nil
	}
	if since.IsZero() {
		since = now
	}
	on := "this round to finish"
	if frozen {
		on = "this candidate's decision"
	}
	w := &types.WaitState{
		Kind: types.WaitKindPlatformUpdate, Class: types.WaitClassPolicy,
		Subject:  "assignment " + strings.TrimSpace(assignmentID),
		Resource: "the platform update to " + target,
		Since:    since.UTC(), Until: since.Add(refreshHoldMax).UTC(),
		Detail: fmt.Sprintf("an update to %s is waiting for %s; the computer takes it between rounds and holds new engineering until then", target, on),
	}
	w.Stamp(now)
	return w
}

// The tracked copy is observed once, at the Freeze: it ends with the
// assignment (Complete follows the Freeze), not when the ask lapses first.
// noteFreezeRefreshWait adds the pending update to a Freeze result (wait,
// wait_sentence) and to the assignment's tracked wait, which management's
// pack shows and Texture is told of once it has lasted a minute.
func (rt *Runtime) noteFreezeRefreshWait(ctx context.Context, rec *types.RunRecord, out map[string]any) {
	if rt == nil || rec == nil || out == nil {
		return
	}
	assignmentID := metadataStringValue(rec.Metadata, "assignment_id")
	now := rt.refreshHoldNow()
	w := rt.pendingRefreshWait(assignmentID, true, now)
	if w == nil {
		return
	}
	out["wait"] = w
	out["wait_sentence"] = w.Detail
	if rt.store == nil {
		return
	}
	assignment, err := rt.store.GetEngineeringAssignment(ctx, rec.OwnerID, rec.ComputerID, assignmentID, uint64(metadataIntValue(rec.Metadata, "assignment_attempt")))
	if err != nil {
		return
	}
	rt.observeAssignmentWait(ctx, assignment, func(since time.Time) *types.WaitState {
		return w.Clone()
	})
}

// engineerWaitViews is an engineer's run context: its own assignment's
// tracked wait, and the pending update when nothing tracked says so.
func (rt *Runtime) engineerWaitViews(ownerID, computerID, assignmentID string) []types.AssignmentWaitView {
	if rt == nil {
		return nil
	}
	var out []types.AssignmentWaitView
	for _, v := range rt.assignmentWaitViews(ownerID, computerID) {
		if v.AssignmentID == assignmentID {
			out = append(out, v)
		}
	}
	if len(out) == 0 || out[len(out)-1].Wait == nil {
		if w := rt.pendingRefreshWait(assignmentID, false, rt.refreshHoldNow()); w != nil {
			if len(out) == 0 {
				out = append(out, types.AssignmentWaitView{AssignmentID: assignmentID, Attempt: 0})
			}
			out[len(out)-1].Wait = w
		}
	}
	return out
}
