package vmctl

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
)

// Memory routes (docs/reports/memory-hotplug-controller-design-2026-10-14.md
// section 4.5):
//
//	POST /internal/vmctl/memory        host only: {vm_id|computer_id, target_mib, ttl_seconds?, wait_seconds?}
//	GET  /internal/vmctl/memory[?vm_id=|computer_id=]  host only
//	POST /internal/vmctl/memory/guest  guest only: {target_mib, ttl_seconds?, wait_seconds?}
//
// target_mib is the total guest memory; at or below the base it releases.
// A guest's computer is the one its tap source address binds to; it can
// never name another.

type memoryRequestBody struct {
	VMID        string `json:"vm_id,omitempty"`
	ComputerID  string `json:"computer_id,omitempty"`
	TargetMiB   int    `json:"target_mib"`
	TTLSeconds  int    `json:"ttl_seconds,omitempty"`
	WaitSeconds int    `json:"wait_seconds,omitempty"`
}

func decodeMemoryRequest(w http.ResponseWriter, r *http.Request) (memoryRequestBody, bool) {
	var body memoryRequestBody
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		writeVMCTLJSON(w, http.StatusBadRequest, vmctlErrorResponse{Error: "invalid memory request"})
		return body, false
	}
	if body.TTLSeconds < 0 || body.WaitSeconds < 0 {
		writeVMCTLJSON(w, http.StatusBadRequest, vmctlErrorResponse{Error: "ttl_seconds and wait_seconds must not be negative"})
		return body, false
	}
	return body, true
}

func (h *Handler) ownershipForMemory(vmID, computerID string) *VMOwnership {
	vmID, computerID = strings.TrimSpace(vmID), strings.TrimSpace(computerID)
	if computerID != "" {
		own := h.registry.GetOwnershipByComputerID(computerID)
		if own == nil || (vmID != "" && own.VMID != vmID) {
			return nil
		}
		return own
	}
	if vmID == "" {
		return nil
	}
	for _, own := range h.registry.ListOwnerships() {
		if own != nil && own.VMID == vmID {
			snapshot := *own
			return &snapshot
		}
	}
	return nil
}

func (h *Handler) serveMemoryRequest(w http.ResponseWriter, r *http.Request, own *VMOwnership, body memoryRequestBody, holder string) {
	view, err := h.registry.RequestMemory(own, body.TargetMiB, time.Duration(body.TTLSeconds)*time.Second, holder)
	if err != nil {
		var reqErr *MemoryRequestError
		if errors.As(err, &reqErr) {
			writeVMCTLJSON(w, reqErr.Status, vmctlErrorResponse{Error: reqErr.Reason})
			return
		}
		writeVMCTLJSON(w, http.StatusInternalServerError, vmctlErrorResponse{Error: err.Error()})
		return
	}
	if body.WaitSeconds > 0 {
		view = h.registry.WaitMemory(r.Context(), own, time.Duration(body.WaitSeconds)*time.Second)
	}
	writeVMCTLJSON(w, http.StatusOK, view)
}

// HandleMemory serves the host memory routes.
func (h *Handler) HandleMemory(w http.ResponseWriter, r *http.Request) {
	if !isInternalCaller(r) {
		writeVMCTLJSON(w, http.StatusForbidden, vmctlErrorResponse{Error: "vmctl control endpoints are not publicly accessible"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		vmID, computerID := r.URL.Query().Get("vm_id"), r.URL.Query().Get("computer_id")
		if vmID == "" && computerID == "" {
			writeVMCTLJSON(w, http.StatusOK, h.registry.MemorySummary())
			return
		}
		own := h.ownershipForMemory(vmID, computerID)
		if own == nil {
			writeVMCTLJSON(w, http.StatusNotFound, vmctlErrorResponse{Error: "computer not found"})
			return
		}
		writeVMCTLJSON(w, http.StatusOK, h.registry.observeMemory(own))
	case http.MethodPost:
		body, ok := decodeMemoryRequest(w, r)
		if !ok {
			return
		}
		own := h.ownershipForMemory(body.VMID, body.ComputerID)
		if own == nil {
			writeVMCTLJSON(w, http.StatusNotFound, vmctlErrorResponse{Error: "computer not found"})
			return
		}
		h.serveMemoryRequest(w, r, own, body, MemoryLeaseHolderHost)
	default:
		writeVMCTLJSON(w, http.StatusMethodNotAllowed, vmctlErrorResponse{Error: "method not allowed"})
	}
}

// HandleGuestMemory serves a guest's lease request for its own computer.
func (h *Handler) HandleGuestMemory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeVMCTLJSON(w, http.StatusMethodNotAllowed, vmctlErrorResponse{Error: "method not allowed"})
		return
	}
	srcHost := guestRemoteAddrHost(r)
	if srcHost == "" || internalTransport(r) {
		writeVMCTLJSON(w, http.StatusForbidden, vmctlErrorResponse{Error: "guest memory route requires a tap-sourced request"})
		return
	}
	own := h.registry.GuestOwnershipForAddress(srcHost)
	if own == nil {
		writeVMCTLJSON(w, http.StatusForbidden, vmctlErrorResponse{Error: "caller " + srcHost + " is not bound to a live computer"})
		return
	}
	body, ok := decodeMemoryRequest(w, r)
	if !ok {
		return
	}
	if (body.VMID != "" && strings.TrimSpace(body.VMID) != own.VMID) || (body.ComputerID != "" && strings.TrimSpace(body.ComputerID) != own.ComputerID) {
		writeVMCTLJSON(w, http.StatusForbidden, vmctlErrorResponse{Error: "a guest may only change its own memory"})
		return
	}
	h.serveMemoryRequest(w, r, own, body, MemoryLeaseHolderGuest)
}

// MemoryEndpoint returns the host memory endpoint for a vmctl base URL.
func MemoryEndpoint(baseURL string) string {
	return strings.TrimRight(baseURL, "/") + "/internal/vmctl/memory"
}

// GuestMemoryEndpoint returns the guest memory lease endpoint.
func GuestMemoryEndpoint(baseURL string) string {
	return strings.TrimRight(baseURL, "/") + "/internal/vmctl/memory/guest"
}

// fillMemoryFields sets the list's current-memory fields from the RSS
// sample and the controller cache.
func (h *Handler) fillMemoryFields(resp *ownershipResponse, own *VMOwnership, rss map[string]uint64) {
	if own.State != VMStateActive {
		return
	}
	resp.MemoryRSSBytes = rss[own.VMID]
	_, base := machineShapeForOwnership(own)
	resp.MemoryBaseMiB, resp.MemoryCurrentMiB = base, base
	resp.MemoryHotplugSelected = h.registry.memoryHotplugMiBFor(own) > 0
	if view, ok := h.registry.MemoryView(own.VMID); ok && view.BaseMiB > 0 {
		resp.MemoryBaseMiB = view.BaseMiB
		resp.MemoryCurrentMiB = view.CurrentMiB
		resp.MemoryRequestedMiB = view.RequestedMiB
		resp.MemoryPluggedMiB = view.PluggedMiB
		resp.MemoryHotplugPhase = view.Phase
		if view.Lease != nil {
			resp.MemoryLeaseExpiresAt = view.Lease.ExpiresAt
		}
	}
}
