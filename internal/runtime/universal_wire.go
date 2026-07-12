package runtime

import (
	"context"
	"github.com/yusefmosiah/go-choir/internal/store"
	"regexp"
	"strings"
)

const universalWireEditionSourcePath = "universal-wire/Wire.texture"

var textureTransclusionRefRE = regexp.MustCompile(`texture:([A-Za-z0-9_.:-]{1,160})`)

func universalWireEditionIncludedDocIDs(content, editionDocID string) []string {
	seen := map[string]bool{}
	editionDocID = strings.TrimSpace(editionDocID)
	out := []string{}
	for _, match := range textureTransclusionRefRE.FindAllStringSubmatch(content, -1) {
		if len(match) < 2 {
			continue
		}
		docID := strings.TrimSpace(match[1])
		if docID == "" || docID == editionDocID || seen[docID] {
			continue
		}
		seen[docID] = true
		out = append(out, docID)
	}
	return out
}

func universalWirePlatformOwnerID() string {
	ownerID := strings.TrimSpace(getenvFirst("SOURCE_SERVICE_RUNTIME_OWNER_ID", "SOURCECYCLED_RUNTIME_OWNER_ID"))
	if ownerID == "" {
		ownerID = "universal-wire-platform"
	}
	return ownerID
}

// resolveUniversalWireTextureReadOwner returns the document owner to use for a
// read-only Texture API request. Authenticated users may read platform-owned
// Texture articles that are transcluded in the Universal Wire edition.
func (h *APIHandler) resolveUniversalWireTextureReadOwner(ctx context.Context, requesterOwnerID, docID string) (string, error) {
	if h == nil || h.rt == nil || h.rt.Store() == nil {
		return "", store.ErrNotFound
	}
	requesterOwnerID = strings.TrimSpace(requesterOwnerID)
	docID = strings.TrimSpace(docID)
	if requesterOwnerID == "" || docID == "" {
		return "", store.ErrNotFound
	}
	if _, err := h.rt.Store().GetDocument(ctx, docID, requesterOwnerID); err == nil {
		return requesterOwnerID, nil
	} else if err != store.ErrNotFound {
		return "", err
	}
	platformOwner := universalWirePlatformOwnerID()
	if _, err := h.rt.Store().GetDocument(ctx, docID, platformOwner); err != nil {
		return "", err
	}
	if !h.universalWireEditionIncludesDoc(ctx, docID) {
		return "", store.ErrNotFound
	}
	return platformOwner, nil
}

func (h *APIHandler) universalWireEditionIncludesDoc(ctx context.Context, docID string) bool {
	if h == nil || h.rt == nil || h.rt.Store() == nil {
		return false
	}
	platformOwner := universalWirePlatformOwnerID()
	editionDocID, err := h.rt.Store().GetDocumentAlias(ctx, platformOwner, universalWireEditionSourcePath)
	if err != nil {
		return false
	}
	editionDoc, err := h.rt.Store().GetDocument(ctx, editionDocID, platformOwner)
	if err != nil || strings.TrimSpace(editionDoc.CurrentRevisionID) == "" {
		return false
	}
	editionRev, err := h.rt.Store().GetRevision(ctx, editionDoc.CurrentRevisionID, platformOwner)
	if err != nil {
		return false
	}
	for _, included := range universalWireEditionIncludedDocIDs(editionRev.Content, editionDoc.DocID) {
		if included == docID {
			return true
		}
	}
	return false
}

func wirePlatformRoutePath(meta map[string]any) string {
	// Accept both the new "corpusd_route_path" key and the legacy
	// "platformd_route_path" key for backward compatibility with existing
	// published revisions in Dolt (renamed in PR 6 of store-consolidation).
	if route := metadataString(meta, "corpusd_route_path"); route != "" {
		return route
	}
	if route := metadataString(meta, "platformd_route_path"); route != "" {
		return route
	}
	if ref, ok := meta["corpusd_publication_ref"].(map[string]any); ok {
		return metadataString(ref, "route_path")
	}
	if ref, ok := meta["platformd_publication_ref"].(map[string]any); ok {
		return metadataString(ref, "route_path")
	}
	return ""
}
