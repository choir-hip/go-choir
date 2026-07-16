package proxy

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/persistentdisk"
	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

type computeStatusResponse struct {
	Status          string                     `json:"status"`
	Service         string                     `json:"service"`
	GeneratedAt     string                     `json:"generated_at"`
	CurrentComputer computeComputer            `json:"current_computer"`
	Computers       []computeComputer          `json:"computers"`
	Runtime         *computeRuntimeStatus      `json:"runtime,omitempty"`
	Recovery        *computeRecoveryStatus     `json:"recovery,omitempty"`
	PersistentDisk  *persistentdisk.Status     `json:"persistent_disk,omitempty"`
	Capabilities    computeMonitorCapabilities `json:"capabilities"`
	Warnings        []string                   `json:"warnings,omitempty"`
}

type computeComputer struct {
	DesktopID        string `json:"desktop_id"`
	Role             string `json:"role,omitempty"`
	Current          bool   `json:"current,omitempty"`
	Kind             string `json:"kind"`
	State            string `json:"state"`
	WarmnessClass    string `json:"warmness_class"`
	Published        bool   `json:"published"`
	Epoch            int64  `json:"epoch,omitempty"`
	StoppedBy        string `json:"stopped_by,omitempty"`
	LastActiveAt     string `json:"last_active_at,omitempty"`
	Protection       string `json:"protection"`
	Reclaimable      bool   `json:"reclaimable"`
	RecoveryEligible bool   `json:"recovery_eligible"`
	LookupStatus     string `json:"lookup_status"`
}

type computeRuntimeStatus struct {
	Reachable        bool                   `json:"reachable"`
	Status           string                 `json:"status,omitempty"`
	Service          string                 `json:"service,omitempty"`
	RuntimeHealth    string                 `json:"runtime_health,omitempty"`
	RunningRuns      int                    `json:"running_runs,omitempty"`
	ResearcherCount  int                    `json:"researcher_count,omitempty"`
	ObservationError string                 `json:"observation_error,omitempty"`
	PersistentDisk   *persistentdisk.Status `json:"persistent_disk,omitempty"`
}

type computeMonitorCapabilities struct {
	StatusAPI                bool     `json:"status_api"`
	WakeCurrentComputer      bool     `json:"wake_current_computer"`
	StopCurrentComputer      bool     `json:"stop_current_computer"`
	DesktopStateRecovery     bool     `json:"desktop_state_recovery"`
	LazyAppHydration         bool     `json:"lazy_app_hydration"`
	ArbitraryProcessKill     bool     `json:"arbitrary_process_kill"`
	UnsupportedRecoveryModes []string `json:"unsupported_recovery_modes,omitempty"`
}

type computeRecoveryRequest struct {
	Action    string `json:"action"`
	DesktopID string `json:"desktop_id,omitempty"`
}

type computeRecoveryResponse struct {
	OK              bool                   `json:"ok"`
	Action          string                 `json:"action"`
	CurrentComputer computeComputer        `json:"current_computer"`
	Runtime         *computeRuntimeStatus  `json:"runtime,omitempty"`
	Recovery        *computeRecoveryStatus `json:"recovery,omitempty"`
}

func (h *Handler) HandleComputeStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}

	authResult, err := h.authenticate(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "authentication required"})
		return
	}
	if !h.authorizeAPIKeyScope(w, r, authResult) {
		return
	}

	desktopID := requestDesktopID(r)
	resp := computeStatusResponse{
		Status:      "ok",
		Service:     "compute-monitor",
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		CurrentComputer: computeComputer{
			DesktopID:        desktopID,
			Role:             computerRole(desktopID),
			Current:          true,
			Kind:             "interactive",
			State:            "unknown",
			WarmnessClass:    "unknown",
			Protection:       "status unavailable",
			LookupStatus:     "unavailable",
			RecoveryEligible: h.vmctlClient != nil,
		},
		Capabilities: computeMonitorCapabilities{
			StatusAPI:            true,
			WakeCurrentComputer:  h.vmctlClient != nil,
			StopCurrentComputer:  h.vmctlClient != nil,
			DesktopStateRecovery: true,
			LazyAppHydration:     true,
			ArbitraryProcessKill: false,
			UnsupportedRecoveryModes: []string{
				"arbitrary_process_kill",
				"primary_computer_force_reset",
			},
		},
	}

	if h.vmctlClient == nil {
		resp.Status = "degraded"
		resp.Computers = []computeComputer{resp.CurrentComputer}
		if h.cfg != nil && h.cfg.AllowDirectSandboxForTests {
			resp.CurrentComputer.State = "static"
			resp.CurrentComputer.WarmnessClass = "static"
			resp.CurrentComputer.LookupStatus = "test-direct"
			resp.CurrentComputer.Protection = "test-only direct sandbox routing"
			resp.Computers = []computeComputer{resp.CurrentComputer}
			if strings.TrimSpace(h.cfg.SandboxURL) != "" {
				resp.Runtime = h.probeRuntimeHealthForTarget(h.cfg.SandboxURL)
			}
		} else {
			resp.Warnings = append(resp.Warnings, "ComputerVersion route authority is not configured")
		}
		h.writeComputeStatus(w, &resp, authResult.UserID, desktopID)
		return
	}
	if err := h.ensureComputerVersionRoute(r.Context(), authResult.UserID, desktopID); err != nil {
		resp.Status = "degraded"
		resp.CurrentComputer.Protection = "immutable ComputerVersion route unavailable"
		resp.Warnings = append(resp.Warnings, err.Error())
		resp.Computers = []computeComputer{resp.CurrentComputer}
		h.writeComputeStatus(w, &resp, authResult.UserID, desktopID)
		return
	}

	own, err := h.vmctlClient.LookupDesktopContext(r.Context(), authResult.UserID, desktopID)
	if err != nil {
		resp.Status = "degraded"
		resp.CurrentComputer.LookupStatus = "error"
		resp.CurrentComputer.Protection = "computer lookup failed"
		resp.Warnings = append(resp.Warnings, "current computer lookup failed")
		listed, listWarnings := h.userComputersForStatus(r.Context(), authResult.UserID, resp.CurrentComputer)
		resp.Computers, resp.Warnings = appendComputerList(resp.Computers, resp.Warnings, listed, listWarnings)
		h.writeComputeStatus(w, &resp, authResult.UserID, desktopID)
		return
	}
	if own == nil {
		resp.CurrentComputer.State = "not_started"
		resp.CurrentComputer.WarmnessClass = currentWarmnessFallback(desktopID)
		resp.CurrentComputer.LookupStatus = "not_found"
		resp.CurrentComputer.Protection = protectionText(resp.CurrentComputer.WarmnessClass)
		resp.CurrentComputer.Reclaimable = reclaimableWarmness(resp.CurrentComputer.WarmnessClass)
		listed, listWarnings := h.userComputersForStatus(r.Context(), authResult.UserID, resp.CurrentComputer)
		resp.Computers, resp.Warnings = appendComputerList(resp.Computers, resp.Warnings, listed, listWarnings)
		h.writeComputeStatus(w, &resp, authResult.UserID, desktopID)
		return
	}

	resp.CurrentComputer = computeComputer{
		DesktopID:        own.DesktopID,
		Role:             computerRole(own.DesktopID),
		Current:          true,
		Kind:             string(own.Kind),
		State:            own.State,
		WarmnessClass:    own.WarmnessClass,
		Published:        own.Published,
		Epoch:            own.Epoch,
		StoppedBy:        own.StoppedBy,
		LastActiveAt:     own.LastActiveAt,
		Protection:       protectionText(own.WarmnessClass),
		Reclaimable:      reclaimableWarmness(own.WarmnessClass),
		RecoveryEligible: true,
		LookupStatus:     "ok",
	}
	if resp.CurrentComputer.WarmnessClass == "" {
		resp.CurrentComputer.WarmnessClass = currentWarmnessFallback(own.DesktopID)
		resp.CurrentComputer.Protection = protectionText(resp.CurrentComputer.WarmnessClass)
		resp.CurrentComputer.Reclaimable = reclaimableWarmness(resp.CurrentComputer.WarmnessClass)
	}
	if own.SandboxURL != "" && strings.EqualFold(own.State, string(vmctl.VMStateActive)) {
		resp.Runtime = h.probeRuntimeHealthForTarget(own.SandboxURL)
	}
	if resp.Runtime != nil && resp.Runtime.PersistentDisk != nil {
		resp.PersistentDisk = resp.Runtime.PersistentDisk
	} else if own.DataImage != nil {
		resp.PersistentDisk = persistentDiskFromHostImage(own.DataImage.FileBytes, own.DataImage.CapBytes)
	}
	if resp.PersistentDisk != nil {
		resp.Warnings = appendPersistentDiskWarnings(resp.Warnings, resp.PersistentDisk)
	}
	listed, listWarnings := h.userComputersForStatus(r.Context(), authResult.UserID, resp.CurrentComputer)
	resp.Computers, resp.Warnings = appendComputerList(resp.Computers, resp.Warnings, listed, listWarnings)

	h.writeComputeStatus(w, &resp, authResult.UserID, desktopID)
}

func (h *Handler) writeComputeStatus(w http.ResponseWriter, resp *computeStatusResponse, userID, desktopID string) {
	if resp != nil && h != nil && h.recoveries != nil {
		if recovery, _, _, _, ok := h.recoveries.snapshot(userID, desktopID); ok {
			resp.Recovery = recovery
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

func appendComputerList(computers []computeComputer, warnings []string, listed []computeComputer, listWarnings []string) ([]computeComputer, []string) {
	if len(listed) > 0 {
		computers = listed
	}
	if len(listWarnings) > 0 {
		warnings = append(warnings, listWarnings...)
	}
	return computers, warnings
}

func (h *Handler) userComputersForStatus(ctx context.Context, userID string, current computeComputer) ([]computeComputer, []string) {
	if h.vmctlClient == nil {
		return []computeComputer{current}, nil
	}
	owns, err := h.vmctlClient.ListOwnershipsContext(ctx)
	if err != nil {
		return []computeComputer{current}, []string{"user computer list unavailable"}
	}
	seen := map[string]bool{}
	computers := make([]computeComputer, 0, len(owns)+1)
	for _, own := range owns {
		if own.UserID != userID {
			continue
		}
		if own.Kind != "" && own.Kind != vmctl.VMKindInteractive {
			continue
		}
		computer := computeComputer{
			DesktopID:        own.DesktopID,
			Role:             computerRole(own.DesktopID),
			Current:          own.DesktopID == current.DesktopID,
			Kind:             string(own.Kind),
			State:            own.State,
			WarmnessClass:    own.WarmnessClass,
			Published:        own.Published,
			Epoch:            own.Epoch,
			StoppedBy:        own.StoppedBy,
			LastActiveAt:     own.LastActiveAt,
			Protection:       protectionText(own.WarmnessClass),
			Reclaimable:      reclaimableWarmness(own.WarmnessClass),
			RecoveryEligible: true,
			LookupStatus:     "ok",
		}
		if computer.WarmnessClass == "" {
			computer.WarmnessClass = currentWarmnessFallback(own.DesktopID)
			computer.Protection = protectionText(computer.WarmnessClass)
			computer.Reclaimable = reclaimableWarmness(computer.WarmnessClass)
		}
		computers = append(computers, computer)
		seen[own.DesktopID] = true
	}
	if current.DesktopID != "" && !seen[current.DesktopID] {
		computers = append(computers, current)
	}
	sort.SliceStable(computers, func(i, j int) bool {
		if computers[i].Current != computers[j].Current {
			return computers[i].Current
		}
		if computers[i].Role != computers[j].Role {
			return computers[i].Role == "primary"
		}
		return computers[i].DesktopID < computers[j].DesktopID
	})
	return computers, nil
}

func (h *Handler) HandleComputeRecovery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}

	authResult, err := h.authenticate(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "authentication required"})
		return
	}
	if !h.authorizeAPIKeyScope(w, r, authResult) {
		return
	}
	if h.vmctlClient == nil {
		writeJSON(w, http.StatusNotImplemented, errorResponse{Error: "computer recovery requires vmctl routing"})
		return
	}

	var req computeRecoveryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid request body"})
		return
	}
	action := strings.TrimSpace(req.Action)
	desktopID := strings.TrimSpace(req.DesktopID)
	if desktopID == "" {
		desktopID = requestDesktopID(r)
	}
	if err := h.ensureComputerVersionRoute(r.Context(), authResult.UserID, desktopID); err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "immutable ComputerVersion route unavailable"})
		return
	}

	switch action {
	case "wake_current_computer", "resume_current_computer":
		op := h.recoveries.startOrJoin(authResult.UserID, desktopID, action, func(ctx context.Context) computeRecoveryRunResult {
			current, runtimeStatus, runErr := h.runComputeRecovery(ctx, authResult.UserID, desktopID)
			return computeRecoveryRunResult{
				Current: current,
				Runtime: runtimeStatus,
				Err:     runErr,
			}
		})
		if op == nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to start computer recovery"})
			return
		}
		select {
		case <-op.done:
			recovery, current, runtimeStatus, runErr, _ := h.recoveries.snapshot(authResult.UserID, desktopID)
			if runErr != nil {
				writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to recover current computer"})
				return
			}
			writeJSON(w, http.StatusOK, computeRecoveryResponse{
				OK:              true,
				Action:          action,
				CurrentComputer: current,
				Runtime:         runtimeStatus,
				Recovery:        recovery,
			})
		case <-r.Context().Done():
			return
		case <-time.After(2 * time.Second):
			recovery, current, runtimeStatus, _, _ := h.recoveries.snapshot(authResult.UserID, desktopID)
			if current.DesktopID == "" {
				current = computeComputerFromFields(
					desktopID,
					string(vmctl.VMKindInteractive),
					"refreshing",
					currentWarmnessFallback(desktopID),
					desktopID == vmctl.PrimaryDesktopID,
					0,
					"",
					"",
				)
			}
			writeJSON(w, http.StatusAccepted, computeRecoveryResponse{
				OK:              true,
				Action:          action,
				CurrentComputer: current,
				Runtime:         runtimeStatus,
				Recovery:        recovery,
			})
		}
	case "stop_current_computer":
		if err := h.vmctlClient.StopDesktop(authResult.UserID, desktopID); err != nil {
			log.Printf("proxy compute recovery: stop current computer desktop=%s: %v", desktopID, err)
			writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to stop current computer"})
			return
		}
		current := computeComputerFromFields(
			desktopID,
			string(vmctl.VMKindInteractive),
			string(vmctl.VMStateStopped),
			currentWarmnessFallback(desktopID),
			desktopID == vmctl.PrimaryDesktopID,
			0,
			"user",
			"",
		)
		if own, lookupErr := h.vmctlClient.LookupDesktopContext(r.Context(), authResult.UserID, desktopID); lookupErr == nil && own != nil {
			current = computeComputerFromFields(
				own.DesktopID,
				string(own.Kind),
				own.State,
				own.WarmnessClass,
				own.Published,
				own.Epoch,
				own.StoppedBy,
				own.LastActiveAt,
			)
		}
		writeJSON(w, http.StatusOK, computeRecoveryResponse{
			OK:              true,
			Action:          action,
			CurrentComputer: current,
		})
	default:
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "unsupported recovery action"})
	}
}

func (h *Handler) runComputeRecovery(ctx context.Context, userID, desktopID string) (computeComputer, *computeRuntimeStatus, error) {
	own, err := h.vmctlClient.LookupDesktopContext(ctx, userID, desktopID)
	if err != nil {
		log.Printf("proxy compute recovery: lookup current computer desktop=%s: %v", desktopID, err)
		return computeComputer{}, nil, err
	}

	var runtimeStatus *computeRuntimeStatus
	var current computeComputer
	if own == nil {
		resolved, resolveErr := h.vmctlClient.ResolveDesktopContext(ctx, userID, desktopID)
		if resolveErr != nil {
			log.Printf("proxy compute recovery: wake current computer desktop=%s: %v", desktopID, resolveErr)
			return computeComputer{}, nil, resolveErr
		}
		current = computeComputerFromFields(
			resolved.DesktopID,
			string(resolved.Kind),
			resolved.State,
			resolved.WarmnessClass,
			resolved.Published,
			0,
			"",
			"",
		)
		if resolved.SandboxURL != "" {
			runtimeStatus = h.probeRuntimeHealthForTarget(resolved.SandboxURL)
		}
	} else if own.State == string(vmctl.VMStateStopped) || own.State == string(vmctl.VMStateHibernated) {
		resolved, resolveErr := h.vmctlClient.ResolveDesktopContext(ctx, userID, desktopID)
		if resolveErr == nil {
			current = computeComputerFromFields(
				resolved.DesktopID,
				string(resolved.Kind),
				resolved.State,
				resolved.WarmnessClass,
				resolved.Published,
				0,
				"",
				"",
			)
			if resolved.SandboxURL != "" {
				runtimeStatus = h.probeRuntimeHealthForTarget(resolved.SandboxURL)
			}
		} else {
			log.Printf("proxy compute recovery: wake current computer failed; refreshing stopped desktop=%s: %v", desktopID, resolveErr)
			refreshed, refreshErr := h.vmctlClient.RefreshDesktopContext(ctx, userID, desktopID)
			if refreshErr != nil {
				log.Printf("proxy compute recovery: refresh stopped current computer desktop=%s: %v", desktopID, refreshErr)
				return computeComputer{}, nil, refreshErr
			}
			current = computeComputerFromFields(
				refreshed.DesktopID,
				string(refreshed.Kind),
				refreshed.State,
				refreshed.WarmnessClass,
				refreshed.Published,
				0,
				"",
				"",
			)
			if refreshed.SandboxURL != "" {
				runtimeStatus = h.probeRuntimeHealthForTarget(refreshed.SandboxURL)
			}
		}
	} else {
		current = computeComputerFromFields(
			own.DesktopID,
			string(own.Kind),
			own.State,
			own.WarmnessClass,
			own.Published,
			own.Epoch,
			own.StoppedBy,
			own.LastActiveAt,
		)
	}

	ownWasStopped := own != nil && (own.State == string(vmctl.VMStateStopped) || own.State == string(vmctl.VMStateHibernated))
	if own != nil && !ownWasStopped && own.SandboxURL != "" {
		runtimeStatus = h.probeRuntimeHealthForTarget(own.SandboxURL)
	}
	shouldRefresh := own != nil && !ownWasStopped && (own.SandboxURL == "" ||
		own.State == string(vmctl.VMStateBooting) ||
		own.State == string(vmctl.VMStateDegraded) ||
		own.State == string(vmctl.VMStateFailed) ||
		(runtimeStatus != nil && !runtimeStatus.Reachable))
	if shouldRefresh {
		refreshed, refreshErr := h.vmctlClient.RefreshDesktopContext(ctx, userID, desktopID)
		if refreshErr != nil {
			log.Printf("proxy compute recovery: refresh unreachable current computer desktop=%s: %v", desktopID, refreshErr)
		} else {
			current = computeComputerFromFields(
				refreshed.DesktopID,
				string(refreshed.Kind),
				refreshed.State,
				refreshed.WarmnessClass,
				refreshed.Published,
				0,
				"",
				"",
			)
			if refreshed.SandboxURL != "" {
				runtimeStatus = h.probeRuntimeHealthForTarget(refreshed.SandboxURL)
			}
		}
	}
	return current, runtimeStatus, nil
}

func computeComputerFromFields(desktopID, kind, state, warmnessClass string, published bool, epoch int64, stoppedBy, lastActiveAt string) computeComputer {
	current := computeComputer{
		DesktopID:        desktopID,
		Role:             computerRole(desktopID),
		Current:          true,
		Kind:             kind,
		State:            state,
		WarmnessClass:    warmnessClass,
		Published:        published,
		Epoch:            epoch,
		StoppedBy:        stoppedBy,
		LastActiveAt:     lastActiveAt,
		Protection:       protectionText(warmnessClass),
		Reclaimable:      reclaimableWarmness(warmnessClass),
		RecoveryEligible: true,
		LookupStatus:     "ok",
	}
	if current.WarmnessClass == "" {
		current.WarmnessClass = currentWarmnessFallback(desktopID)
		current.Protection = protectionText(current.WarmnessClass)
		current.Reclaimable = reclaimableWarmness(current.WarmnessClass)
	}
	return current
}

func (h *Handler) probeRuntimeHealthForTarget(targetURL string) *computeRuntimeStatus {
	targetURL = strings.TrimSpace(targetURL)
	if targetURL == "" {
		return &computeRuntimeStatus{Reachable: false, ObservationError: "missing target"}
	}
	u, err := url.Parse(targetURL)
	if err != nil {
		return &computeRuntimeStatus{Reachable: false, ObservationError: "invalid target"}
	}
	u.Path = "/health"
	u.RawQuery = ""

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(u.String())
	if err != nil {
		return &computeRuntimeStatus{Reachable: false, ObservationError: "runtime health unavailable"}
	}
	defer func() { _ = resp.Body.Close() }()

	var body struct {
		Status          string                 `json:"status"`
		Service         string                 `json:"service"`
		RuntimeHealth   string                 `json:"runtime_health"`
		RunningRuns     int                    `json:"running_runs"`
		ResearcherCount int                    `json:"researcher_count"`
		PersistentDisk  *persistentdisk.Status `json:"persistent_disk,omitempty"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return &computeRuntimeStatus{Reachable: resp.StatusCode >= 200 && resp.StatusCode < 500, ObservationError: "runtime health decode failed"}
	}
	runtimeStatus := &computeRuntimeStatus{
		Reachable:       resp.StatusCode >= 200 && resp.StatusCode < 500,
		Status:          body.Status,
		Service:         body.Service,
		RuntimeHealth:   body.RuntimeHealth,
		RunningRuns:     body.RunningRuns,
		ResearcherCount: body.ResearcherCount,
	}
	if body.PersistentDisk != nil {
		runtimeStatus.PersistentDisk = body.PersistentDisk
	}
	return runtimeStatus
}

func persistentDiskFromHostImage(fileBytes, capBytes uint64) *persistentdisk.Status {
	if capBytes == 0 {
		return nil
	}
	status := persistentdisk.StatusFromHostImage(fileBytes, capBytes)
	return &status
}

func appendPersistentDiskWarnings(warnings []string, disk *persistentdisk.Status) []string {
	if disk == nil {
		return warnings
	}
	if disk.Critical {
		return append(warnings, "persistent data image is critically full")
	}
	if disk.Warning {
		return append(warnings, "persistent data image is nearing capacity")
	}
	return warnings
}

func computerRole(desktopID string) string {
	if strings.TrimSpace(desktopID) == "" || desktopID == vmctl.PrimaryDesktopID {
		return "primary"
	}
	return "candidate"
}

func currentWarmnessFallback(desktopID string) string {
	if strings.TrimSpace(desktopID) == "" || desktopID == vmctl.PrimaryDesktopID {
		return string(vmctl.WarmnessClassPrimary)
	}
	return string(vmctl.WarmnessClassCandidate)
}

func protectionText(class string) string {
	switch strings.TrimSpace(class) {
	case string(vmctl.WarmnessClassPremiumAlwaysOn):
		return "protected always-on primary computer"
	case string(vmctl.WarmnessClassCriticalProtected):
		return "protected critical background work"
	case string(vmctl.WarmnessClassPrimary):
		return "primary computer kept warm while capacity allows"
	case string(vmctl.WarmnessClassPublicPlatform):
		return "public platform computer lane"
	case string(vmctl.WarmnessClassCandidate):
		return "candidate computer; hibernates before primary desktops"
	case string(vmctl.WarmnessClassWorker):
		return "worker computer; lowest retention priority"
	case "static":
		return "static sandbox routing"
	default:
		return "priority class unavailable"
	}
}

func reclaimableWarmness(class string) bool {
	switch strings.TrimSpace(class) {
	case string(vmctl.WarmnessClassCandidate), string(vmctl.WarmnessClassWorker):
		return true
	default:
		return false
	}
}
