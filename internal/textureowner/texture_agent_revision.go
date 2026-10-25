package textureowner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

var errTextureLifecycleOpenWorkUnavailable = fmt.Errorf("Texture lifecycle open work unavailable")

// textureAgentRevisionRequest is the JSON payload for
// POST /api/texture/documents/{id}/revise.
// On a lifecycle-bound document the owner's directive commits as a canonical
// owner-authored revision carrying forward the current head content; the
// document revision event is the desk's input (VAL-ETEXT-003).
type textureAgentRevisionRequest struct {
	Intent                 string                `json:"intent,omitempty"`
	Prompt                 string                `json:"prompt,omitempty"`
	ClientRequestID        string                `json:"client_request_id,omitempty"`
	ExpectedHeadRevisionID string                `json:"expected_head_revision_id,omitempty"`
	SourceEntities         []textureSourceEntity `json:"-"`
	RequestedByRunID       string                `json:"-"`
	Provenance             map[string]any        `json:"-"`
}

// HandleTextureAgentRevision handles POST
// /api/texture/documents/{id}/revise.
//
// It creates a runtime task that, when completed, will create a canonical
// appagent-authored revision. The task ID is returned so the client can
// track progress and completion through the existing event stream
// (VAL-ETEXT-003, VAL-ETEXT-004).
//
// If a pending agent mutation already exists for this document (e.g., from
// a previous request that is still in-flight), the existing task ID is
// returned instead of creating a new mutation, preventing duplicate
// canonical revisions when renewal/retry occurs mid-mutation
// (VAL-CROSS-122).
func (h *Handler) HandleTextureAgentRevision(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}

	docID := extractDocID(r.URL.Path)
	if docID == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "document ID is required"})
		return
	}

	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}

	var req textureAgentRevisionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid request body"})
		return
	}

	// Verify the document exists and belongs to this owner.
	doc, err := h.getTextureDocument(r.Context(), ownerID, docID)
	if err != nil {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "document not found"})
		return
	}
	if strings.TrimSpace(doc.TrajectoryID) != "" {
		h.handleLifecycleOwnerRevision(w, r, doc, ownerID, req)
		return
	}
	// Unbound documents have no lifecycle authority: there is no canonical
	// revision event to carry owner input, and the legacy worker-mailbox path
	// was deleted with the owner-instruction channel.
	writeAPIJSON(w, http.StatusConflict, apiError{Error: "document is not lifecycle-bound; owner revisions require a lifecycle Texture document"})
}

// handleLifecycleOwnerRevision commits the owner's /revise directive as a
// canonical owner-authored revision on the document head and dispatches the
// revision occurrence that activates the Texture desk. The revision carries
// the head content forward unchanged; the directive lives in revision
// metadata (input_origin=user_prompt, owner_prompt).
func (h *Handler) handleLifecycleOwnerRevision(w http.ResponseWriter, r *http.Request, doc types.Document, ownerID string, req textureAgentRevisionRequest) {
	snapshot, snapshotErr := h.Store.GetLifecycleSnapshot(r.Context(), ownerID, doc.ComputerID, doc.TrajectoryID)
	if snapshotErr != nil || snapshot.Trajectory.Status != types.TrajectoryLive {
		writeAPIJSON(w, http.StatusConflict, apiError{Error: "durable lifecycle state is unavailable or terminal"})
		return
	}
	content := strings.TrimSpace(firstNonEmpty(req.Prompt, req.Intent))
	if content == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "prompt or intent is required"})
		return
	}
	clientRequestID := strings.TrimSpace(firstNonEmpty(req.ClientRequestID, r.Header.Get("Idempotency-Key")))
	if clientRequestID == "" {
		clientRequestID = uuid.NewString()
	}
	expectedHead := strings.TrimSpace(req.ExpectedHeadRevisionID)
	if expectedHead == "" {
		expectedHead = snapshot.Document.CurrentRevisionID
	}
	head := snapshot.HeadRevision
	if head.RevisionID != expectedHead {
		writeAPIJSON(w, http.StatusConflict, apiError{Error: "document head changed; reload the latest version before revising"})
		return
	}
	targetProfile, targetWorkItemID, multiple := ownerRevisionTarget(snapshot, doc.DocID)
	if multiple {
		writeAPIJSON(w, http.StatusConflict, apiError{Error: "lifecycle has multiple open desk target work items"})
		return
	}
	if targetWorkItemID == "" && (textureWorkSettledByDesk(snapshot, doc.DocID) || storyTextureDeskLive(snapshot, doc.DocID)) {
		// The desk completed its work on a live document, or a living story
		// (which has no work items by design, problem doc P5) has a live
		// desk; owner input is new work (texture-settled-work-refuses-owner-revise-2026-10-09).
		// A lost race to another reopen is fine: the re-read below joins
		// whichever single open item won.
		if _, reopenErr := h.reopenTextureWorkForOwner(r.Context(), ownerID, doc, clientRequestID); reopenErr != nil {
			log.Printf("texture api: reopen Texture work for owner revise: %v", reopenErr)
		}
		if reloaded, reloadErr := h.Store.GetLifecycleSnapshot(r.Context(), ownerID, doc.ComputerID, doc.TrajectoryID); reloadErr == nil && reloaded.Trajectory.Status == types.TrajectoryLive {
			snapshot = reloaded
			targetProfile, targetWorkItemID, multiple = ownerRevisionTarget(snapshot, doc.DocID)
			if multiple {
				writeAPIJSON(w, http.StatusConflict, apiError{Error: "lifecycle has multiple open desk target work items"})
				return
			}
		}
	}
	if targetWorkItemID == "" {
		writeAPIJSON(w, http.StatusConflict, apiError{Error: "lifecycle has no open desk target work item"})
		return
	}
	if head = snapshot.HeadRevision; head.RevisionID != expectedHead {
		writeAPIJSON(w, http.StatusConflict, apiError{Error: "document head changed; reload the latest version before revising"})
		return
	}
	metadata := mergeTextureRevisionMetadata(nil, map[string]any{
		"input_origin":              textureInputOriginUserPrompt,
		"owner_prompt":              content,
		textureMetadataPromptUnixTS: time.Now().UTC().Unix(),
	})
	metadata = carryForwardDurableTextureMetadata(metadata, head.Metadata)
	revisionID := uuid.NewSHA1(uuid.NameSpaceOID, []byte(strings.Join([]string{ownerID, doc.ComputerID, doc.TrajectoryID, clientRequestID}, "\x00"))).String()
	rev := types.Revision{
		RevisionID:       revisionID,
		DocID:            doc.DocID,
		OwnerID:          ownerID,
		ComputerID:       doc.ComputerID,
		TrajectoryID:     doc.TrajectoryID,
		AuthorKind:       types.AuthorUser,
		AuthorLabel:      ownerID,
		Content:          head.Content,
		BodyDoc:          head.BodyDoc,
		SourceEntities:   head.SourceEntities,
		Citations:        head.Citations,
		Metadata:         metadata,
		ParentRevisionID: expectedHead,
	}
	requestID, _ := textureOwnerOccurrenceIdentity(ownerID, doc.ComputerID, doc.DocID, clientRequestID)
	command := types.CommitLifecycleArtifactHeadRequest{
		OwnerID: ownerID, ComputerID: doc.ComputerID,
		CommandID:                "owner-revise:" + clientRequestID,
		TrajectoryID:             doc.TrajectoryID,
		ExpectedLifecycleVersion: snapshot.Trajectory.LifecycleVersion,
		ExpectedHeadRevisionID:   expectedHead,
		Revision:                 rev,
	}
	commandDigest, digestErr := store.ComputeCommitLifecycleArtifactHeadWithSourceGraphDigest(command, store.TextureSourceGraphWriteSet{})
	if digestErr != nil {
		log.Printf("texture api: digest owner revision command: %v", digestErr)
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid lifecycle revision payload"})
		return
	}
	command.CommandDigest = commandDigest
	result, commitErr := h.Store.CommitLifecycleArtifactHeadWithSourceGraph(r.Context(), command, store.TextureSourceGraphWriteSet{})
	if commitErr != nil {
		if errors.Is(commitErr, store.ErrConcurrentStateChange) || errors.Is(commitErr, store.ErrLifecycleCommandConflict) || errors.Is(commitErr, store.ErrLifecycleInvalidTransition) {
			writeAPIJSON(w, http.StatusConflict, apiError{Error: commitErr.Error()})
		} else {
			log.Printf("texture api: commit owner revision: %v", commitErr)
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to commit owner revision"})
		}
		return
	}
	if result.Revision == nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "lifecycle revision result unavailable"})
		return
	}
	if !result.Replay {
		h.recordTextureAudit(r.Context(), "revision_committed", ownerID, doc.ComputerID, doc.TrajectoryID, doc.DocID, result.Revision.RevisionID, command.CommandID, command.CommandDigest, result.Trajectory.LifecycleVersion)
		h.emitTextureDocumentRevisionEvent(r.Context(), ownerID, *result.Revision)
		h.dispatchTextureRevisionWake(ownerID, doc.ComputerID, doc.TrajectoryID, *result.Revision, requestID, result.Events, targetProfile)
	}
	writeAPIJSON(w, http.StatusAccepted, textureOwnerRevisionResponse{
		Schema:     textureOwnerRevisionSchemaV1,
		DocID:      doc.DocID,
		RevisionID: result.Revision.RevisionID,
		RequestID:  requestID,
		Replay:     result.Replay,
	})
}

// ownerRevisionTarget finds the one open desk work item an owner revise
// feeds: Texture's, assigned to this document's desk agent (the
// engineering-bound document, path B, is deleted: C1e step 3).
func ownerRevisionTarget(snapshot types.LifecycleSnapshot, docID string) (profile, workItemID string, multiple bool) {
	for _, work := range snapshot.WorkItems {
		if work.Status != types.WorkItemOpen {
			continue
		}
		p := strings.TrimSpace(work.AuthorityProfile)
		if p != agentprofile.Texture {
			continue
		}
		if work.AssignedAgentID != p+":"+docID {
			continue
		}
		if workItemID != "" {
			return "", "", true
		}
		profile, workItemID = p, work.WorkItemID
	}
	return profile, workItemID, false
}

// storyTextureDeskLive reports a live living-story trajectory whose document
// is docID and whose Texture desk agent texture:<docID> exists, and which is
// not engineering-bound. Only such a story opens Texture work for an owner
// revise when it has none.
func storyTextureDeskLive(snapshot types.LifecycleSnapshot, docID string) bool {
	trajectory := snapshot.Trajectory
	if trajectory.Status != types.TrajectoryLive || trajectory.Kind != types.TrajectoryKindStory || trajectory.SubjectRefs["doc_id"] != docID {
		return false
	}
	textureAgentID := agentprofile.Texture + ":" + docID
	deskLive := false
	for _, agent := range snapshot.Agents {
		if agent.AgentID == agentprofile.Engineering+":"+docID && agent.LifecycleVersion > 0 {
			return false
		}
		if agent.AgentID == textureAgentID && agent.LifecycleVersion > 0 {
			deskLive = true
		}
	}
	return deskLive
}

// textureWorkSettledByDesk reports a live document whose Texture desk
// completed its latest work item (not refused or cancelled) and which is not
// engineering-bound. Only this state reopens work on owner input; a refused
// item still refuses the revise.
func textureWorkSettledByDesk(snapshot types.LifecycleSnapshot, docID string) bool {
	if snapshot.Trajectory.Status != types.TrajectoryLive {
		return false
	}
	textureAgentID := agentprofile.Texture + ":" + docID
	agentLive := false
	for _, agent := range snapshot.Agents {
		if agent.AgentID == agentprofile.Engineering+":"+docID && agent.LifecycleVersion > 0 {
			return false
		}
		if agent.AgentID == textureAgentID && agent.LifecycleVersion > 0 {
			agentLive = true
		}
	}
	if !agentLive {
		return false
	}
	var latest *types.WorkItemRecord
	for i := range snapshot.WorkItems {
		work := &snapshot.WorkItems[i]
		if work.AssignedAgentID != textureAgentID || strings.TrimSpace(work.AuthorityProfile) != agentprofile.Texture {
			continue
		}
		if latest == nil || work.UpdatedAt.After(latest.UpdatedAt) {
			latest = work
		}
	}
	return latest != nil && latest.Status == types.WorkItemCompleted
}

// textureOwnerReopenObjective is constant so the open-work fingerprint lets
// only one concurrent reopen win; the loser re-reads and joins it.
const textureOwnerReopenObjective = "Revise the document for the owner's new request."

func (h *Handler) reopenTextureWorkForOwner(ctx context.Context, ownerID string, doc types.Document, clientRequestID string) (types.LifecycleResult, error) {
	workID := uuid.NewSHA1(uuid.NameSpaceOID, []byte(strings.Join([]string{"texture-owner-reopen", ownerID, doc.ComputerID, doc.TrajectoryID, clientRequestID}, "\x00"))).String()
	req := types.OpenLifecycleWorkRequest{
		OwnerID: ownerID, ComputerID: doc.ComputerID, CommandID: "owner-reopen-texture:" + workID,
		TrajectoryID: doc.TrajectoryID,
		WorkItem: types.WorkItemRecord{
			WorkItemID: workID, Objective: textureOwnerReopenObjective,
			AssignedAgentID: agentprofile.Texture + ":" + doc.DocID, AuthorityProfile: agentprofile.Texture,
		},
	}
	digest, err := store.ComputeOpenLifecycleWorkDigest(req)
	if err != nil {
		return types.LifecycleResult{}, err
	}
	req.CommandDigest = digest
	return h.Store.OpenLifecycleWork(ctx, req)
}

const textureOwnerRevisionSchemaV1 = "choir.texture_owner_revision.v1"

// textureOwnerRevisionResponse is the /revise response on a lifecycle-bound
// document: the committed owner revision is the durable receipt and the desk
// follows the document revision event stream.
type textureOwnerRevisionResponse struct {
	Schema     string `json:"schema"`
	DocID      string `json:"doc_id"`
	RevisionID string `json:"revision_id"`
	RequestID  string `json:"request_id"`
	Replay     bool   `json:"replay"`
}

func (h *Handler) pendingAgentMutationByDoc(ctx context.Context, ownerID, computerID, docID string) (*store.AgentMutation, error) {
	mutation, err := h.Store.GetPendingAgentMutationByDoc(ctx, ownerID, computerID, docID)
	if err != nil || mutation == nil {
		return mutation, err
	}
	var run types.RunRecord
	if strings.TrimSpace(computerID) != "" {
		run, err = h.Store.GetLifecycleRun(ctx, ownerID, computerID, mutation.RunID)
	} else {
		runPtr, runErr := h.Core.GetRun(ctx, mutation.RunID, ownerID)
		err = runErr
		if runPtr != nil {
			run = *runPtr
		}
	}
	if err == nil && !run.State.Terminal() {
		// A long-running Texture actor keeps one pending mutation across
		// revisions; cleanup applies only to terminal or missing runs.
		return mutation, nil
	}
	reconciled, reconcileErr := h.reconcilePendingMutationFromDocumentHead(ctx, mutation)
	if reconcileErr != nil {
		log.Printf("texture api: reconcile pending mutation %s from document head: %v", mutation.RunID, reconcileErr)
	} else if reconciled {
		return nil, nil
	}
	if err != nil {
		return mutation, nil
	}
	if err := h.Store.MarkAgentMutationStale(ctx, mutation.OwnerID, mutation.ComputerID, mutation.RunID); err != nil {
		log.Printf("texture api: mark stale pending mutation %s: %v", mutation.RunID, err)
		return mutation, nil
	}
	return nil, nil
}

func (h *Handler) reconcilePendingMutationFromDocumentHead(ctx context.Context, mutation *store.AgentMutation) (bool, error) {
	if mutation == nil || strings.TrimSpace(mutation.RunID) == "" || strings.TrimSpace(mutation.DocID) == "" || strings.TrimSpace(mutation.OwnerID) == "" {
		return false, nil
	}
	doc, err := h.getTextureDocument(ctx, mutation.OwnerID, mutation.DocID)
	if err != nil {
		return false, err
	}
	if strings.TrimSpace(doc.CurrentRevisionID) == "" {
		return false, nil
	}
	rev, err := h.getTextureRevision(ctx, mutation.OwnerID, doc.CurrentRevisionID)
	if err != nil {
		return false, err
	}
	if rev.AuthorKind != types.AuthorAppAgent {
		return false, nil
	}
	meta := decodeRevisionMetadata(rev.Metadata)
	if !isTextureWriteToolName(metadataStringValue(meta, "source")) || metadataStringValue(meta, "loop_id") != mutation.RunID {
		return false, nil
	}
	if err := h.Store.CompleteAgentMutation(ctx, mutation.OwnerID, mutation.ComputerID, mutation.RunID, rev.RevisionID); err != nil && err != store.ErrMutationAlreadyCompleted {
		return false, err
	}
	return true, nil
}

func textureRevisionMatchesDocument(revision types.Revision, doc types.Document, ownerID string) bool {
	return strings.TrimSpace(revision.RevisionID) == strings.TrimSpace(doc.CurrentRevisionID) &&
		strings.TrimSpace(revision.DocID) == strings.TrimSpace(doc.DocID) &&
		strings.TrimSpace(revision.OwnerID) == strings.TrimSpace(ownerID) &&
		strings.TrimSpace(revision.ComputerID) == strings.TrimSpace(doc.ComputerID)
}

func (rt *Handler) submitTextureAgentRevisionRun(ctx context.Context, doc types.Document, ownerID string, req textureAgentRevisionRequest, scheduledMessageSeq int64) (*types.RunRecord, error) {
	requestIntent := strings.TrimSpace(req.Intent)
	if requestIntent == "" {
		requestIntent = "revise"
	}
	// Build the backend-owned Texture revision request from current document state.
	currentRevisionID := strings.TrimSpace(doc.CurrentRevisionID)
	if currentRevisionID == "" {
		return nil, fmt.Errorf("start Texture revision: current_revision_id is required")
	}
	currentRevision, err := rt.getTextureRevision(ctx, ownerID, currentRevisionID)
	if err != nil {
		return nil, fmt.Errorf("start Texture revision: load current revision: %w", err)
	}
	if !textureRevisionMatchesDocument(currentRevision, doc, ownerID) {
		return nil, fmt.Errorf("start Texture revision: current revision scope mismatch")
	}
	metadata := decodeRevisionMetadata(currentRevision.Metadata)
	if metadata == nil {
		metadata = map[string]any{}
	}
	var previousRevision *types.Revision
	if currentRevision.ParentRevisionID != "" {
		prev, err := rt.getTextureRevision(ctx, ownerID, currentRevision.ParentRevisionID)
		if err == nil {
			previousRevision = &prev
		}
	}

	var ownerDirections []textureOwnerDirection
	if rt.textureOwnerDirectionsOn(ctx, ownerID, doc.ComputerID) {
		ownerDirections = collectTextureOwnerDirections(currentRevision, previousRevision, func(revisionID string) (types.Revision, error) {
			return rt.getTextureRevision(ctx, ownerID, revisionID)
		})
	}

	diffSummary := ""
	if previousRevision != nil {
		if diff, err := rt.Store.GetDiff(ctx, previousRevision.RevisionID, currentRevision.RevisionID, ownerID); err == nil {
			diffSummary = summarizeDiffResult(diff)
		}
	}

	workerWake := scheduledMessageSeq > 0 || strings.HasPrefix(strings.TrimSpace(req.Intent), "integrate_")
	currentSources := decodeTextureSourceEntities(currentRevision.SourceEntities)
	mediaSourceEntities, addedMediaSourceEntities, err := rt.registerTextureMediaSourceEntities(ctx, ownerID, currentRevision.Content, currentSources)
	if err != nil {
		return nil, err
	}
	sourceEntities, changedSourceEntities := normalizeTextureSourceEntities(metadata, mediaSourceEntities)
	evidenceEntities, sourceRejections := rt.evidenceSourceEntitiesFromCommitmentRecords(ctx, ownerID, strings.TrimSpace(doc.ComputerID), currentTextureAgentID(doc.DocID), 12)
	if len(evidenceEntities) > 0 {
		var changedEvidenceEntities bool
		sourceEntities, changedEvidenceEntities = mergeTextureSourceEntities(sourceEntities, evidenceEntities)
		changedSourceEntities = changedSourceEntities || changedEvidenceEntities
	}
	mergeCoagentSourceRejectionsIntoMetadata(metadata, sourceRejections)
	if len(sourceEntities) > 0 {
		metadata[textureAvailableSourceEntitiesKey] = sourceEntities
		if changedSourceEntities || addedMediaSourceEntities {
			delete(metadata, "media_source_research_required")
		}
	}
	if len(req.SourceEntities) > 0 {
		sourceEntities, changedSourceEntities := mergeTextureSourceEntities(decodeAvailableTextureSourceEntities(metadata), req.SourceEntities)
		if len(sourceEntities) > 0 {
			metadata[textureAvailableSourceEntitiesKey] = sourceEntities
			if changedSourceEntities {
				delete(metadata, "media_source_research_required")
			}
		}
	}

	contextMode := textureAgentRevisionContextMode(currentRevision, previousRevision)
	agentPrompt := buildAgentRevisionRequest(currentRevision, previousRevision, metadata, req, diffSummary, ownerDirections)

	// Create the runtime run with Texture agent revision metadata.
	// Carry forward durable context keys from the current head revision
	// so they survive into appagent revision metadata.
	runMetadata := map[string]any{
		"type":                 textureAgentRevisionTaskType,
		"agent_profile":        agentprofile.Texture,
		"agent_role":           agentprofile.Texture,
		"agent_id":             currentTextureAgentID(doc.DocID),
		"channel_id":           doc.DocID,
		"doc_id":               doc.DocID,
		"current_revision_id":  doc.CurrentRevisionID,
		"current_author_kind":  string(currentRevision.AuthorKind),
		"request_intent":       requestIntent,
		"original_prompt":      strings.TrimSpace(req.Prompt),
		"texture_context_mode": contextMode,
		"texture_prompt_chars": len(agentPrompt),
	}
	if trajectoryID := strings.TrimSpace(doc.TrajectoryID); trajectoryID != "" {
		runMetadata["trajectory_id"] = trajectoryID
		runMetadata["lifecycle_trajectory_id"] = trajectoryID
		snapshot, snapshotErr := rt.Store.GetLifecycleSnapshot(ctx, ownerID, strings.TrimSpace(doc.ComputerID), trajectoryID)
		if snapshotErr != nil {
			return nil, fmt.Errorf("load lifecycle work for Texture revision: %w", snapshotErr)
		}
		for _, work := range snapshot.WorkItems {
			if work.Status == types.WorkItemOpen && work.AssignedAgentID == currentTextureAgentID(doc.DocID) {
				runMetadata["lifecycle_work_item_id"] = work.WorkItemID
				break
			}
		}
		// A turn started for unread records needs no open work item: the
		// records authorize it (Cut 1). Any other turn still needs one.
		if req.Provenance["request_source"] == "inbox_record" {
			runMetadata[textureRecordsRouteMetadata] = true
		} else if _, ok := runMetadata["lifecycle_work_item_id"]; !ok {
			return nil, fmt.Errorf("load lifecycle work for Texture revision: %w", errTextureLifecycleOpenWorkUnavailable)
		}
	}
	if rt != nil && rt.Core != nil && rt.Core.TextureActorParkIdle() > 0 {
		runMetadata["actor_park_on_idle"] = true
		runMetadata["actor_park_idle_seconds"] = int((rt.Core.TextureActorParkIdle() + time.Second - 1) / time.Second)
	}
	if rt != nil {
		if sourceRunID, ok, err := rt.Core.LatestTextureActorMemorySourceRun(ctx, ownerID, currentTextureAgentID(doc.DocID)); err != nil {
			log.Printf("texture api: load actor memory source for doc %s: %v", doc.DocID, err)
		} else if ok {
			runMetadata["actor_rewarm_source_loop_id"] = sourceRunID
		}
	}
	if scheduledMessageSeq > 0 {
		runMetadata["scheduled_message_seq"] = scheduledMessageSeq
	}
	// Integrate wakes carry pending coagent findings. Mark the run so runtime
	// appends those findings as the first durable mailbox turn in run memory,
	// matching the warm-injection contract instead of relying on prompt-prefix
	// reconstruction.
	if workerWake {
		runMetadata["request_source"] = "update_coagent"
	}
	for _, key := range durableMetadataKeys {
		if val, ok := metadata[key]; ok && val != nil && val != "" {
			runMetadata[key] = val
		}
	}
	for key, val := range req.Provenance {
		if val != nil && val != "" {
			runMetadata[key] = val
		}
	}
	if requestedByRunID := strings.TrimSpace(req.RequestedByRunID); requestedByRunID != "" {
		runMetadata["requested_by_run_id"] = requestedByRunID
	}
	promoteCanonicalTextureSourcePath(runMetadata, metadata)
	initialPromptText := metadataString(metadata, "seed_prompt") + " " + req.Prompt
	if decision, ok := explicitNoWorkerDecisionRequestFromPrompt(initialPromptText); ok && scheduledMessageSeq == 0 {
		runMetadata["texture_initial_decision_required"] = true
		runMetadata["texture_initial_decision_kind"] = decision.DecisionKind
		runMetadata["texture_initial_decision_reason"] = decision.Reason
		runMetadata["texture_initial_decision_evidence_refs"] = decision.EvidenceRefs
		runMetadata["texture_initial_decision_next_action"] = decision.NextAction
	}
	rec, err := rt.Core.StartRunWithMetadata(ctx, agentPrompt, ownerID, runMetadata)
	if err != nil {
		if errors.Is(err, store.ErrLifecycleInvalidTransition) {
			// The initial_dispatch wake from StartLifecycle can fire reconcileAgentWakeLocked
			// before EnsureTextureHandoff's own submit lands. When that happens the agent
			// already carries ActiveRunID for this trajectory — treat the committed run as
			// the activation instead of surfacing a lifecycle invalid-transition 500.
			if recovered := rt.recoverRacedTextureActivation(ctx, ownerID, doc); recovered != nil {
				log.Printf("texture api: recovered raced activation for doc %s run %s", doc.DocID, recovered.RunID)
				return recovered, nil
			}
		}
		return nil, err
	}
	// The adapter's checked dispatch wrapper records the synchronous SQLite
	// append result from StartRun's unique initial occurrence.
	if err := rt.Core.CheckedActorDispatchError("initial_dispatch", rec.RunID); err != nil {
		return nil, fmt.Errorf("persist Texture initial dispatch: %w", err)
	}

	// Emit the texture-specific agent revision started event.
	startedPayload, _ := json.Marshal(map[string]string{
		"doc_id":  doc.DocID,
		"loop_id": rec.RunID,
	})
	rt.emitTextureAgentEvent(ctx, rec, types.EventTextureAgentRevisionStarted,
		events.CauseTaskLifecycle, startedPayload)

	return rec, nil
}

// recoverRacedTextureActivation returns the already-committed active run when a
// concurrent activation (the initial_dispatch wake racing EnsureTextureHandoff)
// committed the same trajectory's activation first. The winner's run already
// carries agent.ActiveRunID; loading and returning it makes the second caller
// idempotent instead of surfacing lifecycle invalid transition.
func (rt *Handler) recoverRacedTextureActivation(ctx context.Context, ownerID string, doc types.Document) *types.RunRecord {
	if rt == nil || rt.Store == nil {
		return nil
	}
	computerID := strings.TrimSpace(doc.ComputerID)
	agentID := currentTextureAgentID(doc.DocID)
	agent, err := rt.Store.GetAgentByScope(ctx, ownerID, computerID, agentID)
	if err != nil || strings.TrimSpace(agent.ActiveRunID) == "" {
		return nil
	}
	run, err := rt.Store.GetLifecycleRun(ctx, ownerID, computerID, agent.ActiveRunID)
	if err != nil || !run.State.Active() {
		return nil
	}
	return &run
}

func textureHardRequirementHints(parts ...string) []string {
	seen := make(map[string]bool)
	var out []string
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			return
		}
		seen[value] = true
		out = append(out, value)
	}
	for _, part := range parts {
		text := strings.TrimSpace(part)
		if text == "" {
			continue
		}
		for _, match := range textureMarkerLineRE.FindAllString(text, -1) {
			add("Preserve exact marker line: " + truncatePromptSnippet(match, 180))
		}
		for _, match := range textureSectionUpdatePrefixRE.FindAllString(text, -1) {
			add("Required sentence prefix: " + strings.Join(strings.Fields(match), " "))
		}
		for _, match := range textureNumberedHeadingRE.FindAllStringSubmatch(text, -1) {
			if len(match) > 1 {
				add("Required numbered heading: " + strings.TrimSpace(match[1]))
			}
		}
		for _, match := range textureSHA256RequirementRE.FindAllString(text, -1) {
			add("Required hash/value: " + match)
		}
		for _, label := range []string{"[S1]", "[S2]", "[S3]"} {
			if strings.Contains(text, label) {
				add("Required evidence label: " + label)
			}
		}
		if strings.Contains(text, "[CMD]") {
			add("Final command evidence label: [CMD] (final-only: include it only after a super delivery reports command evidence or a precise execution blocker; do not use it for initial scaffolds, pending source ledger rows, requested state, target hashes, or placeholders)")
		}
	}
	if len(out) > 32 {
		return out[:32]
	}
	return out
}

func textureAgentRevisionContextMode(current types.Revision, previous *types.Revision) string {
	if textureUseFocusedUserEditContext(current, previous) {
		return "focused_user_edit_diff"
	}
	return "current_head_plus_user_edit_diff"
}

func textureUseFocusedUserEditContext(current types.Revision, previous *types.Revision) bool {
	return current.AuthorKind == types.AuthorUser && previous != nil && len(current.Content) >= 12000
}

// buildAgentRevisionRequest is the first user turn of a Texture run: why it
// woke and what the owner asked. The document itself and the records past
// the cursor arrive as the frame turn right after it (Cut 2 2a), so this
// turn carries no document text, outline or source list.
func buildAgentRevisionRequest(current types.Revision, previous *types.Revision, metadata map[string]any, req textureAgentRevisionRequest, diffSummary string, ownerDirections []textureOwnerDirection) string {
	var b strings.Builder
	intent := strings.TrimSpace(req.Intent)
	if intent == "" {
		intent = "revise"
	}
	b.WriteString("Your document woke you. Intent: " + intent + ". Your frame follows: the document as blocks b1..bN and the records addressed to you.")
	if seedPrompt := metadataString(metadata, "seed_prompt"); seedPrompt != "" {
		if len(ownerDirections) > 0 {
			b.WriteString("\n\nThe owner's original request (the owner's later directions below override it wherever they differ):\n")
		} else {
			b.WriteString("\n\nThe owner's request:\n")
		}
		b.WriteString(seedPrompt)
	}
	if len(ownerDirections) > 0 {
		writeTextureOwnerDirections(&b, ownerDirections)
	}
	if ownerPrompt := metadataString(metadata, "owner_prompt"); ownerPrompt != "" {
		b.WriteString("\n\nThe owner's direction for this revision:\n")
		b.WriteString(ownerPrompt)
	}
	if promptUnixTS := metadataIntValue(metadata, textureMetadataPromptUnixTS); promptUnixTS > 0 {
		referenceTime := time.Unix(int64(promptUnixTS), 0).UTC()
		b.WriteString("\n\nOwner prompt reference time: ")
		b.WriteString(referenceTime.Format(time.RFC3339))
		b.WriteString(fmt.Sprintf(" UTC (Unix %d). Treat this as authoritative \"now\" when interpreting relative time words such as \"today\", \"tomorrow\", or \"this week\".", promptUnixTS))
	}
	if legacyPrompt := strings.TrimSpace(req.Prompt); legacyPrompt != "" {
		b.WriteString("\n\nAdditional instruction from the owner:\n")
		b.WriteString(legacyPrompt)
	}
	if sourcePath := metadataString(metadata, "source_path"); sourcePath != "" {
		b.WriteString("\n\nSource path: " + sourcePath + ". Preserve the file-backed structure while producing the next version.")
	}
	if current.AuthorKind == types.AuthorUser {
		// Only a seed request (an owner revision with no parent) is the
		// request itself. An owner /revise carries the whole story forward
		// as a child of the head and is stamped user_prompt too; for it the
		// sentence told Texture to throw a finished story away.
		if metadataStringValue(metadata, "input_origin") == textureInputOriginUserPrompt && strings.TrimSpace(current.ParentRevisionID) == "" {
			b.WriteString("\n\nThe current revision is the owner's request itself, not prose to keep: write the first useful document that answers it.")
		} else {
			b.WriteString("\n\nThe owner edited the document. Read the edit as the instruction: replace text the owner replaced, act on instructions they wrote inline, and keep what they wrote as prose.")
		}
	}
	if diffSummary != "" {
		if current.AuthorKind == types.AuthorUser {
			b.WriteString("\n\nThe owner's edit, from the previous revision:\n")
		} else {
			b.WriteString("\n\nThe last change:\n")
		}
		b.WriteString(diffSummary)
	}
	if hard := textureFilterFinalCommandEvidenceRequirements(textureHardRequirementHints(metadataString(metadata, "seed_prompt"), req.Prompt, current.Content)); len(hard) > 0 {
		b.WriteString("\n\nKeep these exactly in every revision unless the owner changes them:\n")
		for _, requirement := range hard {
			b.WriteString("- " + requirement + "\n")
		}
	}
	return b.String()
}

func textureFilterFinalCommandEvidenceRequirements(requirements []string) []string {
	if len(requirements) == 0 {
		return requirements
	}
	filtered := requirements[:0]
	for _, requirement := range requirements {
		if strings.HasPrefix(requirement, "Final command evidence label: [CMD]") {
			continue
		}
		filtered = append(filtered, requirement)
	}
	return filtered
}

func truncatePromptSnippet(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 || len(value) <= limit {
		return value
	}
	if limit <= 3 {
		return cutAtRuneBoundary(value, limit)
	}
	return cutAtRuneBoundary(value, limit-3) + "..."
}

// cutAtRuneBoundary returns the longest prefix of value of at most n bytes
// that does not split a rune.
func cutAtRuneBoundary(value string, n int) string {
	for n > 0 && n < len(value) && !utf8.RuneStart(value[n]) {
		n--
	}
	return value[:n]
}

func decodeRevisionMetadata(raw json.RawMessage) map[string]any {
	if len(raw) == 0 {
		return nil
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil
	}
	return out
}

func metadataString(metadata map[string]any, key string) string {
	if metadata == nil {
		return ""
	}
	value, _ := metadata[key].(string)
	return strings.TrimSpace(value)
}

func summarizeDiffResult(diff types.DiffResult) string {
	if len(diff.Sections) == 0 {
		return "No line-level changes were detected."
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Added lines: %d. Removed lines: %d.", diff.AddedLines, diff.RemovedLines))
	changesShown := 0
	for _, section := range diff.Sections {
		if section.Type == "unchanged" {
			continue
		}
		if changesShown >= 4 {
			b.WriteString("\n- Additional changed sections omitted for brevity.")
			break
		}
		var snippet string
		switch section.Type {
		case "added":
			snippet = strings.TrimSpace(section.ToContent)
		case "removed":
			snippet = strings.TrimSpace(section.FromContent)
		default:
			snippet = strings.TrimSpace(section.ToContent)
			if snippet == "" {
				snippet = strings.TrimSpace(section.FromContent)
			}
		}
		if snippet == "" {
			snippet = "(empty change block)"
		}
		if len(snippet) > 240 {
			snippet = snippet[:240] + "..."
		}
		b.WriteString("\n- ")
		b.WriteString(section.Type)
		b.WriteString(": ")
		b.WriteString(snippet)
		changesShown++
	}
	return b.String()
}

// emitTextureAgentEvent is a helper that emits an texture-specific agent revision
// event, carrying the doc_id in the payload so the frontend can correlate
// progress to the open document (VAL-ETEXT-004).
func (rt *Handler) emitTextureAgentEvent(ctx context.Context, rec *types.RunRecord, kind types.EventKind, cause events.EventCause, payload json.RawMessage) {
	evRec := &types.EventRecord{
		EventID:      uuid.New().String(),
		RunID:        rec.RunID,
		AgentID:      rec.AgentID,
		ChannelID:    rec.ChannelID,
		OwnerID:      rec.OwnerID,
		TrajectoryID: trajectoryIDForRun(rec),
		Timestamp:    time.Now().UTC(),
		Kind:         kind,
		Payload:      payload,
	}
	if err := rt.Store.AppendEvent(ctx, evRec); err != nil {
		log.Printf("runtime: persist texture agent event: %v", err)
		return
	}
	rt.Bus.Publish(events.RuntimeEvent{
		Record: *evRec,
		Actor:  events.ActorRuntime,
		Cause:  cause,
	})
}

func (rt *Handler) emitTextureDocumentRevisionEvent(ctx context.Context, ownerID string, rev types.Revision) {
	payload, err := json.Marshal(map[string]string{
		"doc_id":              rev.DocID,
		"revision_id":         rev.RevisionID,
		"current_revision_id": rev.RevisionID,
	})
	if err != nil {
		log.Printf("runtime: marshal texture document revision event: %v", err)
		return
	}
	evRec := &types.EventRecord{
		EventID:   uuid.New().String(),
		OwnerID:   ownerID,
		Timestamp: time.Now().UTC(),
		Kind:      types.EventTextureDocumentRevisionCreated,
		Payload:   payload,
	}
	if err := rt.Store.AppendEvent(ctx, evRec); err != nil {
		log.Printf("runtime: persist texture document revision event: %v", err)
		return
	}
	rt.Bus.Publish(events.RuntimeEvent{
		Record: *evRec,
		Actor:  events.ActorRuntime,
		Cause:  events.CauseTaskLifecycle,
	})
}

func (rt *Handler) emitTextureDocumentRevisionEventForRun(ctx context.Context, rec *types.RunRecord, rev types.Revision) {
	if rec == nil {
		rt.emitTextureDocumentRevisionEvent(ctx, rev.OwnerID, rev)
		return
	}
	payload, err := json.Marshal(map[string]string{
		"doc_id":              rev.DocID,
		"revision_id":         rev.RevisionID,
		"current_revision_id": rev.RevisionID,
		"loop_id":             rec.RunID,
	})
	if err != nil {
		log.Printf("runtime: marshal texture document revision event: %v", err)
		return
	}
	evRec := &types.EventRecord{
		EventID:      uuid.New().String(),
		RunID:        rec.RunID,
		AgentID:      rec.AgentID,
		ChannelID:    rev.DocID,
		OwnerID:      rec.OwnerID,
		TrajectoryID: trajectoryIDForRun(rec),
		Timestamp:    time.Now().UTC(),
		Kind:         types.EventTextureDocumentRevisionCreated,
		Payload:      payload,
	}
	if err := rt.Store.AppendEvent(ctx, evRec); err != nil {
		log.Printf("runtime: persist texture document revision event: %v", err)
		return
	}
	rt.Bus.Publish(events.RuntimeEvent{
		Record: *evRec,
		Actor:  events.ActorRuntime,
		Cause:  events.CauseTaskLifecycle,
	})
}

func (rt *Handler) emitTextureDecisionRecordedEvent(ctx context.Context, rec *types.RunRecord, decision types.TextureDecisionRecord) {
	payload, err := json.Marshal(map[string]any{
		"decision_id":   decision.DecisionID,
		"doc_id":        decision.DocID,
		"loop_id":       decision.RunID,
		"trajectory_id": decision.TrajectoryID,
		"actor_id":      decision.ActorID,
		"decision_kind": decision.DecisionKind,
		"reason":        decision.Reason,
		"evidence_refs": decision.EvidenceRefs,
		"next_action":   decision.NextAction,
	})
	if err != nil {
		log.Printf("runtime: marshal Texture decision event: %v", err)
		return
	}
	if rec != nil {
		rt.emitTextureAgentEvent(ctx, rec, types.EventTextureDecisionRecorded, events.CauseToolExecution, payload)
		return
	}
	evRec := &types.EventRecord{
		EventID:      uuid.New().String(),
		RunID:        decision.RunID,
		AgentID:      decision.ActorID,
		ChannelID:    decision.DocID,
		OwnerID:      decision.OwnerID,
		TrajectoryID: decision.TrajectoryID,
		Timestamp:    decision.CreatedAt,
		Kind:         types.EventTextureDecisionRecorded,
		Payload:      payload,
	}
	if err := rt.Store.AppendEvent(ctx, evRec); err != nil {
		log.Printf("runtime: persist Texture decision event: %v", err)
		return
	}
	rt.Bus.Publish(events.RuntimeEvent{
		Record: *evRec,
		Actor:  events.ActorRuntime,
		Cause:  events.CauseToolExecution,
	})
}
