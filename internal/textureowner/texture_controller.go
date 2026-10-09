package textureowner

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentcore"
	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// dispatchTextureRevisionWake sends the owner-input occurrence to the Texture
// agent for the given doc: the committed owner-authored revision is the
// trigger. The actor mailbox dedupes on the encoded occurrence identity, and
// the boot scan re-derives the same wake from the document head, so a crash
// between commit and dispatch loses nothing.
func (rt *Handler) dispatchTextureRevisionWake(ownerID, computerID, trajectoryID string, revision types.Revision, requestID string, events []types.LifecycleEvent, deskProfile string) {
	if rt == nil || rt.Core == nil {
		return
	}
	reducerSeq := int64(0)
	for _, event := range events {
		if event.Kind == types.LifecycleArtifactHeadAdvanced &&
			len(event.ArtifactRefs) >= 2 && strings.TrimSpace(event.ArtifactRefs[1]) == revision.RevisionID {
			reducerSeq = event.ReducerSeq
		}
	}
	occurrence, err := agentcore.DocumentRevisionOccurrence(revision, deskProfile, requestID, reducerSeq)
	if err != nil {
		log.Printf("runtime: build document revision wake for doc %s: %v", revision.DocID, err)
		return
	}
	content, err := agentcore.EncodeTextureActorOccurrence(occurrence)
	if err != nil {
		log.Printf("runtime: encode document revision wake for doc %s: %v", revision.DocID, err)
		return
	}
	if err := rt.Core.DispatchActor(context.Background(), ownerID, computerID, occurrence.TargetAgentID, "coagent_result", content, trajectoryID, "owner:"+ownerID); err != nil {
		log.Printf("runtime: dispatch document revision wake for doc %s: %v", revision.DocID, err)
	}
}

// lifecycleDocDeskProfile derives the bound desk profile for one lifecycle
// document from its open desk work item. Returns "" when the document has no
// bound desk (unbound or non-desk trajectory).
func (rt *Handler) lifecycleDocDeskProfile(ctx context.Context, ownerID string, doc types.Document) string {
	if rt == nil || rt.Store == nil || strings.TrimSpace(doc.TrajectoryID) == "" {
		return ""
	}
	snapshot, err := rt.Store.GetLifecycleSnapshot(ctx, ownerID, doc.ComputerID, doc.TrajectoryID)
	if err != nil {
		return ""
	}
	textureWorkItemID := ""
	for _, work := range snapshot.WorkItems {
		if work.Status != types.WorkItemOpen {
			continue
		}
		// The supervision surface minted beside a document cast is a report
		// target, never a revision consumer — exclude it so a bound document's
		// executor desk resolves deterministically.
		if store.IsTextureSupervisionWorkItem(work.WorkItemID) {
			continue
		}
		profile := strings.TrimSpace(work.AuthorityProfile)
		if (profile != agentprofile.Texture && profile != agentprofile.Engineering) ||
			work.AssignedAgentID != profile+":"+doc.DocID {
			continue
		}
		if profile == agentprofile.Engineering {
			// The engineering desk is the document's executor when bound; an
			// owner revision occurrence must name it, never the supervision
			// subject that shares the document channel.
			return profile
		}
		textureWorkItemID = work.WorkItemID
	}
	if textureWorkItemID != "" {
		return agentprofile.Texture
	}
	return ""
}

// Start reconciles durable Texture documents after the generic core has
// recovered interrupted activations and before the actor mailbox boot sweep.
// Exact pending trigger occurrences are durably dispatched before any run is
// projected pending, preventing an already-used initial_dispatch from becoming
// reactivation authority.
func (rt *Handler) Start(ctx context.Context) error {
	if rt.Core != nil && rt.Core.MaintenanceHeld() {
		// The computer is under a maintenance hold: no run admission or Texture
		// reconcile dispatch is permitted while held. Defer the durable
		// reconcile to the next unheld boot; the guest stays up (health +
		// bootstrap route) but mutation-fenced. This converts the hold from a
		// fatal startup failure into a benign held state.
		log.Printf("textureowner: maintenance hold active; deferring Texture reconcile until unhold")
		return nil
	}
	subjects, err := rt.Store.ListLifecycleSubjects(ctx, rt.Core.TextureComputerID())
	if err != nil {
		return fmt.Errorf("reconcile lifecycle Texture subjects: %w", err)
	}
	for _, subject := range subjects {
		subjectProfile, _ := agentprofile.Canonical(subject.Profile)
		if subjectProfile == agentprofile.Engineering && subject.LifecycleVersion > 0 && subject.ChannelID != subject.AgentID {
			// Engineering desk: the desk agent's channel is the bound document,
			// not its own mailbox — that discriminates it from assigned agents
			// (engineering:{assignmentID}, channel = own mailbox). Reconcile
			// the pending cast directly; the desk agent never runs.
			docID := strings.TrimSpace(strings.TrimPrefix(subject.AgentID, agentprofile.Engineering+":"))
			if docID == "" || docID != strings.TrimSpace(subject.ChannelID) {
				continue
			}
			if _, reconcileErr := rt.Core.ReconcileEngineeringDesk(ctx, subject.OwnerID, docID); reconcileErr != nil {
				log.Printf("textureowner: boot engineering desk reconcile %s: %v", subject.AgentID, reconcileErr)
			}
			continue
		}
		// Texture subjects: nothing at boot. A restart is a system failure, not
		// a wake: interrupted runs stay passivated (runtime_restarted), pending
		// reports and owner revisions wait for the owner's next action, and no
		// turn starts or resumes on its own (owner rule, docs/problems/texture-
		// zombie-activations-revising-forever-2026-10-09.md).
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

// errAmbiguousPassivatedTextureAuthority is refused when passivated residue
// cannot be disambiguated by the canonical supersession order.
var errAmbiguousPassivatedTextureAuthority = errors.New("ambiguous passivated Texture run authority")

// passivatedTextureAuthority is one fully-validated passivated Texture
// recovery claim paired with its canonical supersession ordering fields.
type passivatedTextureAuthority struct {
	run               types.RunRecord
	sequence          int64
	mutationCreatedAt time.Time
}

// selectPassivatedTextureAuthority applies the passivated-residue supersession
// contract (docs/reviews/passivated-authority-structural-assessment-2026-08-28.md
// section 6): passivated multiplicity is crash-loop residue, not concurrent
// live authority; exactly one uniquely dominant candidate may be resumed while
// every loser stays passivated. Candidates are ordered by mutation
// ScheduledMessageSeq desc (activation freshness), then immutable mutation
// CreatedAt desc, then run CreatedAt desc, then run UpdatedAt desc. A
// full-tuple tie across different run IDs refuses.
func selectPassivatedTextureAuthority(candidates []passivatedTextureAuthority) (*types.RunRecord, error) {
	var best *passivatedTextureAuthority
	for i := range candidates {
		candidate := &candidates[i]
		if best == nil {
			best = candidate
			continue
		}
		switch comparePassivatedTextureAuthority(candidate, best) {
		case 1:
			best = candidate
		case 0:
			return nil, errAmbiguousPassivatedTextureAuthority
		}
	}
	if best == nil {
		return nil, nil
	}
	winner := best.run
	return &winner, nil
}

func comparePassivatedTextureAuthority(a, b *passivatedTextureAuthority) int {
	if a.sequence != b.sequence {
		if a.sequence > b.sequence {
			return 1
		}
		return -1
	}
	if !a.mutationCreatedAt.Equal(b.mutationCreatedAt) {
		if a.mutationCreatedAt.After(b.mutationCreatedAt) {
			return 1
		}
		return -1
	}
	if !a.run.CreatedAt.Equal(b.run.CreatedAt) {
		if a.run.CreatedAt.After(b.run.CreatedAt) {
			return 1
		}
		return -1
	}
	if !a.run.UpdatedAt.Equal(b.run.UpdatedAt) {
		if a.run.UpdatedAt.After(b.run.UpdatedAt) {
			return 1
		}
		return -1
	}
	return 0
}

// validateOccurrenceReconciliationCandidates is a read-only gate. It proves
// that generic reconciliation cannot stale/reactivate a foreign or ambiguous
// candidate before the exact occurrence/run join is known.
func (rt *Handler) validateOccurrenceReconciliationCandidates(ctx context.Context, doc types.Document, agentID, resolvedWorkID string, occurrence agentcore.TextureActorOccurrence) error {
	runs, err := rt.Store.ListLifecycleRunsByChannel(ctx, doc.OwnerID, doc.ComputerID, doc.DocID, 0)
	if err != nil {
		return err
	}
	var passivated []passivatedTextureAuthority
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
		if metadataStringValue(candidate.Metadata, "lifecycle_work_item_id") != resolvedWorkID {
			return invalidTextureOccurrence("Texture occurrence does not authorize passivated candidate work")
		}
		passivated = append(passivated, passivatedTextureAuthority{run: *candidate, sequence: mutation.ScheduledMessageSeq, mutationCreatedAt: mutation.CreatedAt})
	}
	if _, selectErr := selectPassivatedTextureAuthority(passivated); selectErr != nil {
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
	if strings.TrimSpace(run.OwnerID) != ownerID || strings.TrimSpace(run.ComputerID) != computerID ||
		strings.TrimSpace(run.TrajectoryID) != strings.TrimSpace(doc.TrajectoryID) ||
		strings.TrimSpace(run.AgentID) != agentID ||
		!isTextureAgentRevisionTaskType(metadataStringValue(run.Metadata, "type")) ||
		strings.TrimSpace(metadataStringValue(run.Metadata, "doc_id")) != docID ||
		strings.TrimSpace(metadataStringValue(run.Metadata, "current_revision_id")) != strings.TrimSpace(doc.CurrentRevisionID) {
		return fmt.Errorf("validate Texture activation: run authority mismatch")
	}
	agent, err := rt.Store.GetAgentByScope(ctx, ownerID, computerID, agentID)
	agentProfile, _ := agentprofile.Canonical(agent.Profile)
	agentRole, _ := agentprofile.Canonical(agent.Role)
	if err != nil || agentProfile != agentprofile.Texture || agentRole != agentprofile.Texture ||
		agent.ChannelID != docID || agent.LifecycleVersion <= 0 {
		if err != nil {
			return fmt.Errorf("validate Texture activation subject: %w", err)
		}
		return fmt.Errorf("validate Texture activation: subject authority mismatch")
	}
	workID := strings.TrimSpace(metadataStringValue(run.Metadata, "lifecycle_work_item_id"))
	work, err := rt.Store.GetLifecycleWorkItem(ctx, ownerID, computerID, workID)
	workAuthorityProfile, _ := agentprofile.Canonical(work.AuthorityProfile)
	if err != nil || work.Status != types.WorkItemOpen || work.TrajectoryID != doc.TrajectoryID || work.AssignedAgentID != agentID || workAuthorityProfile != agentprofile.Texture {
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
	if rec == nil || rec.RunID == "" || rec.OwnerID != o.OwnerID || rec.ComputerID != o.ComputerID || rec.TrajectoryID != o.TrajectoryID || rec.AgentID != o.TargetAgentID || rec.ChannelID != o.DocumentID {
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
	snapshot, err := rt.Store.GetLifecycleHeadView(ctx, ownerID, computerID, trajectoryID)
	if err != nil {
		return false, fmt.Errorf("load Texture lifecycle activation head: %w", err)
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
	snapshot, snapshotErr := rt.Store.GetLifecycleSnapshot(ctx, ownerID, doc.ComputerID, doc.TrajectoryID)
	if snapshotErr != nil {
		return nil, fmt.Errorf("load lifecycle Texture snapshot: %w", snapshotErr)
	}
	_, ownerHeadSeq, ownerHeadPending := store.PendingTextureOwnerRevision(snapshot)
	// A texture:<docID> subject whose document is bound to the engineering
	// desk is the supervision surface minted beside the cast — it consumes
	// producer reports only. Owner revisions are cast directives owned by the
	// engineering desk's occurrence consumer, never this agent, so the pending
	// owner head must not arm a Texture revision cell here.
	supervisionOnly := false
	for _, agent := range snapshot.Agents {
		if agent.AgentID == agentprofile.Engineering+":"+docID && agent.LifecycleVersion > 0 {
			supervisionOnly = true
			break
		}
	}
	if supervisionOnly {
		ownerHeadPending = false
	}

	// Consume-at-commit stranded repair: a pending producer report bound to a
	// run that terminated without a committed turn re-enters the eligible set
	// — the dead run proved nothing. Packets whose delivery budget is spent
	// terminalize as delivered (delivery_attempts_exhausted) inside the
	// reconcile-delivery command so nothing loops forever.
	// Receipt: docs/problems/texture-desk-activation-contract-respawn-loop-2026-09-28.md
	stranded, exhausted, strandedErr := rt.textureStrandedDeliveries(ctx, ownerID, doc, textureAgentID)
	if strandedErr != nil {
		return nil, strandedErr
	}
	armedUpdates := make([]types.CoagentSourcePacket, 0, len(updates)+len(stranded))
	armedUpdates = append(armedUpdates, updates...)
	armedUpdates = append(armedUpdates, stranded...)

	// Breaker (B6): consecutive dead desk activations (stale_activation or
	// failed mutations) stop update-driven dispatch at the cap. The owner head
	// stays armed — an unanswered owner directive is product-visible pressure,
	// not packet bookkeeping.
	breakerStreak, breakerErr := rt.textureActivationBreakerStreak(ctx, ownerID, doc)
	if breakerErr != nil {
		return nil, breakerErr
	}
	breakerTripped := breakerStreak >= textureDeliveryMaxAttempts
	breakerNewestRun := ""
	if breakerTripped {
		breakerNewestRun = strings.TrimSpace(rt.textureBreakerNewestRunID(ctx, ownerID, doc))
	}
	breakerReason := fmt.Sprintf("activation_breaker: %d consecutive desk activations terminated without a committed turn", breakerStreak)

	initialWorkWake := false
	if len(armedUpdates) == 0 && !ownerHeadPending {
		for _, work := range snapshot.WorkItems {
			if work.Status == types.WorkItemOpen && work.AssignedAgentID == textureAgentID &&
				!store.IsTextureSupervisionWorkItem(work.WorkItemID) {
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
				// Only a live activation suppresses the work-item wake. A
				// passivated or terminal run is a dead authority — its stale
				// obligation must re-arm through reactivatePassivatedTextureRun,
				// not stand down the open-work wake. This is the runtime_restarted
				// restart gap: the interrupted run suppressed its own recovery.
				if strings.TrimSpace(runs[i].AgentID) == textureAgentID &&
					runs[i].State.Active() &&
					isTextureAgentRevisionTaskType(metadataStringValue(runs[i].Metadata, "type")) {
					initialWorkWake = false
					break
				}
			}
		}
	}
	var scheduledSeq int64
	for _, update := range armedUpdates {
		if update.MessageSeq > scheduledSeq {
			scheduledSeq = update.MessageSeq
		}
	}
	if ownerHeadSeq > scheduledSeq {
		scheduledSeq = ownerHeadSeq
	}

	// Nothing armed: repair exhausted bindings and record the breaker trip,
	// then stop. No dispatch means no new delivery claims.
	if len(armedUpdates) == 0 && !ownerHeadPending && !initialWorkWake {
		if recErr := rt.reconcileTextureUpdateDelivery(ctx, doc, textureAgentID, "", nil, exhausted, breakerTripped, breakerStreak, breakerNewestRun, breakerReason); recErr != nil {
			return nil, recErr
		}
		return nil, nil
	}
	// Breaker-tripped update-driven pressure: exhaust the packet claims so
	// poison packets die, emit the durable breaker event, and refuse dispatch.
	// Owner-head pressure bypasses this gate entirely.
	if breakerTripped && !ownerHeadPending {
		if recErr := rt.reconcileTextureUpdateDelivery(ctx, doc, textureAgentID, "", armedUpdates, exhausted, true, breakerStreak, breakerNewestRun, breakerReason); recErr != nil {
			return nil, recErr
		}
		return nil, nil
	}
	if rec, reactivated, err := rt.reactivatePassivatedTextureRun(ctx, doc, textureAgentID, scheduledSeq, ownerHeadPending || initialWorkWake); err != nil {
		return nil, err
	} else if reactivated {
		if bindErr := rt.reconcileTextureUpdateDelivery(ctx, doc, textureAgentID, rec.RunID, armedUpdates, exhausted, false, 0, "", ""); bindErr != nil {
			log.Printf("texture controller: bind producer reports to reactivated run %s: %v", rec.RunID, bindErr)
		}
		return rec, nil
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
		if ownerHeadPending {
			return "apply_owner_revision"
		}
		return ""
	}(), "integrate_execution_findings")
	rec, err := rt.submitTextureAgentRevisionRun(ctx, doc, ownerID, textureAgentRevisionRequest{
		Intent: intent,
	}, scheduledSeq)
	if err != nil {
		if errors.Is(err, errTextureLifecycleOpenWorkUnavailable) {
			// The submit-time work gate is the durable guard against a run minted
			// without lifecycle authority. A live trajectory whose texture-agent
			// work item is already closed (drained, never minted, or settled after
			// snapshot) is quiescent — not a boot-fatal. Recheck the exact open-work
			// condition on a fresh snapshot; only propagate when work genuinely
			// exists for this agent, which would be a real contract violation.
			recheck, recheckErr := rt.Store.GetLifecycleSnapshot(ctx, ownerID, doc.ComputerID, doc.TrajectoryID)
			if recheckErr != nil {
				return nil, fmt.Errorf("recheck Texture lifecycle work after open-work refusal: %w", recheckErr)
			}
			workNowOpen := false
			for _, work := range recheck.WorkItems {
				if work.Status == types.WorkItemOpen && work.AssignedAgentID == textureAgentID {
					workNowOpen = true
					break
				}
			}
			if !workNowOpen {
				return nil, nil
			}
		}
		return nil, fmt.Errorf("start reconciled Texture revision: %w", err)
	}
	// Bind-at-dispatch: claim the covered packets onto the new run. Failure is
	// logged not fatal — consume-at-commit keys on scheduledSeq, so the commit
	// consumes them regardless; stranded repair catches a missed claim later.
	if bindErr := rt.reconcileTextureUpdateDelivery(ctx, doc, textureAgentID, rec.RunID, armedUpdates, exhausted, false, 0, "", ""); bindErr != nil {
		log.Printf("texture controller: bind producer reports to run %s: %v", rec.RunID, bindErr)
	}
	return rec, nil

}

// textureDeliveryMaxAttempts bounds dispatch-time delivery claims per packet
// (and doubles as the consecutive-dead-activation breaker cap). Three keeps
// poison packets terminal in seconds while tolerating one transient crash.
const textureDeliveryMaxAttempts = 3

// textureStrandedDeliveries returns pending producer reports addressed to this
// desk whose DeliveredToRunID names a run that is terminal or absent. Reports
// with delivery budget left rebind on dispatch; reports at the cap return in
// the exhausted slice for terminal delivery bookkeeping.
func (rt *Handler) textureStrandedDeliveries(ctx context.Context, ownerID string, doc types.Document, textureAgentID string) (stranded []types.CoagentSourcePacket, exhausted []types.CoagentSourcePacket, err error) {
	if rt == nil || rt.Store == nil {
		return nil, nil, nil
	}
	bound, err := rt.Store.ListBoundPendingUpdatesForTarget(ctx, ownerID, strings.TrimSpace(doc.ComputerID), textureAgentID)
	if err != nil {
		return nil, nil, fmt.Errorf("list bound pending Texture producer reports: %w", err)
	}
	dead := map[string]bool{}
	checked := map[string]bool{}
	for _, update := range bound {
		if strings.TrimSpace(update.TrajectoryID) != strings.TrimSpace(doc.TrajectoryID) ||
			strings.TrimSpace(update.TargetAgentID) != textureAgentID {
			continue
		}
		runID := strings.TrimSpace(update.DeliveredToRunID)
		live, seen := checked[runID]
		if !seen {
			run, getErr := rt.Store.GetLifecycleRun(ctx, ownerID, doc.ComputerID, runID)
			switch {
			case errors.Is(getErr, store.ErrNotFound):
				live = false
			case getErr != nil:
				return nil, nil, fmt.Errorf("check bound run %s liveness: %w", runID, getErr)
			default:
				live = !run.State.Terminal()
			}
			checked[runID] = live
		}
		if live {
			continue
		}
		if dead[update.UpdateID] {
			continue
		}
		dead[update.UpdateID] = true
		if update.DeliveryAttempts+1 > textureDeliveryMaxAttempts {
			exhausted = append(exhausted, update)
		} else {
			stranded = append(stranded, update)
		}
	}
	return stranded, exhausted, nil
}

// textureActivationBreakerStreak counts consecutive dead desk activations —
// stale_activation or failed mutation rows, newest first. Any row that
// committed a turn (completed or sleeping) or is still in flight (pending)
// ends the streak.
func (rt *Handler) textureActivationBreakerStreak(ctx context.Context, ownerID string, doc types.Document) (int, error) {
	mutations, err := rt.Store.ListRecentAgentMutationsByDoc(ctx, ownerID, strings.TrimSpace(doc.ComputerID), strings.TrimSpace(doc.DocID), 2*textureDeliveryMaxAttempts)
	if err != nil {
		return 0, fmt.Errorf("list recent Texture mutations: %w", err)
	}
	streak := 0
	for _, m := range mutations {
		if m.State == "stale_activation" || m.State == "failed" {
			streak++
			continue
		}
		break
	}
	return streak, nil
}

// textureBreakerNewestRunID returns the run id of the newest counted dead
// activation, keying breaker events so each distinct collapse emits once.
func (rt *Handler) textureBreakerNewestRunID(ctx context.Context, ownerID string, doc types.Document) string {
	mutations, err := rt.Store.ListRecentAgentMutationsByDoc(ctx, ownerID, strings.TrimSpace(doc.ComputerID), strings.TrimSpace(doc.DocID), 1)
	if err != nil || len(mutations) == 0 {
		return ""
	}
	return mutations[0].RunID
}

// reconcileTextureUpdateDelivery issues the consume-at-commit bookkeeping
// command: binds covered packets to the armed run (targetRunID empty means
// exhaust-only), terminalizes packets at their delivery cap, and records a
// breaker event when tripped. bindCandidates carry their current claim in
// ExpectedRunID so a stranded packet rebinds only from the dead run observed.
func (rt *Handler) reconcileTextureUpdateDelivery(ctx context.Context, doc types.Document, textureAgentID, targetRunID string, bindCandidates, exhaustCandidates []types.CoagentSourcePacket, breakerTripped bool, breakerStreak int, breakerRunID, breakerReason string) error {
	if rt == nil || rt.Store == nil {
		return nil
	}
	if len(bindCandidates) == 0 && len(exhaustCandidates) == 0 && !breakerTripped {
		return nil
	}
	ownerID := strings.TrimSpace(doc.OwnerID)
	trajectoryID := strings.TrimSpace(doc.TrajectoryID)
	items := make([]types.ReconcileUpdateDeliveryItem, 0, len(bindCandidates)+len(exhaustCandidates))
	for _, u := range bindCandidates {
		items = append(items, types.ReconcileUpdateDeliveryItem{
			UpdateID: strings.TrimSpace(u.UpdateID), ProducerAgentID: u.AgentID, ProducerUpdateID: u.ProducerUpdateID,
			ExpectedLifecycleVersion: u.LifecycleVersion, ExpectedRunID: strings.TrimSpace(u.DeliveredToRunID),
		})
	}
	for _, u := range exhaustCandidates {
		items = append(items, types.ReconcileUpdateDeliveryItem{
			UpdateID: strings.TrimSpace(u.UpdateID), ProducerAgentID: u.AgentID, ProducerUpdateID: u.ProducerUpdateID,
			ExpectedLifecycleVersion: u.LifecycleVersion, ExpectedRunID: strings.TrimSpace(u.DeliveredToRunID),
			Exhaust: true,
		})
	}
	req := types.ReconcileUpdateDeliveryRequest{
		OwnerID: ownerID, ComputerID: strings.TrimSpace(doc.ComputerID), TrajectoryID: trajectoryID,
		TargetAgentID: textureAgentID, TargetRunID: strings.TrimSpace(targetRunID),
		MaxAttempts: textureDeliveryMaxAttempts, Items: items,
	}
	// The CommandID is the command's durable identity. Because the item set is
	// part of command identity (see ComputeReconcileUpdateDeliveryDigest), the
	// ID must be content-derived: an identical request replays the stored
	// receipt, while a request whose armed/exhausted set drifted across a boot
	// gets a fresh ID instead of colliding on the prior receipt's digest —
	// which surfaced as `lifecycle command digest conflict` startup refusals
	// (docs/problems/m0-residual-texture-delivery-command-digest-crashloop-2026-09-30.md).
	commandKey := "bind:" + strings.TrimSpace(targetRunID)
	if strings.TrimSpace(targetRunID) == "" {
		commandKey = "exhaust:" + breakerRunID + ":" + fmt.Sprintf("%d", breakerStreak)
	}
	if breakerTripped {
		req.BreakerReason = breakerReason
	}
	req.CommandID = "texture-delivery:" + trajectoryID + ":" + textureAgentID + ":" + commandKey + ":" + textureDeliveryContentKey(req)
	var err error
	req.CommandDigest, err = store.ComputeReconcileUpdateDeliveryDigest(req)
	if err != nil {
		return fmt.Errorf("digest texture update delivery: %w", err)
	}
	if _, err := rt.Store.ReconcileUpdateDelivery(context.WithoutCancel(ctx), req); err != nil {
		return fmt.Errorf("reconcile texture update delivery: %w", err)
	}
	return nil
}

// textureDeliveryContentKey digests the reconcile request's drifting content —
// the target run/agent, the armed+exhausted item set, breaker state, and the
// attempt cap — so the durable CommandID names this exact batch. Items are
// sorted by UpdateID before hashing so a reordered equivalent batch reuses the
// same identity (replay dedup), while a genuinely changed set gets a fresh ID.
// Called before CommandDigest is computed, so CommandID and CommandDigest are
// empty here and never feed the key. Returns "" only if the content fails to
// marshal.
func textureDeliveryContentKey(req types.ReconcileUpdateDeliveryRequest) string {
	items := make([]types.ReconcileUpdateDeliveryItem, len(req.Items))
	copy(items, req.Items)
	sort.Slice(items, func(i, j int) bool { return items[i].UpdateID < items[j].UpdateID })
	content := struct {
		TargetAgentID string                              `json:"target_agent_id"`
		TargetRunID   string                              `json:"target_run_id"`
		MaxAttempts   int                                 `json:"max_attempts"`
		BreakerReason string                              `json:"breaker_reason,omitempty"`
		Items         []types.ReconcileUpdateDeliveryItem `json:"items"`
	}{
		TargetAgentID: req.TargetAgentID,
		TargetRunID:   req.TargetRunID,
		MaxAttempts:   req.MaxAttempts,
		BreakerReason: req.BreakerReason,
		Items:         items,
	}
	payload, err := json.Marshal(content)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])[:16]
}

// docWakeArmed reports that the document still carries wake pressure
// independent of unbound packet coverage: an unconsumed owner head or open
// desk work. It keeps a sleeping mutation reactivatable after consume-at-commit
// bound its packets to a run that died before consuming them.
func (rt *Handler) reactivatePassivatedTextureRun(ctx context.Context, doc types.Document, textureAgentID string, scheduledSeq int64, docWakeArmed bool) (*types.RunRecord, bool, error) {
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
	var passivated []passivatedTextureAuthority
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
		// Sleeping reactivates whenever the document still carries wake
		// pressure: unbound packets (scheduledSeq > 0), an unconsumed owner
		// head, or open desk work — plus packets already bound to this run
		// that never consumed. scheduledSeq only measures *unbound* coverage;
		// a bound-but-pending packet proves its claim run died before
		// consuming it.
		if mutation.State != "pending" && mutation.State != "stale_activation" {
			sleepingReactivable := docWakeArmed || scheduledSeq > 0
			if mutation.State == "sleeping" && !sleepingReactivable {
				sleepingReactivable, mutationErr = rt.mutationHasPendingBoundPackets(ctx, ownerID, doc.ComputerID, doc.TrajectoryID, textureAgentID, candidate.RunID)
				if mutationErr != nil {
					return nil, false, fmt.Errorf("check bound pending packets for passivated run %s: %w", candidate.RunID, mutationErr)
				}
			}
			if mutation.State != "sleeping" || !sleepingReactivable {
				continue
			}
		}
		passivated = append(passivated, passivatedTextureAuthority{run: *candidate, sequence: mutation.ScheduledMessageSeq, mutationCreatedAt: mutation.CreatedAt})
	}
	rec, selectErr := selectPassivatedTextureAuthority(passivated)
	if selectErr != nil {
		return nil, false, fmt.Errorf("%w for document %s", selectErr, docID)
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
	// The coverage watermark never shrinks: a sleeping mutation reactivated on
	// bound-but-pending packets or owner-head pressure keeps the watermark that
	// already claims them.
	if selectedMutation != nil && scheduledSeq < selectedMutation.ScheduledMessageSeq {
		scheduledSeq = selectedMutation.ScheduledMessageSeq
	}
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

// mutationHasPendingBoundPackets reports whether any pending producer report is
// already claimed by runID — a bound-but-unconsumed packet is wake pressure for
// exactly the sleeping mutation that owns the claim.
func (rt *Handler) mutationHasPendingBoundPackets(ctx context.Context, ownerID, computerID, trajectoryID, textureAgentID, runID string) (bool, error) {
	bound, err := rt.Store.ListBoundPendingUpdatesForTarget(ctx, ownerID, computerID, textureAgentID)
	if err != nil {
		return false, err
	}
	for _, update := range bound {
		if strings.TrimSpace(update.TrajectoryID) == strings.TrimSpace(trajectoryID) &&
			strings.TrimSpace(update.DeliveredToRunID) == strings.TrimSpace(runID) {
			return true, nil
		}
	}
	return false, nil
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
	case agentprofile.Research, agentprofile.Management, agentprofile.Engineering:
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
	return producerOccurrenceScopeMatches(o, update)
}

func revisionOccurrenceScopeMatches(o agentcore.TextureActorOccurrence, revision types.Revision) bool {
	return o.Version == agentcore.TextureActorOccurrenceVersion && o.Kind == agentcore.TextureActorOccurrenceDocumentRevision &&
		o.OwnerID == strings.TrimSpace(revision.OwnerID) && o.ComputerID == strings.TrimSpace(revision.ComputerID) &&
		o.TrajectoryID == strings.TrimSpace(revision.TrajectoryID) && o.DocumentID == strings.TrimSpace(revision.DocID) &&
		o.HeadRevisionID == strings.TrimSpace(revision.RevisionID)
}

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
			if _, docErr := rt.Store.GetLifecycleDocument(ctx, ownerID, computerID, docID); docErr == nil {
				if revision, revErr := rt.Store.GetLifecycleRevision(ctx, ownerID, computerID, strings.TrimSpace(content)); revErr == nil {
					o, err = agentcore.TextureDocumentRevisionOccurrence(revision, "", 0)
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
	// An Engineering report's binding is proven by its stored assignment
	// (ValidateLifecycleProducerReportAuthority pins ParentControlID); a
	// delegated cast binds a commitment record, not a lifecycle control
	// update, so the update scan below may find none for it.
	producerAssignmentBound := false
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
		producerProfile, _ := agentprofile.Canonical(producerAgent.Profile)
		// Doc-bound producers carry the document channel; self-channeled
		// producers (Engineering assignments, persistent Management) carry
		// their own agent id — the channel is the scope, not the document.
		producerChannelMatches := producerAgent.ChannelID == o.DocumentID ||
			(producerProfile != agentprofile.Research && producerAgent.ChannelID == producerAgent.AgentID)
		if producerAgent.OwnerID != o.OwnerID || producerAgent.ComputerID != o.ComputerID || !producerChannelMatches ||
			(producerProfile != agentprofile.Research && producerProfile != agentprofile.Management && producerProfile != agentprofile.Engineering) ||
			(producerProfile == agentprofile.Management && producerAgent.AgentID != persistentManagementAgentID(o.OwnerID)) ||
			(producerProfile != agentprofile.Management && producerAgent.LifecycleVersion <= 0) {
			return zero, "", invalidTextureOccurrence("Texture producer agent authority mismatch")
		}
		producerWork, producerWorkErr := rt.Store.GetLifecycleWorkItem(ctx, o.OwnerID, o.ComputerID, o.ProducerWorkID)
		if producerWorkErr != nil {
			if errors.Is(producerWorkErr, store.ErrNotFound) {
				return zero, "", invalidTextureOccurrence("Texture producer work is missing")
			}
			return zero, "", producerWorkErr
		}
		producerWorkAuthorityProfile, _ := agentprofile.Canonical(producerWork.AuthorityProfile)
		if producerWork.TrajectoryID != o.TrajectoryID || producerWork.AssignedAgentID != o.ProducerAgentID || producerWork.LifecycleVersion <= 0 || producerWorkAuthorityProfile != producerProfile {
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
		if producerProfile == agentprofile.Management {
			trajectoryBound = producerRun.TrajectoryID == "" && metadataStringValue(producerRun.Metadata, "assignment_trajectory_id") == o.TrajectoryID
		}
		producerRunProfile, _ := agentprofile.Canonical(producerRun.AgentProfile)
		producerRunRole, _ := agentprofile.Canonical(producerRun.AgentRole)
		// Doc-bound producer runs carry the document channel; Engineering
		// assignment runs are self-channeled (ChannelID == AgentID) and
		// Management runs carry the persistent agent's channel.
		runChannelMatches := producerRun.ChannelID == o.DocumentID ||
			(producerProfile == agentprofile.Engineering && producerRun.ChannelID == producerRun.AgentID) ||
			(producerProfile == agentprofile.Management && producerRun.ChannelID == producerAgent.ChannelID)
		if producerRun.RunID != canonical.SourceRunID || producerRun.OwnerID != o.OwnerID || producerRun.ComputerID != o.ComputerID || producerRun.AgentID != o.ProducerAgentID || !trajectoryBound || !runChannelMatches || producerRunProfile != producerProfile || producerRunRole != producerProfile {
			return zero, "", invalidTextureOccurrence("Texture producer source run authority mismatch")
		}
		if authorityErr := rt.Core.ValidateLifecycleProducerReportAuthority(ctx, canonical); authorityErr != nil {
			if errors.Is(authorityErr, agentcore.ErrInvalidLifecycleProducerReportAuthority) {
				return zero, "", invalidTextureOccurrence("Texture producer control/run authority mismatch: %v", authorityErr)
			}
			return zero, "", fmt.Errorf("validate Texture producer control/run authority: %w", authorityErr)
		}
		producerBindingID = strings.TrimSpace(canonical.ControlBindingID)
		producerAssignmentBound = producerProfile == agentprofile.Engineering
		pending = canonical.Disposition == types.UpdatePending
	case agentcore.TextureActorOccurrenceDocumentRevision:
		canonical, getErr := rt.Store.GetLifecycleRevision(ctx, o.OwnerID, o.ComputerID, o.HeadRevisionID)
		if getErr != nil {
			if errors.Is(getErr, store.ErrNotFound) {
				return zero, "", invalidTextureOccurrence("exact Texture owner revision occurrence is missing")
			}
			return zero, "", fmt.Errorf("load exact Texture owner revision occurrence: %w", getErr)
		}
		if !revisionOccurrenceScopeMatches(o, canonical) {
			return zero, "", invalidTextureOccurrence("Texture owner revision occurrence canonical identity mismatch")
		}
		pending = true
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
		if bindingMatches > 1 || (bindingMatches == 0 && !producerAssignmentBound) {
			return zero, "", invalidTextureOccurrence("Texture producer control binding authority mismatch")
		}
	}
	if o.Kind == agentcore.TextureActorOccurrenceDocumentRevision &&
		(doc.CurrentRevisionID != o.HeadRevisionID || store.TextureTurnConsumedHead(snapshot.Events, o.HeadRevisionID)) {
		return o, TextureActorOccurrenceTerminal, nil
	}
	if o.Kind == agentcore.TextureActorOccurrenceDocumentRevision {
		// A texture:<docID> agent on an engineering-bound document is the
		// supervision surface, never a revision executor — owner-revision
		// occurrences minted before the wake resolver learned the executor
		// first rule terminalize here so they cannot arm a Texture apply.
		for _, agent := range snapshot.Agents {
			if agent.AgentID == agentprofile.Engineering+":"+doc.DocID && agent.LifecycleVersion > 0 {
				return o, TextureActorOccurrenceTerminal, nil
			}
		}
	}

	agent, err := rt.Store.GetAgentByScope(ctx, o.OwnerID, o.ComputerID, o.TargetAgentID)
	agentProfile, _ := agentprofile.Canonical(agent.Profile)
	agentRole, _ := agentprofile.Canonical(agent.Role)
	if err != nil || agentProfile != agentprofile.Texture || agentRole != agentprofile.Texture || agent.ChannelID != o.DocumentID || agent.LifecycleVersion <= 0 {
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
			candidateAuthorityProfile, _ := agentprofile.Canonical(candidate.AuthorityProfile)
			if candidate.Status != types.WorkItemOpen || candidate.AssignedAgentID != o.TargetAgentID || candidateAuthorityProfile != agentprofile.Texture {
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
	workAuthorityProfile, _ := agentprofile.Canonical(work.AuthorityProfile)
	if err != nil || work.Status != types.WorkItemOpen || work.TrajectoryID != o.TrajectoryID || work.AssignedAgentID != o.TargetAgentID || workAuthorityProfile != agentprofile.Texture {
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
	case agentcore.TextureActorOccurrenceDocumentRevision:
		canonical, err := rt.Store.GetLifecycleRevision(ctx, o.OwnerID, o.ComputerID, o.HeadRevisionID)
		if err != nil {
			return "", err
		}
		if !revisionOccurrenceScopeMatches(o, canonical) {
			return "", fmt.Errorf("Texture owner revision occurrence changed immutable identity")
		}
		if snapshot.Document.CurrentRevisionID == o.HeadRevisionID && !store.TextureTurnConsumedHead(snapshot.Events, o.HeadRevisionID) {
			return TextureActorOccurrencePending, nil
		}
		return TextureActorOccurrenceTerminal, nil
	default:
		return "", fmt.Errorf("unsupported Texture occurrence kind")
	}
}
