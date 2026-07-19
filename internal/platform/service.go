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
	publicTexturePrefix = "/pub/texture/"
	textMediaType       = "text/plain; charset=utf-8"

	defaultPublishedTextureTitle    = "Published Texture"
	defaultUntitledTextureTitle     = "Untitled Texture"
	defaultTextureProposalTitle     = "Texture proposal"
	defaultPublishedTextureSlugBase = "published-texture"
)

type Service struct {
	store         *Store
	artifactsRoot string
	signingKey    *SigningKey
	writeMu       sync.Mutex
	graphStore    *PublicationGraphStore // graph-native dual-write target
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

// NewService builds a platform Service. signingKeyPath is the platform Ed25519
// signing key (loaded or auto-created); an empty path disables D6 signing
// (signatures omitted, publish still works) for callers that opt out.
func NewService(store *Store, artifactsRoot, signingKeyPath string) *Service {
	svc := &Service{
		store:         store,
		artifactsRoot: filepath.Clean(artifactsRoot),
	}
	if signingKeyPath != "" {
		if key, err := LoadOrCreateSigningKey(signingKeyPath); err == nil {
			svc.signingKey = key
		}
	}
	// Wire graph-native publication store.
	ogStore := NewObjectGraphStore(store)
	svc.graphStore = NewPublicationGraphStore(ogStore)
	return svc
}

func (s *Service) Health(ctx context.Context) error {
	if s == nil || s.store == nil {
		return fmt.Errorf("platform service unavailable")
	}
	return s.store.Ping(ctx)
}

func (s *Service) SyncTextureDocument(ctx context.Context, req SyncTextureDocumentRequest) (*SyncTextureDocumentResponse, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("platform service unavailable")
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	req.DocID = strings.TrimSpace(req.DocID)
	req.OwnerID = strings.TrimSpace(req.OwnerID)
	req.Title = strings.TrimSpace(req.Title)
	if req.DocID == "" {
		return nil, fmt.Errorf("doc_id is required")
	}
	if req.OwnerID == "" {
		return nil, fmt.Errorf("owner_id is required")
	}

	if err := s.store.UpsertTextureDocument(ctx, req.DocID, req.OwnerID, req.Title); err != nil {
		return nil, fmt.Errorf("platform sync texture: upsert document: %w", err)
	}

	for _, rev := range req.Revisions {
		platformRev := PlatformTextureRevision{
			RevisionID:       strings.TrimSpace(rev.RevisionID),
			DocID:            req.DocID,
			OwnerID:          req.OwnerID,
			ParentRevisionID: strings.TrimSpace(rev.ParentRevisionID),
			AuthorKind:       strings.TrimSpace(rev.AuthorKind),
			AuthorLabel:      strings.TrimSpace(rev.AuthorLabel),
			Content:          rev.Content,
			BodyDoc:          rev.BodyDoc,
			SourceEntities:   rev.SourceEntities,
			Citations:        rev.Citations,
			Metadata:         rev.Metadata,
			CreatedAt:        rev.CreatedAt,
		}
		if platformRev.RevisionID == "" {
			continue
		}
		if err := s.store.UpsertTextureRevision(ctx, platformRev); err != nil {
			return nil, fmt.Errorf("platform sync texture: upsert revision %s: %w", platformRev.RevisionID, err)
		}
	}

	if err := s.store.commitDolt(ctx, "sync texture document "+req.DocID+" with "+fmt.Sprintf("%d", len(req.Revisions))+" revisions"); err != nil {
		return nil, err
	}

	return &SyncTextureDocumentResponse{
		DocID:         req.DocID,
		RevisionCount: len(req.Revisions),
	}, nil
}

func (s *Service) GetPlatformTextureDocument(ctx context.Context, docID string) (*PlatformTextureDocument, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("platform service unavailable")
	}
	return s.store.GetTextureDocument(ctx, docID)
}

func (s *Service) ListPlatformTextureRevisions(ctx context.Context, docID string) ([]PlatformTextureRevision, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("platform service unavailable")
	}
	return s.store.ListTextureRevisions(ctx, docID)
}

func (s *Service) GetPlatformTextureRevision(ctx context.Context, revisionID string) (*PlatformTextureRevision, error) {
	if s == nil || s.store == nil {
		return nil, fmt.Errorf("platform service unavailable")
	}
	return s.store.GetTextureRevision(ctx, revisionID)
}

func (s *Service) PublishTexture(ctx context.Context, req PublishTextureRequest) (*PublishTextureResponse, error) {
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
		req.Title = defaultUntitledTextureTitle
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
	if err := normalizePublishTextureStructuredInput(&req); err != nil {
		return nil, err
	}
	if req.Content == "" {
		return nil, fmt.Errorf("content is required")
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
	sourceID := id("source")
	spanID := id("span")
	retrievalManifestID := id("retman")
	consentID := id("consent")
	reviewID := id("review")
	activityID := id("activity")
	attestationID := id("att")

	contentHash := sha256Hex([]byte(req.Content))
	projectionHash := contentHash
	sourceRevisionHash := sha256Hex([]byte(req.SourceDocID + "\n" + req.SourceRevisionID + "\n" + req.Content + "\n" + string(req.BodyDoc) + "\n" + string(req.SourceEntities) + "\n" + string(req.Citations) + "\n" + string(req.Metadata)))
	routePath := publicTexturePrefix + slugify(firstNonEmpty(req.Slug, req.Title)) + "-" + shortID(publicationID)
	storageRef := filepath.Join("sha256", contentHash+".txt")
	if err := s.writeBlob(storageRef, []byte(req.Content)); err != nil {
		return nil, err
	}

	versionHistory, _, versionHistoryHash := buildVersionHistoryManifest(req.History, s.signingKey)

	manifest := map[string]any{
		"schema":                     "choir.platform.artifact_manifest.v0",
		"subject_kind":               "publication_version",
		"subject_id":                 versionID,
		"media_type":                 textMediaType,
		"content_hash":               contentHash,
		"source_revision_hash":       sourceRevisionHash,
		"projection_hash":            projectionHash,
		"storage_ref":                storageRef,
		"byte_size":                  len([]byte(req.Content)),
		"publication_id":             publicationID,
		"publication_version_id":     versionID,
		"route_path":                 routePath,
		"source_metadata_hash":       sourceMetadata.MetadataHash,
		"body_doc":                   json.RawMessage(req.BodyDoc),
		"structured_source_entities": json.RawMessage(req.SourceEntities),
		"source_entities":            sourceMetadata.SourceEntities,
		"transclusions":              sourceMetadata.Transclusions,
		"access_policy":              json.RawMessage(sourceMetadata.AccessPolicy),
		"export_policy":              json.RawMessage(sourceMetadata.ExportPolicy),
	}
	// A Texture is its full versioned history, not just the head projection.
	// Persist the canonical version-history manifest (chain + per-revision
	// provenance + transclusions, with the tamper-evident hash chain) inside the
	// artifact manifest so the published artifact is self-contained and the
	// reader can serve every revision. Omitted when no chain is supplied so the
	// head-only path is byte-identical to before.
	if versionHistoryHash != "" {
		manifest["version_history"] = versionHistory
		manifest["version_history_hash"] = versionHistoryHash
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
	externalCitations := parseCitationInputs(req.Citations)

	// Write to the object graph — single path, no relational tables.
	graphParams := PublishGraphParams{
		OwnerID:              req.OwnerID,
		RequestedBy:          req.RequestedBy,
		SourceDocID:          req.SourceDocID,
		SourceRevisionID:     req.SourceRevisionID,
		SourceRevisionHash:   sourceRevisionHash,
		SourceTraceID:        req.SourceTraceID,
		Title:                req.Title,
		Slug:                 strings.TrimPrefix(routePath, publicTexturePrefix),
		Content:              req.Content,
		ContentHash:          contentHash,
		ContentSize:          len([]byte(req.Content)),
		ProjectionHash:       projectionHash,
		WholeSelector:        wholeSelector,
		PublicURI:            publicURI,
		RoutePath:            routePath,
		StorageRef:           storageRef,
		ManifestJSON:         manifestJSON,
		ManifestHash:         manifestHash,
		TokenCount:           len(strings.Fields(req.Content)),
		SelectedRefsJSON:     json.RawMessage(retrievalSelectedRefs),
		PublicationID:        publicationID,
		ProposalID:           proposalID,
		PublicationVersionID: versionID,
		ArtifactManifestID:   manifestID,
		ConsentID:            consentID,
		ReviewID:             reviewID,
		RetrievalSourceID:    sourceID,
		RetrievalSpanID:      spanID,
		RetrievalManifestID:  retrievalManifestID,
		ActivityID:           activityID,
		AttestationID:        attestationID,
		AttestationEvidenceJSON: json.RawMessage(mustJSON(map[string]string{
			"route_path":           routePath,
			"source_revision_hash": sourceRevisionHash,
		})),
		AccessPolicy: sourceMetadata.AccessPolicy,
		ExportPolicy: sourceMetadata.ExportPolicy,
		Now:          now,
	}
	for _, se := range sourceMetadata.SourceEntities {
		graphParams.SourceEntities = append(graphParams.SourceEntities, GraphSourceEntity{
			SourceEntityID: se.SourceEntityID,
			Kind:           se.Kind,
			TargetKind:     se.TargetKind,
			TargetID:       se.TargetID,
			DisplayPolicy:  se.DisplayPolicy,
			OpenSurface:    se.OpenSurface,
			EntityJSON:     se.EntityJSON,
		})
	}
	for _, tr := range sourceMetadata.Transclusions {
		graphParams.Transclusions = append(graphParams.Transclusions, GraphTransclusion{
			SourceEntityID:     tr.SourceEntityID,
			HostSelector:       tr.HostSelector,
			SourceSelector:     tr.SourceSelector,
			RelationType:       tr.RelationType,
			DefaultDisplayMode: tr.DefaultDisplayMode,
			SnapshotText:       tr.SnapshotText,
			ContentHash:        tr.ContentHash,
			EntityJSON:         tr.EntityJSON,
		})
	}
	for _, cite := range externalCitations {
		toID, ok := publicCitationTarget(cite)
		if !ok {
			continue
		}
		selector := "{}"
		if len(cite.Selector) > 0 && json.Valid(cite.Selector) {
			selector = string(cite.Selector)
		}
		state := strings.TrimSpace(cite.State)
		if state == "" {
			state = "candidate"
		}
		graphParams.Citations = append(graphParams.Citations, GraphCitation{
			ToID:         toID,
			RelationType: "references",
			FromSelector: wholeSelector,
			ToSelector:   selector,
			State:        state,
			EvidenceRef:  firstNonEmpty(cite.Title, toID),
			Confidence:   0.5,
		})
		citationIDs = append(citationIDs, toID)
	}
	// Source citation edge (is_version_of)
	graphParams.Citations = append(graphParams.Citations, GraphCitation{
		ToID:         req.SourceRevisionID,
		RelationType: "is_version_of",
		FromSelector: wholeSelector,
		ToSelector:   wholeSelector,
		State:        "accepted",
		EvidenceRef:  "source_revision_hash:" + sourceRevisionHash,
		Confidence:   1.0,
	})
	citationIDs = append(citationIDs, sourceCitationID)

	if err := s.graphStore.PublishTextureToGraph(ctx, graphParams); err != nil {
		return nil, fmt.Errorf("platform publish: graph write: %w", err)
	}

	return &PublishTextureResponse{
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
		RollbackID:           "",
		State:                "published",
		VersionHistoryHash:   versionHistoryHash,
		VersionCount:         versionHistory.RevisionCount,
	}, nil
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
		req.Title = defaultTextureProposalTitle
	}
	if req.Citations == nil {
		req.Citations = json.RawMessage("[]")
	}
	if !json.Valid(req.Citations) {
		return nil, fmt.Errorf("citations must be valid JSON")
	}

	var sourceOwnerID, latestVersionID string
	// Look up the publication in the object graph by publication_id metadata.
	og := s.ogStore()
	if og == nil {
		return nil, fmt.Errorf("platform service: object graph store unavailable")
	}
	pubObjs, err := og.ListObjectsByMetadata(ctx, "choir.publication", "$.publication_id", req.PublicationID, 1)
	if err != nil || len(pubObjs) == 0 {
		return nil, fmt.Errorf("publication not found")
	}
	pubObj := pubObjs[0]
	var pubMeta struct {
		OwnerID         string `json:"owner_id"`
		State           string `json:"state"`
		LatestVersionID string `json:"latest_version_id"`
	}
	_ = json.Unmarshal(pubObj.Metadata, &pubMeta)
	sourceOwnerID = pubObj.OwnerID
	latestVersionID = pubMeta.LatestVersionID
	if pubMeta.State != "published" {
		return nil, fmt.Errorf("publication not found")
	}
	if req.PublicationVersionID == "" {
		req.PublicationVersionID = latestVersionID
	}
	// Look up the version in the object graph by publication_version_id metadata.
	versionObjs, err := og.ListObjectsByMetadata(ctx, "choir.publication_version", "$.publication_version_id", req.PublicationVersionID, 1)
	if err != nil || len(versionObjs) == 0 {
		return nil, fmt.Errorf("publication version not found")
	}
	versionContentHash := versionObjs[0].ContentHash

	now := time.Now().UTC()
	proposalID := id("readerprop")
	manifestID := id("artman")
	deliveryID := id("delivery")
	attestationID := id("att")
	activityID := id("activity")
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

	// Build transclusion edges for graph write.
	var transclusionEdges []ProposalTransclusionEdge
	transclusionIDs := []string{}
	for _, ref := range req.Transclusions {
		selector := "{}"
		if len(ref.Selector) > 0 && json.Valid(ref.Selector) {
			selector = string(ref.Selector)
		}
		toID := firstNonEmpty(ref.SpanID, ref.PublicationVersionID, req.PublicationVersionID)
		toKind := "published_texture_span"
		if ref.SpanID == "" {
			toKind = "publication_version"
		}
		transclusionEdges = append(transclusionEdges, ProposalTransclusionEdge{
			ToID:        toID,
			ToKind:      toKind,
			Selector:    selector,
			EvidenceRef: "source_content_hash:" + firstNonEmpty(ref.ContentHash, versionContentHash),
		})
		transclusionIDs = append(transclusionIDs, toID)
	}

	// Build citation edges for graph write.
	var citationEdges []ProposalCitationEdge
	citationIDs := []string{}
	for _, citation := range parseCitationInputs(req.Citations) {
		toID, ok := publicCitationTarget(citation)
		if !ok {
			continue
		}
		citationEdges = append(citationEdges, ProposalCitationEdge{
			ToID:        toID,
			EvidenceRef: firstNonEmpty(citation.Title, toID),
		})
		citationIDs = append(citationIDs, toID)
	}

	// Write to the object graph — single path, no relational tables.
	proposalParams := ProposalGraphParams{
		ProposalID:               proposalID,
		PublicationID:            req.PublicationID,
		PublicationVersionID:     req.PublicationVersionID,
		SourceOwnerID:            sourceOwnerID,
		SubmitterID:              req.SubmitterID,
		SubmitterDocID:           req.SubmitterDocID,
		SubmitterRevisionID:      req.SubmitterRevisionID,
		ProposalRevisionHash:     proposalRevisionHash,
		Content:                  req.Content,
		ContentHash:              contentHash,
		ContentSize:              len([]byte(req.Content)),
		ProjectionHash:           projectionHash,
		Title:                    req.Title,
		ArtifactManifestID:       manifestID,
		ManifestJSON:             manifestJSON,
		ManifestHash:             manifestHash,
		StorageRef:               storageRef,
		DeliveryID:               deliveryID,
		DeliveryRef:              "platform-dolt:publication_version_proposals/" + proposalID,
		ActivityID:               activityID,
		AttestationID:            attestationID,
		SourceVersionContentHash: versionContentHash,
		TransclusionEdges:        transclusionEdges,
		CitationEdges:            citationEdges,
		Now:                      now,
	}
	if err := s.graphStore.SubmitProposalToGraph(ctx, proposalParams); err != nil {
		return nil, fmt.Errorf("platform proposal: graph write: %w", err)
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
	if s == nil || s.store == nil {
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
	if s.graphStore != nil {
		if err := s.graphStore.UpdateProposalDeliveryState(ctx, req.ProposalID, req.DeliveryID, req.DeliveryState, req.DeliveryRef, now); err != nil {
			return nil, fmt.Errorf("platform proposal delivery: update graph: %w", err)
		}
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
	if normalized != "/" && strings.HasPrefix(normalized, publicTexturePrefix) {
		normalized = strings.TrimRight(normalized, "/")
	}
	return normalized
}

func slugify(raw string) string {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" {
		return "texture"
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
		return "texture"
	}
	if len(slug) > 96 {
		slug = strings.Trim(slug[:96], "-")
	}
	if slug == "" {
		return "texture"
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
