package textureowner

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentcore"
	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// scheduleTextureWorkerWake sends an actor message to the Texture agent for
// the given doc. The actor mailbox replaces the old debounce timer system —
// the handler processes the message when the actor activates, and the tool
// loop's park-resume handles coalescing naturally.
func (rt *Handler) scheduleTextureWorkerWake(ownerID, docID, instructionID string) {
	ownerID = strings.TrimSpace(ownerID)
	docID = strings.TrimSpace(docID)
	instructionID = strings.TrimSpace(instructionID)
	if ownerID == "" || docID == "" || instructionID == "" || rt == nil || rt.wakeOwnerInstruction == nil {
		return
	}
	if err := rt.wakeOwnerInstruction(context.Background(), ownerID, docID, instructionID); err != nil {
		log.Printf("runtime: schedule texture owner-instruction wake for doc %s: %v", docID, err)
	}
}

// Start reconciles durable Texture documents after the generic core has
// recovered interrupted activations and before the actor mailbox boot sweep.
// Exact pending trigger occurrences are durably dispatched before any run is
// projected pending, preventing an already-used initial_dispatch from becoming
// reactivation authority.
func (rt *Handler) Start(ctx context.Context) error {
	subjects, err := rt.Store.ListLifecycleSubjects(ctx, rt.Core.TextureSandboxID())
	if err != nil {
		return fmt.Errorf("reconcile lifecycle Texture subjects: %w", err)
	}
	textureSubjects := make([]types.AgentRecord, 0)
	for _, subject := range subjects {
		if agentprofile.Canonical(subject.Profile) != agentprofile.Texture {
			continue
		}
		docID := docIDFromTextureAgentID(subject.AgentID)
		doc, err := rt.Store.GetLifecycleDocument(ctx, subject.OwnerID, subject.ComputerID, docID)
		if err != nil {
			return fmt.Errorf("load boot Texture document %s: %w", docID, err)
		}
		activationEligible, err := rt.textureLifecycleActivationEligible(ctx, doc)
		if err != nil {
			return fmt.Errorf("classify boot Texture lifecycle %s: %w", subject.AgentID, err)
		}
		updates, err := rt.Store.ListAllPendingLifecycleUpdates(ctx, subject.OwnerID, subject.ComputerID, subject.AgentID)
		if err != nil {
			return fmt.Errorf("list boot Texture reports %s: %w", subject.AgentID, err)
		}
		instructions, err := rt.Store.ListPendingLifecycleOwnerInstructionsForHead(ctx, subject.OwnerID, subject.ComputerID, doc.TrajectoryID, subject.AgentID, "")
		if err != nil {
			return fmt.Errorf("list boot Texture instructions %s: %w", subject.AgentID, err)
		}

		var runID, tailID, mutationIdentity string
		candidateRunID := ""
		if activationEligible {
			candidateRunID = strings.TrimSpace(subject.ActiveRunID)
		}
		if activationEligible && candidateRunID == "" {
			memoryRunID, entries, memoryErr := rt.Store.LatestActorRunMemoryEntries(ctx, subject.OwnerID, subject.ComputerID, subject.AgentID, "")
			if memoryErr == nil {
				candidateRunID = memoryRunID
				if len(entries) > 0 {
					tailID = entries[len(entries)-1].EntryID
				}
			} else if !errors.Is(memoryErr, store.ErrNotFound) {
				return fmt.Errorf("load boot Texture actor memory: %w", memoryErr)
			}
		}
		if activationEligible && candidateRunID == "" {
			// A pre-repair passivated run can have neither ActiveRunID nor memory.
			// Enumerate only to prove a unique exact document/trajectory/mutation
			// join; list recency/order is never selection authority.
			runs, listErr := rt.Store.ListLifecycleRunsByChannel(ctx, subject.OwnerID, subject.ComputerID, docID, 0)
			if listErr != nil {
				return fmt.Errorf("list boot Texture recovery candidates: %w", listErr)
			}
			for i := range runs {
				candidate := runs[i]
				if candidate.AgentID != subject.AgentID || candidate.OwnerID != subject.OwnerID || candidate.SandboxID != subject.ComputerID || candidate.ChannelID != docID || candidate.TrajectoryID != doc.TrajectoryID || candidate.State.Terminal() || !isTextureAgentRevisionTaskType(metadataStringValue(candidate.Metadata, "type")) {
					continue
				}
				mutation, mutationErr := rt.Store.GetAgentMutationByRun(ctx, subject.OwnerID, subject.ComputerID, candidate.RunID)
				if mutationErr != nil {
					return fmt.Errorf("load boot Texture candidate mutation %s: %w", candidate.RunID, mutationErr)
				}
				if mutation == nil || mutation.RunID != candidate.RunID || mutation.DocID != docID || mutation.OwnerID != subject.OwnerID || mutation.ComputerID != subject.ComputerID {
					continue
				}
				if candidateRunID != "" {
					return fmt.Errorf("ambiguous boot Texture recovery run authority")
				}
				candidateRunID = candidate.RunID
			}
		}
		if activationEligible && candidateRunID != "" {
			run, runErr := rt.Store.GetLifecycleRun(ctx, subject.OwnerID, subject.ComputerID, candidateRunID)
			if runErr != nil {
				return fmt.Errorf("load boot Texture run %s: %w", candidateRunID, runErr)
			}
			if run.AgentID != subject.AgentID || run.OwnerID != subject.OwnerID || run.SandboxID != subject.ComputerID || run.ChannelID != docID || run.TrajectoryID != doc.TrajectoryID || run.State.Terminal() || !isTextureAgentRevisionTaskType(metadataStringValue(run.Metadata, "type")) {
				return fmt.Errorf("boot Texture run %s is not exact canonical authority", candidateRunID)
			}
			runID = run.RunID
			if tailID == "" {
				entries, memoryErr := rt.Store.ListRunMemoryEntries(ctx, subject.OwnerID, run.RunID)
				if memoryErr != nil {
					return fmt.Errorf("load boot Texture run memory %s: %w", run.RunID, memoryErr)
				}
				if len(entries) > 0 {
					tailID = entries[len(entries)-1].EntryID
				}
			}
			mutation, mutationErr := rt.Store.GetAgentMutationByRun(ctx, subject.OwnerID, subject.ComputerID, run.RunID)
			if mutationErr != nil {
				return fmt.Errorf("load boot Texture mutation %s: %w", run.RunID, mutationErr)
			}
			if mutation == nil || mutation.RunID != run.RunID || mutation.DocID != docID || mutation.OwnerID != subject.OwnerID || mutation.ComputerID != subject.ComputerID {
				return fmt.Errorf("boot Texture run %s lacks exact mutation join", candidateRunID)
			}
			mutationIdentity = fmt.Sprintf("%s:%d:%s", mutation.State, mutation.ScheduledMessageSeq, mutation.RevisionID)
		}
		dispatch := func(base agentcore.TextureActorOccurrence, source string) error {
			content, err := agentcore.EncodeTextureActorOccurrence(base)
			if err != nil {
				return err
			}
			if err := rt.Core.DispatchActor(ctx, base.OwnerID, base.ComputerID, base.TargetAgentID, "coagent_result", content, base.TrajectoryID, source); err != nil {
				return err
			}
			// Recovery is a deterministic join over the exact trigger and current
			// canonical run-memory/head/mutation state. It is harmless on fresh rows
			// and essential after MarkProcessed-before-snapshot cuts.
			if runID != "" {
				recovery := agentcore.TextureRecoveryOccurrence(base, runID, tailID, doc.CurrentRevisionID, mutationIdentity)
				recoveryContent, encodeErr := agentcore.EncodeTextureActorOccurrence(recovery)
				if encodeErr != nil {
					return encodeErr
				}
				if recoveryContent != content {
					if err := rt.Core.DispatchActor(ctx, base.OwnerID, base.ComputerID, base.TargetAgentID, "coagent_result", recoveryContent, base.TrajectoryID, source); err != nil {
						return err
					}
				}
			}
			return nil
		}
		for _, update := range updates {
			if update.Direction != types.LifecyclePacketDirectionProducerReport {
				continue
			}
			base, err := agentcore.TextureProducerReportOccurrence(update)
			if err != nil {
				return fmt.Errorf("build boot Texture report occurrence: %w", err)
			}
			if err := dispatch(base, base.ProducerAgentID); err != nil {
				return fmt.Errorf("dispatch boot Texture report occurrence: %w", err)
			}
		}
		for _, instruction := range instructions {
			base, err := agentcore.TextureOwnerInstructionOccurrence(instruction)
			if err != nil {
				return fmt.Errorf("build boot Texture owner occurrence: %w", err)
			}
			if err := dispatch(base, "owner:"+base.OwnerID); err != nil {
				return fmt.Errorf("dispatch boot Texture owner occurrence: %w", err)
			}
		}
		if activationEligible {
			textureSubjects = append(textureSubjects, subject)
		}
	}
	for _, subject := range textureSubjects {
		if _, err := rt.ReconcileActorWake(ctx, subject.OwnerID, subject.ComputerID, subject.AgentID); err != nil {
			return fmt.Errorf("reconcile subject %s/%s/%s: %w", subject.OwnerID, subject.ComputerID, subject.AgentID, err)
		}
	}
	return nil
}

// ReconcileActorWake resolves a Texture actor from canonical document state,
// persists its durable identity when first seen, and reconciles its mailbox.
// This path does not depend on a pre-existing generic agents row.
func (rt *Handler) ReconcileActorWake(ctx context.Context, ownerID, computerID, agentID string) (*types.RunRecord, error) {
	ownerID, computerID, agentID = strings.TrimSpace(ownerID), strings.TrimSpace(computerID), strings.TrimSpace(agentID)
	if rt == nil || rt.Store == nil || rt.Core == nil || ownerID == "" || computerID == "" || agentID == "" {
		return nil, fmt.Errorf("resolve Texture actor wake: incomplete scoped owner state")
	}
	docID := docIDFromTextureAgentID(agentID)
	if docID == "" {
		return nil, fmt.Errorf("resolve Texture actor wake: invalid Texture agent id")
	}
	doc, err := rt.getTextureDocument(ctx, ownerID, docID)
	if err != nil {
		return nil, fmt.Errorf("resolve Texture actor wake: document not found: %w", err)
	}
	if doc.ComputerID != computerID || strings.TrimSpace(doc.TrajectoryID) == "" {
		return nil, fmt.Errorf("resolve Texture actor wake: document lifecycle binding conflict")
	}
	if _, err := rt.Store.GetAgentByScope(ctx, ownerID, computerID, agentID); err != nil {
		return nil, fmt.Errorf("resolve Texture actor wake: durable subject unavailable: %w", err)
	}
	return rt.ReconcileAgentWake(ctx, ownerID, doc.DocID)
}

// ReconcileActorOccurrenceWake returns the exact run that the current
// unprocessed occurrence must execute, including a run already projected
// pending by a crash after mutation CAS/UpdateRun.
func (rt *Handler) ReconcileActorOccurrenceWake(ctx context.Context, ownerID, computerID, agentID, resolvedWorkID string, occurrence agentcore.TextureActorOccurrence) (*types.RunRecord, error) {
	ownerID, computerID, agentID, resolvedWorkID = strings.TrimSpace(ownerID), strings.TrimSpace(computerID), strings.TrimSpace(agentID), strings.TrimSpace(resolvedWorkID)
	docID := docIDFromTextureAgentID(agentID)
	if ownerID == "" || computerID == "" || docID == "" || resolvedWorkID == "" {
		return nil, invalidTextureOccurrence("exact Texture reconciliation scope is incomplete")
	}
	doc, err := rt.getTextureDocument(ctx, ownerID, docID)
	if err != nil {
		return nil, err
	}
	if doc.ComputerID != computerID || doc.TrajectoryID != occurrence.TrajectoryID {
		return nil, invalidTextureOccurrence("exact Texture reconciliation document mismatch")
	}
	unlockWake := rt.lockTextureWakeScope(ownerID, computerID, docID)
	defer unlockWake()
	doc, err = rt.getTextureDocument(ctx, ownerID, docID)
	if err != nil {
		return nil, err
	}
	if doc.ComputerID != computerID || doc.TrajectoryID != occurrence.TrajectoryID {
		return nil, invalidTextureOccurrence("exact Texture reconciliation document advanced")
	}
	if err := rt.validateOccurrenceReconciliationCandidates(ctx, doc, agentID, resolvedWorkID, occurrence); err != nil {
		return nil, err
	}
	rec, err := rt.reconcileAgentWakeLocked(ctx, doc, agentID)
	if err != nil {
		return nil, err
	}
	if rec == nil {
		active, found, activeErr := rt.Core.TextureActiveRunByAgent(ctx, ownerID, computerID, agentID)
		if activeErr != nil {
			return nil, activeErr
		}
		if !found {
			return nil, nil
		}
		rec = &active
	}
	if err := rt.ValidateOccurrenceActivationAuthority(ctx, occurrence, rec); err != nil {
		return nil, err
	}
	return rec, nil
}

// validateOccurrenceReconciliationCandidates is a read-only gate. It proves
// that generic reconciliation cannot stale/reactivate a foreign or ambiguous
// candidate before the exact occurrence/run join is known.
func (rt *Handler) validateOccurrenceReconciliationCandidates(ctx context.Context, doc types.Document, agentID, resolvedWorkID string, occurrence agentcore.TextureActorOccurrence) error {
	runs, err := rt.Store.ListLifecycleRunsByChannel(ctx, doc.OwnerID, doc.ComputerID, doc.DocID, 0)
	if err != nil {
		return err
	}
	eligible := 0
	for i := range runs {
		candidate := &runs[i]
		if candidate.State != types.RunPassivated || candidate.AgentID != agentID || candidate.TrajectoryID != doc.TrajectoryID ||
			!isTextureAgentRevisionTaskType(metadataStringValue(candidate.Metadata, "type")) || metadataStringValue(candidate.Metadata, "doc_id") != doc.DocID {
			continue
		}
		mutation, mutationErr := rt.Store.GetAgentMutationByRun(ctx, doc.OwnerID, doc.ComputerID, candidate.RunID)
		if mutationErr != nil {
			return mutationErr
		}
		if mutation == nil || mutation.DocID != doc.DocID || mutation.RunID != candidate.RunID || mutation.OwnerID != doc.OwnerID || mutation.ComputerID != doc.ComputerID ||
			(mutation.State != "pending" && mutation.State != "stale_activation" && mutation.State != "sleeping") {
			continue
		}
		head := strings.TrimSpace(mutation.RevisionID)
		if head == "" {
			head = metadataStringValue(candidate.Metadata, "current_revision_id")
		}
		if head != doc.CurrentRevisionID {
			continue
		}
		eligible++
		if metadataStringValue(candidate.Metadata, "lifecycle_work_item_id") != resolvedWorkID {
			return invalidTextureOccurrence("Texture occurrence does not authorize passivated candidate work")
		}
	}
	if eligible > 1 {
		return invalidTextureOccurrence("ambiguous passivated Texture run authority for document %s", doc.DocID)
	}
	active, found, activeErr := rt.Core.TextureActiveRunByAgent(ctx, doc.OwnerID, doc.ComputerID, agentID)
	if activeErr != nil {
		return activeErr
	}
	if found {
		if active.TrajectoryID != doc.TrajectoryID || metadataStringValue(active.Metadata, "lifecycle_work_item_id") != resolvedWorkID {
			return invalidTextureOccurrence("Texture occurrence does not authorize active candidate")
		}
	}
	if occurrence.TargetAgentID != agentID || occurrence.DocumentID != doc.DocID || occurrence.TrajectoryID != doc.TrajectoryID {
		return invalidTextureOccurrence("Texture occurrence reconciliation scope mismatch")
	}
	return nil
}

// ValidateActivationAuthority proves that an initial Texture dispatch is bound
// to the canonical document head and a pending scoped mutation.
func (rt *Handler) ValidateActivationAuthority(ctx context.Context, ownerID, computerID, agentID, runID string) error {
	ownerID, computerID, agentID, runID = strings.TrimSpace(ownerID), strings.TrimSpace(computerID), strings.TrimSpace(agentID), strings.TrimSpace(runID)
	docID := docIDFromTextureAgentID(agentID)
	if rt == nil || rt.Store == nil || ownerID == "" || computerID == "" || docID == "" || runID == "" {
		return fmt.Errorf("validate Texture activation: incomplete scoped authority")
	}
	doc, err := rt.getTextureDocument(ctx, ownerID, docID)
	if err != nil {
		return fmt.Errorf("validate Texture activation document: %w", err)
	}
	if strings.TrimSpace(doc.ComputerID) != computerID || strings.TrimSpace(doc.CurrentRevisionID) == "" || strings.TrimSpace(doc.TrajectoryID) == "" {
		return fmt.Errorf("validate Texture activation: document authority mismatch")
	}
	snapshot, err := rt.Store.GetLifecycleSnapshot(ctx, ownerID, computerID, doc.TrajectoryID)
	if err != nil || snapshot.Trajectory.Status != types.TrajectoryLive || snapshot.Document.CurrentRevisionID != doc.CurrentRevisionID {
		if err != nil {
			return fmt.Errorf("validate Texture activation trajectory: %w", err)
		}
		return fmt.Errorf("validate Texture activation: trajectory/head authority mismatch")
	}
	if _, cancelErr := rt.Store.GetLifecycleCancellationIntent(ctx, ownerID, computerID, doc.TrajectoryID); cancelErr == nil {
		return fmt.Errorf("validate Texture activation: cancellation intent exists")
	} else if !errors.Is(cancelErr, store.ErrNotFound) {
		return fmt.Errorf("validate Texture activation cancellation: %w", cancelErr)
	}
	revision, err := rt.getTextureRevision(ctx, ownerID, doc.CurrentRevisionID)
	if err != nil {
		return fmt.Errorf("validate Texture activation revision: %w", err)
	}
	if !textureRevisionMatchesDocument(revision, doc, ownerID) {
		return fmt.Errorf("validate Texture activation: revision authority mismatch")
	}
	run, err := rt.Store.GetLifecycleRun(ctx, ownerID, computerID, runID)
	if err != nil {
		return fmt.Errorf("validate Texture activation run: %w", err)
	}
	if strings.TrimSpace(run.OwnerID) != ownerID || strings.TrimSpace(run.SandboxID) != computerID ||
		strings.TrimSpace(run.TrajectoryID) != strings.TrimSpace(doc.TrajectoryID) ||
		strings.TrimSpace(run.AgentID) != agentID ||
		!isTextureAgentRevisionTaskType(metadataStringValue(run.Metadata, "type")) ||
		strings.TrimSpace(metadataStringValue(run.Metadata, "doc_id")) != docID ||
		strings.TrimSpace(metadataStringValue(run.Metadata, "current_revision_id")) != strings.TrimSpace(doc.CurrentRevisionID) {
		return fmt.Errorf("validate Texture activation: run authority mismatch")
	}
	agent, err := rt.Store.GetAgentByScope(ctx, ownerID, computerID, agentID)
	if err != nil || agentprofile.Canonical(agent.Profile) != agentprofile.Texture || agentprofile.Canonical(agent.Role) != agentprofile.Texture ||
		agent.ChannelID != docID || agent.LifecycleVersion <= 0 {
		if err != nil {
			return fmt.Errorf("validate Texture activation subject: %w", err)
		}
		return fmt.Errorf("validate Texture activation: subject authority mismatch")
	}
	workID := strings.TrimSpace(metadataStringValue(run.Metadata, "lifecycle_work_item_id"))
	work, err := rt.Store.GetLifecycleWorkItem(ctx, ownerID, computerID, workID)
	if err != nil || work.Status != types.WorkItemOpen || work.TrajectoryID != doc.TrajectoryID || work.AssignedAgentID != agentID || agentprofile.Canonical(work.AuthorityProfile) != agentprofile.Texture {
		if err != nil {
			return fmt.Errorf("validate Texture activation work: %w", err)
		}
		return fmt.Errorf("validate Texture activation: open work authority mismatch")
	}
	mutation, err := rt.Store.GetAgentMutationByRun(ctx, ownerID, computerID, runID)
	if err != nil {
		return fmt.Errorf("validate Texture activation mutation: %w", err)
	}
	if mutation == nil ||
		strings.TrimSpace(mutation.DocID) != docID ||
		strings.TrimSpace(mutation.RunID) != runID ||
		strings.TrimSpace(mutation.OwnerID) != ownerID ||
		strings.TrimSpace(mutation.ComputerID) != computerID ||
		(strings.TrimSpace(mutation.RevisionID) != "" &&
			strings.TrimSpace(mutation.RevisionID) != strings.TrimSpace(doc.CurrentRevisionID)) ||
		mutation.State != "pending" {
		return fmt.Errorf("validate Texture activation: mutation authority mismatch")
	}
	return nil
}

// ValidateOccurrenceActivationAuthority joins the exact canonical wake to the
// run and mutation selected for synchronous execution. Independent validity of
// two same-document objects is insufficient authority.
func (rt *Handler) ValidateOccurrenceActivationAuthority(ctx context.Context, o agentcore.TextureActorOccurrence, rec *types.RunRecord) error {
	if rec == nil || rec.RunID == "" || rec.OwnerID != o.OwnerID || rec.SandboxID != o.ComputerID || rec.TrajectoryID != o.TrajectoryID || rec.AgentID != o.TargetAgentID || rec.ChannelID != o.DocumentID {
		return invalidTextureOccurrence("Texture occurrence/run scope mismatch")
	}
	runWorkID := strings.TrimSpace(metadataStringValue(rec.Metadata, "lifecycle_work_item_id"))
	if runWorkID == "" || runWorkID != strings.TrimSpace(o.ResolvedTargetWorkItemID) {
		return invalidTextureOccurrence("Texture occurrence/run target work mismatch")
	}
	mutation, err := rt.Store.GetAgentMutationByRun(ctx, o.OwnerID, o.ComputerID, rec.RunID)
	if err != nil {
		return err
	}
	if mutation == nil || mutation.RunID != rec.RunID || mutation.DocID != o.DocumentID || mutation.OwnerID != o.OwnerID || mutation.ComputerID != o.ComputerID || mutation.State != "pending" {
		return invalidTextureOccurrence("Texture occurrence/run mutation mismatch")
	}
	latest, found, err := rt.latestEligibleWorkerMessage(ctx, o.OwnerID, o.DocumentID, 0)
	if err != nil {
		return err
	}
	if found && mutation.ScheduledMessageSeq != latest.Seq {
		return invalidTextureOccurrence("Texture occurrence mutation is not joined to latest canonical message sequence")
	}
	if o.Kind == agentcore.TextureActorOccurrenceProducerReport && (o.MessageSeq <= 0 || mutation.ScheduledMessageSeq < o.MessageSeq) {
		return invalidTextureOccurrence("Texture producer occurrence is newer than mutation schedule")
	}
	return nil
}

// ReconcileAgentWake starts or reuses a Texture activation when pending
// update_coagent records are addressed to texture:<docID>. Delivery uses the
// same typed coagent update packets as other actors; integrate intent only
// selects the Texture revision run shape.
func (rt *Handler) ReconcileAgentWake(ctx context.Context, ownerID, docID string) (*types.RunRecord, error) {
	ownerID = strings.TrimSpace(ownerID)
	docID = strings.TrimSpace(docID)
	if ownerID == "" || docID == "" {
		return nil, nil
	}
	textureAgentID := currentTextureAgentID(docID)
	doc, err := rt.getTextureDocument(ctx, ownerID, docID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("load doc for texture wake: %w", err)
	}
	if strings.TrimSpace(doc.ComputerID) == "" || strings.TrimSpace(doc.TrajectoryID) == "" {
		return nil, fmt.Errorf("texture wake requires durable lifecycle document binding")
	}
	wakeComputerID, wakeTrajectoryID := strings.TrimSpace(doc.ComputerID), strings.TrimSpace(doc.TrajectoryID)
	unlockWake := rt.lockTextureWakeScope(ownerID, wakeComputerID, docID)
	defer unlockWake()
	// The document may have advanced while this caller waited for its exact
	// owner/computer/document wake scope. Re-read before deriving activation
	// authority or current-head inputs.
	doc, err = rt.getTextureDocument(ctx, ownerID, docID)
	if err != nil {
		return nil, fmt.Errorf("reload doc for texture wake: %w", err)
	}
	if strings.TrimSpace(doc.ComputerID) != wakeComputerID || strings.TrimSpace(doc.TrajectoryID) != wakeTrajectoryID {
		return nil, fmt.Errorf("texture wake durable lifecycle document binding changed")
	}
	return rt.reconcileAgentWakeLocked(ctx, doc, textureAgentID)
}

func classifyTextureLifecycleActivationSnapshot(doc types.Document, snapshot types.LifecycleSnapshot) (bool, error) {
	ownerID := strings.TrimSpace(doc.OwnerID)
	computerID := strings.TrimSpace(doc.ComputerID)
	trajectoryID := strings.TrimSpace(doc.TrajectoryID)
	docID := strings.TrimSpace(doc.DocID)
	headID := strings.TrimSpace(doc.CurrentRevisionID)
	if ownerID == "" || computerID == "" || trajectoryID == "" || docID == "" || headID == "" {
		return false, fmt.Errorf("classify Texture lifecycle activation: incomplete document authority")
	}
	if snapshot.Trajectory.OwnerID != ownerID || snapshot.Trajectory.ComputerID != computerID || snapshot.Trajectory.TrajectoryID != trajectoryID ||
		snapshot.Document.OwnerID != ownerID || snapshot.Document.ComputerID != computerID || snapshot.Document.TrajectoryID != trajectoryID ||
		snapshot.Document.DocID != docID || snapshot.Document.CurrentRevisionID != headID || snapshot.HeadRevision.RevisionID != headID {
		return false, fmt.Errorf("classify Texture lifecycle activation: snapshot authority mismatch")
	}
	switch snapshot.Trajectory.Status {
	case types.TrajectoryCancelled, types.TrajectorySettled:
		return false, nil
	case types.TrajectoryLive:
		return true, nil
	default:
		return false, fmt.Errorf("classify Texture lifecycle activation: unknown trajectory status %q", snapshot.Trajectory.Status)
	}
}

func textureCancellationIntentPermitsActivation(cancelErr error) (bool, error) {
	if cancelErr == nil {
		return false, nil
	}
	if errors.Is(cancelErr, store.ErrNotFound) {
		return true, nil
	}
	return false, fmt.Errorf("load Texture lifecycle cancellation intent: %w", cancelErr)
}

func (rt *Handler) textureLifecycleActivationEligible(ctx context.Context, doc types.Document) (bool, error) {
	if rt == nil || rt.Store == nil {
		return false, fmt.Errorf("classify Texture lifecycle activation: Store unavailable")
	}
	ownerID := strings.TrimSpace(doc.OwnerID)
	computerID := strings.TrimSpace(doc.ComputerID)
	trajectoryID := strings.TrimSpace(doc.TrajectoryID)
	snapshot, err := rt.Store.GetLifecycleSnapshot(ctx, ownerID, computerID, trajectoryID)
	if err != nil {
		return false, fmt.Errorf("load Texture lifecycle activation snapshot: %w", err)
	}
	eligible, err := classifyTextureLifecycleActivationSnapshot(doc, snapshot)
	if err != nil || !eligible {
		return eligible, err
	}
	_, cancelErr := rt.Store.GetLifecycleCancellationIntent(ctx, ownerID, computerID, trajectoryID)
	return textureCancellationIntentPermitsActivation(cancelErr)
}

func (rt *Handler) reconcileAgentWakeLocked(ctx context.Context, doc types.Document, textureAgentID string) (*types.RunRecord, error) {
	ownerID := strings.TrimSpace(doc.OwnerID)
	docID := strings.TrimSpace(doc.DocID)
	activationEligible, err := rt.textureLifecycleActivationEligible(ctx, doc)
	if err != nil {
		return nil, err
	}
	if !activationEligible {
		return nil, nil
	}
	if _, err := rt.Store.GetAgentByScope(ctx, ownerID, doc.ComputerID, textureAgentID); err != nil {
		return nil, fmt.Errorf("load durable Texture subject: %w", err)
	}
	active, found, err := rt.Core.TextureActiveRunByAgent(ctx, ownerID, doc.ComputerID, textureAgentID)
	if err != nil {
		return nil, fmt.Errorf("check resident Texture loop: %w", err)
	}
	if found {
		if authorityErr := rt.ValidateActivationAuthority(ctx, ownerID, doc.ComputerID, textureAgentID, active.RunID); authorityErr == nil {
			return nil, nil
		}
		cleanupCtx := context.WithoutCancel(ctx)
		passivated := active
		passivated.State = types.RunPassivated
		passivated.Error = ""
		passivated.FinishedAt = nil
		passivated.UpdatedAt = time.Now().UTC()
		passivated.Metadata = cloneMetadata(passivated.Metadata)
		passivated.Metadata["passivated_reason"] = "invalid_texture_activation_authority"
		req := types.ReplaceLifecycleActivationRequest{
			OwnerID: ownerID, ComputerID: doc.ComputerID,
			CommandID:    "texture-owner-passivate-invalid:" + passivated.RunID,
			TrajectoryID: passivated.TrajectoryID, AgentID: passivated.AgentID, Run: passivated,
		}
		req.CommandDigest, _ = store.ComputeReplaceLifecycleActivationDigest(req)
		if _, replaceErr := rt.Store.ReplaceLifecycleActivation(cleanupCtx, req); replaceErr != nil {
			return nil, fmt.Errorf("passivate invalid active Texture run: %w", replaceErr)
		}
		mutation, mutationErr := rt.Store.GetAgentMutationByRun(cleanupCtx, ownerID, doc.ComputerID, passivated.RunID)
		if mutationErr != nil {
			return nil, fmt.Errorf("load invalid active Texture mutation: %w", mutationErr)
		}
		if mutation != nil && mutation.State == "pending" {
			if staleErr := rt.Store.MarkAgentMutationStale(cleanupCtx, ownerID, doc.ComputerID, passivated.RunID); staleErr != nil {
				return nil, fmt.Errorf("stale invalid active Texture mutation: %w", staleErr)
			}
		}
	}
	updates, err := rt.Store.ListPendingLifecycleUpdates(ctx, ownerID, doc.ComputerID, textureAgentID, 100)
	if err != nil {
		return nil, fmt.Errorf("list pending lifecycle Texture updates: %w", err)
	}
	instructions, err := rt.Store.ListPendingLifecycleOwnerInstructionsForHead(ctx, ownerID, doc.ComputerID, doc.TrajectoryID, textureAgentID, "")
	if err != nil {
		return nil, fmt.Errorf("list pending lifecycle owner instructions: %w", err)
	}
	initialWorkWake := false
	if len(updates) == 0 && len(instructions) == 0 {
		snapshot, snapshotErr := rt.Store.GetLifecycleSnapshot(ctx, ownerID, doc.ComputerID, doc.TrajectoryID)
		if snapshotErr != nil {
			return nil, fmt.Errorf("load initial lifecycle Texture work: %w", snapshotErr)
		}
		for _, work := range snapshot.WorkItems {
			if work.Status == types.WorkItemOpen && work.AssignedAgentID == textureAgentID {
				initialWorkWake = true
				break
			}
		}
		if initialWorkWake {
			runs, runsErr := rt.Store.ListLifecycleRunsByChannel(ctx, ownerID, doc.ComputerID, docID, 0)
			if runsErr != nil {
				return nil, fmt.Errorf("list initial lifecycle Texture runs: %w", runsErr)
			}
			for i := range runs {
				if strings.TrimSpace(runs[i].AgentID) == textureAgentID &&
					isTextureAgentRevisionTaskType(metadataStringValue(runs[i].Metadata, "type")) {
					initialWorkWake = false
					break
				}
			}
		}
	}
	var scheduledSeq int64
	for _, update := range updates {
		if update.MessageSeq > scheduledSeq {
			scheduledSeq = update.MessageSeq
		}
	}
	for _, instruction := range instructions {
		if instruction.ReducerSeq > scheduledSeq {
			scheduledSeq = instruction.ReducerSeq
		}
	}
	if rec, reactivated, err := rt.reactivatePassivatedTextureRun(ctx, doc, textureAgentID, scheduledSeq); err != nil {
		return nil, err
	} else if reactivated {
		return rec, nil
	}
	if len(updates) == 0 && len(instructions) == 0 && !initialWorkWake {
		return nil, nil
	}
	pendingCleanupCtx := context.WithoutCancel(ctx)
	for {
		mutation, mutationErr := rt.Store.GetPendingAgentMutationByDoc(pendingCleanupCtx, ownerID, doc.ComputerID, docID)
		if mutationErr != nil {
			return nil, fmt.Errorf("check pending doc mutation: %w", mutationErr)
		}
		if mutation == nil {
			break
		}
		if staleErr := rt.Store.MarkAgentMutationStale(pendingCleanupCtx, ownerID, doc.ComputerID, mutation.RunID); staleErr != nil {
			return nil, fmt.Errorf("stale unbound pending Texture mutation: %w", staleErr)
		}
	}
	intent := firstNonEmpty(func() string {
		if initialWorkWake {
			return "initial_owner_work"
		}
		if len(instructions) > 0 {
			return "apply_owner_instruction"
		}
		return ""
	}(), "integrate_execution_findings")
	rec, err := rt.submitTextureAgentRevisionRun(ctx, doc, ownerID, textureAgentRevisionRequest{
		Intent: intent,
	}, scheduledSeq)
	if err != nil {
		if errors.Is(err, errTextureLifecycleOpenWorkUnavailable) {
			stillEligible, authorityErr := rt.textureLifecycleActivationEligible(ctx, doc)
			if authorityErr != nil {
				return nil, fmt.Errorf("recheck Texture lifecycle activation after work refusal: %w", authorityErr)
			}
			if !stillEligible {
				return nil, nil
			}
		}
		return nil, fmt.Errorf("start reconciled Texture revision: %w", err)
	}
	return rec, nil

}

func (rt *Handler) reactivatePassivatedTextureRun(ctx context.Context, doc types.Document, textureAgentID string, scheduledSeq int64) (*types.RunRecord, bool, error) {
	if rt == nil || rt.Store == nil {
		return nil, false, nil
	}
	ownerID := strings.TrimSpace(doc.OwnerID)
	docID := strings.TrimSpace(doc.DocID)
	textureAgentID = strings.TrimSpace(textureAgentID)
	if ownerID == "" || docID == "" || textureAgentID == "" {
		return nil, false, nil
	}
	runs, err := rt.Store.ListLifecycleRunsByChannel(ctx, ownerID, doc.ComputerID, docID, 0)
	if err != nil {
		return nil, false, fmt.Errorf("list passivated Texture runs: %w", err)
	}
	var rec *types.RunRecord
	for i := range runs {
		candidate := &runs[i]
		if candidate.State != types.RunPassivated ||
			strings.TrimSpace(candidate.TrajectoryID) != strings.TrimSpace(doc.TrajectoryID) ||
			strings.TrimSpace(candidate.AgentID) != textureAgentID ||
			!isTextureAgentRevisionTaskType(metadataStringValue(candidate.Metadata, "type")) ||
			strings.TrimSpace(metadataStringValue(candidate.Metadata, "doc_id")) != docID {
			continue
		}
		mutation, mutationErr := rt.Store.GetAgentMutationByRun(ctx, ownerID, doc.ComputerID, candidate.RunID)
		if mutationErr != nil {
			return nil, false, fmt.Errorf("lookup passivated Texture mutation: %w", mutationErr)
		}
		if mutation == nil ||
			strings.TrimSpace(mutation.DocID) != docID ||
			strings.TrimSpace(mutation.RunID) != strings.TrimSpace(candidate.RunID) ||
			strings.TrimSpace(mutation.OwnerID) != ownerID ||
			strings.TrimSpace(mutation.ComputerID) != strings.TrimSpace(doc.ComputerID) {
			continue
		}
		documentRevisionID := strings.TrimSpace(doc.CurrentRevisionID)
		runRevisionID := strings.TrimSpace(metadataStringValue(candidate.Metadata, "current_revision_id"))
		mutationRevisionID := strings.TrimSpace(mutation.RevisionID)
		if documentRevisionID == "" {
			continue
		}
		if mutationRevisionID != "" {
			if mutationRevisionID != documentRevisionID {
				continue
			}
		} else if runRevisionID != documentRevisionID {
			continue
		}
		if mutation == nil || (mutation.State != "pending" && mutation.State != "stale_activation" && (mutation.State != "sleeping" || scheduledSeq <= 0)) {
			continue
		}
		if rec != nil {
			return nil, false, fmt.Errorf("ambiguous passivated Texture run authority for document %s", docID)
		}
		rec = candidate
	}
	if rec == nil {
		return nil, false, nil
	}
	selectedMutation, mutationErr := rt.Store.GetAgentMutationByRun(ctx, ownerID, doc.ComputerID, rec.RunID)
	if mutationErr != nil {
		return nil, false, fmt.Errorf("reload selected passivated Texture mutation: %w", mutationErr)
	}
	if selectedMutation != nil && selectedMutation.State == "pending" {
		if staleErr := rt.Store.MarkAgentMutationStale(ctx, ownerID, doc.ComputerID, rec.RunID); staleErr != nil {
			return nil, false, fmt.Errorf("repair selected passivated Texture mutation authority: %w", staleErr)
		}
	}
	rec.Metadata = cloneMetadata(rec.Metadata)
	rec.Metadata["request_source"] = "update_coagent"
	rec.Metadata["request_intent"] = "integrate_execution_findings"
	rec.Metadata["scheduled_message_seq"] = scheduledSeq
	rec.Metadata["actor_reactivate_existing_memory"] = true
	rec.Metadata["actor_reactivated_from_passivated"] = true
	rec.Metadata["actor_resume_source_loop_id"] = rec.RunID
	rec.Metadata["current_revision_id"] = strings.TrimSpace(doc.CurrentRevisionID)
	if spend, ok, err := rt.Core.LatestTextureActorToolLoopBudgetSpend(ctx, ownerID, textureAgentID); err != nil {
		return nil, false, fmt.Errorf("load passivated Texture budget spend: %w", err)
	} else if ok {
		rec.Metadata["actor_budget_spent_provider_calls"] = spend.ProviderCalls
		rec.Metadata["actor_budget_spent_input_tokens"] = spend.InputTokens
		rec.Metadata["actor_budget_spent_output_tokens"] = spend.OutputTokens
		if spend.SourceRunID != "" {
			rec.Metadata["actor_resume_source_loop_id"] = spend.SourceRunID
		}
	}
	rec.State = types.RunPending
	rec.Error = ""
	rec.Result = ""
	rec.FinishedAt = nil
	rec.UpdatedAt = time.Now().UTC()
	if err := rt.Store.ReactivateAgentMutation(ctx, ownerID, doc.ComputerID, rec.RunID, scheduledSeq); err != nil {
		if errors.Is(err, store.ErrMutationAlreadyCompleted) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("reactivate passivated Texture mutation: %w", err)
	}
	if err := rt.Store.UpdateRun(ctx, *rec); err != nil {
		if rollbackErr := rt.Store.MarkAgentMutationStale(context.WithoutCancel(ctx), ownerID, doc.ComputerID, rec.RunID); rollbackErr != nil {
			return nil, false, fmt.Errorf("reactivate passivated Texture run: %v; restore mutation authority: %w", err, rollbackErr)
		}
		return nil, false, fmt.Errorf("reactivate passivated Texture run: %w", err)
	}
	// The current coagent_result occurrence is the execution authority. Do not
	// redispatch the already-used one-shot initial_dispatch identity.
	return rec, true, nil
}

func (rt *Handler) latestEligibleWorkerMessage(ctx context.Context, ownerID, channelID string, afterSeq int64) (types.ChannelMessage, bool, error) {
	const batchSize = 200
	cache := make(map[string]bool)
	cursor := afterSeq
	var latest types.ChannelMessage
	found := false
	for {
		messages, err := rt.Store.ListChannelMessages(ctx, ownerID, channelID, cursor, batchSize)
		if err != nil {
			return types.ChannelMessage{}, false, err
		}
		if len(messages) == 0 {
			break
		}
		for _, message := range messages {
			if message.Seq > cursor {
				cursor = message.Seq
			}
			ok, err := rt.isEligibleWorkerMessage(ctx, ownerID, channelID, message, cache)
			if err != nil {
				return types.ChannelMessage{}, false, err
			}
			if !ok {
				continue
			}
			latest = message
			found = true
		}
		if len(messages) < batchSize {
			break
		}
	}
	return latest, found, nil
}

func (rt *Handler) isEligibleWorkerMessage(ctx context.Context, ownerID, docID string, message types.ChannelMessage, cache map[string]bool) (bool, error) {
	if strings.TrimSpace(message.ToAgentID) != "texture:"+strings.TrimSpace(docID) {
		return false, nil
	}
	runID := strings.TrimSpace(message.FromRunID)
	if runID == "" {
		return false, nil
	}
	if cached, ok := cache[runID]; ok {
		return cached, nil
	}
	run, err := rt.Core.GetRun(ctx, runID, ownerID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			cache[runID] = false
			return false, nil
		}
		return false, err
	}
	switch agentProfileForRun(run) {
	case agentprofile.Researcher, agentprofile.Super, agentprofile.CoSuper:
		cache[runID] = true
		return true, nil
	default:
		cache[runID] = false
		return false, nil
	}
}

// TextureActorOccurrenceState is the Store-owned fate of one exact actor wake.
type TextureActorOccurrenceState string

var ErrInvalidTextureActorOccurrence = errors.New("invalid Texture actor occurrence")

func invalidTextureOccurrence(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidTextureActorOccurrence, fmt.Sprintf(format, args...))
}

const (
	TextureActorOccurrencePending  TextureActorOccurrenceState = "pending"
	TextureActorOccurrenceTerminal TextureActorOccurrenceState = "terminal"
)

func producerOccurrenceScopeMatches(o agentcore.TextureActorOccurrence, update types.CoagentSourcePacket) bool {
	return o.Version == agentcore.TextureActorOccurrenceVersion && o.Kind == agentcore.TextureActorOccurrenceProducerReport &&
		o.OwnerID == strings.TrimSpace(update.OwnerID) && o.ComputerID == strings.TrimSpace(update.ComputerID) &&
		o.TrajectoryID == strings.TrimSpace(update.TrajectoryID) && o.DocumentID == strings.TrimSpace(update.ChannelID) &&
		o.TargetAgentID == strings.TrimSpace(update.TargetAgentID) && o.TargetWorkItemID == strings.TrimSpace(update.TargetWorkItemID) && o.ProducerAgentID == strings.TrimSpace(update.AgentID) &&
		o.UpdateID == strings.TrimSpace(update.UpdateID) && o.ProducerUpdateID == strings.TrimSpace(update.ProducerUpdateID) &&
		o.ProducerWorkID == strings.TrimSpace(firstNonEmpty(update.ProducerWorkItemID, update.WorkItemID)) && o.MessageSeq == update.MessageSeq
}

func producerOccurrenceMatches(o agentcore.TextureActorOccurrence, update types.CoagentSourcePacket) bool {
	return producerOccurrenceScopeMatches(o, update) && o.LifecycleVersion == update.LifecycleVersion && o.ReducerSeq == update.ReducerSeq
}

func ownerOccurrenceScopeMatches(o agentcore.TextureActorOccurrence, instruction types.LifecycleOwnerInstruction) bool {
	return o.Version == agentcore.TextureActorOccurrenceVersion && o.Kind == agentcore.TextureActorOccurrenceOwnerInstruction &&
		o.OwnerID == strings.TrimSpace(instruction.OwnerID) && o.ComputerID == strings.TrimSpace(instruction.ComputerID) &&
		o.TrajectoryID == strings.TrimSpace(instruction.TrajectoryID) && o.DocumentID == strings.TrimSpace(instruction.DocumentID) &&
		o.TargetAgentID == strings.TrimSpace(instruction.TargetAgentID) && o.TargetWorkItemID == strings.TrimSpace(instruction.TargetWorkItemID) &&
		o.InstructionID == strings.TrimSpace(instruction.InstructionID) && o.RequestID == strings.TrimSpace(instruction.RequestID) &&
		o.HeadRevisionID == strings.TrimSpace(instruction.HeadRevisionID) && o.InstructionKind == strings.TrimSpace(string(instruction.Kind))
}

func ownerOccurrenceMatches(o agentcore.TextureActorOccurrence, instruction types.LifecycleOwnerInstruction) bool {
	return ownerOccurrenceScopeMatches(o, instruction) && o.LifecycleVersion == instruction.LifecycleVersion && o.ReducerSeq == instruction.ReducerSeq
}

// ResolveTextureActorOccurrence reloads the exact Store trigger and joins it to
// live trajectory, head, Texture subject, open work, run, and mutation
// authority. A disposed/cancelled/late occurrence is a typed zero-provider
// terminal result; malformed or foreign identities are errors and stay
// retryable in the actor log.
func (rt *Handler) ResolveTextureActorOccurrence(ctx context.Context, ownerID, computerID, agentID, content string) (agentcore.TextureActorOccurrence, TextureActorOccurrenceState, error) {
	var zero agentcore.TextureActorOccurrence
	ownerID, computerID, agentID = strings.TrimSpace(ownerID), strings.TrimSpace(computerID), strings.TrimSpace(agentID)
	o, err := agentcore.DecodeTextureActorOccurrence(content)
	if err != nil {
		// Direct pre-repair tests and retained SQLite rows can carry update
		// content/digests. Resolve uniquely, then all later checks use the new
		// canonical tuple. Production dispatch persists only the encoded form.
		updates, listErr := rt.Store.ListAllPendingLifecycleUpdates(ctx, ownerID, computerID, agentID)
		if listErr != nil {
			return zero, "", listErr
		}
		for i := range updates {
			u := updates[i]
			if strings.TrimSpace(content) == strings.TrimSpace(u.Content) || strings.TrimSpace(content) == strings.TrimSpace(u.UpdateID) || strings.TrimSpace(content) == agentcore.LifecycleControlActorOccurrenceContent(u) {
				if o.Kind != "" {
					return zero, "", invalidTextureOccurrence("ambiguous legacy Texture occurrence")
				}
				o, err = agentcore.TextureProducerReportOccurrence(u)
				if err != nil {
					return zero, "", err
				}
			}
		}
		if o.Kind == "" {
			docID := docIDFromTextureAgentID(agentID)
			doc, docErr := rt.Store.GetLifecycleDocument(ctx, ownerID, computerID, docID)
			if docErr == nil {
				instruction, instructionErr := rt.Store.GetLifecycleOwnerInstruction(ctx, ownerID, computerID, doc.TrajectoryID, strings.TrimSpace(content))
				if instructionErr == nil {
					o, err = agentcore.TextureOwnerInstructionOccurrence(instruction)
				}
			}
		}
		if o.Kind == "" || err != nil {
			return zero, "", invalidTextureOccurrence("resolve exact Texture occurrence: %v", err)
		}
	}
	if o.OwnerID != ownerID || o.ComputerID != computerID || o.TargetAgentID != agentID || o.DocumentID != docIDFromTextureAgentID(agentID) {
		return zero, "", invalidTextureOccurrence("Texture occurrence envelope mismatch")
	}

	pending := false
	producerBindingID := ""
	switch o.Kind {
	case agentcore.TextureActorOccurrenceProducerReport:
		canonical, getErr := rt.Store.GetLifecycleUpdate(ctx, o.OwnerID, o.ComputerID, o.TrajectoryID, o.TargetAgentID, o.ProducerAgentID, o.ProducerUpdateID)
		if getErr != nil {
			if errors.Is(getErr, store.ErrNotFound) {
				return zero, "", invalidTextureOccurrence("exact Texture producer occurrence is missing")
			}
			return zero, "", fmt.Errorf("load exact Texture producer occurrence: %w", getErr)
		}
		if !producerOccurrenceMatches(o, canonical) {
			return zero, "", invalidTextureOccurrence("Texture producer occurrence canonical identity mismatch")
		}
		producerAgent, producerAgentErr := rt.Store.GetAgentByScope(ctx, o.OwnerID, o.ComputerID, o.ProducerAgentID)
		if producerAgentErr != nil {
			if errors.Is(producerAgentErr, store.ErrNotFound) {
				return zero, "", invalidTextureOccurrence("Texture producer agent is missing")
			}
			return zero, "", producerAgentErr
		}
		producerProfile := agentprofile.Canonical(producerAgent.Profile)
		if producerAgent.OwnerID != o.OwnerID || producerAgent.ComputerID != o.ComputerID || producerAgent.ChannelID != o.DocumentID ||
			(producerProfile != agentprofile.Researcher && producerProfile != agentprofile.Super && producerProfile != agentprofile.CoSuper) ||
			(producerProfile == agentprofile.Super && producerAgent.AgentID != persistentSuperAgentID(o.OwnerID)) ||
			(producerProfile != agentprofile.Super && producerAgent.LifecycleVersion <= 0) {
			return zero, "", invalidTextureOccurrence("Texture producer agent authority mismatch")
		}
		producerWork, producerWorkErr := rt.Store.GetLifecycleWorkItem(ctx, o.OwnerID, o.ComputerID, o.ProducerWorkID)
		if producerWorkErr != nil {
			if errors.Is(producerWorkErr, store.ErrNotFound) {
				return zero, "", invalidTextureOccurrence("Texture producer work is missing")
			}
			return zero, "", producerWorkErr
		}
		if producerWork.TrajectoryID != o.TrajectoryID || producerWork.AssignedAgentID != o.ProducerAgentID || producerWork.LifecycleVersion <= 0 || agentprofile.Canonical(producerWork.AuthorityProfile) != producerProfile {
			return zero, "", invalidTextureOccurrence("Texture producer work authority mismatch")
		}
		producerRun, producerRunErr := rt.Core.GetRun(ctx, canonical.SourceRunID, o.OwnerID)
		if producerRunErr != nil {
			if errors.Is(producerRunErr, store.ErrNotFound) {
				return zero, "", invalidTextureOccurrence("Texture producer source run is missing")
			}
			return zero, "", producerRunErr
		}
		trajectoryBound := producerRun.TrajectoryID == o.TrajectoryID
		if producerProfile == agentprofile.Super {
			trajectoryBound = producerRun.TrajectoryID == "" && metadataStringValue(producerRun.Metadata, "assignment_trajectory_id") == o.TrajectoryID
		}
		if producerRun.RunID != canonical.SourceRunID || producerRun.OwnerID != o.OwnerID || producerRun.SandboxID != o.ComputerID || producerRun.AgentID != o.ProducerAgentID || !trajectoryBound || producerRun.ChannelID != o.DocumentID || agentprofile.Canonical(producerRun.AgentProfile) != producerProfile || agentprofile.Canonical(producerRun.AgentRole) != producerProfile {
			return zero, "", invalidTextureOccurrence("Texture producer source run authority mismatch")
		}
		if authorityErr := rt.Core.ValidateLifecycleProducerReportAuthority(ctx, canonical); authorityErr != nil {
			if errors.Is(authorityErr, agentcore.ErrInvalidLifecycleProducerReportAuthority) {
				return zero, "", invalidTextureOccurrence("Texture producer control/run authority mismatch: %v", authorityErr)
			}
			return zero, "", fmt.Errorf("validate Texture producer control/run authority: %w", authorityErr)
		}
		producerBindingID = strings.TrimSpace(canonical.ControlBindingID)
		pending = canonical.Disposition == types.UpdatePending
	case agentcore.TextureActorOccurrenceOwnerInstruction:
		canonical, getErr := rt.Store.GetLifecycleOwnerInstruction(ctx, o.OwnerID, o.ComputerID, o.TrajectoryID, o.InstructionID)
		if getErr != nil {
			if errors.Is(getErr, store.ErrNotFound) {
				return zero, "", invalidTextureOccurrence("exact Texture owner occurrence is missing")
			}
			return zero, "", fmt.Errorf("load exact Texture owner occurrence: %w", getErr)
		}
		if !ownerOccurrenceMatches(o, canonical) {
			return zero, "", invalidTextureOccurrence("Texture owner occurrence canonical identity mismatch")
		}
		pending = canonical.Status == types.LifecycleOwnerInstructionPending
	default:
		return zero, "", invalidTextureOccurrence("unsupported Texture occurrence kind %q", o.Kind)
	}
	if !pending {
		return o, TextureActorOccurrenceTerminal, nil
	}

	snapshot, err := rt.Store.GetLifecycleSnapshot(ctx, o.OwnerID, o.ComputerID, o.TrajectoryID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return zero, "", invalidTextureOccurrence("Texture occurrence trajectory is missing")
		}
		return zero, "", fmt.Errorf("load Texture occurrence trajectory: %w", err)
	}
	if snapshot.Trajectory.Status != types.TrajectoryLive || snapshot.Trajectory.OwnerID != o.OwnerID || snapshot.Trajectory.ComputerID != o.ComputerID {
		return o, TextureActorOccurrenceTerminal, nil
	}
	if _, cancelErr := rt.Store.GetLifecycleCancellationIntent(ctx, o.OwnerID, o.ComputerID, o.TrajectoryID); cancelErr == nil {
		return o, TextureActorOccurrenceTerminal, nil
	} else if !errors.Is(cancelErr, store.ErrNotFound) {
		return zero, "", fmt.Errorf("load Texture cancellation intent: %w", cancelErr)
	}
	doc := snapshot.Document
	if doc.DocID != o.DocumentID || doc.OwnerID != o.OwnerID || doc.ComputerID != o.ComputerID || doc.TrajectoryID != o.TrajectoryID || strings.TrimSpace(doc.CurrentRevisionID) == "" {
		return zero, "", invalidTextureOccurrence("Texture occurrence document authority mismatch")
	}
	if o.Kind == agentcore.TextureActorOccurrenceProducerReport && producerBindingID != "" {
		bindingMatches := 0
		for _, update := range snapshot.Updates {
			if update.UpdateID == "" || update.UpdateID != producerBindingID {
				continue
			}
			if update.Direction == types.LifecyclePacketDirectionControl && update.TargetAgentID == o.ProducerAgentID && update.TargetWorkItemID == o.ProducerWorkID && update.TrajectoryID == o.TrajectoryID {
				bindingMatches++
			}
		}
		if bindingMatches != 1 {
			return zero, "", invalidTextureOccurrence("Texture producer control binding authority mismatch")
		}
	}
	if o.Kind == agentcore.TextureActorOccurrenceOwnerInstruction && doc.CurrentRevisionID != o.HeadRevisionID {
		return o, TextureActorOccurrenceTerminal, nil
	}
	agent, err := rt.Store.GetAgentByScope(ctx, o.OwnerID, o.ComputerID, o.TargetAgentID)
	if err != nil || agentprofile.Canonical(agent.Profile) != agentprofile.Texture || agentprofile.Canonical(agent.Role) != agentprofile.Texture || agent.ChannelID != o.DocumentID || agent.LifecycleVersion <= 0 {
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return zero, "", invalidTextureOccurrence("Texture occurrence subject is missing")
			}
			return zero, "", fmt.Errorf("load Texture occurrence subject: %w", err)
		}
		return zero, "", invalidTextureOccurrence("Texture occurrence subject authority mismatch")
	}
	workID := strings.TrimSpace(o.TargetWorkItemID)
	if workID == "" && strings.TrimSpace(agent.ActiveRunID) != "" {
		targetRun, runErr := rt.Store.GetLifecycleRun(ctx, o.OwnerID, o.ComputerID, agent.ActiveRunID)
		if runErr != nil {
			if errors.Is(runErr, store.ErrNotFound) {
				return zero, "", invalidTextureOccurrence("Texture occurrence active run is missing")
			}
			return zero, "", fmt.Errorf("load Texture occurrence active run: %w", runErr)
		}
		if targetRun.AgentID != o.TargetAgentID || targetRun.TrajectoryID != o.TrajectoryID {
			return zero, "", invalidTextureOccurrence("Texture occurrence active run authority mismatch")
		}
		workID = strings.TrimSpace(metadataStringValue(targetRun.Metadata, "lifecycle_work_item_id"))
	}
	if workID == "" {
		// Historical producer rows did not persist target_work_item_id. Migrate
		// only when the canonical snapshot has one unique open Texture work; an
		// ambiguous scope is not mailbox authority.
		for _, candidate := range snapshot.WorkItems {
			if candidate.Status != types.WorkItemOpen || candidate.AssignedAgentID != o.TargetAgentID || agentprofile.Canonical(candidate.AuthorityProfile) != agentprofile.Texture {
				continue
			}
			if workID != "" {
				return zero, "", invalidTextureOccurrence("ambiguous historical Texture target work identity")
			}
			workID = candidate.WorkItemID
		}
	}
	if workID == "" {
		return zero, "", invalidTextureOccurrence("Texture occurrence lacks exact target work identity")
	}
	work, err := rt.Store.GetLifecycleWorkItem(ctx, o.OwnerID, o.ComputerID, workID)
	if err != nil || work.Status != types.WorkItemOpen || work.TrajectoryID != o.TrajectoryID || work.AssignedAgentID != o.TargetAgentID || agentprofile.Canonical(work.AuthorityProfile) != agentprofile.Texture {
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return zero, "", invalidTextureOccurrence("Texture occurrence open work is missing")
			}
			return zero, "", fmt.Errorf("load Texture occurrence open work: %w", err)
		}
		return zero, "", invalidTextureOccurrence("Texture occurrence open work authority mismatch")
	}
	if o.RecoveryRunID != "" || o.RecoveryTailID != "" || o.RecoveryHeadID != "" || o.RecoveryMutation != "" {
		if o.RecoveryRunID == "" || o.RecoveryHeadID == "" || o.RecoveryMutation == "" {
			return zero, "", invalidTextureOccurrence("Texture recovery occurrence has incomplete joined state")
		}
		// A recovery row is authority only for the exact Store state from which
		// it was derived. An advanced head, memory tail, run, or mutation makes
		// this row a typed zero-provider stale outcome; the boot scan derives the
		// next exact identity behind it.
		if doc.CurrentRevisionID != o.RecoveryHeadID {
			return o, TextureActorOccurrenceTerminal, nil
		}
		recoveryRun, runErr := rt.Store.GetLifecycleRun(ctx, o.OwnerID, o.ComputerID, o.RecoveryRunID)
		if runErr != nil || recoveryRun.State.Terminal() || recoveryRun.AgentID != o.TargetAgentID || recoveryRun.TrajectoryID != o.TrajectoryID || recoveryRun.ChannelID != o.DocumentID {
			if runErr != nil && !errors.Is(runErr, store.ErrNotFound) {
				return zero, "", fmt.Errorf("load Texture recovery run: %w", runErr)
			}
			return o, TextureActorOccurrenceTerminal, nil
		}
		entries, memoryErr := rt.Store.ListRunMemoryEntries(ctx, o.OwnerID, o.RecoveryRunID)
		if memoryErr != nil {
			return zero, "", fmt.Errorf("load Texture recovery run memory: %w", memoryErr)
		}
		tailID := ""
		if len(entries) > 0 {
			tailID = entries[len(entries)-1].EntryID
		}
		if tailID != o.RecoveryTailID {
			return o, TextureActorOccurrenceTerminal, nil
		}
		mutation, mutationErr := rt.Store.GetAgentMutationByRun(ctx, o.OwnerID, o.ComputerID, o.RecoveryRunID)
		if mutationErr != nil {
			return zero, "", fmt.Errorf("load Texture recovery mutation: %w", mutationErr)
		}
		if mutation == nil || fmt.Sprintf("%s:%d:%s", mutation.State, mutation.ScheduledMessageSeq, mutation.RevisionID) != o.RecoveryMutation {
			return o, TextureActorOccurrenceTerminal, nil
		}
	}
	o.ResolvedTargetWorkItemID = workID
	return o, TextureActorOccurrencePending, nil
}

// TextureActorOccurrencePostcondition is checked after provider/tool execution.
// Model visibility alone is insufficient: the exact canonical trigger must no
// longer be pending before its actor row may be acknowledged.
func (rt *Handler) TextureActorOccurrencePostcondition(ctx context.Context, o agentcore.TextureActorOccurrence, runID string) (TextureActorOccurrenceState, error) {
	snapshot, err := rt.Store.GetLifecycleSnapshot(ctx, o.OwnerID, o.ComputerID, o.TrajectoryID)
	if err != nil {
		return "", err
	}
	if snapshot.Trajectory.Status != types.TrajectoryLive {
		return TextureActorOccurrenceTerminal, nil
	}
	if _, cancelErr := rt.Store.GetLifecycleCancellationIntent(ctx, o.OwnerID, o.ComputerID, o.TrajectoryID); cancelErr == nil {
		return TextureActorOccurrenceTerminal, nil
	} else if !errors.Is(cancelErr, store.ErrNotFound) {
		return "", cancelErr
	}
	mutation, err := rt.Store.GetAgentMutationByRun(ctx, o.OwnerID, o.ComputerID, strings.TrimSpace(runID))
	if err != nil {
		return "", err
	}
	if mutation == nil || mutation.RunID != strings.TrimSpace(runID) || mutation.DocID != o.DocumentID || mutation.ComputerID != o.ComputerID || strings.TrimSpace(mutation.RevisionID) == "" || snapshot.Document.CurrentRevisionID != mutation.RevisionID {
		return TextureActorOccurrencePending, nil
	}
	switch o.Kind {
	case agentcore.TextureActorOccurrenceProducerReport:
		canonical, err := rt.Store.GetLifecycleUpdate(ctx, o.OwnerID, o.ComputerID, o.TrajectoryID, o.TargetAgentID, o.ProducerAgentID, o.ProducerUpdateID)
		if err != nil {
			return "", err
		}
		if !producerOccurrenceScopeMatches(o, canonical) || canonical.LifecycleVersion <= o.LifecycleVersion || canonical.ReducerSeq <= o.ReducerSeq {
			return "", fmt.Errorf("Texture producer occurrence changed immutable identity or did not advance atomically")
		}
		if canonical.Disposition == types.UpdatePending || canonical.DispositionRef != mutation.RevisionID {
			return TextureActorOccurrencePending, nil
		}
		return TextureActorOccurrenceTerminal, nil
	case agentcore.TextureActorOccurrenceOwnerInstruction:
		canonical, err := rt.Store.GetLifecycleOwnerInstruction(ctx, o.OwnerID, o.ComputerID, o.TrajectoryID, o.InstructionID)
		if err != nil {
			return "", err
		}
		if !ownerOccurrenceScopeMatches(o, canonical) || canonical.LifecycleVersion <= o.LifecycleVersion || canonical.ReducerSeq <= o.ReducerSeq {
			return "", fmt.Errorf("Texture owner occurrence changed immutable identity or did not advance atomically")
		}
		if canonical.Status == types.LifecycleOwnerInstructionPending {
			return TextureActorOccurrencePending, nil
		}
		return TextureActorOccurrenceTerminal, nil
	default:
		return "", fmt.Errorf("unsupported Texture occurrence kind")
	}
}
