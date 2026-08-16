package agentcore

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/buildinfo"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
)

const runMetadataWorkerUpdatesInjected = "worker_updates_injected"

const (
	lifecycleLogicalActivationKeyMetadata = "lifecycle_logical_activation_key"
	lifecycleFailedAttemptKeyMetadata     = "lifecycle_failed_attempt_key"
	lifecycleActivationBuildMetadata      = "lifecycle_activation_build_commit"
	lifecycleActivationVersionsMetadata   = "lifecycle_activation_versions"
)

// ErrDurablyTerminalLifecycleControlActivation is a handled actor outcome. It
// is returned only after a deterministic lifecycle bind rejection has been
// durably persisted on the exact fingerprinted run, or when replay observes
// that same durable failure. Actor delivery may acknowledge this error; all
// other reconcile errors remain retryable.
var ErrDurablyTerminalLifecycleControlActivation = errors.New("durably terminal lifecycle control activation")

type lifecycleActivationVersion = types.LifecycleControlActivationVersion

type lifecycleActivationIdentity struct {
	OwnerID      string                       `json:"owner_id"`
	ComputerID   string                       `json:"computer_id"`
	TrajectoryID string                       `json:"trajectory_id"`
	AgentID      string                       `json:"agent_id"`
	Joins        []lifecycleActivationVersion `json:"joins"`
}

func digestLifecycleActivationIdentity(value any) (string, error) {
	body, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(body)
	return "sha256:" + hex.EncodeToString(digest[:]), nil
}

func lifecycleActivationKeys(ownerID, computerID, trajectoryID, agentID, buildCommit string, updates []types.CoagentSourcePacket, workByID map[string]types.WorkItemRecord) (string, string, []lifecycleActivationVersion, error) {
	ownerID, computerID = strings.TrimSpace(ownerID), strings.TrimSpace(computerID)
	trajectoryID, agentID, buildCommit = strings.TrimSpace(trajectoryID), strings.TrimSpace(agentID), strings.TrimSpace(buildCommit)
	if ownerID == "" || computerID == "" || trajectoryID == "" || agentID == "" || buildCommit == "" || len(updates) == 0 {
		return "", "", nil, store.ErrLifecycleInvalidTransition
	}
	versions := make([]lifecycleActivationVersion, 0, len(updates))
	seenUpdates := make(map[string]bool, len(updates))
	for _, update := range updates {
		updateID, workID := strings.TrimSpace(update.UpdateID), strings.TrimSpace(update.TargetWorkItemID)
		work, ok := workByID[workID]
		if updateID == "" || workID == "" || seenUpdates[updateID] || !ok || update.LifecycleVersion <= 0 || work.LifecycleVersion <= 0 {
			return "", "", nil, store.ErrLifecycleInvalidTransition
		}
		seenUpdates[updateID] = true
		versions = append(versions, lifecycleActivationVersion{UpdateID: updateID, TargetWorkItemID: workID, ControlLifecycleVersion: update.LifecycleVersion, WorkLifecycleVersion: work.LifecycleVersion})
	}
	identity := lifecycleActivationIdentity{OwnerID: ownerID, ComputerID: computerID, TrajectoryID: trajectoryID, AgentID: agentID, Joins: versions}
	logicalIdentity := struct {
		OwnerID      string `json:"owner_id"`
		ComputerID   string `json:"computer_id"`
		TrajectoryID string `json:"trajectory_id"`
		AgentID      string `json:"agent_id"`
		Joins        []struct {
			UpdateID         string `json:"update_id"`
			TargetWorkItemID string `json:"target_work_item_id"`
		} `json:"joins"`
	}{OwnerID: identity.OwnerID, ComputerID: identity.ComputerID, TrajectoryID: identity.TrajectoryID, AgentID: identity.AgentID}
	logicalIdentity.Joins = make([]struct {
		UpdateID         string `json:"update_id"`
		TargetWorkItemID string `json:"target_work_item_id"`
	}, 0, len(versions))
	for _, version := range versions {
		logicalIdentity.Joins = append(logicalIdentity.Joins, struct {
			UpdateID         string `json:"update_id"`
			TargetWorkItemID string `json:"target_work_item_id"`
		}{UpdateID: version.UpdateID, TargetWorkItemID: version.TargetWorkItemID})
	}
	logicalKey, err := digestLifecycleActivationIdentity(logicalIdentity)
	if err != nil {
		return "", "", nil, err
	}
	failedKey, err := digestLifecycleActivationIdentity(struct {
		BuildCommit string                      `json:"build_commit"`
		Identity    lifecycleActivationIdentity `json:"identity"`
	}{BuildCommit: buildCommit, Identity: identity})
	if err != nil {
		return "", "", nil, err
	}
	return logicalKey, failedKey, versions, nil
}

// reconcilePersistentSuperActor is the durable controller boundary for the
// user's privileged execution actor. update_coagent can append addressed work
// for the persistent super, but only this runtime controller starts or reuses
// the super execution loop that drains those durable updates.
func (rt *Runtime) reconcilePersistentSuperActor(ctx context.Context, ownerID, agentID string) (*types.RunRecord, error) {
	ownerID = strings.TrimSpace(ownerID)
	agentID = strings.TrimSpace(agentID)
	if ownerID == "" {
		return nil, fmt.Errorf("owner_id is required")
	}
	if agentID == "" {
		agentID = persistentSuperAgentID(ownerID)
	}
	var blockedActive *types.RunRecord
	if resident, found, err := rt.activeRunByAgent(ctx, ownerID, agentID); err != nil {
		return nil, fmt.Errorf("check resident super run: %w", err)
	} else if found {
		return &resident, nil
	}
	if resumed, ok, err := rt.reactivateRestartedPersistentSuperControlRun(ctx, ownerID, agentID); err != nil {
		return nil, err
	} else if ok {
		return resumed, nil
	}
	if active, err := rt.latestActiveRunByAgent(ctx, ownerID, agentID); err == nil {
		if active.State == types.RunBlocked {
			blockedActive = &active
		}
	} else if !errors.Is(err, store.ErrNotFound) {
		return nil, fmt.Errorf("check blocked super run: %w", err)
	}

	computerID := strings.TrimSpace(rt.TextureComputerID())
	updates, err := rt.listPendingPersistentSuperLifecycleControls(ctx, ownerID, computerID, agentID, 100)
	updates = selectLifecycleControlActivation(updates, "", nil)
	lifecycleControls := len(updates) > 0
	if err != nil {
		return nil, err
	}
	if !lifecycleControls {
		updates, err = rt.listAndSettlePersistentSuperBacklog(ctx, ownerID, agentID)
		if err != nil {
			return nil, err
		}
		updates = filterPersistentSuperExecutionUpdates(updates)
	}
	if len(updates) == 0 {
		if blockedActive != nil {
			return blockedActive, nil
		}
		return nil, nil
	}

	first := updates[0]
	requestSource := "update_coagent"
	if lifecycleControls {
		requestSource = "lifecycle_texture_control"
	}
	metadata := map[string]any{
		runMetadataAgentProfile: agentprofile.Super,
		runMetadataAgentRole:    agentprofile.Super,
		runMetadataAgentID:      agentID,
		"request_source":        requestSource,
		"requested_by_agent_id": first.AgentID,
		"requested_by_profile":  strings.TrimSpace(first.Role),
	}
	if first.ChannelID != "" {
		metadata[runMetadataChannelID] = first.ChannelID
	}
	if first.TrajectoryID != "" {
		metadata["assignment_trajectory_id"] = first.TrajectoryID
	}
	targetWorkItemID := firstNonEmpty(first.TargetWorkItemID, first.WorkItemID)
	if targetWorkItemID != "" {
		metadata["lifecycle_work_item_id"] = targetWorkItemID
		workIDs, seenWork := make([]string, 0, len(updates)), map[string]bool{}
		for _, update := range updates {
			if id := firstNonEmpty(update.TargetWorkItemID, update.WorkItemID); id != "" && !seenWork[id] {
				seenWork[id] = true
				workIDs = append(workIDs, id)
			}
		}
		metadata["work_item_ids"] = workIDs
	}
	updateIDs := make([]string, 0, len(updates))
	for _, update := range updates {
		if id := strings.TrimSpace(update.UpdateID); id != "" {
			updateIDs = append(updateIDs, id)
		}
	}
	if len(updateIDs) > 0 && !lifecycleControls {
		metadata["worker_update_ids"] = updateIDs
	}
	if first.AgentID != "" {
		if requester, err := rt.latestActiveRunByAgent(ctx, ownerID, first.AgentID); err == nil {
			metadata["requested_by_run_id"] = requester.RunID
			if metadataStringValue(requester.Metadata, "agent_profile") != "" && metadata["requested_by_profile"] == "" {
				metadata["requested_by_profile"] = metadataStringValue(requester.Metadata, "agent_profile")
			}
			if desktopID := metadataStringValue(requester.Metadata, runMetadataDesktopID); desktopID != "" {
				metadata[runMetadataDesktopID] = desktopID
			}
		}
	}

	rec, err := rt.createRunWithMetadata(ctx, "Process pending coagent update packets for privileged execution.", ownerID, metadata)
	if err != nil {
		return nil, err
	}
	if lifecycleControls {
		// Persistent Super is deliberately non-lifecycle. createRunWithMetadata
		// normally stamps a generic trajectory projection; remove only that
		// projection while retaining the exact assignment_trajectory_id used by
		// the lifecycle control-delivery authority.
		rec.TrajectoryID = ""
		delete(rec.Metadata, runMetadataTrajectoryID)
		if err := rt.store.UpdateRun(ctx, *rec); err != nil {
			rt.failUnactivatedLifecycleControlRun(ctx, rec, err)
			return nil, fmt.Errorf("preserve non-lifecycle persistent-Super run: %w", err)
		}
		if _, err := rt.bindLifecycleControlsToRun(ctx, rec, updates); err != nil {
			rt.failUnactivatedLifecycleControlRun(ctx, rec, err)
			return nil, err
		}
	}
	rt.activate(rec)
	return rec, nil
}

func (rt *Runtime) reactivateRestartedPersistentSuperControlRun(ctx context.Context, ownerID, agentID string) (*types.RunRecord, bool, error) {
	runs, err := rt.store.ListAllRunsByState(ctx, types.RunPassivated)
	if err != nil {
		return nil, false, fmt.Errorf("list restarted persistent-Super runs: %w", err)
	}
	var candidate *types.RunRecord
	for i := range runs {
		run := &runs[i]
		passivatedReason := metadataStringValue(run.Metadata, "passivated_reason")
		if run.OwnerID != ownerID || run.AgentID != agentID || !isPersistentSuperAgentRun(run) ||
			metadataStringValue(run.Metadata, "request_source") != "lifecycle_texture_control" ||
			(passivatedReason != "runtime_restarted" && passivatedReason != runtimeInjectionAppendFailurePassivationReason) {
			continue
		}
		controls, readErr := rt.listPendingLifecyclePacketsDeliveredToRun(ctx, run)
		if readErr != nil {
			return nil, false, fmt.Errorf("validate restarted persistent-Super control run %s: %w", run.RunID, readErr)
		}
		if len(controls) == 0 {
			continue
		}
		if candidate == nil || run.UpdatedAt.After(candidate.UpdatedAt) || (run.UpdatedAt.Equal(candidate.UpdatedAt) && run.RunID < candidate.RunID) {
			copy := *run
			candidate = &copy
		}
	}
	if candidate == nil {
		return nil, false, nil
	}
	candidate.Metadata = cloneMetadata(candidate.Metadata)
	candidate.Metadata["actor_reactivate_existing_memory"] = true
	candidate.Metadata["actor_reactivated_from_passivated"] = true
	candidate.Metadata["passivated_reason"] = ""
	candidate.State = types.RunPending
	candidate.Error = ""
	candidate.Result = ""
	candidate.FinishedAt = nil
	candidate.UpdatedAt = time.Now().UTC()
	if err := rt.store.UpdateRun(ctx, *candidate); err != nil {
		return nil, false, fmt.Errorf("reactivate restarted persistent-Super control run %s: %w", candidate.RunID, err)
	}
	rt.activate(candidate)
	return candidate, true, nil
}

func (rt *Runtime) markPersistentSuperRunUpdatesDelivered(ctx context.Context, rec *types.RunRecord) error {
	if rec == nil || strings.TrimSpace(rec.OwnerID) == "" || strings.TrimSpace(rec.RunID) == "" {
		return nil
	}
	updateIDs := coagentUpdateIDsForRun(rec)
	if len(updateIDs) == 0 {
		return nil
	}
	if err := rt.store.MarkWorkerUpdatesDelivered(ctx, rec.OwnerID, rec.AgentID, updateIDs, rec.RunID); err != nil {
		return fmt.Errorf("mark persistent super updates delivered: %w", err)
	}
	return nil
}

func coagentUpdateIDsForRun(rec *types.RunRecord) []string {
	if rec == nil {
		return nil
	}
	if !runSupportsCoagentUpdateInjection(rec) {
		return nil
	}
	if metadataStringValue(rec.Metadata, "request_source") != "update_coagent" &&
		!metadataBoolValue(rec.Metadata, runMetadataWorkerUpdatesInjected) {
		return nil
	}
	return metadataStringSlice(rec.Metadata["worker_update_ids"])
}

func appendCoagentUpdateIDsForRun(rec *types.RunRecord, updateIDs []string) {
	if rec == nil || len(updateIDs) == 0 {
		return
	}
	if rec.Metadata == nil {
		rec.Metadata = map[string]any{}
	}
	seen := map[string]bool{}
	merged := make([]string, 0, len(updateIDs))
	for _, id := range metadataStringSlice(rec.Metadata["worker_update_ids"]) {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		merged = append(merged, id)
	}
	for _, id := range updateIDs {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		merged = append(merged, id)
	}
	rec.Metadata["worker_update_ids"] = merged
	rec.Metadata[runMetadataWorkerUpdatesInjected] = true
}

func (rt *Runtime) updateRunAndMarkSuccessfulCoagentActivationDelivered(ctx context.Context, rec *types.RunRecord) error {
	if rec == nil {
		return nil
	}
	updateIDs := coagentUpdateIDsForRun(rec)
	if runHasProfile(rec, agentprofile.Texture) || metadataStringValue(rec.Metadata, "request_source") == "lifecycle_texture_control" {
		if err := rt.store.UpdateRun(ctx, *rec); err != nil {
			return err
		}
		if metadataStringValue(rec.Metadata, "request_source") == "lifecycle_texture_control" {
			return nil
		}
		return rt.completeSuccessfulRunWorkItems(ctx, rec)
	}
	if len(updateIDs) == 0 || rec.State != types.RunCompleted {
		if err := rt.store.UpdateRun(ctx, *rec); err != nil {
			return err
		}
		return rt.completeSuccessfulRunWorkItems(ctx, rec)
	}
	if err := rt.store.UpdateRunAndMarkWorkerUpdatesDelivered(ctx, *rec, rec.OwnerID, updateIDs); err != nil {
		return err
	}
	return rt.completeSuccessfulRunWorkItems(ctx, rec)
}

func (rt *Runtime) completeSuccessfulRunWorkItems(ctx context.Context, rec *types.RunRecord) error {
	if rt == nil || rt.store == nil || rec == nil || rec.State != types.RunCompleted {
		return nil
	}
	ownerID := strings.TrimSpace(rec.OwnerID)
	if ownerID == "" {
		return nil
	}
	if strings.TrimSpace(rec.ComputerID) != "" && strings.TrimSpace(rec.TrajectoryID) != "" {
		if _, err := rt.store.GetLifecycleTrajectory(ctx, ownerID, rec.ComputerID, rec.TrajectoryID); err == nil {
			return nil
		} else if !errors.Is(err, store.ErrNotFound) {
			return err
		}
	}
	for _, workItemID := range metadataStringSlice(rec.Metadata["work_item_ids"]) {
		workItemID = strings.TrimSpace(workItemID)
		if workItemID == "" {
			continue
		}
		if _, err := rt.store.UpdateWorkItemStatus(ctx, ownerID, workItemID, types.WorkItemCompleted); err != nil {
			return fmt.Errorf("complete run work item %s: %w", workItemID, err)
		}
	}
	return nil
}

func (rt *Runtime) maybeContinuePersistentSuperInbox(ctx context.Context, rec *types.RunRecord) {
	if !isPersistentSuperAgentRun(rec) || (rec.State != types.RunPassivated && !rec.State.Terminal()) {
		return
	}
	if isPersistentSuperInboxRun(rec) && rec.State == types.RunCompleted {
		if err := rt.markPersistentSuperRunUpdatesDelivered(ctx, rec); err != nil {
			log.Printf("runtime: mark persistent super updates delivered after %s: %v", rec.RunID, err)
			return
		}
	}
	// A persistent Super run owns exactly one lifecycle trajectory. Once that
	// realization passivates or terminates, release the singleton actor slot and
	// deterministically reconcile the oldest still-pending control trajectory.
	if _, err := rt.reconcilePersistentSuperActor(ctx, rec.OwnerID, rec.AgentID); err != nil {
		log.Printf("runtime: continue persistent super actor after %s: %v", rec.RunID, err)
	}
}

func isPersistentSuperInboxRun(rec *types.RunRecord) bool {
	if rec == nil {
		return false
	}
	if !isPersistentSuperAgentRun(rec) {
		return false
	}
	if metadataStringValue(rec.Metadata, "request_source") != "update_coagent" {
		return false
	}
	return true
}

func isPersistentSuperAgentRun(rec *types.RunRecord) bool {
	if rec == nil {
		return false
	}
	if agentProfileForRun(rec) != agentprofile.Super {
		return false
	}
	if strings.TrimSpace(rec.OwnerID) == "" || strings.TrimSpace(rec.AgentID) == "" {
		return false
	}
	return rec.AgentID == persistentSuperAgentID(rec.OwnerID)
}

func filterPersistentSuperExecutionUpdates(updates []types.CoagentSourcePacket) []types.CoagentSourcePacket {
	if len(updates) == 0 {
		return nil
	}
	out := make([]types.CoagentSourcePacket, 0, len(updates))
	for _, update := range updates {
		if persistentSuperExecutableUpdate(update) {
			out = append(out, update)
		}
	}
	return out
}

func (rt *Runtime) listAndSettlePersistentSuperBacklog(ctx context.Context, ownerID, agentID string) ([]types.CoagentSourcePacket, error) {
	const limit = 100
	for i := 0; i < 10; i++ {
		updates, err := rt.store.ListCoagentMailboxBacklog(ctx, ownerID, agentID, limit)
		if err != nil {
			return nil, fmt.Errorf("list super pending updates: %w", err)
		}
		settled, err := rt.settlePersistentSuperNonExecutionUpdates(ctx, ownerID, agentID, updates)
		if err != nil {
			return nil, fmt.Errorf("settle non-execution super updates: %w", err)
		}
		if !settled {
			return updates, nil
		}
	}
	return nil, fmt.Errorf("settle non-execution super updates: mailbox did not converge")
}

func (rt *Runtime) settlePersistentSuperNonExecutionUpdates(ctx context.Context, ownerID, agentID string, updates []types.CoagentSourcePacket) (bool, error) {
	var nonExecIDs []string
	for _, u := range updates {
		if u.DeliveredAt != nil || strings.TrimSpace(u.DeliveredToRunID) != "" {
			continue
		}
		if !persistentSuperExecutableUpdate(u) && !persistentSuperAdmissibleReport(u) {
			if id := strings.TrimSpace(u.UpdateID); id != "" {
				nonExecIDs = append(nonExecIDs, id)
			}
		}
	}
	if len(nonExecIDs) == 0 {
		return false, nil
	}
	if err := rt.store.MarkWorkerUpdatesDelivered(ctx, ownerID, agentID, nonExecIDs, "settled_non_executable"); err != nil {
		return false, fmt.Errorf("mark non-execution updates settled: %w", err)
	}
	return true, nil
}

func lifecycleControlTrajectoryForRun(rec *types.RunRecord) string {
	if rec == nil {
		return ""
	}
	return strings.TrimSpace(firstNonEmpty(rec.TrajectoryID, metadataStringValue(rec.Metadata, "assignment_trajectory_id"), metadataStringValue(rec.Metadata, runMetadataTrajectoryID)))
}

func (rt *Runtime) listAllLifecyclePacketsDeliveredToRun(ctx context.Context, rec *types.RunRecord) ([]types.CoagentSourcePacket, error) {
	if rt == nil || rt.store == nil || rec == nil {
		return nil, nil
	}
	const pageSize = 100
	var out []types.CoagentSourcePacket
	var after int64
	for {
		page, err := rt.store.ListLifecycleControlsDeliveredToRunPage(ctx, rec.OwnerID, rec.ComputerID, lifecycleControlTrajectoryForRun(rec), rec.AgentID, rec.RunID, after, pageSize)
		if err != nil {
			return nil, err
		}
		out = append(out, page.Packets...)
		if !page.HasMore {
			return out, nil
		}
		if page.NextCursor <= after {
			return nil, fmt.Errorf("exact-run lifecycle delivery cursor did not advance")
		}
		after = page.NextCursor
	}
}

func (rt *Runtime) listPendingLifecyclePacketsDeliveredToRun(ctx context.Context, rec *types.RunRecord) ([]types.CoagentSourcePacket, error) {
	all, err := rt.listAllLifecyclePacketsDeliveredToRun(ctx, rec)
	if err != nil {
		return nil, err
	}
	pending := make([]types.CoagentSourcePacket, 0, len(all))
	for _, packet := range all {
		if packet.Disposition == types.UpdatePending {
			pending = append(pending, packet)
		}
	}
	return pending, nil
}

func lifecycleControlWorkIDsForRun(rec *types.RunRecord) map[string]bool {
	out := map[string]bool{}
	if rec == nil {
		return out
	}
	if id := strings.TrimSpace(metadataStringValue(rec.Metadata, "lifecycle_work_item_id")); id != "" {
		out[id] = true
	}
	for _, id := range metadataStringSlice(rec.Metadata["work_item_ids"]) {
		if id = strings.TrimSpace(id); id != "" {
			out[id] = true
		}
	}
	return out
}

func selectLifecycleControlActivation(updates []types.CoagentSourcePacket, trajectoryID string, workIDs map[string]bool) []types.CoagentSourcePacket {
	if len(updates) == 0 {
		return nil
	}
	trajectoryID = strings.TrimSpace(trajectoryID)
	if trajectoryID == "" {
		for _, update := range updates {
			if update.Direction == types.LifecyclePacketDirectionControl {
				trajectoryID = strings.TrimSpace(update.TrajectoryID)
				break
			}
		}
	}
	if trajectoryID == "" {
		return nil
	}
	out := make([]types.CoagentSourcePacket, 0, len(updates))
	for _, update := range updates {
		if update.Direction != types.LifecyclePacketDirectionControl {
			continue
		}
		if strings.TrimSpace(update.TrajectoryID) != trajectoryID {
			continue
		}
		if len(workIDs) > 0 && !workIDs[strings.TrimSpace(update.TargetWorkItemID)] {
			continue
		}
		out = append(out, update)
	}
	return out
}

func (rt *Runtime) lifecycleControlBindRequest(ctx context.Context, rec *types.RunRecord, updates []types.CoagentSourcePacket) (types.BindLifecycleControlDeliveryRequest, error) {
	if rt == nil || rt.store == nil || rec == nil || len(updates) == 0 {
		return types.BindLifecycleControlDeliveryRequest{}, store.ErrLifecycleInvalidTransition
	}
	updates = selectLifecycleControlActivation(updates, lifecycleControlTrajectoryForRun(rec), lifecycleControlWorkIDsForRun(rec))
	if len(updates) == 0 {
		return types.BindLifecycleControlDeliveryRequest{}, fmt.Errorf("bind lifecycle controls: no exact run/trajectory/work controls")
	}
	controlTrajectoryID := lifecycleControlTrajectoryForRun(rec)
	snapshot, err := rt.store.GetLifecycleSnapshot(ctx, rec.OwnerID, rec.ComputerID, controlTrajectoryID)
	if err != nil {
		return types.BindLifecycleControlDeliveryRequest{}, fmt.Errorf("bind lifecycle controls snapshot: %w", err)
	}
	items := make([]types.BindLifecycleControlDeliveryItem, 0, len(updates))
	ids := make([]string, 0, len(updates))
	fingerprinted := metadataStringValue(rec.Metadata, lifecycleLogicalActivationKeyMetadata) != ""
	fingerprintedUnbound := fingerprinted
	if fingerprinted {
		canonicallyBound, boundErr := rt.lifecycleRunHasCanonicalControlDelivery(ctx, rec)
		if boundErr != nil {
			return types.BindLifecycleControlDeliveryRequest{}, fmt.Errorf("classify lifecycle activation delivery state: %w", boundErr)
		}
		fingerprintedUnbound = !canonicallyBound
	}
	versionByUpdate := map[string]types.LifecycleControlActivationVersion{}
	var activationVersions []types.LifecycleControlActivationVersion
	if fingerprintedUnbound {
		activationVersions, err = lifecycleActivationVersionsForRun(rec)
		if err != nil {
			return types.BindLifecycleControlDeliveryRequest{}, fmt.Errorf("read lifecycle activation versions: %w", err)
		}
		for _, version := range activationVersions {
			versionByUpdate[version.UpdateID] = version
		}
	}
	for _, update := range updates {
		work, workErr := rt.store.GetLifecycleWorkItem(ctx, rec.OwnerID, rec.ComputerID, update.TargetWorkItemID)
		if workErr != nil {
			return types.BindLifecycleControlDeliveryRequest{}, fmt.Errorf("bind lifecycle control work %s: %w", update.TargetWorkItemID, workErr)
		}
		controlVersion, workVersion := update.LifecycleVersion, work.LifecycleVersion
		if fingerprintedUnbound {
			version, ok := versionByUpdate[update.UpdateID]
			if !ok || version.TargetWorkItemID != update.TargetWorkItemID {
				return types.BindLifecycleControlDeliveryRequest{}, fmt.Errorf("lifecycle control %s has no exact activation version/work binding: %w", update.UpdateID, store.ErrLifecycleInvalidTransition)
			}
			controlVersion, workVersion = version.ControlLifecycleVersion, version.WorkLifecycleVersion
		}
		items = append(items, types.BindLifecycleControlDeliveryItem{UpdateID: update.UpdateID, ProducerAgentID: update.AgentID, ProducerUpdateID: update.ProducerUpdateID, TargetWorkItemID: update.TargetWorkItemID, ExpectedControlLifecycleVersion: controlVersion, ExpectedWorkLifecycleVersion: workVersion})
		ids = append(ids, update.UpdateID)
	}
	commandID := "bind-control-delivery:" + rec.RunID + ":" + strings.Join(ids, ",")
	req := types.BindLifecycleControlDeliveryRequest{OwnerID: rec.OwnerID, ComputerID: rec.ComputerID, CommandID: commandID, TrajectoryID: controlTrajectoryID, TargetAgentID: rec.AgentID, TargetRunID: rec.RunID, ExpectedLifecycleVersion: snapshot.Trajectory.LifecycleVersion, Controls: items}
	if fingerprintedUnbound {
		workItemIDs := make([]string, 0, len(activationVersions))
		seenWork := map[string]bool{}
		for _, version := range activationVersions {
			if !seenWork[version.TargetWorkItemID] {
				seenWork[version.TargetWorkItemID] = true
				workItemIDs = append(workItemIDs, version.TargetWorkItemID)
			}
		}
		req.ActivationRefresh = &types.LifecycleControlActivationRefresh{Prompt: rec.Prompt, LogicalActivationKey: metadataStringValue(rec.Metadata, lifecycleLogicalActivationKeyMetadata), FailedAttemptKey: metadataStringValue(rec.Metadata, lifecycleFailedAttemptKeyMetadata), BuildCommit: metadataStringValue(rec.Metadata, lifecycleActivationBuildMetadata), Versions: activationVersions, WorkItemIDs: workItemIDs}
	}
	req.CommandDigest, err = store.ComputeBindLifecycleControlDeliveryDigest(req)
	if err != nil {
		return types.BindLifecycleControlDeliveryRequest{}, err
	}
	return req, nil
}

func (rt *Runtime) bindLifecycleControlsToRun(ctx context.Context, rec *types.RunRecord, updates []types.CoagentSourcePacket) (types.LifecycleResult, error) {
	if rt == nil || rt.store == nil || rec == nil || len(updates) == 0 {
		return types.LifecycleResult{}, nil
	}
	req, err := rt.lifecycleControlBindRequest(ctx, rec, updates)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	result, err := rt.store.BindLifecycleControlDelivery(ctx, req)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	var rebound types.RunRecord
	if isPersistentSuperAgentRun(rec) {
		rebound, err = rt.store.GetRunByOwner(ctx, rec.OwnerID, rec.RunID)
	} else {
		rebound, err = rt.store.GetLifecycleRun(ctx, rec.OwnerID, rec.ComputerID, rec.RunID)
	}
	if err != nil {
		// BindLifecycleControlDelivery already committed. Preserve its nonempty
		// receipt so callers cannot mistake a post-commit reload outage for a
		// pre-commit rejection and terminalize a stale run copy.
		return result, fmt.Errorf("reload bound lifecycle control run: %w", err)
	}
	*rec = rebound
	return result, nil
}

func (rt *Runtime) failUnactivatedLifecycleControlRun(ctx context.Context, rec *types.RunRecord, bindErr error) {
	if rt == nil || rt.store == nil || rec == nil {
		return
	}
	now := time.Now().UTC()
	rec.State, rec.UpdatedAt, rec.FinishedAt = types.RunFailed, now, &now
	if rec.Metadata == nil {
		rec.Metadata = map[string]any{}
	}
	rec.Metadata["lifecycle_control_bind_failed"] = true
	rec.Metadata["lifecycle_control_bind_failure"] = strings.TrimSpace(bindErr.Error())
	if err := rt.store.UpdateRun(context.WithoutCancel(ctx), *rec); err != nil {
		log.Printf("runtime: terminalize unactivated lifecycle control run %s: %v", rec.RunID, err)
	}
}

func (rt *Runtime) terminalizeFingerprintedLifecycleControlRun(ctx context.Context, rec *types.RunRecord, updates []types.CoagentSourcePacket, bindErr error) error {
	if rt == nil || rt.store == nil || rec == nil {
		return bindErr
	}
	// CAS, context, command-conflict, and store failures are not terminal
	// evidence. They must retry the same active logical run.
	if !errors.Is(bindErr, store.ErrLifecycleInvalidTransition) {
		return bindErr
	}
	logicalKey := metadataStringValue(rec.Metadata, lifecycleLogicalActivationKeyMetadata)
	failedKey := metadataStringValue(rec.Metadata, lifecycleFailedAttemptKeyMetadata)
	if logicalKey == "" || failedKey == "" {
		return fmt.Errorf("refuse unfingerprinted lifecycle bind failure: %w", bindErr)
	}
	bindReq, err := rt.lifecycleControlBindRequest(context.WithoutCancel(ctx), rec, updates)
	if err != nil {
		return fmt.Errorf("reconstruct deterministic lifecycle bind attempt for run %s: %w", rec.RunID, err)
	}
	failure := types.FailLifecycleControlActivationRequest{
		OwnerID: rec.OwnerID, ComputerID: rec.ComputerID,
		CommandID:    store.LifecycleControlActivationFailureCommandID(failedKey),
		TrajectoryID: bindReq.TrajectoryID, AgentID: rec.AgentID, RunID: rec.RunID,
		ExpectedLifecycleVersion: bindReq.ExpectedLifecycleVersion,
		LogicalActivationKey:     logicalKey, FailedAttemptKey: failedKey,
		BindCommandID: bindReq.CommandID, BindCommandDigest: bindReq.CommandDigest,
		Controls: bindReq.Controls, ActivationRefresh: bindReq.ActivationRefresh,
		Failure: store.LifecycleControlActivationMissingRunWorkBindingFailure,
	}
	failure.CommandDigest, err = store.ComputeFailLifecycleControlActivationDigest(failure)
	if err != nil {
		return fmt.Errorf("digest lifecycle control activation failure for run %s: %w", rec.RunID, err)
	}
	result, err := rt.store.FailLifecycleControlActivation(context.WithoutCancel(ctx), failure)
	if err != nil {
		return fmt.Errorf("persist deterministic lifecycle bind failure for run %s: %w", rec.RunID, err)
	}
	if result.Receipt.Kind != types.LifecycleFailControlActivation || len(result.Events) != 1 || result.Events[0].Kind != types.LifecycleControlActivationFailed {
		return fmt.Errorf("persist deterministic lifecycle bind failure for run %s returned invalid typed receipt", rec.RunID)
	}
	if failed, reloadErr := rt.store.GetLifecycleRun(context.WithoutCancel(ctx), rec.OwnerID, rec.ComputerID, rec.RunID); reloadErr == nil {
		*rec = failed
	}
	return fmt.Errorf("%w: run=%s failed_attempt=%s", ErrDurablyTerminalLifecycleControlActivation, rec.RunID, failedKey)
}

func (rt *Runtime) listPendingPersistentSuperLifecycleControls(ctx context.Context, ownerID, computerID, agentID string, limit int) ([]types.CoagentSourcePacket, error) {
	ownerID, computerID, agentID = strings.TrimSpace(ownerID), strings.TrimSpace(computerID), strings.TrimSpace(agentID)
	if ownerID == "" || computerID == "" || agentID != persistentSuperAgentID(ownerID) {
		return nil, fmt.Errorf("list persistent-Super lifecycle controls: exact owner, computer, and persistent agent are required")
	}
	agent, err := rt.store.GetAgentByScope(ctx, ownerID, computerID, agentID)
	if err != nil {
		return nil, fmt.Errorf("load exact persistent Super: %w", err)
	}
	if agentprofile.Canonical(agent.Profile) != agentprofile.Super || agentprofile.Canonical(agent.Role) != agentprofile.Super || agent.LifecycleVersion != 0 || agent.OwnerID != ownerID || agent.ComputerID != computerID {
		return nil, fmt.Errorf("persistent Super lifecycle control target has invalid authority")
	}
	updates, err := rt.store.ListAllPendingLifecycleUpdates(ctx, ownerID, computerID, agentID)
	if err != nil {
		return nil, fmt.Errorf("list persistent-Super lifecycle controls: %w", err)
	}
	return rt.validateTargetBoundLifecycleControls(ctx, ownerID, computerID, agentID, updates, true)
}

func (rt *Runtime) validateTargetBoundLifecycleControls(ctx context.Context, ownerID, computerID, agentID string, updates []types.CoagentSourcePacket, executionOnly bool) ([]types.CoagentSourcePacket, error) {
	out := make([]types.CoagentSourcePacket, 0, len(updates))
	for _, update := range updates {
		if update.Direction != types.LifecyclePacketDirectionControl || update.TargetAgentID != agentID || update.OwnerID != ownerID || update.ComputerID != computerID || update.Disposition != types.UpdatePending || strings.TrimSpace(update.TargetWorkItemID) == "" || strings.TrimSpace(update.TrajectoryID) == "" {
			return nil, fmt.Errorf("pending lifecycle control %q has ambiguous target binding", update.UpdateID)
		}
		if update.LifecycleVersion <= 0 {
			return nil, fmt.Errorf("pending lifecycle control %q has no canonical lifecycle version", update.UpdateID)
		}
		work, err := rt.store.GetLifecycleWorkItem(ctx, ownerID, computerID, update.TargetWorkItemID)
		if err != nil {
			return nil, fmt.Errorf("load exact lifecycle control work %q: %w", update.UpdateID, err)
		}
		if work.LifecycleVersion <= 0 || work.WorkItemID != update.TargetWorkItemID || work.Status != types.WorkItemOpen || work.AssignedAgentID != agentID || work.TrajectoryID != update.TrajectoryID || work.OwnerID != ownerID || work.ComputerID != computerID {
			return nil, fmt.Errorf("pending lifecycle control %q is not joined to exact open target work", update.UpdateID)
		}
		if executionOnly && !persistentSuperExecutableUpdate(update) {
			return nil, fmt.Errorf("persistent-Super lifecycle control %q is not an execution request", update.UpdateID)
		}
		out = append(out, update)
	}
	return out, nil
}

func persistentSuperSenderAuthorized(update types.CoagentSourcePacket) bool {
	return agentprofile.Canonical(update.Role) == agentprofile.Texture &&
		update.Direction == types.LifecyclePacketDirectionControl
}

func persistentSuperAdmissibleReport(update types.CoagentSourcePacket) bool {
	if agentprofile.Canonical(update.Role) != agentprofile.CoSuper ||
		update.Direction != types.LifecyclePacketDirectionProducerReport {
		return false
	}
	packet := normalizeCoagentSourcePacketPayload(update.Packet)
	switch packet.Kind {
	case "evidence_update", "execution_result", "blocker", "question", "proposal", "decision_request":
		return validateCoagentSourcePacketPayload(packet) == nil
	default:
		return false
	}
}

func persistentSuperExecutablePacket(update types.CoagentSourcePacket) bool {
	if !persistentSuperSenderAuthorized(update) {
		return false
	}
	packet := normalizeCoagentSourcePacketPayload(update.Packet)
	if packet.Kind != "execution_request" {
		return false
	}
	return validateCoagentSourcePacketPayload(packet) == nil
}

func persistentSuperExecutableUpdate(update types.CoagentSourcePacket) bool {
	if update.DeliveredAt != nil || strings.TrimSpace(update.DeliveredToRunID) != "" {
		return false
	}
	return persistentSuperExecutablePacket(update)
}

func persistentSuperMailboxInjectable(rec *types.RunRecord, update types.CoagentSourcePacket) bool {
	if rec == nil {
		return false
	}
	if update.DeliveredAt != nil {
		delivered := strings.TrimSpace(update.DeliveredToRunID)
		if delivered != "" && delivered != strings.TrimSpace(rec.RunID) {
			return false
		}
	}
	if metadataStringValue(rec.Metadata, "request_source") == "lifecycle_texture_control" {
		return update.DeliveredAt != nil && strings.TrimSpace(update.DeliveredToRunID) == strings.TrimSpace(rec.RunID)
	}
	return update.DeliveredAt == nil || strings.TrimSpace(update.DeliveredToRunID) == strings.TrimSpace(rec.RunID)
}

func coagentUpdateDeliverableForRun(rec *types.RunRecord, update types.CoagentSourcePacket) bool {
	if rec == nil {
		return false
	}
	if isPersistentSuperAgentRun(rec) {
		if persistentSuperAdmissibleReport(update) || persistentSuperExecutablePacket(update) {
			return persistentSuperMailboxInjectable(rec, update)
		}
		return false
	}
	if update.Direction == types.LifecyclePacketDirectionControl || metadataStringValue(rec.Metadata, "request_source") == "lifecycle_texture_control" {
		if strings.TrimSpace(update.DeliveredToRunID) != strings.TrimSpace(rec.RunID) || update.DeliveredAt == nil {
			return false
		}
	}
	return true
}

func buildPersistentSuperUpdatePrompt(updates []types.CoagentSourcePacket) string {
	var b strings.Builder
	b.WriteString("Process the pending update_coagent records addressed to you as the user's persistent super actor.\n\n")
	b.WriteString("Each delivered packet is a validated packet.kind=execution_request with executable actions. When you have command output, diffs, tests, artifacts, questions, or blockers, report them back with update_coagent as packet.sources, claims, actions, questions, and notes.\n")
	for i, update := range updates {
		b.WriteString("\nUpdate ")
		b.WriteString(fmt.Sprintf("%d", i+1))
		if update.ChannelID != "" {
			b.WriteString(" channel=")
			b.WriteString(update.ChannelID)
		}
		if update.AgentID != "" {
			b.WriteString(" from=")
			b.WriteString(update.AgentID)
		}
		if kind := coagentPacketKind(update.Packet); kind != "" {
			b.WriteString(" kind=")
			b.WriteString(kind)
		}
		b.WriteString(":\n")
		b.WriteString(strings.TrimSpace(update.Content))
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}

func (rt *Runtime) hydrateLifecycleControlWorkItems(ctx context.Context, ownerID, computerID, agentID string, updates []types.CoagentSourcePacket) ([]types.WorkItemRecord, map[string]types.WorkItemRecord, string, error) {
	if len(updates) == 0 {
		return nil, nil, "", store.ErrLifecycleInvalidTransition
	}
	trajectoryID := strings.TrimSpace(updates[0].TrajectoryID)
	seenUpdates := make(map[string]bool, len(updates))
	seenWork := make(map[string]bool, len(updates))
	workByID := make(map[string]types.WorkItemRecord, len(updates))
	workItems := make([]types.WorkItemRecord, 0, len(updates))
	for _, update := range updates {
		updateID, workID := strings.TrimSpace(update.UpdateID), strings.TrimSpace(update.TargetWorkItemID)
		if updateID == "" || seenUpdates[updateID] || update.Direction != types.LifecyclePacketDirectionControl || update.OwnerID != ownerID || update.ComputerID != computerID || update.TargetAgentID != agentID || strings.TrimSpace(update.TrajectoryID) != trajectoryID || update.Disposition != types.UpdatePending || update.DeliveredAt != nil || strings.TrimSpace(update.DeliveredToRunID) != "" || update.LifecycleVersion <= 0 {
			return nil, nil, "", store.ErrLifecycleInvalidTransition
		}
		canonical, err := rt.store.GetLifecycleUpdate(ctx, ownerID, computerID, trajectoryID, agentID, update.AgentID, update.ProducerUpdateID)
		if err != nil {
			return nil, nil, "", fmt.Errorf("hydrate exact lifecycle control %s: %w", updateID, err)
		}
		if canonical.UpdateID != updateID || canonical.LifecycleVersion != update.LifecycleVersion || canonical.TargetWorkItemID != workID || canonical.TargetAgentID != agentID || canonical.TrajectoryID != trajectoryID || canonical.OwnerID != ownerID || canonical.ComputerID != computerID || canonical.Direction != types.LifecyclePacketDirectionControl || canonical.Disposition != types.UpdatePending || canonical.DeliveredAt != nil || strings.TrimSpace(canonical.DeliveredToRunID) != "" {
			return nil, nil, "", store.ErrLifecycleInvalidTransition
		}
		seenUpdates[updateID] = true
		work, err := rt.store.GetLifecycleWorkItem(ctx, ownerID, computerID, workID)
		if err != nil {
			return nil, nil, "", fmt.Errorf("hydrate exact lifecycle work %s: %w", workID, err)
		}
		if work.WorkItemID != workID || work.OwnerID != ownerID || work.ComputerID != computerID || work.TrajectoryID != trajectoryID || work.AssignedAgentID != agentID || work.Status != types.WorkItemOpen || work.LifecycleVersion <= 0 {
			return nil, nil, "", store.ErrLifecycleInvalidTransition
		}
		workByID[workID] = work
		if !seenWork[workID] {
			seenWork[workID] = true
			workItems = append(workItems, work)
		}
	}
	if trajectoryID == "" || len(workItems) == 0 {
		return nil, nil, "", store.ErrLifecycleInvalidTransition
	}
	return workItems, workByID, trajectoryID, nil
}

func lifecycleControlActivationPrompt(workItems []types.WorkItemRecord) string {
	prompt := "Continue assigned actor work. Process the coagent update packets in context."
	if workPrompt := buildAssignedWorkItemPrompt(workItems); workPrompt != "" {
		prompt += "\n\n" + workPrompt
	}
	return prompt
}

func lifecycleActivationVersionsForStore(versions []lifecycleActivationVersion) []types.LifecycleControlActivationVersion {
	out := make([]types.LifecycleControlActivationVersion, 0, len(versions))
	for _, version := range versions {
		out = append(out, types.LifecycleControlActivationVersion{UpdateID: version.UpdateID, TargetWorkItemID: version.TargetWorkItemID, ControlLifecycleVersion: version.ControlLifecycleVersion, WorkLifecycleVersion: version.WorkLifecycleVersion})
	}
	return out
}

func stampLifecycleActivationMetadata(metadata map[string]any, logicalKey, failedKey, buildCommit string, versions []lifecycleActivationVersion) map[string]any {
	metadata = cloneMetadata(metadata)
	metadata[lifecycleLogicalActivationKeyMetadata] = logicalKey
	metadata[lifecycleFailedAttemptKeyMetadata] = failedKey
	metadata[lifecycleActivationBuildMetadata] = buildCommit
	metadata[lifecycleActivationVersionsMetadata] = versions
	return metadata
}

func lifecycleActivationVersionsForRun(rec *types.RunRecord) ([]types.LifecycleControlActivationVersion, error) {
	if rec == nil || rec.Metadata == nil {
		return nil, store.ErrLifecycleInvalidTransition
	}
	raw, ok := rec.Metadata[lifecycleActivationVersionsMetadata]
	if !ok {
		return nil, store.ErrLifecycleInvalidTransition
	}
	body, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	var versions []types.LifecycleControlActivationVersion
	if err := json.Unmarshal(body, &versions); err != nil || len(versions) == 0 {
		return nil, store.ErrLifecycleInvalidTransition
	}
	return versions, nil
}

func sameOrderedStringValues(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if strings.TrimSpace(left[i]) != strings.TrimSpace(right[i]) {
			return false
		}
	}
	return true
}

func (rt *Runtime) lifecycleRunHasCanonicalControlDelivery(ctx context.Context, rec *types.RunRecord) (bool, error) {
	if rt == nil || rt.store == nil || rec == nil || strings.TrimSpace(rec.TrajectoryID) == "" {
		return false, store.ErrLifecycleInvalidTransition
	}
	controls, err := rt.store.ListLifecycleControlsDeliveredToRun(ctx, rec.OwnerID, rec.ComputerID, rec.TrajectoryID, rec.AgentID, rec.RunID, 1)
	if errors.Is(err, store.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return len(controls) > 0, nil
}

func (rt *Runtime) bindAppendedLifecycleControlsToResident(ctx context.Context, rec *types.RunRecord, updates []types.CoagentSourcePacket) (*types.RunRecord, error) {
	result, bindErr := rt.bindLifecycleControlsToRun(ctx, rec, updates)
	if bindErr == nil {
		rt.activate(rec)
		return rec, nil
	}
	if strings.TrimSpace(result.Receipt.CommandID) != "" {
		bound, reloadErr := rt.store.GetLifecycleRun(context.WithoutCancel(ctx), rec.OwnerID, rec.ComputerID, rec.RunID)
		if reloadErr == nil {
			*rec = bound
			rt.activate(rec)
			return rec, nil
		}
		return rec, fmt.Errorf("appended lifecycle control bind committed for run %s but canonical reload failed: %w", rec.RunID, bindErr)
	}
	// Later-control append failures are never deterministic activation
	// terminalization authority. Leave the control pending and the exact
	// resident active so actor delivery remains unacknowledged and retryable.
	return rec, bindErr
}

func (rt *Runtime) bindOrReplayLifecycleControlActivation(ctx context.Context, rec *types.RunRecord, updates []types.CoagentSourcePacket) (*types.RunRecord, error) {
	result, bindErr := rt.bindLifecycleControlsToRun(ctx, rec, updates)
	if bindErr == nil {
		rt.activate(rec)
		return rec, nil
	}
	if strings.TrimSpace(result.Receipt.CommandID) != "" {
		// The atomic bind is authoritative even though its reload failed. Never
		// write the stale pre-bind record. Best-effort canonical reload may allow
		// immediate dispatch; otherwise actor retry will reload the bound run.
		bound, reloadErr := rt.store.GetLifecycleRun(context.WithoutCancel(ctx), rec.OwnerID, rec.ComputerID, rec.RunID)
		if reloadErr == nil {
			*rec = bound
			rt.activate(rec)
			return rec, nil
		}
		return rec, fmt.Errorf("lifecycle control bind committed for run %s but canonical reload failed: %w", rec.RunID, bindErr)
	}
	return rec, rt.terminalizeFingerprintedLifecycleControlRun(ctx, rec, updates, bindErr)
}

func (rt *Runtime) reconcileUpdatedCoagentActor(ctx context.Context, ownerID, agentID string) (*types.RunRecord, error) {
	ownerID = strings.TrimSpace(ownerID)
	agentID = strings.TrimSpace(agentID)
	if ownerID == "" || agentID == "" {
		return nil, nil
	}
	if isTextureAgentID(agentID) {
		return nil, nil
	}
	// Reconcile is a pre-provider activation transaction. Serialize it with
	// boot work reconciliation so an actor wake and restart sweep cannot mint
	// competing provisional runs for the same canonical control join.
	rt.lifecycleWorkReconcileMu.Lock()
	defer rt.lifecycleWorkReconcileMu.Unlock()
	resident, residentFound, err := rt.activeRunByAgent(ctx, ownerID, agentID)
	if err != nil {
		return nil, fmt.Errorf("check resident coagent run: %w", err)
	}
	// This preserves the pre-cutover/legacy resident branch exactly. Only an
	// explicitly lifecycle-control activation enters fingerprint reconciliation.
	if residentFound && metadataStringValue(resident.Metadata, "request_source") != "lifecycle_texture_control" {
		return &resident, nil
	}
	computerID := strings.TrimSpace(rt.TextureComputerID())
	agent, err := rt.store.GetAgentByScope(ctx, ownerID, computerID, agentID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("lookup coagent: %w", err)
	}
	profile := agentprofile.Canonical(firstNonEmpty(agent.Profile, agent.Role))
	lifecycleAgent := profile == agentprofile.Researcher && agent.LifecycleVersion > 0
	if residentFound && !lifecycleAgent {
		return &resident, nil
	}

	var updates []types.CoagentSourcePacket
	lifecycleControls := false
	if lifecycleAgent {
		updates, err = rt.store.ListAllPendingLifecycleUpdates(ctx, ownerID, computerID, agentID)
		if err == nil {
			updates, err = rt.validateTargetBoundLifecycleControls(ctx, ownerID, computerID, agentID, updates, false)
		}
		if err != nil {
			return nil, err
		}
		if residentFound && len(updates) > 0 && metadataStringValue(resident.Metadata, lifecycleLogicalActivationKeyMetadata) != "" {
			canonicallyBound, deliveryErr := rt.lifecycleRunHasCanonicalControlDelivery(ctx, &resident)
			if deliveryErr != nil {
				return &resident, fmt.Errorf("verify resident lifecycle control delivery before append: %w", deliveryErr)
			}
			if canonicallyBound {
				appendUpdates := selectLifecycleControlActivation(updates, lifecycleControlTrajectoryForRun(&resident), lifecycleControlWorkIDsForRun(&resident))
				if len(appendUpdates) > 0 {
					return rt.bindAppendedLifecycleControlsToResident(ctx, &resident, appendUpdates)
				}
			}
		}
		// One activation owns one exact trajectory join. Other validated pending
		// trajectories remain pending for a later wake; this is the established
		// activation selection behavior, not cross-trajectory hydration.
		updates = selectLifecycleControlActivation(updates, "", nil)
		lifecycleControls = len(updates) > 0
		if lifecycleControls && !residentFound {
			parked, parkedErr := rt.parkedLifecycleControlCandidate(ctx, ownerID, computerID, agentID, updates)
			if parkedErr != nil {
				return nil, parkedErr
			}
			if parked != nil {
				// A parked actor-memory run is a valid recovery candidate but actor
				// memory is the selection authority. Boot composition must call the
				// exact-run entrypoint; generic reconcile must never guess or mint B.
				return nil, fmt.Errorf("parked lifecycle run %s requires exact actor-memory recovery: %w", parked.RunID, store.ErrLifecycleInvalidTransition)
			}
		}
		if !lifecycleControls && residentFound && metadataStringValue(resident.Metadata, lifecycleLogicalActivationKeyMetadata) != "" {
			// Recover a committed bind only from canonical exact-run delivery,
			// never from a model-shaped or stale metadata slice.
			delivered, deliveryErr := rt.lifecycleRunHasCanonicalControlDelivery(ctx, &resident)
			if deliveryErr != nil {
				return nil, fmt.Errorf("verify canonical lifecycle control delivery: %w", deliveryErr)
			}
			if delivered {
				rt.activate(&resident)
				return &resident, nil
			}
		}
	}
	if !lifecycleControls {
		// Deliberately unchanged legacy mailbox/projection path.
		updates, err = rt.store.ListCoagentMailboxBacklog(ctx, ownerID, agentID, 100)
		if err != nil {
			return nil, fmt.Errorf("list coagent pending updates: %w", err)
		}
	}
	if len(updates) == 0 {
		return nil, nil
	}
	first := updates[0]
	if profile == "" || profile == agentprofile.Email || profile == agentprofile.Conductor || profile == agentprofile.Super {
		return nil, nil
	}
	role := strings.TrimSpace(firstNonEmpty(agent.Role, profile))
	channelID := strings.TrimSpace(firstNonEmpty(agent.ChannelID, first.ChannelID))
	updateIDs := make([]string, 0, len(updates))
	for _, update := range updates {
		if id := strings.TrimSpace(update.UpdateID); id != "" {
			updateIDs = append(updateIDs, id)
		}
	}
	metadata := map[string]any{
		runMetadataAgentProfile: profile,
		runMetadataAgentRole:    role,
		runMetadataAgentID:      agentID,
		"request_source":        "update_coagent",
	}
	if !lifecycleControls {
		metadata["worker_update_ids"] = updateIDs
	}
	if channelID != "" {
		metadata[runMetadataChannelID] = channelID
	}
	if first.TrajectoryID != "" {
		metadata[runMetadataTrajectoryID] = first.TrajectoryID
	}

	var workItems []types.WorkItemRecord
	var activationVersions []lifecycleActivationVersion
	if lifecycleControls {
		var workByID map[string]types.WorkItemRecord
		var trajectoryID string
		workItems, workByID, trajectoryID, err = rt.hydrateLifecycleControlWorkItems(ctx, ownerID, computerID, agentID, updates)
		if err != nil {
			return nil, fmt.Errorf("hydrate lifecycle Researcher controls: %w", err)
		}
		logicalKey, failedKey, versions, keyErr := lifecycleActivationKeys(ownerID, computerID, trajectoryID, agentID, buildinfo.Commit, updates, workByID)
		activationVersions = versions
		if keyErr != nil {
			return nil, fmt.Errorf("fingerprint lifecycle Researcher controls: %w", keyErr)
		}
		metadata["request_source"] = "lifecycle_texture_control"
		metadata[runMetadataTrajectoryID] = trajectoryID
		metadata = stampLifecycleActivationMetadata(metadata, logicalKey, failedKey, strings.TrimSpace(buildinfo.Commit), versions)
		if workItemIDs := workItemIDsForMetadata(workItems); len(workItemIDs) > 0 {
			metadata["work_item_ids"] = workItemIDs
		}

		replay, replayErr := rt.store.ResolveLifecycleControlActivation(ctx, ownerID, computerID, trajectoryID, agentID, logicalKey, failedKey, lifecycleActivationVersionsForStore(versions))
		if replayErr != nil {
			return nil, fmt.Errorf("resolve lifecycle control activation replay: %w", replayErr)
		}
		if replay.Active != nil {
			active := replay.Active
			if active.OwnerID != ownerID || active.ComputerID != computerID || active.AgentID != agentID || active.TrajectoryID != trajectoryID || !active.State.Active() {
				return nil, store.ErrLifecycleInvalidTransition
			}
			// Refresh only the local desired projection. BindLifecycleControlDelivery
			// validates the exact control/work versions and merges these narrowly
			// owned fields into the canonical run in the same conditional batch.
			active.Prompt = lifecycleControlActivationPrompt(workItems)
			active.Metadata = stampLifecycleActivationMetadata(active.Metadata, logicalKey, failedKey, strings.TrimSpace(buildinfo.Commit), versions)
			active.Metadata["request_source"] = "lifecycle_texture_control"
			active.Metadata[runMetadataTrajectoryID] = trajectoryID
			active.Metadata["work_item_ids"] = workItemIDsForMetadata(workItems)
			return rt.bindOrReplayLifecycleControlActivation(ctx, active, updates)
		}
		if replay.DurablyFailed != nil {
			return replay.DurablyFailed, fmt.Errorf("%w: run=%s failed_attempt=%s", ErrDurablyTerminalLifecycleControlActivation, replay.DurablyFailed.RunID, failedKey)
		}
		if residentFound {
			// Another active run owns this lifecycle agent but not this exact
			// logical join. Never replace or mint around ambiguous authority.
			return &resident, store.ErrLifecycleInvalidTransition
		}
	} else {
		workItems, err = rt.assignedOpenWorkItemsForAgentUpdateBacklog(ctx, ownerID, agentID, updates)
		if err != nil {
			return nil, err
		}
		if workItemIDs := workItemIDsForMetadata(workItems); len(workItemIDs) > 0 {
			metadata["work_item_ids"] = workItemIDs
		}
	}

	prompt := lifecycleControlActivationPrompt(workItems)
	rec, err := rt.createRunWithMetadata(ctx, prompt, ownerID, metadata)
	if err != nil {
		if lifecycleControls && (errors.Is(err, store.ErrLifecycleInvalidTransition) || errors.Is(err, store.ErrConcurrentStateChange)) {
			logicalKey := metadataStringValue(metadata, lifecycleLogicalActivationKeyMetadata)
			failedKey := metadataStringValue(metadata, lifecycleFailedAttemptKeyMetadata)
			trajectoryID := metadataStringValue(metadata, runMetadataTrajectoryID)
			replay, replayErr := rt.store.ResolveLifecycleControlActivation(ctx, ownerID, computerID, trajectoryID, agentID, logicalKey, failedKey, lifecycleActivationVersionsForStore(activationVersions))
			if replayErr == nil && replay.Active != nil {
				return rt.bindOrReplayLifecycleControlActivation(ctx, replay.Active, updates)
			}
			if replayErr == nil && replay.DurablyFailed != nil {
				return replay.DurablyFailed, fmt.Errorf("%w: run=%s failed_attempt=%s", ErrDurablyTerminalLifecycleControlActivation, replay.DurablyFailed.RunID, failedKey)
			}
		}
		return nil, err
	}
	if lifecycleControls {
		return rt.bindOrReplayLifecycleControlActivation(ctx, rec, updates)
	}
	rt.activate(rec)
	return rec, nil
}

func (rt *Runtime) assignedOpenWorkItemsForAgentUpdateBacklog(ctx context.Context, ownerID, agentID string, updates []types.CoagentSourcePacket) ([]types.WorkItemRecord, error) {
	seenTrajectories := map[string]bool{}
	var out []types.WorkItemRecord
	for _, update := range updates {
		trajectoryID := strings.TrimSpace(update.TrajectoryID)
		if trajectoryID == "" || seenTrajectories[trajectoryID] {
			continue
		}
		seenTrajectories[trajectoryID] = true
		items, err := rt.assignedOpenWorkItemsForAgentTrajectory(ctx, ownerID, agentID, trajectoryID)
		if err != nil {
			return nil, err
		}
		out = append(out, items...)
	}
	return out, nil
}

func (rt *Runtime) assignedOpenWorkItemsForAgentTrajectory(ctx context.Context, ownerID, agentID, trajectoryID string) ([]types.WorkItemRecord, error) {
	ownerID = strings.TrimSpace(ownerID)
	agentID = strings.TrimSpace(agentID)
	trajectoryID = strings.TrimSpace(trajectoryID)
	if rt == nil || rt.store == nil || ownerID == "" || agentID == "" || trajectoryID == "" {
		return nil, nil
	}
	items, err := rt.store.ListWorkItemsByTrajectory(ctx, ownerID, trajectoryID, true)
	if err != nil {
		return nil, fmt.Errorf("list assigned open work items for coagent wake: %w", err)
	}
	out := make([]types.WorkItemRecord, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.AssignedAgentID) == agentID {
			out = append(out, item)
		}
	}
	return out, nil
}

func workItemIDsForMetadata(workItems []types.WorkItemRecord) []string {
	ids := make([]string, 0, len(workItems))
	seen := map[string]bool{}
	for _, item := range workItems {
		id := strings.TrimSpace(item.WorkItemID)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}
	return ids
}

func buildCoagentBacklogPrompt(updates []types.CoagentSourcePacket, workItems []types.WorkItemRecord) string {
	updatePrompt := buildCoagentUpdatePrompt(updates)
	if len(workItems) == 0 {
		return updatePrompt
	}
	workPrompt := buildAssignedWorkItemPrompt(workItems)
	if updatePrompt == "" {
		return workPrompt
	}
	if workPrompt == "" {
		return updatePrompt
	}
	return updatePrompt + "\n\n" + workPrompt
}

func buildCoagentUpdatePrompt(updates []types.CoagentSourcePacket) string {
	var b strings.Builder
	b.WriteString("Process the pending update_coagent records addressed to you.\n")
	b.WriteString("Respond with the appropriate tool or final result for your role; report blockers with update_coagent when you cannot proceed.\n")
	for i, update := range updates {
		b.WriteString("\nUpdate ")
		b.WriteString(fmt.Sprintf("%d", i+1))
		if update.ChannelID != "" {
			b.WriteString(" channel=")
			b.WriteString(update.ChannelID)
		}
		if update.AgentID != "" {
			b.WriteString(" from=")
			b.WriteString(update.AgentID)
		}
		if kind := coagentPacketKind(update.Packet); kind != "" {
			b.WriteString(" kind=")
			b.WriteString(kind)
		}
		b.WriteString(":\n")
		b.WriteString(strings.TrimSpace(update.Content))
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}

func (rt *Runtime) coagentUpdateTurnInjector(rec *types.RunRecord) toolregistry.InjectUserTurnsFunc {
	return rt.coagentUpdateTurnInjectorWithInitialPhase(rec, "")
}

// CoagentUpdateTurnInjector exposes the one production injector to owner-layer
// fixtures without reimplementing lifecycle delivery or seen-occurrence fate.
func (rt *Runtime) CoagentUpdateTurnInjector(rec *types.RunRecord) toolregistry.InjectUserTurnsFunc {
	return rt.coagentUpdateTurnInjector(rec)
}

func (rt *Runtime) pendingCoagentUpdatesForRun(ctx context.Context, rec *types.RunRecord, ownerID, agentID string, limit int) ([]types.CoagentSourcePacket, error) {
	lifecycleRun := false
	if rec != nil && strings.TrimSpace(rec.OwnerID) != "" && strings.TrimSpace(rec.ComputerID) != "" && strings.TrimSpace(rec.RunID) != "" {
		if _, err := rt.store.GetLifecycleRun(ctx, rec.OwnerID, rec.ComputerID, rec.RunID); err == nil {
			lifecycleRun = true
		} else if !errors.Is(err, store.ErrNotFound) {
			return nil, fmt.Errorf("resolve lifecycle run authority: %w", err)
		}
	}
	computerID := strings.TrimSpace(rec.ComputerID)
	if isPersistentSuperAgentRun(rec) {
		if metadataStringValue(rec.Metadata, "request_source") == "lifecycle_texture_control" {
			return rt.listPendingLifecyclePacketsDeliveredToRun(ctx, rec)
		}
		return rt.store.ListCoagentMailboxBacklog(ctx, ownerID, agentID, limit)
	}
	if lifecycleRun {
		if computerID == "" {
			return nil, fmt.Errorf("list pending lifecycle updates: computer_id is required")
		}
		if agentProfileForRun(rec) == agentprofile.Researcher {
			return rt.listPendingLifecyclePacketsDeliveredToRun(ctx, rec)
		}
		return rt.store.ListAllPendingLifecycleUpdates(ctx, ownerID, computerID, agentID)
	}
	return rt.store.ListCoagentMailboxBacklog(ctx, ownerID, agentID, limit)
}

const (
	textureOwnerInstructionIDsMetadataRuntime = "texture_owner_instruction_ids"
	textureOwnerRequestIDsMetadataRuntime     = "texture_owner_request_ids"
)

func (rt *Runtime) lifecycleOwnerInstructionTurnsForRun(ctx context.Context, rec *types.RunRecord, phase string, seen map[string]bool) ([]json.RawMessage, []string, error) {
	if rt == nil || rt.store == nil || rec == nil || agentProfileForRun(rec) != agentprofile.Texture {
		return nil, nil, nil
	}
	ownerID, computerID, trajectoryID, agentID := strings.TrimSpace(rec.OwnerID), strings.TrimSpace(rec.ComputerID), strings.TrimSpace(rec.TrajectoryID), strings.TrimSpace(rec.AgentID)
	docID := strings.TrimSpace(firstNonEmpty(metadataStringValue(rec.Metadata, "doc_id"), rec.ChannelID))
	if ownerID == "" || computerID == "" || trajectoryID == "" || agentID == "" || docID == "" {
		return nil, nil, nil
	}
	snapshot, err := rt.store.GetLifecycleSnapshot(ctx, ownerID, computerID, trajectoryID)
	if err != nil || snapshot.Document.DocID != docID || snapshot.Document.TrajectoryID != trajectoryID {
		return nil, nil, fmt.Errorf("owner instruction lifecycle scope is unavailable")
	}
	instructions, err := rt.store.ListPendingLifecycleOwnerInstructionsForHead(ctx, ownerID, computerID, trajectoryID, agentID, snapshot.Document.CurrentRevisionID)
	if err != nil {
		return nil, nil, err
	}
	if rec.Metadata == nil {
		rec.Metadata = map[string]any{}
	}
	rec.Metadata[textureOwnerInstructionIDsMetadataRuntime] = []string{}
	rec.Metadata[textureOwnerRequestIDsMetadataRuntime] = []string{}
	if len(instructions) == 0 {
		return nil, nil, nil
	}
	openWork := map[string]bool{}
	for _, work := range snapshot.WorkItems {
		if work.Status == types.WorkItemOpen && work.OwnerID == ownerID && work.ComputerID == computerID && work.TrajectoryID == trajectoryID && work.AssignedAgentID == agentID {
			openWork[work.WorkItemID] = true
		}
	}
	instructionIDs, requestIDs := make([]string, 0, len(instructions)), make([]string, 0, len(instructions))
	for _, instruction := range instructions {
		if instruction.Schema != types.LifecycleOwnerInstructionSchemaV1 || instruction.DocumentID != docID || instruction.TrajectoryID != trajectoryID || instruction.TargetAgentID != agentID || !openWork[instruction.TargetWorkItemID] {
			return nil, nil, fmt.Errorf("owner instruction %q fails exact run/work binding", instruction.InstructionID)
		}
		instructionIDs, requestIDs = append(instructionIDs, instruction.InstructionID), append(requestIDs, instruction.RequestID)
	}
	rec.Metadata[textureOwnerInstructionIDsMetadataRuntime], rec.Metadata[textureOwnerRequestIDsMetadataRuntime] = instructionIDs, requestIDs
	fresh := make([]types.LifecycleOwnerInstruction, 0, len(instructions))
	freshIDs := make([]string, 0, len(instructions))
	for _, instruction := range instructions {
		if !seen[instruction.InstructionID] {
			fresh = append(fresh, instruction)
			freshIDs = append(freshIDs, instruction.InstructionID)
		}
	}
	if len(fresh) == 0 {
		return nil, nil, nil
	}
	payload, err := json.Marshal(map[string]any{"schema": lifecycleInjectionEnvelopeSchemaV1, "packet_type": "owner_instruction", "owner_id": ownerID, "computer_id": computerID, "target_run_id": rec.RunID, "delivery_phase": phase, "document_id": docID, "trajectory_id": trajectoryID, "target_agent_id": agentID, "instructions": fresh})
	if err != nil {
		return nil, nil, err
	}
	message, err := json.Marshal(map[string]any{"role": "user", "content": []map[string]string{{"type": "text", "text": "Choir authenticated owner instruction packet.\n\n" + string(payload)}}})
	if err != nil {
		return nil, nil, err
	}
	return []json.RawMessage{message}, freshIDs, nil
}

func lifecycleInjectionIDsFromRunMemory(rec *types.RunRecord, entries []types.RunMemoryEntry) (map[string]bool, map[string]bool) {
	updates, owners := map[string]bool{}, map[string]bool{}
	if rec == nil {
		return updates, owners
	}
	for _, entry := range entries {
		if entry.Kind != types.RunMemoryEntryMessage || entry.Role != types.RunMemoryRoleRuntimeInjection || len(entry.Message) == 0 {
			continue
		}
		for _, text := range runMemoryUserMessageTexts(entry.Message) {
			packetType := ""
			switch {
			case strings.HasPrefix(text, "Choir authenticated owner instruction packet.\n\n"):
				packetType = "owner_instruction"
			default:
				for _, phase := range []string{coagentPacketDeliveryMid, coagentPacketDeliveryFinal, coagentPacketDeliveryCold, coagentPacketDeliveryThread} {
					if strings.HasPrefix(text, coagentUpdatePacketPreamble(phase)+"\n\n") {
						packetType = coagentPacketTypeUpdate
						break
					}
				}
			}
			start := strings.Index(text, "{")
			if packetType == "" || start < 0 {
				continue
			}
			var envelope struct {
				Schema        string `json:"schema"`
				PacketType    string `json:"packet_type"`
				OwnerID       string `json:"owner_id"`
				ComputerID    string `json:"computer_id"`
				TrajectoryID  string `json:"trajectory_id"`
				TargetAgentID string `json:"target_agent_id"`
				TargetRunID   string `json:"target_run_id"`
				Updates       []struct {
					UpdateID string `json:"update_id"`
				} `json:"updates"`
				Instructions []struct {
					InstructionID string `json:"instruction_id"`
				} `json:"instructions"`
			}
			expectedTrajectory := lifecycleControlTrajectoryForRun(rec)
			if json.Unmarshal([]byte(text[start:]), &envelope) != nil {
				continue
			}
			targetRunID, expectedRunID := strings.TrimSpace(envelope.TargetRunID), strings.TrimSpace(rec.RunID)
			exactLifecycleDelivery := metadataStringValue(rec.Metadata, "request_source") == "lifecycle_texture_control"
			ownerID, computerID := strings.TrimSpace(envelope.OwnerID), strings.TrimSpace(envelope.ComputerID)
			if envelope.Schema != lifecycleInjectionEnvelopeSchemaV1 || envelope.PacketType != packetType ||
				(ownerID != "" && ownerID != strings.TrimSpace(rec.OwnerID)) ||
				(computerID != "" && computerID != strings.TrimSpace(rec.ComputerID)) ||
				strings.TrimSpace(envelope.TrajectoryID) != expectedTrajectory ||
				strings.TrimSpace(envelope.TargetAgentID) != strings.TrimSpace(rec.AgentID) ||
				(targetRunID != "" && targetRunID != expectedRunID) ||
				(exactLifecycleDelivery && (ownerID != strings.TrimSpace(rec.OwnerID) || computerID != strings.TrimSpace(rec.ComputerID) || targetRunID != expectedRunID)) {
				continue
			}
			switch envelope.PacketType {
			case coagentPacketTypeUpdate:
				for _, update := range envelope.Updates {
					if id := strings.TrimSpace(update.UpdateID); id != "" {
						updates[id] = true
					}
				}
			case "owner_instruction":
				for _, instruction := range envelope.Instructions {
					if id := strings.TrimSpace(instruction.InstructionID); id != "" {
						owners[id] = true
					}
				}
			}
		}
	}
	return updates, owners
}

func runMemoryUserMessageTexts(raw json.RawMessage) []string {
	var message struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	}
	if json.Unmarshal(raw, &message) != nil || strings.TrimSpace(message.Role) != "user" {
		return nil
	}
	var scalar string
	if json.Unmarshal(message.Content, &scalar) == nil {
		return []string{scalar}
	}
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(message.Content, &blocks) != nil {
		return nil
	}
	out := make([]string, 0, len(blocks))
	for _, block := range blocks {
		if block.Type == "text" && strings.TrimSpace(block.Text) != "" {
			out = append(out, block.Text)
		}
	}
	return out
}

func (rt *Runtime) coagentUpdateTurnInjectorWithInitialPhase(rec *types.RunRecord, initialPhase string) toolregistry.InjectUserTurnsFunc {
	if rt == nil || rt.store == nil || rec == nil || !runSupportsCoagentUpdateInjection(rec) {
		return nil
	}
	ownerID := strings.TrimSpace(rec.OwnerID)
	agentID := strings.TrimSpace(rec.AgentID)
	if ownerID == "" || agentID == "" {
		return nil
	}
	initialPhase = strings.TrimSpace(initialPhase)
	return func(finalCheckpoint bool) ([]json.RawMessage, error) {
		// Durable runtime-authenticated memory is the sole seen-occurrence
		// authority. Do not mutate a process-local seen map or trust RunRecord
		// metadata before the returned message has actually appended.
		entries, err := rt.store.ListRunMemoryEntries(context.Background(), rec.OwnerID, rec.RunID)
		if err != nil {
			return nil, fmt.Errorf("derive delivered lifecycle occurrences from run memory: %w", err)
		}
		seenUpdates, seenOwnerInstructions := lifecycleInjectionIDsFromRunMemory(rec, entries)
		phase := coagentPacketDeliveryMid
		if finalCheckpoint {
			phase = coagentPacketDeliveryFinal
		} else if initialPhase != "" && len(seenUpdates) == 0 && len(seenOwnerInstructions) == 0 {
			// The first phase is durable-memory-derived rather than process-local:
			// an append failure retries the identical cold/thread projection, while
			// later arrivals after a successful append are mid-activation turns.
			phase = initialPhase
		}
		ownerMessages, _, err := rt.lifecycleOwnerInstructionTurnsForRun(context.Background(), rec, phase, seenOwnerInstructions)
		if err != nil {
			return nil, fmt.Errorf("list pending owner instruction turns: %w", err)
		}
		updates, err := rt.pendingCoagentUpdatesForRun(context.Background(), rec, ownerID, agentID, 100)
		if err != nil {
			return nil, fmt.Errorf("list pending update_coagent turns: %w", err)
		}
		fresh := make([]types.CoagentSourcePacket, 0, len(updates))
		for _, update := range updates {
			id := strings.TrimSpace(update.UpdateID)
			if id == "" || seenUpdates[id] || !coagentUpdateDeliverableForRun(rec, update) {
				continue
			}
			fresh = append(fresh, update)
		}
		if len(fresh) == 0 {
			return ownerMessages, nil
		}
		projected, err := rt.projectTerminalOutcomeContent(context.Background(), fresh)
		if err != nil {
			return nil, err
		}
		msgs, _, err := buildCoagentUpdateUserMessages(projected, phase, agentID, nil, nil)
		if err != nil {
			return nil, err
		}
		return append(ownerMessages, msgs...), nil
	}
}

func shouldAppendInitialCoagentMailboxTurns(rec *types.RunRecord) bool {
	if rec == nil {
		return false
	}
	requestSource := metadataStringValue(rec.Metadata, "request_source")
	// Exact lifecycle packets must pass through the authenticated runtime
	// injection append, including Researcher and persistent Super cold starts.
	if requestSource == "lifecycle_texture_control" {
		return true
	}
	if agentProfileForRun(rec) != agentprofile.Texture {
		return false
	}
	if metadataStringValue(rec.Metadata, "request_intent") == "apply_owner_instruction" {
		return true
	}
	if requestSource == "update_coagent" {
		return true
	}
	return len(coagentUpdateIDsForRun(rec)) > 0
}

func (rt *Runtime) coagentParkWaiter(rec *types.RunRecord) toolregistry.ToolLoopParkWaiterFunc {
	if rt == nil || rt.store == nil || rec == nil || !runSupportsCoagentUpdateInjection(rec) {
		return nil
	}
	if !metadataBoolValue(rec.Metadata, "actor_park_on_idle") {
		return nil
	}
	ownerID := strings.TrimSpace(rec.OwnerID)
	agentID := strings.TrimSpace(rec.AgentID)
	if ownerID == "" || agentID == "" {
		return nil
	}
	return func(ctx context.Context, state toolregistry.ToolLoopParkState) (toolregistry.ToolLoopParkResult, error) {
		ready := func() (bool, error) {
			updates, err := rt.pendingCoagentUpdatesForRun(ctx, rec, ownerID, agentID, 100)
			if err != nil {
				return false, fmt.Errorf("list pending update_coagent records for park wait: %w", err)
			}
			entries, err := rt.store.ListRunMemoryEntries(ctx, rec.OwnerID, rec.RunID)
			if err != nil {
				return false, fmt.Errorf("derive parked delivery occurrences from run memory: %w", err)
			}
			seen, _ := lifecycleInjectionIDsFromRunMemory(rec, entries)
			for _, update := range updates {
				id := strings.TrimSpace(update.UpdateID)
				if id != "" && !seen[id] {
					if !coagentUpdateDeliverableForRun(rec, update) {
						continue
					}
					return true, nil
				}
			}
			return false, nil
		}
		// Actor mode: do not block on a channel. If there are no
		// pending updates, passivate immediately. The actor will
		// re-activate when a new coagent update arrives via actor.Send,
		// and the handler will resume the tool loop from the park point.
		ok, err := ready()
		if err != nil {
			return toolregistry.ToolLoopParkResult{}, err
		}
		if ok {
			return toolregistry.ToolLoopParkResult{Continue: true, Reason: "update_coagent_signal"}, nil
		}
		return toolregistry.ToolLoopParkResult{Continue: false, Passivate: true, Reason: "idle_actor_passivate"}, nil
	}
}

func runSupportsCoagentUpdateInjection(rec *types.RunRecord) bool {
	if rec == nil {
		return false
	}
	switch agentProfileForRun(rec) {
	case agentprofile.Super, agentprofile.CoSuper, agentprofile.Researcher, agentprofile.Texture:
		return strings.TrimSpace(rec.AgentID) != ""
	default:
		return false
	}
}

func (rt *Runtime) prependInitialCoagentUpdatePackets(ctx context.Context, rec *types.RunRecord, messages []json.RawMessage) ([]json.RawMessage, error) {
	if rt == nil || rt.store == nil || rec == nil || !runSupportsCoagentUpdateInjection(rec) {
		return messages, nil
	}
	if !shouldPrependInitialCoagentUpdates(rec) {
		return messages, nil
	}
	ownerID := strings.TrimSpace(rec.OwnerID)
	agentID := strings.TrimSpace(rec.AgentID)
	if ownerID == "" || agentID == "" {
		return messages, nil
	}
	entries, err := rt.store.ListRunMemoryEntries(ctx, rec.OwnerID, rec.RunID)
	if err != nil {
		return messages, fmt.Errorf("derive cold delivery occurrences from run memory: %w", err)
	}
	seen, _ := lifecycleInjectionIDsFromRunMemory(rec, entries)
	updates, err := rt.pendingCoagentUpdatesForRun(ctx, rec, ownerID, agentID, 100)
	if err != nil {
		return messages, fmt.Errorf("list pending coagent updates for cold delivery: %w", err)
	}
	fresh := make([]types.CoagentSourcePacket, 0, len(updates))
	for _, update := range updates {
		id := strings.TrimSpace(update.UpdateID)
		if id == "" || seen[id] {
			continue
		}
		if !coagentUpdateDeliverableForRun(rec, update) {
			continue
		}
		seen[id] = true
		fresh = append(fresh, update)
	}
	if len(fresh) == 0 {
		return messages, nil
	}
	projected, err := rt.projectTerminalOutcomeContent(ctx, fresh)
	if err != nil {
		return messages, err
	}
	msgs, _, err := buildCoagentUpdateUserMessages(projected, coagentPacketDeliveryCold, agentID, nil, nil)
	if err != nil {
		return messages, err
	}
	return append(msgs, messages...), nil
}

func shouldPrependInitialCoagentUpdates(rec *types.RunRecord) bool {
	if rec == nil {
		return false
	}
	if agentProfileForRun(rec) == agentprofile.Texture {
		return false
	}
	requestSource := metadataStringValue(rec.Metadata, "request_source")
	if requestSource == "update_coagent" || requestSource == "lifecycle_texture_control" {
		return true
	}
	if agentProfileForRun(rec) == agentprofile.Researcher && len(lifecycleControlWorkIDsForRun(rec)) > 0 {
		return true
	}
	return len(coagentUpdateIDsForRun(rec)) > 0
}

// parkedLifecycleControlCandidate resolves restart recovery only inside the
// exact pending trajectory/work join. Passivated/blocked recency is never
// authority: exactly one fingerprinted run with canonical prior delivery must
// exist, otherwise recovery either has no candidate or fails closed.
func (rt *Runtime) parkedLifecycleControlCandidate(ctx context.Context, ownerID, computerID, agentID string, updates []types.CoagentSourcePacket) (*types.RunRecord, error) {
	if len(updates) == 0 {
		return nil, nil
	}
	trajectoryID := strings.TrimSpace(updates[0].TrajectoryID)
	if trajectoryID == "" {
		return nil, store.ErrLifecycleInvalidTransition
	}
	targetWork := make(map[string]struct{}, len(updates))
	for _, update := range updates {
		if strings.TrimSpace(update.TrajectoryID) != trajectoryID || strings.TrimSpace(update.TargetAgentID) != agentID {
			return nil, store.ErrLifecycleInvalidTransition
		}
		workID := strings.TrimSpace(update.TargetWorkItemID)
		if workID == "" {
			return nil, store.ErrLifecycleInvalidTransition
		}
		targetWork[workID] = struct{}{}
	}
	runs, err := rt.store.ListLifecycleRunsByTrajectory(ctx, ownerID, computerID, trajectoryID, 0)
	if err != nil {
		return nil, err
	}
	var candidate *types.RunRecord
	for index := range runs {
		run := &runs[index]
		if strings.TrimSpace(run.AgentID) != agentID || (run.State != types.RunPassivated && run.State != types.RunBlocked) ||
			agentprofile.Canonical(run.AgentProfile) != agentprofile.Researcher || agentprofile.Canonical(run.AgentRole) != agentprofile.Researcher ||
			metadataStringValue(run.Metadata, "request_source") != "lifecycle_texture_control" ||
			metadataStringValue(run.Metadata, lifecycleLogicalActivationKeyMetadata) == "" || metadataStringValue(run.Metadata, lifecycleFailedAttemptKeyMetadata) == "" {
			continue
		}
		runWork := make(map[string]struct{})
		for workID := range lifecycleControlWorkIDsForRun(run) {
			runWork[strings.TrimSpace(workID)] = struct{}{}
		}
		matches := true
		for workID := range targetWork {
			if _, ok := runWork[workID]; !ok {
				matches = false
				break
			}
		}
		if !matches {
			continue
		}
		delivered, deliveryErr := rt.lifecycleRunHasCanonicalControlDelivery(ctx, run)
		if deliveryErr != nil {
			return nil, fmt.Errorf("verify boot parked lifecycle candidate %s: %w", run.RunID, deliveryErr)
		}
		if !delivered {
			continue
		}
		if candidate != nil {
			return nil, fmt.Errorf("ambiguous canonical parked lifecycle candidates %s and %s: %w", candidate.RunID, run.RunID, store.ErrLifecycleInvalidTransition)
		}
		copyRun := *run
		candidate = &copyRun
	}
	return candidate, nil
}

func (rt *Runtime) enqueueCanonicalLifecycleControlOccurrences(ctx context.Context, rec *types.RunRecord) error {
	if rt == nil || rt.store == nil || rec == nil || rt.dispatchActor == nil {
		return fmt.Errorf("runtime lifecycle occurrence dispatch unavailable")
	}
	trajectoryID := lifecycleControlTrajectoryForRun(rec)
	var after int64
	for {
		page, err := rt.store.ListLifecycleControlsDeliveredToRunPage(ctx, rec.OwnerID, rec.ComputerID, trajectoryID, rec.AgentID, rec.RunID, after, 100)
		if err != nil {
			return fmt.Errorf("list canonical lifecycle occurrences for run %s: %w", rec.RunID, err)
		}
		for _, update := range page.Packets {
			content := lifecycleControlActorOccurrenceContent(update)
			if content == "" {
				return store.ErrLifecycleInvalidTransition
			}
			if err := rt.dispatchActor(context.WithoutCancel(ctx), rec.OwnerID, rec.ComputerID, rec.AgentID, "coagent_result", content, trajectoryID, update.AgentID); err != nil {
				return fmt.Errorf("enqueue canonical lifecycle occurrence %s for run %s: %w", update.UpdateID, rec.RunID, err)
			}
		}
		if !page.HasMore {
			return nil
		}
		if page.NextCursor <= after {
			return fmt.Errorf("non-advancing lifecycle occurrence cursor for run %s: %w", rec.RunID, store.ErrLifecycleInvalidTransition)
		}
		after = page.NextCursor
	}
}

// ReconcileParkedLifecycleCoagentWake reactivates one exact actor-memory
// Researcher run and appends any pending controls before the handler may
// execute or acknowledge the wake. The actor-supplied run ID is required;
// broad passivated-run discovery is not request-path authority.
func (rt *Runtime) ReconcileParkedLifecycleCoagentWake(ctx context.Context, ownerID, agentID, runID string) (*types.RunRecord, error) {
	if rt == nil || rt.store == nil {
		return nil, fmt.Errorf("runtime store unavailable")
	}
	rt.lifecycleWorkReconcileMu.Lock()
	defer rt.lifecycleWorkReconcileMu.Unlock()
	return rt.reconcileParkedLifecycleCoagentWakeLocked(ctx, ownerID, agentID, runID)
}

func (rt *Runtime) reconcileParkedLifecycleCoagentWakeLocked(ctx context.Context, ownerID, agentID, runID string) (*types.RunRecord, error) {
	ownerID, agentID, runID = strings.TrimSpace(ownerID), strings.TrimSpace(agentID), strings.TrimSpace(runID)
	computerID := strings.TrimSpace(rt.TextureComputerID())
	if ownerID == "" || computerID == "" || agentID == "" || runID == "" {
		return nil, store.ErrLifecycleInvalidTransition
	}
	agent, err := rt.store.GetAgentByScope(ctx, ownerID, computerID, agentID)
	if err != nil {
		return nil, err
	}
	if agent.OwnerID != ownerID || agent.ComputerID != computerID || agent.AgentID != agentID || agentprofile.Canonical(agent.Profile) != agentprofile.Researcher || agentprofile.Canonical(agent.Role) != agentprofile.Researcher || agent.LifecycleVersion <= 0 {
		return nil, store.ErrLifecycleInvalidTransition
	}
	rec, err := rt.store.GetLifecycleRun(ctx, ownerID, computerID, runID)
	if err != nil {
		return nil, err
	}
	if rec.OwnerID != ownerID || rec.ComputerID != computerID || rec.AgentID != agentID || agentprofile.Canonical(rec.AgentProfile) != agentprofile.Researcher || agentprofile.Canonical(rec.AgentRole) != agentprofile.Researcher ||
		(rec.State != types.RunPassivated && !rec.State.Active()) || metadataStringValue(rec.Metadata, "request_source") != "lifecycle_texture_control" ||
		metadataStringValue(rec.Metadata, lifecycleLogicalActivationKeyMetadata) == "" || metadataStringValue(rec.Metadata, lifecycleFailedAttemptKeyMetadata) == "" {
		return nil, store.ErrLifecycleInvalidTransition
	}
	if activeRunID := strings.TrimSpace(agent.ActiveRunID); activeRunID != "" && activeRunID != runID {
		return nil, store.ErrLifecycleInvalidTransition
	}
	canonicallyBound, err := rt.lifecycleRunHasCanonicalControlDelivery(ctx, &rec)
	if err != nil {
		return nil, fmt.Errorf("verify parked lifecycle control delivery: %w", err)
	}
	if !canonicallyBound {
		return nil, store.ErrLifecycleInvalidTransition
	}
	updates, err := rt.store.ListAllPendingLifecycleUpdates(ctx, ownerID, computerID, agentID)
	if err == nil {
		updates, err = rt.validateTargetBoundLifecycleControls(ctx, ownerID, computerID, agentID, updates, false)
	}
	if err != nil {
		return nil, err
	}
	validatedPendingCount := len(updates)
	updates = selectLifecycleControlActivation(updates, lifecycleControlTrajectoryForRun(&rec), lifecycleControlWorkIDsForRun(&rec))
	if validatedPendingCount > 0 && len(updates) == 0 {
		return nil, fmt.Errorf("pending lifecycle controls do not target exact parked run trajectory/work: %w", store.ErrLifecycleInvalidTransition)
	}
	if rec.State != types.RunPending && rec.State != types.RunRunning {
		// Passivated and blocked lifecycle projections do not own Agent.ActiveRunID.
		// Narrowly reactivate the exact actor-memory run before append.
		rec.Metadata = cloneMetadata(rec.Metadata)
		rec.State, rec.Error, rec.Result, rec.FinishedAt = types.RunPending, "", "", nil
		rec.UpdatedAt = time.Now().UTC()
		if err := rt.store.UpdateRun(ctx, rec); err != nil {
			return nil, fmt.Errorf("reactivate exact parked lifecycle run %s: %w", runID, err)
		}
	}
	if len(updates) > 0 {
		bound, bindErr := rt.bindAppendedLifecycleControlsToResident(ctx, &rec, updates)
		if bindErr != nil {
			return bound, bindErr
		}
		if err := rt.enqueueCanonicalLifecycleControlOccurrences(ctx, bound); err != nil {
			return bound, err
		}
		return bound, nil
	}
	// A retry after append commit but before actor acknowledgement observes no
	// pending controls. Re-enqueue every canonical occurrence: the original A
	// and any already durable B IDs deduplicate while an unpersisted B is created.
	rt.activate(&rec)
	if err := rt.enqueueCanonicalLifecycleControlOccurrences(ctx, &rec); err != nil {
		return &rec, err
	}
	return &rec, nil
}

// ReconcileCoagentWake is the actor-mode entry point for creating a new run
// when a coagent update arrives for an agent with no parked run. It is called
// by the actor handler (handleCoagentResult) when the actor's memory snapshot
// has no resume pointer. The reconcile logic creates a new run (if appropriate
// for the agent type — Texture, persistent super, or generic coagent) and
// calls rt.activate(rec), which sends an initial_dispatch actor message.
func (rt *Runtime) ReconcileCoagentWake(ctx context.Context, ownerID, agentID string) (*types.RunRecord, error) {
	ownerID = strings.TrimSpace(ownerID)
	agentID = strings.TrimSpace(agentID)
	if ownerID == "" || agentID == "" {
		return nil, nil
	}
	if agentID == persistentSuperAgentID(ownerID) {
		return rt.reconcilePersistentSuperActor(ctx, ownerID, agentID)
	}
	return rt.reconcileUpdatedCoagentActor(ctx, ownerID, agentID)
}

// LifecycleControlActorOccurrenceContent returns the deterministic actor-log
// occurrence identity for one canonical lifecycle packet. Adapter boot recovery
// uses the same authority when converging already durable occurrences.
func LifecycleControlActorOccurrenceContent(update types.CoagentSourcePacket) string {
	return lifecycleControlActorOccurrenceContent(update)
}

func lifecycleControlActorOccurrenceContent(update types.CoagentSourcePacket) string {
	updateID := strings.TrimSpace(update.UpdateID)
	if updateID == "" {
		return ""
	}

	// Length prefixes make authored update identifiers unambiguous even when
	// either one contains the delimiter used by the legacy occurrence content.
	identity := make([]byte, 0, len(updateID)+len(update.ProducerUpdateID)+64)
	for _, field := range []string{
		"choir:lifecycle-control-actor-occurrence:v2",
		updateID,
		strings.TrimSpace(update.ProducerUpdateID),
	} {
		identity = binary.AppendUvarint(identity, uint64(len(field)))
		identity = append(identity, field...)
	}
	digest := sha256.Sum256(identity)
	return "sha256:" + hex.EncodeToString(digest[:])
}

func (rt *Runtime) wakeUpdatedCoagent(ctx context.Context, update types.CoagentSourcePacket) {
	if rt == nil || rt.store == nil {
		return
	}
	target := strings.TrimSpace(update.TargetAgentID)
	if target == "" {
		return
	}
	if update.Direction == types.LifecyclePacketDirectionControl {
		if resident, found, err := rt.activeRunByAgent(ctx, update.OwnerID, target); err != nil {
			log.Printf("runtime: resolve resident lifecycle control target %s: %v", target, err)
			return
		} else if found {
			updates, listErr := rt.store.ListAllPendingLifecycleUpdates(ctx, update.OwnerID, update.ComputerID, target)
			if listErr == nil {
				updates, listErr = rt.validateTargetBoundLifecycleControls(ctx, update.OwnerID, update.ComputerID, target, updates, agentProfileForRun(&resident) == agentprofile.Super)
			}
			updates = selectLifecycleControlActivation(updates, resident.TrajectoryID, lifecycleControlWorkIDsForRun(&resident))
			if listErr != nil || len(updates) == 0 {
				log.Printf("runtime: bind resident lifecycle controls target=%s: %v", target, listErr)
				return
			}
			if _, bindErr := rt.bindLifecycleControlsToRun(ctx, &resident, updates); bindErr != nil {
				log.Printf("runtime: bind resident lifecycle controls target=%s: %v", target, bindErr)
				return
			}
		}
	}
	// The coagent update is already in canonical lifecycle state. Send an actor
	// message to wake the target agent — the handler will resume the
	// parked run (or start a new one) and inject the update via
	// injectUserTurns. No channel signal, no reconcile-new-run.
	if rt.dispatchActor == nil {
		panic("runtime: wakeUpdatedCoagent called without dispatchActor set — actor runtime is required")
	}
	if err := rt.dispatchActor(context.Background(), update.OwnerID, firstNonEmpty(update.ComputerID, rt.TextureComputerID()), target, "coagent_result", lifecycleControlActorOccurrenceContent(update), update.TrajectoryID, update.AgentID); err != nil {
		log.Printf("runtime: actor wake coagent for update %s: %v", update.UpdateID, err)
	}
}
