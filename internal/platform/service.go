package platform

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
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
	sourceRevisionHash := sha256Hex([]byte(req.SourceDocID + "\n" + req.SourceRevisionID + "\n" + req.Content + "\n" + string(req.Citations)))
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

func (s *Service) GetPublishedPage(ctx context.Context, routePath string) (*PublishedPage, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("platform service unavailable")
	}
	routePath = "/" + strings.TrimLeft(strings.TrimSpace(routePath), "/")
	var rec publishedRouteRecord
	err := s.store.db.QueryRowContext(ctx, `
SELECT p.publication_id, pv.publication_version_id, p.title, pv.content_hash, pv.source_revision_hash, ab.storage_ref, pv.published_at
FROM public_routes pr
JOIN publications p ON p.publication_id = pr.target_id
JOIN publication_versions pv ON pv.publication_version_id = pr.target_version_id
JOIN artifact_blobs ab ON ab.artifact_manifest_id = pv.artifact_manifest_id
WHERE pr.route_path = ? AND pr.state = 'active' AND p.state = 'published'`, routePath).
		Scan(&rec.PublicationID, &rec.PublicationVersionID, &rec.Title, &rec.ContentHash, &rec.SourceRevisionHash, &rec.StorageRef, &rec.PublishedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, err
		}
		return nil, fmt.Errorf("platform page: query route: %w", err)
	}
	content, err := s.readBlob(rec.StorageRef)
	if err != nil {
		return nil, err
	}
	return &PublishedPage{
		PublicationID:        rec.PublicationID,
		PublicationVersionID: rec.PublicationVersionID,
		Title:                rec.Title,
		Content:              string(content),
		ContentHash:          rec.ContentHash,
		SourceRevisionHash:   rec.SourceRevisionHash,
		PublishedAt:          rec.PublishedAt,
	}, nil
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
