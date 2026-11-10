package textureowner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/texturedoc"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// PublicationVersion is a published version as the revise call reads it from
// the platform: the immutable body a take writes and the pin a link records.
type PublicationVersion struct {
	PublicationVersionID string          `json:"publication_version_id"`
	OwnerID              string          `json:"owner_id"`
	AuthorLabel          string          `json:"author_label,omitempty"`
	Title                string          `json:"title"`
	RoutePath            string          `json:"route_path"`
	ManifestHash         string          `json:"manifest_hash"`
	Content              string          `json:"content"`
	BodyDoc              json.RawMessage `json:"body_doc,omitempty"`
	SourceEntities       json.RawMessage `json:"source_entities,omitempty"`
}

// PublicationVersionReader reads one published version through the platform.
// ErrSourceNotReadable is the platform saying the version is unknown,
// tombstoned or unpublished; any other error is the platform being unreachable.
type PublicationVersionReader interface {
	ReadPublicationVersion(ctx context.Context, versionID string) (PublicationVersion, error)
}

// ErrSourceNotReadable is a source the platform refuses to serve.
var ErrSourceNotReadable = errors.New("publication version is not readable")

// corpusdVersionReader is the real reader: the bound-guest route
// GET /internal/platform/publications/versions/{version_id}.
type corpusdVersionReader struct {
	baseURL string
	client  *http.Client
}

func (c corpusdVersionReader) ReadPublicationVersion(ctx context.Context, versionID string) (PublicationVersion, error) {
	var out PublicationVersion
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/internal/platform/publications/versions/"+url.PathEscape(versionID), nil)
	if err != nil {
		return out, err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return out, err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return out, err
	}
	switch {
	case resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone:
		return out, ErrSourceNotReadable
	case resp.StatusCode != http.StatusOK:
		return out, fmt.Errorf("platform answered %d", resp.StatusCode)
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return out, fmt.Errorf("decode publication version: %w", err)
	}
	return out, nil
}

// versionReader is the handler's reader: the one set for tests, else the
// platform's.
func (h *Handler) versionReader() PublicationVersionReader {
	var platform PublicationVersionReader
	if h.PublicationVersions != nil {
		platform = h.PublicationVersions
	} else if h.Core != nil {
		if base := strings.TrimRight(h.Core.CorpusdURL(), "/"); base != "" {
			platform = corpusdVersionReader{baseURL: base, client: &http.Client{Timeout: 20 * time.Second}}
		}
	}
	// A local algorithm2 proposal is served beside the platform's versions
	// (algorithm2_owner.go): the one seam where "a source must be a published
	// version" (contract 1, decision D9) is relaxed, and only for a call the
	// owner's acceptance of that proposal makes.
	if strings.TrimSpace(h.Algorithm2Dir) != "" {
		return proposalVersionReader{h: h, platform: platform}
	}
	return platform
}

// resolvedSource is one source after validation: its version and its link.
type resolvedSource struct {
	Version PublicationVersion
	Link    types.ParentLink
}

// resolveReviseSources reads every source from the platform and checks it
// before anything is written: it is served (so published, not tombstoned), its
// manifest hash is the request's, and it holds no private embed. The source's
// owner and author label come from the platform's answer, never the request.
func (h *Handler) resolveReviseSources(ctx context.Context, sources []ReviseSource) ([]resolvedSource, error) {
	reader := h.versionReader()
	out := make([]resolvedSource, 0, len(sources))
	for _, s := range sources {
		id, pin := strings.TrimSpace(s.PublicationVersionID), strings.TrimSpace(s.ManifestHash)
		if reader == nil {
			return nil, refuseOwnerRevision(http.StatusServiceUnavailable, "the platform is not reachable; nothing was written")
		}
		v, err := reader.ReadPublicationVersion(ctx, id)
		switch {
		case errors.Is(err, ErrSourceNotReadable):
			return nil, refuseOwnerRevision(http.StatusUnprocessableEntity, "source "+id+" is not a published version")
		case err != nil:
			log.Printf("texture api: read source %s: %v", id, err)
			return nil, refuseOwnerRevision(http.StatusServiceUnavailable, "the platform is not reachable; nothing was written")
		}
		if v.PublicationVersionID != id || v.ManifestHash != pin {
			return nil, refuseOwnerRevision(http.StatusUnprocessableEntity, "source "+id+" does not match its manifest hash")
		}
		if private := texturedoc.PrivateEmbedsInBody(v.BodyDoc); len(private) > 0 {
			return nil, refuseOwnerRevision(http.StatusUnprocessableEntity, "source "+id+" holds private embeds and cannot be taken")
		}
		out = append(out, resolvedSource{Version: v, Link: types.ParentLink{
			Role: types.ParentLinkSource, PublicationVersionID: id, ManifestHash: v.ManifestHash,
			RoutePath: v.RoutePath, AuthorLabel: v.AuthorLabel,
		}})
	}
	return out, nil
}

func sourceLinks(resolved []resolvedSource) []types.ParentLink {
	links := make([]types.ParentLink, 0, len(resolved))
	for _, r := range resolved {
		links = append(links, r.Link)
	}
	return links
}

// reviseTake is the take case: a published version's body becomes the
// document's next revision, authored by the requester, with no model call and
// no wake. On a settled document it is the existing continue-then-commit
// sequence (two store acts).
func (h *Handler) reviseTake(ctx context.Context, doc types.Document, ownerID string, req ReviseRequest, fingerprint string) (ReviseReceipt, error) {
	head := doc.CurrentRevisionID
	for _, named := range []string{req.BaseRevisionID, req.ExpectedHeadRevisionID} {
		if named = strings.TrimSpace(named); named != "" && named != head {
			return ReviseReceipt{}, refuseOwnerRevision(http.StatusConflict, "document head changed; the head is "+head+"; reload it before taking a version")
		}
	}
	resolved, err := h.resolveReviseSources(ctx, req.Sources)
	if err != nil {
		return ReviseReceipt{}, err
	}
	v := resolved[0].Version
	body := ReviseBody{Content: v.Content, BodyDoc: v.BodyDoc, SourceEntities: v.SourceEntities}
	continuedWorkID := ""
	if strings.TrimSpace(doc.TrajectoryID) != "" {
		snapshot, err := h.Store.GetLifecycleSnapshot(ctx, ownerID, doc.ComputerID, doc.TrajectoryID)
		if err != nil {
			return ReviseReceipt{}, refuseOwnerRevision(http.StatusConflict, "durable lifecycle state is unavailable or terminal")
		}
		if snapshot.Trajectory.Status == types.TrajectorySettled {
			continued, continuedSnapshot, err := h.continueSettledDocumentForOwner(ctx, ownerID, doc, snapshot)
			if err != nil {
				log.Printf("texture api: continue settled document %s for take: %v", doc.DocID, err)
				return ReviseReceipt{}, refuseOwnerRevision(http.StatusConflict, "durable lifecycle state is unavailable or terminal")
			}
			doc, snapshot = continued, continuedSnapshot
			_, continuedWorkID, _ = ownerRevisionTarget(snapshot, doc.DocID)
		}
		if snapshot.Document.CurrentRevisionID != head {
			return ReviseReceipt{}, refuseOwnerRevision(http.StatusConflict, "document head changed; the head is "+snapshot.Document.CurrentRevisionID+"; reload it before taking a version")
		}
		body.ExpectedLifecycleVersion = snapshot.Trajectory.LifecycleVersion
	}
	links := append([]types.ParentLink{{Role: types.ParentLinkBase, RevisionID: head}}, sourceLinks(resolved)...)
	metadata := map[string]any{
		reviseFingerprintMetadataKey: fingerprint,
		reviseClientIDMetadataKey:    strings.TrimSpace(req.ClientRequestID),
	}
	if req.forkedFrom {
		metadata["forked_from"] = resolved[0].Link
	}
	receipt, err := h.commitOwnerBodyEdit(ctx, doc, ownerID, ownerBodyEdit{
		Body: body, ParentRevisionID: head, IdempotencyKey: strings.TrimSpace(req.ClientRequestID),
		Links: links, NoWake: true, Verbatim: true,
		Metadata: metadata,
	})
	receipt.Case = ReviseTake
	if err == nil && continuedWorkID != "" {
		// The continue opened desk work that no desk will serve (a take wakes
		// nobody): the take that answers it closes it, as a fork's does.
		if serr := h.settleUnservedWork(ctx, ownerID, doc, continuedWorkID, receipt.RevisionID); serr != nil {
			return ReviseReceipt{}, serr
		}
	}
	return receipt, err
}

// settleUnservedWork completes a work item of the document's desk that a
// no-model commit answered. The command id is the item's, so a retry settles once.
func (h *Handler) settleUnservedWork(ctx context.Context, ownerID string, doc types.Document, workItemID, resultRef string) error {
	settle := types.SettleLifecycleWorkRequest{
		OwnerID: ownerID, ComputerID: doc.ComputerID, CommandID: "revise-settle:" + workItemID,
		TrajectoryID: doc.TrajectoryID, WorkItemID: workItemID, ActingAgentID: currentTextureAgentID(doc.DocID), ResultRef: resultRef,
	}
	var err error
	if settle.CommandDigest, err = store.ComputeSettleLifecycleWorkDigest(settle); err != nil {
		return refuseOwnerRevision(http.StatusInternalServerError, "failed to close the unserved work")
	}
	if _, err := h.Store.SettleLifecycleWork(asOwner(ctx), settle); err != nil {
		log.Printf("texture api: close unserved work %s: %v", workItemID, err)
		return refuseOwnerRevision(http.StatusInternalServerError, "failed to close the unserved work")
	}
	return nil
}
