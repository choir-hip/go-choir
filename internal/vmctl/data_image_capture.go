package vmctl

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"
)

// DataImageCapture is one host capture of a running computer's data image:
// a reflink clone taken while the VM was paused (vmmanager
// data_image_capture.go; S2 of docs/problems/paper-never-checkpointed-2026-10-27.md).
type DataImageCapture struct {
	Path      string
	PausedFor time.Duration
	ClonedFor time.Duration
}

// dataImageCapturer is implemented by the Firecracker manager adapter.
type dataImageCapturer interface {
	CaptureDataImage(vmID string) (DataImageCapture, error)
}

// ErrVMOperationInProgress is a capture refused because another operation
// holds the VM; the manager adapter wraps the manager's own error in it.
var ErrVMOperationInProgress = errors.New("VM operation in progress")

type captureDataImageRequest struct {
	ComputerID string `json:"computer_id"`
	// DryRun names the computer's VM and owner without pausing it.
	DryRun bool `json:"dry_run,omitempty"`
}

type captureDataImageResponse struct {
	VMID        string `json:"vm_id"`
	ComputerID  string `json:"computer_id"`
	UserID      string `json:"user_id"`
	CapturePath string `json:"capture_path"`
	PausedMS    int64  `json:"paused_ms"`
	ClonedMS    int64  `json:"cloned_ms"`
}

// HandleCaptureDataImage serves POST /internal/vmctl/capture-data-image
// {"computer_id"}: pause the computer's VM, reflink its data image beside it,
// resume. The caller (`checkpointd capture-base`) owns the capture and
// removes it. An operator act for one named computer; nothing calls it on a
// schedule.
func (h *Handler) HandleCaptureDataImage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeVMCTLJSON(w, http.StatusMethodNotAllowed, vmctlErrorResponse{Error: "method not allowed"})
		return
	}
	if !isInternalCaller(r) {
		writeVMCTLJSON(w, http.StatusForbidden, vmctlErrorResponse{Error: "vmctl control endpoints are not publicly accessible"})
		return
	}
	var req captureDataImageRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req); err != nil || strings.TrimSpace(req.ComputerID) == "" {
		writeVMCTLJSON(w, http.StatusBadRequest, vmctlErrorResponse{Error: "computer_id is required"})
		return
	}
	h.registry.mu.RLock()
	mgr := h.registry.vmManager
	h.registry.mu.RUnlock()
	capturer, ok := mgr.(dataImageCapturer)
	if !ok || capturer == nil {
		writeVMCTLJSON(w, http.StatusNotImplemented, vmctlErrorResponse{Error: "data image capture unsupported by this VM manager"})
		return
	}
	// A direct registry lookup: one computer's capture does not wait on
	// classifying every other ownership (review 364). The route-authority
	// classification listOwnershipResponses adds (constructed-version
	// identity, disk receipt) is not needed here: the capture reads the
	// active interactive VM's own data image, whatever version built it, and
	// the base it yields is checked against the tape off the guest, so an
	// unavailable route authority need not block it (review 365).
	computerID := strings.TrimSpace(req.ComputerID)
	for _, own := range h.registry.ListOwnerships() {
		if stableComputerID(own.UserID, own.DesktopID, own.ComputerID) != computerID || own.State != VMStateActive || (own.Kind != "" && own.Kind != VMKindInteractive) {
			continue
		}
		if req.DryRun {
			writeVMCTLJSON(w, http.StatusOK, captureDataImageResponse{VMID: own.VMID, ComputerID: computerID, UserID: own.UserID})
			return
		}
		c, err := capturer.CaptureDataImage(own.VMID)
		if err != nil {
			log.Printf("vmctl: data image capture %s (computer %s): %v", own.VMID, computerID, err)
			code := http.StatusInternalServerError
			if errors.Is(err, ErrVMOperationInProgress) {
				// Another operation holds the VM: contention, not failure.
				code = http.StatusConflict
			}
			writeVMCTLJSON(w, code, vmctlErrorResponse{Error: err.Error()})
			return
		}
		log.Printf("vmctl: data image capture %s (computer %s): %s paused %s", own.VMID, computerID, c.Path, c.PausedFor.Round(time.Millisecond))
		writeVMCTLJSON(w, http.StatusOK, captureDataImageResponse{
			VMID: own.VMID, ComputerID: computerID, UserID: own.UserID, CapturePath: c.Path,
			PausedMS: c.PausedFor.Milliseconds(), ClonedMS: c.ClonedFor.Milliseconds(),
		})
		return
	}
	writeVMCTLJSON(w, http.StatusNotFound, vmctlErrorResponse{Error: "no active computer " + computerID})
}
