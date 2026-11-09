package proxy

import (
	"net/http"
	"net/url"
	"strings"
)

// GET /api/computers/{id}/update: the host-side "Updates available" signal
// (docs/design/update-now-2026-11-09.md item 9). It is composed from vmctl's
// pending tracker, not from the guest, so it survives a guest restart and
// shows for a hibernate-fenced computer. The target is the release commit.

func computerUpdateSignalComputerID(path string) (string, bool) {
	const prefix = "/api/computers/"
	if !strings.HasPrefix(path, prefix) {
		return "", false
	}
	parts := strings.Split(strings.TrimPrefix(path, prefix), "/")
	if len(parts) != 2 || parts[1] != "update" {
		return "", false
	}
	computerID, err := url.PathUnescape(parts[0])
	if err != nil || strings.TrimSpace(computerID) == "" {
		return "", false
	}
	return strings.TrimSpace(computerID), true
}

func isComputerUpdateSignalPath(path string) bool {
	_, ok := computerUpdateSignalComputerID(path)
	return ok
}

func (h *Handler) HandleComputerUpdateSignal(w http.ResponseWriter, r *http.Request) {
	computerID, ok := computerUpdateSignalComputerID(r.URL.Path)
	if !ok {
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "not found"})
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}
	authResult, err := h.authenticate(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "authentication required"})
		return
	}
	// An owner session, or an api key with read:runtime (admin included).
	if !h.authorizeAPIKeyScope(w, r, authResult) {
		return
	}
	var target *resolvedComputerTarget
	if authResult.AuthMethod == "api_key" {
		target, ok = h.requireAPIKeyComputerTarget(w, r, authResult, computerID, "")
		if !ok {
			return
		}
	} else {
		target, err = h.resolveAuthorizedComputer(r.Context(), authResult, computerID)
		if err != nil || target == nil || target.ComputerID != computerID {
			writeJSON(w, http.StatusNotFound, errorResponse{Error: "computer not found"})
			return
		}
	}
	if h.vmctlClient == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "computer ownership authority unavailable"})
		return
	}
	signal, err := h.vmctlClient.UpdateAvailable(r.Context(), target.UserID, target.DesktopID)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "update signal unavailable"})
		return
	}
	body := map[string]any{"computer_id": target.ComputerID, "update_available": nil}
	if signal.Available {
		body["update_available"] = map[string]string{"target": signal.Target, "since": signal.Since}
	}
	writeJSON(w, http.StatusOK, body)
}
