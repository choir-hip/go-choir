package platform

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// The version read: a computer's Texture takes, merges or forks a published
// version by naming it (docs/design/autopaper-step0-contracts-2026-11-10.md,
// contract 1). The route answers one immutable choir.publication_version by
// its canonical id, with the body the take writes, so the reader's computer
// never trusts a local id. Like published search it admits a host-internal
// caller or any guest vmctl binds to a live computer: the versions are public.

// errVersionNotReadable is every reason a version is not served: unknown,
// not a version, tombstoned, its publication not published, or not public.
var errVersionNotReadable = errors.New("publication version is not available")

// PublicationVersionRead is a published version as a computer takes it. The
// owner comes from the graph object, never from the request.
type PublicationVersionRead struct {
	PublicationVersionID string          `json:"publication_version_id"`
	PublicationID        string          `json:"publication_id"`
	OwnerID              string          `json:"owner_id"`
	AuthorLabel          string          `json:"author_label,omitempty"`
	Title                string          `json:"title"`
	RoutePath            string          `json:"route_path"`
	Revision             int             `json:"revision"`
	ManifestHash         string          `json:"manifest_hash"`
	Content              string          `json:"content"`
	BodyDoc              json.RawMessage `json:"body_doc,omitempty"`
	SourceEntities       json.RawMessage `json:"source_entities,omitempty"`
}

// ReadPublicationVersion serves one published version by its version id.
func (s *Service) ReadPublicationVersion(ctx context.Context, canonicalID string) (*PublicationVersionRead, error) {
	og := s.ogStore()
	if og == nil {
		return nil, fmt.Errorf("platform service: object graph store unavailable")
	}
	// The id a publish returns and a link carries is the version id in the
	// object's metadata; the graph's canonical id is accepted too.
	var versionObj objectgraph.Object
	found, err := og.ListObjectsByMetadata(ctx, "choir.publication_version", "$.publication_version_id", canonicalID, 1)
	if err != nil {
		return nil, err
	}
	if len(found) > 0 {
		versionObj = found[0]
	} else if versionObj, err = og.GetObject(ctx, canonicalID); err != nil {
		if errors.Is(err, objectgraph.ErrNotFound) {
			return nil, errVersionNotReadable
		}
		return nil, err
	}
	if string(versionObj.ObjectKind) != "choir.publication_version" || versionObj.Tombstone {
		return nil, errVersionNotReadable
	}
	meta, err := decodeVersionMetadata(versionObj)
	if err != nil {
		return nil, err
	}
	identity, err := publicationIdentityFor(versionObj.OwnerID, meta.SourceDocID)
	if err != nil {
		return nil, err
	}
	pubObj, err := og.GetObject(ctx, identity.CanonicalID)
	if err != nil {
		if errors.Is(err, objectgraph.ErrNotFound) {
			return nil, errVersionNotReadable
		}
		return nil, err
	}
	var pub struct {
		Title       string `json:"title"`
		State       string `json:"state"`
		AuthorLabel string `json:"author_label"`
	}
	if pubObj.Tombstone || json.Unmarshal(pubObj.Metadata, &pub) != nil || pub.State != "published" {
		return nil, errVersionNotReadable
	}
	versions, err := publishedVersions(ctx, og, pubObj.CanonicalID)
	if err != nil {
		return nil, err
	}
	if ref, ok := findPublishedVersion(versions, meta.Revision); !ok || ref.CanonicalID != versionObj.CanonicalID {
		return nil, errVersionNotReadable
	}
	if policy := s.graphPublicationPolicy(ctx, og, versionObj.CanonicalID); !publicationAccessPolicyAllowsPublicRoute(policy.Access) {
		return nil, errVersionNotReadable
	}
	manifestEdges, err := og.ListEdgesByKind(ctx, versionObj.CanonicalID, "has_manifest")
	if err != nil || len(manifestEdges) == 0 {
		return nil, fmt.Errorf("publication version %s: no manifest", canonicalID)
	}
	manifestObj, err := og.GetObject(ctx, manifestEdges[0].ToID)
	if err != nil {
		return nil, err
	}
	if sha256Hex(manifestObj.Body) != meta.ManifestHash {
		return nil, fmt.Errorf("publication version %s: manifest does not hash to %s", canonicalID, meta.ManifestHash)
	}
	blobEdges, err := og.ListEdgesByKind(ctx, manifestObj.CanonicalID, "contains_blob")
	if err != nil || len(blobEdges) == 0 {
		return nil, fmt.Errorf("publication version %s: no blob", canonicalID)
	}
	blobObj, err := og.GetObject(ctx, blobEdges[0].ToID)
	if err != nil {
		return nil, err
	}
	var blobMeta struct {
		StorageRef string `json:"storage_ref"`
	}
	if err := json.Unmarshal(blobObj.Metadata, &blobMeta); err != nil {
		return nil, err
	}
	content, err := s.readBlob(blobMeta.StorageRef)
	if err != nil {
		return nil, err
	}
	bodyDoc, entities := structuredArtifactFieldsFromManifest(string(manifestObj.Body))
	return &PublicationVersionRead{
		PublicationVersionID: meta.PublicationVersionID, PublicationID: meta.PublicationID,
		OwnerID: versionObj.OwnerID, AuthorLabel: pub.AuthorLabel, Title: pub.Title,
		RoutePath: firstNonEmptyString(meta.PermalinkPath, meta.RoutePath), Revision: meta.Revision,
		ManifestHash: meta.ManifestHash, Content: string(content), BodyDoc: bodyDoc, SourceEntities: entities,
	}, nil
}

func firstNonEmptyString(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// HandleInternalPublicationVersion serves GET
// /internal/platform/publications/versions/{version_id}.
func (h *Handler) HandleInternalPublicationVersion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if !trustedInternalCaller(r) {
		if _, err := bindTapCaller(r.Context(), h.guestLookup, r); err != nil {
			writeJSON(w, http.StatusForbidden, apiError{Error: "internal caller required"})
			return
		}
	}
	id := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/internal/platform/publications/versions/"))
	if id == "" || strings.Contains(id, "/") {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "publication version id is required"})
		return
	}
	out, err := h.service.ReadPublicationVersion(r.Context(), id)
	if err != nil {
		if errors.Is(err, errVersionNotReadable) {
			writeJSON(w, http.StatusNotFound, apiError{Error: err.Error()})
			return
		}
		log.Printf("corpusd: read publication version %s: %v", id, err)
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "failed to read publication version"})
		return
	}
	writeJSON(w, http.StatusOK, out)
}
