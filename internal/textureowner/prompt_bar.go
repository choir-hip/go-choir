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
	contentowner "github.com/yusefmosiah/go-choir/internal/content"
	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/promptroute"
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
	// Focus is the focused window's app agent and object (plan MA4.8,
	// "Focus-aware request"; it replaced continue_doc_id). With a focus the
	// text goes to that app agent, which decides what it is for; without one
	// the conductor routes it. The focus never reaches the conductor.
	Focus *promptBarFocus `json:"focus,omitempty"`
}

// promptBarFocus is a focused submit's focus: a desk id of the registry, the
// typed ref of the focused object and an optional bounded selection.
type promptBarFocus struct {
	AppAgentID string `json:"app_agent_id"`
	ObjectRef  string `json:"object_ref"`
	Selection  string `json:"selection,omitempty"`
}

// promptBarSelectionBytes bounds a focus selection.
const promptBarSelectionBytes = 16 << 10

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
	r = asOwnerRoute(r)
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
	if req.Focus != nil {
		h.submitFocusedPrompt(commitCtx, w, r, ownerID, *req.Focus, text, commandID)
		return
	}
	commitCtx = ownerTextContext(commitCtx, r)
	// The owner's own submit is routed; an in-process desk caller (management
	// creating a Texture through choir.ProductAPI) asked for a Texture.
	rec, handoff, err := h.submitPromptBar(commitCtx, ownerID, authenticatedUserEmail(r), text, commandID, true, strings.TrimSpace(r.RemoteAddr) != "")
	if err != nil {
		status, message := promptBarSubmitFailure(err)
		if status != http.StatusConflict && status != http.StatusServiceUnavailable {
			log.Printf("texture prompt bar: submit: %v", err)
		}
		writeAPIJSON(w, status, apiError{Error: message})
		return
	}
	// A routed submit to a package desk has no Texture hand-off; its command
	// id is still the submit's (review r1 item 10).
	if handoff.Conductor.CommandID == "" {
		handoff.Conductor.CommandID = commandID
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

// submitFocusedPrompt sends a focused submit to the focused app agent (plan
// MA4.8): the default desk's focus on one of the owner's documents is that
// document's owner revise; a package desk's focus is an owner-origin request
// record to that desk, which decides what the text is for, and the decision
// opens its app. The conductor is not asked and sees nothing of the focus. A
// focus naming any other desk (a system desk, a role, a candidate, a package
// the owner has not installed) is refused.
func (h *Handler) submitFocusedPrompt(ctx context.Context, w http.ResponseWriter, r *http.Request, ownerID string, focus promptBarFocus, text, commandID string) {
	defaultDesk := deskmanifest.DocumentCustodianName()
	app := strings.TrimSpace(focus.AppAgentID)
	ref := strings.TrimSpace(focus.ObjectRef)
	// An in-process caller is refused before any registry answer, so the
	// refusal says nothing about which desks exist (review r1 item 6).
	if strings.TrimSpace(r.RemoteAddr) == "" {
		writeAPIJSON(w, http.StatusForbidden, apiError{Error: "a focused submit is an owner act; address the app agent's desk"})
		return
	}
	if len(focus.Selection) > promptBarSelectionBytes {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "focus selection is too long"})
		return
	}
	if len(ref) > promptroute.MaxObjectRefBytes {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "focus object_ref is too long"})
		return
	}
	if !promptroute.Addressable(defaultDesk, app) {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "focus names an app agent the prompt bar cannot address"})
		return
	}
	if app == defaultDesk {
		if ref == "" {
			writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "focus object_ref is required"})
			return
		}
		h.continuePromptBarDocument(ctx, w, r, ownerID, ref, text, commandID)
		return
	}
	// A package desk's focus names the owner's object of a kind that desk is
	// the custodian of; the request carries that kind's owner_private label.
	kind, err := promptroute.FocusKind(app, ownerID, ref)
	if err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
		return
	}
	runID := h.Core.PromptBarRunID(ownerID, commandID)
	if _, err := h.Core.DeliverOwnerRequest(ctx, ownerID, runID, app, text, &agentcore.OwnerFocus{AppAgentID: app, ObjectRef: ref, Kind: kind, Selection: focus.Selection}, ""); err != nil {
		log.Printf("texture prompt bar: focused request to %s: %v", app, err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to submit prompt"})
		return
	}
	metadata := promptBarMetadata(text, commandID, authenticatedUserEmail(r), promptroute.AppOf(app))
	metadata["focus_app_agent_id"] = app
	rec, err := h.Core.CompletePromptBarDecision(ctx, text, ownerID, metadata, agentcore.PromptBarDecisionSpec{
		Action: "open_app", App: promptroute.AppOf(app), Title: provider.InitialTextureTitle(text, ""),
	})
	if err != nil {
		status, message := promptBarSubmitFailure(err)
		writeAPIJSON(w, status, apiError{Error: message})
		return
	}
	writeAPIJSON(w, http.StatusAccepted, promptBarSubmitResponse{
		Schema: types.DurableWorkSchemaV1, SubmissionID: rec.RunID, State: rec.State,
		CreatedAt: rec.CreatedAt.Format("2006-01-02T15:04:05.000Z"),
		StatusURL: "/api/prompt-bar/submissions/" + rec.RunID, CommandID: commandID,
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
		writeAPIJSON(w, http.StatusForbidden, apiError{Error: "a focused submit is an owner act; only the Texture desk writes a Texture, so address the Texture desk"})
		return
	}
	receipt, err := h.reviseDocument(ctx, ReviseRequest{OwnerID: ownerID, DocID: docID, Text: text, ClientRequestID: commandID})
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

// submitPromptBar records the conductor decision and hands off: the prompt
// bar's submit path. classify routes content links to their apps by the URL
// rule; route asks the prompt router (MA4.8) for everything else, and only the
// owner's own submit is routed. An owner request built by the computer itself
// (a release adoption, P8) is always Texture's. Every route is a route record,
// and the default route (Texture) is the fallback of every router failure.
func (h *Handler) submitPromptBar(ctx context.Context, ownerID, ownerEmail, text, commandID string, classify, route bool) (*types.RunRecord, HandoffDecision, error) {
	defaultDesk := deskmanifest.DocumentCustodianName()
	requestedApp := defaultDesk
	var sourceURL, mediaType, appHint string
	var routed *agentcore.PromptRoute
	runID := h.Core.PromptBarRunID(ownerID, commandID)
	if classify {
		if classifiedApp, classifiedURL, classifiedMediaType, ok := contentowner.ClassifyPromptBarContentIntent(text); ok {
			requestedApp, appHint = classifiedApp, classifiedApp
			sourceURL, mediaType = classifiedURL, classifiedMediaType
			if route {
				r := h.Core.RecordRuleRoute(ctx, ownerID, runID, classifiedApp, defaultDesk)
				routed = &r
			}
		} else if route {
			r := h.Core.RoutePrompt(ctx, ownerID, runID, text, defaultDesk)
			routed = &r
		}
	}
	// A route to a package desk hands off by an owner-origin request record
	// to that desk and opens its app. A hand-off that cannot be written takes
	// the default route.
	if routed != nil && routed.Rule != promptroute.RuleContentURL && routed.Destination != defaultDesk {
		if _, err := h.Core.DeliverOwnerRequest(ctx, ownerID, runID, routed.Destination, text, nil, routed.RecordID); err != nil {
			log.Printf("texture prompt bar: hand-off to %s: %v; taking the default route", routed.Destination, err)
			// The route record stays as frozen (the router's prediction); the
			// dispatch record and the run say it was not delivered.
			if rerr := h.Core.RecordRouteDispatchFailure(ctx, ownerID, runID, *routed, err); rerr != nil {
				log.Printf("texture prompt bar: record the failed hand-off of %s: %v", runID, rerr)
			}
			routed.Destination, routed.App = defaultDesk, defaultDesk
			routed.Reason, routed.Dispatch = "the hand-off could not be written", agentcore.RouteDispatchFailed
		} else {
			requestedApp = routed.App
			routed.Dispatch = agentcore.RouteDispatchDelivered
		}
	}
	title := provider.InitialTextureTitle(text, "")
	metadata := promptBarMetadata(text, commandID, ownerEmail, requestedApp)
	if sourceURL != "" {
		metadata["content_source_url"] = sourceURL
		metadata["content_media_type"] = mediaType
		metadata["content_app_hint"] = appHint
	}
	var handoff HandoffDecision
	rec, err := h.Core.CompletePromptBarDecision(ctx, text, ownerID, metadata, agentcore.PromptBarDecisionSpec{
		Action: "open_app", App: requestedApp, Title: title,
		SourceURL: sourceURL, MediaType: mediaType, AppHint: appHint, Route: routed,
	})
	if err == nil && requestedApp == defaultDesk {
		handoff, err = h.EnsureTextureHandoff(ctx, rec, HandoffRequest{
			Kind: HandoffKindUserPrompt, CallerProfile: deskmanifest.FrontDoorName(),
			Objective: text, Title: title,
		})
	}
	return rec, handoff, err
}

// promptBarMetadata is a prompt-bar decision run's metadata.
func promptBarMetadata(text, commandID, ownerEmail, requestedApp string) map[string]any {
	metadata := map[string]any{
		"agent_profile":          deskmanifest.FrontDoorName(),
		"agent_role":             deskmanifest.FrontDoorName(),
		"input_source":           "prompt_bar",
		"requested_app":          requestedApp,
		"seed_prompt":            text,
		"initial_document_title": provider.InitialTextureTitle(text, ""),
		"submission_surface":     "prompt_bar",
		"lifecycle_command_id":   commandID,
	}
	if ownerEmail != "" {
		metadata["owner_email"] = ownerEmail
	}
	return metadata
}

// HandlePromptBarSubmission returns product-level state for one owner-scoped submission.
func (h *Handler) HandlePromptBarSubmission(w http.ResponseWriter, r *http.Request) {
	r = asOwnerRoute(r)
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
	if err != nil || agentProfileForRun(rec) != deskmanifest.FrontDoorName() || metadataStringValue(rec.Metadata, "input_source") != "prompt_bar" {
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
