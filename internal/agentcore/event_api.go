package agentcore

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/store"
)

// HandleWorldEvents serves GET /api/autopaper/events: the computer's world
// events, newest first, with their members (event layer V2). Read-only:
// what the owner, or a judge sampling memberships for V1's acceptance,
// reads of the event layer without a desk. limit is at most 300 (default
// 50); corroborated=true keeps corroborated events only.
func (h *APIHandler) HandleWorldEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	limit := 50
	if n, err := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("limit"))); err == nil && n > 0 {
		limit = min(n, 300)
	}
	onlyCorroborated := r.URL.Query().Get("corroborated") == "true"
	rt := h.rt
	computerID := rt.TextureComputerID()
	idx := rt.eventIndexFor(ownerID, computerID)
	idx.mu.Lock()
	if err := idx.loadLocked(r.Context(), rt.store, ownerID, computerID); err != nil {
		idx.mu.Unlock()
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: err.Error()})
		return
	}
	type member struct {
		store.EventMembership
		Counted string `json:"counted_origin,omitempty"`
	}
	type eventOut struct {
		eventSummary
		CorroboratedAt *time.Time `json:"corroborated_at,omitempty"`
		MemberList     []member   `json:"member_list"`
	}
	var out []eventOut
	for _, v := range idx.events {
		if onlyCorroborated && !v.corroborated() {
			continue
		}
		e := eventOut{eventSummary: v.summary()}
		if !v.corroboratedAt.IsZero() {
			at := v.corroboratedAt
			e.CorroboratedAt = &at
		}
		for _, m := range v.memberships {
			m.OwnerID, m.ComputerID = "", ""
			e.MemberList = append(e.MemberList, member{EventMembership: m, Counted: m.CountedOrigin()})
		}
		out = append(out, e)
	}
	idx.mu.Unlock()
	sort.Slice(out, func(i, j int) bool { return out[i].LastSeen.After(out[j].LastSeen) })
	total := len(out)
	if len(out) > limit {
		out = out[:limit]
	}
	writeAPIJSON(w, http.StatusOK, map[string]any{"events": out, "total": total})
}
