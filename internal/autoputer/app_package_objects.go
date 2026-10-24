package autoputer

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/apppackage"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/server"
	"github.com/yusefmosiah/go-choir/internal/store"
)

// The owner's route to app package kinds (doctrine I27; P2b of
// docs/design/app-packages-replan-2026-10-24.md):
//
//	GET /api/app-packages/{package}/objects/{kind}         list the owner's objects
//	GET /api/app-packages/{package}/objects/{kind}/{id}    read one
//	PUT /api/app-packages/{package}/objects/{kind}/{id}    write one (the body is the object)
//
// {kind} is the short name; the route builds choir.app.{package}.{kind}, so no
// path can name a kind outside the package's namespace. The kind must have been
// registered through the package seam (a first-party manifest, at boot), the
// body must satisfy the kind's schema, and a write needs a kind that admits
// owner edits: the owner's own window is the only live writer until the
// package has a desk (P4).
//
// It is an owner act: the caller must be the host side of the guest boundary
// with an authenticated owner, and the route is not on management's ProductAPI
// allowlist (a positive list), so no desk can reach it.
//
// The whole route is behind the app_packages.objects computer setting, read
// live on every request and off by default. Off, every path answers 404 as if
// the route did not exist, and nothing is read or written. Turning it off is the
// rollback; objects already written stay on the tape as inert rows.

const (
	appPackageObjectsPrefix   = "/api/app-packages/"
	appPackageObjectMaxBody   = 256 << 10
	appPackageObjectListLimit = 200
)

var appPackageObjectID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

// appPackageObjectStore is the slice of the runtime store the route uses.
type appPackageObjectStore interface {
	GetComputerSetting(ctx context.Context, ownerID, computerID, key string) (store.ComputerSetting, error)
	PutPackageObject(ctx context.Context, kind objectgraph.ObjectKind, ownerID, id string, body json.RawMessage) (store.PackageObject, error)
	GetPackageObject(ctx context.Context, kind objectgraph.ObjectKind, ownerID, id string) (store.PackageObject, error)
	ListPackageObjects(ctx context.Context, kind objectgraph.ObjectKind, ownerID string, limit int) ([]store.PackageObject, error)
}

type appPackageObjects struct {
	store      appPackageObjectStore
	computerID string
}

// RegisterAppPackageObjectRoutes registers the first-party package kinds with
// the write gate (apppackage.RegisterFirstPartyAtBoot; it registers nothing
// until a manifest declares a kind) and the owner's object route. Only this
// route writes package kinds, so registering here, before the server starts
// serving, is as early as anything can write one.
func RegisterAppPackageObjectRoutes(s interface {
	HandleFunc(string, http.HandlerFunc)
}, st appPackageObjectStore, computerID string) {
	apppackage.RegisterFirstPartyAtBoot()
	h := &appPackageObjects{store: st, computerID: strings.TrimSpace(computerID)}
	s.HandleFunc(appPackageObjectsPrefix, h.serve)
}

func writePackageJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func packageNotFound(w http.ResponseWriter) {
	writePackageJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
}

func (h *appPackageObjects) enabled(ctx context.Context, ownerID string) (on bool, status int) {
	if h.store == nil || h.computerID == "" {
		return false, http.StatusNotFound
	}
	setting, err := h.store.GetComputerSetting(ctx, ownerID, h.computerID, store.SettingAppPackageObjects)
	switch {
	case err == nil:
		if setting.Value {
			return true, 0
		}
		return false, http.StatusNotFound
	case errors.Is(err, store.ErrNotFound):
		return false, http.StatusNotFound
	default:
		log.Printf("app packages: read %s on %s: %v; the route stays closed", store.SettingAppPackageObjects, h.computerID, err)
		return false, http.StatusServiceUnavailable
	}
}

func (h *appPackageObjects) serve(w http.ResponseWriter, r *http.Request) {
	ownerID, err := server.AuthenticatedOwner(r)
	if err != nil {
		writePackageJSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
		return
	}
	if on, status := h.enabled(r.Context(), ownerID); !on {
		if status == http.StatusNotFound {
			packageNotFound(w)
			return
		}
		writePackageJSON(w, status, map[string]string{"error": "app package objects are unavailable"})
		return
	}

	rest := strings.TrimPrefix(r.URL.Path, appPackageObjectsPrefix)
	parts := strings.Split(rest, "/")
	if len(parts) < 3 || len(parts) > 4 || parts[1] != "objects" {
		packageNotFound(w)
		return
	}
	pkg, short := parts[0], parts[2]
	id := ""
	if len(parts) == 4 {
		id = parts[3]
	}
	kind := objectgraph.ObjectKind(objectgraph.PackageKindNamespace(pkg) + short)
	if !objectgraph.ValidPackageName(pkg) || objectgraph.ValidPackageKindName(pkg, kind) != nil {
		packageNotFound(w)
		return
	}
	reg, ok := objectgraph.LookupPackageKind(kind)
	if !ok {
		packageNotFound(w)
		return
	}
	if len(parts) == 4 && !appPackageObjectID.MatchString(id) {
		writePackageJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid object id"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.read(w, r, kind, ownerID, id)
	case http.MethodPut:
		if id == "" {
			w.Header().Set("Allow", "GET")
			writePackageJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "PUT needs an object id"})
			return
		}
		h.write(w, r, kind, reg, ownerID, id)
	default:
		w.Header().Set("Allow", "GET, PUT")
		writePackageJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (h *appPackageObjects) read(w http.ResponseWriter, r *http.Request, kind objectgraph.ObjectKind, ownerID, id string) {
	if id != "" {
		obj, err := h.store.GetPackageObject(r.Context(), kind, ownerID, id)
		switch {
		case errors.Is(err, store.ErrNotFound):
			packageNotFound(w)
		case err != nil:
			writePackageJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		default:
			writePackageJSON(w, http.StatusOK, map[string]any{"object": obj})
		}
		return
	}
	limit := appPackageObjectListLimit
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > appPackageObjectListLimit {
			writePackageJSON(w, http.StatusBadRequest, map[string]string{"error": "limit must be 1 to " + strconv.Itoa(appPackageObjectListLimit)})
			return
		}
		limit = n
	}
	objs, err := h.store.ListPackageObjects(r.Context(), kind, ownerID, limit)
	if err != nil {
		writePackageJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writePackageJSON(w, http.StatusOK, map[string]any{"objects": objs})
}

func (h *appPackageObjects) write(w http.ResponseWriter, r *http.Request, kind objectgraph.ObjectKind, reg objectgraph.KindRegistration, ownerID, id string) {
	// The gate observes by default, so the route holds the owner-edit rule
	// itself: a kind that does not admit the owner is read-only here.
	if !reg.OwnerEdits {
		writePackageJSON(w, http.StatusForbidden, map[string]string{"error": "this kind is written by its package's desk, not by the owner"})
		return
	}
	schema, ok := apppackage.SchemaFor(kind)
	if !ok {
		// Registered without a schema: the seam and the schema table move
		// together, so this is a bug. Refuse rather than write unchecked.
		writePackageJSON(w, http.StatusInternalServerError, map[string]string{"error": "kind has no schema"})
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, appPackageObjectMaxBody))
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writePackageJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "body too large"})
			return
		}
		writePackageJSON(w, http.StatusBadRequest, map[string]string{"error": "unreadable body"})
		return
	}
	if err := schema.Validate(body); err != nil {
		status := http.StatusUnprocessableEntity
		if !json.Valid(body) {
			status = http.StatusBadRequest
		}
		writePackageJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	obj, err := h.store.PutPackageObject(r.Context(), kind, ownerID, id, json.RawMessage(body))
	switch {
	case errors.Is(err, objectgraph.ErrWrongWriter), errors.Is(err, objectgraph.ErrUndeclaredWriter), errors.Is(err, objectgraph.ErrUnregisteredKind):
		writePackageJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
	case err != nil:
		log.Printf("app packages: write %s/%s: %v", kind, id, err)
		writePackageJSON(w, http.StatusInternalServerError, map[string]string{"error": "write failed"})
	default:
		writePackageJSON(w, http.StatusOK, map[string]any{"object": obj})
	}
}
