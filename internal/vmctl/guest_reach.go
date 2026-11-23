// Guest-reach decisions of this package, moved here whole from handlers.go so the
// security-epoch gate (nix/guest-security-surfaces.txt) sees a change to them. A
// change to this file changes what a guest computer may reach on the host.
// Keep it to those decisions; put anything else in another file.

package vmctl

import (
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/server"
)

// guestRemoteAddrHost returns the caller's source IP when the request
// arrived over a guest tap (any non-loopback source), else "". The S1a
// anti-spoof rule at the tap boundary makes this unforgeable from the
// guest side, so it is a sound guest-identity discriminator.
func guestRemoteAddrHost(r *http.Request) string {
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil && host != "127.0.0.1" && host != "::1" {
		return host
	}
	return ""
}

// bindRequestToGuestComputer checks that the caller's tap source IP binds,
// by the registry's one rule (GuestOwnershipForAddress), to the computer
// named by wantComputerID: the guest's computer_url host is its tap IP, and
// remote_addr post-DNAT carries that IP verbatim (observed 2026-10-04, s1a
// evidence). A stale ownership whose old address a newer computer now holds
// never binds. Returns nil when the caller is bound, an error otherwise.
// Loopback callers are rejected by the caller's loopback gate, not here.
func (h *Handler) bindRequestToGuestComputer(r *http.Request, wantComputerID string) error {
	srcHost := guestRemoteAddrHost(r)
	if srcHost == "" {
		return fmt.Errorf("guest-scoped call requires a tap-sourced request (remote_addr=%s)", r.RemoteAddr)
	}
	own := h.registry.GuestOwnershipForAddress(srcHost)
	if own == nil {
		return fmt.Errorf("caller %s is not bound to a live computer", srcHost)
	}
	if strings.TrimSpace(own.ComputerID) != strings.TrimSpace(wantComputerID) {
		return fmt.Errorf("caller %s is bound to computer %s, not %s", srcHost, own.ComputerID, wantComputerID)
	}
	return nil
}

// bindRequestToRouteSlot checks that the caller's tap source IP binds, by
// the registry's one rule, to the ownership named by a route slot's
// owner:desktop pair. The ComputerVersion route slot ID is
// computer:<owner>:<desktop> — its third segment is the desktop, not a
// stable computer ID. Returns nil when the caller is bound.
func (h *Handler) bindRequestToRouteSlot(r *http.Request, ownerID, desktopID string) error {
	srcHost := guestRemoteAddrHost(r)
	if srcHost == "" {
		return fmt.Errorf("guest-scoped call requires a tap-sourced request (remote_addr=%s)", r.RemoteAddr)
	}
	own := h.registry.GuestOwnershipForAddress(srcHost)
	if own == nil {
		return fmt.Errorf("caller %s is not bound to a live computer", srcHost)
	}
	if strings.TrimSpace(own.UserID) != strings.TrimSpace(ownerID) || normalizeDesktopID(own.DesktopID) != normalizeDesktopID(desktopID) {
		return fmt.Errorf("caller %s is not bound to route slot %s:%s", srcHost, ownerID, desktopID)
	}
	return nil
}

// callerBoundToAnyOwnership reports whether the request came over a guest
// tap AND its source IP binds to a live computer by the registry's one
// rule. Used for guest-scoped endpoints whose payload names no computer
// (the inputs-resolve POST): bound guest identity is enforced by the tap
// anti-spoof rule + this check, and what they read is a public artifact
// lookup, so any bound guest may call.
func (h *Handler) callerBoundToAnyOwnership(r *http.Request) bool {
	srcHost := guestRemoteAddrHost(r)
	if srcHost == "" || h.registry == nil {
		return false
	}
	return h.registry.GuestOwnershipForAddress(srcHost) != nil
}

// RegisterRoutes registers all vmctl routes on the given server.
// All control endpoints use the /internal/vmctl/ prefix to make it
// clear these are not public-facing routes (VAL-VM-012).
func RegisterRoutes(s *server.Server, h *Handler) {
	s.SetHealthHandler(h.HandleHealth)
	s.HandleFunc("/internal/vmctl/resolve", h.HandleResolve)
	s.HandleFunc("/internal/vmctl/computers/{computerID}/cold-recover", attributedLifecycle(h.HandleColdRecover))
	s.HandleFunc("/internal/vmctl/computer-version-inputs/resolve", h.HandleResolveComputerVersionInputs)
	s.HandleFunc("/internal/vmctl/computer-version-routes/resolve", h.HandleResolveComputerVersionRoute)
	s.HandleFunc("/internal/vmctl/computer-version-routes/apply-self-development", h.HandleApplySelfDevelopmentRouteProjection)
	s.HandleFunc("/internal/vmctl/computer-version-routes/apply-platform-follow", h.HandleApplyPlatformFollowRouteProjection)
	s.HandleFunc("/internal/vmctl/lookup", h.HandleLookup)
	s.HandleFunc("/internal/vmctl/lookup-guest", h.HandleLookupGuestOwnership)
	s.HandleFunc("/internal/vmctl/stop", attributedLifecycle(h.HandleStop))
	s.HandleFunc("/internal/vmctl/remove", attributedLifecycle(h.HandleRemove))
	s.HandleFunc("/internal/vmctl/list", h.HandleList)
	s.HandleFunc("/internal/vmctl/account/computers", h.HandleAccountComputers)
	s.HandleFunc("/internal/vmctl/account/stop", attributedLifecycle(h.HandleAccountStop))
	s.HandleFunc("/internal/vmctl/account/destroy", attributedLifecycle(h.HandleAccountDestroy))
	s.HandleFunc(effectiveReleaseReportPath, h.HandleEffectiveReleaseReport)
	s.HandleFunc(capsuleReservationsPath, h.HandleCapsuleReservations)
	s.HandleFunc("/internal/vmctl/maintenance-serve", attributedLifecycle(h.HandleMaintenanceServe))
	s.HandleFunc("/internal/vmctl/refresh", attributedLifecycle(h.HandleRefresh))
	s.HandleFunc("/internal/vmctl/deploy-refresh", attributedLifecycle(h.HandleDeployRefresh))
	s.HandleFunc("/internal/vmctl/update-available", h.HandleUpdateAvailable)
	s.HandleFunc("/internal/vmctl/capture-data-image", attributedLifecycle(h.HandleCaptureDataImage))
	s.HandleFunc("/internal/vmctl/idle-check", h.HandleIdleCheck)
	s.HandleFunc("/internal/vmctl/reclaim", attributedLifecycle(h.HandleReclaim))
	s.HandleFunc("/internal/vmctl/touch", h.HandleTouch)
	s.HandleFunc("/internal/vmctl/hold", h.HandleHold)
	s.HandleFunc("/internal/vmctl/unhold", h.HandleUnhold)
	s.HandleFunc("/internal/vmctl/divergence", h.HandleSetDivergence)
	s.HandleFunc("/internal/vmctl/retention-plan", h.HandleRetentionPlan)
	s.HandleFunc("/internal/vmctl/pulse", h.HandlePulse)
	s.HandleFunc("/internal/vmctl/boot-timeline", h.HandleBootTimeline)
	s.HandleFunc("/internal/vmctl/guest-releases", h.HandleGuestReleases)
	s.HandleFunc("/internal/vmctl/ring-walk", h.HandleRingWalk)
	s.HandleFunc("/internal/vmctl/ring-walk/expedite", attributedLifecycle(h.HandleRingWalkExpedite))
	s.HandleFunc(HostBootStatePath, h.HandleHostBootState)
	s.HandleFunc("/internal/vmctl/hibernate", attributedLifecycle(h.HandleHibernate))
	s.HandleFunc("/internal/vmctl/memory/balloon", h.HandleBalloonStats)
}
