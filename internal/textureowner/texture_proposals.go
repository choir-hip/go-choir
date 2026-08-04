package textureowner

import (
	"encoding/json"
	"fmt"
	"github.com/yusefmosiah/go-choir/internal/types"
	"log"
	"net/http"
	"strings"
)

type internalTextureProposalDeliveryRequest struct {
	OwnerID              string `json:"owner_id"`
	ProposalID           string `json:"proposal_id"`
	PublicationID        string `json:"publication_id"`
	PublicationVersionID string `json:"publication_version_id,omitempty"`
	SubmitterID          string `json:"submitter_id,omitempty"`
	DeliveryID           string `json:"delivery_id,omitempty"`
	State                string `json:"state,omitempty"`
}

type internalTextureProposalDeliveryResponse struct {
	DeliveryID    string `json:"delivery_id"`
	OwnerID       string `json:"owner_id"`
	TargetAgentID string `json:"target_agent_id"`
	ChannelID     string `json:"channel_id"`
	State         string `json:"state"`
	RunID         string `json:"loop_id,omitempty"`
}

func (h *Handler) HandleInternalTextureProposalDelivery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if err := requireInternalRuntimeCaller(r); err != nil {
		writeAPIJSON(w, http.StatusForbidden, apiError{Error: "internal caller required"})
		return
	}
	var req internalTextureProposalDeliveryRequest
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
	if err := h.refuseLegacyTextureWriter("deliver publication proposal to Texture"); err != nil {
		log.Printf("texture proposal delivery: %v", err)
		writeAPIJSON(w, http.StatusConflict, apiError{Error: err.Error()})
		return
	}
}

func buildTextureLifecycleUpdateMessage(update types.CoagentSourcePacket) string {
	packet := update.Packet
	var b strings.Builder
	b.WriteString("Coagent source packet ready.")
	if role := strings.TrimSpace(update.Role); role != "" {
		b.WriteString("\nRole: ")
		b.WriteString(role)
		b.WriteString(".")
	}
	b.WriteString("\nSchema: ")
	b.WriteString(packet.SchemaVersion)
	b.WriteString("\nKind: ")
	b.WriteString(packet.Kind)
	if summary := strings.TrimSpace(packet.Summary); summary != "" {
		b.WriteString("\nSummary: ")
		b.WriteString(summary)
	}
	if len(packet.Actions) > 0 {
		b.WriteString("\n\nActions:\n")
		for _, action := range packet.Actions {
			actionType := strings.TrimSpace(action.Type)
			objective := strings.TrimSpace(action.Objective)
			if actionType == "" && objective == "" {
				continue
			}
			b.WriteString("- ")
			if action.ActionID != "" {
				b.WriteString(action.ActionID)
				b.WriteString(": ")
			}
			b.WriteString(actionType)
			if objective != "" {
				b.WriteString(" - ")
				b.WriteString(objective)
			}
			b.WriteString("\n")
		}
	}
	if len(packet.Notes) > 0 {
		b.WriteString("\n\nNotes:\n")
		for _, note := range packet.Notes {
			note = strings.TrimSpace(note)
			if note == "" {
				continue
			}
			b.WriteString("- ")
			b.WriteString(note)
			b.WriteString("\n")
		}
	}
	return b.String()
}

func requireInternalRuntimeCaller(r *http.Request) error {
	if r.Header.Get("X-Internal-Caller") != "true" {
		return fmt.Errorf("missing internal caller marker")
	}
	return nil
}
