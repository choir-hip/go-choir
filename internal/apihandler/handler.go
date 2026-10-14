package apihandler

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/yusefmosiah/go-choir/internal/server"
	"github.com/yusefmosiah/go-choir/internal/store"
)

// Handler provides HTTP handlers backed by the application store.
type Handler struct {
	store *store.Store
}

// NewHandler creates a Handler backed by the given store.
func NewHandler(store *store.Store) *Handler {
	return &Handler{store: store}
}

// apiError is the JSON error envelope for API responses.
type apiError struct {
	Error string `json:"error"`
}

// authenticateUser extracts the authenticated user identity from the trusted
// X-Authenticated-User header injected by the proxy.
// authenticateUser is the guest's one owner-identity rule: the proxy's
// X-Authenticated-User, believed only from the host side of the guest
// boundary, never from an in-guest caller
// (docs/problems/auth-boundary-gaps-from-surface-audit-2026-10-14.md F2).
var authenticateUser = server.AuthenticatedOwner

func writeAPIJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("api handler: json encode error: %v", err)
	}
}
