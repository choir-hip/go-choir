package runtime

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
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
	"unicode"

	"github.com/google/uuid"

	"github.com/yusefmosiah/go-choir/internal/promotion"
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
		Description: "Request a headless worker VM under the current desktop and return a typed worker handle. This only leases the worker; after a successful result, call delegate_worker_vm next using next_required_args plus the full execution objective. Supported machine classes are worker-small, worker-medium, and worker-large; omit machine_class for worker-small.",
		Parameters: jsonSchemaObject(map[string]any{
			"purpose":        map[string]any{"type": "string"},
			"machine_class":  map[string]any{"type": "string", "enum": []string{"worker-small", "worker-medium", "worker-large"}},
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
			cacheKey := workerVMRequestCacheKey(ownerID, desktopID, parentAgentID, parentRunID)
			if !in.AllowParallel && cacheKey != "" {
				rt.workerRequestMu.Lock()
				if cached := strings.TrimSpace(rt.workerRequests[cacheKey]); cached != "" {
					rt.workerRequestMu.Unlock()
					return markToolResultDeduped(cached, "super_run_already_requested_worker_vm")
				}
				if cached, ok, err := rt.findExistingWorkerVMRequest(ctx, parentRunID); err != nil {
					rt.workerRequestMu.Unlock()
					return "", err
				} else if ok {
					rt.workerRequests[cacheKey] = cached
					rt.workerRequestMu.Unlock()
					return markToolResultDeduped(cached, "super_run_already_requested_worker_vm")
				}
				handle, err := client.RequestWorker(vmctl.WorkerRequest{
					UserID:        ownerID,
					DesktopID:     desktopID,
					ParentAgentID: parentAgentID,
					TrajectoryID:  trajectoryID,
					Purpose:       strings.TrimSpace(in.Purpose),
					MachineClass:  machineClass,
					AllowParallel: false,
				})
				if err != nil {
					rt.workerRequestMu.Unlock()
					return "", err
				}
				result := workerVMRequestResult(handle)
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
		"delegation_required": true,
		"next_required_tool":  "delegate_worker_vm",
		"next_instruction":    "Call delegate_worker_vm next with next_required_args plus the full execution objective; do not stop after leasing the worker VM.",
	}
	if handle != nil {
		result["next_required_args"] = map[string]any{
			"worker_sandbox_url": handle.SandboxURL,
			"worker_id":          handle.WorkerID,
			"vm_id":              handle.VMID,
			"profile":            AgentProfileVSuper,
			"timeout_seconds":    int(defaultDelegateWorkerVMTimeout.Seconds()),
		}
	}
	return result
}

func workerVMRequestCacheKey(ownerID, desktopID, parentAgentID, parentRunID string) string {
	ownerID = strings.TrimSpace(ownerID)
	desktopID = strings.TrimSpace(desktopID)
	parentAgentID = strings.TrimSpace(parentAgentID)
	parentRunID = strings.TrimSpace(parentRunID)
	if ownerID == "" || desktopID == "" || parentAgentID == "" || parentRunID == "" {
		return ""
	}
	return ownerID + "\x00" + desktopID + "\x00" + parentAgentID + "\x00" + parentRunID
}

func (rt *Runtime) findExistingWorkerVMRequest(ctx context.Context, runID string) (string, bool, error) {
	if rt == nil || rt.store == nil || strings.TrimSpace(runID) == "" {
		return "", false, nil
	}
	eventsForRun, err := rt.store.ListEvents(ctx, runID, 500)
	if err != nil {
		return "", false, fmt.Errorf("request_worker_vm dedupe scan: %w", err)
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
		if err := json.Unmarshal(ev.Payload, &payload); err != nil || payload.IsError || payload.Tool != "request_worker_vm" {
			continue
		}
		var output map[string]any
		if err := json.Unmarshal([]byte(payload.Output), &output); err != nil {
			continue
		}
		status, _ := output["status"].(string)
		if strings.TrimSpace(status) == "worker_requested" && output["handle"] != nil {
			return payload.Output, true, nil
		}
	}
	return "", false, nil
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
	default:
		return strings.TrimSpace(raw)
	}
}

func newDelegateWorkerVMTool(rt *Runtime, cwd string) Tool {
	return Tool{
		Name:        "delegate_worker_vm",
		Description: "Start and monitor a vsuper, co-super, or researcher run inside a requested worker VM through internal runtime endpoints.",
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
					"status":             status,
					"worker_id":          strings.TrimSpace(in.WorkerID),
					"worker_vm_id":       strings.TrimSpace(in.VMID),
					"worker_sandbox_url": strings.TrimSpace(in.WorkerSandboxURL),
					"export_patchsets":   []map[string]any{},
					"promotion_queue":    []map[string]any{},
					"event_count":        0,
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
				if exports := collectExportPatchsetResults(evidence.Events); len(exports) > 0 {
					result["export_patchsets"] = exports
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
						result = resultWithWorkerEvents(result, workerRunID)
						result = checkpointDelegateResult(result, status)
						return toolResultJSON(result)
					}
					result := baseResult("worker_run_status_failed")
					result["loop_id"] = startResp.RunID
					result["error"] = err.Error()
					result["terminal_error"] = err.Error()
					result["attempt"] = attempt
					result = resultWithWorkerEvents(result, startResp.RunID)
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
			exports := collectExportPatchsetResults(evidence.Events)
			var promotionCandidates []map[string]any
			if finalResp.State == types.RunCompleted {
				promotionCandidates, err = queuePromotionCandidatesForWorkerExports(ctx, rt, workerExportQueueContext{
					OwnerID:             ownerID,
					ParentRunID:         runID,
					CandidateRunID:      finalResp.RunID,
					TraceID:             trajectoryID,
					WorkerVMID:          strings.TrimSpace(in.VMID),
					WorkerID:            strings.TrimSpace(in.WorkerID),
					ForegroundDesktopID: metadataStringValue(metadata, runMetadataDesktopID),
					Objective:           delegatedObjective,
					Exports:             exports,
				})
				if err != nil {
					return "", err
				}
			}

			result := map[string]any{
				"status":             delegateWorkerRunStatus(finalResp.State),
				"worker_id":          strings.TrimSpace(in.WorkerID),
				"worker_vm_id":       strings.TrimSpace(in.VMID),
				"worker_sandbox_url": strings.TrimSpace(in.WorkerSandboxURL),
				"loop_id":            finalResp.RunID,
				"agent_id":           finalResp.AgentID,
				"profile":            finalResp.AgentProfile,
				"state":              finalResp.State,
				"result":             finalResp.Result,
				"error":              finalResp.Error,
				"export_patchsets":   exports,
				"promotion_queue":    promotionCandidates,
			}
			applyWorkerRunEvidence(result, evidence)
			if profile == AgentProfileVSuper && finalResp.State == types.RunCompleted && vSuperDelegateIncomplete(evidence, exports) {
				result["status"] = "worker_run_incomplete"
				result["completion_blocker"] = "vsuper_completed_without_export_or_worker_update"
				result["terminal_error"] = "worker vsuper completed after child coordination without export_patchset or submit_worker_update evidence"
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

type workerExportQueueContext struct {
	OwnerID             string
	ParentRunID         string
	CandidateRunID      string
	TraceID             string
	WorkerVMID          string
	WorkerID            string
	ForegroundDesktopID string
	Objective           string
	Exports             []map[string]any
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
	remoteURL, baseSHA := remoteWorkerRepoBootstrapSourceFromGit(ctx, cwd)
	if remoteURL == "" || baseSHA == "" {
		remoteURL, baseSHA = remoteWorkerRepoBootstrapSourceFromEnv()
	}
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
		"Use set -euo pipefail for multi-step bash commands so a failed commit, test, or export cannot be hidden by a later successful command.",
		"Commit candidate changes before calling export_patchset.",
		"Use repo_path \"go-choir-candidate\" and base_sha " + baseSHA + " when exporting a patchset.",
		"If clone, checkout, build, or export fails, report diagnostics with submit_worker_update instead of claiming repository work.",
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
		"- Set spawn_agent slot=\"implementation\" for the implementation worker and slot=\"verifier\" for the verifier, and put the role plus terminal obligation directly in each objective; do not depend on a later role-correction cast as the child's first authoritative instruction.",
		"- If you spawn an implementation co-super, treat that child as the exclusive writer for go-choir-candidate while it is active; do not run reset, clean, edit, or commit commands in the same checkout until the child reports commit/export/blocker evidence.",
		"- Do not cancel a child that has produced export_patchset evidence. Incorporate the child export instead.",
		"- The verifier should inspect only after the implementation child has reported a commit, export, or blocker; avoid racing the worker by repeatedly reading a checkout that is still being mutated.",
		"- If the objective asks a helper to export, do not override that with \"do not export\"; let the helper export, then report that child export.",
		"- Once a committed repo diff and focused verification evidence exist, make exactly one export_patchset call for the candidate. If a child already exported, do not parent-export again.",
		"- Starting children, casting assignments, or receiving acknowledgement-only messages is not a terminal result; wait for commit/export/verifier/blocker evidence, or submit_worker_update with the precise missing-evidence blocker.",
		"- Reserve the last " + reserve.String() + " of the delegate budget for exactly one terminal action: export_patchset or submit_worker_update with a precise blocker.",
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
		"Commit any repo changes in this worktree before calling export_patchset.",
		"Use repo_path \".\" and base_sha " + baseSHA + " when exporting a patchset.",
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

func objectiveFingerprint(ownerID, trajectoryID, parentRunID, objective string) string {
	parts := []string{
		strings.TrimSpace(ownerID),
		strings.TrimSpace(trajectoryID),
		strings.TrimSpace(parentRunID),
		normalizeObjectiveText(objective),
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:])
}

func normalizeObjectiveText(raw string) string {
	var b strings.Builder
	lastSpace := false
	for _, r := range strings.ToLower(strings.TrimSpace(raw)) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastSpace = false
			continue
		}
		if !lastSpace && b.Len() > 0 {
			b.WriteByte(' ')
			lastSpace = true
		}
	}
	return strings.TrimSpace(b.String())
}

func patchsetDigest(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", nil
	}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("read patchset digest: %w", err)
	}
	defer func() { _ = f.Close() }()
	hash := sha256.New()
	if _, err := io.Copy(hash, f); err != nil {
		return "", fmt.Errorf("hash patchset: %w", err)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func queuePromotionCandidatesForWorkerExports(ctx context.Context, rt *Runtime, in workerExportQueueContext) ([]map[string]any, error) {
	if rt == nil || len(in.Exports) == 0 {
		return nil, nil
	}
	objectiveFingerprint := objectiveFingerprint(in.OwnerID, in.TraceID, in.ParentRunID, in.Objective)
	queued := make([]map[string]any, 0, len(in.Exports))
	for _, export := range in.Exports {
		candidateID := uuid.NewString()
		patchsetSHA256 := strings.TrimSpace(exportString(export, "patchset_sha256"))
		if patchsetSHA256 == "" {
			if content := exportRawString(export, "patchset_content"); content != "" {
				sum := sha256.Sum256([]byte(content))
				patchsetSHA256 = hex.EncodeToString(sum[:])
			}
		}
		if patchsetSHA256 == "" {
			var err error
			patchsetSHA256, err = patchsetDigest(exportString(export, "patchset_path"))
			if err != nil {
				return nil, err
			}
		}
		if existing, ok, err := existingPromotionCandidateForWorkerExport(ctx, rt, in, export, objectiveFingerprint, patchsetSHA256); err != nil {
			return nil, err
		} else if ok {
			queued = append(queued, promotionCandidateQueueMap(existing))
			continue
		}
		materializedExport, err := materializeWorkerExportArtifacts(rt, candidateID, export)
		if err != nil {
			return nil, err
		}
		export = materializedExport
		if patchsetSHA256 == "" {
			patchsetSHA256 = strings.TrimSpace(exportString(export, "patchset_sha256"))
		}
		vmID := firstNonEmpty(in.WorkerVMID, exportString(export, "vm_id"), in.WorkerID, "worker-vm")
		candidateRunID := firstNonEmpty(exportString(export, "loop_id"), in.CandidateRunID)
		candidate := promotion.CandidateWorld{
			CandidateID:          candidateID,
			OwnerID:              in.OwnerID,
			ForegroundDesktopID:  in.ForegroundDesktopID,
			ParentRunID:          in.ParentRunID,
			CandidateRunID:       candidateRunID,
			VMID:                 vmID,
			SnapshotID:           exportString(export, "snapshot_id"),
			Purpose:              in.Objective,
			ObjectiveFingerprint: objectiveFingerprint,
			BaseSHA:              exportString(export, "base_sha"),
			WorkerHeadSHA:        firstNonEmpty(exportString(export, "worker_head_sha"), exportString(export, "worker_head")),
			PatchsetSHA256:       patchsetSHA256,
			ManifestPath:         exportString(export, "manifest_path"),
			PatchsetPath:         exportString(export, "patchset_path"),
			IntegrationBranch:    "agent/" + sanitizeExportPart(candidateRunID) + "/candidate",
			CreatedAt:            time.Now().UTC().Format(time.RFC3339),
		}
		candidateJSON, err := json.Marshal(candidate)
		if err != nil {
			return nil, fmt.Errorf("marshal queued promotion candidate: %w", err)
		}
		rec, err := rt.QueuePromotionCandidate(ctx, types.PromotionCandidateRecord{
			CandidateID:       candidateID,
			OwnerID:           in.OwnerID,
			Status:            types.PromotionCandidateQueued,
			SourceRunID:       in.ParentRunID,
			TraceID:           in.TraceID,
			VMID:              vmID,
			SnapshotID:        candidate.SnapshotID,
			BaseSHA:           candidate.BaseSHA,
			WorkerHeadSHA:     candidate.WorkerHeadSHA,
			ManifestPath:      candidate.ManifestPath,
			PatchsetPath:      candidate.PatchsetPath,
			IntegrationBranch: candidate.IntegrationBranch,
			DestinationBranch: "main",
			Summary:           in.Objective,
			CandidateJSON:     candidateJSON,
			ContractsJSON:     json.RawMessage(`[]`),
			ReportJSON:        json.RawMessage(`{}`),
		})
		if err != nil {
			return nil, fmt.Errorf("queue promotion candidate for worker export: %w", err)
		}
		queued = append(queued, promotionCandidateQueueMap(rec))
	}
	return queued, nil
}

func materializeWorkerExportArtifacts(rt *Runtime, candidateID string, export map[string]any) (map[string]any, error) {
	if len(export) == 0 {
		return export, nil
	}
	manifestContent := exportRawString(export, "manifest_json")
	patchsetContent := exportRawString(export, "patchset_content")
	if strings.TrimSpace(manifestContent) == "" && strings.TrimSpace(patchsetContent) == "" {
		return export, nil
	}
	root := promotionArtifactRoot(rt)
	if root == "" {
		return export, nil
	}
	dir := filepath.Join(root, sanitizeExportPart(candidateID))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create promotion artifact dir: %w", err)
	}
	out := make(map[string]any, len(export)+3)
	for key, value := range export {
		out[key] = value
	}
	if strings.TrimSpace(manifestContent) != "" {
		manifestPath := filepath.Join(dir, "manifest.json")
		if err := os.WriteFile(manifestPath, []byte(manifestContent), 0o644); err != nil {
			return nil, fmt.Errorf("write materialized manifest: %w", err)
		}
		out["manifest_path"] = manifestPath
	}
	if strings.TrimSpace(patchsetContent) != "" {
		patchPath := filepath.Join(dir, "changes.patch")
		if err := os.WriteFile(patchPath, []byte(patchsetContent), 0o644); err != nil {
			return nil, fmt.Errorf("write materialized patchset: %w", err)
		}
		sum := sha256.Sum256([]byte(patchsetContent))
		out["patchset_path"] = patchPath
		out["patchset_sha256"] = hex.EncodeToString(sum[:])
	}
	return out, nil
}

func promotionArtifactRoot(rt *Runtime) string {
	base := ""
	if rt != nil {
		base = filepath.Dir(strings.TrimSpace(rt.cfg.StorePath))
	}
	if strings.TrimSpace(base) == "" || base == "." {
		base = filepath.Join(os.TempDir(), "go-choir-promotion-artifacts")
	}
	return filepath.Join(base, "promotion-artifacts")
}

func exportRawString(export map[string]any, key string) string {
	if export == nil {
		return ""
	}
	value, _ := export[key].(string)
	return value
}

func existingPromotionCandidateForWorkerExport(ctx context.Context, rt *Runtime, in workerExportQueueContext, export map[string]any, objectiveFingerprint, patchsetSHA256 string) (types.PromotionCandidateRecord, bool, error) {
	workerHead := firstNonEmpty(exportString(export, "worker_head_sha"), exportString(export, "worker_head"))
	patchsetPath := exportString(export, "patchset_path")
	baseSHA := exportString(export, "base_sha")
	manifestPath := exportString(export, "manifest_path")
	if strings.TrimSpace(workerHead) == "" || strings.TrimSpace(patchsetPath) == "" || strings.TrimSpace(baseSHA) == "" {
		return types.PromotionCandidateRecord{}, false, nil
	}
	candidates, err := rt.store.ListPromotionCandidates(ctx, in.OwnerID, 500)
	if err != nil {
		return types.PromotionCandidateRecord{}, false, fmt.Errorf("list promotion candidates for worker export dedupe: %w", err)
	}
	for _, rec := range candidates {
		if rec.SourceRunID == in.ParentRunID &&
			rec.BaseSHA == baseSHA &&
			rec.WorkerHeadSHA == workerHead &&
			rec.PatchsetPath == patchsetPath &&
			rec.ManifestPath == manifestPath {
			return rec, true, nil
		}
		var candidate promotion.CandidateWorld
		if len(rec.CandidateJSON) == 0 || json.Unmarshal(rec.CandidateJSON, &candidate) != nil {
			continue
		}
		if rec.SourceRunID == in.ParentRunID &&
			rec.BaseSHA == baseSHA &&
			strings.TrimSpace(objectiveFingerprint) != "" &&
			strings.TrimSpace(patchsetSHA256) != "" &&
			candidate.ObjectiveFingerprint == objectiveFingerprint &&
			candidate.PatchsetSHA256 == patchsetSHA256 {
			return rec, true, nil
		}
	}
	return types.PromotionCandidateRecord{}, false, nil
}

func promotionCandidateQueueMap(rec types.PromotionCandidateRecord) map[string]any {
	out := map[string]any{
		"candidate_id":       rec.CandidateID,
		"status":             rec.Status,
		"source_loop_id":     rec.SourceRunID,
		"candidate_loop_id":  candidateLoopIDForPromotionRecord(rec),
		"vm_id":              rec.VMID,
		"base_sha":           rec.BaseSHA,
		"worker_head":        rec.WorkerHeadSHA,
		"manifest_path":      rec.ManifestPath,
		"patchset_path":      rec.PatchsetPath,
		"integration_branch": rec.IntegrationBranch,
		"destination_branch": rec.DestinationBranch,
	}
	if len(rec.CandidateJSON) != 0 {
		var candidate promotion.CandidateWorld
		if err := json.Unmarshal(rec.CandidateJSON, &candidate); err == nil {
			if candidate.ObjectiveFingerprint != "" {
				out["objective_fingerprint"] = candidate.ObjectiveFingerprint
			}
			if candidate.PatchsetSHA256 != "" {
				out["patchset_sha256"] = candidate.PatchsetSHA256
			}
		}
	}
	return out
}

func candidateLoopIDForPromotionRecord(rec types.PromotionCandidateRecord) string {
	if len(rec.CandidateJSON) == 0 {
		return ""
	}
	var candidate promotion.CandidateWorld
	if err := json.Unmarshal(rec.CandidateJSON, &candidate); err != nil {
		return ""
	}
	return strings.TrimSpace(candidate.CandidateRunID)
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
			if isRetryableWorkerStatusCode(resp.StatusCode) && time.Now().Before(deadline) {
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

func isRetryableWorkerStatusCode(statusCode int) bool {
	switch statusCode {
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
	}
	return evidence, nil
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
		evidence.ChildRunStates = states
		evidence.ChildStatusErrors = statusErrors
		evidence.ChildEventErrors["_refresh"] = err.Error()
		return evidence
	}
	refreshed.ChildRunStates = states
	refreshed.ChildStatusErrors = statusErrors
	return refreshed
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

func vSuperDelegateIncomplete(evidence workerRunEvidence, exports []map[string]any) bool {
	if len(evidence.ChildRunIDs) == 0 || len(exports) > 0 {
		return false
	}
	return !hasSuccessfulToolResult(evidence.Events, "submit_worker_update")
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

func collectExportPatchsetResults(events []types.EventRecord) []map[string]any {
	var exports []map[string]any
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
		if err := json.Unmarshal(ev.Payload, &payload); err != nil || payload.IsError || payload.Tool != "export_patchset" {
			continue
		}
		var output map[string]any
		if err := json.Unmarshal([]byte(payload.Output), &output); err != nil {
			output = map[string]any{"raw_output": payload.Output}
		}
		output["loop_id"] = ev.RunID
		if fingerprint := workerExportPatchsetResultFingerprint(output); fingerprint != "" {
			if seen[fingerprint] {
				continue
			}
			seen[fingerprint] = true
		}
		exports = append(exports, output)
	}
	return exports
}

func workerExportPatchsetResultFingerprint(output map[string]any) string {
	if sha := workerExportPatchsetResultString(output, "patchset_sha256"); sha != "" {
		return "patchset_sha256:" + sha
	}
	if patchsetPath := workerExportPatchsetResultString(output, "patchset_path"); patchsetPath != "" {
		return "patchset_path:" + patchsetPath
	}
	if manifestPath := workerExportPatchsetResultString(output, "manifest_path"); manifestPath != "" {
		return "manifest_path:" + manifestPath
	}

	parts := make([]string, 0, 4)
	for _, key := range []string{"base_sha", "worker_head_sha", "worker_head", "loop_id"} {
		if value := workerExportPatchsetResultString(output, key); value != "" {
			parts = append(parts, key+"="+value)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "|")
}

func workerExportPatchsetResultString(output map[string]any, key string) string {
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

func exportString(export map[string]any, key string) string {
	value, _ := export[key].(string)
	return strings.TrimSpace(value)
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
