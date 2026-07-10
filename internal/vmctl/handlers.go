package vmctl

import (
	"archive/tar"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/buildinfo"
	"github.com/yusefmosiah/go-choir/internal/server"
)

// errorResponse is a JSON error envelope.
type vmctlErrorResponse struct {
	Error string `json:"error"`
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
}

// resolveRequest is the JSON payload for POST /internal/vmctl/resolve.
type resolveRequest struct {
	UserID    string `json:"user_id"`
	DesktopID string `json:"desktop_id,omitempty"`
}

// resolveResponse is the JSON response for POST /internal/vmctl/resolve.
type resolveResponse struct {
	VMID            string `json:"vm_id"`
	UserID          string `json:"user_id"`
	DesktopID       string `json:"desktop_id"`
	Kind            VMKind `json:"kind,omitempty"`
	ParentDesktopID string `json:"parent_desktop_id,omitempty"`
	ParentVMID      string `json:"parent_vm_id,omitempty"`
	SnapshotKind    string `json:"snapshot_kind,omitempty"`
	Published       bool   `json:"published"`
	WarmnessClass   string `json:"warmness_class,omitempty"`
	SandboxURL      string `json:"sandbox_url"`
	State           string `json:"state"`
}

// ownershipResponse is the JSON response for ownership queries.
type ownershipResponse struct {
	VMID                 string             `json:"vm_id"`
	UserID               string             `json:"user_id"`
	DesktopID            string             `json:"desktop_id"`
	Kind                 VMKind             `json:"kind,omitempty"`
	ParentDesktopID      string             `json:"parent_desktop_id,omitempty"`
	ParentVMID           string             `json:"parent_vm_id,omitempty"`
	SnapshotKind         string             `json:"snapshot_kind,omitempty"`
	WorkerID             string             `json:"worker_id,omitempty"`
	ParentAgentID        string             `json:"parent_agent_id,omitempty"`
	TrajectoryID         string             `json:"trajectory_id,omitempty"`
	Purpose              string             `json:"purpose,omitempty"`
	ObjectiveFingerprint string             `json:"objective_fingerprint,omitempty"`
	MachineClass         string             `json:"machine_class,omitempty"`
	WarmnessClass        string             `json:"warmness_class,omitempty"`
	Published            bool               `json:"published"`
	SandboxURL           string             `json:"sandbox_url"`
	State                string             `json:"state"`
	CreatedAt            string             `json:"created_at"`
	LastActiveAt         string             `json:"last_active_at"`
	Epoch                int64              `json:"epoch"`
	StoppedBy            string             `json:"stopped_by,omitempty"`
	DataImage            *dataImageResponse `json:"data_image,omitempty"`
}

type forkDesktopRequest struct {
	UserID          string `json:"user_id"`
	SourceDesktopID string `json:"source_desktop_id,omitempty"`
	TargetDesktopID string `json:"target_desktop_id,omitempty"`
}

type requestWorkerRequest struct {
	UserID               string `json:"user_id"`
	DesktopID            string `json:"desktop_id,omitempty"`
	ParentAgentID        string `json:"parent_agent_id"`
	TrajectoryID         string `json:"trajectory_id,omitempty"`
	Purpose              string `json:"purpose"`
	ObjectiveFingerprint string `json:"objective_fingerprint,omitempty"`
	MachineClass         string `json:"machine_class,omitempty"`
	AllowParallel        bool   `json:"allow_parallel,omitempty"`
}

type workerActionRequest struct {
	WorkerID string `json:"worker_id"`
}

type reclaimResponse struct {
	Status            string               `json:"status"`
	VMsReclaimed      int                  `json:"vms_reclaimed"`
	StaleStateDeleted int                  `json:"stale_state_deleted"`
	RetentionPruned   int                  `json:"retention_pruned"`
	VMsStopped        int                  `json:"vms_stopped"`
	ReclaimBefore     PressureReclaimPlan  `json:"reclaim_before"`
	ReclaimAfter      PressureReclaimPlan  `json:"reclaim_after"`
	Retention         RetentionPruneResult `json:"retention"`
}

// Handler provides HTTP handlers for the vmctl service.
type Handler struct {
	registry                 *OwnershipRegistry
	sandboxRuntimePackageDir string
}

// NewHandler creates a vmctl Handler with the given ownership registry.
func NewHandler(registry *OwnershipRegistry) *Handler {
	return &Handler{registry: registry}
}

// SetSandboxRuntimePackageDir configures the host-side package directory that
// VM guests fetch at boot. This lets ordinary guest images stay stable while
// sandbox/runtime code moves through the fast host service pointer path.
func (h *Handler) SetSandboxRuntimePackageDir(path string) {
	h.sandboxRuntimePackageDir = strings.TrimSpace(path)
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
	idleEligible := h.registry.CheckIdleOwnerships()
	writeVMCTLJSON(w, http.StatusOK, vmctlHealthResponse{
		Status:          "ok",
		Service:         "vmctl",
		ActiveVMs:       h.registry.ActiveCount(),
		TotalOwnerships: len(ownerships),
		IdleEligible:    len(idleEligible),
		ByState:         byState,
		ByKind:          byKind,
		Reclaim:         h.registry.PressureReclaimPlan(),
		Warmness:        h.registry.WarmnessSummary(idleEligible),
	})
}

// HandleResolve handles POST /internal/vmctl/resolve.
// Given a user ID, it resolves or assigns a VM for that user.
// This is the primary endpoint the proxy calls to route authenticated
// requests through VM ownership (VAL-VM-001).
//
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
	if req.UserID == UniversalWirePlatformOwnerID && req.DesktopID == UniversalWirePlatformDesktopID {
		if err := h.registry.EnsureUniversalWirePlatformComputer(r.Context()); err != nil {
			log.Printf("vmctl: resolve platform computer failed: %v", err)
			writeVMCTLJSON(w, http.StatusInternalServerError, vmctlErrorResponse{Error: "failed to resolve platform computer"})
			return
		}
		own := h.registry.GetOwnershipForDesktop(req.UserID, req.DesktopID)
		if own == nil {
			log.Printf("vmctl: resolve platform computer lookup failed after ensure")
			writeVMCTLJSON(w, http.StatusInternalServerError, vmctlErrorResponse{Error: "failed to resolve platform computer"})
			return
		}
		writeVMCTLJSON(w, http.StatusOK, resolveResponse{
			VMID:            own.VMID,
			UserID:          own.UserID,
			DesktopID:       own.DesktopID,
			Kind:            own.Kind,
			ParentDesktopID: own.ParentDesktopID,
			ParentVMID:      own.ParentVMID,
			SnapshotKind:    own.SnapshotKind,
			Published:       own.Published,
			WarmnessClass:   string(h.registry.WarmnessClassForOwnership(own)),
			SandboxURL:      own.SandboxURL,
			State:           string(own.State),
		})
		return
	}
	if req.DesktopID != PrimaryDesktopID {
		writeVMCTLJSON(w, http.StatusBadRequest, vmctlErrorResponse{
			Error: "resolve can provision only the primary desktop; use lookup for published desktops or fork/publish for new desktops",
		})
		return
	}

	own, err := h.registry.ResolveOrAssignDesktopContext(r.Context(), req.UserID, req.DesktopID)
	if err != nil {
		log.Printf("vmctl: resolve failed for user %s desktop %s: %v", req.UserID, req.DesktopID, err)
		writeVMCTLJSON(w, http.StatusInternalServerError, vmctlErrorResponse{Error: "failed to resolve VM"})
		return
	}

	writeVMCTLJSON(w, http.StatusOK, resolveResponse{
		VMID:            own.VMID,
		UserID:          own.UserID,
		DesktopID:       own.DesktopID,
		Kind:            own.Kind,
		ParentDesktopID: own.ParentDesktopID,
		ParentVMID:      own.ParentVMID,
		SnapshotKind:    own.SnapshotKind,
		Published:       own.Published,
		WarmnessClass:   string(h.registry.WarmnessClassForOwnership(own)),
		SandboxURL:      own.SandboxURL,
		State:           string(own.State),
	})
}

// HandleForkDesktop handles POST /internal/vmctl/fork-desktop.
// It creates or resumes a distinct interactive VM for the target desktop, with
// lineage back to the source desktop.
func (h *Handler) HandleForkDesktop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeVMCTLJSON(w, http.StatusMethodNotAllowed, vmctlErrorResponse{Error: "method not allowed"})
		return
	}
	if !isInternalCaller(r) {
		writeVMCTLJSON(w, http.StatusForbidden, vmctlErrorResponse{Error: "vmctl control endpoints are not publicly accessible"})
		return
	}

	var req forkDesktopRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeVMCTLJSON(w, http.StatusBadRequest, vmctlErrorResponse{Error: "invalid request body"})
		return
	}
	if strings.TrimSpace(req.UserID) == "" {
		writeVMCTLJSON(w, http.StatusBadRequest, vmctlErrorResponse{Error: "user_id is required"})
		return
	}
	req.SourceDesktopID = normalizeDesktopID(req.SourceDesktopID)
	req.TargetDesktopID = normalizeDesktopID(req.TargetDesktopID)
	if req.TargetDesktopID == PrimaryDesktopID {
		writeVMCTLJSON(w, http.StatusBadRequest, vmctlErrorResponse{Error: "target_desktop_id must not be primary"})
		return
	}

	own, err := h.registry.ForkDesktop(req.UserID, req.SourceDesktopID, req.TargetDesktopID)
	if err != nil {
		writeVMCTLJSON(w, http.StatusBadRequest, vmctlErrorResponse{Error: err.Error()})
		return
	}

	writeVMCTLJSON(w, http.StatusOK, resolveResponse{
		VMID:            own.VMID,
		UserID:          own.UserID,
		DesktopID:       own.DesktopID,
		Kind:            own.Kind,
		ParentDesktopID: own.ParentDesktopID,
		ParentVMID:      own.ParentVMID,
		SnapshotKind:    own.SnapshotKind,
		Published:       own.Published,
		WarmnessClass:   string(h.registry.WarmnessClassForOwnership(own)),
		SandboxURL:      own.SandboxURL,
		State:           string(own.State),
	})
}

// HandlePublishDesktop handles POST /internal/vmctl/publish-desktop.
// It marks a background candidate desktop as user-switchable.
func (h *Handler) HandlePublishDesktop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeVMCTLJSON(w, http.StatusMethodNotAllowed, vmctlErrorResponse{Error: "method not allowed"})
		return
	}
	if !isInternalCaller(r) {
		writeVMCTLJSON(w, http.StatusForbidden, vmctlErrorResponse{Error: "vmctl control endpoints are not publicly accessible"})
		return
	}

	var req resolveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeVMCTLJSON(w, http.StatusBadRequest, vmctlErrorResponse{Error: "invalid request body"})
		return
	}
	if strings.TrimSpace(req.UserID) == "" {
		writeVMCTLJSON(w, http.StatusBadRequest, vmctlErrorResponse{Error: "user_id is required"})
		return
	}
	req.DesktopID = normalizeDesktopID(req.DesktopID)
	if req.DesktopID == PrimaryDesktopID {
		writeVMCTLJSON(w, http.StatusBadRequest, vmctlErrorResponse{Error: "primary desktop is already published"})
		return
	}

	own, err := h.registry.PublishDesktop(req.UserID, req.DesktopID)
	if err != nil {
		writeVMCTLJSON(w, http.StatusBadRequest, vmctlErrorResponse{Error: err.Error()})
		return
	}
	writeVMCTLJSON(w, http.StatusOK, resolveResponse{
		VMID:            own.VMID,
		UserID:          own.UserID,
		DesktopID:       own.DesktopID,
		Kind:            own.Kind,
		ParentDesktopID: own.ParentDesktopID,
		ParentVMID:      own.ParentVMID,
		SnapshotKind:    own.SnapshotKind,
		Published:       own.Published,
		WarmnessClass:   string(h.registry.WarmnessClassForOwnership(own)),
		SandboxURL:      own.SandboxURL,
		State:           string(own.State),
	})
}

// HandleRequestWorker handles POST /internal/vmctl/request-worker.
// It provisions a headless worker VM under an existing desktop and returns a
// typed worker handle instead of a browser-routable desktop URL.
func (h *Handler) HandleRequestWorker(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeVMCTLJSON(w, http.StatusMethodNotAllowed, vmctlErrorResponse{Error: "method not allowed"})
		return
	}
	if !isInternalCaller(r) {
		writeVMCTLJSON(w, http.StatusForbidden, vmctlErrorResponse{Error: "vmctl control endpoints are not publicly accessible"})
		return
	}

	var req requestWorkerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeVMCTLJSON(w, http.StatusBadRequest, vmctlErrorResponse{Error: "invalid request body"})
		return
	}
	own, err := h.registry.RequestWorker(WorkerRequest{
		UserID:               req.UserID,
		DesktopID:            req.DesktopID,
		ParentAgentID:        req.ParentAgentID,
		TrajectoryID:         req.TrajectoryID,
		Purpose:              req.Purpose,
		ObjectiveFingerprint: req.ObjectiveFingerprint,
		MachineClass:         req.MachineClass,
		AllowParallel:        req.AllowParallel,
	})
	if err != nil {
		writeVMCTLJSON(w, http.StatusBadRequest, vmctlErrorResponse{Error: err.Error()})
		return
	}

	writeVMCTLJSON(w, http.StatusOK, workerHandleFromOwnership(own))
}

// HandleHibernateWorker handles POST /internal/vmctl/hibernate-worker.
// It suspends a typed worker VM without touching the parent user desktop.
func (h *Handler) HandleHibernateWorker(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeVMCTLJSON(w, http.StatusMethodNotAllowed, vmctlErrorResponse{Error: "method not allowed"})
		return
	}
	if !isInternalCaller(r) {
		writeVMCTLJSON(w, http.StatusForbidden, vmctlErrorResponse{Error: "vmctl control endpoints are not publicly accessible"})
		return
	}

	var req workerActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeVMCTLJSON(w, http.StatusBadRequest, vmctlErrorResponse{Error: "invalid request body"})
		return
	}
	req.WorkerID = strings.TrimSpace(req.WorkerID)
	if req.WorkerID == "" {
		writeVMCTLJSON(w, http.StatusBadRequest, vmctlErrorResponse{Error: "worker_id is required"})
		return
	}
	if err := h.registry.HibernateWorker(req.WorkerID); err != nil {
		writeVMCTLJSON(w, http.StatusNotFound, vmctlErrorResponse{Error: err.Error()})
		return
	}
	for _, own := range h.registry.ListOwnerships() {
		if own.WorkerID == req.WorkerID {
			writeVMCTLJSON(w, http.StatusOK, map[string]any{
				"status":        "hibernated",
				"worker_id":     own.WorkerID,
				"vm_id":         own.VMID,
				"user_id":       own.UserID,
				"desktop_id":    own.DesktopID,
				"state":         string(own.State),
				"stopped_by":    own.StoppedBy,
				"sandbox_url":   own.SandboxURL,
				"machine_class": own.MachineClass,
			})
			return
		}
	}
	writeVMCTLJSON(w, http.StatusOK, map[string]string{"status": "hibernated", "worker_id": req.WorkerID})
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

	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		writeVMCTLJSON(w, http.StatusBadRequest, vmctlErrorResponse{Error: "user_id query parameter is required"})
		return
	}
	desktopID := normalizeDesktopID(r.URL.Query().Get("desktop_id"))

	own := h.registry.GetOwnershipForDesktop(userID, desktopID)
	if own == nil {
		writeVMCTLJSON(w, http.StatusNotFound, vmctlErrorResponse{Error: "no VM found for user"})
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
		VMID:                 own.VMID,
		UserID:               own.UserID,
		DesktopID:            own.DesktopID,
		Kind:                 own.Kind,
		ParentDesktopID:      own.ParentDesktopID,
		ParentVMID:           own.ParentVMID,
		SnapshotKind:         own.SnapshotKind,
		WorkerID:             own.WorkerID,
		ParentAgentID:        own.ParentAgentID,
		TrajectoryID:         own.TrajectoryID,
		Purpose:              own.Purpose,
		ObjectiveFingerprint: workerObjectiveFingerprintForOwnership(own),
		MachineClass:         own.MachineClass,
		WarmnessClass:        string(h.registry.WarmnessClassForOwnership(own)),
		Published:            own.Published,
		SandboxURL:           own.SandboxURL,
		State:                string(own.State),
		CreatedAt:            own.CreatedAt.Format("2006-01-02T15:04:05.000Z"),
		LastActiveAt:         own.LastActiveAt.Format("2006-01-02T15:04:05.000Z"),
		Epoch:                own.Epoch,
		StoppedBy:            own.StoppedBy,
		DataImage:            dataImage,
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

	if err := h.registry.StopVMForDesktop(req.UserID, req.DesktopID); err != nil {
		writeVMCTLJSON(w, http.StatusNotFound, vmctlErrorResponse{Error: err.Error()})
		return
	}

	writeVMCTLJSON(w, http.StatusOK, map[string]string{"status": "stopped"})
}

// HandleRemove handles POST /internal/vmctl/remove.
// Removes the ownership for a user (used during logout).
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

	_ = h.registry.RemoveOwnershipForDesktop(req.UserID, req.DesktopID)
	writeVMCTLJSON(w, http.StatusOK, map[string]string{"status": "removed"})
}

// HandleHibernate handles POST /internal/vmctl/hibernate.
// Hibernates the VM for a given user, preserving persistent state
// for later resume (VAL-VM-008, VAL-CROSS-116).
func (h *Handler) HandleHibernate(w http.ResponseWriter, r *http.Request) {
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

	if err := h.registry.HibernateVMForDesktop(req.UserID, req.DesktopID); err != nil {
		writeVMCTLJSON(w, http.StatusNotFound, vmctlErrorResponse{Error: err.Error()})
		return
	}

	own := h.registry.GetOwnershipForDesktop(req.UserID, req.DesktopID)
	writeVMCTLJSON(w, http.StatusOK, map[string]interface{}{
		"status":     "hibernated",
		"vm_id":      own.VMID,
		"desktop_id": own.DesktopID,
		"epoch":      own.Epoch,
	})
}

// HandleResume handles POST /internal/vmctl/resume.
// Resumes a stopped or hibernated VM for a user, restoring the
// same user's persisted state (VAL-CROSS-116).
// The epoch does NOT increment on resume (VAL-CROSS-117).
func (h *Handler) HandleResume(w http.ResponseWriter, r *http.Request) {
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

	own, err := h.registry.ResumeVMForDesktop(req.UserID, req.DesktopID)
	if err != nil {
		writeVMCTLJSON(w, http.StatusNotFound, vmctlErrorResponse{Error: err.Error()})
		return
	}

	writeVMCTLJSON(w, http.StatusOK, resolveResponse{
		VMID:            own.VMID,
		UserID:          own.UserID,
		DesktopID:       own.DesktopID,
		Kind:            own.Kind,
		ParentDesktopID: own.ParentDesktopID,
		ParentVMID:      own.ParentVMID,
		SnapshotKind:    own.SnapshotKind,
		Published:       own.Published,
		SandboxURL:      own.SandboxURL,
		State:           string(own.State),
	})
}

// HandleRecover handles POST /internal/vmctl/recover.
// Recovers an unhealthy or failed VM by creating a fresh boot with
// a new epoch (VAL-VM-009, VAL-CROSS-117).
func (h *Handler) HandleRecover(w http.ResponseWriter, r *http.Request) {
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

	own, err := h.registry.RecoverVMForDesktop(req.UserID, req.DesktopID)
	if err != nil {
		writeVMCTLJSON(w, http.StatusNotFound, vmctlErrorResponse{Error: err.Error()})
		return
	}

	writeVMCTLJSON(w, http.StatusOK, resolveResponse{
		VMID:            own.VMID,
		UserID:          own.UserID,
		DesktopID:       own.DesktopID,
		Kind:            own.Kind,
		ParentDesktopID: own.ParentDesktopID,
		ParentVMID:      own.ParentVMID,
		SnapshotKind:    own.SnapshotKind,
		Published:       own.Published,
		SandboxURL:      own.SandboxURL,
		State:           string(own.State),
	})
}

// HandleRefresh handles POST /internal/vmctl/refresh.
// It force-reboots an active computer onto the current guest image while
// preserving persistent state. This endpoint is internal deploy machinery.
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

	own, err := h.registry.RefreshVMForDesktop(req.UserID, req.DesktopID)
	if err != nil {
		writeVMCTLJSON(w, http.StatusNotFound, vmctlErrorResponse{Error: err.Error()})
		return
	}

	writeVMCTLJSON(w, http.StatusOK, resolveResponse{
		VMID:            own.VMID,
		UserID:          own.UserID,
		DesktopID:       own.DesktopID,
		Kind:            own.Kind,
		ParentDesktopID: own.ParentDesktopID,
		ParentVMID:      own.ParentVMID,
		SnapshotKind:    own.SnapshotKind,
		Published:       own.Published,
		SandboxURL:      own.SandboxURL,
		State:           string(own.State),
	})
}

// HandleLogout handles POST /internal/vmctl/logout.
// Transitions only the current user's VM to stopped state on logout
// (VAL-VM-008). Other users' VMs are not affected.
func (h *Handler) HandleLogout(w http.ResponseWriter, r *http.Request) {
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

	_ = h.registry.LogoutVMForDesktop(req.UserID, req.DesktopID)
	writeVMCTLJSON(w, http.StatusOK, map[string]string{"status": "stopped", "reason": "logout"})
}

func (h *Handler) runReclaimSweep() reclaimResponse {
	before := h.registry.PressureReclaimPlan()
	reclaimed := h.registry.ReclaimPressureVMs()
	staleStateDeleted := h.registry.ReclaimStaleVMState()
	retention := h.registry.PruneRetention()
	stopped := h.registry.StopIdleVMs()
	return reclaimResponse{
		Status:            "ok",
		VMsReclaimed:      reclaimed,
		StaleStateDeleted: staleStateDeleted,
		RetentionPruned:   retention.Deleted,
		VMsStopped:        stopped,
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

	result := h.runReclaimSweep()
	writeVMCTLJSON(w, http.StatusOK, map[string]interface{}{
		"status":              result.Status,
		"vms_reclaimed":       result.VMsReclaimed,
		"stale_state_deleted": result.StaleStateDeleted,
		"retention_pruned":    result.RetentionPruned,
		"vms_stopped":         result.VMsStopped,
		"reclaim":             result.ReclaimAfter,
		"reclaim_before":      result.ReclaimBefore,
		"retention":           result.Retention,
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

	writeVMCTLJSON(w, http.StatusOK, h.runReclaimSweep())
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

// HandlePrune handles POST /internal/vmctl/prune. It applies the bounded
// retention policy for orphan and explicitly ephemeral VM state.
func (h *Handler) HandlePrune(w http.ResponseWriter, r *http.Request) {
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

	writeVMCTLJSON(w, http.StatusOK, h.registry.PruneRetention())
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

	ownerships := h.registry.ListOwnerships()
	result := make([]ownershipResponse, 0, len(ownerships))
	for _, own := range ownerships {
		result = append(result, ownershipResponse{
			VMID:                 own.VMID,
			UserID:               own.UserID,
			DesktopID:            own.DesktopID,
			Kind:                 own.Kind,
			ParentDesktopID:      own.ParentDesktopID,
			ParentVMID:           own.ParentVMID,
			SnapshotKind:         own.SnapshotKind,
			WorkerID:             own.WorkerID,
			ParentAgentID:        own.ParentAgentID,
			TrajectoryID:         own.TrajectoryID,
			Purpose:              own.Purpose,
			ObjectiveFingerprint: workerObjectiveFingerprintForOwnership(own),
			MachineClass:         own.MachineClass,
			WarmnessClass:        string(h.registry.WarmnessClassForOwnership(own)),
			Published:            own.Published,
			SandboxURL:           own.SandboxURL,
			State:                string(own.State),
			CreatedAt:            own.CreatedAt.Format("2006-01-02T15:04:05.000Z"),
			LastActiveAt:         own.LastActiveAt.Format("2006-01-02T15:04:05.000Z"),
			Epoch:                own.Epoch,
			StoppedBy:            own.StoppedBy,
		})
	}

	writeVMCTLJSON(w, http.StatusOK, map[string]interface{}{
		"ownerships": result,
		"count":      len(result),
	})
}

// HandleRuntimePackage streams the current sandbox runtime package as a tar
// archive. It is intended for guest VMs booting over the vmctl tap path; it
// never exposes provider credentials and remains guarded by the same internal
// caller contract as other vmctl control endpoints.
func (h *Handler) HandleRuntimePackage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeVMCTLJSON(w, http.StatusMethodNotAllowed, vmctlErrorResponse{Error: "method not allowed"})
		return
	}
	if !isInternalCaller(r) {
		writeVMCTLJSON(w, http.StatusForbidden, vmctlErrorResponse{Error: "vmctl control endpoints are not publicly accessible"})
		return
	}
	if strings.Trim(r.URL.Path, "/") != "internal/vmctl/runtime-package/sandbox" {
		writeVMCTLJSON(w, http.StatusNotFound, vmctlErrorResponse{Error: "runtime package not found"})
		return
	}

	root := h.sandboxRuntimePackageDir
	if root == "" {
		writeVMCTLJSON(w, http.StatusServiceUnavailable, vmctlErrorResponse{Error: "sandbox runtime package directory is not configured"})
		return
	}
	info, err := os.Stat(root)
	if err != nil {
		writeVMCTLJSON(w, http.StatusServiceUnavailable, vmctlErrorResponse{Error: "sandbox runtime package directory is not available"})
		return
	}
	if !info.IsDir() {
		writeVMCTLJSON(w, http.StatusServiceUnavailable, vmctlErrorResponse{Error: "sandbox runtime package path is not a directory"})
		return
	}
	artifactBuild, err := sandboxRuntimeBuildInfo(root)
	if err != nil {
		log.Printf("vmctl: sandbox runtime build manifest: %v", err)
		writeVMCTLJSON(w, http.StatusServiceUnavailable, vmctlErrorResponse{Error: "sandbox runtime package build manifest is invalid"})
		return
	}

	w.Header().Set("Content-Type", "application/x-tar")
	w.Header().Set("Content-Disposition", `attachment; filename="go-choir-sandbox-runtime.tar"`)
	tw := tar.NewWriter(w)
	defer func() {
		if err := tw.Close(); err != nil {
			log.Printf("vmctl: close runtime package tar: %v", err)
		}
	}()

	if err := writeRuntimePackageTar(tw, root, artifactBuild, runtimePackageServiceEnv(r)); err != nil {
		log.Printf("vmctl: stream runtime package from %s: %v", root, err)
		return
	}
}

type runtimePackageBuildManifest struct {
	SchemaVersion int    `json:"schema_version"`
	Artifact      string `json:"artifact"`
	Version       string `json:"version"`
	Commit        string `json:"commit"`
	BuiltAt       string `json:"built_at"`
}

func sandboxRuntimeBuildInfo(root string) (buildinfo.Info, error) {
	path := filepath.Join(root, "share", "go-choir", "build.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return buildinfo.Info{}, fmt.Errorf("read %s: %w", path, err)
	}
	var manifest runtimePackageBuildManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return buildinfo.Info{}, fmt.Errorf("decode %s: %w", path, err)
	}
	manifest.Artifact = strings.TrimSpace(manifest.Artifact)
	manifest.Version = strings.TrimSpace(manifest.Version)
	manifest.Commit = strings.TrimSpace(manifest.Commit)
	manifest.BuiltAt = strings.TrimSpace(manifest.BuiltAt)
	if manifest.SchemaVersion != 1 || manifest.Artifact != "sandbox" || manifest.Commit == "" {
		return buildinfo.Info{}, fmt.Errorf("manifest must identify a schema-v1 sandbox artifact with a commit")
	}
	return buildinfo.Info{
		Service: "sandbox",
		Version: manifest.Version,
		Commit:  manifest.Commit,
		BuiltAt: manifest.BuiltAt,
	}, nil
}

// HandleSandboxProxy resolves the live sandbox URL and reverse-proxies the request.
// Path format: /internal/vmctl/sandbox-proxy/{owner-id}/{...remaining-path}
func (h *Handler) HandleSandboxProxy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		writeVMCTLJSON(w, http.StatusMethodNotAllowed, vmctlErrorResponse{Error: "method not allowed"})
		return
	}
	if !isInternalCaller(r) {
		writeVMCTLJSON(w, http.StatusForbidden, vmctlErrorResponse{Error: "sandbox proxy is not publicly accessible"})
		return
	}

	// Extract owner from path: /internal/vmctl/sandbox-proxy/{owner}/{...rest}
	const prefix = "/internal/vmctl/sandbox-proxy/"
	path := strings.TrimPrefix(r.URL.Path, prefix)
	if path == "" || path == r.URL.Path {
		writeVMCTLJSON(w, http.StatusBadRequest, vmctlErrorResponse{Error: "missing owner in proxy path"})
		return
	}

	// Split into owner and remaining path.
	slashIdx := strings.Index(path, "/")
	var ownerID, remainingPath string
	if slashIdx < 0 {
		ownerID = path
		remainingPath = "/"
	} else {
		ownerID = path[:slashIdx]
		remainingPath = path[slashIdx:]
	}

	ownerID = strings.TrimSpace(ownerID)
	if ownerID == "" {
		writeVMCTLJSON(w, http.StatusBadRequest, vmctlErrorResponse{Error: "empty owner in proxy path"})
		return
	}

	desktopID := UniversalWirePlatformDesktopID
	if ownerID == UniversalWirePlatformOwnerID {
		if err := h.registry.EnsureUniversalWirePlatformComputer(r.Context()); err != nil {
			log.Printf("vmctl: ensure platform sandbox for %s: %v", ownerID, err)
			writeVMCTLJSON(w, http.StatusServiceUnavailable, vmctlErrorResponse{Error: "platform sandbox is not ready"})
			return
		}
	}

	// Resolve live sandbox URL.
	sandboxURL, err := h.registry.LiveSandboxURL(ownerID, desktopID)
	if err != nil {
		writeVMCTLJSON(w, http.StatusServiceUnavailable, vmctlErrorResponse{Error: fmt.Sprintf("resolve sandbox for %s: %v", ownerID, err)})
		return
	}

	target, err := url.Parse(sandboxURL)
	if err != nil {
		writeVMCTLJSON(w, http.StatusInternalServerError, vmctlErrorResponse{Error: fmt.Sprintf("invalid sandbox URL: %v", err)})
		return
	}

	// Rewrite the request path to the remaining path.
	r.URL.Path = remainingPath
	r.URL.RawPath = ""

	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.ServeHTTP(w, r)
}

func runtimePackageServiceEnv(r *http.Request) map[string]string {
	out := make(map[string]string)
	host := strings.TrimSpace(r.Host)
	if host == "" {
		return out
	}
	wireHost := host
	if h, _, err := net.SplitHostPort(host); err == nil && h != "" {
		wireHost = net.JoinHostPort(h, "8082")
	} else if !strings.Contains(host, ":") {
		wireHost = net.JoinHostPort(host, "8082")
	}
	out["RUNTIME_WIRE_PUBLISH_URL"] = "http://" + wireHost
	out["RUNTIME_CORPUSD_URL"] = "http://" + wireHost
	return out
}

func writeRuntimePackageTar(tw *tar.Writer, root string, snapshot buildinfo.Info, serviceEnv map[string]string) error {
	root = filepath.Clean(root)
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		rel = filepath.ToSlash(filepath.Clean(rel))
		if rel == "." || strings.HasPrefix(rel, "../") || strings.HasPrefix(rel, "/") {
			return fmt.Errorf("refuse unsafe runtime package path %q", rel)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		var link string
		if info.Mode()&os.ModeSymlink != 0 {
			link, err = os.Readlink(path)
			if err != nil {
				return err
			}
		}
		hdr, err := tar.FileInfoHeader(info, link)
		if err != nil {
			return err
		}
		hdr.Name = rel
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(tw, f)
			closeErr := f.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
		}
		return nil
	}); err != nil {
		return err
	}

	deployedAt := strings.TrimSpace(snapshot.DeployedAt)
	var env string
	if deployedAt != "" {
		env = fmt.Sprintf("CHOIR_DEPLOYED_AT=%s\n", shellEnvValue(deployedAt))
	}
	for _, key := range []string{"RUNTIME_WIRE_PUBLISH_URL", "RUNTIME_CORPUSD_URL"} {
		if value := strings.TrimSpace(serviceEnv[key]); value != "" {
			env += fmt.Sprintf("%s=%s\n", key, shellEnvValue(value))
		}
	}
	hdr := &tar.Header{
		Name: "choir-runtime.env",
		Mode: 0o644,
		Size: int64(len(env)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	_, err := tw.Write([]byte(env))
	return err
}

func shellEnvValue(value string) string {
	return strings.NewReplacer("\n", "", "\r", "", "\x00", "").Replace(value)
}

// isInternalCaller checks whether the request originated from an internal
// caller (localhost or internal service). vmctl control endpoints must only
// be reachable from internal host/service paths (VAL-VM-012).
func isInternalCaller(r *http.Request) bool {
	internal := map[string]bool{
		"localhost": true,
		"127.0.0.1": true,
		"::1":       true,
	}

	// Check if the request has the internal service header.
	// This allows service-to-service calls where the request
	// comes through a loopback connection.
	if r.Header.Get("X-Internal-Caller") == "true" {
		return true
	}

	// Extract host from Host header, handling both host:port and [ipv6]:port.
	if host, _, err := net.SplitHostPort(r.Host); err == nil {
		if internal[host] {
			return true
		}
	} else {
		// No port in Host, check directly.
		if internal[r.Host] {
			return true
		}
	}

	// Check RemoteAddr for loopback connections.
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		if internal[host] {
			return true
		}
	} else {
		if internal[r.RemoteAddr] {
			return true
		}
	}

	return false
}

// RegisterRoutes registers all vmctl routes on the given server.
// All control endpoints use the /internal/vmctl/ prefix to make it
// clear these are not public-facing routes (VAL-VM-012).
func RegisterRoutes(s *server.Server, h *Handler) {
	s.SetHealthHandler(h.HandleHealth)
	s.HandleFunc("/internal/vmctl/resolve", h.HandleResolve)
	s.HandleFunc("/internal/vmctl/fork-desktop", h.HandleForkDesktop)
	s.HandleFunc("/internal/vmctl/publish-desktop", h.HandlePublishDesktop)
	s.HandleFunc("/internal/vmctl/request-worker", h.HandleRequestWorker)
	s.HandleFunc("/internal/vmctl/hibernate-worker", h.HandleHibernateWorker)
	s.HandleFunc("/internal/vmctl/lookup", h.HandleLookup)
	s.HandleFunc("/internal/vmctl/stop", h.HandleStop)
	s.HandleFunc("/internal/vmctl/remove", h.HandleRemove)
	s.HandleFunc("/internal/vmctl/list", h.HandleList)
	s.HandleFunc("/internal/vmctl/hibernate", h.HandleHibernate)
	s.HandleFunc("/internal/vmctl/resume", h.HandleResume)
	s.HandleFunc("/internal/vmctl/recover", h.HandleRecover)
	s.HandleFunc("/internal/vmctl/refresh", h.HandleRefresh)
	s.HandleFunc("/internal/vmctl/logout", h.HandleLogout)
	s.HandleFunc("/internal/vmctl/idle-check", h.HandleIdleCheck)
	s.HandleFunc("/internal/vmctl/reclaim", h.HandleReclaim)
	s.HandleFunc("/internal/vmctl/retention-plan", h.HandleRetentionPlan)
	s.HandleFunc("/internal/vmctl/retention-shadow-plan", h.HandleRetentionShadowPlan)
	s.HandleFunc("/internal/vmctl/pulse", h.HandlePulse)
	s.HandleFunc("/internal/vmctl/prune", h.HandlePrune)
	s.HandleFunc("/internal/vmctl/runtime-package/sandbox", h.HandleRuntimePackage)
	s.HandleFunc("/internal/vmctl/sandbox-proxy/", h.HandleSandboxProxy)
}

// ResolveEndpoint returns the full resolve endpoint URL for the vmctl
// service at the given base URL.
func ResolveEndpoint(baseURL string) string {
	return baseURL + "/internal/vmctl/resolve"
}

// LookupEndpoint returns the full lookup endpoint URL for the vmctl
// service at the given base URL.
func LookupEndpoint(baseURL string) string {
	return baseURL + "/internal/vmctl/lookup"
}

// ListEndpoint returns the full ownership-list endpoint URL for the vmctl
// service at the given base URL.
func ListEndpoint(baseURL string) string {
	return baseURL + "/internal/vmctl/list"
}

// ForkDesktopEndpoint returns the full fork-desktop endpoint URL for the vmctl
// service at the given base URL.
func ForkDesktopEndpoint(baseURL string) string {
	return baseURL + "/internal/vmctl/fork-desktop"
}

// PublishDesktopEndpoint returns the full publish-desktop endpoint URL for the
// vmctl service at the given base URL.
func PublishDesktopEndpoint(baseURL string) string {
	return baseURL + "/internal/vmctl/publish-desktop"
}

// RequestWorkerEndpoint returns the full request-worker endpoint URL for the
// vmctl service at the given base URL.
func RequestWorkerEndpoint(baseURL string) string {
	return baseURL + "/internal/vmctl/request-worker"
}

// HibernateWorkerEndpoint returns the full hibernate-worker endpoint URL for
// the vmctl service at the given base URL.
func HibernateWorkerEndpoint(baseURL string) string {
	return baseURL + "/internal/vmctl/hibernate-worker"
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

// HibernateEndpoint returns the full hibernate endpoint URL for the vmctl
// service at the given base URL.
func HibernateEndpoint(baseURL string) string {
	return fmt.Sprintf("%s/internal/vmctl/hibernate", baseURL)
}

// ResumeEndpoint returns the full resume endpoint URL for the vmctl
// service at the given base URL.
func ResumeEndpoint(baseURL string) string {
	return fmt.Sprintf("%s/internal/vmctl/resume", baseURL)
}

// RecoverEndpoint returns the full recover endpoint URL for the vmctl
// service at the given base URL.
func RecoverEndpoint(baseURL string) string {
	return fmt.Sprintf("%s/internal/vmctl/recover", baseURL)
}

// RefreshEndpoint returns the full refresh endpoint URL for the vmctl service
// at the given base URL.
func RefreshEndpoint(baseURL string) string {
	return fmt.Sprintf("%s/internal/vmctl/refresh", baseURL)
}

// LogoutEndpoint returns the full logout endpoint URL for the vmctl
// service at the given base URL.
func LogoutEndpoint(baseURL string) string {
	return fmt.Sprintf("%s/internal/vmctl/logout", baseURL)
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

// PruneEndpoint returns the full prune endpoint URL for the vmctl service at
// the given base URL.
func PruneEndpoint(baseURL string) string {
	return fmt.Sprintf("%s/internal/vmctl/prune", baseURL)
}
