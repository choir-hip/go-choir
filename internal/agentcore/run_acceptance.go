package agentcore

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

// SynthesizeRunAcceptance derives a trajectory-level diagnostic from durable
// runs, trace events, and guest-local capsule evidence. It never proves a
// self-development decision, materialization, checkpoint, or route transition.
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
		run, err := rt.getRunForComputer(ctx, ownerID, in.RunID)
		if err != nil {
			return types.RunAcceptanceRecord{}, fmt.Errorf("synthesize run acceptance: load source run: %w", err)
		}
		if run.OwnerID != ownerID {
			return types.RunAcceptanceRecord{}, fmt.Errorf("synthesize run acceptance: source run not found")
		}
		in.TrajectoryID = traceTrajectoryIDForRun(run)
	}

	runs, err := rt.ListRunsByOwner(ctx, ownerID, 1000)
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

	addAcceptanceDurableAgentCapsuleCheckpoints(&builder, trajectoryRuns, events)

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

func addAcceptanceDurableAgentCapsuleCheckpoints(builder *acceptanceBuilder, runs []types.RunRecord, events []types.EventRecord) {
	var completedRefs []string
	for _, run := range runs {
		if traceRunProfile(run) != agentprofile.CoSuper || run.State != types.RunCompleted {
			continue
		}
		completedRefs = append(completedRefs, builder.addRunEvidence(run, "durable CoSuper run completed"))
	}
	if len(completedRefs) > 0 {
		builder.addCheckpoint("durable_agent_completed", "passed", time.Time{}, 0, completedRefs, map[string]any{
			"role": agentprofile.CoSuper, "run_count": len(completedRefs),
		})
	}
	if results := collectAcceptanceToolResults(events, "commit_transaction"); len(results) > 0 {
		item := results[len(results)-1]
		ref := builder.addEventEvidence(item.event, "guest-local capsule effect bundle frozen", map[string]any{
			"tool": "commit_transaction", "bundle_digest": payloadString(item.output, "bundle_digest"),
		})
		builder.addCheckpoint("capsule_effect_frozen", "passed", item.event.Timestamp, item.event.StreamSeq, []string{ref}, map[string]any{
			"bundle_digest": payloadString(item.output, "bundle_digest"),
		})
	}
	if results := collectAcceptanceToolResults(events, "record_self_development_verification"); len(results) > 0 {
		item := results[len(results)-1]
		ref := builder.addEventEvidence(item.event, "independent capsule verification recorded", map[string]any{
			"tool":               "record_self_development_verification",
			"verification_event": payloadString(item.output, "verification_event"),
		})
		builder.addCheckpoint("capsule_verification_recorded", "passed", item.event.Timestamp, item.event.StreamSeq, []string{ref}, map[string]any{
			"verification_event": payloadString(item.output, "verification_event"),
		})
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
	if has["submitted"] && textureOpened && has["super_requested"] {
		state = types.RunAcceptanceAccepted
	}
	// RunAcceptance is trajectory diagnostics only. Export, promotion, and
	// continuation claims require the canonical ComputerEvent/checkpoint/route
	// path and are deliberately unavailable here.
	return level, state
}

func buildAcceptanceInvariantChecks(rec types.RunAcceptanceRecord) []types.RunAcceptanceInvariantCheck {
	kindSet := map[string]bool{}
	for _, checkpoint := range rec.Checkpoints {
		if checkpoint.State == "passed" {
			kindSet[checkpoint.Kind] = true
		}
	}
	textureOpened := kindSet[runAcceptanceCheckpointTextureOpened]
	promptPathObserved := kindSet["submitted"] && textureOpened
	durableAgentObserved := kindSet["durable_agent_completed"]
	return []types.RunAcceptanceInvariantCheck{
		{
			Name:   "trajectory_path_observed",
			State:  stateForBool(promptPathObserved || durableAgentObserved),
			Detail: "diagnostic evidence is derived from durable runs and trace events",
		},
		{
			Name:   "capsule_effect_bounded",
			State:  "passed",
			Detail: "a frozen capsule effect remains speculative; this record has no decision or materialization authority",
		},
		{
			Name:   "core_acceptance_not_claimed",
			State:  "passed",
			Detail: "canonical ComputerEvent, checkpoint, and route receipts are outside RunAcceptance authority",
		},
		{
			Name:   "checkpoint_causal_order",
			State:  stateForBool(acceptanceCheckpointPhaseOrderOK(rec.Checkpoints)),
			Detail: "passed run and capsule checkpoints are causal where trace events provide stream_seq",
		},
	}
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
		"super_requested":               1,
		"durable_agent_completed":       2,
		"capsule_effect_frozen":         3,
		"capsule_verification_recorded": 4,
		"compacted":                     5,
		"continued":                     6,
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
	for phase := 1; phase <= 6; phase++ {
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
	hasCapsuleEvidence := false
	for _, checkpoint := range rec.Checkpoints {
		if checkpoint.State == "passed" && (checkpoint.Kind == "capsule_effect_frozen" || checkpoint.Kind == "capsule_verification_recorded") {
			hasCapsuleEvidence = true
		}
	}
	return []types.RunAcceptanceVerifierContract{
		{
			Name:    "trace-derived-trajectory",
			Purpose: "derive diagnostic checkpoints from durable run and trace evidence",
			State:   stateForBool(len(rec.Checkpoints) > 0 && len(rec.EvidenceRefs) > 0),
		},
		{
			Name:    "capsule-evidence-is-non-authoritative",
			Purpose: "preserve capsule evidence without treating it as an accepted ComputerEvent",
			State:   stateForBool(!hasCapsuleEvidence || rec.AcceptanceLevel == types.RunAcceptanceStagingSmokeLevel),
		},
	}
}

func buildAcceptanceResidualRisks(rec types.RunAcceptanceRecord) []string {
	risks := []string{"RunAcceptance is trajectory diagnostics only; self-development acceptance requires canonical event, checkpoint, materialization, and route receipts"}
	if !acceptanceRecordHasPassedCheckpoint(rec, "compacted") {
		risks = append(risks, "continuation diagnostics remain incomplete until run-memory compaction evidence is recorded")
	}
	for _, check := range rec.InvariantChecks {
		if check.State == "blocked" {
			risks = append(risks, fmt.Sprintf("acceptance invariant %s is blocked: %s", check.Name, check.Detail))
		}
	}
	return risks
}
