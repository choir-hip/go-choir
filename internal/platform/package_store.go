package platform

import (
	"context"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// SoftwareReleaseKind is the retired P7 software release kind: its records
// stay readable in Store B, and nothing writes one (its routes are deleted,
// docs/design/package-install-2026-11-04.md).
const SoftwareReleaseKind objectgraph.ObjectKind = "choir.software_release"

// releaseStore is the object graph the package routes read and write.
type releaseStore interface {
	InsertObjectIfAbsent(ctx context.Context, obj objectgraph.Object) (bool, error)
	GetObjectByMetadata(ctx context.Context, kind, jsonPath, value string) (objectgraph.Object, error)
	ListObjects(ctx context.Context, filter objectgraph.ListFilter) ([]objectgraph.Object, error)
	ListObjectsByMetadata(ctx context.Context, kind, jsonPath, value string, limit int) ([]objectgraph.Object, error)
	ListObjectsByMetadataForOwner(ctx context.Context, kind, jsonPath, value, ownerID string, limit int) ([]objectgraph.Object, error)
	ListObjectsByMetadataPage(ctx context.Context, kind, jsonPath, value, ownerID, afterCanonicalID string, limit int) ([]objectgraph.Object, error)
	ListKeyedObjectsPage(ctx context.Context, kind, jsonPath, value, afterCanonicalID string, limit int) ([]objectgraph.Object, string, error)
	LookupKeysLive() bool
	GetObject(ctx context.Context, id string) (objectgraph.Object, error)
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
