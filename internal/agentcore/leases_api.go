package agentcore

import (
	"net/http"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/lease"
)

// The owner's view of leases (MA3.4): GET /api/leases lists every lease of the
// computer as a record and a sentence; POST /api/leases/{id}/revoke ends one
// at once, and the holder's next read fails. Both are owner routes: the owner
// is the authenticated user, and no desk reaches them (a desk's ProductAPI
// call does not list /api/leases as a route it may use).

type leaseView struct {
	lease.Record
	Sentence string `json:"sentence"`
}

type leaseListResponse struct {
	Leases []leaseView `json:"leases"`
}

// HandleLeases serves GET /api/leases.
func (h *APIHandler) HandleLeases(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	svc := h.rt.Leases()
	if svc == nil {
		writeAPIJSON(w, http.StatusOK, leaseListResponse{Leases: []leaseView{}})
		return
	}
	recs, err := svc.All(r.Context(), ownerID, h.rt.TextureComputerID())
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: err.Error()})
		return
	}
	out := leaseListResponse{Leases: make([]leaseView, 0, len(recs))}
	for _, rec := range recs {
		out.Leases = append(out.Leases, leaseView{Record: rec, Sentence: rec.Sentence()})
	}
	writeAPIJSON(w, http.StatusOK, out)
}

// HandleLeaseRouter serves POST /api/leases/{id}/revoke.
func (h *APIHandler) HandleLeaseRouter(w http.ResponseWriter, r *http.Request) {
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	rest := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/leases/"), "/")
	id, action, _ := strings.Cut(rest, "/")
	if id == "" || action != "revoke" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	svc := h.rt.Leases()
	if svc == nil {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "no lease service"})
		return
	}
	// Only the owner's own leases: the live index carries the owner.
	owned := false
	for _, rec := range svc.Live(ownerID) {
		if rec.LeaseID == id {
			owned = true
		}
	}
	if !owned {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "no such live lease"})
		return
	}
	if err := svc.Revoke(r.Context(), id, lease.ReasonOwner); err != nil {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: err.Error()})
		return
	}
	writeAPIJSON(w, http.StatusOK, map[string]string{"lease_id": id, "state": lease.StateRevoked})
}
