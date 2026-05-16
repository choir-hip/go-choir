package runtime

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/yusefmosiah/go-choir/internal/types"
)

type internalVTextProposalDeliveryRequest struct {
	OwnerID              string `json:"owner_id"`
	ProposalID           string `json:"proposal_id"`
	PublicationID        string `json:"publication_id"`
	PublicationVersionID string `json:"publication_version_id,omitempty"`
	SubmitterID          string `json:"submitter_id,omitempty"`
	DeliveryID           string `json:"delivery_id,omitempty"`
	State                string `json:"state,omitempty"`
}

type internalVTextProposalDeliveryResponse struct {
	DeliveryID    string `json:"delivery_id"`
	OwnerID       string `json:"owner_id"`
	TargetAgentID string `json:"target_agent_id"`
	ChannelID     string `json:"channel_id"`
	State         string `json:"state"`
	RunID         string `json:"loop_id,omitempty"`
}

func (h *APIHandler) HandleInternalVTextProposalDelivery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if err := requireInternalRuntimeCaller(r); err != nil {
		writeAPIJSON(w, http.StatusForbidden, apiError{Error: "internal caller required"})
		return
	}
	var req internalVTextProposalDeliveryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "invalid request body"})
		return
	}
	req.OwnerID = strings.TrimSpace(req.OwnerID)
	req.ProposalID = strings.TrimSpace(req.ProposalID)
	req.PublicationID = strings.TrimSpace(req.PublicationID)
	req.PublicationVersionID = strings.TrimSpace(req.PublicationVersionID)
	req.SubmitterID = strings.TrimSpace(req.SubmitterID)
	req.DeliveryID = strings.TrimSpace(req.DeliveryID)
	if req.OwnerID == "" || req.ProposalID == "" || req.PublicationID == "" {
		writeAPIJSON(w, http.StatusBadRequest, apiError{Error: "owner_id, proposal_id, and publication_id are required"})
		return
	}
	if req.DeliveryID == "" {
		req.DeliveryID = uuid.NewString()
	}

	superAgent, err := h.rt.EnsurePersistentSuperAgent(r.Context(), req.OwnerID)
	if err != nil {
		log.Printf("vtext proposal delivery: ensure super: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to prepare author inbox"})
		return
	}
	now := time.Now().UTC()
	content := fmt.Sprintf(
		"Publication proposal received.\nproposal_id=%s\npublication_id=%s\npublication_version_id=%s\nsubmitter_id=%s\ndelivery_id=%s\n\nReview this proposal as an author-side event. Do not mutate canonical publication content without owner acceptance.",
		req.ProposalID,
		req.PublicationID,
		req.PublicationVersionID,
		req.SubmitterID,
		req.DeliveryID,
	)
	message := types.ChannelMessage{
		ChannelID:   superAgent.ChannelID,
		From:        "platform",
		FromAgentID: "platform:publication-proposals",
		ToAgentID:   superAgent.AgentID,
		Role:        "publication_proposal",
		Content:     content,
		Timestamp:   now,
	}
	if err := h.rt.store.AppendChannelMessage(r.Context(), &message, req.OwnerID); err != nil {
		log.Printf("vtext proposal delivery: append channel message: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to record author inbox message"})
		return
	}
	if err := h.rt.store.EnqueueInboxDelivery(r.Context(), types.InboxDelivery{
		DeliveryID:  req.DeliveryID,
		OwnerID:     req.OwnerID,
		ToAgentID:   superAgent.AgentID,
		FromAgentID: message.FromAgentID,
		ChannelID:   superAgent.ChannelID,
		Role:        message.Role,
		Content:     message.Content,
		CreatedAt:   now,
	}); err != nil {
		log.Printf("vtext proposal delivery: enqueue inbox: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "failed to enqueue author inbox delivery"})
		return
	}

	run, err := h.rt.reconcilePersistentSuperActor(r.Context(), req.OwnerID, superAgent.AgentID)
	if err != nil {
		log.Printf("vtext proposal delivery: reconcile super: %v", err)
		writeAPIJSON(w, http.StatusAccepted, internalVTextProposalDeliveryResponse{
			DeliveryID:    req.DeliveryID,
			OwnerID:       req.OwnerID,
			TargetAgentID: superAgent.AgentID,
			ChannelID:     superAgent.ChannelID,
			State:         "queued",
		})
		return
	}
	resp := internalVTextProposalDeliveryResponse{
		DeliveryID:    req.DeliveryID,
		OwnerID:       req.OwnerID,
		TargetAgentID: superAgent.AgentID,
		ChannelID:     superAgent.ChannelID,
		State:         "delivered",
	}
	if run != nil {
		resp.RunID = run.RunID
	}
	writeAPIJSON(w, http.StatusCreated, resp)
}
