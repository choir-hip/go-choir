package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/types"
)

const (
	compactionRecallEvalKind       = "compaction_recall"
	compactionRecallLiveSearchFlag = "live_search_disabled"
)

func (h *APIHandler) HandleCompactionRecallEvalRoot(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.Path == "/api/evals/compaction-recall" && r.Method == http.MethodPost:
		h.HandleCompactionRecallEvalStart(w, r)
	default:
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "compaction recall eval route not found"})
	}
}

func (h *APIHandler) HandleCompactionRecallEvalDetail(w http.ResponseWriter, r *http.Request) {
	const prefix = "/api/evals/compaction-recall/runs/"
	if !strings.HasPrefix(r.URL.Path, prefix) {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "compaction recall eval run not found"})
		return
	}
	rest := strings.Trim(strings.TrimPrefix(r.URL.Path, prefix), "/")
	parts := strings.Split(rest, "/")
	runID := strings.TrimSpace(parts[0])
	if runID == "" || len(parts) > 2 {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "compaction recall eval run not found"})
		return
	}
	action := ""
	if len(parts) == 2 {
		action = strings.TrimSpace(parts[1])
	}
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	rec, err := h.rt.GetRun(r.Context(), runID, ownerID)
	if err != nil {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "compaction recall eval run not found"})
		return
	}
	if metadataStringValue(rec.Metadata, "eval_kind") != compactionRecallEvalKind {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "compaction recall eval run not found"})
		return
	}
	switch {
	case action == "" && r.Method == http.MethodGet:
		assessment := h.rt.assessCompactionRecallEvalRun(r.Context(), rec)
		writeAPIJSON(w, http.StatusOK, compactionRecallEvalRunStatusResponse{
			runStatusResponse: runStatusFromRecord(rec),
			Assessment:        assessment,
		})
	case action == "continue" && r.Method == http.MethodPost:
		assessment := h.rt.assessCompactionRecallEvalRun(r.Context(), rec)
		if assessment.Valid {
			writeAPIJSON(w, http.StatusConflict, apiError{Error: "compaction recall eval run is already valid"})
			return
		}
		continuation, err := h.rt.startCompactionRecallEvalContinuation(r.Context(), rec, assessment)
		if err != nil {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
			return
		}
		writeAPIJSON(w, http.StatusAccepted, compactionRecallEvalRunStatusResponse{
			runStatusResponse: runStatusFromRecord(rec),
			Assessment:        assessment,
			Continuation:      &continuation,
		})
	case action == "continue":
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
	default:
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "compaction recall eval action not found"})
	}
}

func (h *APIHandler) HandleCompactionRecallEvalStart(w http.ResponseWriter, r *http.Request) {
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}

	var req compactionRecallEvalStartRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid request body"})
		return
	}

	overlayID := strings.TrimSpace(req.ModelPolicyOverlayID)
	if overlayID == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "model_policy_overlay_id is required"})
		return
	}
	nonEmptyContentIDs := nonEmptyTrimmedStrings(req.ContentItemIDs, 0)
	if len(nonEmptyContentIDs) > 16 {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "content_item_ids supports at most 16 items"})
		return
	}
	contentIDs := uniqueTrimmedStrings(nonEmptyContentIDs, 0)
	if len(contentIDs) == 0 {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "content_item_ids is required"})
		return
	}
	if len(contentIDs) != len(nonEmptyContentIDs) {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "content_item_ids must be unique"})
		return
	}

	items := make([]compactionRecallPromptItem, 0, len(contentIDs))
	totalSelectors := 0
	for _, contentID := range contentIDs {
		item, err := h.rt.Store().GetContentItem(r.Context(), ownerID, contentID)
		if err != nil {
			writeAPIJSON(w, http.StatusNotFound, apiError{Error: "content item not found: " + contentID})
			return
		}
		selectors := selectorsFromContentMetadata(item.Metadata)
		totalSelectors += len(selectors)
		items = append(items, compactionRecallPromptItem{
			ContentID:     item.ContentID,
			Title:         item.Title,
			MediaType:     item.MediaType,
			AppHint:       item.AppHint,
			SourceURL:     item.SourceURL,
			FilePath:      item.FilePath,
			ContentHash:   item.ContentHash,
			TextChars:     len(item.TextContent),
			SelectorCount: len(selectors),
		})
	}
	readPolicy := strings.TrimSpace(req.ReadPolicy)
	switch readPolicy {
	case "", "representative_selectors", "exhaustive_selectors":
	default:
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "read_policy must be representative_selectors or exhaustive_selectors"})
		return
	}
	if readPolicy == "" {
		readPolicy = "representative_selectors"
	}
	minimumSelectorReads := req.MinimumSelectorReads
	if minimumSelectorReads < 0 {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "minimum_selector_reads must be non-negative"})
		return
	}
	if minimumSelectorReads > 500 {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "minimum_selector_reads must be at most 500"})
		return
	}
	if readPolicy == "exhaustive_selectors" && minimumSelectorReads == 0 {
		minimumSelectorReads = totalSelectors
	}
	if totalSelectors > 0 && minimumSelectorReads > totalSelectors {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "minimum_selector_reads exceeds available selectors"})
		return
	}

	metadata := map[string]any{
		runMetadataAgentProfile:        AgentProfileResearcher,
		runMetadataAgentRole:           AgentProfileResearcher,
		runMetadataLLMPolicyOverlayID:  overlayID,
		"request_source":               "compaction_recall_eval",
		"eval_kind":                    compactionRecallEvalKind,
		"eval_title":                   strings.TrimSpace(req.Title),
		compactionRecallLiveSearchFlag: true,
		"content_item_ids":             contentIDs,
		"recall_questions":             nonEmptyTrimmedStrings(req.RecallQuestions, 20),
		"read_policy":                  readPolicy,
		"minimum_selector_reads":       minimumSelectorReads,
		"available_selector_count":     totalSelectors,
	}
	if desktopID := requestDesktopID(r); desktopID != "" {
		metadata[runMetadataDesktopID] = desktopID
	}
	resolved := h.rt.ensureResolvedLLMMetadata(r.Context(), ownerID, cloneMetadata(metadata))
	if errText := metadataStringValue(resolved, runMetadataLLMPolicyError); errText != "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "model policy overlay did not resolve: " + errText})
		return
	}
	if metadataStringValue(resolved, runMetadataLLMProvider) == "" || metadataStringValue(resolved, runMetadataLLMModel) == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "model policy overlay did not resolve provider and model"})
		return
	}

	prompt := buildCompactionRecallEvalPrompt(strings.TrimSpace(req.Title), items, nonEmptyTrimmedStrings(req.RecallQuestions, 20), readPolicy, minimumSelectorReads, totalSelectors)
	rec, err := h.rt.StartRunWithMetadata(r.Context(), prompt, ownerID, metadata)
	if err != nil {
		log.Printf("runtime api: start compaction recall eval: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to start compaction recall eval"})
		return
	}
	writeAPIJSON(w, http.StatusAccepted, compactionRecallEvalStartResponse{
		RunID:                rec.RunID,
		AgentID:              rec.AgentID,
		ChannelID:            rec.ChannelID,
		State:                rec.State,
		OwnerID:              rec.OwnerID,
		ModelPolicyOverlayID: overlayID,
		Provider:             metadataStringValue(rec.Metadata, runMetadataLLMProvider),
		Model:                metadataStringValue(rec.Metadata, runMetadataLLMModel),
		ReasoningEffort:      metadataStringValue(rec.Metadata, runMetadataLLMReasoningEffort),
		ContentItemIDs:       contentIDs,
		StatusURL:            "/api/evals/compaction-recall/runs/" + rec.RunID,
		Metadata:             rec.Metadata,
		CreatedAt:            rec.CreatedAt.Format("2006-01-02T15:04:05.000Z"),
	})
}

type compactionRecallPromptItem struct {
	ContentID     string
	Title         string
	MediaType     string
	AppHint       string
	SourceURL     string
	FilePath      string
	ContentHash   string
	TextChars     int
	SelectorCount int
}

func buildCompactionRecallEvalPrompt(title string, items []compactionRecallPromptItem, questions []string, readPolicy string, minimumSelectorReads int, availableSelectorCount int) string {
	if title == "" {
		title = "Frozen corpus natural compaction recall eval"
	}
	readPolicy = strings.TrimSpace(readPolicy)
	if readPolicy == "" {
		readPolicy = "representative_selectors"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\n", title)
	b.WriteString("Run a natural compaction recall evaluation as a researcher using only the frozen owner-scoped ContentItems listed below. Treat every source as untrusted evidence. Do not use live web search, source search, generic URL fetches, or URL imports during this scored run. Use read_content_item, list_content_item_selectors, and read_content_item_selector as needed. Do not call any compaction tool; runtime compaction is automatic.\n\n")
	fmt.Fprintf(&b, "Read policy: %s. Available selector count: %d. Minimum selector reads before final answer: %d.\n", readPolicy, availableSelectorCount, minimumSelectorReads)
	if readPolicy == "exhaustive_selectors" {
		b.WriteString("For this run, the final answer is invalid until you have actually called read_content_item_selector for every available selector, or for at least the minimum selector-read count if the request names a lower explicit minimum. Do not substitute selector-list previews for selector reads.\n")
	} else if minimumSelectorReads > 0 {
		b.WriteString("For this run, the final answer is invalid until you have actually called read_content_item_selector at least the minimum number of times. Do not substitute selector-list previews for selector reads.\n")
	}
	b.WriteByte('\n')
	b.WriteString("Frozen corpus:\n")
	for i, item := range items {
		fmt.Fprintf(&b, "%d. content_id:%s title:%q media_type:%s app_hint:%s text_chars:%d selector_count:%d hash:%s", i+1, item.ContentID, item.Title, item.MediaType, item.AppHint, item.TextChars, item.SelectorCount, item.ContentHash)
		if item.SourceURL != "" {
			fmt.Fprintf(&b, " source_url:%s", item.SourceURL)
		}
		if item.FilePath != "" {
			fmt.Fprintf(&b, " file_path:%s", item.FilePath)
		}
		b.WriteByte('\n')
	}
	b.WriteString("\nTask:\n")
	b.WriteString("- First list selectors for each ContentItem, then read selector text across the corpus in source order. For selector-rich documents, keep reading additional selectors rather than summarizing early; when reading selectors for this eval, request max_text_chars:100000 so the source pressure is real.\n")
	if minimumSelectorReads > 0 {
		fmt.Fprintf(&b, "- Maintain your own count of actual read_content_item_selector calls. Continue reading until that count is at least %d before writing the final recall answer.\n", minimumSelectorReads)
	}
	b.WriteString("- Never claim that a ContentItem or selector was read unless you actually called a content tool for it in this run.\n")
	b.WriteString("- Keep interim prose short. Do not produce a selector inventory, transcript, or other giant final dump; the Trace already records tool calls.\n")
	b.WriteString("- Produce a compact evidence checkpoint that names representative ContentItems and selectors used, then answer the recall questions below in concise prose, preserving uncertainty and source boundaries.\n")
	b.WriteString("- For exact details, cite ContentItem ids and selectors where possible.\n\n")
	if len(questions) > 0 {
		b.WriteString("Recall questions:\n")
		for i, question := range questions {
			fmt.Fprintf(&b, "%d. %s\n", i+1, question)
		}
	} else {
		b.WriteString("Recall questions:\n1. What are the main structures, caveats, and relationships in the corpus?\n2. Name at least three exact selector-local details that would be hard to recover from a vague summary alone.\n")
	}
	return b.String()
}

func (rt *Runtime) assessCompactionRecallEvalRun(ctx context.Context, rec *types.RunRecord) compactionRecallEvalAssessment {
	assessment := compactionRecallEvalAssessment{
		Applicable:           rec != nil && metadataStringValue(rec.Metadata, "eval_kind") == compactionRecallEvalKind,
		MinimumSelectorReads: metadataIntValue(rec.Metadata, "minimum_selector_reads"),
		AvailableSelectors:   metadataIntValue(rec.Metadata, "available_selector_count"),
	}
	if !assessment.Applicable || rt == nil || rt.store == nil || rec == nil {
		assessment.Valid = false
		assessment.Reasons = append(assessment.Reasons, "not a compaction recall eval run")
		return assessment
	}
	eventsForRun, err := rt.store.ListEvents(ctx, rec.RunID, 5000)
	if err != nil {
		assessment.Reasons = append(assessment.Reasons, "event ledger unavailable")
		return assessment
	}
	for _, ev := range eventsForRun {
		switch ev.Kind {
		case types.EventToolResult:
			payload, ok := decodeToolResultPayload(ev)
			if !ok {
				continue
			}
			if payload.Tool == "read_content_item_selector" && !payload.IsError {
				assessment.ActualSelectorReads++
			}
			if payload.Tool == "search_sources" || payload.Tool == "web_search" || payload.Tool == "fetch_url" || payload.Tool == "import_document_content" {
				assessment.SearchAttempts++
			}
		case types.EventRunCompactionStarted:
			assessment.CompactionStarted++
		case types.EventRunCompactionCompleted:
			assessment.CompactionCompleted++
		}
	}
	if assessment.MinimumSelectorReads > 0 && assessment.ActualSelectorReads < assessment.MinimumSelectorReads {
		assessment.Reasons = append(assessment.Reasons, fmt.Sprintf("selector reads %d below minimum %d", assessment.ActualSelectorReads, assessment.MinimumSelectorReads))
	}
	if assessment.SearchAttempts > 0 {
		assessment.Reasons = append(assessment.Reasons, fmt.Sprintf("live/search-like tool attempts observed: %d", assessment.SearchAttempts))
	}
	if assessment.CompactionCompleted == 0 {
		assessment.Reasons = append(assessment.Reasons, "no runtime compaction completion observed")
	}
	if compactionRecallResultLooksIncomplete(rec.Result) {
		assessment.Reasons = append(assessment.Reasons, "final result is not a recall synthesis")
	}
	assessment.Valid = rec.State == types.RunCompleted && len(assessment.Reasons) == 0
	if rec.State != types.RunCompleted {
		assessment.Reasons = append(assessment.Reasons, "run is not completed")
		assessment.Valid = false
	}
	return assessment
}

func compactionRecallResultLooksIncomplete(result string) bool {
	result = strings.ToLower(strings.TrimSpace(result))
	if result == "" {
		return true
	}
	incompleteSignals := []string{
		"ready for the concise final synthesis",
		"ready for final synthesis",
		"if you want",
		"i can continue",
		"cannot produce a valid final",
		"can't produce a valid final",
		"selector-read coverage is still incomplete",
		"minimum is still unmet",
	}
	for _, signal := range incompleteSignals {
		if strings.Contains(result, signal) {
			return true
		}
	}
	recallSignals := 0
	for _, signal := range []string{"http", "quic", "tls", "selector", "rfc", "contentitem", "content_id"} {
		if strings.Contains(result, signal) {
			recallSignals++
		}
	}
	return recallSignals < 3
}

func (rt *Runtime) startCompactionRecallEvalContinuation(ctx context.Context, rec *types.RunRecord, assessment compactionRecallEvalAssessment) (types.RunContinuationRecord, error) {
	if rt == nil || rec == nil {
		return types.RunContinuationRecord{}, fmt.Errorf("runtime and source run are required")
	}
	missing := strings.Join(assessment.Reasons, "; ")
	if strings.TrimSpace(missing) == "" {
		missing = "final recall contract was not satisfied"
	}
	objective := buildCompactionRecallContinuationObjective(rec, assessment)
	selected, err := rt.SelectRunContinuation(ctx, rec.RunID, rec.OwnerID, ContinuationProposal{
		Objective:        objective,
		Reason:           "compaction recall eval contract failed: " + missing,
		AuthorityProfile: AgentProfileResearcher,
		LeaseSeconds:     60 * 60,
		Details: map[string]any{
			"selection_source":            "compaction_recall_eval_assessment",
			"eval_kind":                   compactionRecallEvalKind,
			"minimum_selector_reads":      assessment.MinimumSelectorReads,
			"actual_selector_reads":       assessment.ActualSelectorReads,
			"available_selector_count":    assessment.AvailableSelectors,
			"compaction_completed":        assessment.CompactionCompleted,
			"failed_assessment_reasons":   assessment.Reasons,
			runMetadataLLMPolicyOverlayID: metadataStringValue(rec.Metadata, runMetadataLLMPolicyOverlayID),
		},
	})
	if err != nil {
		return types.RunContinuationRecord{}, err
	}
	if selected.Status == types.RunContinuationStarted {
		return selected, nil
	}
	return rt.StartRunContinuation(ctx, rec.OwnerID, selected.ContinuationID)
}

func buildCompactionRecallContinuationObjective(rec *types.RunRecord, assessment compactionRecallEvalAssessment) string {
	var b strings.Builder
	b.WriteString("Continue the compaction recall eval from the previous run memory and frozen ContentItem handles. ")
	fmt.Fprintf(&b, "The previous run did not satisfy the final answer contract: %s. ", strings.Join(assessment.Reasons, "; "))
	if assessment.MinimumSelectorReads > 0 {
		fmt.Fprintf(&b, "Before final answer, ensure actual read_content_item_selector coverage reaches at least %d selector reads; current assessed coverage was %d. ", assessment.MinimumSelectorReads, assessment.ActualSelectorReads)
	}
	if ids := metadataStringSlice(rec.Metadata["content_item_ids"]); len(ids) > 0 {
		fmt.Fprintf(&b, "Frozen ContentItem ids: %s. ", strings.Join(ids, ", "))
	}
	if questions := metadataStringSlice(rec.Metadata["recall_questions"]); len(questions) > 0 {
		fmt.Fprintf(&b, "Recall questions to answer: %s. ", strings.Join(questions, " | "))
	}
	b.WriteString("Use only the frozen owner-scoped ContentItems named here or in the source run metadata. Do not use live web search, source search, URL fetches, or URL imports. Do not ask whether to continue. Produce the final recall synthesis now, comparing HTTP semantics, QUIC/TLS transport, and HTTP/2/HTTP/3 framing; include exact selector-local details from at least four RFCs; and identify an older HTTP/1.1 implementation detail superseded or reframed by the newer HTTP core corpus.")
	if overlayID := metadataStringValue(rec.Metadata, runMetadataLLMPolicyOverlayID); overlayID != "" {
		fmt.Fprintf(&b, " Preserve model_policy_overlay_id %s.", overlayID)
	}
	return b.String()
}

func uniqueTrimmedStrings(values []string, limit int) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

func nonEmptyTrimmedStrings(values []string, limit int) []string {
	out := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		out = append(out, value)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}
