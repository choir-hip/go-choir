package platform

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
)

// The bound-guest routes of the op-ed and notice feeds
// (autopaper contracts 2 and 3). A guest's owner is the owner vmctl binds to
// its address; the routes take no owner parameter from a guest. The proxy,
// which relays a guest as the trusted internal caller, overwrites owner_id
// with the owner it bound.

const (
	opedFeedPath     = "/internal/platform/opeds"
	noticeFeedPath   = "/internal/platform/notices"
	opedVerdictsPath = "/internal/platform/oped-verdicts"
)

// callerOwner is the owner a feed or verdict request is for: the proxy's
// bound owner for a trusted internal caller, else the owner vmctl binds to
// the calling guest. A caller that is neither is refused.
func (h *Handler) callerOwner(r *http.Request, claimed string) (string, bool) {
	if trustedInternalCaller(r) {
		owner := strings.TrimSpace(claimed)
		return owner, owner != ""
	}
	ownership, err := bindTapCaller(r.Context(), h.guestLookup, r)
	if err != nil || strings.TrimSpace(ownership.UserID) == "" {
		return "", false
	}
	return strings.TrimSpace(ownership.UserID), true
}

func feedParams(r *http.Request) (cursor string, limit int, ok bool) {
	q := r.URL.Query()
	cursor = strings.TrimSpace(q.Get("after"))
	if raw := strings.TrimSpace(q.Get("limit")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 {
			return "", 0, false
		}
		limit = n
	}
	return cursor, limit, true
}

// HandleInternalOpeds serves GET /internal/platform/opeds?after=&limit=: the
// op-eds on the bound owner's stories.
func (h *Handler) HandleInternalOpeds(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	owner, ok := h.callerOwner(r, r.URL.Query().Get("owner_id"))
	if !ok {
		writeJSON(w, http.StatusForbidden, apiError{Error: "internal caller required"})
		return
	}
	cursor, limit, ok := feedParams(r)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "limit must be a non-negative integer"})
		return
	}
	feed, err := h.service.ListOpedsForOwner(r.Context(), owner, cursor, limit)
	if err != nil {
		log.Printf("corpusd: op-ed feed: %v", err)
		writeJSON(w, http.StatusBadRequest, apiError{Error: "failed to read the op-ed feed"})
		return
	}
	writeJSON(w, http.StatusOK, feed)
}

// HandleInternalNotices serves GET /internal/platform/notices?after=&limit=:
// the bound owner's changelog notices.
func (h *Handler) HandleInternalNotices(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	owner, ok := h.callerOwner(r, r.URL.Query().Get("owner_id"))
	if !ok {
		writeJSON(w, http.StatusForbidden, apiError{Error: "internal caller required"})
		return
	}
	cursor, limit, ok := feedParams(r)
	if !ok {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "limit must be a non-negative integer"})
		return
	}
	feed, err := h.service.ListNoticesForOwner(r.Context(), owner, cursor, limit)
	if err != nil {
		log.Printf("corpusd: notice feed: %v", err)
		writeJSON(w, http.StatusBadRequest, apiError{Error: "failed to read the notice feed"})
		return
	}
	writeJSON(w, http.StatusOK, feed)
}

// HandleInternalOpedVerdicts serves POST /internal/platform/oped-verdicts:
// the paper's computer posting its gate verdict on an op-ed to one of its
// stories. A verdict for another owner's story is 403.
func (h *Handler) HandleInternalOpedVerdicts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<16))
	var post OpedVerdictPost
	if err != nil || json.Unmarshal(raw, &post) != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "invalid request body"})
		return
	}
	owner, ok := h.callerOwner(r, post.OwnerID)
	if !ok {
		writeJSON(w, http.StatusForbidden, apiError{Error: "internal caller required"})
		return
	}
	post.OwnerID = owner
	state, err := h.service.PostOpedVerdict(r.Context(), post)
	switch {
	case errors.Is(err, ErrNotYourStory):
		writeJSON(w, http.StatusForbidden, apiError{Error: err.Error()})
	case err != nil:
		log.Printf("corpusd: oped verdict: %v", err)
		writeJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
	default:
		writeJSON(w, http.StatusOK, state)
	}
}
