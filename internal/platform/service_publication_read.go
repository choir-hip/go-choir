package platform

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// GetPublicationBundleByRoute resolves a public route to one version of a
// publication (Cut 2 unit 2b). The bare link /pub/texture/<slug> serves the
// latest version in place; /pub/texture/<slug>/v<n> serves version n, and a
// number that was never published (a gap) is not found. The bundle carries
// the version's immutable payload and, beside it, the live nav and check
// overlay.
// publicationHead is what every resolve reads first: the route, the
// publication it names, and the published version the link asks for.
type publicationHead struct {
	og        *ObjectGraphStore
	routePath string
	bare      string
	revision  int
	routeMeta struct {
		State    string `json:"state"`
		TargetID string `json:"target_id"`
	}
	pubMeta struct {
		PublicationID string `json:"publication_id"`
		Slug          string `json:"slug"`
		Title         string `json:"title"`
		State         string `json:"state"`
		PublishMode   string `json:"publish_mode"`
		AuthorLabel   string `json:"author_label"`
		OwnerID       string `json:"owner_id"`
	}
	// story is the server's record that this is an Autopaper story
	// (isAutopaperStory), set where the head is resolved.
	story    bool
	versions []publishedVersion
	ref      publishedVersion
}

// resolvePublicationHead reads a route's publication and the version its
// link names (the latest for a bare link). sql.ErrNoRows means no public
// version answers the link.
func (s *Service) resolvePublicationHead(ctx context.Context, routePath string, steps *resolveSteps) (*publicationHead, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("platform service unavailable")
	}
	og := s.ogStore()
	if og == nil {
		return nil, fmt.Errorf("platform service: object graph store unavailable")
	}
	h := &publicationHead{og: og, routePath: normalizePublicationRoutePath(routePath)}
	bare, revision, ok := parsePublicationRoute(h.routePath)
	if !ok {
		return nil, sql.ErrNoRows
	}
	h.bare, h.revision = bare, revision

	// 1. The bare route names the publication.
	routeObj, err := og.GetObjectByMetadata(ctx, "choir.public_route", "$.route_path", bare)
	if err != nil {
		if err == objectgraph.ErrNotFound || err == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, fmt.Errorf("platform bundle: find route: %w", err)
	}
	steps.mark("route")
	if err := json.Unmarshal(routeObj.Metadata, &h.routeMeta); err != nil {
		return nil, fmt.Errorf("platform bundle: parse route metadata: %w", err)
	}
	if h.routeMeta.State != "active" {
		return nil, sql.ErrNoRows
	}
	pubObj, err := og.GetObject(ctx, h.routeMeta.TargetID)
	if err != nil {
		if err == objectgraph.ErrNotFound {
			return nil, sql.ErrNoRows
		}
		return nil, fmt.Errorf("platform bundle: get publication: %w", err)
	}
	if err := json.Unmarshal(pubObj.Metadata, &h.pubMeta); err != nil {
		return nil, fmt.Errorf("platform bundle: parse publication metadata: %w", err)
	}
	if h.pubMeta.State != "published" {
		return nil, sql.ErrNoRows
	}
	h.story = s.isAutopaperStory(frontPublicationMeta{PublishMode: h.pubMeta.PublishMode, OwnerID: h.pubMeta.OwnerID})

	// 2. The version: n, or the latest when the link is bare.
	h.versions, err = publishedVersions(ctx, og, pubObj.CanonicalID)
	if err != nil {
		return nil, fmt.Errorf("platform bundle: list versions: %w", err)
	}
	steps.mark("versions")
	if len(h.versions) == 0 {
		return nil, sql.ErrNoRows
	}
	if h.revision == 0 {
		h.revision = h.versions[len(h.versions)-1].Revision
	}
	h.ref, ok = findPublishedVersion(h.versions, h.revision)
	if !ok {
		return nil, sql.ErrNoRows
	}
	return h, nil
}

func (h *publicationHead) route() PublicationRoute {
	return PublicationRoute{Path: h.routePath, State: h.routeMeta.State, Bare: h.bare, Permalink: versionPermalink(h.bare, h.revision)}
}

func (h *publicationHead) publication() PublicationSummary {
	out := PublicationSummary{ID: h.pubMeta.PublicationID, Title: h.pubMeta.Title, Slug: h.pubMeta.Slug, State: h.pubMeta.State}
	if h.pubMeta.PublishMode == PublishModeOpinion {
		out.Kind, out.AuthorLabel = "opinion", h.pubMeta.AuthorLabel
	} else if h.story {
		out.Kind = "story"
	}
	return out
}

// GetPublicationNavByRoute answers a link with its route, publication and
// nav alone: the reader's poll for a newer version needs nothing else, and
// the full bundle costs about a second to build
// (docs/archive/problems/article-open-slow-store-b-scans-2026-10-16.md).
func (s *Service) GetPublicationNavByRoute(ctx context.Context, routePath string) (*PublicationBundle, error) {
	steps := newResolveSteps()
	defer steps.logIfSlow(routePath + " (nav)")
	h, err := s.resolvePublicationHead(ctx, routePath, steps)
	if err != nil {
		return nil, err
	}
	if policy := s.graphPublicationPolicy(ctx, h.og, h.ref.CanonicalID); !publicationAccessPolicyAllowsPublicRoute(policy.Access) {
		return nil, sql.ErrNoRows
	}
	return &PublicationBundle{Route: h.route(), Publication: h.publication(), Nav: navFor(h.versions, h.revision)}, nil
}

func (s *Service) GetPublicationBundleByRoute(ctx context.Context, routePath string) (*PublicationBundle, error) {
	steps := newResolveSteps()
	defer steps.logIfSlow(routePath)
	h, err := s.resolvePublicationHead(ctx, routePath, steps)
	if err != nil {
		return nil, err
	}
	og, revision, ref, versions := h.og, h.revision, h.ref, h.versions
	versionObj, err := og.GetObject(ctx, ref.CanonicalID)
	if err != nil {
		return nil, fmt.Errorf("platform bundle: get version: %w", err)
	}
	versionMeta, err := decodeVersionMetadata(versionObj)
	if err != nil {
		return nil, fmt.Errorf("platform bundle: %w", err)
	}
	publishedAt, _ := time.Parse(time.RFC3339Nano, versionMeta.PublishedAt)
	steps.mark("version")

	// 3. The payload: manifest, text blob, resolved blocks, signatures.
	manifestEdges, err := og.ListEdgesByKind(ctx, versionObj.CanonicalID, "has_manifest")
	if err != nil {
		return nil, fmt.Errorf("platform bundle: find manifest edges: %w", err)
	}
	if len(manifestEdges) == 0 {
		return nil, fmt.Errorf("platform bundle: no manifest edge for version %s", versionObj.CanonicalID)
	}
	manifestObj, err := og.GetObject(ctx, manifestEdges[0].ToID)
	if err != nil {
		return nil, fmt.Errorf("platform bundle: get manifest: %w", err)
	}
	if sha256Hex(manifestObj.Body) != versionMeta.ManifestHash {
		return nil, fmt.Errorf("platform bundle: v%d manifest does not hash to %s", revision, versionMeta.ManifestHash)
	}
	steps.mark("manifest")
	blobEdges, err := og.ListEdgesByKind(ctx, manifestObj.CanonicalID, "contains_blob")
	if err != nil {
		return nil, fmt.Errorf("platform bundle: find blob edges: %w", err)
	}
	if len(blobEdges) == 0 {
		return nil, fmt.Errorf("platform bundle: no blob edge for manifest %s", manifestObj.CanonicalID)
	}
	blobObj, err := og.GetObject(ctx, blobEdges[0].ToID)
	if err != nil {
		return nil, fmt.Errorf("platform bundle: get blob: %w", err)
	}
	var blobMeta struct {
		StorageRef string `json:"storage_ref"`
	}
	if err := json.Unmarshal(blobObj.Metadata, &blobMeta); err != nil {
		return nil, fmt.Errorf("platform bundle: parse blob metadata: %w", err)
	}
	content, err := s.readBlob(blobMeta.StorageRef)
	if err != nil {
		return nil, err
	}
	steps.mark("blob")
	resolvedBlocks, err := s.graphResolvedBlocks(ctx, og, versionObj.CanonicalID, manifestObj.Body)
	if err != nil {
		return nil, err
	}
	steps.mark("blocks")
	signatures := versionMeta.Signatures
	if signatures == nil {
		signatures = []PublicationSignature{}
	}

	// 4. Retrieval spans, citations, source entities, transclusions, policy, provenance.
	spans, sourceID := s.graphRetrievalSpans(ctx, og, versionObj.CanonicalID, string(content))
	steps.mark("spans")
	citations := s.graphCitationEdges(ctx, og, versionObj.CanonicalID, versionMeta.SourceRevisionHash)
	steps.mark("citations")
	policy := s.graphPublicationPolicy(ctx, og, versionObj.CanonicalID)
	steps.mark("policy")
	sourceEntities := s.graphSourceEntities(ctx, og, versionObj.CanonicalID)
	steps.mark("entities")
	transclusions := s.graphTransclusions(ctx, og, versionObj.CanonicalID)
	steps.mark("transclusions")
	provenance := s.graphProvenanceSummary(ctx, og, versionObj.CanonicalID)
	steps.mark("provenance")
	bodyDoc, structuredSourceEntities := structuredArtifactFieldsFromManifest(string(manifestObj.Body))

	if !publicationAccessPolicyAllowsPublicRoute(policy.Access) {
		return nil, sql.ErrNoRows
	}

	// 5. The live fields: nav among the published versions, and the check
	// overlay keyed by this version and its citations.
	overlay := s.checkOverlay(ctx, CheckOverlayVersion{
		OwnerID:              versionObj.OwnerID,
		PublicationID:        versionMeta.PublicationID,
		PublicationVersionID: versionMeta.PublicationVersionID,
		Revision:             revision,
		ManifestHash:         versionMeta.ManifestHash,
	}, manifestAnchors(manifestObj.Body))
	steps.mark("overlay")

	return &PublicationBundle{
		Route:       h.route(),
		Publication: h.publication(),
		Version: PublicationVersionSummary{
			ID:                   versionMeta.PublicationVersionID,
			Revision:             revision,
			ContentHash:          versionObj.ContentHash,
			SourceRevisionHash:   versionMeta.SourceRevisionHash,
			ProjectionHash:       versionMeta.ProjectionHash,
			ManifestHash:         versionMeta.ManifestHash,
			PreviousManifestHash: versionMeta.PreviousManifestHash,
			SupersedesVersionID:  versionMeta.SupersedesVersionID,
			PublishedAt:          publishedAt,
		},
		Artifact: PublicationArtifact{
			ManifestID:     versionMeta.ArtifactManifestID,
			MediaType:      textMediaType,
			Content:        string(content),
			BodyDoc:        bodyDoc,
			SourceEntities: structuredSourceEntities,
			RenderModel:    renderBlocks(string(content), spans),
		},
		Retrieval: RetrievalBundle{
			SourceID: sourceID,
			Spans:    spans,
		},
		Citations:      citations,
		SourceEntities: sourceEntities,
		Transclusions:  transclusions,
		Policy:         policy,
		Proposals: PublicationProposalCapability{
			CanSubmit:           true,
			SourcePublicationID: h.pubMeta.PublicationID,
		},
		Provenance: provenance,
		Payload: PublicationVersionPayload{
			Revision:       revision,
			Text:           string(content),
			Manifest:       json.RawMessage(manifestObj.Body),
			ManifestHash:   versionMeta.ManifestHash,
			Signatures:     signatures,
			ResolvedBlocks: resolvedBlocks,
		},
		Nav:    navFor(versions, revision),
		Checks: overlay,
	}, nil
}

// graphResolvedBlocks reads a version's pinned blocks and checks them
// against the hash its manifest names.
func (s *Service) graphResolvedBlocks(ctx context.Context, og *ObjectGraphStore, versionID string, manifest []byte) ([]ResolvedBlock, error) {
	var envelope struct {
		ResolvedBlocksHash string `json:"resolved_blocks_hash"`
	}
	if err := json.Unmarshal(manifest, &envelope); err != nil {
		return nil, fmt.Errorf("platform bundle: parse manifest: %w", err)
	}
	edges, err := og.ListEdgesByKind(ctx, versionID, "has_resolved_blocks")
	if err != nil {
		return nil, fmt.Errorf("platform bundle: find resolved blocks: %w", err)
	}
	if len(edges) == 0 {
		return nil, fmt.Errorf("platform bundle: no resolved blocks for version %s", versionID)
	}
	obj, err := og.GetObject(ctx, edges[0].ToID)
	if err != nil {
		return nil, fmt.Errorf("platform bundle: get resolved blocks: %w", err)
	}
	if sha256Hex(obj.Body) != envelope.ResolvedBlocksHash {
		return nil, fmt.Errorf("platform bundle: resolved blocks of %s do not hash to the manifest's %s", versionID, envelope.ResolvedBlocksHash)
	}
	var doc resolvedBlocksDoc
	if err := json.Unmarshal(obj.Body, &doc); err != nil {
		return nil, fmt.Errorf("platform bundle: parse resolved blocks: %w", err)
	}
	if doc.Blocks == nil {
		doc.Blocks = []ResolvedBlock{}
	}
	return doc.Blocks, nil
}

// graphRetrievalSpans reads retrieval spans from the object graph.
func (s *Service) graphRetrievalSpans(ctx context.Context, og *ObjectGraphStore, versionID, content string) ([]RetrievalSpan, string) {
	// Find the retrieval source via has_retrieval_source edge from the version.
	sourceEdges, err := og.ListEdgesByKind(ctx, versionID, "has_retrieval_source")
	if err == nil && len(sourceEdges) > 0 {
		sourceObj, err := og.GetObject(ctx, sourceEdges[0].ToID)
		if err == nil {
			var srcMeta struct {
				SourceID string `json:"source_id"`
			}
			_ = json.Unmarshal(sourceObj.Metadata, &srcMeta)
			// Now follow contains_span edges from the source.
			spanEdges, err := og.ListEdgesByKind(ctx, sourceObj.CanonicalID, "contains_span")
			if err == nil {
				spans := []RetrievalSpan{}
				for _, edge := range spanEdges {
					spanObj, err := og.GetObject(ctx, edge.ToID)
					if err != nil {
						continue
					}
					var spanMeta struct {
						SourceVersionID string      `json:"source_version_id"`
						SelectorKind    string      `json:"selector_kind"`
						SelectorJSON    string      `json:"selector_json"`
						TextHash        string      `json:"text_hash"`
						ChunkHash       string      `json:"chunk_hash"`
						TokenCount      json.Number `json:"token_count"`
					}
					spans = append(spans, RetrievalSpan{
						SourceID:        srcMeta.SourceID,
						ID:              spanObj.CanonicalID,
						SourceVersionID: spanMeta.SourceVersionID,
						SelectorKind:    spanMeta.SelectorKind,
						Selector:        json.RawMessage(firstNonEmpty(spanMeta.SelectorJSON, "{}")),
						TextHash:        spanMeta.TextHash,
						ChunkHash:       spanMeta.ChunkHash,
						TokenCount:      jsonInt64(spanMeta.TokenCount),
						Snippet:         snippet(content, ""),
					})
				}
				if len(spans) > 0 {
					return spans, srcMeta.SourceID
				}
			}
		}
	}
	// Fallback: look for contains_span edges directly from the version.
	spanEdges, err := og.ListEdgesByKind(ctx, versionID, "contains_span")
	if err != nil {
		return []RetrievalSpan{}, ""
	}
	spans := []RetrievalSpan{}
	sourceID := ""
	for _, edge := range spanEdges {
		spanObj, err := og.GetObject(ctx, edge.ToID)
		if err != nil {
			continue
		}
		var spanMeta struct {
			SourceVersionID string      `json:"source_version_id"`
			SelectorKind    string      `json:"selector_kind"`
			SelectorJSON    string      `json:"selector_json"`
			TextHash        string      `json:"text_hash"`
			ChunkHash       string      `json:"chunk_hash"`
			TokenCount      json.Number `json:"token_count"`
		}
		if err := json.Unmarshal(spanObj.Metadata, &spanMeta); err != nil {
			continue
		}
		spans = append(spans, RetrievalSpan{
			SourceID:        sourceID,
			ID:              spanObj.CanonicalID,
			SourceVersionID: spanMeta.SourceVersionID,
			SelectorKind:    spanMeta.SelectorKind,
			Selector:        json.RawMessage(firstNonEmpty(spanMeta.SelectorJSON, "{}")),
			TextHash:        spanMeta.TextHash,
			ChunkHash:       spanMeta.ChunkHash,
			TokenCount:      jsonInt64(spanMeta.TokenCount),
			Snippet:         snippet(content, ""),
		})
	}
	return spans, sourceID
}

// graphCitationEdges reads citation edges from the object graph.
func (s *Service) graphCitationEdges(ctx context.Context, og *ObjectGraphStore, versionID, sourceRevisionHash string) []CitationEdge {
	edges, err := og.ListEdgesFrom(ctx, versionID)
	if err != nil {
		return []CitationEdge{}
	}
	citations := []CitationEdge{}
	for _, edge := range edges {
		// Citation edges are any edge that's not a structural edge.
		switch string(edge.Kind) {
		case "has_version", "has_manifest", "routes_to", "has_policy",
			"references_entity", "transcludes", "has_retrieval_manifest",
			"contains_span", "contains_blob", "has_resolved_blocks", "supersedes",
			string(edgeCitesCapture):
			continue
		}
		var meta struct {
			FromSelectorJSON string `json:"from_selector_json"`
			ToSelectorJSON   string `json:"to_selector_json"`
			State            string `json:"state"`
			ProposedBy       string `json:"proposed_by"`
			EvidenceRef      string `json:"evidence_ref"`
			Confidence       string `json:"confidence"`
		}
		_ = json.Unmarshal(edge.Metadata, &meta)
		confidence := 0.5
		fmt.Sscanf(meta.Confidence, "%f", &confidence)
		toKind := "external_reference"
		toID := edge.ToID
		if edge.Kind == "is_version_of" {
			toKind = "source_revision_hash"
			toID = sourceRevisionHash
			if strings.HasPrefix(meta.EvidenceRef, "source_revision_hash:") {
				toID = strings.TrimPrefix(meta.EvidenceRef, "source_revision_hash:")
			}
		}
		citations = append(citations, CitationEdge{
			ID:           edge.EdgeID,
			FromKind:     "publication_version",
			FromID:       versionID,
			FromSelector: json.RawMessage(firstNonEmpty(meta.FromSelectorJSON, "{}")),
			ToKind:       toKind,
			ToID:         toID,
			ToSelector:   json.RawMessage(firstNonEmpty(meta.ToSelectorJSON, "{}")),
			RelationType: string(edge.Kind),
			State:        meta.State,
			ProposedBy:   meta.ProposedBy,
			EvidenceRef:  meta.EvidenceRef,
			Confidence:   confidence,
		})
	}
	return citations
}

// graphPublicationPolicy reads the publication policy from the object graph.
func (s *Service) graphPublicationPolicy(ctx context.Context, og *ObjectGraphStore, versionID string) PublicationPolicy {
	edges, err := og.ListEdgesByKind(ctx, versionID, "has_policy")
	if err != nil || len(edges) == 0 {
		return PublicationPolicy{Access: defaultPublicationAccessPolicy(), Export: defaultPublicationExportPolicy()}
	}
	policyObj, err := og.GetObject(ctx, edges[0].ToID)
	if err != nil {
		return PublicationPolicy{Access: defaultPublicationAccessPolicy(), Export: defaultPublicationExportPolicy()}
	}
	var meta struct {
		AccessPolicyJSON string `json:"access_policy_json"`
		ExportPolicyJSON string `json:"export_policy_json"`
	}
	_ = json.Unmarshal(policyObj.Metadata, &meta)
	return PublicationPolicy{
		Access: json.RawMessage(firstNonEmpty(meta.AccessPolicyJSON, "{}")),
		Export: json.RawMessage(firstNonEmpty(meta.ExportPolicyJSON, "{}")),
	}
}

// graphSourceEntities reads publication source entities from the object graph.
func (s *Service) graphSourceEntities(ctx context.Context, og *ObjectGraphStore, versionID string) []PublicationSourceEntity {
	edges, err := og.ListEdgesByKind(ctx, versionID, "references_entity")
	if err != nil {
		return []PublicationSourceEntity{}
	}
	entities := []PublicationSourceEntity{}
	for _, edge := range edges {
		obj, err := og.GetObject(ctx, edge.ToID)
		if err != nil {
			continue
		}
		var meta struct {
			SourceEntityID string `json:"source_entity_id"`
			Kind           string `json:"kind"`
			TargetKind     string `json:"target_kind"`
			TargetID       string `json:"target_id"`
			DisplayPolicy  string `json:"display_policy"`
			OpenSurface    string `json:"open_surface"`
		}
		_ = json.Unmarshal(obj.Metadata, &meta)
		entities = append(entities, PublicationSourceEntity{
			ID:             obj.CanonicalID,
			SourceEntityID: meta.SourceEntityID,
			Kind:           meta.Kind,
			TargetKind:     meta.TargetKind,
			TargetID:       meta.TargetID,
			DisplayPolicy:  meta.DisplayPolicy,
			OpenSurface:    meta.OpenSurface,
			Entity:         json.RawMessage(firstNonEmpty(string(obj.Body), "{}")),
		})
	}
	return entities
}

// graphTransclusions reads publication transclusions from the object graph.
func (s *Service) graphTransclusions(ctx context.Context, og *ObjectGraphStore, versionID string) []PublicationTransclusion {
	edges, err := og.ListEdgesByKind(ctx, versionID, "transcludes")
	if err != nil {
		return []PublicationTransclusion{}
	}
	transclusions := []PublicationTransclusion{}
	for _, edge := range edges {
		obj, err := og.GetObject(ctx, edge.ToID)
		if err != nil {
			continue
		}
		var meta struct {
			SourceEntityID     string     `json:"source_entity_id"`
			AnchorID           string     `json:"anchor_id"`
			Ref                *types.Ref `json:"ref"`
			HostSelectorJSON   string     `json:"host_selector_json"`
			SourceSelectorJSON string     `json:"source_selector_json"`
			RelationType       string     `json:"relation_type"`
			DefaultDisplayMode string     `json:"default_display_mode"`
			AccessPolicyJSON   string     `json:"access_policy_json"`
			ExportPolicyJSON   string     `json:"export_policy_json"`
		}
		_ = json.Unmarshal(obj.Metadata, &meta)
		transclusions = append(transclusions, PublicationTransclusion{
			ID:                 obj.CanonicalID,
			SourceEntityID:     meta.SourceEntityID,
			AnchorID:           meta.AnchorID,
			Ref:                meta.Ref,
			HostSelector:       json.RawMessage(firstNonEmpty(meta.HostSelectorJSON, "{}")),
			SourceSelector:     json.RawMessage(firstNonEmpty(meta.SourceSelectorJSON, "{}")),
			RelationType:       meta.RelationType,
			DefaultDisplayMode: meta.DefaultDisplayMode,
			SnapshotText:       string(obj.Body),
			ContentHash:        obj.ContentHash,
			AccessPolicy:       json.RawMessage(firstNonEmpty(meta.AccessPolicyJSON, "{}")),
			ExportPolicy:       json.RawMessage(firstNonEmpty(meta.ExportPolicyJSON, "{}")),
		})
	}
	return transclusions
}

// provenanceEdgesSQL reads a version's consent, review and attestation
// edges: a seek on (to_id, kind). The plan tests explain this text.
const provenanceEdgesSQL = `SELECT edge_id, from_id, to_id, kind, metadata, created_at, tombstone
		 FROM og_edges WHERE to_id = ? AND kind IN ('consent_for', 'reviews', 'attests_to') AND tombstone = FALSE ORDER BY created_at`

// graphProvenanceSummary reads provenance summary from the object graph.
func (s *Service) graphProvenanceSummary(ctx context.Context, og *ObjectGraphStore, versionID string) PublicationProvenanceSummary {
	var out PublicationProvenanceSummary
	done := og.store.retrievalTimer(retrievalListEdgesTo)
	rowCount := 0
	var readErr error
	defer func() { done("", rowCount, readErr) }()
	// Find consent, review, and attestation objects via edges to this
	// version: a seek on (to_id, kind) for the three kinds read below.
	rows, err := og.store.corpus().QueryContext(ctx, provenanceEdgesSQL, versionID)
	if err != nil {
		readErr = err
		return out
	}
	defer rows.Close()
	for rows.Next() {
		edge, err := scanObjectGraphEdge(rows)
		if err != nil {
			continue
		}
		rowCount++
		switch string(edge.Kind) {
		case "consent_for":
			out.ConsentIDs = append(out.ConsentIDs, edge.FromID)
		case "reviews":
			out.ReviewIDs = append(out.ReviewIDs, edge.FromID)
		case "attests_to":
			out.AttestationIDs = append(out.AttestationIDs, edge.FromID)
		}
	}
	return out
}

func (s *Service) ExportPublicationByRoute(ctx context.Context, routePath, format string) (*PublicationExport, error) {
	bundle, err := s.GetPublicationBundleByRoute(ctx, routePath)
	if err != nil {
		return nil, err
	}
	format = normalizeExportFormat(format)
	if format == "" {
		return nil, fmt.Errorf("unsupported export format")
	}
	if !publicationExportAllowed(bundle.Policy.Export, format) {
		return nil, fmt.Errorf("export format %s is not allowed by publication policy", format)
	}
	exported, err := buildPublicationExportBytes(bundle, format)
	if err != nil {
		return nil, err
	}
	mediaType := exportMediaType(format)
	filename := publicationExportFilename(bundle.Publication.Slug, bundle.Publication.Title, format)
	content := string(exported.content)
	contentBase64 := ""
	if exportFormatIsBinary(format) {
		content = ""
		contentBase64 = base64.StdEncoding.EncodeToString(exported.content)
	}
	return &PublicationExport{
		RoutePath:            bundle.Route.Path,
		PublicationID:        bundle.Publication.ID,
		PublicationVersionID: bundle.Version.ID,
		Format:               format,
		MediaType:            mediaType,
		Filename:             filename,
		Content:              content,
		ContentBase64:        contentBase64,
		ContentHash:          sha256Hex(exported.content),
		Metadata:             exported.metadata,
	}, nil
}

// SearchPublished searches published textures via the object graph.
func (s *Service) SearchPublished(ctx context.Context, query string) (*RetrievalSearchResponse, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("platform service unavailable")
	}
	og := s.ogStore()
	if og == nil {
		return nil, fmt.Errorf("platform service: object graph store unavailable")
	}
	query = strings.TrimSpace(query)
	lowerQuery := strings.ToLower(query)

	// Every publication, newest first, up to the scan cap (this read used to
	// stop at the first 100 rows), then follow edges to get route + version +
	// blob.
	pubObjs, _, err := s.scanPublicationRows(ctx, publishedPieceScanMax)
	if err != nil {
		return nil, fmt.Errorf("platform retrieval: list publications: %w", err)
	}

	results := []RetrievalSearchResult{}
	for _, pubObj := range pubObjs {
		var pubMeta struct {
			PublicationID string `json:"publication_id"`
			Title         string `json:"title"`
			Slug          string `json:"slug"`
			State         string `json:"state"`
		}
		if err := json.Unmarshal(pubObj.Metadata, &pubMeta); err != nil {
			continue
		}
		if pubMeta.State != "published" {
			continue
		}

		// The latest published version.
		versions, err := publishedVersions(ctx, og, pubObj.CanonicalID)
		if err != nil || len(versions) == 0 {
			continue
		}
		versionObj, err := og.GetObject(ctx, versions[len(versions)-1].CanonicalID)
		if err != nil {
			continue
		}
		versionMeta, err := decodeVersionMetadata(versionObj)
		if err != nil {
			continue
		}

		// Get blob via has_manifest -> contains_blob.
		manifestEdges, err := og.ListEdgesByKind(ctx, versionObj.CanonicalID, "has_manifest")
		if err != nil || len(manifestEdges) == 0 {
			continue
		}
		blobEdges, err := og.ListEdgesByKind(ctx, manifestEdges[0].ToID, "contains_blob")
		if err != nil || len(blobEdges) == 0 {
			continue
		}
		blobObj, err := og.GetObject(ctx, blobEdges[0].ToID)
		if err != nil {
			continue
		}
		var blobMeta struct {
			StorageRef string `json:"storage_ref"`
		}
		_ = json.Unmarshal(blobObj.Metadata, &blobMeta)

		// Get route via routes_to edge (find route objects pointing to this publication).
		// Since routes point TO publications, we need to query edges by to_id.
		routeRows, err := og.store.corpus().QueryContext(ctx,
			`SELECT from_id FROM og_edges WHERE to_id = ? AND kind = 'routes_to' AND tombstone = FALSE LIMIT 1`,
			pubObj.CanonicalID)
		if err != nil {
			continue
		}
		var routeID string
		if routeRows.Next() {
			_ = routeRows.Scan(&routeID)
		}
		routeRows.Close()
		if routeID == "" {
			continue
		}
		routeObj, err := og.GetObject(ctx, routeID)
		if err != nil {
			continue
		}
		var routeMeta struct {
			RoutePath string `json:"route_path"`
			State     string `json:"state"`
		}
		_ = json.Unmarshal(routeObj.Metadata, &routeMeta)
		if routeMeta.State != "active" {
			continue
		}

		// Check policy.
		policy := s.graphPublicationPolicy(ctx, og, versionObj.CanonicalID)
		if !publicationAccessPolicyAllowsPublicRoute(policy.Access) {
			continue
		}

		// Get retrieval spans for the version.
		spans, sourceID := s.graphRetrievalSpans(ctx, og, versionObj.CanonicalID, "")
		spanID := ""
		if len(spans) > 0 {
			spanID = spans[0].ID
		}

		// Read content.
		contentBytes, err := s.readBlob(blobMeta.StorageRef)
		if err != nil {
			continue
		}
		content := string(contentBytes)
		score := 1
		if lowerQuery != "" {
			lowerContent := strings.ToLower(content)
			lowerTitle := strings.ToLower(pubMeta.Title)
			score = strings.Count(lowerContent, lowerQuery)
			if strings.Contains(lowerTitle, lowerQuery) {
				score += 3
			}
			if score == 0 {
				continue
			}
		}
		results = append(results, RetrievalSearchResult{
			PublicationID:        pubMeta.PublicationID,
			PublicationVersionID: versionMeta.PublicationVersionID,
			Title:                pubMeta.Title,
			RoutePath:            routeMeta.RoutePath,
			SourceID:             sourceID,
			SpanID:               spanID,
			ContentHash:          versionObj.ContentHash,
			SourceRevisionHash:   versionMeta.SourceRevisionHash,
			Snippet:              snippet(content, query),
			Score:                score,
		})
		if len(results) >= 20 {
			break
		}
	}
	return &RetrievalSearchResponse{Query: query, Results: results}, nil
}

// ogStore returns the object graph store from the service, or nil.
func (s *Service) ogStore() *ObjectGraphStore {
	if s == nil || s.store == nil {
		return nil
	}
	return NewObjectGraphStore(s.store)
}

// ── Helper functions (kept from the relational version) ─────────────────────

func normalizeExportFormat(format string) string {
	switch strings.TrimPrefix(strings.TrimSpace(strings.ToLower(format)), ".") {
	case "", "txt", "text":
		return "txt"
	case "md", "markdown":
		return "md"
	case "html":
		return "html"
	case "docx":
		return "docx"
	case "pdf":
		return "pdf"
	default:
		return ""
	}
}

func publicationExportAllowed(raw json.RawMessage, format string) bool {
	if len(raw) == 0 {
		return true
	}
	var policy map[string]any
	if err := json.Unmarshal(raw, &policy); err != nil {
		return false
	}
	if value, ok := policy["download_allowed"].(bool); ok && !value {
		return false
	}
	formats, ok := policy["formats"].([]any)
	if !ok || len(formats) == 0 {
		return true
	}
	for _, value := range formats {
		if strings.EqualFold(strings.TrimPrefix(strings.TrimSpace(fmt.Sprint(value)), "."), format) {
			return true
		}
	}
	return false
}

func publicationAccessPolicyAllowsPublicRoute(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	var policy struct {
		Visibility string `json:"visibility"`
		Route      string `json:"route"`
	}
	if err := json.Unmarshal(raw, &policy); err != nil {
		return false
	}
	visibility := strings.TrimSpace(strings.ToLower(policy.Visibility))
	route := strings.TrimSpace(strings.ToLower(policy.Route))
	return visibility == "public" && (route == "" || route == "public")
}

func exportMediaType(format string) string {
	switch format {
	case "html":
		return "text/html; charset=utf-8"
	case "md":
		return "text/markdown; charset=utf-8"
	case "docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case "pdf":
		return "application/pdf"
	default:
		return textMediaType
	}
}

func exportFormatIsBinary(format string) bool {
	switch format {
	case "docx", "pdf":
		return true
	default:
		return false
	}
}

func publicationExportFilename(slug, title, format string) string {
	base := slugify(firstNonEmpty(slug, title, defaultPublishedTextureSlugBase))
	if base == "" {
		base = defaultPublishedTextureSlugBase
	}
	return base + "." + format
}
