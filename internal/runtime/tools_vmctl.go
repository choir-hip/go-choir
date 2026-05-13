package runtime

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
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
		Description: "Request a headless worker VM under the current desktop and return a typed worker handle. Use delegate_worker_vm to run work inside it.",
		Parameters: jsonSchemaObject(map[string]any{
			"purpose":        map[string]any{"type": "string"},
			"machine_class":  map[string]any{"type": "string"},
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

			var trajectoryID string
			if runRec, _ := ctx.Value(toolCtxRunRecord).(*types.RunRecord); runRec != nil && runRec.Metadata != nil {
				if id, _ := runRec.Metadata[runMetadataTrajectoryID].(string); strings.TrimSpace(id) != "" {
					trajectoryID = strings.TrimSpace(id)
				}
			}

			client := vmctl.NewClient(rt.cfg.VmctlURL)
			handle, err := client.RequestWorker(vmctl.WorkerRequest{
				UserID:        ownerID,
				DesktopID:     desktopID,
				ParentAgentID: parentAgentID,
				TrajectoryID:  trajectoryID,
				Purpose:       strings.TrimSpace(in.Purpose),
				MachineClass:  strings.TrimSpace(in.MachineClass),
				AllowParallel: in.AllowParallel,
			})
			if err != nil {
				return "", err
			}

			return toolResultJSON(map[string]any{
				"status": "worker_requested",
				"handle": handle,
			})
		},
	}
}

func newDelegateWorkerVMTool(rt *Runtime, cwd string) Tool {
	type args struct {
		WorkerSandboxURL string `json:"worker_sandbox_url"`
		WorkerID         string `json:"worker_id,omitempty"`
		VMID             string `json:"vm_id,omitempty"`
		Objective        string `json:"objective"`
		Profile          string `json:"profile,omitempty"`
		TimeoutSeconds   int    `json:"timeout_seconds,omitempty"`
	}
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
			var in args
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
				timeout = 2 * time.Minute
			}
			if timeout > 15*time.Minute {
				timeout = 15 * time.Minute
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
			}

			startResp, err := submitInternalWorkerRun(ctx, client, in.WorkerSandboxURL, internalRunSubmitRequest{
				OwnerID:  ownerID,
				Prompt:   objective,
				Metadata: metadata,
			})
			if err != nil {
				return "", err
			}
			finalResp, err := pollInternalWorkerRun(ctx, client, in.WorkerSandboxURL, ownerID, startResp.RunID, timeout)
			if err != nil {
				return "", err
			}
			if finalResp.State != types.RunCompleted {
				return "", fmt.Errorf("worker run %s ended in state %s: %s", finalResp.RunID, finalResp.State, strings.TrimSpace(finalResp.Error))
			}
			eventsResp, err := fetchInternalWorkerRunEvents(ctx, client, in.WorkerSandboxURL, ownerID, startResp.RunID)
			if err != nil {
				return "", err
			}
			exports := collectExportPatchsetResults(eventsResp.Events)
			promotionCandidates, err := queuePromotionCandidatesForWorkerExports(ctx, rt, workerExportQueueContext{
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

			result := map[string]any{
				"status":             "worker_run_completed",
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
				"event_count":        len(eventsResp.Events),
			}
			if isolation.Enabled {
				result["worker_isolation"] = isolation.Kind
				result["worker_worktree_path"] = isolation.WorktreePath
				result["worker_branch"] = isolation.Branch
				result["worker_base_sha"] = isolation.BaseSHA
			}
			return toolResultJSON(result)
		},
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
		vmID := firstNonEmpty(in.WorkerVMID, exportString(export, "vm_id"), in.WorkerID, "worker-vm")
		candidateRunID := firstNonEmpty(exportString(export, "loop_id"), in.CandidateRunID)
		patchsetSHA256, err := patchsetDigest(exportString(export, "patchset_path"))
		if err != nil {
			return nil, err
		}
		if existing, ok, err := existingPromotionCandidateForWorkerExport(ctx, rt, in, export, objectiveFingerprint, patchsetSHA256); err != nil {
			return nil, err
		} else if ok {
			queued = append(queued, promotionCandidateQueueMap(existing))
			continue
		}
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
			return nil, fmt.Errorf("delegate_worker_vm status: %w", err)
		}
		payload, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			return nil, readErr
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("delegate_worker_vm status failed: %s: %s", resp.Status, strings.TrimSpace(string(payload)))
		}
		if err := json.Unmarshal(payload, &last); err != nil {
			return nil, fmt.Errorf("decode worker run status response: %w", err)
		}
		if last.State.Terminal() || last.State == types.RunBlocked {
			return &last, nil
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("worker run %s did not finish within %s; last state=%s", runID, timeout, last.State)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
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
		exports = append(exports, output)
	}
	return exports
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
