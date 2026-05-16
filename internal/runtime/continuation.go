package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// ContinuationProposal is the runtime's bounded next-goal selection after a
// run has produced enough memory to keep learning without manual re-prompting.
type ContinuationProposal struct {
	Objective        string
	Reason           string
	AuthorityProfile string
	LeaseSeconds     int
	Details          map[string]any
}

// SynthesizeRunContinuation chooses the next bounded objective from durable
// runtime signals. It is intentionally deterministic and conservative: queued
// promotion work beats open-ended mission continuation, and it only returns a
// proposal for SelectRunContinuation to record through the normal compaction
// and fingerprint path.
func (rt *Runtime) SynthesizeRunContinuation(ctx context.Context, sourceRunID, ownerID string) (ContinuationProposal, error) {
	if rt == nil || rt.store == nil {
		return ContinuationProposal{}, fmt.Errorf("synthesize run continuation: runtime store is unavailable")
	}
	source, err := rt.GetRun(ctx, sourceRunID, ownerID)
	if err != nil {
		return ContinuationProposal{}, err
	}
	candidates, err := rt.store.ListPromotionCandidates(ctx, ownerID, 100)
	if err != nil {
		return ContinuationProposal{}, fmt.Errorf("synthesize run continuation: list promotion candidates: %w", err)
	}
	if proposal, ok := continuationFromPromotionCandidates(source, candidates); ok {
		return proposal, nil
	}
	missionDoc := metadataStringValue(source.Metadata, "mission_doc")
	if missionDoc == "" {
		missionDoc = "docs/mission-choir-grand-deformation-v0.md"
	}
	return ContinuationProposal{
		Objective:        "Continue " + missionDoc + " by selecting the next verifier-dense Choir-in-Choir deformation from run memory, Trace, promotion state, and product gaps.",
		Reason:           "run control memory found no pending promotion candidate, so it falls back to mission-gradient continuation",
		AuthorityProfile: AgentProfileVSuper,
		LeaseSeconds:     4 * 60 * 60,
		Details: map[string]any{
			"selection_source": "run_control_memory",
			"signal":           "mission_gradient",
			"mission_doc":      missionDoc,
		},
	}, nil
}

// SelectSynthesizedRunContinuation records the deterministic controller choice
// as a normal continuation. It does not start the continuation or mutate
// canonical state.
func (rt *Runtime) SelectSynthesizedRunContinuation(ctx context.Context, sourceRunID, ownerID string) (types.RunContinuationRecord, error) {
	proposal, err := rt.SynthesizeRunContinuation(ctx, sourceRunID, ownerID)
	if err != nil {
		return types.RunContinuationRecord{}, err
	}
	return rt.SelectRunContinuation(ctx, sourceRunID, ownerID, proposal)
}

// SelectRunContinuation records the next objective for a completed or blocked
// run. It first attempts a durable compaction checkpoint so the continuation has
// operational memory rather than only chat transcript state.
func (rt *Runtime) SelectRunContinuation(ctx context.Context, sourceRunID, ownerID string, proposal ContinuationProposal) (types.RunContinuationRecord, error) {
	if rt == nil || rt.store == nil {
		return types.RunContinuationRecord{}, fmt.Errorf("select run continuation: runtime store is unavailable")
	}
	source, err := rt.GetRun(ctx, sourceRunID, ownerID)
	if err != nil {
		return types.RunContinuationRecord{}, err
	}
	if source.State != types.RunCompleted && source.State != types.RunBlocked {
		return types.RunContinuationRecord{}, fmt.Errorf("select run continuation: source run state %s is not continuable", source.State)
	}
	objective := strings.TrimSpace(proposal.Objective)
	if objective == "" {
		return types.RunContinuationRecord{}, fmt.Errorf("select run continuation: objective is required")
	}
	profile, err := boundedContinuationProfile(proposal.AuthorityProfile)
	if err != nil {
		return types.RunContinuationRecord{}, err
	}
	leaseSeconds := proposal.LeaseSeconds
	if leaseSeconds <= 0 {
		leaseSeconds = 8 * 60 * 60
	}
	if leaseSeconds > 24*60*60 {
		leaseSeconds = 24 * 60 * 60
	}
	details := cloneDetailsMap(proposal.Details)
	details["source_state"] = string(source.State)
	details["compaction_required"] = true
	fingerprint := objectiveFingerprint(ownerID, metadataStringValue(source.Metadata, runMetadataTrajectoryID), sourceRunID, objective)
	details["objective_fingerprint"] = fingerprint
	if existing, ok, err := rt.existingRunContinuationForObjective(ctx, ownerID, sourceRunID, fingerprint); err != nil {
		return types.RunContinuationRecord{}, err
	} else if ok {
		return existing, nil
	}
	if err := rt.CompactRunMemory(ctx, sourceRunID, ownerID, "continuation_selection"); err != nil {
		details["compaction_status"] = "skipped"
		details["compaction_error"] = err.Error()
	} else {
		details["compaction_status"] = "completed"
	}

	rec, err := rt.store.CreateRunContinuation(ctx, types.RunContinuationRecord{
		OwnerID:          ownerID,
		SourceRunID:      sourceRunID,
		Objective:        objective,
		Reason:           strings.TrimSpace(proposal.Reason),
		AuthorityProfile: profile,
		LeaseSeconds:     leaseSeconds,
		Status:           types.RunContinuationSelected,
		Details:          details,
	})
	if err != nil {
		return types.RunContinuationRecord{}, err
	}
	rt.emitContinuationEvent(ctx, source, rec, types.EventRunContinuationSelected)
	return rec, nil
}

// StartRunContinuation starts a selected continuation as a child run and marks
// the continuation record as started.
func (rt *Runtime) StartRunContinuation(ctx context.Context, ownerID, continuationID string) (types.RunContinuationRecord, error) {
	if rt == nil || rt.store == nil {
		return types.RunContinuationRecord{}, fmt.Errorf("start run continuation: runtime store is unavailable")
	}
	rec, err := rt.store.GetRunContinuation(ctx, ownerID, continuationID)
	if err != nil {
		return types.RunContinuationRecord{}, err
	}
	if rec.Status != types.RunContinuationSelected {
		return rec, fmt.Errorf("start run continuation: continuation status %s is not selected", rec.Status)
	}
	profile, err := boundedContinuationProfile(rec.AuthorityProfile)
	if err != nil {
		return rec, err
	}
	metadata := map[string]any{
		runMetadataAgentProfile: profile,
		runMetadataAgentRole:    profile,
		"request_source":        "run_continuation",
		"continuation_id":       rec.ContinuationID,
		"lease_seconds":         rec.LeaseSeconds,
	}
	if fingerprint := detailStringValue(rec.Details, "objective_fingerprint"); fingerprint != "" {
		metadata["objective_fingerprint"] = fingerprint
	}
	child, err := rt.StartChildRun(ctx, rec.SourceRunID, rec.Objective, ownerID, metadata)
	if err != nil {
		rec.Status = types.RunContinuationBlocked
		rec.Details = cloneDetailsMap(rec.Details)
		rec.Details["start_error"] = err.Error()
		if updated, updateErr := rt.store.UpdateRunContinuation(ctx, rec); updateErr == nil {
			rec = updated
		}
		return rec, err
	}
	rec.NextRunID = child.RunID
	rec.Status = types.RunContinuationStarted
	rec.AuthorityProfile = profile
	rec.Details = cloneDetailsMap(rec.Details)
	rec.Details["next_agent_id"] = child.AgentID
	rec.Details["next_channel_id"] = child.ChannelID
	rec, err = rt.store.UpdateRunContinuation(ctx, rec)
	if err != nil {
		return types.RunContinuationRecord{}, err
	}
	if source, err := rt.GetRun(ctx, rec.SourceRunID, ownerID); err == nil {
		rt.emitContinuationEvent(ctx, source, rec, types.EventRunContinuationStarted)
	}
	return rec, nil
}

func (rt *Runtime) existingRunContinuationForObjective(ctx context.Context, ownerID, sourceRunID, fingerprint string) (types.RunContinuationRecord, bool, error) {
	fingerprint = strings.TrimSpace(fingerprint)
	if fingerprint == "" {
		return types.RunContinuationRecord{}, false, nil
	}
	existing, err := rt.store.ListRunContinuationsBySource(ctx, ownerID, sourceRunID)
	if err != nil {
		return types.RunContinuationRecord{}, false, err
	}
	for _, rec := range existing {
		if rec.Status != types.RunContinuationSelected && rec.Status != types.RunContinuationStarted {
			continue
		}
		if detailStringValue(rec.Details, "objective_fingerprint") == fingerprint {
			return rec, true, nil
		}
	}
	return types.RunContinuationRecord{}, false, nil
}

func (rt *Runtime) maybeStartConfiguredContinuation(ctx context.Context, rec *types.RunRecord) {
	if rt == nil || rec == nil || rec.Metadata == nil {
		return
	}
	objective := metadataStringValue(rec.Metadata, runMetadataContObjective)
	if objective == "" {
		return
	}
	selected, err := rt.SelectRunContinuation(ctx, rec.RunID, rec.OwnerID, ContinuationProposal{
		Objective:        objective,
		Reason:           metadataStringValue(rec.Metadata, runMetadataContReason),
		AuthorityProfile: metadataStringValue(rec.Metadata, runMetadataContAuthority),
		LeaseSeconds:     metadataIntValue(rec.Metadata, runMetadataContLeaseSeconds),
		Details: map[string]any{
			"selection_source": "run_metadata",
		},
	})
	if err != nil {
		return
	}
	if metadataBoolValue(rec.Metadata, runMetadataContAutoStart) {
		_, _ = rt.StartRunContinuation(ctx, rec.OwnerID, selected.ContinuationID)
	}
}

func continuationFromPromotionCandidates(source *types.RunRecord, candidates []types.PromotionCandidateRecord) (ContinuationProposal, bool) {
	if source == nil {
		return ContinuationProposal{}, false
	}
	sourceRunID := strings.TrimSpace(source.RunID)
	trajectoryID := metadataStringValue(source.Metadata, runMetadataTrajectoryID)
	for _, status := range []types.PromotionCandidateStatus{
		types.PromotionCandidateQueued,
		types.PromotionCandidateIntegrated,
		types.PromotionCandidateVerificationFailed,
		types.PromotionCandidateVerified,
	} {
		for _, candidate := range candidates {
			if candidate.Status != status || !promotionCandidateImpactsSource(candidate, sourceRunID, trajectoryID) {
				continue
			}
			return continuationFromPromotionCandidate(candidate), true
		}
	}
	return ContinuationProposal{}, false
}

func promotionCandidateImpactsSource(candidate types.PromotionCandidateRecord, sourceRunID, trajectoryID string) bool {
	if strings.TrimSpace(candidate.SourceRunID) == sourceRunID {
		return true
	}
	if trajectoryID != "" && strings.TrimSpace(candidate.TraceID) == trajectoryID {
		return true
	}
	return false
}

func continuationFromPromotionCandidate(candidate types.PromotionCandidateRecord) ContinuationProposal {
	summary := strings.TrimSpace(candidate.Summary)
	if summary == "" {
		summary = "candidate-world patchset"
	}
	details := map[string]any{
		"selection_source":     "run_control_memory",
		"signal":               "promotion_candidate",
		"candidate_id":         candidate.CandidateID,
		"candidate_status":     string(candidate.Status),
		"source_loop_id":       candidate.SourceRunID,
		"trace_id":             candidate.TraceID,
		"verifier_target":      "promotion_candidate",
		"integration_branch":   candidate.IntegrationBranch,
		"destination_branch":   candidate.DestinationBranch,
		"candidate_summary":    summary,
		"canonical_mutation":   "forbidden_until_verified_and_approved",
		"promotion_queue_only": true,
	}
	switch candidate.Status {
	case types.PromotionCandidateVerified:
		return ContinuationProposal{
			Objective:        fmt.Sprintf("Prepare owner review for verified promotion candidate %s: %s", candidate.CandidateID, summary),
			Reason:           "run control memory found a verified candidate that still requires explicit owner approval before promotion",
			AuthorityProfile: AgentProfileCoSuper,
			LeaseSeconds:     60 * 60,
			Details:          details,
		}
	case types.PromotionCandidateVerificationFailed:
		details["error"] = candidate.Error
		return ContinuationProposal{
			Objective:        fmt.Sprintf("Recover failed promotion candidate %s without mutating canonical state: %s", candidate.CandidateID, summary),
			Reason:           "run control memory found a failed verifier contract that should produce diagnostics or a safer candidate",
			AuthorityProfile: AgentProfileVSuper,
			LeaseSeconds:     2 * 60 * 60,
			Details:          details,
		}
	default:
		return ContinuationProposal{
			Objective:        fmt.Sprintf("Verify queued promotion candidate %s before any canonical promotion: %s", candidate.CandidateID, summary),
			Reason:           "run control memory found a queued candidate that needs verifier contracts before promotion",
			AuthorityProfile: AgentProfileVSuper,
			LeaseSeconds:     2 * 60 * 60,
			Details:          details,
		}
	}
}

func boundedContinuationProfile(profile string) (string, error) {
	profile = canonicalAgentProfile(profile)
	if profile == "" {
		profile = AgentProfileVSuper
	}
	switch profile {
	case AgentProfileVSuper, AgentProfileCoSuper, AgentProfileResearcher:
		return profile, nil
	default:
		return "", fmt.Errorf("continuation authority profile %q is not bounded", profile)
	}
}

func (rt *Runtime) emitContinuationEvent(ctx context.Context, source *types.RunRecord, rec types.RunContinuationRecord, kind types.EventKind) {
	if rt == nil || source == nil {
		return
	}
	payloadMap := map[string]any{
		"continuation_id":       rec.ContinuationID,
		"status":                rec.Status,
		"objective":             rec.Objective,
		"objective_fingerprint": detailStringValue(rec.Details, "objective_fingerprint"),
		"authority_profile":     rec.AuthorityProfile,
		"next_loop_id":          rec.NextRunID,
		"lease_seconds":         rec.LeaseSeconds,
	}
	if len(rec.Details) > 0 {
		payloadMap["details"] = rec.Details
		for _, key := range []string{
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
			"objective_fingerprint",
		} {
			if value, ok := rec.Details[key]; ok && value != nil {
				payloadMap[key] = value
			}
		}
	}
	payload, _ := json.Marshal(payloadMap)
	rt.emitEvent(ctx, source, kind, events.CauseSupervisorRecovery, payload)
}

func cloneDetailsMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in)+2)
	for k, v := range in {
		out[k] = v
	}
	return out
}

func detailStringValue(details map[string]any, key string) string {
	if details == nil {
		return ""
	}
	if value, _ := details[key].(string); strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return ""
}
