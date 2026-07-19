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
)

// GetPublicationBundleByRoute resolves a public route path to the full
// publication bundle using the object graph. The graph path is:
//  1. Find choir.public_route object by metadata route_path
//  2. Follow routes_to edge to choir.publication
//  3. Follow has_version edge to choir.publication_version
//  4. Follow has_manifest edge to choir.artifact_manifest
//  5. Follow contains_blob edge to choir.artifact_blob (for storage_ref)
//  6. Follow edges to retrieval, citations, source entities, transclusions, policy, provenance
func (s *Service) GetPublicationBundleByRoute(ctx context.Context, routePath string) (*PublicationBundle, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("platform service unavailable")
	}
	og := s.ogStore()
	if og == nil {
		return nil, fmt.Errorf("platform service: object graph store unavailable")
	}
	routePath = normalizePublicationRoutePath(routePath)

	// 1. Find the public_route object by route_path metadata.
	routeObj, err := og.GetObjectByMetadata(ctx, "choir.public_route", "$.route_path", routePath)
	if err != nil {
		if err == objectgraph.ErrNotFound || err == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, fmt.Errorf("platform bundle: find route: %w", err)
	}
	var routeMeta struct {
		State           string `json:"state"`
		TargetID        string `json:"target_id"`
		TargetVersionID string `json:"target_version_id"`
	}
	if err := json.Unmarshal(routeObj.Metadata, &routeMeta); err != nil {
		return nil, fmt.Errorf("platform bundle: parse route metadata: %w", err)
	}
	if routeMeta.State != "active" {
		return nil, sql.ErrNoRows
	}

	// 2. Get the publication object.
	pubObj, err := og.GetObject(ctx, routeMeta.TargetID)
	if err != nil {
		// The target_id in route metadata is the relational publication_id,
		// not the canonical_id. We need to find the publication by searching
		// for it. But since we stored it with the relational ID as the key,
		// we can look it up by metadata.
		pubObjs, err2 := og.ListObjectsByMetadata(ctx, "choir.publication", "$.latest_version_id", routeMeta.TargetVersionID, 1)
		if err2 != nil || len(pubObjs) == 0 {
			if err == objectgraph.ErrNotFound {
				return nil, sql.ErrNoRows
			}
			return nil, fmt.Errorf("platform bundle: get publication: %w", err)
		}
		pubObj = pubObjs[0]
	}
	var pubMeta struct {
		Slug            string `json:"slug"`
		Title           string `json:"title"`
		State           string `json:"state"`
		LatestVersionID string `json:"latest_version_id"`
	}
	if err := json.Unmarshal(pubObj.Metadata, &pubMeta); err != nil {
		return nil, fmt.Errorf("platform bundle: parse publication metadata: %w", err)
	}
	if pubMeta.State != "published" {
		return nil, sql.ErrNoRows
	}

	// 3. Get the publication_version object.
	// The route's target_version_id is the relational version ID.
	// Find the version object by metadata.
	versionObjs, err := og.ListObjectsByMetadata(ctx, "choir.publication_version", "$.artifact_manifest_id", "", 0)
	_ = versionObjs
	// Actually, we should follow the has_version edge from the publication.
	versionEdges, err := og.ListEdgesByKind(ctx, pubObj.CanonicalID, "has_version")
	if err != nil {
		return nil, fmt.Errorf("platform bundle: find version edges: %w", err)
	}
	if len(versionEdges) == 0 {
		return nil, fmt.Errorf("platform bundle: no version edges for publication %s", pubObj.CanonicalID)
	}
	// Use the latest version (last edge, since edges are ordered by created_at).
	versionEdge := versionEdges[len(versionEdges)-1]
	versionObj, err := og.GetObject(ctx, versionEdge.ToID)
	if err != nil {
		return nil, fmt.Errorf("platform bundle: get version: %w", err)
	}
	var versionMeta struct {
		SourceDocID         string `json:"source_doc_id"`
		SourceRevisionID    string `json:"source_revision_id"`
		SourceRevisionHash  string `json:"source_revision_hash"`
		ProjectionHash      string `json:"projection_hash"`
		ArtifactManifestID  string `json:"artifact_manifest_id"`
		PublishedAt         string `json:"published_at"`
		SupersedesVersionID string `json:"supersedes_version_id"`
	}
	if err := json.Unmarshal(versionObj.Metadata, &versionMeta); err != nil {
		return nil, fmt.Errorf("platform bundle: parse version metadata: %w", err)
	}
	publishedAt, _ := time.Parse(time.RFC3339, versionMeta.PublishedAt)

	// 4. Get the artifact manifest object.
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

	// 5. Get the artifact blob object (for storage_ref).
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

	// Read content from blob storage.
	content, err := s.readBlob(blobMeta.StorageRef)
	if err != nil {
		return nil, err
	}

	// 6. Get retrieval spans, citations, source entities, transclusions, policy, provenance.
	spans, sourceID := s.graphRetrievalSpans(ctx, og, versionObj.CanonicalID, string(content))
	citations := s.graphCitationEdges(ctx, og, versionObj.CanonicalID, versionMeta.SourceRevisionHash)
	policy := s.graphPublicationPolicy(ctx, og, versionObj.CanonicalID)
	sourceEntities := s.graphSourceEntities(ctx, og, versionObj.CanonicalID)
	transclusions := s.graphTransclusions(ctx, og, versionObj.CanonicalID)
	provenance := s.graphProvenanceSummary(ctx, og, versionObj.CanonicalID)
	versionHistory := graphVersionHistoryFromManifest(manifestObj.Body)
	bodyDoc, structuredSourceEntities := structuredArtifactFieldsFromManifest(string(manifestObj.Body))

	if !publicationAccessPolicyAllowsPublicRoute(policy.Access) {
		return nil, sql.ErrNoRows
	}

	return &PublicationBundle{
		Route: PublicationRoute{Path: routePath, State: routeMeta.State},
		Publication: PublicationSummary{
			ID:    pubMeta.LatestVersionID, // keep relational ID for API compat
			Title: pubMeta.Title,
			Slug:  pubMeta.Slug,
			State: pubMeta.State,
		},
		Version: PublicationVersionSummary{
			ID:                 versionMeta.ArtifactManifestID, // keep for API compat
			ContentHash:        versionObj.ContentHash,
			SourceRevisionHash: versionMeta.SourceRevisionHash,
			ProjectionHash:     versionMeta.ProjectionHash,
			PublishedAt:        publishedAt,
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
			SourcePublicationID: pubMeta.LatestVersionID,
		},
		Provenance:     provenance,
		VersionHistory: versionHistory,
	}, nil
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
			"contains_span", "contains_blob":
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
			SourceEntityID     string `json:"source_entity_id"`
			HostSelectorJSON   string `json:"host_selector_json"`
			SourceSelectorJSON string `json:"source_selector_json"`
			RelationType       string `json:"relation_type"`
			DefaultDisplayMode string `json:"default_display_mode"`
			AccessPolicyJSON   string `json:"access_policy_json"`
			ExportPolicyJSON   string `json:"export_policy_json"`
		}
		_ = json.Unmarshal(obj.Metadata, &meta)
		transclusions = append(transclusions, PublicationTransclusion{
			ID:                 obj.CanonicalID,
			SourceEntityID:     meta.SourceEntityID,
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

// graphProvenanceSummary reads provenance summary from the object graph.
func (s *Service) graphProvenanceSummary(ctx context.Context, og *ObjectGraphStore, versionID string) PublicationProvenanceSummary {
	var out PublicationProvenanceSummary
	// Find consent, review, and attestation objects via edges to this version.
	// These are edges TO the version, so we need to query by to_id.
	// Since our ObjectGraphStore doesn't have a ListEdgesTo method,
	// we query og_edges directly.
	rows, err := og.store.db.QueryContext(ctx,
		`SELECT edge_id, from_id, to_id, kind, metadata, created_at, tombstone
		 FROM og_edges WHERE to_id = ? AND tombstone = FALSE ORDER BY created_at`, versionID)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		edge, err := scanObjectGraphEdge(rows)
		if err != nil {
			continue
		}
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

// graphVersionHistoryFromManifest extracts version history from the manifest JSON body.
func graphVersionHistoryFromManifest(manifestBody []byte) *PublicationVersionHistory {
	if len(manifestBody) == 0 {
		return nil
	}
	var envelope struct {
		VersionHistory     *PublicationVersionHistory `json:"version_history"`
		VersionHistoryHash string                     `json:"version_history_hash"`
	}
	if err := json.Unmarshal(manifestBody, &envelope); err != nil {
		return nil
	}
	if envelope.VersionHistory == nil || envelope.VersionHistory.RevisionCount == 0 {
		return nil
	}
	if envelope.VersionHistory.ManifestHash == "" {
		envelope.VersionHistory.ManifestHash = envelope.VersionHistoryHash
	}
	return envelope.VersionHistory
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

	// List all publication objects, then follow edges to get route + version + blob.
	pubObjs, err := og.ListObjects(ctx, objectgraph.ListFilter{Kind: "choir.publication", Limit: 100})
	if err != nil {
		return nil, fmt.Errorf("platform retrieval: list publications: %w", err)
	}

	results := []RetrievalSearchResult{}
	for _, pubObj := range pubObjs {
		var pubMeta struct {
			Title           string `json:"title"`
			Slug            string `json:"slug"`
			State           string `json:"state"`
			LatestVersionID string `json:"latest_version_id"`
		}
		if err := json.Unmarshal(pubObj.Metadata, &pubMeta); err != nil {
			continue
		}
		if pubMeta.State != "published" {
			continue
		}

		// Get version via has_version edge.
		versionEdges, err := og.ListEdgesByKind(ctx, pubObj.CanonicalID, "has_version")
		if err != nil || len(versionEdges) == 0 {
			continue
		}
		versionObj, err := og.GetObject(ctx, versionEdges[len(versionEdges)-1].ToID)
		if err != nil {
			continue
		}
		var versionMeta struct {
			SourceRevisionHash string `json:"source_revision_hash"`
			ArtifactManifestID string `json:"artifact_manifest_id"`
		}
		_ = json.Unmarshal(versionObj.Metadata, &versionMeta)

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
		routeRows, err := og.store.db.QueryContext(ctx,
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
			PublicationID:        pubMeta.LatestVersionID,
			PublicationVersionID: versionMeta.ArtifactManifestID,
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
