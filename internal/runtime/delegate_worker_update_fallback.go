package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/yusefmosiah/go-choir/internal/types"
)

func (rt *Runtime) synthesizeDelegateWorkerUpdateOnSuperFailure(ctx context.Context, rec *types.RunRecord, runErr error) error {
	if rt == nil || rt.store == nil || rec == nil || agentProfileForRun(rec) != AgentProfileSuper {
		return nil
	}
	channelID, targetAgentID, ok := delegateWorkerUpdateTarget(rec)
	if !ok {
		return nil
	}

	eventsForRun, err := rt.store.ListEvents(ctx, rec.RunID, 1000)
	if err != nil {
		return err
	}
	if hasSuccessfulToolResult(eventsForRun, "submit_worker_update") {
		return nil
	}
	delegateEvent, delegateOutput, ok := latestSuccessfulToolResultOutput(eventsForRun, "delegate_worker_vm")
	if !ok {
		return nil
	}

	now := time.Now().UTC()
	update := delegateWorkerFallbackUpdate(rec, runErr, delegateEvent, delegateOutput, targetAgentID, channelID, now)
	return rt.dispatchDelegateWorkerUpdate(ctx, rec, update)
}

func (rt *Runtime) synthesizeDelegateWorkerUpdateCheckpoint(ctx context.Context, rec *types.RunRecord, output map[string]any, source string) error {
	if rt == nil || rt.store == nil || rec == nil || agentProfileForRun(rec) != AgentProfileSuper {
		return nil
	}
	channelID, targetAgentID, ok := delegateWorkerUpdateTarget(rec)
	if !ok {
		return nil
	}
	eventsForRun, err := rt.store.ListEvents(ctx, rec.RunID, 1000)
	if err != nil {
		return err
	}
	if hasSuccessfulToolResult(eventsForRun, "submit_worker_update") {
		return nil
	}
	now := time.Now().UTC()
	update := delegateWorkerCheckpointUpdate(rec, output, targetAgentID, channelID, strings.TrimSpace(source), now)
	return rt.dispatchDelegateWorkerUpdate(ctx, rec, update)
}

func delegateWorkerUpdateTarget(rec *types.RunRecord) (string, string, bool) {
	if rec == nil {
		return "", "", false
	}
	channelID := metadataStringValue(rec.Metadata, runMetadataChannelID)
	if channelID == "" {
		return "", "", false
	}
	targetAgentID := metadataStringValue(rec.Metadata, "requested_by_agent_id")
	if targetAgentID == "" && metadataStringValue(rec.Metadata, "requested_by_profile") == AgentProfileVText {
		targetAgentID = "vtext:" + channelID
	}
	if !strings.HasPrefix(targetAgentID, "vtext:") {
		return "", "", false
	}
	return channelID, targetAgentID, true
}

func (rt *Runtime) dispatchDelegateWorkerUpdate(ctx context.Context, rec *types.RunRecord, update types.WorkerUpdateRecord) error {
	update.Content = buildWorkerUpdateMessage(update)
	message := &types.ChannelMessage{
		ChannelID:    update.ChannelID,
		From:         rec.RunID,
		FromAgentID:  agentIDForRun(rec),
		FromRunID:    rec.RunID,
		ToAgentID:    update.TargetAgentID,
		TrajectoryID: metadataStringValue(rec.Metadata, runMetadataTrajectoryID),
		Role:         AgentProfileSuper,
		Content:      update.Content,
		Timestamp:    update.CreatedAt,
	}
	delivery := types.InboxDelivery{
		DeliveryID:   uuid.NewString(),
		OwnerID:      rec.OwnerID,
		ToAgentID:    update.TargetAgentID,
		FromAgentID:  message.FromAgentID,
		FromRunID:    rec.RunID,
		ChannelID:    update.ChannelID,
		Role:         message.Role,
		Content:      message.Content,
		TrajectoryID: message.TrajectoryID,
		CreatedAt:    update.CreatedAt,
	}
	stored, created, err := rt.store.DispatchWorkerUpdate(ctx, update, message, delivery)
	if err != nil {
		return err
	}
	if created {
		message.Seq = stored.MessageSeq
		rt.emitChannelMessageEvent(ctx, *message, rec.OwnerID)
	}
	return nil
}

func delegateWorkerFallbackUpdate(rec *types.RunRecord, runErr error, ev types.EventRecord, output map[string]any, targetAgentID, channelID string, now time.Time) types.WorkerUpdateRecord {
	status := stringMapValue(output, "status")
	state := stringMapValue(output, "state")
	workerLoopID := stringMapValue(output, "loop_id")
	workerVMID := stringMapValue(output, "worker_vm_id")
	workerID := stringMapValue(output, "worker_id")
	terminalError := firstNonEmpty(stringMapValue(output, "terminal_error"), stringMapValue(output, "error"))
	eventCount := intMapValue(output, "event_count")
	channelMessages := intMapValue(output, "worker_channel_message_count")
	exportCount := len(mapSliceValue(output, "export_patchsets"))

	findings := []string{
		fmt.Sprintf("delegate_worker_vm returned status %q with worker state %q before super reported back to VText.", firstNonEmpty(status, "unknown"), firstNonEmpty(state, "unknown")),
		fmt.Sprintf("super run ended with %s; preserving delegate evidence as a structured worker update.", firstNonEmpty(string(rec.State), "terminal state")),
	}
	if exportCount > 0 {
		findings = append(findings, fmt.Sprintf("delegate_worker_vm returned %d export patchset(s).", exportCount))
	} else {
		findings = append(findings, "delegate_worker_vm returned no export patchsets; treat this as blocked below export-level.")
	}
	if eventCount > 0 {
		findings = append(findings, fmt.Sprintf("worker event summary was preserved with %d event(s) and %d channel message(s).", eventCount, channelMessages))
	}

	evidenceIDs := []string{"event:" + ev.EventID, "run:" + rec.RunID}
	if workerLoopID != "" {
		evidenceIDs = append(evidenceIDs, "worker_loop:"+workerLoopID)
	}

	refs := []string{}
	for _, ref := range []string{
		"trajectory:" + metadataStringValue(rec.Metadata, runMetadataTrajectoryID),
		"worker_vm:" + workerVMID,
		"worker:" + workerID,
		"delegate_status:" + status,
		"worker_state:" + state,
	} {
		if !strings.HasSuffix(ref, ":") {
			refs = append(refs, ref)
		}
	}
	artifacts := exportArtifactRefs(output)
	notes := []string{
		"auto_synthesized_from=delegate_worker_vm_after_super_failure",
		"provider_error=" + strings.TrimSpace(runErr.Error()),
	}
	if terminalError != "" {
		notes = append(notes, "delegate_terminal_error="+terminalError)
	}
	if eventCount > 0 {
		notes = append(notes, fmt.Sprintf("worker_event_count=%d", eventCount))
	}

	return types.WorkerUpdateRecord{
		UpdateID:      "delegate-worker-vm-" + sanitizeExportPart(rec.RunID),
		OwnerID:       rec.OwnerID,
		AgentID:       agentIDForRun(rec),
		TargetAgentID: targetAgentID,
		ChannelID:     channelID,
		TrajectoryID:  metadataStringValue(rec.Metadata, runMetadataTrajectoryID),
		Role:          AgentProfileSuper,
		Findings:      trimNonEmpty(findings),
		EvidenceIDs:   trimNonEmpty(evidenceIDs),
		Artifacts:     trimNonEmpty(artifacts),
		Refs:          trimNonEmpty(refs),
		Proposals: []string{
			"Continue with a termination/export probe that makes vsuper call export_patchset or submit_worker_update before delegate timeout.",
		},
		Notes:     trimNonEmpty(notes),
		CreatedAt: now,
	}
}

func delegateWorkerCheckpointUpdate(rec *types.RunRecord, output map[string]any, targetAgentID, channelID, source string, now time.Time) types.WorkerUpdateRecord {
	status := stringMapValue(output, "status")
	state := stringMapValue(output, "state")
	workerLoopID := stringMapValue(output, "loop_id")
	workerVMID := stringMapValue(output, "worker_vm_id")
	workerID := stringMapValue(output, "worker_id")
	terminalError := firstNonEmpty(stringMapValue(output, "terminal_error"), stringMapValue(output, "error"))
	eventCount := intMapValue(output, "event_count")
	channelMessages := intMapValue(output, "worker_channel_message_count")
	exportCount := len(mapSliceValue(output, "export_patchsets"))

	findings := []string{
		fmt.Sprintf("delegate_worker_vm returned status %q with worker state %q.", firstNonEmpty(status, "unknown"), firstNonEmpty(state, "unknown")),
		"super preserved this delegate result as a VText worker-update checkpoint before relying on another LLM turn.",
	}
	if exportCount > 0 {
		findings = append(findings, fmt.Sprintf("delegate_worker_vm returned %d export patchset(s).", exportCount))
	} else {
		findings = append(findings, "delegate_worker_vm returned no export patchsets; treat this as blocked below export-level.")
	}
	if eventCount > 0 {
		findings = append(findings, fmt.Sprintf("worker event summary was preserved with %d event(s) and %d channel message(s).", eventCount, channelMessages))
	}

	evidenceIDs := []string{"run:" + rec.RunID}
	if workerLoopID != "" {
		evidenceIDs = append(evidenceIDs, "worker_loop:"+workerLoopID)
	}
	refs := []string{}
	for _, ref := range []string{
		"trajectory:" + metadataStringValue(rec.Metadata, runMetadataTrajectoryID),
		"worker_vm:" + workerVMID,
		"worker:" + workerID,
		"delegate_status:" + status,
		"worker_state:" + state,
	} {
		if !strings.HasSuffix(ref, ":") {
			refs = append(refs, ref)
		}
	}
	notes := []string{
		"auto_synthesized_from=delegate_worker_vm_checkpoint",
		"checkpoint_source=" + firstNonEmpty(source, "delegate_worker_vm_result"),
	}
	if terminalError != "" {
		notes = append(notes, "delegate_terminal_error="+terminalError)
	}
	if eventCount > 0 {
		notes = append(notes, fmt.Sprintf("worker_event_count=%d", eventCount))
	}
	updateKey := firstNonEmpty(workerLoopID, status, "result")
	return types.WorkerUpdateRecord{
		UpdateID:      "delegate-worker-vm-checkpoint-" + sanitizeExportPart(rec.RunID) + "-" + sanitizeExportPart(updateKey),
		OwnerID:       rec.OwnerID,
		AgentID:       agentIDForRun(rec),
		TargetAgentID: targetAgentID,
		ChannelID:     channelID,
		TrajectoryID:  metadataStringValue(rec.Metadata, runMetadataTrajectoryID),
		Role:          AgentProfileSuper,
		Findings:      trimNonEmpty(findings),
		EvidenceIDs:   trimNonEmpty(evidenceIDs),
		Artifacts:     trimNonEmpty(exportArtifactRefs(output)),
		Refs:          trimNonEmpty(refs),
		Proposals: []string{
			"Continue with a termination/export probe that prevents candidate checkout races and requires export_patchset or submit_worker_update before delegate timeout.",
		},
		Notes:     trimNonEmpty(notes),
		CreatedAt: now,
	}
}

func latestSuccessfulToolResultOutput(eventsForRun []types.EventRecord, tool string) (types.EventRecord, map[string]any, bool) {
	var latest types.EventRecord
	var latestOutput map[string]any
	found := false
	for _, ev := range eventsForRun {
		if ev.Kind != types.EventToolResult {
			continue
		}
		payload, ok := decodeToolResultPayload(ev)
		if !ok || payload.IsError || payload.Tool != tool {
			continue
		}
		var output map[string]any
		if err := json.Unmarshal([]byte(payload.Output), &output); err != nil {
			output = map[string]any{"raw_output": strings.TrimSpace(payload.Output)}
		}
		latest = ev
		latestOutput = output
		found = true
	}
	return latest, latestOutput, found
}

func hasSuccessfulToolResult(eventsForRun []types.EventRecord, tool string) bool {
	for _, ev := range eventsForRun {
		if ev.Kind != types.EventToolResult {
			continue
		}
		payload, ok := decodeToolResultPayload(ev)
		if ok && !payload.IsError && payload.Tool == tool {
			return true
		}
	}
	return false
}

type toolResultPayload struct {
	Tool    string `json:"tool"`
	IsError bool   `json:"is_error"`
	Output  string `json:"output"`
}

func decodeToolResultPayload(ev types.EventRecord) (toolResultPayload, bool) {
	var payload toolResultPayload
	if err := json.Unmarshal(ev.Payload, &payload); err != nil {
		return payload, false
	}
	payload.Tool = strings.TrimSpace(payload.Tool)
	return payload, payload.Tool != ""
}

func stringMapValue(m map[string]any, key string) string {
	value, _ := m[key].(string)
	return strings.TrimSpace(value)
}

func intMapValue(m map[string]any, key string) int {
	switch value := m[key].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	default:
		return 0
	}
}

func mapSliceValue(m map[string]any, key string) []map[string]any {
	raw, _ := m[key].([]any)
	out := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		if entry, ok := item.(map[string]any); ok {
			out = append(out, entry)
		}
	}
	return out
}

func exportArtifactRefs(output map[string]any) []string {
	exports := mapSliceValue(output, "export_patchsets")
	refs := []string{}
	for _, export := range exports {
		for _, key := range []string{"manifest_path", "patchset_path"} {
			if value := stringMapValue(export, key); value != "" {
				refs = append(refs, value)
			}
		}
	}
	return refs
}
