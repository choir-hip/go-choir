package proxy

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func isComputerSurfacePath(path string) bool {
	switch {
	case path == "/api" || strings.HasPrefix(path, "/api/"):
		return false
	case path == "/auth" || strings.HasPrefix(path, "/auth/"):
		return false
	case path == "/health" || strings.HasPrefix(path, "/health/"):
		return false
	case strings.HasPrefix(path, "/provider/") || strings.HasPrefix(path, "/internal/"):
		return false
	default:
		return true
	}
}

// HandleComputerSurface reverse-proxies Desktop/Texture/apps/Settings/assets
// to the vmctl-resolved guest after authentication. Unsigned callers receive
// the host platform shell (picker/auth chrome), which stays OUT of restore.
func (h *Handler) HandleComputerSurface(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	if !isComputerSurfacePath(r.URL.Path) {
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "not found"})
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
		return
	}

	authStarted := time.Now()
	authResult, err := h.authenticate(r)
	if err != nil {
		h.lifecycle.record("surface.auth", "unauthorized", time.Since(authStarted))
		h.servePlatformShell(w, r)
		h.lifecycle.record("surface.total", "platform_shell", time.Since(started))
		return
	}
	h.lifecycle.record("surface.auth", "ok", time.Since(authStarted))
	if !h.authorizeAPIKeyScope(w, r, authResult) {
		h.lifecycle.record("surface.authz", "forbidden", time.Since(authStarted))
		h.lifecycle.record("surface.total", "forbidden", time.Since(started))
		return
	}
	if !preserveOriginalRequestTarget(r) {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid request path"})
		h.lifecycle.record("surface.path", "invalid", time.Since(started))
		h.lifecycle.record("surface.total", "invalid_path", time.Since(started))
		return
	}

	desktopID := requestDesktopID(r)
	computerTarget, ok := h.requireAPIKeyComputerTarget(w, r, authResult, "", desktopID)
	if !ok {
		h.lifecycle.record("surface.authz", "forbidden", time.Since(authStarted))
		h.lifecycle.record("surface.total", "forbidden", time.Since(started))
		return
	}
	resolveStarted := time.Now()
	autoputerURL, err := h.resolveComputerURLForComputerTarget(r.Context(), authResult, computerTarget, desktopID)
	if err != nil {
		log.Printf("proxy: failed to resolve computer surface for owner %s desktop %s: %v", authResult.UserID, desktopID, err)
		writeResolveError(w, err)
		h.lifecycle.record("surface.resolve", "error", time.Since(resolveStarted))
		h.lifecycle.record("surface.total", "resolve_error", time.Since(started))
		return
	}
	h.lifecycle.record("surface.resolve", "ok", time.Since(resolveStarted))
	h.setTrustedAuthHeaders(r, authResult)
	if autoputerURL != h.cfg.ComputerURL {
		r.Header.Set("X-Resolved-Autoputer-URL", autoputerURL)
	}

	upstreamStarted := time.Now()
	recorder := &lifecycleStatusRecorder{ResponseWriter: w}
	h.reverseProxy.ServeHTTP(recorder, r)
	h.lifecycle.record("surface.upstream", lifecycleHTTPStatus(recorder.status), time.Since(upstreamStarted))
	h.lifecycle.record("surface.total", lifecycleHTTPStatus(recorder.status), time.Since(started))
}

func (h *Handler) servePlatformShell(w http.ResponseWriter, r *http.Request) {
	root := ""
	if h != nil && h.cfg != nil {
		root = strings.TrimSpace(h.cfg.PlatformShellRoot)
	}
	if root == "" {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "authentication required"})
		return
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "authentication required"})
		return
	}
	index := filepath.Join(abs, "index.html")
	if info, err := os.Stat(index); err != nil || info.IsDir() {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "authentication required"})
		return
	}
	rel := strings.TrimPrefix(filepath.Clean("/"+r.URL.Path), "/")
	if rel == "." {
		rel = "index.html"
	}
	target := filepath.Join(abs, filepath.FromSlash(rel))
	if !strings.HasPrefix(target, abs+string(filepath.Separator)) && target != abs {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Error: "authentication required"})
		return
	}
	if info, err := os.Stat(target); err != nil || info.IsDir() {
		target = index
		w.Header().Set("Cache-Control", "no-store")
	}
	http.ServeFile(w, r, target)
}
