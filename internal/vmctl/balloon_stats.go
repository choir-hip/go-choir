package vmctl

import (
	"errors"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

// Balloon statistics read (RES-10 step 2;
// docs/problems/balloon-stats-unread-no-emergency-inflation-2026-10-21.md):
//
//	GET /internal/vmctl/memory/balloon[?vm_id=|computer_id=]  host only
//
// Each computer's guest-reported memory (Firecracker GET /balloon/statistics,
// read through the manager) or the reason there is none. Read only: no
// route here or anywhere changes a balloon, and no guest route exists. No
// vmctl loop calls it; each reading is bounded in the manager, and the
// all-computer read fans out so a stuck VMM costs one bound, not N.

// BalloonStatistics mirrors vmmanager.BalloonStatistics (Firecracker's names
// and units; absent optional fields stay absent).
type BalloonStatistics struct {
	TargetPages        uint64  `json:"target_pages"`
	ActualPages        uint64  `json:"actual_pages"`
	TargetMiB          uint64  `json:"target_mib"`
	ActualMiB          uint64  `json:"actual_mib"`
	SwapIn             *uint64 `json:"swap_in,omitempty"`
	SwapOut            *uint64 `json:"swap_out,omitempty"`
	MajorFaults        *uint64 `json:"major_faults,omitempty"`
	MinorFaults        *uint64 `json:"minor_faults,omitempty"`
	FreeMemory         *uint64 `json:"free_memory,omitempty"`
	TotalMemory        *uint64 `json:"total_memory,omitempty"`
	AvailableMemory    *uint64 `json:"available_memory,omitempty"`
	DiskCaches         *uint64 `json:"disk_caches,omitempty"`
	HugetlbAllocations *uint64 `json:"hugetlb_allocations,omitempty"`
	HugetlbFailures    *uint64 `json:"hugetlb_failures,omitempty"`
}

// GuestMemoryStats is one reading of a running VM's balloon.
type GuestMemoryStats struct {
	VMID                  string            `json:"vm_id"`
	StatsPollingIntervalS int               `json:"stats_polling_interval_s"`
	Stats                 BalloonStatistics `json:"stats"`
	ObservedAt            time.Time         `json:"observed_at"`
}

// GuestMemoryStatsError is an unavailable reading: Reason is one of
// vmmanager's GuestMemoryStatsReason* values.
type GuestMemoryStatsError struct {
	VMID   string
	Reason string
	Detail string
}

func (e *GuestMemoryStatsError) Error() string {
	return "balloon stats for VM " + e.VMID + " unavailable: " + e.Reason
}

// guestMemoryStatsReader is implemented by the Firecracker manager adapter.
type guestMemoryStatsReader interface {
	GuestMemoryStats(vmID string) (GuestMemoryStats, error)
}

// GuestMemoryStatsView is one computer's entry: a reading, or a reason with
// no numbers.
type GuestMemoryStatsView struct {
	VMID       string            `json:"vm_id"`
	ComputerID string            `json:"computer_id,omitempty"`
	Reason     string            `json:"reason,omitempty"`
	Detail     string            `json:"detail,omitempty"`
	Reading    *GuestMemoryStats `json:"reading,omitempty"`
}

const balloonStatsUnits = "target_pages/actual_pages: 4 KiB pages; target_mib/actual_mib: MiB; swap_in, swap_out, free_memory, total_memory, available_memory, disk_caches: bytes; faults: counts (Firecracker GET /balloon/statistics)"

// GuestMemoryStatsFor reads each ownership's balloon in parallel.
func (r *OwnershipRegistry) GuestMemoryStatsFor(owns []*VMOwnership) []GuestMemoryStatsView {
	r.mu.RLock()
	mgr := r.vmManager
	r.mu.RUnlock()
	reader, ok := mgr.(guestMemoryStatsReader)
	views := make([]GuestMemoryStatsView, len(owns))
	var wg sync.WaitGroup
	for i, own := range owns {
		views[i] = GuestMemoryStatsView{VMID: own.VMID, ComputerID: own.ComputerID}
		if !ok || reader == nil {
			views[i].Reason = "unsupported"
			continue
		}
		wg.Add(1)
		go func(v *GuestMemoryStatsView) {
			defer wg.Done()
			st, err := reader.GuestMemoryStats(v.VMID)
			if err != nil {
				var se *GuestMemoryStatsError
				if errors.As(err, &se) {
					v.Reason, v.Detail = se.Reason, se.Detail
				} else {
					v.Reason, v.Detail = "api_error", err.Error()
				}
				return
			}
			v.Reading = &st
		}(&views[i])
	}
	wg.Wait()
	return views
}

// HandleBalloonStats serves GET /internal/vmctl/memory/balloon.
func (h *Handler) HandleBalloonStats(w http.ResponseWriter, r *http.Request) {
	if !isInternalCaller(r) {
		writeVMCTLJSON(w, http.StatusForbidden, vmctlErrorResponse{Error: "vmctl control endpoints are not publicly accessible"})
		return
	}
	if r.Method != http.MethodGet {
		writeVMCTLJSON(w, http.StatusMethodNotAllowed, vmctlErrorResponse{Error: "method not allowed"})
		return
	}
	var owns []*VMOwnership
	vmID, computerID := r.URL.Query().Get("vm_id"), r.URL.Query().Get("computer_id")
	if vmID != "" || computerID != "" {
		own := h.ownershipForMemory(vmID, computerID)
		if own == nil {
			writeVMCTLJSON(w, http.StatusNotFound, vmctlErrorResponse{Error: "computer not found"})
			return
		}
		owns = []*VMOwnership{own}
	} else {
		seen := map[string]bool{}
		for _, own := range h.registry.ListOwnerships() {
			if own == nil || strings.TrimSpace(own.VMID) == "" || seen[own.VMID] {
				continue
			}
			seen[own.VMID] = true
			owns = append(owns, own)
		}
		sort.Slice(owns, func(i, j int) bool { return owns[i].VMID < owns[j].VMID })
	}
	writeVMCTLJSON(w, http.StatusOK, map[string]any{
		"vms":   h.registry.GuestMemoryStatsFor(owns),
		"units": balloonStatsUnits,
	})
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
