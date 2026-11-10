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

// Reason codes of an owner update that restarted nothing. The chip names each
// by code, never by matching reason text.
const (
	ownerReasonInFlight     = "refresh_in_flight"
	ownerReasonRouteRefused = "route_refused"
	ownerReasonHeld         = "held"
	ownerReasonDivergent    = "divergent"
	ownerReasonNotActive    = "not_active"
	ownerReasonRingWaits    = "ring_waits"
	ownerReasonRetryLater   = "retry_later"
	ownerReasonFailed       = "failed"
	ownerReasonNoRelease    = "no_release"
	ownerReasonRestartRate  = "restart_rate_bound"
)

// restartInPlaceMinGap bounds the operator's restart in place: a second one of
// the same computer within it is refused. Five minutes is longer than a boot
// plus the guest's first settling (the wedge detector and health windows run
// in minutes), so a restart that did not cure the wedge is looked at, not
// repeated; and well inside the guest's 15 minute marker age, so an operator
// can try again in the same sitting. A correctness refusal like the
// one-in-flight rule, not a switch. In memory, as the tracker's failed-target
// map is.
const restartInPlaceMinGap = 5 * time.Minute

// ownerReasonCode names why an owner update that did not refresh or find the
// computer current restarted nothing. A guest refusal already carries its code.
func ownerReasonCode(resp deployRefreshResponse) string {
	if resp.ReasonCode != "" {
		return resp.ReasonCode
	}
	switch resp.Outcome {
	case deployRefreshDeferred:
		if resp.Reason == deployRefreshInFlightReason {
			return ownerReasonInFlight
		}
		return ownerReasonRetryLater
	case deployRefreshSkipped:
		switch {
		case resp.Reason == "held":
			return ownerReasonHeld
		case resp.Reason == "divergent":
			return ownerReasonDivergent
		case strings.HasPrefix(resp.Reason, "not active"):
			return ownerReasonNotActive
		case resp.Reason == "no release to update to":
			return ownerReasonNoRelease
		}
		// The ring gate is the only other skip.
		return ownerReasonRingWaits
	case deployRefreshFailed:
		return ownerReasonFailed
	}
	return ""
}

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
			UserID: own.UserID, DesktopID: own.DesktopID, State: string(own.State), ReasonCode: ownerReasonNoRelease}, http.StatusConflict
	}
	req.Target = commit
	if own.State != VMStateActive {
		// Nothing to restart; and never into trackedDeployRefresh, whose
		// realDeploy side effects (GrantDeployRetry, a rescue boot of a stopped
		// computer) are not an owner's click.
		return deployRefreshResponse{Outcome: deployRefreshSkipped, Reason: "not active (state=" + string(own.State) + ")", ReasonCode: ownerReasonNotActive,
			Target: commit, VMID: own.VMID, ComputerID: own.ComputerID, UserID: own.UserID, DesktopID: own.DesktopID, State: string(own.State)}, http.StatusConflict
	}
	if req.RestartInPlace {
		// Before any probe of the guest: a wedged guest must not make the
		// operator wait on /health, and a refused call asks nothing of it.
		return h.operatorRestartInPlace(ctx, req, own, commit)
	}
	// Before the gate, the marker or any pending entry: a computer already
	// on the release is a no-op. A served commit that cannot be read is
	// not "current".
	if served := h.servedCommit(ctx, *own); served != "" && served == commit {
		h.deployRefreshState().forget(own.UserID, own.DesktopID)
		return deployRefreshResponse{Outcome: deployRefreshCurrent, Target: commit, VMID: own.VMID, ComputerID: own.ComputerID,
			UserID: own.UserID, DesktopID: own.DesktopID, ComputerURL: own.ComputerURL, State: string(own.State)}, http.StatusOK
	}
	resp, status := h.trackedDeployRefresh(ctx, req, "owner-update")
	resp.Target = commit
	resp.ReasonCode = ownerReasonCode(resp)
	return resp, status
}

// restartProbeTimeout bounds the one served-commit probe of an operator
// restart; restartMarkTimeout bounds its planned-restart marker call. A
// guest that answers neither is wedged, which is what the verb is for. Vars so
// tests can shorten them.
var (
	restartProbeTimeout = 3 * time.Second
	restartMarkTimeout  = 10 * time.Second
)

// operatorRestartInPlace is the platform operator's restart: the same
// trackedDeployRefresh as an update (ring gate, held and divergent rules, one
// in flight, the guest's planned-restart marker and its busy refusals), onto
// the release the computer already serves. It adds the atomic rate bound and
// the outcome name: refreshed when the probe shows an update was pending and
// this applied it, restarted otherwise (including a probe that got no
// answer). The reservation is taken first, so a refused call asks nothing of
// the guest; the probe is bounded and made once. When the guest answers the
// marker call, its refusals (apply, live capsule, pending decision) hold and
// the restart resumes work (work=resumed). When it cannot answer, the restart
// is markerless, a crash boot that closes open work (work=closed), the safe
// outcome for a wedged computer (deployRefreshWith).
func (h *Handler) operatorRestartInPlace(ctx context.Context, req deployRefreshRequest, own *VMOwnership, commit string) (deployRefreshResponse, int) {
	_, clock := h.behindGate()
	tracker := h.deployRefreshState()
	finish, refusal := tracker.beginRestartInPlace(own.UserID, own.DesktopID, clock(), restartInPlaceMinGap)
	if refusal != "" {
		return deployRefreshResponse{Outcome: deployRefreshSkipped, ReasonCode: ownerReasonRestartRate, Target: commit,
			Reason: "an operator restart of this computer is running or was less than " + restartInPlaceMinGap.String() + " ago; nothing was restarted",
			VMID:   own.VMID, ComputerID: own.ComputerID, UserID: own.UserID, DesktopID: own.DesktopID, State: string(own.State)}, http.StatusConflict
	}
	charged := false
	defer func() { finish(charged) }()
	probeCtx, cancel := context.WithTimeout(ctx, restartProbeTimeout)
	served := h.servedCommit(probeCtx, *own)
	cancel()
	behind := served != "" && served != commit
	resp, status := h.trackedDeployRefresh(ctx, req, "operator-restart")
	resp.Target = commit
	resp.ReasonCode = ownerReasonCode(resp)
	if resp.Outcome == deployRefreshRefreshed || resp.Outcome == deployRefreshFailed {
		charged = true
	}
	if resp.Outcome == deployRefreshRefreshed {
		if !behind {
			resp.Outcome = deployRefreshRestarted
		}
		resp.Work = "closed"
		if resp.PlannedRestart {
			resp.Work = "resumed"
		}
	}
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
