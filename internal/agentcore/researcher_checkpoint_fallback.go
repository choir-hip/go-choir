package agentcore

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

const terminalOutcomeReferenceUpdatePrefix = "terminal-coagent-outcome-"

type terminalOutcomeBinding struct {
	Update  types.CoagentSourcePacket
	Present bool
	Wake    bool
}

// bindTerminalRunOutcome binds one delegated child's final safe update_coagent
// packet to the authoritative terminal RunRecord. Root runs have no requester
// and return before any store access.
func (rt *Runtime) bindTerminalRunOutcome(ctx context.Context, rec *types.RunRecord, activate bool) error {
	if rt == nil || rt.store == nil || rec == nil ||
		strings.TrimSpace(rec.RunID) == "" ||
		strings.TrimSpace(rec.RequestedByRunID) == "" ||
		!terminalOutcomeCapableProfile(agentProfileForRun(rec)) {
		return nil
	}
	persisted, err := rt.store.GetRun(ctx, rec.RunID)
	if err != nil {
		return fmt.Errorf("reload terminal run %s: %w", rec.RunID, err)
	}
	binding, err := rt.ensurePersistedTerminalRunOutcome(ctx, &persisted)
	if err != nil {
		return err
	}
	if activate && binding.Wake {
		rt.wakeUpdatedCoagent(ctx, binding.Update)
	}
	return nil
}

func (rt *Runtime) ensurePersistedTerminalRunOutcome(ctx context.Context, persisted *types.RunRecord) (terminalOutcomeBinding, error) {
	if rt == nil || rt.store == nil || persisted == nil || !persisted.State.Terminal() || !terminalOutcomeCapableProfile(agentProfileForRun(persisted)) {
		return terminalOutcomeBinding{}, nil
	}
	// Only delegated child runs owe a terminal outcome to a requester. Root
	// runs have no addressed parent and must never synthesize mailbox traffic.
	if strings.TrimSpace(persisted.RequestedByRunID) == "" {
		return terminalOutcomeBinding{}, nil
	}
	targetAgentID, channelID, ok, err := rt.terminalOutcomeRequesterTarget(ctx, persisted)
	if err != nil {
		return terminalOutcomeBinding{}, err
	}
	if !ok {
		return terminalOutcomeBinding{}, nil
	}
	outcomeDigest := types.TerminalRunOutcomeSHA256(persisted.RunID, persisted.State, persisted.Result, persisted.Error)
	producerUpdates, err := rt.store.ListWorkerUpdatesBySourceRun(ctx, persisted.OwnerID, persisted.RunID)
	if err != nil {
		return terminalOutcomeBinding{}, fmt.Errorf("list producer updates for terminal run %s: %w", persisted.RunID, err)
	}

	var bound []types.CoagentSourcePacket
	var finalExplicit *types.CoagentSourcePacket
	for i := range producerUpdates {
		update := producerUpdates[i]
		if strings.TrimSpace(update.SourceOutcomeSHA256) != "" {
			bound = append(bound, update)
			continue
		}
		if !terminalOutcomeExplicitProducerIdentityMatches(update, persisted, targetAgentID, channelID) {
			continue
		}
		if finalExplicit == nil || terminalOutcomeUpdateLater(update, *finalExplicit) {
			candidate := update
			finalExplicit = &candidate
		}
	}
	if len(bound) > 1 {
		return terminalOutcomeBinding{}, fmt.Errorf("terminal run %s has %d bound outcomes, want exactly one", persisted.RunID, len(bound))
	}
	if len(bound) == 1 {
		existing := bound[0]
		if existing.SourceOutcomeSHA256 != outcomeDigest {
			return terminalOutcomeBinding{}, fmt.Errorf("terminal run %s outcome binding %s has a different digest", persisted.RunID, existing.UpdateID)
		}
		if !terminalOutcomeBoundIdentityMatches(existing, persisted, targetAgentID, channelID) {
			return terminalOutcomeBinding{}, fmt.Errorf("terminal run %s outcome binding %s has mismatched sender identity", persisted.RunID, existing.UpdateID)
		}
		if finalExplicit != nil && existing.UpdateID != finalExplicit.UpdateID {
			return terminalOutcomeBinding{}, fmt.Errorf("terminal run %s outcome binding %s was superseded by final update %s", persisted.RunID, existing.UpdateID, finalExplicit.UpdateID)
		}
		return terminalOutcomeBinding{Update: existing, Present: true}, nil
	}
	if finalExplicit != nil {
		boundUpdate, bindErr := rt.store.BindWorkerUpdateTerminalOutcome(ctx, persisted.OwnerID, finalExplicit.UpdateID, persisted.RunID, outcomeDigest)
		if bindErr != nil {
			return terminalOutcomeBinding{}, fmt.Errorf("bind final producer update %s: %w", finalExplicit.UpdateID, bindErr)
		}
		if !terminalOutcomeExplicitProducerIdentityMatches(boundUpdate, persisted, targetAgentID, channelID) {
			return terminalOutcomeBinding{}, fmt.Errorf("bound terminal update %s changed producer identity", finalExplicit.UpdateID)
		}
		return terminalOutcomeBinding{Update: boundUpdate, Present: true}, nil
	}

	synthetic := terminalOutcomeReferenceUpdate(persisted, targetAgentID, channelID, outcomeDigest)
	if existing, getErr := rt.store.GetWorkerUpdate(ctx, persisted.OwnerID, synthetic.UpdateID); getErr == nil {
		if err := validateTerminalOutcomeReferenceUpdate(existing, synthetic); err != nil {
			return terminalOutcomeBinding{}, err
		}
		return terminalOutcomeBinding{Update: existing, Present: true}, nil
	} else if !errors.Is(getErr, store.ErrNotFound) {
		return terminalOutcomeBinding{}, fmt.Errorf("get terminal outcome %s: %w", synthetic.UpdateID, getErr)
	}

	message := &types.ChannelMessage{
		ChannelID:    synthetic.ChannelID,
		From:         persisted.RunID,
		FromAgentID:  synthetic.AgentID,
		FromRunID:    persisted.RunID,
		ToAgentID:    synthetic.TargetAgentID,
		TrajectoryID: synthetic.TrajectoryID,
		Role:         synthetic.Role,
		Content:      synthetic.Content,
		Timestamp:    synthetic.CreatedAt,
	}
	stored, created, err := rt.store.DispatchWorkerUpdate(ctx, synthetic, message)
	if err != nil {
		return terminalOutcomeBinding{}, fmt.Errorf("dispatch terminal outcome: %w", err)
	}
	if !created {
		if err := validateTerminalOutcomeReferenceUpdate(stored, synthetic); err != nil {
			return terminalOutcomeBinding{}, err
		}
		return terminalOutcomeBinding{Update: stored, Present: true}, nil
	}
	message.Seq = stored.MessageSeq
	rt.emitChannelMessageEvent(ctx, *message, persisted.OwnerID)
	return terminalOutcomeBinding{Update: stored, Present: true, Wake: true}, nil
}

func terminalOutcomeCapableProfile(profile string) bool {
	return agentprofile.Canonical(profile) != ""
}

func (rt *Runtime) terminalOutcomeRequesterTarget(ctx context.Context, rec *types.RunRecord) (string, string, bool, error) {
	if strings.TrimSpace(rec.RequestedByRunID) == "" {
		return "", "", false, nil
	}
	targetAgentID := strings.TrimSpace(metadataStringValue(rec.Metadata, "requested_by_agent_id"))
	channelID := strings.TrimSpace(metadataStringValue(rec.Metadata, runMetadataChannelID))
	parent, parentErr := rt.store.GetRun(ctx, rec.RequestedByRunID)
	if parentErr != nil && !errors.Is(parentErr, store.ErrNotFound) {
		return "", "", false, fmt.Errorf("resolve terminal outcome requester run %s: %w", rec.RequestedByRunID, parentErr)
	}
	if parentErr == nil {
		if targetAgentID == "" {
			targetAgentID = agentIDForRun(&parent)
		}
		if strings.TrimSpace(parent.ChannelID) != "" {
			channelID = strings.TrimSpace(parent.ChannelID)
		}
	}
	if targetAgentID == "" {
		return "", "", false, nil
	}
	if target, err := rt.store.GetAgentByScope(ctx, rec.OwnerID, rec.SandboxID, targetAgentID); err == nil {
		if strings.TrimSpace(target.ChannelID) != "" {
			channelID = strings.TrimSpace(target.ChannelID)
		}
	} else if !errors.Is(err, store.ErrNotFound) {
		return "", "", false, fmt.Errorf("resolve terminal outcome requester agent %s: %w", targetAgentID, err)
	}
	if channelID == "" {
		channelID = strings.TrimSpace(rec.ChannelID)
	}
	if channelID == "" || targetAgentID == agentIDForRun(rec) {
		return "", "", false, nil
	}
	return targetAgentID, channelID, true, nil
}

func terminalOutcomeReferenceUpdate(rec *types.RunRecord, targetAgentID, channelID, outcomeDigest string) types.CoagentSourcePacket {
	profile := agentprofile.Canonical(agentProfileForRun(rec))
	kind := "evidence_update"
	summary := fmt.Sprintf("%s run completed with an authoritative terminal result.", profile)
	if rec.State != types.RunCompleted {
		kind = "blocker"
		summary = fmt.Sprintf("%s run reached terminal state %s.", profile, rec.State)
	}
	packet := newCoagentPacket(
		kind,
		summary,
		nil,
		nil,
		nil,
		nil,
		[]string{"The runtime-owned source_run_id references the authoritative RunRecord; source_outcome_sha256 is only its terminal outcome witness."},
	)
	createdAt := rec.UpdatedAt
	if rec.FinishedAt != nil {
		createdAt = *rec.FinishedAt
	}
	if createdAt.IsZero() {
		createdAt = time.Unix(0, 0).UTC()
	}
	update := types.CoagentSourcePacket{
		UpdateID:            terminalOutcomeReferenceUpdatePrefix + outcomeDigest[:32],
		OwnerID:             rec.OwnerID,
		AgentID:             agentIDForRun(rec),
		TargetAgentID:       targetAgentID,
		ChannelID:           channelID,
		TrajectoryID:        trajectoryIDForRun(rec),
		Role:                profile,
		SourceRunID:         rec.RunID,
		SourceOutcomeSHA256: outcomeDigest,
		Packet:              packet,
		CreatedAt:           createdAt,
	}
	update.Content = buildWorkerUpdateMessage(update)
	return update
}

func terminalOutcomeExplicitProducerIdentityMatches(update types.CoagentSourcePacket, rec *types.RunRecord, targetAgentID, channelID string) bool {
	return update.UpdateID == deriveWorkerUpdateID(update) &&
		update.OwnerID == rec.OwnerID &&
		update.AgentID == agentIDForRun(rec) &&
		update.TargetAgentID == targetAgentID &&
		update.ChannelID == channelID &&
		update.TrajectoryID == trajectoryIDForRun(rec) &&
		update.SourceRunID == rec.RunID &&
		agentprofile.Canonical(update.Role) == agentprofile.Canonical(agentProfileForRun(rec))
}

func terminalOutcomeUpdateLater(candidate, current types.CoagentSourcePacket) bool {
	if !candidate.CreatedAt.Equal(current.CreatedAt) {
		return candidate.CreatedAt.After(current.CreatedAt)
	}
	if candidate.MessageSeq != current.MessageSeq {
		return candidate.MessageSeq > current.MessageSeq
	}
	return candidate.UpdateID > current.UpdateID
}

func terminalOutcomeBoundIdentityMatches(update types.CoagentSourcePacket, rec *types.RunRecord, targetAgentID, channelID string) bool {
	return update.OwnerID == rec.OwnerID &&
		update.AgentID == agentIDForRun(rec) &&
		update.TargetAgentID == targetAgentID &&
		update.ChannelID == channelID &&
		update.TrajectoryID == trajectoryIDForRun(rec) &&
		update.SourceRunID == rec.RunID
}

func validateTerminalOutcomeReferenceUpdate(existing, want types.CoagentSourcePacket) error {
	if err := validateExistingWorkerUpdate(existing, want); err != nil {
		return err
	}
	if existing.TrajectoryID != want.TrajectoryID ||
		existing.SourceRunID != want.SourceRunID ||
		existing.SourceOutcomeSHA256 != want.SourceOutcomeSHA256 {
		return fmt.Errorf("terminal coagent outcome %s already exists with different runtime binding metadata", want.UpdateID)
	}
	return nil
}
