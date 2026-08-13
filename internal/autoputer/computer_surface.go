package autoputer

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const computerSurfacePrefix = "frontend"

// ComputerSurface serves the computer-surface SPA from the staged guest
// release. The bytes live under CHOIR_UPDATER_ROOT/current/frontend; a missing
// index.html is fail-closed so a digest without a serving join cannot green.
type ComputerSurface struct {
	Root string
}

func ComputerSurfaceRoot(updaterRoot string) string {
	return filepath.Join(strings.TrimSpace(updaterRoot), "current", computerSurfacePrefix)
}

func NewComputerSurfaceFromEnv() *ComputerSurface {
	return &ComputerSurface{Root: ComputerSurfaceRoot(os.Getenv("CHOIR_UPDATER_ROOT"))}
}

func (h *ComputerSurface) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	root, err := h.resolvedRoot()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	rel := strings.TrimPrefix(filepath.Clean("/"+r.URL.Path), "/")
	if rel == "." {
		rel = "index.html"
	}
	target := filepath.Join(root, filepath.FromSlash(rel))
	if !strings.HasPrefix(target, root+string(filepath.Separator)) && target != root {
		http.Error(w, "self-development checkpoint: served SPA is underivable", http.StatusBadRequest)
		return
	}
	info, err := os.Stat(target)
	if err != nil || info.IsDir() {
		target = filepath.Join(root, "index.html")
		info, err = os.Stat(target)
		if err != nil || info.IsDir() {
			http.Error(w, "self-development checkpoint: served SPA is underivable", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
	} else if strings.HasPrefix(rel, "assets/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else if rel == "index.html" {
		w.Header().Set("Cache-Control", "no-store")
	}
	http.ServeFile(w, r, target)
}

func (h *ComputerSurface) resolvedRoot() (string, error) {
	if h == nil || strings.TrimSpace(h.Root) == "" {
		return "", fmt.Errorf("self-development checkpoint: served SPA is underivable")
	}
	root, err := filepath.Abs(h.Root)
	if err != nil {
		return "", fmt.Errorf("self-development checkpoint: served SPA is underivable")
	}
	index := filepath.Join(root, "index.html")
	info, err := os.Stat(index)
	if err != nil || info.IsDir() {
		return "", fmt.Errorf("self-development checkpoint: served SPA is underivable")
	}
	return root, nil
}

func RegisterComputerSurface(s interface{ Handle(string, http.Handler) }, surface *ComputerSurface) {
	if s == nil || surface == nil {
		return
	}
	s.Handle("/", surface)
}
