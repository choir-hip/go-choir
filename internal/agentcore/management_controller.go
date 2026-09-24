package agentcore

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/buildinfo"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
)

const runMetadataWorkerUpdatesInjected = "worker_updates_injected"
const runMetadataProducerReportIDs = "producer_report_ids"
const runMetadataEngineeringReplacementRequested = "cosuper_replacement_requested"
const runMetadataEngineeringReplacementOmitReports = "cosuper_replacement_omit_reports"

const persistentManagementCoagentInboxPrompt = "Process pending coagent update packets for privileged execution."
const persistentManagementEngineeringCancelContinuationPrompt = "Prior implementation Engineering assignment is terminal. Open a fresh implementation Engineering assignment."

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

// Version two intentionally advances the actor-log identity after the
// superseded v1 occurrence was durably acknowledged before multi-packet
// recovery validation was corrected.
const PersistentManagementRecoveryPrefix = "persistent-super-recovery:v2:"

var ErrInvalidPersistentManagementRecovery = errors.New("invalid persistent Management recovery occurrence")

type PersistentManagementRecoveryControl struct {
	UpdateID         string
	AgentID          string
	Direction        types.LifecyclePacketDirection
	LifecycleVersion int64
	ReducerSeq       int64
}

type PersistentManagementRecoveryOccurrence struct {
	OwnerID, ComputerID, TrajectoryID, AgentID, RunID, SourceAgentID string
	Controls                                                         []PersistentManagementRecoveryControl
}

func EncodePersistentManagementRecovery(o PersistentManagementRecoveryOccurrence) (string, error) {
	fields := []string{o.OwnerID, o.ComputerID, o.TrajectoryID, o.AgentID, o.RunID, o.SourceAgentID, strconv.Itoa(len(o.Controls))}
	for _, field := range fields[:6] {
		if strings.TrimSpace(field) == "" {
			return "", fmt.Errorf("%w: incomplete scope", ErrInvalidPersistentManagementRecovery)
		}
	}
	if len(o.Controls) == 0 || len(o.Controls) > 10000 {
		return "", fmt.Errorf("%w: controls are required", ErrInvalidPersistentManagementRecovery)
	}
	raw := make([]byte, 0, 512)
	for _, field := range fields {
		raw = appendTextureOccurrenceField(raw, strings.TrimSpace(field))
	}
	for _, control := range o.Controls {
		if strings.TrimSpace(control.UpdateID) == "" || strings.TrimSpace(control.AgentID) == "" ||
			control.Direction == "" || control.LifecycleVersion <= 0 || control.ReducerSeq <= 0 ||
			strings.TrimSpace(control.AgentID) != strings.TrimSpace(o.SourceAgentID) {
			return "", fmt.Errorf("%w: invalid control", ErrInvalidPersistentManagementRecovery)
		}
		raw = appendTextureOccurrenceField(raw, strings.TrimSpace(control.UpdateID))
		raw = appendTextureOccurrenceField(raw, strings.TrimSpace(control.AgentID))
		raw = appendTextureOccurrenceField(raw, string(control.Direction))
		raw = appendTextureOccurrenceField(raw, strconv.FormatInt(control.LifecycleVersion, 10))
		raw = appendTextureOccurrenceField(raw, strconv.FormatInt(control.ReducerSeq, 10))
	}
	return PersistentManagementRecoveryPrefix + base64.RawURLEncoding.EncodeToString(raw), nil
}

func DecodePersistentManagementRecovery(content string) (PersistentManagementRecoveryOccurrence, error) {
	var out PersistentManagementRecoveryOccurrence
	content = strings.TrimSpace(content)
	if !strings.HasPrefix(content, PersistentManagementRecoveryPrefix) {
		return out, fmt.Errorf("%w: unsupported identity", ErrInvalidPersistentManagementRecovery)
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(content, PersistentManagementRecoveryPrefix))
	if err != nil {
		return out, fmt.Errorf("%w: decode: %v", ErrInvalidPersistentManagementRecovery, err)
	}
	at := 0
	fields := make([]string, 7)
	for i := range fields {
		fields[i], err = readTextureOccurrenceField(raw, &at)
		if err != nil {
			return out, fmt.Errorf("%w: read scope: %v", ErrInvalidPersistentManagementRecovery, err)
		}
	}
	count64, err := strconv.ParseInt(fields[6], 10, 32)
	if err != nil || count64 <= 0 || count64 > 10000 || strconv.FormatInt(count64, 10) != fields[6] {
		return out, fmt.Errorf("%w: invalid control count", ErrInvalidPersistentManagementRecovery)
	}
	out = PersistentManagementRecoveryOccurrence{
		OwnerID: fields[0], ComputerID: fields[1], TrajectoryID: fields[2],
		AgentID: fields[3], RunID: fields[4], SourceAgentID: fields[5],
		Controls: make([]PersistentManagementRecoveryControl, int(count64)),
	}
	for i := range out.Controls {
		updateID, updateErr := readTextureOccurrenceField(raw, &at)
		if updateErr != nil {
			return PersistentManagementRecoveryOccurrence{}, fmt.Errorf("%w: read control: %v", ErrInvalidPersistentManagementRecovery, updateErr)
		}
		agentID, agentErr := readTextureOccurrenceField(raw, &at)
		if agentErr != nil {
			return PersistentManagementRecoveryOccurrence{}, fmt.Errorf("%w: read control agent: %v", ErrInvalidPersistentManagementRecovery, agentErr)
		}
		direction, directionErr := readTextureOccurrenceField(raw, &at)
		if directionErr != nil {
			return PersistentManagementRecoveryOccurrence{}, fmt.Errorf("%w: read control direction: %v", ErrInvalidPersistentManagementRecovery, directionErr)
		}
		versionRaw, versionErr := readTextureOccurrenceField(raw, &at)
		if versionErr != nil {
			return PersistentManagementRecoveryOccurrence{}, fmt.Errorf("%w: read lifecycle version: %v", ErrInvalidPersistentManagementRecovery, versionErr)
		}
		seqRaw, seqErr := readTextureOccurrenceField(raw, &at)
		if seqErr != nil {
			return PersistentManagementRecoveryOccurrence{}, fmt.Errorf("%w: read reducer sequence: %v", ErrInvalidPersistentManagementRecovery, seqErr)
		}
		version, versionErr := strconv.ParseInt(versionRaw, 10, 64)
		seq, seqErr := strconv.ParseInt(seqRaw, 10, 64)
		if strings.TrimSpace(updateID) == "" || strings.TrimSpace(agentID) != strings.TrimSpace(out.SourceAgentID) ||
			(direction != string(types.LifecyclePacketDirectionControl) && direction != string(types.LifecyclePacketDirectionProducerReport)) ||
			version <= 0 || seq <= 0 || versionErr != nil || seqErr != nil ||
			strconv.FormatInt(version, 10) != versionRaw || strconv.FormatInt(seq, 10) != seqRaw {
			return PersistentManagementRecoveryOccurrence{}, fmt.Errorf("%w: invalid control authority", ErrInvalidPersistentManagementRecovery)
		}
		out.Controls[i] = PersistentManagementRecoveryControl{
			UpdateID: strings.TrimSpace(updateID), AgentID: strings.TrimSpace(agentID),
			Direction: types.LifecyclePacketDirection(direction), LifecycleVersion: version, ReducerSeq: seq,
		}
	}
	if at != len(raw) {
		return PersistentManagementRecoveryOccurrence{}, fmt.Errorf("%w: trailing bytes", ErrInvalidPersistentManagementRecovery)
	}
	canonical, canonicalErr := EncodePersistentManagementRecovery(out)
	if canonicalErr != nil || canonical != content {
		return PersistentManagementRecoveryOccurrence{}, fmt.Errorf("%w: noncanonical encoding", ErrInvalidPersistentManagementRecovery)
	}
	return out, nil
}

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

// reconcilePersistentManagementActor is the durable controller boundary for the
// user's privileged execution actor. update_coagent can append addressed work
// for the persistent super, but only this runtime controller starts or reuses
// the super execution loop that drains those durable updates. Creation is
// serialized across every entry path by superReconcileMu: without it, an
// inbox-continuation reconcile can interleave with a selfdevStartMu-holding
// API start and both mint operation-bound Supers (multiple-runs defect).
func (rt *Runtime) reconcilePersistentManagementActor(ctx context.Context, ownerID, agentID string) (*types.RunRecord, error) {
	rt.managementReconcileMu.Lock()
	defer rt.managementReconcileMu.Unlock()
	return rt.reconcilePersistentManagementActorLocked(ctx, ownerID, agentID, "")
}

// ResumeInterruptedPersistentManagementControlRun re-enters the exact
// persistent Management control run that was passivated by a process restart.
// Boot is a recovery event, never a scheduler tick: it resumes the named run
// or returns without falling through to any backlog selection.
func (rt *Runtime) ResumeInterruptedPersistentManagementControlRun(ctx context.Context, ownerID, agentID string) (*types.RunRecord, bool, error) {
	rt.managementReconcileMu.Lock()
	defer rt.managementReconcileMu.Unlock()
	return rt.resumeInterruptedPersistentManagementControlRunLocked(ctx, ownerID, agentID)
}

func (rt *Runtime) resumeInterruptedPersistentManagementControlRunLocked(ctx context.Context, ownerID, agentID string) (*types.RunRecord, bool, error) {
	if ownerID == "" {
		return nil, false, fmt.Errorf("owner_id is required")
	}
	if agentID == "" {
		agentID = persistentManagementAgentID(ownerID)
	}
	if resident, found, err := rt.activeRunByAgent(ctx, ownerID, agentID); err != nil {
		return nil, false, fmt.Errorf("check resident super run: %w", err)
	} else if found {
		log.Printf("runtime: persistent-Management exact-run resume found resident run=%s owner=%s agent=%s", resident.RunID, ownerID, agentID)
		return &resident, false, nil
	}
	resumed, ok, err := rt.reactivateRestartedPersistentManagementControlRun(ctx, ownerID, agentID)
	if err != nil {
		return nil, false, err
	}
	if ok && resumed != nil {
		log.Printf("runtime: persistent-Management exact-run resume reactivated run=%s owner=%s agent=%s", resumed.RunID, ownerID, agentID)
		return resumed, true, nil
	}
	// Boot is a recovery event, never a scheduler tick. Return unconditionally
	// without falling through to any backlog selection.
	log.Printf("runtime: persistent-Management exact-run resume did not enter selection (resumed=false) owner=%s agent=%s", ownerID, agentID)
	return nil, false, nil
}

func (rt *Runtime) reconcilePersistentManagementActorLocked(ctx context.Context, ownerID, agentID string, exactUpdateID string) (*types.RunRecord, error) {
	if ownerID == "" {
		return nil, fmt.Errorf("owner_id is required")
	}
	if agentID == "" {
		agentID = persistentManagementAgentID(ownerID)
	}
	if resident, found, err := rt.activeRunByAgent(ctx, ownerID, agentID); err != nil {
		return nil, fmt.Errorf("check resident super run: %w", err)
	} else if found {
		if err := rt.persistentManagementResidentMatchesExact(&resident, exactUpdateID); err != nil {
			return nil, err
		}
		// The live occurrence binding to a pending resident is the only
		// observer a lost mint dispatch ever gets; arm the fresh-mint
		// watchdog so the strand cannot self-seal behind this bind.
		rt.armFreshMintManagementResumeWatchdog(&resident)
		return &resident, nil
	}
	if resumed, ok, err := rt.reactivateRestartedPersistentManagementControlRun(ctx, ownerID, agentID); err != nil {
		return nil, err
	} else if ok {
		if err := rt.persistentManagementResidentMatchesExact(resumed, exactUpdateID); err != nil {
			return nil, err
		}
		return resumed, nil
	}
	if active, err := rt.latestActiveRunByAgent(ctx, ownerID, agentID); err == nil {
		if active.State == types.RunBlocked {
			// A blocked Management is awaiting external input/approval; do not spawn
			// a competing Management or mint an unprompted Texture rewake.
			if err := rt.persistentManagementResidentMatchesExact(&active, exactUpdateID); err != nil {
				return nil, err
			}
			return &active, nil
		}
	} else if !errors.Is(err, store.ErrNotFound) {
		return nil, fmt.Errorf("check blocked super run: %w", err)
	}

	computerID := strings.TrimSpace(rt.TextureComputerID())
	// I26 scheduling contract: fail closed on assignments past their deadline
	// before selecting fresh work, so an expired holder releases the slot.
	// The stranded-frozen resume runs in the same gate: a Complete reduce
	// that failed mid-terminal-saga must release its slot through the fate
	// it already staged, not sit frozen until the deadline cancels it.
	rt.resumeStrandedFrozenAssignmentCommits(ctx)
	rt.enforceEngineeringAssignmentDeadlines(ctx)
	updates, err := rt.listPendingPersistentManagementLifecycleControls(ctx, ownerID, computerID, agentID, 100)
	if err != nil {
		return nil, err
	}
	lifecycleControls := len(updates) > 0
	if !lifecycleControls {
		updates, err = rt.listAndSettlePersistentManagementBacklog(ctx, ownerID, agentID)
		if err != nil {
			return nil, err
		}
		updates = filterPersistentManagementExecutionUpdates(updates)
	}
	if len(updates) == 0 {
		return nil, nil
	}

	first := updates[0]
	if exact := strings.TrimSpace(exactUpdateID); exact != "" {
		matched := false
		for _, update := range updates {
			if strings.TrimSpace(update.UpdateID) != exact {
				continue
			}
			targetWork := firstNonEmpty(update.TargetWorkItemID, update.WorkItemID)
			selected := selectLifecycleControlActivation(updates, update.TrajectoryID, map[string]bool{strings.TrimSpace(targetWork): true})
			if len(selected) == 0 {
				selected = []types.CoagentSourcePacket{update}
			}
			updates = selected
			first = selected[0]
			matched = true
			break
		}
		if !matched {
			return nil, fmt.Errorf("exact live Management control %s is not pending", exact)
		}
	}
	requestSource := "update_coagent"
	if lifecycleControls {
		requestSource = "lifecycle_texture_control"
	}
	metadata := map[string]any{
		runMetadataAgentProfile: agentprofile.Management,
		runMetadataAgentRole:    agentprofile.Management,
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
	if operationID := selfDevelopmentOperationIDFromPacketSources(first.Packet.Sources); operationID != "" {
		metadata["self_development_operation_id"] = operationID
	}
	targetWorkItemID := firstNonEmpty(first.TargetWorkItemID, first.WorkItemID)
	if targetWorkItemID != "" {
		metadata["lifecycle_work_item_id"] = targetWorkItemID
		metadata["work_item_ids"] = []string{targetWorkItemID}
	}
	if !lifecycleControls {
		metadata["worker_update_ids"] = []string{first.UpdateID}
	}

	prompt := persistentManagementCoagentInboxPrompt
	rec, err := rt.createRunWithMetadata(ctx, prompt, ownerID, metadata)
	if err != nil {
		return nil, err
	}
	if lifecycleControls {
		rec.TrajectoryID = ""
		delete(rec.Metadata, runMetadataTrajectoryID)
		if err := rt.store.UpdateRun(ctx, *rec); err != nil {
			rt.failUnactivatedLifecycleControlRun(ctx, rec, err)
			return nil, fmt.Errorf("preserve non-lifecycle persistent-Management run: %w", err)
		}
		targetControls := selectLifecycleControlActivation(updates, first.TrajectoryID, map[string]bool{targetWorkItemID: true})
		if len(targetControls) == 0 {
			targetControls = []types.CoagentSourcePacket{first}
		}
		if _, err := rt.bindLifecycleControlsToRun(ctx, rec, targetControls); err != nil {
			rt.failUnactivatedLifecycleControlRun(ctx, rec, err)
			return nil, err
		}
	}
	rt.activate(rec)
	return rec, nil
}

func (rt *Runtime) reactivateRestartedPersistentManagementControlRun(ctx context.Context, ownerID, agentID string) (*types.RunRecord, bool, error) {
	var runs []types.RunRecord
	var err error
	computerID := ""
	if rt != nil {
		computerID = strings.TrimSpace(rt.TextureComputerID())
	}
	if strings.TrimSpace(ownerID) != "" {
		runs, err = rt.store.ListPassivatedPersistentManagementControlRunsByOwner(ctx, ownerID, computerID, agentID, bootPersistentManagementRewarmLimit)
	} else {
		runs, err = rt.store.ListAllRunsByState(ctx, types.RunPassivated)
	}
	if err != nil {
		return nil, false, fmt.Errorf("list restarted persistent-Management runs: %w", err)
	}
	var pendingIDsByRun map[string][]string
	if strings.TrimSpace(ownerID) != "" && rt != nil && rt.store != nil {
		pendingIDsByRun, err = rt.store.PendingDeliveredWorkerUpdateCanonicalIDsByRun(ctx, ownerID, computerID)
		if err != nil {
			return nil, false, fmt.Errorf("index delivered worker updates: %w", err)
		}
		log.Printf("runtime: persistent-Management rewarm delivered-pending-runs=%d", len(pendingIDsByRun))
	}
	var candidate *types.RunRecord
	var candidateControls []types.CoagentSourcePacket
	for i := range runs {
		run := &runs[i]
		passivatedReason := metadataStringValue(run.Metadata, "passivated_reason")
		if run.OwnerID != ownerID || run.AgentID != agentID || !isPersistentManagementAgentRun(run) ||
			metadataStringValue(run.Metadata, "request_source") != "lifecycle_texture_control" ||
			(passivatedReason != "runtime_restarted" && passivatedReason != runtimeInjectionAppendFailurePassivationReason) {
			continue
		}
		if strings.TrimSpace(run.TrajectoryID) != "" {
			// Management delivery requires empty TrajectoryID. Listing packets for a
			// tombstone used to ReadObjectSnapshot the whole computer.
			log.Printf("runtime: skip restarted persistent-Management control run %s: non-empty trajectory_id", run.RunID)
			continue
		}
		if pendingIDsByRun != nil && len(pendingIDsByRun[run.RunID]) == 0 {
			log.Printf("runtime: persistent-Management rewarm packets run=%s pending=0", run.RunID)
			continue
		}
		log.Printf("runtime: persistent-Management rewarm validate run=%s", run.RunID)
		var controls []types.CoagentSourcePacket
		var readErr error
		if pendingIDsByRun != nil {
			controls, readErr = rt.pendingManagementRecoveryPacketsByCanonicalID(ctx, run, pendingIDsByRun[run.RunID])
		} else {
			controls, readErr = rt.listPendingLifecyclePacketsDeliveredToRun(ctx, run)
		}
		if readErr != nil {
			if errors.Is(readErr, store.ErrNotFound) || strings.Contains(readErr.Error(), "not found") {
				continue
			}
			if errors.Is(readErr, store.ErrLifecycleInvalidTransition) {
				// A Management run that cannot present its delivered controls (empty
				// TrajectoryID contract, stale assignment_trajectory_id, etc.)
				// is not reactivation authority. Keep scanning; aborting here
				// blocks minting a healthy Management and makes every boot work-item
				// trajectory repeat ListAllRunsByState until the guest OOMs.
				log.Printf("runtime: skip restarted persistent-Management control run %s: %v", run.RunID, readErr)
				continue
			}
			return nil, false, fmt.Errorf("validate restarted persistent-Management control run %s: %w", run.RunID, readErr)
		}
		if len(controls) == 0 {
			log.Printf("runtime: persistent-Management rewarm packets run=%s pending=0", run.RunID)
			continue
		}
		log.Printf("runtime: persistent-Management rewarm packets run=%s pending=%d", run.RunID, len(controls))
		// Passivated Management refs are already newest-first. The first run with
		// pending controls is the reactivation authority; later tombstones
		// must not each GetObject worker updates.
		copy := *run
		candidate = &copy
		candidateControls = append([]types.CoagentSourcePacket(nil), controls...)
		break
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
		return nil, false, fmt.Errorf("reactivate restarted persistent-Management control run %s: %w", candidate.RunID, err)
	}
	if err := rt.enqueuePersistentManagementRecoveryOccurrence(ctx, candidate, candidateControls); err != nil {
		return nil, false, fmt.Errorf("enqueue restarted persistent-Management recovery: %w", err)
	}
	rt.armReactivatedManagementResumeWatchdog(ownerID, candidate.RunID, candidate.UpdatedAt)
	return candidate, true, nil
}

// persistentManagementResumeDispatchDeadline bounds how long a reactivated persistent
// Management run may sit in pending without its recovery occurrence dispatching.
// Running runs self-terminate through the tool-loop iteration cap, and blocked
// runs are excluded from residency, so pending-never-dispatched is the only
// state that can jam the I26 singleton slot indefinitely (2026-09-03 fe92ea2b:
// reactivated at boot, never dispatched, blocked every live wake for hours).
const persistentManagementResumeDispatchDeadline = 10 * time.Minute

// resumeWatchdogFiredMetadata marks runs terminalized by the resume watchdog,
// distinguishing them from model-loop failures in forensics.
const resumeWatchdogFiredMetadata = "resume_watchdog_fired"

// reactivatedManagementResumeExpired is the pure hang predicate: a persistent Management
// run carrying the reactivation flag, still pending past the dispatch deadline.
// Scoped strictly to persistent Management runs — the Research injection-recovery
// path shares the flag and must never be failed here. Zero UpdatedAt counts as
// expired: with no freshness signal the fail-closed choice releases the slot.
func reactivatedManagementResumeExpired(rec *types.RunRecord, now time.Time) bool {
	if rec == nil || !isPersistentManagementAgentRun(rec) {
		return false
	}
	if rec.State != types.RunPending {
		return false
	}
	if !metadataBoolValue(rec.Metadata, "actor_reactivated_from_passivated") {
		return false
	}
	if rec.UpdatedAt.IsZero() {
		return true
	}
	return now.Sub(rec.UpdatedAt) > persistentManagementResumeDispatchDeadline
}

// failExpiredReactivatedManagementResume terminalizes a reactivated persistent Management
// run stuck in pending past the dispatch deadline, releasing the singleton
// slot. Fail-closed with a structured reason; delivered packets stay durable
// and a fresh live trigger can re-drive the work. Returns true when it failed
// a run. A recovery occurrence arriving after this fail resolves terminal and
// no-ops, so late dispatch is harmless in either order.
func (rt *Runtime) failExpiredReactivatedManagementResume(ctx context.Context, ownerID, runID string, now time.Time) (bool, error) {
	if rt == nil || rt.store == nil {
		return false, fmt.Errorf("resume watchdog: store unavailable")
	}
	rec, err := rt.store.GetRunByOwner(ctx, strings.TrimSpace(ownerID), strings.TrimSpace(runID))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return false, nil
		}
		return false, err
	}
	if !reactivatedManagementResumeExpired(&rec, now.UTC()) {
		return false, nil
	}
	finished := now.UTC()
	rec.State = types.RunFailed
	rec.Error = "persistent Management resume activation never dispatched within 10m; slot released, re-drive via live trigger"
	rec.FinishedAt = &finished
	rec.UpdatedAt = finished
	rec.Metadata = cloneMetadata(rec.Metadata)
	rec.Metadata[resumeWatchdogFiredMetadata] = true
	if err := rt.store.UpdateRun(ctx, rec); err != nil {
		return false, err
	}
	log.Printf("runtime: persistent-Management resume watchdog failed undispatched run=%s owner=%s agent=%s", rec.RunID, rec.OwnerID, rec.AgentID)
	return true, nil
}

// freshMintManagementResumeStranded is the fresh-mint hang predicate: a persistent
// Management run minted pending whose initial_dispatch never executed, still
// pending past the dispatch deadline. The reactivation watchdog covers only
// runs carrying the reactivation flag, so a fresh mint with a lost
// boot-window dispatch has no watchdog and no retry authority and the strand
// self-seals (2026-09-13 run 2454bdd4: minted 04:14:01Z during the boot
// rewarm window, pending 45 minutes, no dispatch log and no inference). Zero
// UpdatedAt counts as stranded: with no freshness signal the fail-closed
// choice releases the slot.
func freshMintManagementResumeStranded(rec *types.RunRecord, now time.Time) bool {
	if rec == nil || !isPersistentManagementAgentRun(rec) {
		return false
	}
	if rec.State != types.RunPending {
		return false
	}
	if metadataBoolValue(rec.Metadata, "actor_reactivated_from_passivated") {
		// The reactivation resume watchdog owns those runs.
		return false
	}
	if rec.UpdatedAt.IsZero() {
		return true
	}
	return now.Sub(rec.UpdatedAt) > persistentManagementResumeDispatchDeadline
}

// freshMintManagementResumeArming is the arm-time predicate: every freshly minted
// pending persistent Management run gets one watchdog; the deadline check happens
// again at fire time so an in-flight dispatch is never raced.
func freshMintManagementResumeArming(rec *types.RunRecord) bool {
	return rec != nil && isPersistentManagementAgentRun(rec) &&
		rec.State == types.RunPending &&
		!metadataBoolValue(rec.Metadata, "actor_reactivated_from_passivated")
}

// armFreshMintManagementResumeWatchdog bounds a freshly minted persistent Management
// run. Fire-and-forget, mirroring the reactivation resume watchdog, but it
// re-drives through a recovery occurrence instead of failing the run: a
// fresh mint whose initial_dispatch was lost has no live trigger queued
// behind it, so releasing the slot would strand the bound controls too. When
// the deadline passes and the run is still pending, a recovery occurrence
// re-drives it through the exact recovery branch; if the dispatch executed
// first the recovery occurrence resolves terminal and no-ops. A process
// death before firing is covered by boot passivation plus the rewarm.
func (rt *Runtime) armFreshMintManagementResumeWatchdog(rec *types.RunRecord) {
	if rt == nil || !freshMintManagementResumeArming(rec) {
		return
	}
	delay := persistentManagementResumeDispatchDeadline
	if !rec.UpdatedAt.IsZero() {
		if remaining := persistentManagementResumeDispatchDeadline - time.Since(rec.UpdatedAt); remaining > 0 {
			delay = remaining
		} else {
			delay = time.Second // already stranded: fire just off the current goroutine
		}
	}
	deadline := time.Now().UTC().Add(delay)
	// The durable not_before wake is the continuation authority under kernel
	// mode; the dispatcher fires HandleFreshMintManagementResumeDeadline.
	rt.scheduleContinuation(context.Background(), rec.OwnerID, rec.ComputerID, rec.AgentID,
		freshMintManagementDeadlineUpdateKind, rec.RunID, lifecycleControlTrajectoryForRun(rec), "", deadline)
}

// redriveStrandedFreshMintSuper re-drives one stranded fresh mint: re-read
// the run, confirm the strand predicate still holds, and enqueue a recovery
// occurrence bound to one of the run's still-pending delivered controls.
// Returns true when a recovery occurrence was enqueued.
func (rt *Runtime) redriveStrandedFreshMintManagement(ctx context.Context, ownerID, runID string) (bool, error) {
	if rt == nil || rt.store == nil {
		return false, fmt.Errorf("fresh-mint watchdog: store unavailable")
	}
	rec, err := rt.store.GetRunByOwner(ctx, ownerID, runID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return false, nil
		}
		return false, err
	}
	if !freshMintManagementResumeStranded(&rec, time.Now().UTC()) {
		return false, nil
	}
	packets, err := rt.listPendingLifecyclePacketsDeliveredToRun(ctx, &rec)
	if err != nil {
		return false, fmt.Errorf("fresh-mint watchdog list bound controls: %w", err)
	}
	if len(packets) == 0 {
		return false, nil
	}
	if err := rt.enqueuePersistentManagementRecoveryOccurrence(ctx, &rec, packets); err != nil {
		return false, fmt.Errorf("fresh-mint watchdog re-drive: %w", err)
	}
	return true, nil
}

// armReactivatedManagementResumeWatchdog starts the dispatch watchdog for a freshly
// reactivated run. Fire-and-forget: if the process dies first, the boot rewarm
// below re-arms or fails it, so no lost timer can jam the slot.
func (rt *Runtime) armReactivatedManagementResumeWatchdog(ownerID, runID string, armedAt time.Time) {
	ownerID, runID = strings.TrimSpace(ownerID), strings.TrimSpace(runID)
	if rt == nil || ownerID == "" || runID == "" {
		return
	}
	delay := persistentManagementResumeDispatchDeadline
	if !armedAt.IsZero() {
		if remaining := persistentManagementResumeDispatchDeadline - time.Since(armedAt); remaining > 0 {
			delay = remaining
		}
	}
	deadline := time.Now().UTC().Add(delay)
	rt.scheduleContinuation(context.Background(), ownerID, rt.TextureComputerID(), persistentManagementAgentID(ownerID),
		reactivatedManagementDeadlineUpdateKind, runID, "", "", deadline)
}

// errResumeWatchdogScanCap stops the boot rewarm walk once the scan budget is
// exhausted; it is a control-flow sentinel, not a failure.
var errResumeWatchdogScanCap = errors.New("resume watchdog scan cap reached")

// rewarmReactivatedManagementResumeWatchdogs re-arms the in-memory resume
// watchdog for every persistent Management run that was reactivated from a
// passivated state before the restart. The watchdog deadline is process-local,
// so a restart loses it; this re-arms (or fails an already-expired) timer so a
// lost resume deadline cannot jam the persistent Management slot.
func (rt *Runtime) rewarmReactivatedManagementResumeWatchdogs(ctx context.Context, ownerID, computerID string) {
	if rt == nil || rt.store == nil {
		return
	}
	ownerID, computerID = strings.TrimSpace(ownerID), strings.TrimSpace(computerID)
	if ownerID == "" || computerID == "" {
		return
	}
	now := time.Now().UTC()
	scanned := 0
	walkErr := rt.store.ForEachLifecycleRunsByState(ctx, ownerID, computerID, types.RunPending, func(rec types.RunRecord) error {
		if scanned >= bootPersistentManagementRewarmLimit {
			return errResumeWatchdogScanCap
		}
		scanned++
		if !isPersistentManagementAgentRun(&rec) ||
			!metadataBoolValue(rec.Metadata, "actor_reactivated_from_passivated") {
			return nil
		}
		if reactivatedManagementResumeExpired(&rec, now) {
			if _, err := rt.failExpiredReactivatedManagementResume(ctx, rec.OwnerID, rec.RunID, now); err != nil {
				log.Printf("runtime: boot resume-watchdog fail run %s: %v", rec.RunID, err)
			}
			return nil
		}
		rt.armReactivatedManagementResumeWatchdog(rec.OwnerID, rec.RunID, rec.UpdatedAt)
		return nil
	})
	if walkErr != nil && !errors.Is(walkErr, errResumeWatchdogScanCap) {
		log.Printf("runtime: boot resume-watchdog rewarm list: %v", walkErr)
	}
}

func (rt *Runtime) enqueuePersistentManagementRecoveryOccurrence(ctx context.Context, rec *types.RunRecord, packets []types.CoagentSourcePacket) error {
	if rt == nil || rt.store == nil || rt.dispatchActor == nil || rec == nil || len(packets) == 0 {
		return fmt.Errorf("persistent Management recovery occurrence dispatch unavailable")
	}
	trajectoryID := lifecycleControlTrajectoryForRun(rec)
	if trajectoryID == "" || !isPersistentManagementAgentRun(rec) {
		return fmt.Errorf("%w: recovery run scope is not persistent Management", ErrInvalidPersistentManagementRecovery)
	}
	if len(packets) == 0 {
		return fmt.Errorf("persistent Management recovery occurrence has no packets")
	}
	packet := packets[0]
	if packet.OwnerID != rec.OwnerID || packet.ComputerID != rec.ComputerID ||
		packet.TargetAgentID != rec.AgentID || packet.TrajectoryID != trajectoryID ||
		packet.AgentID == "" || packet.Direction == "" {
		return fmt.Errorf("%w: recovery packet scope mismatch", ErrInvalidPersistentManagementRecovery)
	}
	occurrence, err := EncodePersistentManagementRecovery(PersistentManagementRecoveryOccurrence{
		OwnerID: rec.OwnerID, ComputerID: rec.ComputerID, TrajectoryID: trajectoryID,
		AgentID: rec.AgentID, RunID: rec.RunID, SourceAgentID: packet.AgentID,
		Controls: []PersistentManagementRecoveryControl{{
			UpdateID: packet.UpdateID, AgentID: packet.AgentID, Direction: packet.Direction,
			LifecycleVersion: packet.LifecycleVersion, ReducerSeq: packet.ReducerSeq,
		}},
	})
	if err != nil {
		return err
	}
	log.Printf("runtime: persistent-Management recovery occurrence queued run=%s update=%s source=%s", rec.RunID, packet.UpdateID, packet.AgentID)
	if err := rt.dispatchActor(context.WithoutCancel(ctx), rec.OwnerID, rec.ComputerID, rec.AgentID,
		"coagent_result", occurrence, trajectoryID, packet.AgentID); err != nil {
		return fmt.Errorf("dispatch persistent Management recovery %s: %w", packet.UpdateID, err)
	}
	return nil
}

// ResolvePersistentManagementRecovery authenticates a distinct restart wake for the
// exact persistent Management run. It does not discover a replacement run: the
// occurrence names the run and the delivered packet authority it must resume.
func (rt *Runtime) ResolvePersistentManagementRecovery(ctx context.Context, ownerID, computerID, agentID, content, trajectoryID, fromAgentID string) (*types.RunRecord, bool, error) {
	occurrence, err := DecodePersistentManagementRecovery(content)
	if err != nil {
		return nil, false, err
	}
	if occurrence.OwnerID != strings.TrimSpace(ownerID) ||
		occurrence.ComputerID != strings.TrimSpace(computerID) ||
		occurrence.AgentID != strings.TrimSpace(agentID) ||
		occurrence.TrajectoryID != strings.TrimSpace(trajectoryID) ||
		occurrence.SourceAgentID != strings.TrimSpace(fromAgentID) ||
		occurrence.AgentID != persistentManagementAgentID(occurrence.OwnerID) {
		return nil, false, fmt.Errorf("%w: envelope mismatch", ErrInvalidPersistentManagementRecovery)
	}
	trajectory, err := rt.store.GetLifecycleTrajectory(ctx, occurrence.OwnerID, occurrence.ComputerID, occurrence.TrajectoryID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, false, fmt.Errorf("%w: trajectory is missing", ErrInvalidPersistentManagementRecovery)
		}
		return nil, false, fmt.Errorf("load persistent Management recovery trajectory: %w", err)
	}
	if trajectory.Status != types.TrajectoryLive {
		return nil, true, nil
	}
	if _, cancelErr := rt.store.GetLifecycleCancellationIntent(ctx, occurrence.OwnerID, occurrence.ComputerID, occurrence.TrajectoryID); cancelErr == nil {
		return nil, true, nil
	} else if !errors.Is(cancelErr, store.ErrNotFound) {
		return nil, false, fmt.Errorf("load persistent Management recovery cancellation intent: %w", cancelErr)
	}
	rec, err := rt.store.GetRunByOwner(ctx, occurrence.OwnerID, occurrence.RunID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, false, fmt.Errorf("%w: run is missing", ErrInvalidPersistentManagementRecovery)
		}
		return nil, false, fmt.Errorf("load persistent Management recovery run: %w", err)
	}
	if rec.State.Terminal() {
		return nil, true, nil
	}
	if rec.OwnerID != occurrence.OwnerID || rec.ComputerID != occurrence.ComputerID ||
		rec.AgentID != occurrence.AgentID || !isPersistentManagementAgentRun(&rec) ||
		rec.TrajectoryID != "" ||
		metadataStringValue(rec.Metadata, "assignment_trajectory_id") != occurrence.TrajectoryID {
		return nil, false, fmt.Errorf("%w: run authority mismatch", ErrInvalidPersistentManagementRecovery)
	}
	if rec.State != types.RunPending && rec.State != types.RunRunning {
		return nil, false, fmt.Errorf("%w: run is not executable", ErrInvalidPersistentManagementRecovery)
	}
	packets, err := rt.listPendingLifecyclePacketsDeliveredToRun(ctx, &rec)
	if err != nil {
		return nil, false, fmt.Errorf("load persistent Management recovery packets: %w", err)
	}
	if len(packets) == 0 {
		return nil, true, nil
	}
	for _, expected := range occurrence.Controls {
		matched := false
		for _, packet := range packets {
			if packet.UpdateID == expected.UpdateID && packet.AgentID == expected.AgentID &&
				packet.Direction == expected.Direction && packet.TargetAgentID == occurrence.AgentID &&
				packet.LifecycleVersion == expected.LifecycleVersion && packet.ReducerSeq == expected.ReducerSeq {
				matched = true
				break
			}
		}
		if !matched {
			return nil, false, fmt.Errorf("%w: packet authority changed", ErrInvalidPersistentManagementRecovery)
		}
	}
	return &rec, false, nil
}

func (rt *Runtime) markPersistentManagementRunUpdatesDelivered(ctx context.Context, rec *types.RunRecord) error {
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
	return rt.updateRunAndMarkSuccessfulCoagentActivationDeliveredWithEvent(ctx, rec, nil)
}

// updateRunAndMarkSuccessfulCoagentActivationDeliveredWithEvent persists the
// run's activation state and, when event is non-nil and the run is
// lifecycle-bound, folds the runtime event into the same atomic batch as the
// run projection. Worker-update delivery marks and work-item completion keep
// their existing post-write behavior.
func (rt *Runtime) updateRunAndMarkSuccessfulCoagentActivationDeliveredWithEvent(ctx context.Context, rec *types.RunRecord, event *types.EventRecord) error {
	if rec == nil {
		return nil
	}
	updateIDs := coagentUpdateIDsForRun(rec)
	if runHasProfile(rec, agentprofile.Texture) || metadataStringValue(rec.Metadata, "request_source") == "lifecycle_texture_control" {
		if err := rt.store.UpdateRunWithEvent(ctx, *rec, event); err != nil {
			return err
		}
		if metadataStringValue(rec.Metadata, "request_source") == "lifecycle_texture_control" {
			return nil
		}
		return rt.completeSuccessfulRunWorkItems(ctx, rec)
	}
	if len(updateIDs) == 0 || rec.State != types.RunCompleted {
		if err := rt.store.UpdateRunWithEvent(ctx, *rec, event); err != nil {
			return err
		}
		return rt.completeSuccessfulRunWorkItems(ctx, rec)
	}
	if err := rt.store.UpdateRunAndMarkWorkerUpdatesDelivered(ctx, *rec, rec.OwnerID, updateIDs); err != nil {
		return err
	}
	if event != nil {
		if err := rt.store.AppendEvent(ctx, event); err != nil {
			log.Printf("runtime: persist activation event %s: %v", event.EventID, err)
		}
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

func (rt *Runtime) maybeContinuePersistentManagementInbox(ctx context.Context, rec *types.RunRecord) {
	if !isPersistentManagementAgentRun(rec) || (rec.State != types.RunPassivated && !rec.State.Terminal()) {
		return
	}
	if isPersistentManagementInboxRun(rec) && rec.State == types.RunCompleted {
		if err := rt.markPersistentManagementRunUpdatesDelivered(ctx, rec); err != nil {
			log.Printf("runtime: mark persistent super updates delivered after %s: %v", rec.RunID, err)
			return
		}
	}
	// Terminal events wake Texture, never select backlog.
}

func isPersistentManagementInboxRun(rec *types.RunRecord) bool {
	if rec == nil {
		return false
	}
	if !isPersistentManagementAgentRun(rec) {
		return false
	}
	if metadataStringValue(rec.Metadata, "request_source") != "update_coagent" {
		return false
	}
	return true
}

func isPersistentManagementAgentRun(rec *types.RunRecord) bool {
	if rec == nil {
		return false
	}
	if agentProfileForRun(rec) != agentprofile.Management {
		return false
	}
	if strings.TrimSpace(rec.OwnerID) == "" || strings.TrimSpace(rec.AgentID) == "" {
		return false
	}
	return rec.AgentID == persistentManagementAgentID(rec.OwnerID)
}

func filterPersistentManagementExecutionUpdates(updates []types.CoagentSourcePacket) []types.CoagentSourcePacket {
	if len(updates) == 0 {
		return nil
	}
	out := make([]types.CoagentSourcePacket, 0, len(updates))
	for _, update := range updates {
		if persistentManagementExecutableUpdate(update) {
			out = append(out, update)
		}
	}
	return out
}

func (rt *Runtime) listAndSettlePersistentManagementBacklog(ctx context.Context, ownerID, agentID string) ([]types.CoagentSourcePacket, error) {
	const limit = 100
	for i := 0; i < 10; i++ {
		updates, err := rt.store.ListCoagentMailboxBacklog(ctx, ownerID, agentID, limit)
		if err != nil {
			return nil, fmt.Errorf("list super pending updates: %w", err)
		}
		settled, err := rt.settlePersistentManagementNonExecutionUpdates(ctx, ownerID, agentID, updates)
		if err != nil {
			return nil, fmt.Errorf("settle non-execution super updates: %w", err)
		}
		if !settled {
			return updates, nil
		}
	}
	return nil, fmt.Errorf("settle non-execution super updates: mailbox did not converge")
}

func (rt *Runtime) settlePersistentManagementNonExecutionUpdates(ctx context.Context, ownerID, agentID string, updates []types.CoagentSourcePacket) (bool, error) {
	var nonExecIDs []string
	for _, u := range updates {
		if u.DeliveredAt != nil || strings.TrimSpace(u.DeliveredToRunID) != "" {
			continue
		}
		if !persistentManagementExecutableUpdate(u) && !persistentManagementAdmissibleReport(u) {
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

func (rt *Runtime) pendingManagementRecoveryPacketsByCanonicalID(ctx context.Context, rec *types.RunRecord, canonicalIDs []string) ([]types.CoagentSourcePacket, error) {
	if rt == nil || rt.store == nil || rec == nil {
		return nil, nil
	}
	trajectoryID := lifecycleControlTrajectoryForRun(rec)
	if trajectoryID == "" {
		return nil, store.ErrLifecycleInvalidTransition
	}
	out := make([]types.CoagentSourcePacket, 0, 1)
	for _, canonicalID := range canonicalIDs {
		packet, err := rt.store.GetCoagentSourcePacket(ctx, canonicalID)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) || strings.Contains(err.Error(), "not found") {
				continue
			}
			return nil, err
		}
		if packet.DeliveredToRunID != rec.RunID || packet.Disposition != types.UpdatePending ||
			packet.OwnerID != rec.OwnerID || packet.ComputerID != rec.ComputerID ||
			packet.TargetAgentID != rec.AgentID || packet.TrajectoryID != trajectoryID ||
			packet.AgentID == "" || packet.Direction == "" {
			continue
		}
		out = append(out, packet)
		break
	}
	return out, nil
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
	if isPersistentManagementAgentRun(rec) {
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

func (rt *Runtime) listPendingPersistentManagementLifecycleControls(ctx context.Context, ownerID, computerID, agentID string, limit int) ([]types.CoagentSourcePacket, error) {
	ownerID, computerID, agentID = strings.TrimSpace(ownerID), strings.TrimSpace(computerID), strings.TrimSpace(agentID)
	if ownerID == "" || computerID == "" || agentID != persistentManagementAgentID(ownerID) {
		return nil, fmt.Errorf("list persistent-Management lifecycle controls: exact owner, computer, and persistent agent are required")
	}
	agent, err := rt.store.GetAgentByScope(ctx, ownerID, computerID, agentID)
	if err != nil {
		return nil, fmt.Errorf("load exact persistent Management: %w", err)
	}
	agentProfile, _ := agentprofile.Canonical(agent.Profile)
	agentRole, _ := agentprofile.Canonical(agent.Role)
	if agentProfile != agentprofile.Management || agentRole != agentprofile.Management || agent.LifecycleVersion != 0 || agent.OwnerID != ownerID || agent.ComputerID != computerID {
		return nil, fmt.Errorf("persistent Management lifecycle control target has invalid authority")
	}
	updates, err := rt.store.ListAllPendingLifecycleUpdates(ctx, ownerID, computerID, agentID)
	if err != nil {
		return nil, fmt.Errorf("list persistent-Management lifecycle controls: %w", err)
	}
	controls := make([]types.CoagentSourcePacket, 0, len(updates))
	for _, update := range updates {
		if update.Direction == types.LifecyclePacketDirectionControl {
			controls = append(controls, update)
		}
	}
	validated, valErr := rt.validateTargetBoundLifecycleControls(ctx, ownerID, computerID, agentID, controls, true)
	return validated, valErr
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
		if executionOnly && !persistentManagementExecutableUpdate(update) {
			return nil, fmt.Errorf("persistent-Management lifecycle control %q is not an execution request", update.UpdateID)
		}
		out = append(out, update)
	}
	return out, nil
}

func persistentManagementSenderAuthorized(update types.CoagentSourcePacket) bool {
	role, _ := agentprofile.Canonical(update.Role)
	return role == agentprofile.Texture &&
		update.Direction == types.LifecyclePacketDirectionControl
}

func persistentManagementAdmissibleReport(update types.CoagentSourcePacket) bool {
	role, _ := agentprofile.Canonical(update.Role)
	if role != agentprofile.Engineering ||
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

func persistentManagementExecutablePacket(update types.CoagentSourcePacket) bool {
	if !persistentManagementSenderAuthorized(update) {
		return false
	}
	packet := normalizeCoagentSourcePacketPayload(update.Packet)
	if packet.Kind != "execution_request" {
		return false
	}
	return validateCoagentSourcePacketPayload(packet) == nil
}

func persistentManagementExecutableUpdate(update types.CoagentSourcePacket) bool {
	if update.DeliveredAt != nil || strings.TrimSpace(update.DeliveredToRunID) != "" {
		return false
	}
	return persistentManagementExecutablePacket(update)
}

func persistentManagementMailboxInjectable(rec *types.RunRecord, update types.CoagentSourcePacket) bool {
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
		if update.Direction == types.LifecyclePacketDirectionControl {
			return update.DeliveredAt != nil && strings.TrimSpace(update.DeliveredToRunID) == strings.TrimSpace(rec.RunID)
		}
	}
	return update.DeliveredAt == nil || strings.TrimSpace(update.DeliveredToRunID) == strings.TrimSpace(rec.RunID)
}

func coagentUpdateDeliverableForRun(rec *types.RunRecord, update types.CoagentSourcePacket) bool {
	if rec == nil {
		return false
	}
	if isPersistentManagementAgentRun(rec) {
		if persistentManagementAdmissibleReport(update) || persistentManagementExecutablePacket(update) {
			return persistentManagementMailboxInjectable(rec, update)
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

func buildPersistentManagementUpdatePrompt(updates []types.CoagentSourcePacket) string {
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
	profile, _ := agentprofile.Canonical(firstNonEmpty(agent.Profile, agent.Role))
	lifecycleAgent := profile == agentprofile.Research && agent.LifecycleVersion > 0
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
	if profile == "" || profile == agentprofile.Email || profile == agentprofile.Conductor || profile == agentprofile.Management {
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
			return nil, fmt.Errorf("hydrate lifecycle Research controls: %w", err)
		}
		logicalKey, failedKey, versions, keyErr := lifecycleActivationKeys(ownerID, computerID, trajectoryID, agentID, buildinfo.Commit, updates, workByID)
		activationVersions = versions
		if keyErr != nil {
			return nil, fmt.Errorf("fingerprint lifecycle Research controls: %w", keyErr)
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
	if isPersistentManagementAgentRun(rec) {
		if metadataStringValue(rec.Metadata, "request_source") == "lifecycle_texture_control" {
			if _, err := rt.EnsurePersistentManagementAgent(ctx, rec.OwnerID); err != nil {
				return nil, fmt.Errorf("restore persistent Management agent: %w", err)
			}
			packets, err := rt.listPendingLifecyclePacketsDeliveredToRun(ctx, rec)
			if err != nil && errors.Is(err, store.ErrNotFound) {
				return nil, nil
			}
			trajectoryID := lifecycleControlTrajectoryForRun(rec)
			if trajectoryID != "" {
				pending, listErr := rt.store.ListAllPendingLifecycleUpdates(ctx, ownerID, computerID, agentID)
				if listErr == nil {
					omitClaimed := metadataBoolValue(rec.Metadata, runMetadataEngineeringReplacementOmitReports)
					claimed := map[string]bool{}
					if omitClaimed {
						for _, id := range metadataStringSlice(rec.Metadata[runMetadataProducerReportIDs]) {
							if id = strings.TrimSpace(id); id != "" {
								claimed[id] = true
							}
						}
					}
					for _, p := range pending {
						if p.TrajectoryID != trajectoryID || !persistentManagementAdmissibleReport(p) {
							continue
						}
						if omitClaimed && claimed[strings.TrimSpace(p.UpdateID)] {
							continue
						}
						packets = append(packets, p)
					}
				}
			}
			return packets, err
		}
		return rt.store.ListCoagentMailboxBacklog(ctx, ownerID, agentID, limit)
	}
	if lifecycleRun {
		if computerID == "" {
			return nil, fmt.Errorf("list pending lifecycle updates: computer_id is required")
		}
		if agentProfileForRun(rec) == agentprofile.Research {
			return rt.listPendingLifecyclePacketsDeliveredToRun(ctx, rec)
		}
		return rt.store.ListAllPendingLifecycleUpdates(ctx, ownerID, computerID, agentID)
	}
	return rt.store.ListCoagentMailboxBacklog(ctx, ownerID, agentID, limit)
}

// lifecycleOwnerRevisionTurnForRun injects the pending owner-authored head
// revision into the Texture run's context. The owner revision is the desk's
// input: the run consumes it by committing a texture turn against that head.
func (rt *Runtime) lifecycleOwnerRevisionTurnForRun(ctx context.Context, rec *types.RunRecord, phase string, seen map[string]bool) ([]json.RawMessage, []string, error) {
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
		return nil, nil, fmt.Errorf("owner revision lifecycle scope is unavailable")
	}
	head, _, pending := store.PendingTextureOwnerRevision(snapshot)
	if !pending {
		return nil, nil, nil
	}
	openWork := false
	for _, work := range snapshot.WorkItems {
		if work.Status == types.WorkItemOpen && work.OwnerID == ownerID && work.ComputerID == computerID && work.TrajectoryID == trajectoryID && work.AssignedAgentID == agentID {
			openWork = true
		}
	}
	if !openWork {
		return nil, nil, fmt.Errorf("owner revision %q has no open Texture work item", head.RevisionID)
	}
	if seen[head.RevisionID] {
		return nil, nil, nil
	}
	revision := map[string]any{
		"revision_id":        head.RevisionID,
		"doc_id":             head.DocID,
		"trajectory_id":      head.TrajectoryID,
		"author_kind":        string(head.AuthorKind),
		"author_label":       head.AuthorLabel,
		"parent_revision_id": head.ParentRevisionID,
		"created_at":         head.CreatedAt,
	}
	if len(head.Metadata) > 0 {
		var meta map[string]any
		if json.Unmarshal(head.Metadata, &meta) == nil && len(meta) > 0 {
			revision["metadata"] = meta
		}
	}
	payload, err := json.Marshal(map[string]any{"schema": lifecycleInjectionEnvelopeSchemaV1, "packet_type": "owner_revision", "owner_id": ownerID, "computer_id": computerID, "target_run_id": rec.RunID, "delivery_phase": phase, "document_id": docID, "trajectory_id": trajectoryID, "target_agent_id": agentID, "revisions": []map[string]any{revision}})
	if err != nil {
		return nil, nil, err
	}
	message, err := json.Marshal(map[string]any{"role": "user", "content": []map[string]string{{"type": "text", "text": "Choir authenticated owner revision packet.\n\n" + string(payload)}}})
	if err != nil {
		return nil, nil, err
	}
	return []json.RawMessage{message}, []string{head.RevisionID}, nil
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
			case strings.HasPrefix(text, "Choir authenticated owner revision packet.\n\n"):
				packetType = "owner_revision"
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
				Revisions []struct {
					RevisionID string `json:"revision_id"`
				} `json:"revisions"`
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
			case "owner_revision":
				for _, revision := range envelope.Revisions {
					if id := strings.TrimSpace(revision.RevisionID); id != "" {
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
		seenUpdates, seenOwnerRevisions := lifecycleInjectionIDsFromRunMemory(rec, entries)
		phase := coagentPacketDeliveryMid
		if finalCheckpoint {
			phase = coagentPacketDeliveryFinal
		} else if initialPhase != "" && len(seenUpdates) == 0 && len(seenOwnerRevisions) == 0 {
			// The first phase is durable-memory-derived rather than process-local:
			// an append failure retries the identical cold/thread projection, while
			// later arrivals after a successful append are mid-activation turns.
			phase = initialPhase
		}
		ownerMessages, _, err := rt.lifecycleOwnerRevisionTurnForRun(context.Background(), rec, phase, seenOwnerRevisions)
		if err != nil {
			return nil, fmt.Errorf("list pending owner revision turns: %w", err)
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
	// injection append, including Research and persistent Management cold starts.
	if requestSource == "lifecycle_texture_control" {
		return true
	}
	if agentProfileForRun(rec) != agentprofile.Texture {
		return false
	}
	if metadataStringValue(rec.Metadata, "request_intent") == "apply_owner_revision" {
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
	case agentprofile.Management, agentprofile.Engineering, agentprofile.Research, agentprofile.Texture:
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
	if agentProfileForRun(rec) == agentprofile.Research && len(lifecycleControlWorkIDsForRun(rec)) > 0 {
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
		scanProfile, _ := agentprofile.Canonical(run.AgentProfile)
		scanRole, _ := agentprofile.Canonical(run.AgentRole)
		if strings.TrimSpace(run.AgentID) != agentID || (run.State != types.RunPassivated && run.State != types.RunBlocked) ||
			scanProfile != agentprofile.Research || scanRole != agentprofile.Research ||
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
// Research run and appends any pending controls before the handler may
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
	agentProfile, _ := agentprofile.Canonical(agent.Profile)
	agentRole, _ := agentprofile.Canonical(agent.Role)
	if agent.OwnerID != ownerID || agent.ComputerID != computerID || agent.AgentID != agentID || agentProfile != agentprofile.Research || agentRole != agentprofile.Research || agent.LifecycleVersion <= 0 {
		return nil, store.ErrLifecycleInvalidTransition
	}
	rec, err := rt.store.GetLifecycleRun(ctx, ownerID, computerID, runID)
	if err != nil {
		return nil, err
	}
	runProfile, _ := agentprofile.Canonical(rec.AgentProfile)
	runRole, _ := agentprofile.Canonical(rec.AgentRole)
	if rec.OwnerID != ownerID || rec.ComputerID != computerID || rec.AgentID != agentID || runProfile != agentprofile.Research || runRole != agentprofile.Research ||
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
	if agentID == persistentManagementAgentID(ownerID) {
		return rt.reconcilePersistentManagementActor(ctx, ownerID, agentID)
	}
	return rt.reconcileUpdatedCoagentActor(ctx, ownerID, agentID)
}

func (rt *Runtime) persistentManagementResidentMatchesExact(rec *types.RunRecord, exactUpdateID string) error {
	exact := strings.TrimSpace(exactUpdateID)
	if rec == nil || exact == "" {
		return nil
	}
	for _, id := range metadataStringSlice(rec.Metadata["worker_update_ids"]) {
		if strings.TrimSpace(id) == exact {
			return nil
		}
	}
	for _, id := range persistentManagementBoundUpdateIDs(rec.Metadata["lifecycle_control_bindings"]) {
		if id == exact {
			return nil
		}
	}
	return fmt.Errorf("%w: persistent Management slot occupied by run %s", ErrActivationOccurrenceMustRemainUnprocessed, rec.RunID)
}

func persistentManagementBoundUpdateIDs(raw any) []string {
	appendID := func(dst []string, id string) []string {
		id = strings.TrimSpace(id)
		if id == "" {
			return dst
		}
		return append(dst, id)
	}
	switch value := raw.(type) {
	case []any:
		out := make([]string, 0, len(value))
		for _, rawEntry := range value {
			switch entry := rawEntry.(type) {
			case map[string]any:
				out = appendID(out, fmt.Sprint(entry["update_id"]))
			case map[string]string:
				out = appendID(out, entry["update_id"])
			}
		}
		return out
	case []map[string]any:
		out := make([]string, 0, len(value))
		for _, entry := range value {
			out = appendID(out, fmt.Sprint(entry["update_id"]))
		}
		return out
	case []map[string]string:
		out := make([]string, 0, len(value))
		for _, entry := range value {
			out = appendID(out, entry["update_id"])
		}
		return out
	default:
		return nil
	}
}

// ResolvePersistentManagementLiveOccurrence binds a hashed live Texture→Management wake
// to the exact pending control named by the occurrence. Generic
// ReconcileCoagentWake remains FIFO; this path is the live trigger.
func (rt *Runtime) ResolvePersistentManagementLiveOccurrence(ctx context.Context, ownerID, computerID, agentID, content, trajectoryID, fromAgentID string) (*types.RunRecord, bool, error) {
	if rt == nil || rt.store == nil {
		return nil, false, fmt.Errorf("runtime store unavailable")
	}
	ownerID, computerID, agentID = strings.TrimSpace(ownerID), strings.TrimSpace(computerID), strings.TrimSpace(agentID)
	content, trajectoryID, fromAgentID = strings.TrimSpace(content), strings.TrimSpace(trajectoryID), strings.TrimSpace(fromAgentID)
	if ownerID == "" || computerID == "" || agentID == "" || !strings.HasPrefix(content, "sha256:") {
		return nil, false, fmt.Errorf("%w: unsupported live Management occurrence", ErrInvalidPersistentManagementRecovery)
	}
	if agentID != persistentManagementAgentID(ownerID) {
		return nil, false, fmt.Errorf("%w: not persistent Management", ErrInvalidPersistentManagementRecovery)
	}
	if computerID != strings.TrimSpace(rt.TextureComputerID()) {
		return nil, false, fmt.Errorf("%w: computer mismatch", ErrInvalidPersistentManagementRecovery)
	}
	rt.managementReconcileMu.Lock()
	defer rt.managementReconcileMu.Unlock()
	pending, err := rt.listPendingPersistentManagementLifecycleControls(ctx, ownerID, computerID, agentID, 100)
	if err != nil {
		return nil, false, err
	}
	var matched types.CoagentSourcePacket
	found := false
	for _, update := range pending {
		if lifecycleControlActorOccurrenceContent(update) != content {
			continue
		}
		if trajectoryID != "" && strings.TrimSpace(update.TrajectoryID) != trajectoryID {
			continue
		}
		if fromAgentID != "" && strings.TrimSpace(update.AgentID) != fromAgentID {
			continue
		}
		if found {
			return nil, false, fmt.Errorf("%w: ambiguous live Management occurrence", ErrInvalidPersistentManagementRecovery)
		}
		matched = update
		found = true
	}
	if !found {
		return nil, true, nil
	}
	rec, err := rt.reconcilePersistentManagementActorLocked(ctx, ownerID, agentID, matched.UpdateID)
	if err != nil {
		return nil, false, err
	}
	if rec == nil {
		return nil, true, nil
	}
	return rec, false, nil
}

// LifecycleControlActorOccurrenceContent returns the deterministic actor-log
// occurrence identity for one canonical lifecycle packet. Adapter boot recovery
// uses the same authority when converging already durable occurrences.
func LifecycleControlActorOccurrenceContent(update types.CoagentSourcePacket) string {
	return lifecycleControlActorOccurrenceContent(update)
}

func lifecycleControlActorOccurrenceContent(update types.CoagentSourcePacket) string {
	return types.LifecycleControlActorOccurrenceContent(update)
}

func (rt *Runtime) wakeUpdatedCoagent(ctx context.Context, update types.CoagentSourcePacket) {
	if rt == nil || rt.store == nil {
		return
	}
	target := strings.TrimSpace(update.TargetAgentID)
	if target == "" {
		return
	}
	if target == persistentManagementAgentID(update.OwnerID) && update.Direction == types.LifecyclePacketDirectionProducerReport {
		return
	}
	if update.Direction == types.LifecyclePacketDirectionControl {
		if resident, found, err := rt.activeRunByAgent(ctx, update.OwnerID, target); err != nil {
			log.Printf("runtime: resolve resident lifecycle control target %s: %v", target, err)
			return
		} else if found {
			updates, listErr := rt.store.ListAllPendingLifecycleUpdates(ctx, update.OwnerID, update.ComputerID, target)
			if listErr == nil {
				updates, listErr = rt.validateTargetBoundLifecycleControls(ctx, update.OwnerID, update.ComputerID, target, updates, agentProfileForRun(&resident) == agentprofile.Management)
			}
			updates = selectLifecycleControlActivation(updates, resident.TrajectoryID, lifecycleControlWorkIDsForRun(&resident))
			if listErr != nil || len(updates) == 0 {
				// The live trigger's control does not bind to the resident run's
				// trajectory/work scope (or the pending list could not be read).
				// Never drop the wake: fall through to the actor dispatch so the
				// serialized actor boundary processes it when the slot frees —
				// binding when warm, reconciling when cold. FIFO backlog order
				// is preserved because selection still picks the lowest
				// pending arrival ordinal.
				log.Printf("runtime: resident lifecycle control bind deferred target=%s: %v (dispatching wake)", target, listErr)
			} else if _, bindErr := rt.bindLifecycleControlsToRun(ctx, &resident, updates); bindErr != nil {
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
	if err := rt.dispatchActor(ctx, update.OwnerID, firstNonEmpty(update.ComputerID, rt.TextureComputerID()), target, "coagent_result", lifecycleControlActorOccurrenceContent(update), update.TrajectoryID, update.AgentID); err != nil {
		log.Printf("runtime: actor wake coagent for update %s: %v", update.UpdateID, err)
	}
}
