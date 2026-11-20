package platform

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"unicode/utf8"
)

// The two routes a computer's host writes search leads through
// (search_leads.go). Both take a host-internal caller or the guest vmctl binds
// to exactly the owner and computer the body names, as every guest write
// route does. Neither returns a lead.

func (h *ObjectGraphHandler) searchLeadsGate(w http.ResponseWriter, r *http.Request, owner, computer string) bool {
	if strings.TrimSpace(owner) == "" || strings.TrimSpace(computer) == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "owner_id and computer_id are required"})
		return false
	}
	if err := h.requireInternalOrBoundGuest(r, owner, computer, false); err != nil {
		writeJSON(w, http.StatusForbidden, apiError{Error: err.Error()})
		return false
	}
	if h.captureIndex == nil {
		writeJSON(w, http.StatusServiceUnavailable, apiError{Error: "search leads unavailable"})
		return false
	}
	return true
}

func searchLeadsFailure(w http.ResponseWriter, op string, err error) {
	if errors.Is(err, errSearchLeadsNotReady) {
		writeJSON(w, http.StatusServiceUnavailable, apiError{Error: err.Error()})
		return
	}
	log.Printf("corpusd search_leads op=%s err=%v", op, err)
	writeJSON(w, http.StatusInternalServerError, apiError{Error: "search leads failed"})
}

// HandleSearchLeads handles POST /internal/platform/search-leads: one arrival's
// results, upserted whole.
func (h *ObjectGraphHandler) HandleSearchLeads(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	var req SearchLeadsWrite
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<20)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "invalid request body"})
		return
	}
	if len(req.Results) > searchLeadMaxResults {
		writeJSON(w, http.StatusBadRequest, apiError{Error: fmt.Sprintf("too many results: at most %d per write", searchLeadMaxResults)})
		return
	}
	if !h.searchLeadsGate(w, r, req.OwnerID, req.ComputerID) {
		return
	}
	rows, err := h.captureIndex.WriteSearchLeads(r.Context(), req)
	if err != nil {
		searchLeadsFailure(w, "lead_write", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"rows": rows})
}

// HandleSearchLeadsTried handles POST /internal/platform/search-leads/tried: a
// run read these URLs.
func (h *ObjectGraphHandler) HandleSearchLeadsTried(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	var req struct {
		OwnerID    string   `json:"owner_id"`
		ComputerID string   `json:"computer_id"`
		URLs       []string `json:"urls"`
		CaptureID  string   `json:"capture_id,omitempty"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "invalid request body"})
		return
	}
	if len(req.URLs) > searchLeadMaxURLs {
		writeJSON(w, http.StatusBadRequest, apiError{Error: fmt.Sprintf("too many urls: at most %d per call", searchLeadMaxURLs)})
		return
	}
	if utf8.RuneCountInString(req.CaptureID) > searchLeadCaptureIDRunes {
		writeJSON(w, http.StatusBadRequest, apiError{Error: fmt.Sprintf("capture_id too long: at most %d characters", searchLeadCaptureIDRunes)})
		return
	}
	if !h.searchLeadsGate(w, r, req.OwnerID, req.ComputerID) {
		return
	}
	n, err := h.captureIndex.MarkSearchLeadsTried(r.Context(), req.OwnerID, req.ComputerID, req.URLs, req.CaptureID)
	if err != nil {
		searchLeadsFailure(w, "lead_tried", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"marked": n})
}
