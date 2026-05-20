package sandbox

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/server"
)

// BootstrapResponse is the JSON payload returned by GET /api/shell/bootstrap.
// It includes the sandbox identity, the authenticated user context forwarded by
// the proxy, and a bootstrap payload for the shell.
type BootstrapResponse struct {
	SandboxID  string `json:"sandbox_id"`
	User       string `json:"user,omitempty"`
	Bootstrap  string `json:"bootstrap"`
	Path       string `json:"path"`
	Method     string `json:"method"`
	Query      string `json:"query,omitempty"`
	StatusCode int    `json:"status_code"`
}

// ErrorResponse is the JSON payload returned by deliberate non-2xx paths.
type ErrorResponse struct {
	SandboxID  string `json:"sandbox_id"`
	StatusCode int    `json:"status_code"`
	Error      string `json:"error"`
}

// Handler provides the placeholder sandbox HTTP handlers.
type Handler struct {
	cfg Config
}

// NewHandler creates a sandbox handler with the given sandbox ID.
func NewHandler(sandboxID string) *Handler {
	return &Handler{
		cfg: Config{SandboxID: sandboxID},
	}
}

// HandleBootstrap handles GET /api/shell/bootstrap.
// It returns the shell bootstrap payload including sandbox identity,
// authenticated user context, and request echo data.
func (h *Handler) HandleBootstrap(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	user := r.Header.Get("X-Authenticated-User")

	resp := BootstrapResponse{
		SandboxID:  h.cfg.SandboxID,
		User:       user,
		Bootstrap:  "placeholder-shell-v1",
		Path:       r.URL.Path,
		Method:     r.Method,
		Query:      r.URL.RawQuery,
		StatusCode: 200,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// HandleError returns a deliberate 500 response for proxy passthrough testing.
func (h *Handler) HandleError(w http.ResponseWriter, r *http.Request) {
	resp := ErrorResponse{
		SandboxID:  h.cfg.SandboxID,
		StatusCode: 500,
		Error:      "deliberate sandbox error for passthrough testing",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	_ = json.NewEncoder(w).Encode(resp)
}

// RegisterRoutes registers all sandbox routes on the given server.
func RegisterRoutes(s *server.Server, h *Handler) {
	s.HandleFunc("/api/shell/bootstrap", h.HandleBootstrap)
	if sandboxTestRoutesEnabled() {
		s.HandleFunc("/api/shell/error", h.HandleError)
	}
}

func sandboxTestRoutesEnabled() bool {
	switch strings.TrimSpace(strings.ToLower(os.Getenv("RUNTIME_ENABLE_TEST_APIS"))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
