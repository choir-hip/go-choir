package agentcore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/buildinfo"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/vocabmigrate"
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

	build := buildinfo.Snapshot("autoputer")
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

	directionResults := collectAcceptanceToolResults(events, "apply_texture_turn")
	if len(directionResults) > 0 {
		item := directionResults[0]
		ref := builder.addEventEvidence(item.event, "Texture committed a typed lifecycle direction opener", map[string]any{
			"tool": "apply_texture_turn", "command_id": payloadString(item.output, "command_id"),
		})
		builder.addCheckpoint("super_direction_opened", "passed", item.event.Timestamp, item.event.StreamSeq, []string{ref}, map[string]any{
			"command_id": payloadString(item.output, "command_id"), "receipt_kind": "apply_texture_turn",
		})
	}
	// The document-channel cast's opener (path B's engineering desk) is
	// deleted (C1e step 3): an assignment opens from management's Cast.

	addAcceptanceDurableAgentCapsuleCheckpoints(ctx, rt, &builder, trajectoryRuns, events)

	addAcceptanceCompactionCheckpoint(&builder, events)

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
	return strings.Join(roles, vocabmigrate.AuthorityPathSeparator)
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
		URL:        runEventEvidenceURL(ev),
		Details:    details,
	})
	return refID
}

// runEventEvidenceURL names the served, owner-scoped read that returns
// exactly this event: the page of its run's events that starts at its seq.
// A runtime-level event with no run has no served per-event read.
func runEventEvidenceURL(ev types.EventRecord) string {
	if strings.TrimSpace(ev.RunID) == "" {
		return ""
	}
	after := ev.Seq - 1
	if after < 0 {
		after = 0
	}
	return fmt.Sprintf("/api/runs/%s/events?after=%d&limit=1", url.PathEscape(ev.RunID), after)
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

func addAcceptanceDurableAgentCapsuleCheckpoints(ctx context.Context, rt *Runtime, builder *acceptanceBuilder, runs []types.RunRecord, events []types.EventRecord) {
	var completedRefs []string
	var implRun, verifyRun *types.RunRecord
	for i := range runs {
		run := runs[i]
		if traceRunProfile(run) != agentprofile.Engineering || run.State != types.RunCompleted {
			continue
		}
		completedRefs = append(completedRefs, builder.addRunEvidence(run, "durable Engineering run completed"))
		switch metadataStringValue(run.Metadata, "assignment_kind") {
		case string(types.EngineeringAssignmentImplementation):
			implRun = &runs[i]
		case string(types.EngineeringAssignmentVerification):
			verifyRun = &runs[i]
		}
	}
	if len(completedRefs) > 0 {
		builder.addCheckpoint("durable_agent_completed", "passed", time.Time{}, 0, completedRefs, map[string]any{
			"role": agentprofile.Engineering, "run_count": len(completedRefs),
		})
	}
	// Freeze/verify evidence is canonical operation state, not tool names:
	// the reducer commits the selfdev operation transition, and acceptance
	// reads the operation row.
	var operation *selfdev.Operation
	if rt != nil && rt.selfdevOperations != nil {
		// A trajectory-wide caller (C1e D2): list and pick explicitly.
		if ops, err := rt.selfdevOperations.ListByTrajectory(ctx, runsComputerID(runs), trajectoryIDFromRuns(runs)); err == nil {
			if op, ok := pickTrajectoryOperation(ops); ok {
				operation = &op
			}
		}
	}
	// Freeze evidence is the durable bundle digest on the operation row, not
	// a state allowlist: every state reachable after frozen (verified,
	// awaiting_approval, accepted, applied, failed, rolled_back, degraded)
	// preserves the freeze fact while the digest is bound. A verify-fail
	// trajectory keeps its frozen bundle; the checkpoint must not misreport
	// that history as absence. Freeze evidence is only expected when a
	// self-development operation is bound to this trajectory; a
	// document-channel task without an operation has no bundle to freeze.
	if operation != nil && operation.BundleDigest != "" {
		builder.addCheckpoint("capsule_effect_frozen", "passed", time.Time{}, 0, nil, map[string]any{
			"operation_id": operation.OperationID, "bundle_digest": operation.BundleDigest, "state": operation.State,
		})
	} else if operation != nil && implRun != nil {
		builder.addCheckpoint("capsule_effect_frozen", "failed", time.Time{}, 0, nil, map[string]any{
			"detail": "completed implementation run without a frozen self-development bundle",
		})
	}
	if operation != nil && len(operation.VerifierRefs) > 0 {
		builder.addCheckpoint("capsule_verification_recorded", "passed", time.Time{}, 0, nil, map[string]any{
			"operation_id": operation.OperationID, "verifier_refs": operation.VerifierRefs, "state": operation.State,
		})
	} else if verifyRun != nil {
		builder.addCheckpoint("capsule_verification_recorded", "failed", time.Time{}, 0, nil, map[string]any{
			"detail": "completed verification run without a recorded verifier decision",
		})
	}
}

func runsComputerID(runs []types.RunRecord) string {
	for _, run := range runs {
		if run.ComputerID != "" {
			return run.ComputerID
		}
	}
	return ""
}

func trajectoryIDFromRuns(runs []types.RunRecord) string {
	for _, run := range runs {
		if id := traceTrajectoryIDForRun(run); id != "" {
			return id
		}
	}
	return ""
}

func addAcceptanceCompactionCheckpoint(builder *acceptanceBuilder, events []types.EventRecord) {
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

func acceptanceLevelAndState(checkpoints []types.RunAcceptanceCheckpoint) (types.RunAcceptanceLevel, types.RunAcceptanceState) {
	has := map[string]bool{}
	capsuleFailed := false
	for _, checkpoint := range checkpoints {
		if checkpoint.State == "passed" {
			has[checkpoint.Kind] = true
		}
		// A failed capsule evidence checkpoint is loud AND gating: a
		// completed implementation/verification run without its freeze/verify
		// evidence must not report an accepted level.
		if checkpoint.State == "failed" &&
			(checkpoint.Kind == "capsule_effect_frozen" || checkpoint.Kind == "capsule_verification_recorded") {
			capsuleFailed = true
		}
	}
	level := types.RunAcceptanceDocsLevel
	state := types.RunAcceptanceBlocked
	textureOpened := has[runAcceptanceCheckpointTextureOpened]
	if has["submitted"] && textureOpened {
		level = types.RunAcceptanceStagingSmokeLevel
	}
	if has["submitted"] && textureOpened && has["super_direction_opened"] && !capsuleFailed {
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
		"super_direction_opened":        1,
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
