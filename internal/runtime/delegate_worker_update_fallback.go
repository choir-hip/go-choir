package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
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
	stored, created, err := rt.dispatchDelegateWorkerUpdate(ctx, rec, update)
	if err != nil {
		return err
	}
	if created && delegateWorkerContinuationRequired(delegateOutput) {
		return rt.enqueueDelegateWorkerSuperContinuation(ctx, rec, stored, delegateOutput, "super_failure")
	}
	return nil
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
	stored, created, err := rt.dispatchDelegateWorkerUpdate(ctx, rec, update)
	if err != nil {
		return err
	}
	if created && delegateWorkerContinuationRequired(output) {
		return rt.enqueueDelegateWorkerSuperContinuation(ctx, rec, stored, output, source)
	}
	return nil
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

func (rt *Runtime) dispatchDelegateWorkerUpdate(ctx context.Context, rec *types.RunRecord, update types.WorkerUpdateRecord) (types.WorkerUpdateRecord, bool, error) {
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
		return types.WorkerUpdateRecord{}, false, err
	}
	if created {
		message.Seq = stored.MessageSeq
		rt.emitChannelMessageEvent(ctx, *message, rec.OwnerID)
	}
	return stored, created, nil
}

func (rt *Runtime) enqueueDelegateWorkerSuperContinuation(ctx context.Context, rec *types.RunRecord, update types.WorkerUpdateRecord, output map[string]any, source string) error {
	if rt == nil || rt.store == nil || rec == nil {
		return nil
	}
	ownerID := strings.TrimSpace(rec.OwnerID)
	channelID := strings.TrimSpace(update.ChannelID)
	if ownerID == "" || channelID == "" {
		return nil
	}
	superAgent, err := rt.EnsurePersistentSuperAgent(ctx, ownerID)
	if err != nil {
		return fmt.Errorf("ensure persistent super for active worker continuation: %w", err)
	}
	content := buildDelegateWorkerSuperContinuationMessage(update, output, source)
	if strings.TrimSpace(content) == "" {
		return nil
	}
	castCtx := WithToolExecutionContext(ctx, rec)
	if _, err := rt.ChannelCast(castCtx, channelID, superAgent.AgentID, "", agentIDForRun(rec), "runtime-supervision", content); err != nil {
		return fmt.Errorf("enqueue active worker continuation: %w", err)
	}
	if _, err := rt.reconcilePersistentSuperActor(context.Background(), ownerID, superAgent.AgentID); err != nil {
		return fmt.Errorf("start active worker continuation super: %w", err)
	}
	return nil
}

func buildDelegateWorkerSuperContinuationMessage(update types.WorkerUpdateRecord, output map[string]any, source string) string {
	workerRunID := firstNonEmpty(stringMapValue(output, "worker_run_id"), stringMapValue(output, "loop_id"))
	if workerRunID == "" {
		return ""
	}
	workerSandboxURL := stringMapValue(output, "worker_sandbox_url")
	var b strings.Builder
	b.WriteString("Runtime supervision continuation required for an active worker delegation.\n")
	b.WriteString("This is a control request for persistent super, copied from the VText-visible worker checkpoint. VText may narrate or ask for clarification, but only super may observe, redirect, cancel, or finish this worker.\n\n")
	b.WriteString("Continue the existing worker; do not start a duplicate worker run.\n")
	b.WriteString("Use observe_worker_delegation or finish_worker_delegation against the existing worker_run_id. If the worker remains active without terminal evidence, redirect the vsuper with a precise instruction. Stop only when there is an AppChangePackage, a reviewable blocker, a cancellation certificate, or a bounded timeout certificate, then report back to VText with submit_worker_update.\n\n")
	b.WriteString("Worker refs:\n")
	b.WriteString("- worker_run_id: ")
	b.WriteString(workerRunID)
	b.WriteString("\n")
	if workerSandboxURL != "" {
		b.WriteString("- worker_sandbox_url: ")
		b.WriteString(workerSandboxURL)
		b.WriteString("\n")
	}
	for _, kv := range []struct {
		label string
		key   string
	}{
		{"worker_id", "worker_id"},
		{"worker_vm_id", "worker_vm_id"},
		{"status", "status"},
		{"state", "state"},
		{"profile", "profile"},
	} {
		if value := stringMapValue(output, kv.key); value != "" {
			b.WriteString("- ")
			b.WriteString(kv.label)
			b.WriteString(": ")
			b.WriteString(value)
			b.WriteString("\n")
		}
	}
	if update.UpdateID != "" {
		b.WriteString("- worker_update_id: ")
		b.WriteString(update.UpdateID)
		b.WriteString("\n")
	}
	if update.MessageSeq > 0 {
		b.WriteString("- vtext_channel_seq: ")
		b.WriteString(fmt.Sprintf("%d", update.MessageSeq))
		b.WriteString("\n")
	}
	if update.TrajectoryID != "" {
		b.WriteString("- trajectory_id: ")
		b.WriteString(update.TrajectoryID)
		b.WriteString("\n")
	}
	if source = strings.TrimSpace(source); source != "" {
		b.WriteString("- checkpoint_source: ")
		b.WriteString(source)
		b.WriteString("\n")
	}
	if children := stringSliceMapValue(output, "worker_child_run_ids"); len(children) > 0 {
		b.WriteString("- child_run_ids: ")
		b.WriteString(strings.Join(children, ", "))
		b.WriteString("\n")
	}
	if eventCount := intMapValue(output, "event_count"); eventCount > 0 {
		b.WriteString("- worker_event_count: ")
		b.WriteString(fmt.Sprintf("%d", eventCount))
		b.WriteString("\n")
	}
	if len(mapSliceValue(output, "app_change_packages")) == 0 {
		b.WriteString("- missing_terminal_evidence: no AppChangePackage or terminal blocker is present yet.\n")
	}
	return strings.TrimSpace(b.String())
}

func delegateWorkerContinuationRequired(output map[string]any) bool {
	if output == nil {
		return false
	}
	if len(mapSliceValue(output, "app_change_packages")) > 0 ||
		strings.TrimSpace(stringMapValue(output, "completion_blocker")) != "" ||
		strings.TrimSpace(stringMapValue(output, "terminal_error")) != "" {
		return false
	}
	if strings.EqualFold(stringMapValue(output, "status"), "worker_run_active") {
		return true
	}
	if value, ok := boolMapValue(output, "finish_ready"); ok && !value {
		return true
	}
	switch strings.ToLower(stringMapValue(output, "state")) {
	case strings.ToLower(string(types.RunPending)), strings.ToLower(string(types.RunRunning)):
		if stringMapValue(output, "terminal_error") == "" && stringMapValue(output, "completion_blocker") == "" {
			return true
		}
	}
	return false
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
	exportCount := len(mapSliceValue(output, "app_change_packages"))

	findings := []string{
		fmt.Sprintf("delegate_worker_vm returned status %q with worker state %q before super reported back to VText.", firstNonEmpty(status, "unknown"), firstNonEmpty(state, "unknown")),
		fmt.Sprintf("super run ended with %s; preserving delegate evidence as a structured worker update.", firstNonEmpty(string(rec.State), "terminal state")),
	}
	if exportCount > 0 {
		findings = append(findings, fmt.Sprintf("delegate_worker_vm returned %d AppChangePackage(s).", exportCount))
	} else {
		findings = append(findings, "delegate_worker_vm returned no AppChangePackages; treat this as blocked below package-level.")
	}
	if eventCount > 0 {
		findings = append(findings, fmt.Sprintf("worker event summary was preserved with %d event(s) and %d channel message(s).", eventCount, channelMessages))
	}
	provenance := delegateWorkerStructuredProvenance(output)
	findings = append(findings, provenance.Findings...)

	evidenceIDs := []string{"event:" + ev.EventID, "run:" + rec.RunID}
	if workerLoopID != "" {
		evidenceIDs = append(evidenceIDs, "worker_loop:"+workerLoopID)
	}
	evidenceIDs = append(evidenceIDs, provenance.EvidenceIDs...)

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
	refs = append(refs, provenance.Refs...)
	artifacts := append(exportArtifactRefs(output), provenance.Artifacts...)
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
	notes = append(notes, provenance.Notes...)

	return types.WorkerUpdateRecord{
		UpdateID:      "delegate-worker-vm-" + sanitizeExportPart(rec.RunID),
		OwnerID:       rec.OwnerID,
		AgentID:       agentIDForRun(rec),
		TargetAgentID: targetAgentID,
		ChannelID:     channelID,
		TrajectoryID:  metadataStringValue(rec.Metadata, runMetadataTrajectoryID),
		Role:          AgentProfileSuper,
		Findings:      trimDedupeNonEmpty(findings),
		EvidenceIDs:   trimDedupeNonEmpty(evidenceIDs),
		Artifacts:     trimDedupeNonEmpty(artifacts),
		Refs:          trimDedupeNonEmpty(refs),
		Proposals: []string{
			"Continue with a termination/package probe that makes vsuper call publish_app_change_package or submit_worker_update before delegate timeout.",
		},
		Notes:     trimDedupeNonEmpty(notes),
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
	exportCount := len(mapSliceValue(output, "app_change_packages"))

	findings := []string{
		fmt.Sprintf("delegate_worker_vm returned status %q with worker state %q.", firstNonEmpty(status, "unknown"), firstNonEmpty(state, "unknown")),
		"super preserved this delegate result as a VText worker-update checkpoint before relying on another LLM turn.",
	}
	if exportCount > 0 {
		findings = append(findings, fmt.Sprintf("delegate_worker_vm returned %d AppChangePackage(s).", exportCount))
	} else {
		findings = append(findings, "delegate_worker_vm returned no AppChangePackages; treat this as blocked below package-level.")
	}
	if eventCount > 0 {
		findings = append(findings, fmt.Sprintf("worker event summary was preserved with %d event(s) and %d channel message(s).", eventCount, channelMessages))
	}
	provenance := delegateWorkerStructuredProvenance(output)
	findings = append(findings, provenance.Findings...)

	evidenceIDs := []string{"run:" + rec.RunID}
	if workerLoopID != "" {
		evidenceIDs = append(evidenceIDs, "worker_loop:"+workerLoopID)
	}
	evidenceIDs = append(evidenceIDs, provenance.EvidenceIDs...)
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
	refs = append(refs, provenance.Refs...)
	notes := []string{
		"auto_synthesized_from=delegate_worker_vm_checkpoint",
		"checkpoint_source=" + firstNonEmpty(source, "delegate_worker_vm_result"),
	}
	if terminalError != "" {
		notes = append(notes, "delegate_terminal_error="+terminalError)
	}
	if delegateWorkerContinuationRequired(output) {
		notes = append(notes, "active_worker_obligation=true")
		if workerLoopID != "" {
			notes = append(notes, "continuation_worker_run_id="+workerLoopID)
		}
	}
	if eventCount > 0 {
		notes = append(notes, fmt.Sprintf("worker_event_count=%d", eventCount))
	}
	notes = append(notes, provenance.Notes...)
	updateKey := delegateWorkerCheckpointUpdateKey(output, source)
	return types.WorkerUpdateRecord{
		UpdateID:      delegateWorkerCheckpointUpdateID(rec, output, updateKey),
		OwnerID:       rec.OwnerID,
		AgentID:       agentIDForRun(rec),
		TargetAgentID: targetAgentID,
		ChannelID:     channelID,
		TrajectoryID:  metadataStringValue(rec.Metadata, runMetadataTrajectoryID),
		Role:          AgentProfileSuper,
		Findings:      trimDedupeNonEmpty(findings),
		EvidenceIDs:   trimDedupeNonEmpty(evidenceIDs),
		Artifacts:     trimDedupeNonEmpty(append(exportArtifactRefs(output), provenance.Artifacts...)),
		Refs:          trimDedupeNonEmpty(refs),
		Proposals: []string{
			"Continue with a termination/package probe that prevents candidate checkout races and requires publish_app_change_package or submit_worker_update before delegate timeout.",
		},
		Notes:     trimDedupeNonEmpty(notes),
		CreatedAt: now,
	}
}

func delegateWorkerCheckpointUpdateID(rec *types.RunRecord, output map[string]any, updateKey string) string {
	prefix := "delegate-worker-vm-checkpoint-"
	if delegateWorkerContinuationRequired(output) {
		return prefix + sanitizeExportPart(updateKey)
	}
	runPart := ""
	if rec != nil {
		runPart = sanitizeExportPart(rec.RunID)
	}
	if runPart == "" {
		return prefix + sanitizeExportPart(updateKey)
	}
	return prefix + runPart + "-" + sanitizeExportPart(updateKey)
}

func delegateWorkerCheckpointUpdateKey(output map[string]any, source string) string {
	workerLoopID := firstNonEmpty(stringMapValue(output, "worker_run_id"), stringMapValue(output, "loop_id"))
	status := stringMapValue(output, "status")
	parts := []string{
		firstNonEmpty(workerLoopID, status, "result"),
	}
	if delegateWorkerContinuationRequired(output) {
		if source = strings.TrimSpace(source); source != "" {
			parts = append(parts, source)
		}
		if eventCount := intMapValue(output, "event_count"); eventCount > 0 {
			parts = append(parts, fmt.Sprintf("events-%d", eventCount))
		}
		if channelMessages := intMapValue(output, "worker_channel_message_count"); channelMessages > 0 {
			parts = append(parts, fmt.Sprintf("messages-%d", channelMessages))
		}
		if children := stringSliceMapValue(output, "worker_child_run_ids"); len(children) > 0 {
			parts = append(parts, fmt.Sprintf("children-%d", len(children)))
		}
	}
	return strings.Join(trimDedupeNonEmpty(parts), "-")
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
	value := m[key]
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case fmt.Stringer:
		return strings.TrimSpace(typed.String())
	}
	rv := reflect.ValueOf(value)
	if rv.IsValid() && rv.Kind() == reflect.String {
		return strings.TrimSpace(rv.String())
	}
	return ""
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

func boolMapValue(m map[string]any, key string) (bool, bool) {
	value, ok := m[key]
	if !ok {
		return false, false
	}
	switch typed := value.(type) {
	case bool:
		return typed, true
	case string:
		switch strings.ToLower(strings.TrimSpace(typed)) {
		case "true":
			return true, true
		case "false":
			return false, true
		}
	}
	return false, false
}

func mapSliceValue(m map[string]any, key string) []map[string]any {
	switch raw := m[key].(type) {
	case []map[string]any:
		out := make([]map[string]any, 0, len(raw))
		for _, item := range raw {
			if item != nil {
				out = append(out, item)
			}
		}
		return out
	case []any:
		out := make([]map[string]any, 0, len(raw))
		for _, item := range raw {
			if entry, ok := item.(map[string]any); ok {
				out = append(out, entry)
			}
		}
		return out
	default:
		return nil
	}
}

func exportArtifactRefs(output map[string]any) []string {
	exports := mapSliceValue(output, "app_change_packages")
	refs := []string{}
	for _, export := range exports {
		for _, key := range []string{"package_id", "package_manifest_sha256", "runtime_source_delta_sha256", "ui_source_delta_sha256"} {
			if value := stringMapValue(export, key); value != "" {
				refs = append(refs, value)
			}
		}
	}
	return refs
}

type delegateWorkerProvenance struct {
	Findings    []string
	EvidenceIDs []string
	Refs        []string
	Artifacts   []string
	Notes       []string
}

func delegateWorkerStructuredProvenance(output map[string]any) delegateWorkerProvenance {
	var provenance delegateWorkerProvenance
	for _, childRunID := range stringSliceMapValue(output, "worker_child_run_ids") {
		provenance.EvidenceIDs = append(provenance.EvidenceIDs, "worker_child_loop:"+childRunID)
		provenance.Refs = append(provenance.Refs, "worker_child_loop:"+childRunID)
	}
	if counts := stringIntMapValue(output, "worker_child_event_counts"); len(counts) > 0 {
		for _, runID := range sortedMapKeys(counts) {
			provenance.Notes = append(provenance.Notes, fmt.Sprintf("worker_child_event_count:%s=%d", runID, counts[runID]))
		}
	}
	if rootCount := intMapValue(output, "worker_root_event_count"); rootCount > 0 {
		provenance.Notes = append(provenance.Notes, fmt.Sprintf("worker_root_event_count=%d", rootCount))
	}
	if profiles := stringSliceMapValue(output, "worker_spawned_profiles"); len(profiles) > 0 {
		provenance.Findings = append(provenance.Findings, "worker spawned child profile(s): "+strings.Join(profiles, ", "))
	}

	for _, export := range mapSliceValue(output, "app_change_packages") {
		provenance = appendExportProvenance(provenance, export)
	}
	for _, adoption := range mapSliceValue(output, "app_adoptions") {
		provenance = appendAdoptionProvenance(provenance, adoption)
	}
	for _, item := range mapSliceValue(output, "worker_event_summary") {
		provenance = appendWorkerEventProvenance(provenance, item)
	}
	return provenance
}

func appendExportProvenance(provenance delegateWorkerProvenance, export map[string]any) delegateWorkerProvenance {
	packageID := stringMapValue(export, "package_id")
	baseSHA := stringMapValue(export, "base_sha")
	workerHead := firstNonEmpty(stringMapValue(export, "candidate_head_sha"), stringMapValue(export, "worker_head_sha"), stringMapValue(export, "worker_head"))
	manifestSHA := stringMapValue(export, "package_manifest_sha256")
	runtimeDeltaSHA := stringMapValue(export, "runtime_source_delta_sha256")
	uiDeltaSHA := stringMapValue(export, "ui_source_delta_sha256")
	loopID := stringMapValue(export, "loop_id")
	if packageID != "" {
		provenance.EvidenceIDs = append(provenance.EvidenceIDs, "app_change_package:"+packageID)
		provenance.Refs = append(provenance.Refs, "app_change_package:"+packageID)
	}
	if baseSHA != "" {
		provenance.Refs = append(provenance.Refs, "package_base_sha:"+baseSHA)
	}
	if workerHead != "" {
		provenance.Refs = append(provenance.Refs, "worker_head:"+workerHead)
	}
	if manifestSHA != "" {
		provenance.Refs = append(provenance.Refs, "package_manifest_sha256:"+manifestSHA)
	}
	if runtimeDeltaSHA != "" {
		provenance.Refs = append(provenance.Refs, "runtime_source_delta_sha256:"+runtimeDeltaSHA)
	}
	if uiDeltaSHA != "" {
		provenance.Refs = append(provenance.Refs, "ui_source_delta_sha256:"+uiDeltaSHA)
	}
	if loopID != "" {
		provenance.EvidenceIDs = append(provenance.EvidenceIDs, "package_loop:"+loopID)
	}
	details := []string{}
	for _, detail := range []string{
		"package_id=" + packageID,
		"base_sha=" + baseSHA,
		"worker_head=" + workerHead,
		"manifest_sha256=" + manifestSHA,
		"runtime_delta_sha256=" + runtimeDeltaSHA,
		"ui_delta_sha256=" + uiDeltaSHA,
	} {
		if !strings.HasSuffix(detail, "=") {
			details = append(details, detail)
		}
	}
	if len(details) > 0 {
		provenance.Findings = append(provenance.Findings, "worker package evidence: "+strings.Join(details, "; "))
	}
	return provenance
}

func appendAdoptionProvenance(provenance delegateWorkerProvenance, adoption map[string]any) delegateWorkerProvenance {
	adoptionID := firstNonEmpty(stringMapValue(adoption, "adoption_id"), stringMapValue(adoption, "adoptionID"))
	if adoptionID == "" {
		return provenance
	}
	provenance.EvidenceIDs = append(provenance.EvidenceIDs, "app_adoption:"+adoptionID)
	provenance.Refs = append(provenance.Refs, "app_adoption:"+adoptionID)
	details := []string{"id=" + adoptionID}
	for _, key := range []string{"status", "package_id", "target_computer_id", "target_candidate_id", "candidate_source_ref", "runtime_artifact_digest", "ui_artifact_digest"} {
		if value := stringMapValue(adoption, key); value != "" {
			details = append(details, key+"="+value)
		}
	}
	provenance.Findings = append(provenance.Findings, "App adoption evidence: "+strings.Join(details, "; "))
	return provenance
}

func appendWorkerEventProvenance(provenance delegateWorkerProvenance, item map[string]any) delegateWorkerProvenance {
	kind := stringMapValue(item, "kind")
	tool := stringMapValue(item, "tool")
	role := stringMapValue(item, "role")
	fromAgentID := stringMapValue(item, "from_agent_id")
	toAgentID := stringMapValue(item, "to_agent_id")
	if kind == "tool.result" && tool == "spawn_agent" {
		if output := parseJSONMapString(stringMapValue(item, "output_excerpt")); output != nil {
			provenance = appendSpawnAgentProvenance(provenance, output)
		}
	}
	if kind == "tool.result" && tool == "publish_app_change_package" {
		if output := parseJSONMapString(stringMapValue(item, "output_excerpt")); output != nil {
			provenance = appendExportProvenance(provenance, output)
		}
	}
	if fromAgentID != "" || toAgentID != "" {
		ref := "worker_channel_message"
		if fromAgentID != "" || toAgentID != "" {
			ref += ":" + fromAgentID + "->" + toAgentID
		}
		provenance.Refs = append(provenance.Refs, ref)
		content := compactEvidenceText(stringMapValue(item, "content_excerpt"), 260)
		parts := []string{}
		for _, part := range []string{
			"role=" + role,
			"from=" + fromAgentID,
			"to=" + toAgentID,
			"content=" + content,
		} {
			if !strings.HasSuffix(part, "=") {
				parts = append(parts, part)
			}
		}
		if len(parts) > 0 {
			provenance.Notes = append(provenance.Notes, "worker_channel_message:"+strings.Join(parts, "; "))
		}
	}
	if isError, ok := item["is_error"].(bool); ok && isError {
		provenance.Notes = append(provenance.Notes, "worker_tool_error:"+firstNonEmpty(tool, kind))
	}
	return provenance
}

func appendSpawnAgentProvenance(provenance delegateWorkerProvenance, output map[string]any) delegateWorkerProvenance {
	agentID := stringMapValue(output, "agent_id")
	loopID := firstNonEmpty(stringMapValue(output, "loop_id"), stringMapValue(output, "run_id"))
	channelID := stringMapValue(output, "channel_id")
	profile := firstNonEmpty(stringMapValue(output, "profile"), stringMapValue(output, "role"))
	state := stringMapValue(output, "state")
	if agentID != "" {
		provenance.EvidenceIDs = append(provenance.EvidenceIDs, "worker_child_agent:"+agentID)
		provenance.Refs = append(provenance.Refs, "worker_child_agent:"+agentID)
	}
	if loopID != "" {
		provenance.EvidenceIDs = append(provenance.EvidenceIDs, "worker_child_loop:"+loopID)
		provenance.Refs = append(provenance.Refs, "worker_child_loop:"+loopID)
	}
	if channelID != "" {
		provenance.EvidenceIDs = append(provenance.EvidenceIDs, "worker_channel:"+channelID)
		provenance.Refs = append(provenance.Refs, "worker_channel:"+channelID)
	}
	details := []string{}
	for _, detail := range []string{
		"profile=" + profile,
		"agent_id=" + agentID,
		"loop_id=" + loopID,
		"channel_id=" + channelID,
		"state=" + state,
	} {
		if !strings.HasSuffix(detail, "=") {
			details = append(details, detail)
		}
	}
	if len(details) > 0 {
		provenance.Findings = append(provenance.Findings, "worker spawned child agent: "+strings.Join(details, "; "))
	}
	return provenance
}

func stringSliceMapValue(m map[string]any, key string) []string {
	switch raw := m[key].(type) {
	case []string:
		return trimDedupeNonEmpty(raw)
	case []any:
		out := make([]string, 0, len(raw))
		for _, item := range raw {
			switch typed := item.(type) {
			case string:
				out = append(out, typed)
			case fmt.Stringer:
				out = append(out, typed.String())
			default:
				rv := reflect.ValueOf(item)
				if rv.IsValid() && rv.Kind() == reflect.String {
					out = append(out, rv.String())
				}
			}
		}
		return trimDedupeNonEmpty(out)
	default:
		return nil
	}
}

func stringIntMapValue(m map[string]any, key string) map[string]int {
	out := map[string]int{}
	switch raw := m[key].(type) {
	case map[string]int:
		for k, v := range raw {
			if strings.TrimSpace(k) != "" {
				out[strings.TrimSpace(k)] = v
			}
		}
	case map[string]any:
		for k, v := range raw {
			if strings.TrimSpace(k) == "" {
				continue
			}
			out[strings.TrimSpace(k)] = anyToInt(v)
		}
	}
	for key, value := range out {
		if value == 0 {
			delete(out, key)
		}
	}
	return out
}

func anyToInt(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case json.Number:
		parsed, _ := typed.Int64()
		return int(parsed)
	default:
		return 0
	}
}

func sortedMapKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func parseJSONMapString(raw string) map[string]any {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var mapped map[string]any
	if err := json.Unmarshal([]byte(raw), &mapped); err != nil {
		return nil
	}
	return mapped
}

func compactEvidenceText(raw string, limit int) string {
	raw = strings.Join(strings.Fields(strings.TrimSpace(raw)), " ")
	if limit <= 0 || len(raw) <= limit {
		return raw
	}
	return strings.TrimSpace(raw[:limit]) + "..."
}

func trimDedupeNonEmpty(items []string) []string {
	out := make([]string, 0, len(items))
	seen := map[string]bool{}
	for _, item := range trimNonEmpty(items) {
		if seen[item] {
			continue
		}
		seen[item] = true
		out = append(out, item)
	}
	return out
}
