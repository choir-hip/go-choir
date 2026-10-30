package textureowner

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/yusefmosiah/go-choir/internal/agentcore"
	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	contentowner "github.com/yusefmosiah/go-choir/internal/content"
	"github.com/yusefmosiah/go-choir/internal/provider"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// promptBarCommitTimeout bounds the durable conductor-decision commit. It is
// deliberately decoupled from the client request context: a proxy/upstream
// disconnect must not abort an in-flight Dolt projection batch mid-commit
// (receipt: docs/archive/problems/s0m-prompt-bar-submit-context-canceled-2026-10-02.md).
const promptBarCommitTimeout = 120 * time.Second

type promptBarSubmitRequest struct {
	Text      string `json:"text"`
	CommandID string `json:"command_id,omitempty"`
	// ContinueDocID names the owner's document the prompt builds on: the
	// submit is that document's owner revise instead of a new document
	// (FE-02 step 1, docs/problems/prompt-bar-always-creates-a-new-document-2026-10-26.md).
	ContinueDocID string `json:"continue_doc_id,omitempty"`
}

const promptBarContinueSchemaV1 = "choir.prompt_bar_continue.v1"

// promptBarContinueResponse is the 202 of a continue: the owner revision is
// the receipt and there is no conductor submission to poll. TrajectoryID is
// the trajectory the revision committed on, a new one when a settled
// document continued (F1), so the window follows it.
type promptBarContinueResponse struct {
	Schema       string `json:"schema"`
	Continued    bool   `json:"continued"`
	CommandID    string `json:"command_id"`
	DocID        string `json:"doc_id"`
	TrajectoryID string `json:"trajectory_id"`
	RevisionID   string `json:"revision_id"`
	RequestID    string `json:"request_id"`
	Replay       bool   `json:"replay"`
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

	commitCtx, commitCancel := context.WithTimeout(context.WithoutCancel(r.Context()), promptBarCommitTimeout)
	defer commitCancel()
	if docID := strings.TrimSpace(req.ContinueDocID); docID != "" {
		h.continuePromptBarDocument(commitCtx, w, r, ownerID, docID, text, commandID)
		return
	}
	commitCtx = ownerTextContext(commitCtx, r)
	rec, handoff, err := h.submitPromptBar(commitCtx, ownerID, authenticatedUserEmail(r), text, commandID, true)
	if err != nil {
		status, message := promptBarSubmitFailure(err)
		if status != http.StatusConflict && status != http.StatusServiceUnavailable {
			log.Printf("texture prompt bar: submit: %v", err)
		}
		writeAPIJSON(w, status, apiError{Error: message})
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

// continuePromptBarDocument answers a prompt-bar submit that builds on
// docID: the owner revise of that document, with the same authority, head
// and idempotency rules as /revise (the command id is its client request
// id) and the owner's text as typed. A desk's in-process call is refused:
// product_api refuses a desk any Texture write, and an owner-authored
// revision through the prompt bar must not open that door.
func (h *Handler) continuePromptBarDocument(ctx context.Context, w http.ResponseWriter, r *http.Request, ownerID, docID, text, commandID string) {
	if strings.TrimSpace(r.RemoteAddr) == "" {
		writeAPIJSON(w, http.StatusForbidden, apiError{Error: "continue_doc_id is an owner act; only the Texture desk writes a Texture, so address the Texture desk"})
		return
	}
	receipt, err := h.reviseOwnerDocument(ctx, ownerID, docID, textureAgentRevisionRequest{Prompt: text, ClientRequestID: commandID})
	if err != nil {
		writeOwnerRevisionRefusal(w, err)
		return
	}
	writeAPIJSON(w, http.StatusAccepted, promptBarContinueResponse{
		Schema: promptBarContinueSchemaV1, Continued: true, CommandID: commandID,
		DocID: receipt.DocID, TrajectoryID: receipt.TrajectoryID, RevisionID: receipt.RevisionID,
		RequestID: receipt.RequestID, Replay: receipt.Replay,
	})
}

// ownerTextContext decides which rule the text of a new Texture is prepared
// under. The owner's literal input (a request the proxy carried from the
// owner's browser) is kept as typed. A caller with an empty RemoteAddr is
// in-process: server.GuestHostCaller documents that marker and
// choir.ProductAPI, the management desk's route to /api/prompt-bar, is its
// only producer. A desk composes text from captures and Stories, so its text
// keeps the Texture authoring rules and a refusal is a 400 it can correct
// (round 2 review F1, docs/problems/prompt-bar-refuses-bracketed-index-as-server-error-2026-10-22.md).
func ownerTextContext(ctx context.Context, r *http.Request) context.Context {
	if strings.TrimSpace(r.RemoteAddr) == "" {
		return store.WithStrictInitialText(ctx)
	}
	return ctx
}

// promptBarSubmitFailure answers a failed prompt-bar submit with the status
// and message the owner's client sees. A command conflict is a 409 and a
// computer without genesis is a 503. A validation refusal of the owner's
// input (store.ErrInvalidTextureRevision) is the owner's to correct, so it is
// a 400 that carries the validator's reason, not a server error (receipt:
// docs/problems/prompt-bar-refuses-bracketed-index-as-server-error-2026-10-22.md,
// HTTP 500 "failed to submit prompt" with the reason only in the guest log).
// Everything else is a 500 that does not describe the internals.
func promptBarSubmitFailure(err error) (int, string) {
	switch {
	case errors.Is(err, agentcore.ErrPromptCommandConflict), errors.Is(err, store.ErrLifecycleCommandConflict), errors.Is(err, store.ErrConcurrentStateChange):
		return http.StatusConflict, "command identity conflicts with the stored request"
	case errors.Is(err, agentcore.ErrPreGenesis):
		return http.StatusServiceUnavailable, "computer initializing"
	case errors.Is(err, store.ErrInvalidTextureRevision):
		reason := err.Error()
		if marker := store.ErrInvalidTextureRevision.Error() + ": "; strings.Contains(reason, marker) {
			reason = reason[strings.Index(reason, marker)+len(marker):]
		}
		return http.StatusBadRequest, "prompt refused: " + reason
	default:
		return http.StatusInternalServerError, "failed to submit prompt"
	}
}

// submitPromptBar records the conductor decision and, for Texture, the
// handoff: the prompt bar's submit path. classify routes content links to
// their apps; an owner request built by the computer itself (a release
// adoption, P8) is always Texture's.
func (h *Handler) submitPromptBar(ctx context.Context, ownerID, ownerEmail, text, commandID string, classify bool) (*types.RunRecord, HandoffDecision, error) {
	requestedApp := agentprofile.Texture
	var sourceURL, mediaType, appHint string
	if classify {
		if classifiedApp, classifiedURL, classifiedMediaType, ok := contentowner.ClassifyPromptBarContentIntent(text); ok {
			requestedApp, appHint = classifiedApp, classifiedApp
			sourceURL, mediaType = classifiedURL, classifiedMediaType
		}
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
	if ownerEmail != "" {
		metadata["owner_email"] = ownerEmail
	}
	if sourceURL != "" {
		metadata["content_source_url"] = sourceURL
		metadata["content_media_type"] = mediaType
		metadata["content_app_hint"] = appHint
	}
	var handoff HandoffDecision
	rec, err := h.Core.CompletePromptBarDecision(ctx, text, ownerID, metadata, agentcore.PromptBarDecisionSpec{
		Action: "open_app", App: requestedApp, Title: title,
		SourceURL: sourceURL, MediaType: mediaType, AppHint: appHint,
	})
	if err == nil && requestedApp == agentprofile.Texture {
		handoff, err = h.EnsureTextureHandoff(ctx, rec, HandoffRequest{
			Kind: HandoffKindUserPrompt, CallerProfile: agentprofile.Conductor,
			Objective: text, Title: title,
		})
	}
	return rec, handoff, err
}

// SubmitOwnerPrompt submits a request the computer makes on its owner's
// behalf, after the owner's act, through the prompt bar's own path (a
// release adoption, P8: docs/archive/reports/p6-p8-release-adopt-2026-10-15.md).
// The commit outlives the caller's context like the prompt bar's does.
func (h *Handler) SubmitOwnerPrompt(ctx context.Context, ownerID, text, commandID string) (agentcore.PromptSubmission, error) {
	commitCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), promptBarCommitTimeout)
	defer cancel()
	rec, handoff, err := h.submitPromptBar(commitCtx, ownerID, "", strings.TrimSpace(text), strings.TrimSpace(commandID), false)
	if err != nil {
		return agentcore.PromptSubmission{}, err
	}
	if strings.TrimSpace(handoff.Conductor.TrajectoryID) == "" {
		return agentcore.PromptSubmission{}, errors.New("prompt submit returned no trajectory")
	}
	return agentcore.PromptSubmission{SubmissionID: rec.RunID, TrajectoryID: handoff.Conductor.TrajectoryID, DocID: handoff.Conductor.DocID}, nil
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
