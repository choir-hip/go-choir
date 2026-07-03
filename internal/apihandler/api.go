// Package apihandler provides the public HTTP API surface for the runtime
// service. It is the extraction target for the runtime API handlers as part of
// Mission A (actor-runtime defactoring, State 3).
//
// During the defactoring the actual handler methods still live inside the
// internal/runtime package. The types in this package wrap the runtime
// APIHandler so callers outside runtime can construct and register handlers
// without importing the runtime package directly. Later passes will migrate the
// handler methods and their business-logic dependencies out of runtime.
package apihandler

import (
	"github.com/yusefmosiah/go-choir/internal/runtime"
	"github.com/yusefmosiah/go-choir/internal/server"
)

// Handler exposes the runtime API HTTP handlers. It wraps the runtime
// internal APIHandler while the handler methods are being migrated.
type Handler struct {
	*runtime.APIHandler
}

// NewAPIHandler creates a Handler for the given runtime.
func NewAPIHandler(rt *runtime.Runtime) *Handler {
	return &Handler{runtime.NewAPIHandler(rt)}
}

// RegisterRoutes registers runtime API routes on the given server.
func RegisterRoutes(s *server.Server, h *Handler) {
	runtime.RegisterRoutes(s, h.APIHandler)
}

// RegisterTextureRoutes registers the Texture API routes on the given server.
func RegisterTextureRoutes(s *server.Server, h *Handler) {
	runtime.RegisterTextureRoutes(s, h.APIHandler)
}
