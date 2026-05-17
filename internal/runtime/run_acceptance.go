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

	"github.com/yusefmosiah/go-choir/internal/buildinfo"
	"github.com/yusefmosiah/go-choir/internal/promotion"
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

type acceptanceBuilder struct {
	record      types.RunAcceptanceRecord
	evidence    []types.RunAcceptanceEvidenceRef
	evidenceSet map[string]bool
}

type acceptanceToolResult struct {
	event  types.EventRecord
	output map[string]any
}

type acceptanceToolError struct {
	event  types.EventRecord
	output string
}

type acceptanceToolInvocation struct {
	event     types.EventRecord
	callID    string
	arguments map[string]any
}

// SynthesizeRunAcceptance derives a durable run acceptance record from
// product-path evidence already present in runs, Trace events, worker export
// results, and promotion records. The caller chooses the target trajectory; the
// verifier chooses checkpoints.
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

	promotionCandidates, err := rt.store.ListPromotionCandidates(ctx, ownerID, 500)
	if err != nil {
		return types.RunAcceptanceRecord{}, fmt.Errorf("synthesize run acceptance: list promotion candidates: %w", err)
	}
	promotionCandidates = filterAcceptancePromotionCandidates(promotionCandidates, in.TrajectoryID, in.RunID, trajectoryRuns)

	root := chooseAcceptanceRootRun(trajectoryRuns, in.RunID)
	if root.RunID == "" && len(events) == 0 {
		return types.RunAcceptanceRecord{}, fmt.Errorf("synthesize run acceptance: trajectory not found")
	}

	build := buildinfo.Snapshot("sandbox")
	deploymentCommit := firstNonEmpty(build.DeployedCommit, build.Commit)
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
		builder.addCheckpoint("vtext_opened", "passed", at, 0, []string{refID}, map[string]any{"doc_id": docID})
	}

	superResults := collectAcceptanceToolResults(events, "request_super_execution")
	if len(superResults) > 0 {
		item := superResults[0]
		ref := builder.addEventEvidence(item.event, "vtext requested persistent super execution", map[string]any{
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

	delegateResults := collectAcceptanceToolResults(events, "delegate_worker_vm")
	var exportRefs []string
	var nonExportDelegateResults []acceptanceToolResult
	exportCount := 0
	for _, item := range delegateResults {
		exports := acceptanceOutputSlice(item.output, "export_patchsets")
		status := payloadString(item.output, "status")
		if len(exports) == 0 {
			nonExportDelegateResults = append(nonExportDelegateResults, item)
			continue
		}
		exportCount += len(exports)
		if mode := payloadString(item.output, "worker_isolation"); mode != "" {
			builder.record.VMMode = mode
		}
		for _, export := range exports {
			if builder.record.BaseSHA == "" {
				builder.record.BaseSHA = payloadString(export, "base_sha")
			}
		}
		delegateDetails := acceptanceDelegateWorkerDetails(item.output, exports)
		evidenceSummary := "worker run exported concrete patchset evidence"
		if status != "" && status != "worker_run_completed" {
			delegateDetails["non_clean_delegate_status"] = status
			evidenceSummary = "worker run returned reviewable export evidence with non-clean delegate status"
		}
		ref := builder.addEventEvidence(item.event, evidenceSummary, delegateDetails)
		exportRefs = append(exportRefs, ref)
		builder.addCheckpoint("worker_delegated", "passed", item.event.Timestamp, item.event.StreamSeq, []string{ref}, delegateDetails)
	}
	if exportCount == 0 {
		delegateErrors := collectAcceptanceToolErrors(events, "delegate_worker_vm")
		pendingDelegates := collectAcceptancePendingToolInvocations(events, "delegate_worker_vm")
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
		if len(nonExportDelegateResults) > 0 {
			for _, item := range nonExportDelegateResults {
				ref := builder.addEventEvidence(item.event, "worker VM delegation returned without export evidence", acceptanceDelegateWorkerDetails(item.output, nil))
				refs = append(refs, ref)
				rememberLatest(item.event)
			}
			last := nonExportDelegateResults[len(nonExportDelegateResults)-1]
			details["result_count"] = len(nonExportDelegateResults)
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
				ref := builder.addEventEvidence(item.event, "worker VM delegation failed before export", map[string]any{
					"tool":  "delegate_worker_vm",
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
					"tool":               "delegate_worker_vm",
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
			if len(nonExportDelegateResults) == 0 && len(delegateErrors) == 0 {
				details["worker_id"] = payloadString(last.arguments, "worker_id")
				details["worker_vm_id"] = payloadString(last.arguments, "vm_id")
				details["worker_sandbox_url"] = payloadString(last.arguments, "worker_sandbox_url")
				details["status"] = "invoked_without_terminal_result"
			}
		}
		if len(refs) > 0 {
			builder.addCheckpoint("worker_delegated", "blocked", blockedAt, blockedSeq, refs, details)
		}
	}
	if exportCount > 0 {
		builder.addCheckpoint("export_observed", "passed", time.Now().UTC(), 0, exportRefs, map[string]any{"export_count": exportCount})
	}

	var candidateRefs []string
	for _, candidate := range promotionCandidates {
		ref := builder.addPromotionEvidence(candidate)
		candidateRefs = append(candidateRefs, ref)
		if builder.record.BaseSHA == "" {
			builder.record.BaseSHA = candidate.BaseSHA
		}
		if builder.record.VMMode == "" && candidate.VMID != "" {
			builder.record.VMMode = "worker"
		}
	}
	if len(candidateRefs) > 0 {
		builder.addCheckpoint("promotion_candidate_queued", "passed", promotionCandidates[0].UpdatedAt, 0, candidateRefs, map[string]any{"candidate_count": len(candidateRefs)})
		builder.record.RollbackRefs = append(builder.record.RollbackRefs, acceptanceRollbackRefs(promotionCandidates)...)
		if len(builder.record.RollbackRefs) > 0 {
			builder.addCheckpoint("rollback_available", "passed", promotionCandidates[0].UpdatedAt, 0, candidateRefs, map[string]any{"rollback_ref_count": len(builder.record.RollbackRefs)})
		}
	}
	if candidate := firstPromotionWithStatus(promotionCandidates, types.PromotionCandidateVerified, types.PromotionCandidatePromoted); candidate != nil {
		builder.addCheckpoint("verification_passed", "passed", candidate.UpdatedAt, 0, []string{"promotion:" + candidate.CandidateID}, map[string]any{"candidate_id": candidate.CandidateID})
	}
	if candidate := firstPromotionWithOwnerReview(promotionCandidates); candidate != nil {
		builder.addCheckpoint("owner_reviewed", "passed", candidate.UpdatedAt, 0, []string{"promotion:" + candidate.CandidateID}, map[string]any{
			"candidate_id": candidate.CandidateID,
			"status":       candidate.Status,
		})
	}
	if candidate := firstPromotionWithStatus(promotionCandidates, types.PromotionCandidatePromoted); candidate != nil {
		builder.addCheckpoint("promoted", "passed", candidate.UpdatedAt, 0, []string{"promotion:" + candidate.CandidateID}, map[string]any{"candidate_id": candidate.CandidateID})
	}

	addAcceptanceContinuationAndCompactionCheckpoints(&builder, events)

	builder.record.AcceptanceLevel, builder.record.State = acceptanceLevelAndState(builder.record.Checkpoints)
	builder.record.EvidenceRefs = builder.evidence
	builder.record.InvariantChecks = buildAcceptanceInvariantChecks(builder.record)
	builder.record.VerifierContracts = buildAcceptanceVerifierContracts(builder.record)
	builder.record.FailureResidualRisks = buildAcceptanceResidualRisks(builder.record)
	rec, err := rt.store.UpsertRunAcceptance(ctx, builder.record)
	if err != nil {
		return types.RunAcceptanceRecord{}, err
	}
	return rec, nil
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
		if traceRunProfile(run) == AgentProfileConductor && metadataStringValue(run.Metadata, "input_source") == "prompt_bar" {
			return run
		}
	}
	if len(runs) > 0 {
		return runs[0]
	}
	return types.RunRecord{}
}

func filterAcceptancePromotionCandidates(candidates []types.PromotionCandidateRecord, trajectoryID, runID string, runs []types.RunRecord) []types.PromotionCandidateRecord {
	runIDs := map[string]bool{}
	for _, run := range runs {
		runIDs[run.RunID] = true
	}
	if runID != "" {
		runIDs[runID] = true
	}
	var filtered []types.PromotionCandidateRecord
	for _, candidate := range candidates {
		if candidate.TraceID == trajectoryID || runIDs[candidate.SourceRunID] {
			filtered = append(filtered, candidate)
		}
	}
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].UpdatedAt.After(filtered[j].UpdatedAt)
	})
	return filtered
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
				ref := builder.addRunEvidence(root, "conductor decision opened vtext document")
				return docID, root.UpdatedAt, ref
			}
		}
	}
	for _, ev := range events {
		if ev.Kind != types.EventVTextDocumentRevisionCreated && ev.Kind != types.EventVTextAgentRevisionCompleted {
			continue
		}
		payload := parseTracePayload(ev.Payload)
		docID := payloadString(payload, "doc_id")
		if docID == "" {
			continue
		}
		ref := builder.addEventEvidence(ev, "vtext document revision exists for trajectory", map[string]any{"doc_id": docID})
		return docID, ev.Timestamp, ref
	}
	return "", time.Time{}, ""
}

func collectAcceptanceToolResults(events []types.EventRecord, tool string) []acceptanceToolResult {
	var results []acceptanceToolResult
	for _, ev := range events {
		if ev.Kind != types.EventToolResult {
			continue
		}
		payload := parseTracePayload(ev.Payload)
		if payloadString(payload, "tool") != tool {
			continue
		}
		if isError, _ := payload["is_error"].(bool); isError {
			continue
		}
		output := parseTraceToolOutput(payload)
		if len(output) == 0 {
			continue
		}
		results = append(results, acceptanceToolResult{event: ev, output: output})
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].event.StreamSeq < results[j].event.StreamSeq
	})
	return results
}

func collectAcceptanceToolErrors(events []types.EventRecord, tool string) []acceptanceToolError {
	var results []acceptanceToolError
	for _, ev := range events {
		if ev.Kind != types.EventToolResult {
			continue
		}
		payload := parseTracePayload(ev.Payload)
		if payloadString(payload, "tool") != tool {
			continue
		}
		if isError, _ := payload["is_error"].(bool); !isError {
			continue
		}
		output := traceToolErrorText(payload)
		if output == "" {
			continue
		}
		results = append(results, acceptanceToolError{event: ev, output: output})
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].event.StreamSeq < results[j].event.StreamSeq
	})
	return results
}

func collectAcceptancePendingToolInvocations(events []types.EventRecord, tool string) []acceptanceToolInvocation {
	completedCallIDs := map[string]bool{}
	for _, ev := range events {
		if ev.Kind != types.EventToolResult {
			continue
		}
		payload := parseTracePayload(ev.Payload)
		if payloadString(payload, "tool") != tool {
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
		if payloadString(payload, "tool") != tool {
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

func acceptanceDelegateWorkerDetails(output map[string]any, exports []map[string]any) map[string]any {
	if exports == nil {
		exports = acceptanceOutputSlice(output, "export_patchsets")
	}
	details := map[string]any{
		"tool":         "delegate_worker_vm",
		"export_count": len(exports),
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
	if len(exports) > 0 {
		details["export_patchsets"] = acceptanceExportSummaries(exports)
	}
	if promotions := acceptanceOutputSlice(output, "promotion_queue"); len(promotions) > 0 {
		details["promotion_queue"] = acceptancePromotionSummaries(promotions)
	}
	if checkpoint := output["worker_update_checkpoint"]; checkpoint != nil {
		details["worker_update_checkpoint"] = checkpoint
	}
	return details
}

func acceptanceExportSummaries(exports []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(exports))
	for _, export := range exports {
		item := map[string]any{}
		for _, key := range []string{"status", "loop_id", "manifest_path", "patchset_path", "base_sha", "worker_head", "worker_head_sha", "patchset_sha256"} {
			if value := payloadString(export, key); value != "" {
				item[key] = value
			}
		}
		if len(item) > 0 {
			out = append(out, item)
		}
	}
	return out
}

func acceptancePromotionSummaries(candidates []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(candidates))
	for _, candidate := range candidates {
		item := map[string]any{}
		for _, key := range []string{"candidate_id", "status", "source_loop_id", "candidate_loop_id", "trace_id", "vm_id", "base_sha", "worker_head", "worker_head_sha", "manifest_path", "patchset_path", "integration_branch", "destination_branch", "objective_fingerprint", "patchset_sha256"} {
			if value := payloadString(candidate, key); value != "" {
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

func (b *acceptanceBuilder) addPromotionEvidence(candidate types.PromotionCandidateRecord) string {
	refID := "promotion:" + candidate.CandidateID
	details := map[string]any{
		"status":          candidate.Status,
		"source_loop_id":  candidate.SourceRunID,
		"trace_id":        candidate.TraceID,
		"vm_id":           candidate.VMID,
		"base_sha":        candidate.BaseSHA,
		"worker_head_sha": candidate.WorkerHeadSHA,
		"manifest_path":   candidate.ManifestPath,
		"patchset_path":   candidate.PatchsetPath,
	}
	if len(candidate.CandidateJSON) > 0 {
		var world promotion.CandidateWorld
		if json.Unmarshal(candidate.CandidateJSON, &world) == nil {
			details["objective_fingerprint"] = world.ObjectiveFingerprint
			details["patchset_sha256"] = world.PatchsetSHA256
		}
	}
	b.addEvidence(types.RunAcceptanceEvidenceRef{
		RefID:      refID,
		Kind:       "promotion_candidate",
		Summary:    "worker export queued a promotion candidate",
		RunID:      candidate.SourceRunID,
		Trajectory: candidate.TraceID,
		URL:        "/api/promotions/" + candidate.CandidateID,
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

func firstPromotionWithStatus(candidates []types.PromotionCandidateRecord, statuses ...types.PromotionCandidateStatus) *types.PromotionCandidateRecord {
	want := map[types.PromotionCandidateStatus]bool{}
	for _, status := range statuses {
		want[status] = true
	}
	for i := range candidates {
		if want[candidates[i].Status] {
			return &candidates[i]
		}
	}
	return nil
}

func firstPromotionWithOwnerReview(candidates []types.PromotionCandidateRecord) *types.PromotionCandidateRecord {
	for i := range candidates {
		if candidates[i].Status == types.PromotionCandidatePromoted || candidates[i].Status == types.PromotionCandidateRejected {
			return &candidates[i]
		}
		var report promotion.Report
		if len(candidates[i].ReportJSON) == 0 || !json.Valid(candidates[i].ReportJSON) {
			continue
		}
		if err := json.Unmarshal(candidates[i].ReportJSON, &report); err != nil {
			continue
		}
		if report.PromotionApproved || strings.TrimSpace(report.PromotionDecisionAt) != "" {
			return &candidates[i]
		}
	}
	return nil
}

func acceptanceRollbackRefs(candidates []types.PromotionCandidateRecord) []types.RunAcceptanceRollbackRef {
	var refs []types.RunAcceptanceRollbackRef
	for _, candidate := range candidates {
		if candidate.BaseSHA != "" {
			refs = append(refs, types.RunAcceptanceRollbackRef{
				Kind:    "git_base",
				Ref:     candidate.BaseSHA,
				Summary: "candidate can be discarded or integrated work can return to the recorded base SHA before promotion",
			})
		}
		if candidate.Status != types.PromotionCandidatePromoted {
			refs = append(refs, types.RunAcceptanceRollbackRef{
				Kind:    "candidate_world",
				Ref:     candidate.CandidateID,
				Summary: "candidate has not been canonically promoted; rollback is discard/archive of this queued candidate",
			})
		}
	}
	return refs
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
		"candidate_id",
		"trace_id",
		"vm_id",
		"base_sha",
		"worker_head_sha",
		"manifest_path",
		"patchset_path",
		"patchset_sha256",
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
	if has["submitted"] && has["vtext_opened"] {
		level = types.RunAcceptanceStagingSmokeLevel
	}
	if has["export_observed"] && has["promotion_candidate_queued"] {
		level = types.RunAcceptanceExportLevel
		state = types.RunAcceptanceAccepted
	}
	if has["verification_passed"] && has["owner_reviewed"] && (has["promoted"] || has["rollback_available"]) {
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
	var lastSeq int64
	orderOK := true
	for _, checkpoint := range rec.Checkpoints {
		if checkpoint.State == "passed" {
			kindSet[checkpoint.Kind] = true
		}
		if checkpoint.StreamSeq > 0 {
			if lastSeq > 0 && checkpoint.StreamSeq < lastSeq {
				orderOK = false
			}
			lastSeq = checkpoint.StreamSeq
		}
	}
	checks := []types.RunAcceptanceInvariantCheck{
		{
			Name:   "product_path_observed",
			State:  stateForBool(kindSet["submitted"] && kindSet["vtext_opened"] && kindSet["super_requested"]),
			Detail: "acceptance is derived from prompt/VText/super trace evidence, not caller-supplied checkpoints",
		},
		{
			Name:   "worker_mutation_bounded",
			State:  stateForBool(kindSet["worker_leased"] && kindSet["worker_delegated"] && kindSet["export_observed"]),
			Detail: "mutable coding work reached a worker VM/export boundary before becoming reviewable",
		},
		{
			Name:   "promotion_not_overclaimed",
			State:  "passed",
			Detail: "export-level acceptance remains distinct from promotion-level acceptance",
		},
		{
			Name:   "checkpoint_causal_order",
			State:  stateForBool(orderOK),
			Detail: "checkpoint stream sequence is monotonic where trace events provide stream_seq",
		},
	}
	return checks
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
			Purpose: "derive acceptance checkpoints from durable run/trace/promotion evidence",
			State:   stateForBool(len(rec.Checkpoints) > 0 && len(rec.EvidenceRefs) > 0),
		},
		{
			Name:    "export-level-product-path",
			Purpose: "prove prompt/VText/super/vmctl/delegate/export/promotion-queue prefix without browser-public internal orchestration APIs",
			State:   stateForBool(rec.AcceptanceLevel == types.RunAcceptanceExportLevel || rec.AcceptanceLevel == types.RunAcceptancePromotionLevel || rec.AcceptanceLevel == types.RunAcceptanceContinuationLevel),
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
		risks = append(risks, "promotion-level acceptance is not proven until verifier contracts, owner review, promotion or rollback evidence are recorded")
	}
	hasVerification := false
	hasOwnerReview := false
	for _, checkpoint := range rec.Checkpoints {
		if checkpoint.Kind == "verification_passed" && checkpoint.State == "passed" {
			hasVerification = true
		}
		if checkpoint.Kind == "owner_reviewed" && checkpoint.State == "passed" {
			hasOwnerReview = true
		}
	}
	if hasVerification && !hasOwnerReview {
		risks = append(risks, "verified candidates still require durable owner review before promotion-level acceptance")
	}
	if delegationBlocked && !has["worker_delegated"] {
		risks = append(risks, "worker VM delegation did not complete, so co-super, export, and promotion acceptance remain unproven")
	}
	if nonCleanExportStatus != "" {
		risks = append(risks, "worker export evidence was reviewable but delegate returned non-clean status "+nonCleanExportStatus+"; termination behavior still needs hardening before promotion-level acceptance")
	}
	if !has["compacted"] {
		risks = append(risks, "continuation-level acceptance is not proven until run-memory compaction and continuation evidence are recorded")
	}
	if rec.VMMode == "local_worktree" {
		risks = append(risks, "worker isolation used local worktree mode; this is a diagnostic fallback unless staging vmctl is expected to run that mode")
	}
	return risks
}
