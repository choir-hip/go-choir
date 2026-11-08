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
//	GET /api/packages/files/<sha256>.json    one data file (a theme pack's theme)
//	GET /api/packages/reputation?publisher=<track>  a publisher's track record as sentences (MA7.6)
//
// All are the owner's reads on their own computer (the proxy forwards
// /api/* with the authenticated owner). A module is served only when an
// install in force names it, so the store is not a general file server.

const (
	packagesInstalledPath = "/api/packages/installed"
	packageModulesPrefix  = "/api/packages/modules/"
	packageFilesPrefix    = "/api/packages/files/"
)

// InstalledPackage is one installed package as the shell lists it.
type InstalledPackage struct {
	Name        string `json:"name"`
	ReleaseID   string `json:"release_id"`
	InstallID   string `json:"install_id"`
	InstalledAt string `json:"installed_at"`
	Publisher   string `json:"publisher_computer_id,omitempty"`
	// Track is the owner id of the track the release is on: the shell tells
	// the owner's own track from a stranger's by it.
	Track string `json:"track,omitempty"`
	// Isolation is where the shell runs the package's code (MA6.1): "shell"
	// for a release on the owner's own track (the owner's code, in the
	// shell's realm like the base packages), "frame" for every other track
	// (an opaque-origin sandboxed iframe that reaches the shell only through
	// the bridge, grant-checked per call). The shell treats any other value,
	// or none, as "frame".
	Isolation string `json:"isolation"`
	// Uses are the SDK names the computer's build extracted from the
	// package's source: the bridge answers a framed package only for these.
	Uses        []string        `json:"uses"`
	RequiresSDK string          `json:"requires_sdk"`
	Manifest    json.RawMessage `json:"manifest"`
	// Module is empty for a pack (a package with no app: its themes,
	// commands and keybindings are data in the manifest and Files).
	Module InstalledModule `json:"module"`
	// Files are the data files the manifest names (theme files), each served
	// at URL; the shell resolves contributes.themes[].file through them.
	Files     []InstalledFile `json:"files,omitempty"`
	Available bool            `json:"available"`
	// Update is the last update sync's view of the package (F1): an update
	// available, one that needs the owner's approval, or what the platform
	// did. Absent until a sync has run.
	Update *PackageUpdateView `json:"update,omitempty"`
}

// Where the shell runs an installed package's code (MA6.1).
const (
	isolationShell = "shell"
	isolationFrame = "frame"
)

// packageIsolation is "shell" only for a release on the owner's own track;
// a stranger's release, or one whose track is unknown, runs in the frame.
func packageIsolation(track, ownerID string) string {
	if track != "" && track == ownerID {
		return isolationShell
	}
	return isolationFrame
}

// InstalledFile is one data file of an installed package.
type InstalledFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	URL    string `json:"url"`
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
	case r.URL.Path == packagesReviewsPath:
		h.servePackageReviews(w, r, ownerID, computerID)
	case r.URL.Path == packagesReputationPath:
		h.servePackageReputation(w, r, ownerID, computerID)
	case r.URL.Path == packagesInstalledPath:
		out := make([]InstalledPackage, 0, len(current))
		for name, item := range current {
			var module InstalledModule
			if item.ModuleSHA256 != "" {
				module = InstalledModule{ABI: item.ModuleABI, SHA256: item.ModuleSHA256, URL: packageModulesPrefix + item.ModuleSHA256 + ".js"}
			}
			var files []InstalledFile
			for _, f := range item.Files {
				files = append(files, InstalledFile{Path: f.Path, SHA256: f.SHA256, URL: packageFilesPrefix + f.SHA256 + ".json"})
			}
			out = append(out, InstalledPackage{
				Name: name, ReleaseID: item.ReleaseID, InstallID: item.InstallID,
				InstalledAt: item.RequestedAt.UTC().Format("2006-01-02T15:04:05.000Z"),
				Publisher:   item.PublisherComputerID, Track: item.PublisherTrack, RequiresSDK: item.RequiresSDK, Manifest: item.Manifest,
				Isolation: packageIsolation(item.PublisherTrack, ownerID), Uses: append([]string{}, item.Uses...),
				Module: module, Files: files,
				Available: rt.installBytesPresent(*item),
				Update:    rt.packageUpdateView(name),
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
	case strings.HasPrefix(r.URL.Path, packageFilesPrefix):
		sha, ok := strings.CutSuffix(strings.TrimPrefix(r.URL.Path, packageFilesPrefix), ".json")
		named := false
		for _, item := range current {
			for _, f := range item.Files {
				if ok && f.SHA256 == sha {
					named = true
					break
				}
			}
			if named {
				break
			}
		}
		if !named {
			writeAPIJSON(w, http.StatusNotFound, apiError{Error: "file not found"})
			return
		}
		raw, err := rt.packageModules.ReadFile(sha)
		if err != nil {
			writeAPIJSON(w, http.StatusNotFound, apiError{Error: "file not found"})
			return
		}
		header := w.Header()
		header.Set("Content-Type", "application/json; charset=utf-8")
		header.Set("X-Content-Type-Options", "nosniff")
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
