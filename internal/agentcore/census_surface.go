package agentcore

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/vocabmigrate"
)

// censusLimit bounds the objects listed per kind; a census that reaches it
// says so (Truncated) rather than undercounting silently.
const censusLimit = 5000

type censusObject struct {
	Kind         string    `json:"kind"`
	CanonicalID  string    `json:"canonical_id"`
	TrajectoryID string    `json:"trajectory_id,omitempty"`
	From         string    `json:"from,omitempty"`
	To           string    `json:"to,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

type censusResponse struct {
	ComputerID  string         `json:"computer_id"`
	GeneratedAt time.Time      `json:"generated_at"`
	Kinds       []string       `json:"kinds"`
	Counts      map[string]int `json:"counts"`
	Objects     []censusObject `json:"objects"`
	Truncated   bool           `json:"truncated,omitempty"`
}

// HandleCensus serves the owner's census of retired work-state objects
// (vocabmigrate.RetiredWorkStateKinds) on this computer, each with its
// route, so the checker reads an authoritative list instead of inferring
// absence from a snapshot field that may be gone (C1e B0).
func (h *APIHandler) HandleCensus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	resp := censusResponse{
		ComputerID:  h.rt.TextureComputerID(),
		GeneratedAt: time.Now().UTC(),
		Kinds:       append([]string(nil), vocabmigrate.RetiredWorkStateKinds...),
		Counts:      map[string]int{},
		Objects:     []censusObject{},
	}
	for _, kind := range resp.Kinds {
		objs, err := h.rt.store.ListOwnerObjectsOfKind(r.Context(), kind, ownerID, censusLimit)
		if err != nil {
			writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "census " + kind + ": " + err.Error()})
			return
		}
		if len(objs) >= censusLimit {
			resp.Truncated = true
		}
		resp.Counts[kind] = len(objs)
		for _, o := range objs {
			var body map[string]any
			_ = json.Unmarshal(o.Body, &body)
			resp.Objects = append(resp.Objects, censusObject{
				Kind:         kind,
				CanonicalID:  o.CanonicalID,
				TrajectoryID: bodyString(body, "trajectory_id"),
				From:         bodyString(body, "agent_id", "created_by_loop_id"),
				To:           bodyString(body, "target_agent_id", "assigned_agent_id"),
				CreatedAt:    o.CreatedAt,
			})
		}
	}
	writeAPIJSON(w, http.StatusOK, resp)
}

// bodyString is the first non-empty string field among keys.
func bodyString(body map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := body[k].(string); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
