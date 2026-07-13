package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/buildinfo"
	"github.com/yusefmosiah/go-choir/internal/types"
)

type runAcceptanceSynthesizeInput struct {
	TargetMissionID       string
	SourcePromptObjective string
	TrajectoryID          string
	RunID                 string
	CIRunID               string
	DeployRunID           string
	StagingURL            string
}

const (
	runAcceptanceCheckpointTextureOpened = "texture_opened"
)

type acceptanceBuilder struct {
	record      types.RunAcceptanceRecord
	evidence    []types.RunAcceptanceEvidenceRef
	evidenceSet map[string]bool
}

type acceptanceToolResult struct {
	tool   string
	event  types.EventRecord
	output map[string]any
}

type acceptanceToolError struct {
	tool   string
	event  types.EventRecord
	output string
}

type acceptanceToolInvocation struct {
	tool      string
	event     types.EventRecord
	callID    string
	arguments map[string]any
}

// SynthesizeRunAcceptance derives a durable run acceptance record from
// product-path evidence already present in runs, Trace events, worker
// AppChangePackages, and recipient adoption records. The caller chooses the
// target trajectory; the verifier chooses checkpoints.
func (rt *Runtime) SynthesizeRunAcceptance(ctx context.Context, ownerID string, in runAcceptanceSynthesizeInput) (types.RunAcceptanceRecord, error) {
	if rt == nil || rt.store == nil {
		return types.RunAcceptanceRecord{}, fmt.Errorf("synthesize run acceptance: runtime store is unavailable")
	}
	ownerID = strings.TrimSpace(ownerID)
	if ownerID == "" {
		return types.RunAcceptanceRecord{}, fmt.Errorf("synthesize run acceptance: user_id is required")
	}
	in.TargetMissionID = strings.TrimSpace(in.TargetMissionID)
	if in.TargetMissionID == "" {
		return types.RunAcceptanceRecord{}, fmt.Errorf("synthesize run acceptance: target_mission_id is required")
	}
	in.TrajectoryID = strings.TrimSpace(in.TrajectoryID)
	in.RunID = strings.TrimSpace(in.RunID)
	if in.TrajectoryID == "" && in.RunID == "" {
		return types.RunAcceptanceRecord{}, fmt.Errorf("synthesize run acceptance: trajectory_id or loop_id is required")
	}
	if in.TrajectoryID == "" {
		run, err := rt.store.GetRun(ctx, in.RunID)
		if err != nil {
			return types.RunAcceptanceRecord{}, fmt.Errorf("synthesize run acceptance: load source run: %w", err)
		}
		if run.OwnerID != ownerID {
			return types.RunAcceptanceRecord{}, fmt.Errorf("synthesize run acceptance: source run not found")
		}
		in.TrajectoryID = traceTrajectoryIDForRun(run)
	}

	runs, err := rt.store.ListRunsByOwner(ctx, ownerID, 1000)
	if err != nil {
		return types.RunAcceptanceRecord{}, fmt.Errorf("synthesize run acceptance: list runs: %w", err)
	}
	var trajectoryRuns []types.RunRecord
	for _, run := range runs {
		if traceTrajectoryIDForRun(run) == in.TrajectoryID {
			trajectoryRuns = append(trajectoryRuns, run)
		}
	}
	sort.Slice(trajectoryRuns, func(i, j int) bool {
		return trajectoryRuns[i].CreatedAt.Before(trajectoryRuns[j].CreatedAt)
	})

	events, err := rt.store.ListEventsByTrajectory(ctx, ownerID, in.TrajectoryID, 3000)
	if err != nil {
		return types.RunAcceptanceRecord{}, fmt.Errorf("synthesize run acceptance: list trajectory events: %w", err)
	}
	sort.Slice(events, func(i, j int) bool {
		return events[i].StreamSeq < events[j].StreamSeq
	})

	root := chooseAcceptanceRootRun(trajectoryRuns, in.RunID)
	if root.RunID == "" && len(events) == 0 {
		return types.RunAcceptanceRecord{}, fmt.Errorf("synthesize run acceptance: trajectory not found")
	}

	build := buildinfo.Snapshot("sandbox")
	deploymentCommit := acceptanceServingCommit(build)
	sourceObjective := strings.TrimSpace(in.SourcePromptObjective)
	if sourceObjective == "" {
		sourceObjective = root.Prompt
	}
	runID := firstNonEmpty(in.RunID, root.RunID)
	desktopID := firstNonEmpty(traceRunMetadataString(root, runMetadataDesktopID), types.PrimaryDesktopID)

	builder := acceptanceBuilder{
		record: types.RunAcceptanceRecord{
			AcceptanceID:            stableRunAcceptanceID(ownerID, in.TargetMissionID, in.TrajectoryID),
			TargetMissionID:         in.TargetMissionID,
			SourcePromptObjective:   sourceObjective,
			OwnerID:                 ownerID,
			DesktopID:               desktopID,
			TrajectoryID:            in.TrajectoryID,
			RunID:                   runID,
			AuthorityProfile:        acceptanceAuthorityProfile(trajectoryRuns),
			DeploymentCommit:        deploymentCommit,
			CIRunID:                 strings.TrimSpace(in.CIRunID),
			DeployRunID:             strings.TrimSpace(in.DeployRunID),
			StagingURL:              strings.TrimSpace(in.StagingURL),
			HealthCommit:            deploymentCommit,
			AcceptanceLevel:         types.RunAcceptanceDocsLevel,
			GatewayProviderEvidence: acceptanceGatewayProviderEvidence(rt, events),
			State:                   types.RunAcceptanceBlocked,
			FailureResidualRisks:    []string{},
			Checkpoints:             []types.RunAcceptanceCheckpoint{},
			InvariantChecks:         []types.RunAcceptanceInvariantCheck{},
			VerifierContracts:       []types.RunAcceptanceVerifierContract{},
			EvidenceRefs:            []types.RunAcceptanceEvidenceRef{},
			RollbackRefs:            []types.RunAcceptanceRollbackRef{},
		},
		evidenceSet: map[string]bool{},
	}

	submittedRef := builder.addRunEvidence(root, "source prompt-bar/conductor run")
	if root.RunID != "" {
		builder.addCheckpoint("submitted", "passed", root.CreatedAt, 0, []string{submittedRef}, map[string]any{
			"loop_id": root.RunID,
			"profile": traceRunProfile(root),
		})
	}

	if docID, at, refID := acceptanceDocumentEvidence(&builder, root, events); docID != "" {
		builder.addCheckpoint(runAcceptanceCheckpointTextureOpened, "passed", at, 0, []string{refID}, map[string]any{"doc_id": docID})
	}

	superResults := collectAcceptanceToolResults(events, "request_super_execution")
	if len(superResults) > 0 {
		item := superResults[0]
		ref := builder.addEventEvidence(item.event, "Texture requested persistent super execution", map[string]any{
			"tool":    "request_super_execution",
			"loop_id": payloadString(item.output, "loop_id"),
		})
		builder.addCheckpoint("super_requested", "passed", item.event.Timestamp, item.event.StreamSeq, []string{ref}, map[string]any{
			"super_loop_id": payloadString(item.output, "loop_id"),
			"agent_id":      payloadString(item.output, "agent_id"),
		})
	}

	workerResults := collectAcceptanceToolResults(events, "request_worker_vm")
	if len(workerResults) > 0 {
		item := workerResults[0]
		handle, _ := item.output["handle"].(map[string]any)
		if handle != nil {
			desktopID = firstNonEmpty(payloadString(handle, "desktop_id"), builder.record.DesktopID)
			builder.record.DesktopID = desktopID
			builder.record.VMMode = firstNonEmpty(payloadString(handle, "kind"), "worker")
		}
		ref := builder.addEventEvidence(item.event, "super leased a worker VM through vmctl", map[string]any{
			"tool":        "request_worker_vm",
			"vm_id":       payloadString(handle, "vm_id"),
			"worker_id":   payloadString(handle, "worker_id"),
			"sandbox_url": payloadString(handle, "sandbox_url"),
		})
		builder.addCheckpoint("worker_leased", "passed", item.event.Timestamp, item.event.StreamSeq, []string{ref}, map[string]any{
			"vm_id":         payloadString(handle, "vm_id"),
			"worker_id":     payloadString(handle, "worker_id"),
			"machine_class": payloadString(handle, "machine_class"),
		})
	}

	workerDelegationTools := []string{"delegate_worker_vm", "start_worker_delegation", "observe_worker_delegation", "finish_worker_delegation"}
	delegateResults := collectAcceptanceToolResultsAny(events, workerDelegationTools...)
	var packageRefs []string
	var nonPackageDelegateResults []acceptanceToolResult
	packageCount := 0
	for _, item := range delegateResults {
		packages := acceptanceOutputSlice(item.output, "app_change_packages")
		status := payloadString(item.output, "status")
		if len(packages) == 0 {
			if acceptanceDelegateRuntimeSupervisionPassed(item.output) {
				delegateDetails := acceptanceDelegateWorkerDetails(item.tool, item.output, nil)
				delegateDetails["acceptance_contract"] = "runtime_supervision"
				ref := builder.addEventEvidence(item.event, "worker run completed with live worker-update supervision evidence", delegateDetails)
				builder.addCheckpoint("worker_delegated", "passed", item.event.Timestamp, item.event.StreamSeq, []string{ref}, delegateDetails)
				builder.addCheckpoint("worker_supervision_observed", "passed", item.event.Timestamp, item.event.StreamSeq, []string{ref}, map[string]any{
					"worker_update_checkpoint": delegateDetails["worker_update_checkpoint"],
					"worker_update_count":      delegateDetails["mirrored_worker_update_count"],
					"worker_loop_id":           delegateDetails["worker_loop_id"],
				})
				continue
			}
			nonPackageDelegateResults = append(nonPackageDelegateResults, item)
			continue
		}
		packageCount += len(packages)
		if mode := payloadString(item.output, "worker_isolation"); mode != "" {
			builder.record.VMMode = mode
		}
		for _, pkg := range packages {
			if builder.record.BaseSHA == "" {
				builder.record.BaseSHA = payloadString(pkg, "base_sha")
			}
		}
		delegateDetails := acceptanceDelegateWorkerDetails(item.tool, item.output, packages)
		evidenceSummary := "worker run published concrete AppChangePackage evidence"
		if status != "" && status != "worker_run_completed" {
			delegateDetails["non_clean_delegate_status"] = status
			evidenceSummary = "worker run returned reviewable AppChangePackage evidence with non-clean delegate status"
		}
		ref := builder.addEventEvidence(item.event, evidenceSummary, delegateDetails)
		packageRefs = append(packageRefs, ref)
		builder.addCheckpoint("worker_delegated", "passed", item.event.Timestamp, item.event.StreamSeq, []string{ref}, delegateDetails)
	}
	if packageCount == 0 {
		delegateErrors := collectAcceptanceToolErrorsAny(events, workerDelegationTools...)
		pendingDelegates := collectAcceptancePendingToolInvocationsAny(events, workerDelegationTools...)
		var refs []string
		details := map[string]any{}
		var blockedAt time.Time
		var blockedSeq int64
		rememberLatest := func(ev types.EventRecord) {
			if blockedAt.IsZero() || ev.Timestamp.After(blockedAt) {
				blockedAt = ev.Timestamp
				blockedSeq = ev.StreamSeq
			}
		}
		if len(nonPackageDelegateResults) > 0 {
			for _, item := range nonPackageDelegateResults {
				ref := builder.addEventEvidence(item.event, "worker VM delegation returned without AppChangePackage evidence", acceptanceDelegateWorkerDetails(item.tool, item.output, nil))
				refs = append(refs, ref)
				rememberLatest(item.event)
			}
			last := nonPackageDelegateResults[len(nonPackageDelegateResults)-1]
			details["result_count"] = len(nonPackageDelegateResults)
			details["last_result_status"] = payloadString(last.output, "status")
			details["last_result_state"] = payloadString(last.output, "state")
			details["worker_id"] = payloadString(last.output, "worker_id")
			details["worker_vm_id"] = payloadString(last.output, "worker_vm_id")
			details["worker_loop_id"] = payloadString(last.output, "loop_id")
			details["worker_sandbox_url"] = payloadString(last.output, "worker_sandbox_url")
			if errText := payloadString(last.output, "error"); errText != "" {
				details["last_worker_error"] = errText
			}
			if terminal := payloadString(last.output, "terminal_error"); terminal != "" {
				details["terminal_error"] = terminal
			}
			if blocker := payloadString(last.output, "completion_blocker"); blocker != "" {
				details["completion_blocker"] = blocker
			}
			if summary := last.output["worker_event_summary"]; summary != nil {
				details["worker_event_summary"] = summary
			}
			if profiles := last.output["worker_spawned_profiles"]; profiles != nil {
				details["worker_spawned_profiles"] = profiles
			}
			if count := last.output["worker_channel_message_count"]; count != nil {
				details["worker_channel_message_count"] = count
			}
			if eventCount := last.output["event_count"]; eventCount != nil {
				details["event_count"] = eventCount
			}
			if childRunIDs := acceptanceStringSlice(last.output, "worker_child_run_ids"); len(childRunIDs) > 0 {
				details["worker_child_run_ids"] = childRunIDs
			}
			if counts := acceptanceStringAnyMap(last.output, "worker_child_event_counts"); len(counts) > 0 {
				details["worker_child_event_counts"] = counts
			}
			if errors := acceptanceStringAnyMap(last.output, "worker_child_event_errors"); len(errors) > 0 {
				details["worker_child_event_errors"] = errors
			}
			if states := acceptanceStringAnyMap(last.output, "worker_child_run_states"); len(states) > 0 {
				details["worker_child_run_states"] = states
			}
			if errors := acceptanceStringAnyMap(last.output, "worker_child_status_errors"); len(errors) > 0 {
				details["worker_child_status_errors"] = errors
			}
			if details["last_result_status"] != "" {
				details["status"] = details["last_result_status"]
			}
		}
		if len(delegateErrors) > 0 {
			for _, item := range delegateErrors {
				ref := builder.addEventEvidence(item.event, "worker VM delegation failed before AppChangePackage publication", map[string]any{
					"tool":  item.tool,
					"error": item.output,
				})
				refs = append(refs, ref)
				rememberLatest(item.event)
			}
			last := delegateErrors[len(delegateErrors)-1]
			details["error_count"] = len(delegateErrors)
			details["last_error"] = last.output
		}
		if len(pendingDelegates) > 0 {
			for _, item := range pendingDelegates {
				ref := builder.addEventEvidence(item.event, "worker VM delegation was invoked without a terminal tool result", map[string]any{
					"tool":               item.tool,
					"call_id":            item.callID,
					"worker_id":          payloadString(item.arguments, "worker_id"),
					"worker_vm_id":       payloadString(item.arguments, "vm_id"),
					"worker_sandbox_url": payloadString(item.arguments, "worker_sandbox_url"),
				})
				refs = append(refs, ref)
				rememberLatest(item.event)
			}
			last := pendingDelegates[len(pendingDelegates)-1]
			details["pending_invocation_count"] = len(pendingDelegates)
			details["last_call_id"] = last.callID
			details["last_pending_worker_id"] = payloadString(last.arguments, "worker_id")
			details["last_pending_worker_vm_id"] = payloadString(last.arguments, "vm_id")
			details["last_pending_worker_sandbox_url"] = payloadString(last.arguments, "worker_sandbox_url")
			details["pending_status"] = "invoked_without_terminal_result"
			if len(nonPackageDelegateResults) == 0 && len(delegateErrors) == 0 {
				details["worker_id"] = payloadString(last.arguments, "worker_id")
				details["worker_vm_id"] = payloadString(last.arguments, "vm_id")
				details["worker_sandbox_url"] = payloadString(last.arguments, "worker_sandbox_url")
				details["status"] = "invoked_without_terminal_result"
			}
		}
		if len(refs) > 0 && !acceptanceRecordHasPassedCheckpoint(builder.record, "worker_supervision_observed") {
			builder.addCheckpoint("worker_delegated", "blocked", blockedAt, blockedSeq, refs, details)
		}
	}
	if packageCount > 0 {
		builder.addCheckpoint("app_package_published", "passed", time.Now().UTC(), 0, packageRefs, map[string]any{"package_count": packageCount})
	}
	addAcceptanceAppPromotionCheckpoints(&builder, events)

	addAcceptanceContinuationAndCompactionCheckpoints(&builder, events)

	builder.record.AcceptanceLevel, builder.record.State = acceptanceLevelAndState(builder.record.Checkpoints)
	builder.record.EvidenceRefs = builder.evidence
	builder.record.InvariantChecks = buildAcceptanceInvariantChecks(builder.record)
	if builder.record.State == types.RunAcceptanceAccepted && acceptanceHasBlockedInvariant(builder.record.InvariantChecks) {
		builder.record.State = types.RunAcceptanceBlocked
	}
	builder.record.VerifierContracts = buildAcceptanceVerifierContracts(builder.record)
	builder.record.FailureResidualRisks = buildAcceptanceResidualRisks(builder.record)
	rec, err := rt.store.UpsertRunAcceptance(ctx, builder.record)
	if err != nil {
		return types.RunAcceptanceRecord{}, err
	}
	return rec, nil
}

// acceptanceServingCommit records the immutable serving artifact when it is
// available. Deploy metadata is a release target/receipt and cannot override
// the commit compiled into the process that synthesized the acceptance.
func acceptanceServingCommit(build buildinfo.Info) string {
	return firstNonEmpty(build.Commit, build.DeployedCommit)
}

func stableRunAcceptanceID(ownerID, missionID, trajectoryID string) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{ownerID, missionID, trajectoryID}, "\x00")))
	return "runacc-" + hex.EncodeToString(sum[:])[:20]
}

func chooseAcceptanceRootRun(runs []types.RunRecord, runID string) types.RunRecord {
	if runID != "" {
		for _, run := range runs {
			if run.RunID == runID {
				return run
			}
		}
	}
	for _, run := range runs {
		if traceRunProfile(run) == agentprofile.Conductor && metadataStringValue(run.Metadata, "input_source") == "prompt_bar" {
			return run
		}
	}
	if len(runs) > 0 {
		return runs[0]
	}
	return types.RunRecord{}
}

func acceptanceAuthorityProfile(runs []types.RunRecord) string {
	seen := map[string]bool{}
	var roles []string
	for _, run := range runs {
		role := traceRunRole(run)
		if role == "" {
			role = traceRunProfile(run)
		}
		if role == "" || seen[role] {
			continue
		}
		seen[role] = true
		roles = append(roles, role)
	}
	return strings.Join(roles, " > ")
}

func acceptanceGatewayProviderEvidence(rt *Runtime, events []types.EventRecord) string {
	provider := "unknown"
	if rt != nil && rt.provider != nil {
		provider = rt.provider.ProviderName()
	}
	search := buildTraceSearchSummary(events)
	if search.Attempts == 0 {
		return "active_provider=" + provider
	}
	return fmt.Sprintf("active_provider=%s; search_attempts=%d; search_successes=%d; search_rate_limits=%d", provider, search.Attempts, search.Successes, search.RateLimits)
}

func acceptanceDocumentEvidence(builder *acceptanceBuilder, root types.RunRecord, events []types.EventRecord) (string, time.Time, string) {
	if root.Result != "" {
		var decision map[string]any
		if json.Unmarshal([]byte(root.Result), &decision) == nil {
			docID := payloadString(decision, "doc_id")
			if docID != "" {
				ref := builder.addRunEvidence(root, "conductor decision opened Texture document")
				return docID, root.UpdatedAt, ref
			}
		}
	}
	for _, ev := range events {
		if ev.Kind != types.EventTextureDocumentRevisionCreated &&
			ev.Kind != types.EventTextureAgentRevisionCompleted {
			continue
		}
		payload := parseTracePayload(ev.Payload)
		docID := payloadString(payload, "doc_id")
		if docID == "" {
			continue
		}
		ref := builder.addEventEvidence(ev, "Texture document revision exists for trajectory", map[string]any{"doc_id": docID})
		return docID, ev.Timestamp, ref
	}
	return "", time.Time{}, ""
}

func collectAcceptanceToolResults(events []types.EventRecord, tool string) []acceptanceToolResult {
	return collectAcceptanceToolResultsAny(events, tool)
}

func collectAcceptanceToolResultsAny(events []types.EventRecord, tools ...string) []acceptanceToolResult {
	wanted := map[string]bool{}
	for _, tool := range tools {
		wanted[tool] = true
	}
	var results []acceptanceToolResult
	for _, ev := range events {
		if ev.Kind != types.EventToolResult {
			continue
		}
		payload := parseTracePayload(ev.Payload)
		tool := payloadString(payload, "tool")
		if !wanted[tool] {
			continue
		}
		if isError, _ := payload["is_error"].(bool); isError {
			continue
		}
		output := parseTraceToolOutput(payload)
		if len(output) == 0 {
			continue
		}
		results = append(results, acceptanceToolResult{tool: tool, event: ev, output: output})
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].event.StreamSeq < results[j].event.StreamSeq
	})
	return results
}

func collectAcceptanceToolErrors(events []types.EventRecord, tool string) []acceptanceToolError {
	return collectAcceptanceToolErrorsAny(events, tool)
}

func collectAcceptanceToolErrorsAny(events []types.EventRecord, tools ...string) []acceptanceToolError {
	wanted := map[string]bool{}
	for _, tool := range tools {
		wanted[tool] = true
	}
	var results []acceptanceToolError
	for _, ev := range events {
		if ev.Kind != types.EventToolResult {
			continue
		}
		payload := parseTracePayload(ev.Payload)
		tool := payloadString(payload, "tool")
		if !wanted[tool] {
			continue
		}
		if isError, _ := payload["is_error"].(bool); !isError {
			continue
		}
		output := traceToolErrorText(payload)
		if output == "" {
			continue
		}
		results = append(results, acceptanceToolError{tool: tool, event: ev, output: output})
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].event.StreamSeq < results[j].event.StreamSeq
	})
	return results
}

func collectAcceptancePendingToolInvocations(events []types.EventRecord, tool string) []acceptanceToolInvocation {
	return collectAcceptancePendingToolInvocationsAny(events, tool)
}

func collectAcceptancePendingToolInvocationsAny(events []types.EventRecord, tools ...string) []acceptanceToolInvocation {
	wanted := map[string]bool{}
	for _, tool := range tools {
		wanted[tool] = true
	}
	completedCallIDs := map[string]bool{}
	for _, ev := range events {
		if ev.Kind != types.EventToolResult {
			continue
		}
		payload := parseTracePayload(ev.Payload)
		tool := payloadString(payload, "tool")
		if !wanted[tool] {
			continue
		}
		callID := payloadString(payload, "call_id")
		if callID != "" {
			completedCallIDs[callID] = true
		}
	}

	var invocations []acceptanceToolInvocation
	seenPendingCallIDs := map[string]bool{}
	for _, ev := range events {
		if ev.Kind != types.EventToolInvoked {
			continue
		}
		payload := parseTracePayload(ev.Payload)
		tool := payloadString(payload, "tool")
		if !wanted[tool] {
			continue
		}
		callID := payloadString(payload, "call_id")
		if callID != "" && completedCallIDs[callID] {
			continue
		}
		if callID != "" {
			if seenPendingCallIDs[callID] {
				continue
			}
			seenPendingCallIDs[callID] = true
		}
		args, _ := payload["arguments"].(map[string]any)
		invocations = append(invocations, acceptanceToolInvocation{
			tool:      tool,
			event:     ev,
			callID:    callID,
			arguments: args,
		})
	}
	sort.Slice(invocations, func(i, j int) bool {
		return invocations[i].event.StreamSeq < invocations[j].event.StreamSeq
	})
	return invocations
}

func traceToolErrorText(payload map[string]any) string {
	switch value := payload["output"].(type) {
	case string:
		text := strings.TrimSpace(value)
		if text == "" {
			return ""
		}
		var decoded map[string]any
		if json.Unmarshal([]byte(text), &decoded) == nil {
			if raw := payloadString(decoded, "raw_output"); raw != "" {
				return raw
			}
			if errText := payloadString(decoded, "error"); errText != "" {
				return errText
			}
		}
		return text
	case map[string]any:
		if raw := payloadString(value, "raw_output"); raw != "" {
			return raw
		}
		if errText := payloadString(value, "error"); errText != "" {
			return errText
		}
	}
	return ""
}

func acceptanceOutputSlice(output map[string]any, key string) []map[string]any {
	switch raw := output[key].(type) {
	case []map[string]any:
		items := make([]map[string]any, 0, len(raw))
		for _, item := range raw {
			if item != nil {
				items = append(items, item)
			}
		}
		return items
	case []any:
		items := make([]map[string]any, 0, len(raw))
		for _, item := range raw {
			mapped, _ := item.(map[string]any)
			if mapped != nil {
				items = append(items, mapped)
			}
		}
		return items
	default:
		return nil
	}
}

func acceptanceDelegateWorkerDetails(tool string, output map[string]any, packages []map[string]any) map[string]any {
	if packages == nil {
		packages = acceptanceOutputSlice(output, "app_change_packages")
	}
	if tool == "" {
		tool = "delegate_worker_vm"
	}
	details := map[string]any{
		"tool":          tool,
		"package_count": len(packages),
	}
	for _, key := range []string{
		"status",
		"state",
		"worker_id",
		"worker_vm_id",
		"worker_sandbox_url",
		"loop_id",
		"agent_id",
		"profile",
		"terminal_error",
		"error",
		"completion_blocker",
	} {
		if value := payloadString(output, key); value != "" {
			details[key] = value
		}
	}
	if loopID := payloadString(output, "loop_id"); loopID != "" {
		details["worker_loop_id"] = loopID
	}
	for _, key := range []string{"event_count", "worker_root_event_count", "worker_channel_message_count"} {
		if value := output[key]; value != nil {
			details[key] = value
		}
	}
	if childRunIDs := acceptanceStringSlice(output, "worker_child_run_ids"); len(childRunIDs) > 0 {
		details["worker_child_run_ids"] = childRunIDs
	}
	if counts := acceptanceStringAnyMap(output, "worker_child_event_counts"); len(counts) > 0 {
		details["worker_child_event_counts"] = counts
	}
	if errors := acceptanceStringAnyMap(output, "worker_child_event_errors"); len(errors) > 0 {
		details["worker_child_event_errors"] = errors
	}
	if states := acceptanceStringAnyMap(output, "worker_child_run_states"); len(states) > 0 {
		details["worker_child_run_states"] = states
	}
	if errors := acceptanceStringAnyMap(output, "worker_child_status_errors"); len(errors) > 0 {
		details["worker_child_status_errors"] = errors
	}
	if profiles := acceptanceStringSlice(output, "worker_spawned_profiles"); len(profiles) > 0 {
		details["worker_spawned_profiles"] = profiles
	}
	if summary := acceptanceOutputSlice(output, "worker_event_summary"); len(summary) > 0 {
		details["worker_event_summary"] = summary
	}
	if len(packages) > 0 {
		details["app_change_packages"] = acceptanceAppPackageSummaries(packages)
	}
	if checkpoint := output["worker_update_checkpoint"]; checkpoint != nil {
		details["worker_update_checkpoint"] = checkpoint
	}
	if count := output["mirrored_worker_update_count"]; count != nil {
		details["mirrored_worker_update_count"] = count
	}
	if ids := acceptanceStringSlice(output, "mirrored_worker_update_ids"); len(ids) > 0 {
		details["mirrored_worker_update_ids"] = ids
	}
	if errors := acceptanceStringSlice(output, "mirrored_worker_update_errors"); len(errors) > 0 {
		details["mirrored_worker_update_errors"] = errors
	}
	return details
}

func acceptanceDelegateRuntimeSupervisionPassed(output map[string]any) bool {
	status := payloadString(output, "status")
	if status != "worker_run_completed" && status != "worker_observed" {
		return false
	}
	if payloadString(output, "terminal_error") != "" || payloadString(output, "completion_blocker") != "" {
		return false
	}
	if intMapValue(output, "mirrored_worker_update_count") > 0 {
		return true
	}
	if payloadString(output, "worker_update_checkpoint") == "worker_submit_update_mirrored" {
		return true
	}
	return false
}

func acceptanceAppPackageSummaries(packages []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(packages))
	for _, pkg := range packages {
		item := map[string]any{}
		for _, key := range []string{"status", "package_id", "app_id", "loop_id", "base_sha", "candidate_head_sha", "source_computer_id", "source_candidate_id", "candidate_source_ref", "package_manifest_sha256", "runtime_source_delta_sha256", "ui_source_delta_sha256"} {
			if value := payloadString(pkg, key); value != "" {
				item[key] = value
			} else if value := pkg[key]; value != nil {
				item[key] = value
			}
		}
		if len(item) > 0 {
			out = append(out, item)
		}
	}
	return out
}

func acceptanceStringSlice(output map[string]any, key string) []string {
	switch raw := output[key].(type) {
	case []string:
		return compactStringRefs(raw)
	case []any:
		out := make([]string, 0, len(raw))
		for _, item := range raw {
			text, _ := item.(string)
			if text != "" {
				out = append(out, text)
			}
		}
		return compactStringRefs(out)
	default:
		return nil
	}
}

func acceptanceStringAnyMap(output map[string]any, key string) map[string]any {
	switch raw := output[key].(type) {
	case map[string]any:
		out := map[string]any{}
		for k, v := range raw {
			if strings.TrimSpace(k) != "" {
				out[strings.TrimSpace(k)] = v
			}
		}
		return out
	case map[string]int:
		out := map[string]any{}
		for k, v := range raw {
			if strings.TrimSpace(k) != "" {
				out[strings.TrimSpace(k)] = v
			}
		}
		return out
	case map[string]string:
		out := map[string]any{}
		for k, v := range raw {
			if strings.TrimSpace(k) != "" && strings.TrimSpace(v) != "" {
				out[strings.TrimSpace(k)] = strings.TrimSpace(v)
			}
		}
		return out
	default:
		return nil
	}
}

func (b *acceptanceBuilder) addRunEvidence(run types.RunRecord, summary string) string {
	if run.RunID == "" {
		return ""
	}
	refID := "run:" + run.RunID
	b.addEvidence(types.RunAcceptanceEvidenceRef{
		RefID:      refID,
		Kind:       "run",
		Summary:    summary,
		RunID:      run.RunID,
		Trajectory: traceTrajectoryIDForRun(run),
		Details: map[string]any{
			"state":   run.State,
			"profile": traceRunProfile(run),
			"role":    traceRunRole(run),
		},
	})
	return refID
}

func (b *acceptanceBuilder) addEventEvidence(ev types.EventRecord, summary string, details map[string]any) string {
	refID := "event:" + ev.EventID
	b.addEvidence(types.RunAcceptanceEvidenceRef{
		RefID:      refID,
		Kind:       string(ev.Kind),
		Summary:    summary,
		RunID:      ev.RunID,
		EventID:    ev.EventID,
		Trajectory: ev.TrajectoryID,
		URL:        "/api/trace/trajectories/" + ev.TrajectoryID + "/moments/" + ev.EventID,
		Details:    details,
	})
	return refID
}

func (b *acceptanceBuilder) addEvidence(ref types.RunAcceptanceEvidenceRef) {
	if ref.RefID == "" || b.evidenceSet[ref.RefID] {
		return
	}
	b.evidenceSet[ref.RefID] = true
	b.evidence = append(b.evidence, ref)
}

func (b *acceptanceBuilder) addRollbackRef(kind, ref, summary string) {
	kind = strings.TrimSpace(kind)
	ref = strings.TrimSpace(ref)
	if kind == "" || ref == "" {
		return
	}
	for _, existing := range b.record.RollbackRefs {
		if existing.Kind == kind && existing.Ref == ref {
			return
		}
	}
	b.record.RollbackRefs = append(b.record.RollbackRefs, types.RunAcceptanceRollbackRef{
		Kind:    kind,
		Ref:     ref,
		Summary: strings.TrimSpace(summary),
	})
}

func (b *acceptanceBuilder) addCheckpoint(kind, state string, at time.Time, streamSeq int64, refs []string, details map[string]any) {
	b.record.Checkpoints = append(b.record.Checkpoints, types.RunAcceptanceCheckpoint{
		Kind:           kind,
		State:          state,
		At:             at,
		StreamSeq:      streamSeq,
		EvidenceRefIDs: compactStringRefs(refs),
		Details:        details,
	})
}

func compactStringRefs(refs []string) []string {
	out := make([]string, 0, len(refs))
	seen := map[string]bool{}
	for _, ref := range refs {
		ref = strings.TrimSpace(ref)
		if ref == "" || seen[ref] {
			continue
		}
		seen[ref] = true
		out = append(out, ref)
	}
	return out
}

func addAcceptanceAppPromotionCheckpoints(builder *acceptanceBuilder, events []types.EventRecord) {
	var packageRefs []string
	var verifyingRefs []string
	var verifiedRefs []string
	var promotedRefs []string
	var rollbackRefs []string
	var blockedRefs []string
	var lastPackage types.EventRecord
	var lastVerifying types.EventRecord
	var lastVerified types.EventRecord
	var lastPromoted types.EventRecord
	var lastRollback types.EventRecord
	var lastBlocked types.EventRecord
	for _, ev := range events {
		payload := parseTracePayload(ev.Payload)
		switch ev.Kind {
		case types.EventAppChangePackagePublished:
			ref := builder.addEventEvidence(ev, "AppChangePackage published from candidate source lineage", map[string]any{
				"package_id":              payloadString(payload, "package_id"),
				"app_id":                  payloadString(payload, "app_id"),
				"source_computer_id":      payloadString(payload, "source_computer_id"),
				"source_candidate_id":     payloadString(payload, "source_candidate_id"),
				"candidate_source_ref":    payloadString(payload, "candidate_source_ref"),
				"package_manifest_sha256": payloadString(payload, "package_manifest_sha"),
			})
			packageRefs = append(packageRefs, ref)
			lastPackage = ev
		case types.EventAppAdoptionVerificationStarted:
			ref := builder.addEventEvidence(ev, "recipient candidate app adoption verification started", map[string]any{
				"adoption_id":              payloadString(payload, "adoption_id"),
				"package_id":               payloadString(payload, "package_id"),
				"target_computer_id":       payloadString(payload, "target_computer_id"),
				"target_candidate_id":      payloadString(payload, "target_candidate_id"),
				"candidate_source_ref":     payloadString(payload, "candidate_source_ref"),
				"recipient_build_required": payloadBool(payload, "recipient_build_required"),
				"recipient_build_status":   payloadString(payload, "recipient_build_status"),
			})
			verifyingRefs = append(verifyingRefs, ref)
			lastVerifying = ev
		case types.EventAppAdoptionVerified:
			ref := builder.addEventEvidence(ev, "recipient candidate rebuilt and verified AppChangePackage", map[string]any{
				"adoption_id":                  payloadString(payload, "adoption_id"),
				"package_id":                   payloadString(payload, "package_id"),
				"target_computer_id":           payloadString(payload, "target_computer_id"),
				"runtime_artifact_digest":      payloadString(payload, "runtime_artifact_digest"),
				"ui_artifact_digest":           payloadString(payload, "ui_artifact_digest"),
				"foreground_tail_merge_result": payloadString(payload, "foreground_tail_merge_result"),
			})
			verifiedRefs = append(verifiedRefs, ref)
			lastVerified = ev
		case types.EventAppAdoptionBlocked:
			ref := builder.addEventEvidence(ev, "recipient candidate app adoption blocked", map[string]any{
				"adoption_id":                  payloadString(payload, "adoption_id"),
				"package_id":                   payloadString(payload, "package_id"),
				"target_computer_id":           payloadString(payload, "target_computer_id"),
				"runtime_artifact_digest":      payloadString(payload, "runtime_artifact_digest"),
				"ui_artifact_digest":           payloadString(payload, "ui_artifact_digest"),
				"foreground_tail_merge_result": payloadString(payload, "foreground_tail_merge_result"),
				"error":                        payloadString(payload, "error"),
			})
			blockedRefs = append(blockedRefs, ref)
			lastBlocked = ev
		case types.EventAppAdoptionPromoted:
			rollbackSourceRef := payloadString(payload, "rollback_source_ref")
			ref := builder.addEventEvidence(ev, "target computer source lineage advanced to adopted app candidate", map[string]any{
				"adoption_id":             payloadString(payload, "adoption_id"),
				"package_id":              payloadString(payload, "package_id"),
				"target_computer_id":      payloadString(payload, "target_computer_id"),
				"candidate_source_ref":    payloadString(payload, "candidate_source_ref"),
				"runtime_artifact_digest": payloadString(payload, "runtime_artifact_digest"),
				"ui_artifact_digest":      payloadString(payload, "ui_artifact_digest"),
				"route_profile":           payloadString(payload, "route_profile"),
				"default_base_profile":    payloadString(payload, "default_base_profile"),
				"rollback_source_ref":     rollbackSourceRef,
			})
			builder.addRollbackRef("source_ref", rollbackSourceRef, "previous active source ref before app adoption promotion")
			promotedRefs = append(promotedRefs, ref)
			lastPromoted = ev
			rollbackRefs = append(rollbackRefs, ref)
		case types.EventAppAdoptionRolledBack:
			rollbackSourceRef := payloadString(payload, "rollback_source_ref")
			ref := builder.addEventEvidence(ev, "app adoption rollback recorded", map[string]any{
				"adoption_id":         payloadString(payload, "adoption_id"),
				"package_id":          payloadString(payload, "package_id"),
				"target_computer_id":  payloadString(payload, "target_computer_id"),
				"rollback_source_ref": rollbackSourceRef,
			})
			builder.addRollbackRef("source_ref", rollbackSourceRef, "active source ref restored by app adoption rollback")
			rollbackRefs = append(rollbackRefs, ref)
			lastRollback = ev
		}
	}
	if len(packageRefs) > 0 {
		builder.addCheckpoint("app_package_published", "passed", lastPackage.Timestamp, lastPackage.StreamSeq, packageRefs, map[string]any{"package_event_count": len(packageRefs)})
	}
	if len(verifyingRefs) > 0 {
		builder.addCheckpoint("app_adoption_verifying", "pending", lastVerifying.Timestamp, lastVerifying.StreamSeq, verifyingRefs, map[string]any{"verifying_event_count": len(verifyingRefs)})
	}
	if len(verifiedRefs) > 0 {
		builder.addCheckpoint("app_adoption_verified", "passed", lastVerified.Timestamp, lastVerified.StreamSeq, verifiedRefs, map[string]any{"verified_event_count": len(verifiedRefs)})
	}
	if len(blockedRefs) > 0 {
		builder.addCheckpoint("app_adoption_blocked", "failed", lastBlocked.Timestamp, lastBlocked.StreamSeq, blockedRefs, map[string]any{"blocked_event_count": len(blockedRefs)})
	}
	if len(promotedRefs) > 0 {
		builder.addCheckpoint("app_adoption_promoted", "passed", lastPromoted.Timestamp, lastPromoted.StreamSeq, promotedRefs, map[string]any{"promoted_event_count": len(promotedRefs)})
	}
	if len(rollbackRefs) > 0 {
		at := lastPromoted.Timestamp
		seq := lastPromoted.StreamSeq
		if !lastRollback.Timestamp.IsZero() {
			at = lastRollback.Timestamp
			seq = lastRollback.StreamSeq
		}
		builder.addCheckpoint("rollback_available", "passed", at, seq, rollbackRefs, map[string]any{"app_rollback_ref_count": len(rollbackRefs)})
	}
}

func addAcceptanceContinuationAndCompactionCheckpoints(builder *acceptanceBuilder, events []types.EventRecord) {
	compacted := false
	for _, ev := range events {
		if compacted {
			break
		}
		switch ev.Kind {
		case types.EventRunCompactionCompleted:
			details := acceptanceCompactionEventDetails(ev)
			ref := builder.addEventEvidence(ev, "run-memory compaction checkpoint recorded", details)
			builder.addCheckpoint("compacted", "passed", ev.Timestamp, ev.StreamSeq, []string{ref}, details)
			compacted = true
		}
	}
	continued := false
	for _, ev := range events {
		if continued {
			break
		}
		switch ev.Kind {
		case types.EventRunContinuationSelected, types.EventRunContinuationStarted:
			details := acceptanceContinuationEventDetails(ev)
			ref := builder.addEventEvidence(ev, "run continuation selected or started", details)
			builder.addCheckpoint("continued", "passed", ev.Timestamp, ev.StreamSeq, []string{ref}, details)
			continued = true
		}
	}
}

func acceptanceCompactionEventDetails(ev types.EventRecord) map[string]any {
	payload := parseTracePayload(ev.Payload)
	details := map[string]any{"kind": ev.Kind}
	for _, key := range []string{
		"entry_id",
		"reason",
		"source",
		"source_state",
		"tokens_before",
		"tokens_after",
		"first_kept_entry_id",
		"compacted_messages",
		"kept_messages",
		"event_count",
		"summarized_events",
		"omitted_delta_events",
	} {
		if value, ok := payload[key]; ok && value != nil {
			details[key] = value
		}
	}
	return details
}

func acceptanceContinuationEventDetails(ev types.EventRecord) map[string]any {
	payload := parseTracePayload(ev.Payload)
	details := map[string]any{"kind": ev.Kind}
	for _, key := range []string{
		"continuation_id",
		"status",
		"objective_fingerprint",
		"authority_profile",
		"next_loop_id",
		"lease_seconds",
		"compaction_status",
		"compaction_error",
		"adoption_id",
		"package_id",
		"adoption_status",
		"trace_id",
		"target_computer_id",
		"target_candidate_id",
		"candidate_source_ref",
	} {
		if value, ok := payload[key]; ok && value != nil {
			details[key] = value
		}
	}
	return details
}

func acceptanceLevelAndState(checkpoints []types.RunAcceptanceCheckpoint) (types.RunAcceptanceLevel, types.RunAcceptanceState) {
	has := map[string]bool{}
	for _, checkpoint := range checkpoints {
		if checkpoint.State == "passed" {
			has[checkpoint.Kind] = true
		}
	}
	level := types.RunAcceptanceDocsLevel
	state := types.RunAcceptanceBlocked
	textureOpened := has[runAcceptanceCheckpointTextureOpened]
	if has["submitted"] && textureOpened {
		level = types.RunAcceptanceStagingSmokeLevel
	}
	if has["submitted"] && textureOpened && has["super_requested"] && has["worker_leased"] && has["worker_supervision_observed"] {
		level = types.RunAcceptanceStagingSmokeLevel
		state = types.RunAcceptanceAccepted
	}
	if has["app_package_published"] && (has["worker_delegated"] || has["app_adoption_verified"]) {
		level = types.RunAcceptanceExportLevel
		state = types.RunAcceptanceAccepted
	}
	if has["app_adoption_verified"] && has["app_adoption_promoted"] && has["rollback_available"] {
		level = types.RunAcceptancePromotionLevel
		state = types.RunAcceptanceAccepted
	}
	if level == types.RunAcceptancePromotionLevel && has["continued"] {
		level = types.RunAcceptanceContinuationLevel
	}
	return level, state
}

func buildAcceptanceInvariantChecks(rec types.RunAcceptanceRecord) []types.RunAcceptanceInvariantCheck {
	kindSet := map[string]bool{}
	blockedKindSet := map[string]bool{}
	for _, checkpoint := range rec.Checkpoints {
		if checkpoint.State == "passed" {
			kindSet[checkpoint.Kind] = true
		}
		if checkpoint.State == "blocked" {
			blockedKindSet[checkpoint.Kind] = true
		}
	}
	textureOpened := kindSet[runAcceptanceCheckpointTextureOpened]
	promptPathObserved := kindSet["submitted"] && textureOpened
	adoptionPathObserved := kindSet["app_adoption_verified"] || kindSet["app_adoption_promoted"]
	workerMutationBounded := kindSet["worker_leased"] && kindSet["worker_delegated"] && kindSet["app_package_published"]
	workerSupervisionBounded := kindSet["worker_leased"] && kindSet["worker_delegated"] && kindSet["worker_supervision_observed"]
	adoptionMutationBounded := kindSet["app_adoption_verified"] && kindSet["rollback_available"]
	workerPathAttempted := kindSet["worker_leased"] || kindSet["worker_delegated"] || kindSet["worker_supervision_observed"] || kindSet["app_package_published"] || blockedKindSet["worker_delegated"]
	adoptionPathAttempted := kindSet["app_adoption_verified"] || kindSet["app_adoption_promoted"]
	workerMutationInvariant := (!workerPathAttempted && !adoptionPathAttempted) || workerMutationBounded || workerSupervisionBounded || adoptionMutationBounded
	checks := []types.RunAcceptanceInvariantCheck{
		{
			Name:   "product_path_observed",
			State:  stateForBool(promptPathObserved || adoptionPathObserved),
			Detail: "acceptance is derived from prompt/Texture/super trace evidence or product AppChangePackage/adoption events, not caller-supplied checkpoints",
		},
		{
			Name:   "worker_mutation_bounded",
			State:  stateForBool(workerMutationInvariant),
			Detail: "mutable coding work reached a worker VM/AppChangePackage boundary, runtime-only worker supervision stayed bounded with worker-update evidence, or recipient adoption had rollback before becoming accepted",
		},
		{
			Name:   "promotion_not_overclaimed",
			State:  "passed",
			Detail: "package/adoption acceptance remains distinct from promotion-level acceptance",
		},
		{
			Name:   "checkpoint_causal_order",
			State:  stateForBool(acceptanceCheckpointPhaseOrderOK(rec.Checkpoints)),
			Detail: "primary passed checkpoint phases are causal where trace events provide stream_seq; repeated observations and superseded async probes are tolerated",
		},
	}
	return checks
}

func acceptanceRecordHasPassedCheckpoint(rec types.RunAcceptanceRecord, kind string) bool {
	for _, checkpoint := range rec.Checkpoints {
		if checkpoint.Kind == kind && checkpoint.State == "passed" {
			return true
		}
	}
	return false
}

func acceptanceCheckpointPhaseOrderOK(checkpoints []types.RunAcceptanceCheckpoint) bool {
	phaseByKind := map[string]int{
		"super_requested":             1,
		"worker_leased":               2,
		"worker_delegated":            3,
		"worker_supervision_observed": 3,
		"app_package_published":       4,
		"app_adoption_verifying":      5,
		"app_adoption_verified":       6,
		"app_adoption_promoted":       7,
		"rollback_available":          8,
		"compacted":                   9,
		"continued":                   10,
	}
	earliestByPhase := map[int]int64{}
	for _, checkpoint := range checkpoints {
		if checkpoint.State != "passed" || checkpoint.StreamSeq <= 0 {
			continue
		}
		phase, ok := phaseByKind[checkpoint.Kind]
		if !ok {
			continue
		}
		if current, exists := earliestByPhase[phase]; !exists || checkpoint.StreamSeq < current {
			earliestByPhase[phase] = checkpoint.StreamSeq
		}
	}
	var lastSeq int64
	for phase := 1; phase <= 10; phase++ {
		seq := earliestByPhase[phase]
		if seq <= 0 {
			continue
		}
		if lastSeq > 0 && seq < lastSeq {
			return false
		}
		if seq > lastSeq {
			lastSeq = seq
		}
	}
	return true
}

func acceptanceHasBlockedInvariant(checks []types.RunAcceptanceInvariantCheck) bool {
	for _, check := range checks {
		if check.State == "blocked" {
			return true
		}
	}
	return false
}

func stateForBool(ok bool) string {
	if ok {
		return "passed"
	}
	return "blocked"
}

func buildAcceptanceVerifierContracts(rec types.RunAcceptanceRecord) []types.RunAcceptanceVerifierContract {
	return []types.RunAcceptanceVerifierContract{
		{
			Name:    "trace-derived-state-machine",
			Purpose: "derive acceptance checkpoints from durable run/trace/package/adoption evidence",
			State:   stateForBool(len(rec.Checkpoints) > 0 && len(rec.EvidenceRefs) > 0),
		},
		{
			Name:    "export-level-product-path",
			Purpose: "prove prompt/Texture/super/vmctl/delegate/AppChangePackage/adoption prefix without browser-public internal orchestration APIs",
			State: stateForBool(
				rec.State == types.RunAcceptanceAccepted &&
					(rec.AcceptanceLevel == types.RunAcceptanceExportLevel ||
						rec.AcceptanceLevel == types.RunAcceptancePromotionLevel ||
						rec.AcceptanceLevel == types.RunAcceptanceContinuationLevel),
			),
		},
	}
}

func buildAcceptanceResidualRisks(rec types.RunAcceptanceRecord) []string {
	var risks []string
	has := map[string]bool{}
	delegationBlocked := false
	nonCleanExportStatus := ""
	for _, checkpoint := range rec.Checkpoints {
		if checkpoint.State == "passed" {
			has[checkpoint.Kind] = true
		}
		if checkpoint.Kind == "worker_delegated" && checkpoint.State == "blocked" {
			delegationBlocked = true
		}
		if checkpoint.Kind == "worker_delegated" && checkpoint.State == "passed" {
			if status, _ := checkpoint.Details["non_clean_delegate_status"].(string); status != "" {
				nonCleanExportStatus = status
			}
		}
	}
	if rec.AcceptanceLevel == types.RunAcceptanceExportLevel {
		risks = append(risks, "promotion-level acceptance is not proven until recipient build verifier contracts, app adoption promotion, and rollback evidence are recorded")
	}
	hasAdoptionVerification := false
	hasAdoptionPromotion := false
	for _, checkpoint := range rec.Checkpoints {
		if checkpoint.Kind == "app_adoption_verified" && checkpoint.State == "passed" {
			hasAdoptionVerification = true
		}
		if checkpoint.Kind == "app_adoption_promoted" && checkpoint.State == "passed" {
			hasAdoptionPromotion = true
		}
	}
	if hasAdoptionVerification && !hasAdoptionPromotion {
		risks = append(risks, "verified app adoptions still require durable promote/rollback closure before promotion-level acceptance")
	}
	if delegationBlocked && !has["worker_delegated"] {
		risks = append(risks, "worker VM delegation did not complete, so co-super, package, and adoption acceptance remain unproven")
	}
	if nonCleanExportStatus != "" {
		risks = append(risks, "AppChangePackage evidence was reviewable but delegate returned non-clean status "+nonCleanExportStatus+"; termination behavior still needs hardening before promotion-level acceptance")
	}
	if !has["compacted"] {
		risks = append(risks, "continuation-level acceptance is not proven until run-memory compaction and continuation evidence are recorded")
	}
	if rec.VMMode == "local_worktree" {
		risks = append(risks, "worker isolation used local worktree mode; this is a diagnostic fallback unless staging vmctl is expected to run that mode")
	}
	for _, check := range rec.InvariantChecks {
		if check.State == "blocked" {
			risks = append(risks, fmt.Sprintf("acceptance invariant %s is blocked: %s", check.Name, check.Detail))
		}
	}
	return risks
}
