package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"unicode"

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
	if overlayID := detailStringValue(rec.Details, runMetadataLLMPolicyOverlayID); overlayID != "" {
		metadata[runMetadataLLMPolicyOverlayID] = overlayID
	}
	child, err := rt.StartCoagentRun(ctx, rec.SourceRunID, rec.Objective, ownerID, metadata)
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
	terms := []string{}
	var b strings.Builder
	lastSpace := false
	flush := func() {
		token := strings.TrimSpace(b.String())
		if token == "" {
			return
		}
		terms = append(terms, token)
		b.Reset()
	}
	for _, r := range strings.ToLower(strings.TrimSpace(raw)) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastSpace = false
			continue
		}
		if !lastSpace {
			flush()
			lastSpace = true
		}
	}
	flush()
	return strings.Join(terms, " ")
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
			"adoption_id",
			"package_id",
			"adoption_status",
			"trace_id",
			"target_computer_id",
			"target_candidate_id",
			"candidate_source_ref",
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
