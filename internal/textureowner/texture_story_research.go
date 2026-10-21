package textureowner

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/store"
)

// A reader's request for more research on a published story goes to the
// story's research desk, not to Texture
// (docs/design/research-more-goes-to-research-2026-10-21.md). The route
// takes only the request id and the head the reader saw: the paper writes
// research's packet from that head itself.

const storyResearchSchema = "choir.texture.story_research_request.v1"

type storyResearchHTTPRequest struct {
	ClientRequestID        string `json:"client_request_id"`
	ExpectedHeadRevisionID string `json:"expected_head_revision_id"`
}

type storyResearchHTTPReceipt struct {
	Schema    string `json:"schema"`
	DocID     string `json:"doc_id"`
	RequestID string `json:"request_id"`
	store.StoryResearchReceipt
}

type storyResearchRefusal struct {
	Error  string `json:"error"`
	Reason string `json:"reason"`
}

// HandleTextureStoryResearch serves POST .../research and
// GET .../research/{client_request_id}.
func (h *Handler) HandleTextureStoryResearch(w http.ResponseWriter, r *http.Request) {
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
	doc, err := h.getTextureDocument(r.Context(), ownerID, docID)
	if err != nil {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "document not found"})
		return
	}
	_, requestID, hasRequestID := strings.Cut(strings.TrimPrefix(r.URL.Path, textureDocumentsPathPrefix+docID+"/research"), "/")
	switch {
	case r.Method == http.MethodGet && hasRequestID && requestID != "":
		state, err := h.Store.StoryResearchRequestState(r.Context(), ownerID, doc.ComputerID, docID, requestID)
		if errors.Is(err, store.ErrNotFound) {
			writeAPIJSON(w, http.StatusNotFound, apiError{Error: "research request not found"})
			return
		}
		if err != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "research request state unavailable"})
			return
		}
		writeAPIJSON(w, http.StatusOK, state)
	case r.Method == http.MethodPost && !hasRequestID:
		h.postStoryResearch(w, r, ownerID, doc.ComputerID, docID)
	default:
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
	}
}

func (h *Handler) postStoryResearch(w http.ResponseWriter, r *http.Request, ownerID, computerID, docID string) {
	var body storyResearchHTTPRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.ClientRequestID) == "" || strings.TrimSpace(body.ExpectedHeadRevisionID) == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "client_request_id and expected_head_revision_id are required"})
		return
	}
	ctx := r.Context()
	// A committed request replays before any current-state refusal; only a
	// new one is refused while research is still running.
	if _, err := h.Store.StoryResearchRequestState(ctx, ownerID, computerID, docID, body.ClientRequestID); errors.Is(err, store.ErrNotFound) {
		if _, active, err := h.Core.TextureActiveRunByAgent(ctx, ownerID, computerID, agentprofile.Research+":"+docID); err != nil || active {
			writeAPIJSON(w, http.StatusConflict, storyResearchRefusal{Error: "the story's research desk is busy", Reason: "busy"})
			return
		}
	}
	receipt, err := h.Store.CommitStoryResearchRequest(ctx, store.StoryResearchRequest{
		OwnerID: ownerID, ComputerID: computerID, DocumentID: docID,
		ClientRequestID: body.ClientRequestID, ExpectedHeadRevisionID: body.ExpectedHeadRevisionID,
	})
	switch {
	case errors.Is(err, store.ErrStoryResearchBusy):
		writeAPIJSON(w, http.StatusConflict, storyResearchRefusal{Error: "the story's research desk is busy", Reason: "busy"})
	case errors.Is(err, store.ErrStoryResearchChanged):
		writeAPIJSON(w, http.StatusConflict, storyResearchRefusal{Error: "the story changed", Reason: "changed"})
	case errors.Is(err, store.ErrNotFound):
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "story not found"})
	case err != nil:
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "research request failed"})
	default:
		writeAPIJSON(w, http.StatusAccepted, storyResearchHTTPReceipt{Schema: storyResearchSchema, DocID: docID, RequestID: body.ClientRequestID, StoryResearchReceipt: receipt})
	}
}
