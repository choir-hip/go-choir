package autoputer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/apppackage"
	"github.com/yusefmosiah/go-choir/internal/server"
	"github.com/yusefmosiah/go-choir/internal/store"
)

// App-package guest API (S7-0a, docs/design/s7-0-app-packages-2026-10-14.md):
//
//	GET  /api/app-packages                         installed packages (the tape fold)
//	GET  /api/app-packages/catalog                 builds the trusted sources offer
//	POST /api/app-packages/install                 {"build_digest": "sha256:…"}; never bytes
//	POST /api/app-packages/{name}/{enable|disable|purge}
//	GET|PUT /api/app-packages/{name}/data/{key}    package_kv/v1, body {"value": …}
//	GET  /api/app-packages/builds/{hex}/{path…}    an enabled package's build file
//
// Every route acts as the owner and requires the proxy-authenticated owner.

const appPackagePrefix = "/api/app-packages"

// appPackageStore is the store surface the service needs; *store.Store
// implements it.
type appPackageStore interface {
	AppPackageInventory(ctx context.Context, computerID string) (apppackage.Inventory, error)
	RecordAppPackageEvent(ctx context.Context, ownerID, computerID string, ev apppackage.Event) (apppackage.Inventory, apppackage.Event, error)
	PutAppPackageDatum(ctx context.Context, ownerID, computerID string, d apppackage.Datum) error
	GetAppPackageDatum(ctx context.Context, computerID, pkg, key string) (apppackage.Datum, error)
}

// AppPackageService serves the guest app-package API for one computer.
type AppPackageService struct {
	Store      appPackageStore
	Builds     apppackage.BuildStore // the computer's installed builds
	Sources    []apppackage.Source   // trusted build sources, in order
	ComputerID string
}

// NewAppPackageServiceFromEnv wires the service to the runtime store. The
// build store lives beside the runtime store; the catalog is the
// release-shipped directory under the updater's current release, overridable
// with CHOIR_APP_PACKAGE_CATALOG.
func NewAppPackageServiceFromEnv(db appPackageStore, computerID, storePath string) *AppPackageService {
	svc := &AppPackageService{
		Store:      db,
		Builds:     apppackage.BuildStore{Root: filepath.Join(storeDir(storePath), "app-packages")},
		ComputerID: strings.TrimSpace(computerID),
	}
	catalog := strings.TrimSpace(os.Getenv("CHOIR_APP_PACKAGE_CATALOG"))
	if catalog == "" {
		if updaterRoot := strings.TrimSpace(os.Getenv("CHOIR_UPDATER_ROOT")); updaterRoot != "" {
			catalog = filepath.Join(updaterRoot, "current", "app-packages")
		}
	}
	if catalog != "" {
		svc.Sources = append(svc.Sources, apppackage.BuildStore{Root: catalog})
	}
	return svc
}

// RegisterAppPackageRoutes mounts the service.
func RegisterAppPackageRoutes(s interface {
	Handle(string, http.Handler)
}, svc *AppPackageService) {
	if s == nil || svc == nil {
		return
	}
	s.Handle(appPackagePrefix, svc)
	s.Handle(appPackagePrefix+"/", svc)
}

type appPackageFrontend struct {
	Entry  string `json:"entry"`
	ABI    string `json:"abi"`
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
}

type appPackageRecord struct {
	Name           string                `json:"name"`
	Version        string                `json:"version"`
	Status         string                `json:"status"`
	Title          string                `json:"title"`
	Icon           string                `json:"icon,omitempty"`
	Description    string                `json:"description,omitempty"`
	ManifestDigest string                `json:"manifest_digest"`
	BuildDigest    string                `json:"build_digest"`
	DataStore      string                `json:"data_store"`
	Available      bool                  `json:"available"`
	Window         apppackage.WindowSpec `json:"window"`
	Frontend       appPackageFrontend    `json:"frontend"`
}

type appPackageList struct {
	Seq      uint64             `json:"seq"`
	Head     string             `json:"head,omitempty"`
	Packages []appPackageRecord `json:"packages"`
}

type appPackageCatalogEntry struct {
	Package     string `json:"package"`
	Version     string `json:"version"`
	BuildDigest string `json:"build_digest"`
	Recipe      string `json:"recipe"`
}

func (svc *AppPackageService) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	owner, err := server.AuthenticatedOwner(r)
	if err != nil {
		writeAppPackageError(w, http.StatusUnauthorized, err)
		return
	}
	rest := strings.TrimPrefix(r.URL.Path, appPackagePrefix)
	rest = strings.TrimPrefix(rest, "/")
	parts := strings.Split(rest, "/")
	switch {
	case rest == "":
		svc.requireMethod(w, r, http.MethodGet, svc.handleList)
	case rest == "catalog":
		svc.requireMethod(w, r, http.MethodGet, svc.handleCatalog)
	case rest == "install":
		svc.requireMethod(w, r, http.MethodPost, func(w http.ResponseWriter, r *http.Request) { svc.handleInstall(w, r, owner) })
	case parts[0] == "builds" && len(parts) >= 3:
		svc.requireMethod(w, r, http.MethodGet, func(w http.ResponseWriter, r *http.Request) {
			svc.handleBuildFile(w, r, parts[1], strings.Join(parts[2:], "/"))
		})
	case len(parts) == 2:
		op := apppackage.Op(parts[1])
		if op != apppackage.OpEnable && op != apppackage.OpDisable && op != apppackage.OpPurge {
			writeAppPackageError(w, http.StatusNotFound, fmt.Errorf("unknown app-package route"))
			return
		}
		svc.requireMethod(w, r, http.MethodPost, func(w http.ResponseWriter, r *http.Request) { svc.handleLifecycle(w, r, owner, parts[0], op) })
	case len(parts) == 3 && parts[1] == "data":
		switch r.Method {
		case http.MethodGet:
			svc.handleDataGet(w, r, parts[0], parts[2])
		case http.MethodPut:
			svc.handleDataPut(w, r, owner, parts[0], parts[2])
		default:
			writeAppPackageError(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
		}
	default:
		writeAppPackageError(w, http.StatusNotFound, fmt.Errorf("unknown app-package route"))
	}
}

func (svc *AppPackageService) requireMethod(w http.ResponseWriter, r *http.Request, method string, next http.HandlerFunc) {
	if r.Method != method {
		writeAppPackageError(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
		return
	}
	next(w, r)
}

func (svc *AppPackageService) handleList(w http.ResponseWriter, r *http.Request) {
	inv, err := svc.Store.AppPackageInventory(r.Context(), svc.ComputerID)
	if err != nil {
		writeAppPackageError(w, appPackageStatusFor(err), err)
		return
	}
	list := appPackageList{Seq: inv.Seq, Head: inv.Head, Packages: []appPackageRecord{}}
	for _, name := range inv.Names() {
		p := inv.Packages[name]
		entry, _ := p.Build.File(p.Manifest.Frontend.Entry)
		list.Packages = append(list.Packages, appPackageRecord{
			Name:           p.Name,
			Version:        p.Version,
			Status:         string(p.Status),
			Title:          p.Manifest.Display.Title,
			Icon:           p.Manifest.Display.Icon,
			Description:    p.Manifest.Display.Description,
			ManifestDigest: p.ManifestDigest,
			BuildDigest:    p.BuildDigest,
			DataStore:      p.Manifest.Data.Store,
			Available:      svc.Builds.Has(p.BuildDigest),
			Window:         p.Manifest.Frontend.Window,
			Frontend: appPackageFrontend{
				Entry:  p.Manifest.Frontend.Entry,
				ABI:    p.Manifest.Frontend.ABI,
				URL:    appPackageBuildURL(p.BuildDigest, p.Manifest.Frontend.Entry),
				SHA256: entry.SHA256,
			},
		})
	}
	writeAppPackageJSON(w, http.StatusOK, list)
}

func appPackageBuildURL(buildDigest, path string) string {
	return appPackagePrefix + "/builds/" + strings.TrimPrefix(buildDigest, "sha256:") + "/" + path
}

func (svc *AppPackageService) handleCatalog(w http.ResponseWriter, _ *http.Request) {
	entries := []appPackageCatalogEntry{}
	for _, source := range svc.Sources {
		lister, ok := source.(interface {
			List() ([]apppackage.Build, error)
		})
		if !ok {
			continue
		}
		builds, err := lister.List()
		if err != nil {
			writeAppPackageError(w, http.StatusInternalServerError, err)
			return
		}
		for _, b := range builds {
			digest, err := b.Digest()
			if err != nil {
				continue
			}
			entries = append(entries, appPackageCatalogEntry{Package: b.Package, Version: b.Version, BuildDigest: digest, Recipe: b.Recipe})
		}
	}
	writeAppPackageJSON(w, http.StatusOK, map[string]any{"builds": entries})
}

type appPackageInstallRequest struct {
	BuildDigest string `json:"build_digest"`
}

// handleInstall takes a build digest and nothing else (F28): the bytes come
// from the computer's own build store or a trusted source, are verified, and
// are stored before the install event is recorded, so the tape never names a
// build that is not on disk.
func (svc *AppPackageService) handleInstall(w http.ResponseWriter, r *http.Request, owner string) {
	var request appPackageInstallRequest
	if err := decodeAppPackageBody(r, &request); err != nil {
		writeAppPackageError(w, http.StatusBadRequest, err)
		return
	}
	if !apppackage.IsDigest(request.BuildDigest) {
		writeAppPackageError(w, http.StatusBadRequest, fmt.Errorf("build_digest must be sha256:<64 hex>"))
		return
	}
	build, files, err := svc.resolveBuild(request.BuildDigest)
	if err != nil {
		writeAppPackageError(w, appPackageStatusFor(err), err)
		return
	}
	// The receipt embeds the manifest it was built from (Build.Validate
	// checks it digests to manifest_digest); the reducer re-checks the
	// whole binding before the event is recorded.
	manifest := build.Manifest
	if _, err := svc.Builds.Put(build, files); err != nil {
		writeAppPackageError(w, appPackageStatusFor(err), err)
		return
	}
	inv, ev, err := svc.Store.RecordAppPackageEvent(r.Context(), owner, svc.ComputerID, apppackage.Event{
		Op: apppackage.OpInstall, Package: manifest.Name,
		Manifest: &manifest, Build: &build,
		ManifestDigest: build.ManifestDigest, BuildDigest: request.BuildDigest,
	})
	if err != nil {
		writeAppPackageError(w, appPackageStatusFor(err), err)
		return
	}
	log.Printf("autoputer: app package %s %s installed (build %s, seq %d)", manifest.Name, manifest.Version, request.BuildDigest, ev.Seq)
	writeAppPackageJSON(w, http.StatusOK, map[string]any{"seq": inv.Seq, "head": inv.Head, "event": ev.Op, "package": ev.Package})
}

func (svc *AppPackageService) resolveBuild(buildDigest string) (apppackage.Build, map[string][]byte, error) {
	if svc.Builds.Has(buildDigest) {
		return svc.Builds.Lookup(buildDigest)
	}
	for _, source := range svc.Sources {
		build, files, err := source.Lookup(buildDigest)
		if errors.Is(err, apppackage.ErrBuildNotFound) {
			continue
		}
		if err != nil {
			return apppackage.Build{}, nil, err
		}
		if got, err := build.Digest(); err != nil || got != buildDigest {
			return apppackage.Build{}, nil, fmt.Errorf("%w: source returned another build", apppackage.ErrDigestMismatch)
		}
		return build, files, nil
	}
	return apppackage.Build{}, nil, fmt.Errorf("%w: %s is not offered by any trusted source", apppackage.ErrBuildNotFound, buildDigest)
}

func (svc *AppPackageService) handleLifecycle(w http.ResponseWriter, r *http.Request, owner, name string, op apppackage.Op) {
	inv, ev, err := svc.Store.RecordAppPackageEvent(r.Context(), owner, svc.ComputerID, apppackage.Event{Op: op, Package: name})
	if err != nil {
		writeAppPackageError(w, appPackageStatusFor(err), err)
		return
	}
	log.Printf("autoputer: app package %s %s (seq %d)", name, op, ev.Seq)
	writeAppPackageJSON(w, http.StatusOK, map[string]any{"seq": inv.Seq, "head": inv.Head, "event": ev.Op, "package": ev.Package})
}

type appPackageDatumBody struct {
	Value json.RawMessage `json:"value"`
}

func (svc *AppPackageService) handleDataGet(w http.ResponseWriter, r *http.Request, name, key string) {
	datum, err := svc.Store.GetAppPackageDatum(r.Context(), svc.ComputerID, name, key)
	if err != nil {
		writeAppPackageError(w, appPackageStatusFor(err), err)
		return
	}
	writeAppPackageJSON(w, http.StatusOK, map[string]any{"key": datum.Key, "value": datum.Value, "schema_version": datum.SchemaVersion})
}

func (svc *AppPackageService) handleDataPut(w http.ResponseWriter, r *http.Request, owner, name, key string) {
	var body appPackageDatumBody
	if err := decodeAppPackageBody(r, &body); err != nil {
		writeAppPackageError(w, http.StatusBadRequest, err)
		return
	}
	if len(body.Value) == 0 {
		writeAppPackageError(w, http.StatusBadRequest, fmt.Errorf("value is required"))
		return
	}
	if err := svc.Store.PutAppPackageDatum(r.Context(), owner, svc.ComputerID, apppackage.Datum{Package: name, Key: key, Value: body.Value}); err != nil {
		writeAppPackageError(w, appPackageStatusFor(err), err)
		return
	}
	writeAppPackageJSON(w, http.StatusOK, map[string]any{"key": key})
}

// handleBuildFile serves a build file only when an enabled package on the
// tape binds that build and the path is in its receipt (F26, F27). The bytes
// are re-verified against the receipt on every read.
func (svc *AppPackageService) handleBuildFile(w http.ResponseWriter, r *http.Request, hex, path string) {
	buildDigest := "sha256:" + hex
	if !apppackage.IsDigest(buildDigest) {
		writeAppPackageError(w, http.StatusNotFound, fmt.Errorf("no such build"))
		return
	}
	inv, err := svc.Store.AppPackageInventory(r.Context(), svc.ComputerID)
	if err != nil {
		writeAppPackageError(w, appPackageStatusFor(err), err)
		return
	}
	bound := false
	for _, p := range inv.Packages {
		if p.Status == apppackage.StatusEnabled && p.BuildDigest == buildDigest {
			bound = true
			break
		}
	}
	if !bound {
		writeAppPackageError(w, http.StatusNotFound, fmt.Errorf("no enabled package binds this build"))
		return
	}
	data, record, err := svc.Builds.Open(buildDigest, path)
	if err != nil {
		status := appPackageStatusFor(err)
		if errors.Is(err, apppackage.ErrDigestMismatch) {
			log.Printf("autoputer: app package build %s file %s failed verification: %v", buildDigest, path, err)
		}
		writeAppPackageError(w, status, err)
		return
	}
	w.Header().Set("Content-Type", record.MediaType+appPackageCharset(record.MediaType))
	w.Header().Set("Cache-Control", "private, max-age=31536000, immutable")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Choir-Package-Digest", record.SHA256)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func appPackageCharset(mediaType string) string {
	if strings.HasPrefix(mediaType, "text/") || mediaType == "application/json" {
		return "; charset=utf-8"
	}
	return ""
}

func decodeAppPackageBody(r *http.Request, target any) error {
	raw, err := io.ReadAll(io.LimitReader(r.Body, apppackage.MaxDatumBytes+4096))
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("invalid request body: %w", err)
	}
	if decoder.More() {
		return fmt.Errorf("invalid request body: trailing data")
	}
	return nil
}

// appPackageStatusFor maps package errors onto HTTP statuses.
func appPackageStatusFor(err error) int {
	switch {
	case errors.Is(err, apppackage.ErrNotInstalled), errors.Is(err, apppackage.ErrBuildNotFound),
		errors.Is(err, apppackage.ErrBuildFileNotFound), errors.Is(err, store.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, apppackage.ErrAlreadyInstalled), errors.Is(err, apppackage.ErrInvalidTransition),
		errors.Is(err, apppackage.ErrNotEnabled), errors.Is(err, apppackage.ErrIncompatibleData):
		return http.StatusConflict
	case errors.Is(err, apppackage.ErrUnsupported), errors.Is(err, apppackage.ErrBindingMismatch),
		errors.Is(err, apppackage.ErrInvalidManifest), errors.Is(err, apppackage.ErrInvalidBuild):
		return http.StatusUnprocessableEntity
	case errors.Is(err, apppackage.ErrInvalidDatum), errors.Is(err, apppackage.ErrInvalidEvent):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

func writeAppPackageJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeAppPackageError(w http.ResponseWriter, status int, err error) {
	writeAppPackageJSON(w, status, map[string]string{"error": err.Error()})
}
