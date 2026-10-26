package platform

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/release"
)

// SoftwareReleaseKind is the Store B object a published release record is
// (P7, docs/reports/p6-p8-release-adopt-2026-10-15.md). Its body is the
// record's canonical JSON; its id names the publisher's owner and the
// release id (the record's sha256).
const SoftwareReleaseKind objectgraph.ObjectKind = "choir.software_release"

const releasesRoute = "/internal/platform/releases"

// ReleaseReadsMode is CHOIR_RELEASE_READS (Library S1,
// docs/design/library-package-propagation-2026-10-26.md 4.3): open serves a
// published release to any reader, as before; owner serves a release only to
// the owner who published it. It is read once at startup (SetReleaseReads).
type ReleaseReadsMode string

const (
	ReleaseReadsOpen  ReleaseReadsMode = "open"
	ReleaseReadsOwner ReleaseReadsMode = "owner"

	// EnvReleaseReads names the switch.
	EnvReleaseReads = "CHOIR_RELEASE_READS"
)

// ParseReleaseReads reads the switch value. Unset or "open" is open. Any
// other value is owner: a typo narrows reads, it never opens them.
func ParseReleaseReads(raw string) ReleaseReadsMode {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", string(ReleaseReadsOpen):
		return ReleaseReadsOpen
	default:
		return ReleaseReadsOwner
	}
}

// SetReleaseReads sets the mode once at startup, before the handler serves.
func (h *Handler) SetReleaseReads(mode ReleaseReadsMode) {
	if h != nil {
		h.releaseReads = mode
	}
}

// releaseStore is the slice of the Store B object graph the release routes
// use; *ObjectGraphStore is it.
type releaseStore interface {
	InsertObjectIfAbsent(ctx context.Context, obj objectgraph.Object) (bool, error)
	GetObjectByMetadata(ctx context.Context, kind, jsonPath, value string) (objectgraph.Object, error)
	ListObjects(ctx context.Context, filter objectgraph.ListFilter) ([]objectgraph.Object, error)
}

func (h *Handler) releaseStore() releaseStore {
	if h == nil || h.service == nil {
		return nil
	}
	if og := h.service.ogStore(); og != nil {
		return og
	}
	return nil
}

// releasePublishRequest is the host's publish call. The proxy sends it after
// the owner's act: OwnerID and ComputerID are the owner and the computer the
// proxy authorized, and must be the record's publisher (failure mode R4).
type releasePublishRequest struct {
	ReleaseID  string          `json:"release_id"`
	Record     json.RawMessage `json:"record"`
	OwnerID    string          `json:"owner_id"`
	ComputerID string          `json:"computer_id"`
}

// ReleaseView is one published release as the routes serve it.
type ReleaseView struct {
	ReleaseID   string          `json:"release_id"`
	CanonicalID string          `json:"canonical_id"`
	PublishedAt string          `json:"published_at"`
	Published   bool            `json:"published,omitempty"`
	Record      json.RawMessage `json:"record"`
}

// HandleReleases serves
//
//	POST /internal/platform/releases                  publish        host only
//	GET  /internal/platform/releases?limit=&publisher_computer_id=   host only
//	GET  /internal/platform/releases/<id>?computer_id= host, or a guest with its own event:read
//	PUT  /internal/platform/releases/blobs/<sha>      upload patch   host only
//	GET  /internal/platform/releases/blobs/<sha>?computer_id= host, or a guest with its own event:read
//
// A guest never writes here: publication is the owner's act through the
// proxy (failure mode R3). Any bound guest may read any published release:
// a publication is readable on this cloud.
func (h *Handler) HandleReleases(w http.ResponseWriter, r *http.Request) {
	rest := strings.Trim(strings.TrimPrefix(r.URL.Path, releasesRoute), "/")
	switch {
	case rest == "" && r.Method == http.MethodPost:
		if !trustedInternalCaller(r) {
			writeJSON(w, http.StatusForbidden, apiError{Error: "release publish is the owner's act through the proxy; no guest publishes"})
			return
		}
		h.publishRelease(w, r)
	case rest == "" && r.Method == http.MethodGet:
		if !trustedInternalCaller(r) {
			writeJSON(w, http.StatusForbidden, apiError{Error: errInternalCallerRequired.Error()})
			return
		}
		h.listReleases(w, r)
	case strings.HasPrefix(rest, "blobs/"):
		h.handleReleaseBlob(w, r, strings.TrimPrefix(rest, "blobs/"))
	case computerevent.IsSHA256(rest) && r.Method == http.MethodGet:
		if !h.authorizeFileCAS(r, strings.TrimSpace(r.URL.Query().Get("computer_id")), "event:read") {
			writeJSON(w, http.StatusForbidden, apiError{Error: "computer capability required"})
			return
		}
		view, err := h.readRelease(r.Context(), rest)
		switch {
		case errors.Is(err, objectgraph.ErrNotFound):
			writeJSON(w, http.StatusNotFound, apiError{Error: "release not published"})
		case err != nil:
			writeJSON(w, http.StatusInternalServerError, apiError{Error: "release unreadable"})
		default:
			writeJSON(w, http.StatusOK, view)
		}
	default:
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
	}
}

func (h *Handler) handleReleaseBlob(w http.ResponseWriter, r *http.Request, digest string) {
	if !computerevent.IsSHA256(digest) {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "release blob: sha256 digest required"})
		return
	}
	store := h.contentStore(contentNamespaceRelease)
	switch r.Method {
	case http.MethodPut:
		if !trustedInternalCaller(r) {
			writeJSON(w, http.StatusForbidden, apiError{Error: "release blob upload is not publicly accessible"})
			return
		}
		// The patch cap holds at upload too, so an oversize upload never
		// lands (R9); the store streams and hashes, never buffering.
		store.maxBytes = release.MaxPatchBytes
		switch err := store.Put(digest, r.Body); {
		case err == nil:
			writeJSON(w, http.StatusCreated, map[string]string{"digest": digest})
		case errors.Is(err, errContentDigestMismatch):
			writeJSON(w, http.StatusBadRequest, apiError{Error: "release blob: digest mismatch"})
		case errors.Is(err, errContentTooLarge):
			writeJSON(w, http.StatusRequestEntityTooLarge, apiError{Error: "release blob: exceeds the 2 MiB patch cap"})
		default:
			writeJSON(w, http.StatusInternalServerError, apiError{Error: "release blob: write"})
		}
	case http.MethodGet:
		if !h.authorizeFileCAS(r, strings.TrimSpace(r.URL.Query().Get("computer_id")), "event:read") {
			writeJSON(w, http.StatusForbidden, apiError{Error: "computer capability required"})
			return
		}
		raw, err := store.ReadVerified(digest, release.MaxPatchBytes)
		switch {
		case errors.Is(err, errContentNotFound):
			writeJSON(w, http.StatusNotFound, apiError{Error: "release blob unavailable"})
		case errors.Is(err, errContentDigestMismatch), errors.Is(err, errContentTooLarge):
			writeJSON(w, http.StatusConflict, apiError{Error: "release blob does not hash to its name"})
		case err != nil:
			writeJSON(w, http.StatusInternalServerError, apiError{Error: "release blob unreadable"})
		default:
			w.Header().Set("Content-Type", "application/octet-stream")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(raw)
		}
	default:
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
	}
}

func (h *Handler) publishRelease(w http.ResponseWriter, r *http.Request) {
	store := h.releaseStore()
	if store == nil {
		writeJSON(w, http.StatusServiceUnavailable, apiError{Error: "release store unavailable"})
		return
	}
	var request releasePublishRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, release.MaxRecordBytes+(64<<10)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "invalid release publish request"})
		return
	}
	record, err := release.Decode(request.Record)
	if err == nil {
		err = release.VerifyID(record, request.ReleaseID)
	}
	if err != nil {
		// R1: the id is the record's digest, or nothing is stored.
		writeJSON(w, http.StatusUnprocessableEntity, apiError{Error: err.Error()})
		return
	}
	releaseID := strings.ToLower(strings.TrimSpace(request.ReleaseID))
	if strings.TrimSpace(request.OwnerID) == "" || strings.TrimSpace(request.ComputerID) == "" ||
		record.Publisher.OwnerID != strings.TrimSpace(request.OwnerID) || record.Publisher.ComputerID != strings.TrimSpace(request.ComputerID) {
		writeJSON(w, http.StatusForbidden, apiError{Error: "the record's publisher is not the owner and computer that published it"})
		return
	}
	// R2/A3: the patch is in the store and its bytes hash to the record.
	patch, err := h.contentStore(contentNamespaceRelease).ReadVerified(record.Source.PatchSHA256, release.MaxPatchBytes)
	if err == nil {
		err = release.VerifyPatch(record, patch)
	}
	if err != nil {
		writeJSON(w, http.StatusConflict, apiError{Error: "release patch is not in the host content store as the record names it: " + err.Error()})
		return
	}
	body, err := record.Canonical()
	if err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, apiError{Error: err.Error()})
		return
	}
	canonicalID, err := objectgraph.BuildCanonicalID(SoftwareReleaseKind, record.Publisher.OwnerID, releaseID)
	if err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, apiError{Error: err.Error()})
		return
	}
	now := time.Now().UTC()
	metadata, err := objectgraph.NormalizeMetadata(map[string]any{
		"release_id":            releaseID,
		"publisher_computer_id": record.Publisher.ComputerID,
		"operation_id":          record.Operation.OperationID,
		"bundle_digest":         record.Operation.BundleDigest,
		"base_commit":           record.Source.BaseCommit,
		"patch_sha256":          record.Source.PatchSHA256,
		"tree_hash":             record.Source.TreeHash,
		"published_at":          now.Format(time.RFC3339Nano),
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "release metadata"})
		return
	}
	obj := objectgraph.Object{
		CanonicalID: canonicalID, ObjectKind: SoftwareReleaseKind,
		OwnerID: record.Publisher.OwnerID, ComputerID: record.Publisher.ComputerID,
		VersionID: releaseID, Body: body, Metadata: metadata,
		ContentHash: objectgraph.ContentHash(SoftwareReleaseKind, body, metadata),
		CreatedAt:   now, UpdatedAt: now,
	}
	// R8: insert-if-absent; a republish names the first object and never
	// replaces it.
	inserted, err := store.InsertObjectIfAbsent(r.Context(), obj)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "release publish: " + err.Error()})
		return
	}
	view, err := h.readRelease(r.Context(), releaseID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "release publish: read back"})
		return
	}
	status := http.StatusOK
	if inserted {
		status, view.Published = http.StatusCreated, true
	}
	writeJSON(w, status, view)
}

// readRelease finds a release by its id and re-verifies the stored record
// against it: the store is not trusted by name either (A3).
func (h *Handler) readRelease(ctx context.Context, releaseID string) (ReleaseView, error) {
	store := h.releaseStore()
	if store == nil {
		return ReleaseView{}, errors.New("release store unavailable")
	}
	obj, err := store.GetObjectByMetadata(ctx, string(SoftwareReleaseKind), "$.release_id", releaseID)
	if err != nil {
		return ReleaseView{}, err
	}
	return releaseViewFromObject(obj)
}

func releaseViewFromObject(obj objectgraph.Object) (ReleaseView, error) {
	if obj.ObjectKind != SoftwareReleaseKind || obj.Tombstone {
		return ReleaseView{}, objectgraph.ErrNotFound
	}
	var metadata struct {
		ReleaseID   string `json:"release_id"`
		PublishedAt string `json:"published_at"`
	}
	if err := json.Unmarshal(obj.Metadata, &metadata); err != nil {
		return ReleaseView{}, err
	}
	record, err := release.Decode(obj.Body)
	if err != nil {
		return ReleaseView{}, err
	}
	if err := release.VerifyID(record, metadata.ReleaseID); err != nil {
		return ReleaseView{}, err
	}
	return ReleaseView{ReleaseID: metadata.ReleaseID, CanonicalID: obj.CanonicalID, PublishedAt: metadata.PublishedAt, Record: json.RawMessage(obj.Body)}, nil
}

func (h *Handler) listReleases(w http.ResponseWriter, r *http.Request) {
	store := h.releaseStore()
	if store == nil {
		writeJSON(w, http.StatusServiceUnavailable, apiError{Error: "release store unavailable"})
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	objects, err := store.ListObjects(r.Context(), objectgraph.ListFilter{
		Kind: SoftwareReleaseKind, ComputerID: strings.TrimSpace(r.URL.Query().Get("publisher_computer_id")), Limit: limit,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "release list unavailable"})
		return
	}
	views := make([]ReleaseView, 0, len(objects))
	for _, obj := range objects {
		view, err := releaseViewFromObject(obj)
		if err != nil {
			continue // a record that no longer verifies is not served
		}
		views = append(views, view)
	}
	writeJSON(w, http.StatusOK, map[string]any{"releases": views, "count": len(views)})
}
