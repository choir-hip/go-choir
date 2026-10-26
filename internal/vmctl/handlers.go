package vmctl

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerversion"
	"github.com/yusefmosiah/go-choir/internal/hostauth"
	"github.com/yusefmosiah/go-choir/internal/server"
)

// errorResponse is a JSON error envelope.
type vmctlErrorResponse struct {
	Error string `json:"error"`
	// Recovery refusal fields: reason, condition kind, retry hint, the frozen
	// input witness and the repair job status. They are omitted for ordinary
	// errors so existing clients decode `error` unchanged.
	Reason            string                `json:"reason,omitempty"`
	Kind              string                `json:"kind,omitempty"`
	RetryAfterSeconds int                   `json:"retry_after_seconds,omitempty"`
	Witness           *RecoveryInputWitness `json:"witness,omitempty"`
	Repair            *CheckpointJobStatus  `json:"repair,omitempty"`
}

// writeVMCTLRecoveryRefusal renders a durable recovery refusal as a structured
// 503 with Retry-After. Known blocked state must fail promptly and durably.
func writeVMCTLRecoveryRefusal(w http.ResponseWriter, refusal *RecoveryRefusal) {
	if refusal == nil {
		return
	}
	retryAfter := refusal.RetryAfterSeconds
	if retryAfter <= 0 {
		retryAfter = recoveryRetryAfterSeconds
	}
	w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
	writeVMCTLJSON(w, http.StatusServiceUnavailable, vmctlErrorResponse{
		Error:             "computer recovery blocked",
		Reason:            refusal.Reason,
		Kind:              string(refusal.Kind),
		RetryAfterSeconds: retryAfter,
		Witness:           &refusal.Witness,
		Repair:            refusal.Job,
	})
}

// vmctlHealthResponse is the JSON structure for GET /health.
type vmctlHealthResponse struct {
	Status          string                `json:"status"`
	Service         string                `json:"service"`
	ActiveVMs       int                   `json:"active_vms"`
	TotalOwnerships int                   `json:"total_ownerships"`
	IdleEligible    int                   `json:"idle_eligible"`
	ByState         map[string]int        `json:"by_state,omitempty"`
	ByKind          map[string]int        `json:"by_kind,omitempty"`
	Reclaim         PressureReclaimPlan   `json:"reclaim"`
	Warmness        WarmnessHealthSummary `json:"warmness"`
	// Processes is the latest reconcile pass: live guests outside the
	// registry (waiting to reattach), wedged stops, orphan reaps.
	Processes VMProcessReconcileResult `json:"processes"`
	// Admission is the start admission ledger: reservations for starts in
	// flight and the recent decisions with their inputs.
	Admission HostAdmissionSummary `json:"admission"`
	// GuestReleaseCurrent and GuestReleases are the D2 release facts: the
	// current release and each computer launched from one, with whether it is
	// behind (guest_release.go). Absent while the switch is off.
	GuestReleaseCurrent string                 `json:"guest_release_current,omitempty"`
	GuestReleases       []GuestReleaseComputer `json:"guest_releases,omitempty"`
}

// resolveRequest is the JSON payload for POST /internal/vmctl/resolve.
type resolveRequest struct {
	UserID    string `json:"user_id"`
	DesktopID string `json:"desktop_id,omitempty"`
	// Force bypasses divergence protection on refresh.
	Force bool `json:"force,omitempty"`
}

// resolveResponse is the JSON response for POST /internal/vmctl/resolve.
type resolveResponse struct {
	VMID          string `json:"vm_id"`
	ComputerID    string `json:"computer_id"`
	UserID        string `json:"user_id"`
	DesktopID     string `json:"desktop_id"`
	Kind          VMKind `json:"kind,omitempty"`
	WarmnessClass string `json:"warmness_class,omitempty"`
	ComputerURL   string `json:"computer_url"`
	State         string `json:"state"`
}

// ownershipResponse is the JSON response for ownership queries.
type ownershipResponse struct {
	VMID                      string                           `json:"vm_id"`
	ComputerID                string                           `json:"computer_id"`
	UserID                    string                           `json:"user_id"`
	DesktopID                 string                           `json:"desktop_id"`
	Kind                      VMKind                           `json:"kind,omitempty"`
	WarmnessClass             string                           `json:"warmness_class,omitempty"`
	ComputerURL               string                           `json:"computer_url"`
	State                     string                           `json:"state"`
	Held                      bool                             `json:"held,omitempty"`
	CreatedAt                 string                           `json:"created_at"`
	LastActiveAt              string                           `json:"last_active_at"`
	Epoch                     int64                            `json:"epoch"`
	StoppedBy                 string                           `json:"stopped_by,omitempty"`
	DataImage                 *dataImageResponse               `json:"data_image,omitempty"`
	SnapshotKind              string                           `json:"snapshot_kind,omitempty"`
	ConstructionVersion       *computerversion.ComputerVersion `json:"construction_version,omitempty"`
	ConstructionDiskReceiptID string                           `json:"construction_disk_receipt_id,omitempty"`
	DivergenceStatus          string                           `json:"divergence_status,omitempty"`
	PlatformBaseRef           string                           `json:"platform_base_ref,omitempty"`
	// MemoryPeakRSSBytes is the peak measured memory of the last start;
	// admission estimates the next start from it.
	MemoryPeakRSSBytes uint64 `json:"memory_peak_rss_bytes,omitempty"`
	MemoryMeasuredAt   string `json:"memory_measured_at,omitempty"`
	// MemorySettledRSSBytes is the resident memory its last start settled
	// at; admission with VMCTL_ADMISSION_NEED=wake charges it (density D4).
	MemorySettledRSSBytes uint64 `json:"memory_settled_rss_bytes,omitempty"`
	// Current memory, read without SSH: the latest host RSS sample, the
	// base size, and with a hotplug device the plugged and requested
	// totals and the lease (memory_hotplug.go). From the controller cache;
	// the list never calls Firecracker.
	MemoryRSSBytes        uint64 `json:"memory_rss_bytes,omitempty"`
	MemoryBaseMiB         int    `json:"memory_base_mib,omitempty"`
	MemoryCurrentMiB      int    `json:"memory_current_mib,omitempty"`
	MemoryRequestedMiB    int    `json:"memory_requested_mib,omitempty"`
	MemoryPluggedMiB      int    `json:"memory_plugged_mib,omitempty"`
	MemoryHotplugPhase    string `json:"memory_hotplug_phase,omitempty"`
	MemoryLeaseExpiresAt  string `json:"memory_lease_expires_at,omitempty"`
	MemoryHotplugSelected bool   `json:"memory_hotplug_selected,omitempty"`
	// ReportedRelease is the computer's last effective-release report (P6):
	// release, tree, event head, CodeRef. Shown beside construction_version,
	// never merged into it.
	ReportedRelease *ReportedRelease `json:"reported_release,omitempty"`
	// The wake-orphan fix's row fields (boot_claim.go), for the operator:
	// an open start's claim, the consecutive interrupted starts, and when
	// the automatic failure retry was last spent. Read-only.
	BootClaim          *bootClaimResponse `json:"boot_claim,omitempty"`
	StartInterruptions int                `json:"start_interruptions,omitempty"`
	FailureWarmedAt    string             `json:"failure_warmed_at,omitempty"`
}

// bootClaimResponse is a copy of a row's open BootClaim, so the answer
// never shares the registry's pointer.
type bootClaimResponse struct {
	Kind      string `json:"kind"`
	Epoch     int64  `json:"epoch"`
	ClaimedAt string `json:"claimed_at,omitempty"`
}

func bootClaimResponseFor(claim *BootClaim) *bootClaimResponse {
	if claim == nil {
		return nil
	}
	return &bootClaimResponse{Kind: claim.Kind, Epoch: claim.Epoch, ClaimedAt: claim.ClaimedAt}
}

type reclaimResponse struct {
	Status            string               `json:"status"`
	VMsReclaimed      int                  `json:"vms_reclaimed"`
	RetentionPruned   int                  `json:"retention_pruned"`
	VMsStopped        int                  `json:"vms_stopped"`
	QuarantinesPruned int                  `json:"quarantines_pruned"`
	ReclaimBefore     PressureReclaimPlan  `json:"reclaim_before"`
	ReclaimAfter      PressureReclaimPlan  `json:"reclaim_after"`
	Retention         RetentionPruneResult `json:"retention"`
}

// Handler provides HTTP handlers for the vmctl service.
type Handler struct {
	registry               *OwnershipRegistry
	routeAuthority         *RouteAuthority
	routeAuthorityRequired bool
	coldRecoveryMu         sync.Mutex
	coldRecoveryState      *coldRecoveryState
	// guestMarker asks a guest to write its planned-restart marker before a
	// deploy refresh (nil: the HTTP client).
	guestMarker guestPlannedRestartMarker
	// deployRefreshMu guards the lazily built deploy refresh tracker (the
	// pending set the retry sweep works through, and the in-flight guard).
	// storeCompact runs guest store compactions (store_compaction.go); built
	// on first use unless a test sets it.
	storeCompactOnce sync.Once
	storeCompact     *storeCompactState
	// storeCompactPressure configures the disk-pressure compaction loop
	// (store_compaction_pressure.go); set before StartStoreCompactPressure.
	storeCompactPressure StoreCompactPressureConfig
	// dataImageStatsFor reads a VM's host data image stats (nil: the
	// registry's); tests substitute it.
	dataImageStatsFor    func(vmID string) (DataImageStats, bool)
	deployRefreshMu      sync.Mutex
	deployRefreshTracker *deployRefreshTracker
	// guestServedCommit reads the build a guest serves (nil: its /health).
	guestServedCommit func(context.Context, VMOwnership) string
}

// NewHandler creates a vmctl Handler with the given ownership registry.
func NewHandler(registry *OwnershipRegistry) *Handler {
	return &Handler{registry: registry}
}

// writeJSON writes a JSON response.
func writeVMCTLJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("vmctl: json encode error: %v", err)
	}
}

// HandleHealth handles GET /health for the vmctl service.
func (h *Handler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeVMCTLJSON(w, http.StatusMethodNotAllowed, vmctlErrorResponse{Error: "method not allowed"})
		return
	}

	ownerships := h.registry.ListOwnerships()
	byState := make(map[string]int)
	byKind := make(map[string]int)
	for _, own := range ownerships {
		byState[string(own.State)]++
		byKind[string(own.Kind)]++
	}
	// Health never probes a guest: one that does not answer would make
	// vmctl itself look down (docs/problems/vmctl-health-fate-shares-a-hung-guest-2026-10-10.md).
	idleEligible := h.registry.IdleOwnershipsSeen()
	releaseCurrent, releaseComputers := h.guestReleaseHealth(ownerships)
	writeVMCTLJSON(w, http.StatusOK, vmctlHealthResponse{
		Status:          "ok",
		Service:         "vmctl",
		ActiveVMs:       h.registry.ActiveCount(),
		TotalOwnerships: len(ownerships),
		IdleEligible:    len(idleEligible),
		ByState:         byState,
		ByKind:          byKind,
		Reclaim:         h.registry.PressureReclaimPlanSeen(),
		Warmness:        h.registry.WarmnessSummary(idleEligible),
		Processes:       h.registry.LastProcessReconcile(),
		Admission:       h.registry.HostAdmissionSummary(),

		GuestReleaseCurrent: releaseCurrent,
		GuestReleases:       releaseComputers,
	})
}

// HandleLookupGuestOwnership answers "which bound computer owns this guest
// source IP?" for host-internal services that need to authenticate a
// tap-sourced caller (maild, corpusd, proxy). It is a host-only endpoint:
// isInternalCaller admits loopback/UDS only, and the response is keyed by
// RemoteAddr, which a guest cannot forge under tap anti-spoofing. The
// answer is the registry's one binding rule (GuestOwnershipForAddress):
// the single live computer at that address, never a stale one.
// POST /internal/vmctl/lookup-guest {"remote_addr":"10.200.0.2"} ->
//
//	200 {"found":true,"user_id":...,"computer_url":...,"computer_id":...}
//	200 {"found":false}
func (h *Handler) HandleLookupGuestOwnership(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeVMCTLJSON(w, http.StatusMethodNotAllowed, vmctlErrorResponse{Error: "method not allowed"})
		return
	}
	if !isInternalCaller(r) {
		writeVMCTLJSON(w, http.StatusForbidden, vmctlErrorResponse{Error: "vmctl control endpoints are not publicly accessible"})
		return
	}
	var req struct {
		RemoteAddr string `json:"remote_addr"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
		writeVMCTLJSON(w, http.StatusBadRequest, vmctlErrorResponse{Error: "invalid request body"})
		return
	}
	wantHost := req.RemoteAddr
	if host, _, err := net.SplitHostPort(wantHost); err == nil {
		wantHost = host
	}
	wantHost = strings.TrimSpace(wantHost)
	if wantHost == "" {
		writeVMCTLJSON(w, http.StatusBadRequest, vmctlErrorResponse{Error: "remote_addr is required"})
		return
	}
	if own := h.registry.GuestOwnershipForAddress(wantHost); own != nil {
		writeVMCTLJSON(w, http.StatusOK, map[string]any{
			"found":        true,
			"user_id":      own.UserID,
			"desktop_id":   own.DesktopID,
			"computer_id":  own.ComputerID,
			"computer_url": own.ComputerURL,
			"vm_id":        own.VMID,
		})
		return
	}
	writeVMCTLJSON(w, http.StatusOK, map[string]any{"found": false})
}

// HandleResolve handles POST /internal/vmctl/resolve.
// Given a user ID, it resolves or assigns a VM for that user.
// This is the primary endpoint the proxy calls to route authenticated
// requests through VM ownership (VAL-VM-001).
//
// HandleResolve finds or boots the VM for a user/desktop pair.
// This endpoint is internal-only and must not be exposed publicly
// (VAL-VM-012). The proxy is the only intended caller.
func (h *Handler) HandleResolve(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeVMCTLJSON(w, http.StatusMethodNotAllowed, vmctlErrorResponse{Error: "method not allowed"})
		return
	}

	// Enforce internal-only access (VAL-VM-012).
	if !isInternalCaller(r) {
		writeVMCTLJSON(w, http.StatusForbidden, vmctlErrorResponse{
			Error: "vmctl control endpoints are not publicly accessible",
		})
		return
	}

	var req resolveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeVMCTLJSON(w, http.StatusBadRequest, vmctlErrorResponse{Error: "invalid request body"})
		return
	}

	if req.UserID == "" {
		writeVMCTLJSON(w, http.StatusBadRequest, vmctlErrorResponse{Error: "user_id is required"})
		return
	}
	req.DesktopID = normalizeDesktopID(req.DesktopID)
	route, routeKnown, err := h.resolveComputerVersionRoute(r.Context(), req.UserID, req.DesktopID)
	if err != nil {
		writeVMCTLJSON(w, http.StatusConflict, vmctlErrorResponse{Error: err.Error()})
		return
	}
	expectedVMID := ""
	if routeKnown && !route.RouteAbsent {
		existing := h.registry.GetOwnershipForDesktop(req.UserID, req.DesktopID)
		if existing == nil {
			writeVMCTLJSON(w, http.StatusConflict, vmctlErrorResponse{
				Error: "immutable ComputerVersion route has no matching realized ownership",
			})
			return
		}
		_, _, constructed, joinErr := h.routeAuthority.constructedOwnershipIdentity(
			r.Context(), req.UserID, req.DesktopID, existing.VMID,
		)
		if joinErr != nil || !constructed {
			writeVMCTLJSON(w, http.StatusConflict, vmctlErrorResponse{
				Error: "immutable ComputerVersion route does not join realized ownership",
			})
			return
		}
		expectedVMID = existing.VMID
	}
	if req.UserID == UniversalWirePlatformOwnerID && req.DesktopID == UniversalWirePlatformDesktopID {
		var own *VMOwnership
		if routeKnown && !route.RouteAbsent {
			own, err = h.registry.resolveExistingDesktopContext(r.Context(), req.UserID, req.DesktopID, expectedVMID)
		} else {
			if err = h.registry.EnsureUniversalWirePlatformComputer(r.Context()); err == nil {
				own = h.registry.GetOwnershipForDesktop(req.UserID, req.DesktopID)
			}
		}
		if own == nil {
			if refusal := RecoveryRefusalFrom(err); refusal != nil {
				log.Printf("vmctl: platform computer recovery blocked: %s", refusal.Reason)
				writeVMCTLRecoveryRefusal(w, refusal)
				return
			}
			log.Printf("vmctl: resolve platform computer lookup failed after ensure")
			writeVMCTLJSON(w, http.StatusInternalServerError, vmctlErrorResponse{Error: "failed to resolve platform computer"})
			return
		}
		writeVMCTLJSON(w, http.StatusOK, resolveResponse{
			VMID:          own.VMID,
			ComputerID:    stableComputerID(own.UserID, own.DesktopID, own.ComputerID),
			UserID:        own.UserID,
			DesktopID:     own.DesktopID,
			Kind:          own.Kind,
			WarmnessClass: string(h.registry.WarmnessClassForOwnership(own)),
			ComputerURL:   own.ComputerURL,
			State:         string(own.State),
		})
		return
	}
	if req.DesktopID != PrimaryDesktopID {
		writeVMCTLJSON(w, http.StatusBadRequest, vmctlErrorResponse{
			Error: "resolve can provision only the primary desktop",
		})
		return
	}

	var own *VMOwnership
	if routeKnown && !route.RouteAbsent {
		own, err = h.registry.resolveExistingDesktopContext(r.Context(), req.UserID, req.DesktopID, expectedVMID)
	} else {
		own, err = h.registry.ResolveOrAssignDesktopContext(r.Context(), req.UserID, req.DesktopID)
	}
	if err != nil {
		if refusal := RecoveryRefusalFrom(err); refusal != nil {
			log.Printf("vmctl: resolve blocked for user %s desktop %s: %s", req.UserID, req.DesktopID, refusal.Reason)
			writeVMCTLRecoveryRefusal(w, refusal)
			return
		}
		// A coalesced waiter released by a sibling refusal carries no typed
		// error; the durable condition is still the truthful reason.
		if refusal := h.recordedRecoveryRefusal(req.UserID, req.DesktopID); refusal != nil {
			log.Printf("vmctl: resolve blocked (recorded condition) for user %s desktop %s: %s", req.UserID, req.DesktopID, refusal.Reason)
			writeVMCTLRecoveryRefusal(w, refusal)
			return
		}
		log.Printf("vmctl: resolve failed for user %s desktop %s: %v", req.UserID, req.DesktopID, err)
		status := http.StatusInternalServerError
		if routeKnown && !route.RouteAbsent {
			status = http.StatusConflict
		}
		writeVMCTLJSON(w, status, vmctlErrorResponse{Error: "failed to resolve VM"})
		return
	}

	writeVMCTLJSON(w, http.StatusOK, resolveResponse{
		VMID:          own.VMID,
		ComputerID:    stableComputerID(own.UserID, own.DesktopID, own.ComputerID),
		UserID:        own.UserID,
		DesktopID:     own.DesktopID,
		Kind:          own.Kind,
		WarmnessClass: string(h.registry.WarmnessClassForOwnership(own)),
		ComputerURL:   own.ComputerURL,
		State:         string(own.State),
	})
}

// recordedRecoveryRefusal renders the durable condition for a user/desktop
// whose resolve failed for another reason (for example a coalesced waiter
// released by a sibling refusal): the durable reason is still the truth.
func (h *Handler) recordedRecoveryRefusal(userID, desktopID string) *RecoveryRefusal {
	if h == nil || h.registry == nil {
		return nil
	}
	own := h.registry.GetOwnershipForDesktop(userID, desktopID)
	if own == nil {
		return nil
	}
	condition, ok := h.registry.RecoveryConditionFor(stableComputerID(own.UserID, own.DesktopID, own.ComputerID))
	if !ok {
		return nil
	}
	return &RecoveryRefusal{
		ComputerID:        condition.ComputerID,
		Kind:              condition.Kind,
		Reason:            condition.Reason,
		Witness:           condition.Witness,
		Job:               condition.Job,
		RetryAfterSeconds: recoveryRetryAfterSeconds,
	}
}

// HandleLookup handles GET /internal/vmctl/lookup?user_id=...
// Returns the current ownership for a user without creating a new VM.
func (h *Handler) HandleLookup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeVMCTLJSON(w, http.StatusMethodNotAllowed, vmctlErrorResponse{Error: "method not allowed"})
		return
	}

	if !isInternalCaller(r) {
		writeVMCTLJSON(w, http.StatusForbidden, vmctlErrorResponse{
			Error: "vmctl control endpoints are not publicly accessible",
		})
		return
	}

	userID := strings.TrimSpace(r.URL.Query().Get("user_id"))
	computerID := strings.TrimSpace(r.URL.Query().Get("computer_id"))
	if userID == "" && computerID == "" {
		writeVMCTLJSON(w, http.StatusBadRequest, vmctlErrorResponse{Error: "user_id or computer_id query parameter is required"})
		return
	}
	var own *VMOwnership
	if computerID != "" && userID == "" {
		own = h.registry.GetOwnershipByComputerID(computerID)
	} else if computerID != "" {
		own = h.registry.GetOwnershipForComputer(userID, computerID)
	} else {
		own = h.registry.GetOwnershipForDesktop(userID, normalizeDesktopID(r.URL.Query().Get("desktop_id")))
	}
	if own != nil {
		if err := h.requireComputerVersionRoute(r.Context(), own.UserID, own.DesktopID); err != nil {
			writeVMCTLJSON(w, http.StatusConflict, vmctlErrorResponse{Error: err.Error()})
			return
		}
	}
	own = h.registry.reconcileLookupReadiness(own)
	if own == nil {
		writeVMCTLJSON(w, http.StatusNotFound, vmctlErrorResponse{Error: "no VM found for target"})
		return
	}
	if own.IsReady() {
		h.registry.ensureExistingGatewayCredential(own.VMID)
	}

	var dataImage *dataImageResponse
	if stats, ok := h.registry.DataImageStatsForVM(own.VMID); ok {
		dataImage = dataImageResponseFromStats(stats)
	}
	writeVMCTLJSON(w, http.StatusOK, ownershipResponse{
		VMID:                  own.VMID,
		ComputerID:            stableComputerID(own.UserID, own.DesktopID, own.ComputerID),
		UserID:                own.UserID,
		DesktopID:             own.DesktopID,
		Kind:                  own.Kind,
		WarmnessClass:         string(h.registry.WarmnessClassForOwnership(own)),
		ComputerURL:           own.ComputerURL,
		State:                 string(own.State),
		CreatedAt:             own.CreatedAt.Format("2006-01-02T15:04:05.000Z"),
		LastActiveAt:          own.LastActiveAt.Format("2006-01-02T15:04:05.000Z"),
		Epoch:                 own.Epoch,
		StoppedBy:             own.StoppedBy,
		DataImage:             dataImage,
		MemoryPeakRSSBytes:    own.MemoryPeakRSSBytes,
		MemoryMeasuredAt:      own.MemoryMeasuredAt,
		MemorySettledRSSBytes: own.MemorySettledRSSBytes,
		BootClaim:             bootClaimResponseFor(own.BootClaim),
		StartInterruptions:    own.StartInterruptions,
		FailureWarmedAt:       own.FailureWarmedAt,
	})
}

// HandleStop handles POST /internal/vmctl/stop.
// Stops the VM for a given user.
func (h *Handler) HandleStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeVMCTLJSON(w, http.StatusMethodNotAllowed, vmctlErrorResponse{Error: "method not allowed"})
		return
	}

	if !isInternalCaller(r) {
		writeVMCTLJSON(w, http.StatusForbidden, vmctlErrorResponse{
			Error: "vmctl control endpoints are not publicly accessible",
		})
		return
	}

	var req resolveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeVMCTLJSON(w, http.StatusBadRequest, vmctlErrorResponse{Error: "invalid request body"})
		return
	}

	if req.UserID == "" {
		writeVMCTLJSON(w, http.StatusBadRequest, vmctlErrorResponse{Error: "user_id is required"})
		return
	}
	req.DesktopID = normalizeDesktopID(req.DesktopID)
	if err := h.requireComputerVersionRoute(r.Context(), req.UserID, req.DesktopID); err != nil {
		writeVMCTLJSON(w, http.StatusConflict, vmctlErrorResponse{Error: err.Error()})
		return
	}

	if err := h.registry.StopVMForDesktop(req.UserID, req.DesktopID); err != nil {
		writeVMCTLJSON(w, http.StatusNotFound, vmctlErrorResponse{Error: err.Error()})
		return
	}

	writeVMCTLJSON(w, http.StatusOK, map[string]string{"status": "stopped"})
}

// HandleRemove handles POST /internal/vmctl/remove.
// Removes the ownership for a user (used during logout).
// holdRequest is the JSON payload for POST /internal/vmctl/hold and
// POST /internal/vmctl/unhold.
type holdRequest struct {
	ComputerID string `json:"computer_id"`
	Reason     string `json:"reason,omitempty"`
	HeldBy     string `json:"held_by,omitempty"`
}

// HandleHold places a computer under a durable host-authoritative maintenance
// hold. While held, vmctl refuses any automatic lifecycle action for the VM
// and the deploy active-VM refresh skips it. This is an authorized owner/
// recovery operation, not a reconciler-driven call.
func (h *Handler) HandleHold(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeVMCTLJSON(w, http.StatusMethodNotAllowed, vmctlErrorResponse{Error: "method not allowed"})
		return
	}
	if !isInternalCaller(r) {
		writeVMCTLJSON(w, http.StatusForbidden, vmctlErrorResponse{Error: "vmctl control endpoints are not publicly accessible"})
		return
	}
	var req holdRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeVMCTLJSON(w, http.StatusBadRequest, vmctlErrorResponse{Error: "invalid request body"})
		return
	}
	if req.ComputerID == "" {
		writeVMCTLJSON(w, http.StatusBadRequest, vmctlErrorResponse{Error: "computer_id is required"})
		return
	}
	if err := h.registry.SetHold(req.ComputerID, req.Reason, req.HeldBy); err != nil {
		writeVMCTLJSON(w, http.StatusNotFound, vmctlErrorResponse{Error: err.Error()})
		return
	}
	writeVMCTLJSON(w, http.StatusOK, map[string]string{"status": "held", "computer_id": req.ComputerID})
}

type touchRequest struct {
	UserID     string `json:"user_id"`
	ComputerID string `json:"computer_id"`
}

type touchResponse struct {
	Touched bool   `json:"touched"`
	State   string `json:"state,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

// keyTouchEnabled reports the VMCTL_KEY_TOUCH switch. It defaults on; "off",
// "0", "false" or "no" turns the route into a no-op that answers
// {"touched":false,"reason":"disabled"}.
func keyTouchEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("VMCTL_KEY_TOUCH"))) {
	case "off", "0", "false", "no":
		return false
	}
	return true
}

// HandleTouch moves a computer's idle clock for an authenticated API-key
// request the proxy served. It is host-internal (loopback or the vmctl socket,
// with the host-internal token), joins on the exact (user, computer) pair, and
// never resolves, wakes or starts a computer: an API key still cannot wake a
// hibernated computer.
func (h *Handler) HandleTouch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeVMCTLJSON(w, http.StatusMethodNotAllowed, vmctlErrorResponse{Error: "method not allowed"})
		return
	}
	if !isInternalCaller(r) {
		writeVMCTLJSON(w, http.StatusForbidden, vmctlErrorResponse{Error: "vmctl control endpoints are not publicly accessible"})
		return
	}
	var req touchRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&req); err != nil {
		writeVMCTLJSON(w, http.StatusBadRequest, vmctlErrorResponse{Error: "invalid request body"})
		return
	}
	if strings.TrimSpace(req.UserID) == "" || strings.TrimSpace(req.ComputerID) == "" {
		writeVMCTLJSON(w, http.StatusBadRequest, vmctlErrorResponse{Error: "user_id and computer_id are required"})
		return
	}
	if !keyTouchEnabled() {
		writeVMCTLJSON(w, http.StatusOK, touchResponse{Reason: "disabled"})
		return
	}
	result := h.registry.TouchComputerActivity(req.UserID, req.ComputerID, time.Now())
	if !result.Found {
		writeVMCTLJSON(w, http.StatusNotFound, vmctlErrorResponse{Error: "no VM found for target"})
		return
	}
	writeVMCTLJSON(w, http.StatusOK, touchResponse{Touched: result.Touched, State: string(result.State)})
}

// HandleUnhold removes a maintenance hold from a computer. This is an
// authorized owner/recovery operation, not a reconciler-driven call.
func (h *Handler) HandleUnhold(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeVMCTLJSON(w, http.StatusMethodNotAllowed, vmctlErrorResponse{Error: "method not allowed"})
		return
	}
	if !isInternalCaller(r) {
		writeVMCTLJSON(w, http.StatusForbidden, vmctlErrorResponse{Error: "vmctl control endpoints are not publicly accessible"})
		return
	}
	var req holdRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeVMCTLJSON(w, http.StatusBadRequest, vmctlErrorResponse{Error: "invalid request body"})
		return
	}
	if req.ComputerID == "" {
		writeVMCTLJSON(w, http.StatusBadRequest, vmctlErrorResponse{Error: "computer_id is required"})
		return
	}
	if err := h.registry.ClearHold(req.ComputerID); err != nil {
		writeVMCTLJSON(w, http.StatusNotFound, vmctlErrorResponse{Error: err.Error()})
		return
	}
	writeVMCTLJSON(w, http.StatusOK, map[string]string{"status": "unheld", "computer_id": req.ComputerID})
}

type setDivergenceRequest struct {
	ComputerID       string `json:"computer_id"`
	DivergenceStatus string `json:"divergence_status"`
	PlatformBaseRef  string `json:"platform_base_ref,omitempty"`
}

// HandleSetDivergence updates the divergence classification for a computer.
func (h *Handler) HandleSetDivergence(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeVMCTLJSON(w, http.StatusMethodNotAllowed, vmctlErrorResponse{Error: "method not allowed"})
		return
	}
	if !isInternalCaller(r) {
		writeVMCTLJSON(w, http.StatusForbidden, vmctlErrorResponse{Error: "vmctl control endpoints are not publicly accessible"})
		return
	}
	var req setDivergenceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeVMCTLJSON(w, http.StatusBadRequest, vmctlErrorResponse{Error: "invalid request body"})
		return
	}
	if req.ComputerID == "" {
		writeVMCTLJSON(w, http.StatusBadRequest, vmctlErrorResponse{Error: "computer_id is required"})
		return
	}
	if req.DivergenceStatus == "" {
		writeVMCTLJSON(w, http.StatusBadRequest, vmctlErrorResponse{Error: "divergence_status is required"})
		return
	}
	if err := h.registry.SetDivergenceStatus(req.ComputerID, req.DivergenceStatus, req.PlatformBaseRef); err != nil {
		writeVMCTLJSON(w, http.StatusNotFound, vmctlErrorResponse{Error: err.Error()})
		return
	}
	writeVMCTLJSON(w, http.StatusOK, map[string]string{
		"status":            "updated",
		"computer_id":       req.ComputerID,
		"divergence_status": req.DivergenceStatus,
	})
}
func (h *Handler) HandleRemove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeVMCTLJSON(w, http.StatusMethodNotAllowed, vmctlErrorResponse{Error: "method not allowed"})
		return
	}

	if !isInternalCaller(r) {
		writeVMCTLJSON(w, http.StatusForbidden, vmctlErrorResponse{
			Error: "vmctl control endpoints are not publicly accessible",
		})
		return
	}

	var req resolveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeVMCTLJSON(w, http.StatusBadRequest, vmctlErrorResponse{Error: "invalid request body"})
		return
	}

	if req.UserID == "" {
		writeVMCTLJSON(w, http.StatusBadRequest, vmctlErrorResponse{Error: "user_id is required"})
		return
	}
	req.DesktopID = normalizeDesktopID(req.DesktopID)
	if err := h.requireComputerVersionRoute(r.Context(), req.UserID, req.DesktopID); err != nil {
		writeVMCTLJSON(w, http.StatusConflict, vmctlErrorResponse{Error: err.Error()})
		return
	}

	_ = h.registry.RemoveOwnershipForDesktop(req.UserID, req.DesktopID)
	writeVMCTLJSON(w, http.StatusOK, map[string]string{"status": "removed"})
}

// HandleMaintenanceServe boots a held, route-bound computer under the
// guest-visible maintenance fence. It is an internal recovery exception:
// the hold must already exist, and replayOnly is deliberately false so the
// restored desktop becomes servable while Runtime.Start still refuses all
// mutation and self-development admission.
func (h *Handler) HandleMaintenanceServe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeVMCTLJSON(w, http.StatusMethodNotAllowed, vmctlErrorResponse{Error: "method not allowed"})
		return
	}
	if !isInternalCaller(r) {
		writeVMCTLJSON(w, http.StatusForbidden, vmctlErrorResponse{Error: "vmctl control endpoints are not publicly accessible"})
		return
	}
	var req resolveRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req); err != nil {
		writeVMCTLJSON(w, http.StatusBadRequest, vmctlErrorResponse{Error: "invalid request body"})
		return
	}
	if strings.TrimSpace(req.UserID) == "" {
		writeVMCTLJSON(w, http.StatusBadRequest, vmctlErrorResponse{Error: "user_id is required"})
		return
	}
	req.DesktopID = normalizeDesktopID(req.DesktopID)
	if err := h.requireComputerVersionRoute(r.Context(), req.UserID, req.DesktopID); err != nil {
		writeVMCTLJSON(w, http.StatusConflict, vmctlErrorResponse{Error: err.Error()})
		return
	}
	own := h.registry.GetOwnershipForDesktop(req.UserID, req.DesktopID)
	if own == nil || !own.IsHeld() {
		writeVMCTLJSON(w, http.StatusConflict, vmctlErrorResponse{Error: "maintenance serve requires an existing host maintenance hold"})
		return
	}
	recovered, err := h.registry.RecoverVMForDesktopMaintenance(req.UserID, req.DesktopID, false)
	if err != nil {
		writeVMCTLJSON(w, http.StatusConflict, vmctlErrorResponse{Error: err.Error()})
		return
	}
	writeVMCTLJSON(w, http.StatusOK, map[string]interface{}{
		"status": "maintenance_serving", "vm_id": recovered.VMID,
		"computer_id": recovered.ComputerID, "desktop_id": recovered.DesktopID,
		"computer_url": recovered.ComputerURL, "state": recovered.State,
		"epoch": recovered.Epoch, "held": recovered.IsHeld(),
	})
}

// HandleRefresh handles POST /internal/vmctl/refresh.
// It force-reboots an active computer onto the current guest image while
// preserving persistent state. It is the explicit operator/owner refresh: it
// asks the guest for no planned-restart marker, so the boot is a crash boot.
// Deploys use /internal/vmctl/deploy-refresh, which goes through the marker
// and defers busy computers.
func (h *Handler) HandleRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeVMCTLJSON(w, http.StatusMethodNotAllowed, vmctlErrorResponse{Error: "method not allowed"})
		return
	}

	if !isInternalCaller(r) {
		writeVMCTLJSON(w, http.StatusForbidden, vmctlErrorResponse{
			Error: "vmctl control endpoints are not publicly accessible",
		})
		return
	}

	var req resolveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeVMCTLJSON(w, http.StatusBadRequest, vmctlErrorResponse{Error: "invalid request body"})
		return
	}

	if req.UserID == "" {
		writeVMCTLJSON(w, http.StatusBadRequest, vmctlErrorResponse{Error: "user_id is required"})
		return
	}
	req.DesktopID = normalizeDesktopID(req.DesktopID)
	if err := h.requireComputerVersionRoute(r.Context(), req.UserID, req.DesktopID); err != nil {
		writeVMCTLJSON(w, http.StatusConflict, vmctlErrorResponse{Error: err.Error()})
		return
	}

	existing := h.registry.GetOwnershipForDesktop(req.UserID, req.DesktopID)
	if existing != nil && existing.DivergenceStatus == "divergent" && !req.Force {
		writeVMCTLJSON(w, http.StatusConflict, vmctlErrorResponse{
			Error: fmt.Sprintf("vmctl: computer %s is classified as divergent; refusing automatic refresh without force=true", existing.ComputerID),
		})
		return
	}

	own, err := h.registry.RefreshVMForDesktop(req.UserID, req.DesktopID)
	if err != nil {
		if refusal := RecoveryRefusalFrom(err); refusal != nil {
			writeVMCTLRecoveryRefusal(w, refusal)
			return
		}
		writeVMCTLJSON(w, http.StatusNotFound, vmctlErrorResponse{Error: err.Error()})
		return
	}

	writeVMCTLJSON(w, http.StatusOK, resolveResponse{
		VMID:        own.VMID,
		ComputerID:  stableComputerID(own.UserID, own.DesktopID, own.ComputerID),
		UserID:      own.UserID,
		DesktopID:   own.DesktopID,
		Kind:        own.Kind,
		ComputerURL: own.ComputerURL,
		State:       string(own.State),
	})
}

func (h *Handler) runReclaimSweep(ctx context.Context) reclaimResponse {
	before := h.registry.PressureReclaimPlan()
	reclaimed := h.registry.ReclaimPressureVMs(ctx, h.AuthorizeComputerVersionRoute)
	retention := h.registry.PruneRetention(ctx, h.AuthorizeComputerVersionRoute)
	stopped := h.registry.StopIdleVMs(ctx, h.AuthorizeComputerVersionRoute)
	quarantinesPruned := 0
	if state := h.coldRecovery(); state != nil {
		state.mu.Lock()
		storage, root := state.storage, state.stateRoot
		state.mu.Unlock()
		if storage != nil && root != "" {
			if n, err := storage.PruneRecoveryQuarantines(root, recoveryQuarantineRetained()); err == nil {
				quarantinesPruned = n
			}
		}
	}
	return reclaimResponse{
		Status:            "ok",
		VMsReclaimed:      reclaimed,
		RetentionPruned:   retention.Deleted,
		VMsStopped:        stopped,
		QuarantinesPruned: quarantinesPruned,
		ReclaimBefore:     before,
		ReclaimAfter:      h.registry.PressureReclaimPlan(),
		Retention:         retention,
	}
}

// HandleIdleCheck handles POST /internal/vmctl/idle-check.
// Triggers the bounded lifecycle sweep used by deploy and operators: pressure
// hibernation, stale disposable state reclaim, then ordinary idle hibernation
// (VAL-VM-008).
func (h *Handler) HandleIdleCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeVMCTLJSON(w, http.StatusMethodNotAllowed, vmctlErrorResponse{Error: "method not allowed"})
		return
	}

	if !isInternalCaller(r) {
		writeVMCTLJSON(w, http.StatusForbidden, vmctlErrorResponse{
			Error: "vmctl control endpoints are not publicly accessible",
		})
		return
	}

	result := h.runReclaimSweep(r.Context())
	writeVMCTLJSON(w, http.StatusOK, map[string]interface{}{
		"status":           result.Status,
		"vms_reclaimed":    result.VMsReclaimed,
		"retention_pruned": result.RetentionPruned,
		"vms_stopped":      result.VMsStopped,
		"reclaim":          result.ReclaimAfter,
		"reclaim_before":   result.ReclaimBefore,
		"retention":        result.Retention,
	})
}

// HandleReclaim handles POST /internal/vmctl/reclaim.
// It is an explicit operator/deploy alias for the same bounded reclaim sweep as
// idle-check, with a more precise name for disk-pressure preflight calls.
func (h *Handler) HandleReclaim(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeVMCTLJSON(w, http.StatusMethodNotAllowed, vmctlErrorResponse{Error: "method not allowed"})
		return
	}

	if !isInternalCaller(r) {
		writeVMCTLJSON(w, http.StatusForbidden, vmctlErrorResponse{
			Error: "vmctl control endpoints are not publicly accessible",
		})
		return
	}

	writeVMCTLJSON(w, http.StatusOK, h.runReclaimSweep(r.Context()))
}

// HandleRetentionPlan handles GET /internal/vmctl/retention-plan. It returns a
// dry-run inventory of VM state that matches the configured deletion policy.
func (h *Handler) HandleRetentionPlan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeVMCTLJSON(w, http.StatusMethodNotAllowed, vmctlErrorResponse{Error: "method not allowed"})
		return
	}

	if !isInternalCaller(r) {
		writeVMCTLJSON(w, http.StatusForbidden, vmctlErrorResponse{
			Error: "vmctl control endpoints are not publicly accessible",
		})
		return
	}

	writeVMCTLJSON(w, http.StatusOK, h.registry.RetentionPrunePlan())
}

// HandleRetentionShadowPlan handles GET /internal/vmctl/retention-shadow-plan.
// It returns an observation-only retention inventory. The shadow plan is never
// consumed by prune or reclaim deletion paths.
func (h *Handler) HandleRetentionShadowPlan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeVMCTLJSON(w, http.StatusMethodNotAllowed, vmctlErrorResponse{Error: "method not allowed"})
		return
	}

	if !isInternalCaller(r) {
		writeVMCTLJSON(w, http.StatusForbidden, vmctlErrorResponse{
			Error: "vmctl control endpoints are not publicly accessible",
		})
		return
	}

	writeVMCTLJSON(w, http.StatusOK, h.registry.RetentionShadowPlan())
}

// HandlePulse handles GET /internal/vmctl/pulse. It returns public-safe
// aggregate usage and health facts without raw users, emails, prompts,
// documents, traces, IPs, or per-user timelines.
func (h *Handler) HandlePulse(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeVMCTLJSON(w, http.StatusMethodNotAllowed, vmctlErrorResponse{Error: "method not allowed"})
		return
	}

	if !isInternalCaller(r) {
		writeVMCTLJSON(w, http.StatusForbidden, vmctlErrorResponse{
			Error: "vmctl control endpoints are not publicly accessible",
		})
		return
	}

	writeVMCTLJSON(w, http.StatusOK, h.registry.PulseSummary())
}

// HandleList handles GET /internal/vmctl/list.
// Lists all current ownerships (operator visibility).
func (h *Handler) HandleList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeVMCTLJSON(w, http.StatusMethodNotAllowed, vmctlErrorResponse{Error: "method not allowed"})
		return
	}

	if !isInternalCaller(r) {
		writeVMCTLJSON(w, http.StatusForbidden, vmctlErrorResponse{
			Error: "vmctl control endpoints are not publicly accessible",
		})
		return
	}

	result, err := h.listOwnershipResponses(r.Context())
	if err != nil {
		writeVMCTLJSON(w, http.StatusServiceUnavailable, vmctlErrorResponse{Error: err.Error()})
		return
	}

	writeVMCTLJSON(w, http.StatusOK, map[string]interface{}{
		"ownerships": result,
		"count":      len(result),
	})
}

// listOwnershipResponses classifies every ownership for operator listing and
// the deploy refresh selection (one classification, two readers).
func (h *Handler) listOwnershipResponses(ctx context.Context) ([]ownershipResponse, error) {
	if h.routeAuthorityRequired && h.routeAuthority == nil {
		return nil, fmt.Errorf("classify immutable ownerships: route/evidence authority is unavailable")
	}

	ownerships := h.registry.ListOwnerships()
	result := make([]ownershipResponse, 0, len(ownerships))
	rss := h.registry.latestVMResidentBytes()
	for _, own := range ownerships {
		var constructionVersion *computerversion.ComputerVersion
		var diskReceiptID string
		var constructed bool
		if h.routeAuthority != nil && own.State == VMStateActive && (own.Kind == "" || own.Kind == VMKindInteractive) {
			var err error
			constructionVersion, diskReceiptID, constructed, err = h.routeAuthority.constructedOwnershipIdentity(ctx, own.UserID, own.DesktopID, own.VMID)
			if err != nil {
				return nil, fmt.Errorf("classify immutable active ownership %s: %v", own.VMID, err)
			}
		}
		snapshotKind := ""
		if constructed {
			snapshotKind = "constructed-computer-version"
		}
		divergenceStatus := own.DivergenceStatus
		if divergenceStatus == "" {
			if constructed {
				divergenceStatus = "canary"
			} else {
				divergenceStatus = "tracking"
			}
		}
		result = append(result, ownershipResponse{
			VMID:                      own.VMID,
			ComputerID:                own.ComputerID,
			UserID:                    own.UserID,
			DesktopID:                 own.DesktopID,
			Kind:                      own.Kind,
			WarmnessClass:             string(h.registry.WarmnessClassForOwnership(own)),
			ComputerURL:               own.ComputerURL,
			State:                     string(own.State),
			Held:                      own.IsHeld(),
			CreatedAt:                 own.CreatedAt.Format("2006-01-02T15:04:05.000Z"),
			LastActiveAt:              own.LastActiveAt.Format("2006-01-02T15:04:05.000Z"),
			Epoch:                     own.Epoch,
			StoppedBy:                 own.StoppedBy,
			SnapshotKind:              snapshotKind,
			ConstructionVersion:       constructionVersion,
			ConstructionDiskReceiptID: diskReceiptID,
			DivergenceStatus:          divergenceStatus,
			PlatformBaseRef:           own.PlatformBaseRef,
			MemoryPeakRSSBytes:        own.MemoryPeakRSSBytes,
			MemoryMeasuredAt:          own.MemoryMeasuredAt,
			MemorySettledRSSBytes:     own.MemorySettledRSSBytes,
			ReportedRelease:           own.ReportedRelease,
			BootClaim:                 bootClaimResponseFor(own.BootClaim),
			StartInterruptions:        own.StartInterruptions,
			FailureWarmedAt:           own.FailureWarmedAt,
		})
		h.fillMemoryFields(&result[len(result)-1], own, rss)
	}
	return result, nil
}

// bootTimelineReader is the optional capability the real vmmanager adapter
// provides. Mocks that don't implement it get a 503, not a panic.
type bootTimelineReader interface {
	BootTimeline(vmID string) (json.RawMessage, error)
}

// hibernateTimelineReader is the optional S3 capability that returns the
// VM's last hibernate receipt (fence, pause, snapshot times and bytes, and
// the outcome: snapshot or kill with its reason).
type hibernateTimelineReader interface {
	HibernateTimeline(vmID string) (json.RawMessage, error)
}

// guestReleaseReader is the optional D2 capability of the real vmmanager
// adapter: the releases in use (what retention may not delete) and the
// release each live VM runs. A manager without it answers 404 and adds
// nothing to health.
type guestReleaseReader interface {
	GuestReleases() (json.RawMessage, error)
	GuestReleaseHealth() (current string, byVM map[string]GuestReleaseVMStatus)
}

// GuestReleaseVMStatus is the release a live VM runs, and whether it is
// behind the current release.
type GuestReleaseVMStatus struct {
	GuestRelease string
	Behind       bool
}

// GuestReleaseComputer is one computer's release in vmctl health.
type GuestReleaseComputer struct {
	ComputerID   string `json:"computer_id"`
	VMID         string `json:"vm_id"`
	GuestRelease string `json:"guest_release"`
	Behind       bool   `json:"behind"`
}

// HandleGuestReleases serves GET /internal/vmctl/guest-releases: the guest
// releases live VMs run and unconsumed snapshots need, for the release
// retention job (scripts/node-b-guest-release-retention). Internal callers
// only. 404 when the manager cannot tell (an older vmctl answers 404 too).
func (h *Handler) HandleGuestReleases(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeVMCTLJSON(w, http.StatusMethodNotAllowed, vmctlErrorResponse{Error: "method not allowed"})
		return
	}
	if !isInternalCaller(r) {
		writeVMCTLJSON(w, http.StatusForbidden, vmctlErrorResponse{Error: "vmctl control endpoints are not publicly accessible"})
		return
	}
	reader, ok := h.registry.VMManagerHandle().(guestReleaseReader)
	if !ok || reader == nil {
		writeVMCTLJSON(w, http.StatusNotFound, vmctlErrorResponse{Error: "guest releases unavailable on this vm manager"})
		return
	}
	raw, err := reader.GuestReleases()
	if err != nil {
		writeVMCTLJSON(w, http.StatusInternalServerError, vmctlErrorResponse{Error: err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(raw)
}

// guestReleaseHealth lists, for health, the current release and each active
// computer launched from a release.
func (h *Handler) guestReleaseHealth(ownerships []*VMOwnership) (string, []GuestReleaseComputer) {
	reader, ok := h.registry.VMManagerHandle().(guestReleaseReader)
	if !ok || reader == nil {
		return "", nil
	}
	current, byVM := reader.GuestReleaseHealth()
	var out []GuestReleaseComputer
	for _, own := range ownerships {
		if st, ok := byVM[own.VMID]; ok && st.GuestRelease != "" {
			out = append(out, GuestReleaseComputer{ComputerID: own.ComputerID, VMID: own.VMID, GuestRelease: st.GuestRelease, Behind: st.Behind})
		}
	}
	return current, out
}

// hibernateRequest is the JSON payload for POST /internal/vmctl/hibernate.
type hibernateRequest struct {
	ComputerID string `json:"computer_id,omitempty"`
	UserID     string `json:"user_id,omitempty"`
	DesktopID  string `json:"desktop_id,omitempty"`
}

// HandleHibernate serves POST /internal/vmctl/hibernate: an operator asks
// for one computer to hibernate now, the same transition the idle sweep
// makes (stop reason "operator"). With VM_HIBERNATE_MODE=snapshot the
// manager fences the guest and writes a memory snapshot; a busy guest, or
// any snapshot failure, falls back to the kill hibernate. The receipt is at
// GET /internal/vmctl/boot-timeline?computer_id=...&kind=hibernate.
func (h *Handler) HandleHibernate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeVMCTLJSON(w, http.StatusMethodNotAllowed, vmctlErrorResponse{Error: "method not allowed"})
		return
	}
	if !isInternalCaller(r) {
		writeVMCTLJSON(w, http.StatusForbidden, vmctlErrorResponse{Error: "vmctl control endpoints are not publicly accessible"})
		return
	}
	var req hibernateRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 64<<10)).Decode(&req); err != nil {
		writeVMCTLJSON(w, http.StatusBadRequest, vmctlErrorResponse{Error: "invalid request body"})
		return
	}
	userID, desktopID := strings.TrimSpace(req.UserID), normalizeDesktopID(req.DesktopID)
	if computerID := strings.TrimSpace(req.ComputerID); computerID != "" {
		own := h.registry.GetOwnershipByComputerID(computerID)
		if own == nil {
			writeVMCTLJSON(w, http.StatusNotFound, vmctlErrorResponse{Error: "no VM found for computer " + computerID})
			return
		}
		userID, desktopID = own.UserID, normalizeDesktopID(own.DesktopID)
	}
	if userID == "" {
		writeVMCTLJSON(w, http.StatusBadRequest, vmctlErrorResponse{Error: "computer_id or user_id is required"})
		return
	}
	if err := h.requireComputerVersionRoute(r.Context(), userID, desktopID); err != nil {
		writeVMCTLJSON(w, http.StatusConflict, vmctlErrorResponse{Error: err.Error()})
		return
	}
	if err := h.registry.hibernateVMForDesktopWithReason(userID, desktopID, "operator"); err != nil {
		writeVMCTLJSON(w, http.StatusConflict, vmctlErrorResponse{Error: err.Error()})
		return
	}
	resp := map[string]any{"status": "hibernated"}
	if own := h.registry.GetOwnershipForDesktop(userID, desktopID); own != nil {
		resp["computer_id"] = own.ComputerID
		resp["vm_id"] = own.VMID
		resp["epoch"] = own.Epoch
		if provider, ok := h.registry.VMManagerHandle().(hibernateTimelineReader); ok && provider != nil {
			if raw, err := provider.HibernateTimeline(own.VMID); err == nil {
				resp["hibernate_timeline"] = json.RawMessage(raw)
			}
		}
	}
	writeVMCTLJSON(w, http.StatusOK, resp)
}

// HandleBootTimeline serves GET /internal/vmctl/boot-timeline.
// Query: computer_id, or user_id (+ desktop_id). Returns the VM's merged
// host+guest boot receipt.
func (h *Handler) HandleBootTimeline(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeVMCTLJSON(w, http.StatusMethodNotAllowed, vmctlErrorResponse{Error: "method not allowed"})
		return
	}
	if !isInternalCaller(r) {
		writeVMCTLJSON(w, http.StatusForbidden, vmctlErrorResponse{Error: "vmctl control endpoints are not publicly accessible"})
		return
	}
	userID := strings.TrimSpace(r.URL.Query().Get("user_id"))
	computerID := strings.TrimSpace(r.URL.Query().Get("computer_id"))
	if userID == "" && computerID == "" {
		writeVMCTLJSON(w, http.StatusBadRequest, vmctlErrorResponse{Error: "user_id or computer_id query parameter is required"})
		return
	}
	var own *VMOwnership
	if computerID != "" {
		own = h.registry.GetOwnershipByComputerID(computerID)
	} else {
		own = h.registry.GetOwnershipForDesktop(userID, normalizeDesktopID(r.URL.Query().Get("desktop_id")))
	}
	if own == nil {
		writeVMCTLJSON(w, http.StatusNotFound, vmctlErrorResponse{Error: "no VM found for target"})
		return
	}
	var raw json.RawMessage
	var err error
	switch kind := strings.TrimSpace(r.URL.Query().Get("kind")); kind {
	case "", "boot":
		provider, ok := h.registry.VMManagerHandle().(bootTimelineReader)
		if !ok || provider == nil {
			writeVMCTLJSON(w, http.StatusServiceUnavailable, vmctlErrorResponse{Error: "boot timeline reader unavailable on this vm manager"})
			return
		}
		raw, err = provider.BootTimeline(own.VMID)
	case "hibernate":
		provider, ok := h.registry.VMManagerHandle().(hibernateTimelineReader)
		if !ok || provider == nil {
			writeVMCTLJSON(w, http.StatusServiceUnavailable, vmctlErrorResponse{Error: "hibernate timeline reader unavailable on this vm manager"})
			return
		}
		raw, err = provider.HibernateTimeline(own.VMID)
	default:
		writeVMCTLJSON(w, http.StatusBadRequest, vmctlErrorResponse{Error: "kind must be boot or hibernate"})
		return
	}
	if err != nil {
		writeVMCTLJSON(w, http.StatusNotFound, vmctlErrorResponse{Error: fmt.Sprintf("no boot timeline for VM %s: %v", own.VMID, err)})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(raw)
}

// isInternalCaller checks whether the request originated from an internal
// caller (localhost or internal service). vmctl control endpoints must only
// be reachable from internal host/service paths (VAL-VM-012).

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
func isInternalCaller(r *http.Request) bool {
	// Transport first (so guest requests never reach the credential check),
	// then the host-internal credential: loopback alone is not authority
	// (docs/problems/loopback-is-a-credential-2026-10-14.md).
	return internalTransport(r) && hostauth.Authorize(r)
}

func internalTransport(r *http.Request) bool {
	// S1a: internal authority is transport-bound, not caller-asserted.
	// A guest can set X-Internal-Caller and Host headers at will; only the
	// socket-layer source is unforgeable once tap anti-spoofing is in
	// place. Loopback TCP RemoteAddr or the autoputer-proxy unix socket
	// (empty/@/path RemoteAddr — host filesystem authority) are the only
	// internal signals; host services reach vmctl over either.
	if r.RemoteAddr == "" || r.RemoteAddr == "@" || strings.HasPrefix(r.RemoteAddr, "/") {
		return true
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		// TEST-NET-1 (192.0.2.x) is httptest.NewRequest's synthetic
		// RemoteAddr; it is admitted only inside a Go test binary, never in
		// production (hostauth.TestNetworkAllowed).
		return host == "127.0.0.1" || host == "::1" || hostauth.TestNetworkAllowed(host)
	}
	return r.RemoteAddr == "127.0.0.1" || r.RemoteAddr == "::1" || hostauth.TestNetworkAllowed(r.RemoteAddr)

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
	s.HandleFunc(effectiveReleaseReportPath, h.HandleEffectiveReleaseReport)
	s.HandleFunc("/internal/vmctl/maintenance-serve", attributedLifecycle(h.HandleMaintenanceServe))
	s.HandleFunc("/internal/vmctl/refresh", attributedLifecycle(h.HandleRefresh))
	s.HandleFunc("/internal/vmctl/deploy-refresh", attributedLifecycle(h.HandleDeployRefresh))
	s.HandleFunc("/internal/vmctl/compact-store", attributedLifecycle(h.HandleCompactStore))
	s.HandleFunc("/internal/vmctl/compact-sweep", attributedLifecycle(h.HandleCompactSweep))
	s.HandleFunc("/internal/vmctl/idle-check", h.HandleIdleCheck)
	s.HandleFunc("/internal/vmctl/reclaim", attributedLifecycle(h.HandleReclaim))
	s.HandleFunc("/internal/vmctl/touch", h.HandleTouch)
	s.HandleFunc("/internal/vmctl/hold", h.HandleHold)
	s.HandleFunc("/internal/vmctl/unhold", h.HandleUnhold)
	s.HandleFunc("/internal/vmctl/divergence", h.HandleSetDivergence)
	s.HandleFunc("/internal/vmctl/retention-plan", h.HandleRetentionPlan)
	s.HandleFunc("/internal/vmctl/retention-shadow-plan", h.HandleRetentionShadowPlan)
	s.HandleFunc("/internal/vmctl/pulse", h.HandlePulse)
	s.HandleFunc("/internal/vmctl/boot-timeline", h.HandleBootTimeline)
	s.HandleFunc("/internal/vmctl/guest-releases", h.HandleGuestReleases)
	s.HandleFunc(HostBootStatePath, h.HandleHostBootState)
	s.HandleFunc("/internal/vmctl/hibernate", attributedLifecycle(h.HandleHibernate))
	s.HandleFunc("/internal/vmctl/memory", h.HandleMemory)
	s.HandleFunc("/internal/vmctl/memory/guest", h.HandleGuestMemory)
	s.HandleFunc("/internal/vmctl/memory/balloon", h.HandleBalloonStats)
}

// attributedLifecycle logs who asked for a lifecycle call that can stop,
// replace or delete a computer, so a destroyed computer's destruction receipt
// (vmmanager/receipts.go) joins to its caller (docs/vmctl-360-review-2026-10-10.md,
// Phase 0 step 1). Resolve is not wrapped: it is too frequent to log, and a
// resolve-driven kill already names resolveDesktopContext in its receipt stack.
func attributedLifecycle(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		caller := strings.TrimSpace(r.Header.Get(LifecycleCallerHeader))
		if caller == "" {
			caller = "unattributed"
		}
		log.Printf("vmctl: lifecycle request %s %s caller=%q remote=%s", r.Method, r.URL.Path, caller, r.RemoteAddr)
		next(w, r)
	}
}

// ResolveEndpoint returns the full resolve endpoint URL for the vmctl
// service at the given base URL.
func ResolveEndpoint(baseURL string) string {
	return baseURL + "/internal/vmctl/resolve"
}

// TouchEndpoint returns the endpoint that refreshes a computer's idle clock.
func TouchEndpoint(baseURL string) string {
	return baseURL + "/internal/vmctl/touch"
}

// LookupEndpoint returns the full lookup endpoint URL for the vmctl
// service at the given base URL.
func LookupEndpoint(baseURL string) string {
	return baseURL + "/internal/vmctl/lookup"
}

// LookupGuestEndpoint returns the endpoint that resolves a tap-sourced guest
// RemoteAddr to its live ownership.
func LookupGuestEndpoint(baseURL string) string {
	return strings.TrimRight(baseURL, "/") + "/internal/vmctl/lookup-guest"
}

// ListEndpoint returns the full ownership-list endpoint URL for the vmctl
// service at the given base URL.
func ListEndpoint(baseURL string) string {
	return baseURL + "/internal/vmctl/list"
}

// StopEndpoint returns the full stop endpoint URL for the vmctl
// service at the given base URL.
func StopEndpoint(baseURL string) string {
	return fmt.Sprintf("%s/internal/vmctl/stop", baseURL)
}

// RemoveEndpoint returns the full remove endpoint URL for the vmctl
// service at the given base URL.
func RemoveEndpoint(baseURL string) string {
	return fmt.Sprintf("%s/internal/vmctl/remove", baseURL)
}

// ColdRecoverEndpoint returns the recover_current endpoint for one stable
// ComputerID. The ID occupies a path segment and is escaped as such.
func ColdRecoverEndpoint(baseURL, computerID string) string {
	return strings.TrimRight(baseURL, "/") + "/internal/vmctl/computers/" + url.PathEscape(strings.TrimSpace(computerID)) + "/cold-recover"
}

// UnholdEndpoint returns the full unhold endpoint URL for the vmctl service at
// the given base URL.
func UnholdEndpoint(baseURL string) string {
	return fmt.Sprintf("%s/internal/vmctl/unhold", baseURL)
}

// RefreshEndpoint returns the full refresh endpoint URL for the vmctl service
// at the given base URL.
func RefreshEndpoint(baseURL string) string {
	return fmt.Sprintf("%s/internal/vmctl/refresh", baseURL)
}

// IdleCheckEndpoint returns the full idle-check endpoint URL for the vmctl
// service at the given base URL.
func IdleCheckEndpoint(baseURL string) string {
	return fmt.Sprintf("%s/internal/vmctl/idle-check", baseURL)
}

// ReclaimEndpoint returns the full reclaim endpoint URL for the vmctl service
// at the given base URL.
func ReclaimEndpoint(baseURL string) string {
	return fmt.Sprintf("%s/internal/vmctl/reclaim", baseURL)
}

// RetentionPlanEndpoint returns the full retention-plan endpoint URL for the
// vmctl service at the given base URL.
func RetentionPlanEndpoint(baseURL string) string {
	return fmt.Sprintf("%s/internal/vmctl/retention-plan", baseURL)
}

// RetentionShadowPlanEndpoint returns the full retention-shadow-plan endpoint
// URL for the vmctl service at the given base URL.
func RetentionShadowPlanEndpoint(baseURL string) string {
	return fmt.Sprintf("%s/internal/vmctl/retention-shadow-plan", baseURL)
}

// PulseEndpoint returns the full Pulse aggregate endpoint URL for the vmctl
// service at the given base URL.
func PulseEndpoint(baseURL string) string {
	return fmt.Sprintf("%s/internal/vmctl/pulse", baseURL)
}
