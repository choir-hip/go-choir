package agentcore

// Boot-time state-repair passes. These are NOT obligation re-fire (the
// actor-wake outbox owns that) — they are state corrections with no canonical
// event to fold: passivating interrupted runs, minting spawned work items,
// re-entering persistent Management, and re-driving open work items. A process
// restart is not a canonical event, so the outbox cannot express these. They
// were deleted in 75e04196 under the claim that the outbox subsumed them; the
// consensus panel (agentic-consensus-20260924-171648) and the restart tests
// falsified that claim. Restored verbatim and run unconditionally at Start.
//
// See docs/problems/kernel-cutover-wake-gap-analysis-2026-09-24.md "second gap
// class" for the systematic treatment.

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

func (rt *Runtime) rewarmInterruptedLifecycleActivations(ctx context.Context) {
	if rt == nil || rt.store == nil {
		return
	}
	computerID := strings.TrimSpace(rt.TextureComputerID())
	if computerID == "" {
		return
	}
	for _, state := range []types.RunState{types.RunPending, types.RunRunning} {
		runs, err := rt.store.ListLifecycleRunsByState(ctx, "", computerID, state)
		if err != nil {
			log.Printf("runtime: boot lifecycle rewarm: query %s runs: %v", state, err)
			continue
		}
		for i := range runs {
			rec := &runs[i]
			eligible, eligibilityErr := rt.lifecycleActivationBindingsEligible(ctx, rec)
			if eligibilityErr != nil {
				log.Printf("runtime: boot lifecycle rewarm: validate run %s bindings: %v", rec.RunID, eligibilityErr)
				continue
			}
			if !eligible {
				if passivateErr := rt.passivateInterruptedLifecycleActivation(ctx, rec); passivateErr != nil {
					log.Printf("runtime: boot lifecycle rewarm: passivate stale run %s: %v", rec.RunID, passivateErr)
					continue
				}
				log.Printf("runtime: passivated stale lifecycle run %s before restart dispatch", rec.RunID)
				continue
			}
			if assignedEngineeringRun(rec) {
				if err := rt.ReconcileEngineeringAssignmentsForTrajectory(ctx, rec.OwnerID, rec.ComputerID, rec.TrajectoryID); err != nil {
					log.Printf("runtime: boot assigned Engineering reconcile run %s: %v", rec.RunID, err)
					continue
				}
				stored, loadErr := rt.getRunForComputer(ctx, rec.OwnerID, rec.RunID)
				if loadErr != nil {
					log.Printf("runtime: boot assigned Engineering reload run %s: %v", rec.RunID, loadErr)
					continue
				}
				*rec = stored
				if !rec.State.Active() {
					log.Printf("runtime: boot assigned Engineering run %s already terminal after reconcile", rec.RunID)
					continue
				}
				assignmentID := metadataStringValue(rec.Metadata, "assignment_id")
				attempt := uint64(metadataIntValue(rec.Metadata, "assignment_attempt"))
				assignment, assignErr := rt.store.GetEngineeringAssignment(ctx, rec.OwnerID, rec.ComputerID, assignmentID, attempt)
				if assignErr != nil || assignment.Disposition.Terminal() || !rt.assignedEngineeringCapsuleUsable(assignment) {
					if assignErr != nil {
						log.Printf("runtime: boot assigned Engineering lookup run %s: %v", rec.RunID, assignErr)
					} else {
						log.Printf("runtime: boot assigned Engineering run %s skipped wake; capsule is not restartable", rec.RunID)
					}
					continue
				}
			}
			lifecycleResearch, admitted, refusal, admissionErr := rt.admitLifecycleResearchProviderEntry(ctx, rec)
			if admissionErr != nil {
				if passivateErr := rt.passivateLifecycleResearchAfterAdmissionError(ctx, rec, admissionErr); passivateErr != nil {
					log.Printf("runtime: boot lifecycle rewarm: persist Research admission retry run %s: %v", rec.RunID, passivateErr)
				}
				log.Printf("runtime: boot lifecycle rewarm: Research admission run %s: %v", rec.RunID, admissionErr)
				continue
			}
			if lifecycleResearch && !admitted {
				if passivateErr := rt.passivateLifecycleResearchWithoutProviderAuthority(ctx, rec, refusal); passivateErr != nil {
					log.Printf("runtime: boot lifecycle rewarm: persist Research admission refusal run %s: %v", rec.RunID, passivateErr)
				}
				log.Printf("runtime: boot lifecycle Research run %s remains idle: %s", rec.RunID, refusal)
				continue
			}
			rt.activate(rec)
			rewarmProfile, _ := agentprofile.Canonical(rec.AgentProfile)
			if rewarmProfile == agentprofile.Research &&
				metadataStringValue(rec.Metadata, "request_source") == "lifecycle_texture_control" &&
				metadataStringValue(rec.Metadata, lifecycleLogicalActivationKeyMetadata) != "" {
				delivered, deliveryErr := rt.lifecycleRunHasCanonicalControlDelivery(ctx, rec)
				if deliveryErr != nil {
					log.Printf("runtime: boot lifecycle rewarm: validate canonical delivery for run %s: %v", rec.RunID, deliveryErr)
				} else if delivered {
					// Recover the crash window after bind commit and before occurrence
					// enqueue. Deterministic actor IDs make the complete replay safe.
					if occurrenceErr := rt.enqueueCanonicalLifecycleControlOccurrences(ctx, rec); occurrenceErr != nil {
						log.Printf("runtime: boot lifecycle rewarm: enqueue canonical occurrences for run %s: %v", rec.RunID, occurrenceErr)
					}
				}
			}
			log.Printf("runtime: re-dispatched lifecycle run %s (state=%s) after restart", rec.RunID, state)
		}
	}
	rt.reactivateRetryableLifecycleInjectionRuns(ctx, computerID)
}

func (rt *Runtime) rewarmInterruptedPersistentManagementActors(ctx context.Context) {
	if rt == nil || rt.store == nil {
		return
	}
	ownerID := strings.TrimSpace(rt.selfdevRouteOwnerID)
	if ownerID == "" {
		ownerID = strings.TrimSpace(os.Getenv("CHOIR_OWNER_ID"))
	}
	computerID := strings.TrimSpace(rt.TextureComputerID())
	var runs []types.RunRecord
	var err error
	if ownerID != "" {
		log.Printf("runtime: boot persistent-Management rewarm owner-scoped owner=%s computer=%s limit=%d", ownerID, computerID, bootPersistentManagementRewarmLimit)
		runs, err = rt.store.ListPassivatedPersistentManagementControlRunsByOwner(ctx, ownerID, computerID, "", bootPersistentManagementRewarmLimit)
		if err != nil {
			log.Printf("runtime: boot persistent-Management rewarm owner-scoped list: %v", err)
			return
		}
		log.Printf("runtime: boot persistent-Management rewarm owner-scoped candidates=%d", len(runs))
	} else {
		runs, err = rt.store.ListAllRunsByState(ctx, types.RunPassivated)
		if err != nil {
			log.Printf("runtime: boot persistent-Management rewarm: list passivated runs: %v", err)
			return
		}
	}
	seen := make(map[string]struct{})
	for _, run := range runs {
		rewarmRunProfile := agentProfileForRun(&run)
		rewarmRunRole, _ := agentprofile.Canonical(run.AgentRole)
		if rewarmRunProfile != agentprofile.Management ||
			rewarmRunRole != agentprofile.Management ||
			metadataStringValue(run.Metadata, "request_source") != "lifecycle_texture_control" {
			continue
		}
		reason := metadataStringValue(run.Metadata, "passivated_reason")
		if reason != "runtime_restarted" && reason != runtimeInjectionAppendFailurePassivationReason {
			continue
		}
		key := strings.TrimSpace(run.OwnerID) + "\x00" + strings.TrimSpace(run.AgentID)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		log.Printf("runtime: boot persistent-Management rewarm candidate run=%s owner=%s agent=%s", run.RunID, run.OwnerID, run.AgentID)
		if resumed, ok, resumeErr := rt.ResumeInterruptedPersistentManagementControlRun(ctx, run.OwnerID, run.AgentID); resumeErr != nil {
			log.Printf("runtime: boot persistent-Management rewarm run %s: %v", run.RunID, resumeErr)
		} else if ok && resumed != nil {
			log.Printf("runtime: boot persistent-Management rewarm dispatched run=%s", resumed.RunID)
		} else {
			log.Printf("runtime: boot persistent-Management rewarm candidate run %s did not resume", run.RunID)
		}
	}
	rt.rewarmReactivatedManagementResumeWatchdogs(ctx, ownerID, computerID)
}

func (rt *Runtime) reactivateRetryableLifecycleInjectionRuns(ctx context.Context, computerID string) {
	runs, err := rt.store.ListLifecycleRunsByState(ctx, "", computerID, types.RunPassivated)
	if err != nil {
		log.Printf("runtime: boot lifecycle injection recovery: query passivated runs: %v", err)
		return
	}
	for i := range runs {
		rec := &runs[i]
		passivatedReason := metadataStringValue(rec.Metadata, "passivated_reason")
		retryProfile, _ := agentprofile.Canonical(rec.AgentProfile)
		retryRole, _ := agentprofile.Canonical(rec.AgentRole)
		if retryProfile != agentprofile.Research ||
			retryRole != agentprofile.Research ||
			metadataStringValue(rec.Metadata, "request_source") != "lifecycle_texture_control" ||
			(passivatedReason != runtimeInjectionAppendFailurePassivationReason && passivatedReason != lifecycleResearchAdmissionRetryReason) {
			continue
		}
		trajectory, trajectoryErr := rt.store.GetLifecycleTrajectory(ctx, rec.OwnerID, rec.ComputerID, rec.TrajectoryID)
		if trajectoryErr != nil || trajectory.Status != types.TrajectoryLive {
			if trajectoryErr != nil {
				log.Printf("runtime: boot lifecycle injection recovery: read trajectory for run %s: %v", rec.RunID, trajectoryErr)
			}
			continue
		}
		eligible, eligibilityErr := rt.lifecycleActivationBindingsEligible(ctx, rec)
		if eligibilityErr != nil {
			log.Printf("runtime: boot lifecycle injection recovery: validate work for run %s: %v", rec.RunID, eligibilityErr)
			continue
		}
		if !eligible {
			continue
		}
		controls, controlsErr := rt.lifecycleResearchAdmissionRecoveryControls(ctx, rec)
		if controlsErr != nil {
			log.Printf("runtime: boot lifecycle injection recovery: validate exact deliveries for run %s: %v", rec.RunID, controlsErr)
			continue
		}
		if len(controls) == 0 {
			continue
		}
		agent, agentErr := rt.store.GetAgentByScope(ctx, rec.OwnerID, rec.ComputerID, rec.AgentID)
		recoveryAgentProfile, _ := agentprofile.Canonical(agent.Profile)
		recoveryAgentRole, _ := agentprofile.Canonical(agent.Role)
		if agentErr != nil || recoveryAgentProfile != agentprofile.Research ||
			recoveryAgentRole != agentprofile.Research ||
			(strings.TrimSpace(agent.ActiveRunID) != "" && strings.TrimSpace(agent.ActiveRunID) != rec.RunID) {
			if agentErr != nil {
				log.Printf("runtime: boot lifecycle injection recovery: validate actor for run %s: %v", rec.RunID, agentErr)
			}
			continue
		}
		// Actor delivery remains paused during boot. Both retry reasons require a
		// distinct exact-run occurrence: initial_dispatch may already be processed
		// across the MarkProcessed-before-SaveSnapshot crash cut.
		if recoveryErr := rt.enqueueLifecycleResearchAdmissionRecoveryOccurrence(ctx, rec, controls); recoveryErr != nil {
			log.Printf("runtime: enqueue lifecycle Research recovery run %s: %v", rec.RunID, recoveryErr)
			continue
		}
		rec.Metadata = cloneMetadata(rec.Metadata)
		rec.Metadata["actor_reactivate_existing_memory"] = true
		rec.Metadata["actor_reactivated_from_passivated"] = true
		rec.Metadata["passivated_reason"] = ""
		rec.State = types.RunPending
		rec.Error = ""
		rec.FinishedAt = nil
		rec.UpdatedAt = time.Now().UTC()
		if err := rt.store.UpdateRun(ctx, *rec); err != nil {
			log.Printf("runtime: boot lifecycle injection recovery: reactivate exact run %s: %v", rec.RunID, err)
			continue
		}
		log.Printf("runtime: reactivated exact lifecycle Research run %s through structured recovery occurrence (%s)", rec.RunID, passivatedReason)
		continue
	}
}

func (rt *Runtime) enqueueLifecycleResearchAdmissionRecoveryOccurrence(ctx context.Context, rec *types.RunRecord, controls []types.CoagentSourcePacket) error {
	if rt == nil || rt.dispatchActor == nil || rec == nil || len(controls) == 0 {
		return fmt.Errorf("lifecycle Research admission recovery dispatch unavailable")
	}
	o := LifecycleResearchAdmissionRecoveryOccurrence{
		OwnerID: rec.OwnerID, ComputerID: rec.ComputerID, TrajectoryID: rec.TrajectoryID,
		AgentID: rec.AgentID, RunID: rec.RunID,
		LogicalKey:    metadataStringValue(rec.Metadata, lifecycleLogicalActivationKeyMetadata),
		SourceAgentID: controls[0].AgentID,
		Controls:      make([]LifecycleResearchAdmissionRecoveryControl, 0, len(controls)),
	}
	for _, control := range controls {
		if control.AgentID != o.SourceAgentID {
			return fmt.Errorf("lifecycle Research admission recovery controls have multiple sources")
		}
		o.Controls = append(o.Controls, LifecycleResearchAdmissionRecoveryControl{UpdateID: control.UpdateID, LifecycleVersion: control.LifecycleVersion, ReducerSeq: control.ReducerSeq})
	}
	content, err := EncodeLifecycleResearchAdmissionRecovery(o)
	if err != nil {
		return err
	}
	return rt.dispatchActor(context.WithoutCancel(ctx), rec.OwnerID, rec.ComputerID, rec.AgentID, "coagent_result", content, rec.TrajectoryID, o.SourceAgentID)
}

func (rt *Runtime) sweepPendingUpdateActors(ctx context.Context, seen map[string]bool) {
	if rt == nil || rt.store == nil {
		return
	}
	updates, err := rt.store.ListCoagentMailboxBacklogAll(ctx, 0)
	if err != nil {
		log.Printf("runtime: boot update sweep: %v", err)
		return
	}
	if seen == nil {
		seen = map[string]bool{}
	}
	for _, update := range updates {
		ownerID := strings.TrimSpace(update.OwnerID)
		target := strings.TrimSpace(update.TargetAgentID)
		if ownerID == "" || target == "" {
			continue
		}
		key := ownerID + "\x00" + target
		if seen[key] {
			continue
		}
		seen[key] = true
		rt.wakeUpdatedCoagent(ctx, update)
	}
}

func (rt *Runtime) sweepOpenWorkItemActors(ctx context.Context) {
	if rt == nil || rt.store == nil {
		return
	}
	items, err := rt.store.ListOpenAssignedLifecycleWorkItems(ctx, rt.TextureComputerID(), 0)
	if err != nil {
		log.Printf("runtime: boot work-item sweep: %v", err)
		return
	}
	log.Printf("runtime: boot work-item sweep open_items=%d", len(items))
	grouped := map[string][]types.WorkItemRecord{}
	for _, item := range items {
		ownerID := strings.TrimSpace(item.OwnerID)
		agentID := strings.TrimSpace(item.AssignedAgentID)
		trajectoryID := strings.TrimSpace(item.TrajectoryID)
		if ownerID == "" || agentID == "" || trajectoryID == "" {
			continue
		}
		key := ownerID + "\x00" + agentID + "\x00" + trajectoryID
		grouped[key] = append(grouped[key], item)
	}
	managementSeen := map[string]struct{}{}
	for _, workItems := range grouped {
		first := workItems[0]
		var err error
		computerID := firstNonEmpty(first.ComputerID, rt.TextureComputerID())
		if intent, intentErr := rt.store.GetLifecycleCancellationIntent(ctx, first.OwnerID, computerID, first.TrajectoryID); intentErr == nil {
			_, _, err = rt.CancelTrajectoryCommand(ctx, first.TrajectoryID, first.OwnerID, intent.CommandID, intent.Reason, intent.RequestedLifecycleVersion, intent.ExpectedHeadRevisionID)
			if err != nil {
				log.Printf("runtime: boot cancellation-intent recovery owner=%s trajectory=%s: %v", first.OwnerID, first.TrajectoryID, err)
			}
			continue
		} else if !errors.Is(intentErr, store.ErrNotFound) {
			log.Printf("runtime: boot cancellation-intent lookup owner=%s trajectory=%s: %v", first.OwnerID, first.TrajectoryID, intentErr)
			continue
		}
		if strings.TrimSpace(first.AssignedAgentID) == persistentManagementAgentID(strings.TrimSpace(first.OwnerID)) {
			managementKey := strings.TrimSpace(first.OwnerID) + "\x00" + strings.TrimSpace(first.AssignedAgentID)
			if _, ok := managementSeen[managementKey]; ok {
				continue
			}
			managementSeen[managementKey] = struct{}{}
			// Boot is recovery, not a scheduler tick: backlog is durable and waits for live triggers.
			log.Printf("runtime: boot work-item sweep skipping persistent Management owner=%s agent=%s (boot does not schedule)", first.OwnerID, first.AssignedAgentID)
			continue
		} else if sweepAuthority, _ := agentprofile.Canonical(first.AuthorityProfile); sweepAuthority == agentprofile.Engineering {
			err = rt.ReconcileEngineeringAssignmentsForTrajectory(ctx, first.OwnerID,
				firstNonEmpty(first.ComputerID, rt.TextureComputerID()), first.TrajectoryID)
		} else if sweepAuthority, _ := agentprofile.Canonical(first.AuthorityProfile); sweepAuthority == agentprofile.Research {
			// Open lifecycle work is durable responsibility, not provider authority.
			// Only an exact pending Texture control enters the fingerprint reconciler;
			// otherwise the Research remains idle and inspectable.
			pendingControls, pendingErr := rt.store.ListAllPendingLifecycleUpdates(ctx, first.OwnerID, computerID, first.AssignedAgentID)
			if pendingErr != nil {
				err = pendingErr
			} else {
				log.Printf("runtime: boot work-item sweep researcher owner=%s agent=%s trajectory=%s pending=%d", first.OwnerID, first.AssignedAgentID, first.TrajectoryID, len(pendingControls))
			}
			if pendingErr == nil && len(pendingControls) > 0 {
				_, err = rt.reconcileUpdatedCoagentActor(ctx, first.OwnerID, first.AssignedAgentID)
				if errors.Is(err, ErrDurablyTerminalLifecycleControlActivation) {
					err = nil
				}
			}
		} else {
			_, err = rt.reconcileAssignedWorkItemActor(ctx, workItems)
		}
		if err != nil {
			log.Printf("runtime: boot work-item sweep owner=%s agent=%s trajectory=%s: %v",
				first.OwnerID, first.AssignedAgentID, first.TrajectoryID, err)
		}
	}
}

func (rt *Runtime) sweepPassivatedSpawnedCoagentWork(ctx context.Context) {
	if rt == nil || rt.store == nil {
		return
	}
	runs, err := rt.store.ListRunsByState(ctx, types.RunPassivated, 1000)
	if err != nil {
		log.Printf("runtime: boot passivated spawned-work sweep: %v", err)
		return
	}
	for i := range runs {
		rec := &runs[i]
		item, err := rt.ensureSpawnedCoagentWorkItem(ctx, rec, nil, "passivated_spawned_work_item_id")
		if err != nil {
			log.Printf("runtime: boot passivated spawned-work sweep run=%s: %v", rec.RunID, err)
			continue
		}
		if item.WorkItemID == "" || item.Status != types.WorkItemOpen {
			continue
		}
		sweptAuthority, _ := agentprofile.Canonical(firstNonEmpty(item.AuthorityProfile, agentProfileForRun(rec)))
		if item.LifecycleVersion > 0 && sweptAuthority == agentprofile.Research {
			computerID := firstNonEmpty(strings.TrimSpace(item.ComputerID), rt.TextureComputerID())
			pendingControls, pendingErr := rt.store.ListAllPendingLifecycleUpdates(ctx, item.OwnerID, computerID, item.AssignedAgentID)
			if pendingErr != nil {
				log.Printf("runtime: boot passivated Research control lookup run=%s work_item=%s: %v", rec.RunID, item.WorkItemID, pendingErr)
			} else if len(pendingControls) > 0 {
				if _, reconcileErr := rt.reconcileUpdatedCoagentActor(ctx, item.OwnerID, item.AssignedAgentID); reconcileErr != nil && !errors.Is(reconcileErr, ErrDurablyTerminalLifecycleControlActivation) {
					log.Printf("runtime: boot passivated Research control reconcile run=%s work_item=%s: %v", rec.RunID, item.WorkItemID, reconcileErr)
				}
			}
			continue
		}
		if err := rt.store.UpdateRun(ctx, *rec); err != nil {
			log.Printf("runtime: boot passivated spawned-work annotate run=%s work_item=%s: %v", rec.RunID, item.WorkItemID, err)
		}
		if _, err := rt.reconcileAssignedWorkItemActor(ctx, []types.WorkItemRecord{item}); err != nil {
			log.Printf("runtime: boot passivated spawned-work rewarm run=%s work_item=%s: %v", rec.RunID, item.WorkItemID, err)
		}
	}
}
