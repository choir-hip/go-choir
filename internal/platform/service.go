package platform

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/google/uuid"
)

const (
	publicVTextPrefix = "/pub/vtext/"
	textMediaType     = "text/plain; charset=utf-8"
)

type Service struct {
	store         *Store
	artifactsRoot string
	writeMu       sync.Mutex
}

type citationInput struct {
	ID       string          `json:"id"`
	URL      string          `json:"url"`
	URI      string          `json:"uri"`
	Ref      string          `json:"ref"`
	Title    string          `json:"title"`
	Selector json.RawMessage `json:"selector"`
	State    string          `json:"state"`
}

func NewService(store *Store, artifactsRoot string) *Service {
	return &Service{
		store:         store,
		artifactsRoot: filepath.Clean(artifactsRoot),
	}
}

func (s *Service) Health(ctx context.Context) error {
	if s == nil || s.store == nil {
		return fmt.Errorf("platform service unavailable")
	}
	return s.store.Ping(ctx)
}

func (s *Service) PublishVText(ctx context.Context, req PublishVTextRequest) (*PublishVTextResponse, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("platform service unavailable")
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	req.OwnerID = strings.TrimSpace(req.OwnerID)
	req.SourceDocID = strings.TrimSpace(req.SourceDocID)
	req.SourceRevisionID = strings.TrimSpace(req.SourceRevisionID)
	req.Title = strings.TrimSpace(req.Title)
	req.RequestedBy = strings.TrimSpace(req.RequestedBy)
	req.SourceTraceID = strings.TrimSpace(req.SourceTraceID)
	if req.OwnerID == "" {
		return nil, fmt.Errorf("owner_id is required")
	}
	if req.RequestedBy == "" {
		req.RequestedBy = req.OwnerID
	}
	if req.SourceDocID == "" {
		return nil, fmt.Errorf("source_doc_id is required")
	}
	if req.SourceRevisionID == "" {
		return nil, fmt.Errorf("source_revision_id is required")
	}
	if req.Title == "" {
		req.Title = "Untitled VText"
	}
	if req.Content == "" {
		return nil, fmt.Errorf("content is required")
	}
	if req.Citations == nil {
		req.Citations = json.RawMessage("[]")
	}
	if !json.Valid(req.Citations) {
		return nil, fmt.Errorf("citations must be valid JSON")
	}
	if req.Metadata == nil {
		req.Metadata = json.RawMessage("{}")
	}
	sourceMetadata, err := buildPublicationSourceMetadata(req)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	publicationID := id("pub")
	proposalID := id("pubprop")
	versionID := id("pubver")
	manifestID := id("artman")
	blobID := id("blob")
	routeID := id("route")
	sourceID := id("source")
	spanID := id("span")
	retrievalManifestID := id("retman")
	consentID := id("consent")
	reviewID := id("review")
	rollbackID := id("rollback")
	activityID := id("activity")
	publicEntityID := id("entity")
	privateEntityID := id("entity")
	agentID := id("agent")
	provEdgeID := id("edge")
	attestationID := id("att")

	contentHash := sha256Hex([]byte(req.Content))
	projectionHash := contentHash
	sourceRevisionHash := sha256Hex([]byte(req.SourceDocID + "\n" + req.SourceRevisionID + "\n" + req.Content + "\n" + string(req.Citations) + "\n" + string(req.Metadata)))
	routePath := publicVTextPrefix + slugify(firstNonEmpty(req.Slug, req.Title)) + "-" + shortID(publicationID)
	storageRef := filepath.Join("sha256", contentHash+".txt")
	if err := s.writeBlob(storageRef, []byte(req.Content)); err != nil {
		return nil, err
	}

	manifest := map[string]any{
		"schema":                 "choir.platform.artifact_manifest.v0",
		"subject_kind":           "publication_version",
		"subject_id":             versionID,
		"media_type":             textMediaType,
		"content_hash":           contentHash,
		"source_revision_hash":   sourceRevisionHash,
		"projection_hash":        projectionHash,
		"storage_ref":            storageRef,
		"byte_size":              len([]byte(req.Content)),
		"publication_id":         publicationID,
		"publication_version_id": versionID,
		"route_path":             routePath,
		"source_metadata_hash":   sourceMetadata.MetadataHash,
		"source_entities":        sourceMetadata.SourceEntities,
		"transclusions":          sourceMetadata.Transclusions,
		"access_policy":          json.RawMessage(sourceMetadata.AccessPolicy),
		"export_policy":          json.RawMessage(sourceMetadata.ExportPolicy),
	}
	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		return nil, fmt.Errorf("platform publish: marshal artifact manifest: %w", err)
	}
	manifestHash := sha256Hex(manifestJSON)

	wholeSelector := `{"type":"TextPositionSelector","start":0,"end":` + fmt.Sprintf("%d", len([]rune(req.Content))) + `}`
	retrievalSelectedRefs := mustJSON([]map[string]string{{
		"source_id":  sourceID,
		"span_id":    spanID,
		"version_id": versionID,
	}})
	publicURI := "choir:" + strings.TrimPrefix(routePath, "/")

	citationIDs := []string{}
	sourceCitationID := id("cite")
	citationIDs = append(citationIDs, sourceCitationID)
	externalCitations := parseCitationInputs(req.Citations)

	tx, err := s.store.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("platform publish: begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if err := execAll(ctx, tx, []statement{
		{"INSERT INTO platform_subjects (subject_id, subject_kind, display_name, canonical_uri, created_at, updated_at) VALUES (?, 'user', ?, '', ?, ?) ON DUPLICATE KEY UPDATE updated_at=VALUES(updated_at)", []any{req.OwnerID, req.OwnerID, now, now}},
		{"INSERT INTO publication_proposals (proposal_id, owner_id, source_doc_id, source_revision_id, source_revision_hash, projection_hash, title, state, created_by, created_trace_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, 'published', ?, ?, ?, ?)", []any{proposalID, req.OwnerID, req.SourceDocID, req.SourceRevisionID, sourceRevisionHash, projectionHash, req.Title, req.RequestedBy, req.SourceTraceID, now, now}},
		{"INSERT INTO publications (publication_id, owner_id, slug, title, state, latest_version_id, created_at, updated_at) VALUES (?, ?, ?, ?, 'published', ?, ?, ?)", []any{publicationID, req.OwnerID, strings.TrimPrefix(routePath, publicVTextPrefix), req.Title, versionID, now, now}},
		{"INSERT INTO publication_versions (publication_version_id, publication_id, proposal_id, source_doc_id, source_revision_id, source_revision_hash, projection_hash, content_hash, artifact_manifest_id, published_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)", []any{versionID, publicationID, proposalID, req.SourceDocID, req.SourceRevisionID, sourceRevisionHash, projectionHash, contentHash, manifestID, now}},
		{"INSERT INTO public_routes (route_id, route_path, target_kind, target_id, target_version_id, state, created_at, updated_at) VALUES (?, ?, 'publication', ?, ?, 'active', ?, ?)", []any{routeID, routePath, publicationID, versionID, now, now}},
		{"INSERT INTO artifact_manifests (artifact_manifest_id, subject_kind, subject_id, media_type, manifest_hash, manifest_json, created_at) VALUES (?, 'publication_version', ?, ?, ?, ?, ?)", []any{manifestID, versionID, textMediaType, manifestHash, string(manifestJSON), now}},
		{"INSERT INTO artifact_blobs (blob_id, artifact_manifest_id, content_hash, hash_algorithm, media_type, byte_size, storage_ref, created_at) VALUES (?, ?, ?, 'sha256', ?, ?, ?, ?)", []any{blobID, manifestID, contentHash, textMediaType, len([]byte(req.Content)), storageRef, now}},
		{"INSERT INTO consent_records (consent_id, subject_id, target_kind, target_id, action, state, evidence_ref, created_at) VALUES (?, ?, 'publication_version', ?, 'publish', 'granted', ?, ?)", []any{consentID, req.OwnerID, versionID, "requested_by:" + req.RequestedBy, now}},
		{"INSERT INTO review_records (review_id, target_kind, target_id, reviewer_subject_id, decision, body, created_at) VALUES (?, 'publication_version', ?, ?, 'approve', 'v0 owner consent publication path', ?)", []any{reviewID, versionID, req.RequestedBy, now}},
		{"INSERT INTO retrieval_sources (source_id, source_kind, canonical_uri, content_hash, visibility, state, created_at) VALUES (?, 'publication_version', ?, ?, 'public', 'active', ?)", []any{sourceID, publicURI, contentHash, now}},
		{"INSERT INTO retrieval_spans (span_id, source_id, source_version_id, selector_kind, selector_json, text_hash, chunk_hash, token_count, metadata_json, created_at) VALUES (?, ?, ?, 'text_position', ?, ?, ?, ?, ?, ?)", []any{spanID, sourceID, versionID, wholeSelector, contentHash, contentHash, len(strings.Fields(req.Content)), `{"scope":"whole_document"}`, now}},
		{"INSERT INTO retrieval_manifests (retrieval_manifest_id, output_kind, output_id, query_or_objective_hash, index_manifest_id, selected_refs_json, created_at) VALUES (?, 'publication_version', ?, ?, ?, ?, ?)", []any{retrievalManifestID, versionID, sha256Hex([]byte("publish:" + versionID)), manifestID, retrievalSelectedRefs, now}},
		{"INSERT INTO citation_edges (citation_id, from_kind, from_id, from_selector_json, to_kind, to_id, to_selector_json, relation_type, state, proposed_by, accepted_by, evidence_ref, confidence, created_at, updated_at) VALUES (?, 'publication_version', ?, ?, 'private_vtext_revision', ?, ?, 'is_version_of', 'accepted', ?, ?, ?, 1, ?, ?)", []any{sourceCitationID, versionID, wholeSelector, req.SourceRevisionID, wholeSelector, req.RequestedBy, req.RequestedBy, "source_revision_hash:" + sourceRevisionHash, now, now}},
		{"INSERT INTO publication_policies (policy_id, publication_version_id, access_policy_json, export_policy_json, created_at) VALUES (?, ?, ?, ?, ?)", []any{id("policy"), versionID, string(sourceMetadata.AccessPolicy), string(sourceMetadata.ExportPolicy), now}},
		{"INSERT INTO provenance_entities (entity_id, entity_kind, content_hash, canonical_uri, metadata_json, created_at) VALUES (?, 'private_vtext_revision', ?, ?, ?, ?)", []any{privateEntityID, sourceRevisionHash, "choir-private:vtext/" + req.SourceDocID + "/revisions/" + req.SourceRevisionID, `{"visibility":"private","projection":"hash_only"}`, now}},
		{"INSERT INTO provenance_entities (entity_id, entity_kind, content_hash, canonical_uri, metadata_json, created_at) VALUES (?, 'publication_version', ?, ?, ?, ?)", []any{publicEntityID, contentHash, publicURI, string(manifestJSON), now}},
		{"INSERT INTO provenance_agents (agent_ref_id, agent_kind, subject_id, metadata_json, created_at) VALUES (?, 'user', ?, ?, ?)", []any{agentID, req.RequestedBy, `{"authority":"owner_publish_v0"}`, now}},
		{"INSERT INTO provenance_activities (activity_id, activity_kind, trace_id, started_at, ended_at, metadata_json) VALUES (?, 'publish_vtext_revision', ?, ?, ?, ?)", []any{activityID, req.SourceTraceID, now, now, mustJSON(map[string]string{"proposal_id": proposalID, "route_path": routePath})}},
		{"INSERT INTO provenance_edges (edge_id, edge_kind, from_id, to_id, activity_id, metadata_json, created_at) VALUES (?, 'wasDerivedFrom', ?, ?, ?, ?, ?)", []any{provEdgeID, publicEntityID, privateEntityID, activityID, `{"source_private_content":"not_copied_as_private_ref"}`, now}},
		{"INSERT INTO verifier_attestations (attestation_id, target_kind, target_id, verifier_id, verifier_kind, result, subject_digest, predicate_type, evidence_json, created_at) VALUES (?, 'publication_version', ?, 'platformd', 'service', 'passed', ?, 'choir.platform.publish_vtext.v0', ?, ?)", []any{attestationID, versionID, contentHash, mustJSON(map[string]string{"route_path": routePath, "source_revision_hash": sourceRevisionHash}), now}},
		{"INSERT INTO rollback_refs (rollback_id, target_kind, target_id, rollback_kind, ref, created_at) VALUES (?, 'public_route', ?, 'disable_route', ?, ?)", []any{rollbackID, routeID, "UPDATE public_routes SET state='disabled' WHERE route_id='" + routeID + "'", now}},
	}); err != nil {
		return nil, err
	}

	for i, sourceEntity := range sourceMetadata.SourceEntities {
		if _, err := tx.ExecContext(ctx, `INSERT INTO publication_source_entities (entity_record_id, publication_version_id, source_entity_id, kind, target_kind, target_id, display_policy, open_surface, entity_json, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			id("pubsrc"), versionID, sourceEntity.SourceEntityID, sourceEntity.Kind, sourceEntity.TargetKind, sourceEntity.TargetID, sourceEntity.DisplayPolicy, sourceEntity.OpenSurface, string(sourceEntity.EntityJSON), now); err != nil {
			return nil, fmt.Errorf("platform publish: insert source entity %d: %w", i, err)
		}
	}
	for i, transclusion := range sourceMetadata.Transclusions {
		if _, err := tx.ExecContext(ctx, `INSERT INTO publication_transclusions (transclusion_id, publication_version_id, source_entity_id, host_selector_json, source_selector_json, relation_type, default_display_mode, snapshot_text, content_hash, access_policy_json, export_policy_json, entity_json, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			id("trans"), versionID, transclusion.SourceEntityID, string(transclusion.HostSelector), string(transclusion.SourceSelector), transclusion.RelationType, transclusion.DefaultDisplayMode, transclusion.SnapshotText, transclusion.ContentHash, string(sourceMetadata.AccessPolicy), string(sourceMetadata.ExportPolicy), string(transclusion.EntityJSON), now); err != nil {
			return nil, fmt.Errorf("platform publish: insert transclusion %d: %w", i, err)
		}
	}

	for _, citation := range externalCitations {
		toID, ok := publicCitationTarget(citation)
		if !ok {
			continue
		}
		citationID := id("cite")
		citationIDs = append(citationIDs, citationID)
		state := strings.TrimSpace(citation.State)
		switch state {
		case "", "candidate":
			state = "candidate"
		case "accepted", "asserted", "rejected", "disputed", "retracted":
		default:
			state = "candidate"
		}
		selector := "{}"
		if len(citation.Selector) > 0 && json.Valid(citation.Selector) {
			selector = string(citation.Selector)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO citation_edges (citation_id, from_kind, from_id, from_selector_json, to_kind, to_id, to_selector_json, relation_type, state, proposed_by, evidence_ref, confidence, created_at, updated_at) VALUES (?, 'publication_version', ?, ?, 'external_reference', ?, ?, 'references', ?, ?, ?, 0.5, ?, ?)`,
			citationID, versionID, wholeSelector, toID, selector, state, req.RequestedBy, firstNonEmpty(citation.Title, toID), now, now); err != nil {
			return nil, fmt.Errorf("platform publish: insert external citation edge: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("platform publish: commit transaction: %w", err)
	}
	if err := s.store.commitDolt(ctx, "publish vtext revision "+req.SourceRevisionID); err != nil {
		return nil, err
	}

	return &PublishVTextResponse{
		PublicationID:        publicationID,
		ProposalID:           proposalID,
		PublicationVersionID: versionID,
		ArtifactManifestID:   manifestID,
		ContentHash:          contentHash,
		SourceRevisionHash:   sourceRevisionHash,
		ProjectionHash:       projectionHash,
		RoutePath:            routePath,
		RetrievalSourceID:    sourceID,
		RetrievalSpanIDs:     []string{spanID},
		CitationIDs:          citationIDs,
		ConsentID:            consentID,
		ReviewID:             reviewID,
		RollbackID:           rollbackID,
		State:                "published",
	}, nil
}

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
	sourceEntities, err := s.publicationSourceEntities(ctx, rec.PublicationVersionID)
	if err != nil {
		return nil, err
	}
	transclusions, err := s.publicationTransclusions(ctx, rec.PublicationVersionID)
	if err != nil {
		return nil, err
	}
	policy, err := s.publicationPolicy(ctx, rec.PublicationVersionID)
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
       rs.source_id, rsp.span_id
FROM publications p
JOIN publication_versions pv ON pv.publication_version_id = p.latest_version_id
JOIN public_routes pr ON pr.target_id = p.publication_id AND pr.target_version_id = pv.publication_version_id
JOIN artifact_blobs ab ON ab.artifact_manifest_id = pv.artifact_manifest_id
JOIN retrieval_sources rs ON rs.content_hash = pv.content_hash AND rs.state = 'active'
JOIN retrieval_spans rsp ON rsp.source_id = rs.source_id AND rsp.source_version_id = pv.publication_version_id
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
		}
		if err := rows.Scan(&rec.PublicationID, &rec.Title, &rec.RoutePath, &rec.PublicationVersionID,
			&rec.ContentHash, &rec.SourceRevisionHash, &rec.StorageRef, &rec.SourceID, &rec.SpanID); err != nil {
			return nil, fmt.Errorf("platform retrieval: scan source: %w", err)
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

func (s *Service) SubmitPublicationProposal(ctx context.Context, req SubmitPublicationProposalRequest) (*SubmitPublicationProposalResponse, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("platform service unavailable")
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	req.PublicationID = strings.TrimSpace(req.PublicationID)
	req.PublicationVersionID = strings.TrimSpace(req.PublicationVersionID)
	req.SubmitterID = strings.TrimSpace(req.SubmitterID)
	req.SubmitterDocID = strings.TrimSpace(req.SubmitterDocID)
	req.SubmitterRevisionID = strings.TrimSpace(req.SubmitterRevisionID)
	req.Title = strings.TrimSpace(req.Title)
	req.RequestedBy = strings.TrimSpace(req.RequestedBy)
	if req.PublicationID == "" {
		return nil, fmt.Errorf("publication_id is required")
	}
	if req.SubmitterID == "" {
		return nil, fmt.Errorf("submitter_id is required")
	}
	if req.RequestedBy == "" {
		req.RequestedBy = req.SubmitterID
	}
	if req.RequestedBy != req.SubmitterID {
		return nil, fmt.Errorf("requested_by must match submitter_id")
	}
	if req.SubmitterDocID == "" {
		return nil, fmt.Errorf("submitter_doc_id is required")
	}
	if req.SubmitterRevisionID == "" {
		return nil, fmt.Errorf("submitter_revision_id is required")
	}
	if req.Content == "" {
		return nil, fmt.Errorf("content is required")
	}
	if req.Title == "" {
		req.Title = "VText proposal"
	}
	if req.Citations == nil {
		req.Citations = json.RawMessage("[]")
	}
	if !json.Valid(req.Citations) {
		return nil, fmt.Errorf("citations must be valid JSON")
	}

	var sourceOwnerID, latestVersionID string
	err := s.store.db.QueryRowContext(ctx, `SELECT owner_id, latest_version_id FROM publications WHERE publication_id = ? AND state = 'published'`, req.PublicationID).Scan(&sourceOwnerID, &latestVersionID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("publication not found")
		}
		return nil, fmt.Errorf("platform proposal: query publication: %w", err)
	}
	if req.PublicationVersionID == "" {
		req.PublicationVersionID = latestVersionID
	}
	var versionContentHash string
	if err := s.store.db.QueryRowContext(ctx, `SELECT content_hash FROM publication_versions WHERE publication_id = ? AND publication_version_id = ?`, req.PublicationID, req.PublicationVersionID).Scan(&versionContentHash); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("publication version not found")
		}
		return nil, fmt.Errorf("platform proposal: query publication version: %w", err)
	}

	now := time.Now().UTC()
	proposalID := id("readerprop")
	manifestID := id("artman")
	blobID := id("blob")
	deliveryID := id("delivery")
	attestationID := id("att")
	activityID := id("activity")
	sourceEntityID := id("entity")
	proposalEntityID := id("entity")
	agentID := id("agent")
	provEdgeID := id("edge")
	contentHash := sha256Hex([]byte(req.Content))
	transclusionsJSON, err := json.Marshal(req.Transclusions)
	if err != nil {
		return nil, fmt.Errorf("platform proposal: marshal transclusions: %w", err)
	}
	proposalRevisionHash := sha256Hex([]byte(req.SubmitterDocID + "\n" + req.SubmitterRevisionID + "\n" + req.Content + "\n" + string(transclusionsJSON) + "\n" + string(req.Citations)))
	projectionHash := contentHash
	storageRef := filepath.Join("sha256", "proposals", contentHash+".txt")
	if err := s.writeBlob(storageRef, []byte(req.Content)); err != nil {
		return nil, err
	}
	manifest := map[string]any{
		"schema":                        "choir.platform.proposal_artifact_manifest.v0",
		"subject_kind":                  "publication_version_proposal",
		"subject_id":                    proposalID,
		"media_type":                    textMediaType,
		"content_hash":                  contentHash,
		"proposal_revision_hash":        proposalRevisionHash,
		"projection_hash":               projectionHash,
		"storage_ref":                   storageRef,
		"byte_size":                     len([]byte(req.Content)),
		"source_publication_id":         req.PublicationID,
		"source_publication_version_id": req.PublicationVersionID,
		"submitter_doc_id":              req.SubmitterDocID,
		"submitter_revision_id":         req.SubmitterRevisionID,
	}
	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		return nil, fmt.Errorf("platform proposal: marshal artifact manifest: %w", err)
	}
	manifestHash := sha256Hex(manifestJSON)

	tx, err := s.store.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("platform proposal: begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := execAll(ctx, tx, []statement{
		{"INSERT INTO platform_subjects (subject_id, subject_kind, display_name, canonical_uri, created_at, updated_at) VALUES (?, 'user', ?, '', ?, ?) ON DUPLICATE KEY UPDATE updated_at=VALUES(updated_at)", []any{req.SubmitterID, req.SubmitterID, now, now}},
		{"INSERT INTO artifact_manifests (artifact_manifest_id, subject_kind, subject_id, media_type, manifest_hash, manifest_json, created_at) VALUES (?, 'publication_version_proposal', ?, ?, ?, ?, ?)", []any{manifestID, proposalID, textMediaType, manifestHash, string(manifestJSON), now}},
		{"INSERT INTO artifact_blobs (blob_id, artifact_manifest_id, content_hash, hash_algorithm, media_type, byte_size, storage_ref, created_at) VALUES (?, ?, ?, 'sha256', ?, ?, ?, ?)", []any{blobID, manifestID, contentHash, textMediaType, len([]byte(req.Content)), storageRef, now}},
		{"INSERT INTO publication_version_proposals (proposal_id, publication_id, publication_version_id, source_owner_id, submitter_id, submitter_doc_id, submitter_revision_id, submitter_revision_hash, content_hash, projection_hash, artifact_manifest_id, title, transclusions_json, citations_json, state, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'proposed', ?, ?)", []any{proposalID, req.PublicationID, req.PublicationVersionID, sourceOwnerID, req.SubmitterID, req.SubmitterDocID, req.SubmitterRevisionID, proposalRevisionHash, contentHash, projectionHash, manifestID, req.Title, string(transclusionsJSON), string(req.Citations), now, now}},
		{"INSERT INTO proposal_delivery_records (delivery_id, proposal_id, target_owner_id, target_kind, target_id, delivery_state, delivery_ref, created_at, updated_at) VALUES (?, ?, ?, 'publication_author', ?, 'recorded_for_author', ?, ?, ?)", []any{deliveryID, proposalID, sourceOwnerID, req.PublicationID, "platform-dolt:publication_version_proposals/" + proposalID, now, now}},
		{"INSERT INTO provenance_entities (entity_id, entity_kind, content_hash, canonical_uri, metadata_json, created_at) VALUES (?, 'publication_version', ?, ?, ?, ?)", []any{sourceEntityID, versionContentHash, "choir:publication/" + req.PublicationID + "/versions/" + req.PublicationVersionID, mustJSON(map[string]string{"publication_id": req.PublicationID, "publication_version_id": req.PublicationVersionID}), now}},
		{"INSERT INTO provenance_entities (entity_id, entity_kind, content_hash, canonical_uri, metadata_json, created_at) VALUES (?, 'publication_version_proposal', ?, ?, ?, ?)", []any{proposalEntityID, contentHash, "choir:publication/" + req.PublicationID + "/proposals/" + proposalID, string(manifestJSON), now}},
		{"INSERT INTO provenance_agents (agent_ref_id, agent_kind, subject_id, metadata_json, created_at) VALUES (?, 'user', ?, ?, ?)", []any{agentID, req.SubmitterID, `{"authority":"reader_derivative_proposal_v0"}`, now}},
		{"INSERT INTO provenance_activities (activity_id, activity_kind, trace_id, started_at, ended_at, metadata_json) VALUES (?, 'submit_publication_derivative_proposal', '', ?, ?, ?)", []any{activityID, now, now, mustJSON(map[string]string{"proposal_id": proposalID, "source_publication_id": req.PublicationID})}},
		{"INSERT INTO provenance_edges (edge_id, edge_kind, from_id, to_id, activity_id, metadata_json, created_at) VALUES (?, 'wasDerivedFrom', ?, ?, ?, ?, ?)", []any{provEdgeID, proposalEntityID, sourceEntityID, activityID, mustJSON(map[string]string{"delivery_id": deliveryID}), now}},
		{"INSERT INTO verifier_attestations (attestation_id, target_kind, target_id, verifier_id, verifier_kind, result, subject_digest, predicate_type, evidence_json, created_at) VALUES (?, 'publication_version_proposal', ?, 'platformd', 'service', 'passed', ?, 'choir.platform.reader_proposal.v0', ?, ?)", []any{attestationID, proposalID, contentHash, mustJSON(map[string]string{"source_publication_id": req.PublicationID, "source_publication_version_id": req.PublicationVersionID}), now}},
	}); err != nil {
		return nil, err
	}

	transclusionIDs := []string{}
	for _, ref := range req.Transclusions {
		citationID := id("cite")
		transclusionIDs = append(transclusionIDs, citationID)
		selector := "{}"
		if len(ref.Selector) > 0 && json.Valid(ref.Selector) {
			selector = string(ref.Selector)
		}
		toID := firstNonEmpty(ref.SpanID, ref.PublicationVersionID, req.PublicationVersionID)
		toKind := "published_vtext_span"
		if ref.SpanID == "" {
			toKind = "publication_version"
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO citation_edges (citation_id, from_kind, from_id, from_selector_json, to_kind, to_id, to_selector_json, relation_type, state, proposed_by, evidence_ref, confidence, created_at, updated_at) VALUES (?, 'publication_version_proposal', ?, '{}', ?, ?, ?, 'transcludes', 'proposed', ?, ?, 0.9, ?, ?)`,
			citationID, proposalID, toKind, toID, selector, req.SubmitterID, "source_content_hash:"+firstNonEmpty(ref.ContentHash, versionContentHash), now, now); err != nil {
			return nil, fmt.Errorf("platform proposal: insert transclusion edge: %w", err)
		}
	}

	citationIDs := []string{}
	for _, citation := range parseCitationInputs(req.Citations) {
		toID, ok := publicCitationTarget(citation)
		if !ok {
			continue
		}
		citationID := id("cite")
		citationIDs = append(citationIDs, citationID)
		if _, err := tx.ExecContext(ctx, `INSERT INTO citation_edges (citation_id, from_kind, from_id, from_selector_json, to_kind, to_id, to_selector_json, relation_type, state, proposed_by, evidence_ref, confidence, created_at, updated_at) VALUES (?, 'publication_version_proposal', ?, '{}', 'external_reference', ?, '{}', 'references', 'candidate', ?, ?, 0.5, ?, ?)`,
			citationID, proposalID, toID, req.SubmitterID, firstNonEmpty(citation.Title, toID), now, now); err != nil {
			return nil, fmt.Errorf("platform proposal: insert citation edge: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("platform proposal: commit transaction: %w", err)
	}
	if err := s.store.commitDolt(ctx, "propose vtext revision "+req.SubmitterRevisionID+" for publication "+req.PublicationID); err != nil {
		return nil, err
	}

	return &SubmitPublicationProposalResponse{
		ProposalID:           proposalID,
		PublicationID:        req.PublicationID,
		PublicationVersionID: req.PublicationVersionID,
		SourceOwnerID:        sourceOwnerID,
		SubmitterID:          req.SubmitterID,
		ContentHash:          contentHash,
		ProposalRevisionHash: proposalRevisionHash,
		ArtifactManifestID:   manifestID,
		TransclusionIDs:      transclusionIDs,
		CitationIDs:          citationIDs,
		DeliveryID:           deliveryID,
		DeliveryState:        "recorded_for_author",
		State:                "proposed",
	}, nil
}

func (s *Service) UpdateProposalDeliveryState(ctx context.Context, req UpdateProposalDeliveryStateRequest) (*UpdateProposalDeliveryStateResponse, error) {
	if s == nil || s.store == nil || s.store.db == nil {
		return nil, fmt.Errorf("platform service is not initialized")
	}
	req.ProposalID = strings.TrimSpace(req.ProposalID)
	req.DeliveryID = strings.TrimSpace(req.DeliveryID)
	req.DeliveryState = normalizeProposalDeliveryState(req.DeliveryState)
	req.DeliveryRef = strings.TrimSpace(req.DeliveryRef)
	if req.ProposalID == "" {
		return nil, fmt.Errorf("proposal_id is required")
	}
	if req.DeliveryID == "" {
		return nil, fmt.Errorf("delivery_id is required")
	}
	if req.DeliveryState == "" {
		return nil, fmt.Errorf("delivery_state is required")
	}
	if req.DeliveryRef == "" {
		req.DeliveryRef = "platform-dolt:proposal_delivery_records/" + req.DeliveryID
	}
	now := time.Now().UTC()
	res, err := s.store.db.ExecContext(ctx, `
UPDATE proposal_delivery_records
   SET delivery_state = ?, delivery_ref = ?, updated_at = ?
 WHERE proposal_id = ? AND delivery_id = ?`,
		req.DeliveryState, req.DeliveryRef, now, req.ProposalID, req.DeliveryID)
	if err != nil {
		return nil, fmt.Errorf("platform proposal delivery: update record: %w", err)
	}
	if rows, err := res.RowsAffected(); err == nil && rows == 0 {
		return nil, fmt.Errorf("proposal delivery not found")
	}
	if err := s.store.commitDolt(ctx, "record proposal delivery "+req.DeliveryState+" "+req.DeliveryID); err != nil {
		return nil, err
	}
	return &UpdateProposalDeliveryStateResponse{
		ProposalID:    req.ProposalID,
		DeliveryID:    req.DeliveryID,
		DeliveryState: req.DeliveryState,
	}, nil
}

func normalizeProposalDeliveryState(state string) string {
	switch strings.TrimSpace(strings.ToLower(state)) {
	case "recorded_for_author", "queued", "delivered", "failed":
		return strings.TrimSpace(strings.ToLower(state))
	default:
		return ""
	}
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

func formatPublicationExportContent(bundle *PublicationBundle, format string) string {
	if bundle == nil {
		return ""
	}
	content := bundle.Artifact.Content
	switch format {
	case "html":
		title := html.EscapeString(firstNonEmpty(bundle.Publication.Title, "Published VText"))
		body := strings.ReplaceAll(html.EscapeString(content), "\n", "<br>\n")
		return "<!doctype html>\n<html><head><meta charset=\"utf-8\"><title>" + title + "</title></head><body><article><h1>" + title + "</h1><p>" + body + "</p></article></body></html>\n"
	default:
		return content
	}
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

func (s *Service) writeBlob(storageRef string, data []byte) error {
	path, err := s.artifactPath(storageRef)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("platform artifact: create dir: %w", err)
	}
	tmp := path + ".tmp-" + shortID(id("write"))
	if err := os.WriteFile(tmp, data, 0o640); err != nil {
		return fmt.Errorf("platform artifact: write blob: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("platform artifact: install blob: %w", err)
	}
	return nil
}

func publicCitationTarget(c citationInput) (string, bool) {
	for _, candidate := range []string{c.URI, c.URL, c.Ref} {
		target := strings.TrimSpace(candidate)
		lower := strings.ToLower(target)
		if strings.HasPrefix(lower, "https://") ||
			strings.HasPrefix(lower, "http://") ||
			strings.HasPrefix(lower, "doi:") ||
			strings.HasPrefix(lower, "urn:") ||
			strings.HasPrefix(lower, "ipfs://") ||
			strings.HasPrefix(lower, "ar://") ||
			strings.HasPrefix(lower, "choir:pub/") {
			return target, true
		}
	}
	return "", false
}

func (s *Service) readBlob(storageRef string) ([]byte, error) {
	path, err := s.artifactPath(storageRef)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("platform artifact: read blob: %w", err)
	}
	return data, nil
}

func (s *Service) artifactPath(storageRef string) (string, error) {
	if s == nil {
		return "", fmt.Errorf("platform artifact: service unavailable")
	}
	root := filepath.Clean(s.artifactsRoot)
	if root == "" || root == "." {
		return "", fmt.Errorf("platform artifact: root is not configured")
	}
	cleaned := filepath.Clean(strings.TrimLeft(storageRef, string(filepath.Separator)))
	path := filepath.Join(root, cleaned)
	rel, err := filepath.Rel(root, path)
	if err != nil || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return "", fmt.Errorf("platform artifact: invalid storage ref")
	}
	return path, nil
}

type statement struct {
	query string
	args  []any
}

func execAll(ctx context.Context, tx *sql.Tx, stmts []statement) error {
	for _, stmt := range stmts {
		if _, err := tx.ExecContext(ctx, stmt.query, stmt.args...); err != nil {
			return fmt.Errorf("platform publish: exec %s: %w", summarizeSQL(stmt.query), err)
		}
	}
	return nil
}

func summarizeSQL(query string) string {
	fields := strings.Fields(query)
	if len(fields) > 5 {
		fields = fields[:5]
	}
	return strings.Join(fields, " ")
}

func id(prefix string) string {
	return prefix + "-" + uuid.NewString()
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func normalizePublicationRoutePath(routePath string) string {
	normalized := "/" + strings.TrimLeft(strings.TrimSpace(routePath), "/")
	if normalized != "/" && strings.HasPrefix(normalized, publicVTextPrefix) {
		normalized = strings.TrimRight(normalized, "/")
	}
	return normalized
}

func slugify(raw string) string {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" {
		return "vtext"
	}
	var b strings.Builder
	lastDash := false
	for _, r := range raw {
		switch {
		case unicode.IsLetter(r) || unicode.IsNumber(r):
			b.WriteRune(r)
			lastDash = false
		case unicode.IsSpace(r) || r == '-' || r == '_' || r == '.':
			if b.Len() > 0 && !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	slug := strings.Trim(b.String(), "-")
	if slug == "" {
		return "vtext"
	}
	if len(slug) > 96 {
		slug = strings.Trim(slug[:96], "-")
	}
	if slug == "" {
		return "vtext"
	}
	return slug
}

var idShortener = regexp.MustCompile(`[^a-zA-Z0-9]+`)

func shortID(value string) string {
	short := idShortener.ReplaceAllString(value, "")
	if len(short) > 12 {
		short = short[:12]
	}
	if short == "" {
		return "id"
	}
	return short
}

func mustJSON(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(data)
}

func parseCitationInputs(raw json.RawMessage) []citationInput {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var citations []citationInput
	if err := json.Unmarshal(raw, &citations); err == nil {
		return citations
	}
	var wrapper struct {
		Citations []citationInput `json:"citations"`
	}
	if err := json.Unmarshal(raw, &wrapper); err == nil {
		return wrapper.Citations
	}
	return nil
}
