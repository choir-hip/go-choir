package api

import (
	"fmt"
	"net/http"

	"github.com/yusefmosiah/go-choir/internal/base/blob"
	"github.com/yusefmosiah/go-choir/internal/base/journal"
)

// PersistentHandlerConfig names the Base persistence paths for a writable Base
// API handler. Opening this boundary creates or migrates the journal/blob roots;
// read-only observation paths must use computerversion.OpenBaseCurrentStateSource
// instead.
type PersistentHandlerConfig struct {
	JournalPath string `json:"journal_path"`
	BlobRoot    string `json:"blob_root"`
}

// RouteRegistrar is the small server surface needed to mount the Base API route
// tree. *http.ServeMux and internal/server.Server both satisfy it.
type RouteRegistrar interface {
	Handle(pattern string, handler http.Handler)
}

// PersistentHandler owns the writable persistence handles backing a Base API
// handler. It is a small non-runtime wiring boundary: cmd packages can provide
// paths and auth, while tests can prove those paths later feed read-only
// observation.
type PersistentHandler struct {
	Handler *Handler
	journal *journal.SQLiteJournal
}

// OpenPersistentHandler opens the configured Base journal/blob persistence and
// wires them into a Handler. The caller owns the returned handler and must close
// it when the service shuts down.
func OpenPersistentHandler(cfg PersistentHandlerConfig, validator APIKeyValidator) (*PersistentHandler, error) {
	if cfg.JournalPath == "" {
		return nil, fmt.Errorf("base api persistent handler: journal path is required")
	}
	if cfg.BlobRoot == "" {
		return nil, fmt.Errorf("base api persistent handler: blob root is required")
	}
	blobs, err := blob.NewStore(cfg.BlobRoot)
	if err != nil {
		return nil, fmt.Errorf("base api persistent handler: open blob store: %w", err)
	}
	jr, err := journal.NewSQLiteJournal(cfg.JournalPath)
	if err != nil {
		return nil, fmt.Errorf("base api persistent handler: open journal: %w", err)
	}
	return &PersistentHandler{Handler: NewHandler(blobs, jr, validator), journal: jr}, nil
}

// Routes returns the wrapped Base API routes.
func (h *PersistentHandler) Routes() http.Handler {
	if h == nil || h.Handler == nil {
		return http.NewServeMux()
	}
	return h.Handler.Routes()
}

// RegisterPersistentRoutes mounts the persistent Base API route tree under
// /api/base/. This is a local wiring helper only; calling it from a deployed cmd
// service is a red mutation under the mission definition.
func RegisterPersistentRoutes(registrar RouteRegistrar, handler *PersistentHandler) error {
	if registrar == nil {
		return fmt.Errorf("base api persistent routes: registrar is required")
	}
	if handler == nil || handler.Handler == nil {
		return fmt.Errorf("base api persistent routes: handler is required")
	}
	registrar.Handle("/api/base/", handler.Routes())
	return nil
}

// Close releases the writable journal handle.
func (h *PersistentHandler) Close() error {
	if h == nil || h.journal == nil {
		return nil
	}
	return h.journal.Close()
}
