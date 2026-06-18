package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/runtime/textureprompts"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// textureAgentRevisionRequest is the JSON payload for
// POST /api/texture/documents/{id}/revise.
// Submitting a natural-language revision request from within an open document
// creates a new canonical revision attributable to the appagent
// (VAL-ETEXT-003).
type textureAgentRevisionRequest struct {
	Intent string `json:"intent,omitempty"`
	Prompt string `json:"prompt,omitempty"`
}

// textureAgentRevisionResponse is the JSON response for agent revision
// submission. It returns the stable task handle so runtime/trace surfaces can
// correlate the mutation even though the editor now follows the document stream
// instead of polling the run directly (VAL-ETEXT-004).
type textureAgentRevisionResponse struct {
	RunID     string         `json:"loop_id"`
	DocID     string         `json:"doc_id"`
	State     types.RunState `json:"state"`
	CreatedAt string         `json:"created_at"`
}

type textureCancelRevisionResponse struct {
	DocID           string   `json:"doc_id"`
	RunID           string   `json:"loop_id,omitempty"`
	Status          string   `json:"status"`
	CancelledRunIDs []string `json:"cancelled_loop_ids,omitempty"`
	Resumable       bool     `json:"resumable"`
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
func (h *APIHandler) HandleTextureAgentRevision(w http.ResponseWriter, r *http.Request) {
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
	doc, err := h.rt.Store().GetDocument(r.Context(), docID, ownerID)
	if err != nil {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "document not found"})
		return
	}

	// Check for an existing pending agent mutation on this document.
	// If one exists, return the existing run ID instead of creating a new
	// mutation. This prevents duplicate canonical revisions when
	// renewal/retry occurs mid-mutation (VAL-CROSS-122).
	existing, err := h.pendingAgentMutationByDoc(r.Context(), docID, ownerID)
	if err != nil {
		log.Printf("texture api: check pending mutation: %v", err)
	} else if existing != nil {
		// A Texture actor is already resident (running or parked) for this
		// document. Deliver the owner's new request to that actor as an
		// addressed update and wake it, instead of silently dropping the prompt.
		// This preserves the "no lost foreground updates" invariant now that a
		// long-running actor is pending for most of its life. The same run ID is
		// returned because the owner's request is handled by the resident actor,
		// not a new run.
		if err := h.rt.deliverOwnerRevisionToTextureActor(r.Context(), doc, ownerID, req); err != nil {
			log.Printf("texture api: deliver owner revision to resident texture actor %s: %v", existing.RunID, err)
			writeAPIJSON(w, http.StatusConflict, apiError{Error: "a revision is already in progress for this document; please retry shortly"})
			return
		}
		writeAPIJSON(w, http.StatusAccepted, textureAgentRevisionResponse{
			RunID:     existing.RunID,
			DocID:     docID,
			State:     types.RunPending,
			CreatedAt: existing.CreatedAt.Format("2006-01-02T15:04:05.000Z"),
		})
		return
	}

	rec, err := h.rt.submitTextureAgentRevisionRun(r.Context(), doc, ownerID, req, 0)
	if err != nil {
		log.Printf("texture api: submit agent revision run: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to submit agent revision"})
		return
	}

	writeAPIJSON(w, http.StatusAccepted, textureAgentRevisionResponse{
		RunID:     rec.RunID,
		DocID:     docID,
		State:     rec.State,
		CreatedAt: rec.CreatedAt.Format("2006-01-02T15:04:05.000Z"),
	})
}

// HandleTextureCancelAgentRevision handles POST
// /api/texture/documents/{id}/cancel. It cancels the pending Texture appagent
// revision trajectory without changing the canonical document head.
func (h *APIHandler) HandleTextureCancelAgentRevision(w http.ResponseWriter, r *http.Request) {
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
	if _, err := h.rt.Store().GetDocument(r.Context(), docID, ownerID); err != nil {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "document not found"})
		return
	}
	mutation, err := h.pendingAgentMutationByDoc(r.Context(), docID, ownerID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeAPIJSON(w, http.StatusOK, textureCancelRevisionResponse{DocID: docID, Status: "no_pending_revision", Resumable: true})
			return
		}
		log.Printf("texture api: get pending mutation for cancel: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to load pending revision"})
		return
	}
	if mutation == nil {
		writeAPIJSON(w, http.StatusOK, textureCancelRevisionResponse{DocID: docID, Status: "no_pending_revision", Resumable: true})
		return
	}
	cancelled, err := h.rt.CancelRunTrajectory(r.Context(), mutation.RunID, ownerID)
	if err != nil {
		log.Printf("texture api: cancel revision trajectory: %v", err)
		writeAPIJSON(w, http.StatusConflict, apiError{Error: err.Error()})
		return
	}
	if err := h.rt.Store().CancelAgentMutation(r.Context(), mutation.RunID); err != nil {
		log.Printf("texture api: mark mutation cancelled: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to record cancellation"})
		return
	}
	writeAPIJSON(w, http.StatusOK, textureCancelRevisionResponse{
		DocID:           docID,
		RunID:           mutation.RunID,
		Status:          "cancelled",
		CancelledRunIDs: cancelled,
		Resumable:       true,
	})
}

func (h *APIHandler) pendingAgentMutationByDoc(ctx context.Context, docID, ownerID string) (*store.AgentMutation, error) {
	mutation, err := h.rt.Store().GetPendingAgentMutationByDoc(ctx, docID, ownerID)
	if err != nil || mutation == nil {
		return mutation, err
	}
	run, runErr := h.rt.GetRun(ctx, mutation.RunID, ownerID)
	if runErr == nil && !run.State.Terminal() {
		// The Texture actor is still resident (running or parked). A long-running
		// actor keeps its single pending mutation across many canonical revisions,
		// so it must not be reconciled/completed here. Completing a live actor's
		// mutation would lock it out of further writes (its next write fails the
		// pending-state gate in commitTextureToolEdit). Reconcile is cleanup for
		// terminal/missing runs only.
		return mutation, nil
	}
	// The run is terminal or unresolvable. Clean up its pending mutation: if the
	// canonical head is the appagent write this run produced, complete the
	// mutation; otherwise (for a confirmed terminal run) mark it stale so a fresh
	// revision can start.
	reconciled, reconcileErr := h.reconcilePendingMutationFromDocumentHead(ctx, mutation)
	if reconcileErr != nil {
		log.Printf("texture api: reconcile pending mutation %s from document head: %v", mutation.RunID, reconcileErr)
	} else if reconciled {
		return nil, nil
	}
	if runErr != nil {
		// Could not resolve the run; leave the pending mutation as-is rather than
		// marking a possibly-transient lookup failure stale.
		return mutation, nil
	}
	if err := h.rt.Store().MarkAgentMutationStale(ctx, mutation.RunID); err != nil {
		log.Printf("texture api: mark stale pending mutation %s: %v", mutation.RunID, err)
		return mutation, nil
	}
	return nil, nil
}

func (h *APIHandler) reconcilePendingMutationFromDocumentHead(ctx context.Context, mutation *store.AgentMutation) (bool, error) {
	if mutation == nil || strings.TrimSpace(mutation.RunID) == "" || strings.TrimSpace(mutation.DocID) == "" || strings.TrimSpace(mutation.OwnerID) == "" {
		return false, nil
	}
	doc, err := h.rt.Store().GetDocument(ctx, mutation.DocID, mutation.OwnerID)
	if err != nil {
		return false, err
	}
	if strings.TrimSpace(doc.CurrentRevisionID) == "" {
		return false, nil
	}
	rev, err := h.rt.Store().GetRevision(ctx, doc.CurrentRevisionID, mutation.OwnerID)
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
	if err := h.rt.Store().CompleteAgentMutation(ctx, mutation.RunID, rev.RevisionID); err != nil && err != store.ErrMutationAlreadyCompleted {
		return false, err
	}
	return true, nil
}

func stableOwnerRevisionUpdateID(ownerID, docID, intent, prompt string) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{ownerID, docID, intent, prompt}, "\x00")))
	return "owner-rev-" + hex.EncodeToString(sum[:])[:20]
}

// deliverOwnerRevisionToTextureActor delivers an owner-originated revision
// request to the document's resident Texture actor as an addressed coagent
// update, then signals the actor so a parked run wakes. It reuses the same
// pending-update substrate that researcher/super deliveries use, so the warm
// injector folds the owner instruction into the running actor's context and it
// produces a new canonical revision. This is the foreground-update path for a
// long-running, mostly-parked Texture actor; without it a new /revise during a
// pending actor would be silently dropped.
func (rt *Runtime) deliverOwnerRevisionToTextureActor(ctx context.Context, doc types.Document, ownerID string, req textureAgentRevisionRequest) error {
	if rt == nil || rt.store == nil {
		return fmt.Errorf("runtime store unavailable")
	}
	targetAgentID := currentTextureAgentID(doc.DocID)
	if strings.TrimSpace(targetAgentID) == "" {
		return fmt.Errorf("texture agent id unavailable for doc %s", doc.DocID)
	}
	intent := strings.TrimSpace(req.Intent)
	prompt := strings.TrimSpace(req.Prompt)
	summary := "Owner revision request"
	if intent != "" {
		summary = "Owner revision request: " + intent
	}
	var notes []string
	if prompt != "" {
		notes = append(notes, prompt)
	}
	update := types.WorkerUpdateRecord{
		// Deterministic update id keyed on the request content so a renewal/retry
		// of the same owner request dedupes (DispatchWorkerUpdate is idempotent by
		// owner_id+update_id, preserving VAL-CROSS-122) while a genuinely new
		// owner instruction is delivered as a distinct packet.
		UpdateID:      stableOwnerRevisionUpdateID(ownerID, doc.DocID, intent, prompt),
		OwnerID:       ownerID,
		AgentID:       targetAgentID,
		TargetAgentID: targetAgentID,
		ChannelID:     doc.DocID,
		Role:          "owner",
		Kind:          "owner_revision_request",
		Summary:       summary,
		Notes:         notes,
		CreatedAt:     time.Now().UTC(),
	}
	update.Content = buildWorkerUpdateMessage(update)
	message := &types.ChannelMessage{
		ChannelID: doc.DocID,
		From:      "owner",
		ToAgentID: targetAgentID,
		Role:      update.Role,
		Content:   update.Content,
		Timestamp: update.CreatedAt,
	}
	stored, created, err := rt.store.DispatchWorkerUpdate(ctx, update, message)
	if err != nil {
		return fmt.Errorf("dispatch owner revision update: %w", err)
	}
	if created {
		rt.emitChannelMessageEvent(ctx, *message, ownerID)
		rt.wakeUpdatedCoagent(ctx, stored)
	}
	return nil
}

func (rt *Runtime) submitTextureAgentRevisionRun(ctx context.Context, doc types.Document, ownerID string, req textureAgentRevisionRequest, scheduledMessageSeq int64) (*types.RunRecord, error) {
	// Build the backend-owned Texture revision request from current document state.
	var currentRevision types.Revision
	var currentRevisionLoaded bool
	if doc.CurrentRevisionID != "" {
		rev, err := rt.Store().GetRevision(ctx, doc.CurrentRevisionID, ownerID)
		if err == nil {
			currentRevision = rev
			currentRevisionLoaded = true
		}
	}
	metadata := decodeRevisionMetadata(currentRevision.Metadata)
	if metadata == nil {
		metadata = map[string]any{}
	}
	var previousRevision *types.Revision
	if currentRevisionLoaded && currentRevision.ParentRevisionID != "" {
		prev, err := rt.Store().GetRevision(ctx, currentRevision.ParentRevisionID, ownerID)
		if err == nil {
			previousRevision = &prev
		}
	}

	diffSummary := ""
	if currentRevisionLoaded && previousRevision != nil {
		if diff, err := rt.Store().GetDiff(ctx, previousRevision.RevisionID, currentRevision.RevisionID, ownerID); err == nil {
			diffSummary = summarizeDiffResult(diff)
		}
	}

	workerWake := scheduledMessageSeq > 0 || strings.HasPrefix(strings.TrimSpace(req.Intent), "integrate_")
	hasGroundedHistory := false
	if workerWake {
		historyState, historyErr := rt.channelHasGroundedHistory(ctx, ownerID, doc.DocID, time.Time{})
		if historyErr != nil {
			log.Printf("texture api: check grounded history: %v", historyErr)
		} else {
			hasGroundedHistory = historyState
		}
	}

	var recentWorkerMessages []ChannelMessage
	if workerWake {
		var workerErr error
		recentWorkerMessages, workerErr = rt.recentWorkerMessages(ctx, ownerID, doc.DocID, 12)
		if workerErr != nil {
			log.Printf("texture api: recent worker messages: %v", workerErr)
		}
	}
	if currentRevisionLoaded {
		mediaSourceRefs, addedMediaSourceRefs := rt.registerTextureMediaSourceRefs(ctx, ownerID, currentRevision.Content, metadata)
		if len(mediaSourceRefs) > 0 {
			metadata["media_source_refs"] = mediaSourceRefs
			metadata["media_source_research_required"] = addedMediaSourceRefs
		}
		sourceEntities, changedSourceEntities := normalizeTextureSourceEntities(metadata, mediaSourceRefs)
		if workerSourceEntities := rt.sourceEntitiesFromWorkerMessages(ctx, ownerID, recentWorkerMessages); len(workerSourceEntities) > 0 {
			var changedWorkerSourceEntities bool
			sourceEntities, changedWorkerSourceEntities = mergeTextureSourceEntities(sourceEntities, workerSourceEntities)
			changedSourceEntities = changedSourceEntities || changedWorkerSourceEntities
		}
		if len(sourceEntities) > 0 {
			metadata["source_entities"] = sourceEntities
			if changedSourceEntities {
				if _, ok := metadata["media_source_research_required"]; !ok {
					metadata["media_source_research_required"] = addedMediaSourceRefs
				}
			}
		}
	}

	contextMode := textureAgentRevisionContextMode(currentRevision, previousRevision)
	agentPrompt := buildAgentRevisionRequest(currentRevision, previousRevision, metadata, req, diffSummary, hasGroundedHistory, nil, nil)

	// Create the runtime run with Texture agent revision metadata.
	// Carry forward durable context keys from the current head revision
	// so they survive into appagent revision metadata.
	runMetadata := map[string]any{
		"type":                 textureAgentRevisionTaskType,
		"agent_profile":        AgentProfileTexture,
		"agent_role":           AgentProfileTexture,
		"agent_id":             currentTextureAgentID(doc.DocID),
		"channel_id":           doc.DocID,
		"doc_id":               doc.DocID,
		"current_revision_id":  doc.CurrentRevisionID,
		"request_intent":       strings.TrimSpace(req.Intent),
		"original_prompt":      strings.TrimSpace(req.Prompt),
		"texture_context_mode": contextMode,
		"texture_prompt_chars": len(agentPrompt),
	}
	if rt != nil && rt.cfg.TextureActorParkIdle > 0 {
		runMetadata["actor_park_on_idle"] = true
		runMetadata["actor_park_idle_seconds"] = int((rt.cfg.TextureActorParkIdle + time.Second - 1) / time.Second)
	}
	if rt != nil {
		if spend, ok, err := rt.latestActorToolLoopBudgetSpend(ctx, ownerID, currentTextureAgentID(doc.DocID)); err != nil {
			log.Printf("texture api: load actor budget spend for doc %s: %v", doc.DocID, err)
		} else if ok {
			runMetadata["actor_rewarm_source_loop_id"] = spend.SourceRunID
			if spend.ProviderCalls > 0 {
				runMetadata["actor_budget_spent_provider_calls"] = spend.ProviderCalls
			}
			if spend.InputTokens > 0 {
				runMetadata["actor_budget_spent_input_tokens"] = spend.InputTokens
			}
			if spend.OutputTokens > 0 {
				runMetadata["actor_budget_spent_output_tokens"] = spend.OutputTokens
			}
			if spend.ObservedUsageEvent {
				runMetadata["actor_budget_spend_source"] = "tool_loop_budget_usage"
			} else {
				runMetadata["actor_budget_spend_source"] = "provider_call_events"
			}
		}
	}
	if scheduledMessageSeq > 0 {
		runMetadata["scheduled_message_seq"] = scheduledMessageSeq
	}
	// Integrate wakes carry pending coagent findings. Mark the run so the cold
	// packet prepend fires on the first inference turn (shouldPrependInitialCoagentUpdates),
	// matching the warm-injection contract instead of relying on a later end_turn
	// re-injection. See docs/texture-agentic-invariants-2026-06-13.md.
	if workerWake {
		runMetadata["request_source"] = "update_coagent"
	}
	for _, key := range durableMetadataKeys {
		if val, ok := metadata[key]; ok && val != nil && val != "" {
			runMetadata[key] = val
		}
	}
	promoteCanonicalTextureSourcePath(runMetadata, metadata)
	initialPromptText := metadataString(metadata, "seed_prompt") + " " + req.Prompt
	if decision, ok := explicitNoWorkerDecisionRequestFromPrompt(initialPromptText); ok && scheduledMessageSeq == 0 {
		runMetadata["texture_initial_decision_required"] = true
		runMetadata["texture_initial_decision_kind"] = decision.DecisionKind
		runMetadata["texture_initial_decision_reason"] = decision.Reason
		runMetadata["texture_initial_decision_evidence_refs"] = decision.EvidenceRefs
		runMetadata["texture_initial_decision_next_action"] = decision.NextAction
	} else if superRequest, ok := explicitTextureSuperExecutionRequestFromPrompt(initialPromptText); ok && scheduledMessageSeq == 0 {
		runMetadata["texture_initial_super_request_required"] = true
		runMetadata["texture_initial_super_request_objective"] = superRequest.Objective
		runMetadata["texture_initial_super_request_reason"] = superRequest.Reason
	}
	var (
		rec *types.RunRecord
		err error
	)
	rec, err = rt.StartRunWithMetadata(ctx, agentPrompt, ownerID, runMetadata)
	if err != nil {
		return nil, err
	}

	// Record the agent mutation for idempotency tracking (VAL-CROSS-122).
	if err := rt.Store().CreateAgentMutation(ctx, store.AgentMutation{
		DocID:               doc.DocID,
		RunID:               rec.RunID,
		OwnerID:             ownerID,
		State:               "pending",
		ScheduledMessageSeq: scheduledMessageSeq,
		CreatedAt:           time.Now().UTC(),
	}); err != nil {
		log.Printf("texture api: create agent mutation: %v", err)
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
		for _, match := range textureInlineSourceRefRE.FindAllString(text, -1) {
			add("Preserve inline source ref exactly: " + truncatePromptSnippet(match, 180))
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

// buildAgentRevisionRequest constructs the backend-owned Texture revision
// request sent as the user turn for the Texture appagent.
func buildAgentRevisionRequest(current types.Revision, previous *types.Revision, metadata map[string]any, req textureAgentRevisionRequest, diffSummary string, hasGroundedHistory bool, recentWorkerMessages []ChannelMessage, _ []string) string {
	var b strings.Builder
	ownerPromptRequestRevision := current.AuthorKind == types.AuthorUser && metadataStringValue(metadata, "input_origin") == textureInputOriginUserPrompt
	b.WriteString("A revise event was triggered for the current Texture document.")

	intent := strings.TrimSpace(req.Intent)
	if intent == "" {
		intent = "revise"
	}
	b.WriteString("\nIntent: ")
	b.WriteString(intent)
	b.WriteString(".")

	if seedPrompt := metadataString(metadata, "seed_prompt"); seedPrompt != "" {
		b.WriteString("\n\nOriginal user request:\n")
		b.WriteString(seedPrompt)
	}
	if promptUnixTS := metadataIntValue(metadata, textureMetadataPromptUnixTS); promptUnixTS > 0 {
		referenceTime := time.Unix(int64(promptUnixTS), 0).UTC()
		b.WriteString("\n\nOwner prompt reference time: ")
		b.WriteString(referenceTime.Format(time.RFC3339))
		b.WriteString(" UTC (Unix ")
		b.WriteString(fmt.Sprintf("%d", promptUnixTS))
		b.WriteString("). Treat this as authoritative \"now\" when interpreting relative time words such as \"today\", \"tomorrow\", or \"this week\".")
	}
	if legacyPrompt := strings.TrimSpace(req.Prompt); legacyPrompt != "" {
		b.WriteString("\n\nAdditional user instruction:\n")
		b.WriteString(legacyPrompt)
	}
	if sourcePath := metadataString(metadata, "source_path"); sourcePath != "" {
		b.WriteString("\n\nSource path: ")
		b.WriteString(sourcePath)
		b.WriteString(". Preserve the file-backed structure while producing the next version.")
	}
	if conductorLoopID := metadataString(metadata, "conductor_loop_id"); conductorLoopID != "" {
		b.WriteString("\nConductor loop: ")
		b.WriteString(conductorLoopID)
		b.WriteString(".")
	}
	mediaSourceRefs := decodeTextureMediaSourceRefs(metadata["media_source_refs"])
	if formattedRefs := formatTextureMediaSourceRefsForPrompt(mediaSourceRefs); formattedRefs != "" {
		b.WriteString("\n\nDetected durable media source refs:\n")
		b.WriteString(formattedRefs)
		b.WriteString(textureprompts.RevisionMediaSourceRefsIntro())
		if metadataBoolValue(metadata, "media_source_research_required") {
			b.WriteString(textureprompts.RevisionMediaSourceResearchRequired())
		}
	}
	sourceEntities := decodeTextureSourceEntities(metadata["source_entities"])
	if formattedEntities := formatTextureSourceEntitiesForPrompt(sourceEntities); formattedEntities != "" {
		b.WriteString("\n\nDetected Texture source entities:\n")
		b.WriteString(formattedEntities)
		b.WriteString(textureprompts.RevisionSourceEntitiesIntro())
	}
	if current.RevisionID != "" {
		b.WriteString("\n\nCurrent head revision: ")
		b.WriteString(current.RevisionID)
		b.WriteString(" (")
		b.WriteString(string(current.AuthorKind))
		b.WriteString(" by ")
		b.WriteString(current.AuthorLabel)
		b.WriteString(").")
	}
	if previous != nil {
		b.WriteString("\nPrevious revision: ")
		b.WriteString(previous.RevisionID)
		b.WriteString(".")
	}
	if diffSummary != "" {
		if current.AuthorKind == types.AuthorUser {
			b.WriteString("\n\nUser edit diff from previous canonical revision to current user-authored draft:\n")
		} else {
			b.WriteString("\n\nLatest revision diff/context:\n")
		}
		b.WriteString(diffSummary)
	}
	if len(recentWorkerMessages) > 0 {
		b.WriteString("\n\nRecent addressed worker messages:\n")
		for _, message := range recentWorkerMessages {
			b.WriteString("- [")
			if !message.Timestamp.IsZero() {
				b.WriteString(message.Timestamp.UTC().Format(time.RFC3339))
			} else {
				b.WriteString("unknown-time")
			}
			b.WriteString("] ")
			if role := strings.TrimSpace(message.Role); role != "" {
				b.WriteString(role)
			} else {
				b.WriteString("worker")
			}
			if from := strings.TrimSpace(message.From); from != "" {
				b.WriteString(" ")
				b.WriteString(from)
			}
			b.WriteString(": ")
			b.WriteString(truncatePromptSnippet(message.Content, 800))
			b.WriteString("\n")
		}
		seedAndPrompt := metadataString(metadata, "seed_prompt") + " " + req.Prompt
		b.WriteString(textureprompts.RevisionWorkerFindingsOverlay(textureprompts.RevisionWorkerFindingsOptions{
			IntegrateWorkerFindings: strings.EqualFold(intent, "integrate_worker_findings") && !texturePromptNeedsSuperExecution(seedAndPrompt),
			NeedsSuperExecution:     texturePromptNeedsSuperExecution(seedAndPrompt),
			HasSuperDelivery:        textureWorkerMessagesContainRole(recentWorkerMessages, AgentProfileSuper),
			ActiveWorkerDelegation:  workerMessagesContainActiveDelegation(recentWorkerMessages),
		}))
	}
	if textureUseFocusedUserEditContext(current, previous) {
		b.WriteString("\n\nFocused current-head context for this long user-authored draft:\n---\n")
		b.WriteString(summarizeFocusedUserEditContext(current, previous))
		b.WriteString("\n---\n")
		b.WriteString("\nThe complete current document is intentionally not preloaded in this ordinary long-document revise turn. Use the exact changed regions above and the user edit diff to call apply_edits against the current base revision. Retrieve prior versions, metadata, or broader document context only when the edit cannot be safely resolved from the changed regions.")
	} else {
		b.WriteString("\n\nCurrent canonical document content:\n---\n")
		if current.Content != "" {
			b.WriteString(current.Content)
		} else {
			b.WriteString("(empty document)")
		}
		b.WriteString("\n---\n")
	}
	hardRequirements := textureHardRequirementHints(metadataString(metadata, "seed_prompt"), req.Prompt, current.Content)
	hasSuperDelivery := textureWorkerMessagesContainRole(recentWorkerMessages, AgentProfileSuper)
	if !hasSuperDelivery {
		hardRequirements = textureFilterFinalCommandEvidenceRequirements(hardRequirements)
		if strings.Contains(metadataString(metadata, "seed_prompt")+req.Prompt+current.Content, "[CMD]") {
			hardRequirements = append(hardRequirements, "Pending command evidence rule: before a super delivery exists, do not include a Source Ledger row, status row, or placeholder whose label is [CMD]; describe command evidence as pending without that label.")
		}
	}
	if len(hardRequirements) > 0 {
		b.WriteString("\nHard requirements checklist for the next canonical revision:\n")
		for _, requirement := range hardRequirements {
			b.WriteString("- ")
			b.WriteString(requirement)
			b.WriteString("\n")
		}
		b.WriteString("Treat this checklist as acceptance criteria for any rewrite_texture call; preserve these prefixes, labels, values, and headings verbatim unless the user explicitly changed them.\n")
	}
	b.WriteString(textureprompts.RevisionPolicyOverlay(textureprompts.RevisionPolicyOptions{
		OwnerPromptRequestRevision: ownerPromptRequestRevision,
		UserAuthoredRevision:       current.AuthorKind == types.AuthorUser,
		ExplicitResearcherRequest:  metadataBoolValue(metadata, runMetadataExplicitResearcher) || texturePromptExplicitlyRequestsResearcher(metadataString(metadata, "seed_prompt")+" "+req.Prompt),
		HasGroundedHistory:         hasGroundedHistory,
		DocID:                      current.DocID,
		RevisionID:                 current.RevisionID,
	}))
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

func textureWorkerMessagesContainRole(messages []ChannelMessage, role string) bool {
	role = strings.TrimSpace(role)
	if role == "" {
		return false
	}
	for _, message := range messages {
		if strings.EqualFold(strings.TrimSpace(message.Role), role) {
			return true
		}
	}
	return false
}

func workerMessagesContainActiveDelegation(messages []ChannelMessage) bool {
	for _, message := range messages {
		content := strings.ToLower(message.Content)
		for _, marker := range []string{
			"worker_run_active",
			"finish_ready=false",
			"finish_ready: false",
			"active_worker_obligation=true",
			"runtime supervision continuation required",
			"missing_terminal_evidence",
		} {
			if strings.Contains(content, marker) {
				return true
			}
		}
	}
	return false
}

func (rt *Runtime) recentWorkerMessages(ctx context.Context, ownerID, channelID string, limit int) ([]ChannelMessage, error) {
	if limit <= 0 {
		limit = 12
	}
	messages, err := rt.Store().ListChannelMessages(ctx, ownerID, channelID, 0, 200)
	if err != nil {
		return nil, err
	}
	runs, err := rt.Store().ListRunsByChannel(ctx, ownerID, channelID, 200)
	if err != nil {
		return nil, err
	}
	runProfiles := make(map[string]string, len(runs))
	for _, run := range runs {
		runProfiles[run.RunID] = agentProfileForRun(&run)
	}
	filtered := make([]ChannelMessage, 0, len(messages))
	for _, message := range messages {
		if !textureAgentIDMatchesDoc(message.ToAgentID, channelID) {
			continue
		}
		switch runProfiles[strings.TrimSpace(message.FromRunID)] {
		case AgentProfileResearcher, AgentProfileSuper, AgentProfileCoSuper:
			filtered = append(filtered, message)
		}
	}
	if len(filtered) > limit {
		filtered = filtered[len(filtered)-limit:]
	}
	return filtered, nil
}

func (rt *Runtime) userRevisionDiffSummaries(ctx context.Context, ownerID, docID string, limit int) ([]string, error) {
	revs, err := rt.Store().ListRevisionsByDoc(ctx, docID, ownerID, limit)
	if err != nil {
		return nil, err
	}
	summaries := make([]string, 0, len(revs))
	for i := len(revs) - 1; i >= 0; i-- {
		rev := revs[i]
		if rev.AuthorKind != types.AuthorUser {
			continue
		}
		label := rev.CreatedAt.UTC().Format(time.RFC3339)
		if rev.ParentRevisionID == "" {
			summaries = append(summaries, fmt.Sprintf("%s %s: initial user-authored draft", rev.RevisionID, label))
			continue
		}
		diff, err := rt.Store().GetDiff(ctx, rev.ParentRevisionID, rev.RevisionID, ownerID)
		if err != nil {
			continue
		}
		summaries = append(summaries, fmt.Sprintf("%s %s: %s", rev.RevisionID, label, summarizeDiffResult(diff)))
	}
	return summaries, nil
}

func truncatePromptSnippet(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 || len(value) <= limit {
		return value
	}
	if limit <= 3 {
		return value[:limit]
	}
	return value[:limit-3] + "..."
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

func summarizeFocusedUserEditContext(current types.Revision, previous *types.Revision) string {
	currentLines := splitPromptLines(current.Content)
	if previous == nil {
		return truncatePromptSnippet(current.Content, 12000)
	}
	changed := changedToLineIndexes(previous.Content, current.Content)
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Document length: %d chars, %d lines. Full document omitted for ordinary long-document edit latency.\n", len(current.Content), len(currentLines)))
	if len(changed) == 0 {
		b.WriteString("No changed lines detected. First bounded current-head excerpt:\n")
		b.WriteString(truncatePromptSnippet(current.Content, 6000))
		return b.String()
	}
	ranges := focusedLineRanges(changed, len(currentLines), 4)
	const maxChars = 18000
	for i, r := range ranges {
		if b.Len() >= maxChars {
			b.WriteString("\nAdditional changed regions omitted for prompt size.")
			break
		}
		start, end := r[0], r[1]
		if start < 0 {
			start = 0
		}
		if end >= len(currentLines) {
			end = len(currentLines) - 1
		}
		if start > end || start >= len(currentLines) {
			continue
		}
		excerpt := strings.Join(currentLines[start:end+1], "")
		if len(excerpt) > 5000 {
			excerpt = truncatePromptSnippet(excerpt, 5000)
		}
		b.WriteString(fmt.Sprintf("\nChanged region %d, current lines %d-%d:\n", i+1, start+1, end+1))
		b.WriteString(excerpt)
		if !strings.HasSuffix(excerpt, "\n") {
			b.WriteString("\n")
		}
	}
	return strings.TrimSpace(b.String())
}

func splitPromptLines(content string) []string {
	if content == "" {
		return nil
	}
	lines := strings.SplitAfter(content, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

type promptLineMatch struct {
	from int
	to   int
}

func changedToLineIndexes(from, to string) []int {
	fromLines := splitPromptLines(from)
	toLines := splitPromptLines(to)
	matches := promptLineLCS(fromLines, toLines)
	changed := make(map[int]bool)
	fi, ti := 0, 0
	markToGap := func(start, end int) {
		for i := start; i < end; i++ {
			if i >= 0 && i < len(toLines) {
				changed[i] = true
			}
		}
	}
	for _, match := range matches {
		if ti < match.to {
			markToGap(ti, match.to)
		}
		if fi < match.from && ti == match.to {
			if match.to < len(toLines) {
				changed[match.to] = true
			} else if match.to > 0 {
				changed[match.to-1] = true
			}
		}
		fi = match.from + 1
		ti = match.to + 1
	}
	if ti < len(toLines) {
		markToGap(ti, len(toLines))
	}
	if fi < len(fromLines) && ti == len(toLines) && len(toLines) > 0 {
		changed[len(toLines)-1] = true
	}
	out := make([]int, 0, len(changed))
	for idx := range changed {
		out = append(out, idx)
	}
	sort.Ints(out)
	return out
}

func promptLineLCS(fromLines, toLines []string) []promptLineMatch {
	m, n := len(fromLines), len(toLines)
	if m == 0 || n == 0 {
		return nil
	}
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}
	for i := m - 1; i >= 0; i-- {
		for j := n - 1; j >= 0; j-- {
			if fromLines[i] == toLines[j] {
				dp[i][j] = dp[i+1][j+1] + 1
			} else if dp[i+1][j] >= dp[i][j+1] {
				dp[i][j] = dp[i+1][j]
			} else {
				dp[i][j] = dp[i][j+1]
			}
		}
	}
	var matches []promptLineMatch
	for i, j := 0, 0; i < m && j < n; {
		switch {
		case fromLines[i] == toLines[j]:
			matches = append(matches, promptLineMatch{from: i, to: j})
			i++
			j++
		case dp[i+1][j] >= dp[i][j+1]:
			i++
		default:
			j++
		}
	}
	return matches
}

func focusedLineRanges(changed []int, lineCount, radius int) [][2]int {
	if lineCount <= 0 {
		return nil
	}
	if radius < 0 {
		radius = 0
	}
	ranges := make([][2]int, 0, len(changed))
	for _, idx := range changed {
		start := idx - radius
		if start < 0 {
			start = 0
		}
		end := idx + radius
		if end >= lineCount {
			end = lineCount - 1
		}
		if len(ranges) == 0 || start > ranges[len(ranges)-1][1]+1 {
			ranges = append(ranges, [2]int{start, end})
			continue
		}
		if end > ranges[len(ranges)-1][1] {
			ranges[len(ranges)-1][1] = end
		}
	}
	return ranges
}

// emitTextureAgentEvent is a helper that emits an texture-specific agent revision
// event, carrying the doc_id in the payload so the frontend can correlate
// progress to the open document (VAL-ETEXT-004).
func (rt *Runtime) emitTextureAgentEvent(ctx context.Context, rec *types.RunRecord, kind types.EventKind, cause events.EventCause, payload json.RawMessage) {
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
	if err := rt.store.AppendEvent(ctx, evRec); err != nil {
		log.Printf("runtime: persist texture agent event: %v", err)
		return
	}
	rt.bus.Publish(events.RuntimeEvent{
		Record: *evRec,
		Actor:  events.ActorRuntime,
		Cause:  cause,
	})
}

func (rt *Runtime) emitTextureDocumentRevisionEvent(ctx context.Context, ownerID string, rev types.Revision) {
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
	if err := rt.store.AppendEvent(ctx, evRec); err != nil {
		log.Printf("runtime: persist texture document revision event: %v", err)
		return
	}
	rt.bus.Publish(events.RuntimeEvent{
		Record: *evRec,
		Actor:  events.ActorRuntime,
		Cause:  events.CauseTaskLifecycle,
	})
}

func (rt *Runtime) emitTextureDocumentRevisionEventForRun(ctx context.Context, rec *types.RunRecord, rev types.Revision) {
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
	if err := rt.store.AppendEvent(ctx, evRec); err != nil {
		log.Printf("runtime: persist texture document revision event: %v", err)
		return
	}
	rt.bus.Publish(events.RuntimeEvent{
		Record: *evRec,
		Actor:  events.ActorRuntime,
		Cause:  events.CauseTaskLifecycle,
	})
}

func (rt *Runtime) emitTextureDecisionRecordedEvent(ctx context.Context, rec *types.RunRecord, decision types.TextureDecisionRecord) {
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
	if err := rt.store.AppendEvent(ctx, evRec); err != nil {
		log.Printf("runtime: persist Texture decision event: %v", err)
		return
	}
	rt.bus.Publish(events.RuntimeEvent{
		Record: *evRec,
		Actor:  events.ActorRuntime,
		Cause:  events.CauseToolExecution,
	})
}
