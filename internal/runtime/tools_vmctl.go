package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/vmctl"
)

const (
	defaultDelegateWorkerVMTimeout = 15 * time.Minute
	maxDelegateWorkerVMTimeout     = 15 * time.Minute
	maxDelegateWorkerRunAttempts   = 2
)

func RegisterVMControlTools(registry *ToolRegistry, rt *Runtime, cwd string) error {
	for _, tool := range []Tool{
		newForkDesktopTool(rt),
		newPublishDesktopTool(rt),
		newRequestWorkerVMTool(rt),
		newStartWorkerDelegationTool(rt, cwd),
		newObserveWorkerDelegationTool(rt),
		newRedirectWorkerDelegationTool(rt),
		newFinishWorkerDelegationTool(rt),
		newCancelWorkerDelegationTool(rt),
		newDelegateWorkerVMTool(rt, cwd),
	} {
		if err := registry.Register(tool); err != nil {
			return err
		}
	}
	return nil
}

func newForkDesktopTool(rt *Runtime) Tool {
	type args struct {
		DesktopID string `json:"desktop_id,omitempty"`
	}
	return Tool{
		Name:        "fork_desktop",
		Description: "Create a background candidate desktop VM cloned from the current desktop's layout, without exposing it for user switching yet.",
		Parameters:  jsonSchemaObject(map[string]any{"desktop_id": map[string]any{"type": "string"}}, nil, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var in args
			if len(raw) > 0 {
				if err := json.Unmarshal(raw, &in); err != nil {
					return "", fmt.Errorf("decode fork_desktop args: %w", err)
				}
			}
			if rt == nil {
				return "", fmt.Errorf("fork_desktop missing runtime")
			}
			if strings.TrimSpace(rt.cfg.VmctlURL) == "" {
				return "", fmt.Errorf("fork_desktop requires runtime vmctl configuration")
			}

			ownerID := stringFromToolContext(ctx, toolCtxOwnerID)
			sourceDesktopID := strings.TrimSpace(stringFromToolContext(ctx, toolCtxDesktopID))
			if ownerID == "" {
				return "", fmt.Errorf("fork_desktop missing owner context")
			}
			if sourceDesktopID == "" {
				sourceDesktopID = types.PrimaryDesktopID
			}

			targetDesktopID := normalizeForkDesktopID(in.DesktopID)
			if targetDesktopID == sourceDesktopID {
				return "", fmt.Errorf("fork_desktop target must differ from source desktop")
			}

			client := vmctl.NewClient(rt.cfg.VmctlURL)
			resolved, err := client.ForkDesktop(ownerID, sourceDesktopID, targetDesktopID)
			if err != nil {
				return "", err
			}

			sourceState, err := rt.store.GetDesktopStateForDesktop(ctx, ownerID, sourceDesktopID)
			if err != nil {
				return "", fmt.Errorf("fork_desktop load source state: %w", err)
			}
			clonedState := cloneDesktopState(sourceState)
			clonedState.OwnerID = ownerID
			clonedState.DesktopID = resolved.DesktopID
			clonedState.UpdatedAt = time.Now().UTC()
			if err := rt.store.SaveDesktopStateForDesktop(ctx, clonedState); err != nil {
				return "", fmt.Errorf("fork_desktop save cloned state: %w", err)
			}

			return toolResultJSON(map[string]any{
				"status":              "forked_background",
				"desktop_id":          resolved.DesktopID,
				"parent_desktop_id":   sourceDesktopID,
				"parent_vm_id":        resolved.ParentVMID,
				"snapshot_kind":       resolved.SnapshotKind,
				"published":           resolved.Published,
				"availability":        "background_only",
				"copied_window_count": len(clonedState.Windows),
			})
		},
	}
}

func newPublishDesktopTool(rt *Runtime) Tool {
	type args struct {
		DesktopID string `json:"desktop_id"`
	}
	return Tool{
		Name:        "publish_desktop",
		Description: "Publish a prepared candidate desktop so it becomes user-switchable.",
		Parameters:  jsonSchemaObject(map[string]any{"desktop_id": map[string]any{"type": "string"}}, []string{"desktop_id"}, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var in args
			if err := json.Unmarshal(raw, &in); err != nil {
				return "", fmt.Errorf("decode publish_desktop args: %w", err)
			}
			if rt == nil {
				return "", fmt.Errorf("publish_desktop missing runtime")
			}
			if strings.TrimSpace(rt.cfg.VmctlURL) == "" {
				return "", fmt.Errorf("publish_desktop requires runtime vmctl configuration")
			}
			ownerID := stringFromToolContext(ctx, toolCtxOwnerID)
			if ownerID == "" {
				return "", fmt.Errorf("publish_desktop missing owner context")
			}
			desktopID := strings.TrimSpace(in.DesktopID)
			if desktopID == "" {
				return "", fmt.Errorf("publish_desktop requires desktop_id")
			}

			client := vmctl.NewClient(rt.cfg.VmctlURL)
			resolved, err := client.PublishDesktop(ownerID, desktopID)
			if err != nil {
				return "", err
			}
			return toolResultJSON(map[string]any{
				"status":            "published",
				"desktop_id":        resolved.DesktopID,
				"parent_desktop_id": resolved.ParentDesktopID,
				"published":         resolved.Published,
				"desktop_url":       "/?desktop_id=" + resolved.DesktopID,
			})
		},
	}
}

func newRequestWorkerVMTool(rt *Runtime) Tool {
	type args struct {
		Purpose       string `json:"purpose"`
		MachineClass  string `json:"machine_class,omitempty"`
		AllowParallel bool   `json:"allow_parallel,omitempty"`
	}
	return Tool{
		Name:        "request_worker_vm",
		Description: "Request a headless worker VM under the current desktop and return a typed worker handle. This only leases the worker; after a successful result, call start_worker_delegation with start_args plus the full execution objective. Supported machine classes are worker-small, worker-medium, worker-large, and worker-playwright. Use worker-playwright only for high-fidelity browser evidence such as screenshots/video; omit machine_class for worker-small. A different requested machine_class receives a distinct lease instead of reusing the current run's ordinary worker.",
		Parameters: jsonSchemaObject(map[string]any{
			"purpose":        map[string]any{"type": "string"},
			"machine_class":  map[string]any{"type": "string", "enum": []string{"worker-small", "worker-medium", "worker-large", "worker-playwright"}},
			"allow_parallel": map[string]any{"type": "boolean"},
		}, []string{"purpose"}, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			var in args
			if err := json.Unmarshal(raw, &in); err != nil {
				return "", fmt.Errorf("decode request_worker_vm args: %w", err)
			}
			if rt == nil {
				return "", fmt.Errorf("request_worker_vm missing runtime")
			}
			if strings.TrimSpace(rt.cfg.VmctlURL) == "" {
				return "", fmt.Errorf("request_worker_vm requires runtime vmctl configuration")
			}

			ownerID := stringFromToolContext(ctx, toolCtxOwnerID)
			desktopID := strings.TrimSpace(stringFromToolContext(ctx, toolCtxDesktopID))
			parentAgentID := stringFromToolContext(ctx, toolCtxAgentID)
			if ownerID == "" {
				return "", fmt.Errorf("request_worker_vm missing owner context")
			}
			if desktopID == "" {
				desktopID = types.PrimaryDesktopID
			}
			if parentAgentID == "" {
				return "", fmt.Errorf("request_worker_vm missing parent agent context")
			}
			parentRunID := stringFromToolContext(ctx, toolCtxRunID)

			var trajectoryID string
			if runRec, _ := ctx.Value(toolCtxRunRecord).(*types.RunRecord); runRec != nil && runRec.Metadata != nil {
				if id, _ := runRec.Metadata[runMetadataTrajectoryID].(string); strings.TrimSpace(id) != "" {
					trajectoryID = strings.TrimSpace(id)
				}
			}

			client := vmctl.NewClient(rt.cfg.VmctlURL)
			requestedMachineClass := strings.TrimSpace(in.MachineClass)
			machineClass := normalizeRuntimeWorkerMachineClass(requestedMachineClass)
			cacheKey := workerVMRequestCacheKey(ownerID, desktopID, parentAgentID, parentRunID, machineClass)
			forceFreshWorker := false
			if !in.AllowParallel && cacheKey != "" {
				rt.workerRequestMu.Lock()
				if cached := strings.TrimSpace(rt.workerRequests[cacheKey]); cached != "" {
					invalidated, err := rt.workerVMRequestInvalidatedByRunEvents(ctx, parentRunID, cached)
					if err != nil {
						rt.workerRequestMu.Unlock()
						return "", err
					}
					if !invalidated {
						rt.workerRequestMu.Unlock()
						return markToolResultDeduped(cached, "super_run_already_requested_worker_vm")
					}
					delete(rt.workerRequests, cacheKey)
					forceFreshWorker = true
				}
				if cached, ok, invalidated, err := rt.findExistingWorkerVMRequest(ctx, parentRunID, machineClass); err != nil {
					rt.workerRequestMu.Unlock()
					return "", err
				} else if ok {
					rt.workerRequests[cacheKey] = cached
					rt.workerRequestMu.Unlock()
					return markToolResultDeduped(cached, "super_run_already_requested_worker_vm")
				} else if invalidated {
					forceFreshWorker = true
				}
				handle, err := client.RequestWorker(vmctl.WorkerRequest{
					UserID:        ownerID,
					DesktopID:     desktopID,
					ParentAgentID: parentAgentID,
					TrajectoryID:  trajectoryID,
					Purpose:       strings.TrimSpace(in.Purpose),
					MachineClass:  machineClass,
					AllowParallel: forceFreshWorker,
				})
				if err != nil {
					rt.workerRequestMu.Unlock()
					return "", err
				}
				result := workerVMRequestResult(handle)
				if forceFreshWorker {
					result["replaced_unreachable_worker_request"] = true
				}
				if requestedMachineClass != "" && requestedMachineClass != machineClass {
					result["machine_class_normalized_from"] = requestedMachineClass
					result["machine_class"] = handle.MachineClass
				}
				out, err := toolResultJSON(result)
				if err == nil {
					rt.workerRequests[cacheKey] = out
				}
				rt.workerRequestMu.Unlock()
				return out, err
			}
			handle, err := client.RequestWorker(vmctl.WorkerRequest{
				UserID:        ownerID,
				DesktopID:     desktopID,
				ParentAgentID: parentAgentID,
				TrajectoryID:  trajectoryID,
				Purpose:       strings.TrimSpace(in.Purpose),
				MachineClass:  machineClass,
				AllowParallel: in.AllowParallel,
			})
			if err != nil {
				return "", err
			}

			result := workerVMRequestResult(handle)
			if requestedMachineClass != "" && requestedMachineClass != machineClass {
				result["machine_class_normalized_from"] = requestedMachineClass
				result["machine_class"] = handle.MachineClass
			}
			return toolResultJSON(result)
		},
	}
}

func workerVMRequestResult(handle *vmctl.WorkerVMHandle) map[string]any {
	result := map[string]any{
		"status":              "worker_requested",
		"handle":              handle,
		"delegation_required": false,
		"next_tool":           "start_worker_delegation",
		"next_instruction":    "Call start_worker_delegation next with start_args plus the full execution objective. It starts the worker run and returns immediately; use observe_worker_delegation for checkpoints and finish_worker_delegation for terminal evidence.",
	}
	if handle != nil {
		result["start_args"] = map[string]any{
			"worker_sandbox_url": handle.SandboxURL,
			"worker_id":          handle.WorkerID,
			"vm_id":              handle.VMID,
			"profile":            AgentProfileVSuper,
			"timeout_seconds":    int(defaultDelegateWorkerVMTimeout.Seconds()),
		}
		result["next_required_args"] = result["start_args"]
	}
	return result
}

func workerVMRequestCacheKey(ownerID, desktopID, parentAgentID, parentRunID, machineClass string) string {
	ownerID = strings.TrimSpace(ownerID)
	desktopID = strings.TrimSpace(desktopID)
	parentAgentID = strings.TrimSpace(parentAgentID)
	parentRunID = strings.TrimSpace(parentRunID)
	machineClass = normalizeRuntimeWorkerMachineClass(machineClass)
	if ownerID == "" || desktopID == "" || parentAgentID == "" || parentRunID == "" {
		return ""
	}
	return ownerID + "\x00" + desktopID + "\x00" + parentAgentID + "\x00" + parentRunID + "\x00" + machineClass
}

func workerVMRequestCachePrefix(ownerID, desktopID, parentAgentID, parentRunID string) string {
	ownerID = strings.TrimSpace(ownerID)
	desktopID = strings.TrimSpace(desktopID)
	parentAgentID = strings.TrimSpace(parentAgentID)
	parentRunID = strings.TrimSpace(parentRunID)
	if ownerID == "" || desktopID == "" || parentAgentID == "" || parentRunID == "" {
		return ""
	}
	return ownerID + "\x00" + desktopID + "\x00" + parentAgentID + "\x00" + parentRunID + "\x00"
}

func (rt *Runtime) findExistingWorkerVMRequest(ctx context.Context, runID, machineClass string) (string, bool, bool, error) {
	if rt == nil || rt.store == nil || strings.TrimSpace(runID) == "" {
		return "", false, false, nil
	}
	machineClass = normalizeRuntimeWorkerMachineClass(machineClass)
	eventsForRun, err := rt.listWorkerToolEventsForCurrentScope(ctx, runID, 500)
	if err != nil {
		return "", false, false, fmt.Errorf("request_worker_vm dedupe scan: %w", err)
	}
	var candidate string
	var candidateKey workerVMLeaseKey
	var invalidated []workerVMLeaseKey
	invalidatedAny := false
	for _, ev := range eventsForRun {
		if ev.Kind != types.EventToolResult {
			continue
		}
		var payload struct {
			Tool    string `json:"tool"`
			IsError bool   `json:"is_error"`
			Output  string `json:"output"`
		}
		if err := json.Unmarshal(ev.Payload, &payload); err != nil || payload.IsError || payload.Tool != "request_worker_vm" {
			if err == nil && !payload.IsError && payload.Tool == "delegate_worker_vm" {
				if output, ok := decodeWorkerToolOutput(payload.Output); ok && shouldInvalidateWorkerVMRequestFromDelegateResult(output) {
					key := workerVMLeaseKeyFromDelegateOutput(output)
					if key.Valid() {
						invalidated = append(invalidated, key)
						invalidatedAny = true
						if candidate != "" && key.Matches(candidateKey) {
							candidate = ""
							candidateKey = workerVMLeaseKey{}
						}
					}
				}
			}
			continue
		}
		output, ok := decodeWorkerToolOutput(payload.Output)
		if !ok {
			continue
		}
		status, _ := output["status"].(string)
		if strings.TrimSpace(status) == "worker_requested" && output["handle"] != nil {
			if requested := workerMachineClassFromRequestOutput(output); machineClass != "" && requested != "" && requested != machineClass {
				continue
			}
			key := workerVMLeaseKeyFromRequestOutput(output)
			if workerVMLeaseKeyInvalidated(key, invalidated) {
				invalidatedAny = true
				continue
			}
			candidate = payload.Output
			candidateKey = key
		}
	}
	if candidate != "" {
		return candidate, true, invalidatedAny, nil
	}
	return "", false, invalidatedAny, nil
}

func (rt *Runtime) workerVMRequestInvalidatedByRunEvents(ctx context.Context, runID, raw string) (bool, error) {
	output, ok := decodeWorkerToolOutput(raw)
	if !ok {
		return false, nil
	}
	key := workerVMLeaseKeyFromRequestOutput(output)
	if !key.Valid() || rt == nil || rt.store == nil || strings.TrimSpace(runID) == "" {
		return false, nil
	}
	eventsForRun, err := rt.listWorkerToolEventsForCurrentScope(ctx, runID, 500)
	if err != nil {
		return false, fmt.Errorf("request_worker_vm dedupe invalidation scan: %w", err)
	}
	for _, ev := range eventsForRun {
		if ev.Kind != types.EventToolResult {
			continue
		}
		var payload struct {
			Tool    string `json:"tool"`
			IsError bool   `json:"is_error"`
			Output  string `json:"output"`
		}
		if err := json.Unmarshal(ev.Payload, &payload); err != nil || payload.IsError || (payload.Tool != "delegate_worker_vm" && payload.Tool != "start_worker_delegation") {
			continue
		}
		output, ok := decodeWorkerToolOutput(payload.Output)
		if !ok || !shouldInvalidateWorkerVMRequestFromDelegateResult(output) {
			continue
		}
		if workerVMLeaseKeyFromDelegateOutput(output).Matches(key) {
			return true, nil
		}
	}
	return false, nil
}

func (rt *Runtime) findExistingWorkerVMDelegation(ctx context.Context, runID string, in delegateWorkerVMArgs, profile string) (string, bool, error) {
	if rt == nil || rt.store == nil || strings.TrimSpace(runID) == "" {
		return "", false, nil
	}
	key := workerVMLeaseKey{
		WorkerID:   strings.TrimSpace(in.WorkerID),
		VMID:       strings.TrimSpace(in.VMID),
		SandboxURL: strings.TrimSpace(in.WorkerSandboxURL),
	}
	if !key.Valid() {
		return "", false, nil
	}
	eventsForRun, err := rt.listWorkerToolEventsForCurrentScope(ctx, runID, 1000)
	if err != nil {
		return "", false, fmt.Errorf("delegate_worker_vm dedupe scan: %w", err)
	}
	profile = canonicalAgentProfile(profile)
	var candidate string
	for _, ev := range eventsForRun {
		if ev.Kind != types.EventToolResult {
			continue
		}
		var payload struct {
			Tool    string `json:"tool"`
			IsError bool   `json:"is_error"`
			Output  string `json:"output"`
		}
		if err := json.Unmarshal(ev.Payload, &payload); err != nil || payload.IsError || (payload.Tool != "delegate_worker_vm" && payload.Tool != "start_worker_delegation") {
			continue
		}
		output, ok := decodeWorkerToolOutput(payload.Output)
		if !ok {
			continue
		}
		if !workerVMLeaseKeyFromDelegateOutput(output).Matches(key) {
			continue
		}
		if outputProfile := canonicalAgentProfile(stringMapValue(output, "profile")); profile != "" && outputProfile != "" && outputProfile != profile {
			continue
		}
		if !delegateWorkerVMResultReusable(output) {
			continue
		}
		candidate = payload.Output
	}
	return candidate, candidate != "", nil
}

func (rt *Runtime) listWorkerToolEventsForCurrentScope(ctx context.Context, runID string, limit int) ([]types.EventRecord, error) {
	if rt == nil || rt.store == nil || strings.TrimSpace(runID) == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 500
	}
	if rec := ctxRunRecord(ctx); rec != nil {
		ownerID := strings.TrimSpace(rec.OwnerID)
		trajectoryID := metadataStringValue(rec.Metadata, runMetadataTrajectoryID)
		if ownerID != "" && trajectoryID != "" {
			return rt.store.ListEventsByTrajectory(ctx, ownerID, trajectoryID, limit)
		}
	}
	return rt.store.ListEvents(ctx, runID, limit)
}

func delegateWorkerVMResultReusable(output map[string]any) bool {
	status := stringMapValue(output, "status")
	switch status {
	case "worker_run_started",
		"worker_observed",
		"worker_run_active":
		return true
	case "worker_run_completed",
		"worker_run_incomplete",
		"worker_run_failed",
		"worker_run_cancelled",
		"worker_run_blocked",
		"worker_run_timeout":
		return true
	default:
		return false
	}
}

func markToolResultDeduped(raw, reason string) (string, error) {
	var output map[string]any
	if err := json.Unmarshal([]byte(raw), &output); err != nil {
		return "", err
	}
	output["deduped"] = true
	output["dedupe_reason"] = reason
	return toolResultJSON(output)
}

type workerVMLeaseKey struct {
	WorkerID   string
	VMID       string
	SandboxURL string
}

func (k workerVMLeaseKey) Valid() bool {
	return k.WorkerID != "" || k.VMID != "" || k.SandboxURL != ""
}

func (k workerVMLeaseKey) Matches(other workerVMLeaseKey) bool {
	if !k.Valid() || !other.Valid() {
		return false
	}
	if k.WorkerID != "" && other.WorkerID != "" {
		return k.WorkerID == other.WorkerID
	}
	if k.VMID != "" && other.VMID != "" {
		return k.VMID == other.VMID
	}
	return k.SandboxURL != "" && other.SandboxURL != "" && k.SandboxURL == other.SandboxURL
}

func workerVMLeaseKeyInvalidated(key workerVMLeaseKey, invalidated []workerVMLeaseKey) bool {
	for _, stale := range invalidated {
		if stale.Matches(key) {
			return true
		}
	}
	return false
}

func decodeWorkerToolOutput(raw string) (map[string]any, bool) {
	var output map[string]any
	if err := json.Unmarshal([]byte(raw), &output); err != nil {
		return nil, false
	}
	return output, true
}

func workerVMLeaseKeyFromRequestOutput(output map[string]any) workerVMLeaseKey {
	handle, _ := output["handle"].(map[string]any)
	return workerVMLeaseKey{
		WorkerID:   stringMapValue(handle, "worker_id"),
		VMID:       stringMapValue(handle, "vm_id"),
		SandboxURL: stringMapValue(handle, "sandbox_url"),
	}
}

func workerMachineClassFromRequestOutput(output map[string]any) string {
	handle, _ := output["handle"].(map[string]any)
	return normalizeRuntimeWorkerMachineClass(firstNonEmpty(
		stringMapValue(handle, "machine_class"),
		stringMapValue(output, "machine_class"),
	))
}

func workerVMLeaseKeyFromDelegateOutput(output map[string]any) workerVMLeaseKey {
	return workerVMLeaseKey{
		WorkerID:   stringMapValue(output, "worker_id"),
		VMID:       firstNonEmpty(stringMapValue(output, "worker_vm_id"), stringMapValue(output, "vm_id")),
		SandboxURL: stringMapValue(output, "worker_sandbox_url"),
	}
}

func shouldInvalidateWorkerVMRequestFromDelegateResult(output map[string]any) bool {
	status := stringMapValue(output, "status")
	if status != "worker_run_submit_failed" && status != "worker_run_status_failed" {
		return false
	}
	text := strings.ToLower(strings.Join([]string{
		stringMapValue(output, "error"),
		stringMapValue(output, "terminal_error"),
		stringMapValue(output, "worker_event_error"),
	}, "\n"))
	for _, marker := range []string{
		"no route to host",
		"network is unreachable",
		"connection refused",
		"connection reset by peer",
		"connect: cannot assign requested address",
		"connect: can't assign requested address",
		"no such host",
		"i/o timeout",
	} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func (rt *Runtime) invalidateWorkerVMRequestCacheForDelegateResult(ctx context.Context, result map[string]any) map[string]any {
	if rt == nil || !shouldInvalidateWorkerVMRequestFromDelegateResult(result) {
		return result
	}
	ownerID := stringFromToolContext(ctx, toolCtxOwnerID)
	desktopID := strings.TrimSpace(stringFromToolContext(ctx, toolCtxDesktopID))
	if desktopID == "" {
		desktopID = types.PrimaryDesktopID
	}
	parentAgentID := stringFromToolContext(ctx, toolCtxAgentID)
	parentRunID := stringFromToolContext(ctx, toolCtxRunID)
	cachePrefix := workerVMRequestCachePrefix(ownerID, desktopID, parentAgentID, parentRunID)
	if cachePrefix == "" {
		return result
	}
	rt.workerRequestMu.Lock()
	invalidated := false
	for key := range rt.workerRequests {
		if strings.HasPrefix(key, cachePrefix) {
			delete(rt.workerRequests, key)
			invalidated = true
		}
	}
	if invalidated {
		result["worker_request_cache_invalidated"] = true
		result["worker_request_cache_invalidation_reason"] = "worker_runtime_unreachable"
	}
	rt.workerRequestMu.Unlock()
	return result
}

func normalizeRuntimeWorkerMachineClass(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "default":
		return ""
	case "standard", "worker", "worker-standard", "worker-default":
		return "worker-small"
	case "small":
		return "worker-small"
	case "medium":
		return "worker-medium"
	case "large":
		return "worker-large"
	case "playwright", "evidence", "evidence-browser", "verifier-browser":
		return "worker-playwright"
	default:
		return strings.TrimSpace(raw)
	}
}

func newStartWorkerDelegationTool(rt *Runtime, cwd string) Tool {
	return newStartWorkerDelegationToolNamed(
		"start_worker_delegation",
		"Start a vsuper, co-super, or researcher run inside a leased worker VM and return immediately with an async worker run handle. Use observe_worker_delegation for checkpoints and finish_worker_delegation for terminal evidence.",
		rt,
		cwd,
	)
}

func newDelegateWorkerVMTool(rt *Runtime, cwd string) Tool {
	return newStartWorkerDelegationToolNamed(
		"delegate_worker_vm",
		"Deprecated compatibility alias for start_worker_delegation. It no longer waits for terminal completion; it starts the worker run and returns immediately.",
		rt,
		cwd,
	)
}

func newStartWorkerDelegationToolNamed(name, description string, rt *Runtime, cwd string) Tool {
	return Tool{
		Name:        name,
		Description: description,
		Parameters: jsonSchemaObject(map[string]any{
			"worker_sandbox_url": map[string]any{"type": "string"},
			"worker_id":          map[string]any{"type": "string"},
			"vm_id":              map[string]any{"type": "string"},
			"objective":          map[string]any{"type": "string"},
			"profile":            map[string]any{"type": "string", "enum": []string{AgentProfileVSuper, AgentProfileCoSuper, AgentProfileResearcher}},
			"timeout_seconds":    map[string]any{"type": "integer"},
		}, []string{"worker_sandbox_url", "objective"}, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			return rt.startWorkerDelegation(ctx, cwd, raw)
		},
	}
}

func (rt *Runtime) startWorkerDelegation(ctx context.Context, cwd string, raw json.RawMessage) (string, error) {
	if profile := stringFromToolContext(ctx, toolCtxProfile); profile != AgentProfileSuper {
		return "", fmt.Errorf("start_worker_delegation is only available to super agents")
	}
	if rt == nil {
		return "", fmt.Errorf("start_worker_delegation missing runtime")
	}
	var in delegateWorkerVMArgs
	if err := json.Unmarshal(raw, &in); err != nil {
		return "", fmt.Errorf("decode start_worker_delegation args: %w", err)
	}
	ownerID := stringFromToolContext(ctx, toolCtxOwnerID)
	if ownerID == "" {
		return "", fmt.Errorf("start_worker_delegation missing owner context")
	}
	objective := strings.TrimSpace(in.Objective)
	if objective == "" {
		return "", fmt.Errorf("objective must not be empty")
	}
	delegatedObjective := objective
	profile := canonicalAgentProfile(in.Profile)
	if profile == "" {
		profile = AgentProfileVSuper
	}
	if profile != AgentProfileVSuper && profile != AgentProfileCoSuper && profile != AgentProfileResearcher {
		return "", fmt.Errorf("profile must be %s, %s, or %s", AgentProfileVSuper, AgentProfileCoSuper, AgentProfileResearcher)
	}
	timeout := time.Duration(in.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = defaultDelegateWorkerVMTimeout
	}
	if timeout > maxDelegateWorkerVMTimeout {
		timeout = maxDelegateWorkerVMTimeout
	}
	client := &http.Client{Timeout: 30 * time.Second}

	runID := stringFromToolContext(ctx, toolCtxRunID)
	agentID := stringFromToolContext(ctx, toolCtxAgentID)
	if cached, ok, err := rt.findExistingWorkerVMDelegation(ctx, runID, in, profile); err != nil {
		return "", err
	} else if ok {
		return markToolResultDeduped(cached, "super_run_already_started_worker_delegation")
	}
	trajectoryID := ""
	if rec := ctxRunRecord(ctx); rec != nil && rec.Metadata != nil {
		trajectoryID = metadataStringValue(rec.Metadata, runMetadataTrajectoryID)
	}
	metadata := map[string]any{
		runMetadataAgentProfile: profile,
		runMetadataAgentRole:    profile,
		"request_source":        "worker_vm_delegation",
		"delegated_by_run_id":   runID,
		"delegated_by_agent_id": agentID,
		"delegated_by_profile":  AgentProfileSuper,
		"worker_id":             strings.TrimSpace(in.WorkerID),
		"worker_vm_id":          strings.TrimSpace(in.VMID),
		"parent_sandbox_id":     stringFromToolContext(ctx, toolCtxSandboxID),
	}
	if rec := ctxRunRecord(ctx); rec != nil && rec.Metadata != nil {
		for _, key := range []string{"requested_by_profile", "requested_by_agent_id", "requested_by_run_id", runMetadataDesktopID} {
			if value := metadataStringValue(rec.Metadata, key); value != "" {
				metadata[key] = value
			}
		}
	}
	if channelID := stringFromToolContext(ctx, toolCtxChannelID); channelID != "" {
		metadata[runMetadataChannelID] = channelID
	}
	if desktopID := stringFromToolContext(ctx, toolCtxDesktopID); desktopID != "" {
		metadata[runMetadataDesktopID] = desktopID
	}
	if trajectoryID != "" {
		metadata[runMetadataTrajectoryID] = trajectoryID
	}

	isolation, err := prepareSameRuntimeWorkerIsolation(ctx, cwd, in.WorkerSandboxURL, in.WorkerID, in.VMID, runID)
	if err != nil {
		return "", err
	}
	if isolation.Enabled {
		metadata[runMetadataToolCWD] = isolation.WorktreePath
		metadata[runMetadataWorkerIsolation] = isolation.Kind
		metadata[runMetadataWorkerBaseSHA] = isolation.BaseSHA
		metadata[runMetadataWorkerBranch] = isolation.Branch
		metadata[runMetadataWorkerWorktree] = isolation.WorktreePath
		objective = isolation.WorkerPrompt + "\n\n" + delegatedObjective
	} else if bootstrap, err := prepareRemoteWorkerRepoBootstrap(ctx, cwd, in.WorkerSandboxURL, profile); err != nil {
		return "", err
	} else if bootstrap.Enabled {
		metadata[runMetadataWorkerRepoBootstrap] = bootstrap.Kind
		metadata[runMetadataWorkerRepoRemote] = bootstrap.RemoteURL
		metadata[runMetadataWorkerRepoBaseSHA] = bootstrap.BaseSHA
		objective = bootstrap.WorkerPrompt + "\n\n" + delegatedObjective
	}
	if profile == AgentProfileVSuper {
		objective = workerVSuperDelegateContract(timeout) + "\n\n" + objective
	}

	startResp, err := submitInternalWorkerRun(ctx, client, in.WorkerSandboxURL, internalRunSubmitRequest{
		OwnerID:  ownerID,
		Prompt:   objective,
		Metadata: metadata,
	})
	if err != nil {
		result := map[string]any{
			"status":              "worker_run_submit_failed",
			"worker_id":           strings.TrimSpace(in.WorkerID),
			"worker_vm_id":        strings.TrimSpace(in.VMID),
			"worker_sandbox_url":  strings.TrimSpace(in.WorkerSandboxURL),
			"app_change_packages": []map[string]any{},
			"event_count":         0,
			"error":               err.Error(),
			"terminal_error":      err.Error(),
		}
		result = rt.invalidateWorkerVMRequestCacheForDelegateResult(ctx, result)
		return toolResultJSON(result)
	}
	result := map[string]any{
		"status":              "worker_run_started",
		"worker_id":           strings.TrimSpace(in.WorkerID),
		"worker_vm_id":        strings.TrimSpace(in.VMID),
		"worker_sandbox_url":  strings.TrimSpace(in.WorkerSandboxURL),
		"loop_id":             startResp.RunID,
		"worker_run_id":       startResp.RunID,
		"agent_id":            startResp.AgentID,
		"channel_id":          startResp.ChannelID,
		"profile":             firstNonEmpty(startResp.AgentProfile, profile),
		"state":               startResp.State,
		"result":              startResp.Result,
		"error":               startResp.Error,
		"timeout_seconds":     int(timeout.Seconds()),
		"app_change_packages": []map[string]any{},
		"next_tools":          []string{"observe_worker_delegation", "finish_worker_delegation", "cancel_worker_delegation"},
	}
	if isolation.Enabled {
		result["worker_isolation"] = isolation.Kind
		result["worker_worktree_path"] = isolation.WorktreePath
		result["worker_branch"] = isolation.Branch
		result["worker_base_sha"] = isolation.BaseSHA
	}
	if bootstrap := metadataStringValue(metadata, runMetadataWorkerRepoBootstrap); bootstrap != "" {
		result["worker_repo_bootstrap"] = bootstrap
		result["worker_repo_remote_url"] = metadataStringValue(metadata, runMetadataWorkerRepoRemote)
		result["worker_repo_base_sha"] = metadataStringValue(metadata, runMetadataWorkerRepoBaseSHA)
	}
	return toolResultJSON(result)
}

func newObserveWorkerDelegationTool(rt *Runtime) Tool {
	type args struct {
		WorkerSandboxURL string `json:"worker_sandbox_url"`
		WorkerID         string `json:"worker_id,omitempty"`
		VMID             string `json:"vm_id,omitempty"`
		WorkerRunID      string `json:"worker_run_id,omitempty"`
		LoopID           string `json:"loop_id,omitempty"`
		Profile          string `json:"profile,omitempty"`
	}
	return Tool{
		Name:        "observe_worker_delegation",
		Description: "Read bounded state and evidence for an async worker delegation without waiting for terminal completion.",
		Parameters: jsonSchemaObject(map[string]any{
			"worker_sandbox_url": map[string]any{"type": "string"},
			"worker_id":          map[string]any{"type": "string"},
			"vm_id":              map[string]any{"type": "string"},
			"worker_run_id":      map[string]any{"type": "string"},
			"loop_id":            map[string]any{"type": "string"},
			"profile":            map[string]any{"type": "string", "enum": []string{AgentProfileVSuper, AgentProfileCoSuper, AgentProfileResearcher}},
		}, []string{"worker_sandbox_url"}, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			if profile := stringFromToolContext(ctx, toolCtxProfile); profile != AgentProfileSuper {
				return "", fmt.Errorf("observe_worker_delegation is only available to super agents")
			}
			if rt == nil {
				return "", fmt.Errorf("observe_worker_delegation missing runtime")
			}
			var in args
			if err := json.Unmarshal(raw, &in); err != nil {
				return "", fmt.Errorf("decode observe_worker_delegation args: %w", err)
			}
			ownerID := stringFromToolContext(ctx, toolCtxOwnerID)
			if ownerID == "" {
				return "", fmt.Errorf("observe_worker_delegation missing owner context")
			}
			workerRunID := firstNonEmpty(strings.TrimSpace(in.WorkerRunID), strings.TrimSpace(in.LoopID))
			if workerRunID == "" {
				return "", fmt.Errorf("worker_run_id or loop_id is required")
			}
			client := &http.Client{Timeout: 10 * time.Second}
			status, statusErr := fetchInternalWorkerRunStatus(ctx, client, in.WorkerSandboxURL, ownerID, workerRunID)
			result := map[string]any{
				"status":              "worker_observed",
				"worker_id":           strings.TrimSpace(in.WorkerID),
				"worker_vm_id":        strings.TrimSpace(in.VMID),
				"worker_sandbox_url":  strings.TrimSpace(in.WorkerSandboxURL),
				"loop_id":             workerRunID,
				"worker_run_id":       workerRunID,
				"app_change_packages": []map[string]any{},
			}
			if profile := canonicalAgentProfile(in.Profile); profile != "" {
				result["profile"] = profile
			}
			if statusErr != nil {
				result["status"] = "worker_observe_status_failed"
				result["error"] = statusErr.Error()
				return toolResultJSON(result)
			}
			result["agent_id"] = status.AgentID
			result["profile"] = firstNonEmpty(status.AgentProfile, stringMapValue(result, "profile"))
			result["state"] = status.State
			result["result"] = status.Result
			result["error"] = status.Error
			rt.applyAsyncWorkerEvidence(ctx, client, in.WorkerSandboxURL, ownerID, workerRunID, status.State.Terminal() || status.State == types.RunBlocked, ctxRunRecord(ctx), result)
			updateCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if intMapValue(result, "mirrored_worker_update_count") > 0 {
				result["worker_update_checkpoint"] = "worker_submit_update_mirrored"
			} else if err := rt.synthesizeDelegateWorkerUpdateCheckpoint(updateCtx, ctxRunRecord(ctx), result, "async_observe"); err != nil {
				result["worker_update_error"] = err.Error()
			} else {
				result["worker_update_checkpoint"] = "submitted_or_existing"
			}
			return toolResultJSON(result)
		},
	}
}

func newFinishWorkerDelegationTool(rt *Runtime) Tool {
	type args struct {
		WorkerSandboxURL string `json:"worker_sandbox_url"`
		WorkerID         string `json:"worker_id,omitempty"`
		VMID             string `json:"vm_id,omitempty"`
		WorkerRunID      string `json:"worker_run_id,omitempty"`
		LoopID           string `json:"loop_id,omitempty"`
		Profile          string `json:"profile,omitempty"`
		Objective        string `json:"objective,omitempty"`
		TimeoutSeconds   int    `json:"timeout_seconds,omitempty"`
	}
	return Tool{
		Name:        "finish_worker_delegation",
		Description: "Collect terminal evidence for an async worker delegation. If the worker is still active, returns active state instead of blocking.",
		Parameters: jsonSchemaObject(map[string]any{
			"worker_sandbox_url": map[string]any{"type": "string"},
			"worker_id":          map[string]any{"type": "string"},
			"vm_id":              map[string]any{"type": "string"},
			"worker_run_id":      map[string]any{"type": "string"},
			"loop_id":            map[string]any{"type": "string"},
			"profile":            map[string]any{"type": "string", "enum": []string{AgentProfileVSuper, AgentProfileCoSuper, AgentProfileResearcher}},
			"objective":          map[string]any{"type": "string"},
			"timeout_seconds":    map[string]any{"type": "integer"},
		}, []string{"worker_sandbox_url"}, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			if profile := stringFromToolContext(ctx, toolCtxProfile); profile != AgentProfileSuper {
				return "", fmt.Errorf("finish_worker_delegation is only available to super agents")
			}
			if rt == nil {
				return "", fmt.Errorf("finish_worker_delegation missing runtime")
			}
			var in args
			if err := json.Unmarshal(raw, &in); err != nil {
				return "", fmt.Errorf("decode finish_worker_delegation args: %w", err)
			}
			ownerID := stringFromToolContext(ctx, toolCtxOwnerID)
			if ownerID == "" {
				return "", fmt.Errorf("finish_worker_delegation missing owner context")
			}
			workerRunID := firstNonEmpty(strings.TrimSpace(in.WorkerRunID), strings.TrimSpace(in.LoopID))
			if workerRunID == "" {
				return "", fmt.Errorf("worker_run_id or loop_id is required")
			}
			client := &http.Client{Timeout: 15 * time.Second}
			status, err := fetchInternalWorkerRunStatus(ctx, client, in.WorkerSandboxURL, ownerID, workerRunID)
			if err != nil {
				return toolResultJSON(map[string]any{
					"status":              "worker_finish_status_failed",
					"worker_id":           strings.TrimSpace(in.WorkerID),
					"worker_vm_id":        strings.TrimSpace(in.VMID),
					"worker_sandbox_url":  strings.TrimSpace(in.WorkerSandboxURL),
					"loop_id":             workerRunID,
					"worker_run_id":       workerRunID,
					"app_change_packages": []map[string]any{},
					"error":               err.Error(),
					"terminal_error":      err.Error(),
				})
			}
			result := map[string]any{
				"status":              delegateWorkerRunStatus(status.State),
				"worker_id":           firstNonEmpty(strings.TrimSpace(in.WorkerID), metadataStringValue(status.Metadata, "worker_id")),
				"worker_vm_id":        firstNonEmpty(strings.TrimSpace(in.VMID), metadataStringValue(status.Metadata, "worker_vm_id")),
				"worker_sandbox_url":  strings.TrimSpace(in.WorkerSandboxURL),
				"loop_id":             workerRunID,
				"worker_run_id":       workerRunID,
				"agent_id":            status.AgentID,
				"profile":             firstNonEmpty(status.AgentProfile, canonicalAgentProfile(in.Profile)),
				"state":               status.State,
				"result":              status.Result,
				"error":               status.Error,
				"app_change_packages": []map[string]any{},
			}
			if isolation := metadataStringValue(status.Metadata, runMetadataWorkerIsolation); isolation != "" {
				result["worker_isolation"] = isolation
				result["worker_worktree_path"] = metadataStringValue(status.Metadata, runMetadataWorkerWorktree)
				result["worker_branch"] = metadataStringValue(status.Metadata, runMetadataWorkerBranch)
				result["worker_base_sha"] = metadataStringValue(status.Metadata, runMetadataWorkerBaseSHA)
			}
			if bootstrap := metadataStringValue(status.Metadata, runMetadataWorkerRepoBootstrap); bootstrap != "" {
				result["worker_repo_bootstrap"] = bootstrap
				result["worker_repo_remote_url"] = metadataStringValue(status.Metadata, runMetadataWorkerRepoRemote)
				result["worker_repo_base_sha"] = metadataStringValue(status.Metadata, runMetadataWorkerRepoBaseSHA)
			}
			if !status.State.Terminal() && status.State != types.RunBlocked {
				result["status"] = "worker_run_active"
				result["finish_ready"] = false
				rt.applyAsyncWorkerEvidence(ctx, client, in.WorkerSandboxURL, ownerID, workerRunID, false, ctxRunRecord(ctx), result)
				updateCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if intMapValue(result, "mirrored_worker_update_count") > 0 {
					result["worker_update_checkpoint"] = "worker_submit_update_mirrored"
				} else if !shouldCheckpointActiveWorkerFinish(result) {
					result["worker_update_checkpoint"] = "active_evidence_deferred"
				} else if err := rt.synthesizeDelegateWorkerUpdateCheckpoint(updateCtx, ctxRunRecord(ctx), result, "async_finish_active"); err != nil {
					result["worker_update_error"] = err.Error()
				} else {
					result["worker_update_checkpoint"] = "submitted_or_existing"
				}
				return toolResultJSON(result)
			}
			evidence, evidenceErr := fetchWorkerRunEvidence(ctx, client, in.WorkerSandboxURL, ownerID, workerRunID)
			if evidenceErr != nil {
				result["worker_event_error"] = evidenceErr.Error()
			} else {
				profile := firstNonEmpty(stringMapValue(result, "profile"), canonicalAgentProfile(in.Profile))
				timeout := time.Duration(in.TimeoutSeconds) * time.Second
				if timeout <= 0 {
					timeout = defaultDelegateWorkerVMTimeout
				}
				if timeout > maxDelegateWorkerVMTimeout {
					timeout = maxDelegateWorkerVMTimeout
				}
				if profile == AgentProfileVSuper && status.State == types.RunCompleted {
					evidence = followWorkerChildRuns(ctx, client, in.WorkerSandboxURL, ownerID, workerRunID, evidence, timeout)
				}
				rt.applyWorkerEvidenceToResult(ctx, client, in.WorkerSandboxURL, ownerID, evidence, true, ctxRunRecord(ctx), result)
			}
			if status.State != types.RunCompleted {
				result["terminal_error"] = strings.TrimSpace(fmt.Sprintf("worker run %s ended in state %s: %s", workerRunID, status.State, strings.TrimSpace(status.Error)))
			}
			if status.State == types.RunCompleted && delegateRequiresAppChangePackage(stringMapValue(result, "profile"), in.Objective) && len(result["app_change_packages"].([]map[string]any)) == 0 {
				result["status"] = "worker_run_incomplete"
				result["completion_blocker"] = "vsuper_completed_without_required_app_change_package"
				result["terminal_error"] = "worker vsuper completed a package-required objective without publish_app_change_package evidence"
			}
			updateCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if intMapValue(result, "mirrored_worker_update_count") > 0 {
				result["worker_update_checkpoint"] = "worker_submit_update_mirrored"
			} else if err := rt.synthesizeDelegateWorkerUpdateCheckpoint(updateCtx, ctxRunRecord(ctx), result, "async_finish"); err != nil {
				result["worker_update_error"] = err.Error()
			} else {
				result["worker_update_checkpoint"] = "submitted_or_existing"
			}
			return toolResultJSON(result)
		},
	}
}

func shouldCheckpointActiveWorkerFinish(result map[string]any) bool {
	if result == nil {
		return false
	}
	if stringMapValue(result, "worker_event_error") != "" {
		return true
	}
	if len(mapSliceValue(result, "app_change_packages")) > 0 {
		return true
	}
	if len(stringSliceMapValue(result, "worker_child_run_ids")) > 0 {
		return true
	}
	for _, item := range mapSliceValue(result, "worker_event_summary") {
		if isError, _ := item["is_error"].(bool); isError {
			return true
		}
	}
	return false
}

func newRedirectWorkerDelegationTool(rt *Runtime) Tool {
	type args struct {
		WorkerSandboxURL string `json:"worker_sandbox_url"`
		WorkerRunID      string `json:"worker_run_id,omitempty"`
		LoopID           string `json:"loop_id,omitempty"`
		ChannelID        string `json:"channel_id"`
		TargetAgentID    string `json:"target_agent_id"`
		Message          string `json:"message"`
		MessageClass     string `json:"message_class,omitempty"`
	}
	return Tool{
		Name:        "redirect_worker_delegation",
		Description: "Send a super-authored redirect directive to the worker vsuper's coordination channel without blocking. Direct worker control should target vsuper; vsuper remains responsible for coordinating its co-supers.",
		Parameters: jsonSchemaObject(map[string]any{
			"worker_sandbox_url": map[string]any{"type": "string"},
			"worker_run_id":      map[string]any{"type": "string"},
			"loop_id":            map[string]any{"type": "string"},
			"channel_id":         map[string]any{"type": "string"},
			"target_agent_id":    map[string]any{"type": "string"},
			"message":            map[string]any{"type": "string"},
			"message_class":      map[string]any{"type": "string"},
		}, []string{"worker_sandbox_url", "channel_id", "target_agent_id", "message"}, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			if profile := stringFromToolContext(ctx, toolCtxProfile); profile != AgentProfileSuper {
				return "", fmt.Errorf("redirect_worker_delegation is only available to super agents")
			}
			var in args
			if err := json.Unmarshal(raw, &in); err != nil {
				return "", fmt.Errorf("decode redirect_worker_delegation args: %w", err)
			}
			ownerID := stringFromToolContext(ctx, toolCtxOwnerID)
			if ownerID == "" {
				return "", fmt.Errorf("redirect_worker_delegation missing owner context")
			}
			workerRunID := firstNonEmpty(strings.TrimSpace(in.WorkerRunID), strings.TrimSpace(in.LoopID))
			messageClass := strings.TrimSpace(in.MessageClass)
			if messageClass == "" {
				messageClass = "directive"
			}
			content := fmt.Sprintf("[message_class=%s]\n%s", messageClass, strings.TrimSpace(in.Message))
			client := &http.Client{Timeout: 10 * time.Second}
			cursor, err := postInternalWorkerChannelCast(ctx, client, in.WorkerSandboxURL, internalChannelCastRequest{
				OwnerID:     ownerID,
				ChannelID:   strings.TrimSpace(in.ChannelID),
				ToAgentID:   strings.TrimSpace(in.TargetAgentID),
				ToRunID:     workerRunID,
				FromAgentID: firstNonEmpty(stringFromToolContext(ctx, toolCtxAgentID), AgentProfileSuper),
				FromRunID:   stringFromToolContext(ctx, toolCtxRunID),
				From:        firstNonEmpty(stringFromToolContext(ctx, toolCtxAgentID), AgentProfileSuper),
				Role:        AgentProfileSuper,
				Content:     content,
			})
			if err != nil {
				return "", err
			}
			return toolResultJSON(map[string]any{
				"status":          "worker_redirect_sent",
				"worker_run_id":   workerRunID,
				"loop_id":         workerRunID,
				"target_agent_id": strings.TrimSpace(in.TargetAgentID),
				"channel_id":      strings.TrimSpace(in.ChannelID),
				"cursor":          cursor,
				"message_class":   messageClass,
			})
		},
	}
}

func (rt *Runtime) applyAsyncWorkerEvidence(ctx context.Context, client *http.Client, workerSandboxURL, ownerID, workerRunID string, mirrorTerminalPackages bool, superRec *types.RunRecord, result map[string]any) {
	evidence, evidenceErr := fetchWorkerRunEvidence(ctx, client, workerSandboxURL, ownerID, workerRunID)
	if evidenceErr != nil {
		result["worker_event_error"] = evidenceErr.Error()
		return
	}
	rt.applyWorkerEvidenceToResult(ctx, client, workerSandboxURL, ownerID, evidence, mirrorTerminalPackages, superRec, result)
}

func (rt *Runtime) applyWorkerEvidenceToResult(ctx context.Context, client *http.Client, workerSandboxURL, ownerID string, evidence workerRunEvidence, mirrorTerminalPackages bool, superRec *types.RunRecord, result map[string]any) {
	applyWorkerRunEvidence(result, evidence)
	if summary := summarizeWorkerRunEvents(evidence.Events); len(summary) > 0 {
		result["worker_event_summary"] = summary
	}
	if profiles := collectWorkerSpawnProfiles(evidence.Events); len(profiles) > 0 {
		result["worker_spawned_profiles"] = profiles
	}
	if count := countWorkerChannelMessages(evidence.Events); count > 0 {
		result["worker_channel_message_count"] = count
	}
	rt.mirrorWorkerSubmitUpdates(ctx, superRec, evidence, result)
	packages := collectAppChangePackageResults(evidence.Events)
	if len(packages) > 0 {
		if mirrorTerminalPackages && rt != nil {
			var mirrorErrors []string
			packages, mirrorErrors = rt.mirrorWorkerAppChangePackages(ctx, client, workerSandboxURL, ownerID, packages)
			annotateWorkerPackageMirrorResult(result, packages, mirrorErrors)
		} else {
			result["reviewable_package_observed"] = true
		}
		result["app_change_packages"] = packages
	}
	if mirrorTerminalPackages && rt != nil {
		if stringMapValue(result, "profile") == AgentProfileVSuper && vSuperDelegateIncomplete(evidence, packages) {
			result["status"] = "worker_run_incomplete"
			result["completion_blocker"] = "vsuper_completed_without_app_change_package_or_worker_update"
			result["terminal_error"] = "worker vsuper completed after child coordination without publish_app_change_package or submit_worker_update evidence"
		}
	}
}

type mirroredWorkerSubmitUpdate struct {
	InvokedEvent types.EventRecord
	ResultEvent  types.EventRecord
	Args         submitWorkerUpdateArgs
}

func (rt *Runtime) mirrorWorkerSubmitUpdates(ctx context.Context, superRec *types.RunRecord, evidence workerRunEvidence, result map[string]any) {
	if rt == nil || rt.store == nil || superRec == nil || result == nil {
		return
	}
	channelID, targetAgentID, ok := delegateWorkerUpdateTarget(superRec)
	if !ok {
		return
	}
	submitted := collectSuccessfulWorkerSubmitUpdates(evidence.Events)
	if len(submitted) == 0 {
		return
	}

	var mirroredIDs []string
	var mirrorErrors []string
	for _, item := range submitted {
		sourceUpdateID := strings.TrimSpace(item.Args.UpdateID)
		if sourceUpdateID == "" {
			continue
		}
		sourceRunID := strings.TrimSpace(item.InvokedEvent.RunID)
		sourceAgentID := strings.TrimSpace(item.InvokedEvent.AgentID)
		if sourceAgentID == "" {
			sourceAgentID = stringMapValue(result, "agent_id")
		}
		role := firstNonEmpty(stringMapValue(result, "profile"), AgentProfileVSuper)
		update := types.WorkerUpdateRecord{
			UpdateID:      "mirrored-worker-update-" + sanitizeExportPart(superRec.RunID) + "-" + sanitizeExportPart(sourceUpdateID),
			OwnerID:       superRec.OwnerID,
			AgentID:       firstNonEmpty(sourceAgentID, agentIDForRun(superRec)),
			TargetAgentID: targetAgentID,
			ChannelID:     channelID,
			TrajectoryID:  metadataStringValue(superRec.Metadata, runMetadataTrajectoryID),
			Role:          role,
			Findings:      trimNonEmpty(item.Args.Findings),
			EvidenceIDs:   trimNonEmpty(item.Args.EvidenceIDs),
			Artifacts:     trimNonEmpty(item.Args.Artifacts),
			Refs:          trimNonEmpty(item.Args.Refs),
			Tests:         trimNonEmpty(item.Args.Tests),
			Questions:     trimNonEmpty(item.Args.Questions),
			Proposals:     trimNonEmpty(item.Args.Proposals),
			Notes:         trimNonEmpty(item.Args.Notes),
			CreatedAt:     item.ResultEvent.Timestamp,
		}
		if update.CreatedAt.IsZero() {
			update.CreatedAt = time.Now().UTC()
		}
		update.EvidenceIDs = trimDedupeNonEmpty(append(update.EvidenceIDs,
			"worker_run:"+sourceRunID,
			"worker_tool_invoked_event:"+item.InvokedEvent.EventID,
			"worker_tool_result_event:"+item.ResultEvent.EventID,
		))
		update.Refs = trimDedupeNonEmpty(append(update.Refs,
			"worker_update:"+sourceUpdateID,
			"worker_agent:"+sourceAgentID,
			"worker_run:"+sourceRunID,
		))
		update.Notes = trimDedupeNonEmpty(append(update.Notes,
			"mirrored_from=worker_submit_worker_update",
			"super_run:"+superRec.RunID,
		))
		update.Content = buildWorkerUpdateMessage(update)

		message := &types.ChannelMessage{
			ChannelID:    update.ChannelID,
			From:         superRec.RunID,
			FromAgentID:  agentIDForRun(superRec),
			FromRunID:    superRec.RunID,
			ToAgentID:    update.TargetAgentID,
			TrajectoryID: update.TrajectoryID,
			Role:         AgentProfileSuper,
			Content:      update.Content,
			Timestamp:    update.CreatedAt,
		}
		delivery := types.InboxDelivery{
			DeliveryID:   uuid.NewString(),
			OwnerID:      update.OwnerID,
			ToAgentID:    update.TargetAgentID,
			FromAgentID:  message.FromAgentID,
			FromRunID:    message.FromRunID,
			ChannelID:    update.ChannelID,
			Role:         message.Role,
			Content:      message.Content,
			TrajectoryID: update.TrajectoryID,
			CreatedAt:    update.CreatedAt,
		}
		stored, created, err := rt.store.DispatchWorkerUpdate(ctx, update, message, delivery)
		if err != nil {
			mirrorErrors = append(mirrorErrors, fmt.Sprintf("%s: %v", sourceUpdateID, err))
			continue
		}
		if !created {
			if err := validateExistingWorkerUpdate(stored, update); err != nil {
				mirrorErrors = append(mirrorErrors, err.Error())
				continue
			}
		} else {
			message.Seq = stored.MessageSeq
			rt.emitChannelMessageEvent(ctx, *message, update.OwnerID)
		}
		mirroredIDs = append(mirroredIDs, stored.UpdateID)
	}
	if len(mirroredIDs) > 0 {
		result["mirrored_worker_update_count"] = len(mirroredIDs)
		result["mirrored_worker_update_ids"] = mirroredIDs
	}
	if len(mirrorErrors) > 0 {
		result["mirrored_worker_update_errors"] = mirrorErrors
	}
}

func collectSuccessfulWorkerSubmitUpdates(events []types.EventRecord) []mirroredWorkerSubmitUpdate {
	invokedByCallID := map[string]mirroredWorkerSubmitUpdate{}
	var order []string
	for _, ev := range events {
		if ev.Kind != types.EventToolInvoked {
			continue
		}
		var payload struct {
			Tool      string          `json:"tool"`
			CallID    string          `json:"call_id"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(ev.Payload, &payload); err != nil || strings.TrimSpace(payload.Tool) != "submit_worker_update" {
			continue
		}
		callID := strings.TrimSpace(payload.CallID)
		if callID == "" {
			continue
		}
		var args submitWorkerUpdateArgs
		if err := json.Unmarshal(payload.Arguments, &args); err != nil || strings.TrimSpace(args.UpdateID) == "" {
			continue
		}
		if _, exists := invokedByCallID[callID]; !exists {
			order = append(order, callID)
		}
		invokedByCallID[callID] = mirroredWorkerSubmitUpdate{InvokedEvent: ev, Args: args}
	}
	if len(invokedByCallID) == 0 {
		return nil
	}

	successByCallID := map[string]types.EventRecord{}
	for _, ev := range events {
		if ev.Kind != types.EventToolResult {
			continue
		}
		var payload struct {
			Tool    string `json:"tool"`
			CallID  string `json:"call_id"`
			IsError bool   `json:"is_error"`
		}
		if err := json.Unmarshal(ev.Payload, &payload); err != nil ||
			strings.TrimSpace(payload.Tool) != "submit_worker_update" ||
			strings.TrimSpace(payload.CallID) == "" ||
			payload.IsError {
			continue
		}
		successByCallID[strings.TrimSpace(payload.CallID)] = ev
	}

	var out []mirroredWorkerSubmitUpdate
	seenUpdates := map[string]bool{}
	for _, callID := range order {
		item := invokedByCallID[callID]
		resultEvent, ok := successByCallID[callID]
		if !ok {
			continue
		}
		sourceUpdateID := strings.TrimSpace(item.Args.UpdateID)
		if sourceUpdateID == "" || seenUpdates[sourceUpdateID] {
			continue
		}
		seenUpdates[sourceUpdateID] = true
		item.ResultEvent = resultEvent
		out = append(out, item)
	}
	return out
}

func newCancelWorkerDelegationTool(rt *Runtime) Tool {
	type args struct {
		WorkerSandboxURL string `json:"worker_sandbox_url"`
		WorkerRunID      string `json:"worker_run_id,omitempty"`
		LoopID           string `json:"loop_id,omitempty"`
		Reason           string `json:"reason,omitempty"`
	}
	return Tool{
		Name:        "cancel_worker_delegation",
		Description: "Request cancellation of an async worker delegation. The worker runtime records cancellation and preserves durable evidence already produced.",
		Parameters: jsonSchemaObject(map[string]any{
			"worker_sandbox_url": map[string]any{"type": "string"},
			"worker_run_id":      map[string]any{"type": "string"},
			"loop_id":            map[string]any{"type": "string"},
			"reason":             map[string]any{"type": "string"},
		}, []string{"worker_sandbox_url"}, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			if profile := stringFromToolContext(ctx, toolCtxProfile); profile != AgentProfileSuper {
				return "", fmt.Errorf("cancel_worker_delegation is only available to super agents")
			}
			var in args
			if err := json.Unmarshal(raw, &in); err != nil {
				return "", fmt.Errorf("decode cancel_worker_delegation args: %w", err)
			}
			ownerID := stringFromToolContext(ctx, toolCtxOwnerID)
			if ownerID == "" {
				return "", fmt.Errorf("cancel_worker_delegation missing owner context")
			}
			workerRunID := firstNonEmpty(strings.TrimSpace(in.WorkerRunID), strings.TrimSpace(in.LoopID))
			if workerRunID == "" {
				return "", fmt.Errorf("worker_run_id or loop_id is required")
			}
			client := &http.Client{Timeout: 10 * time.Second}
			if err := cancelInternalWorkerRun(ctx, client, in.WorkerSandboxURL, ownerID, workerRunID); err != nil {
				return "", err
			}
			return toolResultJSON(map[string]any{
				"status":  "cancel_requested",
				"loop_id": workerRunID,
				"reason":  strings.TrimSpace(in.Reason),
			})
		},
	}
}

func newLegacySynchronousDelegateWorkerVMTool(rt *Runtime, cwd string) Tool {
	return Tool{
		Name:        "delegate_worker_vm_sync_legacy_disabled",
		Description: "Disabled legacy synchronous worker delegation implementation retained only for rollback reference; do not register.",
		Parameters: jsonSchemaObject(map[string]any{
			"worker_sandbox_url": map[string]any{"type": "string"},
			"worker_id":          map[string]any{"type": "string"},
			"vm_id":              map[string]any{"type": "string"},
			"objective":          map[string]any{"type": "string"},
			"profile":            map[string]any{"type": "string", "enum": []string{AgentProfileVSuper, AgentProfileCoSuper, AgentProfileResearcher}},
			"timeout_seconds":    map[string]any{"type": "integer"},
		}, []string{"worker_sandbox_url", "objective"}, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			if profile := stringFromToolContext(ctx, toolCtxProfile); profile != AgentProfileSuper {
				return "", fmt.Errorf("delegate_worker_vm is only available to super agents")
			}
			if rt == nil {
				return "", fmt.Errorf("delegate_worker_vm missing runtime")
			}
			var in delegateWorkerVMArgs
			if err := json.Unmarshal(raw, &in); err != nil {
				return "", fmt.Errorf("decode delegate_worker_vm args: %w", err)
			}
			ownerID := stringFromToolContext(ctx, toolCtxOwnerID)
			if ownerID == "" {
				return "", fmt.Errorf("delegate_worker_vm missing owner context")
			}
			objective := strings.TrimSpace(in.Objective)
			if objective == "" {
				return "", fmt.Errorf("objective must not be empty")
			}
			delegatedObjective := objective
			profile := canonicalAgentProfile(in.Profile)
			if profile == "" {
				profile = AgentProfileVSuper
			}
			if profile != AgentProfileVSuper && profile != AgentProfileCoSuper && profile != AgentProfileResearcher {
				return "", fmt.Errorf("profile must be %s, %s, or %s", AgentProfileVSuper, AgentProfileCoSuper, AgentProfileResearcher)
			}
			timeout := time.Duration(in.TimeoutSeconds) * time.Second
			if timeout <= 0 {
				timeout = defaultDelegateWorkerVMTimeout
			}
			if timeout > maxDelegateWorkerVMTimeout {
				timeout = maxDelegateWorkerVMTimeout
			}
			client := &http.Client{Timeout: 30 * time.Second}

			runID := stringFromToolContext(ctx, toolCtxRunID)
			agentID := stringFromToolContext(ctx, toolCtxAgentID)
			if cached, ok, err := rt.findExistingWorkerVMDelegation(ctx, runID, in, profile); err != nil {
				return "", err
			} else if ok {
				return markToolResultDeduped(cached, "super_run_already_delegated_worker_vm")
			}
			trajectoryID := ""
			if rec := ctxRunRecord(ctx); rec != nil && rec.Metadata != nil {
				trajectoryID = metadataStringValue(rec.Metadata, runMetadataTrajectoryID)
			}
			metadata := map[string]any{
				runMetadataAgentProfile: profile,
				runMetadataAgentRole:    profile,
				"request_source":        "worker_vm_delegation",
				"delegated_by_run_id":   runID,
				"delegated_by_agent_id": agentID,
				"delegated_by_profile":  AgentProfileSuper,
				"worker_id":             strings.TrimSpace(in.WorkerID),
				"worker_vm_id":          strings.TrimSpace(in.VMID),
				"parent_sandbox_id":     stringFromToolContext(ctx, toolCtxSandboxID),
			}
			if channelID := stringFromToolContext(ctx, toolCtxChannelID); channelID != "" {
				metadata[runMetadataChannelID] = channelID
			}
			if desktopID := stringFromToolContext(ctx, toolCtxDesktopID); desktopID != "" {
				metadata[runMetadataDesktopID] = desktopID
			}
			if trajectoryID != "" {
				metadata[runMetadataTrajectoryID] = trajectoryID
			}

			isolation, err := prepareSameRuntimeWorkerIsolation(ctx, cwd, in.WorkerSandboxURL, in.WorkerID, in.VMID, runID)
			if err != nil {
				return "", err
			}
			if isolation.Enabled {
				metadata[runMetadataToolCWD] = isolation.WorktreePath
				metadata[runMetadataWorkerIsolation] = isolation.Kind
				metadata[runMetadataWorkerBaseSHA] = isolation.BaseSHA
				metadata[runMetadataWorkerBranch] = isolation.Branch
				metadata[runMetadataWorkerWorktree] = isolation.WorktreePath
				objective = isolation.WorkerPrompt + "\n\n" + delegatedObjective
			} else if bootstrap, err := prepareRemoteWorkerRepoBootstrap(ctx, cwd, in.WorkerSandboxURL, profile); err != nil {
				return "", err
			} else if bootstrap.Enabled {
				metadata[runMetadataWorkerRepoBootstrap] = bootstrap.Kind
				metadata[runMetadataWorkerRepoRemote] = bootstrap.RemoteURL
				metadata[runMetadataWorkerRepoBaseSHA] = bootstrap.BaseSHA
				objective = bootstrap.WorkerPrompt + "\n\n" + delegatedObjective
			}
			if profile == AgentProfileVSuper {
				objective = workerVSuperDelegateContract(timeout) + "\n\n" + objective
			}

			baseResult := func(status string) map[string]any {
				result := map[string]any{
					"status":              status,
					"worker_id":           strings.TrimSpace(in.WorkerID),
					"worker_vm_id":        strings.TrimSpace(in.VMID),
					"worker_sandbox_url":  strings.TrimSpace(in.WorkerSandboxURL),
					"app_change_packages": []map[string]any{},
					"event_count":         0,
				}
				if isolation.Enabled {
					result["worker_isolation"] = isolation.Kind
					result["worker_worktree_path"] = isolation.WorktreePath
					result["worker_branch"] = isolation.Branch
					result["worker_base_sha"] = isolation.BaseSHA
				}
				if bootstrap := metadataStringValue(metadata, runMetadataWorkerRepoBootstrap); bootstrap != "" {
					result["worker_repo_bootstrap"] = bootstrap
					result["worker_repo_remote_url"] = metadataStringValue(metadata, runMetadataWorkerRepoRemote)
					result["worker_repo_base_sha"] = metadataStringValue(metadata, runMetadataWorkerRepoBaseSHA)
				}
				return result
			}
			resultWithWorkerEvents := func(result map[string]any, workerRunID string) map[string]any {
				workerRunID = strings.TrimSpace(workerRunID)
				if workerRunID == "" {
					return result
				}
				evidence, eventsErr := fetchWorkerRunEvidence(ctx, client, in.WorkerSandboxURL, ownerID, workerRunID)
				if eventsErr != nil {
					result["worker_event_error"] = eventsErr.Error()
					return result
				}
				applyWorkerRunEvidence(result, evidence)
				if packages := collectAppChangePackageResults(evidence.Events); len(packages) > 0 {
					var mirrorErrors []string
					packages, mirrorErrors = rt.mirrorWorkerAppChangePackages(ctx, client, in.WorkerSandboxURL, ownerID, packages)
					result["app_change_packages"] = packages
					annotateWorkerPackageMirrorResult(result, packages, mirrorErrors)
				}
				if summary := summarizeWorkerRunEvents(evidence.Events); len(summary) > 0 {
					result["worker_event_summary"] = summary
				}
				if profiles := collectWorkerSpawnProfiles(evidence.Events); len(profiles) > 0 {
					result["worker_spawned_profiles"] = profiles
				}
				if count := countWorkerChannelMessages(evidence.Events); count > 0 {
					result["worker_channel_message_count"] = count
				}
				return result
			}
			checkpointDelegateResult := func(result map[string]any, source string) map[string]any {
				status := stringMapValue(result, "status")
				if status == "" {
					return result
				}
				updateCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				if err := rt.synthesizeDelegateWorkerUpdateCheckpoint(updateCtx, ctxRunRecord(ctx), result, source); err != nil {
					result["worker_update_error"] = err.Error()
				} else {
					result["worker_update_checkpoint"] = "submitted_or_existing"
				}
				return result
			}
			var startResp *runStatusResponse
			var finalResp *runStatusResponse
			for attempt := 1; attempt <= maxDelegateWorkerRunAttempts; attempt++ {
				attemptMetadata := copyMetadataMap(metadata)
				attemptObjective := objective
				if attempt > 1 && finalResp != nil {
					attemptMetadata["retry_of_run_id"] = finalResp.RunID
					attemptMetadata["retry_reason"] = strings.TrimSpace(finalResp.Error)
					attemptObjective = strings.Join([]string{
						"Previous delegated worker run was interrupted before completion.",
						"Retry once on the same worker VM. If a go-choir-candidate checkout already exists, reset it to the requested base SHA before editing.",
						objective,
					}, "\n")
				}
				startResp, err = submitInternalWorkerRun(ctx, client, in.WorkerSandboxURL, internalRunSubmitRequest{
					OwnerID:  ownerID,
					Prompt:   attemptObjective,
					Metadata: attemptMetadata,
				})
				if err != nil {
					result := baseResult("worker_run_submit_failed")
					result["error"] = err.Error()
					result["terminal_error"] = err.Error()
					result["attempt"] = attempt
					result = rt.invalidateWorkerVMRequestCacheForDelegateResult(ctx, result)
					result = checkpointDelegateResult(result, "submit_failed")
					return toolResultJSON(result)
				}
				finalResp, err = pollInternalWorkerRun(ctx, client, in.WorkerSandboxURL, ownerID, startResp.RunID, timeout)
				if err != nil {
					var pollErr *workerRunPollError
					if errors.As(err, &pollErr) {
						status := "worker_run_status_failed"
						if pollErr.TimedOut {
							status = "worker_run_timeout"
						}
						workerRunID := firstNonEmpty(pollErr.RunID, startResp.RunID)
						result := baseResult(status)
						result["loop_id"] = workerRunID
						result["agent_id"] = pollErr.Last.AgentID
						result["profile"] = pollErr.Last.AgentProfile
						result["state"] = pollErr.Last.State
						result["result"] = pollErr.Last.Result
						result["error"] = err.Error()
						result["terminal_error"] = err.Error()
						result["attempt"] = attempt
						result["timeout_seconds"] = int(timeout.Seconds())
						evidence, eventsErr := fetchWorkerRunEvidence(ctx, client, in.WorkerSandboxURL, ownerID, workerRunID)
						if eventsErr != nil {
							result["worker_event_error"] = eventsErr.Error()
						} else {
							applyWorkerRunEvidence(result, evidence)
							packages := collectAppChangePackageResults(evidence.Events)
							if len(packages) > 0 {
								var mirrorErrors []string
								packages, mirrorErrors = rt.mirrorWorkerAppChangePackages(ctx, client, in.WorkerSandboxURL, ownerID, packages)
								result["app_change_packages"] = packages
								annotateWorkerPackageMirrorResult(result, packages, mirrorErrors)
								result["reviewable_package_observed"] = true
								if pollErr.TimedOut {
									result["completion_blocker"] = "vsuper_timed_out_after_reviewable_package"
								}
							}
							if summary := summarizeWorkerRunEvents(evidence.Events); len(summary) > 0 {
								result["worker_event_summary"] = summary
							}
							if profiles := collectWorkerSpawnProfiles(evidence.Events); len(profiles) > 0 {
								result["worker_spawned_profiles"] = profiles
							}
							if count := countWorkerChannelMessages(evidence.Events); count > 0 {
								result["worker_channel_message_count"] = count
							}
						}
						result = rt.invalidateWorkerVMRequestCacheForDelegateResult(ctx, result)
						result = checkpointDelegateResult(result, status)
						return toolResultJSON(result)
					}
					result := baseResult("worker_run_status_failed")
					result["loop_id"] = startResp.RunID
					result["error"] = err.Error()
					result["terminal_error"] = err.Error()
					result["attempt"] = attempt
					result = resultWithWorkerEvents(result, startResp.RunID)
					result = rt.invalidateWorkerVMRequestCacheForDelegateResult(ctx, result)
					result = checkpointDelegateResult(result, "status_failed")
					return toolResultJSON(result)
				}
				if finalResp.State == types.RunCompleted {
					break
				}
				if attempt < maxDelegateWorkerRunAttempts && isInterruptedWorkerRun(finalResp) {
					continue
				}
				break
			}
			if finalResp == nil || startResp == nil {
				return "", fmt.Errorf("delegate_worker_vm missing worker run status")
			}
			evidence, err := fetchWorkerRunEvidence(ctx, client, in.WorkerSandboxURL, ownerID, finalResp.RunID)
			if err != nil {
				if finalResp.State == types.RunCompleted {
					return "", err
				}
				evidence = workerRunEvidence{}
			}
			if profile == AgentProfileVSuper && finalResp.State == types.RunCompleted {
				evidence = followWorkerChildRuns(ctx, client, in.WorkerSandboxURL, ownerID, finalResp.RunID, evidence, timeout)
			}
			packages := collectAppChangePackageResults(evidence.Events)
			var packageMirrorErrors []string
			packages, packageMirrorErrors = rt.mirrorWorkerAppChangePackages(ctx, client, in.WorkerSandboxURL, ownerID, packages)

			result := map[string]any{
				"status":              delegateWorkerRunStatus(finalResp.State),
				"worker_id":           strings.TrimSpace(in.WorkerID),
				"worker_vm_id":        strings.TrimSpace(in.VMID),
				"worker_sandbox_url":  strings.TrimSpace(in.WorkerSandboxURL),
				"loop_id":             finalResp.RunID,
				"agent_id":            finalResp.AgentID,
				"profile":             finalResp.AgentProfile,
				"state":               finalResp.State,
				"result":              finalResp.Result,
				"error":               finalResp.Error,
				"app_change_packages": packages,
			}
			annotateWorkerPackageMirrorResult(result, packages, packageMirrorErrors)
			if finalResp.State != types.RunCompleted && len(packages) > 0 {
				result["reviewable_package_observed"] = true
				result["completion_blocker"] = firstNonEmpty(stringMapValue(result, "completion_blocker"), "vsuper_ended_non_completed_after_reviewable_package")
			}
			applyWorkerRunEvidence(result, evidence)
			requiresPackage := delegateRequiresAppChangePackage(profile, in.Objective)
			if finalResp.State == types.RunCompleted && requiresPackage && len(packages) == 0 {
				result["status"] = "worker_run_incomplete"
				result["completion_blocker"] = "vsuper_completed_without_required_app_change_package"
				result["terminal_error"] = "worker vsuper completed a package-required objective without publish_app_change_package evidence"
			} else if finalResp.State == types.RunCompleted && requiresPackage && len(packages) > 0 && countProductVisibleAppChangePackages(packages) == 0 {
				result["status"] = "worker_run_incomplete"
				result["completion_blocker"] = "app_change_package_not_product_visible"
				result["terminal_error"] = "worker vsuper published AppChangePackage evidence, but no package could be mirrored into the product-visible package store"
			} else if profile == AgentProfileVSuper && finalResp.State == types.RunCompleted && vSuperDelegateIncomplete(evidence, packages) {
				result["status"] = "worker_run_incomplete"
				result["completion_blocker"] = "vsuper_completed_without_app_change_package_or_worker_update"
				result["terminal_error"] = "worker vsuper completed after child coordination without publish_app_change_package or submit_worker_update evidence"
			}
			if finalResp.State != types.RunCompleted {
				result["terminal_error"] = strings.TrimSpace(fmt.Sprintf("worker run %s ended in state %s: %s", finalResp.RunID, finalResp.State, strings.TrimSpace(finalResp.Error)))
				if err != nil {
					result["worker_event_error"] = err.Error()
				}
			}
			if summary := summarizeWorkerRunEvents(evidence.Events); len(summary) > 0 {
				result["worker_event_summary"] = summary
			}
			if profiles := collectWorkerSpawnProfiles(evidence.Events); len(profiles) > 0 {
				result["worker_spawned_profiles"] = profiles
			}
			if count := countWorkerChannelMessages(evidence.Events); count > 0 {
				result["worker_channel_message_count"] = count
			}
			for key, value := range baseResult("") {
				if key == "status" {
					continue
				}
				if _, ok := result[key]; !ok {
					result[key] = value
				}
			}
			result = checkpointDelegateResult(result, "terminal_result")
			return toolResultJSON(result)
		},
	}
}

type delegateWorkerVMArgs struct {
	WorkerSandboxURL string `json:"worker_sandbox_url"`
	WorkerID         string `json:"worker_id,omitempty"`
	VMID             string `json:"vm_id,omitempty"`
	Objective        string `json:"objective"`
	Profile          string `json:"profile,omitempty"`
	TimeoutSeconds   int    `json:"timeout_seconds,omitempty"`
}

func delegateWorkerRunStatus(state types.RunState) string {
	switch state {
	case types.RunCompleted:
		return "worker_run_completed"
	case types.RunFailed:
		return "worker_run_failed"
	case types.RunCancelled:
		return "worker_run_cancelled"
	case types.RunBlocked:
		return "worker_run_blocked"
	default:
		stateText := strings.TrimSpace(string(state))
		if stateText == "" {
			return "worker_run_terminal"
		}
		return "worker_run_" + stateText
	}
}

type localWorkerIsolation struct {
	Enabled      bool
	Kind         string
	WorktreePath string
	Branch       string
	BaseSHA      string
	WorkerPrompt string
}

type remoteWorkerRepoBootstrap struct {
	Enabled      bool
	Kind         string
	RemoteURL    string
	BaseSHA      string
	WorkerPrompt string
}

func prepareRemoteWorkerRepoBootstrap(ctx context.Context, cwd, workerSandboxURL, profile string) (remoteWorkerRepoBootstrap, error) {
	if sameRuntimeWorkerURL(workerSandboxURL) {
		return remoteWorkerRepoBootstrap{}, nil
	}
	profile = canonicalAgentProfile(profile)
	if profile != AgentProfileVSuper && profile != AgentProfileCoSuper {
		return remoteWorkerRepoBootstrap{}, nil
	}
	gitRemoteURL, gitBaseSHA := remoteWorkerRepoBootstrapSourceFromGit(ctx, cwd)
	envRemoteURL, envBaseSHA := remoteWorkerRepoBootstrapSourceFromEnv()
	remoteURL := firstNonEmptyString(gitRemoteURL, envRemoteURL)
	baseSHA := firstNonEmptyString(envBaseSHA, gitBaseSHA)
	if remoteURL == "" {
		return remoteWorkerRepoBootstrap{}, nil
	}
	if !usableWorkerRepoBaseSHA(baseSHA) {
		return remoteWorkerRepoBootstrap{}, nil
	}
	prompt := remoteWorkerRepoBootstrapPrompt(remoteURL, baseSHA)
	return remoteWorkerRepoBootstrap{
		Enabled:      true,
		Kind:         "remote_git_clone",
		RemoteURL:    remoteURL,
		BaseSHA:      baseSHA,
		WorkerPrompt: prompt,
	}, nil
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func remoteWorkerRepoBootstrapSourceFromGit(ctx context.Context, cwd string) (string, string) {
	repoRoot, err := gitOutputInDir(ctx, cwd, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", ""
	}
	repoRoot = strings.TrimSpace(repoRoot)
	baseSHA, err := gitOutputInDir(ctx, repoRoot, "rev-parse", "HEAD")
	if err != nil {
		return "", ""
	}
	remoteRaw, err := gitOutputInDir(ctx, repoRoot, "remote", "get-url", "origin")
	if err != nil {
		return "", ""
	}
	return safeWorkerGitRemote(remoteRaw), strings.TrimSpace(baseSHA)
}

func remoteWorkerRepoBootstrapSourceFromEnv() (string, string) {
	remoteURL := safeWorkerGitRemote(firstNonEmptyEnv(
		"RUNTIME_WORKER_REPO_REMOTE",
		"CHOIR_WORKER_REPO_REMOTE",
	))
	baseSHA := firstNonEmptyEnv(
		"RUNTIME_WORKER_REPO_BASE_SHA",
		"CHOIR_WORKER_REPO_BASE_SHA",
		"CHOIR_DEPLOYED_COMMIT",
	)
	return remoteURL, normalizeWorkerRepoBaseSHA(baseSHA)
}

func remoteWorkerRepoBootstrapPrompt(remoteURL, baseSHA string) string {
	return strings.Join([]string{
		"Remote worker repository bootstrap is available.",
		"The worker VM may start in an empty files directory. Before repository work, create or use a checkout named go-choir-candidate under the current working directory.",
		"Bootstrap commands:",
		"if [ ! -d go-choir-candidate/.git ]; then git clone " + remoteURL + " go-choir-candidate; fi",
		"cd go-choir-candidate",
		"git config user.name \"Choir Worker\"",
		"git config user.email \"worker@choir.local\"",
		"git fetch --all --prune",
		"git checkout " + baseSHA,
		"git reset --hard " + baseSHA,
		"git clean -fdx",
		"Perform all repository edits inside go-choir-candidate. Do not push from the worker VM.",
		"The worker VM exposes repo tools directly in PATH, including git, go, gofmt, python3, perl, node, npm, curl, make, gcc, pkg-config, the Obscura browser binary, and ICU libraries.",
		"Run gofmt, go test, node/npm, Obscura, and scripts directly from the checkout. Do not run nix develop, nix build, or nix-store inside the worker VM; the guest Nix store is read-only and those commands are not verifier evidence.",
		"If Obscura is required and `command -v obscura` fails, check `CHOIR_OBSCURA_BIN` and `OBSCURA_BIN` before concluding the browser substrate is missing; report PATH and those env vars in the blocker.",
		"For UI/human-proof work, tests must mount the actual app/component or use the product path. Use Obscura for VM-local browser/extraction evidence when suitable; Chrome/Playwright is an external verifier, not a worker-VM dependency. A static fixture that hand-creates expected markup is diagnostic only and must not be treated as screenshot/video behavior proof.",
		"Use set -euo pipefail for multi-step bash commands so a failed commit, test, or export cannot be hidden by a later successful command.",
		"Commit candidate changes before calling publish_app_change_package.",
		"Use repo_path \"go-choir-candidate\" and base_sha " + baseSHA + " when publishing an AppChangePackage.",
		"If clone, checkout, build, verification, or package publication fails, report diagnostics with submit_worker_update instead of claiming repository work or ending with a plain narrative.",
	}, "\n")
}

func workerVSuperDelegateContract(timeout time.Duration) string {
	reserve := 2 * time.Minute
	if timeout < 4*time.Minute {
		reserve = timeout / 3
	}
	return strings.Join([]string{
		"Worker-vsuper delegate contract:",
		"- Keep at most one implementation co-super and one verifier co-super active for candidate repo work.",
		"- Spawn the implementation co-super first with slot=\"implementation\" and put the implementation role plus terminal obligation directly in that objective; do not depend on a later role-correction cast as the child's first authoritative instruction.",
		"- Do not spawn slot=\"verifier\" until the implementation child has reported commit/package/blocker evidence. When you spawn the verifier, name the exact commit/package/evidence to inspect in the verifier objective.",
		"- If a verifier was accidentally spawned before implementation evidence and completed against the base checkout, label that result stale and spawn at most one replacement verifier after implementation evidence exists.",
		"- If you spawn an implementation co-super, treat that child as the exclusive writer for go-choir-candidate while it is active; do not run reset, clean, edit, or commit commands in the same checkout until the child reports commit/package/blocker evidence.",
		"- Do not cancel a child that has produced publish_app_change_package evidence. Incorporate the child package instead.",
		"- The verifier must inspect only after the implementation child has reported a commit, package, or blocker; avoid racing the worker by repeatedly reading a checkout that is still being mutated.",
		"- If the objective asks a helper to publish a package, do not override that with \"do not publish\"; let the helper publish, then report that child package.",
		"- Tell the implementation child that missing tools, failed tests, or package publication failure must end in submit_worker_update with exact command output refs, not a plain final answer.",
		"- Once a committed repo diff and focused verification evidence exist, make exactly one publish_app_change_package call for the candidate. If a child already published, do not parent-publish again.",
		"- After package evidence exists, immediately produce the terminal summary or submit_worker_update. Do not sleep, poll for narrative confirmation, or run broad discovery unless the package is invalid and you are doing one focused repair.",
		"- Starting children, casting assignments, or receiving acknowledgement-only messages is not a terminal result; wait for commit/package/verifier/blocker evidence, or submit_worker_update with the precise missing-evidence blocker.",
		"- If both child runs finish without publish_app_change_package or submit_worker_update evidence, inspect their final results and tool errors, then submit_worker_update naming the child loop ids and the missing terminal evidence.",
		"- Reserve the last " + reserve.String() + " of the delegate budget for exactly one terminal action: publish_app_change_package or submit_worker_update with a precise blocker.",
		"- A blocked submit_worker_update is preferred to running until the parent delegate timeout.",
	}, "\n")
}

func usableWorkerRepoBaseSHA(baseSHA string) bool {
	baseSHA = normalizeWorkerRepoBaseSHA(baseSHA)
	if baseSHA == "" || baseSHA == "local" || baseSHA == "unknown" {
		return false
	}
	if len(baseSHA) < 7 || len(baseSHA) > 64 {
		return false
	}
	for _, r := range baseSHA {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			return false
		}
	}
	return true
}

func normalizeWorkerRepoBaseSHA(baseSHA string) string {
	return strings.TrimSuffix(strings.TrimSpace(strings.ToLower(baseSHA)), "-dirty")
}

func firstNonEmptyEnv(keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return ""
}

func prepareSameRuntimeWorkerIsolation(ctx context.Context, cwd, workerSandboxURL, workerID, vmID, runID string) (localWorkerIsolation, error) {
	if !sameRuntimeWorkerURL(workerSandboxURL) {
		return localWorkerIsolation{}, nil
	}
	mode := strings.ToLower(strings.TrimSpace(os.Getenv("RUNTIME_LOCAL_WORKER_MODE")))
	if mode == "" {
		return localWorkerIsolation{}, fmt.Errorf("delegate_worker_vm refused same-runtime worker delegation without isolation; set RUNTIME_LOCAL_WORKER_MODE=worktree or use a distinct worker sandbox")
	}
	if mode != "worktree" {
		return localWorkerIsolation{}, fmt.Errorf("delegate_worker_vm unsupported local worker isolation mode %q", mode)
	}
	return createLocalWorkerWorktree(ctx, cwd, workerID, vmID, runID)
}

func sameRuntimeWorkerURL(workerSandboxURL string) bool {
	selfURL := strings.TrimSpace(os.Getenv("RUNTIME_SELF_URL"))
	if selfURL == "" {
		if port := strings.TrimSpace(os.Getenv("SANDBOX_PORT")); port != "" {
			selfURL = "http://127.0.0.1:" + port
		}
	}
	if selfURL == "" {
		return false
	}
	return normalizedRuntimeBaseURL(workerSandboxURL) != "" &&
		normalizedRuntimeBaseURL(workerSandboxURL) == normalizedRuntimeBaseURL(selfURL)
}

func normalizedRuntimeBaseURL(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	return strings.ToLower(parsed.Scheme + "://" + parsed.Host)
}

func createLocalWorkerWorktree(ctx context.Context, cwd, workerID, vmID, runID string) (localWorkerIsolation, error) {
	repoRoot, err := gitOutputInDir(ctx, cwd, "rev-parse", "--show-toplevel")
	if err != nil {
		return localWorkerIsolation{}, fmt.Errorf("local worker isolation requires a git repository: %w", err)
	}
	repoRoot = strings.TrimSpace(repoRoot)
	baseSHA, err := gitOutputInDir(ctx, repoRoot, "rev-parse", "HEAD")
	if err != nil {
		return localWorkerIsolation{}, fmt.Errorf("local worker isolation base sha: %w", err)
	}
	baseSHA = strings.TrimSpace(baseSHA)
	root := strings.TrimSpace(os.Getenv("RUNTIME_LOCAL_WORKER_ROOT"))
	if root == "" {
		root = filepath.Join(os.TempDir(), "go-choir-worker-worktrees")
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return localWorkerIsolation{}, fmt.Errorf("create local worker root: %w", err)
	}
	identity := sanitizeExportPart(firstNonEmpty(workerID, vmID, runID, uuid.NewString()))
	suffix := uuid.NewString()[:8]
	branch := "agent/local-worker/" + identity + "-" + suffix
	worktreePath := filepath.Join(root, identity+"-"+suffix)
	if _, err := gitOutputInDir(ctx, repoRoot, "worktree", "add", "-b", branch, worktreePath, baseSHA); err != nil {
		return localWorkerIsolation{}, fmt.Errorf("create local worker worktree: %w", err)
	}
	prompt := strings.Join([]string{
		"Local worker isolation is active.",
		"The current working directory is an isolated git worktree for this worker, not the foreground repository.",
		"Do not write outside the current working directory.",
		"Commit any repo changes in this worktree before calling publish_app_change_package.",
		"Use repo_path \".\" and base_sha " + baseSHA + " when publishing an AppChangePackage.",
	}, "\n")
	return localWorkerIsolation{
		Enabled:      true,
		Kind:         "local_worktree",
		WorktreePath: worktreePath,
		Branch:       branch,
		BaseSHA:      baseSHA,
		WorkerPrompt: prompt,
	}, nil
}

func gitOutputInDir(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	data, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(data)))
	}
	return string(data), nil
}

func safeWorkerGitRemote(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "git@github.com:") {
		path := strings.TrimPrefix(raw, "git@github.com:")
		path = strings.TrimLeft(path, "/")
		if path == "" || strings.ContainsAny(path, " \t\r\n") {
			return ""
		}
		return "https://github.com/" + path
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return ""
	}
	if parsed.User != nil {
		parsed.User = nil
	}
	if strings.ContainsAny(parsed.String(), " \t\r\n") {
		return ""
	}
	return parsed.String()
}

func copyMetadataMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func isInterruptedWorkerRun(resp *runStatusResponse) bool {
	if resp == nil {
		return false
	}
	if resp.State != types.RunFailed {
		return false
	}
	errText := strings.ToLower(strings.TrimSpace(resp.Error))
	return strings.Contains(errText, "runtime restarted") && strings.Contains(errText, "interrupted")
}

func submitInternalWorkerRun(ctx context.Context, client *http.Client, baseURL string, body internalRunSubmitRequest) (*runStatusResponse, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	endpoint, err := workerRuntimeURL(baseURL, "/internal/runtime/runs", nil)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Caller", "true")
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("delegate_worker_vm submit: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusAccepted {
		return nil, fmt.Errorf("delegate_worker_vm submit failed: %s: %s", resp.Status, strings.TrimSpace(string(payload)))
	}
	var out runStatusResponse
	if err := json.Unmarshal(payload, &out); err != nil {
		return nil, fmt.Errorf("decode worker run submit response: %w", err)
	}
	if out.RunID == "" {
		return nil, fmt.Errorf("worker run submit response missing loop_id")
	}
	return &out, nil
}

func pollInternalWorkerRun(ctx context.Context, client *http.Client, baseURL, ownerID, runID string, timeout time.Duration) (*runStatusResponse, error) {
	deadline := time.Now().Add(timeout)
	notFoundRetryUntil := time.Now().Add(workerRunStatusNotFoundRetryWindow)
	var last runStatusResponse
	var lastStatusErr error
	for {
		values := url.Values{"owner_id": []string{ownerID}}
		endpoint, err := workerRuntimeURL(baseURL, "/internal/runtime/runs/"+url.PathEscape(runID), values)
		if err != nil {
			return nil, err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("X-Internal-Caller", "true")
		resp, err := client.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			statusErr := fmt.Errorf("delegate_worker_vm status: %w", err)
			if shouldRetryWorkerStatusPoll(statusErr) && time.Now().Before(deadline) {
				lastStatusErr = statusErr
				if err := sleepUntilNextWorkerStatusPoll(ctx); err != nil {
					return nil, err
				}
				continue
			}
			return nil, newWorkerRunPollError(runID, timeout, last, statusErr, false)
		}
		payload, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			return nil, readErr
		}
		if resp.StatusCode != http.StatusOK {
			statusErr := fmt.Errorf("delegate_worker_vm status failed: %s: %s", resp.Status, strings.TrimSpace(string(payload)))
			if isRetryableWorkerStatusCode(resp.StatusCode, time.Now().Before(notFoundRetryUntil)) && time.Now().Before(deadline) {
				lastStatusErr = statusErr
				if err := sleepUntilNextWorkerStatusPoll(ctx); err != nil {
					return nil, err
				}
				continue
			}
			return nil, newWorkerRunPollError(runID, timeout, last, statusErr, false)
		}
		if err := json.Unmarshal(payload, &last); err != nil {
			return nil, fmt.Errorf("decode worker run status response: %w", err)
		}
		if last.State.Terminal() || last.State == types.RunBlocked {
			return &last, nil
		}
		if time.Now().After(deadline) {
			if lastStatusErr != nil {
				return nil, newWorkerRunPollError(runID, timeout, last, fmt.Errorf("worker run %s did not finish within %s; last state=%s; last status error=%v", runID, timeout, last.State, lastStatusErr), true)
			}
			return nil, newWorkerRunPollError(runID, timeout, last, fmt.Errorf("worker run %s did not finish within %s; last state=%s", runID, timeout, last.State), true)
		}
		if err := sleepUntilNextWorkerStatusPoll(ctx); err != nil {
			return nil, err
		}
	}
}

type workerRunPollError struct {
	RunID    string
	Timeout  time.Duration
	Last     runStatusResponse
	Err      error
	TimedOut bool
}

func newWorkerRunPollError(runID string, timeout time.Duration, last runStatusResponse, err error, timedOut bool) *workerRunPollError {
	if strings.TrimSpace(last.RunID) == "" {
		last.RunID = strings.TrimSpace(runID)
	}
	return &workerRunPollError{
		RunID:    strings.TrimSpace(runID),
		Timeout:  timeout,
		Last:     last,
		Err:      err,
		TimedOut: timedOut,
	}
}

func (e *workerRunPollError) Error() string {
	if e == nil {
		return ""
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return fmt.Sprintf("worker run %s status polling failed", strings.TrimSpace(e.RunID))
}

const workerRunStatusNotFoundRetryWindow = 5 * time.Second

func sleepUntilNextWorkerStatusPoll(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(500 * time.Millisecond):
		return nil
	}
}

func shouldRetryWorkerStatusPoll(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	if os.IsTimeout(err) {
		return true
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Timeout() || netErr.Temporary()
	}
	return false
}

func isRetryableWorkerStatusCode(statusCode int, retryNotFound bool) bool {
	switch statusCode {
	case http.StatusNotFound:
		return retryNotFound
	case http.StatusTooManyRequests, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

func fetchInternalWorkerRunEvents(ctx context.Context, client *http.Client, baseURL, ownerID, runID string) (*eventListResponse, error) {
	values := url.Values{"owner_id": []string{ownerID}, "limit": []string{"500"}}
	endpoint, err := workerRuntimeURL(baseURL, "/internal/runtime/runs/"+url.PathEscape(runID)+"/events", values)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Internal-Caller", "true")
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("delegate_worker_vm events: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("delegate_worker_vm events failed: %s: %s", resp.Status, strings.TrimSpace(string(payload)))
	}
	var out eventListResponse
	if err := json.Unmarshal(payload, &out); err != nil {
		return nil, fmt.Errorf("decode worker run events response: %w", err)
	}
	return &out, nil
}

type workerRunEvidence struct {
	Events            []types.EventRecord
	RootEventCount    int
	ChildRunIDs       []string
	ChildEventCounts  map[string]int
	ChildEventErrors  map[string]string
	ChildRunStates    map[string]types.RunState
	ChildStatusErrors map[string]string
}

func fetchWorkerRunEvidence(ctx context.Context, client *http.Client, baseURL, ownerID, rootRunID string) (workerRunEvidence, error) {
	rootResp, err := fetchInternalWorkerRunEvents(ctx, client, baseURL, ownerID, rootRunID)
	if err != nil {
		return workerRunEvidence{}, err
	}
	evidence := workerRunEvidence{
		Events:            append([]types.EventRecord{}, rootResp.Events...),
		RootEventCount:    len(rootResp.Events),
		ChildEventCounts:  map[string]int{},
		ChildEventErrors:  map[string]string{},
		ChildRunStates:    map[string]types.RunState{},
		ChildStatusErrors: map[string]string{},
	}
	for _, childRunID := range collectWorkerChildRunIDs(rootResp.Events) {
		childResp, err := fetchInternalWorkerRunEvents(ctx, client, baseURL, ownerID, childRunID)
		evidence.ChildRunIDs = append(evidence.ChildRunIDs, childRunID)
		if err != nil {
			evidence.ChildEventErrors[childRunID] = err.Error()
			continue
		}
		evidence.ChildEventCounts[childRunID] = len(childResp.Events)
		evidence.Events = append(evidence.Events, childResp.Events...)
		if status, err := fetchInternalWorkerRunStatus(ctx, client, baseURL, ownerID, childRunID); err != nil {
			evidence.ChildStatusErrors[childRunID] = err.Error()
		} else {
			evidence.ChildRunStates[childRunID] = status.State
		}
	}
	return evidence, nil
}

func fetchInternalWorkerRunStatus(ctx context.Context, client *http.Client, baseURL, ownerID, runID string) (*runStatusResponse, error) {
	values := url.Values{"owner_id": []string{ownerID}}
	endpoint, err := workerRuntimeURL(baseURL, "/internal/runtime/runs/"+url.PathEscape(runID), values)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Internal-Caller", "true")
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("delegate_worker_vm status: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("delegate_worker_vm status failed: %s: %s", resp.Status, strings.TrimSpace(string(payload)))
	}
	var out runStatusResponse
	if err := json.Unmarshal(payload, &out); err != nil {
		return nil, fmt.Errorf("decode worker run status response: %w", err)
	}
	return &out, nil
}

func cancelInternalWorkerRun(ctx context.Context, client *http.Client, baseURL, ownerID, runID string) error {
	values := url.Values{"owner_id": []string{ownerID}}
	endpoint, err := workerRuntimeURL(baseURL, "/internal/runtime/runs/"+url.PathEscape(runID)+"/cancel", values)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-Internal-Caller", "true")
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("cancel_worker_delegation: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusConflict {
		return fmt.Errorf("cancel_worker_delegation failed: %s: %s", resp.Status, strings.TrimSpace(string(payload)))
	}
	return nil
}

type internalChannelCastRequest struct {
	OwnerID     string `json:"owner_id"`
	ChannelID   string `json:"channel_id"`
	ToAgentID   string `json:"to_agent_id,omitempty"`
	ToRunID     string `json:"to_loop_id,omitempty"`
	FromAgentID string `json:"from_agent_id,omitempty"`
	FromRunID   string `json:"from_loop_id,omitempty"`
	From        string `json:"from,omitempty"`
	Role        string `json:"role,omitempty"`
	Content     string `json:"content"`
}

type internalChannelCastResponse struct {
	Status string `json:"status"`
	Cursor uint64 `json:"cursor"`
}

func postInternalWorkerChannelCast(ctx context.Context, client *http.Client, baseURL string, reqBody internalChannelCastRequest) (uint64, error) {
	endpoint, err := workerRuntimeURL(baseURL, "/internal/runtime/channel-casts", nil)
	if err != nil {
		return 0, err
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(string(payload)))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Caller", "true")
	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("redirect_worker_delegation channel cast: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("redirect_worker_delegation channel cast failed: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var out internalChannelCastResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return 0, fmt.Errorf("decode redirect_worker_delegation response: %w", err)
	}
	return out.Cursor, nil
}

func followWorkerChildRuns(ctx context.Context, client *http.Client, baseURL, ownerID, rootRunID string, evidence workerRunEvidence, timeout time.Duration) workerRunEvidence {
	if len(evidence.ChildRunIDs) == 0 {
		return evidence
	}
	states := map[string]types.RunState{}
	statusErrors := map[string]string{}
	deadline := time.Now().Add(timeout)
	for _, childRunID := range evidence.ChildRunIDs {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			statusErrors[childRunID] = "delegate worker child follow-up budget exhausted"
			continue
		}
		status, err := pollInternalWorkerRun(ctx, client, baseURL, ownerID, childRunID, remaining)
		if err != nil {
			statusErrors[childRunID] = err.Error()
			continue
		}
		states[childRunID] = status.State
	}
	refreshed, err := fetchWorkerRunEvidence(ctx, client, baseURL, ownerID, rootRunID)
	if err != nil {
		evidence = mergeFollowedWorkerChildRunStates(evidence, states, statusErrors)
		evidence.ChildEventErrors["_refresh"] = err.Error()
		return evidence
	}
	return mergeFollowedWorkerChildRunStates(refreshed, states, statusErrors)
}

func mergeFollowedWorkerChildRunStates(evidence workerRunEvidence, states map[string]types.RunState, statusErrors map[string]string) workerRunEvidence {
	if evidence.ChildRunStates == nil {
		evidence.ChildRunStates = map[string]types.RunState{}
	}
	if evidence.ChildStatusErrors == nil {
		evidence.ChildStatusErrors = map[string]string{}
	}
	for childRunID := range evidence.ChildRunStates {
		delete(evidence.ChildStatusErrors, childRunID)
	}
	for childRunID, state := range states {
		evidence.ChildRunStates[childRunID] = state
		delete(evidence.ChildStatusErrors, childRunID)
	}
	for childRunID, statusErr := range statusErrors {
		if _, ok := evidence.ChildRunStates[childRunID]; ok {
			continue
		}
		evidence.ChildStatusErrors[childRunID] = statusErr
	}
	return evidence
}

func applyWorkerRunEvidence(result map[string]any, evidence workerRunEvidence) {
	result["event_count"] = len(evidence.Events)
	if evidence.RootEventCount > 0 {
		result["worker_root_event_count"] = evidence.RootEventCount
	}
	if len(evidence.ChildRunIDs) > 0 {
		result["worker_child_run_ids"] = evidence.ChildRunIDs
	}
	if len(evidence.ChildEventCounts) > 0 {
		result["worker_child_event_counts"] = evidence.ChildEventCounts
	}
	if len(evidence.ChildEventErrors) > 0 {
		result["worker_child_event_errors"] = evidence.ChildEventErrors
	}
	if len(evidence.ChildRunStates) > 0 {
		result["worker_child_run_states"] = evidence.ChildRunStates
	}
	if len(evidence.ChildStatusErrors) > 0 {
		result["worker_child_status_errors"] = evidence.ChildStatusErrors
	}
}

func vSuperDelegateIncomplete(evidence workerRunEvidence, packages []map[string]any) bool {
	if len(evidence.ChildRunIDs) == 0 || len(packages) > 0 {
		return false
	}
	return !hasSuccessfulToolResult(evidence.Events, "submit_worker_update")
}

func delegateRequiresAppChangePackage(profile, objective string) bool {
	if canonicalAgentProfile(profile) != AgentProfileVSuper {
		return false
	}
	objective = strings.ToLower(objective)
	for _, negative := range []string{
		"do not publish_app_change_package",
		"do not call publish_app_change_package",
		"do not publish an appchangepackage",
		"do not publish appchangepackage",
		"do not publish an app change package",
		"do not publish app change package",
		"no appchangepackage",
		"no app change package",
	} {
		if strings.Contains(objective, negative) {
			return false
		}
	}
	for _, needle := range []string{
		"publish_app_change_package",
		"appchangepackage",
		"app change package",
		"package id",
		"publish exactly one package",
		"publish one package",
		"owner-pullable package",
		"package/adoption",
	} {
		if strings.Contains(objective, needle) {
			return true
		}
	}
	return false
}

func workerRuntimeURL(baseURL, path string, query url.Values) (string, error) {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return "", fmt.Errorf("worker_sandbox_url is required")
	}
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("parse worker_sandbox_url: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("worker_sandbox_url must be http or https")
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("worker_sandbox_url missing host")
	}
	parsed.Path = path
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func collectAppChangePackageResults(events []types.EventRecord) []map[string]any {
	var packages []map[string]any
	seen := make(map[string]bool)
	for _, ev := range events {
		if ev.Kind != types.EventToolResult {
			continue
		}
		var payload struct {
			Tool    string `json:"tool"`
			IsError bool   `json:"is_error"`
			Output  string `json:"output"`
		}
		if err := json.Unmarshal(ev.Payload, &payload); err != nil || payload.IsError || payload.Tool != "publish_app_change_package" {
			continue
		}
		var output map[string]any
		if err := json.Unmarshal([]byte(payload.Output), &output); err != nil {
			output = map[string]any{"raw_output": payload.Output}
		}
		output["loop_id"] = ev.RunID
		if fingerprint := appChangePackageResultFingerprint(output); fingerprint != "" {
			if seen[fingerprint] {
				continue
			}
			seen[fingerprint] = true
		}
		packages = append(packages, output)
	}
	return packages
}

func (rt *Runtime) mirrorWorkerAppChangePackages(ctx context.Context, client *http.Client, baseURL, ownerID string, packages []map[string]any) ([]map[string]any, []string) {
	if len(packages) == 0 {
		return packages, nil
	}
	out := make([]map[string]any, 0, len(packages))
	var mirrorErrors []string
	for _, pkg := range packages {
		item := copyStringAnyMap(pkg)
		packageID := appChangePackageResultString(item, "package_id")
		if packageID == "" {
			out = append(out, item)
			continue
		}
		if rt == nil || rt.store == nil {
			item["canonical_mirror_status"] = "failed"
			item["canonical_mirror_error"] = "active runtime store unavailable"
			mirrorErrors = append(mirrorErrors, packageID+": active runtime store unavailable")
			out = append(out, item)
			continue
		}
		rec, err := fetchInternalWorkerAppChangePackage(ctx, client, baseURL, ownerID, packageID)
		if err != nil {
			item["canonical_mirror_status"] = "failed"
			item["canonical_mirror_error"] = err.Error()
			mirrorErrors = append(mirrorErrors, packageID+": "+err.Error())
			out = append(out, item)
			continue
		}
		rec, err = rt.store.UpsertAppChangePackage(ctx, rec)
		if err != nil {
			item["canonical_mirror_status"] = "failed"
			item["canonical_mirror_error"] = err.Error()
			mirrorErrors = append(mirrorErrors, packageID+": "+err.Error())
			out = append(out, item)
			continue
		}
		item["canonical_mirror_status"] = "mirrored"
		item["product_visible"] = true
		item["canonical_package_id"] = rec.PackageID
		item["canonical_owner_id"] = rec.OwnerID
		item["runtime_source_delta_present"] = strings.TrimSpace(rec.RuntimeSourceDelta) != ""
		item["ui_source_delta_present"] = strings.TrimSpace(rec.UISourceDelta) != ""
		out = append(out, item)
	}
	return out, mirrorErrors
}

func fetchInternalWorkerAppChangePackage(ctx context.Context, client *http.Client, baseURL, ownerID, packageID string) (types.AppChangePackageRecord, error) {
	values := url.Values{"owner_id": []string{strings.TrimSpace(ownerID)}}
	endpoint, err := workerRuntimeURL(baseURL, "/internal/runtime/app-change-packages/"+url.PathEscape(packageID), values)
	if err != nil {
		return types.AppChangePackageRecord{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return types.AppChangePackageRecord{}, err
	}
	req.Header.Set("X-Internal-Caller", "true")
	resp, err := client.Do(req)
	if err != nil {
		return types.AppChangePackageRecord{}, fmt.Errorf("worker app change package detail: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return types.AppChangePackageRecord{}, err
	}
	if resp.StatusCode != http.StatusOK {
		return types.AppChangePackageRecord{}, fmt.Errorf("worker app change package detail failed: %s: %s", resp.Status, strings.TrimSpace(string(payload)))
	}
	var out types.AppChangePackageRecord
	if err := json.Unmarshal(payload, &out); err != nil {
		return types.AppChangePackageRecord{}, fmt.Errorf("decode worker app change package detail: %w", err)
	}
	if strings.TrimSpace(out.PackageID) == "" {
		return types.AppChangePackageRecord{}, fmt.Errorf("worker app change package detail missing package_id")
	}
	return out, nil
}

func annotateWorkerPackageMirrorResult(result map[string]any, packages []map[string]any, mirrorErrors []string) {
	if len(packages) == 0 {
		return
	}
	result["product_visible_app_change_package_count"] = countProductVisibleAppChangePackages(packages)
	if len(mirrorErrors) > 0 {
		result["app_change_package_mirror_errors"] = mirrorErrors
	}
}

func countProductVisibleAppChangePackages(packages []map[string]any) int {
	count := 0
	for _, pkg := range packages {
		if value, ok := pkg["product_visible"].(bool); ok && value {
			count++
			continue
		}
		if appChangePackageResultString(pkg, "canonical_mirror_status") == "mirrored" {
			count++
		}
	}
	return count
}

func copyStringAnyMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func appChangePackageResultFingerprint(output map[string]any) string {
	if packageID := appChangePackageResultString(output, "package_id"); packageID != "" {
		return "package_id:" + packageID
	}
	if sha := appChangePackageResultString(output, "runtime_source_delta_sha256"); sha != "" {
		return "runtime_delta_sha256:" + sha
	}
	if sha := appChangePackageResultString(output, "ui_source_delta_sha256"); sha != "" {
		return "ui_delta_sha256:" + sha
	}

	parts := make([]string, 0, 4)
	for _, key := range []string{"app_id", "base_sha", "worker_head_sha", "worker_head", "loop_id"} {
		if value := appChangePackageResultString(output, key); value != "" {
			parts = append(parts, key+"="+value)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "|")
}

func appChangePackageResultString(output map[string]any, key string) string {
	value, ok := output[key]
	if !ok || value == nil {
		return ""
	}
	switch value := value.(type) {
	case string:
		return strings.TrimSpace(value)
	case fmt.Stringer:
		return strings.TrimSpace(value.String())
	default:
		return strings.TrimSpace(fmt.Sprint(value))
	}
}

func summarizeWorkerRunEvents(events []types.EventRecord) []map[string]any {
	summary := make([]map[string]any, 0, len(events))
	for _, ev := range events {
		switch ev.Kind {
		case types.EventToolInvoked, types.EventToolResult, types.EventChannelMessage:
		default:
			continue
		}
		payload := map[string]any{}
		_ = json.Unmarshal(ev.Payload, &payload)
		item := map[string]any{
			"seq":        ev.Seq,
			"stream_seq": ev.StreamSeq,
			"kind":       ev.Kind,
		}
		if tool := payloadString(payload, "tool"); tool != "" {
			item["tool"] = tool
		}
		if isError, ok := payload["is_error"].(bool); ok {
			item["is_error"] = isError
		}
		if role := payloadString(payload, "role"); role != "" {
			item["role"] = role
		}
		if from := payloadString(payload, "from_agent_id"); from != "" {
			item["from_agent_id"] = from
		}
		if to := payloadString(payload, "to_agent_id"); to != "" {
			item["to_agent_id"] = to
		}
		if output := payloadString(payload, "output"); output != "" {
			item["output_excerpt"] = workerEventExcerpt(output, 700)
		}
		if content := payloadString(payload, "content"); content != "" {
			item["content_excerpt"] = workerEventExcerpt(content, 700)
		}
		summary = append(summary, item)
		if len(summary) >= 80 {
			break
		}
	}
	return summary
}

func collectWorkerSpawnProfiles(events []types.EventRecord) []string {
	seen := map[string]bool{}
	var profiles []string
	for _, ev := range events {
		if ev.Kind != types.EventToolResult {
			continue
		}
		payload := map[string]any{}
		if err := json.Unmarshal(ev.Payload, &payload); err != nil || payloadString(payload, "tool") != "spawn_agent" {
			continue
		}
		output := map[string]any{}
		if err := json.Unmarshal([]byte(payloadString(payload, "output")), &output); err != nil {
			continue
		}
		profile := firstNonEmpty(payloadString(output, "profile"), payloadString(output, "role"))
		if profile == "" || seen[profile] {
			continue
		}
		seen[profile] = true
		profiles = append(profiles, profile)
	}
	return profiles
}

func collectWorkerChildRunIDs(events []types.EventRecord) []string {
	seen := map[string]bool{}
	var runIDs []string
	for _, ev := range events {
		if ev.Kind != types.EventToolResult {
			continue
		}
		payload := map[string]any{}
		if err := json.Unmarshal(ev.Payload, &payload); err != nil || payloadString(payload, "tool") != "spawn_agent" {
			continue
		}
		output := map[string]any{}
		if err := json.Unmarshal([]byte(payloadString(payload, "output")), &output); err != nil {
			continue
		}
		runID := payloadString(output, "loop_id")
		if runID == "" || seen[runID] {
			continue
		}
		seen[runID] = true
		runIDs = append(runIDs, runID)
	}
	return runIDs
}

func countWorkerChannelMessages(events []types.EventRecord) int {
	count := 0
	for _, ev := range events {
		if ev.Kind == types.EventChannelMessage {
			count++
		}
	}
	return count
}

func workerEventExcerpt(text string, limit int) string {
	text = strings.TrimSpace(text)
	if limit <= 0 || len(text) <= limit {
		return text
	}
	return text[:limit] + fmt.Sprintf("…[%d bytes]", len(text))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func normalizeForkDesktopID(raw string) string {
	trimmed := strings.ToLower(strings.TrimSpace(raw))
	var b strings.Builder
	for _, r := range trimmed {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			continue
		}
		if r == '-' || r == '_' || r == ' ' {
			if b.Len() > 0 {
				b.WriteByte('-')
			}
		}
	}
	id := strings.Trim(b.String(), "-")
	if id == "" || id == types.PrimaryDesktopID {
		return "branch-" + uuid.New().String()[:8]
	}
	return id
}

func cloneDesktopState(state types.DesktopState) types.DesktopState {
	raw, err := json.Marshal(state)
	if err != nil {
		return state
	}
	var cloned types.DesktopState
	if err := json.Unmarshal(raw, &cloned); err != nil {
		return state
	}
	return cloned
}
