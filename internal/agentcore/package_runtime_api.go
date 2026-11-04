package agentcore

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/pkgruntime"
)

// The shell's side of runtime install (S12,
// docs/design/s12-runtime-install-build-2026-11-04.md):
//
//	GET /api/packages/installed              this computer's installed packages
//	GET /api/packages/modules/<sha256>.js    one module, verified against its name
//
// Both are the owner's reads on their own computer (the proxy forwards
// /api/* with the authenticated owner). A module is served only when an
// install in force names it, so the store is not a general file server.

const (
	packagesInstalledPath = "/api/packages/installed"
	packageModulesPrefix  = "/api/packages/modules/"
)

// InstalledPackage is one installed package as the shell lists it.
type InstalledPackage struct {
	Name        string          `json:"name"`
	ReleaseID   string          `json:"release_id"`
	InstallID   string          `json:"install_id"`
	InstalledAt string          `json:"installed_at"`
	Publisher   string          `json:"publisher_computer_id,omitempty"`
	RequiresSDK string          `json:"requires_sdk"`
	Manifest    json.RawMessage `json:"manifest"`
	Module      InstalledModule `json:"module"`
	Available   bool            `json:"available"`
}

// InstalledModule names the module the shell fetches and checks.
type InstalledModule struct {
	ABI    string `json:"abi"`
	SHA256 string `json:"sha256"`
	URL    string `json:"url"`
}

// HandlePackages serves /api/packages/.
func (h *APIHandler) HandlePackages(w http.ResponseWriter, r *http.Request) {
	ownerID, err := authenticateUser(r)
	if err != nil {
		writeAPIJSON(w, http.StatusUnauthorized, apiError{Error: "authentication required"})
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if h == nil || h.rt == nil || h.rt.store == nil {
		writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "packages unavailable"})
		return
	}
	rt := h.rt
	computerID := strings.TrimSpace(rt.cfg.ComputerID)
	records, err := rt.packageInstalls(r.Context(), ownerID, computerID)
	if err != nil {
		writeAPIJSON(w, http.StatusInternalServerError, apiError{Error: "installs unreadable"})
		return
	}
	current := currentPackageInstalls(records)

	switch {
	case r.URL.Path == packagesInstalledPath:
		out := make([]InstalledPackage, 0, len(current))
		for name, item := range current {
			_, readErr := rt.packageModules.Read(item.ModuleSHA256)
			out = append(out, InstalledPackage{
				Name: name, ReleaseID: item.ReleaseID, InstallID: item.InstallID,
				InstalledAt: item.RequestedAt.UTC().Format("2006-01-02T15:04:05.000Z"),
				Publisher:   item.PublisherComputerID, RequiresSDK: item.RequiresSDK, Manifest: item.Manifest,
				Module:    InstalledModule{ABI: item.ModuleABI, SHA256: item.ModuleSHA256, URL: packageModulesPrefix + item.ModuleSHA256 + ".js"},
				Available: readErr == nil,
			})
		}
		sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
		w.Header().Set("Cache-Control", "no-store")
		writeAPIJSON(w, http.StatusOK, map[string]any{"sdk_abi": pkgruntime.ModuleABI, "packages": out})
	case strings.HasPrefix(r.URL.Path, packageModulesPrefix):
		sha, ok := strings.CutSuffix(strings.TrimPrefix(r.URL.Path, packageModulesPrefix), ".js")
		named := false
		for _, item := range current {
			if ok && item.ModuleSHA256 == sha {
				named = true
				break
			}
		}
		if !named {
			writeAPIJSON(w, http.StatusNotFound, apiError{Error: "module not found"})
			return
		}
		raw, err := rt.packageModules.Read(sha)
		if err != nil {
			writeAPIJSON(w, http.StatusNotFound, apiError{Error: "module not found"})
			return
		}
		header := w.Header()
		header.Set("Content-Type", "text/javascript; charset=utf-8")
		header.Set("X-Content-Type-Options", "nosniff")
		// Content-addressed: the bytes behind a name never change.
		header.Set("Cache-Control", "private, max-age=31536000, immutable")
		header.Set("ETag", `"`+sha+`"`)
		w.WriteHeader(http.StatusOK)
		if r.Method == http.MethodGet {
			_, _ = w.Write(raw)
		}
	default:
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "not found"})
	}
}
