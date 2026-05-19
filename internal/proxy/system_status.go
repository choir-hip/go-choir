package proxy

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/buildinfo"
	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

type systemStatusResponse struct {
	Status          string                    `json:"status"`
	Service         string                    `json:"service"`
	GeneratedAt     string                    `json:"generated_at"`
	Build           buildinfo.Info            `json:"build"`
	Lifecycle       lifecycleHealthSummary    `json:"lifecycle"`
	CurrentComputer systemCurrentComputer     `json:"current_computer"`
	Runtime         *systemRuntimeStatus      `json:"runtime,omitempty"`
	VMctl           *systemVMctlStatus        `json:"vmctl,omitempty"`
	Capabilities    systemMonitorCapabilities `json:"capabilities"`
	Warnings        []string                  `json:"warnings,omitempty"`
}

type systemVMctlStatus struct {
	RoutingEnabled  bool                        `json:"routing_enabled"`
	Status          string                      `json:"status,omitempty"`
	ActiveVMs       int                         `json:"active_vms"`
	TotalOwnerships int                         `json:"total_ownerships"`
	IdleEligible    int                         `json:"idle_eligible"`
	Reclaim         systemPressureReclaimPlan   `json:"reclaim"`
	Warmness        vmctl.WarmnessHealthSummary `json:"warmness"`
}

type systemPressureReclaimPlan struct {
	Mode      string                         `json:"mode"`
	Decision  string                         `json:"decision"`
	Reason    string                         `json:"reason"`
	Pressure  vmctl.HostPressureSample       `json:"pressure"`
	Inventory vmctl.PressureReclaimInventory `json:"inventory"`
}

type systemCurrentComputer struct {
	DesktopID        string `json:"desktop_id"`
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

type systemRuntimeStatus struct {
	Reachable        bool           `json:"reachable"`
	Status           string         `json:"status,omitempty"`
	Service          string         `json:"service,omitempty"`
	RuntimeHealth    string         `json:"runtime_health,omitempty"`
	RunningRuns      int            `json:"running_runs,omitempty"`
	ResearcherCount  int            `json:"researcher_count,omitempty"`
	ActiveProvider   string         `json:"active_provider,omitempty"`
	Build            buildinfo.Info `json:"build,omitempty"`
	ObservationError string         `json:"observation_error,omitempty"`
}

type systemMonitorCapabilities struct {
	StatusAPI                bool     `json:"status_api"`
	WakeCurrentComputer      bool     `json:"wake_current_computer"`
	DesktopStateRecovery     bool     `json:"desktop_state_recovery"`
	LazyAppHydration         bool     `json:"lazy_app_hydration"`
	ArbitraryProcessKill     bool     `json:"arbitrary_process_kill"`
	UnsupportedRecoveryModes []string `json:"unsupported_recovery_modes,omitempty"`
}

type systemRecoveryRequest struct {
	Action    string `json:"action"`
	DesktopID string `json:"desktop_id,omitempty"`
}

type systemRecoveryResponse struct {
	OK              bool                  `json:"ok"`
	Action          string                `json:"action"`
	CurrentComputer systemCurrentComputer `json:"current_computer"`
	Runtime         *systemRuntimeStatus  `json:"runtime,omitempty"`
}

func (h *Handler) HandleSystemStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}

	authResult, err := h.validateAccessJWT(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "authentication required"})
		return
	}

	desktopID := requestDesktopID(r)
	resp := systemStatusResponse{
		Status:      "ok",
		Service:     "system-monitor",
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Build:       buildinfo.Snapshot("proxy"),
		Lifecycle:   h.lifecycle.summary(),
		CurrentComputer: systemCurrentComputer{
			DesktopID:        desktopID,
			Kind:             "interactive",
			State:            "unknown",
			WarmnessClass:    "unknown",
			Protection:       "status unavailable",
			LookupStatus:     "unavailable",
			RecoveryEligible: h.vmctlClient != nil,
		},
		Capabilities: systemMonitorCapabilities{
			StatusAPI:            true,
			WakeCurrentComputer:  h.vmctlClient != nil,
			DesktopStateRecovery: true,
			LazyAppHydration:     true,
			ArbitraryProcessKill: false,
			UnsupportedRecoveryModes: []string{
				"arbitrary_process_kill",
				"primary_computer_force_reset",
			},
		},
	}

	if h.cfg != nil && h.cfg.VmctlRoutingEnabled() {
		if vmctlHealth, ok := h.probeVMctlHealth(); ok {
			resp.VMctl = &systemVMctlStatus{
				RoutingEnabled:  true,
				Status:          vmctlHealth.Status,
				ActiveVMs:       vmctlHealth.ActiveVMs,
				TotalOwnerships: vmctlHealth.TotalOwnerships,
				IdleEligible:    vmctlHealth.IdleEligible,
				Reclaim:         redactedPressureReclaimPlan(vmctlHealth.Reclaim),
				Warmness:        vmctlHealth.Warmness,
			}
		} else {
			resp.Status = "degraded"
			resp.Warnings = append(resp.Warnings, "vmctl health is unavailable")
			resp.VMctl = &systemVMctlStatus{RoutingEnabled: true, Status: "unavailable"}
		}
	} else {
		resp.VMctl = &systemVMctlStatus{RoutingEnabled: false, Status: "static"}
	}

	if h.vmctlClient == nil {
		resp.CurrentComputer.State = "static"
		resp.CurrentComputer.WarmnessClass = "static"
		resp.CurrentComputer.LookupStatus = "static"
		resp.CurrentComputer.Protection = "static sandbox routing"
		if h.cfg != nil && strings.TrimSpace(h.cfg.SandboxURL) != "" {
			resp.Runtime = h.probeRuntimeHealthForTarget(h.cfg.SandboxURL)
		}
		writeJSON(w, http.StatusOK, resp)
		return
	}

	own, err := h.vmctlClient.LookupDesktopContext(r.Context(), authResult.UserID, desktopID)
	if err != nil {
		resp.Status = "degraded"
		resp.CurrentComputer.LookupStatus = "error"
		resp.CurrentComputer.Protection = "computer lookup failed"
		resp.Warnings = append(resp.Warnings, "current computer lookup failed")
		writeJSON(w, http.StatusOK, resp)
		return
	}
	if own == nil {
		resp.CurrentComputer.State = "not_started"
		resp.CurrentComputer.WarmnessClass = currentWarmnessFallback(desktopID)
		resp.CurrentComputer.LookupStatus = "not_found"
		resp.CurrentComputer.Protection = protectionText(resp.CurrentComputer.WarmnessClass)
		resp.CurrentComputer.Reclaimable = reclaimableWarmness(resp.CurrentComputer.WarmnessClass)
		writeJSON(w, http.StatusOK, resp)
		return
	}

	resp.CurrentComputer = systemCurrentComputer{
		DesktopID:        own.DesktopID,
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

	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) HandleSystemRecovery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}

	authResult, err := h.validateAccessJWT(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "authentication required"})
		return
	}
	if h.vmctlClient == nil {
		writeJSON(w, http.StatusNotImplemented, errorResponse{Error: "computer recovery requires vmctl routing"})
		return
	}

	var req systemRecoveryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid request body"})
		return
	}
	action := strings.TrimSpace(req.Action)
	desktopID := strings.TrimSpace(req.DesktopID)
	if desktopID == "" {
		desktopID = requestDesktopID(r)
	}

	switch action {
	case "wake_current_computer", "resume_current_computer":
		own, err := h.vmctlClient.ResolveDesktopContext(r.Context(), authResult.UserID, desktopID)
		if err != nil {
			log.Printf("proxy system recovery: wake current computer desktop=%s: %v", desktopID, err)
			writeJSON(w, http.StatusBadGateway, errorResponse{Error: "failed to wake current computer"})
			return
		}
		current := systemCurrentComputer{
			DesktopID:        own.DesktopID,
			Kind:             string(own.Kind),
			State:            own.State,
			WarmnessClass:    own.WarmnessClass,
			Published:        own.Published,
			Protection:       protectionText(own.WarmnessClass),
			Reclaimable:      reclaimableWarmness(own.WarmnessClass),
			RecoveryEligible: true,
			LookupStatus:     "ok",
		}
		if current.WarmnessClass == "" {
			current.WarmnessClass = currentWarmnessFallback(own.DesktopID)
			current.Protection = protectionText(current.WarmnessClass)
			current.Reclaimable = reclaimableWarmness(current.WarmnessClass)
		}
		var runtimeStatus *systemRuntimeStatus
		if own.SandboxURL != "" {
			runtimeStatus = h.probeRuntimeHealthForTarget(own.SandboxURL)
		}
		writeJSON(w, http.StatusOK, systemRecoveryResponse{
			OK:              true,
			Action:          action,
			CurrentComputer: current,
			Runtime:         runtimeStatus,
		})
	default:
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "unsupported recovery action"})
	}
}

func (h *Handler) probeRuntimeHealthForTarget(targetURL string) *systemRuntimeStatus {
	targetURL = strings.TrimSpace(targetURL)
	if targetURL == "" {
		return &systemRuntimeStatus{Reachable: false, ObservationError: "missing target"}
	}
	u, err := url.Parse(targetURL)
	if err != nil {
		return &systemRuntimeStatus{Reachable: false, ObservationError: "invalid target"}
	}
	u.Path = "/health"
	u.RawQuery = ""

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(u.String())
	if err != nil {
		return &systemRuntimeStatus{Reachable: false, ObservationError: "runtime health unavailable"}
	}
	defer func() { _ = resp.Body.Close() }()

	var body struct {
		Status          string         `json:"status"`
		Service         string         `json:"service"`
		RuntimeHealth   string         `json:"runtime_health"`
		RunningRuns     int            `json:"running_runs"`
		ResearcherCount int            `json:"researcher_count"`
		ActiveProvider  string         `json:"active_provider"`
		Build           buildinfo.Info `json:"build"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return &systemRuntimeStatus{Reachable: resp.StatusCode >= 200 && resp.StatusCode < 500, ObservationError: "runtime health decode failed"}
	}
	return &systemRuntimeStatus{
		Reachable:       resp.StatusCode >= 200 && resp.StatusCode < 500,
		Status:          body.Status,
		Service:         body.Service,
		RuntimeHealth:   body.RuntimeHealth,
		RunningRuns:     body.RunningRuns,
		ResearcherCount: body.ResearcherCount,
		ActiveProvider:  body.ActiveProvider,
		Build:           body.Build,
	}
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

func redactedPressureReclaimPlan(plan vmctl.PressureReclaimPlan) systemPressureReclaimPlan {
	return systemPressureReclaimPlan{
		Mode:      plan.Mode,
		Decision:  plan.Decision,
		Reason:    plan.Reason,
		Pressure:  plan.Pressure,
		Inventory: plan.Inventory,
	}
}
