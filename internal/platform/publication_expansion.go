package platform

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
)

// ExpansionSource names the paper document a public Autopaper story is
// published from. The public bundle hides it on purpose; this internal read
// exists so the proxy can carry a signed-in owner's expansion request to
// that document's Texture desk (docs/reports/story-expansion-request-design-
// 2026-10-20.md, section 4.1). The proxy builds its call to the paper from
// this answer alone, never from the request body.
type ExpansionSource struct {
	PublicationID string `json:"publication_id"`
	// OwnerID is the publication's owner: the paper's dedicated account.
	OwnerID     string `json:"owner_id"`
	SourceDocID string `json:"source_doc_id"`
	// Route is the story's bare route.
	Route string `json:"route"`
	// LatestRevision is the newest public version's number and
	// SourceRevisionID the document revision it was cut from.
	LatestRevision   int    `json:"latest_revision"`
	SourceRevisionID string `json:"source_revision_id"`
}

// ExpansionSource resolves a public route (bare or /v<n>) to the latest
// version's source document. Anything that is not a public Autopaper story
// with a published version (an unknown route, a hand-published Texture, an
// owner off the Autopaper allowlist, a held or unlisted story) is one error,
// ErrPublicationNotFound, so a caller cannot probe which routes exist.
func (s *Service) ExpansionSource(ctx context.Context, routePath string) (*ExpansionSource, error) {
	bare, _, ok := parsePublicationRoute(normalizePublicationRoutePath(routePath))
	if !ok {
		return nil, ErrPublicationNotFound
	}
	head, err := s.resolvePublicationHead(ctx, bare, newResolveSteps())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrPublicationNotFound
		}
		return nil, err
	}
	og := head.og
	pubObj, err := og.GetObject(ctx, head.routeMeta.TargetID)
	if err != nil {
		return nil, fmt.Errorf("expansion source: get publication: %w", err)
	}
	var meta frontPublicationMeta
	if err := json.Unmarshal(pubObj.Metadata, &meta); err != nil {
		return nil, fmt.Errorf("expansion source: decode publication %s: %w", pubObj.CanonicalID, err)
	}
	// The front list's own test: a published, public, publish-auto story of
	// an allowed owner. A request may only reach what a reader can see.
	if _, listed, err := s.frontStory(ctx, og, frontPublicationRow{CanonicalID: pubObj.CanonicalID, Metadata: pubObj.Metadata, UpdatedAt: pubObj.UpdatedAt.UTC()}); err != nil {
		return nil, err
	} else if !listed {
		return nil, ErrPublicationNotFound
	}
	latest := head.versions[len(head.versions)-1]
	versionObj, err := og.GetObject(ctx, latest.CanonicalID)
	if err != nil {
		return nil, fmt.Errorf("expansion source: get v%d: %w", latest.Revision, err)
	}
	versionMeta, err := decodeVersionMetadata(versionObj)
	if err != nil {
		return nil, fmt.Errorf("expansion source: %w", err)
	}
	ownerID := strings.TrimSpace(meta.OwnerID)
	docID := strings.TrimSpace(versionMeta.SourceDocID)
	if ownerID == "" || docID == "" || strings.TrimSpace(versionMeta.SourceRevisionID) == "" {
		// A version that does not name its source cannot be expanded.
		return nil, ErrPublicationNotFound
	}
	return &ExpansionSource{
		PublicationID:    firstNonEmpty(meta.PublicationID, versionMeta.PublicationID),
		OwnerID:          ownerID,
		SourceDocID:      docID,
		Route:            bare,
		LatestRevision:   latest.Revision,
		SourceRevisionID: strings.TrimSpace(versionMeta.SourceRevisionID),
	}, nil
}

// HandleInternalExpansionSource handles GET
// /internal/platform/publications/expansion-source?route=<bare or /v<n>>.
// Internal callers only: the answer carries a source document id the public
// bundle withholds. Every route that is not an expandable story answers the
// same 404.
func (h *Handler) HandleInternalExpansionSource(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if !trustedInternalCaller(r) {
		writeJSON(w, http.StatusForbidden, apiError{Error: "internal caller required"})
		return
	}
	routePath := strings.TrimSpace(r.URL.Query().Get("route"))
	if routePath == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "route is required"})
		return
	}
	out, err := h.service.ExpansionSource(r.Context(), routePath)
	if errors.Is(err, ErrPublicationNotFound) {
		writeJSON(w, http.StatusNotFound, apiError{Error: "story not found"})
		return
	}
	if err != nil {
		log.Printf("corpusd: expansion source %s: %v", routePath, err)
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "failed to read story source"})
		return
	}
	writeJSON(w, http.StatusOK, out)
}
