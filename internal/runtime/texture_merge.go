package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/agentprofile"
)

var textureMergePreviewCommentRE = regexp.MustCompile(`(?is)\n*\s*<!--\s*Texture merge preview provenance\b.*?-->\s*`)

func selectMergeSuggestions(suggestions []textureMergeSuggestion, ids []string) []textureMergeSuggestion {
	wanted := map[string]bool{}
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id != "" {
			wanted[id] = true
		}
	}
	if len(wanted) == 0 {
		if len(suggestions) <= 3 {
			return suggestions
		}
		return append([]textureMergeSuggestion(nil), suggestions[:3]...)
	}
	var selected []textureMergeSuggestion
	for _, suggestion := range suggestions {
		if wanted[suggestion.ID] {
			selected = append(selected, suggestion)
		}
	}
	return selected
}

func suggestionIDs(suggestions []textureMergeSuggestion) []string {
	ids := make([]string, 0, len(suggestions))
	for _, suggestion := range suggestions {
		ids = append(ids, suggestion.ID)
	}
	return ids
}

func sanitizeTextureMergeContent(content string) string {
	cleaned := textureMergePreviewCommentRE.ReplaceAllString(content, "\n\n")
	return strings.TrimSpace(cleaned) + "\n"
}

func applyTextureModelMergeEdits(targetContent string, edits []textureModelMergeEdit) (string, []map[string]any, error) {
	content := sanitizeTextureMergeContent(targetContent)
	applied := make([]map[string]any, 0, len(edits))
	for i, edit := range edits {
		operation := strings.TrimSpace(strings.ToLower(edit.Operation))
		switch operation {
		case "replace_exact":
			oldText := edit.OldText
			newText := edit.NewText
			if strings.TrimSpace(oldText) == "" {
				return "", applied, fmt.Errorf("merge edit %d replace_exact missing old_text", i)
			}
			if !strings.Contains(content, oldText) {
				return "", applied, fmt.Errorf("merge edit %d old_text not found in target", i)
			}
			content = strings.Replace(content, oldText, newText, 1)
		case "append":
			newText := strings.TrimSpace(edit.NewText)
			if newText == "" {
				return "", applied, fmt.Errorf("merge edit %d append missing new_text", i)
			}
			content = strings.TrimSpace(content) + "\n\n" + newText + "\n"
		case "noop", "no_op":
			// Keep explicit no-op edits as provenance without changing content.
		default:
			return "", applied, fmt.Errorf("merge edit %d has unsupported operation %q", i, edit.Operation)
		}
		applied = append(applied, map[string]any{
			"suggestion_id": edit.SuggestionID,
			"operation":     operation,
			"rationale":     edit.Rationale,
		})
	}
	return sanitizeTextureMergeContent(content), applied, nil
}

func normalizeModelSemanticMergeResult(result textureModelSemanticMergeResult, sourceRev, targetRev types.Revision, requireEdits bool) (textureModelSemanticMergeResult, error) {
	if len(result.Summary) == 0 {
		return result, fmt.Errorf("model response missing summary")
	}
	if len(result.Suggestions) == 0 {
		for i, finding := range result.Summary {
			finding = strings.TrimSpace(finding)
			if finding == "" {
				continue
			}
			result.Suggestions = append(result.Suggestions, textureMergeSuggestion{
				ID:          "model_finding_" + strconv.Itoa(i+1),
				Label:       snippet(finding, 72),
				Description: finding,
				Status:      "Needs review",
				Source:      sourceRev.RevisionID,
				Preview:     finding,
			})
		}
		if len(result.Suggestions) == 0 {
			return result, fmt.Errorf("model response missing suggestions")
		}
	}
	for i := range result.Suggestions {
		result.Suggestions[i].ID = strings.TrimSpace(result.Suggestions[i].ID)
		result.Suggestions[i].Label = strings.TrimSpace(result.Suggestions[i].Label)
		result.Suggestions[i].Description = strings.TrimSpace(result.Suggestions[i].Description)
		result.Suggestions[i].Status = strings.TrimSpace(result.Suggestions[i].Status)
		result.Suggestions[i].Source = strings.TrimSpace(result.Suggestions[i].Source)
		result.Suggestions[i].Preview = strings.TrimSpace(result.Suggestions[i].Preview)
		if result.Suggestions[i].ID == "" {
			result.Suggestions[i].ID = "merge_suggestion_" + strconv.Itoa(i+1)
		}
		if result.Suggestions[i].Label == "" {
			return result, fmt.Errorf("model suggestion %d missing label", i)
		}
		if result.Suggestions[i].Description == "" {
			return result, fmt.Errorf("model suggestion %d missing description", i)
		}
		if result.Suggestions[i].Status == "" {
			result.Suggestions[i].Status = "Needs review"
		}
		if result.Suggestions[i].Source == "" {
			result.Suggestions[i].Source = sourceRev.RevisionID
		}
		if result.Suggestions[i].Source != sourceRev.RevisionID && result.Suggestions[i].Source != targetRev.RevisionID {
			result.Suggestions[i].Source = sourceRev.RevisionID
		}
	}
	if requireEdits && len(result.Edits) == 0 {
		return result, fmt.Errorf("model response missing merge edits")
	}
	return result, nil
}

func (rt *Runtime) callTextureSemanticMergeModel(ctx context.Context, ownerID string, sourceRev, targetRev types.Revision, diff types.DiffResult, mode string, suggestionIDs []string, sourceLabel, targetLabel string) (textureModelSemanticMergeResult, map[string]any, error) {
	if rt == nil || rt.provider == nil {
		return textureModelSemanticMergeResult{}, nil, fmt.Errorf("runtime provider unavailable")
	}
	policy, err := rt.loadModelPolicy(ctx, ownerID)
	policySource := "policy"
	if err != nil {
		policySource = "policy_error:" + err.Error()
	}
	selection := policy.Resolve(agentprofile.Texture)
	if strings.TrimSpace(selection.Provider) == "" || strings.TrimSpace(selection.Model) == "" {
		return textureModelSemanticMergeResult{}, nil, fmt.Errorf("texture model policy did not resolve provider/model")
	}
	maxTokens := selection.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 4096
	}
	prompt := buildTextureSemanticMergePrompt(sourceRev, targetRev, diff, mode, suggestionIDs, sourceLabel, targetLabel)
	message, _ := json.Marshal(map[string]any{
		"role":    "user",
		"content": []map[string]string{{"type": "text", "text": prompt}},
	})
	start := time.Now()
	resp, err := toolregistry.CallToolLoopProviderWithRetries(ctx, toolregistry.AsToolLoopProvider(rt.provider), provideriface.ToolLoopRequest{
		Provider:        selection.Provider,
		Model:           selection.Model,
		ReasoningEffort: selection.ReasoningEffort,
		System:          "You are Choir's Texture semantic merge engine. Return only valid JSON matching the requested schema. Do not write markdown prose.",
		Messages:        []json.RawMessage{message},
		ToolChoice:      "none",
		MaxTokens:       maxTokens,
	}, nil)
	latency := time.Since(start)
	evidence := map[string]any{
		"provider":                  selection.Provider,
		"model":                     selection.Model,
		"reasoning_effort":          selection.ReasoningEffort,
		"policy_source":             firstNonEmpty(selection.Source, policySource),
		"mode":                      mode,
		"prompt_chars":              len(prompt),
		"source_revision_id":        sourceRev.RevisionID,
		"target_revision_id":        targetRev.RevisionID,
		"source_chars":              len(sourceRev.Content),
		"target_chars":              len(targetRev.Content),
		"latency_ms":                latency.Milliseconds(),
		"max_tokens":                maxTokens,
		"selected_suggestion_ids":   suggestionIDs,
		"model_response_id":         "",
		"model_stop_reason":         "",
		"model_input_tokens":        0,
		"model_output_tokens":       0,
		"reasoning_content_present": false,
	}
	if err != nil {
		evidence["error"] = err.Error()
		return textureModelSemanticMergeResult{}, evidence, err
	}
	evidence["model_response_id"] = resp.ID
	evidence["model_stop_reason"] = resp.StopReason
	evidence["model_input_tokens"] = resp.Usage.InputTokens
	evidence["model_output_tokens"] = resp.Usage.OutputTokens
	evidence["response_model"] = resp.Model
	evidence["reasoning_content_present"] = strings.TrimSpace(resp.ReasoningContent) != ""

	jsonText, err := extractJSONObject(resp.Text)
	if err != nil {
		evidence["response_excerpt"] = snippet(resp.Text, 1000)
		return textureModelSemanticMergeResult{}, evidence, err
	}
	var result textureModelSemanticMergeResult
	if err := json.Unmarshal([]byte(jsonText), &result); err != nil {
		evidence["response_excerpt"] = snippet(resp.Text, 1000)
		return textureModelSemanticMergeResult{}, evidence, fmt.Errorf("decode model semantic merge JSON: %w", err)
	}
	result, err = normalizeModelSemanticMergeResult(result, sourceRev, targetRev, mode == "preview")
	if err != nil {
		evidence["response_excerpt"] = snippet(resp.Text, 1000)
		return textureModelSemanticMergeResult{}, evidence, err
	}
	return result, evidence, nil
}

func buildTextureSemanticMergePrompt(sourceRev, targetRev types.Revision, diff types.DiffResult, mode string, suggestionIDs []string, sourceLabel, targetLabel string) string {
	schema := `{
  "summary": ["short semantic finding"],
  "suggestions": [{
    "id": "stable_snake_case_id",
    "label": "short user-facing merge action",
    "description": "what concept should move or be preserved",
    "status": "Clean merge | Needs review | Conflicts with latest",
    "source": "source or target revision id",
    "preview": "brief evidence excerpt"
  }],
  "edits": [{
    "suggestion_id": "matching suggestion id",
    "operation": "replace_exact | append | noop",
    "old_text": "exact substring from target for replace_exact",
    "new_text": "replacement or appended text",
    "rationale": "why this edit is semantically correct"
  }]
}`
	var b strings.Builder
	b.WriteString("Compare two versions of one Texture document and")
	if mode == "preview" {
		b.WriteString(" produce a minimal structured edit preview that applies selected concepts into the target Primary draft.")
	} else {
		b.WriteString(" produce semantic findings and merge suggestions.")
	}
	b.WriteString("\n\nRules:\n")
	b.WriteString("- Return only JSON with this schema:\n")
	b.WriteString(schema)
	b.WriteString("\n- Do not include markdown fences, hidden comments, HTML comments, or visible provenance text.\n")
	b.WriteString("- Suggestions must be content-specific, not template/stub labels.\n")
	b.WriteString("- For preview mode, edits must be minimal. Prefer replace_exact over whole-document rewrite. old_text must be an exact substring of the target content.\n")
	b.WriteString("- Preserve target content unless a selected source concept clearly improves it.\n")
	b.WriteString("- Keep citations, source markers, and metadata references from the target unless the selected source concept requires an exact replacement.\n\n")
	b.WriteString("Mode: ")
	b.WriteString(mode)
	b.WriteString("\nSource label: ")
	b.WriteString(firstNonEmpty(sourceLabel, "source"))
	b.WriteString("\nSource revision id: ")
	b.WriteString(sourceRev.RevisionID)
	b.WriteString("\nTarget label: ")
	b.WriteString(firstNonEmpty(targetLabel, "target"))
	b.WriteString("\nTarget revision id: ")
	b.WriteString(targetRev.RevisionID)
	b.WriteString("\nLine diff: +")
	b.WriteString(strconv.Itoa(diff.AddedLines))
	b.WriteString(" / -")
	b.WriteString(strconv.Itoa(diff.RemovedLines))
	if len(suggestionIDs) > 0 {
		b.WriteString("\nSelected suggestion ids for preview: ")
		b.WriteString(strings.Join(suggestionIDs, ", "))
	}
	b.WriteString("\n\nSOURCE CONTENT:\n")
	b.WriteString(sourceRev.Content)
	b.WriteString("\n\nTARGET CONTENT:\n")
	b.WriteString(targetRev.Content)
	return b.String()
}

func (h *APIHandler) HandleTextureSemanticCompare(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	docID := extractDocID(r.URL.Path)
	if docID == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "document id is required"})
		return
	}
	doc, err := h.rt.Store().GetDocument(r.Context(), docID, ownerID)
	if err != nil {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "document not found"})
		return
	}
	sourceID := strings.TrimSpace(r.URL.Query().Get("source"))
	targetID := strings.TrimSpace(r.URL.Query().Get("target"))
	if targetID == "" {
		targetID = doc.CurrentRevisionID
	}
	if sourceID == "" || targetID == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "source and target revisions are required"})
		return
	}
	sourceRev, err := h.rt.Store().GetRevision(r.Context(), sourceID, ownerID)
	if err != nil || sourceRev.DocID != docID {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "source revision not found"})
		return
	}
	targetRev, err := h.rt.Store().GetRevision(r.Context(), targetID, ownerID)
	if err != nil || targetRev.DocID != docID {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "target revision not found"})
		return
	}
	diff, err := h.rt.Store().GetDiff(r.Context(), sourceID, targetID, ownerID)
	if err != nil {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: fmt.Sprintf("failed to compute diff: %v", err)})
		return
	}
	modelResult, modelEvidence, err := h.rt.callTextureSemanticMergeModel(r.Context(), ownerID, sourceRev, targetRev, diff, "compare", nil, r.URL.Query().Get("source_label"), r.URL.Query().Get("target_label"))
	if err != nil {
		log.Printf("texture api: model semantic compare: %v", err)
		writeAPIJSON(w, http.StatusBadGateway, apiError{Error: "model-backed semantic compare failed"})
		return
	}
	resp := textureSemanticCompareResponse{
		CompareID:        uuid.NewString(),
		SourceRevisionID: sourceRev.RevisionID,
		TargetRevisionID: targetRev.RevisionID,
		DraftLine:        defaultDraftLine(),
		Summary:          modelResult.Summary,
		Suggestions:      modelResult.Suggestions,
		Diff:             diff,
		ModelEvidence:    modelEvidence,
	}
	evidenceID := uuid.New().String()
	if evidenceErr := h.rt.Store().CreateEvidence(r.Context(), types.EvidenceRecord{
		EvidenceID: evidenceID,
		OwnerID:    ownerID,
		AgentID:    "texture:compare",
		Kind:       "texture.semantic_compare",
		SourceURI:  "texture://" + docID,
		Title:      "Semantic compare " + shortHash(sourceID) + " -> " + shortHash(targetID),
		Content:    mustMarshalString(resp),
		Metadata:   json.RawMessage(fmt.Sprintf(`{"doc_id":%q,"source_revision_id":%q,"target_revision_id":%q}`, docID, sourceID, targetID)),
		CreatedAt:  time.Now().UTC(),
	}); evidenceErr != nil {
		log.Printf("texture api: persist compare evidence: %v", evidenceErr)
	} else {
		resp.EvidenceID = evidenceID
	}
	writeAPIJSON(w, http.StatusOK, resp)
}

func (h *APIHandler) HandleTextureMergePreview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	docID := extractDocID(r.URL.Path)
	if docID == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "document id is required"})
		return
	}
	var req textureMergePreviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid request body"})
		return
	}
	sourceRev, err := h.rt.Store().GetRevision(r.Context(), strings.TrimSpace(req.SourceRevisionID), ownerID)
	if err != nil || sourceRev.DocID != docID {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "source revision not found"})
		return
	}
	targetRev, err := h.rt.Store().GetRevision(r.Context(), strings.TrimSpace(req.TargetRevisionID), ownerID)
	if err != nil || targetRev.DocID != docID {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "target revision not found"})
		return
	}
	diff, _ := h.rt.Store().GetDiff(r.Context(), sourceRev.RevisionID, targetRev.RevisionID, ownerID)
	modelResult, modelEvidence, err := h.rt.callTextureSemanticMergeModel(r.Context(), ownerID, sourceRev, targetRev, diff, "preview", req.SuggestionIDs, req.SourceVersionLabel, req.TargetVersionLabel)
	if err != nil {
		log.Printf("texture api: model merge preview: %v", err)
		writeAPIJSON(w, http.StatusBadGateway, apiError{Error: "model-backed merge preview failed"})
		return
	}
	selected := selectMergeSuggestions(modelResult.Suggestions, req.SuggestionIDs)
	if len(selected) == 0 {
		selected = modelResult.Suggestions
	}
	content, appliedEdits, err := applyTextureModelMergeEdits(targetRev.Content, modelResult.Edits)
	if err != nil {
		log.Printf("texture api: apply model merge edits: %v", err)
		writeAPIJSON(w, http.StatusBadGateway, apiError{Error: "model-backed merge preview returned edits that could not be applied"})
		return
	}
	modelEvidence["applied_edits"] = appliedEdits
	modelEvidence["applied_edit_count"] = len(appliedEdits)
	previewID := uuid.New().String()
	resp := textureMergePreviewResponse{
		PreviewID:        previewID,
		DocID:            docID,
		SourceRevisionID: sourceRev.RevisionID,
		TargetRevisionID: targetRev.RevisionID,
		DraftLine:        defaultDraftLine(),
		Content:          content,
		Suggestions:      selected,
		ModelEvidence:    modelEvidence,
		Provenance: map[string]any{
			"kind":               "texture_concept_merge_preview",
			"preview_id":         previewID,
			"source_revision_id": sourceRev.RevisionID,
			"target_revision_id": targetRev.RevisionID,
			"source_label":       strings.TrimSpace(req.SourceVersionLabel),
			"target_label":       strings.TrimSpace(req.TargetVersionLabel),
			"suggestion_ids":     suggestionIDs(selected),
			"draft_line":         defaultDraftLine(),
		},
	}
	evidenceID := uuid.New().String()
	if evidenceErr := h.rt.Store().CreateEvidence(r.Context(), types.EvidenceRecord{
		EvidenceID: evidenceID,
		OwnerID:    ownerID,
		AgentID:    "texture:merge",
		Kind:       "texture.merge_preview",
		SourceURI:  "texture://" + docID,
		Title:      "Merge preview " + shortHash(previewID),
		Content:    mustMarshalString(resp),
		Metadata:   json.RawMessage(fmt.Sprintf(`{"doc_id":%q,"preview_id":%q,"source_revision_id":%q,"target_revision_id":%q}`, docID, previewID, sourceRev.RevisionID, targetRev.RevisionID)),
		CreatedAt:  time.Now().UTC(),
	}); evidenceErr != nil {
		log.Printf("texture api: persist merge preview evidence: %v", evidenceErr)
	} else {
		resp.EvidenceID = evidenceID
		resp.Provenance["evidence_id"] = evidenceID
	}
	writeAPIJSON(w, http.StatusOK, resp)
}

func (h *APIHandler) HandleTextureAcceptMerge(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	docID := extractDocID(r.URL.Path)
	if docID == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "document id is required"})
		return
	}
	var req textureAcceptMergeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid request body"})
		return
	}
	if strings.TrimSpace(req.Content) == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "merge content is required"})
		return
	}
	targetRev, err := h.rt.Store().GetRevision(r.Context(), strings.TrimSpace(req.TargetRevisionID), ownerID)
	if err != nil || targetRev.DocID != docID {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "target revision not found"})
		return
	}
	doc, err := h.rt.Store().GetDocument(r.Context(), docID, ownerID)
	if err != nil {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "document not found"})
		return
	}
	if err := h.canonicalizeAliasedTextureDocumentTitle(r.Context(), ownerID, &doc, time.Now().UTC()); err != nil {
		log.Printf("texture api: canonicalize merge document title: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to canonicalize document title"})
		return
	}
	metadata := map[string]any{}
	for k, v := range req.Metadata {
		metadata[k] = v
	}
	metadata["source"] = "texture_concept_merge"
	metadata["merge_preview_id"] = strings.TrimSpace(req.PreviewID)
	metadata["merge_source_revision_id"] = strings.TrimSpace(req.SourceRevisionID)
	metadata["merge_target_revision_id"] = targetRev.RevisionID
	metadata["merge_suggestion_ids"] = req.SuggestionIDs
	metadata["draft_line"] = defaultDraftLine()
	encoded, _ := json.Marshal(metadata)
	rev := types.Revision{
		RevisionID:       uuid.New().String(),
		DocID:            docID,
		OwnerID:          ownerID,
		AuthorKind:       types.AuthorUser,
		AuthorLabel:      ownerID,
		Content:          sanitizeTextureMergeContent(req.Content),
		Citations:        targetRev.Citations,
		Metadata:         encoded,
		ParentRevisionID: targetRev.RevisionID,
		CreatedAt:        time.Now().UTC(),
	}
	if err := h.rt.Store().CreateRevision(r.Context(), rev); err != nil {
		log.Printf("texture api: accept merge revision: %v", err)
		writeAPIJSON(w, http.StatusConflict, apiError{Error: "failed to accept merge; document head may have changed"})
		return
	}
	storedRev, err := h.rt.Store().GetRevision(r.Context(), rev.RevisionID, ownerID)
	if err != nil {
		log.Printf("texture api: load accepted merge revision: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to load accepted merge revision"})
		return
	}
	h.rt.emitTextureDocumentRevisionEvent(r.Context(), ownerID, storedRev)
	writeAPIJSON(w, http.StatusCreated, h.revisionResponseFromRecord(r.Context(), storedRev))
}
