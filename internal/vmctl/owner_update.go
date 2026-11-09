package vmctl

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

// The owner update (docs/design/update-now-2026-11-09.md): the owner's click
// "Restart to update" is a deploy refresh of one computer, asked with
// allow_busy, onto the release a boot of that computer launches now. It runs
// through trackedDeployRefresh, never /internal/vmctl/refresh, so the ring
// gate, the held and divergent rules, the one-in-flight rule, the
// planned-restart marker and the pending bookkeeping are the deploy's own.

// ownerUpdate resolves the target (pin-aware guestReleaseFor, never
// caller-chosen), answers "current" when the guest already serves it, and
// otherwise runs the tracked deploy refresh with allow_busy.
func (h *Handler) ownerUpdate(ctx context.Context, req deployRefreshRequest) (deployRefreshResponse, int) {
	own := h.registry.GetOwnershipForDesktop(req.UserID, req.DesktopID)
	if own == nil {
		return deployRefreshResponse{Outcome: deployRefreshSkipped, Reason: "no computer", UserID: req.UserID, DesktopID: req.DesktopID}, http.StatusNotFound
	}
	_, commit := h.guestReleaseFor(stableComputerID(own.UserID, own.DesktopID, own.ComputerID))
	if commit == "" {
		return deployRefreshResponse{Outcome: deployRefreshSkipped, Reason: "no release to update to", VMID: own.VMID, ComputerID: own.ComputerID,
			UserID: own.UserID, DesktopID: own.DesktopID, State: string(own.State)}, http.StatusConflict
	}
	req.Target = commit
	if own.State == VMStateActive {
		// Before the gate, the marker or any pending entry: a computer already
		// on the release is a no-op. A served commit that cannot be read is
		// not "current".
		if served := h.servedCommit(ctx, *own); served != "" && served == commit {
			h.deployRefreshState().forget(own.UserID, own.DesktopID)
			return deployRefreshResponse{Outcome: deployRefreshCurrent, Target: commit, VMID: own.VMID, ComputerID: own.ComputerID,
				UserID: own.UserID, DesktopID: own.DesktopID, ComputerURL: own.ComputerURL, State: string(own.State)}, http.StatusOK
		}
	}
	resp, status := h.trackedDeployRefresh(ctx, req, "owner-update")
	resp.Target = commit
	return resp, status
}

// guestRefusalCode reads a short code and the raw body from a guest's busy
// refusal. A body that is not JSON yields the generic code and no body.
func guestRefusalCode(detail string) (string, json.RawMessage) {
	var r struct {
		Reason                  string `json:"reason"`
		SelfdevActiveOps        int    `json:"selfdev_active_operations"`
		LiveCapsules            int    `json:"live_capsules"`
		SnapshotInFlight        bool   `json:"snapshot_in_flight"`
		SelfdevAwaitingDecision int    `json:"selfdev_awaiting_decision"`
	}
	if json.Unmarshal([]byte(detail), &r) != nil {
		return "busy", nil
	}
	code := "busy"
	switch {
	case r.Reason != "":
		code = r.Reason
	case r.SelfdevAwaitingDecision > 0:
		code = "selfdev_awaiting_decision"
	case r.LiveCapsules > 0:
		code = "live_capsule"
	case r.SelfdevActiveOps > 0:
		code = "selfdev_active"
	case r.SnapshotInFlight:
		code = "snapshot_in_flight"
	}
	return code, json.RawMessage(detail)
}

// pendingEntryFor returns the computer's pending entry, if any.
func (t *deployRefreshTracker) pendingEntryFor(userID, desktopID string) (deployRefreshPendingEntry, bool) {
	key := ownershipKey(userID, normalizeDesktopID(desktopID))
	t.mu.Lock()
	defer t.mu.Unlock()
	entry, ok := t.pending[key]
	return entry, ok
}

// UpdateAvailableResponse is vmctl's host-side "updates available" signal.
type UpdateAvailableResponse struct {
	Available bool   `json:"available"`
	Target    string `json:"target,omitempty"`
	Since     string `json:"since,omitempty"`
}

// updateAvailable: the computer is active, vmctl has a pending entry for it
// (a deploy or the sweep deferred it), the ring gate admits the ring-intended
// target, and the served commit differs. A ring-held computer shows nothing;
// a rolled-back one shows the older target.
func (h *Handler) updateAvailable(ctx context.Context, userID, desktopID string) UpdateAvailableResponse {
	own := h.registry.GetOwnershipForDesktop(userID, desktopID)
	if own == nil || own.State != VMStateActive || own.IsHeld() {
		return UpdateAvailableResponse{}
	}
	entry, pending := h.deployRefreshState().pendingEntryFor(own.UserID, own.DesktopID)
	if !pending {
		return UpdateAvailableResponse{}
	}
	h.syncRingWalk()
	_, target := h.guestReleaseFor(stableComputerID(own.UserID, own.DesktopID, own.ComputerID))
	if target == "" || h.ringGate(own, target) != "" {
		return UpdateAvailableResponse{}
	}
	if h.servedCommit(ctx, *own) == target {
		return UpdateAvailableResponse{}
	}
	out := UpdateAvailableResponse{Available: true, Target: target}
	if !entry.FirstDeferredAt.IsZero() {
		out.Since = entry.FirstDeferredAt.UTC().Format(time.RFC3339)
	}
	return out
}

// HandleUpdateAvailable serves GET /internal/vmctl/update-available.
func (h *Handler) HandleUpdateAvailable(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeVMCTLJSON(w, http.StatusMethodNotAllowed, vmctlErrorResponse{Error: "method not allowed"})
		return
	}
	if !isInternalCaller(r) {
		writeVMCTLJSON(w, http.StatusForbidden, vmctlErrorResponse{Error: "vmctl control endpoints are not publicly accessible"})
		return
	}
	userID := strings.TrimSpace(r.URL.Query().Get("user_id"))
	if userID == "" {
		writeVMCTLJSON(w, http.StatusBadRequest, vmctlErrorResponse{Error: "user_id is required"})
		return
	}
	writeVMCTLJSON(w, http.StatusOK, h.updateAvailable(r.Context(), userID, normalizeDesktopID(r.URL.Query().Get("desktop_id"))))
}
