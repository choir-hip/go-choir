package textureowner

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/yusefmosiah/go-choir/internal/agentcore"
	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	contentowner "github.com/yusefmosiah/go-choir/internal/content"
	"github.com/yusefmosiah/go-choir/internal/provider"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

type promptBarSubmitRequest struct {
	Text      string `json:"text"`
	CommandID string `json:"command_id,omitempty"`
}

type promptBarSubmitResponse struct {
	Schema             string         `json:"schema"`
	SubmissionID       string         `json:"submission_id"`
	State              types.RunState `json:"state"`
	CreatedAt          string         `json:"created_at"`
	StatusURL          string         `json:"status_url"`
	CommandID          string         `json:"command_id,omitempty"`
	StartRequestDigest string         `json:"start_request_digest,omitempty"`
	TrajectoryID       string         `json:"trajectory_id,omitempty"`
	DocID              string         `json:"doc_id,omitempty"`
	RevisionID         string         `json:"revision_id,omitempty"`
	SubjectID          string         `json:"subject_id,omitempty"`
	ObligationIDs      []string       `json:"obligation_ids,omitempty"`
	ReducerSeq         int64          `json:"reducer_seq,omitempty"`
	SnapshotCursor     int64          `json:"snapshot_cursor,omitempty"`
}

type promptBarSubmissionStatusResponse struct {
	SubmissionID string             `json:"submission_id"`
	State        types.RunState     `json:"state"`
	CreatedAt    string             `json:"created_at"`
	UpdatedAt    string             `json:"updated_at"`
	FinishedAt   *string            `json:"finished_at,omitempty"`
	Decision     *ConductorDecision `json:"decision,omitempty"`
	Error        string             `json:"error,omitempty"`
}

// HandlePromptBar is the browser-public product entrypoint. Agentcore records
// the conductor decision; the selected app owner materializes product state.
func (h *Handler) HandlePromptBar(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	var req promptBarSubmitRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid prompt-bar request"})
		return
	}
	text := strings.TrimSpace(req.Text)
	if text == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "text is required"})
		return
	}
	commandID := strings.TrimSpace(req.CommandID)
	if commandID == "" {
		commandID = uuid.NewString()
	}

	requestedApp := agentprofile.Texture
	var sourceURL, mediaType, appHint string
	if classifiedApp, classifiedURL, classifiedMediaType, ok := contentowner.ClassifyPromptBarContentIntent(text); ok {
		requestedApp, appHint = classifiedApp, classifiedApp
		sourceURL, mediaType = classifiedURL, classifiedMediaType
	}
	title := provider.InitialTextureTitle(text, "")
	metadata := map[string]any{
		"agent_profile":          agentprofile.Conductor,
		"agent_role":             agentprofile.Conductor,
		"input_source":           "prompt_bar",
		"requested_app":          requestedApp,
		"seed_prompt":            text,
		"initial_document_title": title,
		"submission_surface":     "prompt_bar",
		"lifecycle_command_id":   commandID,
	}
	if ownerEmail := authenticatedUserEmail(r); ownerEmail != "" {
		metadata["owner_email"] = ownerEmail
	}
	if sourceURL != "" {
		metadata["content_source_url"] = sourceURL
		metadata["content_media_type"] = mediaType
		metadata["content_app_hint"] = appHint
	}
	var handoff HandoffDecision
	rec, err := h.Core.CompletePromptBarDecision(r.Context(), text, ownerID, metadata, agentcore.PromptBarDecisionSpec{
		Action: "open_app", App: requestedApp, Title: title,
		SourceURL: sourceURL, MediaType: mediaType, AppHint: appHint,
	})
	if err == nil && requestedApp == agentprofile.Texture {
		handoff, err = h.EnsureTextureHandoff(r.Context(), rec, HandoffRequest{
			Kind: HandoffKindUserPrompt, CallerProfile: agentprofile.Conductor,
			Objective: text, Title: title,
		})
	}
	if err != nil {
		if errors.Is(err, agentcore.ErrPromptCommandConflict) || errors.Is(err, store.ErrLifecycleCommandConflict) || errors.Is(err, store.ErrConcurrentStateChange) {
			writeAPIJSON(w, http.StatusConflict, apiError{Error: "command identity conflicts with the stored request"})
			return
		}
		log.Printf("texture prompt bar: submit: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to submit prompt"})
		return
	}
	writeAPIJSON(w, http.StatusAccepted, promptBarSubmitResponse{
		Schema:       types.DurableWorkSchemaV1,
		SubmissionID: rec.RunID,
		State:        rec.State,
		CreatedAt:    rec.CreatedAt.Format("2006-01-02T15:04:05.000Z"),
		StatusURL:    "/api/prompt-bar/submissions/" + rec.RunID,
		CommandID:    handoff.Conductor.CommandID, TrajectoryID: handoff.Conductor.TrajectoryID,
		StartRequestDigest: handoff.Conductor.StartRequestDigest,
		DocID:              handoff.Conductor.DocID, RevisionID: handoff.Conductor.UserRevisionID,
		SubjectID: handoff.Conductor.SubjectID, ObligationIDs: handoff.Conductor.ObligationIDs,
		ReducerSeq: handoff.Conductor.ReducerSeq, SnapshotCursor: handoff.Conductor.SnapshotCursor,
	})
}

// HandlePromptBarSubmission returns product-level state for one owner-scoped submission.
func (h *Handler) HandlePromptBarSubmission(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	const prefix = "/api/prompt-bar/submissions/"
	submissionID := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, prefix))
	if !strings.HasPrefix(r.URL.Path, prefix) || submissionID == "" || strings.Contains(submissionID, "/") {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "submission not found"})
		return
	}
	rec, err := h.Core.GetRun(r.Context(), submissionID, ownerID)
	if err != nil || agentProfileForRun(rec) != agentprofile.Conductor || metadataStringValue(rec.Metadata, "input_source") != "prompt_bar" {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "submission not found"})
		return
	}
	var finishedAt *string
	if rec.FinishedAt != nil {
		value := rec.FinishedAt.Format("2006-01-02T15:04:05.000Z")
		finishedAt = &value
	}
	response := promptBarSubmissionStatusResponse{
		SubmissionID: rec.RunID, State: rec.State,
		CreatedAt:  rec.CreatedAt.Format("2006-01-02T15:04:05.000Z"),
		UpdatedAt:  rec.UpdatedAt.Format("2006-01-02T15:04:05.000Z"),
		FinishedAt: finishedAt, Error: rec.Error,
	}
	if raw := strings.TrimSpace(rec.Result); raw != "" {
		var decision ConductorDecision
		if json.Unmarshal([]byte(raw), &decision) == nil && strings.TrimSpace(decision.Action) != "" {
			response.Decision = &decision
		}
	}
	writeAPIJSON(w, http.StatusOK, response)
}
