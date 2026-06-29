package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/base/blob"
	"github.com/yusefmosiah/go-choir/internal/base/journal"
	"github.com/yusefmosiah/go-choir/internal/base/model"
)

func (h *Handler) validatePutItemRequest(req putItemRequest) (int, string) {
	if !req.ItemID.Valid() {
		return http.StatusBadRequest, "invalid item_id"
	}
	if !req.EventType.Valid() {
		return http.StatusBadRequest, "invalid event_type"
	}
	if !req.Kind.Valid() {
		return http.StatusBadRequest, "invalid kind"
	}
	if req.ParentItemID != "" && !req.ParentItemID.Valid() {
		return http.StatusBadRequest, "invalid parent_item_id"
	}
	if !req.VersionID.Valid() {
		return http.StatusBadRequest, "invalid version_id"
	}
	if !req.BlobRef.Valid() {
		return http.StatusBadRequest, "invalid blob_ref"
	}
	if req.Kind == model.KindFolder {
		if req.BlobRef != "" {
			return http.StatusBadRequest, "folder item cannot reference a blob"
		}
		if req.ContentHash != "" {
			return http.StatusBadRequest, "folder item cannot carry a content_hash"
		}
		return 0, ""
	}
	if !eventRequiresVersion(req.EventType) {
		return 0, ""
	}
	if req.VersionID == "" {
		return http.StatusBadRequest, "file item requires version_id"
	}
	if req.BlobRef == "" {
		return http.StatusBadRequest, "file item requires blob_ref"
	}
	if h.blobs == nil {
		return http.StatusInternalServerError, "blob store not configured"
	}
	stat, err := h.blobs.Stat(req.BlobRef)
	if err != nil {
		if errors.Is(err, blob.ErrNotFound) {
			return http.StatusBadRequest, "blob_ref not found"
		}
		return http.StatusInternalServerError, "stat blob: " + err.Error()
	}
	hexDigest := strings.TrimPrefix(string(req.BlobRef), "sha256:")
	if stat.SHA256 != "" && stat.SHA256 != hexDigest {
		return http.StatusInternalServerError, "blob metadata mismatch"
	}
	if req.ContentHash != "" && req.ContentHash != hexDigest {
		return http.StatusBadRequest, "content_hash does not match blob_ref"
	}
	return 0, ""
}

func eventRequiresVersion(eventType model.EventType) bool {
	switch eventType {
	case model.EventCreate, model.EventUpdate, model.EventBlobUpload:
		return true
	case model.EventDelete, model.EventMove:
		return false
	default:
		return false
	}
}

func entriesForOwner(entries []journal.Entry, ownerID string) []journal.Entry {
	out := make([]journal.Entry, 0, len(entries))
	for _, entry := range entries {
		if entry.Event.OwnerID == ownerID {
			out = append(out, entry)
		}
	}
	return out
}
