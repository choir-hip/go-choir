package platform

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// ErrPublicationNotFound marks an owner's doc with no publication.
var ErrPublicationNotFound = errors.New("publication not found")

// OwnedPublication is the owner's view of one doc's publication: whatever
// its mode, and whether or not the front list shows it. OnFront says
// whether it does.
type OwnedPublication struct {
	PublicationID  string     `json:"publication_id"`
	DocID          string     `json:"doc_id"`
	Title          string     `json:"title"`
	State          string     `json:"state"`
	PublishMode    string     `json:"publish_mode"`
	VersionCount   int        `json:"version_count"`
	LatestRevision int        `json:"latest_revision"`
	Route          FrontRoute `json:"route"`
	OnFront        bool       `json:"on_front"`
}

// OwnedPublication reads ownerID's publication of docID. The publication's
// identity derives from (owner, doc), so another owner's publication of the
// same doc id is never found.
func (s *Service) OwnedPublication(ctx context.Context, ownerID, docID string) (*OwnedPublication, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("platform service unavailable")
	}
	ownerID, docID = strings.TrimSpace(ownerID), strings.TrimSpace(docID)
	if ownerID == "" || docID == "" {
		return nil, fmt.Errorf("owner_id and doc_id are required")
	}
	og := s.ogStore()
	if og == nil {
		return nil, fmt.Errorf("platform service: object graph store unavailable")
	}
	ident, err := publicationIdentityFor(ownerID, docID)
	if err != nil {
		return nil, fmt.Errorf("owned publication: publication id: %w", err)
	}
	obj, err := og.GetObject(ctx, ident.CanonicalID)
	if errors.Is(err, objectgraph.ErrNotFound) {
		return nil, ErrPublicationNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("owned publication: read publication: %w", err)
	}
	var meta frontPublicationMeta
	if err := json.Unmarshal(obj.Metadata, &meta); err != nil {
		return nil, fmt.Errorf("owned publication: decode %s: %w", ident.CanonicalID, err)
	}
	versions, err := publishedVersions(ctx, og, ident.CanonicalID)
	if err != nil {
		return nil, fmt.Errorf("owned publication: list versions: %w", err)
	}
	out := &OwnedPublication{
		PublicationID: firstNonEmpty(meta.PublicationID, ident.PublicationID),
		DocID:         docID,
		Title:         strings.TrimSpace(meta.Title),
		State:         meta.State,
		PublishMode:   meta.PublishMode,
		VersionCount:  len(versions),
	}
	bare := normalizePublicationRoutePath(meta.RoutePath)
	out.Route.Bare = bare
	if len(versions) > 0 {
		out.LatestRevision = versions[len(versions)-1].Revision
		out.Route.Permalink = versionPermalink(bare, out.LatestRevision)
	}
	_, out.OnFront, err = s.frontStory(ctx, og, frontPublicationRow{CanonicalID: ident.CanonicalID, Metadata: obj.Metadata, UpdatedAt: obj.UpdatedAt})
	if err != nil {
		return nil, err
	}
	return out, nil
}
