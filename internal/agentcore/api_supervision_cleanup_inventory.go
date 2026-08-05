package agentcore

import (
	"log"
	"net/http"
	"os"
	"strings"
)

// HandleSupervisionCleanupInventory exposes a bounded, read-only inventory of
// canonical supervision residue and owner data for the current computer. It is
// temporary acceptance instrumentation for the deletion-first cutover.
func (h *APIHandler) HandleSupervisionCleanupInventory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	computerID := strings.TrimSpace(os.Getenv("CHOIR_COMPUTER_ID"))
	if h == nil || h.rt == nil || h.rt.Store() == nil || computerID == "" {
		writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "supervision cleanup inventory unavailable"})
		return
	}
	inventory, err := h.rt.Store().SupervisionCleanupInventory(r.Context(), ownerID, computerID)
	if err != nil {
		log.Printf("runtime api: supervision cleanup inventory: %v", err)
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "supervision cleanup inventory failed"})
		return
	}
	inventory.RealizationID = strings.TrimSpace(os.Getenv("CHOIR_REALIZATION_ID"))
	writeAPIJSON(w, http.StatusOK, inventory)
}
