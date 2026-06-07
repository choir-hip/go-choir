package platform

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

func (s *Service) GetPublicationBundleByRoute(ctx context.Context, routePath string) (*PublicationBundle, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("platform service unavailable")
	}
	routePath = normalizePublicationRoutePath(routePath)
	var rec struct {
		RouteState           string
		PublicationID        string
		OwnerID              string
		Slug                 string
		Title                string
		PublicationState     string
		PublicationVersionID string
		ContentHash          string
		SourceRevisionHash   string
		ProjectionHash       string
		ArtifactManifestID   string
		StorageRef           string
		PublishedAt          time.Time
	}
	err := s.store.db.QueryRowContext(ctx, `
SELECT pr.state, p.publication_id, p.owner_id, p.slug, p.title, p.state,
       pv.publication_version_id, pv.content_hash, pv.source_revision_hash,
       pv.projection_hash, pv.artifact_manifest_id, ab.storage_ref, pv.published_at
FROM public_routes pr
JOIN publications p ON p.publication_id = pr.target_id
JOIN publication_versions pv ON pv.publication_version_id = pr.target_version_id
JOIN artifact_blobs ab ON ab.artifact_manifest_id = pv.artifact_manifest_id
WHERE pr.route_path = ? AND pr.state = 'active' AND p.state = 'published'`, routePath).
		Scan(&rec.RouteState, &rec.PublicationID, &rec.OwnerID, &rec.Slug, &rec.Title, &rec.PublicationState,
			&rec.PublicationVersionID, &rec.ContentHash, &rec.SourceRevisionHash, &rec.ProjectionHash,
			&rec.ArtifactManifestID, &rec.StorageRef, &rec.PublishedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, err
		}
		return nil, fmt.Errorf("platform bundle: query route: %w", err)
	}
	content, err := s.readBlob(rec.StorageRef)
	if err != nil {
		return nil, err
	}
	spans, sourceID, err := s.retrievalSpans(ctx, rec.PublicationVersionID, string(content))
	if err != nil {
		return nil, err
	}
	citations, err := s.citationEdges(ctx, rec.PublicationVersionID, rec.SourceRevisionHash)
	if err != nil {
		return nil, err
	}
	policy, err := s.publicationPolicy(ctx, rec.PublicationVersionID)
	if err != nil {
		return nil, err
	}
	if !publicationAccessPolicyAllowsPublicRoute(policy.Access) {
		return nil, sql.ErrNoRows
	}
	sourceEntities, err := s.publicationSourceEntities(ctx, rec.PublicationVersionID)
	if err != nil {
		return nil, err
	}
	transclusions, err := s.publicationTransclusions(ctx, rec.PublicationVersionID)
	if err != nil {
		return nil, err
	}
	provenance, err := s.provenanceSummary(ctx, rec.PublicationVersionID)
	if err != nil {
		return nil, err
	}
	return &PublicationBundle{
		Route: PublicationRoute{Path: routePath, State: rec.RouteState},
		Publication: PublicationSummary{
			ID:    rec.PublicationID,
			Title: rec.Title,
			Slug:  rec.Slug,
			State: rec.PublicationState,
		},
		Version: PublicationVersionSummary{
			ID:                 rec.PublicationVersionID,
			ContentHash:        rec.ContentHash,
			SourceRevisionHash: rec.SourceRevisionHash,
			ProjectionHash:     rec.ProjectionHash,
			PublishedAt:        rec.PublishedAt,
		},
		Artifact: PublicationArtifact{
			ManifestID:  rec.ArtifactManifestID,
			MediaType:   textMediaType,
			Content:     string(content),
			RenderModel: renderBlocks(string(content), spans),
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
			SourcePublicationID: rec.PublicationID,
		},
		Provenance: provenance,
	}, nil
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

func (s *Service) SearchPublished(ctx context.Context, query string) (*RetrievalSearchResponse, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("platform service unavailable")
	}
	query = strings.TrimSpace(query)
	lowerQuery := strings.ToLower(query)
	rows, err := s.store.db.QueryContext(ctx, `
SELECT p.publication_id, p.title, pr.route_path, pv.publication_version_id,
       pv.content_hash, pv.source_revision_hash, ab.storage_ref,
       rs.source_id, rsp.span_id, pp.access_policy_json
FROM publications p
JOIN publication_versions pv ON pv.publication_version_id = p.latest_version_id
JOIN public_routes pr ON pr.target_id = p.publication_id AND pr.target_version_id = pv.publication_version_id
JOIN artifact_blobs ab ON ab.artifact_manifest_id = pv.artifact_manifest_id
JOIN retrieval_sources rs ON rs.content_hash = pv.content_hash AND rs.state = 'active'
JOIN retrieval_spans rsp ON rsp.source_id = rs.source_id AND rsp.source_version_id = pv.publication_version_id
JOIN publication_policies pp ON pp.publication_version_id = pv.publication_version_id
WHERE p.state = 'published' AND pr.state = 'active'
ORDER BY pv.published_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("platform retrieval: query published sources: %w", err)
	}
	defer rows.Close()

	results := []RetrievalSearchResult{}
	for rows.Next() {
		var rec struct {
			PublicationID        string
			Title                string
			RoutePath            string
			PublicationVersionID string
			ContentHash          string
			SourceRevisionHash   string
			StorageRef           string
			SourceID             string
			SpanID               string
			AccessPolicy         string
		}
		if err := rows.Scan(&rec.PublicationID, &rec.Title, &rec.RoutePath, &rec.PublicationVersionID,
			&rec.ContentHash, &rec.SourceRevisionHash, &rec.StorageRef, &rec.SourceID, &rec.SpanID, &rec.AccessPolicy); err != nil {
			return nil, fmt.Errorf("platform retrieval: scan source: %w", err)
		}
		if !publicationAccessPolicyAllowsPublicRoute(json.RawMessage(rec.AccessPolicy)) {
			continue
		}
		contentBytes, err := s.readBlob(rec.StorageRef)
		if err != nil {
			return nil, err
		}
		content := string(contentBytes)
		score := 1
		if lowerQuery != "" {
			lowerContent := strings.ToLower(content)
			lowerTitle := strings.ToLower(rec.Title)
			score = strings.Count(lowerContent, lowerQuery)
			if strings.Contains(lowerTitle, lowerQuery) {
				score += 3
			}
			if score == 0 {
				continue
			}
		}
		results = append(results, RetrievalSearchResult{
			PublicationID:        rec.PublicationID,
			PublicationVersionID: rec.PublicationVersionID,
			Title:                rec.Title,
			RoutePath:            rec.RoutePath,
			SourceID:             rec.SourceID,
			SpanID:               rec.SpanID,
			ContentHash:          rec.ContentHash,
			SourceRevisionHash:   rec.SourceRevisionHash,
			Snippet:              snippet(content, query),
			Score:                score,
		})
		if len(results) >= 20 {
			break
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("platform retrieval: iterate sources: %w", err)
	}
	return &RetrievalSearchResponse{Query: query, Results: results}, nil
}

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
	base := slugify(firstNonEmpty(slug, title, "published-vtext"))
	if base == "" {
		base = "published-vtext"
	}
	return base + "." + format
}

func (s *Service) retrievalSpans(ctx context.Context, versionID, content string) ([]RetrievalSpan, string, error) {
	rows, err := s.store.db.QueryContext(ctx, `
SELECT rs.source_id, rsp.span_id, rsp.source_version_id, rsp.selector_kind,
       rsp.selector_json, rsp.text_hash, rsp.chunk_hash, rsp.token_count
FROM retrieval_sources rs
JOIN retrieval_spans rsp ON rsp.source_id = rs.source_id
WHERE rsp.source_version_id = ? AND rs.state = 'active'
ORDER BY rsp.created_at ASC`, versionID)
	if err != nil {
		return nil, "", fmt.Errorf("platform bundle: query retrieval spans: %w", err)
	}
	defer rows.Close()
	spans := []RetrievalSpan{}
	sourceID := ""
	for rows.Next() {
		var span RetrievalSpan
		var selector string
		if err := rows.Scan(&span.SourceID, &span.ID, &span.SourceVersionID, &span.SelectorKind, &selector, &span.TextHash, &span.ChunkHash, &span.TokenCount); err != nil {
			return nil, "", fmt.Errorf("platform bundle: scan retrieval span: %w", err)
		}
		span.Selector = json.RawMessage(selector)
		span.Snippet = snippet(content, "")
		if sourceID == "" {
			sourceID = span.SourceID
		}
		spans = append(spans, span)
	}
	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("platform bundle: iterate retrieval spans: %w", err)
	}
	return spans, sourceID, nil
}

func (s *Service) citationEdges(ctx context.Context, versionID, sourceRevisionHash string) ([]CitationEdge, error) {
	rows, err := s.store.db.QueryContext(ctx, `
SELECT citation_id, from_kind, from_id, from_selector_json, to_kind, to_id,
       to_selector_json, relation_type, state, proposed_by, accepted_by,
       evidence_ref, confidence
FROM citation_edges
WHERE from_id = ?
ORDER BY created_at ASC`, versionID)
	if err != nil {
		return nil, fmt.Errorf("platform bundle: query citation edges: %w", err)
	}
	defer rows.Close()
	edges := []CitationEdge{}
	for rows.Next() {
		var edge CitationEdge
		var fromSelector, toSelector string
		if err := rows.Scan(&edge.ID, &edge.FromKind, &edge.FromID, &fromSelector, &edge.ToKind, &edge.ToID, &toSelector, &edge.RelationType, &edge.State, &edge.ProposedBy, &edge.AcceptedBy, &edge.EvidenceRef, &edge.Confidence); err != nil {
			return nil, fmt.Errorf("platform bundle: scan citation edge: %w", err)
		}
		edge.FromSelector = json.RawMessage(firstNonEmpty(fromSelector, "{}"))
		edge.ToSelector = json.RawMessage(firstNonEmpty(toSelector, "{}"))
		if edge.ToKind == "private_vtext_revision" {
			edge.ToKind = "source_revision_hash"
			edge.ToID = sourceRevisionHash
			if strings.HasPrefix(edge.EvidenceRef, "source_revision_hash:") {
				edge.ToID = strings.TrimPrefix(edge.EvidenceRef, "source_revision_hash:")
			}
		}
		edges = append(edges, edge)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("platform bundle: iterate citation edges: %w", err)
	}
	return edges, nil
}

func (s *Service) publicationSourceEntities(ctx context.Context, versionID string) ([]PublicationSourceEntity, error) {
	rows, err := s.store.db.QueryContext(ctx, `
SELECT entity_record_id, source_entity_id, kind, target_kind, target_id,
       display_policy, open_surface, entity_json
FROM publication_source_entities
WHERE publication_version_id = ?
ORDER BY created_at ASC`, versionID)
	if err != nil {
		return nil, fmt.Errorf("platform bundle: query source entities: %w", err)
	}
	defer rows.Close()
	entities := []PublicationSourceEntity{}
	for rows.Next() {
		var entity PublicationSourceEntity
		var raw string
		if err := rows.Scan(&entity.ID, &entity.SourceEntityID, &entity.Kind, &entity.TargetKind, &entity.TargetID, &entity.DisplayPolicy, &entity.OpenSurface, &raw); err != nil {
			return nil, fmt.Errorf("platform bundle: scan source entity: %w", err)
		}
		entity.Entity = json.RawMessage(firstNonEmpty(raw, "{}"))
		entities = append(entities, entity)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("platform bundle: iterate source entities: %w", err)
	}
	return entities, nil
}

func (s *Service) publicationTransclusions(ctx context.Context, versionID string) ([]PublicationTransclusion, error) {
	rows, err := s.store.db.QueryContext(ctx, `
SELECT transclusion_id, source_entity_id, host_selector_json, source_selector_json,
       relation_type, default_display_mode, snapshot_text, content_hash,
       access_policy_json, export_policy_json
FROM publication_transclusions
WHERE publication_version_id = ?
ORDER BY created_at ASC`, versionID)
	if err != nil {
		return nil, fmt.Errorf("platform bundle: query transclusions: %w", err)
	}
	defer rows.Close()
	transclusions := []PublicationTransclusion{}
	for rows.Next() {
		var transclusion PublicationTransclusion
		var hostSelector, sourceSelector, accessPolicy, exportPolicy string
		if err := rows.Scan(&transclusion.ID, &transclusion.SourceEntityID, &hostSelector, &sourceSelector, &transclusion.RelationType, &transclusion.DefaultDisplayMode, &transclusion.SnapshotText, &transclusion.ContentHash, &accessPolicy, &exportPolicy); err != nil {
			return nil, fmt.Errorf("platform bundle: scan transclusion: %w", err)
		}
		transclusion.HostSelector = json.RawMessage(firstNonEmpty(hostSelector, "{}"))
		transclusion.SourceSelector = json.RawMessage(firstNonEmpty(sourceSelector, "{}"))
		transclusion.AccessPolicy = json.RawMessage(firstNonEmpty(accessPolicy, "{}"))
		transclusion.ExportPolicy = json.RawMessage(firstNonEmpty(exportPolicy, "{}"))
		transclusions = append(transclusions, transclusion)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("platform bundle: iterate transclusions: %w", err)
	}
	return transclusions, nil
}

func (s *Service) publicationPolicy(ctx context.Context, versionID string) (PublicationPolicy, error) {
	var accessPolicy, exportPolicy string
	err := s.store.db.QueryRowContext(ctx, `
SELECT access_policy_json, export_policy_json
FROM publication_policies
WHERE publication_version_id = ?
ORDER BY created_at DESC
LIMIT 1`, versionID).Scan(&accessPolicy, &exportPolicy)
	if err != nil {
		if err == sql.ErrNoRows {
			return PublicationPolicy{Access: defaultPublicationAccessPolicy(), Export: defaultPublicationExportPolicy()}, nil
		}
		return PublicationPolicy{}, fmt.Errorf("platform bundle: query publication policy: %w", err)
	}
	return PublicationPolicy{
		Access: json.RawMessage(firstNonEmpty(accessPolicy, "{}")),
		Export: json.RawMessage(firstNonEmpty(exportPolicy, "{}")),
	}, nil
}

func (s *Service) provenanceSummary(ctx context.Context, versionID string) (PublicationProvenanceSummary, error) {
	var out PublicationProvenanceSummary
	if ids, err := s.idsForTarget(ctx, "consent_records", "consent_id", "target_id", versionID); err != nil {
		return out, err
	} else {
		out.ConsentIDs = ids
	}
	if ids, err := s.idsForTarget(ctx, "review_records", "review_id", "target_id", versionID); err != nil {
		return out, err
	} else {
		out.ReviewIDs = ids
	}
	if ids, err := s.idsForTarget(ctx, "verifier_attestations", "attestation_id", "target_id", versionID); err != nil {
		return out, err
	} else {
		out.AttestationIDs = ids
	}
	return out, nil
}

func (s *Service) idsForTarget(ctx context.Context, table, idColumn, targetColumn, targetID string) ([]string, error) {
	rows, err := s.store.db.QueryContext(ctx, fmt.Sprintf("SELECT %s FROM %s WHERE %s = ? ORDER BY created_at ASC", idColumn, table, targetColumn), targetID)
	if err != nil {
		return nil, fmt.Errorf("platform bundle: query %s: %w", table, err)
	}
	defer rows.Close()
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("platform bundle: scan %s: %w", table, err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("platform bundle: iterate %s: %w", table, err)
	}
	return ids, nil
}

func renderBlocks(content string, spans []RetrievalSpan) []RenderBlock {
	blocks := []RenderBlock{}
	lines := strings.SplitAfter(content, "\n")
	var paragraph strings.Builder
	start := 0
	cursor := 0
	flush := func(end int) {
		text := strings.TrimSpace(paragraph.String())
		if text == "" {
			paragraph.Reset()
			return
		}
		kind := "paragraph"
		if strings.HasPrefix(text, "#") {
			kind = "heading"
		} else if strings.HasPrefix(text, "- ") || strings.HasPrefix(text, "* ") {
			kind = "list"
		}
		spanID := ""
		textHash := ""
		if len(spans) > 0 {
			spanID = spans[0].ID
			textHash = sha256Hex([]byte(text))
		}
		blocks = append(blocks, RenderBlock{
			ID:       fmt.Sprintf("block-%d", len(blocks)+1),
			Kind:     kind,
			Text:     text,
			Start:    start,
			End:      end,
			SpanID:   spanID,
			TextHash: textHash,
		})
		paragraph.Reset()
	}
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			flush(cursor)
			cursor += len([]rune(line))
			start = cursor
			continue
		}
		if paragraph.Len() == 0 {
			start = cursor
		}
		paragraph.WriteString(line)
		cursor += len([]rune(line))
	}
	flush(cursor)
	if len(blocks) == 0 && strings.TrimSpace(content) != "" {
		blocks = append(blocks, RenderBlock{
			ID:       "block-1",
			Kind:     "paragraph",
			Text:     strings.TrimSpace(content),
			Start:    0,
			End:      len([]rune(content)),
			TextHash: sha256Hex([]byte(strings.TrimSpace(content))),
		})
	}
	return blocks
}

func snippet(content, query string) string {
	content = strings.TrimSpace(content)
	if len([]rune(content)) <= 260 {
		return content
	}
	lowerContent := strings.ToLower(content)
	lowerQuery := strings.ToLower(strings.TrimSpace(query))
	start := 0
	if lowerQuery != "" {
		if idx := strings.Index(lowerContent, lowerQuery); idx >= 0 {
			start = idx - 90
			if start < 0 {
				start = 0
			}
		}
	}
	runes := []rune(content)
	if start > len(runes) {
		start = 0
	}
	end := start + 240
	if end > len(runes) {
		end = len(runes)
	}
	prefix := ""
	suffix := ""
	if start > 0 {
		prefix = "..."
	}
	if end < len(runes) {
		suffix = "..."
	}
	return prefix + strings.TrimSpace(string(runes[start:end])) + suffix
}
