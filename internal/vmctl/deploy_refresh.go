package vmctl

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// Deploy refresh: how a deploy moves active computers onto the new guest
// image without crash-closing their work (owner rule "Restarts End Work
// (Crash) Or Resume It (Planned Update)";
// docs/problems/owner-computer-pinned-as-canary-2026-10-13.md).
//
// Before killing a VM for a deploy, vmctl asks the guest to write its own
// planned-restart marker (POST /internal/runtime/planned-restart). The guest
// refuses while busy, and vmctl leaves that computer for the next deploy. The
// operator refresh (/internal/vmctl/refresh) stays a forced, markerless
// (crash-boot) refresh for explicit owner/operator requests.

const plannedRestartGuestPath = "/internal/runtime/planned-restart"

type guestMarkOutcome int

const (
	// guestMarkWritten: the guest's marker is durable; refresh now.
	guestMarkWritten guestMarkOutcome = iota + 1
	// guestMarkBusy: the guest refused (409); defer to the next deploy.
	guestMarkBusy
	// guestMarkUnsupported: the guest predates the endpoint (404/405).
	guestMarkUnsupported
	// guestMarkUnavailable: no usable answer; never kill blind.
	guestMarkUnavailable
)

func (o guestMarkOutcome) String() string {
	switch o {
	case guestMarkWritten:
		return "written"
	case guestMarkBusy:
		return "busy"
	case guestMarkUnsupported:
		return "unsupported"
	case guestMarkUnavailable:
		return "unavailable"
	}
	return "unknown"
}

type guestPlannedRestartMarker interface {
	mark(ctx context.Context, own VMOwnership, target string) (guestMarkOutcome, string)
	clear(ctx context.Context, own VMOwnership)
}

// httpGuestMarker reaches the guest at its computer_url from the host, the
// same host-peer transport the guest's /internal/runtime routes require.
type httpGuestMarker struct {
	client *http.Client
}

func (m httpGuestMarker) httpClient() *http.Client {
	if m.client != nil {
		return m.client
	}
	// The busy check reads the guest's self-development operations; give a
	// contended store a few seconds.
	return &http.Client{Timeout: 20 * time.Second}
}

func (m httpGuestMarker) mark(ctx context.Context, own VMOwnership, target string) (guestMarkOutcome, string) {
	base := strings.TrimRight(strings.TrimSpace(own.ComputerURL), "/")
	if base == "" {
		return guestMarkUnavailable, "computer has no URL"
	}
	body, _ := json.Marshal(map[string]string{"target": target})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+plannedRestartGuestPath, bytes.NewReader(body))
	if err != nil {
		return guestMarkUnavailable, err.Error()
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Caller", "true")
	resp, err := m.httpClient().Do(req)
	if err != nil {
		return guestMarkUnavailable, err.Error()
	}
	defer resp.Body.Close()
	detail := strings.TrimSpace(string(mustReadLimited(resp.Body, 4<<10)))
	switch resp.StatusCode {
	case http.StatusOK:
		return guestMarkWritten, detail
	case http.StatusConflict:
		return guestMarkBusy, detail
	case http.StatusNotFound, http.StatusMethodNotAllowed:
		return guestMarkUnsupported, detail
	}
	return guestMarkUnavailable, fmt.Sprintf("guest answered %d: %s", resp.StatusCode, detail)
}

func (m httpGuestMarker) clear(ctx context.Context, own VMOwnership) {
	base := strings.TrimRight(strings.TrimSpace(own.ComputerURL), "/")
	if base == "" {
		return
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, base+plannedRestartGuestPath, nil)
	if err != nil {
		return
	}
	req.Header.Set("X-Internal-Caller", "true")
	resp, err := m.httpClient().Do(req)
	if err != nil {
		log.Printf("vmctl: deploy refresh: clear planned-restart marker on %s: %v (it expires on its own)", own.VMID, err)
		return
	}
	_ = resp.Body.Close()
}

func mustReadLimited(r io.Reader, n int64) []byte {
	raw, _ := io.ReadAll(io.LimitReader(r, n))
	return raw
}

func (h *Handler) plannedRestartMarker() guestPlannedRestartMarker {
	if h.guestMarker != nil {
		return h.guestMarker
	}
	return httpGuestMarker{}
}

const (
	deployRefreshRefreshed = "refreshed"
	deployRefreshDeferred  = "deferred"
	deployRefreshSkipped   = "skipped"
	deployRefreshFailed    = "failed"
)

type deployRefreshRequest struct {
	UserID    string `json:"user_id"`
	DesktopID string `json:"desktop_id,omitempty"`
	// Target is the build the refresh moves the computer onto; it is recorded
	// in the guest's planned-restart marker.
	Target string `json:"target"`
}

type deployRefreshResponse struct {
	Outcome        string `json:"outcome"`
	Reason         string `json:"reason,omitempty"`
	PlannedRestart bool   `json:"planned_restart"`
	VMID           string `json:"vm_id,omitempty"`
	ComputerID     string `json:"computer_id,omitempty"`
	UserID         string `json:"user_id,omitempty"`
	DesktopID      string `json:"desktop_id,omitempty"`
	ComputerURL    string `json:"computer_url,omitempty"`
	State          string `json:"state,omitempty"`
}

type deployRefreshCandidate struct {
	VMID          string `json:"vm_id"`
	ComputerID    string `json:"computer_id"`
	UserID        string `json:"user_id"`
	DesktopID     string `json:"desktop_id"`
	ComputerURL   string `json:"computer_url"`
	WarmnessClass string `json:"warmness_class,omitempty"`
	SnapshotKind  string `json:"snapshot_kind,omitempty"`
	Reason        string `json:"reason,omitempty"`
}

type deployRefreshPlan struct {
	Candidates []deployRefreshCandidate `json:"candidates"`
	Skipped    []deployRefreshCandidate `json:"skipped"`
}

// selectDeployRefresh picks the computers a deploy refreshes: every active
// interactive computer, constructed computer versions and always-on ones
// included, except held ones. Busy computers and those with a
// self-development operation in flight are decided per computer by the
// guest's marker refusal, not here.
func selectDeployRefresh(ownerships []ownershipResponse) deployRefreshPlan {
	plan := deployRefreshPlan{Candidates: []deployRefreshCandidate{}, Skipped: []deployRefreshCandidate{}}
	for _, own := range ownerships {
		if own.State != string(VMStateActive) {
			continue
		}
		if own.Kind != "" && own.Kind != VMKindInteractive {
			continue
		}
		desktopID := normalizeDesktopID(own.DesktopID)
		entry := deployRefreshCandidate{
			VMID: own.VMID, ComputerID: own.ComputerID, UserID: own.UserID, DesktopID: desktopID,
			ComputerURL: own.ComputerURL, WarmnessClass: own.WarmnessClass, SnapshotKind: own.SnapshotKind,
		}
		if own.Held {
			entry.Reason = "held"
			plan.Skipped = append(plan.Skipped, entry)
			continue
		}
		plan.Candidates = append(plan.Candidates, entry)
	}
	return plan
}

// HandleDeployRefresh serves /internal/vmctl/deploy-refresh.
//
//   - GET lists the deploy refresh selection (candidates and skipped).
//   - POST {"user_id","desktop_id","target"} refreshes one computer through
//     the guest's planned-restart marker. 200 outcome "refreshed"; 409
//     outcome "deferred" (busy, unreachable) or "skipped" (held, not active,
//     divergent) — neither is a deploy failure; other errors are "failed".
func (h *Handler) HandleDeployRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		writeVMCTLJSON(w, http.StatusMethodNotAllowed, vmctlErrorResponse{Error: "method not allowed"})
		return
	}
	if !isInternalCaller(r) {
		writeVMCTLJSON(w, http.StatusForbidden, vmctlErrorResponse{Error: "vmctl control endpoints are not publicly accessible"})
		return
	}
	if r.Method == http.MethodGet {
		ownerships, err := h.listOwnershipResponses(r.Context())
		if err != nil {
			writeVMCTLJSON(w, http.StatusServiceUnavailable, vmctlErrorResponse{Error: err.Error()})
			return
		}
		writeVMCTLJSON(w, http.StatusOK, selectDeployRefresh(ownerships))
		return
	}

	var req deployRefreshRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req); err != nil {
		writeVMCTLJSON(w, http.StatusBadRequest, vmctlErrorResponse{Error: "invalid request body"})
		return
	}
	req.UserID = strings.TrimSpace(req.UserID)
	req.Target = strings.TrimSpace(req.Target)
	if req.UserID == "" || req.Target == "" {
		writeVMCTLJSON(w, http.StatusBadRequest, vmctlErrorResponse{Error: "user_id and target are required"})
		return
	}
	req.DesktopID = normalizeDesktopID(req.DesktopID)
	if err := h.requireComputerVersionRoute(r.Context(), req.UserID, req.DesktopID); err != nil {
		writeVMCTLJSON(w, http.StatusConflict, vmctlErrorResponse{Error: err.Error()})
		return
	}
	resp, status := h.deployRefresh(r.Context(), req)
	log.Printf("vmctl: deploy refresh user=%s desktop=%s vm=%s target=%s outcome=%s planned_restart=%v reason=%q",
		req.UserID, req.DesktopID, resp.VMID, req.Target, resp.Outcome, resp.PlannedRestart, resp.Reason)
	writeVMCTLJSON(w, status, resp)
}

func (h *Handler) deployRefresh(ctx context.Context, req deployRefreshRequest) (deployRefreshResponse, int) {
	existing := h.registry.GetOwnershipForDesktop(req.UserID, req.DesktopID)
	if existing == nil {
		return deployRefreshResponse{Outcome: deployRefreshSkipped, Reason: "no computer", UserID: req.UserID, DesktopID: req.DesktopID}, http.StatusNotFound
	}
	own := *existing
	base := deployRefreshResponse{VMID: own.VMID, ComputerID: own.ComputerID, UserID: own.UserID, DesktopID: own.DesktopID, ComputerURL: own.ComputerURL, State: string(own.State)}
	reply := func(outcome, reason string, status int) (deployRefreshResponse, int) {
		base.Outcome, base.Reason = outcome, reason
		return base, status
	}
	switch {
	case own.IsHeld():
		return reply(deployRefreshSkipped, "held", http.StatusConflict)
	case own.State != VMStateActive:
		// A computer that is not running boots the current image when it
		// next starts; there is nothing to restart.
		return reply(deployRefreshSkipped, "not active (state="+string(own.State)+")", http.StatusConflict)
	case own.DivergenceStatus == "divergent":
		return reply(deployRefreshSkipped, "divergent", http.StatusConflict)
	}

	marker := h.plannedRestartMarker()
	outcome, detail := marker.mark(ctx, own, req.Target)
	planned := false
	switch outcome {
	case guestMarkWritten:
		planned = true
	case guestMarkBusy:
		return reply(deployRefreshDeferred, "busy: "+detail, http.StatusConflict)
	case guestMarkUnsupported:
		// A guest built before the endpoint cannot write a marker, and the
		// deploy that ships the endpoint must still reach it. Refresh it only
		// when its /health reports no work (the idle sweep's test); the boot
		// is then a crash boot, as an idle stop and wake would be.
		if h.registry.guestBusy()(&own) {
			return reply(deployRefreshDeferred, "busy (guest predates planned-restart; /health reports work or did not answer)", http.StatusConflict)
		}
	default:
		return reply(deployRefreshDeferred, "guest did not answer the planned-restart request: "+detail, http.StatusConflict)
	}

	refreshed, err := h.registry.RefreshVMForDesktop(req.UserID, req.DesktopID)
	if err != nil {
		if planned {
			// The marker must not outlive a restart that did not happen.
			marker.clear(ctx, own)
		}
		status := http.StatusInternalServerError
		if RecoveryRefusalFrom(err) != nil {
			status = http.StatusConflict
		}
		return reply(deployRefreshFailed, err.Error(), status)
	}
	resp := deployRefreshResponse{
		Outcome:        deployRefreshRefreshed,
		PlannedRestart: planned,
		VMID:           refreshed.VMID,
		ComputerID:     stableComputerID(refreshed.UserID, refreshed.DesktopID, refreshed.ComputerID),
		UserID:         refreshed.UserID,
		DesktopID:      refreshed.DesktopID,
		ComputerURL:    refreshed.ComputerURL,
		State:          string(refreshed.State),
	}
	if !planned {
		resp.Reason = "guest predates planned-restart; idle, refreshed as a crash boot"
	}
	return resp, http.StatusOK
}
